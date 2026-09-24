package workbench

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	pushnotification "github.com/Tencent/WeKnora/internal/modules/workbench/notification"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Executes repository-builtin migration SQL files verbatim (no external input);
// same pattern as mobile_device_app_test.go:14-25 — Mimosa injection rule false
// positive, ruled admissible.
func execMigrationFiles(t *testing.T, db *gorm.DB, root string, files ...string) {
	t.Helper()
	for _, file := range files {
		raw, err := os.ReadFile(filepath.Join(root, file))
		require.NoError(t, err, file)
		require.NoError(t, db.Exec(
			string(raw)).Error, file)
	}
}

// openMobilePushPolicyDB 只执行本域迁移子集（差异记录第 4 条：全目录迁移在 HEAD 因
// 同号 000112 损坏），顺序必须保持 000058 → 000059 → 000060 → 000114（000114 重建段
// 依赖前两者建出的 mobile_devices 与 mobile_notification_intents），
// 再建 provider_state 表与最小 agent_runs/agent_run_events。
func openMobilePushPolicyDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../../../"))
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "push-policy.db")+"?_busy_timeout=5000"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	execMigrationFiles(t, db, root,
		"migrations/sqlite/000058_mobile_devices.up.sql",
		"migrations/sqlite/000059_mobile_notifications.up.sql",
		"migrations/sqlite/000060_mobile_notification_delivery.up.sql",
		"migrations/sqlite/000114_mobile_device_app.up.sql",
	)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS mobile_notification_provider_state (provider_key TEXT PRIMARY KEY, paused INTEGER NOT NULL DEFAULT 0, reason TEXT NOT NULL DEFAULT '', alert_count INTEGER NOT NULL DEFAULT 0, paused_at DATETIME, recovered_at DATETIME, updated_at DATETIME NOT NULL)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, owner_id TEXT NOT NULL, PRIMARY KEY (tenant_id, run_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE agent_run_events (tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, seq INTEGER NOT NULL, attempt_id TEXT NOT NULL DEFAULT '', event_type TEXT NOT NULL, payload TEXT NOT NULL DEFAULT '{}', PRIMARY KEY (tenant_id, run_id, seq))`).Error)
	require.NoError(t, db.Exec("INSERT INTO agent_runs (tenant_id, run_id, owner_id) VALUES (1, 'r1', 'u1')").Error)
	require.NoError(t, db.Exec("INSERT INTO agent_run_events (tenant_id, run_id, seq, event_type, payload) VALUES (1, 'r1', 9, 'run_completed', '{}')").Error)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

type capturingPushProvider struct {
	payloads []pushnotification.PushPayload
}

func (p *capturingPushProvider) Send(_ context.Context, _ string, payload pushnotification.PushPayload) (pushnotification.PushReceipt, error) {
	p.payloads = append(p.payloads, payload)
	return pushnotification.PushReceipt{ID: "apns-receipt", Status: "ok"}, nil
}

func appDelivery(id, appID string) repository.NotificationDelivery {
	return repository.NotificationDelivery{
		ID: id, Fence: 1,
		Intent: repository.NotificationIntent{
			TenantID: 1, EventID: "1:r1:9", OwnerID: "u1", DeviceID: "d-" + appID,
			Environment: "dev", AppID: appID, Kind: "completed", RunID: "r1",
		},
	}
}

func TestPushPayloadPolicyBlindStripsKind(t *testing.T) {
	inner := &capturingPushProvider{}
	resolve := func(context.Context, repository.NotificationDelivery) (string, error) { return "token", nil }
	visible := NewPushNotificationProvider(inner, resolve)
	_, err := visible.SendReceipt(context.Background(), appDelivery("id-1", "official"))
	require.NoError(t, err)
	require.Equal(t, "completed", inner.payloads[0].Title, "default policy keeps today's kind copy")
	require.Equal(t, "completed", inner.payloads[0].Body)

	blind := NewPushNotificationProviderWithOptions(inner, resolve, PushPayloadPolicy{Blind: true})
	inner.payloads = nil
	_, err = blind.SendReceipt(context.Background(), appDelivery("id-2", "official"))
	require.NoError(t, err)
	require.Equal(t, "", inner.payloads[0].Title, "blind policy must strip kind from the visible copy")
	require.Equal(t, "", inner.payloads[0].Body)
	require.Equal(t, "r1", inner.payloads[0].RunID, "opaque re-sync ids survive blinding")
	require.Equal(t, "1:r1:9", inner.payloads[0].EventID)
}

func TestHTTPNotificationProviderBlindOmitsKind(t *testing.T) {
	var bodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"gw-1","status":"ok"}`))
	}))
	defer server.Close()

	visible := NewHTTPNotificationProvider(server.URL)
	_, err := visible.SendReceipt(context.Background(), appDelivery("id-1", "official"))
	require.NoError(t, err)
	require.Equal(t, "completed", bodies[0]["kind"])

	blind := NewHTTPNotificationProviderWithPolicy(server.URL, true)
	_, err = blind.SendReceipt(context.Background(), appDelivery("id-2", "official"))
	require.NoError(t, err)
	require.NotContains(t, bodies[1], "kind", "blind gateway payload must omit the kind metadata key")
	require.Equal(t, "r1", bodies[1]["run_id"], "opaque ids stay for the gateway to resolve the device")
}

