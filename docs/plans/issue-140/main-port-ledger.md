# Issue #140 main 架构移植执行 Ledger

## 事实源与基线

- 根 Issue：<https://github.com/1123786563/WeKnora-fork01/issues/140>；认证 GitHub REST API 递归审计于 2026-09-28 完成：#140 + #141–#173 共 34 节点、33 条正式 contains 边、无更深子 Issue；全部 open / ready-for-agent，评论为空，和 2026-09-24 快照无变化。
- 正文依赖来源：每个子 Issue 的 `Blocked by`。完整 `depends_on` 与拓扑在 `2026-09-28-issue-140-main-port-dag.md`。
- 起始 BASE：`db234c5eb171f2dde7427d382b55b503a038f879`，来源 `.worktrees/issue30-sweep` 实际分支 `codex/issue30-mobile-office`。复核其 `git status --short` 得到 5 个未跟踪文件：两份 OCR 报告及 T55/T63/T64 计划；全部留在原工作区，未复制、未改动、未纳入。
- 执行工作区：`/Users/wuyongjun/.codex/worktrees/issue-140-issue30-base/WeKnora-fork01`；分支 `codex/issue-140-issue30-base`。BASE `db234c5e` 后依次提交批准设计 `fa0b882d`、计划/DAG/intake `a443dbae`、Task 0 摘要 `37a5af02`、测试命令修正 `79df6990`；当前 HEAD 以 `git rev-parse HEAD` 为准，Task 0 仍在 Review 修复中。
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

Plan command correction: inspection of the selected baseline’s workspace manifests showed `packages/contracts` and `packages/api-client` have no individual `test` scripts. The shared gates are root `pnpm test:shared` and `pnpm typecheck:shared`; Task 1 runs a focused `pnpm exec tsx --test` RED test and then those root gates.

## Task Status

