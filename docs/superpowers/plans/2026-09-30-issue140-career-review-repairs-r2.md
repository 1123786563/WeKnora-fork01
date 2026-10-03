# Issue 140 Career Workflow Review Repairs R2 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement these tasks. Steps use checkbox (`- [ ]`) syntax for tracking.

## 2026-10-03 复选框核销与 low 发现裁定（核销轮对账）

**结论：本计划 25 个任务、124 个步骤复选框全部核销为已完成。** 此前未勾选状态是**文档落后于代码的漂移**，非工作未做：R2 波次实际经 sweep-integration 合并（`34d1412ee`，携带 405 提交）+ 13 个 `issue140-r2-*` 合并 + 8 个 task 合并全部落入 main，计划文件本身未回头核销。本核销以 2026-10-03 审计报告（`.superpowers/sdd/2026-10-03-career-audit/report.md`，main @ a315f79d0 只读核实）为证据源：§3 五项 Review Focus 逐项给出 file:line 代码证据，§4 career/workbench Go 套件全绿（exit 0）。

**勘误：** 审计报告 §2 与核销计划所称「33 个复选框」为计数误差——该计划文件在 a315f79d0 与 HEAD 上均含 **124** 个步骤复选框（25 任务 × 3–7 步）。漂移性质结论不变，本次按实际 124 个全部核销。

### 任务级核销证据表（每任务内复选框由该行证据整体覆盖）

| 任务 | 落地证据（审计 §3 行 × main 合并 SHA × 分支代表提交） |
|---|---|
| T1 删除完整性 | §3 删栅/撤销行（lifecycle_gate.go `admitLifecycleClaim`、career_export.go:1126 `deletionPurgeCareerData`）× merge `536f5f2e6` × d21b1ddb0 |
| T2 导出边界保真 | §3 撤销行（`purgeCareerRows` 枚举含 revoked 行）× `536f5f2e6` × d21b1ddb0 |
| T3a 规则重校验+列表 | §3 规则行（search_rule.go:21/:610）× `1a35d2a29` × 1f44705c9 |
| T3b Web 规则恢复 | `1f93483b9` × f63f6f053/25ef240c9 |
| T4 提交绑定 | §3 回执行 × `3e2e22205` × 04f08470c |
| T5 Web 档案录入/清除 | `35f14215c`（6bf56e0be/fa82b9d55）+ `7f4fb4a08` × 44bb18a49 |
| T6 Web 搜索恢复 | `db4771f46` × c2ab52569 |
| T7 小程序写持久化 | `6b6778cdd` × 5a2e3d73d 等 |
| T8 移动 auth smoke | `fc63fde70` × 507427b24/4d0b7fdc7 |
| T9 迁移身份保全（验证型） | 迁移现序 SQLite 124–145 / versioned 203–225（143–145 为 T13/T21/T17 追加，零重编号）；审计 §4 全绿；2026-09-30 用户裁定见 addendum |
| T11 周期 claim 原子化 | §3 规则行 × `e7c4d1e7e` × 18de01641 |
| T12 RulePage 恢复锁 | `1f93483b9` × 25ef240c9 |
| T13 跨实例删除门 | §3 删栅行 × `aecb713d0` × 76809f586/d1d85ddda |
| T14 保存编辑如实报告 | `6b6778cdd` × fd1d1b636 |
| T15 推送意图持久化 | `6b6778cdd` × 413c52601 |
| T16 scope 绑定 | `6b6778cdd` × 7112cf0c7 |
| T17 规则编辑保护 | §3 规则行 × `70d047b4a` × 7bfc8fbca |
| T18 可空 nextDueAt 解码 | `1f93483b9` × 271508f5c |
| T19 畸形回执边界 | `6b6778cdd` × 5eec14e06/ddb1893dc |
| T20 写锁/陈旧读围栏 | `1f93483b9` × 271508f5c |
| T21 上传 claim 所有权 | §3 unknown-request 行 × `c3e593449` × f3d542427/78f04fa7d（审计引 99e1d55a8/d420bb042 为同义短 SHA） |
| T22 提交→进度投影 | §3 回执行 × `7d384edd8` × a66cbf155 |
| T23 物料身份传递 | `3e2e22205` × 9fc83ceec |
| T24 直查回执绑定 | `db4771f46` × 24e9ef4db |
| T25 物料传递集成测试 | `3e2e22205` × c4b0a06d4 |

### 11 条 low OCR 历史发现裁定（源自部分会话 b5b5cf36，comments 导出 /tmp/issue140-ocr-fresh-comments.json；2026-10-03 对 main @ 210e164a1 逐条核码）

| # | 位置 | 裁定 | 理由/证据 |
|---|---|---|---|
| 1 | wx-driver.cjs:22-26 辅助函数与 t24r1 驱动重复 | accept-with-reason | T33 冻结证据快照（一次性重放工件），头部已注明「复用 T24/T26 模式」；非长期维护产品码，抽共享模块反而破坏证据自包含性 |
| 2 | wx-driver.cjs:40-42 静默 catch 吞错 | accept-with-reason | 同上，证据脚本定位为可归因 FAIL 断言；轮询容错的静默 catch 属驱动惯例 |
| 3 | button.tsx 复刻无回归护栏 | **fixed-by-r2** | `1db12dcca`（在 main）新增 apps/embed/src/embed-styles.test.ts：embed-btn--primary/--text、wk-emb-34、text-underline-offset 均有断言，embed 套件 21/21 绿 |
| 4 | button.tsx:1 死 React 默认导入 | accept-with-reason | tsconfig `jsx: react-jsx` 下零行为影响（现核：文件内无 `React.` 引用）；下次触碰该文件顺手移除，不值得立票 |
| 5 | EmbedEntryPage.tsx:826 死类+下划线偏移丢失 | **fixed-by-r2** | `1db12dcca` 已在 .wk-emb-34:hover 补 `text-underline-offset: 2px`（embed-u.css:320-323）并加测试断言；残留 `underline-offset-2` 类串因 Tailwind 已移除而 inert，无行为 |
| 6 | break-all 译为 overflow-wrap:anywhere 语义漂移 | accept-with-reason | anywhere 优先整词断行，窄容器长 URL/标题观感优于逐字符 break-all，属改良；若要求严格保真一行改回 `word-break: break-all` |
| 7 | #f8fafc→#f3f3f3 例外未在文件头标注 | accept-with-reason | 行内注释（embed-u.css:225）已载明对齐 Vue 基线 `--td-bg-color-secondarycontainer` 的依据；单一有据例外，头注重构收益不抵扰动 |
| 8 | miniprogram config 闭包种子 'button/button' 硬编码 | **needs-followup** | 真实护栏缺口：未来页面引入新 TDesign 组件（如 t-dialog）时构建期不报错、真机复现 D1 同款白屏；现仅 button 在用故未触发。建议按页面 usingComponents 反向扫描自动收集种子或构建期 throw |
| 9 | config/index.ts realpathSync 先于友好报错抛 ENOENT | accept-with-reason | 原生 ENOENT 已点名完整缺失路径，可诊断；「run pnpm install first」属 DX 打磨非缺陷 |
| 10 | files.ts ".pdf"（dot==0）扩展名边界 | accept-with-reason | 触发需服务端下发无主体名的隐藏文件 ".pdf"，现实中不可达（现核 dot>0 仍在）；`dot>=0` 一行可封，随下次触碰顺手修 |
| 11 | files.ts unlink 时机+清理错误文案坍缩 | accept-with-reason | 清理失败已置为业务失败对外可见（finally 块注释明示承诺），路径信息保留在 cause；真机查看器按需读取风险并入已阻塞的 T33/#172 真机验收门一并复核 |

