# T02/#145 live iOS validation (受认证 Task 薄切片 — iOS 实测)

Date: 2026-09-25 Asia/Shanghai. Candidate: integration HEAD `7cbad8941` (detached in `/Users/wuyongjun/.codex/worktrees/issue-140-t02-live-validation/WeKnora-fork01`). Validator: frontend_validator，只读验证，**生产代码零改动**（worktree 内 git status 全程 clean；Metro 曾自动改写 `apps/mobile/tsconfig.json`，已 `git checkout --` 还原并复核）。Source: Issue #145 验收、`wave-2-t02-ios-validation-brief.md`、主计划 Task 2。

**结论先行**：iOS 侧打通了「真实构建 → 真机模拟器启动 → 应用自身 GUI 登录表单 → 经 TLS 的真实 `POST /api/v1/auth/login` 200」，但**授权后界面（任务列表/详情）在真实运行时未能建立**——登录 200 之后运行时不再发出任何后续请求（无 `/auth/me`、无 `/system/capabilities`、无 workbench 读取），UI 停在登录面，凭据不落盘（冷重启回登录门）。该缺陷在本机可稳定复现（3 次），证据链见 D-iOS-1；其根因边界（本机 ad-hoc 签名导致 keychain 写入路径异常 vs 运行时缺陷）在本环境无法进一步分离（无 Apple 签名身份可修复签名面）。跨 Tenant、未认证、会话过期按简报允许的层级在 HTTP 边界全部取证通过。

## 1. 环境（命令 + 版本 + UDID）

| 项 | 值（实测输出） |
| --- | --- |
| Xcode | `xcodebuild -version` → Xcode 27.0, Build 27A266a |
| 模拟器 | `xcrun simctl list devices available` → **iPhone 18 Pro (0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC)**, iOS 27.0 runtime，`simctl boot` 后 Booted |
| Expo CLI | `npx expo --version` → 55.0.36（工程 expo ~55.0.0 / RN 0.83.10） |
| CocoaPods | `pod --version` → 1.17.0 |
| Node/pnpm/Go | v22.22.3 / 10.28.2 / go1.26.3 darwin/arm64 |
| UI 自动化 | `idb`（`idb ui tap/text/set-value/describe-all --udid 0A38…`，`idb_companion --udid 0A38…`） |
| 截图 | `xcrun simctl io <udid> screenshot` |

## 2. API origin 拓扑（为何不是简报里的 http://127.0.0.1:57802）

简报建议 `EXPO_PUBLIC_WEKNORA_CLOUD_ORIGIN=http://127.0.0.1:57802`。实测该值**不生效**：`apps/mobile/src/screens/DeploymentLoginScreen.tsx:26`（`validatedDeploymentOrigin` 拒绝非 https）→ `officialCloudOrigin === undefined`；且手输 http origin 会被登录表单拒绝；`packages/mobile-core/src/runtime/mobile-runtime.ts:17`（`normalizeDeployment`）与 `packages/api-client/src/mobile/{runtime,task-office,resources}.ts` 的 `requireDeploymentOrigin` 三层同样强制 HTTPS。**应用对 `https://127.0.0.1:<port>` 无任何拒绝**（host 防线只存在于 Node 冒烟测试文件 `apps/mobile/src/runtime-integration-smoke.ts`，未接入运行时）。

因此实测拓扑：

- Lite 服务器照 T06 模式跑在 `http://127.0.0.1:57802`（env 全集见 §7）。
- 本地 TLS 终结代理 `https://127.0.0.1:57803` → `http://127.0.0.1:57802`（测试 harness：/tmp 下 30 行 Go ReverseProxy，SSE 透传 FlushInterval=-1）。
- 自签 CA + 127.0.0.1 叶证书（openssl，SAN=IP:127.0.0.1），CA 仅通过 `xcrun simctl keychain 0A38… add-root-cert ca.pem` 注入**模拟器**信任库（不动 macOS 系统钥匙串）。真实 app 内 `trustd` 评估通过（unified log: `trustd returned 1`，`POST /auth/login` 200 全程 TLS）。
- 构建期注入 `EXPO_PUBLIC_WEKNORA_CLOUD_ORIGIN=https://127.0.0.1:57803`：Release bundle 内 `grep -c "127.0.0.1:57803" main.jsbundle` = 1；GUI 登录表单 origin 预填该值，且出现 "Use WeKnora Cloud" 按钮（`officialCloudOrigin` 生成的直接证据，截图 10）。

## 3. 构建与启动（真实记录）

