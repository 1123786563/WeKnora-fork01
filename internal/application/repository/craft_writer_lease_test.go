package repository

// T16 (#134): the Workspace writer lease over the REAL store and BOTH
// dialects — N concurrent writers race the unique lease row and exactly one
// acquires; every loser receives a stable conflict naming the winner; the
// holder's retry is idempotent. (The full journey matrix — read-only
// isolation, unknown-outcome retention, recovery, versions — lives at the
// service seam in craft_workspace_lease_t16_test.go.)
import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
)

func TestCraftWorkspaceWriterLeaseConcurrentCAS(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			store, ok := NewCraftStore(db).(*CraftStore)
			require.True(t, ok, "the lease methods live on the concrete store")
			ctx := context.Background()
			scope := craftTestScope()
			ws := putCraftWorkspace(t, store)

			const writers = 4
			runs := make([]string, writers)
			for i := range runs {
				runs[i] = "r-lease-" + string(rune('a'+i))
			}
			// The first writer arrives through the real admission API; the
			// racing writers are durable rows the slot cannot see yet.
			seedCraftRun(t, db, runs[0], "c-lease-a")
			for _, runID := range runs[1:] {
				require.NoError(t, db.Exec(
					`INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id,
					   assistant_message_id, request_hash, status, snapshot, deadline)
					 VALUES (1, ?, 's1', 'u1', ?, ?, ?, 'queued', ?, ?)`,
					runID, "craft-"+runID, "craft-a-"+runID, "craft-h-"+runID,
					`{"version":1,"craft":true}`, time.Now().Add(time.Hour).UTC(),
				).Error)
			}

			acquisitions := make([]craft.WriterAcquisition, writers)
			errs := make([]error, writers)
			var wg sync.WaitGroup
			for i := range runs {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					acquisitions[i], errs[i] = store.AcquireWriterLease(ctx, scope, ws.ID, runs[i])
				}(i)
			}
			wg.Wait()

			winners, conflicts := 0, 0
			winnerRun := ""
			for i, acq := range acquisitions {
				require.NoError(t, errs[i], "a lease race never fails; it answers acquired or conflict")
				switch acq.Outcome.Status {
				case craft.WriterAcquired:
					winners++
					require.NotNil(t, acq.Lease)
					require.Equal(t, ws.ID, acq.Lease.WorkspaceID)
					require.Equal(t, scope.SessionID, acq.Lease.TaskID)
					require.Equal(t, runs[i], acq.Lease.RunID)
					require.Zero(t, acq.Lease.Revision, "the fresh draft head fences revision 0")
					winnerRun = runs[i]
				case craft.WriterConflict:
					conflicts++
					require.NotNil(t, acq.Holder)
				default:
					t.Fatalf("unexpected acquisition status %q", acq.Outcome.Status)
				}
			}
			require.Equal(t, 1, winners, "exactly one concurrent writer acquires")
			require.Equal(t, writers-1, conflicts)

			var rows int64
			require.NoError(t, db.Table("craft_workspace_writer_leases").
				Where("workspace_id = ?", ws.ID).Count(&rows).Error)
			require.EqualValues(t, 1, rows, "the lease is exactly one durable row")

			// Every loser retries into the SAME stable conflict; the winner's
			// retry is idempotent.
			for i, runID := range runs {
				if runID == winnerRun {
					continue
				}
				retry, err := store.AcquireWriterLease(ctx, scope, ws.ID, runID)
				require.NoError(t, err)
				require.Equal(t, craft.WriterConflict, retry.Outcome.Status)
				require.Equal(t, winnerRun, retry.Holder.RunID, "loser %d conflicts with a stable holder", i)
			}
			again, err := store.AcquireWriterLease(ctx, scope, ws.ID, winnerRun)
			require.NoError(t, err)
			require.Equal(t, craft.WriterAcquired, again.Outcome.Status)
			require.Equal(t, winnerRun, again.Lease.RunID)

			// A foreign scope never reaches the lease: another user is
			// forbidden, another session does not exist.
			_, err = store.AcquireWriterLease(ctx, craft.Scope{TenantID: 1, UserID: "u2", SessionID: scope.SessionID}, ws.ID, "r-lease-x")
			require.ErrorIs(t, err, craft.ErrForbidden)
			_, err = store.AcquireWriterLease(ctx, craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s2"}, ws.ID, "r-lease-x")
			require.ErrorIs(t, err, craft.ErrForbidden)
		})
	}
}
