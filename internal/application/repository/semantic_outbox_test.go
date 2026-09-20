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
	"time"

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
	require.Equal(t, "config-v1", event.ConfigDigest)
	second, err := repo.ClaimSemanticOutbox(ctx, "worker-b", 30)
	require.NoError(t, err)
	require.Nil(t, second)
	require.ErrorIs(t, repo.AckSemanticOutbox(ctx, event.EventID, "worker-a", event.LeaseToken-1), ErrSemanticOutboxLeaseLost)
	require.ErrorIs(t, repo.AckSemanticOutbox(ctx, event.EventID, "wrong-worker", event.LeaseToken), ErrSemanticOutboxLeaseLost)
	require.NoError(t, repo.AckSemanticOutbox(ctx, event.EventID, "worker-a", event.LeaseToken))
}

func TestSemanticOutboxFailureReclaimsStableEvent(t *testing.T) {
	ctx, db := context.Background(), newSemanticSQLiteTestDB(t)
	repo := NewSemanticControlRepository(db)
	_, err := repo.WithSemanticMutation(ctx, mutationFixture(0, false, []byte("payload")), noBusinessWrite)
	require.NoError(t, err)
	first, err := repo.ClaimSemanticOutbox(ctx, "worker-a", 30)
	require.NoError(t, err)
	require.NoError(t, repo.FailSemanticOutbox(ctx, first.EventID, "worker-a", first.LeaseToken, time.Unix(1, 0), "unavailable"))
	second, err := repo.ClaimSemanticOutbox(ctx, "worker-b", 30)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, first.EventID, second.EventID)
	require.Equal(t, first.PayloadHash, second.PayloadHash)
	require.Equal(t, "config-v1", second.ConfigDigest)
	require.Equal(t, "unavailable", second.ErrorCode)
	require.ErrorIs(t, repo.AckSemanticOutbox(ctx, first.EventID, "worker-a", first.LeaseToken), ErrSemanticOutboxLeaseLost)
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
	event, err := repo.ClaimSemanticOutbox(ctx, "max-worker", 30)
	require.NoError(t, err)
	require.Equal(t, maximum, event.Scope.TenantID)
	require.Equal(t, maximum, event.Revision)
	require.NoError(t, db.Exec("INSERT INTO semantic_access_epochs(tenant_id, kb_id, epoch) VALUES (?, ?, ?)", strconv.FormatUint(maximum, 10), "kb-1", strconv.FormatUint(maximum-1, 10)).Error)
	var epoch uint64
	err = db.Transaction(func(tx *gorm.DB) error { var e error; epoch, e = repo.BumpSemanticEpoch(tx, scope); return e })
	require.NoError(t, err)
	require.Equal(t, maximum, epoch)
	err = db.Transaction(func(tx *gorm.DB) error { _, e := repo.BumpSemanticEpoch(tx, scope); return e })
	require.ErrorIs(t, err, ErrSemanticEpochOverflow)
}

func TestSemanticUint64StorageRejectsValuesAboveMaximumSQLite(t *testing.T) {
	db := newSemanticSQLiteTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("PRAGMA foreign_keys=OFF").Error)
	const tooLarge = "18446744073709551616"
	invalidRows := []struct {
		name  string
		query string
	}{
		{"revision tenant", "INSERT INTO semantic_document_revisions VALUES('" + tooLarge + "','kb-1','bad-tenant','0','hash',0)"},
		{"document revision", "INSERT INTO semantic_document_revisions VALUES('1','kb-1','bad-revision','" + tooLarge + "','hash',0)"},
		{"epoch tenant", "INSERT INTO semantic_access_epochs VALUES('" + tooLarge + "','kb-1','0')"},
		{"epoch value", "INSERT INTO semantic_access_epochs VALUES('1','kb-1','" + tooLarge + "')"},
		{"denial tenant", "INSERT INTO semantic_denials VALUES('" + tooLarge + "','kb-1','orphan','0')"},
		{"denial revision", "INSERT INTO semantic_denials VALUES('1','kb-1','parent','" + tooLarge + "')"},
		{"outbox tenant", "INSERT INTO semantic_outbox(event_id,tenant_id,kb_id,document_id,revision,content_hash,config_digest,deleted,payload,payload_hash,retry_at) VALUES('tenant-overflow','" + tooLarge + "','kb-1','doc','1','hash','config',0,X'01','hash','2000-01-01 00:00:00')"},
		{"outbox revision", "INSERT INTO semantic_outbox(event_id,tenant_id,kb_id,document_id,revision,content_hash,config_digest,deleted,payload,payload_hash,retry_at) VALUES('revision-overflow','1','kb-1','doc','" + tooLarge + "','hash','config',0,X'01','hash','2000-01-01 00:00:00')"},
		{"outbox attempts", "INSERT INTO semantic_outbox(event_id,tenant_id,kb_id,document_id,revision,content_hash,config_digest,deleted,payload,payload_hash,attempt_count,retry_at) VALUES('attempt-overflow','1','kb-1','doc','1','hash','config',0,X'01','hash','" + tooLarge + "','2000-01-01 00:00:00')"},
		{"outbox lease token", "INSERT INTO semantic_outbox(event_id,tenant_id,kb_id,document_id,revision,content_hash,config_digest,deleted,payload,payload_hash,lease_token,retry_at) VALUES('token-overflow','1','kb-1','doc','1','hash','config',0,X'01','hash','" + tooLarge + "','2000-01-01 00:00:00')"},
	}
	for _, tc := range invalidRows {
		t.Run(tc.name, func(t *testing.T) {
			require.Error(t, db.Exec(tc.query).Error)
		})
	}
}

