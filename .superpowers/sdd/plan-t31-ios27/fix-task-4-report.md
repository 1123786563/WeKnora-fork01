# Task R4 report — clean Release framework closure

## Outcome

Aligned generated iOS native dependencies on Expo/RN source mode and added a Release post-build framework load closure gate. The original mismatch was directly visible in clean generated output: `Podfile.properties.json` enabled `EXPO_USE_PRECOMPILED_MODULES=true`, `Podfile.lock` contained React-Core 0.86.3 source and no React-Core-prebuilt, while the precompiled `ExpoModulesWorklets.framework` slice required `@rpath/React.framework/React`. `Pods-WeKnora-frameworks.sh` embedded Worklets but not React. Expo dependency alignment passed, which rules out package version drift but not packaging mode mismatch.

The supported `expo-build-properties` options now explicitly set `buildReactNativeFromSource: true` and `usePrecompiledModules: false`. The generated clean project selected source React and Expo modules (`EXPO_USE_PRECOMPILED_MODULES=false`); resulting app embeds ExpoModulesJSI and Hermes dynamic frameworks while React and ExpoModulesWorklets are linked from source/static Pods. The app executable and all embedded dynamic frameworks have no unresolved non-system `@rpath` dependencies. The script names missing framework dependencies and fails on `otool` inspection errors. Under source mode it also rejects accidental embedding of the precompiled ExpoModulesWorklets framework.

## TDD and checks

- RED fixture contract: Worklets requiring absent React returns `React.framework`; complete graph returns no missing dependencies. Source/precompiled mode cases are pinned in `ios-framework-closure.test.ts`.
- Negative check: fixture graph reported `MISSING_FRAMEWORK_DEPENDENCY: ExpoModulesWorklets.framework requires React.framework` and exited 1.
- Focused framework and script tests: 7/7 pass.
- Full mobile suite: 303 total, 289 pass, 14 opt-in environment skips, 0 fail.
- Typecheck: `pnpm --filter @weknora/mobile typecheck` — pass.
- Expo alignment: `pnpm --filter @weknora/mobile exec expo install --check` — Dependencies are up to date.
- Diff: `git diff --check` — pass.

## iOS 27 Release evidence

- Command: `IOS_BUILD_EVIDENCE_DIR=<worktree>/docs/testing/evidence/mobile-runtime-login/2026-09-29-r4 bash apps/mobile/scripts/ios-release-build.sh <worktree>`.
- Result: exit 0; `** BUILD SUCCEEDED **`, `FRAMEWORK_MODE=source-expo-modules`, `FRAMEWORK_CLOSURE_OK`, and the Release app path were emitted. Full script stdout/stderr is preserved in `docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/xcodebuild-release-final.log`; Xcode `tee` output is `xcodebuild-release.log`.
- Installed and launched on iPhone 18 Pro, iOS 27.0, UDID `0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC`. `simctl launch --console` remained attached and logged `ReactInstance: evaluateJavaScript() with JS bundle`; no dyld loader failure occurred.
- Screenshot: `docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/ios27-release-launch.png`, SHA256 `2bcd87c0d9e6c757c59348fcad03717466b724b5fe97c2dbb08a0d240ce25485`. Inspected image shows WeKnora sign-in root below status bar. Launch record is `launch.log`.
- T39 tracked build log restored/verified unchanged at SHA256 `12a27ed4f698074356c5e1d7722b5d7af78b2d60b5cdf9e9856e44e37b7bb71e`.

## Changed files

- `apps/mobile/app.json`
- `apps/mobile/scripts/ios-release-build.sh`
- `apps/mobile/src/ios-framework-closure.ts`
- `apps/mobile/src/ios-framework-closure.test.ts`
- `apps/mobile/src/native-project-config.test.ts`
- `apps/mobile/src/scripts/ios-acceptance-scripts.test.ts`
- `.superpowers/sdd/plan-t31-ios27/fix-progress.md`
- This report and native evidence under `docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/`.

## Remaining notes

- Simulator emitted existing warnings for missing background fetch/remote notification UIBackgroundModes and duplicate accessibility classes in the iOS 27 simulator runtime; neither prevented launch or affected loader closure.
- Independent frontend-validator and reviewer reports remain parent-coordinated.
