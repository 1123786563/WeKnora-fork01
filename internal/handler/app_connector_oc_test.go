package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/application/service/appconnector"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/modules/commercial"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// T13 scaffolding: in-memory SQLite + a fully wired OC product surface.
// ---------------------------------------------------------------------------

// ocStubCredentialSource feeds the connection service's subject chain
// without touching any credential material (the API layer never sees any).
type ocStubCredentialSource struct {
	member bool
}

func (s *ocStubCredentialSource) FindConnectionByID(ctx context.Context, connectionID string) (appconnector.Connection, error) {
	var row appconnectorrepo.ConnectionRow
	if err := ocTestDBFrom(ctx).Where("id = ?", connectionID).First(&row).Error; err != nil {
		return appconnector.Connection{}, err
	}
	return appconnector.Connection{
		ID: row.ID, InstallationID: row.InstallationID, Kind: row.Kind,
		OwnerID: row.OwnerID, State: row.State, TenantID: row.TenantID, AuthVersion: row.AuthVersion,
	}, nil
}

func (s *ocStubCredentialSource) LoadCredential(ctx context.Context, c appconnector.Connection) ([]byte, error) {
	return nil, nil
}

func (s *ocStubCredentialSource) MemberActive(ctx context.Context, tenantID uint64, userID string) (bool, error) {
	return s.member, nil
}

func (s *ocStubCredentialSource) TryAcquireRefreshLease(ctx context.Context, c appconnector.Connection, leaseID string, until time.Time) (bool, error) {
	return false, nil
}

// ocTestDBKey carries the *gorm.DB through the request context for the stub
// credential source (the production store carries its own handle).
type ocTestDBKey struct{}

func ocTestDBFrom(ctx context.Context) *gorm.DB {
	return ctx.Value(ocTestDBKey{}).(*gorm.DB)
}

// ocStubDispatcher counts outbound HTTP-equivalent calls (never real ones).
type ocStubDispatcher struct {
	mu    sync.Mutex
	calls int
}

func (d *ocStubDispatcher) Dispatch(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	d.mu.Lock()
	d.calls++
	d.mu.Unlock()
	return appconnectorsvc.DispatchOutcome{Status: appconnector.ActionSucceeded, ProviderResult: "stub"}, nil
}

func (d *ocStubDispatcher) callsCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.calls
}

// ocCountingGate counts budget Begin/Finish calls (the unconfigured matrix
// requires BOTH to stay at zero).
type ocCountingGate struct {
	mu       sync.Mutex
	begins   int
	finishes int
}

func (g *ocCountingGate) Begin(ctx context.Context, req commercial.BudgetRequest) (commercial.Reservation, error) {
	g.mu.Lock()
	g.begins++
	g.mu.Unlock()
	return commercial.Reservation{ID: "res-1"}, nil
}

func (g *ocCountingGate) Finish(ctx context.Context, reservationID string, fact commercial.UsageFact) error {
	g.mu.Lock()
	g.finishes++
	g.mu.Unlock()
	return nil
}

// ocStubGuard is a switchable A02 stub: denial returns one of the T04
// sentinel denials so the handler mapping (CARRY T04-QF-1) is exercised on
// the real wrapped-error path.
type ocStubGuard struct {
	mu     sync.Mutex
	denial error
}

func (g *ocStubGuard) Check(ctx context.Context, subject appconnector.OCSubject, connectionID string, expectedVersion int64) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.denial
}

// ocTestEnv bundles the engine plus the knobs the matrix asserts on.
type ocTestEnv struct {
	engine     *gin.Engine
	db         *gorm.DB
	dispatcher *ocStubDispatcher
	gate       *ocCountingGate
	guard      *ocStubGuard
	apiKeys    *middleware.APIKeyRouteAuthorizer
}

const (
	ocTenantA   = uint64(101)
	ocTenantB   = uint64(202)
	ocUserA     = "user-a"
	ocUserB     = "user-b"
	ocSchemaOK  = `{"type":"object","properties":{"target":{"type":"string"},"q":{"type":"string"}},"required":["target"],"additionalProperties":false}`
	ocSchemaNoT = `{"type":"object","properties":{"q":{"type":"string"}},"additionalProperties":false}`
)

