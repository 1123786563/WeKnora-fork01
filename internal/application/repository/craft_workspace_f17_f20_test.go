package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func craftPrepareFixture(t *testing.T, runID, callID string) (craft.Store, craft.Task, *gorm.DB) {
	t.Helper()
	db := openCraftDB(t)
	store := NewCraftStore(db)
	fence := seedCraftRun(t, db, runID, callID)
	ws := putCraftWorkspace(t, store)
	task := craft.Task{
		ToolCallID: callID, Prompt: "build a page", RequestHash: "request-" + callID,
		Scope: craftTestScope(), Fence: fence, WorkspaceID: ws.ID,
		SnapshotDigestVersion: fence.SnapshotDigestVersion, SnapshotDigest: fence.SnapshotDigest,
		Deadline: time.Now().Add(time.Minute).UTC().Truncate(time.Microsecond),
	}
	return store, task, db
}

func TestCraftPrepareTaskStopIntentFencesOnlyItsRun(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		for _, status := range []craft.StopOutcomeStatus{craft.StopRequested, craft.StopUnknown, craft.StopConfirmed} {
			t.Run(dialect+"/"+string(status), func(t *testing.T) {
				store, task, db := craftPrepareFixture(t, "r-stop-fence", "c-stop-fence")
				prior, err := store.PrepareTask(context.Background(), task)
				require.NoError(t, err)
				_, err = NewCraftStopIntentStore(db).PutStopIntent(context.Background(), task.Scope,
					craft.StopIntent{RunID: task.Fence.RunID, Status: status})
				require.NoError(t, err)

				replayed, err := store.PrepareTask(context.Background(), task)
				require.NoError(t, err, "an already prepared delegation remains available for idempotent reuse")
				require.Equal(t, prior.ID, replayed.ID)

				_, err = NewAgentRunStore(db).EnsureToolPlan(context.Background(), task.Fence, craftToolPlan("c-stop-fresh"))
				require.NoError(t, err)
				fresh := task
				fresh.ToolCallID, fresh.RequestHash = "c-stop-fresh", "request-c-stop-fresh"
				_, err = store.PrepareTask(context.Background(), fresh)
				require.ErrorIs(t, err, craft.ErrConflict, "a durable stop intent refuses fresh preparation for the same Run")
				var count int64
				require.NoError(t, db.Table("craft_delegations").Where("tenant_id = ? AND run_id = ?", task.Fence.TenantID, task.Fence.RunID).Count(&count).Error)
				require.EqualValues(t, 1, count, "refused fresh preparation must not insert a second delegation")
			})
		}
	}
}

func TestCraftPrepareTaskStopIntentStorageFailureFailsClosed(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			store, task, db := craftPrepareFixture(t, "r-stop-store-failure", "c-stop-store-failure")
			require.NoError(t, db.Migrator().DropTable(&craftStopIntentRow{}))

			_, err := store.PrepareTask(context.Background(), task)
			require.Error(t, err, "only typed ErrNotFound may mean no stop intent")
			var count int64
			require.NoError(t, db.Table("craft_delegations").Where("tenant_id = ? AND run_id = ?", task.Fence.TenantID, task.Fence.RunID).Count(&count).Error)
			require.Zero(t, count)
		})
	}
}

func TestCraftPrepareTaskSameSessionDifferentRunAfterPriorStop(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			firstStore, first, db := craftPrepareFixture(t, "r-stop-first", "c-stop-first")
			require.NoError(t, NewAgentRunStore(db).CancelRun(context.Background(), first.Fence.RunKey, "member_stop"))
			_, err := NewCraftStopIntentStore(db).PutStopIntent(context.Background(), first.Scope,
				craft.StopIntent{RunID: first.Fence.RunID, Status: craft.StopConfirmed})
			require.NoError(t, err)

			// A subsequent Run in the same Task has its own Run identity and is not
			// fenced by the earlier Run's tombstoned stop intent.
			require.NoError(t, db.Exec("UPDATE sessions SET active_agent_run_id = NULL WHERE tenant_id = ? AND id = ?", 1, "s1").Error)
			secondFence := seedCraftRun(t, db, "r-stop-second", "c-stop-second")
			ws, err := firstStore.GetWorkspace(context.Background(), first.Scope)
			require.NoError(t, err)
			second := craft.Task{
				ToolCallID: "c-stop-second", Prompt: "continue page", RequestHash: "request-c-stop-second",
				Scope: first.Scope, Fence: secondFence, WorkspaceID: ws.ID,
				SnapshotDigestVersion: secondFence.SnapshotDigestVersion, SnapshotDigest: secondFence.SnapshotDigest,
				Deadline: time.Now().Add(time.Minute).UTC().Truncate(time.Microsecond),
			}
			_, err = firstStore.PrepareTask(context.Background(), second)
			require.NoError(t, err)
		})
	}
}

func TestCraftStopIntentAndPrepareTaskSerializeOnRun(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			store, task, db := craftPrepareFixture(t, "r-stop-order", "c-stop-order")
			_, err := NewAgentRunStore(db).EnsureToolPlan(context.Background(), task.Fence, craftToolPlan("c-after-stop"))
			require.NoError(t, err)

			locked := make(chan struct{})
			release := make(chan struct{})
			var once sync.Once
			err = db.Callback().Update().After("gorm:after_update").Register("test:hold_run_lock", func(tx *gorm.DB) {
				if tx.Statement.Table == "agent_runs" {
					once.Do(func() {
						close(locked)
						<-release
					})
				}
			})
			require.NoError(t, err)

			prepareDone := make(chan error, 1)
			go func() {
				_, err := store.PrepareTask(context.Background(), task)
				prepareDone <- err
			}()
			select {
			case <-locked: // PrepareTask has updated/locked the Run row and is still in its transaction.
			case <-time.After(5 * time.Second):
				close(release)
				require.FailNow(t, "PrepareTask did not reach its Run-row lock")
			}

			stopDone := make(chan error, 1)
			go func() {
				_, err := NewCraftStopIntentStore(db).PutStopIntent(context.Background(), task.Scope,
					craft.StopIntent{RunID: task.Fence.RunID, Status: craft.StopRequested})
				stopDone <- err
			}()
			select {
			case err := <-stopDone:
				close(release)
				require.NoError(t, err)
				<-prepareDone
				require.FailNow(t, "StopIntent committed while PrepareTask still owned the Run lock")
			case <-time.After(100 * time.Millisecond):
			}
			close(release)
			require.NoError(t, <-prepareDone, "the preparation that acquired the Run lock first may commit")
			require.NoError(t, <-stopDone, "the queued StopIntent commits after the earlier preparation")

			second := task
			second.ToolCallID = "c-after-stop"
			second.RequestHash = "request-c-after-stop"
			_, err = store.PrepareTask(context.Background(), second)
			require.ErrorIs(t, err, craft.ErrConflict, "any fresh preparation after accepted Stop must refuse")
		})
	}
}
