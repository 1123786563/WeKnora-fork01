package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	pushnotification "github.com/Tencent/WeKnora/internal/modules/workbench/notification"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type notificationProviderSpy struct {
	mu    sync.Mutex
	sends []repository.NotificationDelivery
}

type batchNotificationProvider struct {
	mu        sync.Mutex
	called    int
	failID    string
	failIndex int
	failOnce  bool
	seenIDs   []string
	calls     [][]string
}

type directBatchProvider struct {
	configured bool
	items      []pushnotification.PushBatchItem
}

func (p *directBatchProvider) Configured() bool { return p != nil && p.configured }
func (p *directBatchProvider) Send(context.Context, string, pushnotification.PushPayload) (pushnotification.PushReceipt, error) {
	return pushnotification.PushReceipt{ID: "single"}, nil
}

func (p *directBatchProvider) SendBatch(_ context.Context, items []pushnotification.PushBatchItem) ([]pushnotification.PushBatchResult, error) {
	p.items = append([]pushnotification.PushBatchItem(nil), items...)
	results := make([]pushnotification.PushBatchResult, 0, len(items))
	for _, item := range items {
		results = append(results, pushnotification.PushBatchResult{ID: item.ID, Receipt: pushnotification.PushReceipt{ID: "receipt-" + item.ID, Status: "ok"}})
	}
	return results, nil
}

func TestPushNotificationProviderBatchFailsClosedAndPreservesValidDevices(t *testing.T) {
	provider := &directBatchProvider{configured: true}
	resolverErr := errors.New("token decrypt failed")
	adapter := NewPushNotificationProvider(provider, func(_ context.Context, d repository.NotificationDelivery) (string, error) {
		if d.ID == "bad" {
			return "", resolverErr
		}
		return "token-valid", nil
	})
	results, err := adapter.SendBatch(context.Background(), []repository.NotificationDelivery{{ID: "bad"}, {ID: "good"}})
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Len(t, provider.items, 1, "resolver failures must never submit an empty token")
	require.Equal(t, "good", provider.items[0].ID)
	require.Equal(t, "token-valid", provider.items[0].Token)
	var providerErr *pushnotification.ProviderError
	require.ErrorAs(t, results[0].Err, &providerErr)
	require.Equal(t, "InvalidProviderConfig", providerErr.Code)
	require.False(t, providerErr.Revoke)
	require.False(t, providerErr.Retry)
	require.ErrorIs(t, providerErr, resolverErr)
	require.Equal(t, "receipt-good", results[1].Receipt.ID)
}

func TestPushNotificationProviderBatchOmitsEmptyResolvedTokenAndPreservesValidDevice(t *testing.T) {
	provider := &directBatchProvider{configured: true}
	adapter := NewPushNotificationProvider(provider, func(_ context.Context, d repository.NotificationDelivery) (string, error) {
		if d.ID == "empty" {
			return "  ", nil
		}
		return "token-valid", nil
	})
	results, err := adapter.SendBatch(context.Background(), []repository.NotificationDelivery{{ID: "empty"}, {ID: "good"}})
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Len(t, provider.items, 1, "empty resolved tokens must never reach the provider")
	require.Equal(t, "good", provider.items[0].ID)
	require.Equal(t, "token-valid", provider.items[0].Token)
	var providerErr *pushnotification.ProviderError
	require.ErrorAs(t, results[0].Err, &providerErr)
	require.Equal(t, "InvalidProviderConfig", providerErr.Code)
	require.False(t, providerErr.Revoke)
	require.False(t, providerErr.Retry)
	require.Equal(t, "receipt-good", results[1].Receipt.ID)
}

func TestPushNotificationProviderBatchRejectsUnconfiguredAdapter(t *testing.T) {
	provider := &directBatchProvider{configured: true}
	adapter := NewPushNotificationProvider(provider, nil)
	_, err := adapter.SendBatch(context.Background(), []repository.NotificationDelivery{{ID: "d1"}, {ID: "d2"}})
	var providerErr *pushnotification.ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "InvalidProviderConfig", providerErr.Code)
	require.False(t, providerErr.Revoke)
	require.False(t, providerErr.Retry)

	provider = &directBatchProvider{configured: false}
	adapter = NewPushNotificationProvider(provider, func(context.Context, repository.NotificationDelivery) (string, error) { return "token", nil })
	_, err = adapter.SendBatch(context.Background(), []repository.NotificationDelivery{{ID: "d1"}, {ID: "d2"}})
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "InvalidProviderConfig", providerErr.Code)
	require.Empty(t, provider.items)
}

