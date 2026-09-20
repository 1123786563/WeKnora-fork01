package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"math"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var fixtureScope = types.SemanticScopeKey{TenantID: 1, KBID: "kb-1"}
var noBusinessWrite = func(*gorm.DB) error { return nil }

func TestSemanticMutationRollback(t *testing.T) {
	ctx, db := context.Background(), newSemanticSQLiteTestDB(t)
	require.NoError(t, db.Exec("CREATE TABLE semantic_test_business (id TEXT PRIMARY KEY)").Error)
	repo := NewSemanticControlRepository(db)
	_, err := repo.WithSemanticMutation(ctx, mutationFixture(0, true, nil), func(tx *gorm.DB) error {
		require.NoError(t, tx.Exec("INSERT INTO semantic_test_business(id) VALUES (?)", "resource-1").Error)
		return errors.New("abort business mutation")
	})
	require.Error(t, err)
	assertRowCount(t, db, "semantic_test_business", 0)
	assertRowCount(t, db, "semantic_document_revisions", 0)
	assertRowCount(t, db, "semantic_outbox", 0)
	assertRowCount(t, db, "semantic_denials", 0)
	assertRowCount(t, db, "semantic_access_epochs", 0)
}

func TestSemanticMutationCommitsBusinessRevisionAndOutboxTogether(t *testing.T) {
	ctx, db := context.Background(), newSemanticSQLiteTestDB(t)
	require.NoError(t, db.Exec("CREATE TABLE semantic_test_business (id TEXT PRIMARY KEY)").Error)
	revision, err := NewSemanticControlRepository(db).WithSemanticMutation(ctx, mutationFixture(0, false, []byte("apply-1")), func(tx *gorm.DB) error {
		return tx.Exec("INSERT INTO semantic_test_business(id) VALUES (?)", "resource-1").Error
	})
	require.NoError(t, err)
	require.Equal(t, uint64(1), revision)
	assertRowCount(t, db, "semantic_test_business", 1)
	assertRowCount(t, db, "semantic_document_revisions", 1)
	assertRowCount(t, db, "semantic_outbox", 1)
}

func TestSemanticMutationCASDeleteAndRestore(t *testing.T) {
	ctx, db := context.Background(), newSemanticSQLiteTestDB(t)
	repo := NewSemanticControlRepository(db)
	first, err := repo.WithSemanticMutation(ctx, mutationFixture(0, false, []byte("apply-1")), noBusinessWrite)
	require.NoError(t, err)
	require.Equal(t, uint64(1), first)
	deleted, err := repo.WithSemanticMutation(ctx, mutationFixture(1, true, nil), noBusinessWrite)
	require.NoError(t, err)
	require.Equal(t, uint64(2), deleted)
	assertDenyRevision(t, db, 1, "kb-1", "doc-1", 2)
	assertSemanticEpoch(t, db, 1, "kb-1", 1)
	restored, err := repo.WithSemanticMutation(ctx, mutationFixture(2, false, []byte("apply-3")), noBusinessWrite)
	require.NoError(t, err)
	require.Equal(t, uint64(3), restored)
	assertNoDeny(t, db, 1, "kb-1", "doc-1")
	assertSemanticEpoch(t, db, 1, "kb-1", 2)
	_, err = repo.WithSemanticMutation(ctx, mutationFixture(1, false, []byte("stale")), noBusinessWrite)
	require.ErrorIs(t, err, ErrSemanticRevisionConflict)
}

func TestSemanticPayloadHashAndOutboxLeaseFence(t *testing.T) {
	ctx, db := context.Background(), newSemanticSQLiteTestDB(t)
	repo := NewSemanticControlRepository(db)
	_, err := repo.WithSemanticMutation(ctx, mutationFixture(0, false, []byte("apply-payload")), noBusinessWrite)
	require.NoError(t, err)
	event, err := repo.ClaimSemanticOutbox(ctx, "worker-a", 30)
	require.NoError(t, err)
	require.Equal(t, sha256Hex([]byte("apply-payload")), event.PayloadHash)
	second, err := repo.ClaimSemanticOutbox(ctx, "worker-b", 30)
	require.NoError(t, err)
	require.Nil(t, second)
	require.ErrorIs(t, repo.AckSemanticOutbox(ctx, event.EventID, event.LeaseToken-1), ErrSemanticOutboxLeaseLost)
	require.NoError(t, repo.AckSemanticOutbox(ctx, event.EventID, event.LeaseToken))
}

