package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	repository "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/commercial"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
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
		RunRevision   int64
		Provider      *string
		ContainerID   *string
		ExecID        *string
		SendClaimedAt *time.Time
	}
	require.NoError(t, budget.db.Table("craft_charge_start_journal").Where("tenant_id = ? AND run_id = ? AND activity_key = ?", journal.TenantID, journal.RunID, journal.ActivityKey).Take(&row).Error)
	require.Equal(t, "intent", row.State)
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
