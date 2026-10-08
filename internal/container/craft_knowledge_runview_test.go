package container

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/opencode"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

type craftRunViewKnowledgeStore struct {
	craft.Store
	ownerScope craft.Scope
	workspace  craft.Workspace
	seenScope  craft.Scope
}

func (s *craftRunViewKnowledgeStore) GetWorkspace(_ context.Context, scope craft.Scope) (craft.Workspace, error) {
	s.seenScope = scope
	if scope != s.ownerScope {
		return craft.Workspace{}, craft.ErrForbidden
	}
	return s.workspace, nil
}

type craftRunViewKnowledgeAccess struct {
	rows []*types.Knowledge
	err  error
}

func (a *craftRunViewKnowledgeAccess) Get(_ context.Context, _ uint64, _ []string) ([]*types.Knowledge, error) {
	return a.rows, a.err
}

type craftRunViewKnowledgeSearcher struct {
	queries []string
	kbs     [][]string
	err     error
}

func (s *craftRunViewKnowledgeSearcher) Search(_ context.Context, kbID string, params types.SearchParams) ([]*types.SearchResult, error) {
	s.queries = append(s.queries, params.QueryText)
	s.kbs = append(s.kbs, append([]string(nil), params.KnowledgeBaseIDs...))
	if s.err != nil {
		return nil, s.err
	}
	return []*types.SearchResult{{ID: "chunk-1", KnowledgeID: "doc-1", KnowledgeBaseID: kbID, Content: "approved excerpt"}}, nil
}

type craftRunViewKnowledgeTaskAccess struct {
	allowed bool
	scopes  []craft.Scope
}

func (a *craftRunViewKnowledgeTaskAccess) CheckTaskAccess(_ context.Context, scope craft.Scope, action craft.TaskAction) error {
	a.scopes = append(a.scopes, scope)
	if !a.allowed || action != craft.TaskWrite {
		return craft.ErrForbidden
	}
	return nil
}

type craftRunViewKnowledgeRecords struct {
	byRun map[string]craft.KnowledgeRecord
}

func (r *craftRunViewKnowledgeRecords) Save(_ context.Context, record craft.KnowledgeRecord) error {
	if r.byRun == nil {
		r.byRun = map[string]craft.KnowledgeRecord{}
	}
	if old, ok := r.byRun[record.RunID]; ok {
		if reflect.DeepEqual(old, record) {
			return nil
		}
		return craft.ErrConflict
	}
	r.byRun[record.RunID] = record
	return nil
}

func (r *craftRunViewKnowledgeRecords) Load(_ context.Context, scope craft.Scope, runID string) (craft.KnowledgeRecord, error) {
	record, ok := r.byRun[runID]
	if !ok {
		return craft.KnowledgeRecord{}, craft.ErrNotFound
	}
	if record.Scope.TenantID != scope.TenantID || record.Scope.UserID != scope.UserID || record.Scope.SessionID != scope.SessionID {
		return craft.KnowledgeRecord{}, craft.ErrForbidden
	}
	return record, nil
}

func (r *craftRunViewKnowledgeRecords) MarkPublished(_ context.Context, scope craft.Scope, runID, digest string) error {
	record, ok := r.byRun[runID]
	if !ok || record.Scope != scope || record.PackageDigest != digest {
		return craft.ErrConflict
	}
	record.PublicationState = craft.KnowledgePublicationPublished
	r.byRun[runID] = record
	return nil
}

