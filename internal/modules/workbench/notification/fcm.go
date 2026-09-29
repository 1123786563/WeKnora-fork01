package notification

import (
	"bytes"
	"context"
	"crypto/rsa"
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

const fcmScope = "https://www.googleapis.com/auth/firebase.messaging"

// FcmTokenSource supplies OAuth2 access tokens for the FCM HTTP v1 API. The
// production source exchanges a service-account RS256 assertion; tests inject
// a static source.
type FcmTokenSource interface {
	Token(ctx context.Context) (string, error)
}

type staticFcmTokenSource struct{ token string }

func NewStaticFcmTokenSource(token string) FcmTokenSource {
	return &staticFcmTokenSource{token: strings.TrimSpace(token)}
}

func (s *staticFcmTokenSource) Token(context.Context) (string, error) {
	if s.token == "" {
		return "", errors.New("fcm access token is not configured")
	}
	return s.token, nil
}

type fcmCredentials struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

type FcmServiceAccountTokenSource struct {
	credentials fcmCredentials
	key         *rsa.PrivateKey
	tokenURL    string
	client      *http.Client

	mu        sync.Mutex
	cached    string
	cachedExp time.Time
}

// NewFcmServiceAccountTokenSource parses a service-account JSON key once and
// exchanges signed assertions for access tokens at tokenURL (the production
// default is https://oauth2.googleapis.com/token, set by the container).
func NewFcmServiceAccountTokenSource(credentialsPath, tokenURL string, client *http.Client) (*FcmServiceAccountTokenSource, error) {
	raw, err := os.ReadFile(strings.TrimSpace(credentialsPath))
	if err != nil {
		return nil, fmt.Errorf("fcm service account: %w", err)
	}
	var credentials fcmCredentials
	if err := json.Unmarshal(raw, &credentials); err != nil {
		return nil, fmt.Errorf("fcm service account: %w", err)
	}
	block, _ := pem.Decode([]byte(credentials.PrivateKey))
	if block == nil {
		return nil, errors.New("fcm service account private key is not PEM encoded")
	}
	parsed, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("fcm service account private key: %w", err)
	}
	// scheme-only by design: host validation (loopback/private/reserved rejection) is the config-assembly layer's responsibility (Tasks 3/4, controller ruling) — keeps provider unit-testable with httptest.
	tokenURL = strings.TrimSpace(tokenURL)
	if tokenURL == "" {
		tokenURL = credentials.TokenURI
	}
	if tokenURL == "" {
		tokenURL = "https://oauth2.googleapis.com/token"
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &FcmServiceAccountTokenSource{credentials: credentials, key: parsed, tokenURL: tokenURL, client: client}, nil
}

// TokenURL exposes the resolved exchange URL (explicit override, credential
// file token_uri, or the Google OAuth2 default) so the config-assembly layer
// can run the same push-endpoint host gate on it as on the FCM endpoint
// (story 67 final-fix round: the credential-file token_uri fallback is the
// production path — the container passes an empty override — so an admin
// credentials file pointing the signed assertion at a loopback/private/
// reserved host must fail closed at assembly, not at first send).
func (s *FcmServiceAccountTokenSource) TokenURL() string {
	return s.tokenURL
}

func (s *FcmServiceAccountTokenSource) Token(ctx context.Context) (string, error) {
	if s == nil || s.key == nil {
		return "", errors.New("fcm service account source is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached != "" && time.Now().Before(s.cachedExp) {
		return s.cached, nil
	}
	now := time.Now()
	assertion := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":   s.credentials.ClientEmail,
		"scope": fcmScope,
		"aud":   s.tokenURL,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	})
	signed, err := assertion.SignedString(s.key)
	if err != nil {
		return "", fmt.Errorf("fcm assertion signing: %w", err)
	}
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {signed}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var token struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil || resp.StatusCode != http.StatusOK || token.AccessToken == "" {
		return "", fmt.Errorf("fcm token exchange status %d: %w", resp.StatusCode, err)
	}
	ttl := time.Duration(token.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	s.cached, s.cachedExp = token.AccessToken, now.Add(ttl-time.Minute)
	return token.AccessToken, nil
}

// FcmProvider speaks the FCM HTTP v1 API: POST {endpoint}/v1/projects/{project}/messages:send.
// A payload without Title is a data-only message (blind mode): no notification
// copy, the client re-syncs on wake.
type FcmProvider struct {
	endpoint string
	project  string
	tokens   FcmTokenSource
	client   *http.Client
}

func NewFcmProvider(endpoint, project string, tokens FcmTokenSource) *FcmProvider {
	return NewFcmProviderWithClient(endpoint, project, tokens, nil)
}

func NewFcmProviderWithClient(endpoint, project string, tokens FcmTokenSource, client *http.Client) *FcmProvider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &FcmProvider{endpoint: strings.TrimSpace(endpoint), project: strings.TrimSpace(project), tokens: tokens, client: client}
}

