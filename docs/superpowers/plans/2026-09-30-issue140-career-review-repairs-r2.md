# Issue 140 Career Workflow Review Repairs R2 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement these tasks. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve verified career workflow and privacy findings from independent review of `db234c5eb171f2dde7427d382b55b503a038f879..e7edfa72728c5d44940d9f145a0b5489089f4692`.

**Architecture:** Keep Career Office as the source of career facts and use existing scope/revision/idempotency contracts. Make deletion a complete barrier across stored files, Career rows, Workbench projections and receipts. Preserve unknown write identities until a receipt or same-request replay resolves them.

**Tech Stack:** Go, GORM, React/TypeScript, Taro/TypeScript, SQLite/PostgreSQL tests.

**Spec:** `docs/specs/2026-09-23-weknora-job-search-design.md`; `docs/adr/0015-*` through `docs/adr/0018-*`; `CONTEXT.md`; `docs/plans/issue-140/2026-09-24-issue-140-dag.md`, especially T07, T09–T13, T17, T22, T26, T29 and #162–#170. Review evidence is recorded in `docs/plans/issue-140/final-review-addendum-2026-09-29.md` and reviewer messages for shards 1, 3, 4, 9–11.

## Global Constraints

- Keep original issue30-sweep BASE `db234c5eb171f2dde7427d382b55b503a038f879` as ancestor; use isolated task worktrees from integration checkpoint `e7edfa72728c5d44940d9f145a0b5489089f4692`.
- Keep authenticated user, tenant, owner, expected-revision, request-ID and scope-generation checks intact.
- A user-confirmed submission must carry actual channel/time/material version or an explicit unknown marker.
- Do not turn malformed or incomplete source/evaluation data into successful empty results.
- Preserve manual Career entry for candidates without resumes and show the full shared JD that will be submitted.
- Local commits are authorized; no push, merge, publish, deployment, or GitHub issue mutation.
- User confirmed on 2026-09-30 that no database has applied the #140 Career/Workbench migrations and authorized version reordering. Keep the 19 #140 migrations at SQLite 112–130 and versioned/PostgreSQL 191–209; move the 12 colliding #30 migrations to SQLite 131–142 and versioned/PostgreSQL 210–221. This is safe only for database histories that have not already recorded the displaced #30 versions; verify/restate this deployment assumption in the final migration ruling.

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

- [ ] Add regressions for revoke-export then delete (both PDF and DOCX bytes absent), publish paused across deletion finalization, and application commit paused before Workbench linking while deletion runs.
- [ ] Run the three regressions and record the current leak/race failures.
- [ ] Implement the smallest profile-owned deletion fence or equivalent serialized lifecycle contract; enumerate all export rows regardless of `revoked` status; remove late-created objects before removing their locators; repeat/serialize Workbench projection removal before writing the terminal receipt.
- [ ] Ensure failed physical deletion or a still-running writer prevents `deleted` from being returned; retain retryability/idempotency for cleanup.
- [ ] Run `go test -count=1 ./internal/modules/career ./internal/modules/workbench/service/workbench ./internal/container` and `git diff --check`; expected: all deletion races leave no files/rows and a retry completes safely.
- [ ] Commit only owned paths and report BASE/HEAD plus race evidence.

**Acceptance:** After a successful deletion receipt, no Career export bytes (including revoked versions), Career rows, Workbench sessions/runs/application mappings, or reachable receipts remain for the captured owner scope; concurrent writes either complete before cleanup or fail without creating orphaned data.

## Task 2: Preserve all promised deletion-boundary data and fail on count errors

**Dependency:** None; files overlap Task 1’s export module. Execute serially after Task 1 integration, or merge both under the same backend task if an isolated seam is impossible.

**Role:** `backend_implementer`; validator `backend_validator`; reviewer `reviewer`.

**Files:** `internal/modules/career/career_export.go`, `internal/modules/career/career_export_test.go`.

**Consumes / produces:** `CareerDeletionBoundary(ctx) (CareerDeletionBoundaryView, error)` and the deletion archive’s existing `CareerExportPreparation` item. Preserve the `PreparationReceipt` body, sources, and submitted-version anchor currently stored in `preparationRecord.ReceiptBody`.

**Steps:**

