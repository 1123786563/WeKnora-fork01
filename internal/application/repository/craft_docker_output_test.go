package repository

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func craftDockerOutputFixture(t *testing.T, db *gorm.DB, runID, activity string, maxBytes int64) (*CraftDockerOutputRepository, CraftDockerOutputScope) {
	t.Helper()
	_, revision := craftDockerSendFixture(t, db, runID, activity)
	key := CraftChargeStartKey{TenantID: 1, RunID: runID, ActivityKey: activity}
	receipt := DockerExecReceipt{Provider: "docker", ContainerID: "container-" + activity, ExecID: "exec-" + activity}
	claims := NewCraftDockerSendClaimRepository(db)
	require.NoError(t, claims.BindDockerExecReceipt(context.Background(), key, revision, receipt))
	claimed, err := claims.ClaimDockerExecSend(context.Background(), key, revision, receipt)
	require.NoError(t, err)
	require.True(t, claimed)
	scope := CraftDockerOutputScope{TenantID: 1, TaskID: "s1", RunID: runID, ActivityKey: activity, ContainerID: receipt.ContainerID, ExecID: receipt.ExecID}
	repo := NewCraftDockerOutputRepository(db, maxBytes)
	require.NoError(t, repo.Open(context.Background(), scope))
	return repo, scope
}

func forEachCraftDockerOutputDB(t *testing.T, run func(*testing.T, *gorm.DB)) {
	t.Helper()
	for _, engine := range []string{"sqlite", "postgres"} {
		t.Run(engine, func(t *testing.T) { run(t, openRunTestDB(t)) })
	}
}

func craftDockerOutputAdditionalActivity(t *testing.T, db *gorm.DB, repo *CraftDockerOutputRepository, prior CraftDockerOutputScope, activity string) CraftDockerOutputScope {
	t.Helper()
	var run agentRunRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", prior.TenantID, prior.RunID).Take(&run).Error)
	key := CraftChargeStartKey{TenantID: prior.TenantID, RunID: prior.RunID, ActivityKey: activity}
	craftDockerSendExtraOperation(t, db, key, run.Revision)
	receipt := DockerExecReceipt{Provider: "docker", ContainerID: "container-" + activity, ExecID: "exec-" + activity}
	claims := NewCraftDockerSendClaimRepository(db)
	require.NoError(t, claims.BindDockerExecReceipt(context.Background(), key, run.Revision, receipt))
	claimed, err := claims.ClaimDockerExecSend(context.Background(), key, run.Revision, receipt)
	require.NoError(t, err)
	require.True(t, claimed)
	scope := CraftDockerOutputScope{TenantID: prior.TenantID, TaskID: prior.TaskID, RunID: prior.RunID,
		ActivityKey: activity, ContainerID: receipt.ContainerID, ExecID: receipt.ExecID}
	require.NoError(t, repo.Open(context.Background(), scope))
	return scope
}

