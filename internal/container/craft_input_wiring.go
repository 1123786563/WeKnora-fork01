package container

import (
	"fmt"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler/session"
)

func wireCraftInputFeature(svc *service.CraftSessionService, routes *session.CraftFeatureRoutes) error {
	if svc == nil {
		return fmt.Errorf("Craft session service unavailable")
	}
	if routes == nil {
		return fmt.Errorf("Craft input feature routes unavailable")
	}
	return RegisterCraftFeature(routes, "input", func(group session.CraftRouteGroup) {
		session.RegisterCraftInputRoutes(group, session.NewCraftInputHandler(svc))
	})
}

// registerCraftInputFeature runs before router construction. Feature routes
// belong to the current container assembly and failures fail application setup.
func registerCraftInputFeature(svc *service.CraftSessionService, routes *session.CraftFeatureRoutes) error {
	return wireCraftInputFeature(svc, routes)
}
