package commercial

import (
	"errors"
	"fmt"
)

// This file grows the frozen Commercial Platform seam ADDITIVELY per
// ADR-0014 (the exact purchase_settlement_command.go precedent): one new
// CommandKind (issue_credit_note) and its typed provider-neutral payload
// for the #96 (Lago 24) plan-refund correction chain. No frozen signature
// changes; provider vocabulary stays inside commercialplatform.

// CommandKindIssueCreditNote corrects a FINALIZED invoice by issuing a
// credit note — the only legal correction form (spec Lago 24: a finalized
// invoice is never rewritten; refunds adjust through credit notes or a
// later bill). The command is append-only: it records the correction, it
// never mutates the invoice or the settlement history behind it.
const CommandKindIssueCreditNote CommandKind = "issue_credit_note"

// CreditNoteReasonSubscriptionRefund is the single closed reason token of
// the credit-note command today (#96): a plan-subscription refund. New
// reasons join this closed set only with a matching spec line.
const CreditNoteReasonSubscriptionRefund = "subscription_refund"

// IssueCreditNotePayload is the typed payload of issue_credit_note.
// InvoiceExternalID anchors the correction to ONE finalized invoice;
// RefundID is the stable WeKnora refund identity the idempotency key
// derives from — one refund issues at most one note, ever.
type IssueCreditNotePayload struct {
	TenantID           uint64
	ExternalCustomerID string // must equal ExternalCustomerID(TenantID)
	InvoiceExternalID  string // the finalized invoice being corrected
	Reason             string // closed set: subscription_refund
	CreditAmountFen    int64  // the credit amount (> 0, integer fen)
	Currency           string // closed token "CNY"
	RefundID           string // the stable refund identity
}

// Validate enforces the derived-identity equality (a mismatch is a
// definitive integrity violation), the invoice anchor, the closed reason
// token, a positive integer amount, the closed currency token, and the
// refund identity.
func (p IssueCreditNotePayload) Validate() error {
	if p.TenantID == 0 {
		return errors.New("invalid issue_credit_note payload: tenant is required")
	}
	if p.ExternalCustomerID != ExternalCustomerID(p.TenantID) {
		return fmt.Errorf("%w: invalid issue_credit_note payload: derived identity mismatch for tenant %d",
			ErrPlatformInvalidResponse, p.TenantID)
	}
	if p.InvoiceExternalID == "" {
		return errors.New("invalid issue_credit_note payload: invoice_external_id is required")
	}
	if p.Reason != CreditNoteReasonSubscriptionRefund {
		return errors.New("invalid issue_credit_note payload: reason must be the closed token subscription_refund")
	}
	if p.CreditAmountFen <= 0 {
		return errors.New("invalid issue_credit_note payload: credit_amount_fen must be positive")
	}
	if p.Currency != CurrencyCNY {
		return errors.New("invalid issue_credit_note payload: currency must be the closed token CNY")
	}
	if p.RefundID == "" {
		return errors.New("invalid issue_credit_note payload: refund_id is required")
	}
	return nil
}

// IssueCreditNoteCommandKey derives the seam command idempotency identity
// "issue_credit_note:<refund-id>" — deterministic per refund: outbox
// re-deliveries and worker restarts of the SAME refund address the same
// note; two different refunds are two different keys. A replay under this
// key with a DIVERGENT payload is a definitive conflict, never a second
// note.
func IssueCreditNoteCommandKey(refundID string) string {
	return string(CommandKindIssueCreditNote) + ":" + refundID
}