- [ ] Change archive fixture to include a non-empty preparation body, sources and version anchor; assert the exported archive preserves them.
- [ ] Add an injected count-query error test and assert the boundary returns the error instead of a successful zero count.
- [ ] Run tests RED, propagate query errors, and serialize the complete preparation receipt into the archive using the existing decoder/validation contract.
- [ ] Run `go test -count=1 ./internal/modules/career -run 'TestExportCareerArchiveCarriesPreparationsSearchRulesAndReminders|TestCareerDeletionBoundary'` and `git diff --check`; expected: payload retained and DB error fails closed.
- [ ] Commit only owned paths and report evidence.

**Acceptance:** Export-before-delete preserves every preparation fact promised by the boundary; unavailable counts are reported as unavailable/error, never as absence.

## Task 3a: Revalidate scheduled rules and expose the scoped rule-list contract

**Dependency:** None. The HTTP contract below is frozen before parallel Web work.

**Role:** `backend_implementer`; validator `backend_validator`; reviewer `reviewer`.

**Files:** `internal/modules/career/search_rule.go`, `handler.go`, focused handler/service tests, and `internal/router/routes_career.go` only if explicit route registration is required.

**Consumes / produces:** Add authenticated paginated `GET /api/v1/career/rules?cursor=<opaque>` returning `{ "rules": RuleSummary[], "nextCursor": string | null }`; each bounded page contains at most 50 summaries. Each summary has `ruleId`, `query`, `intervalMinutes`, `status`, `revision`, `nextDueAt` (timestamp for enabled; `null` for paused/disabled), `estimate`, `createdAt`, and `updatedAt`; empty scope returns `{ "rules": [], "nextCursor": null }`; sort `updated_at DESC, id ASC`. Do not load per-rule run/todo history to build summaries. Keep the existing multiple-rule policy. Before quota admission and starting an external search, establish a durable run claim transaction that locks/revalidates the current rule ID, enabled status, revision, and query and persists the period as started. The claim commit is the linearization point: a pause/edit committed first prevents external search; a pause/edit committed after the claim is an already-started operation and may finish. A process crash after the claim must recover the same deterministic request ID through SearchOnce and must not strand a started run.

**Steps:**

- [ ] Add failing service tests where a pause/edit commits after due-row collection but before quota/search; assert no new charged search begins. Add list contract tests for scope isolation, empty response shape, sort order, exact summary fields, and authentication.
- [ ] Run RED tests and capture the stale-dispatch failure.
- [ ] Implement the pre-charge revalidation and list signature/handler; register the route and add contract tests without changing quota or multi-rule policy.
- [ ] Run `go test -count=1 ./internal/modules/career -run 'Rule|TriggerDue'` plus the route/handler contract test and `git diff --check`; report which checks started external I/O.
- [ ] Commit only owned backend files and report exact behavior.

**Acceptance:** A committed pause/edit prevents any later scheduled search from starting; the scoped list returns bounded summaries with the frozen response contract.

## Task 3b: Recover Web search rules after browser storage loss

**Dependency:** Interface-only dependency on frozen Task 3a contract; implementation may run in parallel with Task 3a from the same BASE. Integrate and verify after both task reviews.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `packages/api-client/src/career.ts` and focused contract tests; `apps/web/src/career/RulePage.tsx` and focused tests.

**Consumes / produces:** Strictly decode each `GET /api/v1/career/rules?cursor=<opaque>` page as `{ rules: RuleSummary[], nextCursor: string | null }`; accept `nextDueAt: null` only for non-enabled rules, and require a valid timestamp for enabled rules. Follow cursors with repeated-cursor protection until all summaries are loaded. Resolve localStorage's rule ID only if present in the scoped server list; auto-select the sole rule, require explicit selection for multiple rules, and block create on list failure. Capture user/tenant scope and persist unresolved set-rule `{requestId, ruleId?, query, intervalMinutes, status, expectedRevision}` before sending; after remount reconcile the same receipt or replay the exact request before enabling another create. Keep outgoing scope state isolated on switch.

**Steps:**

- [ ] Add failing tests for empty localStorage with an existing rule, multiple-rule selection, malformed/failed list decode, and reload after unknown create proving same request ID/revision and no duplicate create.
- [ ] Run RED tests and record existing duplicate/create-on-storage-loss behavior.
- [ ] Implement strict list decoding and server-first rule discovery/selection plus scope-keyed unknown-write persistence and receipt recovery.
- [ ] Run API client contract tests, RulePage tests, Web typecheck and `git diff --check`; record executable test results.
- [ ] Commit only the API client and RulePage files.

