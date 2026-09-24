package service

import (
	acatsvc "github.com/Tencent/WeKnora/internal/modules/agentcatalog/service"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/experts"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Pass B (25c) transitional shim — the implementation moved to
// internal/modules/agentcatalog/service/agent_marketplace.go. Consumers are
// switched to the module package by IB2, after which this file is deleted
// (12-commercial §5.4 / 25a §2.3 pattern). No business logic may be added
// here — remove_at: ib2.

// NewAgentMarketplaceService keeps the legacy 4-arg signature
// (internal/container/container.go and routes_agent_marketplace_test.go are
// untouched) and binds the real agents-runtime builder into the module's
// MarketHostAdapters seam.
func NewAgentMarketplaceService(versions interfaces.AgentVersionService, resolver interfaces.ReleaseDependencyResolver, repo interfaces.AgentMarketplaceRepository, bundleRoot string) *acatsvc.AgentMarketplaceService {
	return acatsvc.NewAgentMarketplaceService(versions, resolver, repo, bundleRoot,
		acatsvc.MarketHostAdapters{BuildReleaseBundle: experts.BuildAgentReleaseBundle})
}

// Sentinel vars forward to the module definitions so errors.Is keeps
// matching the same error values across the transition.
var (
	ErrAgentMarketplaceInvalidInput      = acatsvc.ErrAgentMarketplaceInvalidInput
	ErrAgentMarketplaceStaleDigest       = acatsvc.ErrAgentMarketplaceStaleDigest
	ErrAgentMarketplaceMissingDependency = acatsvc.ErrAgentMarketplaceMissingDependency
)
