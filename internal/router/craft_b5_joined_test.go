package router

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/craft"
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

// This fake is used only by the separately named test-gated preview request.
// Production preview configuration in TestCraftB5JoinedCurrentProduction
// keeps both preview gates closed.
type craftB5PreviewNoEgress struct{}

func (craftB5PreviewNoEgress) CheckPreviewNoEgress(context.Context, craft.Scope) error { return nil }

// The auth fakes stop at credential verification. middleware.Auth still
// resolves tenants, active membership roles, principals and API-key scopes.
type craftB5AuthUsers struct{ interfaces.UserService }

func (craftB5AuthUsers) ValidateToken(_ context.Context, token string) (*types.User, uint64, error) {
	if !strings.HasPrefix(token, "b5-user:") {
		return nil, 0, errors.New("invalid test user token")
	}
	userID := strings.TrimPrefix(token, "b5-user:")
	tenantID := uint64(1)
	if userID == "tenant-b" {
		tenantID = 2
	}
	return &types.User{ID: userID, TenantID: tenantID, IsActive: true}, tenantID, nil
}

func (craftB5AuthUsers) GetUserByTenantID(_ context.Context, tenantID uint64) (*types.User, error) {
	return &types.User{ID: "owner", TenantID: tenantID, IsActive: true}, nil
}

type craftB5APIKeys struct{ interfaces.TenantAPIKeyService }

func (craftB5APIKeys) AuthenticateAPIKey(_ context.Context, token string) (*types.TenantAPIKey, error) {
	if token != "b5-api-no-chat" && token != "b5-api-chat" {
		return nil, errors.New("invalid test API key")
	}
	tenantID := uint64(1)
	key := &types.TenantAPIKey{ID: 90, TenantID: &tenantID, ScopeType: types.APIKeyScopeTenant}
	if token == "b5-api-chat" {
		key.Capabilities = types.StringArray{string(types.APIKeyCapabilityChat)}
	}
	return key, nil
}

func (f *craftB5FileService) GetFile(context.Context, string) (io.ReadCloser, error) {
	f.opens++
	return io.NopCloser(bytes.NewReader(append([]byte(nil), f.data...))), nil
}

