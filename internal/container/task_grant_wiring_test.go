package container

import (
	"testing"

	"go.uber.org/dig"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// T17 (#47) review round 1: pin the workbench collaboration wiring.
//
// NewWorkbenchTaskGrantsHandler was Provided since #42, but its
// *repository.TaskGrantStore dependency never was — the provider was
// unbuildable, the optional RouterParams.WorkbenchTaskGrantsHandler field
// silently resolved to nil, and RegisterWorkbenchTaskGrantRoutes returned
// early: the three /workbench/tasks/:task_id/grants routes never mounted in
// the production assembly. A missing provider only surfaces when the process
// starts (same reason TestRetrieveEngineRegistryWiring pins its graph), so
// the provider subset both collaboration handlers stand on is pinned here.

// wiringMessageServiceStub satisfies interfaces.MessageService for
// construction only; dig never calls its methods while wiring the handlers.
type wiringMessageServiceStub struct{ interfaces.MessageService }

// taskCollabWiringParams mirrors the consumed slice of RouterParams with
// optionality REMOVED. Production marks both fields `optional:"true"`, which
// turns an unbuildable provider into a silent nil and a silently unmounted
// route tree — the exact failure mode under review. Requesting the handlers
// as required fields makes any missing provider fail this test loudly
// instead of silently at deploy time.
type taskCollabWiringParams struct {
	dig.In

	Grants   *session.WorkbenchTaskGrantsHandler
	Research *session.WorkbenchResearchHandler
}

func TestTaskGrantsAndResearchHandlersBuildable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-mem db: %v", err)
	}

	c := dig.New()
	provide := func(constructor interface{}) {
		t.Helper()
		if err := c.Provide(constructor); err != nil {
			t.Fatalf("provide %T: %v", constructor, err)
		}
	}
	provide(func() *gorm.DB { return db })
	provide(func() interfaces.MessageService { return wiringMessageServiceStub{} })
	provide(repository.NewAgentRunStore)
	// Review round 1 fix: the grant store must be Provided or both
	// collaboration providers below are unbuildable (grants routes silently
	// dark; research routes would be too under the shared-service wiring).
	provide(repository.NewTaskGrantStore)
	provide(repository.NewSessionRepository)
	provide(repository.NewTenantMemberRepository)
	provide(NewWorkbenchTaskGrantsHandler)
	provide(NewResearchSourceAuthorizer)
	provide(NewWorkbenchResearchHandler)

	err = c.Invoke(func(p taskCollabWiringParams) {
		if p.Grants == nil {
			t.Error("WorkbenchTaskGrantsHandler built as nil — /workbench/tasks/:task_id/grants routes would silently not mount")
		}
		if p.Research == nil {
			t.Error("WorkbenchResearchHandler built as nil — research/annotation routes would silently not mount")
		}
	})
	if err != nil {
		t.Fatalf("container could not build the workbench collaboration handlers: %v", err)
	}
}
