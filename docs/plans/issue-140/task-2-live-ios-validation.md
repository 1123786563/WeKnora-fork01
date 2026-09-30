# T02/#145 live iOS validation (受认证 Task 薄切片 — iOS 实测)

Date: 2026-09-25 Asia/Shanghai. Candidate: integration HEAD `7cbad8941` (detached in `/Users/wuyongjun/.codex/worktrees/issue-140-t02-live-validation/WeKnora-fork01`). Validator: frontend_validator，只读验证，**生产代码零改动**（worktree 内 git status 全程 clean；Metro 曾自动改写 `apps/mobile/tsconfig.json`，已 `git checkout --` 还原并复核）。Source: Issue #145 验收、`wave-2-t02-ios-validation-brief.md`、主计划 Task 2。

**结论先行**（已按 R1 评审修正，见 §9）：iOS 侧打通了「真实构建 → 真机模拟器启动 → 应用自身 GUI 登录表单 → 经 TLS 的真实 `POST /api/v1/auth/login` 200」，但**授权后界面（任务列表/详情）在真实运行时未能建立**——登录 200 之后运行时不再发出任何后续请求（无 `/auth/me`、无 `/system/capabilities`、无 workbench 读取），UI 落入 fail-closed 的 **"Update required"（UpgradeRequiredScreen）面**（即 `signIn` 的 catch → `safe()` 异常路径，R1 评审 OCR + 本轮像素带比对确证；初版报告误读为「停留登录面」，已在 §9 更正），凭据不落盘（冷重启回登录门）。该缺陷在本机可稳定复现（R1 三次 + R2 两次），证据链见 D-iOS-1；其根因边界（本机 ad-hoc 签名导致 keychain 写入路径异常 vs 运行时缺陷）在本环境无法进一步分离（无 Apple 签名身份可修复签名面）。跨 Tenant、未认证、会话过期、**空间切换后旧 Task 不可见（seam 层）**在 HTTP 边界全部取证通过（脱敏原始记录：`task-2-evidence-ios/http-probes-r2.txt`）。

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
| 4 | 真实 GUI 登录（键盘输入邮箱密码 → Sign in） | GUI | **PASS（HTTP 200）**：服务器日志多次 `POST /api/v1/auth/login … 200`（R1 04:53:07、05:28:24、05:41 起；R2 复测两次 `email":"a2@t2.io"` 200），请求经 TLS 代理（自签 CA 仅模拟器信任），CFNetwork/trustd 佐证。填入态截图：05（长邮箱尝试，后弃用）与 **17（a2@t2.io 最终复现）**。注意：键盘注入下长邮箱 React state 易截断（环境/自动化限制，见 §5-E4），最终采用 8 字符邮箱 `a2@t2.io` 稳定复现 |
| 5 | 登录后读取受保护任务列表/详情 | GUI | **BLOCKED（D-iOS-1）**：登录 200 后 app 不再发任何请求（`/auth/me`、`/system/capabilities`、workbench 全部为 0 次，服务器日志与 CFNetwork task 日志双重佐证），UI 落入 fail-closed 的 "Update required" 面（截图 12/14/18/20，R1 评审 OCR + R2 像素带比对确证；下半部为 iOS 系统保存密码对话框——表单提交成功的旁证），凭据未持久化（冷重启回登录门；R3 更正：原引截图 19 实为 launch 早期全白屏、不能作证，有效证据改为截图 22，见 §10）。同凭据同服务器的 HTTP 层读取全部 200（见 #8、http-probes-r2.txt P2） |
| 6 | 未认证读取被拒 | HTTP + GUI | **PASS**：无 bearer `GET /api/v1/workbench/executions` → **401**、`…/t02run0001/snapshot` → **401**、`GET /api/v1/career/open` → 401（启动自检）。GUI 层：未登录态 app 只渲染登录门，受保护 /tasks 路由在未授权快照下渲染 null（`apps/mobile/src/composition.ts` `MobileTasks`，代码级声明） |
| 7 | 跨 Tenant 拒绝（另一 Tenant 用户对同一 Task） | HTTP（声明：未做第二账号 GUI 登录，因 #5 授权面未建立） | **PASS**：Tenant 2 用户 B `GET /workbench/executions?limit=5` → 200 `{"items":[]}`；B `GET /workbench/executions/t02run0001/snapshot`（属主 Tenant 3）→ **404**；属主（Tenant 3）同 run → 200 |
| 8 | 属主读取受保护 Task（列表 + 详情快照） | HTTP | **PASS**：`GET /workbench/executions?limit=5` → 200 含 `t02run0001 / T02 受保护任务实测 / succeeded`；`GET /…/t02run0001/snapshot` → 200（task.title、execution.run_status=succeeded、events[]）；`GET /workbench/overview` → 200 |
| 9 | 会话过期/凭据失效（scope 语义） | HTTP 运行时 seam（声明：GUI 层不可构造，因 #5） | **PASS**：A 登录取 token → 200 读取；停服并以**同 DB、新 JWT_SECRET** 重启（14s 起）→ 旧 token 重放 `GET /workbench/executions` → **401 `Unauthorized: invalid or expired token`**；同凭据重新登录 → 200，新 token 列表 → 200 含 fixture。迟到/旧响应的 GUI 不可见性未验证（声明为限制） |
| 10 | 空间切换后旧数据不可见 | HTTP 运行时 seam（声明：GUI 层不可构造，因 #5） | **PASS（seam，R2 补测，http-probes-r2.txt P4）**：属主 A（Tenant 1，可见 fixture）经 `POST /api/v1/auth/switch-tenant {tenant_id:3}` 换签至其第二空间 → 新 token `GET /workbench/executions` → 200 `{"items":[]}`（旧空间 Task 不可见）、`GET …/t02run0001/snapshot` → **404**；对照：Tenant 1 token 仍 200 可见 fixture（作用域双向隔离）。迟到响应的 GUI 不可见性仍未验证（单测覆盖该语义） |
| 11 | Android 设备证据 | 设备 | **环境门槛（但既往记录需修正）**：`which adb`/`emulator` 不在 PATH（既往「无 adb」裁定即源于此）；实勘 `~/Library/Android/sdk` **存在** platform-tools/adb（37.0.1）、emulator 二进制与 AVD `test36`、`test36-small`，`adb devices` = 0 设备。本任务按简报范围未尝试 Android 构建/启动（如实声明，交主控裁决） |
| 12 | 移动端单测/类型 | 仓库 | `pnpm --filter @weknora/mobile test` → **77/77 pass, 0 fail**；`pnpm --filter @weknora/mobile typecheck` → 0 error |

