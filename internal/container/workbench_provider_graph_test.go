package container

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"go.uber.org/dig"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appservice "github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

func TestResearchSourceAuthorizerPreservesLookupErrors(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-mem db: %v", err)
	}
	if err := db.AutoMigrate(&types.KnowledgeBase{}); err != nil {
		t.Fatalf("migrate knowledge bases: %v", err)
	}
	authorizer := NewResearchSourceAuthorizer(db)

	// A missing row and a row owned by another tenant both appear as the
	// repository's scoped not-found sentinel and are intentionally concealed.
	if err := authorizer.AuthorizeResearchSource(context.Background(), 7, "missing"); !errors.Is(err, session.ErrResearchSourceOutOfScope) {
		t.Fatalf("missing source error = %v, want out-of-scope", err)
	}
	if err := db.Create(&types.KnowledgeBase{ID: "other-tenant", TenantID: 8}).Error; err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}
	if err := authorizer.AuthorizeResearchSource(context.Background(), 7, "other-tenant"); !errors.Is(err, session.ErrResearchSourceOutOfScope) {
		t.Fatalf("cross-tenant source error = %v, want out-of-scope", err)
	}

	// A database failure must remain distinguishable so the handler returns
	// its backend 500 response instead of misreporting a scope rejection.
	if err := db.Migrator().DropTable(&types.KnowledgeBase{}); err != nil {
		t.Fatalf("drop knowledge bases: %v", err)
	}
	infraErr := authorizer.AuthorizeResearchSource(context.Background(), 7, "missing")
	if infraErr == nil || errors.Is(infraErr, session.ErrResearchSourceOutOfScope) {
		t.Fatalf("infrastructure error = %v, want original lookup failure", infraErr)
	}
}

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
	if err := provideWorkbenchTaskHandlers(c); err != nil {
		t.Fatalf("register production workbench task handlers: %v", err)
	}

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
