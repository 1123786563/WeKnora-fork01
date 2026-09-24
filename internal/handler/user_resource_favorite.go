package handler

import (
	agentcataloghandler "github.com/Tencent/WeKnora/internal/modules/agentcatalog/handler"
)

// Pass B (25a) transitional shim — the implementation moved to
// internal/modules/agentcatalog/handler/user_resource_favorite.go. Consumers
// are switched to the module package by IB2, after which this file is deleted
// (12-commercial §5.4 pattern; transition deviation registered per
// conventions §1.5 / framework:29).

// UserResourceFavoriteHandler aliases the module type so router field and
// route-registration signatures keep compiling unchanged.
type UserResourceFavoriteHandler = agentcataloghandler.UserResourceFavoriteHandler

// NewUserResourceFavoriteHandler forwards to the module constructor.
var NewUserResourceFavoriteHandler = agentcataloghandler.NewUserResourceFavoriteHandler
