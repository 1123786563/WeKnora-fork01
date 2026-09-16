package workbench

import (
	"errors"
	"strings"
)

// InteractionKind is the permission domain of an interactive execution
// prompt. Domains are intentionally disjoint: accepting a tool never grants
// budget and extending budget never resolves a recovery prompt.
type InteractionKind string

const (
	InteractionToolApproval InteractionKind = "tool_approval"
	InteractionBudget       InteractionKind = "budget"
	InteractionRecovery     InteractionKind = "recovery"
)

var ErrInteractionActionMismatch = errors.New("interaction_action_mismatch")
var ErrCommandActionMismatch = errors.New("command_action_mismatch")

// InteractionDecision is the wire-safe, optimistic-concurrency input for a
// decision. The service fills identity and ownership from its durable record;
// client supplied kind/args_hash are consistency hints only.
type InteractionDecision struct {
	ID               string `json:"id"`
	DecisionID       string `json:"decision_id"`
	Kind             string `json:"kind"`
	Action           string `json:"action"`
	ArgsHash         string `json:"args_hash"`
	ExpectedRevision int64  `json:"expected_revision"`
}

func ValidateInteractionAction(kind, action string) error {
	kind = strings.TrimSpace(kind)
	action = strings.TrimSpace(action)
	allowed := map[InteractionKind]map[string]bool{
		InteractionToolApproval: {"approve": true, "reject": true},
		InteractionBudget:       {"extend": true},
		InteractionRecovery:     {"retry": true, "provide_result": true, "terminate": true},
	}
	if !allowed[InteractionKind(kind)][action] {
		return ErrInteractionActionMismatch
	}
	return nil
}

// ExecutionCommand is a closed union. cancel has no payload; steer carries a
// text payload which is delivered through the existing steer queue.
type ExecutionCommand struct {
	Action           string `json:"action"`
	Text             string `json:"text,omitempty"`
	ExpectedRevision int64  `json:"expected_revision"`
}

func (c ExecutionCommand) Validate() error {
	switch strings.TrimSpace(c.Action) {
	case "cancel":
		if strings.TrimSpace(c.Text) != "" {
			return ErrCommandActionMismatch
		}
	case "steer":
		if strings.TrimSpace(c.Text) == "" {
			return ErrCommandActionMismatch
		}
	default:
		return ErrCommandActionMismatch
	}
	if c.ExpectedRevision < 0 {
		return ErrCommandActionMismatch
	}
	return nil
}
