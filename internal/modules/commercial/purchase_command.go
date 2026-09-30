package commercial

import (
	"errors"
	"time"
)

// This file grows the frozen Commercial Platform seam ADDITIVELY per ADR-0014
// (the exact plan_command.go / subscription_command.go precedent): one new
// CommandKind (create_purchase_subscription), its typed provider-neutral
// payload, and one new SnapshotKind (purchase) for the #81 payment-gated
// purchase path. The frozen file itself gains ONLY the additive
// Snapshot.Purchase field; the three method signatures never change and no
// per-object wrapper methods exist anywhere.
//
// Spec: docs/specs/2026-09-20-lago-billing-migration-design.md —
// "Quotes, payments, and fulfillment" (initial subscriptions are pay-in-
// advance, have no trial, and use a payment activation rule; they remain
// incomplete until the gating payment succeeds) and "Public product states"
// (awaiting payment). Binding lab verdicts: docs/migrations/lago/
// t02-payment-activation/DECISION.md (gated creation semantics F1-F10) and
// the 2026-09-23 user ruling: the payment-activation channel is a SUPPORTED
// Lago payment provider integration (option (b) of T02 §5).

// CommandKindCreatePurchaseSubscription idempotently creates the tenant's
// payment-gated PURCHASE subscription under the deterministic purchase
// identity (#81, Lago T09). Unlike ensure_subscription (#80, the free Base
// Plan standard subscription), this command carries a payment activation
// gate: the authority creates the subscription as incomplete alongside an
// open gating invoice, and only a successful provider payment activates it
// (that activation is #82/#83 — never this command).
const CommandKindCreatePurchaseSubscription CommandKind = "create_purchase_subscription"

// ExternalPurchaseSubscriptionID is THE deterministic WeKnora→authority
// purchase-subscription identity (#81, design decision D1): a pure function
// of the tenant ID only. The paid family (first purchase, upgrade,
// downgrade, fallback) shares ONE purchase identity for spec subscription
// continuity; it is deliberately distinct from ExternalSubscriptionID
// ("…-sub", the #80 free Base Plan), so the Base Plan never terminates and
// the space keeps its Base fallback entitlements while the purchase awaits
// payment.
func ExternalPurchaseSubscriptionID(tenantID uint64) string {
	return ExternalCustomerID(tenantID) + "-purchase"
}

// CreatePurchaseSubscriptionPayload is the typed payload of
// create_purchase_subscription. Both external identities are DERIVED — the
// adapter refuses any mismatch before a request leaves the seam. AmountFen
// is the plan version's frozen integer minor-unit price (> 0: a paid
// purchase; the free Base Plan path is ensure_subscription) and Currency is
// the closed token CNY (first slice, spec: integer minor units, CNY only).
type CreatePurchaseSubscriptionPayload struct {
	TenantID                       uint64
	ExternalCustomerID             string // must equal ExternalCustomerID(TenantID)
	ExternalPurchaseSubscriptionID string // must equal ExternalPurchaseSubscriptionID(TenantID)
	PlanCode                       string // the plan version's deterministic code (seam-internal)
	AmountFen                      int64  // > 0; the frozen price in integer fen
	Currency                       string // closed token "CNY"
}

// Validate enforces the derived-identity equality, a plan code, a positive
// integer amount, and the closed CNY token.
func (p CreatePurchaseSubscriptionPayload) Validate() error {
	if p.TenantID == 0 {
		return errors.New("invalid create_purchase payload: tenant is required")
	}
	if p.ExternalCustomerID != ExternalCustomerID(p.TenantID) {
		return errors.New("invalid create_purchase payload: external_customer_id must equal ExternalCustomerID(tenant)")
	}
	if p.ExternalPurchaseSubscriptionID != ExternalPurchaseSubscriptionID(p.TenantID) {
		return errors.New("invalid create_purchase payload: external_purchase_subscription_id must equal ExternalPurchaseSubscriptionID(tenant)")
	}
	if p.PlanCode == "" {
		return errors.New("invalid create_purchase payload: plan_code is required")
	}
	if p.AmountFen <= 0 {
		return errors.New("invalid create_purchase payload: amount_fen must be positive")
	}
	if p.Currency != CurrencyCNY {
		return errors.New("invalid create_purchase payload: currency must be the closed token CNY")
	}
	return nil
}

