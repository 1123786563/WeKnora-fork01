package commercial

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	"github.com/Tencent/WeKnora/internal/modules/commercial/commercialplatform"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newBenefitsService builds the T08 chain over shared-cache in-memory
// SQLite (the single-writer pool convention) with the fake adapter as the
// authority and an injectable clock.
func newBenefitsService(t *testing.T, platform domain.CommercialPlatform) (*BenefitsService, *FakeBroker, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&repocommercial.BillingAccount{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS tenants (
		id INTEGER PRIMARY KEY, name TEXT, storage_used INTEGER NOT NULL DEFAULT 0
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS tenant_members (
		id INTEGER PRIMARY KEY AUTOINCREMENT, user_id TEXT, tenant_id INTEGER NOT NULL,
		status TEXT NOT NULL DEFAULT 'active', deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS commercial_resource_counters (
		tenant_id INTEGER NOT NULL, resource TEXT NOT NULL, used INTEGER NOT NULL DEFAULT 0,
		hard_limit INTEGER NULL, PRIMARY KEY (tenant_id, resource)
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	accounts, err := NewBillingAccountService(db, platform)
	if err != nil {
		t.Fatal(err)
	}
	plans, err := NewPlanVersionService(db, platform)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := NewBenefitsService(db, accounts, plans, platform, repocommercial.NewBudgetStore(db))
	if err != nil {
		t.Fatal(err)
	}
	return svc, &FakeBroker{DB: db}, db
}

// FakeBroker is the test observation surface over the raw tables (counter
// and registry assertions).
type FakeBroker struct{ DB *gorm.DB }

func (b *FakeBroker) counter(tenantID uint64, resource string) (used, hardLimit int64, hasLimit bool) {
	var row struct {
		Used      *int64
		HardLimit *int64
	}
	if err := b.DB.Raw(`SELECT used, hard_limit FROM commercial_resource_counters
		WHERE tenant_id = ? AND resource = ?`, tenantID, resource).Scan(&row).Error; err != nil {
		return 0, 0, false
	}
	if row.Used != nil {
		used = *row.Used
	}
	if row.HardLimit != nil {
		hardLimit, hasLimit = *row.HardLimit, true
	}
	return
}

func (b *FakeBroker) batchCount(tenantID uint64) int64 {
	var n int64
	b.DB.Raw(`SELECT COUNT(*) FROM commercial_credit_batches WHERE tenant_id = ?`, tenantID).Scan(&n)
	return n
}

func (b *FakeBroker) insertMember(t *testing.T, tenantID uint64, userID string) {
	t.Helper()
	if err := b.DB.Exec(`INSERT INTO tenant_members (user_id, tenant_id, status) VALUES (?, ?, 'active')`,
		userID, tenantID).Error; err != nil {
		t.Fatal(err)
	}
}

const benefitsTenant = uint64(501)

// septemberClock anchors the mid-month instant of the CURRENT month: the
// domain grant validator compares expires_at against real time.Now, so a
// fixed past calendar month rots the suite once its period end passes.
func septemberClock() func() time.Time {
	y, m, _ := time.Now().UTC().Date()
	return func() time.Time { return time.Date(y, m, 15, 10, 0, 0, 0, time.UTC) }
}

// septemberPeriod derives the current month's "YYYY-MM" from the same
// dynamic anchor.
func septemberPeriod() string {
	return domain.MonthlyPeriod(septemberClock()())
}

// TestEnsureBenefitsHappyChain: fresh tenant → the whole lazy chain lands —
// account linked, exactly one (base,1) publication, exactly one
// subscription, one period batch granted with exactly one month's balance,
// projection row with the base plan's features/limits, quotas applied.
func TestEnsureBenefitsHappyChain(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true, "advanced_models": false, "priority_support": false})
	svc, broker, db := newBenefitsService(t, fake)
	svc.SetNow(septemberClock())

	status, err := svc.EnsureBenefits(context.Background(), benefitsTenant, "Chain Space", "user-1")
	if err != nil {
		t.Fatalf("EnsureBenefits: %v", err)
	}
	if status.Account.State != BillingAccountLinked || status.Reason != "" {
		t.Fatalf("account must be linked with an empty reason, got %+v", status.Account)
	}
	if status.Plan == nil || status.Plan.Key != domain.BasePlanKey || status.Plan.Version != 1 ||
		status.Plan.State != domain.SubscriptionStateActive {
		t.Fatalf("plan view = %+v", status.Plan)
	}
	if !status.Plan.Features["api_access"] || status.Plan.Features["advanced_models"] {
		t.Fatalf("features = %+v", status.Plan.Features)
	}
	if status.Plan.Limits["members"] != 5 || status.Plan.Limits["storage_gb"] != 10 || status.Plan.Limits["concurrent_tasks"] != 2 {
		t.Fatalf("limits = %+v", status.Plan.Limits)
	}
	if status.Credits == nil || status.Credits.BalanceMicro != BasePlanSeedIncludedCreditsMicro {
		t.Fatalf("credits = %+v", status.Credits)
	}
	if len(status.Credits.Batches) != 1 || status.Credits.Batches[0].Period != septemberPeriod() ||
		status.Credits.Batches[0].BalanceMicro != BasePlanSeedIncludedCreditsMicro {
		t.Fatalf("batches = %+v", status.Credits.Batches)
	}
	// Exactly one publication, one subscription, one wallet, one batch row.
	var pubs int64
	db.Raw(`SELECT COUNT(*) FROM commercial_plan_publications WHERE plan_key = 'base'`).Scan(&pubs)
	if pubs != 1 {
		t.Fatalf("exactly one (base,1) publication expected, got %d", pubs)
	}
	if subs := fake.Subscriptions(); len(subs) != 1 || subs[0].PlanCode != domain.DeterministicPlanCode(domain.BasePlanKey, 1) {
		t.Fatalf("subscriptions = %+v", subs)
	}
	if wallets := fake.Wallets(); len(wallets) != 1 || wallets[0].BalanceCents != BasePlanSeedIncludedCreditsMicro/10_000 {
		t.Fatalf("wallets = %+v", wallets)
	}
	if n := broker.batchCount(benefitsTenant); n != 1 {
		t.Fatalf("one batch row expected, got %d", n)
	}
	// Quotas applied from the projection's limits.
	if used, limit, has := broker.counter(benefitsTenant, "members"); !has || limit != 5 || used != 0 {
		t.Fatalf("members counter = (%d, %d, has=%v)", used, limit, has)
	}
	_, storageLimit, hasStorage := broker.counter(benefitsTenant, "storage_gb")
	if !hasStorage || storageLimit != 10*(1<<30) {
		t.Fatalf("storage_gb hard_limit = %d (has=%v), want 10×2^30", storageLimit, hasStorage)
	}
	if _, taskLimit, hasTasks := broker.counter(benefitsTenant, "concurrent_tasks"); !hasTasks || taskLimit != 2 {
		t.Fatalf("concurrent_tasks counter missing/wrong")
	}
}

// (F-4 flow evidence) The purchase first-period batch joins the credits
// view: the PurchaseFulfiller's grant rides its OWN wallet identity (D4)
// and bypasses this coordinator's registry — the activation month's batch
// line must aggregate BOTH wallets' balances (SUM, never an arbitrary
// last-write overwrite) and the total must carry the purchase credits.
func TestEnsureBenefitsPurchaseBatchJoinsCreditsView(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true})
	svc, _, _ := newBenefitsService(t, fake)
	svc.SetNow(septemberClock())
	tenant := uint64(502)
	if _, err := svc.EnsureBenefits(context.Background(), tenant, "Purchase Space", "user-1"); err != nil {
		t.Fatalf("EnsureBenefits: %v", err)
	}
	// The fulfiller's purchase grant for the activation month: own key
	// family, own deterministic wallet name (the PurchaseFulfiller path).
	period := septemberPeriod()
	purchaseMicro := int64(9_900_000)
	end, err := domain.PeriodEnd(period)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fake.SubmitCommand(context.Background(), domain.Command{
		Kind:  domain.CommandKindGrantIncludedCredits,
		Key:   domain.SettlePurchasePaymentCommandKey(domain.ExternalPurchaseSubscriptionID(tenant), "txn-1") + ":grant",
		Actor: "fulfiller", Reason: "purchase first period",
		Payload: domain.GrantIncludedCreditsPayload{
			TenantID: tenant, ExternalCustomerID: domain.ExternalCustomerID(tenant),
			Period: period, CreditsMicro: purchaseMicro, ExpiresAt: end,
			WalletName: domain.PurchaseWalletName(tenant, period),
			Priority:   domain.MonthlyWalletPriority,
		},
	}); err != nil {
		t.Fatalf("purchase grant: %v", err)
	}
	status, err := svc.EnsureBenefits(context.Background(), tenant, "Purchase Space", "user-1")
	if err != nil {
		t.Fatalf("re-ensure: %v", err)
	}
	if status.Credits == nil {
		t.Fatalf("credits view must answer, got nil")
	}
	want := BasePlanSeedIncludedCreditsMicro + purchaseMicro
	if status.Credits.BalanceMicro != want {
		t.Fatalf("balance must carry base + purchase, got %d want %d", status.Credits.BalanceMicro, want)
	}
	found := false
	for _, batch := range status.Credits.Batches {
		if batch.Period == period {
			found = true
			if batch.BalanceMicro != want {
				t.Fatalf("the activation month's line must aggregate BOTH wallets, got %d want %d", batch.BalanceMicro, want)
			}
		}
	}
	if !found {
		t.Fatalf("the activation month's batch must be present, got %+v", status.Credits.Batches)
	}
}

