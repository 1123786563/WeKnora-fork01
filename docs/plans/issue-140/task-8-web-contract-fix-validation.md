# T08 Web Contract Fix Validation

- Result: **DONE**
- Revision: `9fbf9f5ce73eada47fc357e01da1b4d08dfd5ba3` (`HEAD` at validation)
- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-t08-web/WeKnora-fork01`
- Source and tests were read-only during validation; only this report was created.

## Commands and results

| Command | Result |
| --- | --- |
| `pnpm exec tsx --test packages/career-core/src/contracts.test.ts packages/api-client/src/career.test.ts` | PASS, 9 tests, 0 failed |
| `pnpm typecheck:web` | PASS, exit 0 |
| `git diff --check 9fbf9f5^ 9fbf9f5` | PASS |
| `git diff --name-status 9fbf9f5^ 9fbf9f5` | Only `packages/career-core/src/contracts.ts` and `contracts.test.ts` changed |
| `git status --short` before report | Clean |

## Acceptance evidence

- The evidence decoder rejects blank and whitespace-only `rawText`, then returns a valid raw text fixture with leading newline, indentation, CRLF, tab, trailing spaces, and final newline byte-for-byte/string-for-string unchanged. It does not trim or normalize the accepted value.
- The receipt decoder returns an offset timestamp exactly as received (`2026-09-24T09:02:03+08:00`). The date validator rejects impossible 30 February input in both receipt and evidence fixtures.
- The date validation checks month/day including leap years, clock fields, and offset fields before accepting `Date.parse`; the original timestamp string is preserved in the decoded contract.
- Focused tests and Web typecheck pass on the assigned revision.

No acceptance gap found within this decoder fix. Browser/UI states and accessibility are outside the assigned contract-fix brief.