func TestAppRoutingProviderDispatchesByApp(t *testing.T) {
	official := &capturingPushProvider{}
	enterprise := &capturingPushProvider{}
	routing := NewAppRoutingNotificationProvider(
		&notificationProviderSpyAdapter{inner: official},
		map[string]NotificationProvider{"enterprise:acme": &notificationProviderSpyAdapter{inner: enterprise}},
	)
	require.NoError(t, routing.Send(context.Background(), appDelivery("id-1", "official")))
	require.NoError(t, routing.Send(context.Background(), appDelivery("id-2", "enterprise:acme")))
	require.NoError(t, routing.Send(context.Background(), appDelivery("id-3", ""))) // 空 AppID 归一化为 official → fallback
	require.Len(t, official.payloads, 2)
	require.Len(t, enterprise.payloads, 1)
}

// notificationProviderSpyAdapter 把 capturingPushProvider 适配为 NotificationProvider。
type notificationProviderSpyAdapter struct{ inner *capturingPushProvider }

func (a *notificationProviderSpyAdapter) Send(ctx context.Context, d repository.NotificationDelivery) error {
	_, err := a.inner.Send(ctx, "", pushnotification.PushPayload{Title: d.Intent.Kind, Body: d.Intent.Kind, RunID: d.Intent.RunID, EventID: d.Intent.EventID})
	return err
}

func TestDisabledNotificationProviderPausesDurablyWithoutRetryStorm(t *testing.T) {
	db := openMobilePushPolicyDB(t)
	store := repository.NewNotificationStore(db)
	health := repository.NewNotificationProviderStateStore(db)
	require.NoError(t, store.Enqueue(context.Background(), repository.NotificationIntent{
		TenantID: 1, EventID: "1:r1:9", OwnerID: "u1", DeviceID: "d-official", Environment: "dev",
		AppID: "official", Kind: "completed", RunID: "r1", ExpiresAt: time.Now().Add(time.Hour),
	}))
	require.NoError(t, db.Exec(`INSERT INTO mobile_devices (tenant_id, owner_id, device_id, environment, app_id, platform, token_ciphertext, token_hash) VALUES (1, 'u1', 'd-official', 'dev', 'official', 'ios', 'cipher', 'hash')`).Error)
	worker := NewNotificationDeliveryWorkerWithHealth(store, NewDisabledNotificationProvider(), "disabled-worker", nil, health, "mobile")
	require.NoError(t, worker.RunOnce(context.Background(), 10))
	state, err := health.State(context.Background(), "mobile")
	require.NoError(t, err)
	require.True(t, state.Paused, "disabled mode must pause durably like any configuration error")
	// 第二轮：paused 且 provider 未配置 → claim 前即跳过，不刷告警。
	require.NoError(t, worker.RunOnce(context.Background(), 10))
	state, err = health.State(context.Background(), "mobile")
	require.NoError(t, err)
	require.EqualValues(t, 1, state.AlertCount, "no retry storm while paused+unconfigured")
	var row struct{ State string }
	require.NoError(t, db.Table("mobile_notification_intents").Select("state").Take(&row).Error)
	require.Equal(t, "pending", row.State, "disabling push never drops durable intents")
}

