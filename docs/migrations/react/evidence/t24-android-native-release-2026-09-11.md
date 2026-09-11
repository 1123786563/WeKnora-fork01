# T24 Android native release artifact evidence — 2026-09-11

## Release build

The Android release build initially reached native compilation but failed in
offline mode because four AndroidX artifacts were not cached locally. The
same build was rerun with the explicit local React Android artifact repository
and the configured Google/Maven Central repositories:

```text
NODE_ENV=production ANDROID_HOME=/Users/wuyongjun/Library/Android/sdk \
./gradlew :app:bundleRelease --no-daemon \
  --init-script /tmp/weknora-gradle-local-repo.init.gradle
BUILD SUCCESSFUL in 6m 47s
605 actionable tasks: 227 executed, 378 up-to-date
```

The missing Babel preset that blocked the first release attempt was added as
the mobile package's direct development dependency:

```text
babel-preset-expo: ~55.0.25
```

The signed bundle and installable release APK were then both produced. The
APK task was up-to-date apart from packaging work:

```text
./gradlew :app:assembleRelease --no-daemon \
  --init-script /tmp/weknora-gradle-local-repo.init.gradle
BUILD SUCCESSFUL in 35s
603 actionable tasks: 33 executed, 570 up-to-date
```

Artifacts:

```text
apps/mobile/android/app/build/outputs/apk/release/app-release.apk  75M
sha256 01218881118402459aa734ac10cc9da8ffd76f85199e57fa4a9ed020d2ac2178

apps/mobile/android/app/build/outputs/bundle/release/app-release.aab  52M
sha256 bd39035caa62a422b98b585ee0785627b5aaf2e7dd39465cd4031dfc9a36de8f
```

## Emulator install and launch

On the local `test36-small` Android Emulator API 36 (`emulator-5554`):

```text
adb install -r app-release.apk
Success

resolved activity: com.weknora.mobile/.MainActivity
versionCode=1 minSdk=24 targetSdk=36 versionName=0.0.0
topResumedActivity=... com.weknora.mobile/.MainActivity ... visible=true
pidof com.weknora.mobile: 3053
```

The release APK launched without Metro. A bounded logcat inspection after
launch found no `FATAL EXCEPTION`, `AndroidRuntime`, `ANR`, or
`ReactNativeJS` failure lines. This proves release packaging, installation,
and native host process startup on this emulator. A UIAutomator dump also
showed the embedded React login surface with `WeKnora`, `Sign in to your
workspace`, `Email`, `Password`, `Sign in`, `Continue with SSO`, `Create
account`, `Join with invitation`, and `Change server`; no Metro URL was
required for this release launch.

## Boundary

The release artifact launch does not prove authenticated Android business
flows, live backend login, SSE, upload/share, AppState recovery, or logout.
Those remain open T20–T24 runtime gates. The Android native runtime attempt
with the debug dev-client package remains separately recorded as an ANR during
cold Metro loading; this release launch is not being used to erase that
negative evidence. T24 remains `review` and T25 remains gated.
