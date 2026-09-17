package workbench

import (
	"context"

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
