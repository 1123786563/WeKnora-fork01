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

// hostSkillsForRun is skillsForRun on Lite: skills installed on this machine,
// offered only while their files are actually on disk.
func hostSkillsForRun(
	ctx context.Context, skills installedSkillLister, tree HostSkillTree, tenantID uint64,
) (string, []*types.TenantSkillEntity) {
	if skills == nil || tree == nil || tenantID == 0 {
		return sandbox.HostSkillTargetID, nil
	}
	rows, err := skills.ListSkillsByConfig(ctx, tenantID, sandbox.HostSkillTargetID)
	if err != nil {
		logger.Warnf(ctx, "[skill] list local skills failed: %v", err)
		return sandbox.HostSkillTargetID, nil
	}
	usable := make([]*types.TenantSkillEntity, 0, len(rows))
	for _, row := range rows {
		if row == nil || !row.Enabled || !tree.Installed(row.Name) {
			continue
		}
		if served := row.ServedView(); served != nil {
			usable = append(usable, served)
		}
	}
	if len(usable) == 0 {
		return sandbox.HostSkillTargetID, nil
	}
	return sandbox.HostSkillTargetID, usable
}
