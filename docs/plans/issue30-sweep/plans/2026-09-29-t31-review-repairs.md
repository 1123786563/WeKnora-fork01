# Issue #31 Review Repair Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve the two medium review findings and the low scene-test coverage finding from the independent review of Issue #31's SDK57 iOS 27 startup change.

**Architecture:** Keep Expo's generated iOS project as the native lifecycle source of truth. Make the supported Release script cleanly regenerate native files, centralize top safe-area treatment at the Expo Router shell, and replace assertions about the retired SDK55 config plugin with a generated SDK57 project contract exercised by the build script.

**Tech Stack:** Expo SDK57, Expo Router, expo-build-properties, React Native 0.86, Node test runner, Bash, CocoaPods, Xcode.

**Spec:** `docs/specs/2026-09-20-mobile-ai-office-design.md`; approved seams: `docs/specs/2026-09-20-mobile-module-seams.md`; ADR `docs/adr/0005-mobile-app-boundaries-and-runtime-session-ownership.md`; source Issue #31 snapshot `docs/plans/issue30-sweep/issues/issue-31.md`; original implementation plan `docs/plans/issue30-sweep/plans/plan-t31-ios27.md`; independent review `.superpowers/sdd/plan-t31-ios27/review.md`.

## Global Constraints

- SDK57 generated `EXExpoAppSceneDelegate` and `ExpoReactNativeFactoryProvider` remain responsible for scene startup and URL lifecycle.
- Preserve `expo.ios.deploymentTarget` at `16.4` and `expo-build-properties.ios.enableSceneSupport=true`.
- Keep `apps/mobile/ios/` ignored/generated and do not commit generated native files.
- Keep staging authentication/OIDC and Android device acceptance recorded as pending without credentials or devices.
- No source or acceptance claim may exceed its test/build/screenshot evidence.

## Review Focus

- Stale ignored SDK55 `apps/mobile/ios/` tree — script must regenerate SDK57 scene manifest/delegate before pods and build; test both clean and stale generated-tree input.
- iPhone top safe area — title and first form controls must begin below status bar/notch; pin with an app-shell/screen structure test and fresh Release screenshot.
- Generated native drift — clean generated manifest/delegate/factory, iOS target and app URL callbacks must be checked automatically, not by tests for removed plugin transforms.

## DAG and ownership

| ID | Depends on | Owner role | Validator | Owned files | Interface / acceptance | Status |
|---|---|---|---|---|---|---|
| R1 | none | frontend_implementer | reviewer | `apps/mobile/scripts/ios-release-build.sh`, `apps/mobile/src/scripts/ios-acceptance-scripts.test.ts` | Clean prebuild; remove stale SDK55 comments; release script test requires clean and correct build order | pending |
| R2 | none | frontend_implementer | frontend_validator | `apps/mobile/src/app/_layout.tsx` or shared mobile shell and focused test | Shared safe-area provider/inset; simulator screenshot shows title/form below status bar | pending |
| R3 | R1 | frontend_implementer | reviewer | `apps/mobile/src/native-project-config.test.ts`, `apps/mobile/src/plugins/ios-xcode27.test.ts`, `apps/mobile/scripts/ios-release-build.sh` | Retire SDK55 plugin behavior tests; run generated SDK57 project assertions during Release script after clean prebuild | pending |

Task R2 is independent of R1/R3 and can be reviewed in the same repair round after R1/R3 land. R3 consumes the clean generated-project contract produced by R1. No cyclic dependencies.

## SDD execution ledger

- Workspace: `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-cont`
- Root BASE: `db234c5eb171f2dde7427d382b55b503a038f879`; T31 implementation currently integrated at `39c1359ed2f7a8a8031091edbd712330930df780`, evidence at `62ee15032`.
- Commit strategy: local task commits authorized; no push, merge-to-main, deploy, or Issue mutation.
- Review source: `.superpowers/sdd/plan-t31-ios27/review.md`; findings R1 Medium, R2 Medium, R3 Low remain unaddressed at plan creation.
- Statuses pending until each task implementation is independently reviewed and validated in this worktree.

## Task R1: Clean SDK57 prebuild in the supported Release script

**Files:** modify `apps/mobile/scripts/ios-release-build.sh`, `apps/mobile/src/scripts/ios-acceptance-scripts.test.ts`.

**Consumes:** SDK57 app config and the reviewer-confirmed generated scene contract. **Produces:** deterministic script preflight that removes ignored stale native output and executes `npx expo prebuild -p ios --clean --no-install` before `pod install`.

