# 五环境真实闭环验收矩阵（T33/#172 终局验收）

- 验收轮：2026-09-26（Asia/Shanghai），集成 HEAD **`d6b3e1bb5`**（本轮全部新跑证据均在该 HEAD 的独立 worktree `/Users/wuyongjun/.codex/worktrees/issue-140-t33-closure/WeKnora-fork01` 上取得）。
- **修复轮 r2（2026-09-27，fix-resume 1/5）**：同 worktree 在 HEAD `92718d338` 上补两项 Wave-15 评审 medium——M1 §0 六个回执 ID 的响应原文落档（§0.1，evidence/web-receipts-r2.txt）；M2 分享导入/通知权限两列专门格（§2A）。生产代码零改动。
- 验收链定义（主计划 Task 33）：同一档案 → 岗位 → 评估 → 申请 → 材料 → 本人投递 → 进展；失败恢复（scope 切换 / 未知回执 / 越权）逐环境取证。
- 本轮新跑隔离拓扑：Lite 服务器 `127.0.0.1:57828`（SQLite 一次性 DB + 一次性密钥 + `WEKNORA_CAREER_EXPORT_SIGNING_KEY`），Vite `127.0.0.1:57829`（代理→57828），iOS TLS 终结代理 `127.0.0.1:57830`→57828，微信 DevTools `--auto-port 9433`。
- 本轮证据目录：`.superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/`（相对本 worktree 根；下文以 `evidence/` 简写）。
- **结论（如实）**：Web 与微信小程序（DevTools 模拟器）两环境本轮完成端到端抽查链；iOS 完成构建/启动/登录抽查与 HTTP 边界链，App 内授权后链路存在已知缺陷 D-iOS-1（见 §iOS）；**鸿蒙原生与 Expo Android 两环境 blocked（无设备/无工具链），未验收**。公开发布门槛未达成（来源覆盖与运营核验未完成，见 source-coverage.md）。

## 0. 本轮共享验收链数据（同一档案一次走通，跨端同源）

在 57828 Lite 服务器上以一次性账号 `t33a@t33.io`（单成员 Tenant）经真实浏览器（Playwright Chromium）走通完整链：

| 链步 | 结果 | 关键标识（本轮真实回执） | 证据 |
| --- | --- | --- | --- |
| 1 档案 | 提案「毕业时间=2026-06」(manual) → 确认 → 修订 0→1→2，确认事实 1 条，提案清空 | 回执 `d2e5d8bb…`（提案）/`441a6c02…`（确认） | evidence/web-network-requests.log；浏览器快照（本轮会话记录） |
| 2 岗位 | 对话页「保存职位描述」粘贴 JD → 机会+不可变快照「已保存，待确认」 | opportunity `3e57ca16-3037-4a13-9d98-4cc141a7b8ea` / snapshot `f0a759b5-b79d-46ff-9117-d53662a3f2fd` / requestId `c666ff58…` | 同上 + evidence/web-chain-final.png |
| 3 评估 | 同快照评估 → 三值判断「待确认」+逐项证据指针（职位依据/档案依据）+ 固定版本可重开 | evaluation `34d83e35-99c3-4b12-9952-78a442326b16`（档案修订 2，规则 career-qualification-v1） | 同上 |
| 4 申请 | 选固定评估+批次「2026 秋招 A 批」→ 申请创建并关联独立 Task | application `de3fa49d-a0f3-47d0-b8f2-33c1a6182fd5` / Task(session) `ef1125b7-8f72-4704-aabd-bf3ec4c8e9c4` / Run `349d0617-d97f-4e9b-a7fd-33d785144f68` | 同上 |
| 5 材料 | 结构化正文（1 章节）草稿→发布不可变版本 V1→发布导出：PDF+DOCX 双格式核验通过（submittable） | material `beb6dd4f-5286-40c4-8634-90af12fd9f95` / export `9a9eb655-b256-4654-bde1-935c407bef87`（正文摘要 8a333f7f…；PDF 2642B / DOCX 2198B） | 同上 |
| 6 投递 | 本人确认投递（渠道=招聘网站，绑定版本 V1）；再次确认被拒并保留原记录 | 投递记录 `06817966-7027-44a8-a9a2-0a00b7322794`，确认者=user 本人 | 同上 |
| 7 进展 | 记录「已投递」事件 → 当前阶段由事件确定性投影「已投递」（事件修订 0→1） | 进展事件 `e32f4d90-5ea9-4374-8905-8ba5db4905cd` | 同上 |
| 文件下载 | 签发授权→PDF 真实落盘，SHA-256 `a65d6bd4ebfcadde…` 与页面摘要一致（PDF 1.7，1 页，2642B） | — | evidence/career-material-v1.pdf；`file`/`shasum -a 256` 实测 |
| 失败态 ① | 一次性找岗（指令「上海 前端开发 实习」）→ 诚实失败「暂无已核验来源，本次未抓取任何数据」+可同编号重试恢复 | searchId `936c6f6e-bd9f-43e2-acb1-17cce6ea256f` | 浏览器快照（本轮会话记录） |
| 失败态 ② | 未配导出签名密钥时下载 → 诚实失败「career export signing key not configured」；补配重启（同 DB/JWT）后下载成功 | — | evidence/web-cross-tenant-probes.txt 附注 |

