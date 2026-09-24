package handler

// T09 (#39) 授权扩额 → 恢复同一 Run：extend 成功后，同一预算根下停靠的
// budget_exhausted run 被翻回 queued（resumed_runs 如实计数）；幂等重放同键
// 不再加额但同样执行 requeue（治愈竞态）；collaborator 403 不触发任何唤醒
// （Review Focus #1：角色门禁先于幂等与 requeue）。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newBudgetResumeDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, owner_id TEXT NOT NULL,
		session_id TEXT, status TEXT NOT NULL DEFAULT 'queued',
		wait_reason TEXT NOT NULL DEFAULT '', driver TEXT NOT NULL DEFAULT 'platform',
		revision INTEGER NOT NULL DEFAULT 0, lease_owner TEXT NOT NULL DEFAULT '',
		lease_until DATETIME, epoch INTEGER NOT NULL DEFAULT 0,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE sessions (id TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL, user_id TEXT)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE commercial_budget_accounts (
		tenant_id INTEGER PRIMARY KEY, verified_micro BIGINT NOT NULL, unreflected_micro BIGINT NOT NULL DEFAULT 0,
		held_micro BIGINT NOT NULL DEFAULT 0, refund_locked_micro BIGINT NOT NULL DEFAULT 0,
		watermark TEXT NOT NULL DEFAULT 'w0', verified_until DATETIME NOT NULL, version BIGINT NOT NULL DEFAULT 0)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE commercial_task_budgets (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, root_run_id TEXT NOT NULL DEFAULT '',
		limit_micro BIGINT NOT NULL DEFAULT 0, spent_micro BIGINT NOT NULL DEFAULT 0,
		held_micro BIGINT NOT NULL DEFAULT 0, deadline DATETIME NOT NULL,
		version BIGINT NOT NULL DEFAULT 1, PRIMARY KEY (tenant_id, run_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE commercial_task_budget_extensions (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, key TEXT NOT NULL,
		extra_micro BIGINT NOT NULL, applied_at DATETIME NOT NULL,
		PRIMARY KEY (tenant_id, run_id, key))`).Error)
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, user_id) VALUES ('s1', 7, 'u1')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, owner_id, session_id, status, wait_reason)
		VALUES (7, 'r1', 'u1', 's1', 'waiting_user', 'budget_exhausted')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, owner_id, session_id, status, wait_reason)
		VALUES (7, 'c1', 'u1', 's1', 'waiting_user', 'budget_exhausted')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO commercial_budget_accounts
		(tenant_id, verified_micro, verified_until, version) VALUES (7, 100000000, ?, 0)`,
		time.Now().Add(time.Hour).UTC()).Error)
	require.NoError(t, db.Exec(`INSERT INTO commercial_task_budgets
		(tenant_id, run_id, limit_micro, deadline, version) VALUES (7, 'r1', 1000, ?, 1)`,
		time.Now().Add(time.Hour).UTC()).Error)
	require.NoError(t, db.Exec(`INSERT INTO commercial_task_budgets
		(tenant_id, run_id, root_run_id, deadline) VALUES (7, 'c1', 'r1', ?)`,
		time.Now().Add(time.Hour).UTC()).Error)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func postBudgetExtendResume(t *testing.T, db *gorm.DB, role, userID, runID, key string) (int, map[string]any, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewCommercialHandler(db)
	r := gin.New()
	r.POST("/commercial/tasks/:id/budget/extend", h.ExtendTaskBudget)
	body := `{"additional_credits": 10, "idempotency_key": "` + key + `"}`
	req := httptest.NewRequest(http.MethodPost, "/commercial/tasks/"+runID+"/budget/extend", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, uint64(7))
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	if role != "" {
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRole(role))
	}
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var data map[string]any
	if w.Code == http.StatusOK {
		var envelope map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
		// appOK 信封 {"success":true,"data":{...}}：断言指向内层 data
		// （与 commercial_task_budget_read_test.go 的既有 helper 同一约定）。
		if inner, ok := envelope["data"].(map[string]any); ok {
			data = inner
		}
	}
	return w.Code, data, w.Body.String()
}

func runStatusOf(t *testing.T, db *gorm.DB, runID string) string {
	t.Helper()
	var status string
	require.NoError(t, db.Table("agent_runs").Select("status").
		Where("tenant_id = ? AND run_id = ?", 7, runID).Scan(&status).Error)
	return status
}

func TestExtendTaskBudgetResumesParkedRunsEndToEnd(t *testing.T) {
	db := newBudgetResumeDB(t)

	// Collaborator（非 Owner、无 grant 的普通成员）重放 owner 尚未应用的键：403，
	// 且不唤醒任何 run（角色门禁在幂等与 requeue 之前）。
	code, _, body := postBudgetExtendResume(t, db, "contributor", "u2", "r1", "k-e2e-1")
	require.Equal(t, http.StatusForbidden, code, "collaborator refused: %s", body)
	require.Equal(t, "waiting_user", runStatusOf(t, db, "r1"))
	require.Equal(t, "waiting_user", runStatusOf(t, db, "c1"))

	// 任务 Owner 扩额：200 + resumed_runs=2（根 + 委派子），两行翻回 queued。
	code, data, body := postBudgetExtendResume(t, db, "contributor", "u1", "r1", "k-e2e-1")
	require.Equal(t, http.StatusOK, code, "owner extends and resumes: %s", body)
	require.EqualValues(t, 2, data["resumed_runs"])
	require.Equal(t, "queued", runStatusOf(t, db, "r1"))
	require.Equal(t, "queued", runStatusOf(t, db, "c1"))
	var limit int64
	require.NoError(t, db.Table("commercial_task_budgets").Select("limit_micro").
		Where("tenant_id = ? AND run_id = ?", 7, "r1").Scan(&limit).Error)
	require.EqualValues(t, 1010, limit, "limit raised exactly once")

	// 幂等重放同键：200、limit 不再加倍（exactly-once），requeue 谓词不再匹配
	// （已 queued）→ resumed_runs=0。
	code, data, body = postBudgetExtendResume(t, db, "contributor", "u1", "r1", "k-e2e-1")
	require.Equal(t, http.StatusOK, code)
	require.EqualValues(t, 0, data["resumed_runs"])
	require.NoError(t, db.Table("commercial_task_budgets").Select("limit_micro").
		Where("tenant_id = ? AND run_id = ?", 7, "r1").Scan(&limit).Error)
	require.EqualValues(t, 1010, limit, "idempotent replay never raises twice")

	// 竞态治愈：若 run 在重放前又被停靠（模拟 requeue 失败后的重新 park），
	// 同键重放把它翻回 queued 而不加额。
	require.NoError(t, db.Exec(`UPDATE agent_runs SET status='waiting_user', wait_reason='budget_exhausted'
		WHERE tenant_id=7 AND run_id='r1'`).Error)
	code, data, body = postBudgetExtendResume(t, db, "contributor", "u1", "r1", "k-e2e-1")
	require.Equal(t, http.StatusOK, code)
	require.EqualValues(t, 1, data["resumed_runs"], "replay heals a stranded park without re-raising")
	require.EqualValues(t, 1010, limitAfter(t, db))
}

func limitAfter(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var limit int64
	require.NoError(t, db.Table("commercial_task_budgets").Select("limit_micro").
		Where("tenant_id = ? AND run_id = ?", 7, "r1").Scan(&limit).Error)
	return limit
}
