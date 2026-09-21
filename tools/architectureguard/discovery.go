package main

import "fmt"

// RouteReg 是一次 HTTP 路由注册的静态发现结果。
// Kind: "literal"（gin 方法字面调用）、"apiKeyRoute"（rbac 助手）、"handle"（直连 .Handle( 且方法为字面量）。
// 与 F0 基线 §3.2 的计数口径一致：internal/router/*.go 与 internal/handler/**/*.go 非测试文件，
// AST 解析天然排除注释文本。
type RouteReg struct {
	File   string // 仓库相对路径
	Line   int
	Func   string // 所属顶层函数名
	Method string // GET/POST/.../ANY（Any 记为 ANY）
	Path   string // 调用点的字面路径参数（不可解析时为 ""）
	Kind   string
}

// WorkerReg 是一次任务处理器注册的静态发现结果。
// Mode: "redis"（internal/router task.go 侧 mux.HandleFunc）或 "lite"（sync_task.go 侧 RegisterHandler）。
type WorkerReg struct {
	TaskType string // 首参选择器文本，如 "types.TypeChunkExtract"
	File     string
	Line     int
	Func     string
	Mode     string
}

// HookReg 是一次 container.Invoke(<fn>) 生命周期挂点注册的静态发现结果。
type HookReg struct {
	Name string // 首参表达式文本（如 "registerPoolCleanup" 或 "chatpipeline.NewPluginSearch"）
	File string
	Line int
}

// DiscoverRoutes 扫描 internal/router 与 internal/handler 下的非测试 .go，
// 按基线 §3.2 口径发现全部路由注册点。
func DiscoverRoutes(root string) ([]RouteReg, error) {
	return nil, fmt.Errorf("not implemented: architectureguard.DiscoverRoutes(%s)", root)
}

// DiscoverWorkers 扫描 internal/router 下的非测试 .go，分别发现 Redis（mux.HandleFunc）
// 与 Lite（RegisterHandler）两侧的任务注册。
func DiscoverWorkers(root string) (redis, lite []WorkerReg, err error) {
	return nil, nil, fmt.Errorf("not implemented: architectureguard.DiscoverWorkers(%s)", root)
}

// DiscoverHooks 扫描 internal/container 下的非测试 .go，发现全部 container.Invoke 挂点。
func DiscoverHooks(root string) ([]HookReg, error) {
	return nil, fmt.Errorf("not implemented: architectureguard.DiscoverHooks(%s)", root)
}

// FuncDeclNames 返回指定仓库相对目录（递归）内全部顶层函数名（非测试 .go）。
// 用于路由入口覆盖检查。
func FuncDeclNames(root string, dirs ...string) (map[string]bool, error) {
	return nil, fmt.Errorf("not implemented: architectureguard.FuncDeclNames(%s, %v)", root, dirs)
}
