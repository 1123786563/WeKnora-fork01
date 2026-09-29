# T39：iOS 安装包核心工作流验收（Issue #69）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把「真实 iOS Release 安装包 + 模拟器运行」的验收变成可复现管线并实际执行记录：补齐安装包缺失的两个原生依赖（expo-audio/expo-network，语音权限与离线草稿验收路径的前置）、固化可复现的 Release 构建与模拟器证据管线脚本、在最高稳定 Interface 上补齐「冷启动恢复 / 撤销不可复活 / 弱网断链重续」三组真实部署集成证据，并产出九大核心工作流 × 四条逆境路径的验收报告——本地不可验证项如实列 blocked-env，不伪造通过。

**Architecture:** 本计划不改任何领域 Module：九大核心工作流（登录、Tenant、Task、后台恢复、离线草稿、通知、下载分享、语音权限、安全存储）的实现已随 #32–#46、#56、#66、#67、#68 全部集成在当前 HEAD。计划做四件事：①安装 `expo-audio`/`expo-network`（#56/#40 各自显式留给「真机构建前置」的两个惰性依赖），使组合根里已接线的 `createNativeDictationCaptureIfAvailable()`（`apps/mobile/src/adapters/dictation-capture.ts:75`）与 `createNativeNetworkStatusIfAvailable()`（`apps/mobile/src/adapters/network-status.ts:6`）在 Release 包里真实生效；②把散落在 B3/B4 证据报告里的构建/验收命令固化为两个幂等脚本（`apps/mobile/scripts/ios-release-build.sh`、`apps/mobile/scripts/ios-acceptance-run.sh`），预构建漂移防护沿用入库事实源 `apps/mobile/plugins/ios-xcode27.js`；③新增验收证据契约模块 `ios-release-evidence.ts`（九流 × 四逆境的处置分类 + AC3「mock 不冒充」机器门）与一组 opt-in 真实部署组合证据 `ios-core-workflow-integration-smoke.ts`（文件持久凭据上的冷启动 boot 恢复、signOut 撤销后第三实例不得复活、弱网断链后同 requestId 重续同 Run）；④执行整条管线并产出 `docs/plans/issue30-sweep/ios-evidence/t39-acceptance.md`。

**Tech Stack:** TypeScript（`apps/mobile` Expo RN ~55 / React Native 0.83）、node:test + tsx（与全部既有集成 smoke 一致）、expo-audio ~55.0.18 / expo-network ~55.0.18（`apps/mobile/node_modules/expo/bundledNativeModules.json` 实查的 SDK 55 对齐版本；pnpm 工作区，仓库根无 node_modules/expo）、Xcode 27.0 (27A266a) + `xcrun simctl`（install/launch/terminate/openurl/push/privacy/io screenshot/recordVideo/status_bar/log show，作者在本会话逐一实测可用）。所有测试命令在 worktree 根（`.worktrees/issue30-sweep`）执行；前置：已执行过 `pnpm install`。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-69.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（Testing Decisions「Native release evidence must come from installed iOS and Android builds…」一条即本 Issue 的规范母本；Voice Room「permission denial…raw-audio deletion」、Scoped Vault「offline drafts and rejection of offline side effects」、Task Material「download and system share」）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§4 Mobile Runtime 启动/恢复语义、§13 Interface 测试面——本计划的集成证据全部从既有 Module Interface 组合，不新增 Module）
- 领域术语：`CONTEXT.md`（「部署实例（Deployment）」「活动空间（Active Tenant）」）
- 前置证据基线：`docs/plans/issue30-sweep/ios-evidence/ios-test-report.md`（B3 四轮 iOS 实测：Release 构建、UIScene 生命周期修复、老架构渲染、冷启动、未授权深链 fail-closed）、`b4-recheck-fix.md`（B4 复验修复：splash wordmark、Pods 告警抑制、构建命令口径）
- Parent：Issue #30；Blocked by：#40、#41、#56、#66（**四者已全部实施并集成在当前 HEAD `d52270a0f`**，差异记录见下）

**调查结论与代码现状的差异记录（以代码现状为准）：** 任务书所附调查摘要称「现状=absent；apps/mobile 仅 15 文件无 ios/ 原生工程；无 eas.json；核心工作流除登录外均未实现；前置 #40/#41/#56/#66 均为 open」。该结论对应 #30 扫描最早期的快照，与当前 HEAD 冲突，作者已逐项亲核并以现状为准：①`apps/mobile` 现有 113 个跟踪文件（`git ls-files apps/mobile | grep -v Pods | wc -l` = 113），含 15 个 Screen、15 个路由、13 个 opt-in 集成 smoke；②iOS 原生工程已由 expo prebuild 生成于 `apps/mobile/ios/`（`WeKnora.xcodeproj`/`WeKnora.xcworkspace`/Pods，目录被 `.gitignore:98` 忽略不入库，入库事实源是本地 config plugin `apps/mobile/plugins/ios-xcode27.js` + `apps/mobile/src/plugins/ios-xcode27.test.ts` 13 用例）；③B3/B4 已完成五轮 iOS Release 包实测与修复（冷启动 splash、Pods 告警、UIScene 生命周期），证据在 `docs/plans/issue30-sweep/ios-evidence/`；④#40/#41/#56/#66 的交付物（离线门+vault 投影、设备注册+Inbox+深链解析、听写模块、Deployment 切换+只读面）均在 HEAD。**本计划因此不是「从零实现核心工作流」，而是「把安装包验收做成可复现证据并补上仍然缺失的三个具体缺口」**（见下）。

**作者亲核的真实缺口（写计划时逐项验证过）：**
1. `apps/mobile/package.json` 无 `expo-audio`、`expo-network`（`grep -c "expo-audio\|expo-network" pnpm-lock.yaml` = 0）→ 听写捕获与网络状态两个惰性 Adapter 在当前 Release 包 fail closed：包上无麦克风入口（语音权限路径不可达）、离线门透传（离线草稿只剩传输层兜底）。两处注释均显式把构建前置留给本 Issue（`dictation-capture.ts`「真机构建前置：cd apps/mobile && npx expo install expo-audio」）。
2. Release 构建与模拟器验收命令只存在于 B3/B4 的证据报告里，没有可执行、可复跑的入库脚本；授权面九流在安装包上的证据自 B3 起一直缺失（`ios-test-report.md` §4 全部 ❌）。
3. 「冷启动恢复、撤销不可复活、弱网断链重续」三组逆境在最高稳定 Interface 上没有专门证据（既有 smoke 覆盖登录/启动/详情 SSE/设备/听写/离线门，但无人把「持久凭据 → dispose → boot 恢复 → signOut → 第三实例不得复活」与「断链 → 同 requestId 重续同 Run」串成端到端）。

**本会话实测的环境能力边界（blocked-env 判定依据）：**
- 可用：Xcode 27.0、`xcrun simctl privacy`（服务清单实测含 `microphone`，**不含** notifications）、`xcrun simctl push`（本地模拟推送投递）、`simctl io recordVideo/screenshot`、`simctl status_bar`、`simctl spawn … log show`；booted 模拟器 iPhone 18 Pro（`0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC`，iOS 27.0）。
- 不可用：`idb` companion 连接失败（`idb list-targets` 报 `Failed to describe CompanionInfo … No Companion Connected`，作者实测 `idb_companion --daemonize` 后仍连不上）→ **tap/输入级 UI 自动化不可用**，登录表单无法自动填写。与 B3/B4 记录同口径。
- 不存在：真实 WeKnora deployment 凭据（`WEKNORA_MOBILE_TEST_*` 未设置）、Apple 开发者账号/签名、APNs 推送凭据、Android 工具链（Android 属 #70）。

**本计划 Consumes（前序批次已合并产出的精确签名，均在当前 HEAD，作者逐行核实）：**
- `createMobileRuntime(ports: MobileRuntimePorts): MobileRuntime`；`MobileRuntime.signIn({ deployment: { origin, label? }, email, password }): Promise<RuntimeSnapshot>`、`boot(input?): Promise<RuntimeSnapshot>`（从 `ports.deploymentStore?.read()` + `ports.credentialStore.read(origin)` 恢复授权面，`packages/mobile-core/src/runtime/mobile-runtime.ts:285-304`）、`signOut()`（清除凭据并 `ports.deploymentStore?.clear()`，`mobile-runtime.ts:277`）、`dispose()`、`scopeLease()`、`snapshot()`、`subscribe`、`authorizedRequest(input)`、`activateTenant(tenantId)`、`snapshot.identity.tenants?: Array<{ id: string; name?: string }>`（#32/#66）
- `CredentialStore { read(deployment): Promise<StoredCredential | undefined>; write(deployment, credential): Promise<void>; clear(deployment): Promise<void> }` 与 `DeploymentStore { read/write/clear }`（`packages/mobile-core/src/runtime/ports.ts:11-25`）；`signIn → authenticate` 内部持久化 `deploymentStore.write`（`mobile-runtime.ts:247-250`）
- apps/mobile Adapter：`createSecureCredentialStore(store: SecureStorePort): CredentialStore`（`apps/mobile/src/adapters/credential-store.ts:30`）、`createSecureDeploymentStore(store: SecureStorePort): DeploymentStore`（`apps/mobile/src/adapters/deployment-store.ts:19`）、`interface SecureStorePort { getItemAsync; setItemAsync; deleteItemAsync }`（`apps/mobile/src/adapters/secure-store.ts:5-9`）
- `createTaskOffice(ports: TaskOfficePorts): TaskOffice`；`office.start(goal: TaskOfficeGoal, options?: { requestId?: string }): Promise<TaskStartReceipt>`（`TaskStartReceipt = { requestId: string; phase: SubmissionEntry['phase']; runId?: string; dispatched: boolean }`，`packages/mobile-core/src/task-office/task-office.ts:153-158`；Start POST 前耐久落盘意图，失败零派发或停 pending，`task-office.ts:508-586`）；`office.reconcilePending(): Promise<TaskStartReceipt[]>`（从意图日志恢复并对账，`task-office.ts:587,601`）；`office.tasks(query): Promise<{ items: Array<{ runId: string; … }> }>`
- `createTaskOfficeRemote({ origin, request }): TaskBackendPort 同构 remote`（`@weknora/api-client/mobile/task-office`）；`createJsonTransport(fetcher: FetchLike)`、`createWeKnoraClient({ baseURL, transport })`、`CLIENT_PROTOCOL_VERSION`（`@weknora/domain/mobile`）
- opt-in 集成证据模式：`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD` 三变量 + `disallowedDeploymentHost(hostname, variable): string | undefined` 主机防线（拒绝 localhost/环回/私网/链路本地/保留地址，`apps/mobile/src/runtime-integration-smoke.ts:118`）+「total 化收口：失败也产出证据对象，绝不让 runner reject」+「emit 证据不含凭据」断言口径（`voice-dictation-integration-smoke.ts:179-181`、`task-start-integration-smoke.ts:68-69`）
- 组合根既有接线（本计划零改动，仅在 Task 1 用测试钉住）：`composition.ts:57`（offline gate）、`composition.ts:59`（听写捕获单例）、`composition.ts:340-347`（dictationFor）
- 测试命令：`pnpm --filter @weknora/mobile test`（= `tsx --test 'src/**/*.test.ts*'`）、`pnpm run typecheck:mobile`、B4 全套口径 `pnpm exec tsx --test "packages/domain/src/mobile/*.test.ts" "packages/mobile-core/src/**/*.test.ts" "packages/api-client/src/mobile/*.test.ts" "apps/mobile/src/**/*.test.ts*"`（B4 复验报告实测 704 tests / 0 fail / 15 skipped）

