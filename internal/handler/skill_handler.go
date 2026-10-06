package handler

import (
	"context"

	acatsvc "github.com/Tencent/WeKnora/internal/application/service"
	acathandler "github.com/Tencent/WeKnora/internal/modules/agentcatalog/handler"
	"github.com/Tencent/WeKnora/internal/types"
)

// —— Pass B 25b 过渡残差（b2-ac-skills 产出；IB2 按 brief 删除；禁止新增业务逻辑）——
// remove_at: ib2
//
// 保零改动编译：routes_agent.go:76（RegisterSkillRoutes 参数与方法调用，方法集
// 经 SkillHandler 别名整体可达）、container.go:801-803（handler.NewSkillHandler(s, s)）、
// router_api_key_capabilities_test.go:448 / routes_skill_market_test.go:31
// （&handler.SkillHandler{} 复合字面量）。

// usableSkillLister / skillCatalogService：宿主同形未导出接口（计划 §5.4），
// 返回类型指 acatsvc 冻结视图；*service.TenantSkillService（残差别名）结构满足。
type usableSkillLister interface {
	ListUsableSkills(ctx context.Context, tenantID uint64, configID string) []*types.TenantSkillEntity
}

type skillCatalogService interface {
	ListCatalog(ctx context.Context, tenantID uint64) ([]acatsvc.SkillCatalogView, error)
	RegisterCatalogFromArchive(ctx context.Context, tenantID uint64, archive []byte) (*types.TenantSkillCatalogEntity, error)
	RegisterCatalogFromSource(ctx context.Context, tenantID uint64, source string) (*types.TenantSkillCatalogEntity, error)
	InstallCatalogToConfigs(ctx context.Context, tenantID uint64, catalogID string, configIDs []string) (*acatsvc.CatalogInstallResult, error)
	DeleteCatalog(ctx context.Context, tenantID uint64, catalogID string) error
	ListCatalogFiles(ctx context.Context, tenantID uint64, catalogID string) ([]acatsvc.SkillFileEntry, error)
	ReadCatalogFile(ctx context.Context, tenantID uint64, catalogID, relativePath string) (*acatsvc.SkillFileContent, error)
}

// SkillHandler 别名：ListSkills/ListCatalog/RegisterCatalog/InstallCatalog/
// ListCatalogFiles/GetCatalogFile/DeleteCatalog 全部方法随别名可达
// （Go 不允许经别名在宿主包重声明方法，计划 §5.4 的「8 方法委托」形态不可实现，
// 以别名承载方法集替代，详见报告偏差登记）。
type SkillHandler = acathandler.SkillHandler

// NewSkillHandler 1:1 转发；宿主接口值满足新包同形未导出接口（结构化类型）。
func NewSkillHandler(usableSkills usableSkillLister, catalog skillCatalogService) *SkillHandler {
	return acathandler.NewSkillHandler(usableSkills, catalog)
}
