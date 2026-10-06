package container

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/opencode"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestCraftKnowledgeRunViewFinalVerifierRequiresPublishedExactPackage(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *h2KnowledgeRuntimeFixture)
	}{
		{name: "foreign accepted digest", mutate: func(_ *testing.T, fixture *h2KnowledgeRuntimeFixture) {
			fixture.accepted.PackageDigest = strings.Repeat("b", 64)
		}},
		{name: "changed accepted bytes", mutate: func(t *testing.T, fixture *h2KnowledgeRuntimeFixture) {
			packagePath := filepath.Join(fixture.material.root, filepath.FromSlash(craft.KnowledgeRunDir(fixture.run.Key.RunID)))
			manifest := filepath.Join(packagePath, "manifest.json")
			require.NoError(t, os.Chmod(packagePath, 0o755))
			require.NoError(t, os.Chmod(manifest, 0o644))
			require.NoError(t, os.WriteFile(manifest, []byte("changed after resolver"), 0o644))
			require.NoError(t, os.Chmod(manifest, 0o444))
			require.NoError(t, os.Chmod(packagePath, 0o555))
		}},
		{name: "candidate only", mutate: func(t *testing.T, fixture *h2KnowledgeRuntimeFixture) {
			prepareH2CandidateOnly(t, fixture)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newH2KnowledgeRuntimeFixture(t, nil)
			accepted, err := fixture.builder.BuildForRun(context.Background(), fixture.run, fixture.material)
			require.NoError(t, err)
			fixture.accepted = accepted
			searchCount := len(fixture.search.queries)
			tc.mutate(t, fixture)
			err = fixture.builder.VerifyAcceptedForRun(context.Background(), fixture.run, fixture.material, fixture.accepted)
			require.Error(t, err)
			require.Len(t, fixture.search.queries, searchCount, "final verification must not search")
		})
	}
}

func prepareH2Candidate(t *testing.T, fixture *h2KnowledgeRuntimeFixture) (*craftKnowledgeFilesystemPublisher, service.CraftKnowledgeMaterialPackage) {
	t.Helper()
	publisher, err := NewCraftKnowledgePackagePublisher(fixture.material.root, fixture.run.Key.RunID)
	require.NoError(t, err)
	workspace, err := fixture.store.GetWorkspace(context.Background(), fixture.scope)
	require.NoError(t, err)
	pkg, err := publisher.Resume(context.Background(), workspace, fixture.run.Key.RunID, fixture.accepted.PackageDigest)
	require.NoError(t, err)
	candidate := publisher.candidatePath(pkg.RunID, pkg.Digest)
	payload := filepath.Join(candidate, craftKnowledgePublisherPayload)
	require.NoError(t, os.MkdirAll(payload, 0o700))
	seal := craftKnowledgePublisherSeal{
		Version: 1, RunID: pkg.RunID, Directory: pkg.Directory, Digest: pkg.Digest,
		Files: make(map[string]craftKnowledgePublisherSealFile, len(pkg.Files)),
	}
	for relative, content := range pkg.Files {
		name := filepath.Base(relative)
		sum := sha256.Sum256(content)
		seal.Files[name] = craftKnowledgePublisherSealFile{Bytes: len(content), SHA256: hex.EncodeToString(sum[:])}
		require.NoError(t, os.WriteFile(filepath.Join(payload, name), content, 0o600))
	}
	sealBytes, err := json.Marshal(seal)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(candidate, craftKnowledgePublisherSealName), sealBytes, 0o600))
	return publisher, pkg
}