func newCraftRunViewKnowledgeTestBuilder(t *testing.T, selected []string, owner, actor, runID string) (*CraftKnowledgeRunViewBuilder, agentruntime.Run, CraftRunViewMaterialHandle, *CraftRunViewContainerProvider, *craftRunViewKnowledgeStore, *craftRunViewKnowledgeAccess, *craftRunViewKnowledgeSearcher, *craftRunViewKnowledgeTaskAccess) {
	t.Helper()
	engine := newFakeCraftRunViewContainerEngine()
	provider, api := newRVTestProviderWithSessionAPI(t, engine)
	generation := "generation-" + runID
	runtimeContainer, err := provider.InspectOrCreateContainer(context.Background(), craftRunViewSpecForGeneration(generation))
	require.NoError(t, err)
	root := provider.config.SandboxRoot
	sessionID := "ses_0123456789ab0123456789ABCD"
	api.sessions = []opencode.SessionInfo{rvTestOpenCodeSession(sessionID, runtimeContainer.Directory, runtimeContainer.ProjectID)}
	t.Cleanup(func() {
		_ = filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry == nil {
				return nil
			}
			mode := os.FileMode(0o700)
			if !entry.IsDir() {
				mode = 0o600
			}
			_ = os.Chmod(name, mode)
			return nil
		})
	})
	if err := os.MkdirAll(filepath.Join(root, "knowledge"), 0o700); err != nil {
		t.Fatal(err)
	}
	ownerScope := craft.Scope{TenantID: 7, UserID: owner, SessionID: "task-1"}
	store := &craftRunViewKnowledgeStore{ownerScope: ownerScope, workspace: craft.Workspace{ID: "workspace-1", Scope: ownerScope}}
	search := &craftRunViewKnowledgeSearcher{}
	access := &craftRunViewKnowledgeAccess{rows: []*types.Knowledge{{ID: "doc-1", KnowledgeBaseID: "kb-1", TenantID: 7, Title: "Approved source"}}}
	taskAccess := &craftRunViewKnowledgeTaskAccess{allowed: true}
	records := &craftRunViewKnowledgeRecords{}
	serviceCfg := CraftKnowledgeRunViewConfig{
		Store: store, Access: access.Get, Search: search.Search, TaskAccess: taskAccess, Records: records,
	}
	builder, err := NewCraftKnowledgeRunViewBuilder(serviceCfg)
	require.NoError(t, err)
	config := &types.AgentConfig{AllowedTools: []string{"thinking"}}
	raw, err := service.BuildDurableCraftRunSnapshotWithKnowledgeSelection(
		"model-composed prompt", nil, "model-1", "", config, []craft.Input{},
		service.CraftKnowledgeSelectionSnapshot{Query: "original user query", KnowledgeBaseIDs: selected},
	)
	require.NoError(t, err)
	run := agentruntime.Run{
		Key: agentruntime.RunKey{TenantID: 7, RunID: runID}, SessionID: "task-1", UserID: owner,
		ActorUserID: actor, Snapshot: json.RawMessage(raw),
	}
	key := craft.RunViewKey{TenantID: 7, OwnerID: owner, SessionID: "task-1", RunID: runID}
	view := boundMaterialTestView(key, runtimeContainer, generation, sessionID)
	material, err := provider.MaterialHandle(context.Background(), &materialTestStore{view: view}, key,
		CraftRunViewRuntimeHandle{View: view, Directory: runtimeContainer.Directory})
	require.NoError(t, err)
	return builder, run, material, provider, store, access, search, taskAccess
}

func TestCraftKnowledgeRunViewBuilderUsesDurableSelectionAndPublishesAcceptedPackage(t *testing.T) {
	builder, run, handle, _, store, _, search, taskAccess := newCraftRunViewKnowledgeTestBuilder(t, []string{"kb-1"}, "owner-1", "collaborator-1", "run-1")
	ctx := types.WithCaller(context.Background(), types.Caller{TenantID: 7, UserID: "collaborator-1"})

	accepted, err := builder.BuildForRun(ctx, run, handle)
	require.NoError(t, err)
	require.Equal(t, "run-1", accepted.RunID)
	require.Len(t, accepted.PackageDigest, 64)
	require.Equal(t, []string{"original user query"}, search.queries)
	require.Equal(t, [][]string{{"kb-1"}}, search.kbs)
	require.Equal(t, craft.Scope{TenantID: 7, UserID: "owner-1", SessionID: "task-1"}, store.seenScope)
	require.Equal(t, []craft.Scope{{TenantID: 7, UserID: "collaborator-1", SessionID: "task-1"}}, taskAccess.scopes)
	manifest, err := os.ReadFile(filepath.Join(handle.root, filepath.FromSlash(craft.KnowledgeRunDir(run.Key.RunID)), "manifest.json"))
	require.NoError(t, err)
	require.Contains(t, string(manifest), `"query": "original user query"`)
	require.Contains(t, string(manifest), `"user_id": "collaborator-1"`)
}

