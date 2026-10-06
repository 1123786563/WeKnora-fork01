package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
)

// HostAdapters 承接旧宿主包内仍留驻的 conversation/execution 未导出能力与
// agent 运行时模块工具面（Pass B 25b 搬迁解耦），由旧路径残差构造器装配（IB2 起、
// 35b/13-execution 搬迁后收口为对端导出端口）。字段语义与被替换符号 1:1；
// 除 SessionUserID 外全部必填（NewTenantSkillService 内显式校验并 fail-fast）。
//
// remove_at: ib2 — 对端符号导出后逐位删除。
type HostAdapters struct {
	// ResolveConfigManager 替代 resolveTenantSandboxForConfig（execution，
	// internal/application/service/tenant_sandbox_resolve.go）：显式
	// (tenantID, configID) 的 Manager，不得静默回退默认 Manager；
	// 旧调用第 3 参恒为 nil、policy 吸收进残差闭包。
	ResolveConfigManager func(ctx context.Context, tenantID uint64, configID string) (sandbox.Manager, error)
	// InstallShellExecutor 替代 sessionSandboxInstallShellExecutor（conversation，
	// internal/application/service/session_attachment_staging.go）：类型断言
	// SessionInstallCapabilityProvider。
	InstallShellExecutor func(mgr sandbox.Manager) sandbox.SessionInstallShellExecutor
	// SessionUserID 替代 sessionUserIDFromContext（conversation，
	// internal/application/service/session.go，1:1 委托公共 API）。
	// nil 缺省为 types.SessionOwnerIDFromContext(ctx)。
	SessionUserID func(ctx context.Context) string
	// SkillManifestParser 替代 skills.ParseSkillFile（agent 运行时 skills 包）
	// + SkillBundle 的 4 字段拷贝，见 SkillManifestView。
	SkillManifestParser func(content string) (SkillManifestView, error)
	// FrontmatterVersionParser 替代 skills.UnmarshalSkillFrontmatter
	// （agent 运行时 skills 包），bundle 版本探测 helper 消费。
	FrontmatterVersionParser func(frontmatter string, dest any) (bool, error)
	// InstallerToolNames 替代 tools.ToolShellExec/ToolWriteSkillFile/ToolEditSkillFile
	// （agent 运行时 tools 包）；返回 [shellExec, writeSkillFile, editSkillFile]。
	InstallerToolNames func() [3]string
	// StreamContentForToolResult / SanitizeToolResultForClient / SanitizeAgentStepsForStorage
	// 替代 agent 运行时 tools persist 三个自由函数。
	StreamContentForToolResult   func(toolName string, success bool, errMsg string, data map[string]interface{}) string
	SanitizeToolResultForClient  func(toolName string, result *types.ToolResult) map[string]interface{}
	SanitizeAgentStepsForStorage func(steps []types.AgentStep) []types.AgentStep
	// OnDemandInstallerPath 替代 skills.IsOnDemandInstallerPath（agent 运行时
	// skills 包），判定 bundle 内按需安装脚本路径（安装 prompt 消费）。
	OnDemandInstallerPath func(scriptPath string) bool
	// UniqueNonEmptyStrings 替代 conversation 属主 uniqueNonEmptyStrings
	// （session_knowledge_qa.go：跳过空串再去重）。本面 uniqueStrings（bundle.go，
	// 仅去重、保留空串）语义不同，禁止替代。
	UniqueNonEmptyStrings func(values []string) []string
}

// SkillManifestView 是 SKILL.md 解析结果中安装/目录流程实际消费的 4 字段数据视图
// （数据形状，非逻辑复制；对应 agent 运行时 skills.Skill 的消费面）。
type SkillManifestView struct {
	Name                string
	Description         string
	Instructions        string
	FrontmatterRepaired bool
}