## 5. 缺陷与发现（只记录，不改生产代码）

- **D-iOS-1（主要阻断）：真实 iOS 运行时登录成功后授权链路立即异常回退到 fail-closed 面。**（定性已按 R1 评审修正：初版「promise 挂起/静默 return state」假说与实际 UI 证据矛盾，予以撤回。）证据链：服务器只见 login 200（R1 `05:28:24`、`05:41` 起多次；R2 复测两次 `email":"a2@t2.io"` 200），其后 `/auth/me`、`/system/capabilities`、workbench 请求 0 次（server.log 全文 grep，R2 复核 `grep -c "auth/me"` = 0）；app 侧 unified log 同窗口仅 1 个 CFNetwork task（即 login 本身）；**UI 落在 UpgradeRequiredScreen（"Update required / …cannot open this version of WeKnora / Sign out"）**——R1 评审 swift+Vision OCR 对截图 12/14 的两轮确证（与 `apps/mobile/src/screens/UpgradeRequiredScreen.tsx:12-19` 逐字一致）+ R2 像素带比对（截图 18/20 顶部四带 5.4/11.5/19.7/17.0 vs 已知 Update-required 截图 15 的 5.2/10.8/19.4/16.3，且与登录门 04 的 5.6/1.1/11.6/… 显著不同；下半部差异带为 iOS 系统保存密码对话框，本身是密码表单提交成功的旁证）；冷重启无凭据恢复（R3 更正：原引截图 19 实为 launch 早期全白屏、拍摄过早不能作证；有效证据为 R3 截图 22——冷重启 UI 稳定后呈登录门且零恢复请求，见 §10）。`upgrade-required` 面只能由 `signIn` 的 catch → `safe()`（`packages/mobile-core/src/runtime/mobile-runtime.ts:302-314`，异常路径）产生——即 **passwordLogin 成功返回之后、`authenticate` 首个动作 `me()` 之前有异常抛出**，最可疑点是 `persistCredential` 的 SecureStore 写入（本机 ad-hoc 签名、空 entitlements：keychain 读正常（SecItemCopyMatching 无错）、无 SecItemAdd 活动、无 -34018 报错）。本机无 Apple 签名身份（0 identities），无法以合规签名修复环境面；注入 entitlements 的 ad-hoc 重签使 app 无法启动。**修复派发建议**：在可用签名环境（真实开发证书 + `expo run:ios` 完整链）复跑；若仍复现，则属 mobile-runtime/SecureStore 组合的运行时缺陷，单测（注入 fake credentialStore）不可见。附：取证期间 app 转场后 `idb ui describe-all` 多次仅返回 1 元素（AX 快照被系统对话框干扰，harness 限制），故屏幕定性采用 OCR + 像素带比对双源，方法学在此如实声明。
- **D-iOS-2：简报的本地联调指引与实现不符。** `EXPO_PUBLIC_WEKNORA_CLOUD_ORIGIN=http://127.0.0.1:57802` 因三层 HTTPS 强校验而静默失效（§2）；官方文档/简报需改述为「https origin（本地自签 + 模拟器信任）」或显式提供 dev 例外。这不改变生产行为（强制 HTTPS 是既有设计），但让「模拟器连本地 Lite」的路径目前只能走本报告的 TLS 代理方案。
- **D-iOS-3（UX 观察）：登录失败的呈现是 "Update required"。** 错误凭据（400）经 `signIn` catch → `safe()` → `UpgradeRequiredScreen`（"https://127.0.0.1:57803 cannot open this version of WeKnora."），与协议失配文案完全同屏（截图 15）。用户无法从 UI 区分「密码错了」与「版本不兼容」；属 fail-closed 设计的副作用，交产品裁决。
- **环境注记 E1**：本机无 Simulator.app GUI（`expo run:ios` 无法完成最后一步），simctl/idb 全功能可用。
- **环境注记 E2**：`adb`/`emulator` 实际存在于 `~/Library/Android/sdk`（仅不在 PATH）且存在 2 个 AVD——既往「永久不可测（无 adb）」的环境裁定在本机不成立；Android 侧是否补测由主控裁决。
- **环境注记 E4（自动化保真度）**：HID 注入长字符串时 RN 受控 TextInput 的 React state 尾部易截断（AX 值与 state 不一致；`idb ui set-value` 只改 AX 不改 state）。属自动化环境限制，非 app 缺陷；实测以 8 字符邮箱 + 「补删同步」手法绕过。报告中的 GUI 取证均以服务器侧请求体为准。

