// Package container tests for the W06 env-driven craft runtime assembly
// (coordinator-authorized extension): fail-closed default, prompt framing,
// session-scoped artifact source and the craft event emitter payload.
package container

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
)

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