1. Add/adjust a focused script source test to fail unless the prebuild command includes `--clean`, no removed plugin is named as a source of truth, and prebuild precedes Pods and Release build.
2. Run that test before production edits and record RED.
3. Change prebuild to `--clean`; update stale comments about the SDK55 plugin, Podfile clamp, and its splash edits. Keep existing dependency verification and build-output assertions.
4. Run focused `pnpm --filter @weknora/mobile exec tsx --test src/scripts/ios-acceptance-scripts.test.ts`; expect all tests pass. Run `git diff --check` on production paths.
5. Commit `fix(mobile): cleanly regenerate iOS release project`.

**Failure handling:** If clean prebuild breaks codegen/pods, retain generated output only in ignored worktree and report exact command/log. Do not revert to stale prebuild to pass.

## Task R2: Safe-area layout for deployment login

**Files:** modify `apps/mobile/src/app/_layout.tsx` or its focused shared shell component and add/update the relevant app smoke test.

**Consumes:** Expo Router root stack; `react-native-safe-area-context` is already a direct dependency. **Produces:** a provider and safe top inset applied consistently to routed mobile screens.

1. Add a focused rendering/structure assertion that root content is wrapped in the safe-area provider and safe container.
2. Confirm the test fails on the current header-hidden bare stack/root layout.
3. Apply `SafeAreaProvider` + a screen shell with `SafeAreaView` edges that protect top/bottom system areas while preserving scroll/input behavior. Avoid stacking duplicate insets inside nested screens.
4. Run focused `pnpm --filter @weknora/mobile exec tsx --test src/app-smoke.test.tsx`, full mobile suite and `pnpm --filter @weknora/mobile typecheck`; expect no failures.
5. Build/install/launch Release on the booted iOS27 iPhone 18 Pro simulator and visually inspect a new screenshot: title and deployment field must sit below status icons/notch. Record screenshot and command/log.
6. Commit `fix(mobile): respect safe area on deployment login`.

**Failure handling:** If safe-area wrapping causes doubled padding on nested navigation, move the inset to the narrowest common routed content shell and repeat the focused screenshot check.

## Task R3: Generated SDK57 scene contract and retired test cleanup

**Files:** modify `apps/mobile/src/native-project-config.test.ts`, `apps/mobile/src/plugins/ios-xcode27.test.ts`, `apps/mobile/scripts/ios-release-build.sh`.

**Consumes:** R1's clean generated project before CocoaPods. **Produces:** automated assertion over generated Info.plist/AppDelegate/Xcode project for scene manifest, Expo delegate/factory, app callback URL forwarders, and 16.4 deployment target.

1. Replace JSON-only test shape with a scriptable contract helper that accepts the generated iOS directory and checks Info.plist `UIApplicationSceneManifest` contains `EXExpoAppSceneDelegate`, AppDelegate conforms to `ExpoReactNativeFactoryProvider` and retains `RCTLinkingManager` callbacks, and Xcode project/Podfile all use 16.4.
2. Add a focused fixture test for valid generated output and negative cases for missing scene delegate, URL callback, or wrong deployment target. It must not import or execute `plugins/ios-xcode27.js`.
3. Delete or rewrite `ios-xcode27.test.ts` so no test asserts transformations of the retired SDK55 plugin. The unused plugin may remain in the repository only if no active config/script claims it as lifecycle source of truth; do not re-add it to app.json.
4. Wire the contract check into `ios-release-build.sh` after clean prebuild and before Pods. Run it on generated output and verify clean prebuild/build sequence.
5. Run focused native-project and acceptance script tests, full mobile suite, typecheck; expect green, with only declared env-gated skips. Run clean Release script once and a pre-created stale SDK55 output simulation to ensure script deletes it and resulting project carries the current contract. Capture simulator Release startup screenshot for R2.
6. Commit `test(mobile): verify generated SDK57 iOS scene project`.

**Failure handling:** If Expo-generated template details differ from current SDK57 output, inspect generated artifacts and update the contract to the observed supported delegate/callback shape; do not hardcode SDK55 source transformations.

## Integration and Review

- Topological order R1 → R3; R2 can run after R1/R3 are integrated in the same repair worktree so its screenshot uses the clean release path.
- Keep code changes in their scoped files; after each task bind tests to the exact task commit and request independent review.
- Re-run whole mobile suite/typecheck and full source-only diff check after all repairs; preserve env-gated skip counts.
- Mark verified only after reviewer PASS and matching evidence. Record report/package hashes and review result below.

## Repair ledger

- (pending)
