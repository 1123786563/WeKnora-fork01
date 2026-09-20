//go:build semantic_integration

package repository

import (
	"context"
	"database/sql"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSemanticMutationPostgresConcurrentSameRevision(t *testing.T) {
	db := newSemanticPostgresDB(t)
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			_, err := NewSemanticControlRepository(db.Session(&gorm.Session{NewDB: true})).WithSemanticMutation(context.Background(), mutationFixture(0, false, []byte("race")), noBusinessWrite)
			results <- err
		}()
	}
	close(start)
	a, b := <-results, <-results
	require.NotEqual(t, a == nil, b == nil)
	var revision string
	require.NoError(t, db.Raw("SELECT revision FROM semantic_document_revisions WHERE tenant_id='1' AND kb_id='kb-1' AND document_id='doc-1'").Scan(&revision).Error)
	require.Equal(t, "1", revision)
}

func TestSemanticMutationPostgresConcurrentDistinctTombstones(t *testing.T) {
	db := newSemanticPostgresDB(t)
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, doc := range []string{"a", "b"} {
		go func(doc string) {
			<-start
			m := mutationFixture(0, true, nil)
			m.DocumentID = doc
			_, err := NewSemanticControlRepository(db.Session(&gorm.Session{NewDB: true})).WithSemanticMutation(context.Background(), m, noBusinessWrite)
			results <- err
		}(doc)
	}
	close(start)
	require.NoError(t, <-results)
	require.NoError(t, <-results)
	var epoch string
	require.NoError(t, db.Raw("SELECT epoch FROM semantic_access_epochs WHERE tenant_id='1' AND kb_id='kb-1'").Scan(&epoch).Error)
	require.Equal(t, "2", epoch)
}

func TestSemanticOutboxPostgresFailureReclaimAndLeaseFencing(t *testing.T) {
	db := newSemanticPostgresDB(t)
	repo := NewSemanticControlRepository(db)
	_, err := repo.WithSemanticMutation(context.Background(), mutationFixture(0, false, []byte("pg-payload")), noBusinessWrite)
	require.NoError(t, err)
	first, err := repo.ClaimSemanticOutbox(context.Background(), "worker-a", 30)
	require.NoError(t, err)
	require.NoError(t, repo.FailSemanticOutbox(context.Background(), first.EventID, "worker-a", first.LeaseToken, time.Unix(1, 0), "pg-unavailable"))
	second, err := repo.ClaimSemanticOutbox(context.Background(), "worker-b", 30)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, first.EventID, second.EventID)
	require.Equal(t, first.PayloadHash, second.PayloadHash)
	require.Equal(t, first.ConfigDigest, second.ConfigDigest)
	require.Equal(t, "pg-unavailable", second.ErrorCode)
	require.ErrorIs(t, repo.AckSemanticOutbox(context.Background(), first.EventID, "worker-a", first.LeaseToken), ErrSemanticOutboxLeaseLost)
	require.ErrorIs(t, repo.FailSemanticOutbox(context.Background(), first.EventID, "worker-a", first.LeaseToken, time.Now(), "stale"), ErrSemanticOutboxLeaseLost)
	require.NoError(t, repo.AckSemanticOutbox(context.Background(), second.EventID, "worker-b", second.LeaseToken))
}

