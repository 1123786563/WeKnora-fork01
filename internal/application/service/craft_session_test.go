package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openCraftSessionDB applies the real SQLite migrations (000045_craft_sessions
// included) and seeds the tenant/user fixtures.
func openCraftSessionDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dbPath := filepath.Join(t.TempDir(), "craft-sessions.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"

	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver,
	)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()

	db, err := gorm.Open(gormsqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec(
		`INSERT INTO tenants (id, name, business) VALUES (1, 'tenant-1', 'test')`,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO users (id, username, email, password_hash, tenant_id)
		 VALUES ('u1', 'u1', 'u1@example.test', 'x', 1), ('u2', 'u2', 'u2@example.test', 'x', 1)`,
	).Error)
	t.Cleanup(func() {
		conn, e := db.DB()
		if e == nil {
			_ = conn.Close()
		}
	})
	return db
}

// fakeCraftSessions backs the session permission chain with the real rows:
// GetSession is the read path (owner scope with an Admin+ fallback for
// tenant channel sessions, mirroring loadSessionForRead), GetOwnedSession is
// the strict owner path.
type fakeCraftSessions struct {
	interfaces.SessionService
	db *gorm.DB
}

func (f *fakeCraftSessions) GetSession(ctx context.Context, id string) (*types.Session, error) {
	tenant, _ := types.TenantIDFromContext(ctx)
	user := types.SessionOwnerIDFromContext(ctx)
	var session types.Session
	err := f.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenant, id).First(&session).Error
	if err != nil {
		return nil, errors.New("session not found")
	}
	if session.UserID != user && !types.TenantRoleFromContext(ctx).HasPermission(types.TenantRoleAdmin) {
		return nil, errors.New("session not found")
	}
	return &session, nil
}

func (f *fakeCraftSessions) GetOwnedSession(ctx context.Context, id string) (*types.Session, error) {
	tenant, _ := types.TenantIDFromContext(ctx)
	user := types.SessionOwnerIDFromContext(ctx)
	var session types.Session
	err := f.db.WithContext(ctx).Where("tenant_id = ? AND id = ? AND user_id = ?", tenant, id, user).First(&session).Error
	if err != nil {
		return nil, errors.New("session not found")
	}
	return &session, nil
}

// fakeCraftDocs fakes the session attachment entrance.
type fakeCraftDocs struct {
	interfaces.TemporaryDocumentService
	docs  map[string]*types.TemporaryDocument
	blobs map[string][]byte
}

func (f *fakeCraftDocs) Get(_ context.Context, tenantID uint64, sessionID, documentID string) (*types.TemporaryDocument, error) {
	for _, doc := range f.docs {
		if doc.TenantID == tenantID && doc.SessionID == sessionID && doc.ID == documentID {
			return doc, nil
		}
	}
	return nil, errors.New("document not found")
}

func (f *fakeCraftDocs) OpenFile(_ context.Context, tenantID uint64, sessionID, documentID string) (io.ReadCloser, string, error) {
	doc, err := f.Get(context.Background(), tenantID, sessionID, documentID)
	if err != nil {
		return nil, "", err
	}
	return io.NopCloser(strings.NewReader(string(f.blobs[doc.ID]))), doc.FileName, nil
}

// fakeCraftModels fakes the model catalog with one default chat model.
type fakeCraftModels struct {
	interfaces.ModelService
	models []*types.Model
}

func (f *fakeCraftModels) ListModels(context.Context) ([]*types.Model, error) {
	return f.models, nil
}

// craftSessionEnv assembles the real service over the real stores.
type craftSessionEnv struct {
	db       *gorm.DB
	sessions *fakeCraftSessions
	docs     *fakeCraftDocs
	models   *fakeCraftModels
	svc      *CraftSessionService
	runs     *AgentRunService
	store    craft.Store
	versions craft.VersionStore
}

func newCraftSessionEnv(t *testing.T, gate CraftFeatureGate) *craftSessionEnv {
	t.Helper()
	db := openCraftSessionDB(t)
	env := &craftSessionEnv{
		db:       db,
		sessions: &fakeCraftSessions{db: db},
		docs:     &fakeCraftDocs{docs: map[string]*types.TemporaryDocument{}, blobs: map[string][]byte{}},
		models:   &fakeCraftModels{models: []*types.Model{{ID: "m-chat", Type: types.ModelTypeVLLM, IsDefault: true}}},
		runs:     NewAgentRunService(repository.NewAgentRunStore(db)),
		store:    repository.NewCraftStore(db),
		versions: repository.NewCraftVersionStore(db),
	}
	svc, err := NewCraftSessionService(CraftSessionConfig{
		DB: db, Sessions: env.sessions, Store: env.store, Versions: env.versions,
		Runs: env.runs, ActiveRuns: CraftActiveRunsQuery(db),
		TemporaryDocs: env.docs, Files: &fakeCraftFiles{blobs: map[string][]byte{}}, Models: env.models, Gate: gate,
	})
	require.NoError(t, err)
	env.svc = svc
	return env
}

// craftCtx builds a request context carrying the craft scope identity.
func craftCtx(tenantID uint64, userID, sessionID string) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, tenantID)
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	return ctx
}

func adminCraftCtx(tenantID uint64, userID, sessionID string) context.Context {
	return context.WithValue(craftCtx(tenantID, userID, sessionID),
		types.TenantRoleContextKey, types.TenantRoleAdmin)
}

func ownerScope(tenantID uint64, userID, sessionID string) craft.Scope {
	return craft.Scope{TenantID: tenantID, UserID: userID, SessionID: sessionID}
}

var openGate = CraftFeatureGate{Enabled: true, Kinds: []string{"web", "document"}}

// createCraftSession drives the real create path and returns the workspace.
func createCraftSession(t *testing.T, env *craftSessionEnv, user, key, title, kind string) craft.Workspace {
	t.Helper()
	ctx := craftCtx(1, user, "")
	ws, err := env.svc.Create(ctx, ownerScope(1, user, ""),
		craft.CreateRequest{RequestID: key, Title: title, Kind: kind})
	require.NoError(t, err)
	require.NotEmpty(t, ws.ID)
	return ws
}

// --- Create -----------------------------------------------------------------

func TestCraftSessionCreateRegistersTrpcSessionWithIdempotency(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	ctx := craftCtx(1, "u1", "")

	first, err := env.svc.Create(ctx, ownerScope(1, "u1", ""),
		craft.CreateRequest{RequestID: "key-1", Title: "我的作品站点", Kind: "web"})
	require.NoError(t, err)
	require.NotEmpty(t, first.ID)
	require.NotEmpty(t, first.SessionID)

	// The sessions row is trpc and owned by the caller; the craft
	// registration pins the kind; the workspace binding row exists.
	var session types.Session
	require.NoError(t, env.db.Where("tenant_id = ? AND id = ?", 1, first.SessionID).First(&session).Error)
	require.Equal(t, "trpc", session.EngineType)
	require.Equal(t, "u1", session.UserID)
	var registration struct{ Kind string }
	require.NoError(t, env.db.Table("craft_sessions").
		Where("session_id = ?", first.SessionID).Select("kind").Scan(&registration).Error)
	require.Equal(t, "web", registration.Kind)
	ws, err := env.store.GetWorkspace(ctx, ownerScope(1, "u1", first.SessionID))
	require.NoError(t, err)
	require.Equal(t, first.ID, ws.ID)

	// Same key, same parameters: the same session.
	replay, err := env.svc.Create(ctx, ownerScope(1, "u1", ""),
		craft.CreateRequest{RequestID: "key-1", Title: "我的作品站点", Kind: "web"})
	require.NoError(t, err)
	require.Equal(t, first.SessionID, replay.SessionID)
	require.Equal(t, first.ID, replay.ID)

	// Same key, different parameters: a conflict, not a replacement.
	_, err = env.svc.Create(ctx, ownerScope(1, "u1", ""),
		craft.CreateRequest{RequestID: "key-1", Title: "换个标题", Kind: "web"})
	require.ErrorIs(t, err, craft.ErrConflict)

	// A different key creates a different session; keys are user-scoped.
	second, err := env.svc.Create(craftCtx(1, "u2", ""), ownerScope(1, "u2", ""),
		craft.CreateRequest{RequestID: "key-1", Title: "我的作品站点", Kind: "web"})
	require.NoError(t, err)
	require.NotEqual(t, first.SessionID, second.SessionID)
}

func TestCraftSessionCreateGateFiltersKindAgain(t *testing.T) {
	// The kind is valid but not open: type validity is not openness.
	env := newCraftSessionEnv(t, CraftFeatureGate{Enabled: true, Kinds: []string{"web"}})
	_, err := env.svc.Create(craftCtx(1, "u1", ""), ownerScope(1, "u1", ""),
		craft.CreateRequest{RequestID: "k", Title: "x", Kind: "slides"})
	require.ErrorIs(t, err, craft.ErrUnsupported)

	// The whole surface stays closed by default.
	closed := newCraftSessionEnv(t, CraftFeatureGate{})
	_, err = closed.svc.Create(craftCtx(1, "u1", ""), ownerScope(1, "u1", ""),
		craft.CreateRequest{RequestID: "k", Title: "x", Kind: "web"})
	require.ErrorIs(t, err, craft.ErrUnsupported)

	// Unknown kinds are refused by the pure rules first.
	_, err = env.svc.Create(craftCtx(1, "u1", ""), ownerScope(1, "u1", ""),
		craft.CreateRequest{RequestID: "k", Title: "x", Kind: "public-hosting"})
	require.ErrorIs(t, err, craft.ErrUnsupported)
}

// --- Read / write chains ----------------------------------------------------

func TestCraftSessionPermissionChains(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	ws := createCraftSession(t, env, "u1", "k1", "站点", "web")
	owner := ownerScope(1, "u1", ws.SessionID)

	// Owner reads the workspace and view.
	got, err := env.svc.Get(craftCtx(1, "u1", ws.SessionID), owner)
	require.NoError(t, err)
	require.Equal(t, ws.ID, got.ID)
	view, err := env.svc.View(craftCtx(1, "u1", ws.SessionID), owner)
	require.NoError(t, err)
	require.Equal(t, "web", view.Kind)
	require.Equal(t, "trpc", view.EngineType)
	require.Equal(t, "站点", view.Title)

	// Another tenant cannot even see the session.
	_, err = env.svc.Get(craftCtx(2, "u1", ws.SessionID), ownerScope(2, "u1", ws.SessionID))
	require.ErrorIs(t, err, craft.ErrNotFound)

	// A same-tenant non-member does not see it either (no admin role).
	_, err = env.svc.Get(craftCtx(1, "u2", ws.SessionID), ownerScope(1, "u2", ws.SessionID))
	require.ErrorIs(t, err, craft.ErrNotFound)
	// A tenant admin reads it through the shared fallback but cannot write.
	_, err = env.svc.Get(adminCraftCtx(1, "u2", ws.SessionID), ownerScope(1, "u2", ws.SessionID))
	require.NoError(t, err)
	_, err = env.svc.AssociateInput(adminCraftCtx(1, "u2", ws.SessionID),
		ownerScope(1, "u2", ws.SessionID), "doc-1", strings.Repeat("a", 64))
	require.ErrorIs(t, err, craft.ErrForbidden)

	// A trpc session without a craft registration is not a craft session.
	require.NoError(t, env.db.Exec(
		`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s-plain', 1, 'plain', 'u1', 'trpc')`,
	).Error)
	_, err = env.svc.Get(craftCtx(1, "u1", "s-plain"), ownerScope(1, "u1", "s-plain"))
	require.ErrorIs(t, err, craft.ErrNotFound)

	// A builtin session can never become a craft session.
	require.NoError(t, env.db.Exec(
		`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s-builtin', 1, 'old', 'u1', 'builtin')`,
	).Error)
	_, err = env.svc.AssociateInput(craftCtx(1, "u1", "s-builtin"), ownerScope(1, "u1", "s-builtin"),
		"doc-1", strings.Repeat("a", 64))
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Contains(t, err.Error(), "builtin sessions cannot become craft sessions")
}

