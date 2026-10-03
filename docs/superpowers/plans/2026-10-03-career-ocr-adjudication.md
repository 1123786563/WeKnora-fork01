# Career OCR High/Medium 裁定轮实施计划（/loop 第 6 循环）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-t # tracking.
> REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans.

**Goal:** 对 Career OCR 首份完整报告的 133 条 High/Medium findings（13H/120M，docs/plans/issue-140/ocr/）逐条对照当前 main 裁定：already-fixed / still-open（重定级）/ false-positive；真开放 High 修复；产出裁定表与处置判词，为 Career 族 closure 扫清发现面。

**Architecture:** 报告范围终点 b3d48d5cb 早于 main 上至少三个后续修复提交（Task 2 agent 提示：部分发现可能在 main 已修）——裁定基线=当前 main（8adbac874+），不按快照终局。裁定优先级：main 代码实况 > 报告文字。

**Tech Stack:** Go/TS 视 finding 所在面；TDD 修复真开放 High。

## Global Constraints

- 裁定必须引 main 证据（file:line 或修复提交 SHA）；「已修」须可验证。
- 真开放 High 修复走 TDD 独立提交 `(CAREER-OCR)`；Medium 开放项登记（不内联批量修）。
- 不 push（轮末统一）；工作区 `.superpowers/sdd/2026-10-03-career-ocr/`。

---

### Task 1: 133 条逐条裁定（subagent）
- [ ] 读 OCR digest；逐条对照 main：already-fixed（引证）/ open-high / open-medium / false-positive；产出四分表+统计 → task-1-report.md。

### Task 2: 真开放 High 修复（条件）
- [ ] 逐条 TDD 修复+焦点测试；提交。

### Task 3: 判词（controller）
- [ ] 终局段：四分统计、修复清单、Medium 登记表；推送。

## Self-Review
覆盖：裁定→Task 1、修复→Task 2（条件契约明确）、判词→Task 3。
