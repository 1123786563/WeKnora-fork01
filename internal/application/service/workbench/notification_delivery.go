package workbench

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
)

// NotificationProvider is the final push vendor boundary. Implementations
// must resolve the device token by the tenant/owner/device tuple carried by
// the delivery and must not cache authorization across a lease.
type NotificationProvider interface {
	Send(context.Context, repository.NotificationDelivery) error
}

// NotificationDeliveryWorker consumes leased intents and performs the final
// authorization check immediately before the provider call. This closes the
// revoke/member-removal TOCTOU window; invalidated deliveries are retried
// without invoking the provider.
type NotificationDeliveryWorker struct {
	store    *repository.NotificationStore
	provider NotificationProvider
	worker   string
	lease    time.Duration
}

func NewNotificationDeliveryWorker(store *repository.NotificationStore, provider NotificationProvider, worker string) *NotificationDeliveryWorker {
	return &NotificationDeliveryWorker{store: store, provider: provider, worker: worker, lease: time.Minute}
}

func (w *NotificationDeliveryWorker) RunOnce(ctx context.Context, limit int) error {
	if w == nil || w.store == nil || w.provider == nil || w.worker == "" {
		return context.Canceled
	}
	deliveries, err := w.store.Claim(ctx, w.worker, limit, w.lease)
	if err != nil {
		return err
	}
	for _, delivery := range deliveries {
		if !w.store.RevalidateDelivery(ctx, delivery, w.worker) {
			w.store.Retry(ctx, delivery.ID, w.worker, delivery.Fence)
			continue
		}
		if err := w.provider.Send(ctx, delivery); err != nil {
			w.store.Retry(ctx, delivery.ID, w.worker, delivery.Fence)
			continue
		}
		w.store.Ack(ctx, delivery.ID, w.worker, delivery.Fence)
	}
	return nil
}
