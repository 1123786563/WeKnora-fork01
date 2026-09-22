package repository_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/handler/session"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openWorkbenchHTTPDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "workbench-http.db") + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (1, 'tenant-1', 'test')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES ('u1', 'u1', 'u1@example.test', 'x', 1)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s1', 1, 'session-1', 'u1', 'trpc')`).Error)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func workbenchAdmission() agentruntime.Admission {
	return agentruntime.Admission{
		Key: agentruntime.RunKey{TenantID: 1, RunID: "r1"}, SessionID: "s1", UserID: "u1",
		RequestID: "q1", AssistantMessageID: "a1", RequestHash: "hash-1",
		Snapshot: json.RawMessage(`{"version":1}`), UserMessage: json.RawMessage(`{"role":"user","content":"hello"}`),
		AssistantMessage: json.RawMessage(`{"role":"assistant","content":""}`), Deadline: time.Now().Add(time.Hour),
	}
}

func TestWorkbenchHTTPOwnershipUsesRealStoreAndProjection(t *testing.T) {
	db := openWorkbenchHTTPDB(t)
	store := repository.NewAgentRunStore(db)
	_, err := store.Admit(context.Background(), workbenchAdmission())
	require.NoError(t, err)
	h := session.NewWorkbenchReadHandler(store, repository.NewAgentRunSnapshotRepository(db))
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/workbench/executions/:run_id/snapshot", h.GetWorkbenchSnapshot)

	request := func(tenant uint64, owner string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/workbench/executions/r1/snapshot", nil)
		ctx := context.WithValue(req.Context(), types.TenantIDContextKey, tenant)
		ctx = context.WithValue(ctx, types.UserIDContextKey, owner)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	require.Equal(t, http.StatusOK, request(1, "u1").Code)
	require.Equal(t, http.StatusNotFound, request(1, "u2").Code)
	require.Equal(t, http.StatusNotFound, request(2, "u1").Code)
}

func TestWorkbenchHTTPSourceEventToSnapshotAndSSEUsesRealRepository(t *testing.T) {
	db := openWorkbenchHTTPDB(t)
	store := repository.NewAgentRunStore(db)
	_, err := store.Admit(context.Background(), workbenchAdmission())
	require.NoError(t, err)
	require.NoError(t, db.Exec(`INSERT INTO execution_dispatches (tenant_id, command_id, run_id, attempt_id, payload_hash, state, worker, epoch) VALUES (1, 'binding-1', 'r1', 'a1', '', 'completed', 'w', 1)`).Error)
	h := session.NewWorkbenchReadHandler(store, repository.NewAgentRunSnapshotRepository(db), repository.NewExecutionObservationStore(db))
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/workbench/executions/:run_id/source-events", h.IngestWorkbenchSourceEvent)
	r.GET("/workbench/executions/:run_id/snapshot", h.GetWorkbenchSnapshot)
	r.GET("/workbench/executions/:run_id/events", h.StreamWorkbenchEvents)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		ctx := context.WithValue(req.Context(), types.TenantIDContextKey, uint64(1))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	body := `{"binding_id":"binding-1","generation":"g1","event_id":"e1","attempt_id":"a1","type":"text.delta","source_seq":1,"payload":{"text":"confirmed"}}`
	// source_seq is carried by the provider body and is persisted separately
	// from the product seq allocated by the repository.
	require.Equal(t, http.StatusAccepted, request(http.MethodPost, "/workbench/executions/r1/source-events", body).Code)
	require.NoError(t, db.Exec(`UPDATE agent_runs SET status = 'succeeded' WHERE tenant_id = 1 AND run_id = 'r1'`).Error)
	snapshot := request(http.MethodGet, "/workbench/executions/r1/snapshot", "")
	require.Equal(t, http.StatusOK, snapshot.Code)
	require.Contains(t, snapshot.Body.String(), "confirmed")
	events := request(http.MethodGet, "/workbench/executions/r1/events", "")
	require.Equal(t, http.StatusOK, events.Code)
	require.Contains(t, events.Body.String(), "text.delta")
}
