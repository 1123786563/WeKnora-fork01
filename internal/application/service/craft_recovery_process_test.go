package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/opencode"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// This file borrows the tRPC recoverytest SIGKILL pattern (barrier file +
// real subprocess + durable database + resume) for the C04 process-failure
// matrix. The provider is a re-exec of this test binary
// (CRAFT_RECOVERY_PROVIDER_CASE), so the child runs the same compiled
// production packages: the migrated SQLite database, the fenced
// AgentRunStore, the R02 CraftStore, the R04 OpenCode executor, the R05
// delegate service and the C04 CraftRecovery program. Only the OpenCode
// serve is a deterministic in-parent HTTP double — it survives every child
// kill, which is exactly the "external OC stays alive" premise.

const (
	craftProcCaseRecordBeforeSubmit     = "after_record_before_submit"
	craftProcCaseAcceptedNoResponse     = "oc_accepted_before_response"
	craftProcCaseResultBeforeCheckpoint = "after_result_before_checkpoint"
	craftProcCaseLeaseLost              = "lease_lost_after_accept"

	craftProcOCSession  = "ses_craftrecovery000"
	craftProcRunID      = "craft-rec-run"
	craftProcSessionID  = "craft-rec-session"
	craftProcDigest     = "test-runtime-digest"
	craftProcCallID     = "call-craft-proc-1"
	craftProcDelegation = "dlg_craft_proc_1"
)

// TestMain routes provider re-executions away from the test runner.
func TestMain(m *testing.M) {
	if os.Getenv("CRAFT_RECOVERY_PROVIDER_CASE") != "" {
		craftRecoveryProviderMain()
		return
	}
	if os.Getenv("CRAFT_DECISION_PROVIDER_CASE") != "" {
		craftDecisionProviderMain()
		return
	}
	os.Exit(m.Run())
}

// ---- deterministic OpenCode serve double (lives in the parent) ----------

type craftFakeOpenCode struct {
	mu       sync.Mutex
	posts    int
	messages []map[string]any
	// withholdPromptResponse keeps the POST accepted (counted, round
	// recorded) but the HTTP response never delivered.
	withholdPromptResponse bool
	// streamReplay closes the event stream after the accepted round so the
	// R04 executor's live consumption reaches its snapshot verification.
	streamReplay bool
}

func newCraftFakeOpenCode(withhold, replay bool) *craftFakeOpenCode {
	return &craftFakeOpenCode{withholdPromptResponse: withhold, streamReplay: replay}
}

func (f *craftFakeOpenCode) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.posts
}

func (f *craftFakeOpenCode) snapshotMessages() []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]map[string]any(nil), f.messages...)
}

func (f *craftFakeOpenCode) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/session/status", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(`{"%s":{"type":"idle"}}`, craftProcOCSession)))
	})
	mux.HandleFunc("/session/"+craftProcOCSession+"/message", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(f.snapshotMessages())
	})
	mux.HandleFunc("/session/"+craftProcOCSession+"/prompt_async", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			MessageID string `json:"messageID"`
			Parts     []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"parts"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.MessageID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.posts++
		completed := time.Now().UnixMilli()
		// The external runtime accepted the prompt and completed this exact
		// round: the user message and a finished assistant child exist from
		// now on, whether or not the caller ever sees the response.
		f.messages = append(f.messages,
			map[string]any{
				"info": map[string]any{"id": body.MessageID, "parentID": "", "role": "user"},
				"parts": []map[string]any{
					{"type": "text", "text": body.Parts[0].Text},
				},
			},
			map[string]any{
				"info": map[string]any{
					"id": "asst_" + body.MessageID, "parentID": body.MessageID, "role": "assistant",
					"finish": "stop", "time": map[string]any{"completed": completed},
				},
				"parts": []map[string]any{
					{"type": "text", "text": "recovered craft round"},
				},
			})
		withhold := f.withholdPromptResponse
		f.mu.Unlock()
		if withhold {
			// Accepted, response never sent: the caller dies waiting.
			<-r.Context().Done()
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/event", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		if flusher != nil {
			_, _ = w.Write([]byte(": keepalive\n\n"))
			flusher.Flush()
		}
		if !f.streamReplay {
			<-r.Context().Done()
			return
		}
		// Wait until the round was accepted, then replay the completed
		// assistant message and the idle signal, and end the stream.
		deadline := time.Now().Add(30 * time.Second)
		for f.count() == 0 && time.Now().Before(deadline) {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(20 * time.Millisecond):
			}
		}
		messages := f.snapshotMessages()
		for _, msg := range messages {
			info := msg["info"].(map[string]any)
			if info["role"] != "assistant" {
				continue
			}
			frame := map[string]any{
				"type": "message.updated",
				"properties": map[string]any{
					"sessionID": craftProcOCSession, "info": info,
				},
			}
			raw, _ := json.Marshal(frame)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", raw)
		}
		idle, _ := json.Marshal(map[string]any{
			"type": "message.updated", "properties": map[string]any{"sessionID": craftProcOCSession},
		})
		_ = idle
		idleFrame, _ := json.Marshal(map[string]any{
			"type": "session.idle", "properties": map[string]any{"sessionID": craftProcOCSession},
		})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", idleFrame)
		if flusher != nil {
			flusher.Flush()
		}
	})
	mux.HandleFunc("/count", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"count": f.count()})
	})
	return mux
}

