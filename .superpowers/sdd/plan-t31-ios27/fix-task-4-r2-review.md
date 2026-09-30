# T31 R4 repair round 2 — independent scoped review

- Reviewed range: `1ebf2dfe55b4b0211c4753a82744e0a61cb86890..10c05c1e73d961aa64f5de986c90e5dcaa933e2a` (`0311f57e1` code/tests, `10c05c1e7` report). The controller's uncommitted ledger and plan edits were excluded.
- Authority: `.superpowers/sdd/plan-t31-ios27/fix-task-4-r2-brief.md`, round-1 review, Issue #31 snapshot, approved mobile spec, ADR 0005, and `CONTEXT.md`.
- **Spec compliance: FAIL on the round-2 brief's sibling-framework integration-test criterion.** The prior Medium and Low findings are corrected in the production path, but the fixture named for the sibling case does not create the requested sibling-framework path. This verdict does not imply Issue #31's staging login/OIDC, capability, Android, or real-device acceptance is complete.
- **Code quality: PASS with one Low test-coverage finding.** No remaining false-positive closure path or mode-label regression was found in this diff.

## Prior findings

1. **R4-R1-1 Medium — addressed.** `verify-ios-framework-closure.py:69–70` resolves each framework's declared `CFBundleExecutable` within its own bundle. Lines 81–85 resolve the requested `@rpath/<Framework>.framework/<path>` from that named bundle and require the same regular file. A plist value of `../Bar.framework/Foo` now fails the bundle-containment check even if `Bar.framework/Foo` exists; an absent `Foo.framework/Foo` cannot pass by name alone. The requested-name mismatch fixture at `ios-framework-closure.test.ts:109` exercises the second comparison.
2. **R4-R1-2 Low — addressed.** `ios-release-build.sh:65` passes generated `Podfile.properties.json`. Checker lines 51–63 read both mode properties, reject missing, unknown, and disallowed combinations, and line 90 reports that configuration. Lines 71–72 reject precompiled Worklets embedded under configured source mode. Mode is no longer inferred from the absence of one framework.
3. **Versioned framework path — addressed.** Checker lines 81–85 compare fully resolved paths. The fixture at `ios-framework-closure.test.ts:95` proves `@rpath/Alpha.framework/Versions/A/Alpha` resolves to the executable declared as `Alpha` through a bundle-local symlink.

## Finding

### R4-R2-1 — Low — Sibling framework fixture does not use a sibling framework path

- **Evidence:** `ios-framework-closure.test.ts:84–92` calls the case a sibling bundle escape, but writes `CFBundleExecutable=../Beta/Alpha`. It creates `Frameworks/Beta.framework/Alpha`, so the declared path resolves to absent `Frameworks/Beta/Alpha`. This does prove an escape is rejected, but it does not exercise the concrete prior false-positive layout `../Beta.framework/Alpha` with a regular same-named file in the sibling framework. There is also no fixture for a symlink into a sibling framework, although the brief expressly includes it.
- **Impact:** The test would still pass if a future checker rejected only the absent `../Beta/Alpha` case and accidentally reintroduced acceptance of a real sibling-framework executable. Current production code independently checks own-bundle containment, so this is a coverage risk rather than a present runtime failure.
- **Smallest correction:** Change the plist fixture to `../Beta.framework/Alpha` and keep the existing sibling file; optionally add the sibling symlink case required by the brief. Assert nonzero exit as the test already does.

## Evidence and limits

- `git diff 1ebf2dfe5..10c05c1e7 --check` had no whitespace errors. The retained app executable still hashes to `8d2141d06f3b6f0d185be79dff122e7fd0d102e901d72862bed7dec200e073e4`; generated properties contain `ios.buildReactNativeFromSource=true` and `EXPO_USE_PRECOMPILED_MODULES=false`. The range changes no native evidence artifact or generated iOS file.
- The implementation report records focused tests 12/12, full mobile suite 310 total / 296 pass / 14 opt-in skips, typecheck, Expo dependency check, and diff check passing; it records the revised checker returning `FRAMEWORK_MODE=source-expo-modules` and `FRAMEWORK_CLOSURE_OK` for that retained app. Raw command transcripts for those round-2 checks are not tracked in the reviewed range, so these results are implementation-reported evidence, not independently reproduced by this review.
- Review commands: `git rev-parse HEAD`, `git status --short`, `git log --oneline 1ebf2dfe5..10c05c1e7`, `git diff --stat/--check/-- apps/mobile/scripts apps/mobile/src ... 1ebf2dfe5..10c05c1e7`, `cat` of the brief and report, `nl -ba` of checker/tests, `shasum -a 256` of the retained executable, and `cat` of generated Podfile properties. No OCR, build, test, simulator command, or implementation edit was run.
