package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func claimedToolRun(t *testing.T) (*AgentRunStore, agentruntime.Fence) {
	t.Helper()
	store := NewAgentRunStore(openRunTestDB(t))
	_, err := store.Admit(context.Background(), testAdmission())
	require.NoError(t, err)
	fence, err := store.Claim(context.Background(), testAdmission().Key, "tool-worker", time.Minute)
	require.NoError(t, err)
	return store, fence
}

func testToolPlan() agentruntime.ToolPlan {
	return agentruntime.ToolPlan{
		CallID: "call-1", Name: "write_sandbox_file", Identity: "builtin/write_sandbox_file@1",
		ArgsHash: "sha256:args-1", Args: json.RawMessage(`{"path":"report.md","content":"done"}`),
		IdempotencyKey: "idem-1", IdempotencyExpiresAt: time.Now().Add(time.Hour).UTC().Truncate(time.Millisecond),
	}
}

func TestAgentRunToolPlanIdentityAndTenantGuards(t *testing.T) {
	store, fence := claimedToolRun(t)
	ctx := context.Background()
	plan := testToolPlan()

	first, err := store.EnsureToolPlan(ctx, fence, plan)
	require.NoError(t, err)
	require.Equal(t, agentruntime.ToolStatusPlanned, first.Status)
	require.EqualValues(t, 1, first.CallSeq)
	require.Equal(t, plan.IdempotencyKey, first.Plan.IdempotencyKey)
	require.WithinDuration(t, plan.IdempotencyExpiresAt, first.Plan.IdempotencyExpiresAt, time.Millisecond)

	again, err := store.EnsureToolPlan(ctx, fence, plan)
	require.NoError(t, err)
	require.Equal(t, first.CallSeq, again.CallSeq)

	changed := plan
	changed.Args = json.RawMessage(`{"path":"other.md","content":"done"}`)
	require.ErrorIs(t, func() error {
		_, e := store.EnsureToolPlan(ctx, fence, changed)
		return e
	}(), agentruntime.ErrConflict)
	changed = plan
	changed.Identity = "builtin/write_sandbox_file@2"
	require.ErrorIs(t, func() error {
		_, e := store.EnsureToolPlan(ctx, fence, changed)
		return e
	}(), agentruntime.ErrConflict)

	wrongTenant := fence
	wrongTenant.TenantID = 2
	require.ErrorIs(t, func() error {
		_, e := store.EnsureToolPlan(ctx, wrongTenant, plan)
		return e
	}(), agentruntime.ErrLeaseLost)
}

func TestToolResultEnvelopeSurvivesReopenWithoutDuplicateExecution(t *testing.T) {
	store, fence := claimedToolRun(t)
	ctx := context.Background()
	plan := testToolPlan()
	var calls atomic.Int32
	execute := func(ctx context.Context, _ string, _ json.RawMessage) (*types.ToolResult, error) {
		if err := agentruntime.BeforeToolDispatch(ctx); err != nil {
			return nil, err
		}
		calls.Add(1)
		return &types.ToolResult{
			Success: true, Output: "created", Data: map[string]any{"bytes": float64(7)},
			OutputFiles: []string{"sandbox:report.md"}, Images: []string{"data:image/png;base64,AAAA"},
		}, nil
	}

	first, err := agentruntime.NewToolExecutor(store, store, execute).Execute(ctx, fence, plan)
	require.NoError(t, err)
	require.Equal(t, []string{"sandbox:report.md"}, first.OutputFiles)
	require.Equal(t, first.OutputFiles, first.Result.OutputFiles)
	require.Equal(t, []string{"data:image/png;base64,AAAA"}, first.Result.Images)

	reopened := NewAgentRunStore(reopenRunDB(t, store.db))
	second, err := agentruntime.NewToolExecutor(reopened, reopened, execute).Execute(ctx, fence, plan)
	require.NoError(t, err)
	require.EqualValues(t, 1, calls.Load(), "a committed result must be reused after process reconstruction")
	require.Equal(t, first, second)

	record, err := reopened.EnsureToolPlan(ctx, fence, plan)
	require.NoError(t, err)
	require.Equal(t, agentruntime.ToolStatusSucceeded, record.Status)
	require.NotNil(t, record.Result)
	require.Equal(t, []string{"sandbox:report.md"}, record.Result.OutputFiles)
	require.Equal(t, record.Result.OutputFiles, record.Result.Result.OutputFiles)
	require.Equal(t, []string{"data:image/png;base64,AAAA"}, record.Result.Result.Images)
}

