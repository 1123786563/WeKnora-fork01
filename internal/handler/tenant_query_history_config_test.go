package handler

// KV tests for the query-history privacy config: GET normalizes (nil config
// reads as mode=normal), PUT validates the mode enum (400 on anything else)
// and persists through UpdateTenant.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func serveTenantKV(
	t *testing.T,
	engine *gin.Engine,
	method, key, body string,
) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, "/tenants/kv/"+key, nil)
	} else {
		req = httptest.NewRequest(method, "/tenants/kv/"+key, strings.NewReader(body))
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

// newTenantKVEngineWithService mounts the KV endpoints on a handler bound to
// the caller's stub service, so PUT tests can assert what reached UpdateTenant.
func newTenantKVEngineWithService(t *testing.T, role types.TenantRole, svc *stubTenantService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := &TenantHandler{service: svc}
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, types.TenantIDContextKey, svc.tenant.ID)
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
		ctx = context.WithValue(ctx, types.TenantInfoContextKey, svc.tenant)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.GET("/tenants/kv/:key", h.GetTenantKV)
	r.PUT("/tenants/kv/:key", h.UpdateTenantKV)
	return r
}

func TestGetTenantQueryHistoryConfigDefaultsToNormal(t *testing.T) {
	// nil QueryHistoryConfig must surface as {mode:"normal"}, not null.
	svc := &stubTenantService{tenant: &types.Tenant{ID: 1}}
	w := serveTenantKV(t, newTenantKVEngineWithService(t, types.TenantRoleViewer, svc), http.MethodGet, "query-history-config", "")
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Mode string `json:"mode"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, types.QueryHistoryModeNormal, body.Data.Mode)
}

func TestGetTenantQueryHistoryConfigReturnsStoredMode(t *testing.T) {
	svc := &stubTenantService{tenant: &types.Tenant{
		ID: 1, QueryHistoryConfig: &types.QueryHistoryConfig{Mode: types.QueryHistoryModeAnonymized},
	}}
	w := serveTenantKV(t, newTenantKVEngineWithService(t, types.TenantRoleViewer, svc), http.MethodGet, "query-history-config", "")
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data struct {
			Mode string `json:"mode"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, types.QueryHistoryModeAnonymized, body.Data.Mode)
}

func TestUpdateTenantQueryHistoryConfigValidModes(t *testing.T) {
	for _, mode := range []string{
		types.QueryHistoryModeNormal,
		types.QueryHistoryModeAnonymized,
		types.QueryHistoryModeDisabled,
	} {
		svc := &stubTenantService{tenant: &types.Tenant{ID: 1}}
		w := serveTenantKV(
			t, newTenantKVEngineWithService(t, types.TenantRoleAdmin, svc),
			http.MethodPut, "query-history-config", `{"mode":"`+mode+`"}`,
		)
		require.Equal(t, http.StatusOK, w.Code, "mode %s must be accepted", mode)

		var body struct {
			Success bool `json:"success"`
			Data    struct {
				Mode string `json:"mode"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.True(t, body.Success)
		require.Equal(t, mode, body.Data.Mode)

		require.NotNil(t, svc.tenant.QueryHistoryConfig)
		require.Equal(t, mode, svc.tenant.QueryHistoryConfig.Mode)
	}
}

func TestUpdateTenantQueryHistoryConfigRejectsInvalidMode(t *testing.T) {
	for _, body := range []string{
		`{"mode":"bogus"}`,
		`{"mode":""}`,
		`{"mode":"NORMAL"}`,
		`{}`,
	} {
		svc := &stubTenantService{tenant: &types.Tenant{ID: 1}}
		w := serveTenantKV(
			t, newTenantKVEngineWithService(t, types.TenantRoleAdmin, svc),
			http.MethodPut, "query-history-config", body,
		)
		require.Equal(t, http.StatusBadRequest, w.Code, "body %s must be rejected", body)

		var resp struct {
			Success bool `json:"success"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.False(t, resp.Success)
		require.Nil(t, svc.tenant.QueryHistoryConfig, "a rejected update must not touch the stored config")
	}
}

func TestTenantQueryHistoryConfigUnknownKeyStillRejected(t *testing.T) {
	svc := &stubTenantService{tenant: &types.Tenant{ID: 1}}
	w := serveTenantKV(t, newTenantKVEngineWithService(t, types.TenantRoleAdmin, svc), http.MethodGet, "query-history-configx", "")
	require.Equal(t, http.StatusBadRequest, w.Code, "unknown keys stay unsupported")
}
