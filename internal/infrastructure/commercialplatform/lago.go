package commercialplatform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
)

// healthRequestTimeout bounds one readiness probe of the authority's health
// signal. publishRequestTimeout bounds one publish command's HTTP requests
// (plan create + metric resolution + feature/entitlement calls are each
// individually bounded through ctx).
const (
	healthRequestTimeout  = 5 * time.Second
	publishRequestTimeout = 60 * time.Second
)

// LagoAdapter implements the frozen commercial.CommercialPlatform seam
// against a self-hosted Lago deployment (ADR-0012). T05 implemented the
// readiness snapshot; T07 (#79) adds the first command kind:
// publish_plan_version — POST /api/v1/plans with the pinned v1.53.0 payload
// contract (pay_in_advance mandatory, integer amount_cents, decimal-string
// charge amounts ×100, billable_metric_id references, MAP-form entitlement
// attach; lab facts from #74/#76/#77). The reconcile family stays frozen
// and fails closed.
//
// Idempotency is coordinator-owned: the caller's Command.Key addresses the
// deterministic plan code; a 422 already-exists is resolved by a read-back
// GET + field compare (#76 lab: 422 bodies cannot distinguish replay from
// conflict), never by a second create.
//
// Fail-closed contract: the adapter never fabricates an outcome. Missing
// config fails fast with ErrPlatformUnconfigured; transport failures,
// timeouts and 5xx wrap ErrPlatformUnreachable (the outcome is unknown); a
// definitive wrong answer (other 4xx, read-back mismatch) wraps
// ErrPlatformInvalidResponse. Error strings carry no URL, no status text,
// no response body and no credential — only the provider-neutral sentinel
// and a short closed description.
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

// SubmitCommand applies the enabled command families. publish_plan_version
// is the first (T07); every other kind fails closed unsupported.
func (a *LagoAdapter) SubmitCommand(ctx context.Context, cmd commercial.Command) (commercial.CommandReceipt, error) {
	if err := cmd.Validate(); err != nil {
		return commercial.CommandReceipt{}, err
	}
	switch cmd.Kind {
	case commercial.CommandKindPublishPlanVersion:
		return a.submitPublishPlanVersion(ctx, cmd)
	default:
		return commercial.CommandReceipt{}, commercial.ErrPlatformUnsupported
	}
}

// Reconcile stays frozen and disabled: fail closed.
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

// ---- publish_plan_version (T07, #79) ----
// Provider vocabulary (plan, charges, billable_metric_id, entitlements)
// lives ONLY in this section — it never crosses into the commercial port
// types or any caller-visible response.

// submitPublishPlanVersion runs the publish pipeline: validate payload →
// ensure features exist → resolve charge dimensions to billable metrics →
// create the plan (422 → read-back verify) → attach entitlements in the
// MAP form → receipt. Every step fails closed with the shared sentinel
// mapping; no provider text rides along.
func (a *LagoAdapter) submitPublishPlanVersion(ctx context.Context, cmd commercial.Command) (commercial.CommandReceipt, error) {
	if err := a.configured(); err != nil {
		return commercial.CommandReceipt{}, err
	}
	payload, ok := cmd.Payload.(commercial.PublishPlanVersionPayload)
	if !ok {
		return commercial.CommandReceipt{}, fmt.Errorf("%w: publish payload has the wrong type", commercial.ErrPlatformInvalidResponse)
	}
	if err := payload.Validate(); err != nil {
		return commercial.CommandReceipt{}, fmt.Errorf("%w: %v", commercial.ErrPlatformInvalidResponse, err)
	}
	ctx, cancel := context.WithTimeout(ctx, publishRequestTimeout)
	defer cancel()

	for key := range payload.Features {
		if err := a.ensureFeature(ctx, key); err != nil {
			return commercial.CommandReceipt{}, err
		}
	}
	charges, err := a.resolveCharges(ctx, payload.Charges)
	if err != nil {
		return commercial.CommandReceipt{}, err
	}
	if err := a.createPlan(ctx, payload, charges); err != nil {
		return commercial.CommandReceipt{}, err
	}
	if err := a.attachEntitlements(ctx, payload); err != nil {
		return commercial.CommandReceipt{}, err
	}
	return commercial.CommandReceipt{
		Key:        cmd.Key,
		ExternalID: payload.PlanCode,
		RecordedAt: time.Now().UTC(),
	}, nil
}

