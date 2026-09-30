package service

import (
	repository "github.com/Tencent/WeKnora/internal/application/repository"
	acatsvc "github.com/Tencent/WeKnora/internal/modules/agentcatalog/service"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Pass B (25c) transitional shim — the implementation moved to
// internal/modules/agentcatalog/service/tenant_skill_market_service.go.
// Consumers are switched to the module package by IB2, after which this file
// is deleted (12-commercial §5.4 / 25a §2.3 pattern). No business logic may
// be added here — remove_at: ib2.

// TenantSkillMarketCatalogStore is an alias of the module interface
// definition (unexported name, single source of truth).
type TenantSkillMarketCatalogStore = acatsvc.TenantSkillMarketCatalogStore

// TenantSkillMarketInstaller is an alias of the module interface
// definition (unexported name, single source of truth).
type TenantSkillMarketInstaller = acatsvc.TenantSkillMarketInstaller

// NewTenantSkillMarketService forwards 1:1 to the module constructor.
func NewTenantSkillMarketService(
	published repository.PublishedSkillRepository,
	catalogs TenantSkillMarketCatalogStore,
	installer TenantSkillMarketInstaller,
	users interfaces.TenantSkillPublisherNames,
) *acatsvc.TenantSkillMarketService {
	return acatsvc.NewTenantSkillMarketService(published, catalogs, installer, users)
}
