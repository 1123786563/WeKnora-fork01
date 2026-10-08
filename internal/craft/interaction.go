package craft

// Interaction kinds and the user actions each kind accepts. The pairs are
// exhaustive: a question is answered or rejected, a permission request is
// approved or rejected, and nothing else is a user decision.
const (
	InteractionQuestion   = "question"
	InteractionPermission = "permission"

	DecisionAnswer  = "answer"
	DecisionApprove = "approve"
	DecisionReject  = "reject"
)

// Interaction is one durable question or permission request raised by a
// delegated sub-execution while the main run keeps waiting for a human.
// ArgsHash digests the arguments the user saw, so a decision recorded for
// one payload can never authorize a different one; Revision supports
// compare-and-swap decision writes.
type Interaction struct {
	ID       string
	Kind     string
	ArgsHash string
	Prompt   string
	Revision int64
}

// DecisionAllowed reports whether the (kind, action) pair is a real user
// decision. A question is never approved — "approve" on a question, an
// unknown kind or an unknown action is rejected, so no caller can turn a
// question into a silent permission grant.
func DecisionAllowed(kind, action string) bool {
	return (kind == "question" && (action == "answer" || action == "reject")) ||
		(kind == "permission" && (action == "approve" || action == "reject"))
}

// StopStatus reports the verifiable stop phase of a delegated execution
// after a stop was requested. "canceled" requires the runtime to confirm
// both the abort and an idle session; anything less keeps "stopping" — an
// accepted abort HTTP call alone never claims cancellation. Without a stop
// request the execution is "running".
func StopStatus(requested bool, o Observation) string {
	if requested && o.Aborted && o.Idle {
		return "canceled"
	}
	if requested {
		return "stopping"
	}
	return "running"
}