**Acceptance:** Storage loss cannot hide an existing server rule or create a duplicate while an earlier write is unresolved; all rule access remains user/tenant scoped.

## Task 11: Make due-rule claiming atomic with the pause/edit decision and bound list reads

**Dependency:** Repair to Task 3a Review finding `RULE-R2-BE-01`; use the exact persisted claim invariant below before code changes.

**Role:** `backend_implementer`; validator `backend_validator`; reviewer `reviewer`.

**Files:** `internal/modules/career/search_rule.go`, its database models/schema migration files and search-rule tests. Migration numbering must use the integration's finally ruled migration sequence.

**Consumes / produces:** In one short database transaction, lock the current rule row, require the scanned ID, enabled status, revision and query to match, then persist a recoverable period claim before any external call. The claim commit is the operation's start point. After the claim, `SearchOnce` keeps its deterministic `rule:<id>:<period>` request ID; retries recover/replay that same operation after a crash. Pause/edit transactions serialize against the same rule row. Paginate summaries with a fixed maximum of 50 rows, an opaque owner-scoped cursor over `(updated_at DESC, id ASC)`, and `{rules,nextCursor}` response; never load run/todo histories. `nextDueAt` is nullable for paused/disabled rules and a valid time for enabled rules.

**Steps:**

- [ ] Add a two-Office/shared-DB race test that blocks after candidate scan, commits pause/edit through another Office, then proves no claim/search starts. Add a second test where claim commits first and pause follows, proving the same request ID completes/reconciles as an already-started run. Add pagination tests for same-timestamp IDs, empty end page, malformed cursor, and >50 rules.
- [ ] Run regressions RED and record which race is currently accepted.
- [ ] Implement transactionally serialized claims and restart reconciliation without time-only claim takeover; do not hold a DB transaction over quota/network I/O. Add bounded cursor pagination and nullable `nextDueAt` schema/contract behavior.
- [ ] Run focused Career Go tests and the SQLite migration/up-down tests; run Postgres migration integration if configured, plus `git diff --check`.
- [ ] Commit only owned backend/schema paths and report exact linearization/recovery semantics.

**Acceptance:** A pause/edit that commits before a due-period claim prevents the external search; a committed claim resolves under the same deterministic request ID after restart; each list page is bounded and stable.

## Task 12: Keep RulePage locked through list, detail, and unknown-write recovery

**Dependency:** Task 11 response contract is fixed above; this frontend repair may run concurrently only after Task 11's interface is frozen, and integration must verify both together.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `packages/api-client/src/career.ts` and career API client tests; `apps/web/src/career/RulePage.tsx` and focused RulePage tests.

**Consumes / produces:** Match Task 11 cursor page and nullable schedule contract. Keep `viewPhase='loading'` until the rule list, selected detail, and any stored pending request have reached a known outcome. A found receipt is accepted; a missing receipt replays the exact attempt; any unresolved result keeps create/edit disabled. Add a request generation token for manual rule selection so an older `getRule` response cannot replace the current selection. Never create a new rule while either list/detail discovery or pending-write recovery is in flight or failed.

**Steps:**

- [ ] Add failing tests for paused/disabled `nextDueAt:null`, paged lists, pending create during slow receipt lookup, failed detail read, and out-of-order selection responses.
- [ ] Run tests RED; verify a new create is currently possible during each incomplete state.
- [ ] Implement strict cursor and nullable-field decoding; keep the form unavailable until initialization/recovery completes; fence detail responses by scope and selection generation.
- [ ] Run API-client tests, RulePage tests, Web typecheck, and `git diff --check`.
- [ ] Commit only API-client and RulePage paths; report race timelines and cursor termination evidence.

**Acceptance:** No second rule write can start before existing rule state and unresolved requests are known; delayed reads cannot overwrite the user's latest selection.

## Task 13: Coordinate Career deletion with operations across handlers and replicas

**Dependency:** Repair of Task 1's same-Office mutex; must integrate before claiming deletion races are closed.

**Role:** `backend_implementer`; validator `backend_validator`; reviewer `reviewer`.

**Files:** Career persistence/model/migration and repository seams, `internal/modules/career` operation admission and deletion paths, focused Career tests, and narrow Workbench/storage adapters only if fencing tokens are required.

