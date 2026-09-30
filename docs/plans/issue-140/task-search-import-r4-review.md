# Search URL import receipt identity: independent review

Reviewed commit `8c58a9a27d3dbd04aa6b41836a121c4f17669eb9` against the approved job-search spec, ADR-0015 and ADR-0017, `CONTEXT.md`, Issue #140 acceptance snapshot, and the SearchPage medium finding in `ocr/round4-highrisk-analysis.md:51`. Scope: `apps/web/src/career/SearchPage.tsx` and `SearchPage.test.tsx` only. Review is read-only; other worktree changes were not assessed.

## Spec-compliance verdict: Pass for the assigned repair

`importResult` now compares the URL-import receipt's `requestId` to the exact ID sent for that row before storing the receipt (`SearchPage.tsx:251-262`). On mismatch, it discards the foreign receipt, clears the retained attempt ID, leaves busy and uncertain states, and displays a row-level alert. The evidence link is rendered only from a stored receipt (`SearchPage.tsx:334`), so the mismatched response cannot be presented as a fixed job snapshot. This directly resolves the round 4 finding and preserves the spec's evidence provenance and request-identity boundaries. The existing uncertain transport path still retains its original request ID for retry (`SearchPage.tsx:267-270`).

## Code-quality verdict: Pass, no blocking findings

The new check is placed after the scope-current guard and before the only receipt-storage path. The state update clears any previous receipt for the row, and the existing `finally` releases the pending-import guard. The new test asserts a visible alert, absence of an evidence link and uncertain retry state, and exactly one import call. Existing success and uncertain retry fixtures now echo the submitted ID, so they exercise the intended contract.

No critical, high, or medium findings in the assigned diff.

### Low-severity test gap

- **Evidence:** `SearchPage.test.tsx:308-326` stops after the mismatch state. It does not click “导入为岗位证据” again to assert that the cleared attempt produces a fresh request ID and can accept a valid matching receipt.
- **Impact:** The recovery path claimed by the task report is supported by the current state expression but lacks a behavioral regression test.
- **Smallest correction:** Extend the mismatch test with a second, matching mock response; assert a different request ID is sent and the evidence link appears. This is non-blocking for the identity-mismatch repair.

## Independent verification

- `node --import tsx --test apps/web/src/career/SearchPage.test.tsx`: exit 0, 18 passed, 0 failed.
- `pnpm typecheck:web`: exit 0.
- `git diff --check 8c58a9a27^ 8c58a9a27 -- apps/web/src/career/SearchPage.tsx apps/web/src/career/SearchPage.test.tsx`: exit 0.

No browser or end-to-end test was run for this review; the focused component test covers the changed request/receipt transition.
