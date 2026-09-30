package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWorkbenchNotificationsTableExistsAfterMigrations: workbench_notifications
// 是 InboxService（MX-021）与 overview unread 计数的读模型底表，必须由生产迁移
// 建出（此前仅存在于测试 AutoMigrate，生产库缺表——见计划差异记录 11）。
// 列集与 InboxNotificationRow（internal/handler/session/workbench_inbox.go:21-33）
// 逐列对齐。
func TestWorkbenchNotificationsTableExistsAfterMigrations(t *testing.T) {
	db := openRunTestDB(t)
	require.True(t, db.Migrator().HasTable("workbench_notifications"),
		"workbench_notifications must be created by the production migrations")
	for _, column := range []string{"tenant_id", "id", "owner_id", "kind", "title", "body", "deep_link", "read", "created_at"} {
		require.True(t, db.Migrator().HasColumn("workbench_notifications", column),
			"workbench_notifications.%s must exist (aligned with InboxNotificationRow)", column)
	}
}
