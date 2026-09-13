package connectorcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	repoapp "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/application/service/appconnector"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// T07 fixtures: attempts, correlators, sealed handoffs
// ---------------------------------------------------------------------------

var t07Now = time.Unix(1700000000, 0).UTC()

// t07Attempt seeds one attempt for tenant 7 against conn1 at version 1.
func t07Attempt(t *testing.T, db *gorm.DB, state string) appconn.OCAuthorizationAttempt {
	t.Helper()
	a, err := appconnectorsvc.NewOCAttempt(
		appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "conn1", "rt-1", "app-oc", 1, t07Now)
	if err != nil {
		t.Fatal(err)
	}
	a.State = state
	if err := repoapp.NewOCStore(db).SaveOCAttempt(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	return a
}

func t07LoadAttempt(t *testing.T, db *gorm.DB, id string) appconn.OCAuthorizationAttempt {
	t.Helper()
	a, err := repoapp.NewOCStore(db).GetOCAttempt(context.Background(), 7, id)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// fakeCorrelator stands in for the runtime correlation surface.
type fakeCorrelator struct {
	mu         sync.Mutex
	resolves   int
	submits    int
	gotAlias   string
	gotAPIKey  string
	resolve    RuntimeConnection
	resolveErr error
	submit     RuntimeConnection
	submitErr  error
}

func (f *fakeCorrelator) ResolveExternalConnection(ctx context.Context, service, alias string) (RuntimeConnection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resolves++
	f.gotAlias = alias
	if f.resolveErr != nil {
		return RuntimeConnection{}, f.resolveErr
	}
	return f.resolve, nil
}

func (f *fakeCorrelator) SubmitExternalCredential(ctx context.Context, service, alias, apiKey string) (RuntimeConnection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.submits++
	f.gotAlias = alias
	f.gotAPIKey = apiKey
	if f.submitErr != nil {
		return RuntimeConnection{}, f.submitErr
	}
	return f.submit, nil
}

func (f *fakeCorrelator) counts() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.resolves, f.submits
}

func t07ConfirmedConnection(alias string) RuntimeConnection {
	return RuntimeConnection{
		ID: "ext-42", Alias: alias, ProviderAccountID: "acct-1",
		Provider: "app-oc", Status: "active",
	}
}

// ---------------------------------------------------------------------------
// kind parity: the service-side enqueue constants must spell exactly the
// frozen control kinds (the service cannot import this package).
// ---------------------------------------------------------------------------

func TestControlKindParityWithServiceConstants(t *testing.T) {
	if appconnectorsvc.KindOCAuthorize != KindAuthorize ||
		appconnectorsvc.KindOCConfirm != KindConfirm ||
		appconnectorsvc.KindOCDeleteConnection != KindDeleteConnection {
		t.Fatalf("service kind constants drifted: %q/%q/%q",
			appconnectorsvc.KindOCAuthorize, appconnectorsvc.KindOCConfirm, appconnectorsvc.KindOCDeleteConnection)
	}
}

// ---------------------------------------------------------------------------
// authorize (OAuth start under the attempt alias)
// ---------------------------------------------------------------------------

func TestWorkerAuthorizeStartsUnderAttemptAlias(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	seedConnectionRow(t, db, 7, "conn1", 1)
	a := t07Attempt(t, db, appconnectorsvc.OCAttemptPending)
	seedOp(t, db, "op1", KindAuthorize, 7, a.ID, 1, 0, time.Unix(1699990000, 0).UTC())

	admin := newFakeAdmin()
	w := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("admin-secret-1"))
	if err := w.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if calls, kinds := admin.snapshot(); calls != 1 || kinds[KindAuthorize] != 1 {
		t.Fatalf("expected exactly one start call, got %v", kinds)
	}
	if got := t07LoadAttempt(t, db, a.ID); got.State != appconnectorsvc.OCAttemptAuthorizing {
		t.Fatalf("attempt state = %q, want authorizing", got.State)
	}
	if row := outboxRow(t, db, "op1"); row != nil {
		t.Fatal("authorize operation must complete")
	}

	// Idempotent re-run (retry after a partially failed start): the alias is
	// the same, so re-starting is safe and the row completes again.
	seedOp(t, db, "op2", KindAuthorize, 7, a.ID, 1, 0, time.Unix(1699990000, 0).UTC())
	admin2 := newFakeAdmin()
	w2 := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), admin2, &fakeSink{}, staticSecret("admin-secret-1"))
	if err := w2.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if calls, _ := admin2.snapshot(); calls != 1 {
		t.Fatal("re-run must start the authorization again (same alias)")
	}
	if got := t07LoadAttempt(t, db, a.ID); got.State != appconnectorsvc.OCAttemptAuthorizing {
		t.Fatalf("attempt state after re-run = %q", got.State)
	}
}

