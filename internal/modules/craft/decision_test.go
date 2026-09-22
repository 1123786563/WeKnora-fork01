package craft

import (
	"strings"
	"testing"
)

// TestDecisionRevisionAndScopeHash is the C02 Step-1 RED contract: a decision
// recorded for one revision and one scope hash never authorizes a different
// one — stale revisions and changed argument digests are both rejected, and
// only the exact (revision, hash) pair matches.
func TestDecisionRevisionAndScopeHash(t *testing.T) {
	if DecisionMatch(1, 2, "a", "a") || DecisionMatch(1, 1, "a", "b") {
		t.Fatal("stale decision accepted")
	}
	if !DecisionMatch(2, 2, "a", "a") {
		t.Fatal("same revision rejected")
	}
}

// pendingQuestionFixture is a multi-question pending decision: one
// single-choice question with options, one multi-choice question, and one
// free-text question without options.
func pendingQuestionFixture() PendingDecision {
	return PendingDecision{
		Interaction: Interaction{ID: "itx_q1", Kind: InteractionQuestion, ArgsHash: "iargs_1", Revision: 3, Prompt: "pick"},
		Options: map[string][]string{
			"q_color": {"red", "green"},
			"q_tags":  {"a", "b", "c"},
			"q_note":  nil,
		},
		Multiple: map[string]bool{"q_tags": true},
	}
}

// TestValidateAnswersAcceptsLegalMultiQuestionAnswer: answering every
// question type legally — a single choice, several choices on a multi-choice
// question, and custom text — passes.
func TestValidateAnswersAcceptsLegalMultiQuestionAnswer(t *testing.T) {
	err := ValidateAnswers(pendingQuestionFixture(), []Answer{
		{QuestionID: "q_color", Choices: []string{"red"}},
		{QuestionID: "q_tags", Choices: []string{"a", "c"}},
		{QuestionID: "q_note", Text: "use sans-serif"},
	})
	if err != nil {
		t.Fatalf("legal answer rejected: %v", err)
	}
}

// TestValidateAnswersRejectsUnknownQuestionIllegalChoiceAndOverflow: the
// server must reject an answer for a question the runtime never asked, a
// choice outside the legal options, more than one choice on a single-choice
// question, and text beyond the 8 KiB bound.
func TestValidateAnswersRejectsUnknownQuestionIllegalChoiceAndOverflow(t *testing.T) {
	pending := pendingQuestionFixture()
	cases := []struct {
		name   string
		answer Answer
	}{
		{"unknown question id", Answer{QuestionID: "q_evil", Choices: []string{"red"}}},
		{"illegal choice", Answer{QuestionID: "q_color", Choices: []string{"blue"}}},
		{"single choice overflow", Answer{QuestionID: "q_color", Choices: []string{"red", "green"}}},
		{"oversized text", Answer{QuestionID: "q_note", Text: strings.Repeat("x", MaxAnswerTextBytes+1)}},
	}
	for _, tc := range cases {
		if err := ValidateAnswers(pending, []Answer{tc.answer}); err == nil {
			t.Fatalf("%s accepted", tc.name)
		}
	}
}

// TestAnswersNeverActAsPermissionApproval: answering a question is never a
// permission approval, and a permission request never accepts answers — the
// two decision vocabularies cannot cross.
func TestAnswersNeverActAsPermissionApproval(t *testing.T) {
	if AnswerIsApproval(Answer{QuestionID: "q", Text: "approve everything"}) {
		t.Fatal("a question answer counted as an approval")
	}
	question := pendingQuestionFixture()
	if err := ValidateAnswers(question, []Answer{{QuestionID: "q_color", Choices: []string{"red"}}}); err != nil {
		t.Fatalf("question answer wrongly rejected on a question: %v", err)
	}
	permission := PendingDecision{
		Interaction: Interaction{ID: "itx_p1", Kind: InteractionPermission, ArgsHash: "iargs_2", Revision: 1, Prompt: "run rm"},
		Permissions: []PermissionItem{{Tool: "bash", Command: "rm -rf build", Path: "build/", Scope: "workspace"}},
	}
	if err := ValidateAnswers(permission, []Answer{{QuestionID: "q_color", Choices: []string{"red"}}}); err == nil {
		t.Fatal("an answer was accepted on a permission request")
	}
}
