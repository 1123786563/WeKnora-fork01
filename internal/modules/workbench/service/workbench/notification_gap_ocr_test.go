package workbench

// T18 (#137) OCR regression: a run whose retained window carries a hole in
// front of the notification checkpoint must not wedge the lexicographic run
// page — the projector recovers an expired cursor by resuming from the
// retained window head (losing exactly the trimmed events, the pre-gap-rule
// outcome) and every later run keeps projecting on the same tick.
import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/stretchr/testify/require"
)

func TestNotificationWorkerRecoversFromCursorHoleWithoutStarvingLaterRuns(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	ctx := context.Background()
	seedGapWorkerRun := func(runID string, seqs ...int64) {
		t.Helper()
		require.NoError(t, db.Exec(`INSERT INTO agent_runs
			(tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, deadline)
			VALUES (1, ?, 's1', 'u1', ?, ?, 'hash', '{}', ?)`,
			runID, "request-"+runID, "assistant-"+runID, time.Now().Add(time.Hour)).Error)
		for _, seq := range seqs {
			require.NoError(t, db.Exec(`INSERT INTO agent_run_events
				(tenant_id, run_id, seq, event_type, payload) VALUES (1, ?, ?, 'run_completed', '{}')`, runID, seq).Error)
		}
	}

	// "aaa-hole" carries the hole and sorts BEFORE "zzz-healthy": without
	// recovery, the worker would fail on it every tick and the healthy run's
	// projection would starve forever.
	seedGapWorkerRun("aaa-hole", 1, 2, 7, 8)
	seedGapWorkerRun("zzz-healthy", 1)
	require.NoError(t, db.Exec(`INSERT INTO mobile_devices
		(tenant_id, owner_id, device_id, environment, platform, token_ciphertext, token_hash)
		VALUES (1, 'u1', 'gap-device', 'dev', 'ios', 'cipher', 'hash-gap')`).Error)

	runs := repository.NewAgentRunStore(db)
	store := repository.NewNotificationStore(db)
	worker := NewNotificationWorker(NewNotificationProjector(runs, store), store)

	// First pass: the hole run's page [1,2,7,8] from cursor 0 hits the gap
	// at 3 — the recovery projects the CONTIGUOUS PREFIX [1,2] and walks the
	// checkpoint to the hole's edge (strictly forward progress, no wedge).
	require.NoError(t, worker.RunOnce(ctx), "a hole in front of the checkpoint must recover, not wedge the page")
	cursor, err := store.LoadCheckpoint(ctx, notificationConsumerName, agentruntime.RunKey{TenantID: 1, RunID: "aaa-hole"})
	require.NoError(t, err)
	require.EqualValues(t, 2, cursor, "the first pass projects the contiguous prefix and stops at the hole's edge")

	// The lexicographically later healthy run was NOT starved on that pass.
	cursor, err = store.LoadCheckpoint(ctx, notificationConsumerName, agentruntime.RunKey{TenantID: 1, RunID: "zzz-healthy"})
	require.NoError(t, err)
	require.EqualValues(t, 1, cursor, "runs after the hole-carrying one keep projecting on the same tick")

	// Second pass: the cursor now stands at the hole's edge — the same page
	// starts with the gap, so the recovery JUMPS to the next contiguous
	// segment and projects the tail [7,8] to the end of the window.
	require.NoError(t, worker.RunOnce(ctx))
	cursor, err = store.LoadCheckpoint(ctx, notificationConsumerName, agentruntime.RunKey{TenantID: 1, RunID: "aaa-hole"})
	require.NoError(t, err)
	require.EqualValues(t, 8, cursor, "the second pass jumps the hole and settles at the last retained event")

	// Every retained run_completed event produced exactly one intent: the
	// prefix 1,2, the tail 7,8 and the healthy run's 1 — the hole (3..6
	// never existed) is the only loss.
	var intents int64
	require.NoError(t, db.Table("mobile_notification_intents").Count(&intents).Error)
	require.EqualValues(t, 5, intents, fmt.Sprintf("one intent per projected run_completed event (1,2,7,8 + healthy 1), got %d", intents))
}
