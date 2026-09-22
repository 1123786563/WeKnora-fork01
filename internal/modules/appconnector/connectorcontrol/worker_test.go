package connectorcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	repoapp "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// ---------------------------------------------------------------------------
// fixtures
// ---------------------------------------------------------------------------

// newWorkerTestDB opens a real sqlite database (file-backed, WAL) carrying the
// production OC migration from migrations/sqlite/000041 — the same frozen
// schema production uses — plus the parent installation tables the binding
// foreign keys require. File-backed (not :memory:) so concurrent claims from
// two workers serialize the way lite-mode production would.
func newWorkerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	raw, err := os.ReadFile("../../../../migrations/sqlite/000041_open_connector_bindings.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	dsn := filepath.Join(t.TempDir(), "oc-worker.db") + "?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=1&_txlock=immediate"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:  gormlogger.Discard,
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&repoapp.AppVersion{}, &repoapp.InstallationRow{}, &repoapp.ConnectionRow{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(raw)).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func seedConnectionRow(t *testing.T, db *gorm.DB, tenant uint64, connID string, authVersion int64) {
	t.Helper()
	instID := fmt.Sprintf("inst-%d", tenant)
	db.Exec("INSERT OR IGNORE INTO app_versions (app_id, version) VALUES ('app-oc','1.0.0')")
	db.Exec("INSERT OR IGNORE INTO installations (id, tenant_id, app_id, app_version, state) VALUES (?,?,'app-oc','1.0.0','active')", instID, tenant)
	if err := db.Exec("INSERT INTO connections (tenant_id, id, installation_id, kind, owner_id, credential_ref, state, auth_version) VALUES (?,?,?,?, 'owner', 'ref/cred', 'active', ?)",
		tenant, connID, instID, appconn.ConnectionKindSpace, authVersion).Error; err != nil {
		t.Fatal(err)
	}
}

func seedBinding(t *testing.T, db *gorm.DB, tenant uint64, connID, state string, authVersion int64) {
	t.Helper()
	store := repoapp.NewOCStore(db)
	b := appconn.OCBinding{
		TenantID: tenant, ConnectionID: connID, RuntimeID: "rt-1", Provider: "github",
		ExternalID: "ext-1", Alias: "alias-1", AuthVersion: authVersion, BindingVersion: 1, State: state,
	}
	if err := store.SaveBinding(context.Background(), b); err != nil {
		t.Fatal(err)
	}
}

func seedOp(t *testing.T, db *gorm.DB, id, kind string, tenant uint64, connID string, version, attempts int64, due time.Time) {
	t.Helper()
	if err := db.Exec("INSERT INTO connector_operations_outbox (id, tenant_id, resource_id, resource_version, kind, attempts, next_at) VALUES (?,?,?,?,?,?,?)",
		id, tenant, connID, version, kind, attempts, due).Error; err != nil {
		t.Fatal(err)
	}
}

func outboxRow(t *testing.T, db *gorm.DB, id string) *repoapp.OCOperationsOutboxRow {
	t.Helper()
	var row repoapp.OCOperationsOutboxRow
	if err := db.Where("id = ?", id).First(&row).Error; err != nil {
		return nil
	}
	return &row
}

// fakeAdmin counts admin-surface calls; it never performs HTTP.
type fakeAdmin struct {
	mu           sync.Mutex
	calls        int
	kindCalls    map[string]int
	createReqs   []CreateTokenRequest
	revokeIDs    []string
	createResult RuntimeTokenCreated
	err          error
	delay        time.Duration
}

func newFakeAdmin() *fakeAdmin { return &fakeAdmin{kindCalls: map[string]int{}} }

func (f *fakeAdmin) record(kind string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.kindCalls[kind]++
}

func (f *fakeAdmin) snapshot() (int, map[string]int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[string]int, len(f.kindCalls))
	for k, v := range f.kindCalls {
		out[k] = v
	}
	return f.calls, out
}

func (f *fakeAdmin) CreateRuntimeToken(ctx context.Context, req CreateTokenRequest) (RuntimeTokenCreated, error) {
	f.record(KindCreateToken)
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return RuntimeTokenCreated{}, f.err
	}
	f.createReqs = append(f.createReqs, req)
	return f.createResult, nil
}

func (f *fakeAdmin) RevokeRuntimeToken(ctx context.Context, tokenRecordID string) error {
	f.record(KindDeleteToken)
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.revokeIDs = append(f.revokeIDs, tokenRecordID)
	return nil
}

