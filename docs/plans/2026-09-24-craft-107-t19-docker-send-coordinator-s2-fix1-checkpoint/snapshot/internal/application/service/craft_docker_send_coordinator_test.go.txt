package service

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	repository "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newCraftDockerCoordinatorFixture(t *testing.T) (*CraftDockerSendCoordinator, *CraftBudgetService, repository.DockerExecReceipt, uint64, string) {
	t.Helper()
	db := openCraftBudgetTestDB(t)
	const tenant uint64 = 701
	const runID = "run-docker-coordinator"
	const sessionID = "task-docker-coordinator"
	seedCraftFundedTenant(t, db, tenant, 10000)
	seedCraftBudgetRun(t, db, tenant, runID, sessionID)
	budget, err := NewCraftBudgetService(db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	grant, err := budget.Admit(context.Background(), craft.Scope{TenantID: tenant, UserID: "owner", SessionID: sessionID}, runID)
	require.NoError(t, err)
	claims := repository.NewCraftDockerSendClaimRepository(db)
	coordinator, err := NewCraftDockerSendCoordinator(budget, claims)
	require.NoError(t, err)
	return coordinator, budget, repository.DockerExecReceipt{Provider: "docker", ContainerID: "container-coordinator", ExecID: "exec-coordinator"}, tenant, grant.ID
}

func TestCraftDockerSendCoordinatorPreparesHeldIntentThenBindsExactReceipt(t *testing.T) {
	coordinator, budget, receipt, _, grantID := newCraftDockerCoordinatorFixture(t)
	ctx := context.Background()
	binding := CraftCallBinding{DelegationID: "delegate", ModelID: "model", Funding: commercial.FundingPlatform}
	op, err := coordinator.Prepare(ctx, grantID, "activity-1", binding)
	require.NoError(t, err)
	journal := op.journal
	assertCoordinatorIntentHeld(t, budget, journal, false, false)

	// A crash before binding can repeat only inert create under the same intent.
	replayed, err := coordinator.Prepare(ctx, grantID, "activity-1", binding)
	require.NoError(t, err)
	require.Equal(t, journal, replayed.journal)
	require.NoError(t, op.Bind(ctx, receipt))
	require.NoError(t, replayed.Bind(ctx, receipt), "exact receipt binding is idempotent")
	resumed, persistedReceipt, err := coordinator.ResumeBound(ctx, grantID, "activity-1")
	require.NoError(t, err)
	require.Equal(t, receipt, persistedReceipt)
	resumedClaim, err := resumed.Claim(ctx, persistedReceipt)
	require.NoError(t, err)
	require.NotNil(t, resumedClaim.Permission, "crash after bind can claim only the persisted exact receipt")
	_, err = replayed.Claim(ctx, repository.DockerExecReceipt{Provider: "docker", ContainerID: receipt.ContainerID, ExecID: "divergent"})
	require.ErrorIs(t, err, craft.ErrConflict)
}

func TestCraftDockerSendCoordinatorClaimIsOneOwnerAndReplayCannotResend(t *testing.T) {
	coordinator, budget, receipt, tenant, grantID := newCraftDockerCoordinatorFixture(t)
	ctx := context.Background()
	binding := CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}
	first, err := coordinator.Prepare(ctx, grantID, "activity-2", binding)
	require.NoError(t, err)
	second, err := coordinator.Prepare(ctx, grantID, "activity-2", binding)
	require.NoError(t, err)
	require.NoError(t, first.Bind(ctx, receipt))

	start := make(chan struct{})
	claims := make([]DockerSendClaim, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, op := range []*DockerSendOperation{first, second} {
		wg.Add(1)
		go func(i int, op *DockerSendOperation) {
			defer wg.Done()
			<-start
			claims[i], errs[i] = op.Claim(ctx, receipt)
		}(i, op)
	}
	close(start)
	wg.Wait()
	var sendCalls atomic.Int32
	permissions := 0
	for i := range claims {
		require.NoError(t, errs[i])
		if claims[i].Permission != nil {
			receiptOut, ok := claims[i].Permission.Consume()
			require.True(t, ok)
			require.Equal(t, receipt, receiptOut)
			sendCalls.Add(1)
			permissions++
		} else {
			require.True(t, claims[i].Replay)
		}
	}
	require.Equal(t, 1, permissions)
	require.EqualValues(t, 1, sendCalls.Load())

	// After an ambiguous send/crash, the unresolved intent and hold remain;
	// cancellation and process-style replay cannot issue another permission.
	sendCtx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, sendCtx.Err(), context.Canceled)
	postClaimReplay, err := first.Claim(ctx, receipt)
	require.NoError(t, err)
	require.Nil(t, postClaimReplay.Permission)
	require.True(t, postClaimReplay.Replay)
	assertCoordinatorIntentHeld(t, budget, first.journal, true, true)
	reloadedObservation, err := coordinator.Observe(ctx, grantID, first.journal.ActivityKey)
	require.NoError(t, err)
	require.True(t, reloadedObservation.Claimed)
	require.Equal(t, "intent", reloadedObservation.State)
	require.Equal(t, &receipt, reloadedObservation.Receipt)
	var claimed int64
	require.NoError(t, budget.db.Table("craft_charge_start_journal").Where("tenant_id = ? AND run_id = ? AND activity_key = ? AND send_claimed_at IS NOT NULL", tenant, first.journal.RunID, first.journal.ActivityKey).Count(&claimed).Error)
	require.EqualValues(t, 1, claimed)
}

