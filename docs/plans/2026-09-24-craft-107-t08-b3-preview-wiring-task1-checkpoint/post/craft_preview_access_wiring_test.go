package container

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

func TestCraftPreviewConstructorUsesCurrentPersistentTaskAccessAndStaysDisabled(t *testing.T) {
	db := wiringTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id, tenant_id, kind) VALUES ('s-wiring', 1, 'web')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id, user_id, role, status, joined_at, created_at, updated_at) VALUES
		(1, 'u-wiring', 'contributor', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
		(1, 'u-viewer', 'viewer', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
		(1, 'u-admin', 'admin', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
		(2, 'u-foreign', 'viewer', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)

	policy := service.NewCraftAccessService(db)
	checker := craftTaskAccessChecker(policy)
	require.Same(t, policy, checker, "the preview receives the persistent policy instance")
	owner := craft.Scope{TenantID: 1, UserID: "u-wiring", SessionID: "s-wiring"}
	viewer := craft.Scope{TenantID: 1, UserID: "u-viewer", SessionID: "s-wiring"}
	admin := craft.Scope{TenantID: 1, UserID: "u-admin", SessionID: "s-wiring"}
	foreign := craft.Scope{TenantID: 2, UserID: "u-foreign", SessionID: "s-wiring"}
	require.NoError(t, policy.Grant(context.Background(), owner, "u-viewer", craft.TaskRoleViewer))
	require.NoError(t, checker.CheckTaskAccess(context.Background(), owner, craft.TaskPreview))
	require.NoError(t, checker.CheckTaskAccess(context.Background(), viewer, craft.TaskPreview))
	require.ErrorIs(t, checker.CheckTaskAccess(context.Background(), admin, craft.TaskPreview), craft.ErrForbidden)
	require.Error(t, checker.CheckTaskAccess(context.Background(), foreign, craft.TaskPreview))

	// The production constructor receives the exact same T08 provider. Its
	// browser boundary remains disabled; test-only preview gates are not wired
	// into production to make this assertion pass.
	t.Setenv("WEKNORA_CRAFT_APP_ORIGIN", "https://app.test")
	t.Setenv("WEKNORA_CRAFT_PREVIEW_ORIGIN", "https://preview.test")
	preview := newCraftPreviewService(
		struct{ craft.VersionStore }{}, struct{ interfaces.FileService }{}, nil, policy,
	)
	require.True(t, preview.Enabled(), "valid isolated origins assemble without enabling browser navigation")
	_, err := preview.Issue(context.Background(), owner, "v-preview-test")
	require.ErrorIs(t, err, craft.ErrUnsupported, "production BrowserNavigationProtected remains false")

	require.NoError(t, policy.Revoke(context.Background(), owner, "u-viewer"))
	require.ErrorIs(t, checker.CheckTaskAccess(context.Background(), viewer, craft.TaskPreview), craft.ErrForbidden)

	// A member with a stale membership incarnation cannot keep the old grant.
	require.NoError(t, policy.Grant(context.Background(), owner, "u-viewer", craft.TaskRoleViewer))
	var oldID uint64
	require.NoError(t, db.Table("tenant_members").Where("tenant_id = ? AND user_id = ? AND status = 'active'", 1, "u-viewer").Pluck("id", &oldID).Error)
	require.NoError(t, db.Exec(`UPDATE tenant_members SET deleted_at = CURRENT_TIMESTAMP, status = 'inactive' WHERE id = ?`, oldID).Error)
	require.ErrorIs(t, checker.CheckTaskAccess(context.Background(), viewer, craft.TaskPreview), craft.ErrForbidden)

}
