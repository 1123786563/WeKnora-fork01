package workbench

import (
	"context"
	"errors"

	"github.com/Tencent/WeKnora/internal/application/repository"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
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
	// Each expired page carries BOTH positions around its hole. The page's
	// contiguous PREFIX is projected first — the checkpoint walks to the
	// hole's edge and the next projection pass jumps it — so every call
	// makes strictly forward progress and the pass is bounded by the number
	// of holes in the window (a healthy stream has none); a pathological
	// swiss-cheese window surfaces instead of spinning.
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
			contiguous, resume = after, first-1
		}
		if contiguous > after {
			// The prefix up to the hole's edge is contiguous and
			// projectable: return exactly it (a bounded read of
			// contiguous-after rows can never reach the hole) and let the
			// checkpoint advance to the edge; the NEXT pass crosses.
			prefix, err := p.runs.ReadEvents(ctx, key, after, int(contiguous-after))
			if err == nil {
				return after, prefix, nil
			}
			if !errors.Is(err, agentruntime.ErrCursorExpired) {
				return after, nil, err
			}
			readErr = err
			continue
		}
		// The hole starts AT the cursor — jump to the next segment.
		events, err := p.runs.ReadEvents(ctx, key, resume, limit)
		if err == nil {
			return resume, events, nil
		}
		if !errors.Is(err, agentruntime.ErrCursorExpired) {
			return after, nil, err
		}
		readErr = err
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
