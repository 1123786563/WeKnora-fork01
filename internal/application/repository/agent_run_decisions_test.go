package repository

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func testDeadline() time.Time { return time.Now().Add(time.Hour) }

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
	_, err = store.ApplyDecision(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"}, "u1", agentruntime.Decision{
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
	require.NoError(t, db.Exec("UPDATE agent_runs SET status='waiting_user', wait_reason='p1', revision=7 WHERE tenant_id=1 AND run_id='r1'").Error)
	decision := agentruntime.Decision{PendingID: "p1", DecisionID: "d1", Action: "retry", Reason: "已检查外部系统", ExpectedRevision: 7}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	runs := make(chan agentruntime.Run, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := store.ApplyDecision(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"}, "u1", decision)
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
	var revisions []int64
	for r := range runs {
		revisions = append(revisions, r.Revision)
	}
	require.ElementsMatch(t, []int64{8, 8}, revisions)
	require.Equal(t, int64(8), mustRunRevision(t, db))
}

func mustRunRevision(t *testing.T, db *gorm.DB) int64 {
	var row struct{ Revision int64 }
	require.NoError(t, db.Raw("SELECT revision FROM agent_runs WHERE tenant_id=1 AND run_id='r1'").Scan(&row).Error)
	return row.Revision
}

func TestAgentRunDecisionSameIDIsIdempotent(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	_, err := store.Admit(context.Background(), agentruntime.Admission{Key: agentruntime.RunKey{TenantID: 1, RunID: "r1"}, SessionID: "s1", UserID: "u1", RequestID: "q1", AssistantMessageID: "a1", RequestHash: "h1", Snapshot: json.RawMessage(`{"version":1}`), UserMessage: json.RawMessage(`{"role":"user"}`), AssistantMessage: json.RawMessage(`{"role":"assistant"}`), Deadline: testDeadline()})
	require.NoError(t, err)
	require.NoError(t, db.Exec("UPDATE agent_runs SET status='waiting_user', wait_reason='p1', revision=7 WHERE tenant_id=1 AND run_id='r1'").Error)
	d := agentruntime.Decision{PendingID: "p1", DecisionID: "d1", Action: "provide_result", Reason: "user supplied", ExpectedRevision: 7, Result: json.RawMessage(`{"success":true,"output":"ok"}`)}
	first, err := store.ApplyDecision(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"}, "u1", d)
	require.NoError(t, err)
	second, err := store.ApplyDecision(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"}, "u1", d)
	require.NoError(t, err)
	require.Equal(t, first.Revision, second.Revision)
	d.Reason = "different"
	_, err = store.ApplyDecision(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"}, "u1", d)
	require.ErrorIs(t, err, agentruntime.ErrConflict)
}
