package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func craftDockerSendFixture(t *testing.T, db *gorm.DB, runID, activity string) (CraftChargeStartKey, int64) {
	t.Helper()
	seedCraftRun(t, db, runID, "call-"+activity)
	key := CraftChargeStartKey{TenantID: 1, RunID: runID, ActivityKey: activity}
	var run agentRunRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Take(&run).Error)
	now := time.Now().UTC()
	require.NoError(t, db.Exec(`INSERT INTO craft_charge_start_journal
		(tenant_id, run_id, activity_key, grant_id, call_id, reservation_key, state, run_revision, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 'intent', ?, ?, ?)`, key.TenantID, key.RunID, key.ActivityKey,
		"grant-"+activity, "call-"+activity, "reservation-"+activity, run.Revision, now, now).Error)
	require.NoError(t, db.Exec(`INSERT INTO commercial_reservations
		(tenant_id, key, run_id, upper_micro, state, deadline, fence)
		VALUES (?, ?, ?, 100, 'dispatched', ?, 'fixture')`, key.TenantID, "reservation-"+activity,
		key.RunID, time.Now().Add(time.Hour)).Error)
	return key, run.Revision
}

func craftDockerSendExtraOperation(t *testing.T, db *gorm.DB, key CraftChargeStartKey, revision int64) {
	t.Helper()
	now := time.Now().UTC()
	require.NoError(t, db.Exec(`INSERT INTO craft_charge_start_journal
		(tenant_id, run_id, activity_key, grant_id, call_id, reservation_key, state, run_revision, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 'intent', ?, ?, ?)`, key.TenantID, key.RunID, key.ActivityKey,
		"grant-"+key.ActivityKey, "call-"+key.ActivityKey, "reservation-"+key.ActivityKey, revision, now, now).Error)
}

func forEachCraftDockerSendDB(t *testing.T, run func(*testing.T, *gorm.DB)) {
	t.Helper()
	for _, engine := range []string{"sqlite", "postgres"} {
		t.Run(engine, func(t *testing.T) { run(t, openRunTestDB(t)) })
	}
}

