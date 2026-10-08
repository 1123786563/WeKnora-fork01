package service

// T09 (#135) round-2 OCR regressions: the idempotent-replay timeline dedup
// and the single role derivation feeding both the TaskWrite gate and the
// timeline detail.
import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
)

// TestCraftRunT09IdempotentReplayDoesNotDuplicateTimeline pins the replay
// dedup: a client retrying the SAME request id replays the SAME admission
// (T01 idempotent key) — the craft.run_started timeline row must be written
// ONCE, not once per retry.
func TestCraftRunT09IdempotentReplayDoesNotDuplicateTimeline(t *testing.T) {
	env := newT09Env(t, craft.WriterAcquired)
	ws := createCraftSession(t, env.craftSessionEnv, "u1", "t09-replay", "站点", "web")
	owner := ownerScope(1, "u1", ws.SessionID)
	t09Grant(t, env, owner, "u2", craft.TaskRoleCollaborator)
	ctx := craftCtx(1, "u2", ws.SessionID)
	scope := ownerScope(1, "u2", ws.SessionID)

	first, _, err := env.svc.StartCollaboratorRun(ctx, scope, CraftRunRequest{
		RequestID: "t09-replay-1", Prompt: "把标题改成蓝色",
	})
	require.NoError(t, err)

	// The client retries the same key (the run is still live — the
	// idempotent admission replays without the busy conflict).
	second, _, err := env.svc.StartCollaboratorRun(ctx, scope, CraftRunRequest{
		RequestID: "t09-replay-1", Prompt: "把标题改成蓝色",
	})
	require.NoError(t, err, "the T01 idempotent key replays the same admission")
	require.Equal(t, first.Key.RunID, second.Key.RunID, "the replay is the SAME run")

	started := t09AuditRows(t, env, "craft.run_started")
	require.Len(t, started, 1, "an idempotent replay must not append a second run_started row")
	require.Equal(t, first.Key.RunID, started[0].TargetID)
	require.Equal(t, "u2", started[0].ActorUserID)

	// A third retry stays deduped as well (the probe keys on the Run identity).
	_, _, err = env.svc.StartCollaboratorRun(ctx, scope, CraftRunRequest{
		RequestID: "t09-replay-1", Prompt: "把标题改成蓝色",
	})
	require.NoError(t, err)
	require.Len(t, t09AuditRows(t, env, "craft.run_started"), 1, "every replay of one admission shares one timeline row")
}

// TestCraftRunT09SingleDerivationKeepsGateAndDetail pins the single-derivation
// rework: ONE CheckTaskAccessWithRole pass enforces the audited TaskWrite
// gate AND feeds the timeline detail — the denial audit, the refusal shape
// and the recorded role stay exactly as before.
func TestCraftRunT09SingleDerivationKeepsGateAndDetail(t *testing.T) {
	env := newT09Env(t, craft.WriterAcquired)
	ws := createCraftSession(t, env.craftSessionEnv, "u1", "t09-onederive", "站点", "web")
	owner := ownerScope(1, "u1", ws.SessionID)
	t09Grant(t, env, owner, "u2", craft.TaskRoleCollaborator)
	t09Grant(t, env, owner, "u3", craft.TaskRoleViewer)

	// Admitted path: the derived role lands in the timeline detail from the
	// same derivation that opened the gate.
	_, _, err := env.svc.StartCollaboratorRun(craftCtx(1, "u2", ws.SessionID), ownerScope(1, "u2", ws.SessionID), CraftRunRequest{
		RequestID: "t09-onederive-1", Prompt: "发起修改",
	})
	require.NoError(t, err)
	rows := t09AuditRows(t, env, "craft.run_started")
	require.Len(t, rows, 1)
	var details map[string]string
	require.NoError(t, json.Unmarshal(rows[0].Details, &details))
	require.Equal(t, string(craft.TaskRoleCollaborator), details["role"],
		"the single derivation still feeds the timeline role detail")

	// Refused path: the audited denial keeps its exact shape through the
	// role-returning seam.
	deniedBefore := len(t09AuditRows(t, env, "craft.access_denied"))
	_, _, err = env.svc.StartCollaboratorRun(craftCtx(1, "u3", ws.SessionID), ownerScope(1, "u3", ws.SessionID), CraftRunRequest{
		RequestID: "t09-onederive-viewer", Prompt: "viewer 尝试",
	})
	require.ErrorIs(t, err, craft.ErrForbidden)
	denied := t09AuditRows(t, env, "craft.access_denied")
	require.Len(t, denied, deniedBefore+1, "the viewer refusal is still audited through the gate")
	require.Equal(t, "u3", denied[0].ActorUserID)
	require.Equal(t, string(craft.TaskWrite), denied[0].TargetID)
	require.Empty(t, t09RunsForSession(t, env, ws.SessionID)[1:], "no second run was admitted")

	// CheckTaskAccess (the interface checker) still enforces identically
	// after delegating to the role-returning variant.
	require.ErrorIs(t, env.access.CheckTaskAccess(context.Background(), ownerScope(1, "u3", ws.SessionID), craft.TaskWrite), craft.ErrForbidden)
	require.NoError(t, env.access.CheckTaskAccess(context.Background(), ownerScope(1, "u1", ws.SessionID), craft.TaskWrite))
}
