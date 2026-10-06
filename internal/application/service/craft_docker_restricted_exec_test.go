package service

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	repository "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/commercial"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/google/uuid"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
)

type fakeOutputlessDocker struct {
	createCalls        int
	startCalls         int
	startErr           error
	observation        sandbox.DockerOutputlessExecObservation
	observations       []sandbox.DockerOutputlessExecObservation
	observeCalls       int
	observeErr         error
	receipts           []sandbox.DockerOutputlessExecReceipt
	runningSeen        []bool
	observeEntered     chan struct{}
	observeRelease     chan struct{}
	observeDone        <-chan struct{}
	blockOnObserveCall int
	afterCreate        func() error
}

func (f *fakeOutputlessDocker) CreateOutputlessExec(context.Context, sandbox.RemoteSandboxHandle, sandbox.DockerOutputlessExecRequest) (sandbox.DockerOutputlessExecReceipt, error) {
	f.createCalls++
	if f.afterCreate != nil {
		if err := f.afterCreate(); err != nil {
			return sandbox.DockerOutputlessExecReceipt{}, err
		}
	}
	return sandbox.DockerOutputlessExecReceipt{ContainerID: "container-fake", ExecID: "exec-fake"}, nil
}
func (f *fakeOutputlessDocker) StartOutputlessExec(context.Context, sandbox.DockerOutputlessExecReceipt, time.Duration) error {
	f.startCalls++
	return f.startErr
}
func (f *fakeOutputlessDocker) ObserveOutputlessExec(ctx context.Context, receipt sandbox.DockerOutputlessExecReceipt, running bool) (sandbox.DockerOutputlessExecObservation, error) {
	f.observeCalls++
	f.receipts = append(f.receipts, receipt)
	f.runningSeen = append(f.runningSeen, running)
	if f.observeEntered != nil && f.observeCalls == f.blockOnObserveCall {
		f.observeDone = ctx.Done()
		close(f.observeEntered)
		<-f.observeRelease // deliberately ignore cancellation to model an inspect that returns late
	}
	if f.observeErr != nil {
		return sandbox.DockerOutputlessExecObservation{State: sandbox.DockerOutputlessUnknown}, f.observeErr
	}
	if len(f.observations) > 0 {
		index := f.observeCalls - 1
		if index >= len(f.observations) {
			index = len(f.observations) - 1
		}
		return f.observations[index], nil
	}
	return f.observation, nil
}

type fakeDockerHandle struct{}

func (fakeDockerHandle) ID() string                       { return "container-fake" }
func (fakeDockerHandle) Provider() sandbox.RemoteProvider { return sandbox.SandboxTypeDocker }
func (fakeDockerHandle) Metadata() map[string]string      { return nil }

func TestCraftDockerRestrictedRejectsUnsupportedContractBeforeHold(t *testing.T) {
	coordinator, budget, _, tenant, grantID := newCraftDockerCoordinatorFixture(t)
	service, err := NewCraftDockerRestrictedExec(coordinator, &sandbox.DockerRemoteClient{}, 0)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	callback := func(string, []byte) {}
	for _, request := range []CraftDockerOutputlessRequest{
		{Exec: sandbox.RemoteExecRequest{Command: "true"}},
		{DiscardOutput: true, Exec: sandbox.RemoteExecRequest{Command: "true", Stdin: "input"}},
		{DiscardOutput: true, Exec: sandbox.RemoteExecRequest{Command: "true", OnOutput: callback}},
	} {
		_, err := service.Start(context.Background(), grantID, "activity-rejected", CraftCallBinding{}, nil, request)
		require.Error(t, err)
	}
	var rows int64
	require.NoError(t, budget.db.Table("craft_charge_start_journal").Where("tenant_id = ? AND activity_key = ?", tenant, "activity-rejected").Count(&rows).Error)
	require.Zero(t, rows, "unsupported output contracts are rejected before any durable hold")
}

func TestCraftDockerRestrictedPhysicalStartRequiresExclusiveDurableClaim(t *testing.T) {
	coordinator, budget, _, tenant, grantID := newCraftDockerCoordinatorFixture(t)
	docker := &fakeOutputlessDocker{}
	service, err := NewCraftDockerRestrictedExec(coordinator, docker, time.Second)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	request := CraftDockerOutputlessRequest{DiscardOutput: true, Exec: sandbox.RemoteExecRequest{Command: "true"}}
	binding := CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}
	result, err := service.Start(context.Background(), grantID, "activity-one-send", binding, fakeDockerHandle{}, request)
	require.NoError(t, err)
	require.True(t, result.Accepted)
	require.False(t, result.StdoutAvailable)
	require.False(t, result.StderrAvailable)
	require.Equal(t, 1, docker.createCalls)
	require.Equal(t, 1, docker.startCalls)
	_, err = service.ResumeBound(context.Background(), grantID, "activity-one-send")
	require.Error(t, err, "claimed receipt cannot be resumed into another physical start")
	require.Equal(t, 1, docker.startCalls)
	var claimed int64
	require.NoError(t, budget.db.Table("craft_charge_start_journal").Where("tenant_id = ? AND activity_key = ? AND send_claimed_at IS NOT NULL", tenant, "activity-one-send").Count(&claimed).Error)
	require.EqualValues(t, 1, claimed)
}

func TestCraftDockerRestrictedLostStartResponseStaysUnknownWithoutRetry(t *testing.T) {
	coordinator, budget, _, tenant, grantID := newCraftDockerCoordinatorFixture(t)
	docker := &fakeOutputlessDocker{startErr: errors.New("connection lost after daemon acceptance")}
	service, err := NewCraftDockerRestrictedExec(coordinator, docker, time.Second)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	request := CraftDockerOutputlessRequest{DiscardOutput: true, Exec: sandbox.RemoteExecRequest{Command: "true"}}
	result, err := service.Start(context.Background(), grantID, "activity-lost-response", CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}, fakeDockerHandle{}, request)
	require.ErrorIs(t, err, sandbox.ErrRemoteOperationUnknown)
	require.False(t, result.StdoutAvailable)
	_, resumeErr := service.ResumeBound(context.Background(), grantID, "activity-lost-response")
	require.Error(t, resumeErr)
	require.Equal(t, 1, docker.startCalls)
	var row struct {
		State         string
		SendClaimedAt *time.Time
	}
	require.NoError(t, budget.db.Table("craft_charge_start_journal").Select("state, send_claimed_at").Where("tenant_id = ? AND activity_key = ?", tenant, "activity-lost-response").Take(&row).Error)
	require.Equal(t, "intent", row.State)
	require.NotNil(t, row.SendClaimedAt)
}

type boundaryCanceledContext struct{ context.Context }

