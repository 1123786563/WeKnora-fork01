# Issue #55（T25 交付协同/恢复）收口实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 判定 round5（8500ea74e）交付的 T55 流是否满足 `2026-09-30-t55-integrated-review-repairs-ledger.md` 末行所列悬置链（Task 1 High 修复复审、Task 2 报告溯源/集成、Task 3 shell-boundary 实现、集成验证、独立终审、OCR），补齐缺口，给出 #55 issue 级判词。

**Architecture:** 权威标准 = 该 ledger 的 T55-F1..F4 判定 + `2026-09-30-t55-task1-high-review-repairs.md`/`-task3-credential-path-repair.md`/`-task3-scope-negative-tests.md` 三份修复计划 + `plan-t55.md` 任务面。事实源 = main（8500ea74e，含 round5 合并的 t55-repair-backend/mobile/shell-boundary/task6/7/8 与 owner-confirmed-recovery r1-r3 工件）。工作分支沿用 `codex/issue30-t63-closure`（已含 #63/#64/#65 收口，#55 与其无文件冲突时可直接续用；冲突则改由独立分支）。

**Tech Stack:** Go（codedelivery/appconnector/repository）+ mobile-core（delivery-recovery）；node:test 前端验证（Node 26）。

## Global Constraints

- 审计/复审只读；修复仅动审计点名文件，TDD 证据入报告。
- 判定优先级：T55 修复计划文本 > integrated-review-repairs ledger > plan-t55.md > 代码实况。
- live GitHub 路径与原生环境为 blocked-env（如实登记，不伪造）。
- 不 push、不动 GitHub；提交带 `(T25 #55)`。
- 报告产出到 `.superpowers/sdd/2026-10-02-issue55-t25/`。

---

### Task 1: round5 交付 vs 悬置链审计（只读）

- [ ] 对照 ledger 末行六项悬置，逐项核 main 实况：① F1 High 修复（CAS claim + 无重复 PR 写）是否随 `codex/issue30-t55-repair-backend` 落库且有复审记录；② F2/F3/F4 是否同链覆盖；③ Task 2 R1 溯源（报告源提交名/载荷哈希）是否在 round5 工件中补齐；④ Task 3 shell-boundary 是否实现+审查；⑤ 集成验证（backend/frontend validation 报告）是否重跑于集成 HEAD；⑥ 终审/OCR 是否存在。
- [ ] 跑 T55 聚焦测试组（codedelivery/appconnector/repository + `packages/mobile-core/src/delivery/`，实测名 grep）并记录；`TestUnknownResolution*`/`TestT25*` 族全绿为底线。
- [ ] 产出判定表+缺口清单（severity+file:line+最小修正）→ task-1-report.md。

### Task 2: 缺口修复（条件执行）

- [ ] 仅对 Task 1 清单中 Critical/Important 逐条 TDD 修复（RED→GREEN 证据），提交 `fix(delivery): <摘要> (T25 #55)`；复跑聚焦组 + `go build ./...` + `git diff --check`；报告 → task-2-report.md。无缺口则记账跳过。

### Task 3: 独立终审 + 台账判词

- [ ] 派最强模型终审：F1–F4 逐条 + 集成态一致性 + 隐私/安全红线；判词「#55 closure-ready（live GitHub/原生环境 blocked-env 边界内）」或阻塞清单。
- [ ] ledger 追加终局段（round5 交付对账 + 判词 + follow-up）；OCR 按上两轮口径（规则性 N/A / 配额受阻如实记录）。

## Self-Review

- 规格覆盖：ledger 六项悬置 → Task 1 逐项；修复→Task 2；终审/OCR→Task 3。
- 占位扫描：条件任务契约明确，无 TBD。