越权（另一租户，HTTP 层）：`t33b@t33.io`（另一 Tenant 属主）读 A 的岗位快照→**404 not_found**、申请→**403 forbidden**、材料→**403 forbidden**、进展→**404**；B `/career/open`→200 仅空档案；匿名→**401**。证据：evidence/web-cross-tenant-probes.txt。

### 0.1 修复轮 r2：§0 六个回执 ID 的响应原文落档（M1 闭环）

- **缺口（如实）**：上表引用的六个截断标识（提案回执 `d2e5d8bb…`、确认回执 `441a6c02…`、机会导入 requestId `c666ff58…`、导出正文摘要 `8a333f7f…`、PDF SHA-256 `a65d6bd4…`、申请关联 Task(session) `ef1125b7…`）当轮只在浏览器会话与 network 日志（URL+status）中出现，**响应原文未落档**；当轮为一次性 SQLite DB + 一次性密钥，会话结束后已销毁，**原始 ID 的响应体不可复现**（如实标注，不虚报）。
- **补证（等价链路重跑）**：2026-09-27 在同一 worktree（HEAD `92718d338`，与 d6b3e1bb5 同源含全部功能提交）重开隔离 Lite 服务器（`go run ./cmd/server` @127.0.0.1:57828，一次性 SQLite + 一次性 JWT/AES/导出签名密钥 + 一次性 local 存储目录），以全新一次性账号经真实 HTTP 契约（与 Web 前端同一 API 面）复走同一七步链 + 失败态。**全链 EXIT=0，17×HTTP 200**；每步请求/响应原文（token/密码脱敏）逐字落档 **evidence/web-receipts-r2.txt**；PDF 真实落盘 **evidence/career-material-v1-r2.pdf**（PDF 1.7，1 页，2211B，`file`+`shasum -a 256` 实测 `e74104c4…` 与导出回执 `files[pdf].fileDigest` 一致——自洽）。
- **六 ID → r2 回执映射**：提案回执→r2 步 1b（kind=proposed）+步 1d 同 requestId 回执重放（对账实证）；确认回执→r2 步 1c（kind=confirmed，修订 0→1→2、确认事实 1 条）；机会导入→r2 步 2（opportunityId+snapshotId+requestId 原文）；导出正文摘要→r2 步 5c（`contentDigest`+双格式 `fileDigest`）；PDF SHA-256→r2 步 6b（sha256 与 5c fileDigest 相等）；Task(session)→r2 步 4（申请回执 `taskId`+`runId` 原文）。附加：一次性找岗失败态 searchId→r2 步 9（`no_vetted_sources` 诚实失败原文）；步 8 含一次真实的 revision 冲突→**同 requestId 重试成功**恢复实证；步 7b 二次投递诚实拒绝 `submission_already_confirmed`（409）。
- §0 原表各行标识保持历史记录不动；本小节为矩阵对六 ID 的**指针更新**（原文以 r2 等价链路为准）。

