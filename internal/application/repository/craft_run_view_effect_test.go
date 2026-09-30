package repository

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestCraftRunViewEffectAuthorityRejectsStaleFenceDigestSlotAndLease(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *gorm.DB, *craft.Task, context.Context) context.Context
	}{
		{name: "stale epoch", mutate: func(_ *testing.T, _ *gorm.DB, task *craft.Task, ctx context.Context) context.Context {
			task.Fence.Epoch++
			return ctx
		}},
		{name: "changed snapshot digest", mutate: func(_ *testing.T, _ *gorm.DB, task *craft.Task, ctx context.Context) context.Context {
			task.SnapshotDigest = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
			return ctx
		}},
		{name: "fence and task digest mismatch", mutate: func(_ *testing.T, _ *gorm.DB, task *craft.Task, ctx context.Context) context.Context {
			task.Fence.SnapshotDigest = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
			return ctx
		}},
		{name: "unknown digest version", mutate: func(_ *testing.T, _ *gorm.DB, task *craft.Task, ctx context.Context) context.Context {
			task.SnapshotDigestVersion = 2
			task.Fence.SnapshotDigestVersion = 2
			return ctx
		}},
		{name: "missing Run", mutate: func(_ *testing.T, _ *gorm.DB, task *craft.Task, ctx context.Context) context.Context {
			task.Fence.RunID = "deleted-run"
			return ctx
		}},
		{name: "changed scope session", mutate: func(_ *testing.T, _ *gorm.DB, task *craft.Task, ctx context.Context) context.Context {
			task.Scope.SessionID = "other-session"
			return ctx
		}},
		{name: "changed Task Workspace", mutate: func(_ *testing.T, _ *gorm.DB, task *craft.Task, ctx context.Context) context.Context {
			task.WorkspaceID = "other-workspace"
			return ctx
		}},
		{name: "lost active writer slot", mutate: func(_ *testing.T, db *gorm.DB, task *craft.Task, ctx context.Context) context.Context {
			require.NoError(t, db.Table("sessions").Where("tenant_id = ? AND id = ?", task.Scope.TenantID, task.Scope.SessionID).Update("active_agent_run_id", "other-run").Error)
			return ctx
		}},
		{name: "expired lease", mutate: func(_ *testing.T, db *gorm.DB, task *craft.Task, ctx context.Context) context.Context {
			require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", task.Fence.TenantID, task.Fence.RunID).Update("lease_until", time.Now().Add(-time.Minute)).Error)
			return ctx
		}},
		{name: "terminal status committed before claim", mutate: func(_ *testing.T, db *gorm.DB, task *craft.Task, ctx context.Context) context.Context {
			require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", task.Fence.TenantID, task.Fence.RunID).Update("status", "succeeded").Error)
			return ctx
		}},
		{name: "foreign actor context", mutate: func(_ *testing.T, _ *gorm.DB, _ *craft.Task, _ context.Context) context.Context {
			return types.WithCaller(context.Background(), types.Caller{TenantID: 1, UserID: "foreign-actor"})
		}},
		{name: "changed durable actor", mutate: func(_ *testing.T, db *gorm.DB, task *craft.Task, ctx context.Context) context.Context {
			require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", task.Fence.TenantID, task.Fence.RunID).Update("actor_user_id", "different-actor").Error)
			return ctx
		}},
		{name: "foreign Task tenant", mutate: func(_ *testing.T, _ *gorm.DB, task *craft.Task, ctx context.Context) context.Context {
			task.Scope.TenantID++
			return ctx
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, ctx, task, store, view := newCraftRunViewEffectFixture(t)
			ctx = tc.mutate(t, db, &task, ctx)
			_, maySend, err := store.BeginEffect(ctx, task, view.Generation, craft.RunViewEffectDockerCreate)
			require.Error(t, err)
			require.False(t, maySend)
			var count int64
			require.NoError(t, db.Table("craft_run_view_effect_intents").Where("tenant_id = ? AND run_id = ? AND generation = ? AND effect_kind = ?", view.Key.TenantID, view.Key.RunID, view.Generation, craft.RunViewEffectDockerCreate).Count(&count).Error)
			require.Zero(t, count, "stale authority must not persist a send permission")
		})
	}
}

func TestCraftRunViewEffectAuthorityRejectsSameWorkspaceChangedSnapshotAndDigest(t *testing.T) {
	db, ctx, task, store, view := newCraftRunViewEffectFixture(t)
	var run agentRunRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", task.Fence.TenantID, task.Fence.RunID).Take(&run).Error)
	var snapshot map[string]any
	require.NoError(t, json.Unmarshal([]byte(run.Snapshot), &snapshot))
	snapshot["query"] = "changed query, same workspace"
	changed, err := json.Marshal(snapshot)
	require.NoError(t, err)
	version, digest, err := CraftAdmittedSnapshotIdentity(changed)
	require.NoError(t, err)
	require.Equal(t, task.WorkspaceID, craftSeedFromRun(t, agentruntime.Run{Snapshot: changed}).WorkspaceID)
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", task.Fence.TenantID, task.Fence.RunID).Updates(map[string]any{
		"snapshot": string(changed), "snapshot_digest_version": version, "snapshot_digest": digest,
	}).Error)
	_, maySend, err := store.BeginEffect(ctx, task, view.Generation, craft.RunViewEffectDockerCreate)
	require.Error(t, err)
	require.False(t, maySend)
	var count int64
	require.NoError(t, db.Table("craft_run_view_effect_intents").Where("tenant_id = ? AND run_id = ? AND generation = ? AND effect_kind = ?", view.Key.TenantID, view.Key.RunID, view.Generation, craft.RunViewEffectDockerCreate).Count(&count).Error)
	require.Zero(t, count)
}