func (p *batchNotificationProvider) Send(_ context.Context, d repository.NotificationDelivery) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.called++
	p.calls = append(p.calls, []string{d.ID})
	return nil
}

func (p *batchNotificationProvider) SendBatch(_ context.Context, deliveries []repository.NotificationDelivery) ([]NotificationBatchResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.called++
	callIDs := make([]string, 0, len(deliveries))
	results := make([]NotificationBatchResult, 0, len(deliveries))
	for i, d := range deliveries {
		p.seenIDs = append(p.seenIDs, d.ID)
		callIDs = append(callIDs, d.ID)
		if (d.ID == p.failID || (p.failIndex >= 0 && i == p.failIndex)) && (!p.failOnce || p.called == 1) {
			results = append(results, NotificationBatchResult{DeliveryID: d.ID, Err: &pushnotification.ProviderError{Code: "UnknownTransport", Retry: true}})
			continue
		}
		results = append(results, NotificationBatchResult{DeliveryID: d.ID, Receipt: pushnotification.PushReceipt{ID: "receipt-" + d.ID, Status: "ok"}})
	}
	p.calls = append(p.calls, callIDs)
	return results, nil
}

func (p *notificationProviderSpy) Send(_ context.Context, d repository.NotificationDelivery) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sends = append(p.sends, d)
	return nil
}

func (p *notificationProviderSpy) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.sends)
}

func seedDeliveryFixture(t *testing.T, deviceID string) (*repository.NotificationStore, *gorm.DB) {
	t.Helper()
	db := openAdmissionConcurrencyDB(t)
	runs := repository.NewAgentRunStore(db)
	_, err := runs.Admit(context.Background(), deliveryTestAdmission("delivery-run"))
	require.NoError(t, err)
	require.NoError(t, db.Exec(`INSERT INTO mobile_devices
		(tenant_id, owner_id, device_id, environment, platform, token_ciphertext, token_hash)
		VALUES (1, 'u1', ?, 'dev', 'ios', 'cipher', ?)`, deviceID, "hash-"+deviceID).Error)
	store := repository.NewNotificationStore(db)
	require.NoError(t, store.Enqueue(context.Background(), repository.NotificationIntent{
		TenantID: 1, EventID: "delivery-event-" + deviceID, OwnerID: "u1", DeviceID: deviceID,
		Environment: "dev", Kind: "completed", RunID: "delivery-run", ExpiresAt: time.Now().Add(time.Hour),
	}))
	return store, db
}

// TestNotificationDeliveryRevalidatesBeforeProviderSend covers the provider
// control and the revoke interleaving at the final authorization seam.
func TestNotificationDeliveryRevalidatesBeforeProviderSend(t *testing.T) {
	store, db := seedDeliveryFixture(t, "valid-device")
	spy := &notificationProviderSpy{}
	worker := NewNotificationDeliveryWorker(store, spy, "delivery-worker")
	require.NoError(t, worker.RunOnce(context.Background(), 10))
	require.Equal(t, 1, spy.count())

	store, db = seedDeliveryFixture(t, "revoked-device")
	require.NoError(t, db.Exec("UPDATE mobile_devices SET revoked_at = CURRENT_TIMESTAMP WHERE device_id = 'revoked-device'").Error)
	spy = &notificationProviderSpy{}
	worker = NewNotificationDeliveryWorker(store, spy, "delivery-worker")
	require.NoError(t, worker.RunOnce(context.Background(), 10))
	require.Equal(t, 0, spy.count(), "revoked device must be rejected before provider invocation")
}