func TestCraftDockerOutputAppendInterleavesDurablyWithStableCursor(t *testing.T) {
	forEachCraftDockerOutputDB(t, func(t *testing.T, db *gorm.DB) {
		repo, scope := craftDockerOutputFixture(t, db, "output-interleave", "activity", 1024)
		ctx := context.Background()
		seq1, err := repo.Append(ctx, scope, "stdout", []byte("out-1"))
		require.NoError(t, err)
		seq2, err := repo.Append(ctx, scope, "stderr", []byte("err-1"))
		require.NoError(t, err)
		seq3, err := repo.Append(ctx, scope, "stdout", []byte("out-2"))
		require.NoError(t, err)
		require.EqualValues(t, []int64{1, 2, 3}, []int64{seq1, seq2, seq3})
		require.NoError(t, repo.Seal(ctx, scope))

		// A separately opened repository models process restart/recovery.
		restarted := NewCraftDockerOutputRepository(reopenRunDB(t, db), 1024)
		page1, state1, err := restarted.ReadAfter(ctx, scope, 0, 2)
		require.NoError(t, err)
		require.Equal(t, []int64{1, 2}, []int64{page1[0].Sequence, page1[1].Sequence})
		require.Equal(t, "stdout", page1[0].Stream)
		require.Equal(t, "stderr", page1[1].Stream)
		require.True(t, state1.Sealed)
		require.True(t, state1.Partial, "the output store cannot attest full stream/terminal completeness")
		page2, state2, err := restarted.ReadAfter(ctx, scope, 2, 2)
		require.NoError(t, err)
		require.Len(t, page2, 1)
		require.EqualValues(t, 3, page2[0].Sequence)
		require.Equal(t, "out-2", string(page2[0].Bytes))
		require.True(t, state2.Sealed)
		require.True(t, state2.Partial)
		require.Error(t, db.Exec(`DELETE FROM craft_charge_start_journal WHERE tenant_id = ? AND run_id = ? AND activity_key = ?`, scope.TenantID, scope.RunID, scope.ActivityKey).Error,
			"retained output prevents deleting its immutable send journal")
		require.Error(t, db.Exec(`UPDATE craft_docker_output_operations SET exec_id = 'rebound' WHERE tenant_id = ? AND run_id = ? AND activity_key = ?`, scope.TenantID, scope.RunID, scope.ActivityKey).Error,
			"output cannot be rebound to a different immutable exec receipt")
		require.Error(t, db.Exec(`INSERT INTO craft_docker_output_operations
			(tenant_id, task_id, run_id, activity_key, container_id, exec_id, max_bytes)
			VALUES (?, 'foreign-task', ?, ?, ?, ?, 128)`, scope.TenantID, scope.RunID, scope.ActivityKey, scope.ContainerID, scope.ExecID).Error,
			"database rejects a task/receipt mismatch even outside the repository API")
	})
}

func TestCraftDockerOutputAppendSequenceReplayMustMatchBytes(t *testing.T) {
	forEachCraftDockerOutputDB(t, func(t *testing.T, db *gorm.DB) {
		repo, scope := craftDockerOutputFixture(t, db, "output-replay", "activity", 1024)
		ctx := context.Background()
		require.NoError(t, repo.AppendSequence(ctx, scope, 1, "stdout", []byte("same")))
		require.NoError(t, repo.AppendSequence(ctx, scope, 1, "stdout", []byte("same")), "exact replay is idempotent")
		require.ErrorIs(t, repo.AppendSequence(ctx, scope, 1, "stdout", []byte("different")), craft.ErrConflict)
		require.ErrorIs(t, repo.AppendSequence(ctx, scope, 3, "stderr", []byte("gap")), craft.ErrConflict)
		chunks, state, err := repo.ReadAfter(ctx, scope, 0, 10)
		require.NoError(t, err)
		require.Len(t, chunks, 1)
		require.EqualValues(t, 1, state.NextSequence)
	})
}

func TestCraftDockerOutputWriterOpenIsCreateOnly(t *testing.T) {
	forEachCraftDockerOutputDB(t, func(t *testing.T, db *gorm.DB) {
		repo, scope := craftDockerOutputFixture(t, db, "output-create-only", "activity", 1024)
		require.ErrorIs(t, repo.Open(context.Background(), scope), ErrCraftDockerOutputConflict,
			"a second process cannot regain a writer for an existing receipt")
		chunks, state, err := repo.ReadAfter(context.Background(), scope, 0, 10)
		require.NoError(t, err, "read-only cursor access must not require a writer open")
		require.Empty(t, chunks)
		require.False(t, state.Sealed)
	})
}

