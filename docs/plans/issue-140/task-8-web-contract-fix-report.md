# T08 Web Opportunity contract review fix

## RED / GREEN

- Added tests for empty/whitespace-only evidence raw text, impossible `2026-02-30` acquisition timestamps, an accepted RFC3339 offset timestamp, and byte-for-byte preservation of multiline JD text with CRLF and leading/trailing whitespace.
- RED: `pnpm exec tsx --test packages/career-core/src/contracts.test.ts` failed both focused Opportunity decoder tests with “Missing expected exception” for February 30 and blank JD evidence.
- GREEN: decoder rejects raw text whose trimmed value is empty, without modifying valid raw text; timestamp validation now checks month/day including leap years, time and offset ranges, then confirms JavaScript can parse it. Valid timestamp strings are returned unchanged.

## Verification

- `pnpm exec tsx --test packages/career-core/src/contracts.test.ts packages/api-client/src/career.test.ts` — PASS, 9 tests, 0 failures.
- `pnpm typecheck:web` — PASS.
- `git diff --check` — PASS.

## Changed files

- `packages/career-core/src/contracts.ts`
- `packages/career-core/src/contracts.test.ts`

## Commit

Local code commit `9fbf9f5ce73eada47fc357e01da1b4d08dfd5ba3` (`fix(career): validate opportunity evidence dates`), based on `efd18d67feaac09d8ee6347df509f537ee0cf10e`.