## 6. 证据文件（同目录 `task-2-evidence-ios/`）

- R1：01-app-initial（Debug 无 Metro URL 红屏）｜04/10（登录门，origin 预填）｜05（GUI 填入长邮箱凭据——该次尝试其后被弃用）｜12/14（login 200 后的 Update required 面 + iOS 保存密码对话框；R1 评审 OCR 确证）｜15（fail-closed Update required，键盘已收起、无对话框，作带比对基准）。
- R2（R1 评审修正轮）：16（全新安装后的登录门）｜17（**a2@t2.io + 12 位密码的 GUI 填入态**）｜18（提交 login 200 后的屏幕：顶部=Update required 面 + 下半部保存密码对话框）｜19（冷重启后屏幕——R3 更正：实为 launch 早期全白屏、非登录门，不作「凭据未持久化」证据，该结论改由 R3 截图 22 支撑，见 §10）｜20（二次登录后的终态，与 18 同构）。
- `http-probes-r2.txt`：P1 未认证 401×3｜P2 属主列表/snapshot/overview 200（含 fixture 行）｜P3 跨 Tenant 空列表 + 404｜P4 空间切换 seam（switch-tenant 换签后旧 Task 空列表/404 + 双向对照）｜P5 JWT_SECRET 轮转后旧 token 401、重登 200、（含租户偏好语义注记与回切对照）。凭据/token 均脱敏。
- 服务器/构建原始日志与证书/fixture 在一次性 /tmp 目录，取证后销毁（凭据/密钥不入库）；HTTP 结论以入库的脱敏 transcript 为准。

