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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/modules/plugins/plugintest"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
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

// TestSetInstallationToolPolicyRequireApprovalOnlyMaterializesDefault
// （T18-OCR1-F1 回归）：requireApproval 单独 patch 一个**无策略行**的工具时，
// 共享 UpsertPolicy 的首插默认会把省略的 enabled 落为 true——对写工具即
// 「只设审批位就隐式启用派发资格」的 fail-open。修复后：无行工具先物化
// 插件域默认（enabled=ReadOnly），既有行的 enabled 不受 requireApproval
// 单独 patch 影响。
func TestSetInstallationToolPolicyRequireApprovalOnlyMaterializesDefault(t *testing.T) {
	const tenantID = uint64(11)
	s := newUpgradeAcceptStackWithTools(t, tenantID, toolPolicyV1Tools(), toolPolicyV1Tools())
	ctx := context.Background()

	// 制造无行异常：直接删除写工具 write_probe 的安装期策略行。
	delete(s.approvalRepo.rows, approvalKey(s.inst.ServiceID, "write_probe"))
	require.NotContains(t, s.policyRows(t, tenantID), "write_probe")

	// requireApproval 单独 patch 无行写工具 → 行必须落 enabled=false（插件域
	// 默认物化），不得落入共享首插默认 enabled=true。
	requireApproval := true
	rows, err := s.svc.SetInstallationToolPolicy(ctx, tenantID, s.inst.ID, "write_probe", nil, &requireApproval)
	require.NoError(t, err)
	write := policyByName(rows)["write_probe"]
	require.False(t, write.Enabled, "a requireApproval-only patch on a MISSING row must materialize the plugin default (write=disabled), never the shared insert default enabled=true")
	require.True(t, write.RequireApproval)
	require.Equal(t, interfaces.PluginWriteToolDisabledReason, write.DisabledReason)
	row := s.policyRows(t, tenantID)["write_probe"]
	require.NotNil(t, row)
	require.False(t, row.Enabled, "the persisted row itself must carry enabled=false")

	// 既有行不受影响：read_probe 行在场（Enabled=true），requireApproval 单独
	// patch 只改审批位，不改启停。
	rows, err = s.svc.SetInstallationToolPolicy(ctx, tenantID, s.inst.ID, "read_probe", nil, &requireApproval)
	require.NoError(t, err)
	read := policyByName(rows)["read_probe"]
	require.True(t, read.Enabled, "an EXISTING row keeps its enabled verdict under a requireApproval-only patch")
	require.True(t, read.RequireApproval)
}