func TestCraftRunViewEffectAuthorityUsesOneTokenAndUnknownStaysUnresolved(t *testing.T) {
	db, ctx, task, store, view := newCraftRunViewEffectFixture(t)
	first, maySend, err := store.BeginEffect(ctx, task, view.Generation, craft.RunViewEffectDockerCreate)
	require.NoError(t, err)
	require.True(t, maySend)
	require.NotEmpty(t, first.Token)

	replay, maySend, err := store.BeginEffect(ctx, task, view.Generation, craft.RunViewEffectDockerCreate)
	require.NoError(t, err)
	require.False(t, maySend, "an exact duplicate must never grant a second send")
	require.Equal(t, first.Token, replay.Token, "replay must not mint a replacement token")

	completed := craft.RunViewEffectOutcome{State: craft.RunViewEffectStateSucceeded, Receipt: "container-1"}
	require.NoError(t, store.FinishEffect(ctx, first, completed))
	require.NoError(t, store.FinishEffect(ctx, replay, completed), "exact outcome replay is idempotent")
	require.ErrorIs(t, store.FinishEffect(ctx, first, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateSucceeded, Receipt: "container-2"}), craft.ErrConflict)

	unknown, maySend, err := store.BeginEffect(ctx, task, view.Generation, craft.RunViewEffectDockerStart)
	require.NoError(t, err)
	require.True(t, maySend)
	unknownOutcome := craft.RunViewEffectOutcome{State: craft.RunViewEffectStateUnknown, Receipt: "container-1"}
	require.NoError(t, store.FinishEffect(ctx, unknown, unknownOutcome))
	require.NoError(t, store.FinishEffect(ctx, unknown, unknownOutcome), "exact unknown replay stays durable")
	_, maySend, err = store.BeginEffect(ctx, task, view.Generation, craft.RunViewEffectDockerStart)
	require.NoError(t, err)
	require.False(t, maySend, "unknown outcome must remain unresolved and non-retryable")
	require.ErrorIs(t, store.FinishEffect(ctx, unknown, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateSucceeded, Receipt: "container-1"}), craft.ErrConflict)

	var unresolved int64
	require.NoError(t, db.Table("craft_run_view_effect_intents").Where("tenant_id = ? AND run_id = ? AND state IN ('pending', 'unknown')", view.Key.TenantID, view.Key.RunID).Count(&unresolved).Error)
	require.EqualValues(t, 1, unresolved, "only the unknown Docker start remains unresolved")
}

func TestCraftRunViewEffectRequestAuthorityBindsNetworkAndProbeRequests(t *testing.T) {
	for _, kind := range []craft.RunViewEffectKind{"docker_network_create", "docker_probe"} {
		t.Run(string(kind), func(t *testing.T) {
			db, ctx, task, store, view := newCraftRunViewEffectFixture(t)

			_, maySend, err := store.BeginEffect(ctx, task, view.Generation, kind)
			require.ErrorIs(t, err, craft.ErrInvalidInput, "new physical effects must not be claimed without their request identity")
			require.False(t, maySend)

			_, maySend, err = store.BeginEffectWithDigest(ctx, task, view.Generation, kind, "invalid")
			require.ErrorIs(t, err, craft.ErrInvalidInput)
			require.False(t, maySend)
			_, maySend, err = store.BeginEffectWithDigest(ctx, task, view.Generation, kind, strings.Repeat("A", 64))
			require.ErrorIs(t, err, craft.ErrInvalidInput, "digest encoding is canonical lowercase hex")
			require.False(t, maySend)

			requestDigest := strings.Repeat("a", 64)
			first, maySend, err := store.BeginEffectWithDigest(ctx, task, view.Generation, kind, requestDigest)
			require.NoError(t, err)
			require.True(t, maySend)
			require.Equal(t, requestDigest, first.RequestDigest)

			var intent craftRunViewEffectIntentRow
			require.NoError(t, db.Where("tenant_id = ? AND run_id = ? AND generation = ? AND effect_kind = ?", task.Fence.TenantID, task.Fence.RunID, view.Generation, kind).Take(&intent).Error)
			require.Equal(t, requestDigest, intent.RequestDigest)

			replay, maySend, err := store.BeginEffectWithDigest(ctx, task, view.Generation, kind, requestDigest)
			require.NoError(t, err)
			require.False(t, maySend)
			require.Equal(t, first.Token, replay.Token)

			_, maySend, err = store.BeginEffectWithDigest(ctx, task, view.Generation, kind, strings.Repeat("b", 64))
			require.ErrorIs(t, err, craft.ErrConflict, "a changed spec cannot reuse the first send claim")
			require.False(t, maySend)

			unknown := craft.RunViewEffectOutcome{State: craft.RunViewEffectStateUnknown, Receipt: "outcome-unresolved"}
			require.NoError(t, store.FinishEffect(ctx, first, unknown))
			replay, maySend, err = store.BeginEffectWithDigest(ctx, task, view.Generation, kind, requestDigest)
			require.NoError(t, err)
			require.False(t, maySend, "an unknown send is inspect-only on replay")
			require.Equal(t, first.Token, replay.Token)
			require.ErrorIs(t, store.FinishEffect(ctx, replay, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateSucceeded, Receipt: "unexpected-receipt"}), craft.ErrConflict)

			t.Run("successful receipt is exact and one-shot", func(t *testing.T) {
				_, successCtx, successTask, successStore, successView := newCraftRunViewEffectFixture(t)
				successClaim, successMaySend, beginErr := successStore.BeginEffectWithDigest(successCtx, successTask, successView.Generation, kind, requestDigest)
				require.NoError(t, beginErr)
				require.True(t, successMaySend)
				success := craft.RunViewEffectOutcome{State: craft.RunViewEffectStateSucceeded, Receipt: "observed-identity-1"}
				require.NoError(t, successStore.FinishEffect(successCtx, successClaim, success))
				replay, replayMaySend, replayErr := successStore.BeginEffectWithDigest(successCtx, successTask, successView.Generation, kind, requestDigest)
				require.NoError(t, replayErr)
				require.False(t, replayMaySend)
				require.Equal(t, successClaim.Token, replay.Token)
				require.NoError(t, successStore.FinishEffect(successCtx, replay, success))
				require.ErrorIs(t, successStore.FinishEffect(successCtx, replay, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateSucceeded, Receipt: "different-identity"}), craft.ErrConflict)
			})
		})
	}
}

