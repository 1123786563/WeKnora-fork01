package commercialplatform

import (
	"context"
	"errors"
	"fmt"
	"testing"

	commercial "github.com/Tencent/WeKnora/internal/modules/commercial"
)

// TestFakeIssueCreditNoteReplaysOneNotePerRefund (#96 / Lago 24): the
// credit-note command is idempotent per refund identity — a replayed
// submit (outbox re-delivery, worker restart) returns the SAME receipt and
// never appends a second note; the note corrects the named finalized
// invoice without rewriting it.
func TestFakeIssueCreditNoteReplaysOneNotePerRefund(t *testing.T) {
	f := NewFakeAdapter()
	payload := commercial.IssueCreditNotePayload{
		TenantID:           42,
		ExternalCustomerID: commercial.ExternalCustomerID(42),
		InvoiceExternalID:  "lago_inv_202610_0001",
		Reason:             commercial.CreditNoteReasonSubscriptionRefund,
		CreditAmountFen:    1500,
		Currency:           commercial.CurrencyCNY,
		RefundID:           "rf_000123",
	}
	cmd := commercial.Command{
		Kind:    commercial.CommandKindIssueCreditNote,
		Key:     commercial.IssueCreditNoteCommandKey(payload.RefundID),
		Actor:   "refund-service",
		Reason:  "subscription refund",
		Payload: payload,
	}

	first, err := f.SubmitCommand(context.Background(), cmd)
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	replay, err := f.SubmitCommand(context.Background(), cmd)
	if err != nil {
		t.Fatalf("replay submit: %v", err)
	}
	if first.ExternalID == "" {
		t.Fatal("receipt carries no note identity")
	}
	if first.ExternalID != replay.ExternalID || first.Key != replay.Key {
		t.Fatalf("replay diverged: %+v vs %+v", first, replay)
	}
	if got := f.CreditNoteCount(); got != 1 {
		t.Fatalf("notes=%d want 1 (history is appended, never rewritten)", got)
	}
}

// TestFakeIssueCreditNoteRefusesDivergentReplay: the same refund identity
// replayed with a DIFFERENT payload is a definitive conflict, never a
// silent second note.
func TestFakeIssueCreditNoteRefusesDivergentReplay(t *testing.T) {
	f := NewFakeAdapter()
	base := commercial.IssueCreditNotePayload{
		TenantID:           7,
		ExternalCustomerID: commercial.ExternalCustomerID(7),
		InvoiceExternalID:  "lago_inv_202610_0002",
		Reason:             commercial.CreditNoteReasonSubscriptionRefund,
		CreditAmountFen:    900,
		Currency:           commercial.CurrencyCNY,
		RefundID:           "rf_000777",
	}
	cmd := commercial.Command{
		Kind:    commercial.CommandKindIssueCreditNote,
		Key:     commercial.IssueCreditNoteCommandKey(base.RefundID),
		Actor:   "refund-service",
		Reason:  "subscription refund",
		Payload: base,
	}
	if _, err := f.SubmitCommand(context.Background(), cmd); err != nil {
		t.Fatal(err)
	}
	divergent := base
	divergent.CreditAmountFen = 999999
	cmd.Payload = divergent
	_, err := f.SubmitCommand(context.Background(), cmd)
	if err == nil {
		t.Fatal("divergent replay accepted")
	}
	if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("divergent replay error=%v want ErrPlatformInvalidResponse", err)
	}
	if got := f.CreditNoteCount(); got != 1 {
		t.Fatalf("notes=%d want 1 after refused divergent replay", got)
	}
}

func TestFakeIssueCreditNoteValidatesPayload(t *testing.T) {
	f := NewFakeAdapter()
	_, err := f.SubmitCommand(context.Background(), commercial.Command{
		Kind: commercial.CommandKindIssueCreditNote,
		Key:  commercial.IssueCreditNoteCommandKey("rf_bad"),
		Payload: commercial.IssueCreditNotePayload{
			TenantID:           1,
			ExternalCustomerID: commercial.ExternalCustomerID(1),
			// invoice/reason/amount/currency/refund all invalid or missing
		},
	})
	if err == nil {
		t.Fatal("invalid payload accepted")
	}
	if fmt.Sprint(err) == "" {
		t.Fatal("error carries no detail")
	}
}
