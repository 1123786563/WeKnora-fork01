# Independent Task 11 follow-up review — round 2

Scope: committed delta `daddfcd85..e81c867f4` only. Reviewed against the approved job-search spec, ADR-0015/0018, `CONTEXT.md`, and Task 11 in `docs/superpowers/plans/2026-09-29-issue140-miniprogram-residuals.md`. No source or test files were changed by this review; OCR and tests were not run. `git diff --check daddfcd85 e81c867f4` passed.

## Findings

None in the reviewed delta.

## Evidence and verdict

**Spec compliance: PASS for Task 11 follow-up.** `definiteLocalFailure` in `apps/miniprogram/src/services/career-intent.ts` recognizes only native `Error` objects whose message is exactly `SCOPE_CHANGED`, `AUTH_REQUIRED`, or `RUNTIME_UNAUTHORIZED`, and whose `code` and `status` are absent. `MobileRuntime` throws the exact `RUNTIME_UNAUTHORIZED` error before transport. The existing logout tests for material publish, search, and deletion assert no network call, no persisted intent, and the original error as cause. This preserves the brief's definite local failure behavior.

**Code quality: PASS for this delta.** The previous medium finding is resolved: the classifier no longer matches sentinel substrings in server messages. `errorFromResult` creates an `ApiError` with HTTP `status` and `code`, so a 503 response remains ambiguous even when its message contains sentinel words. The expanded `application-material.test.mjs` regression puts all three names in one 503 response and asserts `outcome_unknown`, the original 503 cause, and a persisted publish intent. Existing recovery catch paths share the classifier. The changed files are within Task 11 ownership; the report-only addition records evidence.

Limit: this is a read-only review of the stated commit range. Test pass counts in the implementation report were not independently rerun. Concurrent uncommitted changes to `export-deletion.test.mjs` and other agents' reports were outside this scope.