func TestWorkerAuthorizeTerminalAttemptNeverRevives(t *testing.T) {
	for _, state := range []string{appconnectorsvc.OCAttemptFailed, appconnectorsvc.OCAttemptExpired, appconnectorsvc.OCAttemptRevoked, appconnectorsvc.OCAttemptActive, appconnectorsvc.OCAttemptVerifying} {
		db := newWorkerTestDB(t)
		ctx := context.Background()
		seedConnectionRow(t, db, 7, "conn1", 1)
		a := t07Attempt(t, db, state)
		seedOp(t, db, "op1", KindAuthorize, 7, a.ID, 1, 0, time.Unix(1699990000, 0).UTC())
		admin := newFakeAdmin()
		w := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("admin-secret-1"))
		if err := w.RunOnce(ctx); err != nil {
			t.Fatalf("state %s: permanent drop expected, got %v", state, err)
		}
		if calls, _ := admin.snapshot(); calls != 0 {
			t.Fatalf("state %s: terminal attempt must never reach the admin client", state)
		}
		if row := outboxRow(t, db, "op1"); row != nil {
			t.Fatalf("state %s: row must be dropped", state)
		}
		if got := t07LoadAttempt(t, db, a.ID); got.State != state {
			t.Fatalf("state %s: attempt must stay untouched, got %q", state, got.State)
		}
	}
}

// ---------------------------------------------------------------------------
// confirm: correlation gate, one activation, forgery, reconciliation
// ---------------------------------------------------------------------------

// Without correlation endpoints on the wired admin client and no explicit
// correlator, confirm stays queued (fail-closed, ErrUnpinnedEndpoint
// convention) and never touches the admin surface.
func TestWorkerConfirmHeldWithoutCorrelationEndpoints(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	seedConnectionRow(t, db, 7, "conn1", 1)
	a := t07Attempt(t, db, appconnectorsvc.OCAttemptVerifying)
	seedOp(t, db, "op1", KindConfirm, 7, a.ID, 1, 0, time.Unix(1699990000, 0).UTC())

	admin := newFakeAdmin()
	w := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("admin-secret-1"))
	err := w.RunOnce(ctx)
	if !errors.Is(err, ErrCorrelationUnavailable) {
		t.Fatalf("expected ErrCorrelationUnavailable, got %v", err)
	}
	if calls, _ := admin.snapshot(); calls != 0 {
		t.Fatal("held confirm must not touch the admin client")
	}
	if row := outboxRow(t, db, "op1"); row == nil {
		t.Fatal("held confirm must stay queued")
	}
}

