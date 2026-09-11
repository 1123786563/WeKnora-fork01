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
	"github.com/Tencent/WeKnora/internal/commercial"

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

type stubGuard struct{ err error }

func (g *stubGuard) Resolve(ctx context.Context, connectionID string, expectedVersion int64) ([]byte, error) {
	if g.err != nil {
		return nil, g.err
	}
	return []byte("cred"), nil
}

type stubDispatcher struct {
	mu        sync.Mutex
	outcome   DispatchOutcome
	err       error
	calls     int
	lastArgs  []byte
	lastKey   string
	lastFence int64
}

func (d *stubDispatcher) Dispatch(ctx context.Context, snap ActionSnapshot, providerKey string) (DispatchOutcome, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls++
	d.lastArgs = append([]byte(nil), snap.Args...)
	d.lastKey = providerKey
	d.lastFence = snap.Fence
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
	atomic.AddInt32(&g.finishes, 1)
	return nil
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
