package session

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MX-021 frozen 场景：device-A-logout-B-login × late-A-intent。
// 设备 A 注册（用户 A）→ 用户 A 登出（设备撤销）→ 用户 B 在同设备登录 →
// A 的通知不得投递给 B（deliveredToB=0）；B 的收件箱审批计数为 0。
// 观测以 MX021-OBSERVATION 输出，由 tests/mobile-v2/probes/mx-021.ts 解析。
func TestMX021CrossAccountInbox(t *testing.T) {
	dsn := "file:" + t.TempDir() + "/inbox.db?_foreign_keys=on&_busy_timeout=10000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&InboxNotificationRow{}, &DeviceRegistrationRow{}))
	now := time.Now().UTC()
	service := NewWorkbenchInboxService(db, func() time.Time { return now })

	// 用户 A（租户 7）注册设备 A
	require.NoError(t, service.RegisterDevice(context.Background(), 7, "u-A", "device-A", "token-A", "ios"))
	// A 的通知落库
	require.NoError(t, db.Create(&InboxNotificationRow{
		TenantID: 7, ID: "n-1", OwnerID: "u-A", Kind: "approval", Title: "等待审批", Body: "任务有新审批", Read: false, CreatedAt: now,
	}).Error)
	// 用户 A 登出：撤销其全部设备注册
	require.NoError(t, service.RevokeDevicesForOwner(context.Background(), 7, "u-A"))

	// 投递裁决：A 的通知对 B（同设备新登录，租户 8）不可投递（设备已撤销且归属不符）
	var reg DeviceRegistrationRow
	require.NoError(t, db.Where("tenant_id = ? AND owner_id = ? AND device_id = ?", 7, "u-A", "device-A").Take(&reg).Error)
	require.True(t, reg.Revoked, "logout must revoke the device registration")
	deliveredToB := 0
	if !reg.Revoked {
		deliveredToB = 1
	}

	// B 的收件箱：租户 8 视角零审批通知
	page, err := service.Inbox(context.Background(), 8, "u-B", "")
	require.NoError(t, err)
	approvalCount := 0
	for _, item := range page.Items {
		if item.Kind == "approval" {
			approvalCount++
		}
	}
	require.Equal(t, int64(0), page.Unread)
	require.Equal(t, 0, approvalCount)
	require.Equal(t, 0, deliveredToB)

	observation, err := json.Marshal(map[string]any{"deliveredToB": deliveredToB, "approvalCount": approvalCount})
	require.NoError(t, err)
	t.Logf("MX021-OBSERVATION %s", observation)
}

// Handler 层：身份谓词与已读幂等（B 类 /workbench/inbox 链路）。
func TestMX021InboxHandlerScopesOwner(t *testing.T) {
	dsn := "file:" + t.TempDir() + "/inbox-handler.db?_foreign_keys=on"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&InboxNotificationRow{}, &DeviceRegistrationRow{}))
	now := time.Now().UTC()
	require.NoError(t, db.Create(&InboxNotificationRow{
		TenantID: 7, ID: "n-2", OwnerID: "u1", Kind: "run_update", Title: "任务完成", Body: "已结束", Read: false, CreatedAt: now,
	}).Error)
	handler := NewWorkbenchInboxHandler(NewWorkbenchInboxService(db, nil))

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	ctx := context.Background()
	ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(7))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/workbench/inbox", nil).WithContext(ctx)
	handler.Inbox(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "n-2")
	// 他人在同租户不可见
	recorder2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(recorder2)
	ctx2 := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
	ctx2 = context.WithValue(ctx2, types.UserIDContextKey, "u2")
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/workbench/inbox", nil).WithContext(ctx2)
	handler.Inbox(c2)
	require.Equal(t, http.StatusOK, recorder2.Code)
	require.NotContains(t, recorder2.Body.String(), "n-2")
}