func TestCraftB5JoinedCurrentProduction(t *testing.T) {
	previousGinMode := gin.Mode()
	t.Cleanup(func() { gin.SetMode(previousGinMode) })
	gin.SetMode(gin.TestMode)
	db := craftB5MigratedDB(t)
	seedCraftB5Rows(t, db)

	access := service.NewCraftAccessService(db)
	versions := repository.NewCraftVersionStore(db)
	files := &craftB5FileService{data: []byte("<!doctype html><title>pinned-b5</title>")}
	// sessionsRepo 还原：重构前版本（13c656301）在此构造 db 真源 repo 并作
	// 第 2 参传入；上游合并把该实参吞成 nil，直接 Session 读路径随即空指针。
	sessionsRepo := repository.NewSessionRepository(db)
	sessionSvc := service.NewSessionService(
		&config.Config{}, sessionsRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, service.HostSandboxManager{}, nil, nil, nil,
		struct{ interfaces.FeedbackRepository }{}, access,
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
	cfg := &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enabled}}
	tenantService := service.NewTenantService(repository.NewTenantRepository(db), nil)
	memberService := service.NewTenantMemberService(repository.NewTenantMemberRepository(db), nil, nil, nil)
	userService := craftB5AuthUsers{}
	apiKeyService := craftB5APIKeys{}
	g := &rbacGuards{cfg: cfg}
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	// Production registers isolated /p/ routes before global credential auth.
	session.RegisterCraftPreviewRoutes(r, previewHandler)
	r.Use(middleware.Auth(tenantService, userService, memberService, apiKeyService, cfg))
	authorizer := g.ensureAPIKeyAuthorizer()
	v1 := r.Group("/api/v1")
	v1.Use(authorizer.Middleware())
	registerSessionRoutes(v1, RouterParams{SessionHandler: session.NewHandler(sessionSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil), CraftFeatureRoutes: features}, g)

	request := func(method, path, user, apiKey, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if user != "" {
			req.Header.Set("Authorization", "Bearer b5-user:"+user)
		}
		if apiKey != "" {
			req.Header.Set("X-API-Key", "b5-api-"+apiKey)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	denialRows := func() []struct {
		ID                                                                  uint64
		TenantID                                                            uint64
		ActorUserID, Action, ScopeType, ScopeID, TargetID, Outcome, Details string
	} {
		t.Helper()
		var rows []struct {
			ID                                                                  uint64
			TenantID                                                            uint64
			ActorUserID, Action, ScopeType, ScopeID, TargetID, Outcome, Details string
		}
		if err := db.Table("audit_logs").Where("action = ? AND scope_id = ?", "craft.access_denied", "task-b5").Order("id").Scan(&rows).Error; err != nil {
			t.Fatal(err)
		}
		return rows
	}
	assertDenied := func(actor, user string, listStatus int, taskACLReached bool) {
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
			beforeAuditRows := denialRows()
			w := request(route.method, route.path, user, "", "")
			listRoute := route.path == "/api/v1/craft/sessions"
			if listRoute && w.Code != listStatus {
				t.Errorf("%s %s actor=%s list status=%d want %d body=%s", route.method, route.path, actor, w.Code, listStatus, w.Body.String())
				if listStatus == http.StatusOK {
					continue
				}
			}
			if !listRoute && w.Code != http.StatusForbidden && w.Code != http.StatusNotFound {
				t.Errorf("%s %s actor=%s status=%d body=%s; want route denial 403/404", route.method, route.path, actor, w.Code, w.Body.String())
			}
			afterAuditRows := denialRows()
			if !listRoute && taskACLReached {
				// Window dedup contract (mirrors middleware LogDenied): the
				// FIRST denial of an (actor, task_action) tuple writes one
				// durable row; repeated identical denials inside the window
				// write none. The route matrix repeats the same read tuple,
				// so the delta is 0 or 1 per route.
				delta := len(afterAuditRows) - len(beforeAuditRows)
				if delta != 0 && delta != 1 {
					t.Errorf("%s %s actor=%s denial audit delta=%d want 0 (window-deduped) or 1 (first tuple denial)", route.method, route.path, actor, delta)
				}
				if delta == 1 {
					row := afterAuditRows[len(afterAuditRows)-1]
					if len(beforeAuditRows) > 0 && row.ID <= beforeAuditRows[len(beforeAuditRows)-1].ID {
						t.Errorf("%s %s actor=%s audit row id=%d did not follow prior row id=%d", route.method, route.path, actor, row.ID, beforeAuditRows[len(beforeAuditRows)-1].ID)
					}
					// Details keys are map-marshaled and therefore sorted:
					// reason before task_action.
					if row.TenantID != 1 || row.ActorUserID != user || row.Action != "craft.access_denied" || row.ScopeType != "session" || row.ScopeID != "task-b5" || row.Outcome != "denied" || row.TargetID != "read" || row.Details != `{"reason":"policy_denied","task_action":"read"}` {
						t.Errorf("%s %s actor=%s audit attribution/details=%+v", route.method, route.path, actor, row)
					}
				}
			} else if len(afterAuditRows) != len(beforeAuditRows) {
				t.Errorf("%s %s actor=%s status=%d added Task denial audit rows: before=%d after=%d", route.method, route.path, actor, w.Code, len(beforeAuditRows), len(afterAuditRows))
			}
			if listRoute && w.Code == http.StatusOK {
				var payload struct {
					Data []struct {
						SessionID string `json:"session_id"`
					} `json:"data"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
					t.Errorf("%s %s actor=%s list JSON: %v", route.method, route.path, actor, err)
				} else {
					for _, item := range payload.Data {
						if item.SessionID == "task-b5" {
							t.Errorf("%s %s actor=%s list included private Task", route.method, route.path, actor)
						}
					}
				}
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
	assertDenied("viewer before grant", "viewer", http.StatusOK, true)
	assertDenied("ordinary admin", "admin", http.StatusOK, true)
	assertDenied("tenant B actor", "tenant-b", http.StatusOK, false)
	assertDenied("same-tenant nonmember", "nonmember", http.StatusForbidden, false)
	// Window-dedup contract: each denied (actor, read) tuple from the joined
	// matrix above owns exactly one durable row even though several routes
	// repeated the denial.
	for _, actor := range []string{"viewer", "admin"} {
		var tupleRows int64
		if err := db.Table("audit_logs").Where("action = ? AND actor_user_id = ? AND scope_id = ? AND target_id = ?", "craft.access_denied", actor, "task-b5", "read").Count(&tupleRows).Error; err != nil {
			t.Fatal(err)
		}
		if tupleRows != 1 {
			t.Errorf("actor=%s read-denial tuple rows=%d want exactly one (window dedup collapses repeats)", actor, tupleRows)
		}
	}
	var joinedDenials []struct {
		TenantID                                                                          uint64
		ActorUserID, Action, ScopeType, ScopeID, TargetID, TargetUserID, Outcome, Details string
	}
	if err := db.Table("audit_logs").Where("action = ?", "craft.access_denied").Find(&joinedDenials).Error; err != nil {
		t.Fatal(err)
	}
	if len(joinedDenials) == 0 {
		t.Fatal("denied joined production HTTP requests created no Craft ACL audit rows")
	}
	validTaskActions := map[string]bool{"read": true, "write": true, "share": true, "open_source": true, "preview": true}
	for _, row := range joinedDenials {
		// TargetID carries the denied task action (the dedup key element);
		// TargetUserID stays empty for policy denials.
		if row.TenantID != 1 || row.ScopeType != "session" || row.ScopeID != "task-b5" || row.Outcome != "denied" || !validTaskActions[row.TargetID] || row.TargetUserID != "" {
			t.Errorf("invalid Craft ACL denial audit attribution: %+v", row)
		}
		if strings.Contains(row.Details, "pinned-b5") || strings.Contains(row.Details, "secret") || strings.Contains(row.Details, "index.html") || strings.Contains(row.Details, "task-b5") {
			t.Errorf("Craft ACL denial details contain sensitive or identifying request data: %q", row.Details)
		}
	}
	var auditBeforeViewerWrite, auditAfterViewerWrite int64
	if err := db.Table("audit_logs").Where("action = ?", "craft.access_denied").Count(&auditBeforeViewerWrite).Error; err != nil {
		t.Fatal(err)
	}
	viewerWrite := request(http.MethodPost, "/api/v1/sessions/task-b5/craft/access?query_secret=secret-marker", "viewer", "", `{"user_id":"body-marker","role":"viewer"}`)
	if viewerWrite.Code != http.StatusForbidden {
		t.Errorf("Viewer access mutation status=%d body=%s want 403", viewerWrite.Code, viewerWrite.Body.String())
	}
	if err := db.Table("audit_logs").Where("action = ?", "craft.access_denied").Count(&auditAfterViewerWrite).Error; err != nil {
		t.Fatal(err)
	}
	if auditAfterViewerWrite != auditBeforeViewerWrite+1 {
		t.Errorf("denied Viewer access mutation audit rows: before=%d after=%d want exactly one", auditBeforeViewerWrite, auditAfterViewerWrite)
	}
	var viewerWriteAudit struct{ ActorUserID, ScopeID, Details string }
	if err := db.Table("audit_logs").Select("actor_user_id, scope_id, details").Where("action = ? AND actor_user_id = ?", "craft.access_denied", "viewer").Order("id DESC").Take(&viewerWriteAudit).Error; err != nil {
		t.Fatal(err)
	}
	if viewerWriteAudit.ScopeID != "task-b5" || strings.Contains(viewerWriteAudit.Details, "secret-marker") || strings.Contains(viewerWriteAudit.Details, "body-marker") || strings.Contains(viewerWriteAudit.Details, "collaborator") {
		t.Errorf("Craft access denial audit leaked query/body/target data: %+v", viewerWriteAudit)
	}
	var preGrantRows int64
	if err := db.Table("craft_task_grants").Count(&preGrantRows).Error; err != nil {
		t.Fatal(err)
	}
	if preGrantRows != 0 {
		t.Errorf("denied Viewer write persisted %d Task grants", preGrantRows)
	}
	var deniedAuditRows int64
	if err := db.Table("audit_logs").Where("action = ?", "craft.access_denied").Count(&deniedAuditRows).Error; err != nil {
		t.Fatal(err)
	}
	t.Logf("durable Craft ACL denial rows before grant/revoke: %d", deniedAuditRows)
	if w := request(http.MethodGet, "/api/v1/sessions/task-b5", "", "", ""); w.Code != http.StatusUnauthorized {
		t.Errorf("anonymous direct Session status=%d want 401", w.Code)
	}
	if w := request(http.MethodGet, "/api/v1/sessions/task-b5", "", "no-chat", ""); w.Code != http.StatusForbidden {
		t.Errorf("API key without chat capability status=%d body=%s want 403", w.Code, w.Body.String())
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
		beforePreviewDenials := len(denialRows())
		w := request(http.MethodPost, "/api/v1/sessions/task-b5/craft/versions/"+version.ID+"/preview", actor, "", "")
		if w.Code != http.StatusBadRequest || strings.Contains(w.Body.String(), "ticket") {
			t.Errorf("disabled preview issue actor=%s status=%d body=%s; want unsupported/no ticket", actor, w.Code, w.Body.String())
		}
		if after := len(denialRows()); after != beforePreviewDenials {
			t.Errorf("production disabled preview gate actor=%s wrote Task ACL denial rows: before=%d after=%d", actor, beforePreviewDenials, after)
		}
	}
	beforePreviewOpens := files.opens
	preview := httptest.NewRecorder()
	previewRequest := httptest.NewRequest(http.MethodGet, "/p/not-a-ticket/index.html", nil)
	previewRequest.Host = "preview.example.test"
	if !previewSvc.AcceptsPreviewHost(previewRequest.Host) {
		t.Fatalf("preview test request host %q is not the configured preview host", previewRequest.Host)
	}
	r.ServeHTTP(preview, previewRequest)
	if preview.Code != http.StatusNotFound || preview.Body.Len() != 0 || files.opens != beforePreviewOpens {
		t.Errorf("disabled preview file status=%d bytes=%d opens=%d want 404/no bytes/no open", preview.Code, preview.Body.Len(), files.opens-beforePreviewOpens)
	}

	for _, actor := range []string{"viewer", "collaborator"} {
		revoke := request(http.MethodPost, "/api/v1/sessions/task-b5/craft/access/revoke", "owner", "", `{"user_id":"`+actor+`"}`)
		if revoke.Code != http.StatusOK {
			t.Errorf("owner revoke %s status=%d body=%s", actor, revoke.Code, revoke.Body.String())
		}
		assertDenied("revoked "+actor, actor, http.StatusOK, true)
	}

	// This separate test-gated route reaches the real preview service ACL with
	// test-only isolation gates. It is not production preview enablement evidence.
	testGatedPreview := service.NewCraftPreviewService(versions, files, nil, service.CraftPreviewConfig{
		AppOrigin: "https://app.example.test", PreviewOrigin: "https://preview.example.test",
		AccessChecker: access, NetworkChecker: craftB5PreviewNoEgress{}, BrowserNavigationProtected: true,
	})
	testPreviewHandler := session.NewCraftPreviewHandler(testGatedPreview)
	testPreviewRouter := gin.New()
	testPreviewRouter.Use(middleware.ErrorHandler(), func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "viewer")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	session.RegisterCraftPreviewIssueRoute(testPreviewRouter.Group("/api/v1/sessions"), testPreviewHandler)
	beforeRevokedPreview := denialRows()
	beforeGatedPreviewOpens := files.opens
	revokedPreview := httptest.NewRecorder()
	testPreviewRouter.ServeHTTP(revokedPreview, httptest.NewRequest(http.MethodPost, "/api/v1/sessions/task-b5/craft/versions/"+version.ID+"/preview", nil))
	if revokedPreview.Code != http.StatusForbidden {
		t.Errorf("test-gated revoked Viewer preview status=%d body=%s want 403", revokedPreview.Code, revokedPreview.Body.String())
	}
	afterRevokedPreview := denialRows()
	if len(afterRevokedPreview) != len(beforeRevokedPreview)+1 {
		t.Errorf("test-gated revoked Viewer preview audit delta=%d want exactly one", len(afterRevokedPreview)-len(beforeRevokedPreview))
	} else {
		row := afterRevokedPreview[len(afterRevokedPreview)-1]
		// Details keys are map-marshaled and therefore sorted.
		if row.TenantID != 1 || row.ActorUserID != "viewer" || row.Action != "craft.access_denied" || row.ScopeType != "session" || row.ScopeID != "task-b5" || row.Outcome != "denied" || row.TargetID != "preview" || row.Details != `{"reason":"policy_denied","task_action":"preview"}` {
			t.Errorf("test-gated revoked Viewer preview audit attribution/details=%+v", row)
		}
	}
	if bytes.Contains(revokedPreview.Body.Bytes(), files.data) || files.opens != beforeGatedPreviewOpens {
		t.Errorf("test-gated revoked Viewer preview exposed bytes or opened storage: body=%q opens=%d", revokedPreview.Body.Bytes(), files.opens-beforeGatedPreviewOpens)
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
	assertDenied("stale membership", "viewer", http.StatusOK, true)
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
	gotAuditTargets := make(map[string]int)
	for _, row := range audits {
		if row.TenantID != 1 || row.ActorUserID != "owner" || row.ScopeType != "session" || row.ScopeID != "task-b5" || row.TargetUserID == "" || row.Outcome != "success" {
			t.Errorf("invalid owner access audit attribution: %+v", row)
		}
		gotAuditTargets[row.Action+"|"+row.TargetUserID]++
	}
	wantAuditTargets := map[string]int{
		"craft.member_added|viewer":         3,
		"craft.member_added|collaborator":   1,
		"craft.member_revoked|viewer":       1,
		"craft.member_revoked|collaborator": 1,
	}
	if !reflect.DeepEqual(gotAuditTargets, wantAuditTargets) {
		t.Errorf("access audit target counts=%v want %v", gotAuditTargets, wantAuditTargets)
	}

	// API-key chat capability authorizes entry to the route group but never
	// substitutes for Task access. A missing actor identity remains private.
	apiOpens := files.opens
	apiFile := request(http.MethodGet, "/api/v1/sessions/task-b5/craft/versions/"+version.ID+"/files/index.html", "", "chat", "")
	if apiFile.Code != http.StatusForbidden {
		t.Errorf("chat API-key without Task grant status=%d body=%s want Craft authorization 403", apiFile.Code, apiFile.Body.String())
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
		`INSERT INTO tenant_members (tenant_id,user_id,role,status,joined_at,created_at,updated_at) VALUES (1,'owner','owner','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),(1,'viewer','viewer','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),(1,'collaborator','contributor','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),(1,'admin','admin','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),(1,'other-owner','viewer','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),(2,'tenant-b','owner','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
}
