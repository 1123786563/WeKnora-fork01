package service

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/commercial"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/google/uuid"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type normalExecTestHandle struct{ id string }

func (h normalExecTestHandle) ID() string                     { return h.id }
func (normalExecTestHandle) Provider() sandbox.RemoteProvider { return sandbox.SandboxTypeDocker }
func (normalExecTestHandle) Metadata() map[string]string      { return nil }

type normalExecTestProvider struct {
	mu            sync.Mutex
	created       int
	started       int
	observed      int
	receipt       sandbox.DockerNormalExecReceipt
	startErr      error
	startResult   sandbox.DockerNormalExecOutcome
	stdin         []byte
	stdinEnabled  bool
	createRequest sandbox.DockerNormalExecRequest
}

type countedNormalExecProvider struct {
	inner    craftDockerNormalProvider
	mu       sync.Mutex
	creates  int
	starts   int
	observes int
}

func (p *countedNormalExecProvider) CreateAttachedExec(ctx context.Context, handle sandbox.RemoteSandboxHandle, request sandbox.DockerNormalExecRequest) (sandbox.DockerNormalExecReceipt, error) {
	p.mu.Lock()
	p.creates++
	p.mu.Unlock()
	return p.inner.CreateAttachedExec(ctx, handle, request)
}

func (p *countedNormalExecProvider) StartAttachedExecOnce(ctx context.Context, receipt sandbox.DockerNormalExecReceipt, stdinEnabled bool, stdin []byte, sink sandbox.DockerNormalExecChunkSink) (sandbox.DockerNormalExecOutcome, error) {
	p.mu.Lock()
	p.starts++
	p.mu.Unlock()
	return p.inner.StartAttachedExecOnce(ctx, receipt, stdinEnabled, stdin, sink)
}

func (p *countedNormalExecProvider) ObserveAttachedExec(ctx context.Context, receipt sandbox.DockerNormalExecReceipt, startEvidence bool) (sandbox.DockerNormalExecObservation, error) {
	p.mu.Lock()
	p.observes++
	p.mu.Unlock()
	return p.inner.ObserveAttachedExec(ctx, receipt, startEvidence)
}

func (p *normalExecTestProvider) CreateAttachedExec(_ context.Context, handle sandbox.RemoteSandboxHandle, request sandbox.DockerNormalExecRequest) (sandbox.DockerNormalExecReceipt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.created++
	p.createRequest = request
	p.receipt.ContainerID = handle.ID()
	p.receipt.ExecID = "exec-normal-service"
	p.receipt.StdinEnabled = request.StdinEnabled
	p.receipt.StdinBytes = int64(len(request.Request.Stdin))
	p.receipt.StdinSHA256 = normalServiceSHA256([]byte(request.Request.Stdin))
	p.receipt.Timeout = request.Request.Timeout
	return p.receipt, nil
}

func (p *normalExecTestProvider) StartAttachedExecOnce(_ context.Context, receipt sandbox.DockerNormalExecReceipt, stdinEnabled bool, stdin []byte, sink sandbox.DockerNormalExecChunkSink) (sandbox.DockerNormalExecOutcome, error) {
	p.mu.Lock()
	p.started++
	p.stdin = append([]byte(nil), stdin...)
	p.stdinEnabled = stdinEnabled
	result, startErr := p.startResult, p.startErr
	p.mu.Unlock()
	if result.Receipt.ExecID == "" {
		result.Receipt = receipt
	}
	if result.StartEvidence && startErr == nil {
		if err := sink.Append(context.Background(), receipt, "stdout", []byte("out")); err != nil {
			return result, err
		}
		if err := sink.Append(context.Background(), receipt, "stderr", []byte("err")); err != nil {
			return result, err
		}
	}
	return result, startErr
}

