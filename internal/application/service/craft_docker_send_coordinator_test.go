package service

import (
	"context"
	"crypto/sha256"
	"fmt"
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
	"github.com/Tencent/WeKnora/internal/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/commercial/service/commercial"
	"github.com/Tencent/WeKnora/internal/craft"
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
	// The local isolated PostgreSQL target does not install project-wide
	// optional extensions. The coordinator fixture only needs uuid-ossp; skip
	// the unrelated vector/pg_search migration so this verifies the service
	// path without claiming a full production migration chain.
	schema := "craft_s2_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	require.NoError(t, admin.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA `+schema).Error)
	// Extension installation can be database-wide and already pinned in
	// another schema. Ensure the migration session's private search_path can
	// still resolve the UUID default without relying on public visibility.
	require.NoError(t, admin.Exec("CREATE OR REPLACE FUNCTION "+schema+`.uuid_generate_v4() RETURNS uuid LANGUAGE SQL VOLATILE AS 'SELECT gen_random_uuid()'`).Error)
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		require.NoError(t, err)
		query := u.Query()
		query.Set("search_path", schema+",public")
		query.Set("options", "-c app.skip_embedding=true")
		u.RawQuery = query.Encode()
		dsn = u.String()
	} else {
		dsn += " search_path=" + schema + ",public options='-c app.skip_embedding=true'"
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

func forEachCraftDockerCoordinatorNormalDB(t *testing.T, fn func(*testing.T, *gorm.DB)) {
	t.Helper()
	t.Run("sqlite", func(t *testing.T) { fn(t, openCraftBudgetTestDB(t)) })
	if os.Getenv("TRPC_TEST_POSTGRES_DSN") == "" {
		t.Log("PostgreSQL normal coordinator acceptance skipped: TRPC_TEST_POSTGRES_DSN unset")
		return
	}
	t.Run("postgres", func(t *testing.T) { fn(t, openCraftDockerCoordinatorPostgresDB(t)) })
}

func seedCraftDockerNormalCoordinator(t *testing.T, db *gorm.DB, tenant uint64, runID, sessionID string) (*CraftDockerSendCoordinator, *CraftBudgetService, string) {
	t.Helper()
	seedCraftFundedTenant(t, db, tenant, 10000)
	if db.Dialector.Name() == "postgres" {
		now := time.Now().UTC()
		require.NoError(t, db.Exec("INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES (?, ?, 'budget test', 'owner', 'trpc') ON CONFLICT (id) DO NOTHING", sessionID, tenant).Error)
		require.NoError(t, db.Exec(`INSERT INTO agent_runs
			(tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, status, wait_reason, deadline, created_at, updated_at)
			VALUES (?, ?, ?, 'owner', ?, ?, 'hash', '{}', 'running', '', ?, ?, ?)`,
			tenant, runID, sessionID, "req-"+runID, "msg-"+runID, now.Add(time.Hour), now, now).Error)
	} else {
		seedCraftBudgetRun(t, db, tenant, runID, sessionID)
	}
	budget, err := NewCraftBudgetService(db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	grant, err := budget.Admit(context.Background(), craft.Scope{TenantID: tenant, UserID: "owner", SessionID: sessionID}, runID)
	require.NoError(t, err)
	coordinator, err := NewCraftDockerSendCoordinator(budget, repository.NewCraftDockerSendClaimRepository(db))
	require.NoError(t, err)
	return coordinator, budget, grant.ID
}

func craftDockerNormalCoordinatorRequest(tenant uint64, taskID, runID, activity string) repository.CraftDockerNormalInputRequest {
	return repository.CraftDockerNormalInputRequest{
		TenantID: tenant, TaskID: taskID, RunID: runID, ActivityKey: activity,
		Command: []string{"/bin/sh", "-lc", "printf normal"}, Environment: map[string]string{"MODE": "normal"},
		User: "1000:1000", WorkingDir: "/workspace", TimeoutMillis: 30000,
		StdinEnabled: true, Stdin: []byte("normal input"), OutputLimit: 4096, OutputPolicy: "bounded-partial-v1",
	}
}

func TestCraftDockerNormalCoordinatorStagesBeforeCreateAndRejectsChangedReplay(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	forEachCraftDockerCoordinatorNormalDB(t, func(t *testing.T, db *gorm.DB) {
		ctx := context.Background()
		const tenant uint64 = 2701
		const runID, taskID, activity = "run-normal-coordinator", "task-normal-coordinator", "activity-normal-coordinator"
		coordinator, budget, grantID := seedCraftDockerNormalCoordinator(t, db, tenant, runID, taskID)
		binding := CraftCallBinding{DelegationID: "delegate", ModelID: "model", Funding: commercial.FundingPlatform}
		request := craftDockerNormalCoordinatorRequest(tenant, taskID, runID, activity)

		first, staged, err := coordinator.PrepareNormal(ctx, grantID, activity, binding, request)
		require.NoError(t, err)
		require.Equal(t, request, staged.Request)
		require.Nil(t, staged.Receipt, "input must be durable before an inert provider create")
		assertCoordinatorIntentHeld(t, budget, first.send.journal, false, false)
		projection := first.StagedInput()
		projection.Request.Command[0] = "/bin/false"
		projection.Request.Environment["MODE"] = "mutated"
		projection.Request.Stdin[0] = 'X'
		require.Equal(t, request, first.StagedInput().Request, "the provider-facing copy cannot mutate durable identity")
		_, err = first.Claim(ctx, craftDockerNormalCoordinatorReceipt(request, "early-container", "early-exec"))
		require.ErrorIs(t, err, craft.ErrConflict, "a staged request cannot be claimed without the full bound receipt")

		// Replaying after inert ExecCreate but before Bind may reuse only this
		// exact staged identity; no external provider I/O occurs in the service.
		replay, replayStage, err := coordinator.PrepareNormal(ctx, grantID, activity, binding, request)
		require.NoError(t, err)
		require.Equal(t, first.send.journal, replay.send.journal)
		require.Equal(t, staged.RequestSHA256, replayStage.RequestSHA256)

		changedCommand := cloneCraftDockerNormalCoordinatorRequest(request)
		changedCommand.Command[2] = "printf changed"
		_, _, err = coordinator.PrepareNormal(ctx, grantID, activity, binding, changedCommand)
		require.ErrorIs(t, err, craft.ErrConflict, "changed command must fail before the provider can create")
		changedTimeout := cloneCraftDockerNormalCoordinatorRequest(request)
		changedTimeout.TimeoutMillis++
		_, _, err = coordinator.PrepareNormal(ctx, grantID, activity, binding, changedTimeout)
		require.ErrorIs(t, err, craft.ErrConflict, "changed timeout must fail before the provider can create")
		changedBinding := binding
		changedBinding.ModelID = "other-model"
		_, _, err = coordinator.PrepareNormal(ctx, grantID, activity, changedBinding, request)
		require.ErrorIs(t, err, craft.ErrConflict)

		var intents, calls int64
		require.NoError(t, db.Table("craft_charge_start_journal").Where("tenant_id = ? AND run_id = ? AND activity_key = ?", tenant, runID, activity).Count(&intents).Error)
		require.NoError(t, db.Table("craft_budget_calls").Where("tenant_id = ? AND run_id = ?", tenant, runID).Count(&calls).Error)
		require.EqualValues(t, 1, intents)
		require.EqualValues(t, 1, calls, "exact replay and rejected changed inputs cannot allocate a second hold")
	})
}

func TestCraftDockerNormalCoordinatorResumeBoundAndConcurrentClaimAreObservationSafe(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	forEachCraftDockerCoordinatorNormalDB(t, func(t *testing.T, db *gorm.DB) {
		ctx := context.Background()
		const tenant uint64 = 2702
		const runID, taskID, activity = "run-normal-resume", "task-normal-resume", "activity-normal-resume"
		coordinator, budget, grantID := seedCraftDockerNormalCoordinator(t, db, tenant, runID, taskID)
		binding := CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}
		request := craftDockerNormalCoordinatorRequest(tenant, taskID, runID, activity)
		operation, _, err := coordinator.PrepareNormal(ctx, grantID, activity, binding, request)
		require.NoError(t, err)

		receipt := craftDockerNormalCoordinatorReceipt(request, "normal-container", "normal-exec")
		// Simulate a crash after the one inert ExecCreate and successful full bind.
		require.NoError(t, operation.Bind(ctx, receipt))
		assertCoordinatorIntentHeld(t, budget, operation.send.journal, true, false)
		restartedBudget, err := NewCraftBudgetService(db, nil, craftBudgetPolicy())
		require.NoError(t, err)
		restarted, err := NewCraftDockerSendCoordinator(restartedBudget, repository.NewCraftDockerSendClaimRepository(db))
		require.NoError(t, err)
		resumed, resumedStage, err := restarted.ResumeBoundNormal(ctx, grantID, activity, request)
		require.NoError(t, err)
		require.Equal(t, receipt, *resumedStage.Receipt)
		alteredReceipt := receipt
		alteredReceipt.TimeoutMillis++
		require.ErrorIs(t, resumed.Bind(ctx, alteredReceipt), craft.ErrConflict)
		_, err = resumed.Claim(ctx, alteredReceipt)
		require.ErrorIs(t, err, craft.ErrConflict)

		changedStdin := cloneCraftDockerNormalCoordinatorRequest(request)
		changedStdin.Stdin = []byte("different input")
		_, _, err = restarted.ResumeBoundNormal(ctx, grantID, activity, changedStdin)
		require.ErrorIs(t, err, craft.ErrConflict, "recovery must be tied to the exact encrypted staged request")
		changedTimeout := cloneCraftDockerNormalCoordinatorRequest(request)
		changedTimeout.TimeoutMillis++
		_, _, err = restarted.ResumeBoundNormal(ctx, grantID, activity, changedTimeout)
		require.ErrorIs(t, err, craft.ErrConflict)

		// Two reconstructed operations may race the durable claim; only one may
		// obtain the process-local permission. A claimed replay is observation only.
		second, _, err := restarted.ResumeBoundNormal(ctx, grantID, activity, request)
		require.NoError(t, err)
		start := make(chan struct{})
		claims := make([]DockerSendClaim, 2)
		errs := make([]error, 2)
		var wg sync.WaitGroup
		for i, op := range []*DockerNormalSendOperation{resumed, second} {
			wg.Add(1)
			go func(i int, op *DockerNormalSendOperation) {
				defer wg.Done()
				<-start
				claims[i], errs[i] = op.Claim(ctx, receipt)
			}(i, op)
		}
		close(start)
		wg.Wait()
		permissions := 0
		for i := range claims {
			require.NoError(t, errs[i])
			if claims[i].Permission != nil {
				actual, ok := claims[i].Permission.Consume()
				require.True(t, ok)
				require.Equal(t, receipt.DockerExecReceipt, actual)
				permissions++
			} else {
				require.True(t, claims[i].Replay)
			}
		}
		require.Equal(t, 1, permissions)
		assertCoordinatorIntentHeld(t, budget, operation.send.journal, true, true)
		_, _, err = restarted.ResumeBoundNormal(ctx, grantID, activity, request)
		require.ErrorIs(t, err, craft.ErrConflict, "claimed replay cannot regain a start token")
	})
}

func TestCraftDockerNormalCoordinatorFencesStaleRunBeforeBind(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	forEachCraftDockerCoordinatorNormalDB(t, func(t *testing.T, db *gorm.DB) {
		ctx := context.Background()
		const tenant uint64 = 2703
		const runID, taskID, activity = "run-normal-stale", "task-normal-stale", "activity-normal-stale"
		coordinator, budget, grantID := seedCraftDockerNormalCoordinator(t, db, tenant, runID, taskID)
		request := craftDockerNormalCoordinatorRequest(tenant, taskID, runID, activity)
		operation, _, err := coordinator.PrepareNormal(ctx, grantID, activity, CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}, request)
		require.NoError(t, err)
		require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", tenant, runID).Update("revision", operation.send.journal.RunRevision+1).Error)
		receipt := craftDockerNormalCoordinatorReceipt(request, "stale-container", "stale-exec")
		require.ErrorIs(t, operation.Bind(ctx, receipt), craft.ErrConflict)
		stage, err := repository.NewCraftDockerNormalInputRepository(db).Read(ctx, repository.CraftChargeStartKey{TenantID: tenant, RunID: runID, ActivityKey: activity})
		require.NoError(t, err)
		require.Nil(t, stage.Receipt, "stale Run cannot bind or claim a created exec")
		assertCoordinatorIntentHeld(t, budget, operation.send.journal, false, false)
	})
}

