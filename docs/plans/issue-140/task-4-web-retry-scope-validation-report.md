# T10 UI Retry and Scope Repair Independent Validation

- **Verdict:** DONE_WITH_CONCERNS
- **Validated code SHA:** `4262f948b45b8a947f054b0f3f8aba2cd82accd0` (`fix(web): preserve evaluation retry identity`)
- **Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-t10-ui/WeKnora-fork01`
- **Observed HEAD:** `1d203b23566d5b944499e9b0eae88dbb1810aee0`, documentation-only descendant. The two changed Task 4 source/test paths are unchanged between assigned SHA and HEAD.
- **Tracked source/test changes by validator:** none.

## Same-SHA verification evidence

Reused `task-4-web-retry-scope-r1-report.md`:

- `pnpm --filter @weknora/web exec node --import tsx --test src/career/OpportunityPage.test.tsx src/routes.test.ts src/router.test.tsx` — **PASS**, 49 tests.
- `pnpm typecheck:web` — **PASS**.
- `pnpm test:web` — **PASS**, 2,345 passed, 0 failures.
- `pnpm build:web` — **PASS**; report records the build succeeded with existing CSS and bundle warnings.
- `git diff --check` — **PASS**.

Read-only revision checks:

- `git rev-parse HEAD` — `1d203b23566d5b944499e9b0eae88dbb1810aee0`.
- `git show --stat --oneline 4262f948b45b8a947f054b0f3f8aba2cd82accd0` — assigned source commit changes the Career page and its focused tests.
- `git diff --quiet 4262f948b45b8a947f054b0f3f8aba2cd82accd0 HEAD -- apps/web/src/career/OpportunityPage.tsx apps/web/src/career/OpportunityPage.test.tsx` — **PASS**, no Task 4 code/test drift after the assigned SHA.

## Focused findings

- **TIMEOUT, committed POST, receipt/retry identity:** `TIMEOUT` and `outcome_unknown` classify as uncertain writes. Both transient conversation and stable saved-evidence flows retain the request ID, expose receipt lookup, and on `not_found` retry with that same ID. Successful receipt lookup passes through `accept`, which checks request ID plus opportunity/snapshot before showing the result. Receipt success therefore resolves the committed-timeout case without issuing another POST by code inspection.
- **Transient and stable actions:** separate same-SHA tests exercise both entry points. Duplicate in-flight work is guarded synchronously. A new ID is used only after a completed evaluation or definite failure; uncertain retries preserve the intent ID.
- **A→B route identity:** evidence-page tests confirm A’s private text and ineligible result disappear while B loads, then assert a later evaluation uses B’s opportunity and snapshot IDs. Evaluation detail tests confirm A’s conclusion and provenance are hidden while B loads and B’s result is shown after completion. Components also key local evaluation action state by immutable route identity.
- **403/logout/scope:** explicit forbidden read tests exist; evaluation actions clear latest/history on forbidden responses and when the active scope signal aborts. Scope-switch tests verify private detail content clears. The signal handler labels both scope switch and logout. No dedicated test switches scope during the new evaluation POST itself.

## Gaps and risks

- No incorrect behavior was found in the reviewed retry or identity logic.
- The focused TIMEOUT tests exercise receipt `not_found` then same-ID retry, but do **not** exercise the committed-timeout branch where the receipt lookup succeeds immediately; that branch is supported by the shared receipt acceptance path but is not directly asserted.
- No live authenticated API/browser run was performed, and there is no direct pending-POST logout test. The implementation report assigns interactive browser acceptance to the controller. These remain acceptance evidence limits, not observed failures.
