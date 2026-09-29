package repository

// T09 (#39) RequeueBudgetPausedRuns：授权扩额后把同一任务预算（根行 + 全部委派
// 子 Run）下停靠的 budget_exhausted run 翻回 claimable 的 queued。只有同根、同
// wait 理由的行翻转；异根/异因/非停靠行一律不动（Review Focus #3 跨任务误唤醒）。

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openRequeueTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL,
		session_id TEXT NOT NULL DEFAULT '', owner_id TEXT NOT NULL DEFAULT '',
		request_id TEXT NOT NULL DEFAULT '', assistant_message_id TEXT NOT NULL DEFAULT '',
		request_hash TEXT NOT NULL DEFAULT '', engine_type TEXT NOT NULL DEFAULT '',
		driver TEXT NOT NULL DEFAULT 'platform', target_id TEXT NOT NULL DEFAULT '',
		budget_ref TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'queued',
		wait_reason TEXT NOT NULL DEFAULT '', snapshot TEXT NOT NULL DEFAULT '{}',
		graph_version TEXT NOT NULL DEFAULT '', sdk_version TEXT NOT NULL DEFAULT '',
		schema_version INTEGER NOT NULL DEFAULT 0, lease_owner TEXT NOT NULL DEFAULT '',
		lease_until DATETIME, epoch INTEGER NOT NULL DEFAULT 0, revision INTEGER NOT NULL DEFAULT 0,
		max_rounds INTEGER NOT NULL DEFAULT 0, max_tool_calls INTEGER NOT NULL DEFAULT 0,
		token_budget INTEGER NOT NULL DEFAULT 0, deadline DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (tenant_id, run_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE commercial_task_budgets (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, root_run_id TEXT NOT NULL DEFAULT '',
		limit_micro BIGINT NOT NULL DEFAULT 0, spent_micro BIGINT NOT NULL DEFAULT 0,
		held_micro BIGINT NOT NULL DEFAULT 0, deadline DATETIME NOT NULL,
		version BIGINT NOT NULL DEFAULT 1, PRIMARY KEY (tenant_id, run_id))`).Error)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func seedParkedRun(t *testing.T, db *gorm.DB, runID, status, waitReason string) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO agent_runs
		(tenant_id, run_id, session_id, owner_id, driver, status, wait_reason, revision, deadline)
		VALUES (7, ?, 's1', 'u1', 'platform', ?, ?, 3, ?)`,
		runID, status, waitReason, time.Now().UTC().Add(time.Hour)).Error)
}

func TestRequeueBudgetPausedRunsResumesOnlySameBudgetRoot(t *testing.T) {
	db := openRequeueTestDB(t)
	store := NewAgentRunStore(db)
	// 预算根 r1：自身 + 委派子 c1 均因达限停靠 → 两者都恢复。
	seedParkedRun(t, db, "r1", "waiting_user", "budget_exhausted")
	seedParkedRun(t, db, "c1", "waiting_user", "budget_exhausted")
	require.NoError(t, db.Exec(`INSERT INTO commercial_task_budgets
		(tenant_id, run_id, root_run_id, deadline) VALUES (7, 'r1', '', ?)`,
		time.Now().UTC().Add(time.Hour)).Error)
	require.NoError(t, db.Exec(`INSERT INTO commercial_task_budgets
		(tenant_id, run_id, root_run_id, deadline) VALUES (7, 'c1', 'r1', ?)`,
		time.Now().UTC().Add(time.Hour)).Error)
	// 异预算根 other-root 的停靠 run：不得唤醒。
	seedParkedRun(t, db, "x1", "waiting_user", "budget_exhausted")
	require.NoError(t, db.Exec(`INSERT INTO commercial_task_budgets
		(tenant_id, run_id, root_run_id, deadline) VALUES (7, 'x1', 'other-root', ?)`,
		time.Now().UTC().Add(time.Hour)).Error)
	// 同根但异因停靠（工具结果未知）：不得唤醒。
	seedParkedRun(t, db, "w1", "waiting_user", "tool_outcome_unknown")
	// 非停靠态：不得触碰。
	seedParkedRun(t, db, "run-running", "running", "")

	n, err := store.RequeueBudgetPausedRuns(context.Background(), 7, "r1")
	require.NoError(t, err)
	require.EqualValues(t, 2, n, "only the root and its delegated child resume")

	for _, runID := range []string{"r1", "c1"} {
		var row struct {
			Status, WaitReason, LeaseOwner string
			Revision                       int64
		}
		require.NoError(t, db.Table("agent_runs").Select("status, wait_reason, lease_owner, revision").
			Where("tenant_id = ? AND run_id = ?", 7, runID).Take(&row).Error)
		require.Equal(t, "queued", row.Status, runID)
		require.Equal(t, "", row.WaitReason, runID)
		require.Equal(t, "", row.LeaseOwner, runID)
		require.EqualValues(t, 4, row.Revision, "revision must advance exactly once (%s)", runID)
	}
	for _, runID := range []string{"x1", "w1"} {
		var row struct{ Status, WaitReason string }
		require.NoError(t, db.Table("agent_runs").Select("status, wait_reason").
			Where("tenant_id = ? AND run_id = ?", 7, runID).Take(&row).Error)
		require.Equal(t, "waiting_user", row.Status, "unrelated parked run %s must stay parked", runID)
		require.NotEqual(t, "", row.WaitReason)
	}
	var running struct{ Status string }
	require.NoError(t, db.Table("agent_runs").Select("status").
		Where("tenant_id = ? AND run_id = ?", 7, "run-running").Take(&running).Error)
	require.Equal(t, "running", running.Status)

	// 幂等：再次调用无行可翻（已 queued 不再匹配谓词）。
	n2, err := store.RequeueBudgetPausedRuns(context.Background(), 7, "r1")
	require.NoError(t, err)
	require.EqualValues(t, 0, n2)

	// 输入护栏：空 root / 零租户拒绝。
	_, err = store.RequeueBudgetPausedRuns(context.Background(), 0, "r1")
	require.Error(t, err)
	_, err = store.RequeueBudgetPausedRuns(context.Background(), 7, "")
	require.Error(t, err)
}