func (f *fakeAdmin) StartAuthorization(ctx context.Context, service, connectionName string) (AuthorizationStart, error) {
	f.record(KindAuthorize)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return AuthorizationStart{}, f.err
	}
	return AuthorizationStart{URL: "https://ext.example/auth", State: "st-1"}, nil
}

func (f *fakeAdmin) LookupRuntimeConnection(ctx context.Context, appID string) (RuntimeConnection, error) {
	f.record(KindConfirm)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return RuntimeConnection{}, f.err
	}
	return RuntimeConnection{ID: appID, Alias: "alias-1", ProviderAccountID: "acct-1"}, nil
}

func (f *fakeAdmin) DeleteRuntimeConnection(ctx context.Context, appID string) error {
	f.record(KindDeleteConnection)
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}

type fakeSink struct {
	mu      sync.Mutex
	refs    []string
	secrets []string
	err     error
}

func (f *fakeSink) PutSecret(ctx context.Context, ref, secret string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.refs = append(f.refs, ref)
	f.secrets = append(f.secrets, secret)
	return nil
}

func staticSecret(v string) AdminSecret {
	return func(context.Context) (string, error) { return v, nil }
}

// recordingOutbox wraps the real store so tests can assert per-operation
// claim/complete counts on top of real transactions.
type recordingOutbox struct {
	inner    OutboxStore
	mu       sync.Mutex
	claims   map[string]int
	complete map[string]int
	extends  map[string]int
}

func newRecordingOutbox(inner OutboxStore) *recordingOutbox {
	return &recordingOutbox{
		inner:    inner,
		claims:   map[string]int{},
		complete: map[string]int{},
		extends:  map[string]int{},
	}
}

func (r *recordingOutbox) ClaimNextOperation(ctx context.Context, owner string, now time.Time, lease time.Duration) (*repoapp.OCOperationsOutboxRow, error) {
	row, err := r.inner.ClaimNextOperation(ctx, owner, now, lease)
	if row != nil {
		r.mu.Lock()
		r.claims[row.ID]++
		r.mu.Unlock()
	}
	return row, err
}

func (r *recordingOutbox) CompleteOperation(ctx context.Context, id, owner string, fence int64) (bool, error) {
	ok, err := r.inner.CompleteOperation(ctx, id, owner, fence)
	if ok {
		r.mu.Lock()
		r.complete[id]++
		r.mu.Unlock()
	}
	return ok, err
}

func (r *recordingOutbox) ExtendOperation(ctx context.Context, id, owner string, fence int64, until, now time.Time) (bool, error) {
	ok, err := r.inner.ExtendOperation(ctx, id, owner, fence, until, now)
	if ok {
		r.mu.Lock()
		r.extends[id]++
		r.mu.Unlock()
	}
	return ok, err
}

func newTestWorker(owner string, outbox OutboxStore, bindings ControlStore, admin RuntimeAdmin, sink SecretSink, secret AdminSecret) *ControlWorker {
	w, err := NewControlWorker(outbox, bindings, admin, sink, secret, WorkerConfig{
		OwnerID:       owner,
		Lease:         30 * time.Second,
		RenewInterval: 10 * time.Second,
		BackoffBase:   2 * time.Second,
		BackoffMax:    5 * time.Minute,
	})
	if err != nil {
		panic(err)
	}
	w.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	w.jitter = func() float64 { return 0 }
	return w
}

// ---------------------------------------------------------------------------
// validation (plan sketch, verbatim)
// ---------------------------------------------------------------------------

func TestWorkerRejectsUnscopedOperation(t *testing.T) {
	op := ControlOperation{ID: "op1", ConnectionID: "c", Kind: "delete_token", AuthVersion: 1}
	if err := ValidateControlOperation(op); err == nil {
		t.Fatal("global operation accepted")
	}
}