// The context becomes observably canceled at the claim boundary while keeping
// its Done channel open during the database call. This isolates the fence
// between a successful durable Claim and provider start.
func (boundaryCanceledContext) Err() error { return context.Canceled }

func TestCraftDockerRestrictedCancellationAfterClaimSkipsProviderStart(t *testing.T) {
	coordinator, budget, _, tenant, grantID := newCraftDockerCoordinatorFixture(t)
	docker := &fakeOutputlessDocker{}
	service, err := NewCraftDockerRestrictedExec(coordinator, docker, time.Second)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	ctx := boundaryCanceledContext{Context: context.Background()}
	result, err := service.Start(ctx, grantID, "activity-cancel-after-claim", CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}, fakeDockerHandle{}, CraftDockerOutputlessRequest{DiscardOutput: true, Exec: sandbox.RemoteExecRequest{Command: "true"}})
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, err, sandbox.ErrRemoteOperationUnknown)
	require.False(t, result.Accepted)
	require.Equal(t, 1, docker.createCalls)
	require.Zero(t, docker.startCalls, "canceled context must be fenced after durable claim and before provider start")
	var row struct {
		State         string
		SendClaimedAt *time.Time
	}
	require.NoError(t, budget.db.Table("craft_charge_start_journal").Select("state, send_claimed_at").Where("tenant_id = ? AND activity_key = ?", tenant, "activity-cancel-after-claim").Take(&row).Error)
	require.Equal(t, "intent", row.State)
	require.NotNil(t, row.SendClaimedAt, "claim remains durable and must not be released")
}

func TestCraftDockerRestrictedWaitReturnsOutputlessTerminalState(t *testing.T) {
	coordinator, _, _, _, grantID := newCraftDockerCoordinatorFixture(t)
	docker := &fakeOutputlessDocker{observations: []sandbox.DockerOutputlessExecObservation{
		{State: sandbox.DockerOutputlessRunning},
		{State: sandbox.DockerOutputlessRunning},
		{State: sandbox.DockerOutputlessSucceeded, ExitCode: ptr(0)},
	}}
	service, err := NewCraftDockerRestrictedExec(coordinator, docker, time.Second)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	result, err := service.Start(context.Background(), grantID, "activity-wait-terminal", CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}, fakeDockerHandle{}, CraftDockerOutputlessRequest{DiscardOutput: true, Exec: sandbox.RemoteExecRequest{Command: "true"}})
	require.NoError(t, err)
	terminal, err := service.Wait(context.Background(), grantID, "activity-wait-terminal", time.Second)
	require.NoError(t, err)
	require.Equal(t, result.Receipt, terminal.Receipt)
	require.Equal(t, sandbox.DockerOutputlessSucceeded, terminal.State)
	require.NotNil(t, terminal.ExitCode)
	require.Equal(t, 0, *terminal.ExitCode)
	require.False(t, terminal.StdoutAvailable)
	require.False(t, terminal.StderrAvailable)
	require.False(t, terminal.DurationAvailable, "Docker exec inspect has no authoritative execution duration")
	require.Equal(t, 3, docker.observeCalls)
	for i, receipt := range docker.receipts {
		require.Equal(t, sandbox.DockerOutputlessExecReceipt{ContainerID: result.Receipt.ContainerID, ExecID: result.Receipt.ExecID}, receipt)
		if i > 0 {
			require.True(t, docker.runningSeen[i], "running provenance must carry forward for this exact receipt")
		}
	}
}

func TestCraftDockerRestrictedWaitCancellationRemainsUnknown(t *testing.T) {
	coordinator, budget, _, tenant, grantID := newCraftDockerCoordinatorFixture(t)
	docker := &fakeOutputlessDocker{observations: []sandbox.DockerOutputlessExecObservation{{State: sandbox.DockerOutputlessRunning}}}
	service, err := NewCraftDockerRestrictedExec(coordinator, docker, time.Second)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	_, err = service.Start(context.Background(), grantID, "activity-wait-canceled", CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}, fakeDockerHandle{}, CraftDockerOutputlessRequest{DiscardOutput: true, Exec: sandbox.RemoteExecRequest{Command: "true"}})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := service.Wait(ctx, grantID, "activity-wait-canceled", time.Second)
	require.ErrorIs(t, err, sandbox.ErrRemoteOperationUnknown)
	require.Equal(t, sandbox.DockerOutputlessUnknown, result.State)
	require.False(t, result.StdoutAvailable)
	require.False(t, result.StderrAvailable)
	require.Zero(t, docker.observeCalls, "canceled wait must not start a poll")
	var row struct{ SendClaimedAt *time.Time }
	require.NoError(t, budget.db.Table("craft_charge_start_journal").Select("send_claimed_at").Where("tenant_id = ? AND activity_key = ?", tenant, "activity-wait-canceled").Take(&row).Error)
	require.NotNil(t, row.SendClaimedAt, "wait cancellation must preserve durable hold/claim")
}

func TestCraftDockerRestrictedWaitDeadlineRemainsUnknown(t *testing.T) {
	coordinator, _, _, _, grantID := newCraftDockerCoordinatorFixture(t)
	docker := &fakeOutputlessDocker{observations: []sandbox.DockerOutputlessExecObservation{{State: sandbox.DockerOutputlessRunning}}}
	service, err := NewCraftDockerRestrictedExec(coordinator, docker, time.Second)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	_, err = service.Start(context.Background(), grantID, "activity-wait-deadline", CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}, fakeDockerHandle{}, CraftDockerOutputlessRequest{DiscardOutput: true, Exec: sandbox.RemoteExecRequest{Command: "true"}})
	require.NoError(t, err)
	result, err := service.Wait(context.Background(), grantID, "activity-wait-deadline", 15*time.Millisecond)
	require.ErrorIs(t, err, sandbox.ErrRemoteOperationUnknown)
	require.Equal(t, sandbox.DockerOutputlessUnknown, result.State)
	require.False(t, result.DurationAvailable, "deadline does not provide execution duration")
}

func ptr[T any](value T) *T { return &value }

func TestCraftDockerRestrictedWaitInspectErrorRemainsUnknown(t *testing.T) {
	coordinator, budget, _, tenant, grantID := newCraftDockerCoordinatorFixture(t)
	docker := &fakeOutputlessDocker{observeErr: errors.New("inspect unavailable")}
	service, err := NewCraftDockerRestrictedExec(coordinator, docker, time.Second)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	_, err = service.Start(context.Background(), grantID, "activity-wait-inspect-error", CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}, fakeDockerHandle{}, CraftDockerOutputlessRequest{DiscardOutput: true, Exec: sandbox.RemoteExecRequest{Command: "true"}})
	require.NoError(t, err)
	result, err := service.Wait(context.Background(), grantID, "activity-wait-inspect-error", time.Second)
	require.ErrorIs(t, err, sandbox.ErrRemoteOperationUnknown)
	require.ErrorContains(t, err, "inspect unavailable")
	require.Equal(t, sandbox.DockerOutputlessUnknown, result.State)
	require.Equal(t, 1, docker.observeCalls)
	var row struct{ SendClaimedAt *time.Time }
	require.NoError(t, budget.db.Table("craft_charge_start_journal").Select("send_claimed_at").Where("tenant_id = ? AND activity_key = ?", tenant, "activity-wait-inspect-error").Take(&row).Error)
	require.NotNil(t, row.SendClaimedAt)
}

