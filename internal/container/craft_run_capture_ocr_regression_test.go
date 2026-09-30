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
	"gorm.io/gorm"
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
		Scope:     craft.Scope{TenantID: 1, UserID: "u-wiring", SessionID: "s-wiring"},
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
		Scope:     craft.Scope{TenantID: 1, UserID: "u-wiring", SessionID: "s-wiring"},
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
		Scope:       craft.Scope{TenantID: 1, UserID: "u-wiring", SessionID: "s-wiring"},
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

// captureWiringAdmissionFor is captureWiringAdmission parameterized by the
// craft session, so one test can hold two independent Workspaces.
func captureWiringAdmissionFor(sessionID, runID string) runtime.Admission {
	snapshot, err := json.Marshal(map[string]any{
		"version": 1, "query": "build", "model_id": "model-1",
		"agent_config": json.RawMessage(`{}`), "craft_input_manifest": []craft.Input{},
	})
	if err != nil {
		panic(err)
	}
	return runtime.Admission{
		Key:                runtime.RunKey{TenantID: 1, RunID: runID},
		SessionID:          sessionID,
		UserID:             "u-wiring",
		ActorUserID:        "u-wiring",
		RequestID:          runID + "-request",
		AssistantMessageID: runID + "-assistant",
		RequestHash:        runID + "-hash",
		Snapshot:           snapshot,
		UserMessage:        json.RawMessage(`{"role":"user","content":"build"}`),
		AssistantMessage:   json.RawMessage(`{"role":"assistant","content":""}`),
		Deadline:           time.Now().Add(time.Hour),
	}
}

// seedPendingCaptureReceipt materializes one fully bound, terminal Run with
// a pending capture receipt and returns the fence of its final claim.
func seedPendingCaptureReceipt(t *testing.T, db *gorm.DB, provider *CraftRunViewContainerProvider, sessionID, runID, sessionMarker, html string) runtime.Fence {
	t.Helper()
	ctx := context.Background()
	_, err := repository.NewCraftStore(db).PutWorkspace(ctx, craft.Workspace{
		Scope:     craft.Scope{TenantID: 1, UserID: "u-wiring", SessionID: sessionID},
		SandboxID: "sbx-" + runID, Generation: "0",
		OpenCodeSessionID: "oc-" + runID, RuntimeDigest: "sha256:runtime",
	}, 0)
	require.NoError(t, err)

	runs := repository.NewAgentRunStore(db)
	admission := captureWiringAdmissionFor(sessionID, runID)
	_, err = runs.Admit(ctx, admission)
	require.NoError(t, err)

	views := repository.NewCraftRunViewStore(db)
	key := craft.RunViewKey{TenantID: 1, OwnerID: "u-wiring", SessionID: sessionID, RunID: runID}
	view, err := views.Allocate(ctx, key)
	require.NoError(t, err)
	spec := rvTestSpec(view.Generation)
	container, err := provider.InspectOrCreateContainer(ctx, spec)
	require.NoError(t, err)
	_, _, err = views.BeginSessionCreate(ctx, key, view.Generation)
	require.NoError(t, err)
	_, err = views.BindRuntime(ctx, key, view.Generation, runtimeForSession(container, CraftRunViewRuntimeSession{
		ID: sessionMarker, ProjectID: container.ProjectID, Directory: container.Directory,
	}))
	require.NoError(t, err)

	layout := generationLayoutPaths(provider.config.SandboxRoot, spec)
	require.NoError(t, os.MkdirAll(layout.output, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(layout.output, "index.html"), []byte(html), 0o644))

	fence, err := runs.Claim(ctx, admission.Key, "worker-"+runID, time.Hour)
	require.NoError(t, err)
	require.NoError(t, runs.Finalize(ctx, fence, json.RawMessage(`{"content":"done"}`)))
	var pending string
	require.NoError(t, db.Table("craft_run_captures").Select("state").
		Where("tenant_id = ? AND run_id = ?", 1, runID).Scan(&pending).Error)
	require.Equal(t, "pending", pending)
	return fence
}