func TestCraftDockerSendCoordinatorStaleRunFenceAndCanceledPreparation(t *testing.T) {
	coordinator, budget, receipt, tenant, grantID := newCraftDockerCoordinatorFixture(t)
	ctx := context.Background()
	binding := CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err := coordinator.Prepare(canceled, grantID, "activity-cancel-before-create", binding)
	require.ErrorIs(t, err, context.Canceled)
	var intents int64
	require.NoError(t, budget.db.Table("craft_charge_start_journal").Where("tenant_id = ? AND activity_key = ?", tenant, "activity-cancel-before-create").Count(&intents).Error)
	require.Zero(t, intents, "cancellation before preparation cannot create intent/hold")

	op, err := coordinator.Prepare(ctx, grantID, "activity-stale", binding)
	require.NoError(t, err)
	require.NoError(t, budget.db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", tenant, op.journal.RunID).Update("revision", op.journal.RunRevision+1).Error)
	require.ErrorIs(t, op.Bind(ctx, receipt), craft.ErrConflict)
	assertCoordinatorIntentHeld(t, budget, op.journal, false, false)
}

func TestCraftDockerSendCoordinatorDoesNotResolveClaimOnTransportError(t *testing.T) {
	coordinator, budget, receipt, tenant, grantID := newCraftDockerCoordinatorFixture(t)
	ctx := context.Background()
	op, err := coordinator.Prepare(ctx, grantID, "activity-unknown", CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform})
	require.NoError(t, err)
	require.NoError(t, op.Bind(ctx, receipt))
	permission, err := op.Claim(ctx, receipt)
	require.NoError(t, err)
	require.NotNil(t, permission.Permission)
	// This layer intentionally exposes no Resolve/definitely-unstarted path:
	// once claimed, any caller-side error leaves the durable hold fenced.
	var state string
	require.NoError(t, budget.db.Table("craft_charge_start_journal").Select("state").Where("tenant_id = ? AND run_id = ? AND activity_key = ?", tenant, op.journal.RunID, op.journal.ActivityKey).Scan(&state).Error)
	require.Equal(t, "intent", state)
	var held int64
	require.NoError(t, budget.db.Table("commercial_reservations").Where("tenant_id = ? AND run_id = ? AND key = ? AND state = 'dispatched'", tenant, op.journal.RunID, op.journal.ReservationKey).Count(&held).Error)
	require.EqualValues(t, 1, held)
}