// ensureFeature registers the entitlement feature; an already-exists answer
// (422 value_already_exist, the T04 idempotency fact) is success.
func (a *LagoAdapter) ensureFeature(ctx context.Context, code string) error {
	status, _, err := a.do(ctx, http.MethodPost, "/api/v1/features", map[string]any{
		"feature": map[string]any{"code": code, "name": code},
	})
	if err != nil {
		return err
	}
	switch {
	case status >= 200 && status < 300, status == http.StatusUnprocessableEntity:
		return nil
	case status >= 500:
		return fmt.Errorf("%w: entitlement registration unavailable", commercial.ErrPlatformUnreachable)
	default:
		return fmt.Errorf("%w: entitlement registration rejected", commercial.ErrPlatformInvalidResponse)
	}
}

// lagoCharge is the provider wire shape of one usage charge. The dimension
// arrives as the provider metric's id reference (T04 fact: id, not code);
// the fen amount crosses as an exact decimal string the provider multiplies
// by 100 itself.
type lagoCharge struct {
	BillableMetricID string `json:"billable_metric_id"`
	ChargeModel      string `json:"charge_model"`
	PayInAdvance     bool   `json:"pay_in_advance"`
	Properties       struct {
		Amount      string `json:"amount"`
		PackageSize int64  `json:"package_size,omitempty"`
		FreeUnits   int64  `json:"free_units,omitempty"`
	} `json:"properties"`
}

// resolveCharges maps every product charge dimension to the provider
// billable metric. An unprovisioned dimension fails closed — no plan is
// created on a pricing surface we cannot fully express.
func (a *LagoAdapter) resolveCharges(ctx context.Context, charges []commercial.PlanCharge) ([]lagoCharge, error) {
	if len(charges) == 0 {
		return nil, nil
	}
	out := make([]lagoCharge, 0, len(charges))
	for _, c := range charges {
		id, err := a.resolveBillableMetric(ctx, c.Dimension)
		if err != nil {
			return nil, err
		}
		charge := lagoCharge{
			BillableMetricID: id,
			// Usage charges are pay-in-arrears SUM charges (T04 fixture).
			PayInAdvance: false,
		}
		charge.Properties.Amount = commercial.FenToDecimalString(c.AmountFen)
		charge.Properties.FreeUnits = c.FreeUnits
		switch c.Model {
		case commercial.ChargeModelFixedUnit:
			charge.ChargeModel = "standard"
		case commercial.ChargeModelPackage:
			charge.ChargeModel = "package"
			charge.Properties.PackageSize = c.PackageUnits
		}
		out = append(out, charge)
	}
	return out, nil
}