// (F-4) A purchase batch whose activation month has NO registry row (the
// tenant never visited billing that month — the purchase grant rides the
// fulfiller, not this coordinator) still answers the view honestly from
// the authority snapshot.
func TestRefreshProjectionUnionsSnapshotOnlyPurchasePeriod(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	svc, _, _ := newBenefitsService(t, fake)
	svc.SetNow(septemberClock())
	tenant := uint64(503)
	period := septemberPeriod()
	purchaseMicro := int64(9_900_000)
	end, err := domain.PeriodEnd(period)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fake.SubmitCommand(context.Background(), domain.Command{
		Kind:  domain.CommandKindGrantIncludedCredits,
		Key:   domain.SettlePurchasePaymentCommandKey(domain.ExternalPurchaseSubscriptionID(tenant), "txn-1") + ":grant",
		Actor: "fulfiller", Reason: "purchase first period",
		Payload: domain.GrantIncludedCreditsPayload{
			TenantID: tenant, ExternalCustomerID: domain.ExternalCustomerID(tenant),
			Period: period, CreditsMicro: purchaseMicro, ExpiresAt: end,
			WalletName: domain.PurchaseWalletName(tenant, period),
			Priority:   domain.MonthlyWalletPriority,
		},
	}); err != nil {
		t.Fatalf("purchase grant: %v", err)
	}
	_, batches, _, err := svc.refreshAndCollect(context.Background(), tenant)
	if err != nil {
		t.Fatalf("refreshAndCollect: %v", err)
	}
	found := false
	for _, batch := range batches {
		if batch.Period == period {
			found = true
			if batch.BalanceMicro != purchaseMicro {
				t.Fatalf("the snapshot-only purchase batch must answer its balance, got %+v", batches)
			}
		}
	}
	if !found {
		t.Fatalf("a registry-less purchase period must join the view, got %+v", batches)
	}
}

