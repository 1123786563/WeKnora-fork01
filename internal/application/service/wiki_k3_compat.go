// Pass B (23-knowledge-wikifaq) 过渡 shim — 删除点 ib2（Integration Brief 登记）。
//
// wiki 域 12 文件随上游对齐 round 2 自 internal/knowledge/wiki 归位
// 本包（真源现为 wiki_ingest.go 等）。本文件仅为留驻宿主调用点（K1
// extract/image_multimodal、K4 knowledge_*、router/task.go、
// recover_pending_wiki_tasks.go 及留驻宿主测试）保留未导出调用面的同名
// 薄包装；原 wiki.X 别名/转发（ErrWikiIngestConcurrent、WikiIngestPayload、
// EnqueueWikiIngest 等）已随包合并删除——真源即本包同名符号。
package service

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// --- 原未导出符号：保 K1/K4 同包裸调用点编译 ---

func previewText(s string, maxRunes int) string { return PreviewText(s, maxRunes) }

func enqueueWikiIngestTrigger(
	ctx context.Context,
	task interfaces.TaskEnqueuer,
	tenantID uint64,
	kbID string,
) error {
	return EnqueueWikiIngestTrigger(ctx, task, tenantID, kbID)
}

func enqueueWikiRetract(
	ctx context.Context,
	task interfaces.TaskEnqueuer,
	pendingRepo interfaces.TaskPendingOpsRepository,
	payload WikiRetractPayload,
) error {
	return EnqueueWikiRetractWithError(ctx, task, pendingRepo, payload)
}

func extractRealText(content string) string { return ExtractRealText(content) }

func newWikiIngestPendingOp(
	ctx context.Context,
	tenantID uint64,
	kbID, knowledgeID string,
) (*types.TaskPendingOp, error) {
	return NewWikiIngestPendingOp(ctx, tenantID, kbID, knowledgeID)
}

// minTextContentRunes 与 MinTextContentRunes 初始化值相同（10）。
// 生产代码从不改写两者；宿主侧测试（knowledge_summary_test.go）改写的是
// 本副本，其断言的 K4 读点（checkSufficientSummaryContent）消费
// MinTextContentRunes——包合并后两副本指向各自 var，行为与迁移前一致。
var minTextContentRunes = MinTextContentRunes

func realTextRuneCount(content string) int { return RealTextRuneCount(content) }

func uniqueWikiFolderIDs(values []string) []string { return UniqueWikiFolderIDs(values) }

// wikiDeletedTTL 照录 WikiDeletedTTL 常量值（knowledge_delete.go:178 写墓碑
// 键 TTL 用）。包合并后真源为 WikiDeletedTTL。
const wikiDeletedTTL = WikiDeletedTTL
