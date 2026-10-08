package container

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
	"gorm.io/gorm"
)

// joinedWiringKnowledgeReader exposes only the shared-aware batch read the
// production BindCraftKnowledgeAccess port consumes.
type joinedWiringKnowledgeReader struct {
	interfaces.KnowledgeService
	rows []*types.Knowledge
	err  error
}

func (r *joinedWiringKnowledgeReader) GetKnowledgeBatchWithSharedAccess(context.Context, uint64, []string) ([]*types.Knowledge, error) {
	return r.rows, r.err
}

// joinedWiringKnowledgeSearcher exposes only the ACL-guarded per-library
// retrieval the production BindCraftKnowledgeSearch port consumes.
type joinedWiringKnowledgeSearcher struct {
	interfaces.KnowledgeBaseService
	results []([]*types.SearchResult)
	queries []string
	err     error
}

func (s *joinedWiringKnowledgeSearcher) HybridSearch(_ context.Context, kbID string, params types.SearchParams) ([]*types.SearchResult, error) {
	s.queries = append(s.queries, params.QueryText)
	if s.err != nil {
		return nil, s.err
	}
	if len(s.results) > 0 {
		return s.results[0], nil
	}
	return []*types.SearchResult{{ID: "chunk-1", KnowledgeID: "doc-1", KnowledgeBaseID: kbID, Content: "approved excerpt"}}, nil
}

// joinedKnowledgeWiringFixture drives the production wiring path: the runtime's
// knowledgeResolver/knowledgeVerifier come exclusively from
// wireCraftKnowledgeRuntime, records persist through the real SQLite
// repository, and the dispatch recheck runs on a production-shaped
// CraftKnowledgeService. Only the T01-owned materialResolver stays manual —
// its production assembly is a recorded open item outside T05.
type joinedKnowledgeWiringFixture struct {
	runtime    *localCraftRuntime
	task       craft.Task
	material   CraftRunViewMaterialHandle
	run        agentruntime.Run
	store      *craftRunViewKnowledgeStore
	scope      craft.Scope
	reader     *joinedWiringKnowledgeReader
	searcher   *joinedWiringKnowledgeSearcher
	taskAccess *craftRunViewKnowledgeTaskAccess
	records    *repository.CraftKnowledgeRecordRepository
	svc        *service.CraftKnowledgeService
	spy        *h2FailedCraftExecutor
	db         *gorm.DB
}