func TestNotificationDeliveryRejectsRevocationAndMemberRemovalAfterClaim(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*gorm.DB)
	}{
		{name: "device-revoked", mutate: func(db *gorm.DB) {
			require.NoError(t, db.Exec("UPDATE mobile_devices SET revoked_at = CURRENT_TIMESTAMP WHERE device_id = 'race-device'").Error)
		}},
		{name: "member-removed", mutate: func(db *gorm.DB) {
			require.NoError(t, db.Exec("UPDATE agent_runs SET owner_id = 'removed-owner' WHERE run_id = 'delivery-run'").Error)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, db := seedDeliveryFixture(t, "race-device")
			spy := &notificationProviderSpy{}
			worker := NewNotificationDeliveryWorker(store, spy, "race-worker")
			claimed := make(chan repository.NotificationDelivery, 1)
			continueRun := make(chan struct{})
			worker.afterClaim = func(_ context.Context, d repository.NotificationDelivery) {
				claimed <- d
				<-continueRun
			}
			done := make(chan error, 1)
			go func() { done <- worker.RunOnce(context.Background(), 1) }()
			delivery := <-claimed
			// The mutation occurs after the real worker claims the lease but before
			// its final RevalidateDelivery call.
			tc.mutate(db)
			close(continueRun)
			require.NoError(t, <-done)
			require.Zero(t, spy.count())
			var state struct {
				State      string
				Fence      int64
				LeaseOwner string
			}
			require.NoError(t, db.Table("mobile_notification_intents").Select("state, fence, lease_owner").Where("id = ?", delivery.ID).Take(&state).Error)
			require.Equal(t, "pending", state.State)
			require.Equal(t, int64(1), state.Fence)
			require.Empty(t, state.LeaseOwner)
		})
	}
}

func TestNotificationDeliveryConcurrentWorkersSendExactlyOnce(t *testing.T) {
	store, db := seedDeliveryFixture(t, "concurrent-device")
	spy := &notificationProviderSpy{}
	workerA := NewNotificationDeliveryWorker(store, spy, "worker-a")
	workerB := NewNotificationDeliveryWorker(store, spy, "worker-b")
	workerA.lease = 25 * time.Millisecond
	claimed := make(chan repository.NotificationDelivery, 1)
	releaseA := make(chan struct{})
	workerA.afterClaim = func(_ context.Context, d repository.NotificationDelivery) {
		claimed <- d
		<-releaseA
	}
	aDone := make(chan error, 1)
	go func() { aDone <- workerA.RunOnce(context.Background(), 1) }()
	deliveryA := <-claimed
	// Expire A's lease while A is paused before final authorization. B then
	// acquires a new fence and is the only worker allowed to send/ack.
	require.NoError(t, db.Exec("UPDATE mobile_notification_intents SET lease_until = ? WHERE id = ?", time.Now().Add(-time.Second), deliveryA.ID).Error)
	require.NoError(t, workerB.RunOnce(context.Background(), 1))
	close(releaseA)
	require.NoError(t, <-aDone)
	require.Equal(t, 1, spy.count(), "only the winning fence may reach the provider")
	var state struct {
		State      string
		Fence      int64
		LeaseOwner string
	}
	require.NoError(t, db.Table("mobile_notification_intents").Select("state, fence, lease_owner").Where("id = ?", deliveryA.ID).Take(&state).Error)
	require.Equal(t, "sent", state.State)
	require.Equal(t, int64(2), state.Fence)
	require.Equal(t, "", state.LeaseOwner)
	require.False(t, store.Ack(context.Background(), deliveryA.ID, "worker-a", deliveryA.Fence), "stale worker cannot ack winner")
	require.False(t, store.Retry(context.Background(), deliveryA.ID, "worker-a", deliveryA.Fence), "stale worker cannot retry winner")
}

