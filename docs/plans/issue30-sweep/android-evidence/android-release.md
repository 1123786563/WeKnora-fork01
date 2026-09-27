# Android Release 构建与验收 Runbook（Issue #70）

## 工程生成（CNG，本地已验证可行）

    cd apps/mobile
    npx expo prebuild -p android --no-install   # 纯 Node；生成 android/（gitignored）

生成物要点（2026-09-26 实测，见 prebuild-android-manifest.txt）：
- AndroidManifest.xml 含 `weknora` scheme VIEW intent-filter 与 `launchMode="singleTask"`（深链 warm/cold 返回）。
- app.json `android.permissions` 显式声明的权限写入 manifest（RECORD_AUDIO/POST_NOTIFICATIONS/INTERNET/VIBRATE）。
- 模板默认权限（READ/WRITE_EXTERNAL_STORAGE ≤32、SYSTEM_ALERT_WINDOW）当前不被剔除（`android.blockedPermissions` 探测无效，plan-t70 差异记录 5）。
- 库 manifest（expo-notifications 的 RECEIVE_BOOT_COMPLETED 等）在 Gradle 构建期合并，不在 prebuild 产物中。

## Release 构建（blocked-env：本环境无 Gradle/Android SDK）

前置：Android Studio 或命令行 Android SDK（ANDROID_HOME）、JDK 17（已在：openjdk 17.0.19）、`pnpm install` 完成。

    cd apps/mobile/android
    ./gradlew assembleRelease    # 产物 app/build/outputs/apk/release/app-release.apk
    # 或 EAS 队列：npx eas-cli build -p android --profile preview

## 签名（blocked-env：无 release Keystore）

现状：`android/app/build.gradle` release buildType 回退 debug signing（模板默认）。
正式发布前（EAS 托管凭据路径，keystore 不入库）：

    npx eas-cli credentials       # 生成/绑定 production keystore
    # 或本地 keystore + android/app/ 自定义 signingConfig（keystore 与口令绝不入库，走环境变量/密钥服务）

## 真机验收清单（对应 Issue #70 七工作流；全部 blocked-env，待真机轮执行）

| 工作流 | 验证点 |
|---|---|
| 系统返回 | 返回键/手势逐级回退；OIDC 返回后返回不复活已登出内容；weknora://oidc 冷/热返回 |
| 后台限制 | 后台→前台触发权威同步（前台同步环）；Doze 下的行为记录 |
| 通知 | POST_NOTIFICATIONS 首启弹窗（Task 1/2）；授权/拒绝两路径的设备注册结果；FCM 送达（需推送凭据部署） |
| 文件 URI | 录音→file:// URI→multipart 上载→转写后/取消即删（Task 3/4）；材料签名链接下载/分享 |
| Keystore | SecureStore 凭据/Vault/意图日志落盘；系统备份规则排除（manifest fullBackupContent 已由 expo-secure-store 注入） |
| 麦克风 | RECORD_AUDIO 拒权→听写入口隐藏（fail closed）；授权→录音→转写 |
| 弱网恢复 | 断网→离线草稿加密保存→派发被 Offline Gate 拒→联网后显式 resync |

证据回填到本目录（截图/日志），完成后在 Issue #70 勾选验收项——在此之前任何项不得勾选。
