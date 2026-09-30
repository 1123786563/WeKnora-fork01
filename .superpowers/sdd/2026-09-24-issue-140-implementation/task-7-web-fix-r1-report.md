# T07 Web Review Fix R1 Report

## Scope

Fixed review findings `T07-WEB-R0-F1` and `T07-WEB-R0-F2` on code BASE `a9e4c69844adfc109cbc62d84f808102ca5a852e`. Changed only `apps/web/src/career/CareerPage.tsx` and `apps/web/src/career/CareerPage.test.tsx`.

## Changes

- Removed filename-based resolution of unknown uploads. `/sources` polling refreshes visible history and explicitly states it has no request ID, so it cannot identify the active claim. Recovery only ends through exact replay with the retained `File`, request ID, and expected revision. Replay response is authoritative: processing keeps the tuple, terminal ready/failed clears it.
- Forbidden outcomes increment the page async epoch, clear page and CareerDesk private state, and clear file/recovery data. Source loading, upload, scope recovery, and existing action/recovery continuations check epoch after awaits before state writes. An explicit fresh read reactivates the current scope and reloads both view and source history.
- Added same-name old ready/failed/processing cases, asserting source polling cannot settle the unknown attempt and exact replay reuses the same File object and request/revision tuple.
- Added forbidden source-read tests while upload is pending and during recovery; late continuations must not write private state, upload notices, or start follow-up source reads.

## TDD and verification

RED evidence: running the new CareerPage tests against committed BASE `a9e4c6984` failed the same-name identity assertions. The late-upload test with a deferred follow-up source read failed because BASE started a second source request after authorization loss (`sourceCalls` was 2 instead of 1). The recovery status assertion also failed on the old processing-only filename inference.

GREEN evidence:

- `pnpm typecheck:web` — passed.
- `pnpm exec tsx --test packages/career-core/src/contracts.test.ts packages/api-client/src/career.test.ts apps/web/src/career/CareerPage.test.tsx` — passed, 16 tests, 0 failures.
- `pnpm test:web` — passed, 2,317 tests, 0 failures.
- `pnpm build:web` — passed. Existing CSS `@import` ordering, `calc()` spacing, and large chunk warnings were emitted.
- `git diff --check` — passed.

## Handoff and limitations

The backend wire contract remains unchanged; public sources intentionally do not expose request IDs, so exact replay is the only authoritative outcome check. Live server/browser acceptance and independent round 1 re-review remain with root. No push, merge, or deploy performed.
