package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/execution"
	"github.com/stretchr/testify/require"
)

func TestExecutionCleanupTombstoneClaimAndSettlement(t *testing.T) {
	db := openRunTestDB(t)
	s := NewAgentRunStore(db)
	ctx := context.Background()
	require.NoError(t, s.TombstoneSession(ctx, 1, "u1", "s1"))
	require.ErrorIs(t, s.TombstoneSession(ctx, 1, "other", "s1"), runtime.ErrNotFound)
	claim, err := s.ClaimCleanup(ctx, "cleanup-worker", time.Minute)
	require.NoError(t, err)
	require.Equal(t, uint64(1), claim.TenantID)
	require.Equal(t, "s1", claim.SessionID)
	require.Equal(t, "u1", claim.OwnerID)
	require.Positive(t, claim.DeletionRevision)
	require.NoError(t, s.CompleteCleanup(ctx, claim, execution.CleanupFacts{Stopped: true, Settled: false, RetentionElapsed: true}))
	var state string
	require.NoError(t, db.Table("execution_cleanup").Where("tenant_id=? AND session_id=?", 1, "s1").Pluck("state", &state).Error)
	require.Equal(t, "cleanup_pending", state)
	claim, err = s.ClaimCleanup(ctx, "cleanup-worker-2", time.Minute)
	require.NoError(t, err)
	require.NoError(t, s.CompleteCleanup(ctx, claim, execution.CleanupFacts{Stopped: true, Settled: true, RetentionElapsed: true}))
	require.NoError(t, db.Table("execution_cleanup").Where("tenant_id=? AND session_id=?", 1, "s1").Pluck("state", &state).Error)
	require.Equal(t, "purged", state)
}

func TestExecutionCleanupFactsAreMonotonicAndRevisionFenced(t *testing.T) {
	db := openRunTestDB(t)
	s := NewAgentRunStore(db)
	ctx := context.Background()
	require.NoError(t, s.TombstoneSession(ctx, 1, "u1", "s1"))
	oldClaim, err := s.ClaimCleanup(ctx, "cleanup-worker", time.Minute)
	require.NoError(t, err)
	require.NoError(t, s.CompleteCleanup(ctx, oldClaim, execution.CleanupFacts{Stopped: true}))
	claim, err := s.ClaimCleanup(ctx, "cleanup-worker-2", time.Minute)
	require.NoError(t, err)
	require.NoError(t, s.TombstoneSession(ctx, 1, "u1", "s1"))
	require.ErrorIs(t, s.CompleteCleanup(ctx, claim, execution.CleanupFacts{Settled: true, RetentionElapsed: true}), runtime.ErrLeaseLost)
	newClaim, err := s.ClaimCleanup(ctx, "cleanup-worker-3", time.Minute)
	require.NoError(t, err)
	require.Greater(t, newClaim.DeletionRevision, claim.DeletionRevision)
	require.NoError(t, s.CompleteCleanup(ctx, newClaim, execution.CleanupFacts{Stopped: true, Settled: false, RetentionElapsed: false}))
	var stopped, settled, retained bool
	require.NoError(t, db.Table("execution_cleanup").Where("tenant_id=? AND session_id=?", 1, "s1").Select("stopped, settled, retention_elapsed").Row().Scan(&stopped, &settled, &retained))
	require.True(t, stopped)
	require.False(t, settled)
	require.False(t, retained)
}