// ocMigrate sets up every table the OC product surface touches.
func ocMigrate(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(
		&appconnectorrepo.InstallationRow{}, &appconnectorrepo.ConnectionRow{},
		&appconnectorrepo.ActionRow{}, &appconnectorrepo.ApprovalRow{}, &appconnectorrepo.PreAuthorizationRow{},
		&appconnectorrepo.OCRuntimeRow{}, &appconnectorrepo.OCBindingRow{}, &appconnectorrepo.OCDefinitionRow{},
		&appconnectorrepo.OCAuthorizationAttemptRow{}, &appconnectorrepo.OCOperationsOutboxRow{},
		&appconnectorrepo.OCDispatchRecordRow{}, &appconnectorrepo.OCDispatchLeaseRow{},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE IF NOT EXISTS connector_provider_retry_state (provider TEXT PRIMARY KEY, retry_after DATETIME NOT NULL, observed_at DATETIME NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE IF NOT EXISTS connector_dispatch_lease_scopes (scope TEXT PRIMARY KEY, acquired_total INTEGER NOT NULL DEFAULT 0)").Error; err != nil {
		t.Fatal(err)
	}
}

// ocSeedTenantA seeds tenant 101 with the full happy-path chain plus the
// deliberately-unavailable variants the catalog must filter out, and an
// empty tenant 202 for isolation checks.
func ocSeedTenantA(t *testing.T, db *gorm.DB) {
	t.Helper()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.Create(&appconnectorrepo.OCRuntimeRow{ID: "rt-1", Enabled: true, Version: "1.5.0"}).Error)
	must(db.Create(&appconnectorrepo.InstallationRow{ID: "inst-gh", TenantID: ocTenantA, AppID: "github", AppVersion: "1.0.0", State: appconnector.InstallationActive, Version: 1}).Error)
	must(db.Create(&appconnectorrepo.ConnectionRow{TenantID: ocTenantA, ID: "conn-gh", InstallationID: "inst-gh", Kind: appconnector.ConnectionKindPersonal, OwnerID: ocUserA, State: appconnector.ConnectionActive, AuthVersion: 3}).Error)
	must(db.Create(&appconnectorrepo.OCBindingRow{TenantID: ocTenantA, ConnectionID: "conn-gh", RuntimeID: "rt-1", Provider: "github", ExternalID: "ext-gh-1", Alias: "alias-gh-1", AuthVersion: 3, BindingVersion: 1, State: "active"}).Error)
	// Published + scope-bearing + provider-matching: the ONLY visible action.
	must(db.Create(&appconnectorrepo.OCDefinitionRow{AppID: "github", AppVersion: "1.0.0", ActionID: "github.get_current_user", Provider: "github", InputSchema: ocSchemaOK, RequiredScopes: `["read:user"]`, Risk: "read", Published: true}).Error)
	// no_auth-shaped (empty scopes): default-excluded from the tenant catalog.
	must(db.Create(&appconnectorrepo.OCDefinitionRow{AppID: "github", AppVersion: "1.0.0", ActionID: "github.noauth_ping", Provider: "github", InputSchema: ocSchemaNoT, RequiredScopes: `[]`, Risk: "read", Published: true}).Error)
	// staged (unpublished) review: never tenant-visible.
	must(db.Create(&appconnectorrepo.OCDefinitionRow{AppID: "github", AppVersion: "1.0.0", ActionID: "github.staged_write", Provider: "github", InputSchema: ocSchemaOK, RequiredScopes: `["repo"]`, Risk: "write", Published: false}).Error)
	// foreign provider: fail-closed against this tenant's github binding.
	must(db.Create(&appconnectorrepo.OCDefinitionRow{AppID: "github", AppVersion: "1.0.0", ActionID: "slack.cross_post", Provider: "slack", InputSchema: ocSchemaOK, RequiredScopes: `["chat"]`, Risk: "send", Published: true}).Error)
	// targetless read schema (F-08 fallback: target = action id).
	must(db.Create(&appconnectorrepo.OCDefinitionRow{AppID: "github", AppVersion: "1.0.0", ActionID: "github.list_repos", Provider: "github", InputSchema: ocSchemaNoT, RequiredScopes: `["repo"]`, Risk: "read", Published: true}).Error)
	// Disabled installation chain: definition exists but is unreachable.
	must(db.Create(&appconnectorrepo.InstallationRow{ID: "inst-gl", TenantID: ocTenantA, AppID: "gitlab", AppVersion: "2.0.0", State: "disabled", Version: 1}).Error)
	must(db.Create(&appconnectorrepo.ConnectionRow{TenantID: ocTenantA, ID: "conn-gl", InstallationID: "inst-gl", Kind: appconnector.ConnectionKindSpace, OwnerID: "", State: appconnector.ConnectionActive, AuthVersion: 1}).Error)
	must(db.Create(&appconnectorrepo.OCBindingRow{TenantID: ocTenantA, ConnectionID: "conn-gl", RuntimeID: "rt-1", Provider: "gitlab", ExternalID: "ext-gl-1", Alias: "alias-gl-1", AuthVersion: 1, BindingVersion: 1, State: "active"}).Error)
	must(db.Create(&appconnectorrepo.OCDefinitionRow{AppID: "gitlab", AppVersion: "2.0.0", ActionID: "gitlab.read_issue", Provider: "gitlab", InputSchema: ocSchemaNoT, RequiredScopes: `["api"]`, Risk: "read", Published: true}).Error)
	// Revoked binding chain: unreachable.
	must(db.Create(&appconnectorrepo.InstallationRow{ID: "inst-as", TenantID: ocTenantA, AppID: "asana", AppVersion: "1.0.0", State: appconnector.InstallationActive, Version: 1}).Error)
	must(db.Create(&appconnectorrepo.ConnectionRow{TenantID: ocTenantA, ID: "conn-as", InstallationID: "inst-as", Kind: appconnector.ConnectionKindSpace, OwnerID: "", State: appconnector.ConnectionActive, AuthVersion: 1}).Error)
	must(db.Create(&appconnectorrepo.OCBindingRow{TenantID: ocTenantA, ConnectionID: "conn-as", RuntimeID: "rt-1", Provider: "asana", ExternalID: "ext-as-1", Alias: "alias-as-1", AuthVersion: 1, BindingVersion: 1, State: "revoked"}).Error)
	must(db.Create(&appconnectorrepo.OCDefinitionRow{AppID: "asana", AppVersion: "1.0.0", ActionID: "asana.list_tasks", Provider: "asana", InputSchema: ocSchemaNoT, RequiredScopes: `["read"]`, Risk: "read", Published: true}).Error)
}

