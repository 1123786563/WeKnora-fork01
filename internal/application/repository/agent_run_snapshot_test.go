package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/workbench"
	"github.com/stretchr/testify/require"
)

type capabilityResolverStub struct{}

func (capabilityResolverStub) ResolveExecutionCapabilities(context.Context, string) map[string]workbench.Capability {
	return map[string]workbench.Capability{
		"platform": {State: workbench.CapabilityUnavailable, Reason: "provider_probe_failed"},
		"paseo":    {State: workbench.CapabilityForbidden, Reason: "tenant_disabled"},
	}
}

func TestReadRunSnapshotUsesAuthoritativeProjection(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	_, err := store.Admit(context.Background(), testAdmission())
	require.NoError(t, err)
	require.NoError(t, db.Exec(`INSERT INTO agent_run_events
		(tenant_id, run_id, seq, attempt_id, event_type, payload, created_at)
		VALUES (1, 'r1', 1, 'attempt-1', 'text.delta', '{"text":"hello"}', ?)`, time.Now().UTC()).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_run_events
		(tenant_id, run_id, seq, attempt_id, event_type, payload, created_at)
		VALUES (1, 'r1', 2, 'attempt-1', 'run.completed', '{"ok":true}', ?)`, time.Now().UTC()).Error)

	snapshot, err := NewAgentRunSnapshotRepository(db).ReadRunSnapshot(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"})
	require.NoError(t, err)
	require.Equal(t, int64(2), snapshot.Watermark)
	require.Equal(t, []int64{1, 2}, []int64{snapshot.Events[0].Seq, snapshot.Events[1].Seq})
	require.Equal(t, "text.delta", snapshot.Events[0].Type)
	require.Equal(t, "platform", snapshot.Execution.Driver)
}

func TestReadRunSnapshotProjectsProviderTerminalEventAndMarksMissingHistory(t *testing.T) {
	db := openRunTestDB(t)
	_, err := NewAgentRunStore(db).Admit(context.Background(), testAdmission())
	require.NoError(t, err)
	require.NoError(t, db.Exec(`INSERT INTO agent_run_events (tenant_id, run_id, seq, attempt_id, event_type, payload) VALUES (1, 'r1', 4, 'attempt-1', 'execution.succeeded', '{"ok":true}')`).Error)
	snapshot, err := NewAgentRunSnapshotRepository(db).ReadRunSnapshot(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"})
	require.NoError(t, err)
	require.True(t, snapshot.Incomplete)
	require.Equal(t, "succeeded", snapshot.Execution.RunStatus)
	require.Equal(t, "settled", snapshot.Execution.SettlementStatus)
	require.Len(t, snapshot.Events, 1)
}

func TestReadRunSnapshotUsesConfiguredCapabilityResolver(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	_, err := store.Admit(context.Background(), testAdmission())
	require.NoError(t, err)
	snapshot, err := NewAgentRunSnapshotRepositoryWithResolver(db, capabilityResolverStub{}).ReadRunSnapshot(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"})
	require.NoError(t, err)
	require.Equal(t, workbench.CapabilityUnavailable, snapshot.Execution.Capabilities["platform"].State)
	require.Equal(t, "provider_probe_failed", snapshot.Execution.Capabilities["platform"].Reason)
}

func TestReadRunEventsRejectsCursorBeforeRetainedHistory(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	_, err := store.Admit(context.Background(), testAdmission())
	require.NoError(t, err)
	require.NoError(t, db.Exec(`INSERT INTO agent_run_events
		(tenant_id, run_id, seq, attempt_id, event_type, payload)
		VALUES (1, 'r1', 7, 'attempt-1', 'text.delta', '{"text":"retained"}')`).Error)

	_, _, err = NewAgentRunSnapshotRepository(db).ReadRunEvents(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"}, 0, 10)
	require.ErrorIs(t, err, agentruntime.ErrCursorExpired)
}

func TestReadRunEventsPreservesPageAndCursorContinuity(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	_, err := store.Admit(context.Background(), testAdmission())
	require.NoError(t, err)
	for seq := 1; seq <= 257; seq++ {
		require.NoError(t, db.Exec(`INSERT INTO agent_run_events
			(tenant_id, run_id, seq, attempt_id, event_type, payload)
			VALUES (1, 'r1', ?, 'attempt-1', 'text.delta', ?)`, seq, fmt.Sprintf(`{"seq":%d}`, seq)).Error)
	}
	repo := NewAgentRunSnapshotRepository(db)
	first, watermark, err := repo.ReadRunEvents(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"}, 0, 256)
	require.NoError(t, err)
	require.Len(t, first, 256)
	require.Equal(t, int64(257), watermark)
	second, _, err := repo.ReadRunEvents(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"}, first[len(first)-1].Seq, 256)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Equal(t, int64(257), second[0].Seq)
}

func TestReadRunEventsRejectsGapAfterFirstRow(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	_, err := store.Admit(context.Background(), testAdmission())
	require.NoError(t, err)
	for _, seq := range []int{1, 2, 7} {
		require.NoError(t, db.Exec(`INSERT INTO agent_run_events
			(tenant_id, run_id, seq, attempt_id, event_type, payload)
			VALUES (1, 'r1', ?, 'attempt-1', 'text.delta', '{}')`, seq).Error)
	}
	_, _, err = NewAgentRunSnapshotRepository(db).ReadRunEvents(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"}, 0, 256)
	require.ErrorIs(t, err, agentruntime.ErrCursorExpired)
}

func TestReadRunSnapshotConcurrentAppendRemainsConsistent(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	_, err := store.Admit(context.Background(), testAdmission())
	require.NoError(t, err)
	repo := NewAgentRunSnapshotRepository(db)
	var writer sync.WaitGroup
	writer.Add(1)
	go func() {
		defer writer.Done()
		for seq := 1; seq <= 32; seq++ {
			_ = db.Exec(`INSERT INTO agent_run_events
				(tenant_id, run_id, seq, attempt_id, event_type, payload)
				VALUES (1, 'r1', ?, 'attempt-1', 'text.delta', '{}')`, seq).Error
		}
	}()
	for i := 0; i < 32; i++ {
		snapshot, readErr := repo.ReadRunSnapshot(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"})
		if readErr == nil {
			require.NoError(t, snapshot.Validate())
			for index, event := range snapshot.Events {
				require.Equal(t, int64(index+1), event.Seq)
			}
		}
	}
	writer.Wait()
}
