package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/logger"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
)

// CraftOutboxItem is one durable-but-undelivered user decision: the exact
// interaction, action and answers the user confirmed, the revision and
// argument digest they decided against, and the OpenCode addresses the
// decision must be forwarded to.
type CraftOutboxItem struct {
	TenantID          uint64
	OwnerID           string
	SessionID         string
	RunID             string
	InteractionID     string
	PendingID         string
	DecisionID        string
	Kind              string
	Action            string
	ArgsHash          string
	ExpectedRevision  int64
	Answers           []craft.Answer
	Prompt            string
	OpenCodeSessionID string
	OpenCodeRequestID string
	Delivery          string // pending | unknown
	Attempts          int
	DecidedBy         string
}

// CraftDecisionOutboxRo is the durable outbox read surface the reliable
// delivery worker consumes: undelivered items by run key, plus the honest
// delivery-state transitions (ack, or delivery_unknown when the protocol
// cannot confirm).
type CraftDecisionOutboxRo interface {
	PendingDecisions(ctx context.Context, key agentruntime.RunKey) ([]CraftOutboxItem, error)
	AckDecision(ctx context.Context, key agentruntime.RunKey, interactionID, decisionID, note string) error
	MarkDecisionUnknown(ctx context.Context, key agentruntime.RunKey, interactionID, decisionID, note string) error
}

// CraftDecisionDelivery is the C02 reliable decision delivery worker. Every
// user decision is durable before any forward happens; this worker takes the
// undelivered outbox items under one run fence, re-checks that the durable
// interaction still carries exactly that decision (revision + argument
// digest), forwards it over the locked OpenCode reply routes, and marks the
// ack only on a protocol-confirmed success. A forward the protocol cannot
// confirm is kept as an honest delivery_unknown — never fabricated as
// accepted — and the run stays parked at waiting_user with no goroutine or
// worker lease held; redelivery rides the existing recovery scheduling.
type CraftDecisionDelivery struct {
	interactions CraftInteractionStore
	outbox       CraftDecisionOutboxRo
	reply        CraftOpenCodeReplier
	runs         CraftRunController
}

// NewCraftDecisionDelivery assembles the delivery worker. A nil replier keeps
// every delivery honestly unknown (fail-closed), never a silent success.
func NewCraftDecisionDelivery(
	interactions CraftInteractionStore,
	outbox CraftDecisionOutboxRo,
	reply CraftOpenCodeReplier,
	runs CraftRunController,
) *CraftDecisionDelivery {
	return &CraftDecisionDelivery{interactions: interactions, outbox: outbox, reply: reply, runs: runs}
}

// Deliver processes the run's undelivered decisions. pendingID == "" is the
// sweep mode: only delivery=pending items (an unknown item needs an explicit
// targeted redelivery). The call holds no lease and blocks no goroutine: it
// reads, forwards with a bounded context, and records the honest marker.
func (d *CraftDecisionDelivery) Deliver(ctx context.Context, key agentruntime.RunKey, pendingID string) error {
	if d == nil || d.outbox == nil || d.interactions == nil {
		return fmt.Errorf("%w: decision delivery is not assembled", craft.ErrInvalidInput)
	}
	items, err := d.outbox.PendingDecisions(ctx, key)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	// The run must still be live: a terminal run keeps its durable decision
	// but nothing is forwarded into a dead execution.
	if d.runs != nil {
		run, getErr := d.runs.Get(ctx, key)
		if getErr != nil {
			return getErr
		}
		switch run.Status {
		case "succeeded", "failed", "canceled":
			return fmt.Errorf("%w: run %s is %s; decision %s stays recorded but is not delivered",
				craft.ErrConflict, key.RunID, run.Status, firstDecisionID(items))
		}
	}
	var unconfirmed []string
	for _, item := range items {
		if pendingID != "" && item.PendingID != pendingID {
			continue
		}
		if pendingID == "" && item.Delivery != "pending" {
			continue // sweep mode only retries never-attempted items
		}
		if note, ok := d.deliverOne(ctx, key, item); !ok {
			unconfirmed = append(unconfirmed, note)
		}
	}
	if len(unconfirmed) > 0 {
		return fmt.Errorf("%w: decision delivery unconfirmed: %s", craft.ErrUnknown, strings.Join(unconfirmed, "; "))
	}
	return nil
}

