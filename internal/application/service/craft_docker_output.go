package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/sandbox"
)

const craftDockerOutputSealTimeout = 2 * time.Second

type craftDockerOutputPersistence interface {
	Open(context.Context, repository.CraftDockerOutputScope) error
	Append(context.Context, repository.CraftDockerOutputScope, string, []byte) (int64, error)
	Seal(context.Context, repository.CraftDockerOutputScope) error
	ReadAfter(context.Context, repository.CraftDockerOutputScope, int64, int) ([]repository.CraftDockerOutputChunk, repository.CraftDockerOutputSnapshot, error)
}

// CraftDockerOutputService creates operation-bound durable sinks and exposes
// their committed output through an independent cursor reader.
type CraftDockerOutputService struct{ store craftDockerOutputPersistence }

func NewCraftDockerOutputService(store craftDockerOutputPersistence) (*CraftDockerOutputService, error) {
	if store == nil {
		return nil, errors.New("Docker output persistence is required")
	}
	return &CraftDockerOutputService{store: store}, nil
}

func (s *CraftDockerOutputService) OpenSink(ctx context.Context, scope repository.CraftDockerOutputScope, receipt sandbox.DockerNormalExecReceipt) (*CraftDockerOutputSink, error) {
	if s == nil || s.store == nil || receipt.ContainerID != scope.ContainerID || receipt.ExecID != scope.ExecID {
		return nil, repository.ErrCraftDockerOutputConflict
	}
	if err := s.store.Open(ctx, scope); err != nil {
		return nil, err
	}
	return &CraftDockerOutputSink{store: s.store, scope: scope, receipt: receipt, active: make(map[uint64]context.CancelFunc)}, nil
}

// CraftDockerOutputSink implements the provider's narrow durable append/seal
// interface. It contains no user callback; consumers poll ReadAfter separately.
type CraftDockerOutputSink struct {
	store   craftDockerOutputPersistence
	scope   repository.CraftDockerOutputScope
	receipt sandbox.DockerNormalExecReceipt

	mu            sync.Mutex
	sealMu        sync.Mutex
	sealed        bool
	durableSealed bool
	sealErr       error
	nextID        uint64
	active        map[uint64]context.CancelFunc
}

var _ sandbox.DockerNormalExecChunkSink = (*CraftDockerOutputSink)(nil)

func (s *CraftDockerOutputSink) Append(ctx context.Context, receipt sandbox.DockerNormalExecReceipt, stream string, chunk []byte) error {
	if s == nil || s.store == nil || receipt != s.receipt {
		return repository.ErrCraftDockerOutputConflict
	}
	s.mu.Lock()
	if s.sealed {
		s.mu.Unlock()
		return repository.ErrCraftDockerOutputSealed
	}
	s.nextID++
	id := s.nextID
	appendCtx, cancel := context.WithCancel(ctx)
	s.active[id] = cancel
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		delete(s.active, id)
		s.mu.Unlock()
	}()
	_, err := s.store.Append(appendCtx, s.scope, stream, chunk)
	if contextErr := appendCtx.Err(); contextErr != nil {
		return contextErr
	}
	return err
}

// Seal closes local admission first and cancels active DB appends. It reports
// whether the durable cutoff committed; after an error a later call retries
// that marker while local writes stay denied. The repository serializes its
// marker with append commits and honors the fixed database timeout.
func (s *CraftDockerOutputSink) Seal() error {
	if s == nil || s.store == nil {
		return repository.ErrCraftDockerOutputUnavailable
	}
	s.sealMu.Lock()
	defer s.sealMu.Unlock()
	s.mu.Lock()
	s.sealed = true
	if s.durableSealed {
		s.mu.Unlock()
		return nil
	}
	cancels := make([]context.CancelFunc, 0, len(s.active))
	for _, cancel := range s.active {
		cancels = append(cancels, cancel)
	}
	s.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}

	ctx, cancel := context.WithTimeout(context.Background(), craftDockerOutputSealTimeout)
	defer cancel()
	err := s.store.Seal(ctx, s.scope)
	s.mu.Lock()
	s.sealErr = err
	s.durableSealed = err == nil
	s.mu.Unlock()
	return err
}

func (s *CraftDockerOutputSink) ReadAfter(ctx context.Context, cursor int64, limit int) ([]repository.CraftDockerOutputChunk, repository.CraftDockerOutputSnapshot, error) {
	if s == nil || s.store == nil {
		return nil, repository.CraftDockerOutputSnapshot{Unavailable: true}, repository.ErrCraftDockerOutputUnavailable
	}
	chunks, state, err := s.store.ReadAfter(ctx, s.scope, cursor, limit)
	s.mu.Lock()
	sealErr := s.sealErr
	s.mu.Unlock()
	if err != nil {
		state.Unavailable = true
		return nil, state, err
	}
	if sealErr != nil {
		state.Unavailable = true
		return chunks, state, fmt.Errorf("%w: seal state could not be persisted", repository.ErrCraftDockerOutputUnavailable)
	}
	return chunks, state, nil
}

// ReadAfter exposes committed cursor reads independently of obtaining a writer
// sink. A process restart can read an existing operation without reopening it.
func (s *CraftDockerOutputService) ReadAfter(ctx context.Context, scope repository.CraftDockerOutputScope, cursor int64, limit int) ([]repository.CraftDockerOutputChunk, repository.CraftDockerOutputSnapshot, error) {
	if s == nil || s.store == nil {
		return nil, repository.CraftDockerOutputSnapshot{Unavailable: true}, repository.ErrCraftDockerOutputUnavailable
	}
	chunks, state, err := s.store.ReadAfter(ctx, scope, cursor, limit)
	if err != nil {
		state.Unavailable = true
		return nil, state, err
	}
	return chunks, state, nil
}
