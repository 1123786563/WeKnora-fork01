package container

import (
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/handler/session"
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
