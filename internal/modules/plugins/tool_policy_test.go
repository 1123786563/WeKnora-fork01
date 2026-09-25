package plugins_test

// T18（plan 10 Task 18 Step 1）：写工具默认关闭治理面——fake 层语义测试。
//
// 覆盖计划 Step 1 的三个用例：
//  1. TestListInstallationToolsShowsDisabledReason：安装（1 只读 + 1 写）→
//     列表：只读 Enabled=true 无 reason；写 Enabled=false、DisabledReason=
//     "write tool disabled by default; enable explicit"。
//  2. TestSetInstallationToolPolicyTogglesWriteTool：PUT enabled=true → 行更新、
//     列表反映；再关闭 → 回到默认 reason；未知工具名 → 错误；非本租户安装 →
//     not found。
//  3. TestListInstallationToolsDropsRemovedSnapshotTools：升级移除工具 X（其旧
//     MCPToolApproval 行残留 Enabled=true）→ ListInstallationTools 以快照为源，
//     X 不出现在列表；Agent 目录（T09 快照守卫）同样不见 X——残留行不复活
//     已移除工具。
//
// 外部测试包 plugins_test 并 import plugintest（总索引「测试包名约定」）。
// fake 基建复用 upgrade_accept_test.go 的 upgradeAcceptStack（同测试包）：
// 真实 PluginService + plugintest 受控远端 + 内存仓储。

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/modules/plugins/plugintest"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// toolPolicyV1Tools：v1 = 1 只读 read_probe + 1 写 write_probe（安装落行：
// 只读 Enabled=true、写 Enabled=false）。
func toolPolicyV1Tools() []plugintest.Tool {
	return []plugintest.Tool{
		{Name: "read_probe", Description: "read probe", ReadOnly: true,
			InputSchema: upgradeV1SearchSchema},
		{Name: "write_probe", Description: "write probe", ReadOnly: false,
			InputSchema: upgradeV2WriteSchema},
	}
}

// policyByName 把治理列表按工具名索引。
func policyByName(rows []interfaces.PluginInstallationToolPolicy) map[string]interfaces.PluginInstallationToolPolicy {
	out := make(map[string]interfaces.PluginInstallationToolPolicy, len(rows))
	for _, row := range rows {
		out[row.Name] = row
	}
	return out
}

// TestListInstallationToolsShowsDisabledReason（计划 Step 1 用例 1）：安装后
// 治理列表逐行给出确定值——只读工具 Enabled=true、无 reason；写工具
// Enabled=false 且 DisabledReason 是插件域默认关闭文案（B5：清单声明不是
// 执行授权）。
func TestListInstallationToolsShowsDisabledReason(t *testing.T) {
	const tenantID = uint64(11)
	s := newUpgradeAcceptStackWithTools(t, tenantID, toolPolicyV1Tools(), toolPolicyV1Tools())
	ctx := context.Background()

	rows, err := s.svc.ListInstallationTools(ctx, tenantID, s.inst.ID)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	byName := policyByName(rows)
	read := byName["read_probe"]
	require.True(t, read.Enabled, "a read-only tool installs enabled")
	require.False(t, read.RequireApproval)
	require.Empty(t, read.DisabledReason)
	require.True(t, read.ReadOnly)

	write := byName["write_probe"]
	require.False(t, write.Enabled, "a write tool installs disabled (B5)")
	require.Equal(t, "write tool disabled by default; enable explicit", write.DisabledReason)
	require.False(t, write.RequireApproval)
	require.False(t, write.ReadOnly)

	// 跨租户安装 ID → not found（无存在性泄露）。
	_, err = s.svc.ListInstallationTools(ctx, tenantID+1, s.inst.ID)
	require.ErrorIs(t, err, service.ErrInstallationNotFound)
}