**裁定统计：** fixed-by-r2 2 / accept-with-reason 8 / needs-followup 1（#8 闭包种子护栏，建议随下一代码轮立票）。核销轮未改任何生产码。

**Goal:** Resolve verified career workflow and privacy findings from independent review of `db234c5eb171f2dde7427d382b55b503a038f879..e7edfa72728c5d44940d9f145a0b5489089f4692`.

**Architecture:** Keep Career Office as the source of career facts and use existing scope/revision/idempotency contracts. Make deletion a complete barrier across stored files, Career rows, Workbench projections and receipts. Preserve unknown write identities until a receipt or same-request replay resolves them.

**Tech Stack:** Go, GORM, React/TypeScript, Taro/TypeScript, SQLite/PostgreSQL tests.

**Spec:** `docs/specs/2026-09-23-weknora-job-search-design.md`; `docs/adr/0015-*` through `docs/adr/0018-*`; `CONTEXT.md`; `docs/plans/issue-140/2026-09-24-issue-140-dag.md`, especially T07, T09–T13, T17, T22, T26, T29 and #162–#170. Review evidence is recorded in `docs/plans/issue-140/final-review-addendum-2026-09-29.md` and reviewer messages for shards 1, 3, 4, 9–11.

## Global Constraints

- Keep original issue30-sweep BASE `db234c5eb171f2dde7427d382b55b503a038f879` as ancestor. The original R2 execution checkpoint was `e7edfa72728c5d44940d9f145a0b5489089f4692`; this repair wave began at `4cac8ac0e5cbd196e414737f871d025eaf4a20e7`; Tasks 11–15 and new review repairs branch from integration checkpoint `49914ca3b27ff5162c4afcdc8eaef200f9df3cf8`. Each task report must record its exact BASE and HEAD.
- Keep authenticated user, tenant, owner, expected-revision, request-ID and scope-generation checks intact.
- A user-confirmed submission must carry actual channel/time/material version or an explicit unknown marker.
- Do not turn malformed or incomplete source/evaluation data into successful empty results.
- Preserve manual Career entry for candidates without resumes and show the full shared JD that will be submitted.
- Local commits are authorized; no push, merge, publish, deployment, or GitHub issue mutation.
- User confirmed on 2026-09-30 that no database has applied the #140 Career/Workbench migrations, but is unsure whether older databases recorded #30 versions and explicitly instructed us to preserve existing migration numbers. Therefore keep #30 at SQLite 112–123 and versioned/PostgreSQL 191–202; keep #140 at SQLite 124–142 and versioned/PostgreSQL 203–221. Do not integrate migration re-numbering commit `65ada6a5388a27a681fdf991ed33f726e1c62e33`. Task13 appends SQLite 143 / versioned 222; Task21 appends SQLite 144 / versioned 223; any further #140 schema changes append after both at SQLite 145 / versioned 224. Verify pairings/unique sequence and record that no existing IDs were changed.

## Review Focus

- Career deletion racing with export publish, source upload, application creation and delayed Workbench linking leaves no file, row, task, run, or locator after a `deleted` receipt.
- A revoked export still has physical data until Career deletion removes it; export must preserve every promised recovery draft and fail closed when counts cannot be read.
- Pausing/editing a rule before a scheduled trigger prevents the stale query from starting a paid search.
- Browser reload, tab close, app restart, and space switch never discard an unknown request ID or expose another identity's pending write.
- Generic progress cannot record a submitted stage without submission facts; invalid service receipts never render as completed empty source checks.

## Task 1: Make Career deletion complete across stored artifacts and Workbench projections

**Dependency:** None. Backend deletion owns a separate worktree; integrate before final deletion verification.

**Role:** `backend_implementer`; validator `backend_validator`; reviewer `reviewer`.

**Files:** `internal/modules/career/career_export.go`, `internal/modules/career/rendering.go`, `internal/modules/career/application.go`, `internal/modules/career/career_export_test.go`, `internal/modules/career/rendering_test.go`, `internal/modules/workbench/service/workbench/application_task_removal.go`, relevant Workbench tests, and a narrow container/wiring adapter only if required by the chosen lifecycle seam.

**Consumes / produces:** Existing `CareerOffice.DeleteCareer(ctx, request) (CareerDeletionReceipt, error)`, export object-store `Delete(ctx, key) error`, `EnsureCareerApplicationTask` linker, and the deletion receipt’s `deleted` terminal state. Define one explicit deletion fence/coordination seam before coding; all application/task linking and export publication must either finish before the final deletion sweep or observe the fenced/deleted profile and refuse to create new private projections.

**Steps:**

- [x] Add regressions for revoke-export then delete (both PDF and DOCX bytes absent), publish paused across deletion finalization, and application commit paused before Workbench linking while deletion runs.
- [x] Run the three regressions and record the current leak/race failures.
- [x] Implement the smallest profile-owned deletion fence or equivalent serialized lifecycle contract; enumerate all export rows regardless of `revoked` status; remove late-created objects before removing their locators; repeat/serialize Workbench projection removal before writing the terminal receipt.
- [x] Ensure failed physical deletion or a still-running writer prevents `deleted` from being returned; retain retryability/idempotency for cleanup.
- [x] Run `go test -count=1 ./internal/modules/career ./internal/modules/workbench/service/workbench ./internal/container` and `git diff --check`; expected: all deletion races leave no files/rows and a retry completes safely.
- [x] Commit only owned paths and report BASE/HEAD plus race evidence.

**Acceptance:** After a successful deletion receipt, no Career export bytes (including revoked versions), Career rows, Workbench sessions/runs/application mappings, or reachable receipts remain for the captured owner scope; concurrent writes either complete before cleanup or fail without creating orphaned data.

## Task 2: Preserve all promised deletion-boundary data and fail on count errors

**Dependency:** None; files overlap Task 1’s export module. Execute serially after Task 1 integration, or merge both under the same backend task if an isolated seam is impossible.

**Role:** `backend_implementer`; validator `backend_validator`; reviewer `reviewer`.

