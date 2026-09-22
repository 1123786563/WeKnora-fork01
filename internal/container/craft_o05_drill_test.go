package container

// O05 fault drill: snapshot restore after a REAL hard kill.
//
// Env-gated like the C04 process matrix (CRAFT_RECOVERY_PROCESS_MATRIX) and
// the C05 live smoke (CRAFT_SNAPSHOT_LIVE_SMOKE): set CRAFT_O05_DRILL=1 to
// run. Without it the drill is skipped with an explicit NOT VERIFIED reason —
// a skipped drill never counts as passed evidence.
//
// What this drill performs, end to end against PRODUCTION code paths:
//
//   - a real pinned opencode serve (isolated XDG triple, mock model provider
//     through a local OpenAI-compatible endpoint — no real credentials);
//   - a real delegation round through the R04 executor over the real SQLite
//     migration chain (workspace binding, run fence, tool plan, result);
//   - a published immutable version + a C05 snapshot captured through the
//     real service quiescence gate (using the production
//     localCraftSnapshotSource);
//   - SIGKILL of the serve process (a real crash — never a graceful exit),
//     verified dead, then a FRESH serve boot on the same persistent XDG data
//     directory (the crash-restart a hit node goes through);
//   - restore of the workspace from the persistent snapshot: files
//     materialized into a new generation with digests re-verified, the same
//     OpenCode session continued atomically, the pre-crash version still
//     byte-identical;
//   - a second real delegation round on the restored workspace (the user's
//     next step after the crash) against the new serve instance.
//
// The drill logs the O05 Step 5 observables (main-run terminal states, real
// prompt POST counts, pre-crash version SHA, user next steps) so the -v
// output is directly citable as evidence.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
	"syscall"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/opencode"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const drillOpenCodeBinary = "/Users/wuyongjun/.opencode/bin/opencode"

// drillFiles is a REAL disk-backed controlled storage for the drill: saves are
// content-addressed under a drill-owned directory and reads come back from
// disk. Only the two methods the snapshot service calls are implemented; the
// rest of FileService stays nil-embedded (never reached).
type drillFiles struct {
	interfaces.FileService
	root string
}

func (f *drillFiles) SaveBytes(_ context.Context, data []byte, tenantID uint64, name string, _ bool) (string, error) {
	sum := sha256.Sum256(data)
	ref := fmt.Sprintf("resource://drill/%d/%s/%s", tenantID, name, hex.EncodeToString(sum[:]))
	target := filepath.Join(f.root, hex.EncodeToString(sum[:]))
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return "", err
	}
	return ref, nil
}

func (f *drillFiles) GetFile(_ context.Context, ref string) (io.ReadCloser, error) {
	parts := strings.Split(ref, "/")
	if len(parts) == 0 {
		return nil, fmt.Errorf("drill files: bad ref %q", ref)
	}
	return os.Open(filepath.Join(f.root, parts[len(parts)-1]))
}

// drillSessions is the session ACL shim over the real sessions table (same
// semantics as the service test harness: tenant + owner scoped read).
type drillSessions struct {
	interfaces.SessionService
	db *gorm.DB
}

func (s *drillSessions) GetSession(ctx context.Context, id string) (*types.Session, error) {
	tenant, _ := types.TenantIDFromContext(ctx)
	user, _ := types.UserIDFromContext(ctx)
	var session types.Session
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenant, id).First(&session).Error
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	if session.UserID != user {
		return nil, fmt.Errorf("session %s not owned by caller", id)
	}
	return &session, nil
}

// drillModel is the deterministic OpenAI-compatible mock the serve uses: the
// first answer of a turn streams a write tool call creating report.md; once
// the tool result is back it streams the final text. Same contract as the
// R07 live evidence mock.
type drillModel struct {
	mu   sync.Mutex
	hits int
}

type drillChatMsg struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

