# Independent review — T26 mini-program OCR round-4 repair

## Scope and evidence

- Reviewed commit `bc19b9fd36d473ce3e3afd8a0c11f0067634a2af` against parent `76df0cee0bf3ae23c14411c345b151ad518077ee` in `/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01`. The shared worktree had unrelated later uncommitted changes; conclusions here use the named commit range and the two mini-program page files at that commit.
- Fact sources: approved `docs/specs/2026-09-23-weknora-job-search-design.md` (explicit continuation for hard ineligibility; truthful unknown outcomes and deletion boundaries), `CONTEXT.md` career terms, `docs/adr/0017-immutable-job-and-application-evidence.md`, T26/T32 draft tickets, `task-26-ocr-r4-fix.md`, and OCR `ocr-round-4.md`, `round4-highrisk-analysis.md`, `round4-resume-increment-analysis.md`, `round4-resume2-increment-analysis.md`.
- Commands: `git diff --stat 76df0cee0..bc19b9fd3`, `git diff 76df0cee0..bc19b9fd3 -- apps/miniprogram/src/career apps/miniprogram/tests`, `git diff --check 76df0cee0..bc19b9fd3` (pass), and `node --experimental-strip-types --test tests/application-material.test.mjs tests/export-deletion.test.mjs` from `apps/miniprogram` (60 passed, 0 failed). No OCR invocation and no source edits.

## Findings

### R1 — Medium — abandonment can erase a recovery intent during an in-flight operation

**Evidence and affected symbols:** `apps/miniprogram/src/career/export-deletion.tsx:123-137,149-163` disables the new abandon actions based on the current `useAction().busy` render, but `abandonExportRecovery` and `abandonDeletionRecovery` do not recheck activity or the pending request ID after `await confirmAction(...)`. `career.abandonPendingSpaceExport/Deletion` removes whichever current-scope intent exists at call time (`apps/miniprogram/src/services/career.ts:264-265`; `career-intent.ts:132-134`). A pending request can settle, be retried, or be replaced while the confirmation promise is outstanding; a stale confirmation then clears the current intent. In particular, `retryPendingSpaceDeletion` does not restore an intent after an ambiguous retry, so clearing it during that request can permanently lose its only recovery ID. The export abandon control also omits `exportBusy.busy`, unlike the deletion counterpart's `deletionBusy.busy`.

**Impact:** The UI can say recovery was intentionally abandoned for the displayed request while actually discarding a later/current request's recovery record. An unknown export or deletion may then lose its original-ID reconciliation path, contrary to the documented unknown-outcome contract. Native modal behavior reduces the ordinary same-page tap window, but does not make the asynchronous handler atomic with service completion or other mounted actions.

**Smallest correction:** Capture the displayed pending request ID before opening the modal; after confirmation, re-read the current intent and clear only if its ID still matches and all relevant operations are idle. Guard against concurrent confirmation taps with a dedicated busy/ref state; include `exportBusy.busy` in the export abandon disabled condition. Prefer a service operation that conditionally removes the captured ID to make the check and remove atomic at the storage seam.

### R2 — Medium — behavioral tests do not exercise the new page transitions

**Evidence and affected tests:** `apps/miniprogram/tests/application-material.test.mjs` uses two source regexes to check the button expression and request flag. `apps/miniprogram/tests/export-deletion.test.mjs` similarly checks source strings for setters, confirmation count, and disabled props. Its N3 service scenario computes `unresolved = uncertain.code === 'outcome_unknown'` itself before calling the pure gate; it never runs the page catch handler or confirms that `setDelRecoveryUnresolved(true)` reaches the rendered gate. The tests pass even if the abandon handler clears a different pending request or races with retry; none calls `confirmAction` in a rendered page.

**Impact:** The reported 60/60 verifies service and pure-gate behavior plus source spelling, but does not validate the new asynchronous UI transitions or confirmation safety. This limits the evidence for the N2/N3 repairs; a refactor can also make the source tests fail despite preserved behavior.

**Smallest correction:** Add a page-level rendered interaction test (or extract a small testable controller for pending-ID conditional abandon and unknown-delete transition) with deferred confirm/retry promises. Assert no clear on cancel or changed ID, no clear during in-flight recovery, and disabled deletion after an ambiguous fresh attempt with an old partial receipt.

## Targeted spec and quality verdict

- **Hard-ineligible application:** The changed button gate and `continueDespiteHardFailure: ineligible && acknowledged` now reflect explicit continuation. Existing server tests cover the typed refusal and explicit continuation. **Spec compliant for the reviewed change.** The new UI test is only a source assertion (R2).
- **Unknown deletion after stale partial:** The catch now sets `delRecoveryUnresolved(true)`, which feeds `deletionOutcomeUnknown` and disables another deletion while the new intent remains pending. **Conditionally compliant for the reviewed page path.** The fresh-request service still has the separately reported OCR N1 weakness: `apps/miniprogram/src/services/career.ts:598-615` can start a new request while an earlier partial intent exists. That pre-existing service defect is outside this commit but must be resolved before an overall T32 integrity approval; this commit's N3 test demonstrates it by issuing a second request after partial.
- **Explicit abandonment:** Both recovery blocks expose confirmation, explain local-only clearing, and update local state. **Not yet quality-approved** because the clear is not tied to the confirmed request ID or checked after the async confirmation (R1).
- **Security, architecture, data consistency:** No new cross-scope data exposure or ADR violation was found in the diff. R1 is a recovery-ID integrity risk. The source-assertion test gap in R2 warrants a behavioral check. The recorded mini-program typecheck remains blocked by unrelated account-page `CommercialSummary` errors; this review did not rerun typecheck or device verification.

**Overall:** The application gate and stale-partial lock fix the specified immediate paths, but the abandonment race and missing behavioral coverage prevent a clean code-quality verdict for this repair. The separate N1 service guard remains necessary for end-to-end deletion safety.
