package container

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/craft"
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

	assembly := provideCraftRunViewProductionAssembly(nil)
	require.Nil(t, assembly.Provider)
	require.Nil(t, assembly.ResolveMaterial)
	require.NotEmpty(t, assembly.Unavailable)
}

func TestCraftRunViewProductionAssemblyResolvesOnlyPersistedBoundTaskRun(t *testing.T) {
	provider, engine, api, store, _, _ := newMaterialHandleFixture(t, "assembly-generation")
	assembly, err := assembleCraftRunViewProductionWithAPI(provider.config, store, engine,
		func(string, string, string) (craftRunViewSessionAPI, error) { return api, nil })
	require.NoError(t, err)
	require.NotNil(t, assembly.Provider)
	require.NotNil(t, assembly.ResolveMaterial)

	task := craft.Task{Scope: craft.Scope{TenantID: 7, UserID: "owner", SessionID: "task-session"}}
	task.Fence.TenantID = 7
	task.Fence.RunID = "run-layout"
	material, err := assembly.ResolveMaterial(context.Background(), task)
	require.NoError(t, err)
	require.Same(t, assembly.Provider, material.provider)
	require.Equal(t, "assembly-generation", material.generation)
	require.NoError(t, assembly.Provider.verifyMaterialHandle(material))

	task.Scope.UserID = "foreign-owner"
	_, err = assembly.ResolveMaterial(context.Background(), task)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
}

func TestCraftRunViewProductionAssemblyRejectsUnboundPersistedRunView(t *testing.T) {
	provider, engine, api, store, _, _ := newMaterialHandleFixture(t, "assembly-unbound-generation")
	store.view.State = craft.RunViewStateAllocating
	assembly, err := assembleCraftRunViewProductionWithAPI(provider.config, store, engine,
		func(string, string, string) (craftRunViewSessionAPI, error) { return api, nil })
	require.NoError(t, err)

	task := craft.Task{Scope: craft.Scope{TenantID: 7, UserID: "owner", SessionID: "task-session"}}
	task.Fence.TenantID = 7
	task.Fence.RunID = "run-layout"
	_, err = assembly.ResolveMaterial(context.Background(), task)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
}
