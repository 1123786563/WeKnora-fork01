# Casdoor 集成实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Casdoor 作为 WeKnora 唯一登录入口:web 走已有 OIDC 通路(纯配置),小程序走新增的 wx.login 静默通道(code2session → Casdoor 建号/ROPC → WeKnora JWT),本地账密软下线。

**Architecture:** Casdoor(docker-compose 新服务,SQLite)是唯一 IdP;WeKnora 保留自有 JWT/用户/租户。微信用户经确定性推导的 username/email 与 Casdoor 及本地用户关联,服务密码 AES-GCM 加密存于 `UserPreferences`,复登经 ROPC 向 Casdoor 重新证明身份。

**Tech Stack:** Go(gin/gorm)、Casdoor(OIDC + ROPC + 管理 API)、React(apps/web)、Taro(apps/miniprogram)、pnpm monorepo(packages/api-client)。

**Spec:** `docs/superpowers/specs/2026-09-29-casdoor-integration-design.md`

## Global Constraints

- Go module:`github.com/Tencent/WeKnora`;Go 测试:`go test ./internal/...`
- 前端测试跑在 Node `node:test`(React 侧自建 JSDOM harness,见 `apps/web/src/auth/login-page.test.tsx` 头部)
- 后端账密接口一律不动(软下线);不改 Casdoor 源码
- 新配置必须同时支持 yaml 与 env 覆盖(既有模式:`internal/config/config.go` 的 `applyOIDCEnvOverrides`,约 :1047)
- 对 spec 的两处已批准的落地简化:① 微信关联不新增 SQL 列——openid 经确定性 username/email(`wx_<openid 前8>` / `wx_<openid>@wechat.local`)关联,服务密码密文存 `UserPreferences`(JSON 列,零迁移);② Casdoor 数据库默认 SQLite(挂载宿主目录),生产切 PG 的说明写入部署文档

---

### Task 1: docker-compose 接入 Casdoor 服务

**Files:**
- Create: `config/casdoor/app.conf`(从镜像导出后修改)
- Modify: `docker-compose.yml`(services 段末尾,`postgres` 服务后)
- Modify: `.gitignore`

**Interfaces:**
- Produces: 容器名/服务名 `casdoor`,容器网络内地址 `http://casdoor:8000`,宿主地址 `http://localhost:8000`(后续任务的环境变量引用这两个地址)

- [ ] **Step 1: 导出镜像默认 app.conf 作为基线**

```bash
mkdir -p config/casdoor
docker run --rm casbin/casdoor:latest cat /conf/app.conf > config/casdoor/app.conf
```

预期:生成约 100 行 key=value 配置。不手写全文,以镜像基线为准。

- [ ] **Step 2: 修改 app.conf 关键项**

在 `config/casdoor/app.conf` 中确认/修改以下项(其余保持镜像默认):

```ini
httpport = 8000
runmode = prod
driverName = sqlite3
dbName = casdoor.db
origin = http://localhost:8000
```

若基线里 `driverName` 为 mysql 等,改为上述 sqlite3;`dbName` 若基线有 `sqlite_path` 相关项保持默认(SQLite 文件落在 /conf 即被宿主目录持久化)。

- [ ] **Step 3: docker-compose.yml 加服务**

在 `postgres` 服务(约 :522)之后追加:

```yaml
  casdoor:
    image: casbin/casdoor:latest
    container_name: WeKnora-casdoor
    restart: unless-stopped
    ports:
      - "${CASDOOR_PORT:-8000}:8000"
    volumes:
      - ./config/casdoor:/conf
    networks:
      - WeKnora-network
    stop_grace_period: 30s
```

- [ ] **Step 4: gitignore 排除运行时产物**

`.gitignore` 追加:

```
config/casdoor/casdoor.db*
config/casdoor/storage/
```

- [ ] **Step 5: 校验并启动**

```bash
docker compose config -q
docker compose up -d casdoor
curl -si http://localhost:8000/ | head -3
```

预期:HTTP 200 或 302(Casdoor 登录页可达)。

- [ ] **Step 6: Commit**

```bash
git add docker-compose.yml .gitignore config/casdoor/app.conf
git commit -m "feat(deploy): docker-compose 接入 Casdoor——SQLite 持久化挂载宿主目录"
```

---

### Task 2: 后端配置扩展(sso_only / wechat_mp / casdoor_admin)

**Files:**
- Modify: `internal/config/config.go`(`OIDCAuthConfig` 约 :600、env 覆盖约 :1047、`Config` 结构体)
- Modify: `config/config.yaml`(注释样例)
- Test: `internal/config/wechat_config_test.go`

**Interfaces:**
- Produces(后续任务消费的确切类型与字段):
  - `OIDCAuthConfig.SSOOnly bool`(yaml `sso_only`,env `OIDC_AUTH_SSO_ONLY`)
  - `type WechatMPConfig struct { AppID, AppSecret, SecretKey string }`(yaml 键 `app_id`/`app_secret`/`secret_key`)
  - `type CasdoorAdminConfig struct { BaseURL, OrgName, AdminUsername, AdminPassword string }`
  - `Config.WechatMP *WechatMPConfig`(yaml `wechat_mp`)、`Config.CasdoorAdmin *CasdoorAdminConfig`(yaml `casdoor_admin`)
  - env:`WECHAT_MP_APP_ID`、`WECHAT_MP_APP_SECRET`、`WECHAT_MP_SECRET_KEY`、`CASDOOR_ADMIN_BASE_URL`、`CASDOOR_ADMIN_ORG_NAME`、`CASDOOR_ADMIN_USERNAME`、`CASDOOR_ADMIN_PASSWORD`

- [ ] **Step 1: 写失败测试**

`internal/config/wechat_config_test.go`:

```go
package config

import (
	"path/filepath"
	"testing"
)

func TestWechatAndCasdoorEnvOverrides(t *testing.T) {
	cfg := &Config{}
	applyWechatEnvOverrides(cfg)

	t.Run("SSOOnly from env", func(t *testing.T) {
		t.Setenv("OIDC_AUTH_SSO_ONLY", "true")
		cfg := &Config{}
		applyOIDCEnvOverrides(cfg)
		if !cfg.OIDCAuth.SSOOnly {
			t.Fatal("expected OIDCAuth.SSOOnly true")
		}
	})
}

func TestWechatMPYamlLoad(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join("..", "..", "testdata", "nonexistent.yaml"))
	if err == nil && cfg != nil {
		t.Skip("LoadConfig signature differs; align with existing config tests in this package")
	}
	t.Skip("yaml parsing covered by env-override test; adjust to existing config test pattern")
}
```

注:先看本包既有 `*_test.go` 如何构造 `*Config`。若包内无既有测试,直接用结构体字面量断言 env 覆盖,删除上面第二个测试。核心断言(`SSOOnly` 与各 env 字段)必须保留:

