package runtime

// RecoveryFacts are durable facts about one logical tool call. A capability
// flag is trusted only when the tool implementation supplied it; callers must
// not infer safety from a tool name or MCP annotation.
type RecoveryFacts struct {
	HasResult     bool
	NotDispatched bool
	Queryable     bool
	Idempotent    bool
	KeyValid      bool
	ReadOnly      bool
}

// RecoveryAction chooses the only safe next operation for a tool call.
//
// Open-connector app tool calls (T14) ride the default discipline without a
// dedicated policy: the registry withholds the tool from the EAGER dispatch
// boundary (the tool calls BeforeToolDispatch itself, only after its gates,
// right before an observable answer), so a journaled OC call is either
// planned (NotDispatched → execute: replay re-prepares idempotently under
// the (tenant, session, tool_call) binding and re-queries the action state)
// or carries a committed result (reuse). An awaiting_approval or unknown
// ACTION state is surfaced as ErrOCActionApprovalWait BEFORE any attempt and
// parks at the existing waiting_user — resolution is the human approval or
// the read-only provider query, never a re-dispatch, and no indefinite
// idempotency is claimed on the action's dispatch itself (the replay window
// belongs to the durable dispatch record, T10).
func RecoveryAction(f RecoveryFacts) string {
	switch {
	case f.HasResult:
		return "reuse"
	case f.NotDispatched:
		return "execute"
	case f.Queryable:
		return "query"
	case f.Idempotent && f.KeyValid:
		return "retry"
	case f.ReadOnly:
		return "retry"
	default:
		return "wait_user"
	}
}
