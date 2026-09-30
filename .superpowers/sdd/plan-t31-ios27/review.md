# Independent review — Issue #31 T01 iOS 27 follow-up

Reviewed 2026-09-29. Scope: implementation delta `db234c5eb171f2dde7427d382b55b503a038f879..2b363502068aaf0ae29c5c92958448db17e00527`, with the code/acceptance subset in `issue31-code.patch` (SHA-256 `61c4e0e641ac6c059613565cdc79b76edf3816d89fc5879ae4cfbbccc1462fe5`). Later report and screenshot commits were read as evidence, not included in the reviewed diff. Sources: approved mobile AI Office spec, ADR 0005, `CONTEXT.md`, Issue #31 snapshot, T31 plan and brief, generated iOS project, task report, and Release simulator screenshot. This was read-only analysis; no tests, build, OCR, or issue mutation were performed by the reviewer.

## Findings

### R1 — Medium — canonical iOS Release build can reuse an obsolete native project

**Evidence:** `apps/mobile/app.json:28-45` removes `./plugins/ios-xcode27` and configures the SDK 57 scene plugin. The repository's supported `apps/mobile/scripts/ios-release-build.sh:8-11,30-45` still describes the removed plugin as its native source of truth and calls `npx expo prebuild -p ios --no-install` without `--clean`. The retired plugin injects an Expo 55 `AppDelegate`/`SceneDelegate`, old-architecture flags, Podfile edits, and a 16.0 target (`apps/mobile/plugins/ios-xcode27.js:1-27,359-400`). Existing ignored `apps/mobile/ios/` output is deliberately reused by this script. The successful Task report instead built from an isolated **clean** SDK 57 prebuild, so it does not cover this repository build path with an existing generated project.

**Impact:** Operators following the documented release entry point can retain native source/settings from the SDK 55 plugin after updating JS dependencies. The resulting build may fail or produce a different scene/auth-callback configuration from the validated Release app. This weakens the claimed reproducible iOS 27 build gate.

**Smallest correction:** Make the supported release script regenerate the native project cleanly for this SDK transition, update its obsolete plugin comments, then verify that script from both a clean state and a pre-existing SDK 55 `ios/` directory. Keep generated native output untracked.

### R2 — Medium — installed login screen places controls beneath the iOS status bar

**Evidence:** `docs/testing/evidence/mobile-runtime-login/2026-09-29/ios27-release-deployment-login.png` shows `Sign in to WeKnora` at the top edge and the deployment, email, and password fields overlapping the 20:07/status icons. `apps/mobile/src/app/_layout.tsx:5` hides the stack header, and `apps/mobile/src/screens/DeploymentLoginScreen.tsx:47-65` uses a bare `View` with no safe-area inset. The image is the reported full-source SDK 57 Release startup for the implementation commit.

**Impact:** The startup surface renders, but the first form fields are obscured on the iPhone 18 Pro simulator. This is a visible usability and accessibility defect on the native login gate, even though scene activation succeeds.

**Smallest correction:** Apply the top safe-area inset at the app shell or login screen and capture a fresh installed Release screenshot. Keep the presentation change centralized so other authorized surfaces use the same inset policy.

### R3 — Low — native lifecycle regression tests still exercise the retired SDK 55 plugin

**Evidence:** `apps/mobile/src/native-project-config.test.ts:19-34` checks package/config JSON only. Existing `apps/mobile/src/plugins/ios-xcode27.test.ts:5-19,91-120` tests the removed Expo 55 plugin's hand-written scene rewrite; the implementation diff leaves that file and plugin in place. The generated scene manifest, delegate, and cold/warm link forwarding were checked manually in the task report, while the automated test suite can stay green if SDK 57 prebuild output drifts.

**Impact:** The reported 290-test pass overstates current scene lifecycle regression coverage; it includes tests for code no longer invoked by `app.json`. A future SDK patch can change generated native output without failing the config test.

**Smallest correction:** Replace or retire the old-plugin tests and add a generated-project contract check for the SDK 57 scene manifest, delegate/factory, deployment target, and link handoff. Run it against clean prebuild output in the native build gate.

## Verdict

**Spec compliance: Partial.** The reviewed config uses Expo SDK 57, scene support and iOS 16.4; the isolated generated project has `EXExpoAppSceneDelegate`/`ExpoReactNativeFactoryProvider`; a full-source Release simulator build renders Deployment Login. That resolves the documented iOS 27 black-screen blocker for the isolated build. The approved spec and Issue #31 additionally require authorized HTTPS login/OIDC, capability behavior, and installed iOS/Android acceptance; those remain pending and are correctly disclosed. The screenshot also shows the R2 form layout defect.

**Code quality: Changes requested.** R1 leaves the repository's supported release path unverified across the SDK transition; R2 affects the visible native gate. R3 is a smaller test-coverage gap. The task report provides credible install, typecheck, test, Expo Doctor, export, clean prebuild, pod install, and Release build evidence for its recorded snapshot. This reviewer did not repeat those commands.
