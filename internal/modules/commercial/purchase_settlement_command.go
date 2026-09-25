package commercial

import (
	"errors"
)

// This file grows the frozen Commercial Platform seam ADDITIVELY per ADR-0014
// (the exact purchase_command.go precedent): one new CommandKind
// (settle_purchase_payment) and its typed provider-neutral payload. The
// frozen file itself gains nothing; the three method signatures never change
// and no per-object wrapper methods exist anywhere.
//
// Spec: docs/specs/2026-09-20-lago-billing-migration-design.md —
// "Quotes, payments, and fulfillment" (a verified channel result produces an
// immutable Payment Fact; a supported Lago external-payment integration
// records the Payment; only a Lago Subscription observed as active enables
// Entitlement and fulfillment; response loss is recovered with the original
// invoice, channel, and idempotency identities). Binding verdicts:
// docs/migrations/lago/t02-payment-activation/DECISION.md §5 (the
// payment-activation channel is a SUPPORTED provider integration — option ②)
// and docs/migrations/lago/t10-payment-trigger/DECISION.md (the runtime
// contract of the settle trigger on the pinned v1.53.0 stack).

// CommandKindSettlePurchasePayment settles ONE trusted channel payment fact
// onto the authority's provider rails (#82): the adapter cancels the stuck
// provider intent, promotes the configured settle payment method, and
// retriggers the collection on the gating invoice. Activation itself stays
// authority-observed: the command NEVER writes a subscription state locally
// (spec: the implementation may not force the Subscription active locally).
const CommandKindSettlePurchasePayment CommandKind = "settle_purchase_payment"

// SettlePurchasePaymentPayload is the typed payload of
// settle_purchase_payment. Both external identities are DERIVED — the
// adapter refuses any mismatch before a request leaves the seam.
// ChannelTransactionID is the channel payment number of the verified
// Payment Fact: it is NON-EMPTY and is the idempotency anchor (spec: response
// loss is recovered with the original invoice, channel, and idempotency
// identities) — a settle replays to the same receipt, never a second
// collection.
type SettlePurchasePaymentPayload struct {
	TenantID                       uint64
	ExternalCustomerID             string // must equal ExternalCustomerID(TenantID)
	ExternalPurchaseSubscriptionID string // must equal ExternalPurchaseSubscriptionID(TenantID)
	ChannelTransactionID           string // channel payment number, non-empty; the idempotency anchor
}

// Validate enforces the derived-identity equality and the non-empty channel
// transaction id (the idempotency anchor of the whole settle→grant chain).
func (p SettlePurchasePaymentPayload) Validate() error {
	if p.TenantID == 0 {
		return errors.New("invalid settle_purchase_payment payload: tenant is required")
	}
	if p.ExternalCustomerID != ExternalCustomerID(p.TenantID) {
		return errors.New("invalid settle_purchase_payment payload: external_customer_id must equal ExternalCustomerID(tenant)")
	}
	if p.ExternalPurchaseSubscriptionID != ExternalPurchaseSubscriptionID(p.TenantID) {
		return errors.New("invalid settle_purchase_payment payload: external_purchase_subscription_id must equal ExternalPurchaseSubscriptionID(tenant)")
	}
	if p.ChannelTransactionID == "" {
		return errors.New("invalid settle_purchase_payment payload: channel_transaction_id is required (idempotency anchor)")
	}
	return nil
}

// SettlePurchasePaymentCommandKey derives the seam command idempotency
// identity "settle_purchase_payment:<ext-purchase-id>:<channel-txn>" —
// deterministic per (purchase identity, channel transaction): a replayed or
// late-replayed settle (outbox replay, lost response) addresses the same
// trigger, never a second collection.
func SettlePurchasePaymentCommandKey(externalPurchaseSubscriptionID, channelTransactionID string) string {
	return string(CommandKindSettlePurchasePayment) + ":" + externalPurchaseSubscriptionID + ":" + channelTransactionID
}
