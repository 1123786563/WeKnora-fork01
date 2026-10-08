package container

// T37 #67 最终修复批次（审查发现 2）：newMobileNotificationProvider 的装配
// 裁决此前零测试——Task 2 把 host 校验防线移到装配层时自认「若遗漏则防线落
// 空」。本文件在装配边界钉住四类行为：
//  1. host gate 对 official（gateway/expo）与企业 APNs/FCM 通道对称生效，
//     命中 loopback/私有/保留主机 fail closed 为 Disabled；
//  2. 空 URL 保留「未配置」语义（unconfigured），与「配置了但被拒」（disabled）
//     区分；
//  3. 未声明 / 非法企业 App id 时不建企业 lane（fail closed）；
//  4. 未知 provider 模式即使配置 URL 也落到空 endpoint；
//  5. FCM 凭据文件 token_uri 回落与 endpoint 共用同一 host gate。
//
// 全部断言走可观察行为（Configured() / Send 错误码），无真实网络：Disabled
// 与空 endpoint provider 不出网，企业 lane 的 resolver 在 devices==nil 时
// 先行失败（InvalidRegistration），恰好证明 lane 已装配。

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	pushnotification "github.com/Tencent/WeKnora/internal/workbench/notification"
	workbenchservice "github.com/Tencent/WeKnora/internal/workbench/service/workbench"
	"github.com/stretchr/testify/require"
)

// resetPushAssemblyEnv 清空全部 push 装配相关 env，避免用例间顺序耦合；
// t.Setenv 保证用例结束自动恢复进程 env。
func resetPushAssemblyEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"MOBILE_NOTIFICATION_PROVIDER", "MOBILE_NOTIFICATION_PROVIDER_URL",
		"MOBILE_NOTIFICATION_ACCESS_TOKEN", "MOBILE_NOTIFICATION_PAYLOAD",
		"MOBILE_ENTERPRISE_APP_ID", "MOBILE_ENTERPRISE_PUSH_PROVIDER",
		"MOBILE_APNS_ENDPOINT", "MOBILE_APNS_TOPIC", "MOBILE_APNS_KEY_PATH",
		"MOBILE_APNS_KEY_ID", "MOBILE_APNS_TEAM_ID",
		"MOBILE_FCM_ENDPOINT", "MOBILE_FCM_PROJECT_ID", "MOBILE_FCM_CREDENTIALS_PATH",
	} {
		t.Setenv(key, "")
	}
}

// sendCodeForApp 通过 AppRouting/单通道 Send 的错误码观察装配结果。
func sendCodeForApp(t *testing.T, provider workbenchservice.NotificationProvider, appID string) string {
	t.Helper()
	err := provider.Send(context.Background(), repository.NotificationDelivery{
		Intent: repository.NotificationIntent{TenantID: 1, OwnerID: "u1", DeviceID: "d1", AppID: appID},
	})
	require.Error(t, err, "probe send must surface a config-class error")
	var providerErr *pushnotification.ProviderError
	require.ErrorAs(t, err, &providerErr)
	return providerErr.Code
}

// configuredOf 经 NotificationProviderConfiguration 可选接口读取 Configured()。
func configuredOf(t *testing.T, provider workbenchservice.NotificationProvider) bool {
	t.Helper()
	configured, ok := provider.(workbenchservice.NotificationProviderConfiguration)
	require.True(t, ok, "assembly providers must expose Configured()")
	return configured.Configured()
}

func writeFcmCredentials(t *testing.T, tokenURI string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	credentials := map[string]any{
		"client_email": "push@proj-1.iam.gserviceaccount.test",
		"private_key":  string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})),
		"token_uri":    tokenURI,
	}
	raw, err := json.Marshal(credentials)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "service-account.json")
	require.NoError(t, os.WriteFile(path, raw, 0o600))
	return path
}

