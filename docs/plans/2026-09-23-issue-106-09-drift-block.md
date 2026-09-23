# 插件 09｜阻止已接受版本的远端能力漂移 实施计划（Issue #116）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 远端在已接受端点改变工具目录或 schema 时，Agent 拒绝未经审阅的能力：新增/移除/schema 变更形成明确漂移状态，调用被阻止并提示管理员复审；从成员对话观察拒绝；其他空间不受影响；手工 MCP 服务不被静默施加版本治理；提供管理员复审后的处置闭环（GAP-6）。

**Architecture:** 运行时阻断已由 T09 提供（快照外剔除 + `ErrPluginDrift`）。本切片补齐**持久化漂移状态与处置闭环**：`PluginService.CheckDrift`（Admin 触发：漂移核验的远端真相来源为 **`EndpointLister` 直连 `installation.endpoint_url` 实时 `ListTools`，不经清单重抓**——见总索引"安装后远端真相的统一口径"：清单可能已改指新版，漂移的定义是"已接受端点偏离已接受快照"→ 差异持久化 `drift_state=detected` + `drift_detail`（JSON：`{added:[], removed:[], schema_changed:[], checked_at}`））；`GetDrift`（Viewer 读）；`ResolveDrift`（Admin：重核验 → 生成差异 → 以当前远端目录为新快照重置 `tools_digest`/`drift_state=none`——版本号不变，"接受当前目录为已核验快照"；对新增写工具仍按规则写 `Enabled=false` 行）。运行时目录加载检测到差异时 best-effort 置位（loader 内不引 DB——由 `PluginRepository.MarkDriftDetected` 经 provider 闭包调用，失败仅日志）。

**Tech Stack:** Go 1.26；T10 `plugintest.SetTools`（运行时改目录）；`//go:build integration` 真 PG。

**Spec:** `docs/specs/2026-09-23-self-hosted-plugins-spec.md`（User Story 25；Implementation Decisions 52 行）；范围裁决 GAP-6、Ruling R6。

**Issue:** https://github.com/1123786563/WeKnora-fork01/issues/116 （blocked_by #115 → 任务级前置 T16）

## Global Constraints

见总索引。`drift_detail` 仅存工具名/digest/时间等元数据，不存远端 schema 原文（不可信数据不落审计面）。

## Review Focus（本切片）

- 漂移三形态（新增/移除/schema 变）各自：CheckDrift 置位 + 成员对话调用拒绝 + 提示复审文案（T17）。
- resolve 后：新快照生效、漂移清零、新增写工具仍关闭（T17）。
- 空间 1 漂移不影响空间 2（同端点两安装——受控服务同目录两空间快照不同时，只有漂移方被阻断；同快照时双方都检测）断言"其他空间不受影响"用独立受控服务实例（T17）。
- 手工 MCP 服务：目录/schema 随意变化不受治理（现状语义），无 drift 概念（T17）。

---

### Task 17: 漂移状态持久化、检测强化与复审闭环

**Files:**
- Modify: `internal/modules/plugins/snapshot.go`（`DriftReport` 计算：live 目录 vs 快照，输出 `types.PluginDriftDetail`）
- Modify: `internal/types/plugin.go`（`PluginDriftDetail{Added, Removed, SchemaChanged []string; CheckedAt time.Time}`）
- Modify: `internal/types/interfaces/plugin.go`、`internal/application/repository/plugin.go`、`internal/application/service/plugin_install_service.go`（`CheckDrift`/`GetDrift`/`ResolveDrift`）
- Modify: `internal/application/service/agent_service.go` 的 provider（`PluginSnapshotLookup` 扩展：检测到目录差异时调用 `MarkDriftDetected` best-effort——provider 改为返回快照 + 差异回调；实现放 `plugin_service.go`，agent_service 只消费接口不变）
- Modify: `internal/handler/dto/plugin.go`、`internal/handler/plugin.go`、`internal/router/routes_plugins.go`
- Test: `internal/modules/plugins/drift_test.go`（fake 层）
- Test: `internal/modules/plugins/plugin_drift_integration_test.go`（`//go:build integration`）

