# T22 iOS native chat entry evidence — 2026-09-11

## Environment

- App: `com.weknora.mobile` iOS Release build from the current React
  worktree.
- Simulator: iPhone 17 Pro, iOS 26.5, UDID
  `5EECD8BB-4B4A-473C-85C3-7841329FDF3C`.
- Build command: `npx expo run:ios --device 'iPhone 17 Pro' --no-bundler
  --configuration Release`.
- Build result: `Build Succeeded`, 0 errors; the app was installed and opened
  on the simulator.

## Native entry result

The authenticated knowledge-base route rendered a native `Chat` button in
the header. Tapping that button changed the native route to `chat/index` and
rendered the native chat screen with its knowledge-base selector, session
list, composer, and `Send` control.

The temporary Lite server was restarted during this probe, so the previously
persisted test bearer was invalidated by the process-local JWT secret. The
chat screen consequently showed an observable HTTP 401 load error; no iOS
SSE/token-delivery acceptance is claimed from this run.

## Boundary

This proves that the iOS release app has a reachable native entry into the
chat route without relying on a broken scheme deep link. It does not prove
iOS authenticated SSE, provider behavior, approval/OAuth, attachments,
steering, or recovery. Those remain open T22/T24 gates.