func TestMobileProviderAssemblyOfficialGatewayHostGateFailsClosed(t *testing.T) {
	resetPushAssemblyEnv(t)
	t.Setenv("MOBILE_NOTIFICATION_PROVIDER", "gateway")
	t.Setenv("MOBILE_NOTIFICATION_PROVIDER_URL", "https://10.1.2.3/gateway")
	provider := newMobileNotificationProvider(nil, nil)
	require.False(t, configuredOf(t, provider), "a private gateway endpoint must fail closed")
	require.Equal(t, "mobile_notification_provider_disabled", sendCodeForApp(t, provider, repository.MobileAppIDOfficial))
}

func TestMobileProviderAssemblyOfficialExpoLoopbackGateFailsClosed(t *testing.T) {
	resetPushAssemblyEnv(t)
	t.Setenv("MOBILE_NOTIFICATION_PROVIDER", "expo")
	t.Setenv("MOBILE_NOTIFICATION_PROVIDER_URL", "http://127.0.0.1:2197/push/send")
	provider := newMobileNotificationProvider(nil, nil)
	require.False(t, configuredOf(t, provider), "a loopback expo endpoint must fail closed")
	require.Equal(t, "mobile_notification_provider_disabled", sendCodeForApp(t, provider, repository.MobileAppIDOfficial))
}

func TestMobileProviderAssemblyEmptyEndpointKeepsUnconfiguredSemantics(t *testing.T) {
	resetPushAssemblyEnv(t)
	t.Setenv("MOBILE_NOTIFICATION_PROVIDER", "gateway")
	// 空 URL ≠ 被拒主机：保留既有「未配置」语义（unconfigured），运维可据此
	// 区分「还没配网关」与「网关地址被防线拒绝」。
	provider := newMobileNotificationProvider(nil, nil)
	require.False(t, configuredOf(t, provider))
	_, isHTTP := provider.(*workbenchservice.HTTPNotificationProvider)
	require.True(t, isHTTP, "empty gateway URL stays the unconfigured HTTP provider, not Disabled")
	require.Equal(t, "mobile_notification_provider_unconfigured", sendCodeForApp(t, provider, repository.MobileAppIDOfficial))
}

func TestMobileProviderAssemblyHealthyPublicGatewayIsConfigured(t *testing.T) {
	resetPushAssemblyEnv(t)
	t.Setenv("MOBILE_NOTIFICATION_PROVIDER", "gateway")
	t.Setenv("MOBILE_NOTIFICATION_PROVIDER_URL", "https://gateway.example.com/push")
	provider := newMobileNotificationProvider(nil, nil)
	require.True(t, configuredOf(t, provider), "a public DNS gateway endpoint passes the host gate")
}

func TestMobileProviderAssemblyUnknownProviderModeFallsBackToEmptyEndpoint(t *testing.T) {
	resetPushAssemblyEnv(t)
	t.Setenv("MOBILE_NOTIFICATION_PROVIDER", "carrier-pigeon")
	t.Setenv("MOBILE_NOTIFICATION_PROVIDER_URL", "https://gateway.example.com/push")
	provider := newMobileNotificationProvider(nil, nil)
	require.False(t, configuredOf(t, provider), "unknown provider mode ignores the configured URL")
	_, isHTTP := provider.(*workbenchservice.HTTPNotificationProvider)
	require.True(t, isHTTP)
	require.Equal(t, "mobile_notification_provider_unconfigured", sendCodeForApp(t, provider, repository.MobileAppIDOfficial))
}

func TestMobileProviderAssemblyUndeclaredEnterpriseAppHasNoEnterpriseLane(t *testing.T) {
	resetPushAssemblyEnv(t)
	t.Setenv("MOBILE_NOTIFICATION_PROVIDER", "disabled")
	// 未声明企业 App：返回值就是 official 通道本身，无 AppRouting 聚合。
	provider := newMobileNotificationProvider(nil, nil)
	_, isRouting := provider.(*workbenchservice.AppRoutingNotificationProvider)
	require.False(t, isRouting, "no enterprise lane without MOBILE_ENTERPRISE_APP_ID")
	_, isDisabled := provider.(*workbenchservice.DisabledNotificationProvider)
	require.True(t, isDisabled)
}

