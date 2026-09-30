# Task R4 report — clean Release framework closure

## Initial R4 implementation (reviewed; subsequently repaired)

Aligned generated iOS native dependencies on Expo/RN source mode and added a Release post-build framework load closure gate. The original mismatch was directly visible in clean generated output: `Podfile.properties.json` enabled `EXPO_USE_PRECOMPILED_MODULES=true`, `Podfile.lock` contained React-Core 0.86.3 source and no React-Core-prebuilt, while the precompiled `ExpoModulesWorklets.framework` slice required `@rpath/React.framework/React`. `Pods-WeKnora-frameworks.sh` embedded Worklets but not React. Expo dependency alignment passed, which rules out package version drift but not packaging mode mismatch.

The supported `expo-build-properties` options now explicitly set `buildReactNativeFromSource: true` and `usePrecompiledModules: false`. The generated clean project selected source React and Expo modules (`EXPO_USE_PRECOMPILED_MODULES=false`); resulting app embeds ExpoModulesJSI and Hermes dynamic frameworks while React and ExpoModulesWorklets are linked from source/static Pods. The app executable and all embedded dynamic frameworks have no unresolved non-system `@rpath` dependencies. The script names missing framework dependencies and fails on `otool` inspection errors. Under source mode it also rejects accidental embedding of the precompiled ExpoModulesWorklets framework.

### Initial implementation TDD and checks (superseded by round-1 counts below)

- RED fixture contract: Worklets requiring absent React returns `React.framework`; complete graph returns no missing dependencies. Source/precompiled mode cases are pinned in `ios-framework-closure.test.ts`.
- Negative check: fixture graph reported `MISSING_FRAMEWORK_DEPENDENCY: ExpoModulesWorklets.framework requires React.framework` and exited 1.
- Focused framework and script tests: 7/7 pass.
- Full mobile suite: 303 total, 289 pass, 14 opt-in environment skips, 0 fail.
- Typecheck: `pnpm --filter @weknora/mobile typecheck` — pass.
- Expo alignment: `pnpm --filter @weknora/mobile exec expo install --check` — Dependencies are up to date.
- Diff: `git diff --check` — pass.

### Initial iOS 27 Release evidence

- Command: `IOS_BUILD_EVIDENCE_DIR=<worktree>/docs/testing/evidence/mobile-runtime-login/2026-09-29-r4 bash apps/mobile/scripts/ios-release-build.sh <worktree>`.
- Result: exit 0; `** BUILD SUCCEEDED **`, `FRAMEWORK_MODE=source-expo-modules`, `FRAMEWORK_CLOSURE_OK`, and the Release app path were emitted. Full script stdout/stderr is preserved in `docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/xcodebuild-release-final.log`; Xcode `tee` output is `xcodebuild-release.log`.
- Installed and launched on iPhone 18 Pro, iOS 27.0, UDID `0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC`. `simctl launch --console` remained attached and logged `ReactInstance: evaluateJavaScript() with JS bundle`; no dyld loader failure occurred.
- Screenshot: `docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/ios27-release-launch.png`, SHA256 `2bcd87c0d9e6c757c59348fcad03717466b724b5fe97c2dbb08a0d240ce25485`. Inspected image shows WeKnora sign-in root below status bar. Launch record is `launch.log`.
- T39 tracked build log restored/verified unchanged at SHA256 `12a27ed4f698074356c5e1d7722b5d7af78b2d60b5cdf9e9856e44e37b7bb71e`.

### Initial implementation changed files

- `apps/mobile/app.json`
- `apps/mobile/scripts/ios-release-build.sh`
- `apps/mobile/src/ios-framework-closure.ts`
- `apps/mobile/src/ios-framework-closure.test.ts`
- `apps/mobile/src/native-project-config.test.ts`
- `apps/mobile/src/scripts/ios-acceptance-scripts.test.ts`
- `.superpowers/sdd/plan-t31-ios27/fix-progress.md`
- This report and native evidence under `docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/`.

### Initial implementation notes

- Simulator emitted existing warnings for missing background fetch/remote notification UIBackgroundModes and duplicate accessibility classes in the iOS 27 simulator runtime; neither prevented launch or affected loader closure.
- The screenshot-only frontend-validator passed visible safe-area positioning; the independent task reviewer found actionable issues. See `.superpowers/sdd/plan-t31-ios27/fix-task-4-review.md` and the round-1 addendum below.

## Review repair round 1 addendum

Authority: `.superpowers/sdd/plan-t31-ios27/fix-task-4-r1-brief.md`; repaired findings 2–4 from `fix-task-4-review.md`. Per controller ruling in the brief, source-linked React is accepted when the actual Release load graph is closed and the app launches. No separate embedded `React.framework` claim is made. No native build was run in this round; the existing Release artifact was checked and launched unchanged.

