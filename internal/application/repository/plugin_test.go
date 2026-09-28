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

// TestCreateInstallationDuplicateKeySentinel（OCR 一轮 R12 F15）：并发确认
// 的输家在 (tenant_id, plugin_id) 唯一索引处收到约束冲突——仓储必须改写
// 为 ErrInstallationDuplicateKey 哨兵（服务层据此映射 409），不得把原始
// 驱动错误透传成笼统 500。AutoMigrate 依 gorm tag 建出唯一索引。
func TestCreateInstallationDuplicateKeySentinel(t *testing.T) {
	ctx := context.Background()
	db := newPluginPreviewTestDB(t)
	require.NoError(t, db.AutoMigrate(&types.PluginInstallation{}))
	repo := NewPluginRepository(db)
	row := &types.PluginInstallation{
		ID: "inst-dup", TenantID: 7, PluginID: "com.example.jira-todo",
		Name: "Jira", ManifestURL: "https://x.example.com/m.json", AcceptedVersion: "1.0.0",
		TransportType: "http-streamable", EndpointURL: "https://x.example.com/mcp",
		ToolsSnapshot: types.PluginPreviewTools{{Name: "t", InputSchemaDigest: "d"}},
		ToolsDigest:   "digest", ServiceID: "svc", State: types.PluginInstallationActive, CreatedBy: "a",
	}
	require.NoError(t, repo.CreateInstallation(ctx, row))

	// 同 (tenant, plugin) 第二行：SQLite 报 UNIQUE constraint failed → 哨兵。
	dup := *row
	dup.ID = "inst-dup-2"
	err := repo.CreateInstallation(ctx, &dup)
	require.ErrorIs(t, err, ErrInstallationDuplicateKey)

	// 不同租户不受唯一索引约束。
	dup3 := *row
	dup3.ID = "inst-dup-3"
	dup3.TenantID = 8
	require.NoError(t, repo.CreateInstallation(ctx, &dup3))
}

// TestHardDeleteServiceCascadeCleansOAuthRows（T06-OCR1-F5）：以 service_id
// 键控的派生表不止 mcp_tool_approvals——mcp_oauth_tokens（成员个人令牌，
// AES-256-GCM 加密）与 mcp_oauth_clients 同键控。卸载硬删 mcp_services 后
// 这些行若不被级联清理，敏感凭据在服务已删后无限期滞留（旗舰 Jira 场景：
// 成员授权后管理员卸载必然命中）。手工服务的行原样保留。mcp_metadata 为
// PG-only 表（SQLite 面不存在），由方言分支跳过。
func TestHardDeleteServiceCascadeCleansOAuthRows(t *testing.T) {
	ctx := context.Background()
	db := newPluginPreviewTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&types.MCPService{}, &types.MCPToolApproval{},
		&types.MCPOAuthToken{}, &types.MCPOAuthClient{},
	))
	repo := NewPluginRepository(db)

	mkService := func(id string) *types.MCPService {
		return &types.MCPService{ID: id, TenantID: 7, Name: "svc-" + id, TransportType: "http"}
	}
	require.NoError(t, db.Create(mkService("svc-plugin")).Error)
	require.NoError(t, db.Create(mkService("svc-manual")).Error)
	for _, svc := range []string{"svc-plugin", "svc-manual"} {
		require.NoError(t, db.Create(&types.MCPToolApproval{
			ID: "appr-" + svc, TenantID: 7, ServiceID: svc, ToolName: "tool-a",
		}).Error)
		// UserID/PrincipalID 唯一索引（tenant, principal, service）——每服务唯一值。
		require.NoError(t, db.Create(&types.MCPOAuthToken{
			ID: "tok-" + svc, TenantID: 7, UserID: "user-" + svc, ServiceID: svc,
			PrincipalType: "user", PrincipalID: "user-" + svc, AccessToken: "tok",
		}).Error)
		require.NoError(t, db.Create(&types.MCPOAuthClient{
			ID: "cli-" + svc, TenantID: 7, ServiceID: svc, ClientID: "cid-" + svc,
		}).Error)
	}

	require.NoError(t, repo.HardDeleteServiceCascade(ctx, 7, "svc-plugin"))

	count := func(model any, where string, args ...any) int64 {
		t.Helper()
		var n int64
		require.NoError(t, db.Model(model).Where(where, args...).Count(&n).Error)
		return n
	}
	// 插件侧全清：服务、审批、成员令牌、动态客户端注册。
	require.EqualValues(t, 0, count(&types.MCPService{}, "id = ?", "svc-plugin"))
	require.EqualValues(t, 0, count(&types.MCPToolApproval{}, "service_id = ?", "svc-plugin"))
	require.EqualValues(t, 0, count(&types.MCPOAuthToken{}, "service_id = ?", "svc-plugin"),
		"member OAuth tokens keyed to the removed service must be cascade-cleaned")
	require.EqualValues(t, 0, count(&types.MCPOAuthClient{}, "service_id = ?", "svc-plugin"),
		"dynamic-client rows keyed to the removed service must be cascade-cleaned")
	// 手工侧原样保留。
	require.EqualValues(t, 1, count(&types.MCPService{}, "id = ?", "svc-manual"))
	require.EqualValues(t, 1, count(&types.MCPToolApproval{}, "service_id = ?", "svc-manual"))
	require.EqualValues(t, 1, count(&types.MCPOAuthToken{}, "service_id = ?", "svc-manual"))
	require.EqualValues(t, 1, count(&types.MCPOAuthClient{}, "service_id = ?", "svc-manual"))
}

