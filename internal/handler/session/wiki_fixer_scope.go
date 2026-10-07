package session

import (
	"context"

	"github.com/Tencent/WeKnora/internal/application/access"
	"github.com/Tencent/WeKnora/internal/logger"
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
// source tenant. The package-level form (with injected lookup/share ports)
// was introduced by Pass B 23-knowledge-wikifaq (B0.3 Step 3 去方法化)；
// order 60 handler 批迁回 session 包后保留该形态供 wiki_fixer_scope_test
// 注入 stub 白盒验证，conversation 属主方法经下方方法转发复用同一实现。
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
	// The run now executes in the KB owner's workspace, where every ID and
	// selection mode in the config resolves. The caller's own customizations
	// (an "all" KB or MCP scope, skills, sandbox, models) would reach the
	// owner's other resources, so start from the built-in defaults and pin
	// the config to this one KB. Models fall back to the KB's own.
	if builtin := types.GetBuiltinAgent(types.BuiltinWikiFixerID, kb.TenantID); builtin != nil {
		scopedAgent.Config = builtin.Config
	}
	scopedAgent.Config.KBSelectionMode = "selected"
	scopedAgent.Config.KnowledgeBases = []string{kb.ID}
	scopedAgent.Config.MCPSelectionMode = "none"
	scopedAgent.Config.MCPServices = nil
	scopedAgent.Config.SkillsSelectionMode = "none"
	scopedAgent.Config.SelectedSkills = nil
	scopedAgent.Config.SandboxConfigID = ""
	scopedAgent.Config.WebSearchEnabled = false
	scopedAgent.Config.ModelID = ""
	scopedAgent.Config.RerankModelID = ""
	scopedAgent.Config.VLMModelID = ""
	logger.Infof(ctx, "wiki fixer: using shared KB source tenant %d for KB %s", kb.TenantID, secutils.SanitizeForLog(kb.ID))
	return &scopedAgent, kb.TenantID
}

// resolveWikiFixerTenantScope binds the package-level implementation to the
// conversation Handler's own KB services（qa.go 属主调用面；方法形态还原自
// Pass B 前的原定义 internal/handler/session/wiki_fixer_scope.go:18）。
func (h *Handler) resolveWikiFixerTenantScope(
	ctx context.Context,
	agent *types.CustomAgent,
	currentTenantID uint64,
	callerTenantRole types.TenantRole,
	kbIDs []string,
) (*types.CustomAgent, uint64) {
	return ResolveBuiltinWikiFixerTenantScope(
		ctx,
		agent,
		currentTenantID,
		callerTenantRole,
		kbIDs,
		h.knowledgebaseService,
		h.kbShareService,
	)
}