func TestCraftKnowledgeRunViewBuilderPublishesExplicitEmptySelection(t *testing.T) {
	builder, run, handle, _, _, _, search, _ := newCraftRunViewKnowledgeTestBuilder(t, []string{}, "owner-1", "owner-1", "run-empty")
	ctx := types.WithCaller(context.Background(), types.Caller{TenantID: 7, UserID: "owner-1"})

	accepted, err := builder.BuildForRun(ctx, run, handle)
	require.NoError(t, err)
	require.Len(t, accepted.PackageDigest, 64)
	require.Empty(t, search.queries, "an explicit empty selection must not search all knowledge bases")
	manifest, err := os.ReadFile(filepath.Join(handle.root, filepath.FromSlash(craft.KnowledgeRunDir(run.Key.RunID)), "manifest.json"))
	require.NoError(t, err)
	require.Contains(t, string(manifest), `"empty": true`)
}

func TestCraftKnowledgeRunViewBuilderRejectsMismatchedVerifiedHandleFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *CraftRunViewContainerProvider, *CraftRunViewMaterialHandle)
	}{
		{name: "wrong generation", mutate: func(_ *testing.T, _ *CraftRunViewContainerProvider, handle *CraftRunViewMaterialHandle) {
			handle.generation = "generation-foreign"
		}},
		{name: "wrong container directory", mutate: func(_ *testing.T, _ *CraftRunViewContainerProvider, handle *CraftRunViewMaterialHandle) {
			handle.directory = "/workspace/foreign"
		}},
		{name: "wrong generation root", mutate: func(t *testing.T, _ *CraftRunViewContainerProvider, handle *CraftRunViewMaterialHandle) {
			root := handle.root + "-foreign"
			require.NoError(t, os.MkdirAll(filepath.Join(root, "knowledge"), 0o700))
			handle.root = root
			handle.knowledge = filepath.Join(root, "knowledge")
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			builder, run, handle, provider, _, _, search, _ := newCraftRunViewKnowledgeTestBuilder(t, []string{"kb-1"}, "owner-1", "owner-1", "run-mismatched-handle")
			tc.mutate(t, provider, &handle)
			ctx := types.WithCaller(context.Background(), types.Caller{TenantID: 7, UserID: "owner-1"})

			_, err := builder.BuildForRun(ctx, run, handle)
			require.ErrorIs(t, err, craft.ErrForbidden)
			require.Empty(t, search.queries, "handle identity must be checked before retrieval or publication")
		})
	}
}

func TestCraftKnowledgeRunViewBuilderRejectsRootRecreatedAfterHandleIssuance(t *testing.T) {
	builder, run, handle, provider, _, _, search, _ := newCraftRunViewKnowledgeTestBuilder(t, []string{"kb-1"}, "owner-1", "owner-1", "run-replaced-root")
	spec := craftRunViewSpecForGeneration(handle.generation)
	layout := generationLayoutPaths(provider.config.SandboxRoot, spec)
	require.NoError(t, os.Rename(layout.root, layout.root+".old"))
	_, err := provider.prepareGenerationLayout(spec)
	require.NoError(t, err, "the replacement has a valid new layout identity for the same path and generation")
	ctx := types.WithCaller(context.Background(), types.Caller{TenantID: 7, UserID: "owner-1"})

	_, err = builder.BuildForRun(ctx, run, handle)
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Empty(t, search.queries, "a replaced generation root must fail before retrieval")
	_, err = os.Lstat(filepath.Join(layout.root, craftRunViewLayoutIdentityFile))
	require.NoError(t, err, "the test replacement retains a valid identity manifest")
}

func TestCraftKnowledgeRunViewBuilderRejectsForeignRunHandleBeforeSearch(t *testing.T) {
	builder, run, handle, _, _, _, search, _ := newCraftRunViewKnowledgeTestBuilder(t, []string{"kb-1"}, "owner-1", "owner-1", "run-a")
	handle.key.RunID = "run-b"
	ctx := types.WithCaller(context.Background(), types.Caller{TenantID: 7, UserID: "owner-1"})

	_, err := builder.BuildForRun(ctx, run, handle)
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Empty(t, search.queries)
}

