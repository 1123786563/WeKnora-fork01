package container

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestCraftRunViewMaterialRevalidationRejectsChangedKnowledgeMount(t *testing.T) {
	task := craftRuntimeInputTaskForMountTest()
	material := newCraftRuntimeTestMaterial(t, task, materialTestGeneration(t))
	require.NoError(t, material.provider.RevalidateMaterialHandle(context.Background(), material))

	engine, ok := material.provider.engine.(*fakeCraftRunViewContainerEngine)
	require.True(t, ok)
	engine.mu.Lock()
	container := engine.containers[craftRunViewSpecForGeneration(material.generation).ContainerID]
	for i := range container.Mounts {
		if container.Mounts[i].Destination == filepath.ToSlash(filepath.Join(material.directory, "knowledge")) {
			container.Mounts[i].ReadOnly = false
		}
	}
	engine.containers[container.Name] = container
	engine.mu.Unlock()

	err := material.provider.RevalidateMaterialHandle(context.Background(), material)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
}

func TestLocalCraftRuntimeRequiresAcceptedKnowledgeBeforeDispatch(t *testing.T) {
	fixture := newRunViewInputFixture(t, nil, map[string][]byte{})
	spy := &craftRuntimeExecutorSpy{}
	fixture.runtime.inner = spy
	fixture.runtime.materialResolver = func(context.Context, craft.Task) (CraftRunViewMaterialHandle, error) {
		return fixture.material, nil
	}
	_, err := fixture.runtime.Execute(context.Background(), fixture.task)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
	require.Contains(t, err.Error(), "accepted Run knowledge package resolver")
	require.Zero(t, spy.calls, "unaccepted Run knowledge must not reach the prompt executor")
}

func TestLocalCraftRuntimeRejectsForeignAcceptedKnowledgeBeforeWorkspaceDispatch(t *testing.T) {
	fixture := newRunViewInputFixture(t, nil, map[string][]byte{})
	spy := &craftRuntimeExecutorSpy{}
	fixture.runtime.inner = spy
	fixture.runtime.materialResolver = func(context.Context, craft.Task) (CraftRunViewMaterialHandle, error) {
		return fixture.material, nil
	}
	fixture.runtime.knowledgeResolver = func(context.Context, craft.Task, CraftRunViewMaterialHandle) (CraftKnowledgeRunViewAcceptance, error) {
		return CraftKnowledgeRunViewAcceptance{RunID: "run-foreign", PackageDigest: strings.Repeat("a", 64)}, nil
	}
	_, err := fixture.runtime.Execute(context.Background(), fixture.task)
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Zero(t, spy.calls, "a foreign Run package must not reach the prompt executor")
}

// This opt-in proof exercises the running Linux container engine, rather than
// inferring read-only behavior from Docker request fields or a fake engine.
func TestCraftRunViewLinuxEngineKnowledgeBindIsReadOnly(t *testing.T) {
	if os.Getenv("CRAFT_RUNVIEW_LINUX_MOUNT_TEST") != "1" {
		t.Skip("set CRAFT_RUNVIEW_LINUX_MOUNT_TEST=1 to run the real engine proof")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker CLI is unavailable")
	}
	engineOS, err := exec.Command("docker", "info", "--format", "{{.OSType}}").Output()
	require.NoError(t, err, "Docker daemon inspection must succeed")
	require.Equal(t, "linux", strings.TrimSpace(string(engineOS)), "the actual engine proof requires a Linux Docker daemon")

	image := "weknora-craft-runtime:1.18.4"
	imageID, err := exec.Command("docker", "image", "inspect", "--format", "{{.Id}}", image).Output()
	require.NoError(t, err, "the pinned Craft runtime image must be present locally")
	image = strings.TrimSpace(string(imageID))
	builder, run, material, _, _, _, _, _ := newCraftRunViewKnowledgeTestBuilder(t, []string{}, "owner-1", "owner-1", "run-mount-proof")
	ctx := types.WithCaller(context.Background(), types.Caller{TenantID: 7, UserID: "owner-1"})
	accepted, err := builder.BuildForRun(ctx, run, material)
	require.NoError(t, err)
	require.Equal(t, run.Key.RunID, accepted.RunID)
	source := filepath.Join(material.root, filepath.FromSlash(craft.KnowledgeRunDir(run.Key.RunID)))
	require.FileExists(t, filepath.Join(source, "manifest.json"), "proof mounts the actual sealed Run package")

	cmd := exec.Command("docker", "run", "--rm", "--network", "none",
		"--mount", "type=bind,src="+source+",dst=/workspace/knowledge,readonly",
		"--entrypoint", "/bin/sh", image, "-c",
		"test -s /workspace/knowledge/manifest.json && ! touch /workspace/knowledge/write-probe")
	output, err := cmd.CombinedOutput()
	if err != nil {
		require.False(t, errors.Is(err, exec.ErrNotFound), "Docker engine should be available")
		t.Fatalf("Linux read-only knowledge mount proof failed: %v\n%s", err, output)
	}
	_, err = os.Lstat(filepath.Join(source, "write-probe"))
	require.True(t, os.IsNotExist(err), "container write must not alter the host package")
}

func craftRuntimeInputTaskForMountTest() craft.Task {
	return craft.Task{Scope: craft.Scope{TenantID: 1, UserID: "owner", SessionID: "task-session"},
		Fence: agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 1, RunID: "run-layout"}}}
}
