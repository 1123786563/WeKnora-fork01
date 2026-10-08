package craft

import (
	"errors"
	"fmt"
)

// C02 decision contracts: the pure rules a durable user decision obeys at
// every layer (HTTP validation, control service, delivery worker). A decision
// is only ever compared against the exact pending interaction it was recorded
// for: same revision AND same argument digest. Anything else is stale and
// must be rejected, so a decision for one payload can never authorize a
// different question, permission or argument set.

// ErrGone reports that the addressed interaction is canceled or otherwise
// terminal: the decision target no longer accepts user input (HTTP 410).
var ErrGone = errors.New("craft gone")

// MaxAnswerTextBytes bounds one custom (free-text) answer. 8 KiB keeps a
// question answer a short structured answer, not a document upload.
const MaxAnswerTextBytes = 8 << 10

// Answer is one answered question inside a pending decision: the question id
// the runtime raised, the chosen options and/or the custom text answer.
type Answer struct {
	QuestionID string
	Choices    []string
	Text       string
}

// PermissionItem is one itemized permission fact the user approves: the
// command (or tool) asked for, the path it touches and the scope it applies
// to. The first phase only offers approve-once and reject — never a sticky
// session grant — so the item is display data, not a grant record.
type PermissionItem struct {
	Tool    string
	Command string
	Path    string
	Scope   string
}

// PendingDecision is the full pending question or permission payload the
// user decides on. Options maps each question id to its legal choices;
// Multiple marks which questions accept more than one choice; Answers carries
// previously recorded answers so a restart re-displays the original answer
// state; Permissions itemizes a permission request (command/path/scope).
type PendingDecision struct {
	Interaction
	Options     map[string][]string
	Multiple    map[string]bool
	Answers     []Answer
	Permissions []PermissionItem
}

// DecisionMatch reports whether the pending interaction a decision was
// recorded for is still the exact one in front of the user: the revision must
// be current and the argument digest must be non-empty and equal. A stale
// revision, a changed digest or an empty expected digest never matches.
func DecisionMatch(expected, current int64, expectedHash, currentHash string) bool {
	return expected == current && expectedHash != "" && expectedHash == currentHash
}

// ValidateAnswers checks submitted answers against the pending question set:
// every answer must address a question id the runtime actually raised, every
// choice must be one of that question's legal options, a single-choice
// question accepts at most one choice, and custom text is bounded by
// MaxAnswerTextBytes. Answers are only valid on a question — a permission
// request never takes answers.
func ValidateAnswers(pending PendingDecision, answers []Answer) error {
	if pending.Kind != InteractionQuestion {
		return fmt.Errorf("%w: answers only apply to a question, not a %s", ErrInvalidInput, pending.Kind)
	}
	for _, answer := range answers {
		if answer.QuestionID == "" {
			return fmt.Errorf("%w: answer is missing the question id", ErrInvalidInput)
		}
		options, known := pending.Options[answer.QuestionID]
		if !known {
			return fmt.Errorf("%w: unknown question id %q", ErrInvalidInput, answer.QuestionID)
		}
		limit := 1
		if pending.Multiple[answer.QuestionID] {
			limit = len(options)
		}
		if len(answer.Choices) > limit {
			return fmt.Errorf("%w: question %q accepts at most %d choice(s), got %d",
				ErrInvalidInput, answer.QuestionID, limit, len(answer.Choices))
		}
		for _, choice := range answer.Choices {
			legal := false
			for _, option := range options {
				if option == choice {
					legal = true
					break
				}
			}
			if !legal {
				return fmt.Errorf("%w: choice %q is not an option of question %q", ErrInvalidInput, choice, answer.QuestionID)
			}
		}
		if len(answer.Text) > MaxAnswerTextBytes {
			return fmt.Errorf("%w: answer text for %q exceeds %d bytes", ErrInvalidInput, answer.QuestionID, MaxAnswerTextBytes)
		}
	}
	return nil
}

// AnswerIsApproval is always false: answering a question is never a
// permission approval, no matter how the answer text reads. Callers use this
// to keep the two decision vocabularies from ever crossing.
func AnswerIsApproval(Answer) bool { return false }
