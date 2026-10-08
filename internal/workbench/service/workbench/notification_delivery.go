package workbench

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	pushnotification "github.com/Tencent/WeKnora/internal/workbench/notification"
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

// NotificationBatchResult is the per-item outcome returned by a batch push
// provider. A result must retain the stable durable delivery ID so a partial
// response can acknowledge successful rows and retry only failed rows.
type NotificationBatchResult struct {
	DeliveryID string
	Receipt    pushnotification.PushReceipt
	Err        error
}

// NotificationBatchProvider is optional. Existing single-item providers keep
// their contract; providers implementing this seam are called once for a
// claimed batch and must return one result per accepted item.
type NotificationBatchProvider interface {
	SendBatch(context.Context, []repository.NotificationDelivery) ([]NotificationBatchResult, error)
}

// NotificationProviderConfiguration lets a paused provider recover after a
// restart once its configuration is fixed, without sending a request first.
type NotificationProviderConfiguration interface{ Configured() bool }

type NotificationProviderHealth interface {
	IsPaused(context.Context, string) (bool, error)
	Pause(context.Context, string, string) error
	Recover(context.Context, string) error
}

// NotificationDeviceRevoker invalidates the device registration that produced
// a permanent provider failure. Implementations must scope the operation to
// the tenant, owner, and app carried by the durable intent.
type NotificationDeviceRevoker interface {
	RevokeForApp(context.Context, uint64, string, string, string, int64) error
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
	policy   PushPayloadPolicy
}

// PushPayloadPolicy is the deployment metadata-exposure policy (story 67):
// blind strips every human-readable kind/title/body from the vendor payload;
// opaque re-sync ids (run_id/event_id) always survive.
type PushPayloadPolicy struct{ Blind bool }

func (p *PushNotificationProvider) Configured() bool {
	if p == nil || p.provider == nil || p.resolve == nil {
		return false
	}
	if configured, ok := p.provider.(interface{ Configured() bool }); ok {
		return configured.Configured()
	}
	return true
}

func NewPushNotificationProvider(provider pushnotification.PushProvider, resolve func(context.Context, repository.NotificationDelivery) (string, error)) *PushNotificationProvider {
	return NewPushNotificationProviderWithOptions(provider, resolve, PushPayloadPolicy{})
}

func NewPushNotificationProviderWithOptions(provider pushnotification.PushProvider, resolve func(context.Context, repository.NotificationDelivery) (string, error), policy PushPayloadPolicy) *PushNotificationProvider {
	return &PushNotificationProvider{provider: provider, resolve: resolve, policy: policy}
}

func (p *PushNotificationProvider) payloadOf(d repository.NotificationDelivery) pushnotification.PushPayload {
	if p.policy.Blind {
		return pushnotification.PushPayload{RunID: d.Intent.RunID, EventID: d.Intent.EventID}
	}
	return pushnotification.PushPayload{Title: d.Intent.Kind, Body: d.Intent.Kind, RunID: d.Intent.RunID, EventID: d.Intent.EventID}
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
	if strings.TrimSpace(token) == "" {
		return pushnotification.PushReceipt{}, &pushnotification.ProviderError{Code: "InvalidProviderConfig", Retry: false, Revoke: false, Err: errors.New("resolved push token is empty")}
	}
	return p.provider.Send(ctx, token, p.payloadOf(d))
}

