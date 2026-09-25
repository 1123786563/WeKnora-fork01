package handler

// T09 (#39) 预算读端点：四数字来自根预算行（委派子 Run 预占天然计入根行），
// 读门 = 账单权威 OR 任务 Owner（sessions.user_id，ADR-0004）OR #42 task grant
// 持有者（active 成员）；can_extend 只对账单权威/Owner 为 true——grant 协作者可读
// 不可扩（AC1 在 UI 面的如实投影）。未知/跨租户 run 对任何调用者都是 404，不向
// 无权者泄漏 run 存在性（Review Focus #5）。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newBudgetReadDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newBudgetGateDB(t) // sessions/commercial_budget_accounts/commercial_task_budgets 已备（commercial_task_budget_test.go:24-54）
	// 重建 agent_runs：读端点需要 status/wait_reason 列（newBudgetGateDB 的最小表没有）。
	require.NoError(t, db.Exec(`DROP TABLE agent_runs`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, owner_id TEXT NOT NULL,
		session_id TEXT, status TEXT NOT NULL DEFAULT 'queued',
		wait_reason TEXT NOT NULL DEFAULT '', driver TEXT NOT NULL DEFAULT 'platform',
		revision INTEGER NOT NULL DEFAULT 0, lease_owner TEXT NOT NULL DEFAULT '',
		lease_until DATETIME, epoch INTEGER NOT NULL DEFAULT 0)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, owner_id, session_id)
		VALUES (7, 'r1', 'u1', 's1')`).Error)
	// newBudgetGateDB 的 commercial_task_budgets 没有 root_run_id 列（父/子映射），
	// 追加后再种委派子行。
	require.NoError(t, db.Exec(`ALTER TABLE commercial_task_budgets ADD COLUMN root_run_id TEXT NOT NULL DEFAULT ''`).Error)
	// 追加读门与投影需要的表：#42 grants + 成员表 + 扩展记录（extend 落 TaskBudgetExtensionRow）。
	require.NoError(t, db.Exec(`CREATE TABLE task_grants (
		tenant_id INTEGER NOT NULL, task_id TEXT NOT NULL, grantee_id TEXT NOT NULL,
		role TEXT NOT NULL, granted_by TEXT NOT NULL DEFAULT '', granted_at DATETIME,
		PRIMARY KEY (tenant_id, task_id, grantee_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE tenant_members (
		tenant_id INTEGER NOT NULL, user_id TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active',
		deleted_at DATETIME, PRIMARY KEY (tenant_id, user_id))`).Error)
	require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id, user_id, status) VALUES
		(7, 'u1', 'active'), (7, 'u2', 'active'), (7, 'u3', 'active')`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE commercial_task_budget_extensions (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, key TEXT NOT NULL,
		extra_micro BIGINT NOT NULL, applied_at DATETIME NOT NULL,
		PRIMARY KEY (tenant_id, run_id, key))`).Error)
	// 根行 r1 带真实四数字；委派子 c1 挂根（零 limit 映射行）；r1 因达限停靠。
	require.NoError(t, db.Exec(`UPDATE commercial_task_budgets
		SET limit_micro = 1000, spent_micro = 400, held_micro = 100 WHERE tenant_id = 7 AND run_id = 'r1'`).Error)
	// 委派子行：零 limit 的映射行，仅指向根（budget_task.go AttachChildRun 语义）。
	require.NoError(t, db.Exec(`INSERT INTO commercial_task_budgets
		(tenant_id, run_id, root_run_id, limit_micro, deadline) VALUES (7, 'c1', 'r1', 0, ?)`,
		time.Now().Add(time.Hour).UTC()).Error)
	require.NoError(t, db.Exec(`UPDATE agent_runs SET status = 'waiting_user', wait_reason = 'budget_exhausted'
		WHERE tenant_id = 7 AND run_id = 'r1'`).Error)
	return db
}

func getTaskBudget(t *testing.T, db *gorm.DB, role, userID, runID string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewCommercialHandler(db)
	r := gin.New()
	r.GET("/commercial/tasks/:id/budget", h.GetTaskBudget)
	req := httptest.NewRequest(http.MethodGet, "/commercial/tasks/"+runID+"/budget", nil)
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, uint64(7))
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	if role != "" {
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRole(role))
	}
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var body map[string]any
	if w.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		// appOK 信封 {"success":true,"data":{...}}：断言指向内层 data。
		if inner, ok := body["data"].(map[string]any); ok {
			body = inner
		}
	}
	return w, body
}

func TestGetTaskBudgetReadGateArms(t *testing.T) {
	db := newBudgetReadDB(t)

	// 任务 Owner（普通 contributor，非空间 owner）：可读，can_extend=true，
	// 四数字来自根行（1000/400/100/500），委派与停靠清单如实。
	w, data := getTaskBudget(t, db, "contributor", "u1", "r1")
	require.Equal(t, http.StatusOK, w.Code, "task owner may read: %s", w.Body.String())
	require.Equal(t, "r1", data["task_id"])
	require.Equal(t, "r1", data["root_run_id"])
	require.EqualValues(t, 1000, data["limit_credits"])
	require.EqualValues(t, 400, data["used_credits"])
	require.EqualValues(t, 100, data["held_credits"])
	require.EqualValues(t, 500, data["remaining_credits"])
	require.Equal(t, true, data["can_extend"])
	require.Equal(t, []any{"c1"}, data["delegated_run_ids"])
	require.Equal(t, []any{"r1"}, data["paused_run_ids"])

	// grant 协作者（#42 真实 grant 行，active 成员）：可读，但 can_extend=false。
	require.NoError(t, db.Exec(`INSERT INTO task_grants (tenant_id, task_id, grantee_id, role)
		VALUES (7, 's1', 'u2', 'collaborator')`).Error)
	w2, data2 := getTaskBudget(t, db, "contributor", "u2", "r1")
	require.Equal(t, http.StatusOK, w2.Code, "granted collaborator may READ the budget")
	require.Equal(t, false, data2["can_extend"], "a grant never carries budget authority (AC1)")

	// 账单权威（空间 owner 角色）：可读，can_extend=true。
	w3, data3 := getTaskBudget(t, db, "owner", "boss", "r1")
	require.Equal(t, http.StatusOK, w3.Code)
	require.Equal(t, true, data3["can_extend"])

	// 无 grant 的普通成员（bystander）：403，不泄漏数字。
	w4, _ := getTaskBudget(t, db, "contributor", "u3", "r1")
	require.Equal(t, http.StatusForbidden, w4.Code)
	require.Contains(t, w4.Body.String(), "BUDGET_FORBIDDEN")

	// 未知 run 对任何人都 404（与写门同口径，不泄漏存在性）。
	w5, _ := getTaskBudget(t, db, "contributor", "u1", "missing")
	require.Equal(t, http.StatusNotFound, w5.Code)
	require.Contains(t, w5.Body.String(), "TASK_BUDGET_NOT_FOUND")
	// 跨租户 run 同样 404。
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, owner_id) VALUES (8, 'r8', 'u1')`).Error)
	w6, _ := getTaskBudget(t, db, "contributor", "u1", "r8")
	require.Equal(t, http.StatusNotFound, w6.Code)

	// 以委派子 Run 寻址：返回根行数字 + root_run_id（cumulative budget）。
	w7, data7 := getTaskBudget(t, db, "contributor", "u1", "c1")
	require.Equal(t, http.StatusOK, w7.Code)
	require.Equal(t, "r1", data7["root_run_id"])
	require.EqualValues(t, 1000, data7["limit_credits"])
	require.EqualValues(t, 500, data7["remaining_credits"])
}
