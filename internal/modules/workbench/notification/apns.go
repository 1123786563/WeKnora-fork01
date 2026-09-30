package notification

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ApnsTokenSource supplies the short-lived provider JWT APNs requires. The
// production source signs ES256 JWTs from a deployment-owned .p8 key; tests
// and externally-minted deployments inject a static source.
type ApnsTokenSource interface {
	Token(ctx context.Context) (string, error)
}

type staticApnsTokenSource struct{ token string }

func NewStaticApnsTokenSource(token string) ApnsTokenSource {
	return &staticApnsTokenSource{token: strings.TrimSpace(token)}
}

func (s *staticApnsTokenSource) Token(context.Context) (string, error) {
	if s.token == "" {
		return "", errors.New("apns provider token is not configured")
	}
	return s.token, nil
}

const apnsTokenTTL = 50 * time.Minute

type ApnsP8TokenSource struct {
	key    *ecdsa.PrivateKey
	keyID  string
	teamID string

	mu        sync.Mutex
	cached    string
	cachedExp time.Time
}

// NewApnsP8TokenSource parses a PKCS#8 .p8 provider key once. Credentials are
// read from the deployment's key file only; they never enter logs or env echo.
func NewApnsP8TokenSource(keyPath, keyID, teamID string) (*ApnsP8TokenSource, error) {
	raw, err := os.ReadFile(strings.TrimSpace(keyPath))
	if err != nil {
		return nil, fmt.Errorf("apns p8 key: %w", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("apns p8 key is not PEM encoded")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("apns p8 key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("apns p8 key must hold an EC private key")
	}
	return &ApnsP8TokenSource{key: key, keyID: strings.TrimSpace(keyID), teamID: strings.TrimSpace(teamID)}, nil
}

func (s *ApnsP8TokenSource) Token(ctx context.Context) (string, error) {
	if s == nil || s.key == nil {
		return "", errors.New("apns p8 token source is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached != "" && time.Now().Before(s.cachedExp) {
		return s.cached, nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": s.teamID, "iat": now.Unix(), "exp": now.Add(apnsTokenTTL).Unix(),
	})
	token.Header["kid"] = s.keyID
	signed, err := token.SignedString(s.key)
	if err != nil {
		return "", fmt.Errorf("apns provider token signing: %w", err)
	}
	s.cached, s.cachedExp = signed, now.Add(apnsTokenTTL-time.Minute)
	return signed, nil
}

// ApnsProvider speaks the APNs HTTP API: POST {endpoint}/3/device/{token}.
// A payload without Title is delivered as a background content-available
// push (blind mode): no lock-screen copy, the client re-syncs on wake.
type ApnsProvider struct {
	endpoint string
	topic    string
	tokens   ApnsTokenSource
	client   *http.Client
}

func NewApnsProvider(endpoint, topic string, tokens ApnsTokenSource) *ApnsProvider {
	return NewApnsProviderWithClient(endpoint, topic, tokens, nil)
}

func NewApnsProviderWithClient(endpoint, topic string, tokens ApnsTokenSource, client *http.Client) *ApnsProvider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &ApnsProvider{endpoint: strings.TrimSpace(endpoint), topic: strings.TrimSpace(topic), tokens: tokens, client: client}
}

func (p *ApnsProvider) Configured() bool {
	if p == nil || !validPushEndpoint(p.endpoint) || p.topic == "" || p.tokens == nil {
		return false
	}
	return true
}

// scheme-only by design: host validation (loopback/private/reserved rejection) is the config-assembly layer's responsibility (Tasks 3/4, controller ruling) — keeps provider unit-testable with httptest.
func validPushEndpoint(endpoint string) bool {
	u, err := url.Parse(strings.TrimSpace(endpoint))
	return err == nil && u.Scheme != "" && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https")
}

func (p *ApnsProvider) Send(ctx context.Context, token string, payload PushPayload) (PushReceipt, error) {
	if p == nil || !p.Configured() {
		return PushReceipt{}, &ProviderError{Code: "InvalidProviderConfig", Retry: false, Err: errors.New("apns provider is not configured")}
	}
	if strings.TrimSpace(token) == "" {
		return PushReceipt{}, &ProviderError{Code: "InvalidRegistration", Revoke: true, Retry: false, Err: errors.New("empty push token")}
	}
	bearer, err := p.tokens.Token(ctx)
	if err != nil {
		return PushReceipt{}, &ProviderError{Code: "InvalidProviderConfig", Retry: false, Err: err}
	}
	body := map[string]any{"run_id": payload.RunID, "event_id": payload.EventID}
	pushType := "background"
	if payload.Title != "" {
		pushType = "alert"
		body["aps"] = map[string]any{"alert": map[string]any{"title": payload.Title, "body": payload.Body}}
	} else {
		body["aps"] = map[string]any{"content-available": 1}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return PushReceipt{}, err
	}
	endpoint := strings.TrimRight(p.endpoint, "/") + "/3/device/" + url.PathEscape(strings.TrimSpace(token))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return PushReceipt{}, &ProviderError{Code: "InvalidProviderConfig", Retry: false, Err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("apns-topic", p.topic)
	req.Header.Set("apns-push-type", pushType)
	resp, err := p.client.Do(req)
	if err != nil {
		return PushReceipt{}, &ProviderError{Code: "UnknownTransport", Retry: true, Err: err}
	}
	defer resp.Body.Close()
	var reason struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&reason)
	if resp.StatusCode == http.StatusTooManyRequests {
		return PushReceipt{}, &ProviderError{Code: "MessageRateExceeded", Retry: true, StatusCode: resp.StatusCode, RetryAfter: ParseRetryAfter(resp.Header.Get("Retry-After"))}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		code := apnsFailureCode(resp.StatusCode, reason.Reason)
		revoke, retry := ClassifyPushFailure(code)
		return PushReceipt{}, &ProviderError{Code: code, Revoke: revoke, Retry: retry, StatusCode: resp.StatusCode, Err: fmt.Errorf("apns status %d reason %s", resp.StatusCode, strings.TrimSpace(reason.Reason))}
	}
	receiptID := resp.Header.Get("apns-unique-id")
	if receiptID == "" {
		return PushReceipt{}, &ProviderError{Code: "MissingReceipt", Retry: true, StatusCode: resp.StatusCode, Err: ErrMissingReceiptID}
	}
	return PushReceipt{ID: receiptID, Status: "ok"}, nil
}

func apnsFailureCode(status int, reason string) string {
	switch strings.TrimSpace(reason) {
	case "Unregistered":
		return "DeviceNotRegistered"
	case "BadDeviceToken", "DeviceTokenNotForTopic":
		return "BadDeviceToken"
	case "InvalidProviderToken", "ExpiredProviderToken":
		return "InvalidProviderToken"
	case "MissingTopic", "TopicDisallowed":
		return "InvalidProviderConfig"
	}
	if status == http.StatusGone {
		return "DeviceNotRegistered"
	}
	if status == 400 || status == 413 {
		return "MessageTooBig"
	}
	if status == http.StatusForbidden {
		return "InvalidProviderToken"
	}
	if status >= 500 {
		return "UnknownTransport"
	}
	return "InvalidProviderToken"
}
