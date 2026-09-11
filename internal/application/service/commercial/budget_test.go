package commercial

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// entitlementGate flips a space/plan entitlement on and off to model a
// downgrade mid-run: denied means the tenant lost the entitlement its
// NEW budget steps depend on.
type entitlementGate struct {
	mu     sync.Mutex
	denied bool
	reason string
}

func (g *entitlementGate) check(_ context.Context, _ uint64) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.denied {
		return errors.New(g.reason)
	}
	return nil
}

func (g *entitlementGate) deny(reason string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.denied = true
	g.reason = reason
}

func budgetTestEnv(t *testing.T) (*gorm.DB, *repocommercial.BudgetStore, *BudgetService, *entitlementGate) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1) // serialize SQLite writers; races stay logical
	}
	if err := db.AutoMigrate(
		&repocommercial.BudgetAccountRow{}, &repocommercial.TaskBudgetRow{},
		&repocommercial.ReservationRow{}, &repocommercial.BudgetLotRow{},
		&repocommercial.BudgetLotAllocationRow{}, &repocommercial.TaskBudgetExtensionRow{},
		&SettlementRecord{},
	); err != nil {
		t.Fatal(err)
	}
	store := repocommercial.NewBudgetStore(db)
	gate := &entitlementGate{}
	svc, err := NewBudgetService(db, store, gate.check)
	if err != nil {
		t.Fatal(err)
	}
	return db, store, svc, gate
}

