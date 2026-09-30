package session

import (
	"context"
	"net/http"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCraftPreviewCapabilityStopsServingAfterPersistentGrantRevocation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	env := newPreviewEnv(t)
	env.scope = craft.Scope{TenantID: 42, UserID: "user-2", SessionID: "sess-1"}
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	for _, ddl := range []string{
		`CREATE TABLE sessions (id text primary key, tenant_id integer not null, user_id text not null, deleted_at datetime)`,
		`CREATE TABLE craft_sessions (session_id text primary key, tenant_id integer not null, kind text not null)`,
		`CREATE TABLE tenant_members (id integer primary key, tenant_id integer not null, user_id text not null, status text not null, deleted_at datetime)`,
		`CREATE TABLE craft_task_grants (tenant_id integer not null, session_id text not null, user_id text not null, membership_id integer not null, role text not null, granted_by text not null, created_at datetime, updated_at datetime, primary key (tenant_id, session_id, user_id))`,
		`CREATE TABLE audit_logs (id integer primary key autoincrement, tenant_id integer, actor_user_id text, action text, scope_type text, scope_id text, target_type text, target_id text, target_user_id text, outcome text, details text, created_at datetime)`,
		`INSERT INTO sessions (id, tenant_id, user_id) VALUES ('sess-1', 42, 'user-1')`,
		`INSERT INTO craft_sessions (session_id, tenant_id, kind) VALUES ('sess-1', 42, 'web')`,
		`INSERT INTO tenant_members (id, tenant_id, user_id, status) VALUES (1, 42, 'user-1', 'active'), (2, 42, 'user-2', 'active')`,
		`INSERT INTO craft_task_grants (tenant_id, session_id, user_id, membership_id, role, granted_by) VALUES (42, 'sess-1', 'user-2', 2, 'viewer', 'user-1')`,
	} {
		require.NoError(t, db.Exec(ddl).Error)
	}
	access := service.NewCraftAccessService(db)
	viewer := craft.Scope{TenantID: 42, UserID: "user-2", SessionID: "sess-1"}
	owner := craft.Scope{TenantID: 42, UserID: "user-1", SessionID: "sess-1"}

	preview := service.NewCraftPreviewService(env.store, env.files, env.checks, service.CraftPreviewConfig{
		AppOrigin: "https://app.test", PreviewOrigin: "https://preview.test",
		AccessChecker: access, NetworkChecker: previewNoEgress{}, BrowserNavigationProtected: true,
	})
	h := NewCraftPreviewHandler(preview)
	env.preview = gin.New()
	RegisterCraftPreviewRoutes(env.preview, h)
	version := env.publishPreviewVersion(t, "ws-persistent-acl", "run-1", [][2]string{{"index.html", "private preview bytes"}})
	_, roleErr := access.Role(context.Background(), viewer)
	require.NoError(t, roleErr)
	ticket, err := preview.Issue(context.Background(), viewer, version.ID)
	require.NoError(t, err)
	require.NoError(t, access.CheckTaskAccess(context.Background(), viewer, craft.TaskPreview))
	capPath := env.redeem(t, ticket.URL)
	served := env.previewGet(capPath)
	require.Equal(t, http.StatusOK, served.Code)
	require.Equal(t, "private preview bytes", served.Body.String())

	require.NoError(t, access.Revoke(context.Background(), owner, viewer.UserID))
	denied := env.previewGet(capPath)
	require.Equal(t, http.StatusNotFound, denied.Code)
	require.Empty(t, denied.Body.Bytes(), "revoked capability returns no preview bytes")
}