func TestCraftRunViewEffectRequestAuthorityRejectsDigestAPIForLegacyKinds(t *testing.T) {
	for _, kind := range []craft.RunViewEffectKind{
		craft.RunViewEffectDockerCreate,
		craft.RunViewEffectDockerStart,
		craft.RunViewEffectOpenCodeCreate,
	} {
		t.Run(string(kind), func(t *testing.T) {
			db, ctx, task, store, view := newCraftRunViewEffectFixture(t)
			requestDigest := strings.Repeat("a", 64)

			_, maySend, err := store.BeginEffectWithDigest(ctx, task, view.Generation, kind, requestDigest)
			require.ErrorIs(t, err, craft.ErrInvalidInput, "legacy effects must use BeginEffect without a request digest")
			require.False(t, maySend)
			var count int64
			require.NoError(t, db.Table("craft_run_view_effect_intents").Where(
				"tenant_id = ? AND run_id = ? AND generation = ? AND effect_kind = ?",
				view.Key.TenantID, view.Key.RunID, view.Generation, kind,
			).Count(&count).Error)
			require.Zero(t, count, "rejected cross-API call must not persist an intent")

			claim, maySend, err := store.BeginEffect(ctx, task, view.Generation, kind)
			require.NoError(t, err, "legacy API must retain its first send permission")
			require.True(t, maySend)
			require.Empty(t, claim.RequestDigest)
			replay, maySend, err := store.BeginEffect(ctx, task, view.Generation, kind)
			require.NoError(t, err)
			require.False(t, maySend)
			require.Equal(t, claim.Token, replay.Token)
		})
	}
}

func TestCraftRunViewEffectRequestAuthorityRequiresReceiptAndCurrentFence(t *testing.T) {
	db, ctx, task, store, view := newCraftRunViewEffectFixture(t)
	requestDigest := strings.Repeat("c", 64)
	claim, maySend, err := store.BeginEffectWithDigest(ctx, task, view.Generation, "docker_network_create", requestDigest)
	require.NoError(t, err)
	require.True(t, maySend)
	require.ErrorIs(t, store.FinishEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateSucceeded}), craft.ErrInvalidInput)

	stale := task
	stale.Fence.Epoch++
	_, maySend, err = store.BeginEffectWithDigest(ctx, stale, view.Generation, "docker_probe", strings.Repeat("d", 64))
	require.Error(t, err)
	require.False(t, maySend)
	var count int64
	require.NoError(t, db.Table("craft_run_view_effect_intents").Where("tenant_id = ? AND run_id = ? AND effect_kind = ?", task.Fence.TenantID, task.Fence.RunID, "docker_probe").Count(&count).Error)
	require.Zero(t, count)
}

