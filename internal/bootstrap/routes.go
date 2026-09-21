// Package bootstrap defines the composition contracts used to wire route
// modules and background workers into the server. Feature packages depend
// on these small interfaces instead of concrete registration plumbing;
// production wiring migrates onto them incrementally.
package bootstrap

import "github.com/gin-gonic/gin"

// RouteModule attaches one feature's HTTP routes to a Gin router group.
// Implementations must only register routes on the group they are given —
// never on package-level or global router state — so composition stays
// instance-scoped and duplicate registrations surface at wiring time.
type RouteModule interface {
	RegisterRoutes(group *gin.RouterGroup)
}
