package container

import (
	"github.com/Tencent/WeKnora/internal/application/repository"
	workbenchservice "github.com/Tencent/WeKnora/internal/application/service/workbench"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"gorm.io/gorm"
)

// NewWorkbenchReadHandler wires the ownership facade to the same durable run
// store used by workers. Keeping this provider in the container prevents a
// second, unscoped read path from being introduced by HTTP handlers.
func NewWorkbenchReadHandler(
	runs *repository.AgentRunStore,
	snapshots *repository.AgentRunSnapshotRepository,
) *session.WorkbenchReadHandler {
	return session.NewWorkbenchReadHandler(runs, snapshots)
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