// ---- provider (re-exec of the test binary) ------------------------------

type craftRecoveryProcessReport struct {
	Case                string `json:"case"`
	TaskID              string `json:"task_id"`
	Status              string `json:"status"`
	Error               string `json:"error,omitempty"`
	Reused              bool   `json:"reused"`
	PostsBeforeResume   int    `json:"posts_before_resume"`
	PostsAfterResume    int    `json:"posts_after_resume"`
	StaleResultRejected bool   `json:"stale_result_rejected"`
	StaleEventRejected  bool   `json:"stale_event_rejected"`
	TakeoverEpoch       int64  `json:"takeover_epoch"`
}

func craftRecoveryProviderMain() {
	caseName := os.Getenv("CRAFT_RECOVERY_PROVIDER_CASE")
	phase := os.Getenv("CRAFT_RECOVERY_PROVIDER_PHASE")
	dbPath := os.Getenv("CRAFT_RECOVERY_DB")
	ocURL := os.Getenv("CRAFT_RECOVERY_OC_URL")
	barrier := os.Getenv("CRAFT_RECOVERY_BARRIER")
	reportPath := os.Getenv("CRAFT_RECOVERY_REPORT")
	if caseName == "" || dbPath == "" || ocURL == "" {
		fmt.Fprintln(os.Stderr, "craft recovery provider: missing env")
		os.Exit(2)
	}
	db, err := openCraftRecoveryProviderDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "craft recovery provider: open db: %v\n", err)
		os.Exit(2)
	}
	runs := repository.NewAgentRunStore(db)
	craftStore := repository.NewCraftStore(db)
	client, cerr := opencode.NewClient(ocURL, nil)
	if cerr != nil {
		fmt.Fprintf(os.Stderr, "craft recovery provider: client: %v\n", cerr)
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if phase != "resume" {
		craftRecoveryProviderCrash(ctx, caseName, runs, craftStore, client, barrier)
		select {} // the parent SIGKILLs at the barrier
	}

	report := craftRecoveryProviderResume(ctx, caseName, runs, craftStore, client, reportPath)
	raw, _ := json.Marshal(report)
	if reportPath != "" {
		_ = os.WriteFile(reportPath, append(raw, '\n'), 0o644)
	}
	fmt.Println(string(raw))
}

