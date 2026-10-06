package container

import (
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// The process-wide Manager is DisabledManager, so the runner, ID lookup, and
// fork snapshot port go through the session pin + per-tenant resolver — the
// same path ArtifactCollector already uses. Direct Manager type-asserts still
// win when a deployment injects a SessionBoundManager as the process default.
func newPinnedSessionSandbox(
	mgr sandbox.Manager,
	resolver sandbox.TenantSandboxResolver,
	pinner *service.SessionSandboxPinner,
) *service.PinnedSessionSandbox {
	return service.NewPinnedSessionSandbox(pinner, resolver, mgr)
}

// newSessionForkService builds the session-fork service from the repositories.
// The sandbox snapshot port resolves the session's pinned manager at request
// time; per the upstream contract a nil port (no pin / sandbox-less
// deployment) makes every sandbox-carrying fork degrade to a fresh sandbox
// instead of failing, while message-only forks copy history and lineage
// unchanged.
func newSessionForkService(
	sessions interfaces.SessionRepository,
	messages interfaces.MessageRepository,
	mgr sandbox.Manager,
	pinned *service.PinnedSessionSandbox,
) *service.SessionForkService {
	if port, ok := mgr.(service.SessionForkSandboxPort); ok {
		return service.NewSessionForkServiceFromRepos(sessions, messages, port)
	}
	if pinned == nil {
		return service.NewSessionForkServiceFromRepos(sessions, messages, nil)
	}
	return service.NewSessionForkServiceFromRepos(sessions, messages, pinned)
}

// newWorkspaceCheckpointer prefers a process-wide shell runner, falling back
// to the pinned per-session resolver so turn checkpoints reach the session's
// own sandbox. With neither (DisabledManager and no pin) a nil checkpointer is
// returned — the trpc completion hook then skips checkpointing, exactly like
// an upstream deployment without a sandbox backend.
func newWorkspaceCheckpointer(
	mgr sandbox.Manager,
	pinned *service.PinnedSessionSandbox,
) *service.WorkspaceCheckpointer {
	if runner, ok := mgr.(service.SandboxShellRunner); ok {
		return service.NewWorkspaceCheckpointer(runner)
	}
	if pinned == nil {
		return nil
	}
	return service.NewWorkspaceCheckpointer(pinned)
}

// newSandboxIDLookup follows the same adaptation: the pinned resolver reports
// the session's bound sandbox, nil keeps the checkpoint hook a no-op.
func newSandboxIDLookup(
	mgr sandbox.Manager,
	pinned *service.PinnedSessionSandbox,
) session.SandboxIDLookup {
	if lookup, ok := mgr.(session.SandboxIDLookup); ok {
		return lookup
	}
	if pinned == nil {
		return nil
	}
	return pinned
}