// --- Inputs -----------------------------------------------------------------

func addCraftUpload(t *testing.T, env *craftSessionEnv, sessionID, docID, status, content string) {
	t.Helper()
	sum := sha256Sum(content)
	env.docs.docs[docID] = &types.TemporaryDocument{
		ID: docID, TenantID: 1, SessionID: sessionID, FileName: docID + ".txt",
		FileType: "txt", FileSize: int64(len(content)), Status: status,
		ResourceRef: "tempdocs://" + docID,
	}
	env.docs.blobs[docID] = []byte(content)
	_ = sum
}

func TestCraftSessionAssociateInputVerifiesCompletedUpload(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	ws := createCraftSession(t, env, "u1", "k1", "站点", "web")
	ctx := craftCtx(1, "u1", ws.SessionID)
	good := "craft input material"
	addCraftUpload(t, env, ws.SessionID, "doc-ready", types.TemporaryDocumentStatusReady, good)
	addCraftUpload(t, env, ws.SessionID, "doc-processing", types.TemporaryDocumentStatusProcessing, good)
	addCraftUpload(t, env, "other-session", "doc-foreign", types.TemporaryDocumentStatusReady, good)

	digest := sha256Sum(good)
	input, err := env.svc.AssociateInput(ctx, ownerScope(1, "u1", ws.SessionID), "doc-ready", digest)
	require.NoError(t, err)
	require.Equal(t, "tempdocs://doc-ready", input.Ref)
	require.Equal(t, "doc-ready.txt", input.Name)
	require.Equal(t, digest, input.SHA256)
	require.Equal(t, int64(len(good)), input.Bytes)

	// Re-associating the same upload is idempotent.
	again, err := env.svc.AssociateInput(ctx, ownerScope(1, "u1", ws.SessionID), "doc-ready", digest)
	require.NoError(t, err)
	require.Equal(t, input, again)

	// A wrong digest never enters the manifest.
	_, err = env.svc.AssociateInput(ctx, ownerScope(1, "u1", ws.SessionID),
		"doc-ready", sha256Sum("tampered"))
	require.ErrorIs(t, err, craft.ErrInvalidInput)

	// An upload still processing is refused until it completes.
	_, err = env.svc.AssociateInput(ctx, ownerScope(1, "u1", ws.SessionID),
		"doc-processing", digest)
	require.ErrorIs(t, err, craft.ErrConflict)

	// Another session's upload does not exist for this caller.
	_, err = env.svc.AssociateInput(ctx, ownerScope(1, "u1", ws.SessionID),
		"doc-foreign", digest)
	require.ErrorIs(t, err, craft.ErrNotFound)

	// Malformed digests are refused before any read.
	_, err = env.svc.AssociateInput(ctx, ownerScope(1, "u1", ws.SessionID), "doc-ready", "zz")
	require.ErrorIs(t, err, craft.ErrInvalidInput)
}