func newJoinedKnowledgeWiringFixture(t *testing.T, selectedKnowledgeBases []string) *joinedKnowledgeWiringFixture {
	t.Helper()
	base := newRunViewInputFixture(t, nil, map[string][]byte{})
	task := base.task
	task.ID = "delegation-joined"
	task.ToolCallID = "call-joined"
	task.Prompt = "build from the accepted sources"
	task.PromptMessageID = "msg_joined-test"
	const epoch = int64(1)
	task.Fence.Owner = task.Scope.UserID
	task.Fence.Epoch = epoch
	base.task = task
	require.NoError(t, base.db.Exec("ALTER TABLE agent_runs ADD COLUMN epoch INTEGER NOT NULL DEFAULT 0").Error)
	require.NoError(t, base.db.Exec("ALTER TABLE agent_runs ADD COLUMN status TEXT NOT NULL DEFAULT 'running'").Error)
	require.NoError(t, base.db.Exec("ALTER TABLE agent_runs ADD COLUMN actor_user_id TEXT NOT NULL DEFAULT ''").Error)
	require.NoError(t, base.db.Exec("ALTER TABLE agent_runs ADD COLUMN lease_owner TEXT NOT NULL DEFAULT ''").Error)
	require.NoError(t, base.db.Exec("UPDATE agent_runs SET epoch = ?, actor_user_id = ?, lease_owner = ? WHERE tenant_id = ? AND run_id = ?", epoch,
		task.Scope.UserID, task.Fence.Owner, task.Fence.TenantID, task.Fence.RunID).Error)
	require.NoError(t, base.db.Exec(`CREATE TABLE craft_workspace_draft_heads (
		workspace_id TEXT, tenant_id INTEGER, revision INTEGER, state TEXT, source_run_id TEXT, manifest_digest TEXT)`).Error)
	require.NoError(t, base.db.Exec(`CREATE TABLE craft_workspace_draft_origins (
		workspace_id TEXT, tenant_id INTEGER, origin_revision INTEGER, origin_state TEXT)`).Error)
	require.NoError(t, base.db.Exec(`CREATE TABLE craft_workspace_draft_revisions (
		workspace_id TEXT, revision INTEGER, tenant_id INTEGER, source_run_id TEXT, manifest_digest TEXT)`).Error)
	require.NoError(t, base.db.Exec(`CREATE TABLE craft_workspace_draft_files (
		workspace_id TEXT, revision INTEGER, path TEXT, object_ref TEXT, sha256 TEXT, bytes INTEGER, mime TEXT)`).Error)
	require.NoError(t, base.db.Exec(`INSERT INTO craft_workspace_draft_origins(workspace_id, tenant_id, origin_revision, origin_state)
		VALUES (?, ?, 0, 'empty')`, task.WorkspaceID, task.Fence.TenantID).Error)
	require.NoError(t, base.db.Exec(`INSERT INTO craft_workspace_draft_heads(workspace_id, tenant_id, revision, state)
		VALUES (?, ?, 0, 'empty')`, task.WorkspaceID, task.Fence.TenantID).Error)
	require.NoError(t, base.db.Exec(`CREATE TABLE craft_knowledge_records (
		tenant_id BIGINT NOT NULL, session_id VARCHAR(128) NOT NULL, run_id VARCHAR(128) NOT NULL,
		record_json TEXT NOT NULL, digest CHAR(64) NOT NULL, acquired_at DATETIME NOT NULL,
		PRIMARY KEY (tenant_id, session_id, run_id))`).Error)
	scope := craft.Scope{TenantID: task.Scope.TenantID, UserID: task.Scope.UserID, SessionID: task.Scope.SessionID}
	workspace := craft.Workspace{ID: task.WorkspaceID, Scope: scope, OpenCodeSessionID: "ses_0123456789ab0123456789ABCD"}
	store := &craftRunViewKnowledgeStore{ownerScope: scope, workspace: workspace}
	reader := &joinedWiringKnowledgeReader{rows: []*types.Knowledge{{ID: "doc-1", KnowledgeBaseID: "kb-1", TenantID: scope.TenantID, Title: "Approved source"}}}
	searcher := &joinedWiringKnowledgeSearcher{}
	taskAccess := &craftRunViewKnowledgeTaskAccess{allowed: true}
	raw, err := service.BuildDurableCraftRunSnapshotWithKnowledgeSelection(
		"model-composed prompt", nil, "model-1", "", &types.AgentConfig{AllowedTools: []string{"thinking"}}, []craft.Input{},
		service.CraftKnowledgeSelectionSnapshot{Query: "original user query", KnowledgeBaseIDs: selectedKnowledgeBases},
	)
	require.NoError(t, err)
	var snapshot map[string]any
	require.NoError(t, json.Unmarshal(raw, &snapshot))
	snapshot["craft_workspace_seed"] = service.CraftWorkspaceSeedSnapshot{
		WorkspaceID: task.WorkspaceID, State: craft.DraftHeadEmpty, DraftRevision: 0,
	}
	raw, err = json.Marshal(snapshot)
	require.NoError(t, err)
	require.NoError(t, base.db.Exec(`UPDATE agent_runs SET snapshot = ? WHERE tenant_id = ? AND run_id = ?`, string(raw), scope.TenantID, task.Fence.RunID).Error)
	require.NoError(t, base.db.Exec(`CREATE TABLE craft_delegations (
		tenant_id INTEGER, run_id TEXT, tool_call_id TEXT, workspace_id TEXT, task_json TEXT, prompt_message_id TEXT)`).Error)
	taskJSON, err := json.Marshal(task)
	require.NoError(t, err)
	require.NoError(t, base.db.Exec(`INSERT INTO craft_delegations(tenant_id, run_id, tool_call_id, workspace_id, task_json, prompt_message_id)
		VALUES (?, ?, ?, ?, ?, ?)`, task.Fence.TenantID, task.Fence.RunID, task.ToolCallID, task.WorkspaceID, string(taskJSON), task.PromptMessageID).Error)
	run := agentruntime.Run{Key: task.Fence.RunKey, SessionID: scope.SessionID, UserID: scope.UserID, ActorUserID: scope.UserID,
		Owner: task.Fence.Owner, Epoch: task.Fence.Epoch, Snapshot: json.RawMessage(raw)}
	material := base.material
	t.Cleanup(func() {
		// The published package directory is deliberately read-only (0555);
		// restore permissions so t.TempDir cleanup can remove it.
		_ = filepath.WalkDir(material.root, func(name string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil || entry == nil || entry.Type()&os.ModeSymlink != 0 {
				return nil
			}
			mode := os.FileMode(0o600)
			if entry.IsDir() {
				mode = 0o700
			}
			_ = os.Chmod(name, mode)
			return nil
		})
	})
	spy := &h2FailedCraftExecutor{}
	runtime := base.runtime
	runtime.store = store
	runtime.outputDir = "output"
	runtime.sessionsRoot = filepath.Join(runtime.workDir, "ws")
	runtime.inner = spy
	runtime.materialResolver = func(context.Context, craft.Task) (CraftRunViewMaterialHandle, error) { return material, nil }
	records := repository.NewCraftKnowledgeRecordRepository(base.db)
	svc, err := service.NewCraftKnowledgeService(service.CraftKnowledgeConfig{
		Store:  store,
		Access: service.BindCraftKnowledgeAccess(reader),
		Search: service.BindCraftKnowledgeSearch(searcher),
		Writer: func(context.Context, craft.Workspace, string, []byte) error {
			return errors.New("craft: direct workspace knowledge writes are disabled on the dispatch recheck service")
		},
		TaskAccess: taskAccess,
		Records:    records,
	})
	require.NoError(t, err)
	require.NoError(t, wireCraftKnowledgeRuntime(
		runtime, store, reader, searcher,
		repository.NewAgentRunStore(base.db),
		&CraftRunViewProductionAssembly{Provider: material.provider},
		taskAccess, records, svc,
	))
	return &joinedKnowledgeWiringFixture{runtime: runtime, task: task, material: material, run: run, store: store,
		scope: scope, reader: reader, searcher: searcher, taskAccess: taskAccess, records: records, svc: svc, spy: spy, db: base.db}
}

