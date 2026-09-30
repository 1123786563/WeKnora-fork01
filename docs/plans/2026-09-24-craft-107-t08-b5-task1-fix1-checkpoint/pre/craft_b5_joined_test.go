package router

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type craftB5FileService struct {
	interfaces.FileService
	data  []byte
	opens int
}

func (f *craftB5FileService) GetFile(context.Context, string) (io.ReadCloser, error) {
	f.opens++
	return io.NopCloser(bytes.NewReader(append([]byte(nil), f.data...))), nil
}

func TestCraftB5JoinedCurrentProduction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := craftB5MigratedDB(t)
	seedCraftB5Rows(t, db)

	access := service.NewCraftAccessService(db)
	versions := repository.NewCraftVersionStore(db)
	files := &craftB5FileService{data: []byte("<!doctype html><title>pinned-b5</title>")}
	sessionsRepo := repository.NewSessionRepository(db)
	sessionSvc := service.NewSessionService(
		&config.Config{}, sessionsRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, struct{ interfaces.FeedbackRepository }{}, access,
	)
	runs := service.NewAgentRunService(repository.NewAgentRunStore(db))
	craftSvc, err := service.NewCraftSessionService(service.CraftSessionConfig{
		DB: db, Sessions: sessionSvc, Store: repository.NewCraftStore(db), Versions: versions,
		Runs: runs, ActiveRuns: func(context.Context, craft.Scope) (bool, error) { return false, nil },
		Access: access, TaskList: access, TemporaryDocs: struct {
			interfaces.TemporaryDocumentService
		}{},
		Files: files, Models: struct{ interfaces.ModelService }{},
	})
	if err != nil {
		t.Fatal(err)
	}
	scope := craft.Scope{TenantID: 1, UserID: "owner", SessionID: "task-b5"}
	sum := sha256.Sum256(files.data)
	versionFiles := []craft.File{{Path: "index.html", Ref: "craft-b5-index", SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(files.data)), MIME: "text/html"}}
	digest, err := craft.ManifestDigest(versionFiles)
	if err != nil {
		t.Fatal(err)
	}
	versionID := craft.VersionID("ws-b5", "run-b5", digest)
	version, err := versions.Publish(context.Background(), scope, craft.Version{
		ID: versionID, WorkspaceID: "ws-b5", RunID: "run-b5", Kind: craft.KindWeb,
		Files:  versionFiles,
		Checks: []craft.Check{{Name: craft.CheckEntry, Status: craft.CheckPassed}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if version.ID != versionID {
		t.Fatalf("published version = %q, want %q", version.ID, versionID)
	}

	previewSvc := service.NewCraftPreviewService(versions, files, nil, service.CraftPreviewConfig{
		AppOrigin: "https://app.example.test", PreviewOrigin: "https://preview.example.test",
		AccessChecker: access,
		// Production gates intentionally remain closed; no fake network checker is installed.
	})
	previewHandler := session.NewCraftPreviewHandler(previewSvc)
	oldCraft := session.RegisteredCraftSessionHandler()
	oldPreview := session.RegisteredCraftPreviewRouteHandler()
	session.RegisterCraftSessionHandler(craftSvc)
	session.RegisterCraftPreviewRouteHandler(previewHandler)
	t.Cleanup(func() {
		session.RegisterCraftSessionHandler(oldCraft)
		session.RegisterCraftPreviewRouteHandler(oldPreview)
	})
	features := session.NewCraftFeatureRoutes()
	if err := session.RegisterCraftAccessFeature(features, access); err != nil {
		t.Fatal(err)
	}

	enabled := true
	g := &rbacGuards{cfg: &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enabled}}}
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	// Production registers isolated /p/ routes before global credential auth.
	session.RegisterCraftPreviewRoutes(r, previewHandler)
	// Test credential verifier edge: it supplies the same identity context that
	// middleware.Auth supplies after validating a real credential. Route groups,
	// RBAC checks and API-key policy below are the production implementations.
	r.Use(func(c *gin.Context) {
		user := c.GetHeader("X-Test-User")
		apiKey := c.GetHeader("X-Test-Key")
		if user == "" && apiKey == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		tenantID := uint64(1)
		role := types.TenantRoleViewer
		if user == "" && apiKey != "" {
			user = "api-key-user"
		}
		switch user {
		case "owner":
			role = types.TenantRoleOwner
		case "admin":
			role = types.TenantRoleAdmin
		case "tenant-b":
			tenantID = 2
		}
		ctx := c.Request.Context()
		if user != "" {
			ctx = types.WithCaller(ctx, types.Caller{TenantID: tenantID, UserID: user, Role: role})
			ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
			ctx = context.WithValue(ctx, types.UserIDContextKey, user)
			ctx = context.WithValue(ctx, types.TenantIDContextKey, tenantID)
			c.Set(types.TenantIDContextKey.String(), tenantID)
		}
		if key := c.GetHeader("X-Test-Key"); key != "" {
			scope := types.TenantAPIKeyScope{KeyID: 90}
			if key == "chat" {
				scope.Capabilities = types.StringArray{string(types.APIKeyCapabilityChat)}
			}
			ctx = types.WithTenantAPIKeyScope(ctx, scope)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(g.ensureAPIKeyAuthorizer().Middleware())
	v1 := r.Group("/api/v1")
	registerSessionRoutes(v1, RouterParams{SessionHandler: session.NewHandler(
		sessionSvc, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	), CraftFeatureRoutes: features}, g)

	request := func(method, path, user, apiKey, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if user != "" {
			req.Header.Set("X-Test-User", user)
		}
		if apiKey != "" {
			req.Header.Set("X-Test-Key", apiKey)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	assertDenied := func(actor, user string) {
		t.Helper()
		beforeOpens := files.opens
		paths := []struct{ method, path string }{
			{http.MethodGet, "/api/v1/craft/sessions"},
			{http.MethodGet, "/api/v1/sessions/task-b5"},
			{http.MethodGet, "/api/v1/sessions/task-b5/craft/versions"},
			{http.MethodGet, "/api/v1/sessions/task-b5/craft/versions/" + version.ID},
			{http.MethodGet, "/api/v1/sessions/task-b5/craft/versions/" + version.ID + "/files/index.html"},
		}
		for _, route := range paths {
			w := request(route.method, route.path, user, "", "")
			listRoute := route.path == "/api/v1/craft/sessions"
			if !listRoute && w.Code != http.StatusForbidden && w.Code != http.StatusNotFound {
				t.Errorf("%s %s actor=%s status=%d body=%s; want route denial 403/404", route.method, route.path, actor, w.Code, w.Body.String())
			}
			for _, marker := range []string{"task-b5", version.ID, "pinned-b5", "ws-b5"} {
				if strings.Contains(w.Body.String(), marker) {
					t.Errorf("%s %s actor=%s leaked %q: %s", route.method, route.path, actor, marker, w.Body.String())
				}
			}
			if route.path[strings.LastIndex(route.path, "/")+1:] == "index.html" && bytes.Contains(w.Body.Bytes(), files.data) {
				t.Errorf("file denial actor=%s wrote pinned file bytes", actor)
			}
		}
		if files.opens != beforeOpens {
			t.Errorf("denied requests for actor=%s opened storage %d times", actor, files.opens-beforeOpens)
		}
	}

	// No implicit tenant membership, role or remembered grant exposes Task data.
	assertDenied("viewer before grant", "viewer")
	assertDenied("ordinary admin", "admin")
	assertDenied("tenant B actor", "tenant-b")
	assertDenied("same-tenant nonmember", "nonmember")
	viewerWrite := request(http.MethodPost, "/api/v1/sessions/task-b5/craft/access", "viewer", "", `{"user_id":"collaborator","role":"viewer"}`)
	if viewerWrite.Code != http.StatusForbidden {
		t.Errorf("Viewer access mutation status=%d body=%s want 403", viewerWrite.Code, viewerWrite.Body.String())
	}
	var preGrantRows int64
	if err := db.Table("craft_task_grants").Count(&preGrantRows).Error; err != nil {
		t.Fatal(err)
	}
	if preGrantRows != 0 {
		t.Errorf("denied Viewer write persisted %d Task grants", preGrantRows)
	}
	var deniedAuditRows int64
	if err := db.Table("audit_logs").Count(&deniedAuditRows).Error; err != nil {
		t.Fatal(err)
	}
	t.Logf("audit rows after denied content requests and before grant/revoke: %d (denial-audit acceptance policy remains unresolved)", deniedAuditRows)
	if w := request(http.MethodGet, "/api/v1/sessions/task-b5", "", "", ""); w.Code != http.StatusUnauthorized {
		t.Errorf("anonymous direct Session status=%d want 401", w.Code)
	}
	if w := request(http.MethodGet, "/api/v1/sessions/task-b5", "", "no-chat", ""); w.Code != http.StatusForbidden {
		t.Errorf("API key without chat capability status=%d want 403", w.Code)
	}
	if w := request(http.MethodGet, "/api/v1/sessions/ordinary-b5", "other-owner", "", ""); w.Code != http.StatusOK {
		t.Errorf("non-Craft direct Session owner status=%d want 200", w.Code)
	}
	if w := request(http.MethodGet, "/api/v1/sessions/ordinary-b5", "admin", "", ""); w.Code != http.StatusNotFound {
		t.Errorf("non-Craft direct Session ordinary Admin status=%d want owner-scoped 404", w.Code)
	}
	if w := request(http.MethodGet, "/api/v1/sessions/api-session-b5", "admin", "", ""); w.Code != http.StatusOK {
		t.Errorf("non-Craft API-key Session Admin read status=%d want existing Admin+ channel-read 200", w.Code)
	}
	if w := request(http.MethodGet, "/api/v1/sessions/api-session-b5", "viewer", "", ""); w.Code != http.StatusNotFound {
		t.Errorf("non-Craft API-key Session Viewer read status=%d want owner-scoped 404", w.Code)
	}

	// Owner grants both roles through the production access route. Joined reads
	// must resolve the same Task and immutable version, including generic GetSession.
	for _, actor := range []struct{ user, role string }{{"viewer", "viewer"}, {"collaborator", "collaborator"}} {
		grant := request(http.MethodPost, "/api/v1/sessions/task-b5/craft/access", "owner", "", `{"user_id":"`+actor.user+`","role":"`+actor.role+`"}`)
		if grant.Code != http.StatusOK {
			t.Fatalf("grant %s status=%d body=%s", actor.user, grant.Code, grant.Body.String())
		}
		for _, route := range []struct{ path, marker string }{
			{"/api/v1/craft/sessions", "task-b5"},
			{"/api/v1/sessions/task-b5", "task-b5"},
			{"/api/v1/sessions/task-b5/craft/versions", version.ID},
			{"/api/v1/sessions/task-b5/craft/versions/" + version.ID, version.ID},
		} {
			w := request(http.MethodGet, route.path, actor.user, "", "")
			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), route.marker) {
				t.Errorf("granted %s GET %s status=%d body=%s; want 200 with %q", actor.user, route.path, w.Code, w.Body.String(), route.marker)
			}
		}
		w := request(http.MethodGet, "/api/v1/sessions/task-b5/craft/versions/"+version.ID+"/files/index.html", actor.user, "", "")
		if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), files.data) {
			t.Errorf("granted %s download status=%d bytes=%q; want pinned %q", actor.user, w.Code, w.Body.Bytes(), files.data)
		}
	}

	// Preview routes are mounted from the configured production service, but
	// no ticket or isolated-origin bytes are available while production gates stay off.
	for _, actor := range []string{"owner", "viewer", "collaborator", "admin", "tenant-b"} {
		w := request(http.MethodPost, "/api/v1/sessions/task-b5/craft/versions/"+version.ID+"/preview", actor, "", "")
		if w.Code != http.StatusBadRequest || strings.Contains(w.Body.String(), "ticket") {
			t.Errorf("disabled preview issue actor=%s status=%d body=%s; want unsupported/no ticket", actor, w.Code, w.Body.String())
		}
	}
	beforePreviewOpens := files.opens
	preview := httptest.NewRecorder()
	r.ServeHTTP(preview, httptest.NewRequest(http.MethodGet, "/p/not-a-ticket/index.html", nil))
	if preview.Code != http.StatusNotFound || preview.Body.Len() != 0 || files.opens != beforePreviewOpens {
		t.Errorf("disabled preview file status=%d bytes=%d opens=%d want 404/no bytes/no open", preview.Code, preview.Body.Len(), files.opens-beforePreviewOpens)
	}

	for _, actor := range []string{"viewer", "collaborator"} {
		revoke := request(http.MethodPost, "/api/v1/sessions/task-b5/craft/access/revoke", "owner", "", `{"user_id":"`+actor+`"}`)
		if revoke.Code != http.StatusOK {
			t.Errorf("owner revoke %s status=%d body=%s", actor, revoke.Code, revoke.Body.String())
		}
		assertDenied("revoked "+actor, actor)
	}

	// An old grant is bound to the old membership incarnation and cannot revive
	// when the same principal rejoins; it needs an explicit fresh grant.
	if err := access.Grant(context.Background(), scope, "viewer", craft.TaskRoleViewer); err != nil {
		t.Fatal(err)
	}
	var oldID uint64
	if err := db.Table("tenant_members").Where("tenant_id = ? AND user_id = ? AND status = 'active'", 1, "viewer").Pluck("id", &oldID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DELETE FROM tenant_members WHERE id = ?", oldID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO tenant_members (tenant_id,user_id,role,status,joined_at,created_at,updated_at) VALUES (1,'viewer','viewer','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)").Error; err != nil {
		t.Fatal(err)
	}
	assertDenied("stale membership", "viewer")
	if w := request(http.MethodPost, "/api/v1/sessions/task-b5/craft/access", "owner", "", `{"user_id":"viewer","role":"viewer"}`); w.Code != http.StatusOK {
		t.Fatalf("fresh owner grant after rejoin status=%d body=%s", w.Code, w.Body.String())
	}
	w := request(http.MethodGet, "/api/v1/sessions/task-b5/craft/versions/"+version.ID, "viewer", "", "")
	if w.Code != http.StatusOK {
		t.Errorf("fresh grant after rejoin version status=%d body=%s", w.Code, w.Body.String())
	}

	ownerRead := request(http.MethodGet, "/api/v1/sessions/task-b5/craft/versions/"+version.ID+"/files/index.html", "owner", "", "")
	if ownerRead.Code != http.StatusOK || !bytes.Equal(ownerRead.Body.Bytes(), files.data) {
		t.Errorf("owner read after revokes status=%d bytes=%q", ownerRead.Code, ownerRead.Body.Bytes())
	}
	var audits []struct {
		TenantID                                                       uint64
		ActorUserID, Action, ScopeType, ScopeID, TargetUserID, Outcome string
	}
	if err := db.Table("audit_logs").Select("tenant_id, actor_user_id, action, scope_type, scope_id, target_user_id, outcome").Where("action IN ?", []string{"craft.member_added", "craft.member_revoked"}).Scan(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if len(audits) != 6 {
		t.Errorf("successful grant/revoke audit rows=%d want 6; rows=%+v", len(audits), audits)
	}
	for _, row := range audits {
		if row.TenantID != 1 || row.ActorUserID != "owner" || row.ScopeType != "session" || row.ScopeID != "task-b5" || row.TargetUserID == "" || row.Outcome != "success" {
			t.Errorf("invalid owner access audit attribution: %+v", row)
		}
	}

	// API-key chat capability authorizes entry to the route group but never
	// substitutes for Task access. A missing actor identity remains private.
	apiOpens := files.opens
	apiFile := request(http.MethodGet, "/api/v1/sessions/task-b5/craft/versions/"+version.ID+"/files/index.html", "", "chat", "")
	if apiFile.Code != http.StatusForbidden && apiFile.Code != http.StatusUnauthorized && apiFile.Code != http.StatusNotFound {
		t.Errorf("chat API-key without Task grant status=%d body=%s", apiFile.Code, apiFile.Body.String())
	}
	if bytes.Contains(apiFile.Body.Bytes(), files.data) || files.opens != apiOpens {
		t.Errorf("chat API-key without Task grant exposed bytes or opened storage: response=%q opens=%d", apiFile.Body.Bytes(), files.opens-apiOpens)
	}
}

func craftB5MigratedDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve router test source path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "craft-b5.db") + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatal(err)
	}
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	if err != nil {
		t.Fatal(err)
	}
	migrator, err := migrate.NewWithDatabaseInstance("file:"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrator.Up(); err != nil {
		t.Fatal(err)
	}
	_, _ = migrator.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func seedCraftB5Rows(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, stmt := range []string{
		`INSERT INTO tenants (id, name, business) VALUES (1, 'tenant-a', 'test'), (2, 'tenant-b', 'test')`,
		`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('task-b5', 1, 'private-task-b5', 'owner', 'trpc'), ('ordinary-b5', 1, 'ordinary-owner', 'other-owner', 'trpc'), ('api-session-b5', 1, 'api-key-session', 'api_tenant_key:90', 'trpc')`,
		`INSERT INTO craft_sessions (session_id, tenant_id, kind) VALUES ('task-b5', 1, 'web')`,
		`INSERT INTO craft_workspaces (id, tenant_id, session_id, owner_id, sandbox_id, generation) VALUES ('ws-b5', 1, 'task-b5', 'owner', 'sandbox-b5', '1')`,
		`INSERT INTO tenant_members (tenant_id,user_id,role,status,joined_at,created_at,updated_at) VALUES (1,'owner','owner','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),(1,'viewer','viewer','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),(1,'collaborator','contributor','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),(1,'admin','admin','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),(2,'tenant-b','owner','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
}