// (F-2' value semantics) The definitions are the VALUE truth: entitlement
// existence alone would over-claim a base-only tenant (the publish leg
// attaches entitlements for false-valued codes too on the pinned
// authority). A base-only tenant keeps advanced_models FALSE; an ACTIVE
// purchase ORs its plan's definition on top — the purchaser's
// advanced_models answers TRUE.
func TestBenefitsFeaturesPurchaseFaceORsPurchasePlanDefinition(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true, "advanced_models": false, "priority_support": false})
	svc, _, db := newBenefitsService(t, fake)
	svc.SetNow(septemberClock())
	// Publish the pro plan locally (definition + publication rows).
	proDef := domain.PlanVersion{Key: "pro", Version: 1, Price: 99_00, Monthly: 9_900_000,
		Features: map[string]bool{"advanced_models": true}, Currency: domain.CurrencyCNY}
	proJSON, err := json.Marshal(proDef)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&repocommercial.PlanRow{PlanKey: "pro", Version: 1,
		DefinitionJSON: string(proJSON), State: domain.PlanStatePublished}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&repocommercial.PublicationRow{
		CommandKey: domain.PublishCommandKey("pro", 1), PlanKey: "pro", Version: 1,
		PlanCode: domain.DeterministicPlanCode("pro", 1), ReceiptJSON: "{}",
		PublishedBy: "test", PublishedAt: time.Now().UTC(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	tenant := uint64(504)
	// Base-only first: advanced_models must stay FALSE.
	status, err := svc.EnsureBenefits(ctx, tenant, "OR Space", "user-1")
	if err != nil {
		t.Fatalf("EnsureBenefits (base-only): %v", err)
	}
	if status.Plan == nil || status.Plan.Features["advanced_models"] {
		t.Fatalf("base-only tenant must keep advanced_models FALSE, got %+v", status.Plan)
	}
	if !status.Plan.Features["api_access"] {
		t.Fatalf("base features must answer, got %+v", status.Plan.Features)
	}
	// The purchase chain on the fake: create + settle → ACTIVE.
	if _, err := fake.SubmitCommand(ctx, domain.Command{
		Kind:  domain.CommandKindCreatePurchaseSubscription,
		Key:   domain.CreatePurchaseSubscriptionCommandKey(domain.ExternalPurchaseSubscriptionID(tenant), domain.DeterministicPlanCode("pro", 1)),
		Actor: "test", Reason: "purchase",
		Payload: domain.CreatePurchaseSubscriptionPayload{
			TenantID: tenant, ExternalCustomerID: domain.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: domain.ExternalPurchaseSubscriptionID(tenant),
			PlanCode:                       domain.DeterministicPlanCode("pro", 1), AmountFen: 99_00, Currency: domain.CurrencyCNY,
		},
	}); err != nil {
		t.Fatalf("purchase create: %v", err)
	}
	if _, err := fake.SubmitCommand(ctx, domain.Command{
		Kind:  domain.CommandKindSettlePurchasePayment,
		Key:   domain.SettlePurchasePaymentCommandKey(domain.ExternalPurchaseSubscriptionID(tenant), "txn-or-1"),
		Actor: "test", Reason: "settle",
		Payload: domain.SettlePurchasePaymentPayload{
			TenantID: tenant, ExternalCustomerID: domain.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: domain.ExternalPurchaseSubscriptionID(tenant),
			PlanCode:                       domain.DeterministicPlanCode("pro", 1),
			ChannelTransaction:             "txn-or-1", AmountFen: 99_00, Currency: domain.CurrencyCNY,
		},
	}); err != nil {
		t.Fatalf("settle: %v", err)
	}
	// The purchaser's face: advanced_models TRUE (the purchase plan's
	// definition ORs on top), api_access still TRUE.
	status2, err := svc.EnsureBenefits(ctx, tenant, "OR Space", "user-1")
	if err != nil {
		t.Fatalf("EnsureBenefits (purchaser): %v", err)
	}
	if status2.Plan == nil || !status2.Plan.Features["advanced_models"] {
		t.Fatalf("an active purchase must OR advanced_models TRUE, got %+v", status2.Plan.Features)
	}
	if !status2.Plan.Features["api_access"] || status2.Plan.Features["priority_support"] {
		t.Fatalf("the base booleans must hold otherwise, got %+v", status2.Plan.Features)
	}
}

// TestEnsureBenefitsIdempotentReplay: three runs — one subscription, one
// wallet, ONE balance (not tripled — the E3 anti-pattern), one batch row,
// one publication; the third run answers the same plan view.
func TestEnsureBenefitsIdempotentReplay(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true})
	svc, broker, db := newBenefitsService(t, fake)
	svc.SetNow(septemberClock())
	var last BenefitsStatus
	for i := 0; i < 3; i++ {
		status, err := svc.EnsureBenefits(context.Background(), benefitsTenant, "Replay Space", "user-1")
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		last = status
	}
	if subs := fake.Subscriptions(); len(subs) != 1 {
		t.Fatalf("subscription count = %d, want 1", len(subs))
	}
	wallets := fake.Wallets()
	if len(wallets) != 1 || wallets[0].BalanceCents != BasePlanSeedIncludedCreditsMicro/10_000 {
		t.Fatalf("wallet balance after replays = %+v (must be exactly one month)", wallets)
	}
	if n := broker.batchCount(benefitsTenant); n != 1 {
		t.Fatalf("batch rows = %d, want 1", n)
	}
	var pubs int64
	db.Raw(`SELECT COUNT(*) FROM commercial_plan_publications WHERE plan_key = 'base'`).Scan(&pubs)
	if pubs != 1 {
		t.Fatalf("publications = %d, want 1", pubs)
	}
	if last.Plan == nil || last.Plan.Key != domain.BasePlanKey || last.Credits == nil ||
		last.Credits.BalanceMicro != BasePlanSeedIncludedCreditsMicro {
		t.Fatalf("third-run view = %+v %+v", last.Plan, last.Credits)
	}
}

// TestEnsureBenefitsConcurrentExactlyOnce: 8 goroutines racing the chain on
// one tenant — exactly 1 subscription, 1 wallet, 1 batch row, 1
// publication; the total granted micro is EXACTLY one month.
func TestEnsureBenefitsConcurrentExactlyOnce(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true})
	svc, broker, db := newBenefitsService(t, fake)
	svc.SetNow(septemberClock())
	const racers = 8
	started := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-started
			if _, err := svc.EnsureBenefits(context.Background(), benefitsTenant, "Race Space", "user-1"); err != nil {
				t.Errorf("racer: %v", err)
			}
		}()
	}
	close(started)
	wg.Wait()
	if subs := fake.Subscriptions(); len(subs) != 1 {
		t.Fatalf("subscription count after race = %d, want 1", len(subs))
	}
	var total int64
	for _, w := range fake.Wallets() {
		total += w.GrantedCents
	}
	if total != BasePlanSeedIncludedCreditsMicro/10_000 {
		t.Fatalf("total granted cents = %d, want exactly one month (%d)", total, BasePlanSeedIncludedCreditsMicro/10_000)
	}
	if len(fake.Wallets()) != 1 {
		t.Fatalf("wallet count after race = %d, want 1", len(fake.Wallets()))
	}
	if n := broker.batchCount(benefitsTenant); n != 1 {
		t.Fatalf("batch rows after race = %d, want 1", n)
	}
	var pubs int64
	db.Raw(`SELECT COUNT(*) FROM commercial_plan_publications WHERE plan_key = 'base'`).Scan(&pubs)
	if pubs != 1 {
		t.Fatalf("publications after race = %d, want 1", pubs)
	}
}

// TestMonthlyGrantNewPeriod: a month boundary mints the SECOND short-TTL
// wallet for the new period (the ≤2-transient-slots budget); the old batch
// stays in the registry and surfaces as expired/zero once past its
// ExpiresAt — the lazy-termination overlay.
func TestMonthlyGrantNewPeriod(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true})
	svc, broker, _ := newBenefitsService(t, fake)
	clock := septemberClock()()
	period := domain.MonthlyPeriod(clock)
	svc.SetNow(func() time.Time { return clock })

	if _, err := svc.EnsureBenefits(context.Background(), benefitsTenant, "Period Space", "user-1"); err != nil {
		t.Fatal(err)
	}
	// Cross into the next month: the month's batch (end = next month's 1st)
	// is expired.
	clock = clock.AddDate(0, 1, 0)
	status, err := svc.EnsureBenefits(context.Background(), benefitsTenant, "Period Space", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if wallets := fake.Wallets(); len(wallets) != 2 {
		t.Fatalf("wallet count = %d, want 2 (one per period)", len(wallets))
	}
	if status.Credits == nil || status.Credits.BalanceMicro != BasePlanSeedIncludedCreditsMicro {
		t.Fatalf("balance must be the NEW period only, got %+v", status.Credits)
	}
	byPeriod := map[string]int64{}
	for _, b := range status.Credits.Batches {
		byPeriod[b.Period] = b.BalanceMicro
	}
	if byPeriod[period] != 0 {
		t.Fatalf("the expired %s batch must surface zero, got %d", period, byPeriod[period])
	}
	if byPeriod[domain.MonthlyPeriod(clock)] != BasePlanSeedIncludedCreditsMicro {
		t.Fatalf("the next-month batch must carry one month, got %d", byPeriod[domain.MonthlyPeriod(clock)])
	}
	if n := broker.batchCount(benefitsTenant); n != 2 {
		t.Fatalf("registry must keep both periods, got %d", n)
	}
}