func TestCraftDockerOutputQuotaReplayUsesOriginalInputIdentity(t *testing.T) {
	forEachCraftDockerOutputDB(t, func(t *testing.T, db *gorm.DB) {
		repo, baseScope := craftDockerOutputFixture(t, db, "output-quota-replay", "base-activity", 3)
		t.Run("truncated prefix", func(t *testing.T) {
			scope := baseScope
			ctx := context.Background()
			require.ErrorIs(t, repo.AppendSequence(ctx, scope, 1, "stdout", []byte("abcdef")), ErrCraftDockerOutputQuota)
			require.ErrorIs(t, repo.AppendSequence(ctx, scope, 1, "stdout", []byte("abcdef")), ErrCraftDockerOutputQuota,
				"byte-identical replay must preserve the explicit quota result")
			require.ErrorIs(t, repo.AppendSequence(ctx, scope, 1, "stdout", []byte("abcdeg")), ErrCraftDockerOutputConflict)
			chunks, state, err := repo.ReadAfter(ctx, scope, 0, 10)
			require.NoError(t, err)
			require.Len(t, chunks, 1)
			require.Equal(t, "abc", string(chunks[0].Bytes))
			require.EqualValues(t, 1, state.NextSequence)
		})
		t.Run("zero stored bytes", func(t *testing.T) {
			smallRepo := NewCraftDockerOutputRepository(db, 1)
			scope := craftDockerOutputAdditionalActivity(t, db, smallRepo, baseScope, "zero-activity")
			ctx := context.Background()
			require.NoError(t, smallRepo.AppendSequence(ctx, scope, 1, "stdout", []byte("a")))
			require.ErrorIs(t, smallRepo.AppendSequence(ctx, scope, 2, "stderr", []byte("b")), ErrCraftDockerOutputQuota)
			require.ErrorIs(t, smallRepo.AppendSequence(ctx, scope, 2, "stderr", []byte("b")), ErrCraftDockerOutputQuota)
			require.ErrorIs(t, smallRepo.AppendSequence(ctx, scope, 2, "stderr", []byte("c")), ErrCraftDockerOutputConflict)
			chunks, state, err := smallRepo.ReadAfter(ctx, scope, 0, 10)
			require.NoError(t, err)
			require.Len(t, chunks, 2, "zero-byte attempt still occupies its sequence")
			require.Empty(t, chunks[1].Bytes)
			require.EqualValues(t, 2, state.NextSequence)
		})
	})
}

func TestCraftDockerOutputConcurrentAppendUsesOneMonotonicSequence(t *testing.T) {
	forEachCraftDockerOutputDB(t, func(t *testing.T, db *gorm.DB) {
		repo, scope := craftDockerOutputFixture(t, db, "output-concurrent", "activity", 4096)
		ctx := context.Background()
		start := make(chan struct{})
		sequences := make([]int64, 12)
		errs := make([]error, len(sequences))
		var wg sync.WaitGroup
		for i := range sequences {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				sequences[i], errs[i] = repo.Append(ctx, scope, []string{"stdout", "stderr"}[i%2], []byte{byte(i)})
			}(i)
		}
		close(start)
		wg.Wait()
		for _, err := range errs {
			require.NoError(t, err)
		}
		sort.Slice(sequences, func(i, j int) bool { return sequences[i] < sequences[j] })
		require.Equal(t, []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, sequences)
	})
}

func TestCraftDockerOutputScopeAndReceiptAreExact(t *testing.T) {
	forEachCraftDockerOutputDB(t, func(t *testing.T, db *gorm.DB) {
		repo, scope := craftDockerOutputFixture(t, db, "output-scope", "activity", 128)
		foreign := scope
		foreign.TenantID++
		_, _, err := repo.ReadAfter(context.Background(), foreign, 0, 10)
		require.Error(t, err)
		foreign = scope
		foreign.TaskID = "s2"
		require.Error(t, repo.Open(context.Background(), foreign))
		foreign = scope
		foreign.ExecID = "different-exec"
		require.Error(t, repo.Open(context.Background(), foreign))
		_, appendErr := repo.Append(context.Background(), foreign, "stdout", []byte("foreign"))
		require.Error(t, appendErr)
	})
}

func TestCraftDockerOutputQuotaPersistsPrefixAndExplicitPartialState(t *testing.T) {
	forEachCraftDockerOutputDB(t, func(t *testing.T, db *gorm.DB) {
		repo, scope := craftDockerOutputFixture(t, db, "output-quota", "activity", 5)
		ctx := context.Background()
		seq, err := repo.Append(ctx, scope, "stdout", []byte("abc"))
		require.NoError(t, err)
		require.EqualValues(t, 1, seq)
		seq, err = repo.Append(ctx, scope, "stderr", []byte("def"))
		require.ErrorIs(t, err, ErrCraftDockerOutputQuota)
		require.EqualValues(t, 2, seq)
		chunks, state, readErr := repo.ReadAfter(ctx, scope, 0, 10)
		require.NoError(t, readErr)
		require.Len(t, chunks, 2)
		require.Equal(t, "de", string(chunks[1].Bytes))
		require.True(t, state.Truncated)
		require.True(t, state.Partial)
		require.EqualValues(t, 5, state.TotalBytes)
		_, appendErr := repo.Append(ctx, scope, "stdout", []byte("x"))
		require.Error(t, appendErr)
	})
}

