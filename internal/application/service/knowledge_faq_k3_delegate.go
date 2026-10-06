// Pass B (23-knowledge-wikifaq) 过渡 shim — 删除点 ib2（Integration Brief 登记）。
//
// D1：FAQ 域五文件 + handler/faq.go 已迁入 internal/modules/knowledge/faq
// （K3.2）。本文件在宿主 *knowledgeService 上保留全部 FAQ 面方法，逐条一行
// 委托到 faq.Service：
//
//	(a) interfaces.KnowledgeService 冻结接口 FAQ 面 15 方法（计划 §4 表 14 +
//	    编译器枚举补齐的 UpdateLastFAQImportResultDisplayStatus，报告登记）；
//	(b) 宿主他 owner（K2 knowledgebase_access.go:42、K4 knowledge_clone_move.go
//	    :700/:855/:885）调用的 4 个未导出方法；
//	(c) faqSvc()：以 knowledgeService 实际字段构造 faq.Service，并把 faq 侧
//	    seam 接到宿主现行符号（与迁移前同包直引完全相同的目标实现）。
//
// 行为零变化：seam 闭包捕获本 receiver；memFAQProgress/memFAQRunningImport
// 以指针共享，FAQ 导入进度状态跨 faqSvc() 重构造不丢；接口满足性由编译器
// 强制（冻结端口零改动）。knowledgeService 结构体零改动（K4 knowledge.go
// 未触碰）。
package service

import (
	"context"

	"github.com/Tencent/WeKnora/internal/models/embedding"
	"github.com/Tencent/WeKnora/internal/modules/knowledge/faq"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/hibiken/asynq"
)

// ---- (c) faqSvc：faq.Service 构造 + seam 接线 ----

// faqSvc 构造 faq.Service。每次调用重建（knowledgeService 结构体不可改，
// 无缓存字段位）；两个内存回落表以指针共享保证状态连续。
func (s *knowledgeService) faqSvc() *faq.Service {
	return faq.NewService(faq.Deps{
		Repo:                s.repo,
		ChunkRepo:           s.chunkRepo,
		ChunkService:        s.chunkService,
		TagRepo:             s.tagRepo,
		TagService:          s.tagService,
		TenantRepo:          s.tenantRepo,
		KBService:           s.kbService,
		KBShareService:      s.kbShareService,
		FileSvc:             s.fileSvc,
		ModelService:        s.modelService,
		Task:                s.task,
		Audit:               s.audit,
		RedisClient:         s.redisClient,
		RetrieveEngine:      s.retrieveEngine,
		Ownership:           s.ownership,
		MemFAQProgress:      &s.memFAQProgress,
		MemFAQRunningImport: &s.memFAQRunningImport,
		Seams: faq.Seams{
			RecordKBActivity:             recordKBActivity,
			KBActivityTrigger:            kbActivityTrigger,
			WithKBActivityTask:           withKBActivityTask,
			KBActivityAppendSampleTitles: kbActivityAppendSampleTitles,
			ResolveKBReadTenant: func(
				ctx context.Context,
				kb *types.KnowledgeBase,
				shares faq.KBShareLookup,
			) (uint64, error) {
				return resolveKBReadTenant(ctx, kb, shares)
			},
			WritableFAQKnowledgeBase: func(
				ctx context.Context,
				kbID string,
			) (*types.KnowledgeBase, context.Context, error) {
				return s.writableFAQKnowledgeBase(ctx, kbID)
			},
		},
	})
}

// ---- (a) 冻结接口 FAQ 面（interfaces.KnowledgeService，contracts.yaml knowledge.service）----

func (s *knowledgeService) ListFAQEntries(ctx context.Context,
	kbID string, page *types.Pagination, tagUUIDs []string, legacyTagSeqID int64, keyword string, searchField string, sortOrder string,
	isEnabled *bool,
) (*types.PageResult, error) {
	return s.faqSvc().ListFAQEntries(ctx, kbID, page, tagUUIDs, legacyTagSeqID, keyword, searchField, sortOrder, isEnabled)
}

func (s *knowledgeService) UpsertFAQEntries(ctx context.Context,
	kbID string, payload *types.FAQBatchUpsertPayload,
) (string, error) {
	return s.faqSvc().UpsertFAQEntries(ctx, kbID, payload)
}

func (s *knowledgeService) CreateFAQEntry(ctx context.Context,
	kbID string, payload *types.FAQEntryPayload,
) (*types.FAQEntry, error) {
	return s.faqSvc().CreateFAQEntry(ctx, kbID, payload)
}

func (s *knowledgeService) GetFAQEntry(ctx context.Context,
	kbID string, entrySeqID int64,
) (*types.FAQEntry, error) {
	return s.faqSvc().GetFAQEntry(ctx, kbID, entrySeqID)
}