func TestWorkerValidateControlOperationKinds(t *testing.T) {
	valid := []string{"authorize", "create_token", "delete_token", "delete_connection", "confirm"}
	for _, kind := range valid {
		op := ControlOperation{ID: "op1", TenantID: 7, ConnectionID: "c", Kind: kind, AuthVersion: 1}
		if err := ValidateControlOperation(op); err != nil {
			t.Fatalf("kind %q rejected: %v", kind, err)
		}
	}
	invalid := []struct {
		name string
		op   ControlOperation
	}{
		{"missing id", ControlOperation{TenantID: 7, ConnectionID: "c", Kind: "confirm", AuthVersion: 1}},
		{"zero tenant", ControlOperation{ID: "op1", ConnectionID: "c", Kind: "confirm", AuthVersion: 1}},
		{"missing connection", ControlOperation{ID: "op1", TenantID: 7, Kind: "confirm", AuthVersion: 1}},
		{"zero auth version", ControlOperation{ID: "op1", TenantID: 7, ConnectionID: "c", Kind: "confirm"}},
		{"negative auth version", ControlOperation{ID: "op1", TenantID: 7, ConnectionID: "c", Kind: "confirm", AuthVersion: -1}},
		{"generic http kind", ControlOperation{ID: "op1", TenantID: 7, ConnectionID: "c", Kind: "http_post", AuthVersion: 1}},
		{"empty kind", ControlOperation{ID: "op1", TenantID: 7, ConnectionID: "c", Kind: "", AuthVersion: 1}},
		{"refresh kind", ControlOperation{ID: "op1", TenantID: 7, ConnectionID: "c", Kind: "refresh", AuthVersion: 1}},
	}
	for _, tc := range invalid {
		if err := ValidateControlOperation(tc.op); err == nil {
			t.Fatalf("%s: accepted invalid operation %+v", tc.name, tc.op)
		}
	}
}

// ---------------------------------------------------------------------------
// store claim/complete/extend semantics (sqlite-expressible part; the real
// FOR UPDATE SKIP LOCKED competition runs on PostgreSQL, see worker_pg_test.go)
// ---------------------------------------------------------------------------

