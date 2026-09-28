# 插件 04｜在对话中调用无账号插件工具 实施计划（Issue #111）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 成员在对话中经由 Agent 调用已安装插件版本内的无账号只读 MCP 工具并获得带来源标识的结果；Agent 只发现空间已安装、已启用且属于已接受版本的工具；未安装/停用/跨空间的调用被拒绝；成功/空/错误如实呈现；手工 MCP 服务行为不变。

**Architecture:** `tools.RegisterMCPTools` 新增第 9 参数 `pluginGuard PluginSnapshotProvider`（可空——nil 时行为与现状完全一致）：目录加载（`loadMCPDirectory` 结果消费处，mcp_tool.go:553-571 的 loader 闭包内）后对插件物化服务做快照过滤——快照外工具剔除、快照内工具 schema digest 不符返回 `ErrPluginDrift`。生产 provider 由 `PluginRepository` 按 `service_id` 反查安装快照构造（`service.PluginSnapshotLookup`）。GAP-5 裁决（总索引）：授权链走既有 `approval.Gate`，不接 ADR-0013 preflight。

**Tech Stack:** Go 1.26；`tools` 包现有测试基建（`catalogTestContext`、`discoverPage`、`describeTool`、`WithToolExecContext`，mcp_catalog_test.go:31-34 / mcp_catalog_integration_test.go:108-124）；受控 MCP 服务（sdkserver + httptest + SSRF 白名单）。

**Spec:** `docs/specs/2026-09-23-self-hosted-plugins-spec.md`（Solution 第 11 行；Implementation Decisions 52-53 行）；范围裁决 GAP-5 落实。

**Issue:** https://github.com/1123786563/WeKnora-fork01/issues/111 （blocked_by #110 → 任务级前置 T06）

## Global Constraints

见总索引。本切片修改共享热路径 `RegisterMCPTools`（生产调用点仅 agent_service.go:289 一处 + 既有测试）：任何改动必须保持 `pluginGuard == nil` 或服务非插件物化时的行为与现状逐字节一致（手工 MCP 兼容）。

## Review Focus（本切片）

- 快照外新增工具：远端悄悄加工具 → Agent 目录不可见（T09）。
- 快照内 schema 变：`ErrPluginDrift` 明确错误（T09）。
- 未安装/停用/跨空间：不可发现或调用拒绝（T09/T10）。
- `pluginGuard` 对手工服务返回 nil → 行为不变（T09）。
- guard 查询失败（DB 错误）：fail-closed——目录加载报错而非跳过过滤（T09）。

---

### Task 9: RegisterMCPTools 快照守卫注入

**Files:**
- Modify: `internal/modules/agentruntime/agent/tools/mcp_tool.go`（`RegisterMCPTools` 签名 + loader 闭包内过滤；新增类型与错误）
- Modify: `internal/application/service/agent_service.go`（registerMCPTools 注入 provider，289 行调用点）
- Modify: `internal/container/container.go` 或 `internal/application/service/agent_service.go` 构造（把 `PluginSnapshotLookup` 装配进 agentService——以 agentService 现有构造参数与 container 注册方式为准，执行时 `grep -n "NewAgentService(" internal/` 定位）
- Test: `internal/modules/agentruntime/agent/tools/mcp_plugin_guard_test.go`

**Interfaces:**
- Consumes: T06 的 `types.PluginInstallation`/`types.PluginToolSnapshot`/`PluginRepository.GetByServiceID`；`RegisterMCPTools` 现签名（mcp_tool.go:539-548）。
- Produces（T10/T13/T16-T19 依赖）:
  - `tools.PluginRuntimeSnapshot struct{ InstallationID string; Tools []types.PluginToolSnapshot }`
  - `type tools.PluginSnapshotProvider func(ctx context.Context, tenantID uint64, serviceID string) (*PluginRuntimeSnapshot, error)`（nil 返回值 = 非插件服务；error = fail-closed）
  - `var tools.ErrPluginDrift = errors.New("plugin capability drift detected; administrator review required")`
  - `tools.FilterToolsBySnapshot(snap *PluginRuntimeSnapshot, defs []*types.MCPTool) ([]*types.MCPTool, error)`（导出以便单测与 T17 复用）
  - `tools.RegisterMCPTools(ctx, registry, services, mcpManager, gate, authWaitTimeoutSeconds, lookup, metadata, pluginGuard PluginSnapshotProvider) (int, error)`
  - `service.PluginSnapshotLookup(repo interfaces.PluginRepository) tools.PluginSnapshotProvider`（生产 provider：`repo.GetByServiceID(ctx, tenantID, serviceID)` → 未找到返回 `(nil, nil)`；找到则反序列化 `ToolsSnapshot`）
  - `PluginRepository` 扩展：`GetByServiceID(ctx context.Context, tenantID uint64, serviceID string) (*types.PluginInstallation, error)`

