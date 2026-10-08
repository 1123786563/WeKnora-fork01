// #102 / Lago 30 — the closure service over the fake authority: the
// tombstone lands first and stops every new-commercial-work entry (reserve,
// purchase, ensure — the identity is never re-ensured), the authority
// disposal terminates the subscription/wallet objects and de-identifies the
// customer, the local paid term is capped at the closure instant, and a
// platform failure leaves a converging closing state without ever touching
// the financial records.
package commercial

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/commercial/commercialplatform"
	"github.com/Tencent/WeKnora/internal/commercial/payment"
	repocommercial "github.com/Tencent/WeKnora/internal/commercial/repository/commercial"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newClosureTestEnv(t *testing.T) (*WorkspaceClosureService, *commercialplatform.FakeAdapter, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repocommercial.Subscription{}, &repocommercial.BillingAccount{},
		&repocommercial.OrderRow{}, &repocommercial.PaymentAttemptRow{},
		&repocommercial.QuoteRow{}, &repocommercial.PlanRow{}, &repocommercial.OutboxEvent{}); err != nil {
		t.Fatal(err)
	}
	fake := commercialplatform.NewFakeAdapter()
	svc, err := NewWorkspaceClosureService(db, fake)
	if err != nil {
		t.Fatal(err)
	}
	return svc, fake, db
}