**Consumes / produces:** The scope `(tenant_id, owner_user_id)`, existing request IDs for Career operations, `career_spaces` retained through deletion, Workbench `EnsureCareerApplicationTask`, and export storage writes/removals. Use a durable per-scope lifecycle gate shared by independently constructed `Office` handlers and processes. Keep database transactions short; never hold one across rendering, object storage, or Workbench calls.

**Steps:**

- [ ] Add deterministic two-Office tests with a shared database/storage/linker: pause an application linker and material writer after admission, start deletion through a second Office, and assert deletion cannot return terminal `deleted` while either admitted effect is unresolved; also assert new work is rejected after deletion enters `deleting`.
- [ ] Run the regressions and record the current cross-instance leak/order failure.
- [ ] Add a persistent scope gate and operation claims. Admit each external effect before its first side effect; retain claims until the operation outcome or compensation is durably known. Deletion transitions active→deleting only after claims are reconciled, retains deleting through cleanup and the terminal receipt, and rejects new claims. Do not expire claims by elapsed time alone; expose retry/recovery by original request ID.
- [ ] Ensure SQLite and PostgreSQL transitions serialize on the same durable row with conditional updates/row locks, and that failure or process restart leaves a retryable, non-terminal state.
- [ ] Run focused race and recovery tests, relevant Career/Workbench/container suites, migration tests, and `git diff --check`; expected: two Offices observe one ordering and no effect can appear after a successful deletion receipt.
- [ ] Commit owned paths and report schema IDs, claim recovery behavior, and exact test evidence.

**Acceptance:** Across separately constructed handlers/processes, every application link or material object effect is admitted by the shared gate; deletion cannot finalize while an earlier claim is unresolved and no later operation is admitted after deletion begins.

## Task 14: Correctly report saved preparation edits when follow-up reads fail

**Dependency:** None; scoped to the Career preparation revision UI and its behavior tests.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/miniprogram/src/career/progress-preparation.tsx` and its focused tests only.

**Consumes / produces:** Existing `career.editMaterial` durable receipt and `career.material` read. A successful edit is a committed fact even if the subsequent detail refresh fails; refresh failure must never re-enter the write-failure path or tell the user the edit was not submitted. The preflight material read used to preserve claims remains before submission and its failure must retain the explicit local draft and present typed recovery guidance.

**Steps:**

- [ ] Add regressions for (a) preflight claim read failure, and (b) edit success followed by material-detail read failure; assert only (a) says the edit remains unsubmitted and both retain enough recovery state.
- [ ] Run tests RED and capture the contradictory saved/not-saved message on (b).
- [ ] Move the preflight read into the handled local-draft error path; isolate post-commit detail refresh from the edit failure catch and keep the committed receipt/success state visible.
- [ ] Run focused preparation tests, Mini Program typecheck where executable, and `git diff --check`; report existing unrelated type errors separately.
- [ ] Commit only the page and its focused tests.

**Acceptance:** Once `editMaterial` resolves successfully, no later read error can tell the user that the edit was not submitted; a preflight failure remains an explicit non-submission with recoverable local draft.

## Task 15: Persist push subscription intent after native authorization is accepted

**Dependency:** None; owns the reminder subscription UI and its behavior tests.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/miniprogram/src/career/rules-usage-reminders.tsx` and focused page/adapter tests.

**Consumes / produces:** Existing `requestReminderSubscription()` native result, `setPushSubscription(value, expectedRevision)`, and Career revision. Native authorization is not delivery evidence. On an accepted authorization, persist `subscribed` under the current revision; on denial/unavailability leave server preference unchanged. If revision is unavailable or persistence outcome is unknown, state that authorization succeeded but server preference was not confirmed and retain same-request recovery semantics rather than claiming subscription complete.

**Steps:**

- [ ] Add tests for accepted authorization followed by successful preference write, accepted authorization with missing revision, rejected/unavailable results leaving preference untouched, and unknown preference-write recovery without a second native request.
- [ ] Run tests RED.
- [ ] Persist the subscription fact after accepted native authorization; display separate authorization and server-preference states and do not claim message delivery.
- [ ] Run focused rules/reminder tests, Mini Program typecheck where executable, and `git diff --check`; report existing unrelated type errors separately.
- [ ] Commit only owned reminder UI/test files.

**Acceptance:** Users who opt back in update the shared Career push preference; permission is never represented as delivery, and ambiguous preference writes remain recoverable.

## Task 4: Bind submitted progress to an actual submission record

**Dependency:** None; backend file ownership is disjoint from Tasks 1–3.

