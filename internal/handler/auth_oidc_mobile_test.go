package handler

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const mobileOIDCTestRedirect = "weknora://oidc"

type stubOIDCMobileUserService struct {
	interfaces.UserService
	authorizationURL  func(context.Context, string) (*types.OIDCAuthURLResponse, error)
	loginWithOIDC     func(context.Context, string, string, types.TenantProvisioningMode) (*types.OIDCCallbackResponse, error)
	loginWithOIDCPKCE func(context.Context, string, string, types.TenantProvisioningMode, string) (*types.OIDCCallbackResponse, error)
}

func (s *stubOIDCMobileUserService) GetUserByID(context.Context, string) (*types.User, error) {
	return &types.User{ID: "u-mobile", TenantID: 1}, nil
}
func (s *stubOIDCMobileUserService) GenerateTokens(context.Context, *types.User) (string, string, error) {
	return "access", "refresh", nil
}
func (s *stubOIDCMobileUserService) BuildLoginMemberships(context.Context, *types.User, *types.Tenant) []types.Membership {
	return []types.Membership{}
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
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&repository.MobileExchange{}))
	store := repository.NewMobileExchangeStore(db)
	service := &stubOIDCMobileUserService{
		authorizationURL: func(context.Context, string) (*types.OIDCAuthURLResponse, error) { return nil, nil },
		loginWithOIDC: func(context.Context, string, string, types.TenantProvisioningMode) (*types.OIDCCallbackResponse, error) {
			return &types.OIDCCallbackResponse{Success: true, User: &types.User{ID: "u-mobile"}}, nil
		},
	}
	h := NewAuthHandler(&config.Config{}, service, nil, nil, nil)
	h.SetMobileExchangeStore(store)
	r := mobileOIDCTestRouter(h)
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
	if got := location.Query().Get("code"); got == "" || got == "provider-code" {
		t.Errorf("code = %q, want a server-issued one-time code", got)
	}
	if location.Query().Get("state") != state {
		t.Errorf("state was not returned unchanged")
	}
	if strings.Contains(location.String(), "access") || strings.Contains(location.String(), "refresh") || location.Query().Get("oidc_result") != "" {
		t.Errorf("Location leaked a bearer payload: %q", location.String())
	}
	if strings.Contains(location.String(), "provider-code") {
		t.Fatal("provider authorization code must not cross the native redirect")
	}
}

func TestOIDCMobileStartCallbackExchangeIsOneTime(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&repository.MobileExchange{}))
	verifier := "mobile-verifier-value-abcdefghijklmnopqrstuvwxyz123456"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	service := &stubOIDCMobileUserService{
		authorizationURL: func(context.Context, string) (*types.OIDCAuthURLResponse, error) {
			return &types.OIDCAuthURLResponse{Success: true, AuthorizationURL: "https://idp.example/authorize", State: "provider-state", Nonce: "nonce"}, nil
		},
		loginWithOIDC: func(_ context.Context, code, redirect string, _ types.TenantProvisioningMode) (*types.OIDCCallbackResponse, error) {
			if code != "provider-code" || redirect != "https://api.example.test/api/v1/auth/oidc/callback" {
				t.Fatalf("provider callback args = %q %q", code, redirect)
			}
			return &types.OIDCCallbackResponse{Success: true, User: &types.User{ID: "u-mobile"}}, nil
		},
	}
	h := NewAuthHandler(&config.Config{}, service, nil, nil, nil)
	h.SetMobileExchangeStore(repository.NewMobileExchangeStore(db))
	r := mobileOIDCTestRouter(h)
	start := httptest.NewRequest(http.MethodGet, "/auth/oidc/url?redirect_uri=https%3A%2F%2Fapi.example.test%2Fapi%2Fv1%2Fauth%2Foidc%2Fcallback&frontend_redirect_uri=weknora%3A%2F%2Foidc&code_challenge="+url.QueryEscape(challenge), nil)
	startW := httptest.NewRecorder()
	r.ServeHTTP(startW, start)
	require.Equal(t, http.StatusOK, startW.Code)
	var authURL types.OIDCAuthURLResponse
	require.NoError(t, json.Unmarshal(startW.Body.Bytes(), &authURL))
	authState := authURL.State
	provider := httptest.NewRequest(http.MethodGet, "/auth/oidc/callback?code=provider-code&state="+url.QueryEscape(authState), nil)
	providerW := httptest.NewRecorder()
	r.ServeHTTP(providerW, provider)
	require.Equal(t, http.StatusFound, providerW.Code)
	location, err := url.Parse(providerW.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "weknora://oidc", location.Scheme+"://"+location.Host+location.Path)
	code := location.Query().Get("code")
	require.NotEmpty(t, code)
	body, _ := json.Marshal(map[string]string{"code": code, "state": authState, "redirect_uri": "weknora://oidc", "code_verifier": verifier})
	exchange := httptest.NewRequest(http.MethodPost, "/auth/mobile/exchange", strings.NewReader(string(body)))
	exchange.Header.Set("Content-Type", "application/json")
	exchangeW := httptest.NewRecorder()
	r.ServeHTTP(exchangeW, exchange)
	require.Equal(t, http.StatusOK, exchangeW.Code)
	require.Contains(t, exchangeW.Body.String(), `"token":"access"`)
	replay := httptest.NewRequest(http.MethodPost, "/auth/mobile/exchange", strings.NewReader(string(body)))
	replay.Header.Set("Content-Type", "application/json")
	replayW := httptest.NewRecorder()
	r.ServeHTTP(replayW, replay)
	require.Equal(t, http.StatusUnauthorized, replayW.Code)
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
