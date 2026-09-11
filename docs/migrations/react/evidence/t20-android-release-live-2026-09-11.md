# T20 Android release authenticated runtime evidence — 2026-09-11

## Environment

- App: `com.weknora.mobile` release APK from the current React worktree.
- AVD: `test36-small`, Android API 36, serial `emulator-5554`.
- Backend: a newly built `EDITION=lite` arm64 binary on `127.0.0.1:18082`,
  with temporary SQLite and local files under
  `/tmp/weknora-react-t24-mobile-live.9xMSH9`; no production data was used.
- Android transport: `adb reverse tcp:18082 tcp:18082`; the app used the
  persisted `http://127.0.0.1:18082` server address.
- Account: temporary `androidrelease2026@example.test`; the password was used
  only for this isolated probe and is not stored in the repository.

## Root-cause fix and build

The first release APK could render its login UI but HTTP login failed before
the request reached Lite. Android's merged manifest had no cleartext policy,
while the mobile server-address contract explicitly accepts HTTP(S) URLs.
The fix is a standard Expo config plugin in
`apps/mobile/withAndroidCleartextTraffic.js`, registered by
`apps/mobile/app.config.ts`; its behavior is covered by a red-to-green unit
test in `apps/mobile/src/android-config.test.ts`.

The regenerated manifest contains:

```xml
android:usesCleartextTraffic="true"
```

The current release APK was rebuilt with:

```text
./gradlew :app:assembleRelease --no-daemon \
  --init-script /tmp/weknora-gradle-local-repo.init.gradle
BUILD SUCCESSFUL in 48s
603 actionable tasks: 41 executed, 562 up-to-date
```

Artifact:

```text
apps/mobile/android/app/build/outputs/apk/release/app-release.apk  75M
sha256 8fb6f96140103b59c6edf19559e130949603e4651bd40e249d11257d4d85cf4a
```

## Live sequence

1. A temporary owner registration returned HTTP `201`; a direct login probe
   returned HTTP `200` with access and refresh credentials. Tokens were not
   recorded.
2. The release APK installed with `adb install -r` returning `Success`.
3. The app accepted the server address and, after the cleartext manifest fix,
   the Android UI login completed against the isolated Lite backend.
4. The authenticated screen showed `Knowledge bases`, `Workspace`, and the
   server-backed empty state `No knowledge bases available.`.
5. After `am force-stop` and relaunch, the app restored the SecureStore bearer
   session without re-entering credentials and again showed `Knowledge bases`,
   `Workspace`, and `No knowledge bases available.`. The process remained alive
   and `MainActivity` stayed resumed/visible.

## Boundary

This proves Android release HTTP transport, real password authentication,
authenticated KB-list rendering, and cold SecureStore session restoration on
the isolated Lite backend. It does not prove Android SSE/chat, upload/download
and share, AppState/network recovery, logout, provider callback, or the full
role/tenant matrix. T20/T24 therefore remain `review`; T25 remains gated.
