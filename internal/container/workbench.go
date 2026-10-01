package container

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appservice "github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/approval"
	workbenchservice "github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"go.uber.org/dig"
)

// NewWorkbenchReadHandler wires the ownership facade to the same durable run
// store used by workers. Keeping this provider in the container prevents a
// second, unscoped read path from being introduced by HTTP handlers.
func NewWorkbenchReadHandler(
	runs *repository.AgentRunStore,
	snapshots *repository.AgentRunSnapshotRepository,
	ingestor *repository.ExecutionObservationStore,
	lists *repository.WorkbenchListStore,
) *session.WorkbenchReadHandler {
	return session.NewWorkbenchReadHandler(runs, snapshots, ingestor).WithTaskFacts(lists)
}

// NewWorkbenchArtifactHandler wires the artifact list + signed-link surfaces
// to the same owned-run store as the read handler. The signing secret is read
// from the environment per request; deployments without the key get an
// honest 501 instead of links signed with a default secret.
func NewWorkbenchArtifactHandler(
	runs *repository.AgentRunStore,
	messages interfaces.MessageService,
	snapshots *repository.AgentRunSnapshotRepository,
	versions *repository.ArtifactVersionStore,
) *session.WorkbenchArtifactHandler {
	return session.NewWorkbenchArtifactHandler(runs, messages).
		WithTerminalLog(snapshots).
		WithArtifactVersions(versions).
		WithArtifactVersionRevoker(versions)
}

// NewWorkbenchListHandler wires the mobile workbench list to the ownership-
// scoped repository; tenant/owner always come from the authenticated context.
func NewWorkbenchListHandler(lists *repository.WorkbenchListStore) *session.WorkbenchListHandler {
	return session.NewWorkbenchListHandler(lists)
}

// NewWorkbenchAdmissionCoordinator keeps budget admission and durable run
// creation behind one DI seam. Deployments with a credit ledger can replace
// the no-op budget adapter without changing HTTP or repository code.
// The W34 capability gate (workbench.worker_drain / platform_admission) is
// installed here so every NEW admission consults the switches before any
// budget reservation or durable write; already-admitted runs and cleanup
// stay untouched (drain semantics). W24 additionally wires the durable
// credit budget and the database-backed admission binding resolver so only
// trusted execution-target usage binds at admission time.
func NewWorkbenchAdmissionCoordinator(cfg *config.Config, db *gorm.DB, runs *repository.AgentRunStore, targets repository.ExecutionTargetStore, adoptions repository.AgentAdoptionRepository, security *appservice.AgentSecurityService) *workbenchservice.AdmissionCoordinator {
	coordinator, err := workbenchservice.NewAdmissionCoordinatorWithBinding(db, runs, workbenchservice.NewDurableTaskBudget(db), nil, workbenchservice.NewDatabaseAdmissionBindingResolver(targets))
	if err != nil {
		panic(err)
	}
	coordinator.SetAdmissionGate(workbenchservice.NewWorkbenchCapabilityGate(cfg))
	coordinator.SetAgentUseGate(func(ctx context.Context, tenantID uint64, agentID string) error {
		retired, err := adoptions.RetiredVariantAgentExists(ctx, tenantID, agentID)
		if err != nil {
			return err
		}
		if retired {
			return fmt.Errorf("%w: agent %s belongs to a retired variant", workbenchservice.ErrAgentUseDenied, agentID)
		}
		return nil
	})
	if security != nil {
		coordinator.SetAgentSecurityGate(security)
		coordinator.SetPublishedAgentVersionResolver(security)
	}
	return coordinator
}

func NewWorkbenchStartHandler(admission *workbenchservice.AdmissionCoordinator) *session.WorkbenchStartHandler {
	return session.NewWorkbenchStartHandler(admission)
}

func NewWorkbenchInteractionStore(db *gorm.DB) *workbenchservice.GormInteractionStore {
	return workbenchservice.NewGormInteractionStore(db)
}

func NewWorkbenchInteractionService(store *workbenchservice.GormInteractionStore, gate *approval.Gate, streams interfaces.StreamManager, runs *repository.AgentRunStore, admission *workbenchservice.AdmissionCoordinator) *workbenchservice.Service {
	db := storeDB(store)
	return workbenchservice.NewInteractionServiceWithRestart(store, workbenchservice.NewGormSteerPort(db, streams), workbenchservice.NewGormCancelPort(runs), gate, workbenchservice.NewGormRunRestartPort(db, admission))
}

func NewResearchSourceAuthorizer(db *gorm.DB) session.ResearchSourceAuthorizer {
	knowledgeBases := repository.NewKnowledgeBaseRepository(db)
	return researchSourceAuthorizerFunc(func(ctx context.Context, tenantID uint64, id string) error {
		_, err := knowledgeBases.GetKnowledgeBaseByIDAndTenant(ctx, id, tenantID)
		if errors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			return session.ErrResearchSourceOutOfScope
		}
		return err
	})
}

