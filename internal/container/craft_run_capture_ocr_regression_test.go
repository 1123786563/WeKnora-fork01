package container

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
)

// TestCraftRunCaptureRunnerRecoversStrandedReceiptsAfterProcessRestart is the
// OCR high-finding regression: after the process restarts (fresh provider
// with an empty in-memory binding table and a fresh engine that no longer
// knows the container) a stranded terminal capture receipt must still drain.
// The capture source's binding check must rely on the durable writer fence
// and the on-disk immutable generation identity, never on the live engine
// binding that only the container-start path populated.
func TestCraftRunCaptureRunnerRecoversStrandedReceiptsAfterProcessRestart(t *testing.T) {
	db := wiringTestDB(t)
	seedCraftCaptureWiringDB(t, db)
	ctx := context.Background()
	_, err := repository.NewCraftStore(db).PutWorkspace(ctx, craft.Workspace{
		Scope: craft.Scope{TenantID: 1, UserID: "u-wiring", SessionID: "s-wiring"},
		SandboxID: "sbx-restart", Generation: "0",
		OpenCodeSessionID: "oc-restart", RuntimeDigest: "sha256:runtime",
	}, 0)
	require.NoError(t, err)

	runs := repository.NewAgentRunStore(db)
	admission := captureWiringAdmission("run-restart-a")
	_, err = runs.Admit(ctx, admission)
	require.NoError(t, err)

	engine := newFakeCraftRunViewContainerEngine()
	provider, _ := newRVTestProviderWithSessionAPI(t, engine)
	views := repository.NewCraftRunViewStore(db)
	key := craft.RunViewKey{TenantID: 1, OwnerID: "u-wiring", SessionID: "s-wiring", RunID: "run-restart-a"}
	view, err := views.Allocate(ctx, key)
	require.NoError(t, err)
	spec := rvTestSpec(view.Generation)
	container, err := provider.InspectOrCreateContainer(ctx, spec)
	require.NoError(t, err)
	const sessionID = "ses_0123456789ab0123456789ABCF"
	_, _, err = views.BeginSessionCreate(ctx, key, view.Generation)
	require.NoError(t, err)
	_, err = views.BindRuntime(ctx, key, view.Generation, runtimeForSession(container, CraftRunViewRuntimeSession{
		ID: sessionID, ProjectID: container.ProjectID, Directory: container.Directory,
	}))
	require.NoError(t, err)

	layout := generationLayoutPaths(provider.config.SandboxRoot, spec)
	require.NoError(t, os.MkdirAll(layout.output, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(layout.output, "index.html"), []byte("<html>R1</html>"), 0o644))

	fence, err := runs.Claim(ctx, admission.Key, "worker-restart", time.Hour)
	require.NoError(t, err)
	require.NoError(t, runs.Finalize(ctx, fence, json.RawMessage(`{"content":"done"}`)))
	var pending string
	require.NoError(t, db.Table("craft_run_captures").Select("state").
		Where("tenant_id = ? AND run_id = ?", 1, "run-restart-a").Scan(&pending).Error)
	require.Equal(t, "pending", pending, "the terminal transition must enqueue the durable capture receipt")

	// Process restart: a brand-new provider over the same sandbox root and a
	// fresh engine. Its in-memory binding table is empty and no container
	// start will ever repopulate it for this terminal Run.
	restartProvider, err := NewCraftRunViewContainerProvider(provider.config, newFakeCraftRunViewContainerEngine())
	require.NoError(t, err)

	runner := newCraftRunCaptureRunner(db, newCaptureWiringFiles(t, db),
		repository.NewCraftVersionStore(db),
		&CraftRunViewProductionAssembly{Provider: restartProvider, Store: views})
	require.Empty(t, runner.Unavailable())
	runner.AfterTerminal(ctx, fence)

	var state string
	require.NoError(t, db.Table("craft_run_captures").Select("state").
		Where("tenant_id = ? AND run_id = ?", 1, "run-restart-a").Scan(&state).Error)
	require.Equal(t, "advanced", state, "a stranded receipt must converge after a process restart without a live engine binding")
	var head struct {
		Revision    int64
		SourceRunID string `gorm:"column:source_run_id"`
	}
	require.NoError(t, db.Table("craft_workspace_draft_heads").Select("revision, source_run_id").
		Where("workspace_id = ?", materialWorkspaceID(t, db, "s-wiring")).Scan(&head).Error)
	require.EqualValues(t, 1, head.Revision)
	require.Equal(t, "run-restart-a", head.SourceRunID)
}

// budgetRecordingDrain records the fence and the context state observed at
// the moment the bounded drain seam invokes the runner.
type budgetRecordingDrain struct {
	errAtCall    error
	deadlineAtGo time.Time
	hasDeadline  bool
	fence        runtime.Fence
	calls        int
}

func (d *budgetRecordingDrain) AfterTerminal(ctx context.Context, fence runtime.Fence) {
	d.calls++
	d.errAtCall = ctx.Err()
	d.deadlineAtGo, d.hasDeadline = ctx.Deadline()
	d.fence = fence
}

