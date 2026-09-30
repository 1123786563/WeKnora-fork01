# T10 Task 4 Career Evaluation Web Report

- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-t10-ui/WeKnora-fork01`
- Base: `ed0a71919e1bdb0347e4fa4397db56bfa2578a9c`
- Brief: T10 plan Task 4 in `docs/plans/2026-09-24-issue-140-t10-evaluation.md`; approved #150 and Spec `docs/specs/2026-09-23-weknora-job-search-design.md`.
- Contract consumed: reviewed `EvaluationReceipt` / `Evaluation` and `client.career.evaluateOpportunity`, `evaluationReceipt`, `evaluation` in `packages/career-core/src/contracts.ts` and `packages/api-client/src/career.ts`.

## Delivered

- Added explicit evaluation from a saved JD with pinned opportunity and snapshot IDs, request receipt recovery, retry, and error states.
- Added a protected stable `/platform/career/evaluations/:evaluationId` detail route. The page prioritizes the three-valued hard qualification status, links cited JD excerpts and confirmed fact versions, shows unknown reasons and later soft matches, and renders the pinned raw JD as inert text.
- Kept prior evaluation links when the user re-evaluates the same JD against a changed profile, labeled by captured profile revision.
- Fenced private evaluation content on scope/logout changes; a scope switch clears the current result without refetching its old ID in the new scope.
- Added narrow layout wrapping and stacked spacing for evaluation citations and facts.

## Verification

- RED attempt: `pnpm --filter @weknora/web exec node --import tsx --test src/career/OpportunityPage.test.tsx` could not start because this isolated worktree had no dependencies (`ERR_MODULE_NOT_FOUND: tsx`). Then installed exactly from lockfile with `pnpm install --frozen-lockfile` (no lockfile changes); wrote the tests before production UI changes.
- Focused: `pnpm --filter @weknora/web exec node --import tsx --test src/career/OpportunityPage.test.tsx src/routes.test.ts src/router.test.tsx` — 40 passed, 0 failed.
- `pnpm typecheck:web` — passed after correcting test fixtures to the integrated `CareerConfirmation` contract.
- `pnpm test:web` — 2,336 passed, 0 failed, 0 cancelled.
- `pnpm build:web` — passed (`✓ built in 21.82s`). Existing warnings include stylesheet `@import` ordering, CSS `calc()` whitespace and large chunks; no build error.
- `git diff --check` — passed.
- Responsive evidence: DOM/CSS regression asserts the 640px media rule, narrow list padding, wrapping of citations, and pre-wrapped JD text. No live authenticated API/browser acceptance was run; that remains with the assigned validator/controller acceptance gate.

## Changed files

- `apps/web/src/career/OpportunityPage.tsx`
- `apps/web/src/career/OpportunityPage.test.tsx`
- `apps/web/src/career/opportunity.css`
- `apps/web/src/routes.tsx`
- `apps/web/src/routes.test.ts`
- `apps/web/src/router.tsx`
- `apps/web/src/router.test.tsx`
