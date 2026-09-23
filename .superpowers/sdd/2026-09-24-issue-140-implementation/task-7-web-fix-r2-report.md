# T07 Web Review Fix R2 Report

## Scope

Fixed findings `T07-WEB-R1-F1` and `T07-WEB-R1-F2` from code BASE `1b7f095a87780207956786ffd4d3d5cfabb1c42f`. Changed only `apps/web/src/career/CareerPage.tsx` and `apps/web/src/career/CareerPage.test.tsx`.

## Changes

- Split upload POST handling from follow-up source/profile reads. Only after the POST rejects can its error set the attempt to unknown. Later refresh failures now show a read error while retaining the known source, batch receipt, proposals, and success notice.
- `invalid_request`, `idempotency_conflict`, and `revision_conflict` are terminal for the current claim. They clear the retained file/attempt and enable a deliberate fresh selection/upload. The picker value is cleared so the same local file can be selected again.
- Added a visible “刷新来源” action after success so the user can retry a failed source-list read.
- Preserved R1 exact replay identity behavior, same-name safeguards, and forbidden/scope async epoch fencing. New tests keep existing ambiguity and forbidden coverage in the focused suite.

## TDD and verification

RED evidence: before the change, tests for `invalid_request` and known `ready` upload followed by source refresh failure both failed. The first was surfaced as “upload outcome unknown” and retained/locked the attempt; the second replaced the successful receipt notice with the unknown-outcome notice.

GREEN evidence:

- `pnpm exec tsx --test --test-name-pattern='definitive upload|known ready upload' apps/web/src/career/CareerPage.test.tsx` — passed after implementation, 2 tests, 0 failures.
- `pnpm exec tsx --test packages/career-core/src/contracts.test.ts packages/api-client/src/career.test.ts apps/web/src/career/CareerPage.test.tsx` — passed, 18 tests, 0 failures.
- `pnpm typecheck:web` — passed.
- `pnpm test:web` — passed, 2,319 tests, 0 failures.
- `pnpm build:web` — passed. Existing CSS `@import` ordering, `calc()` spacing, and large chunk warnings were emitted.
- `git diff --check` — passed.

## Handoff and limits

No API/backend contract change was required. Live server/browser acceptance and independent R2 validation/review remain with root. No push, merge, or deploy performed.
