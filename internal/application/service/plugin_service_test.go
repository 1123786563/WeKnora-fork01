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