func sha256Sum(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// --- Runs -------------------------------------------------------------------

func TestCraftSessionStartRunAdmitsThroughSubmit(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	ws := createCraftSession(t, env, "u1", "k1", "站点", "web")
	ctx := craftCtx(1, "u1", ws.SessionID)
	addCraftUpload(t, env, ws.SessionID, "doc-ready", types.TemporaryDocumentStatusReady, "material")
	input, err := env.svc.AssociateInput(ctx, ownerScope(1, "u1", ws.SessionID), "doc-ready", sha256Sum("material"))
	require.NoError(t, err)

	run, err := env.svc.StartRun(ctx, ownerScope(1, "u1", ws.SessionID), CraftRunRequest{
		RequestID: "run-1", Prompt: "做一个落地页", InputRefs: []string{input.Ref},
	})
	require.NoError(t, err)
	require.Equal(t, "queued", run.Status)
	require.Equal(t, ws.SessionID, run.SessionID)

	// The run slot is reserved: a second run is refused with the run id.
	_, err = env.svc.StartRun(ctx, ownerScope(1, "u1", ws.SessionID), CraftRunRequest{
		RequestID: "run-2", Prompt: "再改一版",
	})
	require.ErrorIs(t, err, craft.ErrBusy)
	var conflict *ActiveRunConflict
	require.ErrorAs(t, err, &conflict)
	require.Equal(t, run.Key.RunID, conflict.RunID)

	// Releasing the slot (terminal run) lets the second round in.
	fence, err := env.runs.Store().Claim(ctx, run.Key, "craft-test-worker", time.Hour)
	require.NoError(t, err)
	events, ok := env.runs.Store().(interface {
		Finalize(context.Context, agentruntime.Fence, json.RawMessage) error
	})
	require.True(t, ok)
	require.NoError(t, events.Finalize(ctx, fence, json.RawMessage(`{"done":true}`)))

	second, err := env.svc.StartRun(ctx, ownerScope(1, "u1", ws.SessionID), CraftRunRequest{
		RequestID: "run-2", Prompt: "再改一版",
	})
	require.NoError(t, err)
	require.NotEqual(t, run.Key.RunID, second.Key.RunID)

	// Retrying the FIRST key after completion replays the original admission.
	replay, err := env.svc.StartRun(ctx, ownerScope(1, "u1", ws.SessionID), CraftRunRequest{
		RequestID: "run-1", Prompt: "做一个落地页", InputRefs: []string{input.Ref},
	})
	require.NoError(t, err)
	require.Equal(t, run.Key.RunID, replay.Key.RunID)
}

func TestCraftSessionStartRunValidatesWorkspaceState(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	ws := createCraftSession(t, env, "u1", "k1", "站点", "web")
	ctx := craftCtx(1, "u1", ws.SessionID)

	// Unknown input refs never reach admission.
	_, err := env.svc.StartRun(ctx, ownerScope(1, "u1", ws.SessionID), CraftRunRequest{
		RequestID: "r", Prompt: "go", InputRefs: []string{"tempdocs://not-associated"},
	})
	require.ErrorIs(t, err, craft.ErrInvalidInput)

	// Unknown base versions are refused.
	_, err = env.svc.StartRun(ctx, ownerScope(1, "u1", ws.SessionID), CraftRunRequest{
		RequestID: "r", Prompt: "go", BaseVersionID: "ver_" + strings.Repeat("0", 64),
	})
	require.ErrorIs(t, err, craft.ErrNotFound)

	// Oversized prompts are refused before admission.
	_, err = env.svc.StartRun(ctx, ownerScope(1, "u1", ws.SessionID), CraftRunRequest{
		RequestID: "r", Prompt: strings.Repeat("长", 32769),
	})
	require.ErrorIs(t, err, craft.ErrInvalidInput)

	// Without a chat model the surface fails closed instead of inventing one.
	env.models.models = nil
	_, err = env.svc.StartRun(ctx, ownerScope(1, "u1", ws.SessionID), CraftRunRequest{
		RequestID: "r", Prompt: "go",
	})
	require.ErrorIs(t, err, craft.ErrUnsupported)
}

func TestCraftSessionListCursorFollowsExistingScope(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	first := createCraftSession(t, env, "u1", "k1", "one", "web")
	second := createCraftSession(t, env, "u1", "k2", "two", "web")
	createCraftSession(t, env, "u2", "k1", "other user", "web")
	// Keyset cursors need distinguishable updated_at values. Write the backdated
	// value in the same UTC form gorm stores, so lexicographic comparisons on
	// drivers that compare DATETIME as text stay consistent.
	require.NoError(t, env.db.Exec("UPDATE sessions SET updated_at = datetime('now', '-1 hour') WHERE id = ?",
		first.SessionID).Error)

	page, next, err := env.svc.List(craftCtx(1, "u1", ""), ownerScope(1, "u1", ""), "", 1)
	require.NoError(t, err)
	require.Len(t, page, 1)
	require.NotEmpty(t, next)
	// Newest first, own sessions only.
	require.Equal(t, second.SessionID, page[0].SessionID)
	require.Equal(t, "trpc", page[0].EngineType)

	page2, next2, err := env.svc.List(craftCtx(1, "u1", ""), ownerScope(1, "u1", ""), next, 1)
	require.NoError(t, err)
	require.Len(t, page2, 1)
	require.Empty(t, next2)
	require.Equal(t, first.SessionID, page2[0].SessionID)

	// Malformed cursors are input errors.
	_, _, err = env.svc.List(craftCtx(1, "u1", ""), ownerScope(1, "u1", ""), "!!!", 10)
	require.ErrorIs(t, err, craft.ErrInvalidInput)
}

// --- Versions and download --------------------------------------------------

func publishCraftVersion(t *testing.T, env *craftSessionEnv, scope craft.Scope, body string) craft.Version {
	t.Helper()
	files := []craft.File{{
		Path: "index.html", Ref: "blob://v1-index", SHA256: sha256Sum(body),
		MIME: "text/html", Bytes: int64(len(body)),
	}}
	digest, err := craft.ManifestDigest(files)
	require.NoError(t, err)
	workspaceID := "ws-of-" + scope.SessionID
	version, err := env.versions.Publish(context.Background(), scope, craft.Version{
		ID: craft.VersionID(workspaceID, "run-v1", digest), WorkspaceID: workspaceID,
		RunID: "run-v1", Kind: craft.KindWeb, Files: files,
		Checks: craft.BuildChecks(craft.KindWeb, files, craft.ArtifactEvidence{}),
	})
	require.NoError(t, err)
	return version
}

func TestCraftSessionVersionsAndDownloadChain(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	ws := createCraftSession(t, env, "u1", "k1", "站点", "web")
	owner := ownerScope(1, "u1", ws.SessionID)
	env.db.Exec(`UPDATE craft_workspaces SET id = ? WHERE session_id = ?`, "ws-of-"+ws.SessionID, ws.SessionID)
	version := publishCraftVersion(t, env, owner, "<h1>v1</h1>")

	versions, err := env.svc.ListVersions(craftCtx(1, "u1", ws.SessionID), owner)
	require.NoError(t, err)
	require.Len(t, versions, 1)

	got, err := env.svc.GetVersion(craftCtx(1, "u1", ws.SessionID), owner, version.ID)
	require.NoError(t, err)
	require.Equal(t, version.ID, got.ID)

	// The view reports the current version.
	view, err := env.svc.View(craftCtx(1, "u1", ws.SessionID), owner)
	require.NoError(t, err)
	require.NotNil(t, view.CurrentVersion)
	require.Equal(t, version.ID, view.CurrentVersion.ID)

	// The download chain serves the pinned manifest member and nothing else.
	files := &fakeCraftFiles{blobs: map[string][]byte{"blob://v1-index": []byte("<h1>v1</h1>")}}
	env.svc.files = files
	file, reader, err := env.svc.OpenVersionFile(craftCtx(1, "u1", ws.SessionID), owner, version.ID, "index.html")
	require.NoError(t, err)
	defer reader.Close()
	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, "<h1>v1</h1>", string(content))
	require.Equal(t, "text/html", file.MIME)

	_, _, err = env.svc.OpenVersionFile(craftCtx(1, "u1", ws.SessionID), owner, version.ID, "style.css")
	require.ErrorIs(t, err, craft.ErrNotFound)
	_, _, err = env.svc.OpenVersionFile(craftCtx(1, "u1", ws.SessionID), owner, version.ID, "../escape")
	require.ErrorIs(t, err, craft.ErrInvalidInput)

	// Another session's version is invisible.
	_, err = env.svc.GetVersion(craftCtx(1, "u1", "s-plain"), ownerScope(1, "u1", "s-plain"), version.ID)
	require.ErrorIs(t, err, craft.ErrNotFound)
}

// fakeCraftFiles stores objects under refs.
type fakeCraftFiles struct {
	interfaces.FileService
	blobs map[string][]byte
}

func (f *fakeCraftFiles) GetFile(_ context.Context, ref string) (io.ReadCloser, error) {
	data, ok := f.blobs[ref]
	if !ok {
		return nil, errors.New("object missing")
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}
