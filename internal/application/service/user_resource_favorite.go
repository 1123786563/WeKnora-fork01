package service

import (
	agentcatalogservice "github.com/Tencent/WeKnora/internal/modules/agentcatalog/service"
)

// Pass B (25a) transitional shim — the implementation moved to
// internal/modules/agentcatalog/service/user_resource_favorite.go. Consumers
// are switched to the module package by IB2, after which this file is deleted
// (12-commercial §5.4 pattern; transition deviation registered per
// conventions §1.5 / framework:29).

// NewUserResourceFavoriteService forwards to the module constructor.
var NewUserResourceFavoriteService = agentcatalogservice.NewUserResourceFavoriteService

// Sentinel forwarding keeps errors.Is identity with the module vars.
var (
	ErrFavoriteInvalidType = agentcatalogservice.ErrFavoriteInvalidType
	ErrFavoriteEmptyID     = agentcatalogservice.ErrFavoriteEmptyID
)
