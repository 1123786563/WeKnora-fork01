package service

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// semanticTaskFixture wires the REAL coordinator over a migrated SQLite
// store so late/duplicate terminal responses are exercised against real
// transactional state (I05 plan step 1).
type semanticTaskFixture struct {
	t  *testing.T
	co *SemanticTaskCoordinator
	db *gorm.DB
}

func newSemanticTaskFixture(t *testing.T) *semanticTaskFixture {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dbPath := filepath.Join(t.TempDir(), "semantic-tasks.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver,
	)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		conn, e := db.DB()
		if e == nil {
			_ = conn.Close()
		}
	})
	return &semanticTaskFixture{t: t, co: NewSemanticTaskCoordinator(db), db: db}
}

const (
	taskTenant uint64 = 7
	taskKB            = "kb-i05"
	taskDoc           = "doc-i05"
)

// StartAttempt simulates the business side starting attempt N with one
// pending semantic operation to complete.
func (f *semanticTaskFixture) StartAttempt(attempt uint64) {
	f.t.Helper()
	require.NoError(f.t, f.co.Submit(context.Background(), taskTenant, taskKB, taskDoc, attempt, attemptKey(attempt)))
}

func attemptKey(attempt uint64) string {
	switch attempt {
	case 1:
		return "old-operation"
	case 2:
		return "new-operation"
	default:
		return "op-other"
	}
}

// DeliverTerminal feeds a terminal RPC notification for (operation, attempt).
func (f *semanticTaskFixture) DeliverTerminal(operationID string, attempt uint64, state string) {
	f.t.Helper()
	require.NoError(f.t, f.co.Complete(context.Background(), taskTenant, taskKB, taskDoc, attempt, operationID, state))
}

func (f *semanticTaskFixture) PendingCount(attempt uint64) int64 {
	f.t.Helper()
	var pending int64
	require.NoError(f.t, f.db.Raw(
		"SELECT pending FROM semantic_attempt_counters WHERE tenant_id = ? AND kb_id = ? AND document_id = ? AND attempt = ?",
		taskTenant, taskKB, taskDoc, attempt).Scan(&pending).Error)
	return pending
}

func TestSemanticCompletionCannotDrainNewAttempt(t *testing.T) {
	f := newSemanticTaskFixture(t)
	f.StartAttempt(2)
	f.DeliverTerminal("old-operation", 1, "succeeded")
	if f.PendingCount(2) != 1 {
		t.Fatal("old attempt drained current counter")
	}
	f.DeliverTerminal("new-operation", 2, "succeeded")
	f.DeliverTerminal("new-operation", 2, "succeeded")
	if f.PendingCount(2) != 0 {
		t.Fatal("completion was not idempotent")
	}
}

func TestSemanticReceiptsAreAttemptNamespaced(t *testing.T) {
	f := newSemanticTaskFixture(t)
	// Seed a COLLIDING I02-namespace receipt: same revision==attempt AND the
	// same task_key as our operation id - pre-fix this suppresses Complete's
	// insert (ON CONFLICT) and permanently wedges the attempt counter.
	require.NoError(t, f.db.Exec(
		"INSERT INTO semantic_completion_receipts (tenant_id, kb_id, document_id, revision, task_key) VALUES (?, ?, ?, ?, ?)",
		taskTenant, taskKB, taskDoc, 3, attemptKey(3)).Error)
	// Attempt 3 completion must NOT collide with nor be confused by it.
	f.StartAttempt(3)
	f.DeliverTerminal(attemptKey(3), 3, "succeeded")
	require.Equal(t, int64(0), f.PendingCount(3), "attempt completion must drain its own counter despite I02 rows")
	// Reconcile for attempt 3 must find the ATTEMPT receipt, never biz-write.
	require.NoError(t, f.co.Reconcile(context.Background(), taskTenant, taskKB, taskDoc, 3))
	var state string
	f.db.Raw("SELECT state FROM semantic_task_operations WHERE attempt = 3").Scan(&state)
	require.Equal(t, "succeeded", state, "reconcile must read only attempt-namespaced receipts")
	// The seeded I02 row must remain untouched AND the attempt receipt
	// must coexist under its own namespace.
	var seeded, namespaced int64
	f.db.Raw("SELECT COUNT(*) FROM semantic_completion_receipts WHERE task_key = ?", attemptKey(3)).Scan(&seeded)
	require.Equal(t, int64(1), seeded, "the seeded pre-I05 row must survive")
	f.db.Raw("SELECT COUNT(*) FROM semantic_completion_receipts WHERE attempt = 3 AND operation_id = ?", attemptKey(3)).Scan(&namespaced)
	require.Equal(t, int64(1), namespaced, "the attempt-namespaced receipt must exist")
}

func TestSemanticConcurrentSubmitIsIdempotent(t *testing.T) {
	f := newSemanticTaskFixture(t)
	const workers = 8
	barrier := make(chan struct{})
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func() {
			<-barrier
			errs <- f.co.Submit(context.Background(), taskTenant, taskKB, taskDoc, 6, "op-race")
		}()
	}
	close(barrier)
	for i := 0; i < workers; i++ {
		require.NoError(t, <-errs, "concurrent same-key submits must be idempotent, never an error")
	}
	var rows int64
	f.db.Raw("SELECT COUNT(*) FROM semantic_task_operations WHERE attempt = 6").Scan(&rows)
	require.Equal(t, int64(1), rows)
	require.Equal(t, int64(1), f.PendingCount(6))
}

func TestSemanticSubmitIsIdempotentPerKey(t *testing.T) {
	f := newSemanticTaskFixture(t)
	// Same attempt + same idempotency key resubmitted: one operation, one pending.
	require.NoError(t, f.co.Submit(context.Background(), taskTenant, taskKB, taskDoc, 3, "op-dedup"))
	require.NoError(t, f.co.Submit(context.Background(), taskTenant, taskKB, taskDoc, 3, "op-dedup"))
	var rows int64
	f.db.Raw("SELECT COUNT(*) FROM semantic_task_operations WHERE attempt = 3").Scan(&rows)
	require.Equal(t, int64(1), rows)
	require.Equal(t, int64(1), f.PendingCount(3))
}

func TestSemanticCancelDoesNotDrainAnyAttempt(t *testing.T) {
	f := newSemanticTaskFixture(t)
	f.StartAttempt(4)
	require.NoError(t, f.co.Cancel(context.Background(), taskTenant, taskKB, taskDoc, 4, attemptKey(4)))
	// The cancelled operation must not complete the counter by itself.
	require.Equal(t, int64(1), f.PendingCount(4))
	f.DeliverTerminal(attemptKey(4), 4, "cancelled")
	require.Equal(t, int64(1), f.PendingCount(4), "cancelled terminal must not drain the pending counter")
}

func TestSemanticReconcileRecoversState(t *testing.T) {
	f := newSemanticTaskFixture(t)
	f.StartAttempt(5)
	// A terminal that arrived while the row was still pending: reconcile
	// re-applies the mapping state.
	f.DeliverTerminal(attemptKey(5), 5, "succeeded")
	// Simulate the operation row losing its terminal state (lost write);
	// reconcile must RECOVER it from the durable receipt.
	require.NoError(t, f.db.Exec(
		"UPDATE semantic_task_operations SET state = 'pending' WHERE attempt = 5").Error)
	require.NoError(t, f.co.Reconcile(context.Background(), taskTenant, taskKB, taskDoc, 5))
	var state string
	f.db.Raw("SELECT state FROM semantic_task_operations WHERE attempt = 5").Scan(&state)
	require.Equal(t, "succeeded", state)
}
