# Task 2 independent Spec and code review — Career Web OCR r4

- Reviewed range: `76df0cee0bf3ae23c14411c345b151ad518077ee..f35ed2427024aae214db4aa42ecb61fd541d1c21` (reviewed commit `f35ed2427024aae214db4aa42ecb61fd541d1c21`). The branch working tree has unrelated concurrent edits; all source evidence below was read with `git show f35ed242:<path>` or the locked range.
- Review commands: `git diff --stat 76df0cee0..f35ed2427024aae214db4aa42ecb61fd541d1c21`; `git diff 76df0cee0..f35ed2427024aae214db4aa42ecb61fd541d1c21 -- <nine changed paths>`; `git show f35ed242:<relevant path> | nl -ba`; `git diff --check 76df0cee0..f35ed2427024aae214db4aa42ecb61fd541d1c21` (exit 0). No OCR was run and no tests were rerun in this review.
- Fact sources: approved `docs/specs/2026-09-23-weknora-job-search-design.md` §§2, 3, 6–8; `CONTEXT.md` Career terms; ADR-0015–0018; `docs/plans/issue-140/2026-09-24-issue-140-dag.md` T22/#162; `docs/plans/issue-140/ocr/round4-highrisk-analysis.md`; assigned implementation report `docs/plans/issue-140/task-career-web-ocr-r4-fix.md`.

## Findings

### CWEB-R4-01 — High — scope switch leaves export/deletion page inaccessible

**Evidence:** `ExportDeletionPage.tsx:71-76` clears state and sets `boundaryState='scope-changed'` on the old scope's abort; `:85-97` now re-reads the new scope revision; but `:293-349` renders only the scope-changed alert and no action that can change `boundaryState` back to `ready` or `idle`. `loadBoundary()` is the available recovery path (`:192-207`), yet its button is inside the hidden branch (`:320`). The new test asserts only a second `open` call and cleared content; it asserts the scope-changed alert, but never re-enters the new scope's export/deletion flow.

**Impact:** After switching accounts or spaces while CareerPage stays mounted (`CareerPage.tsx:215`), the new revision may be read but the new authorized user cannot export or delete until remount/navigation. This fails the intended lifecycle recovery and the Spec's complete Web export/deletion flow. The scope generation dependency addresses a stale read, but does not restore access.

**Smallest correction:** Provide a recovery action in the `scope-changed` branch that re-reads the current revision and deletion boundary under the current scope, then transitions to a usable state only on success. Preserve the alert/fail-closed behavior for a true `forbidden` response. Add a test that switches scope, invokes the recovery action, and performs a new-scope export with its revision.

### CWEB-R4-02 — Medium — deleting receipt has no usable recovery action

**Evidence:** `ExportDeletionPage.tsx:209-225` accepts a matching receipt with `status='deleting'`, retains `deletionAttempt`, sets `deletionPhase='error'`, and tells the user to query the receipt. The query button is rendered only when `deletionPhase==='unknown'` (`:333-336`); the same-request retry button is rendered only for `status==='partial'` (`:337`). No action is exposed for `deleting`. The added lifecycle test uses a `deleted` receipt and does not cover `deleting`.

**Impact:** An asynchronous deletion can remain in progress while the UI offers no way to observe its terminal result or resume recovery; a later fresh deletion can mint a new request ID. The text promises a query the user cannot perform.

**Smallest correction:** Render `查询删除回执` whenever a `deleting` receipt and its original attempt exist, with a focused test that returns `deleting` then `deleted` for the same request ID.

### CWEB-R4-03 — Medium — same-file receipt mismatch still loops in export and deletion recovery

**Evidence:** `acceptExport` and `acceptDeletion` throw `ReceiptMismatchError` on an alien request ID (`ExportDeletionPage.tsx:118-120,209-210`). Their write paths classify this deterministically (`:154,258`), but `lookupExportReceipt` and `lookupDeletionReceipt` catch the same error and unconditionally set `unknown` after a forbidden check (`:170-176,274-280`). Their recovery buttons remain available and repeat the same query. No new tests cover a mismatched export/deletion lookup.

**Impact:** A protocol identity failure is reported as a transient unknown outcome and can cause an indefinite retry loop with a stale attempt. This is the same error-class inconsistency that this repair corrected in ProgressPage and SearchPage.

**Smallest correction:** In both lookup catches, handle `ReceiptMismatchError` before uncertain errors: clear the relevant attempt, set `error`, show a deterministic mismatch message, and test both paths. Do not accept or display the alien receipt.

## Positive evidence and limits

- **ProgressPage:** `lookupReceipt` now classifies `ReceiptMismatchError`, clears its attempt, and exits recovery (`:167`); the new test asserts both recovery controls disappear. This closes the assigned mismatch finding.
- **SearchPage:** a mismatched stored receipt now sets `invalid_response`, clears the attempt, and returns to idle (`:210`); the new test checks the message and controls. This closes the assigned mismatch finding. The pre-existing `importResult` request-ID check gap in the source analysis is outside this task's changed lines and remains for the separate medium repair.
- **API client:** `open`, `list`, and `changes` now call the existing `decodeCareerView`/`decodeCareerChangeSet` validators (`career.ts:1255-1257`); malformed view/change-set tests reject all three. The decoder throws `TypeError('invalid career ...')`, consistent with the existing contract. The test covers malformed top-level revision; nested shape validation is supplied by the shared decoder, not newly tested here.
- **Scope and CAS:** The changed Web request completion paths check `scopeController.isCurrent` before accepting results, and export/deletion requests still carry `expectedRevision`. No new authorization bypass or CAS removal was found in this diff. The `readRevision` generation dependency produces a fresh read after a rendered scope change; its result is hidden by CWEB-R4-01.
- Implementation report records 87 focused tests and `pnpm typecheck:web` passing. This review did not independently rerun them, and those passing results do not exercise CWEB-R4-01 through CWEB-R4-03.

## Independent verdict

- **Spec compliance: FAIL for this repair checkpoint.** The assigned ProgressPage/SearchPage/API decoding corrections are present, but the scope-switch export/deletion flow remains inaccessible (CWEB-R4-01), and deletion-in-progress recovery remains absent (CWEB-R4-02). These conflict with the approved Web lifecycle and failure-recovery expectations.
- **Code quality: FAIL.** CWEB-R4-03 leaves two same-file deterministic mismatch paths classified as transient unknown; focused behavioral tests omit all three remaining scenarios. No security/CAS regression was found in the changed lines.
