package commercialplatform

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/modules/commercial"
)

// settlePurchasePayment drives the α dual-track settlement leg (#82, D2' —
// t11 P-A..P-E verified on the pinned v1.53.0): after a VERIFIED channel
// payment fact, WeKnora resolves the stuck gating PaymentIntent on the
// PROVIDER rails, attaches the configured settle payment method, and
// confirms the charge off-session. The activation itself is finalized by
// the authority's BUILT-IN webhook chain (payment_intent.succeeded →
// PaymentIntentSucceededService → invoice finalized + subscription active);
// this command performs NO Lago state write of any kind (spec L125).
//
// Algorithm (D2'):
//
//	(i)   read the purchase snapshot: already active → idempotent receipt,
//	      zero outbound calls (a late replay never re-charges);
//	(ii)  GET /v1/payment_intents?customer={pcid} — candidates are intents
//	      in an UNSETTLED shape (requires_payment_method — t10/t11's only
//	      observed shape — or requires_action) carrying a non-empty
//	      metadata.lago_invoice_id; more than one candidate resolves to the
//	      LATEST created (the current purchase's gating intent is always the
//	      most recent); ties or unparsable created values fail closed;
//	(iii) attach the configured settle pm (clone id) and set it as the
//	      customer default;
//	(iv)  POST /v1/payment_intents/{pi} (payment_method=…) then
//	      POST /v1/payment_intents/{pi}/confirm → succeeded (off-session
//	      synchronous charge; both calls carry Idempotency-Key = cmd.Key);
//	(v)   return the receipt — observation of active is the fulfiller's
//	      job (Task 7), never a wait here.
//
// Error taxonomy: provider 429/5xx → ErrPlatformUnreachable (retryable);
// other non-2xx → ErrPlatformInvalidResponse; a missing settle pm token →
// ErrPlatformUnconfigured (fail closed, never a guessed instrument).
func (a *LagoAdapter) settlePurchasePayment(ctx context.Context, cmd commercial.Command) (commercial.CommandReceipt, error) {
	payload, ok := cmd.Payload.(commercial.SettlePurchasePaymentPayload)
	if !ok {
		return commercial.CommandReceipt{}, commercial.ErrPlatformUnsupported
	}
	if err := payload.Validate(); err != nil {
		return commercial.CommandReceipt{}, fmt.Errorf("%w: %v", commercial.ErrPlatformInvalidResponse, err)
	}
	if err := a.configured(); err != nil {
		return commercial.CommandReceipt{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, settleRequestTimeout)
	defer cancel()

	// (i) The authority truth first: an active purchase makes the whole
	// command an idempotent no-op (Review-Focus 2: late replays and outbox
	// re-deliveries after a lost response must not re-charge).
	snap, err := a.readPurchaseSnapshot(ctx, payload.TenantID)
	if err != nil {
		return commercial.CommandReceipt{}, err
	}
	receipt := commercial.CommandReceipt{
		Key:        cmd.Key,
		ExternalID: payload.ExternalPurchaseSubscriptionID,
		RecordedAt: time.Now().UTC(),
	}
	if snap.Purchase.State == commercial.PurchaseStateActive {
		return receipt, nil
	}
	if snap.Purchase.State != commercial.PurchaseStateAwaitingPayment {
		// canceled/absent: nothing to settle — the purchase is closed or was
		// never created; definitive, never a charge.
		return commercial.CommandReceipt{}, fmt.Errorf(
			"%w: settle target purchase is %s, not awaiting payment",
			commercial.ErrPlatformInvalidResponse, snap.Purchase.State)
	}

	// The provider-side customer identity (the Stripe cus_…) comes from the
	// authority's binding read — never a guess.
	_, bound, err := a.customerProviderBound(ctx, payload.ExternalCustomerID)
	if err != nil {
		return commercial.CommandReceipt{}, err
	}
	if !bound {
		return commercial.CommandReceipt{}, fmt.Errorf(
			"%w: settle target customer carries no provider binding", commercial.ErrPlatformInvalidResponse)
	}
	providerCustomerID, err := a.boundProviderCustomerID(ctx, payload.ExternalCustomerID)
	if err != nil {
		return commercial.CommandReceipt{}, err
	}

	// (ii) Locate the stuck gating intent on the provider rails.
	intents, err := a.stripeListUnsettledIntents(ctx, providerCustomerID)
	if err != nil {
		return commercial.CommandReceipt{}, err
	}
	if len(intents) == 0 {
		return commercial.CommandReceipt{}, fmt.Errorf(
			"%w: no unsettled gating intent carries the invoice identity", commercial.ErrPlatformInvalidResponse)
	}
	intent, err := latestIntent(intents)
	if err != nil {
		return commercial.CommandReceipt{}, err
	}

	// (iii) Attach the settle payment method and promote it to default.
	// An unconfigured token fails closed BEFORE any charge-bearing call.
	if a.cfg.StripeSettlePmToken == "" {
		return commercial.CommandReceipt{}, fmt.Errorf(
			"%w: no settle payment method configured", commercial.ErrPlatformUnconfigured)
	}
	attachedID, err := a.stripeAttachSettlePaymentMethod(ctx, providerCustomerID, a.cfg.StripeSettlePmToken)
	if err != nil {
		return commercial.CommandReceipt{}, err
	}

	// (iv) Update the intent's payment method, then confirm off-session.
	// The provider idempotency identities DERIVE from the command key with
	// per-endpoint suffixes: Stripe binds a key to its first request's
	// endpoint forever (one key across update AND confirm answers 400
	// idempotency_error — t9 integration evidence), so each rail call keys
	// as "<cmd.Key>:update" / "<cmd.Key>:confirm" (still deterministic per
	// settle drive: a replay of the same command replays the same pair).
	updateStatus, _, err := a.providerOutboundRequest(ctx, http.MethodPost,
		"/v1/payment_intents/"+url.PathEscape(intent.ID),
		"payment_method="+url.QueryEscape(attachedID), cmd.Key+":update")
	if err != nil {
		return commercial.CommandReceipt{}, err
	}
	if err := classifyStripeStatus(updateStatus, "payment intent update"); err != nil {
		return commercial.CommandReceipt{}, err
	}
	confirmStatus, confirmBody, err := a.providerOutboundRequest(ctx, http.MethodPost,
		"/v1/payment_intents/"+url.PathEscape(intent.ID)+"/confirm", "", cmd.Key+":confirm")
	if err != nil {
		return commercial.CommandReceipt{}, err
	}
	if err := classifyStripeStatus(confirmStatus, "payment intent confirm"); err != nil {
		return commercial.CommandReceipt{}, err
	}
	var confirmed struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(confirmBody, &confirmed) != nil || confirmed.Status != "succeeded" {
		return commercial.CommandReceipt{}, fmt.Errorf(
			"%w: settle confirm did not reach succeeded (status=%q)",
			commercial.ErrPlatformInvalidResponse, confirmed.Status)
	}

	// (v) Activation observation belongs to the fulfiller (Task 7).
	return receipt, nil
}

// settleRequestTimeout bounds the whole settle drive (five outbound calls).
const settleRequestTimeout = outboundProviderTimeout * 4

// stripeIntent is one PaymentIntent row from the provider list (only the
// fields the locator needs).
type stripeIntent struct {
	ID      string
	Status  string
	Created int64
	// LagoInvoiceID is metadata.lago_invoice_id — the pinned provider
	// payment-create stamps it on every gating payment (F5).
	LagoInvoiceID string
}

// stripeListUnsettledIntents lists the customer's PaymentIntents and keeps
// the unsettled ones carrying the Lago invoice identity (D2' step ii
// locator predicate).
func (a *LagoAdapter) stripeListUnsettledIntents(ctx context.Context, providerCustomerID string) ([]stripeIntent, error) {
	status, body, err := a.providerOutboundRequest(ctx, http.MethodGet,
		"/v1/payment_intents?customer="+url.QueryEscape(providerCustomerID)+"&limit=20", "", "")
	if err != nil {
		return nil, err
	}
	if err := classifyStripeStatus(status, "payment intent list"); err != nil {
		return nil, err
	}
	var parsed struct {
		Data []struct {
			ID       string `json:"id"`
			Status   string `json:"status"`
			Created  int64  `json:"created"`
			Metadata struct {
				LagoInvoiceID string `json:"lago_invoice_id"`
			} `json:"metadata"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("%w: payment intent list malformed", commercial.ErrPlatformInvalidResponse)
	}
	out := make([]stripeIntent, 0, len(parsed.Data))
	for _, row := range parsed.Data {
		switch row.Status {
		case "requires_payment_method", "requires_action":
		default:
			continue // succeeded/canceled/processing… are not the stuck gate
		}
		if row.Metadata.LagoInvoiceID == "" {
			continue
		}
		if row.Created <= 0 {
			return nil, fmt.Errorf("%w: payment intent created unparsable", commercial.ErrPlatformInvalidResponse)
		}
		out = append(out, stripeIntent{
			ID: row.ID, Status: row.Status, Created: row.Created,
			LagoInvoiceID: row.Metadata.LagoInvoiceID,
		})
	}
	return out, nil
}

// latestIntent resolves the disambiguation rule (D2' step ii): the current
// purchase's gating intent is necessarily the MOST RECENTLY created
// candidate; a tie is ambiguous and fails closed — never a blind charge.
func latestIntent(intents []stripeIntent) (stripeIntent, error) {
	best := intents[0]
	tied := false
	for _, candidate := range intents[1:] {
		switch {
		case candidate.Created > best.Created:
			best = candidate
			tied = false
		case candidate.Created == best.Created:
			tied = true
		}
	}
	if tied {
		return stripeIntent{}, fmt.Errorf("%w: multiple gating intents share the same created timestamp", commercial.ErrPlatformInvalidResponse)
	}
	return best, nil
}

// stripeAttachSettlePaymentMethod attaches the settle token and promotes
// the CLONED id to the customer default (the provider clones on attach —
// the default update must reference the clone, F6/t02 evidence).
func (a *LagoAdapter) stripeAttachSettlePaymentMethod(ctx context.Context, providerCustomerID, pmToken string) (string, error) {
	attachStatus, attachBody, err := a.providerOutboundRequest(ctx, http.MethodPost,
		"/v1/payment_methods/"+url.PathEscape(pmToken)+"/attach",
		"customer="+url.QueryEscape(providerCustomerID), "")
	if err != nil {
		return "", err
	}
	if err := classifyStripeStatus(attachStatus, "settle payment method attach"); err != nil {
		return "", err
	}
	var attached struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(attachBody, &attached) != nil || attached.ID == "" {
		return "", fmt.Errorf("%w: settle payment method attach malformed", commercial.ErrPlatformInvalidResponse)
	}
	defaultStatus, _, err := a.providerOutboundRequest(ctx, http.MethodPost,
		"/v1/customers/"+url.PathEscape(providerCustomerID),
		"invoice_settings[default_payment_method]="+url.QueryEscape(attached.ID), "")
	if err != nil {
		return "", err
	}
	if err := classifyStripeStatus(defaultStatus, "settle default payment method"); err != nil {
		return "", err
	}
	return attached.ID, nil
}

// classifyStripeStatus maps a provider HTTP status onto the seam's closed
// error taxonomy (GC: 429/5xx unreachable — retryable; other non-2xx
// invalid response).
func classifyStripeStatus(status int, what string) error {
	switch {
	case status >= 200 && status < 300:
		return nil
	case status == http.StatusTooManyRequests || status >= 500:
		return fmt.Errorf("%w: provider %s unavailable (HTTP %d)", commercial.ErrPlatformUnreachable, what, status)
	default:
		return fmt.Errorf("%w: provider %s rejected (HTTP %d)", commercial.ErrPlatformInvalidResponse, what, status)
	}
}

// providerOutboundRequest issues ONE pre-validated provider call with an
// arbitrary method (the settle rail needs GET in addition to POST). The
// POST-only providerOutboundCall is a thin wrapper over it; the credential
// rides ONLY the Authorization header and the S1 host check always runs.
func (a *LagoAdapter) providerOutboundRequest(ctx context.Context, method, path, form, idempotencyKey string) (int, []byte, error) {
	base := a.cfg.StripeAPIBase
	if base == "" {
		base = outboundProviderHost
	}
	if err := validateOutboundHostWithBypass(base, a.cfg.OutboundAllowLoopback); err != nil {
		return 0, nil, fmt.Errorf("%w: outbound host policy violation", commercial.ErrPlatformUnconfigured)
	}
	var bodyReader io.Reader
	if form != "" {
		bodyReader = strings.NewReader(form)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimSuffix(base, "/")+path, bodyReader)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: outbound request invalid", commercial.ErrPlatformUnconfigured)
	}
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(a.cfg.StripeAPIKey+":")))
	if form != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := providerOutboundClient().Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: outbound provider call not reachable", commercial.ErrPlatformUnreachable)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	return resp.StatusCode, data, nil
}

// boundProviderCustomerID reads the provider_customer_id off the
// authority's customer binding (the Stripe cus_… identity).
func (a *LagoAdapter) boundProviderCustomerID(ctx context.Context, externalCustomerID string) (string, error) {
	status, body, err := a.do(ctx, http.MethodGet,
		"/api/v1/customers/"+url.PathEscape(externalCustomerID), nil)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("%w: binding read answered HTTP %d", commercial.ErrPlatformInvalidResponse, status)
	}
	var parsed struct {
		Customer struct {
			BillingConfiguration struct {
				ProviderCustomerID string `json:"provider_customer_id"`
			} `json:"billing_configuration"`
		} `json:"customer"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil ||
		parsed.Customer.BillingConfiguration.ProviderCustomerID == "" {
		return "", fmt.Errorf("%w: binding read malformed", commercial.ErrPlatformInvalidResponse)
	}
	return parsed.Customer.BillingConfiguration.ProviderCustomerID, nil
}
