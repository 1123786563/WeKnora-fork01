package container

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/opencode"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestCraftRunViewProductionAssemblyDefaultsOffWithoutRegistryDigest(t *testing.T) {
	t.Setenv("CRAFT_RUNVIEW_SANDBOX_ROOT", t.TempDir())
	t.Setenv("CRAFT_RUNVIEW_IMAGE_REFERENCE", "registry.example/craft/opencode:1.18.4")
	t.Setenv("CRAFT_RUNVIEW_IMAGE_DIGEST", "")
	t.Setenv("CRAFT_RUNVIEW_RUNTIME_CONFIG_SHA256", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	t.Setenv("CRAFT_RUNVIEW_PROJECT_ID", "craft-project")
	t.Setenv("CRAFT_RUNVIEW_DOCKER_ENDPOINT", "unix:///var/run/docker.sock")

	config, endpoint, complete := craftRunViewProductionConfigFromEnv()
	require.False(t, complete, "a local or tagged image must never fill the absent registry pin")
	require.Equal(t, "unix:///var/run/docker.sock", endpoint)
	require.Empty(t, config.ImageDigest)

	assembly := provideCraftRunViewProductionAssembly(nil, nil)
	require.Nil(t, assembly.Provider)
	require.Nil(t, assembly.ResolveMaterial)
	require.NotEmpty(t, assembly.Unavailable)
}

func TestCraftRunViewProductionAssemblyResolvesOnlyPersistedBoundTaskRun(t *testing.T) {
	provider, engine, api, store, _, _ := newMaterialHandleFixture(t, "assembly-generation")
	reader := &craftRunViewAssemblyRunReader{run: validCraftRunViewAssemblyRun(t, store.view.Key)}
	// The fake engine cannot reconstruct a read-only probe observation, so the
	// pinned runtime probe is claimed and sent once through the effect chain.
	authority := &admittedRuntimeAuthorityFake{view: store.view,
		maySend: map[craft.RunViewEffectKind]bool{craft.RunViewEffectDockerProbe: true}}
	assembly, err := assembleCraftRunViewProductionWithAPI(provider.config, store, authority, reader, engine,
		func(string, string, string) (craftRunViewSessionAPI, error) { return api, nil })
	require.NoError(t, err)
	require.NotNil(t, assembly.Provider)
	require.NotNil(t, assembly.ResolveMaterial)

	task := validCraftRunViewAssemblyTask(t)
	material, err := assembly.ResolveMaterial(craftRunViewAssemblyActorContext(), task)
	require.NoError(t, err)
	require.Same(t, assembly.Provider, material.provider)
	require.Equal(t, "assembly-generation", material.generation)
	require.NoError(t, assembly.Provider.verifyMaterialHandle(material))

	task.Scope.UserID = "foreign-owner"
	_, err = assembly.ResolveMaterial(craftRunViewAssemblyActorContext(), task)
	require.ErrorIs(t, err, craft.ErrConflict)
}

// TestCraftRunViewProductionAssemblyRejectsPersistedBindingThatDiffersFromEvidence
// keeps the fail-closed half of the old unbound-view rejection: an allocating
// generation with complete verified evidence now completes its binding through
// the admitted effect chain, so the durable rejection is a persisted runtime
// binding that differs from the observed container/session identity.
func TestCraftRunViewProductionAssemblyRejectsPersistedBindingThatDiffersFromEvidence(t *testing.T) {
	provider, engine, api, store, _, _ := newMaterialHandleFixture(t, "assembly-unbound-generation")
	store.view.Runtime = craft.RunViewRuntime{RuntimeID: "rv-runtime-foreign", ContainerID: "rv-container-foreign", OpenCodeSessionID: store.view.Runtime.OpenCodeSessionID}
	reader := &craftRunViewAssemblyRunReader{run: validCraftRunViewAssemblyRun(t, store.view.Key)}
	assembly, err := assembleCraftRunViewProductionWithAPI(provider.config, store,
		&admittedRuntimeAuthorityFake{view: store.view}, reader, engine,
		func(string, string, string) (craftRunViewSessionAPI, error) { return api, nil })
	require.NoError(t, err)

	task := validCraftRunViewAssemblyTask(t)
	_, err = assembly.ResolveMaterial(craftRunViewAssemblyActorContext(), task)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
}

