package service

import (
	"context"
	"fmt"
	"sort"
	"sync"

	agenttools "github.com/Tencent/WeKnora/internal/agent/tools"
	acrepo "github.com/Tencent/WeKnora/internal/modules/agentcatalog/repository"
	acatsvc "github.com/Tencent/WeKnora/internal/modules/agentcatalog/service"
	"github.com/Tencent/WeKnora/internal/types"
)

// Ruling 2026-09-24-TEST-SUPPORT-SHIM；remove_at: IB2 或 25c/宿主测试文件随迁时（先到者）；
// 仅供 internal/application/service 测试编译使用。
// 宿主 3 个禁改测试文件（user_env_test.go / agent_service_skill_bundle_test.go /
// agent_service_install_shell_test.go）以未导出符号共享 25b 搬迁面的测试装置；
// 本文件按裁定提供最小宿主侧定义，禁止夹带业务逻辑。
// 25c（b2-ac-market）按同一裁定的「宿主包唯一垫片文件」约束追加 fakePublisherNames
// （见文件尾标注小节），删除期限按其独立注释执行。

// validSkillMD：agent_service_skill_bundle_test.go 消费的 SKILL.md fixture
// （与 agentcatalog/service/tenant_skill_bundle_test.go 同源副本，仅测试数据）。
const validSkillMD = `---
name: pdf-tools
description: Extract text from PDF files
---

Use scripts/extract.py to pull text out of a PDF.
`

// skillArchiveSHA256 转发新包导出名（转发，无逻辑）。
func skillArchiveSHA256(archive []byte) string {
	return acatsvc.SkillArchiveSHA256(archive)
}

// installerAgentConfig 保迁移前 3 参形状。第 4 参取 agent/tools 三个 install-mode
// 工具名常量（ToolShellExec/ToolWriteSkillFile/ToolEditSkillFile）——与旧 3 参
// 语义等价（旧实现用同一常量元组，现由 HostAdapters.InstallerToolNames 承载），
// 依据见 agentcatalog/service/tenant_skill_install.go 的 InstallerAgentConfig 注释。
func installerAgentConfig(defaults *types.CustomAgent, configID, skillDir string) *types.AgentConfig {
	return acatsvc.InstallerAgentConfig(defaults, configID, skillDir, []string{
		agenttools.ToolShellExec, agenttools.ToolWriteSkillFile, agenttools.ToolEditSkillFile,
	})
}

// installSkillRepo 是 installSkillRepository 测试替身的宿主侧副本：嵌入
// acrepo 接口（nil 底层）以满足 TenantSkillRepository 全接口编译，显式实现
// user_env_test.go 的方法面（技能种子/读取 + userEnvs 存储）。未显式实现的
// 接口方法不在 user-env 测试执行路径上；若被调用将触发 nil 接口 panic（视为
// 越出测试面，属预期保护）。完整替身随迁于
// agentcatalog/service/tenant_skill_install_test.go。
type installSkillRepo struct {
	acrepo.TenantSkillRepository
	mu     sync.Mutex
	skills map[string]*types.TenantSkillEntity
	// userEnvs is a real store rather than a stub: the env-var flows are about
	// which principal's value wins, so a fake that cannot distinguish
	// principals would let the interesting bugs through.
	userEnvs []*types.TenantUserEnvVar
}

func newInstallSkillRepo() *installSkillRepo {
	return &installSkillRepo{skills: map[string]*types.TenantSkillEntity{}}
}

func testInstallSkillKey(tenantID uint64, configID, skillID string) string {
	return fmt.Sprintf("%d|%s|%s", tenantID, configID, skillID)
}

func (r *installSkillRepo) CreateSkill(_ context.Context, e *types.TenantSkillEntity) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *e
	r.skills[testInstallSkillKey(e.TenantID, e.SandboxConfigID, e.ID)] = &cp
	return nil
}

// GetSkill and UpdateSkill honour the context because the real gorm repository
// does: a write attempted on a cancelled context never reaches the database.
func (r *installSkillRepo) GetSkill(
	ctx context.Context, tenantID uint64, configID, skillID string,
) (*types.TenantSkillEntity, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	e := r.skills[testInstallSkillKey(tenantID, configID, skillID)]
	if e == nil {
		return nil, nil
	}
	cp := *e
	return &cp, nil
}

