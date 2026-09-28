# OCR Round 4 修复台账

> 状态注记（2026-09-28）：下方早期执行记录保留为历史快照；本节追加记录后续集成及审查，不覆盖既有证据。

## 集成更新

- 当前起始 BASE 仍为 `76df0cee0bf3ae23c14411c345b151ad518077ee`；共享集成分支连续提交，未 cherry-pick，实施 Agent 的源文件写权限按责任文件划分。该环境不是各 Agent 独立 Worktree，已如实记录为共享目录串行集成。
- 已提交修复：Web 高项 `f35ed2427`；T26 首轮 `bc19b9fd3`；后端首轮 `b2e6a1e7b`；Web 中项首轮 `50932d609`；导出/删除恢复两轮 `1a4acb662`, `a511c8960`；T26 首次 Review 修复 `443ed6982`；后端 reminder 删除竞争 `b3006beb0`；Search URL import mismatch `8c58a9a27`；Web medium Review 修复 `535c414db`；API digest `8eaa6ba7e`；Submission forbidden 异步修复两轮 `c9ef0147c`, `fd2ee0a20`；T26 服务并发/controller 修复 `cf19364ff`。
- 已有专项 Reviewer 结论：Search import `task-search-import-r4-review.md` 通过（仅 low 测试建议）；ExportDeletion 第二轮 `task-export-deletion-r4-review-fix2-review.md` 通过；T26 `443ed6982` 未通过（并发预留/跨调用 abandon/交互覆盖），由 `cf19364ff` 修正大部分，后续 reviewer 又发现 export recovery 未纳入共享 active 状态，待修；Submission 两轮分别见 `task-submission-forbidden-race-review.md`、`task-submission-forbidden-race-fix2-review.md`，后者指出 history forbidden 未完整清除详情，待修；Web Medium Review 指出的 3 项由 `535c414db` 修复，复审发现迟到读取权限竞态，已有 `fd2ee0a20` 修正并复审仍提出一个 forbidden history pane-reuse 清理问题，待修。
- 后端删除竞争在 SQLite WAL barrier/race 检查和 Career suite 通过；PostgreSQL `FOR UPDATE` 语义未能在本地验证（无测试 DSN）。
- OCR 第五轮完整 `BASE..HEAD` 调用生成 `ocr-round-5.md`，但由于分组 token 估算超过限制，20/20 文件组 failed；budget-limited resume 同样 skipped；无预算 resume 长时间不返回而停止。API 单提交 OCR 因 provider 请求 cancelled。`ocr llm test` 连接成功。上述报告均不是 OCR 通过证据。待最终修复稳定后以低并发/合适分组恢复并取得完整覆盖报告。
- Mini Program typecheck 仍受未修改账号页 `CommercialSummary` 13 个 TS2339 错误阻塞；`build:weapp`、tests pass。根工作区 issue30-sweep 改动与截图未触碰。
- 新增修复：T26 共享并发/controller `cf19364ff`，导出 recovery 互斥补丁 `d5a3c6f57`（复审 ruling `c1d10e3ab`，F2 approved；F3 rendered-page coverage 未覆盖）；Submission forbidden-history 私密清理 `c189ea5f4`（独立复审通过）。
- 最终 Web 全量 `pnpm test:web`：PASS，2527/2527，0 failed；`pnpm typecheck:web` 在最终 Submission 修复前启动并通过，之后提交仅更新相同 Submission 文件，Agent 独立复审和验证均通过。Go 最终 `go test -count=1 ./internal/modules/career` PASS。Mini Program full test 201/201，Taro build PASS，typecheck 为上述无关 baseline 错误。
- 当前剩余质量限制：没有真实 Taro 页面渲染 harness/device 验证；T26 页面 async handlers 是 controller/service 层测试，不是页面渲染级测试。此项记录为 evidence gap，OCR final 是否再报有效问题待决。
- OCR 所有 round-5 报告保存在 `docs/plans/issue-140/ocr/`。此前全范围 OCR 调用取消/超预算；一次 20-file 调用 exit nonzero（20/20 failed），因此不是通过。正在以 source/test 范围、低并发、低 effort 再跑；文档报告另需完整 workspace OCR 覆盖。未取得覆盖完整且成功的报告前，OCR gate 仍 pending。