## 1. 环境矩阵

### 1.1 Web（浏览器）——✅ 本轮完成

| 项 | 值 |
| --- | --- |
| 环境/版本 | macOS（darwin 27.0 arm64）+ Playwright Chromium（真实浏览器，本轮实测）；Lite Go 服务器 `go run ./cmd/server`（go1.26.3）@127.0.0.1:57828（SQLite/一次性 JWT/AES/签名密钥）；Vite 7.3.6 @57829（`VITE_DEV_PROXY_TARGET=http://127.0.0.1:57828`） |
| 命令 | 见 §0 拓扑；完整命令记录于 evidence/web-network-requests.log（HTTP 面）与 wave 报告 |
| 本轮新跑链 | **完整七步链 + 失败态①② + 越权五探针**（§0 全表） |
| 失败恢复 | 找岗失败→同 searchId 重试入口；申请/材料/投递/进展均带 request-id 对账与「空间切换后旧响应失效」声明（页面原文）；PDF 下载失败→配置后恢复（失败态②） |
| 越权 | §0 越权行（B 403/404、匿名 401） |
| 既有归档 | task-3-web-browser-validation.md（注册/提案/确认/刷新/登出）、task-4-live-http-validation.md、task-7-web-integration-validation.md、task-8-live-browser-validation.md、task-10-live-browser-validation.md、各 wave-report-T{07..22}-Web.md |
| 边界（如实） | 本轮为 57828 隔离 Lite（非生产部署）；浏览器为 Playwright 管道 Chrome（非人工手指，T28 口径）；scope 切换 GUI 不可构造（单成员 Tenant），语义由 T02 http-probes P4（switch-tenant 旧空间 404/空列表）+ mobile-core 149 单测覆盖 |

### 1.2 Expo iOS（模拟器）——⚠️ 构建运行抽查完成；App 内授权后链路存在已知缺陷（不代裁）

| 项 | 值 |
| --- | --- |
| 环境/版本 | Xcode 27.0 (27A266a)；iPhone 18 Pro 模拟器 UDID `0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC`（iOS 27.0，Booted）；Expo prebuild + xcodebuild `-configuration Release -sdk iphonesimulator`（`EXPO_PUBLIC_WEKNORA_CLOUD_ORIGIN=https://127.0.0.1:57830`，`RCT_NO_LAUNCH_PACKAGER=1`）；TLS 终结代理 57830→57828（自签 CA 仅注入模拟器钥匙串，不动系统钥匙串） |
| 本轮新跑 | Release 构建于集成 HEAD `d6b3e1bb5`：`EXPO_PUBLIC_WEKNORA_CLOUD_ORIGIN=https://127.0.0.1:57830 RCT_NO_LAUNCH_PACKAGER=1 xcodebuild … -configuration Release -sdk iphonesimulator … build` → **BUILD SUCCEEDED（exit 0）**，`main.jsbundle` 内联 origin（grep=1）；ad-hoc 重签+安装+启动 → **应用真实启动至自身登录门**（origin 预填，AX 8 元素）。GUI 登录（idb 键盘注入 a33@t.io）：**服务端 auth_tokens 表 22:52:53/22:54:17 两次真实签发 token ⇒ POST /auth/login 经 TLS 200**；App UI 终态为 Update required 面（**D-iOS-1 复现**，evidence/ios/ios-spotcheck.md + 截图 2 张） |
| 受保护 Task 读取（HTTP 边界） | 属主 A `GET /api/v1/workbench/executions?limit=5`→**200**（含 career 申请创建的 Task：session `ef1125b7…` status waiting_user）；`GET …/executions/349d0617…/snapshot`→**200**。证据：evidence/ios-http-probes.txt |
| 跨租户拒绝 | B 同 snapshot→**404**；B 列表→200 空数组；匿名→**401**（evidence/ios-http-probes.txt） |
| 既有归档 | task-2-live-ios-validation.md（12 探针表 + D-iOS-1 证据链）+ task-2-evidence-ios/（http-probes-r2/r3、截图 21/22）+ wave-report-T02实测iOS.md / wave-report-T02补缺.md |
| 已知缺陷（如实，不代裁） | **D-iOS-1**：真实 iOS 运行时 GUI 登录 200 后授权链路异常回退 fail-closed「Update required」面；此后 `/auth/me`/workbench 请求 0 次。T02 R3 完整复现，修复依赖可用签名环境（本机 0 valid identities）。⇒ **Career 链在 App 内不可达（本轮如实记录边界）**；App 内 Career UI 亦因 T05/T23/T25/T27/T29/T31 blocked（§1.5）从未实现 |
| 边界 | 本轮 ad-hoc 签名、空 entitlements（本机无 Apple 开发证书）；模拟器而非真机 |

