package repository_test

// T37 (#67) end-to-end evidence at the highest stable interface. Everything
// here is real: focused sqlite migration files (the full migration dir is
// broken at HEAD by duplicate 000112 from #42/#59 — see the plan's difference
// record), the real MobileDeviceHandler (two-step intent→register over gin),
// the real NotificationStore projector and delivery worker, and real HTTP
// provider boundaries (gateway + APNs shape) behind httptest. No mocked
// service, no hand-written projection.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/handler"
	pushnotification "github.com/Tencent/WeKnora/internal/workbench/notification"
	workbenchservice "github.com/Tencent/WeKnora/internal/workbench/service/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type pushIsolationEnv struct {
	db       *gorm.DB
	engine   *gin.Engine
	gateway  *httptest.Server
	gwBodies []map[string]any
	apns     *httptest.Server
	apnsSeen []apnsRequest
}

type apnsRequest struct {
	path     string
	pushType string
	body     map[string]any
}

const enterpriseTokenHex = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"

func newPushIsolationEnv(t *testing.T) *pushIsolationEnv {
	t.Helper()
	t.Setenv("SYSTEM_AES_KEY", "12345678901234567890123456789012")
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "push-isolation.db") + "?_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	// 顺序固定：000058 → 000059 → 000060 → 000118（000118 的重建段 INSERT...SELECT
	// 依赖前序迁移建出的 mobile_devices 与 mobile_notification_intents）。
	for _, file := range []string{
		"migrations/sqlite/000058_mobile_devices.up.sql",
		"migrations/sqlite/000059_mobile_notifications.up.sql",
		"migrations/sqlite/000060_mobile_notification_delivery.up.sql",
		"migrations/sqlite/000118_mobile_device_app.up.sql",
	} {
		raw, err := os.ReadFile(filepath.Join(root, file))
		require.NoError(t, err, file)
		require.NoError(t, db.Exec(string(raw)).Error, file)
	}
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS mobile_notification_provider_state (provider_key TEXT PRIMARY KEY, paused INTEGER NOT NULL DEFAULT 0, reason TEXT NOT NULL DEFAULT '', alert_count INTEGER NOT NULL DEFAULT 0, paused_at DATETIME, recovered_at DATETIME, updated_at DATETIME NOT NULL)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, owner_id TEXT NOT NULL, PRIMARY KEY (tenant_id, run_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE agent_run_events (tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, seq INTEGER NOT NULL, attempt_id TEXT NOT NULL DEFAULT '', event_type TEXT NOT NULL, payload TEXT NOT NULL DEFAULT '{}', PRIMARY KEY (tenant_id, run_id, seq))`).Error)
	require.NoError(t, db.Exec("INSERT INTO agent_runs (tenant_id, run_id, owner_id) VALUES (1, 'r1', 'u1')").Error)
	require.NoError(t, db.Exec("INSERT INTO agent_run_events (tenant_id, run_id, seq, event_type, payload) VALUES (1, 'r1', 9, 'run_completed', '{}')").Error)

	devices := repository.NewMobileDeviceStore(db, "dev")
	deviceHandler := handler.NewMobileDeviceHandlerWithSealer(devices, "dev", func(token string) (string, error) {
		return utils.EncryptAESGCM(token, utils.GetAESKey())
	}).WithMobileAppPolicy(handler.MobileAppPolicy{EnterpriseAppID: "enterprise:acme"})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	v1 := engine.Group("/api/v1")
	devicesGroup := v1.Group("/mobile/devices")
	devicesGroup.POST("/:id/registration-intent", deviceHandler.IssueIntent)
	devicesGroup.PUT("/:id", deviceHandler.Register)
	devicesGroup.DELETE("/:id", deviceHandler.Revoke)
	devicesGroup.GET("", deviceHandler.List)

	env := &pushIsolationEnv{db: db, engine: engine}
	env.gateway = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		env.gwBodies = append(env.gwBodies, body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"gw-receipt","status":"ok"}`))
	}))
	env.apns = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		env.apnsSeen = append(env.apnsSeen, apnsRequest{path: r.URL.Path, pushType: r.Header.Get("apns-push-type"), body: body})
		w.Header().Set("apns-unique-id", "apns-receipt")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(func() {
		env.gateway.Close()
		env.apns.Close()
		conn, _ := db.DB()
		_ = conn.Close()
	})
	return env
}

func (e *pushIsolationEnv) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := req.Context()
	ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