- [ ] **Step 1: 写失败测试**

`internal/modules/agentruntime/agent/tools/mcp_plugin_guard_test.go`（组装先例 mcp_catalog_integration_test.go:34-105：sdkserver + httptest + `proxyApprovalGate` 风格替身 + `catalogTestContext`）：

```go
package tools

func TestRegisterMCPToolsFiltersPluginToolsToAcceptedSnapshot(t *testing.T) {
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)
	// 受控服务暴露 2 工具：snapshot_tool（快照内、schema 匹配）、extra_tool（快照外）。
	// guard 返回快照 [snapshot_tool]。
	// 断言：list_tools 只见 snapshot_tool；describe→call 成功且结果带 [MCP service ... (external)] 来源前缀。
}

func TestRegisterMCPToolsRejectsSchemaDrift(t *testing.T) {
	// 受控服务的 snapshot_tool schema 与快照 digest 不符：
	// list_tools 返回错误，文案含 "plugin capability drift detected"（ErrPluginDrift）。
}

func TestRegisterMCPToolsNilGuardKeepsLegacyBehavior(t *testing.T) {
	// pluginGuard = nil：服务暴露 2 工具（无快照）→ list_tools 全部可见（现状语义）。
	// 再构造一行手工服务（PluginInstallationID nil）+ guard 返回 (nil,nil) → 同样全部可见。
}

func TestRegisterMCPToolsGuardErrorFailsClosed(t *testing.T) {
	// guard 返回错误 → list_tools 报错（不回退为不过滤）。
}

func TestFilterToolsBySnapshot(t *testing.T) {
	// 纯函数：快照外剔除；digest 不符 → ErrPluginDrift；快照内多余 live 工具剔除后保序。
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/agentruntime/agent/tools/ -run 'TestRegisterMCPTools.*Plugin|TestFilterToolsBySnapshot' -v`
Expected: FAIL —— `PluginSnapshotProvider`/`FilterToolsBySnapshot` 未定义（编译错误）。

- [ ] **Step 3: 实现**

`mcp_tool.go`：新增类型/错误/`FilterToolsBySnapshot`（对每个 def：找快照同名项——找不到则丢弃；`plugins.ToolSchemaDigest(def.InputSchema) != snap.InputSchemaDigest` 则包 `ErrPluginDrift` 错误 `fmt.Errorf("%w: tool %q schema changed", ErrPluginDrift, def.Name)`；保序返回命中项）。`RegisterMCPTools` loader 闭包在 `loadMCPDirectory` 返回后：

```go
if pluginGuard != nil {
	tenant, _ := types.TenantIDFromContext(loadCtx)
	snap, guardErr := pluginGuard(loadCtx, tenant, service.ID)
	if guardErr != nil {
		return nil, fmt.Errorf("plugin snapshot guard failed for %s: %w", service.Name, guardErr)
	}
	if snap != nil {
		filtered, filterErr := FilterToolsBySnapshot(snap, definitions)
		if filterErr != nil { return nil, filterErr }
		definitions = filtered
	}
}
```

`agent_service.go`：agentService 构造/字段加 `pluginSnapshotProvider tools.PluginSnapshotProvider`（可空），`registerMCPTools` 第 9 参传 `s.pluginSnapshotProvider`；container 装配 `service.PluginSnapshotLookup(pluginRepo)`。既有 `RegisterMCPTools` 测试调用点补 `nil` 参数（`grep -rn "RegisterMCPTools(" internal/ --include="*_test.go"`）。

- [ ] **Step 4: 运行确认通过 + 回归**

Run: `go test ./internal/modules/agentruntime/agent/tools/ -run 'TestRegisterMCPTools|TestFilterToolsBySnapshot|TestMCPProxy' -v && go test ./internal/modules/airesource/mcp/ -count=1 && go build ./...`
Expected: 新测试 PASS；既有 MCP 测试（含 TestMCPProxyHTTPApprovalArgumentsAndImages）不回归。

- [ ] **Step 5: Commit**