// walletPriorityByName answers the observable priority of one wallet (nil
// when absent) — the #86 consumption-order assertion helper.
func walletPriorityByName(t *testing.T, fake *commercialplatform.FakeAdapter, name string) (int, bool) {
	t.Helper()
	for _, w := range fake.Wallets() {
		if w.Name == name {
			return w.Priority, true
		}
	}
	return 0, false
}

// TestRefreshSyncsLotsFromSnapshot (#86 Task 3): one benefits refresh
// projects the authority batch read-back onto commercial_budget_lots — both
// the monthly-family and the top-up batch land as lot rows with the correct
// balances (the allocation order's comparison keys ride the same rows).
func TestRefreshSyncsLotsFromSnapshot(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true})
	svc, _, db := newBenefitsService(t, fake)
	if err := db.AutoMigrate(&repocommercial.BudgetLotRow{}); err != nil {
		t.Fatal(err)
	}
	svc.SetNow(septemberClock())
	tenant := uint64(507)
	ctx := context.Background()

	if _, err := svc.EnsureBenefits(ctx, tenant, "Lot Space", "user-1"); err != nil {
		t.Fatal(err)
	}
	// A top-up batch beside the monthly one.
	ext := domain.ExternalCustomerID(tenant)
	clock := septemberClock()()
	fake.SeedTopUpWallet(ext+"-topup-x", ext, 5_000,
		clock.AddDate(0, 6, 0),
		time.Now().UTC().AddDate(0, 0, -1), domain.TopUpWalletPriority)
	if _, err := svc.EnsureBenefits(ctx, tenant, "Lot Space", "user-1"); err != nil {
		t.Fatal(err)
	}
	type lotRow struct {
		LotID          string
		RemainingMicro int64
	}
	var lots []lotRow
	if err := db.Raw(`SELECT lot_id, remaining_micro FROM commercial_budget_lots WHERE tenant_id = ? ORDER BY lot_id`, tenant).Scan(&lots).Error; err != nil {
		t.Fatal(err)
	}
	byID := map[string]int64{}
	for _, l := range lots {
		byID[l.LotID] = l.RemainingMicro
	}
	monthly := domain.MonthlyWalletName(tenant, septemberPeriod())
	if got, ok := byID[monthly]; !ok || got != BasePlanSeedIncludedCreditsMicro {
		t.Fatalf("monthly lot row missing/wrong: %+v", byID)
	}
	if got, ok := byID[ext+"-topup-x"]; !ok || got != domain.CentsToMicro(5_000) {
		t.Fatalf("topup lot row missing/wrong: %+v", byID)
	}
	if len(byID) != 2 {
		t.Fatalf("exactly two lot rows expected, got %+v", byID)
	}
}

// TestBreakdownHoldsSurviveMissingAccountRow (#86 Task 4): a tenant with NO
// budget account row answers held=0, refund_locked=0 and available=balance
// — an honest zero face, never an error.
func TestBreakdownHoldsSurviveMissingAccountRow(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true})
	svc, _, _ := newBenefitsService(t, fake)
	svc.SetNow(septemberClock())
	tenant := uint64(508)
	status, err := svc.EnsureBenefits(context.Background(), tenant, "Holds Space", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if status.Credits == nil {
		t.Fatal("credits view must answer")
	}
	if status.Credits.HeldMicro != 0 || status.Credits.RefundLockedMicro != 0 {
		t.Fatalf("no budget row must answer zero holds, got held=%d locked=%d",
			status.Credits.HeldMicro, status.Credits.RefundLockedMicro)
	}
	if status.Credits.BalanceMicro != BasePlanSeedIncludedCreditsMicro {
		t.Fatalf("balance = %d", status.Credits.BalanceMicro)
	}
	if status.Credits.ProjectedAt.IsZero() {
		t.Fatal("projected_at must answer the projection instant")
	}
}

// TestBreakdownCrossMonthBatchesCarryGrantedAt (OCR r1, CR-86-1): after a
// month rolls over the registry KEEPS the expired month's batch row while
// the authority snapshot no longer lists the terminated wallet — the view
// line's GrantedAt must fall back to the REGISTRY row's grant instant
// (created_at), never the zero time (the wire omits granted_at for a zero
// instant and the frontend contract then rejects the whole breakdown).
func TestBreakdownCrossMonthBatchesCarryGrantedAt(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true})
	svc, _, _ := newBenefitsService(t, fake)
	ctx := context.Background()
	tenant := uint64(509)
	// The current month: the registry row is minted (grant completed).
	clock := septemberClock()()
	period := domain.MonthlyPeriod(clock)
	svc.SetNow(func() time.Time { return clock })
	if _, err := svc.EnsureBenefits(ctx, tenant, "CrossMonth Space", "user-1"); err != nil {
		t.Fatal(err)
	}
	// The registry's grant instant for the month's row.
	row, err := svc.store.GetBatch(ctx, tenant, period)
	if err != nil {
		t.Fatal(err)
	}
	if row.CreatedAt.IsZero() {
		t.Fatal("test setup: the registry row must carry a grant instant")
	}
	// Next month: the month's wallet is TERMINATED on the authority (absent
	// from the snapshot — the post-lazy-termination steady state), yet the
	// registry row lingers. The expired month's view line must still carry a
	// NON-ZERO GrantedAt (the registry's grant instant).
	clock = clock.AddDate(0, 1, 0)
	fake.TerminateWallet(domain.MonthlyWalletName(tenant, period))
	status, err := svc.EnsureBenefits(ctx, tenant, "CrossMonth Space", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if status.Credits == nil {
		t.Fatal("credits view must answer")
	}
	sawExpiredMonth := false
	for _, b := range status.Credits.Batches {
		if b.Period != period {
			continue
		}
		sawExpiredMonth = true
		if b.GrantedAt.IsZero() {
			t.Fatal("CR-86-1: the expired month's lingering batch must carry the registry grant instant, got the zero time")
		}
		if !b.GrantedAt.Equal(row.CreatedAt) {
			t.Fatalf("%s GrantedAt = %v, want the registry row's CreatedAt %v (the snapshot no longer contributes one)", period, b.GrantedAt, row.CreatedAt)
		}
		if b.BalanceMicro != 0 {
			t.Fatalf("the expired %s batch must surface zero (no rollover), got %d", period, b.BalanceMicro)
		}
	}
	if !sawExpiredMonth {
		t.Fatalf("the %s registry batch must stay in the view (expired/zero, not dropped)", period)
	}
}

