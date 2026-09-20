//go:build semantic_integration

package database

import (
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
)

func TestSemanticPostgresMigrationUpDownUp(t *testing.T) {
	semanticControlTables := []string{"semantic_document_revisions", "semantic_outbox", "semantic_access_epochs", "semantic_denials"}
	semanticPolicyTables := []string{"semantic_model_policies", "semantic_model_policy_revisions"}
	dsn := os.Getenv("TRPC_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("TRPC_TEST_POSTGRES_DSN is required for semantic PostgreSQL integration")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
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
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{})
	require.NoError(t, err)
	c, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	driver, err := pgmigrate.WithInstance(c, &pgmigrate.Config{SchemaName: schema})
	require.NoError(t, err)
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
	m, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/versioned"), "postgres", driver)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close() })
	require.NoError(t, m.Up())
	version, dirty, err := m.Version()
	require.NoError(t, err)
	require.Equal(t, uint(179), version)
	require.False(t, dirty)
	assertTablesAndIndex := func(want bool) {
		for _, table := range append(semanticControlTables, semanticPolicyTables...) {
			var n int
			require.NoError(t, db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=? AND table_name=?", schema, table).Scan(&n).Error)
			if want {
				require.Equal(t, 1, n)
			} else {
				require.Zero(t, n)
			}
		}
		var n int
		require.NoError(t, db.Raw("SELECT COUNT(*) FROM pg_indexes WHERE schemaname=? AND indexname='idx_semantic_outbox_claim'", schema).Scan(&n).Error)
		if want {
			require.Equal(t, 1, n)
		} else {
			require.Zero(t, n)
		}
	}
	assertTablesAndIndex(true)
	require.NoError(t, m.Steps(-1))
	for _, table := range semanticPolicyTables {
		var n int
		require.NoError(t, db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=? AND table_name=?", schema, table).Scan(&n).Error)
		require.Zero(t, n)
	}
	for _, table := range semanticControlTables {
		var n int
		require.NoError(t, db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=? AND table_name=?", schema, table).Scan(&n).Error)
		require.Equal(t, 1, n)
	}
	require.NoError(t, m.Up())
	version, dirty, err = m.Version()
	require.NoError(t, err)
	require.Equal(t, uint(179), version)
	require.False(t, dirty)
	assertTablesAndIndex(true)
	_ = sql.ErrNoRows
}
