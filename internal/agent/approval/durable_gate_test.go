package approval

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

type recordingGate struct{ oauthCalled int }

func (g *recordingGate) NeedsApproval(context.Context, uint64, string, string) bool {
	return false
}

func (g *recordingGate) IsEnabled(context.Context, uint64, string, string) (bool, error) {
	return true, nil
}

func (g *recordingGate) RequestAndWait(context.Context, PendingRequest) (Decision, error) {
	return Decision{}, nil
}

func (g *recordingGate) RequestOAuthAndWait(context.Context, OAuthPendingRequest) (Decision, error) {
	g.oauthCalled++
	return Decision{}, errors.New("live wait would block here")
}

// gateRunStore and gateJournal are the minimal durable seams the tool
// executor needs, so the park test exercises the real dispatch context.
type gateRunStore struct{ fence agentruntime.Fence }

func (s *gateRunStore) Admit(context.Context, agentruntime.Admission) (agentruntime.Run, error) {
	return agentruntime.Run{}, nil
}

func (s *gateRunStore) Get(context.Context, agentruntime.RunKey) (agentruntime.Run, error) {
	return agentruntime.Run{
		Key: s.fence.RunKey, Owner: s.fence.Owner, Epoch: s.fence.Epoch, Status: "running",
		LeaseUntil: time.Now().Add(time.Minute),
	}, nil
}

func (s *gateRunStore) Claim(context.Context, agentruntime.RunKey, string, time.Duration) (agentruntime.Fence, error) {
	return s.fence, nil
}

func (s *gateRunStore) Renew(context.Context, agentruntime.Fence, time.Duration) error {
	return nil
}

func (s *gateRunStore) Scan(context.Context, int) ([]agentruntime.RunKey, error) {
	return nil, nil
}

func (s *gateRunStore) SaveCheckpoint(
	context.Context, agentruntime.Fence, agentruntime.CheckpointRecord,
) error {
	return nil
}

func (s *gateRunStore) SetStatus(context.Context, agentruntime.Fence, string, string) error {
	return nil
}

func (s *gateRunStore) LoadCheckpoint(
	context.Context, agentruntime.RunKey,
) (agentruntime.CheckpointRecord, error) {
	return agentruntime.CheckpointRecord{}, agentruntime.ErrNotFound
}

type gateJournal struct{}

func (gateJournal) EnsureToolPlan(
	_ context.Context, _ agentruntime.Fence, plan agentruntime.ToolPlan,
) (agentruntime.ToolRecord, error) {
	return agentruntime.ToolRecord{Plan: plan, Status: agentruntime.ToolStatusPlanned}, nil
}

func (gateJournal) BeginToolAttempt(
	context.Context, agentruntime.Fence, string, ...int64,
) (agentruntime.ToolAttempt, error) {
	return agentruntime.ToolAttempt{Number: 1}, nil
}

func (gateJournal) ReviseToolPlan(
	context.Context, agentruntime.Fence, string, int64, json.RawMessage,
) (agentruntime.ToolPlan, error) {
	return agentruntime.ToolPlan{}, nil
}

func (gateJournal) CommitToolResult(
	context.Context, agentruntime.Fence, agentruntime.ToolAttempt, agentruntime.StoredToolResult,
) error {
	return nil
}

func (gateJournal) CommitToolRejection(
	context.Context, agentruntime.Fence, string, agentruntime.StoredToolResult,
) error {
	return nil
}

func (gateJournal) MarkToolUnknown(
	context.Context, agentruntime.Fence, agentruntime.ToolAttempt, string,
) error {
	return nil
}

func TestDurableGateParksPreflightOAuthWait(t *testing.T) {
	prev := durableOAuthPark
	parked := false
	var gotFence agentruntime.Fence
	var gotDispatch agentruntime.ToolDispatch
	SetDurableOAuthPark(func(
		_ context.Context, fence agentruntime.Fence, dispatch agentruntime.ToolDispatch,
		pendingID string, _ OAuthPendingRequest,
	) error {
		parked = true
		gotFence, gotDispatch = fence, dispatch
		require.NotEmpty(t, pendingID)
		require.Contains(t, pendingID, "mcp_oauth_")
		require.LessOrEqual(t, len(pendingID), 64, "wait_reason column is VARCHAR(64)")
		return nil
	})
	t.Cleanup(func() { SetDurableOAuthPark(prev) })

	fence := agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 7, RunID: "r1"}, Owner: "w", Epoch: 3}
	gate := NewDurableGate(nil)
	executor := agentruntime.NewToolExecutor(&gateRunStore{fence: fence}, gateJournal{},
		func(ctx context.Context, _ string, _ json.RawMessage) (*types.ToolResult, error) {
			_, err := gate.RequestOAuthAndWait(ctx, OAuthPendingRequest{ServiceID: "svc-1"})
			return nil, err
		})
	_, err := executor.Execute(context.Background(), fence, agentruntime.ToolPlan{
		Version: 1, CallID: "c1", Name: "fetch", Identity: "fetch",
		ArgsHash: "ah1", Args: []byte("{}"),
	})
	require.True(t, parked)
	require.Equal(t, fence, gotFence)
	require.Equal(t, "c1", gotDispatch.CallID)
	require.ErrorIs(t, err, agentruntime.ErrMCPOAuthWait)
	waitErr := &agentruntime.OAuthWaitError{}
	require.True(t, errors.As(err, &waitErr))
	require.Equal(t, "svc-1", waitErr.ServiceID)
}

