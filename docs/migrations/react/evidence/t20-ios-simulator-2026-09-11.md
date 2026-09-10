# T20 iOS Simulator native runtime evidence — 2026-09-11

## Build and launch

- Added fixed `com.weknora.mobile` iOS/Android identifiers and the
  `expo-dev-client` plugin so `expo run:ios` can produce an addressable native
  development build.
- `pnpm --filter @weknora/mobile exec expo run:ios --device "iPhone 17 Pro" --no-bundler` completed with `Build Succeeded`, `0 error(s)`, installed the app, and launched bundle id `com.weknora.mobile` on the iPhone 17 Pro iOS 26.5 simulator.
- `pnpm --filter @weknora/mobile exec expo start --dev-client --lan --port 8082 --clear` served the worktree Metro bundle.
- The simulator development menu reported `Connected to: http://127.0.0.1:8082`; Metro reported the iOS entry bundle successfully built with 1,280 modules.
- The simulator visibly rendered the WeKnora native login screen with email,
  password, and sign-in controls.

## Boundary

This proves native iOS host compilation, installation, Metro connection, and
the login shell. It does not prove real login/refresh/logout, SecureStore
rotation, AppState/network recovery, or backend data flows because no API base
URL and test account were supplied. Android has no `adb` or emulator in this
environment, so Android native runtime remains open.
