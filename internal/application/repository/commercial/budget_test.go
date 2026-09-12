package commercial

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// testBudgetStore mirrors order_test.go's SQLite setup. A single pooled
// connection keeps the read-then-CAS transaction free of shared-cache
// table-lock deadlocks: SQLite is single-writer anyway, and the guarded
// UPDATE — never a process mutex — decides the single winner.
func testBudgetStore(t *testing.T) (*BudgetStore, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&BudgetAccountRow{}, &TaskBudgetRow{}, &ReservationRow{}, &BudgetLotRow{}, &BudgetLotAllocationRow{}); err != nil {
		t.Fatal(err)
	}
	return NewBudgetStore(db), db
}

func seedBudget(t *testing.T, db *gorm.DB, verified, limit int64) {
	t.Helper()
	end := time.Now().UTC().Add(time.Hour)
	expiry := end
	for _, row := range []any{
		&BudgetAccountRow{TenantID: 7, VerifiedMicro: verified, Watermark: "w1", Version: 1, VerifiedUntil: end},
		&TaskBudgetRow{TenantID: 7, RunID: "r1", LimitMicro: limit, Deadline: end, Version: 1},
		&BudgetLotRow{TenantID: 7, LotID: "lot1", RemainingMicro: verified, ExpiresAt: &expiry, IssuedAt: time.Now().UTC()},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func budgetRequest(run, key string, upper domain.Credits) domain.BudgetRequest {
	return domain.BudgetRequest{TenantID: 7, RunID: run, Key: key, Upper: upper, Deadline: time.Now().Add(time.Minute)}
}

func budgetAccountRow(t *testing.T, db *gorm.DB) BudgetAccountRow {
	t.Helper()
	var acct BudgetAccountRow
	if err := db.Where("tenant_id = 7").First(&acct).Error; err != nil {
		t.Fatal(err)
	}
	return acct
}

func budgetTaskRow(t *testing.T, db *gorm.DB, run string) TaskBudgetRow {
	t.Helper()
	var task TaskBudgetRow
	if err := db.Where("tenant_id = 7 AND run_id = ?", run).First(&task).Error; err != nil {
		t.Fatal(err)
	}
	return task
}

// TestBudgetReserveConcurrentWinnersMatchDB: 20 concurrent reservations
// race for a balance that fits exactly one of them. The successes' total,
// the DB account hold, the task hold, the lot hold, and the real lot
// allocations must all agree — exactly one winner of exactly 60.
func TestBudgetReserveConcurrentWinnersMatchDB(t *testing.T) {
	s, db := testBudgetStore(t)
	seedBudget(t, db, 100, 100)
	var accepted atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := s.Reserve(context.Background(), budgetRequest("r1", fmt.Sprintf("call-%d", i), 60))
			if err == nil {
				accepted.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if got := accepted.Load(); got != 1 {
		t.Fatalf("accepted=%d want exactly one reservation", got)
	}
	if acct := budgetAccountRow(t, db); acct.HeldMicro != 60 {
		t.Fatalf("account held=%d want 60", acct.HeldMicro)
	}
	if task := budgetTaskRow(t, db, "r1"); task.HeldMicro != 60 {
		t.Fatalf("task held=%d want 60", task.HeldMicro)
	}
	var lot BudgetLotRow
	if err := db.Where("tenant_id = 7").First(&lot).Error; err != nil {
		t.Fatal(err)
	}
	if lot.HeldMicro != 60 {
		t.Fatalf("lot held=%d want 60", lot.HeldMicro)
	}
	var allocSum int64
	if err := db.Model(&BudgetLotAllocationRow{}).Select("COALESCE(SUM(micro),0)").Scan(&allocSum).Error; err != nil {
		t.Fatal(err)
	}
	if allocSum != 60 {
		t.Fatalf("lot allocations=%d want 60", allocSum)
	}
	var reservations int64
	if err := db.Model(&ReservationRow{}).Count(&reservations).Error; err != nil {
		t.Fatal(err)
	}
	if reservations != 1 {
		t.Fatalf("reservations=%d want 1", reservations)
	}
}

// TestBudgetReserveParentChildNoCopy: a child run charges the parent's
// single task budget. The child row owns no independent limit, so parent
// and child draws share one headroom and a budget is never copied down.
func TestBudgetReserveParentChildNoCopy(t *testing.T) {
	s, db := testBudgetStore(t)
	// The account projection is roomier than the task limit so the denial
	// under test comes from the shared TASK budget, not the account.
	seedBudget(t, db, 200, 100)
	ctx := context.Background()
	if err := s.AttachChildRun(ctx, 7, "child-run", "r1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reserve(ctx, budgetRequest("child-run", "child-call", 40)); err != nil {
		t.Fatal(err)
	}
	// 40 (child) + 70 (parent) > 100 must fail against the ONE shared budget.
	if _, err := s.Reserve(ctx, budgetRequest("r1", "parent-call", 70)); !errors.Is(err, ErrTaskBudgetExhausted) {
		t.Fatalf("got %v want ErrTaskBudgetExhausted", err)
	}
	// Exactly 60 remaining fits: exhaustion is exact, not off-by-one.
	if _, err := s.Reserve(ctx, budgetRequest("r1", "parent-call", 60)); err != nil {
		t.Fatal(err)
	}
	parent := budgetTaskRow(t, db, "r1")
	if parent.HeldMicro != 100 {
		t.Fatalf("parent held=%d want 100 (child counted once)", parent.HeldMicro)
	}
	child := budgetTaskRow(t, db, "child-run")
	if child.LimitMicro != 0 || child.HeldMicro != 0 || child.RootRunID != "r1" {
		t.Fatalf("child row must be a zero-limit mapping, got limit=%d held=%d root=%q", child.LimitMicro, child.HeldMicro, child.RootRunID)
	}
	if acct := budgetAccountRow(t, db); acct.HeldMicro != 100 {
		t.Fatalf("account held=%d want 100", acct.HeldMicro)
	}
}

// TestBudgetExternalBalanceNeverOverwritesUnreflected: an independent
// external balance read may advance verified_micro and the watermark, but
// the store never overwrites the local unreflected spend from it.
func TestBudgetExternalBalanceNeverOverwritesUnreflected(t *testing.T) {
	s, db := testBudgetStore(t)
	seedBudget(t, db, 100, 100)
	if err := db.Model(&BudgetAccountRow{}).Where("tenant_id = 7").Updates(map[string]any{"unreflected_micro": 20, "version": 2}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyExternalBalance(context.Background(), 7, 500, "w2"); err != nil {
		t.Fatal(err)
	}
	acct := budgetAccountRow(t, db)
	if acct.UnreflectedMicro != 20 {
		t.Fatalf("unreflected=%d want unchanged 20", acct.UnreflectedMicro)
	}
	if acct.VerifiedMicro != 500 || acct.Watermark != "w2" {
		t.Fatalf("verified=%d watermark=%q want 500/w2", acct.VerifiedMicro, acct.Watermark)
	}
	if acct.Version < 3 {
		t.Fatalf("version=%d want bumped", acct.Version)
	}
	if got, err := domain.Available(domain.Credits(acct.VerifiedMicro), domain.Credits(acct.UnreflectedMicro), domain.Credits(acct.HeldMicro), domain.Credits(acct.RefundLockedMicro)); err != nil || got != 480 {
		t.Fatalf("Available=%d err=%v want 480", got, err)
	}
	// A stale watermark is rejected outright.
	if err := s.ApplyExternalBalance(context.Background(), 7, 999, "w1"); !errors.Is(err, ErrStaleWatermark) {
		t.Fatalf("got %v want ErrStaleWatermark", err)
	}
}

// TestBudgetReserveReplayIsIdempotent: the same (tenant, key) replays as the
// same held reservation without double-counting, while the same key with
// different content is a conflict.
func TestBudgetReserveReplayIsIdempotent(t *testing.T) {
	s, db := testBudgetStore(t)
	seedBudget(t, db, 100, 100)
	ctx := context.Background()
	first, err := s.Reserve(ctx, budgetRequest("r1", "k", 30))
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Reserve(ctx, budgetRequest("r1", "k", 30))
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || first.Upper != second.Upper || first.Version != second.Version {
		t.Fatal("replay must return the same reservation")
	}
	if acct := budgetAccountRow(t, db); acct.HeldMicro != 30 {
		t.Fatalf("held=%d want 30 counted once", acct.HeldMicro)
	}
	if _, err := s.Reserve(ctx, budgetRequest("r1", "k", 40)); !errors.Is(err, ErrReservationKeyConflict) {
		t.Fatalf("got %v want ErrReservationKeyConflict", err)
	}
	if acct := budgetAccountRow(t, db); acct.HeldMicro != 30 {
		t.Fatalf("held=%d want still 30 after conflict", acct.HeldMicro)
	}
}

// TestBudgetReserveInsufficientAccountLeavesNoHold: a reservation the
// account projection cannot cover is rejected with no side effects.
func TestBudgetReserveInsufficientAccountLeavesNoHold(t *testing.T) {
	s, db := testBudgetStore(t)
	seedBudget(t, db, 50, 100)
	if _, err := s.Reserve(context.Background(), budgetRequest("r1", "call-big", 60)); !errors.Is(err, ErrInsufficientBudget) {
		t.Fatalf("got %v want ErrInsufficientBudget", err)
	}
	if acct := budgetAccountRow(t, db); acct.HeldMicro != 0 {
		t.Fatalf("held=%d want 0", acct.HeldMicro)
	}
	if task := budgetTaskRow(t, db, "r1"); task.HeldMicro != 0 {
		t.Fatalf("task held=%d want 0", task.HeldMicro)
	}
}

// TestBudgetLockRefundsCompetesWithReserve: refund locks and reservations
// draw from one projection under the same CAS discipline, so concurrent
// draws never overdraw it together.
func TestBudgetLockRefundsCompetesWithReserve(t *testing.T) {
	s, db := testBudgetStore(t)
	seedBudget(t, db, 100, 100)
	var wins atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				if err := s.LockRefunds(context.Background(), 7, 60); err == nil {
					wins.Add(1)
				}
				return
			}
			if _, err := s.Reserve(context.Background(), budgetRequest("r1", fmt.Sprintf("lock-call-%d", i), 60)); err == nil {
				wins.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if got := wins.Load(); got != 1 {
		t.Fatalf("winners=%d want exactly one of lock-or-reserve", got)
	}
	acct := budgetAccountRow(t, db)
	if got := acct.HeldMicro + acct.RefundLockedMicro; got != 60 {
		t.Fatalf("held+refund_locked=%d want 60", got)
	}
	if acct.HeldMicro != 0 && acct.HeldMicro != 60 {
		t.Fatalf("held=%d want 0 or 60", acct.HeldMicro)
	}
}

// assertBudgetNoSideEffects pins the rollback contract of a failed Reserve /
// LockRefunds: no account hold, no task hold, no reservation row, no lot
// hold, no allocation row — and the rolled-back rows keep their original
// version, proving the guarded writes were undone, not merely netted out.
func assertBudgetNoSideEffects(t *testing.T, db *gorm.DB) {
	t.Helper()
	acct := budgetAccountRow(t, db)
	if acct.HeldMicro != 0 || acct.RefundLockedMicro != 0 || acct.Version != 1 {
		t.Fatalf("account held=%d refund_locked=%d version=%d want 0/0/1 (rolled back)", acct.HeldMicro, acct.RefundLockedMicro, acct.Version)
	}
	if task := budgetTaskRow(t, db, "r1"); task.HeldMicro != 0 || task.Version != 1 {
		t.Fatalf("task held=%d version=%d want 0/1 (rolled back)", task.HeldMicro, task.Version)
	}
	var reservations int64
	if err := db.Model(&ReservationRow{}).Count(&reservations).Error; err != nil {
		t.Fatal(err)
	}
	if reservations != 0 {
		t.Fatalf("reservations=%d want 0", reservations)
	}
	var lot BudgetLotRow
	if err := db.Where("tenant_id = 7").First(&lot).Error; err != nil {
		t.Fatal(err)
	}
	if lot.HeldMicro != 0 {
		t.Fatalf("lot held=%d want 0", lot.HeldMicro)
	}
	var allocSum int64
	if err := db.Model(&BudgetLotAllocationRow{}).Select("COALESCE(SUM(micro),0)").Scan(&allocSum).Error; err != nil {
		t.Fatal(err)
	}
	if allocSum != 0 {
		t.Fatalf("lot allocations=%d want 0", allocSum)
	}
}

// TestBudgetReserveLotShortfallRollsBackAllWrites: live lot capacity below
// account availability (or an expired lot — the allocation SELECT excludes
// it, so the shortfall path is identical) must reject the reservation with a
// REAL domain error and leave zero side effects.
func TestBudgetReserveLotShortfallRollsBackAllWrites(t *testing.T) {
	s, db := testBudgetStore(t)
	seedBudget(t, db, 200, 200)
	// Account and task both cover 60, but the only lot holds 50 live credits.
	if err := db.Model(&BudgetLotRow{}).Where("tenant_id = 7 AND lot_id = ?", "lot1").Update("remaining_micro", 50).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reserve(context.Background(), budgetRequest("r1", "shortfall", 60)); !errors.Is(err, ErrBudgetLotsInsufficient) {
		t.Fatalf("got %v want ErrBudgetLotsInsufficient", err)
	}
	assertBudgetNoSideEffects(t, db)
	// An expired lot is the same shape: excluded from allocation entirely.
	past := time.Now().UTC().Add(-time.Minute)
	if err := db.Model(&BudgetLotRow{}).Where("tenant_id = 7 AND lot_id = ?", "lot1").Updates(map[string]any{"remaining_micro": 200, "expires_at": past}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reserve(context.Background(), budgetRequest("r1", "expired-lot", 60)); !errors.Is(err, ErrBudgetLotsInsufficient) {
		t.Fatalf("got %v want ErrBudgetLotsInsufficient for expired lot", err)
	}
	assertBudgetNoSideEffects(t, db)
}

// TestBudgetReserveLotGuardMissRollsBackAllWrites: a lot capacity guard miss
// (RowsAffected != 1 on the guarded lot UPDATE — live lot capacity was taken
// between the lot read and the UPDATE, i.e. lot capacity below account
// availability mid-transaction). The single-conn pool serializes whole
// transactions, so the miss is simulated deterministically with a
// RAISE(IGNORE) trigger: the guarded UPDATE affects zero rows without
// erroring, exactly like a lost concurrent hold. The whole transaction —
// account/task increments, reservation row, earlier lot holds — must roll
// back; the under-allocated reservation must never surface as success.
func TestBudgetReserveLotGuardMissRollsBackAllWrites(t *testing.T) {
	s, db := testBudgetStore(t)
	seedBudget(t, db, 200, 200)
	if err := db.Exec(`CREATE TRIGGER lot_guard_miss BEFORE UPDATE ON commercial_budget_lots
BEGIN
	SELECT RAISE(IGNORE);
END;`).Error; err != nil {
		t.Fatal(err)
	}
	_, err := s.Reserve(context.Background(), budgetRequest("r1", "guard-miss", 60))
	if err == nil {
		t.Fatal("under-allocated reservation must not be returned as success")
	}
	if errors.Is(err, errBudgetCASRetry) {
		t.Fatalf("retry sentinel leaked to API: %v", err)
	}
	if !errors.Is(err, ErrBudgetContention) {
		t.Fatalf("got %v want ErrBudgetContention after retry exhaustion", err)
	}
	assertBudgetNoSideEffects(t, db)
}

// TestBudgetReserveInsertRaceRollsBackAllWrites: a reservation INSERT that
// loses the (tenant, key) unique race mid-transaction must roll back the
// account/task increments already applied in that transaction. The race is
// simulated deterministically with a RAISE(ABORT) trigger on INSERT — the
// statement fails while the transaction stays alive, exactly like a lost
// unique-index race.
func TestBudgetReserveInsertRaceRollsBackAllWrites(t *testing.T) {
	s, db := testBudgetStore(t)
	seedBudget(t, db, 200, 200)
	if err := db.Exec(`CREATE TRIGGER reservation_insert_race BEFORE INSERT ON commercial_reservations
BEGIN
	SELECT RAISE(ABORT, 'reservation insert race');
END;`).Error; err != nil {
		t.Fatal(err)
	}
	_, err := s.Reserve(context.Background(), budgetRequest("r1", "insert-race", 60))
	if !errors.Is(err, ErrBudgetContention) {
		t.Fatalf("got %v want ErrBudgetContention after retry exhaustion", err)
	}
	if errors.Is(err, errBudgetCASRetry) {
		t.Fatalf("retry sentinel leaked to API: %v", err)
	}
	assertBudgetNoSideEffects(t, db)
}

// TestBudgetRetrySentinelNeverCrossesAPI: when an account guard misses on
// stale data but would pass on fresh data (a lost version race), the
// errBudgetCASRetry sentinel must drive the internal retry loop — Reserve
// and LockRefunds re-read and retry to exhaustion, and the caller sees a
// real domain error, never the sentinel. The version race is simulated
// deterministically: a RAISE(IGNORE) trigger makes the guarded account
// UPDATE affect zero rows without erroring, while the fresh re-read still
// sees full availability — precisely the would-pass-on-fresh-data case.
func TestBudgetRetrySentinelNeverCrossesAPI(t *testing.T) {
	s, db := testBudgetStore(t)
	seedBudget(t, db, 200, 200)
	if err := db.Exec(`CREATE TRIGGER account_version_race BEFORE UPDATE ON commercial_budget_accounts
BEGIN
	SELECT RAISE(IGNORE);
END;`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reserve(context.Background(), budgetRequest("r1", "version-race", 60)); err == nil || errors.Is(err, errBudgetCASRetry) {
		t.Fatalf("Reserve must return success or a real domain error, got %v", err)
	}
	if err := s.LockRefunds(context.Background(), 7, 60); err == nil || errors.Is(err, errBudgetCASRetry) {
		t.Fatalf("LockRefunds must return success or a real domain error, got %v", err)
	}
	assertBudgetNoSideEffects(t, db)
}

// TestBudgetReserveConcurrentSameKeyReplayIdempotent: 20 workers replaying
// the SAME (tenant, key) concurrently must all observe the one held
// reservation, with the hold counted exactly once against the account, the
// task budget, and the lot.
func TestBudgetReserveConcurrentSameKeyReplayIdempotent(t *testing.T) {
	s, db := testBudgetStore(t)
	seedBudget(t, db, 100, 100)
	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := 0
	firstID := ""
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := s.Reserve(context.Background(), budgetRequest("r1", "same-key", 30))
			if err != nil {
				t.Errorf("same-key replay: %v", err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			seen++
			if firstID == "" {
				firstID = res.ID
			} else if res.ID != firstID {
				t.Errorf("replay returned different reservation %q != %q", res.ID, firstID)
			}
		}()
	}
	wg.Wait()
	if seen != 20 {
		t.Fatalf("successful same-key replays=%d want 20", seen)
	}
	if acct := budgetAccountRow(t, db); acct.HeldMicro != 30 {
		t.Fatalf("account held=%d want 30 counted once", acct.HeldMicro)
	}
	var reservations int64
	if err := db.Model(&ReservationRow{}).Count(&reservations).Error; err != nil {
		t.Fatal(err)
	}
	if reservations != 1 {
		t.Fatalf("reservations=%d want 1", reservations)
	}
	var lot BudgetLotRow
	if err := db.Where("tenant_id = 7").First(&lot).Error; err != nil {
		t.Fatal(err)
	}
	if lot.HeldMicro != 30 {
		t.Fatalf("lot held=%d want 30 counted once", lot.HeldMicro)
	}
}
