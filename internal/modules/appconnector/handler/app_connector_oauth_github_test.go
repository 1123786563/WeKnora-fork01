package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

// GitHub OAuth 注册与交换形状：github 在默认端点表中、token 交换发
// form-encoded + Accept: application/json、authorize URL 带 repo scope。
func TestGitHubOAuthRegistrationAndExchangeShape(t *testing.T) {
	cfg := DefaultAppOAuthProviderConfigs()
	github, ok := cfg["github"]
	require.True(t, ok, "github must be a first-batch OAuth app")
	require.Equal(t, "https://github.com/login/oauth/authorize", github.AuthorizeURL)
	require.Equal(t, "https://github.com/login/oauth/access_token", github.TokenURL)
	_, known := appOAuthDefaults["github"]
	require.True(t, known)

	// 交换形状：对 httptest token 端点做真实 HTTP。
	var gotContentType, gotAccept string
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotAccept = r.Header.Get("Accept")
		require.NoError(t, r.ParseForm())
		gotForm = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "gho_x", "token_type": "bearer", "scope": "repo read:user"})
	}))
	defer srv.Close()

	token, err := exchangeAppOAuthCode(t.Context(), AppOAuthProviderConfig{
		AuthorizeURL: github.AuthorizeURL, TokenURL: srv.URL,
		ClientID: "cid", ClientSecret: "csecret",
	}, "github", "the-code", "https://deploy.example.com/api/v1/apps/connections/oauth/callback")
	require.NoError(t, err)
	require.Equal(t, "gho_x", token.AccessToken)
	require.Equal(t, "application/x-www-form-urlencoded", gotContentType)
	require.Equal(t, "application/json", gotAccept)
	require.Equal(t, "cid", gotForm.Get("client_id"))
	require.Equal(t, "csecret", gotForm.Get("client_secret"))
	require.Equal(t, "the-code", gotForm.Get("code"))
}

func TestGitHubAuthorizeURLCarriesRepoScope(t *testing.T) {
	_, known := appOAuthDefaults["github"]
	require.True(t, known, "github must be a first-batch OAuth app")
	cfg := AppOAuthProviderConfig{
		AuthorizeURL: "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		ClientID:     "cid", ClientSecret: "csecret",
	}
	url := appAuthorizeURL(cfg, "github", "st-1", "https://deploy.example.com/cb")
	require.Contains(t, url, "https://github.com/login/oauth/authorize?")
	require.Contains(t, url, "scope=repo+read%3Auser")
	require.Contains(t, url, "state=st-1")
	require.Contains(t, url, "client_id=cid")
}
