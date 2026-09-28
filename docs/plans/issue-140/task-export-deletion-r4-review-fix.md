# Export and deletion OCR r4 review fixes

- Assigned findings: `CWEB-R4-01`, `CWEB-R4-02`, `CWEB-R4-03` from `task-career-web-ocr-r4-review.md`.
- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01`.
- Starting BASE: `bc19b9fd36d473ce3e3afd8a0c11f0067634a2af` (verified with `git rev-parse HEAD`). This is the allowed integrated state after `f35ed2427`.
- Implementation commit: `1a4acb662bb6e0484c6c86c2c230b1e3f35ea885` (`codex: fix export deletion OCR r4 recovery`); the final report update follows in a documentation-only commit.
- Scope: `ExportDeletionPage.tsx`, its focused test, and this report only. Concurrent edits in other files were preserved.

## Changes and evidence

- `CWEB-R4-01`: the scope-changed alert now offers “重新加载当前空间”. Recovery concurrently re-reads the current scope revision and deletion boundary, and restores the usable page only when both succeed and the scope remains current. A non-forbidden failure keeps the page locked with retry guidance; forbidden remains fail-closed. The scope-switch test verifies old private content is cleared, recovery loads revision 9 and a fresh boundary, and export uses `expectedRevision: 9`.
- `CWEB-R4-02`: a retained `deleting` receipt exposes receipt lookup for the original attempt. Starting a new deletion is disabled while deletion is in progress, and the same-request write retry is only offered for an unknown outcome. The test verifies the initial and lookup request IDs match and the lookup reaches `deleted`.
- `CWEB-R4-03`: export and deletion receipt lookup now classify `ReceiptMismatchError` as a deterministic error, clear the relevant attempt, and show the mismatch message. Focused tests verify neither lookup nor same-ID retry controls remain after an alien request ID is returned.

## Commands and results

- RED: `node --import tsx --test src/career/ExportDeletionPage.test.tsx` from `apps/web` — expected failures for the four new behavior tests before implementation; the existing eight tests passed.
- GREEN/final targeted run: `node --import tsx --test src/career/ExportDeletionPage.test.tsx` from `apps/web` — 12 tests passed, 0 failed. The existing jsdom warning `Not implemented: navigation to another Document` is emitted by the download test; the test passes.
- `pnpm typecheck:web` — failed on a concurrent unrelated edit: `apps/web/src/career/RulePage.tsx:229:59`, TS2345 (`"error"` is not assignable to `SetStateAction<Phase>`). No RulePage changes were made.
- `git diff --check -- apps/web/src/career/ExportDeletionPage.tsx apps/web/src/career/ExportDeletionPage.test.tsx` — passed.

## Remaining limits

- The full Web typecheck remains blocked by the concurrent RulePage type error. No browser automation beyond the focused jsdom behavior tests was run.