func TestCraftDockerRestrictedWaitReturnsFastTerminalState(t *testing.T) {
	coordinator, _, _, _, grantID := newCraftDockerCoordinatorFixture(t)
	docker := &fakeOutputlessDocker{observations: []sandbox.DockerOutputlessExecObservation{
		{State: sandbox.DockerOutputlessRunning},
		{State: sandbox.DockerOutputlessFailed, ExitCode: ptr(7)},
	}}
	service, err := NewCraftDockerRestrictedExec(coordinator, docker, time.Second)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	_, err = service.Start(context.Background(), grantID, "activity-wait-fast", CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}, fakeDockerHandle{}, CraftDockerOutputlessRequest{DiscardOutput: true, Exec: sandbox.RemoteExecRequest{Command: "false"}})
	require.NoError(t, err)
	result, err := service.Wait(context.Background(), grantID, "activity-wait-fast", time.Second)
	require.NoError(t, err)
	require.Equal(t, sandbox.DockerOutputlessFailed, result.State)
	require.NotNil(t, result.ExitCode)
	require.Equal(t, 7, *result.ExitCode)
	require.False(t, result.DurationAvailable)
	require.False(t, result.StdoutAvailable)
	require.False(t, result.StderrAvailable)
}

func TestCraftDockerRestrictedWaitCancellationDuringInspectRejectsLateTerminal(t *testing.T) {
	for _, tc := range []struct {
		name     string
		state    sandbox.DockerOutputlessExecState
		exitCode int
		deadline bool
	}{
		{name: "late success after cancellation", state: sandbox.DockerOutputlessSucceeded, exitCode: 0},
		{name: "late failure after deadline", state: sandbox.DockerOutputlessFailed, exitCode: 23, deadline: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			coordinator, budget, _, tenant, grantID := newCraftDockerCoordinatorFixture(t)
			docker := &fakeOutputlessDocker{
				observations: []sandbox.DockerOutputlessExecObservation{
					{State: sandbox.DockerOutputlessRunning},
					{State: tc.state, ExitCode: ptr(tc.exitCode)},
				},
				observeEntered:     make(chan struct{}),
				observeRelease:     make(chan struct{}),
				blockOnObserveCall: 2,
			}
			service, err := NewCraftDockerRestrictedExec(coordinator, docker, time.Second)
			require.NoError(t, err)
			service.WithExecutionPolicy(&permissiveExecPolicyGate{})
			_, err = service.Start(context.Background(), grantID, "activity-late-terminal", CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}, fakeDockerHandle{}, CraftDockerOutputlessRequest{DiscardOutput: true, Exec: sandbox.RemoteExecRequest{Command: "true"}})
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			maxWait := time.Second
			if tc.deadline {
				maxWait = 100 * time.Millisecond
			}
			type waitResult struct {
				result CraftDockerOutputlessResult
				err    error
			}
			done := make(chan waitResult, 1)
			go func() {
				result, waitErr := service.Wait(ctx, grantID, "activity-late-terminal", maxWait)
				done <- waitResult{result: result, err: waitErr}
			}()
			<-docker.observeEntered
			require.NotNil(t, docker.observeDone)
			if tc.deadline {
				<-docker.observeDone
			} else {
				cancel()
				<-docker.observeDone
			}
			close(docker.observeRelease)
			got := <-done
			require.ErrorIs(t, got.err, sandbox.ErrRemoteOperationUnknown)
			require.Equal(t, sandbox.DockerOutputlessUnknown, got.result.State)
			require.Equal(t, repository.DockerExecReceipt{Provider: "docker", ContainerID: "container-fake", ExecID: "exec-fake"}, got.result.Receipt)
			require.False(t, got.result.StdoutAvailable)
			require.False(t, got.result.StderrAvailable)
			require.False(t, got.result.DurationAvailable)
			var row struct{ SendClaimedAt *time.Time }
			require.NoError(t, budget.db.Table("craft_charge_start_journal").Select("send_claimed_at").Where("tenant_id = ? AND activity_key = ?", tenant, "activity-late-terminal").Take(&row).Error)
			require.NotNil(t, row.SendClaimedAt, "late terminal must not release the durable claim")
		})
	}
}

func TestCraftDockerRestrictedWaitFirstFalseZeroRemainsUnknown(t *testing.T) {
	coordinator, budget, _, tenant, grantID := newCraftDockerCoordinatorFixture(t)
	docker := &fakeOutputlessDocker{observations: []sandbox.DockerOutputlessExecObservation{{State: sandbox.DockerOutputlessUnknown, ExitCode: ptr(0)}}}
	service, err := NewCraftDockerRestrictedExec(coordinator, docker, time.Second)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	_, err = service.Start(context.Background(), grantID, "activity-first-false-zero", CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}, fakeDockerHandle{}, CraftDockerOutputlessRequest{DiscardOutput: true, Exec: sandbox.RemoteExecRequest{Command: "true"}})
	require.NoError(t, err)
	result, err := service.Wait(context.Background(), grantID, "activity-first-false-zero", 45*time.Millisecond)
	require.ErrorIs(t, err, sandbox.ErrRemoteOperationUnknown)
	require.Equal(t, sandbox.DockerOutputlessUnknown, result.State)
	require.Nil(t, result.ExitCode, "false/zero without prior running provenance is not a terminal result")
	require.False(t, result.StdoutAvailable)
	require.False(t, result.StderrAvailable)
	require.GreaterOrEqual(t, docker.observeCalls, 1)
	require.False(t, docker.runningSeen[0], "the first inspect has no positive running provenance")
	var row struct{ SendClaimedAt *time.Time }
	require.NoError(t, budget.db.Table("craft_charge_start_journal").Select("send_claimed_at").Where("tenant_id = ? AND activity_key = ?", tenant, "activity-first-false-zero").Take(&row).Error)
	require.NotNil(t, row.SendClaimedAt)
}

