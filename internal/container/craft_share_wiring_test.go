package container

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newTestCraftShareService builds a minimal-but-real share service through
// the production constructor (inert stubs for the read ports; the wiring
// assertions never exercise their bodies).
func newTestCraftShareService(t *testing.T) *service.CraftShareService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	svc, err := service.NewCraftShareService(service.CraftShareConfig{
		DB:         db,
		Versions:   shareStubVersions{},
		Files:      shareStubFiles{},
		Records:    shareStubRecords{},
		TaskAccess: shareStubAccess{},
		Now:        func() time.Time { return time.Now() },
	})
	require.NoError(t, err)
	return svc
}

type shareStubVersions struct{ craft.VersionStore }

func (shareStubVersions) List(context.Context, craft.Scope) ([]craft.Version, error) {
	return nil, nil
}

type shareStubFiles struct{ interfaces.FileService }

type shareStubRecords struct {
	service.CraftCitationRecordReader
}

func (shareStubRecords) Load(context.Context, craft.Scope, string) (craft.KnowledgeRecord, error) {
	return craft.KnowledgeRecord{}, craft.ErrNotFound
}

type shareStubAccess struct{ craft.TaskAccessChecker }

func (shareStubAccess) CheckTaskAccess(context.Context, craft.Scope, craft.TaskAction) error {
	return craft.ErrForbidden
}

// TestCraftShareFeatureAssemblyRegistersRoutes pins the T11 central wiring:
// the feature registration mounts the share feature name into the T00
// constrained registry (fail-closed on a nil service, like the knowledge
// precedent), and a real service instance registers cleanly.
func TestCraftShareFeatureAssemblyRegistersRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	routes := session.NewCraftFeatureRoutes()
	// A nil service is refused by the registration seam (fail-closed).
	require.Error(t, registerCraftShareFeature(nil, routes))

	// A real service instance registers the "share" feature cleanly.
	// Registry introspection is unexported, so the observable proof is the
	// mount itself: the registry now has features and mounting onto a real
	// constrained group reaches the share routes (any registration error
	// would have failed above).
	svc := newTestCraftShareService(t)
	require.NoError(t, registerCraftShareFeature(svc, routes))

	// Mount onto a real gin group: the three share routes exist as static
	// siblings of the existing version-file routes.
	engine := gin.New()
	group := engine.Group("/sessions")
	require.NoError(t, routes.Mount(group))
	recorded := map[string]bool{}
	for _, route := range engine.Routes() {
		recorded[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"GET /sessions/:id/craft/versions/:version_id/share",
		"POST /sessions/:session_id/craft/versions/:version_id/share/decision",
		"POST /sessions/:session_id/craft/versions/:version_id/share/revocation",
	} {
		require.True(t, recorded[want], "share route %s must be mounted", want)
	}
}
