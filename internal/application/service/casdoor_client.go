package service

import (
	"bytes"
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
// The username is sent as "org/name" — the form Casdoor's ROPC expects.
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

// EnsureUser creates the Casdoor user via the admin API (POST /api/add-user,
// JSON body + Bearer). If creation fails but the user verifiably exists
// (GET /api/get-user reports it), the password is reset to the given one
// instead — that keeps first-login provisioning idempotent across
// half-finished attempts. Any other failure is returned verbatim.
func (c *casdoorHTTPClient) EnsureUser(ctx context.Context, username, email, password string) error {
	token, err := c.adminToken(ctx)
	if err != nil {
		return err
	}
	payload := map[string]string{
		"owner":       c.settings.OrgName,
		"name":        username,
		"displayName": username,
		"email":       email,
		"password":    password,
		"type":        "normal-user",
	}
	body, err := c.postJSON(ctx, c.settings.BaseURL+"/api/add-user", payload, token)
	if err == nil && casdoorStatus(body) == "ok" {
		return nil
	}
	if err == nil {
		err = fmt.Errorf("casdoor add-user: %s", strings.TrimSpace(string(body)))
	}
	if !c.userExists(ctx, token, username) {
		return err
	}
	// add-user 失败但用户确已存在:重置密码对齐给定值
	return c.SetPassword(ctx, username, password)
}

// SetPassword resets a Casdoor user's password. The upstream controller only
// reads the userOwner/userName/newPassword form fields — the ?id= query
// parameter is ignored by it.
func (c *casdoorHTTPClient) SetPassword(ctx context.Context, username, password string) error {
	token, err := c.adminToken(ctx)
	if err != nil {
		return err
	}
	form := url.Values{
		"userOwner":   {c.settings.OrgName},
		"userName":    {username},
		"newPassword": {password},
	}
	body, err := c.postForm(ctx, c.settings.BaseURL+"/api/set-password", form, token)
	if err != nil {
		return fmt.Errorf("casdoor set-password failed: %w", err)
	}
	if casdoorStatus(body) != "ok" {
		return fmt.Errorf("casdoor set-password: %s", strings.TrimSpace(string(body)))
	}
	return nil
}

// userExists asks Casdoor for the user record and reports whether it is
// really there. Any transport or decode failure counts as "not existing" —
// this only steers the recovery path inside EnsureUser.
func (c *casdoorHTTPClient) userExists(ctx context.Context, token, username string) bool {
	u := c.settings.BaseURL + "/api/get-user?id=" +
		url.QueryEscape(c.settings.OrgName+"/"+username)
	body, err := c.get(ctx, u, token)
	if err != nil {
		return false
	}
	// Casdoor 的 get-user 响应把用户字段平铺在顶层(name),有的版本包在
	// data 里;两种形状都认,以 status ok + 命中用户名为准。
	var resp struct {
		Status string `json:"status"`
		Name   string `json:"name"`
		Data   struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return false
	}
	name := resp.Name
	if name == "" {
		name = resp.Data.Name
	}
	return resp.Status == "ok" && name == username
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

// casdoorStatus extracts the "status" field Casdoor wraps every API response
// in; "" means the body was not a recognisable Casdoor response.
func casdoorStatus(body []byte) string {
	var resp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return ""
	}
	return resp.Status
}

func (c *casdoorHTTPClient) postForm(ctx context.Context, rawURL string, form url.Values, bearer string) ([]byte, error) {
	return c.do(ctx, http.MethodPost, rawURL, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()), bearer)
}

func (c *casdoorHTTPClient) postJSON(ctx context.Context, rawURL string, payload any, bearer string) ([]byte, error) {
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, rawURL, "application/json", bytes.NewReader(buf), bearer)
}

func (c *casdoorHTTPClient) get(ctx context.Context, rawURL, bearer string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, rawURL, "", nil, bearer)
}

// do issues one Casdoor request. Casdoor answers most API errors with HTTP
// 200 plus a {"status":"error"} body, so 200 bodies are handed back to the
// caller for status-based interpretation.
func (c *casdoorHTTPClient) do(ctx context.Context, method, rawURL, contentType string, body io.Reader, bearer string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s -> HTTP %d: %s", req.URL.Path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return b, nil
}