func (p *PushNotificationProvider) SendBatch(ctx context.Context, deliveries []repository.NotificationDelivery) ([]NotificationBatchResult, error) {
	if p == nil || p.provider == nil || p.resolve == nil {
		return nil, &pushnotification.ProviderError{Code: "InvalidProviderConfig", Retry: false, Revoke: false, Err: errors.New("direct push provider is not configured")}
	}
	if configured, ok := p.provider.(interface{ Configured() bool }); ok && !configured.Configured() {
		return nil, &pushnotification.ProviderError{Code: "InvalidProviderConfig", Retry: false, Revoke: false, Err: errors.New("direct push provider is not configured")}
	}
	batch, ok := p.provider.(pushnotification.PushBatchProvider)
	if !ok {
		return nil, &pushnotification.ProviderError{Code: "InvalidProviderConfig", Retry: false, Revoke: false, Err: errors.New("push provider does not support batch delivery")}
	}
	items := make([]pushnotification.PushBatchItem, 0, len(deliveries))
	resultsByID := make(map[string]NotificationBatchResult, len(deliveries))
	for _, d := range deliveries {
		token, err := p.resolve(ctx, d)
		if err != nil {
			// Do not submit an empty token.  A resolver/decryption failure is a
			// provider/configuration failure and must never be mistaken for a
			// confirmed invalid device registration.
			resultsByID[d.ID] = NotificationBatchResult{DeliveryID: d.ID, Err: &pushnotification.ProviderError{
				Code: "InvalidProviderConfig", Retry: false, Revoke: false,
				Err: fmt.Errorf("resolve push token for delivery %s: %w", d.ID, err),
			}}
			continue
		}
		if strings.TrimSpace(token) == "" {
			// A successful resolver call can still yield an empty value when a
			// registration is stale or decryption/configuration is incomplete.
			// Keep this delivery isolated so valid siblings are still submitted
			// and only this item is retried/recorded as failed.
			resultsByID[d.ID] = NotificationBatchResult{DeliveryID: d.ID, Err: &pushnotification.ProviderError{
				Code: "InvalidProviderConfig", Retry: false, Revoke: false,
				Err: fmt.Errorf("resolve push token for delivery %s: empty token", d.ID),
			}}
			continue
		}
		items = append(items, pushnotification.PushBatchItem{ID: d.ID, Token: token, Payload: p.payloadOf(d)})
	}
	if len(items) > 0 {
		results, err := batch.SendBatch(ctx, items)
		if err != nil {
			return nil, err
		}
		for _, result := range results {
			resultsByID[result.ID] = NotificationBatchResult{DeliveryID: result.ID, Receipt: result.Receipt, Err: result.Err}
		}
	}
	out := make([]NotificationBatchResult, 0, len(deliveries))
	for _, delivery := range deliveries {
		if result, ok := resultsByID[delivery.ID]; ok {
			out = append(out, result)
		}
	}
	return out, nil
}

// HTTPNotificationProvider is the server-side adapter for the mobile push
// gateway. The gateway resolves the encrypted token from the tenant/device
// tuple; token material never enters the notification intent or this process's
// delivery payload. An empty endpoint is deliberately fail-closed so a
// deployment can start the durable worker before configuring a push gateway.
type HTTPNotificationProvider struct {
	endpoint string
	blind    bool
	client   *http.Client
}

func (p *HTTPNotificationProvider) Configured() bool {
	return p != nil && validNotificationEndpoint(p.endpoint)
}

func NewHTTPNotificationProvider(endpoint string) *HTTPNotificationProvider {
	return NewHTTPNotificationProviderWithPolicy(endpoint, false)
}

// NewHTTPNotificationProviderWithPolicy: blind=true omits the kind metadata
// key from the gateway payload; device identity and opaque ids stay so the
// gateway can still resolve the encrypted token server-side.
func NewHTTPNotificationProviderWithPolicy(endpoint string, blind bool) *HTTPNotificationProvider {
	return &HTTPNotificationProvider{endpoint: strings.TrimSpace(endpoint), blind: blind, client: &http.Client{Timeout: 10 * time.Second}}
}

func validNotificationEndpoint(endpoint string) bool {
	u, err := url.Parse(strings.TrimSpace(endpoint))
	return err == nil && u.Scheme != "" && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https")
}

