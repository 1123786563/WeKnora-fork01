// plugintest 基建自测（计划 Task 10 Step 1）：受控替身的清单合法性、
// digest 与 live 目录一致性（经 plugins.FetchAndVerify 全链核验）、
// 读写调用计数准确性、SetTools 运行时改目录与 ManifestHandler_mutate
// 清单改写钩子。本文件仅供测试使用。
package plugintest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	internalmcp "github.com/Tencent/WeKnora/internal/modules/airesource/mcp"
	"github.com/Tencent/WeKnora/internal/modules/plugins"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// selfTestNoArgSchema 与 plugintest.Tool.InputSchema 的固定无参 schema 一致。
const selfTestNoArgSchema = `{"type":"object","properties":{},"additionalProperties":false}`

// selfTestLister 经真实 MCPManager 客户端栈做 live ListTools（与生产
// container.NewPluginMCPEndpointLister 同构的测试内版本：每核验一次独立
// service ID + 用后即收，不复用连接）。
func selfTestLister(t *testing.T, manager *internalmcp.MCPManager) plugins.EndpointLister {
	t.Helper()
	var seq int
	return func(ctx context.Context, transportType, endpointURL string) ([]*types.MCPTool, error) {
		seq++
		id := fmt.Sprintf("plugintest-self-list-%d", seq)
		endpoint := endpointURL
		service := &types.MCPService{
			ID:            id,
			Enabled:       true,
			Name:          "plugintest-self",
			TransportType: types.MCPTransportType(transportType),
			URL:           &endpoint,
		}
		client, err := manager.GetOrCreateClient(ctx, service)
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = client.Disconnect()
			_ = manager.CloseClient(id)
		}()
		return client.ListTools(ctx)
	}
}

// selfTestCall 经真实 MCPManager 客户端调用一次受控工具（无 OAuth：替身
// 目录公开、执行鉴权仅对启用 OAuth 的替身生效）。
func selfTestCall(t *testing.T, manager *internalmcp.MCPManager, remote *Server, toolName string) *internalmcp.CallToolResult {
	t.Helper()
	id := "plugintest-self-call-" + toolName
	endpoint := remote.BaseURL() + "/mcp"
	service := &types.MCPService{
		ID:            id,
		Enabled:       true,
		Name:          "plugintest-self",
		TransportType: types.MCPTransportHTTPStreamable,
		URL:           &endpoint,
	}
	client, err := manager.GetOrCreateClient(context.Background(), service)
	require.NoError(t, err)
	defer func() {
		_ = client.Disconnect()
		_ = manager.CloseClient(id)
	}()
	result, err := client.CallTool(context.Background(), toolName, map[string]interface{}{})
	require.NoError(t, err)
	return result
}

// fetchSelfTestManifest 直接 GET /manifest.json 并解码（对 HTTP 面而非
// Manifest() 方法做断言——改写钩子必须影响线上序列化结果）。
func fetchSelfTestManifest(t *testing.T, remote *Server) *types.PluginManifest {
	t.Helper()
	resp, err := http.Get(remote.ManifestURL())
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var manifest types.PluginManifest
	require.NoError(t, json.Unmarshal(body, &manifest))
	return &manifest
}

func TestControlledServerServesManifestAndTools(t *testing.T) {
	remote := New()
	readTool := Tool{Name: "read_demo", Description: "只读演示", ReadOnly: true, InputSchema: selfTestNoArgSchema}
	writeTool := Tool{Name: "write_demo", Description: "写演示", ReadOnly: false, InputSchema: selfTestNoArgSchema}
	remote.SetTools([]Tool{readTool, writeTool})
	remote.Start(t)

	manager := internalmcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	lister := selfTestLister(t, manager)

	// 1) /manifest.json 是合法清单，且声明 digest 与 live 目录一致
	//    （FetchAndVerify：SSRF 门 + 协议校验 + 实时 ListTools 比对）。
	verified, err := plugins.FetchAndVerify(context.Background(), remote.ManifestURL(), lister)
	require.NoError(t, err)
	require.Equal(t, "com.example.plugintest", verified.Manifest.PluginID)
	require.Equal(t, "1.0.0", verified.Manifest.Version)
	require.Equal(t, remote.BaseURL()+"/mcp", verified.Manifest.Transport.Endpoint)
	names := make([]string, 0, len(verified.Snapshot))
	readOnlyByName := map[string]bool{}
	for _, snap := range verified.Snapshot {
		names = append(names, snap.Name)
		readOnlyByName[snap.Name] = snap.ReadOnly
	}
	require.Equal(t, []string{"read_demo", "write_demo"}, names)
	require.True(t, readOnlyByName["read_demo"])
	require.False(t, readOnlyByName["write_demo"])
	// 权威快照 digest 从 live schema 现算——与固定无参 schema 的口径一致。
	noArgDigest := plugins.ToolSchemaDigest([]byte(selfTestNoArgSchema))
	for _, decl := range verified.Manifest.Tools {
		require.Equal(t, noArgDigest, decl.InputSchemaDigest)
	}

	// 2) 调用计数准确：读调用只进 Calls，写调用只进 WriteCalls。
	readResult := selfTestCall(t, manager, remote, "read_demo")
	require.False(t, readResult.IsError)
	require.EqualValues(t, 1, remote.Calls())
	require.Zero(t, remote.WriteCalls())
	writeResult := selfTestCall(t, manager, remote, "write_demo")
	require.False(t, writeResult.IsError)
	require.EqualValues(t, 1, remote.Calls(), "write calls must not inflate the read counter")
	require.EqualValues(t, 1, remote.WriteCalls())

	// 3) SetTools 运行时改目录后，清单与 live 目录保持同步（漂移/升级场景的前提）。
	upgraded := []Tool{
		readTool,
		{Name: "new_tool", Description: "升级新增", ReadOnly: true, InputSchema: selfTestNoArgSchema},
	}
	remote.SetTools(upgraded)
	verified2, err := plugins.FetchAndVerify(context.Background(), remote.ManifestURL(), lister)
	require.NoError(t, err)
	names2 := make([]string, 0, len(verified2.Snapshot))
	for _, snap := range verified2.Snapshot {
		names2 = append(names2, snap.Name)
	}
	require.Equal(t, []string{"read_demo", "new_tool"}, names2)

	// 4) ManifestHandler_mutate：清单改写钩子（版本/端点差异场景）——
	//    只改清单声明，不动 live 目录；digest 校验必须仍然通过。
	remote.ManifestHandler_mutate(func(m *types.PluginManifest) {
		m.Version = "2.0.0"
	})
	served := fetchSelfTestManifest(t, remote)
	require.Equal(t, "2.0.0", served.Version)
	require.Len(t, served.Tools, 2)
	_, err = plugins.FetchAndVerify(context.Background(), remote.ManifestURL(), lister)
	require.NoError(t, err, "a version-only manifest mutation must keep the directory digest consistent")
}