func TestSemanticMutationPreservesMaximumUint64(t *testing.T) {
	ctx, db := context.Background(), newSemanticSQLiteTestDB(t)
	repo, maximum := NewSemanticControlRepository(db), uint64(math.MaxUint64)
	scope := types.SemanticScopeKey{TenantID: maximum, KBID: "kb-1"}
	require.NoError(t, db.Exec("INSERT INTO semantic_document_revisions(tenant_id,kb_id,document_id,revision,content_hash,deleted) VALUES(?,?,?,?,?,?)", strconv.FormatUint(maximum, 10), "kb-1", "doc-1", strconv.FormatUint(maximum-1, 10), "old", false).Error)
	mutation := mutationFixture(maximum-1, false, []byte("final"))
	mutation.TenantID = maximum
	_, err := repo.WithSemanticMutation(ctx, mutation, noBusinessWrite)
	require.NoError(t, err)
	require.Equal(t, maximum, loadSemanticRevisionAsUint64(t, db, scope, "doc-1"))
	overflow := mutationFixture(maximum, false, []byte("overflow"))
	overflow.TenantID = maximum
	_, err = repo.WithSemanticMutation(ctx, overflow, noBusinessWrite)
	require.ErrorIs(t, err, ErrSemanticRevisionOverflow)
	require.NoError(t, db.Exec("INSERT INTO semantic_access_epochs(tenant_id, kb_id, epoch) VALUES (?, ?, ?)", strconv.FormatUint(maximum, 10), "kb-1", strconv.FormatUint(maximum-1, 10)).Error)
	var epoch uint64
	err = db.Transaction(func(tx *gorm.DB) error { var e error; epoch, e = repo.BumpSemanticEpoch(tx, scope); return e })
	require.NoError(t, err)
	require.Equal(t, maximum, epoch)
	err = db.Transaction(func(tx *gorm.DB) error { _, e := repo.BumpSemanticEpoch(tx, scope); return e })
	require.ErrorIs(t, err, ErrSemanticEpochOverflow)
}

func mutationFixture(expected uint64, deleted bool, payload []byte) types.SemanticMutation {
	return types.SemanticMutation{TenantID: 1, KBID: "kb-1", DocumentID: "doc-1", ExpectedRevision: expected, ContentHash: "opaque-content-hash", Deleted: deleted, Payload: append([]byte(nil), payload...)}
}
func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func assertRowCount(t *testing.T, db *gorm.DB, table string, want int) {
	t.Helper()
	var n int
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM "+table).Scan(&n).Error)
	assert.Equal(t, want, n)
}
func assertDenyRevision(t *testing.T, db *gorm.DB, tenant uint64, kb, document string, want uint64) {
	t.Helper()
	var value string
	require.NoError(t, db.Raw("SELECT revision FROM semantic_denials WHERE tenant_id = ? AND kb_id = ? AND document_id = ?", strconv.FormatUint(tenant, 10), kb, document).Scan(&value).Error)
	assert.Equal(t, strconv.FormatUint(want, 10), value)
}
func assertSemanticEpoch(t *testing.T, db *gorm.DB, tenant uint64, kb string, want uint64) {
	t.Helper()
	var value string
	require.NoError(t, db.Raw("SELECT epoch FROM semantic_access_epochs WHERE tenant_id = ? AND kb_id = ?", strconv.FormatUint(tenant, 10), kb).Scan(&value).Error)
	assert.Equal(t, strconv.FormatUint(want, 10), value)
}
func assertNoDeny(t *testing.T, db *gorm.DB, tenant uint64, kb, document string) {
	t.Helper()
	var n int
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM semantic_denials WHERE tenant_id = ? AND kb_id = ? AND document_id = ?", strconv.FormatUint(tenant, 10), kb, document).Scan(&n).Error)
	assert.Zero(t, n)
}
func loadSemanticRevisionAsUint64(t *testing.T, db *gorm.DB, scope types.SemanticScopeKey, document string) uint64 {
	t.Helper()
	var value string
	require.NoError(t, db.Raw("SELECT revision FROM semantic_document_revisions WHERE tenant_id = ? AND kb_id = ? AND document_id = ?", strconv.FormatUint(scope.TenantID, 10), scope.KBID, document).Scan(&value).Error)
	parsed, err := strconv.ParseUint(value, 10, 64)
	require.NoError(t, err)
	return parsed
}

func newSemanticSQLiteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
	path := filepath.Join(t.TempDir(), "semantic.db")
	sqlDB, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	m, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, m.Up())
	_, _ = m.Close()
	db, err := gorm.Open(sqlite.Open(path+"?_foreign_keys=on"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		c, _ := db.DB()
		if c != nil {
			_ = c.Close()
		}
	})
	return db
}
