# Task R2 report — safe-area root shell

## Outcome
Implemented centralized safe-area protection for routed screens. `RootLayout` now nests the existing header-hidden `Stack` in `SafeAreaProvider` and `SafeAreaView` with top and bottom edges. No inner screen padding was added and routing/header options remain the same.

## TDD and verification
- RED: `pnpm --filter @weknora/mobile exec tsx --test src/app-smoke.test.tsx` failed on the newly added shell assertion because the original root had no safe-area provider (73 pass, 1 fail).
- GREEN focused: `pnpm --filter @weknora/mobile exec tsx --test src/app-smoke.test.tsx` — 74/74 pass.
- Full suite: `pnpm --filter @weknora/mobile test` — 301 total, 287 pass, 14 opt-in staging/environment skips, 0 fail.
- Typecheck: `pnpm --filter @weknora/mobile typecheck` — pass.
- Diff: `git diff --check` — pass.

## iOS Release and runtime evidence
Command: `bash apps/mobile/scripts/ios-release-build.sh /Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-cont`.

Clean Expo prebuild passed the generated SDK57 scene contract; CocoaPods installed `react-native-safe-area-context (5.7.0)`; Xcode Release completed with `** BUILD SUCCEEDED **`. The log is `docs/testing/evidence/mobile-runtime-login/2026-09-29-r2/xcodebuild-release.log` (SHA-256 `9f484fd2704cdf9a028dfa088f6f00a0791ff76330c6f3f296df256b0837db0b`).

Installed the result on iPhone 18 Pro, iOS 27.0, UDID `0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC`. `xcrun simctl launch --console ... com.weknora.mobile` showed immediate dyld termination: `Library not loaded: @rpath/React.framework/React`, referenced from `ExpoModulesWorklets.framework`. The generated app has no `React.framework` under `Frameworks/`. Therefore no app UI could be captured; the generated `ios27-release-safe-area.png` is the SpringBoard and is explicitly not acceptance evidence. A fresh Release screenshot proving field bounds remains blocked by native package startup, which is outside the R2-owned files.

The canonical build script temporarily rewrote the tracked T39 log. Restored it from HEAD and verified the original SHA-256 `12a27ed4f698074356c5e1d7722b5d7af78b2d60b5cdf9e9856e44e37b7bb71e`; this task's build log is stored separately as above.

## Changed source
- `apps/mobile/src/app/_layout.tsx`
- `apps/mobile/src/app-smoke.test.tsx`

## Patch hash
Source SHA-256: `_layout.tsx` `1dcb178d008ad9418a1a394d0fe38dd49593232b458a4a6de5f7508c19e9744a`; `app-smoke.test.tsx` `48c15e53741c991a7eb0ddac719b95346a309cf41e12c8030dee439b5ef4a7b5`.
