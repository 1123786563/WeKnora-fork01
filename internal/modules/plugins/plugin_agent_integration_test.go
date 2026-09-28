// 无账号插件工具应用边界集成测试（计划 Task 10）：tools 组装层全链——
// 受控 plugintest 远端 + 真实 MCPManager + RegisterMCPTools 快照守卫 +
// 自动批准的 approval 替身。按仓库测试包名约定（总索引「测试包名约定」），
// import plugintest 的测试一律写外部测试包 plugins_test。本文件仅供测试使用。
package plugins_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/approval"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/tools"
	internalmcp "github.com/Tencent/WeKnora/internal/modules/airesource/mcp"
	"github.com/Tencent/WeKnora/internal/modules/plugins"
	"github.com/Tencent/WeKnora/internal/modules/plugins/plugintest"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// agentITNoArgSchema 是受控工具的固定无参 schema：安装快照 digest 由
// plugins.ToolSchemaDigest 现算，与 plugintest live 目录同一口径。
const agentITNoArgSchema = `{"type":"object","properties":{},"additionalProperties":false}`

// agentITContext 构造带租户与成员身份的执行上下文（先例 tools 包
// catalogTestContext，mcp_catalog_test.go:31-34；本包无法复用未导出辅助）。
func agentITContext(tenantID uint64, principalID string) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, tenantID)
	return types.WithPrincipal(ctx, types.Principal{Type: types.PrincipalWebUser, ID: principalID})
}

// agentITGate 是自动批准的审批替身（先例 proxyApprovalGate，
// mcp_catalog_integration_test.go:21-39）：插件工具调用必须经过既有
// approval.Gate 链（GAP-5），这里断言链路确实被走到。
type agentITGate struct {
	approved atomic.Int32
}

func (*agentITGate) IsEnabled(context.Context, uint64, string, string) (bool, error) {
	return true, nil
}
func (*agentITGate) NeedsApproval(context.Context, uint64, string, string) bool { return true }
func (g *agentITGate) RequestAndWait(
	_ context.Context, _ approval.PendingRequest,
) (approval.Decision, error) {
	g.approved.Add(1)
	return approval.Decision{Approved: true}, nil
}

