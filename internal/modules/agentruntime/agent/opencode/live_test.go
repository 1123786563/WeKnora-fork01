package opencode

// R07 live vertical acceptance: two real delegation rounds against the
// pinned OpenCode binary, the real SQLite persistence chain and a
// controlled local model endpoint. Everything below lives in the test
// binary only; no production backdoor is introduced.
//
// TestLiveCraftTwoTurns is gated on CRAFT_LIVE=1 because it boots a real
// opencode serve process. Without that flag it skips with an explicit
// "NOT verified" reason - a skip never counts as acceptance.

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/craft"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// LiveReport is the evidence record produced by one live two-turn run.
type LiveReport struct {
	FirstSession, SecondSession, FirstHash, SecondHash string
	PromptPosts                                        int
	Checks                                             []craft.Check
}

// ValidTwoTurn accepts a report only when both rounds ran in the same
// OpenCode session, both file digests exist and differ, and exactly two
// prompt posts were submitted.
func ValidTwoTurn(r LiveReport) bool {
	return r.FirstSession != "" && r.FirstSession == r.SecondSession &&
		r.FirstHash != "" && r.SecondHash != "" && r.FirstHash != r.SecondHash && r.PromptPosts == 2
}

func TestLiveReportRejectsFreshSession(t *testing.T) {
	r := LiveReport{FirstSession: "s1", SecondSession: "s2", FirstHash: "a", SecondHash: "b", PromptPosts: 2}
	if ValidTwoTurn(r) {
		t.Fatal("new session disguised as continuation")
	}
	r.SecondSession = "s1"
	if !ValidTwoTurn(r) {
		t.Fatal("valid continuation rejected")
	}
}

// ---------------------------------------------------------------------------
// Controlled model endpoint (OpenAI-compatible, local, no real key).
// ---------------------------------------------------------------------------

const (
	liveMonthlyPrompt   = "读取当前目录的 sales.csv，生成按月收入报告，把报告写入 report.md。"
	liveQuarterlyPrompt = "把 report.md 改写为季度汇总。"
	liveMonthlyReport   = "# 按月收入报告\n\n- 2026-01 东区 100\n- 2026-02 西区 200\n"
	liveQuarterlyReport = "# 季度汇总\n\n- 2026-Q1 总计 300（东区 100 + 西区 200）\n"
	liveMonthlyFinal    = "已按月生成 report.md（2026-01 东区 100，2026-02 西区 200）。"
	liveQuarterlyFinal  = "已把 report.md 改写为季度汇总（2026-Q1 总计 300）。"
)

// craftMockProvider plays the model for both turns: the first response of a
// turn streams an OpenAI tool_calls delta asking the runtime's real write
// tool to create report.md; once the tool result comes back it streams the
// final text. The figures mirror testdata/sales.csv.
type craftMockProvider struct {
	hits  atomic.Int32
	mu    sync.Mutex
	roles []string // "call<N>:role,role,..." protocol evidence per request
}

type mockChatMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// turnView walks the conversation backwards past tool results to the last
// real user prompt, returning that prompt text and whether the current turn
// already carries a tool result.
func (m *craftMockProvider) turnView(msgs []mockChatMessage) (string, bool) {
	lastUser, toolDone := "", false
	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]
		raw := string(msg.Content)
		if msg.Role == "tool" || strings.Contains(raw, "tool-result") {
			toolDone = true
			continue
		}
		if msg.Role != "user" {
			continue
		}
		var blocks []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(msg.Content, &blocks); err == nil && len(blocks) > 0 {
			for _, block := range blocks {
				if block.Type == "text" && block.Text != "" {
					lastUser = block.Text
				}
			}
		} else {
			lastUser = strings.Trim(raw, "\"")
		}
		break
	}
	return lastUser, toolDone
}

