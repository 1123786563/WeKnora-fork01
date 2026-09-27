package repository

import (
	acrepo "github.com/Tencent/WeKnora/internal/modules/agentcatalog/repository"
	"gorm.io/gorm"
)

// —— Pass B 25b 过渡残差（b2-ac-skills 产出；IB2 按 brief 删除；remove_at: ib2）——
// 生产实现已随迁 internal/modules/agentcatalog/repository/tenant_skill.go；
// 本文件仅保留类型别名与 1:1 转发构造器（无业务逻辑），保以下消费方零改动编译：
// container.go:293/398/780/828（集成工程师独占）、session.go:141/169（conversation）、
// agent_service.go:671/730/744（agentruntime）、user_env.go:67/72 与
// tenant_sandbox_config.go(svc):313-317（execution）。

// TenantSkillRepository aliases the module-owned interface; the method set is
// unchanged, so every existing consumer keeps compiling.
type TenantSkillRepository = acrepo.TenantSkillRepository

// NewTenantSkillRepository forwards to the module implementation.
func NewTenantSkillRepository(db *gorm.DB) TenantSkillRepository {
	return acrepo.NewTenantSkillRepository(db)
}