// TestCraftRunCaptureAfterTerminalDrainsOnlyTheFinishedRun is the OCR
// medium-finding regression: the post-terminal drain must be directed by the
// finished Run's fence instead of running a global recovery pass, so an
// unrelated Workspace's receipt is left for the periodic scan.
func TestCraftRunCaptureAfterTerminalDrainsOnlyTheFinishedRun(t *testing.T) {
	db := wiringTestDB(t)
	seedCraftCaptureWiringDB(t, db)
	require.NoError(t, db.Exec("INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s-wiring-2', 1, 'wiring', 'u-wiring', 'trpc')").Error)
	require.NoError(t, db.Exec("INSERT INTO craft_sessions (session_id, tenant_id, kind) VALUES ('s-wiring-2', 1, 'web')").Error)

	engine := newFakeCraftRunViewContainerEngine()
	provider, _ := newRVTestProviderWithSessionAPI(t, engine)
	fenceA := seedPendingCaptureReceipt(t, db, provider, "s-wiring", "run-directed-a", "ses_0123456789ab0123456789ABE1", "<html>D1</html>")
	_ = seedPendingCaptureReceipt(t, db, provider, "s-wiring-2", "run-directed-b", "ses_0123456789ab0123456789ABE2", "<html>D2</html>")

	runner := newCraftRunCaptureRunner(db, newCaptureWiringFiles(t, db),
		repository.NewCraftVersionStore(db),
		&CraftRunViewProductionAssembly{Provider: provider, Store: repository.NewCraftRunViewStore(db)})
	require.Empty(t, runner.Unavailable())
	runner.AfterTerminal(context.Background(), fenceA)

	var stateA, stateB string
	require.NoError(t, db.Table("craft_run_captures").Select("state").
		Where("tenant_id = ? AND run_id = ?", 1, "run-directed-a").Scan(&stateA).Error)
	require.NoError(t, db.Table("craft_run_captures").Select("state").
		Where("tenant_id = ? AND run_id = ?", 1, "run-directed-b").Scan(&stateB).Error)
	require.Equal(t, "advanced", stateA, "the finished Run's own receipt must drain")
	require.Equal(t, "pending", stateB, "an unrelated Workspace's receipt must wait for the periodic scan")
}

// TestCraftRunCaptureScanBudgetExceedsDrainBudgetAndInterval pins the OCR
// high-finding contract: the periodic scan's per-round budget must be
// materially larger than both the scan interval and the immediate drain
// budget, otherwise receipts cut off mid-seal on both paths could never
// converge.
func TestCraftRunCaptureScanBudgetExceedsDrainBudgetAndInterval(t *testing.T) {
	require.Greater(t, craftRunCaptureScanBudget, 2*craftRunCaptureScanInterval,
		"the scan budget must be decoupled from (and larger than) the scan interval")
	require.Greater(t, craftRunCaptureScanBudget, 2*craftCaptureAfterTerminalBudget,
		"the scan budget must materially exceed the immediate drain budget")
}

// TestCraftRunCaptureInertLogEmittedOnce is the OCR low-finding regression:
// in an unconfigured deployment the inert explanation is logged once per
// process, not once per graph-run completion.
func TestCraftRunCaptureInertLogEmittedOnce(t *testing.T) {
	runner := newCraftRunCaptureRunner(nil, nil, nil, nil)
	logs := 0
	runner.inertLog = func(context.Context, string, ...any) { logs++ }
	for i := 0; i < 3; i++ {
		runner.AfterTerminal(context.Background(), runtime.Fence{RunKey: runtime.RunKey{TenantID: 1, RunID: "run-noise"}})
		runner.Recover(context.Background(), 10)
	}
	require.Equal(t, 1, logs, "the inert explanation must be logged once, not per run completion")
}

