# 插件 10｜新增写工具默认关闭 实施计划（Issue #117）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 管理员接受带新写工具的版本后，写工具仍关闭直到逐项启用：安装/升级后新写工具默认不可调用，成员授权或 Agent 选择不能绕过；管理员可见关闭原因并逐项启用；只读工具按原策略；用受控测试服务确认关闭期间外部写入计数为零；不改变手工 MCP 服务的既有默认语义。

**Architecture:** 默认关闭的写入在 T06（安装）与 T16（升级/漂移 resolve）已落行（写工具 `MCPToolApproval.Enabled=false` 显式行）；运行时核对复用 `MCPTool.Execute` 的 `gate.IsEnabled`（mcp_tool.go:114-133——调用时重查，注册期绕过无效）。本切片交付治理面：`GET /api/v1/plugins/installations/:id/tools`（Viewer 可读目录+策略+关闭原因；Admin 同一端点）与 `PUT /api/v1/plugins/installations/:id/tools/:tool_name/policy`（Admin：`{"enabled": bool}`，本任务只开 `enabled`；`require_approval` 留给 T19 在同一端点扩展）。关闭原因 = 快照 `read_only=false` 且 `Enabled=false` → `"write tool disabled by default; enable explicitly"`。

**Tech Stack:** Go 1.26；gin；`//go:build integration` 真 PG + `plugintest` 写计数。

**Spec:** `docs/specs/2026-09-23-self-hosted-plugins-spec.md`（User Stories 14/15；Implementation Decisions 54 行）；基线勘察风险 1（MCPToolApproval 缺行默认启用与 Spec 冲突——处置：插件域显式行，缺省语义保留给手工服务）。

**Issue:** https://github.com/1123786563/WeKnora-fork01/issues/117 （blocked_by #116 → 任务级前置 T17）

## Global Constraints

见总索引。**不得修改** `types.MCPToolApproval.Enabled` 的 gorm 默认值或 `MCPToolApprovalService` 对缺行的判定（types/mcp.go:148-152）——手工服务兼容；插件域关闭全部通过显式行实现。

## Review Focus（本切片）

- Agent 在写工具关闭时直接 `call_mcp_tool` → 拒绝且外部写计数 0（T18）。
- 成员完成个人授权后写工具仍关闭（授权≠启用）（T18）。
- 管理员启用后写工具可调（开启路径正向可用）（T18）。
- 只读工具在安装/升级后立即可用（原策略）（T18）。
- 手工服务无行工具默认启用语义不变（回归断言）（T18）。

---

### Task 18: 写工具默认关闭治理与零外部写入验收

**Files:**
- Modify: `internal/types/interfaces/plugin.go`、`internal/application/service/plugin_install_service.go`（`ListInstallationTools`/`SetInstallationToolPolicy`）
- Modify: `internal/handler/dto/plugin.go`（`PluginInstallationTool` 补齐 `Enabled bool`/`DisabledReason string`/`RequireApproval bool` 字段——T07 详情 DTO 已有 `Enabled *bool`，统一为确定值：快照工具无行时视为 Enabled=仅当只读）
- Modify: `internal/handler/plugin.go`、`internal/router/routes_plugins.go`
- Test: `internal/modules/plugins/tool_policy_test.go`（fake 层）
- Test: `internal/modules/plugins/plugin_write_tools_integration_test.go`（`//go:build integration`）

**Interfaces:**
- Consumes: T06/T16 策略行；`interfaces.MCPToolApprovalService.ListByService/SetPolicy/IsEnabled`——`SetPolicy` 真实签名为 `SetPolicy(ctx context.Context, tenantID uint64, serviceID, toolName string, requireApproval, enabled *bool) error`（internal/types/interfaces/mcp_tool_approval.go:20；本任务 enabled 与 requireApproval 各自为独立可空指针，与该签名天然对齐）。
- Produces:
  - `interfaces.PluginService.ListInstallationTools(ctx, tenantID uint64, installationID string) ([]dto.PluginInstallationTool, error)`
  - `interfaces.PluginService.SetInstallationToolPolicy(ctx, tenantID uint64, installationID, toolName string, enabled *bool, requireApproval *bool) ([]dto.PluginInstallationTool, error)`（T19 消费 requireApproval；本任务 enabled 生效，requireApproval 参数已接收并透传 `SetPolicy`——两者同一 patch 结构，避免 T19 改签名）
  - `dto.PluginInstallationTool{Name, Description string; ReadOnly, RequiresPersonalAuth bool; Scopes []string; Enabled bool; RequireApproval bool; DisabledReason string}`
  - 路由：`GET /api/v1/plugins/installations/:id/tools`（Viewer+）、`PUT /api/v1/plugins/installations/:id/tools/:tool_name/policy`（Admin）