**Files:** `internal/modules/career/career_export.go`, `internal/modules/career/career_export_test.go`.

**Consumes / produces:** `CareerDeletionBoundary(ctx) (CareerDeletionBoundaryView, error)` and the deletion archive’s existing `CareerExportPreparation` item. Preserve the `PreparationReceipt` body, sources, and submitted-version anchor currently stored in `preparationRecord.ReceiptBody`.

**Steps:**

- [x] Change archive fixture to include a non-empty preparation body, sources and version anchor; assert the exported archive preserves them.
- [x] Add an injected count-query error test and assert the boundary returns the error instead of a successful zero count.
- [x] Run tests RED, propagate query errors, and serialize the complete preparation receipt into the archive using the existing decoder/validation contract.
- [x] Run `go test -count=1 ./internal/modules/career -run 'TestExportCareerArchiveCarriesPreparationsSearchRulesAndReminders|TestCareerDeletionBoundary'` and `git diff --check`; expected: payload retained and DB error fails closed.
- [x] Commit only owned paths and report evidence.

**Acceptance:** Export-before-delete preserves every preparation fact promised by the boundary; unavailable counts are reported as unavailable/error, never as absence.

## Task 3a: Revalidate scheduled rules and expose the scoped rule-list contract

**Dependency:** None. The HTTP contract below is frozen before parallel Web work.

**Role:** `backend_implementer`; validator `backend_validator`; reviewer `reviewer`.

**Files:** `internal/modules/career/search_rule.go`, `handler.go`, focused handler/service tests, and `internal/router/routes_career.go` only if explicit route registration is required.

**Consumes / produces:** Add authenticated paginated `GET /api/v1/career/rules?cursor=<opaque>` returning `{ "rules": RuleSummary[], "nextCursor": string | null }`; each bounded page contains at most 50 summaries. Each summary has `ruleId`, `query`, `intervalMinutes`, `status`, `revision`, `nextDueAt` (timestamp for enabled; `null` for paused/disabled), `estimate`, `createdAt`, and `updatedAt`; empty scope returns `{ "rules": [], "nextCursor": null }`; sort `updated_at DESC, id ASC`. Do not load per-rule run/todo history to build summaries. Keep the existing multiple-rule policy. Before quota admission and starting an external search, establish a durable run claim transaction that locks/revalidates the current rule ID, enabled status, revision, and query and persists the period as started. The claim commit is the linearization point: a pause/edit committed first prevents external search; a pause/edit committed after the claim is an already-started operation and may finish. A process crash after the claim must recover the same deterministic request ID through SearchOnce and must not strand a started run.

**Steps:**

- [x] Add failing service tests where a pause/edit commits after due-row collection but before quota/search; assert no new charged search begins. Add list contract tests for scope isolation, empty response shape, sort order, exact summary fields, and authentication.
- [x] Run RED tests and capture the stale-dispatch failure.
- [x] Implement the pre-charge revalidation and list signature/handler; register the route and add contract tests without changing quota or multi-rule policy.
- [x] Run `go test -count=1 ./internal/modules/career -run 'Rule|TriggerDue'` plus the route/handler contract test and `git diff --check`; report which checks started external I/O.
- [x] Commit only owned backend files and report exact behavior.

**Acceptance:** A committed pause/edit prevents any later scheduled search from starting; the scoped list returns bounded summaries with the frozen response contract.

## Task 3b: Recover Web search rules after browser storage loss

**Dependency:** Interface-only dependency on frozen Task 3a contract; implementation may run in parallel with Task 3a from the same BASE. Integrate and verify after both task reviews.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `packages/api-client/src/career.ts` and focused contract tests; `apps/web/src/career/RulePage.tsx` and focused tests.

**Consumes / produces:** Strictly decode each `GET /api/v1/career/rules?cursor=<opaque>` page as `{ rules: RuleSummary[], nextCursor: string | null }`; accept `nextDueAt: null` only for non-enabled rules, and require a valid timestamp for enabled rules. Follow cursors with repeated-cursor protection until all summaries are loaded. Resolve localStorage's rule ID only if present in the scoped server list; auto-select the sole rule, require explicit selection for multiple rules, and block create on list failure. Capture user/tenant scope and persist unresolved set-rule `{requestId, ruleId?, query, intervalMinutes, status, expectedRevision}` before sending; after remount reconcile the same receipt or replay the exact request before enabling another create. Keep outgoing scope state isolated on switch.

**Steps:**

- [x] Add failing tests for empty localStorage with an existing rule, multiple-rule selection, malformed/failed list decode, and reload after unknown create proving same request ID/revision and no duplicate create.
- [x] Run RED tests and record existing duplicate/create-on-storage-loss behavior.
- [x] Implement strict list decoding and server-first rule discovery/selection plus scope-keyed unknown-write persistence and receipt recovery.
- [x] Run API client contract tests, RulePage tests, Web typecheck and `git diff --check`; record executable test results.
- [x] Commit only the API client and RulePage files.

**Acceptance:** Storage loss cannot hide an existing server rule or create a duplicate while an earlier write is unresolved; all rule access remains user/tenant scoped.

## Task 11: Make due-rule claiming atomic with the pause/edit decision and bound list reads

**Dependency:** Repair to Task 3a Review finding `RULE-R2-BE-01`; use the exact persisted claim invariant below before code changes.

**Role:** `backend_implementer`; validator `backend_validator`; reviewer `reviewer`.

**Files:** `internal/modules/career/search_rule.go`, its database models/schema migration files and search-rule tests. Migration numbering must use the integration's finally ruled migration sequence.

**Consumes / produces:** In one short database transaction, lock the current rule row, require the scanned ID, enabled status, revision and query to match, then persist a recoverable period claim before any external call. The claim commit is the operation's start point. After the claim, `SearchOnce` keeps its deterministic `rule:<id>:<period>` request ID; retries recover/replay that same operation after a crash. Pause/edit transactions serialize against the same rule row. Paginate summaries with a fixed maximum of 50 rows, an opaque owner-scoped cursor over `(updated_at DESC, id ASC)`, and `{rules,nextCursor}` response; never load run/todo histories. `nextDueAt` is nullable for paused/disabled rules and a valid time for enabled rules.

**Steps:**

- [x] Add a two-Office/shared-DB race test that blocks after candidate scan, commits pause/edit through another Office, then proves no claim/search starts. Add a second test where claim commits first and pause follows, proving the same request ID completes/reconciles as an already-started run. Add pagination tests for same-timestamp IDs, empty end page, malformed cursor, and >50 rules.
- [x] Run regressions RED and record which race is currently accepted.
- [x] Implement transactionally serialized claims and restart reconciliation without time-only claim takeover; do not hold a DB transaction over quota/network I/O. Add bounded cursor pagination and nullable `nextDueAt` schema/contract behavior.
- [x] Run focused Career Go tests and the SQLite migration/up-down tests; run Postgres migration integration if configured, plus `git diff --check`.
- [x] Commit only owned backend/schema paths and report exact linearization/recovery semantics.