- 计划：`docs/plans/2026-09-28-issue-140-ocr-r4-repair.md`
- BASE：`76df0cee0bf3ae23c14411c345b151ad518077ee`
- 集成工作区：`/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01`，分支 `codex/issue-140-integration`
- 起始时存在 7 份 round-4 OCR 原始报告未跟踪，保留原样；用户主工作区 issue30-sweep 改动和 `leg1-01-awaiting.png` 不触碰。
- 验证/实现 Agent 均需显式使用上述 workdir；独占的写文件面不重叠。没有 push/merge/publish/close Issue。
- Review 并发可对不同专属提交进行，只读。

## Tasks

| Task | Owner role | Agent | 独占范围 | 状态/检查点 |
|---|---|---|---|---|
| T1 Mini Program gates/recovery | frontend_implementer | `/root/t26_gate_fix`（模型和推理信息待运行时回报） | 两个 mini-program career 页面、两项测试、独立报告 | implemented `bc19b9fd3`; review pending |
| T2 Career Web/API high | frontend_implementer | `/root/career_web_high_fix`（模型和推理信息待运行时回报） | Progress/Search/ExportDeletion、测试、api-client career decoder/tests、独立报告 | implemented `f35ed2427`; review pending |
| T3 Backend export/error/concurrency | backend_implementer | `/root/career_backend_ocr_fix`（模型和推理信息待运行时回报） | Career export/handler/reminder 与对应 tests/报告 | running |
| T4 Career Web medium recovery | frontend_implementer | `/root/career_web_medium_fix`（已再次指明显式 workdir） | Material/Submission/Preparation/Rule/Opportunity 页及测试，SearchPage 归 T2 不碰 | running |

Review agents: `/root/t26_repair_review` 只写 `task-26-ocr-r4-review.md`; `/root/career_web_high_review` 只写 `task-career-web-ocr-r4-review.md`。

## Baseline Verification

- `pnpm test:web`: PASS，2503/2503，0 failures，exit 0（HEAD 76df0cee0）。
- `pnpm typecheck:web`: PASS，exit 0（HEAD 76df0cee0）。
- Mini-program tests: PASS，190/190（HEAD 76df0cee0；`/tmp/issue140-mp-baseline-test.log`）。
- Mini-program typecheck: FAIL，12 条账号页 `CommercialSummary` TS2339 diagnostics，不涉及 T26 Career files；此前总账称 13，当前精确基线输出需核对。
- Mini-program baseline build not run before edits; post-fix `build:weapp` is reported PASS in T1 task report.
- Career Web related baseline: 83 tests pass on 76df0cee0, but no assertions covered malformed read views / mismatched receipt IDs; see `task-career-web-ocr-r4-validation.md`.
- Backend baseline: `go test -count=1 ./internal/modules/career` PASS (backend validator report).

## Integration and Review

- T1 commit `bc19b9fd3`: implementer reports Mini Program suite 191/191, build pass with existing warnings, typecheck blocked by baseline account-page errors; independent reviewer pending.
- T2 commit `f35ed2427`: implementer reports targeted four-file suite 87/87, `typecheck:web` pass and `git diff --check` pass; independent reviewer pending.
- Current integration HEAD observed after commits: `bc19b9fd3`. Remaining source edits in `git status --short` belong to active T3/T4 agents; do not stage or modify their ranges.
- Integrate nothing further until reviewers return Spec-compliance and code-quality conclusions. Re-run tests against final integrated HEAD, then run OCR workspace covering all delivered uncommitted content and/or BASE..HEAD as required. Any content change after review invalidates affected evidence.
