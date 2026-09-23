# HarmonyOS native compatibility gate (#143)

**Decision: BLOCKED — no native build/device evidence.** This is an environment and supported-stack gate result, not a successful HarmonyOS validation and not proof that a future custom port is impossible.

Checked 2026-09-24 (Asia/Shanghai), in isolated worktree `codex/issue-140-t01` at base `7041e38aba9183e89e4fea894f2088d6d3d40cbd`.

## Repository and target

| Item | Evidence |
| --- | --- |
| Expo | `apps/mobile/package.json`: `expo ~55.0.0`; lockfile resolves `55.0.31` |
| React Native | `apps/mobile/package.json`: `react-native 0.83.10`; lockfile resolves `0.83.10` |
| React | `19.2.0` |
| Native app config | `apps/mobile/app.json` explicitly declares `platforms: ["ios", "android"]`; no Harmony/OpenHarmony platform, OHOS native project, or Harmony config plugin is present |
| Native modules in use | `expo-auth-session ~55.0.18`, `expo-linking ~55.0.17`, `expo-router ~55.0.18`, `expo-secure-store ~55.0.18`, `expo-web-browser ~55.0.20`, `react-native-gesture-handler ~2.30.1`, `react-native-safe-area-context ~5.6.2`, `react-native-screens ~4.23.0` |
| Host versions | Node `v26.7.0`; pnpm `10.28.2` |
| DevEco / OH SDK / hdc / hvigor / ohpm | Not found in PATH or checked standard installation roots. `DEVECOSTUDIO_VM_OPTIONS` exists but points to a VM-options file only; it does not resolve a DevEco executable or SDK. |
| Target device / OS | No `hdc` executable, no target listing, no attached HarmonyOS device or OS version evidence available |
| Build artifact | No `.hap`, `.apk`, `.aab`, or `.app` exists under `apps/mobile` |

The toolchain search covered `command -v` for `ohpm`, `hvigorw`, `hvigor`, `hdc`; the common `~/Library/OpenHarmony/{Sdk,sdk}`, `~/Library/Developer/OpenHarmony/Sdk`, `/Applications/DevEco-Studio.app` locations; `~/Applications` and `~/Library` searches for those executables/app; and a `mdfind` search for DevEco. No RNOH, hvigor, or OH SDK references were found in the mobile app or its lockfile.

## Compatibility evidence

