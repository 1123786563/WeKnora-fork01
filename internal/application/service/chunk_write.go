package service

import (
	"context"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
)

// guard 返回写授权 seam；零值构造（宿主测试夹具以字面量构造 chunkService）
// 回退宿主提供器 hostKnowledgeWriteGuard，与搬迁前测试垫片
// （chunk_ingest_test_shim，Ruling 2026-09-24-TEST-SUPPORT-SHIM）的惰性委托
// 语义一致；生产装配始终显式注入，不触达回退分支。
func (s *chunkService) guard() KnowledgeWriteGuard {
	if s.writeGuard == nil {
		return hostKnowledgeWriteGuard{}
	}
	return s.writeGuard
}

func (s *chunkService) writableChunk(ctx context.Context, id string) (*types.Chunk, error) {
	tenant, err := s.guard().WriteExecutionTenant(ctx)
	if err != nil {
		return nil, err
	}
	chunk, err := s.chunkRepository.GetChunkByID(ctx, tenant, id)
	if err != nil {
		return nil, err
	}
	if chunk == nil || chunk.ID != id || chunk.TenantID != tenant {
		return nil, apperrors.NewNotFoundError("chunk not found")
	}
	knowledge, _, err := s.guard().LoadKnowledgeWrite(ctx, s.knowledgeRepo, s.kbRepository, chunk.KnowledgeID)
	if err != nil {
		return nil, err
	}
	if chunk.KnowledgeBaseID != knowledge.KnowledgeBaseID {
		return nil, apperrors.NewForbiddenError("chunk does not belong to its knowledge base")
	}
	copyOfChunk := *chunk
	return &copyOfChunk, nil
}

// SameChunkDocument 判定两个 chunk 是否属于同一租户下的同一文档。
// Pass B K1.2 导出（R1 增量）：宿主 knowledge_process.go:2256/:3002（K4 属主，
// 禁改）经宿主薄 shim 消费同一实现，真源唯一在 ingest（plan 21 §6.2/§7.5）。
func SameChunkDocument(a, b *types.Chunk) bool {
	return a != nil && b != nil && a.TenantID == b.TenantID &&
		a.KnowledgeBaseID == b.KnowledgeBaseID && a.KnowledgeID == b.KnowledgeID
}

// Batch validation is completed before calling the repository. Supplied full
// rows cannot reparent an existing chunk to a different authorized document.
func (s *chunkService) validateChunkWrites(ctx context.Context, chunks []*types.Chunk, create bool) error {
	ids := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk == nil || chunk.ID == "" {
			return apperrors.NewBadRequestError("chunk ID cannot be empty")
		}
		ids = append(ids, chunk.KnowledgeID)
	}
	knowledge, err := s.guard().LoadKnowledgeWriteBatch(ctx, s.knowledgeRepo, s.kbRepository, ids)
	if err != nil {
		return err
	}
	parents := make(map[string]*types.Knowledge, len(knowledge))
	for _, row := range knowledge {
		parents[row.ID] = row
	}
	storedByID := make(map[string]*types.Chunk)
	if !create && len(chunks) > 0 {
		chunkIDs := make([]string, 0, len(chunks))
		for _, chunk := range chunks {
			chunkIDs = append(chunkIDs, chunk.ID)
		}
		stored, err := s.chunkRepository.ListChunksByID(ctx, knowledge[0].TenantID, chunkIDs)
		if err != nil {
			return err
		}
		for _, chunk := range stored {
			if chunk != nil {
				storedByID[chunk.ID] = chunk
			}
		}
	}
	for _, chunk := range chunks {
		parent := parents[chunk.KnowledgeID]
		if parent == nil || parent.TenantID != chunk.TenantID || parent.KnowledgeBaseID != chunk.KnowledgeBaseID {
			return apperrors.NewForbiddenError("chunk does not belong to its knowledge document")
		}
		if !create {
			stored := storedByID[chunk.ID]
			if stored == nil || stored.ID != chunk.ID || !SameChunkDocument(stored, chunk) {
				return apperrors.NewForbiddenError("chunk ownership cannot be changed")
			}
		}
	}
	return nil
}

func (s *chunkService) writableChunkIDs(ctx context.Context, ids []string) ([]string, error) {
	ids, err := s.guard().WriteResourceIDs(ids)
	if err != nil || len(ids) == 0 {
		return ids, err
	}
	tenant, err := s.guard().WriteExecutionTenant(ctx)
	if err != nil {
		return nil, err
	}
	chunks, err := s.chunkRepository.ListChunksByID(ctx, tenant, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*types.Chunk, len(chunks))
	for _, chunk := range chunks {
		if chunk != nil {
			byID[chunk.ID] = chunk
		}
	}
	ordered := make([]*types.Chunk, 0, len(ids))
	for _, id := range ids {
		if byID[id] == nil {
			return nil, apperrors.NewNotFoundError("chunk not found")
		}
		ordered = append(ordered, byID[id])
	}
	// Rows were loaded from storage already; only their parent grants remain.
	return ids, s.validateChunkWrites(ctx, ordered, true)
}

// Editing a text chunk may also rewrite its parent and image children. Validate
// those persisted relationships before saving the first revision.
func (s *chunkService) validateDocumentChunkRelations(ctx context.Context, chunk *types.Chunk) error {
	parents := []*types.Chunk{chunk}
	if chunk.ParentChunkID != "" {
		parent, err := s.chunkRepository.GetChunkByID(ctx, chunk.TenantID, chunk.ParentChunkID)
		if err != nil {
			return err
		}
		if parent == nil || parent.ID != chunk.ParentChunkID || !SameChunkDocument(chunk, parent) {
			return apperrors.NewForbiddenError("parent chunk does not belong to its document")
		}
		parents = append(parents, parent)
	}
	for _, parent := range parents {
		children, err := s.chunkRepository.ListChunkByParentID(ctx, chunk.TenantID, parent.ID)
		if err != nil {
			return err
		}
		for _, child := range children {
			if !SameChunkDocument(chunk, child) || child.ParentChunkID != parent.ID {
				return apperrors.NewForbiddenError("child chunk does not belong to its document")
			}
		}
	}
	return nil
}
