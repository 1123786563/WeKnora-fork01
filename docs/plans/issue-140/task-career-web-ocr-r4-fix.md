# Career Web OCR r4 high fixes

- Task base: `76df0cee0bf3ae23c14411c345b151ad518077ee` (worktree `/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01`).
- Assigned scope: ProgressPage mismatch receipt recovery; SearchPage mismatched receipt recovery; ExportDeletionPage lifecycle scope reset; api-client open/list/changes response decoding.
- Ownership: only the three assigned web pages and their tests, `packages/api-client/src/career.ts` and its test, and this report.
- Commit: authorized by parent; intended message `codex: fix career web OCR r4 high`.

## Changes

- ProgressPage now recognizes `ReceiptMismatchError` during receipt lookup, clears the attempt and exits recovery into an explicit error alert.
- SearchPage now treats a stored receipt with a different request ID as `invalid_response`, clears the attempt and returns to idle so the user can start a new search.
- ExportDeletionPage clears revision, revision state, verification message, prior export request and deletion announcement flag on private-state clearing. The `readRevision` callback now tracks scope generation, causing a fresh read after a scope change.
- Career API `open`, `list` and `changes` now pass responses through `decodeCareerView` / `decodeCareerChangeSet`.
- Added regression tests for each behavior. New tests were run RED before implementation: 4 failures reproduced the original behavior (lifecycle reread, both mismatch recovery paths, and malformed client responses).

## Verification

| Command | Result |
|---|---|
| `node --import tsx --test apps/web/src/career/ProgressPage.test.tsx apps/web/src/career/SearchPage.test.tsx apps/web/src/career/ExportDeletionPage.test.tsx packages/api-client/src/career.test.ts` | Exit 0; 87 tests passed, 0 failed. Includes the four added regression tests. |
| `pnpm typecheck:web` | Exit 0; `@weknora/web` TypeScript check passed. |
| `git diff --check` | Exit 0; no whitespace errors. |

The test process prints jsdom's existing `Not implemented: navigation to another Document` notice during lifecycle coverage; the process exits successfully and all tests pass.

## Review concerns and limits

- No independent code review has been run for this repair yet; parent owns the SDD/OCR review stages.
- The scope-switch regression asserts a fresh `open` call and that prior export/deletion/verification UI is cleared. The component deliberately shows its scope-changed notice after abort, so the newly read revision is not displayed until the page is re-entered.
- No visual or real-browser layout change was made. The affected behavior is request recovery, lifecycle state reset, and payload validation.

## Checkpoint

- Source changes were made against base `76df0cee0bf3ae23c14411c345b151ad518077ee`.
- Implementation checkpoint HEAD / commit: `b1b84a411` (`codex: fix career web OCR r4 high`), based on `76df0cee0bf3ae23c14411c345b151ad518077ee`.
- No unrelated shared-worktree changes were included in the task commit.