func TestCraftKnowledgeRunViewBuilderReplayConflictsOnChangedSnapshotAndDoesNotSearch(t *testing.T) {
	builder, run, handle, _, _, _, search, _ := newCraftRunViewKnowledgeTestBuilder(t, []string{"kb-1"}, "owner-1", "owner-1", "run-replay")
	ctx := types.WithCaller(context.Background(), types.Caller{TenantID: 7, UserID: "owner-1"})
	_, err := builder.BuildForRun(ctx, run, handle)
	require.NoError(t, err)
	searchCount := len(search.queries)

	changed, err := service.BuildDurableCraftRunSnapshotWithKnowledgeSelection(
		"model-composed prompt", nil, "model-1", "", &types.AgentConfig{AllowedTools: []string{"thinking"}}, []craft.Input{},
		service.CraftKnowledgeSelectionSnapshot{Query: "changed query", KnowledgeBaseIDs: []string{"kb-1"}},
	)
	require.NoError(t, err)
	run.Snapshot = changed
	_, err = builder.BuildForRun(ctx, run, handle)
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Len(t, search.queries, searchCount)
	changedSelection, err := service.BuildDurableCraftRunSnapshotWithKnowledgeSelection(
		"model-composed prompt", nil, "model-1", "", &types.AgentConfig{AllowedTools: []string{"thinking"}}, []craft.Input{},
		service.CraftKnowledgeSelectionSnapshot{Query: "original user query", KnowledgeBaseIDs: []string{"kb-2"}},
	)
	require.NoError(t, err)
	run.Snapshot = changedSelection
	_, err = builder.BuildForRun(ctx, run, handle)
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Len(t, search.queries, searchCount)
}

func TestCraftKnowledgeRunViewBuilderRequiresCurrentTaskAuthority(t *testing.T) {
	builder, run, handle, _, _, _, search, taskAccess := newCraftRunViewKnowledgeTestBuilder(t, []string{"kb-1"}, "owner-1", "collaborator-1", "run-denied")
	taskAccess.allowed = false
	ctx := types.WithCaller(context.Background(), types.Caller{TenantID: 7, UserID: "collaborator-1"})
	_, err := builder.BuildForRun(ctx, run, handle)
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Empty(t, search.queries)
}

func TestCraftKnowledgeRunViewBuilderDeniesRevokedSelectedKnowledgeOnReplay(t *testing.T) {
	builder, run, handle, _, _, access, search, _ := newCraftRunViewKnowledgeTestBuilder(t, []string{"kb-1"}, "owner-1", "owner-1", "run-revoked-doc")
	ctx := types.WithCaller(context.Background(), types.Caller{TenantID: 7, UserID: "owner-1"})
	accepted, err := builder.BuildForRun(ctx, run, handle)
	require.NoError(t, err)
	searchCount := len(search.queries)
	manifestBefore, err := os.ReadFile(filepath.Join(handle.root, filepath.FromSlash(craft.KnowledgeRunDir(run.Key.RunID)), "manifest.json"))
	require.NoError(t, err)

	access.rows = nil
	_, err = builder.BuildForRun(ctx, run, handle)
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Equal(t, searchCount, len(search.queries), "document revocation must fail during source ACL resolution")
	manifest, err := os.ReadFile(filepath.Join(handle.root, filepath.FromSlash(craft.KnowledgeRunDir(run.Key.RunID)), "manifest.json"))
	require.NoError(t, err)
	require.Equal(t, manifestBefore, manifest, "revoked replay must preserve the previously accepted package")
	require.NotEmpty(t, accepted.PackageDigest)
}

func TestCraftKnowledgeRunViewBuilderDeniesRevokedSelectedKB(t *testing.T) {
	builder, run, handle, _, _, _, search, _ := newCraftRunViewKnowledgeTestBuilder(t, []string{"kb-1"}, "owner-1", "owner-1", "run-revoked-kb")
	search.err = craft.ErrForbidden
	ctx := types.WithCaller(context.Background(), types.Caller{TenantID: 7, UserID: "owner-1"})

	_, err := builder.BuildForRun(ctx, run, handle)
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Equal(t, []string{"original user query"}, search.queries)
	_, statErr := os.Stat(filepath.Join(handle.root, filepath.FromSlash(craft.KnowledgeRunDir(run.Key.RunID))))
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestCraftKnowledgeRunViewBuilderRejectsConflictingCaller(t *testing.T) {
	builder, run, handle, _, _, _, search, _ := newCraftRunViewKnowledgeTestBuilder(t, []string{"kb-1"}, "owner-1", "collaborator-1", "run-caller")
	ctx := types.WithCaller(context.Background(), types.Caller{TenantID: 7, UserID: "intruder"})
	_, err := builder.BuildForRun(ctx, run, handle)
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Empty(t, search.queries)
}
