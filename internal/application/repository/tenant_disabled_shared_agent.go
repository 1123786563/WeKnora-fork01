package repository

import (
	agentcatalogrepository "github.com/Tencent/WeKnora/internal/modules/agentcatalog/repository"
)

// Pass B (25a) transitional shim — the implementation moved to
// internal/modules/agentcatalog/repository/tenant_disabled_shared_agent.go.
// Consumers are switched to the module package by IB2, after which this file
// is deleted (12-commercial §5.4 pattern; transition deviation registered per
// conventions §1.5 / framework:29).

// NewTenantDisabledSharedAgentRepository forwards to the module constructor.
var NewTenantDisabledSharedAgentRepository = agentcatalogrepository.NewTenantDisabledSharedAgentRepository
