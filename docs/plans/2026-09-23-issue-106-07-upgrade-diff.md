# 插件 07｜预览候选插件版本差异 实施计划（Issue #114）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 管理员预览候选插件版本相对已接受版本的能力差异（工具新增/移除、schema、scope、读写分类、执行端点变化），预览不切换版本；候选不可达或声明不符时明确报错、旧版继续可用；两个空间可停留不同版本。

**Architecture:** `internal/modules/plugins/snapshot.go` 新增 `DiffSnapshots`（纯函数：以 `(name → snapshot)` 双射对比五维差异）；`PluginService.PreviewUpgrade` 重抓清单+核验（复用 `FetchAndVerify`），与安装的已接受快照对比生成差异报告——只读不写任何安装状态。候选版本号不高于已接受时仍允许预览（降级也是管理员决定）但报告标注 `downgrade`。

**Tech Stack:** Go 1.26；gin；T10 `plugintest`（目录可变——差异场景）。

**Spec:** `docs/specs/2026-09-23-self-hosted-plugins-spec.md`（User Stories 12/24；Implementation Decisions 51-52 行；设计文档「清单与版本契约」升级段）。

**Issue:** https://github.com/1123786563/WeKnora-fork01/issues/114 （blocked_by #110 → 任务级前置 T06）

## Global Constraints

见总索引。`PreviewUpgrade` 对远端只做读操作；任何失败不得修改 `plugin_installations`（旧版可用性由不写保证）。

## Review Focus（本切片）

- 五维差异每一维都有独立断言（新增/移除/schema/scope+读写分类/端点）——scope 与读写分类合并为一个"权限面差异"维度但断言分开（T14）。
- 候选不可达 → 错误明确、安装行零变化（T14）。
- 预览后重复预览（幂等读）→ 同结果不累积状态（T14）。
- 清单声明与候选端点目录不符 → 复用 T01 核验拒绝（T14）。

---

### Task 14: 差异计算与升级预览 API

**Files:**
- Modify: `internal/modules/plugins/snapshot.go`（`DiffSnapshots`）
- Modify: `internal/types/plugin.go`（差异 DTO 类型）
- Modify: `internal/types/interfaces/plugin.go`、`internal/application/service/plugin_install_service.go`（`PreviewUpgrade`）
- Modify: `internal/handler/dto/plugin.go`、`internal/handler/plugin.go`、`internal/router/routes_plugins.go`
- Test: `internal/modules/plugins/upgrade_diff_test.go`
- Test: `internal/handler/plugin_upgrade_test.go`

**Interfaces:**
- Consumes: T01 `FetchAndVerify`/`FetchResult`；T06 `GetInstallation`/`types.PluginInstallation`（**含 `ManifestURL` 列——`PreviewUpgrade` 重抓来源即 `installation.ManifestURL`**，见总索引"安装后远端真相的统一口径"）。
- Produces（T15/T16 依赖）:
  - `types.PluginVersionDiff{PluginID string; CurrentVersion, CandidateVersion string; IsDowngrade bool; EndpointChanged bool; CurrentEndpoint, CandidateEndpoint string; AddedTools, RemovedTools []types.PluginToolSnapshot; ChangedTools []types.PluginToolChange{Name string; SchemaChanged, ScopeChanged, ReadWriteClassChanged, PersonalAuthChanged bool; Current, Candidate types.PluginToolSnapshot}}`
  - `plugins.DiffSnapshots(current, candidate []types.PluginToolSnapshot, currentEndpoint, candidateEndpoint string) *types.PluginVersionDiff`
  - `interfaces.PluginService.PreviewUpgrade(ctx context.Context, tenantID uint64, installationID string) (*dto.PluginUpgradePreviewResponse, error)`（`dto.PluginUpgradePreviewResponse{Diff types.PluginVersionDiff; CandidateFingerprint string; CandidateToolsDigest string}`）
  - 路由：`POST /api/v1/plugins/installations/:id/upgrade-preview`（Admin）

- [ ] **Step 1: 写失败测试（纯函数五维差异）**

`internal/modules/plugins/upgrade_diff_test.go`（**`package plugins_test`**——本文件 import `plugintest` 受控服务，外部测试包约定见总索引"测试包名约定"；plugintest import plugins 包，内部测试包会构成导入环）：