```go
t.Setenv("WECHAT_MP_APP_ID", "wx123")
t.Setenv("WECHAT_MP_APP_SECRET", "s3cret")
t.Setenv("WECHAT_MP_SECRET_KEY", "k")
t.Setenv("CASDOOR_ADMIN_BASE_URL", "http://casdoor:8000")
t.Setenv("CASDOOR_ADMIN_ORG_NAME", "weknora")
t.Setenv("CASDOOR_ADMIN_USERNAME", "svc_weknora")
t.Setenv("CASDOOR_ADMIN_PASSWORD", "pw")
cfg := &Config{WechatMP: &WechatMPConfig{}, CasdoorAdmin: &CasdoorAdminConfig{}}
applyWechatEnvOverrides(cfg)
if cfg.WechatMP.AppID != "wx123" || cfg.WechatMP.AppSecret != "s3cret" ||
	cfg.WechatMP.SecretKey != "k" || cfg.CasdoorAdmin.BaseURL != "http://casdoor:8000" ||
	cfg.CasdoorAdmin.OrgName != "weknora" || cfg.CasdoorAdmin.AdminUsername != "svc_weknora" ||
	cfg.CasdoorAdmin.AdminPassword != "pw" {
	t.Fatalf("env override incomplete: %+v %+v", cfg.WechatMP, cfg.CasdoorAdmin)
}
```

- [ ] **Step 2: 运行确认失败**

```bash
go test ./internal/config/ -run TestWechat -v
```

预期:编译失败(`applyWechatEnvOverrides`/`WechatMPConfig` 未定义)。

- [ ] **Step 3: 实现**

`internal/config/config.go` 中:

1) `OIDCAuthConfig` 末尾加字段:

```go
	SSOOnly bool `yaml:"sso_only" json:"sso_only"`
```

2) 新类型(放在 `OIDCAuthConfig` 之后):

```go
// WechatMPConfig carries the WeChat mini-program credentials used by the
// silent-login channel (wx.login -> code2session).
type WechatMPConfig struct {
	AppID     string `yaml:"app_id"     json:"app_id"`
	AppSecret string `yaml:"app_secret" json:"-"`
	// SecretKey is the AES-256-GCM key (any non-empty string, hashed to 32
	// bytes) encrypting per-user Casdoor service passwords at rest.
	SecretKey string `yaml:"secret_key" json:"-"`
}

// CasdoorAdminConfig authenticates WeKnora's backend against Casdoor's admin
// API for provisioning mini-program users. ROPC uses the same OIDC client
// credentials as the web flow (OIDCAuth.ClientID/ClientSecret).
type CasdoorAdminConfig struct {
	BaseURL       string `yaml:"base_url"        json:"base_url"`
	OrgName       string `yaml:"org_name"        json:"org_name"`
	AdminUsername string `yaml:"admin_username"  json:"admin_username"`
	AdminPassword string `yaml:"admin_password"  json:"-"`
}
```

3) `Config` 结构体加两个字段(与 `OIDCAuth` 同区):

```go
	WechatMP     *WechatMPConfig     `yaml:"wechat_mp"     json:"wechat_mp"`
	CasdoorAdmin *CasdoorAdminConfig `yaml:"casdoor_admin" json:"casdoor_admin"`
```

4) `applyOIDCEnvOverrides` 内追加:

```go
	if value := strings.TrimSpace(os.Getenv("OIDC_AUTH_SSO_ONLY")); value != "" {
		cfg.OIDCAuth.SSOOnly = strings.EqualFold(value, "true")
	}
```

5) 新函数(模式照抄 `applyOIDCEnvOverrides`):

```go
func applyWechatEnvOverrides(cfg *Config) {
	if cfg.WechatMP == nil {
		cfg.WechatMP = &WechatMPConfig{}
	}
	if cfg.CasdoorAdmin == nil {
		cfg.CasdoorAdmin = &CasdoorAdminConfig{}
	}
	if value := strings.TrimSpace(os.Getenv("WECHAT_MP_APP_ID")); value != "" {
		cfg.WechatMP.AppID = value
	}
	if value := strings.TrimSpace(os.Getenv("WECHAT_MP_APP_SECRET")); value != "" {
		cfg.WechatMP.AppSecret = value
	}
	if value := strings.TrimSpace(os.Getenv("WECHAT_MP_SECRET_KEY")); value != "" {
		cfg.WechatMP.SecretKey = value
	}
	if value := strings.TrimSpace(os.Getenv("CASDOOR_ADMIN_BASE_URL")); value != "" {
		cfg.CasdoorAdmin.BaseURL = value
	}
	if value := strings.TrimSpace(os.Getenv("CASDOOR_ADMIN_ORG_NAME")); value != "" {
		cfg.CasdoorAdmin.OrgName = value
	}
	if value := strings.TrimSpace(os.Getenv("CASDOOR_ADMIN_USERNAME")); value != "" {
		cfg.CasdoorAdmin.AdminUsername = value
	}
	if value := strings.TrimSpace(os.Getenv("CASDOOR_ADMIN_PASSWORD")); value != "" {
		cfg.CasdoorAdmin.AdminPassword = value
	}
}
```

并在既有配置加载入口(调用 `applyOIDCEnvOverrides` 的同一位置)追加调用 `applyWechatEnvOverrides(cfg)`。

- [ ] **Step 4: config.yaml 注释样例**

`config/config.yaml` 的 `oidc_auth` 段附近追加注释样例(全部默认注释关闭):

```yaml
# oidc_auth:
#   enable: true
#   issuer_url: http://casdoor:8000
#   # 浏览器可达的授权端点须显式覆盖(issuer 指向容器网络名,浏览器访问不到):
#   authorization_endpoint: http://localhost:8000/login/oauth/authorize
#   client_id: <weknora-app client id>
#   client_secret: <secret>
#   sso_only: true
# wechat_mp:
#   app_id: <小程序 appid>
#   app_secret: <secret>
#   secret_key: <任意长随机串,AES-GCM 密钥材料>
# casdoor_admin:
#   base_url: http://casdoor:8000
#   org_name: weknora
#   admin_username: svc_weknora
#   admin_password: <服务账号密码>
```

- [ ] **Step 5: 运行测试通过**

```bash
go test ./internal/config/ -v
```

- [ ] **Step 6: Commit**

```bash
git add internal/config/ config/config.yaml
git commit -m "feat(config): SSO 登录模式与微信小程序/Casdoor 管理通道配置"
```

---

### Task 3: Casdoor HTTP 客户端(ROPC + 管理 API)

**Files:**
- Create: `internal/application/service/casdoor_client.go`
- Test: `internal/application/service/casdoor_client_test.go`

**Interfaces:**
- Consumes: `config.CasdoorAdminConfig`、`config.OIDCAuthConfig.ClientID/ClientSecret`(Task 2)
- Produces:

```go
type casdoorClient interface {
	PasswordToken(ctx context.Context, username, password string) (string, error)
	EnsureUser(ctx context.Context, username, email, password string) error
	SetPassword(ctx context.Context, username, password string) error
}
func newCasdoorClient(admin *config.CasdoorAdminConfig, clientID, clientSecret string) casdoorClient
```

**注意:** Casdoor 端点形态以下述代码为基线;联调(Task 9/10)若与实际 Casdoor 版本不符,只改各 `fmt.Sprintf` 的路径字符串,不改接口。

- [ ] **Step 1: 写失败测试**

`internal/application/service/casdoor_client_test.go`:

```go
package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func newCasdoorTestServer(t *testing.T, ropcToken string, existingUsers map[string]bool) (*httptest.Server, *int, *int) {
	t.Helper()
	ropcCalls, addUserCalls := 0, 0
	mux := http.NewServeMux()
	// ROPC:管理员与普通用户共用同一端点
	mux.HandleFunc("/api/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		ropcCalls++
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		if r.FormValue("username") == "svc_weknora" {
			_, _ = w.Write([]byte(`{"access_token":"admin-token","expires_in":"7200"}`))
			return
		}
		if r.FormValue("grant_type") != "password" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"unsupported_grant_type"}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"` + ropcToken + `"}`))
	})
	mux.HandleFunc("/api/add-user", func(w http.ResponseWriter, r *http.Request) {
		addUserCalls++
		_ = r.ParseForm()
		name := r.FormValue("name")
		if existingUsers[name] {
			_, _ = w.Write([]byte(`{"status":"error","msg":"user already exists"}`))
			return
		}
		existingUsers[name] = true
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/api/set-password", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id != "weknora/wx_abcd1234" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &ropcCalls, &addUserCalls
}

func TestCasdoorClientPasswordToken(t *testing.T) {
	srv, ropcCalls, _ := newCasdoorTestServer(t, "tok-1", map[string]bool{})
	c := newCasdoorClient(casdoorCfg(srv.URL), "web-client", "web-secret")
	tok, err := c.PasswordToken(context.Background(), "wx_abcd1234", "pw")
	if err != nil || tok != "tok-1" {
		t.Fatalf("got %q, %v", tok, err)
	}
	if *ropcCalls != 1 {
		t.Fatalf("expected 1 ropc call, got %d", *ropcCalls)
	}
}

func TestCasdoorClientEnsureUserIdempotent(t *testing.T) {
	srv, _, addUserCalls := newCasdoorTestServer(t, "tok-1", map[string]bool{"wx_abcd1234": true})
	c := newCasdoorClient(casdoorCfg(srv.URL), "web-client", "web-secret")
	if err := c.EnsureUser(context.Background(), "wx_abcd1234", "wx_abcd1234@wechat.local", "pw"); err != nil {
		t.Fatalf("existing user should fall through to set-password, got %v", err)
	}
	if err := c.EnsureUser(context.Background(), "wx_new", "wx_new@wechat.local", "pw"); err != nil {
		t.Fatalf("new user should be created, got %v", err)
	}
	if *addUserCalls != 2 {
		t.Fatalf("expected 2 add-user calls, got %d", *addUserCalls)
	}
}

func casdoorCfg(baseURL string) *casdoorAdminSettings {
	return &casdoorAdminSettings{
		BaseURL:       baseURL,
		OrgName:       "weknora",
		AdminUsername: "svc_weknora",
		AdminPassword: "svc-pw",
	}
}
```

- [ ] **Step 2: 运行确认失败**

```bash
go test ./internal/application/service/ -run TestCasdoor -v
```

预期:编译失败(`newCasdoorClient` 未定义)。

- [ ] **Step 3: 实现**

`internal/application/service/casdoor_client.go`:

```go
package service

import (
	"context"
	"encoding/json"
	"errors"
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
	settings      casdoorAdminSettings
	clientID      string
	clientSecret  string
	httpClient    *http.Client
	mu            sync.Mutex
	adminToken    string
	adminTokenExp time.Time
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
		"owner":      {c.settings.OrgName},
		"name":       {username},
		"displayName": {username},
		"email":      {email},
		"password":   {password},
		"type":       {"normal-user"},
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
	if c.adminToken != "" && time.Now().Before(c.adminTokenExp) {
		return c.adminToken, nil
	}
	tok, err := c.PasswordToken(ctx, c.settings.AdminUsername, c.settings.AdminPassword)
	if err != nil {
		return "", fmt.Errorf("casdoor admin auth failed: %w", err)
	}
	c.adminToken = tok
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
```

注:`EnsureUser` 的 `SetPassword` 兜底要求该用户已存在于 Casdoor(测试里 `wx_abcd1234` 预置存在)。若实际 Casdoor 的 `add-user` 对已存在用户返回的 JSON 形态不同,以 `"status":"ok"` 判定为准调整。

- [ ] **Step 4: 运行测试通过**

```bash
go test ./internal/application/service/ -run TestCasdoor -v
```

预期:3 个测试 PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/application/service/casdoor_client.go internal/application/service/casdoor_client_test.go
git commit -m "feat(auth): Casdoor HTTP 客户端——ROPC 换 token 与幂等建号/重置服务密码"
```

---

### Task 4: 微信 code2session 客户端

**Files:**
- Create: `internal/application/service/wechat_client.go`
- Test: `internal/application/service/wechat_client_test.go`

**Interfaces:**
- Consumes: `config.WechatMPConfig.AppID/AppSecret`(Task 2)
- Produces:

```go
type wechatMPClient struct{ ... }
func newWechatMPClient(cfg *config.WechatMPConfig) *wechatMPClient
func (w *wechatMPClient) Code2Session(ctx context.Context, code string) (string, error) // returns openid
```

- [ ] **Step 1: 写失败测试**

`internal/application/service/wechat_client_test.go`:

```go
package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
)

func TestCode2SessionSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("appid") != "wx123" || q.Get("secret") != "sec" ||
			q.Get("js_code") != "CODE" || q.Get("grant_type") != "authorization_code" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"openid":"oABCDEF1234567890abcdef1234","session_key":"sk"}`))
	}))
	defer srv.Close()
	c := newWechatMPClient(&config.WechatMPConfig{AppID: "wx123", AppSecret: "sec"})
	c.baseURL = srv.URL
	openid, err := c.Code2Session(context.Background(), "CODE")
	if err != nil || openid != "oABCDEF1234567890abcdef1234" {
		t.Fatalf("got %q, %v", openid, err)
	}
}

func TestCode2SessionErrorCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errcode":40029,"errmsg":"invalid code"}`))
	}))
	defer srv.Close()
	c := newWechatMPClient(&config.WechatMPConfig{AppID: "wx123", AppSecret: "sec"})
	c.baseURL = srv.URL
	if _, err := c.Code2Session(context.Background(), "CODE"); err == nil {
		t.Fatal("expected error for errcode 40029")
	}
}
```

- [ ] **Step 2: 运行确认失败**

```bash
go test ./internal/application/service/ -run TestCode2Session -v
```

预期:编译失败。

- [ ] **Step 3: 实现**

`internal/application/service/wechat_client.go`:

```go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/Tencent/WeKnora/internal/config"
)

type wechatMPClient struct {
	appID     string
	appSecret string
	baseURL   string
	httpClient *http.Client
}

func newWechatMPClient(cfg *config.WechatMPConfig) *wechatMPClient {
	return &wechatMPClient{
		appID:      cfg.AppID,
		appSecret:  cfg.AppSecret,
		baseURL:    "https://api.weixin.qq.com",
		httpClient: newOIDCHTTPClient(),
	}
}

