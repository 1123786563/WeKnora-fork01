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
| R1 | none | frontend_implementer | reviewer | `apps/mobile/scripts/ios-release-build.sh`, `apps/mobile/src/scripts/ios-acceptance-scripts.test.ts` | Clean prebuild; remove stale SDK55 comments; release script test requires clean and correct build order | implemented; prebuild blocked by missing workspace plugin resolution |
| R2 | R4 | frontend_implementer | frontend_validator | `apps/mobile/src/app/_layout.tsx`, `apps/mobile/src/app-smoke.test.tsx` | Shared safe-area provider/inset; simulator screenshot shows title/form below status bar | implemented; focused/full tests pass, runtime acceptance awaits R4 |
| R3 | R1 | frontend_implementer | reviewer | `apps/mobile/src/native-project-config.test.ts`, `apps/mobile/src/plugins/ios-xcode27.test.ts`, `apps/mobile/scripts/ios-release-build.sh` | Retire SDK55 plugin behavior tests; run generated SDK57 project assertions during Release script after clean prebuild | implemented; fixture tests pass, generated output validation blocked with R1 |
| R4 | R1 | frontend_implementer | reviewer + frontend_validator | `apps/mobile/app.json`, `apps/mobile/scripts/ios-release-build.sh`, focused script/config tests and generated evidence | Clean generated Release has complete non-system framework closure and launches on iOS27; source-linked React is allowed when the supported Expo source mode removes the dynamic React dependency; the production guard checks the app executable and embedded framework binaries and fails closed | implementation and three scoped review-repair rounds verified; T31 integrated final review pending |

Task R2 is independent of R1/R3 and can be reviewed in the same repair round after R1/R3 land. R3 consumes the clean generated-project contract produced by R1. R4 repairs the runtime packaging failure exposed while verifying R2; R2 runtime validation consumes R4's verified Release output. No cyclic dependencies.

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

## Task R4: Clean Release framework closure

**Files:** modify `apps/mobile/app.json`, `apps/mobile/scripts/ios-release-build.sh`, and focused mobile config/script tests; add generated build/launch evidence under `docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/`.

**Consumes:** exact dyld failure from Task R2 report and current clean Pods/App artifacts. **Produces:** a consistent Expo/RN native framework graph and a Release guard that validates every app-bundled non-system dynamic framework load command before success.

1. Reproduce and inspect Podfile properties, `Podfile.lock`, Pods project targets, Embed Pods Frameworks script, and ExpoModulesWorklets `otool -L`; record the specific source/prebuilt mismatch.
2. Add a fixture test proving the proposed dependency-closure checker fails for missing React.framework and passes when the required framework is embedded; demonstrate RED.
3. Change only supported app configuration and the canonical Release script. Do not patch generated `ios/`, vendor XCFramework binaries, or hide the error by copying frameworks manually.
4. Run focused tests, full mobile suite, typecheck, Expo dependency check and diff-check. Clean-build; verify `React.framework` is in the app and `simctl launch --console` starts successfully on iOS27. Preserve the T39 log baseline.
5. Commit locally and request independent frontend runtime validation plus code review. R2 then consumes this exact verified build for Safe Area screenshot acceptance.

**Failure handling:** If the correct Expo/RN build mode is incompatible or cannot be established from versioned configuration/API, report the exact evidence and leave R2 runtime pending. Do not introduce a source-built/vendor binary copy fallback.

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

### R4 review repair round 1

- Source: `.superpowers/sdd/plan-t31-ios27/fix-task-4-review.md`; review package is `003f12cc1..eb821ee9d`, SHA256 `0e6c31b7f75fd2c61036356f0045317ef13f512b842c67502b8d785eae4a08e2`.
- Ruling: correct the task-brief requirement for a separately embedded `React.framework`; the approved Issue/spec/ADR require a working runtime, not a dynamic packaging form. The chosen Expo source mode is supported and the actual app launched. Keep closure and launch as mandatory acceptance. Cost if wrong: hidden release policy may require a standalone framework; none is documented in approved requirements.
- Valid findings to repair together: inspect the main app executable; require each referenced framework's actual CFBundleExecutable file; make tests invoke the same checked-in checker as the Release script; preserve raw build/launch evidence in tracked artifacts and bind it to the Release executable hash.
- Implementation Brief: `.superpowers/sdd/plan-t31-ios27/fix-task-4-r1-brief.md`.
- Status: implementation in commits `c4c16ad6c` and `d4e15789d`; subsequent rounds 2–3 verified below.
- Review round 1: Spec PASS / Code Quality FAIL. Medium R4-R1-1: `CFBundleExecutable` can resolve into a sibling bundle under `.app/Frameworks` while the requested load path does not exist. Low R4-R1-2: mode is inferred from Worklets presence. The reviewer also recorded a valid versioned-path false rejection. See `.superpowers/sdd/plan-t31-ios27/fix-task-4-r1-review.md`.
- Repair round 2 brief: `.superpowers/sdd/plan-t31-ios27/fix-task-4-r2-brief.md`; changes must reject sibling-bundle paths, resolve requested load suffix to the declared executable, accept valid versioned paths, and derive mode from generated Podfile properties.
- Status: superseded by the round 2–3 repair and review records below; no native rebuild was run for checker/test repairs.

