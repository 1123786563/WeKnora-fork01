# Issue 140 Mini Program Integration Residuals

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement these tasks. Keep each task in its own isolated worktree and use RED → GREEN → REFACTOR where applicable.

**Goal:** Close Mini Program integration regressions exposed by the configured Node 22 package suite after the Task 1 route merge and Task 7 auth-fixture repair.

**Architecture:** Keep the single `MobileRuntime` as credential and request owner. Do not restore removed hand-rolled execution state modules. Keep recoverable Career intents only for operations whose server outcome can be unknown; an unauthenticated local rejection is definite. Preserve request IDs and captured-scope storage for deletion recovery.

**Spec:** `docs/plans/issue-140/2026-09-24-issue-140-dag.md`, especially #148, #164, #166, #169, #170 and #173; `docs/specs/2026-09-23-weknora-job-search-design.md`; Task 1 and Task 7 sections in `docs/superpowers/plans/2026-09-29-issue140-integration-blockers.md`; observed suite evidence in the Task 7 report and `docs/architecture/integration/research-t7-residual-failures.md` in its task worktree.

**Baseline:** Integration branch before this residual plan is `5c33ee77b` (includes reviewed Tasks 1, 3, 4, 5, 6, and Task 8 implementation/test). Task 7 fixture changes are in a separate reviewed-work-pending worktree and must be integrated before the final package suite gate.

## Global Constraints

- Preserve the exact issue30-sweep base `db234c5eb171f2dde7427d382b55b503a038f879` as an ancestor; do not push.
- Do not modify the user's original worktree or its unrelated changes.
- Do not turn authentication failures into recoverable writes, suppress scope changes, weaken authorization, or retry non-read requests after 401.
- Do not restore `src/core/execution.ts`, which was intentionally deleted when Task Office became the sole execution owner. Existing `office-assembly` behavior tests cover snapshot watermark and SSE assembly.
- Keep task branches/worktrees isolated. Integrate only after focused validation and independent review.
- Commit locally; no push, merge to a remote, deployment, publishing, or GitHub mutation.

## Review Focus

- Read-only authorized requests still refresh once and retry with the rotated token.
- Write requests do not refresh or replay after a 401.
- A local `RUNTIME_UNAUTHORIZED` failure is classified as definite and leaves no Career recovery intent.
- A completed deletion retry removes the original captured-scope intent even if the active UI scope changes after the server response; a different scope never sees or clears that intent.
- Test helpers exercise the public adapter seam; stale tests do not import intentionally removed implementation modules.

## Task 9: Repair Mini Program Test Seams After Execution Module Removal

**Dependency:** None; isolated test harness files do not overlap Task 7 fixture ownership.

**Role:** `mechanical_worker`; validator `frontend_validator`; reviewer `reviewer`.

**Owned files:**
- `apps/miniprogram/tests/core.test.mjs`
- `apps/miniprogram/tests/platform-adapters.test.mjs`
- `apps/miniprogram/tests/helpers/taro-stub.mjs`

**Consumes / produces:** Use the current Task Office API and its existing `office-assembly.test.mjs` snapshot/SSE contract tests. The platform stub should expose a supported dispatch seam for adapter tests, or those tests should be rewritten to the public handler seam. The core suite must not import removed `src/core/execution.ts`.

**Steps:**

- [ ] Confirm `src/core/execution.ts` was intentionally removed by commit `9a5a13f91` and that `office-assembly.test.mjs` exercises snapshot watermark and events after the watermark.
- [ ] Remove or relocate only obsolete direct tests for the deleted projection helper; retain current scope, UTF-8, intent, formatting, and route tests.
- [ ] Fix the three platform adapter tests so stream pre-response status, stream chunk delivery, and binary blob fetch reach the actual transport seam.
- [ ] Run `node --experimental-transform-types --test tests/core.test.mjs tests/platform-adapters.test.mjs tests/office-assembly.test.mjs` from `apps/miniprogram` under Node 22.22.3 and `git diff --check`.
- [ ] Commit the test-only correction and report exact tests and results.

**Acceptance:** The focused tests pass; no production source changes; current execution stream behavior remains tested; no missing-module or undefined-dispatch failure remains.

## Task 10: Limit 401 Refresh Replay to Read Requests

**Dependency:** None; owns only the shared MobileRuntime request policy and its unit tests.

