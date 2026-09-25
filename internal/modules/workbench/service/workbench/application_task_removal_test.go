package workbench

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// TestRemoveCareerApplicationTaskProjectionsIsScopedAndIdempotent covers the
// T22 complete-deletion seam: it removes the mapping rows, agent runs, and
// sessions of the requested owner only, leaves other owners untouched, and a
// retry after removal succeeds with an empty set.
func TestRemoveCareerApplicationTaskProjectionsIsScopedAndIdempotent(t *testing.T) {
	db := openApplicationTaskDB(t)
	coordinator := NewApplicationTaskCoordinator(db)
	ctx := context.Background()

	ownerLink, err := coordinator.EnsureCareerApplicationTask(ctx, 701, "owner-1", applicationTaskIntent())
	require.NoError(t, err)
	otherIntent := interfaces.CareerApplicationTaskIntent{
		ApplicationID: "0f0a5e21-9b1c-4d3a-8e2f-112233445566",
		RequestID:     "other-owner-request",
		Title:         "Other Owner Application",
	}
	otherLink, err := coordinator.EnsureCareerApplicationTask(ctx, 702, "owner-2", otherIntent)
	require.NoError(t, err)

	removed, err := coordinator.RemoveCareerApplicationTaskProjections(ctx, 701, "owner-1")
	require.NoError(t, err)
	require.Len(t, removed, 1)
	require.Equal(t, ownerLink.TaskID, removed[0].TaskID)
	require.Equal(t, ownerLink.RunID, removed[0].RunID)
	require.Equal(t, applicationTaskIntent().ApplicationID, removed[0].ApplicationID)

	var ownerMappings, otherMappings int64
	require.NoError(t, db.Table("workbench_application_tasks").
		Where("tenant_id = ? AND owner_id = ?", uint64(701), "owner-1").Count(&ownerMappings).Error)
	require.NoError(t, db.Table("workbench_application_tasks").
		Where("tenant_id = ? AND owner_id = ?", uint64(702), "owner-2").Count(&otherMappings).Error)
	require.Zero(t, ownerMappings, "requested owner's projections must be gone")
	require.Equal(t, int64(1), otherMappings, "other owners keep their projections")

	var ownerSessions, ownerRuns int64
	require.NoError(t, db.Table("sessions").Where("id = ?", ownerLink.TaskID).Count(&ownerSessions).Error)
	require.NoError(t, db.Table("agent_runs").Where("session_id = ?", ownerLink.TaskID).Count(&ownerRuns).Error)
	require.Zero(t, ownerSessions)
	require.Zero(t, ownerRuns)
	var otherSessions int64
	require.NoError(t, db.Table("sessions").Where("id = ?", otherLink.TaskID).Count(&otherSessions).Error)
	require.Equal(t, int64(1), otherSessions)

	// Idempotent retry: an already-clean scope succeeds with no rows.
	again, err := coordinator.RemoveCareerApplicationTaskProjections(ctx, 701, "owner-1")
	require.NoError(t, err)
	require.Empty(t, again)
}