| Layer | Observed support/version evidence | Implication for this app |
| --- | --- | --- |
| Expo app | [Expo Modules API overview](https://docs.expo.dev/modules/overview/) says its additional-platform support is experimental for macOS/tvOS; [additional platform support](https://docs.expo.dev/modules/additional-platform-support/) documents macOS/tvOS setup. Its native module config lists Android, Apple platforms, web, and devtools, without HarmonyOS ([module config](https://docs.expo.dev/modules/module-config/)). | No maintained first-party Expo Harmony target is documented. The current app's Expo native modules have no demonstrated Harmony autolinking/build path. |
| React Native OpenHarmony | [RNOH official support matrix](https://gitcode.com/OpenHarmony-RN) (the organization page points to the migrated AtomGit repositories) lists `0.84.x` as adapted, `0.83.x` with no RNOH release, and `0.82.x`, `0.77.x`, `0.72.x` as adapted. It announces RNOH `0.84.3` on 2026-08-20. | This is a separate OpenHarmony RN framework, and the current published matrix does not cover this app's RN `0.83.10`. A supported 0.84.x line does not establish compatibility with 0.83.10 or Expo SDK 55. Treat a framework move plus Expo/native-module integration as unproven. |
| RN library adaptation | [RNOH third-party library matrix](https://github.com/react-native-oh-library/usage-docs/blob/master/en/README_EN.md) records upstream baseline versions and whether a Harmony release exists; for example, `react-native-gesture-handler` lists baseline `2.14.1` with an RNOH template release. | It does not evidence the app's installed `2.30.1` version or Expo Router / SecureStore / AuthSession / WebBrowser integration. A per-module port and API compatibility evaluation would be required. |
| RNOH native project | The RNOH organization links its [framework repository](https://atomgit.com/openharmony-RN/ohos_react_native) and [third-party library guide](https://atomgit.com/OpenHarmony-RN/usage-docs); the earlier [5.0.0.813 environment guide](https://gitee.com/openharmony-sig/ohos_react_native/blob/5.0.0.813/docs/zh-cn/%E7%8E%AF%E5%A2%83%E6%90%AD%E5%BB%BA.md) describes creating an OpenHarmony/DevEco project, installing `@rnoh/react-native-openharmony`, configuring native CMake, signing, and using a physical device. | This is a genuine native integration route, but it requires a native host project and toolchain absent from this checkout/host. It is not an Expo config-only switch. |

These sources establish a support/version gap and missing local prerequisites. The RNOH matrix is the current official source checked on 2026-09-24; the older Gitee setup guide is included only to illustrate the native project/tooling requirements. They do **not** establish that a custom adapter or migration can never be built. No credentials or test Task were used, and no authenticated Task read, scope invalidation, file/share/notification/storage, or cross-Tenant/unauthorized Task probe could run.

## Reproduction commands and results

Commands were run from the repository root at the recorded base. Missing command failures are expected evidence and must not be interpreted as pass:

| Command | Result |
| --- | --- |
| `node --version` | `v26.7.0` |
| `pnpm --version` | `10.28.2` |
| `pnpm --dir apps/mobile exec expo --version` | Failed: `ERR_PNPM_RECURSIVE_EXEC_FIRST_FAIL Command "expo" not found`; dependencies/Expo CLI are not installed in this worktree. |
| `pnpm --dir apps/mobile exec expo config --type public` | Failed with the same missing Expo CLI error. Static `apps/mobile/app.json` inspection still confirms only `ios` and `android`. |
| `ohpm --version` | Failed: `zsh: command not found: ohpm` |
| `hvigorw assembleHap` | Failed: `zsh: command not found: hvigorw` |
| `hdc list targets` | Failed: `zsh: command not found: hdc` |
| `command -v ohpm hvigorw hvigor hdc` | No executable paths returned. |
| `find apps/mobile -type f \( -name '*.hap' -o -name '*.apk' -o -name '*.aab' -o -name '*.app' \) -print` | No artifacts returned. |

The checked-out App is not an OpenHarmony project, and all native build/device probes stop at the absent toolchain. Since there is no `.hap` or device, the validator cannot verify package format, native launch, authenticated Task access, or platform capabilities. Android APK/compatibility mode, a WebView, or a conceptual probe would not satisfy this native gate and were not counted.

## Ruling and exit criteria

- Keep T01/#143 **blocked** until an isolated native spike is run with a pinned RNOH/RN/Expo strategy, DevEco + matching OH SDK, `ohpm`/Hvigor, signing, a HarmonyOS device and identified system version.
- The next spike must produce a real signed `.hap`, launch it on the target, and capture native package/device evidence. It must exercise authenticated protected Task read, expired/switching scope rejection, file access, share, notification permission, controlled storage and denial of an unauthorized Task. Preserve sanitized command output/log paths; do not include tokens or personal identifiers.
- Re-evaluate whether to adapt Expo 55/RN 0.83.10 or choose a separate RNOH native composition after testing native module coverage and ownership cost. Current evidence does not justify declaring the custom-port option impossible, nor claiming it maintainable.
- Mark T05 and later mobile tickets whose acceptance requires HarmonyOS **blocked pending this ruling**; iOS/Android validation and implementation can proceed independently. No parent plan or Issue status was edited by this task.

## Cost if this ruling is wrong

If a maintained Expo/RNOH combination or a usable DevEco installation exists outside the searched roots, this blocked ruling may delay HarmonyOS work and may cause duplicate environment setup. If the team instead treats the current evidence as a pass, it could promise a third client without a launchable native package, secure credential storage, scope invalidation, or verified file/share/notification behavior. The ruling is intentionally reversible: provide the missing pinned compatibility evidence and a device/toolchain run, then repeat the gate.