func (p *normalExecTestProvider) ObserveAttachedExec(_ context.Context, receipt sandbox.DockerNormalExecReceipt, startEvidence bool) (sandbox.DockerNormalExecObservation, error) {
	p.mu.Lock()
	p.observed++
	p.mu.Unlock()
	if startEvidence {
		code := 0
		return sandbox.DockerNormalExecObservation{State: sandbox.DockerNormalExecProcessSucceeded, ExitCode: &code}, nil
	}
	return sandbox.DockerNormalExecObservation{State: sandbox.DockerNormalExecProcessUnknown}, nil
}

type cancelAfterOutputOpen struct {
	inner  craftDockerOutputPersistence
	cancel context.CancelFunc
}

func (s cancelAfterOutputOpen) Open(ctx context.Context, scope repository.CraftDockerOutputScope) error {
	if err := s.inner.Open(ctx, scope); err != nil {
		return err
	}
	s.cancel()
	return nil
}

type failingNormalOutputStore struct {
	inner   craftDockerOutputPersistence
	openErr error
	sealErr error
}

func (s failingNormalOutputStore) Open(ctx context.Context, scope repository.CraftDockerOutputScope) error {
	if s.openErr != nil {
		return s.openErr
	}
	return s.inner.Open(ctx, scope)
}
func (s failingNormalOutputStore) Append(ctx context.Context, scope repository.CraftDockerOutputScope, stream string, chunk []byte) (int64, error) {
	return s.inner.Append(ctx, scope, stream, chunk)
}
func (s failingNormalOutputStore) Seal(ctx context.Context, scope repository.CraftDockerOutputScope) error {
	if s.sealErr != nil {
		return s.sealErr
	}
	return s.inner.Seal(ctx, scope)
}
func (s failingNormalOutputStore) ReadAfter(ctx context.Context, scope repository.CraftDockerOutputScope, cursor int64, limit int) ([]repository.CraftDockerOutputChunk, repository.CraftDockerOutputSnapshot, error) {
	return s.inner.ReadAfter(ctx, scope, cursor, limit)
}
func (s cancelAfterOutputOpen) Append(ctx context.Context, scope repository.CraftDockerOutputScope, stream string, chunk []byte) (int64, error) {
	return s.inner.Append(ctx, scope, stream, chunk)
}
func (s cancelAfterOutputOpen) Seal(ctx context.Context, scope repository.CraftDockerOutputScope) error {
	return s.inner.Seal(ctx, scope)
}
func (s cancelAfterOutputOpen) ReadAfter(ctx context.Context, scope repository.CraftDockerOutputScope, cursor int64, limit int) ([]repository.CraftDockerOutputChunk, repository.CraftDockerOutputSnapshot, error) {
	return s.inner.ReadAfter(ctx, scope, cursor, limit)
}

func TestCraftDockerNormalExecComposesOneClaimedAttachAndDurableOutput(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	coordinator, budget, grantID := seedCraftDockerNormalCoordinator(t, openCraftBudgetTestDB(t), 8701, "run-normal-exec", "task-normal-exec")
	request := craftDockerNormalCoordinatorRequest(8701, "task-normal-exec", "run-normal-exec", "activity-normal-exec")
	provider := &normalExecTestProvider{startResult: sandbox.DockerNormalExecOutcome{
		Transport:     sandbox.DockerNormalExecTransportComplete,
		Observation:   sandbox.DockerNormalExecObservation{State: sandbox.DockerNormalExecProcessSucceeded},
		StartEvidence: true, OutputBytes: 6,
	}}
	outputRepo := repository.NewCraftDockerOutputRepository(budget.db, 4096)
	output, err := NewCraftDockerOutputService(outputRepo)
	require.NoError(t, err)
	inputs := repository.NewCraftDockerNormalInputRepository(budget.db)
	service, err := NewCraftDockerNormalExecService(coordinator, inputs, provider, output)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})

	result, err := service.Execute(context.Background(), grantID, request.ActivityKey,
		CraftCallBinding{ModelID: "model-normal", Funding: commercial.FundingPlatform}, normalExecTestHandle{id: "container-normal-service"}, request)
	require.NoError(t, err)
	require.Equal(t, "exec-normal-service", result.Receipt.ExecID)
	require.Equal(t, sandbox.DockerNormalExecProcessSucceeded, result.Process.State)
	require.Equal(t, sandbox.DockerNormalExecTransportComplete, result.Transport)
	require.True(t, result.StartEvidence)
	require.True(t, result.OutputComplete)
	require.False(t, result.Output.Partial)
	require.True(t, result.Output.Sealed)
	require.EqualValues(t, 2, result.Cursor)
	require.Equal(t, []byte("normal input"), provider.stdin)
	require.True(t, provider.createRequest.StdinEnabled)
	require.True(t, provider.stdinEnabled)
	require.Equal(t, 1, provider.created)
	require.Equal(t, 1, provider.started)

	chunks, state, err := service.ReadAfter(context.Background(), grantID, request.ActivityKey, request, 0, 10)
	require.NoError(t, err)
	require.Equal(t, []string{"stdout", "stderr"}, []string{chunks[0].Stream, chunks[1].Stream})
	require.Equal(t, []byte("out"), chunks[0].Bytes)
	require.True(t, state.Sealed)
}