func prepareH2CandidateOnly(t *testing.T, fixture *h2KnowledgeRuntimeFixture) {
	t.Helper()
	publisher, pkg := prepareH2Candidate(t, fixture)
	published := filepath.Join(fixture.material.root, filepath.FromSlash(craft.KnowledgeRunDir(fixture.run.Key.RunID)))
	require.NoError(t, os.Chmod(published, 0o755))
	require.NoError(t, os.RemoveAll(published))
	require.FileExists(t, filepath.Join(publisher.candidatePath(pkg.RunID, pkg.Digest), craftKnowledgePublisherSealName))
	_, err := os.Lstat(published)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestLocalCraftRuntimeRejectsKnowledgeMutationAfterResolver(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *h2KnowledgeRuntimeFixture)
	}{
		{name: "sibling Run", mutate: func(t *testing.T, f *h2KnowledgeRuntimeFixture) {
			runs := filepath.Join(f.material.knowledge, "runs")
			require.NoError(t, os.Mkdir(filepath.Join(runs, "run-a"), 0o555))
		}},
		{name: "unexpected root file", mutate: func(t *testing.T, f *h2KnowledgeRuntimeFixture) {
			require.NoError(t, os.WriteFile(filepath.Join(f.material.knowledge, "extra"), []byte("x"), 0o444))
		}},
		{name: "symlink sibling", mutate: func(t *testing.T, f *h2KnowledgeRuntimeFixture) {
			runs := filepath.Join(f.material.knowledge, "runs")
			require.NoError(t, os.Symlink(filepath.Join(runs, f.run.Key.RunID), filepath.Join(runs, "run-a")))
		}},
		{name: "Run A carried into Run B", mutate: func(t *testing.T, f *h2KnowledgeRuntimeFixture) {
			runs := filepath.Join(f.material.knowledge, "runs")
			require.NoError(t, os.Mkdir(filepath.Join(runs, "run-previous"), 0o555))
		}},
		{name: "missing accepted package", mutate: func(t *testing.T, f *h2KnowledgeRuntimeFixture) {
			published := filepath.Join(f.material.root, filepath.FromSlash(craft.KnowledgeRunDir(f.run.Key.RunID)))
			require.NoError(t, os.Chmod(published, 0o755))
			require.NoError(t, os.RemoveAll(published))
		}},
		{name: "candidate only after resolver", mutate: func(t *testing.T, f *h2KnowledgeRuntimeFixture) {
			prepareH2CandidateOnly(t, f)
		}},
		{name: "changed published bytes after resolver", mutate: func(t *testing.T, f *h2KnowledgeRuntimeFixture) {
			packagePath := filepath.Join(f.material.root, filepath.FromSlash(craft.KnowledgeRunDir(f.run.Key.RunID)))
			manifest := filepath.Join(packagePath, "manifest.json")
			require.NoError(t, os.Chmod(packagePath, 0o755))
			require.NoError(t, os.Chmod(manifest, 0o644))
			original, err := os.ReadFile(manifest)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(manifest, append(original, '\n'), 0o644))
			require.NoError(t, os.Chmod(manifest, 0o444))
			require.NoError(t, os.Chmod(packagePath, 0o555))
			publisher, err := NewCraftKnowledgePackagePublisher(f.material.root, f.run.Key.RunID)
			require.NoError(t, err)
			_, err = os.Lstat(publisher.candidatePath(f.run.Key.RunID, f.accepted.PackageDigest))
			require.ErrorIs(t, err, os.ErrNotExist, "no candidate may mask final published-byte verification")
			_, err = os.Lstat(filepath.Join(packagePath, craftKnowledgePublisherSealName))
			require.ErrorIs(t, err, os.ErrNotExist, "published payload layout has no seal.json")
			require.NoError(t, verifyRunViewKnowledgeRoot(f.material, f.run.Key.RunID), "the mounted package root remains exact")
		}},
		{name: "same Run foreign digest", mutate: func(_ *testing.T, f *h2KnowledgeRuntimeFixture) {
			f.accepted.PackageDigest = strings.Repeat("c", 64)
		}},
		{name: "source drift", mutate: func(_ *testing.T, f *h2KnowledgeRuntimeFixture) {
			engine := f.material.provider.engine.(*fakeCraftRunViewContainerEngine)
			container := engine.containerSnapshot(craftRunViewSpecForGeneration(f.material.generation).ContainerID)
			container.Mounts[0].Source = "/tmp/foreign"
			engine.replaceContainer(container.Name, container)
		}},
		{name: "read write mount", mutate: func(_ *testing.T, f *h2KnowledgeRuntimeFixture) {
			engine := f.material.provider.engine.(*fakeCraftRunViewContainerEngine)
			container := engine.containerSnapshot(craftRunViewSpecForGeneration(f.material.generation).ContainerID)
			container.Mounts[1].ReadOnly = false
			engine.replaceContainer(container.Name, container)
		}},
		{name: "extra mount", mutate: func(_ *testing.T, f *h2KnowledgeRuntimeFixture) {
			engine := f.material.provider.engine.(*fakeCraftRunViewContainerEngine)
			container := engine.containerSnapshot(craftRunViewSpecForGeneration(f.material.generation).ContainerID)
			container.Mounts = append(container.Mounts, CraftRunViewMount{Type: "bind", Source: "/tmp/extra", Destination: "/tmp/extra"})
			engine.replaceContainer(container.Name, container)
		}},
		{name: "recreated container", mutate: func(_ *testing.T, f *h2KnowledgeRuntimeFixture) {
			engine := f.material.provider.engine.(*fakeCraftRunViewContainerEngine)
			container := engine.containerSnapshot(craftRunViewSpecForGeneration(f.material.generation).ContainerID)
			container.ID = "replacement-container"
			engine.replaceContainer(container.Name, container)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newH2KnowledgeRuntimeFixture(t, tc.mutate)
			_, err := fixture.runtime.Execute(context.Background(), fixture.task)
			require.Error(t, err)
			if tc.name == "changed published bytes after resolver" {
				require.ErrorIs(t, err, craft.ErrNotFound, "published digest mismatch must come from Resume at the final verifier")
			}
			require.Zero(t, fixture.spy.calls, "invalid accepted knowledge must not reach inner.Execute")
		})
	}
}

