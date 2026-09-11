package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newSemanticTestDB opens an isolated SQLite database and runs the REAL
// business migrations (migrations/sqlite) - no in-memory shortcuts.
func newSemanticTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))

	dbPath := filepath.Join(t.TempDir(), "semantic-control.db")
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
	return db
}

func mutationFixture() SemanticMutation {
	return SemanticMutation{
		TenantID:         1,
		KBID:             "kb-1",
		DocumentID:       "doc-1",
		ExpectedRevision: 0,
		Deleted:          false,
		Payload:          []byte(`{"chunks":["c1"]}`),
	}
}

func outboxCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Table("semantic_outbox").Count(&count).Error)
	return count
}

func TestSemanticMutationRollback(t *testing.T) {
	db := newSemanticTestDB(t) // 本测试文件定义：隔离SQLite库并执行正式迁移
	repo := NewSemanticControlRepository(db)
	_, err := repo.WithSemanticMutation(context.Background(), mutationFixture(),
		func(tx *gorm.DB) error { return errors.New("abort resource update") })
	if err == nil {
		t.Fatal("expected rollback")
	}
	var count int64
	db.Table("semantic_outbox").Count(&count)
	if count != 0 {
		t.Fatal("event escaped resource transaction")
	}
	// The revision must roll back too: a retried mutation with the same
	// expected revision still succeeds afterwards.
	revision, err := repo.WithSemanticMutation(context.Background(), mutationFixture(),
		func(tx *gorm.DB) error { return nil })
	require.NoError(t, err)
	require.Equal(t, uint64(1), revision)
}

func TestSemanticMutationCommitsAtomically(t *testing.T) {
	db := newSemanticTestDB(t)
	repo := NewSemanticControlRepository(db)
	revision, err := repo.WithSemanticMutation(context.Background(), mutationFixture(),
		func(tx *gorm.DB) error {
			return tx.Exec("INSERT INTO semantic_completion_receipts (tenant_id, kb_id, document_id, revision, task_key) VALUES (1, 'kb-1', 'doc-1', 0, 'biz-write')").Error
		})
	require.NoError(t, err)
	require.Equal(t, uint64(1), revision)
	require.Equal(t, int64(1), outboxCount(t, db))
	var receipt int64
	db.Table("semantic_completion_receipts").Where("task_key = ?", "biz-write").Count(&receipt)
	require.Equal(t, int64(1), receipt)
}

func TestRevisionCASRejectsStaleExpected(t *testing.T) {
	db := newSemanticTestDB(t)
	repo := NewSemanticControlRepository(db)

	first, err := repo.WithSemanticMutation(context.Background(), mutationFixture(), func(tx *gorm.DB) error { return nil })
	require.NoError(t, err)
	require.Equal(t, uint64(1), first)

	stale := mutationFixture() // still expects revision 0
	_, err = repo.WithSemanticMutation(context.Background(), stale, func(tx *gorm.DB) error { return nil })
	require.ErrorIs(t, err, ErrSemanticRevisionConflict)
	require.Equal(t, int64(1), outboxCount(t, db), "conflict must not append events")

	wrong := mutationFixture()
	wrong.ExpectedRevision = 99
	_, err = repo.WithSemanticMutation(context.Background(), wrong, func(tx *gorm.DB) error { return nil })
	require.ErrorIs(t, err, ErrSemanticRevisionConflict)

	second, err := repo.WithSemanticMutation(context.Background(), func() SemanticMutation {
		m := mutationFixture()
		m.ExpectedRevision = 1
		return m
	}(), func(tx *gorm.DB) error { return nil })
	require.NoError(t, err)
	require.Equal(t, uint64(2), second)
}

