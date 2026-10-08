package commercialplatform

import (
	"context"
	"errors"
	"testing"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
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

// --- #82 Task 3: the fake's deterministic settle semantics (D2'(a)(b)(c))
// and the purchase-wallet grant identity (D4). ---

func newSettleCommand(tenant uint64, planCode, txn string, amount int64) commercial.Command {
	return commercial.Command{
		Kind:  commercial.CommandKindSettlePurchasePayment,
		Key:   commercial.SettlePurchasePaymentCommandKey(commercial.ExternalPurchaseSubscriptionID(tenant), txn),
		Actor: "test", Reason: "settle",
		Payload: commercial.SettlePurchasePaymentPayload{
			TenantID: tenant, ExternalCustomerID: commercial.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(tenant),
			PlanCode:                       planCode, ChannelTransaction: txn, AmountFen: amount, Currency: commercial.CurrencyCNY,
		},
	}
}

func TestFakeSettleActivatesOnceThenIdempotent(t *testing.T) { // D2'(a)(b)
	f := NewFakeAdapter()
	if _, err := f.SubmitCommand(context.Background(), newPurchaseCommand(21, "weknora-pro-v1", 9900)); err != nil {
		t.Fatal(err)
	}
	cmd := newSettleCommand(21, "weknora-pro-v1", "2026092622001471", 9900)
	first, err := f.SubmitCommand(context.Background(), cmd)
	if err != nil {
		t.Fatalf("settle #1: %v", err)
	}
	snap, err := f.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 21})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Purchase.State != commercial.PurchaseStateActive {
		t.Fatalf("settle must activate the purchase, got %q", snap.Purchase.State)
	}
	second, err := f.SubmitCommand(context.Background(), cmd)
	if err != nil {
		t.Fatalf("settle #2 (same key): %v", err)
	}
	if first.Key != second.Key || !first.RecordedAt.Equal(second.RecordedAt) {
		t.Fatalf("replayed settle must return the SAME receipt, got %+v then %+v", first, second)
	}
	snap, _ = f.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 21})
	if snap.Purchase.State != commercial.PurchaseStateActive {
		t.Fatalf("replayed settle must not change state, got %q", snap.Purchase.State)
	}
	if n := len(f.PurchaseSubscriptions()); n != 1 {
		t.Fatalf("settle replay must never mint a second purchase, got %d", n)
	}
}

func TestFakeSettleUnknownSubscriptionFailsClosed(t *testing.T) { // (c)
	f := NewFakeAdapter()
	_, err := f.SubmitCommand(context.Background(), newSettleCommand(22, "weknora-pro-v1", "txn-x", 9900))
	if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("settle on an unknown purchase must fail closed, got %v", err)
	}
}

func TestFakeGrantPurchaseWalletNoMonthlyCollision(t *testing.T) { // (d)
	f := NewFakeAdapter()
	tenant := uint64(23)
	// ponytail: dynamic current month — a hardcoded period expires the month after (2026-10 failure mode)
	period := commercial.MonthlyPeriod(time.Now().UTC())
	monthly := commercial.GrantIncludedCreditsPayload{
		TenantID: tenant, ExternalCustomerID: commercial.ExternalCustomerID(tenant),
		Period: period, CreditsMicro: 100_00_00, // 1.00 credit in micro
		ExpiresAt: periodEndOrFatal(t, period),
		Priority:  commercial.MonthlyWalletPriority,
	}
	purchase := monthly
	purchase.WalletName = commercial.PurchaseWalletName(tenant, period)
	if _, err := f.SubmitCommand(context.Background(), commercial.Command{
		Kind:  commercial.CommandKindGrantIncludedCredits,
		Key:   commercial.GrantCreditsCommandKey(commercial.ExternalCustomerID(tenant), period),
		Actor: "test", Reason: "base", Payload: monthly,
	}); err != nil {
		t.Fatalf("monthly grant: %v", err)
	}
	if _, err := f.SubmitCommand(context.Background(), commercial.Command{
		Kind:  commercial.CommandKindGrantIncludedCredits,
		Key:   commercial.SettlePurchasePaymentCommandKey(commercial.ExternalPurchaseSubscriptionID(tenant), "txn-1") + ":grant",
		Actor: "test", Reason: "purchase", Payload: purchase,
	}); err != nil {
		t.Fatalf("purchase grant (own wallet name): %v", err)
	}
	wallets := f.Wallets()
	if len(wallets) != 2 {
		t.Fatalf("two grants with distinct wallet names must persist TWO wallets, got %d", len(wallets))
	}
	// (F-4) BOTH batches answer the benefits face for the same period —
	// the purchase wallet joins the monthly batch, never hidden.
	snap, err := f.ReadSnapshot(context.Background(), commercial.SnapshotQuery{
		Kind: commercial.SnapshotKindBenefits, TenantID: tenant,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Benefits.Batches) != 2 {
		t.Fatalf("the monthly AND purchase batches must both answer, got %+v", snap.Benefits.Batches)
	}
	for _, batch := range snap.Benefits.Batches {
		if batch.Period != period {
			t.Fatalf("both batches belong to %s, got %+v", period, snap.Benefits.Batches)
		}
	}
	if snap.Benefits.BalanceMicro != monthly.CreditsMicro+purchase.CreditsMicro {
		t.Fatalf("balance must carry both wallets, got %d", snap.Benefits.BalanceMicro)
	}
}

func periodEndOrFatal(t *testing.T, period string) (end time.Time) {
	t.Helper()
	end, err := commercial.PeriodEnd(period)
	if err != nil {
		t.Fatalf("period end: %v", err)
	}
	return end
}
