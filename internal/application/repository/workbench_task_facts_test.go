package repository

import (
	"context"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
)

// TestReadTaskFactsForRunDerivesThreeLayerFactsWithinOwnerScope：任务层事实
// （title/archived_at/attention）沿 owner+tenant+run 谓词读取；跨 owner、跨
// 租户与不存在的 run 一律 ErrNotFound，不泄露任何事实。
func TestReadTaskFactsForRunDerivesThreeLayerFactsWithinOwnerScope(t *testing.T) {
	db := openRunTestDB(t)
	seedWorkbenchListFixtures(t, db)
	base := time.Date(2026, 9, 23, 8, 0, 0, 0, time.UTC)
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-live", session: "s1", status: "running", agent: "agent-x", target: "platform", at: base})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-wait", session: "s2", status: "waiting_user", agent: "agent-y", target: "platform", at: base.Add(time.Second)})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-pend", session: "s1", status: "running", agent: "agent-z", target: "platform", at: base.Add(2 * time.Second)})
	archived := base.Add(time.Hour)
	require.NoError(t, db.Exec("UPDATE sessions SET archived_at = ? WHERE id = 's2'", archived).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO workbench_interactions (tenant_id, id, run_id, owner_id, kind, args_hash, status, expected_revision, created_at, updated_at)
		 VALUES (1, 'ix-t5', 'r-pend', 'u1', 'tool_approval', 'h-t5', 'pending', 1, ?, ?)`, archived, archived,
	).Error)
	store := NewWorkbenchListStore(db)
	ctx := context.Background()

	live, err := store.ReadTaskFactsForRun(ctx, 1, "u1", "r-live")
	require.NoError(t, err)
	require.Equal(t, "s1", live.TaskID)
	require.Equal(t, "session-1", live.Title)
	require.Equal(t, "none", live.Attention)
	require.Empty(t, live.ArchivedAt)

	waiting, err := store.ReadTaskFactsForRun(ctx, 1, "u1", "r-wait")
	require.NoError(t, err)
	require.Equal(t, "required", waiting.Attention, "waiting_user derives attention")
	require.Equal(t, archived.UTC().Format(time.RFC3339Nano), waiting.ArchivedAt)

	pending, err := store.ReadTaskFactsForRun(ctx, 1, "u1", "r-pend")
	require.NoError(t, err)
	require.Equal(t, "required", pending.Attention, "a pending interaction derives attention")

	_, err = store.ReadTaskFactsForRun(ctx, 1, "u2", "r-live")
	require.ErrorIs(t, err, agentruntime.ErrNotFound, "another owner in the same tenant sees nothing")
	_, err = store.ReadTaskFactsForRun(ctx, 2, "v1", "r-live")
	require.ErrorIs(t, err, agentruntime.ErrNotFound, "a tenant neighbour sees nothing")
	_, err = store.ReadTaskFactsForRun(ctx, 1, "u1", "r-none")
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
}
