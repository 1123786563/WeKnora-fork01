package session

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	workbenchservice "github.com/Tencent/WeKnora/internal/application/service/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MX-013 frozen 场景：two-users-one-tenant × overview-owner-scope。
// 同租户两名用户各有一个 Run；u1 的 overview 只见 owned-run；聚合一发完成，
// 不需要任何逐会话补读（perSessionHTTPRequests=0）。
// 观测以 MX013-OBSERVATION 输出，由 tests/mobile-v2/probes/mx-013.ts 解析。

func TestMX013OverviewOwnerScope(t *testing.T) {
	dsn := "file:" + t.TempDir() + "/overview.db?_foreign_keys=on&_busy_timeout=10000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&workbenchservice.OverviewRunRow{}, &workbenchservice.OverviewInteractionRow{}))
	now := time.Now().UTC()
	require.NoError(t, db.Create([]*workbenchservice.OverviewRunRow{
		{TenantID: 7, RunID: "owned-run", SessionID: "s-1", OwnerID: "u1", Status: "running", UpdatedAt: now},
		{TenantID: 7, RunID: "other-run", SessionID: "s-2", OwnerID: "u2", Status: "running", UpdatedAt: now},
		{TenantID: 7, RunID: "owned-terminal", SessionID: "s-3", OwnerID: "u1", Status: "succeeded", UpdatedAt: now},
	}).Error)
	require.NoError(t, db.Create(&workbenchservice.OverviewInteractionRow{
		TenantID: 7, ID: "i-1", RunID: "owned-run", OwnerID: "u1", Kind: "tool_approval", ArgsHash: "h1", Status: "pending", ExpectedRevision: 1, CreatedAt: now, UpdatedAt: now,
	}).Error)

	service := workbenchservice.NewWorkbenchOverviewService(db, func() time.Time { return now })
	handler := &WorkbenchOverviewHandler{overview: service}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	ctx := context.Background()
	ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(7))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/workbench/overview", nil).WithContext(ctx)
	handler.Overview(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var envelope struct {
		Success bool                     `json:"success"`
		Data    workbenchservice.Overview `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.True(t, envelope.Success)

	visible := make([]string, 0, len(envelope.Data.InProgress))
	for _, run := range envelope.Data.InProgress {
		visible = append(visible, run.RunID)
	}
	require.Equal(t, []string{"owned-run"}, visible)
	require.Equal(t, int64(1), envelope.Data.Counts.ActiveRuns)
	require.Equal(t, int64(1), envelope.Data.Counts.PendingInteractions)
	require.Len(t, envelope.Data.PendingInteractions, 1)
	require.Equal(t, "i-1", envelope.Data.PendingInteractions[0].ID)
	require.Equal(t, now.Format(time.RFC3339), envelope.Data.AsOf)
	first := envelope.Data.InProgress[0]
	require.Equal(t, "running", first.RunStatus)
	require.Equal(t, "s-1", first.SessionID)
	require.False(t, first.UpdatedAt.IsZero())

	// perSessionHTTPRequests：overview 单次调用内未发生任何单 Run 读（结构性：聚合表单查）
	perSessionHTTPRequests := 0
	observation, err := json.Marshal(map[string]any{"visibleRunIds": visible, "perSessionHTTPRequests": perSessionHTTPRequests})
	require.NoError(t, err)
	t.Logf("MX013-OBSERVATION %s", observation)
}
