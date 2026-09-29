# Task report — Issue #31 iOS 27 scene lifecycle

## Scope and changed paths

Executed on branch `codex/issue30-b0-t31`, BASE `db234c5eb171f2dde7427d382b55b503a038f879`, in the assigned worktree. Generated `apps/mobile/ios/` output remains outside the repository, under `/tmp/weknora-issue31-final-prebuild-_x7w61dy`.

Implementation paths: `apps/mobile/package.json`, `apps/mobile/app.json`, `apps/mobile/tsconfig.json`, `apps/mobile/src/android-release-config.test.ts` (SDK version assertions only), `apps/mobile/src/release-deps.test.ts` (Expo audio/network major assertions only), `apps/mobile/src/app-smoke.test.tsx` (React JSX test stub only), `apps/mobile/src/native-project-config.test.ts`, `pnpm-lock.yaml`, and `docs/testing/mobile-runtime-login-device-acceptance.md`.

## RED → GREEN

- RED: `pnpm --filter @weknora/mobile exec tsx --test src/native-project-config.test.ts` failed as expected at `Expo major must be 57` (`55 !== 57`), not due to test loading/path issues.
- GREEN after SDK/config changes: same focused contract test passed, 1/1.
- Updated SDK 55 dependency contracts for `expo-audio`, `expo-network`, and `expo-file-system`; focused `android-release-config.test.ts` plus native config tests passed 5/5.

## SDK/config findings and resolution

- `expo install --fix` resolved SDK 57 dependencies; final direct versions are Expo `~57.0.26`, Expo Router `~57.0.24`, React `19.2.3`, React Native `0.86.3`, and `expo-build-properties ~57.0.22`.
- Expo Doctor initially identified missing `expo-asset` / `expo-constants`, `@expo/dom-webview` 55, SDK patch lag, and unsupported `newArchEnabled`. Added the two required peers and `@expo/dom-webview ~57.0.1`, used the doctor-selected Expo/router patch versions, and removed `newArchEnabled:false` because SDK 57 requires New Architecture and rejects the old option.
- Removed the legacy `./plugins/ios-xcode27` app plugin entry. It is tied to Expo SDK 55 AppDelegate/Podfile templates and failed prebuild with “no longer matches the Expo SDK 55 template.” SDK 57 now emits the supported scene manifest and `EXExpoAppSceneDelegate`, and its generated AppDelegate conforms to `ExpoReactNativeFactoryProvider`; it defers RN startup to Expo's scene lifecycle and retains app URL callbacks through `super.application` plus `RCTLinkingManager.application`. The previous `RCTNewArchEnabled=false` workaround is obsolete because SDK 57 requires the new architecture. The prior deployment-target clamp is replaced by top-level `expo.ios.deploymentTarget=16.4`. No generated native code is committed.
- TypeScript 6 requires explicit ambient type inclusion (default `types` is empty); `apps/mobile/tsconfig.json` now lists `node` and `react`, fixing hundreds of `TS2591 node:*` errors. The prescribed mobile typecheck passes after this narrow config correction.
- The test harness stubs `react`, while React 19.2.3's automatic JSX runtime reads React client internals. Added a minimal `react/jsx-runtime` stub alongside the existing stub, preserving the screen and routing assertions.

## Verification commands and current outcomes