// TestMonthlyGrantEncodesYieldPriority (#86 Task 2): a top-up batch expiring
// BEFORE this month's end pushes the monthly wallet's creation priority to
// TopUpWalletPriority+1 (it must be consumed after the aging top-up); with
// no aging batch the next month's wallet keeps class 1. This drives
// EnsureMonthlyCredits DIRECTLY — the grant-time initial; the refresh-chain
// rebalance that follows a full EnsureBenefits would immediately converge
// the same set to absolute ranks (TestRefreshRebalancesMixedFamilies).
func TestMonthlyGrantEncodesYieldPriority(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true})
	svc, _, _ := newBenefitsService(t, fake)
	clock := septemberClock()()
	period := domain.MonthlyPeriod(clock)
	svc.SetNow(func() time.Time { return clock })
	tenant := uint64(505)
	ext := domain.ExternalCustomerID(tenant)

	// An aging top-up: expires mid-month, strictly before the period end.
	fake.SeedTopUpWallet(ext+"-topup-x", ext, 5_000,
		clock.AddDate(0, 0, 10),
		time.Now().UTC().AddDate(0, 0, -1), domain.TopUpWalletPriority)

	if _, err := svc.EnsureMonthlyCredits(context.Background(), tenant); err != nil {
		t.Fatal(err)
	}
	p, ok := walletPriorityByName(t, fake, domain.MonthlyWalletName(tenant, period))
	if !ok {
		t.Fatal("the current-month monthly wallet must exist")
	}
	if p != domain.TopUpWalletPriority+1 {
		t.Fatalf("monthly priority = %d, want %d (yielding to the aging top-up)", p, domain.TopUpWalletPriority+1)
	}

	// Next month: the aging batch is gone (expired) — no top-up expires
	// before the new period end, so the new monthly wallet keeps class 1.
	clock = clock.AddDate(0, 1, 0)
	if _, err := svc.EnsureMonthlyCredits(context.Background(), tenant); err != nil {
		t.Fatal(err)
	}
	p2, ok := walletPriorityByName(t, fake, domain.MonthlyWalletName(tenant, domain.MonthlyPeriod(clock)))
	if !ok {
		t.Fatal("the next-month monthly wallet must exist")
	}
	if p2 != domain.MonthlyWalletPriority {
		t.Fatalf("next-month monthly priority = %d, want %d (no aging top-up)", p2, domain.MonthlyWalletPriority)
	}
}

// TestRefreshRebalancesMixedFamilies (#86 Task 2, the r1-review High
// counterexample end-to-end): aging top-up A + monthly M + fresh top-up B
// coexist at their creation-time initials (A=2, B=2, M=3 — statically
// unorderable); one benefits refresh submits the authority rebalance and
// the fake's priorities converge to the true expiry order A=1, M=2, B=3.
func TestRefreshRebalancesMixedFamilies(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true})
	svc, _, _ := newBenefitsService(t, fake)
	clock := septemberClock()()
	period := domain.MonthlyPeriod(clock)
	svc.SetNow(func() time.Time { return clock })
	tenant := uint64(506)
	ext := domain.ExternalCustomerID(tenant)

	// A: aging top-up (expires before the period end).
	fake.SeedTopUpWallet(ext+"-topup-a", ext, 5_000,
		clock.AddDate(0, 0, 10),
		clock.AddDate(-1, 0, 0), domain.TopUpWalletPriority)
	// The first ensure mints the monthly wallet at the YIELDING initial (3).
	if _, err := svc.EnsureBenefits(context.Background(), tenant, "Mixed Space", "user-1"); err != nil {
		t.Fatal(err)
	}
	// B: fresh top-up (expires well after the period end).
	fake.SeedTopUpWallet(ext+"-topup-b", ext, 5_000,
		clock.AddDate(0, 6, 0),
		time.Now().UTC().AddDate(0, 0, -1), domain.TopUpWalletPriority)

	// The refresh chain now sees all three families and submits the
	// rebalance — the fake's priorities must converge to the expiry order.
	if _, err := svc.EnsureBenefits(context.Background(), tenant, "Mixed Space", "user-1"); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		ext + "-topup-a": 1,
		domain.MonthlyWalletName(tenant, period): 2,
		ext + "-topup-b": 3,
	}
	for name, rank := range want {
		p, ok := walletPriorityByName(t, fake, name)
		if !ok {
			t.Fatalf("wallet %s must exist", name)
		}
		if p != rank {
			t.Fatalf("priority of %s = %d, want %d (true expiry order)", name, p, rank)
		}
	}
}

