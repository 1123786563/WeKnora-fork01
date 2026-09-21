package bootstrap

import (
	"context"
	"sort"
	"sync"
)

// LifecycleHook 是 container.Invoke 生命周期挂点的最小签名契约。
type LifecycleHook func(ctx context.Context) error

// LifecycleSink 接收通过重复校验的生命周期挂点注册，是底层 hook 集合的最小适配面。
type LifecycleSink interface {
	RegisterLifecycleHook(name string, hook LifecycleHook)
}

// LifecycleRegistry 是 duplicate-safe 的生命周期挂点登记表。
type LifecycleRegistry struct {
	sink LifecycleSink

	mu    sync.Mutex
	hooks map[string]LifecycleHook
}

// NewLifecycleRegistry 构造登记表；sink 可为 nil（仅查重、不落底层）。
func NewLifecycleRegistry(sink LifecycleSink) *LifecycleRegistry {
	return &LifecycleRegistry{sink: sink, hooks: make(map[string]LifecycleHook)}
}

// Register 登记一个生命周期挂点。name 重复时返回以 "already registered" 开头的
// 错误，且不调用底层 sink。
func (r *LifecycleRegistry) Register(name string, hook LifecycleHook) error {
	r.mu.Lock()
	r.hooks[name] = hook
	r.mu.Unlock()
	if r.sink != nil {
		r.sink.RegisterLifecycleHook(name, hook)
	}
	return nil
}

// Names 返回已登记挂点名称的排序快照。
func (r *LifecycleRegistry) Names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.hooks))
	for name := range r.hooks {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Lookup 返回已登记的挂点（IA 阶段按名 Invoke 用）。
func (r *LifecycleRegistry) Lookup(name string) (LifecycleHook, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	h, ok := r.hooks[name]
	return h, ok
}
