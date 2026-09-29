# T31 OCR Round 2 Repair Plan

**Goal:** Resolve substantiated OCR review gaps in Issue #31's iOS 27 shell and native validation guards; adjudicate false positives and non-blocking Low suggestions with durable evidence.

**T31 source review range:** `1e9315773a971dd72fe621c94e308f45a0ca4692..860b84016c8c6c94edd8c8adcd1ca672132efd17`.

**Spec:** `docs/specs/2026-09-20-mobile-ai-office-design.md`; ADR `docs/adr/0005-weknora-native-mobile-client.md`; Issue #31 snapshot `docs/plans/issue30-sweep/issues/issue-31.md`; current T31 plan `docs/plans/issue30-sweep/plans/plan-t31-ios27.md`; OCR outputs and adjudications in `.superpowers/sdd/plan-t31-ios27/`.

**Global Constraints:** Keep Expo SDK57 generated scene startup, `expo.ios.deploymentTarget = 16.4`, release framework closure, and the current provider → safe-area view → Stack production structure. Do not change external acceptance claims or close Issue #31. Keep generated `apps/mobile/ios/` untracked. Fix only adjudicated issues; do not alter valid config to satisfy false OCR comments.

**Review Focus:** The smoke test must prove the actual provider → SafeAreaView → Stack hierarchy. The production framework checker must return its stable fail-closed error for valid non-object properties input and exercise the app-boundary escape guard. Current prebuild/plugin configuration remains valid. Task work must preserve the verified SDK57 and retained Release behavior.

## OCR finding rulings

| ID | OCR claim | Ruling |
|---|---|---|
| OCR-R1-1 | Fixed inline safe-area style should use StyleSheet | Low style cleanup; include with shell test repair. |
| OCR-R1-2 | `expo.ios.deploymentTarget` is invalid | False: Expo SDK57 supports the built-in field; retain 16.4. |
| OCR-R1-3 | Real PBX config list has no array and guard always fails | False: both actual prebuild artifacts use comma-terminated `buildConfigurations` arrays; production verifier passed both. |
| OCR-R1-4 | `@expo/dom-webview` unused dependency | Low / impact overstated: Expo SDK declares it as a dependency; defer direct-dependency cleanup to dependency audit. |
| OCR-R1-5 | Scene guard is neither tested nor typechecked | False as stated: the generated contract test imports it, test glob includes the test, and TypeScript follows imports. |
| OCR-R1-6 | Nested ternary is hard to read | Valid Low readability cleanup; include with native tooling repair. |
| OCR-R2-1 | Safe-area shell test does not assert provider contains safe view | Valid Medium test gap; repair Task R2-1. |
| OCR-R2-2 | Missing explicit Stack assertion | Valid Low diagnostic improvement; repair Task R2-1. |
| OCR-R2-3 | Retired plugin is invalid and must be deleted | False as Medium: the repair plan permits the unused file to remain; only the `.gitignore` explanation is stale. |
| OCR-R2-4 | Direct Expo dependency declarations are unused (repeated as OCR-R2-10 in the second report) | Low / overstated: both packages are SDK dependencies and `expo-constants` supports Expo Router; defer removal. |
| OCR-R2-5 | Framework properties JSON root may be non-object and cause traceback | Valid Low error-contract gap; repair Task R2-2. |
| OCR-R2-6 | `expo-asset` string plugin has no valid plugin entry | False: installed SDK57 package exports a valid config plugin and clean prebuild evidence exists. |
| OCR-R2-7 | Generated scene test filename/path is misleading | Valid Low discoverability cleanup; rename under Task R2-2. |
| OCR-R2-8 | Framework path-escape branch lacks a negative fixture | Valid Low security-guard coverage gap; repair Task R2-2. |
| OCR-R2-9 | Nested ternary reduces lexer readability | Valid Low readability cleanup; repair Task R2-2. |
| OCR-R2-10 | `.gitignore` still attributes generated iOS tree to retired plugin | Valid Low documentation inconsistency; update the comment only; keep the historical plugin file. |

The OCR cancellation notice names a combined `_layout.tsx`/`app-smoke.test.tsx` request. The OCR session manifest records all 14 included code/test files completed, but final verification will rerun OCR on the full T31 range with the test include rule and check final session coverage. Markdown and binary evidence are unsupported by OCR; T31 plan, records, and DOC3 status have separate independent reviews and hash/path checks. Preserve that tool limitation in the final OCR record.

## Preflight consistency table

| Task | Files / interface overlap | Produced → consumed | Result |
|---|---|---|---|
| R2-1 vs R2-2 | No shared files | Both consume the current generated shell and framework-checker contracts; no produced interface is consumed by the other. | Disjoint; parallel work is safe. |
| R2-1 internal | `_layout.tsx`, `app-smoke.test.tsx` | Static StyleSheet export is consumed by the existing React Native smoke stub; provider and Stack checks inspect the rendered shell. | Keep stub and shell edits in one task; consistent. |
| R2-2 internal | framework checker, framework closure test, scene contract test rename, `.gitignore` | Python checker emits stable failure code asserted by TS harness; test rename keeps the same `src/**/*.test.ts*` discovery glob. | Consistent; no native project output changes. |

## Task R2-1: Pin shell hierarchy and clarify smoke failures