type recordedOutputlessDocker struct {
	delegate       sandbox.DockerOutputlessExecClient
	startCalls     atomic.Int32
	sideEffects    atomic.Int32
	mu             sync.Mutex
	startReceipts  []sandbox.DockerOutputlessExecReceipt
	observeReceipt []sandbox.DockerOutputlessExecReceipt
	startEntered   chan struct{}
	startRelease   <-chan struct{}
	responseLost   bool
	beforeStart    func(sandbox.DockerOutputlessExecReceipt) error
	claimVerified  atomic.Bool
}

func (r *recordedOutputlessDocker) CreateOutputlessExec(ctx context.Context, handle sandbox.RemoteSandboxHandle, request sandbox.DockerOutputlessExecRequest) (sandbox.DockerOutputlessExecReceipt, error) {
	return r.delegate.CreateOutputlessExec(ctx, handle, request)
}

func (r *recordedOutputlessDocker) StartOutputlessExec(ctx context.Context, receipt sandbox.DockerOutputlessExecReceipt, timeout time.Duration) error {
	r.startCalls.Add(1)
	r.mu.Lock()
	r.startReceipts = append(r.startReceipts, receipt)
	r.mu.Unlock()
	if r.beforeStart != nil {
		if err := r.beforeStart(receipt); err != nil {
			return err
		}
	}
	if err := r.delegate.StartOutputlessExec(ctx, receipt, timeout); err != nil {
		return err
	}
	// This counter models the provider-side effect occurring before a response
	// is lost. The real-Docker test additionally proves the marker side effect.
	r.sideEffects.Add(1)
	if r.startEntered != nil {
		close(r.startEntered)
		<-r.startRelease
	}
	if r.responseLost {
		return errors.New("start response lost after provider side effect")
	}
	return nil
}

func (r *recordedOutputlessDocker) ObserveOutputlessExec(ctx context.Context, receipt sandbox.DockerOutputlessExecReceipt, running bool) (sandbox.DockerOutputlessExecObservation, error) {
	r.mu.Lock()
	r.observeReceipt = append(r.observeReceipt, receipt)
	r.mu.Unlock()
	return r.delegate.ObserveOutputlessExec(ctx, receipt, running)
}

func (r *recordedOutputlessDocker) startedReceipts() []sandbox.DockerOutputlessExecReceipt {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]sandbox.DockerOutputlessExecReceipt(nil), r.startReceipts...)
}

func (r *recordedOutputlessDocker) observedReceipts() []sandbox.DockerOutputlessExecReceipt {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]sandbox.DockerOutputlessExecReceipt(nil), r.observeReceipt...)
}

func TestCraftDockerRestrictedCoordinatorBindFailureNeverStarts(t *testing.T) {
	coordinator, budget, _, tenant, grantID := newCraftDockerCoordinatorFixture(t)
	fake := &fakeOutputlessDocker{afterCreate: func() error {
		return budget.db.Exec("UPDATE agent_runs SET revision = revision + 1 WHERE tenant_id = ? AND run_id = ?", tenant, "run-docker-coordinator").Error
	}}
	service, err := NewCraftDockerRestrictedExec(coordinator, fake, time.Second)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	_, err = service.Start(context.Background(), grantID, "activity-bind-fence", CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}, fakeDockerHandle{}, CraftDockerOutputlessRequest{DiscardOutput: true, Exec: sandbox.RemoteExecRequest{Command: "true"}})
	require.Error(t, err, "stale Run revision makes receipt bind fail")
	require.Equal(t, 1, fake.createCalls)
	require.Zero(t, fake.startCalls, "failed exact receipt bind must never reach physical start")
	var journal CraftChargeStartJournalRow
	require.NoError(t, budget.db.Where("tenant_id = ? AND activity_key = ?", tenant, "activity-bind-fence").Take(&journal).Error)
	assertCoordinatorIntentHeld(t, budget, journal, false, false)
}

func TestCraftDockerRestrictedCoordinatorRestartAfterBindUsesPersistedReceipt(t *testing.T) {
	coordinator, budget, _, tenant, grantID := newCraftDockerCoordinatorFixture(t)
	binding := CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}
	op, err := coordinator.Prepare(context.Background(), grantID, "activity-restart-after-bind", binding)
	require.NoError(t, err)
	fake := &fakeOutputlessDocker{}
	created, err := fake.CreateOutputlessExec(context.Background(), fakeDockerHandle{}, sandbox.DockerOutputlessExecRequest{DiscardOutput: true, Request: sandbox.RemoteExecRequest{Command: "true"}})
	require.NoError(t, err)
	durable := repository.DockerExecReceipt{Provider: "docker", ContainerID: created.ContainerID, ExecID: created.ExecID}
	require.NoError(t, op.Bind(context.Background(), durable))

	restartedBudget, err := NewCraftBudgetService(budget.db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	restartedCoordinator, err := NewCraftDockerSendCoordinator(restartedBudget, repository.NewCraftDockerSendClaimRepository(budget.db))
	require.NoError(t, err)
	recorder := &recordedOutputlessDocker{delegate: fake}
	service, err := NewCraftDockerRestrictedExec(restartedCoordinator, recorder, time.Second)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	result, err := service.ResumeBound(context.Background(), grantID, "activity-restart-after-bind")
	require.NoError(t, err)
	require.True(t, result.Accepted)
	require.Equal(t, durable, result.Receipt)
	require.EqualValues(t, 1, recorder.startCalls.Load())
	require.EqualValues(t, 1, recorder.sideEffects.Load())
	require.Equal(t, []sandbox.DockerOutputlessExecReceipt{{ContainerID: durable.ContainerID, ExecID: durable.ExecID}}, recorder.startedReceipts())
	require.Equal(t, 1, fake.createCalls, "resume after bind must not create a new exec ID")
	var journal CraftChargeStartJournalRow
	require.NoError(t, budget.db.Where("tenant_id = ? AND activity_key = ?", tenant, "activity-restart-after-bind").Take(&journal).Error)
	assertCoordinatorIntentHeld(t, budget, journal, true, true)
}

