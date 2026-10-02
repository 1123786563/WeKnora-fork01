# Android Release 构建与验收 Runbook（Issue #70）

## 工程生成（CNG，本地已验证可行）

    cd apps/mobile
    npx expo prebuild -p android --no-install   # 纯 Node；生成 android/（gitignored）

生成物要点（2026-09-26 实测，见 prebuild-android-manifest.txt）：
- AndroidManifest.xml 含 `weknora` scheme VIEW intent-filter 与 `launchMode="singleTask"`（深链 warm/cold 返回）。
- app.json `android.permissions` 显式声明的权限写入 manifest（RECORD_AUDIO/POST_NOTIFICATIONS/INTERNET/VIBRATE）。
- 模板默认权限（READ/WRITE_EXTERNAL_STORAGE ≤32、SYSTEM_ALERT_WINDOW）当前不被剔除（`android.blockedPermissions` 探测无效，plan-t70 差异记录 5）。
- 库 manifest（expo-notifications 的 RECEIVE_BOOT_COMPLETED 等）在 Gradle 构建期合并，不在 prebuild 产物中。

## Release 构建（2026-10-02/03 已尽实：模拟器工程验收，证据 `2026-10-02-live/`）

前置：Android Studio 或命令行 Android SDK（ANDROID_HOME）、JDK 17（已在：openjdk 17.0.19）、`pnpm install` 完成。

    cd apps/mobile/android
    JAVA_HOME=<jdk17> ANDROID_HOME=<sdk> \
    EXPO_PUBLIC_WEKNORA_CLOUD_ORIGIN=https://<authorized-origin> \
    WK_RELEASE_STORE_FILE=<local keystore> WK_RELEASE_STORE_PASSWORD=... \
    WK_RELEASE_KEY_ALIAS=... WK_RELEASE_KEY_PASSWORD=... \
    ./gradlew assembleRelease    # 产物 app/build/outputs/apk/release/app-release.apk

`signingConfigs.release` 由 `WK_RELEASE_*` 环境变量驱动（`android/` 整体 gitignored；未设变量时回退 debug signing 兼容本地联调）。

## 签名（2026-10-02 已尽实：本地 release keystore，工程验收口径）

本地 keystore `/tmp/t40r/weknora-release.keystore`（PKCS12，alias `weknora-release`，不入库），证书 SHA-256
`07:F8:88:0C:F6:18:B5:FC:53:F7:C7:38:8D:9A:34:72:AF:26:FF:C6:02:B9:31:56:CC:95:A3:AE:27:18:61:35`；
apksigner `Verifies`（v2 scheme）。正式发布前换 EAS 托管凭据（keystore 不入库）。

## 七工作流验收矩阵（2026-10-02/03 模拟器 live 轮；用户裁定：模拟器证据=app 端验收证据）

证据根：`2026-10-02-live/`（截图+`logs/`+`SHA256SUMS`+`README.md` 逐行判词）。

| 工作流 | 验证点 | 判定与证据 |
|---|---|---|
| 系统返回 | 返回键/手势逐级回退；OIDC 返回后返回不复活已登出内容；weknora://oidc 冷/热返回 | **evidenced**：`w1-back-1/2`（detail→tasks→home）、`w1-warm-return`、`w1-cold-return`、`w1-signed-out/after-back/revive-check` |
| 后台限制 | 后台→前台触发权威同步（前台同步环）；Doze 下的行为记录 | **evidenced**：`w2-background/foreground-sync`（恢复即 `GET /workbench/inbox 200`）、`w2-detail-synced`（已同步+failed 结算短码）；deviceidle ACTIVE 如实 |
| 通知 | POST_NOTIFICATIONS 首启弹窗（Task 1/2）；授权/拒绝两路径的设备注册结果；FCM 送达（需推送凭据部署） | 弹窗两路径 **evidenced**：`13/14-oidc-authorized`、`w3-deny-dialog/denied-after`（appops allow）；设备注册 no-token 短路 + **FCM 送达 blocked-env**（无推送凭据） |
| 文件 URI | 录音→file:// URI→multipart 上载→转写后/取消即删（Task 3/4）；材料签名链接下载/分享 | **部分 evidenced**：麦克风授权弹窗 `w4-mic-dialog`；模拟器原生录音捕获稳定 capture-failed（fail-closed 文案+手打不受影响，`w4-recording/dictate-retry`，**真机录音链挂账**）；材料页空态 `w4-materials`；签名链接下载/分享 **blocked**（无 executor 产出材料，同 #69 flow7 口径） |
| Keystore | SecureStore 凭据/Vault/意图日志落盘；系统备份规则排除（manifest fullBackupContent 已由 expo-secure-store 注入） | **evidenced**：冷启动/冷深链授权态恢复（`w1-cold-return`）+离线草稿加密留存（`w7-back-online`）；`logs/manifest-tree.txt` + `logs/backup-rules.txt`（exclude sharedpref/SecureStore） |
| 麦克风 | RECORD_AUDIO 拒权→听写入口隐藏（fail closed）；授权→录音→转写 | 拒权 fail-closed **evidenced**：`w6-mic-denied-copy`（denied 文案+RETRY+不崩溃；注：实现语义=权限拒绝保留入口+引导，adapter 缺失才隐藏——见 2026-10-02-live/README 口径差异）；授权→录音→转写 **blocked-env**（模拟器 capture-failed，真机挂账） |
| 弱网恢复 | 断网→离线草稿加密保存→派发被 Offline Gate 拒→联网后显式 resync | **evidenced**：`w7-offline-submit-gate`（Offline Gate 拒+加密草稿文案）→`w7-back-online`（草稿留存）→`POST /workbench/executions 202`→`w7-final-list`（新任务 failed=executor 边界显式结算） |

证据回填到本目录（截图/日志），完成后在 Issue #70 勾选验收项——在此之前任何项不得勾选。

---

## 2026-10-03 终局判词（T40 #70）

模拟器口径内（用户裁定 2026-10-02）**closure-ready**（独立终审 task-3-review.md）：Release 构建+本地 keystore 签名（v2 Verifies，指纹 07:F8:…:61:35）+ 真实 OIDC 全链登录留证；七工作流①②⑤⑦ evidenced、③通知两路径 evidenced（FCM 送达=外部凭据 blocked-env）、④⑥部分项有据归因（模拟器原生录音 capture-failed 真机挂账；材料项依赖 executor 集成，同 #69 口径）。证据 `2026-10-02-live/`（48 png SHA 全校、提交 a252c5e51）。Runbook 行为措辞差异（⑥拒权=保留入口+引导而非隐藏）已记录。Follow-up：真机录音链、FCM 送达、executor 集成联动项。