func TestCraftDockerNormalExecProjectionOnSQLiteAndIsolatedPostgres(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	forEachCraftDockerCoordinatorNormalDB(t, func(t *testing.T, db *gorm.DB) {
		const tenant uint64 = 8711
		const runID, taskID, activity = "run-normal-db-projection", "task-normal-db-projection", "activity-normal-db-projection"
		coordinator, budget, grantID := seedCraftDockerNormalCoordinator(t, db, tenant, runID, taskID)
		request := craftDockerNormalCoordinatorRequest(tenant, taskID, runID, activity)
		provider := &normalExecTestProvider{startResult: sandbox.DockerNormalExecOutcome{
			Transport:   sandbox.DockerNormalExecTransportComplete,
			Observation: sandbox.DockerNormalExecObservation{State: sandbox.DockerNormalExecProcessSucceeded}, StartEvidence: true,
		}}
		output, err := NewCraftDockerOutputService(repository.NewCraftDockerOutputRepository(budget.db, 4096))
		require.NoError(t, err)
		executor, err := NewCraftDockerNormalExecService(coordinator, repository.NewCraftDockerNormalInputRepository(budget.db), provider, output)
		require.NoError(t, err)
		executor.WithExecutionPolicy(&permissiveExecPolicyGate{})
		result, err := executor.Execute(context.Background(), grantID, activity,
			CraftCallBinding{ModelID: "model-normal-db", Funding: commercial.FundingPlatform}, normalExecTestHandle{id: "container-normal-service"}, request)
		require.NoError(t, err)
		require.True(t, result.OutputComplete)
		require.EqualValues(t, 2, result.Cursor)
		chunks, snapshot, err := executor.ReadAfter(context.Background(), grantID, activity, request, 0, 10)
		require.NoError(t, err)
		require.Equal(t, 2, len(chunks))
		require.True(t, snapshot.Sealed)
	})
}

func TestCraftDockerNormalExecPreservesEnabledEmptyStdinIdentity(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	coordinator, budget, grantID := seedCraftDockerNormalCoordinator(t, openCraftBudgetTestDB(t), 8707, "run-normal-empty-stdin", "task-normal-empty-stdin")
	request := craftDockerNormalCoordinatorRequest(8707, "task-normal-empty-stdin", "run-normal-empty-stdin", "activity-normal-empty-stdin")
	request.StdinEnabled = true
	request.Stdin = []byte{}
	provider := &normalExecTestProvider{startResult: sandbox.DockerNormalExecOutcome{
		Transport:   sandbox.DockerNormalExecTransportComplete,
		Observation: sandbox.DockerNormalExecObservation{State: sandbox.DockerNormalExecProcessSucceeded}, StartEvidence: true,
	}}
	output, err := NewCraftDockerOutputService(repository.NewCraftDockerOutputRepository(budget.db, 4096))
	require.NoError(t, err)
	service, err := NewCraftDockerNormalExecService(coordinator, repository.NewCraftDockerNormalInputRepository(budget.db), provider, output)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	result, err := service.Execute(context.Background(), grantID, request.ActivityKey,
		CraftCallBinding{ModelID: "model-normal", Funding: commercial.FundingPlatform}, normalExecTestHandle{id: "container-normal-service"}, request)
	require.NoError(t, err)
	require.True(t, result.Receipt.StdinEnabled)
	require.Zero(t, result.Receipt.StdinByteCount)
	require.True(t, provider.createRequest.StdinEnabled, "inert create must preserve enabled-empty stdin")
	require.True(t, provider.stdinEnabled, "claimed start must preserve enabled-empty stdin")
	require.Equal(t, "exec-normal-service", result.Receipt.ExecID)
}