### 1.3 微信小程序（DevTools 模拟器）——✅ 本轮完成抽查

| 项 | 值 |
| --- | --- |
| 环境/版本 | 微信开发者工具 CLI 2.02.2608070（`/Applications/wechatwebdevtools.app/Contents/MacOS/cli`，tourist appid，基础库 3.16.3 模拟器）；`cli auto --auto-port 9433` + miniprogram-automator（仓库外 /tmp/wk-t33-automator，T24 先例）；构建 `WEKNORA_API_ORIGIN=http://127.0.0.1:57828 pnpm --filter @weknora/miniprogram build:weapp` EXIT=0（origin 内联 dist/common.js） |
| 本轮新跑链 | ①真实 `/auth/login`（同 Web 账号 t33a）→home；②求职工作台 `/career/discovery`：**Web 创建的已确认事实「毕业时间：2026-06 · 已确认 · 修订 2」在微信端可见（同源跨端实证）**；③一次性找岗入口+「原请求对账/空间切换失效」说明在页；④申请与材料页 `/career/application-material`：修订 2/已确认 1 条/岗位与资格评估/不可变版本说明呈现。**4/4 PASS**（驱动真实输出+4 截图：evidence/wx-devtools-spotcheck.md、wx-0*.png） |
| 失败恢复 | 页面原生控件声明+申请/材料三 seam（冲突回执/未知对账/空间切换）既有证据：T26 18/18 PASS、T24 12/12 PASS（DevTools+真实服务器）；搜索诚实失败态本轮在**同服务器 Web 端**真实取得（§0 失败态①） |
| 越权 | 既有：T24 跨租户 B 登录只见自己空档案、匿名 401/B 读 A 回执 403（DevTools 实测）；本轮 HTTP 面复证（§0 越权行，同一服务器） |
| 既有归档 | task-6-live-validation.md、wave-report-T06实测.md、wave-report-T24小程序.md、wave-report-T26小程序.md、wave-report-T28小程序.md、wave-report-T30小程序.md、wave-report-T32修复.md |
| 边界（如实） | 真机 blocked（无设备）；t-button GUI tap 不被 automator 触达（T24 定论）→t-button 动作由 172 项单测覆盖；订阅消息弹层需真实 appid+模板（tourist 不可配，T30 记录 unavailable+站内回落） |

### 1.4 鸿蒙原生 —— ❌ BLOCKED（未验收）

