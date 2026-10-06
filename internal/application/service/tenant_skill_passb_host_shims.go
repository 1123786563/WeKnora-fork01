package service

import (
	"context"

	"github.com/Tencent/WeKnora/internal/agent/skills"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
)

// tenant_skill_passb_host_shims.go —— Pass B 25b 宿主小写名转发面（b2-ac-skills）。
// 25b 实现已归位本包（tenant_skill_*.go 真身，导出面）；本文件仅为
// 宿主禁改消费方（tenant_sandbox_config.go / user_env.go / agent_service.go /
// skill_market_service.go / expert_skills.go / agent_service_*_test.go）保留
// 旧未导出名的裸引用兼容，逐名 1:1 委托同包真身导出名。remove_at: ib2
// （宿主消费方改用导出名后删除；计划 §2.5 / §T3）。

func init() { // 替代随迁走的 env_declare.go init()（计划 §4.4-1），行为等价
	RegisterReservedEnvNames(skills.InjectedSandboxEnvVars())
	// 25b 补充接缝（计划 §4.4-1 同类先例）：adapters-free 的 ParseSkillBundle
	// 入口（25a/25c 禁改测试与本包消费方经此调用）需要 SKILL.md 解析能力。
	RegisterBundleParsers(parseSkillManifest, skills.UnmarshalSkillFrontmatter)
}

// parseSkillManifest 1:1 包装 skills.ParseSkillFile 的 4 字段消费面。
func parseSkillManifest(content string) (SkillManifestView, error) {
	skill, err := skills.ParseSkillFile(content)
	if err != nil {
		return SkillManifestView{}, err
	}
	return SkillManifestView{Name: skill.Name, Description: skill.Description,
		Instructions: skill.Instructions, FrontmatterRepaired: skill.FrontmatterRepaired}, nil
}

// archiveMatchesSHA / zipSkillFiles / maxSkillBundleTotalBytes：
// agent_service.go:685,779,783（agent runtime 面属主，禁改）与
// skill_market_service.go:259（25c，禁改）的转发（计划 §2.5 未列消费点）。
func archiveMatchesSHA(archive []byte, want string) bool {
	return ArchiveMatchesSHA(archive, want)
}

func zipSkillFiles(files map[string][]byte) ([]byte, error) {
	return ZipSkillFiles(files)
}

const maxSkillBundleTotalBytes = MaxSkillBundleTotalBytes

// —— execution→agentcatalog 4 符号收口（DAG required_contracts）：旧名 1:1 委托导出名 ——
// 调用方 tenant_sandbox_config.go:1134,1135,1137 / user_env.go:275,381 本节点零改动；
// IB2 改指导出名后删除。

func skillSnapshotNamePrefix(tenantID uint64, configID string) string {
	return SkillSnapshotNamePrefix(tenantID, configID)
}

func snapshotsNotFromOtherConfig(listed []sandbox.RemoteSnapshotRef, prefix string) []sandbox.RemoteSnapshotRef {
	return SnapshotsNotFromOtherConfig(listed, prefix)
}

func matchSnapshotByName(listed []sandbox.RemoteSnapshotRef, plannedName string) string {
	return MatchSnapshotByName(listed, plannedName)
}

func validateUserEnvName(name string) error { return ValidateUserEnvName(name) }

// —— conversation 调用形状（原 72 行 effective 残差；effectiveTenantSkills 主体
// 已随迁导出，此处只保留 skillsForRun 旧调用形状；remove_at: ib2）——

func skillsForRun(
	ctx context.Context,
	pinner *SessionSandboxPinner,
	configs SkillImageConfigReader,
	skills installedSkillLister,
	tenantID uint64,
	sessionID string,
	agentConfigID string,
) (string, []*types.TenantSkillEntity) {
	var pinned PinnedConfigReader
	if pinner != nil {
		pinned = pinnerConfigReader{pinner}
	}

	return SkillsForRun(ctx, pinned, configs, skills, tenantID, sessionID, agentConfigID)
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

// pinnerConfigReader adapts *SessionSandboxPinner (returns SandboxPin) to the
// seam that expects the pinned config id string.
type pinnerConfigReader struct{ p *SessionSandboxPinner }

func (a pinnerConfigReader) Read(ctx context.Context, sessionID string) (string, error) {
	pin, err := a.p.Read(ctx, sessionID)
	if err != nil {
		return "", err
	}
	return pin.ConfigID, nil
}

// keyedMutex.lock：25c 市场服务文件（skill_market_service.go / 
// tenant_expert_market_service.go，禁改）的旧小写方法消费；真身实现
// （KeyedMutex.Lock）单一存在，此处补历史小写形状（remove_at: ib2）。
func (k *keyedMutex) lock(ctx context.Context, key string) (func(), error) {
	return k.Lock(ctx, key)
}

// installedSkillLister / skillSnapshotLister：expert_skills.go:36,43 与
// tenant_sandbox_config.go:1125 的类型断言消费（计划 §2.5 未列消费点）；
// 真身同名接口导出后此处保旧未导出名（别名，零成本）。
type (
	installedSkillLister = InstalledSkillLister
	skillSnapshotLister  = SkillSnapshotLister
)
