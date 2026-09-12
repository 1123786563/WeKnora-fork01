package session

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// -----------------------------------------------------------------------------
// Real-database harness: the handler tests drive the real craft session
// service over the real SQLite migrations, mounted through the real route
// registration, with only the session ACL / upload entrance / model catalog
// faked around their exact permission semantics.
// -----------------------------------------------------------------------------

type craftHTTPSessions struct {
	interfaces.SessionService
	db *gorm.DB
}

func (f *craftHTTPSessions) GetSession(ctx context.Context, id string) (*types.Session, error) {
	tenant, _ := types.TenantIDFromContext(ctx)
	user := types.SessionOwnerIDFromContext(ctx)
	var session types.Session
	if err := f.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenant, id).First(&session).Error; err != nil {
		return nil, errors.New("session not found")
	}
	if session.UserID != user && !types.TenantRoleFromContext(ctx).HasPermission(types.TenantRoleAdmin) {
		return nil, errors.New("session not found")
	}
	return &session, nil
}

func (f *craftHTTPSessions) GetOwnedSession(ctx context.Context, id string) (*types.Session, error) {
	tenant, _ := types.TenantIDFromContext(ctx)
	user := types.SessionOwnerIDFromContext(ctx)
	var session types.Session
	if err := f.db.WithContext(ctx).Where("tenant_id = ? AND id = ? AND user_id = ?", tenant, id, user).First(&session).Error; err != nil {
		return nil, errors.New("session not found")
	}
	return &session, nil
}

type craftHTTPDocs struct {
	interfaces.TemporaryDocumentService
	docs  map[string]*types.TemporaryDocument
	blobs map[string][]byte
}

func (f *craftHTTPDocs) Get(_ context.Context, tenantID uint64, sessionID, documentID string) (*types.TemporaryDocument, error) {
	for _, doc := range f.docs {
		if doc.TenantID == tenantID && doc.SessionID == sessionID && doc.ID == documentID {
			return doc, nil
		}
	}
	return nil, errors.New("document not found")
}

func (f *craftHTTPDocs) OpenFile(_ context.Context, tenantID uint64, sessionID, documentID string) (io.ReadCloser, string, error) {
	doc, err := f.Get(context.Background(), tenantID, sessionID, documentID)
	if err != nil {
		return nil, "", err
	}
	return io.NopCloser(strings.NewReader(string(f.blobs[doc.ID]))), doc.FileName, nil
}

type craftHTTPModels struct {
	interfaces.ModelService
	models []*types.Model
}

func (f *craftHTTPModels) ListModels(context.Context) ([]*types.Model, error) { return f.models, nil }

type craftHTTPFiles struct {
	interfaces.FileService
	blobs map[string][]byte
}

