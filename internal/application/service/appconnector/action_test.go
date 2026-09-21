package appconnector

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/commercial"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openActionDB(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	// Single connection serializes transactions (shared-cache SQLITE_LOCKED
	// guard, same convention as the A02 suite).
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&repoappconn.ActionRow{}, &repoappconn.ApprovalRow{}, &repoappconn.PreAuthorizationRow{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func memDSN(t *testing.T) string {
	t.Helper()
	return "file:" + t.Name() + "?mode=memory&cache=shared&_busy_timeout=5000"
}

type stubGuard struct {
	mu               sync.Mutex
	err              error
	lastSubject      appconn.OCSubject
	lastConnectionID string
	lastVersion      int64
}

func (g *stubGuard) Check(ctx context.Context, subject appconn.OCSubject, connectionID string, expectedVersion int64) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lastSubject = subject
	g.lastConnectionID = connectionID
	g.lastVersion = expectedVersion
	return g.err
}

func (g *stubGuard) lastCall() (appconn.OCSubject, string, int64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.lastSubject, g.lastConnectionID, g.lastVersion
}

type stubDispatcher struct {
	mu        sync.Mutex
	outcome   DispatchOutcome
	err       error
	calls     int
	lastArgs  []byte
	lastKey   string
	lastFence int64
	lastSnap  ActionSnapshot
	snapshots []ActionSnapshot
}

func (d *stubDispatcher) Dispatch(ctx context.Context, snap ActionSnapshot, providerKey string) (DispatchOutcome, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls++
	d.lastArgs = append([]byte(nil), snap.Args...)
	d.lastKey = providerKey
	d.lastFence = snap.Fence
	d.lastSnap = snap
	d.snapshots = append(d.snapshots, snap)
	return d.outcome, d.err
}

func (d *stubDispatcher) stats() (int, []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.calls, d.lastArgs
}

type stubResolver struct {
	outcome DispatchOutcome
	calls   int32
}

func (r *stubResolver) QueryProvider(ctx context.Context, snap ActionSnapshot, providerKey string) (DispatchOutcome, error) {
	atomic.AddInt32(&r.calls, 1)
	return r.outcome, nil
}

type stubGate struct {
	beginErr  error
	begins    int32
	finishes  int32
	finishErr error

	mu              sync.Mutex
	lastReservation string
	lastFact        commercial.UsageFact
}

func (g *stubGate) Begin(ctx context.Context, req commercial.BudgetRequest) (commercial.Reservation, error) {
	if g.beginErr != nil {
		return commercial.Reservation{}, g.beginErr
	}
	return commercial.Reservation{ID: fmt.Sprintf("res-%d", atomic.AddInt32(&g.begins, 1)), Upper: req.Upper}, nil
}

func (g *stubGate) Finish(ctx context.Context, reservationID string, fact commercial.UsageFact) error {
	if g.finishErr != nil {
		return g.finishErr
	}
	g.mu.Lock()
	g.lastReservation = reservationID
	g.lastFact = fact
	g.mu.Unlock()
	atomic.AddInt32(&g.finishes, 1)
	return nil
}

func (g *stubGate) lastFinish() (string, commercial.UsageFact) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.lastReservation, g.lastFact
}

func sendAction() appconn.Action {
	return appconn.Action{
		TenantID: 7, ActorID: "u1", ConnectionID: "conn-1", Version: "1.0.0",
		Target: "mail.send", Risk: appconn.RiskSend, AuthVersion: 1,
		Args: []byte(`{"to":"a@b.c","body":"hi"}`),
	}
}

func newService(t *testing.T, dsn string) (*ActionService, *stubDispatcher, *stubResolver, *stubGate, *stubGuard) {
	t.Helper()
	db := openActionDB(t, dsn)
	disp := &stubDispatcher{outcome: DispatchOutcome{Status: appconn.ActionSucceeded, ProviderResult: "ok"}}
	res := &stubResolver{outcome: DispatchOutcome{Status: appconn.ActionSucceeded, ProviderResult: "provider:ok"}}
	gate := &stubGate{}
	guard := &stubGuard{}
	svc := NewActionService(repoappconn.NewActionStore(db), guard, gate, disp, res)
	return svc, disp, res, gate, guard
}