func TestCraftDockerNormalExecResumesExactBoundReceiptWithoutCreate(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	coordinator, budget, grantID := seedCraftDockerNormalCoordinator(t, openCraftBudgetTestDB(t), 8708, "run-normal-bound-resume", "task-normal-bound-resume")
	request := craftDockerNormalCoordinatorRequest(8708, "task-normal-bound-resume", "run-normal-bound-resume", "activity-normal-bound-resume")
	binding := CraftCallBinding{ModelID: "model-normal", Funding: commercial.FundingPlatform}
	operation, staged, err := coordinator.PrepareNormal(context.Background(), grantID, request.ActivityKey, binding, request)
	require.NoError(t, err)
	storedReceipt := repository.CraftDockerNormalReceipt{
		DockerExecReceipt: repository.DockerExecReceipt{Provider: "docker", ContainerID: "container-normal-service", ExecID: "exec-bound-before-resume"},
		StdinEnabled:      staged.StdinEnabled, StdinByteCount: staged.StdinByteCount, StdinSHA256: staged.StdinSHA256, TimeoutMillis: staged.Request.TimeoutMillis,
	}
	require.NoError(t, operation.Bind(context.Background(), storedReceipt))
	provider := &normalExecTestProvider{startResult: sandbox.DockerNormalExecOutcome{
		Transport:   sandbox.DockerNormalExecTransportComplete,
		Observation: sandbox.DockerNormalExecObservation{State: sandbox.DockerNormalExecProcessSucceeded}, StartEvidence: true,
	}}
	output, err := NewCraftDockerOutputService(repository.NewCraftDockerOutputRepository(budget.db, 4096))
	require.NoError(t, err)
	service, err := NewCraftDockerNormalExecService(coordinator, repository.NewCraftDockerNormalInputRepository(budget.db), provider, output)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	result, err := service.Execute(context.Background(), grantID, request.ActivityKey, binding, normalExecTestHandle{id: "container-normal-service"}, request)
	require.NoError(t, err)
	require.Equal(t, storedReceipt, result.Receipt)
	require.Zero(t, provider.created, "a persisted bound receipt resumes without another ExecCreate")
	require.Equal(t, 1, provider.started)
}

