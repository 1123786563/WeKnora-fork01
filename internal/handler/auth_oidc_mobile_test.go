package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
)

const mobileOIDCTestRedirect = "weknora://oidc"

type stubOIDCMobileUserService struct {
	interfaces.UserService
	authorizationURL  func(context.Context, string) (*types.OIDCAuthURLResponse, error)
	loginWithOIDC     func(context.Context, string, string, types.TenantProvisioningMode) (*types.OIDCCallbackResponse, error)
	loginWithOIDCPKCE func(context.Context, string, string, types.TenantProvisioningMode, string) (*types.OIDCCallbackResponse, error)
}

func (s *stubOIDCMobileUserService) GetOIDCAuthorizationURL(ctx context.Context, redirectURI string) (*types.OIDCAuthURLResponse, error) {
	return s.authorizationURL(ctx, redirectURI)
}

func (s *stubOIDCMobileUserService) LoginWithOIDC(ctx context.Context, code, redirectURI string, provisioning types.TenantProvisioningMode) (*types.OIDCCallbackResponse, error) {
	return s.loginWithOIDC(ctx, code, redirectURI, provisioning)
}

func (s *stubOIDCMobileUserService) LoginWithOIDCWithPKCE(ctx context.Context, code, redirectURI string, provisioning types.TenantProvisioningMode, codeVerifier string) (*types.OIDCCallbackResponse, error) {
	return s.loginWithOIDCPKCE(ctx, code, redirectURI, provisioning, codeVerifier)
}