func TestDeleteCreatesDenialAndBumpsEpoch(t *testing.T) {
	db := newSemanticTestDB(t)
	repo := NewSemanticControlRepository(db)
	_, err := repo.WithSemanticMutation(context.Background(), mutationFixture(), func(tx *gorm.DB) error { return nil })
	require.NoError(t, err)

	del := mutationFixture()
	del.ExpectedRevision = 1
	del.Deleted = true
	revision, err := repo.WithSemanticMutation(context.Background(), del, func(tx *gorm.DB) error { return nil })
	require.NoError(t, err)
	require.Equal(t, uint64(2), revision)

	var denials int64
	db.Table("semantic_denials").Where("tenant_id = ? AND kb_id = ? AND document_id = ?", 1, "kb-1", "doc-1").Count(&denials)
	require.Equal(t, int64(1), denials, "deletion barrier row required")
	var epoch int64
	require.NoError(t, db.Table("semantic_access_epochs").
		Where("tenant_id = ? AND kb_id = ?", 1, "kb-1").
		Select("epoch").Scan(&epoch).Error)
	require.Equal(t, int64(1), epoch, "KB access epoch must bump on delete")
}

func TestNonDeleteDoesNotBumpEpoch(t *testing.T) {
	db := newSemanticTestDB(t)
	repo := NewSemanticControlRepository(db)
	_, err := repo.WithSemanticMutation(context.Background(), mutationFixture(), func(tx *gorm.DB) error { return nil })
	require.NoError(t, err)
	var count int64
	db.Table("semantic_access_epochs").Count(&count)
	require.Equal(t, int64(0), count, "plain updates must not change authorization epochs")
}

func TestBumpSemanticEpochForA01(t *testing.T) {
	db := newSemanticTestDB(t)
	repo := NewSemanticControlRepository(db)
	require.NoError(t, repo.BumpSemanticEpoch(1, "kb-1"))
	require.NoError(t, repo.BumpSemanticEpoch(1, "kb-1"))
	var epoch int64
	require.NoError(t, db.Table("semantic_access_epochs").
		Where("tenant_id = ? AND kb_id = ?", 1, "kb-1").
		Select("epoch").Scan(&epoch).Error)
	require.Equal(t, int64(2), epoch)
}

func TestOutboxEventCarriesRevisionAndHash(t *testing.T) {
	db := newSemanticTestDB(t)
	repo := NewSemanticControlRepository(db)
	_, err := repo.WithSemanticMutation(context.Background(), mutationFixture(), func(tx *gorm.DB) error { return nil })
	require.NoError(t, err)

	var event struct {
		EventID     string
		Revision    int64
		PayloadHash string
		ConfirmedAt *time.Time
	}
	require.NoError(t, db.Table("semantic_outbox").
		Select("event_id, revision, payload_hash, confirmed_at").Scan(&event).Error)
	require.Equal(t, int64(1), event.Revision)
	require.NotEmpty(t, event.PayloadHash)
	require.Nil(t, event.ConfirmedAt, "events start unconfirmed")
}

func TestOutboxClaimConfirmAndBackoff(t *testing.T) {
	db := newSemanticTestDB(t)
	repo := NewSemanticControlRepository(db)
	_, err := repo.WithSemanticMutation(context.Background(), mutationFixture(), func(tx *gorm.DB) error { return nil })
	require.NoError(t, err)

	events, err := repo.ClaimOutboxEvents(context.Background(), 10, 200*time.Millisecond)
	require.NoError(t, err)
	require.Len(t, events, 1)
	first := events[0]

	// The claim lease holds the event against immediate re-claims.
	held, err := repo.ClaimOutboxEvents(context.Background(), 10, 200*time.Millisecond)
	require.NoError(t, err)
	require.Empty(t, held, "claim lease must hold the event")

	// After the lease expires (crashed dispatcher) the event returns.
	time.Sleep(300 * time.Millisecond)
	reclaimed, err := repo.ClaimOutboxEvents(context.Background(), 10, 200*time.Millisecond)
	require.NoError(t, err)
	require.Len(t, reclaimed, 1)
	require.Equal(t, first.EventID, reclaimed[0].EventID)

	// Failure backs off: the event must not be claimable until the backoff.
	require.NoError(t, repo.FailOutboxEvent(context.Background(), first.EventID, time.Minute))
	cooldown, err := repo.ClaimOutboxEvents(context.Background(), 10, 200*time.Millisecond)
	require.NoError(t, err)
	require.Empty(t, cooldown, "backed-off events are not claimable")

	// Confirmation is terminal: only after durable acceptance.
	require.NoError(t, repo.ConfirmOutboxEvent(context.Background(), first.EventID))
	final, err := repo.ClaimOutboxEvents(context.Background(), 10, 200*time.Millisecond)
	require.NoError(t, err)
	require.Empty(t, final, "confirmed events never return")
	require.NoError(t, repo.FailOutboxEvent(context.Background(), first.EventID, time.Minute), "confirm is idempotent vs fail")
}