func TestWorkerClaimExcludesSecondWorkerUntilLeaseExpiry(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	due := time.Unix(1699990000, 0).UTC()
	seedOp(t, db, "op1", KindCreateToken, 7, "conn1", 1, 0, due)
	store := repoapp.NewOCStore(db)

	now := time.Unix(1700000000, 0).UTC()
	a, err := store.ClaimNextOperation(ctx, "wA", now, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if a == nil || a.ID != "op1" {
		t.Fatalf("worker A did not claim: %+v", a)
	}
	if a.Fence != 1 || a.Attempts != 1 || a.LeaseOwner != "wA" {
		t.Fatalf("claim did not bump lease/fence/attempts: %+v", a)
	}
	if !a.NextAt.Equal(now.Add(30 * time.Second)) {
		t.Fatalf("claim must push next_at to lease expiry, got %v", a.NextAt)
	}

	// While the lease is held, the second worker claims nothing.
	b, err := store.ClaimNextOperation(ctx, "wB", now.Add(time.Second), 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if b != nil {
		t.Fatalf("worker B claimed a leased operation: %+v", b)
	}

	// After lease expiry, B takes over with a NEW fence.
	c, err := store.ClaimNextOperation(ctx, "wB", now.Add(31*time.Second), 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if c == nil || c.Fence != 2 || c.Attempts != 2 || c.LeaseOwner != "wB" {
		t.Fatalf("takeover claim wrong: %+v", c)
	}

	// The stale worker A's complete updates ZERO rows.
	okA, err := store.CompleteOperation(ctx, "op1", "wA", 1)
	if err != nil {
		t.Fatal(err)
	}
	if okA {
		t.Fatal("stale worker complete affected rows")
	}
	if row := outboxRow(t, db, "op1"); row == nil {
		t.Fatal("stale complete must not delete the row")
	}

	// The current owner completes.
	okB, err := store.CompleteOperation(ctx, "op1", "wB", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !okB {
		t.Fatal("current owner complete failed")
	}
	if row := outboxRow(t, db, "op1"); row != nil {
		t.Fatal("completed operation still present")
	}
}

func TestWorkerExtendOnlyCurrentOwnerAndFence(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	seedOp(t, db, "op1", KindDeleteToken, 7, "conn1", 1, 0, time.Unix(1699990000, 0).UTC())
	store := repoapp.NewOCStore(db)

	now := time.Unix(1700000000, 0).UTC()
	a, _ := store.ClaimNextOperation(ctx, "wA", now, 30*time.Second)
	if a == nil {
		t.Fatal("no claim")
	}
	// wrong owner
	ok, err := store.ExtendOperation(ctx, "op1", "wX", a.Fence, now.Add(time.Minute), now)
	if err != nil || ok {
		t.Fatalf("extend with wrong owner must affect 0 rows: ok=%v err=%v", ok, err)
	}
	// wrong fence
	ok, err = store.ExtendOperation(ctx, "op1", "wA", a.Fence+5, now.Add(time.Minute), now)
	if err != nil || ok {
		t.Fatalf("extend with wrong fence must affect 0 rows: ok=%v err=%v", ok, err)
	}
	// correct owner+fence
	ok, err = store.ExtendOperation(ctx, "op1", "wA", a.Fence, now.Add(time.Minute), now)
	if err != nil || !ok {
		t.Fatalf("extend by owner failed: ok=%v err=%v", ok, err)
	}
	if row := outboxRow(t, db, "op1"); row == nil || !row.NextAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("extend did not move next_at: %+v", row)
	}
}

func TestWorkerTwoWorkersEachOperationClaimedOnce(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	due := time.Unix(1699990000, 0).UTC()
	const n = 6
	for i := 0; i < n; i++ {
		seedOp(t, db, fmt.Sprintf("op%d", i), KindDeleteToken, 7, "conn1", 1, 0, due)
	}
	admin := newFakeAdmin()
	rec := newRecordingOutbox(repoapp.NewOCStore(db))
	wA := newTestWorker("wA", rec, repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("admin-secret-1"))
	wB := newTestWorker("wB", rec, repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("admin-secret-1"))

	// Alternate both workers over the same queue until it is drained.
	for i := 0; i < n; i++ {
		w := wA
		if i%2 == 1 {
			w = wB
		}
		if err := w.RunOnce(ctx); err != nil {
			t.Fatalf("worker tick %d failed: %v", i, err)
		}
	}
	// Both workers idle afterwards.
	if err := wA.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err := wB.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.claims) != n {
		t.Fatalf("expected %d distinct claimed operations, got %v", n, rec.claims)
	}
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("op%d", i)
		if rec.claims[id] != 1 {
			t.Fatalf("operation %s claimed %d times (want exactly 1)", id, rec.claims[id])
		}
		if rec.complete[id] != 1 {
			t.Fatalf("operation %s completed %d times (want exactly 1)", id, rec.complete[id])
		}
	}
	calls, _ := admin.snapshot()
	if calls != 0 {
		t.Fatalf("payload-less operations must not reach the admin client, got %d calls", calls)
	}
}

// ---------------------------------------------------------------------------
// pre-execution rejections (admin client untouched)
// ---------------------------------------------------------------------------

func TestWorkerPoisonKindDroppedWithoutAdminCall(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	seedOp(t, db, "op1", "http_post", 7, "conn1", 1, 0, time.Unix(1699990000, 0).UTC())
	admin := newFakeAdmin()
	w := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("admin-secret-1"))
	if err := w.RunOnce(ctx); err != nil {
		t.Fatalf("poison operation must be dropped silently, got %v", err)
	}
	if row := outboxRow(t, db, "op1"); row != nil {
		t.Fatal("poison operation must be removed from the outbox")
	}
	if calls, _ := admin.snapshot(); calls != 0 {
		t.Fatalf("poison operation touched the admin client %d times", calls)
	}
}

func TestWorkerRevokedBindingNotReactivatedByOldCreateOp(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	seedConnectionRow(t, db, 7, "conn1", 1)
	seedBinding(t, db, 7, "conn1", appconn.OCBindingRevoked, 1)
	seedOp(t, db, "op1", KindCreateToken, 7, "conn1", 1, 0, time.Unix(1699990000, 0).UTC())
	admin := newFakeAdmin()
	w := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("admin-secret-1"))
	if err := w.RunOnce(ctx); err != nil {
		t.Fatalf("dropped operation must not error: %v", err)
	}
	if row := outboxRow(t, db, "op1"); row != nil {
		t.Fatal("operation against a revoked binding must be dropped")
	}
	if calls, kinds := admin.snapshot(); calls != 0 {
		t.Fatalf("revoked binding operation reached admin client: %v", kinds)
	}
	// The binding itself must remain revoked.
	b, err := repoapp.NewOCStore(db).GetBinding(ctx, 7, "conn1")
	if err != nil {
		t.Fatal(err)
	}
	if b.State != appconn.OCBindingRevoked {
		t.Fatalf("binding state changed: %s", b.State)
	}
}

