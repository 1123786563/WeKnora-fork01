package service

import (
	"context"

	agenttools "github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
)

// Ruling 2026-09-24-TEST-SUPPORT-SHIM；remove_at: IB2 或 25c/宿主测试文件随迁时（先到者）；
// 仅供 internal/application/service 测试编译使用。
// 宿主 3 个禁改测试文件（user_env_test.go / agent_service_skill_bundle_test.go /
// agent_service_install_shell_test.go）以未导出符号共享 25b 搬迁面的测试装置；
// 本文件按裁定提供最小宿主侧定义，禁止夹带业务逻辑。
// 25c（b2-ac-market）按同一裁定的「宿主包唯一垫片文件」约束追加 fakePublisherNames
// （见文件尾标注小节），删除期限按其独立注释执行。
//
// 目录重构回归修复：tenant_skill_install_test.go 已随上游同构布局归位本包，
// 其完整 installSkillRepo / newInstallSkillRepo 替身（含接口断言，见该文件
// var _ repository.TenantSkillRepository）重新可见，本文件此前的最小同名片段
// 及其方法面按原裁定「宿主测试文件随迁时同删」一并移除，避免同包重声明。

// validSkillMD：agent_service_skill_bundle_test.go 消费的 SKILL.md fixture
// —— Pass B 25b 曾在本地保留同源副本；真身 tenant_skill_bundle_test.go
// 归位本包后共享其定义，此处不再重复声明（remove_at: ib2）。

// skillArchiveSHA256 转发新包导出名（转发，无逻辑）。
func skillArchiveSHA256(archive []byte) string {
	return SkillArchiveSHA256(archive)
}

// installerAgentConfig 保迁移前 3 参形状。第 4 参取 agent/tools 三个 install-mode
// 工具名常量（ToolShellExec/ToolWriteSkillFile/ToolEditSkillFile）——与旧 3 参
// 语义等价（旧实现用同一常量元组，现由 HostAdapters.InstallerToolNames 承载），
// 依据见 agentcatalog/service/tenant_skill_install.go 的 InstallerAgentConfig 注释。
func installerAgentConfig(defaults *types.CustomAgent, configID, skillDir string) *types.AgentConfig {
	return InstallerAgentConfig(defaults, configID, skillDir, []string{
		agenttools.ToolShellExec, agenttools.ToolWriteSkillFile, agenttools.ToolEditSkillFile,
	})
}

// installSkillRepo / newInstallSkillRepo：原宿主侧最小替身片段已移除——
// tenant_skill_install_test.go 归位本包后其完整定义（含 snapshots/catalogs
// 存储与全部 user-env 方法面）重新可见，见文件头注释。

// —— 25c 追加（b2-ac-market；Ruling 2026-09-24-TEST-SUPPORT-SHIM）——
// fakePublisherNames：宿主禁改推迟件测试 tenant_expert_market_service_test.go:365
// 消费的 publisher-names fake；原定义随 25c 搬迁至
// agentcatalog/service/tenant_skill_market_service_test.go，此处为逐字同体副本。
// remove_at: 推迟件 tenant_expert_market_service.go 及其测试搬迁时（25a 批次 2
// / B3 窗口），随该测试文件同删。
type fakePublisherNames struct {
	users map[string]*types.User
	err   error
}

func (f *fakePublisherNames) GetUsersByIDs(_ context.Context, ids []string) (map[string]*types.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make(map[string]*types.User, len(ids))
	for _, id := range ids {
		if u := f.users[id]; u != nil {
			out[id] = u
		}
	}
	return out, nil
}
