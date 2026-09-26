package repository

// Pass B 临时测试装置垫片（Ruling 2026-09-24-TEST-SUPPORT-SHIM）。
//
// 留守测试 internal/application/repository/knowledge_tag_test.go（属主 24-knowledge-process，
// 本节点禁改，22-knowledge-retrieval.md §7.2）在 :219 构造、:241 调用随迁未导出类型
// knowledgeTagRepository（tag.go 已物理迁移至 internal/modules/knowledge/retrieval/app/repository）。
// 本垫片提供最小符号定义并委托落位包导出构造器（不复制实现）；kb_activity/tag 语义零变化。
//
// remove_at: ib2（先到先删，最迟 B5）；台账：docs/architecture/evidence/passb/b2-k-retrieval.md
// 「临时测试装置垫片（B5 清理范围）」小节（K2.8 落盘）。

import (
	"context"

	kbretrieval "github.com/Tencent/WeKnora/internal/modules/knowledge/retrieval/app/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

type knowledgeTagRepository struct{ db *gorm.DB }

// BatchCountReferences 委托落位实现（签名与 tag.go:175-180 一致）。
func (r *knowledgeTagRepository) BatchCountReferences(ctx context.Context, tenantID uint64, kbID string, tagIDs []string) (map[string]types.TagReferenceCounts, error) {
	return kbretrieval.NewKnowledgeTagRepository(r.db).BatchCountReferences(ctx, tenantID, kbID, tagIDs)
}
