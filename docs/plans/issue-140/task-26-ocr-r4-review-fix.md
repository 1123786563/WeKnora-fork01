# T26 OCR round-4 review repair

## Scope and checkout

- Original task BASE: `76df0cee0bf3ae23c14411c345b151ad518077ee`.
- Reviewed R1 implementation: `bc19b9fd36d473ce3e3afd8a0c11f0067634a2af`.
- Repair began on integrated shared-worktree HEAD `b2e6a1e7bb77ddc0d6295769a59e098bde1d18c4`; current checkout before report/commit is `b3006beb01d81bb87c53b8a1f6c161ffe5931150` (shared checkout also contains separate web and planning/OCR changes, excluded from this task commit).
- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01`.
- Inputs: `task-26-ocr-r4-review.md`, OCR round-4 analysis and resume increment analysis, approved job-search spec, career context and ADR 0017.

## Corrections

- R1: recovery abandonment captures the displayed request ID, waits for confirmation, rechecks current ID and synchronous operation activity, then uses an expected-ID conditional service clear. Cancel, changed ID, active operation, and clear errors retain the recovery record and report status. Export abandonment is disabled during export activity as well as recovery activity.
- R2: extracted `confirmAbandonIntent` and added deferred-promise behavior tests for cancellation, request-ID replacement, activity starting during confirmation, and successful same-ID clearing. Existing source wiring checks remain supplemental; no page renderer is configured in the current miniprogram test harness.
- N1: `deleteWholeSpace` refuses with typed `unresolved_action` before allocating/sending a new request if a deletion intent is pending. It leaves the original ID intact; retry/reconcile or explicit expected-ID abandonment are the continuation paths.
- N8: retries use explicit per-operation revision fallback; progress retries no longer fall back to profile revision. A legacy progress intent lacking its expected revision fails closed with `intent_revision_missing` and retains its intent.
- N9: deletion sends capture the scope stamp and intent key before awaiting the request. Writes/removal after completion use that captured key, preventing a scope switch from creating or deleting another scope's record.

## TDD and verification

- RED evidence: N1 guard test failed before implementation; expected-ID helper behavior tests failed before helper implementation; N8 missing-progress-revision test failed before fallback was specialized. N9 tests were run with the old dynamic-key post-await read/write restored temporarily; both failed on the scope-switch seam, then passed with captured-key handling restored.
- `cd apps/miniprogram && node --experimental-strip-types --test tests/export-deletion.test.mjs tests/progress-preparation.test.mjs` — **49 passed, 0 failed** (rerun after report creation).
- `pnpm --filter @weknora/miniprogram test` — **197 passed, 0 failed**.
- `pnpm --filter @weknora/miniprogram build:weapp` — **passed**; webpack reports the existing 477 KiB `common.js` size recommendation and async chunk recommendation.
- `pnpm --filter @weknora/miniprogram typecheck` — **blocked by existing unrelated errors** in `src/features/account/pages.tsx`: 13 `CommercialSummary` property mismatches (`available`, `held`, `refund_locked`, `stale`, `as_of`, `plan_name`, `paid_until`). No diagnostics point to owned files.
- `git diff --check` — **passed**.
- No OCR was run. No device/runtime UI smoke test was available; helper behavior and page wiring are covered through node tests and the build, but native modal rendering was not exercised.

## Change boundary and remaining risk

Owned changes are limited to the mini-program export/deletion page, its pure gating helper, career services, and corresponding tests. Shared web edits and other agents' planning/review/OCR files are intentionally excluded. `reconcilePendingSpaceDeletion` was not changed: the N9 finding and task direction target the deletion send and retry send continuations. Typecheck remains subject to the unrelated account-page baseline issue above.
