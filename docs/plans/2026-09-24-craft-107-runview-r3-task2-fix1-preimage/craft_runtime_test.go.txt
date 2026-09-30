// Package container tests for the W06 env-driven craft runtime assembly
// (coordinator-authorized extension): fail-closed default, prompt framing,
// session-scoped artifact source and the craft event emitter payload.
package container

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/opencode"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type craftRuntimeInputFiles struct {
	interfaces.FileService
	blobs map[string][]byte
	reads map[string]int
}

func (f craftRuntimeInputFiles) GetFile(_ context.Context, ref string) (io.ReadCloser, error) {
	if f.reads != nil {
		f.reads[ref]++
	}
	data, ok := f.blobs[ref]
	if !ok {
		return nil, fmt.Errorf("missing blob %s", ref)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func TestStageWorkspaceInputsUsesOnlySelectedTaskSnapshot(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "inputs.db")+"?mode=memory&cache=shared"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE craft_workspace_inputs (
		workspace_id TEXT, tenant_id INTEGER, ref TEXT, name TEXT, sha256 TEXT, bytes INTEGER, created_at DATETIME
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE craft_workspaces (id TEXT, tenant_id INTEGER, owner_id TEXT, session_id TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO craft_workspaces (id, tenant_id, owner_id, session_id) VALUES ('ws-inputs', 1, 'owner', 'task-session')`).Error; err != nil {
		t.Fatal(err)
	}
	selectedBytes, excludedBytes := []byte("selected payload"), []byte("excluded payload")
	digest := func(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
	for _, row := range []struct {
		ref, name string
		data      []byte
	}{
		{"ref-selected", "selected.json", selectedBytes},
		{"ref-excluded", "excluded.bin", excludedBytes},
	} {
		if err := db.Exec(`INSERT INTO craft_workspace_inputs (workspace_id, tenant_id, ref, name, sha256, bytes, created_at)
			VALUES (?, 1, ?, ?, ?, ?, CURRENT_TIMESTAMP)`, "ws-inputs", row.ref, row.name, digest(row.data), len(row.data)).Error; err != nil {
			t.Fatal(err)
		}
	}
	selected := []craft.Input{{Ref: "ref-selected", Name: "selected.json", SHA256: digest(selectedBytes), Bytes: int64(len(selectedBytes))}}
	snapshot, err := json.Marshal(map[string]any{
		"version": 1, "query": "Build from selected input", "model_id": "model-1",
		"agent_config": json.RawMessage(`{}`), "runtime": map[string]any{}, "craft_input_manifest": selected,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE agent_runs (tenant_id INTEGER, run_id TEXT, session_id TEXT, owner_id TEXT, snapshot TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, snapshot) VALUES (1, 'run-layout', 'task-session', 'owner', ?)`, string(snapshot)).Error; err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	files := craftRuntimeInputFiles{blobs: map[string][]byte{
		"ref-selected": selectedBytes, "ref-excluded": excludedBytes,
	}, reads: map[string]int{}}
	runtime := &localCraftRuntime{db: db, files: files, workDir: work}
	scope := craft.Scope{TenantID: 1, UserID: "owner", SessionID: "task-session"}
	task := craft.Task{Scope: scope, Fence: agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 1, RunID: "run-layout"}}, WorkspaceID: "ws-inputs"}
	material := newCraftRuntimeTestMaterial(t, task, "generation-selected-inputs")
	if err := runtime.stageWorkspaceInputs(context.Background(), task, material); err != nil {
		t.Fatal(err)
	}
	selectedPath := filepath.Join(material.inputs, digest(selectedBytes), "selected.json")
	got, err := os.ReadFile(selectedPath)
	if err != nil || !bytes.Equal(got, selectedBytes) {
		t.Fatalf("selected snapshot input not staged: %q, %v", got, err)
	}
	excludedPath := filepath.Join(material.inputs, digest(excludedBytes), "excluded.bin")
	if _, err := os.Stat(excludedPath); !os.IsNotExist(err) {
		t.Fatalf("unselected accumulated input must not be staged, stat error=%v", err)
	}
	if err := runtime.stageWorkspaceInputs(context.Background(), task, material); err != nil {
		t.Fatalf("unchanged verified staged input should remain reusable: %v", err)
	}
	if files.reads["ref-selected"] != 1 {
		t.Fatalf("unchanged staged input should use the verified fast path, GetFile reads=%d", files.reads["ref-selected"])
	}
	corrupt := bytes.Repeat([]byte("x"), len(selectedBytes))
	if bytes.Equal(corrupt, selectedBytes) {
		t.Fatal("test corruption must differ from the admitted bytes")
	}
	if err := os.WriteFile(selectedPath, corrupt, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runtime.stageWorkspaceInputs(context.Background(), task, material); !errors.Is(err, craft.ErrConflict) {
		t.Fatalf("same-sized corrupt staged input must fail closed, got %v", err)
	}
	task.Inputs = []craft.Input{{Ref: "ref-selected", Name: "selected.json", SHA256: digest(excludedBytes), Bytes: int64(len(excludedBytes))}}
	if err := runtime.stageWorkspaceInputs(context.Background(), task, material); !errors.Is(err, craft.ErrForbidden) {
		t.Fatalf("delegate inputs outside the admitted Run snapshot must fail closed, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(work, "inputs")); !os.IsNotExist(err) {
		t.Fatalf("RunView input staging must not write under shared workDir, stat error=%v", err)
	}
}

func TestNewCraftRuntimeExecutorFailsClosedWithoutEnv(t *testing.T) {
	t.Setenv("CRAFT_OPENCODE_BASE_URL", "")
	executor, err := newCraftRuntimeExecutor(nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unset base url must keep the fail-closed executor, got %v", err)
	}
	if _, err := executor.Execute(context.Background(), craft.Task{}); !errors.Is(err, craft.ErrUnsupported) {
		t.Fatalf("default executor must fail closed with ErrUnsupported, got %v", err)
	}
}

func TestNewCraftRuntimeExecutorRequiresWorkDir(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	t.Setenv("CRAFT_OPENCODE_BASE_URL", server.URL)
	t.Setenv("CRAFT_OPENCODE_WORK_DIR", "")
	if _, err := newCraftRuntimeExecutor(nil, nil, nil, nil, nil, nil); err == nil {
		t.Fatal("a base url without a work dir must refuse to assemble")
	}
}

func TestRunViewInputStagingFailsClosedOnUnboundRuntime(t *testing.T) {
	runtime := &localCraftRuntime{workDir: t.TempDir()}
	_, err := runtime.Execute(context.Background(), craft.Task{})
	if !errors.Is(err, ErrCraftRunViewRuntimeUnresolved) {
		t.Fatalf("RunView material resolution must fail closed before shared workspace writes, got %v", err)
	}
}

func TestRunViewInputStagingRejectsEmptyExtrasRevokedScopeAndUnsafeObjects(t *testing.T) {
	payload := []byte("authorized source")
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])
	input := craft.Input{Ref: "ref-one", Name: "input.txt", SHA256: digest, Bytes: int64(len(payload))}
	tests := []struct {
		name   string
		setup  func(*testing.T, *runViewInputFixture)
		mutate func(*testing.T, *runViewInputFixture) error
		want   error
	}{
		{name: "empty manifest stages empty mount", setup: func(_ *testing.T, f *runViewInputFixture) {
			f.manifest = nil
		}, mutate: func(_ *testing.T, f *runViewInputFixture) error {
			return f.runtime.stageWorkspaceInputs(context.Background(), f.task, f.material)
		}},
		{name: "empty manifest rejects extra", setup: func(t *testing.T, f *runViewInputFixture) {
			f.manifest = nil
			require.NoError(t, os.WriteFile(filepath.Join(f.material.inputs, "extra.txt"), []byte("extra"), 0o644))
		}, mutate: func(_ *testing.T, f *runViewInputFixture) error {
			return f.runtime.stageWorkspaceInputs(context.Background(), f.task, f.material)
		}, want: craft.ErrConflict},
		{name: "revoked workspace association", setup: func(t *testing.T, f *runViewInputFixture) {
			require.NoError(t, f.db.Exec(`DELETE FROM craft_workspaces WHERE id = ?`, f.task.WorkspaceID).Error)
		}, mutate: func(_ *testing.T, f *runViewInputFixture) error {
			return f.runtime.stageWorkspaceInputs(context.Background(), f.task, f.material)
		}, want: craft.ErrForbidden},
		{name: "symlinked digest parent", setup: func(t *testing.T, f *runViewInputFixture) {
			outside := t.TempDir()
			require.NoError(t, os.Symlink(outside, filepath.Join(f.material.inputs, digest)))
		}, mutate: func(_ *testing.T, f *runViewInputFixture) error {
			return f.runtime.stageWorkspaceInputs(context.Background(), f.task, f.material)
		}, want: craft.ErrConflict},
		{name: "symlinked target", setup: func(t *testing.T, f *runViewInputFixture) {
			dir := filepath.Join(f.material.inputs, digest)
			require.NoError(t, os.Mkdir(dir, 0o755))
			outside := filepath.Join(t.TempDir(), "outside.txt")
			require.NoError(t, os.WriteFile(outside, []byte("outside"), 0o644))
			require.NoError(t, os.Symlink(outside, filepath.Join(dir, input.Name)))
		}, mutate: func(_ *testing.T, f *runViewInputFixture) error {
			return f.runtime.stageWorkspaceInputs(context.Background(), f.task, f.material)
		}, want: craft.ErrConflict},
		{name: "cross Run handle", setup: func(_ *testing.T, f *runViewInputFixture) {
			f.task.Fence.RunID = "run-foreign"
		}, mutate: func(_ *testing.T, f *runViewInputFixture) error {
			return f.runtime.stageWorkspaceInputs(context.Background(), f.task, f.material)
		}, want: craft.ErrForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			manifest := []craft.Input{input}
			blobs := map[string][]byte{"ref-one": payload}
			if strings.HasPrefix(tc.name, "empty manifest") {
				manifest, blobs = nil, map[string][]byte{}
			}
			fixture := newRunViewInputFixture(t, manifest, blobs)
			tc.setup(t, fixture)
			err := tc.mutate(t, fixture)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("stage input manifest: %v", err)
				}
				entries, readErr := os.ReadDir(fixture.material.inputs)
				if readErr != nil || len(entries) != 0 {
					t.Fatalf("empty admitted manifest must leave an empty input mount, entries=%v err=%v", entries, readErr)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("stage error = %v, want %v", err, tc.want)
			}
			if tc.name == "revoked workspace association" && fixture.files.reads["ref-one"] != 0 {
				t.Fatalf("revoked input association must be rejected before FileService read, reads=%d", fixture.files.reads["ref-one"])
			}
			if tc.name == "cross Run handle" && fixture.files.reads["ref-one"] != 0 {
				t.Fatalf("cross-Run handle must be rejected before FileService read, reads=%d", fixture.files.reads["ref-one"])
			}
		})
	}
}