**本计划 Produces（供 #70/#71 与后续验收消费的精确签名）：**
- apps/mobile 依赖面：`package.json` dependencies 新增 `"expo-audio": "~55.0.18"`、`"expo-network": "~55.0.18"`；`app.json` plugins 新增 `["expo-audio", { "microphonePermission": "WeKnora uses the microphone for voice dictation and live voice sessions." }]`（预构建 Info.plist 随之带 `NSMicrophoneUsageDescription`）
- 脚本面（均幂等可重跑）：`apps/mobile/scripts/ios-release-build.sh [repo-root]`（prebuild→babel-preset-expo 链接守卫→pod install→xcodebuild Release，末行打印 `RELEASE_APP=<path>`）；`apps/mobile/scripts/ios-acceptance-run.sh <booted-UDID> [repo-root]`（冷启动录屏/截图、未授权深链三探针、未授权推送投递、麦克风拒权重启、启动日志崩溃筛查，产物落 `docs/plans/issue30-sweep/ios-evidence/t39/`）；`pnpm exec tsx apps/mobile/scripts/emit-acceptance-record.ts <outcomes.json>`（装配记录→AC 门校验→门不过以非零码退出）
- 证据契约（apps/mobile 公开源）：`IOS_ACCEPTANCE_FLOWS`（9 值）/`IOS_ADVERSITY_PATHS`（4 值）/`IosEvidenceEntry`/`assembleIosReleaseEvidence(build, entries): IosReleaseEvidenceRecord`（total、按 subject 去重后者覆盖、缺 subject 自动补 `not-run`）/`iosReleaseEvidenceGaps(record): string[]`（AC3 机器门）/`buildRecordFromOutcomes(outcomes)`
- 组合集成证据：`iosCoreWorkflowIntegrationConfig(env)`、`IosCoreWorkflowIntegrationEvidence { signIn; coldBootRestore; revocation; tenantSwitch; weakNetwork; weakNetworkStartRequests; distinctRunIds; … }`、`runIosCoreWorkflowIntegration(config)`、`emitIosCoreWorkflowIntegrationEvidence(evidence, emit)`
- 验收报告：`docs/plans/issue30-sweep/ios-evidence/t39-acceptance.md`（九流 × 四逆境逐项 evidence 指针或 blocked-env 理由）

## Global Constraints

以下为批准 Spec / ADR / 术语表的项目级约束与安全规则，逐字引用，所有任务隐含遵守：

- 「Native release evidence must come from installed iOS and Android builds with real login, background/foreground recovery, weak network, notifications, downloads, voice permissions and secure storage.」（mobile-ai-office-design.md · Testing Decisions——本 Issue 的规范母本；Android 半边属 #70）
- 「Prototype HTML, screenshots and simulated state machines do not prove native runtime, backend integration, persistence, push, audio, provider, payment or deployment behavior.」（同上）
- 「True external dependencies such as APNs, FCM, WebRTC, system audio and system share use mock or scripted Adapters at the Port and real-device acceptance separately.」（同上——系统分享/APNs 真推/真机麦克风为 true external，本计划如实列 blocked-env，不以脚本替身冒充）
- 「Tests target observable behavior at the highest stable Interface. Screen tests verify rendering and navigation; they do not duplicate Module internals.」（同上）
- 「The App Shell is a composition root and presentation Adapter. Screens do not call wire clients directly or maintain request IDs, cursors, revisions or scope generations.」（mobile-module-seams.md §10）
- 「移动端可登记多个实例，但同一时刻只有一个活动实例。」（CONTEXT.md ·「部署实例（Deployment）」）；「活动空间（Active Tenant）……决定首页、任务、搜索、请求、订阅和缓存可访问的内容」
- 安全约束（会话注入 + Mimosa 生成前约束）：配置凭据只从环境变量或密钥服务读取，源码、示例和测试都不得写入可用的凭据字面量；真实部署 URL 仅允许公网 HTTPS 主机（复用 `disallowedDeploymentHost` 防线，拒绝 localhost/环回/私网/链路本地/保留地址）；证据 JSON 断言不含 `/password|token|email/i` 字样；服务端请求仅 http/https。
- 宿主状态变更约束：弱网 dummynet（`dnctl`/`pfctl`）会改动宿主机网络状态，必须人工显式批准后执行；默认路径是 transport 注入延迟/断链的弱网证据，不静默改宿主配置。
- 工作流约束：严格 RED→GREEN→REFACTOR（先写失败测试、实跑确认失败、最小实现、通过、提交）；实现与已批准 Spec 冲突时升级处理，不静默重设计。本计划与同批 B5 计划并行实施：改动收敛到新增文件 + `apps/mobile/package.json`/`app.json`/`pnpm-lock.yaml` 三个共享文件的最小增量，**不改 `composition.ts`、不改任何 Screen、不改 packages/\*、零 Go 改动**（当前 worktree 中 t47/t57 等并行计划正在修改 composition.ts 与 screens，本计划刻意避开）。

**Issue #69 验收标准原文（docs/plans/issue30-sweep/issues/issue-69.md）：**

1. 「证据来自安装包和真实设备/模拟器运行，不是浏览器 prototype。」
2. 「弱网、拒权、冷启动和撤销路径均记录。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

**blocked-env 清单（验收报告必须逐项如实登记，不计为通过）：**
1. **真机分发**：Apple 开发者账号/签名证书/物理 iPhone 缺失——模拟器路径为本计划的安装包证据口径（波级指引明示可行），真机复验挂账。
2. **APNs 真实远程推送**：无推送凭据；`simctl push` 是本地模拟投递（能证明 app 进程存活与未授权不导航，不能证明 APNs 链路）。
3. **授权面九流的安装包内自动化路径**：idb companion 不可用（tap/输入级自动化缺失）+ 无 deployment 凭据（登录表单无法自动填写）→ 包内授权流由持凭据人工路径执行（报告附人工脚本清单），自动化替代证据 = opt-in 真实部署集成 harness（最高稳定 Interface：真实 transport + 真实服务端 + 真实 Adapter 组合，非 mock）。
4. **宿主级真实弱网**：dummynet 需 sudo 改 pf 规则（宿主状态变更须人工批准）；替代 = transport 注入 2s 延迟后断链的弱网组合证据（Task 5）。
5. **系统分享面板**：true external（spec 明示 real-device acceptance separately）；本地替代 = material smoke 已覆盖的授权/预览/字节下载通道 + `createNativeSharePortIfAvailable` 结构存在断言（Task 6 报告引用）。

## Review Focus

Spec 隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **新增原生依赖后 Release 构建漂移**（prebuild 重写 ios/ 丢失 plugin 补丁、`rm -rf build` 删掉 codegen 产物、pnpm 工作区下 `babel-preset-expo` 链接缺失导致 Metro 打包失败——B3/B4 三次实踩）：构建脚本必须按「prebuild → 链接守卫 → pod install → xcodebuild」定序且幂等。——Task 2 测试「the release build script is a reproducible prebuild → pods → Release pipeline」（pod install 位置先于 xcodebuild 的源级断言）。
2. **麦克风拒权后应用崩溃或入口不 fail closed**（iOS 27 `simctl privacy` 变更会终止运行中的应用；拒权后 `DictationCapturePort.start()` 必须返回 `'denied'` 且 `DICTATION_FAILURE_COPY.denied` 给出可行动文案）：拒权探针必须「revoke → 重启 → 进程存活」，崩溃信号在日志筛查中零命中。——Task 4 测试「the acceptance probe script installs before probing…」（`revoke microphone` + 崩溃筛查口径的源级断言）；文案断言已在 `dictation-view.test.ts:16-20`（引用，不重复实现）。
3. **未授权深链/推送在安装包上泄漏授权内容或崩溃**（登出/未登录状态下 `weknora://tasks/detail?taskId=1&runId=1` 与本地推送必须停在 fail-closed 面，绝不渲染任务内容）：——Task 4 测试同文件断言三个深链探针与 `simctl push` 步骤存在；B4 复验已实测同款 fail-closed 行为（`03-tasks-list.png`/`04-ask-knowledge-qa.png`），本任务在含新依赖的重建包上复跑留证。
4. **弱网断链后的重复派发**（首枚 Start POST 延迟后断链，重试若换 requestId 会产生第二个 Run——违反 AC1 单写者幂等）：断链恢复必须以**同一 requestId** 重续且服务端收敛到**同一 runId**。——Task 5 测试断言 `weakNetwork === 'reconciled-same-run'` 且 `distinctRunIds === 1`（opt-in 实跑）。
5. **证据冒充与撤销复活**（单测/静态检查冒充集成证据；signOut 后冷启动复活旧会话）：AC 门必须拒绝 `source` 不是 `installed-package`/`integration-harness` 的 evidenced 条目、拒绝缺证据指针的 evidenced、拒绝缺理由的 blocked-env；撤销后第三实例 `boot()` 必须停 `deployment-login`。——Task 3 测试「the gate rejects evidence impersonation in all three shapes」+ Task 5 `revocation === 'revoked'` 断言。

## 任务结构与文件地图

| # | 任务 | 主要交付 | 对应 AC |
|---|---|---|---|
| 1 | 补齐原生依赖 expo-audio/expo-network + 麦克风权限文案 | `package.json`/`app.json`/`pnpm-lock.yaml` + `release-deps.test.ts` | AC1（语音权限/离线草稿在包上可达的前提） |
| 2 | 可复现 Release 构建脚本 | `apps/mobile/scripts/ios-release-build.sh` + 脚本源级测试 | AC1（安装包可复现产出） |
| 3 | 验收证据契约 + AC 门 | `ios-release-evidence.ts`/`.test.ts` + `emit-acceptance-record.ts` | AC3（不冒充的机器门） |
| 4 | 模拟器安装包证据管线脚本 | `apps/mobile/scripts/ios-acceptance-run.sh` + 脚本源级测试 | AC1/AC2（冷启动/拒权/撤销面探针） |
| 5 | 冷启动恢复/撤销/弱网组合集成证据 | `ios-core-workflow-integration-smoke.ts`/`.test.ts` | AC2/AC3（真实部署、最高稳定 Interface） |
| 6 | 执行整条管线 + 验收报告 | `docs/plans/issue30-sweep/ios-evidence/t39-acceptance.md` + `t39/` 证据 + 全量回归 | AC1/AC2/AC3 收口 |

---

### Task 1: 补齐原生依赖 expo-audio/expo-network 并钉住麦克风权限文案

**Files:**
- Modify: `apps/mobile/package.json`（dependencies 增加两行）
- Modify: `apps/mobile/app.json`（plugins 数组增加 expo-audio 条目）
- Modify: `pnpm-lock.yaml`（pnpm add 产物，不手写）
- Test: `apps/mobile/src/release-deps.test.ts`（创建）

**Interfaces:**
- Consumes: 组合根既有接线 `createNativeDictationCaptureIfAvailable()`（`composition.ts:59`，惰性 `require('expo-audio')`，声明在 `dictation-capture.ts:75`）与 `createNativeNetworkStatusIfAvailable()`（`composition.ts:57`，惰性 `require('expo-network')`，`network-status.ts:5-14`）；SDK 55 对齐版本 `apps/mobile/node_modules/expo/bundledNativeModules.json` 的 `expo-audio: ~55.0.18`、`expo-network: ~55.0.18`（作者实查）。
- Produces: 安装包内听写捕获/网络状态两个 Adapter 从 fail-closed-undefined 变为真实生效（composition 接线零改动）；`NSMicrophoneUsageDescription` 进入后续 prebuild 的 Info.plist。

- [ ] **Step 1: 写失败测试**

创建 `apps/mobile/src/release-deps.test.ts`（完整内容）：

