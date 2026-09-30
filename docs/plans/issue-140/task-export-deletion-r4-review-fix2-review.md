# Export/deletion r4 fix 2 — independent Spec and quality review

## Scope and sources

- Source under review: commit `a511c89603ba282e46dc8b249a630bcaf0526eee`, limited to `apps/web/src/career/ExportDeletionPage.tsx` and `ExportDeletionPage.test.tsx`. Commit `de0ab8ff8` only records the implementation report; it is not included in the source verdict. Later changes to these two files are absent at review time.
- Compared against `task-export-deletion-r4-review.md` findings EXDEL-R4-01/02, the approved `docs/specs/2026-09-23-weknora-job-search-design.md` §§2, 6–8, `CONTEXT.md` Career terms, ADR-0018, and issue #140/T22 acceptance mapping. Review is read-only for requirements and source; OCR was not invoked.

## Findings

No blocking finding in the reviewed source range.

## Prior finding disposition

- **EXDEL-R4-01 — resolved.** On an alien receipt after a trusted `deleting` response, the page rejects the alien receipt and retains the original `deletionAttempt` (`ExportDeletionPage.tsx:316–319`). The original-ID lookup remains visible while `deleting` prevents a new deletion (`:327`, `:378–381`). The new test exercises `deleting` → alien lookup → original-ID lookup → `deleted`, and asserts all three requests use the same ID (`ExportDeletionPage.test.tsx:362–388`).
- **EXDEL-R4-02 — resolved.** Automatic revision reads and explicit scope recovery share `revisionReadSequence` (`ExportDeletionPage.tsx:68`, `:89–100`, `:215–237`). Recovery supersedes an older same-scope read; both success and failure paths reject superseded results. Its loading cleanup also checks the same scope and operation. The deferred test resolves recovery at revision 9 before the older automatic read at revision 8 and confirms the next export submits `expectedRevision: 9` (`ExportDeletionPage.test.tsx:390–415`).

## Independent verification

- `cd apps/web && node --import tsx --test src/career/ExportDeletionPage.test.tsx`: **14/14 passed**, exit 0. The existing jsdom `Not implemented: navigation to another Document` warning occurred in the download test; it did not fail the suite.
- `pnpm typecheck:web`: **passed**, exit 0.
- `git diff --check a511c89603ba282e46dc8b249a630bcaf0526eee^ a511c89603ba282e46dc8b249a630bcaf0526eee -- apps/web/src/career/ExportDeletionPage.tsx apps/web/src/career/ExportDeletionPage.test.tsx`: **passed**, exit 0.

## Independent verdict

- **Spec compliance: PASS for this scoped checkpoint.** The deletion reconciliation path remains tied to the original request, and recovered revision state cannot be overwritten by the demonstrated older read. These satisfy the two assigned corrections without changing the approved export/deletion boundary.
- **Code quality: PASS for this scoped checkpoint.** The sequence guards cover stale success, stale failure, and recovery cleanup; the tests assert externally visible behavior and submitted revision/IDs. Full browser end-to-end behavior and the broader issue #140 delivery are outside this review and are not claimed here.