func TestWorkerConfirmDuplicateCallbackActivatesOnce(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	seedConnectionRow(t, db, 7, "conn1", 1)
	a := t07Attempt(t, db, appconnectorsvc.OCAttemptVerifying)
	seedOp(t, db, "op1", KindConfirm, 7, a.ID, 1, 0, time.Unix(1699990000, 0).UTC())

	fc := &fakeCorrelator{resolve: t07ConfirmedConnection(a.Alias)}
	admin := newFakeAdmin()
	w := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("admin-secret-1"))
	w.SetRuntimeCorrelator(fc)
	if err := w.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if got := t07LoadAttempt(t, db, a.ID); got.State != appconnectorsvc.OCAttemptActive {
		t.Fatalf("attempt state = %q, want active", got.State)
	}
	b, err := repoapp.NewOCStore(db).GetBinding(ctx, 7, "conn1")
	if err != nil || b.State != appconn.OCBindingActive || b.ExternalID != "ext-42" || b.Alias != a.Alias {
		t.Fatalf("activation binding wrong: %+v %v", b, err)
	}
	if row := outboxRow(t, db, "op1"); row != nil {
		t.Fatal("confirm operation must complete")
	}

	// Duplicate callback racing the same attempt: a second confirm op finds
	// an active attempt and is a PERMANENT drop - exactly one activation.
	seedOp(t, db, "op2", KindConfirm, 7, a.ID, 1, 0, time.Unix(1699990000, 0).UTC())
	if err := w.RunOnce(ctx); err != nil {
		t.Fatalf("duplicate confirm must drop permanently, got %v", err)
	}
	if resolves, _ := fc.counts(); resolves != 1 {
		t.Fatalf("duplicate confirm must not resolve again, got %d resolves", resolves)
	}
	var bindings int64
	if err := db.Table("connector_connection_bindings").Where("tenant_id = ?", 7).Count(&bindings).Error; err != nil {
		t.Fatal(err)
	}
	if bindings != 1 {
		t.Fatalf("duplicate callback activated %d times", bindings)
	}
}

func TestWorkerConfirmTenantForgeryDropped(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	seedConnectionRow(t, db, 7, "conn1", 1)
	a := t07Attempt(t, db, appconnectorsvc.OCAttemptVerifying)
	// The op claims tenant 8; the attempt belongs to tenant 7. The
	// tenant-scoped lookup reads not-found and the row is dropped.
	seedOp(t, db, "op1", KindConfirm, 8, a.ID, 1, 0, time.Unix(1699990000, 0).UTC())

	fc := &fakeCorrelator{resolve: t07ConfirmedConnection(a.Alias)}
	admin := newFakeAdmin()
	w := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("admin-secret-1"))
	w.SetRuntimeCorrelator(fc)
	if err := w.RunOnce(ctx); err != nil {
		t.Fatalf("forged op must drop, got %v", err)
	}
	if resolves, _ := fc.counts(); resolves != 0 {
		t.Fatal("forged op must never reach the correlator")
	}
	if calls, _ := admin.snapshot(); calls != 0 {
		t.Fatal("forged op must never reach the admin client")
	}
	if got := t07LoadAttempt(t, db, a.ID); got.State != appconnectorsvc.OCAttemptVerifying {
		t.Fatalf("forged op must leave the attempt untouched, got %q", got.State)
	}
}

func TestWorkerConfirmAliasSubstitutionRejected(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	seedConnectionRow(t, db, 7, "conn1", 1)
	a := t07Attempt(t, db, appconnectorsvc.OCAttemptVerifying)
	seedOp(t, db, "op1", KindConfirm, 7, a.ID, 1, 0, time.Unix(1699990000, 0).UTC())

	// The resolver answers with SOMEONE ELSE's connection: the frozen
	// CanCompleteOCAttempt gate rejects, the attempt fails, and NO remote
	// delete is enqueued (the foreign connection is not provably ours).
	fc := &fakeCorrelator{resolve: t07ConfirmedConnection("someone-else")}
	admin := newFakeAdmin()
	w := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("admin-secret-1"))
	w.SetRuntimeCorrelator(fc)
	if err := w.RunOnce(ctx); err != nil {
		t.Fatalf("alias substitution must be a permanent drop, got %v", err)
	}
	if got := t07LoadAttempt(t, db, a.ID); got.State != appconnectorsvc.OCAttemptFailed {
		t.Fatalf("substituted attempt must fail, got %q", got.State)
	}
	var deletes int64
	if err := db.Table("connector_operations_outbox").Where("kind = ?", KindDeleteConnection).Count(&deletes).Error; err != nil {
		t.Fatal(err)
	}
	if deletes != 0 {
		t.Fatalf("alias substitution must never enqueue a remote delete, got %d", deletes)
	}
}

