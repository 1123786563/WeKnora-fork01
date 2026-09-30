package container

import (
	"fmt"

	"github.com/Tencent/WeKnora/internal/handler/session"
)

// RegisterCraftFeature installs a lane-owned handler only after its service
// has been assembled. T20 invokes registrations before router construction.
// A missing handler fails assembly; routes inherit the authenticated session
// group and the handler/service remains responsible for Task authorization.
func RegisterCraftFeature(name string, mount func(session.CraftRouteGroup)) error {
	if mount == nil {
		return fmt.Errorf("Craft feature %q unavailable", name)
	}
	return session.RegisterCraftFeatureRoute(name, mount)
}
