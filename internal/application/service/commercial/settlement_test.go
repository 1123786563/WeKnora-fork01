package commercial

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// settlementStubGateway fakes ONLY the external boundary: the remote is a
// key→transaction map that persists BEFORE answering, so a lost response
// still leaves a real transaction the next call can correlate. Benefit-side
// methods come from the embedded fulfillment stub; this file proves local
// settlement state transitions against the real SQLite stores.
type settlementStubGateway struct {
	stubGateway
	mu           sync.Mutex
	transactions map[string]string // settlement idempotency key → remote transaction id
	settleErr    error             // returned AFTER the remote persisted (dropped response)
	confirmErr   error             // confirmation response lost / failed
	confirmWM    string            // watermark the confirm answers with
	settles      int
	confirms     int
}

func (g *settlementStubGateway) Settle(_ context.Context, st domain.Settlement) (domain.SettlementReceipt, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.transactions == nil {
		g.transactions = map[string]string{}
	}
	g.settles++
	ext, ok := g.transactions[st.ID]
	if !ok {
		// The provider records the transaction under the idempotency key
		// first; the response may still be lost after this point.
		ext = "om-txn-" + st.ID[len("settle:"):]
		g.transactions[st.ID] = ext
	}
	// Ingest acceptance carries NO watermark: acceptance is not confirmation.
	return domain.SettlementReceipt{ExternalID: ext}, g.settleErr
}

func (g *settlementStubGateway) ConfirmSettlement(_ context.Context, id string) (domain.SettlementReceipt, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.confirms++
	if g.confirmErr != nil {
		return domain.SettlementReceipt{}, g.confirmErr
	}
	ext := g.transactions[id]
	if ext == "" {
		ext = "om-txn-unknown"
	}
	return domain.SettlementReceipt{ExternalID: ext, Watermark: g.confirmWM}, nil
}

func (g *settlementStubGateway) setSettleErr(err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.settleErr = err
}

func (g *settlementStubGateway) setConfirmErr(err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.confirmErr = err
}

func (g *settlementStubGateway) setConfirmWatermark(wm string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.confirmWM = wm
}

func (g *settlementStubGateway) remoteTransactionCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.transactions)
}

func (g *settlementStubGateway) settleCalls() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.settles
}

func (g *settlementStubGateway) confirmCalls() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.confirms
}

const settlementTenant = 7

// Settlement arithmetic of these scenarios: 1_000_000 verified micro, an
// 80_000-token final call at 1500 micro per 1000 tokens = 120_000 micro
// consumed, a 200_000 hold upper bound (80_000 unused released).
const (
	settleVerified = 1_000_000
	settleHold     = 200_000
	settleCharge   = 120_000
)

