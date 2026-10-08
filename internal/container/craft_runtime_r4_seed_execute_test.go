package container

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestCraftRunViewR4ExecuteSeedsFrozenDraftIntoPrivateOutput(t *testing.T) {
	d1 := []byte("draft from failed Run A")
	f := newR4SeedExecuteFixture(t, d1, 3, nil, true)
	require.NoError(t, f.execute())
	require.Equal(t, 1, f.executor.calls)
	require.Equal(t, 1, f.files.reads["draft-d1"])
	require.Zero(t, f.files.reads["draft-d2"], "a later Workspace head cannot replace the admitted D1 predecessor")
	got, err := os.ReadFile(filepath.Join(f.material.output, "index.html"))
	require.NoError(t, err)
	require.Equal(t, d1, got)
	require.NoError(t, f.execute(), "identical retry must accept the already seeded frozen predecessor")
	require.Equal(t, 2, f.executor.calls)
}

func TestCraftRunViewR4ExecuteStartsExplicitEmptyDraftWithEmptyOutput(t *testing.T) {
	f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
	require.NoError(t, f.execute())
	require.Equal(t, 1, f.executor.calls)
	entries, err := os.ReadDir(f.material.output)
	require.NoError(t, err)
	require.Empty(t, entries, "revision zero explicitly seeds an empty private output")
}

func TestCraftRunViewR4ExecuteCompletesPartialFrozenSeedAndRejectsChangedRetry(t *testing.T) {
	f := newR4SeedExecuteFixture(t, []byte("frozen index"), 3, nil, true)
	addR4SecondFrozenFile(t, f, []byte("frozen stylesheet"))
	require.NoError(t, os.WriteFile(filepath.Join(f.material.output, "index.html"), []byte("frozen index"), 0o644))

	_, err := f.runtime.Execute(context.Background(), f.task)
	require.NoError(t, err)
	require.Equal(t, 1, f.executor.calls)
	css, err := os.ReadFile(filepath.Join(f.material.output, "site.css"))
	require.NoError(t, err)
	require.Equal(t, []byte("frozen stylesheet"), css)

	require.NoError(t, os.WriteFile(filepath.Join(f.material.output, "site.css"), []byte("changed after B failed"), 0o644))
	_, err = f.runtime.Execute(context.Background(), f.task)
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Equal(t, 1, f.executor.calls, "changed private bytes cannot reach retry dispatch")
}

func TestCraftRunViewR4FailedBPreservesPersistedDefaultVersion(t *testing.T) {
	f := newR4SeedExecuteFixture(t, []byte("draft for failed B"), 3, nil, true)
	require.NoError(t, f.db.Exec(`CREATE TABLE craft_versions (
		id TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL, workspace_id TEXT NOT NULL, run_id TEXT NOT NULL,
		kind TEXT NOT NULL, manifest_hash TEXT NOT NULL, checks_json TEXT NOT NULL DEFAULT '[]', created_at DATETIME NOT NULL)`).Error)
	require.NoError(t, f.db.Exec(`CREATE TABLE craft_version_files (
		version_id TEXT NOT NULL, path TEXT NOT NULL, resource_ref TEXT NOT NULL, file_hash TEXT NOT NULL,
		file_bytes INTEGER NOT NULL, mime TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`).Error)
	v1Bytes := []byte("published V1 remains default")
	v1File := craft.File{Path: "index.html", Ref: "v1-object", SHA256: shaHex(v1Bytes), Bytes: int64(len(v1Bytes)), MIME: "text/html"}
	v1 := craft.Version{ID: craft.VersionID("workspace-b", "run-v1", mustManifestDigest(t, []craft.File{v1File})),
		WorkspaceID: "workspace-b", RunID: "run-v1", Kind: "web", Files: []craft.File{v1File}}
	// VersionID includes the manifest digest, not a default pointer. The
	// session view defines its current/default version as the newest listed row.
	require.NoError(t, f.db.Exec(`INSERT INTO craft_versions(id, tenant_id, workspace_id, run_id, kind, manifest_hash, checks_json, created_at)
		VALUES (?, 1, ?, ?, ?, ?, '[]', '2026-01-01T00:00:00Z')`, v1.ID, v1.WorkspaceID, v1.RunID, v1.Kind, mustManifestDigest(t, v1.Files)).Error)
	require.NoError(t, f.db.Exec(`INSERT INTO craft_version_files(version_id, path, resource_ref, file_hash, file_bytes, mime)
		VALUES (?, ?, ?, ?, ?, ?)`, v1.ID, v1File.Path, v1File.Ref, v1File.SHA256, v1File.Bytes, v1File.MIME).Error)
	f.files.blobs[v1File.Ref] = append([]byte(nil), v1Bytes...)
	versionStore := repository.NewCraftVersionStore(f.db)
	before, err := versionStore.List(context.Background(), f.task.Scope)
	require.NoError(t, err)
	require.Len(t, before, 1)
	require.Equal(t, v1.ID, before[0].ID)
	require.NoError(t, f.execute())
	require.Equal(t, 1, f.executor.calls)
	after, err := versionStore.List(context.Background(), f.task.Scope)
	require.NoError(t, err)
	require.Len(t, after, 1, "failed B must not create or displace a published version")
	require.Equal(t, v1.ID, after[0].ID, "V1 remains the session's current/default version")
	got, err := f.files.GetFile(context.Background(), v1File.Ref)
	require.NoError(t, err)
	content, err := io.ReadAll(got)
	require.NoError(t, err)
	require.NoError(t, got.Close())
	require.Equal(t, v1Bytes, content)
}