**Acceptance:** A pause/edit that commits before a due-period claim prevents the external search; a committed claim resolves under the same deterministic request ID after restart; each list page is bounded and stable.

## Task 12: Keep RulePage locked through list, detail, and unknown-write recovery

**Dependency:** Task 11 response contract is fixed above; this frontend repair may run concurrently only after Task 11's interface is frozen, and integration must verify both together.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `packages/api-client/src/career.ts` and career API client tests; `apps/web/src/career/RulePage.tsx` and focused RulePage tests.

**Consumes / produces:** Match Task 11 cursor page and nullable schedule contract. Keep `viewPhase='loading'` until the rule list, selected detail, and any stored pending request have reached a known outcome. A found receipt is accepted; a missing receipt replays the exact attempt; any unresolved result keeps create/edit disabled. Add a request generation token for manual rule selection so an older `getRule` response cannot replace the current selection. Never create a new rule while either list/detail discovery or pending-write recovery is in flight or failed.

**Steps:**

- [x] Add failing tests for paused/disabled `nextDueAt:null`, paged lists, pending create during slow receipt lookup, failed detail read, and out-of-order selection responses.
- [x] Run tests RED; verify a new create is currently possible during each incomplete state.
- [x] Implement strict cursor and nullable-field decoding; keep the form unavailable until initialization/recovery completes; fence detail responses by scope and selection generation.
- [x] Run API-client tests, RulePage tests, Web typecheck, and `git diff --check`.
- [x] Commit only API-client and RulePage paths; report race timelines and cursor termination evidence.

**Acceptance:** No second rule write can start before existing rule state and unresolved requests are known; delayed reads cannot overwrite the user's latest selection.

## Task 13: Coordinate Career deletion with operations across handlers and replicas

**Dependency:** Repair of Task 1's same-Office mutex; must integrate before claiming deletion races are closed.

**Role:** `backend_implementer`; validator `backend_validator`; reviewer `reviewer`.

**Files:** Career persistence/model/migration and repository seams, `internal/modules/career` operation admission and deletion paths, focused Career tests, and narrow Workbench/storage adapters only if fencing tokens are required.

**Consumes / produces:** The scope `(tenant_id, owner_user_id)`, existing request IDs for Career operations, `career_spaces` retained through deletion, Workbench `EnsureCareerApplicationTask`, export storage writes/removals, and due-rule dispatch claims. Use a durable per-scope lifecycle gate shared by independently constructed `Office` handlers and processes. Application linking, material publication and rule-period claims must be admitted by the gate before external Workbench/storage/search effects; rule period persistence and its lifecycle claim must commit atomically so deletion cannot slip between them. Keep database transactions short; never hold one across rendering, object storage, Workbench or search network calls.

**Steps:**

- [x] Add deterministic two-Office tests with a shared database/storage/linker/search seam: pause an application linker, material writer and rule search after admission, start deletion through a second Office, and assert deletion cannot return terminal `deleted` while any admitted effect is unresolved; also assert new work is rejected after deletion enters `deleting`.
- [x] Run the regressions and record the current cross-instance leak/order failure.
- [x] Add a persistent scope gate and operation claims. Admit each external effect before its first side effect; retain claims until the operation outcome or compensation is durably known. Integrate rule-period claim creation and lifecycle admission in the same transaction. Deletion transitions active→deleting only after claims are reconciled, retains deleting through cleanup and the terminal receipt, and rejects new claims. Do not expire claims by elapsed time alone; expose retry/recovery by original request ID.
- [x] Ensure SQLite and PostgreSQL transitions serialize on the same durable row with conditional updates/row locks, and that failure or process restart leaves a retryable, non-terminal state.
- [x] Run focused race and recovery tests, relevant Career/Workbench/container suites, migration tests, and `git diff --check`; expected: two Offices observe one ordering and no effect can appear after a successful deletion receipt.
- [x] Commit owned paths and report schema IDs, claim recovery behavior, and exact test evidence.

**Acceptance:** Across separately constructed handlers/processes, every application link, material object effect and paid rule-period dispatch is admitted by the shared gate; deletion cannot finalize while an earlier claim is unresolved and no later operation is admitted after deletion begins.

## Task 14: Correctly report saved preparation edits when follow-up reads fail

**Dependency:** None; scoped to the Career preparation revision UI and its behavior tests.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/miniprogram/src/career/progress-preparation.tsx` and its focused tests only.

**Consumes / produces:** Existing `career.editMaterial` durable receipt and `career.material` read. A successful edit is a committed fact even if the subsequent detail refresh fails; refresh failure must never re-enter the write-failure path or tell the user the edit was not submitted. The preflight material read used to preserve claims remains before submission and its failure must retain the explicit local draft and present typed recovery guidance.

**Steps:**

- [x] Add regressions for (a) preflight claim read failure, and (b) edit success followed by material-detail read failure; assert only (a) says the edit remains unsubmitted and both retain enough recovery state.
- [x] Run tests RED and capture the contradictory saved/not-saved message on (b).
- [x] Move the preflight read into the handled local-draft error path; isolate post-commit detail refresh from the edit failure catch and keep the committed receipt/success state visible.
- [x] Run focused preparation tests, Mini Program typecheck where executable, and `git diff --check`; report existing unrelated type errors separately.
- [x] Commit only the page and its focused tests.

**Acceptance:** Once `editMaterial` resolves successfully, no later read error can tell the user that the edit was not submitted; a preflight failure remains an explicit non-submission with recoverable local draft.

## Task 15: Persist push subscription intent after native authorization is accepted

**Dependency:** None; owns the reminder subscription UI and its behavior tests.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/miniprogram/src/career/rules-usage-reminders.tsx` and focused page/adapter tests.

**Consumes / produces:** Existing `requestReminderSubscription()` native result, `setPushSubscription(value, expectedRevision)`, and Career revision. Native authorization is not delivery evidence. On an accepted authorization, persist `subscribed` under the current revision; on denial/unavailability leave server preference unchanged. If revision is unavailable or persistence outcome is unknown, state that authorization succeeded but server preference was not confirmed and retain same-request recovery semantics rather than claiming subscription complete.

**Steps:**

- [x] Add tests for accepted authorization followed by successful preference write, accepted authorization with missing revision, rejected/unavailable results leaving preference untouched, and unknown preference-write recovery without a second native request.
- [x] Run tests RED.
- [x] Persist the subscription fact after accepted native authorization; display separate authorization and server-preference states and do not claim message delivery.
- [x] Run focused rules/reminder tests, Mini Program typecheck where executable, and `git diff --check`; report existing unrelated type errors separately.
- [x] Commit only owned reminder UI/test files.

**Acceptance:** Users who opt back in update the shared Career push preference; permission is never represented as delivery, and ambiguous preference writes remain recoverable.

