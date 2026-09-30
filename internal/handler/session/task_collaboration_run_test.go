package session

// "Viewer 不能运行"的 wire 级回归（T12 #42 AC1）：运行入口（agent-chat）的
// owner-scope 谓词必须不因 task grant 放宽——viewer、collaborator、无 grant
// 成员与跨租户调用者的 POST /agent-chat 全部 404。真实 sqlite 迁移库 +
// 真实 SessionRepository 支撑的 SessionService stub + 真实 QA handler。

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// runGateSessions is a SessionService stub whose GetOwnedSession delegates to
// the REAL session repository — the production owner predicate (session.go
// GetOwnedSession → sessionRepo.Get, tenant+user scoped).
type runGateSessions struct {
	interfaces.SessionService
	repo interfaces.SessionRepository
}

func (s *runGateSessions) GetOwnedSession(ctx context.Context, id string) (*types.Session, error) {
	tenantID, _ := types.TenantIDFromContext(ctx)
	userID, _ := types.UserIDFromContext(ctx)
	return s.repo.Get(ctx, tenantID, userID, id)
}

func TestAgentQARunGateStaysOwnerScopedForGrantHolders(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "run-gate.db") + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })

	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (1, 't1', 'test')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s1', 1, 'task-1', 'u1', 'builtin')`).Error)
	// Seed REAL grant rows so u2/u3 are genuine viewer/collaborator holders:
	// the run gate must stay owner-scoped even for granted members (AC1).
	// The migrated track (full migrations/sqlite Up above) already created
	// task_grants.
	require.NoError(t, db.Exec(`INSERT INTO task_grants (tenant_id, task_id, grantee_id, role, granted_by) VALUES (1, 's1', 'u2', 'viewer', 'u1')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO task_grants (tenant_id, task_id, grantee_id, role, granted_by) VALUES (1, 's1', 'u3', 'collaborator', 'u1')`).Error)

	h := &Handler{sessionService: &runGateSessions{repo: repository.NewSessionRepository(db)}}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.POST("/agent-chat/:session_id", h.AgentQA)

	post := func(userID string, tenant uint64) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/agent-chat/s1", strings.NewReader(`{"query":"hi"}`))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), types.TenantIDContextKey, tenant)
		ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// Viewer grant holder (u2), collaborator grant holder (u3), grant-less
	// member (u4) and cross-tenant caller: the run surface stays owner-scoped
	// (404) — grants never widen the run channel.
	for _, userID := range []string{"u2", "u3", "u4"} {
		require.Equal(t, http.StatusNotFound, post(userID, 1).Code,
			"POST /agent-chat 对非 owner 一律 404（Viewer 不能运行，AC1）user=%s", userID)
	}
	require.Equal(t, http.StatusNotFound, post("u2", 2).Code, "跨租户 404")
}
