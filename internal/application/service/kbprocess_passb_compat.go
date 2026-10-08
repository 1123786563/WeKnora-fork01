package service

// Pass B 宿主兼容层（b2-k-process / K4.2）：service 独立面自包含子集 3 文件
// （knowledge_write.go、knowledge_index_content.go、knowledge_task_options.go）
// 已物理迁移至 internal/knowledge/process（docs/plans/passb/
// 24-knowledge-process.md §3.1；Ruling 2026-09-25-DEFERRED-FILE-SPLIT 收缩迁移，
// span_tracker/housekeeping 两对生产+测试因 Mimosa DDL 测试常量阻断成对推迟，
// 留守宿主原生解析，故本文件不含其别名面）。
// 本文件为留守宿主消费方（span_trace_seam_adapter.go 的 hostKnowledgeWriteGuard
// 四方法与 BuildKnowledgeIndexContentProvider/KnowledgeWriteKBProvider、
// chunk_ingest_shim.go:204、推迟件 knowledge_create/delete/delete_plan/process/
// replace/transfer/reparse_scope/clone_move、留守 knowledge_index_content_test.go）
// 提供同形一行委托，调用点零改动。
// 删除点：ib2 集成屏障直连后（Integration Brief 指令），随 manifest compat
// 登记行一并删除（Ruling 2026-09-25-TRANSITION-SHIM-ROW-REGISTRATION）。

import (
	"context"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/hibiken/asynq"
)

// writeResourceIDs 委托 process.WriteResourceIDs（knowledge_write.go:17 原定义，
// K4.2 导出；K1 adapter hostKnowledgeWriteGuard 与推迟件消费面保护）。
func writeResourceIDs(ids []string) ([]string, error) {
	return WriteResourceIDs(ids)
}

// writeExecutionTenant 委托 process.WriteExecutionTenant（knowledge_write.go:32 原定义）。
func writeExecutionTenant(ctx context.Context) (uint64, error) {
	return WriteExecutionTenant(ctx)
}

// knowledgeWriteKB 委托 process.KnowledgeWriteKB（knowledge_write.go:42 原定义）；
// 形参接口导出为 process.KnowledgeBaseWriteLookup（原 knowledgeBaseWriteLookup，
// knowledge_write.go:13），与 ingest.KBByIDLookup 结构等价（单方法
// GetKnowledgeBaseByID），接口到接口隐式转换，语义不变。
func knowledgeWriteKB(
	ctx context.Context,
	lookup KnowledgeBaseWriteLookup,
	knowledge *types.Knowledge,
) (*types.KnowledgeBase, error) {
	return KnowledgeWriteKB(ctx, lookup, knowledge)
}

// loadKnowledgeWrite 委托 process.LoadKnowledgeWrite（knowledge_write.go:60 原定义）。
func loadKnowledgeWrite(
	ctx context.Context,
	repo interfaces.KnowledgeRepository,
	lookup KnowledgeBaseWriteLookup,
	id string,
) (*types.Knowledge, *types.KnowledgeBase, error) {
	return LoadKnowledgeWrite(ctx, repo, lookup, id)
}

// loadKnowledgeWriteBatch 委托 process.LoadKnowledgeWriteBatch
// （knowledge_write.go:93 原定义）。
func loadKnowledgeWriteBatch(
	ctx context.Context,
	repo interfaces.KnowledgeRepository,
	lookup KnowledgeBaseWriteLookup,
	ids []string,
) ([]*types.Knowledge, error) {
	return LoadKnowledgeWriteBatch(ctx, repo, lookup, ids)
}

// buildKnowledgeIndexContent 委托 process.BuildKnowledgeIndexContent
// （knowledge_index_content.go:12 原定义；留守 knowledge_index_content_test.go
// 消费面保护，兼作 compat 委托等价差分锚点）。
func buildKnowledgeIndexContent(knowledge *types.Knowledge, content string) string {
	return BuildKnowledgeIndexContent(knowledge, content)
}

// documentProcessTaskOptions 委托 process.DocumentProcessTaskOptions
// （knowledge_task_options.go:11 原定义；推迟件 knowledge_create/clone_move 消费面保护）。
func documentProcessTaskOptions(cfg *config.Config, extra ...asynq.Option) []asynq.Option {
	return DocumentProcessTaskOptions(cfg, extra...)
}

// knowledgePostProcessTaskOptions 委托 process.KnowledgePostProcessTaskOptions
// （knowledge_task_options.go:21 原定义；chunk_ingest_shim.go:204 与推迟件
// knowledge_process.go 消费面保护）。
func knowledgePostProcessTaskOptions() []asynq.Option {
	return KnowledgePostProcessTaskOptions()
}
