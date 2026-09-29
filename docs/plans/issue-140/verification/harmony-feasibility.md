# #143 / Task 0A — Harmony native feasibility ruling

- Date: 2026-09-29 Asia/Shanghai
- Source: approved #143 snapshot `docs/plans/issue-140/issues/issue-143.md`, ADR 0018, Task 0A brief.
- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-p0a-harmony/WeKnora-fork01`
- BASE: `477da9f677e0357ea8bc360c2802314fef534723`
- Probe started at: `2c4438af69b4ea60e5cf783f2e1318f3147d593c`; this report is delivered in the task commit recorded below.
- Raw reproducible output: [`task-0a-environment.log`](../../../apps/mobile/harmony/validation/task-0a-environment.log), [`task-0a-native-attempt.log`](../../../apps/mobile/harmony/validation/task-0a-native-attempt.log).

## Ruling

**Current Expo app stack has no demonstrated maintainable Harmony native route. Do not start Harmony-specific #144/client implementation against this Expo 55 / React Native 0.83.10 configuration.** The candidate native React Native for OpenHarmony (RNOH) distribution still pins React Native 0.72.5 (latest upstream `master` observed at commit `bc7ad74d4df66886733d09f1c3b964caf022903a`, package version 0.72.58). The app uses RN 0.83.10. Expo's official native-module support documentation says first-class native platforms are Android/iOS; out-of-tree support is documented only for macOS/tvOS. Its Expo Modules API does not list Harmony. Expo's prebuild CLI also explicitly skipped this project because it has no Harmony template.

This is a **current-stack incompatibility ruling**, not a claim that Harmony-native delivery is technically impossible forever. An owner can reopen the gate after a maintained RNOH/RN 0.83-compatible release exists, or after an approved replatform/port plan funds a maintained RN 0.72.5 fork and per-module Harmony adapters. The present environment additionally lacks DevEco Studio, `hvigor`, `hdc`, `ohpm`, and Harmony hardware; therefore no native HAP build, device installation, login, authenticated Task fetch or system capability probe was verified. Tool/device absence is reported as an unresolved runtime gate, not as a passing trial.

## Platform identity and version matrix

| Target | What counts as evidence | Observed status |
|---|---|---|
| HarmonyOS native | OpenHarmony/HarmonyOS native application project compiled to a Harmony package (HAP) and launched on a Harmony device; RNOH is the native candidate examined | Not built. Candidate toolchain documentation targets HarmonyOS SDK API 13. No DevEco/Harmony SDK or device is installed/connected. |
| Android compatibility on Harmony | Android APK launched by an Android compatibility layer, with Android runtime/API behavior | Not attempted and does not satisfy #143 native acceptance. `adb` is absent. |
| WebView | Web content in a Harmony browser/WebView | Not attempted and does not satisfy #143 native acceptance. |
| Expo/React Native application baseline | Existing source/dependencies that a native Harmony host must consume | Expo 55.0.36, React Native 0.83.10, React 19.2.0. App config declares only `ios`, `android`. |
| RNOH candidate | `@rnoh/react-native-harmony` native host + Harmony-specific Metro package configuration | Upstream `master` at `bc7ad74d4df66886733d09f1c3b964caf022903a`: package 0.72.58, package dependency pinned to RN 0.72.5; upstream setup guide states RN 0.72.5 and HarmonyOS SDK API 13. Published tag `5.0.0.813` resolves to `c33f63ace9efcb5df192c66e6aebbbf0c9fd3d20`, package 0.72.48, also RN 0.72.5. Neither matches current RN 0.83.10. |

### App native module surface

Local `expo-module.config.json` platform declarations for the installed SDK 55 packages were inspected after `pnpm install --frozen-lockfile`:

| Dependency | Installed | Declared native platforms in package metadata | Harmony adapter evidence |
|---|---:|---|---|
| expo-audio | 55.0.18 | apple, android | None found |
| expo-auth-session | 55.0.18 | No native expo-module config; uses browser/auth-session platform implementations | None found |
| expo-crypto | 55.0.19 | apple, android | None found |
| expo-file-system | 55.0.26 | apple, android | None found |
| expo-linking | 55.0.17 | apple, android, web | None found |
| expo-network | 55.0.18 | apple, android, web | None found |
| expo-notifications | 55.0.27 | apple, android | None found |
| expo-router | 55.0.18 | apple, android, web | None found |
| expo-secure-store | 55.0.18 | apple, android | None found |
| expo-web-browser | 55.0.20 | apple, android | None found |
| react-native-gesture-handler | 2.30.1 | Android and iOS native directories | None found |
| react-native-safe-area-context | 5.6.2 | Android and iOS native directories | None found |
| react-native-screens | 4.23.0 | Android and iOS native directories | None found |

Platform declarations were read from the installed package metadata; absence of a Harmony declaration alone is not proof no community patch exists. Combined with Expo's supported-platform statement, the RN-version mismatch and the absent generated/native host, it means this app cannot currently be treated as a compatible Harmony native app. Auth, secure storage, file, notification, browser and navigation capabilities all require explicit adapters and real probes before acceptance.

## Reproducible attempt and result

- `pnpm install --frozen-lockfile` — passed; lockfile unchanged; installed workspace dependencies.
- `pnpm --dir apps/mobile exec expo --version` — `55.0.36`.
- `node -p "require('./apps/mobile/node_modules/react-native/package.json').version` — `0.83.10`.
- `pnpm --dir apps/mobile exec expo prebuild --platform harmony --no-install` — exit 0 but emitted `only "ios, android" is present in app config` and `Skipping platform harmony. Use a template that contains native files for harmony`; it created no native Harmony project and produced no HAP. This is an unsupported-target skip, **not** a successful build.
- RNOH upstream was checked in a shallow clone outside the worktree. Current `master` commit and package metadata are recorded above; its setup guide still prescribes RN 0.72.5 and HarmonyOS SDK API 13.
- `command -v hvigor hdc ohpm adb` — all unavailable. `xcodebuild` exists but is Apple-only and irrelevant to Harmony. `/Applications/DevEco-Studio.app` is absent.

