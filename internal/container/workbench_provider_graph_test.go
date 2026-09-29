package container

import (
	"reflect"
	"testing"

	"go.uber.org/dig"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appservice "github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type workbenchProviderGraphParams struct {
	dig.In
	Grants     *session.WorkbenchTaskGrantsHandler
	Research   *session.WorkbenchResearchHandler
	LegacyList *session.WorkbenchLegacyListHandler
	Compliance *session.WorkbenchTaskComplianceHandler
}

type complianceAuditStub struct{ interfaces.AuditLogService }

func TestWorkbenchProviderGraph(t *testing.T) {
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
	provide(func() interfaces.AuditLogService { return complianceAuditStub{} })
	provide(repository.NewAgentRunStore)
	provide(repository.NewSessionRepository)
	provide(repository.NewTenantMemberRepository)
	provide(repository.NewTaskGrantStore)
	provide(NewWorkbenchTaskGrantsHandler)
	provide(NewResearchSourceAuthorizer)
	provide(NewWorkbenchResearchHandler)
	provide(NewWorkbenchLegacyListHandler)
	provide(NewTaskComplianceStore)
	provide(NewTaskComplianceService)
	provide(NewWorkbenchTaskComplianceHandler)

	if err := c.Invoke(func(p workbenchProviderGraphParams) {
		if p.Grants == nil || p.Research == nil || p.LegacyList == nil || p.Compliance == nil {
			t.Errorf("required workbench graph handlers must all resolve: grants=%v research=%v legacy=%v compliance=%v", p.Grants != nil, p.Research != nil, p.LegacyList != nil, p.Compliance != nil)
		}
	}); err != nil {
		t.Fatalf("required workbench provider graph did not resolve: %v", err)
	}
}

func TestWireTaskDeletionGuardUsesProvidedComplianceService(t *testing.T) {
	h := &session.Handler{}
	compliance := appservice.NewTaskComplianceService(nil, nil)
	wireTaskDeletionGuard(h, compliance)

	guard := reflect.ValueOf(h).Elem().FieldByName("taskDeletionGuard")
	if !guard.IsValid() || guard.IsNil() {
		t.Fatal("deletion guard was not installed")
	}
	if guard.Elem().Pointer() != reflect.ValueOf(compliance).Pointer() {
		t.Fatal("deletion guard does not delegate to the provided compliance service")
	}
}
