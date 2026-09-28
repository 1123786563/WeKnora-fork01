# Independent review — T26 OCR round-4 review repair 2

## Scope and evidence

Reviewed commit `cf19364ff3fa2c20f641745fcd6c5eb85acac58c` against its parent and the prior F1–F3 findings in `task-26-ocr-r4-review-fix-review.md`. Read the approved job-search spec, `CONTEXT.md`, ADR-0017, and the assigned repair report. This review did not modify source or invoke OCR. `git diff --check cf19364ff^ cf19364ff` passed. From `apps/miniprogram`, `node --experimental-strip-types --test tests/export-deletion.test.mjs` passed (31 tests, 0 failures). These are service/controller tests, not rendered Mini Program interaction evidence.

## Findings

### F2 remains — Medium — export recovery can still be abandoned during a separate in-flight retry or reconcile

**Evidence / affected symbols:** `apps/miniprogram/src/services/career.ts:574-582` routes export recovery through `reconcileIntent` and `retryIntent`, which do not call `withActiveRecovery`. `abandonPendingSpaceExport` calls `abandonRecoverable` (`:264`), whose new active-ID check in `career-intent.ts:154-162` therefore always sees the export ID as idle. The export page's `isBusy` callback only reads its own `exportOperationInFlight.current` (`export-deletion.tsx:153-157`). A retry from another mounted page or service caller can be outstanding while this page confirms abandonment. An ambiguous retry in `retryRecoverable` (`career-intent.ts:140-147`) does not recreate an intent after that removal.

**Impact:** The export's original request ID can be lost even though the retry result remains unknown. This is the export half of the prior F2 race, and conflicts with the spec's failure-recovery requirement and the existing original-ID recovery contract.

**Smallest defensible correction:** Wrap export reconcile and retry at the shared service seam with active tracking using their captured scope, kind `spaceExport`, and request ID; make the page's export busy check use the shared state. Add a deferred export retry/reconcile plus separate abandon test, including an ambiguous outcome that must leave the ID recoverable.

### F3 remains — Medium — page transition behavior is still not exercised through its handlers

**Evidence / affected tests:** The new `createDeletionPageController` is used in `export-deletion.tsx:55,78,115,129,138,191,324`, so the state seam is genuinely connected. The new test at `tests/export-deletion.test.mjs:603-620` calls that controller and `lifecycleGating` directly. The only check that the page invokes the initial unknown handler is a source-text assertion at `:588-601`. Neither test runs the page's `onTap`/`useAction` path, confirmation modal, asynchronous catch-to-render transition, or recovery button disabled state.

**Impact:** A page wiring or render-state regression can still pass all focused tests, including a failure to show the disabled main action after an ambiguous deletion. The repair improves controller coverage but does not satisfy the prior finding's observable page-transition coverage.

**Smallest defensible correction:** Add a rendered Taro/React page interaction test or extract the actual page action handlers into a controller used by the page. Exercise initial ambiguous deletion, cancel, changed ID, concurrent recovery, successful abandon, and the resulting disabled/enabled actions through those handlers.

## Disposition of prior findings

- **F1 closed for the in-process deletion race:** `deleteWholeSpace` checks the scope's `starting` activity then registers it synchronously before its first transport await (`career.ts:599-607`, `career-intent.ts:34-44`). JavaScript execution cannot interleave a second call between those synchronous steps. `.finally(release)` and the synchronous-throw catch release the reservation on success and error; partial/ambiguous results retain the original intent. The deferred-POST test at `export-deletion.test.mjs:246-266` verifies one POST and a recoverable ID. This is process-local protection; it does not claim coordination across independent runtimes.
- **F2 closed for deletion recovery only:** `reconcilePendingSpaceDeletion` and `retryPendingSpaceDeletion` track scope, kind, and original ID for their entire promise lifetime (`career.ts:626-653`); `abandonRecoverable` checks activity immediately before synchronous removal (`career-intent.ts:154-162`). The deferred retry test at `export-deletion.test.mjs:646-679` verifies refusal and intent retention after an ambiguous response. The export path remains open as above.
- **F3 improved but open:** the page uses the tested controller; direct controller tests and source matching do not demonstrate the rendered user transition.

## Verdict

- **Spec compliance: not approved.** The deletion race is addressed, but the export recovery ID can still be discarded during an active operation. No new violation of ADR-0017's immutable evidence rule or cross-scope disclosure was found in this commit.
- **Code quality: not approved.** Focused tests and diff check pass, but the shared activity mechanism is applied unevenly and page-handler behavior remains unverified. No build, device, or full-suite result was independently rerun for this review.