func TestWorkerConfirmTokenFailureNeverActivates(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	seedConnectionRow(t, db, 7, "conn1", 1)
	a := t07Attempt(t, db, appconnectorsvc.OCAttemptVerifying)
	seedOp(t, db, "op1", KindConfirm, 7, a.ID, 1, 0, time.Unix(1699990000, 0).UTC())

	fc := &fakeCorrelator{resolve: t07ConfirmedConnection(a.Alias)}
	admin := newFakeAdmin()
	admin.err = errors.New("connectorcontrol: admin api rejected: status 503 code unavailable")
	w := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("admin-secret-1"))
	w.SetRuntimeCorrelator(fc)

	// Direct-drive with an explicit action grant: mint failure must leave
	// the attempt unactivated and the operation queued for retry.
	row, op := claimForRun(t, db, "wA")
	op.Payload = json.RawMessage(`{"attemptID":"` + a.ID + `","actions":["app-oc.fetch"]}`)
	if err := w.runClaimed(ctx, row, op); err == nil {
		t.Fatal("token failure must surface")
	}
	if got := t07LoadAttempt(t, db, a.ID); got.State != appconnectorsvc.OCAttemptVerifying {
		t.Fatalf("token failure must not activate, got %q", got.State)
	}
	var bindings int64
	if err := db.Table("connector_connection_bindings").Where("tenant_id = ?", 7).Count(&bindings).Error; err != nil {
		t.Fatal(err)
	}
	if bindings != 0 {
		t.Fatal("token failure must not write a binding")
	}
	if kept := outboxRow(t, db, "op1"); kept == nil {
		t.Fatal("failed confirm must stay queued for retry")
	}
}

func TestWorkerConfirmTrimsWhitespaceActionsBeforeMint(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	seedConnectionRow(t, db, 7, "conn1", 1)
	a := t07Attempt(t, db, appconnectorsvc.OCAttemptVerifying)

	fc := &fakeCorrelator{resolve: t07ConfirmedConnection(a.Alias)}
	admin := newFakeAdmin()
	admin.createResult = RuntimeTokenCreated{TokenRecordID: "rtt_1", Token: "oct_SECRET"}
	sink := &fakeSink{}
	w := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), admin, sink, staticSecret("admin-secret-1"))
	w.SetRuntimeCorrelator(fc)

	// Whitespace-only entries collapse away (T06-Q-F5 note); the minted
	// grant carries exactly the real action. Actions are an explicit-payload
	// field (the frozen row has no payload column), so the op is claimed for
	// real and driven with the payload attached.
	seedOp(t, db, "opw", KindConfirm, 7, a.ID, 1, 0, time.Unix(1699990000, 0).UTC())
	row, op := claimForRun(t, db, "wA")
	op.Payload = json.RawMessage(`{"actions":["   ","\t","app-oc.fetch"]}`)
	if err := w.runClaimed(ctx, row, op); err != nil {
		t.Fatal(err)
	}
	admin.mu.Lock()
	defer admin.mu.Unlock()
	if len(admin.createReqs) != 1 || len(admin.createReqs[0].Actions) != 1 || admin.createReqs[0].Actions[0] != "app-oc.fetch" {
		t.Fatalf("minted grant wrong: %+v", admin.createReqs)
	}
	if got := t07LoadAttempt(t, db, a.ID); got.State != appconnectorsvc.OCAttemptActive {
		t.Fatalf("attempt state = %q, want active", got.State)
	}
}

