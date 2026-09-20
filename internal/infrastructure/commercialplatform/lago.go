package commercialplatform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
)

// healthRequestTimeout bounds one readiness probe of the authority's health
// signal and one customers API call. publishRequestTimeout bounds one
// publish command's HTTP requests (plan create + metric resolution +
// feature/entitlement calls are each individually bounded through ctx).
// subscriptionRequestTimeout bounds one ensure_subscription or grant command.
const (
	healthRequestTimeout       = 5 * time.Second
	publishRequestTimeout      = 60 * time.Second
	subscriptionRequestTimeout = 15 * time.Second
)

// Wallet settle-wait and retry tuning (T03 verdict facts). The a1 fact:
// wallet-create credits settle via an after-commit job, so the adapter polls
// the wallet balance (walletSettleTimeout budget, walletSettleTick cadence)
// before answering the grant. The wallet-cap fact: a wallet_limit_reached
// 422 on the monthly grant is retried across the authority's hourly
// termination tick (E1: expired wallets terminate lazily, ~65 min worst
// case) with walletLimitRetryTick cadence inside walletLimitRetryBudget;
// an exhausted budget answers the closed indeterminate unreachable — the
// grant replays safely by identity on the next access. Vars (not consts) so
// tests shorten them; production never touches them.
var (
	walletSettleTimeout    = 10 * time.Second
	walletSettleTick       = 500 * time.Millisecond
	walletLimitRetryBudget = 70 * time.Second
	walletLimitRetryTick   = 5 * time.Second
)

// LagoAdapter implements the frozen commercial.CommercialPlatform seam
// against a self-hosted Lago deployment (ADR-0012). T05 implemented the
// readiness snapshot; W3 (#78) adds ensure_customer (read-before-create on
// GET/POST /api/v1/customers/{external_id}) and the account snapshot; T07
// (#79) adds publish_plan_version — POST /api/v1/plans with the pinned
// v1.53.0 payload
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
	case commercial.SnapshotKindBenefits:
		return a.readBenefitsSnapshot(ctx, query.TenantID)
	default:
		return commercial.Snapshot{}, commercial.ErrPlatformUnsupported
	}
}

