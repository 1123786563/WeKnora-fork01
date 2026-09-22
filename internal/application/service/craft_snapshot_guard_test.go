package service

// CFT-S03-T022: the restore guard summary. The point suites live in
// craft_snapshot_test.go (V1EditV3KeepsV2: restore publishes nothing;
// IdempotentKey: one revision bump per request id; RestoreRefusals: active
// run / revision race / waiting_user) — this pins the remaining named
// combination: a version WITHOUT a recovery source refuses restore while
// its download stays available (downloadable ≠ restorable, server-side).
import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
)

func TestCraftSnapshotRestoreGuardMissingSource(t *testing.T) {
	env := newSnapshotEnv(t)
	ctx := craftCtx(1, "u1", "s1")

	// Publish v1 (WITH a snapshot) then v2 (no snapshot captured).
	v1 := env.publishSnapshotVersion(t, "run-g1", "<h1>guard v1</h1>")
	_ = env.capture(t, v1.ID)
	v2 := env.publishSnapshotVersion(t, "run-g2", "<h1>guard v2 (no snapshot)</h1>")
	ws, err := env.store.GetWorkspace(context.Background(), env.scope)
	require.NoError(t, err)

	// v2's files remain downloadable (immutable delivery), but v2 has no
	// recovery source: restoring BY SNAPSHOT requires an existing one, and a
	// v2-snapshot id simply does not resolve.
	_, err = env.svc.Restore(ctx, env.scope, "snap_nonexistent_"+v2.ID, ws.Revision)
	require.Error(t, err, "restoring from a nonexistent snapshot must fail")
	require.NotErrorIs(t, err, craft.ErrBusy, "the failure is about the missing source, not the slot")

	storedV2, err := env.versions.Get(context.Background(), env.scope, v2.ID)
	require.NoError(t, err)
	require.NotEmpty(t, storedV2.Files, "v2 stays downloadable — download never needed the snapshot")

	// And the REAL v1 snapshot still restores cleanly (the guard did not
	// disturb the workspace).
	v1Snap := env.capture(t, v1.ID)
	restored, err := env.svc.Restore(ctx, env.scope, craft.SnapshotID(v1Snap), ws.Revision)
	require.NoError(t, err)
	require.Equal(t, ws.ID, restored.ID)
}
