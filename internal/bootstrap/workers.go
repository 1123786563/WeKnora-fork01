package bootstrap

import (
	"context"
	"fmt"
	"sync"

	"github.com/hibiken/asynq"
)

// TaskHandler processes one background task. It mirrors asynq's handler
// function shape so the same contract covers both the Redis-backed path
// and the Lite SyncTaskExecutor path.
type TaskHandler func(context.Context, *asynq.Task) error

// WorkerRegistry installs background task handlers by task type pattern.
// Registries are instance-scoped: first registration wins, and a duplicate
// Register returns an error while leaving the originally installed handler
// in place.
type WorkerRegistry interface {
	Register(pattern string, handler TaskHandler) error
}

// AsynqWorkerRegistry adapts an *asynq.ServeMux to WorkerRegistry. The mux
// itself silently overwrites duplicate handlers, so the registry records
// every installed pattern first and rejects duplicates before they reach
// the mux.
type AsynqWorkerRegistry struct {
	mu         sync.Mutex
	mux        *asynq.ServeMux
	registered map[string]struct{}
}

// NewAsynqWorkerRegistry returns a WorkerRegistry backed by mux.
func NewAsynqWorkerRegistry(mux *asynq.ServeMux) WorkerRegistry {
	return &AsynqWorkerRegistry{
		mux:        mux,
		registered: make(map[string]struct{}),
	}
}

// Register installs handler for pattern on the mux unless the pattern is
// already taken; duplicates keep the first handler and return an error.
func (r *AsynqWorkerRegistry) Register(pattern string, handler TaskHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.registered[pattern]; exists {
		return fmt.Errorf("worker %q already registered", pattern)
	}
	r.registered[pattern] = struct{}{}
	r.mux.HandleFunc(pattern, handler)
	return nil
}
