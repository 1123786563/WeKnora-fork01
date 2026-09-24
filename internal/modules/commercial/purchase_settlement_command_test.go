package commercial

import "testing"

// Task 2 (#82): the settle_purchase_payment seam command — provider-neutral
// addenda per ADR-0014. The exact plan-specified tests.

func TestSettlePurchasePaymentPayloadValidate(t *testing.T) {
	good := SettlePurchasePaymentPayload{
		TenantID: 7, ExternalCustomerID: ExternalCustomerID(7),
		ExternalPurchaseSubscriptionID: ExternalPurchaseSubscriptionID(7),
		ChannelTransactionID:           "txn-1",
	}
	if err := good.Validate(); err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}
	wrongIdentity := good
	wrongIdentity.ExternalCustomerID = ExternalCustomerID(8)
	if err := wrongIdentity.Validate(); err == nil {
		t.Fatal("mismatched customer identity must be refused")
	}
	noTxn := good
	noTxn.ChannelTransactionID = ""
	if err := noTxn.Validate(); err == nil {
		t.Fatal("empty channel transaction id must be refused (idempotency anchor)")
	}
}

func TestSettlePurchasePaymentCommandKey(t *testing.T) {
	got := SettlePurchasePaymentCommandKey(ExternalPurchaseSubscriptionID(7), "txn-1")
	if got != "settle_purchase_payment:weknora-tenant-7-purchase:txn-1" {
		t.Fatalf("unexpected key %q", got)
	}
}

func TestPurchaseWalletName(t *testing.T) {
	if got, want := PurchaseWalletName(7, "2026-09"), "weknora-tenant-7-purchase-2026-09"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