func craftRecoveryProviderCrash(
	ctx context.Context, caseName string,
	runs *repository.AgentRunStore, craftStore craft.Store,
	client *opencode.Client, barrier string,
) {
	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "craft recovery provider: "+format+"\n", args...)
		os.Exit(2)
	}
	key := agentruntime.RunKey{TenantID: 1, RunID: craftProcRunID}
	user, _ := json.Marshal(map[string]any{"role": "user", "content": "delegate one craft round"})
	assistant, _ := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	if _, err := runs.Admit(ctx, agentruntime.Admission{
		Key: key, SessionID: craftProcSessionID, UserID: "craft-user", RequestID: "req-1",
		AssistantMessageID: "asst-1", RequestHash: "rh",
		Snapshot:    json.RawMessage(`{"version":1,"query":"delegate","model_id":"m"}`),
		UserMessage: user, AssistantMessage: assistant,
		Deadline: time.Now().Add(10 * time.Minute),
	}); err != nil {
		fail("admit: %v", err)
	}
	// A short lease makes takeover claimable without wall-clock waiting.
	fence, err := runs.Claim(ctx, key, "worker-a", 800*time.Millisecond)
	if err != nil {
		fail("claim: %v", err)
	}
	if _, err := runs.EnsureToolPlan(ctx, fence, agentruntime.ToolPlan{
		Version: 1, CallID: craftProcCallID, Name: "craft_delegate", Identity: "craft_delegate",
		ArgsHash: "ah-1", Args: json.RawMessage(`{"goal":"recover"}`),
	}); err != nil {
		fail("ensure tool plan: %v", err)
	}
	scope := craft.Scope{TenantID: 1, UserID: "craft-user", SessionID: craftProcSessionID}
	workspace, err := craftStore.PutWorkspace(ctx, craft.Workspace{
		Scope: scope, SandboxID: "sbx-1", Generation: "1",
		OpenCodeSessionID: craftProcOCSession, RuntimeDigest: craftProcDigest,
	}, 0)
	if err != nil {
		fail("put workspace: %v", err)
	}
	promptID, err := opencode.NewMessageID()
	if err != nil {
		fail("message id: %v", err)
	}
	task := craft.Task{
		ID: craftProcDelegation, ToolCallID: craftProcCallID,
		Prompt: "goal: build the report", RequestHash: "rh-1", Scope: scope,
		Fence: fence, WorkspaceID: workspace.ID, PromptMessageID: promptID,
		Deadline: time.Now().Add(5 * time.Minute),
	}
	switch caseName {
	case craftProcCaseRecordBeforeSubmit:
		// Persist the delegation record, then die before the prompt POST.
		if _, err := craftStore.PrepareTask(ctx, task); err != nil {
			fail("prepare: %v", err)
		}
		touchCraftBarrier(barrier)
	case craftProcCaseAcceptedNoResponse:
		// Die while the accepted prompt POST response is withheld.
		svc := NewCraftDelegateService(craftStore, opencode.NewExecutor(client, craftStore, nil))
		_, _ = svc.Delegate(ctx, task)
		// Unreachable in the intended flow; stay for the kill.
	case craftProcCaseResultBeforeCheckpoint:
		// Die after the result was durably saved, before any checkpoint.
		wrapped := &craftProcBarrierStore{inner: craftStore, barrier: barrier}
		svc := NewCraftDelegateService(wrapped, opencode.NewExecutor(client, wrapped, nil))
		if _, err := svc.Delegate(ctx, task); err != nil {
			fail("delegate: %v", err)
		}
	case craftProcCaseLeaseLost:
		// Let the lease expire while the accepted prompt round is stranded.
		svc := NewCraftDelegateService(craftStore, opencode.NewExecutor(client, craftStore, nil))
		go func() { _, _ = svc.Delegate(ctx, task) }()
		if err := waitForCraftProcPosts(ocURL(), 1, 30*time.Second); err != nil {
			fail("prompt never accepted: %v", err)
		}
		time.Sleep(1500 * time.Millisecond) // 800ms lease expires
		touchCraftBarrier(barrier)
	default:
		fail("unknown case %q", caseName)
	}
}

