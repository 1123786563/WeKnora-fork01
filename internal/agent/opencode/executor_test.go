package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
)

// runtimeFake is an httptest OpenCode runtime speaking the locked 1.18.4
// protocol shapes. It records prompt/subscription order, serves a
// test-driven message snapshot, and streams frames pushed by the test.
type runtimeFake struct {
	mu           sync.Mutex
	sessionID    string
	prompts      int
	promptAt     time.Time
	eventsAt     time.Time
	promptID     string
	hijackPrompt bool
	// CFT-S02-T015 fault injection: hangs GET /message until the request
	// context ends (a read timeout against a live serve).
	hangMessages    bool
	status          string
	aborts          int
	events          chan string
	cut             chan struct{}
	buildMessages   func(promptID string) string
	server          *httptest.Server
	requestIDs      map[string][]string
	directories     []string
	autoCloseEvents bool
}

type fakeRunResolver struct {
	bindings map[string]RunSessionBinding
	seen     []string
}

func (r *fakeRunResolver) ResolveRunSession(_ context.Context, task craft.Task) (RunSessionBinding, error) {
	r.seen = append(r.seen, task.Fence.RunID)
	binding, ok := r.bindings[task.Fence.RunID]
	if !ok {
		return RunSessionBinding{}, craft.ErrNotFound
	}
	return binding, nil
}

func newRuntimeFake(t *testing.T) *runtimeFake {
	return newRuntimeFakeForSession(t, runSessionID("AAAAAAAAAAAAAA"))
}

func newRuntimeFakeForSession(t *testing.T, sessionID string) *runtimeFake {
	t.Helper()
	f := &runtimeFake{sessionID: sessionID, status: "busy", events: make(chan string, 128), cut: make(chan struct{}), requestIDs: make(map[string][]string)}
	f.buildMessages = func(string) string { return "[]" }
	mux := http.NewServeMux()
	mux.HandleFunc("GET /event", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.eventsAt = time.Now()
		f.directories = append(f.directories, r.Header.Get(directoryHeader))
		autoClose := f.autoCloseEvents
		cut := f.cut
		f.mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, ": subscribed\n\n")
		w.(http.Flusher).Flush()
		if autoClose {
			return
		}
		for {
			select {
			case frame, ok := <-f.events:
				if !ok {
					return
				}
				fmt.Fprint(w, frame)
				w.(http.Flusher).Flush()
			case <-cut:
				// Simulated stream break: the server itself keeps running.
				return
			case <-r.Context().Done():
				return
			}
		}
	})
	mux.HandleFunc("POST /session/{sessionID}/prompt_async", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("sessionID") != f.sessionID {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		messageID, _ := body["messageID"].(string)
		f.mu.Lock()
		f.prompts++
		f.requestIDs["prompt"] = append(f.requestIDs["prompt"], r.PathValue("sessionID"))
		f.directories = append(f.directories, r.Header.Get(directoryHeader))
		f.promptAt = time.Now()
		f.promptID = messageID
		hijack := f.hijackPrompt
		f.mu.Unlock()
		if hijack {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /session/{sessionID}/message", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("sessionID") != f.sessionID {
			http.NotFound(w, r)
			return
		}
		f.mu.Lock()
		f.requestIDs["message"] = append(f.requestIDs["message"], r.PathValue("sessionID"))
		f.directories = append(f.directories, r.Header.Get(directoryHeader))
		hang := f.hangMessages
		f.mu.Unlock()
		if hang {
			<-r.Context().Done()
			return
		}
		f.mu.Lock()
		promptID := f.promptID
		body := f.buildMessages(promptID)
		body = strings.ReplaceAll(body, `"ses_oc"`, `"`+f.sessionID+`"`)
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	})
	mux.HandleFunc("GET /session/status", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requestIDs["status"] = append(f.requestIDs["status"], f.sessionID)
		f.directories = append(f.directories, r.Header.Get(directoryHeader))
		status := f.status
		f.mu.Unlock()
		fmt.Fprintf(w, "{%q:{%q:%q}}", f.sessionID, "type", status)
	})
	mux.HandleFunc("POST /session/{sessionID}/abort", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("sessionID") != f.sessionID {
			http.NotFound(w, r)
			return
		}
		f.mu.Lock()
		f.aborts++
		f.requestIDs["abort"] = append(f.requestIDs["abort"], r.PathValue("sessionID"))
		f.directories = append(f.directories, r.Header.Get(directoryHeader))
		f.mu.Unlock()
		fmt.Fprint(w, "true")
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func (f *runtimeFake) push(t *testing.T, typ string, properties any) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"id": "evt_" + typ, "type": typ, "properties": properties})
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.ReplaceAll(string(raw), `"ses_oc"`, `"`+f.sessionID+`"`))
	select {
	case f.events <- "data: " + string(raw) + "\n\n":
	default:
		t.Fatal("event channel is full")
	}
}

func (f *runtimeFake) promptCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.prompts
}

func (f *runtimeFake) requestSessionIDs(kind string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.requestIDs[kind]...)
}

func (f *runtimeFake) requestDirectories() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.directories...)
}

func (f *runtimeFake) setAutoCloseEvents(value bool) {
	f.mu.Lock()
	f.autoCloseEvents = value
	f.mu.Unlock()
}

func (f *runtimeFake) setHangMessages(hang bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hangMessages = hang
}

func (f *runtimeFake) currentPromptID() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.promptID
}

func (f *runtimeFake) promptTime() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.promptAt
}

func (f *runtimeFake) eventsTime() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.eventsAt
}

func (f *runtimeFake) setStatus(status string) {
	f.mu.Lock()
	f.status = status
	f.mu.Unlock()
}

func (f *runtimeFake) cutStream() {
	f.mu.Lock()
	close(f.cut)
	f.cut = make(chan struct{})
	f.mu.Unlock()
}

// memStore mirrors the R02 store semantics the executor relies on: the
// identical delegation replays the stored task (original prompt message id),
// and different arguments conflict.
type memStore struct {
	mu        sync.Mutex
	workspace craft.Workspace
	tasks     map[string]craft.Task
	results   map[string]craft.Result
	prepareAt time.Time
	prepares  int
	saves     []craft.Result
}

func newMemStore() *memStore {
	return &memStore{tasks: map[string]craft.Task{}, results: map[string]craft.Result{}}
}

