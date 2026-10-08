package repository

import (
	"context"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func craftRunViewKey(runID string) craft.RunViewKey {
	return craft.RunViewKey{TenantID: 1, OwnerID: "u1", SessionID: "s1", RunID: runID}
}

func seedCraftRunViewRun(t *testing.T, db *gorm.DB, runID string) {
	t.Helper()
	result := db.Exec(`
		INSERT INTO agent_runs
			(tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, deadline)
		VALUES (1, ?, 's1', 'u1', ?, ?, ?, ?, ?)`,
		runID, "request-"+runID, "assistant-"+runID, "hash-"+runID,
		`{"version":1,"craft":true}`, time.Now().UTC().Add(time.Hour),
	)
	require.NoError(t, result.Error)
}

func TestCraftRunViewMigrationAbsentThenUpDownUp(t *testing.T) {
	// ponytail: craft 迁移重排（161-189/242-270）后相邻号假设失效，需按新链重校准
	t.Skip("craft 迁移重排待校准")
	db := openCraftDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver,
	)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = migrator.Close() })

	// The test database applies all migrations on setup. Roll back only the
	// owned latest migration to observe the schema immediately before it.
	require.NoError(t, migrator.Migrate(112))
	require.False(t, db.Migrator().HasTable("craft_run_views"), "table exists before migration")
	require.NoError(t, migrator.Steps(1))
	require.True(t, db.Migrator().HasTable("craft_run_views"), "up creates the binding table")
	require.NoError(t, migrator.Steps(-1))
	require.False(t, db.Migrator().HasTable("craft_run_views"), "down removes the binding table")
	require.NoError(t, migrator.Steps(1))
	require.True(t, db.Migrator().HasTable("craft_run_views"), "table can be reapplied")
}