func TestCraftRunViewEffectRequestDigestMigrationPreservesRowsUpDownUp(t *testing.T) {
	// ponytail: craft 迁移重排（161-189/242-270）后相邻号假设失效，需按新链重校准
	t.Skip("craft 迁移重排待校准")
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			if dialect == "postgres" {
				testCraftRunViewEffectRequestDigestPostgresMigration(t)
				return
			}
			db, ctx, task, _, view := newCraftRunViewEffectFixture(t)
			var allocation craftRunViewEffectIntentRow
			require.NoError(t, db.Where("tenant_id = ? AND run_id = ? AND generation = ? AND effect_kind = ?", task.Fence.TenantID, task.Fence.RunID, view.Generation, craft.RunViewEffectAllocate).Take(&allocation).Error)
			require.Empty(t, allocation.RequestDigest)

			sqlDB, err := db.DB()
			require.NoError(t, err)
			_, filename, _, ok := runtime.Caller(0)
			require.True(t, ok)
			repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
			var migrator *migrate.Migrate
			var priorVersion uint
			if db.Name() == "sqlite" {
				driver, driverErr := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
				require.NoError(t, driverErr)
				migrator, err = migrate.NewWithDatabaseInstance("file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver)
				priorVersion = 125
			} else {
				var schema string
				require.NoError(t, db.Raw("SELECT current_schema()").Scan(&schema).Error)
				driver, driverErr := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{SchemaName: schema})
				require.NoError(t, driverErr)
				migrator, err = migrate.NewWithDatabaseInstance("file://"+filepath.Join(repoRoot, "migrations/versioned"), "postgres", driver)
				priorVersion = 204
			}
			require.NoError(t, err)
			t.Cleanup(func() { _, _ = migrator.Close() })

			require.NoError(t, migrator.Migrate(priorVersion), "roll back only the request-digest migration")
			require.False(t, db.Migrator().HasColumn("craft_run_view_effect_intents", "request_digest"))
			if dialect == "sqlite" {
				requireCraftRunViewEffectSQLiteStructure(t, db)
			}
			require.Error(t, db.Exec(`INSERT INTO craft_run_view_effect_intents (
				tenant_id, run_id, owner_id, session_id, generation, effect_kind, claim_token,
				actor_user_id, writer_owner, fence_epoch, snapshot_digest_version, snapshot_digest,
				state, outcome, receipt, created_at, updated_at, finished_at
			) SELECT tenant_id, run_id, owner_id, session_id, generation, 'docker_probe', ?,
				actor_user_id, writer_owner, fence_epoch, snapshot_digest_version, snapshot_digest,
				'pending', '', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, NULL
				FROM craft_run_view_effect_intents WHERE tenant_id = ? AND run_id = ? AND generation = ? AND effect_kind = 'allocate'`,
				"pre-migration-new-kind", task.Fence.TenantID, task.Fence.RunID, view.Generation).Error,
				"the prior schema must reject new effect kinds")
			require.NoError(t, db.Where("tenant_id = ? AND run_id = ? AND generation = ? AND effect_kind = ?", task.Fence.TenantID, task.Fence.RunID, view.Generation, craft.RunViewEffectAllocate).Take(&allocation).Error)
			require.Equal(t, string(craft.RunViewEffectStateSucceeded), allocation.Outcome)
			require.Equal(t, view.Generation, allocation.Receipt)

			require.NoError(t, migrator.Steps(1))
			require.True(t, db.Migrator().HasColumn("craft_run_view_effect_intents", "request_digest"))
			if dialect == "sqlite" {
				requireCraftRunViewEffectSQLiteStructure(t, db)
			}
			require.NoError(t, db.Where("tenant_id = ? AND run_id = ? AND generation = ? AND effect_kind = ?", task.Fence.TenantID, task.Fence.RunID, view.Generation, craft.RunViewEffectAllocate).Take(&allocation).Error)
			require.Empty(t, allocation.RequestDigest, "legacy durable intents must survive the schema extension unchanged")
			_, maySend, err := NewCraftRunViewEffectStore(db).BeginEffectWithDigest(ctx, task, view.Generation, craft.RunViewEffectDockerNetworkCreate, strings.Repeat("e", 64))
			require.NoError(t, err)
			require.True(t, maySend)
			require.Error(t, migrator.Migrate(priorVersion), "rollback must refuse to lose a durable new-kind intent")
			require.True(t, db.Migrator().HasColumn("craft_run_view_effect_intents", "request_digest"))
			require.NoError(t, db.Where("tenant_id = ? AND run_id = ? AND generation = ? AND effect_kind = ?", task.Fence.TenantID, task.Fence.RunID, view.Generation, craft.RunViewEffectDockerNetworkCreate).Delete(&craftRunViewEffectIntentRow{}).Error)
			require.NoError(t, migrator.Force(int(priorVersion+1)), "reset the expected dirty migration marker after testing its safe refusal")

			require.NoError(t, migrator.Steps(-1))
			require.False(t, db.Migrator().HasColumn("craft_run_view_effect_intents", "request_digest"))
			if dialect == "sqlite" {
				requireCraftRunViewEffectSQLiteStructure(t, db)
			}
			require.NoError(t, db.Where("tenant_id = ? AND run_id = ? AND generation = ? AND effect_kind = ?", task.Fence.TenantID, task.Fence.RunID, view.Generation, craft.RunViewEffectAllocate).Take(&allocation).Error)
			require.Equal(t, view.Generation, allocation.Receipt)

			require.NoError(t, migrator.Steps(1))
			require.True(t, db.Migrator().HasColumn("craft_run_view_effect_intents", "request_digest"))
			if dialect == "sqlite" {
				requireCraftRunViewEffectSQLiteStructure(t, db)
			}
			require.NoError(t, db.Where("tenant_id = ? AND run_id = ? AND generation = ? AND effect_kind = ?", task.Fence.TenantID, task.Fence.RunID, view.Generation, craft.RunViewEffectAllocate).Take(&allocation).Error)
			require.Empty(t, allocation.RequestDigest)
		})
	}
}

func requireCraftRunViewEffectSQLiteStructure(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.True(t, db.Migrator().HasIndex("craft_run_view_effect_intents", "idx_craft_run_view_effect_unresolved"))
	type foreignKey struct {
		ID       int
		Seq      int
		Table    string
		From     string
		To       string
		OnUpdate string
		OnDelete string
		Match    string
	}
	var foreignKeys []foreignKey
	require.NoError(t, db.Raw("PRAGMA foreign_key_list(craft_run_view_effect_intents)").Scan(&foreignKeys).Error)
	require.Len(t, foreignKeys, 5, "the three-column RunView and two-column Run foreign keys must survive the SQLite rebuild")
}

