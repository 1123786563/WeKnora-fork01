# T01 iOS 27 Scene Lifecycle Follow-up Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to execute this plan task by task after approval. Each implementation task needs a fresh implementer and independent review.

**Goal:** Ensure the T01 WeKnora iOS app launches to its Deployment Login surface on iOS 27 by adopting Expo's supported UIKit scene lifecycle.

**Architecture:** Keep Expo Router and the existing Mobile Runtime composition. Upgrade the native toolchain from Expo SDK 55 to stable Expo SDK 57 scene support, opt in through Expo's `expo-build-properties` config plugin, and let Expo prebuild generate its supported `ExpoReactNativeFactoryProvider`/scene configuration. Do not maintain a hand-written SceneDelegate.

**Tech Stack:** Expo SDK `~57.0.23` or newer within SDK 57; `expo-build-properties` `~57.0.20` or newer within SDK 57; React Native `0.86.3` and React `19.2.3` as resolved by `expo install --fix`; iOS Simulator SDK 27; Node/pnpm workspace.

**Spec:** `docs/specs/2026-09-20-mobile-ai-office-design.md`; parent Ticket plan: `docs/superpowers/plans/2026-09-20-t01-mobile-runtime-login.md`; GitHub Ticket #31.

## Global Constraints

- WeKnora remains the only identity and Tenant authorization authority.
- The iOS app must keep using the existing Mobile Runtime and may show an authorized surface only after login, `/auth/me`, and capability negotiation succeed.
- Use Expo's supported scene lifecycle config plugin; do not hand-maintain generated `ios/` project files.
- `ios.enableSceneSupport` requires Expo `57.0.23+` and `expo-build-properties` `57.0.20+`.
- Set iOS deployment target to `16.0`, the minimum required by the generated Expo Router LinkPreview native source observed during the simulator build.
- Preserve Android support and all existing Expo app routes, custom URL scheme, SecureStore adapters, and package boundaries.
- No staging credentials are added to tests, source, logs, or the plan.

## Review Focus

- SDK drift must not leave any Expo module on an incompatible major; validate with `expo install --fix` and `expo-doctor`.
- Prebuild must actually emit `UIApplicationSceneManifest` and the supported Expo scene delegate; validating only `app.json` is insufficient.
- The scene delegate must still start the same `main` React Native module and route the custom `weknora://` auth callback once.
- Raising the iOS minimum must be explicit in Expo config and generated Pod/Xcode settings, not only a one-off CLI override.
- Native app startup must render Deployment Login; Expo JavaScript export success or a running process with a black screen does not count.

---

## File Structure

| Path | Responsibility |
|---|---|
| `apps/mobile/package.json` | SDK 57 and matching React Native/React/native module versions. |
| `apps/mobile/app.json` | Expo scene-support opt-in and explicit iOS deployment target. |
| `pnpm-lock.yaml` | Resolved workspace dependency graph. |
| `apps/mobile/src/native-project-config.test.ts` | Public config invariants for SDK floor, scene opt-in, and iOS minimum. |
| `docs/testing/mobile-runtime-login-device-acceptance.md` | Add the iOS 27 simulator build/install/start evidence and remaining real staging-device cases. |

Generated `apps/mobile/ios/` output is verification-only and must not be committed.

## Interface Contract

The existing runtime API and app routes remain unchanged. Expo prebuild must generate:

- `UIApplicationSceneManifest` with Expo's supported scene delegate class;
- an AppDelegate conforming to `ExpoReactNativeFactoryProvider` and no longer creating the React Native `UIWindow` in `didFinishLaunchingWithOptions`;
- iOS build settings with deployment target `16.0`.

## Implementation Tasks

### Task 1: Pin the scene-lifecycle config contract with a failing test

**Files:**
- Create: `apps/mobile/src/native-project-config.test.ts`
- Test: `apps/mobile/src/native-project-config.test.ts`

**Interfaces:**
- Consumes: `apps/mobile/app.json` and `apps/mobile/package.json`.
- Produces: a config-level assertion that SDK 57 scene support and the iOS 16 minimum are explicit.

- [ ] **Step 1: Write the failing config test.** Load the package and Expo config as JSON. Assert Expo SDK major/minor is 57 with patch at least 23; `expo-build-properties` is declared; the plugin config has `ios.enableSceneSupport === true` and `ios.deploymentTarget === "16.0"`.
- [ ] **Step 2: Run the test and verify RED.** Run `pnpm --filter @weknora/mobile exec tsx --test src/native-project-config.test.ts` from `apps/mobile`. It must fail on the current Expo 55 package and missing plugin config, not due to loader or path errors.
- [ ] **Step 3: Commit the failing test only** as `test(mobile): pin iOS scene lifecycle config`.

