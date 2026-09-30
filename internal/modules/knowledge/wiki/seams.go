package wiki

// seams.go — W0（Pass B 23-knowledge-wikifaq §6.1）：模块侧 seam 声明（非 ib2 删除物）。
//
// wiki 包迁自横向宿主包（internal/application/service、internal/handler、
// internal/handler/session），对仍驻宿主包、属 K2/K4 owner（ib2 前未迁出）的
// 未导出符号一律经此处声明的窄端口注入（20 计划 §6.1 R2：消费侧注入 seam，
// spec §4.2 接口优先定义在使用方；禁止包级可变变量绕过注入，spec §4.3）。
// 本包禁止 import 宿主 internal/application/service（防与 W1/W2 宿主兼容层
// 反向成环）；生产实现由宿主构造兼容层（W2/H1）以闭包接到宿主现行符号，
// 子计划测试按 R2 以 fake 注入或不触达 seam 的零值构造。

import (
	"context"
	"reflect"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Seams 汇集 wiki 域服务对宿主 service 包（K2/K4 属主、ib2 前未迁出）纯
// 函数符号的依赖端口。由宿主构造兼容层（W2）接线；零值仅允许出现在不触达
// seam 调用点的测试构造中。
type Seams struct {
	// ResolveDeadSlug 照录宿主 K2 符号 resolveDeadSlug（slug_fuzzy.go:91）
	// 的签名：死链模糊复活（显示名反查 / 连字符归一 / bigram 相似度）。
	ResolveDeadSlug ResolveDeadSlugFn

	// FinalizeSubtaskDetached 照录宿主 K4 符号 finalizeSubtaskDetached
	//（knowledge.go:235）：脱离上下文递减 knowledge 的 finalizing 子任务
	// 计数。repo 为 nil 时实现自身为 no-op（宿主实现的语义）。
	FinalizeSubtaskDetached FinalizeSubtaskDetachedFn

	// IsLikelyRateLimitError 照录宿主 K4 符号 isLikelyRateLimitError
	//（knowledge_process.go:3902）：错误文本启发式判定上游限流。
	IsLikelyRateLimitError IsLikelyRateLimitErrorFn

	// RemoveSourceRef 照录宿主 K4 符号 removeSourceRef
	//（knowledge_delete.go:346）：从 source_refs 剔除指定 knowledge ID。
	RemoveSourceRef RemoveSourceRefFn

	// RecordWikiContentActivity 照录宿主 K2 导出符号
	// RecordWikiContentActivity（kb_activity.go:181）：把 Wiki 变更摘要投影
	// 进知识库活动流。
	RecordWikiContentActivity RecordWikiContentActivityFn

	// IsKnowledgeBaseNotFound 判定错误是否为宿主 repository 包哨兵
	// ErrKnowledgeBaseNotFound（计划 §10.4 备选方案：以 seam 注入
	// matchNotFound 替代 wiki 包直接 import 宿主 repository——后者会与
	// repository 侧 escapeLikePattern 转发 shim 成环）。W2 以
	// errors.Is(err, repository.ErrKnowledgeBaseNotFound) 闭包接线，语义不变。
	IsKnowledgeBaseNotFound func(err error) bool
}

// isKnowledgeBaseNotFound 是 IsKnowledgeBaseNotFound 的 nil 安全形态：
// 未接线（零值构造的测试 service）时返回 false——生产路径恒经 W2 接线，
// 该回落仅在测试构造不触达 KB 判定时出现（与迁移前 errors.Is 对非哨兵
// 错误返回 false 的行为一致）。
func (s Seams) isKnowledgeBaseNotFound(err error) bool {
	if s.IsKnowledgeBaseNotFound == nil {
		return false
	}
	return s.IsKnowledgeBaseNotFound(err)
}

// ResolveDeadSlugFn mirrors the host resolveDeadSlug signature (K2,
// slug_fuzzy.go:91). Both maps are consulted only — never mutated.
type ResolveDeadSlugFn = func(deadSlug string, displayText string, liveSlugs map[string]struct{}, titleToSlug map[string]string) (string, bool)

// FinalizeSubtaskDetachedFn mirrors the host finalizeSubtaskDetached
// signature (K4, knowledge.go:235).
type FinalizeSubtaskDetachedFn = func(ctx context.Context, repo interfaces.KnowledgeRepository, knowledgeID, source string, retErr error, superseded, final bool)

// IsLikelyRateLimitErrorFn mirrors the host isLikelyRateLimitError
// signature (K4, knowledge_process.go:3902).
type IsLikelyRateLimitErrorFn = func(err error) bool

// RemoveSourceRefFn mirrors the host removeSourceRef signature (K4,
// knowledge_delete.go:346). Handles both legacy "knowledgeID" and
// "knowledgeID|title" ref forms.
type RemoveSourceRefFn = func(refs types.StringArray, knowledgeID string) types.StringArray

// RecordWikiContentActivityFn mirrors the host K2 exported symbol
// RecordWikiContentActivity (kb_activity.go:181).
type RecordWikiContentActivityFn = func(ctx context.Context, audit interfaces.AuditLogService, tenantID uint64, kbID string, actions map[string]int)

// Span 是 wiki 摄取管线内部传递的 span 不透明句柄。真实 *Span 结构体
// （含全部字段语义）仍驻宿主 service 包（K4 属主 knowledge_span_tracker.go:72，
// ib2 迁移）；wiki 代码从不构造或读取句柄字段——开启后仅原样回传 tracker
// 方法——因此以最小包装保持类型边界而不复制实现。宿主侧适配器（W2
// wikiK3SpanAdapter）负责装卸底层 *Span。
type Span struct {
	raw any
}

// NewSpan wraps a host span pointer (nil-safe, 含 typed-nil 归一). Exported
// for the host-side adapter (W2) so it can translate between the host
// SpanTracker and the wiki-side narrow interface below.
//
// 宿主 spanTracker.LookupStage（查无记录 / list 失败）与 BeginSubSpan
// （parent==nil / Upsert 失败）均返回 nil *Span；装箱进 any 后 == nil 判定
// 失效（typed-nil），因此以 reflect 归一：任何 nil 指针一律映射 nil 句柄，
// 保 wiki 侧 `span == nil` 控制流与迁移前直传 *Span 语义一致
// （OCR R1 修复，回归测试见 seams_test.go 与宿主侧
// wiki_k3_span_adapter_test.go）。
func NewSpan(raw any) *Span {
	if raw == nil {
		return nil
	}
	if v := reflect.ValueOf(raw); v.Kind() == reflect.Pointer && v.IsNil() {
		return nil
	}
	return &Span{raw: raw}
}

// Raw unwraps the host span pointer (nil-safe). Only the host-side adapter
// calls this; wiki code treats Span values as opaque.
func (s *Span) Raw() any {
	if s == nil {
		return nil
	}
	return s.raw
}

// SpanTracker 是 wiki 摄取对宿主 K4 SpanTracker（knowledge_span_tracker.go:85）
// 的调用子集窄端口（LatestAttempt/LookupStage/BeginSubSpan/EndSpan/FailSpan/
// SkipSpan——wiki 文件实际调用的 6 方法，照录签名）。因 Span 为具名类型且
// 宿主实现返回宿主 *Span，K4 实现无法结构性满足本接口：宿主适配器
// （W2 wikiK3SpanAdapter，包装 NewSpan/Raw）承担签名转换，行为等价
// （句柄原样回传，nil 语义保持）。
type SpanTracker interface {
	// LatestAttempt returns the highest attempt number recorded for the
	// knowledge, or 0 if it's never been parsed.
	LatestAttempt(ctx context.Context, knowledgeID string) int

	// LookupStage returns the stage's span for an in-flight attempt — the
	// cross-process bridge that lets the asynq wiki worker attach subspans
	// to the parent stage span created by the upstream pipeline.
	LookupStage(ctx context.Context, knowledgeID string, attempt int, stage string) *Span

	// BeginSubSpan creates a child span under parent. kind is "subspan" or
	// "generation".
	BeginSubSpan(ctx context.Context, parent *Span, name, kind string, input types.JSONMap) *Span

	// EndSpan marks span as done with optional output. Safe with nil.
	EndSpan(ctx context.Context, span *Span, output types.JSONMap)

	// FailSpan marks span as failed and cascade-cancels its descendants.
	// errorDetail (a Go error) is recorded verbatim in error_detail
	// (truncated to 8 KB) for admin views.
	FailSpan(ctx context.Context, span *Span, errorCode, errorMessage string, errorDetail error)

	// SkipSpan marks an intentionally not-run span.
	SkipSpan(ctx context.Context, span *Span, reason string)
}

// noopSpanTracker collapses every method to a no-op（照录宿主 K4
// noopSpanTracker 语义，knowledge_span_tracker.go:858）。零值/未接线构造的
// wikiIngestService 经 tracker() 回落到此实现。
type noopSpanTracker struct{}

func (noopSpanTracker) LatestAttempt(_ context.Context, _ string) int { return 0 }
func (noopSpanTracker) LookupStage(_ context.Context, _ string, _ int, _ string) *Span {
	return nil
}
func (noopSpanTracker) BeginSubSpan(_ context.Context, _ *Span, _, _ string, _ types.JSONMap) *Span {
	return nil
}
func (noopSpanTracker) EndSpan(_ context.Context, _ *Span, _ types.JSONMap) {}
func (noopSpanTracker) FailSpan(_ context.Context, _ *Span, _, _ string, _ error) {
}
func (noopSpanTracker) SkipSpan(_ context.Context, _ *Span, _ string) {}