```go
func TestDiffSnapshotsCoversAllDimensions(t *testing.T) {
	current := []types.PluginToolSnapshot{
		{Name: "keep", InputSchemaDigest: "d1", ReadOnly: true, Scopes: []string{"read"}},
		{Name: "schema_change", InputSchemaDigest: "d2"},
		{Name: "scope_change", InputSchemaDigest: "d3", Scopes: []string{"read"}},
		{Name: "rw_change", InputSchemaDigest: "d4", ReadOnly: true},
		{Name: "auth_change", InputSchemaDigest: "d5", RequiresPersonalAuth: false},
		{Name: "removed"},
	}
	candidate := []types.PluginToolSnapshot{
		{Name: "keep", InputSchemaDigest: "d1", ReadOnly: true, Scopes: []string{"read"}},
		{Name: "schema_change", InputSchemaDigest: "dX"},
		{Name: "scope_change", InputSchemaDigest: "d3", Scopes: []string{"read", "write"}},
		{Name: "rw_change", InputSchemaDigest: "d4", ReadOnly: false},
		{Name: "auth_change", InputSchemaDigest: "d5", RequiresPersonalAuth: true},
		{Name: "added"},
	}
	d := DiffSnapshots(current, candidate, "https://old/e", "https://new/e")
	require.Len(t, d.AddedTools, 1)   // added
	require.Len(t, d.RemovedTools, 1) // removed
	require.Len(t, d.ChangedTools, 4)
	byName := map[string]types.PluginToolChange{}
	for _, c := range d.ChangedTools { byName[c.Name] = c }
	require.True(t, byName["schema_change"].SchemaChanged)
	require.True(t, byName["scope_change"].ScopeChanged)
	require.True(t, byName["rw_change"].ReadWriteClassChanged)
	require.True(t, byName["auth_change"].PersonalAuthChanged)
	require.True(t, d.EndpointChanged)
}

func TestPreviewUpgradeDoesNotTouchInstallation(t *testing.T) {
	// 受控服务（plugintest）当前 v1 已安装；SetTools 变目录 + 清单升 v2：
	// PreviewUpgrade 返回差异（added=1，endpoint_changed=true）；
	// 断言 plugin_installations 行 accepted_version/tools_digest/state 逐字段不变；
	// Agent 侧调用（RegisterMCPTools）仍走 v1 目录。
}

func TestPreviewUpgradeCandidateUnreachable(t *testing.T) {
	// 受控服务关闭后 PreviewUpgrade → 错误含 "unreachable"/"verification failed"；安装行零变化。
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/plugins/ -run 'TestDiffSnapshots|TestPreviewUpgrade' -v`
Expected: FAIL —— `DiffSnapshots`/`PreviewUpgrade` 未定义。

- [ ] **Step 3: 实现**（`DiffSnapshots` 双 map 对比；`IsDowngrade` 用 semver 三段数值比较；`PreviewUpgrade`：`GetInstallation` → `FetchAndVerify(installation.ManifestURL)` → `DiffSnapshots(安装快照, res.Snapshot, 安装 endpoint, res.Manifest.Transport.Endpoint)`；handler/路由 Admin）。

- [ ] **Step 4: 运行确认通过 + handler 测试 + Commit**

Run: `go test ./internal/modules/plugins/ -run 'TestDiffSnapshots|TestPreviewUpgrade' -v && go test ./internal/handler/ -run 'TestPreviewUpgradeHandler' -v && go build ./...`
Expected: PASS（handler 测试覆盖 200/404/Admin 语义 stub）。
```bash
git add internal/types/ internal/modules/plugins/ internal/application/ internal/handler/ internal/router/
git commit -m "feat(plugins): 候选版本五维差异计算与升级预览 API（不触碰已接受版本）[T14]"
```

---

### Task 15: 前端差异预览 UI

**Files:**
- Modify: `packages/api-client/src/plugins.ts` / `plugins.test.ts`（`previewUpgrade` + `parsePluginUpgradePreview`：diff 各维字段映射）
- Modify: `apps/web/src/settings/PluginsSettingsPanel.tsx` / `PluginsSettingsPanel.test.tsx`（安装行"检查升级"按钮 → 差异面板：版本对（含降级标注）、端点变化行、新增/移除/变更工具表（变更理由徽标：schema/scope/读写/授权）；候选不可达错误条）

**Interfaces:**
- Consumes: T14 `dto.PluginUpgradePreviewResponse` JSON。
- Produces: `client.plugins.previewUpgrade(installationId)`。

- [ ] **Step 1: 写失败测试**（解析器五维字段 + 面板渲染差异表与降级标注；不可达错误显示且列表不变形）。
- [ ] **Step 2: 运行确认失败**：`pnpm gates` → FAIL。
- [ ] **Step 3: 实现**。
- [ ] **Step 4: 运行确认通过 + Commit**

Run: `pnpm gates`（Expected 全过）
```bash
git add packages/api-client/src/plugins.ts packages/api-client/src/plugins.test.ts apps/web/src/settings/
git commit -m "feat(plugins): 管理端候选版本差异预览面板 [T15]"
```
