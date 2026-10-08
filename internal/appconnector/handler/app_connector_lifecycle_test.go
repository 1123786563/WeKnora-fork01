package handler

// Pass B (27-appconnector) T1 characterization baseline: these tests anchor
// the installation / connection / sync / action HTTP behavior of the
// /api/v1/apps surface in the HOST package BEFORE the handler files move to
// internal/appconnector/handler. After the move the same file is
// relocated verbatim (same package name) and must pass unchanged — any
// difference is a migration regression, not an intended behavior change.
//
// Every assertion below is derived from the current implementation:
//   - app_connector_installation.go (view fields, MEMBER_MUST_REQUEST_INSTALLATION
//     write gate, by-tenant 404 lookups)
//   - app_connector_connection.go (CreateConnection branch table 400/404/409/
//     501/400/201)
//   - app_connector_sync.go (DATASOURCE_NOT_FOUND, no-binding legacy shape,
//     binding view + pause_reason=permission on reauthorization)
//   - app_connector_action.go:164-168 (unwired A03 pipeline fails closed 501)

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcprepo "github.com/Tencent/WeKnora/internal/application/repository"
	appconnector "github.com/Tencent/WeKnora/internal/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	aclTenantA = uint64(101)
	aclTenantB = uint64(202)
	aclUser    = "acl-user-1"
)

// aclOpenDB opens the per-test shared-cache in-memory SQLite (same recipe
// as the existing OC tests) so each test function owns its database.
func aclOpenDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	return db
}

// aclMigrate creates every table the four lifecycle surfaces touch. The
// sync-status projection reads the two legacy bare tables directly
// (app_connector_sync.go:50-67); StoredSyncBinding carries no TableName pin,
// so app_datasource_bindings is created with its migration-shaped DDL.
func aclMigrate(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(
		&appconnectorrepo.InstallationRow{}, &appconnectorrepo.ConnectionRow{},
		&appconnectorrepo.AppVersion{}, &mcprepo.MCPOAuthBindingRow{},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE IF NOT EXISTS data_sources (id TEXT PRIMARY KEY, status TEXT NOT NULL, tenant_id INTEGER NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE IF NOT EXISTS app_datasource_bindings (tenant_id INTEGER NOT NULL, datasource_id TEXT NOT NULL, installation_id TEXT NOT NULL, connection_id TEXT NOT NULL, auth_version INTEGER NOT NULL, PRIMARY KEY (tenant_id, datasource_id))").Error; err != nil {
		t.Fatal(err)
	}
}

// aclEnv bundles the engine and the raw handle the seeds assert against.
type aclEnv struct {
	engine *gin.Engine
	db     *gorm.DB
}

// aclNewEngine builds the four single-lifecycle handlers over in-memory
// SQLite and registers the same-shaped subset of RegisterAppConnectorRoutes
// (routes_app_connectors.go) these characterizations exercise. The notion
// OAuth client is registered; feishu stays a known-but-unconfigured provider
// and github is an app with no OAuth flow at all. The A03 action service is
// deliberately NOT wired (SetActionService never called).
func aclNewEngine(t *testing.T) *aclEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := aclOpenDB(t)
	aclMigrate(t, db)

	installationHandler := NewAppInstallationHandler(db)
	connectionHandler := NewAppConnectionHandler(db)
	syncHandler := NewAppSyncHandler(db)
	actionHandler := NewAppActionHandler(db)

	cfg := DefaultAppOAuthProviderConfigs()
	notion := cfg["notion"]
	notion.ClientID = "cid"
	notion.ClientSecret = "csec"
	cfg["notion"] = notion
	connectionHandler.SetAppOAuthProviders(cfg)

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		tenant := aclTenantA
		if c.GetHeader("X-Acl-Tenant") == "B" {
			tenant = aclTenantB
		}
		role := types.TenantRoleOwner
		if r := c.GetHeader("X-Acl-Role"); r != "" {
			role = types.TenantRole(r)
		}
		ctx = context.WithValue(ctx, types.TenantIDContextKey, tenant)
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
		ctx = context.WithValue(ctx, types.UserIDContextKey, aclUser)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})

	v1 := engine.Group("/api/v1")
	installations := v1.Group("/apps/installations", installationHandler.RequireInstallCapabilityForWrites())
	{
		installations.GET("", installationHandler.ListInstallations)
		installations.POST("", installationHandler.CreateInstallation)
		installations.POST("/:id/upgrade", installationHandler.UpgradeInstallation)
		installations.POST("/:id/disable", installationHandler.DisableInstallation)
	}
	connections := v1.Group("/apps/connections", connectionHandler.RequireConnectionCapabilityForWrites())
	{
		connections.POST("", connectionHandler.CreateConnection)
	}
	v1.GET("/apps/datasources/:id/sync-status", syncHandler.GetSyncStatus)
	actions := v1.Group("/apps/actions", actionHandler.RequireActionCapabilityForWrites())
	{
		actions.POST("/prepare", actionHandler.PrepareAction)
	}
	return &aclEnv{engine: engine, db: db}
}