## Task 16: Keep push authorization and saved-edit facts bound to their original scope/outcome

**Dependency:** Follow-up to independent Mini Program review findings M1–M4 on commits `fd1d1b63612835d61ccc92eb7802c25bd719cdcc` and `413c526016d4f80c46bb0e8944ffea00ca7549b6`.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/miniprogram/src/career/rules-usage-reminders.tsx`, its focused tests, `apps/miniprogram/src/career/progress-preparation.tsx`, its focused tests, and only existing shared decoder/helper files if the standard `decodeAs` or scoped recovery API must be imported.

**Consumes / produces:** Capture the authenticated scope before opening the native subscription prompt, bind the authorization marker and preference-write intent to that captured scope, and recheck it after native and network awaits before any storage or server effect. Explicit opt-out must invalidate accepted-but-unsynced authorization; resubscribe requires a fresh native result. Malformed 200 receipts must be classified as a contract violation via the existing decoder wrapper, with no infinite same-request retry loop. A successful `editMaterial` remains committed even if either later local draft cleanup or detail readback fails; report cleanup/readback as post-commit follow-up state, never as write failure.

**Steps:**

- [x] Add behavior tests for account A→B while native prompt is pending (no B marker/write), accepted authorization→missing revision→opt-out→resubscribe (fresh prompt required), malformed success receipt (contract violation is not retryable unknown), and local draft removal failure after successful edit (committed state remains visible).
- [x] Run RED against current implementation and save exact reproductions.
- [x] Bind all pending push state and request IDs to the captured scope; revalidate after every await; clear unsynced authorization on confirmed opt-out; use `decodeAs` for write and receipt payloads. Split the `editMaterial` failure catch from all post-commit cleanup/readback effects and report each independently.
- [x] Run focused rule/reminder and preparation tests, Mini Program typecheck where executable, and `git diff --check`; report unrelated account-page type errors separately.
- [x] Commit only the owned Mini Program files and report exact scope-switch, consent, malformed-receipt, and post-commit-cleanup evidence.

**Acceptance:** Authorization cannot cross account scope or survive explicit opt-out as an implicit new consent; malformed receipts stop safely; once an edit receipt is committed, cleanup/readback failure cannot label the edit unsubmitted.

## Task 19: Keep malformed push receipts behind the profile-reconciliation boundary

**Dependency:** Task 16 commit `7112cf0c7df60a42ef76d958f8c9778bcdebd4cf`; review findings T16-1 and T16-2 in `/tmp/issue140-r2-task16-review.md`.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/miniprogram/src/career/rules-usage-reminders.tsx` and its focused tests; only the existing career platform decoder adapter if required for a verified profile read.

**Consumes / produces:** Reuse the scope-bound `push-receipt-invalid` guard and existing verified profile refresh. Enforce the guard inside every preference persistence entry point, including the sync action, until the user refreshes and inspects the server preference. Invalidate accepted-but-unsynced authorization before starting opt-out network work; a malformed 200 remains an uncertain server fact but cannot preserve authorization that would skip a fresh native prompt. Preserve request IDs and scope checks across awaits.

**Steps:**

- [x] Add tests for malformed subscribed receipt followed by the visible sync action; assert no second POST until a verified profile refresh clears the guard.
- [x] Add the combined-state test: a subscribed intent is pending, opt-out receives a malformed success receipt, and the retry action is attempted; assert the retry seam and control cannot send while profile reconciliation is required.
- [x] Add an opt-out action test where the server may have committed but its 200 receipt is malformed; assert the old accepted marker is already invalidated and a later subscribe requires a fresh native prompt.
- [x] Run the tests RED against Task 16, then enforce the invalid-receipt lock in every shared persistence/retry seam and disable all retry/sync controls while reconciliation is required; invalidate authorization at opt-out intent time while retaining uncertainty messaging.
- [x] Run focused reminder tests, Mini Program typecheck where executable, and `git diff --check`; separate existing account-page type errors from task diagnostics.
- [x] Commit only owned Mini Program files and report the exact recovery timeline and tests.

**Acceptance:** No subscribed write can be retried through a secondary UI entry while the receipt is invalid; profile reconciliation is required. An opt-out attempt cannot leave stale consent that suppresses the next native authorization prompt, including malformed-success outcomes.

## Task 17: Preserve rule edits across active runs and serialize dispatch safely with deletion

**Dependency:** Task 11 implementation and Task 13 lifecycle gate contract. Use Task 13's transaction-scoped lifecycle-admission seam; do not integrate before Task 13's appended gate migration and API are verified.

**Role:** `backend_implementer`; validator `backend_validator`; reviewer `reviewer`.

**Files:** `internal/modules/career/search_rule.go` and tests, lifecycle-gate integration at the rule claim transaction, `internal/database/career_migration_test.go`, and paired page-index migration files at SQLite 145 / versioned 224 (Task 13 owns SQLite 143 / versioned 222; Task 21 owns SQLite 144 / versioned 223). Do not renumber existing migration IDs.

**Consumes / produces:** Preserve the durable `started` run and same-request recovery from Task 11. Advance `last_period` and the next scheduled instant atomically with the rule-period claim; terminalization must write only run/todo results and may not change a later edit's `next_due_at`, revision or `updated_at`. Acquire rows in the same profile→rule lock order as `SetRule`; SQLite must establish a writer/CAS point on `(scope,rule,revision,status,last_period,next_due_at)` and fail/re-read on busy/stale outcomes. Create the lifecycle claim in that same transaction, and keep it until durable run terminalization; deletion then cannot race a newly committed due search. Bound list reads to 50 summaries, select only summary columns, version the owner-bound cursor, and add index `(tenant_id,user_id,updated_at DESC,id ASC)` through migration 145/224.

**Steps:**

- [x] Add a controlled claim→enabled edit→run completion test; assert exact edited schedule/revision/updatedAt survive. Add lock-order/concurrent pause-vs-claim test and deletion-vs-rule-claim test using two Office instances/shared DB. Add query-plan or schema assertion for the keyset index and reject unknown cursor versions.
- [x] Preserve existing production usage evidence: `usageReservationRecord` is unique by `(tenant,user,requestId)`, `admitSearchUsage` returns success for an existing reserved/settled request, and `TestDuplicateRequestDoesNotDoubleReserveOrCharge` asserts two admissions for one request retain one unit. Finding F3 from `/tmp/issue140-r2-task11-review.md` is ruled out for the production gate by this evidence; no new gate architecture is needed for it.
- [x] Run tests RED, then move period/schedule advancement into the claim transaction; ensure both PostgreSQL lock order and SQLite conditional write serialize with `SetRule`; use the transaction-scoped deletion-gate claim. Make terminalization update only run/todos. Add page index migration 145/224 and versioned cursor/summary-only query.
- [x] Run `go test -count=1 ./internal/modules/career`, `go test -count=1 ./internal/database` including migration 143/144/145 up/down, route contract tests and `git diff --check`; use configured PostgreSQL concurrency/migration tests if available, otherwise state limitation.
- [x] Commit only owned backend/schema files and report race timelines, lock order, lifecycle-claim ordering, index migration and quota-idempotency ruling.

