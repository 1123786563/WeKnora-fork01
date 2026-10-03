package service

import (
	"context"

	"github.com/redis/go-redis/v9"

	"github.com/Tencent/WeKnora/internal/application/repository"
	acatsvc "github.com/Tencent/WeKnora/internal/modules/agentcatalog/service"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/skills"
	agenttools "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/tools"
	"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// —— Pass B 25b 过渡残差（b2-ac-skills 产出；IB2 按 brief 删除；禁止新增业务逻辑）——
// remove_at: ib2

func init() { // 替代随迁走的 env_declare.go init()（计划 §4.4-1），行为等价
	acatsvc.RegisterReservedEnvNames(skills.InjectedSandboxEnvVars())
	// 25b 补充接缝（计划 §4.4-1 同类先例）：adapters-free 的 ParseSkillBundle
	// 入口（25a/25c 禁改测试与本包消费方经此调用）需要 SKILL.md 解析能力。
	acatsvc.RegisterBundleParsers(parseSkillManifest, skills.UnmarshalSkillFrontmatter)
}

// parseSkillManifest 1:1 包装 skills.ParseSkillFile 的 4 字段消费面。
func parseSkillManifest(content string) (acatsvc.SkillManifestView, error) {
	skill, err := skills.ParseSkillFile(content)
	if err != nil {
		return acatsvc.SkillManifestView{}, err
	}
	return acatsvc.SkillManifestView{Name: skill.Name, Description: skill.Description,
		Instructions: skill.Instructions, FrontmatterRepaired: skill.FrontmatterRepaired}, nil
}

// 类型别名：保 container.go / session.go / agent_service.go / user_env.go /
// tenant_sandbox_config.go(svc) / skill_market_service.go / tenant_skill_market_service.go /
// handler/sandbox_skill.go 编译（计划 §2.5 清单）。installedSkillLister 保留旧未导出名，
// expert_skills.go:36,43 零改动（计划 §2.5）。
type (
	TenantSkillService        = acatsvc.TenantSkillService
	SkillBundle               = acatsvc.SkillBundle
	SkillBundleParseOptions   = acatsvc.SkillBundleParseOptions
	SkillFileEntry            = acatsvc.SkillFileEntry
	SkillFileContent          = acatsvc.SkillFileContent
	SkillCatalogView          = acatsvc.SkillCatalogView
	SkillCatalogInstallView   = acatsvc.SkillCatalogInstallView
	CatalogInstallResult      = acatsvc.CatalogInstallResult
	SkillAdminUpdate          = acatsvc.SkillAdminUpdate
	SkillProgress             = acatsvc.SkillProgress
	SkillInstallGuidance      = acatsvc.SkillInstallGuidance
	SkillInstallGuidanceState = acatsvc.SkillInstallGuidanceState
	installedSkillLister      = acatsvc.InstalledSkillLister
)

// keyedMutex/newKeyedMutex：25c 市场服务文件（skill_market_service.go 等，禁改）
// 在宿主包内的锁消费（计划 §2.5 未列消费点）。Go 不可对非本包类型定义小写方法，
// 故用嵌入包装补历史 lock 方法，实现仍单一存在于新包（acatsvc.KeyedMutex）。
type keyedMutex struct {
	acatsvc.KeyedMutex
}

func newKeyedMutex() *keyedMutex { return &keyedMutex{KeyedMutex: *acatsvc.NewKeyedMutex()} }

func (k *keyedMutex) lock(ctx context.Context, key string) (func(), error) {
	return k.Lock(ctx, key)
}

// skillSnapshotLister：tenant_sandbox_config.go:1125（execution，禁改）的类型
// 断言消费（计划 §2.5 未列消费点）；新包同名接口导出后此处保旧名。
type skillSnapshotLister = acatsvc.SkillSnapshotLister

// archiveMatchesSHA / zipSkillFiles / maxSkillBundleTotalBytes：
// agent_service.go:685,779,783（agent runtime 面属主，禁改）与
// skill_market_service.go:259（25c，禁改）的转发（计划 §2.5 未列消费点）。
func archiveMatchesSHA(archive []byte, want string) bool {
	return acatsvc.ArchiveMatchesSHA(archive, want)
}

func zipSkillFiles(files map[string][]byte) ([]byte, error) {
	return acatsvc.ZipSkillFiles(files)
}

const maxSkillBundleTotalBytes = acatsvc.MaxSkillBundleTotalBytes

// MaxEnvValueBytes / MaxUserEnvVarsPerScope：user_env.go:388-407（execution，
// 禁改）的常量消费（计划 §2.5 未列消费点）。
const (
	MaxEnvValueBytes       = acatsvc.MaxEnvValueBytes
	MaxUserEnvVarsPerScope = acatsvc.MaxUserEnvVarsPerScope
)

func ParseSkillBundle(archive []byte) (*SkillBundle, error) { return acatsvc.ParseSkillBundle(archive) }

// ErrSkillBundleInvalid / ErrSkillSourceInvalid：handler/sandbox_skill.go:175
// （禁改）的哨兵错误消费（计划 §2.5 未列消费点）；引用同一错误值，identity 等价。
var (
	ErrSkillBundleInvalid = acatsvc.ErrSkillBundleInvalid
	ErrSkillSourceInvalid = acatsvc.ErrSkillSourceInvalid
)

func ParseSkillBundleWithOptions(archive []byte, opts SkillBundleParseOptions) (*SkillBundle, error) {
	return acatsvc.ParseSkillBundleWithOptions(archive, opts)
}

// NewTenantSkillService：旧 12 参签名原样保留（container.go:566 零改动）；
// sandboxPolicy 吸收进 ResolveConfigManager 闭包；计划 §4.4 的 agent 运行时
// 能力位在此绑真源。
func NewTenantSkillService(
	skillsRepo repository.TenantSkillRepository,
	configsRepo repository.TenantSandboxConfigRepository,
	resolver interfaces.StorageBackendResolver,
	sandboxes sandbox.TenantSandboxResolver,
	sandboxPolicy WorkspaceSandboxPolicy,
	agents interfaces.AgentService,
	customAgents interfaces.CustomAgentService,
	sessions interfaces.SessionService,
	models interfaces.ModelService,
	redisClient *redis.Client,
	streams interfaces.StreamManager,
	messages interfaces.MessageRepository,
	host HostSandboxManager,
) *TenantSkillService {
	return acatsvc.NewTenantSkillService(skillsRepo, configsRepo, resolver, sandboxes,
		agents, customAgents, sessions, models, redisClient, streams, messages,
		acatsvc.HostAdapters{
			ResolveConfigManager: func(ctx context.Context, tenantID uint64, configID string) (sandbox.Manager, error) {
				return resolveTenantSandboxForConfig(ctx, sandboxes, nil, tenantID, configID, sandboxPolicy)
			},
			InstallShellExecutor:     sessionSandboxInstallShellExecutor,
			SessionUserID:            nil, // nil → 新包缺省 types.SessionOwnerIDFromContext（与 session.go:26 等价）
			SkillManifestParser:      parseSkillManifest,
			FrontmatterVersionParser: skills.UnmarshalSkillFrontmatter,
			InstallerToolNames: func() [3]string {
				return [3]string{agenttools.ToolShellExec, agenttools.ToolWriteSkillFile, agenttools.ToolEditSkillFile}
			},
			OnDemandInstallerPath:        skills.IsOnDemandInstallerPath,
			StreamContentForToolResult:   agenttools.StreamContentForToolResult, // 与工具名常量同包（agent/tools），单一 import
			SanitizeToolResultForClient:  agenttools.SanitizeToolResultForClient,
			SanitizeAgentStepsForStorage: agenttools.SanitizeAgentStepsForStorage,
			UniqueNonEmptyStrings:        uniqueNonEmptyStrings, // conversation 真源（session_knowledge_qa.go:638，同包捕获）
		})
}

// —— execution→agentcatalog 4 符号收口（DAG required_contracts）：旧名 1:1 委托导出名 ——
// 调用方 tenant_sandbox_config.go:1134,1135,1137 / user_env.go:275,381 本节点零改动；
// IB2 改指 acatsvc 导出名后删除。

func skillSnapshotNamePrefix(tenantID uint64, configID string) string {
	return acatsvc.SkillSnapshotNamePrefix(tenantID, configID)
}

func snapshotsNotFromOtherConfig(listed []sandbox.RemoteSnapshotRef, prefix string) []sandbox.RemoteSnapshotRef {
	return acatsvc.SnapshotsNotFromOtherConfig(listed, prefix)
}

func matchSnapshotByName(listed []sandbox.RemoteSnapshotRef, plannedName string) string {
	return acatsvc.MatchSnapshotByName(listed, plannedName)
}

func validateUserEnvName(name string) error { return acatsvc.ValidateUserEnvName(name) }