// TestTopUpBatchSurvivesMonthRollover (#86 AC1+AC4): the top-up family
// keeps its own breakdown line across a monthly rollover — full balance,
// no period, its own unchanged twelve-month expiry — while the expired
// month surfaces zero (monthly never rolls over) and the new month mints
// its own line. The view stays reconcilable with the authority read-back
// at every step.
func TestTopUpBatchSurvivesMonthRollover(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true})
	svc, _, _ := newBenefitsService(t, fake)
	ctx := context.Background()
	tenant := uint64(510)
	ext := domain.ExternalCustomerID(tenant)
	clock := septemberClock()()
	period := domain.MonthlyPeriod(clock)
	svc.SetNow(func() time.Time { return clock })

	topUpExpiry := clock.AddDate(0, 6, 0)
	// granted a year back: visible under both the service clock and the
	// fake's own wall clock (settle-visibility is evaluated in real time).
	fake.SeedTopUpWallet(ext+"-topup-keep", ext, 5_000,
		topUpExpiry, clock.AddDate(-1, 0, 0), domain.TopUpWalletPriority)

	findTopUp := func(t *testing.T, status BenefitsStatus) BatchView {
		t.Helper()
		for _, b := range status.Credits.Batches {
			if b.Source == domain.BatchSourceTopUp {
				return b
			}
		}
		t.Fatal("the top-up batch must surface its own breakdown line")
		return BatchView{}
	}

	status1, err := svc.EnsureBenefits(ctx, tenant, "Rollover Space", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	topUp1 := findTopUp(t, status1)
	if topUp1.Period != "" {
		t.Fatalf("top-up line must carry no period, got %q", topUp1.Period)
	}
	if topUp1.BalanceMicro != 5_000*10_000 {
		t.Fatalf("top-up balance = %d, want %d", topUp1.BalanceMicro, 5_000*10_000)
	}
	if !topUp1.ExpiresAt.Equal(topUpExpiry) {
		t.Fatalf("top-up expiry = %v, want the granted %v", topUp1.ExpiresAt, topUpExpiry)
	}

	// Rollover: the old month's wallet leaves the snapshot (post-lazy-
	// termination), the clock enters the next month.
	clock = clock.AddDate(0, 1, 0)
	svc.SetNow(func() time.Time { return clock })
	fake.TerminateWallet(domain.MonthlyWalletName(tenant, period))
	status2, err := svc.EnsureBenefits(ctx, tenant, "Rollover Space", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	topUp2 := findTopUp(t, status2)
	if topUp2.BalanceMicro != 5_000*10_000 || !topUp2.ExpiresAt.Equal(topUpExpiry) {
		t.Fatalf("top-up must survive the rollover unchanged, got balance=%d expiry=%v",
			topUp2.BalanceMicro, topUp2.ExpiresAt)
	}
	var oldMonthZero, newMonth bool
	for _, b := range status2.Credits.Batches {
		if b.Source == domain.BatchSourceMonthly && b.Period == period && b.BalanceMicro == 0 {
			oldMonthZero = true
		}
		if b.Source == domain.BatchSourceMonthly && b.Period == domain.MonthlyPeriod(clock) && b.BalanceMicro > 0 {
			newMonth = true
		}
	}
	if !oldMonthZero {
		t.Fatalf("the expired %s month must surface zero (no rollover), got %+v", period, status2.Credits.Batches)
	}
	if !newMonth {
		t.Fatalf("the new month %s must mint its own line, got %+v", domain.MonthlyPeriod(clock), status2.Credits.Batches)
	}
}

// TestExpiredBatchSurfacesZero: the fake wallet is still "active" (lazy
// termination window simulated) yet the view reports zero — the registry
// overlay wins over the raw authority balance.
func TestExpiredBatchSurfacesZero(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true})
	svc, _, _ := newBenefitsService(t, fake)
	clock := septemberClock()()
	period := domain.MonthlyPeriod(clock)
	svc.SetNow(func() time.Time { return clock })
	if _, err := svc.EnsureBenefits(context.Background(), benefitsTenant, "Lazy Space", "user-1"); err != nil {
		t.Fatal(err)
	}
	// The authority STILL reports the wallet active with a full balance
	// (termination is lazy) — advance past the period end.
	clock = clock.AddDate(0, 1, 0)
	raw, err := fake.ReadSnapshot(context.Background(), domain.SnapshotQuery{
		Kind: domain.SnapshotKindBenefits, TenantID: benefitsTenant,
	})
	if err != nil {
		t.Fatal(err)
	}
	if raw.Benefits.BalanceMicro == 0 {
		t.Fatal("test setup: the raw authority balance must still be non-zero")
	}
	status, err := svc.EnsureBenefits(context.Background(), benefitsTenant, "Lazy Space", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	// The next month's fresh grant is present; the old month must NOT be spendable.
	if status.Credits == nil {
		t.Fatal("credits view missing")
	}
	for _, b := range status.Credits.Batches {
		if b.Period == period && b.BalanceMicro != 0 {
			t.Fatalf("expired batch leaked spendable credits: %+v", b)
		}
	}
}

