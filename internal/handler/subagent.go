package handler

import (
	agentcataloghandler "github.com/Tencent/WeKnora/internal/agentcatalog/handler"
)

// Pass B (25a) transitional shim — the implementation moved to
// internal/agentcatalog/handler/subagent.go. Consumers are switched
// to the module package by IB2, after which this file is deleted
// (12-commercial §5.4 pattern; transition deviation registered per
// conventions §1.5 / framework:29).

// SubagentHandler aliases the module type; the alias also keeps the
// &handler.SubagentHandler{} zero-value literal in routes_subagent_test.go
// legal.
type SubagentHandler = agentcataloghandler.SubagentHandler

// NewSubagentHandler forwards to the module constructor.
var NewSubagentHandler = agentcataloghandler.NewSubagentHandler