// TestApprovalLifecycleHappyPath pins the full pipeline: Prepare snapshots
// normalized args under a digest; a wrong-digest approval is refused (old
// approval never authorizes new args); the matching approval authorizes
// exactly one dispatch of exactly the persisted bytes; a second execute is
// refused after the count is consumed.
func TestApprovalLifecycleHappyPath(t *testing.T) {
	svc, disp, _, gate, _ := newService(t, memDSN(t))
	ctx := context.Background()
	id, err := svc.Prepare(ctx, sendAction())
	if err != nil {
		t.Fatal(err)
	}
	store := svc.store
	row, err := store.FindAction(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if row.State != appconn.ActionAwaitingApproval {
		t.Fatalf("fresh action state=%s", row.State)
	}
	norm, _ := appconn.NormalizeArgs([]byte(`{"to":"a@b.c","body":"hi"}`))
	if row.ArgsSnapshot != string(norm) {
		t.Fatalf("snapshot %q != normalized %q", row.ArgsSnapshot, norm)
	}
	// Modified args would be a different action with a different digest.
	other, _ := svc.Prepare(ctx, func() appconn.Action { a := sendAction(); a.Args = []byte(`{"to":"a@b.c","body":"hi!"}`); return a }())
	otherRow, _ := store.FindAction(ctx, other)
	if otherRow.ArgsDigest == row.ArgsDigest {
		t.Fatal("modified args produced the same digest")
	}
	// The OLD digest must not approve the NEW action.
	if err := svc.Approve(ctx, other, "boss", row.ArgsDigest); !errors.Is(err, ErrActionDigestMismatch) {
		t.Fatalf("old digest approved new args: %v", err)
	}
	if err := svc.Approve(ctx, id, "boss", "wrong"); !errors.Is(err, ErrActionDigestMismatch) {
		t.Fatalf("wrong digest accepted: %v", err)
	}
	if err := svc.Approve(ctx, id, "boss", row.ArgsDigest); err != nil {
		t.Fatal(err)
	}
	if err := svc.Execute(ctx, id); err != nil {
		t.Fatal(err)
	}
	calls, sent := disp.stats()
	if calls != 1 {
		t.Fatalf("dispatch calls=%d", calls)
	}
	if string(sent) != string(norm) {
		t.Fatalf("provider received %q, want the approved normalized snapshot %q", sent, norm)
	}
	final, _ := store.FindAction(ctx, id)
	if final.State != appconn.ActionSucceeded || final.Fence != 1 || final.ProviderKey != "conn-1" {
		t.Fatalf("final row: %+v", final)
	}
	// The single approval count is consumed: a second execute is refused
	// and nothing is dispatched again.
	if err := svc.Execute(ctx, id); !errors.Is(err, ErrActionState) {
		t.Fatalf("second execute: %v", err)
	}
	calls, _ = disp.stats()
	if calls != 1 {
		t.Fatalf("dispatch calls after replay=%d", calls)
	}
	if atomic.LoadInt32(&gate.finishes) != 1 {
		t.Fatalf("gate finishes=%d", gate.finishes)
	}
}

// TestExecuteChecksPersistedSubject pins the T04 wiring: the A02 re-check
// carries the PERSISTED row's tenant/actor (never the calling operator and
// never a synthetic admin identity), together with the row's connection id
// and auth_version.
func TestExecuteChecksPersistedSubject(t *testing.T) {
	svc, _, _, _, guard := newService(t, memDSN(t))
	ctx := context.Background()
	id, err := svc.Prepare(ctx, sendAction())
	if err != nil {
		t.Fatal(err)
	}
	row, err := svc.store.FindAction(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Approve(ctx, id, "boss", row.ArgsDigest); err != nil {
		t.Fatal(err)
	}
	if err := svc.Execute(ctx, id); err != nil {
		t.Fatal(err)
	}
	subject, connectionID, version := guard.lastCall()
	want := appconn.OCSubject{TenantID: 7, ActorID: "u1"}
	if subject != want || connectionID != "conn-1" || version != row.AuthVersion {
		t.Fatalf("guard saw (%+v, %s, %d), want (%+v, conn-1, %d)",
			subject, connectionID, version, want, row.AuthVersion)
	}
}

// TestPermissionRevokedBetweenApprovalAndExecute pins the A02 re-check on
// every execute: a connection revoked (or version-stale / owner-gone)
// after approval blocks dispatch before any reservation or intent.
func TestPermissionRevokedBetweenApprovalAndExecute(t *testing.T) {
	svc, disp, _, gate, guard := newService(t, memDSN(t))
	ctx := context.Background()
	id, _ := svc.Prepare(ctx, sendAction())
	row, _ := svc.store.FindAction(ctx, id)
	if err := svc.Approve(ctx, id, "boss", row.ArgsDigest); err != nil {
		t.Fatal(err)
	}
	guard.err = errors.New("connection_revoked")
	err := svc.Execute(ctx, id)
	if err == nil {
		t.Fatal("revoked connection dispatched")
	}
	if got := err.Error(); got != "a02 recheck: connection_revoked" {
		t.Fatalf("unexpected error: %v", err)
	}
	after, _ := svc.store.FindAction(ctx, id)
	if after.State != appconn.ActionAuthorized || after.ProviderKey != "" || after.Fence != 0 {
		t.Fatalf("blocked execute mutated the action: %+v", after)
	}
	if calls, _ := disp.stats(); calls != 0 {
		t.Fatalf("dispatcher called %d times", calls)
	}
	if atomic.LoadInt32(&gate.begins) != 0 {
		t.Fatal("budget reserved on a blocked dispatch")
	}
	// Recovery: permission restored → the same approval still executes.
	guard.err = nil
	if err := svc.Execute(ctx, id); err != nil {
		t.Fatal(err)
	}
}

// TestConcurrentExecuteLimitedByApprovalCount pins that one approval with
// remaining=1 authorizes exactly ONE dispatch no matter how many racers
// execute concurrently: the loser consumes nothing and dispatches nothing.
func TestConcurrentExecuteLimitedByApprovalCount(t *testing.T) {
	svc, disp, _, _, _ := newService(t, memDSN(t))
	ctx := context.Background()
	id, _ := svc.Prepare(ctx, sendAction())
	row, _ := svc.store.FindAction(ctx, id)
	if err := svc.Approve(ctx, id, "boss", row.ArgsDigest); err != nil {
		t.Fatal(err)
	}
	const racers = 8
	var wg sync.WaitGroup
	var wins int32
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := svc.Execute(ctx, id); err == nil {
				atomic.AddInt32(&wins, 1)
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("wins=%d want 1", wins)
	}
	if calls, _ := disp.stats(); calls != 1 {
		t.Fatalf("dispatch calls=%d want 1", calls)
	}
	final, _ := svc.store.FindAction(ctx, id)
	if final.State != appconn.ActionSucceeded {
		t.Fatalf("final state=%s", final.State)
	}
}

// TestPendingApprovalSurvivesRestart pins that the persisted pipeline has
// no in-memory authority: an awaiting_approval action (snapshot + digest)
// survives a full database close/reopen.
func TestPendingApprovalSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "actions.db")
	dsn := path + "?_busy_timeout=5000"
	db := openActionDB(t, dsn)
	guard := &stubGuard{}
	svc := NewActionService(repoappconn.NewActionStore(db), guard, nil, nil, nil)
	ctx := context.Background()
	id, err := svc.Prepare(ctx, sendAction())
	if err != nil {
		t.Fatal(err)
	}
	before, err := svc.store.FindAction(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	// Re-open the SAME file in a fresh store/service.
	db2 := openActionDB(t, dsn)
	svc2 := NewActionService(repoappconn.NewActionStore(db2), guard, nil, nil, nil)
	after, err := svc2.store.FindAction(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if after.State != appconn.ActionAwaitingApproval || after.ArgsDigest != before.ArgsDigest ||
		after.ArgsSnapshot != before.ArgsSnapshot || after.Fence != 0 {
		t.Fatalf("pending did not survive restart: %+v vs %+v", after, before)
	}
}

// TestFeeApprovalIsNotWriteApproval pins that a passing budget gate (fee
// approval) never authorizes a write: without a human digest approval the
// execute is blocked, and an expired approval is equally dead.
func TestFeeApprovalIsNotWriteApproval(t *testing.T) {
	svc, disp, _, gate, _ := newService(t, memDSN(t))
	ctx := context.Background()
	// Budget is healthy the whole time — fees are approvable.
	id, _ := svc.Prepare(ctx, sendAction())
	if err := svc.Execute(ctx, id); !errors.Is(err, ErrActionState) {
		t.Fatalf("fee pass authorized a write: %v", err)
	}
	if calls, _ := disp.stats(); calls != 0 {
		t.Fatal("dispatched without write approval")
	}
	if atomic.LoadInt32(&gate.begins) != 0 {
		t.Fatal("state check must precede any reservation")
	}
	// Human approval granted, then EXPIRED: the budget gate passing fees
	// still does not dispatch — the write approval itself is dead.
	row, _ := svc.store.FindAction(ctx, id)
	if err := svc.Approve(ctx, id, "boss", row.ArgsDigest); err != nil {
		t.Fatal(err)
	}
	if err := svc.store.SaveApproval(ctx, id, row.ArgsDigest, "boss", time.Now().Add(-time.Minute), 1); err != nil {
		t.Fatal(err)
	}
	if err := svc.Execute(ctx, id); err == nil {
		t.Fatal("expired approval dispatched")
	}
	if calls, _ := disp.stats(); calls != 0 {
		t.Fatal("dispatched on expired approval")
	}
	if after, _ := svc.store.FindAction(ctx, id); after.State != appconn.ActionAuthorized || after.Fence != 0 {
		t.Fatalf("expired-approval execute mutated action: %+v", after)
	}
	// A budget DENIAL is a hard stop even with a live approval.
	if err := svc.store.SaveApproval(ctx, id, row.ArgsDigest, "boss", time.Now().Add(time.Hour), 1); err != nil {
		t.Fatal(err)
	}
	gate.beginErr = commercial.ErrInsufficientBudgetGate
	if err := svc.Execute(ctx, id); err == nil {
		t.Fatal("insufficient budget dispatched")
	}
	after, _ := svc.store.FindAction(ctx, id)
	if after.State != appconn.ActionAuthorized || after.Fence != 0 || after.ProviderKey != "" {
		t.Fatalf("denied execute mutated action: %+v", after)
	}
	if calls, _ := disp.stats(); calls != 0 {
		t.Fatal("budget denial still dispatched")
	}
}

// TestSendPreAuthorizationOwnScope pins that a send-category
// pre-authorization authorizes send through ITS OWN scope only: it
// pre-authorizes a matching send action with no human round-trip, never
// covers a write action, and a write pre-authorization never covers send.
func TestSendPreAuthorizationOwnScope(t *testing.T) {
	svc, disp, _, _, _ := newService(t, memDSN(t))
	ctx := context.Background()
	now := time.Now()
	if err := svc.store.SavePreAuthorization(ctx, appconn.PreAuthorization{
		ID: "pre-send", TenantID: 7, AllowedRisks: []string{appconn.RiskSend},
		ConnectionID: "conn-1", TargetScope: "mail.send",
		ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour), BudgetCapMicro: 5000,
	}); err != nil {
		t.Fatal(err)
	}
	// Matching send action: born authorized via its OWN scope, dispatches
	// with no human Approve call.
	sendID, err := svc.Prepare(ctx, sendAction())
	if err != nil {
		t.Fatal(err)
	}
	row, _ := svc.store.FindAction(ctx, sendID)
	if row.State != appconn.ActionAuthorized {
		t.Fatalf("send pre-auth did not authorize its own send scope: %s", row.State)
	}
	if err := svc.Execute(ctx, sendID); err != nil {
		t.Fatal(err)
	}
	if calls, _ := disp.stats(); calls != 1 {
		t.Fatalf("pre-authorized send dispatched %d times", calls)
	}
	// A write action in the same tenant is NOT covered: the send scope is
	// not a general write grant.
	write := sendAction()
	write.Target = "docs.write"
	write.Risk = appconn.RiskWrite
	writeID, err := svc.Prepare(ctx, write)
	if err != nil {
		t.Fatal(err)
	}
	wrow, _ := svc.store.FindAction(ctx, writeID)
	if wrow.State != appconn.ActionAwaitingApproval {
		t.Fatalf("send pre-auth covered a write action: %s", wrow.State)
	}
	// And the write action still executes normally after a human approval.
	if err := svc.Approve(ctx, writeID, "boss", wrow.ArgsDigest); err != nil {
		t.Fatal(err)
	}
	if err := svc.Execute(ctx, writeID); err != nil {
		t.Fatal(err)
	}
	if calls, _ := disp.stats(); calls != 2 {
		t.Fatalf("total dispatch calls=%d", calls)
	}
	// A write-category pre-authorization never covers send either.
	writePre := appconn.PreAuthorization{
		ID: "pre-write", TenantID: 7, AllowedRisks: []string{appconn.RiskWrite},
		ConnectionID: "*", TargetScope: "*",
		ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour), BudgetCapMicro: 5000,
	}
	if err := svc.store.SavePreAuthorization(ctx, writePre); err != nil {
		t.Fatal(err)
	}
	send2 := sendAction()
	send2.Target = "mail.sendBulk"
	send2ID, err := svc.Prepare(ctx, send2)
	if err != nil {
		t.Fatal(err)
	}
	srow, _ := svc.store.FindAction(ctx, send2ID)
	if srow.State != appconn.ActionAwaitingApproval {
		t.Fatalf("write pre-auth covered a send action: %s", srow.State)
	}
}

