package workbench

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	appservice "github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/application/repository"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// WB-GRAPH: a server-owned freezer merges the resolved graph execution core
// (version/query/model_id/agent_config/runtime) into the same admission
// snapshot JSON as the request identity and usage binding, so the durable
// executor's D8 fence (WorkbenchAdmissionSnapshot != nil && ModelID == "")
// passes naturally instead of failing the run terminally.
func TestAdmissionFreezesServerResolvedGraphCore(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	require.NoError(t, db.Exec("INSERT INTO tenant_members (tenant_id,user_id,role,status) VALUES (1,'u1','owner','active')").Error)
	runs := repository.NewAgentRunStore(db)
	coordinator := NewAdmissionCoordinator(db, runs, &retrySafeBudget{}, nil)
	freezer := &stubAdmissionGraphFreezer{snapshot: json.RawMessage(`{
		"version": 1, "query": "整理本周周报", "model_id": "model-9",
		"agent_config": {"max_iterations": 5, "allowed_tools": ["knowledge_search"]},
		"runtime": {"sandbox_config_id": "ws-1"}
	}`)}
	coordinator.SetAdmissionGraphFreezer(freezer)
	ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")

	run, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "builtin-quick-answer", TargetID: "platform", RequestID: "wb-graph-core", Text: "整理本周周报", BudgetUpper: 1})
	require.NoError(t, err)

	stored, err := runs.Get(ctx, agentruntime.RunKey{TenantID: 1, RunID: run.Key.RunID})
	require.NoError(t, err)
	parsed, err := appservice.ParseDurableRunSnapshot(stored.Snapshot)
	require.NoError(t, err, "the merged admission snapshot must round-trip the strict durable reader")
	require.Equal(t, "model-9", parsed.ModelID, "the frozen graph core model identity survives admission")
	require.NotEmpty(t, parsed.AgentConfig)
	require.Contains(t, string(parsed.AgentConfig), "knowledge_search")
	require.Equal(t, "ws-1", parsed.Runtime.SandboxConfigID)
	require.NotNil(t, parsed.WorkbenchAdmissionSnapshot, "admission identity keys are preserved by the merge")
	require.Equal(t, "wb-graph-core", parsed.WorkbenchAdmissionSnapshot.RequestID)
	require.Equal(t, "整理本周周报", parsed.WorkbenchAdmissionSnapshot.Text)
	require.Equal(t, "builtin-quick-answer", parsed.WorkbenchAdmissionSnapshot.AgentID)
	require.NotNil(t, parsed.RunUsageBindingSnapshot, "usage binding keys are preserved by the merge")
	require.Equal(t, "platform_gateway", parsed.RunUsageBindingSnapshot.UsageSource)
	require.Equal(t, 1, freezer.calls)
}

// WB-GRAPH: a resolution failure is an admission rejection with an explicit
// short code — no Run row is created, no reservation is left dangling, and a
// same-key retry replays the terminal rejection instead of re-freezing.
func TestAdmissionGraphResolutionFailureRejectsWithoutRun(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	require.NoError(t, db.Exec("INSERT INTO tenant_members (tenant_id,user_id,role,status) VALUES (1,'u1','owner','active')").Error)
	runs := repository.NewAgentRunStore(db)
	budget := &retrySafeBudget{}
	coordinator := NewAdmissionCoordinator(db, runs, budget, nil)
	freezer := &stubAdmissionGraphFreezer{err: fmt.Errorf("%w: model_unresolved: chat model is not configured: please set model_id on agent builtin-quick-answer", ErrGraphResolutionFailed)}
	coordinator.SetAdmissionGraphFreezer(freezer)
	ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")
	in := StartInput{SessionID: "s1", AgentID: "builtin-quick-answer", TargetID: "platform", RequestID: "wb-graph-reject", Text: "整理本周周报", BudgetUpper: 1}

	_, err := coordinator.Start(ctx, in)
	require.ErrorIs(t, err, ErrGraphResolutionFailed)
	require.Contains(t, err.Error(), "model_unresolved")

	state, err := coordinator.LookupRequest(ctx, in.RequestID)
	require.NoError(t, err)
	require.Equal(t, "rejected", state.State)
	require.Contains(t, state.Reason, "model_unresolved")

	var runRows int64
	require.NoError(t, db.Table("agent_runs").Where("request_id = ?", in.RequestID).Count(&runRows).Error)
	require.Zero(t, runRows, "a graph resolution failure must not leave a Run row behind")
	budget.mu.Lock()
	require.Zero(t, budget.creates, "no reservation is created when freezing fails before budget.Ensure")
	budget.mu.Unlock()

	_, retryErr := coordinator.Start(ctx, in)
	require.ErrorIs(t, retryErr, ErrRequestRejected)
	require.Contains(t, retryErr.Error(), "model_unresolved")
	require.Equal(t, 1, freezer.calls, "a settled rejection replays instead of re-invoking the freezer")
}

// WB-GRAPH: deployments without a freezer keep the exact legacy snapshot
// shape (admission identity + usage binding, no graph execution core), so
// the D8 executor fence still fails those runs explicitly.
func TestAdmissionWithoutFreezerKeepsLegacySnapshot(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	require.NoError(t, db.Exec("INSERT INTO tenant_members (tenant_id,user_id,role,status) VALUES (1,'u1','owner','active')").Error)
	runs := repository.NewAgentRunStore(db)
	coordinator := NewAdmissionCoordinator(db, runs, &retrySafeBudget{}, nil)
	ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")

	run, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "builtin-quick-answer", TargetID: "platform", RequestID: "wb-legacy-shape", Text: "整理本周周报", BudgetUpper: 1})
	require.NoError(t, err)

	stored, err := runs.Get(ctx, agentruntime.RunKey{TenantID: 1, RunID: run.Key.RunID})
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(stored.Snapshot, &fields))
	for _, key := range []string{"session_id", "request_id", "text", "usage_source"} {
		require.Contains(t, fields, key)
	}
	for _, key := range []string{"version", "query", "model_id", "agent_config", "runtime"} {
		require.NotContains(t, fields, key, "legacy admission snapshots carry no graph execution core")
	}
	parsed, err := appservice.ParseDurableRunSnapshot(stored.Snapshot)
	require.NoError(t, err)
	require.Empty(t, parsed.ModelID)
}

type stubAdmissionGraphFreezer struct {
	snapshot json.RawMessage
	err      error
	calls    int
}

func (f *stubAdmissionGraphFreezer) FreezeAdmissionGraph(context.Context, uint64, string, StartInput) (json.RawMessage, error) {
	f.calls++
	return f.snapshot, f.err
}