| 项 | 值 |
| --- | --- |
| 本轮实勘 | `command -v hdc hvigor ohpm` 全部缺失；`/Applications/DevEco-Studio.app` 不存在（evidence/android-harmony-live-probe.txt，2026-09-26 21:35 实测） |
| 既有依据 | verification/harmony-native-gate.md（T01 gate，commit 080cf7d47→5d6904c24）：无 DevEco/OH SDK/hdc/hvigor/ohpm、无 .hap、无设备；RNOH 官方矩阵不覆盖本工程 RN 0.83.10/Expo 55 |
| 结论 | 按主计划约束「鸿蒙原生验收必须有真实原生构建、设备……Android 兼容包或 WebView 不计」，本格 **blocked，不计为通过，不宣称完成**。App 内亦无鸿蒙目录/构建路径（T01 只产出 gate 文档） |

### 1.5 Expo Android —— ❌ BLOCKED（未验收）

| 项 | 值 |
| --- | --- |
| 本轮实勘 | adb 1.0.41 与 emulator 二进制、AVD（test36/test36-small）在 `~/Library/Android/sdk` **存在**，但 `adb devices` 为**空（0 设备接入）**；本工程无任何 .apk/.aab 产物（evidence/android-harmony-live-probe.txt；修正简报表「无 adb/emulator」表述——工具在、设备无） |
| 既有依据 | T02 live 验证 #11 同勘；历次移动票（T02/T05/T23/…）均未做 Android 构建/启动 |
| 结论 | **blocked（无接入设备、无 Android 构建）**。本轮按简报范围未尝试 Android 构建/启动（如实声明，交主控裁决）。连带：依赖 Android 验收面的移动票残项见 §3 裁决材料 |

## 2. 失败恢复 × 越权交叉取证汇总

| 恢复面 | Web | iOS | 微信 | 鸿蒙 | Android |
| --- | --- | --- | --- | --- | --- |
| 未知回执→同 request-id 对账 | 本轮页面声明+T04/T07/T08 既有 HTTP 实证；Web 单测全覆盖 | T02 http-probes（P2）；mobile-core 149 单测（12/33/34/37） | T24 C2、T26 D1-D3、T28 恢复入口真实 POST（DevTools） | blocked | blocked |
| scope 切换→旧响应失效 | T02 P4（switch-tenant 旧空间 404/空列表，HTTP seam）+Web 页面声明 | T02 #10 PASS（HTTP seam）；GUI 层不可构造（T02 R3 §10.2 如实） | T24 C3、T26 E1（单测+DevTools 数据面） | blocked | blocked |
| 越权（跨租户/匿名） | 本轮 5 探针（403/404/401，evidence/web-cross-tenant-probes.txt） | 本轮 3 探针（200 属主/404 跨租户/401 匿名，evidence/ios-http-probes.txt） | T24 DevTools 实测（B 空档案/403）+本轮同服务器 HTTP 复证 | blocked | blocked |
| 额度耗尽仍可读 | T21 Web（额度预估 50/50 展示+触发条件原文）；额度拒绝 429 typed 由 T21/T13 单测与 T24 F1 代理实证 | App 内不可达（D-iOS-1） | T30 规则/额度页 172 单测含覆盖 | blocked | blocked |

## 2A. 专项能力专门格：分享导入 / 通知权限（修复轮 r2 补，M2 闭环）

主计划 T33 验收明文要求「分享导入、PDF/DOCX 下载、通知权限、跨端同步与越权检查有可复现记录」。PDF/DOCX 下载见 §0 文件下载行 + §0.1 r2 步 5c/6b；跨端同步见 §1.3 本轮同源跨端实证；越权见 §0 越权行 + §2。本节补齐前两项的**分环境专门格**（每格注明证据层级：一级=集成仓库已入库归档报告，二级=本轮 r2 矩阵指针；本轮 r2 未重开微信 DevTools——按修复简报允许路径「引用 T24 归档+本轮补充指针，如实标注层级」）。

### 2A.1 分享导入（微信分享卡片 → 小程序先核对后提交）