// TestUnknownOutcomeResolvedViaProviderQuery pins the crash window: an
// unknown provider outcome parks the action in unknown, it is never
// re-queued into the normal path, and only a provider query resolves it.
func TestUnknownOutcomeResolvedViaProviderQuery(t *testing.T) {
	svc, disp, res, _, _ := newService(t, memDSN(t))
	disp.outcome = DispatchOutcome{Status: appconn.ActionUnknown}
	ctx := context.Background()
	id, _ := svc.Prepare(ctx, sendAction())
	row, _ := svc.store.FindAction(ctx, id)
	if err := svc.Approve(ctx, id, "boss", row.ArgsDigest); err != nil {
		t.Fatal(err)
	}
	err := svc.Execute(ctx, id)
	if !errors.Is(err, ErrDispatchUnknown) {
		t.Fatalf("unknown outcome: %v", err)
	}
	after, _ := svc.store.FindAction(ctx, id)
	if after.State != appconn.ActionUnknown {
		t.Fatalf("crash-window state=%s", after.State)
	}
	// Re-queueing into the normal path is impossible.
	if err := svc.Execute(ctx, id); !errors.Is(err, ErrActionState) {
		t.Fatalf("unknown re-queued: %v", err)
	}
	// Provider query is the only resolution path.
	if err := svc.ResolveUnknown(ctx, id); err != nil {
		t.Fatal(err)
	}
	final, _ := svc.store.FindAction(ctx, id)
	if final.State != appconn.ActionSucceeded {
		t.Fatalf("resolved state=%s", final.State)
	}
	if atomic.LoadInt32(&res.calls) != 1 {
		t.Fatalf("provider queries=%d", res.calls)
	}
	if calls, _ := disp.stats(); calls != 1 {
		t.Fatalf("dispatch called again on unknown: %d", calls)
	}
}