func TestCraftDockerNormalExecAttachResponseLossIsUnknownAndNeverResent(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	coordinator, budget, grantID := seedCraftDockerNormalCoordinator(t, openCraftBudgetTestDB(t), 8709, "run-normal-response-loss", "task-normal-response-loss")
	request := craftDockerNormalCoordinatorRequest(8709, "task-normal-response-loss", "run-normal-response-loss", "activity-normal-response-loss")
	provider := &normalExecTestProvider{
		startErr: errors.New("ExecAttach response lost"),
		startResult: sandbox.DockerNormalExecOutcome{Transport: sandbox.DockerNormalExecTransportComplete,
			Observation: sandbox.DockerNormalExecObservation{State: sandbox.DockerNormalExecProcessSucceeded}, StartEvidence: true},
	}
	output, err := NewCraftDockerOutputService(repository.NewCraftDockerOutputRepository(budget.db, 4096))
	require.NoError(t, err)
	service, err := NewCraftDockerNormalExecService(coordinator, repository.NewCraftDockerNormalInputRepository(budget.db), provider, output)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	binding := CraftCallBinding{ModelID: "model-normal", Funding: commercial.FundingPlatform}
	handle := normalExecTestHandle{id: "container-normal-service"}
	first, err := service.Execute(context.Background(), grantID, request.ActivityKey, binding, handle, request)
	require.Error(t, err)
	require.True(t, sandbox.IsRemoteOperationUnknown(err))
	require.Equal(t, "exec-normal-service", first.Receipt.ExecID)
	require.False(t, first.OutputComplete)
	require.True(t, first.Output.Partial)
	require.Equal(t, sandbox.DockerNormalExecTransportPartial, first.Transport)
	require.True(t, first.StartEvidence)
	require.Equal(t, 1, provider.started)

	_, err = service.Execute(context.Background(), grantID, request.ActivityKey, binding, handle, request)
	require.Error(t, err)
	require.True(t, sandbox.IsRemoteOperationUnknown(err))
	require.Equal(t, 1, provider.started, "an ambiguous attach response cannot authorize a second attach")
	require.Equal(t, 1, provider.created)
}

