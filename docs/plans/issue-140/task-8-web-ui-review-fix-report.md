# T08 Web UI review fixes

## Review findings addressed

1. A completed import previously left “保存 JD” active; activating it again allocated another request ID and could create a duplicate Opportunity. The saved state now locks JD and metadata fields, disables the saved action, and requires an explicit “开始新草稿” action to clear the saved form/result before another request can be submitted. Unknown-outcome receipt lookup and same-request/same-input retry are unchanged.
2. `opportunity.css` included 466 lines of copied global styles after the local Opportunity media block. The copied tail was removed; the stylesheet now contains only 71 lines of Opportunity-specific rules. Existing app global CSS imports were not changed.

## RED → GREEN evidence

Added a test asserting that after a successful save the save action is disabled and a second activation does not increase POST count. It also starts an explicit new draft, submits new text, and verifies a distinct request ID. Before the UI change this test failed because “已保存” did not exist and the original save button remained active. It passes after the fix.

## Changed files

- `apps/web/src/career/OpportunityPage.tsx`
- `apps/web/src/career/OpportunityPage.test.tsx`
- `apps/web/src/career/opportunity.css`

## Verification

- `pnpm --filter @weknora/web exec tsx --test src/career/OpportunityPage.test.tsx` — PASS, 9 tests.
- `pnpm typecheck:web` — PASS.
- Focused integration tests: `pnpm --filter @weknora/web exec tsx --test src/career/OpportunityPage.test.tsx src/routes.test.ts src/router.test.tsx src/chat/chat-route-page-send.test.ts` — PASS, 36 tests.
- `pnpm test:web` — PASS, 2329 tests, 0 failures.
- `pnpm build:web` — PASS. Existing CSS import-order, `calc()` and bundle-size warnings remain.
- `git diff --check` — PASS.
- CSS scope check: `wc -l apps/web/src/career/opportunity.css` reports 71; no global body/root selector remains.

## Commit

SHA recorded after committing the three owned code files.
- `3fd9c19369c45cef5be104a11f67f1cce02165eb` — `fix(web): prevent duplicate opportunity imports`
