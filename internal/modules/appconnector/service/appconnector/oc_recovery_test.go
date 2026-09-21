// oc_recovery_test.go drives T12: crash, unknown and settlement recovery —
// the conservative replay deadline, the OCRecovery reconciliation loop and the
// crash-injection matrix (post-Begin crash, post-claim pre-send crash,
// provider-success-then-DB-failure, duplicate Finish delivery,
// ResolveUnknown-then-settlement-failure, 409, window expiry, runtime
// migration, revocation during recovery) with the standing invariant: AT MOST
// ONE side effect per action, and unknown is never treated as free.
package appconnector

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	commsvc "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// ReplayAllowed — time boundaries (VERBATIM plan sketch first)
// ---------------------------------------------------------------------------

// TestReplayDoesNotExtendOriginalDeadline is VERBATIM from the plan: replay
// never crosses the ORIGINAL deadline and a clock running backwards never
// re-opens one.
func TestReplayDoesNotExtendOriginalDeadline(t *testing.T) {
	start := time.Unix(100, 0)
	r := appconn.OCDispatchRecord{Key: "k", RuntimeID: "r", FirstSentAt: start,
		ReplayUntil: start.Add(23*time.Hour + 50*time.Minute)}
	if ReplayAllowed(r, r.ReplayUntil) {
		t.Fatal("replay beyond deadline")
	}
	if ReplayAllowed(r, start.Add(-time.Second)) {
		t.Fatal("clock moved backwards")
	}
}

// TestReplayAllowedBoundaries pins the remaining edges of the conservative
// window: in-window is allowed, a window beyond FirstSentAt+24h is corrupt and
// never replayable, and an incomplete record never qualifies.
func TestReplayAllowedBoundaries(t *testing.T) {
	start := time.Unix(100, 0)
	base := appconn.OCDispatchRecord{Key: "k", RuntimeID: "r", FirstSentAt: start,
		ReplayUntil: start.Add(23*time.Hour + 50*time.Minute)}
	if !ReplayAllowed(base, start.Add(time.Hour)) {
		t.Fatal("in-window replay refused")
	}
	corrupt := base
	corrupt.ReplayUntil = start.Add(24*time.Hour + time.Minute)
	if ReplayAllowed(corrupt, start.Add(time.Hour)) {
		t.Fatal("corrupt window beyond 24h allowed replay")
	}
	noKey := base
	noKey.Key = ""
	if ReplayAllowed(noKey, start.Add(time.Hour)) {
		t.Fatal("record without key allowed replay")
	}
	noRuntime := base
	noRuntime.RuntimeID = ""
	if ReplayAllowed(noRuntime, start.Add(time.Hour)) {
		t.Fatal("record without runtime allowed replay")
	}
	noFirst := base
	noFirst.FirstSentAt = time.Time{}
	if ReplayAllowed(noFirst, start.Add(time.Hour)) {
		t.Fatal("record without first send allowed replay")
	}
}

// ---------------------------------------------------------------------------
// recovery test environment
// ---------------------------------------------------------------------------

