package service

import (
	"context"
	"database/sql"
	"errors"
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

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
)

type semanticBackendFixture struct {
	t     *testing.T
	svc   *BackendService
	db    *gorm.DB
	scope types.SemanticScopeKey
}

func newSemanticBackendFixture(t *testing.T) *semanticBackendFixture {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dbPath := filepath.Join(t.TempDir(), "semantic-backend.db")
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
	repo := apprepo.NewSemanticControlRepository(db)
	svc := NewBackendService(repo)
	return &semanticBackendFixture{t: t, svc: svc, db: db,
		scope: types.SemanticScopeKey{TenantID: 7, KBID: "kb-w03"}}
}

func (f *semanticBackendFixture) SetNativeCheckpoint(revision int64) {
	f.t.Helper()
	require.NoError(f.t, f.db.Exec(
		"INSERT INTO semantic_backend_states (tenant_id, kb_id, native_checkpoint, active_backend) VALUES (?, ?, ?, 'semantic')"+
			" ON CONFLICT (tenant_id, kb_id) DO UPDATE SET native_checkpoint = excluded.native_checkpoint",
		f.scope.TenantID, f.scope.KBID, revision).Error)
}

func (f *semanticBackendFixture) SetLatestDocumentRevision(revision int64) {
	f.t.Helper()
	require.NoError(f.t, f.db.Exec(
		"INSERT INTO semantic_document_revisions (tenant_id, kb_id, document_id, revision) VALUES (?, ?, ?, ?)"+
			" ON CONFLICT (tenant_id, kb_id, document_id) DO UPDATE SET revision = excluded.revision",
		f.scope.TenantID, f.scope.KBID, "d-catchup", revision).Error)
}

func (f *semanticBackendFixture) Rollback() error {
	return f.svc.Rollback(context.Background(), f.scope)
}

func TestSemanticRollbackRequiresNativeCatchup(t *testing.T) {
	f := newSemanticBackendFixture(t)
	f.SetNativeCheckpoint(8)
	f.SetLatestDocumentRevision(9)
	if err := f.Rollback(); !errors.Is(err, ErrNativeCatchupRequired) {
		t.Fatalf("rollback accepted stale native index: %v", err)
	}
}

func TestSemanticSetDesiredKeepsActiveNative(t *testing.T) {
	f := newSemanticBackendFixture(t)
	require.NoError(t, f.svc.SetDesired(context.Background(), f.scope, "semantic"))
	var desired, active string
	f.db.Raw("SELECT desired_backend, active_backend FROM semantic_backend_states WHERE tenant_id = ? AND kb_id = ?",
		f.scope.TenantID, f.scope.KBID).Row().Scan(&desired, &active)
	require.Equal(t, "semantic", desired, "SetDesired records the INTENT")
	require.Equal(t, "native", active, "SetDesired must NOT switch the active backend (shadow build only)")
}

func TestSemanticRollbackSucceedsAfterCatchup(t *testing.T) {
	f := newSemanticBackendFixture(t)
	f.SetNativeCheckpoint(9)
	f.SetLatestDocumentRevision(9)
	require.NoError(t, f.Rollback())
	var active string
	f.db.Raw("SELECT active_backend FROM semantic_backend_states WHERE tenant_id = ? AND kb_id = ?",
		f.scope.TenantID, f.scope.KBID).Row().Scan(&active)
	require.Equal(t, "native", active, "caught-up rollback switches back to native")
}

func TestSemanticConcurrentPromotionSingleWinner(t *testing.T) {
	f := newSemanticBackendFixture(t)
	// Two concurrent promotions with the SAME expected generation: the CAS
	// admits exactly one.
	seed := f.db.Exec(
		"INSERT INTO semantic_backend_states (tenant_id, kb_id, active_backend, active_generation) VALUES (?, ?, 'native', 'g1')",
		f.scope.TenantID, f.scope.KBID)
	require.NoError(t, seed.Error)
	const workers = 4
	errs := make(chan error, workers)
	barrier := make(chan struct{})
	for i := 0; i < workers; i++ {
		go func() {
			<-barrier
			errs <- f.svc.Promote(context.Background(), f.scope, "g1")
		}()
	}
	close(barrier)
	winners := 0
	for i := 0; i < workers; i++ {
		if err := <-errs; err == nil {
			winners++
		}
	}
	require.Equal(t, 1, winners, "exactly one concurrent promotion may win the CAS")
}

func TestSemanticRollbackOnUnpromopedScopeRejected(t *testing.T) {
	f := newSemanticBackendFixture(t)
	// No promotion happened: rollback must refuse with the dedicated
	// sentinel, not a misleading "promotion rejected".
	err := f.Rollback()
	require.ErrorIs(t, err, ErrNotActiveSemantic)
}

func TestSemanticSetDesiredRejectsUnknownBackend(t *testing.T) {
	f := newSemanticBackendFixture(t)
	err := f.svc.SetDesired(context.Background(), f.scope, "magic-backend")
	require.Error(t, err)
}

func TestSemanticTombstonedDocForcesCatchup(t *testing.T) {
	f := newSemanticBackendFixture(t)
	f.SetNativeCheckpoint(3)
	// A DELETED document at revision 9: tombstones count by design (native
	// must replay the deletion for the I04 barrier).
	require.NoError(t, f.db.Exec(
		"INSERT INTO semantic_document_revisions (tenant_id, kb_id, document_id, revision, deleted) VALUES (?, ?, ?, 9, 1)",
		f.scope.TenantID, f.scope.KBID, "d-tomb").Error)
	err := f.Rollback()
	require.ErrorIs(t, err, ErrNativeCatchupRequired, "tombstones must force native catch-up")
}

func TestSemanticPromoteRequiresExpectedGeneration(t *testing.T) {
	f := newSemanticBackendFixture(t)
	err := f.svc.Promote(context.Background(), f.scope, "stale-gen")
	require.Error(t, err, "promotion with a stale expected generation must fail")
}
