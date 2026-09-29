# Issue #140 integration and final review addendum

- Date: 2026-09-29 (Asia/Shanghai)
- Workspace: `/Users/wuyongjun/.codex/worktrees/issue-140-sweep-integration/WeKnora-fork01`
- Branch: `codex/issue-140-sweep-integration`
- Original comparison BASE: `db234c5eb171f2dde7427d382b55b503a038f879`
- Current task HEAD: `3712062df7d672a77d7411aa055660d13d63351a`
- Issue tree and source snapshots: `docs/plans/issue-140/issues/issue-140.md`, `issue-141.md` through `issue-173.md`, and `docs/plans/issue-140/2026-09-24-issue-140-dag.md`.

## Confirmed migration decision

The user confirmed on 2026-09-29 that #140 Career/Workbench migrations had not been applied to databases that need to be retained, and that migration versions could be reordered. The migration plan therefore preserves the issue30 baseline and moves only the #140 migration pairs to SQLite 124–142 and versioned PostgreSQL 203–221. Do not reorder the issue30 migration history.

## Task 11: local authentication sentinels

Integrated commits: `130b96bd4`, `daddfcd85`, `e81c867f4`.

`RUNTIME_UNAUTHORIZED`, `AUTH_REQUIRED`, and `SCOPE_CHANGED` now count as definite local failures only when represented by an exact native `Error` message without `code` or HTTP `status`. A 503 server error containing all three strings remains ambiguous and preserves its recovery intent. The initial reviewer finding on substring classification was corrected; round-2 independent review and frontend validation passed.

Evidence: [implementation report](../../superpowers/reports/2026-09-29-issue140-task11-report.md), [round-2 review](../../superpowers/reports/2026-09-29-issue140-task11-review-round2.md), [round-2 validation](../../superpowers/reports/2026-09-29-issue140-task11-validation-round2.md).

## Task 12: captured-scope deletion retry

Integrated commits: `e0fa1597d`, `3712062df`.

The original N9 failures came from fixtures answering whichever asynchronous native request happened to be last. Tests now wait for the exact deletion POST. Completion coverage switches scope after response resolution but before the Career service continuation, asserts one replay with the original request ID and completed receipt, removes only the original scope key, and preserves a pre-existing other-scope intent. Ambiguous timeout coverage confirms both scopes' intents survive. No production code change was needed after the fixture was made deterministic.

Evidence: [N9 review](../../superpowers/reports/2026-09-29-issue140-task12-n9-review.md), [final validation](../../superpowers/reports/2026-09-29-issue140-task12-validation-final.md).

## Verification recorded for this checkpoint

- Focused N9 tests: 3 passed, 0 failed.
- `apps/miniprogram/tests/export-deletion.test.mjs`: 33 passed, 0 failed.
- Full `pnpm --filter @weknora/miniprogram test` on Node 24.18.1, rerun after the final fixture update: 214 passed, 0 failed, 1 opt-in integration test skipped (215 total). pnpm emitted the declared package-engine warning because the package expects Node >=26. The repository-installed Node 26.4.0 does not support the script's `--experimental-transform-types` option.
- `pnpm --filter @weknora/miniprogram build:weapp` on Node 26.7.0: passed.
- `go test -count=1 ./internal/container ./tools/architectureguard`: passed.
- `go run ./tools/architectureguard --root .`: passed with 17 modules, 763 routes (676 literal, 87 API-key, 0 Handle), 59 hooks, zero violations.
- Final full Web run, default configured file concurrency (`pnpm test:web`): `timeout 900s` exited 124. Node reported 2,523 tests, 2,522 passed, 0 failed, 1 cancelled; `src/settings/GeneralPreferencesPanel.test.tsx` was cancelled with `Promise resolution is still pending but the event loop has already resolved`. This attempt did not complete successfully. Raw command output: `/tmp/issue140-web-final.log` (local temporary evidence).
- Final full Web run, serial file concurrency, same integration HEAD: `timeout 900s pnpm --filter @weknora/web exec node --import tsx --test --test-concurrency=1 'src/**/*.test.ts' 'src/**/*.test.tsx'` exited 0; 2,528 passed, 0 failed, 0 cancelled, 0 skipped, duration 357,052 ms. Raw command output: `/tmp/issue140-web-serial-final.log` (local temporary evidence). The full Web suite is verified by the serial run; the earlier parallel hang remains separately recorded as a scheduler/test-isolation issue to investigate if default-concurrency reproducibility is required.
- Previous focused checks recorded in Task 3–10 reports remain the evidence for those tasks. Existing final-delivery material documents mobile platform/device gates and older OCR rounds; those prior reports do not replace the final OCR requirement for this exact BASE/HEAD pair.

## Parent acceptance adjudication

The controller reviewed the 43-story mapping against the root issue snapshot and final delivery evidence. Thirty-seven stories map only to verified nodes; stories 1, 9, 26, 36, 40, and 42 depend on blocked mobile nodes T23/T29/T31. T33/#172 is now **blocked**, not verified: the five-environment chain is incomplete, Android/Harmony native gates remain unavailable, the production source allowlist is empty, and source/model/WeChat/privacy/payment operations checks remain unverified. The detailed ruling and story mapping are recorded in the [DAG](2026-09-24-issue-140-dag.md#2026-09-29-主控-t33--140-验收裁定). GitHub issue state was not changed.

## Final OCR status and completion boundary