// newOCProductEngine builds the full OC product surface over in-memory
// SQLite. opts tune the service wiring (nil dispatcher, A02 denial, ...).
type ocEngineOpt func(*ocEngineConfig)

type ocEngineConfig struct {
	withDispatcher bool
	withSlots      bool
	denial         error
	armPreparer    bool
}

func withOCDispatcher() ocEngineOpt { return func(c *ocEngineConfig) { c.withDispatcher = true } }

func withOCSlots() ocEngineOpt { return func(c *ocEngineConfig) { c.withSlots = true } }

func withA02Denial(err error) ocEngineOpt { return func(c *ocEngineConfig) { c.denial = err } }

func withoutOCPreparer() ocEngineOpt { return func(c *ocEngineConfig) { c.armPreparer = false } }

func newOCProductEngine(t *testing.T, opts ...ocEngineOpt) *ocTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := ocEngineConfig{armPreparer: true}
	for _, opt := range opts {
		opt(&cfg)
	}
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	ocMigrate(t, db)
	ocSeedTenantA(t, db)

	ocStore := appconnectorrepo.NewOCStore(db)
	installs := appconnectorrepo.NewInstallationStore(db)
	actionStore := appconnectorrepo.NewActionStore(db)

	// ActionService armed per the test matrix knobs.
	dispatcher := &ocStubDispatcher{}
	gate := &ocCountingGate{}
	guard := &ocStubGuard{denial: cfg.denial}
	var disp appconnectorsvc.ActionDispatcher
	if cfg.withDispatcher {
		disp = dispatcher
	}
	svc := appconnectorsvc.NewActionService(actionStore, guard, gate, disp, nil)
	if cfg.withSlots {
		slots, serr := appconnectorsvc.NewOCSlotLimiter(ocStore, appconnectorsvc.DefaultOCSlotLimits(), "test-owner")
		if serr != nil {
			t.Fatal(serr)
		}
		svc.UseOCSlotLimiter(slots)
		svc.UseOCDispatchClaims(ocStore)
	}

	// OC product services: trusted preparer + connection lifecycle.
	catalog := appconnectorsvc.NewOCCatalog(ocStore, ocStore, installs, installs)
	preparer := appconnectorsvc.NewOCPreparer(svc, catalog, ocStore)
	connSvc := appconnectorsvc.NewOCConnectionService(&ocStubCredentialSource{member: true}, installs, ocStore,
		appconnectorsvc.WithTransientCipher(mustTransientCipher(t)))

	installationHandler := NewAppInstallationHandler(db)
	connectionHandler := NewAppConnectionHandler(db)
	actionHandler := NewAppActionHandler(db)
	actionHandler.SetActionService(svc)
	if cfg.armPreparer {
		actionHandler.SetOCPreparer(preparer)
	}
	actionHandler.SetOCConnectionService(connSvc)

	// Real API-key gate with NO declarations: the exact posture the OC
	// product routes live in (default deny for X-API-Key principals).
	apiKeys := middleware.NewAPIKeyRouteAuthorizer()

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, ocTestDBKey{}, db)
		tenant := ocTenantA
		if c.GetHeader("X-Test-Tenant") == "B" {
			tenant = ocTenantB
		}
		role := string(types.TenantRoleAdmin)
		if c.GetHeader("X-Test-Role") != "" {
			role = c.GetHeader("X-Test-Role")
		}
		user := ocUserA
		if c.GetHeader("X-Test-User") != "" {
			user = c.GetHeader("X-Test-User")
		}
		ctx = context.WithValue(ctx, types.TenantIDContextKey, tenant)
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRole(role))
		ctx = context.WithValue(ctx, types.UserIDContextKey, user)
		if c.GetHeader("X-Test-API-Key") != "" {
			ctx = types.WithTenantAPIKeyScope(ctx, types.TenantAPIKeyScope{KeyID: 7, FullAccess: true})
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	engine.Use(apiKeys.Middleware())

	v1 := engine.Group("/api/v1")
	// Mirror of RegisterAppConnectorRoutes' OC product additions.
	v1.GET("/apps/catalog", installationHandler.ListOCCatalog)
	connections := v1.Group("/apps/connections", connectionHandler.RequireConnectionCapabilityForWrites())
	connections.GET("", connectionHandler.ListConnections)
	connections.POST("/:id/authorization-attempts", actionHandler.BeginOCAuthorization)
	v1.GET("/apps/authorization-attempts/:id", actionHandler.GetOCAuthorizationAttempt)
	v1.POST("/apps/oc/actions/prepare", actionHandler.RequireActionCapabilityForWrites(), actionHandler.PrepareOCAction)
	actions := v1.Group("/apps/actions", actionHandler.RequireActionCapabilityForWrites())
	actions.POST("/prepare", actionHandler.PrepareAction)
	actions.GET("/:id", actionHandler.GetAction)
	actions.POST("/:id/approve", actionHandler.ApproveAction)
	actions.POST("/:id/execute", actionHandler.ExecuteAction)

	return &ocTestEnv{engine: engine, db: db, dispatcher: dispatcher, gate: gate, guard: guard, apiKeys: apiKeys}
}