func TestAgentRunToolUnknownOutcomeWaitsAndKeepsAttempt(t *testing.T) {
	store, fence := claimedToolRun(t)
	ctx := context.Background()
	plan := testToolPlan()
	plan.IdempotencyKey = ""
	plan.IdempotencyExpiresAt = time.Time{}
	var calls atomic.Int32
	executor := agentruntime.NewToolExecutor(store, store, func(
		ctx context.Context, _ string, _ json.RawMessage,
	) (*types.ToolResult, error) {
		if err := agentruntime.BeforeToolDispatch(ctx); err != nil {
			return nil, err
		}
		calls.Add(1)
		return nil, context.DeadlineExceeded
	})

	_, err := executor.Execute(ctx, fence, plan)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	reopened := NewAgentRunStore(reopenRunDB(t, store.db))
	_, err = agentruntime.NewToolExecutor(reopened, reopened, func(
		context.Context, string, json.RawMessage,
	) (*types.ToolResult, error) {
		calls.Add(1)
		return nil, errors.New("must not dispatch")
	}).Execute(ctx, fence, plan)
	require.ErrorIs(t, err, agentruntime.ErrToolWaitUser)
	require.EqualValues(t, 1, calls.Load())

	var attempts []agentToolAttemptRow
	require.NoError(t, reopened.db.Where("tenant_id = ? AND run_id = ? AND call_id = ?", 1, "r1", plan.CallID).
		Order("attempt ASC").Find(&attempts).Error)
	require.Len(t, attempts, 1)
	require.Equal(t, agentruntime.ToolStatusUnknown, attempts[0].Status)
}

func TestAgentRunToolOldEpochCannotCommitResult(t *testing.T) {
	store, fence := claimedToolRun(t)
	ctx := context.Background()
	plan := testToolPlan()
	_, err := store.EnsureToolPlan(ctx, fence, plan)
	require.NoError(t, err)
	attempt, err := store.BeginToolAttempt(ctx, fence, plan.CallID)
	require.NoError(t, err)

	require.NoError(t, store.db.Exec("UPDATE agent_runs SET lease_until = ?", time.Now().Add(-time.Hour)).Error)
	newFence, err := store.Claim(ctx, fence.RunKey, "replacement", time.Minute)
	require.NoError(t, err)
	err = store.CommitToolResult(ctx, fence, attempt, agentruntime.StoredToolResult{
		Result: types.ToolResult{Success: true, Output: "stale"}, Source: "tool",
	})
	require.ErrorIs(t, err, agentruntime.ErrLeaseLost)

	record, err := store.EnsureToolPlan(ctx, newFence, plan)
	require.NoError(t, err)
	require.Equal(t, agentruntime.ToolStatusDispatching, record.Status)
	require.Nil(t, record.Result)
}

func TestAgentRunToolIdempotentRetryKeepsKeyAndAttemptHistory(t *testing.T) {
	store, fence := claimedToolRun(t)
	plan := testToolPlan()
	plan.RecoveryPolicy = agentruntime.ToolRecoveryIdempotent
	var calls int
	executor := agentruntime.NewToolExecutor(store, store, func(
		ctx context.Context, _ string, _ json.RawMessage,
	) (*types.ToolResult, error) {
		if err := agentruntime.BeforeToolDispatch(ctx); err != nil {
			return nil, err
		}
		metadata, ok := agentruntime.ToolDispatchFromContext(ctx)
		require.True(t, ok)
		require.Equal(t, "idem-1", metadata.IdempotencyKey)
		require.Equal(t, plan.CallID, metadata.CallID)
		require.Equal(t, fence.RunKey, metadata.RunKey)
		calls++
		require.Equal(t, calls, metadata.Attempt)
		if calls == 1 {
			return nil, context.Canceled
		}
		return &types.ToolResult{Success: true, Output: "saved once"}, nil
	})
	_, err := executor.Execute(context.Background(), fence, plan)
	require.ErrorIs(t, err, context.Canceled)
	result, err := executor.Execute(context.Background(), fence, plan)
	require.NoError(t, err)
	require.Equal(t, "saved once", result.Result.Output)
	var attempts []agentToolAttemptRow
	require.NoError(t, store.db.Order("attempt ASC").Find(&attempts).Error)
	require.Len(t, attempts, 2)
	require.Equal(t, agentruntime.ToolStatusUnknown, attempts[0].Status)
	require.Equal(t, agentruntime.ToolStatusSucceeded, attempts[1].Status)
	require.Contains(t, attempts[0].ErrorMessage, "canceled")
}

func TestAgentRunToolLiveDispatchCannotBeRepeated(t *testing.T) {
	store, fence := claimedToolRun(t)
	plan := testToolPlan()
	plan.RecoveryPolicy = agentruntime.ToolRecoveryReadOnly
	ctx := context.Background()
	_, err := store.EnsureToolPlan(ctx, fence, plan)
	require.NoError(t, err)
	first, err := store.BeginToolAttempt(ctx, fence, plan.CallID)
	require.NoError(t, err)
	_, err = store.BeginToolAttempt(ctx, fence, plan.CallID)
	require.ErrorIs(t, err, agentruntime.ErrConflict)
	require.NoError(t, store.MarkToolUnknown(ctx, fence, first, "timeout"))
	second, err := store.BeginToolAttempt(ctx, fence, plan.CallID)
	require.NoError(t, err)
	require.Equal(t, first.Number+1, second.Number)
	result := agentruntime.StoredToolResult{Result: types.ToolResult{Success: true}}
	err = store.CommitToolResult(ctx, fence, first, result)
	require.ErrorIs(t, err, agentruntime.ErrConflict)
	require.NoError(t, store.CommitToolResult(ctx, fence, second, result))
	err = store.MarkToolUnknown(ctx, fence, second, "late timeout")
	require.ErrorIs(t, err, agentruntime.ErrConflict)
}

