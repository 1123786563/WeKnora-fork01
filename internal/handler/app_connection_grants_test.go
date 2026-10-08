package handler

// T23 (#53) task 3: the space-grant management endpoint. The management
// predicate is appconnector.CanManageConnections (owner/admin), the target
// must be a SPACE connection, and the grantee must be an active same-tenant
// member. Personal connections are refused outright (AC1).

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newSpaceGrantEnv(t *testing.T) (*gin.Engine, *appconnectorrepo.SpaceConnectionGrantStore) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&appconnectorrepo.InstallationRow{}, &appconnectorrepo.ConnectionRow{},
		&appconnectorrepo.SpaceConnectionGrantRow{}, &types.TenantMember{},
	))
	require.NoError(t, db.Create(&appconnectorrepo.InstallationRow{ID: "inst-1", TenantID: 7, AppID: "github", AppVersion: "1", State: appconnector.InstallationActive, Version: 1}).Error)
	require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{TenantID: 7, ID: "conn-space", InstallationID: "inst-1", Kind: appconnector.ConnectionKindSpace, OwnerID: "u1", CredentialRef: "mcp:conn-space:github", State: appconnector.ConnectionActive, AuthVersion: 1}).Error)
	require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{TenantID: 7, ID: "conn-personal", InstallationID: "inst-1", Kind: appconnector.ConnectionKindPersonal, OwnerID: "u1", CredentialRef: "mcp:conn-personal:github", State: appconnector.ConnectionActive, AuthVersion: 1}).Error)
	for _, m := range []struct {
		user string
		role types.TenantRole
	}{
		{"u1", types.TenantRoleOwner},
		{"u2", types.TenantRoleContributor},
	} {
		require.NoError(t, db.Create(&types.TenantMember{UserID: m.user, TenantID: 7, Role: m.role, Status: types.TenantMemberStatusActive, JoinedAt: time.Now().UTC()}).Error)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAppConnectionGrantHandler(db)
	r.POST("/api/v1/apps/connections/:id/grants", h.Grant)
	r.GET("/api/v1/apps/connections/:id/grants", h.List)
	r.DELETE("/api/v1/apps/connections/:id/grants/:grantee_id", h.Revoke)
	return r, appconnectorrepo.NewSpaceConnectionGrantStore(db)
}

// spaceGrantDo injects the authenticated identity the same way the auth
// middleware's context keys do.
func spaceGrantDo(t *testing.T, r *gin.Engine, method, path, body, userID string, role types.TenantRole) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, uint64(7))
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAppConnectionGrantLifecycleOwnerAdmitsMemberRefuses(t *testing.T) {
	r, grants := newSpaceGrantEnv(t)

	// Tenant owner grants an active member on the space connection.
	w := spaceGrantDo(t, r, http.MethodPost, "/api/v1/apps/connections/conn-space/grants", `{"grantee_id":"u2"}`, "u1", types.TenantRoleOwner)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	granted, err := grants.SpaceConnectionGranted(context.Background(), 7, "conn-space", "u2")
	require.NoError(t, err)
	require.True(t, granted)

	// A regular member cannot manage grants (CanManageConnections).
	w = spaceGrantDo(t, r, http.MethodDelete, "/api/v1/apps/connections/conn-space/grants/u2", "", "u2", types.TenantRoleContributor)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "SPACE_GRANT_FORBIDDEN")

	// Owner lists, then revokes; revoke is idempotent.
	w = spaceGrantDo(t, r, http.MethodGet, "/api/v1/apps/connections/conn-space/grants", "", "u1", types.TenantRoleOwner)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"grantee_id":"u2"`)
	w = spaceGrantDo(t, r, http.MethodDelete, "/api/v1/apps/connections/conn-space/grants/u2", "", "u1", types.TenantRoleOwner)
	require.Equal(t, http.StatusOK, w.Code)
	w = spaceGrantDo(t, r, http.MethodDelete, "/api/v1/apps/connections/conn-space/grants/u2", "", "u1", types.TenantRoleOwner)
	require.Equal(t, http.StatusOK, w.Code, "重复撤销幂等")

	granted, err = grants.SpaceConnectionGranted(context.Background(), 7, "conn-space", "u2")
	require.NoError(t, err)
	require.False(t, granted)
}

func TestAppConnectionGrantRefusesPersonalConnectionAndForeignTargets(t *testing.T) {
	r, _ := newSpaceGrantEnv(t)

	// AC1: a personal connection never takes a space grant.
	w := spaceGrantDo(t, r, http.MethodPost, "/api/v1/apps/connections/conn-personal/grants", `{"grantee_id":"u2"}`, "u1", types.TenantRoleOwner)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "SPACE_GRANT_NOT_APPLICABLE")

	// Another tenant's connection id is indistinguishable from a missing one.
	w = spaceGrantDo(t, r, http.MethodPost, "/api/v1/apps/connections/conn-elsewhere/grants", `{"grantee_id":"u2"}`, "u1", types.TenantRoleOwner)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "CONNECTION_NOT_FOUND")

	// A grantee who is not an active member of the tenant is refused.
	w = spaceGrantDo(t, r, http.MethodPost, "/api/v1/apps/connections/conn-space/grants", `{"grantee_id":"ghost"}`, "u1", types.TenantRoleOwner)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "GRANTEE_NOT_ACTIVE_MEMBER")
}
