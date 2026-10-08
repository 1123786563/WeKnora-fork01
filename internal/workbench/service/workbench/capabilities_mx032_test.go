package workbench

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// MX-032 frozen 场景：remote-without-cancel-evidence。
// remote 部署✓健康✓权限✓但缺取消证据 → unavailable(missing_cancel_evidence)。
func TestMX032ProfileWithMissingEvidence(t *testing.T) {
	service := NewCapabilityService()

	state, reason := service.EvaluateCapability(CapabilityInput{
		Profile:        ProfileRemote,
		DeployEnabled:  true,
		ServiceHealthy: true,
		UserPermitted:  true,
		Evidence:       CapabilityEvidence{Profile: ProfileRemote, EvidenceKeys: []string{"remote:paseo-live-probe"}},
	})
	require.Equal(t, "unavailable", state)
	require.Equal(t, "missing_cancel_evidence", reason)

	// 证据齐备 → supported
	state, reason = service.EvaluateCapability(CapabilityInput{
		Profile:        ProfileRemote,
		DeployEnabled:  true,
		ServiceHealthy: true,
		UserPermitted:  true,
		Evidence:       CapabilityEvidence{Profile: ProfileRemote, EvidenceKeys: []string{"remote:cancel-evidence", "remote:paseo-live-probe"}},
	})
	require.Equal(t, "supported", state)
	require.Empty(t, reason)

	// 部署关闭 → unavailable(deploy_disabled)（配置隐藏，不置灰按钮）
	state, reason = service.EvaluateCapability(CapabilityInput{Profile: ProfilePersonalNode, DeployEnabled: false, ServiceHealthy: true, UserPermitted: true})
	require.Equal(t, "unavailable", state)
	require.Equal(t, "deploy_disabled", reason)

	// 权限缺失 → forbidden（区别于不可用）
	state, _ = service.EvaluateCapability(CapabilityInput{Profile: ProfileResources, DeployEnabled: true, ServiceHealthy: true, UserPermitted: false})
	require.Equal(t, "forbidden", state)

	// 撤销：刷新后仍拒绝新准入；core 不可撤销
	require.NoError(t, service.RevokeCapability(ProfileResources))
	service.RefreshCapability(ProfileResources)
	require.False(t, service.IsCapabilityEnabled(ProfileResources))
	state, reason = service.EvaluateCapability(CapabilityInput{Profile: ProfileResources, DeployEnabled: true, ServiceHealthy: true, UserPermitted: true})
	require.Equal(t, "unavailable", state)
	require.Equal(t, "capability_revoked", reason)
	require.Error(t, service.RevokeCapability(ProfileCore), "core profile must not be revocable")
}
