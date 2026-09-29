# T31 OCR Round 3 Repair Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close valid T31 OCR findings in the Release framework checker, generated AppDelegate verifier, and Release script documentation.

**Architecture:** Keep the native build pipeline and SDK57 configuration unchanged. Harden the dynamic dependency checker so every unrecognized non-system load path fails closed, align generated AppDelegate checks with the existing comment/string-masked Swift source, and update the clean-build comment to describe actual `--clean` behavior.

**Tech Stack:** Python 3, TypeScript/tsx Node test runner, Bash, Expo SDK57 generated iOS project verifier.

**Spec:** `docs/plans/issue30-sweep/plans/plan-t31-ios27.md`; T31 acceptance and evidence records in `.superpowers/sdd/plan-t31-ios27/`; OCR report `docs/plans/issue30-sweep/ocr/ocr-t31-r3-final.md` (session `0550113f-093f-4c82-b9c1-7d935b1844ab`).

## Global Constraints

- Preserve Expo SDK57 generated scene startup, callback forwarding, and iOS deployment target 16.4.
- Keep generated `apps/mobile/ios/` untracked and do not claim a native build unless actually run.
- Dynamic loads outside the approved system roots must either resolve to a verified in-app dependency or fail closed.
- Make tests assert stable failure codes and no Python traceback for invalid inputs.
- Do not change T31 acceptance or evidence claims to mask failed coverage.

## Review Focus

- Unknown `@rpath`, `@loader_path`, `@executable_path`, and non-system absolute dylib loads must not pass silently; only explicit `/System/Library/` and `/usr/lib/` paths may be treated as OS-provided.
- A comment or string containing `ExpoReactNativeFactoryProvider` must not satisfy the generated AppDelegate conformance check.
- Release script instructions must agree with the clean prebuild performed by the script, including regenerated codegen output.

---

### Task 1: Close the T31 checker and documentation OCR findings

**Dependencies:** T31 integrated source review head `3e632cb3ec37d61505bfd434dd70181f33063b91`; full-range OCR finding report `ocr-t31-r3-final.md`.

**Owner role:** `frontend_implementer`. **Validator role:** `frontend_validator`. **Independent reviewer:** `reviewer`.

**Files:**
- Modify: `apps/mobile/scripts/verify-ios-framework-closure.py`
- Test: `apps/mobile/src/ios-framework-closure.test.ts`
- Modify: `apps/mobile/scripts/verify-ios-scene-project.ts`
- Test: `apps/mobile/src/scripts/verify-ios-scene-project.test.ts`
- Modify: `apps/mobile/scripts/ios-release-build.sh`

**Interfaces:**
- Consume the current stable checker messages `UNSUPPORTED_FRAMEWORK_LOAD_PATH` and `FRAMEWORK_LOAD_OUTSIDE_APP` and current generated project verifier `verifyIosSceneProject`.
- Produce a stable `UNSUPPORTED_DYNAMIC_LIBRARY_LOAD` failure for any non-system load not proven to be an embedded framework dependency; enforce AppDelegate provider conformance against `maskSwiftNonCode` output; keep the Release script workflow and commands unchanged.

- [ ] **Step 1: Add failing fixtures for the Medium dynamic-load gap.** Add checker fixtures for `@rpath/libMissing.dylib`, `@loader_path/libMissing.dylib`, and a non-system absolute dylib; assert nonzero exit with `UNSUPPORTED_DYNAMIC_LIBRARY_LOAD` and no silent success. Add a positive fixture for a load rooted under `/usr/lib/` (and retain the existing `/System/Library/Frameworks/` case) to pin the OS whitelist.
- [ ] **Step 2: Add failing fixtures for comment-masked provider conformance.** Change the generated AppDelegate fixture so conformance is removed but a comment/string retains `ExpoReactNativeFactoryProvider`; assert the verifier reports the missing conformance. Keep a real conformance fixture passing.
- [ ] **Step 3: Run focused tests to confirm the gaps.** Run `pnpm --filter @weknora/mobile exec tsx --test src/ios-framework-closure.test.ts src/scripts/verify-ios-scene-project.test.ts`. Expected before implementation: the new non-system dynamic-load fixture and masked-conformance fixture expose current false passes.
- [ ] **Step 4: Implement fail-closed checks.** In the framework checker, allow only the explicit system roots before classifying unsupported load forms; retain existing framework resolution and app-boundary checks. In the scene verifier, compute `swiftCode = maskSwiftNonCode(appDelegate)` before checking `ExpoReactNativeFactoryProvider`, and use that masked source for the conformance check.
- [ ] **Step 5: Correct the Release script comment.** State that `expo prebuild --clean` deletes generated `ios/`, `ios/build`, and codegen artifacts, so the xcodebuild step is a full rebuild each time; remove the stale instruction implying a prior build directory may be reused.
- [ ] **Step 6: Run verification.** Run the focused test command above, `pnpm --filter @weknora/mobile typecheck`, `bash -n apps/mobile/scripts/ios-release-build.sh`, `git diff --check`, and the generated scene verifier against retained SDK57 output if available. Expected: all tests/typecheck/syntax checks pass; generated output remains unchanged and passes if present.
- [ ] **Step 7: Commit and report.** Commit only owned source/tests/report. Record the OCR finding IDs, exact commands/results, source commit, diff hash, and generated-output availability in `.superpowers/sdd/plan-t31-ios27/ocr-r3-repair-report.md`.

**Failure handling:** If real SDK57 Release output contains legitimate non-framework dylib loads outside `/usr/lib/` or `/System/Library/`, stop and report the exact load command and bundle evidence; do not add a broad allowlist. A correct complete-bundle resolver can be designed as a separate bounded change.

## Plan self-review

- Finding coverage: all three OCR comments have direct implementation/test or documentation updates.
- Task boundary: all changes are T31 release checker or native contract evidence; no production application behavior is altered.
- Verification: tests cover both fail-closed rejection and system whitelist, and the existing generated project verification is re-run where retained output exists.
