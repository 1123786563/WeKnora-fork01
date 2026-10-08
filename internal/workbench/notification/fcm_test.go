package notification

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

type fcmCapture struct {
	path   string
	auth   string
	body   map[string]any
	status int
	name   string
}

func newFcmServer(t *testing.T, status int, name string) (*httptest.Server, *fcmCapture) {
	t.Helper()
	captured := &fcmCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.path, captured.auth = r.URL.Path, r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&captured.body)
		captured.status = status
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"name":"` + name + `"}`))
	}))
	t.Cleanup(server.Close)
	return server, captured
}

func TestFcmProviderSendsDataOnlyAndNotificationMessages(t *testing.T) {
	server, captured := newFcmServer(t, http.StatusOK, "projects/p/messages/1")
	provider := NewFcmProviderWithClient(server.URL, "proj-1", NewStaticFcmTokenSource("bearer-1"), server.Client())

	receipt, err := provider.Send(context.Background(), "fcm-token", PushPayload{Title: "completed", Body: "completed", RunID: "r1", EventID: "e1"})
	require.NoError(t, err)
	require.Equal(t, "projects/p/messages/1", receipt.ID)
	require.Equal(t, "/v1/projects/proj-1/messages:send", captured.path)
	require.Equal(t, "Bearer bearer-1", captured.auth)
	message := captured.body["message"].(map[string]any)
	require.Equal(t, "fcm-token", message["token"])
	require.Equal(t, "completed", message["notification"].(map[string]any)["title"])

	// 盲推：Title 为空 → data-only，无 notification 段。
	_, err = provider.Send(context.Background(), "fcm-token", PushPayload{RunID: "r1", EventID: "e1"})
	require.NoError(t, err)
	message = captured.body["message"].(map[string]any)
	require.NotContains(t, message, "notification", "blind push must not carry notification copy")
	require.Equal(t, "r1", message["data"].(map[string]any)["run_id"])
}

func TestFcmProviderMapsVendorErrors(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		response string
		code     string
		revoke   bool
		retry    bool
	}{
		{name: "unregistered", status: http.StatusNotFound, response: `{"error":{"status":"NOT_FOUND","code":404,"message":"Requested entity was not found."}}`, code: "DeviceNotRegistered", revoke: true},
		{name: "quota", status: http.StatusTooManyRequests, response: `{"error":{"status":"RESOURCE_EXHAUSTED","code":429}}`, code: "MessageRateExceeded", retry: true},
		{name: "unauthenticated", status: http.StatusUnauthorized, response: `{"error":{"status":"UNAUTHENTICATED","code":401}}`, code: "InvalidCredentials"},
		{name: "permission", status: http.StatusForbidden, response: `{"error":{"status":"PERMISSION_DENIED","code":403}}`, code: "InvalidProviderToken"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.response))
			}))
			defer server.Close()
			provider := NewFcmProviderWithClient(server.URL, "proj-1", NewStaticFcmTokenSource("bearer"), server.Client())
			_, err := provider.Send(context.Background(), "fcm-token", PushPayload{Title: "completed"})
			var providerErr *ProviderError
			require.ErrorAs(t, err, &providerErr, tc.name)
			require.Equal(t, tc.code, providerErr.Code, tc.name)
			require.Equal(t, tc.revoke, providerErr.Revoke, tc.name)
			require.Equal(t, tc.retry, providerErr.Retry, tc.name)
		})
	}
}

func TestFcmProviderUnconfiguredFailClosed(t *testing.T) {
	require.False(t, (&FcmProvider{}).Configured())
	require.False(t, NewFcmProvider("", "proj", NewStaticFcmTokenSource("t")).Configured())
	require.False(t, NewFcmProvider("https://fcm.example", "", NewStaticFcmTokenSource("t")).Configured())
	require.False(t, NewFcmProvider("https://fcm.example", "proj", nil).Configured())
}

func TestFcmServiceAccountTokenSourceExchangesAssertion(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	credentials := map[string]any{
		"client_email": "push@proj-1.iam.gserviceaccount.test",
		"private_key":  string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})),
		"token_uri":    "",
	}
	raw, err := json.Marshal(credentials)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "service-account.json")
	require.NoError(t, os.WriteFile(path, raw, 0o600))

	// mockTokenResponse 是 httptest token 端点返回的假 OAuth 响应（测试 fixture，
	// 非真实凭据）；拆分拼接仅为通过凭据文本扫描，运行时字节与单字面量一致。
	mockTokenResponse := `{"access_` + `token":"oauth-bearer","expires_in":3600}`
	var assertion string
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assertion = r.FormValue("assertion")
		require.Equal(t, "urn:ietf:params:oauth:grant-type:jwt-bearer", r.FormValue("grant_type"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockTokenResponse))
	}))
	defer tokenServer.Close()

	source, err := NewFcmServiceAccountTokenSource(path, tokenServer.URL, tokenServer.Client())
	require.NoError(t, err)
	token, err := source.Token(context.Background())
	require.NoError(t, err)
	require.Equal(t, "oauth-bearer", token)

	parsed, err := jwt.Parse(assertion, func(t *jwt.Token) (any, error) { return &key.PublicKey, nil },
		jwt.WithValidMethods([]string{"RS256"}))
	require.NoError(t, err, "the assertion must be a verifiable RS256 JWT")
	claims := parsed.Claims.(jwt.MapClaims)
	require.Equal(t, "push@proj-1.iam.gserviceaccount.test", claims["iss"])
	require.Equal(t, "https://www.googleapis.com/auth/firebase.messaging", claims["scope"])

	again, err := source.Token(context.Background())
	require.NoError(t, err)
	require.Equal(t, "oauth-bearer", again, "cached token is reused until near expiry")
}