// TestCraftT05WiringJoinedJourneyPublishesRunSourcesBeforeDispatch proves the
// production assembly end to end: one delegation resolves its accepted package
// through the wired builder, the actual sources persist durably through the
// real SQLite record repository, the package is sealed read-only in the Run
// material root, and only then does the delegate prompt dispatch.
func TestCraftT05WiringJoinedJourneyPublishesRunSourcesBeforeDispatch(t *testing.T) {
	fixture := newJoinedKnowledgeWiringFixture(t, []string{"kb-1"})
	require.NotNil(t, fixture.runtime.knowledgeResolver, "production wiring must install the knowledge resolver")
	require.NotNil(t, fixture.runtime.knowledgeVerifier, "production wiring must install the knowledge verifier")

	result, err := fixture.runtime.Execute(context.Background(), fixture.task)
	require.NoError(t, err)
	require.Equal(t, "failed", result.Status, "the spy executor answers failed; the knowledge chain must not fail the delegation")
	require.EqualValues(t, 1, fixture.spy.calls, "exactly one delegate dispatch must happen")

	record, err := fixture.records.Load(context.Background(), fixture.scope, fixture.run.Key.RunID)
	require.NoError(t, err)
	require.Equal(t, craft.KnowledgePublicationPublished, record.PublicationState)
	require.False(t, record.Empty)
	require.Len(t, record.Sources, 1)
	require.Contains(t, record.Sources[0].Ref, "/knowledge/doc-1/", "the persisted ref must name the actually retrieved knowledge doc-1")
	require.Equal(t, fixture.scope.TenantID, record.Sources[0].TenantID)
	require.NotEmpty(t, record.Sources[0].Digest)
	require.False(t, record.Sources[0].AcquiredAt.IsZero())
	require.Positive(t, record.Sources[0].ExcerptBytes)

	published := filepath.Join(fixture.material.root, filepath.FromSlash(craft.KnowledgeRunDir(fixture.run.Key.RunID)))
	info, err := os.Lstat(published)
	require.NoError(t, err)
	require.True(t, info.IsDir())
	require.Equal(t, os.FileMode(0o555), info.Mode().Perm(), "the accepted package must be sealed read-only")

	// The dispatch recheck ran against the Run's persisted actor scope.
	require.NotEmpty(t, fixture.taskAccess.scopes)
	require.Equal(t, fixture.scope, fixture.taskAccess.scopes[len(fixture.taskAccess.scopes)-1])
	// The legacy workDir writer stays unreachable: nothing may stage beside
	// the serve working directory on the RunView path.
	legacy, err := os.Stat(filepath.Join(fixture.runtime.workDir, craft.KnowledgeDir))
	require.True(t, os.IsNotExist(err), "no knowledge/ directory may appear in the legacy serve workDir: %v %v", legacy, err)
}