1. `npx expo run:ios --configuration Debug` → **环境阻断**：`CommandError: Can't determine id of Simulator app; the Simulator is most likely not installed on this machine.`（`mdfind`/`osascript` 均找不到 Simulator.app；`/Applications/Xcode.app/Contents/Developer/Applications/Simulator.app` 不存在。simctl 本身可用。）
2. 改走 prebuild + xcodebuild：`expo prebuild`（CNG，`apps/mobile/ios/` 为 gitignored 生成物）后
   `xcodebuild -workspace WeKnora.xcworkspace -scheme WeKnora -configuration Debug -sdk iphonesimulator -destination 'id=0A38…' -derivedDataPath build CODE_SIGNING_ALLOWED=NO build` → BUILD SUCCEEDED（756+ 编译步，0 error）。但 Debug 产物无内嵌 bundle 且本环境无法注入 Metro URL：`-RCT_jsLocation localhost:8081` 启动参数、`xcrun simctl spawn <udid> defaults write com.weknora.mobile RCT_jsLocation`、`SIMCTL_CHILD_RCT_METRO_PORT/HOST` 三种方式后 app 仍报 `No script URL provided. … unsanitizedScriptURLString = (null)`，且 Metro 日志无任何请求（TCP 层 `simctl spawn <udid> nc -z 127.0.0.1 8081` 成功——不是网络隔离）。根因未完全分离（候选：ATS 对 http 探测、packager 探测路径），如实记录。
3. Debug+内嵌 bundle 尝试：对 gitignored 生成物打补丁（pbxproj 去掉 Debug 强制 SKIP_BUNDLING + `FORCE_BUNDLING=1` + AppDelegate `bundleURL()` 直读内嵌 bundle，T06「dist-only unblock」同例）→ dev=true bundle 启动即红屏 `Cannot create devtools websocket connection`，不可用。
4. **最终取证产物 = Release 配置真实构建**：`xcodebuild … -configuration Release … build`（带 `EXPO_PUBLIC_WEKNORA_CLOUD_ORIGIN=https://127.0.0.1:57803 RCT_NO_LAUNCH_PACKAGER=1`）→ BUILD SUCCEEDED，`main.jsbundle` 3.44MB 内嵌、origin 已内联。首个产物以 `CODE_SIGNING_ALLOWED=NO` 构建（无签名）：可安装可启动，但 keychain 读即报 `securityd … -34018 "Client has neither application-identifier nor keychain-access-groups entitlements"`。去掉该开关重签（ad-hoc）后 keychain 读正常；再尝试手工注入 entitlements（keychain-access-groups / application-identifier）+ ad-hoc 重签 → app 直接无法启动（`FBSOpenApplicationServiceErrorDomain code=3`）；本机 `security find-identity -v -p codesigning` = **0 valid identities**，无法用真实开发签名修复。最终运行态 = ad-hoc 签名、空 entitlements（keychain 读 OK、写异常，见 D-iOS-1）。
5. `simctl install` + `simctl launch` → **应用真实启动至自身登录门**（截图 04/10）：`Sign in to WeKnora`、origin 预填 `https://127.0.0.1:57803`、Email/Password 输入框、`Sign in`、`Continue with single sign-on`。

## 4. 观测表（PASS / blocked + 层级声明）