func TestNotificationDeliveryBatchRetriesOnlyFailedItem(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	runs := repository.NewAgentRunStore(db)
	_, err := runs.Admit(context.Background(), deliveryTestAdmission("batch-run"))
	require.NoError(t, err)
	for _, device := range []string{"batch-a", "batch-b"} {
		require.NoError(t, db.Exec(`INSERT INTO mobile_devices (tenant_id, owner_id, device_id, environment, platform, token_ciphertext, token_hash) VALUES (1,'u1',?,'dev','ios','cipher',?)`, device, "hash-"+device).Error)
		store := repository.NewNotificationStore(db)
		require.NoError(t, store.Enqueue(context.Background(), repository.NotificationIntent{TenantID: 1, EventID: "batch-event-" + device, OwnerID: "u1", DeviceID: device, Environment: "dev", Kind: "completed", RunID: "batch-run", ExpiresAt: time.Now().Add(time.Hour)}))
	}
	store := repository.NewNotificationStore(db)
	provider := &batchNotificationProvider{failIndex: 1, failOnce: true}
	worker := NewNotificationDeliveryWorker(store, provider, "batch-worker")
	// Exercise the public claim/RunOnce path. The provider fails only its
	// second item on the first call; the next claim must select that row alone.
	require.NoError(t, worker.RunOnce(context.Background(), 10))
	require.Len(t, provider.calls, 1)
	require.Len(t, provider.calls[0], 2)
	failedID := provider.calls[0][1]
	require.NoError(t, db.Exec(`UPDATE mobile_notification_intents SET next_attempt_at=NULL WHERE id=?`, failedID).Error)
	require.NoError(t, worker.RunOnce(context.Background(), 10))
	require.Len(t, provider.calls, 2)
	require.Equal(t, []string{failedID}, provider.calls[1], "only the failed delivery is retried")
	var states []struct{ ID, State string }
	require.NoError(t, db.Table("mobile_notification_intents").Select("id,state").Order("id").Find(&states).Error)
	require.Len(t, states, 2)
	for _, state := range states {
		require.Equal(t, "sent", state.State)
	}
}

func TestNotificationDeliveryPermanentProviderErrorDoesNotRevokeDevice(t *testing.T) {
	store, db := seedDeliveryFixture(t, "provider-config-device")
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS mobile_notification_provider_state (provider_key TEXT PRIMARY KEY, paused INTEGER NOT NULL DEFAULT 0, reason TEXT NOT NULL DEFAULT '', alert_count INTEGER NOT NULL DEFAULT 0, paused_at DATETIME, recovered_at DATETIME, updated_at DATETIME NOT NULL)`).Error)
	health := repository.NewNotificationProviderStateStore(db)
	revoker := repository.NewMobileDeviceStore(db, "dev")
	worker := NewNotificationDeliveryWorkerWithHealth(store, &notificationProviderSpy{}, "provider-config-worker", revoker, health, "mobile")
	deliveries, err := store.Claim(context.Background(), "provider-config-worker", 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, deliveries, 1)
	err = worker.releaseDeliveryWithCause(context.Background(), deliveries[0], &pushnotification.ProviderError{Code: "InvalidCredentials", Revoke: false, Retry: false})
	require.NoError(t, err)
	state, stateErr := health.State(context.Background(), "mobile")
	require.NoError(t, stateErr)
	require.True(t, state.Paused)
	_, err = revoker.GetActiveForTenant(context.Background(), 1, "u1", "provider-config-device")
	require.NoError(t, err, "provider-wide configuration failure must preserve registration")
}

func TestInvalidNotificationEndpointPausesDurablyAndDoesNotClaim(t *testing.T) {
	store, db := seedDeliveryFixture(t, "invalid-endpoint-device")
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS mobile_notification_provider_state (provider_key TEXT PRIMARY KEY, paused INTEGER NOT NULL DEFAULT 0, reason TEXT NOT NULL DEFAULT '', alert_count INTEGER NOT NULL DEFAULT 0, paused_at DATETIME, recovered_at DATETIME, updated_at DATETIME NOT NULL)`).Error)
	health := repository.NewNotificationProviderStateStore(db)
	provider := NewHTTPNotificationProvider("://bad")
	worker := NewNotificationDeliveryWorkerWithHealth(store, provider, "invalid-endpoint-worker", nil, health, "mobile")
	require.NoError(t, worker.RunOnce(context.Background(), 10))
	state, err := health.State(context.Background(), "mobile")
	require.NoError(t, err)
	require.True(t, state.Paused)
	require.EqualValues(t, 1, state.AlertCount)
	// A repeated pass observes the durable pause before Claim and must not call
	// the malformed provider or create another alert.
	require.NoError(t, worker.RunOnce(context.Background(), 10))
	state, err = health.State(context.Background(), "mobile")
	require.NoError(t, err)
	require.EqualValues(t, 1, state.AlertCount)
	var delivery struct{ State string }
	require.NoError(t, db.Table("mobile_notification_intents").Select("state").Order("id").Take(&delivery).Error)
	require.Equal(t, "pending", delivery.State)
}