func (s *memStore) GetWorkspace(ctx context.Context, scope craft.Scope) (craft.Workspace, error) {
	if scope.TenantID != s.workspace.TenantID || scope.SessionID != s.workspace.SessionID {
		return craft.Workspace{}, craft.ErrNotFound
	}
	if scope.UserID != s.workspace.UserID {
		return craft.Workspace{}, craft.ErrForbidden
	}
	return s.workspace, nil
}

func (s *memStore) PutWorkspace(ctx context.Context, in craft.Workspace, expected int64) (craft.Workspace, error) {
	return craft.Workspace{}, craft.ErrUnsupported
}

func (s *memStore) PrepareTask(ctx context.Context, in craft.Task) (craft.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.tasks[in.ToolCallID]; ok {
		if existing.Prompt != in.Prompt || existing.RequestHash != in.RequestHash ||
			existing.WorkspaceID != in.WorkspaceID || !existing.Deadline.Equal(in.Deadline) {
			return craft.Task{}, craft.ErrConflict
		}
		return existing, nil
	}
	if in.ID == "" {
		in.ID = "dlg_1"
	}
	s.tasks[in.ToolCallID] = in
	s.prepares++
	s.prepareAt = time.Now()
	return in, nil
}

// lastPreparedPromptID exposes the persisted prompt message id so re-entry
// tests can reuse the SAME delegation identity (CFT-S02-T015).
func (s *memStore) lastPreparedPromptID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task, ok := s.tasks["call_r04"]; ok {
		return task.PromptMessageID
	}
	return ""
}

func (s *memStore) GetTask(_ context.Context, _ craft.Scope, _ string) (craft.Task, error) {
	return craft.Task{}, craft.ErrNotFound
}

func (s *memStore) SaveResult(ctx context.Context, fence agentruntime.Fence, result craft.Result) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results[result.TaskID] = result
	s.saves = append(s.saves, result)
	return nil
}

func (s *memStore) GetResult(ctx context.Context, scope craft.Scope, taskID string) (craft.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if result, ok := s.results[taskID]; ok {
		return result, nil
	}
	return craft.Result{}, craft.ErrNotFound
}

func (s *memStore) savedStatuses() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	statuses := make([]string, 0, len(s.saves))
	for _, result := range s.saves {
		statuses = append(statuses, result.Status)
	}
	return statuses
}

func (s *memStore) prepareTime() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.prepareAt
}

type emitRecorder struct {
	mu       sync.Mutex
	kinds    []string
	payloads map[string][]string
}

func (r *emitRecorder) emit(ctx context.Context, task craft.Task, kind string, data json.RawMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.payloads == nil {
		r.payloads = map[string][]string{}
	}
	r.kinds = append(r.kinds, kind)
	r.payloads[kind] = append(r.payloads[kind], string(data))
	return nil
}

func (r *emitRecorder) count(kind string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, seen := range r.kinds {
		if seen == kind {
			count++
		}
	}
	return count
}

func (r *emitRecorder) payloadsOf(kind string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.payloads[kind]...)
}

func craftWorkspace() craft.Workspace {
	return craft.Workspace{
		ID: "ws_1", Scope: craft.Scope{TenantID: 7, UserID: "user_1", SessionID: "ses_main"},
		SandboxID: "sbx_1", Generation: "g1", OpenCodeSessionID: "ses_oc", RuntimeDigest: "sha256:locked", Revision: 1,
	}
}

func delegationTask(deadline time.Time) craft.Task {
	return craft.Task{
		ToolCallID: "call_r04", Prompt: "build the slide deck", RequestHash: "hash_1",
		Scope:       craft.Scope{TenantID: 7, UserID: "user_1", SessionID: "ses_main"},
		Fence:       agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 7, RunID: "run_1"}, Owner: "worker_1", Epoch: 2},
		WorkspaceID: "ws_1",
		Deadline:    deadline,
	}
}

