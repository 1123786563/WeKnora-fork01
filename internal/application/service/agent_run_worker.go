package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
)

type WorkerConfig struct {
	Enabled                        bool
	Lease, Heartbeat, ScanInterval time.Duration
	MaxWorkers                     int
}

func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{Lease: time.Minute, Heartbeat: 15 * time.Second, ScanInterval: 5 * time.Second, MaxWorkers: 4}
}
func (c WorkerConfig) Validate() error {
	if c.Lease <= 0 || c.Heartbeat <= 0 || c.ScanInterval <= 0 || c.MaxWorkers <= 0 {
		return errors.New("agent worker durations and max_workers must be positive")
	}
	if c.Heartbeat*2 >= c.Lease {
		return fmt.Errorf("agent worker heartbeat must be less than half the lease")
	}
	return nil
}

type AgentRunWorker struct {
	store   agentruntime.RunStore
	execute func(context.Context, agentruntime.Fence) error
	cfg     WorkerConfig
	owner   string
	mu      sync.Mutex
	active  map[string]context.CancelFunc
}

func NewAgentRunWorker(store agentruntime.RunStore, execute func(context.Context, agentruntime.Fence) error, cfg WorkerConfig) (*AgentRunWorker, error) {
	if store == nil || execute == nil {
		return nil, errors.New("agent worker store and executor are required")
	}
	if !cfg.Enabled {
		return &AgentRunWorker{store: store, execute: execute, cfg: cfg, owner: fmt.Sprintf("worker-%d", time.Now().UnixNano()), active: make(map[string]context.CancelFunc)}, nil
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &AgentRunWorker{store: store, execute: execute, cfg: cfg, owner: fmt.Sprintf("worker-%d", time.Now().UnixNano()), active: make(map[string]context.CancelFunc)}, nil
}

func (w *AgentRunWorker) Run(ctx context.Context) error {
	if w == nil {
		return errors.New("agent worker is nil")
	}
	if err := w.cfg.Validate(); err != nil {
		return err
	}
	if !w.cfg.Enabled {
		return nil
	}
	ticker := time.NewTicker(w.cfg.ScanInterval)
	defer ticker.Stop()
	// Recover immediately on process startup, then continue polling.
	if err := w.Tick(ctx); err != nil && ctx.Err() == nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			w.drain(ctx)
			return nil
		case <-ticker.C:
			if err := w.Tick(ctx); err != nil && ctx.Err() == nil {
				return err
			}
		}
	}
}

func (w *AgentRunWorker) Tick(ctx context.Context) error {
	if w == nil {
		return errors.New("agent worker is nil")
	}
	if !w.cfg.Enabled {
		return nil
	}
	keys, err := w.store.Scan(ctx, w.cfg.MaxWorkers)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		run, getErr := w.store.Get(ctx, key)
		if getErr != nil || run.Status == "waiting_user" || run.Status == "canceled" || run.Status == "succeeded" || run.Status == "failed" {
			continue
		}
		id := fmt.Sprintf("%d/%s", key.TenantID, key.RunID)
		w.mu.Lock()
		if len(w.active) >= w.cfg.MaxWorkers {
			w.mu.Unlock()
			break
		}
		if _, ok := w.active[id]; ok {
			w.mu.Unlock()
			continue
		}
		runCtx, cancel := context.WithCancel(context.Background()) // independent of HTTP/request ctx
		w.active[id] = cancel
		w.mu.Unlock()
		fence, claimErr := w.store.Claim(ctx, key, w.owner, w.cfg.Lease)
		if claimErr != nil {
			cancel()
			w.mu.Lock()
			delete(w.active, id)
			w.mu.Unlock()
			continue
		}
		go w.runOne(runCtx, id, fence)
	}
	return nil
}

func (w *AgentRunWorker) runOne(ctx context.Context, id string, fence agentruntime.Fence) {
	defer func() { w.mu.Lock(); delete(w.active, id); w.mu.Unlock() }()
	done := make(chan struct{})
	defer close(done)
	renewCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		ticker := time.NewTicker(w.cfg.Heartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-renewCtx.Done():
				return
			case <-ticker.C:
				if err := w.store.Renew(renewCtx, fence, w.cfg.Lease); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	_ = w.execute(renewCtx, fence)
}

func (w *AgentRunWorker) drain(ctx context.Context) {
	w.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(w.active))
	for _, c := range w.active {
		cancels = append(cancels, c)
	}
	w.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for {
		w.mu.Lock()
		n := len(w.active)
		w.mu.Unlock()
		if n == 0 {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-deadline.C:
			return
		case <-time.After(5 * time.Millisecond):
		}
	}
}