func seedBudgetAccount(t *testing.T, db *gorm.DB, tenant uint64, verified int64) {
	t.Helper()
	if err := db.Create(&repocommercial.BudgetAccountRow{
		TenantID: tenant, VerifiedMicro: verified, Watermark: "w0", Version: 1,
		VerifiedUntil: time.Now().UTC().Add(time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&repocommercial.BudgetLotRow{
		TenantID: tenant, LotID: "lot_1", RemainingMicro: verified, IssuedAt: time.Now().UTC(),
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func seedTaskBudget(t *testing.T, db *gorm.DB, tenant uint64, run string, limit int64) {
	t.Helper()
	if err := db.Create(&repocommercial.TaskBudgetRow{
		TenantID: tenant, RunID: run, LimitMicro: limit, Deadline: time.Now().UTC().Add(time.Hour), Version: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func mustReserve(t *testing.T, store *repocommercial.BudgetStore, tenant uint64, run, key string, upper domain.Credits) domain.Reservation {
	t.Helper()
	res, err := store.Reserve(context.Background(), domain.BudgetRequest{
		TenantID: tenant, RunID: run, Key: key, Upper: upper,
		Deadline: time.Now().UTC().Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func readBudgetAccount(t *testing.T, db *gorm.DB, tenant uint64) repocommercial.BudgetAccountRow {
	t.Helper()
	var acct repocommercial.BudgetAccountRow
	if err := db.Where("tenant_id = ?", tenant).First(&acct).Error; err != nil {
		t.Fatal(err)
	}
	return acct
}

func readTaskBudget(t *testing.T, db *gorm.DB, tenant uint64, run string) repocommercial.TaskBudgetRow {
	t.Helper()
	var task repocommercial.TaskBudgetRow
	if err := db.Where("tenant_id = ? AND run_id = ?", tenant, run).First(&task).Error; err != nil {
		t.Fatal(err)
	}
	return task
}

func readReservation(t *testing.T, db *gorm.DB, tenant uint64, key string) repocommercial.ReservationRow {
	t.Helper()
	var res repocommercial.ReservationRow
	if err := db.Where("tenant_id = ? AND key = ?", tenant, key).First(&res).Error; err != nil {
		t.Fatal(err)
	}
	return res
}

// An expired (or cancelled) reservation that was never dispatched
// releases its hold directly, everywhere the hold was taken: account,
// task budget, and lot capacity.
func TestExpiredLeaseHeldReservationReleasesDirectly(t *testing.T) {
	db, store, svc, _ := budgetTestEnv(t)
	ctx := context.Background()
	seedBudgetAccount(t, db, 7, 1000)
	seedTaskBudget(t, db, 7, "run_a", 1000)
	mustReserve(t, store, 7, "run_a", "res_a", 400)

	if err := svc.Reconcile(ctx, "res_a"); err != nil {
		t.Fatal(err)
	}
	if res := readReservation(t, db, 7, "res_a"); res.State != domain.ReservationStateReleased {
		t.Fatalf("reservation state %q, want released", res.State)
	}
	acct := readBudgetAccount(t, db, 7)
	if acct.HeldMicro != 0 {
		t.Fatalf("account held %d, want 0", acct.HeldMicro)
	}
	if task := readTaskBudget(t, db, 7, "run_a"); task.HeldMicro != 0 {
		t.Fatalf("task held %d, want 0", task.HeldMicro)
	}
	var lot repocommercial.BudgetLotRow
	if err := db.Where("tenant_id = ?", uint64(7)).First(&lot).Error; err != nil {
		t.Fatal(err)
	}
	if lot.HeldMicro != 0 || lot.RemainingMicro != 1000 {
		t.Fatalf("lot held=%d remaining=%d, want 0/1000", lot.HeldMicro, lot.RemainingMicro)
	}
	var allocs int64
	if err := db.Model(&repocommercial.BudgetLotAllocationRow{}).Where("tenant_id = ?", uint64(7)).Count(&allocs).Error; err != nil {
		t.Fatal(err)
	}
	if allocs != 0 {
		t.Fatalf("%d allocation rows survived release", allocs)
	}
	// reconciling an already-released reservation is an idempotent no-op
	if err := svc.Reconcile(ctx, "res_a"); err != nil {
		t.Fatal(err)
	}
}

// Task cancelled but the remote call already succeeded: the dispatched
// settlement keeps the protection and reconciliation must settle toward
// confirmation, never zero the cost.
func TestExpiredLeaseDispatchedCostIsNeverZeroed(t *testing.T) {
	db, store, svc, _ := budgetTestEnv(t)
	ctx := context.Background()
	seedBudgetAccount(t, db, 7, 1000)
	seedTaskBudget(t, db, 7, "run_a", 1000)
	mustReserve(t, store, 7, "run_a", "res_b", 500)

	// Finalize-equivalent conversion: consumed 300 protected, unused 200
	// released, reservation settled.
	if err := store.SettleReservationHold(ctx, 7, "res_b", 300, nil); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := db.Create(&SettlementRecord{
		Key: "settle:u04b", TenantID: 7, CallID: "call_b", AttemptID: "att_b",
		ReservationKey: "res_b", RunID: "run_a", AmountMicro: 300, Revision: 1,
		State: domain.SettlementStateDispatched, OccurredAt: now.Add(30 * time.Second), UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	err := svc.Reconcile(ctx, "res_b")
	if !errors.Is(err, ErrReconcileNeedsConfirmation) {
		t.Fatalf("want ErrReconcileNeedsConfirmation, got %v", err)
	}
	acct := readBudgetAccount(t, db, 7)
	if acct.UnreflectedMicro != 300 || acct.HeldMicro != 0 || acct.VerifiedMicro != 1000 {
		t.Fatalf("protection altered: unreflected=%d held=%d verified=%d, want 300/0/1000",
			acct.UnreflectedMicro, acct.HeldMicro, acct.VerifiedMicro)
	}

	// A dispatched-but-unsettled reservation is retained the same way:
	// query first, release only when confirmed no external usage.
	mustReserve(t, store, 7, "run_a", "res_c", 200)
	if err := store.MarkReservationDispatched(ctx, 7, "res_c"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Reconcile(ctx, "res_c"); !errors.Is(err, ErrReconcileNeedsConfirmation) {
		t.Fatalf("dispatched reservation reconciled without query: %v", err)
	}
	acct = readBudgetAccount(t, db, 7)
	if acct.HeldMicro != 200 {
		t.Fatalf("dispatched hold zeroed: held=%d, want 200", acct.HeldMicro)
	}
	// a late report OUTSIDE the reservation occurrence interval belongs
	// to an earlier interval and does not block this release decision
	if err := db.Create(&SettlementRecord{
		Key: "settle:u04late", TenantID: 7, CallID: "call_old", AttemptID: "att_old",
		ReservationKey: "res_c", RunID: "run_a", AmountMicro: 50, Revision: 1,
		State: domain.SettlementStateDispatched, OccurredAt: now.Add(-time.Hour), UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	// res_c itself stays dispatched, so it still needs a query; the
	// out-of-interval record must not be the reason it flips to zero.
	if err := svc.Reconcile(ctx, "res_c"); !errors.Is(err, ErrReconcileNeedsConfirmation) {
		t.Fatalf("out-of-interval late report changed the outcome: %v", err)
	}
}

// Duplicate Extend with the same idempotency key never increases the
// task limit twice; a different key increases it again.
func TestBudgetExtendDuplicateKeyIsIdempotent(t *testing.T) {
	db, _, svc, _ := budgetTestEnv(t)
	ctx := context.Background()
	seedBudgetAccount(t, db, 7, 1000)
	seedTaskBudget(t, db, 7, "run_a", 1000)

	if err := svc.Extend(ctx, 7, "run_a", "ext_1", 500); err != nil {
		t.Fatal(err)
	}
	if task := readTaskBudget(t, db, 7, "run_a"); task.LimitMicro != 1500 {
		t.Fatalf("limit %d, want 1500", task.LimitMicro)
	}
	for i := 0; i < 2; i++ {
		if err := svc.Extend(ctx, 7, "run_a", "ext_1", 500); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.Extend(ctx, 7, "run_a", "ext_2", 500); err != nil {
		t.Fatal(err)
	}
	task := readTaskBudget(t, db, 7, "run_a")
	if task.LimitMicro != 2000 {
		t.Fatalf("limit %d, want 2000 (one increase per key)", task.LimitMicro)
	}
	var exts int64
	if err := db.Model(&repocommercial.TaskBudgetExtensionRow{}).Where("tenant_id = ?", uint64(7)).Count(&exts).Error; err != nil {
		t.Fatal(err)
	}
	if exts != 2 {
		t.Fatalf("%d extension rows, want 2", exts)
	}
}

// A space downgrade revokes the entitlement of NEW budget steps: Extend
// pauses with an explicit reason instead of silently continuing.
func TestBudgetExtendUnauthorizedPausesWithExplicitReason(t *testing.T) {
	db, _, svc, gate := budgetTestEnv(t)
	ctx := context.Background()
	seedBudgetAccount(t, db, 7, 1000)
	seedTaskBudget(t, db, 7, "run_a", 1000)
	gate.deny("space downgraded below the plan this run was authorized on")

	err := svc.Extend(ctx, 7, "run_a", "ext_1", 500)
	if !errors.Is(err, ErrBudgetUnauthorized) {
		t.Fatalf("want ErrBudgetUnauthorized, got %v", err)
	}
	if !strings.Contains(err.Error(), "space downgraded") {
		t.Fatalf("pause reason not explicit: %v", err)
	}
	if task := readTaskBudget(t, db, 7, "run_a"); task.LimitMicro != 1000 {
		t.Fatalf("unauthorized extend changed the limit: %d", task.LimitMicro)
	}
}

// Downgrade blocks only NEW steps: an existing reservation still
// reconciles (releases) under the denied entitlement.
func TestBudgetRecoveryDowngradeBlocksNewExtendButNotReconcile(t *testing.T) {
	db, store, svc, gate := budgetTestEnv(t)
	ctx := context.Background()
	seedBudgetAccount(t, db, 7, 1000)
	seedTaskBudget(t, db, 7, "run_a", 1000)
	mustReserve(t, store, 7, "run_a", "res_d", 300)
	gate.deny("space downgraded")

	if err := svc.Extend(ctx, 7, "run_a", "ext_1", 500); !errors.Is(err, ErrBudgetUnauthorized) {
		t.Fatalf("new step under downgrade not paused: %v", err)
	}
	if err := svc.Reconcile(ctx, "res_d"); err != nil {
		t.Fatalf("reconcile blocked by downgrade: %v", err)
	}
	if res := readReservation(t, db, 7, "res_d"); res.State != domain.ReservationStateReleased {
		t.Fatalf("state %q, want released", res.State)
	}
	if acct := readBudgetAccount(t, db, 7); acct.HeldMicro != 0 {
		t.Fatalf("held %d, want 0", acct.HeldMicro)
	}
}

// Lease recovery increments the fence: the stale worker that lost the
// takeover cannot commit; the new owner can.
func TestBudgetRecoveryStaleLeaseWorkerCommitRejected(t *testing.T) {
	db, store, _, _ := budgetTestEnv(t)
	ctx := context.Background()
	seedBudgetAccount(t, db, 7, 1000)
	seedTaskBudget(t, db, 7, "run_a", 1000)
	mustReserve(t, store, 7, "run_a", "res_e", 100)
	// worker_1 first acquires the lease (recovery assignment), then loses it
	oldFence, err := store.TakeoverLease(ctx, 7, "res_e", "worker_1")
	if err != nil {
		t.Fatal(err)
	}

	newFence, err := store.TakeoverLease(ctx, 7, "res_e", "worker_2")
	if err != nil {
		t.Fatal(err)
	}
	if newFence == oldFence {
		t.Fatal("takeover did not increment the fence")
	}
	if err := store.VerifyLeaseCommit(ctx, 7, "res_e", "worker_1", oldFence); !errors.Is(err, repocommercial.ErrLeaseFenceStale) {
		t.Fatalf("stale worker commit accepted: %v", err)
	}
	if err := store.VerifyLeaseCommit(ctx, 7, "res_e", "worker_2", oldFence); !errors.Is(err, repocommercial.ErrLeaseFenceStale) {
		t.Fatalf("stale fence under the new owner accepted: %v", err)
	}
	if err := store.VerifyLeaseCommit(ctx, 7, "res_e", "worker_2", newFence); err != nil {
		t.Fatalf("live worker commit rejected: %v", err)
	}
	// a reservation no longer held refuses further takeovers
	if err := store.MarkReservationDispatched(ctx, 7, "res_e"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.TakeoverLease(ctx, 7, "res_e", "worker_3"); !errors.Is(err, repocommercial.ErrReservationNotHeld) {
		t.Fatalf("takeover of a dispatched reservation: %v", err)
	}
}

// Expiry release and a refund lock compete for one projection without
// double free: both fit (400+600 of 1000), the release happens exactly
// once, and the account stays consistent under -race.
func TestBudgetRecoveryExpiryReleaseVersusRefundLockRace(t *testing.T) {
	db, store, _, _ := budgetTestEnv(t)
	ctx := context.Background()
	seedBudgetAccount(t, db, 7, 1000)
	seedTaskBudget(t, db, 7, "run_a", 1000)
	mustReserve(t, store, 7, "run_a", "res_f", 400)

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		errs <- store.ReleaseReservation(ctx, 7, "res_f")
	}()
	go func() {
		defer wg.Done()
		<-start
		errs <- store.LockRefunds(ctx, 7, 600)
	}()
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	acct := readBudgetAccount(t, db, 7)
	if acct.HeldMicro != 0 {
		t.Fatalf("held %d after release, want 0", acct.HeldMicro)
	}
	if acct.RefundLockedMicro != 600 {
		t.Fatalf("refund_locked %d, want 600", acct.RefundLockedMicro)
	}
	avail, err := domain.Available(
		domain.Credits(acct.VerifiedMicro), domain.Credits(acct.UnreflectedMicro),
		domain.Credits(acct.HeldMicro), domain.Credits(acct.RefundLockedMicro))
	if err != nil {
		t.Fatal(err)
	}
	// the released 400 hold returns to availability exactly once: with
	// 600 refund-locked, exactly 400 stays spendable — 400+600=1000, no
	// double free, nothing lost.
	if avail != 400 {
		t.Fatalf("availability %d, want exactly 400 (single release)", avail)
	}
	// a second release of the same reservation never frees again
	if err := store.ReleaseReservation(ctx, 7, "res_f"); !errors.Is(err, repocommercial.ErrReservationNotHeld) {
		t.Fatalf("double release accepted: %v", err)
	}
}