func (m *drillModel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/chat/completions") {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	_ = r.Body.Close()
	var request struct {
		Model    string         `json:"model"`
		Messages []drillChatMsg `json:"messages"`
	}
	_ = json.Unmarshal(body, &request)
	m.mu.Lock()
	m.hits++
	n := m.hits
	m.mu.Unlock()

	lastUser, toolDone := "", false
	for i := len(request.Messages) - 1; i >= 0; i-- {
		msg := request.Messages[i]
		if msg.Role == "tool" || strings.Contains(string(msg.Content), "tool-result") {
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
			lastUser = strings.Trim(string(msg.Content), `"'`)
		}
		break
	}

	w.Header().Set("Content-Type", "text/event-stream")
	flusher := w.(http.Flusher)
	send := func(delta map[string]any, finish string) {
		choice := map[string]any{"index": 0, "delta": delta}
		if finish != "" {
			choice["finish_reason"] = finish
		}
		frame := map[string]any{
			"id": fmt.Sprintf("chatcmpl-drill-%d", n), "object": "chat.completion.chunk",
			"created": time.Now().Unix(), "model": request.Model, "choices": []any{choice},
		}
		data, _ := json.Marshal(frame)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
	send(map[string]any{"role": "assistant", "content": ""}, "")
	if toolDone {
		send(map[string]any{"content": "report.md updated for the drill round."}, "stop")
	} else {
		content := "# drill v1\n\nfirst round written before the crash\n"
		if strings.Contains(lastUser, "second") {
			content = "# drill v2\n\nsecond round written after the restore\n"
		}
		args, _ := json.Marshal(map[string]string{"filePath": "report.md", "content": content})
		send(map[string]any{"tool_calls": []any{map[string]any{
			"index": 0, "id": fmt.Sprintf("toolu-drill-%d", n), "type": "function",
			"function": map[string]any{"name": "write", "arguments": string(args)},
		}}}, "tool_calls")
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// drillServe is one boot of the pinned opencode serve. The XDG data directory
// is PROVIDED by the caller so a reboot after the hard kill reopens the same
// persistent state — that is the crash-restart under drill.
type drillServe struct {
	cmd    *exec.Cmd
	base   string
	logs   bytes.Buffer
	client *opencode.Client
}

func drillServeEnv(dataDir, configDir string) []string {
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
	return append(env,
		"XDG_DATA_HOME="+dataDir,
		"XDG_STATE_HOME="+dataDir,
		"XDG_CONFIG_HOME="+configDir)
}

func bootDrillServe(t *testing.T, dataDir, configDir, workDir string, counting *drillTransport) *drillServe {
	t.Helper()
	if info, err := os.Stat(drillOpenCodeBinary); err != nil || info.IsDir() {
		t.Skipf("drill: locked OpenCode binary missing at %s: %v", drillOpenCodeBinary, err)
	}
	var httpc *http.Client
	if counting != nil {
		httpc = &http.Client{Transport: counting}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("drill: reserve port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	base := "http://127.0.0.1:" + strconv.Itoa(port)
	cmd := exec.Command(drillOpenCodeBinary, "serve", "--port", strconv.Itoa(port), "--hostname", "127.0.0.1")
	cmd.Dir = workDir
	cmd.Env = drillServeEnv(dataDir, configDir)
	srv := &drillServe{cmd: cmd, base: base}
	cmd.Stdout, cmd.Stderr = &srv.logs, &srv.logs
	if err := cmd.Start(); err != nil {
		t.Fatalf("drill: start serve: %v", err)
	}
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if cmd.ProcessState != nil {
			t.Fatalf("drill: serve exited early: %s", srv.logs.String())
		}
		response, err := http.Get(base + "/doc")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				client, err := opencode.NewClient(base, httpc)
				if err != nil {
					t.Fatalf("drill: client: %v", err)
				}
				srv.client = client
				return srv
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("drill: serve never became ready: %s", srv.logs.String())
	return nil
}

// drillTransport counts real POST /session/{id}/prompt_async requests.
type drillTransport struct {
	mu       sync.Mutex
	posts    int
	sessions map[string]bool
}

func (p *drillTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/prompt_async") {
		segments := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if len(segments) == 3 && segments[0] == "session" {
			p.mu.Lock()
			p.posts++
			p.sessions[segments[1]] = true
			p.mu.Unlock()
		}
	}
	return http.DefaultTransport.RoundTrip(r)
}

func (p *drillTransport) snapshot() (int, []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	ids := make([]string, 0, len(p.sessions))
	for id := range p.sessions {
		ids = append(ids, id)
	}
	return p.posts, ids
}

func openDrillDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("drill: cannot locate test file")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
	dbPath := filepath.Join(t.TempDir(), "craft-o05-drill.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatalf("drill: open sqlite: %v", err)
	}
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	if err != nil {
		t.Fatalf("drill: migrate driver: %v", err)
	}
	migrator, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver)
	if err != nil {
		t.Fatalf("drill: migrator: %v", err)
	}
	if err := migrator.Up(); err != nil {
		t.Fatalf("drill: migrations up: %v", err)
	}
	_, _ = migrator.Close()
	db, err := gorm.Open(gormsqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("drill: gorm open: %v", err)
	}
	for _, stmt := range []string{
		"INSERT INTO tenants (id, name, business) VALUES (1, 'drill-tenant', 'test')",
		"INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES ('u1', 'u1', 'u1@example.test', 'x', 1)",
		"INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s1', 1, 'craft-o05-drill', 'u1', 'trpc')",
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("drill: seed %q: %v", stmt, err)
		}
	}
	t.Cleanup(func() {
		if conn, e := db.DB(); e == nil {
			_ = conn.Close()
		}
	})
	return db
}

func drillCtx() context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	return context.WithValue(ctx, types.UserIDContextKey, "u1")
}

