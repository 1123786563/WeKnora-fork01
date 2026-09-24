// Ruling 2026-09-24-TEST-SUPPORT-SHIM（conventions §10.3）
// remove_at: ib2（IB2 先到先删，最迟 B5；追踪：exception-ledger.yaml「临时测试装置
// 垫片台账」注释段 + Integration Brief docs/architecture/passb/briefs/b2-k-ingest.md §1）
//
// 孤儿测试装置垫片：document_write_access_test.go（K4 属主，本节点禁改）以
// `&chunkService{chunkRepository: …, knowledgeRepo: …, kbRepository: …}` 构造
// 真实 chunk 写路径夹具；service/chunk.go + chunk_write.go 随 K1.2 搬迁至
// internal/modules/knowledge/ingest 后，宿主包内 chunkService 未导出类型断链。
// 本垫片提供最小符号定义：同名字段三件 + 惰性委托到 ingest 真实现（经
// KnowledgeWriteGuardProvider 提供真实 K4 写授权语义，测试特征化不断链）。
// ib2 路由/装配切模块门面后，document_write_access_test.go 随 K4 域改写并删除本垫片。
package service

import (
	"context"

	"github.com/Tencent/WeKnora/internal/modules/knowledge/ingest"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// chunkService 是 ingest 包真实 chunkService 的宿主测试垫片（Ruling 3 最小符号定义）。
type chunkService struct {
	chunkRepository interfaces.ChunkRepository
	knowledgeRepo   interfaces.KnowledgeRepository
	kbRepository    interfaces.KnowledgeBaseRepository
	impl            interfaces.ChunkService
}

// delegate 惰性构造 ingest 真实现；写授权走 KnowledgeWriteGuardProvider（真实
// K4 语义），span/summary/indexContent 依赖按夹具场景保持零值语义（垫片内
// chunk_service 的生产调用点对 nil 闭包已做 nil 回退）。
func (s *chunkService) delegate() interfaces.ChunkService {
	if s.impl == nil {
		s.impl = ingest.NewChunkService(
			s.chunkRepository,
			s.knowledgeRepo,
			s.kbRepository,
			nil,
			nil,
			nil,
			nil,
			KnowledgeWriteGuardProvider(),
			nil,
			nil,
			BuildKnowledgeIndexContentProvider(),
		)
	}
	return s.impl
}

func (s *chunkService) CreateChunks(ctx context.Context, chunks []*types.Chunk) error {
	return s.delegate().CreateChunks(ctx, chunks)
}

func (s *chunkService) GetChunkByID(ctx context.Context, id string) (*types.Chunk, error) {
	return s.delegate().GetChunkByID(ctx, id)
}

func (s *chunkService) GetChunkByIDOnly(ctx context.Context, id string) (*types.Chunk, error) {
	return s.delegate().GetChunkByIDOnly(ctx, id)
}

func (s *chunkService) ListChunksByKnowledgeID(ctx context.Context, knowledgeID string) ([]*types.Chunk, error) {
	return s.delegate().ListChunksByKnowledgeID(ctx, knowledgeID)
}

func (s *chunkService) ListPagedChunksByKnowledgeID(
	ctx context.Context, knowledgeID string, page *types.Pagination, chunkType []types.ChunkType,
) (*types.PageResult, error) {
	return s.delegate().ListPagedChunksByKnowledgeID(ctx, knowledgeID, page, chunkType)
}

func (s *chunkService) UpdateChunk(ctx context.Context, chunk *types.Chunk) error {
	return s.delegate().UpdateChunk(ctx, chunk)
}

func (s *chunkService) UpdateChunks(ctx context.Context, chunks []*types.Chunk) error {
	return s.delegate().UpdateChunks(ctx, chunks)
}

func (s *chunkService) DeleteChunk(ctx context.Context, id string) error {
	return s.delegate().DeleteChunk(ctx, id)
}

func (s *chunkService) DeleteChunks(ctx context.Context, ids []string) error {
	return s.delegate().DeleteChunks(ctx, ids)
}

func (s *chunkService) DeleteChunksByKnowledgeID(ctx context.Context, knowledgeID string) error {
	return s.delegate().DeleteChunksByKnowledgeID(ctx, knowledgeID)
}

func (s *chunkService) DeleteByKnowledgeList(ctx context.Context, ids []string) error {
	return s.delegate().DeleteByKnowledgeList(ctx, ids)
}

func (s *chunkService) ListChunkByParentID(ctx context.Context, tenantID uint64, parentID string) ([]*types.Chunk, error) {
	return s.delegate().ListChunkByParentID(ctx, tenantID, parentID)
}

func (s *chunkService) GetRepository() interfaces.ChunkRepository {
	return s.delegate().GetRepository()
}

func (s *chunkService) DeleteGeneratedQuestion(ctx context.Context, chunkID string, questionID string) error {
	return s.delegate().DeleteGeneratedQuestion(ctx, chunkID, questionID)
}

func (s *chunkService) UpdateDocumentChunk(
	ctx context.Context, chunkID string, content *string, isEnabled *bool, expectedRevision *int,
) (*types.Chunk, error) {
	return s.delegate().UpdateDocumentChunk(ctx, chunkID, content, isEnabled, expectedRevision)
}

func (s *chunkService) ListChunkRevisions(ctx context.Context, chunkID string) ([]*types.ChunkRevision, error) {
	return s.delegate().ListChunkRevisions(ctx, chunkID)
}

func (s *chunkService) RevertDocumentChunk(
	ctx context.Context, chunkID string, revision int, expectedRevision *int,
) (*types.Chunk, error) {
	return s.delegate().RevertDocumentChunk(ctx, chunkID, revision, expectedRevision)
}

func (s *chunkService) UpsertGeneratedQuestion(
	ctx context.Context, chunkID string, questionID string, question string,
) (*types.GeneratedQuestion, error) {
	return s.delegate().UpsertGeneratedQuestion(ctx, chunkID, questionID, question)
}
