package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// openCraftSessionDB opens a clone of the once-migrated SQLite template
// (000045_craft_sessions included) and seeds the tenant/user fixtures.
func openCraftSessionDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := cloneMigratedSQLiteDB(t, "craft-sessions.db")
	require.NoError(t, db.Exec(
		`INSERT INTO tenants (id, name, business) VALUES (1, 'tenant-1', 'test')`,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO users (id, username, email, password_hash, tenant_id)
		 VALUES ('u1', 'u1', 'u1@example.test', 'x', 1), ('u2', 'u2', 'u2@example.test', 'x', 1)`,
	).Error)
	require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id,user_id,role,status,joined_at,created_at,updated_at) VALUES
		(1,'u1','owner','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
		(1,'u2','contributor','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
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
	// Without the T08 ACL assembly, the legacy owner-only fallback cannot use
	// SessionService's tenant-admin read exception to reveal Craft content.
	_, err = env.svc.Get(adminCraftCtx(1, "u2", ws.SessionID), ownerScope(1, "u2", ws.SessionID))
	require.ErrorIs(t, err, craft.ErrNotFound)
	_, err = env.svc.AssociateInput(adminCraftCtx(1, "u2", ws.SessionID),
		ownerScope(1, "u2", ws.SessionID), "doc-1", strings.Repeat("a", 64))
	require.ErrorIs(t, err, craft.ErrNotFound)

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
	require.NotNil(t, input.Recognition, "the legacy association write path must classify new rows")
	require.True(t, input.Recognition.Accepted)
	require.False(t, input.Recognition.Understood, "plain text is not understood without a parser/consumer")

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
	_, _, err = env.svc.StartRun(ctx, ownerScope(1, "u1", ws.SessionID), CraftRunRequest{
		RequestID: "run-1", Prompt: "做一个落地页", InputRefs: []string{input.Ref}, KnowledgeScope: "kb-selected",
	})
	require.ErrorIs(t, err, craft.ErrConflict, "new rows from the legacy route also require explicit recognition decisions")
	require.NoError(t, env.svc.DecideInput(ctx, ownerScope(1, "u1", ws.SessionID), input.Ref, "continue"))

	run, _, err := env.svc.StartRun(ctx, ownerScope(1, "u1", ws.SessionID), CraftRunRequest{
		RequestID: "run-1", Prompt: "做一个落地页", InputRefs: []string{input.Ref}, KnowledgeScope: "kb-selected",
	})
	require.NoError(t, err)
	runSnapshot, err := ParseDurableRunSnapshot(run.Snapshot)
	require.NoError(t, err)
	require.NotNil(t, runSnapshot.CraftInputManifest)
	require.Equal(t, []craft.Input{input}, *runSnapshot.CraftInputManifest)
	require.NotNil(t, runSnapshot.CraftKnowledgeSelection)
	require.Equal(t, "做一个落地页", runSnapshot.CraftKnowledgeSelection.Query, "retrieval uses the original authenticated prompt")
	require.Equal(t, []string{"kb-selected"}, runSnapshot.CraftKnowledgeSelection.KnowledgeBaseIDs)
	require.Contains(t, runSnapshot.Query, "[已授权输入材料]", "server-composed input guidance remains in the model query")
	require.NotContains(t, runSnapshot.Query, `"craft_input_manifest"`)
	require.Equal(t, "queued", run.Status)
	require.Equal(t, ws.SessionID, run.SessionID)

	// The run slot is reserved: a second run is refused with the run id.
	_, _, err = env.svc.StartRun(ctx, ownerScope(1, "u1", ws.SessionID), CraftRunRequest{
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

	second, _, err := env.svc.StartRun(ctx, ownerScope(1, "u1", ws.SessionID), CraftRunRequest{
		RequestID: "run-2", Prompt: "再改一版",
	})
	require.NoError(t, err)
	require.NotEqual(t, run.Key.RunID, second.Key.RunID)

	// Retrying the FIRST key after completion replays the original admission.
	replay, _, err := env.svc.StartRun(ctx, ownerScope(1, "u1", ws.SessionID), CraftRunRequest{
		RequestID: "run-1", Prompt: "做一个落地页", InputRefs: []string{input.Ref}, KnowledgeScope: "kb-selected",
	})
	require.NoError(t, err)
	require.Equal(t, run.Key.RunID, replay.Key.RunID)
}

func TestCraftSessionStartRunFreezesKnowledgeScopeAndActorBoundReplay(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	ws := createCraftSession(t, env, "u1", "knowledge-snapshot", "Knowledge task", "web")
	access := NewCraftAccessService(env.db)
	env.svc.access = access
	owner := ownerScope(1, "u1", ws.SessionID)
	ctx := craftCtx(1, "u1", ws.SessionID)
	require.NoError(t, access.Grant(ctx, owner, "u2", craft.TaskRoleCollaborator))

	_, _, err := env.svc.StartRun(ctx, owner, CraftRunRequest{
		RequestID: "invalid-scope", Prompt: "Build a page", KnowledgeScope: "bad\x00kb",
	})
	require.ErrorIs(t, err, craft.ErrInvalidInput, "malformed selection must be rejected before admission")

	request := CraftRunRequest{RequestID: "selected-scope", Prompt: "Build a page", KnowledgeScope: "kb-selected"}
	run, _, err := env.svc.StartRun(ctx, owner, request)
	require.NoError(t, err)
	require.Equal(t, "u1", run.UserID, "Task owner stays the durable storage identity")
	require.Equal(t, "u1", run.ActorUserID)
	snapshot, err := ParseDurableRunSnapshot(run.Snapshot)
	require.NoError(t, err)
	require.NotNil(t, snapshot.CraftKnowledgeSelection)
	require.Equal(t, "Build a page", snapshot.Query)
	require.Equal(t, "Build a page", snapshot.CraftKnowledgeSelection.Query)
	require.Equal(t, []string{"kb-selected"}, snapshot.CraftKnowledgeSelection.KnowledgeBaseIDs)
	require.NotContains(t, snapshot.Query, "kb-selected", "selection metadata does not contaminate the model prompt")

	// A collaborator with current TaskWrite still cannot take over the Owner's
	// request key, even if that actor asks for another KB.
	collaborator := ownerScope(1, "u2", ws.SessionID)
	_, _, err = env.svc.StartRun(craftCtx(1, "u2", ws.SessionID), collaborator, CraftRunRequest{
		RequestID: request.RequestID, Prompt: request.Prompt, KnowledgeScope: "kb-other",
	})
	require.ErrorIs(t, err, craft.ErrConflict)

	fence, err := env.runs.Store().Claim(ctx, run.Key, "knowledge-snapshot-worker", time.Minute)
	require.NoError(t, err)
	finalizer, ok := env.runs.Store().(interface {
		Finalize(context.Context, agentruntime.Fence, json.RawMessage) error
	})
	require.True(t, ok)
	require.NoError(t, finalizer.Finalize(ctx, fence, json.RawMessage(`{"done":true}`)))

	replay, _, err := env.svc.StartRun(ctx, owner, request)
	require.NoError(t, err)
	require.Equal(t, run.Key.RunID, replay.Key.RunID)
	replayedSnapshot, err := ParseDurableRunSnapshot(replay.Snapshot)
	require.NoError(t, err)
	require.Equal(t, []string{"kb-selected"}, replayedSnapshot.CraftKnowledgeSelection.KnowledgeBaseIDs)

	changed := request
	changed.KnowledgeScope = "kb-other"
	_, _, err = env.svc.StartRun(ctx, owner, changed)
	require.ErrorIs(t, err, craft.ErrConflict, "changing selected KB under the same request key changes the durable snapshot digest")

	empty, _, err := env.svc.StartRun(ctx, owner, CraftRunRequest{
		RequestID: "empty-scope", Prompt: "Build a page without references",
	})
	require.NoError(t, err)
	emptySnapshot, err := ParseDurableRunSnapshot(empty.Snapshot)
	require.NoError(t, err)
	require.NotNil(t, emptySnapshot.CraftKnowledgeSelection)
	require.Equal(t, "Build a page without references", emptySnapshot.CraftKnowledgeSelection.Query)
	require.NotNil(t, emptySnapshot.CraftKnowledgeSelection.KnowledgeBaseIDs)
	require.Empty(t, emptySnapshot.CraftKnowledgeSelection.KnowledgeBaseIDs, "empty knowledge scope is explicit no-selection")
}

func TestCraftSessionStartRunValidatesWorkspaceState(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	ws := createCraftSession(t, env, "u1", "k1", "站点", "web")
	ctx := craftCtx(1, "u1", ws.SessionID)
	owner := ownerScope(1, "u1", ws.SessionID)
	version := publishCraftVersion(t, env, owner, "<h1>published history</h1>")

	// Unknown input refs never reach admission.
	_, _, err := env.svc.StartRun(ctx, ownerScope(1, "u1", ws.SessionID), CraftRunRequest{
		RequestID: "r", Prompt: "go", InputRefs: []string{"tempdocs://not-associated"},
	})
	require.ErrorIs(t, err, craft.ErrInvalidInput)

	// Even an existing historical version is unsupported as an edit seed. The
	// rejection precedes the request claim and Run admission writes.
	var runsBefore, requestsBefore int64
	require.NoError(t, env.db.Table("agent_runs").Count(&runsBefore).Error)
	require.NoError(t, env.db.Table("craft_session_requests").Count(&requestsBefore).Error)
	_, _, err = env.svc.StartRun(ctx, owner, CraftRunRequest{
		RequestID: "historical-edit", Prompt: "go", BaseVersionID: string(version.ID),
	})
	require.ErrorIs(t, err, craft.ErrUnsupported)
	var runsAfter, requestsAfter int64
	require.NoError(t, env.db.Table("agent_runs").Count(&runsAfter).Error)
	require.NoError(t, env.db.Table("craft_session_requests").Count(&requestsAfter).Error)
	require.Equal(t, runsBefore, runsAfter)
	require.Equal(t, requestsBefore, requestsAfter)
	_, _, err = env.svc.StartRun(ctx, owner, CraftRunRequest{
		RequestID: "blank-historical-edit", Prompt: "go", BaseVersionID: " ",
	})
	require.ErrorIs(t, err, craft.ErrUnsupported, "every nonempty client value is rejected, including whitespace")
	require.NoError(t, env.db.Table("agent_runs").Count(&runsAfter).Error)
	require.NoError(t, env.db.Table("craft_session_requests").Count(&requestsAfter).Error)
	require.Equal(t, runsBefore, runsAfter)
	require.Equal(t, requestsBefore, requestsAfter)

	// Oversized prompts are refused before admission.
	_, _, err = env.svc.StartRun(ctx, ownerScope(1, "u1", ws.SessionID), CraftRunRequest{
		RequestID: "r", Prompt: strings.Repeat("长", 32769),
	})
	require.ErrorIs(t, err, craft.ErrInvalidInput)

	// Without a chat model the surface fails closed instead of inventing one.
	env.models.models = nil
	_, _, err = env.svc.StartRun(ctx, ownerScope(1, "u1", ws.SessionID), CraftRunRequest{
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
	var workspaces []struct {
		ID string `gorm:"column:id"`
	}
	require.NoError(t, env.db.Table("craft_workspaces").Select("id").
		Where("tenant_id = ? AND session_id = ? AND owner_id = ?", scope.TenantID, scope.SessionID, scope.UserID).
		Find(&workspaces).Error)
	require.Len(t, workspaces, 1, "version fixture must resolve exactly one persisted owner Workspace")
	workspaceID := workspaces[0].ID
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
