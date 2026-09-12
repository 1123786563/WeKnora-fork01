package handler

// A02 app-connector OAuth: the controlled credential exchange for the
// first-batch providers (feishu, notion). The flow has two halves:
//
//  1. CreateConnection (authenticated) verifies the installation, mints a
//     one-time binding state from crypto/rand and returns the provider
//     authorize URL. It NEVER mints a connection and never sees credentials.
//  2. ConnectionOAuthCallback (public; the opaque single-use state is the
//     only credential on the redirect) consumes the state, exchanges the
//     authorization code at the provider token endpoint over TLS, and
//     hands the token to MCPOAuthBindingStore.CompleteBinding, which
//     persists the encrypted token and the personal connection in ONE
//     transaction. Credential material is never logged, never echoed.

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"
	mcprepo "github.com/Tencent/WeKnora/internal/application/repository"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// AppOAuthProviderConfig is one provider's OAuth application registration.
// ClientID/ClientSecret arrive from deployment env; a missing secret keeps
// the provider unconfigured and CreateConnection fails closed for it.
type AppOAuthProviderConfig struct {
	AuthorizeURL string
	TokenURL     string
	ClientID     string
	ClientSecret string
}

// appOAuthDefaults carries the fixed provider endpoints of the first-batch
// apps. Only the client registration is deployment-specific.
var appOAuthDefaults = map[string]AppOAuthProviderConfig{
	"feishu": {
		AuthorizeURL: "https://accounts.feishu.cn/open_site/authen/v1/authorize",
		TokenURL:     "https://open.feishu.cn/open_site/authen/v2/oauth/token",
	},
	"notion": {
		AuthorizeURL: "https://api.notion.com/v1/oauth/authorize",
		TokenURL:     "https://api.notion.com/v1/oauth/token",
	},
}

// DefaultAppOAuthProviderConfigs returns the fixed provider endpoints of
// the first-batch apps with empty client registrations. Deployment env fills
// ClientID/ClientSecret; the map is a fresh copy each call.
func DefaultAppOAuthProviderConfigs() map[string]AppOAuthProviderConfig {
	out := make(map[string]AppOAuthProviderConfig, len(appOAuthDefaults))
	for k, v := range appOAuthDefaults {
		out[k] = v
	}
	return out
}

// SetAppOAuthProviders installs the deployment's client registrations keyed
// by app id (feishu, notion). Unknown apps are never configurable here: an
// unregistered app id fails closed in CreateConnection.
func (h *AppConnectionHandler) SetAppOAuthProviders(m map[string]AppOAuthProviderConfig) {
	h.oauthMu.Lock()
	defer h.oauthMu.Unlock()
	h.oauthProviders = m
}

func (h *AppConnectionHandler) appOAuthProvider(appID string) (AppOAuthProviderConfig, bool) {
	h.oauthMu.Lock()
	defer h.oauthMu.Unlock()
	cfg, ok := h.oauthProviders[appID]
	return cfg, ok && cfg.ClientID != "" && cfg.ClientSecret != ""
}

