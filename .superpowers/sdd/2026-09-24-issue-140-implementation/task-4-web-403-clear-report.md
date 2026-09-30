# Task 4 Web Transient 403 Clear Report

## Scope

Implemented the transient conversation-card 403 repair from `docs/plans/2026-09-24-issue-140-t10-ui-403-clear-fix.md`.

The shared `clearPrivate` path now clears the JD draft, source metadata, import attempt and receipt, evaluation receipt/history, evaluation request ID, and evaluation notices. Evaluation POST and receipt-lookup 403 branches use that path. Existing request-scope checks continue to reject late responses after a scope change.

## Changed Files

- `apps/web/src/career/OpportunityPage.tsx`
- `apps/web/src/career/OpportunityPage.test.tsx`

## Verification

- RED confirmed: new POST-403 and receipt-403 tests failed before the fix because the old JD text remained visible.
- `pnpm --filter @weknora/web exec node --import tsx --test src/career/OpportunityPage.test.tsx` — passed, 27 tests.
- `pnpm --filter @weknora/web exec node --import tsx --test src/platform/global-command-palette-live-search.test.tsx` — passed, 7 tests.
- `pnpm typecheck:web` — passed.
- `pnpm build:web` — passed with existing CSS `@import` ordering, `calc()` whitespace, and large-chunk warnings.
- `git diff --check` — passed.
- First `pnpm test:web` run completed with 2347/2348 passing. The one failure was the unrelated debounced-search timing assertion (`global-command-palette-live-search.test.tsx`); that file passed when run alone (7/7).
- A full-suite rerun was started but interrupted after more than 8 minutes without output while `src/agents/agent-editor.test.tsx` remained CPU-bound. Its output marked the remaining unstarted files as interrupted, so this rerun is not counted as passing.

No authenticated interactive browser session was available; the clearing and late-response behavior were verified in DOM-level tests.

## Commit

Implementation commit: pending.
