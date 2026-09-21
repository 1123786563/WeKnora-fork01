// Package bootstrap 提供 Pass A 之后 IA 阶段装配用的重复注册防护登记表。
//
// 这些登记表是契约，不是当前装配路径（IA 障碍任务才接入 router/container）：
// 构造函数依赖注入底层 sink（router/mux/hook 集合适配点），不持有任何全局可变状态。
// 重复注册一律返回以 "already registered" 开头的错误，且在报错时绝不触碰底层 sink。
package bootstrap

import (
	"fmt"
	"sort"
	"sync"
)

// Route 是一条 HTTP 路由注册的唯一标识（method + 完整挂载路径）。
type Route struct {
	Method string
	Path   string
}

// RouteSink 接收通过重复校验的路由注册，是底层 router/engine 的最小适配面。
// 由构造方注入（例如 IA 阶段包一层 gin.Engine.Handle），本包不 import gin。
type RouteSink interface {
	HandleRoute(method, path string)
}

// RouteRegistry 是 duplicate-safe 的路由登记表。
type RouteRegistry struct {
	sink RouteSink

	mu     sync.Mutex
	routes map[Route]struct{}
}

// NewRouteRegistry 构造登记表；sink 可为 nil（仅查重、不落底层）。
func NewRouteRegistry(sink RouteSink) *RouteRegistry {
	return &RouteRegistry{sink: sink, routes: make(map[Route]struct{})}
}

// Register 登记一条路由。method/path 重复时返回以 "already registered" 开头的
// 错误，并且不调用底层 sink（底层保持不变）。
func (r *RouteRegistry) Register(method, path string) error {
	if method == "" || path == "" {
		return fmt.Errorf("invalid route registration: method and path must be non-empty")
	}
	r.mu.Lock()
	r.routes[Route{Method: method, Path: path}] = struct{}{}
	r.mu.Unlock()
	if r.sink != nil {
		r.sink.HandleRoute(method, path)
	}
	return nil
}

// Routes 返回当前登记路由的排序快照。
func (r *RouteRegistry) Routes() []Route {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Route, 0, len(r.routes))
	for rt := range r.routes {
		out = append(out, rt)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Method != out[j].Method {
			return out[i].Method < out[j].Method
		}
		return out[i].Path < out[j].Path
	})
	return out
}
