// Pass B (23-knowledge-wikifaq) 过渡 shim — 删除点 ib2（Integration Brief 登记）。
// 先例：12-commercial.md:449。
//
// wiki 域 12 文件已迁入 internal/modules/knowledge/wiki（K3.1）。本文件把
// 宿主 service 包仍存在的他 owner 调用点（K1 extract/image_multimodal、
// K4 knowledge_*、router/task.go、container/recover_pending_wiki_tasks.go
// 及留驻宿主测试）所需的 wiki 符号以一行转发/别名保持编译，行为零变化。
package service

import (
	"context"

	"github.com/Tencent/WeKnora/internal/modules/knowledge/wiki"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// --- 原未导出符号：保 K1/K4 同包裸调用点编译 ---

func previewText(s string, maxRunes int) string { return wiki.PreviewText(s, maxRunes) }

func enqueueWikiIngestTrigger(
	ctx context.Context,
	task interfaces.TaskEnqueuer,
	tenantID uint64,
	kbID string,
) error {
	return wiki.EnqueueWikiIngestTrigger(ctx, task, tenantID, kbID)
}

func enqueueWikiRetract(
	ctx context.Context,
	task interfaces.TaskEnqueuer,
	pendingRepo interfaces.TaskPendingOpsRepository,
	payload wiki.WikiRetractPayload,
) error {
	return wiki.EnqueueWikiRetractWithError(ctx, task, pendingRepo, payload)
}

func extractRealText(content string) string { return wiki.ExtractRealText(content) }

func newWikiIngestPendingOp(
	ctx context.Context,
	tenantID uint64,
	kbID, knowledgeID string,
) (*types.TaskPendingOp, error) {
	return wiki.NewWikiIngestPendingOp(ctx, tenantID, kbID, knowledgeID)
}

// minTextContentRunes 与 wiki.MinTextContentRunes 初始化值相同（10）。
// 生产代码从不改写两者；宿主侧测试（knowledge_summary_test.go）改写的是
// 本副本，其断言的 K4 读点（checkSufficientSummaryContent）同在本包，行为
// 与迁移前一致。ib2 K4 迁出时收口为单一变量（见 Integration Brief）。
var minTextContentRunes = wiki.MinTextContentRunes

func realTextRuneCount(content string) int { return wiki.RealTextRuneCount(content) }

func uniqueWikiFolderIDs(values []string) []string { return wiki.UniqueWikiFolderIDs(values) }

// --- 原已导出符号：转发/别名 ---

func EnqueueWikiIngest(
	ctx context.Context,
	task interfaces.TaskEnqueuer,
	pendingRepo interfaces.TaskPendingOpsRepository,
	tenantID uint64,
	kbID, knowledgeID string,
) (bool, error) {
	return wiki.EnqueueWikiIngest(ctx, task, pendingRepo, tenantID, kbID, knowledgeID)
}

// ErrWikiIngestConcurrent 保持同一错误实例（router/task.go:124 errors.Is 语义不变）。
var ErrWikiIngestConcurrent = wiki.ErrWikiIngestConcurrent

// WikiIngestPayload 类型别名（container/recover_pending_wiki_tasks.go:75 及
// reset_pending_tasks_test.go 经 service.WikiIngestPayload 引用；JSON 字段不变）。
type WikiIngestPayload = wiki.WikiIngestPayload

// WikiRetractPayload 类型别名（knowledge_delete.go:233 字面量构造）。
type WikiRetractPayload = wiki.WikiRetractPayload

// WikiPendingOp 类型别名（knowledge_move_wiki_test.go / knowledge_housekeeping_test.go）。
type WikiPendingOp = wiki.WikiPendingOp

// WikiDeletedTombstoneKey 转发（knowledgeService.cleanupWikiOnKnowledgeDelete 写同一键）。
func WikiDeletedTombstoneKey(kbID, knowledgeID string) string {
	return wiki.WikiDeletedTombstoneKey(kbID, knowledgeID)
}

// 宿主测试仍以裸标识符引用的常量（knowledge_housekeeping_test.go:105）。
const (
	wikiTaskType  = wiki.WikiTaskType
	wikiTaskScope = wiki.WikiTaskScope
	WikiOpIngest  = wiki.WikiOpIngest
	WikiOpRetract = wiki.WikiOpRetract
)

// wikiDeletedTTL 照录 wiki 包同名常量值（knowledge_delete.go:178 写墓碑键
// TTL 用）。K3.1 前为同包常量直引；值为纯字面量导出等价。
const wikiDeletedTTL = wiki.WikiDeletedTTL
