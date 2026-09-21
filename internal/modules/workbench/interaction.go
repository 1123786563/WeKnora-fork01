package workbench

import (
	"errors"
	"fmt"
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
	ID                string `json:"id"`
	RunID             string `json:"run_id,omitempty"`
	DecisionID        string `json:"decision_id"`
	Kind              string `json:"kind"`
	Action            string `json:"action"`
	ArgsHash          string `json:"args_hash"`
	ExpectedRevision  int64  `json:"expected_revision"`
	ExternalPendingID string `json:"external_pending_id,omitempty"`
	CredentialVersion int64  `json:"credential_version,omitempty"`
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

// Validate is the shared admission rule: identity fields non-empty, kind/action
// matrix holds, revision stays in the safe range. It is intentionally stricter
// than the wire (the handler takes id from the URL param and the service layer
// overwrites input.ID) so client-side self-checks fail closed.
func (d InteractionDecision) Validate() error {
	if strings.TrimSpace(d.ID) == "" {
		return invalid("id", "required")
	}
	if strings.TrimSpace(d.DecisionID) == "" {
		return invalid("decision_id", "required")
	}
	if strings.TrimSpace(d.ArgsHash) == "" {
		return invalid("args_hash", "required")
	}
	if err := ValidateInteractionAction(d.Kind, d.Action); err != nil {
		return err
	}
	if d.ExpectedRevision < 0 || d.ExpectedRevision > MaxSafeInteger {
		return fmt.Errorf("%w: expected_revision", ErrSequenceOverflow)
	}
	return nil
}

// ExecutionCommand is a closed union. cancel has no payload; steer carries a
// text payload which is delivered through the existing steer queue.
type ExecutionCommand struct {
	Action            string `json:"action"`
	Text              string `json:"text,omitempty"`
	ExpectedRevision  int64  `json:"expected_revision"`
	ExternalPendingID string `json:"external_pending_id,omitempty"`
	CredentialVersion int64  `json:"credential_version,omitempty"`
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