func TestAgentRunToolRejectsRevokedSessionAndCanceledRun(t *testing.T) {
	for _, mutation := range []string{
		"UPDATE sessions SET user_id = 'another-user' WHERE id = 's1'",
		"UPDATE sessions SET deleted_at = CURRENT_TIMESTAMP WHERE id = 's1'",
		"UPDATE agent_runs SET status = 'cancel_requested'",
		"UPDATE agent_runs SET lease_until = '2000-01-01 00:00:00'",
		"UPDATE agent_runs SET deadline = '2000-01-01 00:00:00'",
	} {
		t.Run(mutation, func(t *testing.T) {
			store, fence := claimedToolRun(t)
			plan := testToolPlan()
			_, err := store.EnsureToolPlan(context.Background(), fence, plan)
			require.NoError(t, err)
			require.NoError(t, store.db.Exec(mutation).Error)
			_, err = store.BeginToolAttempt(context.Background(), fence, plan.CallID)
			require.ErrorIs(t, err, agentruntime.ErrLeaseLost)
		})
	}
}

func TestAgentRunToolRecoveryUsesImmutablePlanAndExpiry(t *testing.T) {
	store, fence := claimedToolRun(t)
	plan := testToolPlan()
	plan.RecoveryPolicy = agentruntime.ToolRecoveryIdempotent
	plan.IdempotencyExpiresAt = time.Now().Add(-time.Minute).UTC().Truncate(time.Millisecond)
	ctx := context.Background()
	_, err := store.EnsureToolPlan(ctx, fence, plan)
	require.NoError(t, err)
	attempt, err := store.BeginToolAttempt(ctx, fence, plan.CallID)
	require.NoError(t, err)
	require.NoError(t, store.MarkToolUnknown(ctx, fence, attempt, "timeout"))
	_, err = store.BeginToolAttempt(ctx, fence, plan.CallID)
	require.ErrorIs(t, err, agentruntime.ErrToolWaitUser)
	changed := plan
	changed.IdempotencyExpiresAt = time.Now().Add(time.Hour)
	_, err = store.EnsureToolPlan(ctx, fence, changed)
	require.ErrorIs(t, err, agentruntime.ErrConflict)
	changed = plan
	changed.RecoveryPolicy = agentruntime.ToolRecoveryReadOnly
	_, err = store.EnsureToolPlan(ctx, fence, changed)
	require.ErrorIs(t, err, agentruntime.ErrConflict)
	changed = plan
	changed.IdempotencyKey = "new-key"
	_, err = store.EnsureToolPlan(ctx, fence, changed)
	require.ErrorIs(t, err, agentruntime.ErrConflict)
}

func TestAgentRunToolCommittedResultValidatesCheckpoint(t *testing.T) {
	for _, success := range []bool{true, false} {
		t.Run(fmt.Sprint(success), func(t *testing.T) {
			store, fence := claimedToolRun(t)
			plan := testToolPlan()
			ctx := context.Background()
			_, err := agentruntime.NewToolExecutor(store, store, func(
				ctx context.Context, _ string, _ json.RawMessage,
			) (*types.ToolResult, error) {
				if err := agentruntime.BeforeToolDispatch(ctx); err != nil {
					return nil, err
				}
				return &types.ToolResult{Success: success, Output: "provider response"}, nil
			}).Execute(ctx, fence, plan)
			require.NoError(t, err)
			err = store.ValidateCheckpointCalls(ctx, fence.RunKey, nil, map[string]bool{plan.CallID: true})
			require.NoError(t, err)
		})
	}
}

func TestAgentRunToolPersistsReturnedResultAfterRequestCancellation(t *testing.T) {
	store, fence := claimedToolRun(t)
	plan := testToolPlan()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls int
	execute := func(ctx context.Context, _ string, _ json.RawMessage) (*types.ToolResult, error) {
		if err := agentruntime.BeforeToolDispatch(ctx); err != nil {
			return nil, err
		}
		calls++
		cancel()
		return &types.ToolResult{Success: true, Output: "side effect completed"}, nil
	}
	_, err := agentruntime.NewToolExecutor(store, store, execute).Execute(ctx, fence, plan)
	require.NoError(t, err)
	reopened := NewAgentRunStore(reopenRunDB(t, store.db))
	result, err := agentruntime.NewToolExecutor(reopened, reopened, execute).Execute(context.Background(), fence, plan)
	require.NoError(t, err)
	require.Equal(t, "side effect completed", result.Result.Output)
	require.Equal(t, 1, calls)
}
