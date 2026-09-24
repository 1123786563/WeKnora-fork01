package container

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
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
			publisher, err := NewCraftKnowledgePackagePublisher(fixture.material.root, fixture.run.Key.RunID)
			require.NoError(t, err)
			workspace, err := fixture.store.GetWorkspace(context.Background(), fixture.scope)
			require.NoError(t, err)
			pkg, err := publisher.Resume(context.Background(), workspace, fixture.run.Key.RunID, fixture.accepted.PackageDigest)
			require.NoError(t, err)
			require.NoError(t, publisher.Prepare(context.Background(), workspace, pkg))
			published := filepath.Join(fixture.material.root, filepath.FromSlash(craft.KnowledgeRunDir(fixture.run.Key.RunID)))
			require.NoError(t, os.Chmod(published, 0o755))
			require.NoError(t, os.RemoveAll(published))
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
			require.Zero(t, fixture.spy.calls, "invalid accepted knowledge must not reach inner.Execute")
		})
	}
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

	restarted := *fixture.runtime
	spy := &h2FailedCraftExecutor{}
	restarted.inner = spy
	_, err = restarted.Execute(context.Background(), fixture.task)
	require.NoError(t, err)
	require.Equal(t, 1, spy.calls)
	require.Equal(t, firstDigest, fixture.accepted.PackageDigest, "retry must keep the same accepted Run digest")
	require.Equal(t, 2, len(fixture.search.queries), "the existing H1 replay path rechecks selected KB authority, but discards its result bytes")
	secondManifest, err := os.ReadFile(filepath.Join(packagePath, "manifest.json"))
	require.NoError(t, err)
	require.Equal(t, firstManifest, secondManifest, "restart must use the identical sealed package bytes")
}

type h2KnowledgeRuntimeFixture struct {
	runtime  *localCraftRuntime
	task     craft.Task
	material CraftRunViewMaterialHandle
	run      agentruntime.Run
	accepted CraftKnowledgeRunViewAcceptance
	builder  *CraftKnowledgeRunViewBuilder
	store    *craftRunViewKnowledgeStore
	scope    craft.Scope
	search   *craftRunViewKnowledgeSearcher
	spy      *h2FailedCraftExecutor
}

func newH2KnowledgeRuntimeFixture(t *testing.T, afterResolve func(*testing.T, *h2KnowledgeRuntimeFixture)) *h2KnowledgeRuntimeFixture {
	t.Helper()
	base := newRunViewInputFixture(t, nil, map[string][]byte{})
	task := base.task
	task.Prompt = "build from the accepted sources"
	task.PromptMessageID = "msg_h2-test"
	base.task = task
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
	require.NoError(t, base.db.Exec(`UPDATE agent_runs SET snapshot = ? WHERE tenant_id = ? AND run_id = ?`, string(raw), scope.TenantID, task.Fence.RunID).Error)
	run := agentruntime.Run{Key: task.Fence.RunKey, SessionID: scope.SessionID, UserID: scope.UserID, ActorUserID: scope.UserID, Snapshot: json.RawMessage(raw)}
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
	fixture := &h2KnowledgeRuntimeFixture{runtime: runtime, task: task, material: material, run: run, builder: builder, store: store, scope: scope, search: search, spy: spy}
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
