package repository_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/handler/session"
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