func TestCraftDockerRestrictedCoordinatorRestartAfterClaimNeverStartsAgain(t *testing.T) {
	coordinator, budget, _, tenant, grantID := newCraftDockerCoordinatorFixture(t)
	binding := CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}
	op, err := coordinator.Prepare(context.Background(), grantID, "activity-restart-after-claim", binding)
	require.NoError(t, err)
	fake := &fakeOutputlessDocker{}
	created, err := fake.CreateOutputlessExec(context.Background(), fakeDockerHandle{}, sandbox.DockerOutputlessExecRequest{DiscardOutput: true, Request: sandbox.RemoteExecRequest{Command: "true"}})
	require.NoError(t, err)
	durable := repository.DockerExecReceipt{Provider: "docker", ContainerID: created.ContainerID, ExecID: created.ExecID}
	require.NoError(t, op.Bind(context.Background(), durable))
	claim, err := op.Claim(context.Background(), durable)
	require.NoError(t, err)
	require.NotNil(t, claim.Permission)
	_, ok := claim.Permission.Consume() // model the process losing its local token after the claim.
	require.True(t, ok)

	restartedBudget, err := NewCraftBudgetService(budget.db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	restartedCoordinator, err := NewCraftDockerSendCoordinator(restartedBudget, repository.NewCraftDockerSendClaimRepository(budget.db))
	require.NoError(t, err)
	recorder := &recordedOutputlessDocker{delegate: fake}
	service, err := NewCraftDockerRestrictedExec(restartedCoordinator, recorder, time.Second)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	_, err = service.ResumeBound(context.Background(), grantID, "activity-restart-after-claim")
	require.Error(t, err, "a claimed receipt is observation-only after restart")
	require.Zero(t, recorder.startCalls.Load())
	require.Zero(t, recorder.sideEffects.Load())
	observation, err := restartedCoordinator.Observe(context.Background(), grantID, "activity-restart-after-claim")
	require.NoError(t, err)
	require.True(t, observation.Claimed)
	require.Equal(t, &durable, observation.Receipt)
	var journal CraftChargeStartJournalRow
	require.NoError(t, budget.db.Where("tenant_id = ? AND activity_key = ?", tenant, "activity-restart-after-claim").Take(&journal).Error)
	assertCoordinatorIntentHeld(t, budget, journal, true, true)
}

func TestCraftDockerRestrictedCoordinatorConcurrentStartResumeAndLostResponseSendOnce(t *testing.T) {
	coordinator, budget, _, tenant, grantID := newCraftDockerCoordinatorFixture(t)
	fake := &fakeOutputlessDocker{}
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseStart := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseStart()
	recorder := &recordedOutputlessDocker{delegate: fake, startEntered: make(chan struct{}), startRelease: release, responseLost: true}
	recorder.beforeStart = func(receipt sandbox.DockerOutputlessExecReceipt) error {
		observation, observeErr := coordinator.Observe(context.Background(), grantID, "activity-racing-replay")
		if observeErr != nil {
			return observeErr
		}
		if !observation.Claimed || observation.Receipt == nil || *observation.Receipt != (repository.DockerExecReceipt{Provider: "docker", ContainerID: receipt.ContainerID, ExecID: receipt.ExecID}) {
			return errors.New("provider callback ran without the durable matching claim")
		}
		recorder.claimVerified.Store(true)
		return nil
	}
	service, err := NewCraftDockerRestrictedExec(coordinator, recorder, time.Second)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	binding := CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}
	request := CraftDockerOutputlessRequest{DiscardOutput: true, Exec: sandbox.RemoteExecRequest{Command: "true"}}
	type startResult struct {
		result CraftDockerOutputlessResult
		err    error
	}
	started := make(chan startResult, 1)
	go func() {
		result, startErr := service.Start(context.Background(), grantID, "activity-racing-replay", binding, fakeDockerHandle{}, request)
		started <- startResult{result: result, err: startErr}
	}()
	select {
	case <-recorder.startEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("physical callback did not enter after durable claim")
	}

	gate := make(chan struct{})
	replayErrors := make(chan error, 2)
	go func() {
		<-gate
		_, replayErr := service.ResumeBound(context.Background(), grantID, "activity-racing-replay")
		replayErrors <- replayErr
	}()
	go func() {
		<-gate
		_, replayErr := service.Start(context.Background(), grantID, "activity-racing-replay", binding, fakeDockerHandle{}, request)
		replayErrors <- replayErr
	}()
	close(gate)
	for range 2 {
		require.Error(t, <-replayErrors, "replay must not receive a second physical start")
	}
	require.EqualValues(t, 1, recorder.startCalls.Load())
	require.EqualValues(t, 1, recorder.sideEffects.Load(), "provider side effect occurs once before response loss")
	require.True(t, recorder.claimVerified.Load(), "physical callback must observe its exact durable claim first")
	releaseStart()
	first := <-started
	require.ErrorIs(t, first.err, sandbox.ErrRemoteOperationUnknown)
	require.False(t, first.result.Accepted)
	_, replayErr := service.ResumeBound(context.Background(), grantID, "activity-racing-replay")
	require.Error(t, replayErr)
	require.EqualValues(t, 1, recorder.startCalls.Load())
	require.EqualValues(t, 1, recorder.sideEffects.Load())
	require.Equal(t, []sandbox.DockerOutputlessExecReceipt{{ContainerID: first.result.Receipt.ContainerID, ExecID: first.result.Receipt.ExecID}}, recorder.startedReceipts())

	observation, err := coordinator.Observe(context.Background(), grantID, "activity-racing-replay")
	require.NoError(t, err)
	require.True(t, observation.Claimed)
	require.Equal(t, &first.result.Receipt, observation.Receipt)
	var journal CraftChargeStartJournalRow
	require.NoError(t, budget.db.Where("tenant_id = ? AND activity_key = ?", tenant, "activity-racing-replay").Take(&journal).Error)
	assertCoordinatorIntentHeld(t, budget, journal, true, true)
}

