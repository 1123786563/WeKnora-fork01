package workbench

// T33 #63: the agent-use gate seam. A retired variant's local agent must
// not admit NEW work; starts without an agent (or with a live agent) pass
// untouched. Store-level evidence; the HTTP face is Task 6.

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestAdmissionAgentUseGateBlocksRetiredBeforePersisting(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	require.NoError(t, db.Exec("INSERT INTO sessions (id,tenant_id,title,user_id,engine_type) VALUES ('s2',1,'s2','u1','trpc'),('s3',1,'s3','u1','trpc')").Error)
	coordinator := NewAdmissionCoordinator(db, repository.NewAgentRunStore(db), nil, nil)
	calls := 0
	coordinator.SetAgentUseGate(func(_ context.Context, _ uint64, agentID string) error {
		calls++
		if agentID == "agent-retired" {
			return ErrAgentUseDenied
		}
		return nil
	})
	ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")

	_, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "agent-retired", TargetID: "platform", RequestID: "r1", Text: "hi", BudgetUpper: 1})
	require.ErrorIs(t, err, ErrAgentUseDenied)
	var pending int64
	require.NoError(t, db.Table("workbench_requests").Where("request_id = ?", "r1").Count(&pending).Error)
	require.EqualValues(t, 0, pending)

	run, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "agent-live", TargetID: "platform", RequestID: "r2", Text: "hi", BudgetUpper: 1})
	require.NoError(t, err)
	require.NotEmpty(t, run.Key.RunID)

	before := calls
	_, err = coordinator.Start(ctx, StartInput{SessionID: "s2", TargetID: "platform", RequestID: "r3", Text: "hi", BudgetUpper: 1})
	require.NoError(t, err)
	require.Equal(t, before, calls)

	coordinator.SetAgentUseGate(nil)
	_, err = coordinator.Start(ctx, StartInput{SessionID: "s3", AgentID: "agent-retired", TargetID: "platform", RequestID: "r4", Text: "hi", BudgetUpper: 1})
	require.NoError(t, err)
}
