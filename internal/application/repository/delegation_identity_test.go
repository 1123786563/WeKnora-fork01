package repository

// CFT-S02-T014: delegation identity is DURABLE — the (tenant, run, tool
// call) row with its RequestHash and PromptMessageID lands in the database
// BEFORE any dispatch, so a process that dies after prepare can still be
// reconciled against the remote message by the next process reading the
// same store. (Placed beside the store's own tests to reuse the real DB
// fixtures; the opencode executor consumes these facts — see craft_delegate.go.)
import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
)

func TestDelegationIdentitySameKeySamePayloadReplays(t *testing.T) {
	db := openCraftDB(t)
	store := NewCraftStore(db)
	ctx := context.Background()
	fence := seedCraftRun(t, db, "r-ident-1", "c1")
	scope := craftTestScope()
	ws := putCraftWorkspace(t, store)

	task := craft.Task{
		ToolCallID: "c1", Prompt: "build the report", RequestHash: "rh-ident",
		Scope: scope, Fence: fence, WorkspaceID: ws.ID,
		Deadline: time.Now().Add(30 * time.Minute).UTC().Truncate(time.Microsecond),
	}
	first, err := store.PrepareTask(ctx, task)
	require.NoError(t, err)

	// The SAME key with the SAME payload replays the SAME task — no second
	// delegation is ever created (the unique (tenant, run, tool_call) row is
	// the idempotency boundary, not an in-memory map).
	again, err := store.PrepareTask(ctx, task)
	require.NoError(t, err)
	require.Equal(t, first.ID, again.ID)
	require.Equal(t, first.PromptMessageID, again.PromptMessageID)
	require.Equal(t, first.RequestHash, again.RequestHash)

	var rows int64
	require.NoError(t, db.Table("craft_delegations").Where("run_id = ?", fence.RunID).Count(&rows).Error)
	require.Equal(t, int64(1), rows, "exactly one durable delegation row")
}

func TestDelegationIdentitySameKeyDifferentPayloadConflicts(t *testing.T) {
	db := openCraftDB(t)
	store := NewCraftStore(db)
	ctx := context.Background()
	fence := seedCraftRun(t, db, "r-ident-2", "c1")
	ws := putCraftWorkspace(t, store)

	task := craft.Task{
		ToolCallID: "c1", Prompt: "original prompt", RequestHash: "rh-a",
		Scope: craftTestScope(), Fence: fence, WorkspaceID: ws.ID,
		Deadline: time.Now().Add(30 * time.Minute).UTC().Truncate(time.Microsecond),
	}
	_, err := store.PrepareTask(ctx, task)
	require.NoError(t, err)

	drifted := task
	drifted.Prompt = "mutated prompt"
	drifted.RequestHash = "rh-b"
	_, err = store.PrepareTask(ctx, drifted)
	require.ErrorIs(t, err, craft.ErrConflict, "a same-key different-payload prepare is a conflict, never an overwrite")
}

func TestDelegationIdentitySurvivesProcessExit(t *testing.T) {
	// A fresh store over the SAME database models the next process: the
	// delegation row written BEFORE dispatch carries every fact needed to
	// reconcile against the remote message — task id, prompt message id,
	// request hash, workspace binding and status.
	db := openCraftDB(t)
	ctx := context.Background()
	fence := seedCraftRun(t, db, "r-ident-3", "c1")
	scope := craftTestScope()
	ws := putCraftWorkspace(t, NewCraftStore(db))

	prepared, err := NewCraftStore(db).PrepareTask(ctx, craft.Task{
		ToolCallID: "c1", Prompt: "two-turn report", RequestHash: "rh-restart",
		Scope: scope, Fence: fence, WorkspaceID: ws.ID,
		Deadline: time.Now().Add(30 * time.Minute).UTC().Truncate(time.Microsecond),
	})
	require.NoError(t, err)

	restarted := NewCraftStore(db) // "process exit" — only the database remains
	got, err := restarted.GetTask(ctx, scope, prepared.ID)
	require.NoError(t, err)
	require.Equal(t, prepared.ID, got.ID)
	require.Equal(t, prepared.PromptMessageID, got.PromptMessageID, "the remote-message binding survives the restart")
	require.Equal(t, "rh-restart", got.RequestHash)
	require.Equal(t, fence.RunID, got.Fence.RunID)
	require.Equal(t, ws.ID, got.WorkspaceID)

	var status string
	require.NoError(t, db.Table("craft_delegations").
		Where("tenant_id = ? AND run_id = ? AND tool_call_id = ?", fence.TenantID, fence.RunID, "c1").
		Select("status").Take(&status).Error)
	require.Equal(t, "prepared", status, "the durable row honestly says prepared (not dispatched/succeeded)")
}
