package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

type executorRunStore struct{ run Run }

func (s *executorRunStore) Admit(context.Context, Admission) (Run, error) { return Run{}, nil }
func (s *executorRunStore) Get(context.Context, RunKey) (Run, error)      { return s.run, nil }
func (*executorRunStore) Claim(context.Context, RunKey, string, time.Duration) (Fence, error) {
	return Fence{}, nil
}
func (*executorRunStore) Renew(context.Context, Fence, time.Duration) error { return nil }
func (*executorRunStore) Scan(context.Context, int) ([]RunKey, error)       { return nil, nil }
func (*executorRunStore) SaveCheckpoint(context.Context, Fence, CheckpointRecord) error {
	return nil
}

func (*executorRunStore) LoadCheckpoint(context.Context, RunKey) (CheckpointRecord, error) {
	return CheckpointRecord{}, nil
}

type executorJournal struct {
	record  ToolRecord
	attempt ToolAttempt
}

func (j *executorJournal) EnsureToolPlan(_ context.Context, _ Fence, plan ToolPlan) (ToolRecord, error) {
	if j.record.Plan.CallID == "" {
		j.record = ToolRecord{Plan: plan, Status: ToolStatusPlanned}
	}
	return j.record, nil
}

func (j *executorJournal) BeginToolAttempt(_ context.Context, fence Fence, callID string) (ToolAttempt, error) {
	j.attempt = ToolAttempt{CallID: callID, Number: j.attempt.Number + 1, Epoch: fence.Epoch}
	j.record.Status = ToolStatusDispatching
	return j.attempt, nil
}

func (j *executorJournal) CommitToolResult(
	_ context.Context, _ Fence, _ ToolAttempt, result StoredToolResult,
) error {
	j.record.Status = ToolStatusResult
	j.record.Result = &result
	return nil
}

func (j *executorJournal) MarkToolUnknown(
	_ context.Context, _ Fence, _ ToolAttempt, reason string,
) error {
	j.record.Status = ToolStatusUnknown
	j.record.UnknownReason = reason
	return nil
}

func TestToolExecutorMarksCanceledDispatchUnknown(t *testing.T) {
	fence := Fence{RunKey: RunKey{TenantID: 1, RunID: "r1"}, Owner: "w1", Epoch: 1}
	journal := &executorJournal{}
	executor := NewToolExecutor(
		&executorRunStore{run: Run{Key: fence.RunKey, Status: "running", Owner: fence.Owner, Epoch: fence.Epoch}},
		journal,
		func(context.Context, string, json.RawMessage) (*types.ToolResult, error) {
			return nil, context.DeadlineExceeded
		},
	)

	_, err := executor.Execute(context.Background(), fence, ToolPlan{
		CallID: "c1", Name: "write", Identity: "builtin/write@1", ArgsHash: "h1",
		Args: json.RawMessage(`{"path":"out.txt"}`),
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, ToolStatusUnknown, journal.record.Status)
	require.Contains(t, journal.record.UnknownReason, "deadline exceeded")

	_, err = executor.Execute(context.Background(), fence, journal.record.Plan)
	require.ErrorIs(t, err, ErrToolWaitUser)
	require.Equal(t, 1, journal.attempt.Number, "an unsafe unknown write must not dispatch again")
}

func TestToolExecutorRejectsInvalidDependenciesAndPlan(t *testing.T) {
	_, err := NewToolExecutor(nil, nil, nil).Execute(context.Background(), Fence{}, ToolPlan{})
	require.ErrorIs(t, err, ErrConflict)

	executor := NewToolExecutor(&executorRunStore{}, &executorJournal{}, func(
		context.Context, string, json.RawMessage,
	) (*types.ToolResult, error) {
		return nil, errors.New("must not run")
	})
	_, err = executor.Execute(context.Background(), Fence{}, ToolPlan{})
	require.ErrorIs(t, err, ErrConflict)
}
