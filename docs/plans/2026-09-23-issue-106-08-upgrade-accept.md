# 插件 08｜管理员接受插件升级 实施计划（Issue #115）

> **For agentic workers:** REQUIRED SUBAGENTS SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 管理员接受候选版本后，空间已接受版本与 Agent 后续调用一起切换；接受前重核候选快照、操作幂等；失败/不可达保旧版；仅管理员可接受；两空间可停留不同版本；升级效果从成员对话验证新旧版调用。

**Architecture:** `PluginService.AcceptUpgrade(ctx, tenantID, actorID, installationID, candidateFingerprint string)`：补偿式事务（T06 Step 4 同款裁决）内重抓核验（`FetchAndVerify(installation.ManifestURL)` 的 `IdentityFingerprint` 必须等于调用方从 T14 预览拿到的 `candidateFingerprint`——防"预览后远端又变"；`ManifestURL` 是 T06 落列的长期清单来源，preview 行已消费不可复用）→ 更新 `plugin_installations`（`accepted_version`/`endpoint_url`/`tools_snapshot`/`tools_digest`/`drift_state=none`/`drift_detail=NULL`；后续步骤失败时回写内存持有的旧值）→ 同步物化 `mcp_services`（`URL` 新端点 + `UpdatedAt` 刷新触发 `MCPManager` 连接重建，manager.go 缓存按 `UpdatedAt` 失效）→ 为候选快照**新增**工具写策略行（只读 `Enabled=true`、写 `Enabled=false`）→ 已有工具策略行不动（管理员既有启停/审批决定保留）。幂等：同 fingerprint 再次接受 → 直接返回当前安装（成功）；任何一步失败 → 补偿回写保旧版。

**Tech Stack:** Go 1.26；gorm 补偿式写序；`//go:build integration` 真 PG（复用 `pluginpg` 基建 + `plugintest.SetTools` 模拟新版）。

**Spec:** `docs/specs/2026-09-23-self-hosted-plugins-spec.md`（User Stories 3/12/13/24；Implementation Decisions 51 行）。

**Issue:** https://github.com/1123786563/WeKnora-fork01/issues/115 （blocked_by #111+#114 → 任务级前置 T10、T14）

## Global Constraints

见总索引。物化服务 `Name`（`plugin:<plugin_id>`）与 `PluginInstallationID` 在升级中不变——Agent 会话内 `server_id` 稳定，只有端点与快照切换。

## Review Focus（本切片）

- 接受时远端已再变（fingerprint 不符）→ 拒绝并提示重新预览（T16）。
- 失败保旧版：升级中断/远端不可达 → 旧版本仍可从对话调用（T16）。
- 新增写工具在切换后仍关闭（策略行增量写入）——完整治理验收在 T18，此处断言行存在（T16）。
- 两空间不同版本互不影响（T16）。
- 既有工具策略行不被升级覆盖（T16）。

---

### Task 16: 接受升级服务与对话验证

**Files:**
- Modify: `internal/types/interfaces/plugin.go`、`internal/application/service/plugin_install_service.go`（`AcceptUpgrade`）
- Modify: `internal/handler/dto/plugin.go`、`internal/handler/plugin.go`、`internal/router/routes_plugins.go`（`POST /api/v1/plugins/installations/:id/upgrade-accept`（Admin），body `{"candidate_fingerprint": "..."}`）
- Test: `internal/modules/plugins/upgrade_accept_test.go`（fake 层）
- Test: `internal/modules/plugins/plugin_upgrade_integration_test.go`（`//go:build integration`，真 PG）

**Interfaces:**
- Consumes: T14 `PreviewUpgrade`/`PluginVersionDiff`/`FetchResult.IdentityFingerprint`；T06 `GetInstallation`（**`installation.ManifestURL` 为重抓来源**）与策略行设施；T10 `plugintest`、T11 `pluginpg`。
- Produces:
  - `interfaces.PluginService.AcceptUpgrade(ctx context.Context, tenantID uint64, actorID, installationID, candidateFingerprint string) (*dto.PluginInstallationResponse, error)`
  - T17/T18/T20 依赖的语义：`drift_state` 在成功接受后重置 `none`；新工具策略行规则（只读启用/写关闭）。

