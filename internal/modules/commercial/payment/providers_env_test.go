package payment

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeRSAKeyPEMs(t *testing.T, dir, name string) (pubPath, privPath string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pubPath = filepath.Join(dir, name+"-pub.pem")
	if err := os.WriteFile(pubPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	privPath = filepath.Join(dir, name+"-priv.pem")
	if err := os.WriteFile(privPath, pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key),
	}), 0o600); err != nil {
		t.Fatal(err)
	}
	return pubPath, privPath
}

func envMap(pairs ...string) func(string) string {
	m := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i]] = pairs[i+1]
	}
	return func(k string) string { return m[k] }
}

func TestProvidersFromEnvWithoutChannelsIsEmpty(t *testing.T) {
	providers, err := providersFromEnv(envMap())
	if err != nil {
		t.Fatalf("empty env must stay legal: %v", err)
	}
	if len(providers) != 0 {
		t.Fatalf("expected no providers, got %v", providers)
	}
}

func TestProvidersFromEnvBuildsAlipay(t *testing.T) {
	pub, priv := writeRSAKeyPEMs(t, t.TempDir(), "alipay")
	providers, err := providersFromEnv(envMap(
		"WEKNORA_ALIPAY_APP_ID", "2021000000000001",
		"WEKNORA_ALIPAY_SELLER_ID", "2088000000000001",
		"WEKNORA_ALIPAY_PUBLIC_KEY_PATH", pub,
		"WEKNORA_ALIPAY_MERCHANT_KEY_PATH", priv,
		"WEKNORA_ALIPAY_NOTIFY_URL", "https://edge.example.com/api/v1/commercial/callbacks/alipay",
	))
	if err != nil {
		t.Fatalf("fully configured alipay rejected: %v", err)
	}
	if _, ok := providers[ProviderAlipay].(*AlipayProvider); !ok {
		t.Fatalf("expected an *AlipayProvider, got %T", providers[ProviderAlipay])
	}
	if len(providers) != 1 {
		t.Fatalf("expected exactly one provider, got %d", len(providers))
	}
}

func TestProvidersFromEnvRejectsPartialAlipay(t *testing.T) {
	_, priv := writeRSAKeyPEMs(t, t.TempDir(), "alipay")
	_, err := providersFromEnv(envMap(
		"WEKNORA_ALIPAY_APP_ID", "2021000000000001",
		"WEKNORA_ALIPAY_MERCHANT_KEY_PATH", priv,
	))
	if err == nil || !strings.Contains(err.Error(), "WEKNORA_ALIPAY_SELLER_ID") {
		t.Fatalf("partial alipay config must name the missing variable, got: %v", err)
	}
}

func TestProvidersFromEnvBuildsWechat(t *testing.T) {
	dir := t.TempDir()
	pub, priv := writeRSAKeyPEMs(t, dir, "wechat")
	apiv3 := filepath.Join(dir, "apiv3.key")
	if err := os.WriteFile(apiv3, []byte("0123456789abcdef0123456789abcdef"), 0o600); err != nil {
		t.Fatal(err)
	}
	providers, err := providersFromEnv(envMap(
		"WEKNORA_WECHAT_APP_ID", "wx-test-app",
		"WEKNORA_WECHAT_MCH_ID", "1900000001",
		"WEKNORA_WECHAT_MCH_SERIAL", "SERIAL-1",
		"WEKNORA_WECHAT_MCH_KEY_PATH", priv,
		"WEKNORA_WECHAT_APIV3_KEY_PATH", apiv3,
		"WEKNORA_WECHAT_PLATFORM_CERTS", "PLAT-SERIAL-1:"+pub,
	))
	if err != nil {
		t.Fatalf("fully configured wechat rejected: %v", err)
	}
	if _, ok := providers[ProviderWechat].(*WechatProvider); !ok {
		t.Fatalf("expected a *WechatProvider, got %T", providers[ProviderWechat])
	}
}

func TestProvidersFromEnvRejectsPartialWechat(t *testing.T) {
	dir := t.TempDir()
	_, priv := writeRSAKeyPEMs(t, dir, "wechat")
	apiv3 := filepath.Join(dir, "apiv3.key")
	if err := os.WriteFile(apiv3, []byte("0123456789abcdef0123456789abcdef"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := providersFromEnv(envMap(
		"WEKNORA_WECHAT_APP_ID", "wx-test-app",
		"WEKNORA_WECHAT_MCH_ID", "1900000001",
		"WEKNORA_WECHAT_MCH_SERIAL", "SERIAL-1",
		"WEKNORA_WECHAT_MCH_KEY_PATH", priv,
		"WEKNORA_WECHAT_APIV3_KEY_PATH", apiv3,
	))
	if err == nil || !strings.Contains(err.Error(), "WEKNORA_WECHAT_PLATFORM_CERTS") {
		t.Fatalf("partial wechat config must name the missing variable, got: %v", err)
	}
}
