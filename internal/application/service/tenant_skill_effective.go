package service

import (
	"context"

	acatsvc "github.com/Tencent/WeKnora/internal/modules/agentcatalog/service"
	"github.com/Tencent/WeKnora/internal/types"
)

// —— Pass B 25b 过渡残差（IB2 删除）：conversation 调用点 session_agent_qa.go:357
// 零改动。effectiveTenantSkills 主体随迁导出（EffectiveTenantSkills），本文件
// 只保留 skillsForRun 旧调用形状；skillImageConfigReader 旧名退场。
// remove_at: ib2

func skillsForRun(
	ctx context.Context,
	pinner *SessionSandboxPinner,
	configs acatsvc.SkillImageConfigReader,
	skills installedSkillLister,
	tenantID uint64,
	sessionID string,
	agentConfigID string,
) (string, []*types.TenantSkillEntity) {
	var pinned acatsvc.PinnedConfigReader
	if pinner != nil {
		pinned = pinner
	}
	return acatsvc.SkillsForRun(ctx, pinned, configs, skills, tenantID, sessionID, agentConfigID)
}
