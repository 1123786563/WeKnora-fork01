# Task 12 N9 fixture review

Scope: focused diff of `apps/miniprogram/tests/export-deletion.test.mjs` against `e81c867f4`; reviewed without editing source or running tests. Fact sources: approved job-search spec § privacy/acceptance, T32 mini export/deletion brief, `CONTEXT.md`, Task 12 in `2026-09-29-issue140-miniprogram-residuals.md`, and the current runtime, session, service, and Taro stub code. No directly applicable ADR changes the N9 timing contract.

## Findings

### N9-R1 — Medium — completed retry no longer tests the specified response-to-scope race

**Evidence:** In `apps/miniprogram/tests/export-deletion.test.mjs:365-369`, the retry test waits for the native POST, calls `auth.scope.switchTo(...)`, then calls `stub.succeed(...)`. `stub.succeed` invokes the native success callback synchronously (`tests/helpers/taro-stub.mjs:154-156`), so this is a scope change **before** response completion. Task 12 explicitly requires clearing the captured key when the UI scope changes **after the server response**. The prior fixture wrapped `client.request` and switched scope after its awaited result, before `retryPendingSpaceDeletion` resumed. The new fixture removes that seam. The facade's `ScopeGuard` is separate from `MobileRuntime` (`src/services/session.ts:43,98,147`), so this test may still pass while exercising a different ordering.

**Impact:** A regression that obtains a completed receipt and then re-derives the intent key from the new UI scope could pass this test if it uses a guard or snapshot taken before the receipt callback. The intended narrow continuation window is untested.

**Smallest correction:** Keep the exact-POST wait, but restore a one-use `client.request` wrapper for the retry POST that switches the facade scope immediately after `await originalRequest(...)` returns and before returning the receipt to `retryPendingSpaceDeletion`; answer the selected native POST while the original scope is active. Restore the wrapper in `finally`. Assert the completed receipt and original-key removal.

### N9-R2 — Low — Task 12 evidence is incomplete

**Evidence:** Task 12 steps require assertions for the captured key, current-scope key, request count, completed receipt, and ambiguous timeout behavior. The changed tests at `apps/miniprogram/tests/export-deletion.test.mjs:336-374` assert the original-key write/removal and `pendingSpaceDeletion() === null` in the new scope, but do not assert the exact POST count, returned completed receipt, or that a timeout in the retry leaves the original key intact. The helper selects the correct POST by method, path, and occurrence (`:107-115`), which resolves the original wrong-call fixture problem, but selection alone does not establish those acceptance observations.

**Impact:** Extra deletion sends or an incorrect return value could go unnoticed. This focused change does not supply the requested ambiguous-outcome evidence.

**Smallest correction:** Add exact POST count and returned receipt assertions to these N9 cases; add or cite a focused timeout test that checks the original captured key survives an ambiguous retry under the same scope-switch scenario.

## Verdict

**Spec compliance: incomplete.** Exact native POST targeting is deterministic, and the first N9 case preserves a post-response scope switch before the service writes. The second N9 case changes the ordering required by Task 12, and several explicit acceptance observations are absent.

**Code quality: conditionally sound.** The helper filters `kind`, method, and exact pathname and uses a bounded wait. The indentation at line 330 should be corrected in the same small fixture revision. No source, requirements, or remote issues were modified during this review; tests were not run.

## Addendum — amended working diff

Reviewed the later uncommitted revision against `e81c867f4` and the preceding fixture revision; no tests were run.

**N9-R1 resolved.** The second case now answers the exact second native deletion POST while the original scope is active (`export-deletion.test.mjs:385-387`). Its one-use `client.request` wrapper awaits the original request, then changes the facade scope before returning the receipt to `retryPendingSpaceDeletion` (`:375-382`). This recreates the required post-response, pre-service-continuation ordering. The test asserts the returned `deleted` receipt, captured request ID, one retry POST, original-key removal, and empty new-scope key (`:391-398`). The native request selector still matches method and exact pathname.

**N9-R2 partly resolved; low remains.** The amended second case supplies receipt, request ID, count, and both key assertions. It still does not add N9-specific evidence for an ambiguous retry preserving the original captured key under a scope transition. Existing F2 (`:707-733`) covers an ambiguous timeout retaining an intent in the same scope, which satisfies the general recovery behavior but not the cross-scope N9 variant. The new-scope key starts empty, so asserting it remains empty cannot detect an accidental removal of a pre-existing intent there; seed a distinct new-scope intent if strict noninterference is part of Task 12 acceptance. The new `waitFor` helper at `:116-122` is currently unused; remove it if no follow-up case will consume it. The first N9 wrapper's indentation remains uneven at `:346`.

**Updated verdict:** The focused N9 response-ordering defect is fixed by inspection, and the amended test now covers the core captured-key completion contract. Spec compliance remains partial for the requested ambiguous cross-scope evidence and strict other-scope noninterference observation. Code quality is sound aside from the unused helper and indentation. This is an inspection verdict, not a test-pass claim.

## Final addendum — ambiguous cross-scope case

Reviewed the latest working diff against the previous amendment and Task 12; no tests were run.

**Resolved:** The new N9 ambiguous-retry case (`export-deletion.test.mjs:404-452`) seeds distinct original and other-scope intents, fails the exact next native deletion POST with a timeout, switches the facade scope in the request rejection wrapper, and checks `SCOPE_CHANGED` plus byte-for-byte preservation of both intents. The completion and timeout cases restore the original facade scope after their assertions. The unused `waitFor` helper has been removed, and the first N9 wrapper indentation is corrected. The previous N9-R2 ambiguous-outcome gap is therefore closed.

### N9-R3 — Low — successful retry does not prove it leaves an existing other-scope intent intact

**Evidence:** The successful retry computes `currentScopeKey` and asserts it is `undefined` after completion (`export-deletion.test.mjs:369,397`), but does not seed a value at that key before the retry. The new seeded other-scope intent belongs to the **ambiguous failure** test, whose service path does not execute the success branch's `store.remove(key)` (`src/services/career.ts:651-654`).

**Impact:** A regression in the completed-receipt branch that removes both the captured key and a pre-existing other-scope intent could pass the successful N9 test, while the timeout test would also pass. Task 12 requires that completion clear only the original key.

**Smallest correction:** Seed a distinct intent under `currentScopeKey` before the successful retry, then assert the original key is removed and the seeded other-scope intent is unchanged and visible only under the switched scope. Keep the existing returned-receipt and exact-POST assertions.

**Final inspection verdict:** The N9 fixture deterministically targets the intended POST, exercises the required post-response switch, and covers ambiguous cross-scope retention. One low-severity behavioral-test gap remains for other-scope noninterference on successful completion. Source behavior currently uses the captured key; this finding concerns regression coverage, not an observed production defect. No test-pass claim is made.

## Closure addendum — seeded success-path other scope

Inspected the final small amendment. The successful retry now seeds `currentScopeKey` with a distinct `otherIntent` before the POST (`export-deletion.test.mjs:369-372`), then asserts that the captured original key is gone, the seeded other-scope value is unchanged, and the switched scope sees its own request ID (`:400-402`). It removes the fixture key and restores the original facade scope afterward (`:403-404`). This directly closes N9-R3. The ambiguous case still checks preservation of both intents after the exact native POST fails and the scope changes.

**Final spec verdict: compliant by code inspection for the focused N9 fixture and Task 12 storage/scope assertions. Final quality verdict: no open finding in this focused diff.** The selector targets the exact native POST, success changes scope after the response and before the service continuation, and ambiguous failure preserves both scope records. No tests were run as part of this independent review; runtime pass status requires the controller's validation evidence.