func TestCraftDockerNormalExecDisposableNoEgressMarkerOnce(t *testing.T) {
	if testing.Short() || os.Getenv("CRAFT_T19_NORMAL_OUTPUT_DOCKER_TEST") != "1" {
		t.Skip("set CRAFT_T19_NORMAL_OUTPUT_DOCKER_TEST=1 for disposable no-egress Docker proof")
	}
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	const imageID = "sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11"
	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		dockerHost = client.DefaultDockerHost
	}
	api, err := client.New(client.WithHost(dockerHost))
	require.NoError(t, err)
	defer api.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	image, err := api.ImageInspect(ctx, imageID)
	require.NoError(t, err, "the pinned image must already be local; this test never pulls images")
	require.Equal(t, imageID, image.ID)
	require.Equal(t, "linux", image.Os)
	require.Equal(t, "arm64", image.Architecture)
	created, err := api.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:     &container.Config{Image: imageID, Entrypoint: []string{"/bin/sh", "-c"}, Cmd: []string{"exec sleep infinity"}, User: "root", Tty: false},
		HostConfig: &container.HostConfig{NetworkMode: "none", AutoRemove: false},
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	t.Logf("disposable normal-exec container=%s image=%s network=none", created.ID, imageID)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, removeErr := api.ContainerRemove(cleanupCtx, created.ID, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
		if removeErr != nil {
			t.Errorf("remove disposable normal-exec container %s: %v", created.ID, removeErr)
			return
		}
		if _, inspectErr := api.ContainerInspect(cleanupCtx, created.ID, client.ContainerInspectOptions{}); inspectErr == nil {
			t.Errorf("disposable normal-exec container %s remains after cleanup", created.ID)
		} else {
			t.Logf("disposable normal-exec container cleanup verified absent: %s", created.ID)
		}
	})
	_, err = api.ContainerStart(ctx, created.ID, client.ContainerStartOptions{})
	require.NoError(t, err)
	inspection, err := api.ContainerInspect(ctx, created.ID, client.ContainerInspectOptions{})
	require.NoError(t, err)
	require.Equal(t, "none", string(inspection.Container.HostConfig.NetworkMode), "exec target must have no network")

	coordinator, budget, grantID := seedCraftDockerNormalCoordinator(t, openCraftBudgetTestDB(t), 8710, "run-normal-physical", "task-normal-physical")
	output, err := NewCraftDockerOutputService(repository.NewCraftDockerOutputRepository(budget.db, 4096))
	require.NoError(t, err)
	docker, err := sandbox.NewDockerNormalExecClient(api, sandbox.DockerNormalExecConfig{RPCTimeout: 5 * time.Second, MaxInputBytes: 1024, MaxOutputBytes: 4096})
	require.NoError(t, err)
	counted := &countedNormalExecProvider{inner: docker}
	service, err := NewCraftDockerNormalExecService(coordinator, repository.NewCraftDockerNormalInputRepository(budget.db), counted, output)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	markerPath := "/tmp/craft-t19-normal-once-" + uuid.NewString()
	script := fmt.Sprintf("set -eu; test ! -e '%s'; printf x > '%s'; test \"$(wc -c < '%s')\" -eq 1; printf marker-once", markerPath, markerPath, markerPath)
	request := repository.CraftDockerNormalInputRequest{
		TenantID: 8710, TaskID: "task-normal-physical", RunID: "run-normal-physical", ActivityKey: "activity-normal-physical",
		Command: []string{"/bin/sh", "-c", script}, TimeoutMillis: 10000, OutputLimit: 4096, OutputPolicy: "bounded-partial-v1",
	}
	binding := CraftCallBinding{ModelID: "model-normal-physical", Funding: commercial.FundingPlatform}
	handle := normalExecTestHandle{id: created.ID}
	first, err := service.Execute(ctx, grantID, request.ActivityKey, binding, handle, request)
	require.NoError(t, err)
	require.True(t, first.OutputComplete)
	require.Equal(t, sandbox.DockerNormalExecProcessSucceeded, first.Process.State)
	chunks, _, err := service.ReadAfter(ctx, grantID, request.ActivityKey, request, 0, 10)
	require.NoError(t, err)
	var outputBytes []byte
	for _, chunk := range chunks {
		if chunk.Stream == "stdout" {
			outputBytes = append(outputBytes, chunk.Bytes...)
		}
	}
	require.Equal(t, "marker-once", string(outputBytes))

	_, replayErr := service.Execute(ctx, grantID, request.ActivityKey, binding, handle, request)
	require.ErrorIs(t, replayErr, sandbox.ErrRemoteOperationUnknown, "claimed replay observes the exact exec but cannot attach again")
	counted.mu.Lock()
	require.Equal(t, 1, counted.creates)
	require.Equal(t, 1, counted.starts)
	require.Equal(t, 1, counted.observes)
	counted.mu.Unlock()
	archive, err := api.CopyFromContainer(ctx, created.ID, client.CopyFromContainerOptions{SourcePath: markerPath})
	require.NoError(t, err)
	defer archive.Content.Close()
	reader := tar.NewReader(archive.Content)
	_, err = reader.Next()
	require.NoError(t, err)
	marker, err := io.ReadAll(io.LimitReader(reader, 128))
	require.NoError(t, err)
	require.Equal(t, "x", string(marker), "the single claimed command wrote its marker exactly once")
	t.Logf("normal-exec physical proof container=%s exec=%s cursor=%d marker=%q", created.ID, first.Receipt.ExecID, first.Cursor, string(marker))
}