## 7. 复现提纲

服务器：`DB_DRIVER=sqlite DB_PATH=<tmp>/wk-t02.db RETRIEVE_DRIVER=sqlite STREAM_MANAGER_TYPE=memory STORAGE_TYPE=local LOCAL_STORAGE_BASE_DIR=<tmp>/storage SERVER_HOST=127.0.0.1 SERVER_PORT=57802 DISABLE_REGISTRATION=false APP_EXTERNAL_URL=http://127.0.0.1:57802 JWT_SECRET=<rand> SYSTEM_AES_KEY=<32> WEKNORA_ARTIFACT_SIGNING_KEY=<64hex> go run ./cmd/server`；注册 `a2@t2.io`（属主，R2 顺序下 Tenant 1）与 `t02-other@example.test`（Tenant 2）、`t02-space@example.test`（Tenant 3，空间切换目标）；迁移后 SQLite 插入 `sessions(id=t02task0001, tenant_id=<属主租户>, user_id=<A uuid>, engine_type='builtin', archived_at NULL)`、`agent_runs(tenant_id=<属主租户>, run_id=t02run0001, …同上…, status='succeeded', snapshot='{"agent_id":"builtin-default"}', deadline 未来)`、以及 `tenant_members(user_id=<A uuid>, tenant_id=<T3>, role='contributor', status='active')`（空间切换 seam 所需）。TLS 代理：openssl 自签 CA/叶（SAN IP:127.0.0.1）→ `simctl keychain <udid> add-root-cert ca.pem` → Go ReverseProxy `ListenAndServeTLS("127.0.0.1:57803") → 127.0.0.1:57802`。构建：`cd apps/mobile && npx expo prebuild -p ios`，`EXPO_PUBLIC_WEKNORA_CLOUD_ORIGIN=https://127.0.0.1:57803 xcodebuild -workspace ios/WeKnora.xcworkspace -scheme WeKnora -configuration Release -sdk iphonesimulator -destination 'id=0A38…' build`；`simctl install/launch`。预期：登录门可达、GUI login 200 稳定；登录 200 后 app 落入 Update required 面、零后续请求（D-iOS-1）；HTTP 层 401/404/过期拒绝/空间切换不可见全部可复现（对照 http-probes-r2.txt）。会话过期：停服换 JWT_SECRET 重启（同 DB）→ 旧 token 401。空间切换：`POST /api/v1/auth/switch-tenant`（**Authorization 用 access token**，body 带 refresh_token）→ 新 token 下旧 Task 不可见。

## 8. 清理

server（57802）与 TLS 代理（57803）、Metro、idb_companion、adb daemon 均已停止（端口复核 down）；模拟器内 `com.weknora.mobile` 已卸载；/tmp 一次性目录（DB/密钥/证书/harness）已删除；CA 私钥随目录销毁（模拟器内残留的 3 天期 root cert 无私钥、无风险）；worktree git status clean（基线 `7cbad8941`），仅新增本报告与证据文件。（R2 轮结束后按同一清单再次清理，见 §9。）

## 9. R1 评审修正记录（round 1 review fixes）

R1 评审提出 F1–F6。F1（high）与 F2/F3/F4（medium）已修复；F5/F6（low）按要求仅记录不修：

