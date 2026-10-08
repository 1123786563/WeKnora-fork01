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

	commercial "github.com/Tencent/WeKnora/internal/commercial"
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
//	      observed shape — requires_action, or requires_confirmation: an
//	      update-then-crash replay shape, OCR84-R1-02) carrying a non-empty
//	      metadata.lago_invoice_id; the candidate set must span EXACTLY ONE
//	      distinct invoice identity (D9 — more fails closed, never a
//	      charge), then resolves to the LATEST created (ties or unparsable
//	      created values fail closed). The same read collects the invoice
//	      identities already carried to succeeded: when the invoice the
//	      resolved candidate gates is already succeeded, the settle replays
//	      idempotently (F-1 — the residual sibling of a settled invoice is
//	      never driven, never re-keyed, never a second charge). An EMPTY
//	      candidate set fails closed (OCR84-R1-01): the settled-invoice map
//	      alone proves nothing about THIS order's gate — a previous
//	      purchase's succeeded intent would otherwise mint a fake receipt.
//	(iii) attach the configured settle pm (clone id); the customer default
//	      payment method is NEVER rewritten (D10 — confirm carries the pm
//	      explicitly, and a rewritten default would reroute renewal
//	      auto-collection onto the settlement instrument);
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
	//
	// (r2:166) Configuration gate BEFORE any provider rail call: a missing
	// settle instrument or provider credential is ErrPlatformUnconfigured,
	// not an authority rejection. An empty API key riding the Authorization
	// header would draw a provider 401 and the definitive classification
	// would park the order in attention for an operator-fixable gap.
	if a.cfg.StripeSettlePmToken == "" || a.cfg.StripeAPIKey == "" {
		return commercial.CommandReceipt{}, fmt.Errorf(
			"%w: no settle payment method or provider credential configured", commercial.ErrPlatformUnconfigured)
	}
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

	// (ii) Locate the stuck gating intent on the provider rails. ONE list
	// read answers both faces: the unsettled candidates AND the invoice
	// identities already carried to succeeded (the settle-driven window).
	intents, settledInvoices, err := a.stripeListGatingIntents(ctx, providerCustomerID)
	if err != nil {
		return commercial.CommandReceipt{}, err
	}
	if len(intents) == 0 {
		// (OCR84-R1-01) The empty-candidate branch makes NO invoice-ownership
		// inference. settledInvoices is the set of invoice ids carried by ANY
		// of this customer's succeeded intents — a previous purchase's
		// residual succeeded intent sits in it beside the settle-vs-finalize
		// window's own (t10), and the payload carries no invoice identity to
		// tell them apart (SettlePurchasePaymentPayload has no invoice id; the
		// awaiting-payment authority keeps open invoices invisible, F3-F5).
		// A bare len(settledInvoices) > 0 check would therefore mint a FAKE
		// receipt whenever the CURRENT gate was never created or was canceled
		// while any historical succeeded intent exists — masking a fail-closed
		// data anomaly as a silent no-op the fulfiller replays until D7
		// exhausts. Fail closed instead: the invoice-bound idempotency window
		// survives ONLY in its provable form below (F-1 — the unsettled
		// sibling that carries the SAME invoice id as a succeeded intent).
		// The honest narrow cost: a replay landing inside the settle-vs-
		// finalize window (confirm succeeded, response lost, webhook not yet
		// finalized) answers the definitive sentinel and parks attention —
		// recoverable by design (the event stays pending; the APPLIED record
		// overwrites the transient attention marker once the webhook lands,
		// purchase_fulfillment.go grant step).
		return commercial.CommandReceipt{}, fmt.Errorf(
			"%w: no unsettled gating intent carries the invoice identity", commercial.ErrPlatformInvalidResponse)
	}
	intent, err := latestIntent(intents)
	if err != nil {
		return commercial.CommandReceipt{}, err
	}
	// (D9 / r2:139) Distinct-invoice disambiguation gate: the unsettled
	// candidate set must span EXACTLY ONE gating invoice identity before
	// any charge-bearing call. Candidates from TWO different invoices (a
	// renewal's or a foreign purchase's stuck intent beside the current
	// gate) mean the locator cannot PROVE which invoice this settle owes —
	// the amount guard alone (1..AmountFen) would happily pass a smaller
	// foreign intent, and the latest-created heuristic would then charge
	// the WRONG invoice. Fail closed: zero writes, the attention face
	// hands the ambiguity to operations. The SAME-invoice residual sibling
	// (R-20's legal shape) is unaffected — siblings share one invoice id,
	// distinct == 1.
	if distinct := distinctInvoiceCount(intents); distinct > 1 {
		return commercial.CommandReceipt{}, fmt.Errorf(
			"%w: %d distinct gating invoices among the unsettled candidates — refusing to charge without a provable target",
			commercial.ErrPlatformInvalidResponse, distinct)
	}
	// (A-16 / F84) Amount guard, proration aware: the resolved intent's
	// amount must not EXCEED the command payload's frozen quote amount and
	// must be positive — the created-latest heuristic alone must NEVER
	// decide which PaymentIntent receives a real charge. A strictly-equal
	// check would be WRONG on the real stack: the gating PaymentIntent
	// carries the PRORATED first-period invoice amount (live 82flow
	// evidence: a 9900-fen plan gated on a 1650 intent mid-month), the same
	// "smaller than the order face, never larger, never zero" discipline
	// the D6' line-item re-check enforces. An intent LARGER than the
	// frozen quote (or a zero amount) is a wrong-object/malformed shape
	// and fails closed before any charge-bearing call.
	if intent.Amount <= 0 || intent.Amount > payload.AmountFen {
		return commercial.CommandReceipt{}, fmt.Errorf(
			"%w: gating intent amount %d is not within the frozen quote face (1..%d)",
			commercial.ErrPlatformInvalidResponse, intent.Amount, payload.AmountFen)
	}
	// (F-1 flow evidence) Invoice-bound settle idempotency: a failed 3DS
	// confirm leaves a RESIDUAL sibling PaymentIntent for the SAME invoice
	// (requires_payment_method, pm stripped) next to the one this command
	// already drove to succeeded, so "unsettled candidates remain" is NOT
	// proof the settle is undone. Re-keying "<cmd.Key>:update" onto that
	// sibling both collides on Stripe (a key is bound to its first
	// endpoint forever → 400 idempotency_error, mislabeled attention) and,
	// absent the collision, would confirm a SECOND intent for the invoice
	// — a real second charge. The INVOICE identity is the settle
	// idempotency fact: the gating invoice of the intent this drive
	// resolves is already carried to succeeded → the settle is DONE, the
	// receipt replays idempotently.
	if settledInvoices[intent.LagoInvoiceID] {
		return receipt, nil
	}

	// (iii) Attach the settle payment method. The attach rail call carries
	// a DETERMINISTIC idempotency key derived from the command key (the
	// same discipline as update/confirm): a replay of the same settle drive
	// replays the same call, so a lost response can never double-attach.
	// (D10) The customer default payment method is NEVER rewritten here —
	// see the gate below.
	attachedID, err := a.stripeAttachSettlePaymentMethod(ctx, providerCustomerID, a.cfg.StripeSettlePmToken, cmd.Key)
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

