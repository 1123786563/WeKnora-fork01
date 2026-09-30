// Pass B 过渡 shim：删除点 ib2（Integration Brief 登记，plan 21-knowledge-ingest §6.4/§7.5）。
// K1（b2-k-ingest）的 chunk HTTP 处理器已随 handler 包 chunk.go/chunker_debug.go
// 搬迁至 internal/modules/knowledge/ingest；本文件为宿主包仍被引用的调用方保留
// 包装类型与转发（spec §13 M3 兼容别名，处理逻辑唯一存在于 ingest 包）。
//
// 与 plan §6.4 嵌入方案的偏差（K1.5 实测修正，报告与 Brief 登记）：嵌入提升会把
// 路由方法调用派发给内嵌 *ingest.ChunkHandler 自有字段，而宿主测试
// knowledge_mutation_admission_test.go:203 以 `&ChunkHandler{service: …}` 具名
// 字段字面量构造后调用 UpdateChunk/RevertChunk（内嵌指针为 nil，派发即
// nil-field panic→500），rbac_lookups_test.go:293-466 同为字段字面量构造——
// 嵌入方案无法同时满足「测试文件零改」与「路由方法行为等价」。故改为逐方法
// 委托：wrapper 保持与搬迁前完全相同的结构形状（service/kgService 两字段），
// 路由方法以当前字段值构造 ingest 实现并转发（每调用一次两字分配，无共享
// 可变状态、无 lazy-init 竞态）；rbac_lookups.go:103/:104/:130/:149 的断言、
// 方法定义与字段访问因此照常编译，行为与搬迁前逐字节一致。ib2 删除批与
// identity 去方法化联动不变（Brief 登记）。
package handler

import (
	"github.com/Tencent/WeKnora/internal/modules/knowledge/ingest"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// ChunkHandler 包装类型：结构形状与搬迁前 handler.ChunkHandler 完全一致
// （service/kgService 两字段），路由方法逐条委托至 ingest 实现。
type ChunkHandler struct {
	service   interfaces.ChunkService
	kgService interfaces.KnowledgeService
}

// NewChunkHandler 创建包装实例（保护 container.go:719 dig Provide）。
func NewChunkHandler(service interfaces.ChunkService, kgService interfaces.KnowledgeService) *ChunkHandler {
	return &ChunkHandler{service: service, kgService: kgService}
}

// delegate 以 wrapper 当前字段值构造 ingest 实现（处理逻辑唯一存在处）。
func (h *ChunkHandler) delegate() *ingest.ChunkHandler {
	return ingest.NewChunkHandler(h.service, h.kgService)
}

// GetChunkByIDOnly 转发（routes_knowledge.go /chunks/by-id/:id）。
func (h *ChunkHandler) GetChunkByIDOnly(c *gin.Context) { h.delegate().GetChunkByIDOnly(c) }

// ListKnowledgeChunks 转发（routes_knowledge.go /chunks/:knowledge_id）。
func (h *ChunkHandler) ListKnowledgeChunks(c *gin.Context) { h.delegate().ListKnowledgeChunks(c) }

// UpdateChunk 转发（routes_knowledge.go PUT /chunks/:knowledge_id/:id）。
func (h *ChunkHandler) UpdateChunk(c *gin.Context) { h.delegate().UpdateChunk(c) }

// ListChunkRevisions 转发（routes_knowledge.go /chunks/:knowledge_id/:id/revisions）。
func (h *ChunkHandler) ListChunkRevisions(c *gin.Context) { h.delegate().ListChunkRevisions(c) }

// RevertChunk 转发（routes_knowledge.go POST /chunks/:knowledge_id/:id/revert）。
func (h *ChunkHandler) RevertChunk(c *gin.Context) { h.delegate().RevertChunk(c) }

// UpsertGeneratedQuestion 转发（routes_knowledge.go PUT /chunks/by-id/:id/questions）。
func (h *ChunkHandler) UpsertGeneratedQuestion(c *gin.Context) {
	h.delegate().UpsertGeneratedQuestion(c)
}

// RegenerateGeneratedQuestions 转发（routes_knowledge.go POST /chunks/by-id/:id/questions/regenerate）。
func (h *ChunkHandler) RegenerateGeneratedQuestions(c *gin.Context) {
	h.delegate().RegenerateGeneratedQuestions(c)
}

// DeleteChunk 转发（routes_knowledge.go DELETE /chunks/:knowledge_id/:id）。
func (h *ChunkHandler) DeleteChunk(c *gin.Context) { h.delegate().DeleteChunk(c) }

// DeleteChunksByKnowledgeID 转发（routes_knowledge.go DELETE /chunks/:knowledge_id）。
func (h *ChunkHandler) DeleteChunksByKnowledgeID(c *gin.Context) {
	h.delegate().DeleteChunksByKnowledgeID(c)
}

// DeleteGeneratedQuestion 转发（routes_knowledge.go DELETE /chunks/by-id/:id/questions）。
func (h *ChunkHandler) DeleteGeneratedQuestion(c *gin.Context) {
	h.delegate().DeleteGeneratedQuestion(c)
}

// PreviewChunking 转发（保护 routes_knowledge.go:19 的 handler.PreviewChunking
// 路由注册行，集成工程师独占文件禁改）；真源唯一在 ingest。
func PreviewChunking(c *gin.Context) { ingest.PreviewChunking(c) }
