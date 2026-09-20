package commercialplatform

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
)

// healthRequestTimeout bounds one readiness probe of the authority's health
// signal.
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

// ReadSnapshot answers the readiness snapshot: GET <base>/health with a
// short deadline, honored through ctx. A 2xx means ready; the Release is the
// deployment pin from config, never response text. Unknown kinds fail
// closed unsupported.
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
	default:
		return commercial.Snapshot{}, commercial.ErrPlatformUnsupported
	}
}

// SubmitCommand is frozen and disabled in T05: fail closed.
func (a *LagoAdapter) SubmitCommand(_ context.Context, _ commercial.Command) (commercial.CommandReceipt, error) {
	return commercial.CommandReceipt{}, commercial.ErrPlatformUnsupported
}

// Reconcile is frozen and disabled in T05: fail closed.
func (a *LagoAdapter) Reconcile(_ context.Context, _ commercial.ReconciliationCursor) (commercial.ReconciliationPage, error) {
	return commercial.ReconciliationPage{}, commercial.ErrPlatformUnsupported
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