func TestCraftRunViewR4ExecuteAcceptsLegalDraftOverInputReadBudget(t *testing.T) {
	draft := bytesOfSize(21<<20, 'x')
	f := newR4SeedExecuteFixture(t, draft, 3, nil, true)
	require.NoError(t, f.execute())
	require.Equal(t, 1, f.executor.calls)
	got, err := os.ReadFile(filepath.Join(f.material.output, "index.html"))
	require.NoError(t, err)
	require.Equal(t, int64(len(draft)), int64(len(got)))
	require.Equal(t, sha256.Sum256(draft), sha256.Sum256(got))
}

func TestCraftRunViewR4ExecuteRejectsDraftOverFiftyMiBBeforeFetch(t *testing.T) {
	draft := bytesOfSize((50<<20)+1, 'x')
	f := newR4SeedExecuteFixture(t, draft, 3, nil, true)
	err := f.execute()
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	require.Zero(t, f.files.reads["draft-d1"], "oversize manifests must fail before object fetch")
	require.Zero(t, f.executor.calls)
}

func TestCraftRunViewR4ExecuteRechecksFenceAfterObjectFetch(t *testing.T) {
	d1 := []byte("frozen D1")
	var f *r4SeedExecuteFixture
	f = newR4SeedExecuteFixture(t, d1, 3, func(string) {
		require.NoError(t, f.db.Exec("UPDATE agent_runs SET epoch = 4 WHERE tenant_id = 1 AND run_id = 'run-b'").Error)
	}, true)
	err := f.execute()
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Zero(t, f.executor.calls, "lost writer fence must not reach dispatch")
}

func TestCraftRunViewR4ExecuteRechecksFenceImmediatelyBeforeDispatch(t *testing.T) {
	f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
	base := f.material.provider.engine
	f.material.provider.engine = &r4EpochMutatingEngine{CraftRunViewContainerEngine: base, onInspect: func() {
		require.NoError(t, f.db.Exec("UPDATE agent_runs SET epoch = 4 WHERE tenant_id = 1 AND run_id = 'run-b'").Error)
	}}
	err := f.execute()
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Zero(t, f.executor.calls, "epoch loss during final material revalidation must stop before prompt")
}

func TestCraftRunViewR4ExecuteRejectsWrongWorkspaceBeforeDispatch(t *testing.T) {
	f := newR4SeedExecuteFixture(t, []byte("D1"), 3, nil, true)
	f.task.WorkspaceID = "foreign-workspace"
	err := f.execute()
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Zero(t, f.executor.calls)
}

func TestCraftRunViewR4ExecuteRejectsWrongRunAndObjectBytesBeforeDispatch(t *testing.T) {
	t.Run("foreign Run", func(t *testing.T) {
		f := newR4SeedExecuteFixture(t, []byte("D1"), 3, nil, true)
		f.task.Fence.RunID = "foreign-run"
		err := f.execute()
		require.Error(t, err)
		require.Zero(t, f.executor.calls)
	})
	t.Run("mismatched object bytes", func(t *testing.T) {
		f := newR4SeedExecuteFixture(t, []byte("D1"), 3, nil, true)
		f.files.blobs["draft-d1"] = []byte("D1 tampered")
		err := f.execute()
		require.ErrorIs(t, err, craft.ErrInvalidInput)
		require.Zero(t, f.executor.calls)
	})
	t.Run("stale epoch", func(t *testing.T) {
		f := newR4SeedExecuteFixture(t, []byte("D1"), 3, nil, true)
		f.task.Fence.Epoch = 4
		err := f.execute()
		require.ErrorIs(t, err, craft.ErrConflict)
		require.Zero(t, f.executor.calls)
	})
}