// TestCraftT05WiringRevokedTaskMembershipBlocksRedispatchWithZeroDispatch
// proves the joined revocation gate: after the actor's TaskWrite grant is
// revoked, a retry of the same delegation is refused and the delegate prompt
// never dispatches again.
func TestCraftT05WiringRevokedTaskMembershipBlocksRedispatchWithZeroDispatch(t *testing.T) {
	fixture := newJoinedKnowledgeWiringFixture(t, []string{"kb-1"})
	_, err := fixture.runtime.Execute(context.Background(), fixture.task)
	require.NoError(t, err)
	require.EqualValues(t, 1, fixture.spy.calls)

	fixture.taskAccess.allowed = false
	_, err = fixture.runtime.Execute(context.Background(), fixture.task)
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.EqualValues(t, 1, fixture.spy.calls, "revoked membership must deny dispatch without a second prompt")
}

// TestCraftT05WiringVerifierRunsDispatchRecheckAfterExactVerification pins the
// production dispatch recheck precisely: the wired knowledgeVerifier calls
// RevalidateForDispatch after the exact final-byte verification. The exact
// verifier never consults TaskAccess, so a Forbidden that flips with the
// grant proves the recheck — not the verifier — denied the dispatch.
func TestCraftT05WiringVerifierRunsDispatchRecheckAfterExactVerification(t *testing.T) {
	fixture := newJoinedKnowledgeWiringFixture(t, []string{"kb-1"})
	_, err := fixture.runtime.Execute(context.Background(), fixture.task)
	require.NoError(t, err)
	record, err := fixture.records.Load(context.Background(), fixture.scope, fixture.run.Key.RunID)
	require.NoError(t, err)
	accepted := CraftKnowledgeRunViewAcceptance{RunID: record.RunID, PackageDigest: record.PackageDigest}
	require.NoError(t, fixture.runtime.knowledgeVerifier(context.Background(), fixture.task, fixture.material, accepted))

	fixture.taskAccess.allowed = false
	require.ErrorIs(t, fixture.runtime.knowledgeVerifier(context.Background(), fixture.task, fixture.material, accepted),
		craft.ErrForbidden, "a revoked current grant must fail the wired dispatch recheck")
	fixture.taskAccess.allowed = true
	require.NoError(t, fixture.runtime.knowledgeVerifier(context.Background(), fixture.task, fixture.material, accepted))
}

// TestCraftT05WiringCrossTenantDelegationCannotResolveForeignRun proves the
// joined tenant boundary: a delegation forged for another tenant cannot load
// the durable Run, so no package is resolved and nothing dispatches.
func TestCraftT05WiringCrossTenantDelegationCannotResolveForeignRun(t *testing.T) {
	fixture := newJoinedKnowledgeWiringFixture(t, []string{"kb-1"})
	foreign := fixture.task
	foreign.Scope.TenantID = 2
	foreign.Fence.TenantID = 2
	_, err := fixture.runtime.Execute(context.Background(), foreign)
	require.Error(t, err)
	require.EqualValues(t, 0, fixture.spy.calls)
	record, loadErr := fixture.records.Load(context.Background(), fixture.scope, fixture.run.Key.RunID)
	require.ErrorIs(t, loadErr, craft.ErrNotFound, "no record may exist for the foreign tenant attempt: %v", record)
}

// TestCraftT05WiringEmptySelectionDisclosedOnDurableRecord proves the joined
// empty-selection contract: an explicit empty KB selection builds an accepted
// empty package whose durable record discloses empty=true with zero sources.
func TestCraftT05WiringEmptySelectionDisclosedOnDurableRecord(t *testing.T) {
	fixture := newJoinedKnowledgeWiringFixture(t, []string{})
	result, err := fixture.runtime.Execute(context.Background(), fixture.task)
	require.NoError(t, err)
	require.Equal(t, "failed", result.Status)
	require.EqualValues(t, 1, fixture.spy.calls, "an explicit empty selection is an accepted Run package and still dispatches")

	record, err := fixture.records.Load(context.Background(), fixture.scope, fixture.run.Key.RunID)
	require.NoError(t, err)
	require.Equal(t, craft.KnowledgePublicationPublished, record.PublicationState)
	require.True(t, record.Empty)
	require.Empty(t, record.Sources)
	require.Empty(t, fixture.searcher.queries, "no retrieval ACL/search port may be entered for an empty selection")
}

