# r104 Android 原生构建验收

## 环境与命令

- 工程：`apps/mobile/android`
- Android SDK：`/Users/wuyongjun/Library/Android/sdk`
- 命令：`ANDROID_HOME=/Users/wuyongjun/Library/Android/sdk ANDROID_SDK_ROOT=/Users/wuyongjun/Library/Android/sdk ./android/gradlew -p android assembleDebug`

## 结果

- Gradle 构建成功：`BUILD SUCCESSFUL in 5s`。
- Debug APK 生成：`apps/mobile/android/app/build/outputs/apk/debug/app-debug.apk`。
- 构建包含 Expo、React Native、CMake native targets 和 Android manifest 处理。
- 当前机器没有可用 `adb` 设备（`adb` 命令不可用），因此未执行 APK 安装、启动或认证业务交互。

## 证据边界

本项关闭 Android 原生 Debug 编译和 APK 产物门禁；设备启动及业务页面仍需 Android SDK platform-tools/模拟器或实体设备后继续验收。