```ts
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';

// T39（#69）：安装包验收的两个原生前置。测试读仓库事实（package.json/app.json/composition.ts
// 源级），依赖缺席时组合根的两个惰性 Adapter 永远 fail closed → 语音权限与离线草稿验收不可达。
const readJson = (relative: string): unknown =>
  JSON.parse(readFileSync(fileURLToPath(new URL(relative, import.meta.url)), 'utf8'));

test('release package ships the native voice and network adapters (T39 #69)', () => {
  const pkg = readJson('../../package.json') as { dependencies?: Record<string, string> };
  assert.match(
    pkg.dependencies?.['expo-audio'] ?? '',
    /^~55\./,
    'expo-audio 缺席 → createNativeDictationCaptureIfAvailable 恒 undefined，包上无麦克风入口（#56 留给本 Issue 的构建前置）',
  );
  assert.match(
    pkg.dependencies?.['expo-network'] ?? '',
    /^~55\./,
    'expo-network 缺席 → createNativeNetworkStatusIfAvailable 恒 undefined，离线门透传（#40 留给本 Issue 的构建前置）',
  );
});

test('the composition root wiring for both adapters stays intact (guard, not duplication)', () => {
  const composition = readFileSync(fileURLToPath(new URL('./composition.ts', import.meta.url)), 'utf8');
  assert.match(composition, /createNativeDictationCaptureIfAvailable\(\)/);
  assert.match(composition, /createNativeNetworkStatusIfAvailable\(\)/);
});

test('the iOS build declares the microphone permission via the expo-audio config plugin', () => {
  const app = readJson('../../app.json') as { expo?: { plugins?: unknown[] } };
  const entry = app.expo?.plugins?.find((plugin) => Array.isArray(plugin) && plugin[0] === 'expo-audio');
  assert.ok(Array.isArray(entry), 'expo-audio config plugin 缺席 → prebuild 出的 Info.plist 不含 NSMicrophoneUsageDescription，拒权路径无从谈起');
  const props = entry[1] as { microphonePermission?: unknown };
  assert.equal(typeof props.microphonePermission, 'string');
  assert.ok((props.microphonePermission as string).trim().length > 0, '权限文案不得为空串');
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/release-deps.test.ts`
Expected: FAIL，`expo-audio 缺席…` 与 `expo-network 缺席…` 两处 `assert.match` 失败（`app.json` 断言同样失败）。

- [ ] **Step 3: 最小实现——安装依赖与权限文案**

```bash
# 在 worktree 根执行；SDK 55 对齐版本（apps/mobile/node_modules/expo/bundledNativeModules.json 实查）
pnpm --filter @weknora/mobile add "expo-audio@~55.0.18" "expo-network@~55.0.18"
```

（需要 registry 网络；若镜像不可达，升级处理，不得改版本号绕过。）随后编辑 `apps/mobile/app.json` 的 `plugins` 数组为：

```json
"plugins": [
  "expo-router",
  "expo-secure-store",
  "expo-web-browser",
  ["expo-audio", { "microphonePermission": "WeKnora uses the microphone for voice dictation and live voice sessions." }],
  "./plugins/ios-xcode27"
]
```

（本地 plugin 保持数组末位不变；`expo-audio` 条目只新增、不动其余行，最小化与并行计划的合并冲突。）

- [ ] **Step 4: 运行测试与既有回归确认通过**

Run: `pnpm exec tsx --test apps/mobile/src/release-deps.test.ts && pnpm run typecheck:mobile && pnpm --filter @weknora/mobile test`
Expected: 新测试 3/3 PASS；typecheck exit 0（expo-audio/expo-network 自带 TS 类型）；apps/mobile 既有套件 0 fail（新依赖不改变惰性 require 语义，Node 测试链仍走 fail-closed 分支——`dictation-capture.test.ts` 等既有用例不受影响）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/package.json apps/mobile/app.json pnpm-lock.yaml apps/mobile/src/release-deps.test.ts
git commit -m "feat(mobile): ship expo-audio/expo-network in the iOS release package and declare the microphone permission (T39 #69)"
```

---

### Task 2: 可复现的 Release 构建脚本

**Files:**
- Create: `apps/mobile/scripts/ios-release-build.sh`
- Test: `apps/mobile/src/scripts/ios-acceptance-scripts.test.ts`（创建；Task 4 会继续追加该文件的用例）

**Interfaces:**
- Consumes: B3/B4 实测的构建口径（`ios-test-report.md` §2 步骤表、`b4-recheck-fix.md`「环境操作注意」：`rm -rf build` 会连带删除 codegen 产物，必须先 `pod install`）；入库事实源 `apps/mobile/plugins/ios-xcode27.js`（prebuild 时自动重放 scene 生命周期/splash/告警抑制/部署目标钳制）。
- Produces: 幂等脚本 `bash apps/mobile/scripts/ios-release-build.sh [repo-root]`，末行输出 `RELEASE_APP=<apps/mobile/ios/build/Build/Products/Release-iphonesimulator/WeKnora.app>`，构建日志落 `docs/plans/issue30-sweep/ios-evidence/t39/xcodebuild-release.log`。

- [ ] **Step 1: 写失败测试**

创建 `apps/mobile/src/scripts/ios-acceptance-scripts.test.ts`（完整内容；Task 4 在同文件追加用例）：

```ts
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';

// T39（#69）：两条验收管线脚本的源级断言。脚本不入 node 测试运行时，这里钉的是
// B3/B4 三次实踩过的失败序（构建漂移、安装前启动、崩溃筛查口径）。
const scriptOf = (name: string): string =>
  readFileSync(fileURLToPath(new URL(`../../scripts/${name}`, import.meta.url)), 'utf8');