// mintOAuthState returns 256 bits from crypto/rand, URL-safe. The state is
// the entire bearer credential of the redirect leg, so it is never logged
// and never accepted from the client.
func mintOAuthState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// exchangeAppOAuthCode swaps the authorization code for a token at the
// provider token endpoint. Providers differ only in request shape:
// notion uses HTTP Basic client auth over a form body, feishu puts the
// client registration inside a JSON body.
func exchangeAppOAuthCode(ctx context.Context, cfg AppOAuthProviderConfig, appID, code, redirectURI string) (*types.MCPOAuthToken, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var req *http.Request
	var err error
	switch appID {
	case "notion":
		form := url.Values{}
		form.Set("grant_type", "authorization_code")
		form.Set("code", code)
		form.Set("redirect_uri", redirectURI)
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, err
		}
		req.SetBasicAuth(cfg.ClientID, cfg.ClientSecret)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	default: // feishu shape
		body, _ := json.Marshal(map[string]string{
			"grant_type": "authorization_code", "client_id": cfg.ClientID,
			"client_secret": cfg.ClientSecret, "code": code, "redirect_uri": redirectURI,
		})
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, strings.NewReader(string(body)))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint status %d", resp.StatusCode)
	}
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		Code         int64  `json:"code"` // feishu error envelope
		Msg          string `json:"msg"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out.AccessToken == "" || (appID == "feishu" && out.Code != 0) {
		return nil, fmt.Errorf("token exchange refused: %s", strings.TrimSpace(out.Msg))
	}
	return &types.MCPOAuthToken{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		TokenType:    out.TokenType,
		ExpiresAt:    time.Now().Add(time.Duration(out.ExpiresIn) * time.Second),
	}, nil
}

// appOAuthConfigError distinguishes "provider known but not registered in
// this deployment" (501) from "unknown app" (400) without leaking config.
type appOAuthConfigError struct {
	appID string
	known bool
}

func (e *appOAuthConfigError) Error() string {
	if e.known {
		return "oauth not configured for app " + e.appID
	}
	return "unknown app " + e.appID
}

// createConnectionOAuth runs inside CreateConnection after the installation
// checks: it mints the one-time state, persists the binding and builds the
// provider authorize URL.
func (h *AppConnectionHandler) createConnectionOAuth(c *gin.Context, tenantID uint64, actorID string, inst appconnectorrepo.InstallationRow, redirectURI string) (string, string, time.Time, error) {
	appID := inst.AppID
	cfg, configured := h.appOAuthProvider(appID)
	if !configured {
		_, known := appOAuthDefaults[appID]
		return "", "", time.Time{}, &appOAuthConfigError{appID: appID, known: known}
	}
	state, err := mintOAuthState()
	if err != nil {
		return "", "", time.Time{}, err
	}
	expires := time.Now().Add(15 * time.Minute)
	binding := appconnector.OAuthBinding{
		State: state, InstallationID: inst.ID, ActorID: actorID,
		TenantID: tenantID, ExpiresAt: expires,
	}
	if err := h.bindings.IssueBindingState(c.Request.Context(), binding, appID); err != nil {
		return "", "", time.Time{}, err
	}
	q := url.Values{}
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	if appID == "notion" {
		q.Set("response_type", "code")
		q.Set("owner", "user")
	}
	authorizeURL := cfg.AuthorizeURL + "?" + q.Encode()
	return state, authorizeURL, expires, nil
}

// ConnectionOAuthCallback is the PUBLIC redirect leg. The one-time state is
// the only credential: it proves tenant, installation and initiating actor,
// and it is consumed exactly once inside CompleteBinding's transaction.
func (h *AppConnectionHandler) ConnectionOAuthCallback(c *gin.Context) {
	state := strings.TrimSpace(c.Query("state"))
	code := strings.TrimSpace(c.Query("code"))
	redirectURI := strings.TrimSpace(c.Query("redirect_uri"))
	if state == "" || code == "" {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "state and code are required")
		return
	}
	row, err := h.bindings.GetBindingState(c.Request.Context(), state)
	if err != nil {
		appFail(c, http.StatusBadRequest, "OAUTH_STATE_INVALID", "unknown or expired authorization state")
		return
	}
	cfg, configured := h.appOAuthProvider(row.ServiceID)
	if !configured {
		appFail(c, http.StatusServiceUnavailable, "OAUTH_NOT_CONFIGURED",
			"the provider application is not registered in this deployment")
		return
	}
	token, err := exchangeAppOAuthCode(c.Request.Context(), cfg, row.ServiceID, code, redirectURI)
	if err != nil {
		appFail(c, http.StatusBadGateway, "TOKEN_EXCHANGE_FAILED", "provider refused the authorization code")
		return
	}
	conn, err := h.bindings.CompleteBinding(c.Request.Context(), row.TenantID, state, row.ActorID, token)
	if err != nil {
		switch {
		case errors.Is(err, mcprepo.ErrOAuthBindingInvalid):
			appFail(c, http.StatusBadRequest, "OAUTH_STATE_INVALID", "authorization state is invalid or already used")
		case errors.Is(err, mcprepo.ErrOAuthBindingActorNotMember):
			appFail(c, http.StatusForbidden, "ACTOR_NOT_MEMBER", "the initiating user is no longer an active member")
		default:
			appFail(c, http.StatusInternalServerError, "BINDING_FAILED", "failed to bind the connection")
		}
		return
	}
	appOK(c, http.StatusCreated, appConnectionViewFor(appconnectorrepo.ConnectionRow{
		TenantID: conn.TenantID, ID: conn.ID, InstallationID: conn.InstallationID,
		Kind: conn.Kind, OwnerID: conn.OwnerID, State: conn.State, AuthVersion: conn.AuthVersion,
	}))
}