func TestCraftDockerNormalCoordinatorRecoversInterruptedFullReceiptBind(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	forEachCraftDockerCoordinatorNormalDB(t, func(t *testing.T, db *gorm.DB) {
		ctx := context.Background()
		const tenant uint64 = 2704
		const runID, taskID, activity = "run-normal-partial-bind", "task-normal-partial-bind", "activity-normal-partial-bind"
		coordinator, _, grantID := seedCraftDockerNormalCoordinator(t, db, tenant, runID, taskID)
		request := craftDockerNormalCoordinatorRequest(tenant, taskID, runID, activity)
		_, _, err := coordinator.PrepareNormal(ctx, grantID, activity, CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}, request)
		require.NoError(t, err)

		// Simulate interruption immediately after the S2 journal receipt CAS but
		// before the normal-input repository updates its full receipt columns.
		basic := repository.DockerExecReceipt{Provider: "docker", ContainerID: "partial-container", ExecID: "partial-exec"}
		require.NoError(t, db.Table("craft_charge_start_journal").Where("tenant_id = ? AND run_id = ? AND activity_key = ?", tenant, runID, activity).
			Updates(map[string]any{"provider": basic.Provider, "container_id": basic.ContainerID, "exec_id": basic.ExecID}).Error)

		restartedBudget, err := NewCraftBudgetService(db, nil, craftBudgetPolicy())
		require.NoError(t, err)
		restarted, err := NewCraftDockerSendCoordinator(restartedBudget, repository.NewCraftDockerSendClaimRepository(db))
		require.NoError(t, err)
		resumed, staged, err := restarted.ResumeBoundNormal(ctx, grantID, activity, request)
		require.NoError(t, err, "exact encrypted input plus persisted Docker IDs safely completes receipt persistence")
		expected := craftDockerNormalCoordinatorReceipt(request, basic.ContainerID, basic.ExecID)
		require.Equal(t, &expected, staged.Receipt)
		claim, err := resumed.Claim(ctx, expected)
		require.NoError(t, err)
		require.NotNil(t, claim.Permission)
	})
}

func cloneCraftDockerNormalCoordinatorRequest(in repository.CraftDockerNormalInputRequest) repository.CraftDockerNormalInputRequest {
	out := in
	out.Command = append([]string(nil), in.Command...)
	out.Stdin = append([]byte(nil), in.Stdin...)
	out.Environment = make(map[string]string, len(in.Environment))
	for k, v := range in.Environment {
		out.Environment[k] = v
	}
	return out
}

func craftDockerNormalCoordinatorReceipt(request repository.CraftDockerNormalInputRequest, containerID, execID string) repository.CraftDockerNormalReceipt {
	return repository.CraftDockerNormalReceipt{
		DockerExecReceipt: repository.DockerExecReceipt{Provider: "docker", ContainerID: containerID, ExecID: execID},
		StdinEnabled:      request.StdinEnabled, StdinByteCount: int64(len(request.Stdin)),
		StdinSHA256: fmt.Sprintf("%x", sha256.Sum256(request.Stdin)), TimeoutMillis: request.TimeoutMillis,
	}
}