// deliverOne forwards one item. ok=false means the delivery could not be
// confirmed and was marked delivery_unknown.
func (d *CraftDecisionDelivery) deliverOne(ctx context.Context, key agentruntime.RunKey, item CraftOutboxItem) (string, bool) {
	// 1. Re-check the durable interaction still carries exactly this
	//    decision: same decision id, same action, and the argument digest the
	//    user decided on (revision advanced by exactly the decision write).
	interaction, err := d.interactions.GetInteraction(ctx, craft.Scope{
		TenantID: item.TenantID, UserID: item.OwnerID, SessionID: item.SessionID,
	}, item.InteractionID)
	if err != nil {
		// The owner check needs the real scope; fall back to listing is not
		// available here, so treat an unreadable interaction as unconfirmed.
		d.markUnknown(ctx, key, item, fmt.Sprintf("interaction recheck failed: %v", err))
		return item.DecisionID + " recheck failed", false
	}
	if interaction.DecisionID != item.DecisionID || interaction.DecidedAction != item.Action ||
		interaction.ArgsHash != item.ArgsHash || interaction.Revision != item.ExpectedRevision+1 {
		note := fmt.Sprintf("interaction %s no longer carries decision %s (superseded or changed); not forwarded",
			item.InteractionID, item.DecisionID)
		d.markUnknown(ctx, key, item, note)
		return note, false
	}

	// 2. Forward over the locked reply route with a bounded context.
	forwardCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), craftControlBudget)
	defer cancel()
	var forwardErr error
	if d.reply == nil {
		forwardErr = errors.New("no OpenCode forwarder is wired")
	} else {
		switch {
		case item.Kind == craft.InteractionQuestion && item.Action == craft.DecisionAnswer:
			forwardErr = d.reply.ReplyQuestion(forwardCtx, item.OpenCodeRequestID, answerRows(item.Answers))
		case item.Kind == craft.InteractionQuestion:
			forwardErr = d.reply.RejectQuestion(forwardCtx, item.OpenCodeRequestID)
		case item.Action == craft.DecisionApprove:
			forwardErr = d.reply.ReplyPermission(forwardCtx, item.OpenCodeSessionID, item.OpenCodeRequestID, "once")
		default:
			forwardErr = d.reply.ReplyPermission(forwardCtx, item.OpenCodeSessionID, item.OpenCodeRequestID, "reject")
		}
	}
	if forwardErr == nil {
		if err := d.outbox.AckDecision(ctx, key, item.InteractionID, item.DecisionID, ""); err != nil {
			logger.Warnf(ctx, "[CraftDelivery] ack persistence failed for %s: %v", item.DecisionID, err)
		}
		logger.Infof(ctx, "[CraftDelivery] decision %s delivered to OpenCode request %s (run %s, decided_by %s)",
			item.DecisionID, item.OpenCodeRequestID, item.RunID, item.DecidedBy)
		return "", true
	}

	// 3. The protocol could not confirm. First re-read the durable state: a
	//    concurrent worker may already have acked, and that result stands.
	if current, listErr := d.outbox.PendingDecisions(ctx, key); listErr == nil {
		stillPending := false
		for _, row := range current {
			if row.InteractionID == item.InteractionID && row.DecisionID == item.DecisionID {
				stillPending = true
			}
		}
		if !stillPending {
			return "", true // confirmed elsewhere
		}
	}
	// 4. Honestly unknown: the decision is recorded, the delivery is not
	//    confirmed. The run keeps its waiting_user park; no fabricated
	//    acceptance, no lease held.
	note := fmt.Sprintf("decision %s recorded; OpenCode delivery unconfirmed: %v", item.DecisionID, forwardErr)
	d.markUnknown(ctx, key, item, note)
	return note, false
}

func (d *CraftDecisionDelivery) markUnknown(ctx context.Context, key agentruntime.RunKey, item CraftOutboxItem, note string) {
	if err := d.outbox.MarkDecisionUnknown(ctx, key, item.InteractionID, item.DecisionID, note); err != nil {
		logger.Warnf(ctx, "[CraftDelivery] unknown-marker persistence failed for %s: %v", item.DecisionID, err)
	}
}

// answerRows maps the recorded answers onto the locked answers:[[string]]
// question reply shape: one row per answered question, choices first and the
// custom text appended.
func answerRows(answers []craft.Answer) [][]string {
	rows := make([][]string, 0, len(answers))
	for _, answer := range answers {
		row := append([]string(nil), answer.Choices...)
		if answer.Text != "" {
			row = append(row, answer.Text)
		}
		rows = append(rows, row)
	}
	return rows
}

func firstDecisionID(items []CraftOutboxItem) string {
	if len(items) == 0 {
		return "-"
	}
	return items[0].DecisionID
}
