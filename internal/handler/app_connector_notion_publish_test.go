package handler

// Notion publish endpoint gates (T18 #48). The publish pipeline itself is
// service-level (publish package) and end-to-end (Task 8); these pin the
// HTTP predicates: role gate, personal-connection owner predicate,
// fail-closed unconfigured service, malformed input, tenant-scoped
// lookups.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appconnectorrepo "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// publishTestContext injects the authenticated identity exactly like the
// OC engine middleware (app_connector_oc_test.go:275-298).
func publishTestContext(ctx context.Context, tenant uint64, role, user string) context.Context {
	ctx = context.WithValue(ctx, types.TenantIDContextKey, tenant)
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRole(role))
	ctx = context.WithValue(ctx, types.UserIDContextKey, user)
	return ctx
}

func newNotionPublishTestEngine(t *testing.T) (*gin.Engine, *AppNotionPublishHandler, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	require.NoError(t, err)
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	require.NoError(t, db.AutoMigrate(&appconnectorrepo.ConnectionRow{}, &appconnectorrepo.InstallationRow{}, &appconnectorrepo.AppVersion{}, &appconnectorrepo.ActionRow{}, &appconnectorrepo.ApprovalRow{}, &appconnectorrepo.PreAuthorizationRow{}, &appconnectorrepo.PublicationRow{}))
	// conn-notion is user-a's PERSONAL notion connection.
	require.NoError(t, db.Create(&appconnectorrepo.InstallationRow{ID: "inst-notion", TenantID: 7, AppID: "notion", AppVersion: "v1", State: "active"}).Error)
	require.NoError(t, db.Exec(`INSERT INTO app_versions (app_id, version, schema_json, risk_json) VALUES ('notion', 'v1', '{"scopes":["insert_content"],"approved_parents":["parent-1"]}', '{}')`).Error)
	require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{TenantID: 7, ID: "conn-notion", InstallationID: "inst-notion", Kind: "personal", OwnerID: "user-a", CredentialRef: "mcp_oauth_token:notion", State: "active", AuthVersion: 1}).Error)

	h := NewAppNotionPublishHandler(db)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		tenant := uint64(7)
		role := "admin" // CanDriveActionWrites admits owner/admin only (access.go:43-45)
		user := "user-a"
		if c.GetHeader("X-Test-User") != "" {
			user = c.GetHeader("X-Test-User")
		}
		if c.GetHeader("X-Test-Role") != "" {
			role = c.GetHeader("X-Test-Role")
		}
		if c.GetHeader("X-Test-Tenant") == "8" {
			tenant = 8
		}
		c.Request = c.Request.WithContext(publishTestContext(c.Request.Context(), tenant, role, user))
		c.Next()
	})
	g := engine.Group("/api/v1/apps/notion-publish", h.RequireActionCapabilityForWrites())
	g.POST("/plans", h.FormNotionPublishPlan)
	g.POST("/actions/:id/publish", h.PublishNotionAction)
	g.POST("/actions/:id/reconcile", h.ReconcileNotionAction)
	g.GET("/actions/:id", h.GetNotionPublication)
	return engine, h, db
}

func notionPublishDo(t *testing.T, engine *gin.Engine, method, path, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func TestNotionPublishPlanGates(t *testing.T) {
	engine, h, _ := newNotionPublishTestEngine(t)
	body := `{"connection_id":"conn-notion","session_id":"s1","artifact_version_id":"v1","title":"T","parent_page_id":"parent-1"}`

	// Service not wired: fail closed 501 (mirroring the frozen
	// ACTION_PIPELINE_NOT_CONFIGURED), nothing formed.
	w := notionPublishDo(t, engine, http.MethodPost, "/api/v1/apps/notion-publish/plans", body)
	require.Equal(t, http.StatusNotImplemented, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "PUBLISH_PIPELINE_NOT_CONFIGURED")
	require.NotNil(t, h)

	// Viewer role: the write gate refuses.
	w = notionPublishDo(t, engine, http.MethodPost, "/api/v1/apps/notion-publish/plans", body, "X-Test-Role", "viewer")
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	// Another member may not use user-a's PERSONAL connection (same
	// predicate as PrepareAction).
	w = notionPublishDo(t, engine, http.MethodPost, "/api/v1/apps/notion-publish/plans", body, "X-Test-User", "user-b", "X-Test-Role", "admin")
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "NOT_CONNECTION_OWNER")

	// Malformed input: both destination shapes at once.
	w = notionPublishDo(t, engine, http.MethodPost, "/api/v1/apps/notion-publish/plans",
		`{"connection_id":"conn-notion","session_id":"s1","artifact_version_id":"v1","title":"T","parent_page_id":"p","page_id":"q"}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "INVALID_REQUEST")

	// Unknown connection: 404 without leaking existence details.
	w = notionPublishDo(t, engine, http.MethodPost, "/api/v1/apps/notion-publish/plans",
		`{"connection_id":"conn-nope","session_id":"s1","artifact_version_id":"v1","title":"T","parent_page_id":"parent-1"}`)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

func TestNotionPublishActionLookupIsTenantScoped(t *testing.T) {
	engine, h, db := newNotionPublishTestEngine(t)
	h.SetNotionPublishService(nil) // stays unconfigured; lookup gates first
	require.NoError(t, db.Create(&appconnectorrepo.ActionRow{ID: "act-x", TenantID: 7, ConnectionID: "conn-notion",
		AppVersion: "notion/v1", Target: "parent-1", Risk: "write", ArgsSnapshot: "{}", ArgsDigest: "d",
		State: "authorized"}).Error)
	// B5-F42: the family ledger — plan formation always writes this row, so
	// a resolvable action carries one.
	require.NoError(t, db.Create(&appconnectorrepo.PublicationRow{TenantID: 7, ActionID: "act-x", ConnectionID: "conn-notion",
		Provider: "notion", Mode: "create", Destination: "parent-1", State: "planned"}).Error)
	// Same tenant: reaches the unconfigured refusal (501).
	w := notionPublishDo(t, engine, http.MethodPost, "/api/v1/apps/notion-publish/actions/act-x/publish", "")
	require.Equal(t, http.StatusNotImplemented, w.Code, w.Body.String())
	// Foreign tenant id is indistinguishable from missing: 404.
	w = notionPublishDo(t, engine, http.MethodPost, "/api/v1/apps/notion-publish/actions/act-x/publish", "", "X-Test-Tenant", "8")
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = notionPublishDo(t, engine, http.MethodGet, "/api/v1/apps/notion-publish/actions/act-x", "", "X-Test-Tenant", "8")
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}