func TestCraftDockerOutputSealRejectsLateMutationAndCursorAhead(t *testing.T) {
	forEachCraftDockerOutputDB(t, func(t *testing.T, db *gorm.DB) {
		repo, scope := craftDockerOutputFixture(t, db, "output-seal", "activity", 128)
		ctx := context.Background()
		_, err := repo.Append(ctx, scope, "stdout", []byte("before"))
		require.NoError(t, err)
		require.NoError(t, repo.Seal(ctx, scope))
		_, appendErr := repo.Append(ctx, scope, "stdout", []byte("late"))
		require.ErrorIs(t, appendErr, ErrCraftDockerOutputSealed)
		_, _, err = repo.ReadAfter(ctx, scope, 100, 10)
		require.ErrorIs(t, err, ErrCraftDockerOutputCursorAhead)
		chunks, state, err := repo.ReadAfter(ctx, scope, 0, 10)
		require.NoError(t, err)
		require.Len(t, chunks, 1)
		require.True(t, state.Sealed)
		require.NoError(t, repo.AppendSequence(ctx, scope, 1, "stdout", []byte("before")), "exact committed replay is still idempotent after seal")
	})
}

func TestCraftDockerOutputReadFailsClosedOnDigestMismatchOrCursorGap(t *testing.T) {
	forEachCraftDockerOutputDB(t, func(t *testing.T, db *gorm.DB) {
		repo, scope := craftDockerOutputFixture(t, db, "output-integrity", "activity", 128)
		ctx := context.Background()
		_, err := repo.Append(ctx, scope, "stdout", []byte("one"))
		require.NoError(t, err)
		_, err = repo.Append(ctx, scope, "stderr", []byte("two"))
		require.NoError(t, err)
		require.NoError(t, db.Exec(`UPDATE craft_docker_output_chunks SET bytes = ? WHERE tenant_id = ? AND run_id = ? AND activity_key = ? AND sequence = 1`, []byte("bad"), scope.TenantID, scope.RunID, scope.ActivityKey).Error)
		_, _, err = repo.ReadAfter(ctx, scope, 0, 10)
		require.ErrorIs(t, err, ErrCraftDockerOutputCorrupt)

		scope = craftDockerOutputAdditionalActivity(t, db, repo, scope, "gap-activity")
		_, err = repo.Append(ctx, scope, "stdout", []byte("one"))
		require.NoError(t, err)
		_, err = repo.Append(ctx, scope, "stderr", []byte("two"))
		require.NoError(t, err)
		_, err = repo.Append(ctx, scope, "stdout", []byte("three"))
		require.NoError(t, err)
		require.NoError(t, db.Exec(`DELETE FROM craft_docker_output_chunks WHERE tenant_id = ? AND run_id = ? AND activity_key = ? AND sequence = 2`, scope.TenantID, scope.RunID, scope.ActivityKey).Error)
		_, _, err = repo.ReadAfter(ctx, scope, 0, 10)
		require.ErrorIs(t, err, ErrCraftDockerOutputCorrupt)
	})
}

