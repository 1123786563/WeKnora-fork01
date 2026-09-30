package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestCraftDockerNormalInputStagesEncryptedCreateOnlyRequest(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	forEachCraftDockerNormalInputDB(t, func(t *testing.T, db *gorm.DB) {
		ctx := context.Background()
		key, _ := craftDockerNormalInputFixtureForDB(t, db, "normal-stage", "activity")
		store := NewCraftDockerNormalInputRepository(db)
		request := craftDockerNormalInputFixture(key)
		staged, err := store.Stage(ctx, request)
		require.NoError(t, err)
		require.Equal(t, craftNormalSHA256Hex(request.Stdin), staged.StdinSHA256)
		require.EqualValues(t, len(request.Stdin), staged.StdinByteCount)
		replayed, err := store.Stage(ctx, request)
		require.NoError(t, err)
		require.Equal(t, staged.RequestSHA256, replayed.RequestSHA256)

		var persisted string
		require.NoError(t, db.Table("craft_docker_normal_inputs").Select("request_ciphertext").Where(
			"tenant_id = ? AND run_id = ? AND activity_key = ?", key.TenantID, key.RunID, key.ActivityKey).Scan(&persisted).Error)
		require.NotContains(t, persisted, string(request.Stdin), "plaintext stdin must not be stored")
		recovered, err := store.Read(ctx, key)
		require.NoError(t, err)
		require.Equal(t, request, recovered.Request)
		for _, mutate := range []func(*CraftDockerNormalInputRequest){
			func(r *CraftDockerNormalInputRequest) { r.Command[0] = "different" },
			func(r *CraftDockerNormalInputRequest) { r.Environment["TOKEN"] = "changed" },
			func(r *CraftDockerNormalInputRequest) { r.Stdin = []byte("changed-input") },
			func(r *CraftDockerNormalInputRequest) { r.TimeoutMillis++ },
			func(r *CraftDockerNormalInputRequest) { r.OutputLimit++ },
			func(r *CraftDockerNormalInputRequest) { r.OutputPolicy = "other-policy" },
		} {
			changed := cloneCraftDockerNormalInput(request)
			mutate(&changed)
			_, err := store.Stage(ctx, changed)
			require.ErrorIs(t, err, ErrCraftDockerNormalInputConflict)
		}
	})
}

func TestCraftDockerNormalInputFailsClosedWithoutKeyAndDetectsCorruption(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "")
	forEachCraftDockerNormalInputDB(t, func(t *testing.T, db *gorm.DB) {
		ctx := context.Background()
		key, _ := craftDockerNormalInputFixtureForDB(t, db, "normal-key", "activity")
		store := NewCraftDockerNormalInputRepository(db)
		request := craftDockerNormalInputFixture(key)
		_, err := store.Stage(ctx, request)
		require.ErrorIs(t, err, ErrCraftDockerNormalInputKeyUnavailable)
		var count int64
		require.NoError(t, db.Table("craft_docker_normal_inputs").Count(&count).Error)
		require.Zero(t, count)

		t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
		_, err = store.Stage(ctx, request)
		require.NoError(t, err)
		// Simulate on-disk bit rot beneath the immutable row guard.
		if db.Name() == "sqlite" {
			require.NoError(t, db.Exec("DROP TRIGGER craft_docker_normal_input_immutable").Error)
		} else {
			require.NoError(t, db.Exec("DROP TRIGGER craft_docker_normal_input_immutable ON craft_docker_normal_inputs").Error)
		}
		require.NoError(t, db.Table("craft_docker_normal_inputs").Where("run_id = ?", key.RunID).Update("request_ciphertext", "enc:v1:corrupt").Error)
		_, err = store.Read(ctx, key)
		require.ErrorIs(t, err, ErrCraftDockerNormalInputCorrupt)
	})
}

