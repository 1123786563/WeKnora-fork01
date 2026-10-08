package container

import (
	"fmt"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/craft"
)

// craftTaskAccessChecker exposes the same persistent service through the
// worker's combined classification and authorization port.
func craftTaskAccessChecker(svc *service.CraftAccessService) craft.TaskRunAccess {
	return svc
}

// craftTaskAccessNarrowPort exposes the very same service instance through the
// narrow action-check port. dig cannot downcast interfaces, so assemblies that
// require craft.TaskAccessChecker get it from the combined port explicitly.
func craftTaskAccessNarrowPort(access craft.TaskRunAccess) craft.TaskAccessChecker {
	return access
}

func wireCraftAccessFeature(
	svc *service.CraftAccessService,
	routes *session.CraftFeatureRoutes,
) error {
	if svc == nil {
		return fmt.Errorf("Craft access service unavailable")
	}
	if routes == nil {
		return fmt.Errorf("Craft access feature routes unavailable")
	}
	return session.RegisterCraftAccessFeature(routes, svc)
}

// registerCraftAccessFeature runs before router construction. Registration
// failures are returned to dig.Invoke and fail application assembly closed.
func registerCraftAccessFeature(svc *service.CraftAccessService, routes *session.CraftFeatureRoutes) error {
	return wireCraftAccessFeature(svc, routes)
}