// agentITPage/agentITDefinition 是 discover_mcp_tools 输出的本包解析副本
// （tools 包的 mcpDiscoveryPage 未导出，无法跨包复用）。
type agentITServerRow struct {
	ServerID string `json:"server_id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
}

type agentITToolRow struct {
	ToolRef  string `json:"tool_ref,omitempty"`
	ServerID string `json:"server_id"`
	Name     string `json:"name"`
}

type agentITPage struct {
	Mode    string             `json:"mode"`
	Servers []agentITServerRow `json:"servers"`
	Tools   []agentITToolRow   `json:"tools"`
}

type agentITDefinition struct {
	Notice      string          `json:"notice"`
	ToolRef     string          `json:"tool_ref"`
	ServerID    string          `json:"server_id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

func agentITDiscover(
	ctx context.Context, t *testing.T, registry *tools.ToolRegistry, args map[string]any,
) agentITPage {
	t.Helper()
	raw, err := json.Marshal(args)
	require.NoError(t, err)
	result, err := registry.ExecuteTool(ctx, tools.ToolDiscoverMCPTools, raw)
	require.NoError(t, err)
	require.True(t, result.Success, result.Error)
	var page agentITPage
	require.NoError(t, json.Unmarshal([]byte(result.Output), &page))
	return page
}

// agentITDiscoverResult 返回原始 ToolResult——拒绝类断言必须看 Success=false
// 与错误文案，不能走会 assert 成功的 agentITDiscover。
func agentITDiscoverResult(
	ctx context.Context, t *testing.T, registry *tools.ToolRegistry, args map[string]any,
) *types.ToolResult {
	t.Helper()
	raw, err := json.Marshal(args)
	require.NoError(t, err)
	result, err := registry.ExecuteTool(ctx, tools.ToolDiscoverMCPTools, raw)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func agentITDescribe(
	ctx context.Context, t *testing.T, registry *tools.ToolRegistry, serverID, toolName string,
) agentITDefinition {
	t.Helper()
	result, err := json.Marshal(map[string]any{
		"mode": "describe", "server_id": serverID, "tool_name": toolName,
	})
	require.NoError(t, err)
	outcome, err := registry.ExecuteTool(ctx, tools.ToolDiscoverMCPTools, result)
	require.NoError(t, err)
	require.True(t, outcome.Success, outcome.Error)
	var definition agentITDefinition
	require.NoError(t, json.Unmarshal([]byte(outcome.Output), &definition))
	require.NotEmpty(t, definition.ToolRef)
	return definition
}

// agentITCallCtx 返回携带 ToolExecContext 的调用上下文：MCPTool.Execute 的
// 审批等待分支只在 EventBus 在场时激活（先例 mcp_catalog_integration_test.go:113-122）。
func agentITCallCtx(ctx context.Context) context.Context {
	return tools.WithToolExecContext(ctx, &tools.ToolExecContext{
		EventBus:           event.NewEventBus(),
		ToolCallID:         "agent-it-call",
		SessionID:          "agent-it-session",
		AssistantMessageID: "agent-it-message",
		ApprovalCtx:        ctx,
	})
}

func agentITCall(
	ctx context.Context, t *testing.T, registry *tools.ToolRegistry, toolRef string, arguments json.RawMessage,
) *types.ToolResult {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"tool_ref": toolRef, "arguments": arguments})
	require.NoError(t, err)
	result, err := registry.ExecuteTool(agentITCallCtx(ctx), tools.ToolCallMCPTool, raw)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

// agentITSnapshot 从受控工具声明构造「已接受安装快照」（与 T06 确认安装时
// 从 live 目录计算权威快照同构：digest 用 plugins.ToolSchemaDigest 现算）。
func agentITSnapshot(installationID string, specs []plugintest.Tool) *tools.PluginRuntimeSnapshot {
	snapshot := make([]types.PluginToolSnapshot, 0, len(specs))
	for _, spec := range specs {
		snapshot = append(snapshot, types.PluginToolSnapshot{
			Name:                 spec.Name,
			Description:          spec.Description,
			InputSchemaDigest:    plugins.ToolSchemaDigest([]byte(spec.InputSchema)),
			ReadOnly:             spec.ReadOnly,
			RequiresPersonalAuth: spec.RequiresPersonalAuth,
			Scopes:               spec.Scopes,
		})
	}
	return &tools.PluginRuntimeSnapshot{InstallationID: installationID, Tools: snapshot}
}

// agentITPluginService 物化一行插件服务（PluginInstallationID 指回安装），
// 名称遵循 "plugin:" + plugin_id 的物化惯例（总索引「其余架构决策」）。
func agentITPluginService(id string, tenantID uint64, endpoint, installationID string) *types.MCPService {
	return &types.MCPService{
		ID:                   id,
		TenantID:             tenantID,
		Enabled:              true,
		Name:                 "plugin:" + id,
		URL:                  &endpoint,
		TransportType:        types.MCPTransportHTTPStreamable,
		PluginInstallationID: &installationID,
	}
}

func agentITManualService(id string, tenantID uint64, endpoint string) *types.MCPService {
	return &types.MCPService{
		ID:            id,
		TenantID:      tenantID,
		Enabled:       true,
		Name:          "手工服务-" + id,
		URL:           &endpoint,
		TransportType: types.MCPTransportHTTPStreamable,
	}
}

func agentITRegister(
	ctx context.Context,
	t *testing.T,
	manager *internalmcp.MCPManager,
	gate *agentITGate,
	guard tools.PluginSnapshotProvider,
	services ...*types.MCPService,
) *tools.ToolRegistry {
	t.Helper()
	registry := tools.NewToolRegistry()
	_, err := tools.RegisterMCPTools(ctx, registry, services, manager, gate, 0, nil, nil, guard)
	require.NoError(t, err)
	return registry
}

func agentITServerIDs(page agentITPage) []string {
	ids := make([]string, 0, len(page.Servers))
	for _, server := range page.Servers {
		ids = append(ids, server.ServerID)
	}
	return ids
}

func agentITToolNames(page agentITPage) []string {
	names := make([]string, 0, len(page.Tools))
	for _, tool := range page.Tools {
		names = append(names, tool.Name)
	}
	return names
}

// TestAgentCallsNoAccountPluginToolEndToEnd：无账号只读工具的
// discover→describe→call 全链成功；结果带来源前缀与真实载荷，外呼计数
// 恰好 +1，且调用经过既有 approval.Gate 链（GAP-5：插件不另造授权路径）。
func TestAgentCallsNoAccountPluginToolEndToEnd(t *testing.T) {
	remote := plugintest.New()
	specs := []plugintest.Tool{{
		Name:        "list_todos",
		Description: "查询本空间待办（无账号只读）",
		ReadOnly:    true,
		InputSchema: agentITNoArgSchema,
	}}
	remote.SetTools(specs)
	remote.Start(t)

	ctx := agentITContext(1, "agent-user")
	manager := internalmcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	gate := &agentITGate{}
	service := agentITPluginService("plugin-svc", 1, remote.BaseURL()+"/mcp", "inst-agent-1")
	guard := tools.PluginSnapshotProvider(
		func(_ context.Context, tenantID uint64, serviceID string) (*tools.PluginRuntimeSnapshot, error) {
			require.Equal(t, uint64(1), tenantID, "the guard must resolve with the executing tenant")
			require.Equal(t, "plugin-svc", serviceID)
			return agentITSnapshot("inst-agent-1", specs), nil
		},
	)
	registry := agentITRegister(ctx, t, manager, gate, guard, service)

	// list_servers：物化插件服务对成员可见。
	page := agentITDiscover(ctx, t, registry, map[string]any{"mode": "list_servers"})
	require.Equal(t, []string{"plugin-svc"}, agentITServerIDs(page))

	// list_tools：目录恰为快照内工具；列举不构成外呼。
	toolPage := agentITDiscover(ctx, t, registry, map[string]any{
		"mode": "list_tools", "server_id": service.ID,
	})
	require.Equal(t, []string{"list_todos"}, agentITToolNames(toolPage))
	require.Zero(t, remote.Calls(), "listing must not call the tool")

	// describe：返回可执行 tool_ref，且携带外部数据告示（提示注入缓解）。
	definition := agentITDescribe(ctx, t, registry, service.ID, "list_todos")
	require.Contains(t, definition.Notice, "untrusted data")

	// call：成功、结果带来源前缀与真实载荷、外呼计数 +1、经审批链。
	result := agentITCall(ctx, t, registry, definition.ToolRef, json.RawMessage(`{}`))
	require.True(t, result.Success, result.Error)
	require.Contains(t, result.Output, fmt.Sprintf("[MCP tool result from %q", service.Name),
		"the call result must carry the source prefix")
	require.Contains(t, result.Output, "ok:list_todos:", "the payload is the remote's honest text")
	require.Equal(t, int64(1), remote.Calls())
	require.Zero(t, remote.WriteCalls())
	require.Equal(t, int32(1), gate.approved.Load(), "plugin tool calls go through the existing approval gate")
}

// TestEmptyAndErrorResultsSurfaceAsIs：空结果与错误结果如实呈现——空文本
// 不虚构载荷（显式标注 no text output），工具级错误原样回显且 Success=false；
// 两种情况都真实到达远端（计数 +2），错误来自远端而非本地编造。
func TestEmptyAndErrorResultsSurfaceAsIs(t *testing.T) {
	remote := plugintest.New()
	specs := []plugintest.Tool{
		{
			Name:        "empty_lookup",
			Description: "可能无结果的只读查询",
			ReadOnly:    true,
			InputSchema: agentITNoArgSchema,
			Call:        func(string) (string, error) { return "", nil },
		},
		{
			Name:        "failing_lookup",
			Description: "远端故障面",
			ReadOnly:    true,
			InputSchema: agentITNoArgSchema,
			Call: func(string) (string, error) {
				return "", errors.New("upstream jira unavailable: 503")
			},
		},
	}
	remote.SetTools(specs)
	remote.Start(t)

	ctx := agentITContext(1, "agent-user")
	manager := internalmcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	service := agentITPluginService("plugin-svc", 1, remote.BaseURL()+"/mcp", "inst-agent-2")
	guard := tools.PluginSnapshotProvider(
		func(_ context.Context, _ uint64, _ string) (*tools.PluginRuntimeSnapshot, error) {
			return agentITSnapshot("inst-agent-2", specs), nil
		},
	)
	registry := agentITRegister(ctx, t, manager, &agentITGate{}, guard, service)

	toolPage := agentITDiscover(ctx, t, registry, map[string]any{
		"mode": "list_tools", "server_id": service.ID,
	})
	require.Equal(t, []string{"empty_lookup", "failing_lookup"}, agentITToolNames(toolPage))

	// 空文本：成功执行，来源前缀仍在，正文如实标注无输出——不虚构数据。
	emptyDef := agentITDescribe(ctx, t, registry, service.ID, "empty_lookup")
	empty := agentITCall(ctx, t, registry, emptyDef.ToolRef, json.RawMessage(`{}`))
	require.True(t, empty.Success, empty.Error)
	require.Contains(t, empty.Output, fmt.Sprintf("[MCP tool result from %q", service.Name))
	require.Contains(t, empty.Output, "(no text output)")
	require.NotContains(t, empty.Output, "ok:", "no fabricated payload for an empty result")

	// 协议级 error 面（OCR R1 F14：替身忠实复刻示例服务的 (nil, err) 错误
	// 面，不再是 isError=true 的成功结果）：Success=false，文案为远端原文
	//（经 oauthAwareConnectError 包装但保留原文），如实不虚构成功。
	failingDef := agentITDescribe(ctx, t, registry, service.ID, "failing_lookup")
	failed := agentITCall(ctx, t, registry, failingDef.ToolRef, json.RawMessage(`{}`))
	require.False(t, failed.Success)
	require.Contains(t, failed.Error, "upstream jira unavailable: 503")

	// 两次调用都真实到达远端（错误是远端工具错误面，非本地拦截）。协议级
	// error 在真实客户端链路上会断连重试一次（mcp_tool.go connectAndCall）
	// ——错误调用因此计 2 次、合计 3 次；旧的 isError 形态不重试合计 2 次。
	require.GreaterOrEqual(t, remote.Calls(), int64(2), "both calls reach the remote (the error call retries once over a fresh connection)")
	require.Zero(t, remote.WriteCalls())
}

// TestUninstalledOrDisabledOrForeignTenantRejected：三种不可用形态全部拒绝——
// a) 未安装（服务列表不含该物化行）→ list_servers 不出现、list_tools 拒绝；
// b) 安装停用（物化服务 Enabled=false）→ 不可见；
// c) 跨空间：tenant 2 的上下文调 tenant 1 的 registry 被 authorize 拒绝；
//
//	即使 tenant 2 自建 registry（生产 PluginSnapshotLookup 按租户查无安装行）
//	插件物化行也 fail-closed，目录不可用。
func TestUninstalledOrDisabledOrForeignTenantRejected(t *testing.T) {
	remote := plugintest.New()
	specs := []plugintest.Tool{{
		Name:        "list_todos",
		Description: "查询本空间待办（无账号只读）",
		ReadOnly:    true,
		InputSchema: agentITNoArgSchema,
	}}
	remote.SetTools(specs)
	remote.Start(t)
	endpoint := remote.BaseURL() + "/mcp"

	ctx1 := agentITContext(1, "agent-user")
	ctx2 := agentITContext(2, "user-2")
	manager := internalmcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	gate := &agentITGate{}

	// 与生产 service.PluginSnapshotLookup 同语义：按 (tenant, serviceID)
	// 反查安装行，租户外查无行 → (nil, nil)。
	tenantScopedGuard := func(owner uint64, installationID string) tools.PluginSnapshotProvider {
		return func(_ context.Context, tenantID uint64, serviceID string) (*tools.PluginRuntimeSnapshot, error) {
			if tenantID == owner && serviceID == "plugin-svc" {
				return agentITSnapshot(installationID, specs), nil
			}
			return nil, nil
		}
	}

	// a) 未安装：租户 1 的服务列表只含手工服务——插件物化行不存在。
	manual := agentITManualService("manual-svc", 1, endpoint)
	registryA := agentITRegister(ctx1, t, manager, gate, tenantScopedGuard(1, "inst-1"), manual)
	page := agentITDiscover(ctx1, t, registryA, map[string]any{"mode": "list_servers"})
	require.Equal(t, []string{"manual-svc"}, agentITServerIDs(page),
		"an uninstalled plugin's materialized service must not appear in list_servers")
	rejected := agentITDiscoverResult(ctx1, t, registryA, map[string]any{
		"mode": "list_tools", "server_id": "plugin-svc",
	})
	// 拒绝面在参数校验层：discover 的 server_id 参数 schema 以目录枚举合法
	// 集合（mcp_catalog.go installMCPCatalog 的 mcpSchemaWithEnum），未安装
	// 的物化服务不在枚举内——对模型而言该 server_id 根本不被接受。
	require.False(t, rejected.Success)
	require.Contains(t, rejected.Error, "server_id")
	require.NotContains(t, rejected.Error, "plugin-svc",
		"the uninstalled plugin's service must not appear in the accepted server set")

	// b) 停用：物化服务 Enabled=false → newMCPCatalog 跳过该行，目录服务数为
	// 0，RegisterMCPTools 干脆不安装任何 MCP 入口（mcp_tool.go:755-757）——
	// 停用安装对成员完全不可见。
	disabled := agentITPluginService("plugin-svc", 1, endpoint, "inst-1")
	disabled.Enabled = false
	registryB := tools.NewToolRegistry()
	count, err := tools.RegisterMCPTools(
		ctx1, registryB, []*types.MCPService{disabled}, manager, gate, 0, nil, nil,
		tenantScopedGuard(1, "inst-1"),
	)
	require.NoError(t, err)
	require.Zero(t, count, "a disabled installation materializes no MCP services")
	_, err = registryB.GetTool(tools.ToolDiscoverMCPTools)
	require.Error(t, err, "with the installation disabled, the MCP directory entry point is absent")

	// c) 跨空间：tenant 2 的上下文不能使用 tenant 1 的 registry。
	installed := agentITPluginService("plugin-svc", 1, endpoint, "inst-1")
	registryC := agentITRegister(ctx1, t, manager, gate, tenantScopedGuard(1, "inst-1"), installed)
	foreign := agentITDiscoverResult(ctx2, t, registryC, map[string]any{"mode": "list_servers"})
	require.False(t, foreign.Success)
	require.Contains(t, foreign.Error, "unavailable for this authorization context")

	// c 续）tenant 2 自建 registry：守卫按租户查无安装行 → 插件物化行
	// fail-closed（不回退为不过滤），目录不可用。
	foreignRow := agentITPluginService("plugin-svc-t2", 2, endpoint, "inst-t2")
	registryD := agentITRegister(ctx2, t, manager, gate, tenantScopedGuard(1, "inst-1"), foreignRow)
	foreignTools := agentITDiscoverResult(ctx2, t, registryD, map[string]any{
		"mode": "list_tools", "server_id": "plugin-svc-t2",
	})
	require.False(t, foreignTools.Success,
		"a plugin-materialized service without a tenant-scoped installation must fail closed")
	require.Contains(t, foreignTools.Error, "is error; retry discovery")

	// 全程零外呼：没有任何一次拒绝路径触达远端工具。
	require.Zero(t, remote.Calls())
	require.Zero(t, remote.WriteCalls())
}

// TestManualMCPServiceCoexists：同一 registry 同时挂手工服务（守卫不命中，
// (nil,nil)）与插件安装——手工服务全部工具可见可调（现状语义），插件服务
// 仅快照内工具可见（手工 MCP 兼容约束的应用边界证明）。
func TestManualMCPServiceCoexists(t *testing.T) {
	remote := plugintest.New()
	specs := []plugintest.Tool{
		{
			Name:        "shared_read",
			Description: "两服务共有只读工具",
			ReadOnly:    true,
			InputSchema: agentITNoArgSchema,
		},
		{
			Name:        "manual_only_read",
			Description: "仅手工目录可见",
			ReadOnly:    true,
			InputSchema: agentITNoArgSchema,
		},
	}
	remote.SetTools(specs)
	remote.Start(t)
	endpoint := remote.BaseURL() + "/mcp"

	ctx := agentITContext(1, "agent-user")
	manager := internalmcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	manual := agentITManualService("manual-svc", 1, endpoint)
	pluginService := agentITPluginService("plugin-svc", 1, endpoint, "inst-1")
	accepted := specs[:1] // 空间只接受了 shared_read
	guard := tools.PluginSnapshotProvider(
		func(_ context.Context, _ uint64, serviceID string) (*tools.PluginRuntimeSnapshot, error) {
			if serviceID == pluginService.ID {
				return agentITSnapshot("inst-1", accepted), nil
			}
			return nil, nil // 手工服务：非插件物化，现状语义
		},
	)
	registry := agentITRegister(ctx, t, manager, &agentITGate{}, guard, manual, pluginService)

	// 两个服务并存可见。
	page := agentITDiscover(ctx, t, registry, map[string]any{"mode": "list_servers"})
	require.ElementsMatch(t, []string{"manual-svc", "plugin-svc"}, agentITServerIDs(page))

	// 手工服务：全部工具可见（现状语义）。
	manualTools := agentITDiscover(ctx, t, registry, map[string]any{
		"mode": "list_tools", "server_id": manual.ID,
	})
	require.Equal(t, []string{"manual_only_read", "shared_read"}, agentITToolNames(manualTools))

	// 插件服务：仅快照内工具可见。
	pluginTools := agentITDiscover(ctx, t, registry, map[string]any{
		"mode": "list_tools", "server_id": pluginService.ID,
	})
	require.Equal(t, []string{"shared_read"}, agentITToolNames(pluginTools))

	// 手工独有工具可调；插件快照内工具亦可调——两个外呼都计数。
	manualDef := agentITDescribe(ctx, t, registry, manual.ID, "manual_only_read")
	manualCall := agentITCall(ctx, t, registry, manualDef.ToolRef, json.RawMessage(`{}`))
	require.True(t, manualCall.Success, manualCall.Error)

	pluginDef := agentITDescribe(ctx, t, registry, pluginService.ID, "shared_read")
	pluginCall := agentITCall(ctx, t, registry, pluginDef.ToolRef, json.RawMessage(`{}`))
	require.True(t, pluginCall.Success, pluginCall.Error)

	require.Equal(t, int64(2), remote.Calls())
	require.Zero(t, remote.WriteCalls())
}