func (f *craftHTTPFiles) GetFile(_ context.Context, ref string) (io.ReadCloser, error) {
	data, ok := f.blobs[ref]
	if !ok {
		return nil, errors.New("object missing")
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}

func openCraftHTTPDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dbPath := filepath.Join(t.TempDir(), "craft-http.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()
	db, err := gorm.Open(gormsqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec(
		"INSERT INTO tenants (id, name, business) VALUES (1, 'tenant-1', 'test')").Error)
	require.NoError(t, db.Exec(
		"INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES"+
			" ('u1', 'u1', 'u1@example.test', 'x', 1), ('u2', 'u2', 'u2@example.test', 'x', 1)").Error)
	t.Cleanup(func() {
		conn, e := db.DB()
		if e == nil {
			_ = conn.Close()
		}
	})
	return db
}

// craftHTTPEnv mounts the full authenticated craft surface on a test engine.
// The acting identity is per-request through the auth middleware closure.
type craftHTTPEnv struct {
	db      *gorm.DB
	docs    *craftHTTPDocs
	models  *craftHTTPModels
	files   *craftHTTPFiles
	svc     *service.CraftSessionService
	runs    *service.AgentRunService
	engine  *gin.Engine
	handler *CraftSessionHandler
}

func newCraftHTTPEnv(t *testing.T, gate service.CraftFeatureGate) *craftHTTPEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openCraftHTTPDB(t)
	env := &craftHTTPEnv{
		db:     db,
		docs:   &craftHTTPDocs{docs: map[string]*types.TemporaryDocument{}, blobs: map[string][]byte{}},
		models: &craftHTTPModels{models: []*types.Model{{ID: "m-chat", Type: types.ModelTypeVLLM, IsDefault: true}}},
		files:  &craftHTTPFiles{blobs: map[string][]byte{}},
		runs:   service.NewAgentRunService(repository.NewAgentRunStore(db)),
	}
	svc, err := service.NewCraftSessionService(service.CraftSessionConfig{
		DB: db, Sessions: &craftHTTPSessions{db: db},
		Store: repository.NewCraftStore(db), Versions: repository.NewCraftVersionStore(db),
		Runs: env.runs, ActiveRuns: service.CraftActiveRunsQuery(db),
		TemporaryDocs: env.docs, Files: env.files, Models: env.models, Gate: gate,
	})
	require.NoError(t, err)
	env.svc = svc
	env.handler = NewCraftSessionHandler(svc)

	env.engine = gin.New()
	env.engine.Use(middleware.ErrorHandler())
	// The identity middleware mirrors what production auth sets: the gin KV
	// tenant plus the request-context tenant/user/role values. Each request
	// declares its acting identity through the Identity header so one engine
	// can serve owner, viewer, admin and cross-tenant callers.
	env.engine.Use(func(c *gin.Context) {
		identity := c.GetHeader("X-Test-Identity")
		tenant := uint64(1)
		user, role := "u1", ""
		switch identity {
		case "viewer":
			user = "u2"
		case "admin":
			user, role = "u2", "admin"
		case "foreigntenant":
			tenant, user = 2, "u1"
		}
		c.Set(types.TenantIDContextKey.String(), tenant)
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, tenant)
		ctx = context.WithValue(ctx, types.UserIDContextKey, user)
		if role == "admin" {
			ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleAdmin)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	craftSessions := env.engine.Group("/api/v1/craft/sessions")
	sessions := env.engine.Group("/api/v1/sessions")
	RegisterCraftSessionRoutes(craftSessions, sessions, env.handler, nil)
	return env
}

func (env *craftHTTPEnv) do(t *testing.T, method, path, identity, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if identity != "" {
		req.Header.Set("X-Test-Identity", identity)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	env.engine.ServeHTTP(w, req)
	return w
}

func craftDigest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func (env *craftHTTPEnv) createSession(t *testing.T, key, title, kind string) map[string]any {
	t.Helper()
	w := env.do(t, http.MethodPost, "/api/v1/craft/sessions", "",
		fmt.Sprintf("{\"request_id\":%q,\"title\":%q,\"kind\":%q}", key, title, kind))
	require.Equal(t, http.StatusCreated, w.Code, "create body: %s", w.Body.String())
	var parsed struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &parsed))
	return parsed.Data
}

func (env *craftHTTPEnv) addUpload(t *testing.T, sessionID, docID, status, content string) {
	t.Helper()
	env.docs.docs[docID] = &types.TemporaryDocument{
		ID: docID, TenantID: 1, SessionID: sessionID, FileName: docID + ".txt",
		FileType: "txt", FileSize: int64(len(content)), Status: status,
		ResourceRef: "tempdocs://" + docID,
	}
	env.docs.blobs[docID] = []byte(content)
}

// craftHTTPFinalize releases a run's slot the way the durable worker does.
func (env *craftHTTPEnv) craftHTTPFinalize(t *testing.T, runID string) {
	t.Helper()
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	fence, err := env.runs.Store().Claim(ctx, agentruntime.RunKey{TenantID: 1, RunID: runID}, "craft-http-test", time.Hour)
	require.NoError(t, err)
	events, ok := env.runs.Store().(interface {
		Finalize(context.Context, agentruntime.Fence, json.RawMessage) error
	})
	require.True(t, ok)
	require.NoError(t, events.Finalize(ctx, fence, json.RawMessage(`{"content":"done"}`)))
}

