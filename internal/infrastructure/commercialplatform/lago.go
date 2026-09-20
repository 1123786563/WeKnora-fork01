package commercialplatform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
)

// healthRequestTimeout bounds one readiness probe of the authority's health
// signal and one customers API call.
const healthRequestTimeout = 5 * time.Second

// LagoAdapter implements the frozen commercial.CommercialPlatform seam
// against a self-hosted Lago deployment (ADR-0012). T05 implements exactly
// one snapshot: readiness, read from the /health liveness signal (#73 probe
// fact: unauthenticated; version identity is deployment config — the
// configured Release pin — never text parsed from the response). The command
// and reconcile families stay frozen and fail closed.
//
// Fail-closed contract: the adapter never fabricates readiness. Missing
// config fails fast with ErrPlatformUnconfigured; transport failures,
// timeouts and 5xx wrap ErrPlatformUnreachable (the outcome is unknown,
// never ready); a 4xx wraps ErrPlatformInvalidResponse (a definitive wrong
// answer, not a retry). Error strings carry no URL, no status text, no
// response body and no credential — only the provider-neutral sentinel and
// a short closed description.
type LagoAdapter struct {
	cfg    Config
	client *http.Client
}

// NewLagoAdapter builds the adapter. Construction succeeds unconfigured on
// purpose (blocked-env stays legal, openmeter precedent); the calls then
// fail fast instead of fabricating answers.
func NewLagoAdapter(cfg Config) *LagoAdapter {
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: healthRequestTimeout}
	}
	return &LagoAdapter{cfg: cfg, client: client}
}

// ReadSnapshot answers the readiness snapshot (GET <base>/health) and the
// account snapshot (GET <api/v1/customers/{external_id}> presence).
// Unknown kinds fail closed unsupported.
func (a *LagoAdapter) ReadSnapshot(ctx context.Context, query commercial.SnapshotQuery) (commercial.Snapshot, error) {
	switch query.Kind {
	case commercial.SnapshotKindReadiness:
		if err := a.configured(); err != nil {
			return commercial.Snapshot{}, err
		}
		if err := a.probeHealth(ctx); err != nil {
			return commercial.Snapshot{}, err
		}
		return commercial.Snapshot{
			Kind: commercial.SnapshotKindReadiness,
			Readiness: &commercial.ReadinessSnapshot{
				State:     commercial.ReadinessReady,
				Release:   a.cfg.Release,
				CheckedAt: time.Now().UTC(),
			},
		}, nil
	case commercial.SnapshotKindAccount:
		if err := a.configured(); err != nil {
			return commercial.Snapshot{}, err
		}
		// The snapshot is addressed by the query's tenant ONLY; the identity
		// is derived, so no caller can name another tenant's customer.
		if query.TenantID == 0 {
			return commercial.Snapshot{}, commercial.ErrPlatformUnsupported
		}
		present, err := a.customerPresent(ctx, commercial.ExternalCustomerID(query.TenantID))
		if err != nil {
			return commercial.Snapshot{}, err
		}
		state := commercial.AccountStateAbsent
		if present {
			state = commercial.AccountStateLinked
		}
		return commercial.Snapshot{
			Kind: commercial.SnapshotKindAccount,
			Account: &commercial.AccountSnapshot{
				TenantID:  query.TenantID,
				State:     state,
				CheckedAt: time.Now().UTC(),
			},
		}, nil
	default:
		return commercial.Snapshot{}, commercial.ErrPlatformUnsupported
	}
}

// SubmitCommand implements ensure_customer (the W3 first enabled kind,
// #78) with READ-BEFORE-CREATE: the customer identity is resolved by
// GET /api/v1/customers/{external_id} first and the create POST fires only
// when the authority holds none — so a replay after ANY lost response
// resolves by identity and never issues a second create (Lago's
// create-on-external_id upsert semantics would help but are not
// load-bearing). Every other kind fails closed unsupported.
func (a *LagoAdapter) SubmitCommand(ctx context.Context, cmd commercial.Command) (commercial.CommandReceipt, error) {
	switch cmd.Kind {
	case commercial.CommandKindEnsureCustomer:
		return a.ensureCustomer(ctx, cmd)
	default:
		return commercial.CommandReceipt{}, commercial.ErrPlatformUnsupported
	}
}

// Reconcile stays frozen and disabled: fail closed.
func (a *LagoAdapter) Reconcile(_ context.Context, _ commercial.ReconciliationCursor) (commercial.ReconciliationPage, error) {
	return commercial.ReconciliationPage{}, commercial.ErrPlatformUnsupported
}