- **F1（核心观测失实）— 已修正**：接受评审 OCR 结论——截图 12/14 为 UpgradeRequiredScreen 而非登录面；本报告 §结论/表 #5/D-iOS-1 均已改写：登录 200 后 UI 走 `signIn` catch → `safe()` 的 fail-closed "Update required" 面，初版「promise 挂起/静默 return state」假说撤回。R2 独立复核：重新装 app、重新 GUI 登录（`a2@t2.io` → 服务器 `status_code=200`），提交后截图 18/20 的顶部像素带与已知 Update-required 基准（截图 15）一致（5.4/11.5/19.7/17.0 vs 5.2/10.8/19.4/16.3），与登录门（04：5.6/1.1/11.6/…）显著不同；下半部为 iOS 系统保存密码对话框（评审 F2 指出，亦为表单提交成功的旁证）。屏幕定性的方法学（OCR + 像素带比对，AX 因系统对话框干扰不可靠）已在 D-iOS-1 中声明。
- **F2（HTTP 证据不可复核）— 已修复**：R2 全量重跑 P1–P5 并入库脱敏原始记录 `task-2-evidence-ios/http-probes-r2.txt`（真实 token 只上线、不入文；含 P4 空间切换与 P5 轮转及租户偏好注记）。
- **F3（空间切换 NOT VERIFIED）— 已补测（seam 层）**：给属主 A 建立第二空间成员关系（`tenant_members` 直插 + `POST /api/v1/auth/switch-tenant`），换签后旧 Task 空列表/404、原空间 token 对照仍可见；表 #10 已更新，迟到响应 GUI 不可见仍声明未验证。
- **F4（a2@t2.io 无填入态截图）— 已补齐**：截图 17（a2@t2.io + 12 位密码填入态）、18（提交后）、19（冷重启后屏幕，其「登录门」定性已于 R3 更正为全白屏，见 §10）、20（二次登录终态）；服务器侧 200 时刻与之对应（http-probes-r2.txt 记录环境同 DB；GUI 请求体见 §表 #4/§5 的 R2 复核行）。
- **F5（Debug→Release 口径）/F6（Android SDK 实存）**：维持 R1 记录，交主控裁决。
- R2 重跑校验：`pnpm --filter @weknora/mobile test` → 77/77 pass, 0 fail；`pnpm --filter @weknora/mobile typecheck` → 0 error（输出摘录见 wave 报告）。

## 10. R2 补缺记录（round-2 evidence gaps，2026-09-25 07:00–07:30，基线 d88cd513a 换基）

本轮（R3 轮次、报告 §10）针对集成台账维持 T02 running 的两项缺口补证：①F3「切换空间后迟到响应不可见」的运行时证据；②截图 19 定性更正。生产代码零改动（本轮唯一写入 = 本报告 + 证据目录 + http-probes-r3 transcript）。端口隔离：Lite 服务器 **57807**、TLS 测试代理 **57808**（代理为本轮 /tmp 测试 harness，非生产代码）。

### 10.1 截图 19 定性更正（R1 评审遗留的次要证据声明）

- **验证方法**：PIL 像素统计（本轮实测输出）。截图 19 主色 `#ffffff 99.5%`、8 带亮度 250.9–255.0（全白）；对照登录门截图 16 = `#f2f2f2 94.0%`、顶带 231.9；Update-required 基准 15 = `#f2f2f2 95.0%`；Update-required+保存密码对话框 20 = `#c2c2c2 61.1%`。**结论：截图 19 为全白屏，原「登录门 + 空表单」定性错误，予以更正**（§4 表 #5、§5 D-iOS-1、§6、§9 F4 共 4 处已同步更正）。
- **成因解释（本轮运行时复核）**：R3 轮 `simctl launch` 后 6s 截图仍为 99.5% 白屏、约 18s 后登录门才渲染完成（Release bundle 首启解析慢）——19 是「launch 后立即截图、UI 未及渲染」的产物。
- **「凭据未持久化」结论的证据修复**：该结论原以截图 19 为旁证，19 作废后改由本轮新证据支撑——截图 22（冷重启 terminate→launch→22s 稳定后：主色 `#f2f2f2 94.0%` + AX 树 8 元素含 "Sign in to WeKnora"/"Sign in"/"Continue with single sign-on"，双源确认登录门）+ 冷重启窗口服务器零请求（无任何持久化凭据恢复会话的尝试）。结论本身不变，证据基础替换为有效截图。

### 10.2 F3「切换空间后迟到响应不可见」——运行时取证与诚实结论

**结论：GUI 层（真实 app 内）的迟到响应场景在本环境不可构造；不可构造的运行时证据已取得，语义覆盖由单测 + 静态 seam + HTTP 层时序旁证三源替代。**

