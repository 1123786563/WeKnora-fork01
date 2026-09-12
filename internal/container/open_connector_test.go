package container

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/application/service/appconnector"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/router"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ocTestStore opens an in-memory OC store with the binding tables. Each
// distinct suffix gets its OWN in-memory database (shared-cache DSNs with
// the same name are literally the same database).
func ocTestStore(t *testing.T, suffix ...string) *repoappconn.OCStore {
	t.Helper()
	name := t.Name()
	for _, s := range suffix {
		name += "-" + s
	}
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repoappconn.OCBindingRow{}); err != nil {
		t.Fatal(err)
	}
	return repoappconn.NewOCStore(db)
}

func ocSeedBinding(t *testing.T, store *repoappconn.OCStore) {
	t.Helper()
	if err := store.SaveBinding(context.Background(), appconn.OCBinding{
		TenantID: 1, ConnectionID: "c1", RuntimeID: "rt", Provider: "github",
		ExternalID: "ext", Alias: "alias", AuthVersion: 2, BindingVersion: 1,
		State: appconn.OCBindingActive,
	}); err != nil {
		t.Fatal(err)
	}
}

// TestFileBackedOCTokenSourceIsVersionScoped pins the T11 carry: the
// production token source reads the RESTRICTED runtime token the control
// worker minted for EXACTLY (tenant, connection, authVersion) — it never
// mints, never falls back to another generation, and never touches the
// admin secret.
func TestFileBackedOCTokenSourceIsVersionScoped(t *testing.T) {
	store := ocTestStore(t)
	ocSeedBinding(t, store)
	dir := t.TempDir()
	ref := OCTokenRef(1, "c1", 2)
	// Place the material the way the control worker's FileSecretSink does.
	sink := ocTestSink{dir}
	if err := sink.put(ref, "oct_restricted_material"); err != nil {
		t.Fatal(err)
	}
	src, err := NewFileBackedOCTokenSource(sink, store)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	got, err := src.Token(ctx, 1, "c1", 2)
	if err != nil || got != "oct_restricted_material" {
		t.Fatalf("token=%q err=%v", got, err)
	}
	// Wrong generation: the live binding is at 2 — a caller asking for any
	// other version must be refused even though a file could exist.
	if _, err := src.Token(ctx, 1, "c1", 3); err == nil {
		t.Fatal("stale generation token issued")
	}
	// Foreign tenant: no binding there.
	if _, err := src.Token(ctx, 2, "c1", 2); err == nil {
		t.Fatal("cross-tenant token issued")
	}
	// Revoked binding: refused regardless of the file on disk.
	revoked := appconn.OCBinding{
		TenantID: 1, ConnectionID: "c1", RuntimeID: "rt", Provider: "github",
		ExternalID: "ext", Alias: "alias", AuthVersion: 3, BindingVersion: 2,
		State: appconn.OCBindingRevoked,
	}
	if err := store.SaveBinding(ctx, revoked); err != nil {
		t.Fatal(err)
	}
	if _, err := src.Token(ctx, 1, "c1", 3); err == nil {
		t.Fatal("revoked binding token issued")
	}
	// Missing material: fail closed, never an empty-string success.
	store2 := ocTestStore(t, "fresh")
	ocSeedBinding(t, store2) // fresh active binding at version 2 again
	empty, err := NewFileBackedOCTokenSource(ocTestSink{t.TempDir()}, store2)
	if err != nil {
		t.Fatal(err)
	}
	if tkn, err := empty.Token(ctx, 1, "c1", 2); err == nil || tkn != "" {
		t.Fatalf("missing material: token=%q err=%v", tkn, err)
	}
}

// ocTestSink is a minimal OCSecretReader/putter over a plain directory.
type ocTestSink struct{ dir string }

func (s ocTestSink) put(ref, material string) error {
	return os.WriteFile(filepath.Join(s.dir, ocSinkFileName(ref)), []byte(material), 0o600)
}

func (s ocTestSink) GetSecret(ctx context.Context, ref string) (string, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, ocSinkFileName(ref)))
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", errors.New("empty secret")
	}
	return string(data), nil
}

// TestArmOCDispatchEnforcesT10F3 pins the wiring MANDATE: a non-nil
// dispatcher without the durable claim store is a startup error, and the
// arming order wires claims+slots before/with the dispatcher.
func TestArmOCDispatchEnforcesT10F3(t *testing.T) {
	svc := appconnectorsvc.NewActionService(nil, nil, nil, nil, nil)
	real := &ocFakeDispatcher{}
	// dispatcher != nil && claims == nil → error.
	if err := ArmOCDispatch(svc, real, nil, nil); err == nil {
		t.Fatal("dispatcher without claims accepted")
	}
	// nil dispatcher (the disabled / explicit-refusing wiring) + no claims: fine.
	if err := ArmOCDispatch(svc, nil, nil, nil); err != nil {
		t.Fatalf("disabled wiring rejected: %v", err)
	}
}

