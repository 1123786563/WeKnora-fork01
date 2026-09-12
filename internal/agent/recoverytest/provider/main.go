// Command provider is the real graph provider required by the SIGKILL
// recovery acceptance matrix. It runs the same production recovery path as the
// server: the migrated business database, the fenced AgentRunStore, the real
// durable worker, the SDK graph with the repository checkpoint saver, the
// tool journal, durable decision parking and the finalize transaction. Only
// the chat model and the external side-effect endpoint are deterministic test
// doubles, which the acceptance plan explicitly allows.
package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	trpcagent "github.com/Tencent/WeKnora/internal/agent/trpc"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	sdklog "trpc.group/trpc-go/trpc-agent-go/log"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

const (
	caseAfterAdmission      = "after_admission"
	caseAfterPlan           = "after_plan_before_dispatch"
	caseAfterSideEffect     = "after_side_effect_before_result"
	caseAfterResult         = "after_result_before_checkpoint"
	caseWaitingUser         = "waiting_user"
	caseAfterFinalize       = "after_finalize"
	caseUnknownUserRetry    = "unknown_result_user_retry"
	caseIdempotentRedeliver = "idempotent_redelivery"

	toolName    = "counting_tool"
	runID       = "matrix-run-1"
	requestID   = "matrix-request-1"
	assistantID = "matrix-assistant-1"
)

type crashReport struct {
	ExternalCalls int    `json:"external_calls"`
	FinalStatus   string `json:"final_status"`
	AssistantRows int    `json:"assistant_rows"`
	LostEvents    int    `json:"lost_events"`
	Parked        bool   `json:"parked"`
}

func main() {
	var (
		crashCase = flag.String("recovery-case", "", "durable boundary under test")
		dbPath    = flag.String("recovery-db", "", "durable sqlite database")
		barrier   = flag.String("recovery-barrier", "", "barrier file to touch")
		report    = flag.String("recovery-report", "", "report output path")
		resume    = flag.String("recovery-resume", "", "resume the durable run")
	)
	flag.Parse()
	// The resume invocation carries the case through --recovery-resume.
	if *crashCase == "" && *resume != "" {
		*crashCase = *resume
	}
	if *crashCase == "" || *dbPath == "" || *barrier == "" || *report == "" {
		fatal("all recovery flags are required")
	}
	switch *crashCase {
	case caseAfterAdmission, caseAfterPlan, caseAfterSideEffect, caseAfterResult,
		caseWaitingUser, caseAfterFinalize, caseUnknownUserRetry, caseIdempotentRedeliver,
		"after_tool_result_before_checkpoint":
	default:
		fatal("unknown recovery case " + *crashCase)
	}
	if *crashCase == "after_tool_result_before_checkpoint" {
		*crashCase = caseAfterResult
	}
	counterURL := os.Getenv("TRPC_RECOVERY_COUNTER_URL")
	if counterURL == "" {
		fatal("TRPC_RECOVERY_COUNTER_URL is required")
	}
	if os.Getenv("TRPC_RECOVERY_DEBUG") != "" {
		sdklog.SetLevel(sdklog.LevelDebug)
	}

	db, err := openMigratedDB(*dbPath)
	if err != nil {
		fatal(fmt.Sprintf("open durable database: %v", err))
	}
	store := repository.NewAgentRunStore(db)
	runs := service.NewAgentRunService(store)
	service.RegisterAgentRunService(runs)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if *resume == "" {
		if err := admitMatrixRun(ctx, store); err != nil {
			fatal(fmt.Sprintf("admit: %v", err))
		}
		if *crashCase == caseAfterAdmission {
			touchBarrier(*barrier)
			blockForever()
		}
	}

	executor := newMatrixExecutor(store, runs, counterURL, *crashCase, *barrier, *resume != "")
	worker, err := service.NewAgentRunWorker(store, executor, service.WorkerConfig{
		Enabled: true, Lease: 700 * time.Millisecond, Heartbeat: 250 * time.Millisecond,
		ScanInterval: 100 * time.Millisecond, MaxWorkers: 1,
	})
	if err != nil {
		fatal(fmt.Sprintf("worker: %v", err))
	}
	go func() { _ = worker.Run(ctx) }()

	final, parked, err := waitForSettled(ctx, store)
	if err != nil {
		fatal(fmt.Sprintf("wait for settled run: %v", err))
	}
	if parked && (*crashCase == caseUnknownUserRetry || *crashCase == caseWaitingUser) && *resume != "" {
		if *crashCase == caseUnknownUserRetry {
			// The durable decision path requires an operator policy and an
			// authenticated actor, exactly like the HTTP layer.
			runs.SetDecisionPolicy(func(context.Context, agentruntime.Decision) error { return nil })
			actorCtx := types.WithPrincipal(ctx, types.Principal{
				Type: types.PrincipalWebUser, ID: "matrix-user",
			})
			actorCtx = context.WithValue(actorCtx, types.UserIDContextKey, "matrix-user")
			actorCtx = context.WithValue(actorCtx, types.TenantIDContextKey, uint64(1))
			if _, err := applyUserRetry(actorCtx, runs, db); err != nil {
				fatal(fmt.Sprintf("user retry: %v", err))
			}
			var waitErr error
			final, parked, waitErr = waitForSettled(ctx, store)
			if waitErr != nil {
				fatal(fmt.Sprintf("wait after retry: %v", waitErr))
			}
		}
	}

	r := buildReport(ctx, db, store, counterURL, final, parked)
	raw, _ := json.Marshal(r)
	_ = os.WriteFile(*report, raw, 0o644)
	fmt.Println(string(raw))
}

