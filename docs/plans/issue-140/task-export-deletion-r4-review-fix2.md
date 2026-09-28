# Export/deletion r4 review fix 2 — implementation report

## Scope

Addressed findings from `task-export-deletion-r4-review.md` in `apps/web/src/career/ExportDeletionPage.tsx` and its focused test:

- **EXDEL-R4-01:** after a trusted `deleting` receipt, a mismatched lookup is rejected while the original request ID remains available for another explicit lookup. The `deleting` state continues to disable a new deletion. The sequence test verifies the write ID and both lookup IDs are identical, rejects the alien receipt, and then accepts the terminal receipt for the original ID.
- **EXDEL-R4-02:** revision reads and current-scope recovery share an operation sequence. Starting recovery supersedes older automatic reads; stale success and error responses are ignored. Recovery's `finally` clears its loading state only if that recovery still owns the operation sequence and scope. The deferred ordering test resolves recovery at revision 9 before the older automatic read at revision 8, then verifies the page and export request both retain revision 9.

## TDD and verification evidence

- RED: `cd apps/web && node --import tsx --test src/career/ExportDeletionPage.test.tsx` failed the two new sequence tests before the implementation changes. The deletion test showed no reconciliation action after the mismatched lookup; the revision test showed revision 8 overwriting recovered revision 9.
- GREEN: `cd apps/web && node --import tsx --test src/career/ExportDeletionPage.test.tsx` — **14/14 passed**. The existing jsdom navigation warning (`Not implemented: navigation to another Document`) remains non-fatal.
- Typecheck: `pnpm typecheck:web` — **passed**, exit 0.
- Whitespace: `cd apps/web && git diff --check -- src/career/ExportDeletionPage.tsx src/career/ExportDeletionPage.test.tsx` — **passed**, exit 0.

## Commit and limits

- Implementation and tests: `a511c89603ba282e46dc8b249a630bcaf0526eee` (`fix career export deletion recovery races`).
- No unresolved finding from this task was observed in the focused scenarios. This report does not claim a full app/browser end-to-end run.
