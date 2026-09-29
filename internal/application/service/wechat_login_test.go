package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

const wechatTestOpenID = "oABCDEF1234567890abcdef1234"

// wechatFake stubs the code2session client.
type wechatFake struct{ openid string }

func (w wechatFake) Code2Session(context.Context, string) (string, error) { return w.openid, nil }

// casdoorFake stubs the Casdoor admin client used by the silent-login channel.
type casdoorFake struct {
	ropcCalls  int
	ropcFailN  int // 前 N 次 ROPC 失败
	ensureCall int
	setPwdCall int
	ropcToken  string
	lastPwd    string // 最近一次 ROPC / SetPassword 携带的密码
}

func (c *casdoorFake) PasswordToken(_ context.Context, _, password string) (string, error) {
	c.ropcCalls++
	c.lastPwd = password
	if c.ropcCalls <= c.ropcFailN {
		return "", errors.New("casdoor: invalid password")
	}
	return c.ropcToken, nil
}

func (c *casdoorFake) EnsureUser(context.Context, string, string, string) error {
	c.ensureCall++
	return nil
}

func (c *casdoorFake) SetPassword(_ context.Context, _ string, password string) error {
	c.setPwdCall++
	c.lastPwd = password
	return nil
}

type wechatLoginRepo struct {
	interfaces.UserRepository
	byEmail map[string]*types.User
	updated []*types.User
}

// GetUserByEmail mirrors the not-found shape isUserLookupNotFound recognises
// (error-text match, same contract the gorm repo relies on).
func (r *wechatLoginRepo) GetUserByEmail(_ context.Context, email string) (*types.User, error) {
	if u, ok := r.byEmail[email]; ok {
		return u, nil
	}
	return nil, errors.New("user not found")
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

func (r *wechatLoginRepo) GetUserByUsername(context.Context, string) (*types.User, error) {
	return nil, errors.New("user not found")
}

// wechatTokenRepo accepts token persistence without a DB.
type wechatTokenRepo struct {
	interfaces.AuthTokenRepository
}

func (r *wechatTokenRepo) CreateToken(context.Context, *types.AuthToken) error { return nil }

// newWechatLoginService assembles a userService wired for the silent-login
// channel: stubbed wechat + casdoor clients, an in-memory user repo, and a
// mock userinfo endpoint that first-login reaches with the ROPC token. The
// userinfo payload deliberately omits email so the wx_…@wechat.local
// fallback is exercised.
func newWechatLoginService(t *testing.T, repo *wechatLoginRepo, cd *casdoorFake) *userService {
	t.Helper()
	withOIDCSSRFWhitelist(t, "127.0.0.1")
	t.Setenv("JWT_SECRET", "wechat-login-test-secret")

	// All OIDC endpoints point at the whitelisted local mock so SSRF
	// validation passes without external DNS; only /userinfo is ever called
	// by the silent-login flow (no id_token, so jwks stays unused).
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/userinfo" {
			t.Errorf("unexpected OIDC endpoint hit: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+cd.ropcToken {
			t.Errorf("userinfo Authorization = %q, want Bearer %s", got, cd.ropcToken)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"sub":  wechatTestOpenID,
			"name": "WeChat User",
		})
	}))
	t.Cleanup(idp.Close)

	return &userService{
		userRepo:      repo,
		tokenRepo:     &wechatTokenRepo{},
		tenantService: &provisioningTenantService{},
		config: &config.Config{
			OIDCAuth: &config.OIDCAuthConfig{
				Enable:                true,
				IssuerURL:             idp.URL,
				AuthorizationEndpoint: idp.URL + "/authorize",
				TokenEndpoint:         idp.URL + "/token",
				UserInfoEndpoint:      idp.URL + "/userinfo",
				JwksURI:               idp.URL + "/keys",
				ClientID:              "weknora-client",
				ClientSecret:          "weknora-secret",
				UserInfoMapping:       &config.OIDCUserInfoMapping{Username: "name", Email: "email"},
			},
			WechatMP: &config.WechatMPConfig{
				AppID:     "wx-app",
				AppSecret: "wx-secret",
				SecretKey: "unit-test-key",
			},
			CasdoorAdmin: &config.CasdoorAdminConfig{
				BaseURL:       "http://casdoor.local",
				OrgName:       "weknora",
				AdminUsername: "svc",
				AdminPassword: "svc-pw",
			},
		},
		wechat: &wechatAuthClients{wechat: wechatFake{openid: wechatTestOpenID}, casdoor: cd},
	}
}

