package repository

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Executes repository-builtin migration SQL files verbatim (no external input);
// same pattern as mobile_device_test.go:25 / semantic_model_test.go:341 —
// Mimosa injection rule false positive, ruled admissible.
// openMobileAppDB 只执行本域迁移子集，顺序必须保持 000058（设备表）→ 000059/000060
// （意图表原形 + 补列）→ 000114（两表重建加 App 维度）：000114 的重建段对
// mobile_devices 与 mobile_notification_intents 做 INSERT...SELECT，二者必须已存在。
// 与 openMobileHandlerDB（internal/handler/mobile_device_test.go:20）同一聚焦模式：
// 当前 HEAD 的全目录 migrator.Up() 因 #42/#59 同号 000112 双文件损坏，不可依赖。
func openMobileAppDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "mobile-app.db")+"?_busy_timeout=5000"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	for _, file := range []string{
		"migrations/sqlite/000058_mobile_devices.up.sql",
		"migrations/sqlite/000059_mobile_notifications.up.sql",
		"migrations/sqlite/000060_mobile_notification_delivery.up.sql",
		"migrations/sqlite/000118_mobile_device_app.up.sql",
	} {
		migration, err := os.ReadFile(filepath.Join(root, file))
		require.NoError(t, err, file)
		require.NoError(t, db.Exec(
			string(migration)).Error, file)
	}
	// Claim/Revalidate/投影谓词触及的最小 agent_runs/agent_run_events 形状（仅本测试域）。
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, owner_id TEXT NOT NULL,
		PRIMARY KEY (tenant_id, run_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE agent_run_events (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, seq INTEGER NOT NULL,
		attempt_id TEXT NOT NULL DEFAULT '', event_type TEXT NOT NULL, payload TEXT NOT NULL DEFAULT '{}',
		PRIMARY KEY (tenant_id, run_id, seq))`).Error)
	require.NoError(t, db.Exec("INSERT INTO agent_runs (tenant_id, run_id, owner_id) VALUES (1, 'r1', 'u1')").Error)
	require.NoError(t, db.Exec("INSERT INTO agent_run_events (tenant_id, run_id, seq, event_type, payload) VALUES (1, 'r1', 9, 'run_completed', '{}')").Error)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func bindAppRegistration(t *testing.T, db *gorm.DB, device, token, appID string) DeviceRegistration {
	t.Helper()
	store := NewMobileDeviceStore(db, "dev")
	in := DeviceRegistration{
		TenantID: 1, OwnerID: "u1", DeviceID: device, Environment: "dev", Platform: "ios",
		TokenCiphertext: "cipher-" + token, TokenHash: DeviceTokenHash(token),
		Revision: 0, ScopeGeneration: 1, AppID: appID,
	}
	require.NoError(t, store.Bind(context.Background(), in))
	row, err := store.GetActiveForApp(context.Background(), 1, "u1", device, NormalizeMobileAppID(appID))
	require.NoError(t, err)
	return row
}

func TestValidateMobileAppID(t *testing.T) {
	require.NoError(t, ValidateMobileAppID("official"))
	require.NoError(t, ValidateMobileAppID("enterprise:acme"))
	require.NoError(t, ValidateMobileAppID("enterprise:a1-b2"))
	for _, invalid := range []string{"Official", "enterprise", "enterprise:", "enterprise:Acme", "enterprise:a_b", "enterprise:-ab", "enterprise:" + strings.Repeat("x", 33), "ios", "weknora"} {
		require.Error(t, ValidateMobileAppID(invalid), "%q must be invalid", invalid)
	}
	require.Equal(t, "official", NormalizeMobileAppID(""))
	require.Equal(t, "official", NormalizeMobileAppID("  "))
	require.Equal(t, "enterprise:acme", NormalizeMobileAppID(" enterprise:acme "))
}

func TestBindIsolatesOfficialAndEnterprise(t *testing.T) {
	db := openMobileAppDB(t)
	official := bindAppRegistration(t, db, "shared-device", "tok-official", "official")
	enterprise := bindAppRegistration(t, db, "shared-device", "tok-enterprise", "enterprise:acme")
	// 同一物理设备双 App：两行独立，revision 各自从 1 起，互不覆盖。
	require.Equal(t, "official", official.AppID)
	require.Equal(t, "enterprise:acme", enterprise.AppID)
	require.EqualValues(t, 1, official.Revision)
	require.EqualValues(t, 1, enterprise.Revision)
	// 官方行再次注册（token 轮换）只动官方行。
	rotated := bindAppRegistration(t, db, "shared-device", "tok-official-2", "official")
	require.EqualValues(t, 2, rotated.Revision)
	stillEnterprise, err := NewMobileDeviceStore(db, "dev").GetActiveForApp(context.Background(), 1, "u1", "shared-device", "enterprise:acme")
	require.NoError(t, err)
	require.EqualValues(t, 1, stillEnterprise.Revision, "official rebind must not touch the enterprise row")
}

func TestTokenExclusivityIsPerApp(t *testing.T) {
	db := openMobileAppDB(t)
	bindAppRegistration(t, db, "d-official", "same-token", "official")
	// 同一 token 注册到企业 App 的另一设备：跨 App 不互踢，两行都 active。
	bindAppRegistration(t, db, "d-enterprise", "same-token", "enterprise:acme")
	store := NewMobileDeviceStore(db, "dev")
	_, err := store.GetActiveForApp(context.Background(), 1, "u1", "d-official", "official")
	require.NoError(t, err, "cross-app registration must not revoke the official binding")
	// 同 App 内维持既有排他：官方 App 第二台设备绑定同 token，第一台被撤销。
	bindAppRegistration(t, db, "d-official-2", "same-token", "official")
	_, err = store.GetActiveForApp(context.Background(), 1, "u1", "d-official", "official")
	require.ErrorIs(t, err, ErrMobileDeviceNotFound, "same-app token takeover keeps exclusivity")
}

func TestNotificationIntentFanOutPerApp(t *testing.T) {
	db := openMobileAppDB(t)
	bindAppRegistration(t, db, "d1", "tok-a", "official")
	bindAppRegistration(t, db, "d2", "tok-b", "enterprise:acme")
	store := NewNotificationStore(db)
	require.NoError(t, store.ProjectEvent(context.Background(), RunNotificationEvent{
		TenantID: 1, OwnerID: "u1", RunID: "r1", Seq: 9, Type: "run_completed",
	}))
	var intents []struct {
		DeviceID string
		AppID    string
		State    string
	}
	require.NoError(t, db.Table("mobile_notification_intents").Select("device_id, app_id, state").Order("app_id").Find(&intents).Error)
	require.Len(t, intents, 2, "one intent per (device, app) registration")
	// Order("app_id")：'enterprise:acme' < 'official'（字典序）。
	require.Equal(t, "d2", intents[0].DeviceID)
	require.Equal(t, "enterprise:acme", intents[0].AppID)
	require.Equal(t, "d1", intents[1].DeviceID)
	require.Equal(t, "official", intents[1].AppID)
	deliveries, err := store.Claim(context.Background(), "worker-app", 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, deliveries, 2)
	apps := map[string]string{}
	for _, d := range deliveries {
		apps[d.Intent.DeviceID] = d.Intent.AppID
	}
	require.Equal(t, "official", apps["d1"])
	require.Equal(t, "enterprise:acme", apps["d2"])
}

func TestClaimJoinsAppIDSoRevokedAppDoesNotResurrect(t *testing.T) {
	db := openMobileAppDB(t)
	// 关键布局：官方与企业**同一 device_id** 各一行——若 Claim/Revalidate 的设备 EXISTS
	// 缺 app 对齐，未被撤销的官方行会让已撤销的企业投递复活。
	bindAppRegistration(t, db, "shared-x", "tok-official", "official")
	enterprise := bindAppRegistration(t, db, "shared-x", "tok-enterprise", "enterprise:acme")
	require.NoError(t, NewMobileDeviceStore(db, "dev").RevokeForApp(context.Background(), 1, "u1", "shared-x", "enterprise:acme", enterprise.Revision))
	store := NewNotificationStore(db)
	require.NoError(t, store.Enqueue(context.Background(), NotificationIntent{
		TenantID: 1, EventID: "1:r1:9", OwnerID: "u1", DeviceID: "shared-x", Environment: "dev",
		AppID: "enterprise:acme", Kind: "completed", RunID: "r1", ExpiresAt: time.Now().Add(time.Hour),
	}))
	deliveries, err := store.Claim(context.Background(), "worker-app", 10, time.Minute)
	require.NoError(t, err)
	require.Empty(t, deliveries, "a revoked enterprise registration must not claim even though an official row shares the device id")

	// RevalidateDelivery 的设备 EXISTS 同样按 app 对齐：把该行手工置为 in_flight 后，
	// 最终授权必须仍拒绝（官方行救不活企业投递）。
	var id string
	require.NoError(t, db.Raw("SELECT id FROM mobile_notification_intents LIMIT 1").Scan(&id).Error)
	require.NoError(t, db.Exec("UPDATE mobile_notification_intents SET state = 'in_flight', lease_owner = 'worker-app', fence = 1 WHERE id = ?", id).Error)
	require.False(t, store.RevalidateDelivery(context.Background(), NotificationDelivery{
		ID: id, Fence: 1,
		Intent: NotificationIntent{TenantID: 1, EventID: "1:r1:9", OwnerID: "u1", DeviceID: "shared-x", Environment: "dev", AppID: "enterprise:acme", Kind: "completed", RunID: "r1"},
	}, "worker-app"), "the final authorization seam must join on app_id, not just the device id")
}

// TestMobileDeviceAppDownMigrationsDeleteEnterpriseRows guards the rollback
// symmetry mandated by review round 1: one physical device registering under
// the official AND an enterprise app is the core scenario of this feature, so
// both down migrations must deterministically drop enterprise rows BEFORE the
// pre-app unique/primary constraints are rebuilt. Without the DELETE, the
// duplicate keys from dual-app rows fail the constraint rebuild and the
// rollback is stuck at 000114 (sqlite) / 000193 (PostgreSQL).
func TestMobileDeviceAppDownMigrationsDeleteEnterpriseRows(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	for _, tc := range []struct {
		file          string
		deviceRebuild string
	}{
		{file: "migrations/sqlite/000118_mobile_device_app.down.sql", deviceRebuild: "CREATE TABLE mobile_devices_rebuilt"},
		{file: "migrations/versioned/000197_mobile_device_app.down.sql", deviceRebuild: "ADD PRIMARY KEY (tenant_id, owner_id, device_id, environment)"},
	} {
		script, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(tc.file)))
		require.NoError(t, err, tc.file)
		sql := string(script)
		intentsDelete := strings.Index(sql, "DELETE FROM mobile_notification_intents WHERE app_id <> 'official'")
		require.GreaterOrEqual(t, intentsDelete, 0, "%s must deterministically drop enterprise intents", tc.file)
		devicesDelete := strings.Index(sql, "DELETE FROM mobile_devices WHERE app_id <> 'official'")
		require.GreaterOrEqual(t, devicesDelete, 0, "%s must deterministically drop enterprise devices", tc.file)
		intentsRebuild := strings.Index(sql, "ADD CONSTRAINT uq_mobile_notification_identity")
		if intentsRebuild < 0 {
			intentsRebuild = strings.Index(sql, "UNIQUE (tenant_id, event_id, owner_id, device_id, environment)")
		}
		require.GreaterOrEqual(t, intentsRebuild, 0, "%s must rebuild the intents identity constraint", tc.file)
		require.Less(t, intentsDelete, intentsRebuild, "%s: enterprise intents must be dropped before the identity constraint is rebuilt", tc.file)
		devicePK := strings.Index(sql, tc.deviceRebuild)
		require.GreaterOrEqual(t, devicePK, 0, "%s must rebuild the device primary key", tc.file)
		require.Less(t, devicesDelete, devicePK, "%s: enterprise devices must be dropped before the primary key is rebuilt", tc.file)
	}
}
