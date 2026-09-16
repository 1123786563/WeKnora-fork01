package workbench

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"net/http"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	pushnotification "github.com/Tencent/WeKnora/internal/notification"
)

// NotificationProvider is the final push vendor boundary. Implementations
// must resolve the device token by the tenant/owner/device tuple carried by
// the delivery and must not cache authorization across a lease.
type NotificationProvider interface {
	Send(context.Context, repository.NotificationDelivery) error
}

// NotificationReceiptProvider is an optional stronger provider contract. A
// worker uses it when available so an HTTP 2xx is not recorded as sent without
// the vendor receipt ID. Legacy scoped gateways may implement only Send.
type NotificationReceiptProvider interface {
	SendReceipt(context.Context, repository.NotificationDelivery) (pushnotification.PushReceipt, error)
}

// NotificationDeviceRevoker invalidates the device registration that produced
// a permanent provider failure. Implementations must scope the operation to
// the tenant, owner, and environment carried by the durable intent.
type NotificationDeviceRevoker interface {
	RevokeForTenant(context.Context, uint64, string, string, int64) error
}

// NotificationTokenResolver is used only by direct providers (for example
// Expo). The gateway provider does not need token material because it resolves
// the tenant/device tuple server-side.
type NotificationTokenResolver interface {
	GetActiveForTenant(context.Context, uint64, string, string) (repository.DeviceRegistration, error)
}

// PushNotificationProvider adapts the provider-neutral direct push API to the
// durable delivery contract. Token decryption remains in the composition
// root; notification intents and leases never contain plaintext tokens.
type PushNotificationProvider struct {
	provider pushnotification.PushProvider
	resolve  func(context.Context, repository.NotificationDelivery) (string, error)
}

func NewPushNotificationProvider(provider pushnotification.PushProvider, resolve func(context.Context, repository.NotificationDelivery) (string, error)) *PushNotificationProvider {
	return &PushNotificationProvider{provider: provider, resolve: resolve}
}

func (p *PushNotificationProvider) Send(ctx context.Context, d repository.NotificationDelivery) error {
	_, err := p.SendReceipt(ctx, d)
	return err
}

func (p *PushNotificationProvider) SendReceipt(ctx context.Context, d repository.NotificationDelivery) (pushnotification.PushReceipt, error) {
	if p == nil || p.provider == nil || p.resolve == nil {
		return pushnotification.PushReceipt{}, &pushnotification.ProviderError{Code: "InvalidProviderConfig", Retry: false, Err: errors.New("direct push provider is not configured")}
	}
	token, err := p.resolve(ctx, d)
	if err != nil {
		return pushnotification.PushReceipt{}, &pushnotification.ProviderError{Code: "InvalidRegistration", Revoke: true, Retry: false, Err: err}
	}
	return p.provider.Send(ctx, token, pushnotification.PushPayload{Title: d.Intent.Kind, Body: d.Intent.Kind, RunID: d.Intent.RunID, EventID: d.Intent.EventID})
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
	_, err := p.send(ctx, d, false)
	return err
}

func (p *HTTPNotificationProvider) SendReceipt(ctx context.Context, d repository.NotificationDelivery) (pushnotification.PushReceipt, error) {
	return p.send(ctx, d, true)
}

