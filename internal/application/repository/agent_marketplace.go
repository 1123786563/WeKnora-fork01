package repository

import (
	acrepo "github.com/Tencent/WeKnora/internal/modules/agentcatalog/repository"
)

// Pass B (25c) transitional shim — the implementation moved to
// internal/modules/agentcatalog/repository/agent_marketplace.go. Consumers
// are switched to the module package by IB2, after which this file is
// deleted (12-commercial §5.4 / 25a §2.3 pattern). No business logic may be
// added here — remove_at: ib2.

// AgentMarketplaceRepository is an alias of the module interface definition.
type AgentMarketplaceRepository = acrepo.AgentMarketplaceRepository

// NewAgentMarketplaceRepository forwards to the module constructor.
var NewAgentMarketplaceRepository = acrepo.NewAgentMarketplaceRepository

// Sentinel vars forward to the module definitions so errors.Is keeps
// matching the same error values across the transition.
var (
	ErrAgentMarketplaceDigestMismatch       = acrepo.ErrAgentMarketplaceDigestMismatch
	ErrAgentMarketplacePointerConflict      = acrepo.ErrAgentMarketplacePointerConflict
	ErrAgentMarketplaceNotFound             = acrepo.ErrAgentMarketplaceNotFound
	ErrAgentMarketplaceInvalidDecision      = acrepo.ErrAgentMarketplaceInvalidDecision
	ErrAgentMarketplaceReviewConflict       = acrepo.ErrAgentMarketplaceReviewConflict
	ErrAgentMarketplaceVersionAgentMismatch = acrepo.ErrAgentMarketplaceVersionAgentMismatch
)
