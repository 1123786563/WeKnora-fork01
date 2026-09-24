# T10 Evaluation Draft Race Independent Validation

- **Verdict:** DONE_WITH_CONCERNS
- **Validated code SHA:** `544f74d8e7f9374185857e9714467ea1de7c5d9b` (`fix(career): fence late evaluation results across drafts`)
- **Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-t10-ui/WeKnora-fork01`
- **Observed HEAD:** `544f74d8e7f9374185857e9714467ea1de7c5d9b`.
- **Tracked source/test changes by validator:** none.

## Checks run at the assigned SHA

- `pnpm --filter @weknora/web exec node --import tsx --test src/career/OpportunityPage.test.tsx` — **PASS**, 30 passed, 0 failed.
- `pnpm typecheck:web` — **PASS**, exit 0.
- `pnpm build:web` — **PASS**, Vite completed in 25.38 seconds. Existing CSS `@import` and `calc()` warnings and a large bundle were reported.
- `git diff --quiet 544f74d8e7f9374185857e9714467ea1de7c5d9b HEAD -- apps/web/src/career/OpportunityPage.tsx apps/web/src/career/OpportunityPage.test.tsx` — **PASS**, assigned SHA is HEAD and no post-SHA source/test delta exists.
- `git rev-parse HEAD` — `544f74d8e7f9374185857e9714467ea1de7c5d9b`.

The full Web suite was not rerun, per assignment. The earlier run stalled in the unrelated agent-editor suite and was interrupted.

## Acceptance findings

- **Late POST A after starting/importing B:** deferred-response tests verify A's late success cannot show A's result beneath B or unlock B's in-flight state. B remains in the textarea, and B receives its own fresh request against B's opportunity/snapshot IDs.
- **Late receipt lookup A after starting B:** a deferred A receipt resolving after B is created does not restore A's evaluation result or alter B's state.
- **Late uncertain/unknown A response:** a deferred A timeout after B is imported does not show A's unknown state or original-ID retry beneath B; B remains ready for its own fresh evaluation.
- **Normal same-draft flow:** earlier focused tests in the same 30-test Career suite still cover normal saved-JD evaluation and result-link presentation.
- **Scope/403:** same suite includes scope-switch fencing and both POST/receipt-lookup 403 clear cases from the preceding repair; `clearPrivate` increments draft generation and clears current receipt and in-flight marker.
- **Fixed IDs / state guard:** each attempt captures generation and fixed receipt identity. After awaits, results mutate visible state only if scope, generation, and receipt opportunity/snapshot still match. Pending marker cleanup is conditional on the same attempt token, so A cannot clear B's active attempt.

## Limits and risks

- No acceptance gap found in the assigned race fix.
- Full Web suite remains unverified on this code SHA because its preceding run stalled in unrelated agent-editor work and was intentionally not repeated. Focused Career tests, typecheck, and production build pass on this exact SHA.
- No live authenticated browser/API run was performed; verification used the focused DOM tests and code inspection.