func TestWorkerStaleAuthVersionDroppedWithoutAdminCall(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	seedConnectionRow(t, db, 7, "conn1", 2)
	seedBinding(t, db, 7, "conn1", appconn.OCBindingActive, 2)
	// Operation pinned to the superseded auth generation.
	seedOp(t, db, "op1", KindCreateToken, 7, "conn1", 1, 0, time.Unix(1699990000, 0).UTC())
	admin := newFakeAdmin()
	w := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("admin-secret-1"))
	if err := w.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if row := outboxRow(t, db, "op1"); row != nil {
		t.Fatal("stale operation must be dropped")
	}
	if calls, _ := admin.snapshot(); calls != 0 {
		t.Fatal("stale operation touched the admin client")
	}
}

func TestWorkerMissingBindingDropsOperation(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	seedOp(t, db, "op1", KindCreateToken, 7, "conn1", 1, 0, time.Unix(1699990000, 0).UTC())
	admin := newFakeAdmin()
	w := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("admin-secret-1"))
	if err := w.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if row := outboxRow(t, db, "op1"); row != nil {
		t.Fatal("operation without a binding must be dropped")
	}
	if calls, _ := admin.snapshot(); calls != 0 {
		t.Fatal("operation without a binding touched the admin client")
	}
}

// ---------------------------------------------------------------------------
// admin secret isolation (fail-closed)
// ---------------------------------------------------------------------------

func TestWorkerRefusesAdminOperationsWithoutSecretMount(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	seedOp(t, db, "op1", KindDeleteToken, 7, "conn1", 1, 0, time.Unix(1699990000, 0).UTC())
	admin := newFakeAdmin()
	// API-process-style environment: the dedicated admin secret mount is absent.
	missing := FileAdminSecretSource(filepath.Join(t.TempDir(), "no-such-mount", "connector-admin-token"))
	rec := newRecordingOutbox(repoapp.NewOCStore(db))
	w := newTestWorker("wA", rec, repoapp.NewOCStore(db), admin, &fakeSink{}, missing)

	err := w.RunOnce(ctx)
	if err == nil {
		t.Fatal("worker without the admin secret mount must refuse the operation")
	}
	if !errors.Is(err, ErrAdminSecretUnavailable) {
		t.Fatalf("expected ErrAdminSecretUnavailable, got %v", err)
	}
	if calls, _ := admin.snapshot(); calls != 0 {
		t.Fatal("worker without secret mount touched the admin client")
	}
	row := outboxRow(t, db, "op1")
	if row == nil {
		t.Fatal("refused operation must stay queued, not be dropped")
	}
	// The operation was NOT completed and was pushed to a retry deadline.
	base := time.Unix(1700000000, 0).UTC()
	if !row.NextAt.Equal(base.Add(2 * time.Second)) {
		t.Fatalf("refused operation not scheduled for retry: next_at=%v", row.NextAt)
	}
	// The error text must never contain secret material.
	if got := err.Error(); strings.Contains(got, "admin-secret") {
		t.Fatalf("error leaks secret-like material: %q", got)
	}
}

// ---------------------------------------------------------------------------
// kind dispatch (payload-driven paths driven through runClaimed, because the
// frozen outbox schema has no payload column — enqueue-side payload
// persistence is wired by T07)
// ---------------------------------------------------------------------------

func claimForRun(t *testing.T, db *gorm.DB, owner string) (*repoapp.OCOperationsOutboxRow, ControlOperation) {
	t.Helper()
	row, err := repoapp.NewOCStore(db).ClaimNextOperation(context.Background(), owner, time.Unix(1700000000, 0).UTC(), 30*time.Second)
	if err != nil || row == nil {
		t.Fatalf("claim failed: %+v %v", row, err)
	}
	return row, OperationFromRow(row)
}