func TestMobileProviderAssemblyIllegalEnterpriseAppFailsClosed(t *testing.T) {
	for _, badAppID := range []string{"official", "enterprise:Bad_App", "not-an-app-id"} {
		t.Run(badAppID, func(t *testing.T) {
			resetPushAssemblyEnv(t)
			t.Setenv("MOBILE_NOTIFICATION_PROVIDER", "disabled")
			t.Setenv("MOBILE_ENTERPRISE_APP_ID", badAppID)
			t.Setenv("MOBILE_ENTERPRISE_PUSH_PROVIDER", "apns")
			provider := newMobileNotificationProvider(nil, nil)
			_, isRouting := provider.(*workbenchservice.AppRoutingNotificationProvider)
			require.False(t, isRouting, "an invalid enterprise app id must not open an enterprise lane")
		})
	}
}

func TestMobileProviderAssemblyEnterpriseApnsPrivateEndpointFailsClosed(t *testing.T) {
	resetPushAssemblyEnv(t)
	t.Setenv("MOBILE_NOTIFICATION_PROVIDER", "gateway")
	t.Setenv("MOBILE_NOTIFICATION_PROVIDER_URL", "https://gateway.example.com/push")
	t.Setenv("MOBILE_ENTERPRISE_APP_ID", "enterprise:acme")
	t.Setenv("MOBILE_ENTERPRISE_PUSH_PROVIDER", "apns")
	t.Setenv("MOBILE_APNS_ENDPOINT", "https://192.168.10.4/3/device")
	provider := newMobileNotificationProvider(nil, nil)
	require.True(t, configuredOf(t, provider), "the official lane stays alive under AppRouting OR semantics")
	require.Equal(t, "mobile_notification_provider_disabled",
		sendCodeForApp(t, provider, "enterprise:acme"), "a private APNs endpoint must not wire the enterprise lane")
}

func TestMobileProviderAssemblyEnterpriseFcmTokenURILoopbackFailsClosed(t *testing.T) {
	resetPushAssemblyEnv(t)
	t.Setenv("MOBILE_NOTIFICATION_PROVIDER", "gateway")
	t.Setenv("MOBILE_NOTIFICATION_PROVIDER_URL", "https://gateway.example.com/push")
	t.Setenv("MOBILE_ENTERPRISE_APP_ID", "enterprise:acme")
	t.Setenv("MOBILE_ENTERPRISE_PUSH_PROVIDER", "fcm")
	t.Setenv("MOBILE_FCM_CREDENTIALS_PATH", writeFcmCredentials(t, "http://127.0.0.1:8899/token"))
	provider := newMobileNotificationProvider(nil, nil)
	require.Equal(t, "mobile_notification_provider_disabled",
		sendCodeForApp(t, provider, "enterprise:acme"),
		"a credential-file token_uri pointing at loopback must fail closed at assembly")
}

func TestMobileProviderAssemblyEnterpriseFcmPublicTokenURIWiresLane(t *testing.T) {
	resetPushAssemblyEnv(t)
	t.Setenv("MOBILE_NOTIFICATION_PROVIDER", "gateway")
	t.Setenv("MOBILE_NOTIFICATION_PROVIDER_URL", "https://gateway.example.com/push")
	t.Setenv("MOBILE_ENTERPRISE_APP_ID", "enterprise:acme")
	t.Setenv("MOBILE_ENTERPRISE_PUSH_PROVIDER", "fcm")
	t.Setenv("MOBILE_FCM_ENDPOINT", "https://fcm.example-enterprise.net")
	t.Setenv("MOBILE_FCM_CREDENTIALS_PATH", writeFcmCredentials(t, "https://oauth2.example-enterprise.net/token"))
	provider := newMobileNotificationProvider(nil, nil)
	require.Equal(t, "InvalidRegistration",
		sendCodeForApp(t, provider, "enterprise:acme"),
		"a public token_uri wires the lane; the resolver probe fails first (devices store absent) proving wiring happened")
}
