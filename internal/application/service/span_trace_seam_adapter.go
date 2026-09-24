// Pass B 过渡 seam 适配器：删除点 ib2（Integration Brief 登记，plan 21-knowledge-ingest §6.3 接线义务）。
// 宿主 K4 属主实现（SpanTracker / enqueueSummaryRefresh / write-family /
// buildKnowledgeIndexContent）→ ingest R2 seam 的纯转发提供器，零逻辑复制。
// K1→K4 期间由本包 chunk_ingest_shim.go 内部消费；K4 搬迁 knowledge_write.go /
// knowledge_span_tracker.go / knowledge_summary_refresh.go / knowledge_index_content.go
// 后指到其导出包装，ib2 由集成工程师切换 container 直供并删除本文件。
package service

import (
	"context"

	"github.com/Tencent/WeKnora/internal/modules/knowledge/ingest"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// spanTraceSeamAdapter 把宿主 SpanTracker（knowledge_span_tracker.go:85）投影为
// ingest.SpanTraceSeam；*Span 句柄经 any 不透明传递（ingest 不得 import 宿主，
// service→ingest→service import 环实测裁决，plan §6.3）。
type spanTraceSeamAdapter struct {
	tr SpanTracker
}

// NewSpanTraceSeamAdapter 返回四方法纯转发适配器。
func NewSpanTraceSeamAdapter(tr SpanTracker) ingest.SpanTraceSeam {
	return spanTraceSeamAdapter{tr: tr}
}

// spanPtr 从 seam 的 any 句柄还原 *Span（nil 安全）。
func spanPtr(span any) *Span {
	if span == nil {
		return nil
	}
	return span.(*Span)
}

func (a spanTraceSeamAdapter) LookupStage(ctx context.Context, knowledgeID string, attempt int, stage string) any {
	if a.tr == nil {
		return nil
	}
	return a.tr.LookupStage(ctx, knowledgeID, attempt, stage)
}

func (a spanTraceSeamAdapter) BeginSubSpan(ctx context.Context, parent any, name, kind string, input types.JSONMap) any {
	if a.tr == nil {
		return nil
	}
	return a.tr.BeginSubSpan(ctx, spanPtr(parent), name, kind, input)
}

func (a spanTraceSeamAdapter) EndSpan(ctx context.Context, span any, output types.JSONMap) {
	if a.tr == nil {
		return
	}
	a.tr.EndSpan(ctx, spanPtr(span), output)
}

func (a spanTraceSeamAdapter) FailSpan(ctx context.Context, span any, code, message string, err error) {
	if a.tr == nil {
		return
	}
	a.tr.FailSpan(ctx, spanPtr(span), code, message, err)
}

// EnqueueSummaryRefreshProvider 返回捕获 tracker 的 summary 刷新入队闭包；
// kbReader 参数类型 ingest.KBByIDLookup 与宿主 summaryKnowledgeBaseReader
// 结构等价（单方法 GetKnowledgeBaseByID），Go 结构化接口直接传递。
func EnqueueSummaryRefreshProvider(tr SpanTracker) func(context.Context, interfaces.KnowledgeRepository, interfaces.TaskEnqueuer, ingest.KBByIDLookup, *types.Knowledge) error {
	return func(
		ctx context.Context,
		repo interfaces.KnowledgeRepository,
		taskEnqueuer interfaces.TaskEnqueuer,
		kbReader ingest.KBByIDLookup,
		knowledge *types.Knowledge,
	) error {
		return enqueueSummaryRefresh(ctx, repo, taskEnqueuer, kbReader, tr, knowledge)
	}
}

// hostKnowledgeWriteGuard 以宿主 knowledge_write.go 包级函数（K4 属主）实现
// ingest.KnowledgeWriteGuard，方法名一一对应，纯转发零复制。
type hostKnowledgeWriteGuard struct{}

// KnowledgeWriteGuardProvider 返回宿主写授权 seam 实现；K4 搬迁
// knowledge_write.go 后指到其导出包装（plan §6.3 接线义务）。
func KnowledgeWriteGuardProvider() ingest.KnowledgeWriteGuard {
	return hostKnowledgeWriteGuard{}
}

func (hostKnowledgeWriteGuard) WriteResourceIDs(ids []string) ([]string, error) {
	return writeResourceIDs(ids)
}

func (hostKnowledgeWriteGuard) WriteExecutionTenant(ctx context.Context) (uint64, error) {
	return writeExecutionTenant(ctx)
}

func (hostKnowledgeWriteGuard) LoadKnowledgeWrite(
	ctx context.Context, repo interfaces.KnowledgeRepository, lookup ingest.KBByIDLookup, id string,
) (*types.Knowledge, *types.KnowledgeBase, error) {
	return loadKnowledgeWrite(ctx, repo, lookup, id)
}

func (hostKnowledgeWriteGuard) LoadKnowledgeWriteBatch(
	ctx context.Context, repo interfaces.KnowledgeRepository, lookup ingest.KBByIDLookup, ids []string,
) ([]*types.Knowledge, error) {
	return loadKnowledgeWriteBatch(ctx, repo, lookup, ids)
}

// BuildKnowledgeIndexContentProvider 返回宿主 buildKnowledgeIndexContent
// （knowledge_index_content.go:12，K4 属主）的函数值；K1.2 增量 seam
// （plan §6.3 未枚举，按既有 R2 机制具体化，节点报告登记）。
func BuildKnowledgeIndexContentProvider() func(knowledge *types.Knowledge, content string) string {
	return buildKnowledgeIndexContent
}
