package workbench

import (
	"context"
	"errors"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
)

// NotificationProjector translates durable run events into device-scoped
// intents. It has no provider dependency and never waits for a push vendor.
type NotificationProjector struct {
	runs          *repository.AgentRunStore
	notifications *repository.NotificationStore
}

func NewNotificationProjector(runs *repository.AgentRunStore, notifications *repository.NotificationStore) *NotificationProjector {
	return &NotificationProjector{runs: runs, notifications: notifications}
}

// Project reads a bounded event page and returns the last sequence observed.
// The caller persists this cursor only after the transaction has committed;
// rerunning the page is safe because NotificationStore is idempotent.
func (p *NotificationProjector) Project(ctx context.Context, key agentruntime.RunKey, after int64, limit int) (int64, error) {
	if p == nil || p.runs == nil || p.notifications == nil {
		return after, agentruntime.ErrConflict
	}
	run, err := p.runs.Get(ctx, key)
	if err != nil {
		return after, err
	}
	events, err := p.runs.ReadEvents(ctx, key, after, limit)
	if errors.Is(err, agentruntime.ErrCursorExpired) {
		after, events, err = p.recoverExpiredCursor(ctx, key, after, limit, err)
	}
	if err != nil {
		return after, err
	}
	last := after
	for _, evt := range events {
		if err := p.notifications.ProjectEvent(ctx, repository.RunNotificationEvent{
			TenantID: key.TenantID, OwnerID: run.UserID, RunID: key.RunID,
			Seq: evt.Seq, Type: evt.Type,
		}); err != nil {
			return last, err
		}
		if evt.Seq > last {
			last = evt.Seq
		}
	}
	return last, nil
}

// recoverExpiredCursor is the checkpoint consumer's answer to the T18
// (#137) gap rule. An SSE client replies to ErrCursorExpired by reloading
// the authoritative snapshot; THIS consumer has no snapshot channel, and
// notifications are best-effort fan-out — the honest recovery is to resume
// from the retained window head: the events inside the hole are permanently
// gone either way, and projecting from the head loses exactly them (the
// pre-gap-rule outcome) instead of failing the run on every worker tick and
// starving every lexicographically later run's projection.
func (p *NotificationProjector) recoverExpiredCursor(ctx context.Context, key agentruntime.RunKey, after int64, limit int, readErr error) (int64, []agentruntime.RunEvent, error) {
	if !errors.Is(readErr, agentruntime.ErrCursorExpired) {
		return after, nil, readErr
	}
	// HolePositions are relative to the cursor of the read that PRODUCED
	// the error, so the recovery tracks that cursor as `root`: it starts at
	// the checkpoint cursor, moves to the jump position after a failed
	// jump read, and stays put after a failed prefix read. Anchoring every
	// re-read at root is what makes the pass progress — a prefix re-read
	// anchored at the ORIGINAL cursor would hit the very hole the jump just
	// crossed and the loop would alternate between the same two errors
	// (zero progress, the starvation back again). Each iteration either
	// returns or strictly advances root (a jump target is always > root),
	// so the pass is bounded by the number of holes in the window; a
	// pathological swiss-cheese window surfaces instead of spinning.
	root := after
	for attempt := 0; attempt < maxCursorHoleSkips; attempt++ {
		contiguous, resume, ok := repository.HolePositions(readErr)
		if !ok {
			// No positions on the error: fall back to the retained window
			// head (a trimmed head resumes contiguously from there).
			first, err := p.runs.FirstEventSeq(ctx, key)
			if err != nil {
				return after, nil, err
			}
			if first == 0 {
				// Nothing is retained at all — there is nothing to project
				// and no cursor to advance; the caller keeps its checkpoint.
				return after, nil, nil
			}
			contiguous, resume = root, first-1
		}
		if contiguous > root {
			// The prefix up to the hole's edge is contiguous FROM ROOT and
			// projectable: return exactly it (a bounded read of
			// contiguous-root rows can never reach the hole that produced
			// the error) and let the checkpoint advance to the edge; the
			// NEXT pass crosses.
			prefix, err := p.runs.ReadEvents(ctx, key, root, int(contiguous-root))
			if err == nil {
				return root, prefix, nil
			}
			if !errors.Is(err, agentruntime.ErrCursorExpired) {
				return after, nil, err
			}
			readErr = err // anchored at root — root stays
			continue
		}
		// The hole starts AT root — jump to the next contiguous segment.
		root = resume
		events, err := p.runs.ReadEvents(ctx, key, root, limit)
		if err == nil {
			return root, events, nil
		}
		if !errors.Is(err, agentruntime.ErrCursorExpired) {
			return after, nil, err
		}
		readErr = err // anchored at the NEW root
	}
	return after, nil, agentruntime.ErrCursorExpired
}

// maxCursorHoleSkips bounds the hole-jumping recovery per projection call.
const maxCursorHoleSkips = 1024

// ProjectAndCheckpoint makes the durable fan-out and cursor advancement one
// repository transaction, so a restart can only replay an uncommitted page.
func (p *NotificationProjector) ProjectAndCheckpoint(ctx context.Context, key agentruntime.RunKey, after int64, limit int) (int64, error) {
	if p == nil || p.runs == nil || p.notifications == nil {
		return after, agentruntime.ErrConflict
	}
	run, err := p.runs.Get(ctx, key)
	if err != nil {
		return after, err
	}
	events, err := p.runs.ReadEvents(ctx, key, after, limit)
	if errors.Is(err, agentruntime.ErrCursorExpired) {
		after, events, err = p.recoverExpiredCursor(ctx, key, after, limit, err)
	}
	if err != nil {
		return after, err
	}
	last := after
	refs := make([]repository.RunNotificationEvent, 0, len(events))
	for _, evt := range events {
		refs = append(refs, repository.RunNotificationEvent{TenantID: key.TenantID, OwnerID: run.UserID, RunID: key.RunID, Seq: evt.Seq, Type: evt.Type})
		if evt.Seq > last {
			last = evt.Seq
		}
	}
	if err := p.notifications.ProjectEventsAndCheckpoint(ctx, notificationConsumerName, key, refs, last); err != nil {
		return after, err
	}
	return last, nil
}
