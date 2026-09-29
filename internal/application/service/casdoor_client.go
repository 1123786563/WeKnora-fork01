package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/config"
)

type casdoorClient interface {
	PasswordToken(ctx context.Context, username, password string) (string, error)
	EnsureUser(ctx context.Context, username, email, password string) error
	SetPassword(ctx context.Context, username, password string) error
}

// casdoorAdminSettings is the runtime subset of config.CasdoorAdminConfig,
// separated so tests can build it without a full *config.Config.
type casdoorAdminSettings struct {
	BaseURL       string
	OrgName       string
	AdminUsername string
	AdminPassword string
}

type casdoorHTTPClient struct {
	settings         casdoorAdminSettings
	clientID         string
	clientSecret     string
	httpClient       *http.Client
	mu               sync.Mutex
	cachedAdminToken string
	adminTokenExp    time.Time
}

func newCasdoorClient(admin *config.CasdoorAdminConfig, clientID, clientSecret string) casdoorClient {
	return &casdoorHTTPClient{
		settings: casdoorAdminSettings{
			BaseURL:       strings.TrimRight(admin.BaseURL, "/"),
			OrgName:       admin.OrgName,
			AdminUsername: admin.AdminUsername,
			AdminPassword: admin.AdminPassword,
		},
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   newOIDCHTTPClient(),
	}
}

// PasswordToken exchanges username+password for a Casdoor access token via the
// OAuth2 Resource Owner Password Credentials grant (must be enabled on the app).
func (c *casdoorHTTPClient) PasswordToken(ctx context.Context, username, password string) (string, error) {
	form := url.Values{
		"grant_type":    {"password"},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"username":      {fmt.Sprintf("%s/%s", c.settings.OrgName, username)},
		"password":      {password},
	}
	body, err := c.postForm(ctx, c.settings.BaseURL+"/api/login/oauth/access_token", form, "")
	if err != nil {
		return "", fmt.Errorf("casdoor ropc failed: %w", err)
	}
	var resp struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("casdoor ropc: decode response: %w", err)
	}
	if resp.AccessToken == "" {
		detail := resp.ErrorDesc
		if detail == "" {
			detail = resp.Error
		}
		return "", fmt.Errorf("casdoor ropc: no access token (%s)", detail)
	}
	return resp.AccessToken, nil
}

// EnsureUser creates the Casdoor user, or — if it already exists — resets its
// password to the given one. Idempotent across half-finished first logins.
func (c *casdoorHTTPClient) EnsureUser(ctx context.Context, username, email, password string) error {
	form := url.Values{
		"owner":       {c.settings.OrgName},
		"name":        {username},
		"displayName": {username},
		"email":       {email},
		"password":    {password},
		"type":        {"normal-user"},
	}
	body, err := c.postForm(ctx, c.settings.BaseURL+"/api/add-user", form, "")
	if err == nil && strings.Contains(string(body), `"status":"ok"`) {
		return nil
	}
	// 已存在(或形态不符)则统一走重置密码兜底
	return c.SetPassword(ctx, username, password)
}

func (c *casdoorHTTPClient) SetPassword(ctx context.Context, username, password string) error {
	token, err := c.adminToken(ctx)
	if err != nil {
		return err
	}
	form := url.Values{
		"value": {password},
	}
	u := c.settings.BaseURL + "/api/set-password?id=" +
		url.QueryEscape(c.settings.OrgName+"/"+username)
	body, err := c.postForm(ctx, u, form, token)
	if err != nil {
		return fmt.Errorf("casdoor set-password failed: %w", err)
	}
	if !strings.Contains(string(body), `"status":"ok"`) {
		return fmt.Errorf("casdoor set-password: %s", strings.TrimSpace(string(body)))
	}
	return nil
}

func (c *casdoorHTTPClient) adminToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cachedAdminToken != "" && time.Now().Before(c.adminTokenExp) {
		return c.cachedAdminToken, nil
	}
	tok, err := c.PasswordToken(ctx, c.settings.AdminUsername, c.settings.AdminPassword)
	if err != nil {
		return "", fmt.Errorf("casdoor admin auth failed: %w", err)
	}
	c.cachedAdminToken = tok
	c.adminTokenExp = time.Now().Add(30 * time.Minute)
	return tok, nil
}

func (c *casdoorHTTPClient) postForm(ctx context.Context, rawURL string, form url.Values, bearer string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s -> HTTP %d: %s", req.URL.Path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if bearer != "" {
		return body, nil
	}
	// 无管理 token 的调用可能是 200 + 错误体,交由调用方判别
	return body, nil
}
