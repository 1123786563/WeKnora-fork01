package container

import (
	"github.com/Tencent/WeKnora/internal/agent/approval"
	"github.com/Tencent/WeKnora/internal/application/repository"
	workbenchservice "github.com/Tencent/WeKnora/internal/application/service/workbench"
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

// NewWorkbenchArtifactHandler wires the artifact list + signed-link surfaces
// to the same owned-run store as the read handler. The signing secret is read
// from the environment per request; deployments without the key get an
// honest 501 instead of links signed with a default secret.
func NewWorkbenchArtifactHandler(
	runs *repository.AgentRunStore,
	messages interfaces.MessageService,
) *session.WorkbenchArtifactHandler {
	return session.NewWorkbenchArtifactHandler(runs, messages)
}

// NewWorkbenchAdmissionCoordinator keeps budget admission and durable run
// creation behind one DI seam. Deployments with a credit ledger can replace
// the no-op budget adapter without changing HTTP or repository code.
func NewWorkbenchAdmissionCoordinator(db *gorm.DB, runs *repository.AgentRunStore) *workbenchservice.AdmissionCoordinator {
	return workbenchservice.NewAdmissionCoordinator(db, runs, workbenchservice.NoopTaskBudget{}, nil)
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
