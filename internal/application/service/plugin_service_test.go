package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestPluginPreviewTTLCappedAtMax（跨任务转交 T01-R3-F1）：PLUGIN_PREVIEW_TTL
// 对空串、不可解析、非正值回退默认，但缺少上限钳制——合法可解析的超大正值
// （如 8760h≈1 年）会原样生效，preview 近一年可确认且 DeleteExpiredPreviews
// 长期不回收，违背该配置点「misconfigured environment degrades to the
// spec'd bound」的误配安全语义。超大/超限值必须钳制到 maxPluginPreviewTTL。
func TestPluginPreviewTTLCappedAtMax(t *testing.T) {
	cases := []struct {
		raw      string
		expected time.Duration
	}{
		{"", defaultPluginPreviewTTL},
		{"garbage", defaultPluginPreviewTTL},
		{"-5m", defaultPluginPreviewTTL},
		{"30m", 30 * time.Minute},
		{maxPluginPreviewTTL.String(), maxPluginPreviewTTL},
		{"48h", maxPluginPreviewTTL},
		{"8760h", maxPluginPreviewTTL},
	}
	for _, tc := range cases {
		t.Setenv("PLUGIN_PREVIEW_TTL", tc.raw)
		require.Equalf(t, tc.expected, pluginPreviewTTL(), "PLUGIN_PREVIEW_TTL=%q", tc.raw)
	}
}

// TestPluginPreviewTTLFlooredAtMin（OCR 一轮 F1）：TTL 小于 CreatePreview
// 往返耗时（如 1ms）时，刚写入的 preview 行被同一请求内的
// DeleteExpiredPreviews 删除而响应仍返回 PreviewID（后续 confirm 必 0 行）。
// 与 maxPluginPreviewTTL 的「误配降级到规范界」对称，过小正值必须钳制到
// 1 分钟下限。
func TestPluginPreviewTTLFlooredAtMin(t *testing.T) {
	cases := []struct {
		raw      string
		expected time.Duration
	}{
		{"1ms", time.Minute},
		{"30s", time.Minute},
		{"90s", 90 * time.Second},
	}
	for _, tc := range cases {
		t.Setenv("PLUGIN_PREVIEW_TTL", tc.raw)
		require.Equalf(t, tc.expected, pluginPreviewTTL(), "PLUGIN_PREVIEW_TTL=%q", tc.raw)
	}
}

// TestValidateManifestURLInputRejectsCredentialBearingAndSchemeless（OCR
// 一轮 F7/F6）：进入 SSRF 判定与抓取前的输入预检——(a) url.Parse 失败的
// *url.Error 逐字嵌入完整 URL，userinfo+畸形 URL 会把凭据带进 400 响应体
// （评审实测 supersecret 泄漏），必须以固定消息拒绝且不回显 URL；(b)
// userinfo 一律静默拒绝；(c) 空/非 http(s) scheme（如裸
// "example.com/manifest.json"——ValidateURLForSSRF 会自动补 https 放行，
// 而 fetchLimited 用原始 URL 必失败误吞成 503）必须落 400 输入错误。
func TestValidateManifestURLInputRejectsCredentialBearingAndSchemeless(t *testing.T) {
	for raw, note := range map[string]string{
		"https://user:supersecret@exa[mple/manifest.json": "malformed + userinfo must not echo the URL or credentials",
		"https://user:secret@example.com/manifest.json":   "userinfo must be silently rejected without echoing the URL",
		"example.com/manifest.json":                       "schemeless URL is a deterministic input error, not a fetch fault",
		"ftp://example.com/manifest.json":                 "non-http(s) scheme must be rejected up front",
	} {
		err := validateManifestURLInput(raw)
		require.Errorf(t, err, "%s: %q", note, raw)
		require.ErrorIs(t, err, ErrManifestURLRejected)
		require.NotContains(t, err.Error(), "supersecret")
		require.NotContains(t, err.Error(), "secret@")
		require.NotContains(t, err.Error(), "example.com/manifest.json",
			"the rejection message must not echo the raw URL")
	}
	// 合法输入通过预检（SSRF/抓取核验在其后，不在此测）。
	require.NoError(t, validateManifestURLInput("https://plugins.example.com/manifest.json"))
}