func mustTransientCipher(t *testing.T) appconnectorsvc.TransientCipher {
	t.Helper()
	c, err := appconnectorsvc.NewTransientCipherFromKey("test-transient-key")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func ocDo(t *testing.T, env *ocTestEnv, method, path, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	env.engine.ServeHTTP(w, req)
	return w
}

func ocJSONField(t *testing.T, body, field string) any {
	t.Helper()
	data := ocJSONData(t, body)
	return data[field]
}

// ocJSONData decodes the {success,data} envelope's data object.
func ocJSONData(t *testing.T, body string) map[string]any {
	t.Helper()
	var parsed struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("bad envelope: %v body=%s", err, body)
	}
	return parsed.Data
}

// ocActionDetail reads the nested action detail view (data.action.* plus
// data.expected_version).
func ocActionDetail(t *testing.T, body string) (id, digest, target, state string, version float64) {
	t.Helper()
	data := ocJSONData(t, body)
	action, _ := data["action"].(map[string]any)
	id, _ = action["id"].(string)
	digest, _ = action["digest"].(string)
	target, _ = action["target"].(string)
	state, _ = action["state"].(string)
	version, _ = data["expected_version"].(float64)
	return
}

// ---------------------------------------------------------------------------
// Plan-literal RED test (must exist verbatim) + decode matrix.
// ---------------------------------------------------------------------------

func TestOCPrepareRejectsRuntimeAndRisk(t *testing.T) {
	_, err := DecodeOCPrepare(strings.NewReader(`{"connection_id":"c","action_id":"a","input":{},"risk":"read","runtime_id":"evil"}`))
	if err == nil {
		t.Fatal("client supplied trusted fields")
	}
}