Implementation nodes start `pending`; PH (#143) is a pending early Harmony gate and P145 (#145) is an independently ready iOS/Android Task-boundary slice. An Issue’s historical implementation status cannot make any node `verified`.

| Task | Status | Checkpoint / evidence | Review / ruling |
|---|---|---|---|
| 0 Baseline ledger / DAG | verified | live audit returned 34/34 nodes; source DAG SHA256 `3b95634a…`; plan/DAG snapshots integrated through `9fd22803021a35da1c12932a0385f669392c8da8`; `git diff --check db234c5e..HEAD` PASS. Baseline `pnpm test:shared` PASS (1122 pass, 4 skipped); `pnpm typecheck:shared` has unrelated unchanged `packages/views/src/chat/mermaid.ts:127,158` errors. | Independent review `.superpowers/sdd/2026-09-28-issue-140-main-port/task-0-final-verified-review.md` PASS for spec and quality at fix-worktree checkpoint `5b43589c4ef88e8c81613554aeea47bdbef650ab`; 46 Markdown files, 34 snapshots, 33 children mapped, acyclic issue DAG, 9 local links resolve. Review covered docs/planning only; no implementation/device/OCR claim. |
| 0A Harmony feasibility trial (#143) | blocked-conditionally | Worktree `/Users/wuyongjun/.codex/worktrees/issue-140-p0a-harmony/WeKnora-fork01`; evidence commit `8077b06`, report/clean HEAD `36bc7c3`; independent report `task-0a-validation.md` DONE_WITH_CONCERNS. Expo 55/RN 0.83.10 Prebuild skips Harmony; examined RNOH 5.0.0.813 pins RN 0.72.5. | Current pinned stack has no demonstrated maintainable native Harmony route. HAP/device/auth/capability probes remain unverified; affected Harmony-only acceptance cannot close without approved compatible host/replatform decision. #145 remains independent. |
| 0B Expo iOS/Android Task boundary (#145) | running-review | Worktree `/Users/wuyongjun/.codex/worktrees/issue-140-p0b-task/WeKnora-fork01`; feature `4682dcf`, report `8a7d454`; report `task-0b-report.md`. Focused tests 4/47 pass, mobile suite 279 pass/14 credential-gated skipped, typecheck PASS. | iOS first build printed BUILD SUCCEEDED but incremental run had SwiftCompile internal error; Android assembleDebug failed after 10m36s fetching javapoet with TLS handshake. No install/launch/screenshots, no authenticated Task read; global Expo route wiring remains integration-owner work. Independent validator DONE_WITH_CONCERNS; independent review `task-0b-review.md` FAIL/changes required: High F1 stale cached/store data can publish after lease revocation, High F2 stream guard adopts any new active lease, High F3 route/device/auth gates absent, Medium F4 capability symbols mislabeled available. R1 code repair `a9d7d20` fixes stale hydration/stream lease races and reports capability APIs as installed-untested; independent validation/review pending. F3 route/device/auth evidence remains pending; #145 not verified. |
| 1 Career wire contract/API client | verified | Integrated code through `f147a763d9ad6b4d72482a3bee4a820149ee7795`; Task 1 `75b6343`, hardening waves `a1be4d5`, `e6a4ce5`, `e2040f7`, `b48c24f`; report `docs/plans/issue-140/reviews/task-1-contracts-api-desk.md`, subsequent wave reports and review reports. At integration HEAD, offline frozen install PASS; Career focused suite 41/41 PASS; strict Career tsc PASS; `pnpm test:shared` 1146 passed / 4 skipped; `git diff --check db234c5e..HEAD` PASS. `pnpm typecheck:shared` remains only the recorded unrelated `packages/views/src/chat/mermaid.ts:127,158` baseline failure. | Task 1R4 independent Spec and quality review PASS; no findings (`.superpowers/sdd/2026-09-28-issue-140-main-port/task-1r4-review.md`); review checkpoint `ef9b964431f8ff8937c9e5a9ceb9a974a842db35`. Task 1 and all correction waves integrated. |
| 1R Contract/API/Desk hardening | verified | Strict response/request DTOs, canonical CareerApi surface, shared Desk, typed 403/409/unknown protocol, same-ID lookup, conflict-only new-key rebase, monotone reads, durable write persistence, explicit observer support, deployment-origin guard; implementation/review reports under `docs/plans/issue-140/reviews/`. | Spec PASS / quality PASS at reviewed checkpoint; downstream may consume this integrated interface. |
| 2 Career module/auth/schema + #142 Artifact grant | running | Initial `342dee842b8e825d9a84852fc47c13db1d4befec`; R1 `fd683e903e540b2de160f4194f56fdf7b6b1ec80`; R1a `9b20522cb4ba3c985a4b97f9a0f3b564524bc179`; final R1 `b5c68450dfa32324f19dd4775fdac98e0d899f63`. Reports/reviews in docs/plans/issue-140/reviews/. | Overall #142 lifecycle remains open until R2 routes/catalog/download digest pass. Live PostgreSQL limitation remains explicit. |
| 2R1 Career caller-scope / TTL / repository validation | verified | Integrated source through `6a79b3b4c`; Caller execution-tenant override rejected, TTL nanosecond precision, SQLite GORM owner isolation/roundtrip/FK/append-only/up-down, table-specific PostgreSQL SQL fallback. Full backend package suite PASS; validator and independent final review PASS at `3fc8f6bbfbaa17a807b54c03310ce14d96501fe9`. Live PostgreSQL migration/GORM unverified (service recovery; no tools). | Spec PASS / quality PASS for R1 scope; no new findings |
| 2R2 #142 issuance / download lifecycle | verified* | Integrated through `14cd0411f` (source tip `64948c4b477b53d3726179d5da115a401d0ee45a`); R2 original route/catalog/migration, owner-bound issuance, public HMAC download, digest/size staging, legacy route compatibility, per-request owner/revoke reauthorization, process staging budget and cleanup. Implementation report `task-2r2-artifact-download.md`; findings/fixes and reports `task-2r2-*` under `docs/plans/issue-140/reviews/`. SQLite single-connection production tenant+storage resolver regression passes; backend A→B default switch reads immutable version from stored backend. Independent backend validation and final Spec/quality review PASS. | #142 end-to-end closure remains gated on Task 5 production publisher calling `BindVersion` and a real Career material publish→download acceptance path. Live PostgreSQL and external providers unverified; per-process budget is not shared across replicas. `verified*` denotes R2 scoped checkpoint; parent Task 2 remains running until Task 5 integration. |
| 3 Profile/source/opportunity/evaluation | repair-required | Worktree `/Users/wuyongjun/.codex/worktrees/issue-140-task3-domain/WeKnora-fork01`; implementation `9e489c8`, report `3b08964`. Backend validator passed targeted/race checks with concerns; reviewer report `task-3-review.md` found 2 High + 3 Medium, Spec/quality FAIL. | R1 repairs High false-eligible and forged source observations; R2 repairs concurrent confirmation and cascade deletion; ambiguous/negated graduation parsing (Medium) must also be assigned before integration. #150 skills/projects/intent remains incomplete. Do not integrate until findings are repaired, revalidated and reviewed. |
| 4 Search/Workbench Task/budget | pending | Read-only interface research docs/plans/issue-140/task-4-seam-research.md committed at 07b66f06; based on integrated b5956a9a0. Workbench admission/read seams exist; Task 3 reviewed profile/opportunity/evaluation fields still gate implementation. | Candidate safe split recorded; no Task 4 implementation released before Task 3 checkpoint |
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

## Parallel wave after Task 2 scoped checkpoints

- Active wave BASE: 477da9f677e0357ea8bc360c2802314fef534723, selected integrated issue30-sweep port branch. Independent worktrees: P0A/#143 Harmony feasibility; P0B/#145 iOS/Android Task boundary; Task 3/P1 profile-source-evaluation. Files and platform artifacts are isolated; central router/container/migration registrations remain integration-owner controlled.
- Prior work discovery: no active child agents were present in the collaboration tree at dispatch. Existing old issue-140 worktrees are completed/retained artifacts, not active jobs. Old Task 3 branch 2e966ac1c has eight commits from common ancestor f7753fa; it predates the issue30-sweep BASE db234c5e and current API/module integration, so its behavior is reference only and requires architecture port.
- Host capability snapshot: Xcode 27.0 and booted iPhone 18 Pro simulator available; adb, hvigorw and hdc not found. These observations guide attempts but do not satisfy native gates.
- Ruling: release Task 3 while parent Task 2 remains running for its later Task 5 artifact-publisher closure. Task 3 consumes the integrated/reviewed owner/tenant Career foundation; #142 publisher lifecycle is not consumed by profile/source/evaluation. Cost if wrong: P1 domain branch can be paused/rebased before central integration; no shared registration is changed.

## Task 2R2 review record

- Source branch `codex/issue-140-task2r2-download`; initial implementation `19097b784...`, final source `64948c4b477b53d3726179d5da115a401d0ee45a`; integrated checkpoint `14cd0411f` on `codex/issue-140-issue30-base`.
- Final independent review `task-2r2-final-final-review.md`: Task 2R2 scoped Spec compliance and code quality PASS; earlier High/Medium findings closed. Validator `task-2r2-backendfix-validation.md` PASS with provider/live-PostgreSQL limits.
- Lifecycle remains open until Task 5 binds published immutable material versions.

## Verification, Review and OCR Record

Task 0B and Task 3 implementation verification/review are in progress; no OCR has run for this plan yet. Task 0 first independent review is complete and found 4 high / 2 medium documentation/coverage findings; retain report `.superpowers/sdd/2026-09-28-issue-140-main-port/task-0-review.md` and re-review after fixes. Record each command, exact HEAD, exit code, result summary and report path here as it occurs. Final OCR must cover the full selected BASE-to-HEAD delivery range and any uncommitted delivery content.

- Exact review evidence: `git diff --check fa0b882d 79df6990` returned exit 2 because plan lines used trailing spaces to render Markdown hard breaks; remove these spaces and rerun before marking Task 0 verified.

### Task 0 final independent review

- Reviewed checkpoint: `5b43589c4ef88e8c81613554aeea47bdbef650ab` (`c5e4ca25b678f1e310afb546017b8cb4d0593b7` tree); integrated docs checkpoint: `9fd22803021a35da1c12932a0385f669392c8da8`.
- Report: `.superpowers/sdd/2026-09-28-issue-140-main-port/task-0-final-verified-review.md`; Spec compliance PASS and plan quality PASS, no remaining findings.
- Reviewer ran `git diff --check fa0b882def7e78ff3599cc7bef204ad2398063e5..HEAD` (exit 0), verified 34 snapshots, 33 mapped children, dependency targets and acyclic topological order, 9 local links. The reviewed diff is docs-only; implementation and device gates remain pending.

### Task 1 baseline (before implementation)

- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-t01-contracts/WeKnora-fork01`, branch `codex/issue-140-t01-contracts`, BASE `37a5af02d9deddba1292c66f02924182fda38166`.
- `pnpm install --frozen-lockfile --offline`: PASS; all dependencies reused from cache, install confined to ignored worktree `node_modules`.
- `pnpm test:shared`: PASS, 1126 tests, 1122 pass, 0 fail, 4 skipped, exit 0.
- `pnpm typecheck:shared`: FAIL at unchanged `packages/views/src/chat/mermaid.ts` lines 127/158 (dark/light theme literal incompatibility and missing `themeVariables` property), exit 2. This is the clean baseline’s unrelated type error; record and do not modify it in Task 1. Re-run the same gate after the task to confirm it remains unchanged, plus focused Career type checks.

### Task 0 follow-up: P6 Career Desk consumer seam

- Parent review identified that P6 still named CareerApi without requiring the shared Career Desk. Task 8 now consumes `@weknora/career-core`; Career Desk owns `open/list/act/observe/reconcilePending` and scope invalidation. Taro provides only scope mapping, durable intent-store, and platform adapters.
- Added named assembly/scope test `MiniCareerDesk_Assembly_ScopeSwitchInvalidatesPendingIntentsAndDropsLateReplies`, covering original request ID preservation, scope invalidation, late reply rejection and Desk port delegation.
- This correction is committed as `73022bb85a4f75a3d23a1a6bb92e3f3bc3727ecd` (`docs: require career desk in mini program slice`); Task 0 remains `running` pending independent review.

## Task 0 latest re-review correction checkpoint

- Re-review source: `.superpowers/sdd/2026-09-28-issue-140-main-port/task-0-re-review.md`, reviewed through prior checkpoint `d23246a9ba4f8aa91e97ad7610afe125bad133bc`.
- Docs-only correction commit: `c544e4392f31569a0a390ae5fc54c838462ab9e7`; tree: `ba1d31fd358329c0a713e2119a35fb44f842dbbe`.
- #144 now belongs to P7 / Expo Task 9; P2 only provides the consumed authorized Task read seam, and P6 no longer maps #144. Task 9 sequences #145 Task boundary before #144 entry/restart/offline/scope behavior; #144 closure waits for #143 PH ruling and #145 evidence.
- #169 belongs to P6 / Mini Task 8 with timeline order/correction, submitted-version/unknown preparation, account-switch cache invalidation and real WeChat DevTools/device test evidence.
- #156 `CareerPreparationService/API`, `preparation/**` and named service tests are consistently owned by Task 6 / P4 after Task 5 records the submitted material version.
- Web and Expo briefs explicitly consume the ADR 0019 Career Desk and provide named assembly tests. The dangling discovery-record link was removed from the approved spec; the wording now states the historical raw notes are not in this baseline, without inventing a replacement source.
- Verification commands/results: inline `python3` structural assertions — PASS for #144/#169/#156 task ownership, prereqs/tests, Web/Expo Desk consumption, and the repaired link; owned-Markdown relative-link check — PASS; `git diff --check` — PASS (exit 0). Exact final report and range checks are recorded in `docs/plans/issue-140/reviews/task-0-plan-fix.md`.
- Task 0 stays `running` pending the controller's next independent re-review.

## Task 0 final-review dependency correction

- Final-review finding: P7 had inadvertently gated the independent #145 iOS/Android thin slice on PH/#143 and P0–P4. Corrected plan/DAG now give #145 its own P145 node with no dependencies; it can proceed beside PH and Career API/backend work.
- #144 remains in P7/Task 9 and cannot close until both PH/#143 ruling and verified P145 evidence are integrated. Harmony-specific P7 work alone waits for PH; #145 iOS/Android is never blocked by it.
- Task 9 consumes Task 0B/#145 before implementing #144; the W0/W2 schedule and 0A/0B ledger statuses reflect this split.
- Task 0 remains `running` pending the next independent re-review.

### Final-review fix checkpoint

- Correction commit: `2f5014621c7edd6f9d1c67887c174201bbffde68`; tree: `d97b1766a12e6487804f1e17af12917d54adb317`.
- `P145`/Task 0B owns #145's iOS/Android authenticated Task boundary with `depends_on: —`; it is independent of PH/#143 and P0–P4. The #145 issue snapshot has `Blocked by: None`.
- `P7`/Task 9 owns #144; it consumes verified P145 and PH/#143 ruling. Its iOS/Android work may proceed without PH, but #144 cannot close until both blockers are resolved; PH gates Harmony-only work and overall #144 closure.
- Structural dependency assertions and `git diff --check` passed. Full command descriptions, output and checkpoint hash are recorded in the Task 0 fix report.
- Task 0 remains `running`; awaiting the next independent review.

## Task 0 latest #145 UI acceptance correction

- Latest independent report: `.superpowers/sdd/2026-09-28-issue-140-main-port/task-0-last-review.md`, reviewed checkpoint `3e0552d04772238eef04bad1f662c17a74d9d276`.
- Plan/DAG correction: `b7ca28a9c00bc43beef9cd2f601ba917c8bc9f60`; tree `83f5c8402a5f27bf7c8b2aade98c6d3c6193fa24`.
- P145/Task 0B now owns native React Native Task entry mapping to TDesign Mobile React visual tokens and interactions and prohibits mobile Web imports. Named checks: `ExpoTaskOffice_NativeBoundary_UsesReactNativeTDesignTokensAndInteractions` and `ExpoTaskOffice_NativeBoundary_NoMobileWebComponentImports`. iOS/Android visual-interaction evidence is required before P145 may be verified/integrated.
- Task 9 consumes this full verified P145 checkpoint before #144 implementation.
- Structural assertions against issue #145 and ADR 0018 and `git diff --check` passed. Exact range, report and outputs are in the fix report.
- Task 0 remains `running` pending another independent re-review.

### Parallel wave evidence — 2026-09-29

- P0A/#143 implementation `8077b06e` + report HEAD `36bc7c3`; independent validation `task-0a-validation.md` DONE_WITH_CONCERNS. Current Expo/RN stack has no demonstrated maintainable Harmony native path; HAP, device, auth and native capability acceptance remains unresolved. No remote issues were changed.
- P0B/#145 feature `4682dcf31e3ac3dfb8c87adeb0d49b57c8ce8944`, final report `8a7d45474` in its isolated worktree. `pnpm --filter @weknora/mobile typecheck` PASS; focused task-entry 4/4 and Task detail 47/47 PASS; full mobile suite 279 pass, 14 credential-gated skipped, 0 failures. First iOS Xcode build said BUILD SUCCEEDED, later incremental log recorded a SwiftCompile internal error; Android authoritative Gradle run failed resolving `com.squareup:javapoet:1.13.0` due remote TLS handshake after 10m36s. No app launch, auth read, screenshot or route wiring; independent validation and review pending.
- Task3 feature `9e489c8b60786f5397d99e947e39fa3fd8f6c14f`; implementation report `3b08964545357d19dfefafdea55b122ad68825e7`. Backend validator `task-3-validation.md`: race and migration-specific suites PASS with concerns. Reviewer `task-3-review.md`: Spec FAIL / quality FAIL; High findings are false eligibility and caller-forged source observations; Medium findings are distinct-ID confirmation collision, cascade delete blocked by append-only triggers, and ambiguous/negated graduation parse. R1 (`bd49b30`) and R1b (`e21294e`) add scoped persisted observations and conservative hard-requirement parsing/source labels. R1 review remains FAIL: unresolved polarity/alternatives (`不招 2026 届`, `工作地点：北京，上海`); R2 review `task-3-r2-review.md` remains FAIL: purge still blocked by evidence/artifact FK/triggers; cross-proposal same-request-ID race can commit an unrecoverable fact. R2b repair now underway from latest R1b branch and includes all findings; broader skills/projects/intent still incomplete. Integration is prohibited pending closure.

- P0B independent validation report `task-0b-validation.md`: DONE_WITH_CONCERNS; focused source tests pass but backend auth and real devices absent. Reviewer report `task-0b-review.md`: Spec incomplete / quality changes required (F1–F4). R1 workstream is assigned.
- Task3 R1 exact HEAD `30c6fc6`: owner-scoped persisted observation verification and conservative hard-requirement evaluation committed; Go suite/diff-check pass. Reviewer/validator are checking R1 against original findings. Task3 R2 has separate correct-architecture managed worktree; brief now present there.

- Task3 R1 validation passed the scoped stored-observation/digest and common unknown-hard-requirement cases, but both R1 and R1b independent review verdicts are FAIL on residual JD parser polarity/alternatives. Task3 R2 implementation `bda9edc` passed the assigned tests, but its independent review found two additional High integration gaps: evidence/artifact children still prevent Career space purge; same request ID racing across different proposal IDs may commit two transitions before receipt conflict. R2b is assigned these exact findings on top of current R1b HEAD.
- Task0B R1 commit `a9d7d20` repairs exact-lease checks around async persistence/stream callbacks, invalidates stale view and downgrades capabilities to installed-untested. Independent checks are underway; F3 remains blocked on route, real platform UI/auth probes and Android TLS dependency failure.

- Task3 R2 independent review of `bda9edc` rejected the checkpoint: domain-record-only cascade test omitted `career_evidence`/`career_artifact_bindings` children, and same request ID reused concurrently across two proposal IDs could persist two transitions before receipt collision. R2b assigned against R1b HEAD `0b13ecf`; it also includes parser residuals `不招 2026 届`, `2026届不招`, and ambiguous location alternatives.
- Task0B R1b implementation commit `89fe623f` closes on exact lease, clears controller view synchronously on revocation, and guards command acknowledgements; independent validator/reviewer pending. #145 F3 is still not verified: no route/device/auth proof and native builds inconclusive/failing.

### Expo Task Office route R1/R2 checkpoint

- Integrated route implementation checkpoints: `685641f3d` (entry route), `669280a7b` (synchronous lease invalidation/latest-read-wins), and `320df1db2bfb3b3f3ce058df84ca657dd655ad9b` (R2 moves lease listener from render-time construction to committed effect mount and cleanup).
- R2 exact independent review range `669280a7b..320df1db2`: Spec compliance PASS; code quality PASS; no critical/high/medium finding. Report: `.superpowers/sdd/2026-09-28-issue-140-main-port/taskoffice-route-r2-review.md`.
- Independent validation at exact HEAD `320df1db2`: focused state tests 5/5, app smoke 68/68, `pnpm --filter @weknora/mobile typecheck`, and `git diff --check db234c5..320df1db2` all PASS. Report: `.superpowers/sdd/2026-09-28-issue-140-main-port/taskoffice-route-r2-validation.md`.
- The review confirms constructor purity, effect-only subscription, synchronous clear for already-revoked leases, cleanup/unsubscribe and invalidation of pending reads. No real React StrictMode route renderer or device UI was run. Thus route code review passes, but #145 remains unverified pending authenticated iOS/Android interaction/build evidence.
- Native follow-up: iOS workspace/pods installed; a fresh direct simulator build is running with logs at `/tmp/issue-140-main-taskoffice-ios-build.log` for dedicated iPhone 17e destination `0D4ED257-7E47-40CE-A9D1-840116F6FC63`. Android’s previous authoritative Gradle attempts failed on remote Maven TLS resolution. Authentication test variables are incomplete, so no authenticated read/screenshot is currently possible.
- Task3 R2f final independent review still rejects the parser checkpoint (High multiple-location clause false exact match; Medium unrelated parenthesis hides exact city); R2g repair is in progress in its isolated backend worktree and is not integrated.
- Read-only Task 4/5 architecture preparation at integration HEAD `320df1db2` is persisted in `docs/plans/issue-140/reviews/task-4-5-architecture-exploration.md`. It confirms Task 4 and 5 are serial (Task 5 consumes Task 4 admission), Task 3 source interfaces are not yet stable, Task 4 must isolate reconciliation from Task 3 `source/**`, central router/container/migration registrations stay integration-owned, and Task 5 must invoke `ArtifactCatalogStore.BindVersion` from the real material publisher. No downstream implementation has been prematurely dispatched.

### #143 Harmony gate closure — 2026-10-05

- **Closure ruling: the deliverable IS the gate determination.** #143's own acceptance row (`2026-09-24-issue-140-dag.md` T01) offers a blocked exit: 若无可维护路径，给出最小复现、失败证据、受影响 Ticket 和可选技术裁定. The ticket shipped exactly that work product:
  1. **Blocked-conditionally verdict** — `verification/harmony-native-gate.md` (2026-09-24): BLOCKED, no native build/device evidence; environment + supported-stack gate result, explicitly not proof that a future port is impossible. Reinforced by `verification/harmony-feasibility.md` (2026-09-29): Expo 55 / RN 0.83.10 has no demonstrated maintainable Harmony native route (RNOH 5.0.0.813 pins RN 0.72.5; Expo prebuild skips Harmony).
  2. **Probes** — exact commands/versions/toolchain search (DevEco/hvigor/hdc/ohpm absent), compatibility matrix, raw logs under `apps/mobile/harmony/validation/` (task-0a-environment.log, task-0a-native-attempt.log); implementation `8077b06e` / report HEAD `36bc7c3` in the isolated P0A worktree.
  3. **Independent review** — `task-0a-validation.md` DONE_WITH_CONCERNS accepts the gate result; ledger 0A row records blocked-conditionally with affected downstream enumerated.
- **Affected tickets**: T05/#144 Harmony-only acceptance and overall #144 closure stay blocked pending an owner-approved compatible host / replatform decision; #145 iOS/Android is independent and never blocked by PH. Exit criteria and reopen conditions are recorded in `harmony-native-gate.md` §Ruling.
- **Disposition**: #143 → **closed (deliverable complete; gate verdict stands as blocked-conditionally)**. No code, contract, or remote-issue state changed by this closure note.