// DisallowedPushEndpointHost rejects push endpoints that target loopback,
// private, link-local, or reserved hosts — the server-side counterpart of the
// mobile disallowedDeploymentHost rule. The config-assembly layer (story 67
// controller ruling + final-fix round) runs it on every push URL — the
// official gateway/expo endpoint, the APNs/FCM lanes, and the FCM
// credential-file token_uri fallback — so a misdirected vendor lane fails
// closed at startup instead of sending tokens to an internal address. The
// default vendor endpoints are public DNS names and pass. The provider layer
// stays scheme-only on purpose: validation sits at the assembly boundary so
// providers remain httptest-testable.
//
// Known boundary (documented, accepted): the check inspects literal hosts
// only — IP literals and localhost names. It does NOT resolve DNS, so a
// public-looking hostname that resolves to a private address passes this
// gate. This mirrors the mobile disallowedDeploymentHost precedent and is
// considered acceptable for an admin-controlled config plane whose vendor
// endpoints are fixed public DNS names (exp.host, api.push.apple.com,
// fcm.googleapis.com, oauth2.googleapis.com); revisit only if endpoints
// become operator-supplied hostnames.
func DisallowedPushEndpointHost(endpoint string) error {
	u, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return fmt.Errorf("push endpoint parse: %w", err)
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if host == "" {
		return errors.New("push endpoint host is empty")
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return errors.New("push endpoint must not target localhost")
	}
	if ip := net.ParseIP(host); ip != nil {
		return disallowedPushIP(ip)
	}
	return nil
}

func disallowedPushIP(ip net.IP) error {
	if b4 := ip.To4(); b4 != nil {
		return disallowedPushIPv4(b4)
	}
	return disallowedPushIPv6(ip.To16())
}

func disallowedPushIPv4(b []byte) error {
	a, c := b[0], b[2]
	switch {
	case a == 127 || a == 0 || a >= 240:
		return errors.New("push endpoint must not target loopback or reserved addresses")
	case a == 10:
		return errors.New("push endpoint must not target private addresses")
	case a == 172 && b[1] >= 16 && b[1] <= 31:
		return errors.New("push endpoint must not target private addresses")
	case a == 192 && b[1] == 168:
		return errors.New("push endpoint must not target private addresses")
	case a == 169 && b[1] == 254:
		return errors.New("push endpoint must not target link-local addresses")
	case a == 100 && b[1] >= 64 && b[1] <= 127:
		return errors.New("push endpoint must not target carrier-grade NAT addresses")
	case a == 198 && (b[1] == 18 || b[1] == 19):
		return errors.New("push endpoint must not target benchmarking addresses")
	case a == 192 && b[1] == 0 && c == 0:
		return errors.New("push endpoint must not target reserved addresses")
	case a == 192 && b[1] == 0 && c == 2:
		return errors.New("push endpoint must not target documentation addresses")
	case a == 198 && b[1] == 51 && c == 100:
		return errors.New("push endpoint must not target documentation addresses")
	case a == 203 && b[1] == 0 && c == 113:
		return errors.New("push endpoint must not target documentation addresses")
	}
	return nil
}

