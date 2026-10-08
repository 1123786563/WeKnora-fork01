package handler

// Feishu publish endpoint gates (#49). Mirrors the notion publish gates:
// role gate, personal-connection owner predicate, fail-closed
// unconfigured service, malformed input, tenant-scoped lookups.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appconnectorrepo "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newFeishuPublishTestEngine(t *testing.T) (*gin.Engine, *AppFeishuPublishHandler, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	require.NoError(t, err)
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	require.NoError(t, db.AutoMigrate(&appconnectorrepo.ConnectionRow{}, &appconnectorrepo.InstallationRow{}, &appconnectorrepo.AppVersion{}, &appconnectorrepo.ActionRow{}, &appconnectorrepo.ApprovalRow{}, &appconnectorrepo.PreAuthorizationRow{}, &appconnectorrepo.PublicationRow{}))
	// conn-feishu is user-a's PERSONAL feishu connection with the
	// reviewed write_docx scope; conn-feishu-ro carries only read scopes.
	require.NoError(t, db.Create(&appconnectorrepo.InstallationRow{ID: "inst-feishu", TenantID: 7, AppID: "feishu", AppVersion: "v1", State: "active"}).Error)
	require.NoError(t, db.Exec(`INSERT INTO app_versions (app_id, version, schema_json, risk_json) VALUES ('feishu', 'v1', '{"scopes":["write_docx"],"approved_parents":["fld-1"]}', '{}')`).Error)
	require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{TenantID: 7, ID: "conn-feishu", InstallationID: "inst-feishu", Kind: "personal", OwnerID: "user-a", CredentialRef: "mcp_oauth_token:feishu", State: "active", AuthVersion: 1}).Error)

	h := NewAppFeishuPublishHandler(db)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		tenant := uint64(7)
		role := "admin"
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
	g := engine.Group("/api/v1/apps/feishu-publish", h.RequireActionCapabilityForWrites())
	g.POST("/plans", h.FormFeishuPublishPlan)
	g.POST("/actions/:id/publish", h.PublishFeishuAction)
	g.POST("/actions/:id/reconcile", h.ReconcileFeishuAction)
	g.GET("/actions/:id", h.GetFeishuPublication)
	return engine, h, db
}

func feishuPublishDo(t *testing.T, engine *gin.Engine, method, path, body string, headers ...string) *httptest.ResponseRecorder {
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

func TestFeishuPublishPlanGates(t *testing.T) {
	engine, _, _ := newFeishuPublishTestEngine(t)
	body := `{"connection_id":"conn-feishu","session_id":"s1","artifact_version_id":"v1","title":"T","parent_page_id":"fld-1"}`

	// Service not wired: fail closed 501.
	w := feishuPublishDo(t, engine, http.MethodPost, "/api/v1/apps/feishu-publish/plans", body)
	require.Equal(t, http.StatusNotImplemented, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "PUBLISH_PIPELINE_NOT_CONFIGURED")

	// Viewer role: the write gate refuses.
	w = feishuPublishDo(t, engine, http.MethodPost, "/api/v1/apps/feishu-publish/plans", body, "X-Test-Role", "viewer")
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	// Another member may not use user-a's PERSONAL connection.
	w = feishuPublishDo(t, engine, http.MethodPost, "/api/v1/apps/feishu-publish/plans", body, "X-Test-User", "user-b", "X-Test-Role", "admin")
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "NOT_CONNECTION_OWNER")

	// Both destination shapes at once: malformed.
	w = feishuPublishDo(t, engine, http.MethodPost, "/api/v1/apps/feishu-publish/plans",
		`{"connection_id":"conn-feishu","session_id":"s1","artifact_version_id":"v1","title":"T","parent_page_id":"f","page_id":"d"}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "INVALID_REQUEST")

	// Unknown connection: 404 without leaking existence details.
	w = feishuPublishDo(t, engine, http.MethodPost, "/api/v1/apps/feishu-publish/plans",
		`{"connection_id":"conn-nope","session_id":"s1","artifact_version_id":"v1","title":"T","parent_page_id":"fld-1"}`)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

func TestFeishuPublishActionLookupIsTenantScoped(t *testing.T) {
	engine, h, db := newFeishuPublishTestEngine(t)
	h.SetFeishuPublishService(nil)
	require.NoError(t, db.Create(&appconnectorrepo.ActionRow{ID: "act-fx", TenantID: 7, ConnectionID: "conn-feishu",
		AppVersion: "feishu/v1", Target: "fld-1", Risk: "write", ArgsSnapshot: "{}", ArgsDigest: "d",
		State: "authorized"}).Error)
	// B5-F42: the family ledger — plan formation always writes this row, so
	// a resolvable action carries one.
	require.NoError(t, db.Create(&appconnectorrepo.PublicationRow{TenantID: 7, ActionID: "act-fx", ConnectionID: "conn-feishu",
		Provider: "feishu", Mode: "create", Destination: "fld-1", State: "planned"}).Error)
	w := feishuPublishDo(t, engine, http.MethodPost, "/api/v1/apps/feishu-publish/actions/act-fx/publish", "")
	require.Equal(t, http.StatusNotImplemented, w.Code, w.Body.String())
	// Foreign tenant id is indistinguishable from missing: 404.
	w = feishuPublishDo(t, engine, http.MethodPost, "/api/v1/apps/feishu-publish/actions/act-fx/publish", "", "X-Test-Tenant", "8")
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = feishuPublishDo(t, engine, http.MethodGet, "/api/v1/apps/feishu-publish/actions/act-fx", "", "X-Test-Tenant", "8")
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}