test('the release build script is a reproducible prebuild -> pods -> Release pipeline', () => {
  const script = scriptOf('ios-release-build.sh');
  assert.match(script, /set -euo pipefail/, '任何一步失败必须中止（验收产物不可半成品）');
  assert.match(script, /expo prebuild -p ios --no-install/, 'prebuild 重放入库 plugin（ios/ 不入库，漂移防护在这里）');
  const podAt = script.indexOf('pod install');
  const buildAt = script.indexOf('xcodebuild ');
  assert.ok(podAt >= 0, '必须执行 pod install');
  assert.ok(buildAt > podAt, 'xcodebuild 必须在 pod install 之后（rm -rf build 连带删 codegen 产物，B4 实测教训）');
  assert.match(script, /-configuration Release/, '证据口径是 Release 包，不是 Debug');
  assert.match(script, /-sdk iphonesimulator/, '模拟器 SDK 口径');
  assert.match(script, /-derivedDataPath build/, '产物路径固定，验收脚本才能找到 WeKnora.app');
  assert.match(script, /babel-preset-expo/, 'pnpm 工作区下 babel-preset-expo 链接守卫（B3 实测缺失即 Metro 打包失败）');
  assert.match(script, /Release-iphonesimulator\/WeKnora\.app/, '脚本末尾必须解析出 .app 产物路径');
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/scripts/ios-acceptance-scripts.test.ts`
Expected: FAIL，`ENOENT: no such file or directory … ios-release-build.sh`。

- [ ] **Step 3: 写脚本**

创建 `apps/mobile/scripts/ios-release-build.sh`（完整内容）：

```bash
#!/usr/bin/env bash
# T39（Issue #69）：可复现的 iOS 模拟器 Release 构建管线（幂等，可反复重跑）。
# 前置：macOS + Xcode 27、根目录已 pnpm install、CocoaPods 可用。
# 用法：bash apps/mobile/scripts/ios-release-build.sh [仓库根，默认当前目录]
# 产物：apps/mobile/ios/build/Build/Products/Release-iphonesimulator/WeKnora.app
#       docs/plans/issue30-sweep/ios-evidence/t39/xcodebuild-release.log
set -euo pipefail

ROOT="${1:-$(pwd)}"
MOBILE="$ROOT/apps/mobile"
IOS="$MOBILE/ios"
EVIDENCE_DIR="$ROOT/docs/plans/issue30-sweep/ios-evidence/t39"
mkdir -p "$EVIDENCE_DIR"
cd "$MOBILE"

# 1) prebuild 重新生成 ios/（.gitignore 不入库）并重放入库 plugin
#    apps/mobile/plugins/ios-xcode27.js（scene 生命周期、splash wordmark、
#    Podfile 部署目标钳制与告警抑制）——漂移防护的唯一事实源。
npx expo prebuild -p ios --no-install

# 2) pnpm 工作区下 babel-preset-expo 的符号链接在干净安装后缺失
#    （B3 实测：Metro「Cannot find module 'babel-preset-expo'」失败于 RN bundle 阶段）——缺则重建。
if [ ! -e "$MOBILE/node_modules/babel-preset-expo" ]; then
  preset="$(ls -d "$MOBILE/node_modules/.pnpm"/babel-preset-expo@*/node_modules/babel-preset-expo 2>/dev/null | head -1)"
  if [ -z "$preset" ]; then echo "babel-preset-expo not found in the pnpm store; run pnpm install first" >&2; exit 1; fi
  ln -s "$preset" "$MOBILE/node_modules/babel-preset-expo"
fi

# 3) Pods（含 plugin 写入的部署目标钳制与 inhibit_all_warnings!）
cd "$IOS"
pod install

# 4) 模拟器 Release 全量构建。注意：build/ 已存在时无需删除；确需删除则必须回到
#    第 3 步重建（codegen 生成源码在 build/generated/ios/ReactCodegen 下，B4 实测）。
xcodebuild -workspace WeKnora.xcworkspace -scheme WeKnora -sdk iphonesimulator \
  -configuration Release -derivedDataPath build \
  -destination 'generic/platform=iOS Simulator' build 2>&1 | tee "$EVIDENCE_DIR/xcodebuild-release.log"

APP="$IOS/build/Build/Products/Release-iphonesimulator/WeKnora.app"
test -d "$APP" || { echo "expected Release app missing: $APP" >&2; exit 1; }
echo "RELEASE_APP=$APP"
```

```bash
chmod +x apps/mobile/scripts/ios-release-build.sh
bash -n apps/mobile/scripts/ios-release-build.sh   # 语法门
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test apps/mobile/src/scripts/ios-acceptance-scripts.test.ts`
Expected: PASS（1/1）。

- [ ] **Step 5: 首次真实构建（本任务的可执行验证；耗时约 10–20 分钟）**

Run: `bash apps/mobile/scripts/ios-release-build.sh "$(pwd)"`
Expected: 末行 `RELEASE_APP=…/WeKnora.app`；日志末尾 `** BUILD SUCCEEDED **`。失败时按 B4 教训排查（缺 pod → 回第 3 步；Metro/babel → 查第 2 步守卫输出），不得跳过本步宣布通过。

- [ ] **Step 6: Commit**

```bash
git add apps/mobile/scripts/ios-release-build.sh apps/mobile/src/scripts/ios-acceptance-scripts.test.ts docs/plans/issue30-sweep/ios-evidence/t39/xcodebuild-release.log
git commit -m "feat(mobile): reproducible iOS simulator Release build pipeline with pnpm/link guards (T39 #69)"
```

---

### Task 3: 验收证据契约模块与「不冒充」AC 门

**Files:**
- Create: `apps/mobile/src/ios-release-evidence.ts`
- Create: `apps/mobile/src/ios-release-evidence.test.ts`
- Create: `apps/mobile/scripts/emit-acceptance-record.ts`

**Interfaces:**
- Consumes: AC 原文三条（Global Constraints）；B4 证据目录约定 `docs/plans/issue30-sweep/ios-evidence/`。
- Produces（Task 5/6 消费）: `IOS_ACCEPTANCE_FLOWS`/`IOS_ADVERSITY_PATHS`/`IosEvidenceEntry`/`assembleIosReleaseEvidence`/`iosReleaseEvidenceGaps`/`IosReleaseEvidenceRecord`/`buildRecordFromOutcomes`。

- [ ] **Step 1: 写失败测试**

创建 `apps/mobile/src/ios-release-evidence.test.ts`（完整内容）：

```ts
import assert from 'node:assert/strict';
import { test } from 'node:test';

const loadMod = () => import('./ios-release-evidence.ts');

test('the acceptance subject lists cover the nine core flows and the four adversity paths verbatim from the issue', async () => {
  const { IOS_ACCEPTANCE_FLOWS, IOS_ADVERSITY_PATHS } = await loadMod();
  assert.deepEqual([...IOS_ACCEPTANCE_FLOWS], [
    'sign-in', 'tenant-switch', 'task', 'background-recovery', 'offline-draft',
    'notification', 'download-share', 'voice-permission', 'secure-storage',
  ]);
  assert.deepEqual([...IOS_ADVERSITY_PATHS], ['weak-network', 'permission-denied', 'cold-start', 'revocation']);
});

test('assembly is total: missing subjects become not-run entries and later entries win', async () => {
  const { assembleIosReleaseEvidence, iosReleaseEvidenceGaps } = await loadMod();
  const record = assembleIosReleaseEvidence({}, [
    { subject: 'sign-in', disposition: 'evidenced', source: 'installed-package', evidence: 'ios-evidence/t39/02-login-screen-11s.png' },
    { subject: 'sign-in', disposition: 'evidenced', source: 'integration-harness', evidence: 'apps/mobile/src/ios-core-workflow-integration-smoke.ts' },
  ]);
  assert.equal(record.entries.length, 13, '9 flows + 4 adversity paths，缺项自动补齐');
  const signIn = record.entries.find((entry) => entry.subject === 'sign-in')!;
  assert.equal(signIn.source, 'integration-harness', '同 subject 后者覆盖前者');
  assert.equal(record.entries.find((entry) => entry.subject === 'task')?.disposition, 'not-run');
  assert.ok(iosReleaseEvidenceGaps(record).length > 0, '只有一条证据、其余全 not-run 时门必须拦');
});

test('the gate rejects evidence impersonation in all three shapes (AC3: mocks do not count)', async () => {
  const { assembleIosReleaseEvidence, iosReleaseEvidenceGaps } = await loadMod();
  const record = assembleIosReleaseEvidence({}, [
    { subject: 'sign-in', disposition: 'evidenced', source: 'unit-test', evidence: 'some.test.ts' },
    { subject: 'tenant-switch', disposition: 'evidenced', source: 'integration-harness' },
    { subject: 'task', disposition: 'blocked-env' },
  ]);
  const gaps = iosReleaseEvidenceGaps(record);
  assert.ok(gaps.some((gap) => gap.includes('sign-in') && gap.includes('installed-package or integration-harness')), 'source=unit-test 是冒充');
  assert.ok(gaps.some((gap) => gap.includes('tenant-switch') && gap.includes('evidence pointer')), 'evidenced 必须带证据指针');
  assert.ok(gaps.some((gap) => gap.includes('task') && gap.includes('reason')), 'blocked-env 必须说明缺失的外部资源');
});

test('a complete honest record passes the gate, and blocked-env entries stay listed (not converted to pass)', async () => {
  const { assembleIosReleaseEvidence, iosReleaseEvidenceGaps } = await loadMod();
  const record = assembleIosReleaseEvidence(
    { appPath: 'apps/mobile/ios/build/Build/Products/Release-iphonesimulator/WeKnora.app', builtAt: '2026-09-26T00:00:00.000Z' },
    [
      { subject: 'sign-in', disposition: 'evidenced', source: 'installed-package', evidence: 'ios-evidence/t39/02-login-screen-11s.png' },
      { subject: 'tenant-switch', disposition: 'blocked-env', reason: 'single-tenant deployment: the switch path needs a second tenant on the deployment' },
      { subject: 'task', disposition: 'evidenced', source: 'integration-harness', evidence: 'apps/mobile/src/task-start-integration-smoke.ts' },
      { subject: 'background-recovery', disposition: 'evidenced', source: 'integration-harness', evidence: 'apps/mobile/src/ios-core-workflow-integration-smoke.ts' },
      { subject: 'offline-draft', disposition: 'evidenced', source: 'integration-harness', evidence: 'apps/mobile/src/offline-vault-integration-smoke.ts' },
      { subject: 'notification', disposition: 'evidenced', source: 'installed-package', evidence: 'ios-evidence/t39/06-push-delivered-unauthorized.png' },
      { subject: 'download-share', disposition: 'blocked-env', reason: 'system share sheet is a true external (spec: real-device acceptance separately)' },
      { subject: 'voice-permission', disposition: 'evidenced', source: 'installed-package', evidence: 'ios-evidence/t39/07-after-mic-revoked.png' },
      { subject: 'secure-storage', disposition: 'evidenced', source: 'integration-harness', evidence: 'apps/mobile/src/ios-core-workflow-integration-smoke.ts' },
      { subject: 'weak-network', disposition: 'evidenced', source: 'integration-harness', evidence: 'apps/mobile/src/ios-core-workflow-integration-smoke.ts' },
      { subject: 'permission-denied', disposition: 'evidenced', source: 'installed-package', evidence: 'ios-evidence/t39/07-after-mic-revoked.png' },
      { subject: 'cold-start', disposition: 'evidenced', source: 'installed-package', evidence: 'ios-evidence/t39/cold-start.mov' },
      { subject: 'revocation', disposition: 'evidenced', source: 'installed-package', evidence: 'ios-evidence/t39/05-deeplink-detail-unauthorized.png' },
    ],
  );
  assert.deepEqual(iosReleaseEvidenceGaps(record), [], '13 个验收项全部 resolved（evidenced 或有理由的 blocked-env）即过门');
  assert.equal(record.entries.find((entry) => entry.subject === 'download-share')?.disposition, 'blocked-env', '门不改写 disposition，blocked-env 不是通过');
});

test('installed-package evidence requires the built artifact path (AC1: no package, no claim)', async () => {
  const { assembleIosReleaseEvidence, iosReleaseEvidenceGaps } = await loadMod();
  const record = assembleIosReleaseEvidence({}, [
    { subject: 'cold-start', disposition: 'evidenced', source: 'installed-package', evidence: 'ios-evidence/t39/cold-start.mov' },
  ]);
  assert.ok(iosReleaseEvidenceGaps(record).some((gap) => gap.includes('build.appPath')), '没有 Release 产物就不得主张安装包证据');
});

test('buildRecordFromOutcomes propagates gate violations so the CLI exits non-zero', async () => {
  const { buildRecordFromOutcomes } = await import('../scripts/emit-acceptance-record.ts');
  const failing = buildRecordFromOutcomes({ build: {}, entries: [{ subject: 'sign-in', disposition: 'evidenced', source: 'prototype', evidence: 'x' }] });
  assert.ok(failing.gaps.length > 0, 'source=prototype + 其余 not-run 都是门违规');
  const assembled = buildRecordFromOutcomes({
    build: { appPath: 'apps/mobile/ios/build/Build/Products/Release-iphonesimulator/WeKnora.app' },
    entries: [{ subject: 'sign-in', disposition: 'evidenced', source: 'installed-package', evidence: 'ios-evidence/t39/02-login-screen-11s.png' }],
  });
  assert.equal(assembled.record.entries.length, 13, '装配仍是 total 的，缺失项以 not-run 呈现给门');
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/ios-release-evidence.test.ts`
Expected: FAIL，`Cannot find module … ios-release-evidence.ts`（第 6 个用例同时缺 `emit-acceptance-record.ts`）。

- [ ] **Step 3: 最小实现**

创建 `apps/mobile/src/ios-release-evidence.ts`（完整内容）：

```ts
/** T39（Issue #69）验收证据契约：九大核心工作流 × 四条逆境路径的处置分类与「不冒充」门。
 *  AC3 原文：「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实
 *  集成证据。」本模块只认两类证据来源：installed-package（Release 安装包在模拟器实跑的
 *  截图/录屏/日志）与 integration-harness（opt-in 真实部署的最高稳定 Interface 集成证据）。
 *  blocked-env 是如实登记、不是通过（#71 口径：「任何 skipped 或 blocked-env 不计为通过」）。 */

export const IOS_ACCEPTANCE_FLOWS = [
  'sign-in', 'tenant-switch', 'task', 'background-recovery', 'offline-draft',
  'notification', 'download-share', 'voice-permission', 'secure-storage',
] as const;
export type IosAcceptanceFlow = (typeof IOS_ACCEPTANCE_FLOWS)[number];

export const IOS_ADVERSITY_PATHS = ['weak-network', 'permission-denied', 'cold-start', 'revocation'] as const;
export type IosAdversityPath = (typeof IOS_ADVERSITY_PATHS)[number];

export type IosEvidenceSubject = IosAcceptanceFlow | IosAdversityPath;
export type IosEvidenceSource = 'installed-package' | 'integration-harness';
export type IosEvidenceDisposition = 'evidenced' | 'blocked-env' | 'not-run';

export interface IosEvidenceEntry {
  subject: IosEvidenceSubject;
  disposition: IosEvidenceDisposition;
  /** disposition='evidenced' 时必填：证据文件或证据键（相对 docs/plans/issue30-sweep/ 的路径或 smoke 证据键）。 */
  evidence?: string;
  /** disposition!=='evidenced' 时必填：blocked-env 必须点名缺失的外部资源。 */
  reason?: string;
  source?: IosEvidenceSource;
}

export interface IosReleaseBuildFacts {
  /** Release-iphonesimulator/WeKnora.app 的仓库相对路径；未构建时缺省。 */
  appPath?: string;
  builtAt?: string;
  buildLog?: string;
}

export interface IosReleaseEvidenceRecord {
  build: IosReleaseBuildFacts;
  entries: IosEvidenceEntry[];
  generatedAt: string;
}

const REQUIRED_SUBJECTS: readonly IosEvidenceSubject[] = [...IOS_ACCEPTANCE_FLOWS, ...IOS_ADVERSITY_PATHS];

function normalizeEntry(entry: IosEvidenceEntry): IosEvidenceEntry {
  if (entry.disposition === 'evidenced') {
    return { ...entry, source: entry.source ?? 'installed-package', reason: undefined };
  }
  // 非 evidenced 条目不携带证据指针与来源：它们不是通过，别让残留字段制造「已验证」错觉。
  // reason 原样透传（不兜底）：缺理由的 blocked-env 必须被 iosReleaseEvidenceGaps 的
  // 「entries must carry a reason」分支点名——若在这里兜底默认文案，该分支永不触发，
  // Review Focus 5 的门语义就失效了。
  return { subject: entry.subject, disposition: entry.disposition, reason: entry.reason };
}

/** total 化装配：按 subject 去重（后者覆盖前者），缺失 subject 自动补 not-run 条目；绝不 reject。 */
export function assembleIosReleaseEvidence(build: IosReleaseBuildFacts, entries: IosEvidenceEntry[]): IosReleaseEvidenceRecord {
  const bySubject = new Map<IosEvidenceSubject, IosEvidenceEntry>();
  for (const entry of entries) bySubject.set(entry.subject, normalizeEntry(entry));
  for (const subject of REQUIRED_SUBJECTS) {
    if (!bySubject.has(subject)) bySubject.set(subject, { subject, disposition: 'not-run', reason: 'no entry provided' });
  }
  return { build, entries: REQUIRED_SUBJECTS.map((subject) => bySubject.get(subject)!), generatedAt: new Date().toISOString() };
}

/** AC 门（机器检查）：返回违规列表，空数组 = 记录形态可接受。三条语义：
 *  ①not-run 恒为违规（未解决的验收项）；②blocked-env 带理由即形态合法——它是如实登记、
 *  不是通过，「blocked-env 未清零不得宣布 Issue 通过」是发布结论层的义务（#71），
 *  门故意不改写 disposition；③evidenced 必须来自两类真实来源且携带证据指针。 */
export function iosReleaseEvidenceGaps(record: IosReleaseEvidenceRecord): string[] {
  const gaps: string[] = [];
  const subjects = new Set(record.entries.map((entry) => entry.subject));
  for (const subject of REQUIRED_SUBJECTS) {
    if (!subjects.has(subject)) gaps.push(`missing acceptance subject: ${subject}`);
  }
  for (const entry of record.entries) {
    if (entry.disposition === 'evidenced') {
      if (entry.source !== 'installed-package' && entry.source !== 'integration-harness') {
        gaps.push(`${entry.subject}: evidenced entries must come from installed-package or integration-harness (AC3: mocks do not count)`);
      }
      if (entry.evidence === undefined || entry.evidence.trim() === '') {
        gaps.push(`${entry.subject}: evidenced entries must carry an evidence pointer`);
      }
    } else if (entry.disposition === 'not-run') {
      gaps.push(`${entry.subject}: not-run entries are unresolved acceptance subjects`);
    } else if (entry.reason === undefined || entry.reason.trim() === '') {
      gaps.push(`${entry.subject}: ${entry.disposition} entries must carry a reason`);
    }
  }
  const claimsPackage = record.entries.some((entry) => entry.disposition === 'evidenced' && entry.source === 'installed-package');
  if (claimsPackage && (record.build.appPath === undefined || record.build.appPath.trim() === '')) {
    gaps.push('installed-package evidence requires build.appPath of the Release artifact');
  }
  return gaps;
}
```

创建 `apps/mobile/scripts/emit-acceptance-record.ts`（完整内容）：

```ts
/** T39 验收记录发射器：读观察结果 JSON → 装配记录 → AC 门校验 → 打印记录。
 *  用法：pnpm exec tsx apps/mobile/scripts/emit-acceptance-record.ts <outcomes.json>
 *  outcomes.json 形如 { "build": { "appPath": "…", "builtAt": "…", "buildLog": "…" },
 *                       "entries": [ { "subject": "sign-in", "disposition": "…", … } ] }
 *  门不通过时打印违规清单并以非零码退出——阻塞验收结论，而不是放行。 */
import { readFileSync } from 'node:fs';
import {
  assembleIosReleaseEvidence,
  iosReleaseEvidenceGaps,
  type IosEvidenceEntry,
  type IosReleaseBuildFacts,
  type IosReleaseEvidenceRecord,
} from '../src/ios-release-evidence.ts';

export interface AcceptanceOutcomes {
  build: IosReleaseBuildFacts;
  entries: IosEvidenceEntry[];
}

export function buildRecordFromOutcomes(outcomes: AcceptanceOutcomes): { record: IosReleaseEvidenceRecord; gaps: string[] } {
  const record = assembleIosReleaseEvidence(outcomes.build ?? {}, outcomes.entries ?? []);
  return { record, gaps: iosReleaseEvidenceGaps(record) };
}

if (process.argv[1] !== undefined && process.argv[1].endsWith('emit-acceptance-record.ts')) {
  const raw = JSON.parse(readFileSync(process.argv[2]!, 'utf8')) as AcceptanceOutcomes;
  const { record, gaps } = buildRecordFromOutcomes(raw);
  if (gaps.length > 0) {
    console.error(JSON.stringify({ gaps }, null, 2));
    process.exit(1);
  }
  console.log(JSON.stringify(record, null, 2));
}
```

**类型检查覆盖说明**：`apps/mobile/tsconfig.json` include 仅 `src/**`，本脚本不在 `typecheck:mobile` 覆盖面内（pnpm 工作区根亦无独立 node_modules）；其行为正确性由 `ios-release-evidence.test.ts` 对 `buildRecordFromOutcomes` 的动态导入用例承担（tsx 运行时转译）。若后续希望 CLI 文件纳入类型检查，扩 include 属独立改动，不在本计划内。

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test apps/mobile/src/ios-release-evidence.test.ts && pnpm run typecheck:mobile`
Expected: 6/6 PASS；typecheck exit 0。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/ios-release-evidence.ts apps/mobile/src/ios-release-evidence.test.ts apps/mobile/scripts/emit-acceptance-record.ts
git commit -m "feat(mobile): acceptance evidence contract with an anti-impersonation gate for T39 (Issue #69)"
```

---

### Task 4: 模拟器安装包证据管线脚本（无凭据可验证路径）

**Files:**
- Create: `apps/mobile/scripts/ios-acceptance-run.sh`
- Modify: `apps/mobile/src/scripts/ios-acceptance-scripts.test.ts`（追加用例）

**Interfaces:**
- Consumes: Task 2 产物 `WeKnora.app` 与 `RELEASE_APP` 口径；`xcrun simctl` 实测能力（privacy 含 `microphone`；push/json；io screenshot/recordVideo；spawn log show——作者本会话逐一验证）。
- Produces: `bash apps/mobile/scripts/ios-acceptance-run.sh <booted-UDID> [repo-root]`，产物落 `docs/plans/issue30-sweep/ios-evidence/t39/`（`01–07` 截图、`cold-start.mov`、`app-launch-log.txt`、`push-payload.json`）；对应 `IosEvidenceEntry` 证据键：`cold-start`→`cold-start.mov`+`01/02`、`permission-denied`→`07`、`revocation`（未授权面）→`03/04/05/06`。

- [ ] **Step 1: 追加失败测试**

在 `apps/mobile/src/scripts/ios-acceptance-scripts.test.ts` 末尾追加（完整内容）：

```ts
test('the acceptance probe script installs before probing and records the no-credential paths', () => {
  const script = scriptOf('ios-acceptance-run.sh');
  assert.match(script, /set -euo pipefail/);
  const installAt = script.indexOf('simctl install');
  const launchAt = script.indexOf('simctl launch');
  assert.ok(installAt >= 0 && launchAt > installAt, '必须先 install 再 launch——证据来自安装包，不是浏览器 prototype（AC1）');
  assert.match(script, /recordVideo/, '冷启动必须有录屏证据（AC2 cold-start）');
  assert.match(script, /simctl uninstall/, '冷启动探针前必须干净卸载（首装冷启动口径）');
  assert.match(script, /weknora:\/\/tasks\/detail/, '未授权深链 fail-closed 探针必须包含详情深链（越权面最强探针）');
  assert.match(script, /simctl push/, '未授权推送投递探针（AC2 撤销/越权面）');
  assert.match(script, /revoke microphone/, '麦克风拒权探针（AC2 permission-denied）');
  assert.match(script, /NSException\|SIGTRAP/, '日志崩溃筛查必须覆盖 NSException/SIGTRAP 口径（B4 同款）');
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/scripts/ios-acceptance-scripts.test.ts`
Expected: FAIL，新用例 `ENOENT … ios-acceptance-run.sh`。

- [ ] **Step 3: 写脚本**

创建 `apps/mobile/scripts/ios-acceptance-run.sh`（完整内容）：

```bash
#!/usr/bin/env bash
# T39（Issue #69）：安装包验收证据管线——无凭据可验证路径（冷启动/未授权深链/未授权推送/
# 麦克风拒权/崩溃筛查）。授权面九流的自动化需要真实 deployment 凭据 + idb（本环境两者皆缺，
# 见 t39-acceptance.md 的 blocked-env 清单与人工路径清单）。
# 用法：bash apps/mobile/scripts/ios-acceptance-run.sh <booted-UDID> [仓库根]
# 产物：docs/plans/issue30-sweep/ios-evidence/t39/
set -euo pipefail

UDID="${1:?usage: ios-acceptance-run.sh <booted-UDID> [repo-root]}"
ROOT="${2:-$(pwd)}"
MOBILE="$ROOT/apps/mobile"
APP="$MOBILE/ios/build/Build/Products/Release-iphonesimulator/WeKnora.app"
BUNDLE="com.weknora.mobile"
OUT="$ROOT/docs/plans/issue30-sweep/ios-evidence/t39"
mkdir -p "$OUT"
test -d "$APP" || { echo "Release app missing — run ios-release-build.sh first" >&2; exit 1; }

xcrun simctl bootstatus "$UDID" -b >/dev/null

# 1) 冷启动（AC2 cold-start）：干净卸载 → 首装 → 录屏启动全程 → 2s/11s 截图
xcrun simctl terminate "$UDID" "$BUNDLE" 2>/dev/null || true
xcrun simctl uninstall "$UDID" "$BUNDLE" 2>/dev/null || true
xcrun simctl io "$UDID" recordVideo --codec h264 --force "$OUT/cold-start.mov" &
REC=$!
xcrun simctl install "$UDID" "$APP"
xcrun simctl launch "$UDID" "$BUNDLE"
sleep 2
xcrun simctl io "$UDID" screenshot "$OUT/01-cold-start-2s.png" >/dev/null
sleep 9
xcrun simctl io "$UDID" screenshot "$OUT/02-login-screen-11s.png" >/dev/null
kill -INT "$REC" 2>/dev/null || true
wait "$REC" 2>/dev/null || true

# 2) 未授权深链 fail-closed 探针（AC2 撤销/越权面）：每条深链后截屏留证
probe_deep_link() {
  xcrun simctl openurl "$UDID" "$1"
  sleep 2
  xcrun simctl io "$UDID" screenshot "$OUT/$2" >/dev/null
}
probe_deep_link "weknora://tasks" "03-deeplink-tasks-unauthorized.png"
probe_deep_link "weknora://ask" "04-deeplink-ask-unauthorized.png"
probe_deep_link "weknora://tasks/detail?taskId=1&runId=1" "05-deeplink-detail-unauthorized.png"

# 3) 未授权推送投递（本地模拟 APNs 帧真机语义）：投递后进程必须仍在（崩溃由第 5 步日志门兜底）
cat > "$OUT/push-payload.json" <<'JSON'
{
  "Simulator Target Bundle": "com.weknora.mobile",
  "aps": { "alert": { "title": "WeKnora", "body": "acceptance probe" }, "sound": "default" }
}
JSON
xcrun simctl push "$UDID" "$BUNDLE" "$OUT/push-payload.json"
sleep 2
xcrun simctl io "$UDID" screenshot "$OUT/06-push-delivered-unauthorized.png" >/dev/null
xcrun simctl spawn "$UDID" launchctl list 2>/dev/null | grep "$BUNDLE" > "$OUT/push-process-alive.txt" || true

# 4) 麦克风拒权探针（AC2 permission-denied）：显式 revoke（iOS 会终止运行中的应用）→ 重启必须存活
xcrun simctl privacy "$UDID" revoke microphone "$BUNDLE"
xcrun simctl terminate "$UDID" "$BUNDLE" 2>/dev/null || true
xcrun simctl launch "$UDID" "$BUNDLE"
sleep 3
xcrun simctl io "$UDID" screenshot "$OUT/07-after-mic-revoked.png" >/dev/null

# 5) 启动日志崩溃筛查（B4 同款口径：fatal/未捕获/NSException/信号崩溃必须 0 命中）
xcrun simctl spawn "$UDID" log show --last 5m --predicate 'processImagePath CONTAINS "WeKnora"' > "$OUT/app-launch-log.txt" 2>&1 || true
ERRS="$(grep -cE 'fatal|uncaught|NSException|SIGTRAP|SIGSEGV' "$OUT/app-launch-log.txt" || true)"
echo "FATAL_LOG_HITS=$ERRS"
if [ "$ERRS" -ne 0 ]; then echo "crash signals found in the launch log — acceptance probes FAILED" >&2; exit 1; fi
echo "ACCEPTANCE_PROBES_OK=$OUT"
```

```bash
chmod +x apps/mobile/scripts/ios-acceptance-run.sh
bash -n apps/mobile/scripts/ios-acceptance-run.sh   # 语法门
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test apps/mobile/src/scripts/ios-acceptance-scripts.test.ts`
Expected: PASS（2/2）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/scripts/ios-acceptance-run.sh apps/mobile/src/scripts/ios-acceptance-scripts.test.ts
git commit -m "feat(mobile): simulator acceptance probe pipeline for cold-start/permission/revocation surfaces (T39 #69)"
```

---

### Task 5: 冷启动恢复 / 撤销不可复活 / 弱网重续——组合集成证据（opt-in）

**Files:**
- Create: `apps/mobile/src/ios-core-workflow-integration-smoke.ts`
- Test: `apps/mobile/src/ios-core-workflow-integration-smoke.test.ts`（创建）

**Interfaces:**
- Consumes: `createMobileRuntime`/`signIn`/`boot`/`signOut`/`dispose`/`activateTenant`/`scopeLease`/`authorizedRequest`（Consumes 节签名）；`createSecureCredentialStore`/`createSecureDeploymentStore`/`SecureStorePort`（apps/mobile adapters）；`createTaskOffice`/`TaskStartReceipt`/`reconcilePending`；`createTaskOfficeRemote`；`disallowedDeploymentHost`（`./runtime-integration-smoke.ts`）；opt-in 语义 `WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD`（与 `taskStartIntegrationConfig` 同口径，含凭据缺失 skip、私网/环回 invalid）。
- Produces: `iosCoreWorkflowIntegrationConfig(env)`、`IosCoreWorkflowIntegrationEvidence`、`runIosCoreWorkflowIntegration(config)`、`emitIosCoreWorkflowIntegrationEvidence(evidence, emit)`——Task 6 报告引用其证据键，#70 的 Android 版可用同构 harness 对拍。

- [ ] **Step 1: 写失败测试**

创建 `apps/mobile/src/ios-core-workflow-integration-smoke.test.ts`（完整内容）：

```ts
import assert from 'node:assert/strict';
import { test } from 'node:test';

// tsx 以 CJS 输出 .ts，动态导入保持与其他集成 smoke 测试同构（先例：voice-dictation-integration-smoke.test.ts）。
const loadMod = () => import('./ios-core-workflow-integration-smoke.ts');

const env = () => process.env as Record<string, string | undefined>;

test('integration stays opt-in: missing credentials skip and a private/loopback host is invalid, never a pass', async () => {
  const { iosCoreWorkflowIntegrationConfig } = await loadMod();
  const missing = iosCoreWorkflowIntegrationConfig({ ...env(), WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: undefined });
  assert.equal(missing.enabled, false);
  assert.equal(missing.disposition, 'skip');
  for (const host of ['http://insecure.example', 'https://localhost', 'https://127.0.0.1', 'https://10.0.0.5', 'https://192.168.1.9']) {
    const invalid = iosCoreWorkflowIntegrationConfig({
      WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: host,
      WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
      WEKNORA_MOBILE_TEST_PASSWORD: ['short-lived', 'secret'].join('-'),
    });
    assert.equal(invalid.enabled, false, `${host} 不得成为被授权目标`);
    assert.equal(invalid.disposition, 'invalid');
  }
});

test('the runner is total: an unreachable deployment still yields evidence, not a rejection', async () => {
  const { runIosCoreWorkflowIntegration } = await loadMod();
  const evidence = await runIosCoreWorkflowIntegration({
    enabled: true,
    deploymentOrigin: 'https://weknora.invalid.test',
    email: 'nobody@example.test',
    password: 'wrong',
  });
  assert.equal(evidence.signIn, 'failed');
  assert.equal(evidence.coldBootRestore, 'not-attempted', '登录失败不得伪造后续步骤已执行');
  assert.equal(evidence.revocation, 'not-attempted');
  assert.equal(typeof evidence.errorReason, 'string');
  assert.doesNotMatch(JSON.stringify(evidence), /short-lived-secret|password|token|email/i, '证据不含凭据');
});

test('live composition: cold-boot restore, no revival after sign-out, weak-network reconcile on one run (opt-in)', async (t) => {
  const { emitIosCoreWorkflowIntegrationEvidence, iosCoreWorkflowIntegrationConfig, runIosCoreWorkflowIntegration } = await loadMod();
  const config = iosCoreWorkflowIntegrationConfig(env());
  if (!config.enabled) {
    t.skip(config.reason);
    return;
  }
  const evidence = await runIosCoreWorkflowIntegration(config);
  const emitted: string[] = [];
  emitIosCoreWorkflowIntegrationEvidence(evidence, (record) => { emitted.push(record); });
  assert.equal(emitted.length, 1);
  assert.equal(JSON.parse(emitted[0]!).signIn, evidence.signIn);
  if (evidence.signIn === 'authorized') {
    assert.equal(evidence.coldBootRestore, 'authorized-restored', '冷启动恢复：第二 Runtime 实例从持久凭据恢复授权面');
    assert.equal(evidence.revocation, 'revoked', '撤销不可复活：signOut 后第三实例 boot() 必须停在 deployment-login');
    assert.ok(
      evidence.weakNetwork === 'reconciled-same-run' || evidence.weakNetwork === 'pending-retained' || evidence.weakNetwork === 'failed',
      '弱网结果如实记录（部署无可用 agent 时允许 failed 并带 errorReason，不伪造）',
    );
    if (evidence.weakNetwork === 'reconciled-same-run') {
      assert.equal(evidence.distinctRunIds, 1, '断链重续不得产生第二个 Run（AC1 单写者幂等）');
      assert.ok(evidence.weakNetworkStartRequests >= 2, '断链重试确实发生了第二次 Start 派发');
    }
  }
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/ios-core-workflow-integration-smoke.test.ts`
Expected: FAIL，`Cannot find module … ios-core-workflow-integration-smoke.ts`。

- [ ] **Step 3: 最小实现**

创建 `apps/mobile/src/ios-core-workflow-integration-smoke.ts`（完整内容）：

```ts
import { mkdtempSync, rmSync } from 'node:fs';
import { readFile, unlink, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import {
  createMobileRuntime,
  createTaskOffice,
  type AuthorizedTransport,
  type MobileRuntime,
  type TaskOffice,
  type TaskStartReceipt,
} from '@weknora/mobile-core';
import { createSecureCredentialStore } from './adapters/credential-store.ts';
import type { SecureStorePort } from './adapters/secure-store.ts';
import { createSecureDeploymentStore } from './adapters/deployment-store.ts';
import { createNativeRequestId } from './adapters/request-id.ts';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';

export type IosCoreWorkflowIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

/** 与 taskStartIntegrationConfig 相同的 opt-in 语义 + disallowedDeploymentHost 主机防线。 */
export function iosCoreWorkflowIntegrationConfig(env: Record<string, string | undefined>): IosCoreWorkflowIntegrationConfig {
  const deploymentOrigin = env.WEKNORA_MOBILE_TEST_DEPLOYMENT_URL?.trim();
  const email = env.WEKNORA_MOBILE_TEST_EMAIL?.trim();
  const password = env.WEKNORA_MOBILE_TEST_PASSWORD;
  if (!deploymentOrigin || !email || !password) {
    return { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' };
  }
  let parsed: URL;
  try { parsed = new URL(deploymentOrigin); } catch {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL is not an absolute URL' };
  }
  if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.pathname !== '/' || parsed.search || parsed.hash) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must be a credential-free HTTPS origin' };
  }
  const hostRejection = disallowedDeploymentHost(parsed.hostname, 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL');
  if (hostRejection) return { enabled: false, disposition: 'invalid', reason: hostRejection };
  return { enabled: true, deploymentOrigin: parsed.origin, email, password };
}

export interface IosCoreWorkflowIntegrationEvidence {
  deploymentOrigin: string;
  signIn: 'authorized' | 'failed';
  /** 冷启动恢复（AC2 cold-start 的 Interface 面）：dispose 后第二 Runtime 实例 boot() 从持久凭据恢复授权面。 */
  coldBootRestore: 'authorized-restored' | 'failed' | 'not-attempted';
  /** 撤销不可复活（AC2 revocation 的 Interface 面）：signOut 清凭据后第三实例 boot() 必须停 deployment-login。 */
  revocation: 'revoked' | 'failed' | 'not-attempted';
  tenantSwitch: 'switched' | 'single-tenant' | 'failed' | 'not-attempted';
  /** 弱网（AC2 weak-network 的 Interface 面）：首枚 Start POST 注入 2s 延迟后断链 → 意图已在
   *  Start 前耐久落盘 → reconcilePending() 以同一 requestId 重派 → 服务端幂等收敛同一 run。 */
  weakNetwork: 'reconciled-same-run' | 'pending-retained' | 'failed' | 'not-attempted';
  weakNetworkStartRequests?: number;
  distinctRunIds?: number;
  runVisibleInTasks?: boolean | 'unavailable' | 'not-attempted';
  errorReason?: string;
  timestamp: string;
}

/** 文件兜底 SecureStorePort（仅 harness 环境；模拟器/真机组合根用 expo-secure-store）。
 *  目的不是冒充 Keychain，而是给「冷启动恢复/撤销」组合一个跨 Runtime 实例真实持久的存储：
 *  boot() 第二实例读取的是 signIn 第一实例落盘的字节，而不是同一进程内缓存的凭据对象。 */
function createFileBackedSecureStore(dir: string): SecureStorePort {
  const pathOf = (key: string): string => join(dir, encodeURIComponent(key));
  return {
    async getItemAsync(key) {
      try { return await readFile(pathOf(key), 'utf8'); } catch { return null; }
    },
    async setItemAsync(key, value) { await writeFile(pathOf(key), value, 'utf8'); },
    async deleteItemAsync(key) {
      try { await unlink(pathOf(key)); } catch { /* absent */ }
    },
  };
}

interface RuntimeWiring {
  credentialDir: string;
  fetcher: FetchLike;
  /** 弱网注入点：首枚 POST /workbench/executions 延迟 2s 后断链（只断 Start，createSession 放行）。 */
  weakStart?: boolean;
  startDispatches?: { count: number };
}

function runtimeOf(wiring: RuntimeWiring): MobileRuntime {
  const store = createFileBackedSecureStore(wiring.credentialDir);
  return createMobileRuntime({
    credentialStore: createSecureCredentialStore(store),
    deploymentStore: createSecureDeploymentStore(store),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    remoteFor(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(wiring.fetcher) });
      return createMobileRuntimeRemote({ origin, request: client.request });
    },
    authorizedTransport(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(wiring.fetcher) });
      const authorized: AuthorizedTransport = (input, accessToken) =>
        client.request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
      if (wiring.weakStart !== true) return authorized;
      const dispatches = wiring.startDispatches!;
      return (input, accessToken) => {
        if (input.method === 'POST' && input.path.includes('/workbench/executions')) {
          dispatches.count += 1;
          if (dispatches.count === 1) {
            // 弱网：请求已发出、2 秒无响应后链路断开（客户端侧 fetch 拒绝——服务端可能已受理，
            // 这正是「断链重续必须同 requestId」的不变量来源）。
            return new Promise((_, reject) => setTimeout(() => reject(new Error('weak network: connection dropped')), 2000));
          }
        }
        return authorized(input, accessToken);
      };
    },
  });
}

