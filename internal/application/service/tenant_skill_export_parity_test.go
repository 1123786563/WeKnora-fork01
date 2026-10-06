package service

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/stretchr/testify/require"
)

// tenant_skill_export_parity_test.go —— Pass B 25b 导出化差分装置（计划 §T3/§7.2）。
// 逐例锚定：宿主旧名（残差转发）与新包导出名行为完全一致。remove_at: ib2。
// 真身归位本包后两侧均为同包符号（旧名经 tenant_skill_passb_host_shims.go 转发）。

func TestParitySkillSnapshotNamePrefix(t *testing.T) {
	cases := []struct {
		name     string
		tenantID uint64
		configID string
	}{
		{"plain", 7, "cfg-1"},
		{"empty configID", 7, ""},
		{"uppercase and hyphens", 42, "CFG-abc-123-XYZ"},
		{"long hyphenated id", 1, "a1b2c3d4-e5f6-4789-a012-b3c4d5e6f7g8-hijklmn"},
		{"zero tenant", 0, "cfg"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t,
				SkillSnapshotNamePrefix(tc.tenantID, tc.configID),
				skillSnapshotNamePrefix(tc.tenantID, tc.configID))
			require.Equal(t,
				"weknora-sk-t"+itoaDecimal(tc.tenantID)+"-"+compactForParity(tc.configID),
				SkillSnapshotNamePrefix(tc.tenantID, tc.configID),
				"weknora-sk-t<tenant>-<compact config id> 命名是 Cube/E2B/Docker 跨账号 listing 的外部契约锚点")
		})
	}
}

func itoaDecimal(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func compactForParity(id string) string {
	out := make([]byte, 0, len(id))
	for _, r := range id {
		if r == '-' || r == ' ' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		out = append(out, byte(r))
	}
	return string(out)
}

func TestParitySnapshotsNotFromOtherConfig(t *testing.T) {
	listed := []sandbox.RemoteSnapshotRef{
		{ID: "weknora-skill/weknora-sk-t7-cfg1-g1", Names: []string{"weknora-skill/weknora-sk-t7-cfg1-g1"}},
		{ID: "weknora-skill/weknora-sk-t7-cfg2-g1", Names: []string{"weknora-skill/weknora-sk-t7-cfg2-g1"}},
		{ID: "snap-ours", Names: []string{"weknora-sk-t7-cfg1-g1"}},
		{ID: "snap-foreign", Names: []string{"weknora-sk-t8-cfg1-g1"}},
		{ID: "snap-legacy", Names: []string{"weknora-sk-cfg1-g2"}},
		{ID: "snap-plain"},
	}
	prefixes := []string{
		"weknora-sk-t7-cfg1", "", "weknora-sk-t7-cfg2", "weknora-sk-t8-cfg3",
	}
	for _, prefix := range prefixes {
		require.Equal(t,
			SnapshotsNotFromOtherConfig(listed, prefix),
			snapshotsNotFromOtherConfig(listed, prefix),
			"prefix %q", prefix)
	}
}

func TestParityMatchSnapshotByName(t *testing.T) {
	listed := []sandbox.RemoteSnapshotRef{
		{ID: "snap-1", Names: []string{"weknora-sk-t7-cfg1-g1"}},
		{ID: "repo/weknora-sk-t7-cfg1-g2", Names: []string{"repo/weknora-sk-t7-cfg1-g2"}},
		{ID: "echoed", Names: []string{"planned-echo"}},
		{ID: "  ", Names: []string{" "}},
	}
	names := []string{
		"weknora-sk-t7-cfg1-g1", "weknora-sk-t7-cfg1-g2", "planned-echo",
		"repo/weknora-sk-t7-cfg1-g2", "", "  ", "missing-name",
	}
	for _, name := range names {
		require.Equal(t,
			MatchSnapshotByName(listed, name),
			matchSnapshotByName(listed, name),
			"planned name %q", name)
	}
}

func TestParityValidateUserEnvName(t *testing.T) {
	cases := []string{
		"TAVILY_API_KEY",    // 合法
		"_PRIVATE",          // 合法（下划线开头）
		"",                  // 非法：空
		"path_prefix",       // 非法：小写
		"HAS SPACE",         // 非法：空格
		"1LEADING_DIGIT",    // 非法：数字开头
		"PATH",              // 字面保留名
		"LD_PRELOAD",        // 字面保留名
		"WEKNORA_SKILL_DIR", // WEKNORA_SKILL_ 前缀
		"SESSION_INPUT_DIR", // 注入名（无前缀；经残差 init 的 RegisterReservedEnvNames 通道装载）
		"WEKNORA_API_KEY",   // 非 sandbox 注入，合法
	}
	for _, name := range cases {
		errOld := validateUserEnvName(name)
		errNew := ValidateUserEnvName(name)
		if errOld == nil {
			require.NoError(t, errNew, "name %q", name)
		} else {
			require.Error(t, errNew, "name %q", name)
			require.Equal(t, errOld.Error(), errNew.Error(), "name %q", name)
		}
	}
}

// UniqueNonEmptyStrings 注入位语义：跳过空串再去重（conversation 真源，即残差
// 构造器绑定给 HostAdapters.UniqueNonEmptyStrings 的函数）。对照组：随迁的
// uniqueStrings 对同输入返回 ["", "a"]（保留空串）——两者语义不同，注入位不得
// 被其替代（计划 §2.3，防回归混淆）。
func TestParityUniqueNonEmptyStringsInjection(t *testing.T) {
	got := uniqueNonEmptyStrings([]string{"", "a", "", "a"})
	require.Equal(t, []string{"a"}, got)
	require.NotEqual(t, []string{"", "a"}, got,
		"本面 uniqueStrings 保留空串：注入位若被其替代，此处即回归")
}
