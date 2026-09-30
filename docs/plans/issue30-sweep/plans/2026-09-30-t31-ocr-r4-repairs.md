# T31 OCR Round 4 Review Repair Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the remaining dynamic-load false pass, repair the Swift negative fixture, and accurately disposition all six comments from the complete T31 OCR review.

**Architecture:** Fail closed on every non-system dynamic load not proven to be an embedded framework; preserve explicit OS-root allowlists. Keep the current SafeAreaProvider → SafeAreaView → Stack root contract, seed the nested provider with SDK native initial metrics to prevent a first-frame inset jump, and correct tests so their fake comments/strings remain in source. Record why a hard-coded safe-area background color is deferred because the app has no shared root background token.

**Tech Stack:** Python 3, TypeScript/tsx tests, React Native safe-area-context SDK57 API, Bash.

**Spec:** `docs/plans/issue30-sweep/plans/plan-t31-ios27.md`; complete OCR results at `docs/plans/issue30-sweep/ocr/ocr-t31-r3-final.md` and `ocr-t31-r3-resumed.md`; independent review `.superpowers/sdd/plan-t31-ios27/ocr-r3-repair-review.md`.

## Global Constraints

- Preserve the accepted SafeAreaProvider → SafeAreaView → Stack structure and Expo SDK57 generated contract.
- Do not hard-code an arbitrary light/dark background without a shared theme token.
- Fail closed for unrecognized non-system `otool -L` entries; allow only explicit `/System/Library/` and `/usr/lib/` OS roots.
- Preserve the `initialWindowMetrics` null behavior on platforms where the native constant is unavailable.
- Record all OCR comments by path and line anchor; the report has no stable finding IDs.

## Review Focus

- `@unknown_path/libMissing.dylib`, a bare `libMissing.dylib`, and known token dylibs fail with a stable code; system-root libraries remain allowed.
- Swift comment and string tokens remain present after test fixture mutation, while real conformance remains present in the positive test.
- Root SafeAreaProvider receives `initialWindowMetrics`; no first-frame zero-inset regression is introduced.
- Root background styling remains a documented low-priority disposition until design provides a shared theme surface.

---

### Task 1: Close remaining OCR R3 test and scope gaps

**Dependencies:** T31 R3 source commit `f3a9ca6ea`, report `c9c23cc90`, independent review `.superpowers/sdd/plan-t31-ios27/ocr-r3-repair-review.md`.

**Owner role:** `frontend_implementer`. **Validator role:** `frontend_validator`. **Independent reviewer:** `reviewer`.

**Files:**
- Modify: `apps/mobile/scripts/verify-ios-framework-closure.py`
- Test: `apps/mobile/src/ios-framework-closure.test.ts`
- Test: `apps/mobile/src/scripts/verify-ios-scene-project.test.ts`
- Modify: `apps/mobile/src/app/_layout.tsx`
- Test: `apps/mobile/src/app-smoke.test.tsx`
- Modify only the OCR R3 report: `.superpowers/sdd/plan-t31-ios27/ocr-r3-repair-report.md`

**Interfaces:**
- Consume `maskSwiftNonCode`, `SafeAreaProvider`, `initialWindowMetrics`, and the existing framework checker stable message convention.
- Produce `UNSUPPORTED_DYNAMIC_LIBRARY_LOAD` for unknown/bare/non-system unverified loads; keep approved OS root behavior unchanged; pass native initial metrics to RootLayout's provider and document the intentional nested shell contract.

- [ ] **Step 1: Add failing dynamic-load fixtures.** Cover `@unknown_path/libMissing.dylib`, bare `libMissing.dylib`, and known `@rpath/libMissing.dylib` as failures with `UNSUPPORTED_DYNAMIC_LIBRARY_LOAD`; retain positive `/usr/lib/` and `/System/Library/Frameworks/` fixtures.
- [ ] **Step 2: Correct the masked Swift test fixture.** Replace only the real `ExpoReactNativeFactoryProvider` conformance with a comment and a string that both retain the token. Assert verifier rejection. Assert the positive fixture with real conformance continues to pass.
- [ ] **Step 3: Add a failing RootLayout initial-metrics assertion.** Extend the smoke test's safe-area stub to expose a concrete `initialWindowMetrics` marker and assert the RootLayout's SafeAreaProvider receives it. Confirm the current implementation fails this assertion before the change.
- [ ] **Step 4: Harden the checker and RootLayout.** Make every unrecognized non-system load fail with `UNSUPPORTED_DYNAMIC_LIBRARY_LOAD` after framework-specific failures; remove the permissive `continue`. Import `initialWindowMetrics` and pass it to `SafeAreaProvider`. Add a concise comment preserving the intentional RootLayout shell structure and describing the nested provider's initial metrics.
- [ ] **Step 5: Correct the OCR report's six-comment disposition.** Record path/line anchors and outcomes: checker Medium and provider fixture Low are addressed in R3/R4; release comment Low is addressed; three `_layout.tsx` comments are separately ruled: initial metrics fixed; duplicate provider rationale documented; safe-area background deferred because the codebase has no shared root theme background token and choosing a literal light/dark surface could introduce a product regression.
- [ ] **Step 6: Run focused and package checks.** Run `pnpm --filter @weknora/mobile exec tsx --test src/ios-framework-closure.test.ts src/scripts/verify-ios-scene-project.test.ts src/app-smoke.test.tsx`, `pnpm --filter @weknora/mobile typecheck`, `bash -n apps/mobile/scripts/ios-release-build.sh`, and `git diff --check`. Run generated-project verification only if retained clean SDK57 output exists. Expected: focused tests, typecheck, syntax and whitespace checks pass.
- [ ] **Step 7: Commit and report.** Commit only owned files and the updated report; record commit/diff hashes and exact OCR comment dispositions at `.superpowers/sdd/plan-t31-ios27/ocr-r4-repair-report.md`.

**Failure handling:** If a supported clean SDK57 Release has non-framework non-system dylib dependencies not modeled by this checker, retain fail-closed behavior and report the exact `otool` evidence before designing a broader resolver. Do not silently add a path allowlist.

## Plan self-review

- OCR coverage: all six comments from the completed R3 + resume report are accounted for; the only deferred item is the low-priority safe-area background suggestion, with an explicit product rationale.
- Interface consistency: checker messages remain stable; RootLayout shell hierarchy stays as already accepted.
- Verification: tests target each review correction, and the native environment remains an explicit evidence dependency.
