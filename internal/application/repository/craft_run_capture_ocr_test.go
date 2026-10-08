package repository

// Wrap-up OCR F15: ClaimForDrain is a CAS — losing it (0 rows matched)
// must answer a conflict, never a fabricated "capturing" success that lets
// the caller walk the tree twice (two uploads, one orphaned resource row).
import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
)

func TestCraftRunCaptureClaimForDrainLostAnswersConflict(t *testing.T) {
	ctx := context.Background()
	db := openRunTestDB(t)
	registerCraftSessionForCaptureTest(t, db)
	workspace := putCraftWorkspace(t, NewCraftStore(db))
	runs := NewAgentRunStore(db)
	in := agentRunAdmissionForCaptureTest("capture-claim-race", workspace.ID)
	_, err := runs.Admit(ctx, in)
	require.NoError(t, err)
	fence, err := runs.Claim(ctx, in.Key, "capture-claim-worker", time.Minute)
	require.NoError(t, err)
	// A capture receipt only exists for a TERMINAL run.
	require.NoError(t, runs.CancelRun(ctx, fence.RunKey, "terminal"))
	views := NewCraftRunViewStore(db)
	view, err := views.Allocate(ctx, craft.RunViewKey{TenantID: 1, OwnerID: "u1", SessionID: "s1", RunID: in.Key.RunID})
	require.NoError(t, err)
	view, _, err = views.BeginSessionCreate(ctx, craft.RunViewKey{TenantID: 1, OwnerID: "u1", SessionID: "s1", RunID: in.Key.RunID}, view.Generation)
	require.NoError(t, err)
	_, err = views.BindRuntime(ctx, craft.RunViewKey{TenantID: 1, OwnerID: "u1", SessionID: "s1", RunID: in.Key.RunID}, view.Generation, craft.RunViewRuntime{RuntimeID: "claim-runtime", ContainerID: "claim-container", OpenCodeSessionID: "claim-oc"})
	require.NoError(t, err)

	store := NewCraftRunCaptureStore(db)
	receipt, err := store.EnsurePending(ctx, craftTestScope(), workspace.ID, in.Key.RunID, view.Generation)
	require.NoError(t, err)

	// First claim wins.
	claimed, err := store.ClaimForDrain(ctx, receipt)
	require.NoError(t, err)
	require.Equal(t, "capturing", claimed.State)

	// A racing second claim matches 0 rows: it must NOT report success with
	// State=capturing — the double-walk guard depends on this answer.
	_, err = store.ClaimForDrain(ctx, receipt)
	require.ErrorIs(t, err, craft.ErrConflict, "a lost CAS answers a retryable conflict, never a fabricated claim")

	// A receipt that already moved past pending (sealed) also refuses.
	sealed := claimed
	sealed.State = "sealed"
	_, err = store.ClaimForDrain(ctx, sealed)
	require.ErrorIs(t, err, craft.ErrConflict)
}