func TestNotificationProviderPausePersistsAndRecovers(t *testing.T) {
	store, db := seedDeliveryFixture(t, "pause-device")
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS mobile_notification_provider_state (provider_key TEXT PRIMARY KEY, paused INTEGER NOT NULL DEFAULT 0, reason TEXT NOT NULL DEFAULT '', alert_count INTEGER NOT NULL DEFAULT 0, paused_at DATETIME, recovered_at DATETIME, updated_at DATETIME NOT NULL)`).Error)
	health := repository.NewNotificationProviderStateStore(db)
	worker := NewNotificationDeliveryWorkerWithHealth(store, NewHTTPNotificationProvider(""), "pause-worker", nil, health, "mobile")
	require.NoError(t, worker.RunOnce(context.Background(), 10))
	state, err := health.State(context.Background(), "mobile")
	require.NoError(t, err)
	require.True(t, state.Paused)
	require.EqualValues(t, 1, state.AlertCount)
	// Restart with a configured provider: the durable pause is cleared before
	// claiming, and a successful send records recovery.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"r1","status":"ok"}`))
	}))
	defer server.Close()
	worker = NewNotificationDeliveryWorkerWithHealth(store, NewHTTPNotificationProvider(server.URL), "pause-worker-2", nil, health, "mobile")
	require.NoError(t, worker.RunOnce(context.Background(), 10))
	state, err = health.State(context.Background(), "mobile")
	require.NoError(t, err)
	require.False(t, state.Paused)
}

func TestHTTPNotificationProviderSendsScopedIdentityAndFailsClosed(t *testing.T) {
	seen := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		seen <- body
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	provider := NewHTTPNotificationProvider(server.URL)
	require.NoError(t, provider.Send(context.Background(), repository.NotificationDelivery{
		Attempt: 2,
		Intent:  repository.NotificationIntent{TenantID: 7, OwnerID: "u1", DeviceID: "d1", Environment: "dev", EventID: "e1", RunID: "r1", Kind: "completed"},
	}))
	select {
	case body := <-seen:
		require.EqualValues(t, 7, body["tenant_id"])
		require.Equal(t, "d1", body["device_id"])
		require.NotContains(t, body, "token_ciphertext")
	case <-time.After(time.Second):
		t.Fatal("provider request was not observed")
	}
	require.Error(t, NewHTTPNotificationProvider("").Send(context.Background(), repository.NotificationDelivery{}))
}

func TestHTTPNotificationProviderPropagatesRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header string
		check  func(time.Duration) bool
	}{
		{name: "seconds", header: "7", check: func(got time.Duration) bool { return got == 7*time.Second }},
		{name: "http-date", header: time.Now().Add(3 * time.Second).UTC().Format(http.TimeFormat), check: func(got time.Duration) bool { return got > 0 && got <= 3*time.Second }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", tc.header)
				w.WriteHeader(http.StatusTooManyRequests)
			}))
			defer srv.Close()
			_, err := NewHTTPNotificationProvider(srv.URL).SendReceipt(context.Background(), repository.NotificationDelivery{})
			var providerErr *pushnotification.ProviderError
			require.ErrorAs(t, err, &providerErr)
			require.True(t, providerErr.Retry)
			require.True(t, tc.check(providerErr.RetryAfter), "retry-after=%s", providerErr.RetryAfter)
		})
	}
}

func TestNotificationDeliveryPermanentProviderErrorRevokesDevice(t *testing.T) {
	store, db := seedDeliveryFixture(t, "permanent-device")
	revoker := repository.NewMobileDeviceStore(db, "dev")
	worker := NewNotificationDeliveryWorkerWithRevoker(store, &notificationProviderSpy{}, "permanent-worker", revoker)
	deliveries, err := store.Claim(context.Background(), "permanent-worker", 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, deliveries, 1)
	err = worker.releaseDeliveryWithCause(context.Background(), deliveries[0], &pushnotification.ProviderError{Code: "DeviceNotRegistered", Revoke: true, Retry: false})
	require.NoError(t, err)
	_, getErr := revoker.GetActiveForTenant(context.Background(), 1, "u1", "permanent-device")
	require.ErrorIs(t, getErr, repository.ErrMobileDeviceNotFound)
}

