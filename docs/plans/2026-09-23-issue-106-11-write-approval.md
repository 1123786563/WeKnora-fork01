# 插件 11｜启用写工具并由成员审批 实施计划（Issue #118）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 管理员对已启用写工具配置"调用时成员审批"后，每次调用前成员审阅确定的目标与输入，批准才派发；批准后仅一次派发；拒绝/超时/工具停用/插件停用后不派发；审批卡展示确定操作目标与参数；对话入口全链一致；不增加 Jira 写操作（用受控测试服务验收）。

**Architecture:** GAP-5 裁决（总索引）：复用既有 `approval.Gate` 链——`MCPTool.Execute` 在 `NeedsApproval` 为真时 `RequestAndWait`（mcp_tool.go:139-190），批准/拒绝/超时 Decision 已保证不派发。本切片交付：(1) `require_approval` 配置面（T18 已预留 `SetInstallationToolPolicy` 的 `requireApproval` 参数——本任务接通 `SetPolicy` 的 `RequireApproval` 字段落库 + 前端开关）；(2) 集成验收：真实 `approval.NewGate(cfg, &approval.Adapter{Svc: approvalService}, nil)`（container.go:470-473 生产装配同款）+ `ResolveToolApproval` HTTP 解析 pending（routes_infra.go:205 的 `/agent/tool-approvals/:pending_id`，Viewer+），断言审批卡 `PendingRequest` 的确定目标（`ServiceName`/`MCPToolName`/`Args`）与零写入。

**Tech Stack:** Go 1.26；`approval.Gate`/`DurableGate`（gate.go:230/durable_gate.go:41）；event bus；`//go:build integration` 真 PG。

**Spec:** `docs/specs/2026-09-23-self-hosted-plugins-spec.md`（User Story 22；Implementation Decisions 54 行；Testing Decisions 第 64 行）；ADR-0014（pending 一次性 CAS/取消终结——既有 Gate 实现满足，本切片只验收）。

**Issue:** https://github.com/1123786563/WeKnora-fork01/issues/118 （blocked_by #117 → 任务级前置 T18）

## Global Constraints

见总索引。不新建审批表、不改 Gate 语义；全部行为经配置 + 既有闸门组合达成。

## Review Focus（本切片）

- 批准后仅一次派发：同一 pending 二次 Resolve → 拒绝（一次性），外部写计数恰为 1（T19）。
- 拒绝/超时（构造短 `ToolApprovalTimeoutSeconds` cfg）/停用工具/停用插件 → 零派发、写计数 0、对话结果如实（T19）。
- 审批卡目标与参数 = 确定操作（工具名 + 参数原文），无"批准整个未来操作"语义（T19）。
- 未配置审批的只读工具不弹审批（噪声回归）（T19）。

---

### Task 19: 写工具成员审批闭环

**Files:**
- Modify: `internal/application/service/plugin_install_service.go`（`SetInstallationToolPolicy` 的 `requireApproval` 落库路径核验——透传 `MCPToolApprovalService.SetPolicy` 的 `requireApproval *bool` 参数，真实签名见 internal/types/interfaces/mcp_tool_approval.go:20）
- Modify: `packages/api-client/src/plugins.ts` / `plugins.test.ts`、`apps/web/src/settings/PluginsSettingsPanel.tsx` / `PluginsSettingsPanel.test.tsx`（工具策略面板加 `require_approval` 开关——与 T18 enabled 开关同行）
- Test: `internal/modules/plugins/write_approval_integration_test.go`（`//go:build integration`，真 PG）
- Test: `internal/handler/plugin_tool_policy_test.go`（如 T18 未覆盖 require_approval 的 handler 分支则在此补）

**Interfaces:**
- Consumes: T18 `SetInstallationToolPolicy(enabled, requireApproval)` 与 `PUT .../tools/:tool_name/policy` 请求体（`{"enabled": bool, "require_approval": bool}` 两字段均可选）；`approval.NewGate`；`handler.ResolveToolApproval`（pending_id 经 `EventBus` 待决队列暴露——集成测试从 event bus 捕获 `PendingRequest` 后经 handler 或 `Gate.Resolve` 解析，取与生产 HTTP 路径一致的方式：走 `POST /api/v1/agent/tool-approvals/:pending_id` 需要会话鉴权中间件——集成测试在 gin TestMode 下组装带身份注入的最小路由组复刻该路径的 handler 直调：`ResolveToolApproval` 以 `gin.CreateTestContext` + 上下文注入 tenant/principal 直调，等效 HTTP 语义）。
- Produces: #106 边界 5 的全部验收证据（T20 复用）；前端策略开关。

- [ ] **Step 1: 写失败测试（真 PG 全闭环）**

`write_approval_integration_test.go`：

```go
func newApprovalStack(t *testing.T) /* 真实 Gate + Adapter + approvalService + memberCtx */ {
	// approval.NewGate(&config.Config{Agent: &config.AgentConfig{ToolApprovalTimeoutSeconds: 2}},
	//                  &approval.Adapter{Svc: toolApprovalService}, nil)
}

func TestWriteApprovalApproveDispatchesOnce(t *testing.T) {
	// 安装（写工具 w 已由管理员 enabled=true + require_approval=true）：
	// 成员 call w → 阻塞等待审批；从 Gate 捕获 PendingRequest：
	//   断言 ServiceName/MCPToolName/Args 为确定目标与输入原文、ToolCallID 非空；
	// 经 ResolveToolApproval 语义批准 → 调用返回成功、受控服务 WriteCalls()==1；
	// 同 pending 二次 Resolve → 拒绝（一次性）且 WriteCalls() 仍为 1。
}

func TestWriteApprovalRejectTimeoutDisableZeroWrites(t *testing.T) {
	// 三场景各自新安装态：
	// a) 拒绝 → 调用返回拒绝文案、WriteCalls()==0；
	// b) 超时（cfg 2s，不 Resolve）→ TimedOut 决策、WriteCalls()==0；
	// c) 调用前管理员把 w enabled=false（或停用插件）→ 无审批直接拒绝、WriteCalls()==0。
}

func TestReadOnlyToolsDoNotPromptApproval(t *testing.T) {
	// 未配置审批的只读工具调用无 pending 产生、直接成功。
}
```

- [ ] **Step 2: 运行确认失败**

Run: `PLUGIN_TEST_DATABASE_URL=... go test ./internal/modules/plugins/ -tags integration -run 'TestWriteApproval|TestReadOnlyToolsDoNotPrompt' -v`
Expected: 无 DB Fatal；有 DB FAIL（require_approval 路径/测试未实现）。

- [ ] **Step 3: 实现接通**（`SetInstallationToolPolicy` 的 `requireApproval` 透传已有 patch 结构落库；前端开关；测试基建 `newApprovalStack`）。

- [ ] **Step 4: 运行确认通过**

Run: `PLUGIN_TEST_DATABASE_URL=... go test ./internal/modules/plugins/ -tags integration -run 'TestWriteApproval|TestReadOnlyToolsDoNotPrompt' -v && go test ./internal/modules/plugins/ -v`
Expected: PASS。

- [ ] **Step 5: 前端测试与实现**（`plugins.test.ts` 解析器 `require_approval` 字段；面板开关渲染与调用 stub）。

Run: `pnpm gates`（Expected 全过）

- [ ] **Step 6: Commit**

```bash
git add internal/application/ internal/modules/plugins/ internal/handler/ packages/api-client/src/plugins.ts packages/api-client/src/plugins.test.ts apps/web/src/settings/
git commit -m "feat(plugins): 写工具成员审批闭环（批准一次派发/拒绝超时停用零写入）[T19]"
```