// validate fail-fasts on every required adapter. SessionUserID is optional:
// its nil default is the same public API the conversation helper delegated to.
func (a HostAdapters) validate() error {
	required := map[string]bool{
		"ResolveConfigManager":         a.ResolveConfigManager == nil,
		"InstallShellExecutor":         a.InstallShellExecutor == nil,
		"SkillManifestParser":          a.SkillManifestParser == nil,
		"FrontmatterVersionParser":     a.FrontmatterVersionParser == nil,
		"InstallerToolNames":           a.InstallerToolNames == nil,
		"StreamContentForToolResult":   a.StreamContentForToolResult == nil,
		"SanitizeToolResultForClient":  a.SanitizeToolResultForClient == nil,
		"SanitizeAgentStepsForStorage": a.SanitizeAgentStepsForStorage == nil,
		"OnDemandInstallerPath":        a.OnDemandInstallerPath == nil,
		"UniqueNonEmptyStrings":        a.UniqueNonEmptyStrings == nil,
	}
	for name, missing := range required {
		if missing {
			return fmt.Errorf("tenant skill service: HostAdapters.%s is required", name)
		}
	}
	return nil
}

func (a HostAdapters) resolveConfigManager(ctx context.Context, tenantID uint64, configID string) (sandbox.Manager, error) {
	if a.ResolveConfigManager == nil {
		return nil, errors.New("HostAdapters.ResolveConfigManager is not wired")
	}
	return a.ResolveConfigManager(ctx, tenantID, configID)
}

func (a HostAdapters) installShellExecutor(mgr sandbox.Manager) sandbox.SessionInstallShellExecutor {
	if a.InstallShellExecutor == nil {
		return nil
	}
	return a.InstallShellExecutor(mgr)
}

func (a HostAdapters) sessionUserID(ctx context.Context) string {
	if a.SessionUserID != nil {
		return a.SessionUserID(ctx)
	}
	return types.SessionOwnerIDFromContext(ctx)
}

func (a HostAdapters) skillManifestParser(content string) (SkillManifestView, error) {
	if a.SkillManifestParser == nil {
		return SkillManifestView{}, errors.New("HostAdapters.SkillManifestParser is not wired")
	}
	return a.SkillManifestParser(content)
}

func (a HostAdapters) frontmatterVersionParser(frontmatter string, dest any) (bool, error) {
	if a.FrontmatterVersionParser == nil {
		return false, errors.New("HostAdapters.FrontmatterVersionParser is not wired")
	}
	return a.FrontmatterVersionParser(frontmatter, dest)
}

func (a HostAdapters) installerToolNames() [3]string {
	return a.InstallerToolNames()
}

func (a HostAdapters) onDemandInstallerPath(scriptPath string) bool {
	return a.OnDemandInstallerPath(scriptPath)
}

func (a HostAdapters) uniqueNonEmptyStrings(values []string) []string {
	return a.UniqueNonEmptyStrings(values)
}

// manifestParsers 是 SKILL.md 解析两位（清单 + frontmatter 版本）的成组取用。
func (a HostAdapters) manifestParsers() skillManifestParsers {
	return skillManifestParsers{
		manifest:    a.skillManifestParser,
		frontmatter: a.frontmatterVersionParser,
	}
}

// transcriptSanitizer 收敛 install 转录对 agent 运行时 persist 三个函数的消费，
// 由 newInstallTranscript 的调用方从 adapters 装配。
type transcriptSanitizer struct {
	streamContentForToolResult   func(toolName string, success bool, errMsg string, data map[string]interface{}) string
	sanitizeToolResultForClient  func(toolName string, result *types.ToolResult) map[string]interface{}
	sanitizeAgentStepsForStorage func(steps []types.AgentStep) []types.AgentStep
}

func (a HostAdapters) transcriptSanitizer() transcriptSanitizer {
	return transcriptSanitizer{
		streamContentForToolResult:   a.StreamContentForToolResult,
		sanitizeToolResultForClient:  a.SanitizeToolResultForClient,
		sanitizeAgentStepsForStorage: a.SanitizeAgentStepsForStorage,
	}
}