func TestCraftKnowledgePublisherResumeRejectsCorruptCandidateSeal(t *testing.T) {
	fixture := newH2KnowledgeRuntimeFixture(t, nil)
	accepted, err := fixture.builder.BuildForRun(context.Background(), fixture.run, fixture.material)
	require.NoError(t, err)
	fixture.accepted = accepted
	publisher, pkg := prepareH2Candidate(t, fixture)
	sealPath := filepath.Join(publisher.candidatePath(pkg.RunID, pkg.Digest), craftKnowledgePublisherSealName)
	sealBytes, err := os.ReadFile(sealPath)
	require.NoError(t, err)
	var seal craftKnowledgePublisherSeal
	require.NoError(t, json.Unmarshal(sealBytes, &seal))
	seal.Digest = strings.Repeat("d", 64)
	sealBytes, err = json.Marshal(seal)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(sealPath, sealBytes, 0o600))

	published := filepath.Join(fixture.material.root, filepath.FromSlash(craft.KnowledgeRunDir(fixture.run.Key.RunID)))
	require.NoError(t, os.Chmod(published, 0o755))
	require.NoError(t, os.RemoveAll(published))
	workspace, err := fixture.store.GetWorkspace(context.Background(), fixture.scope)
	require.NoError(t, err)
	_, err = publisher.Resume(context.Background(), workspace, fixture.run.Key.RunID, accepted.PackageDigest)
	require.ErrorIs(t, err, craft.ErrConflict, "Resume must parse and reject the corrupt candidate seal")
}

func TestLocalCraftRuntimeAcceptsIdenticalKnowledgeRetryAndRestart(t *testing.T) {
	fixture := newH2KnowledgeRuntimeFixture(t, nil)
	_, err := fixture.runtime.Execute(context.Background(), fixture.task)
	require.NoError(t, err)
	require.Equal(t, 1, fixture.spy.calls)
	firstDigest := fixture.accepted.PackageDigest
	packagePath := filepath.Join(fixture.material.root, filepath.FromSlash(craft.KnowledgeRunDir(fixture.run.Key.RunID)))
	firstManifest, err := os.ReadFile(filepath.Join(packagePath, "manifest.json"))
	require.NoError(t, err)
	spy := &h2FailedCraftExecutor{}
	restarted, restartedProvider, records, restartedSearch, materialAfterRestart := reconstructH2KnowledgeRuntime(t, fixture, spy)
	searchCount := len(fixture.search.queries)
	_, err = restarted.Execute(context.Background(), fixture.task)
	require.NoError(t, err)
	issuedMaterial := materialAfterRestart()
	require.NotNil(t, issuedMaterial)
	require.Same(t, restartedProvider, issuedMaterial.provider, "restart material must be issued by the reconstructed provider")
	require.NotSame(t, fixture.material.provider, issuedMaterial.provider)
	require.Equal(t, 1, spy.calls)
	require.Equal(t, firstDigest, fixture.accepted.PackageDigest, "retry must keep the same accepted Run digest")
	require.Equal(t, searchCount, len(fixture.search.queries), "a reconstructed accepted-record resolver must not search")
	require.Empty(t, restartedSearch.queries, "a new builder used only for final verification must not search")
	require.Zero(t, records.saves, "restart must not save or republish a Run record")
	require.Zero(t, records.publications, "restart must not mark the Run package published again")
	secondManifest, err := os.ReadFile(filepath.Join(packagePath, "manifest.json"))
	require.NoError(t, err)
	require.Equal(t, firstManifest, secondManifest, "restart must use the identical sealed package bytes")
}

