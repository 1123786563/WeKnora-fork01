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
