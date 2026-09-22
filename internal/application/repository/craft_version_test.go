package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
)

// craftTestFile builds one manifest file with the real content digest, so
// published identities track bytes exactly as production collections do.
func craftTestFile(t *testing.T, path, content string) craft.File {
	t.Helper()
	sum := sha256.Sum256([]byte(content))
	return craft.File{
		Path: path, Ref: "resource://test/" + path,
		SHA256: hex.EncodeToString(sum[:]), MIME: "text/html", Bytes: int64(len(content)),
	}
}

// publishCraftVersion publishes one version for the seeded workspace and
// returns it.
func publishCraftVersion(t *testing.T, store craft.VersionStore, scope craft.Scope, wsID, runID string, files []craft.File) craft.Version {
	t.Helper()
	digest, err := craft.ManifestDigest(files)
	require.NoError(t, err)
	v, err := store.Publish(context.Background(), scope, craft.Version{
		ID:          craft.VersionID(wsID, runID, digest),
		WorkspaceID: wsID, RunID: runID, Kind: craft.KindWeb,
		Files:  files,
		Checks: []craft.Check{{Name: craft.CheckEntry, Status: craft.CheckPassed, Detail: "entry index.html present"}},
	})
	require.NoError(t, err)
	return v
}

// TestCraftVersionPublishIdempotentByIdentity pins the W01 contract: the
// same (workspace, run, manifest digest) publish always answers the same
// version id, and the identical replay never grows the version list.
func TestCraftVersionPublishIdempotentByIdentity(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			store := NewCraftVersionStore(db)
			ws := putCraftWorkspace(t, NewCraftStore(db))
			scope := craftTestScope()
			files := []craft.File{craftTestFile(t, "index.html", "<h1>v1</h1>")}

			first := publishCraftVersion(t, store, scope, ws.ID, "run-1", files)
			require.NotEmpty(t, first.ID)
			require.Equal(t, craft.VersionID(ws.ID, "run-1", mustDigest(t, files)), first.ID)

			second := publishCraftVersion(t, store, scope, ws.ID, "run-1", files)
			require.Equal(t, first.ID, second.ID)
			require.Equal(t, first.Files, second.Files)
			require.Equal(t, first.Checks, second.Checks)

			list, err := store.List(context.Background(), scope)
			require.NoError(t, err)
			require.Len(t, list, 1)
		})
	}
}

func mustDigest(t *testing.T, files []craft.File) string {
	t.Helper()
	digest, err := craft.ManifestDigest(files)
	require.NoError(t, err)
	return digest
}

// TestCraftVersionDifferentContentIsNewVersion pins immutability: rewritten
// content under the same run publishes a second version, and the first
// version's pinned manifest still answers its original files.
func TestCraftVersionDifferentContentIsNewVersion(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			store := NewCraftVersionStore(db)
			ws := putCraftWorkspace(t, NewCraftStore(db))
			scope := craftTestScope()

			v1 := publishCraftVersion(t, store, scope, ws.ID, "run-1",
				[]craft.File{craftTestFile(t, "index.html", "<h1>v1</h1>")})
			v2 := publishCraftVersion(t, store, scope, ws.ID, "run-1",
				[]craft.File{craftTestFile(t, "index.html", "<h1>v2 changes</h1>")})
			require.NotEqual(t, v1.ID, v2.ID)

			list, err := store.List(context.Background(), scope)
			require.NoError(t, err)
			require.Len(t, list, 2)
			require.Equal(t, v2.ID, list[0].ID, "newest version first")
			require.Equal(t, v1.ID, list[1].ID)

			got, err := store.Get(context.Background(), scope, v1.ID)
			require.NoError(t, err)
			require.Equal(t, v1.Files, got.Files, "old version downloads keep the pinned manifest")
			require.Equal(t, v1.Checks, got.Checks)
		})
	}
}