// ocRecoveryEnv wires the full stack over one sqlite database carrying the
// REAL OC migrations: action store, OC store (claims + recovery), dispatcher,
// provider resolver, recording gate and guard, with tenant 7's chain seeded.
func ocRecoveryEnv(t *testing.T) (*ActionService, *stubDispatcher, *stubResolver, *stubGate, *repoappconn.OCStore, *gorm.DB) {
	t.Helper()
	db := ocLimiterDB(t)
	inst := repoappconn.NewInstallationStore(db)
	store := repoappconn.NewOCStore(db)
	ctx := context.Background()
	if err := inst.ApplyInstallation(ctx, appconn.Installation{ID: "inst-7", AppID: "app-oc", Version: "1.0.0", State: appconn.InstallationActive, TenantID: 7}, 0); err != nil {
		t.Fatal(err)
	}
	if err := inst.SaveConnection(ctx, appconn.Connection{
		ID: "conn-oc", InstallationID: "inst-7", Kind: appconn.ConnectionKindSpace,
		OwnerID: "owner", CredentialRef: "cred/x", State: appconn.ConnectionActive,
		TenantID: 7, AuthVersion: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveBinding(ctx, appconn.OCBinding{
		TenantID: 7, ConnectionID: "conn-oc", RuntimeID: "rt-1", Provider: "github",
		ExternalID: "ext-42", Alias: "alias-1", AuthVersion: 1, BindingVersion: 1, State: appconn.OCBindingActive,
	}); err != nil {
		t.Fatal(err)
	}
	disp := &stubDispatcher{outcome: DispatchOutcome{Status: appconn.ActionSucceeded, ProviderResult: "ok"}}
	res := &stubResolver{outcome: DispatchOutcome{Status: appconn.ActionSucceeded, ProviderResult: "provider:ok"}}
	gate := &stubGate{}
	svc := NewActionService(repoappconn.NewActionStore(db), &stubGuard{}, gate, disp, res)
	svc.UseOCDispatchClaims(store)
	return svc, disp, res, gate, store, db
}

// ocRecoveryFor builds the reconciliation loop over an env, with tunable
// knobs so the tests never depend on wall-clock 90s staleness.
func ocRecoveryFor(t *testing.T, svc *ActionService, store *repoappconn.OCStore) *OCRecovery {
	t.Helper()
	loop, err := NewOCRecovery(store, svc)
	if err != nil {
		t.Fatal(err)
	}
	loop.staleAfter = 90 * time.Second
	loop.settleWindow = 24 * time.Hour
	loop.resolveWindow = 24 * time.Hour
	loop.batch = 16
	return loop
}

// ocClaimFor drives the REAL atomic claim for one seeded authorized action —
// the exact post-claim state a crashed worker leaves behind.
func ocClaimFor(t *testing.T, store *repoappconn.OCStore, actionID, reservation string) appconn.OCDispatchRecord {
	t.Helper()
	rec, err := store.ClaimOCDispatch(context.Background(), appconn.OCSubject{TenantID: 7, ActorID: "alice"}, actionID, reservation)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

// ocAgeRecord rewinds a dispatch record's heartbeat/created timestamps into
// the past, simulating a worker that died age ago.
func ocAgeRecord(t *testing.T, db *gorm.DB, actionID string, age time.Duration) {
	t.Helper()
	past := time.Now().UTC().Add(-age)
	if err := db.Exec("UPDATE connector_dispatch_records SET updated_at = ?, created_at = ? WHERE action_id = ?", past, past, actionID).Error; err != nil {
		t.Fatal(err)
	}
}

func ocRecord(t *testing.T, db *gorm.DB, actionID string) repoappconn.OCDispatchRecordRow {
	t.Helper()
	var row repoappconn.OCDispatchRecordRow
	if err := db.Where("action_id = ?", actionID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

func ocActionRow(t *testing.T, db *gorm.DB, actionID string) repoappconn.ActionRow {
	t.Helper()
	var row repoappconn.ActionRow
	if err := db.Where("id = ?", actionID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

func ocDomainRecord(row repoappconn.OCDispatchRecordRow) appconn.OCDispatchRecord {
	return appconn.OCDispatchRecord{
		TenantID: row.TenantID, ActionID: row.ActionID, RuntimeID: row.RuntimeID,
		Key: row.Key, ExecutionID: row.ExecutionID, State: row.State,
		ReservationID: row.ReservationID, FirstSentAt: row.FirstSentAt,
		ReplayUntil: row.ReplayUntil, Fence: row.Fence,
	}
}

// ---------------------------------------------------------------------------
// constructor + no-op
// ---------------------------------------------------------------------------

func TestOCRecoveryConstructorRejectsMissingDependencies(t *testing.T) {
	svc, _, _, _, store, _ := ocRecoveryEnv(t)
	if _, err := NewOCRecovery(nil, svc); !errors.Is(err, ErrOCRecoveryInvalid) {
		t.Fatalf("nil store accepted: %v", err)
	}
	if _, err := NewOCRecovery(store, nil); !errors.Is(err, ErrOCRecoveryInvalid) {
		t.Fatalf("nil service accepted: %v", err)
	}
}

func TestOCRecoveryRunOnceNoopWhenNothingDue(t *testing.T) {
	svc, disp, _, gate, store, _ := ocRecoveryEnv(t)
	loop := ocRecoveryFor(t, svc, store)
	if err := loop.RunOnce(context.Background()); err != nil {
		t.Fatalf("idle loop errored: %v", err)
	}
	if calls, _ := disp.stats(); calls != 0 {
		t.Fatalf("idle loop dispatched %d times", calls)
	}
	if atomic.LoadInt32(&gate.finishes) != 0 {
		t.Fatalf("idle loop settled %d times", gate.finishes)
	}
}

// ---------------------------------------------------------------------------
// crash matrix
// ---------------------------------------------------------------------------

// TestOCRecoveryPostBeginCrashNeverSettlesOrDispatches: a worker that died
// between Begin and the claim leaves an AUTHORIZED action and an orphan
// commercial hold — recovery invents no settlement (the pre-allocation rides
// the EXISTING commercial recovery), sends nothing and touches nothing.
func TestOCRecoveryPostBeginCrashNeverSettlesOrDispatches(t *testing.T) {
	svc, disp, _, gate, store, db := ocRecoveryEnv(t)
	svc.unknown = nil
	ocSeedAuthorizedOCAction(t, db, "act-begin")
	// The dead worker's Begin: an orphan hold nobody will finish.
	if _, err := gate.Begin(context.Background(), commercial.BudgetRequest{
		TenantID: 7, RunID: "appaction:act-begin", Key: "act-begin",
		Upper: 1, Deadline: time.Now().Add(10 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	loop := ocRecoveryFor(t, svc, store)
	if err := loop.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&gate.finishes) != 0 {
		t.Fatalf("recovery settled an undispatched hold: %d", gate.finishes)
	}
	if calls, _ := disp.stats(); calls != 0 {
		t.Fatalf("recovery dispatched %d times", calls)
	}
	if row := ocActionRow(t, db, "act-begin"); row.State != appconn.ActionAuthorized {
		t.Fatalf("authorized action mutated: %s", row.State)
	}
	var records int64
	db.Table("connector_dispatch_records").Where("tenant_id = ?", 7).Count(&records)
	if records != 0 {
		t.Fatalf("recovery invented %d dispatch records", records)
	}
}

// TestOCRecoveryPostClaimPreSendCrashParksUnknown: the worker died after the
// claim committed but before the POST left the process. After the 90s
// no-heartbeat window the fence CAS parks the record AND the action in
// unknown — no replay, no settlement (unknown is not free), and the approval
// stays consumed exactly once by the original claim.
func TestOCRecoveryPostClaimPreSendCrashParksUnknown(t *testing.T) {
	svc, disp, _, gate, store, db := ocRecoveryEnv(t)
	svc.unknown = nil // parking only: no provider query in this pass
	ocSeedAuthorizedOCAction(t, db, "act-crash1")
	rec := ocClaimFor(t, store, "act-crash1", "res-crash1")
	ocAgeRecord(t, db, "act-crash1", 120*time.Second)
	// A FRESH claim must NOT be swept: the 90s heartbeat rule protects it.
	ocSeedAuthorizedOCAction(t, db, "act-fresh")
	ocClaimFor(t, store, "act-fresh", "res-fresh")

	loop := ocRecoveryFor(t, svc, store)
	if err := loop.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	swept := ocRecord(t, db, "act-crash1")
	if swept.State != appconn.ActionUnknown {
		t.Fatalf("stale record state = %s, want unknown", swept.State)
	}
	if swept.Fence != rec.Fence {
		t.Fatalf("sweep moved the fence: %d -> %d", rec.Fence, swept.Fence)
	}
	if row := ocActionRow(t, db, "act-crash1"); row.State != appconn.ActionUnknown {
		t.Fatalf("stale action state = %s, want unknown", row.State)
	}
	fresh := ocRecord(t, db, "act-fresh")
	if fresh.State != appconn.ActionDispatched {
		t.Fatalf("fresh record swept: %s", fresh.State)
	}
	if calls, _ := disp.stats(); calls != 0 {
		t.Fatalf("recovery re-sent a possibly-side-effected request: %d POSTs", calls)
	}
	if atomic.LoadInt32(&gate.finishes) != 0 {
		t.Fatalf("unknown settled as free: %d finishes", gate.finishes)
	}
}

// TestExecuteProviderSuccessDBFailureRetriesOriginalResult: the provider
// answered but the local finish write failed. A bounded retry persists the
// ORIGINAL result; the provider is never called twice.
func TestExecuteProviderSuccessDBFailureRetriesOriginalResult(t *testing.T) {
	svc, disp, _, gate, store, db := ocRecoveryEnv(t)
	ocSeedAuthorizedOCAction(t, db, "act-dberr")
	loop := ocRecoveryFor(t, svc, store)

	var hits int32
	db.Callback().Update().Before("gorm:update").Register("test/fail_record_finish", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "connector_dispatch_records" &&
			atomic.AddInt32(&hits, 1) <= 2 {
			_ = tx.AddError(errors.New("injected finish failure"))
		}
	})
	defer db.Callback().Update().Remove("test/fail_record_finish")

	if err := svc.Execute(context.Background(), "act-dberr"); err != nil {
		t.Fatalf("bounded retry did not absorb the failure: %v", err)
	}
	rec := ocRecord(t, db, "act-dberr")
	if rec.State != appconn.ActionSucceeded {
		t.Fatalf("record state = %s, want the ORIGINAL succeeded", rec.State)
	}
	if row := ocActionRow(t, db, "act-dberr"); row.State != appconn.ActionSucceeded {
		t.Fatalf("action state = %s, want succeeded", row.State)
	}
	if calls, _ := disp.stats(); calls != 1 {
		t.Fatalf("provider called %d times, want exactly 1", calls)
	}
	if atomic.LoadInt32(&gate.finishes) != 1 {
		t.Fatalf("settlements = %d, want 1", gate.finishes)
	}
	if atomic.LoadInt32(&hits) != 4 {
		t.Fatalf("write attempts = %d, want 4 (2 failed finishes + 1 persisted + 1 delivery mark)", hits)
	}
	// The delivered settlement is marked done: a later loop pass stays quiet.
	if err := loop.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&gate.finishes) != 1 {
		t.Fatalf("marked-done record resettled: %d", gate.finishes)
	}
}

// TestExecuteProviderSuccessDBFailureExhaustsToUnknown: when every retry of
// the local finish fails, the process-equivalent outcome is "crash after the
// provider call" — the result stays unrecorded, NOTHING is re-sent, and the
// stale sweep later parks it unknown.
func TestExecuteProviderSuccessDBFailureExhaustsToUnknown(t *testing.T) {
	svc, disp, _, gate, store, db := ocRecoveryEnv(t)
	svc.unknown = nil
	ocSeedAuthorizedOCAction(t, db, "act-exhaust")
	loop := ocRecoveryFor(t, svc, store)

	db.Callback().Update().Before("gorm:update").Register("test/fail_all_finish", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "connector_dispatch_records" {
			_ = tx.AddError(errors.New("injected finish failure"))
		}
	})
	err := svc.Execute(context.Background(), "act-exhaust")
	db.Callback().Update().Remove("test/fail_all_finish")
	if err == nil {
		t.Fatal("exhausted retries reported success")
	}
	if calls, _ := disp.stats(); calls != 1 {
		t.Fatalf("provider called %d times, want exactly 1", calls)
	}
	if atomic.LoadInt32(&gate.finishes) != 0 {
		t.Fatalf("unsettled outcome charged: %d", gate.finishes)
	}
	if rec := ocRecord(t, db, "act-exhaust"); rec.State != appconn.ActionDispatched {
		t.Fatalf("record left dispatched? got %s", rec.State)
	}
	// Crash-equivalent: no heartbeat. The sweep converges it to unknown.
	ocAgeRecord(t, db, "act-exhaust", 120*time.Second)
	if err := loop.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rec := ocRecord(t, db, "act-exhaust"); rec.State != appconn.ActionUnknown {
		t.Fatalf("post-exhaustion state = %s, want unknown", rec.State)
	}
	if calls, _ := disp.stats(); calls != 1 {
		t.Fatalf("recovery re-sent after exhaustion: %d POSTs", calls)
	}
	if atomic.LoadInt32(&gate.finishes) != 0 {
		t.Fatalf("unknown settled as free: %d", gate.finishes)
	}
}

// TestOCRecoverySettlesTerminalRecordAfterCrash: the terminal result and the
// settlement intent were durably recorded, but the worker died before
// delivering the settlement. The loop delivers it ONCE with the stable fact,
// marks the record settled, and a second pass stays quiet. The gate context
// carries a deadline (bounded recovery contexts, T11-QF-3).
func TestOCRecoverySettlesTerminalRecordAfterCrash(t *testing.T) {
	svc, disp, _, _, store, db := ocRecoveryEnv(t)
	svc.unknown = nil
	ocSeedAuthorizedOCAction(t, db, "act-term")
	ocClaimFor(t, store, "act-term", "res-term")
	// Worker finished the record but died before the action write and gate
	// delivery (the action row keeps the claim's dispatched state).
	if err := store.FinishOCDispatch(context.Background(), 7, "act-term", 1, appconn.ActionDispatched, appconn.ActionSucceeded, ""); err != nil {
		t.Fatal(err)
	}
	gate := &deadlineGate{inner: &stubGate{}}
	svc.gate = gate
	loop := ocRecoveryFor(t, svc, store)

	if err := loop.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gate.finishes() != 1 {
		t.Fatalf("settlement deliveries = %d, want 1", gate.finishes())
	}
	if !gate.sawDeadline() {
		t.Fatal("settlement delivered without a bounded context")
	}
	resID, fact := gate.lastFact()
	if resID != "res-term" || fact.Revision != 1 || fact.Dimensions[commercial.DimensionConnector] != 1 ||
		fact.CallID != "act-term" || fact.RunID != "appaction:act-term" || fact.Status != commercial.UsageStatusFinal {
		t.Fatalf("settlement fact = (%q, %+v)", resID, fact)
	}
	// The action row is re-aligned from the durable record...
	if row := ocActionRow(t, db, "act-term"); row.State != appconn.ActionSucceeded {
		t.Fatalf("action not aligned to the record: %s", row.State)
	}
	// ...the delivery is durably marked: the next pass does not re-deliver.
	if err := loop.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gate.finishes() != 1 {
		t.Fatalf("duplicate delivery after mark: %d", gate.finishes())
	}
	if calls, _ := disp.stats(); calls != 0 {
		t.Fatalf("settlement recovery re-sent: %d POSTs", calls)
	}
	// A crash between delivery and mark stays safe: forcing the record back
	// into the due set replays the IDENTICAL fact (idempotent at the ledger).
	if err := db.Exec("UPDATE connector_dispatch_records SET fence = 1 WHERE action_id = 'act-term'").Error; err != nil {
		t.Fatal(err)
	}
	if err := loop.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, fact2 := gate.lastFact()
	if fact2.Revision != 1 || fact2.Dimensions[commercial.DimensionConnector] != 1 || fact2.CallID != fact.CallID {
		t.Fatalf("replayed fact diverged: %+v", fact2)
	}
}

// TestOCRecoveryDuplicateFinishKeepsLedgerConsistent is the T11-QF-5
// acceptance item: the reconcile path must tolerate a duplicate Finish of an
// ALREADY-SETTLED pre-allocation. Against the REAL commercial gate+settlement
// ledger: (a) replaying the identical final fact is an idempotent no-op (one
// usage row, no double count); (b) the release-reclaim interleave — the
// loser's zero-usage release settled revision 1 at charge 0, then the
// winner's real settlement arrives at the same revision with a different
// charge — surfaces the ledger conflict instead of silently double-counting.
func TestOCRecoveryDuplicateFinishKeepsLedgerConsistent(t *testing.T) {
	ledgerDB, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := ledgerDB.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	if err := ledgerDB.AutoMigrate(
		&repocommercial.BudgetAccountRow{}, &repocommercial.TaskBudgetRow{}, &repocommercial.ReservationRow{},
		&repocommercial.BudgetLotRow{}, &repocommercial.BudgetLotAllocationRow{},
		&repocommercial.UsageRow{}, &repocommercial.UsageCurrentRow{}, &repocommercial.OutboxEvent{},
	); err != nil {
		t.Fatal(err)
	}
	end := time.Now().UTC().Add(time.Hour)
	expiry := end
	for _, row := range []any{
		&repocommercial.BudgetAccountRow{TenantID: 7, VerifiedMicro: 1_000_000, Watermark: "w1", Version: 1, VerifiedUntil: end},
		&repocommercial.TaskBudgetRow{TenantID: 7, RunID: "appaction:act-qf5", LimitMicro: 2_000_000, Deadline: end, Version: 1},
		&repocommercial.BudgetLotRow{TenantID: 7, LotID: "lot1", RemainingMicro: 1_000_000, ExpiresAt: &expiry, IssuedAt: time.Now().UTC()},
	} {
		if err := ledgerDB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	gate, err := commsvc.NewExecutionGateService(ledgerDB, &nopGateway{})
	if err != nil {
		t.Fatal(err)
	}
	gate, err = gate.WithRates(func(version string) (commercial.PriceVersionRates, error) {
		return commercial.PriceVersionRates{Version: version, Rates: map[string]commercial.DimensionRate{
			commercial.DimensionConnector: {RateMicro: 1000, Units: 1},
		}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	budget := repocommercial.NewBudgetStore(ledgerDB)
	ctx := context.Background()
	reserve := func(key string) {
		t.Helper()
		if _, err := budget.Reserve(ctx, commercial.BudgetRequest{
			TenantID: 7, RunID: "appaction:act-qf5", Key: key,
			Upper: 2000, Deadline: end,
		}); err != nil {
			t.Fatal(err)
		}
	}
	factFor := func(call string, calls int64) commercial.UsageFact {
		return commercial.UsageFact{
			TenantID: 7, RunID: "appaction:" + call, CallID: call, AttemptID: call,
			Funding: commercial.FundingPlatform, Service: commercial.ServiceConnector, PriceVersion: "v1",
			Revision: 1, OccurredAt: time.Now().UTC(),
			Dimensions: map[string]int64{commercial.DimensionConnector: calls}, Status: commercial.UsageStatusFinal,
		}
	}

	// (a) identical-fact duplicate delivery is an idempotent no-op.
	reserve("res-qf5")
	if err := gate.Finish(ctx, "res-qf5", factFor("act-qf5", 1)); err != nil {
		t.Fatalf("first settlement: %v", err)
	}
	if err := gate.Finish(ctx, "res-qf5", factFor("act-qf5", 1)); err != nil {
		t.Fatalf("identical duplicate Finish not idempotent: %v", err)
	}
	var rows int64
	ledgerDB.Model(&repocommercial.UsageRow{}).Where("call_id = ?", "act-qf5").Count(&rows)
	if rows != 1 {
		t.Fatalf("usage rows = %d, want exactly 1", rows)
	}

	// (b) release-reclaim interleave: revision 1 settled at charge 0 by the
	// loser's zero-release; the winner's dims-1 fact at the same revision must
	// surface the conflict, never double-count.
	reserve("res-qf5b")
	if err := gate.Finish(ctx, "res-qf5b", factFor("act-qf5b", 0)); err != nil {
		t.Fatalf("zero-usage release: %v", err)
	}
	if err := gate.Finish(ctx, "res-qf5b", factFor("act-qf5b", 1)); err == nil {
		t.Fatal("conflicting revision-1 fact silently accepted")
	}
	var rowsB int64
	ledgerDB.Model(&repocommercial.UsageRow{}).Where("call_id = ?", "act-qf5b").Count(&rowsB)
	if rowsB != 1 {
		t.Fatalf("usage rows after conflict = %d, want 1 (no double count)", rowsB)
	}
}

// TestOCRecoveryResolveUnknownThenSettleFailureRetriesSettlementOnly: the
// provider query resolved the unknown to a terminal result, but the
// settlement delivery failed. The terminal writes are durable; the loop
// retries the SETTLEMENT ONLY — no second provider query, no re-POST.
func TestOCRecoveryResolveUnknownThenSettleFailureRetriesSettlementOnly(t *testing.T) {
	svc, disp, res, gate, store, db := ocRecoveryEnv(t)
	ocSeedAuthorizedOCAction(t, db, "act-resu")
	ocClaimFor(t, store, "act-resu", "res-resu")
	ocAgeRecord(t, db, "act-resu", 120*time.Second)
	svc.unknown = nil // park first, without resolving in the same pass
	loop := ocRecoveryFor(t, svc, store)
	if err := loop.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	} // record + action parked unknown

	// Settlement delivery fails on the resolve path...
	svc.unknown = res
	gate.finishErr = errors.New("ledger down")
	if err := svc.ResolveUnknown(context.Background(), "act-resu"); err == nil {
		t.Fatal("settlement failure swallowed")
	}
	if rec := ocRecord(t, db, "act-resu"); rec.State != appconn.ActionSucceeded {
		t.Fatalf("terminal record not durable: %s", rec.State)
	}
	if row := ocActionRow(t, db, "act-resu"); row.State != appconn.ActionSucceeded {
		t.Fatalf("terminal action not durable: %s", row.State)
	}
	if queries := atomic.LoadInt32(&res.calls); queries != 1 {
		t.Fatalf("provider queries = %d, want exactly 1", queries)
	}
	// ...and the loop retries the settlement only.
	gate.finishErr = nil
	if err := loop.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&gate.finishes) != 1 {
		t.Fatalf("settlements = %d, want exactly 1", gate.finishes)
	}
	if atomic.LoadInt32(&res.calls) != 1 {
		t.Fatalf("settlement retry re-queried the provider: %d", res.calls)
	}
	if calls, _ := disp.stats(); calls != 0 {
		t.Fatalf("settlement retry re-POSTed: %d", calls)
	}
	if err := loop.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&gate.finishes) != 1 {
		t.Fatalf("resolved record resettled: %d", gate.finishes)
	}
}

// TestOCRecoveryUnknown409NeverReplays: a 409-classified outcome parks as
// unknown. Even while the replay window is still OPEN, phase one never
// re-POSTs — ReplayAllowed is a permission predicate, not an actor.
func TestOCRecoveryUnknown409NeverReplays(t *testing.T) {
	svc, disp, _, gate, store, db := ocRecoveryEnv(t)
	svc.unknown = nil
	ocSeedAuthorizedOCAction(t, db, "act-409")
	ocClaimFor(t, store, "act-409", "res-409")
	if err := db.Exec("UPDATE connector_dispatch_records SET state = 'unknown' WHERE action_id = 'act-409'").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE app_actions SET state = 'unknown' WHERE id = 'act-409'").Error; err != nil {
		t.Fatal(err)
	}
	if rec := ocRecord(t, db, "act-409"); !ReplayAllowed(ocDomainRecord(rec), time.Now().UTC()) {
		t.Fatal("fixture not in-window; test drift")
	}
	loop := ocRecoveryFor(t, svc, store)
	if err := loop.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls, _ := disp.stats(); calls != 0 {
		t.Fatalf("409 unknown re-POSTed: %d", calls)
	}
	if atomic.LoadInt32(&gate.finishes) != 0 {
		t.Fatalf("409 unknown settled: %d", gate.finishes)
	}
	if rec := ocRecord(t, db, "act-409"); rec.State != appconn.ActionUnknown {
		t.Fatalf("409 unknown mutated: %s", rec.State)
	}
}

// TestOCRecoveryExpiredWindowRecordNeverReplayed: once the original replay
// deadline has passed, the record is beyond ANY replay; the stale sweep still
// converges it to unknown and a provider QUERY (read-only) may resolve it —
// but the POST path stays closed forever.
func TestOCRecoveryExpiredWindowRecordNeverReplayed(t *testing.T) {
	svc, disp, res, gate, store, db := ocRecoveryEnv(t)
	ocSeedAuthorizedOCAction(t, db, "act-exp")
	ocClaimFor(t, store, "act-exp", "res-exp")
	past := time.Now().UTC().Add(-25 * time.Hour)
	if err := db.Exec("UPDATE connector_dispatch_records SET updated_at = ?, first_sent_at = ?, replay_until = ? WHERE action_id = 'act-exp'",
		past.Add(-2*time.Minute), past, past.Add(23*time.Hour+50*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if rec := ocRecord(t, db, "act-exp"); ReplayAllowed(ocDomainRecord(rec), time.Now().UTC()) {
		t.Fatal("expired window allowed replay")
	}
	loop := ocRecoveryFor(t, svc, store)
	if err := loop.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rec := ocRecord(t, db, "act-exp"); rec.State != appconn.ActionSucceeded {
		t.Fatalf("expired-window record not resolved via query: %s", rec.State)
	}
	if calls, _ := disp.stats(); calls != 0 {
		t.Fatalf("expired window re-POSTed: %d", calls)
	}
	if atomic.LoadInt32(&res.calls) != 1 {
		t.Fatalf("provider queries = %d, want 1 (read-only resolution)", res.calls)
	}
	if atomic.LoadInt32(&gate.finishes) != 1 {
		t.Fatalf("post-resolution settlements = %d, want 1", gate.finishes)
	}
}

// TestOCRecoveryRuntimeMigrationNeverReplays: the binding was re-bound to a
// NEW runtime after the dispatch; the record stays pinned to the old one.
// Recovery never re-sends through the new runtime — only the read-only
// provider query may resolve the outcome.
func TestOCRecoveryRuntimeMigrationNeverReplays(t *testing.T) {
	svc, disp, res, gate, store, db := ocRecoveryEnv(t)
	ocSeedAuthorizedOCAction(t, db, "act-mig")
	ocClaimFor(t, store, "act-mig", "res-mig")
	ocAgeRecord(t, db, "act-mig", 120*time.Second)
	// The connection migrates to a new runtime: new binding generation.
	if err := store.SaveBinding(context.Background(), appconn.OCBinding{
		TenantID: 7, ConnectionID: "conn-oc", RuntimeID: "rt-2", Provider: "github",
		ExternalID: "ext-42", Alias: "alias-1", AuthVersion: 1, BindingVersion: 2, State: appconn.OCBindingActive,
	}); err != nil {
		t.Fatal(err)
	}
	loop := ocRecoveryFor(t, svc, store)
	if err := loop.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	rec := ocRecord(t, db, "act-mig")
	if rec.State != appconn.ActionSucceeded || rec.RuntimeID != "rt-1" {
		t.Fatalf("migrated record = %+v", rec)
	}
	if calls, _ := disp.stats(); calls != 0 {
		t.Fatalf("runtime migration caused a re-POST: %d", calls)
	}
	if atomic.LoadInt32(&res.calls) != 1 {
		t.Fatalf("provider queries = %d, want 1 (read-only)", res.calls)
	}
	if atomic.LoadInt32(&gate.finishes) != 1 {
		t.Fatalf("settlements = %d, want 1", gate.finishes)
	}
}

// TestOCRecoveryRevocationDuringRecoveryStillSettles: revoking the connection
// while an outcome is pending must not freeze the compensation — the provider
// query is read-only, the terminal writes and the settlement complete, and no
// new external write ever happens on the revoked connection.
func TestOCRecoveryRevocationDuringRecoveryStillSettles(t *testing.T) {
	svc, disp, res, gate, store, db := ocRecoveryEnv(t)
	ocSeedAuthorizedOCAction(t, db, "act-rev")
	ocClaimFor(t, store, "act-rev", "res-rev")
	ocAgeRecord(t, db, "act-rev", 120*time.Second)
	if err := store.SaveBinding(context.Background(), appconn.OCBinding{
		TenantID: 7, ConnectionID: "conn-oc", RuntimeID: "rt-1", Provider: "github",
		ExternalID: "ext-42", Alias: "alias-1", AuthVersion: 2, BindingVersion: 2, State: appconn.OCBindingRevoked,
	}); err != nil {
		t.Fatal(err)
	}
	loop := ocRecoveryFor(t, svc, store)
	if err := loop.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rec := ocRecord(t, db, "act-rev"); rec.State != appconn.ActionSucceeded {
		t.Fatalf("revoked-connection record stuck: %s", rec.State)
	}
	if atomic.LoadInt32(&res.calls) != 1 || atomic.LoadInt32(&gate.finishes) != 1 {
		t.Fatalf("queries = %d settlements = %d, want 1/1", res.calls, gate.finishes)
	}
	if calls, _ := disp.stats(); calls != 0 {
		t.Fatalf("revoked connection re-POSTed: %d", calls)
	}
}

// ---------------------------------------------------------------------------
// Execute recovery hooks
// ---------------------------------------------------------------------------

// TestExecuteDispatchCallIsBoundedByClientDeadline pins the 30s client
// deadline of the stale protocol: a hung provider call converges to unknown
// under the bounded context instead of pinning the action forever.
func TestExecuteDispatchCallIsBoundedByClientDeadline(t *testing.T) {
	svc, _, _, _, _, db := ocRecoveryEnv(t)
	ocSeedAuthorizedOCAction(t, db, "act-hang")
	svc.dispatchDeadline = 150 * time.Millisecond
	hung := &stubHungDispatcher{}
	svc.dispatcher = hung

	err := svc.Execute(context.Background(), "act-hang")
	if !errors.Is(err, ErrDispatchUnknown) {
		t.Fatalf("hung dispatch error = %v, want ErrDispatchUnknown", err)
	}
	if !hung.deadlineSeen {
		t.Fatal("dispatch call carried no deadline")
	}
	if rec := ocRecord(t, db, "act-hang"); rec.State != appconn.ActionUnknown {
		t.Fatalf("hung record state = %s, want unknown", rec.State)
	}
	if row := ocActionRow(t, db, "act-hang"); row.State != appconn.ActionUnknown {
		t.Fatalf("hung action state = %s, want unknown", row.State)
	}
}

// TestExecuteOCRecordCarriesTerminalOutcomeAndSingleSettlement drives the
// full happy path through the recovery hooks: the durable record settles to
// the provider outcome (the settlement intent is durable IN the same row),
// the action row follows, and exactly one settlement is delivered.
func TestExecuteOCRecordCarriesTerminalOutcomeAndSingleSettlement(t *testing.T) {
	svc, disp, _, gate, store, db := ocRecoveryEnv(t)
	ocSeedAuthorizedOCAction(t, db, "act-happy")
	if err := svc.Execute(context.Background(), "act-happy"); err != nil {
		t.Fatal(err)
	}
	rec := ocRecord(t, db, "act-happy")
	if rec.State != appconn.ActionSucceeded || rec.ReservationID == "" {
		t.Fatalf("record = %+v", rec)
	}
	if row := ocActionRow(t, db, "act-happy"); row.State != appconn.ActionSucceeded {
		t.Fatalf("action = %s", row.State)
	}
	if calls, _ := disp.stats(); calls != 1 {
		t.Fatalf("POSTs = %d", calls)
	}
	if atomic.LoadInt32(&gate.finishes) != 1 {
		t.Fatalf("settlements = %d", gate.finishes)
	}
	loop := ocRecoveryFor(t, svc, store)
	if err := loop.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&gate.finishes) != 1 {
		t.Fatalf("happy path resettled: %d", gate.finishes)
	}
}

// TestResolveUnknownSettlesNativePreAllocation closes precheck fact 6 on the
// native side too: an unknown->terminal transition settles the pre-allocation
// (revision 1, connector_calls 1) — resolution without settlement is gone.
func TestResolveUnknownSettlesNativePreAllocation(t *testing.T) {
	db := openActionDB(t, memDSN(t))
	gate := &stubGate{}
	disp := &stubDispatcher{outcome: DispatchOutcome{Status: appconn.ActionUnknown, ProviderResult: "unprovable"}}
	res := &stubResolver{outcome: DispatchOutcome{Status: appconn.ActionSucceeded, ProviderResult: "provider:ok"}}
	svc := NewActionService(repoappconn.NewActionStore(db), &stubGuard{}, gate, disp, res)
	ctx := context.Background()
	id, err := svc.Prepare(ctx, sendAction())
	if err != nil {
		t.Fatal(err)
	}
	row, _ := svc.store.FindAction(ctx, id)
	if err := svc.Approve(ctx, id, "boss", row.ArgsDigest); err != nil {
		t.Fatal(err)
	}
	if err := svc.Execute(ctx, id); !errors.Is(err, ErrDispatchUnknown) {
		t.Fatalf("execute error = %v, want unknown", err)
	}
	if atomic.LoadInt32(&gate.finishes) != 0 {
		t.Fatal("unknown settled as free")
	}
	// Park the pre-allocation the way the native claim path records it.
	if err := db.Exec("UPDATE app_actions SET reservation_id = 'res-native' WHERE id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.ResolveUnknown(ctx, id); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&gate.finishes) != 1 {
		t.Fatalf("resolve settlements = %d, want 1", gate.finishes)
	}
	_, fact := gate.lastFinish()
	if fact.Revision != 1 || fact.Dimensions[commercial.DimensionConnector] != 1 ||
		fact.CallID != id || fact.Status != commercial.UsageStatusFinal {
		t.Fatalf("resolve fact = %+v", fact)
	}
}

// TestOCRecoveryAlignedRecordMarksSettlementDelivered is the QF-2 regression:
// the alignment pass (action unknown, record terminal — a resolver that
// crashed between its two writes) delivers the settlement AND marks it, so
// the SAME RunOnce's settlement pass does not re-deliver.
func TestOCRecoveryAlignedRecordMarksSettlementDelivered(t *testing.T) {
	svc, _, _, gate, store, db := ocRecoveryEnv(t)
	svc.unknown = nil
	ocSeedAuthorizedOCAction(t, db, "act-qf2")
	ocClaimFor(t, store, "act-qf2", "res-qf2")
	// The resolver settled the record but died before the action write; the
	// action row sits in unknown from an earlier sweep.
	if err := store.FinishOCDispatch(context.Background(), 7, "act-qf2", 1, appconn.ActionDispatched, appconn.ActionSucceeded, ""); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE app_actions SET state = 'unknown' WHERE id = 'act-qf2'").Error; err != nil {
		t.Fatal(err)
	}
	loop := ocRecoveryFor(t, svc, store)
	if err := loop.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&gate.finishes) != 1 {
		t.Fatalf("settlement deliveries = %d, want exactly 1 (align delivers + marks)", gate.finishes)
	}
	if row := ocActionRow(t, db, "act-qf2"); row.State != appconn.ActionSucceeded {
		t.Fatalf("action not aligned: %s", row.State)
	}
	if rec := ocRecord(t, db, "act-qf2"); rec.Fence != 2 {
		t.Fatalf("delivery not marked: record fence = %d, want 2", rec.Fence)
	}
	// And the next pass stays quiet.
	if err := loop.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&gate.finishes) != 1 {
		t.Fatalf("marked record resettled: %d", gate.finishes)
	}
}

// TestExecuteOCSettlesThroughRealGate drives the FULL worker-inline delivery
// against the REAL commercial gate (QF-1 integration): Begin marks the
// reservation dispatched, and Execute's settlement delivery must settle it —
// one usage fact row, hold converted, reservation settled — not fail with
// reservation_key_conflict behind a stub.
func TestExecuteOCSettlesThroughRealGate(t *testing.T) {
	svc, disp, _, _, store, db := ocRecoveryEnv(t)
	// The real gate shares the OC database: commercial tables coexist.
	if err := db.AutoMigrate(
		&repocommercial.BudgetAccountRow{}, &repocommercial.TaskBudgetRow{}, &repocommercial.ReservationRow{},
		&repocommercial.BudgetLotRow{}, &repocommercial.BudgetLotAllocationRow{},
		&repocommercial.UsageRow{}, &repocommercial.UsageCurrentRow{}, &repocommercial.OutboxEvent{},
	); err != nil {
		t.Fatal(err)
	}
	end := time.Now().UTC().Add(time.Hour)
	expiry := end
	for _, row := range []any{
		&repocommercial.BudgetAccountRow{TenantID: 7, VerifiedMicro: 1_000_000, Watermark: "w1", Version: 1, VerifiedUntil: end},
		&repocommercial.TaskBudgetRow{TenantID: 7, RunID: "appaction:act-real", LimitMicro: 2_000_000, Deadline: end, Version: 1},
		&repocommercial.BudgetLotRow{TenantID: 7, LotID: "lot1", RemainingMicro: 1_000_000, ExpiresAt: &expiry, IssuedAt: time.Now().UTC()},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	gate, err := commsvc.NewExecutionGateService(db, &nopGateway{})
	if err != nil {
		t.Fatal(err)
	}
	gate, err = gate.WithRates(func(version string) (commercial.PriceVersionRates, error) {
		return commercial.PriceVersionRates{Version: version, Rates: map[string]commercial.DimensionRate{
			// 1 micro per connector call: the service's default reservation
			// upper bound is 1 credit, and a charge above it is abnormal_cost
			// by the gate's contract — this test pins delivery, not the
			// abnormal-cost stop signal.
			commercial.DimensionConnector: {RateMicro: 1, Units: 1},
		}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	svc.gate = gate
	ocSeedAuthorizedOCAction(t, db, "act-real")

	if err := svc.Execute(context.Background(), "act-real"); err != nil {
		t.Fatalf("Execute against the real gate failed: %v", err)
	}
	var usage int64
	db.Model(&repocommercial.UsageRow{}).Where("call_id = ?", "act-real").Count(&usage)
	if usage != 1 {
		t.Fatalf("usage rows = %d, want exactly 1 (delivered through the real gate)", usage)
	}
	var res repocommercial.ReservationRow
	if err := db.Where("tenant_id = ? AND run_id = ?", 7, "appaction:act-real").First(&res).Error; err != nil {
		t.Fatal(err)
	}
	if res.State != commercial.ReservationStateSettled {
		t.Fatalf("reservation state = %q, want settled (Begin left it dispatched)", res.State)
	}
	if rec := ocRecord(t, db, "act-real"); rec.State != appconn.ActionSucceeded {
		t.Fatalf("record state = %s", rec.State)
	}
	if calls, _ := disp.stats(); calls != 1 {
		t.Fatalf("POSTs = %d, want exactly 1", calls)
	}
	// The loop finds nothing left to do.
	loop := ocRecoveryFor(t, svc, store)
	if err := loop.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	db.Model(&repocommercial.UsageRow{}).Where("call_id = ?", "act-real").Count(&usage)
	if usage != 1 {
		t.Fatalf("usage rows after loop = %d, want 1 (no duplicate delivery)", usage)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// deadlineGate wraps a gate and records whether Finish arrived under a
// context that carries a deadline (T11-QF-3: no naked contexts in recovery).
type deadlineGate struct {
	inner *stubGate
	mu    sync.Mutex
	fini  int32
	saw   bool
	res   string
	fact  commercial.UsageFact
}

func (g *deadlineGate) Begin(ctx context.Context, req commercial.BudgetRequest) (commercial.Reservation, error) {
	return g.inner.Begin(ctx, req)
}

func (g *deadlineGate) Finish(ctx context.Context, reservationID string, fact commercial.UsageFact) error {
	g.mu.Lock()
	if _, ok := ctx.Deadline(); ok {
		g.saw = true
	}
	g.res = reservationID
	g.fact = fact
	g.mu.Unlock()
	atomic.AddInt32(&g.fini, 1)
	return g.inner.Finish(ctx, reservationID, fact)
}

func (g *deadlineGate) finishes() int { return int(atomic.LoadInt32(&g.fini)) }

func (g *deadlineGate) sawDeadline() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.saw
}

func (g *deadlineGate) lastFact() (string, commercial.UsageFact) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.res, g.fact
}

// stubHungDispatcher blocks until its context dies, recording whether the
// caller bounded it with a deadline.
type stubHungDispatcher struct {
	deadlineSeen bool
}

func (d *stubHungDispatcher) Dispatch(ctx context.Context, snap ActionSnapshot, providerKey string) (DispatchOutcome, error) {
	if _, ok := ctx.Deadline(); ok {
		d.deadlineSeen = true
	}
	<-ctx.Done()
	return DispatchOutcome{}, ctx.Err()
}

// nopGateway satisfies the commercial gateway contract for ledger-level
// tests: Finalize never calls the gateway (only the outbox drainer does), so
// every method is an inert stub.
type nopGateway struct{}

func (nopGateway) ApplyBenefit(ctx context.Context, req commercial.BenefitRequest) (commercial.BenefitReceipt, error) {
	return commercial.BenefitReceipt{}, nil
}

func (nopGateway) FindBenefit(ctx context.Context, key string) (commercial.BenefitReceipt, error) {
	return commercial.BenefitReceipt{}, nil
}

func (nopGateway) RevokeBenefit(ctx context.Context, key string, credits commercial.Credits) error {
	return nil
}

func (nopGateway) Settle(ctx context.Context, s commercial.Settlement) (commercial.SettlementReceipt, error) {
	return commercial.SettlementReceipt{}, nil
}

func (nopGateway) ConfirmSettlement(ctx context.Context, settlementID string) (commercial.SettlementReceipt, error) {
	return commercial.SettlementReceipt{}, nil
}