type ocFakeDispatcher struct{ calls int32 }

func (d *ocFakeDispatcher) Dispatch(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	atomic.AddInt32(&d.calls, 1)
	return appconnectorsvc.DispatchOutcome{Status: appconn.ActionSucceeded}, nil
}

// TestWireOpenConnectorDisabledInjectsRefusingDispatcher: disabled config
// leaves the dispatcher nil (the ErrNoDispatcher refusal — Execute fails
// closed BEFORE consuming anything) and wires no claims/slots.
func TestWireOpenConnectorDisabledInjectsRefusingDispatcher(t *testing.T) {
	store := ocTestStore(t)
	svc := appconnectorsvc.NewActionService(nil, nil, nil, nil, nil)
	w, err := WireOpenConnector(svc, OCConfig{}, store, http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	if w.Enabled() || w.Dispatcher() != nil || w.Claims() != nil || w.Slots() != nil {
		t.Fatalf("disabled wiring armed a dispatch path: %+v", w)
	}
}

// TestWireOpenConnectorEnabledRequiresFullConfig: enabled-but-incomplete
// configuration is a STARTUP error — never a fallback to a default
// connection.
func TestWireOpenConnectorEnabledRequiresFullConfig(t *testing.T) {
	store := ocTestStore(t)
	svc := appconnectorsvc.NewActionService(nil, nil, nil, nil, nil)
	for name, cfg := range map[string]OCConfig{
		"no runtime addr": {Enabled: true, TokenDir: t.TempDir()},
		"no token dir":    {Enabled: true, RuntimeAddr: "http://127.0.0.1:8080"},
	} {
		if _, err := WireOpenConnector(svc, cfg, store, http.DefaultClient); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	// Complete config: the full path is armed — dispatcher + claims +
	// slots together (T10-F-3).
	complete := OCConfig{
		Enabled: true, RuntimeAddr: "http://127.0.0.1:8080",
		TokenDir: t.TempDir(), SlotOwner: "oc-test-owner",
	}
	w, err := WireOpenConnector(svc, complete, store, http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	if !w.Enabled() || w.Dispatcher() == nil || w.Claims() == nil || w.Slots() == nil {
		t.Fatal("enabled wiring incomplete")
	}
	if w.Supervisor() == nil {
		t.Fatal("no graceful-shutdown supervisor")
	}
}

// TestOCRecoveryRunnerRunsPeriodicallyAndStops: the periodic RunOnce caller
// is interval-bounded and context-cancellable (graceful-shutdown aware).
func TestOCRecoveryRunnerRunsPeriodicallyAndStops(t *testing.T) {
	rec := &ocCountingRecovery{}
	runner := NewOCRecoveryRunner(rec, 5*time.Millisecond, func(error) {})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { runner.Run(ctx); close(done) }()
	deadline := time.After(2 * time.Second)
	for atomic.LoadInt32(&rec.calls) < 3 {
		select {
		case <-deadline:
			t.Fatal("runner never ticked")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runner ignored shutdown")
	}
}

type ocCountingRecovery struct{ calls int32 }

func (r *ocCountingRecovery) RunOnce(ctx context.Context) error {
	atomic.AddInt32(&r.calls, 1)
	return nil
}

// TestOCSupervisorShutdownStopsClaimsDrainsThenRecovers pins the ruling-8
// sequence: (1) new claims stop, (2) in-flight dispatches get a bounded
// drain window, (3) whatever is still outstanding is handed to recovery
// (one final pass; the periodic stale sweep parks unfinished records
// unknown afterwards).
func TestOCSupervisorShutdownStopsClaimsDrainsThenRecovers(t *testing.T) {
	gate := &OCShutdownGate{}
	tracking := &TrackingOCDispatcher{inner: &ocFakeDispatcher{}}
	rec := &ocCountingRecovery{}
	sup := NewOCSupervisor(gate, tracking, rec, 150*time.Millisecond)

	// An in-flight dispatch blocks past the drain window.
	release := make(chan struct{})
	started := make(chan struct{})
	slow := &ocBlockingDispatcher{started: started, release: release}
	slowTrack := &TrackingOCDispatcher{inner: slow}
	go func() {
		_, _ = slowTrack.Dispatch(context.Background(), appconnectorsvc.ActionSnapshot{}, "")
	}()
	<-started

	shutdownDone := make(chan struct{})
	go func() {
		sup.Shutdown(context.Background())
		close(shutdownDone)
	}()
	// The claim gate is closed as soon as Shutdown starts.
	time.Sleep(30 * time.Millisecond)
	if !gate.Closed() {
		t.Fatal("claim gate still open during shutdown")
	}
	// Shutdown returns after the bounded drain even though the dispatch is
	// still in flight...
	select {
	case <-shutdownDone:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown blocked on in-flight dispatch beyond the drain window")
	}
	// ...and the final recovery pass ran (remainder → recovery).
	if atomic.LoadInt32(&rec.calls) == 0 {
		t.Fatal("no final recovery pass")
	}
	close(release)
}

type ocBlockingDispatcher struct {
	started chan struct{}
	release chan struct{}
}

func (d *ocBlockingDispatcher) Dispatch(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	close(d.started)
	<-d.release
	return appconnectorsvc.DispatchOutcome{Status: appconn.ActionUnknown}, nil
}

// TestGatedOCClaimsRefuseAfterClose: with the gate closed, ClaimOCDispatch
// refuses with the queue-limit sentinel (mapped to 429 at the edge);
// GetOCDispatch stays readable (recovery must keep working during drain).
func TestGatedOCClaimsRefuseAfterClose(t *testing.T) {
	gate := &OCShutdownGate{}
	inner := &ocStubClaims{}
	claims := &GatedOCClaims{OCDispatchClaimSource: inner, gate: gate}
	ctx := context.Background()
	if _, err := claims.ClaimOCDispatch(ctx, appconn.OCSubject{TenantID: 1, ActorID: "u"}, "a1", "res"); err != nil {
		t.Fatalf("open gate refused: %v", err)
	}
	gate.Close()
	_, err := claims.ClaimOCDispatch(ctx, appconn.OCSubject{TenantID: 1, ActorID: "u"}, "a1", "res")
	if err == nil || !errors.Is(err, repoappconn.ErrOCLeaseBusy) {
		t.Fatalf("closed gate error=%v", err)
	}
	if _, err := claims.GetOCDispatch(ctx, 1, "a1"); err != nil {
		t.Fatalf("reads must stay available during drain: %v", err)
	}
}

type ocStubClaims struct {
	mu      sync.Mutex
	claimed int
}

func (s *ocStubClaims) ClaimOCDispatch(ctx context.Context, subject appconn.OCSubject, actionID, reservationID string) (appconn.OCDispatchRecord, error) {
	s.mu.Lock()
	s.claimed++
	s.mu.Unlock()
	return appconn.OCDispatchRecord{TenantID: subject.TenantID, ActionID: actionID, Key: "wk-oc-x", RuntimeID: "rt", Fence: 1}, nil
}

func (s *ocStubClaims) GetOCDispatch(ctx context.Context, tenant uint64, actionID string) (appconn.OCDispatchRecord, error) {
	return appconn.OCDispatchRecord{TenantID: tenant, ActionID: actionID}, nil
}

// TestOCConfigFromEnvParsing covers the env contract incl. the disabled
// default.
func TestOCConfigFromEnvParsing(t *testing.T) {
	cfg := OCConfigFromEnv()
	if cfg.Enabled {
		t.Fatal("OC must be disabled by default")
	}
	t.Setenv(OCEnabledEnv, "true")
	t.Setenv(OCRuntimeAddrEnv, "http://open-connector:8080")
	t.Setenv(OCTokenDirEnv, "/var/run/weknora-oc-secrets")
	t.Setenv(OCRecoveryIntervalEnv, "7s")
	cfg = OCConfigFromEnv()
	if !cfg.Enabled || cfg.RuntimeAddr != "http://open-connector:8080" ||
		cfg.TokenDir != "/var/run/weknora-oc-secrets" || cfg.RecoveryInterval != 7*time.Second {
		t.Fatalf("cfg=%+v", cfg)
	}
	if cfg.DrainTimeout <= 0 {
		t.Fatal("drain timeout must default positive")
	}
}

// TestOCProductRoutesRegisterWithoutConflict proves the REAL registration
// (routes_app_connectors.go) accepts the new OC routes beside the R5
// baseline (revoke / approve / execute / get) without a gin wildcard
// panic — the failure mode only surfaces at engine construction, which no
// other test exercises.
func TestOCProductRoutesRegisterWithoutConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:oc-route-probe-"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	v1 := engine.Group("/api/v1")
	router.RegisterAppConnectorRoutes(v1,
		handler.NewAppInstallationHandler(db),
		handler.NewAppConnectionHandler(db),
		handler.NewAppSyncHandler(db),
		handler.NewAppActionHandler(db))
	registered := map[string]bool{}
	for _, route := range engine.Routes() {
		registered[route.Path] = true
	}
	for _, want := range []string{
		"/api/v1/apps/catalog",
		"/api/v1/apps/connections/:id/authorization-attempts",
		"/api/v1/apps/authorization-attempts/:id",
		"/api/v1/apps/oc/actions/prepare",
		// R5 baseline coexistence:
		"/api/v1/apps/connections/:id/revoke",
		"/api/v1/apps/actions/:id/approve",
		"/api/v1/apps/actions/:id/execute",
		"/api/v1/apps/actions/:id",
	} {
		if !registered[want] {
			t.Fatalf("route %s missing from the registered engine", want)
		}
	}
}