**(a) 本轮运行时复现（证明阻断仍存在，非引用旧轮结论）**：全新构建（origin=https://127.0.0.1:57808，bundle 内联计数 1、GUI origin 字段预填实见）→ 全新安装 → GUI 填表（email `a2@t2.io`、密码 18 圆点完整）→ 提交：服务器 07:22:07 `POST /api/v1/auth/login` **200**（active_tenant=1，memberships 含 Tenant 3 contributor——fixture 生效）；此后 server.log 全量 grep：`/auth/me`=0、`/system/capabilities`=0、workbench app 请求=0（仅剩 1 条请求即 login 本身）；UI 终态截图 21 与 R2 已知基准（截图 20：Update-required + 保存密码对话框）逐带一致（181/194/194/209.8/212.6/197.5/194/194 vs 180.3/194/194.1/209/212.4/197.5/194/194）。**D-iOS-1 完整复现**：授权后 workbench UI 不可达 → app 内无法发起 Task 读取、亦无空间切换入口 → 「慢响应晚于空间切换」的 GUI 时序无构造起点。

**(b) 单测层证据（本轮实跑）**：`npx tsx --test 'packages/mobile-core/src/**/*.test.ts'` → **149/149 pass**；其中 `packages/mobile-core/src/runtime/mobile-runtime.test.ts` 40/40，F3 语义 4 用例全绿：ok 33 `a late authorized response after a scope change is dropped`（迟到授权响应在 scope 变化后被丢弃，reject `RUNTIME_SCOPE_CHANGED`）、ok 34 `a late tenant verification cannot override a completed later switch`（迟到的租户验证不能覆盖已完成的后续切换）、ok 37 `a stream opened before a scope change is dropped, and its chunks never flush`、ok 12 `late responses after sign out are ignored`。

**(c) 静态 seam（代码指针）**：`packages/mobile-core/src/runtime/mobile-runtime.ts` epoch 守卫——scope 变化（切租户 `activateTenant`/切部署/登出）递增 epoch（:137/:143/:498），`current()` 校验 requestEpoch 与活动部署（:148），状态发布前重检（:290 `if (requestEpoch !== epoch) return state`）；`scope-lease.ts` 可撤销 lease（revoke 后 ScopedStore 每次访问复验）。

**(d) HTTP 层时序旁证（`task-2-evidence-ios/http-probes-r3-late-response.txt`，脱敏入库）**：观察层级声明为**传输层**。t0=07:24:28 属主 A（Tenant 1）发起列表读取（测试代理人为保持响应 6000ms）；t1=07:24:30 在读挂起期间 `switch-tenant → 3` 成功，新作用域立即读取：列表 `[]`、`t02run0001/snapshot` 404（服务端作用域即时隔离）；t3=07:24:34 **迟到的 Tenant 1 响应到达**（携带 `t02run0001/succeeded` 旧空间数据）——晚于空间切换约 4s。该 transcript 证明迟到响应确实会晚于空间切换到达客户端传输层；其在 app 内的不可见性由 (b) 单测 + (c) seam 覆盖。**两证据层级互补，共同支撑 F3 语义，不单独冒充 GUI 证据。**

### 10.3 本轮证据清单（同目录 `task-2-evidence-ios/`）

21-r3-post-login-update-required.png（R3 GUI login 200 后终态，与 20 同构）｜22-r3-cold-relaunch-stable-login-gate.png（冷重启 UI 稳定后登录门，凭据未持久化的有效截图证据）｜http-probes-r3-late-response.txt（迟到响应 HTTP 时序 + 观察层级声明 + 单测/seam 指针）。

### 10.4 遗留与裁决

- GUI 层迟到响应证据维持「不可构造」结论（根因 D-iOS-1 未解，修复依赖可用签名环境复跑——见 D-iOS-1 修复派发建议）；T02 终态（verified / 维持 running）由主控依本节证据裁决。
- 表 #9/#10 中「迟到响应 GUI 不可见性未验证（声明为限制）」的表述由本节终局结论取代：GUI 层不可构造（§10.2a），语义覆盖见 §10.2b–d。