func drillSeedRun(t *testing.T, runs *repository.AgentRunStore, runID, callID string) agentruntime.Fence {
	t.Helper()
	ctx := context.Background()
	if _, err := runs.Admit(ctx, agentruntime.Admission{
		Key:       agentruntime.RunKey{TenantID: 1, RunID: runID},
		SessionID: "s1", UserID: "u1",
		RequestID: "drill-" + runID, AssistantMessageID: "drill-a-" + runID,
		RequestHash:      "drill-h-" + runID,
		Snapshot:         json.RawMessage(`{"version":1,"craft":true}`),
		UserMessage:      json.RawMessage(`{"role":"user","content":"drill"}`),
		AssistantMessage: json.RawMessage(`{"role":"assistant","content":""}`),
		Deadline:         time.Now().Add(2 * time.Hour),
	}); err != nil {
		t.Fatalf("drill: admit run %s: %v", runID, err)
	}
	fence, err := runs.Claim(ctx, agentruntime.RunKey{TenantID: 1, RunID: runID}, "drill-worker", time.Hour)
	if err != nil {
		t.Fatalf("drill: claim run %s: %v", runID, err)
	}
	if _, err := runs.EnsureToolPlan(ctx, fence, agentruntime.ToolPlan{
		CallID: callID, Name: "craft_delegate", Identity: "craft",
		ArgsHash: "args-" + callID, Args: json.RawMessage(`{"kind":"craft"}`),
	}); err != nil {
		t.Fatalf("drill: plan tool call %s: %v", callID, err)
	}
	return fence
}

