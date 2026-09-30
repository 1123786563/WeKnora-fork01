package commercial

import (
	"errors"
	"testing"
)

// Task 2 (RED): the additive settle_purchase_payment seam surface — payload
// validation, the command idempotency key, and the purchase wallet identity.
// The plan pins these tests verbatim (issue-72-plan-82.md Task 2 Step 1).

func TestSettlePurchasePaymentPayloadValidate(t *testing.T) {
	base := SettlePurchasePaymentPayload{TenantID: 7,
		ExternalCustomerID:             ExternalCustomerID(7),
		ExternalPurchaseSubscriptionID: ExternalPurchaseSubscriptionID(7),
		PlanCode:                       "pro-v1", ChannelTransaction: "2026092622001471", AmountFen: 9900, Currency: CurrencyCNY}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}
	bad := base
	bad.TenantID = 8 // identity mismatch
	if err := bad.Validate(); !errors.Is(err, ErrPlatformInvalidResponse) {
		t.Fatalf("want invalid_response, got %v", err)
	}
	zero := base
	zero.ChannelTransaction = ""
	if err := zero.Validate(); err == nil {
		t.Fatal("empty channel transaction accepted")
	}
}

func TestSettlePurchasePaymentCommandKeyStable(t *testing.T) {
	if SettlePurchasePaymentCommandKey("weknora-tenant-7-purchase", "txn1") !=
		"settle:weknora-tenant-7-purchase:txn1" {
		t.Fatal("key shape drifted")
	}
}

func TestPurchaseWalletNameDistinctFromMonthly(t *testing.T) {
	if PurchaseWalletName(7, "2026-09") == MonthlyWalletName(7, "2026-09") {
		t.Fatal("purchase wallet collides with base monthly batch")
	}
}