func (s *knowledgeService) UpdateFAQEntry(ctx context.Context,
	kbID string, entrySeqID int64, payload *types.FAQEntryPayload,
) (*types.FAQEntry, error) {
	return s.faqSvc().UpdateFAQEntry(ctx, kbID, entrySeqID, payload)
}

func (s *knowledgeService) AddSimilarQuestions(ctx context.Context,
	kbID string, entrySeqID int64, questions []string,
) (*types.FAQEntry, error) {
	return s.faqSvc().AddSimilarQuestions(ctx, kbID, entrySeqID, questions)
}

func (s *knowledgeService) UpdateFAQEntryFieldsBatch(ctx context.Context,
	kbID string, req *types.FAQEntryFieldsBatchUpdate,
) error {
	return s.faqSvc().UpdateFAQEntryFieldsBatch(ctx, kbID, req)
}

func (s *knowledgeService) DeleteFAQEntries(ctx context.Context,
	kbID string, entrySeqIDs []int64,
) error {
	return s.faqSvc().DeleteFAQEntries(ctx, kbID, entrySeqIDs)
}

func (s *knowledgeService) SearchFAQEntries(ctx context.Context,
	kbID string, req *types.FAQSearchRequest,
) ([]*types.FAQEntry, error) {
	return s.faqSvc().SearchFAQEntries(ctx, kbID, req)
}

func (s *knowledgeService) ExportFAQEntries(ctx context.Context, kbID string) ([]byte, error) {
	return s.faqSvc().ExportFAQEntries(ctx, kbID)
}

func (s *knowledgeService) ExportFAQEntriesJSON(ctx context.Context, kbID string) ([]byte, error) {
	return s.faqSvc().ExportFAQEntriesJSON(ctx, kbID)
}

func (s *knowledgeService) UpdateFAQEntryTagBatch(ctx context.Context, kbID string, updates map[int64]*int64) error {
	return s.faqSvc().UpdateFAQEntryTagBatch(ctx, kbID, updates)
}

func (s *knowledgeService) ProcessFAQImport(ctx context.Context, t *asynq.Task) error {
	return s.faqSvc().ProcessFAQImport(ctx, t)
}

func (s *knowledgeService) GetFAQImportProgress(ctx context.Context, taskID string) (*types.FAQImportProgress, error) {
	return s.faqSvc().GetFAQImportProgress(ctx, taskID)
}

func (s *knowledgeService) UpdateLastFAQImportResultDisplayStatus(ctx context.Context, kbID string, displayStatus string) error {
	return s.faqSvc().UpdateLastFAQImportResultDisplayStatus(ctx, kbID, displayStatus)
}

// ---- (b) 宿主他 owner 调用的未导出方法（K2/K4 留驻文件编译依赖）----

// validateFAQKnowledgeBase 供 K2 knowledgebase_access.go:42（writableFAQKnowledgeBase）
// 调用；faqq 包侧已 R1 导出为 ValidateFAQKnowledgeBase。
func (s *knowledgeService) validateFAQKnowledgeBase(ctx context.Context, kbID string) (*types.KnowledgeBase, error) {
	return s.faqSvc().ValidateFAQKnowledgeBase(ctx, kbID)
}

// buildFAQStatusSyncPlan 供 K4 knowledge_clone_move.go:700 调用。
func (s *knowledgeService) buildFAQStatusSyncPlan(
	ctx context.Context,
	srcTenantID, dstTenantID uint64,
	matched []types.FAQChunkSyncPair,
	resolveTag func(srcTagID string) string,
) (*faq.FAQStatusSyncPlan, error) {
	return s.faqSvc().BuildFAQStatusSyncPlan(ctx, srcTenantID, dstTenantID, matched, resolveTag)
}

// indexFAQChunks 供 K4 knowledge_clone_move.go:855 调用。
func (s *knowledgeService) indexFAQChunks(ctx context.Context,
	kb *types.KnowledgeBase, knowledge *types.Knowledge,
	chunks []*types.Chunk, embeddingModel embedding.Embedder,
	adjustStorage bool, needDelete bool,
) error {
	return s.faqSvc().IndexFAQChunks(ctx, kb, knowledge, chunks, embeddingModel, adjustStorage, needDelete)
}

// syncFAQChunkStatusBatch 供 K4 knowledge_clone_move.go:885 调用。
func (s *knowledgeService) syncFAQChunkStatusBatch(
	ctx context.Context,
	dstKB *types.KnowledgeBase,
	pairs []types.FAQChunkSyncPair,
	srcByID, dstByID map[string]*types.FAQChunkStatus,
	resolveTag func(srcTagID string) string,
) error {
	return s.faqSvc().SyncFAQChunkStatusBatch(ctx, dstKB, pairs, srcByID, dstByID, resolveTag)
}