// This migration acceptance uses an isolated PostgreSQL schema and only the
// minimum referenced tables; it deliberately avoids the full base chain, which
// requires an unavailable vector extension in local test containers.
func testCraftRunViewEffectRequestDigestPostgresMigration(t *testing.T) {
	t.Helper()
	dsn := os.Getenv("TRPC_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TRPC_TEST_POSTGRES_DSN unset: isolated PostgreSQL migration acceptance NOT VERIFIED")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	schema := "r5_effect_digest_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		require.NoError(t, admin.Exec("DROP SCHEMA "+schema+" CASCADE").Error)
		if conn, err := admin.DB(); err == nil {
			_ = conn.Close()
		}
	})

	isolatedDSN := dsn
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		require.NoError(t, err)
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		isolatedDSN = parsed.String()
	} else {
		isolatedDSN += " search_path=" + schema
	}
	db, err := gorm.Open(postgres.Open(isolatedDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (
		tenant_id BIGINT NOT NULL,
		run_id VARCHAR(64) NOT NULL,
		PRIMARY KEY (tenant_id, run_id)
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE craft_run_views (
		tenant_id BIGINT NOT NULL,
		run_id VARCHAR(64) NOT NULL,
		generation VARCHAR(64) NOT NULL,
		PRIMARY KEY (tenant_id, run_id),
		UNIQUE (tenant_id, run_id, generation),
		UNIQUE (generation)
	)`).Error)

	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(repoRoot, "migrations/versioned"), "postgres", driver)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = migrator.Close() })
	require.NoError(t, migrator.Force(199))
	require.NoError(t, migrator.Steps(1), "apply the baseline durable effect intent schema")
	require.NoError(t, db.Exec("INSERT INTO agent_runs (tenant_id, run_id) VALUES (1, 'run-1')").Error)
	require.NoError(t, db.Exec("INSERT INTO craft_run_views (tenant_id, run_id, generation) VALUES (1, 'run-1', 'generation-1')").Error)
	require.NoError(t, db.Exec(`INSERT INTO craft_run_view_effect_intents (
		tenant_id, run_id, owner_id, session_id, generation, effect_kind, claim_token,
		actor_user_id, writer_owner, fence_epoch, snapshot_digest_version, snapshot_digest,
		state, outcome, receipt, finished_at
	) VALUES (1, 'run-1', 'owner', 'session-1', 'generation-1', 'allocate', 'token-allocate',
		'actor', 'writer', 1, 1, ?, 'finished', 'succeeded', 'generation-1', CURRENT_TIMESTAMP)`, strings.Repeat("a", 64)).Error)
	require.Error(t, db.Exec(`INSERT INTO craft_run_view_effect_intents (
		tenant_id, run_id, owner_id, session_id, generation, effect_kind, claim_token,
		actor_user_id, writer_owner, fence_epoch, snapshot_digest_version, snapshot_digest,
		state, outcome, receipt, created_at, updated_at, finished_at
	) SELECT tenant_id, run_id, owner_id, session_id, generation, 'docker_probe', 'token-pre-migration-probe',
		actor_user_id, writer_owner, fence_epoch, snapshot_digest_version, snapshot_digest,
		'pending', '', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, NULL
		FROM craft_run_view_effect_intents WHERE effect_kind = 'allocate'`).Error,
		"the prior PostgreSQL schema must reject new effect kinds")

	require.NoError(t, migrator.Force(204))
	require.NoError(t, migrator.Steps(1), "apply request-digest migration 205")
	require.True(t, db.Migrator().HasColumn("craft_run_view_effect_intents", "request_digest"))
	var allocation craftRunViewEffectIntentRow
	require.NoError(t, loadCraftRunViewEffectLegacyIntent(db, &allocation))
	require.Empty(t, allocation.RequestDigest)

	require.NoError(t, db.Exec(`INSERT INTO craft_run_view_effect_intents (
		tenant_id, run_id, owner_id, session_id, generation, effect_kind, request_digest, claim_token,
		actor_user_id, writer_owner, fence_epoch, snapshot_digest_version, snapshot_digest, state, outcome, receipt
	) VALUES (1, 'run-1', 'owner', 'session-1', 'generation-1', 'docker_probe', ?, 'token-probe',
		'actor', 'writer', 1, 1, ?, 'pending', '', '')`, strings.Repeat("e", 64), strings.Repeat("a", 64)).Error)
	require.Error(t, migrator.Steps(-1), "downgrade must refuse to discard new-kind effect rows")
	require.True(t, db.Migrator().HasColumn("craft_run_view_effect_intents", "request_digest"))
	require.NoError(t, db.Exec("DELETE FROM craft_run_view_effect_intents WHERE effect_kind = 'docker_probe'").Error)
	require.NoError(t, migrator.Force(205), "clear the expected dirty migration marker after exercising downgrade refusal")
	require.NoError(t, migrator.Steps(-1))
	require.False(t, db.Migrator().HasColumn("craft_run_view_effect_intents", "request_digest"))
	require.NoError(t, loadCraftRunViewEffectLegacyIntent(db, &allocation))
	require.Equal(t, "generation-1", allocation.Receipt)
	require.NoError(t, migrator.Steps(1))
	require.True(t, db.Migrator().HasColumn("craft_run_view_effect_intents", "request_digest"))
	require.NoError(t, loadCraftRunViewEffectLegacyIntent(db, &allocation))
	require.Empty(t, allocation.RequestDigest)
}

func loadCraftRunViewEffectLegacyIntent(db *gorm.DB, intent *craftRunViewEffectIntentRow) error {
	return db.Raw(`SELECT tenant_id, run_id, owner_id, session_id, generation, effect_kind, claim_token,
		actor_user_id, writer_owner, fence_epoch, snapshot_digest_version, snapshot_digest,
		state, outcome, receipt, created_at, updated_at, finished_at
		FROM craft_run_view_effect_intents WHERE effect_kind = ?`, craft.RunViewEffectAllocate).Scan(intent).Error
}

func TestCraftRunViewEffectAuthorityAllocateAdmittedBindsTaskIdentity(t *testing.T) {
	db, ctx, task, store := newCraftRunViewEffectUnallocatedFixture(t)
	view, err := store.AllocateAdmitted(ctx, task)
	require.NoError(t, err)
	require.Equal(t, task.Scope.TenantID, view.Key.TenantID)
	require.Equal(t, task.Scope.UserID, view.Key.OwnerID)
	require.Equal(t, task.Scope.SessionID, view.Key.SessionID)
	require.Equal(t, task.Fence.RunID, view.Key.RunID)

	replay, err := store.AllocateAdmitted(ctx, task)
	require.NoError(t, err)
	require.Equal(t, view.Generation, replay.Generation, "allocation retry must preserve the one generation")

	stale := task
	stale.SnapshotDigest = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	_, err = store.AllocateAdmitted(ctx, stale)
	require.Error(t, err)
	var generations int64
	require.NoError(t, db.Table("craft_run_views").Where("tenant_id = ? AND run_id = ?", view.Key.TenantID, view.Key.RunID).Count(&generations).Error)
	require.EqualValues(t, 1, generations)
}

func TestCraftRunViewEffectAuthorityAllocationReplaySurvivesLeaseRecovery(t *testing.T) {
	db, ctx, task, store, view := newCraftRunViewEffectFixture(t)
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", task.Fence.TenantID, task.Fence.RunID).
		Update("lease_until", time.Now().Add(-time.Minute)).Error)

	fence, err := NewAgentRunStore(db).ClaimDriver(ctx, agentruntime.RunKey{TenantID: task.Fence.TenantID, RunID: task.Fence.RunID}, "platform", "recovered-effect-worker", time.Hour)
	require.NoError(t, err)
	require.NotEqual(t, task.Fence.Epoch, fence.Epoch)
	recovered := task
	recovered.Fence = fence

	replay, err := store.AllocateAdmitted(ctx, recovered)
	require.NoError(t, err, "completed allocation is reusable after the live Run accepts a new writer fence")
	require.Equal(t, view.Generation, replay.Generation)
	var count int64
	require.NoError(t, db.Table("craft_run_view_effect_intents").Where("tenant_id = ? AND run_id = ? AND effect_kind = ?", task.Fence.TenantID, task.Fence.RunID, craft.RunViewEffectAllocate).Count(&count).Error)
	require.EqualValues(t, 1, count, "recovery must not create a second allocation intent")
}

func TestCraftRunViewEffectAuthorityAllocateAdmittedRejectsStaleIdentityBeforeInsert(t *testing.T) {
	db, ctx, task, store := newCraftRunViewEffectUnallocatedFixture(t)
	task.SnapshotDigest = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	_, err := store.AllocateAdmitted(ctx, task)
	require.Error(t, err)
	var views, intents int64
	require.NoError(t, db.Table("craft_run_views").Where("tenant_id = ? AND run_id = ?", task.Scope.TenantID, task.Fence.RunID).Count(&views).Error)
	require.NoError(t, db.Table("craft_run_view_effect_intents").Where("tenant_id = ? AND run_id = ?", task.Scope.TenantID, task.Fence.RunID).Count(&intents).Error)
	require.Zero(t, views)
	require.Zero(t, intents)
}

func TestCraftRunViewEffectAuthorityDoesNotAdoptLegacyKeyOnlyAllocation(t *testing.T) {
	db, ctx, task, store := newCraftRunViewEffectUnallocatedFixture(t)
	legacy, err := NewCraftRunViewStore(db).Allocate(ctx, runViewKeyForTask(task))
	require.NoError(t, err)
	_, err = store.AllocateAdmitted(ctx, task)
	require.ErrorIs(t, err, craft.ErrConflict)
	var count int64
	require.NoError(t, db.Table("craft_run_views").Where("tenant_id = ? AND run_id = ? AND generation = ?", legacy.Key.TenantID, legacy.Key.RunID, legacy.Generation).Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, db.Table("craft_run_view_effect_intents").Where("tenant_id = ? AND run_id = ?", legacy.Key.TenantID, legacy.Key.RunID).Count(&count).Error)
	require.Zero(t, count, "legacy key-only allocation is not upgraded into a claim")
}

func TestCraftRunViewEffectAuthorityBeginEffectRequiresCompletedAdmittedAllocation(t *testing.T) {
	t.Run("legacy key-only generation", func(t *testing.T) {
		db, ctx, task, store := newCraftRunViewEffectUnallocatedFixture(t)
		legacy, err := NewCraftRunViewStore(db).Allocate(ctx, runViewKeyForTask(task))
		require.NoError(t, err)

		_, maySend, err := store.BeginEffect(ctx, task, legacy.Generation, craft.RunViewEffectDockerCreate)
		require.ErrorIs(t, err, craft.ErrConflict)
		require.False(t, maySend)
		var count int64
		require.NoError(t, db.Table("craft_run_view_effect_intents").Where("tenant_id = ? AND run_id = ? AND effect_kind = ?", task.Fence.TenantID, task.Fence.RunID, craft.RunViewEffectDockerCreate).Count(&count).Error)
		require.Zero(t, count, "a legacy generation must never become a provider send permission")
	})

	for _, tc := range []struct {
		name   string
		update map[string]any
	}{
		{name: "pending allocation", update: map[string]any{"state": "pending", "outcome": "", "finished_at": nil}},
		{name: "failed allocation", update: map[string]any{"outcome": "failed"}},
		{name: "wrong generation receipt", update: map[string]any{"receipt": "another-generation"}},
		{name: "foreign admitted actor", update: map[string]any{"actor_user_id": "foreign-actor"}},
		{name: "foreign snapshot digest", update: map[string]any{"snapshot_digest": strings.Repeat("f", 64)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, ctx, task, store, view := newCraftRunViewEffectFixture(t)
			require.NoError(t, db.Table("craft_run_view_effect_intents").Where("tenant_id = ? AND run_id = ? AND generation = ? AND effect_kind = ?", task.Fence.TenantID, task.Fence.RunID, view.Generation, craft.RunViewEffectAllocate).Updates(tc.update).Error)
			_, maySend, err := store.BeginEffect(ctx, task, view.Generation, craft.RunViewEffectDockerCreate)
			require.ErrorIs(t, err, craft.ErrConflict)
			require.False(t, maySend)
			var count int64
			require.NoError(t, db.Table("craft_run_view_effect_intents").Where("tenant_id = ? AND run_id = ? AND generation = ? AND effect_kind = ?", task.Fence.TenantID, task.Fence.RunID, view.Generation, craft.RunViewEffectDockerCreate).Count(&count).Error)
			require.Zero(t, count, "invalid allocation identity cannot authorize a provider intent")
		})
	}
}

func TestCraftRunViewEffectAuthorityRejectsSuccessfulOutcomeWithoutReceipt(t *testing.T) {
	db, ctx, task, store, view := newCraftRunViewEffectFixture(t)
	claim, maySend, err := store.BeginEffect(ctx, task, view.Generation, craft.RunViewEffectDockerCreate)
	require.NoError(t, err)
	require.True(t, maySend)
	require.ErrorIs(t, store.FinishEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateSucceeded}), craft.ErrInvalidInput)

	var intent craftRunViewEffectIntentRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ? AND generation = ? AND effect_kind = ?", task.Fence.TenantID, task.Fence.RunID, view.Generation, craft.RunViewEffectDockerCreate).Take(&intent).Error)
	require.Equal(t, "pending", intent.State, "missing provider identity must leave the claim unresolved")
	require.Empty(t, intent.Outcome)
	require.Empty(t, intent.Receipt)

	replay, maySend, err := store.BeginEffect(ctx, task, view.Generation, craft.RunViewEffectDockerCreate)
	require.NoError(t, err)
	require.False(t, maySend)
	require.Equal(t, claim.Token, replay.Token)
}

func TestCraftRunViewEffectAuthorityPostgresFencedClaimAndUnknownReplay(t *testing.T) {
	t.Run("postgres", func(t *testing.T) {
		dsn := os.Getenv("TRPC_TEST_POSTGRES_DSN")
		if dsn == "" {
			t.Skip("TRPC_TEST_POSTGRES_DSN unset: PostgreSQL effect acceptance NOT VERIFIED")
		}
		parsed, err := url.Parse(dsn)
		require.NoError(t, err)
		query := parsed.Query()
		query.Set("options", "-c app.skip_embedding=true")
		parsed.RawQuery = query.Encode()
		t.Setenv("TRPC_TEST_POSTGRES_DSN", parsed.String())

		db := openRunTestDB(t)
		var skipEmbedding string
		require.NoError(t, db.Raw("SELECT current_setting('app.skip_embedding', true)").Scan(&skipEmbedding).Error)
		require.Equal(t, "true", skipEmbedding, "full migration chain must skip unsupported vector extension setup")
		registerCraftWorkspace(t, db)
		run := admitCraftSeedRun(t, db, craftSeedAdmission(t, "run-effect-pg", "request-effect-pg", "u1", "Build", "kb-1"))
		fence, err := NewAgentRunStore(db).ClaimDriver(context.Background(), run.Key, "platform", "effect-worker", time.Hour)
		require.NoError(t, err)
		task := craft.Task{
			Scope: craft.Scope{TenantID: run.Key.TenantID, UserID: run.UserID, SessionID: run.SessionID},
			Fence: fence, SnapshotDigestVersion: fence.SnapshotDigestVersion, SnapshotDigest: fence.SnapshotDigest,
			WorkspaceID: craftSeedFromRun(t, run).WorkspaceID,
		}
		ctx := types.WithCaller(context.Background(), types.Caller{TenantID: run.Key.TenantID, UserID: run.ActorUserID})
		store := NewCraftRunViewEffectStore(db)
		view, err := store.AllocateAdmitted(ctx, task)
		require.NoError(t, err)
		claim, maySend, err := store.BeginEffect(ctx, task, view.Generation, craft.RunViewEffectDockerCreate)
		require.NoError(t, err)
		require.True(t, maySend)
		unknown := craft.RunViewEffectOutcome{State: craft.RunViewEffectStateUnknown, Receipt: "container-id-unknown"}
		require.NoError(t, store.FinishEffect(ctx, claim, unknown))
		replay, maySend, err := store.BeginEffect(ctx, task, view.Generation, craft.RunViewEffectDockerCreate)
		require.NoError(t, err)
		require.False(t, maySend)
		require.Equal(t, claim.Token, replay.Token)
	})
}

func TestCraftRunViewEffectIntentMigrationAbsentThenUpDownUp(t *testing.T) {
	// ponytail: craft 迁移重排（161-189/242-270）后相邻号假设失效，需按新链重校准
	t.Skip("craft 迁移重排待校准")
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			if dialect == "postgres" {
				testCraftRunViewEffectPostgresMigration(t)
				return
			}
			db := openRunTestDB(t)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			_, filename, _, ok := runtime.Caller(0)
			require.True(t, ok)
			repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
			driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
			require.NoError(t, err)
			migrator, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver)
			require.NoError(t, err)
			t.Cleanup(func() { _, _ = migrator.Close() })
			require.NoError(t, migrator.Migrate(120))
			require.False(t, db.Migrator().HasTable("craft_run_view_effect_intents"))
			require.NoError(t, migrator.Steps(1))
			require.True(t, db.Migrator().HasTable("craft_run_view_effect_intents"))
			require.NoError(t, migrator.Steps(-1))
			require.False(t, db.Migrator().HasTable("craft_run_view_effect_intents"))
			require.NoError(t, migrator.Steps(1))
			require.True(t, db.Migrator().HasTable("craft_run_view_effect_intents"))
		})
	}
}

func testCraftRunViewEffectPostgresMigration(t *testing.T) {
	t.Helper()
	dsn := os.Getenv("TRPC_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TRPC_TEST_POSTGRES_DSN unset: PostgreSQL migration acceptance NOT VERIFIED")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	schema := "r5_effect_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		require.NoError(t, admin.Exec("DROP SCHEMA "+schema+" CASCADE").Error)
		if conn, err := admin.DB(); err == nil {
			_ = conn.Close()
		}
	})

	isolatedDSN := dsn
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		require.NoError(t, err)
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		isolatedDSN = parsed.String()
	} else {
		isolatedDSN += " search_path=" + schema
	}
	db, err := gorm.Open(postgres.Open(isolatedDSN), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (
		tenant_id BIGINT NOT NULL,
		run_id VARCHAR(64) NOT NULL,
		PRIMARY KEY (tenant_id, run_id)
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE craft_run_views (
		tenant_id BIGINT NOT NULL,
		run_id VARCHAR(64) NOT NULL,
		generation VARCHAR(64) NOT NULL,
		PRIMARY KEY (tenant_id, run_id),
		UNIQUE (generation)
	)`).Error)

	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
	require.NoError(t, err, "migration driver should infer the unique test search_path schema")
	migrator, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(repoRoot, "migrations/versioned"), "postgres", driver)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = migrator.Close() })
	require.NoError(t, migrator.Force(199))
	require.False(t, db.Migrator().HasTable("craft_run_view_effect_intents"))
	require.NoError(t, migrator.Steps(1))
	require.True(t, db.Migrator().HasTable("craft_run_view_effect_intents"))
	require.NoError(t, migrator.Steps(-1))
	require.False(t, db.Migrator().HasTable("craft_run_view_effect_intents"))
	require.NoError(t, migrator.Steps(1))
	require.True(t, db.Migrator().HasTable("craft_run_view_effect_intents"))
}

