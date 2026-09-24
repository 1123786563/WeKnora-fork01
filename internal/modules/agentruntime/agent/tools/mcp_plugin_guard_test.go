package tools

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	internalmcp "github.com/Tencent/WeKnora/internal/modules/airesource/mcp"
	"github.com/Tencent/WeKnora/internal/modules/plugins"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
	sdkmcp "github.com/mark3labs/mcp-go/mcp"
	sdkserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"
)

// guardNoArgSchema 是受控工具的输入 schema：快照 digest 由
// plugins.ToolSchemaDigest 现算，与运行时过滤用同一口径。
const guardNoArgSchema = `{"type":"object","properties":{},"additionalProperties":false}`
const guardQuerySchema = `{"type":"object","properties":{"q":{"type":"string"}},"required":["q"],"additionalProperties":false}`

// guardControlledService 起一个 sdkserver 受控 MCP 服务（目录 = 给定工具
// 名单，schema 固定无参），返回物化服务行与外呼计数。组装先例：
// mcp_catalog_integration_test.go TestMCPProxyHTTPApprovalArgumentsAndImages。
func guardControlledService(t *testing.T, toolNames ...string) (*types.MCPService, *atomic.Int32) {
	t.Helper()
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)

	server := sdkserver.NewMCPServer("guard-test", "1", sdkserver.WithToolCapabilities(false))
	var requests atomic.Int32
	for _, name := range toolNames {
		toolName := name
		// NewToolWithRawSchema 钉死 schema 字节：快照 digest 与 live digest
		// 因此按构造一致（漂移场景另行改写快照 digest 制造不一致）。
		server.AddTool(
			sdkmcp.NewToolWithRawSchema(toolName, "desc", json.RawMessage(guardNoArgSchema)),
			func(_ context.Context, _ sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
				return sdkmcp.NewToolResultText(toolName + " ok"), nil
			},
		)
	}
	transport := sdkserver.NewStreamableHTTPServer(server, sdkserver.WithStateLess(true))
	httpServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, request *http.Request) {
			requests.Add(1)
			transport.ServeHTTP(w, request)
		},
	))
	t.Cleanup(httpServer.Close)

	service := &types.MCPService{
		ID:            "plugin-guard-svc",
		TenantID:      7,
		Enabled:       true,
		Name:          "plugin:com.example.guard",
		URL:           &httpServer.URL,
		TransportType: types.MCPTransportHTTPStreamable,
	}
	installationID := "inst-guard-1"
	service.PluginInstallationID = &installationID
	return service, &requests
}

func guardSnapshot(toolNames ...string) *PluginRuntimeSnapshot {
	toolsSnapshot := make([]types.PluginToolSnapshot, 0, len(toolNames))
	for _, name := range toolNames {
		toolsSnapshot = append(toolsSnapshot, types.PluginToolSnapshot{
			Name:              name,
			Description:       "desc",
			InputSchemaDigest: plugins.ToolSchemaDigest([]byte(guardNoArgSchema)),
			ReadOnly:          true,
		})
	}
	return &PluginRuntimeSnapshot{InstallationID: "inst-guard-1", Tools: toolsSnapshot}
}

// discoverToolResult 执行一次 list_tools 并返回原始 ToolResult——漂移/
// 守卫失败场景断言 Success=false 与错误文案（discoverPage 会 assert 成功，
// 不适用于 fail-closed 断言）。
func discoverToolResult(t *testing.T, r *ToolRegistry, serverID string) *types.ToolResult {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"mode": "list_tools", "server_id": serverID})
	require.NoError(t, err)
	result, err := r.ExecuteTool(catalogTestContext(), ToolDiscoverMCPTools, raw)
	require.NoError(t, err)
	return result
}

func TestFilterToolsBySnapshot(t *testing.T) {
	snap := guardSnapshot("snapshot_tool")
	defs := []*types.MCPTool{
		{Name: "snapshot_tool", InputSchema: json.RawMessage(guardNoArgSchema)},
		{Name: "extra_tool", InputSchema: json.RawMessage(guardNoArgSchema)},
	}

	// 快照外剔除、快照内保序返回。
	filtered, err := FilterToolsBySnapshot(snap, defs)
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	require.Equal(t, "snapshot_tool", filtered[0].Name)

	// 快照内 schema 变 → ErrPluginDrift（明确错误，不静默剔除）。
	drifted := []*types.MCPTool{
		{Name: "snapshot_tool", InputSchema: json.RawMessage(guardQuerySchema)},
	}
	_, err = FilterToolsBySnapshot(snap, drifted)
	require.ErrorIs(t, err, ErrPluginDrift)
	require.Contains(t, err.Error(), "snapshot_tool")

	// 空目录 → 空结果非错误。
	filtered, err = FilterToolsBySnapshot(snap, nil)
	require.NoError(t, err)
	require.Empty(t, filtered)
}

func TestRegisterMCPToolsFiltersPluginToolsToAcceptedSnapshot(t *testing.T) {
	service, requests := guardControlledService(t, "snapshot_tool", "extra_tool")
	guard := PluginSnapshotProvider(func(_ context.Context, tenantID uint64, serviceID string) (*PluginRuntimeSnapshot, error) {
		require.Equal(t, uint64(7), tenantID)
		require.Equal(t, "plugin-guard-svc", serviceID)
		return guardSnapshot("snapshot_tool"), nil
	})

	ctx := catalogTestContext()
	registry := NewToolRegistry()
	manager := internalmcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	_, err := RegisterMCPTools(ctx, registry, []*types.MCPService{service}, manager, &proxyApprovalGate{}, 0, nil, nil, guard)
	require.NoError(t, err)

	// 快照外工具对 Agent 目录不可见。
	page := discoverPage(ctx, t, registry, map[string]any{"mode": "list_tools", "server_id": service.ID})
	require.Len(t, page.Tools, 1)
	require.Equal(t, "snapshot_tool", page.Tools[0].Name)
	require.Positive(t, requests.Load())
}

