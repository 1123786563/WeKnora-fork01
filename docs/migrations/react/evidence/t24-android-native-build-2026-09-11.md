# T24 Android native package build — 2026-09-11

## Build

Expo managed project prebuild completed successfully:

```text
pnpm --filter @weknora/mobile exec expo prebuild --platform android --no-install
exit 0
```

The generated `apps/mobile/android/` directory is Expo-generated and remains ignored by Git. With the local Android SDK supplied explicitly for this shell, the native debug package compiled successfully:

```text
ANDROID_HOME=/Users/wuyongjun/Library/Android/sdk \
ANDROID_SDK_ROOT=/Users/wuyongjun/Library/Android/sdk \
./gradlew :app:assembleDebug --no-daemon
```

Result:

```text
BUILD SUCCESSFUL in 24m 17s
369 actionable tasks: 337 executed, 32 up-to-date
exit 0
```

Artifact:

```text
apps/mobile/android/app/build/outputs/apk/debug/app-debug.apk
172 MB
sha256 75a4dcb77a19009b24be46292261b80cc03836d5227e746a9454071f247f8359
```

## Evidence boundary

- This proves the current React Native/Expo source can generate and compile an Android native debug package with the local SDK.
- The first Gradle invocation failed only because the shell did not expose `ANDROID_HOME`; rerunning with the explicit SDK paths above passed.
- `adb` is not installed in this environment, so no Android device/emulator was available for install, launch, login, refresh, AppState, upload, or logout evidence.
- This is package-build evidence, not Android runtime acceptance; T20/T24 remain `review` and T25 remains gated.