// TestCraftT05WiringTruncatedRetrievalDisclosedOnDurableRecord proves the
// joined truncation contract: when retrieval exceeds the hard source cap the
// accepted package is truncated and the durable record discloses it.
func TestCraftT05WiringTruncatedRetrievalDisclosedOnDurableRecord(t *testing.T) {
	fixture := newJoinedKnowledgeWiringFixture(t, []string{"kb-1"})
	const extra = 5
	oversized := make([]*types.SearchResult, 0, craft.MaxKnowledgeSources+extra)
	for i := 0; i < craft.MaxKnowledgeSources+extra; i++ {
		suffix := fmt.Sprintf("%02d", i)
		fixture.reader.rows = append(fixture.reader.rows, &types.Knowledge{ID: "doc-trunc-" + suffix, KnowledgeBaseID: "kb-1", TenantID: fixture.scope.TenantID})
		oversized = append(oversized, &types.SearchResult{ID: "chunk-trunc-" + suffix, KnowledgeID: "doc-trunc-" + suffix, KnowledgeBaseID: "kb-1", Content: "approved excerpt"})
	}
	fixture.searcher.results = [][]*types.SearchResult{oversized}

	result, err := fixture.runtime.Execute(context.Background(), fixture.task)
	require.NoError(t, err)
	require.Equal(t, "failed", result.Status)

	record, err := fixture.records.Load(context.Background(), fixture.scope, fixture.run.Key.RunID)
	require.NoError(t, err)
	require.True(t, record.Truncated, "clipping at the hard cap must be disclosed on the durable record")
	require.Len(t, record.Sources, craft.MaxKnowledgeSources)
}

// TestCraftT05WiringFailsClosedWithoutRequiredDependencies pins the wiring's
// fail-closed contract: an unavailable executor or RunView assembly leaves the
// runtime unresolved (every delegation refused), and missing authority ports
// fail application assembly.
func TestCraftT05WiringFailsClosedWithoutRequiredDependencies(t *testing.T) {
	unavailable := service.NewUnavailableCraftExecutor("dial is not assembled")
	require.NoError(t, wireCraftKnowledgeRuntime(unavailable, nil, nil, nil, nil, nil, nil, nil, nil),
		"without a local runtime dial there is no delegation to gate; wiring is a no-op")

	base := newRunViewInputFixture(t, nil, map[string][]byte{})
	require.NoError(t, wireCraftKnowledgeRuntime(base.runtime, nil, nil, nil, nil,
		&CraftRunViewProductionAssembly{Unavailable: "RunView production pins are incomplete"},
		&craftRunViewKnowledgeTaskAccess{allowed: true}, repository.NewCraftKnowledgeRecordRepository(base.db), &service.CraftKnowledgeService{}),
		"an unavailable RunView assembly is a recorded deployment state, not an error")
	require.Nil(t, base.runtime.knowledgeResolver, "the knowledge resolver must stay unresolved without the RunView assembly")
	require.Nil(t, base.runtime.knowledgeVerifier)

	require.Error(t, wireCraftKnowledgeRuntime(base.runtime, nil, nil, nil, nil, nil,
		nil, repository.NewCraftKnowledgeRecordRepository(base.db), &service.CraftKnowledgeService{}),
		"a missing Task access authority must fail assembly")
	require.Error(t, wireCraftKnowledgeRuntime(base.runtime, nil, nil, nil, nil, nil,
		&craftRunViewKnowledgeTaskAccess{allowed: true}, nil, &service.CraftKnowledgeService{}),
		"a missing durable record store must fail assembly")
	require.Error(t, wireCraftKnowledgeRuntime(base.runtime, nil, nil, nil, nil, nil,
		&craftRunViewKnowledgeTaskAccess{allowed: true}, repository.NewCraftKnowledgeRecordRepository(base.db), nil),
		"a missing dispatch recheck service must fail assembly")
}

// TestCraftT05RegisterKnowledgeFeatureMountsReadSurface proves the HTTP read
// surface is registered through the constrained feature registry: routes exist
// on the session group, a nil service (no runtime dial) answers 503
// explicitly, and missing routes fail application assembly closed.
func TestCraftT05RegisterKnowledgeFeatureMountsReadSurface(t *testing.T) {
	gin.SetMode(gin.TestMode)

	features := session.NewCraftFeatureRoutes()
	require.NoError(t, registerCraftKnowledgeFeature(nil, features))
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), types.UserIDContextKey, "owner"))
	})
	group := r.Group("/api/v1/sessions")
	require.NoError(t, features.Mount(group))

	paths := map[string]bool{}
	for _, route := range r.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	require.True(t, paths["GET /api/v1/sessions/:id/craft/runs/:run_id/sources"])
	require.True(t, paths["GET /api/v1/sessions/:id/craft/runs/:run_id/sources/:citation_id/open"])

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/s-joined/craft/runs/run-1/sources", nil)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, req)
	require.Equal(t, http.StatusServiceUnavailable, response.Code,
		"a nil service is the recorded fail-closed state and must answer 503, not a silent 404")

	missingRoutes := dig.New()
	require.NoError(t, missingRoutes.Provide(func() *service.CraftKnowledgeService { return nil }))
	require.Error(t, missingRoutes.Invoke(registerCraftKnowledgeFeature))
}
