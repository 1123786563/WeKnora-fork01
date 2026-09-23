package commercialplatform

import (
	"context"
	"errors"
	"testing"

	commercial "github.com/Tencent/WeKnora/internal/modules/commercial"
)

func newPurchaseCommand(tenant uint64, planCode string, amount int64) commercial.Command {
	return commercial.Command{
		Kind:  commercial.CommandKindCreatePurchaseSubscription,
		Key:   commercial.CreatePurchaseSubscriptionCommandKey(commercial.ExternalPurchaseSubscriptionID(tenant), planCode),
		Actor: "test", Reason: "purchase",
		Payload: commercial.CreatePurchaseSubscriptionPayload{
			TenantID: tenant, ExternalCustomerID: commercial.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(tenant),
			PlanCode:                       planCode, AmountFen: amount, Currency: commercial.CurrencyCNY,
		},
	}
}

func TestFakeCreatePurchaseSubscriptionIdempotent(t *testing.T) {
	f := NewFakeAdapter()
	cmd := newPurchaseCommand(11, "weknora-pro-v1", 9900)
	first, err := f.SubmitCommand(context.Background(), cmd)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if first.ExternalID != commercial.ExternalPurchaseSubscriptionID(11) {
		t.Fatalf("receipt identity = %q", first.ExternalID)
	}
	if _, err := f.SubmitCommand(context.Background(), cmd); err != nil {
		t.Fatalf("replay create: %v", err)
	}
	subs := f.PurchaseSubscriptions()
	if len(subs) != 1 {
		t.Fatalf("replay must not create a second subscription, got %d", len(subs))
	}
	if subs[0].Status != "incomplete" {
		t.Fatalf("fresh purchase subscription must be incomplete, got %q", subs[0].Status)
	}
}

func TestFakeCreatePurchaseConflictOnDifferentPlan(t *testing.T) {
	f := NewFakeAdapter()
	if _, err := f.SubmitCommand(context.Background(), newPurchaseCommand(12, "weknora-pro-v1", 9900)); err != nil {
		t.Fatal(err)
	}
	_, err := f.SubmitCommand(context.Background(), newPurchaseCommand(12, "weknora-max-v1", 19900))
	if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("concurrent plan change must be a definitive conflict, got %v", err)
	}
	if n := len(f.PurchaseSubscriptions()); n != 1 {
		t.Fatalf("conflict must not create a second subscription, got %d", n)
	}
}

func TestFakePurchaseSnapshotStates(t *testing.T) {
	f := NewFakeAdapter()
	snap, err := f.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 13})
	if err != nil || snap.Purchase == nil || snap.Purchase.State != commercial.PurchaseStateAbsent {
		t.Fatalf("untouched tenant must answer absent, got %+v err=%v", snap.Purchase, err)
	}
	if _, err := f.SubmitCommand(context.Background(), newPurchaseCommand(13, "weknora-pro-v1", 9900)); err != nil {
		t.Fatal(err)
	}
	snap, err = f.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 13})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Purchase.State != commercial.PurchaseStateAwaitingPayment ||
		snap.Purchase.PlanCode != "weknora-pro-v1" ||
		snap.Purchase.AmountFen != 9900 ||
		snap.Purchase.Currency != commercial.CurrencyCNY {
		t.Fatalf("awaiting-payment snapshot mismatch: %+v", snap.Purchase)
	}
	f.ActivatePurchase(commercial.ExternalPurchaseSubscriptionID(13))
	snap, _ = f.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 13})
	if snap.Purchase.State != commercial.PurchaseStateActive {
		t.Fatalf("activated purchase must answer active, got %+v", snap.Purchase)
	}
}
