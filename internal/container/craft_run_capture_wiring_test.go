package container

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// captureWiringFiles is a FileService whose SaveBytes persists bytes under a
// temp dir and registers a tenant-scoped resources row, so the capture
// service's sealed-ref verification (resource:// handles with authoritative
// tenant metadata) succeeds exactly like production.
type captureWiringFiles struct {
	interfaces.FileService
	root string
	db   *gorm.DB
	refs map[string]string
	next int
}

func newCaptureWiringFiles(t *testing.T, db *gorm.DB) *captureWiringFiles {
	t.Helper()
	root := t.TempDir()
	return &captureWiringFiles{root: root, db: db, refs: map[string]string{}}
}

func (f *captureWiringFiles) SaveBytes(_ context.Context, data []byte, tenantID uint64, name string, _ bool) (string, error) {
	f.next++
	handle := fmt.Sprintf("%022d", f.next)
	ref := types.BuildResourcePath(handle)
	physical := filepath.Join(f.root, handle)
	if err := os.WriteFile(physical, data, 0o600); err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	location := sha256.Sum256([]byte(physical))
	if err := f.db.Exec(`INSERT INTO resources (id,handle,tenant_id,provider,physical_path,location_hash,size,content_hash,state)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		fmt.Sprintf("capture-wiring-%d", f.next), handle, tenantID, "test", physical,
		hex.EncodeToString(location[:]), len(data), hex.EncodeToString(sum[:]), types.ResourceStateActive).Error; err != nil {
		return "", err
	}
	f.refs[ref] = physical
	return ref, nil
}

func (f *captureWiringFiles) GetFile(_ context.Context, ref string) (io.ReadCloser, error) {
	physical, ok := f.refs[ref]
	if !ok {
		return nil, craft.ErrNotFound
	}
	data, err := os.ReadFile(physical)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(&captureWiringReader{data: data}), nil
}

func (f *captureWiringFiles) DeleteFile(_ context.Context, ref string) error {
	delete(f.refs, ref)
	return nil
}

type captureWiringReader struct{ data []byte }

func (r *captureWiringReader) Read(dst []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := copy(dst, r.data)
	r.data = r.data[n:]
	return n, nil
}

func seedCraftCaptureWiringDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec("INSERT INTO users (id,username,email,password_hash,tenant_id) VALUES ('u-wiring','u-wiring','u-wiring@example.test','x',1)").Error)
	require.NoError(t, db.Exec("INSERT INTO tenant_members (tenant_id,user_id,role,status,joined_at,created_at,updated_at) VALUES (1,'u-wiring','owner','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)").Error)
	require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id, tenant_id, kind) VALUES ('s-wiring', 1, 'web')`).Error)
}

