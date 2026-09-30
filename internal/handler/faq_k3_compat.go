// Pass B (23-knowledge-wikifaq) 过渡 shim — 删除点 ib2（Integration Brief 登记）。
//
// H2：FAQHandler 已迁入 internal/modules/knowledge/faq（faq_handler.go）。
// 本文件以类型别名保 routes_knowledge.go:143 的 *handler.FAQHandler 形参与
// container.go:725 的 handler.NewFAQHandler dig Provide 形参不变，并把 faq
// 侧对宿主 handler 包两个 helper 的 seam（parseCommaSeparatedTagIDs /
// requireTaskProgressTenant，见 faq_handler.go 文件头）接到宿主现行实现——
// 与迁移前同包直引完全相同的目标函数，行为等价。
package handler

import (
	"github.com/Tencent/WeKnora/internal/modules/knowledge/faq"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// FAQHandler aliases faq.FAQHandler so host route registration and dig wiring
// keep compiling unchanged（routes_knowledge.go:143、router 测试的
// &FAQHandler{} 零字段字面量均合法）。
type FAQHandler = faq.FAQHandler

// NewFAQHandler creates a new FAQ handler.
func NewFAQHandler(
	knowledgeService interfaces.KnowledgeService,
	kbService interfaces.KnowledgeBaseService,
) *FAQHandler {
	return faq.NewFAQHandler(
		knowledgeService,
		kbService,
		parseCommaSeparatedTagIDs,
		requireTaskProgressTenant,
	)
}