func openMigratedDB(path string) (*gorm.DB, error) {
	if dsn := os.Getenv("TRPC_RECOVERY_PG_DSN"); dsn != "" && os.Getenv("TRPC_RECOVERY_USE_PG") == "1" {
		return openPostgresDB(dsn)
	}
	return openSQLiteDB(path)
}

// openPostgresDB opens an isolated schema in the configured PostgreSQL
// instance and applies the versioned migrations, so the SIGKILL matrix can
// run against the same durable semantics on the second supported dialect.
func openPostgresDB(dsn string) (*gorm.DB, error) {
	// The matrix never touches embeddings; skip the conditional migration so
	// the provider runs against stock PostgreSQL without vector extensions.
	if parsed, perr := url.Parse(dsn); perr == nil {
		query := parsed.Query()
		if query.Get("options") == "" {
			query.Set("options", "-c app.skip_embedding=true")
			parsed.RawQuery = query.Encode()
			dsn = parsed.String()
		}
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	// The schema is derived from the per-case namespace so the resume
	// process reuses the crashed process's durable state instead of
	// creating a fresh schema; a random name would lose the admitted run.
	schema := "matrix_" + sanitizeSchemaName(os.Getenv("TRPC_RECOVERY_TEST_NAMESPACE"))
	if schema == "matrix_" {
		schema = "matrix_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	var schemaExists bool
	if err := admin.Raw(
		"SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = ?)", schema,
	).Scan(&schemaExists).Error; err != nil {
		return nil, err
	}
	if !schemaExists {
		if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
			return nil, err
		}
		// Extensions are database-wide; if a previous process installed them
		// into its own case schema they are invisible to later schemas, so
		// move them to public (no-op when already there) before ensuring.
		for _, extension := range []string{"uuid-ossp", "pg_trgm"} {
			_ = admin.Exec(`ALTER EXTENSION "` + extension + `" SET SCHEMA public`).Error
			stmt := `CREATE EXTENSION IF NOT EXISTS "` + extension + `" WITH SCHEMA public`
			if err := admin.Exec(stmt).Error; err != nil {
				return nil, err
			}
		}
	}
	// Migrations must run on a connection whose search_path is the case
	// schema; the admin connection resolves unqualified names against
	// public, which leaks objects across runs and breaks idempotent DDL.
	schemaDSN := dsn + "&search_path=" + schema + ",public"
	schemaConn, err := gorm.Open(postgres.Open(schemaDSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	conn, err := schemaConn.DB()
	if err != nil {
		return nil, err
	}
	driver, err := pgmigrate.WithInstance(conn, &pgmigrate.Config{SchemaName: schema})
	if err != nil {
		return nil, err
	}
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot(), "migrations/versioned"), "postgres", driver)
	if err != nil {
		return nil, err
	}
	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return nil, err
	}
	_, _ = migrator.Close()
	_ = schemaConn
	db, err := gorm.Open(postgres.Open(dsn+"&search_path="+schema+",public"), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	seeds := []string{
		"INSERT INTO tenants (id, name, business) VALUES (1, 'matrix', 'recoverytest') ON CONFLICT DO NOTHING",
		"INSERT INTO users (id, username, email, password_hash, tenant_id)" +
			" VALUES ('matrix-user','matrix','matrix@example.test','x',1) ON CONFLICT DO NOTHING",
		"INSERT INTO sessions (id, tenant_id, title, user_id, engine_type)" +
			" VALUES ('matrix-session',1,'matrix','matrix-user','trpc') ON CONFLICT DO NOTHING",
	}
	for _, seed := range seeds {
		if err := db.Exec(seed).Error; err != nil {
			return nil, err
		}
	}
	return db, nil
}

func openSQLiteDB(path string) (*gorm.DB, error) {
	dsn := "file:" + path + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{})
	if err != nil {
		return nil, err
	}
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot(), "migrations/sqlite"), "sqlite3", driver)
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
	if err := db.Exec(
		"INSERT OR IGNORE INTO tenants (id, name, business) VALUES (1, 'matrix', 'recoverytest')").Error; err != nil {
		return nil, err
	}
	if err := db.Exec(
		"INSERT OR IGNORE INTO users (id, username, email, password_hash, tenant_id)" +
			" VALUES ('matrix-user','matrix','matrix@example.test','x',1)").Error; err != nil {
		return nil, err
	}
	if err := db.Exec(
		"INSERT OR IGNORE INTO sessions (id, tenant_id, title, user_id, engine_type)" +
			" VALUES ('matrix-session',1,'matrix','matrix-user','trpc')").Error; err != nil {
		return nil, err
	}
	return db, nil
}