| 环境 | 状态 | 记录 |
| --- | --- | --- |
| 微信小程序 | ✅ 已实现+DevTools 实测（一级归档） | T24「先核对后提交」：`prepareSharedImport` 纯本地预览（零网络）→ confirm 才提交原文；负载缺失→`share_payload_missing` 可恢复。DevTools 真实环境两步取证（发现并修复 query percent-encoding 缺陷，提交 `09a414c8b`）；单测 E1/E2/P1-P4/P7。证据（一级）：集成仓库 `.superpowers/sdd/2026-09-24-issue-140-implementation/wave-report-T24小程序.md`（提交 `d69c251a1`/`09a414c8b`）。（二级：本格指针，本轮 r2 未复跑 DevTools——如实标注） |
| Web | — not-applicable（能力事实） | 分享导入是微信分享卡片进入小程序的场景能力；Web 端无微信分享入口，等价入口=对话页粘贴 JD 保存（§0 步 2 本轮已实证 request-id 幂等）。无「分享导入」可测，如实标注不适用 |
| iOS | ❌ blocked（依据 §1.2） | App 内 Career 链不可达（D-iOS-1 + T05/T23/T25/T27/T29/T31 未实现），无分享导入实现面 |
| 鸿蒙 / Android | ❌ blocked（依据 §1.4/§1.5） | 未验收环境，无实现/取证 |

### 2A.2 通知权限（订阅授权弹层 / 站内待办）

| 环境 | 状态 | 记录 |
| --- | --- | --- |
| 微信小程序 | ⚠️ 真机弹层 **blocked（如实）** + 模拟器语义已证（一级归档） | 订阅消息 `wx.requestSubscribeMessage` 为原生例外（Ruling）：载荷只带模板 id。真机授权弹层 **blocked**——tourist appid 无模板可配（`REMINDER_SUBSCRIBE_TEMPLATE_IDS=[]` 需公众平台与 appid 绑定申请），模拟器内 API 在场但未配模板真实调用失败不弹层→页面**如实 unavailable+errMsg+站内回落，绝不伪造已送达**（outcome.delivered 恒 false）；拒绝/主开关/不可用路径由单测 C2（injectable invoke）钉死，**非未实现**；「查看站内待办」live 读取成功（C2+live）。证据（一级）：`.superpowers/sdd/2026-09-24-issue-140-implementation/wave-report-T30小程序.md`「订阅消息原生探针」「真机验证 blocked」节 |
| Web | ✅ 站内待办 T20 verified（一级归档） | Web 端通知形态=站内待办收件箱（无浏览器通知权限申请面）：隐私正文强制等于后端冻结模板字面量（任何插值正文在 UI 前被拒）；「推送只是提醒，站内待办才是事实源」横幅；重复触发去重（`(tenant,user,source_kind,source_id)` 唯一索引为事实源）；退订（写档案事实 `notifications.push=unsubscribed`，request-id+expected-revision）后待办仍可读+可重订阅；`delivery_failed` 时「待办已保存，以站内为准」。live 4 项检查（截图 01-04）+HTTP 层事件构造实证。证据（一级）：`.superpowers/sdd/2026-09-24-issue-140-implementation/wave-report-T20-Web.md`（T20 已 verified） |
| iOS | ❌ blocked（依据 §1.2） | App 内链路不可达（D-iOS-1 + 移动 Career UI 六票未实现），无通知权限实现面（系统通知权限申请未实现） |
| 鸿蒙 / Android | ❌ blocked（依据 §1.4/§1.5） | 未验收环境，无实现/取证 |



## 3. 终态裁决材料（供主控裁，不代裁）

