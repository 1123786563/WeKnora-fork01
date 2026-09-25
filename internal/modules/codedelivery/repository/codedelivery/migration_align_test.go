package codedelivery

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	_ "github.com/mattn/go-sqlite3"
)

// TestCodeDeliveriesMigrationSQLMatchesModel executes THIS plan's sqlite
// migration verbatim against a clean database and compares the resulting
// columns with the gorm model projection. (本迁移在 issue30 合流时从
// 000113/000192 重编号为 000117/000196 以恢复唯一版本号。)
func TestCodeDeliveriesMigrationSQLMatchesModel(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// 本文件位于 internal/modules/codedelivery/repository/codedelivery/，
	// 5 级 .. 回到 worktree 根。
	upPath := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "..", "migrations", "sqlite", "000117_code_deliveries.up.sql")
	raw, err := os.ReadFile(upPath)
	require.NoError(t, err, "migration file must exist: %s", upPath)

	db, err := sql.Open("sqlite3", "file:"+filepath.Join(t.TempDir(), "migrate.db")+"?_foreign_keys=on")
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec(string(raw))
	require.NoError(t, err)

	rows, err := db.Query(`PRAGMA table_info(code_deliveries)`)
	require.NoError(t, err)
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue any
		var pk int
		require.NoError(t, rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk))
		columns[name] = true
	}
	for _, want := range []string{
		"id", "tenant_id", "task_id", "run_id", "owner_id", "action_id", "connection_id",
		"repo", "baseline_sha", "branch", "commit_sha", "pr_number", "pr_url",
		"remote_login", "state", "failure", "created_at", "updated_at",
	} {
		require.True(t, columns[want], "migration must create column %s (got %v)", want, columns)
	}
}