func (p *HTTPNotificationProvider) send(ctx context.Context, d repository.NotificationDelivery, requireReceipt bool) (pushnotification.PushReceipt, error) {
	if p == nil || p.endpoint == "" {
		return pushnotification.PushReceipt{}, fmt.Errorf("mobile_notification_provider_unconfigured")
	}
	payload, err := json.Marshal(map[string]any{"tenant_id": d.Intent.TenantID, "owner_id": d.Intent.OwnerID, "device_id": d.Intent.DeviceID, "environment": d.Intent.Environment, "event_id": d.Intent.EventID, "run_id": d.Intent.RunID, "kind": d.Intent.Kind, "attempt": d.Attempt})
	if err != nil {
		return pushnotification.PushReceipt{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(payload))
	if err != nil {
		return pushnotification.PushReceipt{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return pushnotification.PushReceipt{}, &pushnotification.ProviderError{Code: "UnknownTransport", Retry: true, Err: err}
	}
	defer resp.Body.Close()
	var body struct {
		ID        string `json:"id"`
		ReceiptID string `json:"receipt_id"`
		Status    string `json:"status"`
		Code      string `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if resp.StatusCode == http.StatusTooManyRequests {
		return pushnotification.PushReceipt{}, &pushnotification.ProviderError{Code: "MessageRateExceeded", Retry: true, StatusCode: resp.StatusCode, RetryAfter: pushnotification.ParseRetryAfter(resp.Header.Get("Retry-After"))}
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		code := "UnknownTransport"
		if body.Code != "" {
			code = body.Code
		}
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			if body.Code == "" {
				code = "InvalidProviderToken"
			}
		}
		revoke, retry := pushnotification.ClassifyPushFailure(code)
		return pushnotification.PushReceipt{}, &pushnotification.ProviderError{Code: code, Revoke: revoke, Retry: retry, StatusCode: resp.StatusCode}
	}
	id := body.ID
	if id == "" {
		id = body.ReceiptID
	}
	if id == "" && requireReceipt {
		return pushnotification.PushReceipt{}, pushnotification.ErrMissingReceiptID
	}
	return pushnotification.PushReceipt{ID: id, Status: body.Status}, nil
}

// NotificationDeliveryWorker consumes leased intents and performs the final
// authorization check immediately before the provider call. This closes the
// revoke/member-removal TOCTOU window; invalidated deliveries are retried
// without invoking the provider.
type NotificationDeliveryWorker struct {
	store         *repository.NotificationStore
	provider      NotificationProvider
	worker        string
	lease         time.Duration
	deviceRevoker NotificationDeviceRevoker
	// afterClaim is an optional in-process seam used by deterministic
	// concurrency tests. Production workers leave it nil; when set it runs
	// after a durable lease is acquired and before final authorization.
	afterClaim func(context.Context, repository.NotificationDelivery)
}

func NewNotificationDeliveryWorker(store *repository.NotificationStore, provider NotificationProvider, worker string) *NotificationDeliveryWorker {
	return &NotificationDeliveryWorker{store: store, provider: provider, worker: worker, lease: time.Minute}
}

func NewNotificationDeliveryWorkerWithRevoker(store *repository.NotificationStore, provider NotificationProvider, worker string, revoker NotificationDeviceRevoker) *NotificationDeliveryWorker {
	w := NewNotificationDeliveryWorker(store, provider, worker)
	w.deviceRevoker = revoker
	return w
}

func (w *NotificationDeliveryWorker) RunOnce(ctx context.Context, limit int) error {
	if w == nil || w.store == nil || w.provider == nil || w.worker == "" {
		return context.Canceled
	}
	deliveries, err := w.store.Claim(ctx, w.worker, limit, w.lease)
	if err != nil {
		return err
	}
	var firstErr error
	for _, delivery := range deliveries {
		if w.afterClaim != nil {
			w.afterClaim(ctx, delivery)
		}
		if !w.store.RevalidateDelivery(ctx, delivery, w.worker) {
			firstErr = firstNonNil(firstErr, w.releaseDelivery(ctx, delivery))
			continue
		}
		var receiptID string
		var sendErr error
		if receiptProvider, ok := w.provider.(NotificationReceiptProvider); ok {
			var receipt pushnotification.PushReceipt
			receipt, sendErr = receiptProvider.SendReceipt(ctx, delivery)
			receiptID = receipt.ID
		} else {
			sendErr = w.provider.Send(ctx, delivery)
		}
		if sendErr != nil {
			firstErr = firstNonNil(firstErr, w.releaseDeliveryWithCause(ctx, delivery, sendErr))
			continue
		}
		ack := false
		if receiptID != "" {
			result := w.store.AckReceipt(ctx, delivery.ID, w.worker, delivery.Fence, receiptID)
			ack = result.Applied
			if result.Err != nil && firstErr == nil {
				firstErr = fmt.Errorf("notification_delivery_ack_persist:%s: %w", delivery.ID, result.Err)
			}
		} else {
			ack = w.store.Ack(ctx, delivery.ID, w.worker, delivery.Fence)
		}
		if !ack && firstErr == nil {
			firstErr = fmt.Errorf("notification_delivery_ack_fence_lost:%s", delivery.ID)
		}
	}
	return firstErr
}

func firstNonNil(existing, next error) error {
	if existing != nil {
		return existing
	}
	return next
}

// releaseDelivery reports persistence failures, while treating a zero-row
// RetryResult as an expected stale-worker outcome only when IsSentResult
// positively confirms that a newer worker already completed the row.
func (w *NotificationDeliveryWorker) releaseDelivery(ctx context.Context, d repository.NotificationDelivery) error {
	return w.releaseDeliveryWithCause(ctx, d, nil)
}

func (w *NotificationDeliveryWorker) releaseDeliveryWithCause(ctx context.Context, d repository.NotificationDelivery, cause error) error {
	// A provider can explicitly classify a permanent receipt failure.  Expire
	// that row instead of retrying a bad token forever; the durable event stays
	// available for audit and the device registration can be revoked separately.
	var providerErr *pushnotification.ProviderError
	if cause != nil && errors.As(cause, &providerErr) && !providerErr.Retry {
		result := w.store.Expire(ctx, d.ID, w.worker, d.Fence, providerErr.Code)
		if result.Err != nil {
			return fmt.Errorf("notification_delivery_expire_persist:%s: %w: %v", d.ID, result.Err, cause)
		}
		if result.Applied {
			if w.deviceRevoker != nil {
				if d.DeviceRevision <= 0 {
					return fmt.Errorf("notification_device_revoke:%s: missing registration revision", d.ID)
				}
				if revokeErr := w.deviceRevoker.RevokeForTenant(ctx, d.Intent.TenantID, d.Intent.OwnerID, d.Intent.DeviceID, d.DeviceRevision); revokeErr != nil {
					return fmt.Errorf("notification_device_revoke:%s: %w", d.ID, revokeErr)
				}
			}
			return nil
		}
	}
	next := time.Now().UTC()
	if cause != nil {
		next = next.Add(notificationRetryDelay(d.ID, d.Attempt, providerErrRetryAfter(providerErr)))
	}
	result := w.store.RetryAt(ctx, d.ID, w.worker, d.Fence, next, notificationErrorText(cause))
	if result.Err != nil {
		if cause != nil {
			return fmt.Errorf("notification_delivery_retry_persist:%s: %w: %v", d.ID, result.Err, cause)
		}
		return fmt.Errorf("notification_delivery_retry_persist:%s: %w", d.ID, result.Err)
	}
	if result.Applied {
		return nil
	}
	sent, err := w.store.IsSentResult(ctx, d.ID)
	if err != nil {
		return fmt.Errorf("notification_delivery_retry_observe:%s: %w", d.ID, err)
	}
	if sent {
		return nil
	}
	if cause != nil {
		return fmt.Errorf("notification_delivery_retry_fence_lost:%s: %w", d.ID, cause)
	}
	return fmt.Errorf("notification_delivery_retry_fence_lost:%s", d.ID)
}

func providerErrRetryAfter(err *pushnotification.ProviderError) time.Duration {
	if err == nil {
		return 0
	}
	return err.RetryAfter
}
func notificationErrorText(err error) string {
	if err == nil {
		return "authorization_revalidation"
	}
	return err.Error()
}

// notificationRetryDelay is bounded exponential backoff with stable per-ID
// jitter. Stable jitter spreads a batch without making retry timing impossible
// to inspect in tests or operations. A provider Retry-After hint wins when it
// is larger, but the five-minute cap always applies.
func notificationRetryDelay(id string, attempt int64, hint time.Duration) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 8 {
		attempt = 8
	}
	base := time.Second << (attempt - 1)
	if base > 5*time.Minute {
		base = 5 * time.Minute
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	jitter := time.Duration(h.Sum32() % uint32(maxDuration(base/2, time.Millisecond)))
	delay := base + jitter
	if hint > delay {
		delay = hint
	}
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	return delay
}
func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
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
