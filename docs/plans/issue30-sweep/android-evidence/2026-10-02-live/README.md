# T40（#70）Android Release + 七工作流模拟器矩阵 — 2026-10-02/03 live

## 环境

- 分支 main @ 26963b08b（D1-D9 全修）；后端从 HEAD 构建（`/tmp/t40r/server-26963b0`，go build 无标签）+ #31 轮 OIDC 环境配方（`OIDC_AUTH_*` 端点显式覆盖：token/userinfo/jwks 指向 Casdoor :18000，issuer 对齐 Casdoor 自宣 `http://localhost:8000`）。
- 部署面：nginx TLS `https://192-168-3-33.nip.io:8443`（mkcert CA）→ 后端 :8084；Casdoor :18000（weknora/t01live）。
- AVD test36（API 36，emulator-5554）；mkcert CA 双 store 残留自 #31 轮（system f160f0cf.0 + zygote64/conscrypt apex 命名空间覆盖仍活）；模拟器无代理（`settings get global http_proxy`=null）——iOS 轮的 CoreTunnel 代理问题不影响 Android 模拟器（宿主代理只影响宿主进程）。
- 登录：真实 OIDC 全链（custom tab → Casdoor t01live 真实认证 → code → 后端 token/JWKS/issuer 验签 → `weknora://oidc` 回跳 → `POST /auth/mobile/exchange`）。

## Release 构建与签名（工程验收口径）

- `./gradlew assembleRelease`（JDK 17 + ANDROID_HOME），`EXPO_PUBLIC_WEKNORA_CLOUD_ORIGIN=https://192-168-3-33.nip.io:8443` 构建期内联（bundle 内 1 处命中）。
- 本地 keystore `/tmp/t40r/weknora-release.keystore`（PKCS12，alias `weknora-release`，**不入库**），signingConfig 走 `WK_RELEASE_*` 环境变量（`android/` 整体 gitignored）。
- 证书 SHA-256：`07:F8:88:0C:F6:18:B5:FC:53:F7:C7:38:8D:9A:34:72:AF:26:FF:C6:02:B9:31:56:CC:95:A3:AE:27:18:61:35`；apksigner `Verifies`（v2 scheme）；APK SHA-256 见 `logs/apk-sha256.txt`（104,049,092 B）。

## 七工作流判词（模拟器口径=用户裁定 2026-10-02）

