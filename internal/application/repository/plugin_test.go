package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newPluginPreviewTestDB 在临时文件 SQLite 上 AutoMigrate PluginPreview，
// 驱动 plugin_previews 的原子消费守卫测试（先例 agent_marketplace_test.go
// 的文件库模式；列宽语义以 PG varchar(512) 为准，此处只测行为分支）。
func newPluginPreviewTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "plugin_previews.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.PluginPreview{}))
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func newPluginPreviewRow(id string, tenantID uint64, expiresAt time.Time, consumedAt *time.Time) *types.PluginPreview {
	return &types.PluginPreview{
		ID:                  id,
		TenantID:            tenantID,
		ManifestURL:         "https://plugins.example.com/manifest.json",
		PluginID:            "com.example.jira-todo",
		Version:             "1.2.0",
		Name:                "Jira 本周待办",
		TransportType:       "http-streamable",
		EndpointURL:         "https://plugins.example.com/mcp",
		ToolsSnapshot:       types.PluginPreviewTools{{Name: "search_my_week_issues", InputSchemaDigest: "d"}},
		ToolsDigest:         "digest",
		IdentityFingerprint: "fingerprint",
		CreatedBy:           "admin-1",
		ExpiresAt:           expiresAt,
		ConsumedAt:          consumedAt,
	}
}

// TestMarkPreviewConsumedGuards：一次性消费的原子守卫必须同时涵盖
// "未消费"与"未过期"——已消费、已过期、不存在三种情况统一收敛为
// gorm.ErrRecordNotFound（T02-R1-3：TTL 判定并入原子 UPDATE，
// 消除 GetPreview+Expired 与 MarkPreviewConsumed 之间的 check-then-act 竞态）。
func TestMarkPreviewConsumedGuards(t *testing.T) {
	ctx := context.Background()
	db := newPluginPreviewTestDB(t)
	repo := NewPluginRepository(db)

	now := time.Now()
	live := newPluginPreviewRow("preview-live", 7, now.Add(time.Minute), nil)
	consumedAt := now.Add(-time.Minute)
	alreadyConsumed := newPluginPreviewRow("preview-consumed", 7, now.Add(time.Minute), &consumedAt)
	expired := newPluginPreviewRow("preview-expired", 7, now.Add(-time.Minute), nil)
	for _, row := range []*types.PluginPreview{live, alreadyConsumed, expired} {
		require.NoError(t, db.Create(row).Error)
	}

	// 未过期未消费 → 消费成功，ConsumedAt 落库。
	require.NoError(t, repo.MarkPreviewConsumed(ctx, 7, "preview-live"))
	var got types.PluginPreview
	require.NoError(t, db.Where("id = ?", "preview-live").First(&got).Error)
	require.NotNil(t, got.ConsumedAt)

	// 已消费 → 拒绝（一次性）。
	require.ErrorIs(t, repo.MarkPreviewConsumed(ctx, 7, "preview-consumed"), gorm.ErrRecordNotFound)
	// 已过期未消费 → 同样拒绝（TTL 并入原子守卫；这是 T02-R1-3 的核心断言）。
	require.ErrorIs(t, repo.MarkPreviewConsumed(ctx, 7, "preview-expired"), gorm.ErrRecordNotFound)
	// 不存在 → 拒绝。
	require.ErrorIs(t, repo.MarkPreviewConsumed(ctx, 7, "preview-absent"), gorm.ErrRecordNotFound)
	// 跨租户不可见。
	otherTenant := newPluginPreviewRow("preview-other-tenant", 8, now.Add(time.Minute), nil)
	require.NoError(t, db.Create(otherTenant).Error)
	require.ErrorIs(t, repo.MarkPreviewConsumed(ctx, 7, "preview-other-tenant"), gorm.ErrRecordNotFound)

	// 被拒绝的行 consumed_at 保持原状（过期行未被置位）。
	var expiredRow types.PluginPreview
	require.NoError(t, db.Where("id = ?", "preview-expired").First(&expiredRow).Error)
	require.Nil(t, expiredRow.ConsumedAt)
}