func TestDurableGateParksPreflightApprovalWait(t *testing.T) {
	prev := durableOAuthPark
	parked := false
	var gotFence agentruntime.Fence
	var gotDispatch agentruntime.ToolDispatch
	SetDurableOAuthPark(func(
		_ context.Context, fence agentruntime.Fence, dispatch agentruntime.ToolDispatch,
		pendingID string, _ OAuthPendingRequest,
	) error {
		parked = true
		gotFence, gotDispatch = fence, dispatch
		require.Contains(t, pendingID, "mcp_approve_")
		require.LessOrEqual(t, len(pendingID), 64, "wait_reason column is VARCHAR(64)")
		return nil
	})
	t.Cleanup(func() { SetDurableOAuthPark(prev) })

	fence := agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 7, RunID: "r-approve"}, Owner: "w", Epoch: 3}
	gate := NewDurableGate(nil)
	executor := agentruntime.NewToolExecutor(&gateRunStore{fence: fence}, gateJournal{},
		func(ctx context.Context, _ string, _ json.RawMessage) (*types.ToolResult, error) {
			_, err := gate.RequestAndWait(ctx, PendingRequest{ServiceID: "svc-approve"})
			return nil, err
		})
	_, err := executor.Execute(context.Background(), fence, agentruntime.ToolPlan{
		Version: 1, CallID: "c1", Name: "fetch", Identity: "fetch",
		ArgsHash: "ah1", Args: []byte("{}"),
	})
	require.True(t, parked)
	require.Equal(t, fence, gotFence)
	require.Equal(t, "c1", gotDispatch.CallID)
	require.ErrorIs(t, err, agentruntime.ErrMCPApprovalWait)
	waitErr := &agentruntime.ApprovalWaitError{}
	require.True(t, errors.As(err, &waitErr))
	require.Equal(t, "svc-approve", waitErr.ServiceID)
}

func TestDurableGateDelegatesWithoutFence(t *testing.T) {
	prev := durableOAuthPark
	SetDurableOAuthPark(func(
		context.Context, agentruntime.Fence, agentruntime.ToolDispatch, string, OAuthPendingRequest,
	) error {
		t.Fatal("must not park without a run fence")
		return nil
	})
	t.Cleanup(func() { SetDurableOAuthPark(prev) })

	inner := &recordingGate{}
	gate := NewDurableGate(inner)
	_, err := gate.RequestOAuthAndWait(context.Background(), OAuthPendingRequest{ServiceID: "svc"})
	require.Error(t, err)
	require.Equal(t, 1, inner.oauthCalled, "the live gate keeps handling builtin waits")
}

func TestDurableGateParkFailurePropagates(t *testing.T) {
	prev := durableOAuthPark
	SetDurableOAuthPark(func(
		context.Context, agentruntime.Fence, agentruntime.ToolDispatch, string, OAuthPendingRequest,
	) error {
		return agentruntime.ErrLeaseLost
	})
	t.Cleanup(func() { SetDurableOAuthPark(prev) })

	fence := agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 1, RunID: "r"}, Owner: "w", Epoch: 1}
	gate := NewDurableGate(nil)
	executor := agentruntime.NewToolExecutor(&gateRunStore{fence: fence}, gateJournal{},
		func(ctx context.Context, _ string, _ json.RawMessage) (*types.ToolResult, error) {
			_, err := gate.RequestOAuthAndWait(ctx, OAuthPendingRequest{ServiceID: "svc"})
			return nil, err
		})
	_, err := executor.Execute(context.Background(), fence, agentruntime.ToolPlan{
		Version: 1, CallID: "c1", Name: "fetch", Identity: "fetch",
		ArgsHash: "ah1", Args: []byte("{}"),
	})
	require.ErrorIs(t, err, agentruntime.ErrLeaseLost)
}