| # | 工作流 | 判定 | 证据 | 说明 |
|---|---|---|---|---|
| 1 | 系统返回 | **evidenced** | `w1-*.png` | 逐级回退 detail→tasks→home（w1-back-1/2）；`weknora://oidc` 热返回保持授权态（w1-warm-return）+ 冷返回经 SecureStore 恢复授权面（w1-cold-return）；登出→back 退桌面无复活、重启仍登录页（w1-signed-out/after-back/revive-check） |
| 2 | 后台限制 | **evidenced** | `w2-*.png` + nginx 序列 | 后台 25s（launcher 可见）→前台恢复即发 `GET /workbench/inbox 200` 权威同步；detail「已同步」+ 显式 failed 结算短码（w2-detail-synced）；deviceidle `mState=ACTIVE`（模拟器不进 deep idle，如实记录） |
| 3 | 通知 | **evidenced（两路径）/ blocked-env（FCM 送达）** | `13/14-*.png`、`w3-*.png` | POST_NOTIFICATIONS 系统弹窗（API 36 模拟器实弹）授权路径=Allow→授权态不受扰；拒绝路径=revoke→再弹→Don't allow→回授权面无崩溃；`appops POST_NOTIFICATION: allow`。FCM 真实送达与设备注册 no-token 短路=blocked-env（无推送凭据部署，#67 disabled provider 形态） |
| 4 | 文件 URI | **部分 evidenced / blocked-env（录音捕获+材料产出）** | `w4-*.png` | 麦克风授权弹窗（w4-mic-dialog）；录音原生捕获在模拟器稳定 `capture-failed`（fail-closed 文案、手打输入不受影响、无崩溃——w4-recording/dictate-retry；expo-audio AudioRecorder prepare 在 AVD 上原生失败，无日志可见，真机复验挂账）。任务材料页 UI 可达+刷新无崩溃但任务无材料（空态，与 iOS 轮 #69 flow7 同口径）；材料签名链接下载/分享=blocked（依赖 executor 产出材料，workbench→graph 集成边界） |
| 5 | Keystore | **evidenced** | `w1-cold-return.png` + `logs/manifest-tree.txt`、`logs/backup-rules.txt` | SecureStore 凭据落盘：force-stop 后冷启动/冷深链直接恢复授权面（=凭据+vault 持久化）；离线草稿加密保存跨断网留存（w7-back-online，Hermes 纯 JS AES-GCM D5 修复实证）。备份排除：manifest `fullBackupContent`/`dataExtractionRules` 指向 `secure_store_backup_rules`（include sharedpref/. + **exclude sharedpref/SecureStore**）与 `secure_store_data_extraction_rules` |
| 6 | 麦克风 | **evidenced（拒权 fail-closed）/ blocked-env（授权→录音→转写）** | `w6-*.png`、`w4-mic-dialog/recording` | 拒权路径：pm revoke→DICTATE 保留（非隐藏）→系统再弹→Don't allow→denied 可行动文案「麦克风权限被拒绝；可直接输入文字，或在系统设置授权后重试」+RETRY DICTATION+手打不受影响（w6-mic-denied-copy）。**口径差异如实记录**：runbook 措辞「拒权→听写入口隐藏」，实现语义为「adapter 缺失才隐藏；权限拒绝保留入口+文案引导」（spec §8.3/`dictation-view.ts`）——建议 runbook 措辞对齐实现。授权→录音→转写=同 W4 模拟器 capture-failed 边界，真机挂账 |
| 7 | 弱网恢复 | **evidenced** | `w7-*.png` + nginx 序列 | 断网（svc wifi/data disable）→草稿写入→SUBMIT 被 Offline Gate 拒+「当前离线：目标已加密保存为草稿；恢复联网后请手动点击提交确认发送」（w7-offline-submit-gate）→联网恢复→草稿原样留存（w7-back-online）→手动 SUBMIT→`POST /workbench/executions 202`→任务列表出现 `T40_offline_draft_weaknet_recovery •failed`（w7-final-list；failed=已知 workbench→graph executor 边界的显式结算，D8/D9 语义在场：结算 settled+显式失败短码） |

## OIDC 通道修复记录（本轮环境侧，非 app 缺陷）

后端从 HEAD 重启时最初 OIDC 403（`OIDC login is disabled`）：t39 轮后端进程未带 OIDC 环境变量。恢复 #31 配方后逐个排掉三个端点问题（均有 nginx/backend 日志实证）：
1. token endpoint 404（discovery 宣告 :8000）→ 显式 `OIDC_AUTH_TOKEN_ENDPOINT=:18000`；
2. JWKS 404 → `OIDC_AUTH_JWKS_URI=http://localhost:18000/.well-known/jwks`；
3. id_token issuer 不符 → `OIDC_AUTH_ISSUER_URL=http://localhost:8000`（Casdoor 自宣 issuer）。
终态 `POST /auth/mobile/exchange` 成功（nginx 序列 `logs/nginx-oidc-workflow-sequence.log`）。

## 已知边界/残留（如实）

- FCM 送达、真机麦克风录音链、真机深链/通知行为=blocked-env（无凭据/无真机）。
- `GET /workbench/executions/{id}/delivery` 500：`workbench_notifications` 表不存在（迁移未跑，#31 轮已知残留）。
- 断网提示文案在联网恢复后未即时清除（UI stale 文案，提交动作不受影响）。
- 任务执行事件流=workbench→graph 集成边界：runs 以 `executor_failed` 显式结算（本轮两任务均 failed 结算，详情页「执行：failed•结算：settled•已接收事件：0」）。

## 产物清单

- 截图 48 张（本目录）；`logs/`：nginx 序列、logcat 全量、manifest 树、备份规则、APK/keystore 指纹、UI dump 样本。
- 后端启动配方：`/tmp/t40r/start-backend.sh`（含 OIDC 端点终态）；token 观测代理 `/tmp/t40r/oidc-proxy.mjs`（18080，调试用，未在生产链路）。
