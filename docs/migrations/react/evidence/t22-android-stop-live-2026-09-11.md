# T22 Android Stop interaction evidence — 2026-09-11

## Environment

- App: `com.weknora.mobile` release APK rebuilt from the React worktree.
- AVD: `test36-small`, Android API 36, serial `emulator-5554`.
- Backend: isolated Lite on `127.0.0.1:18082`, reachable through
  `adb reverse tcp:18082 tcp:18082`.
- APK SHA-256: `6ab9475e6633fa5119836f18c58ca15cf7539d21f647e4454f525c09f5075cae`.

## Regression and live result

The chat screen previously returned from `stop()` without doing anything when
the server had not emitted an assistant message id yet. The follow-up adds a
portable stop seam that aborts the local stream and marks the run stopped
before waiting for an optional remote stop request. RED→GREEN coverage passed:

```text
pnpm --filter @weknora/mobile test
47/47 passed
pnpm typecheck:mobile
exit 0
```

The release artifact was rebuilt with the explicit Android SDK path using
`./gradlew :app:assembleRelease --console=plain`; Gradle reported `BUILD
SUCCESSFUL` with 603 actionable tasks. Installing that APK with `adb install -r`
returned `Success` before the live probe.

On the installed release app:

1. A persisted authenticated owner session opened the real `weknora://chat`
   route and created a new conversation.
2. Entering `hello` and tapping `Send` rendered the real user message and a
   `Stop` accessibility button at `[476,1378][534,1416]`. The isolated Lite
   chat stream remained pending during the bounded observation, before a
   server assistant id was available.
3. Tapping the center of `Stop` at `(505,1395)` returned the accessibility
   tree to `Send` at `[632,1378][696,1416]` within 300ms. A second observation
   one second later still showed `Send`, with no `Stop` node and no error text.

This proves the Android release's local stop/recovery interaction for the
early-stream/no-assistant-id state, including the real native button path. It
does not prove successful assistant token delivery, remote stop acceptance
after an assistant id exists, approval/OAuth flows, steering, or iOS chat
runtime behavior. The pending Lite stream is not counted as a successful
retrieval or answer.