**Acceptance:** A committed edit is never overwritten by a running period; pause/edit and dispatch claim share a deadlock-safe linearization order across PostgreSQL and SQLite; deletion is ordered against rule dispatch; list pagination is indexed, summary-only and cursor-versioned. Existing quota idempotency remains covered by the current ledger test.

## Task 18: Decode nullable schedule fields consistently across rule list, detail and receipts

**Dependency:** Task 17's backend serialization contract; can be implemented in a separate API-client worktree after Task 12 is integrated to avoid overlapping file ownership.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `packages/api-client/src/career.ts` and `career.test.ts`; narrowly scoped RulePage expectations only if a TypeScript nullable type requires it.

**Consumes / produces:** `RuleSummary`, `RuleView` and `SetRuleReceipt` all contain required `nextDueAt: string | null`. Enabled rules require a valid timestamp; paused/disabled rules require null. Reject missing, malformed timestamp, and null for enabled responses. Keep pagination cursors and owner scope checks unchanged.

**Steps:**

- [x] Add contract tests for enabled, paused and disabled list, detail and write receipt JSON; run RED against the current optional-string detail/receipt types.
- [x] Update client types and strict decoders for required nullable fields; preserve the valid Task 12 list behavior and fail closed on inconsistent status/time pairs.
- [x] Run Career API-client tests, RulePage tests, Web typecheck and `git diff --check`.
- [x] Commit only owned API/client test files and report exact JSON fixtures and command results.

**Acceptance:** Every rule read/write surface has the same explicit timestamp-or-null schedule field and validates its status relationship.

## Task 20: Keep RulePage writes locked across inconsistent receipts and stale reads

**Dependency:** Task 12 commit `25ef240c911388db0291d737eb2f6916e00c32a6`; independent review `/tmp/issue140-r2-task12-review.md`. Task 20 may proceed against Task 11's current paginated list contract; Task 18 later updates nullable detail/write-receipt decoding without changing these state transitions.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/web/src/career/RulePage.tsx` and its focused tests. Do not modify the API client or backend in this task.

**Consumes / produces:** Preserve the current durable attempt key, selected rule ID, and selection-generation seam. A listed rule with missing detail stays unresolved and cannot enable create. A write receipt whose request ID differs from the persisted attempt stays in unknown/reconciliation state without overwriting the original ID. Any detail refresh may update `ruleView` only while both captured selected ID and selection generation remain current.

**Steps:**

- [x] Add a listed-rule `not_found` detail test with another rule present; assert no create/write is enabled until rediscovery resolves the inconsistency.
- [x] Add mismatched write-receipt test; assert the original durable ID and unknown lock survive a second submit attempt and remount/recovery.
- [x] Add save-A → select-B → late refresh-A test; assert B remains selected and is the only rule ID on the next save.
- [x] Run tests RED, then make the smallest state-machine changes to keep unresolved writes locked and fence every read by selection generation and selected ID.
- [x] Run focused RulePage tests, Web typecheck and `git diff --check`; commit only owned RulePage files and report each tested interleaving.

**Acceptance:** No list/detail inconsistency or mismatched receipt can unlock a new rule write, and stale post-save reads cannot change the currently selected edit target.

## Task 21: Keep deletion blocked until every admitted upload and retry effect is known

**Dependency:** Task 13 commit `76809f586e38e9860cbd33dbecc9751916208ffb`; independent review findings T13-1 through T13-3 in `/tmp/issue140-r2-task13-review.md`. Do not integrate Task 13 or start Task 17 until this repair is validated and reviewed.

**Role:** `backend_implementer`; validator `backend_validator`; reviewer `reviewer`.

**Files:** Career lifecycle gate and export/rendering/application/upload/profile-intake modules and their focused tests; file-service storage interfaces and local/S3/MinIO implementations only as required to provide stable caller-chosen keys; migration files only if a new durable owner/attempt field is needed, append after SQLite143/versioned222 without renumbering.

**Consumes / produces:** Lifecycle admission must distinguish the owning external-effect attempt from duplicate retries; only the owner may release after a durable result or known compensation. Export writes must use a deterministic, persisted storage key that local, S3 and MinIO adapters honor across retries, or remain claimed until all keys are discoverable and compensated. Resume upload must acquire the same scope gate before its first file/catalog side effect and retain the claim until the resource reference or compensated failure is durable. Deletion waits for all such claims and continues to fail closed when physical cleanup is unavailable.

**Steps:**

- [x] Add production-shaped storage test whose adapter allocates a fresh physical key on each call and loses the first response; assert retry reuses the same key or the first key remains discoverable and is deleted before terminal deletion.
- [x] Add a two-Office exact-same-request interleaving: attempt A owns and pauses before effect, attempt B retries, deletion races; assert B cannot independently release A's ownership and deletion cannot finalize until all effects resolve.
- [x] Add a two-Office resume-upload/deletion interleaving paused before `SaveBytes` or catalog binding; assert deletion waits or upload is rejected before any external write.
- [x] Reproduce all three findings against Task 13; introduce owner-token/attempt state, stable physical object-key contract, and upload lifecycle admission with crash/replay recovery.
- [x] Append migration IDs only when required (next pair SQLite144/versioned223); test up/down and uniqueness while preserving 112–143 / 191–222.
- [x] Run focused lifecycle/export/upload tests, `go test -count=1 ./internal/modules/career ./internal/modules/workbench/service/workbench ./internal/container ./internal/database`, race tests for the cross-Office interleavings, and `git diff --check`; run configured Postgres tests if available, otherwise record the limitation.
- [x] Commit owned backend/storage changes and report the three race timelines, object-key guarantees, migration IDs and exact verification evidence.

**Acceptance:** A `deleted` receipt is issued only after every application/material/upload external effect admitted before deletion is terminal and all discoverable private object keys are removed. Duplicate retries cannot release another attempt's claim, and storage retries cannot orphan an untracked prior object.

## Task 22: Project each recorded submission into the authoritative progress timeline

**Dependency:** Task 4 commit `04f08470cdd2862268abf911b04715c3685ab79f`; review finding T4-1 in `/tmp/issue140-r2-task4-review.md`; Task21/26/30/32 lifecycle gate is verified and integrated. Preserve the generic-submission guard. Task17 is assigned SQLite145/versioned224; if this task requires a new persisted link column, reserve SQLite146/versioned225 and use a dedicated submission migration test file so Task17's `career_migration_test.go` remains disjoint.

**Role:** `backend_implementer`; validator `backend_validator`; reviewer `reviewer`.

