# Task 0B / #145 report — Expo authenticated Task boundary

## Checkpoint

- Branch/worktree: `codex/issue-140-p0b-task` / `/Users/wuyongjun/.codex/worktrees/issue-140-p0b-task/WeKnora-fork01`
- Assigned starting HEAD: `2c4438af69b4ea60e5cf783f2e1318f3147d593c`; feature code baseline: `477da9f677e0357ea8bc360c2802314fef534723`.
- Scope is limited to `packages/mobile-core/src/task-office/**` and `apps/mobile/src/task-office/native-boundary/**`. Global Expo routes/registries/shared composition are integration-owner files.

## Implementation

- `task-detail.ts` captures the exact Runtime auth lease when a handle is created. Hydration/read rejects with `TASK_OFFICE_SCOPE_CHANGED` if the active lease changes, rather than adopting another tenant's authorization.
- The regression test opens an existing Task, switches tenant, and confirms stale handle rejection before another detail network read; same Task restoration remains supported.
- The native boundary resolves current authorization per operation, lists existing Tasks only, opens a selected existing `{taskId, runId}`, and has no create/start operation. It reports explicit states for navigation, file selection/download, system share, notifications and secure storage.
- `TaskEntryScreen` uses React Native primitives and `packages/design-tokens/src/mobile/native-tokens.ts`; loading, error/retry, empty and list states, accessibility roles/labels and touch sizes are represented. No mobile Web component imports.
- Capability/unit and import-boundary checks are included.

## Tests/typecheck

- RED was observed for missing native boundary module and for missing expected rejection on the cross-tenant stale-handle regression.
- `pnpm --filter @weknora/mobile exec tsx --test src/task-office/native-boundary/task-entry.test.ts`: **4 passed, 0 failed**.
- `pnpm --filter @weknora/mobile-core exec tsx --test src/task-office/task-detail.test.ts`: **47 passed, 0 failed**.
- `pnpm --filter @weknora/mobile test`: exit 0; **293 total, 279 passed, 14 skipped, 0 failed**. Skips are opt-in tests requiring live deployment credentials.
- `pnpm --filter @weknora/mobile typecheck`: exit 0 (`tsc --noEmit`). Captured output: `/tmp/issue-140-p0b-typecheck-final.log`.
- `git diff --check`: exit 0.
- Full-suite log: `/tmp/issue-140-p0b-mobile-test-final.log`.

## Native environment and compile probes

Host macOS 27.0 arm64; Expo 55.0.36; Xcode 27.0 (27A266a); iOS runtime 27.0; Android Emulator 37.2.4.0; ADB 1.0.41 / 37.0.1-15733141; Java Temurin 17.0.19; Android SDK `$HOME/Library/Android/sdk`, build-tools 35.0.0 and 36.0.0.

### iOS

- Expo `run:ios` prebuild/CocoaPods succeeded but Expo CLI stopped because `/Applications/Xcode.app/Contents/Developer/Applications/Simulator.app` is absent (`Can't determine id of Simulator app`), despite `xcrun simctl` and runtime availability.
- A dedicated iPhone 17e UDID `0D4ED257-7E47-40CE-A9D1-840116F6FC63` was booted for the probe. The shared already-booted iPhone 18 Pro UDID `0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC` was never installed/launched/captured; other Expo/Metro ownership could not be confirmed. Device probes show no remaining booted simulator currently.
- Direct build-only command from `apps/mobile`: `xcodebuild -workspace ios/WeKnora.xcworkspace -scheme WeKnora -configuration Debug -destination 'id=0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC' -derivedDataPath /tmp/issue-140-p0b-derived build`. The first invocation printed `** BUILD SUCCEEDED **` and produced an app artifact under `/tmp/issue-140-p0b-derived/Build/Products/Debug-iphonesimulator/WeKnora.app` (activity log `.../Logs/Build/AFC053F4-CD52-4E61-BA19-455639B99541.xcactivitylog`). A later incremental/quiet invocation's `/tmp/issue-140-p0b-ios-build.log` has `error: the following command failed with exit code 0 but produced no further output` during `SwiftCompile normal arm64 (target WeKnora)`. Therefore repeatable iOS build status is **inconsistent**, not a clean verified gate. Expo CLI could not manage the Simulator.app, and there is no Task route yet; no install/launch/screenshot was done.

### Android

- SDK is installed; although shell PATH/env defaults initially omitted it, explicit `$HOME/Library/Android/sdk` `adb` works. AVD `test36-small` is configured Android 36 Google APIs arm64, 720×1440 320 dpi. Initial device was `emulator-5554` (`model:sdk_gphone64_arm64`).
- Expo `run:android --variant debug --no-bundler` prebuild succeeded but first attempt ended while the emulator was still offline.
- First direct Gradle build failed resolving Kotlin 2.1.20 artifacts from Maven Central with TLS `bad_record_mac` / `Tag mismatch!`; this was a transport failure, not a source compile diagnostic. The authoritative parent rerun was `ANDROID_HOME="$HOME/Library/Android/sdk" ANDROID_SDK_ROOT="$HOME/Library/Android/sdk" PATH="$HOME/Library/Android/sdk/platform-tools:$HOME/Library/Android/sdk/emulator:$PATH" ./gradlew --no-daemon assembleDebug` in `apps/mobile/android`, shell session 22546. It terminated after 10m36s with `BUILD FAILED`, exit nonzero, while resolving `com.squareup:javapoet:1.13.0` from `https://plugins.gradle.org/m2/...`; the remote terminated the TLS handshake. This is dependency transport failure, not an application source compile diagnostic. No APK was produced; Android build gate is unresolved.
- No APK was installed/launched. No shared device was modified.

## Authenticated/UI gate and integration contract

- `WEKNORA_MOBILE_TEST_DEPLOYMENT_URL`, `_EMAIL`, `_PASSWORD` are absent. No real authenticated deployment read or tenant-switch service interaction is claimed.
- The owned reusable screen is not routed yet by design. Integration owner: add `/task-office` in global Expo Router registry; for each request resolve current Runtime snapshot, `scopeLease()` and `activeTaskOffice()` into `createAuthorizedTaskEntry({ current })`; call `listExisting()` for the screen and `detectNativeTaskCapabilities()` for capability display; selected `{taskId,runId}` navigates to `/tasks/detail`. Key/unmount the screen on deployment-origin + tenant change. Capture interaction/screenshot evidence on dedicated iPhone 17e and Android AVD with isolated Metro port and evidence directories. Do not use the shared iPhone 18 Pro without explicit ownership.
- Accordingly #145 remains **not verified** until both target native UI interaction probes and screenshots, and an authenticated existing-Task read/scope probe, are performed. iOS Simulator.app absence and Android Gradle result remain separate platform build gates.

## Changed files and commit

- Commit: `4682dcf31e3ac3dfb8c87adeb0d49b57c8ce8944` (`feat(mobile): add native authorized Task Office boundary`). No push.
- `git status --short` is clean after commit. Report is ignored by `.gitignore` (`.*`) and remains available in the shared worktree for the parent ledger.