**Role:** `implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Owned files:**
- `packages/mobile-core/src/runtime/mobile-runtime.ts`
- `packages/mobile-core/src/runtime/mobile-runtime.test.ts`

**Consumes / produces:** Keep `unauthorizedStatus` and the existing single-flight refresh. On a 401, only read-only request methods may refresh and replay. A write method returns the original 401 with one transport attempt; retries keep the same deployment/scope guards.

**Steps:**

- [ ] Add focused tests that show GET 401 still refreshes once and succeeds, while POST 401 neither refreshes nor sends a second request.
- [ ] Run the focused tests first and capture the failing POST replay assertion.
- [ ] Implement the method-sensitive replay policy without changing sign-in refresh, scope fencing, or stream authorization.
- [ ] Run `pnpm exec tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts` and `git diff --check`.
- [ ] Commit and report the exact policy and validation.

**Acceptance:** GET behavior remains refresh-once; write behavior is single-attempt and does not call refresh; focused mobile-core tests pass.

## Task 11: Keep Local Runtime Authorization Failure Out of Recovery Intents

**Dependency:** None; code and test files do not overlap Task 10.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Owned files:**
- `apps/miniprogram/src/services/career-intent.ts`
- `apps/miniprogram/tests/application-material.test.mjs`
- `apps/miniprogram/tests/career-discovery.test.mjs`
- `apps/miniprogram/tests/export-deletion.test.mjs`

**Consumes / produces:** MobileRuntime reports a missing credential as `RUNTIME_UNAUTHORIZED` before transport. Career's frozen OCR2-037 behavior treats this as a definite local failure: no POST/search request and no persisted intent; preserve the original error as cause where a scope marker is returned.

**Steps:**

- [x] Reproduce the three OCR2-037 cases after local logout and capture error code/message plus storage state.
- [x] Add/adjust tests to assert there is no network request and no saved intent for material publish, search, and whole-space deletion.
- [x] Extend the shared definite-local-failure classification to recognize exact local sentinels without classifying network ambiguity or server errors as definite; 503 regression contains all three sentinel strings and remains recoverable.
- [x] Run focused auth/sentinel tests (4/4), all application-material tests (38/38), career discovery tests (26/26), and `git diff --check`; round-2 independent review passed.
- [x] Commit and report evidence in `docs/superpowers/reports/2026-09-29-issue140-task11-report.md`, `*-review-round2.md`, and `*-validation-round2.md`.

**Acceptance:** All three operations fail closed before transport, retain the authentication failure for UI handling, and leave their recovery stores empty; ambiguous network failures still persist recoverable intents.

## Task 12: Complete Deletion Retry Against Its Captured Scope

**Dependency:** Task 11 verified and integrated, because both tasks change Career deletion tests and share the Career service recovery contract.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Owned files:**
- `apps/miniprogram/src/services/career.ts`
- `apps/miniprogram/tests/export-deletion.test.mjs`

**Consumes / produces:** Preserve the request ID and key captured for the original deletion. If a retry receives a completed server receipt, remove that captured key even if active scope changes after the response. Never read, expose, or remove the other scope's intent. Ambiguous transport failures retain the old intent. First establish that the test actually responds to the intended native POST; do not change production recovery code unless traces show a product defect after the fixture is deterministic.

**Steps:**

- [x] Reproduce both N9 deletion tests with per-request traces; found the fixtures could answer a preceding asynchronous native request instead of the targeted deletion POST.
- [x] Add captured-scope, new-scope, request-count, receipt, and ambiguous-timeout assertions.
- [x] Add a bounded wait for the intended deletion POST. After deterministic fixture repair, no production defect remained; production recovery code was unchanged.
- [x] Run N9 cases (3/3), full export/deletion file (33/33), full Mini Program suite (214 passed, 0 failed, 1 opt-in skip), and `git diff --check`; independent review passed.
- [x] Commit and report evidence in `docs/superpowers/reports/2026-09-29-issue140-task12-n9-review.md` and `*-task12-validation-final.md`.

**Acceptance:** Both N9 cases finish promptly; completion clears only the original scope's key; an incomplete or ambiguous retry preserves that key; existing deletion recovery tests pass.

## Integration and Final Verification

- Integrated T11/T12 at `3712062df7d672a77d7411aa055660d13d63351a`; Mini Program WeChat build passed with Node 26.7.0.
- Final range OCR attempted for `db234c5eb171f2dde7427d382b55b503a038f879..3712062df7d672a77d7411aa055660d13d63351a`. OCR selected 333 items but all 333 failed because the configured provider returned HTTP 429 rate limits (18 failed and 8 cancelled out of 28 review requests); no findings were produced. `ocr llm test` independently returned account rate limit code 1302. The full-range OCR gate remains incomplete; do not treat this attempt as a pass.

- T9, T10, and T11 are independent and may be implemented in separate worktrees. T12 waits for T11 and must also integrate after Task 7 so the shared deletion test file is current.
- Integrate in dependency order and resolve only task-owned conflicts. Re-run affected focused tests after integration.
- After Task 7 and all residual tasks are integrated, run the configured full Mini Program suite under Node 22.22.3, `pnpm --filter @weknora/miniprogram build:weapp`, the Task 10 package tests, `git diff --check`, then complete-range OCR from `db234c5eb171f2dde7427d382b55b503a038f879` to final HEAD plus workspace content.
- Record any unsupported live PostgreSQL replay (unset `TRPC_TEST_POSTGRES_DSN`) and any true-device environment limits in the persistent final report.
