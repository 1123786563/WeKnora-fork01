package container

import (
	"fmt"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/modules/craft"
)

// craftTaskAccessChecker exposes the same persistent policy instance through
// the narrow port used by Craft services that need Task authorization.
func craftTaskAccessChecker(svc *service.CraftAccessService) craft.TaskAccessChecker {
	return svc
}

func wireCraftAccessFeature(
	svc *service.CraftAccessService,
	register func(string, func(session.CraftRouteGroup)) error,
) error {
	if svc == nil {
		return fmt.Errorf("Craft access service unavailable")
	}
	if register == nil {
		return fmt.Errorf("Craft access feature registrar unavailable")
	}
	return register("access", func(group session.CraftRouteGroup) {
		session.RegisterCraftAccessRoutes(group, session.NewCraftAccessHandler(svc))
	})
}

// registerCraftAccessFeature runs before router construction. Registration
// failures are returned to dig.Invoke and fail application assembly closed.
func registerCraftAccessFeature(svc *service.CraftAccessService) error {
	return wireCraftAccessFeature(svc, RegisterCraftFeature)
}