func TestWorkerCreateTokenStoresRefNotMaterial(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	seedConnectionRow(t, db, 7, "conn1", 1)
	seedBinding(t, db, 7, "conn1", appconn.OCBindingActive, 1)
	seedOp(t, db, "op1", KindCreateToken, 7, "conn1", 1, 0, time.Unix(1699990000, 0).UTC())

	admin := newFakeAdmin()
	admin.createResult = RuntimeTokenCreated{TokenRecordID: "rtt_123", Token: "oct_MATERIAL_SECRET"}
	sink := &fakeSink{}
	w := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), admin, sink, staticSecret("admin-secret-1"))

	row, op := claimForRun(t, db, "wA")
	op.Payload = json.RawMessage(`{"actions":["github.get_profile"]}`)
	if err := w.runClaimed(ctx, row, op); err != nil {
		t.Fatal(err)
	}
	if calls, kinds := admin.snapshot(); calls != 1 || kinds[KindCreateToken] != 1 {
		t.Fatalf("expected exactly one create call, got %v", kinds)
	}
	admin.mu.Lock()
	if len(admin.createReqs) != 1 {
		admin.mu.Unlock()
		t.Fatal("create request not recorded")
	}
	req := admin.createReqs[0]
	admin.mu.Unlock()
	if req.ExternalID != "ext-1" || len(req.Actions) != 1 || req.Actions[0] != "github.get_profile" {
		t.Fatalf("create request wrong: %+v", req)
	}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if len(sink.refs) != 1 || len(sink.secrets) != 1 {
		t.Fatal("token material not stored exactly once")
	}
	if sink.secrets[0] != "oct_MATERIAL_SECRET" {
		t.Fatal("sink must receive the raw token material")
	}
	wantRef := "oc/runtime-token/7/conn1/1"
	if sink.refs[0] != wantRef {
		t.Fatalf("secret ref = %q, want %q", sink.refs[0], wantRef)
	}
	if row := outboxRow(t, db, "op1"); row != nil {
		t.Fatal("operation not completed")
	}
}

func TestWorkerCreateTokenEmptyActionsNeverMints(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	seedConnectionRow(t, db, 7, "conn1", 1)
	seedBinding(t, db, 7, "conn1", appconn.OCBindingActive, 1)
	seedOp(t, db, "op1", KindCreateToken, 7, "conn1", 1, 0, time.Unix(1699990000, 0).UTC())

	admin := newFakeAdmin()
	w := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("admin-secret-1"))
	row, op := claimForRun(t, db, "wA")
	op.Payload = json.RawMessage(`{"actions":[]}`)
	if err := w.runClaimed(ctx, row, op); err != nil {
		t.Fatalf("empty grant must be a permanent drop, got %v", err)
	}
	if calls, _ := admin.snapshot(); calls != 0 {
		t.Fatal("empty grant must never reach the admin client")
	}
	if row := outboxRow(t, db, "op1"); row != nil {
		t.Fatal("empty-grant operation must be dropped, not retried")
	}
}

func TestWorkerDeleteTokenRevokesByRecordID(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	seedOp(t, db, "op1", KindDeleteToken, 7, "conn1", 1, 0, time.Unix(1699990000, 0).UTC())
	admin := newFakeAdmin()
	w := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("admin-secret-1"))
	row, op := claimForRun(t, db, "wA")
	op.Payload = json.RawMessage(`{"tokenID":"rtt_123"}`)
	if err := w.runClaimed(ctx, row, op); err != nil {
		t.Fatal(err)
	}
	if calls, kinds := admin.snapshot(); calls != 1 || kinds[KindDeleteToken] != 1 {
		t.Fatalf("expected one revoke call, got %v", kinds)
	}
	admin.mu.Lock()
	defer admin.mu.Unlock()
	if len(admin.revokeIDs) != 1 || admin.revokeIDs[0] != "rtt_123" {
		t.Fatalf("revoke ids: %v", admin.revokeIDs)
	}
}

func TestWorkerFailureSchedulesExponentialBackoffCapped(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	seedOp(t, db, "op1", KindDeleteToken, 7, "conn1", 1, 0, time.Unix(1699990000, 0).UTC())
	admin := newFakeAdmin()
	admin.err = errors.New("connectorcontrol: admin api status 503 code unavailable")
	w := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("admin-secret-1"))

	row, op := claimForRun(t, db, "wA")
	op.Payload = json.RawMessage(`{"tokenID":"rtt_123"}`)
	if err := w.runClaimed(ctx, row, op); err == nil {
		t.Fatal("failed execution must surface its error")
	}
	kept := outboxRow(t, db, "op1")
	if kept == nil {
		t.Fatal("failed operation must stay queued")
	}
	base := time.Unix(1700000000, 0).UTC()
	if !kept.NextAt.Equal(base.Add(2 * time.Second)) {
		t.Fatalf("first retry backoff = %v, want %v", kept.NextAt, base.Add(2*time.Second))
	}

	// A long-failing operation caps at BackoffMax.
	db2 := newWorkerTestDB(t)
	seedOp(t, db2, "op2", KindDeleteToken, 7, "conn1", 1, 25, time.Unix(1699990000, 0).UTC())
	admin2 := newFakeAdmin()
	admin2.err = errors.New("connectorcontrol: admin api status 503 code unavailable")
	w2 := newTestWorker("wA", repoapp.NewOCStore(db2), repoapp.NewOCStore(db2), admin2, &fakeSink{}, staticSecret("admin-secret-1"))
	row2, op2 := claimForRun(t, db2, "wA")
	op2.Payload = json.RawMessage(`{"tokenID":"rtt_9"}`)
	if err := w2.runClaimed(ctx, row2, op2); err == nil {
		t.Fatal("expected failure")
	}
	kept2 := outboxRow(t, db2, "op2")
	if kept2 == nil {
		t.Fatal("operation must stay queued")
	}
	if !kept2.NextAt.Equal(base.Add(5 * time.Minute)) {
		t.Fatalf("capped backoff = %v, want %v", kept2.NextAt, base.Add(5*time.Minute))
	}
}