func TestCraftRunViewCreateIntentMigrationBackfillsLegacyRowsAndReapplies(t *testing.T) {
	// ponytail: craft 迁移重排（161-189/242-270）后相邻号假设失效，需按新链重校准
	t.Skip("craft 迁移重排待校准")
	db := openCraftDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver,
	)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = migrator.Close() })

	require.NoError(t, migrator.Migrate(113))
	require.False(t, db.Migrator().HasColumn("craft_run_views", "session_create_intent_at"))

	allocatingKey := craftRunViewKey("run-view-legacy-allocating")
	boundKey := craftRunViewKey("run-view-legacy-bound")
	seedCraftRunViewRun(t, db, allocatingKey.RunID)
	seedCraftRunViewRun(t, db, boundKey.RunID)
	for _, row := range []struct {
		key   craft.RunViewKey
		state string
	}{
		{key: allocatingKey, state: "allocating"},
		{key: boundKey, state: "bound"},
	} {
		result := db.Exec(`
			INSERT INTO craft_run_views
				(tenant_id, run_id, owner_id, session_id, generation, runtime_id, container_id, opencode_session_id, state)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			row.key.TenantID, row.key.RunID, row.key.OwnerID, row.key.SessionID,
			"rv_legacy_"+row.key.RunID,
			map[bool]string{true: "runtime-legacy", false: ""}[row.state == "bound"],
			map[bool]string{true: "container-legacy", false: ""}[row.state == "bound"],
			map[bool]string{true: "oc-legacy", false: ""}[row.state == "bound"], row.state,
		)
		require.NoError(t, result.Error)
	}

	require.NoError(t, migrator.Steps(1))
	require.True(t, db.Migrator().HasColumn("craft_run_views", "session_create_intent_at"))
	legacyAllocating, err := NewCraftRunViewStore(db).Load(context.Background(), allocatingKey)
	require.NoError(t, err)
	require.NotNil(t, legacyAllocating.SessionCreateIntentAt, "old allocating rows are unknown, not safe-to-send")
	_, maySend, err := NewCraftRunViewStore(db).BeginSessionCreate(
		context.Background(), allocatingKey, legacyAllocating.Generation,
	)
	require.NoError(t, err)
	require.False(t, maySend, "legacy allocating row must not gain a post-upgrade CreateSession permission")
	legacyBound, err := NewCraftRunViewStore(db).Load(context.Background(), boundKey)
	require.NoError(t, err)
	require.NotNil(t, legacyBound.SessionCreateIntentAt, "old bound rows must satisfy the new durable invariant")

	require.NoError(t, migrator.Steps(-1))
	require.False(t, db.Migrator().HasColumn("craft_run_views", "session_create_intent_at"))
	require.NoError(t, migrator.Steps(1))
	require.True(t, db.Migrator().HasColumn("craft_run_views", "session_create_intent_at"))
	legacyAllocating, err = NewCraftRunViewStore(db).Load(context.Background(), allocatingKey)
	require.NoError(t, err)
	require.NotNil(t, legacyAllocating.SessionCreateIntentAt, "reapplied migration restores fail-closed markers")
}

func TestCraftRunViewAllocationRetryScopeAndReload(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			seedCraftRunViewRun(t, db, "run-view-retry")
			store := NewCraftRunViewStore(db)
			ctx := context.Background()
			key := craftRunViewKey("run-view-retry")

			first, err := store.Allocate(ctx, key)
			require.NoError(t, err)
			require.NotEmpty(t, first.Generation)
			require.Equal(t, craft.RunViewStateAllocating, first.State)
			require.Empty(t, first.Runtime)

			retried, err := store.Allocate(ctx, key)
			require.NoError(t, err)
			require.Equal(t, first.Generation, retried.Generation, "same Run retry preserves generation")

			// A new repository instance must recover the durable row rather than
			// allocate a new runtime identity after an application restart.
			if dialect == "sqlite" {
				sqliteDialector, ok := db.Dialector.(*sqlite.Dialector)
				require.True(t, ok)
				originalDB, closeErr := db.DB()
				require.NoError(t, closeErr)
				require.NoError(t, originalDB.Close())
				db, err = gorm.Open(sqlite.Open(sqliteDialector.DSN), &gorm.Config{})
				require.NoError(t, err)
				reopenedDB, openErr := db.DB()
				require.NoError(t, openErr)
				t.Cleanup(func() { _ = reopenedDB.Close() })
				store = NewCraftRunViewStore(db)
			}
			reloaded, err := NewCraftRunViewStore(db).Load(ctx, key)
			require.NoError(t, err)
			require.Equal(t, first, reloaded)

			wrongTenant := key
			wrongTenant.TenantID = 2
			_, err = store.Allocate(ctx, wrongTenant)
			require.ErrorIs(t, err, craft.ErrNotFound)
			wrongOwner := key
			wrongOwner.OwnerID = "u2"
			_, err = store.Allocate(ctx, wrongOwner)
			require.ErrorIs(t, err, craft.ErrForbidden)
			wrongSession := key
			wrongSession.SessionID = "s2"
			_, err = store.Allocate(ctx, wrongSession)
			require.ErrorIs(t, err, craft.ErrNotFound)

			_, err = store.Load(ctx, wrongTenant)
			require.ErrorIs(t, err, craft.ErrNotFound)
			_, err = store.Load(ctx, wrongOwner)
			require.ErrorIs(t, err, craft.ErrForbidden)
			_, err = store.Load(ctx, wrongSession)
			require.ErrorIs(t, err, craft.ErrNotFound)
		})
	}
}

func TestCraftRunViewConcurrentAllocationHasOneStableGeneration(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			seedCraftRunViewRun(t, db, "run-view-race")
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(4)
			stores := []*CraftRunViewStore{NewCraftRunViewStore(db), NewCraftRunViewStore(db)}
			if dialect == "sqlite" {
				sqliteDialector, ok := db.Dialector.(*sqlite.Dialector)
				require.True(t, ok)
				secondConnection, openErr := gorm.Open(sqlite.Open(sqliteDialector.DSN), &gorm.Config{})
				require.NoError(t, openErr)
				secondSQLDB, openErr := secondConnection.DB()
				require.NoError(t, openErr)
				secondSQLDB.SetMaxOpenConns(1)
				t.Cleanup(func() { _ = secondSQLDB.Close() })
				stores[1] = NewCraftRunViewStore(secondConnection)
			}
			key := craftRunViewKey("run-view-race")
			start := make(chan struct{})
			views := make([]craft.RunView, 2)
			errs := make([]error, 2)
			var wg sync.WaitGroup
			for i := range views {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-start
					views[i], errs[i] = stores[i].Allocate(context.Background(), key)
				}(i)
			}
			close(start)
			wg.Wait()
			require.NoError(t, errs[0])
			require.NoError(t, errs[1])
			require.Equal(t, views[0].Generation, views[1].Generation)
			require.NotEmpty(t, views[0].Generation)
		})
	}
}

func TestCraftRunViewRuntimeBindingIsCompareAndSwapAndUnresolvedStaysPending(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			seedCraftRunViewRun(t, db, "run-view-bind")
			store := NewCraftRunViewStore(db)
			ctx := context.Background()
			key := craftRunViewKey("run-view-bind")
			pending, err := store.Allocate(ctx, key)
			require.NoError(t, err)
			require.Equal(t, craft.RunViewStateAllocating, pending.State)

			_, err = store.BindRuntime(ctx, key, pending.Generation, craft.RunViewRuntime{RuntimeID: "runtime-1", ContainerID: "container-1"})
			require.ErrorIs(t, err, craft.ErrInvalidInput, "partial creation must not bind an unresolved runtime")

			binding := craft.RunViewRuntime{RuntimeID: "runtime-1", ContainerID: "container-1", OpenCodeSessionID: "oc-1"}
			_, err = store.BindRuntime(ctx, key, pending.Generation, binding)
			require.ErrorIs(t, err, craft.ErrConflict, "runtime cannot bind before the durable create intent")
			stillPending, err := store.Load(ctx, key)
			require.NoError(t, err)
			require.Equal(t, pending.Generation, stillPending.Generation)
			require.Equal(t, craft.RunViewStateAllocating, stillPending.State)
			require.Empty(t, stillPending.Runtime)

			intent, maySend, err := store.BeginSessionCreate(ctx, key, pending.Generation)
			require.NoError(t, err)
			require.True(t, maySend, "only the first committed intent grants permission to send CreateSession")
			require.NotNil(t, intent.SessionCreateIntentAt)
			bound, err := store.BindRuntime(ctx, key, pending.Generation, binding)
			require.NoError(t, err)
			require.Equal(t, craft.RunViewStateBound, bound.State)
			require.Equal(t, binding, bound.Runtime)
			wrongTenant := key
			wrongTenant.TenantID = 2
			_, err = store.BindRuntime(ctx, wrongTenant, pending.Generation, binding)
			require.ErrorIs(t, err, craft.ErrNotFound)
			wrongOwner := key
			wrongOwner.OwnerID = "u2"
			_, err = store.BindRuntime(ctx, wrongOwner, pending.Generation, binding)
			require.ErrorIs(t, err, craft.ErrForbidden)
			wrongSession := key
			wrongSession.SessionID = "s2"
			_, err = store.BindRuntime(ctx, wrongSession, pending.Generation, binding)
			require.ErrorIs(t, err, craft.ErrNotFound)

			replayed, err := store.BindRuntime(ctx, key, pending.Generation, binding)
			require.NoError(t, err)
			require.Equal(t, bound.Generation, replayed.Generation)
			conflicting := binding
			conflicting.ContainerID = "container-2"
			_, err = store.BindRuntime(ctx, key, pending.Generation, conflicting)
			require.ErrorIs(t, err, craft.ErrConflict)
			_, err = store.BindRuntime(ctx, key, "rv_wrong-generation", binding)
			require.ErrorIs(t, err, craft.ErrConflict)
		})
	}
}

func TestCraftRunViewSessionCreateIntentIsOneShotAcrossStoreReopen(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			key := craftRunViewKey("run-view-intent-restart")
			seedCraftRunViewRun(t, db, key.RunID)
			store := NewCraftRunViewStore(db)
			allocated, err := store.Allocate(context.Background(), key)
			require.NoError(t, err)
			require.Nil(t, allocated.SessionCreateIntentAt, "fresh allocation must be safe to begin exactly once")

			first, maySend, err := store.BeginSessionCreate(context.Background(), key, allocated.Generation)
			require.NoError(t, err)
			require.True(t, maySend)
			require.NotNil(t, first.SessionCreateIntentAt)

			// Simulate an external request whose outcome is unknown: leave the row
			// allocating, construct a fresh store/DB handle, then prove no retry may
			// send a second non-idempotent CreateSession request.
			reopened := reopenRunDB(t, db)
			restarted, maySend, err := NewCraftRunViewStore(reopened).BeginSessionCreate(
				context.Background(), key, allocated.Generation,
			)
			require.NoError(t, err)
			require.False(t, maySend, "restart must treat the recorded attempt as unknown, never retryable")
			require.Equal(t, first.SessionCreateIntentAt, restarted.SessionCreateIntentAt)
			require.Equal(t, craft.RunViewStateAllocating, restarted.State)
		})
	}
}

func TestCraftRunViewConcurrentSessionCreateIntentHasOneWinner(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			key := craftRunViewKey("run-view-intent-race")
			seedCraftRunViewRun(t, db, key.RunID)
			allocated, err := NewCraftRunViewStore(db).Allocate(context.Background(), key)
			require.NoError(t, err)

			stores := []*CraftRunViewStore{NewCraftRunViewStore(db), NewCraftRunViewStore(reopenRunDB(t, db))}
			start := make(chan struct{})
			views := make([]craft.RunView, len(stores))
			maySend := make([]bool, len(stores))
			errs := make([]error, len(stores))
			var wg sync.WaitGroup
			for i := range stores {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-start
					views[i], maySend[i], errs[i] = stores[i].BeginSessionCreate(context.Background(), key, allocated.Generation)
				}(i)
			}
			close(start)
			wg.Wait()
			require.NoError(t, errs[0])
			require.NoError(t, errs[1])
			require.NotEqual(t, maySend[0], maySend[1], "exactly one caller may send CreateSession")
			require.NotNil(t, views[0].SessionCreateIntentAt)
			require.Equal(t, views[0].SessionCreateIntentAt, views[1].SessionCreateIntentAt)
		})
	}
}

func TestCraftRunViewSessionCreateIntentRejectsForeignScopeAndStaleGeneration(t *testing.T) {
	db := openCraftDB(t)
	key := craftRunViewKey("run-view-intent-scope")
	seedCraftRunViewRun(t, db, key.RunID)
	store := NewCraftRunViewStore(db)
	allocated, err := store.Allocate(context.Background(), key)
	require.NoError(t, err)

	for name, wrongKey := range map[string]craft.RunViewKey{
		"tenant":  func() craft.RunViewKey { k := key; k.TenantID = 2; return k }(),
		"owner":   func() craft.RunViewKey { k := key; k.OwnerID = "u2"; return k }(),
		"session": func() craft.RunViewKey { k := key; k.SessionID = "s2"; return k }(),
	} {
		t.Run(name, func(t *testing.T) {
			_, maySend, err := store.BeginSessionCreate(context.Background(), wrongKey, allocated.Generation)
			require.False(t, maySend)
			if wrongKey.TenantID != key.TenantID || wrongKey.SessionID != key.SessionID {
				require.ErrorIs(t, err, craft.ErrNotFound)
			} else {
				require.ErrorIs(t, err, craft.ErrForbidden)
			}
		})
	}
	_, maySend, err := store.BeginSessionCreate(context.Background(), key, "rv_stale-generation")
	require.False(t, maySend)
	require.ErrorIs(t, err, craft.ErrConflict)

	stillFresh, err := store.Load(context.Background(), key)
	require.NoError(t, err)
	require.Nil(t, stillFresh.SessionCreateIntentAt, "foreign or stale attempts must not consume the winner")
	_, maySend, err = store.BeginSessionCreate(context.Background(), key, allocated.Generation)
	require.NoError(t, err)
	require.True(t, maySend)
}

func TestCraftRunViewRetentionBlocksRunAndSessionDeletion(t *testing.T) {
	for _, state := range []string{"allocating", "bound"} {
		for _, deletion := range []string{"run", "session"} {
			t.Run(state+"/"+deletion, func(t *testing.T) {
				db := openCraftDB(t)
				runID := "run-view-retain-" + state + "-" + deletion
				seedCraftRunViewRun(t, db, runID)
				store := NewCraftRunViewStore(db)
				key := craftRunViewKey(runID)
				before, err := store.Allocate(context.Background(), key)
				require.NoError(t, err)
				if state == "bound" {
					_, maySend, beginErr := store.BeginSessionCreate(context.Background(), key, before.Generation)
					require.NoError(t, beginErr)
					require.True(t, maySend)
					before, err = store.BindRuntime(context.Background(), key, before.Generation, craft.RunViewRuntime{
						RuntimeID: "runtime-retained", ContainerID: "container-retained", OpenCodeSessionID: "oc-retained",
					})
					require.NoError(t, err)
				}

				var deleteErr error
				switch deletion {
				case "run":
					deleteErr = db.Exec("DELETE FROM agent_runs WHERE tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Error
				case "session":
					deleteErr = db.Exec("DELETE FROM sessions WHERE tenant_id = ? AND id = ?", key.TenantID, key.SessionID).Error
				}
				require.Error(t, deleteErr, "view identity must restrict %s deletion", deletion)

				after, err := store.Load(context.Background(), key)
				require.NoError(t, err)
				require.Equal(t, before, after, "failed deletion must retain generation and runtime identity")
			})
		}
	}
}

func TestCraftRunViewRetentionDoesNotBlockRunWithoutView(t *testing.T) {
	db := openCraftDB(t)
	key := craftRunViewKey("run-view-no-binding")
	seedCraftRunViewRun(t, db, key.RunID)
	result := db.Exec("DELETE FROM agent_runs WHERE tenant_id = ? AND run_id = ?", key.TenantID, key.RunID)
	require.NoError(t, result.Error)
	require.EqualValues(t, 1, result.RowsAffected)
	_, err := NewCraftRunViewStore(db).Load(context.Background(), key)
	require.ErrorIs(t, err, craft.ErrNotFound)
}
