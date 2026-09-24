package handler

import (
	acathandler "github.com/Tencent/WeKnora/internal/modules/agentcatalog/handler"
)

// Pass B (25c) transitional shim — the implementation moved to
// internal/modules/agentcatalog/handler/agent_marketplace.go. Consumers are
// switched to the module package by IB2, after which this file is deleted.
// No new business logic may be added here — remove_at: ib2.

// AgentMarketplaceHandler aliases the module type so the router field and
// route-registration signatures keep compiling unchanged.
type AgentMarketplaceHandler = acathandler.AgentMarketplaceHandler

// NewAgentMarketplaceHandler forwards to the module constructor.
var NewAgentMarketplaceHandler = acathandler.NewAgentMarketplaceHandler