func (r *installSkillRepo) ListSkillsByConfig(
	ctx context.Context, tenantID uint64, configID string,
) ([]*types.TenantSkillEntity, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*types.TenantSkillEntity
	for _, e := range r.skills {
		if e.TenantID == tenantID && e.SandboxConfigID == configID {
			cp := *e
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *installSkillRepo) ListSkillsByTenant(
	ctx context.Context, tenantID uint64,
) ([]*types.TenantSkillEntity, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*types.TenantSkillEntity
	for _, e := range r.skills {
		if e.TenantID == tenantID {
			cp := *e
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *installSkillRepo) GetSkillByName(
	_ context.Context, tenantID uint64, configID, name string,
) (*types.TenantSkillEntity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.skills {
		if e.TenantID == tenantID && e.SandboxConfigID == configID && e.Name == name {
			cp := *e
			return &cp, nil
		}
	}
	return nil, nil
}

func (r *installSkillRepo) UpdateSkill(ctx context.Context, e *types.TenantSkillEntity) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := testInstallSkillKey(e.TenantID, e.SandboxConfigID, e.ID)
	cp := *e
	// The real UpdateSkill leaves the envs column alone, so a stale in-memory
	// copy cannot put an old declaration back.
	if stored := r.skills[key]; stored != nil {
		cp.Envs = stored.Envs
	} else {
		cp.Envs = nil
	}
	r.skills[key] = &cp
	return nil
}

// testUserEnvMatches mirrors the real repository's unique index, minus the
// name, which the per-row callers add.
func testUserEnvMatches(
	e *types.TenantUserEnvVar, tenantID uint64, p types.Principal, configID, skillID string,
) bool {
	p = p.Normalize()
	return e.TenantID == tenantID && e.PrincipalType == p.Type &&
		e.PrincipalID == p.ID && e.SandboxConfigID == configID && e.SkillID == skillID
}

func (r *installSkillRepo) ListUserEnvVars(
	ctx context.Context, tenantID uint64, p types.Principal, configID, skillID string,
) ([]*types.TenantUserEnvVar, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*types.TenantUserEnvVar
	for _, e := range r.userEnvs {
		if testUserEnvMatches(e, tenantID, p, configID, skillID) {
			cp := *e
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (r *installSkillRepo) ListUserEnvVarsByConfig(
	ctx context.Context, tenantID uint64, p types.Principal, configID string,
) ([]*types.TenantUserEnvVar, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	p = p.Normalize()
	var out []*types.TenantUserEnvVar
	for _, e := range r.userEnvs {
		if e.TenantID == tenantID && e.PrincipalType == p.Type &&
			e.PrincipalID == p.ID && e.SandboxConfigID == configID {
			cp := *e
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SkillID != out[j].SkillID {
			return out[i].SkillID < out[j].SkillID
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (r *installSkillRepo) UpsertUserEnvVar(ctx context.Context, e *types.TenantUserEnvVar) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	p := types.Principal{Type: e.PrincipalType, ID: e.PrincipalID}.Normalize()
	for _, existing := range r.userEnvs {
		if testUserEnvMatches(existing, e.TenantID, p, e.SandboxConfigID, e.SkillID) &&
			existing.Name == e.Name {
			existing.Value = e.Value
			return nil
		}
	}
	cp := *e
	cp.PrincipalType, cp.PrincipalID = p.Type, p.ID
	r.userEnvs = append(r.userEnvs, &cp)
	return nil
}

func (r *installSkillRepo) DeleteUserEnvVar(
	ctx context.Context, tenantID uint64, p types.Principal, configID, skillID, name string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, e := range r.userEnvs {
		if testUserEnvMatches(e, tenantID, p, configID, skillID) && e.Name == name {
			r.userEnvs = append(r.userEnvs[:i], r.userEnvs[i+1:]...)
			return nil
		}
	}
	return types.ErrEnvVarNotFound
}

func (r *installSkillRepo) DeleteUserEnvVarsByConfig(
	ctx context.Context, tenantID uint64, configID string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := r.userEnvs[:0]
	for _, e := range r.userEnvs {
		if e.TenantID == tenantID && e.SandboxConfigID == configID {
			continue
		}
		kept = append(kept, e)
	}
	r.userEnvs = kept
	return nil
}

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