// TestWorkerAuthorizeAndConfirmRejectUncorrelatedRows pins the WIRED
// semantics (T07, per coordinator R13.1): the authorize/confirm kinds are
// correlation-driven, and a row whose resource_id resolves to no
// authorization attempt is a PERMANENT rejection - dropped, never retried,
// never touching the admin client. (Pre-T07 this test pinned the held
// ErrKindNotWiredYet placeholder; the wiring it announced is now in.)
func TestWorkerAuthorizeAndConfirmHeldUntilCorrelationWiring(t *testing.T) {
	for _, kind := range []string{KindAuthorize, KindConfirm} {
		db := newWorkerTestDB(t)
		ctx := context.Background()
		seedConnectionRow(t, db, 7, "conn1", 1)
		seedBinding(t, db, 7, "conn1", appconn.OCBindingActive, 1)
		seedOp(t, db, "op1", kind, 7, "conn1", 1, 0, time.Unix(1699990000, 0).UTC())
		admin := newFakeAdmin()
		w := newTestWorker("wA", repoapp.NewOCStore(db), repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("admin-secret-1"))
		if err := w.RunOnce(ctx); err != nil {
			t.Fatalf("%s: uncorrelated row must be a permanent drop, got %v", kind, err)
		}
		if calls, _ := admin.snapshot(); calls != 0 {
			t.Fatalf("%s: must not touch admin client without a correlated attempt", kind)
		}
		if row := outboxRow(t, db, "op1"); row != nil {
			t.Fatalf("%s: uncorrelated row must be removed, not retried", kind)
		}
	}
}

func TestWorkerRenewalExtendsLeaseDuringExecution(t *testing.T) {
	db := newWorkerTestDB(t)
	ctx := context.Background()
	seedOp(t, db, "op1", KindDeleteToken, 7, "conn1", 1, 0, time.Unix(1699990000, 0).UTC())
	admin := newFakeAdmin()
	admin.delay = 60 * time.Millisecond
	rec := newRecordingOutbox(repoapp.NewOCStore(db))
	w := newTestWorker("wA", rec, repoapp.NewOCStore(db), admin, &fakeSink{}, staticSecret("admin-secret-1"))
	w.cfg.Lease = 50 * time.Millisecond
	w.cfg.RenewInterval = 10 * time.Millisecond

	row, op := claimForRun(t, db, "wA")
	op.Payload = json.RawMessage(`{"tokenID":"rtt_123"}`)
	if err := w.runClaimed(ctx, row, op); err != nil {
		t.Fatal(err)
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.extends["op1"] < 2 {
		t.Fatalf("expected the lease to be renewed during execution, got %d extends", rec.extends["op1"])
	}
	if rec.complete["op1"] != 1 {
		t.Fatal("renewed lease must still complete")
	}
}

var _ = errors.New

// ---------------------------------------------------------------------------
// T09 HARD PREREQUISITE: mint-path reconciliation revoke guard (T08 dual
// review Q-1/F-1). The two delete paths already treat an admin 404 from
// RevokeRuntimeToken as idempotent success; the mint reconciliation revoke
// must too — a retried mint whose orphaned prior token was ALREADY revoked
// upstream (404) must proceed with the re-cast instead of failing forever.
// ---------------------------------------------------------------------------

// readBackSink is a SecretSink with a working read side (secretReader), so
// mintRuntimeToken takes the reconciliation path.
type readBackSink struct {
	mu   sync.Mutex
	data map[string]string
	puts []string
}

func (s *readBackSink) PutSecret(ctx context.Context, ref, secret string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = map[string]string{}
	}
	s.data[ref] = secret
	s.puts = append(s.puts, ref)
	return nil
}

