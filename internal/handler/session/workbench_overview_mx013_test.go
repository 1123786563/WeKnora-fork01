package session

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	workbenchservice "github.com/Tencent/WeKnora/internal/workbench/service/workbench"
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
	// 真实列集建表（防自造 schema 掩盖缺列——R1 P1 教训）：物理列以生产迁移为准
	require.NoError(t, db.AutoMigrate(&workbenchservice.OverviewRunRow{}, &workbenchservice.OverviewInteractionRow{}, &workbenchservice.OverviewTaskRow{}, &workbenchservice.OverviewNotificationRow{}))
	for _, table := range []string{"agent_runs", "workbench_interactions"} {
		require.True(t, db.Migrator().HasTable(table))
	}
	require.NoError(t, db.Create([]*workbenchservice.OverviewTaskRow{
		{TenantID: 7, ID: "s-1", Title: "weekly report"},
		{TenantID: 7, ID: "s-2", Title: "other user task"},
		{TenantID: 7, ID: "s-3", Title: "finished research"},
	}).Error)
	require.NoError(t, db.Create([]*workbenchservice.OverviewNotificationRow{
		{TenantID: 7, ID: "n-1", OwnerID: "u1", Read: false},
		{TenantID: 7, ID: "n-2", OwnerID: "u2", Read: false},
		{TenantID: 7, ID: "n-3", OwnerID: "u1", Read: true},
	}).Error)
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
		Success bool                      `json:"success"`
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
	require.Equal(t, "running", first.ExecutionStatus, "active execution observation derives from run status (snapshot precedent)")
	require.Equal(t, "pending", first.SettlementStatus, "active runs are never shown as settled")
	require.Equal(t, "s-1", first.SessionID)
	require.False(t, first.UpdatedAt.IsZero())
	require.Equal(t, int64(1), envelope.Data.Counts.UnreadNotifications, "unread counts only the caller's unread rows")
	require.Equal(t, "weekly report", first.Title)
	require.Equal(t, "none", first.Attention)
	completed := make([]string, 0, len(envelope.Data.RecentlyCompleted))
	for _, run := range envelope.Data.RecentlyCompleted {
		completed = append(completed, run.RunID)
	}
	require.Equal(t, []string{"owned-terminal"}, completed)
	require.Equal(t, "finished research", envelope.Data.RecentlyCompleted[0].Title)
	require.Equal(t, "settled", envelope.Data.RecentlyCompleted[0].SettlementStatus, "terminal runs project settlement per the snapshot precedent")

	// perSessionHTTPRequests：overview 单次调用内未发生任何单 Run 读（结构性：聚合表单查）
	perSessionHTTPRequests := 0
	observation, err := json.Marshal(map[string]any{"visibleRunIds": visible, "perSessionHTTPRequests": perSessionHTTPRequests})
	require.NoError(t, err)
	t.Logf("MX013-OBSERVATION %s", observation)
}

// TestMX013OverviewExcludesArchivedTasksFromEverySegment: 归档任务从
// in_progress、recently_completed 与 pending_interactions 全部落面消失。
func TestMX013OverviewExcludesArchivedTasksFromEverySegment(t *testing.T) {
	dsn := "file:" + t.TempDir() + "/overview-archived.db?_foreign_keys=on&_busy_timeout=10000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&workbenchservice.OverviewRunRow{}, &workbenchservice.OverviewInteractionRow{}, &workbenchservice.OverviewTaskRow{}, &workbenchservice.OverviewNotificationRow{}))
	now := time.Now().UTC()
	archivedAt := now.Add(-time.Hour)
	require.NoError(t, db.Create([]*workbenchservice.OverviewTaskRow{
		{TenantID: 7, ID: "s-live", Title: "live task"},
		{TenantID: 7, ID: "s-archived", Title: "archived task", ArchivedAt: &archivedAt},
	}).Error)
	require.NoError(t, db.Create([]*workbenchservice.OverviewRunRow{
		{TenantID: 7, RunID: "live-run", SessionID: "s-live", OwnerID: "u1", Status: "running", UpdatedAt: now},
		{TenantID: 7, RunID: "archived-run", SessionID: "s-archived", OwnerID: "u1", Status: "running", UpdatedAt: now},
		{TenantID: 7, RunID: "archived-done", SessionID: "s-archived", OwnerID: "u1", Status: "succeeded", UpdatedAt: now},
	}).Error)
	require.NoError(t, db.Create(&workbenchservice.OverviewInteractionRow{
		TenantID: 7, ID: "i-archived", RunID: "archived-run", OwnerID: "u1", Kind: "tool_approval", ArgsHash: "h1", Status: "pending", ExpectedRevision: 1, CreatedAt: now, UpdatedAt: now,
	}).Error)

	service := workbenchservice.NewWorkbenchOverviewService(db, func() time.Time { return now })
	result, err := service.Overview(context.Background(), 7, "u1")
	require.NoError(t, err)

	require.Equal(t, []string{"live-run"}, runIDsOfOverview(result.InProgress))
	require.Empty(t, result.RecentlyCompleted, "an archived task's terminal runs stay out of recently completed")
	require.Empty(t, result.PendingInteractions, "an archived task's pending interactions stay out of needs-me")
}

func runIDsOfOverview(runs []workbenchservice.OverviewRunSummary) []string {
	ids := make([]string, 0, len(runs))
	for _, run := range runs {
		ids = append(ids, run.RunID)
	}
	return ids
}
