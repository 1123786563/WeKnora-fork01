package sandbox

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// T25 (#55)：沙箱 Shell 环境的函数级注入纪律——withWorkspaceEnvDefaults 是
// exec env 的唯一函数级注入点（session_manager.go:1492），它只允许补两个
// WEKNORA_ 工作区路径键，永不新增任何其它键（凭据隔离的白名单面）。
func TestWithWorkspaceEnvDefaultsStampsOnlyWorkspacePaths(t *testing.T) {
	got := withWorkspaceEnvDefaults(nil)
	require.Len(t, got, 2)
	require.Equal(t, SessionOutputRoot, got[skillOutputEnvVar])
	require.Equal(t, SessionInputRoot, got[sessionInputEnvVar])
}

func TestWithWorkspaceEnvDefaultsPreservesConfiguredValuesAndAddsNothingElse(t *testing.T) {
	input := map[string]string{
		skillOutputEnvVar: "/workspace/custom-out",
		"PATH":            "/usr/local/bin:/usr/bin",
		"EXISTING_FLAG":   "1",
	}
	configuredKeys := make(map[string]struct{}, len(input))
	for key := range input {
		configuredKeys[key] = struct{}{}
	}
	got := withWorkspaceEnvDefaults(input)
	require.Equal(t, "/workspace/custom-out", got[skillOutputEnvVar], "a configured workspace path is never overridden")
	require.Equal(t, SessionInputRoot, got[sessionInputEnvVar])
	require.Equal(t, "/usr/local/bin:/usr/bin", got["PATH"])
	require.Equal(t, "1", got["EXISTING_FLAG"])
	addedKeys := make(map[string]struct{})
	for key := range got {
		if _, configured := configuredKeys[key]; !configured {
			addedKeys[key] = struct{}{}
		}
	}
	require.Equal(t, map[string]struct{}{sessionInputEnvVar: {}}, addedKeys, "only the missing approved workspace path may be added (T25)")
	require.Contains(t, configuredKeys, skillOutputEnvVar, "the output key was configured and must not be counted as added")
}
