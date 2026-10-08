package service

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/stretchr/testify/require"
)

// TestCraftDockerNormalExecReleasesStartEvidenceAfterTerminalOutcome is the
// OCR low-finding regression: the long-lived normal-exec service must not
// retain one start-evidence key per exec forever — the key is released when
// a TERMINAL process observation closes the receipt's evidence window.
func TestCraftDockerNormalExecReleasesStartEvidenceAfterTerminalOutcome(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	coordinator, budget, grantID := seedCraftDockerNormalCoordinator(t, openCraftBudgetTestDB(t), 8751, "run-normal-evict", "task-normal-evict")
	request := craftDockerNormalCoordinatorRequest(8751, "task-normal-evict", "run-normal-evict", "activity-normal-evict")
	provider := &normalExecTestProvider{startResult: sandbox.DockerNormalExecOutcome{
		Transport:     sandbox.DockerNormalExecTransportComplete,
		Observation:   sandbox.DockerNormalExecObservation{State: sandbox.DockerNormalExecProcessSucceeded},
		StartEvidence: true, OutputBytes: 6,
	}}
	output, err := NewCraftDockerOutputService(repository.NewCraftDockerOutputRepository(budget.db, 4096))
	require.NoError(t, err)
	service, err := NewCraftDockerNormalExecService(coordinator, repository.NewCraftDockerNormalInputRepository(budget.db), provider, output)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})

	result, err := service.Execute(context.Background(), grantID, request.ActivityKey,
		CraftCallBinding{ModelID: "model-normal", Funding: commercial.FundingPlatform}, normalExecTestHandle{id: "container-normal-evict"}, request)
	require.NoError(t, err)
	require.True(t, result.StartEvidence)

	service.mu.Lock()
	defer service.mu.Unlock()
	require.Empty(t, service.started,
		"a terminal observation must release the start-evidence key (unbounded growth otherwise)")
}

// TestCraftDockerRestrictedObserveReleasesRunningFlagAtTerminalOnly is the
// OCR low-finding regression for the restricted side: the running flag is
// deleted on a TERMINAL observation (succeeded/failed) and deliberately
// RETAINED on an unknown observation, because the flag is what attributes a
// later terminal zero-exit to failure rather than unknown.
func TestCraftDockerRestrictedObserveReleasesRunningFlagAtTerminalOnly(t *testing.T) {
	coordinator, _, _, _, grantID := newCraftDockerCoordinatorFixture(t)
	binding := CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}
	op, err := coordinator.Prepare(context.Background(), grantID, "activity-running-evict", binding)
	require.NoError(t, err)
	fake := &fakeOutputlessDocker{}
	created, err := fake.CreateOutputlessExec(context.Background(), fakeDockerHandle{}, sandbox.DockerOutputlessExecRequest{DiscardOutput: true, Request: sandbox.RemoteExecRequest{Command: "true"}})
	require.NoError(t, err)
	durable := repository.DockerExecReceipt{Provider: "docker", ContainerID: created.ContainerID, ExecID: created.ExecID}
	require.NoError(t, op.Bind(context.Background(), durable))
	_, err = op.Claim(context.Background(), durable)
	require.NoError(t, err)

	service, err := NewCraftDockerRestrictedExec(coordinator, fake, time.Second)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	key := grantID + "\x00" + "activity-running-evict"

	// An UNKNOWN observation must retain the attribution flag.
	fake.observation = sandbox.DockerOutputlessExecObservation{State: sandbox.DockerOutputlessUnknown}
	service.mu.Lock()
	service.running[key] = true
	service.mu.Unlock()
	_, err = service.Observe(context.Background(), grantID, "activity-running-evict")
	require.NoError(t, err)
	service.mu.Lock()
	require.True(t, service.running[key], "unknown observation keeps the running attribution flag")
	service.mu.Unlock()

	// A TERMINAL observation releases it.
	fake.observation = sandbox.DockerOutputlessExecObservation{State: sandbox.DockerOutputlessSucceeded}
	_, err = service.Observe(context.Background(), grantID, "activity-running-evict")
	require.NoError(t, err)
	service.mu.Lock()
	defer service.mu.Unlock()
	require.False(t, service.running[key], "terminal observation releases the running flag")
}

// TestCraftChargeStartSequenceContentionIsBoundedDomainConflict is the OCR
// finding regression: when the per-binding sequence INSERT races a concurrent
// charge start and the unique index rejects it, the caller must receive the
// bounded DOMAIN conflict (after whole-transaction retries), never a raw
// unique-constraint error. The race loser is simulated by pre-inserting the
// call row the preparation is about to write under the same call_key.
func TestCraftChargeStartSequenceContentionIsBoundedDomainConflict(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	svc, err := NewCraftBudgetService(db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	seedCraftFundedTenant(t, db, 195, 10000)
	ctx := context.Background()
	scope := craft.Scope{TenantID: 195, UserID: "owner", SessionID: "craft107-t19"}
	seedCraftBudgetRun(t, db, 195, "run-contention", scope.SessionID)
	grant, err := svc.Admit(ctx, scope, "run-contention")
	require.NoError(t, err)

	binding := CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}
	// The racing winner's ledger row: same (tenant, call_key) primary key the
	// preparation is about to insert, with an already-committed sequence.
	callKey := CraftCallKey("activity/activity-contention")
	require.NoError(t, db.Create(&CraftBudgetCallRow{
		TenantID: 195, CallKey: callKey, GrantID: grant.ID, RunID: "run-contention",
		DelegationID: binding.DelegationID, ModelID: binding.ModelID, Funding: binding.Funding,
		CallSeq: 1, CallID: "activity/activity-contention", CreatedAt: time.Now(),
	}).Error)

	_, err = svc.StartBinding(ctx, grant.ID, "activity-contention", binding, func(context.Context) (CraftChargeStartOutcome, error) {
		return CraftChargeStartStarted, nil
	})
	require.ErrorIs(t, err, craft.ErrConflict,
		"sequence contention must surface as the bounded domain conflict, not a raw constraint error")
}

// permissiveExecPolicyGate is the TEST-ONLY gate that admits everything:
// production faces are now fail-closed without a real gate, so every test
// construction must explicitly opt in.
type permissiveExecPolicyGate struct{}

func (permissiveExecPolicyGate) ReviewNormalExec(context.Context, repository.CraftDockerNormalInputRequest) error {
	return nil
}
func (permissiveExecPolicyGate) ReviewOutputlessExec(context.Context, CraftCallBinding, CraftDockerOutputlessRequest) error {
	return nil
}