func TestCraftDockerNormalInputBindsExactFullReceiptAndClaimsOnce(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	forEachCraftDockerNormalInputDB(t, func(t *testing.T, db *gorm.DB) {
		ctx := context.Background()
		key, revision := craftDockerNormalInputFixtureForDB(t, db, "normal-receipt", "activity")
		inputs := NewCraftDockerNormalInputRepository(db)
		request := craftDockerNormalInputFixture(key)
		_, err := inputs.Stage(ctx, request)
		require.NoError(t, err)
		claims := NewCraftDockerSendClaimRepository(db)
		receipt := CraftDockerNormalReceipt{DockerExecReceipt: DockerExecReceipt{Provider: "docker", ContainerID: "container-normal", ExecID: "exec-normal"}, StdinEnabled: true, StdinByteCount: int64(len(request.Stdin)), StdinSHA256: craftNormalSHA256Hex(request.Stdin), TimeoutMillis: request.TimeoutMillis}
		require.ErrorIs(t, claims.BindDockerNormalExecReceipt(ctx, key, revision, receipt.withStdinHash("wrong")), ErrCraftDockerNormalInputConflict)
		require.ErrorIs(t, claims.BindDockerExecReceipt(ctx, key, revision, receipt.DockerExecReceipt), ErrCraftDockerNormalInputConflict,
			"restricted three-field receipt cannot bind a staged normal operation")
		_, err = claims.ClaimDockerExecSend(ctx, key, revision, receipt.DockerExecReceipt)
		require.ErrorIs(t, err, ErrCraftDockerNormalInputConflict, "restricted claim cannot authorize a staged normal operation")
		require.NoError(t, claims.BindDockerNormalExecReceipt(ctx, key, revision, receipt))
		require.NoError(t, claims.BindDockerNormalExecReceipt(ctx, key, revision, receipt))
		changedReceipt := receipt
		changedReceipt.TimeoutMillis++
		require.ErrorIs(t, claims.BindDockerNormalExecReceipt(ctx, key, revision, changedReceipt), ErrCraftDockerNormalInputConflict)
		require.Error(t, db.Table("craft_docker_normal_inputs").Where("run_id = ?", key.RunID).Update("receipt_timeout_ms", receipt.TimeoutMillis+1).Error,
			"the full receipt cannot be changed by direct SQL after binding")
		claimed, err := claims.ClaimDockerNormalExecSend(ctx, key, revision, receipt)
		require.NoError(t, err)
		require.True(t, claimed)
		claimed, err = claims.ClaimDockerNormalExecSend(ctx, key, revision, receipt)
		require.NoError(t, err)
		require.False(t, claimed)
		altered := receipt
		altered.TimeoutMillis++
		_, err = claims.ClaimDockerNormalExecSend(ctx, key, revision, altered)
		require.ErrorIs(t, err, ErrCraftDockerNormalInputConflict)
	})
}

func TestCraftDockerNormalInputRejectsForeignTaskTenantAndRun(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	forEachCraftDockerNormalInputDB(t, func(t *testing.T, db *gorm.DB) {
		ctx := context.Background()
		key, _ := craftDockerNormalInputFixtureForDB(t, db, "normal-scope", "activity")
		store := NewCraftDockerNormalInputRepository(db)
		request := craftDockerNormalInputFixture(key)
		request.TaskID = "other-task"
		_, err := store.Stage(ctx, request)
		require.ErrorIs(t, err, ErrCraftDockerNormalInputConflict)
		request = craftDockerNormalInputFixture(key)
		request.TenantID++
		_, err = store.Stage(ctx, request)
		require.ErrorIs(t, err, ErrCraftDockerNormalInputConflict)
		request = craftDockerNormalInputFixture(key)
		request.RunID += "-other"
		_, err = store.Stage(ctx, request)
		require.ErrorIs(t, err, ErrCraftDockerNormalInputConflict)
	})
}