**Role:** `implementer`; validator `backend_validator` and focused frontend validator; reviewer `reviewer`.

**Files:** `internal/modules/career/progress.go`, `internal/modules/career/progress_test.go`, `apps/web/src/career/ProgressPage.tsx`, its tests, and only the API type file if a dedicated record input needs typing.

**Consumes / produces:** Dedicated `RecordSubmission` contract already binds channel, actual submission time and material version/unknown marker. Generic `AppendProgressInput` accepts an event type and note only.

**Steps:**

- [ ] Add server test rejecting generic `submitted` and `resubmitted` events without a bound submission; add UI test that the generic timeline omits those event choices and submission confirmation remains available through the dedicated flow.
- [ ] Run RED tests.
- [ ] Reject unbound event types at the service boundary and route Web users to the dedicated submission confirmation flow; keep timeline projection unchanged for legacy already-bound events.
- [ ] Run focused career progress/submission Go tests and ProgressPage tests; `git diff --check` must pass.
- [ ] Commit and report.

**Acceptance:** No UI or API path can claim a submission without channel/time/material binding or an explicit unknown-version marker.

## Task 5: Complete Web profile entry, purge private UI state after deletion, and freeze subscription retry payloads

**Dependency:** None; owns only `CareerPage.tsx` and `InboxPage.tsx` plus their tests.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/web/src/career/CareerPage.tsx`, related CareerPage tests, `apps/web/src/career/InboxPage.tsx`, related InboxPage tests.

**Consumes / produces:** Existing Career proposal/confirm actions and captured `ScopeController`. Extend the existing profile field choices to projects, internships and skills. Extend `SubscriptionAttempt` to include the first request’s `expectedRevision` and reuse it exactly on retry.

**Steps:**

- [ ] Add no-resume manual entry tests for projects/internships/skills; add deletion callback test with reload failure asserting profile facts and source names disappear immediately; add subscription retry test where revision advances before retry.
- [ ] Run RED tests.
- [ ] Add structured field choices and synchronous private-state clearing with stale response fencing; pin subscription revision in the attempt object and replay unchanged payload.
- [ ] Run targeted CareerPage/InboxPage tests and `git diff --check`.
- [ ] Commit owned files and report.

**Acceptance:** Candidates without a resume can enter required evidence; deletion immediately removes cached private data even when refresh fails; retries use byte-for-byte the original request semantics.

## Task 6: Make Web search writes recover after reload

**Dependency:** None; owns only `apps/web/src/career/SearchPage.tsx` and focused tests.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/web/src/career/SearchPage.tsx`, `apps/web/src/career/SearchPage.test.tsx` (or nearest existing behavior test).

**Consumes / produces:** Existing `searchOnce`, `searchReceipt`, `retryPendingSearch`, and scope-key storage helper contracts from the Web client. Persist the exact request ID, query and expected revision under the captured user/tenant scope before sending. On remount, reconcile by receipt before enabling a new charged search.

**Steps:**

- [ ] Add remount-after-unknown search tests for receipt-found and receipt-not-found cases; assert no second charge/new request ID.
- [ ] Run RED tests.
- [ ] Persist/load scoped pending search state and preserve unknown/receipt recovery UI after reload; clear only after a confirmed receipt or scope-specific safe resolution.
- [ ] Run targeted SearchPage tests and `git diff --check`.
- [ ] Commit and report.

**Acceptance:** Reload cannot mint a second charged search while an earlier result is unknown; another scope cannot read or clear the pending request.

## Task 7: Make mini-program Career writes durable and source evidence truthful

**Dependency:** None; coordinate file ownership within one task because `services/career.ts` owns the relevant state.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `packages/career-core/src/desk.ts` and tests; `apps/miniprogram/src/services/career.ts`, `apps/miniprogram/src/adapters/career-platform.ts`, `apps/miniprogram/src/career/application-material.tsx`, `apps/miniprogram/src/career/discovery.tsx`, and focused tests.

**Consumes / produces:** Existing controlled scope-aware `career-intent.ts` storage/replay utilities; existing receipt endpoints for profile actions, share import, evaluations and search. Persist `{requestId, exact input, expectedRevision}` before all remote writes. Unknown results reconcile/replay with the same ID after restart. Decode a `completed` search only when coverage, sources, rows, evidence fields and timestamps pass required schema checks. Show the complete shared JD in a scrollable user confirmation view. Show hard-condition evidence, supporting facts, matches and gaps before explicit continue.

