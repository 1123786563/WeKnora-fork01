package appconnector

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openSpaceGrantDB opens a REAL fully-migrated sqlite database (the full
// migrations/sqlite track, same shape as the task-grant suite's
// openTaskGrantDB) — this pins the migration↔projection alignment for
// app_space_connection_grants, not just the gorm model.
func openSpaceGrantDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// Five levels up: <repo>/internal/appconnector/repository/appconnector
	// → <repo> (same depth as oc_store_test.go's ../../../../migrations/sqlite).
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "space-grants.db") + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func TestSpaceConnectionGrantStoreUpsertListRevoke(t *testing.T) {
	db := openSpaceGrantDB(t)
	store := NewSpaceConnectionGrantStore(db)
	ctx := context.Background()

	require.NoError(t, store.GrantSpaceConnection(ctx, 7, "conn-space", "bob", "alice"))
	require.NoError(t, store.GrantSpaceConnection(ctx, 7, "conn-space", "carol", "alice"))
	// Upsert is idempotent: re-granting bob rewrites granted_by, never duplicates.
	require.NoError(t, store.GrantSpaceConnection(ctx, 7, "conn-space", "bob", "alice"))

	grants, err := store.ListSpaceConnectionGrants(ctx, 7, "conn-space")
	require.NoError(t, err)
	require.Len(t, grants, 2, "同一 (tenant, connection, actor) 只能有一行")

	granted, err := store.SpaceConnectionGranted(ctx, 7, "conn-space", "bob")
	require.NoError(t, err)
	require.True(t, granted)

	// Tenant scope is part of every read: tenant 8's rows are unreachable.
	granted, err = store.SpaceConnectionGranted(ctx, 8, "conn-space", "bob")
	require.NoError(t, err)
	require.False(t, granted, "跨租户 grant 行不可达")

	// Revoke is idempotent and immediately converges.
	require.NoError(t, store.RevokeSpaceConnection(ctx, 7, "conn-space", "bob"))
	require.NoError(t, store.RevokeSpaceConnection(ctx, 7, "conn-space", "bob"), "重复撤销是成功")
	granted, err = store.SpaceConnectionGranted(ctx, 7, "conn-space", "bob")
	require.NoError(t, err)
	require.False(t, granted, "撤销后下一次判定立即为 false")

	grants, err = store.ListSpaceConnectionGrants(ctx, 7, "conn-space")
	require.NoError(t, err)
	require.Len(t, grants, 1)
	require.Equal(t, "carol", grants[0].ActorID)
	require.Equal(t, "alice", grants[0].GrantedBy)
}

// TestGrantSpaceConnectionKeepsFirstCreatorOnRegrant pins B5-F56: a
// re-grant rewrites granted_by (the operator intent) and advances
// updated_at, but must never reset created_at — the first grant time is
// audit history, not a mutable column of the latest write.
func TestGrantSpaceConnectionKeepsFirstCreatorOnRegrant(t *testing.T) {
	db := openSpaceGrantDB(t)
	store := NewSpaceConnectionGrantStore(db)
	ctx := context.Background()

	require.NoError(t, store.GrantSpaceConnection(ctx, 7, "conn-space", "bob", "alice"))
	var first SpaceConnectionGrantRow
	require.NoError(t, db.Where("tenant_id = ? AND connection_id = ? AND actor_id = ?", 7, "conn-space", "bob").
		First(&first).Error)
	firstCreatedAt := first.CreatedAt

	// Let the clock tick so a reset would be observable.
	time.Sleep(10 * time.Millisecond)

	require.NoError(t, store.GrantSpaceConnection(ctx, 7, "conn-space", "bob", "dave"))

	var after SpaceConnectionGrantRow
	require.NoError(t, db.Where("tenant_id = ? AND connection_id = ? AND actor_id = ?", 7, "conn-space", "bob").
		First(&after).Error)
	require.Equal(t, firstCreatedAt, after.CreatedAt, "重复授权不得抹掉首次授予时间（B5-F56）")
	require.Equal(t, "dave", after.GrantedBy, "重授权改写 granted_by 为最新操作者")
	require.True(t, after.UpdatedAt.After(first.UpdatedAt) || !after.UpdatedAt.Equal(first.CreatedAt), "updated_at 允许前进")
}
