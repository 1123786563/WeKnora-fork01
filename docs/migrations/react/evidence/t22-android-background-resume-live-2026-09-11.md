# T22 Android background/resume evidence — 2026-09-11

## Environment

- App: `com.weknora.mobile` release APK rebuilt from the React worktree.
- AVD: `test36-small`, Android API 36, serial `emulator-5554`.
- Backend: isolated Lite on `127.0.0.1:18082`, reachable through
  `adb reverse tcp:18082 tcp:18082`.
- APK SHA-256: `91c942ab3110451bb6ba181b9ceae9916a541144a095f0d69397ac111b800135`.

## Build and live result

The release artifact was rebuilt with the explicit Android SDK path using
`./gradlew :app:assembleRelease --console=plain`; Gradle reported `BUILD
SUCCESSFUL` with 603 actionable tasks. Installing the APK with `adb install -r`
returned `Success`.

The installed app then completed this native sequence:

1. The authenticated app opened `weknora://chat`, entered a new conversation,
   sent `hello`, and rendered the real `Stop` button.
2. Android Home (`adb shell input keyevent 3`) moved the app to the background
   while the isolated Lite stream was still pending. The new lifecycle path
   aborted the local stream.
3. Relaunching `com.weknora.mobile/.MainActivity` returned to the chat route.
   The accessibility tree retained the `hello` message and showed `Send` at
   `[632,1378][696,1416]`; there was no `Stop` node or error text. This state
   remained after a two-second bounded observation.

The corresponding pure lifecycle regression tests and full mobile suite passed:

```text
pnpm --filter @weknora/mobile test
50/50 passed
pnpm typecheck:mobile
exit 0
```

This proves native background cancellation and foreground recovery do not leave
the chat UI stuck in a sending state or generate a second local send. It does
not prove successful assistant token delivery, server-side continuation after
an interrupted stream, approval/OAuth recovery, attachment processing, or
network loss-and-reconnection across both platforms.