// TestCraftVersionPublishRejectsMismatchedID pins that the row identity is
// derived, not caller-chosen: an id that disagrees with the manifest digest
// never reaches the database.
func TestCraftVersionPublishRejectsMismatchedID(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			store := NewCraftVersionStore(db)
			ws := putCraftWorkspace(t, NewCraftStore(db))
			files := []craft.File{craftTestFile(t, "index.html", "x")}
			_, err := store.Publish(context.Background(), craftTestScope(), craft.Version{
				ID:          "ver_0000000000000000000000000000000000000000000000000000000000000000",
				WorkspaceID: ws.ID, RunID: "run-1", Kind: craft.KindWeb, Files: files,
			})
			require.ErrorIs(t, err, craft.ErrInvalidInput)
		})
	}
}

// TestCraftVersionScopeGuards pins the ACL: publishing requires the
// workspace's own scope; cross-tenant, cross-session and foreign-owner reads
// are rejected (invisible or forbidden, never leaked).
func TestCraftVersionScopeGuards(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			store := NewCraftVersionStore(db)
			ws := putCraftWorkspace(t, NewCraftStore(db))
			owner := craftTestScope()
			published := publishCraftVersion(t, store, owner, ws.ID, "run-1",
				[]craft.File{craftTestFile(t, "index.html", "x")})

			// Publishing under another session's scope: workspace invisible.
			_, err := store.Publish(context.Background(), craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s2"},
				craft.Version{WorkspaceID: ws.ID, RunID: "run-2", Kind: craft.KindWeb,
					Files: []craft.File{craftTestFile(t, "index.html", "x")}})
			require.ErrorIs(t, err, craft.ErrNotFound)

			// Publishing as another user of the same session: forbidden.
			_, err = store.Publish(context.Background(), craft.Scope{TenantID: 1, UserID: "u2", SessionID: "s1"},
				craft.Version{WorkspaceID: ws.ID, RunID: "run-3", Kind: craft.KindWeb,
					Files: []craft.File{craftTestFile(t, "index.html", "x")}})
			require.ErrorIs(t, err, craft.ErrForbidden)

			// Cross-tenant Get is rejected.
			_, err = store.Get(context.Background(), craft.Scope{TenantID: 2, UserID: "u2", SessionID: "s2"}, published.ID)
			require.ErrorIs(t, err, craft.ErrNotFound)

			// Cross-session Get is invisible, foreign-owner Get forbidden.
			_, err = store.Get(context.Background(), craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s2"}, published.ID)
			require.ErrorIs(t, err, craft.ErrNotFound)
			_, err = store.Get(context.Background(), craft.Scope{TenantID: 1, UserID: "u2", SessionID: "s1"}, published.ID)
			require.ErrorIs(t, err, craft.ErrForbidden)

			// Listing a session without a workspace binding answers not-found.
			_, err = store.List(context.Background(), craft.Scope{TenantID: 1, UserID: "u1", SessionID: "missing"})
			require.ErrorIs(t, err, craft.ErrNotFound)

			// Unknown version ids stay not-found in the owning scope.
			_, err = store.Get(context.Background(), owner, "ver_"+"deadbeef")
			require.ErrorIs(t, err, craft.ErrNotFound)
		})
	}
}

// TestCraftVersionsMigrationDownDropsVersionTables pins the W01 down rule:
// rolling the SQLite migration chain back one step removes the version file
// manifest first and the versions table second, while the R02 workspace
// tables survive untouched.
func TestCraftVersionsMigrationDownDropsVersionTables(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))

	dbPath := filepath.Join(t.TempDir(), "craft-down.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver,
	)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	// Roll back to the version just before W01 explicitly instead of a
	// relative Steps(-1): later chain entries (craft sessions 49, snapshots
	// 50, lifecycle 52) now sit above W01 at 46, and the pinned rule is about
	// the W01 down script itself, not about W01 being the chain head.
	require.NoError(t, migrator.Migrate(45))

	for _, table := range []string{"craft_versions", "craft_version_files"} {
		var count int
		require.NoError(t, sqlDB.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table,
		).Scan(&count))
		require.Zero(t, count, "%s must be dropped by the down migration", table)
	}
	var workspaces int
	require.NoError(t, sqlDB.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'craft_workspaces'",
	).Scan(&workspaces))
	require.Equal(t, 1, workspaces, "R02 workspace tables must survive the W01 down migration")
}