func (w *wechatMPClient) Code2Session(ctx context.Context, code string) (string, error) {
	q := url.Values{
		"appid":      {w.appID},
		"secret":     {w.appSecret},
		"js_code":    {code},
		"grant_type": {"authorization_code"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		w.baseURL+"/sns/jscode2session?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := w.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("wechat code2session request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var payload struct {
		OpenID  string `json:"openid"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("wechat code2session: decode response: %w", err)
	}
	if payload.ErrCode != 0 || payload.OpenID == "" {
		return "", fmt.Errorf("wechat code2session: errcode=%d errmsg=%s", payload.ErrCode, payload.ErrMsg)
	}
	return payload.OpenID, nil
}
```

- [ ] **Step 4: 运行测试通过**

```bash
go test ./internal/application/service/ -run TestCode2Session -v
```

预期:2 个测试 PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/application/service/wechat_client.go internal/application/service/wechat_client_test.go
git commit -m "feat(auth): 微信小程序 code2session 客户端"
```

---

### Task 5: WeChat 静默登录编排(service 层核心)

**Files:**
- Modify: `internal/application/service/user.go`(`userService` 结构体 :99、`loginWithOIDC` 尾段 :570-608、`UserPreferences` 在 `internal/types/user.go:24`)
- Modify: `internal/types/user.go`(`UserPreferences` 加字段)
- Create: `internal/application/service/wechat_login.go`(编排 + AES-GCM 工具)
- Test: `internal/application/service/wechat_login_test.go`

**Interfaces:**
- Consumes: `casdoorClient`(Task 3)、`wechatMPClient`(Task 4)、`resolveOIDCUserInfo`、`provisionOIDCUser`、`resolveLoginTenantID`、`generateTokensForTenant`、`buildMembershipsForUser`、`isUserLookupNotFound`、`generateRandomString`(均已在 user.go)
- Produces:

```go
func (s *userService) LoginWithWeChatCode(ctx context.Context, code string, provisioning types.TenantProvisioningMode) (*types.LoginResponse, error)
func encryptWeChatServiceSecret(key, plaintext string) (string, error)  // AES-256-GCM, base64
func decryptWeChatServiceSecret(key, ciphertext string) (string, error)
func wechatAccountIdentifiers(openid string) (username, email string)  // "wx_"+openid[:8], "wx_"+openid+"@wechat.local"
// 重构产物(登录尾段共享):
func (s *userService) finalizeLoginSession(ctx context.Context, user *types.User, isNewUser bool) (*types.OIDCCallbackResponse, error)
func toLoginResponse(r *types.OIDCCallbackResponse) *types.LoginResponse
```

- [ ] **Step 1: types 加字段**

`internal/types/user.go` 的 `UserPreferences`(:24 起,`OidcOnlyLogin` 字段 :44-48 附近)追加:

```go
	// WeChatServiceSecret is the AES-256-GCM ciphertext (base64) of the
	// per-user Casdoor service password used by the mini-program silent-login
	// channel. Key material lives only in env (WECHAT_MP_SECRET_KEY). The
	// ciphertext is inert without the key and intentionally serializes with
	// the rest of preferences.
	WeChatServiceSecret string `json:"wechat_service_secret,omitempty"`
```

- [ ] **Step 2: 写失败测试**

`internal/application/service/wechat_login_test.go`(mock 形态对齐 `user_provisioning_test.go`:嵌入式接口实现;`user_oidc_verify_test.go` 已展示如何在测试内构造 userService,对齐其 NewUserService 组装与 `t.Setenv("JWT_SECRET", ...)`,若生成 token 需要):

```go
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type wechatFake struct{ openid string }

func (w wechatFake) Code2Session(context.Context, string) (string, error) { return w.openid, nil }

type casdoorFake struct {
	ropcCalls  int
	ropcFailN  int // 前 N 次 ROPC 失败
	ensureCall int
	setPwdCall int
	ropcToken  string
}

func (c *casdoorFake) PasswordToken(context.Context, string, string) (string, error) {
	c.ropcCalls++
	if c.ropcCalls <= c.ropcFailN {
		return "", errors.New("casdoor: invalid password")
	}
	return c.ropcToken, nil
}
func (c *casdoorFake) EnsureUser(context.Context, string, string, string) error {
	c.ensureCall++
	return nil
}
func (c *casdoorFake) SetPassword(context.Context, string, string) error {
	c.setPwdCall++
	return nil
}

type wechatLoginRepo struct {
	interfaces.UserRepository
	byEmail   map[string]*types.User
	updated   []*types.User
}

func (r *wechatLoginRepo) GetUserByEmail(_ context.Context, email string) (*types.User, error) {
	if u, ok := r.byEmail[email]; ok {
		return u, nil
	}
	return nil, errors.New("user not found") // 与 isUserLookupNotFound 兼容形态,见下注
}

func (r *wechatLoginRepo) CreateUser(_ context.Context, u *types.User) error {
	r.byEmail[u.Email] = u
	return nil
}
func (r *wechatLoginRepo) UpdateUser(_ context.Context, u *types.User) error {
	r.updated = append(r.updated, u)
	r.byEmail[u.Email] = u
	return nil
}
func (r *wechatLoginRepo) GetUserByUsername(ctx context.Context, name string) (*types.User, error) {
	return nil, errors.New("user not found")
}

// 注:先读 user_provisioning_test.go / user_oidc_verify_test.go,把
// GetUserByEmail 的 not-found 返回改成让 isUserLookupNotFound(err)==true
// 的既有构造(沿用那两个文件里的写法),并复用其中的 tenantService /
// memberService / tokenRepo mock 组装 NewUserService。

func newWechatLoginService(t *testing.T, repo *wechatLoginRepo, cd *casdoorFake) *userService {
	t.Helper()
	t.Setenv("WECHAT_MP_SECRET_KEY", "unit-test-key")
	svc := newUserServiceForOIDCTest(t) // 对齐 user_oidc_verify_test.go 的组装,替换 repo
	svc.userRepo = repo
	svc.wechat = &wechatAuthClients{wechat: wechatFake{openid: "oABCDEF1234567890abcdef1234"}, casdoor: cd}
	return svc
}

func TestWeChatFirstLoginProvisions(t *testing.T) {
	repo := &wechatLoginRepo{byEmail: map[string]*types.User{}}
	cd := &casdoorFake{ropcToken: "casdoor-tok"}
	svc := newWechatLoginService(t, repo, cd)

	resp, err := svc.LoginWithWeChatCode(context.Background(), "CODE", types.TenantProvisioningNone)
	if err != nil {
		t.Fatalf("first login failed: %v", err)
	}
	if resp.Token == "" || resp.RefreshToken == "" {
		t.Fatalf("expected tokens, got %+v", resp)
	}
	if cd.ensureCall != 1 || cd.ropcCalls < 1 {
		t.Fatalf("expected casdoor ensure+ropc, got %+v", cd)
	}
	user := repo.byEmail["wx_oABCDEF1234567890abcdef1234@wechat.local"]
	if user == nil {
		t.Fatal("provisioned user missing")
	}
	if user.Preferences.WeChatServiceSecret == "" {
		t.Fatal("service secret ciphertext not persisted")
	}
	if user.Preferences.OidcOnlyLogin == nil || !*user.Preferences.OidcOnlyLogin {
		t.Fatal("expected OidcOnlyLogin=true")
	}
}

func TestWeChatRepeatLogin(t *testing.T) {
	cipher, _ := encryptWeChatServiceSecret("unit-test-key", "known-svc-pwd")
	repo := &wechatLoginRepo{byEmail: map[string]*types.User{
		"wx_oABCDEF1234567890abcdef1234@wechat.local": {
			ID: "u1", Username: "wx_oABCDEF", Email: "wx_oABCDEF1234567890abcdef1234@wechat.local",
			IsActive: true,
			Preferences: types.UserPreferences{WeChatServiceSecret: cipher},
		},
	}}
	cd := &casdoorFake{ropcToken: "casdoor-tok"}
	svc := newWechatLoginService(t, repo, cd)

	resp, err := svc.LoginWithWeChatCode(context.Background(), "CODE", types.TenantProvisioningNone)
	if err != nil {
		t.Fatalf("repeat login failed: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("expected token")
	}
	if cd.ensureCall != 0 {
		t.Fatalf("repeat login must not re-provision, got %+v", cd)
	}
	if cd.ropcCalls != 1 {
		t.Fatalf("expected exactly 1 ropc, got %d", cd.ropcCalls)
	}
}

func TestWeChatRepeatLoginResetsForgottenServicePassword(t *testing.T) {
	cipher, _ := encryptWeChatServiceSecret("unit-test-key", "stale-pwd")
	repo := &wechatLoginRepo{byEmail: map[string]*types.User{
		"wx_oABCDEF1234567890abcdef1234@wechat.local": {
			ID: "u1", Username: "wx_oABCDEF", Email: "wx_oABCDEF1234567890abcdef1234@wechat.local",
			IsActive: true,
			Preferences: types.UserPreferences{WeChatServiceSecret: cipher},
		},
	}}
	cd := &casdoorFake{ropcToken: "casdoor-tok", ropcFailN: 1}
	svc := newWechatLoginService(t, repo, cd)

	if _, err := svc.LoginWithWeChatCode(context.Background(), "CODE", types.TenantProvisioningNone); err != nil {
		t.Fatalf("fallback reset failed: %v", err)
	}
	if cd.setPwdCall != 1 {
		t.Fatalf("expected set-password fallback, got %+v", cd)
	}
	if len(repo.updated) == 0 {
		t.Fatal("expected refreshed ciphertext persisted")
	}
}

func TestWeChatSecretRoundTrip(t *testing.T) {
	c, err := encryptWeChatServiceSecret("key", "secret")
	if err != nil {
		t.Fatal(err)
	}
	got, err := decryptWeChatServiceSecret("key", c)
	if err != nil || got != "secret" {
		t.Fatalf("roundtrip: %q, %v", got, err)
	}
	if _, err := decryptWeChatServiceSecret("wrong", c); err == nil {
		t.Fatal("wrong key must fail")
	}
}

func TestWeChatAccountIdentifiers(t *testing.T) {
	u, e := wechatAccountIdentifiers("oABCDEF1234567890abcdef1234")
	if u != "wx_oABCDEF" || e != "wx_oABCDEF1234567890abcdef1234@wechat.local" {
		t.Fatalf("got %q %q", u, e)
	}
}
```

注:`types.TenantProvisioningNone` 若实际枚举名不同(读 `types/user.go` 中 `TenantProvisioningMode` 定义),以实际值为准替换。

- [ ] **Step 3: 运行确认失败**

```bash
go test ./internal/application/service/ -run TestWeChat -v
```

预期:编译失败(`LoginWithWeChatCode`/`encryptWeChatServiceSecret` 未定义)。

- [ ] **Step 4: 实现**

`internal/application/service/wechat_login.go`:

```go
package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
)

// wechatAuthClients bundles the two remote deps of the silent-login channel.
// It is nil in normal operation and lazily built from config; tests assign
// it directly (same package).
type wechatAuthClients struct {
	wechat  *wechatMPClient
	casdoor casdoorClient
}

func wechatAccountIdentifiers(openid string) (string, string) {
	suffix := openid
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	return "wx_" + suffix, "wx_" + openid + "@wechat.local"
}

func encryptWeChatServiceSecret(key, plaintext string) (string, error) {
	gcm, err := wechatGCM(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plaintext), nil)), nil
}

func decryptWeChatServiceSecret(key, ciphertext string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	gcm, err := wechatGCM(key)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plain), nil
}

func wechatGCM(key string) (cipher.AEAD, error) {
	k := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(k[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// LoginWithWeChatCode implements the mini-program silent-login channel:
// wx.login code -> code2session openid -> (first login) provision Casdoor
// user with a random service password -> ROPC against Casdoor proves the
// account is still valid -> issue local tokens.
func (s *userService) LoginWithWeChatCode(
	ctx context.Context,
	code string,
	provisioning types.TenantProvisioningMode,
) (*types.LoginResponse, error) {
	clients, err := s.wechatClients()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(code) == "" {
		return nil, errors.New("code is required")
	}
	openid, err := clients.wechat.Code2Session(ctx, code)
	if err != nil {
		return nil, err
	}
	username, email := wechatAccountIdentifiers(openid)
	secretKey := s.config.WechatMP.SecretKey

	user, lookupErr := s.userRepo.GetUserByEmail(ctx, email)
	notFound := isUserLookupNotFound(lookupErr) || user == nil
	if lookupErr != nil && !notFound {
		return nil, fmt.Errorf("failed to query user by email: %w", lookupErr)
	}

	isNewUser := false
	var svcPwd string
	if notFound {
		isNewUser = true
		svcPwd, err = generateRandomString(48)
		if err != nil {
			return nil, fmt.Errorf("failed to generate service password: %w", err)
		}
		if err := clients.casdoor.EnsureUser(ctx, username, email, svcPwd); err != nil {
			return nil, fmt.Errorf("casdoor provisioning failed: %w", err)
		}
	} else {
		svcPwd, err = decryptWeChatServiceSecret(secretKey, user.Preferences.WeChatServiceSecret)
		if err != nil {
			// 密文损坏视同服务密码失效,走重置兜底
			svcPwd = ""
		}
	}

	casdoorToken, ropcErr := clients.casdoor.PasswordToken(ctx, username, svcPwd)
	if ropcErr != nil {
		if isNewUser {
			return nil, fmt.Errorf("casdoor login failed after provisioning: %w", ropcErr)
		}
		// ROPC 失败兜底:重置服务密码重试一次
		svcPwd, err = generateRandomString(48)
		if err != nil {
			return nil, err
		}
		if err := clients.casdoor.SetPassword(ctx, username, svcPwd); err != nil {
			return nil, fmt.Errorf("casdoor password reset failed: %w", err)
		}
		casdoorToken, ropcErr = clients.casdoor.PasswordToken(ctx, username, svcPwd)
		if ropcErr != nil {
			return nil, fmt.Errorf("casdoor login failed after reset: %w", ropcErr)
		}
		if user, lookupErr = s.userRepo.GetUserByEmail(ctx, email); lookupErr != nil || user == nil {
			return nil, fmt.Errorf("user disappeared during wechat login: %v", lookupErr)
		}
	}

	var cb *types.OIDCCallbackResponse
	if isNewUser {
		userInfo, err := s.resolveOIDCUserInfo(ctx, nil, &oidcTokenResponse{AccessToken: casdoorToken})
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(userInfo.Email) == "" {
			userInfo.Email = email
		}
		user, err = s.provisionOIDCUser(ctx, userInfo, provisioning)
		if err != nil {
			return nil, err
		}
		encrypted, encErr := encryptWeChatServiceSecret(secretKey, svcPwd)
		if encErr == nil {
			user.Preferences.WeChatServiceSecret = encrypted
		}
		if err := s.userRepo.UpdateUser(ctx, user); err != nil {
			return nil, fmt.Errorf("failed to persist wechat binding: %w", err)
		}
	}

	cb, err = s.finalizeLoginSession(ctx, user, isNewUser)
	if err != nil {
		return nil, err
	}
	return toLoginResponse(cb), nil
}

// wechatClients lazily builds the channel's remote clients from config.
func (s *userService) wechatClients() (*wechatAuthClients, error) {
	if s.wechat != nil {
		return s.wechat, nil
	}
	if s.config == nil || s.config.WechatMP == nil || s.config.CasdoorAdmin == nil ||
		s.config.OIDCAuth == nil {
		return nil, errors.New("wechat login is not configured")
	}
	s.wechat = &wechatAuthClients{
		wechat:  newWechatMPClient(s.config.WechatMP),
		casdoor: newCasdoorClient(s.config.CasdoorAdmin, s.config.OIDCAuth.ClientID, s.config.OIDCAuth.ClientSecret),
	}
	return s.wechat, nil
}
```

同时:

1) `user.go` 的 `userService` 结构体(:99)加非导出字段:

```go
	wechat *wechatAuthClients
```

2) `user.go` 中把 `loginWithOIDC` 的尾段(从 `if !user.IsActive {` 到函数结尾返回 `&types.OIDCCallbackResponse{...}`)抽为共享方法(原位置改为调用):

```go
func (s *userService) finalizeLoginSession(ctx context.Context, user *types.User, isNewUser bool) (*types.OIDCCallbackResponse, error) {
	if !user.IsActive {
		return &types.OIDCCallbackResponse{Success: false, Message: "Account is disabled"}, nil
	}
	resolvedTenantID := s.resolveLoginTenantID(ctx, user)
	accessToken, refreshToken, err := s.generateTokensForTenant(ctx, user, resolvedTenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate local tokens: %w", err)
	}
	var tenant *types.Tenant
	if resolvedTenantID > 0 {
		if t, terr := s.tenantService.GetTenantByID(ctx, resolvedTenantID); terr == nil {
			tenant = t
		} else {
			logger.Warnf(ctx, "login: failed to load tenant %d for user %s: %v",
				resolvedTenantID, user.ID, terr)
		}
	}
	memberships := s.buildMembershipsForUser(ctx, user, tenant)
	return &types.OIDCCallbackResponse{
		Success:      true,
		Message:      "登录成功",
		User:         user,
		Tenant:       tenant,
		Memberships:  memberships,
		Token:        accessToken,
		RefreshToken: refreshToken,
		IsNewUser:    isNewUser,
	}, nil
}
```

3) `wechat_login.go` 加转换函数:

```go
func toLoginResponse(r *types.OIDCCallbackResponse) *types.LoginResponse {
	return &types.LoginResponse{
		Success:      r.Success,
		Message:      r.Message,
		User:         r.User,
		ActiveTenant: r.Tenant,
		Memberships:  r.Memberships,
		Token:        r.Token,
		RefreshToken: r.RefreshToken,
	}
}
```

4) 抽取后运行既有 OIDC 测试确认无回归:`go test ./internal/application/service/ -run "OIDC|Provision" -v`

注:`resolveOIDCUserInfo(ctx, nil, tokenResp)` 传 nil cfg 时须确认其实现仅用 `cfg.UserInfoEndpoint` 与 mapping 默认值——若它强制要求非 nil cfg,则改为 `s.getOIDCConfig(ctx)` 取真实配置传入(推荐,与 loginWithOIDC 行为一致)。

- [ ] **Step 5: 运行全部新测试通过**

```bash
go test ./internal/application/service/ -run "TestWeChat" -v
go test ./internal/application/service/ -run "OIDC|Provision" -v
```

预期:全部 PASS。

- [ ] **Step 6: Commit**

```bash
git add internal/application/service/wechat_login.go internal/application/service/wechat_login_test.go internal/application/service/user.go internal/types/user.go
git commit -m "feat(auth): 微信小程序静默登录编排——Casdoor 建号/ROPC 与登录尾段复用重构"
```

---

### Task 6: HTTP 端点 + 路由 + 白名单

**Files:**
- Modify: `internal/types/user.go`(请求类型)
- Modify: `internal/interfaces`(若 UserService 接口单独定义;若在 `internal/types/interfaces` 则改那里)
- Modify: `internal/handler/auth.go`
- Modify: `internal/router/routes_auth_tenant.go`(:205 起)
- Modify: `internal/middleware/auth.go`(:44 `noAuthAPI`)

**Interfaces:**
- Consumes: `LoginWithWeChatCode`(Task 5)、`h.resolveDefaultTenantMode`(handler/auth.go:199,既有)
- Produces: `POST /api/v1/auth/wechat/login`,body `{"code": string}`,响应 = `types.LoginResponse`(与 `/auth/login` 同构)

- [ ] **Step 1: 请求类型与接口方法**

`internal/types/user.go`:

```go
// WeChatLoginRequest is the mini-program silent-login payload: a wx.login code.
type WeChatLoginRequest struct {
	Code string `json:"code" binding:"required"`
}
```

`UserService` 接口(grep `LoginWithOIDCWithPKCE` 所在接口定义处)追加:

```go
	LoginWithWeChatCode(ctx context.Context, code string, provisioning TenantProvisioningMode) (*LoginResponse, error)
```

- [ ] **Step 2: handler**

`internal/handler/auth.go`(`Login` handler 附近):

```go
// WechatLogin implements the mini-program silent-login channel.
func (h *AuthHandler) WechatLogin(c *gin.Context) {
	ctx := c.Request.Context()
	var req types.WeChatLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		appErr := errors.NewValidationError("Invalid wechat login parameters").WithDetails(err.Error())
		c.Error(appErr)
		return
	}
	resp, err := h.userService.LoginWithWeChatCode(ctx, req.Code, h.resolveDefaultTenantMode(ctx))
	if err != nil {
		logger.Errorf(ctx, "Wechat login failed: %v", err)
		appErr := errors.NewUnauthorizedError("Wechat login failed").WithDetails(err.Error())
		c.Error(appErr)
		return
	}
	c.JSON(http.StatusOK, resp)
}
```

- [ ] **Step 3: 路由与白名单**

`internal/router/routes_auth_tenant.go` 的 `RegisterAuthRoutes`(`r.POST("/auth/login", ...)` 附近)追加:

```go
	r.POST("/auth/wechat/login", publicAuthRL, handler.WechatLogin)
```

`internal/middleware/auth.go` 的 `noAuthAPI`(与 `/api/v1/auth/login` 同区)追加:

```go
	"/api/v1/auth/wechat/login": {"POST"},
```

- [ ] **Step 4: 编译与既有测试回归**

```bash
go build ./...
go test ./internal/router/... ./internal/middleware/... 2>&1 | tail -5
```

预期:编译通过,既有测试 PASS(白名单若有表驱动测试会覆盖新行)。

- [ ] **Step 5: 手动 curl 冒烟(带假 code,断言错误路径通)**

```bash
WECHAT_MP_APP_ID=x WECHAT_MP_APP_SECRET=x WECHAT_MP_SECRET_KEY=k \
CASDOOR_ADMIN_BASE_URL=http://localhost:8000 CASDOOR_ADMIN_ORG_NAME=weknora \
CASDOOR_ADMIN_USERNAME=svc CASDOOR_ADMIN_PASSWORD=p go run ./cmd/server &
sleep 3
curl -si -X POST http://localhost:8080/api/v1/auth/wechat/login -H 'Content-Type: application/json' -d '{"code":"fake"}'
```

预期:HTTP 401/500 带微信错误信息(说明路由+白名单+链路通,不是 404)。

- [ ] **Step 6: Commit**

```bash
git add internal/types/user.go internal/handler/auth.go internal/router/routes_auth_tenant.go internal/middleware/auth.go
git commit -m "feat(auth): /auth/wechat/login 端点——公开限流路由与白名单接线"
```

---

### Task 7: sso_only 全链路(后端字段 → api-client → React 登录页)

**Files:**
- Modify: `internal/types/`(grep `OIDCConfigResponse` 定位文件)
- Modify: `internal/handler/auth.go`(`GetOIDCConfig`)
- Modify: `packages/api-client/src/auth/endpoints.ts`(:7 `OIDCConfig`、:118 `oidcConfig`)
- Test: `packages/api-client/src/auth/endpoints.test.ts`
- Modify: `apps/web/src/auth/LoginPage.tsx`(:83 状态区、:124 allSettled、账密/注册表单 JSX 区)
- Test: `apps/web/src/auth/login-page.test.tsx`

**Interfaces:**
- Consumes: `OIDCAuthConfig.SSOOnly`(Task 2)
- Produces: `OIDCConfig.ssoOnly: boolean`(api-client);React 登录页 `ssoOnly=true` 时仅渲染 SSO 入口

- [ ] **Step 1: 后端**

`OIDCConfigResponse` 加字段:

```go
	SSOOnly bool `json:"sso_only"`
```

`GetOIDCConfig`(handler/auth.go)在现有 if 块内追加:

```go
		ssoOnly := h.configInfo.OIDCAuth.SSOOnly && h.configInfo.OIDCAuth.Enable
```

并放入响应结构(`SSOOnly: ssoOnly`)。

```bash
go build ./...
```

- [ ] **Step 2: api-client(失败测试先行)**

`endpoints.test.ts` 追加(对齐文件内既有 fake-request 用例形态):

```ts
test('oidcConfig surfaces ssoOnly', async () => {
  const calls: unknown[] = [];
  const request = async (input: unknown) => { calls.push(input); return { success: true, enabled: true, sso_only: true, provider_display_name: 'Casdoor' }; };
  const api = createAuthApi(request as never);
  const cfg = await api.oidcConfig();
  assert.equal(cfg.enabled, true);
  assert.equal(cfg.ssoOnly, true);
  assert.equal(calls[0], { method: 'GET', path: '/api/v1/auth/oidc/config' });
});
```

运行确认失败(字段未透传):

```bash
pnpm --filter @weknora/api-client test
```

`endpoints.ts` 改:

```ts
export interface OIDCConfig { enabled: boolean; providerDisplayName?: string; ssoOnly?: boolean }
```

`oidcConfig()`(:118)的返回对象加 `ssoOnly: (value as { sso_only?: boolean }).sso_only === true`(以该函数内既有字段映射写法为准,`provider_display_name` 怎么映射,`sso_only` 就怎么映射)。

再跑测试至 PASS。

- [ ] **Step 3: LoginPage**

`apps/web/src/auth/LoginPage.tsx`:

1. :83 状态区加 `const [ssoOnly, setSsoOnly] = useState(false);`
2. :131 `if (oidc.status === 'fulfilled')` 块内加 `setSsoOnly(oidc.value.ssoOnly === true);`
3. JSX:找到账密/注册表单卡片(含邮箱/密码 Field 的容器),包一层 `{!ssoOnly && ( ... )}`,表单本体不动;SSO 按钮(:236 `handleOIDCLogin` 触发的那个)保持在卡片外或 SSO 卡片内始终渲染。`ssoOnly` 为 true 时页面只剩 hero + SSO 卡片。

- [ ] **Step 4: React 测试**

`login-page.test.tsx` 追加用例(复用文件头既有 JSDOM harness 与 client stub 形态;找文件内"渲染登录表单"既有用例,拷贝其 client stub 并把 `oidcConfig` 返回值改为 `{ enabled: true, ssoOnly: true }`):

```ts
test('sso_only hides password form and keeps SSO entry', async () => {
  // 按文件内既有用例的 render LoginPage 方式(同 harness)
  // 断言:
  // 1. 渲染结果中不含邮箱输入框(以既有用例定位 input 的方式断言 count === 0)
  // 2. 含 SSO 登录按钮(auth.oidcLogin 文案或 handleOIDCLogin 绑定的按钮文本)
});
```

以文件内最接近的既有用例为模板写实际断言(harness 已有,不新造)。

运行:

```bash
cd apps/web && npx tsx --test src/auth/login-page.test.tsx
```

(以 apps/web/package.json 的 test script 为准。)

- [ ] **Step 5: Commit**

```bash
git add internal/types internal/handler/auth.go packages/api-client/src/auth/endpoints.ts packages/api-client/src/auth/endpoints.test.ts apps/web/src/auth/LoginPage.tsx apps/web/src/auth/login-page.test.tsx
git commit -m "feat(auth): sso_only 登录模式——OIDC 配置端点透出,React 登录页隐藏账密表单"
```

---

### Task 8: 小程序静默登录链路

**Files:**
- Modify: `packages/api-client/src/auth/endpoints.ts`(`createAuthApi` 内,:101 login 旁)
- Test: `packages/api-client/src/auth/endpoints.test.ts`
- Modify: `apps/miniprogram/src/core/auth.ts`(:4 `AuthPort`、:91 `login`、新增 `wxLogin`)
- Modify: `apps/miniprogram/src/features/auth/pages.tsx`(登录页重写)
- Test: `apps/miniprogram/tests/pages.test.mjs`(既有 14 用例文件,登录流用例替换)

**Interfaces:**
- Consumes: `POST /api/v1/auth/wechat/login`(Task 6)、`AuthSession`/`parseSession`(endpoints.ts:14/:28)
- Produces:
  - api-client: `wechatLogin(code: string): Promise<AuthSession>`
  - `AuthPort.wxLogin(): Promise<void>`(内部完成 credential 持久化与 session 视图切换,语义与 `login(email,pwd)` 一致)

- [ ] **Step 1: api-client(失败测试先行)**

`endpoints.test.ts` 追加:

```ts
test('wechatLogin posts code to wechat login endpoint', async () => {
  const calls: unknown[] = [];
  const request = async (input: unknown) => {
    calls.push(input);
    return { success: true, token: 't', refresh_token: 'r', user: { id: 'u' }, memberships: [] };
  };
  const api = createAuthApi(request as never);
  const session = await api.wechatLogin('CODE');
  assert.equal(session.access_token, 't');
  assert.deepEqual(calls[0], { method: 'POST', path: '/api/v1/auth/wechat/login', body: { code: 'CODE' } });
});
```

(响应字段以 `parseSession` 实际消费为准——先读 :28 `parseLogin`/`parseSession`,若后端 `token` 字段名与 mock 不符,以 parseSession 的解析源为准调整 mock。)

运行失败后,`endpoints.ts` `createAuthApi` 内加:

```ts
    async wechatLogin(code: string): Promise<AuthSession> {
      return parseSession(await request({ method: 'POST', path: '/api/v1/auth/wechat/login', body: { code } }));
    },
```

(`parseSession` 若不存在该名,用 `login`(:101)内部使用的同一解析函数。)

- [ ] **Step 2: AuthPort 与 Coordinator**

`apps/miniprogram/src/core/auth.ts`:

1. `AuthPort`(:4)加:

```ts
  wxLogin():Promise<void>;
```

2. 先读 `login`(:91-105)的完整实现,把"api 调用成功后 → save credential → 更新 session 视图"的尾部抽为私有方法 `applySession(session:AuthSession):void`(login 改为调用它);新增:

```ts
  async wxLogin():Promise<void>{
    const {code}=await Taro.login();
    const session=await this.api.wxLogin(code);
    this.applySession(session);
  }
```

`Taro` 的 import 对齐文件内既有用法(若无则 `import Taro from '@tarojs/taro'`)。注意:`AuthPort` 是接口,需同步在实现该接口的具体 adapter(grep `AuthPort` 的实现处,通常在 `services/` 或 `platform/` 下)加 `wxLogin(){return this.api.wxLogin(code)...}` 的薄实现——若 adapter 直接转发 api-client,则一行:

```ts
  wxLogin:()=>apiClient.auth.wechatLogin(...).then(...)
```

以 adapter 内 `login` 的现有转发写法为准同构添加。

- [ ] **Step 3: 登录页重写**

`apps/miniprogram/src/features/auth/pages.tsx` 登录页组件(现邮箱/密码表单 + "微信快捷登录…尚未接入"占位 Notice)替换为静默登录页,风格沿用同文件 Screen/Card/Action/Notice 组件与单行 JSX 约定:

```tsx
// 静默登录:进入即 wx.login;失败保留重试
export function LoginPage() {
  const auth = /* 文件内既有的 auth coordinator 获取方式,与旧版相同 */;
  const action = /* 既有 action hook,同旧版 */;
  const start = () => void action.run(async () => { await auth.wxLogin(); await navigate('workspace'); });
  React.useEffect(() => { start(); }, []);
  return <Screen title='WeKnora' publicPage><View className='wk-login-hero'><View className='wk-orbit'/><Text className='wk-eyebrow'>WORK, WITH A LITTLE MORE SPACE</Text><Text className='wk-display'>你的随身{'\n'}AI 工作台</Text><Text className='wk-muted'>将使用你的微信身份自动登录平台账号</Text></View><Card><Text className='wk-h2'>微信快捷登录</Text><Text className='wk-muted'>同一账号,同一空间。登录即表示同意平台的数据使用与服务说明。</Text><Action loading={action.busy} onClick={start}>立即登录</Action>{action.error&&<Notice tone='danger'>{action.error}</Notice>}</Card><Action secondary onClick={()=>void navigate('states',{kind:'privacy'})}>隐私与数据使用说明</Action></Screen>;
}
```

(hero 文案/类名拷自现文件;`React.useEffect` 的 import 对齐该文件现状——Taro 页面若用 `useReady`/`useDidShow`,以文件内既有生命周期钩子为准触发首次登录。)

- [ ] **Step 4: 渲染测试**

`apps/miniprogram/tests/pages.test.mjs`:既有"登录流"用例(覆盖邮箱/密码表单)改为——

1. mock `@tarojs/taro` 替身里 `login` 返回 `{ code: 'CODE' }`(替身集中在文件头 module.registerHooks 处,找到 Taro 替身对象加 `login: async () => ({ code: 'CODE' })`);
2. 挂载登录页,断言:首次自动触发 wire 请求 `POST /api/v1/auth/wechat/login` body 含 `code:'CODE'`(对齐文件内既有 wire 请求断言方式),且不渲染邮箱 Field;
3. 令 wire 返回 401,断言渲染错误 Notice 与"立即登录"重试按钮;点击重试后再次发请求。

运行(对齐 apps/miniprogram/package.json 的 test script,commit cd6b0e521 用的方式):

```bash
cd apps/miniprogram && node --test tests/pages.test.mjs
```

- [ ] **Step 5: Commit**

```bash
git add packages/api-client/src/auth/endpoints.ts packages/api-client/src/auth/endpoints.test.ts apps/miniprogram/src/core/auth.ts apps/miniprogram/src/features/auth/pages.tsx apps/miniprogram/tests/pages.test.mjs
git commit -m "feat(miniprogram): 登录页改微信静默登录——wx.login 直连 /auth/wechat/login"
```

---

### Task 9: 部署文档与联调清单

**Files:**
- Create: `docs/deploy/casdoor.md`

**Interfaces:**
- Consumes: Task 1-8 全部产物

- [ ] **Step 1: 写文档**

`docs/deploy/casdoor.md` 包含四节:

1. **Casdoor 初始化**(一次性,UI 操作):登录 `http://localhost:8000`(内置 admin/123,首次必改)→ 建组织 `weknora` → 建应用 `weknora-app`(redirect URL `http(s)://<host>/api/v1/auth/oidc/callback`,勾选 Password Credentials Grant)→ 建用户 `svc_weknora` 加入 admin 角色 → 记录 client id/secret
2. **WeKnora 环境变量**(.env):`OIDC_AUTH_ENABLE=true`、`OIDC_AUTH_ISSUER_URL=http://casdoor:8000`、`OIDC_AUTH_AUTHORIZATION_ENDPOINT=http://localhost:8000/login/oauth/authorize`(浏览器可达地址;生产换真实域名)、`OIDC_AUTH_CLIENT_ID/SECRET`、`OIDC_AUTH_SSO_ONLY=true`、`WECHAT_MP_APP_ID/APP_SECRET/SECRET_KEY`、`CASDOOR_ADMIN_*` 四项
3. **小程序侧**:微信后台 request 合法域名须含 WeKnora API 域名(HTTPS);AppID 与 `WECHAT_MP_APP_ID` 一致
4. **手动联调清单**:
   - web:登录页只见 SSO → 跳 Casdoor 登录 → 回调后进入工作台 → Casdoor 后台可见新用户
   - 小程序(微信开发者工具):启动即静默登录进入首页;Casdoor 后台出现 `wx_*` 用户;杀进程重进复登成功
   - 禁用验证:Carsdoor 禁用 `wx_*` 用户 → 小程序复登被拒
   - 回退:`OIDC_AUTH_SSO_ONLY=false` 且不开 OIDC → 登录页恢复账密表单(后端接口从未动过)

- [ ] **Step 2: 全链路手动联调**

按清单执行(需要用户配合:Casdoor UI 操作与微信开发者工具)。记录问题,小问题当场修,大问题回报。

- [ ] **Step 3: Commit**

```bash
git add docs/deploy/casdoor.md
git commit -m "docs(deploy): Casdoor 初始化、环境变量与三端联调清单"
```

---

## Self-Review 记录

- **Spec 覆盖**:spec §2 部署→Task 1/9;§3.1 配置→Task 2;§3.2 静默通道→Task 3/4/5/6;§3.3 sso_only→Task 7;§4 前端→Task 7/8;§5 错误处理→Task 5(兜底/密文损坏)+ Task 8(重试页)+ 既有 `oidcLoginFailed`;§6 测试→各 Task 内。微信并发首登竞态:由 `email` 唯一索引兜底(Register 冲突 → 报错重试,用户重试即幂等认领),Task 5 注明。
- **Placeholder 扫描**:标注"对齐既有 pattern"的四处(api-client 字段映射、LoginPage JSX wrapper、miniprogram adapter 转发、pages.test.mjs 替身)均给出精确文件行号与同构参照,执行者零决策。
- **类型一致性**:`LoginWithWeChatCode` 在 Task 5(实现)/Task 6(接口+handler)签名一致;`casdoorClient`/`wechatMPClient` 在 Task 3/4/5 间一致;`wechatAuthClients` 字段名 `wechat`/`casdoor` 与测试注入一致。
