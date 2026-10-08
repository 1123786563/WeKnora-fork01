package notification

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

type apnsCapture struct {
	method   string
	path     string
	pushType string
	auth     string
	topic    string
	body     map[string]any
	aps      map[string]any
	status   int
	uniqueID string
}

func newApnsServer(t *testing.T, status int, uniqueID string) (*httptest.Server, *apnsCapture) {
	t.Helper()
	captured := &apnsCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method, captured.path = r.Method, r.URL.Path
		captured.pushType = r.Header.Get("apns-push-type")
		captured.auth = r.Header.Get("Authorization")
		captured.topic = r.Header.Get("apns-topic")
		_ = json.NewDecoder(r.Body).Decode(&captured.body)
		if aps, ok := captured.body["aps"].(map[string]any); ok {
			captured.aps = aps
		}
		if uniqueID != "" {
			w.Header().Set("apns-unique-id", uniqueID)
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return server, captured
}

func TestApnsProviderSendsAlertAndBackgroundPayloads(t *testing.T) {
	server, captured := newApnsServer(t, http.StatusOK, "receipt-1")
	provider := NewApnsProviderWithClient(server.URL, "bundle.acme", NewStaticApnsTokenSource("jwt-1"), server.Client())

	receipt, err := provider.Send(context.Background(), "aabb", PushPayload{Title: "completed", Body: "completed", RunID: "r1", EventID: "e1"})
	require.NoError(t, err)
	require.Equal(t, "receipt-1", receipt.ID)
	require.Equal(t, http.MethodPost, captured.method)
	require.Equal(t, "/3/device/aabb", captured.path)
	require.Equal(t, "alert", captured.pushType)
	require.Equal(t, "Bearer jwt-1", captured.auth)
	require.Equal(t, "bundle.acme", captured.topic)
	require.Equal(t, "completed", captured.aps["alert"].(map[string]any)["title"])
	require.Equal(t, "r1", captured.body["run_id"])

	// 盲推：Title 为空 → background content-available，无任何文案。
	_, err = provider.Send(context.Background(), "aabb", PushPayload{RunID: "r1", EventID: "e1"})
	require.NoError(t, err)
	require.Equal(t, "background", captured.pushType)
	require.Equal(t, float64(1), captured.aps["content-available"])
	require.NotContains(t, captured.aps, "alert", "blind push must not carry alert copy")
}

func TestApnsProviderMapsReceiptAndPermanentFailures(t *testing.T) {
	// 200 但无 apns-unique-id → MissingReceipt（可重试，不撤销）。
	server, _ := newApnsServer(t, http.StatusOK, "")
	provider := NewApnsProviderWithClient(server.URL, "bundle.acme", NewStaticApnsTokenSource("jwt"), server.Client())
	_, err := provider.Send(context.Background(), "aabb", PushPayload{Title: "completed"})
	var providerErr *ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "MissingReceipt", providerErr.Code)
	require.True(t, providerErr.Retry)
	require.False(t, providerErr.Revoke)

	// 410 → DeviceNotRegistered（撤销，不重试）。
	server410, _ := newApnsServer(t, http.StatusGone, "")
	provider = NewApnsProviderWithClient(server410.URL, "bundle.acme", NewStaticApnsTokenSource("jwt"), server410.Client())
	_, err = provider.Send(context.Background(), "aabb", PushPayload{Title: "completed"})
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "DeviceNotRegistered", providerErr.Code)
	require.True(t, providerErr.Revoke)
	require.False(t, providerErr.Retry)

	// 403 → InvalidProviderToken（配置类，不重试不撤销）。
	server403, _ := newApnsServer(t, http.StatusForbidden, "")
	provider = NewApnsProviderWithClient(server403.URL, "bundle.acme", NewStaticApnsTokenSource("jwt"), server403.Client())
	_, err = provider.Send(context.Background(), "aabb", PushPayload{Title: "completed"})
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "InvalidProviderToken", providerErr.Code)
}

func TestApnsProviderUnconfiguredFailClosed(t *testing.T) {
	require.False(t, (&ApnsProvider{}).Configured())
	require.False(t, NewApnsProvider("", "topic", NewStaticApnsTokenSource("jwt")).Configured())
	require.False(t, NewApnsProvider("https://apns.example/3/device", "", NewStaticApnsTokenSource("jwt")).Configured())
	require.False(t, NewApnsProvider("https://apns.example/3/device", "topic", nil).Configured())
	_, err := NewApnsProvider("", "topic", NewStaticApnsTokenSource("jwt")).Send(context.Background(), "aabb", PushPayload{})
	var providerErr *ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "InvalidProviderConfig", providerErr.Code)
}

func TestApnsP8TokenSourceSignsES256(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "AuthKey.p8")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600))

	source, err := NewApnsP8TokenSource(path, "KID123", "TEAM123")
	require.NoError(t, err)
	token, err := source.Token(context.Background())
	require.NoError(t, err)
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) { return &key.PublicKey, nil },
		jwt.WithValidMethods([]string{"ES256"}), jwt.WithTimeFunc(func() time.Time { return time.Now() }))
	require.NoError(t, err, "provider token must be a verifiable ES256 JWT")
	require.Equal(t, "KID123", parsed.Header["kid"])
	claims := parsed.Claims.(jwt.MapClaims)
	require.Equal(t, "TEAM123", claims["iss"])
	// 缓存：同源第二次调用返回同一 token（APNs provider token 生命期内不重签）。
	again, err := source.Token(context.Background())
	require.NoError(t, err)
	require.Equal(t, token, again)
}