// TestGrantWalletCapRetry: a scripted wallet_limit_reached window surfaces
// the closed unreachable reason (state pending), mints NO second batch row,
// and completing after the block clears grants by identity — no duplicate.
func TestGrantWalletCapRetry(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true})
	svc, broker, _ := newBenefitsService(t, fake)
	svc.SetNow(septemberClock())

	fake.RejectWalletCreatesWith(fmt.Errorf("%w: wallet capacity", domain.ErrPlatformUnreachable))
	status, err := svc.EnsureBenefits(context.Background(), benefitsTenant, "Cap Space", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if status.Reason != "unreachable" {
		t.Fatalf("reason = %q, want unreachable", status.Reason)
	}
	if status.Credits != nil {
		t.Fatalf("a pending chain must not fabricate credits, got %+v", status.Credits)
	}
	if n := broker.batchCount(benefitsTenant); n != 1 {
		t.Fatalf("exactly the first batch row (no duplicate minting), got %d", n)
	}
	// Clear the block; the SAME identity completes the grant.
	fake.RejectWalletCreatesWith(nil)
	status, err = svc.EnsureBenefits(context.Background(), benefitsTenant, "Cap Space", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if status.Reason != "" || status.Credits == nil || status.Credits.BalanceMicro != BasePlanSeedIncludedCreditsMicro {
		t.Fatalf("completed chain = reason %q credits %+v", status.Reason, status.Credits)
	}
	if wallets := fake.Wallets(); len(wallets) != 1 {
		t.Fatalf("wallet count = %d, want 1 (identity completion, no duplicate)", len(wallets))
	}
	if n := broker.batchCount(benefitsTenant); n != 1 {
		t.Fatalf("batch rows = %d, want 1", n)
	}
}

// TestPlatformFailureIsPendingState: a platform fault at each chain stage
// is a STATE (pending + closed reason), never a caller error; clearing the
// fault resumes the chain exactly where it stopped.
func TestPlatformFailureIsPendingState(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true})
	svc, _, _ := newBenefitsService(t, fake)
	svc.SetNow(septemberClock())
	ctx := context.Background()
	fault := fmt.Errorf("%w: injected fault", domain.ErrPlatformUnreachable)

	// Stage: the whole chain unavailable — the account leg reports pending.
	fake.FailSubmitsWith(fault)
	status, err := svc.EnsureBenefits(ctx, benefitsTenant, "Fault Space", "user-1")
	if err != nil {
		t.Fatalf("a platform fault must be a state, not an error: %v", err)
	}
	if status.Account.State != BillingAccountPending || status.Reason != "unreachable" {
		t.Fatalf("pending posture = %+v reason %q", status.Account, status.Reason)
	}
	if status.Plan != nil || status.Credits != nil {
		t.Fatalf("a pending chain must not fabricate a plan/credits: %+v %+v", status.Plan, status.Credits)
	}

	// Stage: the account completed; the subscription leg faults.
	fake.FailSubmitsWith(nil)
	if _, err := svc.EnsureBenefits(ctx, benefitsTenant, "Fault Space", "user-1"); err != nil {
		t.Fatal(err)
	}
	fake.FailSubmitsWith(fault)
	status, err = svc.EnsureBenefits(ctx, benefitsTenant, "Fault Space", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if status.Account.State != BillingAccountLinked || status.Reason != "unreachable" {
		t.Fatalf("linked account + subscription fault = %+v %q", status.Account, status.Reason)
	}

	// Stage: the subscription completed; the grant leg faults
	// (persisted-but-response-lost on the fake).
	fake.FailSubmitsWith(nil)
	if _, err := svc.EnsureBenefits(ctx, benefitsTenant, "Fault Space", "user-1"); err != nil {
		t.Fatal(err)
	}
	fake.FailSubmitsWith(fault)
	status, err = svc.EnsureBenefits(ctx, benefitsTenant, "Fault Space", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if status.Reason != "unreachable" || status.Credits != nil {
		t.Fatalf("grant fault = reason %q credits %+v", status.Reason, status.Credits)
	}

	// Clear: the chain completes resumably without duplicates.
	fake.FailSubmitsWith(nil)
	status, err = svc.EnsureBenefits(ctx, benefitsTenant, "Fault Space", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if status.Reason != "" || status.Plan == nil || status.Credits == nil {
		t.Fatalf("completed = reason %q plan %+v credits %+v", status.Reason, status.Plan, status.Credits)
	}
	if subs := fake.Subscriptions(); len(subs) != 1 {
		t.Fatalf("subscriptions = %d, want 1", len(subs))
	}
	if wallets := fake.Wallets(); len(wallets) != 1 || wallets[0].GrantedCents != BasePlanSeedIncludedCreditsMicro/10_000 {
		t.Fatalf("wallets = %+v", wallets)
	}
}

// TestNilPlatformFailsClosed: a nil seam — account pending/unconfigured,
// no projection, no panics.
func TestNilPlatformBenefitsFailsClosed(t *testing.T) {
	svc, broker, _ := newBenefitsService(t, nil)
	svc.SetNow(septemberClock())
	status, err := svc.EnsureBenefits(context.Background(), benefitsTenant, "Nil Space", "user-1")
	if err != nil {
		t.Fatalf("nil platform must be a state, got error %v", err)
	}
	if status.Account.State != BillingAccountPending || status.Reason != "unconfigured" {
		t.Fatalf("nil platform posture = %+v reason %q", status.Account, status.Reason)
	}
	var projections int64
	broker.DB.Raw(`SELECT COUNT(*) FROM commercial_tenant_benefits`).Scan(&projections)
	if projections != 0 {
		t.Fatalf("no projection may be written without an authority answer, got %d", projections)
	}
}

// TestSeedBasePlanIdempotent: seeding twice is a no-op; a pre-published
// operator variant of (base,1) is respected, never re-published.
func TestSeedBasePlanIdempotent(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	svc, _, db := newBenefitsService(t, fake)
	ctx := context.Background()
	seeded, err := svc.SeedBasePlan(ctx)
	if err != nil || !seeded {
		t.Fatalf("first seed = (%v, %v)", seeded, err)
	}
	again, err := svc.SeedBasePlan(ctx)
	if err != nil || again {
		t.Fatalf("second seed must be a no-op, got (%v, %v)", again, err)
	}
	var pubs int64
	db.Raw(`SELECT COUNT(*) FROM commercial_plan_publications WHERE plan_key = 'base'`).Scan(&pubs)
	if pubs != 1 {
		t.Fatalf("publications = %d, want 1", pubs)
	}

	// Operator-published variant: a different service instance (fresh
	// operator draft) must respect the existing publication, not re-publish.
	opFake := commercialplatform.NewFakeAdapter()
	opPlans, err := NewPlanVersionService(db, opFake)
	if err != nil {
		t.Fatal(err)
	}
	opAccounts, err := NewBillingAccountService(db, opFake)
	if err != nil {
		t.Fatal(err)
	}
	opSvc, err := NewBenefitsService(db, opAccounts, opPlans, opFake, nil)
	if err != nil {
		t.Fatal(err)
	}
	opSvc.SetNow(septemberClock())
	seeded, err = opSvc.SeedBasePlan(ctx)
	if err != nil || seeded {
		t.Fatalf("a pre-published (base,1) must be respected, got (%v, %v)", seeded, err)
	}
	db.Raw(`SELECT COUNT(*) FROM commercial_plan_publications WHERE plan_key = 'base'`).Scan(&pubs)
	if pubs != 1 {
		t.Fatalf("publications = %d, want 1 (publish-once immutability)", pubs)
	}
}

// TestApplyQuotasResyncBeforeLimit: occupancy re-sync happens BEFORE limits
// apply — a drifted counter (used=7, real members 3) re-syncs to 3, then
// the limit lands; over-limit derives from real occupancy, never counter
// drift.
func TestApplyQuotasResyncBeforeLimit(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true})
	svc, broker, _ := newBenefitsService(t, fake)
	svc.SetNow(septemberClock())
	ctx := context.Background()
	for _, uid := range []string{"u1", "u2", "u3"} {
		broker.insertMember(t, benefitsTenant, uid)
	}
	// Seed a drifted counter.
	if err := broker.DB.Exec(`INSERT INTO commercial_resource_counters (tenant_id, resource, used, hard_limit)
		VALUES (?, 'members', 7, NULL)`, benefitsTenant).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyQuotas(ctx, benefitsTenant, map[string]int64{"members": 5}); err != nil {
		t.Fatal(err)
	}
	used, limit, has := broker.counter(benefitsTenant, "members")
	if used != 3 || !has || limit != 5 {
		t.Fatalf("members counter after refresh = (%d, %d, has=%v), want used=3 limit=5", used, limit, has)
	}
	// A second refresh with the same occupancy is stable (no drift back).
	if err := svc.ApplyQuotas(ctx, benefitsTenant, map[string]int64{"members": 5}); err != nil {
		t.Fatal(err)
	}
	if used, _, _ = broker.counter(benefitsTenant, "members"); used != 3 {
		t.Fatalf("occupancy re-sync must be stable, got used=%d", used)
	}
	// Absent dimension = unlimited (NULL limit).
	if err := svc.ApplyQuotas(ctx, benefitsTenant, map[string]int64{}); err != nil {
		t.Fatal(err)
	}
	if _, _, has = broker.counter(benefitsTenant, "members"); has {
		t.Fatal("an absent dimension must clear the hard limit (unlimited)")
	}
}

// TestEnsureBenefitsStorageOccupancySynced: tenants.storage_used feeds the
// storage counter (bytes).
func TestEnsureBenefitsStorageOccupancySynced(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true})
	svc, broker, _ := newBenefitsService(t, fake)
	svc.SetNow(septemberClock())
	if err := broker.DB.Exec(`INSERT INTO tenants (id, name, storage_used) VALUES (?, 'S', 4096)`,
		benefitsTenant).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.EnsureBenefits(context.Background(), benefitsTenant, "Storage Space", "user-1"); err != nil {
		t.Fatal(err)
	}
	if used, limit, has := broker.counter(benefitsTenant, "storage_gb"); !has || used != 4096 || limit != 10*(1<<30) {
		t.Fatalf("storage counter = (%d, %d, has=%v), want used=4096 limit=10×2^30", used, limit, has)
	}
}

