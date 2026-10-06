# 测试债清偿·健康轮实施计划（/loop 第 32 循环）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 清偿健康认证轮定位的两族真失败（benefits 日期敏感 ~15+ 测、database 包 5+ 测），使全仓逐包基线全绿。

**Architecture:** 上轮已确证根因与修法方向（钟改动态当月 15 日 + 8 处 2026-09 派生；migration 测试 cwd 相对路径）。本轮在 worktree 落地并验证。

**Tech Stack:** Go testing；`go test -count=1 -timeout 20m`。

## Global Constraints

- 只改测试侧（钟/派生/路径）；生产码零改动。
- 全部改动在 `/tmp/wk-fix` worktree（主 checkout 在用户合并分支上，严禁触碰）。
- 提交带 `(TESTDEBT)`；不 push（轮末统一）。

---

### Task 1: benefits 日期敏感修复（subagent）
- [ ] `septemberClock()` → 动态当月 15 日（`time.Date(y, m, 15, 10, 0, 0, 0, time.UTC)`）。
- [ ] 8 处 `2026-09` 硬编码由 `domain.MonthlyPeriod(septemberClock()())` 派生；跨月测试的 October 推进改 `clock.AddDate(0, 1, 0)`；lot 钱包名同步派生。
- [ ] 门禁：`go test ./internal/modules/commercial/service/commercial/ -count=1 -timeout 10m` 全绿。

### Task 2: database 包修复（subagent）
- [ ] migration 测试的相对路径改绝对路径（`runtime.Caller` 或测试内 `chdir`）。
- [ ] career migration 5 测失败原因排查并修复。
- [ ] 门禁：`go test ./internal/database/... -count=1` 全绿。

### Task 3: 五重包复跑（subagent）
- [ ] 逐包串行复跑 service/repository/container/database/handler.session（各 `-timeout 20m`），全绿。

### Task 4: 合并推送 + 记忆更新（controller）
- [ ] worktree ff 合并到 main、推送 origin。

## Self-Review
覆盖：两族根因→Task 1/2；门禁→Task 3；合并→Task 4；无占位。