// TestCraftHTTPOwnerCreatesUploadsRunsTwoRounds walks the full owner loop:
// create, associate a completed upload, run round one, hit the active-run
// conflict carrying the existing run id, release the slot and replay/retry.
func TestCraftHTTPOwnerCreatesUploadsRunsTwoRounds(t *testing.T) {
	env := newCraftHTTPEnv(t, service.CraftFeatureGate{Enabled: true, Kinds: []string{"web", "document"}})
	created := env.createSession(t, "key-1", "我的作品站点", "web")
	sessionID := created["session_id"].(string)
	require.NotEmpty(t, created["workspace_id"])
	require.Equal(t, "trpc", created["engine_type"])

	// Associate a completed upload from the existing session entrance.
	env.addUpload(t, sessionID, "doc-ready", types.TemporaryDocumentStatusReady, "craft material")
	w := env.do(t, http.MethodPost, "/api/v1/sessions/"+sessionID+"/craft/inputs", "",
		fmt.Sprintf("{\"resource_ref\":\"doc-ready\",\"expected_sha256\":%q}", craftDigest("craft material")))
	require.Equal(t, http.StatusCreated, w.Code, "inputs body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "tempdocs://doc-ready")
	require.Contains(t, w.Body.String(), "sha256")

	// Round one is admitted as a main run and answers the existing RunView.
	w = env.do(t, http.MethodPost, "/api/v1/sessions/"+sessionID+"/craft/runs", "",
		fmt.Sprintf("{\"request_id\":\"run-1\",\"prompt\":\"做一个落地页\",\"input_refs\":[\"tempdocs://doc-ready\"]}"))
	require.Equal(t, http.StatusAccepted, w.Code, "run body: %s", w.Body.String())
	var runBody struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &runBody))
	runID := runBody.Data["run_id"].(string)
	require.NotEmpty(t, runID)
	require.Equal(t, "trpc", runBody.Data["capabilities"].(map[string]any)["engine_type"])

	// While round one is live, a second modification attempt is a 409 that
	// names the existing run.
	w = env.do(t, http.MethodPost, "/api/v1/sessions/"+sessionID+"/craft/runs", "",
		"{\"request_id\":\"run-2\",\"prompt\":\"再改一版\"}")
	require.Equal(t, http.StatusConflict, w.Code, "conflict body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), runID)

	// Release the slot; a retry of the FIRST key replays the original
	// admission, and the same key with different parameters conflicts.
	env.craftHTTPFinalize(t, runID)
	w = env.do(t, http.MethodPost, "/api/v1/sessions/"+sessionID+"/craft/runs", "",
		fmt.Sprintf("{\"request_id\":\"run-1\",\"prompt\":\"做一个落地页\",\"input_refs\":[\"tempdocs://doc-ready\"]}"))
	require.Equal(t, http.StatusAccepted, w.Code, "replay body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), runID)
	w = env.do(t, http.MethodPost, "/api/v1/sessions/"+sessionID+"/craft/runs", "",
		"{\"request_id\":\"run-1\",\"prompt\":\"不同的提示词\"}")
	require.Equal(t, http.StatusConflict, w.Code)

	// Round two goes through after the slot is free.
	w = env.do(t, http.MethodPost, "/api/v1/sessions/"+sessionID+"/craft/runs", "",
		"{\"request_id\":\"run-2\",\"prompt\":\"再改一版\"}")
	require.Equal(t, http.StatusAccepted, w.Code, "second run body: %s", w.Body.String())
}

// TestCraftHTTPViewerReadOnlyAndCrossTenantInvisible pins the session-level
// permission chain over HTTP: a same-tenant non-admin viewer does not see
// another user's craft session, a tenant admin reads it through the existing
// shared fallback but every write is a 403, and a foreign tenant sees nothing.
func TestCraftHTTPViewerReadOnlyAndCrossTenantInvisible(t *testing.T) {
	env := newCraftHTTPEnv(t, service.CraftFeatureGate{Enabled: true, Kinds: []string{"web"}})
	created := env.createSession(t, "key-1", "站点", "web")
	sessionID := created["session_id"].(string)
	env.addUpload(t, sessionID, "doc-ready", types.TemporaryDocumentStatusReady, "material")

	// Owner reads the workspace view.
	w := env.do(t, http.MethodGet, "/api/v1/sessions/"+sessionID+"/craft", "owner", "")
	require.Equal(t, http.StatusOK, w.Code, "owner view: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "engine_type")
	require.Contains(t, w.Body.String(), "workspace_id")

	// A plain same-tenant viewer does not see the session at all.
	w = env.do(t, http.MethodGet, "/api/v1/sessions/"+sessionID+"/craft", "viewer", "")
	require.Equal(t, http.StatusNotFound, w.Code)
	w = env.do(t, http.MethodGet, "/api/v1/sessions/"+sessionID+"/craft/versions", "viewer", "")
	require.Equal(t, http.StatusNotFound, w.Code)

	// A tenant admin reads through the shared fallback but cannot write.
	w = env.do(t, http.MethodGet, "/api/v1/sessions/"+sessionID+"/craft", "admin", "")
	require.Equal(t, http.StatusOK, w.Code, "admin view: %s", w.Body.String())
	w = env.do(t, http.MethodPost, "/api/v1/sessions/"+sessionID+"/craft/inputs", "admin",
		fmt.Sprintf("{\"resource_ref\":\"doc-ready\",\"expected_sha256\":%q}", craftDigest("material")))
	require.Equal(t, http.StatusForbidden, w.Code, "admin write: %s", w.Body.String())
	w = env.do(t, http.MethodPost, "/api/v1/sessions/"+sessionID+"/craft/runs", "admin",
		"{\"request_id\":\"r\",\"prompt\":\"go\"}")
	require.Equal(t, http.StatusForbidden, w.Code)

	// A foreign tenant never sees the session.
	w = env.do(t, http.MethodGet, "/api/v1/sessions/"+sessionID+"/craft", "foreigntenant", "")
	require.Equal(t, http.StatusNotFound, w.Code)
	w = env.do(t, http.MethodGet, "/api/v1/craft/sessions?limit=10", "foreigntenant", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"success":true,"data":[],"next_cursor":""}`, w.Body.String())
}

// TestCraftHTTPOldBuiltinSessionStaysOffCraft pins that a legacy builtin
// session is refused on the craft write surface instead of being converted.
func TestCraftHTTPOldBuiltinSessionStaysOffCraft(t *testing.T) {
	env := newCraftHTTPEnv(t, service.CraftFeatureGate{Enabled: true, Kinds: []string{"web"}})
	require.NoError(t, env.db.Exec(
		"INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s-builtin', 1, 'old', 'u1', 'builtin')").Error)
	w := env.do(t, http.MethodPost, "/api/v1/sessions/s-builtin/craft/inputs", "owner",
		fmt.Sprintf("{\"resource_ref\":\"x\",\"expected_sha256\":%q}", craftDigest("x")))
	require.Equal(t, http.StatusConflict, w.Code, "builtin inputs: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "builtin sessions cannot become craft sessions")
	w = env.do(t, http.MethodPost, "/api/v1/sessions/s-builtin/craft/runs", "owner",
		"{\"request_id\":\"r\",\"prompt\":\"go\"}")
	require.Equal(t, http.StatusConflict, w.Code)
}

// TestCraftHTTPDependencyNotReady pins the fail-closed surface: with the
// feature gate closed (the default) creation answers 503, and a nil handler
// answers 503 without touching anything.
func TestCraftHTTPDependencyNotReady(t *testing.T) {
	env := newCraftHTTPEnv(t, service.CraftFeatureGate{})
	w := env.do(t, http.MethodPost, "/api/v1/craft/sessions", "owner",
		"{\"request_id\":\"k\",\"title\":\"站点\",\"kind\":\"web\"}")
	require.Equal(t, http.StatusServiceUnavailable, w.Code, "closed gate: %s", w.Body.String())

	// No chat model configured: the run surface fails closed instead of
	// inventing a model identity.
	open := newCraftHTTPEnv(t, service.CraftFeatureGate{Enabled: true, Kinds: []string{"web"}})
	created := open.createSession(t, "key-1", "站点", "web")
	open.models.models = nil
	w = open.do(t, http.MethodPost, "/api/v1/sessions/"+created["session_id"].(string)+"/craft/runs", "owner",
		"{\"request_id\":\"r\",\"prompt\":\"go\"}")
	require.Equal(t, http.StatusServiceUnavailable, w.Code, "no model: %s", w.Body.String())

	// A nil service answers 503 on every endpoint.
	nilHandler := NewCraftSessionHandler(nil)
	engine := gin.New()
	engine.Use(middleware.ErrorHandler())
	engine.POST("/api/v1/craft/sessions", nilHandler.CreateCraftSession)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/craft/sessions", strings.NewReader("{}"))
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	require.Equal(t, http.StatusServiceUnavailable, resp.Code)
}

// TestCraftHTTPBodyAndPromptLimits pins the transport limits: request bodies
// over 1 MiB answer 413 and prompts over 64 KiB answer 400 before any
// admission work.
func TestCraftHTTPBodyAndPromptLimits(t *testing.T) {
	env := newCraftHTTPEnv(t, service.CraftFeatureGate{Enabled: true, Kinds: []string{"web"}})
	created := env.createSession(t, "key-1", "站点", "web")
	sessionID := created["session_id"].(string)

	huge := strings.Repeat("站", service.MaxCraftRequestBodyBytes/3+8)
	w := env.do(t, http.MethodPost, "/api/v1/craft/sessions", "owner",
		fmt.Sprintf("{\"request_id\":\"big\",\"title\":%q,\"kind\":\"web\"}", huge))
	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code, "oversized body: %s", w.Body.String())

	longPrompt := strings.Repeat("改", service.MaxCraftPromptBytes/3+1)
	w = env.do(t, http.MethodPost, "/api/v1/sessions/"+sessionID+"/craft/runs", "owner",
		fmt.Sprintf("{\"request_id\":\"r\",\"prompt\":%q}", longPrompt))
	require.Equal(t, http.StatusBadRequest, w.Code, "oversized prompt: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "prompt")
}

// TestCraftHTTPRejectsClientIdentityFields pins that authority fields a
// client must never supply (tenant, user, worker, sandbox identity) are
// rejected at decode time on every craft endpoint.
func TestCraftHTTPRejectsClientIdentityFields(t *testing.T) {
	env := newCraftHTTPEnv(t, service.CraftFeatureGate{Enabled: true, Kinds: []string{"web"}})
	created := env.createSession(t, "key-1", "站点", "web")
	sessionID := created["session_id"].(string)

	for _, body := range []string{
		"{\"request_id\":\"k\",\"title\":\"x\",\"kind\":\"web\",\"tenant_id\":2}",
		"{\"request_id\":\"k\",\"title\":\"x\",\"kind\":\"web\",\"user_id\":\"attacker\"}",
		"{\"request_id\":\"k\",\"title\":\"x\",\"kind\":\"web\",\"worker\":\"w1\"}",
		"{\"request_id\":\"k\",\"title\":\"x\",\"kind\":\"web\",\"sandbox_id\":\"sbx\"}",
	} {
		w := env.do(t, http.MethodPost, "/api/v1/craft/sessions", "owner", body)
		require.Equal(t, http.StatusBadRequest, w.Code, "body %s -> %s", body, w.Body.String())
	}
	w := env.do(t, http.MethodPost, "/api/v1/sessions/"+sessionID+"/craft/inputs", "owner",
		"{\"resource_ref\":\"d\",\"expected_sha256\":\"x\",\"worker\":\"w\"}")
	require.Equal(t, http.StatusBadRequest, w.Code)
	w = env.do(t, http.MethodPost, "/api/v1/sessions/"+sessionID+"/craft/runs", "owner",
		"{\"request_id\":\"r\",\"prompt\":\"p\",\"tenant_id\":9}")
	require.Equal(t, http.StatusBadRequest, w.Code)

	w = env.do(t, http.MethodPost, "/api/v1/craft/sessions", "owner",
		"{\"request_id\":\"k\",\"title\":\"x\",\"kind\":\"web\"}{}")
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCraftHTTPListAndVersionDownload pins the list envelope (data +
// next_cursor) and the session→workspace→version→file download chain, which
// serves the pinned manifest member and nothing else.
func TestCraftHTTPListAndVersionDownload(t *testing.T) {
	env := newCraftHTTPEnv(t, service.CraftFeatureGate{Enabled: true, Kinds: []string{"web"}})
	first := env.createSession(t, "key-1", "one", "web")
	second := env.createSession(t, "key-2", "two", "web")
	require.NoError(t, env.db.Exec("UPDATE sessions SET updated_at = datetime('now','-1 hour') WHERE id = ?",
		first["session_id"].(string)).Error)

	w := env.do(t, http.MethodGet, "/api/v1/craft/sessions?limit=1", "owner", "")
	require.Equal(t, http.StatusOK, w.Code, "list: %s", w.Body.String())
	var listBody struct {
		Data       []map[string]any ` json:"data" `
		NextCursor string           ` json:"next_cursor" `
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listBody))
	require.Len(t, listBody.Data, 1)
	require.Equal(t, second["session_id"], listBody.Data[0]["session_id"])
	require.Equal(t, "trpc", listBody.Data[0]["engine_type"])
	require.NotEmpty(t, listBody.NextCursor)

	w = env.do(t, http.MethodGet, "/api/v1/craft/sessions?limit=1&cursor="+listBody.NextCursor, "owner", "")
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listBody))
	require.Len(t, listBody.Data, 1)
	require.Equal(t, first["session_id"], listBody.Data[0]["session_id"])
	require.Empty(t, listBody.NextCursor)

	// Publish a real version for the first session, then download its pinned
	// entry file through the full chain.
	sessionID := first["session_id"].(string)
	workspaceID := "ws-http-" + sessionID
	env.db.Exec("UPDATE craft_workspaces SET id = ? WHERE session_id = ?", workspaceID, sessionID)
	page := "<h1>v1</h1>"
	files := []craft.File{{
		Path: "index.html", Ref: "blob://http-index", SHA256: craftDigest(page),
		MIME: "text/html", Bytes: int64(len(page)),
	}}
	env.files.blobs["blob://http-index"] = []byte(page)
	digest, err := craft.ManifestDigest(files)
	require.NoError(t, err)
	versionID := craft.VersionID(workspaceID, "run-v1", digest)
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: sessionID}
	_, err = repository.NewCraftVersionStore(env.db).Publish(context.Background(), scope, craft.Version{
		ID: versionID, WorkspaceID: workspaceID, RunID: "run-v1", Kind: craft.KindWeb,
		Files: files, Checks: craft.BuildChecks(craft.KindWeb, files, craft.ArtifactEvidence{}),
	})
	require.NoError(t, err)

	w = env.do(t, http.MethodGet, "/api/v1/sessions/"+sessionID+"/craft/versions", "owner", "")
	require.Equal(t, http.StatusOK, w.Code, "versions: %s", w.Body.String())
	require.Contains(t, w.Body.String(), versionID)

	w = env.do(t, http.MethodGet, "/api/v1/sessions/"+sessionID+"/craft/versions/"+versionID, "owner", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "index.html")

	w = env.do(t, http.MethodGet, "/api/v1/sessions/"+sessionID+"/craft/versions/"+versionID+"/files/index.html", "owner", "")
	require.Equal(t, http.StatusOK, w.Code, "download: %s", w.Body.String())
	require.Equal(t, page, w.Body.String())
	require.Equal(t, "text/html", w.Header().Get("Content-Type"))
	require.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))

	w = env.do(t, http.MethodGet, "/api/v1/sessions/"+sessionID+"/craft/versions/"+versionID+"/files/style.css", "owner", "")
	require.Equal(t, http.StatusNotFound, w.Code)

	// The isolated-origin traversal shape has no route here at all: the path
	// is a manifest member, never a filesystem path.
	w = env.do(t, http.MethodGet, "/api/v1/sessions/"+sessionID+"/craft/versions/"+versionID+"/files/a/../../escape", "owner", "")
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCraftHTTPRoutesAbsentWithoutRegistration pins the fail-closed mounting:
// without a container-registered craft surface no craft route exists.
func TestCraftHTTPRoutesAbsentWithoutRegistration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	craftSessions := engine.Group("/api/v1/craft/sessions")
	sessions := engine.Group("/api/v1/sessions")
	RegisterCraftSessionRoutes(craftSessions, sessions, nil, nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/craft/sessions", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}
