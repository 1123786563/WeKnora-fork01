package commercialplatform

import (
	"context"
	"errors"
	"testing"
	"time"

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

// Task 3 (#82): the fake's settle semantics — the deterministic authority
// behind the service-orchestration tests. Settle activates the incomplete
// purchase exactly like the real provider rails do (F1), replays answer the
// same receipt with zero side effects (Review Focus 2), and an absent
// purchase fails closed.
func TestFakeSettlePurchasePayment(t *testing.T) {
	f := NewFakeAdapter()
	ctx := context.Background()
	create := commercial.Command{Kind: commercial.CommandKindCreatePurchaseSubscription,
		Key: commercial.CreatePurchaseSubscriptionCommandKey(commercial.ExternalPurchaseSubscriptionID(9), "plan-p"),
		Payload: commercial.CreatePurchaseSubscriptionPayload{TenantID: 9,
			ExternalCustomerID: commercial.ExternalCustomerID(9),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(9),
			PlanCode: "plan-p", AmountFen: 9900, Currency: "CNY"}}
	if _, err := f.SubmitCommand(ctx, create); err != nil {
		t.Fatalf("create: %v", err)
	}
	settle := func(txn string) commercial.Command {
		return commercial.Command{Kind: commercial.CommandKindSettlePurchasePayment,
			Key: commercial.SettlePurchasePaymentCommandKey(commercial.ExternalPurchaseSubscriptionID(9), txn),
			Payload: commercial.SettlePurchasePaymentPayload{TenantID: 9,
				ExternalCustomerID: commercial.ExternalCustomerID(9),
				ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(9),
				ChannelTransactionID: txn}}
	}
	r1, err := f.SubmitCommand(ctx, settle("ch-txn-1"))
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if r1.ExternalID == "" {
		t.Fatalf("settle receipt must carry an external id, got %+v", r1)
	}
	snap, err := f.ReadSnapshot(ctx, commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 9})
	if err != nil || snap.Purchase.State != commercial.PurchaseStateActive {
		t.Fatalf("settle must activate, got %+v err=%v", snap.Purchase, err)
	}
	// 迟到重放（不同交易号 / 同交易号）：零副作用同回执语义（Review Focus 2）
	r1b, err := f.SubmitCommand(ctx, settle("ch-txn-1"))
	if err != nil || r1b.ExternalID != r1.ExternalID {
		t.Fatalf("same-txn replay must answer the same receipt, got %+v err=%v", r1b, err)
	}
	if _, err := f.SubmitCommand(ctx, settle("ch-txn-2")); err != nil {
		t.Fatalf("late replay must no-op, got %v", err)
	}
	if got := len(f.PurchaseSubscriptions()); got != 1 {
		t.Fatalf("replay must not create objects, got %d", got)
	}
	// 不存在的 purchase：fail closed
	missing := settle("ch-txn-3")
	missing.Payload = commercial.SettlePurchasePaymentPayload{TenantID: 42,
		ExternalCustomerID: commercial.ExternalCustomerID(42),
		ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(42),
		ChannelTransactionID: "ch-txn-3"}
	if _, err := f.SubmitCommand(ctx, missing); !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("absent purchase must fail closed, got %v", err)
	}
}

// F11 防撞（D4）：Base 月度批次与套餐首期批次同 (tenant, period) 并存——
// 独立钱包名 + 不写 period meta 键，两笔都成功且重放零冲突。
func TestFakeGrantPurchaseWalletNoMonthlyCollision(t *testing.T) {
	f := NewFakeAdapter()
	ctx := context.Background()
	end := time.Date(2099, 10, 1, 0, 0, 0, 0, time.UTC)
	period := "2099-09"
	base := commercial.Command{Kind: commercial.CommandKindGrantIncludedCredits,
		Key: commercial.GrantCreditsCommandKey(commercial.ExternalCustomerID(9), period),
		Payload: commercial.GrantIncludedCreditsPayload{TenantID: 9,
			ExternalCustomerID: commercial.ExternalCustomerID(9),
			Period: period, CreditsMicro: 1_000_000, ExpiresAt: end}}
	if _, err := f.SubmitCommand(ctx, base); err != nil {
		t.Fatalf("base grant: %v", err)
	}
	purchase := commercial.Command{Kind: commercial.CommandKindGrantIncludedCredits,
		Key: commercial.GrantCreditsCommandKey(commercial.ExternalPurchaseSubscriptionID(9), period),
		Payload: commercial.GrantIncludedCreditsPayload{TenantID: 9,
			ExternalCustomerID: commercial.ExternalCustomerID(9),
			Period: period, CreditsMicro: 9_900_000, ExpiresAt: end,
			WalletName: commercial.PurchaseWalletName(9, period)}}
	if _, err := f.SubmitCommand(ctx, purchase); err != nil {
		t.Fatalf("purchase grant must not collide with the base batch: %v", err)
	}
	wallets := f.Wallets()
	if len(wallets) != 2 {
		t.Fatalf("two wallets expected, got %+v", wallets)
	}
	// Base grant 重放不得因套餐钱包存在而 content-conflict。
	if _, err := f.SubmitCommand(ctx, base); err != nil {
		t.Fatalf("base replay must stay idempotent, got %v", err)
	}
	if got := len(f.Wallets()); got != 2 {
		t.Fatalf("replay must not create a third wallet, got %d", got)
	}
	// 回执 ExternalID 即钱包名。
	receipt, err := f.SubmitCommand(ctx, purchase)
	if err != nil || receipt.ExternalID != commercial.PurchaseWalletName(9, period) {
		t.Fatalf("purchase receipt must echo the purchase wallet name, got %+v err=%v", receipt, err)
	}
}
