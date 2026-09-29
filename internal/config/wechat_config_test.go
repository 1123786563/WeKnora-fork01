package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestApplyWechatEnvOverrides pins the env contract for the mini-program
// silent-login channel and the Casdoor admin API channel: every field can be
// sourced from WECHAT_MP_* / CASDOOR_ADMIN_* env vars on top of yaml.
func TestApplyWechatEnvOverrides(t *testing.T) {
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
}

// TestApplyWechatEnvOverridesMaterializesNilSections: a config without the
// wechat_mp / casdoor_admin yaml sections still gets empty structs, so later
// nil-dereference accessors keep working.
func TestApplyWechatEnvOverridesMaterializesNilSections(t *testing.T) {
	for _, key := range []string{
		"WECHAT_MP_APP_ID", "WECHAT_MP_APP_SECRET", "WECHAT_MP_SECRET_KEY",
		"CASDOOR_ADMIN_BASE_URL", "CASDOOR_ADMIN_ORG_NAME",
		"CASDOOR_ADMIN_USERNAME", "CASDOOR_ADMIN_PASSWORD",
	} {
		t.Setenv(key, "")
	}
	cfg := &Config{}
	applyWechatEnvOverrides(cfg)
	if cfg.WechatMP == nil || cfg.CasdoorAdmin == nil {
		t.Fatalf("expected nil wechat_mp / casdoor_admin sections to be materialized: %+v", cfg)
	}
	if cfg.WechatMP.AppID != "" || cfg.CasdoorAdmin.BaseURL != "" {
		t.Fatalf("unset env must leave fields empty: %+v %+v", cfg.WechatMP, cfg.CasdoorAdmin)
	}
}

func TestApplyOIDCEnvOverridesSSOOnly(t *testing.T) {
	t.Run("env true enables sso_only", func(t *testing.T) {
		t.Setenv("OIDC_AUTH_SSO_ONLY", "true")
		cfg := &Config{}
		applyOIDCEnvOverrides(cfg)
		if !cfg.OIDCAuth.SSOOnly {
			t.Fatal("OIDC_AUTH_SSO_ONLY=true should set OIDCAuth.SSOOnly")
		}
	})
	t.Run("env false overrides yaml true", func(t *testing.T) {
		t.Setenv("OIDC_AUTH_SSO_ONLY", "false")
		cfg := &Config{OIDCAuth: &OIDCAuthConfig{SSOOnly: true}}
		applyOIDCEnvOverrides(cfg)
		if cfg.OIDCAuth.SSOOnly {
			t.Fatal("OIDC_AUTH_SSO_ONLY=false should clear OIDCAuth.SSOOnly")
		}
	})
	t.Run("unset leaves yaml value", func(t *testing.T) {
		t.Setenv("OIDC_AUTH_SSO_ONLY", "")
		cfg := &Config{OIDCAuth: &OIDCAuthConfig{SSOOnly: true}}
		applyOIDCEnvOverrides(cfg)
		if !cfg.OIDCAuth.SSOOnly {
			t.Fatal("empty env should leave the yaml sso_only value untouched")
		}
	})
}

// TestWechatAndCasdoorYamlKeys pins the yaml key names (sso_only / wechat_mp /
// casdoor_admin and their fields) so the compose-file and deployment docs can
// rely on them.
func TestWechatAndCasdoorYamlKeys(t *testing.T) {
	const raw = `
oidc_auth:
  sso_only: true
wechat_mp:
  app_id: wx123
  app_secret: s3cret
  secret_key: k
casdoor_admin:
  base_url: http://casdoor:8000
  org_name: weknora
  admin_username: svc_weknora
  admin_password: pw
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.OIDCAuth == nil || !cfg.OIDCAuth.SSOOnly {
		t.Fatalf("oidc_auth.sso_only not parsed: %+v", cfg.OIDCAuth)
	}
	if cfg.WechatMP == nil || cfg.WechatMP.AppID != "wx123" || cfg.WechatMP.AppSecret != "s3cret" || cfg.WechatMP.SecretKey != "k" {
		t.Fatalf("wechat_mp not parsed: %+v", cfg.WechatMP)
	}
	if cfg.CasdoorAdmin == nil || cfg.CasdoorAdmin.BaseURL != "http://casdoor:8000" ||
		cfg.CasdoorAdmin.OrgName != "weknora" || cfg.CasdoorAdmin.AdminUsername != "svc_weknora" ||
		cfg.CasdoorAdmin.AdminPassword != "pw" {
		t.Fatalf("casdoor_admin not parsed: %+v", cfg.CasdoorAdmin)
	}
}

// TestValidateConfigWechatChannelCompleteness pins the startup rule for the
// mini-program silent-login channel: a partially filled wechat_mp section
// fails the boot listing every missing field, sso_only deployments skip the
// check (they never serve the channel), and a fully configured channel — or
// no configuration at all — passes.
func TestValidateConfigWechatChannelCompleteness(t *testing.T) {
	fullyConfigured := func() *Config {
		return &Config{
			WechatMP: &WechatMPConfig{AppID: "wx123", AppSecret: "s3cret", SecretKey: "k"},
			CasdoorAdmin: &CasdoorAdminConfig{
				BaseURL: "http://casdoor:8000", OrgName: "weknora",
				AdminUsername: "svc", AdminPassword: "pw",
			},
		}
	}

	t.Run("fully configured channel passes", func(t *testing.T) {
		if err := ValidateConfig(fullyConfigured()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("empty sections pass (channel off)", func(t *testing.T) {
		cfg := fullyConfigured()
		cfg.WechatMP = &WechatMPConfig{}
		cfg.CasdoorAdmin = &CasdoorAdminConfig{}
		if err := ValidateConfig(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("nil casdoor_admin still counted as missing fields", func(t *testing.T) {
		err := ValidateConfig(&Config{WechatMP: &WechatMPConfig{AppSecret: "s3cret"}})
		if err == nil {
			t.Fatal("partial channel config must fail startup validation")
		}
		for _, want := range []string{
			"wechat_mp.app_id", "wechat_mp.secret_key",
			"casdoor_admin.base_url", "casdoor_admin.org_name",
			"casdoor_admin.admin_username", "casdoor_admin.admin_password",
		} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error should mention %q, got: %v", want, err)
			}
		}
	})

	t.Run("sso_only deployment skips the channel check", func(t *testing.T) {
		cfg := &Config{
			// Enable forces the OIDC block to fire (missing client creds),
			// proving the wechat skip is what keeps the error list clean.
			OIDCAuth: &OIDCAuthConfig{Enable: true, SSOOnly: true},
			WechatMP: &WechatMPConfig{AppID: "wx123"},
		}
		err := ValidateConfig(cfg)
		if err == nil {
			t.Fatal("oidc_auth validation should still fire when enabled")
		}
		if strings.Contains(err.Error(), "wechat_mp") || strings.Contains(err.Error(), "casdoor_admin") {
			t.Fatalf("sso_only must skip the wechat channel check, got: %v", err)
		}
	})
}