func TestRunViewInputStagingVerifiesObjectBytesBeforeWritingAndRollsBack(t *testing.T) {
	good := []byte("valid one")
	bad := []byte("bad bytes")
	goodSum := sha256.Sum256(good)
	second := []byte("valid two")
	secondSum := sha256.Sum256(second)
	tests := []struct {
		name     string
		blobs    map[string][]byte
		manifest []craft.Input
	}{
		{name: "wrong byte count", blobs: map[string][]byte{"one": []byte("too long")}, manifest: []craft.Input{{Ref: "one", Name: "one.txt", SHA256: hex.EncodeToString(goodSum[:]), Bytes: int64(len(good))}}},
		{name: "wrong digest", blobs: map[string][]byte{"one": bad}, manifest: []craft.Input{{Ref: "one", Name: "one.txt", SHA256: hex.EncodeToString(goodSum[:]), Bytes: int64(len(good))}}},
		{name: "later read failure writes nothing", blobs: map[string][]byte{"one": good}, manifest: []craft.Input{
			{Ref: "one", Name: "one.txt", SHA256: hex.EncodeToString(goodSum[:]), Bytes: int64(len(good))},
			{Ref: "missing", Name: "two.txt", SHA256: hex.EncodeToString(secondSum[:]), Bytes: int64(len(second))},
		}},
		{name: "unsafe filename", blobs: map[string][]byte{"one": good}, manifest: []craft.Input{{Ref: "one", Name: "../escape.txt", SHA256: hex.EncodeToString(goodSum[:]), Bytes: int64(len(good))}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newRunViewInputFixture(t, tc.manifest, tc.blobs)
			err := fixture.runtime.stageWorkspaceInputs(context.Background(), fixture.task, fixture.material)
			if !errors.Is(err, craft.ErrInvalidInput) && tc.name != "later read failure writes nothing" {
				t.Fatalf("stage error = %v, want ErrInvalidInput", err)
			}
			if tc.name == "later read failure writes nothing" && err == nil {
				t.Fatal("missing second source object must fail the whole preparation")
			}
			entries, readErr := os.ReadDir(fixture.material.inputs)
			if readErr != nil || len(entries) != 0 {
				t.Fatalf("failed preparation must not publish a partial tree, entries=%v err=%v", entries, readErr)
			}
		})
	}
}

