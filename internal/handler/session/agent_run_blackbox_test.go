package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openHandlerRunDB opens a fully migrated SQLite database for the durable-run
// HTTP contract tests, mirroring the repository/service harnesses.
func openHandlerRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	path := filepath.Join(t.TempDir(), "handler-runs.db")
	dsn := "file:" + path + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec(
		"INSERT INTO tenants (id, name, business) VALUES (1, 'handler', 'test')").Error)
	require.NoError(t, db.Exec(
		"INSERT INTO users (id, username, email, password_hash, tenant_id)"+
			" VALUES ('web_user:u1','u1','u1@example.test','x',1)").Error)
	require.NoError(t, db.Exec(
		"INSERT INTO sessions (id, tenant_id, title, user_id, engine_type)"+
			" VALUES ('s1',1,'handler','web_user:u1','trpc')").Error)
	t.Cleanup(func() {
		conn, e := db.DB()
		if e == nil {
			_ = conn.Close()
		}
	})
	return db
}

type handlerOwnedSession struct{ interfaces.SessionService }

func (handlerOwnedSession) GetOwnedSession(context.Context, string) (*types.Session, error) {
	return &types.Session{ID: "s1", TenantID: 1, UserID: "web_user:u1", EngineType: "trpc"}, nil
}

func newRunRouter(t *testing.T, db *gorm.DB) (*gin.Engine, *repository.AgentRunStore) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	store := repository.NewAgentRunStore(db)
	h := &Handler{sessionService: handlerOwnedSession{}}
	runSvc := service.NewAgentRunService(store)
	runSvc.SetDecisionPolicy(func(context.Context, agentruntime.Decision) error { return nil })
	h.SetAgentRunService(runSvc)
	r := gin.New()
	r.GET("/sessions/:id/runs/:run_id", h.GetAgentRun)
	r.GET("/sessions/:id/runs/:run_id/events", h.GetAgentRunEvents)
	r.POST("/sessions/:id/runs/:run_id/decisions", h.PostAgentRunDecision)
	r.POST("/sessions/:id/runs/:run_id/cancel", h.CancelAgentRun)
	return r, store
}

func runTestCtx() context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	return types.WithPrincipal(ctx, types.Principal{Type: types.PrincipalWebUser, ID: "u1"})
}

func admitHandlerRun(t *testing.T, store *repository.AgentRunStore, runID string) agentruntime.RunKey {
	t.Helper()
	user, err := json.Marshal(map[string]any{"role": "user", "content": "q"})
	require.NoError(t, err)
	assistant, err := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	require.NoError(t, err)
	key := agentruntime.RunKey{TenantID: 1, RunID: runID}
	_, err = store.Admit(runTestCtx(), agentruntime.Admission{
		Key: key, SessionID: "s1", UserID: "web_user:u1", RequestID: "req-" + runID,
		AssistantMessageID: "am-" + runID, RequestHash: "h-" + runID,
		Snapshot:    json.RawMessage(`{"version":1,"query":"q","model_id":"m1"}`),
		UserMessage: user, AssistantMessage: assistant,
		Deadline: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	return key
}

// TestAgentRunEventsEndpointReplayAndCursor is the API reconnect row: events
// committed to the durable store replay in seq order after a disconnect, and
// a cursor older than retained history answers the explicit reload error.
func TestAgentRunEventsEndpointReplayAndCursor(t *testing.T) {
	db := openHandlerRunDB(t)
	r, store := newRunRouter(t, db)
	key := admitHandlerRun(t, store, "r1")
	fence, err := store.Claim(runTestCtx(), key, "w", time.Minute)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		payload, merr := json.Marshal(map[string]int{"n": i})
		require.NoError(t, merr)
		_, err = store.AppendEvent(runTestCtx(), fence, agentruntime.RunEvent{
			Type: "tool_dispatched", Payload: payload,
		})
		require.NoError(t, err)
	}

	// Reconnect after seq 1: the stream must deliver 2 and 3 in order and
	// never redeliver seq 1.
	req := httptest.NewRequest("GET", "/sessions/s1/runs/r1/events?after=1&limit=10&once=1",
		nil).WithContext(runTestCtx())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	body := w.Body.String()
	require.Equal(t, 200, w.Code)
	require.Contains(t, body, `seq":2`)
	require.Contains(t, body, `seq":3`)
	require.False(t, strings.Contains(body, `seq":1`),
		"replay after=1 must not redeliver seq 1: %s", body)

	// Simulate retention trimming of seq 1, then reconnect with a cursor that
	// now precedes the retained start: the endpoint answers the explicit
	// reload error instead of silently skipping events.
	require.NoError(t, db.Exec(
		"DELETE FROM agent_run_events WHERE tenant_id=1 AND run_id='r1' AND seq=1").Error)
	req = httptest.NewRequest("GET", "/sessions/s1/runs/r1/events?after=0&once=1", nil).WithContext(runTestCtx())
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, 409, w.Code)
	require.Contains(t, w.Body.String(), "cursor_expired")
}