func TestCraftDockerLegacyBindAndClaimRecheckStageAfterRunLock(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	forEachCraftDockerNormalInputDB(t, func(t *testing.T, db *gorm.DB) {
		key, revision := craftDockerNormalInputFixtureForDB(t, db, "normal-lock-bind", "activity")
		receipt := DockerExecReceipt{Provider: "docker", ContainerID: "container-lock-bind", ExecID: "exec-lock-bind"}
		exerciseCraftDockerStageLockInterleave(t, db, craftDockerNormalInputFixture(key), nil, func(ctx context.Context, claimDB *gorm.DB) error {
			return NewCraftDockerSendClaimRepository(claimDB).BindDockerExecReceipt(ctx, key, revision, receipt)
		})
		var bound struct{ Provider *string }
		require.NoError(t, db.Table("craft_charge_start_journal").Select("provider").Where("run_id = ?", key.RunID).Take(&bound).Error)
		require.Nil(t, bound.Provider, "legacy Bind must recheck staged normal input after acquiring Run lock")
	})
	forEachCraftDockerNormalInputDB(t, func(t *testing.T, db *gorm.DB) {
		key, revision := craftDockerNormalInputFixtureForDB(t, db, "normal-lock-claim", "activity")
		receipt := DockerExecReceipt{Provider: "docker", ContainerID: "container-lock-claim", ExecID: "exec-lock-claim"}
		// Claim requires a complete receipt. Bind it in the same held
		// transaction, immediately after the staged row is created.
		afterStage := func(tx *gorm.DB) error {
			return tx.Table("craft_charge_start_journal").Where("run_id = ?", key.RunID).Updates(map[string]any{
				"provider": receipt.Provider, "container_id": receipt.ContainerID, "exec_id": receipt.ExecID,
			}).Error
		}
		exerciseCraftDockerStageLockInterleave(t, db, craftDockerNormalInputFixture(key), afterStage, func(ctx context.Context, claimDB *gorm.DB) error {
			claimed, err := NewCraftDockerSendClaimRepository(claimDB).ClaimDockerExecSend(ctx, key, revision, receipt)
			if claimed && err == nil {
				return errors.New("legacy claim unexpectedly received send authority")
			}
			return err
		})
		var claimed struct{ SendClaimedAt *time.Time }
		require.NoError(t, db.Table("craft_charge_start_journal").Select("send_claimed_at").Where("run_id = ?", key.RunID).Take(&claimed).Error)
		require.Nil(t, claimed.SendClaimedAt, "legacy Claim must recheck staged normal input after acquiring Run lock")
	})
}

func TestCraftDockerNormalInputCanonicalSizeBoundAndExactBoundary(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	forEachCraftDockerNormalInputDB(t, func(t *testing.T, db *gorm.DB) {
		key, _ := craftDockerNormalInputFixtureForDB(t, db, "normal-size", "activity")
		store := NewCraftDockerNormalInputRepository(db)
		base := craftDockerNormalInputFixture(key)
		base.WorkingDir = ""
		encoded, err := json.Marshal(base)
		require.NoError(t, err)
		require.Less(t, len(encoded), MaxCraftDockerNormalRequestBytes)
		base.WorkingDir = strings.Repeat("x", MaxCraftDockerNormalRequestBytes-len(encoded))
		encoded, err = json.Marshal(base)
		require.NoError(t, err)
		require.Len(t, encoded, MaxCraftDockerNormalRequestBytes, "test exercises the exact canonical byte limit")
		staged, err := store.Stage(context.Background(), base)
		require.NoError(t, err)
		require.Equal(t, base, staged.Request)
		var ciphertextLen int
		require.NoError(t, db.Table("craft_docker_normal_inputs").Select("length(request_ciphertext)").Where("run_id = ?", key.RunID).Scan(&ciphertextLen).Error)
		require.LessOrEqual(t, ciphertextLen, MaxCraftDockerNormalCiphertextBytes)
	})
}

func TestCraftDockerNormalInputCanonicalSizerMatchesJSONEscaping(t *testing.T) {
	request := craftDockerNormalInputFixture(CraftChargeStartKey{TenantID: 1, RunID: "run", ActivityKey: "activity"})
	request.Command = []string{"<tag>&", "line\nfeed", "quote\"slash\\"}
	request.Environment = map[string]string{"<key>": "line\u2028separator", "control": "\x01"}
	request.User = "invalid-\xff"
	encoded, err := json.Marshal(request)
	require.NoError(t, err)
	require.EqualValues(t, len(encoded), craftDockerNormalCanonicalJSONSize(request))

	// Round-5 additive evidence fields: carrying them must NOT desynchronize
	// the canonical sizer — the request still stages through the durable
	// encrypted identity chain (previously the sizer ignored both fields and
	// Stage rejected every evidence-carrying request as Corrupt).
	request.ResolvedTargetPath = "/host/toolchain/inputs<a>\u2028 x"
	request.TargetSHA256 = strings.Repeat("0123456789abcdef", 4)
	encoded, err = json.Marshal(request)
	require.NoError(t, err)
	require.EqualValues(t, len(encoded), craftDockerNormalCanonicalJSONSize(request),
		"evidence-carrying request must size-match json.Marshal")
}

