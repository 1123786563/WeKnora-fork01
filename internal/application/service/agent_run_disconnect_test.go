package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// TestSubmitDurableAgentRunSurvivesDisconnect pins the spec contract
// (HTTP 断线不取消 Run): a client disconnect that cancels the request
// context between handler entry and admission must not abort the
// admission transaction — the run is durable the moment submission
// starts, and user cancellation flows through the durable cancel
// endpoint instead.
func TestSubmitDurableAgentRunSurvivesDisconnect(t *testing.T) {
	db := openDurableRunTestDB(t)
	store := repository.NewAgentRunStore(db)
	prev := RegisteredAgentRunService()
	RegisterAgentRunService(NewAgentRunService(store))
	t.Cleanup(func() { RegisterAgentRunService(prev) })
	svc := newDurableRunSessionService(t, db)
	svc.cfg = &config.Config{Agent: &config.AgentConfig{Recovery: config.AgentRecoveryConfig{AdmissionEnabled: true}}}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel() // the disconnect already happened
	req := &types.QARequest{
		Query:              "survive disconnect",
		Session:            &types.Session{TenantID: 1, ID: "s1", UserID: "u1", EngineType: "trpc"},
		AssistantMessageID: "amsg-disc-1",
	}
	err := svc.submitDurableAgentRun(cancelled, req, &types.AgentConfig{}, "m1", false)
	require.NoError(t, err, "admission must not abort on a cancelled request context")

	var runs int64
	require.NoError(t, db.Table("agent_runs").
		Where("session_id = ? AND status = ?", "s1", "queued").Count(&runs).Error)
	require.EqualValues(t, 1, runs, "the durable run must be admitted and queued")
}