// TestAgentRunDecisionConflictEnvelope pins the decision-conflict HTTP row:
// the same decision id with different content answers 409 with the error
// envelope, and the identical retry stays idempotent through the API.
func TestAgentRunDecisionConflictEnvelope(t *testing.T) {
	db := openHandlerRunDB(t)
	r, store := newRunRouter(t, db)
	_ = admitHandlerRun(t, store, "r2")
	require.NoError(t, db.Exec(
		"INSERT INTO agent_tool_calls (tenant_id,run_id,call_id,ca"+
			"ll_seq,tool_name,tool_identity,args_hash,args,status,unknown_reason)"+
			" VALUES (1,'r2','c1',1,'fetch','fetch','ah1','{}','unknown','p1')").Error)
	require.NoError(t, db.Exec(
		"UPDATE agent_runs SET status='waiting_user', wait_reason='p1', revision=7"+
			" WHERE tenant_id=1 AND run_id='r2'").Error)

	base := func(reason string) *http.Request {
		payload := fmt.Sprintf(`{"pending_id":"p1","decision_id":"d-1","tool_call_id":"c1",`+
			`"args_hash":"ah1","action":"retry","reason":"%s","expected_revision":7}`, reason)
		req := httptest.NewRequest("POST", "/sessions/s1/runs/r2/decisions", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		return req.WithContext(runTestCtx())
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, base("checked"))
	require.Equal(t, 200, w.Code, "first decision: %s", w.Body.String())

	w = httptest.NewRecorder()
	r.ServeHTTP(w, base("different reason"))
	require.Equal(t, 409, w.Code, "conflicting payload must 409: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "error")

	w = httptest.NewRecorder()
	r.ServeHTTP(w, base("checked"))
	require.Equal(t, 200, w.Code, "identical retry is idempotent")
}

// TestSessionDeleteRacesActiveRun pins the deletion row: deleting a session
// with a claimed run fences the worker out and the run is never reclaimed.
func TestSessionDeleteRacesActiveRun(t *testing.T) {
	db := openHandlerRunDB(t)
	_, store := newRunRouter(t, db)
	key := admitHandlerRun(t, store, "r3")
	_, err := store.Claim(runTestCtx(), key, "w", time.Minute)
	require.NoError(t, err)

	runs := service.NewAgentRunService(store)
	require.NoError(t, runs.DeleteSessionRuns(runTestCtx(), 1, "s1"))

	// Deletion fences the run terminally and removes its durable rows so no
	// worker can resurrect it; the store answers not-found afterwards.
	_, err = store.Get(runTestCtx(), key)
	require.ErrorIs(t, err, agentruntime.ErrNotFound, "a deleted run leaves no reclaimable row")
	var slot *string
	require.NoError(t, db.Raw(
		"SELECT active_agent_run_id FROM sessions WHERE id = 's1'").Scan(&slot).Error)
	require.Nil(t, slot, "deletion releases the session slot")
	_ = key
}