func TestCraftDockerNormalExecClaimedReplayIsObservationOnlyAndInputBound(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	coordinator, budget, grantID := seedCraftDockerNormalCoordinator(t, openCraftBudgetTestDB(t), 8702, "run-normal-replay", "task-normal-replay")
	request := craftDockerNormalCoordinatorRequest(8702, "task-normal-replay", "run-normal-replay", "activity-normal-replay")
	provider := &normalExecTestProvider{startResult: sandbox.DockerNormalExecOutcome{
		Transport:     sandbox.DockerNormalExecTransportPartial,
		Observation:   sandbox.DockerNormalExecObservation{State: sandbox.DockerNormalExecProcessUnknown},
		StartEvidence: true,
	}}
	output, err := NewCraftDockerOutputService(repository.NewCraftDockerOutputRepository(budget.db, 4096))
	require.NoError(t, err)
	service, err := NewCraftDockerNormalExecService(coordinator, repository.NewCraftDockerNormalInputRepository(budget.db), provider, output)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	binding := CraftCallBinding{ModelID: "model-normal", Funding: commercial.FundingPlatform}
	handle := normalExecTestHandle{id: "container-normal-service"}
	_, err = service.Execute(context.Background(), grantID, request.ActivityKey, binding, handle, request)
	require.Error(t, err)
	require.True(t, sandbox.IsRemoteOperationUnknown(err))

	changed := request
	changed.Stdin = []byte("different input")
	_, err = service.Execute(context.Background(), grantID, request.ActivityKey, binding, handle, changed)
	require.Error(t, err, "claimed replay must compare the original staged request before observation")
	require.Equal(t, 1, provider.created)
	require.Equal(t, 1, provider.started)
	require.Zero(t, provider.observed, "mismatched replay must not inspect another process request")

	result, err := service.Execute(context.Background(), grantID, request.ActivityKey, binding, handle, request)
	require.Error(t, err)
	require.True(t, sandbox.IsRemoteOperationUnknown(err))
	require.Equal(t, sandbox.DockerNormalExecProcessSucceeded, result.Process.State, "same-process start evidence may safely classify terminal inspect")
	require.False(t, result.OutputComplete)
	require.Equal(t, 1, provider.created)
	require.Equal(t, 1, provider.started, "claimed replay must never attach again")
	require.Equal(t, 1, provider.observed)

	restarted, err := NewCraftDockerNormalExecService(coordinator, repository.NewCraftDockerNormalInputRepository(budget.db), provider, output)
	require.NoError(t, err)
	restarted.WithExecutionPolicy(&permissiveExecPolicyGate{})
	restartedResult, err := restarted.Execute(context.Background(), grantID, request.ActivityKey, binding, handle, request)
	require.Error(t, err)
	require.True(t, sandbox.IsRemoteOperationUnknown(err))
	require.Equal(t, sandbox.DockerNormalExecProcessUnknown, restartedResult.Process.State, "restart loses volatile positive-start evidence")
	require.Equal(t, 1, provider.started, "process restart remains observation-only")
	require.Equal(t, 2, provider.observed)
}

func TestCraftDockerNormalExecCancellationAfterClaimLeavesUnknownWithoutAttach(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	coordinator, budget, grantID := seedCraftDockerNormalCoordinator(t, openCraftBudgetTestDB(t), 8703, "run-normal-cancel", "task-normal-cancel")
	request := craftDockerNormalCoordinatorRequest(8703, "task-normal-cancel", "run-normal-cancel", "activity-normal-cancel")
	provider := &normalExecTestProvider{}
	outputRepo := repository.NewCraftDockerOutputRepository(budget.db, 4096)
	ctx, cancel := context.WithCancel(context.Background())
	output, err := NewCraftDockerOutputService(cancelAfterOutputOpen{inner: outputRepo, cancel: cancel})
	require.NoError(t, err)
	service, err := NewCraftDockerNormalExecService(coordinator, repository.NewCraftDockerNormalInputRepository(budget.db), provider, output)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	defer cancel()
	_, err = service.Execute(ctx, grantID, request.ActivityKey,
		CraftCallBinding{ModelID: "model-normal", Funding: commercial.FundingPlatform}, normalExecTestHandle{id: "container-normal-service"}, request)
	require.Error(t, err)
	require.Zero(t, provider.started)
}

func TestCraftDockerNormalExecSinkOpenFailureCannotAttach(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	coordinator, budget, grantID := seedCraftDockerNormalCoordinator(t, openCraftBudgetTestDB(t), 8704, "run-normal-open-fail", "task-normal-open-fail")
	request := craftDockerNormalCoordinatorRequest(8704, "task-normal-open-fail", "run-normal-open-fail", "activity-normal-open-fail")
	provider := &normalExecTestProvider{}
	openErr := errors.New("output writer unavailable")
	output, err := NewCraftDockerOutputService(failingNormalOutputStore{inner: repository.NewCraftDockerOutputRepository(budget.db, 4096), openErr: openErr})
	require.NoError(t, err)
	service, err := NewCraftDockerNormalExecService(coordinator, repository.NewCraftDockerNormalInputRepository(budget.db), provider, output)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	result, err := service.Execute(context.Background(), grantID, request.ActivityKey,
		CraftCallBinding{ModelID: "model-normal", Funding: commercial.FundingPlatform}, normalExecTestHandle{id: "container-normal-service"}, request)
	require.Error(t, err)
	require.True(t, sandbox.IsRemoteOperationUnknown(err))
	require.Equal(t, "exec-normal-service", result.Receipt.ExecID)
	require.Zero(t, provider.started)
}