// TestHardDeleteServiceCascadeRollsBackOnMidSweepFailure（T06-OCR2-F1）：
// 级联是 4~5 条独立 DELETE——任一中途失败必须整体回滚，否则敏感凭据清扫
// 半途而废且部分完成状态无法重放（补偿路径随后删安装行会滞留 service 行
// 成孤儿）。用 BEFORE DELETE 触发器让 mcp_oauth_clients（第 3 条 DELETE）
// 报错，断言已执行的 approvals/tokens 删除被回滚、service 行仍在。
func TestHardDeleteServiceCascadeRollsBackOnMidSweepFailure(t *testing.T) {
	ctx := context.Background()
	db := newPluginPreviewTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&types.MCPService{}, &types.MCPToolApproval{},
		&types.MCPOAuthToken{}, &types.MCPOAuthClient{},
	))
	repo := NewPluginRepository(db)

	svc := &types.MCPService{ID: "svc-x", TenantID: 7, Name: "svc", TransportType: "http"}
	require.NoError(t, db.Create(svc).Error)
	require.NoError(t, db.Create(&types.MCPToolApproval{
		ID: "appr-x", TenantID: 7, ServiceID: "svc-x", ToolName: "tool-a",
	}).Error)
	require.NoError(t, db.Create(&types.MCPOAuthToken{
		ID: "tok-x", TenantID: 7, UserID: "user-x", ServiceID: "svc-x",
		PrincipalType: "user", PrincipalID: "user-x", AccessToken: "tok",
	}).Error)
	require.NoError(t, db.Create(&types.MCPOAuthClient{
		ID: "cli-x", TenantID: 7, ServiceID: "svc-x", ClientID: "cid-x",
	}).Error)

	// 第 3 条 DELETE（mcp_oauth_clients）注入确定性失败。
	require.NoError(t, db.Exec(
		"CREATE TRIGGER fail_clients_delete BEFORE DELETE ON mcp_oauth_clients BEGIN SELECT RAISE(ABORT, 'boom: clients sweep failed'); END",
	).Error)

	err := repo.HardDeleteServiceCascade(ctx, 7, "svc-x")
	require.ErrorContains(t, err, "boom: clients sweep failed")

	// 整体回滚：先前执行的 approvals/tokens 删除必须被撤销，service 行
	// 仍在（清扫要么全部完成要么全部不动——重试可完整重放）。
	var n int64
	require.NoError(t, db.Model(&types.MCPToolApproval{}).Where("service_id = ?", "svc-x").Count(&n).Error)
	require.EqualValues(t, 1, n, "approval delete must roll back with the failed sweep")
	require.NoError(t, db.Model(&types.MCPOAuthToken{}).Where("service_id = ?", "svc-x").Count(&n).Error)
	require.EqualValues(t, 1, n, "token delete must roll back with the failed sweep")
	require.NoError(t, db.Model(&types.MCPService{}).Where("id = ?", "svc-x").Count(&n).Error)
	require.EqualValues(t, 1, n, "service row must survive the failed sweep")
}