func TestCraftDockerNormalInputRejectsOversizedCommandEnvironmentAndStoredPayload(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "")
	forEachCraftDockerNormalInputDB(t, func(t *testing.T, db *gorm.DB) {
		key, _ := craftDockerNormalInputFixtureForDB(t, db, "normal-oversize", "activity")
		store := NewCraftDockerNormalInputRepository(db)
		for _, mutate := range []func(*CraftDockerNormalInputRequest){
			func(request *CraftDockerNormalInputRequest) {
				request.Command[0] = strings.Repeat("x", MaxCraftDockerNormalRequestBytes+1)
			},
			func(request *CraftDockerNormalInputRequest) {
				request.Environment["OVERSIZED"] = strings.Repeat("<", MaxCraftDockerNormalRequestBytes/6)
			},
		} {
			request := cloneCraftDockerNormalInput(craftDockerNormalInputFixture(key))
			mutate(&request)
			_, err := store.Stage(context.Background(), request)
			require.ErrorIs(t, err, ErrCraftDockerNormalInputTooLarge)
		}
		// An oversized request is rejected before encryption/key lookup.
		// The empty key therefore does not replace the size error.
		t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
		var count int64
		require.NoError(t, db.Table("craft_docker_normal_inputs").Where("run_id = ?", key.RunID).Count(&count).Error)
		require.Zero(t, count, "oversized canonical requests must not persist")

		malicious := craftDockerNormalInputTestStageRow(key, "s1")
		malicious.RequestCiphertext = strings.Repeat("x", MaxCraftDockerNormalCiphertextBytes+1)
		require.Error(t, db.Create(&malicious).Error, "database ciphertext bound rejects oversized persisted payload")
		malicious.RequestCiphertext = strings.Repeat("x", MaxCraftDockerNormalCiphertextBytes+1)
		_, err := decryptCraftDockerNormalInput(malicious)
		require.ErrorIs(t, err, ErrCraftDockerNormalInputCorrupt, "recovery rejects oversized malicious ciphertext before decryption")
	})
}

func exerciseCraftDockerStageLockInterleave(t *testing.T, db *gorm.DB, request CraftDockerNormalInputRequest, afterStage func(*gorm.DB) error, invoke func(context.Context, *gorm.DB) error) {
	t.Helper()
	stageDB := reopenRunDB(t, db)
	claimDB := reopenRunDB(t, db)
	stageInserted := make(chan struct{})
	commitStage := make(chan struct{})
	stageErr := make(chan error, 1)
	stageCallbackName := "test_craft_docker_stage_barrier_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, stageDB.Callback().Create().After("gorm:create").Register(stageCallbackName, func(tx *gorm.DB) {
		if tx.Statement.Table != "craft_docker_normal_inputs" {
			return
		}
		if afterStage != nil {
			tx.AddError(afterStage(tx))
			if tx.Error != nil {
				return
			}
		}
		close(stageInserted)
		<-commitStage
	}))
	t.Cleanup(func() { _ = stageDB.Callback().Create().Remove(stageCallbackName) })
	go func() {
		_, err := NewCraftDockerNormalInputRepository(stageDB).Stage(context.Background(), request)
		stageErr <- err
	}()
	select {
	case <-stageInserted:
	case <-time.After(3 * time.Second):
		close(commitStage)
		t.Fatal("Stage did not reach its insert barrier")
	}
	lockReached := make(chan struct{})
	var once sync.Once
	callbackName := "test_craft_docker_normal_run_lock_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, claimDB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "agent_runs" {
			once.Do(func() { close(lockReached) })
		}
	}))
	t.Cleanup(func() { _ = claimDB.Callback().Update().Remove(callbackName) })
	done := make(chan error, 1)
	go func() { done <- invoke(context.Background(), claimDB) }()
	select {
	case <-lockReached:
	case <-time.After(3 * time.Second):
		close(commitStage)
		t.Fatal("legacy path did not reach the Run lock")
	}
	// The stage is still invisible to a pre-lock lookup, but committed before
	// the blocked operation can mutate or claim its journal row.
	close(commitStage)
	require.NoError(t, <-stageErr)
	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrCraftDockerNormalInputConflict)
	case <-time.After(5 * time.Second):
		t.Fatal("legacy path did not finish after Run lock release")
	}
}