func TestSemanticOutboxPostgresPreservesMaximumUint64AndRejectsFenceOverflow(t *testing.T) {
	db := newSemanticPostgresDB(t)
	repo, maximum := NewSemanticControlRepository(db), uint64(math.MaxUint64)
	scope := types.SemanticScopeKey{TenantID: maximum, KBID: "max-kb"}
	require.NoError(t, db.Exec("INSERT INTO semantic_document_revisions(tenant_id,kb_id,document_id,revision,content_hash,deleted) VALUES(?,?,?,?,?,?)", strconv.FormatUint(maximum, 10), "max-kb", "max-doc", strconv.FormatUint(maximum-1, 10), "old", false).Error)
	mutation := mutationFixture(maximum-1, false, []byte("last-revision"))
	mutation.TenantID, mutation.KBID, mutation.DocumentID = maximum, "max-kb", "max-doc"
	_, err := repo.WithSemanticMutation(context.Background(), mutation, noBusinessWrite)
	require.NoError(t, err)
	event, err := repo.ClaimSemanticOutbox(context.Background(), "max-worker", 30)
	require.NoError(t, err)
	require.Equal(t, maximum, event.Scope.TenantID)
	require.Equal(t, maximum, event.Revision)
	require.Equal(t, uint64(1), event.AttemptCount)
	require.Equal(t, uint64(1), event.LeaseToken)
	require.NoError(t, db.Exec("INSERT INTO semantic_access_epochs(tenant_id,kb_id,epoch) VALUES(?,?,?)", strconv.FormatUint(maximum, 10), "max-kb", strconv.FormatUint(maximum-1, 10)).Error)
	var epoch uint64
	err = db.Transaction(func(tx *gorm.DB) error { var e error; epoch, e = repo.BumpSemanticEpoch(tx, scope); return e })
	require.NoError(t, err)
	require.Equal(t, maximum, epoch)
	err = db.Transaction(func(tx *gorm.DB) error { _, e := repo.BumpSemanticEpoch(tx, scope); return e })
	require.ErrorIs(t, err, ErrSemanticEpochOverflow)

	for _, row := range []struct {
		id, attempts, token, maxField string
	}{
		{"00000000-0000-0000-0000-000000000001", strconv.FormatUint(maximum-1, 10), "0", "attempt_count"},
		{"00000000-0000-0000-0000-000000000002", "0", strconv.FormatUint(maximum-1, 10), "lease_token"},
	} {
		require.NoError(t, db.Exec("INSERT INTO semantic_outbox(event_id,tenant_id,kb_id,document_id,revision,content_hash,config_digest,deleted,payload,payload_hash,attempt_count,lease_token,retry_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)", row.id, "1", "kb-1", row.id, "1", "hash", "config", false, []byte("p"), "hash", row.attempts, row.token, time.Now().Add(-time.Minute)).Error)
		event, err := repo.ClaimSemanticOutbox(context.Background(), "overflow-worker", 30)
		require.NoError(t, err)
		require.NotNil(t, event)
		if row.maxField == "attempt_count" {
			require.Equal(t, maximum, event.AttemptCount)
		} else {
			require.Equal(t, maximum, event.LeaseToken)
		}
		var stored string
		require.NoError(t, db.Raw("SELECT "+row.maxField+" FROM semantic_outbox WHERE event_id=?", row.id).Row().Scan(&stored))
		require.Equal(t, strconv.FormatUint(maximum, 10), stored)
		require.NoError(t, db.Exec("UPDATE semantic_outbox SET retry_at=now()-interval '1 minute',lease_expires_at=now()-interval '1 minute' WHERE event_id=?", row.id).Error)
		_, err = repo.ClaimSemanticOutbox(context.Background(), "overflow-worker-2", 30)
		require.ErrorIs(t, err, ErrSemanticOutboxOverflow)
		require.NoError(t, db.Exec("DELETE FROM semantic_outbox WHERE event_id=?", row.id).Error)
	}
}