// ensureCustomer runs the read-before-create algorithm. The payload guard
// refuses an identity inconsistent with the derivation BEFORE any request
// leaves the adapter — a mismatched identity is never silently forwarded.
func (a *LagoAdapter) ensureCustomer(ctx context.Context, cmd commercial.Command) (commercial.CommandReceipt, error) {
	if err := a.configured(); err != nil {
		return commercial.CommandReceipt{}, err
	}
	payload, ok := cmd.Payload.(commercial.EnsureCustomerPayload)
	if !ok || payload.TenantID == 0 || payload.ExternalCustomerID == "" ||
		payload.ExternalCustomerID != commercial.ExternalCustomerID(payload.TenantID) {
		return commercial.CommandReceipt{}, commercial.ErrPlatformUnsupported
	}
	present, err := a.customerPresent(ctx, payload.ExternalCustomerID)
	if err != nil {
		return commercial.CommandReceipt{}, err
	}
	if !present {
		if err := a.createCustomer(ctx, payload.ExternalCustomerID, payload.DisplayName); err != nil {
			return commercial.CommandReceipt{}, err
		}
	}
	return commercial.CommandReceipt{
		Key:        cmd.Key,
		ExternalID: payload.ExternalCustomerID,
		RecordedAt: time.Now().UTC(),
	}, nil
}

// customerPresent issues GET /api/v1/customers/{external_id} and reports
// whether the identity exists. Presence is decided by the status alone; no
// response body is parsed or surfaced.
func (a *LagoAdapter) customerPresent(ctx context.Context, externalID string) (bool, error) {
	endpoint := strings.TrimSuffix(a.cfg.BaseURL, "/") + "/api/v1/customers/" + url.PathEscape(externalID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, fmt.Errorf("%w: invalid platform endpoint", commercial.ErrPlatformUnconfigured)
	}
	req.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)
	resp, err := a.client.Do(req)
	if err != nil {
		// Timeout, dropped connection: the remote state is unknown.
		return false, fmt.Errorf("%w: customer read not reachable", commercial.ErrPlatformUnreachable)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 8<<10))
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return true, nil
	case resp.StatusCode == http.StatusNotFound:
		return false, nil
	case resp.StatusCode >= 500:
		return false, fmt.Errorf("%w: customer read unavailable", commercial.ErrPlatformUnreachable)
	default:
		return false, fmt.Errorf("%w: customer read rejected", commercial.ErrPlatformInvalidResponse)
	}
}

// createCustomer issues POST /api/v1/customers with the #73 runtime-proven
// payload {"customer":{"external_id","name"}} and classifies the outcome.
// The response body is discarded — the receipt identity is the REQUESTED
// deterministic identity, never parsed provider text.
func (a *LagoAdapter) createCustomer(ctx context.Context, externalID, displayName string) error {
	body, err := json.Marshal(map[string]any{
		"customer": map[string]string{"external_id": externalID, "name": displayName},
	})
	if err != nil {
		return fmt.Errorf("%w: invalid customer payload", commercial.ErrPlatformInvalidResponse)
	}
	endpoint := strings.TrimSuffix(a.cfg.BaseURL, "/") + "/api/v1/customers"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%w: invalid platform endpoint", commercial.ErrPlatformUnconfigured)
	}
	req.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		// The create outcome is INDETERMINATE: the customer may exist now —
		// the next ensure resolves by identity before any create.
		return fmt.Errorf("%w: customer create not reachable", commercial.ErrPlatformUnreachable)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 8<<10))
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode >= 500:
		return fmt.Errorf("%w: customer create unavailable", commercial.ErrPlatformUnreachable)
	default:
		return fmt.Errorf("%w: customer create rejected", commercial.ErrPlatformInvalidResponse)
	}
}

func (a *LagoAdapter) configured() error {
	if a.cfg.BaseURL == "" || a.cfg.APIKey == "" {
		return commercial.ErrPlatformUnconfigured
	}
	return nil
}

// probeHealth issues the one readiness request and classifies its outcome.
// The liveness signal needs no credential (it is never sent), and no
// response body is parsed or surfaced.
func (a *LagoAdapter) probeHealth(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimSuffix(a.cfg.BaseURL, "/")+"/health", nil)
	if err != nil {
		return fmt.Errorf("%w: invalid platform endpoint", commercial.ErrPlatformUnconfigured)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		// Timeout, dropped connection, refused endpoint: the remote state is
		// unknown — never a fabricated ready.
		return fmt.Errorf("%w: health signal not reachable", commercial.ErrPlatformUnreachable)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode >= 500:
		return fmt.Errorf("%w: health signal unavailable", commercial.ErrPlatformUnreachable)
	default:
		return fmt.Errorf("%w: health signal rejected the request", commercial.ErrPlatformInvalidResponse)
	}
}