Raw environment and native attempt logs live under `apps/mobile/harmony/validation/`.

## Capability probes

| Capability | Result |
|---|---|
| Login/authenticated Task read / cross-scope rejection | Not attempted: no native app package or Harmony device |
| Scope invalidation / stale response behavior on Harmony | Not attempted |
| File selection/download | Not attempted |
| Share intake | Not attempted |
| Notification permission/token/delivery | Not attempted |
| Secure storage | Not attempted |
| Native UI/TDesign token mapping | No Harmony native UI implementation added; #143 does not authorize changing the iOS/Android UI boundary |

No compatibility APK, WebView, conceptual UI or shared-code unit test is counted as native evidence.

## Impact and gate

- **Block only Harmony-specific acceptance** in #144 and Harmony-dependent #163, #165, #167, #168, #171, #172 pending a new compatibility/architecture decision and real native package/device evidence.
- #145 iOS/Android remains independent and is not blocked by this ruling.
- #140's five-environment completion/release gate remains open until the Harmony requirement is resolved; iOS/Android and mini-program tasks may continue.
- Reopen condition: a supported RN 0.83-compatible native Harmony host (or explicitly approved, maintained downgrade/fork plan), Harmony implementations for all used native modules, DevEco/Harmony API target, a real Harmony device, and passing login/scope/file/share/notification/secure-storage probes.

## Independent review status

Pending parent-orchestrated `frontend_validator` review of the source evidence, interpretation, and affected-ticket propagation. The technical ruling is not independently validated until that review result is recorded in the ledger.

## First-party / upstream references

- Expo additional platform support: https://docs.expo.dev/modules/additional-platform-support/ (states first-class iOS/Android and currently supported out-of-tree Apple platforms macOS/tvOS).
- Expo Modules API overview: https://docs.expo.dev/modules/overview/ (Harmony is not included in its documented platform support answer).
- OpenHarmony SIG React Native for OpenHarmony source, current branch: https://gitee.com/openharmony-sig/ohos_react_native (commit and package manifest recorded above).
- RNOH setup guide: https://gitee.com/openharmony-sig/ohos_react_native/blob/master/docs/zh-cn/%E7%8E%AF%E5%A2%83%E6%90%AD%E5%BB%BA.md (RN 0.72.5 setup; SDK/API and IDE prerequisites).
- RNOH tagged release notes: https://gitee.com/openharmony-sig/ohos_react_native/blob/5.0.0.813/docs/zh-cn/%E7%89%88%E6%9C%AC%E8%AF%B4%E6%98%8E.md.
- Huawei HarmonyOS native adaptation guide: https://developer.huawei.com/consumer/en/doc/harmonyos-guides/application-dev-guide (native application SDK reference; this project did not have the required toolchain to execute it).
