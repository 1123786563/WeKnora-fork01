package container

import (
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// newSessionForkService builds the session-fork service from the repositories.
// The sandbox snapshot port stays nil until the per-session manager resolution
// chain (A17 workspace_checkpointer/pinned-sandbox family) exposes fork
// snapshots; per the upstream contract a nil port makes every sandbox-carrying
// fork degrade to a fresh sandbox instead of failing, while message-only forks
// copy history and lineage unchanged.
func newSessionForkService(
	sessions interfaces.SessionRepository,
	messages interfaces.MessageRepository,
) *service.SessionForkService {
	return service.NewSessionForkServiceFromRepos(sessions, messages, nil)
}

// newWorkspaceCheckpointer adapts the process-wide Manager. This deployment's
// process default is the DisabledManager (per-session backends resolve at
// request time), so the type assertion fails and a nil checkpointer is
// returned — the trpc completion hook then skips checkpointing, exactly like
// an upstream deployment without a sandbox backend.
func newWorkspaceCheckpointer(mgr sandbox.Manager) *service.WorkspaceCheckpointer {
	if runner, ok := mgr.(service.SandboxShellRunner); ok {
		return service.NewWorkspaceCheckpointer(runner)
	}
	return nil
}

// newSandboxIDLookup follows the same adaptation: nil on sandbox-less
// deployments so the checkpoint hook no-ops.
func newSandboxIDLookup(mgr sandbox.Manager) session.SandboxIDLookup {
	if lookup, ok := mgr.(session.SandboxIDLookup); ok {
		return lookup
	}
	return nil
}