**Steps:**

- [ ] Add restart-after-ambiguous-write tests for profile mutation, share import and evaluation; malformed completed search receipt tests; long shared JD full-confirmation test; evaluation evidence display tests.
- [ ] Run tests RED and record which receipt endpoints exist/missing. If a receipt endpoint is absent, add the narrow scoped API endpoint and contract test before client adoption.
- [ ] Persist intents and block conflicting writes until receipt resolution; reject malformed completed receipts; display full submitted JD and specific qualification evidence.
- [ ] Run focused mini-program tests, `pnpm --filter @weknora/miniprogram typecheck`, and `git diff --check`; expected: restart recovery uses the original IDs. Record any unrelated pre-existing type errors separately.
- [ ] Commit owned paths and report API contract changes.

**Acceptance:** Restart does not lose any unknown-write request identity; malformed data is not rendered as a legitimate empty result; the user sees exactly the JD and evidence that inform explicit confirmation.

## Task 8: Make mobile authorization smoke evidence distinguish redirects and gate the unauthenticated boundary

**Dependency:** None; owns the smoke driver and its API-client integration test.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/mobile/src/task-office-integration-smoke.ts`, `packages/api-client/src/mobile/task-office.integration.test.ts`, focused smoke test if present.

**Consumes / produces:** `serverAuthBoundary: 'rejected' | 'failed-open' | 'unreachable'` evidence field. Use `redirect: 'manual'`; classify 3xx/login redirects as failed-open. Live gate must assert unauthenticated boundary is rejected and client-side write gate is closed.

**Steps:**

- [ ] Add tests for 302-to-login, 401/403 rejection and 200 unauthenticated success. Add live-gate assertion for `serverAuthBoundary === 'rejected'`.
- [ ] Run RED tests.
- [ ] Implement explicit response classification and fail the gate unless both server and client boundaries are closed.
- [ ] Run targeted smoke tests and API client tests plus `git diff --check`.
- [ ] Commit and report.

**Acceptance:** Redirects cannot be mislabeled unreachable; a backend allowing unauthenticated reads cannot produce a passing live-gate result.

## Task 9: Re-sequence #140 and colliding #30 migrations without duplicate version numbers

**Dependency:** None; migration files and database migration tests are separate from Tasks 1–8. This task must integrate before any release candidate is tested or built.

**Role:** `mechanical_worker`; validator `backend_validator`; reviewer `reviewer`.

**Files:** Paired migration files in `migrations/sqlite/` and `migrations/versioned/` for the 19 #140 Career/Workbench migrations and 12 colliding #30 migrations; `internal/database/migration.go`; `internal/database/migration_version_uniqueness_test.go`; filename/version references in Career, Workbench, code-delivery and mobile migration tests.

**Consumes / produces:** Preserve within-cohort dependency order. Move #140 `career_profile` through `career_reconciliations` from SQLite 124–142 to 112–130 and versioned 203–221 to 191–209. Move #30 `task_grants` through `space_connection_grants` from SQLite 112–123 to 131–142 and versioned 191–202 to 210–221. Move the SQLite `public_agent_marketplace` no-transaction gate and exact migration path from version 114 to 133. Migration loader must observe one unique, contiguous sequence; both public trees must point to the same schema transition for each name.

**Steps:**

- [ ] Add/update a migration uniqueness/identity test that asserts both trees contain the expected paired names and unique versions after the mapping; update version-specific tests to assert the moved paths and down/up behavior.
- [ ] Run migration-focused tests and capture any stale filename or numeric assumptions.
- [ ] Apply the complete paired-file mapping, update the SQLite special transaction gate, and update every discovered filename/version reference.
- [ ] Run `go test -count=1 ./internal/database ./internal/modules/commercial/... ./internal/modules/plugins/...` plus the direct code-delivery and mobile migration tests identified during implementation; run `git diff --check`.
- [ ] Commit only migration files, loader and migration tests. Report the assumption that no existing deployment requires the displaced #30 numeric identities, because golang-migrate stores numeric versions rather than migration identity.

**Acceptance:** No duplicate migration numbers remain; fresh SQLite and PostgreSQL migration sequences apply Career/Workbench at 112–130 / 191–209, #30 schema changes remain in dependency order at 131–142 / 210–221, and the special SQLite marketplace migration still executes outside a transaction.