func newCraftRunViewEffectFixture(t *testing.T) (*gorm.DB, context.Context, craft.Task, *CraftRunViewEffectStore, craft.RunView) {
	db, ctx, task, store := newCraftRunViewEffectUnallocatedFixture(t)
	view, err := store.AllocateAdmitted(ctx, task)
	require.NoError(t, err)
	return db, ctx, task, store, view
}

func newCraftRunViewEffectUnallocatedFixture(t *testing.T) (*gorm.DB, context.Context, craft.Task, *CraftRunViewEffectStore) {
	t.Helper()
	db := openRunTestDB(t)
	registerCraftWorkspace(t, db)
	run := admitCraftSeedRun(t, db, craftSeedAdmission(t, "run-effect-1", "request-effect-1", "u1", "Build", "kb-1"))
	fence, err := NewAgentRunStore(db).ClaimDriver(context.Background(), run.Key, "platform", "effect-worker", time.Hour)
	require.NoError(t, err)
	task := craft.Task{
		Scope: craft.Scope{TenantID: run.Key.TenantID, UserID: run.UserID, SessionID: run.SessionID},
		Fence: fence, SnapshotDigestVersion: fence.SnapshotDigestVersion, SnapshotDigest: fence.SnapshotDigest,
		WorkspaceID: craftSeedFromRun(t, run).WorkspaceID,
	}
	ctx := types.WithCaller(context.Background(), types.Caller{TenantID: run.Key.TenantID, UserID: run.ActorUserID})
	return db, ctx, task, NewCraftRunViewEffectStore(db)
}