// Remote success / local failure: the activation races a drifted connection
// version, loses, and reconciles by THIS attempt's alias - a remote delete
// matching (attempt, alias, external id) is enqueued; once a NEWER activation
// owns the external connection, cleanup never deletes it again.
func TestWorkerConfirmReconcilesByAliasNeverNewest(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	seedConnectionRow(t, db, 7, "conn1", 1)
	a := t07Attempt(t, db, appconnectorsvc.OCAttemptVerifying)
	seedOp(t, db, "op1", KindConfirm, 7, a.ID, 1, 0, time.Unix(1699990000, 0).UTC())

	fc := &fakeCorrelator{resolve: t07ConfirmedConnection(a.Alias)}
	admin := newFakeAdmin()
	w := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("admin-secret-1"))
	w.SetRuntimeCorrelator(fc)

	// Local drift: bump the connection version so the activation tx refuses.
	if err := db.Exec("UPDATE connections SET auth_version = 2 WHERE tenant_id = 7 AND id = 'conn1'").Error; err != nil {
		t.Fatal(err)
	}
	if err := w.RunOnce(ctx); err != nil {
		t.Fatalf("local-failure confirm must be a permanent drop, got %v", err)
	}
	if got := t07LoadAttempt(t, db, a.ID); got.State != appconnectorsvc.OCAttemptFailed {
		t.Fatalf("reconciled attempt must fail, got %q", got.State)
	}
	var row repoapp.OCOperationsOutboxRow
	if err := db.Where("kind = ?", KindDeleteConnection).First(&row).Error; err != nil {
		t.Fatalf("reconciliation must enqueue the remote delete: %v", err)
	}
	if want := a.ID + "|ext-42"; row.ResourceID != want {
		t.Fatalf("cleanup row = %q, want %q", row.ResourceID, want)
	}

	// Never-newest: a newer attempt activates the same external id; a later
	// local failure for an older attempt must NOT enqueue another delete.
	db.Exec("DELETE FROM connector_operations_outbox")
	db.Exec("UPDATE connections SET auth_version = 1 WHERE tenant_id = 7 AND id = 'conn1'")
	newer, err := appconnectorsvc.NewOCAttempt(appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "conn1", "rt-1", "app-oc", 1, t07Now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repoapp.NewOCStore(db).SaveOCAttempt(ctx, newer); err != nil {
		t.Fatal(err)
	}
	if ok, _ := repoapp.NewOCStore(db).AdvanceOCAttempt(ctx, 7, newer.ID, appconnectorsvc.OCAttemptPending, appconnectorsvc.OCAttemptAuthorizing, t07Now); !ok {
		t.Fatal("seed authorizing failed")
	}
	if ok, _ := repoapp.NewOCStore(db).AdvanceOCAttempt(ctx, 7, newer.ID, appconnectorsvc.OCAttemptAuthorizing, appconnectorsvc.OCAttemptVerifying, t07Now); !ok {
		t.Fatal("seed verifying failed")
	}
	if err := repoapp.NewOCStore(db).ActivateOCAttempt(ctx, 7, newer.ID, "ext-42", t07Now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	loser, err := appconnectorsvc.NewOCAttempt(appconn.OCSubject{TenantID: 7, ActorID: "alice"}, "conn1", "rt-1", "app-oc", 1, t07Now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repoapp.NewOCStore(db).SaveOCAttempt(ctx, loser); err != nil {
		t.Fatal(err)
	}
	if err := repoapp.NewOCStore(db).CleanupFailedOCAttempt(ctx, 7, loser.ID, loser.Alias, "ext-42", t07Now); err != nil {
		t.Fatal(err)
	}
	var deletes int64
	db.Table("connector_operations_outbox").Where("kind = ?", KindDeleteConnection).Count(&deletes)
	if deletes != 0 {
		t.Fatalf("cleanup must never delete the newest connection, got %d ops", deletes)
	}
}

// ---------------------------------------------------------------------------
// API-key handoff (dedicated decrypt-and-submit path)
// ---------------------------------------------------------------------------

func TestWorkerAPIKeyHandoffDecryptsSubmitsAndActivates(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	seedConnectionRow(t, db, 7, "conn1", 1)
	a := t07Attempt(t, db, appconnectorsvc.OCAttemptPending)

	cipher, err := appconnectorsvc.NewTransientCipherFromKey("t07-test-key")
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := cipher.Seal("ghp_plain_secret", a.ID)
	if err != nil {
		t.Fatal(err)
	}
	seedOp(t, db, "op1", KindAuthorize, 7, a.ID+"|"+sealed, 1, 0, time.Unix(1699990000, 0).UTC())

	// The runtime mints its own alias for the created connection; the worker
	// adopts it before verification (R14 iv).
	fc := &fakeCorrelator{submit: t07ConfirmedConnection("runtime-minted-alias")}
	admin := newFakeAdmin()
	w := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("admin-secret-1"))
	w.SetRuntimeCorrelator(fc)
	w.SetTransientCipher(cipher)

	if err := w.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if _, submits := fc.counts(); submits != 1 {
		t.Fatalf("expected exactly one submit, got %d", submits)
	}
	fc.mu.Lock()
	if fc.gotAPIKey != "ghp_plain_secret" {
		t.Fatalf("submitted key = %q", fc.gotAPIKey)
	}
	fc.mu.Unlock()
	got := t07LoadAttempt(t, db, a.ID)
	if got.State != appconnectorsvc.OCAttemptActive {
		t.Fatalf("attempt state = %q, want active", got.State)
	}
	if got.Alias != "runtime-minted-alias" {
		t.Fatalf("adopted alias = %q, want runtime-minted-alias", got.Alias)
	}
	b, err := repoapp.NewOCStore(db).GetBinding(ctx, 7, "conn1")
	if err != nil || b.Alias != "runtime-minted-alias" || b.ExternalID != "ext-42" {
		t.Fatalf("activation binding wrong: %+v %v", b, err)
	}
	// Success deletes the row - and with it the transient ciphertext.
	if row := outboxRow(t, db, "op1"); row != nil {
		t.Fatal("api-key handoff must complete (ciphertext deleted)")
	}
}

func TestWorkerAPIKeyHandoffFailureRetainsCiphertextUntilExpiry(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	seedConnectionRow(t, db, 7, "conn1", 1)
	a := t07Attempt(t, db, appconnectorsvc.OCAttemptPending)

	cipher, err := appconnectorsvc.NewTransientCipherFromKey("t07-test-key")
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := cipher.Seal("ghp_plain_secret", a.ID)
	if err != nil {
		t.Fatal(err)
	}
	seedOp(t, db, "op1", KindAuthorize, 7, a.ID+"|"+sealed, 1, 0, time.Unix(1699990000, 0).UTC())

	fc := &fakeCorrelator{submitErr: errors.New("connectorcontrol: admin transport: connection refused")}
	admin := newFakeAdmin()
	w := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("admin-secret-1"))
	w.SetRuntimeCorrelator(fc)
	w.SetTransientCipher(cipher)

	if err := w.RunOnce(ctx); err == nil {
		t.Fatal("submit failure must surface")
	}
	kept := outboxRow(t, db, "op1")
	if kept == nil {
		t.Fatal("failed handoff must stay queued (ciphertext retained)")
	}
	if got := t07LoadAttempt(t, db, a.ID); got.State != appconnectorsvc.OCAttemptPending {
		t.Fatalf("attempt state after failed submit = %q, want pending (handoff never started)", got.State)
	}

	// Once the 15-minute window closes, the retry becomes a permanent drop:
	// the row - and the ciphertext with it - is deleted (ruling 7).
	w.now = func() time.Time { return t07Now.Add(16 * time.Minute) }
	if err := w.RunOnce(ctx); err != nil {
		t.Fatalf("expired handoff must drop permanently, got %v", err)
	}
	if row := outboxRow(t, db, "op1"); row != nil {
		t.Fatal("expired handoff must delete the ciphertext row")
	}
	if got := t07LoadAttempt(t, db, a.ID); got.State != appconnectorsvc.OCAttemptExpired {
		t.Fatalf("expired handoff attempt = %q", got.State)
	}
}

