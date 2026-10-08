package repository

// T16 (#134): the Workspace writer lease over the REAL store and BOTH
// dialects — N concurrent writers race the unique lease row and exactly one
// acquires; every loser receives a stable conflict naming the winner; the
// holder's retry is idempotent. (The full journey matrix — read-only
// isolation, unknown-outcome retention, recovery, versions — lives at the
// service seam in craft_workspace_lease_t16_test.go.)
import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestCraftWorkspaceWriterLeaseConcurrentCAS(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var db *gorm.DB
			if dialect == "postgres" {
				db = openCraftDB(t)
			} else {
				db = openCraftLeaseRaceDB(t)
			}
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
			// forbidden, another session does not exist ([T08] 404
			// discipline — same shape as lockCraftWriterWorkspace).
			_, err = store.AcquireWriterLease(ctx, craft.Scope{TenantID: 1, UserID: "u2", SessionID: scope.SessionID}, ws.ID, "r-lease-x")
			require.ErrorIs(t, err, craft.ErrForbidden)
			_, err = store.AcquireWriterLease(ctx, craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s2"}, ws.ID, "r-lease-x")
			require.ErrorIs(t, err, craft.ErrNotFound)
		})
	}
}

// openCraftLeaseRaceDB opens the craft SQLite store with IMMEDIATE write
// transactions for the concurrent-CAS race above. Under a deferred BEGIN,
// every racing goroutine first takes a SHARED lock (the workspace row read)
// and then tries to upgrade to RESERVED for the lease write — SQLite can
// only break that mutual upgrade wait by failing one side with SQLITE_BUSY,
// which no busy_timeout resolves deterministically under scheduler load.
// With _txlock=immediate the transactions queue on the busy timeout and the
// lease CAS itself — the actual subject under test — decides the winner
// (same fixture shape as native_pending/appconnector worker tests).
func openCraftLeaseRaceDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dbPath := filepath.Join(t.TempDir(), "craft-lease-race.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000&_txlock=immediate"

	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver,
	)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	seedRunFixtures(t, db)
	t.Cleanup(func() {
		if conn, e := db.DB(); e == nil {
			_ = conn.Close()
		}
	})
	return db
}