func (s *readBackSink) GetSecret(ctx context.Context, ref string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data[ref], nil
}

// reconcileAdmin injects an independent revoke result while recording every
// admin call, so the mint seam can be driven with precise errors.
type reconcileAdmin struct {
	mu        sync.Mutex
	revokeErr error
	revoked   []string
	created   int
}

func (a *reconcileAdmin) CreateRuntimeToken(ctx context.Context, req CreateTokenRequest) (RuntimeTokenCreated, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.created++
	return RuntimeTokenCreated{TokenRecordID: "rec-new", Token: "tok-new"}, nil
}

func (a *reconcileAdmin) RevokeRuntimeToken(ctx context.Context, tokenRecordID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.revoked = append(a.revoked, tokenRecordID)
	return a.revokeErr
}

func (a *reconcileAdmin) StartAuthorization(ctx context.Context, service, connectionName string) (AuthorizationStart, error) {
	return AuthorizationStart{}, nil
}

func (a *reconcileAdmin) LookupRuntimeConnection(ctx context.Context, appID string) (RuntimeConnection, error) {
	return RuntimeConnection{}, nil
}

func (a *reconcileAdmin) DeleteRuntimeConnection(ctx context.Context, appID string) error {
	return nil
}

// TestMintReconcileTreatsAlreadyRevokedAsSuccess drives the
// reconcile-retry-after-404 scenario at the mint seam: the sink read back a
// prior record id (an earlier mint whose material persist failed and whose
// row is retrying) and the remote token is ALREADY revoked — the admin 404
// must count as done and the replacement token must be cast and persisted.
func TestMintReconcileTreatsAlreadyRevokedAsSuccess(t *testing.T) {
	sink := &readBackSink{data: map[string]string{
		"oc/runtime-token/7/conn-1/3/record-id": "rec-prior",
	}}
	admin := &reconcileAdmin{revokeErr: &AdminError{Status: http.StatusNotFound, Code: "token_not_found"}}
	w := &ControlWorker{admin: admin, secrets: sink}

	if err := w.mintRuntimeToken(context.Background(), 7, "conn-1", 3, "weknora-7-conn-1-v3", "ext-1", []string{"github.search"}); err != nil {
		t.Fatalf("already-revoked orphan blocked the re-cast: %v", err)
	}
	admin.mu.Lock()
	created, revoked := admin.created, append([]string(nil), admin.revoked...)
	admin.mu.Unlock()
	if created != 1 {
		t.Fatalf("replacement token not minted (created=%d)", created)
	}
	if len(revoked) != 1 || revoked[0] != "rec-prior" {
		t.Fatalf("prior record id not reconciled: %v", revoked)
	}
	sink.mu.Lock()
	material := sink.data["oc/runtime-token/7/conn-1/3"]
	record := sink.data["oc/runtime-token/7/conn-1/3/record-id"]
	sink.mu.Unlock()
	if material != "tok-new" || record != "rec-new" {
		t.Fatalf("replacement token not persisted (material=%q record=%q)", material, record)
	}
}

// TestMintReconcileStillFailsOnRealErrors pins that only the 404 is
// idempotent: any other revoke failure still aborts the mint BEFORE a second
// token is cast, exactly like the two delete paths in the same file.
func TestMintReconcileStillFailsOnRealErrors(t *testing.T) {
	sink := &readBackSink{data: map[string]string{
		"oc/runtime-token/7/conn-1/3/record-id": "rec-prior",
	}}
	admin := &reconcileAdmin{revokeErr: &AdminError{Status: http.StatusInternalServerError, Code: "runtime_unavailable"}}
	w := &ControlWorker{admin: admin, secrets: sink}

	if err := w.mintRuntimeToken(context.Background(), 7, "conn-1", 3, "weknora-7-conn-1-v3", "ext-1", []string{"github.search"}); err == nil {
		t.Fatal("real revoke failure was swallowed")
	}
	admin.mu.Lock()
	created := admin.created
	admin.mu.Unlock()
	if created != 0 {
		t.Fatalf("second token cast while orphan state unknown (created=%d)", created)
	}
}
