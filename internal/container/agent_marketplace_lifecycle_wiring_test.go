package container

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/handler"
	"go.uber.org/dig"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type lifecycleRouterParamsProbe struct {
	dig.In
	LifecycleHandler *handler.AgentMarketplaceLifecycleHandler
}

func TestAgentMarketplaceLifecycleHandlerResolvesForRouterParams(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}

	c := dig.New()
	provide := func(constructor interface{}) {
		t.Helper()
		if err := c.Provide(constructor); err != nil {
			t.Fatalf("provide %T: %v", constructor, err)
		}
	}
	provide(func() *gorm.DB { return db })
	provide(repository.NewAgentAdoptionRepository)
	provide(repository.NewAgentMarketplaceRepository)
	// Mirror the production lifecycle provider chain. Resolve the same
	// required handler field RouterParams asks Dig to build at startup.
	provideAgentMarketplaceLifecycle(c)

	err = c.Invoke(func(params lifecycleRouterParamsProbe) {
		if params.LifecycleHandler == nil {
			t.Error("lifecycle handler resolved as nil; lifecycle routes would not be available")
		}
	})
	if err != nil {
		t.Fatalf("container could not resolve lifecycle handler required by RouterParams: %v", err)
	}
}
