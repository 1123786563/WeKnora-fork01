package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The C02 production interaction store: durable craft_interactions rows plus
// the craft_decision_outbox written in the SAME transaction as the decision.
// These tests run against the real migrated SQLite database through the
// production store, mirroring the repository craft store contract tests.

func interactionStoreDB(t *testing.T) *gorm.DB {
	t.Helper()
	return openDurableRunTestDB(t)
}

func interactionScope() craft.Scope {
	return craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
}

// seedInteractionRun admits and claims one run so interaction rows reference
// a live run through the production APIs.
func seedInteractionRun(t *testing.T, db *gorm.DB, runID string) agentruntime.Fence {
	t.Helper()
	runs := repository.NewAgentRunStore(db)
	user, err := json.Marshal(map[string]any{"role": "user", "content": "decide"})
	require.NoError(t, err)
	assistant, err := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	require.NoError(t, err)
	key := agentruntime.RunKey{TenantID: 1, RunID: runID}
	_, err = runs.Admit(context.Background(), agentruntime.Admission{
		Key: key, SessionID: "s1", UserID: "u1", RequestID: "req-" + runID,
		AssistantMessageID: "asst-" + runID, RequestHash: "rh-" + runID,
		Snapshot:    json.RawMessage(`{"version":1,"craft":true}`),
		UserMessage: user, AssistantMessage: assistant,
		Deadline: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	fence, err := runs.Claim(context.Background(), key, "worker-"+runID, time.Hour)
	require.NoError(t, err)
	return fence
}

func interactionRecord(scope craft.Scope, runID string) CraftInteractionRecord {
	return CraftInteractionRecord{
		Interaction: craft.Interaction{
			ID: "itx_store_1", Kind: craft.InteractionQuestion, ArgsHash: "iargs_a",
			Prompt: "which color?", Revision: 1,
		},
		Scope:             scope,
		RunID:             runID,
		TaskID:            "dlg_1",
		ToolCallID:        "call_1",
		PendingID:         "itx_store_1",
		OpenCodeSessionID: "oc_s1",
		OpenCodeRequestID: "qst_1",
		Pending: craft.PendingDecision{
			Interaction: craft.Interaction{ID: "itx_store_1", Kind: craft.InteractionQuestion, ArgsHash: "iargs_a", Prompt: "which color?"},
			Options:     map[string][]string{"q_color": {"red", "green"}},
			Multiple:    map[string]bool{},
		},
	}
}

func TestInteractionStoreRegisterIsIdempotentPerArguments(t *testing.T) {
	db := interactionStoreDB(t)
	store := NewGormCraftInteractionStore(db)
	ctx := context.Background()
	seedInteractionRun(t, db, "run-itx-1")

	stored, err := store.PutInteraction(ctx, interactionRecord(interactionScope(), "run-itx-1"))
	require.NoError(t, err)
	require.Equal(t, int64(1), stored.Revision)
	require.Equal(t, "pending", stored.Status)

	again, err := store.PutInteraction(ctx, interactionRecord(interactionScope(), "run-itx-1"))
	require.NoError(t, err)
	require.Equal(t, stored.ID, again.ID)
	require.Equal(t, int64(1), again.Revision, "re-registration must not bump the revision")

	changed := interactionRecord(interactionScope(), "run-itx-1")
	changed.ArgsHash = "iargs_evil"
	_, err = store.PutInteraction(ctx, changed)
	require.ErrorIs(t, err, craft.ErrConflict)
}

func TestInteractionStoreGuardsScope(t *testing.T) {
	db := interactionStoreDB(t)
	store := NewGormCraftInteractionStore(db)
	ctx := context.Background()
	seedInteractionRun(t, db, "run-itx-2")
	_, err := store.PutInteraction(ctx, interactionRecord(interactionScope(), "run-itx-2"))
	require.NoError(t, err)

	_, err = store.GetInteraction(ctx, craft.Scope{TenantID: 2, UserID: "u1", SessionID: "s1"}, "itx_store_1")
	require.ErrorIs(t, err, craft.ErrNotFound, "cross-tenant rows are invisible")
	_, err = store.GetInteraction(ctx, craft.Scope{TenantID: 1, UserID: "u2", SessionID: "s1"}, "itx_store_1")
	require.ErrorIs(t, err, craft.ErrForbidden, "cross-user rows are forbidden")
}

func decisionRequest(scope craft.Scope, decisionID string, revision int64) CraftDecisionRequest {
	return CraftDecisionRequest{
		Scope: scope, InteractionID: "itx_store_1", DecisionID: decisionID,
		Action: craft.DecisionAnswer, ArgsHash: "iargs_a", ExpectedRevision: revision,
		Answers:  []craft.Answer{{QuestionID: "q_color", Choices: []string{"red"}}},
		Operator: "u1",
	}
}

// TestInteractionStoreAppliesDecisionAndOutboxInOneTransaction: a decided
// interaction must expose BOTH the durable decision and its pending-delivery
// outbox row; the recorded answers survive for re-display.
func TestInteractionStoreAppliesDecisionAndOutboxInOneTransaction(t *testing.T) {
	db := interactionStoreDB(t)
	store := NewGormCraftInteractionStore(db)
	ctx := context.Background()
	seedInteractionRun(t, db, "run-itx-3")
	_, err := store.PutInteraction(ctx, interactionRecord(interactionScope(), "run-itx-3"))
	require.NoError(t, err)

	applied, err := store.ApplyInteractionDecision(ctx, decisionRequest(interactionScope(), "dec_1", 1))
	require.NoError(t, err)
	require.Equal(t, "decided", applied.Status)
	require.Equal(t, "pending", applied.Delivery)
	require.Equal(t, "dec_1", applied.DecisionID)
	require.Equal(t, "u1", applied.DecidedBy, "the operator identity is recorded for audit")
	require.Len(t, applied.RecordedAnswers, 1)
	require.Equal(t, "q_color", applied.RecordedAnswers[0].QuestionID)

	pending, err := store.PendingDecisions(ctx, agentruntime.RunKey{TenantID: 1, RunID: "run-itx-3"})
	require.NoError(t, err)
	require.Len(t, pending, 1, "the decision must carry one pending outbox item")
	require.Equal(t, "dec_1", pending[0].DecisionID)
	require.Equal(t, "itx_store_1", pending[0].InteractionID)
	require.Equal(t, "qst_1", pending[0].OpenCodeRequestID)
}

// TestInteractionStoreDecisionIdempotencyAndConflicts: replaying the same
// decision id with the same payload returns the original result; the same id
// with a different payload, or a competing revision, answers ErrConflict; a
// decision on an already-decided interaction answers ErrGone (410).
func TestInteractionStoreDecisionIdempotencyAndConflicts(t *testing.T) {
	db := interactionStoreDB(t)
	store := NewGormCraftInteractionStore(db)
	ctx := context.Background()
	seedInteractionRun(t, db, "run-itx-4")
	_, err := store.PutInteraction(ctx, interactionRecord(interactionScope(), "run-itx-4"))
	require.NoError(t, err)

	// A competing revision on a still-pending interaction is a conflict (409).
	_, err = store.ApplyInteractionDecision(ctx, decisionRequest(interactionScope(), "dec_wrong_rev", 7))
	require.ErrorIs(t, err, craft.ErrConflict, "a stale expected revision is a conflict")

	first, err := store.ApplyInteractionDecision(ctx, decisionRequest(interactionScope(), "dec_1", 1))
	require.NoError(t, err)

	replay, err := store.ApplyInteractionDecision(ctx, decisionRequest(interactionScope(), "dec_1", 1))
	require.NoError(t, err)
	require.Equal(t, first.DecisionID, replay.DecisionID)
	require.Equal(t, first.Revision, replay.Revision, "an idempotent replay must not bump the revision")

	different := decisionRequest(interactionScope(), "dec_1", 1)
	different.Answers = []craft.Answer{{QuestionID: "q_color", Choices: []string{"green"}}}
	_, err = store.ApplyInteractionDecision(ctx, different)
	require.ErrorIs(t, err, craft.ErrConflict, "same decision id with a different payload is a conflict")

	_, err = store.ApplyInteractionDecision(ctx, decisionRequest(interactionScope(), "dec_2", 1))
	require.ErrorIs(t, err, craft.ErrGone, "an already-decided interaction is terminal (410)")

	_, err = store.ApplyInteractionDecision(ctx, decisionRequest(interactionScope(), "dec_2", 2))
	require.ErrorIs(t, err, craft.ErrGone, "an already-decided interaction is terminal (410)")

	pending, err := store.PendingDecisions(ctx, agentruntime.RunKey{TenantID: 1, RunID: "run-itx-4"})
	require.NoError(t, err)
	require.Len(t, pending, 1, "no duplicate outbox item may appear")
}

// TestInteractionStoreQuestionAnswerIsNotApproval: a question interaction
// never accepts an approve action, keeping question answers from ever
// becoming permission grants.
func TestInteractionStoreQuestionAnswerIsNotApproval(t *testing.T) {
	db := interactionStoreDB(t)
	store := NewGormCraftInteractionStore(db)
	ctx := context.Background()
	seedInteractionRun(t, db, "run-itx-5")
	_, err := store.PutInteraction(ctx, interactionRecord(interactionScope(), "run-itx-5"))
	require.NoError(t, err)

	approve := decisionRequest(interactionScope(), "dec_1", 1)
	approve.Action = craft.DecisionApprove
	_, err = store.ApplyInteractionDecision(ctx, approve)
	require.ErrorIs(t, err, craft.ErrInvalidInput)
}

// TestInteractionStoreDeliveryMarkersAndListing: ack and unknown markers move
// the outbox and interaction delivery states; the session listing exposes the
// pending decision with its question payload and, once decided, the recorded
// answers for restart re-display.
func TestInteractionStoreDeliveryMarkersAndListing(t *testing.T) {
	db := interactionStoreDB(t)
	store := NewGormCraftInteractionStore(db)
	ctx := context.Background()
	seedInteractionRun(t, db, "run-itx-6")
	_, err := store.PutInteraction(ctx, interactionRecord(interactionScope(), "run-itx-6"))
	require.NoError(t, err)

	listed, err := store.ListInteractions(ctx, interactionScope(), "s1")
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, "pending", listed[0].Status)
	require.NotNil(t, listed[0].Pending.Options, "the question payload must survive registration")
	require.Contains(t, listed[0].Pending.Options["q_color"], "red")

	_, err = store.ApplyInteractionDecision(ctx, decisionRequest(interactionScope(), "dec_1", 1))
	require.NoError(t, err)
	require.NoError(t, store.MarkInteractionDelivery(ctx, interactionScope(), "itx_store_1", "unknown"))

	marked, err := store.ListInteractions(ctx, interactionScope(), "s1")
	require.NoError(t, err)
	require.Equal(t, "unknown", marked[0].Delivery)
	require.Len(t, marked[0].RecordedAnswers, 1, "the original answers survive for re-display")

	// Unknown stays takeable for redelivery; ack removes it.
	require.NoError(t, store.AckDecision(ctx, agentruntime.RunKey{TenantID: 1, RunID: "run-itx-6"}, "itx_store_1", "dec_1", ""))
	none, err := store.PendingDecisions(ctx, agentruntime.RunKey{TenantID: 1, RunID: "run-itx-6"})
	require.NoError(t, err)
	require.Empty(t, none)
	final, err := store.GetInteraction(ctx, interactionScope(), "itx_store_1")
	require.NoError(t, err)
	require.Equal(t, "delivered", final.Delivery)
}
