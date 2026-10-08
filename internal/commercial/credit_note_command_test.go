package commercial

import (
	"strings"
	"testing"
)

func validCreditNotePayload() IssueCreditNotePayload {
	return IssueCreditNotePayload{
		TenantID:           42,
		ExternalCustomerID: ExternalCustomerID(42),
		InvoiceExternalID:  "lago_inv_202610_0001",
		Reason:             CreditNoteReasonSubscriptionRefund,
		CreditAmountFen:    1500,
		Currency:           CurrencyCNY,
		RefundID:           "rf_000123",
	}
}

func TestIssueCreditNotePayloadValidate(t *testing.T) {
	if err := validCreditNotePayload().Validate(); err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(p *IssueCreditNotePayload)
		want   string
	}{
		{"missing tenant", func(p *IssueCreditNotePayload) { p.TenantID = 0 }, "tenant is required"},
		{"identity mismatch", func(p *IssueCreditNotePayload) { p.ExternalCustomerID = "other-customer" }, "identity mismatch"},
		{"missing invoice", func(p *IssueCreditNotePayload) { p.InvoiceExternalID = "" }, "invoice_external_id is required"},
		{"unknown reason", func(p *IssueCreditNotePayload) { p.Reason = "goodwill" }, "reason"},
		{"zero amount", func(p *IssueCreditNotePayload) { p.CreditAmountFen = 0 }, "credit_amount_fen must be positive"},
		{"negative amount", func(p *IssueCreditNotePayload) { p.CreditAmountFen = -1 }, "credit_amount_fen must be positive"},
		{"wrong currency", func(p *IssueCreditNotePayload) { p.Currency = "USD" }, "currency must be the closed token CNY"},
		{"missing refund", func(p *IssueCreditNotePayload) { p.RefundID = "" }, "refund_id is required"},
	}
	for _, tc := range cases {
		p := validCreditNotePayload()
		tc.mutate(&p)
		err := p.Validate()
		if err == nil {
			t.Fatalf("%s: invalid payload accepted", tc.name)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: error %q does not mention %q", tc.name, err.Error(), tc.want)
		}
	}
}

func TestIssueCreditNoteCommandKeyStable(t *testing.T) {
	if got, want := IssueCreditNoteCommandKey("rf_000123"), "issue_credit_note:rf_000123"; got != want {
		t.Fatalf("key=%q want %q", got, want)
	}
	if IssueCreditNoteCommandKey("rf_a") == IssueCreditNoteCommandKey("rf_b") {
		t.Fatal("distinct refunds must derive distinct keys")
	}
}
