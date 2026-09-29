# Task Brief — T31 R4 clean Release framework closure

## Authority and evidence
- Issue #30 end-to-end authorization covers repairs needed to verify descendant Issue #31.
- Source findings: T31 R2 report `.superpowers/sdd/plan-t31-ios27/fix-task-2-report.md`; clean Release reported BUILD SUCCEEDED but `simctl launch --console booted com.weknora.mobile` exits with `dyld: Library not loaded: @rpath/React.framework/React` from `ExpoModulesWorklets.framework`.
- Captured app `apps/mobile/ios/build/Build/Products/Release-iphonesimulator/WeKnora.app/Frameworks` does not contain React.framework. Its generated `Pods-WeKnora-frameworks.sh` embeds ExpoModulesWorklets but not React.framework. `otool -L` confirms the ExpoModulesWorklets XCFramework slice requires React.framework/React.
- Expo dependency alignment check `pnpm --dir apps/mobile exec expo install --check` passed. Generated Podfile properties have `EXPO_USE_PRECOMPILED_MODULES=true`, but Podfile.lock/Pods project lack `React-Core-prebuilt`; React-Core 0.86.3 builds from source. This is evidence of mismatched packaging, not proof of a single root cause.
- Worktree `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-cont`; preserve existing commits and T39 tracked release log (its baseline SHA256 is recorded in R2 report). `apps/mobile/ios/` is generated and ignored.

## Owned files
- `apps/mobile/app.json`
- `apps/mobile/scripts/ios-release-build.sh`
- Focused config/script tests under `apps/mobile/src/` if needed
- `.superpowers/sdd/plan-t31-ios27/fix-progress.md`
- `.superpowers/sdd/plan-t31-ios27/fix-task-4-report.md`
- New native build/launch evidence under `docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/`

## Acceptance / interfaces
- Diagnose and document the Expo/RN precompiled vs source framework mismatch from generated CocoaPods output; do not assume that `EXPO_USE_PRECOMPILED_MODULES=false` alone fixes it.
- Make app configuration and Release pipeline produce a self-consistent framework graph. Prefer supported Expo/RN configuration. Do not add hand-copy steps for vendor frameworks or edit generated/ignored iOS files as the persistent fix.
- Add a deterministic post-build contract check: each non-system `@rpath/*.framework/*` load command of embedded dynamic frameworks must resolve inside the `.app/Frameworks` tree or to an explicitly documented app-bundled system/runtime framework. At minimum the ExpoModulesWorklets -> React dependency must be asserted. The release script must fail with the missing framework name before reporting success.
- Verify a deliberately missing required framework is rejected by the contract check.
- Canonical clean Release build on the booted iPhone 18 Pro iOS 27 simulator must produce a self-consistent framework closure, install, and launch. A separately embedded `React.framework` is not required when the supported source-build mode statically links React and removes the precompiled ExpoModulesWorklets → React runtime dependency. Save the actual app screenshot/log and inspect that Expo root renders without a native loader error. Credentials/login are out of scope.
- Keep R2 status blocked until this Task is verified, then perform the R2 Safe Area screenshot acceptance on the same Release artifact.

## TDD and verification
1. Add a failing checker test using fixture framework load-command data where a missing React.framework is detected; demonstrate RED.
2. Implement supported app configuration and pipeline guard; show GREEN for complete and incomplete fixture graphs.
3. Run focused tests, full mobile suite, typecheck, `expo install --check`, and `git diff --check`.
4. Run canonical `bash apps/mobile/scripts/ios-release-build.sh <worktree>`; preserve its output outside `docs/plans/issue30-sweep/ios-evidence/t39/xcodebuild-release.log` and restore/verify the tracked T39 file.
5. Install and launch via `xcrun simctl install booted <app>` and `xcrun simctl launch --console booted com.weknora.mobile`; capture only after successful app launch and inspect it.
6. Record exact commands, results, screenshot path, hashes, generated Pod lock/framework/embed-script evidence; commit locally. No push, merge, deploy, credentials, or Issue mutation.
7. Independent frontend validator checks app launch/screenshot; independent reviewer checks config and contract checker.

## Constraints
- Do not edit `apps/mobile/ios/` as the persistent change; it is generated and ignored.
- Do not overwrite or leave modifications in the unrelated tracked T39 release log.
- No subdelegation.