**Files:** `internal/modules/career/submission.go`, `progress.go`, their tests, and a Career migration pair only if a persisted link is required. Keep frontend files out of this task.

**Consumes / produces:** `RecordSubmission` remains the only write path for actual submission confirmation and continues to bind channel, actual time, and exact material export/version/digest or explicit unknown version. In the same database transaction, persist or derive exactly one traceable `submitted` progress event linked to the submission ID. Idempotent request replay returns the original submission and does not duplicate the timeline event. Corrections remain append-only and must preserve the original submission linkage. `ApplicationProgress` projects the submitted stage from this authoritative event in deterministic order.

**Steps:**

- [x] Add RED tests that record both a known-export and explicit-unknown submission, read `ApplicationProgress`, and assert one linked submitted event, correct stage, channel/time/version facts and stable replay without duplicate events.
- [x] Test transaction rollback so a failed progress-event write leaves neither a submission nor a timeline event.
- [x] Implement an atomic link or deterministic projection from the submission record; reject generic submitted/resubmitted events as Task4 already requires.
- [x] If a `submission_id` link column is necessary, append SQLite146/versioned225 after Task17's 145/224, add up/down and uniqueness coverage in the task-owned submission migration test file, and do not modify `career_migration_test.go`.
- [x] Run focused submission/progress tests, Career package and database migrations, and `git diff --check`; run PostgreSQL transactional test when configured, otherwise report the limitation.
- [x] Commit only backend event/link/test/schema files and document replay, ordering and correction semantics.

**Acceptance:** A confirmed actual submission becomes visible in `ApplicationProgress` and advances the projected stage exactly once; its channel/time/material version remains traceable, including the explicit unknown-version state.

## Task 23: Carry the application's material identity into submission confirmation

**Dependency:** Task4 commit `04f08470cdd2862268abf911b04715c3685ab79f`; review finding T4-2 in `/tmp/issue140-r2-task4-review.md`. This UI entry fix is independent of Task22's backend timeline projection.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/web/src/career/ApplicationPage.tsx`, `apps/web/src/career/ProgressPage.tsx`, and focused tests for the progress entry. Reuse the existing `SubmissionPage` contract; do not modify its export eligibility behavior.

**Consumes / produces:** `ApplicationPage` already holds the loaded `careerMaterialId`; pass that current value as a prop to `ProgressPage`, then into `SubmissionPage`. Keep export eligibility, revoked-version and explicit-unknown handling in `SubmissionPage`; do not synthesize a material ID or latest version. If the material is not yet loaded, preserve an honest loading/unavailable state and do not silently fall back to a different version.

**Steps:**

- [x] Add a test for the ApplicationPage→ProgressPage→SubmissionPage path where the application has a known submittable export; assert the already loaded material ID reaches the page and its exact export ID/version is submitted.
- [x] Add a case for absent material/export that still offers only an explicit unknown version or a clear unavailable state.
- [x] Run tests RED, pass the real application material ID through the entry seam, and preserve the existing scope/revision behavior.
- [x] Run focused ProgressPage/SubmissionPage tests, Web typecheck and `git diff --check`; commit only owned Web file/test changes.

**Acceptance:** A user confirming from the progress timeline can bind the actual exported material version when known, while unknown or unavailable versions remain explicit and honest.

## Task 24: Keep direct search receipts bound to the persisted query

**Dependency:** Task 6 commit `c2ab52569be088c606dfb050f1679667a9a12441`; review finding SEARCH-R2-1 in `/tmp/issue140-r2-task6-review.md`.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/web/src/career/SearchPage.tsx` and focused SearchPage tests only.

**Consumes / produces:** Reuse the persisted `{requestId, query, expectedRevision}` identity. A direct `searchOnce` result is accepted only when both `requestId` and normalized submitted query match the persisted attempt. A same-ID/different-query response must preserve the original pending attempt as unknown, keep new searches locked, and reconcile only through the original request receipt.

**Steps:**

- [x] Add a RED direct-response test with matching request ID and mismatched query; assert result is not shown, pending identity remains, and a second charged search is blocked.
- [x] Add remount coverage proving the original query/request ID is looked up and cannot be overwritten by the mismatched response.
- [x] Compare both request ID and query in the direct completion path, retain unknown state on mismatch, and keep safe receipt recovery available.
- [x] Run focused SearchPage tests, Web typecheck and `git diff --check`; commit only SearchPage source/tests and report both interleavings.

**Acceptance:** No charged-search result can clear a durable attempt unless request ID and query identify the same submitted operation.

## Task 25: Exercise material ID propagation from ApplicationPage

**Dependency:** Task23 commit `9fc83ceec3216c9e692f63d90bebda272bf48911`; low finding T23-1 in `/tmp/issue140-r2-task23-review.md`.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/web/src/career/ApplicationPage.test.tsx` and only the narrow ApplicationPage/ProgressPage test seam if required.

**Consumes / produces:** Keep Task23 production code unchanged unless the integration test demonstrates a real wiring defect. Mount `ApplicationPage` with its material loader resolving the known current material ID, open the progress submission confirmation, and assert that the exact material ID reaches export lookup and the selected export is sent.

**Steps:**

- [x] Add an ApplicationPage-level regression for the full owner→child material ID prop handoff using the existing MaterialPage/application seam.
- [x] Run RED if the integration currently drops the ID; otherwise document that the behavioral path is already correct and the test closes the coverage gap.
- [x] Run focused ApplicationPage/ProgressPage/SubmissionPage tests, Web typecheck and `git diff --check`; commit only test changes unless RED proves a production wiring correction is needed.

**Acceptance:** The test exercises actual parent state propagation and proves a known export remains selectable and bound through the user-facing confirmation path.

## Task 4: Bind submitted progress to an actual submission record

**Dependency:** None; backend file ownership is disjoint from Tasks 1–3.

**Role:** `implementer`; validator `backend_validator` and focused frontend validator; reviewer `reviewer`.

**Files:** `internal/modules/career/progress.go`, `internal/modules/career/progress_test.go`, `apps/web/src/career/ProgressPage.tsx`, its tests, and only the API type file if a dedicated record input needs typing.

**Consumes / produces:** Dedicated `RecordSubmission` contract already binds channel, actual submission time and material version/unknown marker. Generic `AppendProgressInput` accepts an event type and note only.

**Steps:**

- [x] Add server test rejecting generic `submitted` and `resubmitted` events without a bound submission; add UI test that the generic timeline omits those event choices and submission confirmation remains available through the dedicated flow.
- [x] Run RED tests.
- [x] Reject unbound event types at the service boundary and route Web users to the dedicated submission confirmation flow; keep timeline projection unchanged for legacy already-bound events.
- [x] Run focused career progress/submission Go tests and ProgressPage tests; `git diff --check` must pass.
- [x] Commit and report.

**Acceptance:** No UI or API path can claim a submission without channel/time/material binding or an explicit unknown-version marker.

