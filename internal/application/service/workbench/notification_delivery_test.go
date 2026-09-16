package workbench

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type notificationProviderSpy struct {
	mu    sync.Mutex
	sends []repository.NotificationDelivery
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
