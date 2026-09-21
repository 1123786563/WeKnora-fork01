package bootstrap

import (
	"fmt"
	"sort"
	"sync"
)

// WorkerRegistry 是 duplicate-safe 的任务处理器登记表。
// 同一个模块的处理器集合应分别登记到 Redis（asynq mux）与 Lite（SyncTaskExecutor）
// 两个 registry，再用 VerifyWorkerParity 校验两模式集合一致。
type WorkerRegistry struct {
	mode string
	sink WorkerSink

	mu       sync.Mutex
	handlers map[string]any
}

// WorkerSink 接收通过重复校验的任务注册，是底层 asynq mux / Lite executor 的最小适配面。
type WorkerSink interface {
	RegisterTaskHandler(taskType string, handler any)
}

// NewWorkerRegistry 构造指定模式（如 "redis"/"lite"）的登记表；sink 可为 nil。
// handler 以 any 承载，不约束签名：asynq 与 Lite 处理器同为
// func(context.Context, *asynq.Task) error，由底层 sink 自行断言。
func NewWorkerRegistry(mode string, sink WorkerSink) *WorkerRegistry {
	return &WorkerRegistry{mode: mode, sink: sink, handlers: make(map[string]any)}
}

// Mode 返回登记表对应的 worker 模式。
func (r *WorkerRegistry) Mode() string {
	return r.mode
}

// Register 登记一个任务类型处理器。taskType 重复时返回以 "already registered"
// 开头的错误，且不调用底层 sink。
func (r *WorkerRegistry) Register(taskType string, handler any) error {
	if taskType == "" {
		return fmt.Errorf("invalid worker registration: task type must be non-empty")
	}
	r.mu.Lock()
	if _, dup := r.handlers[taskType]; dup {
		r.mu.Unlock()
		return fmt.Errorf("already registered: worker task type %s (mode %s)", taskType, r.mode)
	}
	r.handlers[taskType] = handler
	r.mu.Unlock()
	if r.sink != nil {
		r.sink.RegisterTaskHandler(taskType, handler)
	}
	return nil
}

// TaskTypes 返回已登记任务类型的排序快照。
func (r *WorkerRegistry) TaskTypes() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.handlers))
	for tt := range r.handlers {
		out = append(out, tt)
	}
	sort.Strings(out)
	return out
}

// VerifyWorkerParity 校验两个模式（Redis vs Lite）登记的任务类型集合完全一致；
// 不一致时返回的错误包含两侧差集（排序、去重）。
func VerifyWorkerParity(redis, lite *WorkerRegistry) error {
	if redis == nil || lite == nil {
		return fmt.Errorf("worker parity check requires two non-nil registries")
	}
	rs, ls := redis.TaskTypes(), lite.TaskTypes()
	rset := make(map[string]bool, len(rs))
	for _, tt := range rs {
		rset[tt] = true
	}
	lset := make(map[string]bool, len(ls))
	for _, tt := range ls {
		lset[tt] = true
	}
	var onlyRedis, onlyLite []string
	for _, tt := range rs {
		if !lset[tt] {
			onlyRedis = append(onlyRedis, tt)
		}
	}
	for _, tt := range ls {
		if !rset[tt] {
			onlyLite = append(onlyLite, tt)
		}
	}
	if len(onlyRedis) == 0 && len(onlyLite) == 0 {
		return nil
	}
	return fmt.Errorf("worker parity mismatch (mode %q vs %q): only-in-%s=%v, only-in-%s=%v",
		redis.mode, lite.mode, redis.mode, onlyRedis, lite.mode, onlyLite)
}