func assertCoordinatorIntentHeld(t *testing.T, budget *CraftBudgetService, journal CraftChargeStartJournalRow, receiptBound, claimed bool) {
	t.Helper()
	var row struct {
		State         string
		Protocol      *string
		RunRevision   int64
		Provider      *string
		ContainerID   *string
		ExecID        *string
		SendClaimedAt *time.Time
	}
	require.NoError(t, budget.db.Table("craft_charge_start_journal").Where("tenant_id = ? AND run_id = ? AND activity_key = ?", journal.TenantID, journal.RunID, journal.ActivityKey).Take(&row).Error)
	require.Equal(t, "intent", row.State)
	require.NotNil(t, row.Protocol)
	require.Equal(t, craftDockerSendProtocol, *row.Protocol)
	require.Equal(t, journal.RunRevision, row.RunRevision)
	if claimed {
		require.NotNil(t, row.SendClaimedAt)
	} else {
		require.Nil(t, row.SendClaimedAt)
	}
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
	require.NoError(t, budget.db.Table("commercial_reservations").Where("tenant_id = ? AND run_id = ? AND key = ? AND state = 'dispatched'", journal.TenantID, journal.RunID, journal.ReservationKey).Count(&held).Error)
	require.EqualValues(t, 1, held)
}

func TestCraftDockerSendCoordinatorRejectsLegacyIntentAdoption(t *testing.T) {
	coordinator, budget, receipt, tenant, grantID := newCraftDockerCoordinatorFixture(t)
	ctx := context.Background()
	binding := CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}

	// Hold a legacy callback after its intent is committed. The coordinator
	// must not adopt its journal while that independent send path is in flight.
	callbackEntered := make(chan struct{})
	releaseCallback := make(chan struct{})
	legacyDone := make(chan error, 1)
	go func() {
		_, err := budget.StartBinding(ctx, grantID, "activity-cross-protocol-legacy", binding,
			func(context.Context) (CraftChargeStartOutcome, error) {
				close(callbackEntered)
				<-releaseCallback
				return CraftChargeStartStarted, nil
			})
		legacyDone <- err
	}()
	select {
	case <-callbackEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("legacy callback did not enter")
	}
	_, err := coordinator.Prepare(ctx, grantID, "activity-cross-protocol-legacy", binding)
	require.ErrorIs(t, err, craft.ErrConflict, "coordinator must not adopt a legacy callback's outstanding intent")
	close(releaseCallback)
	require.NoError(t, <-legacyDone)

	// The inverse ordering also fences legacy preparation and resolution.
	op, err := coordinator.Prepare(ctx, grantID, "activity-cross-protocol-docker", binding)
	require.NoError(t, err)
	_, err = budget.BeginBinding(ctx, grantID, "activity-cross-protocol-docker", binding)
	require.ErrorIs(t, err, craft.ErrConflict, "legacy prepare cannot enter a Docker-owned journal key")
	var legacyStartCalls atomic.Int32
	_, err = budget.StartBinding(ctx, grantID, "activity-cross-protocol-docker", binding, func(context.Context) (CraftChargeStartOutcome, error) {
		legacyStartCalls.Add(1)
		return CraftChargeStartStarted, nil
	})
	require.ErrorIs(t, err, craft.ErrConflict, "legacy callback path cannot prepare or start a Docker-owned row")
	require.Zero(t, legacyStartCalls.Load())

	legacyJournal := op.journal
	legacyJournal.Protocol = nil // model a stale legacy handle over this key
	legacyAttempt := &craftChargeStartAttempt{service: budget, journal: legacyJournal}
	require.ErrorIs(t, legacyAttempt.Resolve(ctx, CraftChargeStartStarted), craft.ErrConflict,
		"legacy resolution cannot clear a coordinator-owned intent before claim")
	require.NoError(t, op.Bind(ctx, receipt))
	claim, err := op.Claim(ctx, receipt)
	require.NoError(t, err)
	require.NotNil(t, claim.Permission)
	require.ErrorIs(t, legacyAttempt.Resolve(ctx, CraftChargeStartDefinitelyNotStarted), craft.ErrConflict,
		"legacy resolution cannot clear a claimed coordinator intent")
	assertCoordinatorIntentHeld(t, budget, op.journal, true, true)
	restartedBudget, err := NewCraftBudgetService(budget.db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	restartedCoordinator, err := NewCraftDockerSendCoordinator(restartedBudget, repository.NewCraftDockerSendClaimRepository(budget.db))
	require.NoError(t, err)
	observation, err := restartedCoordinator.Observe(ctx, grantID, op.journal.ActivityKey)
	require.NoError(t, err)
	_, err = restartedCoordinator.Prepare(ctx, grantID, op.journal.ActivityKey, binding)
	require.ErrorIs(t, err, craft.ErrConflict, "a claimed operation is observation-only after process reconstruction")
	require.Equal(t, "intent", observation.State)
	require.True(t, observation.Claimed)
	require.Equal(t, tenant, op.journal.TenantID)
}

func openCraftDockerCoordinatorPostgresDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TRPC_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TRPC_TEST_POSTGRES_DSN unset: PostgreSQL coordinator acceptance NOT VERIFIED")
	}
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	schema := "craft_s2_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		require.NoError(t, err)
		query := u.Query()
		query.Set("search_path", schema+",public")
		u.RawQuery = query.Encode()
		dsn = u.String()
	} else {
		dsn += " search_path=" + schema + ",public"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	conn, err := db.DB()
	require.NoError(t, err)
	driver, err := pgmigrate.WithInstance(conn, &pgmigrate.Config{SchemaName: schema})
	require.NoError(t, err)
	m, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/versioned"), "postgres", driver)
	require.NoError(t, err)
	require.NoError(t, m.Up())
	require.NoError(t, db.AutoMigrate(
		&repocommercial.BudgetAccountRow{}, &repocommercial.TaskBudgetRow{},
		&repocommercial.ReservationRow{}, &repocommercial.BudgetLotRow{},
		&repocommercial.BudgetLotAllocationRow{}, &repocommercial.TaskBudgetExtensionRow{},
		&commercialsvc.SettlementRecord{},
	))
	t.Cleanup(func() {
		_, _ = m.Close()
		_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		if adminConn, e := admin.DB(); e == nil {
			_ = adminConn.Close()
		}
		_ = conn.Close()
	})
	return db
}

func TestCraftDockerSendCoordinatorPostgresProtocolGuards(t *testing.T) {
	db := openCraftDockerCoordinatorPostgresDB(t)
	const tenant uint64 = 1701
	const runID = "run-docker-coordinator-pg"
	const sessionID = "task-docker-coordinator-pg"
	require.NoError(t, db.Exec("INSERT INTO tenants (id, name, business) VALUES (?, 'craft s2', 'test')", tenant).Error)
	require.NoError(t, db.Exec("INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES (?, ?, 'budget test', 'owner', 'trpc')", sessionID, tenant).Error)
	now := time.Now().UTC()
	require.NoError(t, db.Exec(`INSERT INTO agent_runs
		(tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, status, wait_reason, deadline, created_at, updated_at)
		VALUES (?, ?, ?, 'owner', ?, ?, 'hash', '{}', 'running', '', ?, ?, ?)`,
		tenant, runID, sessionID, "req-"+runID, "msg-"+runID, now.Add(time.Hour), now, now).Error)
	seedCraftFundedTenant(t, db, tenant, 10000)
	budget, err := NewCraftBudgetService(db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	grant, err := budget.Admit(context.Background(), craft.Scope{TenantID: tenant, UserID: "owner", SessionID: sessionID}, runID)
	require.NoError(t, err)
	coordinator, err := NewCraftDockerSendCoordinator(budget, repository.NewCraftDockerSendClaimRepository(db))
	require.NoError(t, err)
	ctx := context.Background()
	binding := CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}
	op, err := coordinator.Prepare(ctx, grant.ID, "activity-pg-coordinator", binding)
	require.NoError(t, err)
	legacy, err := budget.BeginBinding(ctx, grant.ID, "activity-pg-coordinator", binding)
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Nil(t, legacy)
	legacyJournal := op.journal
	legacyJournal.Protocol = nil
	staleAttempt := &craftChargeStartAttempt{service: budget, journal: legacyJournal}
	require.ErrorIs(t, staleAttempt.Resolve(ctx, CraftChargeStartStarted), craft.ErrConflict)
	var marker struct{ Protocol *string }
	require.NoError(t, db.Table("craft_charge_start_journal").Select("protocol").Where("tenant_id = ? AND run_id = ? AND activity_key = ?", tenant, runID, op.journal.ActivityKey).Take(&marker).Error)
	require.NotNil(t, marker.Protocol)
	require.Equal(t, craftDockerSendProtocol, *marker.Protocol)
	var serverVersion string
	require.NoError(t, db.Raw("SHOW server_version").Scan(&serverVersion).Error)
	t.Logf("disposable PostgreSQL server_version=%s", serverVersion)
}