// sanitizeSchemaName keeps the namespace-derived schema name a valid
// PostgreSQL identifier.
func sanitizeSchemaName(namespace string) string {
	var b strings.Builder
	for _, r := range namespace {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune('_')
		}
	}
	return b.String()
}

func repoRoot() string {
	// The provider is executed from the repository test working directory.
	if root := os.Getenv("TRPC_RECOVERY_REPO_ROOT"); root != "" {
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

func admitMatrixRun(ctx context.Context, store *repository.AgentRunStore) error {
	user, _ := json.Marshal(map[string]any{"role": "user", "content": "count one"})
	assistant, _ := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	snapshot := json.RawMessage(`{"version":1,"query":"count one","model_id":"matrix"}`)
	_, err := store.Admit(ctx, agentruntime.Admission{
		Key:                agentruntime.RunKey{TenantID: 1, RunID: runID},
		SessionID:          "matrix-session",
		UserID:             "matrix-user",
		RequestID:          requestID,
		AssistantMessageID: assistantID,
		RequestHash:        "matrix-hash",
		Snapshot:           snapshot,
		UserMessage:        user,
		AssistantMessage:   assistant,
		Deadline:           time.Now().Add(10 * time.Minute),
	})
	return err
}

// ---- deterministic chat model ----

type scriptedModel struct{}

func (*scriptedModel) GetModelName() string { return "matrix-model" }
func (*scriptedModel) GetModelID() string   { return "matrix-model" }

func (m *scriptedModel) Chat(
	_ context.Context, messages []chat.Message, _ *chat.ChatOptions,
) (*types.ChatResponse, error) {
	return m.respond(messages), nil
}

func (m *scriptedModel) ChatStream(
	_ context.Context, messages []chat.Message, _ *chat.ChatOptions,
) (<-chan types.StreamResponse, error) {
	response := m.respond(messages)
	ch := make(chan types.StreamResponse, 2)
	ch <- types.StreamResponse{
		ResponseType: types.ResponseTypeAnswer, Content: response.Content,
		ToolCalls: response.ToolCalls, Done: true, FinishReason: response.FinishReason,
	}
	close(ch)
	return ch, nil
}

func (*scriptedModel) respond(messages []chat.Message) *types.ChatResponse {
	for _, msg := range messages {
		if msg.Role == "tool" {
			return &types.ChatResponse{Content: "recovered answer", FinishReason: "stop"}
		}
	}
	return &types.ChatResponse{
		FinishReason: "tool_calls",
		ToolCalls: []types.LLMToolCall{{
			ID: "call-matrix-1", Type: "function",
			Function: types.FunctionCall{Name: toolName, Arguments: `{"tick":1}`},
		}},
	}
}

// ---- counting tool (external side effect via HTTP) ----

type countingTool struct{ counterURL string }

func (t *countingTool) Name() string        { return toolName }
func (t *countingTool) Description() string { return "increments an external counter once" }
func (t *countingTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"tick":{"type":"number"}}}`)
}

func (t *countingTool) Execute(ctx context.Context, _ json.RawMessage) (*types.ToolResult, error) {
	// Only a trusted dispatch idempotency key deduplicates; without one the
	// external call counts every execution, like a real non-idempotent side
	// effect.
	key := ""
	if dispatch, ok := agentruntime.ToolDispatchFromContext(ctx); ok && dispatch.IdempotencyKey != "" {
		key = dispatch.IdempotencyKey
	}
	body, _ := json.Marshal(map[string]string{"key": key})
	url := t.counterURL + "/incr"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return &types.ToolResult{Success: false, Error: string(payload)}, nil
	}
	return &types.ToolResult{Success: true, Output: string(payload)}, nil
}

func counterCount(counterURL string) int {
	resp, err := http.Get(counterURL + "/count")
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

// ---- barrier-aware journal ----

// barrierJournal decorates the durable journal with the crash barrier the
// matrix needs at commit boundaries. It never alters journal semantics; the
// only mutation is injecting the trusted idempotent recovery policy for the
// idempotent-redelivery case, which production would declare in its tool
// wiring.
type barrierJournal struct {
	inner              agentruntime.ToolJournal
	barrier, crashCase string
	resuming           bool
}

func (j *barrierJournal) EnsureToolPlan(
	ctx context.Context, f agentruntime.Fence, plan agentruntime.ToolPlan,
) (agentruntime.ToolRecord, error) {
	if j.crashCase == caseIdempotentRedeliver && plan.Name == toolName {
		plan.RecoveryPolicy = agentruntime.ToolRecoveryIdempotent
		plan.IdempotencyKey = "matrix-idempotent-key"
		// Deterministic expiry: every EnsureToolPlan call for this plan must
		// derive the identical idempotency window or the journal treats the
		// re-ensured plan as a conflicting revision.
		plan.IdempotencyExpiresAt = time.Unix(4102444800, 0)
	}
	return j.inner.EnsureToolPlan(ctx, f, plan)
}

func (j *barrierJournal) BeginToolAttempt(
	ctx context.Context, f agentruntime.Fence, callID string, versions ...int64,
) (agentruntime.ToolAttempt, error) {
	return j.inner.BeginToolAttempt(ctx, f, callID, versions...)
}

func (j *barrierJournal) ReviseToolPlan(
	ctx context.Context, f agentruntime.Fence, callID string, version int64, args json.RawMessage,
) (agentruntime.ToolPlan, error) {
	return j.inner.ReviseToolPlan(ctx, f, callID, version, args)
}

func (j *barrierJournal) CommitToolRejection(
	ctx context.Context, f agentruntime.Fence, callID string, result agentruntime.StoredToolResult,
) error {
	return j.inner.CommitToolRejection(ctx, f, callID, result)
}

func (j *barrierJournal) MarkToolUnknown(
	ctx context.Context, f agentruntime.Fence, attempt agentruntime.ToolAttempt, reason string,
) error {
	return j.inner.MarkToolUnknown(ctx, f, attempt, reason)
}

func (j *barrierJournal) CommitToolResult(
	ctx context.Context, f agentruntime.Fence, attempt agentruntime.ToolAttempt, result agentruntime.StoredToolResult,
) error {
	if err := j.inner.CommitToolResult(ctx, f, attempt, result); err != nil {
		return err
	}
	if j.crashCase == caseAfterResult && !j.resuming {
		touchBarrier(j.barrier)
		blockForever()
	}
	return nil
}

func (j *barrierJournal) LoadToolResult(
	ctx context.Context, f agentruntime.Fence, callID string,
) (agentruntime.StoredToolResult, error) {
	if reader, ok := j.inner.(agentruntime.ToolResultReader); ok {
		return reader.LoadToolResult(ctx, f, callID)
	}
	return agentruntime.StoredToolResult{}, agentruntime.ErrNotFound
}

// ---- executor assembly ----

func newMatrixExecutor(
	store *repository.AgentRunStore,
	runs *service.AgentRunService,
	counterURL, crashCase, barrierPath string,
	resuming bool,
) func(context.Context, agentruntime.Fence) error {
	journal := &barrierJournal{inner: store, barrier: barrierPath, crashCase: crashCase, resuming: resuming}
	registry := tools.NewToolRegistry()
	registry.RegisterTool(&countingTool{counterURL: counterURL})
	registry.PrepareMCPTools(context.Background())

	toolBridge := func(ctx context.Context, name string, args json.RawMessage) (*types.ToolResult, error) {
		if crashCase == caseAfterPlan && !resuming {
			touchBarrier(barrierPath)
			blockForever()
		}
		result, err := registry.ExecuteTool(ctx, name, args)
		if err != nil {
			return result, err
		}
		if resuming {
			return result, nil
		}
		if crashCase == caseAfterSideEffect || crashCase == caseWaitingUser ||
			crashCase == caseUnknownUserRetry || crashCase == caseIdempotentRedeliver {
			// The external side effect happened but the result must not be
			// persisted for the unknown-outcome cases; the idempotent case
			// keeps running so the journal retry path redelivers.
			if crashCase == caseIdempotentRedeliver {
				touchBarrier(barrierPath)
				blockForever()
			}
			touchBarrier(barrierPath)
			return result, fmt.Errorf("matrix: outcome unobservable after dispatch")
		}
		return result, nil
	}

	modelTools, err := trpcagent.DeclarationTools([]types.FunctionDefinition{{
		Name: toolName, Description: "increments an external counter once",
		Parameters: json.RawMessage(`{"type":"object","properties":{"tick":{"type":"number"}}}`),
	}})
	if err != nil {
		fatal(fmt.Sprintf("declarations: %v", err))
	}

	build := func() *trpcagent.GraphRunner {
		bindings := trpcagent.GraphBindings{
			Model: trpcagent.NewModel(&scriptedModel{}),
			Store: store,
			Tools: agentruntime.NewToolExecutor(store, journal, toolBridge),
			WaitForDecision: func(ctx context.Context, fence agentruntime.Fence, pending string) error {
				err := runs.WaitForDecision(ctx, fence, pending)
				if err == nil && !resuming &&
					(crashCase == caseWaitingUser || crashCase == caseUnknownUserRetry) {
					touchBarrier(barrierPath)
					blockForever()
				}
				return err
			},
			Finalize: func(ctx context.Context, fence agentruntime.Fence, answer json.RawMessage) error {
				err := store.Finalize(ctx, fence, answer)
				if err == nil && !resuming && crashCase == caseAfterFinalize {
					touchBarrier(barrierPath)
					blockForever()
				}
				return err
			},
			InitialState: trpcagent.State{
				Version:  trpcagent.StateVersion,
				Messages: []model.Message{model.NewUserMessage("count one")},
			},
			Capabilities: trpcagent.CapabilitySnapshot{ToolIdentities: []string{toolName}},
			Events:       store,
			ModelTools:   modelTools,
		}
		runner, err := trpcagent.NewGraphRunner(bindings)
		if err != nil {
			fatal(fmt.Sprintf("graph runner: %v", err))
		}
		return runner
	}
	runner := build()
	return runner.Run
}

// ---- orchestration helpers ----

func waitForSettled(ctx context.Context, store *repository.AgentRunStore) (agentruntime.Run, bool, error) {
	key := agentruntime.RunKey{TenantID: 1, RunID: runID}
	deadline := time.Now().Add(30 * time.Second)
	for {
		run, err := store.Get(ctx, key)
		if err != nil {
			return run, false, err
		}
		settled := run.Status == "succeeded" || run.Status == "failed" ||
			run.Status == "waiting_user" || run.Status == "canceled"
		if settled {
			return run, run.Status == "waiting_user", nil
		}
		if time.Now().After(deadline) {
			return run, false, fmt.Errorf("run did not settle, status=%s", run.Status)
		}
		select {
		case <-ctx.Done():
			return run, false, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func applyUserRetry(
	ctx context.Context, runs *service.AgentRunService, db *gorm.DB,
) (agentruntime.Run, error) {
	key := agentruntime.RunKey{TenantID: 1, RunID: runID}
	current, err := storeFor(runs).Get(ctx, key)
	if err != nil {
		return current, err
	}
	// The recovery card surfaces the unknown call's identity and argument
	// hash; the durable decision must bind to exactly that call version.
	var pending struct{ CallID, ArgsHash string }
	pendingQuery := "SELECT call_id, args_hash FROM agent_tool_calls" +
		" WHERE tenant_id = 1 AND run_id = ? AND status = 'unknown' LIMIT 1"
	if err := db.Raw(pendingQuery, runID).Scan(&pending).Error; err != nil {
		return current, err
	}
	return runs.Resolve(ctx, key, agentruntime.Decision{
		PendingID:        current.WaitReason,
		DecisionID:       "matrix-retry-" + uuid.NewString(),
		ToolCallID:       pending.CallID,
		ArgsHash:         pending.ArgsHash,
		Action:           "retry",
		Reason:           "operator verified the external system and accepts duplicate risk",
		ExpectedRevision: current.Revision,
	})
}

func storeFor(runs *service.AgentRunService) agentruntime.RunStore { return runs.Store() }

func buildReport(
	ctx context.Context, db *gorm.DB, store *repository.AgentRunStore,
	counterURL string, final agentruntime.Run, parked bool,
) crashReport {
	report := crashReport{
		ExternalCalls: counterCount(counterURL),
		FinalStatus:   final.Status,
		Parked:        parked,
	}
	var rows int64
	completed := "SELECT COUNT(*) FROM messages WHERE session_id = ? AND role = 'assistant' AND is_completed = 1"
	_ = db.Raw(completed, "matrix-session").Scan(&rows).Error
	report.AssistantRows = int(rows)
	events, err := store.ReadEvents(ctx, agentruntime.RunKey{TenantID: 1, RunID: runID}, 0, 1000)
	if err == nil {
		var last int64
		for i, evt := range events {
			if i > 0 && evt.Seq != last+1 {
				report.LostEvents++
			}
			last = evt.Seq
		}
	}
	return report
}

func touchBarrier(path string) {
	_ = os.WriteFile(path, []byte("barrier"), 0o644)
}

func blockForever() {
	select {}
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, "recovery provider:", message)
	os.Exit(1)
}