func disallowedPushIPv6(b []byte) error {
	words := make([]int, 8)
	for i := range words {
		words[i] = int(b[i*2])<<8 | int(b[i*2+1])
	}
	wordsZero := func(from, to int) bool {
		for _, w := range words[from:to] {
			if w != 0 {
				return false
			}
		}
		return true
	}
	// ::ffff:0:0/96 IPv4-mapped: the embedded IPv4 tail reuses the IPv4 rules.
	if wordsZero(0, 6) && words[6] == 0xffff {
		return disallowedPushIPv4(b[12:])
	}
	if wordsZero(0, 7) {
		switch words[7] {
		case 0:
			return errors.New("push endpoint must not target the unspecified address")
		case 1:
			return errors.New("push endpoint must not target a loopback or wildcard address")
		default:
			// ::/96 IPv4-compatible: the embedded IPv4 tail reuses the IPv4 rules.
			return disallowedPushIPv4(b[12:])
		}
	}
	switch {
	case words[0]&0xffc0 == 0xfe80:
		return errors.New("push endpoint must not target link-local addresses")
	case words[0]&0xfe00 == 0xfc00:
		return errors.New("push endpoint must not target private addresses")
	case words[0]&0xff00 == 0xff00:
		return errors.New("push endpoint must not target multicast or reserved addresses")
	case words[0] == 0x2001 && (words[1] == 0x0db8 || words[1] == 0):
		return errors.New("push endpoint must not target reserved addresses")
	case words[0] == 0x2002:
		return disallowedPushIPv4(b[2:6])
	case words[0] == 0x0100 && wordsZero(1, 4):
		return errors.New("push endpoint must not target reserved addresses")
	case words[0]&0xe000 != 0x2000:
		return errors.New("push endpoint must not target reserved addresses")
	}
	return nil
}

func (p *HTTPNotificationProvider) Send(ctx context.Context, d repository.NotificationDelivery) error {
	_, err := p.send(ctx, d, false)
	return err
}

func (p *HTTPNotificationProvider) SendReceipt(ctx context.Context, d repository.NotificationDelivery) (pushnotification.PushReceipt, error) {
	return p.send(ctx, d, true)
}

