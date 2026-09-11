# T24 Android native runtime attempt — 2026-09-11

## Scope

This records an isolated local attempt to run the current React Native source in the generated Android debug package. It is intentionally a failure/boundary record, not an acceptance claim.

- Worktree: `codex/react-multiclient`
- AVD: `test36-small`, Android Emulator API 36, serial `emulator-5554`
- APK: the T24 debug artifact recorded in `t24-android-native-build-2026-09-11.md`
- Backend: isolated Lite binary on `127.0.0.1:8080`, reachable from Android as `10.0.2.2:8080`; `/health` returned HTTP `200`
- JavaScript: current worktree Metro on `10.0.2.2:8082`, Expo dev-client URL opened explicitly through the `exp+weknora` scheme

## Observed sequence

1. The emulator booted and reported `device` through the explicit `adb` path.
2. The APK installed and launched `com.weknora.mobile`; the Expo Dev Launcher accepted the Metro URL.
3. Metro compiled and served the Android bundle: `1327 modules`, cold bundle time approximately `77.8s`.
4. The native host reached the React login screen. During the cold dev-client load Android displayed a system `WeKnora isn't responding` dialog; after waiting, the login UI remained visible behind the dialog.

## Boundary

No login, authenticated `/auth/me`, Workspace selection, API-key, upload, AppState recovery, or logout flow was completed on Android. The ANR may be specific to this cold dev-client/emulator load, but it is still an unclosed native runtime gate until reproduced or resolved. The result is therefore:

```text
Android package build: passed
Android native host + JS login UI: observed
Android authenticated business acceptance: not accepted
T24: review
T25: gated
```

No user credentials, bearer token, or production data were recorded.