// CreatePurchaseSubscriptionCommandKey derives the seam command idempotency
// identity "create_purchase_subscription:<ext-purchase-id>:<plan-code>" —
// deterministic per (purchase identity, plan version): a replayed create
// addresses the same purchase, never a second subscription or a second
// gating invoice (the provider re-POST risk, t02 contract note F7).
func CreatePurchaseSubscriptionCommandKey(externalPurchaseSubscriptionID, planCode string) string {
	return string(CommandKindCreatePurchaseSubscription) + ":" + externalPurchaseSubscriptionID + ":" + planCode
}

// SnapshotKindPurchase reads the billing authority's purchase truth for one
// tenant: the payment-gated subscription state and its frozen price face —
// the data source for the quote-match gate that must pass BEFORE a channel
// payment order is created (spec L121; W5 additive kind #81).
const SnapshotKindPurchase SnapshotKind = "purchase"

// PurchaseState is the closed authority-truth enum for the purchase
// snapshot. awaiting_payment maps the provider's incomplete state; this is
// the seam-internal truth, not the Billing API token itself.
const (
	// PurchaseStateAbsent: the authority definitively holds no purchase
	// subscription for the tenant.
	PurchaseStateAbsent = "absent"
	// PurchaseStateAwaitingPayment: the gated subscription exists and awaits
	// its gating payment (provider state incomplete) — product state
	// 「待付款」, entitlements NOT open.
	PurchaseStateAwaitingPayment = "awaiting_payment"
	// PurchaseStateActive: the gating payment succeeded and the purchase is
	// active.
	PurchaseStateActive = "active"
	// PurchaseStateCanceled: the purchase was canceled (payment failed,
	// timeout, or explicit cancel).
	PurchaseStateCanceled = "canceled"
	// PurchaseStatePaidAwaitingActivation is a COORDINATOR-COMPOSED product
	// state (design decision D3, #82): the local order is paid but the
	// authority snapshot has not yet been observed active. It is NEVER
	// produced by readPurchaseSnapshot / any authority read — only the
	// service layer synthesizes it (local paid order + authority
	// awaiting_payment). Constant lives here next to the authority-truth set
	// so the wire vocabulary stays in ONE place; the seam's observed set is
	// unchanged.
	PurchaseStatePaidAwaitingActivation = "paid_awaiting_activation"
)

// InvoiceLineSnapshot is one invoice line inside a purchase snapshot
// (design decision D2, user-approved option A, mandatory condition 3: the
// explicit #82/#84 interface for full line-item re-checks at payment time).
// During the awaiting_payment stage this is ALWAYS empty — the pinned
// authority version keeps open invoices API-invisible (t09 evidence F3-F5),
// and an invisible line is never fabricated. After finalization (payment
// time) the adapter fills it authoritatively from the visible invoice fees.
type InvoiceLineSnapshot struct {
	Kind      string // closed "subscription_fee"
	Name      string
	AmountFen int64
}

// PurchaseSnapshot is the authority-side purchase section of a Snapshot: the
// payment-gated subscription truth for one tenant at one check time —
// PlanCode/AmountFen/Currency are the authoritative price face the
// PurchaseService hard-checks against the Quote before creating any channel
// payment request.
type PurchaseSnapshot struct {
	TenantID    uint64
	State       string // PurchaseState* closed set
	PlanCode    string
	AmountFen   int64
	Currency    string
	InvoiceFees []InvoiceLineSnapshot // open stage: always empty; finalized: authoritative (D2 condition 3)
	// InvoicePaymentStatus is the finalized gating invoice's payment_status
	// as the authority reports it (#82 D6' review input: "succeeded" is part
	// of the fulfillment re-check). Empty outside the finalized stage.
	InvoicePaymentStatus string
	CheckedAt            time.Time
}