func TestSemanticPostgresUint64ConstraintsRejectValuesAboveMaximum(t *testing.T) {
	db := newSemanticPostgresDB(t)
	const tooLarge = "18446744073709551616"
	require.NoError(t, db.Exec("INSERT INTO semantic_document_revisions(tenant_id,kb_id,document_id,revision,content_hash,deleted) VALUES('1','kb-1','parent','0','hash',false)").Error)
	invalidRows := []struct {
		name  string
		query string
	}{
		{"revision tenant", "INSERT INTO semantic_document_revisions VALUES('" + tooLarge + "','kb-1','bad-tenant','0','hash',false)"},
		{"document revision", "INSERT INTO semantic_document_revisions VALUES('1','kb-1','bad-revision','" + tooLarge + "','hash',false)"},
		{"epoch tenant", "INSERT INTO semantic_access_epochs VALUES('" + tooLarge + "','kb-1','0')"},
		{"epoch value", "INSERT INTO semantic_access_epochs VALUES('1','kb-1','" + tooLarge + "')"},
		{"denial tenant", "INSERT INTO semantic_denials VALUES('" + tooLarge + "','kb-1','orphan','0')"},
		{"denial revision", "INSERT INTO semantic_denials VALUES('1','kb-1','parent','" + tooLarge + "')"},
		{"outbox tenant", "INSERT INTO semantic_outbox(event_id,tenant_id,kb_id,document_id,revision,content_hash,config_digest,deleted,payload,payload_hash,retry_at) VALUES('00000000-0000-0000-0000-000000000011','" + tooLarge + "','kb-1','doc','1','hash','config',false,'p','hash',now())"},
		{"outbox revision", "INSERT INTO semantic_outbox(event_id,tenant_id,kb_id,document_id,revision,content_hash,config_digest,deleted,payload,payload_hash,retry_at) VALUES('00000000-0000-0000-0000-000000000012','1','kb-1','doc','" + tooLarge + "','hash','config',false,'p','hash',now())"},
		{"outbox attempts", "INSERT INTO semantic_outbox(event_id,tenant_id,kb_id,document_id,revision,content_hash,config_digest,deleted,payload,payload_hash,attempt_count,retry_at) VALUES('00000000-0000-0000-0000-000000000013','1','kb-1','doc','1','hash','config',false,'p','hash','" + tooLarge + "',now())"},
		{"outbox lease token", "INSERT INTO semantic_outbox(event_id,tenant_id,kb_id,document_id,revision,content_hash,config_digest,deleted,payload,payload_hash,lease_token,retry_at) VALUES('00000000-0000-0000-0000-000000000014','1','kb-1','doc','1','hash','config',false,'p','hash','" + tooLarge + "',now())"},
	}
	for _, tc := range invalidRows {
		t.Run(tc.name, func(t *testing.T) { require.Error(t, db.Exec(tc.query).Error) })
	}
}

func TestSemanticMutationPostgresRollback(t *testing.T) {
	db := newSemanticPostgresDB(t)
	repo := NewSemanticControlRepository(db)
	require.NoError(t, db.Exec("CREATE TABLE semantic_test_business(id TEXT PRIMARY KEY)").Error)
	_, err := repo.WithSemanticMutation(t.Context(), mutationFixture(0, true, nil), func(tx *gorm.DB) error {
		require.NoError(t, tx.Exec("INSERT INTO semantic_test_business VALUES('x')").Error)
		return os.ErrPermission
	})
	require.Error(t, err)
	assertRowCount(t, db, "semantic_test_business", 0)
	assertRowCount(t, db, "semantic_document_revisions", 0)
	assertRowCount(t, db, "semantic_outbox", 0)
	assertRowCount(t, db, "semantic_denials", 0)
	assertRowCount(t, db, "semantic_access_epochs", 0)
}

func newSemanticPostgresDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TRPC_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("TRPC_TEST_POSTGRES_DSN is required for semantic PostgreSQL integration")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, admin.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp" SCHEMA public`).Error)
	require.NoError(t, admin.Exec(`CREATE EXTENSION IF NOT EXISTS pg_trgm SCHEMA public`).Error)
	schema := "semantic_i02_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		require.NoError(t, admin.Exec("DROP SCHEMA "+schema+" CASCADE").Error)
		c, _ := admin.DB()
		if c != nil {
			_ = c.Close()
		}
	})
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema+",public")
	q.Set("options", "-c app.skip_embedding=true")
	u.RawQuery = q.Encode()
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	c, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	driver, err := pgmigrate.WithInstance(c, &pgmigrate.Config{SchemaName: schema})
	require.NoError(t, err)
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
	m, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/versioned"), "postgres", driver)
	require.NoError(t, err)
	require.NoError(t, m.Up())
	t.Cleanup(func() { _, _ = m.Close() })
	_ = sql.ErrNoRows
	return db
}