// settleRequestTimeout bounds the whole settle drive. (A-18 / F86) The
// budget covers the chain it actually runs, item by item (the
// purchaseRequestTimeout discipline): THREE authority reads
// (readPurchaseSnapshot — itself bounded by subscriptionRequestTimeout
// including its invoice index + per-invoice GETs — customerProviderBound and
// boundProviderCustomerID) plus FOUR provider round trips (intent list,
// attach, intent update, confirm — the default rewrite left the chain with
// D10), each bounded by outboundProviderTimeout. A budget below the sum
// expires mid-confirm on a slow provider, wrapping a transport error as
// unreachable and re-driving a half-completed leg (pm attached).
const settleRequestTimeout = 4*outboundProviderTimeout + 3*subscriptionRequestTimeout

// stripeIntent is one PaymentIntent row from the provider list (only the
// fields the locator needs).
type stripeIntent struct {
	ID      string
	Status  string
	Created int64
	// Amount is the intent's amount in the provider's minor units — the
	// charge-bearing face the settle must freeze against the payload's
	// frozen quote amount (A-16).
	Amount int64
	// LagoInvoiceID is metadata.lago_invoice_id — the pinned provider
	// payment-create stamps it on every gating payment (F5).
	LagoInvoiceID string
}

// stripeListPageBudget bounds the intent list pagination below (A-17): the
// correctness-critical read may be PAGED to completeness but never runaway.
const stripeListPageBudget = 10