| # | 探针 | 层级 | 结果 |
| --- | --- | --- | --- |
| 1 | iOS 真实构建（Release，prebuild+xcodebuild） | 设备 | **PASS**（BUILD SUCCEEDED；Debug 构建亦 SUCCEEDED 但 Metro 不可注入，见 §3.2） |
| 2 | 应用真实启动至登录门 | GUI | **PASS**（截图 04/10；AX 树 8 元素与源码 `DeploymentLoginScreen` 一致） |
| 3 | 构建期 origin 注入生效 | GUI | **PASS**（表单预填 + "Use WeKnora Cloud" 按钮；bundle 内联计数 1） |
| 4 | 真实 GUI 登录（键盘输入邮箱密码 → Sign in） | GUI | **PASS（HTTP 200）**：服务器日志多次 `POST /api/v1/auth/login … 200`（04:53:07、05:28:24、05:41 起 a2@t2.io 多次），请求经 TLS 代理（自签 CA 仅模拟器信任），CFNetwork/trustd 佐证。截图 05（填好凭据）。注意：键盘注入下长邮箱 React state 易截断（环境/自动化限制，见 §6-E4），最终采用 8 字符邮箱 `a2@t2.io` 稳定复现 |
| 5 | 登录后读取受保护任务列表/详情 | GUI | **BLOCKED（D-iOS-1）**：登录 200 后 app 不再发任何请求（`/auth/me`、`/system/capabilities`、workbench 全部为 0 次，服务器日志与 CFNetwork task 日志双重佐证），UI 停留登录面（截图 12/14 结构=登录面），凭据未持久化（冷重启回登录门）。同凭据同服务器的 HTTP 层读取全部 200（见 #8） |
| 6 | 未认证读取被拒 | HTTP + GUI | **PASS**：无 bearer `GET /api/v1/workbench/executions` → **401**、`…/t02run0001/snapshot` → **401**、`GET /api/v1/career/open` → 401（启动自检）。GUI 层：未登录态 app 只渲染登录门，受保护 /tasks 路由在未授权快照下渲染 null（`apps/mobile/src/composition.ts` `MobileTasks`，代码级声明） |
| 7 | 跨 Tenant 拒绝（另一 Tenant 用户对同一 Task） | HTTP（声明：未做第二账号 GUI 登录，因 #5 授权面未建立） | **PASS**：Tenant 2 用户 B `GET /workbench/executions?limit=5` → 200 `{"items":[]}`；B `GET /workbench/executions/t02run0001/snapshot`（属主 Tenant 3）→ **404**；属主（Tenant 3）同 run → 200 |
| 8 | 属主读取受保护 Task（列表 + 详情快照） | HTTP | **PASS**：`GET /workbench/executions?limit=5` → 200 含 `t02run0001 / T02 受保护任务实测 / succeeded`；`GET /…/t02run0001/snapshot` → 200（task.title、execution.run_status=succeeded、events[]）；`GET /workbench/overview` → 200 |
| 9 | 会话过期/凭据失效（scope 语义） | HTTP 运行时 seam（声明：GUI 层不可构造，因 #5） | **PASS**：A 登录取 token → 200 读取；停服并以**同 DB、新 JWT_SECRET** 重启（14s 起）→ 旧 token 重放 `GET /workbench/executions` → **401 `Unauthorized: invalid or expired token`**；同凭据重新登录 → 200，新 token 列表 → 200 含 fixture。迟到/旧响应的 GUI 不可见性未验证（声明为限制） |
| 10 | 空间切换后旧数据不可见 | — | **NOT VERIFIED**：单用户单 Tenant fixture，且 #5 阻断授权面；单测（77/77）覆盖该语义 |
| 11 | Android 设备证据 | 设备 | **环境门槛（但既往记录需修正）**：`which adb`/`emulator` 不在 PATH（既往「无 adb」裁定即源于此）；实勘 `~/Library/Android/sdk` **存在** platform-tools/adb（37.0.1）、emulator 二进制与 AVD `test36`、`test36-small`，`adb devices` = 0 设备。本任务按简报范围未尝试 Android 构建/启动（如实声明，交主控裁决） |
| 12 | 移动端单测/类型 | 仓库 | `pnpm --filter @weknora/mobile test` → **77/77 pass, 0 fail**；`pnpm --filter @weknora/mobile typecheck` → 0 error |

## 5. 缺陷与发现（只记录，不改生产代码）

- **D-iOS-1（主要阻断）：真实 iOS 运行时登录成功后授权链路死寂。** 证据链：服务器只见 login 200（`05:28:24`、`05:41` 起多次），其后 `/auth/me`、`/system/capabilities`、workbench 请求 0 次（server.log 全文 grep）；app 侧 unified log 同窗口仅 1 个 CFNetwork task（即 login 本身）；UI 停留登录面（截图 12/14 像素带结构 = 登录面 + 键盘；UpgradeRequired/HomeScreen 结构均不符）；再次提交后进入 fail-closed 的 "Update required" 面（截图 15）；冷重启无凭据恢复。定位：`mobile-runtime.ts` `signIn` 在 `passwordLogin` 成功后、`authenticate`（首个动作 `me()`）之前的区段——`persistCredential` 的 SecureStore 写入在本机（ad-hoc 签名、空 entitlements、keychain 读正常但无 `SecItemAdd` 活动、无 -34018 报错）表现为「既不 resolve 也不 reject」或静默 `return state`。本机无 Apple 签名身份（0 identities），无法以合规签名修复环境面；注入 entitlements 的 ad-hoc 重签使 app 无法启动。**修复派发建议**：在可用签名环境（真实开发证书 + `expo run:ios` 完整链）复跑；若仍复现，则属 mobile-runtime/SecureStore 组合的运行时缺陷，单测（注入 fake credentialStore）不可见。
- **D-iOS-2：简报的本地联调指引与实现不符。** `EXPO_PUBLIC_WEKNORA_CLOUD_ORIGIN=http://127.0.0.1:57802` 因三层 HTTPS 强校验而静默失效（§2）；官方文档/简报需改述为「https origin（本地自签 + 模拟器信任）」或显式提供 dev 例外。这不改变生产行为（强制 HTTPS 是既有设计），但让「模拟器连本地 Lite」的路径目前只能走本报告的 TLS 代理方案。
- **D-iOS-3（UX 观察）：登录失败的呈现是 "Update required"。** 错误凭据（400）经 `signIn` catch → `safe()` → `UpgradeRequiredScreen`（"https://127.0.0.1:57803 cannot open this version of WeKnora."），与协议失配文案完全同屏（截图 15）。用户无法从 UI 区分「密码错了」与「版本不兼容」；属 fail-closed 设计的副作用，交产品裁决。
- **环境注记 E1**：本机无 Simulator.app GUI（`expo run:ios` 无法完成最后一步），simctl/idb 全功能可用。
- **环境注记 E2**：`adb`/`emulator` 实际存在于 `~/Library/Android/sdk`（仅不在 PATH）且存在 2 个 AVD——既往「永久不可测（无 adb）」的环境裁定在本机不成立；Android 侧是否补测由主控裁决。
- **环境注记 E4（自动化保真度）**：HID 注入长字符串时 RN 受控 TextInput 的 React state 尾部易截断（AX 值与 state 不一致；`idb ui set-value` 只改 AX 不改 state）。属自动化环境限制，非 app 缺陷；实测以 8 字符邮箱 + 「补删同步」手法绕过。报告中的 GUI 取证均以服务器侧请求体为准。

