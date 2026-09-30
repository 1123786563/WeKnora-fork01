// Pass B (23-knowledge-wikifaq) 过渡 shim — 删除点 ib2（Integration Brief 登记）。
// B0.3 Step 3 去方法化裁定（conventions §7.4 第 4 条）：resolveWikiFixerTenantScope
// 原定义于 handler/session/wiki_fixer_scope.go:18（conversation 属主 *Handler 方法），
// 核心逻辑已随包级函数迁入 wiki.ResolveBuiltinWikiFixerTenantScope。本方法为
// qa.go:205（conversation 属主，不改）保编译；ib2 由集成工程师改写调用点
// 直调 wiki.ResolveBuiltinWikiFixerTenantScope 并删除本文件（10-identity.md:79
// 同型推迟先例）。
package session

import (
	"context"

	"github.com/Tencent/WeKnora/internal/modules/knowledge/wiki"
	"github.com/Tencent/WeKnora/internal/types"
)

func (h *Handler) resolveWikiFixerTenantScope(
	ctx context.Context,
	agent *types.CustomAgent,
	currentTenantID uint64,
	callerTenantRole types.TenantRole,
	kbIDs []string,
) (*types.CustomAgent, uint64) {
	return wiki.ResolveBuiltinWikiFixerTenantScope(
		ctx,
		agent,
		currentTenantID,
		callerTenantRole,
		kbIDs,
		h.knowledgebaseService,
		h.kbShareService,
	)
}