func reconstructH2KnowledgeRuntime(t *testing.T, fixture *h2KnowledgeRuntimeFixture, inner craft.Executor) (*localCraftRuntime, *CraftRunViewContainerProvider, *h2ObservedKnowledgeRecords, *craftRunViewKnowledgeSearcher, func() *CraftRunViewMaterialHandle) {
	t.Helper()
	priorProvider := fixture.material.provider
	engine, ok := priorProvider.engine.(*fakeCraftRunViewContainerEngine)
	require.True(t, ok)
	const sessionID = "ses_0123456789ab0123456789ABCD"
	api := &fakeCraftRunViewSessionAPI{}
	provider, err := newCraftRunViewContainerProvider(priorProvider.config, engine, func(_ string, directory, projectID string) (craftRunViewSessionAPI, error) {
		api.sessions = []opencode.SessionInfo{rvTestOpenCodeSession(sessionID, directory, projectID)}
		return api, nil
	})
	require.NoError(t, err)
	require.NotSame(t, priorProvider, provider, "restart must reconstruct the provider and its in-memory binding map")

	records := &h2ObservedKnowledgeRecords{delegate: fixture.records}
	restartedSearch := &craftRunViewKnowledgeSearcher{}
	restartedBuilder, err := NewCraftKnowledgeRunViewBuilder(CraftKnowledgeRunViewConfig{
		Store: fixture.store, Access: fixture.access.Get, Search: restartedSearch.Search,
		TaskAccess: fixture.taskAccess, Records: records,
	})
	require.NoError(t, err)
	var issuedMaterial *CraftRunViewMaterialHandle
	runtime := &localCraftRuntime{
		db: fixture.runtime.db, store: fixture.store, inner: inner,
		workDir: fixture.runtime.workDir, outputDir: fixture.runtime.outputDir,
		sessionsRoot: fixture.runtime.sessionsRoot,
		materialResolver: func(ctx context.Context, _ craft.Task) (CraftRunViewMaterialHandle, error) {
			container, inspectErr := provider.InspectOrCreateContainer(ctx, craftRunViewSpecForGeneration(fixture.material.generation))
			if inspectErr != nil {
				return CraftRunViewMaterialHandle{}, inspectErr
			}
			view := boundMaterialTestView(fixture.material.key, container, fixture.material.generation, sessionID)
			materialStore := &materialTestStore{view: view}
			material, materialErr := provider.MaterialHandle(ctx, materialStore, fixture.material.key,
				CraftRunViewRuntimeHandle{View: view, Directory: container.Directory})
			if materialErr != nil {
				return CraftRunViewMaterialHandle{}, materialErr
			}
			issuedMaterial = &material
			return material, nil
		},
		knowledgeResolver: func(ctx context.Context, _ craft.Task, _ CraftRunViewMaterialHandle) (CraftKnowledgeRunViewAcceptance, error) {
			record, loadErr := records.Load(ctx, fixture.scope, fixture.run.Key.RunID)
			if loadErr != nil || record.PublicationState != craft.KnowledgePublicationPublished {
				return CraftKnowledgeRunViewAcceptance{}, craft.ErrConflict
			}
			return CraftKnowledgeRunViewAcceptance{RunID: record.RunID, PackageDigest: record.PackageDigest}, nil
		},
		knowledgeVerifier: func(ctx context.Context, _ craft.Task, handle CraftRunViewMaterialHandle, accepted CraftKnowledgeRunViewAcceptance) error {
			return restartedBuilder.VerifyAcceptedForRun(ctx, fixture.run, handle, accepted)
		},
	}
	require.NotSame(t, fixture.runtime, runtime, "restart must reconstruct the runtime")
	require.NotSame(t, fixture.builder, restartedBuilder, "restart must reconstruct the knowledge verifier")
	return runtime, provider, records, restartedSearch, func() *CraftRunViewMaterialHandle { return issuedMaterial }
}

type h2ObservedKnowledgeRecords struct {
	delegate     service.CraftKnowledgeRecordStore
	saves        int
	publications int
}

func (r *h2ObservedKnowledgeRecords) Save(ctx context.Context, record craft.KnowledgeRecord) error {
	r.saves++
	return r.delegate.Save(ctx, record)
}
func (r *h2ObservedKnowledgeRecords) Load(ctx context.Context, scope craft.Scope, runID string) (craft.KnowledgeRecord, error) {
	return r.delegate.Load(ctx, scope, runID)
}
func (r *h2ObservedKnowledgeRecords) MarkPublished(ctx context.Context, scope craft.Scope, runID, digest string) error {
	r.publications++
	return r.delegate.MarkPublished(ctx, scope, runID, digest)
}