func craftRecoveryProviderResume(
	ctx context.Context, caseName string,
	runs *repository.AgentRunStore, craftStore craft.Store,
	client *opencode.Client, reportPath string,
) craftRecoveryProcessReport {
	report := craftRecoveryProcessReport{Case: caseName}
	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "craft recovery provider: "+format+"\n", args...)
		os.Exit(2)
	}
	ocBase := os.Getenv("CRAFT_RECOVERY_OC_URL")
	report.PostsBeforeResume = craftProcCount(ocBase)
	key := agentruntime.RunKey{TenantID: 1, RunID: craftProcRunID}
	var fence agentruntime.Fence
	var err error
	deadline := time.Now().Add(30 * time.Second)
	for {
		fence, err = runs.Claim(ctx, key, "takeover-worker", 30*time.Second)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			fail("takeover claim: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	report.TakeoverEpoch = fence.Epoch
	var taskID string
	if err := craftRecoveryProviderDB().Table("craft_delegations").
		Select("id").Where("tenant_id = 1 AND run_id = ?", craftProcRunID).
		Take(&taskID).Error; err != nil {
		fail("find delegation: %v", err)
	}
	report.TaskID = taskID
	scope := craft.Scope{TenantID: 1, UserID: "craft-user", SessionID: craftProcSessionID}
	if _, gerr := craftStore.GetResult(ctx, scope, taskID); gerr == nil {
		report.Reused = true
	}
	runService := NewAgentRunService(runs)
	recovery, rerr := NewCraftRecovery(craftStore, opencode.NewExecutor(client, craftStore, nil),
		runService, CraftRunScopeQuery(craftRecoveryProviderDB()),
		CraftRecoveryConfig{RuntimeDigest: craftProcDigest, ObserveInterval: 50 * time.Millisecond})
	if rerr != nil {
		fail("assemble recovery: %v", rerr)
	}
	result, recErr := recovery.Reconcile(ctx, fence, taskID)
	report.Status = result.Status
	if recErr != nil {
		report.Error = recErr.Error()
	}
	report.PostsAfterResume = craftProcCount(ocBase)
	if caseName == craftProcCaseLeaseLost {
		stale := agentruntime.Fence{RunKey: key, Owner: "worker-a", Epoch: fence.Epoch - 1}
		report.StaleResultRejected = craftStore.SaveResult(ctx, stale,
			craft.Result{TaskID: taskID, Status: "failed", Summary: "stale late write"}) != nil
		_, eventErr := runs.AppendEvent(ctx, stale, agentruntime.RunEvent{Type: "craft"})
		report.StaleEventRejected = eventErr != nil
	}
	return report
}

// craftProcBarrierStore decorates SaveResult with the crash barrier AFTER
// the result is durably persisted (result saved, checkpoint not yet written).
type craftProcBarrierStore struct {
	inner   craft.Store
	barrier string
}

func (s *craftProcBarrierStore) GetWorkspace(ctx context.Context, sc craft.Scope) (craft.Workspace, error) {
	return s.inner.GetWorkspace(ctx, sc)
}
func (s *craftProcBarrierStore) PutWorkspace(ctx context.Context, w craft.Workspace, rev int64) (craft.Workspace, error) {
	return s.inner.PutWorkspace(ctx, w, rev)
}
func (s *craftProcBarrierStore) PrepareTask(ctx context.Context, t craft.Task) (craft.Task, error) {
	return s.inner.PrepareTask(ctx, t)
}
func (s *craftProcBarrierStore) GetTask(ctx context.Context, sc craft.Scope, id string) (craft.Task, error) {
	return s.inner.GetTask(ctx, sc, id)
}
func (s *craftProcBarrierStore) SaveResult(ctx context.Context, f agentruntime.Fence, r craft.Result) error {
	if err := s.inner.SaveResult(ctx, f, r); err != nil {
		return err
	}
	touchCraftBarrier(s.barrier)
	select {} // killed here: result stored, graph checkpoint pending
}
func (s *craftProcBarrierStore) GetResult(ctx context.Context, sc craft.Scope, id string) (craft.Result, error) {
	return s.inner.GetResult(ctx, sc, id)
}

func touchCraftBarrier(path string) {
	if path == "" {
		return
	}
	_ = os.WriteFile(path, []byte("barrier"), 0o644)
}

func craftProcCount(base string) int {
	resp, err := http.Get(base + "/count")
	if err != nil {
		return -1
	}
	defer func() { _ = resp.Body.Close() }()
	var out struct {
		Count int `json:"count"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out.Count
}

func waitForCraftProcPosts(base string, want int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if craftProcCount(base) >= want {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return fmt.Errorf("prompt post count never reached %d", want)
}

func ocURL() string { return os.Getenv("CRAFT_RECOVERY_OC_URL") }

var craftProviderDB *gorm.DB

func craftRecoveryProviderDB() *gorm.DB { return craftProviderDB }

func openCraftRecoveryProviderDB(path string) (*gorm.DB, error) {
	if craftProviderDB != nil {
		return craftProviderDB, nil
	}
	dsn := "file:" + path + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	if err != nil {
		return nil, err
	}
	migrator, err := migrate.NewWithDatabaseInstance(
		"file:"+filepath.Join(craftProcRepoRoot(), "migrations/sqlite"), "sqlite3", driver)
	if err != nil {
		return nil, err
	}
	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return nil, err
	}
	_, _ = migrator.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}
	seeds := []string{
		"INSERT OR IGNORE INTO tenants (id, name, business) VALUES (1, 'craft', 'recovery')",
		"INSERT OR IGNORE INTO users (id, username, email, password_hash, tenant_id)" +
			" VALUES ('craft-user','craft','craft@example.test','x',1)",
		"INSERT OR IGNORE INTO sessions (id, tenant_id, title, user_id, engine_type)" +
			" VALUES ('" + craftProcSessionID + "',1,'craft','craft-user','trpc')",
	}
	for _, seed := range seeds {
		if err := db.Exec(seed).Error; err != nil {
			return nil, err
		}
	}
	craftProviderDB = db
	return db, nil
}

func craftProcRepoRoot() string {
	if root := os.Getenv("CRAFT_RECOVERY_REPO_ROOT"); root != "" {
		return root
	}
	wd, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return wd
		}
		wd = parent
	}
}

// ---- the SIGKILL matrix ---------------------------------------------------

// TestCraftRecoveryProcessFailureMatrix kills real Go processes at the four
// delegation boundaries and recovers them through the real program against
// the same durable database while the external OpenCode double stays alive.
// It is opt-in (CRAFT_RECOVERY_PROCESS_MATRIX=1) because it forks this test
// binary several times.
func TestCraftRecoveryProcessFailureMatrix(t *testing.T) {
	if os.Getenv("CRAFT_RECOVERY_PROCESS_MATRIX") != "1" {
		t.Skip("CRAFT_RECOVERY_PROCESS_MATRIX unset: craft SIGKILL recovery matrix NOT VERIFIED")
	}
	cases := []struct {
		name              string
		withhold          bool
		replay            bool
		killOnBarrier     bool
		killOnPost        int
		wantStatus        string
		wantFinalPosts    int
		wantReused        bool
		wantStaleRejected bool
	}{
		{
			name: craftProcCaseRecordBeforeSubmit, killOnBarrier: true,
			wantStatus: "failed", wantFinalPosts: 0,
		},
		{
			name: craftProcCaseAcceptedNoResponse, withhold: true, killOnPost: 1,
			wantStatus: "succeeded", wantFinalPosts: 1,
		},
		{
			name: craftProcCaseResultBeforeCheckpoint, replay: true, killOnBarrier: true,
			wantStatus: "succeeded", wantFinalPosts: 1, wantReused: true,
		},
		{
			name: craftProcCaseLeaseLost, withhold: true, killOnBarrier: true, killOnPost: 1,
			wantStatus: "succeeded", wantFinalPosts: 1, wantStaleRejected: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			dbPath := filepath.Join(dir, "craft-recovery.db")
			barrierPath := filepath.Join(dir, "barrier")
			reportPath := filepath.Join(dir, "report.json")
			fake := newCraftFakeOpenCode(tc.withhold, tc.replay)
			server := httptest.NewServer(fake.handler())
			defer server.Close()

			runChild := func(phase string) craftRecoveryProcessReport {
				cmd := exec.Command(os.Args[0], "-test.run=TestCraftRecoveryProcessFailureMatrix", "-test.v=false")
				cmd.Env = append(os.Environ(),
					"CRAFT_RECOVERY_PROVIDER_CASE="+tc.name,
					"CRAFT_RECOVERY_PROVIDER_PHASE="+phase,
					"CRAFT_RECOVERY_DB="+dbPath,
					"CRAFT_RECOVERY_OC_URL="+server.URL,
					"CRAFT_RECOVERY_BARRIER="+barrierPath,
					"CRAFT_RECOVERY_REPORT="+reportPath,
					"CRAFT_RECOVERY_RUNTIME_DIGEST="+craftProcDigest,
				)
				cmd.Dir = craftProcRepoRoot() + "/internal/application/service"
				var stderr strings.Builder
				cmd.Stderr = &stderr
				if phase != "resume" {
					if err := cmd.Start(); err != nil {
						t.Fatalf("start crash provider: %v", err)
					}
					done := make(chan error, 1)
					go func() { done <- cmd.Wait() }()
					killPointReached := func() bool {
						if tc.killOnPost > 0 && fake.count() < tc.killOnPost {
							return false
						}
						if tc.killOnBarrier {
							if _, err := os.Stat(barrierPath); err != nil {
								return false
							}
						}
						return true
					}
					deadline := time.Now().Add(120 * time.Second)
					for !killPointReached() {
						select {
						case werr := <-done:
							t.Fatalf("case %s: crash provider exited before its kill point: %v stderr=%s",
								tc.name, werr, stderr.String())
						default:
						}
						if time.Now().After(deadline) {
							_ = cmd.Process.Kill()
							<-done
							t.Fatalf("case %s: kill point never reached (posts=%d) stderr=%s",
								tc.name, fake.count(), stderr.String())
						}
						time.Sleep(20 * time.Millisecond)
					}
					if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
						t.Fatalf("kill crash provider: %v", err)
					}
					<-done
					return craftRecoveryProcessReport{}
				}
				output, err := cmd.Output()
				if err != nil {
					t.Fatalf("resume provider failed: %v stderr=%s output=%s", err, stderr.String(), output)
				}
				var report craftRecoveryProcessReport
				if jerr := json.Unmarshal(output, &report); jerr != nil {
					raw, rerr := os.ReadFile(reportPath)
					if rerr != nil {
						t.Fatalf("decode resume report: %v (stdout %s)", jerr, output)
					}
					if jerr2 := json.Unmarshal(raw, &report); jerr2 != nil {
						t.Fatalf("decode resume report file: %v", jerr2)
					}
				}
				return report
			}

			postsBeforeKill := func() int { return fake.count() }
			runChild("crash")
			afterKill := postsBeforeKill()
			if tc.killOnPost > 0 && afterKill != tc.killOnPost {
				t.Fatalf("case %s: expected %d accepted prompt(s) before the kill, got %d",
					tc.name, tc.killOnPost, afterKill)
			}
			// Let the short crash lease expire before takeover.
			time.Sleep(900 * time.Millisecond)
			report := runChild("resume")

			if report.Error != "" {
				t.Fatalf("case %s: recovery returned error: %s", tc.name, report.Error)
			}
			if report.Status != tc.wantStatus {
				t.Fatalf("case %s: recovered status %q, want %q (report %#v)",
					tc.name, report.Status, tc.wantStatus, report)
			}
			if final := fake.count(); final != tc.wantFinalPosts {
				t.Fatalf("case %s: prompt POST count after recovery = %d, want %d (already accepted tasks are never re-sent)",
					tc.name, final, tc.wantFinalPosts)
			}
			if report.Reused != tc.wantReused {
				t.Fatalf("case %s: reused=%v, want %v", tc.name, report.Reused, tc.wantReused)
			}
			if tc.wantStaleRejected && !(report.StaleResultRejected && report.StaleEventRejected) {
				t.Fatalf("case %s: stale epoch writes must be rejected (result=%v event=%v)",
					tc.name, report.StaleResultRejected, report.StaleEventRejected)
			}
			if report.TakeoverEpoch < 2 {
				t.Fatalf("case %s: takeover epoch %d must exceed the crashed worker's", tc.name, report.TakeoverEpoch)
			}
			t.Logf("case %s: barrier kill at posts=%d; recovered status=%s reused=%v final posts=%d stale(result=%v event=%v) takeover epoch=%d",
				tc.name, afterKill, report.Status, report.Reused, fake.count(),
				report.StaleResultRejected, report.StaleEventRejected, report.TakeoverEpoch)
		})
	}
}