**Interfaces:**
- Consumes: T09 `FilterToolsBySnapshot`/`ErrPluginDrift`；T16 接受升级语义。
- Produces:
  - `plugins.DiffLiveAgainstSnapshot(live []*types.MCPTool, snap []types.PluginToolSnapshot) *types.PluginDriftDetail`
  - `interfaces.PluginService.CheckDrift(ctx, tenantID uint64, installationID string) (*dto.PluginDriftReport, error)`（Admin）
  - `interfaces.PluginService.GetDrift(ctx, tenantID uint64, installationID string) (*dto.PluginDriftReport, error)`（Viewer；未检测过时 `drift_state` 返回安装行当前值）
  - `interfaces.PluginService.ResolveDrift(ctx, tenantID uint64, actorID, installationID string) (*dto.PluginInstallationResponse, error)`（Admin）
  - `dto.PluginDriftReport{InstallationID string; DriftState string; Detail *types.PluginDriftDetail; SnapshotToolNames []string}`
  - `PluginRepository` 扩展：`UpdateDrift(ctx, tenantID uint64, installationID string, state string, detail json.RawMessage) error`
  - 路由：`GET /api/v1/plugins/installations/:id/drift`（Viewer）、`POST .../drift/check`（Admin）、`POST .../drift/resolve`（Admin）

- [ ] **Step 1: 写失败测试（fake 层）**

`drift_test.go`（**`package plugins_test`**——import `plugintest`，外部测试包约定见总索引）：

```go
func TestDiffLiveAgainstSnapshot(t *testing.T) {
	// 快照 [a,b]；live 目录 [a(schema 变), c(新增)]（b 被移除）：
	// Detail.Added=[c], Removed=[b], SchemaChanged=[a]。
}

func TestCheckDriftPersistsDetectedState(t *testing.T) {
	// 受控服务 SetTools 漂移后 CheckDrift → drift_state=detected、detail 落库；
	// 目录复原后 CheckDrift → drift_state 重回 none、detail 清空。
}

func TestResolveDriftRebasesSnapshot(t *testing.T) {
	// 漂移（新增只读 c + 写 w）后 ResolveDrift：
	// 新快照含 c/w；c approval 行 Enabled=true、w Enabled=false（写工具复审后仍默认关闭）；
	// drift_state=none；后续 Agent 目录见 c/w。
}

func TestResolveDriftRejectsWhenEndpointBroken(t *testing.T) {
	// 远端不可达时 resolve → 错误；快照/状态不变。
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/plugins/ -run 'TestDiffLiveAgainst|TestCheckDrift|TestResolveDrift' -v`
Expected: FAIL —— 未定义。

- [ ] **Step 3: 实现**（`CheckDrift` 用 `EndpointLister` 直连已接受端点；`ResolveDrift` 复用 T16 事务模式）。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/modules/plugins/ -run 'TestDiffLiveAgainst|TestCheckDrift|TestResolveDrift' -v && go build ./...`
Expected: PASS。

- [ ] **Step 5: 写失败测试（真 PG：成员对话观察 + 隔离 + 手工兼容）**

`plugin_drift_integration_test.go`：

```go
func TestDriftBlocksFromMemberConversation(t *testing.T) {
	// 安装 v1（工具 t1）→ 成员调用成功；
	// 受控服务 SetTools：t1 schema 变 + 新增 t2：
	// 成员新会话 list_tools → ErrPluginDrift 文案（"plugin capability drift detected; administrator review required"）；
	// t2 不可见；CheckDrift → detected 且 detail 正确；
	// ResolveDrift 后成员目录 = [t1(新 schema), t2]，调用成功。
}

func TestDriftScopedToSingleInstallation(t *testing.T) {
	// 两个独立受控服务实例 A/B；空间 1 装 A、空间 2 装 B（同 plugin_id 不同 URL——
	// plugin_id 唯一性约束在 (tenant, plugin) 上，跨空间可同名）：
	// 仅 A 漂移 → 空间 1 阻断、空间 2 正常。
}

func TestManualServiceNotGovernedByDrift(t *testing.T) {
	// 手工服务（无插件关联）工具 schema 变化后：Agent 目录正常反映远端现状
	//（mcp_exposure.go 既有 describe/call ref 校验除外——那是现状语义，非本需求新增治理），
	// 无 drift 状态、无 plugin API 可查。
}
```

- [ ] **Step 6: 运行确认失败**

Run: `PLUGIN_TEST_DATABASE_URL=... go test ./internal/modules/plugins/ -tags integration -run 'TestDriftBlocks|TestDriftScoped|TestManualServiceNotGoverned' -v`
Expected: 无 DB Fatal；有 DB FAIL。

- [ ] **Step 7: 补全实现与测试，运行通过**

Run: `PLUGIN_TEST_DATABASE_URL=... go test ./internal/modules/plugins/ -tags integration -run 'TestDrift' -v && go test ./internal/modules/plugins/ -v`
Expected: PASS + 非 integration 不回归。

- [ ] **Step 8: Commit**

```bash
git add internal/types/ internal/modules/plugins/ internal/application/ internal/handler/ internal/router/
git commit -m "feat(plugins): 远端能力漂移检测持久化与管理员复审闭环 [T17]"
```
