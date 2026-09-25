package wiki

import (
	"context"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/policy/access"
	"github.com/Tencent/WeKnora/internal/types"
	secutils "github.com/Tencent/WeKnora/internal/utils"
)

// wikiFixerKBLookup is the minimal KB lookup port ResolveBuiltinWikiFixerTenantScope
// needs; satisfied structurally by interfaces.KnowledgeBaseService.
type wikiFixerKBLookup interface {
	GetKnowledgeBaseByIDOnly(ctx context.Context, id string) (*types.KnowledgeBase, error)
}

type wikiFixerKBSharePermission = access.KBShareLookup

// ResolveBuiltinWikiFixerTenantScope keeps shared-KB wiki-fixer runs on the
// source tenant. Exported by Pass B 23-knowledge-wikifaq (B0.3 Step 3
// 去方法化裁定)；原先定义于 internal/handler/session/wiki_fixer_scope.go。
func ResolveBuiltinWikiFixerTenantScope(
	ctx context.Context,
	agent *types.CustomAgent,
	currentTenantID uint64,
	callerTenantRole types.TenantRole,
	kbIDs []string,
	kbLookup wikiFixerKBLookup,
	kbShare wikiFixerKBSharePermission,
) (*types.CustomAgent, uint64) {
	if agent == nil || agent.ID != types.BuiltinWikiFixerID {
		return agent, 0
	}
	if currentTenantID == 0 || len(kbIDs) != 1 || kbLookup == nil || kbShare == nil {
		return agent, 0
	}

	kbID := kbIDs[0]
	kb, err := kbLookup.GetKnowledgeBaseByIDOnly(ctx, kbID)
	if err != nil {
		logger.Warnf(ctx, "wiki fixer: failed to resolve KB %s for shared scope: %v", secutils.SanitizeForLog(kbID), err)
		return agent, 0
	}
	if kb == nil {
		logger.Warnf(ctx, "wiki fixer: KB %s not found for shared scope", secutils.SanitizeForLog(kbID))
		return agent, 0
	}
	if kb.TenantID == 0 || kb.TenantID == currentTenantID {
		return agent, 0
	}

	permissions := access.NewKBSharePermissions(ctx, kbShare, currentTenantID, callerTenantRole)
	allowed, err := permissions.Check(kb.ID, types.OrgRoleEditor)
	if err != nil {
		logger.Warnf(ctx, "wiki fixer: failed to check shared KB %s permission: %v", secutils.SanitizeForLog(kb.ID), err)
		return agent, 0
	}
	if !allowed {
		return agent, 0
	}

	scopedAgent := *agent
	scopedAgent.TenantID = kb.TenantID
	logger.Infof(ctx, "wiki fixer: using shared KB source tenant %d for KB %s", kb.TenantID, secutils.SanitizeForLog(kb.ID))
	return &scopedAgent, kb.TenantID
}