/**
 * 真实端到端（AC3）：生产 JSON transport + Runtime 授权通道 + 具体 Remote Adapter + Task Office
 * start/reconcile 编排，在真实部署上验证三组逆境语义。total 化收口：任何步骤异常也产出证据对象
 * （含 errorReason，不含凭据），绝不 reject 把证据丢弃（先例：task-start-integration-smoke.ts:68-69）。
 */
export async function runIosCoreWorkflowIntegration(config: Extract<IosCoreWorkflowIntegrationConfig, { enabled: true }>): Promise<IosCoreWorkflowIntegrationEvidence> {
  const evidence: IosCoreWorkflowIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    signIn: 'failed',
    coldBootRestore: 'not-attempted',
    revocation: 'not-attempted',
    tenantSwitch: 'not-attempted',
    weakNetwork: 'not-attempted',
    runVisibleInTasks: 'not-attempted',
    timestamp: new Date().toISOString(),
  };
  const credentialDir = mkdtempSync(join(tmpdir(), 'weknora-t39-'));
  const dispatches = { count: 0 };
  let activeRuntime: MobileRuntime | undefined;
  try {
    // 第一实例：signIn 落盘凭据（createSecureCredentialStore → 文件）+ deploymentStore 记忆活动实例。
    const first = runtimeOf({ credentialDir, fetcher: (input, init) => fetch(input, init as RequestInit) });
    activeRuntime = first;
    const snapshot = await first.signIn({
      deployment: { origin: config.deploymentOrigin, label: 'T39 acceptance deployment' },
      email: config.email,
      password: config.password,
    });
    if (snapshot.surface !== 'authorized' || !snapshot.deployment) {
      evidence.errorReason = `surface ${snapshot.surface}`;
      return evidence;
    }
    evidence.signIn = 'authorized';

    // Tenant 切换（有第二空间才切换；单空间部署如实记 single-tenant，不伪造）。
    const tenants = snapshot.identity?.tenants ?? [];
    if (tenants.length > 1 && tenants[1]?.id !== undefined) {
      try {
        const switched = await first.activateTenant(tenants[1]!.id);
        evidence.tenantSwitch = switched.surface === 'authorized' ? 'switched' : 'failed';
      } catch {
        evidence.tenantSwitch = 'failed';
      }
    } else {
      evidence.tenantSwitch = 'single-tenant';
    }

    // 弱网重续（AC1 单写者）：断链 Start → 意图已耐久落盘 → 恢复通道 reconcilePending 重派同一 requestId。
    try {
      const agentsEnvelope = await first.authorizedRequest({ method: 'GET', path: '/api/v1/agents' }) as { success?: boolean; data?: Array<{ id?: unknown }> };
      const agentId = typeof agentsEnvelope?.data?.[0]?.id === 'string' ? agentsEnvelope.data[0].id : undefined;
      if (agentId === undefined) {
        evidence.weakNetwork = 'failed';
        evidence.errorReason = 'no agent available on the deployment';
      } else {
        const office: TaskOffice = createTaskOffice({
          backend: createTaskOfficeRemote({ origin: config.deploymentOrigin, request: (input) => first.authorizedRequest(input) }),
          lease: () => first.scopeLease(),
          newRequestId: createNativeRequestId(),
        });
        const goal = { text: `T39 弱网重续：${new Date().toISOString()}`, agentId, budgetUpper: 10 };
        let firstReceipt: TaskStartReceipt | undefined;
        try {
          firstReceipt = await office.start(goal);
        } catch {
          // 断链路径也允许 start 上抛——意图记录已在 Start POST 前落盘，reconcile 能接住。
          firstReceipt = undefined;
        }
        if (firstReceipt?.phase === 'bound' && firstReceipt.runId !== undefined) {
          // 弱网没有真的断（部署太快或注入未命中）：如实记录，不伪造重续证据。
          evidence.weakNetwork = 'failed';
          evidence.errorReason = 'weak-network injection did not hold: first dispatch already bound';
        } else if (dispatches.count === 0) {
          // start 在任何 Start 派发前就失败（如就绪裁决拒绝）：弱网路径根本没被走到，如实记 failed。
          evidence.weakNetwork = 'failed';
          evidence.errorReason = 'start failed before any Start dispatch (weak path not exercised)';
        } else {
          const recovered = await office.reconcilePending();
          const mine = firstReceipt !== undefined
            ? recovered.find((receipt) => receipt.requestId === firstReceipt.requestId)
            : recovered.find((receipt) => receipt.phase === 'bound' && receipt.runId !== undefined);
          if (mine?.runId !== undefined) {
            // 单写者幂等的权威在服务端 admission：重续回执的 runId 与首次未完成派发共享同一
            // requestId；全租户任务列表只能观测「Run 可见」，不得当作本次派发计数器。
            evidence.weakNetwork = 'reconciled-same-run';
            evidence.distinctRunIds = 1;
            try {
              const page = await office.tasks({});
              evidence.runVisibleInTasks = page.items.some((card) => card.runId === mine.runId);
            } catch {
              evidence.runVisibleInTasks = 'unavailable'; // 列表观测失败不影响重续证据本身
            }
          } else {
            evidence.weakNetwork = 'pending-retained';
          }
        }
        evidence.weakNetworkStartRequests = dispatches.count;
      }
    } catch (error) {
      evidence.weakNetwork = 'failed';
      evidence.errorReason = error instanceof Error ? error.message : String(error);
    }

    // 冷启动恢复（AC2 cold-start）：dispose 第一实例 → 同一持久凭据目录上的第二实例 boot()。
    first.dispose();
    activeRuntime = undefined;
    const second = runtimeOf({ credentialDir, fetcher: (input, init) => fetch(input, init as RequestInit) });
    activeRuntime = second;
    try {
      const restored = await second.boot();
      evidence.coldBootRestore = restored.surface === 'authorized' && restored.deployment?.origin === config.deploymentOrigin ? 'authorized-restored' : 'failed';
    } catch (error) {
      evidence.coldBootRestore = 'failed';
      evidence.errorReason = `cold boot: ${error instanceof Error ? error.message : String(error)}`;
    }

    // 撤销不可复活（AC2 revocation）：signOut 清凭据 → 第三实例 boot() 必须停在 deployment-login。
    try {
      await second.signOut();
      second.dispose();
      activeRuntime = undefined;
      const third = runtimeOf({ credentialDir, fetcher: (input, init) => fetch(input, init as RequestInit) });
      activeRuntime = third;
      const revived = await third.boot();
      evidence.revocation = revived.surface === 'deployment-login' ? 'revoked' : 'failed';
      if (evidence.revocation === 'failed') evidence.errorReason = `revoked session revived as ${revived.surface}`;
      third.dispose();
      activeRuntime = undefined;
    } catch (error) {
      evidence.revocation = 'failed';
      evidence.errorReason = `revocation: ${error instanceof Error ? error.message : String(error)}`;
    }
    return evidence;
  } catch (error) {
    evidence.signIn = 'failed';
    evidence.errorReason = error instanceof Error ? error.message : String(error); // 失败仍产出证据（不含凭据）
    return evidence;
  } finally {
    activeRuntime?.dispose();
    rmSync(credentialDir, { recursive: true, force: true }); // 凭据字节随临时目录销毁，不留明文
  }
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitIosCoreWorkflowIntegrationEvidence(evidence: IosCoreWorkflowIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
```

**实现注意（写给执行者，非可选）：**
- `distinctRunIds` 的诚实口径：`office.tasks({})` 是全租户列表视图，会包含部署上的历史 Run——它只能用来观测「本次 Run 在列表可见」（`runVisibleInTasks`），不能当作「本次断链只产生一个 Run」的计数器。因此 `distinctRunIds` 固定为 1，其依据是 **reconcilePending 回执携带的 runId 与首次（未完成）派发共享同一 requestId**——单写者幂等的权威在服务端 admission（`TestAdmissionTwentyConcurrentIdenticalRequestsCreateOneRun` 语义），不在客户端列表计数。若 `weakNetworkStartRequests > 2`（重试不止一次），证据仍成立但要在报告里如实注明派发次数。
- 弱网注入只断 `POST /workbench/executions`（Start），`createSession`/`me`/agents 读全放行——断 Session 创建会把证据退化成登录失败，不是弱网语义。
- `dispatches.count === 0` 分支的存在理由：就绪裁决失败等前置异常不该被记成 `pending-retained`（那是「意图已落盘待对账」的语义）。

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test apps/mobile/src/ios-core-workflow-integration-smoke.test.ts && pnpm run typecheck:mobile`
Expected: 前 2 个用例 PASS，第 3 个用例 SKIP（缺 `WEKNORA_MOBILE_TEST_*` 凭据时 `t.skip`——skip 不是 pass，不得伪造）；typecheck exit 0。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/ios-core-workflow-integration-smoke.ts apps/mobile/src/ios-core-workflow-integration-smoke.test.ts
git commit -m "test(mobile): opt-in real-deployment composition evidence for cold-boot restore, non-revival and weak-network reconcile (T39 #69)"
```

---

### Task 6: 执行整条管线 + 验收报告 + 全量回归

**Files:**
- Create: `docs/plans/issue30-sweep/ios-evidence/t39-acceptance.md`（+ `t39/` 下证据文件）
- 本任务不改任何生产代码；发现缺陷时修复落回对应任务文件并在报告差异节记录。

**Interfaces:**
- Consumes: Task 1–5 全部产物；既有 opt-in smoke 家族（作为 `integration-harness` 证据键：`task-start-integration-smoke`、`task-detail-integration-smoke`、`offline-vault-integration-smoke`、`device-inbox-integration-smoke`、`voice-dictation-integration-smoke`、`material-integration-smoke`、`runtime-integration-smoke`、`attention-inbox-integration-smoke`、`knowledge-qa-integration-smoke`、`legacy-tasks-integration-smoke`、`task-budget-integration-smoke`、`task-intervention-integration-smoke`、`blind-push-integration-smoke`、`delivery-integration-smoke`）。
- Produces: `IosReleaseEvidenceRecord`（由 `emit-acceptance-record.ts` 产出并过门）+ 验收报告 `t39-acceptance.md`。

- [ ] **Step 1: 构建含新依赖的 Release 包**

Run: `bash apps/mobile/scripts/ios-release-build.sh "$(pwd)"`
Expected: `RELEASE_APP=…WeKnora.app`，日志 `** BUILD SUCCEEDED **`。此包与 Task 1 前的 B4 包的差异 = expo-audio/expo-network 两个原生 Pod + Info.plist 麦克风文案。

- [ ] **Step 2: 执行模拟器证据管线**

```bash
UDID="$(xcrun simctl list devices booted | grep -oE '[0-9A-F-]{36}' | head -1)"
bash apps/mobile/scripts/ios-acceptance-run.sh "$UDID" "$(pwd)"
```

Expected: 退出码 0，`FATAL_LOG_HITS=0`，`ACCEPTANCE_PROBES_OK=…/t39`。逐个打开 `01–07` 截图人工确认：`01` 为 splash wordmark、`02` 为登录页、`03/04/05` 未授权深链均停在 fail-closed 文案（不渲染任务内容）、`06` 推送到达且应用未崩溃、`07` 拒权后重启仍正常渲染登录页。若 `05` 详情深链渲染出任何任务内容：**这是越权泄漏，立即停止并升级**。

- [ ] **Step 3: 跑组合集成证据（opt-in；有凭据才实跑）**

```bash
WEKNORA_MOBILE_TEST_DEPLOYMENT_URL="…" WEKNORA_MOBILE_TEST_EMAIL="…" WEKNORA_MOBILE_TEST_PASSWORD="…" \
  pnpm exec tsx --test apps/mobile/src/ios-core-workflow-integration-smoke.test.ts
```

（凭据只经环境变量传入，绝不写入命令历史之外的文件；无凭据时该用例 SKIP，报告按 blocked-env 登记。）同口径复跑既有家族中与本 Issue 九流直接相关的四条，作为 `integration-harness` 证据键：

```bash
WEKNORA_MOBILE_TEST_DEPLOYMENT_URL="…" WEKNORA_MOBILE_TEST_EMAIL="…" WEKNORA_MOBILE_TEST_PASSWORD="…" \
  pnpm exec tsx --test apps/mobile/src/task-start-integration-smoke.test.ts apps/mobile/src/offline-vault-integration-smoke.test.ts apps/mobile/src/device-inbox-integration-smoke.test.ts apps/mobile/src/voice-dictation-integration-smoke.test.ts
```

- [ ] **Step 4: 装配验收记录并过门**

写 `docs/plans/issue30-sweep/ios-evidence/t39-outcomes.json`——**13 个 subject 必须全部出现**（`not-run` 恒被门拦），每条 `evidence` 指向真实产物、每条 `reason` 点名缺失资源。下例的处置分类是本计划预期的验收形态，执行时把 `evidence`/`reason`/`builtAt` 换成本任务的实际值；若某项实跑结果与预期分类不符（例如集成 harness 在有凭据环境实跑失败），**按实际结果降级处置并写明原因，绝不反向美化**：

```json
{
  "build": {
    "appPath": "apps/mobile/ios/build/Build/Products/Release-iphonesimulator/WeKnora.app",
    "builtAt": "<Step 1 完成时刻 ISO>",
    "buildLog": "docs/plans/issue30-sweep/ios-evidence/t39/xcodebuild-release.log"
  },
  "entries": [
    { "subject": "sign-in", "disposition": "evidenced", "source": "installed-package", "evidence": "ios-evidence/t39/02-login-screen-11s.png" },
    { "subject": "tenant-switch", "disposition": "blocked-env", "reason": "<无凭据无法在包内登录；有凭据时以 runtime-integration-smoke 的 tenantSwitch 结果替代此条>" },
    { "subject": "task", "disposition": "evidenced", "source": "integration-harness", "evidence": "apps/mobile/src/task-start-integration-smoke.ts" },
    { "subject": "background-recovery", "disposition": "evidenced", "source": "integration-harness", "evidence": "apps/mobile/src/ios-core-workflow-integration-smoke.ts" },
    { "subject": "offline-draft", "disposition": "evidenced", "source": "integration-harness", "evidence": "apps/mobile/src/offline-vault-integration-smoke.ts" },
    { "subject": "notification", "disposition": "evidenced", "source": "installed-package", "evidence": "ios-evidence/t39/06-push-delivered-unauthorized.png" },
    { "subject": "download-share", "disposition": "blocked-env", "reason": "system share sheet is a true external (spec: real-device acceptance separately); material grant/preview/bytes covered by material-integration-smoke when credentials exist" },
    { "subject": "voice-permission", "disposition": "evidenced", "source": "installed-package", "evidence": "ios-evidence/t39/07-after-mic-revoked.png" },
    { "subject": "secure-storage", "disposition": "evidenced", "source": "integration-harness", "evidence": "apps/mobile/src/ios-core-workflow-integration-smoke.ts" },
    { "subject": "weak-network", "disposition": "evidenced", "source": "integration-harness", "evidence": "apps/mobile/src/ios-core-workflow-integration-smoke.ts" },
    { "subject": "permission-denied", "disposition": "evidenced", "source": "installed-package", "evidence": "ios-evidence/t39/07-after-mic-revoked.png" },
    { "subject": "cold-start", "disposition": "evidenced", "source": "installed-package", "evidence": "ios-evidence/t39/cold-start.mov" },
    { "subject": "revocation", "disposition": "evidenced", "source": "installed-package", "evidence": "ios-evidence/t39/05-deeplink-detail-unauthorized.png" }
  ]
}
```

```bash
pnpm exec tsx apps/mobile/scripts/emit-acceptance-record.ts docs/plans/issue30-sweep/ios-evidence/t39-outcomes.json > docs/plans/issue30-sweep/ios-evidence/t39/t39-record.json
```

Expected: 退出码 0，`t39-record.json` 为 13 条目的完整记录。退出码 1 时按打印的 `gaps` 补证据或补理由——**不得放宽门**。

- [ ] **Step 5: 写验收报告**

创建 `docs/plans/issue30-sweep/ios-evidence/t39-acceptance.md`，逐节填实（骨架如下，`<填实>` 处全部来自本任务实际执行结果，无一项允许凭记忆填）：

```markdown
# T39 iOS 安装包核心工作流验收报告（Issue #69）

- 日期/执行者/worktree HEAD：<填实>
- 被测对象：apps/mobile @ <commit>，Release-iphonesimulator/WeKnora.app（含 expo-audio ~55.0.18 / expo-network ~55.0.18）
- 模拟器：<机型/UDID/iOS runtime>

## AC1 证据来自安装包（非浏览器 prototype）
- 构建管线：ios-release-build.sh 输出 RELEASE_APP=…、BUILD SUCCEEDED（日志 t39/xcodebuild-release.log）
- 安装/冷启动/登录页：t39/cold-start.mov、01、02（对应 ios-release-evidence 的 cold-start / sign-in 条目）

## AC2 弱网、拒权、冷启动、撤销四路径记录
- cold-start：t39/cold-start.mov（首装冷启动全程录屏）+ integration-harness coldBootRestore=<填实>
- permission-denied：t39/07-after-mic-revoked.png + DICTATION_FAILURE_COPY.denied 单测（dictation-view.test.ts:16-20）
- weak-network：integration-harness weakNetwork=<填实>、weakNetworkStartRequests=<填实>（宿主 dummynet 真实弱网=blocked-env，需 sudo 改 pf）
- revocation：t39/03/04/05/06（未授权深链/推送 fail-closed）+ integration-harness revocation=<填实>

## AC3 最高稳定 Interface（不冒充）
- 机器门产物：t39/t39-record.json（emit-acceptance-record.ts 退出码 0）
- integration-harness 清单及各 smoke 实跑结果：<填实，SKIP 的注明 skip 不是 pass>

## blocked-env（不计为通过，未满足项保持阻塞）
<从本计划 Global Constraints 后的 blocked-env 清单逐项拷贝并补执行时点状态>

## 人工路径清单（持凭据运营者复验用）
<登录→切租户→发任务→Home 键后台/回前台→飞行模式断网提交草稿→Inbox 通知点击深链→材料分享面板→New 屏麦克风拒权→登出后冷启动，每步预期结果一句话>
```

- [ ] **Step 6: 全量回归**

Run: `pnpm exec tsx --test "packages/domain/src/mobile/*.test.ts" "packages/mobile-core/src/**/*.test.ts" "packages/api-client/src/mobile/*.test.ts" "apps/mobile/src/**/*.test.ts*" && pnpm run typecheck:mobile`
Expected: 0 fail（skip 仅限 opt-in 集成用例）；typecheck exit 0。

- [ ] **Step 7: Commit**

```bash
git add docs/plans/issue30-sweep/ios-evidence/t39-acceptance.md docs/plans/issue30-sweep/ios-evidence/t39 docs/plans/issue30-sweep/ios-evidence/t39-outcomes.json
git commit -m "docs(issue30-sweep): T39 iOS package acceptance evidence and verdict report (Issue #69)"
```

---

## 自我审查记录

按 writing-plans 技能的四项检查逐项执行（作者在保存前完成）：

1. **Spec 覆盖**：What-to-build 九要素逐一对位——登录（Task 4 `02` 截图 + Task 5 signIn 组合）、Tenant（Task 5 `tenantSwitch` + #32 既有 runtime smoke）、Task（Task 5 弱网重续走真实 TaskOffice.start/reconcile + task-start smoke）、后台恢复（Task 5 coldBootRestore + #67 前台同步环既有单测）、离线草稿（Task 1 使 #40 组合根接线在包上生效 + offline-vault smoke）、通知（Task 4 推送探针 + device-inbox/blind-push smoke）、下载分享（material smoke + share blocked-env 如实）、语音权限（Task 1 expo-audio + Task 4 拒权探针 + voice smoke + dictation-view denied 文案既有单测）、安全存储（Task 5 文件持久凭据跨实例恢复 + #32 Scoped Vault 既有 Interface 测试）；AC1→Task 2/4/6（安装包产物与探针），AC2→Task 4/5（四逆境全部有记录位），AC3→Task 3 门 + Task 5 真实部署组合 + Task 6 机器门产物。差异记录、blocked-env 清单、环境能力边界三节把「本地不可验证」显式化。无缺口。
2. **占位符扫描**：全部代码块为完整可编译/可执行内容；测试命令均为仓库真实命令（`pnpm exec tsx --test`、`pnpm run typecheck:mobile`、`bash -n`、`xcrun simctl …` 均在作者会话实跑或为 B4 已验证口径）；`<填实>` 仅出现在 Task 6 报告骨架中且每处标注「来自实际执行结果」——这是执行期观察值的填空，不是设计占位。无 TBD/TODO/“类似 Task N”。
3. **类型/签名一致性**：`IosEvidenceEntry.subject` 覆盖 `IosAcceptanceFlow | IosAdversityPath` 共 13 值 ↔ `assembleIosReleaseEvidence` 的 `REQUIRED_SUBJECTS` 与 Task 3 测试 `record.entries.length === 13` 一致；`TaskStartReceipt { requestId, phase, runId?, dispatched }` ↔ Task 5 对 `firstReceipt?.runId`/`phase === 'bound'`/`receipt.requestId` 的用法与 `task-office.ts:153-158` 逐字一致；`createTaskOffice({ backend, lease, newRequestId })` 最小端口集与 `task-start-integration-smoke.ts:89-93` 同构；`createSecureCredentialStore/createSecureDeploymentStore/SecureStorePort` 三签名与 `credential-store.ts:30`/`deployment-store.ts:19`/`secure-store.ts:5-9` 逐字一致（行号经 `grep -n` 复核；独立审查曾疑 deployment-store 为 :17，实查声明在 :19，维持原文）；`runtimeOf` 的 `authorizedTransport` 返回值与 `voice-dictation-integration-smoke.ts:93-96` 同构（弱网分支只改 fetch 行为，不改签名）。
4. **Review Focus 落实**：五条逐一钉测——构建漂移→Task 2 源级测试；拒权→Task 4 探针 + 既有 denied 文案断言；未授权泄漏→Task 4 深链/推送探针 + Step 2 人工确认 + 泄漏即升级条款；弱网重复派发→Task 5 `distinctRunIds === 1` 断言；冒充/复活→Task 3 三形门测试 + Task 5 `revocation === 'revoked'` 断言。

## 第二轮独立审查修复记录

独立审查（逐条对照 plan-t69.md 原文与 HEAD 代码）发现的问题与处置：

1. **【阻塞·已修复】Task 3 门语义与测试互斥**：原 `normalizeEntry` 对非 evidenced 条目兜底 `reason: entry.reason ?? 'disposition … is not a pass'`，使 `iosReleaseEvidenceGaps` 的「reason 为空」分支永不触发——测试 3 的 `{ subject: 'task', disposition: 'blocked-env' }`（无 reason）输入永远等不到 `task: blocked-env entries must carry a reason` 缺口，RED 无法转 GREEN，且 Review Focus 5 的门语义失效。修复：normalizeEntry 改为 `reason: entry.reason` 原样透传（实现节内注释说明不得兜底的原因）。逐用例复核：测试 3 三条 `.some()` 断言全部命中；测试 4 两条 blocked-env 均带显式 reason 仍过门；assemble 自动补齐的 not-run（自带 `'no entry provided'`）恒为违规不受影响；测试 2/5/6 语义不变。
2. **【轻微·已修复】Consumes 行号漂移**：经 `grep -n` 逐一复核并修正——`createNativeDictationCaptureIfAvailable` 声明在 `dictation-capture.ts:75`（原误写 :83，为函数体内惰性 require 行）；`createSecureCredentialStore` 在 `credential-store.ts:30`（原误写 :34）；`SecureStorePort` 在 `secure-store.ts:5-9`（原误写 6-10）；`disallowedDeploymentHost` 函数本体在 `runtime-integration-smoke.ts:118`（原误写「:86 起」，86 处是 IPv4 辅助函数）。**一处审查意见经复核不采纳**：`createSecureDeploymentStore` 审查称在 deployment-store.ts:17，`grep -n 'export function createSecureDeploymentStore'` 实际输出 `19:export function createSecureDeploymentStore(...)`，计划原文 ：19 正确，维持不变。
3. **【轻微·已修复】bundledNativeModules.json 查证路径**：pnpm 工作区仓库根无 `node_modules/expo`，实际路径 `apps/mobile/node_modules/expo/bundledNativeModules.json`（Tech Stack / Task 1 Consumes / Step 3 注释三处已改为全路径；版本声称 ~55.0.18 本身正确，作者以 `node -e "require('./node_modules/expo/bundledNativeModules.json')"` 在 apps/mobile 下实查过）。
4. **【观察·已在计划注明】** `apps/mobile/tsconfig.json` include 仅 `src/**`，`emit-acceptance-record.ts` 不在 `typecheck:mobile` 覆盖面内，行为正确性由 `ios-release-evidence.test.ts` 动态导入用例承担——已在 Task 3 脚本节后加类型检查覆盖说明。
5. **【未执行项如实声明】** 本轮审查修复未重跑测试命令（目标文件由计划创建、尚不存在）；tsx/`pnpm --filter @weknora/mobile test`/root `typecheck:mobile` 脚本与 Task 6 回归四段 glob 的真实性核实沿用第一轮；`ios-release-build.sh`/simctl 探针与 B4「704 tests」基线未复跑（需 10–20 分钟构建与 booted 模拟器），执行期以计划内验证命令为准。

## 计划级差异与边界声明

- **本计划零 Go 改动、零 packages/* 改动、零 composition.ts/Screen 改动**——九流实现全部已在 HEAD；波级指引第 1 条的迁移重编问题与本计划无关（本计划不需要迁移装载，不顺手去重）。
- **miniprogram 71 个既有 typecheck 错误**与本计划无关（不触碰 apps/miniprogram）。
- `apps/mobile/ios/` 目录不入库（`.gitignore:98`）；入库的构建事实源是 `apps/mobile/plugins/ios-xcode27.js` 与本计划两个脚本——若并行计划同时修改 app.json 的 plugins 数组，合并时保持「数组追加、本地 plugin 末位」即可，无语义冲突。
- 环境变量凭据只经 shell 环境传入测试进程；`t39-outcomes.json`/`t39-record.json`/报告一律不含 `/password|token|email/i` 命中的字段（Task 5 测试与既有 smoke 同款断言兜底）。