func TestCraftRunViewProductionAssemblyRejectsStaleTaskBeforeSideEffects(t *testing.T) {
	tests := []struct {
		name         string
		mutate       func(*craft.Task, *agentruntime.Run)
		callerUserID string
	}{
		{name: "scope and fence tenant mismatch", mutate: func(task *craft.Task, _ *agentruntime.Run) { task.Scope.TenantID++ }},
		{name: "fence tenant differs from scope", mutate: func(task *craft.Task, _ *agentruntime.Run) { task.Fence.TenantID++ }},
		{name: "changed durable owner", mutate: func(_ *craft.Task, run *agentruntime.Run) { run.UserID = "different-owner" }},
		{name: "zero writer epoch", mutate: func(task *craft.Task, _ *agentruntime.Run) { task.Fence.Epoch = 0 }},
		{name: "stale writer epoch", mutate: func(task *craft.Task, _ *agentruntime.Run) { task.Fence.Epoch-- }},
		{name: "changed actor", mutate: func(_ *craft.Task, run *agentruntime.Run) { run.ActorUserID = "different-actor" }},
		{name: "changed caller actor", callerUserID: "different-actor"},
		{name: "changed snapshot workspace", mutate: func(_ *craft.Task, run *agentruntime.Run) {
			run.Snapshot = craftRunViewAssemblySnapshot(t, "other-workspace")
		}},
		{name: "lost running status", mutate: func(_ *craft.Task, run *agentruntime.Run) { run.Status = "succeeded" }},
		{name: "lost worker slot", mutate: func(_ *craft.Task, run *agentruntime.Run) { run.Owner = "other-worker" }},
		{name: "expired lease", mutate: func(_ *craft.Task, run *agentruntime.Run) { run.LeaseUntil = time.Now().Add(-time.Minute) }},
		{name: "missing workspace", mutate: func(task *craft.Task, _ *agentruntime.Run) { task.WorkspaceID = "" }},
		{name: "missing owner", mutate: func(task *craft.Task, _ *agentruntime.Run) { task.Scope.UserID = "" }},
		{name: "missing Run identity", mutate: func(task *craft.Task, _ *agentruntime.Run) { task.Fence.RunID = "" }},
		{name: "missing worker slot", mutate: func(task *craft.Task, _ *agentruntime.Run) { task.Fence.Owner = "" }},
		{name: "durable Run is missing", mutate: func(_ *craft.Task, run *agentruntime.Run) { run.Key.RunID = "deleted-run" }},
		{name: "unknown Task digest version", mutate: func(task *craft.Task, _ *agentruntime.Run) { task.SnapshotDigestVersion = 2 }},
		{name: "Task and Fence digests differ", mutate: func(task *craft.Task, _ *agentruntime.Run) { task.SnapshotDigest = strings.Repeat("f", 64) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, engine, api, store, _, _ := newMaterialHandleFixture(t, "assembly-deny-generation")
			engineSpy := &craftRunViewAssemblyCountingEngine{CraftRunViewContainerEngine: engine}
			apiSpy := &craftRunViewAssemblyCountingSessionAPI{craftRunViewSessionAPI: api}
			storeSpy := &craftRunViewAssemblyCountingStore{RunViewStore: store}
			run := validCraftRunViewAssemblyRun(t, store.view.Key)
			task := validCraftRunViewAssemblyTask(t)
			if tt.mutate != nil {
				tt.mutate(&task, &run)
			}
			reader := &craftRunViewAssemblyRunReader{run: run}
			assembly, err := assembleCraftRunViewProductionWithAPI(provider.config, storeSpy,
				&admittedRuntimeAuthorityFake{view: store.view}, reader, engineSpy,
				func(string, string, string) (craftRunViewSessionAPI, error) { return apiSpy, nil })
			require.NoError(t, err)

			ctx := craftRunViewAssemblyActorContext()
			if tt.callerUserID != "" {
				ctx = types.WithCaller(context.Background(), types.Caller{TenantID: 7, UserID: tt.callerUserID})
			}
			_, err = assembly.ResolveMaterial(ctx, task)
			require.Error(t, err)
			require.Zero(t, storeSpy.allocateCalls, "must deny before allocating a RunView generation")
			require.Zero(t, storeSpy.loadCalls, "must deny before loading/issuing any RunView material")
			require.Zero(t, engineSpy.calls, "must deny before Docker network/container operations")
			require.Zero(t, apiSpy.calls, "must deny before OpenCode session operations")
		})
	}
}