func aclDo(t *testing.T, env *aclEnv, method, path, body string, headers ...string) *httptest.ResponseRecorder {
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

// aclData decodes the {success,data} envelope's data object.
func aclData(t *testing.T, body string) map[string]any {
	t.Helper()
	var parsed struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("bad envelope: %v body=%s", err, body)
	}
	return parsed.Data
}

// aclError extracts the {success,error:{code,message}} failure envelope.
func aclError(t *testing.T, body string) (code, message string) {
	t.Helper()
	var parsed struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("bad envelope: %v body=%s", err, body)
	}
	return parsed.Error.Code, parsed.Error.Message
}

func aclRequireError(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status=%d want=%d body=%s", w.Code, status, w.Body.String())
	}
	got, _ := aclError(t, w.Body.String())
	if got != code {
		t.Fatalf("error code=%q want=%q body=%s", got, code, w.Body.String())
	}
}

// TestAppInstallationLifecycleAndWriteGate anchors the W04 installation
// lifecycle: owner create -> list -> upgrade -> disable all succeed with the
// documented view shape, member writes are refused with
// MEMBER_MUST_REQUEST_INSTALLATION, and a cross-tenant id is
// indistinguishable from a missing one (both 404).
func TestAppInstallationLifecycleAndWriteGate(t *testing.T) {
	env := aclNewEngine(t)
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	// Published catalog entries with IDENTICAL requested scopes so the
	// 1.0.0 -> 2.0.0 upgrade is not scope-expanding (a scope expansion would
	// refuse to land active and demand re-authorization instead).
	must(env.db.Create(&appconnectorrepo.AppVersion{AppID: "notion", Version: "1.0.0", SchemaJSON: `{"scopes":["read:user"]}`}).Error)
	must(env.db.Create(&appconnectorrepo.AppVersion{AppID: "notion", Version: "2.0.0", SchemaJSON: `{"scopes":["read:user"]}`}).Error)

	// Member role cannot install: the request-an-installation path, never a
	// fake success (app_connector_installation.go:29-33).
	w := aclDo(t, env, http.MethodPost, "/api/v1/apps/installations",
		`{"app_key":"notion","version":"1.0.0","expected_version":0}`, "X-Acl-Role", "member")
	aclRequireError(t, w, http.StatusForbidden, "MEMBER_MUST_REQUEST_INSTALLATION")

	// Owner creates the installation: 201 with the full view
	// (id/app_key/version/state/scopes, scopes from the catalog schema).
	w = aclDo(t, env, http.MethodPost, "/api/v1/apps/installations",
		`{"app_key":"notion","version":"1.0.0","expected_version":0}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}
	data := aclData(t, w.Body.String())
	instID, _ := data["id"].(string)
	if instID == "" {
		t.Fatalf("no installation id: %s", w.Body.String())
	}
	if data["app_key"] != "notion" || data["version"] != "1.0.0" || data["state"] != "active" {
		t.Fatalf("create view mismatch: %v", data)
	}
	scopes, _ := data["scopes"].([]any)
	if len(scopes) != 1 || scopes[0] != "read:user" {
		t.Fatalf("scopes=%v want [read:user]", scopes)
	}

	// List shows exactly the one installation.
	w = aclDo(t, env, http.MethodGet, "/api/v1/apps/installations", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", w.Code, w.Body.String())
	}
	var list struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Data) != 1 {
		t.Fatalf("list len=%d body=%s", len(list.Data), w.Body.String())
	}
	if list.Data[0]["id"] != instID || list.Data[0]["state"] != "active" {
		t.Fatalf("list row mismatch: %v", list.Data[0])
	}

	// Upgrade 1.0.0 -> 2.0.0 under the current CAS version: 200, stays active.
	w = aclDo(t, env, http.MethodPost, "/api/v1/apps/installations/"+instID+"/upgrade",
		`{"version":"2.0.0","expected_version":1}`)
	if w.Code != http.StatusOK {
		t.Fatalf("upgrade status=%d body=%s", w.Code, w.Body.String())
	}
	data = aclData(t, w.Body.String())
	if data["version"] != "2.0.0" || data["state"] != "active" {
		t.Fatalf("upgrade view mismatch: %v", data)
	}

	// Disable under the post-upgrade CAS version: 200, state disabled.
	w = aclDo(t, env, http.MethodPost, "/api/v1/apps/installations/"+instID+"/disable",
		`{"expected_version":2}`)
	if w.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", w.Code, w.Body.String())
	}
	if data = aclData(t, w.Body.String()); data["state"] != "disabled" {
		t.Fatalf("disable view mismatch: %v", data)
	}

	// Cross-tenant id and missing id are BOTH 404 INSTALLATION_NOT_FOUND —
	// never a 403 that leaks existence (app_connector.go:20-26 invariant,
	// installation.go:98 by-tenant lookup).
	x := aclDo(t, env, http.MethodPost, "/api/v1/apps/installations/"+instID+"/upgrade",
		`{"version":"2.0.0","expected_version":3}`, "X-Acl-Tenant", "B")
	aclRequireError(t, x, http.StatusNotFound, "INSTALLATION_NOT_FOUND")
	n := aclDo(t, env, http.MethodPost, "/api/v1/apps/installations/inst-none/upgrade",
		`{"version":"2.0.0","expected_version":3}`)
	aclRequireError(t, n, http.StatusNotFound, "INSTALLATION_NOT_FOUND")
}

// TestAppConnectionCreateBranches anchors the A02 CreateConnection branch
// table: 400 invalid/missing fields, 404 unknown installation, 409 stale
// expected_version, 501 known-but-unconfigured provider, 400 unknown app,
// and the 201 authorize-start payload.
func TestAppConnectionCreateBranches(t *testing.T) {
	env := aclNewEngine(t)
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(env.db.Create(&appconnectorrepo.InstallationRow{ID: "inst-notion", TenantID: aclTenantA, AppID: "notion", AppVersion: "1.0.0", State: appconnector.InstallationActive, Version: 4}).Error)
	must(env.db.Create(&appconnectorrepo.InstallationRow{ID: "inst-feishu", TenantID: aclTenantA, AppID: "feishu", AppVersion: "1.0.0", State: appconnector.InstallationActive, Version: 1}).Error)
	// ponytail: main 侧已支持 github/gitlab OAuth，github 不再是未知应用；400 分支用真正未注册的 app
	must(env.db.Create(&appconnectorrepo.InstallationRow{ID: "inst-unknown", TenantID: aclTenantA, AppID: "acme-unknown", AppVersion: "1.0.0", State: appconnector.InstallationActive, Version: 1}).Error)

	// Missing required fields: 400 INVALID_REQUEST (connection.go:118-127).
	aclRequireError(t, aclDo(t, env, http.MethodPost, "/api/v1/apps/connections", `{}`),
		http.StatusBadRequest, "INVALID_REQUEST")
	aclRequireError(t, aclDo(t, env, http.MethodPost, "/api/v1/apps/connections",
		`{"installation_id":"inst-notion","kind":"personal","expected_version":4}`),
		http.StatusBadRequest, "INVALID_REQUEST")

	// Unknown installation in this tenant: 404 INSTALLATION_NOT_FOUND (:132).
	aclRequireError(t, aclDo(t, env, http.MethodPost, "/api/v1/apps/connections",
		`{"installation_id":"inst-none","kind":"personal","expected_version":1,"redirect_uri":"https://cb"}`),
		http.StatusNotFound, "INSTALLATION_NOT_FOUND")

	// Stale expected_version: 409 VERSION_CONFLICT (:140).
	aclRequireError(t, aclDo(t, env, http.MethodPost, "/api/v1/apps/connections",
		`{"installation_id":"inst-notion","kind":"personal","expected_version":3,"redirect_uri":"https://cb"}`),
		http.StatusConflict, "VERSION_CONFLICT")

	// Known app whose provider application is not registered: 501
	// OAUTH_NOT_CONFIGURED, fail closed (:148).
	aclRequireError(t, aclDo(t, env, http.MethodPost, "/api/v1/apps/connections",
		`{"installation_id":"inst-feishu","kind":"space","expected_version":1,"redirect_uri":"https://cb"}`),
		http.StatusNotImplemented, "OAUTH_NOT_CONFIGURED")

	// App with no OAuth flow at all: 400 UNKNOWN_APP (:151).
	aclRequireError(t, aclDo(t, env, http.MethodPost, "/api/v1/apps/connections",
		`{"installation_id":"inst-unknown","kind":"space","expected_version":1,"redirect_uri":"https://cb"}`),
		http.StatusBadRequest, "UNKNOWN_APP")

	// Success: 201 authorize-start payload with the five documented fields
	// (:158-164). The state is minted server-side; the authorize URL points
	// at the notion provider with the redirect and state attached.
	w := aclDo(t, env, http.MethodPost, "/api/v1/apps/connections",
		`{"installation_id":"inst-notion","kind":"personal","expected_version":4,"redirect_uri":"https://cb"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create connection status=%d body=%s", w.Code, w.Body.String())
	}
	data := aclData(t, w.Body.String())
	if state, _ := data["authorization_state"].(string); state == "" {
		t.Fatalf("no authorization_state: %s", w.Body.String())
	}
	if authorizeURL, _ := data["authorize_url"].(string); !strings.Contains(authorizeURL, "api.notion.com") {
		t.Fatalf("authorize_url=%v", data["authorize_url"])
	}
	if data["installation_id"] != "inst-notion" || data["kind"] != "personal" {
		t.Fatalf("payload mismatch: %v", data)
	}
	if _, ok := data["expires_at"].(string); !ok {
		t.Fatalf("expires_at missing: %v", data)
	}
}

// TestAppSyncStatusThreeStates anchors the A07 sync-status projection:
// missing/cross-tenant data source 404, legacy no-binding shape
// (binding=null, requires_reauthorization=false, state=<ds.status>), and the
// bound shape whose view carries the binding ids plus pause_reason=permission
// exactly when reauthorization is required.
func TestAppSyncStatusThreeStates(t *testing.T) {
	env := aclNewEngine(t)
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, ds := range []struct{ id, status string }{
		{"ds-a", "completed"}, {"ds-b", "completed"}, {"ds-c", "completed"},
	} {
		must(env.db.Exec("INSERT INTO data_sources (id, status, tenant_id) VALUES (?, ?, ?)",
			ds.id, ds.status, aclTenantA).Error)
	}
	// ds-b binds an active space connection on an active installation: the
	// only live chain that authorizes team sync.
	must(env.db.Create(&appconnectorrepo.InstallationRow{ID: "inst-s", TenantID: aclTenantA, AppID: "notion", AppVersion: "1.0.0", State: appconnector.InstallationActive, Version: 1}).Error)
	must(env.db.Create(&appconnectorrepo.ConnectionRow{TenantID: aclTenantA, ID: "conn-space", InstallationID: "inst-s", Kind: appconnector.ConnectionKindSpace, State: appconnector.ConnectionActive, AuthVersion: 7}).Error)
	must(env.db.Exec("INSERT INTO app_datasource_bindings (tenant_id, datasource_id, installation_id, connection_id, auth_version) VALUES (?, 'ds-b', 'inst-s', 'conn-space', 5)", aclTenantA).Error)
	// ds-c binds a personal connection: the binding resolves for bookkeeping
	// but requires re-authorization (team sync never runs through personal
	// credentials).
	must(env.db.Create(&appconnectorrepo.ConnectionRow{TenantID: aclTenantA, ID: "conn-pers", InstallationID: "inst-s", Kind: appconnector.ConnectionKindPersonal, OwnerID: aclUser, State: appconnector.ConnectionActive, AuthVersion: 2}).Error)
	must(env.db.Exec("INSERT INTO app_datasource_bindings (tenant_id, datasource_id, installation_id, connection_id, auth_version) VALUES (?, 'ds-c', 'inst-s', 'conn-pers', 5)", aclTenantA).Error)

	// Missing data source: 404 DATASOURCE_NOT_FOUND (sync.go:57).
	aclRequireError(t, aclDo(t, env, http.MethodGet, "/api/v1/apps/datasources/ds-none/sync-status", ""),
		http.StatusNotFound, "DATASOURCE_NOT_FOUND")
	// Cross-tenant data source: indistinguishable from missing (404).
	aclRequireError(t, aclDo(t, env, http.MethodGet, "/api/v1/apps/datasources/ds-a/sync-status", "", "X-Acl-Tenant", "B"),
		http.StatusNotFound, "DATASOURCE_NOT_FOUND")

	// No binding row: the legacy shape — binding=null, no re-authorization,
	// state mirrors the data source status (:75).
	w := aclDo(t, env, http.MethodGet, "/api/v1/apps/datasources/ds-a/sync-status", "")
	if w.Code != http.StatusOK {
		t.Fatalf("legacy status=%d body=%s", w.Code, w.Body.String())
	}
	data := aclData(t, w.Body.String())
	if data["binding"] != nil {
		t.Fatalf("legacy binding=%v want null", data["binding"])
	}
	if data["requires_reauthorization"] != false || data["state"] != "completed" {
		t.Fatalf("legacy view mismatch: %v", data)
	}
	if data["pause_reason"] != nil {
		t.Fatalf("legacy pause_reason=%v want null", data["pause_reason"])
	}

	// Bound to an active space chain: binding view carries
	// installation_id/connection_id/auth_version with the LIVE connection
	// auth version (7), no re-authorization, no pause reason (:103-116).
	w = aclDo(t, env, http.MethodGet, "/api/v1/apps/datasources/ds-b/sync-status", "")
	if w.Code != http.StatusOK {
		t.Fatalf("space status=%d body=%s", w.Code, w.Body.String())
	}
	data = aclData(t, w.Body.String())
	binding, _ := data["binding"].(map[string]any)
	if binding == nil {
		t.Fatalf("space binding missing: %s", w.Body.String())
	}
	if binding["installation_id"] != "inst-s" || binding["connection_id"] != "conn-space" {
		t.Fatalf("space binding mismatch: %v", binding)
	}
	if binding["auth_version"] != "7" {
		t.Fatalf("space auth_version=%v want live 7", binding["auth_version"])
	}
	if data["requires_reauthorization"] != false || data["pause_reason"] != nil {
		t.Fatalf("space view mismatch: %v", data)
	}

	// Bound to a personal connection: requires_reauthorization=true with
	// pause_reason="permission"; the binding view still resolves.
	w = aclDo(t, env, http.MethodGet, "/api/v1/apps/datasources/ds-c/sync-status", "")
	if w.Code != http.StatusOK {
		t.Fatalf("personal status=%d body=%s", w.Code, w.Body.String())
	}
	data = aclData(t, w.Body.String())
	if data["requires_reauthorization"] != true {
		t.Fatalf("personal requires_reauthorization=%v", data["requires_reauthorization"])
	}
	if reason, _ := data["pause_reason"].(string); reason != appconnector.PauseReasonPermission {
		t.Fatalf("personal pause_reason=%v want permission", data["pause_reason"])
	}
	binding, _ = data["binding"].(map[string]any)
	if binding == nil || binding["installation_id"] != "inst-s" || binding["connection_id"] != "conn-pers" {
		t.Fatalf("personal binding mismatch: %v", data["binding"])
	}
}

// TestAppActionPipelineUnwiredFailsClosed anchors the W05 fail-closed
// posture: until SetActionService wires the A03 pipeline, POST
// /apps/actions/prepare refuses with 501 ACTION_PIPELINE_NOT_CONFIGURED
// (action.go:164-168) — the HTTP edge never fabricates an approval. The
// connection lookup happens first, so a live connection row is seeded to
// reach the nil-service branch.
func TestAppActionPipelineUnwiredFailsClosed(t *testing.T) {
	env := aclNewEngine(t)
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(env.db.Create(&appconnectorrepo.ConnectionRow{TenantID: aclTenantA, ID: "conn-1", InstallationID: "inst-s", Kind: appconnector.ConnectionKindSpace, State: appconnector.ConnectionActive, AuthVersion: 1}).Error)

	w := aclDo(t, env, http.MethodPost, "/api/v1/apps/actions/prepare",
		`{"connection_id":"conn-1","target":"doc-1","risk":"read","content":"{}"}`)
	aclRequireError(t, w, http.StatusNotImplemented, "ACTION_PIPELINE_NOT_CONFIGURED")
}
