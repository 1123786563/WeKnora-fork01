package router

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// wechatLoginStubService implements only the seam the /auth/wechat/login
// handler consumes; embedding the interface keeps the 30-method UserService
// satisfiable without re-stubbing the whole surface.
type wechatLoginStubService struct {
	interfaces.UserService
	calls int
}

func (s *wechatLoginStubService) LoginWithWeChatCode(
	ctx context.Context, code string, provisioning types.TenantProvisioningMode,
) (*types.LoginResponse, error) {
	s.calls++
	if code == "fake" {
		return nil, fmt.Errorf("code2session rejected code %q", code)
	}
	return &types.LoginResponse{Success: true, Token: "tok", RefreshToken: "ref"}, nil
}

// newWechatLoginRouter mirrors the production wiring for the auth surface:
// the global error handler plus Auth middleware in front of the v1 group, so
// a passing POST proves the route is registered AND whitelisted (otherwise
// Auth would 401) AND reaches the handler gate/service.
func newWechatLoginRouter(cfg *config.Config, svc interfaces.UserService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	v1 := r.Group("/api/v1")
	v1.Use(middleware.Auth(nil, nil, nil, nil, cfg))
	RegisterAuthRoutes(v1, handler.NewAuthHandler(cfg, svc, nil, nil, nil), &rbacGuards{cfg: &config.Config{}})
	return r
}

func postJSON(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// The public auth limiter is a package singleton keyed by IP with a
	// 30/60s budget; park these requests on their own IP so the whole
	// router test binary cannot drain the shared test-source bucket.
	req.RemoteAddr = "203.0.113.77:54321"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestRegisterAuthRoutesWechatLoginEndpoint: the mini-program silent-login
// channel is wired end to end — route, whitelist, config gate, and the
// /auth/login-isomorphic success envelope. This is the in-process equivalent
// of the Task-6 curl smoke (a 404 would mean the route is missing, a 401 that
// the whitelist row is missing).
func TestRegisterAuthRoutesWechatLoginEndpoint(t *testing.T) {
	configured := &config.Config{
		Auth:     &config.AuthConfig{RegistrationMode: config.AuthRegistrationModeSelfServe},
		WechatMP: &config.WechatMPConfig{AppID: "wx123", AppSecret: "s3cret", SecretKey: "k"},
		CasdoorAdmin: &config.CasdoorAdminConfig{
			BaseURL: "http://casdoor:8000", OrgName: "weknora",
			AdminUsername: "svc", AdminPassword: "pw",
		},
	}
	svc := &wechatLoginStubService{}
	r := newWechatLoginRouter(configured, svc)

	t.Run("valid code reaches the service and returns the login envelope", func(t *testing.T) {
		w := postJSON(r, "/api/v1/auth/wechat/login", `{"code":"ok-code"}`)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Equal(t, 1, svc.calls)
		require.Contains(t, w.Body.String(), `"token":"tok"`)
		require.Contains(t, w.Body.String(), `"success":true`)
	})

	t.Run("service failure maps to 401", func(t *testing.T) {
		w := postJSON(r, "/api/v1/auth/wechat/login", `{"code":"fake"}`)
		require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
		require.Equal(t, 2, svc.calls)
		require.Contains(t, w.Body.String(), "code2session rejected code")
	})

	t.Run("missing code is a 400 validation error", func(t *testing.T) {
		w := postJSON(r, "/api/v1/auth/wechat/login", `{}`)
		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		require.Equal(t, 2, svc.calls)
	})
}

// TestRegisterAuthRoutesWechatLoginUnconfiguredChannel: with the channel not
// configured (materialized-but-empty sections, as LoadConfig leaves them),
// the handler answers 503 before touching the service — the gate the Task-6
// review asked for.
func TestRegisterAuthRoutesWechatLoginUnconfiguredChannel(t *testing.T) {
	unconfigured := &config.Config{
		WechatMP:     &config.WechatMPConfig{},
		CasdoorAdmin: &config.CasdoorAdminConfig{},
	}
	svc := &wechatLoginStubService{}
	r := newWechatLoginRouter(unconfigured, svc)

	w := postJSON(r, "/api/v1/auth/wechat/login", `{"code":"ok-code"}`)
	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "not configured")
	require.Zero(t, svc.calls, "the gate must fire before the service call")
}

// TestRegisterAuthRoutesWechatLoginWhitelistControl: the same engine must
// still 401 a non-whitelisted auth path, proving the wechat request above
// passes because of the noAuthAPI entry, not because Auth is inert.
func TestRegisterAuthRoutesWechatLoginWhitelistControl(t *testing.T) {
	configured := &config.Config{
		WechatMP:     &config.WechatMPConfig{AppID: "wx123", AppSecret: "s3cret", SecretKey: "k"},
		CasdoorAdmin: &config.CasdoorAdminConfig{BaseURL: "http://casdoor:8000"},
	}
	r := newWechatLoginRouter(configured, &wechatLoginStubService{})

	w := postJSON(r, "/api/v1/auth/change-password", `{"old_password":"x","new_password":"y"}`)
	require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
}