func TestDecodeOCPrepareAcceptsOnlyExactShape(t *testing.T) {
	in, err := DecodeOCPrepare(strings.NewReader(`{"connection_id":"c","action_id":"a","input":{"q":"x"}}`))
	if err != nil || in.ConnectionID != "c" || in.ActionID != "a" {
		t.Fatalf("valid decode rejected: %v", err)
	}
	// NOTE: "input":null decodes per the plan sketch (json.Valid passes);
	// the reviewed-schema object-root rule rejects it at prepare time.
	for name, raw := range map[string]string{
		"trailing":    `{"connection_id":"c","action_id":"a","input":{}} {"again":1}`,
		"bad input":   `{"connection_id":"c","action_id":"a","input":not-json}`,
		"no conn":     `{"action_id":"a","input":{}}`,
		"no action":   `{"connection_id":"c","input":{}}`,
		"alias field": `{"connection_id":"c","action_id":"a","input":{},"alias":"x"}`,
	} {
		if _, err := DecodeOCPrepare(strings.NewReader(raw)); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

// ---------------------------------------------------------------------------
// Catalog: A/B isolation + unavailable-action filtering + conventions.
// ---------------------------------------------------------------------------

func TestOCCatalogListsOnlyReachablePublishedActions(t *testing.T) {
	env := newOCProductEngine(t)
	w := ocDo(t, env, http.MethodGet, "/api/v1/apps/catalog", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var parsed struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, e := range parsed.Data {
		seen[e["action_id"].(string)] = true
		if e["published"] != true {
			t.Fatalf("unpublished definition leaked: %v", e)
		}
	}
	if !seen["github.get_current_user"] || !seen["github.list_repos"] {
		t.Fatalf("reachable actions missing: %v", seen)
	}
	for _, banned := range []string{"github.noauth_ping", "github.staged_write", "slack.cross_post", "gitlab.read_issue", "asana.list_tasks"} {
		if seen[banned] {
			t.Fatalf("unavailable action listed: %s", banned)
		}
	}
}

func TestOCCatalogTenantIsolation(t *testing.T) {
	env := newOCProductEngine(t)
	w := ocDo(t, env, http.MethodGet, "/api/v1/apps/catalog", "", "X-Test-Tenant", "B")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if strings.Contains(w.Body.String(), "github.get_current_user") {
		t.Fatal("tenant B saw tenant A actions")
	}
}

func TestOCCatalogReadableByMember(t *testing.T) {
	env := newOCProductEngine(t)
	w := ocDo(t, env, http.MethodGet, "/api/v1/apps/catalog", "", "X-Test-Role", "contributor")
	if w.Code != http.StatusOK {
		t.Fatalf("member catalog read rejected: %d body=%s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Authorization attempts.
// ---------------------------------------------------------------------------

func TestOCAuthorizationAttemptLifecycle(t *testing.T) {
	env := newOCProductEngine(t)
	w := ocDo(t, env, http.MethodPost, "/api/v1/apps/connections/conn-gh/authorization-attempts", `{}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("begin status=%d body=%s", w.Code, w.Body.String())
	}
	attemptID, _ := ocJSONField(t, w.Body.String(), "attempt_id").(string)
	if attemptID == "" {
		t.Fatal("no attempt_id")
	}
	body := w.Body.String()
	for _, leak := range []string{"state", "token", "secret", "credential"} {
		if strings.Contains(strings.ToLower(body), `"`+leak+`"`) {
			t.Fatalf("sensitive field echoed: %s in %s", leak, body)
		}
	}

	g := ocDo(t, env, http.MethodGet, "/api/v1/apps/authorization-attempts/"+attemptID, "")
	if g.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", g.Code, g.Body.String())
	}
	if status, _ := ocJSONField(t, g.Body.String(), "status").(string); status != "pending" {
		t.Fatalf("status=%v", status)
	}

	// Cross-tenant read: indistinguishable from missing (404).
	x := ocDo(t, env, http.MethodGet, "/api/v1/apps/authorization-attempts/"+attemptID, "", "X-Test-Tenant", "B")
	if x.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant get status=%d", x.Code)
	}
	// Foreign actor: also 404 (tenant+actor gate).
	y := ocDo(t, env, http.MethodGet, "/api/v1/apps/authorization-attempts/"+attemptID, "", "X-Test-User", ocUserB)
	if y.Code != http.StatusNotFound {
		t.Fatalf("foreign actor get status=%d", y.Code)
	}
}

func TestOCAuthorizationAttemptAPIKeyHandoff(t *testing.T) {
	env := newOCProductEngine(t)
	w := ocDo(t, env, http.MethodPost, "/api/v1/apps/connections/conn-gh/authorization-attempts",
		`{"credential":"ghp-live-secret"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("api-key begin status=%d body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "ghp-live-secret") {
		t.Fatal("credential material echoed")
	}
	// The credential travels ONLY inside the encrypted handoff: the outbox
	// row's resource id is attempt|ciphertext, never the plaintext.
	var row appconnectorrepo.OCOperationsOutboxRow
	if err := env.db.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(row.ResourceID, "ghp-live-secret") {
		t.Fatal("plaintext credential persisted in outbox")
	}
	if !strings.Contains(row.ResourceID, "v1.") {
		t.Fatalf("outbox resource not a sealed handoff: %q", row.ResourceID)
	}
}

func TestOCAuthorizationAttemptGatesAndErrors(t *testing.T) {
	env := newOCProductEngine(t)
	// Member role cannot begin attempts (write gate not widened).
	if w := ocDo(t, env, http.MethodPost, "/api/v1/apps/connections/conn-gh/authorization-attempts", `{}`, "X-Test-Role", "contributor"); w.Code != http.StatusForbidden {
		t.Fatalf("member begin status=%d", w.Code)
	}
	// Personal connection of another owner: owner role is not enough.
	if w := ocDo(t, env, http.MethodPost, "/api/v1/apps/connections/conn-gh/authorization-attempts", `{}`, "X-Test-User", ocUserB); w.Code != http.StatusForbidden {
		t.Fatalf("non-owner personal begin status=%d", w.Code)
	}
	// Missing / cross-tenant connection: 404.
	if w := ocDo(t, env, http.MethodPost, "/api/v1/apps/connections/nope/authorization-attempts", `{}`); w.Code != http.StatusNotFound {
		t.Fatalf("unknown connection status=%d", w.Code)
	}
	if w := ocDo(t, env, http.MethodPost, "/api/v1/apps/connections/conn-gh/authorization-attempts", `{}`, "X-Test-Tenant", "B"); w.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant connection status=%d", w.Code)
	}
	// Unknown attempt: 404.
	if w := ocDo(t, env, http.MethodGet, "/api/v1/apps/authorization-attempts/none", ""); w.Code != http.StatusNotFound {
		t.Fatalf("unknown attempt status=%d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// OC prepare endpoint.
// ---------------------------------------------------------------------------

func ocPrepareBody(action string) string {
	if action == "" {
		return `{"connection_id":"conn-gh","action_id":"github.get_current_user","input":{"target":"octocat","q":"hi"}}`
	}
	return `{"connection_id":"conn-gh","action_id":"` + action + `","input":{"q":"hi"}}`
}

// ---------------------------------------------------------------------------
// R18 narrow write-set extension (T15-C-1 / T15-C-2): the tenant-facing DTOs
// surface the fields the UI needs — the live auth_version a revoke CAS must
// echo, and the frozen risk the approval template displays.
// ---------------------------------------------------------------------------

func TestOCViewsSurfaceAuthVersionAndRisk(t *testing.T) {
	env := newOCProductEngine(t)

	// conn-gh is seeded at AuthVersion 3 (a distinctive, non-default
	// generation): the list view must pass the PERSISTED value through,
	// proving it is read from the row rather than defaulted.
	w := ocDo(t, env, http.MethodGet, "/api/v1/apps/connections", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", w.Code, w.Body.String())
	}
	var listResp struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("bad list body: %v", err)
	}
	byID := map[string]map[string]any{}
	for _, row := range listResp.Data {
		if id, _ := row["id"].(string); id != "" {
			byID[id] = row
		}
	}
	if row, ok := byID["conn-gh"]; !ok {
		t.Fatal("conn-gh missing from connection list")
	} else if v, _ := row["auth_version"].(float64); v != 3 {
		t.Fatalf("conn-gh auth_version=%v want 3 (persisted pass-through)", row["auth_version"])
	}
	if row, ok := byID["conn-gl"]; !ok {
		t.Fatal("conn-gl missing from connection list")
	} else if v, _ := row["auth_version"].(float64); v != 1 {
		t.Fatalf("conn-gl auth_version=%v want 1", row["auth_version"])
	}

	// The frozen risk recorded at prepare time (github.get_current_user is
	// reviewed with risk=read) must appear on the action detail view.
	pw := ocDo(t, env, http.MethodPost, "/api/v1/apps/oc/actions/prepare", ocPrepareBody(""))
	if pw.Code != http.StatusCreated {
		t.Fatalf("prepare status=%d body=%s", pw.Code, pw.Body.String())
	}
	id, _, _, _, _ := ocActionDetail(t, pw.Body.String())
	gw := ocDo(t, env, http.MethodGet, "/api/v1/apps/actions/"+id, "")
	if gw.Code != http.StatusOK {
		t.Fatalf("get action status=%d body=%s", gw.Code, gw.Body.String())
	}
	data := ocJSONData(t, gw.Body.String())
	action, _ := data["action"].(map[string]any)
	if action == nil {
		t.Fatal("action detail has no action object")
	}
	if r, _ := action["risk"].(string); r != "read" {
		t.Fatalf("action risk=%v want read (frozen at prepare)", action["risk"])
	}
}

func TestOCPrepareEndpointHappyPath(t *testing.T) {
	env := newOCProductEngine(t)
	w := ocDo(t, env, http.MethodPost, "/api/v1/apps/oc/actions/prepare", ocPrepareBody(""))
	if w.Code != http.StatusCreated {
		t.Fatalf("prepare status=%d body=%s", w.Code, w.Body.String())
	}
	_, digest, target, state, _ := ocActionDetail(t, w.Body.String())
	if digest == "" {
		t.Fatal("no digest")
	}
	// F-08: target derived server-side from the input's target field.
	if target != "octocat" {
		t.Fatalf("derived target=%v", target)
	}
	if state != appconnector.ActionAwaitingApproval {
		t.Fatalf("state=%v", state)
	}
	// The row carries the server-derived binding, never client input.
	var row appconnectorrepo.ActionRow
	if err := env.db.Where("tenant_id = ?", ocTenantA).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.OCBindingJSON == "" || row.DigestVersion != appconnector.CurrentDigestVersion {
		t.Fatalf("binding=%q digest_version=%d", row.OCBindingJSON, row.DigestVersion)
	}
}

func TestOCPrepareEndpointTargetlessFallback(t *testing.T) {
	env := newOCProductEngine(t)
	w := ocDo(t, env, http.MethodPost, "/api/v1/apps/oc/actions/prepare", ocPrepareBody("github.list_repos"))
	if w.Code != http.StatusCreated {
		t.Fatalf("prepare status=%d body=%s", w.Code, w.Body.String())
	}
	_, _, target, _, _ := ocActionDetail(t, w.Body.String())
	if target != "github.list_repos" {
		t.Fatalf("targetless fallback target=%v", target)
	}
}

func TestOCPrepareEndpointRejections(t *testing.T) {
	env := newOCProductEngine(t)
	// Client-forged trusted fields (the endpoint-level twin of the literal
	// decode test): 400, never a silent ignore.
	if w := ocDo(t, env, http.MethodPost, "/api/v1/apps/oc/actions/prepare",
		`{"connection_id":"conn-gh","action_id":"a","input":{},"risk":"read","runtime_id":"evil"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("forged fields status=%d", w.Code)
	}
	// Cross-tenant / unknown connection: 404 (indistinguishable).
	if w := ocDo(t, env, http.MethodPost, "/api/v1/apps/oc/actions/prepare", ocPrepareBody(""), "X-Test-Tenant", "B"); w.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant prepare status=%d body=%s", w.Code, w.Body.String())
	}
	// Unreachable action ids: 404 (unpublished / no_auth / foreign provider).
	for _, action := range []string{"github.staged_write", "github.noauth_ping", "slack.cross_post", "gitlab.read_issue", "github.unknown"} {
		if w := ocDo(t, env, http.MethodPost, "/api/v1/apps/oc/actions/prepare", ocPrepareBody(action)); w.Code != http.StatusNotFound {
			t.Fatalf("unreachable %s status=%d body=%s", action, w.Code, w.Body.String())
		}
	}
	// Schema-rejected input: 400.
	if w := ocDo(t, env, http.MethodPost, "/api/v1/apps/oc/actions/prepare",
		`{"connection_id":"conn-gh","action_id":"github.get_current_user","input":{"missing_target":true}}`); w.Code != http.StatusBadRequest {
		t.Fatalf("schema-rejected status=%d body=%s", w.Code, w.Body.String())
	}
	// Member role: 403 (existing action write gate, not widened).
	if w := ocDo(t, env, http.MethodPost, "/api/v1/apps/oc/actions/prepare", ocPrepareBody(""), "X-Test-Role", "contributor"); w.Code != http.StatusForbidden {
		t.Fatalf("member prepare status=%d", w.Code)
	}
	// Over-1MiB body: rejected by the byte cap.
	big := `{"connection_id":"conn-gh","action_id":"github.get_current_user","input":{"target":"` + strings.Repeat("x", 1<<20) + `"}}`
	if w := ocDo(t, env, http.MethodPost, "/api/v1/apps/oc/actions/prepare", big); w.Code != http.StatusBadRequest {
		t.Fatalf("oversized status=%d", w.Code)
	}
}

func TestOCPrepareUnwiredFailsClosed(t *testing.T) {
	env := newOCProductEngine(t, withoutOCPreparer())
	w := ocDo(t, env, http.MethodPost, "/api/v1/apps/oc/actions/prepare", ocPrepareBody(""))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("unwired preparer status=%d body=%s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Execute/approve error mapping (CARRY T04-QF-1 + T09-MINOR-1 + 429 + 503).
// ---------------------------------------------------------------------------

// ocPrepareAndApprove drives the trusted OC path to an authorized action.
func ocPrepareAndApprove(t *testing.T, env *ocTestEnv) string {
	t.Helper()
	w := ocDo(t, env, http.MethodPost, "/api/v1/apps/oc/actions/prepare", ocPrepareBody(""))
	if w.Code != http.StatusCreated {
		t.Fatalf("prepare status=%d body=%s", w.Code, w.Body.String())
	}
	id, digest, _, _, version := ocActionDetail(t, w.Body.String())
	a := ocDo(t, env, http.MethodPost, "/api/v1/apps/actions/"+id+"/approve",
		`{"digest":"`+digest+`","expected_version":`+strconv.FormatInt(int64(version), 10)+`}`)
	if a.Code != http.StatusOK {
		t.Fatalf("approve status=%d body=%s", a.Code, a.Body.String())
	}
	return id
}

func TestOCExecuteUnconfiguredMaps503AndConsumesNothing(t *testing.T) {
	env := newOCProductEngine(t) // no dispatcher wired
	id := ocPrepareAndApprove(t, env)
	w := ocDo(t, env, http.MethodPost, "/api/v1/apps/actions/"+id+"/execute", `{}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("execute status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "OC_DISPATCH_NOT_CONFIGURED") {
		t.Fatalf("code missing: %s", w.Body.String())
	}
	// Unconfigured = zero side effects: no budget Begin, no outbound HTTP,
	// approval remaining intact, row still authorized.
	if env.gate.begins != 0 {
		t.Fatalf("budget begins=%d", env.gate.begins)
	}
	if env.dispatcher.callsCount() != 0 {
		t.Fatalf("http calls=%d", env.dispatcher.callsCount())
	}
	var row appconnectorrepo.ActionRow
	if err := env.db.Where("id = ?", id).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != appconnector.ActionAuthorized {
		t.Fatalf("state=%s", row.State)
	}
	var approval appconnectorrepo.ApprovalRow
	if err := env.db.Where("action_id = ?", id).First(&approval).Error; err != nil {
		t.Fatal(err)
	}
	if approval.Remaining != 1 {
		t.Fatalf("approval consumed: remaining=%d", approval.Remaining)
	}
}

func TestOCExecuteA02DenialsMap4xxWithExplicitCodes(t *testing.T) {
	// T04-QF-1: A02 denials surface as explicit 4xx codes, never a generic
	// 500 ACTION_EXECUTE_FAILED.
	for _, tc := range []struct {
		name   string
		denial error
		status int
		code   string
	}{
		{"forbidden", appconnectorsvc.ErrConnectionForbidden, http.StatusForbidden, "CONNECTION_FORBIDDEN"},
		{"not member", appconnectorsvc.ErrSubjectNotMember, http.StatusForbidden, "SUBJECT_NOT_MEMBER"},
		{"version stale", appconnectorsvc.ErrConnectionVersionStale, http.StatusConflict, "CONNECTION_VERSION_STALE"},
		{"revoked", appconnectorsvc.ErrConnectionRevoked, http.StatusConflict, "CONNECTION_REVOKED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newOCProductEngine(t, withOCDispatcher(), withA02Denial(tc.denial))
			id := ocPrepareAndApprove(t, env)
			w := ocDo(t, env, http.MethodPost, "/api/v1/apps/actions/"+id+"/execute", `{}`)
			if w.Code != tc.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.code) {
				t.Fatalf("explicit code %s missing: %s", tc.code, w.Body.String())
			}
		})
	}
}

func TestOCExecuteRePrepareRequiredMaps409(t *testing.T) {
	// T09-MINOR-1: a legacy-generation (v1) row must re-Prepare — both
	// execute and approve refuse with an explicit 409, never a 500.
	env := newOCProductEngine(t, withOCDispatcher())
	id := ocPrepareAndApprove(t, env)
	if err := env.db.Model(&appconnectorrepo.ActionRow{}).Where("id = ?", id).
		Update("digest_version", 1).Error; err != nil {
		t.Fatal(err)
	}
	if w := ocDo(t, env, http.MethodPost, "/api/v1/apps/actions/"+id+"/execute", `{}`); w.Code != http.StatusConflict {
		t.Fatalf("execute status=%d body=%s", w.Code, w.Body.String())
	}
	var row appconnectorrepo.ActionRow
	if err := env.db.Where("id = ?", id).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	a := ocDo(t, env, http.MethodPost, "/api/v1/apps/actions/"+id+"/approve",
		`{"digest":"`+row.ArgsDigest+`","expected_version":`+strconv.FormatInt(row.Fence, 10)+`}`)
	if a.Code != http.StatusConflict || !strings.Contains(a.Body.String(), "ACTION_REPREPARE_REQUIRED") {
		t.Fatalf("approve status=%d body=%s", a.Code, a.Body.String())
	}
}

func TestOCExecuteProviderThrottledMaps429(t *testing.T) {
	env := newOCProductEngine(t, withOCDispatcher(), withOCSlots())
	id := ocPrepareAndApprove(t, env)
	// Record a live provider cooldown: dispatches fail closed with 429 and
	// consume nothing (queue-limit family).
	now := time.Now().UTC()
	if err := appconnectorrepo.NewOCStore(env.db).NoteOCProviderRetryAfter(context.Background(), "github", now.Add(2*time.Minute), now); err != nil && !strings.Contains(err.Error(), "conflict") {
		t.Fatal(err)
	}
	w := ocDo(t, env, http.MethodPost, "/api/v1/apps/actions/"+id+"/execute", `{}`)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("execute status=%d body=%s", w.Code, w.Body.String())
	}
	if env.dispatcher.callsCount() != 0 || env.gate.begins != 0 {
		t.Fatalf("throttled dispatch consumed resources: http=%d budget=%d", env.dispatcher.callsCount(), env.gate.begins)
	}
	var row appconnectorrepo.ActionRow
	if err := env.db.Where("id = ?", id).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != appconnector.ActionAuthorized {
		t.Fatalf("state=%s", row.State)
	}
}

func TestOCExecuteHappyPathDispatchesOnce(t *testing.T) {
	env := newOCProductEngine(t, withOCDispatcher(), withOCSlots())
	id := ocPrepareAndApprove(t, env)
	w := ocDo(t, env, http.MethodPost, "/api/v1/apps/actions/"+id+"/execute", `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("execute status=%d body=%s", w.Code, w.Body.String())
	}
	if env.dispatcher.callsCount() != 1 {
		t.Fatalf("http calls=%d", env.dispatcher.callsCount())
	}
}

func TestOCCrossTenantActionInvisible(t *testing.T) {
	// Ruling 9 A/B matrix on approve/execute/get: tenant B never sees
	// tenant A's action (404, indistinguishable from missing).
	env := newOCProductEngine(t, withOCDispatcher())
	id := ocPrepareAndApprove(t, env)
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/apps/actions/" + id, ""},
		{http.MethodPost, "/api/v1/apps/actions/" + id + "/approve", `{"digest":"x","expected_version":1}`},
		{http.MethodPost, "/api/v1/apps/actions/" + id + "/execute", `{}`},
	} {
		if w := ocDo(t, env, tc.method, tc.path, tc.body, "X-Test-Tenant", "B"); w.Code != http.StatusNotFound {
			t.Fatalf("%s %s status=%d", tc.method, tc.path, w.Code)
		}
	}
}

// ---------------------------------------------------------------------------
// API-key default deny on the new routes (real gate, undeclared routes).
// ---------------------------------------------------------------------------

func TestOCProductRoutesDefaultDenyAPIKeys(t *testing.T) {
	env := newOCProductEngine(t)
	// A FULL-ACCESS API key is still denied: the OC product routes declare
	// no API-key policy, and the gate default-denies undeclared routes.
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/apps/catalog"},
		{http.MethodPost, "/api/v1/apps/connections/conn-gh/authorization-attempts"},
		{http.MethodGet, "/api/v1/apps/authorization-attempts/any"},
		{http.MethodPost, "/api/v1/apps/oc/actions/prepare"},
	} {
		body := `{}`
		w := ocDo(t, env, tc.method, tc.path, body, "X-Test-API-Key", "wk-1")
		if w.Code != http.StatusForbidden {
			t.Fatalf("api key %s %s status=%d", tc.method, tc.path, w.Code)
		}
	}
	// The same routes answer an authenticated principal (the gate passes
	// non-API-key contexts through untouched).
	if w := ocDo(t, env, http.MethodGet, "/api/v1/apps/catalog", ""); w.Code != http.StatusOK {
		t.Fatalf("bearer principal status=%d", w.Code)
	}
}