func TestCraftRunViewR4DuplicateMessageAdoptionStillChecksFence(t *testing.T) {
	f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
	f.task.Fence.Epoch = 4
	adopted, err := f.runtime.adoptExecutorMessageID(context.Background(), f.task)
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Empty(t, adopted.PromptMessageID)
}

func TestCraftRunViewR4MessageAdoptionUpdatesOnlyUnderCurrentEpoch(t *testing.T) {
	f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
	f.task.PromptMessageID = "uuid-original"
	insertR4Delegation(t, f, f.task, "uuid-original")
	adopted, err := f.runtime.adoptExecutorMessageID(context.Background(), f.task)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(adopted.PromptMessageID, "msg_"))
	var persisted string
	require.NoError(t, f.db.Table("craft_delegations").Select("prompt_message_id").Where("tenant_id = 1 AND run_id = 'run-b' AND tool_call_id = 'call-b'").Scan(&persisted).Error)
	require.Equal(t, adopted.PromptMessageID, persisted)
	f.task.Fence.Epoch = 4
	if _, err := f.runtime.adoptExecutorMessageID(context.Background(), f.task); !errors.Is(err, craft.ErrConflict) {
		t.Fatalf("duplicate adoption after epoch loss = %v, want conflict", err)
	}
	require.NoError(t, f.db.Table("craft_delegations").Select("prompt_message_id").Where("tenant_id = 1 AND run_id = 'run-b' AND tool_call_id = 'call-b'").Scan(&persisted).Error)
	require.Equal(t, adopted.PromptMessageID, persisted, "stale retry must not mutate the durable delegation")
}

func TestCraftRunViewR4MessageAdoptionSameRequestRetryReusesPersistedID(t *testing.T) {
	f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
	f.task.PromptMessageID = "uuid-original"
	insertR4Delegation(t, f, f.task, "uuid-original")
	winner, err := f.runtime.adoptExecutorMessageID(context.Background(), f.task)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(winner.PromptMessageID, "msg_"))

	// The retry re-enters with the original prepared request and UUID. It must
	// reload the durable winner instead of minting a replacement ID.
	retry, err := f.runtime.adoptExecutorMessageID(context.Background(), f.task)
	require.NoError(t, err)
	require.Equal(t, winner.PromptMessageID, retry.PromptMessageID)
	var persisted string
	require.NoError(t, f.db.Table("craft_delegations").Select("prompt_message_id").Where(
		"tenant_id = 1 AND run_id = 'run-b' AND tool_call_id = 'call-b'").Scan(&persisted).Error)
	require.Equal(t, winner.PromptMessageID, persisted)
}

func TestCraftRunViewR4ConcurrentMessageAdoptionUsesOnePersistedID(t *testing.T) {
	f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
	f.task.PromptMessageID = "uuid-original"
	insertR4Delegation(t, f, f.task, "uuid-original")
	start := make(chan struct{})
	results := make([]craft.Task, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = f.runtime.adoptExecutorMessageID(context.Background(), f.task)
		}(i)
	}
	close(start)
	wg.Wait()
	for i := range errs {
		require.NoError(t, errs[i])
		require.True(t, strings.HasPrefix(results[i].PromptMessageID, "msg_"))
	}
	require.Equal(t, results[0].PromptMessageID, results[1].PromptMessageID)
}

