package container

import (
	"fmt"

	"github.com/Tencent/WeKnora/internal/handler/session"
)

// RegisterCraftFeature installs a lane-owned handler on the registry owned by
// one container/router assembly. Routes inherit that router's authenticated
// session group and services remain responsible for Task authorization.
func RegisterCraftFeature(routes *session.CraftFeatureRoutes, name string, mount func(session.CraftRouteGroup)) error {
	if routes == nil || mount == nil {
		return fmt.Errorf("Craft feature %q unavailable", name)
	}
	return routes.Register(name, mount)
}