// TestCraftO05DrillSnapshotRestoreAfterHardKill is the O05 Step 5 drill that
// must restore from a persistent snapshot following a REAL crash.
func TestCraftO05DrillSnapshotRestoreAfterHardKill(t *testing.T) {
	if os.Getenv("CRAFT_O05_DRILL") != "1" {
		t.Skip("O05 drill not requested (set CRAFT_O05_DRILL=1); SNAPSHOT-RESTORE-AFTER-HARD-KILL NOT VERIFIED in this run")
	}

	ctx := drillCtx()
	root := t.TempDir()
	dataDir := filepath.Join(root, "serve-data")
	configDir := filepath.Join(root, "serve-config")
	workDir := filepath.Join(root, "workspace")
	sessionsRoot := filepath.Join(root, "sessions")
	blobs := filepath.Join(root, "blobs")
	for _, dir := range []string{dataDir, configDir, workDir, sessionsRoot, blobs} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	model := &drillModel{}
	modelServer := httptest.NewServer(model)
	defer modelServer.Close()
	if err := os.MkdirAll(filepath.Join(configDir, "opencode"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"model":   "mock/mock-model",
		"provider": map[string]any{
			"mock": map[string]any{
				"npm": "@ai-sdk/openai-compatible", "name": "Drill Mock",
				"options": map[string]any{
					"baseURL": modelServer.URL + "/v1", "apiKey": "drill-mock-not-a-real-key",
				},
				"models": map[string]any{"mock-model": map[string]any{"name": "Mock Model"}},
			},
		},
		"permission": map[string]any{"edit": "allow", "bash": map[string]any{"*": "allow"}},
	}
	rawConfig, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "opencode", "opencode.json"), rawConfig, 0o644); err != nil {
		t.Fatal(err)
	}

	counting := &drillTransport{sessions: map[string]bool{}}

	// --- boot #1 -----------------------------------------------------------
	serve1 := bootDrillServe(t, dataDir, configDir, workDir, counting)
	t.Cleanup(func() {
		if serve1.cmd.Process != nil && serve1.cmd.ProcessState == nil {
			_ = serve1.cmd.Process.Kill()
			_, _ = serve1.cmd.Process.Wait()
		}
	})

	db := openDrillDB(t)
	store := repository.NewCraftStore(db)
	versions := repository.NewCraftVersionStore(db)
	snapshots := repository.NewCraftSnapshotStore(db)
	files := &drillFiles{root: blobs}
	const drillDigest = "sha256:drill-runtime"

	ocSession, err := serve1.client.CreateSession(ctx)
	if err != nil {
		t.Fatalf("drill: create OC session: %v", err)
	}
	workspace, err := store.PutWorkspace(ctx, craft.Workspace{
		Scope:     craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"},
		SandboxID: "sbx-drill", Generation: "1",
		OpenCodeSessionID: ocSession, RuntimeDigest: drillDigest,
	}, 0)
	if err != nil {
		t.Fatalf("drill: PutWorkspace: %v", err)
	}
	scope := workspace.Scope

	// The production snapshot source over the real client.
	rt := &localCraftRuntime{client: serve1.client, files: files, sessionsRoot: sessionsRoot}
	source := &localCraftSnapshotSource{runtime: rt, files: files}
	svc, err := service.NewCraftSnapshotService(service.CraftSnapshotConfig{
		DB: db, Sessions: &drillSessions{db: db}, Store: store, Versions: versions,
		Snapshots: snapshots, Files: files, Source: source,
		ActiveRuns: service.CraftActiveRunsQuery(db), RuntimeDigest: drillDigest,
	})
	if err != nil {
		t.Fatalf("drill: snapshot service: %v", err)
	}

	// --- round 1: a real delegation round on serve #1 -----------------------
	runs := repository.NewAgentRunStore(db)
	fence1 := drillSeedRun(t, runs, "r-drill-a", "call-drill-a")
	executor := opencode.NewExecutor(serve1.client, store, func(context.Context, craft.Task, string, json.RawMessage) error { return nil })
	result1, err := executor.Execute(ctx, craft.Task{
		ToolCallID: "call-drill-a", Prompt: "Write the first drill report.", RequestHash: "drill-rh-a",
		Scope: scope, Fence: fence1, WorkspaceID: workspace.ID,
		Deadline: time.Now().Add(8 * time.Minute),
	})
	if err != nil || result1.Status != "succeeded" {
		t.Fatalf("drill: round 1 result=%+v err=%v", result1, err)
	}
	if err := runs.Finalize(ctx, fence1, json.RawMessage(RUN1_CLOSE_JSON)); err != nil {
		t.Fatalf("drill: finalize round 1: %v", err)
	}
	posts1, sessions1 := counting.snapshot()
	t.Logf("[drill] round1 status=succeeded run=r-drill-a promptPosts=%d sessions=%v", posts1, sessions1)

	// --- publish the immutable version + capture the persistent snapshot ---
	report, err := os.ReadFile(filepath.Join(workDir, "report.md"))
	if err != nil {
		t.Fatalf("drill: read report.md after round 1: %v", err)
	}
	ref, err := files.SaveBytes(ctx, report, 1, "report.md", false)
	if err != nil {
		t.Fatalf("drill: store report blob: %v", err)
	}
	sum := sha256.Sum256(report)
	versionFiles := []craft.File{{
		Path: "report.md", Ref: ref, SHA256: hex.EncodeToString(sum[:]),
		MIME: "text/markdown", Bytes: int64(len(report)),
	}}
	digest, err := craft.ManifestDigest(versionFiles)
	if err != nil {
		t.Fatal(err)
	}
	v1, err := versions.Publish(ctx, scope, craft.Version{
		ID: craft.VersionID(workspace.ID, "r-drill-a", digest), WorkspaceID: workspace.ID,
		RunID: "r-drill-a", Kind: craft.KindWeb, Files: versionFiles,
		Checks: []craft.Check{{Name: craft.CheckEntry, Status: craft.CheckPassed}},
	})
	if err != nil {
		t.Fatalf("drill: publish v1: %v", err)
	}
	oldSHA := hex.EncodeToString(sum[:])
	t.Logf("[drill] v1 published id=%s report.md sha256=%s", v1.ID, oldSHA)

	snap, err := svc.Capture(ctx, scope, v1.ID)
	if err != nil {
		t.Fatalf("drill: capture snapshot: %v", err)
	}
	snapID := craft.SnapshotID(snap)
	t.Logf("[drill] snapshot captured id=%s filesDigest=%s sessionDigest=%s quiescent=%v", snapID, snap.FilesDigest, snap.SessionDigest, snap.Quiescent)

	// --- THE CRASH: SIGKILL serve #1 (never a graceful exit) ---------------
	if err := serve1.cmd.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatalf("drill: SIGKILL serve #1: %v", err)
	}
	if err := serve1.cmd.Wait(); err != nil {
		t.Logf("[drill] serve #1 wait: %v (expected for a signal death)", err)
	}
	if serve1.cmd.ProcessState == nil {
		t.Fatal("drill: serve #1 still running after SIGKILL")
	}
	t.Logf("[drill] serve #1 SIGKILLED exit=%v — a REAL crash, not a graceful exit", serve1.cmd.ProcessState)

	// --- crash-restart: boot #2 on the SAME persistent data dir -------------
	serve2 := bootDrillServe(t, dataDir, configDir, workDir, counting)
	t.Cleanup(func() {
		if serve2.cmd.Process != nil && serve2.cmd.ProcessState == nil {
			_ = serve2.cmd.Process.Kill()
			_, _ = serve2.cmd.Process.Wait()
		}
	})
	if _, err := serve2.client.Status(ctx, ocSession); err != nil {
		t.Fatalf("drill: the OC session did not survive the crash-restart: %v", err)
	}
	t.Logf("[drill] serve #2 booted on the same persistent data dir; OC session %s answered status", ocSession)

	// The production source must now point at the new serve instance (the
	// rebooted process owns the session data).
	rt.client = serve2.client

	// --- restore from the persistent snapshot ------------------------------
	binding, err := store.GetWorkspace(ctx, scope)
	if err != nil {
		t.Fatalf("drill: read binding before restore: %v", err)
	}
	restored, err := svc.Restore(ctx, scope, snapID, binding.Revision)
	if err != nil {
		t.Fatalf("drill: restore from persistent snapshot: %v", err)
	}
	if restored.Generation == binding.Generation {
		t.Fatalf("drill: restore kept the old generation %q", restored.Generation)
	}
	if restored.OpenCodeSessionID != ocSession {
		t.Fatalf("drill: restore changed the OC session: %q -> %q", ocSession, restored.OpenCodeSessionID)
	}
	materialized, err := os.ReadFile(filepath.Join(sessionsRoot, "restored", workspace.ID, restored.Generation, "report.md"))
	if err != nil {
		t.Fatalf("drill: restored report.md not materialized: %v", err)
	}
	materialSum := sha256.Sum256(materialized)
	if hex.EncodeToString(materialSum[:]) != oldSHA {
		t.Fatal("drill: restored report.md digest mismatch")
	}
	t.Logf("[drill] restore ok generation=%s ocSession=%s (unchanged) report.md sha256=%s (byte-identical)",
		restored.Generation, restored.OpenCodeSessionID, hex.EncodeToString(materialSum[:]))

	// The pre-crash immutable version is still byte-identical.
	kept, err := versions.Get(ctx, scope, v1.ID)
	if err != nil {
		t.Fatalf("drill: read back v1 after restore: %v", err)
	}
	reader, err := files.GetFile(ctx, kept.Files[0].Ref)
	if err != nil {
		t.Fatalf("drill: read v1 blob: %v", err)
	}
	keptBytes, _ := io.ReadAll(reader)
	_ = reader.Close()
	keptSum := sha256.Sum256(keptBytes)
	if hex.EncodeToString(keptSum[:]) != oldSHA {
		t.Fatal("drill: v1 changed after the crash")
	}
	t.Logf("[drill] pre-crash version %s still byte-identical after crash+restore", v1.ID)

	// --- the user's next step: a second real round on the restored workspace
	fence2 := drillSeedRun(t, runs, "r-drill-b", "call-drill-b")
	executor2 := opencode.NewExecutor(serve2.client, store, func(context.Context, craft.Task, string, json.RawMessage) error { return nil })
	binding2, err := store.GetWorkspace(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	result2, err := executor2.Execute(ctx, craft.Task{
		ToolCallID: "call-drill-b", Prompt: "Write the second drill report.", RequestHash: "drill-rh-b",
		Scope: scope, Fence: fence2, WorkspaceID: binding2.ID,
		Deadline: time.Now().Add(8 * time.Minute),
	})
	if err != nil || result2.Status != "succeeded" {
		t.Fatalf("drill: round 2 on restored workspace result=%+v err=%v", result2, err)
	}
	if err := runs.Finalize(ctx, fence2, json.RawMessage(RUN2_CLOSE_JSON)); err != nil {
		t.Fatalf("drill: finalize round 2: %v", err)
	}
	second, err := os.ReadFile(filepath.Join(workDir, "report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(second), "second round") {
		t.Fatalf("drill: round 2 did not rewrite report.md (content %q)", second)
	}
	postsTotal, sessionsTotal := counting.snapshot()
	if postsTotal != 2 {
		t.Fatalf("drill: prompt POST count = %d, want exactly 2 (one per round, no resubmit)", postsTotal)
	}
	if len(sessionsTotal) != 1 || sessionsTotal[0] != ocSession {
		t.Fatalf("drill: prompt POSTs hit sessions %v, want exactly [%s]", sessionsTotal, ocSession)
	}
	t.Logf("[drill] round2 status=succeeded run=r-drill-b promptPostsTotal=%d sessions=%v — the restored session continued atomically", postsTotal, sessionsTotal)
	t.Logf("[drill] DRILL COMPLETE: crash(SIGKILL) -> persistent snapshot restore -> session continued, no duplicate prompts, old version immutable")
}

const (
	RUN1_CLOSE_JSON = "{\"content\":\"round one closed\"}"
	RUN2_CLOSE_JSON = "{\"content\":\"round two closed\"}"
)