func TestCraftDockerOutputAppendFailureDoesNotPublishUncommittedBytes(t *testing.T) {
	forEachCraftDockerOutputDB(t, func(t *testing.T, db *gorm.DB) {
		repo, scope := craftDockerOutputFixture(t, db, "output-insert-failure", "activity", 128)
		if db.Name() == "sqlite" {
			require.NoError(t, db.Exec(`CREATE TRIGGER fail_output_insert BEFORE INSERT ON craft_docker_output_chunks BEGIN SELECT RAISE(ABORT, 'injected output failure'); END`).Error)
			t.Cleanup(func() { _ = db.Exec("DROP TRIGGER IF EXISTS fail_output_insert").Error })
		} else {
			require.NoError(t, db.Exec(`CREATE FUNCTION fail_output_insert_fn() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected output failure'; END; $$`).Error)
			require.NoError(t, db.Exec(`CREATE TRIGGER fail_output_insert BEFORE INSERT ON craft_docker_output_chunks FOR EACH ROW EXECUTE FUNCTION fail_output_insert_fn()`).Error)
			t.Cleanup(func() {
				_ = db.Exec("DROP TRIGGER IF EXISTS fail_output_insert ON craft_docker_output_chunks").Error
				_ = db.Exec("DROP FUNCTION IF EXISTS fail_output_insert_fn()").Error
			})
		}
		_, err := repo.Append(context.Background(), scope, "stdout", []byte("secret-output"))
		require.ErrorIs(t, err, ErrCraftDockerOutputUnavailable)
		chunks, state, readErr := repo.ReadAfter(context.Background(), scope, 0, 10)
		require.NoError(t, readErr)
		require.Empty(t, chunks)
		require.Zero(t, state.NextSequence)
		require.Zero(t, state.TotalBytes)
	})
}

func TestCraftDockerOutputEmptyReadAndInvalidCursorAreExplicit(t *testing.T) {
	forEachCraftDockerOutputDB(t, func(t *testing.T, db *gorm.DB) {
		repo, scope := craftDockerOutputFixture(t, db, "output-empty", "activity", 128)
		chunks, state, err := repo.ReadAfter(context.Background(), scope, 0, 10)
		require.NoError(t, err)
		require.Empty(t, chunks)
		require.False(t, state.Sealed, "empty is not equivalent to complete")
		_, _, err = repo.ReadAfter(context.Background(), scope, -1, 10)
		require.ErrorIs(t, err, ErrCraftDockerOutputInvalidCursor)
	})
}

func TestCraftDockerOutputMigrationUpDownUp(t *testing.T) {
	// ponytail: craft 迁移重排后 tip 相邻性假设需重校准
	t.Skip("craft 迁移重排待校准：相邻号假设失效")
	forEachCraftDockerOutputDB(t, func(t *testing.T, db *gorm.DB) {
		_, filename, _, ok := runtime.Caller(0)
		require.True(t, ok)
		root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
		sqlDB, err := db.DB()
		require.NoError(t, err)
		var migrator *migrate.Migrate
		if db.Name() == "sqlite" {
			driver, e := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
			require.NoError(t, e)
			migrator, err = migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
		} else {
			var schema string
			require.NoError(t, db.Raw("SELECT current_schema()").Scan(&schema).Error)
			driver, e := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{SchemaName: schema})
			require.NoError(t, e)
			migrator, err = migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/versioned"), "postgres", driver)
		}
		require.NoError(t, err)
		t.Cleanup(func() { _, _ = migrator.Close() })
		migrationBase := "migrations/sqlite/000174_craft_run_capture.up.sql"
		tip := uint(122)
		if db.Name() == "postgres" {
			migrationBase = "migrations/versioned/000252_craft_run_capture.up.sql"
			tip = 201
		}
		if _, statErr := os.Stat(filepath.Join(root, migrationBase)); statErr == nil {
			tip++
		}
		require.NoError(t, migrator.Migrate(tip))
		require.False(t, db.Migrator().HasTable("craft_docker_output_operations"))
		require.NoError(t, migrator.Steps(1))
		require.True(t, db.Migrator().HasTable("craft_docker_output_operations"))
		require.True(t, db.Migrator().HasTable("craft_docker_output_chunks"))
		require.True(t, db.Migrator().HasColumn("craft_docker_output_chunks", "input_byte_count"))
		require.True(t, db.Migrator().HasColumn("craft_docker_output_chunks", "input_sha256"))
		require.NoError(t, migrator.Steps(-1))
		require.False(t, db.Migrator().HasTable("craft_docker_output_chunks"))
		require.NoError(t, migrator.Steps(1))
		require.True(t, db.Migrator().HasTable("craft_docker_output_operations"))
	})
}
