# Task Brief — Issue #31 iOS 27 scene lifecycle

## Facts and scope

Root Issue #30 → DAG node #31/T01/B0. Read `CONTEXT.md`, `docs/specs/2026-09-20-mobile-ai-office-design.md`, `docs/adr/0005-weknora-native-mobile-client.md`, `docs/plans/issue30-sweep/issues/issue-31.md`, `docs/plans/issue30-sweep/dag.md`, `docs/superpowers/plans/2026-09-20-t01-mobile-runtime-login.md`, and current execution plan `docs/plans/issue30-sweep/plans/plan-t31-ios27.md`.

The ticket implementation already exists on BASE `db234c5`; its confirmed iOS 27 defect is a black screen because UIKit requires scene lifecycle support. The approved product spec requires real-device native behavior but does not pin Expo or iOS minimums. The execution plan amendment uses Expo SDK 57 scene support. Current official Expo docs state SDK57 minimum iOS 16.4+, and recommend top-level Expo `ios.deploymentTarget` rather than deprecated build-properties deploymentTarget.

## Workspace

- Worktree `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-b0-t31`
- Branch `codex/issue30-b0-t31`
- BASE `db234c5eb171f2dde7427d382b55b503a038f879`
- Report `.superpowers/sdd/plan-t31-ios27/task-report.md`; exact code patch + SHA also under this directory.
- Local commit authorized; no push/merge/deploy/Issue mutation. Do not commit generated `apps/mobile/ios/` or Android native output.

## Ownership and intended interface

- Role: frontend_implementer. Validator: frontend_validator. Independent reviewer: reviewer.
- Owned files: `apps/mobile/package.json`, `apps/mobile/app.json`, `pnpm-lock.yaml`, new `apps/mobile/src/native-project-config.test.ts`, `apps/mobile/src/android-release-config.test.ts` (only its Expo dependency version assertions), `apps/mobile/src/release-deps.test.ts` (only audio/network SDK-major assertions), and `apps/mobile/src/app-smoke.test.tsx` only if needed for its hand-written React stub to support the selected React 19.2.3 JSX runtime; preserve all existing assertions, and `docs/testing/mobile-runtime-login-device-acceptance.md`.
- Expected config: Expo SDK `~57.0.23` or compatible newer SDK57 patch; `expo-build-properties` version compatible with scene support; plugin has `ios.enableSceneSupport=true`; top-level `expo.ios.deploymentTarget="16.4"`.
- Preserve WeKnora Mobile Runtime, auth/capability gates, routes, custom scheme, Android behavior, workspace boundaries. No handwritten SceneDelegate or committed generated iOS output.

## Steps (RED → GREEN → REFACTOR)

1. RED: create config test reading package/app JSON; assert Expo major/minor 57 and patch >=23, build-properties dependency exists, plugin scene-support true, top-level deployment target 16.4. Run `pnpm --filter @weknora/mobile exec tsx --test src/native-project-config.test.ts`; record the expected assertion failure (must not be a loader error).
2. Upgrade Expo/build-properties using `pnpm --filter @weknora/mobile exec expo install expo@~57.0.23 expo-build-properties`, then `pnpm --filter @weknora/mobile exec expo install --fix`. Keep one compatible SDK57 dependency set and inspect lockfile impact.
3. Configure plugin with `ios.enableSceneSupport=true`; set top-level `expo.ios.deploymentTarget="16.4"`. Run config test GREEN. Update `android-release-config.test.ts` to assert the selected SDK57 versions for `expo-audio`, `expo-network`, and `expo-file-system`; these are existing direct dependency contracts, not unrelated test cleanup.
4. Run `pnpm install --frozen-lockfile`, mobile typecheck and test, `pnpm --filter @weknora/mobile exec expo-doctor`; resolve SDK compatibility findings within owned files/dependency set.
5. In isolated temporary directory/worktree (do not alter deliverable generated files), run `expo prebuild --clean --platform ios --no-install`; verify `UIApplicationSceneManifest`, Expo scene delegate and `ExpoReactNativeFactoryProvider`, no RN `UIWindow` creation in `didFinishLaunchingWithOptions`, and all Pod/Xcode target values 16.4.
6. Install pods and build for booted iOS 27 simulator UUID `0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC`; install/launch app, capture screenshot and inspect the rendered Deployment Login instead of a black screen. Report exact xcodebuild/simctl commands and logs. Do not claim backend login/OIDC success without HTTPS staging credentials.
7. Re-run mobile tests, typecheck, iOS export, Android export; update `docs/testing/mobile-runtime-login-device-acceptance.md` with simulator evidence and pending external staging/OIDC/Android cases.
8. Commit implementation, generate exact BASE-to-code patch (exclude report metadata) and SHA-256. Report all verification outcomes and any environment limitations.

## Acceptance / review focus

- Generated project—not just `app.json`—contains scene manifest and supported Expo delegate.
- iOS 27 simulator visibly renders Deployment Login, with no UIScene lifecycle runtime failure.
- SDK compatibility checks pass; deployment target is 16.4 in Expo config and generated Pod/Xcode project.
- Existing app/runtime routes, auth callback, SecureStore, and Android export remain intact.
- No credentials added/logged; real HTTPS auth/OIDC and Android device acceptance remain pending if no resources supplied.
