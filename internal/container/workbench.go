package container

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/approval"
	workbenchservice "github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
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
	return session.NewWorkbenchReadHandler(runs, snapshots, ingestor).WithTaskFacts(lists).WithGrantedRuns(runs)
}

// NewWorkbenchArtifactHandler wires the artifact list + signed-link surfaces
// to the same owned-run store as the read handler. The signing secret is read
// from the environment per request; deployments without the key get an
// honest 501 instead of links signed with a default secret. The read-side
// snapshot repository doubles as the terminal-log reader, so the terminal
// availability flag is sourced from the same wiring the terminal-log
// endpoint's 501 check uses (B3-F76).
func NewWorkbenchArtifactHandler(
	runs *repository.AgentRunStore,
	messages interfaces.MessageService,
	snapshots *repository.AgentRunSnapshotRepository,
) *session.WorkbenchArtifactHandler {
	return session.NewWorkbenchArtifactHandler(runs, messages).WithTerminalLog(snapshots)
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
func NewWorkbenchAdmissionCoordinator(cfg *config.Config, db *gorm.DB, runs *repository.AgentRunStore, targets repository.ExecutionTargetStore) *workbenchservice.AdmissionCoordinator {
	coordinator, err := workbenchservice.NewAdmissionCoordinatorWithBinding(db, runs, workbenchservice.NewDurableTaskBudget(db), nil, workbenchservice.NewDatabaseAdmissionBindingResolver(targets))
	if err != nil {
		panic(err)
	}
	coordinator.SetAdmissionGate(workbenchservice.NewWorkbenchCapabilityGate(cfg))
	return coordinator
}

func NewWorkbenchStartHandler(admission *workbenchservice.AdmissionCoordinator) *session.WorkbenchStartHandler {
	return session.NewWorkbenchStartHandler(admission)
}

func NewWorkbenchInteractionStore(db *gorm.DB) *workbenchservice.GormInteractionStore {
	return workbenchservice.NewGormInteractionStore(db)
}

func NewWorkbenchInteractionService(store *workbenchservice.GormInteractionStore, gate *approval.Gate, streams interfaces.StreamManager, runs *repository.AgentRunStore, admission *workbenchservice.AdmissionCoordinator) *workbenchservice.Service {
	// Command ports are intentionally nil until the lifecycle/stream adapters
	// are supplied by the runtime container; command requests fail closed with
	// capability_unavailable rather than mutating a different subsystem.
	return workbenchservice.NewInteractionServiceWithRestart(
		store,
		workbenchservice.NewGormSteerPort(storeDB(store), streams),
		workbenchservice.NewGormCancelPort(runs),
		gate,
		workbenchservice.NewGormRunRestartPort(storeDB(store), admission),
	)
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
	enterpriseApp := strings.TrimSpace(os.Getenv("MOBILE_ENTERPRISE_APP_ID"))
	if enterpriseApp != "" && repository.ValidateMobileAppID(enterpriseApp) != nil {
		enterpriseApp = "" // 非法声明 fail closed：仅 official 通道
	}
	return handler.NewMobileDeviceHandler(store, mobileEnvironment()).
		WithMobileAppPolicy(handler.MobileAppPolicy{EnterpriseAppID: enterpriseApp})
}

// NewWorkbenchTaskStateHandler wires the task archive lifecycle to the same
// ownership predicate the read model uses; tenant/owner always come from the
// authenticated context.
func NewWorkbenchTaskStateHandler(states *repository.WorkbenchTaskStateStore) *session.WorkbenchTaskStateHandler {
	return session.NewWorkbenchTaskStateHandler(states)
}

// NewWorkbenchTaskGrantsHandler wires the task collaboration grants to the
// durable store; tenant/owner always come from the authenticated context.
func NewWorkbenchTaskGrantsHandler(
	grants *repository.TaskGrantStore,
	sessions interfaces.SessionRepository,
	members interfaces.TenantMemberRepository,
) *session.WorkbenchTaskGrantsHandler {
	return session.NewWorkbenchTaskGrantsHandler(service.NewTaskGrantService(grants, sessions, members))
}

// NewWorkbenchLegacyListHandler wires the T14 legacy task projection to the
// ownership-scoped repository; tenant/owner always come from the
// authenticated context.
func NewWorkbenchLegacyListHandler(db *gorm.DB) *session.WorkbenchLegacyListHandler {
	return session.NewWorkbenchLegacyListHandler(repository.NewWorkbenchLegacyListStore(db))
}

// NewTaskComplianceStore wires the T13 compliance store onto the shared DB
// handle (dig provider; the concrete *gorm.DB keeps dig's type graph simple).
func NewTaskComplianceStore(db *gorm.DB) *repository.TaskComplianceStore {
	return repository.NewTaskComplianceStore(db)
}

// NewTaskComplianceService wires the compliance service onto the store and
// the real audit trail. The audit service is REQUIRED — the compliance flow
// fails closed without it (Task 3).
func NewTaskComplianceService(
	store *repository.TaskComplianceStore,
	audit interfaces.AuditLogService,
) *service.TaskComplianceService {
	return service.NewTaskComplianceService(store, audit)
}

// NewWorkbenchTaskComplianceHandler wires the T13 compliance lanes to the
// service built above. Tenant/role always come from the authenticated
// context.
func NewWorkbenchTaskComplianceHandler(compliance *service.TaskComplianceService) *session.WorkbenchTaskComplianceHandler {
	return session.NewWorkbenchTaskComplianceHandler(compliance)
}

// wireTaskDeletionGuard installs the T13 legal-hold gate at the session
// deletion entrances (O03 wireCraftSessionTombstone shape).
func wireTaskDeletionGuard(handler *session.Handler, compliance *service.TaskComplianceService) {
	if handler == nil || compliance == nil {
		return
	}
	handler.SetTaskDeletionGuard(compliance)
}

// researchSourceAuthorizer gates delegated sources against the task's tenant
// knowledge scope. Production binds the tenant-scoped knowledge base lookup;
// KB-level ACLs (shares/groups) stay enforced at retrieval time by the
// existing access seam — a delegation never widens what a later read allows.
type researchSourceAuthorizer struct {
	kb interfaces.KnowledgeBaseRepository
}

// AuthorizeResearchSource rejects sources the task's tenant does not own.
func (a researchSourceAuthorizer) AuthorizeResearchSource(ctx context.Context, tenantID uint64, knowledgeBaseID string) error {
	if strings.TrimSpace(knowledgeBaseID) == "" {
		return errors.New("empty research source")
	}
	if _, err := a.kb.GetKnowledgeBaseByIDAndTenant(ctx, knowledgeBaseID, tenantID); err != nil {
		return fmt.Errorf("research source %q is outside the task's tenant knowledge scope", knowledgeBaseID)
	}
	return nil
}

// NewResearchSourceAuthorizer wires the production source gate.
func NewResearchSourceAuthorizer(db *gorm.DB) session.ResearchSourceAuthorizer {
	return researchSourceAuthorizer{kb: repository.NewKnowledgeBaseRepository(db)}
}

// NewWorkbenchResearchHandler wires the T17 (#47) delegation/annotation
// surface: the same durable run store doubles as the granted reader (owner
// first, #42 grant fallback second), annotations pin the current version
// identity derived from message-bound artifacts.
//
// Wiring note (deviation from the plan's sketch, dig-v1.19 fact verified
// empirically): the provider returns ONLY the handler — dig rejects a second
// constructor result of session.ResearchSourceAuthorizer at Provide time
// ("already provided") once NewResearchSourceAuthorizer is provided, which
// would panic the app at startup via must(). The grant service is assembled
// from the container-provided *repository.TaskGrantStore (review round 1
// added that Provide; the #42 grants handler shares the same instance) so
// both collaboration surfaces resolve through one durable store.
func NewWorkbenchResearchHandler(
	db *gorm.DB,
	runs *repository.AgentRunStore,
	messages interfaces.MessageService,
	grants *repository.TaskGrantStore,
	sessions interfaces.SessionRepository,
	members interfaces.TenantMemberRepository,
) *session.WorkbenchResearchHandler {
	return session.NewWorkbenchResearchHandler(
		runs, runs, messages,
		repository.NewTaskResearchStore(db),
		repository.NewTaskAnnotationStore(db),
		NewResearchSourceAuthorizer(db),
		service.NewTaskGrantService(grants, sessions, members),
	)
}