func TestCraftRunViewR4MessageAdoptionRejectsChangedRequest(t *testing.T) {
	f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
	f.task.PromptMessageID = "uuid-original"
	insertR4Delegation(t, f, f.task, "uuid-original")
	var before string
	require.NoError(t, f.db.Table("craft_delegations").Select("task_json").Where(
		"tenant_id = 1 AND run_id = 'run-b' AND tool_call_id = 'call-b'").Scan(&before).Error)

	changed := f.task
	changed.Prompt = "different request under same tool call"
	changed.PromptMessageID = "uuid-original"
	_, err := f.runtime.Execute(context.Background(), changed)
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Zero(t, f.executor.calls, "changed immutable request must not reach inner prompt dispatch")
	var after, persisted string
	require.NoError(t, f.db.Table("craft_delegations").Select("task_json").Where(
		"tenant_id = 1 AND run_id = 'run-b' AND tool_call_id = 'call-b'").Scan(&after).Error)
	require.NoError(t, f.db.Table("craft_delegations").Select("prompt_message_id").Where(
		"tenant_id = 1 AND run_id = 'run-b' AND tool_call_id = 'call-b'").Scan(&persisted).Error)
	require.Equal(t, before, after, "a colliding request must not rewrite the immutable request")
	require.Equal(t, "uuid-original", persisted)
}

func insertR4Delegation(t *testing.T, f *r4SeedExecuteFixture, task craft.Task, id string) {
	t.Helper()
	raw, err := json.Marshal(task)
	require.NoError(t, err)
	require.NoError(t, f.db.Exec(`DELETE FROM craft_delegations WHERE tenant_id = ? AND run_id = ? AND tool_call_id = ?`,
		task.Fence.TenantID, task.Fence.RunID, task.ToolCallID).Error)
	require.NoError(t, f.db.Exec(`INSERT INTO craft_delegations(tenant_id, run_id, tool_call_id, workspace_id, task_json, prompt_message_id)
		VALUES (?, ?, ?, ?, ?, ?)`, task.Fence.TenantID, task.Fence.RunID, task.ToolCallID,
		task.WorkspaceID, string(raw), id).Error)
}

func addR4SecondFrozenFile(t *testing.T, f *r4SeedExecuteFixture, content []byte) {
	t.Helper()
	second := craft.File{Path: "site.css", Ref: "draft-css", SHA256: shaHex(content), Bytes: int64(len(content)), MIME: "text/css"}
	f.files.blobs[second.Ref] = append([]byte(nil), content...)
	var first craft.File
	require.NoError(t, f.db.Table("craft_workspace_draft_files").Where(
		"workspace_id = ? AND revision = 1", f.task.WorkspaceID).Take(&first).Error)
	files := []craft.File{first, second}
	digest := mustManifestDigest(t, files)
	require.NoError(t, f.db.Exec(`INSERT INTO craft_workspace_draft_files(workspace_id, revision, path, object_ref, sha256, bytes, mime)
		VALUES (?, 1, ?, ?, ?, ?, ?)`, f.task.WorkspaceID, second.Path, second.Ref, second.SHA256, second.Bytes, second.MIME).Error)
	require.NoError(t, f.db.Exec(`UPDATE craft_workspace_draft_revisions SET manifest_digest = ? WHERE workspace_id = ? AND revision = 1`, digest, f.task.WorkspaceID).Error)
	var raw string
	require.NoError(t, f.db.Table("agent_runs").Select("snapshot").Where("tenant_id = 1 AND run_id = 'run-b'").Scan(&raw).Error)
	var snapshot map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &snapshot))
	seed, ok := snapshot["craft_workspace_seed"].(map[string]any)
	require.True(t, ok)
	seed["manifest_digest"] = digest
	encoded, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.NoError(t, f.db.Exec("UPDATE agent_runs SET snapshot = ? WHERE tenant_id = 1 AND run_id = 'run-b'", string(encoded)).Error)
}

func shaHex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func mustManifestDigest(t *testing.T, files []craft.File) string {
	t.Helper()
	digest, err := craft.ManifestDigest(files)
	require.NoError(t, err)
	return digest
}

type r4SeedExecuteFixture struct {
	db       *gorm.DB
	runtime  *localCraftRuntime
	task     craft.Task
	material CraftRunViewMaterialHandle
	executor *r4SeedFailingExecutor
	files    craftRuntimeInputFiles
}

func (f *r4SeedExecuteFixture) execute() error {
	_, err := f.runtime.Execute(context.Background(), f.task)
	return err
}

type r4SeedFailingExecutor struct{ calls int }

func (e *r4SeedFailingExecutor) Execute(context.Context, craft.Task) (craft.Result, error) {
	e.calls++
	return craft.Result{Status: "failed"}, nil
}
func (*r4SeedFailingExecutor) Observe(context.Context, craft.Task) (craft.Observation, error) {
	return craft.Observation{}, nil
}
func (*r4SeedFailingExecutor) Abort(context.Context, craft.Task) error { return nil }