// TestDeleteExpiredPreviewsDropsOnlyExpiredRows（整分支 OCR 一轮 F1）：预览
// 是 TTL 绑定的临时审阅工件——过期行（无论是否已消费）必须可删，未过期行
// 不受清理影响。参数化删除（gorm 占位符），对齐 repository/resource.go:113
// DeleteExpiredGrants 先例。
func TestDeleteExpiredPreviewsDropsOnlyExpiredRows(t *testing.T) {
	ctx := context.Background()
	db := newPluginPreviewTestDB(t)
	repo := NewPluginRepository(db)

	now := time.Now()
	rows := []*types.PluginPreview{
		newPluginPreviewRow("expired-unconsumed", 7, now.Add(-time.Minute), nil),
		newPluginPreviewRow("expired-consumed", 7, now.Add(-time.Minute), &now),
		newPluginPreviewRow("live", 7, now.Add(time.Minute), nil),
	}
	for _, row := range rows {
		require.NoError(t, db.Create(row).Error)
	}

	require.NoError(t, repo.DeleteExpiredPreviews(ctx, now))

	count := func(id string) int64 {
		var n int64
		require.NoError(t, db.Model(&types.PluginPreview{}).Where("id = ?", id).Count(&n).Error)
		return n
	}
	require.EqualValues(t, 0, count("expired-unconsumed"), "expired unconsumed row must be dropped")
	require.EqualValues(t, 0, count("expired-consumed"), "expired consumed row must be dropped too")
	require.EqualValues(t, 1, count("live"), "unexpired row must survive the sweep")
}

// TestGetPreviewScopesByTenant：命中返回行、未命中返回 (nil, nil)。
func TestGetPreviewScopesByTenant(t *testing.T) {
	ctx := context.Background()
	db := newPluginPreviewTestDB(t)
	repo := NewPluginRepository(db)
	require.NoError(t, db.Create(newPluginPreviewRow("preview-1", 7, time.Now().Add(time.Minute), nil)).Error)

	got, err := repo.GetPreview(ctx, 7, "preview-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "com.example.jira-todo", got.PluginID)
	require.Len(t, got.ToolsSnapshot, 1)

	got, err = repo.GetPreview(ctx, 8, "preview-1")
	require.NoError(t, err)
	require.Nil(t, got, "cross-tenant lookup must return (nil, nil)")

	got, err = repo.GetPreview(ctx, 7, "preview-absent")
	require.NoError(t, err)
	require.Nil(t, got)
}

// TestGetByServiceIDScopesByTenant（T09）：运行时快照守卫的仓储查询——
// 同租户按 service_id 命中、跨租户与手工服务（无安装行）返回 (nil,nil)、
// 空 service_id 短路返回 (nil,nil) 不落库查询。
func TestGetByServiceIDScopesByTenant(t *testing.T) {
	ctx := context.Background()
	db := newPluginPreviewTestDB(t)
	require.NoError(t, db.AutoMigrate(&types.PluginInstallation{}))
	repo := NewPluginRepository(db)
	require.NoError(t, db.Create(&types.PluginInstallation{
		ID:              "inst-1",
		TenantID:        7,
		PluginID:        "com.example.jira-todo",
		Name:            "Jira 本周待办",
		ManifestURL:     "https://plugins.example.com/manifest.json",
		AcceptedVersion: "1.2.0",
		TransportType:   "http-streamable",
		EndpointURL:     "https://plugins.example.com/mcp",
		ToolsSnapshot:   types.PluginPreviewTools{{Name: "search_my_week_issues", InputSchemaDigest: "d"}},
		ToolsDigest:     "digest",
		ServiceID:       "svc-1",
		DriftState:      types.PluginDriftNone,
		State:           types.PluginInstallationActive,
		CreatedBy:       "admin-1",
	}).Error)

	got, err := repo.GetByServiceID(ctx, 7, "svc-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "inst-1", got.ID)
	require.Equal(t, "svc-1", got.ServiceID)

	got, err = repo.GetByServiceID(ctx, 8, "svc-1")
	require.NoError(t, err)
	require.Nil(t, got, "cross-tenant service lookup must return (nil, nil)")

	got, err = repo.GetByServiceID(ctx, 7, "manual-svc-no-row")
	require.NoError(t, err)
	require.Nil(t, got, "manual services have no installation row")

	got, err = repo.GetByServiceID(ctx, 7, "")
	require.NoError(t, err)
	require.Nil(t, got, "empty service_id short-circuits")
}