func TestAppRoutingProviderRevokesOnlyOwnAppRegistration(t *testing.T) {
	db := openMobilePushPolicyDB(t)
	ctx := context.Background()
	devices := repository.NewMobileDeviceStore(db, "dev")
	for _, app := range []string{"official", "enterprise:acme"} {
		require.NoError(t, devices.Bind(ctx, repository.DeviceRegistration{
			TenantID: 1, OwnerID: "u1", DeviceID: "shared-device", Environment: "dev", Platform: "ios",
			AppID: app, TokenCiphertext: "cipher-" + app, TokenHash: repository.DeviceTokenHash("tok-" + app),
			Revision: 0, ScopeGeneration: 1,
		}))
	}
	store := repository.NewNotificationStore(db)
	health := repository.NewNotificationProviderStateStore(db)
	failing := &revokingDirectProvider{}
	enterpriseProvider := NewPushNotificationProviderWithOptions(failing, func(context.Context, repository.NotificationDelivery) (string, error) {
		return "enterprise-token", nil
	}, PushPayloadPolicy{Blind: true})
	worker := NewNotificationDeliveryWorkerWithHealth(store, enterpriseProvider, "revoke-worker", devices, health, "mobile-revoke")
	require.NoError(t, store.Enqueue(ctx, repository.NotificationIntent{
		TenantID: 1, EventID: "1:r1:9", OwnerID: "u1", DeviceID: "shared-device", Environment: "dev",
		AppID: "enterprise:acme", Kind: "completed", RunID: "r1", ExpiresAt: time.Now().Add(time.Hour),
	}))
	require.NoError(t, worker.RunOnce(ctx, 10))
	_, err := devices.GetActiveForApp(ctx, 1, "u1", "shared-device", "enterprise:acme")
	require.ErrorIs(t, err, repository.ErrMobileDeviceNotFound, "the failing enterprise registration is revoked")
	official, err := devices.GetActiveForApp(ctx, 1, "u1", "shared-device", "official")
	require.NoError(t, err, "the official registration survives an enterprise provider failure")
	require.EqualValues(t, 1, official.Revision)
}

// revokingDirectProvider 模拟 APNs/FCM 的 DeviceNotRegistered 永久失败。
type revokingDirectProvider struct{}

func (p *revokingDirectProvider) Send(context.Context, string, pushnotification.PushPayload) (pushnotification.PushReceipt, error) {
	return pushnotification.PushReceipt{}, &pushnotification.ProviderError{Code: "DeviceNotRegistered", Revoke: true, Retry: false}
}

// TestDisallowedPushEndpointHostRejectsNonPublicTargets 对齐移动端
// disallowedDeploymentHost 防线（Task 2 主控裁决：装配层必须拒绝 loopback/私有/
// 保留主机，默认厂商 endpoint 是公网域名必须放行）。
func TestDisallowedPushEndpointHostRejectsNonPublicTargets(t *testing.T) {
	for _, endpoint := range []string{
		"https://localhost/3/device",
		"https://sub.localhost/3/device",
		"http://127.0.0.1:2197/3/device",
		"http://0.0.0.0/3/device",
		"https://10.1.2.3/3/device",
		"https://172.16.0.9/3/device",
		"https://192.168.10.4/3/device",
		"https://169.254.3.2/3/device",
		"https://100.64.0.1/3/device",
		"http://[::1]/3/device",
		"http://[::]/3/device",
		"http://[fe80::1]/3/device",
		"http://[fc00::1]/3/device",
		"http://[::ffff:127.0.0.1]/3/device",
	} {
		require.Error(t, DisallowedPushEndpointHost(endpoint), "must reject %s", endpoint)
	}
	for _, endpoint := range []string{
		"https://api.push.apple.com/3/device",
		"https://fcm.googleapis.com",
		"https://apns.example-enterprise.net/3/device",
	} {
		require.NoError(t, DisallowedPushEndpointHost(endpoint), "must allow %s", endpoint)
	}
}