// TestDispatchTimeoutRemainsUnknown pins the crash-window rule: a dispatch
// error that is NOT a proven pre-send rejection (here: a deadline exceeded
// after the request left the process) can never be recorded as failed — the
// provider outcome is unknown and only a provider query may resolve it.
func TestDispatchTimeoutRemainsUnknown(t *testing.T) {
	got := settleOutcome(DispatchOutcome{}, context.DeadlineExceeded)
	if got.Status != appconn.ActionUnknown {
		t.Fatalf("got %s", got.Status)
	}
}

// TestSettleOutcomeClassification pins the full T11 mapping: only a proven
// pre-send rejection (ErrDispatchNotStarted) settles as failed; every other
// dispatch error and every invalid provider status settles as unknown; the
// three valid provider statuses pass through unchanged.
func TestSettleOutcomeClassification(t *testing.T) {
	got := settleOutcome(DispatchOutcome{}, fmt.Errorf("binding drift: %w", ErrDispatchNotStarted))
	if got.Status != appconn.ActionFailed || got.ProviderResult != "dispatch rejected before send" {
		t.Fatalf("pre-send rejection = %+v", got)
	}
	for _, err := range []error{context.DeadlineExceeded, errors.New("transport reset"), context.Canceled} {
		got := settleOutcome(DispatchOutcome{}, err)
		if got.Status != appconn.ActionUnknown || got.ProviderResult != "provider outcome unavailable" {
			t.Fatalf("err %v mapped to %+v", err, got)
		}
	}
	for _, status := range []string{appconn.ActionSucceeded, appconn.ActionFailed, appconn.ActionUnknown} {
		got := settleOutcome(DispatchOutcome{Status: status, ProviderResult: "passthrough"}, nil)
		if got.Status != status || got.ProviderResult != "passthrough" {
			t.Fatalf("status %s mapped to %+v", status, got)
		}
	}
	got = settleOutcome(DispatchOutcome{Status: "sideways"}, nil)
	if got.Status != appconn.ActionUnknown || got.ProviderResult != "invalid provider outcome" {
		t.Fatalf("invalid provider status mapped to %+v", got)
	}
}