func (p *FcmProvider) Configured() bool {
	return p != nil && validPushEndpoint(p.endpoint) && p.project != "" && p.tokens != nil
}

func (p *FcmProvider) Send(ctx context.Context, token string, payload PushPayload) (PushReceipt, error) {
	if p == nil || !p.Configured() {
		return PushReceipt{}, &ProviderError{Code: "InvalidProviderConfig", Retry: false, Err: errors.New("fcm provider is not configured")}
	}
	if strings.TrimSpace(token) == "" {
		return PushReceipt{}, &ProviderError{Code: "InvalidRegistration", Revoke: true, Retry: false, Err: errors.New("empty push token")}
	}
	bearer, err := p.tokens.Token(ctx)
	if err != nil {
		return PushReceipt{}, &ProviderError{Code: "InvalidProviderConfig", Retry: false, Err: err}
	}
	message := map[string]any{"token": strings.TrimSpace(token), "data": map[string]string{"run_id": payload.RunID, "event_id": payload.EventID}}
	if payload.Title != "" {
		message["notification"] = map[string]any{"title": payload.Title, "body": payload.Body}
	}
	raw, err := json.Marshal(map[string]any{"message": message})
	if err != nil {
		return PushReceipt{}, err
	}
	endpoint := strings.TrimRight(p.endpoint, "/") + "/v1/projects/" + url.PathEscape(p.project) + "/messages:send"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return PushReceipt{}, &ProviderError{Code: "InvalidProviderConfig", Retry: false, Err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)
	resp, err := p.client.Do(req)
	if err != nil {
		return PushReceipt{}, &ProviderError{Code: "UnknownTransport", Retry: true, Err: err}
	}
	defer resp.Body.Close()
	var body struct {
		Name  string `json:"name"`
		Error struct {
			Status string `json:"status"`
		} `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if resp.StatusCode == http.StatusTooManyRequests {
		return PushReceipt{}, &ProviderError{Code: "MessageRateExceeded", Retry: true, StatusCode: resp.StatusCode, RetryAfter: ParseRetryAfter(resp.Header.Get("Retry-After"))}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		code := fcmFailureCode(resp.StatusCode, body.Error.Status)
		revoke, retry := ClassifyPushFailure(code)
		return PushReceipt{}, &ProviderError{Code: code, Revoke: revoke, Retry: retry, StatusCode: resp.StatusCode, Err: fmt.Errorf("fcm status %d %s", resp.StatusCode, strings.TrimSpace(body.Error.Status))}
	}
	if body.Name == "" {
		return PushReceipt{}, &ProviderError{Code: "MissingReceipt", Retry: true, StatusCode: resp.StatusCode, Err: ErrMissingReceiptID}
	}
	return PushReceipt{ID: body.Name, Status: "ok"}, nil
}

func fcmFailureCode(status int, vendorStatus string) string {
	switch strings.TrimSpace(vendorStatus) {
	case "NOT_FOUND", "UNREGISTERED":
		return "DeviceNotRegistered"
	case "RESOURCE_EXHAUSTED":
		return "MessageRateExceeded"
	case "UNAUTHENTICATED":
		return "InvalidCredentials"
	case "PERMISSION_DENIED":
		return "InvalidProviderToken"
	case "INVALID_ARGUMENT":
		return "MessageTooBig"
	}
	if status == http.StatusUnauthorized {
		return "InvalidCredentials"
	}
	if status == http.StatusForbidden {
		return "InvalidProviderToken"
	}
	return "UnknownTransport"
}