func TestNotificationDeliveryPermanentFailureDoesNotRevokeReboundRegistration(t *testing.T) {
	store, db := seedDeliveryFixture(t, "rebound-device")
	revoker := repository.NewMobileDeviceStore(db, "dev")
	worker := NewNotificationDeliveryWorkerWithRevoker(store, &notificationProviderSpy{}, "rebound-worker", revoker)
	deliveries, err := store.Claim(context.Background(), "rebound-worker", 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, deliveries, 1)
	require.Equal(t, int64(1), deliveries[0].DeviceRevision)
	// The provider failed for revision 1, but the user rebinds the same
	// installation before the failure callback runs. The CAS revoke must
	// reject revision 1 and preserve the replacement registration.
	require.NoError(t, revoker.Bind(context.Background(), repository.DeviceRegistration{
		TenantID: 1, OwnerID: "u1", DeviceID: "rebound-device", Environment: "dev", Platform: "ios",
		TokenCiphertext: "cipher-v2", TokenHash: repository.DeviceTokenHash("token-v2"), Revision: 1, ScopeGeneration: 0,
	}))
	err = worker.releaseDeliveryWithCause(context.Background(), deliveries[0], &pushnotification.ProviderError{Code: "DeviceNotRegistered", Revoke: true, Retry: false})
	require.ErrorIs(t, err, repository.ErrMobileDeviceRevision)
	active, getErr := revoker.GetActiveForTenant(context.Background(), 1, "u1", "rebound-device")
	require.NoError(t, getErr)
	require.Equal(t, int64(2), active.Revision)
	require.Equal(t, repository.DeviceTokenHash("token-v2"), active.TokenHash)
}

func deliveryTestAdmission(runID string) agentruntime.Admission {
	return agentruntime.Admission{
		Key: agentruntime.RunKey{TenantID: 1, RunID: runID}, SessionID: "s1", UserID: "u1",
		RequestID: "request-" + runID, AssistantMessageID: "assistant-" + runID,
		RequestHash: "hash-" + runID, Snapshot: json.RawMessage(`{"version":1}`),
		UserMessage:      json.RawMessage(`{"role":"user","content":"hi"}`),
		AssistantMessage: json.RawMessage(`{"role":"assistant","content":""}`),
		Deadline:         time.Now().Add(time.Hour),
	}
}

type notificationProviderFunc func(context.Context, repository.NotificationDelivery) error

func (f notificationProviderFunc) Send(ctx context.Context, d repository.NotificationDelivery) error {
	return f(ctx, d)
}

func TestNotificationDeliveryRejectsResolvedInteractionAfterClaim(t *testing.T) {
	store, db := seedDeliveryFixture(t, "interaction-device")
	ctx := context.Background()
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS workbench_interactions (
		tenant_id INTEGER NOT NULL, id TEXT NOT NULL, run_id TEXT NOT NULL, owner_id TEXT NOT NULL,
		kind TEXT NOT NULL, args_hash TEXT NOT NULL DEFAULT '', decision_id TEXT NOT NULL DEFAULT '',
		action TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'pending', expected_revision INTEGER NOT NULL DEFAULT 0,
		expires_at DATETIME, revoked BOOLEAN NOT NULL DEFAULT 0, created_at DATETIME, updated_at DATETIME,
		PRIMARY KEY (tenant_id, id))`).Error)
	require.NoError(t, db.Exec(`INSERT INTO workbench_interactions
		(tenant_id,id,run_id,owner_id,kind,args_hash,status,expires_at)
		VALUES (1,'interaction-1','delivery-run','u1','tool_approval','hash','pending',?)`, time.Now().Add(time.Hour)).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_run_events
		(tenant_id,run_id,seq,attempt_id,event_type,payload) VALUES (1,'delivery-run',1,'a','interaction_requested','{"pending_id":"interaction-1"}')`).Error)
	res := db.Exec(`UPDATE mobile_notification_intents SET event_id = ?, kind = 'interaction_requested' WHERE id = ?`, "1:delivery-run:1", "1:delivery-event-interaction-device:u1:interaction-device:dev")
	require.NoError(t, res.Error)
	require.EqualValues(t, 1, res.RowsAffected)
	spy := &notificationProviderSpy{}
	worker := NewNotificationDeliveryWorker(store, spy, "interaction-worker")
	worker.afterClaim = func(_ context.Context, _ repository.NotificationDelivery) {
		require.NoError(t, db.Exec(`UPDATE workbench_interactions SET status='resolved', decision_id='decision-1', action='approve', expected_revision=1 WHERE tenant_id=1 AND id='interaction-1'`).Error)
	}
	require.NoError(t, worker.RunOnce(ctx, 1))
	require.Zero(t, spy.count())
	var state struct{ State, LeaseOwner string }
	require.NoError(t, db.Table("mobile_notification_intents").Select("state, lease_owner").Where("kind = ?", "interaction_requested").Take(&state).Error)
	require.Equal(t, "pending", state.State)
	require.Empty(t, state.LeaseOwner)
}