```bash
git add internal/modules/agentruntime/agent/tools/ internal/application/service/ internal/container/ internal/types/interfaces/ internal/application/repository/
git commit -m "feat(plugins): Agent 目录按已接受版本快照过滤并拒绝漂移 [T09]"
```

---

### Task 10: 无账号工具应用边界集成测试（受控服务基建）

**Files:**
- Create: `internal/modules/plugins/plugintest/server.go`（受控插件替身，包 `plugintest`——非 `_test.go`，供 T13/T19 复用；文件头注释声明仅供测试使用）
- Create: `internal/modules/plugins/plugintest/server_test.go`（基建自测）
- Test: `internal/modules/plugins/plugin_agent_integration_test.go`

**Interfaces:**
- Consumes: T09 的 `RegisterMCPTools`+`PluginSnapshotProvider`；T04 契约（目录公开/执行鉴权）。
- Produces（T13/T16/T17/T18/T19 复用）:
  - `plugintest.Server`：`Start(t testing.TB) (baseURL string)`（httptest 挂 `/manifest.json` + `/mcp`；自动设 SSRF 白名单 + cleanup）
  - `plugintest.Server.SetTools(tools []plugintest.ToolSpec)`（运行时可改目录——漂移/升级场景）
  - `plugintest.ToolSpec{Name string; ReadOnly bool; Description string; Schema string}`（Schema 默认无参固定 schema）
  - `plugintest.Server.Manifest() *types.PluginManifest`（与当前目录同步的合法清单，digest 由 `plugins.ToolSchemaDigest` 计算）
  - `plugintest.Server.Calls(toolName string) int` / `WriteCalls() int`（调用计数；`ReadOnly=false` 的工具调用计入 WriteCalls）
  - `plugintest.Server.ManifestHandler_mutate(fn func(*types.PluginManifest))`（清单版本/端点改写钩子，升级差异场景）

- [ ] **Step 1: 写失败测试（应用边界场景）**

`internal/modules/plugins/plugin_agent_integration_test.go`（`package plugins_test`；组 `RegisterMCPTools` 全链，fake snapshot provider = 安装快照闭包）：

```go
func TestAgentCallsNoAccountPluginToolEndToEnd(t *testing.T) {
	// 受控服务 1 只读无账号工具 → RegisterMCPTools（guard=安装快照）→
	// list_tools → describe → call_mcp_tool 成功，结果含来源前缀与外呼计数 +1。
}

func TestEmptyAndErrorResultsSurfaceAsIs(t *testing.T) {
	// 受控工具分别返回空文本与 isError 结果 → ToolResult.Success=false 且文案如实（不虚构）。
}

func TestUninstalledOrDisabledOrForeignTenantRejected(t *testing.T) {
	// a) 服务列表不含该安装（未安装）→ list_servers 不出现；
	// b) 安装 disabled（物化服务 Enabled=false）→ 不可见；
	// c) tenant 2 的 ctx 调 tenant 1 的 registry → lookup 空。
}

func TestManualMCPServiceCoexists(t *testing.T) {
	// 同一 registry 同时挂 1 手工服务（无 guard 命中）+ 1 插件安装 →
	// 手工服务全部工具可见可调（现状语义），插件服务仅快照内可见。
}
```

`plugintest/server_test.go`：`TestControlledServerServesManifestAndTools`（清单合法 + digest 与目录一致 + 计数准确）。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/plugins/ -run 'TestAgentCalls|TestEmptyAndError|TestUninstalledOrDisabled|TestManualMCPServiceCoexists|TestControlledServer' -v`
Expected: FAIL —— `plugintest` 包与测试未定义。

- [ ] **Step 3: 实现 plugintest 与集成测试**

`plugintest` 用 `sdkserver.NewMCPServer` + `NewStreamableHTTPServer(WithStateLess(true))`（先例 mcp_catalog_integration_test.go:47-62）；`SetTools` 重建 server handler 挂到同一 mux（目录动态）；清单路由每次请求由 `Manifest()` 现算。集成测试按 mcp_catalog_integration_test.go 的 `discoverPage`/`describeTool` 辅助复制本包版本（无法跨包复用未导出辅助——在测试文件内定义同款小助手 `discover(t, registry, args)`）。

- [ ] **Step 4: 运行确认通过 + Commit**

Run: `go test ./internal/modules/plugins/ -v && go build ./...`
Expected: PASS。
```bash
git add internal/modules/plugins/
git commit -m "test(plugins): 无账号插件工具应用边界集成测试与受控服务基建 [T10]"
```
