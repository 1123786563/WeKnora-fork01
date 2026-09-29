package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

// GitLab OAuth 注册与交换形状（T24 #54）：gitlab 在默认端点表中、token 交换
// 发 form-encoded（与 github 同形）+ Accept: application/json、authorize URL
// 带 api read_user scope（交付链写任务分支与草稿 MR；read_user 记录实际远端身份）。
func TestGitLabOAuthRegistrationAndExchangeShape(t *testing.T) {
	cfg := DefaultAppOAuthProviderConfigs()
	gitlab, ok := cfg["gitlab"]
	require.True(t, ok, "gitlab must be a first-batch OAuth app")
	require.Equal(t, "https://gitlab.com/oauth/authorize", gitlab.AuthorizeURL)
	require.Equal(t, "https://gitlab.com/oauth/token", gitlab.TokenURL)
	_, known := appOAuthDefaults["gitlab"]
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
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "glpat_x", "token_type": "bearer", "scope": "api read_user"})
	}))
	defer srv.Close()

	token, err := exchangeAppOAuthCode(t.Context(), AppOAuthProviderConfig{
		AuthorizeURL: gitlab.AuthorizeURL, TokenURL: srv.URL,
		ClientID: "cid", ClientSecret: "csecret",
	}, "gitlab", "the-code", "https://deploy.example.com/api/v1/apps/connections/oauth/callback")
	require.NoError(t, err)
	require.Equal(t, "glpat_x", token.AccessToken)
	require.Equal(t, "application/x-www-form-urlencoded", gotContentType)
	require.Equal(t, "application/json", gotAccept)
	require.Equal(t, "cid", gotForm.Get("client_id"))
	require.Equal(t, "csecret", gotForm.Get("client_secret"))
	require.Equal(t, "the-code", gotForm.Get("code"))
	require.Equal(t, "authorization_code", gotForm.Get("grant_type"))
	require.Equal(t, "https://deploy.example.com/api/v1/apps/connections/oauth/callback", gotForm.Get("redirect_uri"))
}

func TestGitLabAuthorizeURLCarriesAPIScope(t *testing.T) {
	_, known := appOAuthDefaults["gitlab"]
	require.True(t, known, "gitlab must be a first-batch OAuth app")
	cfg := AppOAuthProviderConfig{
		AuthorizeURL: "https://gitlab.com/oauth/authorize",
		TokenURL:     "https://gitlab.com/oauth/token",
		ClientID:     "cid", ClientSecret: "csecret",
	}
	got := appAuthorizeURL(cfg, "gitlab", "st-1", "https://deploy.example.com/cb")
	require.Contains(t, got, "https://gitlab.com/oauth/authorize?")
	require.Contains(t, got, "scope=api+read_user")
	require.Contains(t, got, "response_type=code")
	require.Contains(t, got, "state=st-1")
	require.Contains(t, got, "client_id=cid")
}