type researchSourceAuthorizerFunc func(context.Context, uint64, string) error

func (f researchSourceAuthorizerFunc) AuthorizeResearchSource(ctx context.Context, tenantID uint64, id string) error {
	return f(ctx, tenantID, id)
}

func NewWorkbenchResearchHandler(db *gorm.DB, runs *repository.AgentRunStore, messages interfaces.MessageService, sessions interfaces.SessionRepository, members interfaces.TenantMemberRepository) *session.WorkbenchResearchHandler {
	grants := appservice.NewTaskGrantService(repository.NewTaskGrantStore(db), sessions, members)
	return session.NewWorkbenchResearchHandler(runs, runs, messages, repository.NewTaskResearchStore(db), repository.NewTaskAnnotationStore(db), NewResearchSourceAuthorizer(db), grants)
}

func NewWorkbenchLegacyListHandler(db *gorm.DB) *session.WorkbenchLegacyListHandler {
	return session.NewWorkbenchLegacyListHandler(repository.NewWorkbenchLegacyListStore(db))
}

func NewWorkbenchTaskGrantsHandler(grants *repository.TaskGrantStore, sessions interfaces.SessionRepository, members interfaces.TenantMemberRepository) *session.WorkbenchTaskGrantsHandler {
	return session.NewWorkbenchTaskGrantsHandler(appservice.NewTaskGrantService(grants, sessions, members))
}

func NewTaskComplianceStore(db *gorm.DB) *repository.TaskComplianceStore {
	return repository.NewTaskComplianceStore(db)
}
func NewTaskComplianceService(store *repository.TaskComplianceStore, audit interfaces.AuditLogService) *appservice.TaskComplianceService {
	return appservice.NewTaskComplianceService(store, audit)
}
func NewWorkbenchTaskComplianceHandler(compliance *appservice.TaskComplianceService) *session.WorkbenchTaskComplianceHandler {
	return session.NewWorkbenchTaskComplianceHandler(compliance)
}

func wireTaskDeletionGuard(handler *session.Handler, compliance *appservice.TaskComplianceService) {
	if handler != nil && compliance != nil {
		handler.SetTaskDeletionGuard(compliance)
	}
}

// storeDB is kept in the service constructor's dependency graph through the
// concrete adapter; it is intentionally private to the container package.
func storeDB(store *workbenchservice.GormInteractionStore) *gorm.DB {
	if store == nil {
		return nil
	}
	return store.DB()
}

func NewWorkbenchOverviewService(db *gorm.DB) *workbenchservice.OverviewService {
	return workbenchservice.NewWorkbenchOverviewService(db, nil)
}

func NewWorkbenchOverviewHandler(overview *workbenchservice.OverviewService) *session.WorkbenchOverviewHandler {
	return session.NewWorkbenchOverviewHandler(overview)
}

func NewWorkbenchInboxHandler(db *gorm.DB) *session.WorkbenchInboxHandler {
	return session.NewWorkbenchInboxHandler(session.NewWorkbenchInboxService(db, nil))
}

func NewWorkbenchCommandHandler(interactions *workbenchservice.Service) *session.WorkbenchCommandHandler {
	return session.NewWorkbenchCommandHandler(interactions)
}

func mobileEnvironment() string {
	if value := strings.TrimSpace(os.Getenv("WEKNORA_MOBILE_ENVIRONMENT")); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("APP_ENV")); value != "" {
		return value
	}
	return "development"
}

func NewMobileDeviceStore(db *gorm.DB) *repository.MobileDeviceStore {
	return repository.NewMobileDeviceStore(db, mobileEnvironment())
}

func NewMobileDeviceHandler(store *repository.MobileDeviceStore) *handler.MobileDeviceHandler {
	return handler.NewMobileDeviceHandler(store, mobileEnvironment())
}

// NewWorkbenchTaskStateHandler wires the task archive lifecycle to the same
// ownership predicate the read model uses; tenant/owner always come from the
// authenticated context.
func NewWorkbenchTaskStateHandler(states *repository.WorkbenchTaskStateStore) *session.WorkbenchTaskStateHandler {
	return session.NewWorkbenchTaskStateHandler(states)
}

// provideWorkbenchTaskHandlers registers the collaboration, research, legacy,
// and compliance handler graph used by both production assembly and its graph test.
func provideWorkbenchTaskHandlers(container *dig.Container) error {
	providers := []interface{}{
		NewResearchSourceAuthorizer,
		NewWorkbenchResearchHandler,
		NewWorkbenchLegacyListHandler,
		repository.NewTaskGrantStore,
		NewWorkbenchTaskGrantsHandler,
		repository.NewTaskResearchStore,
		repository.NewTaskAnnotationStore,
		repository.NewWorkbenchLegacyListStore,
		NewTaskComplianceStore,
		NewTaskComplianceService,
		NewWorkbenchTaskComplianceHandler,
	}
	for _, provider := range providers {
		if err := container.Provide(provider); err != nil {
			return err
		}
	}
	return nil
}