func TestCraftDockerNormalInputZeroLengthIdentityAndConcurrentExactStage(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	forEachCraftDockerNormalInputDB(t, func(t *testing.T, db *gorm.DB) {
		ctx := context.Background()
		key, _ := craftDockerNormalInputFixtureForDB(t, db, "normal-empty", "activity")
		request := craftDockerNormalInputFixture(key)
		request.Stdin = nil
		request.StdinEnabled = false
		storeA := NewCraftDockerNormalInputRepository(db)
		storeB := NewCraftDockerNormalInputRepository(reopenRunDB(t, db))
		start := make(chan struct{})
		var wg sync.WaitGroup
		errs := make([]error, 2)
		stores := []*CraftDockerNormalInputRepository{storeA, storeB}
		for i := range stores {
			wg.Add(1)
			go func(i int) { defer wg.Done(); <-start; _, errs[i] = stores[i].Stage(ctx, request) }(i)
		}
		close(start)
		wg.Wait()
		require.NoError(t, errs[0])
		require.NoError(t, errs[1])
		staged, err := storeA.Read(ctx, key)
		require.NoError(t, err)
		require.False(t, staged.StdinEnabled)
		require.Zero(t, staged.StdinByteCount)
		require.Equal(t, craftNormalSHA256Hex(nil), staged.StdinSHA256)
		request.StdinEnabled = true
		_, err = storeA.Stage(ctx, request)
		require.ErrorIs(t, err, ErrCraftDockerNormalInputConflict)
	})
}

func TestCraftDockerNormalInputMigrationUpDownUp(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	forEachCraftDockerNormalInputDB(t, func(t *testing.T, db *gorm.DB) {
		engine := db.Name()
		if engine == "sqlite" {
			for _, direction := range []string{"down", "up"} {
				body, err := os.ReadFile(filepath.Join(root, "migrations/sqlite/000176_craft_docker_normal_input."+direction+".sql"))
				require.NoError(t, err)
				require.NoError(t, db.Exec(string(body)).Error)
			}
			require.True(t, db.Migrator().HasTable("craft_docker_normal_inputs"))
			return
		}
		for _, direction := range []string{"down", "up"} {
			body, err := os.ReadFile(filepath.Join(root, "migrations/versioned/000204_craft_docker_normal_input."+direction+".sql"))
			require.NoError(t, err)
			require.NoError(t, db.Exec(string(body)).Error)
		}
		require.True(t, db.Migrator().HasTable("craft_docker_normal_inputs"))
	})
}

func craftDockerNormalInputFixture(key CraftChargeStartKey) CraftDockerNormalInputRequest {
	return CraftDockerNormalInputRequest{TenantID: key.TenantID, TaskID: "s1", RunID: key.RunID, ActivityKey: key.ActivityKey,
		Command: []string{"sh", "-c", "cat"}, Environment: map[string]string{"TOKEN": "secret-marker"}, User: "runner", WorkingDir: "/workspace",
		TimeoutMillis: 30000, StdinEnabled: true, Stdin: []byte("secret-input-marker"), OutputLimit: 4096, OutputPolicy: "durable-v1"}
}

func craftDockerNormalInputTestStageRow(key CraftChargeStartKey, taskID string) craftDockerNormalInputRow {
	return craftDockerNormalInputRow{TenantID: key.TenantID, TaskID: taskID, RunID: key.RunID, ActivityKey: key.ActivityKey,
		RequestCiphertext: "enc:v1:synthetic", RequestSHA256: strings.Repeat("a", 64), StdinEnabled: false,
		StdinByteCount: 0, StdinSHA256: craftNormalSHA256Hex(nil), TimeoutMillis: 30000, OutputLimit: 4096, OutputPolicy: "durable-v1"}
}

func forEachCraftDockerNormalInputDB(t *testing.T, run func(*testing.T, *gorm.DB)) {
	t.Helper()
	for _, engine := range []string{"sqlite", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			if engine == "sqlite" {
				run(t, openRunTestDB(t))
				return
			}
			run(t, openCraftDockerNormalInputPostgresDB(t))
		})
	}
}

func openCraftDockerNormalInputPostgresDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TRPC_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TRPC_TEST_POSTGRES_DSN unset: PostgreSQL normal-input acceptance NOT VERIFIED")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	schema := "t19_input_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		require.NoError(t, admin.Exec("DROP SCHEMA "+schema+" CASCADE").Error)
		sqlDB, e := admin.DB()
		if e == nil {
			_ = sqlDB.Close()
		}
	})
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema+",public")
	u.RawQuery = q.Encode()
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (
		tenant_id BIGINT NOT NULL, run_id TEXT NOT NULL, session_id TEXT NOT NULL, revision BIGINT NOT NULL, status TEXT NOT NULL,
		PRIMARY KEY (tenant_id, run_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE craft_charge_start_journal (
		tenant_id BIGINT NOT NULL, run_id TEXT NOT NULL, activity_key TEXT NOT NULL, state TEXT NOT NULL, protocol TEXT,
		provider TEXT, container_id TEXT, exec_id TEXT, send_claimed_at TIMESTAMPTZ, run_revision BIGINT NOT NULL,
		PRIMARY KEY (tenant_id, run_id, activity_key), FOREIGN KEY (tenant_id, run_id) REFERENCES agent_runs(tenant_id,run_id))`).Error)
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	migration, err := os.ReadFile(filepath.Join(root, "migrations/versioned/000204_craft_docker_normal_input.up.sql"))
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(migration)).Error)
	return db
}

func craftDockerNormalInputFixtureForDB(t *testing.T, db *gorm.DB, runID, activity string) (CraftChargeStartKey, int64) {
	t.Helper()
	if db.Name() == "sqlite" {
		return craftDockerSendFixture(t, db, runID, activity)
	}
	key := CraftChargeStartKey{TenantID: 1, RunID: runID, ActivityKey: activity}
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id,run_id,session_id,revision,status) VALUES (1,?,?,1,'running')`, runID, "s1").Error)
	require.NoError(t, db.Exec(`INSERT INTO craft_charge_start_journal (tenant_id,run_id,activity_key,state,protocol,run_revision) VALUES (1,?,?,'intent','docker_coordinator',1)`, runID, activity).Error)
	return key, 1
}

func cloneCraftDockerNormalInput(in CraftDockerNormalInputRequest) CraftDockerNormalInputRequest {
	out := in
	out.Command = append([]string(nil), in.Command...)
	out.Stdin = append([]byte(nil), in.Stdin...)
	out.Environment = map[string]string{}
	for k, v := range in.Environment {
		out.Environment[k] = v
	}
	return out
}

func craftNormalSHA256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

// TestCraftDockerNormalInputRejectsMalformedEvidenceFields is the round-2
// medium finding regression: a malformed TargetSHA256 silently misses the
// manifest digest map (indistinguishable from no evidence) while
// suppressing the stdin channel's identity derivation — it must be rejected
// at shape validation, and both evidence fields reject NUL.
func TestCraftDockerNormalInputRejectsMalformedEvidenceFields(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	forEachCraftDockerNormalInputDB(t, func(t *testing.T, db *gorm.DB) {
		key, _ := craftDockerNormalInputFixtureForDB(t, db, "normal-evidence", "activity-evidence")
		store := NewCraftDockerNormalInputRepository(db)
		short := craftDockerNormalInputFixture(key)
		short.TargetSHA256 = "ABC"
		_, err := store.Stage(context.Background(), short)
		require.ErrorIs(t, err, ErrCraftDockerNormalInputInvalid, "a truncated digest is malformed evidence")
		upper := craftDockerNormalInputFixture(key)
		upper.TargetSHA256 = strings.ToUpper(strings.Repeat("0123456789abcdef", 4))
		_, err = store.Stage(context.Background(), upper)
		require.ErrorIs(t, err, ErrCraftDockerNormalInputInvalid, "an uppercase digest is malformed evidence")
		nulPath := craftDockerNormalInputFixture(key)
		nulPath.ResolvedTargetPath = "/host/\x00toolchain"
		_, err = store.Stage(context.Background(), nulPath)
		require.ErrorIs(t, err, ErrCraftDockerNormalInputInvalid, "a NUL in the resolved path is malformed evidence")

		valid := craftDockerNormalInputFixture(key)
		valid.TargetSHA256 = strings.Repeat("0123456789abcdef", 4)
		staged, err := store.Stage(context.Background(), valid)
		require.NoError(t, err, "a well-formed digest must stage through the durable identity chain")
		require.NotEmpty(t, staged.RequestSHA256)
	})
}