func captureWiringAdmission(runID string) agentruntime.Admission {
	// A full durable Run snapshot: the Run-bound source re-parses the
	// admitted snapshot and compares its Workspace seed, so query/model/config
	// must be present exactly as production admissions persist them.
	snapshot, err := json.Marshal(map[string]any{
		"version": 1, "query": "build", "model_id": "model-1",
		"agent_config": json.RawMessage(`{}`), "craft_input_manifest": []craft.Input{},
	})
	if err != nil {
		panic(err)
	}
	return agentruntime.Admission{
		Key:                agentruntime.RunKey{TenantID: 1, RunID: runID},
		SessionID:          "s-wiring",
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

// TestCraftRunCaptureRunnerAdvancesTerminalBoundRunAndUnblocksNextRun is the
// R4 Task3 acceptance: a bound RunView Run that reached terminal enqueues a
// durable capture receipt; the post-terminal coordinator drains it through
// the quiescent Run-bound source, seals the output, advances the draft head,
// and the same Workspace can admit its next Run.
func TestCraftRunCaptureRunnerAdvancesTerminalBoundRunAndUnblocksNextRun(t *testing.T) {
	db := wiringTestDB(t)
	seedCraftCaptureWiringDB(t, db)
	ctx := context.Background()
	workspace, err := repository.NewCraftStore(db).PutWorkspace(ctx, craft.Workspace{
		Scope:     craft.Scope{TenantID: 1, UserID: "u-wiring", SessionID: "s-wiring"},
		SandboxID: "sbx-capture", Generation: "0",
		OpenCodeSessionID: "oc-capture", RuntimeDigest: "sha256:runtime",
	}, 0)
	require.NoError(t, err)

	runs := repository.NewAgentRunStore(db)
	admission := captureWiringAdmission("run-capture-a")
	_, err = runs.Admit(ctx, admission)
	require.NoError(t, err)

	// Materialize the generation exactly as the provider would during
	// execution, then bind its runtime identity durably.
	engine := newFakeCraftRunViewContainerEngine()
	provider, _ := newRVTestProviderWithSessionAPI(t, engine)
	views := repository.NewCraftRunViewStore(db)
	key := craft.RunViewKey{TenantID: 1, OwnerID: "u-wiring", SessionID: "s-wiring", RunID: "run-capture-a"}
	view, err := views.Allocate(ctx, key)
	require.NoError(t, err)
	spec := rvTestSpec(view.Generation)
	container, err := provider.InspectOrCreateContainer(ctx, spec)
	require.NoError(t, err)
	const sessionID = "ses_0123456789ab0123456789ABCD"
	_, _, err = views.BeginSessionCreate(ctx, key, view.Generation)
	require.NoError(t, err)
	_, err = views.BindRuntime(ctx, key, view.Generation, runtimeForSession(container, CraftRunViewRuntimeSession{
		ID: sessionID, ProjectID: container.ProjectID, Directory: container.Directory,
	}))
	require.NoError(t, err)

	// The delegation's sealed generation output.
	layout := generationLayoutPaths(provider.config.SandboxRoot, spec)
	require.NoError(t, os.MkdirAll(layout.output, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(layout.output, "index.html"), []byte("<html>D1</html>"), 0o644))

	// The Run reaches terminal; the enqueue trigger writes the receipt.
	fence, err := runs.Claim(ctx, admission.Key, "worker-capture", time.Hour)
	require.NoError(t, err)
	require.NoError(t, runs.Finalize(ctx, fence, json.RawMessage(`{"content":"done"}`)))
	var pending string
	require.NoError(t, db.Table("craft_run_captures").Select("state").
		Where("tenant_id = ? AND run_id = ?", 1, "run-capture-a").Scan(&pending).Error)
	require.Equal(t, "pending", pending, "the terminal transition must enqueue the durable capture receipt")

	runner := newCraftRunCaptureRunner(db, newCaptureWiringFiles(t, db),
		repository.NewCraftVersionStore(db),
		&CraftRunViewProductionAssembly{Provider: provider, Store: views})
	require.Empty(t, runner.Unavailable())
	runner.AfterTerminal(ctx, fence)

	var state string
	require.NoError(t, db.Table("craft_run_captures").Select("state").
		Where("tenant_id = ? AND run_id = ?", 1, "run-capture-a").Scan(&state).Error)
	require.Equal(t, "advanced", state, "the post-terminal coordinator must seal and advance the capture")
	var head struct {
		Revision    int64
		State       string
		SourceRunID string `gorm:"column:source_run_id"`
	}
	require.NoError(t, db.Table("craft_workspace_draft_heads").Select("revision, state, source_run_id").
		Where("workspace_id = ?", workspace.ID).Scan(&head).Error)
	require.EqualValues(t, 1, head.Revision)
	require.Equal(t, string(craft.DraftHeadSelected), head.State)
	require.Equal(t, "run-capture-a", head.SourceRunID)

	// The next Run of the same Workspace is admitted: the capture fence is
	// released because the prior terminal capture is advanced.
	next := captureWiringAdmission("run-capture-b")
	_, err = runs.Admit(ctx, next)
	require.NoError(t, err, "an advanced capture must unblock the next Run of the Workspace")
}

// TestCraftRunCaptureRunnerStaysInertWithoutRunViewAssembly pins the
// fail-closed half: without the RunView production assembly the runner is
// inert and draining is a logged no-op.
func TestCraftRunCaptureRunnerStaysInertWithoutRunViewAssembly(t *testing.T) {
	runner := newCraftRunCaptureRunner(nil, nil, nil, nil)
	require.NotEmpty(t, runner.Unavailable())
	require.NotPanics(t, func() { runner.Recover(context.Background(), 10) })
	require.NotPanics(t, func() { runner.AfterTerminal(context.Background(), agentruntime.Fence{}) })
}

// TestCraftRunViewExecuteStagesPrivateCandidateNotVersion drives the Execute
// success tail's Run-bound candidate seam directly: the collected output
// becomes a private candidate bound to the Run and generation, and no
// Version is published.
func TestCraftRunViewExecuteStagesPrivateCandidateNotVersion(t *testing.T) {
	db := wiringTestDB(t)
	seedCraftCaptureWiringDB(t, db)
	ctx := context.Background()
	_, err := repository.NewCraftStore(db).PutWorkspace(ctx, craft.Workspace{
		Scope:     craft.Scope{TenantID: 1, UserID: "u-wiring", SessionID: "s-wiring"},
		SandboxID: "sbx-candidate", Generation: "0",
		OpenCodeSessionID: "oc-candidate", RuntimeDigest: "sha256:runtime",
	}, 0)
	require.NoError(t, err)
	runs := repository.NewAgentRunStore(db)
	admission := captureWiringAdmission("run-candidate-a")
	_, err = runs.Admit(ctx, admission)
	require.NoError(t, err)

	t.Setenv(craftOpenCodeBaseURLEnv, "http://127.0.0.1:1")
	t.Setenv(craftOpenCodeWorkDirEnv, t.TempDir())
	files := newCaptureWiringFiles(t, db)
	executor, err := newCraftRuntimeExecutor(db, repository.NewCraftStore(db), runs,
		files, repository.NewCraftVersionStore(db), nil, nil, nil)
	require.NoError(t, err)
	runtimeExec, ok := executor.(*localCraftRuntime)
	require.True(t, ok)

	engine := newFakeCraftRunViewContainerEngine()
	provider, _ := newRVTestProviderWithSessionAPI(t, engine)
	views := repository.NewCraftRunViewStore(db)
	key := craft.RunViewKey{TenantID: 1, OwnerID: "u-wiring", SessionID: "s-wiring", RunID: "run-candidate-a"}
	view, err := views.Allocate(ctx, key)
	require.NoError(t, err)
	spec := rvTestSpec(view.Generation)
	container, err := provider.InspectOrCreateContainer(ctx, spec)
	require.NoError(t, err)
	const sessionID = "ses_0123456789ab0123456789ABCE"
	_, _, err = views.BeginSessionCreate(ctx, key, view.Generation)
	require.NoError(t, err)
	bound, err := views.BindRuntime(ctx, key, view.Generation, runtimeForSession(container, CraftRunViewRuntimeSession{
		ID: sessionID, ProjectID: container.ProjectID, Directory: container.Directory,
	}))
	require.NoError(t, err)

	layout := generationLayoutPaths(provider.config.SandboxRoot, spec)
	require.NoError(t, os.MkdirAll(layout.output, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(layout.output, "index.html"), []byte("<html>C1</html>"), 0o644))

	material, err := provider.MaterialHandleForCapture(ctx, views, key)
	require.NoError(t, err)
	require.Equal(t, bound.Generation, material.generation)

	var epoch int64
	claimFence, err := runs.Claim(ctx, admission.Key, "worker-candidate", time.Hour)
	require.NoError(t, err)
	epoch = claimFence.Epoch
	task := craft.Task{
		Scope:       craft.Scope{TenantID: 1, UserID: "u-wiring", SessionID: "s-wiring"},
		WorkspaceID: materialWorkspaceID(t, db, "s-wiring"),
		Fence:       agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 1, RunID: "run-candidate-a"}, Epoch: epoch},
	}
	require.NoError(t, runtimeExec.stageRunViewCandidate(ctx, task, material))

	var candidates int64
	require.NoError(t, db.Table("craft_candidates").
		Where("tenant_id = ? AND run_id = ?", 1, "run-candidate-a").Count(&candidates).Error)
	require.EqualValues(t, 1, candidates, "the Run-bound collection must stage exactly one private candidate")
	var versions int64
	require.NoError(t, db.Table("craft_versions").
		Where("workspace_id = ?", materialWorkspaceID(t, db, "s-wiring")).Count(&versions).Error)
	require.Zero(t, versions, "candidate staging must never publish a Version")
}

func materialWorkspaceID(t *testing.T, db *gorm.DB, sessionID string) string {
	t.Helper()
	var id string
	require.NoError(t, db.Table("craft_workspaces").Select("id").
		Where("tenant_id = ? AND session_id = ?", 1, sessionID).Take(&id).Error)
	return id
}
