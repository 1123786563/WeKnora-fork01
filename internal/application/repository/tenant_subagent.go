package repository

import (
	agentcatalogrepository "github.com/Tencent/WeKnora/internal/agentcatalog/repository"
)

// Pass B (25a) transitional shim — the implementation moved to
// internal/agentcatalog/repository/tenant_subagent.go. Consumers are
// switched to the module package by IB2, after which this file is deleted
// (12-commercial §5.4 pattern; transition deviation registered per
// conventions §1.5 / framework:29).

// TenantSubagentRepository is an alias of the module interface definition.
type TenantSubagentRepository = agentcatalogrepository.TenantSubagentRepository

// NewTenantSubagentRepository forwards to the module constructor.
var NewTenantSubagentRepository = agentcatalogrepository.NewTenantSubagentRepository