func mobileOIDCTestState(t *testing.T) string {
	t.Helper()
	challenge, err := secutils.OIDCCodeChallenge("mobile-verifier-value-abcdefghijklmnopqrstuvwxyz123456")
	if err != nil {
		t.Fatalf("OIDCCodeChallenge: %v", err)
	}
	state, err := secutils.SignOIDCState(&secutils.OIDCStatePayload{
		Nonce: "mobile-nonce", RedirectURI: "https://api.example.test/api/v1/auth/oidc/callback",
		FrontendRedirectURI: mobileOIDCTestRedirect, CodeChallenge: challenge, IssuedAt: time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("SignOIDCState: %v", err)
	}
	return state
}

func mobileOIDCTestRouter(h *AuthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(errorCapture())
	r.GET("/auth/oidc/url", h.GetOIDCAuthorizationURL)
	r.GET("/auth/oidc/callback", h.OIDCRedirectCallback)
	r.POST("/auth/oidc/exchange", h.OIDCExchange)
	return r
}

func TestOIDCMobileCallbackReturnsOneTimeCodeWithoutBearerTokens(t *testing.T) {
	state := mobileOIDCTestState(t)
	var exchanged bool
	service := &stubOIDCMobileUserService{
		authorizationURL: func(context.Context, string) (*types.OIDCAuthURLResponse, error) { return nil, nil },
		loginWithOIDC: func(context.Context, string, string, types.TenantProvisioningMode) (*types.OIDCCallbackResponse, error) {
			exchanged = true
			return &types.OIDCCallbackResponse{Success: true, Token: "access", RefreshToken: "refresh"}, nil
		},
	}
	r := mobileOIDCTestRouter(NewAuthHandler(&config.Config{}, service, nil, nil, nil))
	req := httptest.NewRequest(http.MethodGet, "/auth/oidc/callback?code=provider-code&state="+url.QueryEscape(state), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", w.Code)
	}
	location, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	if location.Scheme+"://"+location.Host+location.Path != mobileOIDCTestRedirect {
		t.Fatalf("Location = %q, want %q", location.String(), mobileOIDCTestRedirect)
	}
	if got := location.Query().Get("oidc_code"); got != "provider-code" {
		t.Errorf("oidc_code = %q, want provider-code", got)
	}
	if location.Query().Get("state") != state {
		t.Errorf("state was not returned unchanged")
	}
	if strings.Contains(location.String(), "access") || strings.Contains(location.String(), "refresh") || location.Query().Get("oidc_result") != "" {
		t.Errorf("Location leaked a bearer payload: %q", location.String())
	}
	if exchanged {
		t.Fatal("mobile callback must defer token exchange to /auth/oidc/exchange")
	}
}

func TestOIDCMobileCallbackReturnsProviderCancellationToAllowlistedDeepLink(t *testing.T) {
	state := mobileOIDCTestState(t)
	service := &stubOIDCMobileUserService{authorizationURL: func(context.Context, string) (*types.OIDCAuthURLResponse, error) { return nil, nil }}
	r := mobileOIDCTestRouter(NewAuthHandler(&config.Config{}, service, nil, nil, nil))
	req := httptest.NewRequest(http.MethodGet, "/auth/oidc/callback?error=access_denied&error_description=cancelled&state="+url.QueryEscape(state), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", w.Code)
	}
	location, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	if location.Scheme+"://"+location.Host+location.Path != mobileOIDCTestRedirect {
		t.Fatalf("Location = %q, want mobile deep-link", location.String())
	}
	if location.Query().Get("oidc_error") != "access_denied" || location.Query().Get("oidc_error_description") != "cancelled" || location.Query().Get("state") != state {
		t.Errorf("unexpected provider error redirect: %q", location.String())
	}
}

func TestOIDCMobileAuthorizationRejectsIllegalFrontendRedirect(t *testing.T) {
	called := false
	service := &stubOIDCMobileUserService{authorizationURL: func(context.Context, string) (*types.OIDCAuthURLResponse, error) {
		called = true
		return nil, nil
	}}
	r := mobileOIDCTestRouter(NewAuthHandler(&config.Config{}, service, nil, nil, nil))
	req := httptest.NewRequest(http.MethodGet, "/auth/oidc/url?redirect_uri=https%3A%2F%2Fapi.example.test%2Fapi%2Fv1%2Fauth%2Foidc%2Fcallback&frontend_redirect_uri=evil%3A%2F%2Foidc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if called {
		t.Fatal("illegal frontend redirect must be rejected before generating provider URL")
	}
	if w.Code < 400 || w.Code >= 500 {
		t.Fatalf("status = %d, want client error", w.Code)
	}
}

func TestOIDCMobileExchangeUsesSignedStateAndReturnsSessionJSON(t *testing.T) {
	state := mobileOIDCTestState(t)
	var gotCode, gotRedirect, gotVerifier string
	service := &stubOIDCMobileUserService{
		authorizationURL: func(context.Context, string) (*types.OIDCAuthURLResponse, error) { return nil, nil },
		loginWithOIDC: func(_ context.Context, code, redirect string, _ types.TenantProvisioningMode) (*types.OIDCCallbackResponse, error) {
			gotCode, gotRedirect = code, redirect
			return &types.OIDCCallbackResponse{Success: true, Token: "access", RefreshToken: "refresh"}, nil
		},
		loginWithOIDCPKCE: func(_ context.Context, code, redirect string, _ types.TenantProvisioningMode, verifier string) (*types.OIDCCallbackResponse, error) {
			gotCode, gotRedirect, gotVerifier = code, redirect, verifier
			return &types.OIDCCallbackResponse{Success: true, Token: "access", RefreshToken: "refresh"}, nil
		},
	}
	r := mobileOIDCTestRouter(NewAuthHandler(&config.Config{}, service, nil, nil, nil))
	body, _ := json.Marshal(map[string]string{"code": "provider-code", "state": state, "code_verifier": "mobile-verifier-value-abcdefghijklmnopqrstuvwxyz123456"})
	req := httptest.NewRequest(http.MethodPost, "/auth/oidc/exchange", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s, want 200", w.Code, w.Body.String())
	}
	if gotCode != "provider-code" || gotRedirect != "https://api.example.test/api/v1/auth/oidc/callback" || gotVerifier != "mobile-verifier-value-abcdefghijklmnopqrstuvwxyz123456" {
		t.Errorf("LoginWithOIDC args = code %q redirect %q verifier %q", gotCode, gotRedirect, gotVerifier)
	}
	if !strings.Contains(w.Body.String(), `"token":"access"`) || !strings.Contains(w.Body.String(), `"refresh_token":"refresh"`) {
		t.Errorf("exchange response did not contain the session: %s", w.Body.String())
	}
}
