package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func archivedAt(t *testing.T, db *gorm.DB, sessionID string) *time.Time {
	t.Helper()
	var row struct {
		ArchivedAt *time.Time
	}
	require.NoError(t, db.Raw("SELECT archived_at FROM sessions WHERE id = ?", sessionID).Scan(&row).Error)
	return row.ArchivedAt
}

// TestWorkbenchTaskStateArchiveOwnershipAndIsolation: 归档谓词与读模型一致——
// 只有「该 tenant 内拥有该 session 至少一个 run」的 caller 才能归档/恢复；
// 同租户他人 session、异租户 session 一律 ErrWorkbenchTaskNotFound。
func TestWorkbenchTaskStateArchiveOwnershipAndIsolation(t *testing.T) {
	db := openRunTestDB(t)
	seedWorkbenchListFixtures(t, db)
	base := time.Date(2026, 9, 23, 9, 0, 0, 0, time.UTC)
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-a1", session: "s1", status: "running", agent: "agent-x", target: "platform", at: base})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u2", runID: "r-b1", session: "s3", status: "running", agent: "agent-x", target: "platform", at: base.Add(time.Second)})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 2, owner: "v1", runID: "r-c1", session: "t1", status: "running", agent: "agent-x", target: "platform", at: base.Add(2 * time.Second)})

	store := NewWorkbenchTaskStateStore(db)
	ctx := context.Background()
	now := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)

	require.NoError(t, store.SetTaskArchived(ctx, 1, "u1", "s1", true, now))
	at := archivedAt(t, db, "s1")
	require.NotNil(t, at)
	require.Equal(t, now.UTC(), at.UTC())

	// 同租户他人 session、异租户 session：归属谓词拒绝，不写任何行。
	require.ErrorIs(t, store.SetTaskArchived(ctx, 1, "u1", "s3", true, now), ErrWorkbenchTaskNotFound)
	require.ErrorIs(t, store.SetTaskArchived(ctx, 1, "u1", "t1", true, now), ErrWorkbenchTaskNotFound)
	require.ErrorIs(t, store.SetTaskArchived(ctx, 2, "v1", "s1", true, now), ErrWorkbenchTaskNotFound)
	require.Nil(t, archivedAt(t, db, "s3"))
	require.Nil(t, archivedAt(t, db, "t1"))

	// 恢复：archived_at 置 NULL，幂等可重复。
	require.NoError(t, store.SetTaskArchived(ctx, 1, "u1", "s1", false, now))
	require.Nil(t, archivedAt(t, db, "s1"))
	require.NoError(t, store.SetTaskArchived(ctx, 1, "u1", "s1", false, now))

	// 非法身份：零租户/空 owner/空 taskId 一律 ErrNotFound，零查询副作用。
	require.ErrorIs(t, store.SetTaskArchived(ctx, 0, "u1", "s1", true, now), agentruntime.ErrNotFound)
	require.ErrorIs(t, store.SetTaskArchived(ctx, 1, " ", "s1", true, now), agentruntime.ErrNotFound)
	require.ErrorIs(t, store.SetTaskArchived(ctx, 1, "u1", "  ", true, now), agentruntime.ErrNotFound)
	require.False(t, errors.Is(ErrWorkbenchTaskNotFound, agentruntime.ErrNotFound), "sentinel must stay distinct from ErrNotFound")
}

// TestWorkbenchTaskStateArchiveLegacySessionBySessionOwner：T14——从未有过
// run 的旧会话（Legacy Task）由 session 归属者归档/恢复（同身份生命周期）；
// 他人/异租户依旧 ErrWorkbenchTaskNotFound，不写任何行。
func TestWorkbenchTaskStateArchiveLegacySessionBySessionOwner(t *testing.T) {
	db := openRunTestDB(t)
	seedWorkbenchListFixtures(t, db)
	store := NewWorkbenchTaskStateStore(db)
	ctx := context.Background()
	now := time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC)

	// s2 属于 u1 且 0 run（seedRunFixtures 未给 s2 加 run）——Legacy Task。
	require.NoError(t, store.SetTaskArchived(ctx, 1, "u1", "s2", true, now))
	at := archivedAt(t, db, "s2")
	require.NotNil(t, at)
	require.Equal(t, now.UTC(), at.UTC())
	require.NoError(t, store.SetTaskArchived(ctx, 1, "u1", "s2", false, now))
	require.Nil(t, archivedAt(t, db, "s2"))

	// s3 属于 u2 且 0 run：他人不可归档；t1 属于异租户 v1：同样拒绝。
	require.ErrorIs(t, store.SetTaskArchived(ctx, 1, "u1", "s3", true, now), ErrWorkbenchTaskNotFound)
	require.ErrorIs(t, store.SetTaskArchived(ctx, 2, "v1", "s2", true, now), ErrWorkbenchTaskNotFound)
	require.Nil(t, archivedAt(t, db, "s3"))

	// 软删除的 session 不再是可归档的任务。
	require.NoError(t, db.Exec("UPDATE sessions SET deleted_at = ? WHERE id = 's2'", now).Error)
	require.ErrorIs(t, store.SetTaskArchived(ctx, 1, "u1", "s2", true, now), ErrWorkbenchTaskNotFound)
}
