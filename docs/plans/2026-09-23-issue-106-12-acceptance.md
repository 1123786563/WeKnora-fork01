# Spec 根｜Issue #106 汇总验收 实施计划（Issue #106）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 全部子 Issue 完成后，在**一个应用边界 seam** 上按 Spec 十条验收边界整体走查：受控远程 MCP 测试服务 + 两名成员身份，覆盖管理员安装、成员授权、Agent 实际调用、双成员凭据隔离、跨空间隔离、版本升级、工具漂移拒绝、写操作拒绝/超时后无外部写入、手工 MCP 兼容；输出结构化验收证据；更新用户文档；复核追踪矩阵。不承载新的独立实现（范围裁决 R2）。

**Architecture:** `internal/modules/plugins/plugin_acceptance_test.go`（`//go:build integration`，真 PG）把 T10-T19 已建的全部设施（`plugintest`、`fakejira`、`pluginpg`、真实 Gate/OAuth 组装）串成一条 Spec 边界核销套件，逐场景追加 `PLUGIN106-EVIDENCE` JSON 行（先例：oc_integration_test.go 的 `OC17-EVIDENCE` journal）。文档更新 `website-docs/03-features/08-mcp.md` 增加"自行托管插件安装"节。

**Tech Stack:** Go 1.26；`//go:build integration`；markdown 文档。

**Spec:** `docs/specs/2026-09-23-self-hosted-plugins-spec.md` Testing Decisions 全节（61-66 行）；范围裁决 R2（#106=汇总验收语义）。

**Issue:** https://github.com/1123786563/WeKnora-fork01/issues/106 （blocked_by 全部子 Issue → 任务级前置 T01–T19）

## Global Constraints

见总索引。本切片不新增生产代码（若走查暴露缺陷，回到所属切片计划修复后重跑，不在本切片打补丁绕过）。

## Review Focus（本切片）

- 十条边界逐一可指认到已通过的测试函数名（矩阵与 evidence 双向可核）。
- 证据 journal 场景键与 `docs/plans/2026-09-23-issue-106-trace-matrix.md` 一一对应。
- 文档描述与实现行为一致（不承诺平台锁死远端代码——信任边界声明照设计文档 80 行）。

---

### Task 20: Spec 十条边界核销验收套件、文档与矩阵复核

**Files:**
- Create: `internal/modules/plugins/plugin_acceptance_test.go`（`//go:build integration`）
- Modify: `website-docs/03-features/08-mcp.md`（追加「自行托管插件安装」节：管理员清单安装流程、版本升级与漂移、写工具默认关闭与审批、成员个人授权、自托管信任边界、示例服务指引 `examples/plugins/jira-todo-mcp`）
- Modify: `docs/plans/2026-09-23-issue-106-trace-matrix.md`（执行者按实际通过的证据填充"证据"列）

**Interfaces:**
- Consumes: T10 `plugintest.Server`、T11 `pluginpg.openPluginDB/newPluginStack`、T13 `fakejira`/`NewJiraTodoPlugin`、T19 `newApprovalStack`、T14/T16 差异与升级、T17 漂移、T18 策略。
- Produces: `PLUGIN106-EVIDENCE` journal（场景键 B1…B10 对应十条边界）；文档节。

- [ ] **Step 1: 写验收套件（十条边界各一个子测试）**

`plugin_acceptance_test.go` 骨架（每条边界调用对应前置任务已通过的能力，断言用户可观察结果并 `journal.record`）：

```go
//go:build integration
// #106 汇总验收：Spec 十条边界（任务书 #106 验收）× 应用边界 seam。
// 环境契约：PLUGIN_TEST_DATABASE_URL（缺失 Fatal blocked-env，绝不 Skip 通过）。

func TestPlugin106Acceptance(t *testing.T) {
	// 共享组装：真 PG schema、两空间（tenantA/tenantB）、每空间 owner+两成员（memberA1/memberA2）、
	// Jira 形替身（T13）+ 通用替身（T10）+ 真实 Gate stack（T19）。

	t.Run("B1 清单交付与管理员安装前预览核验", func(t *testing.T) {
		// memberA1（非管理员）调 preview → 403 语义（服务层拒绝）；
		// owner 预览：版本/端点/工具/读写分类/授权要求齐备；受限地址清单 URL 拒绝；
		// 清单声明与目录不符（plugintest 目录改）→ 预览拒绝。journal B1。
	})
	t.Run("B2 安装固定版本+差异+手动接受+失败保旧", func(t *testing.T) { /* T16 场景复述断言 */ })
	t.Run("B3 运行时以已接受快照阻断漂移", func(t *testing.T) { /* T17 场景复述断言 */ })
	t.Run("B4 成员发现+个人授权按空间/成员/插件隔离", func(t *testing.T) { /* T11 场景复述断言 */ })
	t.Run("B5 写工具默认关闭+逐项启用+审批", func(t *testing.T) { /* T18+T19 场景复述断言（拒绝与超时零写入）*/ })
	t.Run("B6 停用/撤销/过期/候选不可达", func(t *testing.T) { /* T06+T11+T14 场景复述断言 */ })
	t.Run("B7 Jira 纵向案例", func(t *testing.T) { /* T13 场景复述断言（本人本周/来源链接/无模型注入/服务端时间范围）*/ })
	t.Run("B8 无账号工具独立能力", func(t *testing.T) { /* T10 场景复述断言 */ })
	t.Run("B9 手工 MCP 兼容", func(t *testing.T) { /* 手工服务全程可用 + 缺省启用语义 + 迁移未把它视为插件版本（000190.down 后仍在）*/ })
	t.Run("B10 网络与租户边界", func(t *testing.T) { /* 受限地址拒绝（预览/升级/漂移核验三处）+ 跨空间隔离复述 */ })
	// t.Cleanup 打印全部 PLUGIN106-EVIDENCE 行。
}
```

- [ ] **Step 2: 运行确认失败**

Run: `PLUGIN_TEST_DATABASE_URL=... go test ./internal/modules/plugins/ -tags integration -run TestPlugin106Acceptance -v`
Expected: 无 DB Fatal；有 DB FAIL（套件尚未写全/断言未满足）。

- [ ] **Step 3: 补全套件至全部通过**

Run: `PLUGIN_TEST_DATABASE_URL=... go test ./internal/modules/plugins/ -tags integration -run TestPlugin106Acceptance -v`
Expected: PASS，输出 B1…B10 十行 evidence。

- [ ] **Step 4: 全量回归**

Run: `go test ./internal/... 2>&1 | tail -30 && go build ./... && pnpm gates`
Expected: 全部通过（integration tag 套件仅在提供 `PLUGIN_TEST_DATABASE_URL` 时运行；无 DB 时以 blocked-env Fatal 的既有契约为准——执行者必须提供 DB 跑过一次并留证据）。

- [ ] **Step 5: 文档与矩阵**

`website-docs/03-features/08-mcp.md` 追加节（内容要点见 Files 描述；含自托管信任边界原文照设计文档 80 行）。`docs/plans/2026-09-23-issue-106-trace-matrix.md` 证据列填实际测试函数名与 evidence 键。

- [ ] **Step 6: Commit**

```bash
git add internal/modules/plugins/plugin_acceptance_test.go website-docs/03-features/08-mcp.md docs/plans/2026-09-23-issue-106-trace-matrix.md
git commit -m "test(plugins): #106 十条边界应用边界汇总验收与文档 [T20]"
```