func TestCraftDockerRestrictedCoordinatorRealDockerResponseLossMarkerOnce(t *testing.T) {
	if os.Getenv("WEKNORA_DOCKER_COORDINATOR_INTEGRATION") != "1" {
		t.Skip("real coordinator-to-Docker proof is opt-in")
	}
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		host = sandbox.DefaultDockerHost
	}
	const image = "wechatopenai/weknora-sandbox:main"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	api, err := client.New(client.WithHost(host))
	require.NoError(t, err)
	defer api.Close()
	server, err := api.ServerVersion(ctx, client.ServerVersionOptions{})
	require.NoError(t, err)
	_, err = api.ImageInspect(ctx, image)
	require.NoError(t, err, "the bounded integration proof requires the pinned local sandbox image")
	t.Logf("Docker host=%s client-api=%s server=%s api=%s image=%s", host, api.ClientVersion(), server.Version, server.APIVersion, image)

	created, err := api.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:     &container.Config{Image: image, Entrypoint: []string{"/bin/sh", "-c"}, Cmd: []string{"exec sleep infinity"}, User: "root"},
		HostConfig: &container.HostConfig{NetworkMode: "none"},
	})
	require.NoError(t, err)
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, cleanupErr := api.ContainerRemove(cleanupCtx, created.ID, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
		t.Logf("cleanup container=%s force_remove=true error=%v", created.ID, cleanupErr)
		require.NoError(t, cleanupErr)
	}()
	_, err = api.ContainerStart(ctx, created.ID, client.ContainerStartOptions{})
	require.NoError(t, err)

	provider, err := sandbox.NewDockerRemoteClientForCheck(&sandbox.Config{DockerImage: image, DockerHost: host, DockerNetworkMode: "none", DockerHTTPTimeout: 5 * time.Second})
	require.NoError(t, err)
	coordinator, _, _, _, grantID := newCraftDockerCoordinatorFixture(t)
	markerPath := "/tmp/weknora-craft-s3-coordinator-" + uuid.NewString()
	recorder := &recordedOutputlessDocker{delegate: provider, responseLost: true}
	recorder.beforeStart = func(receipt sandbox.DockerOutputlessExecReceipt) error {
		observation, observeErr := coordinator.Observe(ctx, grantID, "activity-real-response-loss")
		if observeErr != nil {
			return observeErr
		}
		if !observation.Claimed || observation.Receipt == nil || *observation.Receipt != (repository.DockerExecReceipt{Provider: "docker", ContainerID: receipt.ContainerID, ExecID: receipt.ExecID}) {
			return errors.New("real Docker callback ran without the durable matching claim")
		}
		recorder.claimVerified.Store(true)
		return nil
	}
	service, err := NewCraftDockerRestrictedExec(coordinator, recorder, 3*time.Second)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	request := CraftDockerOutputlessRequest{DiscardOutput: true, Exec: sandbox.RemoteExecRequest{
		Command: "/bin/sh", Args: []string{"-c", fmt.Sprintf("sleep 2; printf 'marker\\n' >> %s", markerPath)}, Timeout: 8 * time.Second,
	}}
	start, err := service.Start(ctx, grantID, "activity-real-response-loss", CraftCallBinding{ModelID: "integration-model", Funding: commercial.FundingPlatform}, integrationDockerHandle{id: created.ID}, request)
	require.ErrorIs(t, err, sandbox.ErrRemoteOperationUnknown, "the physical detached start succeeded before its response was deliberately lost")
	require.Equal(t, created.ID, start.Receipt.ContainerID)
	require.NotEmpty(t, start.Receipt.ExecID)
	require.EqualValues(t, 1, recorder.startCalls.Load())
	require.EqualValues(t, 1, recorder.sideEffects.Load())
	require.True(t, recorder.claimVerified.Load(), "actual ExecStart must be preceded by durable claim on the same receipt")
	_, replayErr := service.ResumeBound(ctx, grantID, "activity-real-response-loss")
	require.Error(t, replayErr, "claimed replay cannot receive another physical start permission")
	require.EqualValues(t, 1, recorder.startCalls.Load())
	observation, err := coordinator.Observe(ctx, grantID, "activity-real-response-loss")
	require.NoError(t, err)
	require.True(t, observation.Claimed)
	require.Equal(t, &start.Receipt, observation.Receipt)

	terminal, err := service.Wait(ctx, grantID, "activity-real-response-loss", 12*time.Second)
	require.NoError(t, err)
	require.Equal(t, start.Receipt, terminal.Receipt)
	require.Equal(t, sandbox.DockerOutputlessSucceeded, terminal.State)
	started := recorder.startedReceipts()
	require.Equal(t, []sandbox.DockerOutputlessExecReceipt{{ContainerID: start.Receipt.ContainerID, ExecID: start.Receipt.ExecID}}, started)
	for _, observed := range recorder.observedReceipts() {
		require.Equal(t, started[0], observed, "every wait inspect stays on the same bound exec and container")
	}
	archive, err := api.CopyFromContainer(ctx, created.ID, client.CopyFromContainerOptions{SourcePath: markerPath})
	require.NoError(t, err)
	defer archive.Content.Close()
	tarReader := tar.NewReader(archive.Content)
	_, err = tarReader.Next()
	require.NoError(t, err)
	content, err := io.ReadAll(tarReader)
	require.NoError(t, err)
	require.Equal(t, "marker\n", string(content), "one physical start produced one marker write")
	logs, err := api.ContainerLogs(ctx, created.ID, client.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Tail: "all"})
	require.NoError(t, err)
	logBytes, err := io.ReadAll(logs)
	require.NoError(t, err)
	require.NoError(t, logs.Close())
	t.Logf("container=%s exec=%s claimed=true claim_verified_at_callback=%t callback_count=%d side_effect_count=%d marker=%q container_log_bytes=%d", created.ID, start.Receipt.ExecID, recorder.claimVerified.Load(), recorder.startCalls.Load(), recorder.sideEffects.Load(), string(content), len(logBytes))
}

type integrationDockerHandle struct{ id string }

func (h integrationDockerHandle) ID() string                     { return h.id }
func (integrationDockerHandle) Provider() sandbox.RemoteProvider { return sandbox.SandboxTypeDocker }
func (integrationDockerHandle) Metadata() map[string]string      { return nil }

// futureSinceEventObserver replays real daemon events but forces the window
// to start after every event of the exec, proving the explicit indeterminate
// contract against the real events API instead of a fabricated stream.
type futureSinceEventObserver struct {
	inner sandbox.DockerExecEventPairObserver
}

func (f futureSinceEventObserver) ObserveExecEventPair(ctx context.Context, receipt sandbox.DockerOutputlessExecReceipt, _ time.Time) (sandbox.DockerExecEventPair, sandbox.DockerExecEventPairStatus, error) {
	return f.inner.ObserveExecEventPair(ctx, receipt, time.Now().Add(time.Hour))
}