## 6. 证据文件（同目录 `task-2-evidence-ios/`）

01-app-initial（Debug 无 Metro URL 红屏）｜04/10（登录门，origin 预填）｜05（GUI 填入凭据）｜12/14（login 200 后 UI 停留登录面）｜15（fail-closed Update required）。服务器/构建原始日志与证书/fixture 均在一次性 /tmp 目录，取证后已销毁（凭据/密钥不入库）。

## 7. 复现提纲

服务器：`DB_DRIVER=sqlite DB_PATH=<tmp>/wk-t02.db RETRIEVE_DRIVER=sqlite STREAM_MANAGER_TYPE=memory STORAGE_TYPE=local LOCAL_STORAGE_BASE_DIR=<tmp>/storage SERVER_HOST=127.0.0.1 SERVER_PORT=57802 DISABLE_REGISTRATION=false APP_EXTERNAL_URL=http://127.0.0.1:57802 JWT_SECRET=<rand> SYSTEM_AES_KEY=<32> WEKNORA_ARTIFACT_SIGNING_KEY=<64hex> go run ./cmd/server`；注册 `a2@t2.io`（Tenant 3）与 `t02-other@example.test`（Tenant 2）；迁移后 SQLite 插入 `sessions(id=t02task0001, tenant_id=3, user_id=<A uuid>, engine_type='builtin', archived_at NULL)` 与 `agent_runs(tenant_id=3, run_id=t02run0001, session_id=t02task0001, owner_id=<A uuid>, request_id/assistant_message_id/request_hash 任意合法, engine_type='trpc', driver='platform', status='succeeded', snapshot='{"agent_id":"builtin-default"}', deadline 未来)`。TLS 代理：openssl 自签 CA/叶（SAN IP:127.0.0.1）→ `simctl keychain <udid> add-root-cert ca.pem` → Go ReverseProxy `ListenAndServeTLS("127.0.0.1:57803") → 127.0.0.1:57802`。构建：`cd apps/mobile && npx expo prebuild -p ios`，`EXPO_PUBLIC_WEKNORA_CLOUD_ORIGIN=https://127.0.0.1:57803 xcodebuild -workspace ios/WeKnora.xcworkspace -scheme WeKnora -configuration Release -sdk iphonesimulator -destination 'id=0A38…' build`；`simctl install/launch`。预期：登录门可达、GUI login 200 稳定；授权面在本机卡死（D-iOS-1）；HTTP 层 401/404/过期拒绝全部可复现。会话过期：停服换 JWT_SECRET 重启（同 DB）→ 旧 token 401。

## 8. 清理

server（57802）与 TLS 代理（57803）、Metro、idb_companion、adb daemon 均已停止（端口复核 down）；模拟器内 `com.weknora.mobile` 已卸载；/tmp 一次性目录（DB/密钥/证书/harness）已删除；CA 私钥随目录销毁（模拟器内残留的 3 天期 root cert 无私钥、无风险）；worktree git status clean（基线 `7cbad8941`），仅新增本报告与证据文件。
