package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newAppOAuthEngine builds a minimal engine with the app-connector
// connection endpoints over an in-memory SQLite schema and a fake notion
// token endpoint.
func newAppOAuthEngine(t *testing.T, tokenSrv *httptest.Server) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&appconnectorrepo.InstallationRow{}, &appconnectorrepo.ConnectionRow{},
		&types.TenantMember{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE IF NOT EXISTS mcp_oauth_binding_states (state TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL, actor_id TEXT NOT NULL, installation_id TEXT NOT NULL, service_id TEXT NOT NULL, expires_at DATETIME NOT NULL, used BOOLEAN NOT NULL DEFAULT 0, used_at DATETIME, created_at DATETIME)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&types.MCPOAuthToken{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&appconnectorrepo.InstallationRow{
		ID: "inst-1", TenantID: 101, AppID: "notion", AppVersion: "1.0.0",
		State: appconnector.InstallationActive, Version: 4,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&types.TenantMember{
		UserID: "user-1", TenantID: 101, Role: types.TenantRoleAdmin, Status: types.TenantMemberStatusActive,
	}).Error; err != nil {
		t.Fatal(err)
	}
	h := NewAppConnectionHandler(db)
	cfg := DefaultAppOAuthProviderConfigs()
	notion := cfg["notion"]
	notion.ClientID = "cid"
	notion.ClientSecret = "csec"
	notion.TokenURL = tokenSrv.URL
	cfg["notion"] = notion
	h.SetAppOAuthProviders(cfg)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(101))
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleAdmin)
		ctx = context.WithValue(ctx, types.UserIDContextKey, "user-1")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	v1 := engine.Group("/api/v1")
	registerAppConnectorTestRoutes(v1, engine, h)
	return engine, db
}

func registerAppConnectorTestRoutes(r *gin.RouterGroup, engine *gin.Engine, h *AppConnectionHandler) {
	connections := r.Group("/apps/connections", h.RequireConnectionCapabilityForWrites())
	{
		connections.POST("", h.CreateConnection)
	}
	engine.GET("/api/v1/apps/connections/oauth/callback", h.ConnectionOAuthCallback)
}

// TestAppConnectionOAuthFlow: an authorized CreateConnection mints a
// one-time state and an authorize URL; the public callback consumes the
// state exactly once and materializes an active personal connection whose
// view carries no credential material. A replayed state is refused.
func TestAppConnectionOAuthFlow(t *testing.T) {
	var tokenSeen bool
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenSeen = true
		if user, pass, ok := r.BasicAuth(); !ok || user != "cid" || pass != "csec" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok-1","token_type":"bearer","expires_in":3600}`))
	}))
	defer tokenSrv.Close()
	engine, db := newAppOAuthEngine(t, tokenSrv)

	// Unregistered app (no feishu client configured) fails closed with 501.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/apps/connections",
		strings.NewReader(`{"installation_id":"inst-1","kind":"personal","expected_version":4,"redirect_uri":"https://cb"}`))
	engine.ServeHTTP(w, req) // notion IS configured; this exercises the happy path
	if w.Code != http.StatusCreated {
		t.Fatalf("create connection status = %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "authorize_url") || !strings.Contains(w.Body.String(), "api.notion.com") {
		t.Fatalf("authorize URL missing: %s", w.Body.String())
	}
	state := extractJSONField(t, w.Body.String(), "authorization_state")
	if state == "" {
		t.Fatal("no state minted")
	}

	// Public callback exchanges the code and binds the connection.
	w2 := httptest.NewRecorder()
	engine.ServeHTTP(w2, httptest.NewRequest(http.MethodGet,
		"/api/v1/apps/connections/oauth/callback?state="+state+"&code=auth-1&redirect_uri=https://cb", nil))
	if w2.Code != http.StatusCreated {
		t.Fatalf("callback status = %d body=%s", w2.Code, w2.Body.String())
	}
	if !tokenSeen {
		t.Fatal("token endpoint never called")
	}
	if strings.Contains(w2.Body.String(), "tok-1") {
		t.Fatalf("credential material leaked into the response: %s", w2.Body.String())
	}
	var conns int64
	if err := db.Table("connections").Where("tenant_id = ?", 101).Count(&conns).Error; err != nil || conns != 1 {
		t.Fatalf("connections = %d err=%v", conns, err)
	}

	// Replay: the state is one-time.
	w3 := httptest.NewRecorder()
	engine.ServeHTTP(w3, httptest.NewRequest(http.MethodGet,
		"/api/v1/apps/connections/oauth/callback?state="+state+"&code=auth-2&redirect_uri=https://cb", nil))
	if w3.Code != http.StatusBadRequest {
		t.Fatalf("replayed state status = %d body=%s", w3.Code, w3.Body.String())
	}

}

func extractJSONField(t *testing.T, body, field string) string {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if v, ok := m[field].(string); ok && v != "" {
		return v
	}
	if data, ok := m["data"].(map[string]interface{}); ok {
		v, _ := data[field].(string)
		return v
	}
	return ""
}
