package ingest

// 本文件承载 K1 摄取域的 R2 消费侧 seam 类型定义（plan 21-knowledge-ingest §6.3，
// spec §4.2「接口优先定义在使用方模块」）。seam 一律构造注入，禁包级 var 注入
// （spec §4.3）；生产接线由集成工程师按 Integration Brief 在 K5/ib2 执行。

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// KBByIDLookup 是宿主 knowledge_write.go 的 knowledgeBaseWriteLookup 与
// knowledge_summary_refresh.go 的 summaryKnowledgeBaseReader 的共同结构
// （单方法 GetKnowledgeBaseByID）；interfaces.KnowledgeBaseRepository 天然满足。
type KBByIDLookup interface {
	GetKnowledgeBaseByID(context.Context, string) (*types.KnowledgeBase, error)
}

// KnowledgeWriteGuard 收敛 K4 属主（internal/application/service 包）的写路径
// 授权族：writeResourceIDs / writeExecutionTenant / loadKnowledgeWrite /
// loadKnowledgeWriteBatch（knowledge_write.go:17/:32/:60/:93）。方法名与宿主
// 包级函数一一对应；chunk_write.go 与 chunk.go 的调用点经本 seam 走宿主实现，
// 零逻辑复制。
type KnowledgeWriteGuard interface {
	// WriteResourceIDs 对应宿主 writeResourceIDs（去空/去重）。
	WriteResourceIDs(ids []string) ([]string, error)
	// WriteExecutionTenant 对应宿主 writeExecutionTenant（写侧租户解析）。
	WriteExecutionTenant(ctx context.Context) (uint64, error)
	// LoadKnowledgeWrite 对应宿主 loadKnowledgeWrite（单文档写授权装载）。
	LoadKnowledgeWrite(ctx context.Context, repo interfaces.KnowledgeRepository, lookup KBByIDLookup, id string) (*types.Knowledge, *types.KnowledgeBase, error)
	// LoadKnowledgeWriteBatch 对应宿主 loadKnowledgeWriteBatch（批量写授权装载）。
	LoadKnowledgeWriteBatch(ctx context.Context, repo interfaces.KnowledgeRepository, lookup KBByIDLookup, ids []string) ([]*types.Knowledge, error)
}

// SpanTraceSeam 是 K4 属主 SpanTracker（knowledge_span_tracker.go:85，11 方法）
// 的最小投影：ingest 侧消费 LookupStage/BeginSubSpan/EndSpan/FailSpan 四方法。
// 句柄以 any 传递（宿主 *Span 的不透明引用）——ingest 若 import 宿主 service 包
// 将与 R1 shim 形成 service→ingest→service import 环，实测裁决不可行（plan §6.3）。
type SpanTraceSeam interface {
	LookupStage(ctx context.Context, knowledgeID string, attempt int, stage string) any
	BeginSubSpan(ctx context.Context, parent any, name, kind string, input types.JSONMap) any
	EndSpan(ctx context.Context, span any, output types.JSONMap)
	FailSpan(ctx context.Context, span any, code, message string, err error)
}