// The REAL admin client carries the correlation endpoints, so the default
// correlator derives from it without any extra wiring (R14). A worker built
// over that client resolves a non-nil default correlator.
func TestDefaultCorrelatorDerivesFromRealAdminClient(t *testing.T) {
	var _ correlationAdmin = (*RuntimeAdminClient)(nil)
	db := newWorkerTestDB(t)
	seedConnectionRow(t, db, 7, "conn1", 1)
	srv, _ := newAdminTestServer(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		io.WriteString(w, `{"success":true,"message":"OK","data":[],"meta":{}}`)
	})
	client := mustClient(t, testClientConfig(srv.URL, staticSecret(testAdminSecret)))
	w := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), client, &fakeSink{}, staticSecret("admin-secret-1"))
	correlator, err := w.runtimeCorrelator()
	if err != nil || correlator == nil {
		t.Fatalf("real admin client must yield the default correlator: %v", err)
	}
	// The derived default resolves through the pinned list endpoint; an
	// exact-alias miss on an empty runtime is a not-found AdminError.
	if _, err := correlator.ResolveExternalConnection(context.Background(), "app-oc", "alias-x"); err == nil {
		t.Fatal("empty runtime must not resolve an alias")
	}
}

// ---------------------------------------------------------------------------
// admin client wire: the R13.5/R14 correlation endpoints against the exact
// /v1 envelope (contract §3.3 + fixtures/oauth_correlation.json shapes)
// ---------------------------------------------------------------------------