// slowGrantPlatform delays every grant submit mid-flight BEFORE it reaches
// the authority — the read-before-create windows of concurrent first
// grants then overlap the way they do against the real authority (which,
// per E3, has no wallet idempotency: every grant POST that reaches it is a
// potential doubled balance). The fake's own internal serialization cannot
// expose the service-level TOCTOU; this wrapper can.
type slowGrantPlatform struct {
	inner domain.CommercialPlatform
	delay time.Duration

	mu           sync.Mutex
	grantSubmits int
}

func (p *slowGrantPlatform) SubmitCommand(ctx context.Context, cmd domain.Command) (domain.CommandReceipt, error) {
	if cmd.Kind == domain.CommandKindGrantIncludedCredits {
		p.mu.Lock()
		p.grantSubmits++
		p.mu.Unlock()
		time.Sleep(p.delay)
	}
	return p.inner.SubmitCommand(ctx, cmd)
}

func (p *slowGrantPlatform) ReadSnapshot(ctx context.Context, q domain.SnapshotQuery) (domain.Snapshot, error) {
	return p.inner.ReadSnapshot(ctx, q)
}

func (p *slowGrantPlatform) Reconcile(ctx context.Context, from domain.ReconciliationCursor) (domain.ReconciliationPage, error) {
	return p.inner.Reconcile(ctx, from)
}

func (p *slowGrantPlatform) grantsSubmitted() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.grantSubmits
}

// TestEnsureBenefitsConcurrentFirstGrantExactlyOnePost: N concurrent
// FIRST-TIME ensures on one tenant with a mid-POST delay — EXACTLY ONE
// grant submit may leave the service (each submit is a potential E3 POST
// against an authority with no wallet idempotency); exactly one wallet and
// one registry row result.
func TestEnsureBenefitsConcurrentFirstGrantExactlyOnePost(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true})
	slow := &slowGrantPlatform{inner: fake, delay: 50 * time.Millisecond}
	svc, broker, _ := newBenefitsService(t, slow)
	svc.SetNow(septemberClock())

	const racers = 8
	started := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-started
			if _, err := svc.EnsureBenefits(context.Background(), benefitsTenant, "TOCTOU Space", "user-1"); err != nil {
				t.Errorf("racer: %v", err)
			}
		}()
	}
	close(started)
	wg.Wait()

	if got := slow.grantsSubmitted(); got != 1 {
		t.Fatalf("EXACTLY ONE grant submit may leave the service under a concurrent first grant (E3): got %d", got)
	}
	if wallets := fake.Wallets(); len(wallets) != 1 || wallets[0].GrantedCents != BasePlanSeedIncludedCreditsMicro/10_000 {
		t.Fatalf("exactly one wallet with one month's grant expected, got %+v", wallets)
	}
	if n := broker.batchCount(benefitsTenant); n != 1 {
		t.Fatalf("one registry row expected, got %d", n)
	}
}

// TestEffectiveFeaturesStalePurchaseEntitlementNotMaterialized（A-22 / F92）：
// authority 的 entitlement 读是 base 与 purchase 两腿的无状态过滤并集——
// canceled 购买的 entitlement 可能残留到懒清理窗口。购买 plan 定义已知的
// codes 只在购买 ACTIVE 时从 authority 物化腿进来；非 ACTIVE 购买（canceled）
// 的 plan 定义 codes 必须从物化中剔除，未付款/已取消者不得凭残留 entitlement
// 保留付费特性。
func TestEffectiveFeaturesStalePurchaseEntitlementNotMaterialized(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true, "advanced_models": false})
	svc, _, db := newBenefitsService(t, fake)
	// 本地发布 pro 计划（定义 advanced_models=true）。
	proDef := domain.PlanVersion{Key: "pro", Version: 1, Price: 99_00, Monthly: 9_900_000,
		Features: map[string]bool{"advanced_models": true}, Currency: domain.CurrencyCNY}
	proJSON, err := json.Marshal(proDef)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&repocommercial.PlanRow{PlanKey: "pro", Version: 1,
		DefinitionJSON: string(proJSON), State: domain.PlanStatePublished}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&repocommercial.PublicationRow{
		CommandKey: domain.PublishCommandKey("pro", 1), PlanKey: "pro", Version: 1,
		PlanCode: domain.DeterministicPlanCode("pro", 1), ReceiptJSON: "{}",
		PublishedBy: "test", PublishedAt: time.Now().UTC(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	const tenant = uint64(522)
	ext := domain.ExternalPurchaseSubscriptionID(tenant)
	// CANCELED 购买（懒清理窗口内 authority 仍带着 entitlement）。
	if _, err := fake.SubmitCommand(ctx, domain.Command{
		Kind:  domain.CommandKindCreatePurchaseSubscription,
		Key:   domain.CreatePurchaseSubscriptionCommandKey(ext, domain.DeterministicPlanCode("pro", 1)),
		Actor: "test", Reason: "purchase",
		Payload: domain.CreatePurchaseSubscriptionPayload{
			TenantID: tenant, ExternalCustomerID: domain.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: ext, PlanCode: domain.DeterministicPlanCode("pro", 1),
			AmountFen: 99_00, Currency: domain.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
	fake.CancelPurchase(ext)
	base := map[string]bool{"api_access": true, "advanced_models": false}
	// 模拟 authority 懒清理窗口的残留 entitlement（购买腿仍带着 code）。
	staleEntitled := map[string]bool{"advanced_models": true}
	out := svc.effectiveFeatures(ctx, tenant, base, staleEntitled)
	if out["advanced_models"] {
		t.Fatalf("a CANCELED purchase's plan-defined code must NOT materialize from the stale authority entitlement leg, got %+v", out)
	}
	if !out["api_access"] {
		t.Fatalf("base features must answer regardless, got %+v", out)
	}
	// 对照组：authority 物化一个本地无定义的 code 仍然生效（未知 code 的
	// 物化语义保留——只剔除非 ACTIVE 购买 plan 定义的 codes）。
	out2 := svc.effectiveFeatures(ctx, tenant, base, map[string]bool{"brand_new_code": true})
	if !out2["brand_new_code"] {
		t.Fatalf("an authority-materialized code no definition knows must still answer true, got %+v", out2)
	}
	// 对照组二：ACTIVE 购买时同 code 经定义腿正常 OR 上来（CancelPurchase
	// 后的 settle 会被 fake 按终态拒绝——直接 ActivatePurchase 表达「权威
	// 已激活」姿态，隔离 settle 细节）。
	fake.ActivatePurchase(ext)
	out3 := svc.effectiveFeatures(ctx, tenant, base, staleEntitled)
	if !out3["advanced_models"] {
		t.Fatalf("an ACTIVE purchase's plan definition must OR advanced_models TRUE, got %+v", out3)
	}
}