- [ ] **Step 1: 写失败测试（fake 层语义）**

`upgrade_accept_test.go`（**`package plugins_test`**——import `plugintest`，外部测试包约定见总索引）：

```go
func TestAcceptUpgradeSwitchesSnapshotAndEndpoint(t *testing.T) {
	// v1 安装（1 只读工具）；受控服务切换到 v2（新增 1 只读 + 1 写工具、端点路径换）：
	// AcceptUpgrade(fingerprint=预览值) →
	// 断言：accepted_version=v2、endpoint_url=新端点、tools_digest 重算、
	//      drift_state=none；物化 mcp_services.URL=新端点 且 Name/ID 不变；
	//      新只读工具 approval 行 Enabled=true、新写工具 Enabled=false；
	//      v1 既有工具的 approval 行保持原值（预置 Enabled=false 后升级仍 false）。
}

func TestAcceptUpgradeIdempotentAndFingerprintGuard(t *testing.T) {
	// a) 同 fingerprint 二次接受 → 成功返回、无重复副作用（策略行数不变）。
	// b) fingerprint 不符（远端 v3）→ 错误 "candidate changed since preview"，安装行全部字段不变。
}

func TestAcceptUpgradeFailureKeepsOldVersion(t *testing.T) {
	// 受控服务关闭 → 错误；安装行/物化服务/Agent 可调用目录均为 v1（RegisterMCPTools 仍见旧工具）。
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/plugins/ -run 'TestAcceptUpgrade' -v`
Expected: FAIL —— `AcceptUpgrade` 未定义。

- [ ] **Step 3: 实现**（事务口径沿用 T06 Step 4 的**补偿式**裁决：重核通过后先更新安装行（新快照/端点/版本），再同步物化 `mcp_services.URL` 与策略行增量；任一步失败时把安装行回写为重核前读取的旧值（内存持有）并返回原错误——保旧版语义由补偿回写保证；策略行增量 = 遍历候选快照，`GetByServiceID+ToolName` 无行才写）。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/modules/plugins/ -run 'TestAcceptUpgrade' -v && go build ./...`
Expected: PASS。

- [ ] **Step 5: 写失败测试（真 PG：从成员对话验证新旧版调用 + 双空间）**

`plugin_upgrade_integration_test.go`：

```go
func TestUpgradeVisibleFromMemberConversation(t *testing.T) {
	// 真实 stack：成员（无账号工具场景，避免 OAuth 噪声）在升级前 call 旧版工具 v1_tool 成功；
	// 管理员 PreviewUpgrade→AcceptUpgrade（受控服务已 SetTools 为 v2，v1_tool 移除、v2_tool 新增）；
	// 同一成员会话（重新 RegisterMCPTools 组装——Agent 引擎按会话组装，升级后新会话）：
	// 新目录只见 v2_tool；v2_tool 调用成功；v1_tool describe/call 不可用。
}

func TestTwoSpacesDifferentVersions(t *testing.T) {
	// 空间 1、2 各自安装同一插件 v1；空间 1 升到 v2，空间 2 停留 v1：
	// 空间 2 成员仍调 v1 工具成功；空间 1 调 v2。互不影响。
}
```

- [ ] **Step 6: 运行确认失败**

Run: `PLUGIN_TEST_DATABASE_URL=... go test ./internal/modules/plugins/ -tags integration -run 'TestUpgradeVisible|TestTwoSpacesDifferentVersions' -v`
Expected: 无 DB `blocked-env` Fatal；有 DB FAIL（集成测试未写完/断言未满足）。

- [ ] **Step 7: 补全集成测试并实现缺失项，运行通过**

Run: `PLUGIN_TEST_DATABASE_URL=... go test ./internal/modules/plugins/ -tags integration -run 'TestUpgradeVisible|TestTwoSpacesDifferentVersions' -v`
Expected: PASS。

- [ ] **Step 8: Commit**

```bash
git add internal/types/ internal/modules/plugins/ internal/application/ internal/handler/ internal/router/
git commit -m "feat(plugins): 管理员接受插件升级（重核/幂等/失败保旧版/对话验证）[T16]"
```