// TestCraftRunViewCandidateStagesWithNonDefaultOutputDirEnv is the OCR
// medium-finding regression: with CRAFT_OPENCODE_OUTPUT_DIR set to a
// non-default value the Run-bound candidate route still stages its private
// candidate from the verified generation's fixed "output" layout instead of
// failing its first ListSessionFiles with only a Warn.
func TestCraftRunViewCandidateStagesWithNonDefaultOutputDirEnv(t *testing.T) {
	db := wiringTestDB(t)
	seedCraftCaptureWiringDB(t, db)
	ctx := context.Background()
	_, err := repository.NewCraftStore(db).PutWorkspace(ctx, craft.Workspace{
		Scope:     craft.Scope{TenantID: 1, UserID: "u-wiring", SessionID: "s-wiring"},
		SandboxID: "sbx-candenv", Generation: "0",
		OpenCodeSessionID: "oc-candenv", RuntimeDigest: "sha256:runtime",
	}, 0)
	require.NoError(t, err)
	runs := repository.NewAgentRunStore(db)
	admission := captureWiringAdmission("run-candenv-a")
	_, err = runs.Admit(ctx, admission)
	require.NoError(t, err)

	t.Setenv(craftOpenCodeBaseURLEnv, "http://127.0.0.1:1")
	t.Setenv(craftOpenCodeWorkDirEnv, t.TempDir())
	t.Setenv(craftOpenCodeOutputDirEnv, "custom-output")
	executor, err := newCraftRuntimeExecutor(db, repository.NewCraftStore(db), runs,
		newCaptureWiringFiles(t, db), repository.NewCraftVersionStore(db), nil, nil, nil)
	require.NoError(t, err)
	runtimeExec, ok := executor.(*localCraftRuntime)
	require.True(t, ok)
	require.NotNil(t, runtimeExec.runViewArtifacts)

	engine := newFakeCraftRunViewContainerEngine()
	provider, _ := newRVTestProviderWithSessionAPI(t, engine)
	views := repository.NewCraftRunViewStore(db)
	key := craft.RunViewKey{TenantID: 1, OwnerID: "u-wiring", SessionID: "s-wiring", RunID: "run-candenv-a"}
	view, err := views.Allocate(ctx, key)
	require.NoError(t, err)
	spec := rvTestSpec(view.Generation)
	container, err := provider.InspectOrCreateContainer(ctx, spec)
	require.NoError(t, err)
	const sessionID = "ses_0123456789ab0123456789ABE3"
	_, _, err = views.BeginSessionCreate(ctx, key, view.Generation)
	require.NoError(t, err)
	_, err = views.BindRuntime(ctx, key, view.Generation, runtimeForSession(container, CraftRunViewRuntimeSession{
		ID: sessionID, ProjectID: container.ProjectID, Directory: container.Directory,
	}))
	require.NoError(t, err)

	layout := generationLayoutPaths(provider.config.SandboxRoot, spec)
	require.NoError(t, os.MkdirAll(layout.output, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(layout.output, "index.html"), []byte("<html>E1</html>"), 0o644))

	claimFence, err := runs.Claim(ctx, admission.Key, "worker-candenv", time.Hour)
	require.NoError(t, err)
	material, err := provider.MaterialHandleForCapture(ctx, views, key)
	require.NoError(t, err)
	task := craft.Task{
		Scope:       craft.Scope{TenantID: 1, UserID: "u-wiring", SessionID: "s-wiring"},
		WorkspaceID: materialWorkspaceID(t, db, "s-wiring"),
		Fence:       runtime.Fence{RunKey: runtime.RunKey{TenantID: 1, RunID: "run-candenv-a"}, Epoch: claimFence.Epoch},
	}
	require.NoError(t, runtimeExec.stageRunViewCandidate(ctx, task, material))

	var candidates int64
	require.NoError(t, db.Table("craft_candidates").
		Where("tenant_id = ? AND run_id = ?", 1, "run-candenv-a").Count(&candidates).Error)
	require.EqualValues(t, 1, candidates, "candidate staging must succeed under a non-default output-dir override")
}