func testSettlementService(t *testing.T) (*SettlementService, *settlementStubGateway, *gorm.DB, *repocommercial.BudgetStore) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1) // serialize SQLite writers; claims still race logically
	}
	if err := db.AutoMigrate(
		&repocommercial.BudgetAccountRow{}, &repocommercial.TaskBudgetRow{}, &repocommercial.ReservationRow{},
		&repocommercial.BudgetLotRow{}, &repocommercial.BudgetLotAllocationRow{},
		&repocommercial.UsageRow{}, &repocommercial.UsageCurrentRow{}, &repocommercial.OutboxEvent{},
	); err != nil {
		t.Fatal(err)
	}
	gw := &settlementStubGateway{}
	budget := repocommercial.NewBudgetStore(db)
	svc, err := NewSettlementService(db, budget, gw, func(version string) (domain.PriceVersionRates, error) {
		return domain.PriceVersionRates{Version: version, Rates: map[string]domain.DimensionRate{
			domain.DimensionModel: {RateMicro: 1500, Units: 1000},
		}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc, gw, db, budget
}

func seedSettlementBudget(t *testing.T, db *gorm.DB) {
	t.Helper()
	end := time.Now().UTC().Add(time.Hour)
	expiry := end
	for _, row := range []any{
		&repocommercial.BudgetAccountRow{TenantID: settlementTenant, VerifiedMicro: settleVerified, Watermark: "w1", Version: 1, VerifiedUntil: end},
		&repocommercial.TaskBudgetRow{TenantID: settlementTenant, RunID: "r1", LimitMicro: 2_000_000, Deadline: end, Version: 1},
		&repocommercial.BudgetLotRow{TenantID: settlementTenant, LotID: "lot1", RemainingMicro: settleVerified, ExpiresAt: &expiry, IssuedAt: time.Now().UTC()},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func settlementFinalFact(call string, revision, tokens int64) domain.UsageFact {
	return domain.UsageFact{
		TenantID:     settlementTenant,
		RunID:        "r1",
		CallID:       call,
		AttemptID:    "attempt_1",
		Funding:      domain.FundingPlatform,
		Service:      "chat",
		PriceVersion: "pv-1",
		Revision:     revision,
		OccurredAt:   time.Date(2028, 3, 1, 10, 0, 0, 0, time.UTC),
		Dimensions:   map[string]int64{domain.DimensionModel: tokens},
		Status:       domain.UsageStatusFinal,
	}
}

// settledCall drives the shared prefix: reserve the upper bound, finalize
// the final fact, and get it accepted remotely.
func settledCall(t *testing.T, svc *SettlementService, budget *repocommercial.BudgetStore, call string) domain.Settlement {
	t.Helper()
	ctx := context.Background()
	resKey := "res_" + call
	if _, err := budget.Reserve(ctx, domain.BudgetRequest{
		TenantID: settlementTenant, RunID: "r1", Key: resKey,
		Upper: domain.Credits(settleHold), Deadline: time.Now().Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Finalize(ctx, settlementFinalFact(call, 1, 80_000), resKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Dispatch(ctx); err != nil {
		t.Fatal(err)
	}
	return st
}

func settlementAccount(t *testing.T, db *gorm.DB) repocommercial.BudgetAccountRow {
	t.Helper()
	var acct repocommercial.BudgetAccountRow
	if err := db.Where("tenant_id = ?", settlementTenant).First(&acct).Error; err != nil {
		t.Fatal(err)
	}
	return acct
}

func settlementTask(t *testing.T, db *gorm.DB) repocommercial.TaskBudgetRow {
	t.Helper()
	var task repocommercial.TaskBudgetRow
	if err := db.Where("tenant_id = ? AND run_id = ?", settlementTenant, "r1").First(&task).Error; err != nil {
		t.Fatal(err)
	}
	return task
}

func settlementRecord(t *testing.T, db *gorm.DB, key string) SettlementRecord {
	t.Helper()
	var rec SettlementRecord
	if err := db.Where("key = ?", key).First(&rec).Error; err != nil {
		t.Fatal(err)
	}
	return rec
}

func settlementReservationState(t *testing.T, db *gorm.DB, key string) string {
	t.Helper()
	var res repocommercial.ReservationRow
	if err := db.Where("tenant_id = ? AND key = ?", settlementTenant, key).First(&res).Error; err != nil {
		t.Fatal(err)
	}
	return res.State
}

func settlementFactCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&repocommercial.UsageRow{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func settlementEventCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&repocommercial.OutboxEvent{}).
		Where("kind = ?", repocommercial.OutboxKindUsageSettlement).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func settlementEventState(t *testing.T, db *gorm.DB, call string) string {
	t.Helper()
	var ev repocommercial.OutboxEvent
	key := fmt.Sprintf("%s:%d:%s:%s:%d", repocommercial.OutboxKindUsageSettlement, settlementTenant, call, "attempt_1", 1)
	if err := db.Where("event_key = ?", key).First(&ev).Error; err != nil {
		t.Fatal(err)
	}
	return ev.State
}

func settlementLot(t *testing.T, db *gorm.DB) repocommercial.BudgetLotRow {
	t.Helper()
	var lot repocommercial.BudgetLotRow
	if err := db.Where("tenant_id = ?", settlementTenant).First(&lot).Error; err != nil {
		t.Fatal(err)
	}
	return lot
}

func settlementAvailable(t *testing.T, acct repocommercial.BudgetAccountRow) domain.Credits {
	t.Helper()
	avail, err := domain.Available(
		domain.Credits(acct.VerifiedMicro),
		domain.Credits(acct.UnreflectedMicro),
		domain.Credits(acct.HeldMicro),
		domain.Credits(acct.RefundLockedMicro),
	)
	if err != nil {
		t.Fatal(err)
	}
	return avail
}

// TestSettlementDispatchedNotConfirmedKeepsProtection — remote aggregation
// delay: the provider accepted the transaction, but confirmation has not
// arrived. Protection must stay: the consumed part sits in unreflected, the
// watermark did not move, and availability still subtracts the spend.
func TestSettlementDispatchedNotConfirmedKeepsProtection(t *testing.T) {
	svc, gw, db, budget := testSettlementService(t)
	seedSettlementBudget(t, db)
	ctx := context.Background()

	st := settledCall(t, svc, budget, "call_delay")
	if gw.remoteTransactionCount() != 1 {
		t.Fatalf("remote transactions=%d want 1", gw.remoteTransactionCount())
	}

	rec := settlementRecord(t, db, st.ID)
	if rec.State != domain.SettlementStateAccepted {
		t.Fatalf("record state=%s want accepted", rec.State)
	}
	if !domain.KeepProtection(rec.State) {
		t.Fatal("accepted-but-unconfirmed settlement must keep protection")
	}
	if rec.ExternalID == "" {
		t.Fatal("accepted record lost its transaction correlation id")
	}

	acct := settlementAccount(t, db)
	if acct.HeldMicro != 0 {
		t.Fatalf("hold not converted: held=%d", acct.HeldMicro)
	}
	if acct.UnreflectedMicro != settleCharge {
		t.Fatalf("consumed part must sit protected in unreflected: got %d want %d", acct.UnreflectedMicro, settleCharge)
	}
	if acct.Watermark != "w1" {
		t.Fatalf("watermark moved without confirmation: %s", acct.Watermark)
	}
	if acct.VerifiedMicro != settleVerified {
		t.Fatalf("verified changed pre-confirmation: %d", acct.VerifiedMicro)
	}
	if got := settlementAvailable(t, acct); got != domain.Credits(settleVerified-settleCharge) {
		t.Fatalf("availability must still subtract unconfirmed spend: %d", got)
	}

	task := settlementTask(t, db)
	if task.SpentMicro != settleCharge || task.HeldMicro != 0 {
		t.Fatalf("task budget mismatch: spent=%d held=%d", task.SpentMicro, task.HeldMicro)
	}
	if state := settlementReservationState(t, db, "res_call_delay"); state != domain.ReservationStateSettled {
		t.Fatalf("reservation state=%s want settled", state)
	}
	lot := settlementLot(t, db)
	if lot.HeldMicro != 0 || lot.RemainingMicro != settleVerified-settleCharge {
		t.Fatalf("lot hold not returned: held=%d remaining=%d", lot.HeldMicro, lot.RemainingMicro)
	}
	var allocs int64
	if err := db.Model(&repocommercial.BudgetLotAllocationRow{}).Count(&allocs).Error; err != nil {
		t.Fatal(err)
	}
	if allocs != 0 {
		t.Fatalf("lot allocations not deleted: %d", allocs)
	}
	_ = ctx
}

// TestSettlementAcceptedThenCrashReplaysIdempotent — HTTP accepted then
// crash: the remote persisted the transaction but the receipt was lost. The
// record turns unknown, the event stays pending, and the replay of both
// Finalize and Dispatch is idempotent: one fact, one event, one remote
// transaction, protection counted exactly once.
func TestSettlementAcceptedThenCrashReplaysIdempotent(t *testing.T) {
	svc, gw, db, budget := testSettlementService(t)
	seedSettlementBudget(t, db)
	ctx := context.Background()

	gw.setSettleErr(fmt.Errorf("%w: connection reset after remote persisted", domain.ErrGatewayIndeterminate))
	resKey := "res_call_crash"
	if _, err := budget.Reserve(ctx, domain.BudgetRequest{
		TenantID: settlementTenant, RunID: "r1", Key: resKey,
		Upper: domain.Credits(settleHold), Deadline: time.Now().Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	fact := settlementFinalFact("call_crash", 1, 80_000)
	if _, err := svc.Finalize(ctx, fact, resKey); err != nil {
		t.Fatal(err)
	}
	if err := svc.Dispatch(ctx); err == nil {
		t.Fatal("lost settlement response must surface an error")
	}

	// The remote persisted the transaction before the response was lost.
	if gw.remoteTransactionCount() != 1 {
		t.Fatalf("remote transactions=%d want 1", gw.remoteTransactionCount())
	}
	rec := settlementRecord(t, db, domain.SettlementKey(settlementTenant, "call_crash", "attempt_1", 1))
	if rec.State != domain.SettlementStateUnknown {
		t.Fatalf("lost response must retain unknown, got %s", rec.State)
	}
	if state := settlementEventState(t, db, "call_crash"); state != repocommercial.OutboxStatePending {
		t.Fatalf("event must stay pending for replay, got %s", state)
	}

	// Replay Finalize: idempotent, nothing counted a second time.
	gw.setSettleErr(nil)
	if _, err := svc.Finalize(ctx, fact, resKey); err != nil {
		t.Fatal(err)
	}
	if got := settlementFactCount(t, db); got != 1 {
		t.Fatalf("fact rows=%d want 1", got)
	}
	if got := settlementEventCount(t, db); got != 1 {
		t.Fatalf("settlement events=%d want 1", got)
	}
	acct := settlementAccount(t, db)
	if acct.UnreflectedMicro != settleCharge {
		t.Fatalf("replay double-protected: unreflected=%d want %d", acct.UnreflectedMicro, settleCharge)
	}

	// Replay Dispatch: same idempotency key, same remote transaction.
	if err := svc.Dispatch(ctx); err != nil {
		t.Fatal(err)
	}
	if gw.settleCalls() != 2 {
		t.Fatalf("settle calls=%d want 2", gw.settleCalls())
	}
	if gw.remoteTransactionCount() != 1 {
		t.Fatalf("replay created a second remote transaction: %d", gw.remoteTransactionCount())
	}
	rec = settlementRecord(t, db, rec.Key)
	if rec.State != domain.SettlementStateAccepted {
		t.Fatalf("record state after replay=%s want accepted", rec.State)
	}
	acct = settlementAccount(t, db)
	if acct.UnreflectedMicro != settleCharge {
		t.Fatalf("dispatch replay double-protected: unreflected=%d", acct.UnreflectedMicro)
	}
	if acct.Watermark != "w1" || acct.VerifiedMicro != settleVerified {
		t.Fatalf("acceptance must not confirm: watermark=%s verified=%d", acct.Watermark, acct.VerifiedMicro)
	}
}

// TestSettlementConfirmationLostAdvancesWatermarkExactlyOnce — the remote
// confirmed, but the confirmation response was lost. The retry confirms
// again, and the watermark plus the protection release happen exactly once.
func TestSettlementConfirmationLostAdvancesWatermarkExactlyOnce(t *testing.T) {
	svc, gw, db, budget := testSettlementService(t)
	seedSettlementBudget(t, db)
	ctx := context.Background()

	st := settledCall(t, svc, budget, "call_clost")
	gw.setConfirmErr(fmt.Errorf("%w: confirmation response lost", domain.ErrGatewayIndeterminate))
	if _, err := svc.ConfirmSettlement(ctx, st.ID); err == nil {
		t.Fatal("lost confirmation must surface an error")
	}
	rec := settlementRecord(t, db, st.ID)
	if rec.State == domain.SettlementStateConfirmed {
		t.Fatal("lost confirmation must not confirm")
	}
	acct := settlementAccount(t, db)
	if acct.Watermark != "w1" || acct.UnreflectedMicro != settleCharge {
		t.Fatalf("lost confirmation changed state: watermark=%s unreflected=%d", acct.Watermark, acct.UnreflectedMicro)
	}

	// The response comes back on retry.
	gw.setConfirmErr(nil)
	gw.setConfirmWatermark("w9")
	receipt, err := svc.ConfirmSettlement(ctx, st.ID)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Watermark != "w9" || receipt.ExternalID == "" {
		t.Fatalf("receipt lacks correlation evidence: %+v", receipt)
	}
	acct = settlementAccount(t, db)
	if acct.Watermark != "w9" {
		t.Fatalf("watermark=%s want w9", acct.Watermark)
	}
	if acct.UnreflectedMicro != 0 {
		t.Fatalf("protection not released: unreflected=%d", acct.UnreflectedMicro)
	}
	if acct.VerifiedMicro != settleVerified-settleCharge {
		t.Fatalf("verified must drop with the release: %d", acct.VerifiedMicro)
	}
	versionOnce := acct.Version

	// A third, at-least-once retry of the SAME confirmation must be a pure
	// replay: no gateway call, no second advance.
	replay, err := svc.ConfirmSettlement(ctx, st.ID)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Watermark != "w9" {
		t.Fatalf("replay receipt watermark=%s", replay.Watermark)
	}
	if gw.confirmCalls() != 2 {
		t.Fatalf("gateway confirmations=%d want 2 (one lost + one successful; the replay must not re-confirm)", gw.confirmCalls())
	}
	acct = settlementAccount(t, db)
	if acct.Version != versionOnce || acct.Watermark != "w9" || acct.UnreflectedMicro != 0 {
		t.Fatalf("confirm replay was not exactly-once: version=%d watermark=%s unreflected=%d", acct.Version, acct.Watermark, acct.UnreflectedMicro)
	}
	rec = settlementRecord(t, db, st.ID)
	if rec.State != domain.SettlementStateConfirmed || rec.Watermark != "w9" {
		t.Fatalf("record not confirmed: %+v", rec)
	}
}

// TestSettlementWatermarkRegressionNeverRewinds — the external side answers
// with a watermark older than the account's. Nothing rewinds: the stale
// confirmation is rejected, the protection stays, the record is retained.
func TestSettlementWatermarkRegressionNeverRewinds(t *testing.T) {
	svc, gw, db, budget := testSettlementService(t)
	seedSettlementBudget(t, db)
	ctx := context.Background()

	st := settledCall(t, svc, budget, "call_regress")
	gw.setConfirmWatermark("w0") // older than the seeded w1
	if _, err := svc.ConfirmSettlement(ctx, st.ID); !errors.Is(err, repocommercial.ErrStaleWatermark) {
		t.Fatalf("stale watermark must be rejected, got %v", err)
	}

	acct := settlementAccount(t, db)
	if acct.Watermark != "w1" {
		t.Fatalf("watermark rewound to %s", acct.Watermark)
	}
	if acct.UnreflectedMicro != settleCharge {
		t.Fatalf("stale confirmation released protection: unreflected=%d", acct.UnreflectedMicro)
	}
	if acct.VerifiedMicro != settleVerified {
		t.Fatalf("stale confirmation changed verified: %d", acct.VerifiedMicro)
	}
	rec := settlementRecord(t, db, st.ID)
	if rec.State == domain.SettlementStateConfirmed {
		t.Fatal("stale confirmation must not confirm")
	}
	if rec.State == "" {
		t.Fatal("history deleted on stale confirmation")
	}
}

// TestSettlementRefundSimultaneousWithConfirmCoordinates — a refund lock and
// a confirmed release race. Both are version-CAS transactions on the same
// account row: they serialize (or retry), the refund locks exactly once, the
// release subtracts exactly once, and availability never re-admits the
// consumed credits.
func TestSettlementRefundSimultaneousWithConfirmCoordinates(t *testing.T) {
	svc, gw, db, budget := testSettlementService(t)
	seedSettlementBudget(t, db)
	ctx := context.Background()

	st := settledCall(t, svc, budget, "call_refund")
	gw.setConfirmWatermark("w5")

	const refund = 50_000
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs <- budget.LockRefunds(ctx, settlementTenant, domain.Credits(refund))
	}()
	go func() {
		defer wg.Done()
		_, err := svc.ConfirmSettlement(ctx, st.ID)
		errs <- err
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	acct := settlementAccount(t, db)
	if acct.RefundLockedMicro != refund {
		t.Fatalf("refund locked=%d want exactly %d (no double subtract)", acct.RefundLockedMicro, refund)
	}
	if acct.UnreflectedMicro != 0 {
		t.Fatalf("protection not released exactly once: unreflected=%d", acct.UnreflectedMicro)
	}
	if acct.VerifiedMicro != settleVerified-settleCharge {
		t.Fatalf("verified=%d want %d (consumed subtracted once)", acct.VerifiedMicro, settleVerified-settleCharge)
	}
	if acct.HeldMicro != 0 {
		t.Fatalf("held=%d want 0", acct.HeldMicro)
	}
	if acct.Watermark != "w5" {
		t.Fatalf("watermark=%s want w5", acct.Watermark)
	}
	if got := settlementAvailable(t, acct); got != domain.Credits(settleVerified-settleCharge-refund) {
		t.Fatalf("availability re-admitted or double-subtracted credits: got %d", got)
	}
	rec := settlementRecord(t, db, st.ID)
	if rec.State != domain.SettlementStateConfirmed {
		t.Fatalf("record state=%s want confirmed", rec.State)
	}
}

// The benefit-side fulfillment stub (fulfillment_test.go) gains the
// settlement surface the CommercialGateway interface grew in U03. Benefit
// tests never drive settlements, so the stub answers with the domain's
// indeterminate error: a settlement the remote boundary never recorded.
func (g *stubGateway) Settle(_ context.Context, st domain.Settlement) (domain.SettlementReceipt, error) {
	return domain.SettlementReceipt{}, fmt.Errorf("%w: no settlement at the benefit boundary", domain.ErrGatewayIndeterminate)
}

func (g *stubGateway) ConfirmSettlement(_ context.Context, settlementID string) (domain.SettlementReceipt, error) {
	return domain.SettlementReceipt{}, fmt.Errorf("%w: no confirmation at the benefit boundary", domain.ErrGatewayIndeterminate)
}