type r4EpochMutatingEngine struct {
	CraftRunViewContainerEngine
	onInspect func()
}

func (e *r4EpochMutatingEngine) InspectContainer(ctx context.Context, name string) (*CraftRunViewEngineContainer, error) {
	if e.onInspect != nil {
		e.onInspect()
	}
	return e.CraftRunViewContainerEngine.InspectContainer(ctx, name)
}

func newR4SeedExecuteFixture(t *testing.T, draft []byte, epoch int64, onGet func(string), selected bool) *r4SeedExecuteFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "r4-seed.db")+"?mode=memory&cache=shared"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, session_id TEXT NOT NULL, owner_id TEXT NOT NULL,
		status TEXT NOT NULL, epoch INTEGER NOT NULL, snapshot TEXT NOT NULL DEFAULT '', PRIMARY KEY (tenant_id, run_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE craft_workspaces (
		id TEXT NOT NULL PRIMARY KEY, tenant_id INTEGER NOT NULL, owner_id TEXT NOT NULL, session_id TEXT NOT NULL,
		sandbox_id TEXT NOT NULL DEFAULT '', generation TEXT NOT NULL DEFAULT '', oc_session_id TEXT NOT NULL DEFAULT 'oc-bound',
		runtime_digest TEXT NOT NULL DEFAULT '', revision INTEGER NOT NULL DEFAULT 1, UNIQUE (tenant_id, id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE craft_workspace_inputs (
		workspace_id TEXT, tenant_id INTEGER, ref TEXT, name TEXT, sha256 TEXT, bytes INTEGER, created_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE craft_workspace_draft_heads (
		workspace_id TEXT, tenant_id INTEGER, revision INTEGER, state TEXT, source_run_id TEXT, manifest_digest TEXT)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE craft_workspace_draft_origins (
		workspace_id TEXT, tenant_id INTEGER, origin_revision INTEGER, origin_state TEXT)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE craft_workspace_draft_revisions (
		workspace_id TEXT, revision INTEGER, tenant_id INTEGER, source_run_id TEXT, manifest_digest TEXT)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE craft_workspace_draft_files (
		workspace_id TEXT, revision INTEGER, path TEXT, object_ref TEXT, sha256 TEXT, bytes INTEGER, mime TEXT)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE craft_delegations (
		tenant_id INTEGER, run_id TEXT, tool_call_id TEXT, workspace_id TEXT, task_json TEXT, prompt_message_id TEXT)`).Error)

	scope := craft.Scope{TenantID: 1, UserID: "owner", SessionID: "session-b"}
	const workspaceID = "workspace-b"
	task := craft.Task{ID: "delegation-b", ToolCallID: "call-b", Prompt: "continue draft", PromptMessageID: "msg_r4_retry",
		Scope: scope, Fence: agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 1, RunID: "run-b"}, Owner: "worker", Epoch: epoch},
		WorkspaceID: workspaceID}
	taskJSON, err := json.Marshal(task)
	require.NoError(t, err)
	require.NoError(t, db.Exec(`INSERT INTO craft_delegations(tenant_id, run_id, tool_call_id, workspace_id, task_json, prompt_message_id)
		VALUES (1, 'run-b', 'call-b', ?, ?, ?)`, workspaceID, string(taskJSON), task.PromptMessageID).Error)
	require.NoError(t, db.Exec(`INSERT INTO craft_workspaces(id, tenant_id, owner_id, session_id, oc_session_id) VALUES (?, 1, 'owner', 'session-b', 'oc-bound')`, workspaceID).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs(tenant_id, run_id, session_id, owner_id, status, epoch, snapshot) VALUES
		(1, 'run-a', 'session-b', 'owner', 'failed', 2, ''),
		(1, 'run-c', 'session-b', 'owner', 'failed', 2, ''),
		(1, 'run-b', 'session-b', 'owner', 'running', ?, '')`, epoch).Error)
	var files []craft.File
	if selected {
		sum := sha256.Sum256(draft)
		files = []craft.File{{Path: "index.html", Ref: "draft-d1", SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(draft)), MIME: "text/html"}}
	}
	digest, err := craft.ManifestDigest(files)
	require.NoError(t, err)
	draft2 := []byte("later D2")
	sum2 := sha256.Sum256(draft2)
	files2 := []craft.File{{Path: "index.html", Ref: "draft-d2", SHA256: hex.EncodeToString(sum2[:]), Bytes: int64(len(draft2)), MIME: "text/html"}}
	digest2, err := craft.ManifestDigest(files2)
	require.NoError(t, err)
	state := craft.DraftHeadSelected
	seed := service.CraftWorkspaceSeedSnapshot{WorkspaceID: workspaceID, State: state, DraftRevision: 1, SourceRunID: "run-a", ManifestDigest: digest}
	if !selected {
		seed = service.CraftWorkspaceSeedSnapshot{WorkspaceID: workspaceID, State: craft.DraftHeadEmpty, DraftRevision: 0}
	}
	raw, err := service.BuildDurableCraftRunSnapshot("original query", nil, "model-1", "", &types.AgentConfig{}, []craft.Input{})
	require.NoError(t, err)
	var snapshot map[string]any
	require.NoError(t, json.Unmarshal(raw, &snapshot))
	snapshot["craft_workspace_seed"] = seed
	raw, err = json.Marshal(snapshot)
	require.NoError(t, err)
	require.NoError(t, db.Exec("UPDATE agent_runs SET snapshot = ? WHERE tenant_id = 1 AND run_id = 'run-b'", string(raw)).Error)
	require.NoError(t, db.Exec(`INSERT INTO craft_workspace_draft_origins(workspace_id, tenant_id, origin_revision, origin_state) VALUES (?, 1, 0, 'empty')`, workspaceID).Error)
	if selected {
		require.NoError(t, db.Exec(`INSERT INTO craft_workspace_draft_heads(workspace_id, tenant_id, revision, state, source_run_id, manifest_digest) VALUES (?, 1, 2, 'selected', 'run-c', ?)`, workspaceID, digest2).Error)
		for _, revision := range []struct {
			number int
			runID  string
			files  []craft.File
			digest string
		}{{1, "run-a", files, digest}, {2, "run-c", files2, digest2}} {
			require.NoError(t, db.Exec(`INSERT INTO craft_workspace_draft_revisions(workspace_id, revision, tenant_id, source_run_id, manifest_digest) VALUES (?, ?, 1, ?, ?)`, workspaceID, revision.number, revision.runID, revision.digest).Error)
			for _, file := range revision.files {
				require.NoError(t, db.Exec(`INSERT INTO craft_workspace_draft_files(workspace_id, revision, path, object_ref, sha256, bytes, mime) VALUES (?, ?, ?, ?, ?, ?, ?)`, workspaceID, revision.number, file.Path, file.Ref, file.SHA256, file.Bytes, file.MIME).Error)
			}
		}
	} else {
		require.NoError(t, db.Exec(`INSERT INTO craft_workspace_draft_heads(workspace_id, tenant_id, revision, state) VALUES (?, 1, 0, 'empty')`, workspaceID).Error)
	}

	material := newCraftRuntimeTestMaterial(t, task, materialTestGeneration(t))
	knowledgeRoot := filepath.Join(material.knowledge, "runs")
	require.NoError(t, os.MkdirAll(filepath.Join(knowledgeRoot, task.Fence.RunID), 0o755))
	require.NoError(t, os.Chmod(filepath.Join(knowledgeRoot, task.Fence.RunID), 0o555))
	filesService := craftRuntimeInputFiles{blobs: map[string][]byte{"draft-d1": draft, "draft-d2": draft2}, reads: map[string]int{}, onGet: onGet}
	executor := &r4SeedFailingExecutor{}
	runtime := &localCraftRuntime{
		db: db, store: repository.NewCraftStore(db), files: filesService, inner: executor,
		workDir: t.TempDir(), outputDir: "output",
		materialResolver: func(context.Context, craft.Task) (CraftRunViewMaterialHandle, error) { return material, nil },
		knowledgeResolver: func(_ context.Context, task craft.Task, _ CraftRunViewMaterialHandle) (CraftKnowledgeRunViewAcceptance, error) {
			return CraftKnowledgeRunViewAcceptance{RunID: task.Fence.RunID, PackageDigest: strings.Repeat("a", 64)}, nil
		},
		knowledgeVerifier: func(context.Context, craft.Task, CraftRunViewMaterialHandle, CraftKnowledgeRunViewAcceptance) error {
			return nil
		},
	}
	return &r4SeedExecuteFixture{db: db, runtime: runtime, task: task, material: material, executor: executor, files: filesService}
}

func bytesOfSize(n int, b byte) []byte { return []byte(strings.Repeat(string([]byte{b}), n)) }
