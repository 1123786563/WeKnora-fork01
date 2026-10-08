package service

import (
	"context"
	"errors"

	"github.com/Tencent/WeKnora/internal/agent/skills"
	agenttools "github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/sandbox"
)

// testInstallShellExecutor 与宿主 sessionSandboxInstallShellExecutor 同构：
// 经 SessionInstallCapabilityProvider 取 install-mode 执行器，无能力即 nil。
func testInstallShellExecutor(mgr sandbox.Manager) sandbox.SessionInstallShellExecutor {
	if p, ok := mgr.(sandbox.SessionInstallCapabilityProvider); ok {
		return p.SessionInstallShellExecutor()
	}
	return nil
}

// 目录重构回归修复（restructure/upstream-align-round2）：本文件自
// internal/agentcatalog/service/ 随 tenant-skill 测试面迁回宿主包
// （HostAdapters / SkillManifestView 真身已在本包 host_adapters.go）。
// ctxWithTenant 原补齐副本随之删除——本包 knowledgebase_pr3_test.go 的
// 同名 helper 重新同包可见。

// 本文件是测试二进制的宿主能力装载（计划 §4.4-2）：注入名与 SKILL.md
// 解析能力在测试二进制内显式注册（与生产同一 agentruntime 真源，guard 对
// _test.go 豁免）。生产侧 tenant_skill_passb_host_shims.go 的 init() 已注册
// 等价函数；RegisterBundleParsers 为幂等重赋值，双注册无害。

func init() {
	RegisterReservedEnvNames(skills.InjectedSandboxEnvVars())
	RegisterBundleParsers(testManifestParser, skills.UnmarshalSkillFrontmatter)
}

// testManifestParser 与宿主残差 parseSkillManifest 相同的 4 字段视图包装。
func testManifestParser(content string) (SkillManifestView, error) {
	skill, err := skills.ParseSkillFile(content)
	if err != nil {
		return SkillManifestView{}, err
	}
	return SkillManifestView{Name: skill.Name, Description: skill.Description,
		Instructions: skill.Instructions, FrontmatterRepaired: skill.FrontmatterRepaired}, nil
}

// testHostAdapters 返回满足构造器 fail-fast 校验的全量适配位（绑定生产真源）。
// 各测试按需以具体 fake 覆盖个别字段；ResolveConfigManager 缺省拒绝（触发即
// 说明该测试需要自行注入解析行为）。
func testHostAdapters() HostAdapters {
	return HostAdapters{
		ResolveConfigManager: func(context.Context, uint64, string) (sandbox.Manager, error) {
			return nil, errors.New("ResolveConfigManager is not wired by this test")
		},
		InstallShellExecutor:     testInstallShellExecutor,
		SkillManifestParser:      testManifestParser,
		FrontmatterVersionParser: skills.UnmarshalSkillFrontmatter,
		InstallerToolNames: func() [3]string {
			return [3]string{agenttools.ToolShellExec, agenttools.ToolWriteSkillFile, agenttools.ToolEditSkillFile}
		},
		StreamContentForToolResult:   agenttools.StreamContentForToolResult,
		SanitizeToolResultForClient:  agenttools.SanitizeToolResultForClient,
		SanitizeAgentStepsForStorage: agenttools.SanitizeAgentStepsForStorage,
		OnDemandInstallerPath:        skills.IsOnDemandInstallerPath,
		UniqueNonEmptyStrings:        skipEmptyStrings,
	}
}

// skipEmptyStrings 是测试期 UniqueNonEmptyStrings 的语义替身（跳过空串再去重，
// 与 conversation 真源 uniqueNonEmptyStrings 同语义）。
func skipEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