// stripeListGatingIntents lists the customer's PaymentIntents (paged to
// completeness) and splits the answer into the two settle faces: the
// unsettled candidates (D2' step ii locator predicate) and the invoice
// identities already carried to succeeded (the invoice-bound settle
// idempotency window — F-1: the residual sibling of a settled invoice must
// never be driven). (A-17 / F85) The list answers BOTH the unsettled
// candidate location AND the settled-invoice idempotency window — a
// correctness-critical read that must never be silently truncated by a page
// boundary: has_more is consumed via starting_after until the provider
// answers a final page, bounded by stripeListPageBudget (beyond is an
// explicit fail-closed error, never a quiet cut).
func (a *LagoAdapter) stripeListGatingIntents(ctx context.Context, providerCustomerID string) ([]stripeIntent, map[string]bool, error) {
	unsettled := make([]stripeIntent, 0, 8)
	settled := make(map[string]bool)
	startingAfter := ""
	for page := 0; page < stripeListPageBudget; page++ {
		path := "/v1/payment_intents?customer=" + url.QueryEscape(providerCustomerID) + "&limit=100"
		if startingAfter != "" {
			path += "&starting_after=" + url.QueryEscape(startingAfter)
		}
		status, body, err := a.providerOutboundRequest(ctx, http.MethodGet, path, "", "")
		if err != nil {
			return nil, nil, err
		}
		if err := classifyStripeStatus(status, "payment intent list"); err != nil {
			return nil, nil, err
		}
		var parsed struct {
			Data []struct {
				ID       string `json:"id"`
				Status   string `json:"status"`
				Created  int64  `json:"created"`
				Amount   int64  `json:"amount"`
				Metadata struct {
					LagoInvoiceID string `json:"lago_invoice_id"`
				} `json:"metadata"`
			} `json:"data"`
			HasMore bool `json:"has_more"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, nil, fmt.Errorf("%w: payment intent list malformed", commercial.ErrPlatformInvalidResponse)
		}
		for _, row := range parsed.Data {
			if row.Metadata.LagoInvoiceID == "" {
				continue
			}
			if row.Status == "succeeded" {
				settled[row.Metadata.LagoInvoiceID] = true
				continue
			}
			switch row.Status {
			// (OCR84-R1-02) requires_confirmation is a REPLAY shape of this
			// drive itself: a settle that died between the pm update and the
			// confirm (process crash, budget expiry, lost response) leaves
			// the intent requires_confirmation — re-running the idempotent
			// pm update is a no-op and the confirm is exactly the missing
			// step. Skipping it here would strand the empty-candidate
			// branch's fail-closed error forever with no automatic drive.
			case "requires_payment_method", "requires_action", "requires_confirmation":
			default:
				continue // canceled/processing… are not the stuck gate
			}
			if row.Created <= 0 {
				return nil, nil, fmt.Errorf("%w: payment intent created unparsable", commercial.ErrPlatformInvalidResponse)
			}
			unsettled = append(unsettled, stripeIntent{
				ID: row.ID, Status: row.Status, Created: row.Created, Amount: row.Amount,
				LagoInvoiceID: row.Metadata.LagoInvoiceID,
			})
		}
		if !parsed.HasMore || len(parsed.Data) == 0 {
			return unsettled, settled, nil
		}
		startingAfter = parsed.Data[len(parsed.Data)-1].ID
	}
	return nil, nil, fmt.Errorf("%w: payment intent list exceeded the page budget", commercial.ErrPlatformInvalidResponse)
}

// latestIntent resolves the disambiguation rule (D2' step ii): the current
// purchase's gating intent is necessarily the MOST RECENTLY created
// candidate; a tie is ambiguous and fails closed — never a blind charge.
// Callers must first pass the D9 distinct-invoice gate: the latest-created
// heuristic only ranks intents that provably gate the SAME invoice.
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

// distinctInvoiceCount answers how many DIFFERENT gating invoice identities
// the unsettled candidate set spans (the D9 gate's input).
func distinctInvoiceCount(intents []stripeIntent) int {
	seen := make(map[string]bool, len(intents))
	for _, intent := range intents {
		seen[intent.LagoInvoiceID] = true
	}
	return len(seen)
}

// stripeAttachSettlePaymentMethod attaches the settle token to the customer
// and answers the CLONED id (the provider clones on attach — the confirm
// below must reference the clone, F6/t02 evidence). (A-18) keyBase is the
// settle command's key; the rail call derives a deterministic idempotency
// key from it (":attach") so a replay of the same drive replays the same
// call — a lost response never double-attaches.
//
// (D10 / r2:357) The customer's invoice_settings default payment method is
// NEVER rewritten on the settle leg. The confirm call carries the attached
// settle pm EXPLICITLY, so the permanent default rewrite buys nothing for
// THIS charge while silently rerouting every RENEWAL invoice's
// auto-collection onto the settlement instrument (a real monthly charge on
// the operator's card). Renewal collection routing stays whatever the
// purchase-create face set (a 3DS pm → requires_action, fail-closed — never
// an automatic charge); the renewal routing question itself is a #84 /
// production-review agenda item, deliberately NOT decided here.
func (a *LagoAdapter) stripeAttachSettlePaymentMethod(ctx context.Context, providerCustomerID, pmToken, keyBase string) (string, error) {
	attachStatus, attachBody, err := a.providerOutboundRequest(ctx, http.MethodPost,
		"/v1/payment_methods/"+url.PathEscape(pmToken)+"/attach",
		"customer="+url.QueryEscape(providerCustomerID), keyBase+":attach")
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
	// (A-19 / F87) A 2xx whose body dies mid-stream (connection reset) is a
	// TRANSIENT transport failure — surfacing it as a truncated JSON would
	// misclassify downstream as malformed (the definitive invalid-response
	// sentinel) and park the order instead of retrying. A body that exceeds
	// the cap is a definitive oversized-response error, never a quiet cut
	// that only later fails the JSON parse.
	limited := io.LimitReader(resp.Body, maxProviderBodyBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: outbound provider response body read failed", commercial.ErrPlatformUnreachable)
	}
	if int64(len(data)) > maxProviderBodyBytes {
		return 0, nil, fmt.Errorf("%w: provider response body exceeds %d bytes", commercial.ErrPlatformInvalidResponse, maxProviderBodyBytes)
	}
	return resp.StatusCode, data, nil
}

// maxProviderBodyBytes caps one provider response body (64 KiB — an intent
// list page or a single object fits far below; anything larger is not a
// shape this seam ever consumes).
const maxProviderBodyBytes = 64 << 10

// boundProviderCustomerID reads the provider_customer_id off the
// authority's customer binding (the Stripe cus_… identity).
// (D13 / r2:445) The status split matches customerProviderBound and every
// other read path: 429/5xx is the RETRYABLE unreachable (the fulfiller
// keeps the event pending), anything else non-200 is the definitive
// invalid_response.
func (a *LagoAdapter) boundProviderCustomerID(ctx context.Context, externalCustomerID string) (string, error) {
	status, body, err := a.do(ctx, http.MethodGet,
		"/api/v1/customers/"+url.PathEscape(externalCustomerID), nil)
	if err != nil {
		return "", err
	}
	switch {
	case status >= 200 && status < 300:
	case status == http.StatusTooManyRequests || status >= 500:
		return "", fmt.Errorf("%w: binding read unavailable (HTTP %d)", commercial.ErrPlatformUnreachable, status)
	default:
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
