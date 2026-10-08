package craft

// Route names of the post-failure reconciliation program. The vocabulary is
// closed and deliberately carries no automatic-resubmission branch: a write
// that may already have been performed (prompt possibly accepted, external
// side effects possibly running) can never be safely re-issued by software
// — the product constraint is "check first, wait durably when undeterminable;
// neither an SSE disconnect nor a prompt timeout re-sends a task that may
// already have executed".
const (
	// RecoveryRouteReuse answers from the already-persisted result.
	RecoveryRouteReuse = "reuse"
	// RecoveryRouteCollect settles from the R04 full-snapshot Completed
	// predicate: the exact prompt round is verifiably finished remotely.
	RecoveryRouteCollect = "collect"
	// RecoveryRouteObserve keeps watching a still-running remote execution,
	// bounded by the delegation's absolute deadline.
	RecoveryRouteObserve = "observe"
	// RecoveryRouteWait parks the run durably on an existing waiting reason
	// until a human decision resolves it. It is the default fallback.
	RecoveryRouteWait = "wait"
)

// RecoveryFacts are the durable, verifiable facts about one delegated
// sub-execution after a process failure. Every fact must come from persisted
// state or a verified runtime snapshot; callers must never infer a fact from
// a tool name, a timeout, or the absence of an error.
type RecoveryFacts struct {
	// ResultStored: the delegation already has a persisted terminal result.
	ResultStored bool
	// ExactCompleted: the R04 full-snapshot predicate proved this exact
	// prompt round finished successfully (only opencode.Completed supplies
	// this; nothing else may set it).
	ExactCompleted bool
	// RemoteRunning: a verified snapshot shows the remote sub-execution is
	// still actively working this session (not idle / tool pending).
	RemoteRunning bool
	// VersionCompatible: the workspace's recorded runtime digest still
	// matches the runtime the recovery is configured to trust. An image
	// change invalidates observation comparability.
	VersionCompatible bool
	// WorkspaceAvailable: the workspace binding exists, is authorized for
	// the rebuilt scope and still carries a live sandbox generation with a
	// bound OpenCode session. An empty session must never be continued.
	WorkspaceAvailable bool
}

// RecoveryRoute chooses the only safe next operation for one delegated
// sub-execution after a process failure:
//
//   - a stored result is always reused (a crash after persistence must not
//     lose or duplicate the outcome);
//   - an incompatible runtime or an unavailable workspace waits before any
//     remote fact is trusted (no execution against an empty session, no
//     collection through a changed image);
//   - only the exact Completed snapshot collects;
//   - only a verified still-running remote is observed;
//   - everything else waits: unknown never resolves to a resubmission.
func RecoveryRoute(f RecoveryFacts) string {
	if f.ResultStored {
		return RecoveryRouteReuse
	}
	if !f.VersionCompatible || !f.WorkspaceAvailable {
		return RecoveryRouteWait
	}
	if f.ExactCompleted {
		return RecoveryRouteCollect
	}
	if f.RemoteRunning {
		return RecoveryRouteObserve
	}
	return RecoveryRouteWait
}
