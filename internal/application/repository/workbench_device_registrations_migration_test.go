package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWorkbenchDeviceRegistrationsTableExistsAfterMigrations:
// workbench_device_registrations 是 POST /workbench/inbox/devices 与登出撤销
// （RevokeDevicesForOwner）的底表，必须由生产迁移建出——此前仅存在于测试
// AutoMigrate，生产库缺表致注册面 500。列集与 DeviceRegistrationRow
// （internal/handler/session/workbench_inbox.go:36-48）逐列对齐。
func TestWorkbenchDeviceRegistrationsTableExistsAfterMigrations(t *testing.T) {
	db := openRunTestDB(t)
	require.True(t, db.Migrator().HasTable("workbench_device_registrations"),
		"workbench_device_registrations must be created by the production migrations")
	for _, column := range []string{"tenant_id", "device_id", "owner_id", "token", "platform", "revoked", "created_at", "updated_at"} {
		require.True(t, db.Migrator().HasColumn("workbench_device_registrations", column),
			"workbench_device_registrations.%s must exist (aligned with DeviceRegistrationRow)", column)
	}
}