type h2KnowledgeRuntimeFixture struct {
	runtime    *localCraftRuntime
	task       craft.Task
	material   CraftRunViewMaterialHandle
	run        agentruntime.Run
	accepted   CraftKnowledgeRunViewAcceptance
	builder    *CraftKnowledgeRunViewBuilder
	store      *craftRunViewKnowledgeStore
	scope      craft.Scope
	search     *craftRunViewKnowledgeSearcher
	access     *craftRunViewKnowledgeAccess
	taskAccess *craftRunViewKnowledgeTaskAccess
	records    *craftRunViewKnowledgeRecords
	spy        *h2FailedCraftExecutor
}

func newH2KnowledgeRuntimeFixture(t *testing.T, afterResolve func(*testing.T, *h2KnowledgeRuntimeFixture)) *h2KnowledgeRuntimeFixture {
	t.Helper()
	base := newRunViewInputFixture(t, nil, map[string][]byte{})
	task := base.task
	task.ID = "delegation-h2"
	task.ToolCallID = "call-h2"
	task.Prompt = "build from the accepted sources"
	task.PromptMessageID = "msg_h2-test"
	const epoch = int64(1)
	task.Fence.Owner = task.Scope.UserID
	task.Fence.Epoch = epoch
	base.task = task
	require.NoError(t, base.db.Exec("ALTER TABLE agent_runs ADD COLUMN epoch INTEGER NOT NULL DEFAULT 0").Error)
	require.NoError(t, base.db.Exec("ALTER TABLE agent_runs ADD COLUMN status TEXT NOT NULL DEFAULT 'running'").Error)
	require.NoError(t, base.db.Exec("UPDATE agent_runs SET epoch = ? WHERE tenant_id = ? AND run_id = ?", epoch,
		task.Fence.TenantID, task.Fence.RunID).Error)
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
	scope := craft.Scope{TenantID: task.Scope.TenantID, UserID: task.Scope.UserID, SessionID: task.Scope.SessionID}
	workspace := craft.Workspace{ID: task.WorkspaceID, Scope: scope, OpenCodeSessionID: "ses_0123456789ab0123456789ABCD"}
	store := &craftRunViewKnowledgeStore{ownerScope: scope, workspace: workspace}
	search := &craftRunViewKnowledgeSearcher{}
	access := &craftRunViewKnowledgeAccess{rows: []*types.Knowledge{{ID: "doc-1", KnowledgeBaseID: "kb-1", TenantID: scope.TenantID, Title: "Approved source"}}}
	taskAccess := &craftRunViewKnowledgeTaskAccess{allowed: true}
	records := &craftRunViewKnowledgeRecords{}
	builder, err := NewCraftKnowledgeRunViewBuilder(CraftKnowledgeRunViewConfig{
		Store: store, Access: access.Get, Search: search.Search, TaskAccess: taskAccess, Records: records,
	})
	require.NoError(t, err)
	raw, err := service.BuildDurableCraftRunSnapshotWithKnowledgeSelection(
		"model-composed prompt", nil, "model-1", "", &types.AgentConfig{AllowedTools: []string{"thinking"}}, []craft.Input{},
		service.CraftKnowledgeSelectionSnapshot{Query: "original user query", KnowledgeBaseIDs: []string{"kb-1"}},
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
		_ = filepath.WalkDir(material.root, func(name string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry == nil || entry.Type()&fs.ModeSymlink != 0 {
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
	fixture := &h2KnowledgeRuntimeFixture{runtime: runtime, task: task, material: material, run: run, builder: builder, store: store, scope: scope, search: search, access: access, taskAccess: taskAccess, records: records, spy: spy}
	runtime.knowledgeResolver = func(ctx context.Context, _ craft.Task, handle CraftRunViewMaterialHandle) (CraftKnowledgeRunViewAcceptance, error) {
		accepted, buildErr := builder.BuildForRun(ctx, run, handle)
		if buildErr == nil {
			fixture.accepted = accepted
			if afterResolve != nil {
				afterResolve(t, fixture)
			}
			accepted = fixture.accepted
		}
		return accepted, buildErr
	}
	runtime.knowledgeVerifier = func(ctx context.Context, _ craft.Task, handle CraftRunViewMaterialHandle, accepted CraftKnowledgeRunViewAcceptance) error {
		return builder.VerifyAcceptedForRun(ctx, run, handle, accepted)
	}
	return fixture
}

type h2FailedCraftExecutor struct{ calls int }

func (e *h2FailedCraftExecutor) Execute(context.Context, craft.Task) (craft.Result, error) {
	e.calls++
	return craft.Result{Status: "failed"}, nil
}
func (*h2FailedCraftExecutor) Observe(context.Context, craft.Task) (craft.Observation, error) {
	return craft.Observation{}, nil
}
func (*h2FailedCraftExecutor) Abort(context.Context, craft.Task) error { return nil }