### R4 review repair rounds 2–3

- Round 2 code/report commits: `0311f57e1` / `10c05c1e7`. Production checker now binds each load path to the bundle-declared executable and reads mode from generated Podfile properties. Implementation reports focused 12/12, full 310 total / 296 pass / 14 gated skips, typecheck, Expo check, retained Release checker and diff-check passing; no build/evidence changes.
- Round 2 review `.superpowers/sdd/plan-t31-ios27/fix-task-4-r2-review.md`: production findings addressed; Spec FAIL / Quality PASS due Low R4-R2-1. Sibling fixture targeted `../Beta/Alpha` rather than the existing `Beta.framework/Alpha`; no sibling-bundle symlink fixture.
- Round 3 brief `.superpowers/sdd/plan-t31-ios27/fix-task-4-r3-brief.md` owns only checker integration tests/report. Correct the real sibling framework path and add sibling symlink coverage; no native build. T31 final review remains gated on scoped round-3 review.
- Round 3 implementation commits `1818cd5a5` / `196f85d02` corrected the plist path to a real `Beta.framework/Alpha` executable and added a framework-local symlink to that same file. Focused 13/13; full suite 311 total / 297 pass / 14 opt-in skips; typecheck, Expo check and diff-check pass per report. No native build/evidence changes.
- Round 3 independent review `.superpowers/sdd/plan-t31-ios27/fix-task-4-r3-review.md`: Spec PASS / Quality PASS, no findings. R4 repair task verified; T31 integrated final review remains next.

### T31 integrated final review findings

- Exact final review slice is `1e9315773a971dd72fe621c94e308f45a0ca4692..894d456cbf1c3506563204b4e018de02ce4e6700`; read-only report `.superpowers/sdd/plan-t31-ios27/final-review.md`.
- Spec Partial / Code Quality Changes Requested. T31-F1 Medium: production framework gate skips unresolved `@loader_path` and `@executable_path` framework load commands; disposable fixture reproduced a false pass. The retained Release app itself has no observed missing dependency and still launches.
- T31-F2 Medium: `docs/plans/issue30-sweep/plans/plan-t31-ios27.md` is cited as current authority but absent. T31-F3 Low: device acceptance summary has stale “JS startup unresolved” heading despite later launch evidence.
- Follow-up plan: `docs/plans/issue30-sweep/plans/2026-09-29-t31-final-review-repairs.md`, base `894d456cbf1c3506563204b4e018de02ce4e6700`. FR1/DOC1/DOC2 are independent; T31 cannot be released to dependent #32 until each is verified, scoped-reviewed, integrated and final T31 review passes.

## Task R3-F1: Tighten effective generated-project checks (review repair round 1)

**Finding:** `.superpowers/sdd/plan-t31-ios27/fix-task-1-review.md` F1 Medium. Current checker can accept wrong effective `UISceneDelegateClassName`, URL markers in the wrong callback, and a single changed target configuration.

**Files:** `apps/mobile/scripts/verify-ios-scene-project.ts`, `apps/mobile/src/plugins/ios-xcode27.test.ts`, `apps/mobile/src/native-project-config.test.ts` only if shared contract fixture needs it; report/ledger files.

**Consumes:** Actual clean SDK57 generated tree `apps/mobile/ios` from prior verified prebuild. **Produces:** relationship-aware, fail-closed contract with regression fixtures.

1. Strengthen fixture with actual plist application-role scene configuration and realistic AppDelegate declaration containing separate open-URL and continue-user-activity bodies, plus multiple app target configurations.
2. Add RED fixtures: delegate class marker exists outside the application scene mapping; open-URL callback does not forward even though RCTLinkingManager remains in continue-user-activity; one of the app's deployment entries is 16.0 while remaining ones are 16.4.
3. Parse plist structurally (Node built-in/XML parsing or focused structural parser, no new runtime dependency) and require the application role maps to `EXExpoAppSceneDelegate`. Match each specific callback body and require URL forwarding in both required callback pathways. Collect all iOS deployment-target entries for the app target/configurations and reject any value other than 16.4; do not impose unrelated Pods settings if Expo may legitimately vary them.
4. Run focused tests, full mobile suite and typecheck. Run verifier against actual ignored SDK57 clean prebuild output. Capture report/package hash; commit fix.
5. Request independent scoped re-review; do not release R2 until R3-F1 passes or is explicitly adjudicated.

