package commercial

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/infrastructure/commercialplatform"

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
	svc, err := NewBenefitsService(db, accounts, plans, platform)
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

func septemberClock() func() time.Time {
	return func() time.Time { return time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC) }
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
	if len(status.Credits.Batches) != 1 || status.Credits.Batches[0].Period != "2026-09" ||
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
	clock := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	svc.SetNow(func() time.Time { return clock })

	if _, err := svc.EnsureBenefits(context.Background(), benefitsTenant, "Period Space", "user-1"); err != nil {
		t.Fatal(err)
	}
	// Cross into October: the September batch (end 2026-10-01) is expired.
	clock = time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
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
	if byPeriod["2026-09"] != 0 {
		t.Fatalf("the expired September batch must surface zero, got %d", byPeriod["2026-09"])
	}
	if byPeriod["2026-10"] != BasePlanSeedIncludedCreditsMicro {
		t.Fatalf("the October batch must carry one month, got %d", byPeriod["2026-10"])
	}
	if n := broker.batchCount(benefitsTenant); n != 2 {
		t.Fatalf("registry must keep both periods, got %d", n)
	}
}

// TestExpiredBatchSurfacesZero: the fake wallet is still "active" (lazy
// termination window simulated) yet the view reports zero — the registry
// overlay wins over the raw authority balance.
func TestExpiredBatchSurfacesZero(t *testing.T) {
	fake := commercialplatform.NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true})
	svc, _, _ := newBenefitsService(t, fake)
	clock := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	svc.SetNow(func() time.Time { return clock })
	if _, err := svc.EnsureBenefits(context.Background(), benefitsTenant, "Lazy Space", "user-1"); err != nil {
		t.Fatal(err)
	}
	// The authority STILL reports the wallet active with a full balance
	// (termination is lazy) — advance past the period end.
	clock = time.Date(2026, 10, 1, 0, 30, 0, 0, time.UTC)
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
	// October's fresh grant is present; September must NOT be spendable.
	if status.Credits == nil {
		t.Fatal("credits view missing")
	}
	for _, b := range status.Credits.Batches {
		if b.Period == "2026-09" && b.BalanceMicro != 0 {
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
	opSvc, err := NewBenefitsService(db, opAccounts, opPlans, opFake)
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
