# Export/deletion r4 fix — independent Spec and quality review

- Reviewed source range: `f35ed2427..1a4acb662bb6e0484c6c86c2c230b1e3f35ea885`, specifically `apps/web/src/career/ExportDeletionPage.tsx` and its test. The later `7f40e44e7` documentation commit and subsequent HEAD changes are excluded from the source verdict.
- Fact sources: approved `docs/specs/2026-09-23-weknora-job-search-design.md` §§2, 6–8; `CONTEXT.md` Career terms; ADR-0018; issue #140 acceptance snapshot; `task-career-web-ocr-r4-review.md`; `task-export-deletion-r4-review-fix.md`.
- Independent checks: `git diff --check` on the two Web files passed. The focused `node --import tsx --test src/career/ExportDeletionPage.test.tsx` run passed 12/12 against the same two source files (they have no later changes); jsdom emitted its existing navigation warning. No OCR was invoked. The implementation report records a full Web typecheck failure in concurrent `RulePage.tsx`, so this review does not claim full typecheck success.

## Findings

### EXDEL-R4-01 — Medium — a mismatched lookup strands an in-progress deletion

**Evidence:** `acceptDeletion` retains the original attempt and `deletion.status='deleting'` (`ExportDeletionPage.tsx:237-253`). A later receipt lookup that returns another request ID throws `ReceiptMismatchError`; its catch clears `deletionAttempt` but leaves the trusted `deletion` value at `deleting` (`:293-306`). The main deletion remains disabled for `deleting` (`:314`), while the query control requires `deletionAttempt` (`:365`). The new mismatch test (`ExportDeletionPage.test.tsx:345-360`) starts from an unknown write, so it does not cover this sequence.

**Impact:** Once an actual deletion has returned `deleting`, one malformed lookup leaves the page with no action to observe its terminal status. The user cannot determine whether the deletion completed, contradicting the recovery lifecycle required by issue #140 and the prior CWEB-R4-02 correction. The alien receipt is correctly rejected, but recovery of the original request is lost.

**Smallest defensible correction:** Keep the original request ID in a quarantined recovery state for this specific `deleting` case and expose an explicit requery of that original ID while continuing to reject/display no alien receipt; never enable a new deletion request during this state. Add a test for `deleting` → mismatched lookup → original-ID requery → terminal receipt.

### EXDEL-R4-02 — Medium — an older same-scope revision read can overwrite successful recovery

**Evidence:** On scope generation change, `readRevision` starts an automatic `open` (`ExportDeletionPage.tsx:87-99`). The recovery button starts a second `open` and boundary read (`:212-224`). Each completion checks only `scopeController.isCurrent`, which compares scope generation, not request order. If the recovery read returns revision 9 and opens the page, then the earlier automatic read returns revision 8 from the same scope, `readRevision` overwrites the displayed revision with 8 (`:91-93`). The new scope test (`ExportDeletionPage.test.tsx:286-306`) gives both new-scope reads revision 9, so it cannot detect the ordering fault.

**Impact:** The user can submit a newly recovered export or deletion with an older `expectedRevision`; the server CAS should reject it, but the page presents the wrong current revision and forces an avoidable conflict. A stale response can also change the revision display after successful recovery.

**Smallest defensible correction:** Give revision reads an operation sequence or cancel/supersede the automatic read when recovery begins, and accept only the newest same-scope read. Add a deferred-response test where the automatic read resolves after recovery with an older revision; the page and subsequent export must retain the recovered revision.

## Verified corrections and limits

- **CWEB-R4-01:** The `scope-changed` branch now has a recovery action. It reads the current revision and deletion boundary, guards both results by scope, stays locked on failure, and remains fail-closed on `forbidden`. The new test proves a scope switch can resume an export with revision 9.
- **CWEB-R4-02:** A `deleting` receipt now exposes lookup under the original request ID; a new deletion is disabled while that status is shown. The test proves `deleting` → `deleted` using the same ID.
- **CWEB-R4-03:** Export and deletion lookup catches classify `ReceiptMismatchError` deterministically, clear attempts, and remove transient retry controls. Both alien-ID tests pass. EXDEL-R4-01 covers a distinct combination with an already accepted `deleting` receipt.
- The new recovery and receipt paths fence successful results and caught failures to the current scope. The `recoverCurrentScope` `finally` resets its loading flag without a scope check (`:232-234`); a late old-scope completion could clear a newer recovery spinner and allow duplicate clicks. This is a lower-impact concurrency gap; guard that cleanup with the same operation identity when addressing EXDEL-R4-02.

## Independent verdict

- **Spec compliance: FAIL for this checkpoint.** The three assigned scenarios are implemented, but EXDEL-R4-01 leaves a reachable in-progress deletion with no usable reconciliation action.
- **Code quality: FAIL.** EXDEL-R4-02 is a same-scope async ordering regression, and the focused tests omit both adverse interleavings. No authorization or cross-scope data exposure was found in the reviewed changes.