func TestCraftDockerRestrictedRealDockerExecEventPairDuration(t *testing.T) {
	if os.Getenv("WEKNORA_DOCKER_COORDINATOR_INTEGRATION") != "1" {
		t.Skip("real coordinator-to-Docker duration proof is opt-in")
	}
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		host = sandbox.DefaultDockerHost
	}
	const image = "wechatopenai/weknora-sandbox:main"
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	api, err := client.New(client.WithHost(host))
	require.NoError(t, err)
	defer api.Close()
	_, err = api.ImageInspect(ctx, image)
	require.NoError(t, err, "the bounded integration proof requires the pinned local sandbox image")
	created, err := api.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:     &container.Config{Image: image, Entrypoint: []string{"/bin/sh", "-c"}, Cmd: []string{"exec sleep infinity"}, User: "root"},
		HostConfig: &container.HostConfig{NetworkMode: "none"},
	})
	require.NoError(t, err)
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, cleanupErr := api.ContainerRemove(cleanupCtx, created.ID, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
		t.Logf("cleanup container=%s force_remove=true error=%v", created.ID, cleanupErr)
		require.NoError(t, cleanupErr)
	}()
	_, err = api.ContainerStart(ctx, created.ID, client.ContainerStartOptions{})
	require.NoError(t, err)

	provider, err := sandbox.NewDockerRemoteClientForCheck(&sandbox.Config{DockerImage: image, DockerHost: host, DockerNetworkMode: "none", DockerHTTPTimeout: 5 * time.Second})
	require.NoError(t, err)
	coordinator, budget, _, _, grantID := newCraftDockerCoordinatorFixture(t)

	captured, err := NewCraftDockerRestrictedExec(coordinator, provider, 3*time.Second)
	require.NoError(t, err)
	captured.WithExecutionPolicy(&permissiveExecPolicyGate{})
	captured.WithExecEventPairObserver(provider)
	request := CraftDockerOutputlessRequest{DiscardOutput: true, Exec: sandbox.RemoteExecRequest{Command: "/bin/sh", Args: []string{"-c", "sleep 1"}, Timeout: 8 * time.Second}}
	start, err := captured.Start(ctx, grantID, "activity-real-duration", CraftCallBinding{ModelID: "integration-model", Funding: commercial.FundingPlatform}, integrationDockerHandle{id: created.ID}, request)
	require.NoError(t, err)
	terminal, err := captured.Wait(ctx, grantID, "activity-real-duration", 15*time.Second)
	require.NoError(t, err)
	require.Equal(t, sandbox.DockerOutputlessSucceeded, terminal.State)
	require.Equal(t, start.Receipt, terminal.Receipt)
	require.True(t, terminal.DurationAvailable, "the real daemon exec_start/exec_die pair must answer the authoritative duration")
	require.Equal(t, "docker_exec_events", terminal.DurationSource)
	require.GreaterOrEqual(t, terminal.Duration.Milliseconds(), int64(900), "the paired sleep-1 command cannot be shorter than its own runtime")
	require.LessOrEqual(t, terminal.Duration.Milliseconds(), int64(10000))
	var pair struct {
		StartedNS  *int64  `gorm:"column:exec_event_started_at_ns"`
		FinishedNS *int64  `gorm:"column:exec_event_finished_at_ns"`
		Source     *string `gorm:"column:duration_source"`
	}
	require.NoError(t, budget.db.Table("craft_charge_start_journal").Select("exec_event_started_at_ns, exec_event_finished_at_ns, duration_source").
		Where("tenant_id = ? AND activity_key = ?", 701, "activity-real-duration").Take(&pair).Error)
	require.NotNil(t, pair.StartedNS)
	require.NotNil(t, pair.FinishedNS)
	require.NotNil(t, pair.Source)
	require.Equal(t, "docker_exec_events", *pair.Source)
	require.Equal(t, terminal.Duration, time.Duration(*pair.FinishedNS-*pair.StartedNS), "the reported duration is exactly the persisted daemon pair")

	// An intentionally unavailable pair (replay window after every event)
	// must keep the terminal result while reporting the indeterminate source.
	empty, err := NewCraftDockerRestrictedExec(coordinator, provider, 3*time.Second)
	require.NoError(t, err)
	empty.WithExecutionPolicy(&permissiveExecPolicyGate{})
	empty.WithExecEventPairObserver(futureSinceEventObserver{inner: provider})
	emptyStart, err := empty.Start(ctx, grantID, "activity-real-duration-empty", CraftCallBinding{ModelID: "integration-model", Funding: commercial.FundingPlatform}, integrationDockerHandle{id: created.ID}, request)
	require.NoError(t, err)
	emptyTerminal, err := empty.Wait(ctx, grantID, "activity-real-duration-empty", 15*time.Second)
	require.NoError(t, err)
	require.Equal(t, sandbox.DockerOutputlessSucceeded, emptyTerminal.State)
	require.Equal(t, emptyStart.Receipt, emptyTerminal.Receipt)
	require.False(t, emptyTerminal.DurationAvailable, "an empty replay window reports the explicit indeterminate contract")
	require.Equal(t, "unavailable", emptyTerminal.DurationSource)
	t.Logf("container=%s captured_exec=%s duration=%s empty_window_exec=%s", created.ID, start.Receipt.ExecID, terminal.Duration, emptyStart.Receipt.ExecID)
}

type fakeExecEventObserver struct {
	calls     int
	pair      sandbox.DockerExecEventPair
	status    sandbox.DockerExecEventPairStatus
	err       error
	sinceSeen []time.Time
}

func (f *fakeExecEventObserver) ObserveExecEventPair(_ context.Context, receipt sandbox.DockerOutputlessExecReceipt, since time.Time) (sandbox.DockerExecEventPair, sandbox.DockerExecEventPairStatus, error) {
	f.calls++
	f.sinceSeen = append(f.sinceSeen, since)
	if f.pair.ExecID == "" {
		f.pair.ExecID = receipt.ExecID
	}
	return f.pair, f.status, f.err
}

func startClaimedOutputlessExec(t *testing.T, service *CraftDockerRestrictedExec, grantID, activity string) repository.DockerExecReceipt {
	t.Helper()
	result, err := service.Start(context.Background(), grantID, activity, CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform}, fakeDockerHandle{}, CraftDockerOutputlessRequest{DiscardOutput: true, Exec: sandbox.RemoteExecRequest{Command: "true"}})
	require.NoError(t, err)
	require.True(t, result.Accepted)
	return result.Receipt
}

func TestCraftDockerRestrictedWaitCapturesExecEventPairDuration(t *testing.T) {
	coordinator, budget, _, _, grantID := newCraftDockerCoordinatorFixture(t)
	docker := &fakeOutputlessDocker{observations: []sandbox.DockerOutputlessExecObservation{
		{State: sandbox.DockerOutputlessSucceeded, ExitCode: ptr(0)},
	}}
	events := &fakeExecEventObserver{status: sandbox.DockerExecEventPairCaptured, pair: sandbox.DockerExecEventPair{StartedNano: 1758796800_000000000, FinishedNano: 1758796801_250000000}}
	service, err := NewCraftDockerRestrictedExec(coordinator, docker, time.Second)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	service.WithExecEventPairObserver(events)
	receipt := startClaimedOutputlessExec(t, service, grantID, "activity-duration-capture")

	terminal, err := service.Wait(context.Background(), grantID, "activity-duration-capture", 2*time.Second)
	require.NoError(t, err)
	require.Equal(t, sandbox.DockerOutputlessSucceeded, terminal.State)
	require.True(t, terminal.DurationAvailable, "captured exec event pair is authoritative duration evidence")
	require.Equal(t, 1250*time.Millisecond, terminal.Duration)
	require.Equal(t, "docker_exec_events", terminal.DurationSource)
	require.Equal(t, 1, events.calls)
	require.Equal(t, receipt.ExecID, events.pair.ExecID)

	// The replay window must start at or before the durable send claim.
	require.NotEmpty(t, events.sinceSeen)
	var claimed struct {
		SendClaimedAt *time.Time `gorm:"column:send_claimed_at"`
	}
	require.NoError(t, budget.db.Table("craft_charge_start_journal").Select("send_claimed_at").
		Where("tenant_id = ? AND activity_key = ?", 701, "activity-duration-capture").Take(&claimed).Error)
	require.NotNil(t, claimed.SendClaimedAt)
	require.False(t, events.sinceSeen[0].After(*claimed.SendClaimedAt), "event replay window starts no later than the durable claim")

	var pair struct {
		StartedNS  *int64  `gorm:"column:exec_event_started_at_ns"`
		FinishedNS *int64  `gorm:"column:exec_event_finished_at_ns"`
		Source     *string `gorm:"column:duration_source"`
	}
	require.NoError(t, budget.db.Table("craft_charge_start_journal").Select("exec_event_started_at_ns, exec_event_finished_at_ns, duration_source").
		Where("tenant_id = ? AND activity_key = ?", 701, "activity-duration-capture").Take(&pair).Error)
	require.NotNil(t, pair.StartedNS)
	require.EqualValues(t, 1758796800_000000000, *pair.StartedNS)
	require.NotNil(t, pair.FinishedNS)
	require.EqualValues(t, 1758796801_250000000, *pair.FinishedNS)
	require.NotNil(t, pair.Source)
	require.Equal(t, "docker_exec_events", *pair.Source)
}