func TestSemanticOutboxClaimRejectsMalformedPersistedUint64SQLite(t *testing.T) {
	for _, column := range []string{"tenant_id", "revision"} {
		t.Run(column, func(t *testing.T) {
			db := newSemanticSQLiteTestDB(t)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			require.NoError(t, db.Exec("PRAGMA ignore_check_constraints=ON").Error)
			values := map[string]string{"tenant_id": "1", "revision": "1"}
			values[column] = "not-a-uint64"
			query := "INSERT INTO semantic_outbox(event_id,tenant_id,kb_id,document_id,revision,content_hash,config_digest,deleted,payload,payload_hash,retry_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)"
			require.NoError(t, db.Exec(query, "malformed-"+column, values["tenant_id"], "kb-1", "doc", values["revision"], "hash", "config", false, []byte("p"), "hash", time.Unix(1, 0)).Error)
			_, err = NewSemanticControlRepository(db).ClaimSemanticOutbox(context.Background(), "worker", 30)
			require.Error(t, err)
		})
	}
}

func TestSemanticOutboxCountersReachMaximumThenRejectOverflowSQLite(t *testing.T) {
	maximum := strconv.FormatUint(math.MaxUint64, 10)
	for _, tc := range []struct {
		name, attempts, token, maxField string
	}{
		{"attempt count", strconv.FormatUint(math.MaxUint64-1, 10), "0", "attempt_count"},
		{"lease token", "0", strconv.FormatUint(math.MaxUint64-1, 10), "lease_token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newSemanticSQLiteTestDB(t)
			query := "INSERT INTO semantic_outbox(event_id,tenant_id,kb_id,document_id,revision,content_hash,config_digest,deleted,payload,payload_hash,attempt_count,lease_token,retry_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)"
			require.NoError(t, db.Exec(query, "counter-max", "1", "kb-1", "doc", "1", "hash", "config", false, []byte("p"), "hash", tc.attempts, tc.token, time.Unix(1, 0)).Error)
			repo := NewSemanticControlRepository(db)
			event, err := repo.ClaimSemanticOutbox(context.Background(), "worker", 30)
			require.NoError(t, err)
			require.NotNil(t, event)
			if tc.maxField == "attempt_count" {
				require.Equal(t, uint64(math.MaxUint64), event.AttemptCount)
			} else {
				require.Equal(t, uint64(math.MaxUint64), event.LeaseToken)
			}
			var stored string
			require.NoError(t, db.Raw("SELECT "+tc.maxField+" FROM semantic_outbox WHERE event_id=?", event.EventID).Row().Scan(&stored))
			require.Equal(t, maximum, stored)
			require.NoError(t, db.Exec("UPDATE semantic_outbox SET retry_at=?,lease_expires_at=? WHERE event_id=?", time.Unix(1, 0), time.Unix(1, 0), event.EventID).Error)
			_, err = repo.ClaimSemanticOutbox(context.Background(), "worker-2", 30)
			require.ErrorIs(t, err, ErrSemanticOutboxOverflow)
		})
	}
}

func mutationFixture(expected uint64, deleted bool, payload []byte) types.SemanticMutation {
	return types.SemanticMutation{TenantID: 1, KBID: "kb-1", DocumentID: "doc-1", ExpectedRevision: expected, ContentHash: "opaque-content-hash", ConfigDigest: "config-v1", Deleted: deleted, Payload: append([]byte(nil), payload...)}
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