func craftDockerSendAbortUpdate(t *testing.T, db *gorm.DB, triggerName, functionName, columns string) func() {
	t.Helper()
	if db.Name() == "sqlite" {
		require.NoError(t, db.Exec(fmt.Sprintf(`CREATE TRIGGER %s BEFORE UPDATE OF %s ON craft_charge_start_journal
			BEGIN SELECT RAISE(ABORT, 'injected %s failure'); END`, triggerName, columns, triggerName)).Error)
	} else {
		require.NoError(t, db.Exec(fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN RAISE EXCEPTION 'injected %s failure'; END; $$`, functionName, triggerName)).Error)
		require.NoError(t, db.Exec(fmt.Sprintf(`CREATE TRIGGER %s BEFORE UPDATE OF %s ON craft_charge_start_journal
			FOR EACH ROW EXECUTE FUNCTION %s()`, triggerName, columns, functionName)).Error)
	}
	drop := func() {
		if db.Name() == "sqlite" {
			_ = db.Exec(fmt.Sprintf("DROP TRIGGER IF EXISTS %s", triggerName)).Error
		} else {
			_ = db.Exec(fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON craft_charge_start_journal", triggerName)).Error
			_ = db.Exec(fmt.Sprintf("DROP FUNCTION IF EXISTS %s()", functionName)).Error
		}
	}
	t.Cleanup(drop)
	return drop
}

func assertCraftDockerSendPreparedAndHeld(t *testing.T, db *gorm.DB, key CraftChargeStartKey, revision int64, receiptBound bool) {
	t.Helper()
	var row struct {
		State         string
		RunRevision   int64
		Provider      *string
		ContainerID   *string
		ExecID        *string
		SendClaimedAt *time.Time
	}
	require.NoError(t, db.Table("craft_charge_start_journal").Where(
		"tenant_id = ? AND run_id = ? AND activity_key = ?", key.TenantID, key.RunID, key.ActivityKey).Take(&row).Error)
	require.Equal(t, "intent", row.State)
	require.Equal(t, revision, row.RunRevision)
	require.Nil(t, row.SendClaimedAt)
	if receiptBound {
		require.NotNil(t, row.Provider)
		require.NotNil(t, row.ContainerID)
		require.NotNil(t, row.ExecID)
	} else {
		require.Nil(t, row.Provider)
		require.Nil(t, row.ContainerID)
		require.Nil(t, row.ExecID)
	}
	var held int64
	require.NoError(t, db.Table("commercial_reservations").Where(
		"tenant_id = ? AND key = ? AND run_id = ? AND state = 'dispatched'", key.TenantID,
		"reservation-"+key.ActivityKey, key.RunID).Count(&held).Error)
	require.EqualValues(t, 1, held)
}

func TestCraftDockerSendClaimSchemaConstraints(t *testing.T) {
	forEachCraftDockerSendDB(t, testCraftDockerSendClaimSchemaConstraints)
}

func testCraftDockerSendClaimSchemaConstraints(t *testing.T, db *gorm.DB) {
	key, revision := craftDockerSendFixture(t, db, "docker-schema", "schema-a")
	columns := []string{"provider", "container_id", "exec_id", "send_claimed_at"}
	for _, column := range columns {
		require.True(t, db.Migrator().HasColumn("craft_charge_start_journal", column), column)
	}
	require.Error(t, db.Exec(`UPDATE craft_charge_start_journal SET provider = 'docker'
		WHERE tenant_id = ? AND run_id = ? AND activity_key = ?`, key.TenantID, key.RunID, key.ActivityKey).Error,
		"partial receipt must be rejected")
	require.Error(t, db.Exec(`UPDATE craft_charge_start_journal SET send_claimed_at = CURRENT_TIMESTAMP
		WHERE tenant_id = ? AND run_id = ? AND activity_key = ?`, key.TenantID, key.RunID, key.ActivityKey).Error,
		"a send claim without a complete receipt must be rejected")
	require.NoError(t, db.Exec(`UPDATE craft_charge_start_journal SET provider = 'docker', container_id = 'container-a', exec_id = 'exec-a'
		WHERE tenant_id = ? AND run_id = ? AND activity_key = ?`, key.TenantID, key.RunID, key.ActivityKey).Error)
	require.Error(t, db.Exec(`INSERT INTO craft_charge_start_journal
		(tenant_id, run_id, activity_key, grant_id, call_id, reservation_key, state, run_revision, created_at, updated_at,
		 provider, container_id, exec_id)
		VALUES (?, ?, ?, ?, ?, ?, 'intent', ?, ?, ?, 'docker', 'container-a', 'exec-a')`,
		key.TenantID, key.RunID, "schema-b", "grant-b", "call-b", "reservation-b", revision, time.Now(), time.Now()).Error,
		"receipt identity must be unique")
}

func TestCraftDockerSendClaimBindsImmutableReceiptAndClaimsOnce(t *testing.T) {
	forEachCraftDockerSendDB(t, testCraftDockerSendClaimBindsImmutableReceiptAndClaimsOnce)
}

func testCraftDockerSendClaimBindsImmutableReceiptAndClaimsOnce(t *testing.T, db *gorm.DB) {
	ctx := context.Background()
	key, revision := craftDockerSendFixture(t, db, "docker-bind", "activity")
	store := NewCraftDockerSendClaimRepository(db)
	receipt := DockerExecReceipt{Provider: "docker", ContainerID: "container-1", ExecID: "exec-1"}
	require.NoError(t, store.BindDockerExecReceipt(ctx, key, revision, receipt))
	require.NoError(t, store.BindDockerExecReceipt(ctx, key, revision, receipt), "exact receipt replay is idempotent")
	require.ErrorIs(t, store.BindDockerExecReceipt(ctx, key, revision, DockerExecReceipt{Provider: "docker", ContainerID: "container-1", ExecID: "exec-other"}), craft.ErrConflict)
	require.ErrorIs(t, store.BindDockerExecReceipt(ctx, key, revision+1, receipt), craft.ErrConflict)
	otherKey := CraftChargeStartKey{TenantID: key.TenantID, RunID: key.RunID, ActivityKey: "activity-other"}
	craftDockerSendExtraOperation(t, db, otherKey, revision)
	require.Error(t, store.BindDockerExecReceipt(ctx, otherKey, revision, receipt), "one Docker exec receipt cannot identify two operations")
	claimed, err := store.ClaimDockerExecSend(ctx, key, revision, receipt)
	require.NoError(t, err)
	require.True(t, claimed)

	// Reconstructed repository and false/zero/404-like observations have no
	// transition API: replay only observes the durable claim and cannot resend.
	replayed, err := NewCraftDockerSendClaimRepository(reopenRunDB(t, db)).ClaimDockerExecSend(ctx, key, revision, receipt)
	require.NoError(t, err)
	require.False(t, replayed)
	var state string
	require.NoError(t, db.Table("craft_charge_start_journal").Select("state").Where(
		"tenant_id = ? AND run_id = ? AND activity_key = ?", key.TenantID, key.RunID, key.ActivityKey).Scan(&state).Error)
	require.Equal(t, "intent", state, "claim remains unresolved")
	var held int64
	require.NoError(t, db.Table("commercial_reservations").Where("tenant_id = ? AND key = ? AND state = 'dispatched'",
		key.TenantID, "reservation-activity").Count(&held).Error)
	require.EqualValues(t, 1, held, "claim/replay cannot release the existing hold")
}

func TestCraftDockerSendClaimConcurrentOwnerRace(t *testing.T) {
	forEachCraftDockerSendDB(t, testCraftDockerSendClaimConcurrentOwnerRace)
}

func testCraftDockerSendClaimConcurrentOwnerRace(t *testing.T, db *gorm.DB) {
	ctx := context.Background()
	other := reopenRunDB(t, db)
	key, revision := craftDockerSendFixture(t, db, "docker-race", "activity")
	receipt := DockerExecReceipt{Provider: "docker", ContainerID: "container-race", ExecID: "exec-race"}
	require.NoError(t, NewCraftDockerSendClaimRepository(db).BindDockerExecReceipt(ctx, key, revision, receipt))
	stores := []*CraftDockerSendClaimRepository{NewCraftDockerSendClaimRepository(db), NewCraftDockerSendClaimRepository(other)}
	start := make(chan struct{})
	results := make([]bool, len(stores))
	errs := make([]error, len(stores))
	var wg sync.WaitGroup
	for i, store := range stores {
		wg.Add(1)
		go func(i int, store *CraftDockerSendClaimRepository) {
			defer wg.Done()
			<-start
			results[i], errs[i] = store.ClaimDockerExecSend(ctx, key, revision, receipt)
		}(i, store)
	}
	close(start)
	wg.Wait()
	wins := 0
	for i := range results {
		require.NoError(t, errs[i])
		if results[i] {
			wins++
		}
	}
	require.Equal(t, 1, wins, "exactly one process may receive send authority")
}

func TestCraftDockerSendClaimFailureAndStaleEpochNeverAuthorizeSend(t *testing.T) {
	forEachCraftDockerSendDB(t, testCraftDockerSendClaimFailureAndStaleEpochNeverAuthorizeSend)
}

func testCraftDockerSendClaimFailureAndStaleEpochNeverAuthorizeSend(t *testing.T, db *gorm.DB) {
	ctx := context.Background()
	key, revision := craftDockerSendFixture(t, db, "docker-failure", "activity")
	store := NewCraftDockerSendClaimRepository(db)
	receipt := DockerExecReceipt{Provider: "docker", ContainerID: "container-failure", ExecID: "exec-failure"}
	require.ErrorIs(t, store.BindDockerExecReceipt(ctx, key, revision+1, receipt), craft.ErrConflict)
	dropBindFailure := craftDockerSendAbortUpdate(t, db, "fail_docker_receipt_bind", "reject_docker_receipt_bind", "provider, container_id, exec_id")
	require.Error(t, store.BindDockerExecReceipt(ctx, key, revision, receipt))
	assertCraftDockerSendPreparedAndHeld(t, db, key, revision, false)
	claimed, err := store.ClaimDockerExecSend(ctx, key, revision, receipt)
	require.ErrorIs(t, err, craft.ErrConflict)
	require.False(t, claimed)
	dropBindFailure()
	require.NoError(t, store.BindDockerExecReceipt(ctx, key, revision, receipt))
	claimed, err = store.ClaimDockerExecSend(ctx, CraftChargeStartKey{TenantID: key.TenantID + 1, RunID: key.RunID, ActivityKey: key.ActivityKey}, revision, receipt)
	require.ErrorIs(t, err, craft.ErrConflict)
	require.False(t, claimed)
	claimed, err = store.ClaimDockerExecSend(ctx, CraftChargeStartKey{TenantID: key.TenantID, RunID: key.RunID + "-wrong", ActivityKey: key.ActivityKey}, revision, receipt)
	require.ErrorIs(t, err, craft.ErrConflict)
	require.False(t, claimed)
	dropClaimFailure := craftDockerSendAbortUpdate(t, db, "fail_docker_claim", "reject_docker_claim", "send_claimed_at")
	claimed, err = store.ClaimDockerExecSend(ctx, key, revision, receipt)
	require.Error(t, err)
	require.False(t, claimed)
	assertCraftDockerSendPreparedAndHeld(t, db, key, revision, true)
	dropClaimFailure()
	var sendClaims int64
	require.NoError(t, db.Table("craft_charge_start_journal").Where(
		"tenant_id = ? AND run_id = ? AND activity_key = ? AND send_claimed_at IS NOT NULL", key.TenantID, key.RunID, key.ActivityKey).Count(&sendClaims).Error)
	require.Zero(t, sendClaims)
	claimed, err = NewCraftDockerSendClaimRepository(db).ClaimDockerExecSend(ctx, key, revision+1, receipt)
	require.ErrorIs(t, err, craft.ErrConflict)
	require.False(t, claimed)
}

func TestCraftDockerSendClaimMigrationUpDownUp(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	db := openRunTestDB(t)
	// The regular test DB is already at tip. Exercise just the owned migration
	// against its real table shape, retaining unrelated journal rows.
	key, _ := craftDockerSendFixture(t, db, "docker-migration-row", "activity")
	for _, direction := range []string{"down", "up"} {
		contents, err := os.ReadFile(filepath.Join(root, "migrations/sqlite/000117_craft_docker_exec_send_claim."+direction+".sql"))
		require.NoError(t, err)
		require.NoError(t, db.Exec(string(contents)).Error)
	}
	var count int64
	require.NoError(t, db.Table("craft_charge_start_journal").Where("tenant_id = ? AND run_id = ? AND activity_key = ?", key.TenantID, key.RunID, key.ActivityKey).Count(&count).Error)
	require.EqualValues(t, 1, count, "down/up migration retains the existing journal row")

	t.Run("postgres", func(t *testing.T) {
		pgdb := openPostgresRunTestDB(t, root)
		pgKey, _ := craftDockerSendFixture(t, pgdb, "docker-pg-migration-row", "activity")
		for _, direction := range []string{"down", "up"} {
			contents, err := os.ReadFile(filepath.Join(root, "migrations/versioned/000196_craft_docker_exec_send_claim."+direction+".sql"))
			require.NoError(t, err)
			require.NoError(t, pgdb.Exec(string(contents)).Error)
		}
		for _, column := range []string{"provider", "container_id", "exec_id", "send_claimed_at"} {
			require.True(t, pgdb.Migrator().HasColumn("craft_charge_start_journal", column), column)
		}
		var pgCount int64
		require.NoError(t, pgdb.Table("craft_charge_start_journal").Where("tenant_id = ? AND run_id = ? AND activity_key = ?", pgKey.TenantID, pgKey.RunID, pgKey.ActivityKey).Count(&pgCount).Error)
		require.EqualValues(t, 1, pgCount, "down/up migration retains the existing journal row")
	})
}

func TestCraftDockerSendClaimRequiresCurrentExecutableRunFence(t *testing.T) {
	forEachCraftDockerSendDB(t, testCraftDockerSendClaimRequiresCurrentExecutableRunFence)
}

func testCraftDockerSendClaimRequiresCurrentExecutableRunFence(t *testing.T, db *gorm.DB) {
	ctx := context.Background()
	store := NewCraftDockerSendClaimRepository(db)

	key, revision := craftDockerSendFixture(t, db, "docker-current-run-fence", "activity")
	receipt := DockerExecReceipt{Provider: "docker", ContainerID: "container-current", ExecID: "exec-current"}
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Update("revision", revision+1).Error)
	require.ErrorIs(t, store.BindDockerExecReceipt(ctx, key, revision, receipt), craft.ErrConflict,
		"receipt bind cannot advance an obsolete current Run epoch")
	assertCraftDockerSendPreparedAndHeld(t, db, key, revision, false)

	// Advance the prepared epoch in this fixture to isolate the status fence.
	require.NoError(t, db.Table("craft_charge_start_journal").Where("tenant_id = ? AND run_id = ? AND activity_key = ?", key.TenantID, key.RunID, key.ActivityKey).Update("run_revision", revision+1).Error)
	require.NoError(t, store.BindDockerExecReceipt(ctx, key, revision+1, receipt))
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Update("status", "waiting_user").Error)
	claimed, err := store.ClaimDockerExecSend(ctx, key, revision+1, receipt)
	require.ErrorIs(t, err, craft.ErrConflict, "a paused Run cannot receive send authority")
	require.False(t, claimed)
	assertCraftDockerSendPreparedAndHeld(t, db, key, revision+1, true)
}
