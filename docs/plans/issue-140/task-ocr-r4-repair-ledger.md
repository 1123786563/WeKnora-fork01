# OCR Round 4 修复台账

- 计划：`docs/plans/2026-09-28-issue-140-ocr-r4-repair.md`
- BASE：`76df0cee0bf3ae23c14411c345b151ad518077ee`
- 集成工作区：`/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01`，分支 `codex/issue-140-integration`
- 起始时集成工作区已有 7 份 OCR round 4 原始报告未跟踪；保留原样。用户主工作区既有 docs/plans/issue30-sweep 修改及 `leg1-01-awaiting.png` 不触碰、不纳入。
- 并行约束：Mini Program、Web/API-client、Backend 三条写流各自独立 worktree，测试端口与构建目录隔离。验证 Agent 只读写专属报告。

## Tasks

| Task | Owner role | Agent / model | 独占范围 | 起始状态 |
|---|---|---|---|---|
| T1 Mini Program gates/recovery | frontend_implementer | 派发中；模型/推理须从运行时回报核实 | `apps/miniprogram/src/career/application-material.tsx`, `export-deletion.tsx`, 专属测试/报告 | running |
| T2 Career Web/API decoder | frontend_implementer | 派发中；模型/推理须从运行时回报核实 | 三个 Career Web 页面及对应测试、`packages/api-client/src/career.ts` 对应测试、专属报告 | running |
| T3 Backend export/error/concurrency | backend_implementer | 只读核验进行中；待报告后按确认 finding 派发 | `internal/modules/career/career_export.go`, `handler.go` 与对应测试 | research |

## Baseline Verification

- `pnpm test:web`: PASS，2503/2503，0 failures，exit 0（在原 BASE 76df0cee0 上）。
- `pnpm typecheck:web`: PASS，exit 0（原 BASE）。
- `pnpm --filter @weknora/miniprogram test`: PASS；该命令后续链因 typecheck 失败被 `&&` 停止，具体测试总数需补录完整摘要。
- `pnpm --filter @weknora/miniprogram typecheck`: FAIL，错误集中 `src/features/account/pages.tsx` 的既有 `CommercialSummary` 字段类型；DAG 与先前报告已将 13 项记作 baseline errors。需补跑/核对错误数；weapp build 尚未运行。
- Web 相关基线验证：83 tests pass（ProgressPage/SearchPage/ExportDeletionPage/api-client career），但缺少本轮错误回执/错配/畸形响应回归用例；见 `task-career-web-ocr-r4-validation.md`。
- 完整 Go 门在本轮尚未重跑；等待后端 finding 核验并按 scoped tests 先验证。

## Review / Integration

等待各实现者提交 SHA 与报告。任何子任务只有在 validator、independent reviewer 双结论均通过，且工作树内容集成至本 BASE 后方可标 verified。随后集成工作区按先后 cherry-pick，核对 diff 和文件清单，并在集成 HEAD 补跑 Web 全量、Go Career 相关测试及可执行的小程序基线/build 门。
