package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

type craftPreviewConfigLoader struct{ config *types.TenantSandboxConfig }

type craftPreviewInspect struct{ mode string }

func (i *craftPreviewInspect) NetworkMode(context.Context, *sandbox.Config, string) (string, error) {
	return i.mode, nil
}

func (l *craftPreviewConfigLoader) Load(context.Context, uint64, string) (sandbox.ResolvedTenantSandboxConfig, error) {
	return sandbox.ResolvedTenantSandboxConfig{Config: l.config, Found: true}, nil
}

func TestCraftPreviewBoundSandboxNetworkPolicy(t *testing.T) {
	ctx := context.Background()
	scope := craft.Scope{TenantID: 129, UserID: "viewer", SessionID: "craft107-t14"}
	key := sandbox.SessionSandboxKey{TenantID: scope.TenantID, SessionID: scope.SessionID}
	bindings := sandbox.NewMemorySessionSandboxBindingStore()
	created, err := bindings.Create(ctx, key, sandbox.SessionSandboxBinding{
		Version: sandbox.SessionSandboxBindingVersion, Provider: sandbox.SandboxTypeDocker,
		TenantID: scope.TenantID, SessionID: scope.SessionID, SandboxID: "craft107-t14-preview",
		TemplateID: "craft-image", ConfigID: "cfg-no-egress", CreatedAt: time.Now(),
	})
	require.NoError(t, err)
	require.True(t, created)
	loader := &craftPreviewConfigLoader{config: &types.TenantSandboxConfig{
		SandboxType: "docker", Docker: &types.DockerSandboxConfig{Image: "craft-image", NetworkMode: "bridge"},
	}}
	inspect := &craftPreviewInspect{mode: "bridge"}
	checker := CraftPreviewDockerNetworkChecker{Bindings: bindings, Loader: loader, Global: sandbox.DefaultConfig(), Inspector: inspect}
	require.ErrorIs(t, checker.CheckPreviewNoEgress(ctx, scope), craft.ErrUnsupported, "Docker's default bridge has outbound routes")
	loader.config.Docker.NetworkMode = "none"
	require.ErrorIs(t, checker.CheckPreviewNoEgress(ctx, scope), craft.ErrUnsupported, "old live container may still use bridge")
	inspect.mode = "none"
	require.NoError(t, checker.CheckPreviewNoEgress(ctx, scope))
	foreign := scope
	foreign.TenantID++
	require.ErrorIs(t, checker.CheckPreviewNoEgress(ctx, foreign), craft.ErrUnsupported)
}

// Opt-in integration: the caller starts a running Docker container and names
// its ID. It tests the production inspector, not just the config predicate.
func TestCraftPreviewLocalDockerInspector(t *testing.T) {
	id := os.Getenv("CRAFT_PREVIEW_CONTAINER_ID")
	if id == "" {
		t.Skip("set CRAFT_PREVIEW_CONTAINER_ID to a running no-egress container")
	}
	mode, err := (CraftPreviewLocalDockerInspector{}).NetworkMode(context.Background(), sandbox.DefaultConfig(), id)
	require.NoError(t, err)
	require.Equal(t, "none", mode)
}
