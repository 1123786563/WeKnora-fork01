package container

import (
	"os"
	"strings"

	"github.com/Tencent/WeKnora/internal/agent/approval"
	"github.com/Tencent/WeKnora/internal/application/repository"
	workbenchservice "github.com/Tencent/WeKnora/internal/application/service/workbench"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/handler/session"
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
) *session.WorkbenchReadHandler {
	return session.NewWorkbenchReadHandler(runs, snapshots, ingestor)
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
// stay untouched (drain semantics).
func NewWorkbenchAdmissionCoordinator(cfg *config.Config, db *gorm.DB, runs *repository.AgentRunStore) *workbenchservice.AdmissionCoordinator {
	coordinator := workbenchservice.NewAdmissionCoordinator(db, runs, workbenchservice.NoopTaskBudget{}, nil)
	coordinator.SetAdmissionGate(workbenchservice.NewWorkbenchCapabilityGate(cfg))
	return coordinator
}

func NewWorkbenchStartHandler(admission *workbenchservice.AdmissionCoordinator) *session.WorkbenchStartHandler {
	return session.NewWorkbenchStartHandler(admission)
}

func NewWorkbenchInteractionStore(db *gorm.DB) *workbenchservice.GormInteractionStore {
	return workbenchservice.NewGormInteractionStore(db)
}

func NewWorkbenchInteractionService(store *workbenchservice.GormInteractionStore, gate *approval.Gate, streams interfaces.StreamManager) *workbenchservice.Service {
	// Command ports are intentionally nil until the lifecycle/stream adapters
	// are supplied by the runtime container; command requests fail closed with
	// capability_unavailable rather than mutating a different subsystem.
	return workbenchservice.NewInteractionServiceWithApproval(store, workbenchservice.NewGormSteerPort(storeDB(store), streams), workbenchservice.NewGormCancelPort(storeDB(store)), gate)
}

// storeDB is kept in the service constructor's dependency graph through the
// concrete adapter; it is intentionally private to the container package.
func storeDB(store *workbenchservice.GormInteractionStore) *gorm.DB {
	if store == nil {
		return nil
	}
	return store.DB()
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