// TestListInstallationToolsDegradedAnchors（T18-OCR1-F4 回归）：治理面对本
// 文件已识别的两种降级锚状态不得误报 500 persist-failed——
//
//	a) 悬空 service_id 锚（级联删掉服务行与策略行、安装行残留）：列表按
//	   无显式行渲染插件域默认值（真实且 fail-closed），策略写返回确定性
//	   状态错误（ErrInstallationServiceMissing）而非 5xx；
//	b) 空 ServiceID 中断窗口且物化行仍在：经 mcp_services 反查自愈读行。
func TestListInstallationToolsDegradedAnchors(t *testing.T) {
	const tenantID = uint64(11)
	ctx := context.Background()

	t.Run("dangling service anchor renders defaults and rejects writes with a state error", func(t *testing.T) {
		s := newUpgradeAcceptStackWithTools(t, tenantID, toolPolicyV1Tools(), toolPolicyV1Tools())

		// 悬空锚：物化服务行消失（策略行随级联消失），安装行 service_id 残留。
		s.mcpRepo.services = nil

		rows, err := s.svc.ListInstallationTools(ctx, tenantID, s.inst.ID)
		require.NoError(t, err, "a dangling anchor must not read as a store fault — the cascade-gone rows ARE the no-row default state")
		byName := policyByName(rows)
		require.True(t, byName["read_probe"].Enabled)
		require.Empty(t, byName["read_probe"].DisabledReason)
		require.False(t, byName["write_probe"].Enabled)
		require.Equal(t, interfaces.PluginWriteToolDisabledReason, byName["write_probe"].DisabledReason)

		enable := true
		_, err = s.svc.SetInstallationToolPolicy(ctx, tenantID, s.inst.ID, "write_probe", &enable, nil)
		require.ErrorIs(t, err, service.ErrInstallationServiceMissing,
			"a policy write against a gone service row is a deterministic state error, not a 5xx persist fault")
	})

	t.Run("empty service_id window self-heals through the service back-reference", func(t *testing.T) {
		s := newUpgradeAcceptStackWithTools(t, tenantID, toolPolicyV1Tools(), toolPolicyV1Tools())

		// 中断窗口：安装行 service_id 为空而物化行已在（反查自愈）。
		s.inst.ServiceID = ""

		rows, err := s.svc.ListInstallationTools(ctx, tenantID, s.inst.ID)
		require.NoError(t, err)
		byName := policyByName(rows)
		require.True(t, byName["read_probe"].Enabled, "the healed lookup must read the install-time rows")
		require.False(t, byName["write_probe"].Enabled)
		require.True(t, byName["write_probe"].RequireApproval == false)

		// 写路径同样经自愈锚定到实际服务行。
		enable := true
		rows, err = s.svc.SetInstallationToolPolicy(ctx, tenantID, s.inst.ID, "write_probe", &enable, nil)
		require.NoError(t, err)
		require.True(t, policyByName(rows)["write_probe"].Enabled)
	})
}

// TestInstallationToolsWireRequireApprovalShapes（T18-OCR1-F2 回归，handler
// 边界）：治理面（GET .../tools）对 require_approval 给确定值；详情面
// （GET .../installations/:id）不携带该键（详情视图契约无审批列——emit 一个
// 硬编码 false 会与治理面/运行时审批门禁矛盾，误导管理决策）。
func TestInstallationToolsWireRequireApprovalShapes(t *testing.T) {
	const tenantID = uint64(11)
	s := newUpgradeAcceptStackWithTools(t, tenantID, toolPolicyV1Tools(), toolPolicyV1Tools())

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), tenantID)
		c.Set(types.UserIDContextKey.String(), "admin-1")
		c.Next()
	})
	h := handler.NewPluginHandler(s.svc)
	r.GET("/plugins/installations/:id", h.GetInstallation)
	r.GET("/plugins/installations/:id/tools", h.ListInstallationTools)
	r.PUT("/plugins/installations/:id/tools/:tool_name/policy", h.SetInstallationToolPolicy)

	// do 返回成功 envelope 的 data 原文（PUT/GET tools 为数组，detail 为对象）。
	do := func(method, path, body string) json.RawMessage {
		t.Helper()
		var req *http.Request
		if body == "" {
			req = httptest.NewRequest(method, path, nil)
		} else {
			req = httptest.NewRequest(method, path, strings.NewReader(body))
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
		return envelope.Data
	}
	toolRow := func(data json.RawMessage, name string) map[string]any {
		t.Helper()
		var asList []map[string]any
		if err := json.Unmarshal(data, &asList); err == nil {
			for _, row := range asList {
				if row["name"] == name {
					return row
				}
			}
			t.Fatalf("tool %q not found in list payload", name)
		}
		var asObject struct {
			Tools []map[string]any `json:"tools"`
		}
		require.NoError(t, json.Unmarshal(data, &asObject))
		for _, row := range asObject.Tools {
			if row["name"] == name {
				return row
			}
		}
		t.Fatalf("tool %q not found in tools payload", name)
		return nil
	}

	// PUT require_approval=true（本端点今天就接受该位——F2 的矛盾根源）。
	put := do(http.MethodPut, "/plugins/installations/"+s.inst.ID+"/tools/write_probe/policy",
		`{"require_approval":true}`)
	writeRow := toolRow(put, "write_probe")
	require.Equal(t, true, writeRow["require_approval"], "the governance surface carries the CURRENT approval verdict as a definite value")
	require.Equal(t, false, writeRow["enabled"], "the requireApproval-only patch keeps the disabled write tool disabled (F1)")

	// GET .../tools：同形状确定值。
	list := do(http.MethodGet, "/plugins/installations/"+s.inst.ID+"/tools", "")
	writeList := toolRow(list, "write_probe")
	require.Equal(t, true, writeList["require_approval"])

	// GET .../installations/:id（详情面）：不携带 require_approval 键——不得
	// 断言一个与真实行矛盾的确定值 false。
	detail := do(http.MethodGet, "/plugins/installations/"+s.inst.ID, "")
	detailWrite := toolRow(detail, "write_probe")
	_, hasKey := detailWrite["require_approval"]
	require.False(t, hasKey, "the detail payload must OMIT require_approval (unknown on this surface), never assert a contradictory false")
	require.Equal(t, false, detailWrite["enabled"], "detail keeps Enabled as a definite value (T18 unification)")
}

