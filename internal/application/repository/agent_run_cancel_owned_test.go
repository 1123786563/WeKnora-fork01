package repository

import (
	"context"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 与 steerRunRow 先例同法：本测试只证明 CancelOwnedRun 的单事务语义，
// 直接 AutoMigrate 三张最小行集（全量迁移链由 Task 4 的 HTTP 集成测试覆盖）。
type cancelOwnedRunRow struct {
	TenantID                       uint64
	RunID, SessionID, OwnerID      string
	Status, WaitReason, LeaseOwner string
	LeaseUntil                     *time.Time
	Revision                       int64
	UpdatedAt                      time.Time
}

func (cancelOwnedRunRow) TableName() string { return "agent_runs" }

type cancelOwnedSessionRow struct {
	TenantID         uint64
	ID               string  `gorm:"column:id"`
	ActiveAgentRunID *string `gorm:"column:active_agent_run_id"`
}

func (cancelOwnedSessionRow) TableName() string { return "sessions" }

type cancelOwnedEventRow struct {
	TenantID                      uint64
	RunID                         string
	Seq                           int64
	AttemptID, EventType, Payload string
}

func (cancelOwnedEventRow) TableName() string { return "agent_run_events" }

func openCancelOwnedDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:cancel_owned_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&cancelOwnedRunRow{}, &cancelOwnedSessionRow{}, &cancelOwnedEventRow{}))
	require.NoError(t, db.Exec(`INSERT INTO sessions (tenant_id, id, active_agent_run_id) VALUES (1, 's1', NULL)`).Error)
	require.NoError(t, db.Exec(`UPDATE sessions SET active_agent_run_id = 'run-1' WHERE id = 's1'`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, status, wait_reason, revision, lease_owner, lease_until, updated_at) VALUES (1, 'run-1', 's1', 'u1', 'running', '', 0, '', NULL, CURRENT_TIMESTAMP)`).Error)
	return db
}

func TestCancelOwnedRunWritesEventReleasesSlotAndFencesRevision(t *testing.T) {
	db := openCancelOwnedDB(t)
	runs := NewAgentRunStore(db)
	ctx := context.Background()

	require.NoError(t, runs.CancelOwnedRun(ctx, 1, "u1", "run-1", 0, "user_requested"))

	var row struct {
		Status, WaitReason string
		Revision           int64
	}
	require.NoError(t, db.Raw(`SELECT status, wait_reason, revision FROM agent_runs WHERE run_id = 'run-1'`).Scan(&row).Error)
	require.Equal(t, "canceled", row.Status)
	require.Equal(t, "user_requested", row.WaitReason)
	require.EqualValues(t, 1, row.Revision)

	var eventType string
	require.NoError(t, db.Raw(`SELECT event_type FROM agent_run_events WHERE run_id = 'run-1' ORDER BY seq DESC LIMIT 1`).Scan(&eventType).Error)
	require.Equal(t, "cancellation_requested", eventType)

	var slot *string
	require.NoError(t, db.Raw(`SELECT active_agent_run_id FROM sessions WHERE id = 's1'`).Scan(&slot).Error)
	require.Nil(t, slot, "the session slot must be released so a restart can be admitted")

	// 幂等：对已取消的 run 重复取消返回 nil，不再推进 revision。
	var revision int64
	require.NoError(t, runs.CancelOwnedRun(ctx, 1, "u1", "run-1", 1, "user_requested"))
	require.NoError(t, db.Raw(`SELECT revision FROM agent_runs WHERE run_id = 'run-1'`).Scan(&revision).Error)
	require.EqualValues(t, 1, revision)
}

func TestCancelOwnedRunStaleRevisionIsAConflict(t *testing.T) {
	db := openCancelOwnedDB(t)
	runs := NewAgentRunStore(db)
	err := runs.CancelOwnedRun(context.Background(), 1, "u1", "run-1", 7, "user_requested")
	require.ErrorIs(t, err, agentruntime.ErrConflict)
	var status string
	require.NoError(t, db.Raw(`SELECT status FROM agent_runs WHERE run_id = 'run-1'`).Scan(&status).Error)
	require.Equal(t, "running", status, "a fenced-out cancel must mutate nothing")
}

func TestCancelOwnedRunForeignOwnerIsUniformNotFound(t *testing.T) {
	db := openCancelOwnedDB(t)
	runs := NewAgentRunStore(db)
	err := runs.CancelOwnedRun(context.Background(), 1, "u2", "run-1", 0, "user_requested")
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
	var count int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM agent_run_events WHERE run_id = 'run-1'`).Scan(&count).Error)
	require.EqualValues(t, 0, count, "a foreign cancel must write no events")
	var slot string
	require.NoError(t, db.Raw(`SELECT COALESCE(active_agent_run_id, '') FROM sessions WHERE id = 's1'`).Scan(&slot).Error)
	require.Equal(t, "run-1", slot, "a foreign cancel must not release the slot")
}