func TestNotificationDeliveryReportsLostAckFence(t *testing.T) {
	store, db := seedDeliveryFixture(t, "ack-fence-device")
	provider := notificationProviderFunc(func(_ context.Context, d repository.NotificationDelivery) error {
		return db.Exec(`UPDATE mobile_notification_intents SET fence = fence + 1 WHERE id = ?`, d.ID).Error
	})
	worker := NewNotificationDeliveryWorker(store, provider, "ack-fence-worker")
	err := worker.RunOnce(context.Background(), 1)
	require.Error(t, err)
	require.ErrorContains(t, err, "ack_fence_lost")
}

func TestNotificationDeliveryReportsLostRetryFence(t *testing.T) {
	store, db := seedDeliveryFixture(t, "retry-fence-device")
	provider := notificationProviderFunc(func(_ context.Context, d repository.NotificationDelivery) error {
		require.NoError(t, db.Exec(`UPDATE mobile_notification_intents SET fence = fence + 1 WHERE id = ?`, d.ID).Error)
		return fmt.Errorf("provider_unavailable")
	})
	worker := NewNotificationDeliveryWorker(store, provider, "retry-fence-worker")
	err := worker.RunOnce(context.Background(), 1)
	require.Error(t, err)
	require.ErrorContains(t, err, "retry_fence_lost")
}

func TestNotificationDeliveryDoesNotMaskRetryDatabaseError(t *testing.T) {
	store, db := seedDeliveryFixture(t, "retry-database-error-device")
	sqlDB, err := db.DB()
	require.NoError(t, err)
	provider := notificationProviderFunc(func(_ context.Context, _ repository.NotificationDelivery) error {
		// The provider has already been called and the delivery is still leased.
		// Closing the underlying connection makes the following Retry fail with a
		// real persistence error, rather than an expected zero-row fence miss.
		_ = sqlDB.Close()
		return fmt.Errorf("provider_unavailable_after_close")
	})
	worker := NewNotificationDeliveryWorker(store, provider, "retry-database-error-worker")
	err = worker.RunOnce(context.Background(), 1)
	require.Error(t, err)
	require.ErrorContains(t, err, "notification_delivery_retry_persist")
}

func TestNotificationRetryBackoffIsBoundedAndJittered(t *testing.T) {
	first := notificationRetryDelay("delivery-a", 1, 0)
	second := notificationRetryDelay("delivery-a", 2, 0)
	if first < time.Second || first >= 2*time.Second {
		t.Fatalf("first backoff=%s", first)
	}
	if second < 2*time.Second || second >= 4*time.Second {
		t.Fatalf("second backoff=%s", second)
	}
	if got := notificationRetryDelay("delivery-a", 99, 10*time.Minute); got != 5*time.Minute {
		t.Fatalf("cap=%s", got)
	}
	if got := notificationRetryDelay("delivery-a", 1, 5*time.Second); got != 5*time.Second {
		t.Fatalf("retry-after hint=%s", got)
	}
}