func TestConcurrentMutationsSingleWinner(t *testing.T) {
	db := newSemanticTestDB(t)
	repo := NewSemanticControlRepository(db)

	wins := make(chan uint64, 2)
	failures := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			revision, err := repo.WithSemanticMutation(context.Background(), mutationFixture(),
				func(tx *gorm.DB) error { return nil })
			if err != nil {
				failures <- err
				return
			}
			wins <- revision
		}()
	}
	var winCount, failCount int
	for winCount+failCount < 2 {
		select {
		case <-wins:
			winCount++
		case <-failures:
			failCount++
		case <-time.After(10 * time.Second):
			t.Fatal("mutations did not settle")
		}
	}
	require.Equal(t, 1, winCount, "exactly one concurrent mutation may win the revision CAS")
	require.Equal(t, 1, failCount)
	require.Equal(t, int64(1), outboxCount(t, db))
}

func TestSemanticMutationEventIdentityUnique(t *testing.T) {
	db := newSemanticTestDB(t)
	repo := NewSemanticControlRepository(db)
	first, err := repo.WithSemanticMutation(context.Background(), mutationFixture(), func(tx *gorm.DB) error { return nil })
	require.NoError(t, err)
	eventID := fmt.Sprintf("sem-ev:%d:%s:%s:%d", 1, "kb-1", "doc-1", first)
	// The unique key is the last line of defense: replaying the same
	// document-revision identity under a different event id must be rejected.
	dup := db.Exec(
		"INSERT INTO semantic_outbox (tenant_id, kb_id, document_id, revision, event_id, payload_hash) VALUES (1, 'kb-1', 'doc-1', ?, ?, 'other-hash')",
		first, eventID+"-x",
	)
	require.Error(t, dup.Error, "unique key must reject a second event for the same document revision")
}

func TestBumpSemanticEpochTxRollsBackWithCaller(t *testing.T) {
	db := newSemanticTestDB(t)
	repo := NewSemanticControlRepository(db)
	_, err := repo.WithSemanticMutation(context.Background(), mutationFixture(),
		func(tx *gorm.DB) error {
			// A01 pattern: bump INSIDE the permission-change transaction.
			if err := repo.BumpSemanticEpochTx(tx, 1, "kb-1"); err != nil {
				return err
			}
			return errors.New("permission write aborted")
		})
	require.Error(t, err)
	var count int64
	db.Table("semantic_access_epochs").Count(&count)
	require.Zero(t, count, "epoch bump must roll back with the caller's transaction")
}

func TestConcurrentFirstBumpsBothSucceed(t *testing.T) {
	db := newSemanticTestDB(t)
	repo := NewSemanticControlRepository(db)
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			// Concurrent FIRST bumps of the same KB: the upsert must let
			// both succeed instead of racing UPDATE-then-INSERT.
			done <- repo.BumpSemanticEpoch(1, "kb-1")
		}()
	}
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			require.NoError(t, err, "concurrent first epoch bumps must both commit")
		case <-time.After(10 * time.Second):
			t.Fatal("first bumps did not settle")
		}
	}
	var epoch int64
	require.NoError(t, db.Table("semantic_access_epochs").
		Where("tenant_id = ? AND kb_id = ?", 1, "kb-1").
		Select("epoch").Scan(&epoch).Error)
	require.Equal(t, int64(2), epoch, "both bumps must be counted")
}
