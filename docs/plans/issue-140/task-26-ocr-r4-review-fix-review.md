# Independent review — T26 OCR round-4 review repair

## Scope and evidence

- Reviewed `443ed6982fa2d469f5328f9e9b70a989adf4ca75` against its parent, limited to the changed Mini Program source and tests. Read the preceding `task-26-ocr-r4-review.md`, approved job-search spec, `CONTEXT.md`, ADR 0017, and OCR round-4 N1/N2/N3/N8/N9 rulings. This report makes no source or issue changes and does not invoke OCR.
- `git diff --check 443ed698^ 443ed698` passed. `node --experimental-strip-types --test tests/export-deletion.test.mjs tests/progress-preparation.test.mjs` passed: 49 tests, 0 failures. These are focused automated checks, not a rendered Mini Program or device test.

## Findings

### F1 — High — the new deletion preflight does not reserve the recovery slot while a request is in flight

**Evidence / affected symbol:** `apps/miniprogram/src/services/career.ts:598-615` checks `readIntent(key)` once, then awaits the POST before writing an intent. Two concurrent calls can both observe no intent and send different request IDs. If each settles as partial or unknown, the later `store.write(key, …)` replaces the first ID. The new test at `tests/export-deletion.test.mjs:214-243` exercises only a second call *after* the first partial receipt, and the page's `deletionOperationInFlight` ref is local to one mounted page instance (`export-deletion.tsx:54-57,305-331`); neither serializes service callers or another page instance.

**Impact:** One unresolved deletion can lose its only original-ID recovery route, recreating the N1 failure under overlapping requests. This conflicts with the documented intent/idempotency rule and the requirement to retain a partial deletion's recovery identity.

**Smallest defensible correction:** Reserve the same-scope deletion slot synchronously before the first await, with a process-local in-flight guard or a stored pending intent. Reject another start until the first outcome is classified; retain its ID for ambiguous/partial outcomes and release the reservation on definite failure or `deleted`. Add a deferred-POST test that starts two calls before either response and verifies one transport call and one recoverable ID.

### F2 — Medium — confirmation safety is scoped to one page instance, while recovery writes have no matching in-flight guard

**Evidence / affected symbols:** The abandon handlers pass page-local `exportOperationInFlight.current` / `deletionOperationInFlight.current` to `confirmAbandonIntent` (`export-deletion.tsx:151-155,182-186`). `abandonRecoverable` checks only the expected request ID (`career-intent.ts:135-140`). A retry or reconcile from a second mounted page/service caller can therefore be in flight for that same ID while the first page confirms abandonment. The first page sees `isBusy() === false` and clears the intent. An ambiguous retry does not restore it (`career-intent.ts:119-129`, `career.ts:629-642`).

**Impact:** A confirmed abandonment can race an unrelated in-flight recovery and erase its only request ID. The request-ID CAS fixes the changed-ID case from R1, but it does not make the operation idle at the shared storage/service seam.

**Smallest defensible correction:** Track in-flight recovery by scope, kind, and request ID at the shared service seam; have conditional abandon refuse while that ID is active. Exercise confirmation while a separately initiated retry is deferred, then an ambiguous response, and assert the intent remains recoverable.

### F3 — Medium — prior page-transition coverage gap remains

**Evidence / affected tests:** `tests/export-deletion.test.mjs:565-580` still verifies page wiring by reading `export-deletion.tsx` and matching source text. The new `confirmAbandonIntent` test (`:582-609`) checks the extracted helper with synthetic callbacks, but does not render the page or execute its handlers, `useAction`, busy props, confirmation state, or the `outcome_unknown` catch-to-gating transition. The N1/N3 test (`:611-637`) now demonstrates the service guard and computes the gate with `recoveryUnresolved: true` directly.

**Impact:** The repair has good service/helper coverage, but the visible user transitions cited in prior R2 remain unverified. A wiring or state-update regression can pass all focused tests.

**Smallest defensible correction:** Add a rendered page interaction test, or extract a page controller whose actual handlers drive observable state. Cover cancel, changed ID, concurrent recovery, successful abandon, and an ambiguous deletion reaching a disabled main action.

## Spec and code-quality verdict

- **Spec compliance: not yet approved.** The sequential N1 path now rejects a second deletion, N8 no longer substitutes the profile revision for a legacy progress intent, N9 pins the deletion key to its sending scope, and the R1 changed-ID abandonment is conditional. F1 and F2 leave recovery-ID retention vulnerable under overlapping operations.
- **Code quality: not yet approved.** Focused tests pass and the diff is clean, but F3 leaves the page's asynchronous behavior untested. No new direct cross-scope read or export-data disclosure was found in the reviewed diff.
