package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/stretchr/testify/require"
)

type blockedCraftDockerOutputStore struct {
	mu            sync.Mutex
	appendEntered chan struct{}
	releaseAppend chan struct{}
	committed     int
	sealed        int
	opened        int
	sealErrors    []error
}

func (s *blockedCraftDockerOutputStore) Open(context.Context, repository.CraftDockerOutputScope) error {
	s.mu.Lock()
	s.opened++
	s.mu.Unlock()
	return nil
}

func (s *blockedCraftDockerOutputStore) Append(ctx context.Context, _ repository.CraftDockerOutputScope, _ string, _ []byte) (int64, error) {
	close(s.appendEntered)
	<-s.releaseAppend
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	s.committed++
	s.mu.Unlock()
	return 1, nil
}

func (s *blockedCraftDockerOutputStore) Seal(context.Context, repository.CraftDockerOutputScope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sealed++
	if len(s.sealErrors) > 0 {
		err := s.sealErrors[0]
		s.sealErrors = s.sealErrors[1:]
		return err
	}
	return nil
}

func (s *blockedCraftDockerOutputStore) ReadAfter(context.Context, repository.CraftDockerOutputScope, int64, int) ([]repository.CraftDockerOutputChunk, repository.CraftDockerOutputSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return nil, repository.CraftDockerOutputSnapshot{Sealed: s.sealed > 0}, nil
}

func TestCraftDockerOutputSealCancelsBlockedAppendBeforeLateCommit(t *testing.T) {
	store := &blockedCraftDockerOutputStore{appendEntered: make(chan struct{}), releaseAppend: make(chan struct{})}
	service, err := NewCraftDockerOutputService(store)
	require.NoError(t, err)
	scope := repository.CraftDockerOutputScope{TenantID: 1, TaskID: "task", RunID: "run", ActivityKey: "activity", ContainerID: "container", ExecID: "exec"}
	receipt := sandbox.DockerNormalExecReceipt{ContainerID: scope.ContainerID, ExecID: scope.ExecID, StdinSHA256: "stdin"}
	sink, err := service.OpenSink(context.Background(), scope, receipt)
	require.NoError(t, err)
	appendDone := make(chan error, 1)
	go func() { appendDone <- sink.Append(context.Background(), receipt, "stdout", []byte("blocked")) }()
	select {
	case <-store.appendEntered:
	case <-time.After(time.Second):
		t.Fatal("append did not enter storage")
	}
	started := time.Now()
	sink.Seal()
	require.Less(t, time.Since(started), 500*time.Millisecond, "Seal must not wait for blocked append work")
	store.mu.Lock()
	require.Equal(t, 1, store.sealed)
	store.mu.Unlock()
	close(store.releaseAppend)
	require.ErrorIs(t, <-appendDone, context.Canceled)
	store.mu.Lock()
	require.Zero(t, store.committed, "late released append must not commit after Seal")
	store.mu.Unlock()
	_, state, err := sink.ReadAfter(context.Background(), 0, 10)
	require.NoError(t, err)
	require.True(t, state.Sealed)
}

func TestCraftDockerOutputReceiptMismatchNeverOpensSink(t *testing.T) {
	store := &blockedCraftDockerOutputStore{appendEntered: make(chan struct{}), releaseAppend: make(chan struct{})}
	service, err := NewCraftDockerOutputService(store)
	require.NoError(t, err)
	scope := repository.CraftDockerOutputScope{TenantID: 1, TaskID: "task", RunID: "run", ActivityKey: "activity", ContainerID: "container", ExecID: "exec"}
	_, err = service.OpenSink(context.Background(), scope, sandbox.DockerNormalExecReceipt{ContainerID: "other", ExecID: scope.ExecID})
	require.ErrorIs(t, err, repository.ErrCraftDockerOutputConflict)
	require.Zero(t, store.opened)
}

func TestCraftDockerOutputSealFailureIsObservableAndRetryable(t *testing.T) {
	sealFailure := errors.New("durable seal timed out")
	store := &blockedCraftDockerOutputStore{appendEntered: make(chan struct{}), releaseAppend: make(chan struct{}), sealErrors: []error{sealFailure, nil}}
	service, err := NewCraftDockerOutputService(store)
	require.NoError(t, err)
	scope := repository.CraftDockerOutputScope{TenantID: 1, TaskID: "task", RunID: "run", ActivityKey: "activity", ContainerID: "container", ExecID: "exec"}
	receipt := sandbox.DockerNormalExecReceipt{ContainerID: scope.ContainerID, ExecID: scope.ExecID, StdinSHA256: "stdin"}
	sink, err := service.OpenSink(context.Background(), scope, receipt)
	require.NoError(t, err)
	require.ErrorIs(t, sink.Seal(), sealFailure, "provider must observe that durable cutoff was not confirmed")
	require.ErrorIs(t, sink.Append(context.Background(), receipt, "stdout", []byte("late")), repository.ErrCraftDockerOutputSealed,
		"a locally closed sink must not admit output while durable seal is uncertain")
	require.NoError(t, sink.Seal(), "a later bounded attempt may confirm durable seal")
	store.mu.Lock()
	require.Equal(t, 2, store.sealed)
	store.mu.Unlock()
}

