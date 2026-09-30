package session

import (
	"fmt"
	"sort"
	"sync"
)

// CraftFeatureRoutes is a construction-time registry. Each feature receives
// only the existing authenticated /sessions route group; it cannot mount an
// unguarded root route through this seam. Services still check Task access.
type CraftFeatureRoutes struct {
	mu     sync.Mutex
	frozen bool
	routes map[string]func(CraftRouteGroup)
}

func NewCraftFeatureRoutes() *CraftFeatureRoutes {
	return &CraftFeatureRoutes{routes: make(map[string]func(CraftRouteGroup))}
}

func (r *CraftFeatureRoutes) hasFeatures() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.routes) > 0
}

func (r *CraftFeatureRoutes) Register(name string, mount func(CraftRouteGroup)) error {
	if r == nil || mount == nil || name == "" {
		return fmt.Errorf("invalid Craft feature route")
	}
	if name[0] < 'a' || name[0] > 'z' {
		return fmt.Errorf("invalid Craft feature name %q", name)
	}
	for _, ch := range name {
		if ch < 'a' || ch > 'z' {
			if ch < '0' || ch > '9' {
				if ch != '_' && ch != '-' {
					return fmt.Errorf("invalid Craft feature name %q", name)
				}
			}
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return fmt.Errorf("Craft feature routes already mounted")
	}
	if _, exists := r.routes[name]; exists {
		return fmt.Errorf("duplicate Craft feature %q", name)
	}
	r.routes[name] = mount
	return nil
}

func (r *CraftFeatureRoutes) Mount(group CraftRouteGroup) error {
	if r == nil || group == nil {
		return fmt.Errorf("Craft feature route group unavailable")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frozen = true
	names := make([]string, 0, len(r.routes))
	for name := range r.routes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		r.routes[name](group)
	}
	return nil
}

var registeredCraftFeatureRoutes = NewCraftFeatureRoutes()

// RegisterCraftFeatureRoute is called by the container before router build.
// Failed registration must fail assembly rather than silently omit a feature.
func RegisterCraftFeatureRoute(name string, mount func(CraftRouteGroup)) error {
	return registeredCraftFeatureRoutes.Register(name, mount)
}