// seedClosedSubscription lands an authority subscription (via the real
// ensure path), one live monthly wallet, and its local mirror with a paid
// term reaching past the closure instant.
func seedClosedSubscription(t *testing.T, fake *commercialplatform.FakeAdapter, db *gorm.DB, tenant uint64) {
	t.Helper()
	extSub := domain.ExternalSubscriptionID(tenant)
	if _, err := fake.SubmitCommand(context.Background(), domain.Command{
		Kind: domain.CommandKindEnsureSubscription,
		Key:  "ensure_subscription:" + extSub,
		Payload: domain.EnsureSubscriptionPayload{
			TenantID: tenant, ExternalCustomerID: domain.ExternalCustomerID(tenant),
			ExternalSubscriptionID: extSub, PlanCode: domain.BasePlanKey,
		},
	}); err != nil {
		t.Fatal(err)
	}
	period := "2026-11"
	end, err := domain.PeriodEnd(period)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fake.SubmitCommand(context.Background(), domain.Command{
		Kind: domain.CommandKindGrantIncludedCredits,
		Key:  domain.MonthlyGrantKey(extSub, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)),
		Payload: domain.GrantIncludedCreditsPayload{
			TenantID: tenant, ExternalCustomerID: domain.ExternalCustomerID(tenant),
			Period: period, CreditsMicro: 1_000_000,
			ExpiresAt: end, Priority: domain.MonthlyWalletPriorityFor(nil, end),
		},
	}); err != nil {
		t.Fatal(err)
	}
	subs := repocommercial.NewSubscriptionStore(db)
	if err := subs.SaveSubscription(context.Background(), &repocommercial.Subscription{
		ID: extSub, TenantID: tenant, PlanKey: domain.BasePlanKey, PlanVersion: 1,
		PlanSnapshotJSON: "{}",
		Anchor:           time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		PaidUntil:        time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
}

// TestCloseWorkspaceDisposesAuthorityAndCapsLocalTerm pins AC2: the
// authority subscription and wallet objects are terminated (the benefits
// snapshot answers the closed terminal token), the customer display name
// is de-identified, and the local paid term is capped at the closure
// instant.
func TestCloseWorkspaceDisposesAuthorityAndCapsLocalTerm(t *testing.T) {
	svc, fake, db := newClosureTestEnv(t)
	seedClosedSubscription(t, fake, db, 7)
	ctx := context.Background()

	row, err := svc.CloseWorkspace(ctx, 7, "operator", WorkspaceClosureReason)
	if err != nil {
		t.Fatal(err)
	}
	if row.State != repocommercial.WorkspaceClosureStateClosed || row.ClosedAt == nil {
		t.Fatalf("closure must confirm closed: %+v", row)
	}

	snap, err := fake.ReadSnapshot(ctx, domain.SnapshotQuery{Kind: domain.SnapshotKindBenefits, TenantID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Benefits == nil || snap.Benefits.SubscriptionState != domain.SubscriptionStateTerminated {
		t.Fatalf("benefits must answer the terminal state, got %+v", snap.Benefits)
	}
	if snap.Benefits.BalanceMicro != 0 {
		t.Fatalf("terminated wallets must not answer balances, got %d", snap.Benefits.BalanceMicro)
	}
	for _, c := range fake.Customers() {
		if c.ExternalID == "weknora-tenant-7" && c.Name != domain.DeidentifiedDisplayName("weknora-tenant-7") {
			t.Fatalf("customer display name must be de-identified, got %q", c.Name)
		}
	}

	sub, err := repocommercial.NewSubscriptionStore(db).Current(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if sub.PaidUntil.After(*row.ClosedAt) {
		t.Fatalf("local paid term must cap at the closure instant, got %v", sub.PaidUntil)
	}
	if sub.DowngradeReason != domain.DowngradeReasonWorkspaceClosed {
		t.Fatalf("closure reason must be recorded, got %q", sub.DowngradeReason)
	}

	// Replay is a no-op that answers the same closed tombstone.
	again, err := svc.CloseWorkspace(ctx, 7, "operator", WorkspaceClosureReason)
	if err != nil || again.State != repocommercial.WorkspaceClosureStateClosed || !again.ClosedAt.Equal(*row.ClosedAt) {
		t.Fatalf("closure replay must converge: %v %+v", err, again)
	}
}

// TestClosedWorkspaceRefusesNewCommercialWork pins AC1 and AC4 end to end:
// after the tombstone lands, a new purchase, a billing-account ensure (the
// identity-reuse face), and a charge reservation are all refused with the
// closed sentinel.
func TestClosedWorkspaceRefusesNewCommercialWork(t *testing.T) {
	purchaseSvc, fake, _, db := newPurchaseTestEnv(t)
	closures, err := NewWorkspaceClosureService(db, fake)
	if err != nil {
		t.Fatal(err)
	}
	purchaseSeedPlan(t, purchaseSvc.plans, "pro", 9900)
	q := purchaseQuote(t, purchaseSvc.orders, 31, "pro")
	ctx := context.Background()

	if _, err := purchaseSvc.Purchase(ctx, 31, q.ID, "wechat", "billing-admin", "WeKnora Space 31"); err != nil {
		t.Fatalf("open workspace purchase must flow: %v", err)
	}

	if _, err := closures.CloseWorkspace(ctx, 31, "operator", WorkspaceClosureReason); err != nil {
		t.Fatal(err)
	}
	if _, err := purchaseSvc.Purchase(ctx, 31, q.ID, "wechat", "billing-admin", "WeKnora Space 31"); !errors.Is(err, domain.ErrWorkspaceClosed) {
		t.Fatalf("closed workspace purchase: want ErrWorkspaceClosed, got %v", err)
	}
	if _, err := purchaseSvc.accounts.EnsureBillingAccount(ctx, 31, "WeKnora Space 31", "billing-admin"); !errors.Is(err, domain.ErrWorkspaceClosed) {
		t.Fatalf("closed identity re-ensure: want ErrWorkspaceClosed, got %v", err)
	}

	budget := repocommercial.NewBudgetStore(db)
	if err := db.AutoMigrate(&repocommercial.TaskBudgetRow{}, &repocommercial.BudgetAccountRow{},
		&repocommercial.ReservationRow{}, &repocommercial.BudgetLotRow{}, &repocommercial.BudgetLotAllocationRow{}); err != nil {
		t.Fatal(err)
	}
	end := time.Now().UTC().Add(time.Hour)
	expiry := end
	for _, row := range []any{
		&repocommercial.BudgetAccountRow{TenantID: 31, VerifiedMicro: 1_000_000, Watermark: "w1", Version: 1, VerifiedUntil: end},
		&repocommercial.TaskBudgetRow{TenantID: 31, RunID: "run-1", LimitMicro: 2_000_000, Deadline: end, Version: 1},
		&repocommercial.BudgetLotRow{TenantID: 31, LotID: "lot1", RemainingMicro: 1_000_000, ExpiresAt: &expiry, IssuedAt: time.Now().UTC()},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := budget.Reserve(ctx, domain.BudgetRequest{
		TenantID: 31, RunID: "run-1", Key: "res-1", Upper: domain.Credits(1),
		Deadline: end,
	}); !errors.Is(err, domain.ErrWorkspaceClosed) {
		t.Fatalf("closed workspace charge execution: want ErrWorkspaceClosed, got %v", err)
	}
}

// TestCloseWorkspacePlatformFailureKeepsClosingAndConverges pins the
// failure posture: the tombstone lands and stops charging even when the
// authority disposal cannot run; the closure stays in state closing and a
// replay after the outage converges to closed.
func TestCloseWorkspacePlatformFailureKeepsClosingAndConverges(t *testing.T) {
	svc, fake, db := newClosureTestEnv(t)
	seedClosedSubscription(t, fake, db, 7)
	ctx := context.Background()

	fake.FailSubmitsWith(domain.ErrPlatformUnreachable)
	row, err := svc.CloseWorkspace(ctx, 7, "operator", WorkspaceClosureReason)
	if err == nil {
		t.Fatal("a disposal failure must surface for the deletion gate")
	}
	if row.State != repocommercial.WorkspaceClosureStateClosing || row.ClosedAt != nil {
		t.Fatalf("tombstone must stay closing: %+v", row)
	}
	if !svc.WorkspaceClosed(ctx, 7) {
		t.Fatal("charging must already be stopped in state closing")
	}

	fake.FailSubmitsWith(nil)
	converged, err := svc.CloseWorkspace(ctx, 7, "operator", WorkspaceClosureReason)
	if err != nil || converged.State != repocommercial.WorkspaceClosureStateClosed {
		t.Fatalf("replay must converge to closed: %v %+v", err, converged)
	}
}

// TestCloseWorkspaceKeepsFinancialHistoryReadable pins AC3: the closure
// path never deletes or rewrites the local financial records — the order,
// the quote, and the subscription mirror stay readable after the workspace
// is closed.
func TestCloseWorkspaceKeepsFinancialHistoryReadable(t *testing.T) {
	svc, fake, db := newClosureTestEnv(t)
	seedClosedSubscription(t, fake, db, 7)
	plans, err := NewPlanVersionService(db, fake)
	if err != nil {
		t.Fatal(err)
	}
	purchaseSeedPlan(t, plans, "pro", 9900)
	ctx := context.Background()

	q := purchaseQuote(t, mustOrderService(t, db), 31, "pro")
	if err := db.Create(&repocommercial.OrderRow{
		ID: "order-31", TenantID: 31, QuoteID: q.ID,
		AmountFen: 9900, Currency: domain.CurrencyCNY, State: "awaiting_payment",
	}).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := svc.CloseWorkspace(ctx, 31, "operator", WorkspaceClosureReason); err != nil {
		t.Fatal(err)
	}
	for table, want := range map[string]int64{
		"commercial_orders":        1,
		"commercial_quotes":        1,
		"commercial_subscriptions": 1,
	} {
		var got int64
		db.Table(table).Where("tenant_id = ?", 31).Count(&got)
		if table == "commercial_subscriptions" {
			db.Table(table).Where("tenant_id = ?", 7).Count(&got)
		}
		if got != want {
			t.Fatalf("%s must survive closure, got %d rows", table, got)
		}
	}
}

func mustOrderService(t *testing.T, db *gorm.DB) *OrderService {
	t.Helper()
	orders, err := NewOrderService(db, map[string]payment.Provider{})
	if err != nil {
		t.Fatal(err)
	}
	return orders
}
