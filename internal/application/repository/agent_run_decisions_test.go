package repository

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
)

func testDeadline() time.Time { return time.Now().Add(time.Hour) }

// admitDecisionRun seeds the admitted run every decision test starts from.
func admitDecisionRun(t *testing.T, store *AgentRunStore) error {
	t.Helper()
	_, err := store.Admit(context.Background(), agentruntime.Admission{
		Key:       agentruntime.RunKey{TenantID: 1, RunID: "r1"},
		SessionID: "s1", UserID: "u1", RequestID: "q1",
		AssistantMessageID: "a1", RequestHash: "h1",
		Snapshot:         json.RawMessage(`{"version":1}`),
		UserMessage:      json.RawMessage(`{"role":"user"}`),
		AssistantMessage: json.RawMessage(`{"role":"assistant"}`),
		Deadline:         testDeadline(),
	})
	return err
}

func TestAgentRunDecisionRequiresPendingAndRevision(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	_, err := store.Admit(context.Background(), agentruntime.Admission{
		Key: agentruntime.RunKey{TenantID: 1, RunID: "r1"}, SessionID: "s1", UserID: "u1",
		RequestID: "q1", AssistantMessageID: "a1", RequestHash: "h1",
		Snapshot: json.RawMessage(`{"version":1}`), UserMessage: json.RawMessage(`{"role":"user"}`),
		AssistantMessage: json.RawMessage(`{"role":"assistant"}`), Deadline: testDeadline(),
	})
	require.NoError(t, err)
	_, err = store.ApplyDecision(context.Background(),
		agentruntime.RunKey{TenantID: 1, RunID: "r1"}, "u1", agentruntime.Decision{
			PendingID: "p1", DecisionID: "d1", Action: "retry", ExpectedRevision: 0,
		})
	require.ErrorIs(t, err, agentruntime.ErrConflict)
}

func TestAgentRunDecisionConcurrentOnlyOneRevision(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	_, err := store.Admit(context.Background(), agentruntime.Admission{
		Key: agentruntime.RunKey{TenantID: 1, RunID: "r1"}, SessionID: "s1", UserID: "u1",
		RequestID: "q1", AssistantMessageID: "a1", RequestHash: "h1",
		Snapshot: json.RawMessage(`{"version":1}`), UserMessage: json.RawMessage(`{"role":"user"}`),
		AssistantMessage: json.RawMessage(`{"role":"assistant"}`), Deadline: testDeadline(),
	})
	require.NoError(t, err)
	// Put the run into a durable waiting state with a pending marker in its reason.
	require.NoError(t, db.Exec("UPDATE agent_runs SET status='waiting_user', wait_reason='p1', revision=7 "+
		"WHERE tenant_id=1 AND run_id='r1'").Error)
	decision := agentruntime.Decision{
		PendingID: "p1", DecisionID: "d1", Action: "retry",
		Reason: "已检查外部系统", ExpectedRevision: 7,
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	runs := make(chan agentruntime.Run, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := store.ApplyDecision(context.Background(),
				agentruntime.RunKey{TenantID: 1, RunID: "r1"}, "u1", decision)
			if e == nil {
				runs <- r
			}
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	var success int
	for e := range errs {
		if e == nil {
			success++
		} else {
			require.ErrorIs(t, e, agentruntime.ErrConflict)
		}
	}
	require.Equal(t, 2, success) // second request is an idempotent replay
	close(runs)
	for range runs {
	}
}

func TestAgentRunDecisionSameIDIsIdempotent(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	require.NoError(t, admitDecisionRun(t, store))
	require.NoError(t, db.Exec("INSERT INTO agent_tool_calls "+
		"(tenant_id,run_id,call_id,call_seq,tool_name,tool_identity,args_hash,args,status,unknown_reason) "+
		"VALUES (1,'r1','c1',1,'write','write:v1','ah1','{}','unknown','p1')").Error)
	require.NoError(t, db.Exec("UPDATE agent_runs SET status='waiting_user', wait_reason='p1', revision=7 "+
		"WHERE tenant_id=1 AND run_id='r1'").Error)
	d := agentruntime.Decision{
		PendingID: "p1", ToolCallID: "c1", ArgsHash: "ah1",
		DecisionID: "d1", Action: "provide_result", Reason: "user supplied",
		ExpectedRevision: 7, Result: json.RawMessage(`{"success":true,"output":"ok"}`),
	}
	first, err := store.ApplyDecision(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"}, "u1", d)
	require.NoError(t, err)
	second, err := store.ApplyDecision(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"}, "u1", d)
	require.NoError(t, err)
	require.Equal(t, first.Revision, second.Revision)
}

func TestAgentRunDecisionProvideResultWithoutLinkageRejected(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	require.NoError(t, admitDecisionRun(t, store))
	require.NoError(t, db.Exec("UPDATE agent_runs SET status='waiting_user', wait_reason='p1', revision=7 "+
		"WHERE tenant_id=1 AND run_id='r1'").Error)
	_, err := store.ApplyDecision(context.Background(),
		agentruntime.RunKey{TenantID: 1, RunID: "r1"}, "u1", agentruntime.Decision{
			PendingID: "p1", DecisionID: "d1", Action: "provide_result",
			Reason: "user supplied", ExpectedRevision: 7,
			Result: json.RawMessage(`{"success":true,"output":"ok"}`),
		})
	require.ErrorIs(t, err, agentruntime.ErrConflict)
}

func TestAgentRunDecisionRetryPreflightParkRequeuesRun(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	require.NoError(t, admitDecisionRun(t, store))
	require.NoError(t, db.Exec("INSERT INTO agent_tool_calls "+
		"(tenant_id,run_id,call_id,call_seq,tool_name,tool_identity,args_hash,args,status) "+
		"VALUES (1,'r1','c1',1,'fetch','fetch:v1','ah1','{}','planned')").Error)
	// Production order: the run holds its lease while the gate parks the
	// planned call, then SetStatus moves the run to waiting_user.
	fence := agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 1, RunID: "r1"}, Owner: "w", Epoch: 1}
	require.NoError(t, db.Exec("UPDATE agent_runs SET status='running', lease_owner='w', epoch=1, "+
		"lease_until=datetime('now','+1 hour') WHERE tenant_id=1 AND run_id='r1'").Error)
	require.NoError(t, store.ParkToolPreflightWait(context.Background(), fence, "c1", "mcp_oauth_x", "svc-1"))
	require.NoError(t, db.Exec("UPDATE agent_runs SET status='waiting_user', wait_reason='mcp_oauth_x', "+
		"revision=7, lease_owner='', lease_until=NULL WHERE tenant_id=1 AND run_id='r1'").Error)
	run, err := store.ApplyDecision(context.Background(),
		agentruntime.RunKey{TenantID: 1, RunID: "r1"}, "u1", agentruntime.Decision{
			PendingID: "mcp_oauth_x", ToolCallID: "c1", ArgsHash: "ah1",
			DecisionID: "d1", Action: "retry", Reason: "reconnected", ExpectedRevision: 7,
			ResourceRef: "svc-1",
		})
	require.NoError(t, err)
	require.Equal(t, "queued", run.Status)
	var tool struct{ Status, UnknownReason string }
	require.NoError(t, db.Raw("SELECT status, unknown_reason FROM agent_tool_calls "+
		"WHERE tenant_id=1 AND run_id='r1' AND call_id='c1'").Scan(&tool).Error)
	require.Equal(t, "planned", tool.Status, "preflight park stays planned; the next dispatch opens the attempt")
	require.Empty(t, tool.UnknownReason, "retry clears the linkage marker")
	var attempts int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM agent_tool_attempts "+
		"WHERE tenant_id=1 AND run_id='r1' AND call_id='c1'").Scan(&attempts).Error)
	require.Zero(t, attempts)
	var decision struct{ ResourceRef, ToolCallID string }
	require.NoError(t, db.Raw("SELECT resource_ref, tool_call_id FROM agent_run_decisions "+
		"WHERE tenant_id=1 AND run_id='r1' AND decision_id='d1'").Scan(&decision).Error)
	require.Equal(t, "svc-1", decision.ResourceRef)
	require.Equal(t, "c1", decision.ToolCallID)
}