func TestCraftRunViewProductionAssemblyRejectsChangedSameWorkspaceSnapshotDigestBeforeSideEffects(t *testing.T) {
	provider, engine, api, store, _, _ := newMaterialHandleFixture(t, "assembly-stale-digest-generation")
	engineSpy := &craftRunViewAssemblyCountingEngine{CraftRunViewContainerEngine: engine}
	apiSpy := &craftRunViewAssemblyCountingSessionAPI{craftRunViewSessionAPI: api}
	storeSpy := &craftRunViewAssemblyCountingStore{RunViewStore: store}
	task := validCraftRunViewAssemblyTask(t) // retains the original admission digest
	run := validCraftRunViewAssemblyRun(t, store.view.Key)

	snapshot, err := service.ParseDurableRunSnapshot(run.Snapshot)
	require.NoError(t, err)
	snapshot.Query = "changed but still valid query"
	run.Snapshot, err = json.Marshal(snapshot)
	require.NoError(t, err)
	version, digest, err := repository.CraftAdmittedSnapshotIdentity(run.Snapshot)
	require.NoError(t, err)
	run.SnapshotDigestVersion, run.SnapshotDigest = version, digest
	require.Equal(t, task.WorkspaceID, snapshot.CraftWorkspaceSeed.WorkspaceID, "the Workspace remains unchanged")
	require.NotEqual(t, task.SnapshotDigest, run.SnapshotDigest, "the durable Run carries a matching digest for the changed snapshot")

	reader := &craftRunViewAssemblyRunReader{run: run}
	assembly, err := assembleCraftRunViewProductionWithAPI(provider.config, storeSpy,
		&admittedRuntimeAuthorityFake{view: store.view}, reader, engineSpy,
		func(string, string, string) (craftRunViewSessionAPI, error) { return apiSpy, nil })
	require.NoError(t, err)
	_, err = assembly.ResolveMaterial(craftRunViewAssemblyActorContext(), task)
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Zero(t, storeSpy.allocateCalls, "stale Task identity must be denied before generation allocation")
	require.Zero(t, storeSpy.loadCalls, "stale Task identity must be denied before material loading")
	require.Zero(t, engineSpy.calls, "stale Task identity must be denied before Docker operations")
	require.Zero(t, apiSpy.calls, "stale Task identity must be denied before OpenCode operations")
}

func validCraftRunViewAssemblyTask(t *testing.T) craft.Task {
	t.Helper()
	snapshot := craftRunViewAssemblySnapshot(t, "workspace-b")
	version, digest, err := repository.CraftAdmittedSnapshotIdentity(snapshot)
	require.NoError(t, err)
	return craft.Task{
		Scope:                 craft.Scope{TenantID: 7, UserID: "owner", SessionID: "task-session"},
		Fence:                 agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 7, RunID: "run-layout"}, Owner: "worker", Epoch: 9, SnapshotDigestVersion: version, SnapshotDigest: digest},
		SnapshotDigestVersion: version, SnapshotDigest: digest, WorkspaceID: "workspace-b",
	}
}

func validCraftRunViewAssemblyRun(t *testing.T, key craft.RunViewKey) agentruntime.Run {
	t.Helper()
	snapshot := craftRunViewAssemblySnapshot(t, "workspace-b")
	version, digest, err := repository.CraftAdmittedSnapshotIdentity(snapshot)
	require.NoError(t, err)
	return agentruntime.Run{
		Key:       agentruntime.RunKey{TenantID: key.TenantID, RunID: key.RunID},
		SessionID: key.SessionID, UserID: key.OwnerID, ActorUserID: "actor",
		Status: "running", Owner: "worker", Epoch: 9, LeaseUntil: time.Now().Add(time.Hour),
		Snapshot: snapshot, SnapshotDigestVersion: version, SnapshotDigest: digest,
	}
}