func TestCraftDockerRestrictedWaitDurationSurvivesRestartFromJournal(t *testing.T) {
	coordinator, _, _, _, grantID := newCraftDockerCoordinatorFixture(t)
	docker := &fakeOutputlessDocker{observations: []sandbox.DockerOutputlessExecObservation{
		{State: sandbox.DockerOutputlessSucceeded, ExitCode: ptr(0)},
	}}
	events := &fakeExecEventObserver{status: sandbox.DockerExecEventPairCaptured, pair: sandbox.DockerExecEventPair{StartedNano: 1758796800_000000000, FinishedNano: 1758796800_500000000}}
	service, err := NewCraftDockerRestrictedExec(coordinator, docker, time.Second)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	service.WithExecEventPairObserver(events)
	startClaimedOutputlessExec(t, service, grantID, "activity-duration-restart")
	first, err := service.Wait(context.Background(), grantID, "activity-duration-restart", 2*time.Second)
	require.NoError(t, err)
	require.True(t, first.DurationAvailable)
	require.Equal(t, 1, events.calls)

	// A restarted process without the observer still answers from the journal.
	restartedDocker := &fakeOutputlessDocker{observations: []sandbox.DockerOutputlessExecObservation{
		{State: sandbox.DockerOutputlessSucceeded, ExitCode: ptr(0)},
	}}
	restarted, err := NewCraftDockerRestrictedExec(coordinator, restartedDocker, time.Second)
	require.NoError(t, err)
	restarted.WithExecutionPolicy(&permissiveExecPolicyGate{})
	again, err := restarted.Wait(context.Background(), grantID, "activity-duration-restart", 2*time.Second)
	require.NoError(t, err)
	require.True(t, again.DurationAvailable)
	require.Equal(t, 500*time.Millisecond, again.Duration)
	require.Equal(t, "docker_exec_events", again.DurationSource)
	require.Equal(t, 1, events.calls, "journal evidence must not trigger a second daemon query")
}

func TestCraftDockerRestrictedWaitExplicitlyUnavailableDuration(t *testing.T) {
	base := int64(1758796800_000000000)
	cases := []struct {
		name   string
		events *fakeExecEventObserver
	}{
		{"missing_exec_die", &fakeExecEventObserver{status: sandbox.DockerExecEventPairUnavailable}},
		{"observer_error", &fakeExecEventObserver{status: sandbox.DockerExecEventPairCaptured, err: errors.New("daemon event stream lost")}},
		{"non_monotonic_pair", &fakeExecEventObserver{status: sandbox.DockerExecEventPairCaptured, pair: sandbox.DockerExecEventPair{StartedNano: base + 10, FinishedNano: base}}},
		{"no_observer_attached", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Each subtest owns its fixture: the journal's receipt uniqueness
			// would otherwise collide the fake provider's constant receipt.
			coordinator, _, _, _, grantID := newCraftDockerCoordinatorFixture(t)
			docker := &fakeOutputlessDocker{observations: []sandbox.DockerOutputlessExecObservation{
				{State: sandbox.DockerOutputlessFailed, ExitCode: ptr(3)},
			}}
			service, err := NewCraftDockerRestrictedExec(coordinator, docker, time.Second)
			require.NoError(t, err)
			service.WithExecutionPolicy(&permissiveExecPolicyGate{})
			if tc.events != nil {
				service.WithExecEventPairObserver(tc.events)
			}
			startClaimedOutputlessExec(t, service, grantID, "activity-duration-missing")
			terminal, err := service.Wait(context.Background(), grantID, "activity-duration-missing", 2*time.Second)
			require.NoError(t, err, "missing timing evidence never fails the terminal observation")
			require.Equal(t, sandbox.DockerOutputlessFailed, terminal.State)
			require.False(t, terminal.DurationAvailable, "an absent or ambiguous event pair must stay explicitly unavailable")
			require.Zero(t, terminal.Duration)
			require.Equal(t, "unavailable", terminal.DurationSource)
		})
	}
}

func TestCraftDockerRestrictedWaitConflictingJournalPairRefusesNewEvidence(t *testing.T) {
	coordinator, _, _, _, grantID := newCraftDockerCoordinatorFixture(t)
	docker := &fakeOutputlessDocker{observations: []sandbox.DockerOutputlessExecObservation{
		{State: sandbox.DockerOutputlessSucceeded, ExitCode: ptr(0)},
	}}
	service, err := NewCraftDockerRestrictedExec(coordinator, docker, time.Second)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	receipt := startClaimedOutputlessExec(t, service, grantID, "activity-duration-conflict")
	require.NoError(t, coordinator.RecordExecEventPair(context.Background(), grantID, "activity-duration-conflict", receipt, 1758796800_000000000, 1758796800_250000000))

	events := &fakeExecEventObserver{status: sandbox.DockerExecEventPairCaptured, pair: sandbox.DockerExecEventPair{StartedNano: 1758796800_000000000, FinishedNano: 1758796800_750000000}}
	service.WithExecEventPairObserver(events)
	terminal, err := service.Wait(context.Background(), grantID, "activity-duration-conflict", 2*time.Second)
	require.NoError(t, err)
	require.True(t, terminal.DurationAvailable, "valid durable journal evidence answers without provider I/O")
	require.Equal(t, 250*time.Millisecond, terminal.Duration, "the persisted journal pair wins over any later divergent observation")
	require.Equal(t, "docker_exec_events", terminal.DurationSource)
	require.Zero(t, events.calls, "journal evidence must not trigger a daemon query that could contradict it")
}
