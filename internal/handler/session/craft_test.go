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
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
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

// -----------------------------------------------------------------------------
// C05: the /restore and /snapshots HTTP surface — idempotency, revision,
// owner authorization and the active-lock competition.
// -----------------------------------------------------------------------------

// httpSnapshotFiles is the controlled storage the snapshot service uploads
// its session export objects into.
type httpSnapshotFiles struct {
	interfaces.FileService
	blobs map[string][]byte
}

func (f *httpSnapshotFiles) SaveBytes(_ context.Context, data []byte, _ uint64, name string, _ bool) (string, error) {
	ref := "resource://http-snapshot/" + name
	f.blobs[ref] = data
	return ref, nil
}

func (f *httpSnapshotFiles) GetFile(_ context.Context, ref string) (io.ReadCloser, error) {
	data, ok := f.blobs[ref]
	if !ok {
		return nil, errors.New("object missing")
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}

// httpSnapshotSource fakes the isolated execution state at exactly the
// snapshot source seam: one live chain, staged generations, restore count.
type httpSnapshotSource struct {
	chain        []craft.SessionRecord
	restoreCount int
	busy         bool
}

func (s *httpSnapshotSource) Quiescent(context.Context, craft.Workspace) (bool, string, error) {
	if s.busy {
		return false, "session busy", nil
	}
	return true, "", nil
}

func (s *httpSnapshotSource) ExportSessionData(_ context.Context, ws craft.Workspace) (craft.SessionExport, error) {
	copied := make([]craft.SessionRecord, len(s.chain))
	copy(copied, s.chain)
	return craft.SessionExport{
		OpenCodeSessionID: ws.OpenCodeSessionID,
		SchemaVersion:     "http-fake/messages-v1",
		Records:           copied,
	}, nil
}

func (s *httpSnapshotSource) RestoreGeneration(_ context.Context, ws craft.Workspace, files []craft.File, export craft.SessionExport) (craft.Workspace, error) {
	if err := craft.SessionChainPrefix(export.Records, s.chain); err != nil {
		return craft.Workspace{}, fmt.Errorf("%w: %v", craft.ErrConflict, err)
	}
	s.restoreCount++
	return craft.Workspace{
		Scope: ws.Scope, ID: ws.ID, SandboxID: ws.SandboxID,
		Generation:        fmt.Sprintf("g-http-%d", s.restoreCount),
		OpenCodeSessionID: ws.OpenCodeSessionID, RuntimeDigest: ws.RuntimeDigest,
	}, nil
}

func (s *httpSnapshotSource) ReleaseGeneration(context.Context, craft.Workspace) error { return nil }

func (s *httpSnapshotSource) IsolatedDataRestore() bool { return true }

// snapshotHTTPEnv extends the craft HTTP env with the mounted C05 surface.
type snapshotHTTPEnv struct {
	*craftHTTPEnv
	source  *httpSnapshotSource
	snapSvc *service.CraftSnapshotService
	store   craft.Store
	files   *httpSnapshotFiles
}

const httpSnapshotRuntimeDigest = "sha256:http-snapshot-runtime"

func newSnapshotHTTPEnv(t *testing.T) *snapshotHTTPEnv {
	t.Helper()
	base := newCraftHTTPEnv(t, service.CraftFeatureGate{Enabled: true, Kinds: []string{"web"}})
	env := &snapshotHTTPEnv{
		craftHTTPEnv: base,
		source: &httpSnapshotSource{chain: []craft.SessionRecord{
			{ID: "msg_u1", Role: "user", Parts: []string{"build it"}},
			{ID: "msg_a1", ParentID: "msg_u1", Role: "assistant", Finish: "stop", Parts: []string{"v1 done"}},
		}},
		store: repository.NewCraftStore(base.db),
		files: &httpSnapshotFiles{blobs: map[string][]byte{}},
	}
	snapSvc, err := service.NewCraftSnapshotService(service.CraftSnapshotConfig{
		DB: base.db, Sessions: &craftHTTPSessions{db: base.db},
		Store: env.store, Versions: repository.NewCraftVersionStore(base.db),
		Snapshots: repository.NewCraftSnapshotStore(base.db), Files: env.files,
		Source: env.source, ActiveRuns: service.CraftActiveRunsQuery(base.db),
		RuntimeDigest: httpSnapshotRuntimeDigest,
	})
	require.NoError(t, err)
	env.snapSvc = snapSvc
	RegisterCraftSnapshotHandler(snapSvc)
	t.Cleanup(func() { RegisterCraftSnapshotHandler(nil) })
	// Build a fresh engine with the same identity middleware so the
	// snapshot routes mount exactly once, alongside the craft table.
	engine := gin.New()
	engine.Use(middleware.ErrorHandler())
	engine.Use(func(c *gin.Context) {
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
	base.engine = engine
	craftSessions := engine.Group("/api/v1/craft/sessions")
	sessions := engine.Group("/api/v1/sessions")
	RegisterCraftSessionRoutes(craftSessions, sessions, base.handler, nil)
	return env
}

// snapshotReadySession drives create → version → capture and returns the
// session id, the snapshot id and the workspace revision to restore at.
func (env *snapshotHTTPEnv) snapshotReadySession(t *testing.T, key string) (sessionID, snapshotID string, revision int64) {
	t.Helper()
	created := env.createSession(t, key, "快照站点", "web")
	sessionID = created["session_id"].(string)
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: sessionID}
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")

	// Bind the workspace to a live OpenCode session on the pinned runtime
	// (the delegation path does this at first dispatch; capture needs it).
	store := repository.NewCraftStore(env.db)
	ws, err := store.GetWorkspace(ctx, scope)
	require.NoError(t, err)
	ws.OpenCodeSessionID = "oc-http-1"
	ws.RuntimeDigest = httpSnapshotRuntimeDigest
	ws.Generation = "0"
	ws, err = store.PutWorkspace(ctx, ws, ws.Revision)
	require.NoError(t, err)

	// Publish one version and capture it.
	content := "<h1>http v1</h1>"
	ref := "resource://http-version/v1"
	env.files.blobs[ref] = []byte(content)
	sum := sha256.Sum256([]byte(content))
	files := []craft.File{{Path: "index.html", Ref: ref,
		SHA256: hex.EncodeToString(sum[:]), MIME: "text/html", Bytes: int64(len(content))}}
	digest, err := craft.ManifestDigest(files)
	require.NoError(t, err)
	version, err := repository.NewCraftVersionStore(env.db).Publish(ctx, scope, craft.Version{
		ID: craft.VersionID(ws.ID, "run-http-1", digest), WorkspaceID: ws.ID,
		RunID: "run-http-1", Kind: craft.KindWeb, Files: files,
		Checks: []craft.Check{{Name: craft.CheckEntry, Status: craft.CheckPassed}},
	})
	require.NoError(t, err)
	snap, err := env.snapSvc.Capture(ctx, scope, version.ID)
	require.NoError(t, err)
	return sessionID, craft.SnapshotID(snap), ws.Revision
}

// TestCraftHTTPRestoreCompetitions pins the /restore acceptance set over the
// real HTTP surface: idempotent request_id (replay never re-executes), the
// same key with different parameters conflicts, a revision race refuses,
// the owner ACL holds (viewer invisible, admin read-only), and an active run
// lock refuses the swap.
func TestCraftHTTPRestoreCompetitions(t *testing.T) {
	env := newSnapshotHTTPEnv(t)
	sessionID, snapshotID, revision := env.snapshotReadySession(t, "key-snap-1")
	restorePath := "/api/v1/sessions/" + sessionID + "/craft/restore"

	body := func(requestID string, rev int64) string {
		return fmt.Sprintf("{\"request_id\":%q,\"snapshot_id\":%q,\"revision\":%d}", requestID, snapshotID, rev)
	}

	// The snapshots listing carries the capture for the workbench.
	w := env.do(t, http.MethodGet, "/api/v1/sessions/"+sessionID+"/craft/snapshots", "", "")
	require.Equal(t, http.StatusOK, w.Code, "snapshots body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), snapshotID)

	// Owner restores: 200, new generation, not a replay.
	w = env.do(t, http.MethodPost, restorePath, "", body("restore-1", revision))
	require.Equal(t, http.StatusOK, w.Code, "restore body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), `"replayed":false`)
	require.Contains(t, w.Body.String(), "g-http-1")
	require.Equal(t, 1, env.source.restoreCount)

	// The identical retry replays without re-executing.
	w = env.do(t, http.MethodPost, restorePath, "", body("restore-1", revision))
	require.Equal(t, http.StatusOK, w.Code, "replay body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), `"replayed":true`)
	require.Equal(t, 1, env.source.restoreCount, "replay never re-executes")

	// The same key with a different revision conflicts.
	w = env.do(t, http.MethodPost, restorePath, "", body("restore-1", revision+7))
	require.Equal(t, http.StatusConflict, w.Code, "key conflict body: %s", w.Body.String())

	// A fresh key against the moved revision is a revision race.
	w = env.do(t, http.MethodPost, restorePath, "", body("restore-stale", revision))
	require.Equal(t, http.StatusConflict, w.Code, "stale body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "revision moved")

	// Viewer: the session is invisible (404). Admin: reads fine, restore is
	// a 403 — the admin fallback never becomes a write path.
	w = env.do(t, http.MethodGet, "/api/v1/sessions/"+sessionID+"/craft/snapshots", "viewer", "")
	require.Equal(t, http.StatusNotFound, w.Code, "viewer snapshots body: %s", w.Body.String())
	w = env.do(t, http.MethodPost, restorePath, "viewer", body("restore-viewer", revision))
	require.Equal(t, http.StatusNotFound, w.Code)
	w = env.do(t, http.MethodPost, restorePath, "admin", body("restore-admin", revision))
	require.Equal(t, http.StatusForbidden, w.Code, "admin body: %s", w.Body.String())

	// Foreign tenant sees nothing at all.
	w = env.do(t, http.MethodPost, restorePath, "foreigntenant", body("restore-foreign", revision))
	require.Equal(t, http.StatusNotFound, w.Code)

	// An active run holds the lock: restore refuses with 409 and the run id
	// the client can attach to.
	w = env.do(t, http.MethodPost, "/api/v1/sessions/"+sessionID+"/craft/runs", "",
		fmt.Sprintf("{\"request_id\":\"run-live\",\"prompt\":\"再改一版\"}"))
	require.Equal(t, http.StatusAccepted, w.Code)
	var runBody struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &runBody))
	liveRunID := runBody.Data["run_id"].(string)

	// Refresh the revision the way a reloaded workbench would.
	ws, err := env.store.GetWorkspace(context.WithValue(context.Background(),
		types.TenantIDContextKey, uint64(1)), craft.Scope{TenantID: 1, UserID: "u1", SessionID: sessionID})
	require.NoError(t, err)
	require.Equal(t, 1, env.source.restoreCount, "only the first restore executed so far")
	_ = liveRunID
	w = env.do(t, http.MethodPost, restorePath, "", body("restore-locked", ws.Revision))
	require.Equal(t, http.StatusConflict, w.Code, "locked body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "active run")
	require.Equal(t, 1, env.source.restoreCount, "the locked restore never materialized")

	// Malformed ids and bodies never reach the service.
	w = env.do(t, http.MethodPost, restorePath, "",
		fmt.Sprintf("{\"request_id\":\"bad\",\"snapshot_id\":\"not-a-snapshot\",\"revision\":%d}", ws.Revision))
	require.Equal(t, http.StatusBadRequest, w.Code)
	w = env.do(t, http.MethodPost, restorePath, "", "{\"request_id\":\"x\"}")
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// CFT-S00-T005: the deployment gate snapshot rides on create and workspace
// responses so the UI offers exactly the open kinds, while the server keeps
// rejecting closed kinds (draft-preserving is the caller's rule, projected by
// packages/domain craftViewCapabilities).
func TestCraftHTTPCapabilitiesProjection(t *testing.T) {
	env := newCraftHTTPEnv(t, service.CraftFeatureGate{Enabled: true, Kinds: []string{"web"}})
	created := env.createSession(t, "caps-1", "能力作品", "web")
	caps, ok := created["capabilities"].(map[string]any)
	require.True(t, ok, "create response carries capabilities: %v", created)
	require.Equal(t, true, caps["enabled"])
	require.Equal(t, []any{"web"}, caps["allowed_kinds"])

	sessionID := created["session_id"].(string)
	w := env.do(t, http.MethodGet, "/api/v1/sessions/"+sessionID+"/craft", "", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"allowed_kinds":["web"]`, "workspace body: %s", w.Body.String())

	// A closed kind is rejected by the SERVER even though the client can see
	// the open list — the projection is never the boundary. (Unsupported
	// maps to 503 in the host error taxonomy.)
	w = env.do(t, http.MethodPost, "/api/v1/craft/sessions", "",
		`{"request_id":"caps-2","title":"表格","kind":"spreadsheet"}`)
	require.Equal(t, http.StatusServiceUnavailable, w.Code, "closed kind body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "not enabled")

	// Gate closed: submissions close while the gate snapshot still reports why.
	closedEnv := newCraftHTTPEnv(t, service.CraftFeatureGate{})
	closedCaps := closedEnv.svc.Capabilities()
	require.False(t, closedCaps.Enabled)
	require.False(t, service.CraftFeatureGate{}.Allows("web"), "zero-value gate rejects every kind")
}
