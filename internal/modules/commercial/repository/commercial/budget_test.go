package commercial

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
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

// TestReserveSameExpiryPicksEarliestIssuedLot (#86 Task 3): two lots sharing
// one expiry — the earlier-ISSUED lot must be allocated first (the spec's
// "earliest expiry, then earliest grant" order; lot1 is the deterministic
// final tie-break). All three rows carry the EXACT same expires_at
// (normalized below — SQLite stores nanosecond-precision strings, so
// seedBudget's own clock read would otherwise break the tie by nanoseconds).
// The earliest-issued lot is deliberately named LAST in (tenant, lot_id)
// index order: SQLite serves equal ORDER BY keys in that index order, so a
// missing issued_at tie-break deterministically picks the WRONG row here.
func TestReserveSameExpiryPicksEarliestIssuedLot(t *testing.T) {
	store, db := testBudgetStore(t)
	seedBudget(t, db, 1_000_000, 1_000_000)
	exp := time.Now().UTC().Add(time.Hour)
	for _, row := range []any{
		&BudgetLotRow{TenantID: 7, LotID: "aa-later", RemainingMicro: 1_000_000,
			ExpiresAt: &exp, IssuedAt: time.Now().UTC().Add(-1 * time.Hour)},
		&BudgetLotRow{TenantID: 7, LotID: "zz-earliest", RemainingMicro: 1_000_000,
			ExpiresAt: &exp, IssuedAt: time.Now().UTC().Add(-2 * time.Hour)},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	// Normalize the seeded lot1 to the SAME instant (parameter-bound).
	if err := db.Exec(`UPDATE commercial_budget_lots SET expires_at = ? WHERE tenant_id = ? AND lot_id = ?`,
		exp, 7, "lot1").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.Reserve(context.Background(), budgetRequest("r1", "k1", 500_000)); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	var early, late, seeded BudgetLotRow
	if err := db.Where("tenant_id = ? AND lot_id = ?", 7, "zz-earliest").First(&early).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("tenant_id = ? AND lot_id = ?", 7, "aa-later").First(&late).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("tenant_id = ? AND lot_id = ?", 7, "lot1").First(&seeded).Error; err != nil {
		t.Fatal(err)
	}
	// 500_000 fits one row's capacity: it must land ENTIRELY on the
	// earliest-issued row (zz-earliest at -2h; aa-later at -1h and lot1 at
	// now both lose the tie-break).
	if early.HeldMicro != 500_000 || late.HeldMicro != 0 || seeded.HeldMicro != 0 {
		t.Fatalf("earliest-issued lot must be allocated first: early=%d late=%d lot1=%d",
			early.HeldMicro, late.HeldMicro, seeded.HeldMicro)
	}
}

// TestReserveDrainsEarliestExpiryFirst (#86 AC2): expires_at is the
// PRIMARY allocation key — a lot expiring sooner is drained first even
// when it was issued LAST (issued_at is only the tie-break; an inverted
// key order passes the same-expiry test above but dies here).
func TestReserveDrainsEarliestExpiryFirst(t *testing.T) {
	store, db := testBudgetStore(t)
	seedBudget(t, db, 1_000_000, 1_000_000)
	now := time.Now().UTC()
	sooner := now.Add(30 * time.Minute) // earliest expiry, LATEST issue
	later := now.Add(2 * time.Hour)     // latest expiry, earliest issue
	for _, row := range []any{
		&BudgetLotRow{TenantID: 7, LotID: "sooner", RemainingMicro: 1_000_000,
			ExpiresAt: &sooner, IssuedAt: now.Add(1 * time.Hour)},
		&BudgetLotRow{TenantID: 7, LotID: "later", RemainingMicro: 1_000_000,
			ExpiresAt: &later, IssuedAt: now.Add(-3 * time.Hour)},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Reserve(context.Background(), budgetRequest("r1", "k1", 500_000)); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	read := func(lotID string) BudgetLotRow {
		var row BudgetLotRow
		if err := db.Where("tenant_id = ? AND lot_id = ?", 7, lotID).First(&row).Error; err != nil {
			t.Fatal(err)
		}
		return row
	}
	// sooner (+30m) precedes the seeded lot1 (+1h) precedes later (+2h):
	// 500_000 fits one lot, so it must land ENTIRELY on "sooner".
	s, l, seeded := read("sooner"), read("later"), read("lot1")
	if s.HeldMicro != 500_000 || l.HeldMicro != 0 || seeded.HeldMicro != 0 {
		t.Fatalf("earliest-expiry lot must be drained first: sooner=%d later=%d lot1=%d",
			s.HeldMicro, l.HeldMicro, seeded.HeldMicro)
	}
}

// ---- #86 Task 3: SyncLots — the authority-batch → local-lot projection ----

// syncLotRow reads one lot row (nil-safe assertions helper).
func syncLotRow(t *testing.T, db *gorm.DB, tenantID uint64, lotID string) BudgetLotRow {
	t.Helper()
	var row BudgetLotRow
	if err := db.Where("tenant_id = ? AND lot_id = ?", tenantID, lotID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

// TestSyncLotsNeverResurrectsExpiredBatch (#86 Task 3, review-focus 3): a
// batch the authority still lists (the lazy-termination window) but whose
// expiry has passed must NOT become allocatable again — remaining collapses
// to held (in-flight holds stay endorsed; nothing new may draw on it). A
// batch ABSENT from the snapshot gets the same treatment; a never-seen
// expired batch is never inserted.
func TestSyncLotsNeverResurrectsExpiredBatch(t *testing.T) {
	store, db := testBudgetStore(t)
	now := time.Now().UTC()
	past := now.Add(-time.Hour)
	future := now.Add(24 * time.Hour)
	for _, row := range []any{
		// Expired-but-listed: the authority balance is 500, an in-flight
		// hold keeps 200.
		&BudgetLotRow{TenantID: 7, LotID: "expired-listed", RemainingMicro: 500, HeldMicro: 200,
			ExpiresAt: &past, IssuedAt: now.Add(-48 * time.Hour)},
		// Absent from the snapshot entirely (terminated): hold 100.
		&BudgetLotRow{TenantID: 7, LotID: "absent", RemainingMicro: 300, HeldMicro: 100,
			ExpiresAt: &future, IssuedAt: now.Add(-2 * time.Hour)},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	// The snapshot carries ONLY the expired batch (balance 500).
	err := store.SyncLots(context.Background(), 7, []domain.LotSyncBatch{
		{LotID: "expired-listed", RemainingMicro: 500, ExpiresAt: past, IssuedAt: now.Add(-48 * time.Hour)},
	}, now)
	if err != nil {
		t.Fatalf("SyncLots: %v", err)
	}
	listed := syncLotRow(t, db, 7, "expired-listed")
	if listed.RemainingMicro != 200 {
		t.Fatalf("expired-listed remaining = %d, want 200 (== held; never allocatable again)", listed.RemainingMicro)
	}
	absent := syncLotRow(t, db, 7, "absent")
	if absent.RemainingMicro != 100 {
		t.Fatalf("absent remaining = %d, want 100 (== held)", absent.RemainingMicro)
	}
	// A never-seen expired batch must never be inserted.
	var n int64
	db.Model(&BudgetLotRow{}).Where("tenant_id = 7").Count(&n)
	if n != 2 {
		t.Fatalf("lot rows = %d, want 2 (no insertion of unseen batches)", n)
	}
}

// TestSyncLotsNeverShrinksBelowHolds (#86 Task 3): the authority balance
// dropping below an in-flight hold never strands the hold — remaining keeps
// at least held (holds are endorsed draws, always allocatable at settle).
func TestSyncLotsNeverShrinksBelowHolds(t *testing.T) {
	store, db := testBudgetStore(t)
	now := time.Now().UTC()
	future := now.Add(24 * time.Hour)
	if err := db.Create(&BudgetLotRow{TenantID: 7, LotID: "held-heavy", RemainingMicro: 900,
		HeldMicro: 400, ExpiresAt: &future, IssuedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	err := store.SyncLots(context.Background(), 7, []domain.LotSyncBatch{
		{LotID: "held-heavy", RemainingMicro: 100, ExpiresAt: future, IssuedAt: now},
	}, now)
	if err != nil {
		t.Fatalf("SyncLots: %v", err)
	}
	row := syncLotRow(t, db, 7, "held-heavy")
	if row.RemainingMicro != 400 {
		t.Fatalf("remaining = %d, want 400 (never below held)", row.RemainingMicro)
	}
}

// TestSyncLotsTenantScoped (#86 Task 3, review-focus 5): tenant 8's sync
// never touches tenant 7's lot rows — the projection is tenant-closed.
func TestSyncLotsTenantScoped(t *testing.T) {
	store, db := testBudgetStore(t)
	now := time.Now().UTC()
	future := now.Add(24 * time.Hour)
	if err := db.Create(&BudgetLotRow{TenantID: 7, LotID: "t7-lot", RemainingMicro: 700,
		ExpiresAt: &future, IssuedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	err := store.SyncLots(context.Background(), 8, []domain.LotSyncBatch{
		{LotID: "t8-lot", RemainingMicro: 300, ExpiresAt: future, IssuedAt: now},
	}, now)
	if err != nil {
		t.Fatalf("SyncLots: %v", err)
	}
	t7 := syncLotRow(t, db, 7, "t7-lot")
	if t7.RemainingMicro != 700 {
		t.Fatalf("tenant 7 lot must be untouched, remaining = %d", t7.RemainingMicro)
	}
	t8 := syncLotRow(t, db, 8, "t8-lot")
	if t8.RemainingMicro != 300 {
		t.Fatalf("tenant 8 lot must be inserted, remaining = %d", t8.RemainingMicro)
	}
}

// TestSyncLotsInsertsAndAlignsFreshBatches (#86 Task 3): a fresh unexpired
// batch inserts (remaining = authority balance); an existing unexpired
// batch's remaining tracks the authority balance upward and identity
// columns (expires_at/issued_at) stay immutable after insert.
func TestSyncLotsInsertsAndAlignsFreshBatches(t *testing.T) {
	store, db := testBudgetStore(t)
	now := time.Now().UTC()
	future := now.Add(24 * time.Hour)
	issued := now.Add(-3 * time.Hour)
	if err := db.Create(&BudgetLotRow{TenantID: 7, LotID: "known", RemainingMicro: 100,
		ExpiresAt: &future, IssuedAt: issued}).Error; err != nil {
		t.Fatal(err)
	}
	err := store.SyncLots(context.Background(), 7, []domain.LotSyncBatch{
		{LotID: "known", RemainingMicro: 800, ExpiresAt: future, IssuedAt: issued},
		{LotID: "fresh", RemainingMicro: 500, ExpiresAt: future.Add(24 * time.Hour), IssuedAt: now},
	}, now)
	if err != nil {
		t.Fatalf("SyncLots: %v", err)
	}
	known := syncLotRow(t, db, 7, "known")
	if known.RemainingMicro != 800 {
		t.Fatalf("known remaining = %d, want 800 (tracks authority)", known.RemainingMicro)
	}
	if !known.IssuedAt.Equal(issued) {
		t.Fatalf("issued_at must be immutable after insert, got %v want %v", known.IssuedAt, issued)
	}
	fresh := syncLotRow(t, db, 7, "fresh")
	if fresh.RemainingMicro != 500 || fresh.HeldMicro != 0 {
		t.Fatalf("fresh lot = (remaining %d, held %d), want (500, 0)", fresh.RemainingMicro, fresh.HeldMicro)
	}
}

// TestSyncLotsConcurrentSqlite (#86 Task 3, the PG gate's portable
// floor): 8 goroutines racing SyncLots (a shrinking, late-expiring balance)
// against Reserves on SQLite — the same no-over-allocation invariants the
// PG integration test asserts, runnable everywhere.
func TestSyncLotsConcurrentSqlite(t *testing.T) {
	store, db := testBudgetStore(t)
	now := time.Now().UTC()
	end := now.Add(time.Hour)
	expired := now.Add(-time.Minute)
	if err := db.Exec(`INSERT INTO commercial_budget_accounts
		(tenant_id, verified_micro, unreflected_micro, held_micro, refund_locked_micro, watermark, version, verified_until)
		VALUES (?, 10000, 0, 0, 0, 'w1', 1, ?)`, 7, end).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO commercial_task_budgets
		(tenant_id, run_id, limit_micro, spent_micro, held_micro, deadline, version)
		VALUES (?, 'r1', 10000, 0, 0, ?, 1)`, 7, end).Error; err != nil {
		t.Fatal(err)
	}
	dying := expired
	if err := db.Exec(`INSERT INTO commercial_budget_lots
		(tenant_id, lot_id, remaining_micro, held_micro, expires_at, issued_at)
		VALUES (?, 'dying-lot', 5000, 0, ?, ?)`, 7, &dying, now.Add(-3*time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	live := end
	if err := db.Exec(`INSERT INTO commercial_budget_lots
		(tenant_id, lot_id, remaining_micro, held_micro, expires_at, issued_at)
		VALUES (?, 'live-lot', 10000, 0, ?, ?)`, 7, &live, now.Add(-2*time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	shrink := func(round int) {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			remaining := 10000 - int64(i*500*(round+1))
			if remaining < 0 {
				remaining = 0
			}
			exp := end
			if i == 4 {
				exp = expired
			}
			_ = store.SyncLots(context.Background(), 7, []domain.LotSyncBatch{
				{LotID: "live-lot", RemainingMicro: remaining, ExpiresAt: exp, IssuedAt: now.Add(-2 * time.Hour)},
			}, time.Now().UTC())
		}
	}
	draw := func(id int) {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			_, err := store.Reserve(context.Background(), domain.BudgetRequest{
				TenantID: 7, RunID: "r1", Key: fmt.Sprintf("race-%d-%d", id, i),
				Upper: domain.Credits(500), Deadline: time.Now().Add(time.Minute),
			})
			if err != nil && err != ErrBudgetLotsInsufficient && err != ErrInsufficientBudget && err != ErrBudgetContention {
				t.Errorf("reserve: %v", err)
				return
			}
		}
	}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		if i%2 == 0 {
			go shrink(i / 2)
		} else {
			go draw(i)
		}
	}
	wg.Wait()
	var lots []BudgetLotRow
	if err := db.Where("tenant_id = ?", 7).Find(&lots).Error; err != nil {
		t.Fatal(err)
	}
	var heldSum int64
	for _, l := range lots {
		if l.RemainingMicro < 0 {
			t.Fatalf("lot %s negative remaining %d", l.LotID, l.RemainingMicro)
		}
		if l.HeldMicro > l.RemainingMicro {
			t.Fatalf("lot %s over-allocated: held %d > remaining %d", l.LotID, l.HeldMicro, l.RemainingMicro)
		}
		if l.LotID == "dying-lot" && l.HeldMicro != 0 {
			t.Fatalf("the expired dying-lot gained allocations: held %d", l.HeldMicro)
		}
		heldSum += l.HeldMicro
	}
	var reserved int64
	if err := db.Model(&ReservationRow{}).Where("tenant_id = ?", 7).Count(&reserved).Error; err != nil {
		t.Fatal(err)
	}
	if reserved*500 > 15000 {
		t.Fatalf("over-draw: %d reservations of 500 exceed the seeded 15000", reserved)
	}
	if heldSum != reserved*500 {
		t.Fatalf("lot holds %d must equal reservations %d × 500", heldSum, reserved)
	}
}
