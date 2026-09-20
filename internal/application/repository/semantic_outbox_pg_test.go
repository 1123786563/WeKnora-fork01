//go:build semantic_integration

package repository

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

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
