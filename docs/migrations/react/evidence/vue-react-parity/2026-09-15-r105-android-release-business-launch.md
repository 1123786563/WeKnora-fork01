# r105 Android Release 业务启动验收

## 环境与命令

- 工程：`apps/mobile/android`
- Android SDK：`/Users/wuyongjun/Library/Android/sdk`
- AVD：`test36-small`（Android 36，headless，GPU swiftshader）
- 构建：`ANDROID_HOME=/Users/wuyongjun/Library/Android/sdk ANDROID_SDK_ROOT=/Users/wuyongjun/Library/Android/sdk ./android/gradlew -p android assembleRelease`
- 安装：`adb install -r android/app/build/outputs/apk/release/app-release.apk`
- 启动：`adb shell monkey -p com.weknora.mobile 1`

## 结果

- Release Gradle 构建成功：`BUILD SUCCESSFUL in 2m 4s`，完成 JS bundle、CMake native targets、签名校验和 APK 打包。
- Release APK 生成并成功安装（`Success`）。
- `com.weknora.mobile/.MainActivity` 成功成为 resumed activity。
- 启动后直接进入 WeKnora 知识库业务页面，显示导航、知识库筛选、空态和“Network request failed”错误提示；截图：`/tmp/weknora-react-multiclient-android-release.png`。
- 由于本地模拟器未连接后端服务，请求失败属于环境边界；本次证据关闭 Android Release 编译、安装、启动和首屏业务渲染门禁，不代表认证后业务链路已验收。

## 证据边界

Android 原生 Release 产物和首屏业务启动已取得设备级证据；认证、知识库 CRUD、聊天、图谱及后端错误恢复仍需连接可用后端后继续验收。