func TestAgentRunDecisionProvideResultLinksUnknownTool(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	require.NoError(t, admitDecisionRun(t, store))
	require.NoError(t, db.Exec("INSERT INTO agent_tool_calls "+
		"(tenant_id,run_id,call_id,call_seq,tool_name,tool_identity,args_hash,args,status,unknown_reason) "+
		"VALUES (1,'r1','c1',1,'write','write:v1','ah1','{}','unknown','p1')").Error)
	require.NoError(t, db.Exec("UPDATE agent_runs SET status='waiting_user', wait_reason='p1', revision=7 "+
		"WHERE tenant_id=1 AND run_id='r1'").Error)
	d := agentruntime.Decision{
		PendingID: "p1", ToolCallID: "c1", ArgsHash: "ah1",
		DecisionID: "d1", Action: "provide_result", Reason: "user supplied",
		ExpectedRevision: 7, Result: json.RawMessage(`{"success":true,"output":"ok"}`),
		ResourceRef: "oauth://example",
	}
	_, err := store.ApplyDecision(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"}, "u1", d)
	require.NoError(t, err)
	var row struct{ Status, Source, Result string }
	require.NoError(t, db.Raw("SELECT status,source,result FROM agent_tool_calls "+
		"WHERE tenant_id=1 AND run_id='r1' AND call_id='c1'").Scan(&row).Error)
	require.Equal(t, "succeeded", row.Status)
	require.Equal(t, "user", row.Source)
	require.JSONEq(t, string(d.Result), row.Result)
	var decision struct{ ArgsHash, ResourceRef, ToolCallID string }
	require.NoError(t, db.Raw("SELECT args_hash,resource_ref,tool_call_id FROM agent_run_decisions "+
		"WHERE tenant_id=1 AND run_id='r1' AND decision_id='d1'").Scan(&decision).Error)
	require.Equal(t, "ah1", decision.ArgsHash)
	require.Equal(t, "oauth://example", decision.ResourceRef)
	require.Equal(t, "c1", decision.ToolCallID)
}

func TestAgentRunDecisionRejectsMismatchedToolLink(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	require.NoError(t, admitDecisionRun(t, store))
	require.NoError(t, db.Exec("INSERT INTO agent_tool_calls "+
		"(tenant_id,run_id,call_id,call_seq,tool_name,tool_identity,args_hash,args,status,unknown_reason) "+
		"VALUES (1,'r1','c1',1,'write','write:v1','ah1','{}','unknown','p1')").Error)
	require.NoError(t, db.Exec("UPDATE agent_runs SET status='waiting_user', wait_reason='p1', revision=7 "+
		"WHERE tenant_id=1 AND run_id='r1'").Error)
	_, err := store.ApplyDecision(context.Background(),
		agentruntime.RunKey{TenantID: 1, RunID: "r1"}, "u1", agentruntime.Decision{
			PendingID: "p1", ToolCallID: "c2", ArgsHash: "ah1",
			DecisionID: "d1", Action: "retry", Reason: "checked", ExpectedRevision: 7,
		})
	require.ErrorIs(t, err, agentruntime.ErrConflict)
}
