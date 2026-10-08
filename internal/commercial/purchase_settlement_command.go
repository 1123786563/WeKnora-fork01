package commercial

import (
	"errors"
	"fmt"
)

// This file grows the frozen Commercial Platform seam ADDITIVELY per
// ADR-0014 (the exact purchase_command.go precedent): one new CommandKind
// (settle_purchase_payment) and its typed provider-neutral payload for the
// #82 R-4 dual-track activation chain (channel-collected payment + WeKnora
// drives the Stripe Provider gated settle charge -> Lago built-in webhook
// finalize -> active). No frozen signature changes; provider vocabulary
// stays inside commercialplatform.

// CommandKindSettlePurchasePayment drives the settle rail of the α
// dual-track (R-4 ruling, 2026-09-26): after a VERIFIED channel payment
// fact lands, WeKnora drives the authority's own provider rails — locate
// the stuck gating PaymentIntent provider-side, attach the settle payment
// method, and confirm the charge off-session. The activation itself is
// ALWAYS finalized by the authority's built-in webhook chain; this command
// never writes any Lago state directly (spec L125: only a supported Lago
// external-payment integration records the Payment — the provider does, not
// WeKnora).
const CommandKindSettlePurchasePayment CommandKind = "settle_purchase_payment"

// SettlePurchasePaymentPayload is the typed payload of
// settle_purchase_payment. ChannelTransaction is the verified channel
// payment fact's transaction id (the Alipay trade no) — the idempotency
// anchor binding ONE settle drive to ONE channel collection (spec L165:
// durable outbox + stable idempotency identity; L126: recovery with the
// original channel identity).
type SettlePurchasePaymentPayload struct {
	TenantID                       uint64
	ExternalCustomerID             string // must equal ExternalCustomerID(TenantID)
	ExternalPurchaseSubscriptionID string // must equal ExternalPurchaseSubscriptionID(TenantID)
	PlanCode                       string // the purchase plan version's code
	ChannelTransaction             string // the verified channel payment fact's transaction id
	AmountFen                      int64  // the frozen integer minor-unit price (> 0)
	Currency                       string // closed token "CNY"
}

// Validate enforces the derived-identity equality (a mismatch is a
// definitive integrity violation — ErrPlatformInvalidResponse), the plan
// code, the channel transaction, a positive integer amount, and the closed
// CNY token.
func (p SettlePurchasePaymentPayload) Validate() error {
	if p.TenantID == 0 {
		return errors.New("invalid settle_purchase payload: tenant is required")
	}
	if p.ExternalCustomerID != ExternalCustomerID(p.TenantID) ||
		p.ExternalPurchaseSubscriptionID != ExternalPurchaseSubscriptionID(p.TenantID) {
		return fmt.Errorf("%w: invalid settle_purchase payload: derived identity mismatch for tenant %d",
			ErrPlatformInvalidResponse, p.TenantID)
	}
	if p.PlanCode == "" {
		return errors.New("invalid settle_purchase payload: plan_code is required")
	}
	if p.ChannelTransaction == "" {
		return errors.New("invalid settle_purchase payload: channel_transaction is required")
	}
	if p.AmountFen <= 0 {
		return errors.New("invalid settle_purchase payload: amount_fen must be positive")
	}
	if p.Currency != CurrencyCNY {
		return errors.New("invalid settle_purchase payload: currency must be the closed token CNY")
	}
	return nil
}

// SettlePurchasePaymentCommandKey derives the seam command idempotency
// identity "settle:<ext-purchase-id>:<channel-transaction>" — deterministic
// per (purchase identity, channel collection): replays and outbox
// re-deliveries of the SAME verified channel payment address the same
// settle drive; a second, different channel transaction (a duplicate
// over-payment) is a DIFFERENT key, and the adapter's already-active
// short-circuit keeps it from ever charging again.
func SettlePurchasePaymentCommandKey(externalPurchaseSubscriptionID, channelTransaction string) string {
	return "settle:" + externalPurchaseSubscriptionID + ":" + channelTransaction
}