## Task 5: Complete Web profile entry, purge private UI state after deletion, and freeze subscription retry payloads

**Dependency:** None; owns only `CareerPage.tsx` and `InboxPage.tsx` plus their tests.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/web/src/career/CareerPage.tsx`, related CareerPage tests, `apps/web/src/career/InboxPage.tsx`, related InboxPage tests.

**Consumes / produces:** Existing Career proposal/confirm actions and captured `ScopeController`. Extend the existing profile field choices to projects, internships and skills. Extend `SubscriptionAttempt` to include the first request’s `expectedRevision` and reuse it exactly on retry.

**Steps:**

- [x] Add no-resume manual entry tests for projects/internships/skills; add deletion callback test with reload failure asserting profile facts and source names disappear immediately; add subscription retry test where revision advances before retry.
- [x] Run RED tests.
- [x] Add structured field choices and synchronous private-state clearing with stale response fencing; pin subscription revision in the attempt object and replay unchanged payload.
- [x] Run targeted CareerPage/InboxPage tests and `git diff --check`.
- [x] Commit owned files and report.

**Acceptance:** Candidates without a resume can enter required evidence; deletion immediately removes cached private data even when refresh fails; retries use byte-for-byte the original request semantics.

## Task 6: Make Web search writes recover after reload

**Dependency:** None; owns only `apps/web/src/career/SearchPage.tsx` and focused tests.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/web/src/career/SearchPage.tsx`, `apps/web/src/career/SearchPage.test.tsx` (or nearest existing behavior test).

**Consumes / produces:** Existing `searchOnce`, `searchReceipt`, `retryPendingSearch`, and scope-key storage helper contracts from the Web client. Persist the exact request ID, query and expected revision under the captured user/tenant scope before sending. On remount, reconcile by receipt before enabling a new charged search.

**Steps:**

- [x] Add remount-after-unknown search tests for receipt-found and receipt-not-found cases; assert no second charge/new request ID.
- [x] Run RED tests.
- [x] Persist/load scoped pending search state and preserve unknown/receipt recovery UI after reload; clear only after a confirmed receipt or scope-specific safe resolution.
- [x] Run targeted SearchPage tests and `git diff --check`.
- [x] Commit and report.

**Acceptance:** Reload cannot mint a second charged search while an earlier result is unknown; another scope cannot read or clear the pending request.

## Task 7: Make mini-program Career writes durable and source evidence truthful

**Dependency:** None; coordinate file ownership within one task because `services/career.ts` owns the relevant state.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `packages/career-core/src/desk.ts` and tests; `apps/miniprogram/src/services/career.ts`, `apps/miniprogram/src/adapters/career-platform.ts`, `apps/miniprogram/src/career/application-material.tsx`, `apps/miniprogram/src/career/discovery.tsx`, and focused tests.

**Consumes / produces:** Existing controlled scope-aware `career-intent.ts` storage/replay utilities; existing receipt endpoints for profile actions, share import, evaluations and search. Persist `{requestId, exact input, expectedRevision}` before all remote writes. Unknown results reconcile/replay with the same ID after restart. Decode a `completed` search only when coverage, sources, rows, evidence fields and timestamps pass required schema checks. Show the complete shared JD in a scrollable user confirmation view. Show hard-condition evidence, supporting facts, matches and gaps before explicit continue.

**Steps:**

- [x] Add restart-after-ambiguous-write tests for profile mutation, share import and evaluation; malformed completed search receipt tests; long shared JD full-confirmation test; evaluation evidence display tests.
- [x] Run tests RED and record which receipt endpoints exist/missing. If a receipt endpoint is absent, add the narrow scoped API endpoint and contract test before client adoption.
- [x] Persist intents and block conflicting writes until receipt resolution; reject malformed completed receipts; display full submitted JD and specific qualification evidence.
- [x] Run focused mini-program tests, `pnpm --filter @weknora/miniprogram typecheck`, and `git diff --check`; expected: restart recovery uses the original IDs. Record any unrelated pre-existing type errors separately.
- [x] Commit owned paths and report API contract changes.

**Acceptance:** Restart does not lose any unknown-write request identity; malformed data is not rendered as a legitimate empty result; the user sees exactly the JD and evidence that inform explicit confirmation.

## Task 8: Make mobile authorization smoke evidence distinguish redirects and gate the unauthenticated boundary

**Dependency:** None; owns the smoke driver and its API-client integration test.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/mobile/src/task-office-integration-smoke.ts`, `packages/api-client/src/mobile/task-office.integration.test.ts`, focused smoke test if present.

**Consumes / produces:** `serverAuthBoundary: 'rejected' | 'failed-open' | 'unreachable'` evidence field. Use `redirect: 'manual'`; classify 3xx/login redirects as failed-open. Live gate must assert unauthenticated boundary is rejected and client-side write gate is closed.

**Steps:**

- [x] Add tests for 302-to-login, 401/403 rejection and 200 unauthenticated success. Add live-gate assertion for `serverAuthBoundary === 'rejected'`.
- [x] Run RED tests.
- [x] Implement explicit response classification and fail the gate unless both server and client boundaries are closed.
- [x] Run targeted smoke tests and API client tests plus `git diff --check`.
- [x] Commit and report.

**Acceptance:** Redirects cannot be mislabeled unreachable; a backend allowing unauthenticated reads cannot produce a passing live-gate result.

## Task 9: Preserve existing #30 and #140 migration identities

**Dependency:** User clarification on 2026-09-30 supersedes the initial re-sequencing request: old #30 application history is unknown, so existing numeric identities must remain stable.

**Role:** `backend_validator`; reviewer `reviewer`.

**Files:** Existing files under `migrations/sqlite/` and `migrations/versioned/`; migration loader and migration identity tests. No existing migration file should be renamed or edited by this task.

**Consumes / produces:** Preserve the established order and identities: #30 `task_grants` through `space_connection_grants` stay at SQLite 112–123 and versioned 191–202; #140 `career_profile` through `career_reconciliations` stay at SQLite 124–142 and versioned 203–221. The SQLite marketplace no-transaction gate remains at its current version/path 114. Any new migration appends after the current tail: SQLite 143 and versioned 222. Verify unique paired sequences without changing any existing filename, numeric version, or loader special case.

**Steps:**

- [x] Verify both migration trees pair names consistently and have unique monotonic versions, with #30 and #140 remaining at their original IDs.
- [x] Run `go test -count=1 ./internal/database` and inspect the loader's SQLite special transaction gate at version 114; run `git diff --check`.
- [x] Do not commit migration renumbering; record the user ruling and compatibility reason in the execution ledger. For appended migrations, add explicit identity tests for 143/222.

**Acceptance:** Existing migration IDs and semantics are unchanged; #30 remains 112–123 / 191–202, #140 remains 124–142 / 203–221, and new schema changes append at 143 / 222.