func mustClient(t *testing.T, f *runtimeFake) *Client {
	t.Helper()
	client, err := NewClient(f.server.URL, f.server.Client())
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func newExecutorFor(t *testing.T, f *runtimeFake, store *memStore, recorder *emitRecorder) craft.Executor {
	client, err := mustClient(t, f).WithDirectory("/workspace/executor-test")
	if err != nil {
		t.Fatal(err)
	}
	sessionID := f.sessionID
	return NewRunBoundExecutor(store, &fakeRunResolver{bindings: map[string]RunSessionBinding{
		"run_1": {Key: taskRunViewKey(delegationTask(time.Time{})), Client: client, SessionID: sessionID, Directory: "/workspace/executor-test"},
	}}, recorder.emit)
}

func taskRunViewKey(task craft.Task) craft.RunViewKey {
	return craft.RunViewKey{TenantID: task.Scope.TenantID, OwnerID: task.Scope.UserID, SessionID: task.Scope.SessionID, RunID: task.Fence.RunID}
}

func runSessionID(suffix string) string { return "ses_012345abcdef" + suffix }

func resolvedBinding(t *testing.T, f *runtimeFake, task craft.Task, directory string) RunSessionBinding {
	t.Helper()
	client, err := mustClient(t, f).WithDirectory(directory)
	if err != nil {
		t.Fatal(err)
	}
	return RunSessionBinding{Key: taskRunViewKey(task), Client: client, SessionID: f.sessionID, Directory: directory}
}

func TestRunBoundExecutorSelectsSeparateRunForEveryOperation(t *testing.T) {
	fa := newRuntimeFakeForSession(t, runSessionID("AAAAAAAAAAAAAA"))
	fb := newRuntimeFakeForSession(t, runSessionID("BBBBBBBBBBBBBB"))
	for _, f := range []*runtimeFake{fa, fb} {
		f.setStatus("idle")
		f.setAutoCloseEvents(true)
		f.buildMessages = func(promptID string) string {
			if promptID == "" {
				return "[]"
			}
			return messagesJSON(t, userEntry(promptID, "build"), assistantEntry("msg_asst", promptID, 3, "stop", "", textPart("prt_a", "msg_asst", "done")))
		}
	}
	store := newMemStore()
	store.workspace = craftWorkspace() // Deliberately has the obsolete shared ses_oc value.
	taskA := delegationTask(time.Now().Add(time.Second))
	taskA.Fence.RunID, taskA.ToolCallID = "run_A", "call_A"
	taskB := delegationTask(time.Now().Add(time.Second))
	taskB.Fence.RunID, taskB.ToolCallID = "run_B", "call_B"
	resolver := &fakeRunResolver{bindings: map[string]RunSessionBinding{
		"run_A": resolvedBinding(t, fa, taskA, "/run/A"),
		"run_B": resolvedBinding(t, fb, taskB, "/run/B"),
	}}
	executor := NewRunBoundExecutor(store, resolver, (&emitRecorder{}).emit)
	for _, task := range []craft.Task{taskA, taskB} {
		result, err := executor.Execute(context.Background(), task)
		if err != nil || result.Status != "succeeded" {
			t.Fatalf("Execute(%s) = %#v, %v", task.Fence.RunID, result, err)
		}
	}
	observeTask := taskA
	observeTask.PromptMessageID = store.tasks[taskA.ToolCallID].PromptMessageID
	if _, err := executor.Observe(context.Background(), observeTask); err != nil {
		t.Fatalf("Observe(run_A) = %v", err)
	}
	abortTask := taskB
	abortTask.PromptMessageID = store.tasks[taskB.ToolCallID].PromptMessageID
	if err := executor.Abort(context.Background(), abortTask); err != nil {
		t.Fatalf("Abort(run_B) = %v", err)
	}
	if got, want := resolver.seen, []string{"run_A", "run_B", "run_A", "run_B"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("resolved Run sequence = %v, want %v", got, want)
	}
	for _, tc := range []struct {
		fake *runtimeFake
		id   string
		dir  string
	}{{fa, fa.sessionID, "/run/A"}, {fb, fb.sessionID, "/run/B"}} {
		for kind, ids := range map[string][]string{"prompt": tc.fake.requestSessionIDs("prompt"), "message": tc.fake.requestSessionIDs("message"), "abort": tc.fake.requestSessionIDs("abort"), "status": tc.fake.requestSessionIDs("status")} {
			for _, id := range ids {
				if id != tc.id || id == "ses_oc" {
					t.Fatalf("%s used session %q; expected per-Run %q", kind, id, tc.id)
				}
			}
		}
		for _, directory := range tc.fake.requestDirectories() {
			if directory != tc.dir {
				t.Fatalf("request used directory %q; expected %q", directory, tc.dir)
			}
		}
	}
}

func TestRunBoundExecutorFailsClosedWithoutValidBinding(t *testing.T) {
	t.Run("legacy constructor has no resolver", func(t *testing.T) {
		f := newRuntimeFake(t)
		store := newMemStore()
		store.workspace = craftWorkspace()
		executor := NewExecutor(mustClient(t, f), store, nil)
		_, err := executor.Execute(context.Background(), delegationTask(time.Now().Add(time.Second)))
		if !errors.Is(err, craft.ErrUnsupported) {
			t.Fatalf("Execute() error = %v, want ErrUnsupported", err)
		}
		observeTask := delegationTask(time.Time{})
		observeTask.PromptMessageID = "msg_persisted"
		if _, err := executor.Observe(context.Background(), observeTask); !errors.Is(err, craft.ErrUnsupported) {
			t.Fatalf("Observe() error = %v, want ErrUnsupported", err)
		}
		if err := executor.Abort(context.Background(), observeTask); !errors.Is(err, craft.ErrUnsupported) {
			t.Fatalf("Abort() error = %v, want ErrUnsupported", err)
		}
		if f.promptCount() != 0 || len(f.requestSessionIDs("status")) != 0 || len(f.requestSessionIDs("message")) != 0 {
			t.Fatal("missing Run resolver reached the runtime")
		}
		if f.aborts != 0 || !f.eventsAt.IsZero() {
			t.Fatal("missing Run resolver reached abort or event stream")
		}
	})
	for _, name := range []string{"mismatched Run identity", "unbound client"} {
		t.Run(name, func(t *testing.T) {
			f := newRuntimeFake(t)
			store := newMemStore()
			store.workspace = craftWorkspace()
			task := delegationTask(time.Now().Add(time.Second))
			binding := resolvedBinding(t, f, task, "/run/only")
			if name == "mismatched Run identity" {
				binding.Key.RunID = "sibling_run"
			} else {
				binding.Client = mustClient(t, f)
			}
			executor := NewRunBoundExecutor(store, &fakeRunResolver{bindings: map[string]RunSessionBinding{"run_1": binding}}, nil)
			_, err := executor.Execute(context.Background(), task)
			if err == nil {
				t.Fatal("invalid Run binding was accepted")
			}
			if f.promptCount() != 0 || len(f.requestSessionIDs("status")) != 0 || len(f.requestSessionIDs("message")) != 0 || f.aborts != 0 || !f.eventsAt.IsZero() {
				t.Fatal("invalid Run binding reached the runtime")
			}
		})
	}
}

func waitFor(t *testing.T, condition func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(2 * time.Millisecond)
	}
	return false
}

type execOutcome struct {
	result craft.Result
	err    error
}

func runExecute(executor craft.Executor, task craft.Task) chan execOutcome {
	done := make(chan execOutcome, 1)
	go func() {
		result, err := executor.Execute(context.Background(), task)
		done <- execOutcome{result: result, err: err}
	}()
	return done
}

func awaitOutcome(t *testing.T, done chan execOutcome) execOutcome {
	t.Helper()
	select {
	case outcome := <-done:
		return outcome
	case <-time.After(8 * time.Second):
		t.Fatal("Execute did not return")
		return execOutcome{}
	}
}

// Message snapshot builders using the real 1.18.4 projection shapes.
func userEntry(id, text string) map[string]any {
	return map[string]any{
		"info": map[string]any{
			"id": id, "sessionID": "ses_oc", "role": "user",
			"time": map[string]any{"created": 1},
		},
		"parts": []any{
			map[string]any{"type": "text", "id": "prt_u_" + id, "sessionID": "ses_oc", "messageID": id, "text": text},
		},
	}
}

func textPart(id, messageID, text string) map[string]any {
	return map[string]any{"type": "text", "id": id, "sessionID": "ses_oc", "messageID": messageID, "text": text}
}

func toolPart(id, messageID, tool, status string) map[string]any {
	return map[string]any{
		"type": "tool", "id": id, "sessionID": "ses_oc", "messageID": messageID,
		"tool": tool, "callID": "call_" + id, "state": map[string]any{"status": status},
	}
}

