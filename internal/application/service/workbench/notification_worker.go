package workbench

import (
	"context"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/logger"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
)

const notificationConsumerName = "mobile-notification-projector"

// NotificationWorker owns event discovery and durable checkpoints. Keeping
// this loop in the server lifecycle makes restart replay a production path;
// no HTTP request or in-memory event callback is required for delivery.
type NotificationWorker struct {
	projector *NotificationProjector
	store     *repository.NotificationStore
	interval  time.Duration
	once      sync.Once
}

func NewNotificationWorker(projector *NotificationProjector, store *repository.NotificationStore) *NotificationWorker {
	return &NotificationWorker{projector: projector, store: store, interval: 2 * time.Second}
}

func (w *NotificationWorker) RunOnce(ctx context.Context) error {
	if w == nil || w.projector == nil || w.store == nil {
		return agentruntime.ErrConflict
	}
	var after agentruntime.RunKey
	for {
		keys, err := w.store.EventRunKeysPage(ctx, 256, after)
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			return nil
		}
		for _, key := range keys {
			cursor, err := w.store.LoadCheckpoint(ctx, notificationConsumerName, key)
			if err != nil {
				return err
			}
			next, err := w.projector.ProjectAndCheckpoint(ctx, key, cursor, 256)
			if err != nil {
				return err
			}
			_ = next // ProjectAndCheckpoint persists the cursor atomically with intents.
			after = key
		}
		if len(keys) < 256 {
			return nil
		}
	}
}

// Start is idempotent and is called with the server application context.
// Projection failures are logged and retried on the next tick; they do not
// take down the HTTP process or advance a checkpoint.
func (w *NotificationWorker) Start(ctx context.Context) {
	if w == nil {
		return
	}
	w.once.Do(func() {
		go func() {
			ticker := time.NewTicker(w.interval)
			defer ticker.Stop()
			for {
				if err := w.RunOnce(ctx); err != nil && ctx.Err() == nil {
					logger.Warnf(ctx, "[mobile-notifications] projection failed: %v", err)
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	})
}
