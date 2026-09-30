# T10 Task 4 Web Review Fix R1 Report

- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-t10-ui/WeKnora-fork01`
- Base: `5061c90d16da1008f7b3d4ae0dab4d880e25da54`
- Plan: `/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01/docs/plans/2026-09-24-issue-140-t10-ui-review-fixes.md`
- Scope: stable fixed-snapshot re-evaluation and immediate tri-state conversation result warning; no contract/backend changes.

## Delivered

- Added a shared stable-page action to re-evaluate the current confirmed profile against the fixed Opportunity/snapshot IDs from either saved Opportunity evidence or a historical Evaluation detail page. Each new evaluation gets a fresh request ID and its own immutable result link; old evaluation revision and conclusion remain visible.
- The stable action prevents duplicate in-flight clicks, retries uncertain writes under the original request ID, reconciles through the evaluation receipt endpoint, and clears evaluation details on scope/logout/403.
- The transient saved-JD conversation card displays the receipt's qualification status immediately. `ineligible` is an alert styled before links/actions; `unknown` remains distinct and never appears eligible.
- Added responsive local status/action styles.

## TDD and verification

- RED: `pnpm --filter @weknora/web exec node --import tsx --test src/career/OpportunityPage.test.tsx` — 4 new assertions failed as expected: missing immediate verdict, no evaluation action on saved evidence, and no re-evaluate action on historical detail. The unknown result card assertion also failed because it had no status presentation.
- Focused GREEN: `pnpm --filter @weknora/web exec node --import tsx --test src/career/OpportunityPage.test.tsx src/routes.test.ts src/router.test.tsx` — 43 passed, 0 failed.
- `pnpm typecheck:web` — passed.
- `pnpm test:web` — 2,341 passed, 0 failed, 0 cancelled.
- `pnpm build:web` — passed (`✓ built in 16.22s`). Existing CSS `@import` ordering, `calc()` spacing, and large-chunk warnings remain.
- `git diff --check` — passed.
- No live authenticated API/browser run was performed; the controller/validator retains that acceptance step.

## Changed files

- `apps/web/src/career/OpportunityPage.tsx`
- `apps/web/src/career/OpportunityPage.test.tsx`
- `apps/web/src/career/opportunity.css`

- Implementation commit: `d2ab0698c`.