func assistantEntry(id, parentID string, completed int64, finish, errorName string, parts ...map[string]any) map[string]any {
	info := map[string]any{
		"id": id, "parentID": parentID, "sessionID": "ses_oc", "role": "assistant",
		"finish": finish, "time": map[string]any{"created": 2},
	}
	if completed > 0 {
		info["time"].(map[string]any)["completed"] = completed
	}
	if errorName != "" {
		info["error"] = map[string]any{"name": errorName, "data": map[string]any{"message": "Aborted"}}
	}
	// An aborted message carries an empty parts array, never JSON null.
	partList := make([]any, 0, len(parts))
	for _, part := range parts {
		partList = append(partList, part)
	}
	return map[string]any{"info": info, "parts": partList}
}

func messagesJSON(t *testing.T, entries ...map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestExecuteCommitsBeforeDispatchAndSubscribesBeforePrompt(t *testing.T) {
	f := newRuntimeFake(t)
	f.buildMessages = func(promptID string) string {
		if promptID == "" {
			return "[]"
		}
		// An old completed round shares the session: it must not be adopted.
		return messagesJSON(t,
			userEntry("msg_old_prompt", "old request"),
			assistantEntry("msg_old_asst", "msg_old_prompt", 999, "stop", "", textPart("prt_old", "msg_old_asst", "OLD-REPLY")),
			userEntry(promptID, "build the slide deck"),
			assistantEntry("msg_asst", promptID, 3, "stop", "", textPart("prt_a", "msg_asst", "MOCK-REPLY")))
	}
	store := newMemStore()
	store.workspace = craftWorkspace()
	recorder := &emitRecorder{}
	executor := newExecutorFor(t, f, store, recorder)
	done := runExecute(executor, delegationTask(time.Now().Add(5*time.Second)))
	if !waitFor(t, func() bool { return f.promptCount() == 1 }) {
		t.Fatal("the prompt was never submitted")
	}
	promptID := f.currentPromptID()
	f.push(t, "session.status", map[string]any{"sessionID": "ses_oc", "status": map[string]any{"type": "busy"}})
	f.push(t, "message.updated", map[string]any{"sessionID": "ses_oc", "info": map[string]any{
		"id": "msg_asst", "parentID": promptID, "role": "assistant", "time": map[string]any{"created": 2},
	}})
	f.push(t, "message.part.updated", map[string]any{
		"sessionID": "ses_oc",
		"part":      map[string]any{"type": "text", "id": "prt_a", "messageID": "msg_asst", "text": "MOCK-REPLY"},
	})
	f.setStatus("idle")
	f.push(t, "message.updated", map[string]any{"sessionID": "ses_oc", "info": map[string]any{
		"id": "msg_asst", "parentID": promptID, "role": "assistant", "finish": "stop",
		"time": map[string]any{"created": 2, "completed": 3},
	}})
	f.push(t, "session.status", map[string]any{"sessionID": "ses_oc", "status": map[string]any{"type": "idle"}})
	f.push(t, "session.idle", map[string]any{"sessionID": "ses_oc"})
	outcome := awaitOutcome(t, done)
	if outcome.err != nil {
		t.Fatalf("Execute error: %v", outcome.err)
	}
	if outcome.result.Status != "succeeded" || outcome.result.Summary != "MOCK-REPLY" {
		t.Fatalf("result = %#v", outcome.result)
	}
	if outcome.result.TaskID != "dlg_1" {
		t.Fatalf("task id = %q", outcome.result.TaskID)
	}
	if got := store.savedStatuses(); len(got) != 1 || got[0] != "succeeded" {
		t.Fatalf("saved results = %v", got)
	}
	if f.promptCount() != 1 {
		t.Fatalf("prompt POSTs = %d", f.promptCount())
	}
	// The mandated commit ordering: persist, subscribe, then prompt.
	if !store.prepareTime().Before(f.eventsTime()) {
		t.Fatal("PrepareTask did not run before the event subscription")
	}
	if !f.eventsTime().Before(f.promptTime()) {
		t.Fatal("the event subscription did not precede the prompt")
	}
	if !store.prepareTime().Before(f.promptTime()) {
		t.Fatal("the delegation was not persisted before the prompt")
	}
	for _, kind := range []string{"delegation.started", "delegation.text", "delegation.finished"} {
		if recorder.count(kind) == 0 {
			t.Fatalf("missing %s event; kinds recorded: %v", kind, recorder.payloadsOf(kind))
		}
	}
	finished := recorder.payloadsOf("delegation.finished")
	if len(finished) != 1 || !containsAll(finished[0], "succeeded") {
		t.Fatalf("delegation.finished payloads = %v", finished)
	}
}

func containsAll(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func TestExecuteStreamBreakWithRemoteStillRunningIsUnknown(t *testing.T) {
	f := newRuntimeFake(t)
	f.buildMessages = func(promptID string) string {
		if promptID == "" {
			return "[]"
		}
		return messagesJSON(t, userEntry(promptID, "build the slide deck"),
			assistantEntry("msg_asst", promptID, 0, "", ""))
	}
	store := newMemStore()
	store.workspace = craftWorkspace()
	executor := newExecutorFor(t, f, store, &emitRecorder{})
	done := runExecute(executor, delegationTask(time.Now().Add(3*time.Second)))
	if !waitFor(t, func() bool { return f.promptCount() == 1 }) {
		t.Fatal("the prompt was never submitted")
	}
	f.push(t, "session.status", map[string]any{"sessionID": "ses_oc", "status": map[string]any{"type": "busy"}})
	f.push(t, "message.updated", map[string]any{"sessionID": "ses_oc", "info": map[string]any{
		"id": "msg_asst", "parentID": f.currentPromptID(), "role": "assistant", "time": map[string]any{"created": 2},
	}})
	f.cutStream()
	outcome := awaitOutcome(t, done)
	if outcome.result.Status != "unknown" || !errors.Is(outcome.err, craft.ErrUnknown) {
		t.Fatalf("outcome = %#v, err = %v", outcome.result, outcome.err)
	}
	if f.promptCount() != 1 {
		t.Fatalf("prompt POSTs = %d", f.promptCount())
	}
	if saves := store.savedStatuses(); len(saves) != 0 {
		t.Fatalf("an unresolved execution stored results: %v", saves)
	}
}

func TestExecutePromptAcceptedButResponseLostStillVerifies(t *testing.T) {
	f := newRuntimeFake(t)
	f.hijackPrompt = true
	f.buildMessages = func(promptID string) string {
		if promptID == "" {
			return "[]"
		}
		return messagesJSON(t, userEntry(promptID, "build the slide deck"),
			assistantEntry("msg_asst", promptID, 3, "stop", "", textPart("prt_a", "msg_asst", "MOCK-REPLY")))
	}
	store := newMemStore()
	store.workspace = craftWorkspace()
	recorder := &emitRecorder{}
	executor := newExecutorFor(t, f, store, recorder)
	done := runExecute(executor, delegationTask(time.Now().Add(4*time.Second)))
	if !waitFor(t, func() bool { return f.promptCount() == 1 }) {
		t.Fatal("the prompt was never submitted")
	}
	promptID := f.currentPromptID()
	f.setStatus("idle")
	f.push(t, "message.updated", map[string]any{"sessionID": "ses_oc", "info": map[string]any{
		"id": "msg_asst", "parentID": promptID, "role": "assistant", "finish": "stop",
		"time": map[string]any{"created": 2, "completed": 3},
	}})
	f.push(t, "session.idle", map[string]any{"sessionID": "ses_oc"})
	outcome := awaitOutcome(t, done)
	if outcome.err != nil || outcome.result.Status != "succeeded" {
		t.Fatalf("outcome = %#v, err = %v", outcome.result, outcome.err)
	}
	if outcome.result.Summary != "MOCK-REPLY" {
		t.Fatalf("summary = %q", outcome.result.Summary)
	}
	if f.promptCount() != 1 {
		t.Fatalf("the lost-response path re-POSTed: %d", f.promptCount())
	}
	if got := store.savedStatuses(); len(got) != 1 || got[0] != "succeeded" {
		t.Fatalf("saved results = %v", got)
	}
	if recorder.count("delegation.started") != 1 {
		t.Fatalf("delegation.started count = %d", recorder.count("delegation.started"))
	}
}

func TestExecuteIgnoresOldCompletedRoundWithoutOwnAssistant(t *testing.T) {
	f := newRuntimeFake(t)
	f.buildMessages = func(promptID string) string {
		if promptID == "" {
			return "[]"
		}
		return messagesJSON(t,
			userEntry("msg_old_prompt", "old request"),
			assistantEntry("msg_old_asst", "msg_old_prompt", 999, "stop", "", textPart("prt_old", "msg_old_asst", "OLD-REPLY")),
			userEntry(promptID, "build the slide deck"))
	}
	store := newMemStore()
	store.workspace = craftWorkspace()
	executor := newExecutorFor(t, f, store, &emitRecorder{})
	done := runExecute(executor, delegationTask(time.Now().Add(500*time.Millisecond)))
	if !waitFor(t, func() bool { return f.promptCount() == 1 }) {
		t.Fatal("the prompt was never submitted")
	}
	f.setStatus("idle")
	f.push(t, "session.idle", map[string]any{"sessionID": "ses_oc"})
	outcome := awaitOutcome(t, done)
	if outcome.result.Status != "unknown" || !errors.Is(outcome.err, craft.ErrUnknown) {
		t.Fatalf("an old completed round was misreported: %#v, %v", outcome.result, outcome.err)
	}
	if f.promptCount() != 1 {
		t.Fatalf("prompt POSTs = %d", f.promptCount())
	}
	if saves := store.savedStatuses(); len(saves) != 0 {
		t.Fatalf("saved results = %v", saves)
	}
}

func TestExecuteCorruptPartsSnapshotIsUnknown(t *testing.T) {
	f := newRuntimeFake(t)
	f.buildMessages = func(promptID string) string {
		if promptID == "" {
			return "[]"
		}
		// The parts array carries a non-object entry: the pinned client must
		// reject the projection instead of half-parsing it.
		return "[{\"info\":{\"id\":\"msg_asst\",\"parentID\":\"" + promptID +
			"\",\"role\":\"assistant\",\"finish\":\"stop\",\"time\":{\"completed\":3}},\"parts\":[{\"type\":\"text\",\"text\":\"ok\"},23]}]"
	}
	store := newMemStore()
	store.workspace = craftWorkspace()
	executor := newExecutorFor(t, f, store, &emitRecorder{})
	done := runExecute(executor, delegationTask(time.Now().Add(2*time.Second)))
	if !waitFor(t, func() bool { return f.promptCount() == 1 }) {
		t.Fatal("the prompt was never submitted")
	}
	f.setStatus("idle")
	f.push(t, "message.updated", map[string]any{"sessionID": "ses_oc", "info": map[string]any{
		"id": "msg_asst", "parentID": f.currentPromptID(), "role": "assistant", "finish": "stop",
		"time": map[string]any{"completed": 3},
	}})
	f.push(t, "session.idle", map[string]any{"sessionID": "ses_oc"})
	outcome := awaitOutcome(t, done)
	if outcome.result.Status != "unknown" || !errors.Is(outcome.err, craft.ErrUnknown) {
		t.Fatalf("corrupt parts were not treated as unverifiable: %#v, %v", outcome.result, outcome.err)
	}
	if f.promptCount() != 1 {
		t.Fatalf("prompt POSTs = %d", f.promptCount())
	}
	if saves := store.savedStatuses(); len(saves) != 0 {
		t.Fatalf("saved results = %v", saves)
	}
}

func TestExecuteToolStillRunningStaysUnknown(t *testing.T) {
	f := newRuntimeFake(t)
	f.buildMessages = func(promptID string) string {
		if promptID == "" {
			return "[]"
		}
		return messagesJSON(t, userEntry(promptID, "build the slide deck"),
			assistantEntry("msg_asst", promptID, 0, "", "", toolPart("prt_tool", "msg_asst", "bash", "running")))
	}
	store := newMemStore()
	store.workspace = craftWorkspace()
	executor := newExecutorFor(t, f, store, &emitRecorder{})
	done := runExecute(executor, delegationTask(time.Now().Add(400*time.Millisecond)))
	if !waitFor(t, func() bool { return f.promptCount() == 1 }) {
		t.Fatal("the prompt was never submitted")
	}
	f.push(t, "message.part.updated", map[string]any{
		"sessionID": "ses_oc",
		"part": map[string]any{
			"type": "tool", "id": "prt_tool", "messageID": "msg_asst", "tool": "bash",
			"state": map[string]any{"status": "running"},
		},
	})
	outcome := awaitOutcome(t, done)
	if outcome.result.Status != "unknown" || !errors.Is(outcome.err, craft.ErrUnknown) {
		t.Fatalf("a running tool was misreported: %#v, %v", outcome.result, outcome.err)
	}
	if f.promptCount() != 1 {
		t.Fatalf("prompt POSTs = %d", f.promptCount())
	}
	if saves := store.savedStatuses(); len(saves) != 0 {
		t.Fatalf("saved results = %v", saves)
	}
	observed, err := executor.Observe(context.Background(), craft.Task{
		Scope:       craft.Scope{TenantID: 7, UserID: "user_1", SessionID: "ses_main"},
		WorkspaceID: "ws_1", PromptMessageID: f.currentPromptID(),
		Fence: agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 7, RunID: "run_1"}, Owner: "worker_1", Epoch: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !observed.PendingTool || observed.Completed || Completed(observed) {
		t.Fatalf("observation = %#v", observed)
	}
}

func TestExecuteDuplicateEventsEmitOnce(t *testing.T) {
	f := newRuntimeFake(t)
	f.buildMessages = func(promptID string) string {
		if promptID == "" {
			return "[]"
		}
		return messagesJSON(t, userEntry(promptID, "build the slide deck"),
			assistantEntry("msg_asst", promptID, 3, "stop", "",
				textPart("prt_a", "msg_asst", "MOCK-REPLY"),
				toolPart("prt_tool", "msg_asst", "bash", "completed")))
	}
	store := newMemStore()
	store.workspace = craftWorkspace()
	recorder := &emitRecorder{}
	executor := newExecutorFor(t, f, store, recorder)
	done := runExecute(executor, delegationTask(time.Now().Add(4*time.Second)))
	if !waitFor(t, func() bool { return f.promptCount() == 1 }) {
		t.Fatal("the prompt was never submitted")
	}
	promptID := f.currentPromptID()
	for i := 0; i < 2; i++ {
		f.push(t, "message.updated", map[string]any{"sessionID": "ses_oc", "info": map[string]any{
			"id": "msg_asst", "parentID": promptID, "role": "assistant",
			"time": map[string]any{"created": 2, "completed": 3},
		}})
		f.push(t, "message.part.updated", map[string]any{
			"sessionID": "ses_oc",
			"part":      map[string]any{"type": "text", "id": "prt_a", "messageID": "msg_asst", "text": "MOCK-REPLY"},
		})
		f.push(t, "message.part.updated", map[string]any{
			"sessionID": "ses_oc",
			"part": map[string]any{
				"type": "tool", "id": "prt_tool", "messageID": "msg_asst", "tool": "bash",
				"state": map[string]any{"status": "completed"},
			},
		})
	}
	f.setStatus("idle")
	f.push(t, "message.updated", map[string]any{"sessionID": "ses_oc", "info": map[string]any{
		"id": "msg_asst", "parentID": promptID, "role": "assistant", "finish": "stop",
		"time": map[string]any{"created": 2, "completed": 3},
	}})
	f.push(t, "session.idle", map[string]any{"sessionID": "ses_oc"})
	outcome := awaitOutcome(t, done)
	if outcome.err != nil || outcome.result.Status != "succeeded" {
		t.Fatalf("outcome = %#v, err = %v", outcome.result, outcome.err)
	}
	texts := recorder.payloadsOf("delegation.text")
	if len(texts) != 1 || containsAll(texts[0], "MOCK-REPLYMOCK-REPLY") || !containsAll(texts[0], "MOCK-REPLY") {
		t.Fatalf("text events = %v", texts)
	}
	tools := recorder.payloadsOf("delegation.tool")
	if len(tools) != 1 {
		t.Fatalf("tool events = %v", tools)
	}
	if containsAll(tools[0], "input") || containsAll(tools[0], "output") {
		t.Fatalf("tool event leaked payloads: %s", tools[0])
	}
}

func TestExecuteShuffledSnapshotStillSelectsOwnTurn(t *testing.T) {
	f := newRuntimeFake(t)
	f.buildMessages = func(promptID string) string {
		if promptID == "" {
			return "[]"
		}
		// Deliberately shuffled: our assistant first, the old round last.
		return messagesJSON(t,
			assistantEntry("msg_asst", promptID, 3, "stop", "", textPart("prt_a", "msg_asst", "MOCK-REPLY")),
			userEntry("msg_old_prompt", "old request"),
			assistantEntry("msg_old_asst", "msg_old_prompt", 999, "stop", "", textPart("prt_old", "msg_old_asst", "OLD-REPLY")),
			userEntry(promptID, "build the slide deck"))
	}
	store := newMemStore()
	store.workspace = craftWorkspace()
	executor := newExecutorFor(t, f, store, &emitRecorder{})
	done := runExecute(executor, delegationTask(time.Now().Add(4*time.Second)))
	if !waitFor(t, func() bool { return f.promptCount() == 1 }) {
		t.Fatal("the prompt was never submitted")
	}
	promptID := f.currentPromptID()
	f.setStatus("idle")
	f.push(t, "message.updated", map[string]any{"sessionID": "ses_oc", "info": map[string]any{
		"id": "msg_asst", "parentID": promptID, "role": "assistant", "finish": "stop",
		"time": map[string]any{"created": 2, "completed": 3},
	}})
	f.push(t, "session.idle", map[string]any{"sessionID": "ses_oc"})
	outcome := awaitOutcome(t, done)
	if outcome.err != nil || outcome.result.Status != "succeeded" || outcome.result.Summary != "MOCK-REPLY" {
		t.Fatalf("shuffled snapshot outcome = %#v, err = %v", outcome.result, outcome.err)
	}
}

func TestExecuteAbortClassificationUsesLockedErrorName(t *testing.T) {
	for _, tc := range []struct {
		name       string
		errorName  string
		wantStatus string
	}{
		{name: "locked abort", errorName: "MessageAbortedError", wantStatus: "canceled"},
		{name: "cancel-named error is not an abort", errorName: "MessageCancelledError", wantStatus: "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRuntimeFake(t)
			f.buildMessages = func(promptID string) string {
				if promptID == "" {
					return "[]"
				}
				// The real aborted shape: completed timestamp set, no finish,
				// info.error with the locked name, empty parts.
				return messagesJSON(t, userEntry(promptID, "build the slide deck"),
					assistantEntry("msg_asst", promptID, 5, "", tc.errorName))
			}
			store := newMemStore()
			store.workspace = craftWorkspace()
			executor := newExecutorFor(t, f, store, &emitRecorder{})
			done := runExecute(executor, delegationTask(time.Now().Add(3*time.Second)))
			if !waitFor(t, func() bool { return f.promptCount() == 1 }) {
				t.Fatal("the prompt was never submitted")
			}
			f.setStatus("idle")
			f.push(t, "session.error", map[string]any{
				"sessionID": "ses_oc",
				"error":     map[string]any{"name": tc.errorName, "data": map[string]any{"message": "Aborted"}},
			})
			f.push(t, "session.idle", map[string]any{"sessionID": "ses_oc"})
			outcome := awaitOutcome(t, done)
			if outcome.err != nil || outcome.result.Status != tc.wantStatus {
				t.Fatalf("outcome = %#v, err = %v; want %s", outcome.result, outcome.err, tc.wantStatus)
			}
			if got := store.savedStatuses(); len(got) != 1 || got[0] != tc.wantStatus {
				t.Fatalf("saved results = %v; want [%s]", got, tc.wantStatus)
			}
			if f.promptCount() != 1 {
				t.Fatalf("prompt POSTs = %d", f.promptCount())
			}
		})
	}
}

func TestExecuteNeverRepostsAcrossRetries(t *testing.T) {
	f := newRuntimeFake(t)
	f.buildMessages = func(promptID string) string {
		if promptID == "" {
			return "[]"
		}
		return messagesJSON(t, userEntry(promptID, "build the slide deck"),
			assistantEntry("msg_asst", promptID, 0, "", "", textPart("prt_a", "msg_asst", "partial")))
	}
	store := newMemStore()
	store.workspace = craftWorkspace()
	executor := newExecutorFor(t, f, store, &emitRecorder{})
	task := delegationTask(time.Now().Add(8 * time.Second))
	done := runExecute(executor, task)
	if !waitFor(t, func() bool { return f.promptCount() == 1 }) {
		t.Fatal("the prompt was never submitted")
	}
	promptID := f.currentPromptID()
	f.push(t, "message.updated", map[string]any{"sessionID": "ses_oc", "info": map[string]any{
		"id": "msg_asst", "parentID": promptID, "role": "assistant", "time": map[string]any{"created": 2},
	}})
	f.cutStream()
	first := awaitOutcome(t, done)
	if first.result.Status != "unknown" || !errors.Is(first.err, craft.ErrUnknown) {
		t.Fatalf("first leg = %#v, %v", first.result, first.err)
	}
	// The round finishes after the interruption.
	f.buildMessages = func(current string) string {
		return messagesJSON(t, userEntry(promptID, "build the slide deck"),
			assistantEntry("msg_asst", promptID, 3, "stop", "", textPart("prt_a", "msg_asst", "MOCK-REPLY")))
	}
	f.setStatus("idle")
	done = runExecute(executor, task)
	// The buffered frames are delivered to the retry's fresh subscription.
	f.push(t, "message.updated", map[string]any{"sessionID": "ses_oc", "info": map[string]any{
		"id": "msg_asst", "parentID": promptID, "role": "assistant", "finish": "stop",
		"time": map[string]any{"created": 2, "completed": 3},
	}})
	f.push(t, "session.idle", map[string]any{"sessionID": "ses_oc"})
	second := awaitOutcome(t, done)
	if second.err != nil || second.result.Status != "succeeded" {
		t.Fatalf("second leg = %#v, err = %v", second.result, second.err)
	}
	if second.result.TaskID != "dlg_1" {
		t.Fatalf("second leg task id = %q", second.result.TaskID)
	}
	if f.promptCount() != 1 {
		t.Fatalf("the retry re-POSTed the prompt: %d POSTs", f.promptCount())
	}
	if got := store.savedStatuses(); len(got) != 1 || got[0] != "succeeded" {
		t.Fatalf("saved results = %v", got)
	}
}

func TestExecuteExpiredDeadlineNeverPrompts(t *testing.T) {
	f := newRuntimeFake(t)
	store := newMemStore()
	store.workspace = craftWorkspace()
	executor := newExecutorFor(t, f, store, &emitRecorder{})
	outcome := awaitOutcome(t, runExecute(executor, delegationTask(time.Now().Add(-time.Second))))
	if outcome.result.Status != "unknown" || !errors.Is(outcome.err, craft.ErrUnknown) {
		t.Fatalf("outcome = %#v, err = %v", outcome.result, outcome.err)
	}
	if f.promptCount() != 0 {
		t.Fatalf("an expired delegation still prompted: %d", f.promptCount())
	}
	if saves := store.savedStatuses(); len(saves) != 0 {
		t.Fatalf("saved results = %v", saves)
	}
}

func TestObserveReadsSnapshotOnly(t *testing.T) {
	f := newRuntimeFake(t)
	f.buildMessages = func(string) string {
		return messagesJSON(t, userEntry("msg_prompt_fix", "build the slide deck"),
			assistantEntry("msg_asst", "msg_prompt_fix", 3, "stop", "", textPart("prt_a", "msg_asst", "MOCK-REPLY")))
	}
	f.setStatus("idle")
	store := newMemStore()
	store.workspace = craftWorkspace()
	recorder := &emitRecorder{}
	executor := newExecutorFor(t, f, store, recorder)
	observation, err := executor.Observe(context.Background(), craft.Task{
		Scope:           craft.Scope{TenantID: 7, UserID: "user_1", SessionID: "ses_main"},
		WorkspaceID:     "ws_1",
		PromptMessageID: "msg_prompt_fix",
		Fence:           agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 7, RunID: "run_1"}, Owner: "worker_1", Epoch: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !Completed(observation) {
		t.Fatalf("observation = %#v", observation)
	}
	if observation.AssistantParentID != "msg_prompt_fix" || observation.Finish != "stop" {
		t.Fatalf("observation = %#v", observation)
	}
	if f.promptCount() != 0 || len(store.savedStatuses()) != 0 || recorder.count("delegation.started") != 0 {
		t.Fatal("Observe must not prompt, persist or emit")
	}
}

func TestAbortRecordsCanceledOnlyWhenSnapshotConfirms(t *testing.T) {
	t.Run("confirmed abort", func(t *testing.T) {
		f := newRuntimeFake(t)
		f.buildMessages = func(string) string {
			return messagesJSON(t, userEntry("msg_prompt_fix", "build the slide deck"),
				assistantEntry("msg_asst", "msg_prompt_fix", 5, "", "MessageAbortedError"))
		}
		f.setStatus("idle")
		store := newMemStore()
		store.workspace = craftWorkspace()
		recorder := &emitRecorder{}
		executor := newExecutorFor(t, f, store, recorder)
		err := executor.Abort(context.Background(), craft.Task{
			Scope:           craft.Scope{TenantID: 7, UserID: "user_1", SessionID: "ses_main"},
			WorkspaceID:     "ws_1",
			PromptMessageID: "msg_prompt_fix",
			Fence:           agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 7, RunID: "run_1"}, Owner: "w", Epoch: 1},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := store.savedStatuses(); len(got) != 1 || got[0] != "canceled" {
			t.Fatalf("saved results = %v", got)
		}
		if f.aborts == 0 || recorder.count("delegation.finished") != 1 {
			t.Fatal("the abort request or its event was lost")
		}
	})
	t.Run("completed before abort", func(t *testing.T) {
		f := newRuntimeFake(t)
		f.buildMessages = func(string) string {
			return messagesJSON(t, userEntry("msg_prompt_fix", "build the slide deck"),
				assistantEntry("msg_asst", "msg_prompt_fix", 5, "stop", "", textPart("prt_a", "msg_asst", "done")))
		}
		f.setStatus("idle")
		store := newMemStore()
		store.workspace = craftWorkspace()
		executor := newExecutorFor(t, f, store, &emitRecorder{})
		err := executor.Abort(context.Background(), craft.Task{
			Scope:           craft.Scope{TenantID: 7, UserID: "user_1", SessionID: "ses_main"},
			WorkspaceID:     "ws_1",
			PromptMessageID: "msg_prompt_fix",
			Fence:           agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 7, RunID: "run_1"}, Owner: "w", Epoch: 1},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := store.savedStatuses(); len(got) != 0 {
			t.Fatalf("an already-completed round was canceled: %v", got)
		}
	})
}

func TestExecuteUsesRunBindingAndRejectsForbiddenWorkspaceScope(t *testing.T) {
	t.Run("workspace legacy session is not required", func(t *testing.T) {
		f := newRuntimeFake(t)
		f.setStatus("idle")
		f.setAutoCloseEvents(true)
		f.buildMessages = func(promptID string) string {
			if promptID == "" {
				return "[]"
			}
			return messagesJSON(t, userEntry(promptID, "build"), assistantEntry("msg_asst", promptID, 3, "stop", "", textPart("prt_a", "msg_asst", "done")))
		}
		store := newMemStore()
		store.workspace = craftWorkspace()
		store.workspace.OpenCodeSessionID = ""
		executor := newExecutorFor(t, f, store, &emitRecorder{})
		outcome := awaitOutcome(t, runExecute(executor, delegationTask(time.Now().Add(time.Second))))
		if outcome.result.Status != "succeeded" || outcome.err != nil {
			t.Fatalf("outcome = %#v, err = %v", outcome.result, outcome.err)
		}
		if got := store.savedStatuses(); len(got) != 1 || got[0] != "succeeded" {
			t.Fatalf("saved results = %v", got)
		}
		if f.promptCount() != 1 {
			t.Fatal("a Run-bound workspace did not prompt exactly once")
		}
		if got := f.requestSessionIDs("prompt"); len(got) != 1 || got[0] != f.sessionID {
			t.Fatalf("prompt sessions = %v; old Task workspace ID was used", got)
		}
	})
	t.Run("foreign scope", func(t *testing.T) {
		f := newRuntimeFake(t)
		store := newMemStore()
		store.workspace = craftWorkspace()
		executor := newExecutorFor(t, f, store, &emitRecorder{})
		task := delegationTask(time.Now().Add(time.Second))
		task.Scope.UserID = "user_2"
		_, err := executor.Execute(context.Background(), task)
		if !errors.Is(err, craft.ErrForbidden) {
			t.Fatalf("err = %v", err)
		}
		if f.promptCount() != 0 {
			t.Fatal("a forbidden scope still prompted")
		}
	})
}

func TestExecuteInteractionPendingIsEmittedLive(t *testing.T) {
	f := newRuntimeFake(t)
	f.buildMessages = func(promptID string) string {
		if promptID == "" {
			return "[]"
		}
		return messagesJSON(t, userEntry(promptID, "build the slide deck"),
			assistantEntry("msg_asst", promptID, 0, "", ""))
	}
	store := newMemStore()
	store.workspace = craftWorkspace()
	recorder := &emitRecorder{}
	executor := newExecutorFor(t, f, store, recorder)
	done := runExecute(executor, delegationTask(time.Now().Add(400*time.Millisecond)))
	if !waitFor(t, func() bool { return f.promptCount() == 1 }) {
		t.Fatal("the prompt was never submitted")
	}
	promptID := f.currentPromptID()
	f.push(t, "message.updated", map[string]any{"sessionID": "ses_oc", "info": map[string]any{
		"id": "msg_asst", "parentID": promptID, "role": "assistant", "time": map[string]any{"created": 2},
	}})
	f.push(t, "message.part.updated", map[string]any{
		"sessionID": "ses_oc",
		"part": map[string]any{
			"type": "tool", "id": "prt_q", "messageID": "msg_asst", "tool": "question",
			"state": map[string]any{"status": "pending"},
		},
	})
	outcome := awaitOutcome(t, done)
	if outcome.result.Status != "unknown" {
		t.Fatalf("a pending question must keep the delegation unresolved: %#v", outcome.result)
	}
	pending := recorder.payloadsOf("interaction.pending")
	if len(pending) != 1 || !containsAll(pending[0], "question") {
		t.Fatalf("interaction.pending events = %v", pending)
	}
}