// seedActionRow inserts an action row directly, bypassing Prepare — used to
// materialize LEGACY rows (digest_version 1, oc_binding_json) exactly as
// migration 000122 leaves pre-existing data.
func seedActionRow(t *testing.T, db *gorm.DB, id, state, digest, ocJSON string, digestVersion int64, fence int64) {
	t.Helper()
	if err := db.Exec(`INSERT INTO app_actions
		(id, tenant_id, actor_id, connection_id, app_version, target, risk, auth_version,
		 args_snapshot, args_digest, state, provider_key, provider_result, reservation_id, fence,
		 oc_binding_json, digest_version)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, 7, "u1", "conn-1", "1.0.0", "mail.send", "send", 1,
		"{\"to\":\"a@b.c\"}", digest, state, "", "", "", fence, ocJSON, digestVersion).Error; err != nil {
		t.Fatal(err)
	}
}

// newServiceWithDB builds the stub service over a DB the test can seed
// legacy rows into directly.
func newServiceWithDB(t *testing.T) (*ActionService, *stubDispatcher, *stubResolver, *gorm.DB) {
	t.Helper()
	db := openActionDB(t, memDSN(t))
	disp := &stubDispatcher{outcome: DispatchOutcome{Status: appconn.ActionSucceeded, ProviderResult: "ok"}}
	res := &stubResolver{outcome: DispatchOutcome{Status: appconn.ActionSucceeded, ProviderResult: "provider:ok"}}
	svc := NewActionService(repoappconn.NewActionStore(db), &stubGuard{}, nil, disp, res)
	return svc, disp, res, db
}

// TestNativePrepareRejectsClientSuppliedOCBinding pins ruling 4: OC fields
// are filled ONLY server-side by PrepareOC. A caller smuggling a full
// execution binding through the native Prepare path is rejected — runtime,
// alias and external identity must never arrive from a client payload.
func TestNativePrepareRejectsClientSuppliedOCBinding(t *testing.T) {
	svc, _, _, _, _ := newService(t, memDSN(t))
	a := sendAction()
	a.OC = &appconn.OCExecutionBinding{
		RuntimeID: "rt-1", Provider: "github", ExternalID: "ext-1", Alias: "alias-1",
		ActionID: "github.search", SchemaDigest: "d1", BindingVersion: 1,
	}
	if _, err := svc.Prepare(context.Background(), a); !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("client-supplied OC binding accepted: %v", err)
	}
}

// TestPrepareWritesCurrentDigestGenerationAndCleanNativeRows pins the
// persistence contract: every NEW row carries digest_version 2 (never the
// migration legacy default 1) and native actions persist an EMPTY
// oc_binding_json; the dispatch snapshot carries the row's AuthVersion and
// digest generation, and no OC binding for native actions.
func TestPrepareWritesCurrentDigestGenerationAndCleanNativeRows(t *testing.T) {
	svc, disp, _, _, _ := newService(t, memDSN(t))
	ctx := context.Background()
	id, err := svc.Prepare(ctx, sendAction())
	if err != nil {
		t.Fatal(err)
	}
	row, err := svc.store.FindAction(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if row.DigestVersion != 2 {
		t.Fatalf("new row digest_version=%d, want 2", row.DigestVersion)
	}
	if row.OCBindingJSON != "" {
		t.Fatalf("native row persisted oc_binding_json=%q", row.OCBindingJSON)
	}
	if err := svc.Approve(ctx, id, "boss", row.ArgsDigest); err != nil {
		t.Fatal(err)
	}
	if err := svc.Execute(ctx, id); err != nil {
		t.Fatal(err)
	}
	disp.mu.Lock()
	snap := disp.lastSnap
	disp.mu.Unlock()
	if snap.AuthVersion != row.AuthVersion {
		t.Fatalf("snapshot auth version=%d, want %d", snap.AuthVersion, row.AuthVersion)
	}
	if snap.DigestVersion != 2 {
		t.Fatalf("snapshot digest version=%d, want 2", snap.DigestVersion)
	}
	if snap.OC != nil {
		t.Fatalf("native snapshot carried an OC binding: %+v", snap.OC)
	}
}

// TestLegacyV1RowsRequireRePrepare pins ruling 3: rows still on digest
// generation 1 (set by the migration on pre-existing data) can NEVER
// continue an old approval — Approve and Execute both refuse them — while
// terminal recovery semantics stay intact: a v1 row parked in unknown still
// resolves through the provider query with its OLD digest untouched.
func TestLegacyV1RowsRequireRePrepare(t *testing.T) {
	svc, _, res, db := newServiceWithDB(t)
	ctx := context.Background()
	seedActionRow(t, db, "act-v1-pending", appconn.ActionAwaitingApproval, "legacydigest1", "", 1, 0)
	if err := svc.Approve(ctx, "act-v1-pending", "boss", "legacydigest1"); !errors.Is(err, ErrActionRePrepareRequired) {
		t.Fatalf("v1 pending approve: %v", err)
	}
	seedActionRow(t, db, "act-v1-authorized", appconn.ActionAuthorized, "legacydigest2", "", 1, 0)
	if err := svc.Execute(ctx, "act-v1-authorized"); !errors.Is(err, ErrActionRePrepareRequired) {
		t.Fatalf("v1 authorized execute: %v", err)
	}
	// Recovery semantics: a v1 row in unknown still resolves via the
	// provider query (old digest preserved — no rewrite, no re-queue).
	seedActionRow(t, db, "act-v1-unknown", appconn.ActionUnknown, "legacydigest3", "", 1, 4)
	if err := svc.ResolveUnknown(ctx, "act-v1-unknown"); err != nil {
		t.Fatalf("v1 unknown resolution broken: %v", err)
	}
	after, _ := svc.store.FindAction(ctx, "act-v1-unknown")
	if after.State != appconn.ActionSucceeded || after.ArgsDigest != "legacydigest3" || after.DigestVersion != 1 {
		t.Fatalf("v1 recovery mutated the legacy row: %+v", after)
	}
	if res.calls != 1 {
		t.Fatalf("provider queries=%d", res.calls)
	}
}

// TestSnapshotCarriesPersistedOCBinding pins the full-snapshot contract for
// OC actions: the dispatcher receives the parsed immutable execution
// binding persisted at Prepare time, together with the row's auth version
// and digest generation.
func TestSnapshotCarriesPersistedOCBinding(t *testing.T) {
	svc, disp, _, db := newServiceWithDB(t)
	ctx := context.Background()
	ocJSON := `{"RuntimeID":"rt-9","Provider":"github","ExternalID":"ext-9","Alias":"alias-9","ActionID":"github.search","SchemaDigest":"dd-9","BindingVersion":7}`
	seedActionRow(t, db, "act-oc-1", appconn.ActionAuthorized, "ocdigest1", ocJSON, 2, 0)
	if err := svc.store.SaveApproval(ctx, "act-oc-1", "ocdigest1", "boss", time.Now().Add(time.Hour), 1); err != nil {
		t.Fatal(err)
	}
	if err := svc.Execute(ctx, "act-oc-1"); err != nil {
		t.Fatal(err)
	}
	disp.mu.Lock()
	snap := disp.lastSnap
	disp.mu.Unlock()
	if snap.OC == nil {
		t.Fatal("OC action snapshot lost its execution binding")
	}
	want := appconn.OCExecutionBinding{
		RuntimeID: "rt-9", Provider: "github", ExternalID: "ext-9", Alias: "alias-9",
		ActionID: "github.search", SchemaDigest: "dd-9", BindingVersion: 7,
	}
	if *snap.OC != want {
		t.Fatalf("snapshot binding=%+v, want %+v", *snap.OC, want)
	}
	if snap.AuthVersion != 1 || snap.DigestVersion != 2 {
		t.Fatalf("snapshot auth/digest version=%d/%d", snap.AuthVersion, snap.DigestVersion)
	}
}

// stubClaimSource stands in for the durable OC claim with a configurable
// outcome (and an optional side effect run before the error is returned, to
// materialize states like "another request's claim already went through").
type stubClaimSource struct {
	err    error
	before func(context.Context)
	calls  int32
}

func (s *stubClaimSource) ClaimOCDispatch(ctx context.Context, subject appconn.OCSubject, actionID, reservationID string) (appconn.OCDispatchRecord, error) {
	atomic.AddInt32(&s.calls, 1)
	if s.before != nil {
		s.before(ctx)
	}
	return appconn.OCDispatchRecord{}, s.err
}

func (s *stubClaimSource) GetOCDispatch(ctx context.Context, tenant uint64, actionID string) (appconn.OCDispatchRecord, error) {
	return appconn.OCDispatchRecord{}, nil
}

// newClaimReleaseEnv seeds one AUTHORIZED open-connector action over a stub
// claim source whose rejection is configured per test, with a recording
// budget gate wired.
func newClaimReleaseEnv(t *testing.T, claimErr error, before func(context.Context)) (*ActionService, *stubGate, *stubClaimSource, *gorm.DB) {
	t.Helper()
	db := openActionDB(t, memDSN(t))
	gate := &stubGate{}
	disp := &stubDispatcher{outcome: DispatchOutcome{Status: appconn.ActionSucceeded, ProviderResult: "ok"}}
	svc := NewActionService(repoappconn.NewActionStore(db), &stubGuard{}, gate, disp, nil)
	claims := &stubClaimSource{err: claimErr, before: before}
	svc.UseOCDispatchClaims(claims)
	seedActionRow(t, db, "act-orphan", appconn.ActionAuthorized, "ocdigest-orphan", ocBindingJSONForExecute(), 2, 0)
	if err := svc.store.SaveApproval(context.Background(), "act-orphan", "ocdigest-orphan", "boss", time.Now().Add(time.Hour), 5); err != nil {
		t.Fatal(err)
	}
	return svc, gate, claims, db
}

// TestExecuteClaimConflictReleasesOrphanReservation is the T10-Q-1 carry: a
// claim rejected while the row is still authorized proves the outbound call
// never happened, so the budget pre-allocation is settled with ZERO usage
// instead of lingering as an orphan hold the recovery would retain forever.
func TestExecuteClaimConflictReleasesOrphanReservation(t *testing.T) {
	svc, gate, claims, db := newClaimReleaseEnv(t, repoappconn.ErrOCDispatchConflict, nil)
	ctx := context.Background()

	err := svc.Execute(ctx, "act-orphan")
	if !errors.Is(err, repoappconn.ErrOCDispatchConflict) {
		t.Fatalf("err = %v, want ErrOCDispatchConflict", err)
	}
	if atomic.LoadInt32(&claims.calls) != 1 {
		t.Fatalf("claim calls = %d", claims.calls)
	}
	if atomic.LoadInt32(&gate.finishes) != 1 {
		t.Fatalf("orphan release finishes = %d, want 1", gate.finishes)
	}
	resID, fact := gate.lastFinish()
	if resID != "res-1" {
		t.Fatalf("released reservation = %q, want the Begin reservation", resID)
	}
	if fact.Dimensions[commercial.DimensionConnector] != 0 || fact.Status != commercial.UsageStatusFinal ||
		fact.RunID != "appaction:act-orphan" || fact.CallID != "act-orphan" {
		t.Fatalf("zero-usage fact = %+v", fact)
	}
	var row repoappconn.ActionRow
	if err := db.Where("id = ?", "act-orphan").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != appconn.ActionAuthorized || row.Fence != 0 {
		t.Fatalf("rejected claim mutated the action: %+v", row)
	}
	var ap repoappconn.ApprovalRow
	if err := db.Where("action_id = ?", "act-orphan").First(&ap).Error; err != nil {
		t.Fatal(err)
	}
	if ap.Remaining != 5 {
		t.Fatalf("approval consumed on rejected claim: %d", ap.Remaining)
	}
}

// TestExecuteClaimExhaustionReleasesOrphanReservation: an exhausted approval
// is equally winnerless — the claim transaction rejected before any state
// move, so the pre-allocation settles at zero usage.
func TestExecuteClaimExhaustionReleasesOrphanReservation(t *testing.T) {
	svc, gate, _, _ := newClaimReleaseEnv(t, repoappconn.ErrApprovalExhausted, nil)
	if err := svc.Execute(context.Background(), "act-orphan"); !errors.Is(err, repoappconn.ErrApprovalExhausted) {
		t.Fatalf("err = %v, want ErrApprovalExhausted", err)
	}
	if atomic.LoadInt32(&gate.finishes) != 1 {
		t.Fatalf("finishes = %d, want 1 (zero-usage release)", gate.finishes)
	}
}

// TestExecuteClaimLostRaceNeverSettlesWinnersReservation: a LOST claim race
// (ErrOCDispatchClaimed) means the winner still owns the shared reservation —
// the loser must never cancel or settle it.
func TestExecuteClaimLostRaceNeverSettlesWinnersReservation(t *testing.T) {
	svc, gate, _, _ := newClaimReleaseEnv(t, repoappconn.ErrOCDispatchClaimed, nil)
	if err := svc.Execute(context.Background(), "act-orphan"); !errors.Is(err, repoappconn.ErrOCDispatchClaimed) {
		t.Fatalf("err = %v, want ErrOCDispatchClaimed", err)
	}
	if atomic.LoadInt32(&gate.finishes) != 0 {
		t.Fatalf("finishes = %d, want 0 (winner owns the reservation)", gate.finishes)
	}
}

// TestExecuteClaimConflictWithLiveWinnerHoldsReservation: ErrOCDispatchConflict
// where the row has ALREADY moved to dispatched means another request's claim
// is live (mixed wiring: the frozen native path claimed first) — the shared
// reservation belongs to that winner and must not be zero-settled here.
func TestExecuteClaimConflictWithLiveWinnerHoldsReservation(t *testing.T) {
	db := openActionDB(t, memDSN(t))
	gate := &stubGate{}
	disp := &stubDispatcher{outcome: DispatchOutcome{Status: appconn.ActionSucceeded, ProviderResult: "ok"}}
	svc := NewActionService(repoappconn.NewActionStore(db), &stubGuard{}, gate, disp, nil)
	claims := &stubClaimSource{
		err: repoappconn.ErrOCDispatchConflict,
		before: func(ctx context.Context) {
			// Simulate the winner's claim committing first.
			if err := db.Exec("UPDATE app_actions SET state = ?, fence = fence + 1 WHERE id = ?", appconn.ActionDispatched, "act-orphan").Error; err != nil {
				t.Error(err)
			}
		},
	}
	svc.UseOCDispatchClaims(claims)
	seedActionRow(t, db, "act-orphan", appconn.ActionAuthorized, "ocdigest-orphan", ocBindingJSONForExecute(), 2, 0)
	if err := svc.store.SaveApproval(context.Background(), "act-orphan", "ocdigest-orphan", "boss", time.Now().Add(time.Hour), 5); err != nil {
		t.Fatal(err)
	}

	if err := svc.Execute(context.Background(), "act-orphan"); !errors.Is(err, repoappconn.ErrOCDispatchConflict) {
		t.Fatalf("err = %v, want ErrOCDispatchConflict", err)
	}
	if atomic.LoadInt32(&gate.finishes) != 0 {
		t.Fatalf("finishes = %d, want 0 (live winner owns the reservation)", gate.finishes)
	}
	var row repoappconn.ActionRow
	if err := db.Where("id = ?", "act-orphan").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != appconn.ActionDispatched {
		t.Fatalf("winner's dispatched state overwritten: %s", row.State)
	}
}