func TestCraftDockerNormalExecOutputSealFailureCannotReportComplete(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	coordinator, budget, grantID := seedCraftDockerNormalCoordinator(t, openCraftBudgetTestDB(t), 8705, "run-normal-seal-fail", "task-normal-seal-fail")
	request := craftDockerNormalCoordinatorRequest(8705, "task-normal-seal-fail", "run-normal-seal-fail", "activity-normal-seal-fail")
	provider := &normalExecTestProvider{startResult: sandbox.DockerNormalExecOutcome{
		Transport:   sandbox.DockerNormalExecTransportComplete,
		Observation: sandbox.DockerNormalExecObservation{State: sandbox.DockerNormalExecProcessSucceeded}, StartEvidence: true,
	}}
	sealErr := errors.New("durable seal lost")
	output, err := NewCraftDockerOutputService(failingNormalOutputStore{inner: repository.NewCraftDockerOutputRepository(budget.db, 4096), sealErr: sealErr})
	require.NoError(t, err)
	service, err := NewCraftDockerNormalExecService(coordinator, repository.NewCraftDockerNormalInputRepository(budget.db), provider, output)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	result, err := service.Execute(context.Background(), grantID, request.ActivityKey,
		CraftCallBinding{ModelID: "model-normal", Funding: commercial.FundingPlatform}, normalExecTestHandle{id: "container-normal-service"}, request)
	require.Error(t, err)
	require.True(t, sandbox.IsRemoteOperationUnknown(err))
	require.False(t, result.OutputComplete)
	require.True(t, result.Output.Partial)
	require.False(t, result.Output.Sealed)
	require.Equal(t, 1, provider.started)
}

func TestCraftDockerNormalExecRequestOutputQuotaMarksOutputPartial(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	coordinator, budget, grantID := seedCraftDockerNormalCoordinator(t, openCraftBudgetTestDB(t), 8706, "run-normal-quota", "task-normal-quota")
	request := craftDockerNormalCoordinatorRequest(8706, "task-normal-quota", "run-normal-quota", "activity-normal-quota")
	request.OutputLimit = 4
	provider := &normalExecTestProvider{startResult: sandbox.DockerNormalExecOutcome{
		Transport:   sandbox.DockerNormalExecTransportComplete,
		Observation: sandbox.DockerNormalExecObservation{State: sandbox.DockerNormalExecProcessSucceeded}, StartEvidence: true,
	}}
	output, err := NewCraftDockerOutputService(repository.NewCraftDockerOutputRepository(budget.db, 4096))
	require.NoError(t, err)
	service, err := NewCraftDockerNormalExecService(coordinator, repository.NewCraftDockerNormalInputRepository(budget.db), provider, output)
	require.NoError(t, err)
	service.WithExecutionPolicy(&permissiveExecPolicyGate{})
	result, err := service.Execute(context.Background(), grantID, request.ActivityKey,
		CraftCallBinding{ModelID: "model-normal", Funding: commercial.FundingPlatform}, normalExecTestHandle{id: "container-normal-service"}, request)
	require.Error(t, err)
	require.True(t, sandbox.IsRemoteOperationUnknown(err))
	require.True(t, result.Output.Truncated)
	require.True(t, result.Output.Partial)
	require.False(t, result.OutputComplete)
	require.EqualValues(t, 4, result.Output.TotalBytes)
}

func normalServiceSHA256(b []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(b))
}