func TestCraftDockerOutputFailedDurableSealCannotReopenWriter(t *testing.T) {
	coordinator, budget, dockerReceipt, tenantID, grantID := newCraftDockerCoordinatorFixture(t)
	ctx := context.Background()
	activity := "activity-output-seal-failure"
	op, err := coordinator.Prepare(ctx, grantID, activity, CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform})
	require.NoError(t, err)
	require.NoError(t, op.Bind(ctx, dockerReceipt))
	claim, err := op.Claim(ctx, dockerReceipt)
	require.NoError(t, err)
	require.NotNil(t, claim.Permission)
	_, ok := claim.Permission.Consume()
	require.True(t, ok)
	scope := repository.CraftDockerOutputScope{TenantID: tenantID, TaskID: "task-docker-coordinator", RunID: op.journal.RunID,
		ActivityKey: activity, ContainerID: dockerReceipt.ContainerID, ExecID: dockerReceipt.ExecID}
	store := repository.NewCraftDockerOutputRepository(budget.db, 16)
	output, err := NewCraftDockerOutputService(store)
	require.NoError(t, err)
	receipt := sandbox.DockerNormalExecReceipt{ContainerID: dockerReceipt.ContainerID, ExecID: dockerReceipt.ExecID, StdinSHA256: "hash"}
	sink, err := output.OpenSink(ctx, scope, receipt)
	require.NoError(t, err)
	require.NoError(t, sink.Append(ctx, receipt, "stdout", []byte("persisted")))
	require.NoError(t, budget.db.Exec(`CREATE TRIGGER fail_output_seal BEFORE UPDATE OF sealed_at ON craft_docker_output_operations BEGIN SELECT RAISE(ABORT, 'injected seal failure'); END`).Error)
	err = sink.Seal()
	require.ErrorIs(t, err, repository.ErrCraftDockerOutputUnavailable)
	require.ErrorIs(t, sink.Append(ctx, receipt, "stdout", []byte("late")), repository.ErrCraftDockerOutputSealed)
	_, err = output.OpenSink(ctx, scope, receipt)
	require.ErrorIs(t, err, repository.ErrCraftDockerOutputConflict, "a restarted writer cannot reopen after an uncertain seal")
	_, state, err := output.ReadAfter(ctx, scope, 0, 10)
	require.NoError(t, err, "cursor read remains available without reopening a writer")
	require.False(t, state.Sealed, "failed durable seal must not masquerade as a committed cutoff")
	require.True(t, state.Partial || state.NextSequence > 0, "committed output stays observable as unfinished data")
}

func TestCraftDockerOutputSinkPersistsThenReadsCommittedCursor(t *testing.T) {
	coordinator, budget, dockerReceipt, tenantID, grantID := newCraftDockerCoordinatorFixture(t)
	ctx := context.Background()
	activity := "activity-output-store"
	op, err := coordinator.Prepare(ctx, grantID, activity, CraftCallBinding{ModelID: "model", Funding: commercial.FundingPlatform})
	require.NoError(t, err)
	require.NoError(t, op.Bind(ctx, dockerReceipt))
	claim, err := op.Claim(ctx, dockerReceipt)
	require.NoError(t, err)
	require.NotNil(t, claim.Permission)
	_, ok := claim.Permission.Consume()
	require.True(t, ok)

	scope := repository.CraftDockerOutputScope{TenantID: tenantID, TaskID: "task-docker-coordinator", RunID: op.journal.RunID,
		ActivityKey: activity, ContainerID: dockerReceipt.ContainerID, ExecID: dockerReceipt.ExecID}
	store := repository.NewCraftDockerOutputRepository(budget.db, 16)
	output, err := NewCraftDockerOutputService(store)
	require.NoError(t, err)
	receipt := sandbox.DockerNormalExecReceipt{ContainerID: dockerReceipt.ContainerID, ExecID: dockerReceipt.ExecID, StdinSHA256: "hash"}
	sink, err := output.OpenSink(ctx, scope, receipt)
	require.NoError(t, err)
	_, err = output.OpenSink(ctx, scope, receipt)
	require.ErrorIs(t, err, repository.ErrCraftDockerOutputConflict, "an existing receipt cannot mint another writer")
	require.NoError(t, sink.Append(ctx, receipt, "stdout", []byte("hello")))
	require.NoError(t, sink.Append(ctx, receipt, "stderr", []byte("warn")))
	reader, err := NewCraftDockerOutputService(store)
	require.NoError(t, err)
	chunks, state, err := reader.ReadAfter(ctx, scope, 0, 10)
	require.NoError(t, err)
	require.Equal(t, []string{"hello", "warn"}, []string{string(chunks[0].Bytes), string(chunks[1].Bytes)})
	require.EqualValues(t, 2, state.NextSequence)
	require.False(t, state.Sealed)

	sink.Seal()
	require.ErrorIs(t, sink.Append(ctx, receipt, "stdout", []byte("late")), repository.ErrCraftDockerOutputSealed)
}
