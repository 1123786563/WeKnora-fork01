# Task 4 Web Retry and Scope Repair Report

## Scope

Implemented the UI retry and identity repair from `docs/plans/2026-09-24-issue-140-t10-ui-retry-scope-fix.md`.

- Timeout or otherwise uncertain evaluation writes retain their original request ID, query the receipt first, and offer a same-ID retry if the receipt is not found.
- Saved evidence evaluation actions and detail pages are keyed to the fixed opportunity/snapshot/evaluation identity.
- Evidence and detail pages suppress stale A data while B is loading; evaluation requests on the evidence page use only the currently matched IDs.
- Receipt acceptance verifies request ID as well as opportunity and snapshot IDs.

## Changed Files

- `apps/web/src/career/OpportunityPage.tsx`
- `apps/web/src/career/OpportunityPage.test.tsx`

## Verification

- RED confirmed: the two new `ApiError(TIMEOUT)` retry tests failed before implementation because the UI reported a definite evaluation failure instead of entering receipt reconciliation.
- `pnpm --filter @weknora/web exec node --import tsx --test src/career/OpportunityPage.test.tsx src/routes.test.ts src/router.test.tsx` — passed, 49 tests.
- `pnpm typecheck:web` — passed.
- `pnpm test:web` — passed, 2345 tests, 0 failures.
- `pnpm build:web` — passed. Existing warnings remain for CSS `@import` ordering, malformed `calc()` whitespace, and a large bundle chunk.
- `git diff --check` — passed.

No authenticated interactive browser session was available; navigation, retry, and scope behavior were verified in DOM-level UI tests.

## Commit

Implementation commit: `4262f948b45b8a947f054b0f3f8aba2cd82accd0`.