func (m *craftMockProvider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/chat/completions") {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	raw, err := readBounded(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	r.Body.Close()
	var request struct {
		Model    string            `json:"model"`
		Messages []mockChatMessage `json:"messages"`
	}
	_ = json.Unmarshal(raw, &request)
	n := int(m.hits.Add(1))
	{
		var b strings.Builder
		fmt.Fprintf(&b, "call%d:", n)
		for i, msg := range request.Messages {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(msg.Role)
		}
		m.mu.Lock()
		m.roles = append(m.roles, b.String())
		m.mu.Unlock()
	}
	lastUser, toolDone := m.turnView(request.Messages)

	w.Header().Set("Content-Type", "text/event-stream")
	flusher := w.(http.Flusher)
	send := func(delta map[string]any, finish string) {
		choice := map[string]any{"index": 0, "delta": delta}
		if finish != "" {
			choice["finish_reason"] = finish
		}
		frame := map[string]any{
			"id": fmt.Sprintf("chatcmpl-craft-%d", n), "object": "chat.completion.chunk",
			"created": time.Now().Unix(), "model": request.Model, "choices": []any{choice},
		}
		data, _ := json.Marshal(frame)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
	send(map[string]any{"role": "assistant", "content": ""}, "")
	switch {
	case toolDone:
		if strings.Contains(lastUser, "季度") {
			send(map[string]any{"content": liveQuarterlyFinal}, "stop")
		} else {
			send(map[string]any{"content": liveMonthlyFinal}, "stop")
		}
	default:
		content := liveMonthlyReport
		if strings.Contains(lastUser, "季度") {
			content = liveQuarterlyReport
		}
		args, _ := json.Marshal(map[string]string{"filePath": "report.md", "content": content})
		send(map[string]any{"tool_calls": []any{map[string]any{
			"index": 0, "id": fmt.Sprintf("toolu-craft-%d", n), "type": "function",
			"function": map[string]any{"name": "write", "arguments": string(args)},
		}}}, "tool_calls")
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// ---------------------------------------------------------------------------
// Live serve boot (XDG isolated; never touches the user's real config).
// ---------------------------------------------------------------------------

// liveServeEnv scrubs credential-shaped variables so no real API key can be
// consumed even if the wider environment carries one.
func liveServeEnv(extra map[string]string) []string {
	var env []string
	for _, entry := range os.Environ() {
		name := entry
		if i := strings.IndexByte(entry, '='); i >= 0 {
			name = entry[:i]
		}
		up := strings.ToUpper(name)
		if strings.Contains(up, "API_KEY") || strings.Contains(up, "TOKEN") || strings.Contains(up, "SECRET") {
			continue
		}
		env = append(env, entry)
	}
	for name, value := range extra {
		env = append(env, name+"="+value)
	}
	return env
}

// startCraftServe boots the pinned opencode serve with an isolated XDG
// triple, working in workDir, and returns a client bound to an optional
// instrumented http.Client.
func startCraftServe(t *testing.T, config map[string]any, workDir string, h *http.Client) *Client {
	t.Helper()
	requireLockedBinary(t)
	// The config dir is ALWAYS an isolated temp dir so the user's real
	// opencode config, providers and keys can never load.
	configDir := t.TempDir()
	if config != nil {
		raw, err := json.Marshal(config)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(configDir, "opencode"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(configDir, "opencode", "opencode.json"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for attempt := 0; attempt < 3; attempt++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("reserve port: %v", err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		base := "http://127.0.0.1:" + strconv.Itoa(port)

		extra := map[string]string{
			"XDG_DATA_HOME": t.TempDir(), "XDG_STATE_HOME": t.TempDir(),
			"XDG_CONFIG_HOME": configDir,
		}
		cmd := exec.Command(lockedOpenCodeBinary, "serve", "--port", strconv.Itoa(port), "--hostname", "127.0.0.1")
		cmd.Dir = workDir
		cmd.Env = liveServeEnv(extra)
		logs := &cappedBuffer{max: 8 << 10}
		cmd.Stdout, cmd.Stderr = logs, logs
		if err := cmd.Start(); err != nil {
			t.Fatalf("start craft serve: %v", err)
		}
		t.Cleanup(func() {
			if cmd.Process != nil {
				cmd.Process.Kill()
				cmd.Wait()
			}
		})
		deadline := time.Now().Add(60 * time.Second)
		for time.Now().Before(deadline) {
			response, err := http.Get(base + "/doc")
			if err == nil {
				response.Body.Close()
				if response.StatusCode == http.StatusOK {
					client, err := NewClient(base, h)
					if err != nil {
						t.Fatal(err)
					}
					return client
				}
			}
			if cmd.ProcessState != nil {
				break
			}
			time.Sleep(300 * time.Millisecond)
		}
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
		t.Logf("craft serve attempt %d did not become ready; logs: %s", attempt, logs.String())
	}
	t.Fatal("craft OpenCode serve never became ready")
	return nil
}

// ---------------------------------------------------------------------------
// Real SQLite persistence chain (G1 entry).
// ---------------------------------------------------------------------------

// openCraftLiveDB applies the real SQLite migration chain (000041_craft,
// 000042_craft_versions included) and seeds the tenant/user/session rows
// the run and craft foreign keys require.
func openCraftLiveDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate live_test.go")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	dbPath := filepath.Join(t.TempDir(), "craft-live.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"

	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	if err != nil {
		t.Fatalf("sqlite migrate driver: %v", err)
	}
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot, "migrations", "sqlite"), "sqlite3", driver)
	if err != nil {
		t.Fatalf("sqlite migrator: %v", err)
	}
	if err := migrator.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("apply sqlite migrations: %v", err)
	}
	_, _ = migrator.Close()

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}
	for _, seed := range []string{
		"INSERT INTO tenants (id, name, business) VALUES (1, 'tenant-1', 'test')",
		"INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES ('u1', 'u1', 'u1@example.test', 'x', 1)",
		"INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s1', 1, 'session-1', 'u1', 'trpc')",
	} {
		if err := db.Exec(seed).Error; err != nil {
			t.Fatalf("seed %q: %v", seed, err)
		}
	}
	t.Cleanup(func() {
		if conn, e := db.DB(); e == nil {
			_ = conn.Close()
		}
	})
	return db
}

// seedLiveRun admits, claims and plans one real agent run so a delegation
// satisfies the agent_runs / agent_tool_calls foreign keys through the
// production APIs.
func seedLiveRun(t *testing.T, runs *repository.AgentRunStore, runID, callID string) agentruntime.Fence {
	t.Helper()
	ctx := context.Background()
	_, err := runs.Admit(ctx, agentruntime.Admission{
		Key:                agentruntime.RunKey{TenantID: 1, RunID: runID},
		SessionID:          "s1",
		UserID:             "u1",
		RequestID:          "craft-live-" + runID,
		AssistantMessageID: "craft-live-a-" + runID,
		RequestHash:        "craft-live-h-" + runID,
		Snapshot:           json.RawMessage(`{"version":1,"craft":true}`),
		UserMessage:        json.RawMessage(`{"role":"user","content":"craft live two turns"}`),
		AssistantMessage:   json.RawMessage(`{"role":"assistant","content":""}`),
		Deadline:           time.Now().Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("admit run %s: %v", runID, err)
	}
	fence, err := runs.Claim(ctx, agentruntime.RunKey{TenantID: 1, RunID: runID}, "craft-live-worker", time.Hour)
	if err != nil {
		t.Fatalf("claim run %s: %v", runID, err)
	}
	_, err = runs.EnsureToolPlan(ctx, fence, agentruntime.ToolPlan{
		CallID: callID, Name: "craft_delegate", Identity: "craft",
		ArgsHash: "args-" + callID, Args: json.RawMessage(`{"kind":"craft"}`),
	})
	if err != nil {
		t.Fatalf("plan tool call %s: %v", callID, err)
	}
	return fence
}

// ---------------------------------------------------------------------------
// Prompt-post instrumentation.
// ---------------------------------------------------------------------------

// promptCountingTransport counts real POST /session/{id}/prompt_async
// requests issued by the executor and records which session ids received
// them.
type promptCountingTransport struct {
	mu       sync.Mutex
	posts    int
	sessions map[string]bool
}

func (p *promptCountingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/prompt_async") {
		segments := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if len(segments) == 3 && segments[0] == "session" {
			p.mu.Lock()
			p.posts++
			if p.sessions == nil {
				p.sessions = map[string]bool{}
			}
			p.sessions[segments[1]] = true
			p.mu.Unlock()
		}
	}
	return http.DefaultTransport.RoundTrip(r)
}

func (p *promptCountingTransport) snapshot() (int, []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	ids := make([]string, 0, len(p.sessions))
	for id := range p.sessions {
		ids = append(ids, id)
	}
	return p.posts, ids
}

// ---------------------------------------------------------------------------
// Live two-turn flow.
// ---------------------------------------------------------------------------

func sha256FileLive(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// liveAssistantText returns the merged text of the assistant reply to one
// prompt and whether a round of that prompt closed with finish=stop. Tool
// loops can project several assistant messages for one prompt; the
// completed one wins, mirroring the executor's assistant selection.
func liveAssistantText(t *testing.T, messages []Message, parentID string) (string, bool) {
	t.Helper()
	var anyText, bestText string
	bestStop := false
	for _, message := range messages {
		if message.Role != "assistant" || message.ParentID != parentID {
			continue
		}
		var text string
		for _, part := range message.Parts {
			var probe struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if json.Unmarshal(part, &probe) == nil && probe.Type == "text" {
				text = probe.Text
			}
		}
		if text != "" {
			anyText = text
		}
		if message.Finish == "stop" && message.CompletedAt > 0 {
			bestText, bestStop = text, true
		}
	}
	if bestStop {
		return bestText, true
	}
	return anyText, false
}

// runLiveTwoTurns executes the mandated scenario end to end: same workspace,
// same OpenCode sub-session, two delegated rounds through the real executor
// and the real SQLite store, with every outcome read back from real HTTP
// responses and persisted rows - never from values the test wrote itself.
func runLiveTwoTurns(t *testing.T) LiveReport {
	t.Helper()
	ctx := context.Background()

	workDir := t.TempDir()
	fixture, err := os.ReadFile(filepath.Join("testdata", "sales.csv"))
	if err != nil {
		t.Fatalf("read sales.csv fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "sales.csv"), fixture, 0o644); err != nil {
		t.Fatal(err)
	}

	model := &craftMockProvider{}
	modelServer := httptest.NewServer(model)
	defer modelServer.Close()

	counting := &promptCountingTransport{}
	client := startCraftServe(t, map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"model":   "mock/mock-model",
		"provider": map[string]any{
			"mock": map[string]any{
				"npm":  "@ai-sdk/openai-compatible",
				"name": "Local Mock",
				"options": map[string]any{
					"baseURL": modelServer.URL + "/v1",
					"apiKey":  "local-mock-not-a-real-key",
				},
				"models": map[string]any{
					"mock-model": map[string]any{"name": "Mock Model"},
				},
			},
		},
		"permission": map[string]any{"edit": "allow", "bash": map[string]any{"*": "allow"}},
	}, workDir, &http.Client{Transport: counting})

	db := openCraftLiveDB(t)
	store := repository.NewCraftStore(db)
	runs := repository.NewAgentRunStore(db)
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}

	ocSession, err := client.CreateSession(ctx)
	if err != nil {
		t.Fatalf("live CreateSession: %v", err)
	}
	if !sessionIDPattern.MatchString(ocSession) {
		t.Fatalf("live session id %q does not follow the pinned format", ocSession)
	}
	workspace, err := store.PutWorkspace(ctx, craft.Workspace{
		Scope:             scope,
		SandboxID:         "sbx-live-1",
		Generation:        "1",
		OpenCodeSessionID: ocSession,
		RuntimeDigest:     "sha256:" + protocolLockDigest(),
	}, 0)
	if err != nil {
		t.Fatalf("PutWorkspace: %v", err)
	}

	// One active run per session: the first run is admitted, executed and
	// finalized (releasing the session slot) before the second run is
	// admitted - exactly how two user turns share one session.
	fenceFirst := seedLiveRun(t, runs, "r-craft-live-a", "call-live-a")

	var eventMu sync.Mutex
	events := map[string][]string{}
	emit := func(_ context.Context, task craft.Task, kind string, _ json.RawMessage) error {
		eventMu.Lock()
		events[task.ID] = append(events[task.ID], kind)
		eventMu.Unlock()
		return nil
	}
	executor := NewExecutor(client, store, emit)

	executeRound := func(name, callID, prompt, requestHash string, fence agentruntime.Fence) craft.Result {
		t.Helper()
		result, err := executor.Execute(ctx, craft.Task{
			ToolCallID: callID, Prompt: prompt, RequestHash: requestHash,
			Scope: scope, Fence: fence, WorkspaceID: workspace.ID,
			Deadline: time.Now().Add(8 * time.Minute),
		})
		if err != nil {
			t.Fatalf("live Execute %s: %v (result %#v)", name, err, result)
		}
		if result.Status != "succeeded" {
			t.Fatalf("live Execute %s status = %q, summary %q", name, result.Status, result.Summary)
		}
		if result.Summary == "" {
			t.Fatalf("live Execute %s produced an empty summary", name)
		}
		return result
	}

	first := executeRound("first turn", "call-live-a", liveMonthlyPrompt, "rh-live-monthly", fenceFirst)
	firstHash := sha256FileLive(t, filepath.Join(workDir, "report.md"))
	bindingFirst, err := store.GetWorkspace(ctx, scope)
	if err != nil {
		t.Fatalf("GetWorkspace after first turn: %v", err)
	}
	// The first user turn closes: the run is finalized through the
	// production API, which also releases the one-active-run-per-session
	// slot so the second turn of the same session can be admitted.
	if err := runs.Finalize(ctx, fenceFirst, json.RawMessage(`{"content":"turn one closed"}`)); err != nil {
		t.Fatalf("finalize first run: %v", err)
	}

	fenceSecond := seedLiveRun(t, runs, "r-craft-live-b", "call-live-b")
	second := executeRound("second turn", "call-live-b", liveQuarterlyPrompt, "rh-live-quarterly", fenceSecond)
	secondHash := sha256FileLive(t, filepath.Join(workDir, "report.md"))
	bindingSecond, err := store.GetWorkspace(ctx, scope)
	if err != nil {
		t.Fatalf("GetWorkspace after second turn: %v", err)
	}
	if err := runs.Finalize(ctx, fenceSecond, json.RawMessage(`{"content":"turn two closed"}`)); err != nil {
		t.Fatalf("finalize second run: %v", err)
	}
	reportBytes, err := os.ReadFile(filepath.Join(workDir, "report.md"))
	if err != nil {
		t.Fatalf("read report.md after second turn: %v", err)
	}

	// Prompt discipline: exactly two prompt posts, both into the same
	// OpenCode session.
	posts, sessionIDs := counting.snapshot()

	// Both rounds are traceable through the persisted chain.
	persistedFirst, err := store.GetTask(ctx, scope, first.TaskID)
	if err != nil {
		t.Fatalf("GetTask first: %v", err)
	}
	persistedSecond, err := store.GetTask(ctx, scope, second.TaskID)
	if err != nil {
		t.Fatalf("GetTask second: %v", err)
	}
	savedFirst, err := store.GetResult(ctx, scope, first.TaskID)
	if err != nil {
		t.Fatalf("GetResult first: %v", err)
	}
	savedSecond, err := store.GetResult(ctx, scope, second.TaskID)
	if err != nil {
		t.Fatalf("GetResult second: %v", err)
	}
	var runRows, delegationRows int64
	if err := db.Raw("SELECT COUNT(*) FROM agent_runs WHERE tenant_id = 1 AND run_id IN ('r-craft-live-a','r-craft-live-b')").Scan(&runRows).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Raw("SELECT COUNT(*) FROM craft_delegations WHERE tool_call_id IN ('call-live-a','call-live-b') AND status = 'succeeded'").Scan(&delegationRows).Error; err != nil {
		t.Fatal(err)
	}

	// The same OpenCode session carries both rounds: four messages, both
	// user ids adopted, ascending ids, assistant replies closed with stop.
	messages, err := client.Messages(ctx, ocSession)
	if err != nil {
		t.Fatalf("live Messages: %v", err)
	}
	var userFirst, userSecond bool
	for _, message := range messages {
		switch message.ID {
		case persistedFirst.PromptMessageID:
			userFirst = message.Role == "user"
		case persistedSecond.PromptMessageID:
			userSecond = message.Role == "user"
		}
	}
	firstText, firstStop := liveAssistantText(t, messages, persistedFirst.PromptMessageID)
	secondText, secondStop := liveAssistantText(t, messages, persistedSecond.PromptMessageID)

	// Hard assertions first: everything below must actually hold.
	if bindingFirst.OpenCodeSessionID != ocSession || bindingSecond.OpenCodeSessionID != ocSession {
		t.Fatalf("workspace session drifted: first=%q second=%q want %q",
			bindingFirst.OpenCodeSessionID, bindingSecond.OpenCodeSessionID, ocSession)
	}
	if posts != 2 || len(sessionIDs) != 1 || sessionIDs[0] != ocSession {
		t.Fatalf("prompt posts = %d to %v; want exactly 2 to [%s]", posts, sessionIDs, ocSession)
	}
	if savedFirst.Status != "succeeded" || savedSecond.Status != "succeeded" || savedFirst.TaskID == savedSecond.TaskID {
		t.Fatalf("results not two distinct succeeded delegations: %#v / %#v", savedFirst, savedSecond)
	}
	if savedFirst.Summary == savedSecond.Summary {
		t.Fatalf("both rounds stored the same summary %q", savedFirst.Summary)
	}
	if runRows != 2 || delegationRows != 2 {
		t.Fatalf("persistence rows: agent_runs=%d craft_delegations=%d; want 2 and 2", runRows, delegationRows)
	}
	if len(messages) < 4 || !userFirst || !userSecond {
		t.Fatalf("session %s does not carry both rounds: %d messages, adopted %v/%v", ocSession, len(messages), userFirst, userSecond)
	}
	if persistedSecond.PromptMessageID <= persistedFirst.PromptMessageID {
		t.Fatalf("prompt message ids are not ascending: %q then %q", persistedFirst.PromptMessageID, persistedSecond.PromptMessageID)
	}
	if !firstStop || !secondStop {
		var projection []string
		for _, message := range messages {
			projection = append(projection, fmt.Sprintf("%s:%s:parent=%s:finish=%s", message.ID, message.Role, message.ParentID, message.Finish))
		}
		t.Fatalf("assistant rounds not closed with stop: %q / %q; messages: %v (want parents %s / %s)",
			firstText, secondText, projection, persistedFirst.PromptMessageID, persistedSecond.PromptMessageID)
	}
	if !strings.Contains(firstText, "report.md") || !strings.Contains(secondText, "report.md") {
		t.Fatalf("assistant replies do not reference report.md: %q / %q", firstText, secondText)
	}
	if !strings.Contains(string(reportBytes), "季度") {
		t.Fatalf("report.md after the second turn is not the quarterly version: %q", reportBytes)
	}
	if model.hits.Load() < 4 {
		t.Fatalf("controlled model endpoint saw %d calls; want at least 4 (tool call + final per turn)", model.hits.Load())
	}
	eventMu.Lock()
	for _, id := range []string{first.TaskID, second.TaskID} {
		kinds := events[id]
		var started, finished bool
		for _, kind := range kinds {
			started = started || kind == "delegation.started"
			finished = finished || kind == "delegation.finished"
		}
		if !started || !finished {
			t.Fatalf("delegation %s events %v miss started/finished", id, kinds)
		}
	}
	eventMu.Unlock()
	model.mu.Lock()
	modelRoles := append([]string(nil), model.roles...)
	model.mu.Unlock()
	t.Logf("controlled model protocol per call: %v", modelRoles)
	t.Logf("turn summaries: first=%q second=%q", savedFirst.Summary, savedSecond.Summary)

	return LiveReport{
		FirstSession:  ocSession,
		SecondSession: bindingSecond.OpenCodeSessionID,
		FirstHash:     firstHash,
		SecondHash:    secondHash,
		PromptPosts:   posts,
		Checks: []craft.Check{
			{Name: "oc_session_stable", Status: "passed",
				Detail: fmt.Sprintf("workspace binding kept session %s across both turns (revision %d)", ocSession, bindingSecond.Revision)},
			{Name: "file_hash_changed", Status: "passed",
				Detail: fmt.Sprintf("report.md sha256 %s.. -> %s..", firstHash[:16], secondHash[:16])},
			{Name: "two_prompt_posts_same_session", Status: "passed",
				Detail: fmt.Sprintf("%d prompt_async posts to %v", posts, sessionIDs)},
			{Name: "run_first_traceable", Status: "passed",
				Detail: fmt.Sprintf("run r-craft-live-a delegation %s saved %s", first.TaskID, savedFirst.Status)},
			{Name: "run_second_traceable", Status: "passed",
				Detail: fmt.Sprintf("run r-craft-live-b delegation %s saved %s", second.TaskID, savedSecond.Status)},
			{Name: "persistence_rows", Status: "passed",
				Detail: fmt.Sprintf("agent_runs=%d craft_delegations(succeeded)=%d", runRows, delegationRows)},
			{Name: "session_carries_both_rounds", Status: "passed",
				Detail: fmt.Sprintf("%d messages; user ids adopted %v/%v; stops %v/%v",
					len(messages), userFirst, userSecond, firstStop, secondStop)},
			{Name: "model_round_trips", Status: "passed",
				Detail: fmt.Sprintf("controlled endpoint called %d times", model.hits.Load())},
		},
	}
}

// protocolLockDigest returns the pinned binary digest recorded in the
// protocol lock so the workspace runtime digest is the locked one.
func protocolLockDigest() string {
	raw, err := os.ReadFile(filepath.Join("testdata", "protocol-lock.json"))
	if err != nil {
		return "unknown"
	}
	var lock map[string]any
	if err := json.Unmarshal(raw, &lock); err != nil {
		return "unknown"
	}
	digest, _ := lock["binary_sha256"].(string)
	if digest == "" {
		return "unknown"
	}
	return digest
}

// TestLiveCraftTwoTurns proves the same workspace and the same OpenCode
// sub-session survive two real delegated rounds: a monthly report first, a
// quarterly rewrite second, with both runs independently traceable.
func TestLiveCraftTwoTurns(t *testing.T) {
	if os.Getenv("CRAFT_LIVE") != "1" {
		t.Skip("CRAFT_LIVE unset: live two-turn execution NOT verified (no serve boot, no evidence collected)")
	}
	report := runLiveTwoTurns(t)
	if !ValidTwoTurn(report) {
		t.Fatalf("live two-turn report rejected by ValidTwoTurn: %#v", report)
	}
	for _, check := range report.Checks {
		if check.Status != "passed" {
			t.Errorf("check %s = %s (%s)", check.Name, check.Status, check.Detail)
		}
	}
	t.Logf("live two-turn report: session=%s report.md %s.. -> %s.. prompt_posts=%d",
		report.FirstSession, report.FirstHash[:16], report.SecondHash[:16], report.PromptPosts)

	// Independent evidence channel: an actually configured model (the
	// binary's built-in free model; no user credential is loaded because
	// the serve runs XDG-isolated with credential env scrubbed). A failure
	// here is recorded, never silently dropped, and never faked.
	check := runLiveRealModelTurn(t)
	if check.Status == "passed" {
		t.Logf("real-model check: %+v", check)
	} else {
		t.Logf("REAL-MODEL EVIDENCE NOT COLLECTED: %+v", check)
	}
}

// runLiveRealModelTurn asks a real (non-scripted) model to summarize the
// sales fixture. It uses a second isolated serve whose default model is the
// opencode built-in free model; user credentials are neither loaded nor
// consumed. Any failure is reported honestly as a not-run check.
func runLiveRealModelTurn(t *testing.T) craft.Check {
	const budget = 4 * time.Minute
	workDir := t.TempDir()
	fixture, err := os.ReadFile(filepath.Join("testdata", "sales.csv"))
	if err != nil {
		return craft.Check{Name: "real_model_turn", Status: "not_run", Detail: "fixture missing: " + err.Error()}
	}
	if err := os.WriteFile(filepath.Join(workDir, "sales.csv"), fixture, 0o644); err != nil {
		return craft.Check{Name: "real_model_turn", Status: "not_run", Detail: err.Error()}
	}
	client := startCraftServe(t, map[string]any{
		"permission": map[string]any{"edit": "allow", "bash": map[string]any{"*": "allow"}},
	}, workDir, nil)

	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	session, err := client.CreateSession(ctx)
	if err != nil {
		return craft.Check{Name: "real_model_turn", Status: "not_run", Detail: "CreateSession: " + err.Error()}
	}
	promptID, err := NewMessageID()
	if err != nil {
		return craft.Check{Name: "real_model_turn", Status: "not_run", Detail: err.Error()}
	}
	prompt := "Read sales.csv in the current directory and write monthly-summary.md with a short revenue summary by month. Do not ask any question."
	if err := client.Prompt(ctx, session, promptID, prompt); err != nil {
		return craft.Check{Name: "real_model_turn", Status: "not_run", Detail: "Prompt: " + err.Error()}
	}
	var reply string
	for reply == "" {
		messages, err := client.Messages(ctx, session)
		if err == nil {
			if text, stop := liveAssistantText(t, messages, promptID); stop {
				reply = text
				break
			}
		}
		select {
		case <-ctx.Done():
			return craft.Check{Name: "real_model_turn", Status: "not_run", Detail: "deadline exceeded before the real model finished"}
		case <-time.After(2 * time.Second):
		}
	}
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return craft.Check{Name: "real_model_turn", Status: "not_run", Detail: err.Error()}
	}
	var artifacts []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			artifacts = append(artifacts, entry.Name()+" sha256:"+sha256FileLive(t, filepath.Join(workDir, entry.Name()))[:16])
		}
	}
	if len(artifacts) == 0 {
		return craft.Check{Name: "real_model_turn", Status: "not_run",
			Detail: "model replied but wrote no markdown artifact; reply head: " + reply}
	}
	return craft.Check{Name: "real_model_turn", Status: "passed",
		Detail: fmt.Sprintf("real model wrote %v; reply head: %.160s", artifacts, reply)}
}