// TestSetInstallationToolPolicyTogglesWriteTool（计划 Step 1 用例 2）：
// 管理员启用写工具 → 策略行翻 true、返回列表反映且 reason 清空；再关闭 →
// 行回 false、列表回到默认关闭 reason；未知工具名（不在已接受快照内，含
// 已被升级移除的工具）→ 确定性错误；非本租户安装 → not found。
func TestSetInstallationToolPolicyTogglesWriteTool(t *testing.T) {
	const tenantID = uint64(11)
	s := newUpgradeAcceptStackWithTools(t, tenantID, toolPolicyV1Tools(), toolPolicyV1Tools())
	ctx := context.Background()

	// 启用写工具。
	enable := true
	rows, err := s.svc.SetInstallationToolPolicy(ctx, tenantID, s.inst.ID, "write_probe", &enable, nil)
	require.NoError(t, err)
	write := policyByName(rows)["write_probe"]
	require.True(t, write.Enabled, "the returned list must reflect the enabled row")
	require.Empty(t, write.DisabledReason)
	policies := s.policyRows(t, tenantID)
	require.True(t, policies["write_probe"].Enabled, "the approval row itself must be updated")

	// 再关闭 → 回到默认 reason。
	disable := false
	rows, err = s.svc.SetInstallationToolPolicy(ctx, tenantID, s.inst.ID, "write_probe", &disable, nil)
	require.NoError(t, err)
	write = policyByName(rows)["write_probe"]
	require.False(t, write.Enabled)
	require.Equal(t, "write tool disabled by default; enable explicit", write.DisabledReason)
	require.False(t, s.policyRows(t, tenantID)["write_probe"].Enabled)

	// 只读工具的既有行不受写工具治理影响。
	require.True(t, s.policyRows(t, tenantID)["read_probe"].Enabled)

	// 未知工具名 → 确定性错误（快照是成员资格权威）。
	_, err = s.svc.SetInstallationToolPolicy(ctx, tenantID, s.inst.ID, "no_such_tool", &enable, nil)
	require.ErrorIs(t, err, service.ErrInstallationToolNotFound)

	// enabled/requireApproval 全空 → 确定性错误（无可更新字段）。
	_, err = s.svc.SetInstallationToolPolicy(ctx, tenantID, s.inst.ID, "write_probe", nil, nil)
	require.ErrorIs(t, err, service.ErrInstallationPolicyInvalid)

	// 非本租户安装 → not found。
	_, err = s.svc.SetInstallationToolPolicy(ctx, tenantID+1, s.inst.ID, "write_probe", &enable, nil)
	require.ErrorIs(t, err, service.ErrInstallationNotFound)
}

// TestListInstallationToolsDropsRemovedSnapshotTools（计划 Step 1 用例 3）：
// v1（search + lookup，均只读）升级到 v2（移除 lookup）后，lookup 的旧
// MCPToolApproval 行残留（Enabled=true——升级策略行只增量写、既有行不删）。
// ListInstallationTools 以已接受快照为成员资格来源：lookup 不出现在治理列表；
// Agent 目录（T09 快照守卫）同样不见 lookup——残留 Enabled=true 行不得复活
// 已被移除的工具。
func TestListInstallationToolsDropsRemovedSnapshotTools(t *testing.T) {
	const tenantID = uint64(11)
	v1 := []plugintest.Tool{
		{Name: "search", Description: "search", ReadOnly: true, InputSchema: upgradeV1SearchSchema},
		{Name: "lookup", Description: "lookup", ReadOnly: true, InputSchema: upgradeV1SearchSchema},
	}
	v2 := []plugintest.Tool{
		{Name: "search", Description: "search", ReadOnly: true, InputSchema: upgradeV1SearchSchema},
	}
	s := newUpgradeAcceptStackWithTools(t, tenantID, v1, v2)
	ctx := context.Background()

	// 升级到 v2（移除 lookup）。
	resp := s.previewV2(t, tenantID)
	_, err := s.svc.AcceptUpgrade(ctx, tenantID, "admin-1", s.inst.ID, resp.CandidateFingerprint)
	require.NoError(t, err)

	// 前提自证：lookup 的策略行确实残留且 Enabled=true（升级不删既有行）。
	residual, ok := s.policyRows(t, tenantID)["lookup"]
	require.True(t, ok, "the removed tool's approval row must still exist (upgrade keeps existing rows)")
	require.True(t, residual.Enabled)

	// 治理列表：只有 search——快照是成员资格来源，残留行不复活。
	rows, err := s.svc.ListInstallationTools(ctx, tenantID, s.inst.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "search", rows[0].Name)

	// Agent 目录（T09 快照守卫）：同样只有 search。
	require.ElementsMatch(t, []string{"search"}, snapshotToolNames(s.guardSnapshot(t, tenantID)),
		"the residual Enabled=true row must not resurrect the removed tool in the agent directory")

	// 残留工具名不可经治理面寻址（不存在于已接受快照）。
	enable := true
	_, err = s.svc.SetInstallationToolPolicy(ctx, tenantID, s.inst.ID, "lookup", &enable, nil)
	require.ErrorIs(t, err, service.ErrInstallationToolNotFound,
		"a tool removed from the accepted snapshot must not be addressable by policy writes")

	// 类型层回归：安装详情视图同样以快照为源（lookup 不出现在详情工具行）。
	view, err := s.svc.GetInstallation(ctx, tenantID, s.inst.ID)
	require.NoError(t, err)
	require.Len(t, view.Tools, 1)
	require.Equal(t, "search", view.Tools[0].Name)
}