**Status:** R1 and R3 verified. R3-F1/F2 received five scoped repair rounds; final independent high-reasoning review is PASS with two low documented limits. R2 is ready.

## Task R3-F1: Include every app configuration and pin counterexamples (repair round 2)

**Source:** `.superpowers/sdd/plan-t31-ios27/fix-task-1-review.md` follow-up review message after `54a7dd071`; remaining Medium configuration blind spot plus low negative-fixture gaps.

**Files:** `apps/mobile/scripts/verify-ios-scene-project.ts`, `apps/mobile/src/plugins/ios-xcode27.test.ts`, repair report/ledger only.

**Change:** Parse every object ID in the WeKnora native target's `buildConfigurations` array without relying on inline Debug/Release labels; resolve each ID to its `XCBuildConfiguration` and reject missing or non-16.4 deployment value. Extend synthetic project with a third `Staging` configuration and negative drift there. Add negative plist where the Expo delegate marker occurs outside application-role scene mapping, and universal-link override without RCT forwarding while open-URL still forwards.

**Verification:** RED/GREEN focused `ios-xcode27.test.ts`; full mobile test; typecheck; `verify-ios-scene-project.ts ios`; source diff-check. Commit, report/hash, scoped reviewer re-review before releasing Task R2.

**Status:** pending.

## Task R3-F1: Read actual PBX build settings, not comment text (repair round 3)

**Source:** `.superpowers/sdd/plan-t31-ios27/fix-task-1-r2-review.md` F1 Medium. A commented-out `IPHONEOS_DEPLOYMENT_TARGET = 16.4;` can satisfy the raw setting regex when a referenced configuration has no actual value.

**Files:** `apps/mobile/scripts/verify-ios-scene-project.ts`, `apps/mobile/src/plugins/ios-xcode27.test.ts`, report/ledger only.

**Acceptance:** For each actual `XCBuildConfiguration` object referenced by the app target, extract its `buildSettings` dictionary; strip PBX `/*...*/` comments before matching keys; require a concrete non-comment `IPHONEOS_DEPLOYMENT_TARGET` of 16.4. Missing config object, missing buildSettings, missing key, or wrong value must fail. Add test whose Staging config has only a comment containing the expected value. Retain arbitrary-name and scene/callback counterexamples.

**Verification:** prove new fixture fails on previous helper; focused/full mobile tests, typecheck, actual generated SDK57 project checker, source diff-check; update report/hash and commit; scoped re-review required before Task R2.

**Status:** pending.

## Task R3-F2: Ignore Swift comments when checking URL forwarding (repair round 4)

**Source:** `.superpowers/sdd/plan-t31-ios27/fix-task-1-r3-review.md` F2 Medium. Callback checks can match `RCTLinkingManager.application` in a Swift comment while the actual call is absent.

**Files:** `apps/mobile/scripts/verify-ios-scene-project.ts`, `apps/mobile/src/plugins/ios-xcode27.test.ts`, report/ledger.

**Acceptance:** Strip Swift line/block comments (preserving quoted string content enough to avoid deleting protocol URL strings) before testing each independently extracted callback body. Add two negative fixtures: open-URL contains only commented forwarding, universal-link contains only commented forwarding while open-URL remains valid. Existing valid fixture and actual generated tree pass.

**Verification:** RED/GREEN focused native contract tests; full mobile suite; typecheck; actual generated SDK57 checker; source diff-check. Commit and send high-reasoning independent review. Status pending.

## Task R3-F2: Distinguish executable Swift calls/signatures (repair round 5, final)

**Source:** `.superpowers/sdd/plan-t31-ios27/fix-task-1-r4-review.md` Medium findings: call marker in a string literal passes; a commented open-URL signature can be selected before a universal-link method.

**Files:** `apps/mobile/scripts/verify-ios-scene-project.ts`, `apps/mobile/src/plugins/ios-xcode27.test.ts`, report/ledger.

**Acceptance:** Run method signature extraction against Swift with comments removed; require a concrete `RCTLinkingManager.application(` call token sequence after both comments and string literals are masked. Add separate regressions for a forwarding marker only in a string and a commented open-URL signature before valid universal-link callback. Preserve valid URL string fixture, comment-only callback negatives, structural application scene and all PBX config checks.

**Verification:** RED/GREEN focused/full suite/typecheck/actual generated checker/diff-check; write report and commit. This is the final SDD repair wave; independently re-review at high reasoning. Any residual finding is adjudicated with evidence rather than starting another implementation loop.

**Status:** pending.