- `pnpm install --frozen-lockfile` — passed; lockfile current.
- `pnpm --filter @weknora/mobile exec -- expo install --check` — passed, “Dependencies are up to date.”
- `npx --yes expo-doctor` from `apps/mobile` — passed, 21/21 checks.
- `pnpm --filter @weknora/mobile typecheck` — passed (`tsc --noEmit`, exit 0).
- `pnpm --filter @weknora/mobile exec -- tsx --test src/app-smoke.test.tsx` — passed, 67/67, including nested typecheck.
- `pnpm --filter @weknora/mobile test` — exit 0, 290 tests: 276 passed, 14 credential-gated integration scenarios skipped, 0 failed.
- `pnpm --filter @weknora/mobile exec -- expo export --platform ios --output-dir /tmp/issue31-ios-export` — passed; iOS JS bundle exported.
- `pnpm --filter @weknora/mobile exec -- expo export --platform android --output-dir /tmp/issue31-android-export` — passed; Android JS bundle exported.
- Final `pnpm install --frozen-lockfile` followed by `pnpm --filter @weknora/mobile exec -- expo install --check` — both passed; dependencies are up to date.
- Generated-project contract script against `/tmp/weknora-issue31-final-prebuild-_x7w61dy/ios` — passed: `UIApplicationSceneManifest` names `EXExpoAppSceneDelegate`; AppDelegate conforms to `ExpoReactNativeFactoryProvider`; no `UIWindow` creation in `didFinishLaunchingWithOptions`; callback URL methods remain; Xcode target settings and `Podfile.properties.json` are all `16.4`.
- `pod install` in the isolated final prebuild — passed, 106 Podfile dependencies / 105 pods; RN 0.86.3 artifacts verified/downloaded.
- Initial reduced harness `xcodebuild ... build` — exited 0, but app had no JS bundle because the disposable project omitted `apps/mobile/src`; its “No script URL provided” screenshot is retained as diagnostic at `docs/testing/evidence/mobile-runtime-login/2026-09-29/ios27-no-script-url-redbox.png`, with initial log `/tmp/issue31-simulator-app.log`.
- Full-source Debug build was rerun with `FORCE_BUNDLING=1` and a temp generated-project `.xcode.env.updates` unsetting `SKIP_BUNDLING`; build succeeded and embedded a 6.9 MB `main.jsbundle`. Its redbox `/tmp/issue31-fullsrc-startup.png` reports a devtools websocket error because the Debug bundle expects Metro; retained as diagnostic at `docs/testing/evidence/mobile-runtime-login/2026-09-29/ios27-debug-embedded-devtools-redbox.png`, not as final runtime evidence.
- Full-source Release command `xcodebuild -workspace WeKnora.xcworkspace -scheme WeKnora -configuration Release -sdk iphonesimulator -destination 'platform=iOS Simulator,id=0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC' CODE_SIGNING_ALLOWED=NO build` — exited 0, `** BUILD SUCCEEDED **`, log `/tmp/issue31-fullsrc-xcodebuild-release.log`; Release app embeds 3.8 MiB `main.jsbundle` at `/Users/wuyongjun/Library/Developer/Xcode/DerivedData/WeKnora-fdytvnxnahftlzglashthygouxky/Build/Products/Release-iphonesimulator/WeKnora.app/main.jsbundle`.
- Fresh `simctl install`, `simctl launch`, 25-second wait, screenshot and system log capture on iPhone 18 Pro iOS 27 simulator — install/launch succeeded; visually inspected screenshot `docs/testing/evidence/mobile-runtime-login/2026-09-29/ios27-release-deployment-login.png` shows Deployment Login rendered; `/tmp/issue31-fullsrc-release-startup.log` records app scene activation without a React Native startup exception. Metro was not needed for the Release bundle.

## Acceptance and risks

Simulator scene project/config contract, Expo Doctor, tests, typecheck, JS exports, iOS simulator Release build and visible local Deployment Login startup are verified. HTTPS staging password/OIDC/capability acceptance and Android device acceptance remain pending due to unavailable staging credentials/device evidence; no secrets were inspected or added. This local simulator gate does not claim external authentication acceptance.

## Patch and commit

Implementation commit: `2b363502068aaf0ae29c5c92958448db17e00527`. The exact code-and-acceptance patch from BASE `db234c5eb171f2dde7427d382b55b503a038f879` through that commit is `.superpowers/sdd/plan-t31-ios27/issue31-code.patch`; SHA-256 `61c4e0e641ac6c059613565cdc79b76edf3816d89fc5879ae4cfbbccc1462fe5`. It contains `apps/mobile` config, dependency/lockfile, directly affected tests and TS config plus `docs/testing/mobile-runtime-login-device-acceptance.md`; it excludes SDD plans/brief/report/progress metadata and later simulator evidence checkpoint. The plan is preserved separately at `docs/plans/issue30-sweep/plans/plan-t31-ios27.md`. Docs/evidence checkpoint `162627734` was followed by the final runtime evidence checkpoint; no business source changes followed the tested implementation commit.
