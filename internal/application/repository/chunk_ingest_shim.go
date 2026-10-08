// Pass B 过渡 shim：删除点 ib2（Integration Brief 登记，plan 21-knowledge-ingest §6.2/§7.5）。
// K1（b2-k-ingest）定义的 chunk 摄取域符号已随 repository/chunk.go 搬迁至
// internal/knowledge/ingest；本文件为宿主包仍被引用的调用方保留
// 无逻辑转发声明（spec §13 M3 兼容别名，真源唯一在 ingest 包）。
package repository

import (
	"github.com/Tencent/WeKnora/internal/knowledge/ingest"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// NewChunkRepository 转发（plan §6.2 R1-8）：保护 container.go:205（dig Provide）
// 与宿主测试 document_write_access_test.go / knowledge_write_access_test.go /
// knowledge_caller_scope_test.go 的 repository.NewChunkRepository 调用点。
func NewChunkRepository(db *gorm.DB) interfaces.ChunkRepository {
	return ingest.NewChunkRepository(db)
}

// ErrChunkRevisionConflict 哨兵转发（R1-8 具体化增补，超出 §6.2 字面清单）：
// service/chunk.go:23 的别名 var 在 K1.2 搬迁删除前仍引用
// repository.ErrChunkRevisionConflict；与 R1-1 同一真源 ingest.ErrChunkRevisionConflict，
// errors.Is 链身份不变（plan §5.3）。
var ErrChunkRevisionConflict = ingest.ErrChunkRevisionConflict

// ErrChunkNotFound 哨兵转发（R1-8 具体化增补，超出 §6.2 字面清单）：
// service/knowledge.go:38（K4 属主，本节点禁改）与宿主测试
// document_write_access_test.go:568 引用 repository.ErrChunkNotFound。
var ErrChunkNotFound = ingest.ErrChunkNotFound
