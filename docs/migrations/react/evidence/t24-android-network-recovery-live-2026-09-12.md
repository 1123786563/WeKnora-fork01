# T24 Android network recovery live evidence — 2026-09-12

## Scope and environment

- App: `com.weknora.mobile` release APK built from the current React
  worktree; APK SHA-256:
  `7e632b78c54595638d89f532bd422986676ef76b4bc2dedd3c480d4324712ceb`.
- AVD: `test36-small`, Android API 36, serial `emulator-5554`.
- Isolated Lite backend: `127.0.0.1:8080`, SQLite database and local files
  under `/tmp/weknora-react-t24-network-20260912*`; `adb reverse tcp:8080
  tcp:8080`; no production data or provider credentials.
- The temporary owner account was created through the real registration API,
  then logged in through the native release UI. Credentials and tokens are not
  stored in the repository.

## Implementation and build

- `apps/mobile/src/platform/network.ts` defines the platform-neutral
  offline→online edge detector. An initial online callback is ignored; a
  recovery callback fires once for each observed offline→online transition.
- `apps/mobile/src/runtime.tsx` subscribes to the native
  `@react-native-community/netinfo` listener and only refreshes an authenticated
  session when the app is foregrounded. The existing `AppState` listener still
  handles a background→foreground transition.
- TDD evidence: the new test first failed with `Cannot find module
  './network.ts'`; after the minimal implementation, the focused test passed
  2/2. The full mobile suite passed 59/59 and `pnpm typecheck:mobile` exited 0.
- A production Android bundle export succeeded with 1171 modules. The native
  release build included the `react-native-community_netinfo` module and
  completed with `BUILD SUCCESSFUL` (640 actionable tasks).

## Live recovery sequence

1. The release APK installed successfully, accepted the isolated HTTP(S)
   server address, authenticated against Lite, and rendered `Knowledge bases`
   with the real empty state `No knowledge bases available.`.
2. With the app active and authenticated, Android Wi-Fi and mobile data were
   disabled using the emulator's system controls. `dumpsys connectivity` no
   longer reported an active `NetworkAgentInfo`; after re-enabling both
   transports it reported a connected, validated `MOBILE[NR]` network.
3. The recovered foreground app remained alive and on the authenticated
   knowledge-base route. The backend log recorded a single real
   `POST /api/v1/auth/refresh` response with HTTP 200 at `03:19:55.950` after
   the offline→online transition.
4. The post-recovery UIAutomator dump contained `Knowledge bases`, `Workspace`,
   `Sign out`, and `No knowledge bases available.`; it contained no login or
   error-alert state. The pulled dump SHA-256 is
   `0eaba9441d7f3447379ab0279b2b61a40da85d7ce183c017b9b90b9b7699a2ec`.
   `com.weknora.mobile` remained the top resumed activity with a live process.

## Boundary

This closes the Android foreground offline→online session-refresh slice on an
isolated Lite backend and the native NetInfo build/runtime path. It does not
claim real cellular/Wi-Fi hardware, iOS network recovery, provider-backed
traffic, or every screen's query refetch after network restoration. T20/T24
remain `review`; T25 remains gated by the broader release, deployment, role,
and cross-platform acceptance matrix.
