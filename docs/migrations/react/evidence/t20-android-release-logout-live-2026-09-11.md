# T20 Android release logout follow-up — 2026-09-11

## Environment

- App: `com.weknora.mobile` release APK rebuilt from the current React worktree.
- AVD: `test36-small`, Android API 36, serial `emulator-5554`.
- Backend: isolated Lite binary on `127.0.0.1:18082`; Android used
  `adb reverse tcp:18082 tcp:18082`.
- The temporary account and password used for this probe are not recorded.

## Regression and live result

The previous release screen put `Workspace`, `Manage`, and `Sign out` in a
non-wrapping row beside the page title. On the 720px emulator the last
control was clipped to `[699,192][720,257]`; the accessibility node existed,
but a real tap did not complete logout.

The follow-up moved the title and action row into a column, made the action
row wrap, and added the public `signOutAndRedirect` seam. Its tests cover
successful cleanup-before-navigation and the failure boundary:

```text
pnpm --filter @weknora/mobile test
45/45 passed
pnpm --filter @weknora/mobile typecheck
exit 0
```

The rebuilt release package was installed with `adb install -r` returning
`Success`:

```text
apps/mobile/android/app/build/outputs/apk/release/app-release.apk  75M
sha256 06139c246cc68cd6a8504543687fe16fe5ed337c612dc58c4c7fd4a3db7aeb00
```

On the running release app:

1. A temporary owner account logged in against the isolated Lite backend.
2. The real `Knowledge bases` screen rendered `No knowledge bases available.`
3. The `Sign out` accessibility node occupied `[319,257][421,295]`, fully
   inside the screen and visible in the screenshot.
4. Tapping its center returned the UI to `Sign in to your workspace` and
   `Sign in` within three seconds.

This proves the Android release logout interaction and route transition in
addition to the existing login, KB-list, and SecureStore restore evidence.
It does not prove Android SSE/chat, upload/download/share, AppState/network
recovery, provider callback, or the complete role/tenant matrix.