| 项 | 证据指针 | 建议终态 |
| --- | --- | --- |
| T01 鸿蒙闸口 | verification/harmony-native-gate.md + 本轮复勘 evidence/android-harmony-live-probe.txt | **blocked（维持）**——需 DevEco+设备+签名 .hap 独立 spike 后重开 |
| T01 传导链 T05/T23/T25/T27/T29/T31（Expo 移动 Career UI 六票） | harmony-native-gate.md 末节「Mark T05 and later mobile tickets … blocked」；主计划 G1 行（T05 需 T01+T02 均 verified）；六票无任何 wave 报告/提交（未派发实现） | **blocked（随 T01）**——Expo 端 Career 链未实现；如产品决定放弃鸿蒙短期目标，可由主控降范围改判 parked 并另立 Android/iOS-only 票 |
| T02（iOS/Android 受认证 Task 薄切片） | task-2-live-ios-validation.md 12 探针（8 PASS/seam）+ D-iOS-1 未解 + wave-report-T02补缺.md；Android 侧 0 设备 | iOS：**verified-with-gate**（HTTP 边界全过，D-iOS-1 GUI 缺陷单列）；Android：**blocked→建议主控裁「永久缺项/降范围」**（本机长期无设备，简报已述） |
| T28-F3（真机/t-button GUI 触达） | wave-report-T28小程序.md §F3 | **parked**——补足需真机或人工录屏，环境不可得 |
| T30 残项（真机订阅弹层/模板未配置/3 项 medium 知悉） | wave-report-T30小程序.md「已知局限」+评审记录 | **parked**——订阅授权真实弹层需真实 appid+公众平台模板 |
| 各票遗留 low 清单 | 各 wave-report 末节「自查与遗留」（T21 F2 注释措辞、T24 t-button 限制、T26 三 seam GUI 层、T28 F3/F4、T32 基线 13 type errors 等） | **parked（逐项低风险）**——均不阻塞链路，建议随日常维护消化 |
| 生产空 allowlist 对发布门槛 | source-coverage.md §1-2（代码 `search_once.go:73`、`source_import.go:219`） | **gate 未达成**——需授权 source-review 记录+运营核验后才能宣称来源覆盖 |

## 4. 本轮完整门（同 HEAD 实跑）

| 门 | 命令 | 结果 |
| --- | --- | --- |
| Go 6 包 | `go test ./internal/modules/career/... ./internal/modules/workbench/service/workbench/... ./internal/database/... ./internal/router/... ./tools/architectureguard/... ./internal/handler/... -count=1`（逐包） | 全部 `ok`（career 109.8s / workbench 230.8s / database 128.0s / router 5.9s / architectureguard 2.2s / handler 4.5+2.1+101.8s），exit=0（gate-logs/go-gate.log） |
| Web 类型 | `pnpm typecheck:web` | exit=0 |
| Mobile 类型 | `pnpm --filter @weknora/mobile typecheck` | exit=0 |
| 小程序类型 | `pnpm --filter @weknora/miniprogram typecheck` | **exit 2：13 errors，全部 `features/account/pages.tsx` CommercialSummary 既有基线（该文件外 0 error）**——与 T30/T32 记录的 13 基线一致，零新增（evidence/mp-typecheck.log） |
| 小程序测试 | `pnpm --filter @weknora/miniprogram test` | `tests 172 / pass 167 / fail 0 / skipped 5`（5 skipped 为 build-output 条件跳过；本轮 build:weapp 已实际产出并通过 D1/D2 断言口径见 T24/T26） |
| Web 聚焦 | `node --import tsx --test --test-concurrency=4 'src/career/*.test.tsx' 'src/career/*.test.ts'` | **174/174 pass 0 fail**（evidence/web-career-focused.log） |
| Web 全量 | `pnpm test:web` | **未通过（争用下失败，待空闲补跑）**：21:39:47 启动（load≈68/10 核，evidence/cpu-contention-before-testweb.txt）；2h09 无进展（0% CPU/无子进程，evidence/testweb-stuck-forensics.txt）终止时缓冲尾吐出真实断言失败 `deepStrictEqual ['Bob',undefined,undefined] vs ['Bob',undefined]`（Exit status 1）；含 Bob 的四个测试文件孤立复跑（含并发 4）**75/75 全过，不可复现**→判定为争用/顺序相关失败；聚焦门 career 174/174 作为本轮 Web 测试证据 |