The required final review command covered `db234c5eb171f2dde7427d382b55b503a038f879..3712062df7d672a77d7411aa055660d13d63351a` with `--audience agent` and business context. OCR selected 333 review items, but all 333 failed; its retry report recorded 18 rate-limited failures and 8 cancellations across 28 requests. `ocr llm test` independently returned HTTP 429, provider code 1302 (account rate limit). A later full-range retry through the subsequent documentation HEAD again reached all 333 items but was interrupted after the same provider's HTTP 429/code 1308 five-hour quota response; zero items completed and no report was generated for that attempt. Neither run is a passing review. The detailed output from the earlier attempt is [the final OCR attempt report](../../superpowers/reports/2026-09-29-issue140-final-ocr.md); the interrupted retry is recorded in the local Open Code Review session log, session `0c8aafeb-1455-41c3-9d9c-f95983814483`.

This checkpoint is therefore not complete under the repository's completion rules. The current DAG HEAD must be committed before a final OCR BASE-to-HEAD review; resume after the configured provider's quota clears, then fix/review any valid critical/high/medium findings using the issue140 repair workflow. No push, PR, merge, or publication was performed.

## OCR finding repair wave and coverage blocker (2026-09-30)

The user confirmed on 2026-09-30 that no database has applied the #140 Career/Workbench migrations and requested a reorderable version if needed. The existing integration plan's numbering remains the selected order: preserve the issue30 baseline versions, retain the original #140 dependency order, and place #140 at SQLite 124–142 / versioned PostgreSQL 203–221. The user-visible confirmation removes the legacy-database constraint; no evidence currently supports changing the internal dependency order.

### OCR attempts and findings

- Original full-range session `c3984cc3-304f-49f2-ae28-b9d540496b5b`, original BASE..HEAD `db234c5eb171f2dde7427d382b55b503a038f879..b3d48d5cb9cd8833ea348d4a6f37a9bef52ae13b`: 333 selected, 216 completed, 117 failed, 367 comments, 18 LLM failures. Resume was rejected because the resolved rule identity differed from the session. This is not a pass.
- Fresh session `b5b5cf36-2ff1-49bd-9303-6a9c9c8641c6`, same range: aborted by OCR after internal orchestration; summary 0 selected, 15 completed tool groups, 2 failed, 15 comments, 0 LLM failures. Findings were harvested from the session artifact but there is no complete file coverage.
- Fresh session `b3e6486a-0e3e-4b3e-8785-71aae5420c1d`, updated range `db234...`..current pre-repair HEAD `b3d48d5cb9`: aborted with 0 selected/completed/failed/comments. JSONL shows only a partial Embed review group and tool replies including “No matches found”; no coverage was recorded.
- All session artifacts are under `/Users/wuyongjun/.opencodereview/sessions/Users-wuyongjun-.codex-worktrees-issue-140-sweep-integration-WeKnora-fork01/`. OCR provider connection test passed at 2026-09-30 02:35 CST. The blocking behavior was session/review orchestration abortion, not provider quota at that time. Repeated identical retries were stopped.

Four medium comments from the 15-comment partial session were checked against source:

1. `.superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs` hard-coded replay credentials contradict the documented env-injection rule. **Valid.** Fixed by commit `6df56a011`; credentials now use `T33_USER_EMAIL` / `T33_USER_PASS`, missing values fail before automator interaction without printing values. `node --check`, missing-env subprocess, and `git diff --check` passed.
2. `apps/embed/src/styles.css` brand values bypass tokens and text controls do not adapt to dark mode. **Valid.** Fixed by `1db12dcca`; token-backed primary colors and current-color based dark-mode control styling added. New embed style assertions were RED before fix; `pnpm --filter @weknora/embed test` passed 21/21; embed TS check and `git diff --check` passed. Node 22.22.3 emitted the workspace Node >=26 engine warning.
3. `apps/web/src/embed/embed-u.css` lost the utility's 2px underline offset. **Valid.** Fixed in `1db12dcca`; new test checks the selector and declaration.
4. `apps/miniprogram/src/platform/files.ts` classified any unknown 401 as invalid artifact grant. **Valid.** Fixed by `1f7ca8519`; only exact expired/invalid grant response codes map to artifact grant codes; generic 401 retains login-expired copy. Assembly suite passed 22/22 both in implementation and integrated re-run; reviewer reported Spec compliance PASS and code quality PASS. Package typecheck remains blocked by pre-existing missing `CommercialSummary` fields in `src/features/account/pages.tsx`.

The partial OCR produced 11 low findings; they remain unruled pending complete review context and are not treated as blocking fixes or as approved deferrals.

### Repair plan and integration record

- Plan: `docs/superpowers/plans/2026-09-30-issue140-ocr-finding-repairs.md`.
- Integrated repair commits: `6df56a011`, `1db12dcca`, `1f7ca8519` (all descendants of original issue30-sweep BASE; no remote mutation).
- Current branch: `codex/issue-140-sweep-integration`; final repair HEAD `1f7ca8519a384bbc6bff554e27462ab6a276f832`.
- Focused independent review and validation are complete for mini-program. Embed task validation is recorded above; task-specific independent review/validation evidence is in the subagent reports. T33 mechanical check output is recorded in the task report/session.
- New full-range OCR after these fixes is still required. The current report file does not exist because OCR aborted before publication. Do not report the outer OCR gate as passed.

### Completion boundary

Issue #140 remains **partially complete**: T33/#172 and downstream native acceptance remain blocked by unavailable Harmony toolchain/device, production-source authorization/allowlist, and manual operations checks. Full-range OCR is also blocked by repeated incomplete session orchestration; no complete coverage report exists for the repair HEAD. Keep this addendum and all worktree/branch evidence; do not clean up or push/merge.