- Replaced inline Python and disconnected TypeScript helper with `apps/mobile/scripts/verify-ios-framework-closure.py`, invoked by the production release script and exercised directly by integration tests.
- Checker parses app/framework `Info.plist` via standard `plistlib`, inspects the app executable and each framework's `CFBundleExecutable` with `otool -L`, checks requested binary names match declared executables, resolves targets within the app bundle, and fails closed on missing executable, missing `otool`, inspection failure, or unresolved framework loads.
- Focused command `node --import tsx --test src/ios-framework-closure.test.ts src/scripts/ios-acceptance-scripts.test.ts` (from `apps/mobile`): 8/8 passed. Cases cover complete closure, missing app direct load, missing framework binary despite directory, escaping symlink, and `otool` nonzero exit.
- Production check on retained artifact: `python3 apps/mobile/scripts/verify-ios-framework-closure.py apps/mobile/ios/build/Build/Products/Release-iphonesimulator/WeKnora.app` → `FRAMEWORK_MODE=source-expo-modules`, `FRAMEWORK_CLOSURE_OK`.
- Unchanged app executable SHA-256: `8d2141d06f3b6f0d185be79dff122e7fd0d102e901d72862bed7dec200e073e4`.
- Fresh install and launch output is captured in tracked `simctl-install-launch.txt` and `simctl-launch-r1.txt` (not a summary). Launch output includes `ReactInstance: evaluateJavaScript() with JS bundle`, PID, and no dyld error. The console command was bounded with `timeout 15s`, therefore wrapper exit 124 indicates expected termination of the still-attached console stream, while the app remained running.
- Fresh screenshot `ios27-release-launch-r1.png`, SHA-256 `6982dfa2728819aec625a1c85785a14271431e7d3135957d42fb2caab48cbd73`, inspected: sign-in content is visible below the status bar.
- Full outer Release log is persisted as tracked `xcodebuild-release-final.log.gz`; uncompressed SHA-256 `46f47407fb270ca1f8361a0442d32c5cbe184eaf7b1ed928420aa8afd1b3de83`, compressed SHA-256 `4d182b68807f739504742a13bb1dfb3c62208f0629d36eb6184da41ce8fa435d`. Original T39 tracked log remains SHA-256 `12a27ed4f698074356c5e1d7722b5d7af78b2d60b5cdf9e9856e44e37b7bb71e`.
- Full mobile suite `pnpm --filter @weknora/mobile test`: 306 total, 292 pass, 14 opt-in skips, 0 failures. `pnpm --filter @weknora/mobile typecheck`, `pnpm --filter @weknora/mobile exec expo install --check`, and `git diff --check` all pass.
- Repair implementation commit: `c4c16ad6cef751cec16e1c5bf22f694f3a83679e`.

## Review repair round 2 addendum

Authority: `.superpowers/sdd/plan-t31-ios27/fix-task-4-r2-brief.md`; addresses findings R4-R1-1 and R4-R1-2 in `.superpowers/sdd/plan-t31-ios27/fix-task-4-r1-review.md`. No native build was run and the evidence set was not modified.

- Framework executable declarations must resolve inside their own framework bundle. For every `@rpath/<Framework>.framework/<requested-path>` load, the requested path is resolved from that framework bundle and must be the same regular file as its resolved `CFBundleExecutable`. This accepts valid `Versions/A/<Executable>` paths while rejecting sibling bundle declarations, absent/mismatched paths, and symlinks outside the bundle.
- The checker now receives the generated `Podfile.properties.json` as its second argument from the production Release script. It requires recognized string values for `ios.buildReactNativeFromSource` and `EXPO_USE_PRECOMPILED_MODULES`, rejects missing or contradictory combinations, reports the configured mode, and rejects `ExpoModulesWorklets.framework` in source mode.
- Integration cases exercise the production Python checker with fake `otool`: complete closure, app direct dependency missing, framework binary missing despite directory, symlink escaping the app, sibling executable, valid versioned load path, requested binary mismatch, source/precompiled mode, missing/contradictory mode properties, source mode containing precompiled Worklets, and `otool` failure.
- Focused command from `apps/mobile`: `node --import tsx --test src/ios-framework-closure.test.ts src/scripts/ios-acceptance-scripts.test.ts` — 12/12 passed.
- Retained app check: `python3 apps/mobile/scripts/verify-ios-framework-closure.py apps/mobile/ios/build/Build/Products/Release-iphonesimulator/WeKnora.app apps/mobile/ios/Podfile.properties.json` → `FRAMEWORK_MODE=source-expo-modules`, `FRAMEWORK_CLOSURE_OK`. The executable SHA-256 remains `8d2141d06f3b6f0d185be79dff122e7fd0d102e901d72862bed7dec200e073e4`.
- Full suite: `pnpm --filter @weknora/mobile test` — 310 total, 296 pass, 14 opt-in skips, 0 fail. `pnpm --filter @weknora/mobile typecheck`, `pnpm --filter @weknora/mobile exec expo install --check`, and `git diff --check` all pass.
- The additional `apps/mobile/src/scripts/ios-acceptance-scripts.test.ts` change asserts the Release script passes both the `.app` and generated Podfile properties to the checked-in production checker.
- Round-2 repair commit: `0311f57e1f03ee54c7e1b967ba16dda92a92c603`.

## Review repair round 3 addendum

Authority: `.superpowers/sdd/plan-t31-ios27/fix-task-4-r3-brief.md`; closes low finding R4-R2-1 in the round-2 review. Only this report and `apps/mobile/src/ios-framework-closure.test.ts` changed; checker, scripts, generated project, and retained native evidence were untouched.

- Corrected sibling plist executable from the nonexistent `../Beta/Alpha` to `../Beta.framework/Alpha`, and the fixture creates that regular executable in the real sibling framework. The production checker rejects it with `FRAMEWORK_BINARY_OUTSIDE_BUNDLE`.
- Added a separate negative fixture with `Alpha.framework/Alpha` symlinked to the real `Beta.framework/Alpha` executable; the production checker rejects it with the same own-bundle containment error.
- Focused command from `apps/mobile`: `node --import tsx --test src/ios-framework-closure.test.ts src/scripts/ios-acceptance-scripts.test.ts` — 13/13 passed.
- Full suite: `pnpm --filter @weknora/mobile test` — 311 total, 297 pass, 14 opt-in skips, 0 fail.
- `pnpm --filter @weknora/mobile typecheck`, `pnpm --filter @weknora/mobile exec expo install --check`, and `git diff --check` all pass.
- No native build was run; existing runtime evidence and binary hash remain unchanged.
- Round-3 repair commit: `1818cd5a59fcbd1bc960e0b5c9ea9b78bbea02fc`.