func TestRegisterMCPToolsRejectsSchemaDrift(t *testing.T) {
	// 受控服务的 snapshot_tool schema 与快照 digest 不符 → 目录加载报
	// ErrPluginDrift（fail-closed，不回退为不过滤）。
	//
	// catalog 既有设计把 loader 错误折叠为确定性 error 态消息（漂移原文
	// 经 TestFilterToolsBySnapshot 纯函数断言），因此此处用「控制孪生」
	// 排除连接失败假阳性：同一受控 server 挂两个服务行——control 用匹配
	// 快照（能列出 → 连通性成立），drift 用不匹配快照（必须失败）。
	// 若失败源于连接，control 也会失败，测试即红。
	service, _ := guardControlledService(t, "snapshot_tool")
	control := *service
	control.ID = "plugin-guard-control"
	driftedSnap := &PluginRuntimeSnapshot{
		InstallationID: "inst-guard-1",
		Tools: []types.PluginToolSnapshot{{
			Name:              "snapshot_tool",
			InputSchemaDigest: plugins.ToolSchemaDigest([]byte(guardQuerySchema)),
			ReadOnly:          true,
		}},
	}
	guard := PluginSnapshotProvider(func(_ context.Context, _ uint64, serviceID string) (*PluginRuntimeSnapshot, error) {
		if serviceID == control.ID {
			return guardSnapshot("snapshot_tool"), nil
		}
		return driftedSnap, nil
	})

	ctx := catalogTestContext()
	registry := NewToolRegistry()
	manager := internalmcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	_, err := RegisterMCPTools(ctx, registry, []*types.MCPService{service, &control}, manager, &proxyApprovalGate{}, 0, nil, nil, guard)
	require.NoError(t, err)

	// 控制孪生先证明连通性与守卫放行：匹配快照可列出。
	controlPage := discoverPage(ctx, t, registry, map[string]any{"mode": "list_tools", "server_id": control.ID})
	require.Len(t, controlPage.Tools, 1)

	// 漂移行 fail-closed：发现失败（对外折叠为 error 态消息），不回退为
	// 不过滤——若目录未守卫，此处会列出 1 个工具而非失败。
	result := discoverToolResult(t, registry, service.ID)
	require.False(t, result.Success)
	require.Contains(t, result.Error, "is error; retry discovery")
}

func TestRegisterMCPToolsNilGuardKeepsLegacyBehavior(t *testing.T) {
	// nil guard：全部工具可见（现状语义，手工 MCP 兼容的字节级前提）。
	service, _ := guardControlledService(t, "tool_a", "tool_b")
	ctx := catalogTestContext()
	registry := NewToolRegistry()
	manager := internalmcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	_, err := RegisterMCPTools(ctx, registry, []*types.MCPService{service}, manager, &proxyApprovalGate{}, 0, nil, nil, nil)
	require.NoError(t, err)
	page := discoverPage(ctx, t, registry, map[string]any{"mode": "list_tools", "server_id": service.ID})
	require.Len(t, page.Tools, 2)

	// guard 命中非插件服务（(nil,nil)）→ 行为与现状一致。
	manual := &types.MCPService{
		ID: "manual-svc", TenantID: 7, Enabled: true, Name: "manual",
		URL: service.URL, TransportType: types.MCPTransportHTTPStreamable,
	}
	registry2 := NewToolRegistry()
	manualGuard := PluginSnapshotProvider(func(_ context.Context, _ uint64, _ string) (*PluginRuntimeSnapshot, error) {
		return nil, nil
	})
	_, err = RegisterMCPTools(ctx, registry2, []*types.MCPService{manual}, manager, &proxyApprovalGate{}, 0, nil, nil, manualGuard)
	require.NoError(t, err)
	page2 := discoverPage(ctx, t, registry2, map[string]any{"mode": "list_tools", "server_id": manual.ID})
	require.Len(t, page2.Tools, 2)
}

func TestRegisterMCPToolsGuardErrorFailsClosed(t *testing.T) {
	// 守卫故障 fail-closed。控制孪生先证明同一 server/守卫框架下匹配
	// 快照可列出（排除连接失败假阳性），erroring 守卫的目录必须失败。
	service, _ := guardControlledService(t, "snapshot_tool")
	control := *service
	control.ID = "plugin-guard-control"
	guard := PluginSnapshotProvider(func(_ context.Context, _ uint64, serviceID string) (*PluginRuntimeSnapshot, error) {
		if serviceID == control.ID {
			return guardSnapshot("snapshot_tool"), nil
		}
		return nil, errors.New("plugin repo down")
	})

	ctx := catalogTestContext()
	registry := NewToolRegistry()
	manager := internalmcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	_, err := RegisterMCPTools(ctx, registry, []*types.MCPService{service, &control}, manager, &proxyApprovalGate{}, 0, nil, nil, guard)
	require.NoError(t, err)

	controlPage := discoverPage(ctx, t, registry, map[string]any{"mode": "list_tools", "server_id": control.ID})
	require.Len(t, controlPage.Tools, 1)

	// 守卫故障的目录失败（对外折叠为 error 态消息），不回退为不过滤。
	result := discoverToolResult(t, registry, service.ID)
	require.False(t, result.Success)
	require.Contains(t, result.Error, "is error; retry discovery")
}
