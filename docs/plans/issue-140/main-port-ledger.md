# Issue #140 main 架构移植执行 Ledger

## 事实源与基线

- 根 Issue：<https://github.com/1123786563/WeKnora-fork01/issues/140>；认证 GitHub REST API 递归审计于 2026-09-28 完成：#140 + #141–#173 共 34 节点、33 条正式 contains 边、无更深子 Issue；全部 open / ready-for-agent，评论为空，和 2026-09-24 快照无变化。
- 正文依赖来源：每个子 Issue 的 `Blocked by`。完整 `depends_on` 与拓扑在 `2026-09-28-issue-140-main-port-dag.md`。
- 起始 BASE：`db234c5eb171f2dde7427d382b55b503a038f879`，来源 `.worktrees/issue30-sweep` 实际分支 `codex/issue30-mobile-office`。复核其 `git status --short` 得到 5 个未跟踪文件：两份 OCR 报告及 T55/T63/T64 计划；全部留在原工作区，未复制、未改动、未纳入。
- 执行工作区：`/Users/wuyongjun/.codex/worktrees/issue-140-issue30-base/WeKnora-fork01`；分支 `codex/issue-140-issue30-base`。创建时 HEAD 是上述 BASE，当前仅领先一次设计文档 cherry-pick `fa0b882def7e78ff3599cc7bef204ad2398063e5`。
- 目标架构证据：本 BASE 已有 `internal/modules/*`、Workbench admission / TaskBudget、`packages/contracts`、`packages/api-client`、`packages/mobile-core`、Web TDesign feature shell、Taro platform adapters 和 Expo composition。Read-only `main_arch_delta` 研究指出与当前 main 的目录和 migration track 有差异；因此将目标限制为在此指定 BASE 上使用可复用的当前模块化 seams，不合并旧 `codex/issue-140-integration` 分叉，不声称与 main 完全无差异。后续需在最终报告列出移植边界。
- Spec/设计：产品规格 `docs/specs/2026-09-23-weknora-job-search-design.md`，ADR 0015–0018、`CONTEXT.md`；技术设计 `docs/superpowers/specs/2026-09-28-issue-140-main-architecture-port-design.md`，用户 2026-09-28 已确认。
- 旧 `docs/plans/issue-140/2026-09-24-issue-140-dag.md` 与历史实现/Review/OCR 仅作验收/行为参考；旧 verified 状态不适用于本次架构移植。

## Plan / SDD

- Plan: `docs/superpowers/plans/2026-09-28-issue-140-main-port.md`
- Fresh SDD workspace: pending setup from `scripts/sdd-workspace` when execution begins.
- 起始检查点：`db234c5e` → `fa0b882d`（批准设计文档）；计划与 DAG 尚待纳入后续任务提交。

### Plan preflight consistency scan

| Pair / task | Shared file or interface | Check and ruling |
|---|---|---|
| Task 1 → Task 2 | Career DTOs and API boundary | Task 1 owns schema/index; Task 2 consumes it. No API implementation begins before DTO fixture tests pass. |
| Task 2 → Tasks 3–6 | Career module ownership, migrations, auth/repository seam | Task 2 creates module foundation and first schema; Tasks 3–6 own distinct domain packages and must coordinate paired migration ownership. Central sequence is serialized. |
| Tasks 3/4/5/6 | central router/container/migration registrations | Shared files are integration-owner-only. Ruling: only the integrator applies registration patches, after domain code and scoped tests exist. |
| Tasks 7/8/9 | `CareerApi` decoded DTOs | Client feature code may run in parallel only after Tasks 1–6 reviewed commits are integrated. Contract method additions after freeze require all three affected clients to revalidate. |
| Task 7 own-text check | Web route/nav/i18n registries | Worker owns career feature/test and locale content; integration owner edits registries. Tests include all five locales and route guard. |
| Task 8 own-text check | Taro config, runtime scope and native controls | Worker owns Career feature/subpackage/adapters; integration owner edits global page config. TDesign dependency/build proof is mandatory; no DOM reuse. |
| Task 9 own-text check | Expo app composition/native adapters | Worker owns Career implementation; integrator performs the shared composition wiring. Native environment gaps stay blocked. |
| Task 10 own-text check | integrated commits and device gates | All verification binds exact integrated HEAD; absence of a native environment is reported as blocked, not passed. |

The task ordering is acyclic: 0 → 1 → 2 → 3 → 4 → 5 → 6 → {7,8,9} → 10. The plan’s expected test names and commands correspond to the owned paths; package script names are verified by each implementer before dispatch.

## Task Status

All implementation nodes start `pending`; an Issue’s historical implementation status cannot make any node `verified`.

| Task | Status | Checkpoint / evidence | Review / ruling |
|---|---|---|---|
| 0 Baseline ledger / DAG | verified | live audit returned 34/34 nodes; source DAG SHA256 `3b95634a…`; full issue snapshots and current DAG committed at `a443dbae230496ceca6b23044d8df79b7a3c031d`; `git diff --check` clean | plan self-review complete; issue-140 reviewer confirmation pending |
| 1 Career wire contract/API client | pending | — | — |
| 2 Career module/auth/schema | pending | — | — |
| 3 Profile/source/opportunity/evaluation | pending | — | — |
| 4 Search/Workbench Task/budget | pending | — | — |
| 5 Application/material/submission | pending | — | — |
| 6 Timeline/reminder/privacy | pending | — | — |
| 7 Web client | pending | — | — |
| 8 Mini Program client | pending | — | — |
| 9 Expo/Harmony client | pending | — | — |
| 10 Integration/final verification | pending | — | — |

## Rulings

- Ruling: Use the verified `issue30-sweep` HEAD `db234c5e` as this plan’s original BASE, and implement against its modular Workbench/contracts/Expo/Taro architecture. Reason: the task attachment explicitly requires this committed worktree as starting point; read-only architecture comparison confirms that it contains the relevant modern seams. Cost if wrong: resulting branch history/diff may need a later transplant to today’s main; no shared branch is modified.
- Ruling: Do not bring the five untracked source worktree artifacts into the plan. Reason: attachment explicitly protects the existing issue30-sweep working scene. Cost if wrong: those documents remain available only at their original worktree location.
- Ruling: Do not inherit legacy issue140 verified/review/OCR outcomes. Reason: changed module/API/client architecture invalidates those code-level claims. Cost if wrong: additional revalidation work, but no unreviewed legacy implementation is accepted.

## Verification, Review and OCR Record

No implementation verification, SDD review, or OCR has run for this plan yet. Record each command, exact HEAD, exit code, result summary and report path here as it occurs. Final OCR must cover the full selected BASE-to-HEAD delivery range and any uncommitted delivery content.