- [ ] **Step 1: 写失败测试（fake 层）**

`tool_policy_test.go`：

```go
func TestListInstallationToolsShowsDisabledReason(t *testing.T) {
	// 安装（1 只读 + 1 写）→ 列表：只读 Enabled=true 无 reason；写 Enabled=false、
	// DisabledReason="write tool disabled by default; enable explicitly"。
}

func TestSetInstallationToolPolicyTogglesWriteTool(t *testing.T) {
	// PUT enabled=true → 行更新、列表反映；再关闭 → 回到默认 reason。
	// 未知工具名 → 错误；非本租户安装 → not found。
}

func TestListInstallationToolsDropsRemovedSnapshotTools(t *testing.T) {
	// 升级/漂移 resolve 后快照移除工具 X（其旧 MCPToolApproval 行残留 Enabled=true）：
	// ListInstallationTools 以快照为源 → X 不出现在列表（残留行不复活已移除工具）；
	// Agent 目录同样不见 X（T09 快照过滤）。
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/plugins/ -run 'TestListInstallationTools|TestSetInstallationToolPolicy' -v`
Expected: FAIL —— 未定义。

- [ ] **Step 3: 实现**（`ListInstallationTools`：快照为源 join `MCPToolApprovalService.ListByService`——无行按只读默认；`SetInstallationToolPolicy`：工具必须在快照内，patch 透传 `SetPolicy`）。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/modules/plugins/ -run 'TestListInstallationTools|TestSetInstallationToolPolicy' -v && go build ./...`
Expected: PASS。

- [ ] **Step 5: 写失败测试（真 PG 应用边界：零外部写入 + 双重绕过尝试 + 手工回归）**

`plugin_write_tools_integration_test.go`：

```go
func TestUpgradedWriteToolStaysDisabledAndZeroExternalWrites(t *testing.T) {
	// v1（只读 r）安装 → 升级 v2（新增写 w，受控服务 WriteCalls 计数）：
	// 1) 成员（含已授权成员——用 OAuth 安装的插件或无账号写工具二选一：用无账号写工具，
	//    受控服务 w.read_only=false 且无 personal auth）目录可见 w（describe 成功）但 call →
	//    拒绝文案（disabled），WriteCalls()==0。
	// 2) 管理员启用 w → 成员 call 成功，WriteCalls()==1。
	// 3) 只读 r 全程可调。
}

func TestMemberAuthorizationDoesNotEnableWriteTool(t *testing.T) {
	// 插件声明 personal_oauth；成员完成授权（connections/me authorized）后：
	// 写工具（requires_personal_auth=true, read_only=false）call 仍拒绝、写计数 0。
}

func TestManualDefaultSemanticsUnchanged(t *testing.T) {
	// 手工服务加 1 工具（无 approval 行）→ Agent 目录可见可调（Enabled 缺省 true）。
	// 断言不因插件策略设施引入而改变。
}
```

- [ ] **Step 6: 运行确认失败**

Run: `PLUGIN_TEST_DATABASE_URL=... go test ./internal/modules/plugins/ -tags integration -run 'TestUpgradedWriteTool|TestMemberAuthorizationDoesNotEnable|TestManualDefault' -v`
Expected: 无 DB Fatal；有 DB FAIL。

- [ ] **Step 7: 补全，运行通过**

Run: `PLUGIN_TEST_DATABASE_URL=... go test ./internal/modules/plugins/ -tags integration -run 'TestUpgradedWriteTool|TestMemberAuthorizationDoesNotEnable|TestManualDefault' -v && go test ./internal/modules/plugins/ -v`
Expected: PASS。

- [ ] **Step 8: Commit**

```bash
git add internal/types/ internal/modules/plugins/ internal/application/ internal/handler/ internal/router/
git commit -m "feat(plugins): 写工具默认关闭治理与零外部写入验收 [T18]"
```