- **Role:** `frontend_implementer`; **validator:** `frontend_validator`; **reviewer:** independent reviewer.
- **Owned files:** `apps/mobile/src/app/_layout.tsx`, `apps/mobile/src/app-smoke.test.tsx`, plus `.superpowers/sdd/plan-t31-ios27/ocr-r2-1-report.md`.
- **Acceptance:** The test proves RootLayout's rendered root is SafeAreaProvider, its direct child is SafeAreaView, and SafeAreaView's direct child is Stack. Missing Stack fails with a clear assertion. If static flex style is moved to `StyleSheet.create`, update the `react-native` test stub accordingly. Existing shell behavior remains unchanged.
- **Steps / verification:** Add the smallest failing hierarchy assertion first; verify it fails when the provider/view are siblings; implement structural assertions and optional StyleSheet cleanup; run the targeted app smoke test and `git diff --check`; commit only owned files and report exact results.
- **Failure handling:** If the smoke renderer cannot observe the provider relationship without broad harness changes, report the smallest alternative structural seam; do not modify unrelated runtime surfaces.

## Task R2-2: Harden framework mode/error edge cases and native contract records

- **Role:** `mechanical_worker`; **validator:** independent reviewer.
- **Owned files:** `apps/mobile/scripts/verify-ios-framework-closure.py`, `apps/mobile/src/ios-framework-closure.test.ts`, `apps/mobile/scripts/verify-ios-scene-project.ts`, `apps/mobile/src/plugins/ios-xcode27.test.ts` (rename target `apps/mobile/src/scripts/verify-ios-scene-project.test.ts`), `.gitignore`, plus `.superpowers/sdd/plan-t31-ios27/ocr-r2-2-report.md`.
- **Acceptance:** A valid JSON or plist root that is not an object exits nonzero with `FRAMEWORK_MODE_PROPERTIES_INVALID` and no traceback; load commands that resolve outside the app exit with `FRAMEWORK_LOAD_OUTSIDE_APP`; the existing green source/precompiled and dependency fixtures remain green. Replace the nested lexer ternary with explicit branches without changing supported Swift syntax. Rename the generated-scene test to match its tested script. Update only the stale `.gitignore` explanation to attribute native generation to clean Expo prebuild; do not delete the historical plugin.
- **Steps / verification:** Add a failing properties-root fixture and app-boundary load fixture; capture RED; implement root-type guard and exact-coded failure; implement path fixtures and assert error code; simplify string delimiter choice; rename test with import/discovery preserved; correct `.gitignore` comment; run focused framework and scene-contract tests, mobile typecheck, `git diff --check`, and verify clean prebuild checker artifacts remain unchanged/passing.
- **Failure handling:** If a fixture cannot express an outside-app path on the host, report the platform reason and preserve static branch evidence; never weaken fail-closed behavior to satisfy the fixture.

## Integration and completion

The tasks have disjoint ownership and run in separate worktrees from task BASE `1df6f5cb2cdfd4253941abc5b113e778248cd189` (planning/evidence commit atop source-review HEAD `860b84016c8c6c94edd8c8adcd1ca672132efd17`). Integrate in task ID order only after each scoped review passes. Run the existing targeted/full mobile verification appropriate to both slices, then re-run OCR for the complete supported T31 code/test range with the test include rule and inspect its session manifest. Independently review DOC3 and T31 records because OCR rejects Markdown and binary evidence. The final integrated T31 review and OCR scope must include the resulting repair commits. Do not release #32 until valid C/H/M findings are resolved and the documented OCR coverage limitation is adjudicated.

### Task R2-1 review repair round 1

The independent review `.superpowers/sdd/plan-t31-ios27/ocr-r2-1-review.md` found that the test pinned Provider → SafeAreaView → Stack but not the actual rendered root. This is a valid Medium plan-compliance gap because the acceptance explicitly requires RootLayout's root to be SafeAreaProvider. The task report also omitted its implementation commit SHA. Reuse the same frontend implementer for round 1; owned scope stays the smoke test and task report. Brief: `.superpowers/sdd/plan-t31-ios27/ocr-r2-1-r1-brief.md`. Implementation checkpoint `ad68a23a83631e336400ae8167ee4bf2307fa225`; review repair pending.

### Task R2-2 review repair round 1

R2-2 scoped review `.superpowers/sdd/plan-t31-ios27/ocr-r2-2-review.md` passed Spec Compliance and found no blocker; one valid Low coverage item remains because only the JSON non-object properties root was tested, while the checker also supports plist input and the plan requires both formats. Add an XML plist array fixture asserting the stable coded failure and absence of traceback. Reuse the same mechanical implementer. Brief `.superpowers/sdd/plan-t31-ios27/ocr-r2-2-r1-brief.md`; base checkpoint `a13263252533db91abf2baab2f00360e4c893543`; pending.

#### R2-1 review record correction

R2-1 re-review report `.superpowers/sdd/plan-t31-ios27/ocr-r2-1-r1-review.md` gives Spec PASS / Code Quality PASS with a Low task-report SHA mismatch. Correct the report to cite final commit `187a0dfdda957ac6372b4d34e58fe103502ac502` (not the intermediate `9ff7dab...`). Brief `.superpowers/sdd/plan-t31-ios27/ocr-r2-1-r1-doc-fix-brief.md`; report-only scope, same implementer, no behavior change.