func (p *HTTPNotificationProvider) send(ctx context.Context, d repository.NotificationDelivery, requireReceipt bool) (pushnotification.PushReceipt, error) {
	if p == nil || strings.TrimSpace(p.endpoint) == "" {
		return pushnotification.PushReceipt{}, &pushnotification.ProviderError{Code: "mobile_notification_provider_unconfigured", Retry: false, Err: errors.New("mobile notification provider is not configured")}
	}
	if !validNotificationEndpoint(p.endpoint) {
		return pushnotification.PushReceipt{}, &pushnotification.ProviderError{Code: "InvalidProviderConfig", Retry: false, Err: fmt.Errorf("invalid mobile notification provider endpoint")}
	}
	payloadMap := map[string]any{"tenant_id": d.Intent.TenantID, "owner_id": d.Intent.OwnerID, "device_id": d.Intent.DeviceID, "environment": d.Intent.Environment, "app_id": repository.NormalizeMobileAppID(d.Intent.AppID), "event_id": d.Intent.EventID, "run_id": d.Intent.RunID, "attempt": d.Attempt}
	if !p.blind {
		payloadMap["kind"] = d.Intent.Kind
	}
	payload, err := json.Marshal(payloadMap)
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
	store          *repository.NotificationStore
	provider       NotificationProvider
	worker         string
	lease          time.Duration
	deviceRevoker  NotificationDeviceRevoker
	providerHealth NotificationProviderHealth
	providerKey    string
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

func NewNotificationDeliveryWorkerWithHealth(store *repository.NotificationStore, provider NotificationProvider, worker string, revoker NotificationDeviceRevoker, health NotificationProviderHealth, providerKey string) *NotificationDeliveryWorker {
	w := NewNotificationDeliveryWorkerWithRevoker(store, provider, worker, revoker)
	w.providerHealth = health
	w.providerKey = providerKey
	return w
}

func (w *NotificationDeliveryWorker) RunOnce(ctx context.Context, limit int) error {
	if w == nil || w.store == nil || w.provider == nil || w.worker == "" {
		return context.Canceled
	}
	if w.providerHealth != nil && w.providerKey != "" {
		paused, healthErr := w.providerHealth.IsPaused(ctx, w.providerKey)
		if healthErr != nil {
			return healthErr
		}
		if paused {
			if configured, ok := w.provider.(NotificationProviderConfiguration); !ok || !configured.Configured() {
				return nil
			}
			if err := w.providerHealth.Recover(ctx, w.providerKey); err != nil {
				return err
			}
		}
	}
	deliveries, err := w.store.Claim(ctx, w.worker, limit, w.lease)
	if err != nil {
		return err
	}
	var firstErr error
	if batchProvider, ok := w.provider.(NotificationBatchProvider); ok && len(deliveries) > 1 {
		return w.runBatch(ctx, batchProvider, deliveries)
	}
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

func (w *NotificationDeliveryWorker) runBatch(ctx context.Context, provider NotificationBatchProvider, deliveries []repository.NotificationDelivery) error {
	eligible := make([]repository.NotificationDelivery, 0, len(deliveries))
	for _, delivery := range deliveries {
		if w.afterClaim != nil {
			w.afterClaim(ctx, delivery)
		}
		if w.store.RevalidateDelivery(ctx, delivery, w.worker) {
			eligible = append(eligible, delivery)
		} else if err := w.releaseDelivery(ctx, delivery); err != nil {
			return err
		}
	}
	if len(eligible) == 0 {
		return nil
	}
	results, batchErr := provider.SendBatch(ctx, eligible)
	byID := make(map[string]NotificationBatchResult, len(results))
	for _, result := range results {
		if result.DeliveryID != "" {
			byID[result.DeliveryID] = result
		}
	}
	var firstErr error
	for _, delivery := range eligible {
		result, found := byID[delivery.ID]
		if batchErr != nil {
			result.Err = batchErr
		} else if !found {
			result.Err = &pushnotification.ProviderError{Code: "MissingBatchResult", Retry: true, Err: errors.New("batch provider omitted delivery result")}
		} else if result.Err == nil && result.Receipt.ID == "" {
			result.Err = &pushnotification.ProviderError{Code: "MissingReceipt", Retry: true, Err: pushnotification.ErrMissingReceiptID}
		}
		if result.Err != nil {
			firstErr = firstNonNil(firstErr, w.releaseDeliveryWithCause(ctx, delivery, result.Err))
			continue
		}
		var ack repository.NotificationRetryResult
		if result.Receipt.ID != "" {
			ack = w.store.AckReceipt(ctx, delivery.ID, w.worker, delivery.Fence, result.Receipt.ID)
		} else {
			ack = repository.NotificationRetryResult{Applied: w.store.Ack(ctx, delivery.ID, w.worker, delivery.Fence)}
		}
		if ack.Err != nil {
			firstErr = firstNonNil(firstErr, ack.Err)
		} else if !ack.Applied {
			firstErr = firstNonNil(firstErr, fmt.Errorf("notification_delivery_ack_fence_lost:%s", delivery.ID))
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
		providerPaused := false
		if isProviderConfigurationError(providerErr.Code) {
			if w.providerHealth != nil && w.providerKey != "" {
				if err := w.providerHealth.Pause(ctx, w.providerKey, notificationErrorText(cause)); err != nil {
					return fmt.Errorf("notification_provider_pause_persist:%s: %w", w.providerKey, err)
				}
				// The row is released so it remains durable for recovery, while
				// the claim gate prevents a retry storm during the pause.
				providerPaused = true
			}
		}
		if providerPaused {
			// Keep the intent pending for recovery after configuration is fixed.
			providerErr.Retry = true
		} else {
			result := w.store.Expire(ctx, d.ID, w.worker, d.Fence, providerErr.Code)
			if result.Err != nil {
				return fmt.Errorf("notification_delivery_expire_persist:%s: %w: %v", d.ID, result.Err, cause)
			}
			if result.Applied {
				if w.deviceRevoker != nil && providerErr.Revoke {
					if d.DeviceRevision <= 0 {
						return fmt.Errorf("notification_device_revoke:%s: missing registration revision", d.ID)
					}
					if revokeErr := w.deviceRevoker.RevokeForApp(ctx, d.Intent.TenantID, d.Intent.OwnerID, d.Intent.DeviceID, repository.NormalizeMobileAppID(d.Intent.AppID), d.DeviceRevision); revokeErr != nil {
						return fmt.Errorf("notification_device_revoke:%s: %w", d.ID, revokeErr)
					}
				}
				return nil
			}
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

func isProviderConfigurationError(code string) bool {
	switch code {
	case "InvalidProviderConfig", "mobile_notification_provider_unconfigured", "mobile_notification_provider_disabled", "InvalidCredentials", "InvalidProviderToken":
		return true
	default:
		return false
	}
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

// DisabledNotificationProvider is the explicit "no push" deployment policy
// (story 67). Sends fail closed with a configuration-class code so the
// durable worker pauses once and never storms; intents stay pending and are
// recoverable the moment the policy is switched back.
type DisabledNotificationProvider struct{}

func NewDisabledNotificationProvider() *DisabledNotificationProvider {
	return &DisabledNotificationProvider{}
}

func (p *DisabledNotificationProvider) Configured() bool { return false }

func (p *DisabledNotificationProvider) Send(context.Context, repository.NotificationDelivery) error {
	return &pushnotification.ProviderError{Code: "mobile_notification_provider_disabled", Retry: false, Err: errors.New("mobile notification provider is disabled by deployment policy")}
}

// AppRoutingNotificationProvider dispatches each durable delivery to the
// provider registered for its app identity. An unknown or empty AppID
// normalizes to official and hits the fallback provider.
type AppRoutingNotificationProvider struct {
	fallback NotificationProvider
	routes   map[string]NotificationProvider
}

func NewAppRoutingNotificationProvider(fallback NotificationProvider, routes map[string]NotificationProvider) *AppRoutingNotificationProvider {
	return &AppRoutingNotificationProvider{fallback: fallback, routes: routes}
}

func (p *AppRoutingNotificationProvider) forApp(d repository.NotificationDelivery) NotificationProvider {
	if p == nil {
		return nil
	}
	app := repository.NormalizeMobileAppID(d.Intent.AppID)
	if routed, ok := p.routes[app]; ok && routed != nil {
		return routed
	}
	return p.fallback
}

func (p *AppRoutingNotificationProvider) Send(ctx context.Context, d repository.NotificationDelivery) error {
	provider := p.forApp(d)
	if provider == nil {
		return &pushnotification.ProviderError{Code: "mobile_notification_provider_unconfigured", Retry: false, Err: errors.New("no notification provider is configured for this app")}
	}
	return provider.Send(ctx, d)
}

func (p *AppRoutingNotificationProvider) SendReceipt(ctx context.Context, d repository.NotificationDelivery) (pushnotification.PushReceipt, error) {
	provider := p.forApp(d)
	if receiptProvider, ok := provider.(NotificationReceiptProvider); ok {
		return receiptProvider.SendReceipt(ctx, d)
	}
	err := p.Send(ctx, d)
	return pushnotification.PushReceipt{}, err
}

// Configured reports whether the aggregate has at least one deliverable lane.
// It is an OR over the fallback and every registered route: the durable worker
// treats "paused && !Configured()" as a skip-everything gate before the claim,
// so an aggregate that only mirrored the fallback would let one disabled lane
// (for example the official lane under MOBILE_NOTIFICATION_PROVIDER=disabled)
// starve a live enterprise APNs/FCM route forever. A single lane's failures
// still pause the shared worker key (pre-existing design); this only keeps the
// recovery gate honest about lanes that are alive.
func (p *AppRoutingNotificationProvider) Configured() bool {
	if p == nil {
		return false
	}
	configured := func(provider NotificationProvider) bool {
		if provider == nil {
			return false
		}
		if configured, ok := provider.(NotificationProviderConfiguration); ok {
			return configured.Configured()
		}
		return true
	}
	if configured(p.fallback) {
		return true
	}
	for _, route := range p.routes {
		if configured(route) {
			return true
		}
	}
	return false
}
