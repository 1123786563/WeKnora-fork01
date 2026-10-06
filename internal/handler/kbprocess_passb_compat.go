package handler

// Pass B 宿主兼容层（b2-k-process / K4.3）：handler 层 2 文件
// （kb_access.go、task_progress_auth.go）已物理迁移至
// internal/modules/knowledge/process/handler（docs/plans/passb/
// 24-knowledge-process.md Task K4.3；Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP
// 同 commit 删行）。
// 本文件为留守宿主消费方（knowledge.go/knowledgebase.go 的
// requireTaskProgressTenant 与 kb_access 4 helper 调用点、
// knowledge_download.go:68、K3 faq_k3_compat.go:30 构造参直引、
// kb_access_test.go 白盒）提供同形一行委托，调用点零改动。
// 删除点：ib2 集成屏障直连后（Integration Brief 指令），随 manifest compat
// 登记行一并删除（Ruling 2026-09-25-TRANSITION-SHIM-ROW-REGISTRATION）。

import (
	"context"

	"github.com/Tencent/WeKnora/internal/application/access"
	"github.com/Tencent/WeKnora/internal/middleware"
	processhandler "github.com/Tencent/WeKnora/internal/modules/knowledge/process/handler"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// requireTaskProgressTenant 委托 processhandler.RequireTaskProgressTenant
// （task_progress_auth.go:14 原定义，K4.3 导出；K3 faq_k3_compat.go 构造参
// seam 与留守 knowledge.go/knowledgebase.go 调用点保护）。
func requireTaskProgressTenant(ctx context.Context, taskID string) error {
	return processhandler.RequireTaskProgressTenant(ctx, taskID)
}

// resolvedKBAccess 委托 processhandler.ResolvedKBAccess（kb_access.go:18 原定义，
// K4.3 导出；留守 knowledge.go:148 消费面保护）。
func resolvedKBAccess(c *gin.Context, kbID string, required types.OrgMemberRole) (*access.KBAccess, bool) {
	return processhandler.ResolvedKBAccess(c, kbID, required)
}

// resolveHandlerKBAccess 委托 processhandler.ResolveHandlerKBAccess
// （kb_access.go:28 原定义；留守 knowledge.go:100、knowledgebase.go:457、
// kb_access_test.go:132/:153/:222 消费面保护）。
func resolveHandlerKBAccess(c *gin.Context, kbID string, kbService middleware.KBLookup,
	shares interfaces.KBShareService, agents interfaces.AgentShareService,
) (*access.KBAccess, error) {
	return processhandler.ResolveHandlerKBAccess(c, kbID, kbService, shares, agents)
}

// resolveHandlerKBAccessFor 委托 processhandler.ResolveHandlerKBAccessFor
// （kb_access.go:34 原定义；留守 knowledge.go:111/:2385/:2389、
// knowledge_download.go:68、knowledgebase.go:883/:907 消费面保护）。
func resolveHandlerKBAccessFor(c *gin.Context, kbID string, kbService middleware.KBLookup,
	shares interfaces.KBShareService, agents interfaces.AgentShareService, required types.OrgMemberRole,
) (*access.KBAccess, error) {
	return processhandler.ResolveHandlerKBAccessFor(c, kbID, kbService, shares, agents, required)
}

// kbAccessHTTPError 委托 processhandler.KBAccessHTTPError（kb_access.go:68 原定义；
// 留守 knowledge.go:123/:160/:2396、knowledgebase.go:941 消费面保护）。
func kbAccessHTTPError(err error) error {
	return processhandler.KBAccessHTTPError(err)
}
