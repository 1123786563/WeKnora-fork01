// Pass B (23-knowledge-wikifaq) 过渡 shim — 删除点 ib2（Integration Brief 登记）。
//
// WikiPageHandler 已迁入 internal/modules/knowledge/wiki（wiki_page_handler.go）。
// 本包装类型保 routes_knowledge.go:309 与 container dig 装配的
// handler.NewWikiPageHandler 形参不变；嵌入提升保全部路由方法集（ListPages、
// CreatePage、…）零改写。
//
// 兼容义务（rbac_lookups.go / rbac_lookups_test.go 字节不变）：
//   - rbac_lookups.go:183 在宿主类型上定义方法 KBCreatorLookupFromKBPath，使用
//     本类型的 kbService 字段——Go 规范下与提升方法同名的显式方法为合法遮蔽；
//   - rbac_lookups.go:105 的方法表达式 (*WikiPageHandler)(nil).KBCreatorLookupFromKBPath
//     在宿主 wrapper 上取值，合法；
//   - rbac_lookups_test.go:327/344/356 的字面量 &WikiPageHandler{kbService: …}
//     字段仍在。10-identity.md:79 同型推迟先例；ib2 由集成工程师收口
//     （调用点改写 + 本文件删除）。
package handler

import (
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/modules/knowledge/wiki"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// WikiPageHandler wraps wiki.WikiPageHandler, keeping the host-package name
// alive for route registration and the rbac creator-lookup method defined on
// the host type.
type WikiPageHandler struct {
	*wiki.WikiPageHandler
	kbService interfaces.KnowledgeBaseService
}

// NewWikiPageHandler creates a new wiki page handler.
func NewWikiPageHandler(
	wikiService interfaces.WikiPageService,
	kbService interfaces.KnowledgeBaseService,
	lintService *wiki.WikiLintService,
	auditService interfaces.AuditLogService,
	memoryService interfaces.MemoryService,
) *WikiPageHandler {
	return &WikiPageHandler{
		WikiPageHandler: wiki.NewWikiPageHandler(
			wikiService, kbService, lintService, auditService, memoryService,
			service.RecordWikiContentActivity,
		),
		kbService: kbService,
	}
}