func TestWeChatFirstLoginProvisions(t *testing.T) {
	repo := &wechatLoginRepo{byEmail: map[string]*types.User{}}
	cd := &casdoorFake{ropcToken: "casdoor-tok"}
	svc := newWechatLoginService(t, repo, cd)

	resp, err := svc.LoginWithWeChatCode(context.Background(), "CODE", types.TenantProvisioningTenantless)
	if err != nil {
		t.Fatalf("first login failed: %v", err)
	}
	if resp.Token == "" || resp.RefreshToken == "" {
		t.Fatalf("expected tokens, got %+v", resp)
	}
	if cd.ensureCall != 1 || cd.ropcCalls != 1 {
		t.Fatalf("expected casdoor ensure+ropc exactly once, got %+v", cd)
	}
	email := "wx_" + wechatTestOpenID + "@wechat.local"
	user := repo.byEmail[email]
	if user == nil {
		t.Fatal("provisioned user missing")
	}
	if user.Preferences.WeChatServiceSecret == "" {
		t.Fatal("service secret ciphertext not persisted")
	}
	if user.Preferences.OidcOnlyLogin == nil || !*user.Preferences.OidcOnlyLogin {
		t.Fatal("expected OidcOnlyLogin=true")
	}
	got, err := decryptWeChatServiceSecret("unit-test-key", user.Preferences.WeChatServiceSecret)
	if err != nil || got != cd.lastPwd {
		t.Fatalf("persisted ciphertext decrypts to %q (err=%v), want ROPC password %q", got, err, cd.lastPwd)
	}
}

func TestWeChatRepeatLogin(t *testing.T) {
	cipher, err := encryptWeChatServiceSecret("unit-test-key", "known-svc-pwd")
	if err != nil {
		t.Fatal(err)
	}
	email := "wx_" + wechatTestOpenID + "@wechat.local"
	repo := &wechatLoginRepo{byEmail: map[string]*types.User{
		email: {
			ID: "u1", Username: "wx_oABCDEF1", Email: email,
			IsActive:    true,
			Preferences: types.UserPreferences{WeChatServiceSecret: cipher},
		},
	}}
	cd := &casdoorFake{ropcToken: "casdoor-tok"}
	svc := newWechatLoginService(t, repo, cd)

	resp, err := svc.LoginWithWeChatCode(context.Background(), "CODE", types.TenantProvisioningTenantless)
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
	if cd.lastPwd != "known-svc-pwd" {
		t.Fatalf("ropc used password %q, want decrypted known-svc-pwd", cd.lastPwd)
	}
}

func TestWeChatRepeatLoginResetsForgottenServicePassword(t *testing.T) {
	cipher, _ := encryptWeChatServiceSecret("unit-test-key", "stale-pwd")
	email := "wx_" + wechatTestOpenID + "@wechat.local"
	repo := &wechatLoginRepo{byEmail: map[string]*types.User{
		email: {
			ID: "u1", Username: "wx_oABCDEF1", Email: email,
			IsActive:    true,
			Preferences: types.UserPreferences{WeChatServiceSecret: cipher},
		},
	}}
	cd := &casdoorFake{ropcToken: "casdoor-tok", ropcFailN: 1}
	svc := newWechatLoginService(t, repo, cd)

	resp, err := svc.LoginWithWeChatCode(context.Background(), "CODE", types.TenantProvisioningTenantless)
	if err != nil {
		t.Fatalf("fallback reset failed: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("expected token after fallback reset")
	}
	if cd.setPwdCall != 1 {
		t.Fatalf("expected set-password fallback, got %+v", cd)
	}
	if len(repo.updated) == 0 {
		t.Fatal("expected refreshed ciphertext persisted")
	}
	refreshed := repo.updated[len(repo.updated)-1].Preferences.WeChatServiceSecret
	got, err := decryptWeChatServiceSecret("unit-test-key", refreshed)
	if err != nil || got != cd.lastPwd || got == "stale-pwd" {
		t.Fatalf("refreshed ciphertext decrypts to %q (err=%v), want the new retry password", got, err)
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
	u, e := wechatAccountIdentifiers(wechatTestOpenID)
	if u != "wx_"+wechatTestOpenID[:8] || e != "wx_"+wechatTestOpenID+"@wechat.local" {
		t.Fatalf("got %q %q", u, e)
	}
}