// TestSetInstallationToolPolicyConcurrentAcceptSerializes（OCR 终局第 2 轮
// f14）：SetInstallationToolPolicy 是插件安装服务里唯一缺 per-installation
// 锁的策略写路径（其余 7 处写路径均持 lockUpgradeAccept）——它在
// GetInstallation 快照成员校验（读）与 toolApprovalService.SetPolicy（写，
// 仅校验服务存在）之间不持锁，经 PUT .../policy（Admin）暴露。与并发
// AcceptUpgrade（同写策略行/安装行/物化行）交错时：级联/接受先行删改服务
// 行后 PATCH 的 Upsert 撞已删行（伪 500），或本应被快照成员校验拒绝的
// 请求成功落行。在 PATCH 的 UpsertPolicy 在途时（onUpsert 钩子）并发发起
// 接受：接受在 PATCH 在途期间必须零进展；串行化后终态一致（安装 v2、
// PATCH 的行裁决保留——接受路径 7c 只补缺行）。
func TestSetInstallationToolPolicyConcurrentAcceptSerializes(t *testing.T) {
	const tenantID = uint64(24)
	s := newUpgradeAcceptStack(t, tenantID)
	resp := s.previewV2(t, tenantID)
	ctx := context.Background()

	acceptDone := make(chan error, 1)
	progressedDuringPatch := make(chan bool, 1)
	s.approvalRepo.onUpsert = func() {
		s.approvalRepo.onUpsert = nil // 只在 PATCH 的首个 upsert 触发一次
		go func() {
			_, err := s.svc.AcceptUpgrade(ctx, tenantID, "admin-1", s.inst.ID, resp.CandidateFingerprint)
			acceptDone <- err
		}()
		select {
		case <-acceptDone:
			progressedDuringPatch <- true // 接受在 PATCH 在途期间完成 → 未串行化
		case <-time.After(1500 * time.Millisecond):
			progressedDuringPatch <- false // 接受全程被阻塞 → 串行化生效
		}
	}

	disabled := false
	_, err := s.svc.SetInstallationToolPolicy(ctx, tenantID, s.inst.ID, "search", &disabled, nil)
	require.NoError(t, err, "the in-flight policy patch completes normally (its write holds the lock)")
	require.False(t, <-progressedDuringPatch,
		"a concurrent accept must make no progress while the policy patch is mid-flight (f14)")
	require.NoError(t, <-acceptDone, "the serialized accept then completes normally")

	// 终态：安装已切 v2；PATCH 的行裁决保留（接受路径 7c 只补缺行）。
	inst := s.innerRepo.installations[0]
	require.Equal(t, "2.0.0", inst.AcceptedVersion)
	rows, err := s.svc.ListInstallationTools(ctx, tenantID, s.inst.ID)
	require.NoError(t, err)
	require.False(t, policyByName(rows)["search"].Enabled,
		"the patch's verdict survives the serialized accept (existing rows keep the admin's verdicts)")
	require.Len(t, rows, 3, "the accept's new tools land their install-time rows")
}