type runViewInputFixture struct {
	db       *gorm.DB
	runtime  *localCraftRuntime
	task     craft.Task
	material CraftRunViewMaterialHandle
	files    craftRuntimeInputFiles
	manifest []craft.Input
}

func newRunViewInputFixture(t *testing.T, manifest []craft.Input, blobs map[string][]byte) *runViewInputFixture {
	t.Helper()
	if manifest == nil {
		manifest = []craft.Input{}
	}
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "runview-inputs.db")+"?mode=memory&cache=shared"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE craft_workspace_inputs (workspace_id TEXT, tenant_id INTEGER, ref TEXT, name TEXT, sha256 TEXT, bytes INTEGER, created_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE craft_workspaces (id TEXT, tenant_id INTEGER, owner_id TEXT, session_id TEXT)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO craft_workspaces (id, tenant_id, owner_id, session_id) VALUES ('ws-inputs', 1, 'owner', 'task-session')`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (tenant_id INTEGER, run_id TEXT, session_id TEXT, owner_id TEXT, snapshot TEXT)`).Error)
	snapshot, err := json.Marshal(map[string]any{
		"version": 1, "query": "RunView input staging", "model_id": "model-1",
		"agent_config": json.RawMessage(`{}`), "runtime": map[string]any{}, "craft_input_manifest": manifest,
	})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, snapshot) VALUES (1, 'run-layout', 'task-session', 'owner', ?)`, string(snapshot)).Error)
	for _, input := range manifest {
		require.NoError(t, db.Exec(`INSERT INTO craft_workspace_inputs (workspace_id, tenant_id, ref, name, sha256, bytes, created_at) VALUES ('ws-inputs', 1, ?, ?, ?, ?, CURRENT_TIMESTAMP)`, input.Ref, input.Name, input.SHA256, input.Bytes).Error)
	}
	task := craft.Task{Scope: craft.Scope{TenantID: 1, UserID: "owner", SessionID: "task-session"},
		Fence: agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 1, RunID: "run-layout"}}, WorkspaceID: "ws-inputs"}
	material := newCraftRuntimeTestMaterial(t, task, materialTestGeneration(t))
	files := craftRuntimeInputFiles{blobs: blobs, reads: map[string]int{}}
	return &runViewInputFixture{db: db, runtime: &localCraftRuntime{db: db, files: files, workDir: t.TempDir()}, task: task, material: material, files: files, manifest: manifest}
}

func newCraftRuntimeTestMaterial(t *testing.T, task craft.Task, generation string) CraftRunViewMaterialHandle {
	t.Helper()
	const sessionID = "ses_0123456789ab0123456789ABCD"
	engine := newFakeCraftRunViewContainerEngine()
	provider, api := newRVTestProviderWithSessionAPI(t, engine)
	container, err := provider.InspectOrCreateContainer(context.Background(), rvTestSpec(generation))
	require.NoError(t, err)
	api.sessions = []opencode.SessionInfo{rvTestOpenCodeSession(sessionID, container.Directory, container.ProjectID)}
	key := craft.RunViewKey{TenantID: task.Fence.TenantID, OwnerID: task.Scope.UserID, SessionID: task.Scope.SessionID, RunID: task.Fence.RunID}
	view := boundMaterialTestView(key, container, generation, sessionID)
	store := materialTestStore{view: view}
	handle := CraftRunViewRuntimeHandle{View: view, Directory: container.Directory}
	material, err := provider.MaterialHandle(context.Background(), store, key, handle)
	require.NoError(t, err)
	return material
}

func materialTestGeneration(t *testing.T) string {
	t.Helper()
	sum := sha256.Sum256([]byte(t.Name()))
	return "generation-input-" + hex.EncodeToString(sum[:6])
}

func TestPointWorkspaceOutputScopesEachDelegation(t *testing.T) {
	work := t.TempDir()
	runtime := &localCraftRuntime{workDir: work, outputDir: "output", sessionsRoot: filepath.Join(work, "ws")}
	first := craft.Task{Prompt: "monthly goal"}
	if err := runtime.pointWorkspaceOutput(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "output", "index.html"), []byte("monthly"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A different delegation prompt must get its own empty directory while
	// the first delegation's bytes stay frozen at their own path.
	second := craft.Task{Prompt: "quarterly goal"}
	if err := runtime.pointWorkspaceOutput(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(work, "output"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("second delegation must start from an empty output dir, found %d entries", len(entries))
	}
	// The same prompt repoints to the same directory (retry stability).
	if err := runtime.pointWorkspaceOutput(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(work, "output", "index.html"))
	if err != nil || string(data) != "monthly" {
		t.Fatalf("first delegation's bytes must survive: %q, %v", data, err)
	}
	// A real directory where the pointer belongs is refused, never deleted.
	blocked := t.TempDir()
	if err := os.MkdirAll(filepath.Join(blocked, "output", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	refused := &localCraftRuntime{workDir: blocked, outputDir: "output", sessionsRoot: filepath.Join(blocked, "ws")}
	if err := refused.pointWorkspaceOutput(context.Background(), first); err == nil {
		t.Fatal("a real output directory must be refused instead of replaced")
	}
	if _, err := os.Stat(filepath.Join(blocked, "output", "x")); err != nil {
		t.Fatalf("the pre-existing directory must be untouched: %v", err)
	}
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func TestLocalCraftArtifactSourceFollowsTheOutputPointer(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "ws", "key_a", "output", "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ws", "key_a", "output", "index.html"), []byte("<html>a</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ws", "key_a", "output", "assets", "app.js"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Another delegation's output stays invisible behind its own pointer.
	if err := os.MkdirAll(filepath.Join(root, "ws", "key_b", "output"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ws", "key_b", "output", "index.html"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The serve-relative output path is the pointer symlink into key_a.
	if err := os.Symlink(filepath.Join(root, "ws", "key_a", "output"), filepath.Join(root, "output")); err != nil {
		t.Fatal(err)
	}
	// A symlink inside the output is reported as non-regular.
	if err := os.Symlink(filepath.Join(root, "ws", "key_b", "output", "index.html"),
		filepath.Join(root, "ws", "key_a", "output", "link.html")); err != nil {
		t.Fatal(err)
	}
	source := &localCraftArtifactSource{workDir: root, outputDir: "output"}

	entries, err := source.ListSessionFiles(context.Background(), "ses_a", "output")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]sandboxEntry{}
	for _, entry := range entries {
		files[entry.Path] = sandboxEntry{typ: string(entry.Type), size: entry.Size}
	}
	if got := files["output/index.html"]; got.typ != "file" || got.size != int64(len("<html>a</html>")) {
		t.Fatalf("output/index.html entry = %+v", got)
	}
	if got := files["output/assets/app.js"]; got.typ != "file" {
		t.Fatalf("nested entry missing: %+v (all: %v)", got, files)
	}
	if got := files["output/link.html"]; got.typ != "other" {
		t.Fatalf("symlink must be reported as other, got %+v", got)
	}
	if _, ok := files["output/key_b"]; ok {
		t.Fatal("another delegation's directory leaked into the listing")
	}

	data, err := source.ReadSessionFile(context.Background(), "ses_a", "output/index.html")
	if err != nil || string(data) != "<html>a</html>" {
		t.Fatalf("read = %q, %v", data, err)
	}
	// Traversal attempts are sanitized to the work dir and miss.
	if _, err := source.ReadSessionFile(context.Background(), "ses_a", "output/../../ws/key_b/output/index.html"); err == nil {
		t.Fatal("traversal must not reach another delegation's file")
	}
	// Without a pointer there is nothing to list.
	bare := t.TempDir()
	bareSource := &localCraftArtifactSource{workDir: bare, outputDir: "output"}
	empty, err := bareSource.ListSessionFiles(context.Background(), "ses_missing", "output")
	if err != nil || len(empty) != 0 {
		t.Fatalf("pointerless listing = %v, %v", empty, err)
	}
}

type sandboxEntry struct {
	typ  string
	size int64
}

func TestCraftRunEventEmitterPayloadShape(t *testing.T) {
	sink := &recordingEventSink{}
	emit := craftRunEventEmitter(sink)
	task := craft.Task{WorkspaceID: "wsp_1", ID: "dlg_1", ToolCallID: "call_1"}
	task.Fence = agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 1, RunID: "r1"}, Owner: "worker", Epoch: 2}
	if err := emit(context.Background(), task, "delegation.started", json.RawMessage(`{"prompt_message_id":"msg_1"}`)); err != nil {
		t.Fatal(err)
	}
	if len(sink.events) != 1 {
		t.Fatalf("one event expected, got %d", len(sink.events))
	}
	event := sink.events[0]
	if event.Type != "craft" {
		t.Fatalf("event type = %q", event.Type)
	}
	var payload struct {
		Kind         string          `json:"kind"`
		WorkspaceID  string          `json:"workspace_id"`
		DelegationID string          `json:"delegation_id"`
		ToolCallID   string          `json:"tool_call_id"`
		Data         json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("payload does not parse: %v (%s)", err, event.Payload)
	}
	if payload.Kind != "delegation.started" || payload.WorkspaceID != "wsp_1" ||
		payload.DelegationID != "dlg_1" || payload.ToolCallID != "call_1" {
		t.Fatalf("payload identity fields wrong: %+v", payload)
	}
	var data map[string]any
	if err := json.Unmarshal(payload.Data, &data); err != nil || data["prompt_message_id"] != "msg_1" {
		t.Fatalf("payload data wrong: %s (%v)", payload.Data, err)
	}
}

type recordingEventSink struct {
	events []agentruntime.RunEvent
}

func (s *recordingEventSink) AppendEvent(_ context.Context, _ agentruntime.Fence, event agentruntime.RunEvent) (agentruntime.RunEvent, error) {
	s.events = append(s.events, event)
	return event, nil
}
