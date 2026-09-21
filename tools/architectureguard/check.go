package main

import (
	"fmt"
	"sort"
)

// Diagnostic 是一条守卫违规（排序后输出）。
type Diagnostic struct {
	Check   string
	Message string
}

// String 输出形如 "architectureguard: <check>: <message>"。
func (d Diagnostic) String() string {
	return fmt.Sprintf("architectureguard: %s: %s", d.Check, d.Message)
}

// SortDiagnostics 按 String 排序并去重。
func SortDiagnostics(ds []Diagnostic) []Diagnostic {
	out := append([]Diagnostic(nil), ds...)
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	dedup := make([]Diagnostic, 0, len(out))
	for i, d := range out {
		if i == 0 || d.String() != out[i-1].String() {
			dedup = append(dedup, d)
		}
	}
	return dedup
}

// Summary 汇总发现规模（与 F0 基线 §3.2 口径对照用）。
type Summary struct {
	RoutesLiteral  int
	RoutesAPIKey   int
	RoutesHandle   int
	RoutesTotal    int
	WorkerRedis    int
	WorkerLite     int
	Hooks          int
	ModulesScanned int
}

// ManifestView 是 architectureguard 需要的 manifest 只读子集
// （与 tools/modulemove 的 LOCKED schema 对齐；guard 不 import main 包，故最小重复）。
type ManifestView struct {
	Module               string
	MoveFroms            []string
	LegacyPaths          []string
	RouteEntries         []string // 原文（"RegisterX — file:line" 形态）
	WorkerTypes          []string
	LifecycleHookEntries []string // 原文（自由文本，识别符形态的才做强校验）
}

// platformRouteFiles 是 moves/README.md 记录的 platform 路由残留文件
// （/health、/swagger/*any、静态前端、files 授权/预签名、workbench HMAC 挂载点）。
var platformRouteFiles = map[string]bool{
	"internal/router/router.go": true,
	"internal/router/static.go": true,
	"internal/router/files.go":  true,
}

// platformHooks 是 moves/README.md 记录的 platform 生命周期/worker 启动残留。
var platformHooks = map[string]bool{
	"registerLangfuseCleanup": true,
	"registerPoolCleanup":     true,
	"RunAsynqServer":          true,
	"RegisterSyncHandlers":    true,
}

// platformLegacyFiles 是横向目录内归 platform 本体的 3 个文件（moves/README.md）。
var platformLegacyFiles = map[string]bool{
	"internal/handler/list_pagination.go":           true,
	"internal/handler/upload_limit.go":              true,
	"internal/application/repository/task_queue.go": true,
}

// horizontalDirs 是 Pass B 之前仍承载多模块遗留文件的水平业务目录。
var horizontalDirs = []string{
	"internal/application/service",
	"internal/application/repository",
	"internal/handler",
}

// Report 汇总一次守卫运行。
type Report struct {
	Summary     Summary
	Diagnostics []Diagnostic
}

// LoadManifestViews 加载 manifest 目录下全部 16 份清单为守卫所需的只读子集
// （严格 KnownFields(true)，与 tools/modulemove 同一 schema 约束）。
func LoadManifestViews(root string) ([]ManifestView, error) {
	return nil, fmt.Errorf("not implemented: architectureguard.LoadManifestViews(%s)", root)
}

// Run 执行全部守卫检查。
func Run(root string, mods []ManifestView) (Report, error) {
	return Report{}, fmt.Errorf("not implemented: architectureguard.Run(%s)", root)
}