### Task 2: Upgrade to supported Expo scene lifecycle configuration

**Files:**
- Modify: `apps/mobile/package.json`
- Modify: `apps/mobile/app.json`
- Modify: `pnpm-lock.yaml`
- Test: `apps/mobile/src/native-project-config.test.ts`

**Interfaces:**
- Consumes: Task 1 config contract.
- Produces: SDK 57-compatible native dependencies; Expo prebuild opt-in for `EXExpoAppSceneDelegate`; explicit iOS 16 minimum.

- [ ] **Step 1: Upgrade the Expo SDK to `~57.0.23` and add Expo's matching `expo-build-properties` plugin** using `pnpm --filter @weknora/mobile exec expo install expo@~57.0.23 expo-build-properties`.
- [ ] **Step 2: Resolve compatible package versions** with `pnpm --filter @weknora/mobile exec expo install --fix`; keep all package versions in the same SDK 57 compatibility set.
- [ ] **Step 3: Configure the plugin** in `apps/mobile/app.json` with `ios.enableSceneSupport: true` and `ios.deploymentTarget: "16.0"`, then run the Task 1 test and verify GREEN.
- [ ] **Step 4: Run `pnpm install --frozen-lockfile`, `pnpm --filter @weknora/mobile typecheck`, and `pnpm --filter @weknora/mobile test`**; all must pass.
- [ ] **Step 5: Run `pnpm --filter @weknora/mobile exec expo-doctor`** and resolve SDK dependency compatibility findings without changing unrelated packages.
- [ ] **Step 6: Commit** the SDK/config/lockfile change as `fix(mobile): enable supported iOS scene lifecycle`.

### Task 3: Verify generated native project and iOS 27 startup

**Files:**
- Modify: `docs/testing/mobile-runtime-login-device-acceptance.md`
- Test: generated native project in an isolated temporary worktree; no generated iOS files committed.

**Interfaces:**
- Consumes: the Expo config and SDK 57 package graph from Task 2.
- Produces: evidence that generated scene configuration builds, installs, and renders Deployment Login on iOS 27.

- [ ] **Step 1: Generate the native project** with `pnpm --filter @weknora/mobile exec expo prebuild --clean --platform ios --no-install` in a disposable worktree.
- [ ] **Step 2: Assert the generated project contract.** `plutil` must find `UIApplicationSceneManifest`; generated `AppDelegate.swift` must use `ExpoReactNativeFactoryProvider`; generated native source must not create the legacy RN window in `didFinishLaunchingWithOptions`; the Pod/Xcode deployment target must be `16.0`.
- [ ] **Step 3: Build and install.** Run `pod install`, `xcodebuild ... -sdk iphonesimulator -destination 'platform=iOS Simulator,id=<iOS27 simulator>' CODE_SIGNING_ALLOWED=NO build`, then `xcrun simctl install` and `simctl launch`.
- [ ] **Step 4: Verify rendered surface.** Capture the simulator screen and assert visually that Deployment Login is visible rather than a black screen; inspect launch logs for absence of the UIScene lifecycle runtime issue.
- [ ] **Step 5: Re-run the existing mobile tests, shared typecheck, mobile typecheck, iOS export, and Android export** against the upgraded lockfile.
- [ ] **Step 6: Update the acceptance checklist** with iOS 27 simulator evidence, preserving real HTTPS staging password/OIDC tests and Android installation as pending if their environments remain unavailable.
- [ ] **Step 7: Commit** the acceptance evidence update as `docs(mobile): record iOS 27 scene startup evidence`.

## Self-Review

1. **Spec coverage:** The follow-up preserves Deployment, auth, capability, and safe-surface behavior from Ticket #31 while making native startup work on the current iOS SDK. It adds no product surface.
2. **Scope:** Only Expo SDK compatibility, scene configuration, generated native verification, and the existing acceptance checklist change.
3. **Conflict requiring approval:** This plan supersedes the parent plan's Expo SDK 55 version pin and raises the native iOS minimum to 16.0. The approved product spec does not specify either value; the parent implementation plan does. This amendment must be approved before implementation.
4. **Placeholder scan:** No unassigned code task remains. The simulator identifier is selected from `xcrun simctl list devices available` during execution.
5. **Review focus:** SDK compatibility, generated scene delegate, deep-link callback path, iOS minimum, and actual visible startup all have explicit verification owners.

## Handoff

Please review this plan amendment. Does it capture the intended iOS 27 compatibility change? After approval, execute it with `superpowers:subagent-driven-development` on a dedicated branch/worktree. Do not update the parent Ticket as complete until real staging login/OIDC and Android acceptance are also resolved.