func craftRunViewAssemblySnapshot(t *testing.T, workspaceID string) json.RawMessage {
	if t != nil {
		t.Helper()
	}
	raw, err := json.Marshal(service.DurableRunSnapshot{
		Version: 1, Query: "build", ModelID: "model", AgentConfig: json.RawMessage(`{}`),
		CraftWorkspaceSeed: &service.CraftWorkspaceSeedSnapshot{WorkspaceID: workspaceID, State: craft.DraftHeadEmpty},
	})
	if err != nil && t != nil {
		t.Fatal(err)
	}
	return raw
}

func craftRunViewAssemblyActorContext() context.Context {
	return types.WithCaller(context.Background(), types.Caller{TenantID: 7, UserID: "actor"})
}

type craftRunViewAssemblyRunReader struct{ run agentruntime.Run }

func (r *craftRunViewAssemblyRunReader) Get(_ context.Context, key agentruntime.RunKey) (agentruntime.Run, error) {
	if r.run.Key != key {
		return agentruntime.Run{}, craft.ErrNotFound
	}
	return r.run, nil
}

type craftRunViewAssemblyCountingEngine struct {
	CraftRunViewContainerEngine
	calls int
}

func (e *craftRunViewAssemblyCountingEngine) EnsurePrivateNetwork(ctx context.Context, spec CraftRunViewContainerNetworkSpec) (CraftRunViewContainerNetwork, error) {
	e.calls++
	return e.CraftRunViewContainerEngine.EnsurePrivateNetwork(ctx, spec)
}
func (e *craftRunViewAssemblyCountingEngine) InspectContainer(ctx context.Context, id string) (*CraftRunViewEngineContainer, error) {
	e.calls++
	return e.CraftRunViewContainerEngine.InspectContainer(ctx, id)
}
func (e *craftRunViewAssemblyCountingEngine) CreateContainer(ctx context.Context, req CraftRunViewContainerCreateRequest) (string, error) {
	e.calls++
	return e.CraftRunViewContainerEngine.CreateContainer(ctx, req)
}
func (e *craftRunViewAssemblyCountingEngine) StartContainer(ctx context.Context, id string) error {
	e.calls++
	return e.CraftRunViewContainerEngine.StartContainer(ctx, id)
}
func (e *craftRunViewAssemblyCountingEngine) ProbeRuntime(ctx context.Context, id string) (CraftRunViewRuntimeProbe, error) {
	e.calls++
	return e.CraftRunViewContainerEngine.ProbeRuntime(ctx, id)
}

type craftRunViewAssemblyCountingSessionAPI struct {
	craftRunViewSessionAPI
	calls int
}

func (a *craftRunViewAssemblyCountingSessionAPI) ListSessions(ctx context.Context) ([]opencode.SessionInfo, error) {
	a.calls++
	return a.craftRunViewSessionAPI.ListSessions(ctx)
}
func (a *craftRunViewAssemblyCountingSessionAPI) GetSession(ctx context.Context, id string) (opencode.SessionInfo, error) {
	a.calls++
	return a.craftRunViewSessionAPI.GetSession(ctx, id)
}
func (a *craftRunViewAssemblyCountingSessionAPI) CreateSession(ctx context.Context) (string, error) {
	a.calls++
	return a.craftRunViewSessionAPI.CreateSession(ctx)
}

type craftRunViewAssemblyCountingStore struct {
	craft.RunViewStore
	allocateCalls int
	loadCalls     int
}

func (s *craftRunViewAssemblyCountingStore) Allocate(ctx context.Context, key craft.RunViewKey) (craft.RunView, error) {
	s.allocateCalls++
	return s.RunViewStore.Allocate(ctx, key)
}
func (s *craftRunViewAssemblyCountingStore) Load(ctx context.Context, key craft.RunViewKey) (craft.RunView, error) {
	s.loadCalls++
	return s.RunViewStore.Load(ctx, key)
}