// resolveBillableMetric reads the metric id for one dimension code.
func (a *LagoAdapter) resolveBillableMetric(ctx context.Context, dimension string) (string, error) {
	status, body, err := a.do(ctx, http.MethodGet, "/api/v1/billable_metrics/"+dimension, nil)
	if err != nil {
		return "", err
	}
	switch {
	case status == http.StatusNotFound:
		return "", fmt.Errorf("%w: pricing dimension not provisioned", commercial.ErrPlatformInvalidResponse)
	case status >= 500:
		return "", fmt.Errorf("%w: pricing dimension lookup unavailable", commercial.ErrPlatformUnreachable)
	case status >= 200 && status < 300:
	default:
		return "", fmt.Errorf("%w: pricing dimension lookup rejected", commercial.ErrPlatformInvalidResponse)
	}
	var parsed struct {
		BillableMetric struct {
			LagoID string `json:"lago_id"`
		} `json:"billable_metric"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.BillableMetric.LagoID == "" {
		return "", fmt.Errorf("%w: pricing dimension lookup malformed", commercial.ErrPlatformInvalidResponse)
	}
	return parsed.BillableMetric.LagoID, nil
}

// createPlan posts the plan create. A 422 already-exists is resolved by the
// read-back verify below — the idempotent replay path (#76 lab: 422 bodies
// are indistinguishable, so the CONTENT decides).
func (a *LagoAdapter) createPlan(ctx context.Context, payload commercial.PublishPlanVersionPayload, charges []lagoCharge) error {
	plan := map[string]any{
		"code":            payload.PlanCode,
		"name":            payload.Name,
		"interval":        payload.Interval,
		"amount_cents":    payload.AmountFen,
		"amount_currency": payload.Currency,
		"pay_in_advance":  true, // T03 fact: plan-level pay_in_advance is mandatory
		"trial_period":    0.0,
	}
	if len(charges) > 0 {
		plan["charges"] = charges
	}
	status, _, err := a.do(ctx, http.MethodPost, "/api/v1/plans", map[string]any{"plan": plan})
	if err != nil {
		return err
	}
	switch {
	case status >= 200 && status < 300:
		return nil
	case status == http.StatusUnprocessableEntity:
		return a.verifyPlanReplay(ctx, payload)
	case status >= 500:
		return fmt.Errorf("%w: plan create unavailable", commercial.ErrPlatformUnreachable)
	default:
		return fmt.Errorf("%w: plan create rejected", commercial.ErrPlatformInvalidResponse)
	}
}

// verifyPlanReplay reads the plan back and compares the priced identity
// fields. Equal content → the 422 was an idempotent replay (nil); different
// content → a publish conflict (spec story 59).
func (a *LagoAdapter) verifyPlanReplay(ctx context.Context, payload commercial.PublishPlanVersionPayload) error {
	status, body, err := a.do(ctx, http.MethodGet, "/api/v1/plans/"+payload.PlanCode, nil)
	if err != nil {
		return err
	}
	switch {
	case status == http.StatusNotFound:
		return fmt.Errorf("%w: plan replay cannot be verified", commercial.ErrPlatformInvalidResponse)
	case status >= 500:
		return fmt.Errorf("%w: plan read-back unavailable", commercial.ErrPlatformUnreachable)
	case status >= 200 && status < 300:
	default:
		return fmt.Errorf("%w: plan read-back rejected", commercial.ErrPlatformInvalidResponse)
	}
	var parsed struct {
		Plan struct {
			AmountCents    json.Number `json:"amount_cents"`
			AmountCurrency string      `json:"amount_currency"`
			Interval       string      `json:"interval"`
			PayInAdvance   bool        `json:"pay_in_advance"`
		} `json:"plan"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("%w: plan read-back malformed", commercial.ErrPlatformInvalidResponse)
	}
	if parsed.Plan.AmountCents.String() != strconv.FormatInt(payload.AmountFen, 10) ||
		parsed.Plan.AmountCurrency != payload.Currency ||
		parsed.Plan.Interval != payload.Interval ||
		!parsed.Plan.PayInAdvance {
		return fmt.Errorf("%w: publish conflict", commercial.ErrPlatformInvalidResponse)
	}
	return nil
}

// attachEntitlements attaches the plan entitlements in the MAP form keyed
// by feature code (T02 lab fact: an array makes the runtime 500).
func (a *LagoAdapter) attachEntitlements(ctx context.Context, payload commercial.PublishPlanVersionPayload) error {
	if len(payload.Features) == 0 {
		return nil
	}
	entitlements := make(map[string]any, len(payload.Features))
	for key := range payload.Features {
		entitlements[key] = map[string]any{}
	}
	status, _, err := a.do(ctx, http.MethodPost,
		"/api/v1/plans/"+payload.PlanCode+"/entitlements",
		map[string]any{"entitlements": entitlements})
	if err != nil {
		return err
	}
	switch {
	case status >= 200 && status < 300:
		return nil
	case status >= 500:
		return fmt.Errorf("%w: entitlement attach unavailable", commercial.ErrPlatformUnreachable)
	default:
		return fmt.Errorf("%w: entitlement attach rejected", commercial.ErrPlatformInvalidResponse)
	}
}

// do issues one authenticated JSON request against the configured base URL
// and classifies transport failures as unreachable. The credential rides
// ONLY in the Authorization header; it never appears in an error.
func (a *LagoAdapter) do(ctx context.Context, method, path string, body any) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		blob, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("%w: request encoding failed", commercial.ErrPlatformInvalidResponse)
		}
		reader = bytes.NewReader(blob)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimSuffix(a.cfg.BaseURL, "/")+path, reader)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: invalid platform endpoint", commercial.ErrPlatformUnconfigured)
	}
	req.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: request not reachable", commercial.ErrPlatformUnreachable)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, data, nil
}