// TestDrainCraftCaptureAfterTerminalBoundsTheDrainContext is the OCR
// medium-finding regression: the post-terminal drain that runs synchronously
// in the worker executor goroutine must carry a deadline, must not be
// pre-canceled by an already torn-down run context, and a nil drain is a
// no-op.
func TestDrainCraftCaptureAfterTerminalBoundsTheDrainContext(t *testing.T) {
	drain := &budgetRecordingDrain{}
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	fence := runtime.Fence{RunKey: runtime.RunKey{TenantID: 1, RunID: "run-budget"}}
	drainCraftCaptureAfterTerminal(parent, drain, fence)

	require.Equal(t, 1, drain.calls)
	require.Equal(t, fence, drain.fence)
	require.True(t, drain.hasDeadline, "the drain context must carry a deadline")
	remaining := time.Until(drain.deadlineAtGo)
	require.Greater(t, remaining, time.Duration(0))
	require.LessOrEqual(t, remaining, craftCaptureAfterTerminalBudget)
	require.NoError(t, drain.errAtCall, "a canceled parent must not pre-cancel the bounded drain")

	require.NotPanics(t, func() { drainCraftCaptureAfterTerminal(context.Background(), nil, runtime.Fence{}) })
}

// TestQuiescenceProofRejectsSameSizeInPlaceRewrite is the OCR medium-finding
// regression: two walks whose projected entries agree on Name/Path/Type/Size
// but whose content was rewritten in place between the walks must fail the
// quiescence proof through the full identity comparison (sha256 digest plus
// stat identity).
func TestQuiescenceProofRejectsSameSizeInPlaceRewrite(t *testing.T) {
	db := wiringTestDB(t)
	seedCraftCaptureWiringDB(t, db)
	ctx := context.Background()
	_, err := repository.NewCraftStore(db).PutWorkspace(ctx, craft.Workspace{
		Scope: craft.Scope{TenantID: 1, UserID: "u-wiring", SessionID: "s-wiring"},
		SandboxID: "sbx-quiesce", Generation: "0",
		OpenCodeSessionID: "oc-quiesce", RuntimeDigest: "sha256:runtime",
	}, 0)
	require.NoError(t, err)

	runs := repository.NewAgentRunStore(db)
	admission := captureWiringAdmission("run-quiesce-a")
	_, err = runs.Admit(ctx, admission)
	require.NoError(t, err)

	engine := newFakeCraftRunViewContainerEngine()
	provider, _ := newRVTestProviderWithSessionAPI(t, engine)
	views := repository.NewCraftRunViewStore(db)
	key := craft.RunViewKey{TenantID: 1, OwnerID: "u-wiring", SessionID: "s-wiring", RunID: "run-quiesce-a"}
	view, err := views.Allocate(ctx, key)
	require.NoError(t, err)
	spec := rvTestSpec(view.Generation)
	container, err := provider.InspectOrCreateContainer(ctx, spec)
	require.NoError(t, err)
	const sessionID = "ses_0123456789ab0123456789ABD0"
	_, _, err = views.BeginSessionCreate(ctx, key, view.Generation)
	require.NoError(t, err)
	_, err = views.BindRuntime(ctx, key, view.Generation, runtimeForSession(container, CraftRunViewRuntimeSession{
		ID: sessionID, ProjectID: container.ProjectID, Directory: container.Directory,
	}))
	require.NoError(t, err)

	output := filepath.Join(generationLayoutPaths(provider.config.SandboxRoot, spec).output, "index.html")
	require.NoError(t, os.MkdirAll(filepath.Dir(output), 0o755))
	require.NoError(t, os.WriteFile(output, []byte("<html>A1</html>"), 0o644))

	claimFence, err := runs.Claim(ctx, admission.Key, "worker-quiesce", time.Hour)
	require.NoError(t, err)
	material, err := provider.MaterialHandleForCapture(ctx, views, key)
	require.NoError(t, err)
	task := craft.Task{
		Scope: craft.Scope{TenantID: 1, UserID: "u-wiring", SessionID: "s-wiring"},
		WorkspaceID: materialWorkspaceID(t, db, "s-wiring"),
		Fence:       runtime.Fence{RunKey: runtime.RunKey{TenantID: 1, RunID: "run-quiesce-a"}, Epoch: claimFence.Epoch},
	}
	source, err := newRunBoundCraftArtifactSourceForCapture(ctx, &localCraftRuntime{db: db}, task, material, craftLocalOutputDir)
	require.NoError(t, err)
	q := &quiescentRunViewArtifactSource{runBoundCraftArtifactSource: source}

	// Control: a stable tree passes the proof.
	require.NoError(t, q.VerifyCraftCaptureQuiescent(ctx))

	// Document the blind spot of the projected-entry comparison: rewrite the
	// file in place with the same size between two explicit walks and the
	// Name/Path/Type/Size projection still agrees ...
	first, err := source.ListSessionFiles(ctx, task.Scope.SessionID, craftLocalOutputDir)
	require.NoError(t, err)
	snapshot := source.snapshotListed()
	require.NoError(t, os.WriteFile(output, []byte("<html>B1</html>"), 0o644))
	second, err := source.ListSessionFiles(ctx, task.Scope.SessionID, craftLocalOutputDir)
	require.NoError(t, err)
	require.Equal(t, len(first), len(second))
	for i := range first {
		require.Equal(t, first[i].Name, second[i].Name)
		require.Equal(t, first[i].Path, second[i].Path)
		require.Equal(t, first[i].Type, second[i].Type)
		require.Equal(t, first[i].Size, second[i].Size)
	}
	// ... while the full identity comparison catches it.
	require.False(t, source.listedMatches(snapshot), "a same-size in-place rewrite must change the identity table")

	// End to end: mutating between the proof's two walks fails the proof.
	rootReads := 0
	source.afterDirectoryRead = func(rel string) {
		if rel != "" {
			return
		}
		rootReads++
		if rootReads == 2 { // the re-list's root read, before its child stats
			require.NoError(t, os.WriteFile(output, []byte("<html>C1</html>"), 0o644))
		}
	}
	require.ErrorIs(t, q.VerifyCraftCaptureQuiescent(ctx), craft.ErrBusy)
}