func TestAdminClientCorrelationWire(t *testing.T) {
	var apiPath string
	var apiBody []byte
	srv, _ := newAdminTestServer(t, func(w http.ResponseWriter, r *http.Request, raw []byte) {
		switch {
		case r.URL.Path == "/v1/connections":
			io.WriteString(w, `{"success":true,"message":"OK","data":[{"id":"ext-a","service":"github","status":"active","alias":"alias-a","providerAccountId":"acct-a"},{"id":"ext-b","service":"github","status":"disconnected","alias":"alias-b","providerAccountId":"acct-b"}],"meta":{}}`)
		case r.URL.Path == "/v1/connections/by-id/ext-a":
			io.WriteString(w, `{"success":true,"message":"OK","data":{"id":"ext-a","service":"github","status":"active","alias":"alias-a","providerAccountId":"acct-a"},"meta":{}}`)
		case r.URL.Path == "/v1/connection-requests/cr-1":
			io.WriteString(w, `{"success":true,"message":"OK","data":{"connectionRequestId":"cr-1","service":"github","status":"connected","appId":"ext-a","errorCode":null,"errorMessage":null,"expiresAt":"2026-09-12T14:30:38.921Z","createdAt":1789222838921,"updatedAt":1789222838998},"meta":{}}`)
		case r.URL.Path == "/v1/connections/github/connect/api-key":
			apiPath = r.URL.Path
			apiBody = raw
			io.WriteString(w, `{"success":true,"message":"OK","data":{"id":"ext-k","service":"github","status":"active","alias":"alias-k","providerAccountId":"acct-k"},"meta":{}}`)
		default:
			http.NotFound(w, r)
		}
	})
	c := mustClient(t, testClientConfig(srv.URL, staticSecret(testAdminSecret)))
	ctx := context.Background()

	conns, err := c.ListRuntimeConnections(ctx)
	if err != nil || len(conns) != 2 {
		t.Fatalf("list = %+v, %v", conns, err)
	}
	if conns[0].Provider != "github" || conns[0].Status != "active" || conns[0].Alias != "alias-a" {
		t.Fatalf("wire mapping wrong: %+v", conns[0])
	}

	byID, err := c.LookupRuntimeConnection(ctx, "ext-a")
	if err != nil || byID.Provider != "github" || byID.ProviderAccountID != "acct-a" || byID.Status != "active" {
		t.Fatalf("by-id = %+v, %v", byID, err)
	}

	cr, err := c.PollConnectionRequest(ctx, "cr-1")
	if err != nil || cr.Status != "connected" || cr.AppID != "ext-a" || cr.Service != "github" {
		t.Fatalf("poll = %+v, %v", cr, err)
	}

	created, err := c.ConnectRuntimeAPIKey(ctx, "github", "ghp_plain")
	if err != nil || created.ID != "ext-k" || created.Alias != "alias-k" {
		t.Fatalf("api-key connect = %+v, %v", created, err)
	}
	if apiPath != "/v1/connections/github/connect/api-key" {
		t.Fatalf("api-key path = %q", apiPath)
	}
	var sent map[string]string
	_ = json.Unmarshal(apiBody, &sent)
	if sent["apiKey"] != "ghp_plain" {
		t.Fatalf("api-key body = %v", sent)
	}
}

func TestAdminClientRuntimeFailureSurfacesErrorCode(t *testing.T) {
	srv, _ := newAdminTestServer(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"success":false,"message":"Not found.","errorCode":"connection_not_found","data":null,"meta":{}}`)
	})
	c := mustClient(t, testClientConfig(srv.URL, staticSecret(testAdminSecret)))
	_, err := c.LookupRuntimeConnection(context.Background(), "ext-missing")
	var ae *AdminError
	if !errors.As(err, &ae) || ae.Status != http.StatusNotFound || ae.Code != "connection_not_found" {
		t.Fatalf("runtime failure mapping = %v", err)
	}
}
