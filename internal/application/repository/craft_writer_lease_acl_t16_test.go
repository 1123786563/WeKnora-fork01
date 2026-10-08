package repository

// T16 (#134) OCR fixes: the exported lease seams carry the workspace ACL on
// EVERY path. GetWriterLease refuses cross-session and non-owner callers
// before projecting any lease fact (the hit path too, not just the empty
// answer), and ReleaseWriterLease locks the workspace row first — a
// same-session non-owner can never release another writer's fence, and
// Release serializes with Acquire under the same workspace→lease lock
// order. Every refusal leaves the fence intact.

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
)

func TestCraftT16LeaseSeamsCarryWorkspaceACLEverywhere(t *testing.T) {
	db := openCraftDB(t)
	store, ok := NewCraftStore(db).(*CraftStore)
	require.True(t, ok, "the lease methods live on the concrete store")
	ws := putCraftWorkspace(t, store)
	owner := craftTestScope()
	ctx := context.Background()

	// A live lease exists for the owner's run.
	acq, err := store.AcquireWriterLease(ctx, owner, ws.ID, "run-acl-1")
	require.NoError(t, err)
	require.Equal(t, craft.WriterAcquired, acq.Outcome.Status)

	sameSessionNonOwner := owner
	sameSessionNonOwner.UserID = "u-not-owner"
	crossSession := owner
	crossSession.SessionID = "s-other"

	// GetWriterLease: the HIT path refuses before projecting facts.
	_, err = store.GetWriterLease(ctx, sameSessionNonOwner, ws.ID)
	require.ErrorIs(t, err, craft.ErrForbidden, "a same-session non-owner never reads the lease")
	_, err = store.GetWriterLease(ctx, crossSession, ws.ID)
	require.ErrorIs(t, err, craft.ErrNotFound, "a foreign session cannot even see the workspace")
	lease, err := store.GetWriterLease(ctx, owner, ws.ID)
	require.NoError(t, err)
	require.Equal(t, "run-acl-1", lease.RunID)
	require.Equal(t, ws.SessionID, lease.TaskID, "the lease names the owning task")

	// The ACL also guards the EMPTY answer (no lease row at all).
	require.NoError(t, db.Where("workspace_id = ?", ws.ID).Delete(&craftWriterLeaseRow{}).Error)
	_, err = store.GetWriterLease(ctx, sameSessionNonOwner, ws.ID)
	require.ErrorIs(t, err, craft.ErrForbidden, "the empty answer is ACL-guarded too")
	_, err = store.GetWriterLease(ctx, owner, ws.ID)
	require.ErrorIs(t, err, craft.ErrNotFound, "the owner's empty answer stays an honest no-lease")

	// ReleaseWriterLease: the workspace lock precedes everything — a
	// same-session non-owner is refused before any run-fact observation,
	// and the fence survives every refusal.
	acq, err = store.AcquireWriterLease(ctx, owner, ws.ID, "run-acl-2")
	require.NoError(t, err)
	require.Equal(t, craft.WriterAcquired, acq.Outcome.Status)
	err = store.ReleaseWriterLease(ctx, sameSessionNonOwner, ws.ID, "run-acl-2", craft.WriterReleaseVerifiedCompletion)
	require.ErrorIs(t, err, craft.ErrForbidden, "a same-session non-owner never releases another writer's fence")
	err = store.ReleaseWriterLease(ctx, crossSession, ws.ID, "run-acl-2", craft.WriterReleaseVerifiedCompletion)
	// Cross-session callers do not exist on this seam ([T08] 404 discipline,
	// aligned with GetWriterLease above): NotFound refuses the release AND
	// hides the binding session. The refusal itself is unchanged — the fence
	// assertion below still proves the release never happened.
	require.ErrorIs(t, err, craft.ErrNotFound, "a foreign session never releases")
	lease, err = store.GetWriterLease(ctx, owner, ws.ID)
	require.NoError(t, err)
	require.Equal(t, "run-acl-2", lease.RunID, "every refusal leaves the fence intact")

	// The owner-facing no-lease answer on release keeps its honest
	// not-found after the workspace ACL passes.
	require.NoError(t, db.Where("workspace_id = ?", ws.ID).Delete(&craftWriterLeaseRow{}).Error)
	err = store.ReleaseWriterLease(ctx, owner, ws.ID, "run-acl-2", craft.WriterReleaseVerifiedCompletion)
	require.ErrorIs(t, err, craft.ErrNotFound, "releasing a fence that does not exist is an honest not-found")
}