func (e *pushIsolationEnv) registerApp(t *testing.T, device, appID, token string) {
	t.Helper()
	w := e.do(t, http.MethodPost, "/api/v1/mobile/devices/"+device+"/registration-intent", `{"app_id":"`+appID+`"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var intent struct {
		Data struct {
			RegistrationIntent string `json:"registration_intent"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &intent))
	w = e.do(t, http.MethodPut, "/api/v1/mobile/devices/"+device, `{"token":"`+token+`","platform":"ios","app_id":"`+appID+`","registration_intent":"`+intent.Data.RegistrationIntent+`"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestMobilePushIsolationOfficialAndEnterpriseNeverMix(t *testing.T) {
	env := newPushIsolationEnv(t)
	env.registerApp(t, "do1", "official", "ExponentPushToken[official]")
	env.registerApp(t, "do2", "enterprise:acme", enterpriseTokenHex)

	// 官方注册不被企业注册干扰（AC1：注册不混用）。
	w := env.do(t, http.MethodGet, "/api/v1/mobile/devices", "")
	require.Equal(t, http.StatusOK, w.Code)
	var listed struct {
		Data []struct {
			DeviceID string `json:"device_id"`
			AppID    string `json:"app_id"`
			Revision int64  `json:"revision"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listed))
	require.Len(t, listed.Data, 2)
	for _, row := range listed.Data {
		require.EqualValues(t, 1, row.Revision, "no cross-app takeover: both rows keep revision 1")
	}

	// 投影：一个事件按 App 扇出到两个 intent（真实 NotificationStore projector）。
	store := repository.NewNotificationStore(env.db)
	require.NoError(t, store.ProjectEvent(context.Background(), repository.RunNotificationEvent{
		TenantID: 1, OwnerID: "u1", RunID: "r1", Seq: 9, Type: "run_completed",
	}))

	// 投递：official → HTTP gateway（非盲推，含 kind）；enterprise → APNs（盲推，
	// background content-available，无 kind/title）。真实 worker + 真实 HTTP 边界。
	devices := repository.NewMobileDeviceStore(env.db, "dev")
	health := repository.NewNotificationProviderStateStore(env.db)
	enterpriseDirect := workbenchservice.NewPushNotificationProviderWithOptions(
		pushnotification.NewApnsProviderWithClient(env.apns.URL, "bundle.acme.enterprise", pushnotification.NewStaticApnsTokenSource("enterprise-jwt"), env.apns.Client()),
		func(ctx context.Context, d repository.NotificationDelivery) (string, error) {
			registration, err := devices.GetActiveForApp(ctx, d.Intent.TenantID, d.Intent.OwnerID, d.Intent.DeviceID, "enterprise:acme")
			if err != nil {
				return "", err
			}
			return utils.DecryptAESGCM(registration.TokenCiphertext, utils.GetAESKey())
		},
		workbenchservice.PushPayloadPolicy{Blind: true})
	provider := workbenchservice.NewAppRoutingNotificationProvider(
		workbenchservice.NewHTTPNotificationProvider(env.gateway.URL),
		map[string]workbenchservice.NotificationProvider{"enterprise:acme": enterpriseDirect},
	)
	worker := workbenchservice.NewNotificationDeliveryWorkerWithHealth(store, provider, "isolation-worker", devices, health, "mobile")
	require.NoError(t, worker.RunOnce(context.Background(), 10))

	require.Len(t, env.gwBodies, 1, "the official token reaches only the deployment gateway")
	require.Equal(t, "do1", env.gwBodies[0]["device_id"])
	require.Equal(t, "official", env.gwBodies[0]["app_id"])
	require.Equal(t, "completed", env.gwBodies[0]["kind"])
	require.NotContains(t, env.gwBodies[0], "token", "token material never enters the gateway payload")

	require.Len(t, env.apnsSeen, 1, "the enterprise token reaches only the enterprise APNs lane")
	require.Equal(t, "/3/device/"+enterpriseTokenHex, env.apnsSeen[0].path)
	require.Equal(t, "background", env.apnsSeen[0].pushType, "enterprise lane runs blind (no lock-screen copy)")
	aps := env.apnsSeen[0].body["aps"].(map[string]any)
	require.Equal(t, float64(1), aps["content-available"])
	require.NotContains(t, aps, "alert")
	require.NotContains(t, env.apnsSeen[0].body, "kind")

	var states []struct {
		AppID string
		State string
	}
	require.NoError(t, env.db.Table("mobile_notification_intents").Select("app_id, state").Order("app_id").Find(&states).Error)
	require.Len(t, states, 2)
	for _, row := range states {
		require.Equal(t, "sent", row.State)
	}
}

func TestMobilePushDisabledKeepsIntentsDurable(t *testing.T) {
	env := newPushIsolationEnv(t)
	env.registerApp(t, "do1", "official", "ExponentPushToken[official]")
	store := repository.NewNotificationStore(env.db)
	require.NoError(t, store.ProjectEvent(context.Background(), repository.RunNotificationEvent{
		TenantID: 1, OwnerID: "u1", RunID: "r1", Seq: 9, Type: "run_completed",
	}))
	health := repository.NewNotificationProviderStateStore(env.db)
	worker := workbenchservice.NewNotificationDeliveryWorkerWithHealth(store, workbenchservice.NewDisabledNotificationProvider(), "disabled-e2e-worker", repository.NewMobileDeviceStore(env.db, "dev"), health, "mobile")
	require.NoError(t, worker.RunOnce(context.Background(), 10))
	require.Empty(t, env.gwBodies, "disabled policy never reaches the gateway")
	var pending int64
	require.NoError(t, env.db.Table("mobile_notification_intents").Where("state = 'pending'").Count(&pending).Error)
	require.EqualValues(t, 1, pending, "disabling push keeps intents durable for later policy recovery")
}