// SubmitCommand applies the enabled command families. ensure_customer (the
// W3 first enabled kind, #78) runs READ-BEFORE-CREATE: the customer identity
// is resolved by GET /api/v1/customers/{external_id} first and the create
// POST fires only when the authority holds none — so a replay after ANY lost
// response resolves by identity and never issues a second create (Lago's
// create-on-external_id upsert semantics would help but are not
// load-bearing). publish_plan_version (T07, #79) runs the publish pipeline.
// Every other kind fails closed unsupported.
func (a *LagoAdapter) SubmitCommand(ctx context.Context, cmd commercial.Command) (commercial.CommandReceipt, error) {
	if err := cmd.Validate(); err != nil {
		return commercial.CommandReceipt{}, err
	}
	switch cmd.Kind {
	case commercial.CommandKindEnsureCustomer:
		return a.ensureCustomer(ctx, cmd)
	case commercial.CommandKindPublishPlanVersion:
		return a.submitPublishPlanVersion(ctx, cmd)
	case commercial.CommandKindEnsureSubscription:
		return a.ensureSubscription(ctx, cmd)
	case commercial.CommandKindGrantIncludedCredits:
		return a.grantIncludedCredits(ctx, cmd)
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

// ---- ensure_subscription / grant_included_credits / benefits (T08, #80) ----
// Provider vocabulary (subscriptions, wallets, entitlements, granted_credits,
// status[] filters) lives ONLY in this section — it never crosses into the
// commercial port types or any caller-visible response. Receipts echo
// WeKnora-derived identities only (the deterministic subscription id / the
// deterministic wallet name); provider lago_ids stay inside the seam.

// lagoSubscription is the provider wire shape of one subscription entry
// (the index answers an array of these; a nested "subscription" object is
// tolerated defensively).
type lagoSubscription struct {
	LagoID             string `json:"lago_id"`
	ExternalID         string `json:"external_id"`
	ExternalCustomerID string `json:"external_customer_id"`
	PlanCode           string `json:"plan_code"`
	Status             string `json:"status"`
}

// subscriptionIndexStatuses is the EXPLICIT status set every identity read
// passes (T02 lab fact: the subscription index defaults to status=active,
// so an incomplete or recently-canceled subscription would otherwise be
// invisible and a second create would be attempted).
var subscriptionIndexStatuses = []string{"active", "incomplete", "canceled", "terminated"}

// ensureSubscription runs the idempotent standard-subscription algorithm:
// identity read with explicit statuses → found + same plan code = replay
// receipt; found + different plan code = definitive conflict (never a
// parallel subscription); absent → ONE create with no activation_rules (the
// free Base Plan must never gate on payment — T02 decision). A create 422
// is resolved by an identity re-read, never a second create.
func (a *LagoAdapter) ensureSubscription(ctx context.Context, cmd commercial.Command) (commercial.CommandReceipt, error) {
	if err := a.configured(); err != nil {
		return commercial.CommandReceipt{}, err
	}
	payload, ok := cmd.Payload.(commercial.EnsureSubscriptionPayload)
	if !ok {
		return commercial.CommandReceipt{}, commercial.ErrPlatformUnsupported
	}
	if err := payload.Validate(); err != nil {
		return commercial.CommandReceipt{}, fmt.Errorf("%w: %v", commercial.ErrPlatformInvalidResponse, err)
	}
	ctx, cancel := context.WithTimeout(ctx, subscriptionRequestTimeout)
	defer cancel()

	receipt := commercial.CommandReceipt{
		Key:        cmd.Key,
		ExternalID: payload.ExternalSubscriptionID,
		RecordedAt: time.Now().UTC(),
	}
	sub, found, err := a.readSubscriptionByIdentity(ctx, payload.ExternalSubscriptionID)
	if err != nil {
		return commercial.CommandReceipt{}, err
	}
	if found {
		if sub.PlanCode != payload.PlanCode {
			return commercial.CommandReceipt{}, fmt.Errorf("%w: subscription plan conflict", commercial.ErrPlatformInvalidResponse)
		}
		return receipt, nil
	}
	status, _, err := a.createSubscription(ctx, payload)
	if err != nil {
		return commercial.CommandReceipt{}, err
	}
	switch {
	case status >= 200 && status < 300:
		return receipt, nil
	case status == http.StatusUnprocessableEntity:
		// Indistinguishable 422 (#76 lab fact): the content decides by
		// identity re-read + plan-code compare — never a second create.
		sub, found, err := a.readSubscriptionByIdentity(ctx, payload.ExternalSubscriptionID)
		if err != nil {
			return commercial.CommandReceipt{}, err
		}
		if found && sub.PlanCode == payload.PlanCode {
			return receipt, nil
		}
		return commercial.CommandReceipt{}, fmt.Errorf("%w: subscription replay cannot be verified", commercial.ErrPlatformInvalidResponse)
	default:
		return commercial.CommandReceipt{}, classifySubscriptionStatus(status, "subscription create")
	}
}

// readSubscriptionByIdentity answers the subscription held under the
// external identity, filtered by the explicit status set.
func (a *LagoAdapter) readSubscriptionByIdentity(ctx context.Context, externalSubscriptionID string) (lagoSubscription, bool, error) {
	query := url.Values{}
	query.Set("external_id", externalSubscriptionID)
	for _, s := range subscriptionIndexStatuses {
		query.Add("status[]", s)
	}
	status, body, err := a.do(ctx, http.MethodGet, "/api/v1/subscriptions?"+query.Encode(), nil)
	if err != nil {
		return lagoSubscription{}, false, err
	}
	switch {
	case status >= 200 && status < 300:
	case status >= 500:
		return lagoSubscription{}, false, fmt.Errorf("%w: subscription read unavailable", commercial.ErrPlatformUnreachable)
	default:
		return lagoSubscription{}, false, fmt.Errorf("%w: subscription read rejected", commercial.ErrPlatformInvalidResponse)
	}
	var parsed struct {
		Subscriptions []json.RawMessage `json:"subscriptions"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return lagoSubscription{}, false, fmt.Errorf("%w: subscription read malformed", commercial.ErrPlatformInvalidResponse)
	}
	for _, raw := range parsed.Subscriptions {
		var sub lagoSubscription
		if err := json.Unmarshal(raw, &sub); err != nil || sub.ExternalID == "" {
			// Tolerate the nested {"subscription": {...}} envelope.
			var nested struct {
				Subscription lagoSubscription `json:"subscription"`
			}
			if json.Unmarshal(raw, &nested) == nil && nested.Subscription.ExternalID != "" {
				sub = nested.Subscription
			} else {
				continue
			}
		}
		if sub.ExternalID == externalSubscriptionID {
			return sub, true, nil
		}
	}
	return lagoSubscription{}, false, nil
}

// createSubscription posts ONE standard subscription create. The body never
// carries activation_rules — payment gating needs a supported provider and
// the free Base Plan must not depend on one (T02 decision).
func (a *LagoAdapter) createSubscription(ctx context.Context, payload commercial.EnsureSubscriptionPayload) (int, []byte, error) {
	body := map[string]any{
		"subscription": map[string]any{
			"external_customer_id": payload.ExternalCustomerID,
			"external_id":          payload.ExternalSubscriptionID,
			"plan_code":            payload.PlanCode,
			"name":                 "WeKnora Space " + strconv.FormatUint(payload.TenantID, 10),
		},
	}
	status, respBody, err := a.do(ctx, http.MethodPost, "/api/v1/subscriptions", body)
	if err != nil {
		return 0, nil, err
	}
	if status >= 500 {
		return status, nil, fmt.Errorf("%w: subscription create unavailable", commercial.ErrPlatformUnreachable)
	}
	if status >= 400 && status < 500 && status != http.StatusUnprocessableEntity {
		return status, nil, fmt.Errorf("%w: subscription create rejected", commercial.ErrPlatformInvalidResponse)
	}
	return status, respBody, nil
}

// classifySubscriptionStatus maps a non-2xx, non-422 status onto the shared
// sentinel set (5xx unreachable; other 4xx invalid response).
func classifySubscriptionStatus(status int, what string) error {
	if status >= 500 {
		return fmt.Errorf("%w: %s unavailable", commercial.ErrPlatformUnreachable, what)
	}
	return fmt.Errorf("%w: %s rejected", commercial.ErrPlatformInvalidResponse, what)
}

// lagoWallet is the provider wire shape of one wallet.
type lagoWallet struct {
	LagoID         string            `json:"lago_id"`
	Name           string            `json:"name"`
	Status         string            `json:"status"`
	BalanceCents   int64             `json:"balance_cents"`
	GrantedCredits string            `json:"granted_credits"`
	ExpirationAt   string            `json:"expiration_at"`
	MetadataMap    map[string]string `json:"metadata"`
	MetadataList   []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"metadata_array"`
}

// meta answers the wallet metadata in map form whatever wire shape it rode.
func (w lagoWallet) meta() map[string]string {
	if w.MetadataMap != nil {
		return w.MetadataMap
	}
	out := make(map[string]string, len(w.MetadataList))
	for _, kv := range w.MetadataList {
		out[kv.Key] = kv.Value
	}
	return out
}

// grantIncludedCredits runs the three-layer grant algorithm. Layer 1 is the
// caller's registry (this adapter never sees it); layer 2 is
// READ-BEFORE-CREATE on the customer wallet list matched by the
// deterministic wallet name (metadata fallback — the E3 recovery obligation);
// layer 3 is simply never issuing a blind create after an indeterminate
// answer. A wallet_limit_reached 422 is a bounded retry across the hourly
// termination tick; the settle-wait polls the wallet balance until the
// after-commit job has credited the grant.
func (a *LagoAdapter) grantIncludedCredits(ctx context.Context, cmd commercial.Command) (commercial.CommandReceipt, error) {
	if err := a.configured(); err != nil {
		return commercial.CommandReceipt{}, err
	}
	payload, ok := cmd.Payload.(commercial.GrantIncludedCreditsPayload)
	if !ok {
		return commercial.CommandReceipt{}, commercial.ErrPlatformUnsupported
	}
	if err := payload.Validate(); err != nil {
		return commercial.CommandReceipt{}, fmt.Errorf("%w: %v", commercial.ErrPlatformInvalidResponse, err)
	}
	ctx, cancel := context.WithTimeout(ctx, subscriptionRequestTimeout)
	defer cancel()

	walletName := commercial.MonthlyWalletName(payload.TenantID, payload.Period)
	grantCents := payload.CreditsMicro / 10_000
	deadline := time.Now().Add(walletLimitRetryBudget)
	for {
		wallets, err := a.listCustomerWallets(ctx, payload.ExternalCustomerID)
		if err != nil {
			return commercial.CommandReceipt{}, err
		}
		for _, w := range wallets {
			meta := w.meta()
			byName := w.Name == walletName
			byMeta := meta[commercial.WalletMetaTenant] == payload.ExternalCustomerID &&
				meta[commercial.WalletMetaPeriod] == payload.Period
			if !byName && !byMeta {
				continue
			}
			// Same identity, different granted amount = same Key with
			// different content — a definitive conflict, never a re-grant.
			if got := parseDecimalCentsLoose(w.GrantedCredits); got > 0 && got != grantCents {
				return commercial.CommandReceipt{}, fmt.Errorf("%w: grant content conflict", commercial.ErrPlatformInvalidResponse)
			}
			return commercial.CommandReceipt{
				Key:        cmd.Key,
				ExternalID: walletName,
				RecordedAt: time.Now().UTC(),
			}, nil
		}
		// Absent: create the short-TTL wallet for this month.
		status, body, err := a.createWallet(ctx, payload, walletName)
		if err != nil {
			return commercial.CommandReceipt{}, err
		}
		switch {
		case status >= 200 && status < 300:
			var parsed struct {
				Wallet struct {
					LagoID string `json:"lago_id"`
				} `json:"wallet"`
			}
			if err := json.Unmarshal(body, &parsed); err != nil || parsed.Wallet.LagoID == "" {
				return commercial.CommandReceipt{}, fmt.Errorf("%w: wallet create malformed", commercial.ErrPlatformInvalidResponse)
			}
			if err := a.waitForWalletSettlement(ctx, parsed.Wallet.LagoID, grantCents); err != nil {
				return commercial.CommandReceipt{}, err
			}
			return commercial.CommandReceipt{
				Key:        cmd.Key,
				ExternalID: walletName,
				RecordedAt: time.Now().UTC(),
			}, nil
		case status == http.StatusUnprocessableEntity && strings.Contains(string(body), "wallet_limit_reached"):
			// Bounded retry across the hourly termination tick (E1); an
			// exhausted budget is the closed indeterminate unreachable —
			// the grant replays safely by identity on the next access.
			if time.Now().After(deadline) {
				return commercial.CommandReceipt{}, fmt.Errorf("%w: wallet capacity not freed in time", commercial.ErrPlatformUnreachable)
			}
			if err := sleepCtx(ctx, walletLimitRetryTick); err != nil {
				return commercial.CommandReceipt{}, fmt.Errorf("%w: wallet capacity retry interrupted", commercial.ErrPlatformUnreachable)
			}
			continue
		default:
			return commercial.CommandReceipt{}, classifySubscriptionStatus(status, "wallet create")
		}
	}
}

// createWallet posts the short-TTL monthly wallet. Credits cross as an
// exact decimal string (never binary float); rate_amount "1" keeps credit
// cents == currency cents; the metadata anchors E3 recovery-by-query.
func (a *LagoAdapter) createWallet(ctx context.Context, payload commercial.GrantIncludedCreditsPayload, walletName string) (int, []byte, error) {
	granted, err := commercial.MicroToDecimalString(payload.CreditsMicro)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: %v", commercial.ErrPlatformInvalidResponse, err)
	}
	body := map[string]any{
		"wallet": map[string]any{
			"external_customer_id": payload.ExternalCustomerID,
			"name":                 walletName,
			"currency":             commercial.CurrencyCNY,
			"granted_credits":      granted,
			"rate_amount":          "1",
			"expiration_at":        payload.ExpiresAt.UTC().Format(time.RFC3339),
			"metadata": map[string]string{
				commercial.WalletMetaTenant: payload.ExternalCustomerID,
				commercial.WalletMetaPeriod: payload.Period,
			},
		},
	}
	status, respBody, err := a.do(ctx, http.MethodPost, "/api/v1/wallets", body)
	if err != nil {
		return 0, nil, err
	}
	return status, respBody, nil
}

// waitForWalletSettlement polls the wallet balance until the after-commit
// settlement job has credited at least the granted amount (a1 fact). A
// timeout is the closed indeterminate unreachable — the grant replays by
// identity on the next access (the wallet exists; a blind re-POST would
// double the balance — E3).
func (a *LagoAdapter) waitForWalletSettlement(ctx context.Context, lagoID string, grantCents int64) error {
	pollCtx, cancel := context.WithTimeout(ctx, walletSettleTimeout)
	defer cancel()
	for {
		wallet, err := a.readWallet(pollCtx, lagoID)
		if err != nil {
			return err
		}
		if wallet.BalanceCents >= grantCents {
			return nil
		}
		if err := sleepCtx(pollCtx, walletSettleTick); err != nil {
			return fmt.Errorf("%w: wallet settlement not observed in time", commercial.ErrPlatformUnreachable)
		}
	}
}

// readWallet reads one wallet by provider id (seam-internal — the id never
// crosses a receipt).
func (a *LagoAdapter) readWallet(ctx context.Context, lagoID string) (lagoWallet, error) {
	status, body, err := a.do(ctx, http.MethodGet, "/api/v1/wallets/"+url.PathEscape(lagoID), nil)
	if err != nil {
		return lagoWallet{}, err
	}
	switch {
	case status >= 200 && status < 300:
	case status >= 500:
		return lagoWallet{}, fmt.Errorf("%w: wallet read unavailable", commercial.ErrPlatformUnreachable)
	default:
		return lagoWallet{}, fmt.Errorf("%w: wallet read rejected", commercial.ErrPlatformInvalidResponse)
	}
	var parsed struct {
		Wallet lagoWallet `json:"wallet"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return lagoWallet{}, fmt.Errorf("%w: wallet read malformed", commercial.ErrPlatformInvalidResponse)
	}
	return parsed.Wallet, nil
}

// walletListPageCap bounds paging over the customer wallet list (defensive:
// a runaway next_page chain can never spin the read).
const walletListPageCap = 20

// listCustomerWallets reads the customer's wallets (paged), all statuses —
// a just-expired wallet still awaiting the hourly termination tick must be
// findable by identity (replay detection), and the benefits snapshot
// filters by status itself.
func (a *LagoAdapter) listCustomerWallets(ctx context.Context, externalCustomerID string) ([]lagoWallet, error) {
	var out []lagoWallet
	for page := 1; page <= walletListPageCap; page++ {
		path := fmt.Sprintf("/api/v1/customers/%s/wallets?page=%d",
			url.PathEscape(externalCustomerID), page)
		status, body, err := a.do(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}
		switch {
		case status >= 200 && status < 300:
		case status >= 500:
			return nil, fmt.Errorf("%w: wallet list unavailable", commercial.ErrPlatformUnreachable)
		default:
			return nil, fmt.Errorf("%w: wallet list rejected", commercial.ErrPlatformInvalidResponse)
		}
		var parsed struct {
			Wallets []lagoWallet `json:"wallets"`
			Meta    struct {
				NextPage *int `json:"next_page"`
			} `json:"meta"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("%w: wallet list malformed", commercial.ErrPlatformInvalidResponse)
		}
		out = append(out, parsed.Wallets...)
		if parsed.Meta.NextPage == nil || *parsed.Meta.NextPage == 0 || len(parsed.Wallets) == 0 {
			return out, nil
		}
	}
	return out, nil
}

// readBenefitsSnapshot joins the three authority reads: subscription truth
// (explicit statuses), entitlement features, and active wallet balances.
// The answer is the RAW authority truth — registry expiry overlay is the
// coordinator's job (an expired-but-lazy-terminated wallet still reports
// its balance here).
func (a *LagoAdapter) readBenefitsSnapshot(ctx context.Context, tenantID uint64) (commercial.Snapshot, error) {
	if err := a.configured(); err != nil {
		return commercial.Snapshot{}, err
	}
	if tenantID == 0 {
		return commercial.Snapshot{}, commercial.ErrPlatformUnsupported
	}
	ctx, cancel := context.WithTimeout(ctx, subscriptionRequestTimeout)
	defer cancel()

	extCustomer := commercial.ExternalCustomerID(tenantID)
	b := &commercial.BenefitsSnapshot{
		TenantID:          tenantID,
		SubscriptionState: commercial.SubscriptionStatePending,
		CheckedAt:         time.Now().UTC(),
	}
	sub, found, err := a.readSubscriptionByIdentity(ctx, commercial.ExternalSubscriptionID(tenantID))
	if err != nil {
		return commercial.Snapshot{}, err
	}
	if found {
		if sub.Status == "active" {
			b.SubscriptionState = commercial.SubscriptionStateActive
		}
		b.PlanCode = sub.PlanCode
	}
	// Entitlements read (features keyed by code — T02 fact). A missing or
	// unparseable entitlement surface leaves Features nil — the service
	// falls back to the publication definition (documented branch).
	if features, err := a.readCustomerFeatures(ctx, extCustomer); err == nil && len(features) > 0 {
		b.Features = features
	}
	wallets, err := a.listCustomerWallets(ctx, extCustomer)
	if err != nil {
		return commercial.Snapshot{}, err
	}
	for _, w := range wallets {
		if w.Status != "active" {
			continue
		}
		b.BalanceMicro += commercial.CentsToMicro(w.BalanceCents)
		meta := w.meta()
		period := meta[commercial.WalletMetaPeriod]
		if period == "" {
			// Deterministic-name fallback: "<ext-customer>-<YYYY-MM>".
			if suffix, ok := strings.CutPrefix(w.Name, extCustomer+"-"); ok {
				if _, err := commercial.PeriodEnd(suffix); err == nil {
					period = suffix
				}
			}
		}
		if period == "" || meta[commercial.WalletMetaTenant] != "" && meta[commercial.WalletMetaTenant] != extCustomer {
			continue // a foreign (future top-up) wallet — counted in balance, not a monthly batch
		}
		expires := time.Time{}
		if t, err := time.Parse(time.RFC3339, w.ExpirationAt); err == nil {
			expires = t.UTC()
		}
		b.Batches = append(b.Batches, commercial.CreditBatchSnapshot{
			Period:       period,
			BalanceMicro: commercial.CentsToMicro(w.BalanceCents),
			ExpiresAt:    expires,
		})
	}
	return commercial.Snapshot{Kind: commercial.SnapshotKindBenefits, Benefits: b}, nil
}

// readCustomerFeatures reads the customer's attached entitlements into the
// closed feature map.
func (a *LagoAdapter) readCustomerFeatures(ctx context.Context, externalCustomerID string) (map[string]bool, error) {
	status, body, err := a.do(ctx, http.MethodGet,
		"/api/v1/customers/"+url.PathEscape(externalCustomerID)+"/entitlements", nil)
	if err != nil {
		return nil, err
	}
	switch {
	case status >= 200 && status < 300:
	case status >= 500:
		return nil, fmt.Errorf("%w: entitlement read unavailable", commercial.ErrPlatformUnreachable)
	default:
		return nil, fmt.Errorf("%w: entitlement read rejected", commercial.ErrPlatformInvalidResponse)
	}
	var parsed struct {
		Entitlements []struct {
			FeatureCode string `json:"feature_code"`
			Feature     *struct {
				Code string `json:"code"`
			} `json:"feature"`
		} `json:"entitlements"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("%w: entitlement read malformed", commercial.ErrPlatformInvalidResponse)
	}
	features := make(map[string]bool)
	for _, e := range parsed.Entitlements {
		code := e.FeatureCode
		if code == "" && e.Feature != nil {
			code = e.Feature.Code
		}
		if code != "" {
			features[code] = true
		}
	}
	return features, nil
}

// parseDecimalCentsLoose parses a "credits.cents" decimal string to cents;
// unparseable input answers 0 (callers treat 0 as unknown).
func parseDecimalCentsLoose(s string) int64 {
	whole, frac, _ := strings.Cut(s, ".")
	if len(frac) > 2 {
		frac = frac[:2]
	}
	var w, f int64
	for _, c := range whole {
		if c < '0' || c > '9' {
			return 0
		}
		w = w*10 + int64(c-'0')
	}
	for len(frac) < 2 {
		frac += "0"
	}
	for _, c := range frac {
		if c < '0' || c > '9' {
			return 0
		}
		f = f*10 + int64(c-'0')
	}
	return w*100 + f
}

// sleepCtx sleeps for d or until ctx ends; it reports ctx's cancellation.
func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
