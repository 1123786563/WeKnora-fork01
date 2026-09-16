package workbench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
)

// NotificationProvider is the final push vendor boundary. Implementations
// must resolve the device token by the tenant/owner/device tuple carried by
// the delivery and must not cache authorization across a lease.
type NotificationProvider interface {
	Send(context.Context, repository.NotificationDelivery) error
}

// HTTPNotificationProvider is the server-side adapter for the mobile push
// gateway. The gateway resolves the encrypted token from the tenant/device
// tuple; token material never enters the notification intent or this process's
// delivery payload. An empty endpoint is deliberately fail-closed so a
// deployment can start the durable worker before configuring a push gateway.
type HTTPNotificationProvider struct {
	endpoint string
	client   *http.Client
}

func NewHTTPNotificationProvider(endpoint string) *HTTPNotificationProvider {
	return &HTTPNotificationProvider{endpoint: endpoint, client: &http.Client{Timeout: 10 * time.Second}}
}

func (p *HTTPNotificationProvider) Send(ctx context.Context, d repository.NotificationDelivery) error {
	if p == nil || p.endpoint == "" {
		return fmt.Errorf("mobile_notification_provider_unconfigured")
	}
	payload, err := json.Marshal(map[string]any{
		"tenant_id": d.Intent.TenantID, "owner_id": d.Intent.OwnerID,
		"device_id": d.Intent.DeviceID, "environment": d.Intent.Environment,
		"event_id": d.Intent.EventID, "run_id": d.Intent.RunID,
		"kind": d.Intent.Kind, "attempt": d.Attempt,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("mobile_notification_provider_status_%d", resp.StatusCode)
	}
	return nil
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
	// afterClaim is an optional in-process seam used by deterministic
	// concurrency tests. Production workers leave it nil; when set it runs
	// after a durable lease is acquired and before final authorization.
	afterClaim func(context.Context, repository.NotificationDelivery)
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
		if w.afterClaim != nil {
			w.afterClaim(ctx, delivery)
		}
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

// Start runs the delivery loop in the same lifecycle as projection. Provider
// failures release the lease for retry and never terminate the HTTP server.
func (w *NotificationDeliveryWorker) Start(ctx context.Context) {
	if w == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			if err := w.RunOnce(ctx, 64); err != nil && ctx.Err() == nil {
				// The next tick retries; a provider outage must not crash the API.
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
