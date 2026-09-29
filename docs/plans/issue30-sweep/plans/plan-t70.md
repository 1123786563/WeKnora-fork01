# T40：Android 安装包核心工作流验收（Issue #70）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 Android Release 包具备可构建、可安装、可验收的完整工程面（原生工程可从零 prebuild 生成、Release 签名/构建配置齐备、原生工作流依赖已声明），并把 Issue #70 要求的七项核心工作流（系统返回、后台限制、通知、文件 URI、Keystore、麦克风、弱网恢复）收敛为「双平台同一领域 Interface + 平台差异仅在 Adapter」的结构化验收矩阵——其中本地可验证部分全部以真实测试/集成冒烟证据落库，真机/外部凭据残余如实声明 blocked-env，绝不伪造通过。

**Architecture:** 七项工作流的领域面已由前序批次在 `packages/mobile-core`（深 Module）+ `packages/api-client`（wire Adapter）+ `apps/mobile/src/adapters/`（平台 Adapter 层）建成；本计划不改任何领域模块的行为语义，只做四类收口：①补齐两个真实的 Android Adapter 缺口（Android 13+ 通知运行时权限、录音临时文件的确定性删除——#56 明确延迟到 #69/#70 的项）；②Android Release 工程配置面（app.json android 段、eas.json、原生依赖声明、CNG gitignore 约定）；③把「平台差异仅在 Adapter」与「验收矩阵引用真实证据」变成可执行的源级守卫测试；④全部新增测试走既有 RED→GREEN 节奏，集成证据沿用 `WEKNORA_MOBILE_TEST_*` opt-in 模式。**Go 服务端零改动，无迁移装载需求（按波级指引不触碰迁移去重）。**

**Tech Stack:** TypeScript（`apps/mobile` Expo ~55 / RN 0.83 / expo-router ~55；`packages/mobile-core` 深模块）、node:test + tsx（`pnpm --filter @weknora/mobile test`）、Expo CNG（`npx expo prebuild -p android --no-install`，本计划作者已实跑成功）、EAS Build 配置（`eas.json`，声明式）。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-70.md`（验收标准原文见下方 Global Constraints 末尾；What to build 原文：「真实 Android Release 包完成核心流程，并验证系统返回、后台限制、通知、文件 URI、Keystore、麦克风和弱网恢复。」）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（Testing Decisions：「Native release evidence must come from installed iOS and Android builds with real login, background/foreground recovery, weak network, notifications, downloads, voice permissions and secure storage.」「True external dependencies such as APNs, FCM, WebRTC, system audio and system share use mock or scripted Adapters at the Port and real-device acceptance separately.」「Tests target observable behavior at the highest stable Interface. Screen tests verify rendering and navigation; they do not duplicate Module internals.」；User Story 80「As a release operator, I want iOS and Android behavior validated on real devices, so that browser prototypes and bundle exports are not mistaken for native acceptance.」；Out of Scope 不含本计划新增面）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§10 App Shell 与 presentation Adapter——apps/mobile 只含 composition/navigation/screens/**native adapters**；禁止 Screen 直接导入 contracts/api-client；§12 依赖分类——APNs/FCM、OS audio、system share 属 true external，Port + native Adapter + scripted Adapter，真机验收单独进行；§13 Interface 测试面）
- ADR：`docs/adr/0005-weknora-native-mobile-client.md`（原生移动端以 WeKnora 领域模型重建）、`docs/adr/0007-registered-devices-and-encrypted-cache.md`（可撤销设备身份 + 加密离线缓存；登出/撤销后密钥不可用——Keystore 工作流语义）、`docs/adr/0010-self-hosted-mobile-push.md`（自托管盲推送——本地无 FCM 凭据时的通知通道形态）、`docs/adr/0012-mobile-business-logic-lives-behind-deep-modules.md`（移动业务逻辑在深 Module 后，Adapter 不拥有业务时序）
- 领域术语：`CONTEXT.md`（「语音交互记录……原始音频默认在实时处理后删除」CONTEXT.md:339——本计划 Task 3/4 的音频清理语义权威）
- 前置：#40、#41、#56、#66 的代码产出均已在当前 HEAD（逐项核实见「与调查结论的差异记录」第 3 条）；下游：#71（跨平台发布证据矩阵）消费本计划的验收矩阵与证据台账

## Global Constraints

以下为批准 Spec / ADR / Issue 的项目级约束，逐字引用，所有任务隐含遵守：

- 「证据来自可安装 Release 包。」（Issue #70 验收标准 1）——Android Release 包的构建/安装/真机运行在本环境不可执行（无 Gradle、无 Android SDK、无真机、无 release Keystore，见 blocked-env 声明），本计划交付可构建工程面 + 逐条如实声明的 blocked-env 残余 + 本地可验证的等价证据，**不得把任何本地替身记为真机通过**。
- 「与 iOS 的领域行为一致，平台差异仅在 Adapter。」（Issue #70 验收标准 2）——双平台共享同一 `packages/mobile-core` 深模块与同一 screens/composition；平台 API（react-native Platform/BackHandler/AppState 等）只允许出现在 `apps/mobile/src/adapters/`（Task 7 源级扫描强制）；本计划不新增任何 `Platform.OS` 分支到 screens/领域层。
- 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」（Issue #70 验收标准 3）——工作流的本地端到端证据是既有真实 HTTP/SSE 集成冒烟（`*-integration-smoke.test.ts`，opt-in `WEKNORA_MOBILE_TEST_*`）；mock/scripted Adapter 只用于 true external 面（spec §12）；验收矩阵引用的证据文件必须真实存在（Task 7 existsSync 强制）。
- 「Mobile core does not depend on React Native, DOM or concrete transport. Remote and native details are injected as Adapters.」（design spec · Implementation Decisions）——Task 3 的 `audioCleanup` 是 `DictationPorts` 上的注入端口，`packages/mobile-core` 不出现 require/RN 类型。
- 「Voice Room 拥有实时语音 session、microphone permission、音频状态、转写草稿和断线结束语义。」（module-seams §8.1）＋「原始音频默认在实时处理后删除」（CONTEXT.md:339）——录音临时文件的删除时序由模块（dropIntent）与 Adapter（cancel 路径）分担，均为尽最大努力且失败静默，绝不影响转写/取消主流程；VoiceRoom 实时会话的原音频删除属 #57，本计划不触碰。
- 「True external dependencies such as APNs, FCM, WebRTC, system audio and system share use mock or scripted Adapters at the Port and real-device acceptance separately.」（design spec · Testing Decisions）——FCM/系统返回手势/Doze/硬件 Keystore 的真机验收属 blocked-env；本地以 scripted Adapter + 既有集成冒烟为等价证据。
- 服务端 URL/凭据约束（会话注入）：集成冒烟复用 `disallowedDeploymentHost` 主机防线（拒 localhost/环回/私网/链路本地/保留地址）；凭据只从 `WEKNORA_MOBILE_TEST_*` 环境变量读取，源码与测试不写可用凭据字面量；无凭据时 `t.skip`，不伪造。本计划 **Go/SQL 零改动**（参数绑定约束自动满足）。
- 工作流约束：严格 RED→GREEN→REFACTOR（每个任务先写失败测试、实跑确认失败、最小实现、通过、提交）；实现不与已批准 Spec 冲突，冲突时升级而非静默重设计。
- CNG 约定（仓库现状，根 `.gitignore:97-98` 注释：「apps/mobile 为 CNG（expo prebuild）流程：ios/ 目录是生成产物，勿提交（ios-xcode27 插件负责生成内容）」）——`apps/mobile/android/` 沿用同一约定：gitignore 忽略，本计划无 Android 模板缺陷需修，不新增 Android config plugin。

**Issue #70 验收标准原文（docs/plans/issue30-sweep/issues/issue-70.md）：**

1. 「证据来自可安装 Release 包。」
2. 「与 iOS 的领域行为一致，平台差异仅在 Adapter。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

**blocked-env 验收项声明（本地不可验证、如实列阻、不给本地替身记通过）：**

| # | 验收残余 | 阻塞原因（本计划作者实测） | 本地等价证据 | 解锁条件 |
|---|---|---|---|---|
| B1 | Android Release 包构建 + 真机安装运行（验收标准 1 的直接面） | `which gradle adb sdkmanager` 全部不存在、`ANDROID_HOME/ANDROID_SDK_ROOT` 未设（java 17 存在但 SDK 缺失）；`android/app/build.gradle:110-115` release buildType 仍签 debug keystore（模板注释「Caution! In production, you need to generate your own keystore file.」）；无真机 | Task 5：prebuild 从零生成成功 + 生成物清单证据入库 + `eas.json` Release profile + 签名/构建 runbook | Android SDK + 真机，或 EAS 构建队列（`eas build -p android --profile preview`） |
| B2 | 真实 FCM 推送通道通知送达 | 服务端 #67 本地形态为 disabled provider；本环境无 FCM 凭据 | 盲推送/前台同步/设备注册集成冒烟（`blind-push-integration-smoke.test.ts`、`device-inbox-integration-smoke.test.ts`）+ Task 1/2 的 POST_NOTIFICATIONS 权限路径 | 有 FCM 凭据的部署 + 真机 |
| B3 | 真机系统返回手势/返回键、Doze 后台限制、硬件 backed Keystore、麦克风真实拒权/录音 | 全部需要 Android 真机 OS 行为 | `app-state.test.ts`（Task 6，生命周期两态映射）、`foreground-sync.test.ts`、`dictation-capture.test.ts`（Task 4，拒权/取消路径）、`offline-vault-integration-smoke.test.ts`（Keystore 语义面：SecureStore 端口 + 加密投影） | Android 真机验收轮（与 #69 iOS 同型） |

## 与调查结论的差异记录（以代码现状为准，均为本计划作者亲眼核实/实跑）

1. 调查证据「`apps/mobile/src/adapters/` 仅 credential-store/deployment-store/secure-store/oidc-browser 四个登录相关适配器，不存在平台差异 Adapter 层」**已严重过时**：当前 HEAD 的 adapters/ 有 **24 个文件（15 个源文件 + 9 个测试文件）**，15 个源模块为：app-state、credential-store、deployment-registry(+test)、deployment-store、device-identity(+test)、dictation-capture(+test)、intent-log(+test)、material-adapters、network-status(+test)、oidc-browser（其测试文件名为 `oidc-adapters.test.ts`）、push-token、request-id(+test)、secure-store、sse-stream(+test)、vault-adapters(+test)，覆盖生命周期/推送/设备身份/麦克风/网络/分享/SSE/Vault/意图日志全 seams。本计划只补两个真实缺口（Task 1 通知权限、Task 3/4 音频清理）。
2. 调查缺口「核心工作流除登录外均未实现，与 iOS 领域行为一致性无从验证」**已过时**：七工作流的领域面已实现且有真实集成冒烟——通知（`blind-push-integration-smoke.ts` #67、`device-inbox-integration-smoke.ts` #41）、麦克风/文件 URI（`voice-dictation-integration-smoke.ts` #56、`material-integration-smoke.ts` #46）、弱网恢复（`offline-vault-integration-smoke.ts` #40）、后台限制（`foreground-sync.ts` #67）、Keystore（`vault-adapters.ts`/`intent-log.ts`/`deployment-registry.ts` #32/#36/#66）、系统返回（`app/auth-return.tsx` + `app-smoke.test.tsx:81-96` OIDC 回调路由测试）。缺的是 Android 打包面与下述 Adapter 缺口。
3. 调查缺口「前置 #40、#41、#56、#66 均为 open 未完成」**已过时**：四者代码产出均在 HEAD（`offline-vault-integration-smoke.ts`、`device-inbox-integration-smoke.ts` + composition `registerActiveDeviceIfPossible`、`adapters/dictation-capture.ts` + `voice-dictation-integration-smoke.ts`、`adapters/deployment-registry.ts` + `listDeployments/switchDeployment` 接线，逐一读源确认）。
4. 波级指引称 iOS「工程已生成并提交」——**与 git 事实不符**：`apps/mobile/ios/` 被根 `.gitignore:98` 显式忽略（`git ls-files apps/mobile/ios` 计 0），原生定制以 `plugins/ios-xcode27.js` config plugin 入库。Android 沿用同一 CNG 约定（Task 5）。
5. **`android.blockedPermissions` 探测无效**：本计划作者以临时 app.json 配置实跑 prebuild 两次——`android.permissions` 白名单会把显式列出的权限（含 RECORD_AUDIO/POST_NOTIFICATIONS）写入生成的 AndroidManifest.xml，但模板默认权限（READ/WRITE_EXTERNAL_STORAGE ≤32、SYSTEM_ALERT_WINDOW）即使声明 `blockedPermissions` 也**不被剔除**（探测后已还原 app.json 并删除 android/）。本计划不做权限裁剪承诺，只做显式声明（验收所需权限的最小显式集），模板默认权限的剔除留待真实证据出现后再议。
6. **prebuild 实测可行**：`cd apps/mobile && npx expo prebuild -p android --no-install` 在本环境 exit 0（纯 Node，无需 Android SDK），生成的 `android/app/src/main/AndroidManifest.xml` 已带 `<data android:scheme="weknora"/>` VIEW intent-filter 与 `android:launchMode="singleTask"`（深链 warm/cold 返回的 Android 前置齐备，无需自定义 plugin）；`android/app/debug.keystore` 由 prebuild 自动生成。库 manifest（如 expo-notifications 的 POST_NOTIFICATIONS/RECEIVE_BOOT_COMPLETED）在 Gradle 构建期合并、不在 prebuild 产物中——故 RECORD_AUDIO/POST_NOTIFICATIONS 必须在 app.json 显式声明才能进入 prebuild 产物（探测证据）。
7. 原生依赖现状：`expo-audio`、`expo-network`、`expo-file-system` 均**不在** `apps/mobile/package.json`（Node 测试链靠惰性 require fail closed，`dictation-capture.ts:7`、`network-status.ts:3` 注释明确「真机构建前置」）。#70 是该前置的落地批次（Task 5），版本取自 `apps/mobile/node_modules/expo/bundledNativeModules.json`（实读）：`expo-audio ~55.0.18`、`expo-network ~55.0.18`、`expo-file-system ~55.0.26`。

## Review Focus

Spec 隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **通知权限被拒后的假注册/重试风暴**：Android 13+ 拒绝 POST_NOTIFICATIONS 后仍注册设备——收不到可显示通知的设备记录是幽灵数据；反复弹窗则骚扰用户。——Task 2 测试「denied permission short-circuits registration as permission-denied without touching the token or registry」+ Task 1 测试「already-granted permission never re-prompts」（已授权零弹窗）；bounded 重试由既有 `registrationAttempts` 上限（composition.ts MobileApp effect）承担。
2. **取消录音后临时音频文件滞留**（文件 URI 工作流 + CONTEXT.md:339 原音频删除）：cancel 路径的音频从未进入模块，若 Adapter 不删，临时文件滞留至平台回收。——Task 4 测试「cancel deletes the recorder's temp file best-effort」+ Task 3 测试「dropping an intent deletes the uri-sourced audio exactly once」（模块侧覆盖 stop 产物路径：成功/空转写/放弃/换新开始）。
3. **清理失败破坏主流程**：deleteAsync 抛错若外泄，会让取消/转写失败或产生 unhandled rejection——清理必须尽最大努力。——Task 3 测试「a rejecting cleanup never fails the flow nor leaks an unhandled rejection」+ Task 4 同型测试。
4. **平台 API 泄漏进 Screen/领域层造成双平台漂移**：后续演进把 `require('react-native')`/Platform 分支写进 screens 或 mobile-core——违反验收标准 2 且无测试拦截。——Task 7 测试「platform APIs stay inside src/adapters」（全 src 递归扫描 + sanity 断言扫描器真的看到了 adapters 内的合法接线）。
5. **验收矩阵沦为纸面装饰**：矩阵引用不存在的证据文件、或把 blocked-env 残余记成通过——验收标准 3 被静默架空。——Task 7 测试「the Android acceptance matrix covers exactly the seven core workflows with real evidence paths」（七工作流精确覆盖 + existsSync 全部证据 + residual.kind 恒 'blocked-env' 且 reason/unblock 非空）。

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 1 | 通知权限 Adapter（Android 13+ POST_NOTIFICATIONS） | `apps/mobile/src/adapters/notification-permission.ts`（注入式 + 惰性原生 fail closed）+ 测试 |
| 2 | 组合根接线：设备注册前请求通知权限 | `composition.ts` `registerActiveDeviceIfPossible` 增第 4 可选参 + `'permission-denied'` 返回值 + app-smoke 行为测试 |
| 3 | mobile-core：Dictation 原始音频清理端口 | `packages/mobile-core/src/voice/dictation.ts` `DictationPorts.audioCleanup?` + dropIntent 清理 + 测试 |
| 4 | 录音临时文件清理 Adapter | `dictation-capture.ts`（cancel 路径删除 + `createNativeAudioFileCleanupIfAvailable`）+ composition dictationFor 接线 + 测试 |
| 5 | Android Release 工程配置 | `app.json` android 段补全 + `apps/mobile/.gitignore` android/ + `eas.json` + `package.json` 原生依赖 + 配置测试 + prebuild 证据入库 + runbook |
| 6 | AppState 生命周期 Adapter 测试补位 | `apps/mobile/src/adapters/app-state.test.ts`（两态映射 + no-op 降级） |
| 7 | 平台一致性扫描 + 七工作流验收矩阵 | `android-acceptance.ts`（矩阵数据）+ `android-parity.test.ts`（扫描 + 矩阵完整性） |

### Task 1: 通知权限 Adapter（Android 13+ POST_NOTIFICATIONS）

**Files:**
- Create: `apps/mobile/src/adapters/notification-permission.ts`
- Test: `apps/mobile/src/adapters/notification-permission.test.ts`

**Interfaces:**
- Consumes: `expo-notifications` 的 `getPermissionsAsync()/requestPermissionsAsync()`（SDK 55 面：`NotificationPermissionsStatus extends PermissionResponse { granted: boolean; ... }`，`apps/mobile/node_modules/expo-notifications/build/NotificationPermissions.d.ts:21,44` 实读核实）；既有惰性 require fail-closed 模式（`adapters/push-token.ts`、`adapters/dictation-capture.ts` 同型）。
- Produces: `NotificationPermissionPort { ensure(): Promise<'granted'|'denied'|'unavailable'> }`、`createNotificationPermissionFrom(notifications)`（注入式）、`createNativeNotificationPermissionIfAvailable(): NotificationPermissionPort | undefined`（惰性原生）——Task 2 的组合根按此消费。

- [ ] **Step 1: Write the failing test**

创建 `apps/mobile/src/adapters/notification-permission.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createNativeNotificationPermissionIfAvailable, createNotificationPermissionFrom, type ExpoNotificationsPermissionsLike } from './notification-permission.ts';

interface StubOptions {
  /** getPermissionsAsync 的 granted（缺省 false = 触发请求路径）。 */
  currentGranted?: boolean;
  /** requestPermissionsAsync 的 granted（缺省 true）。 */
  requestGranted?: boolean;
  /** getPermissionsAsync 必抛（权限面不可用）。 */
  failGet?: boolean;
  /** requestPermissionsAsync 必抛。 */
  failRequest?: boolean;
}

function stubPermissions(options: StubOptions = {}): ExpoNotificationsPermissionsLike & { requests: number } {
  const state = { requests: 0 };
  return Object.assign(state, {
    async getPermissionsAsync() {
      if (options.failGet === true) throw new Error('permissions module broken');
      return { granted: options.currentGranted ?? false };
    },
    async requestPermissionsAsync() {
      state.requests += 1;
      if (options.failRequest === true) throw new Error('user never saw the prompt');
      return { granted: options.requestGranted ?? true };
    },
  }) as ExpoNotificationsPermissionsLike & { requests: number };
}

test('the native factory is unavailable in the Node test chain (fail closed)', () => {
  assert.equal(createNativeNotificationPermissionIfAvailable(), undefined, 'expo-notifications 不可 require 时必须返回 undefined');
});

test('an already-granted permission never re-prompts the user', async () => {
  const notifications = stubPermissions({ currentGranted: true });
  const port = createNotificationPermissionFrom(notifications);
  assert.equal(await port.ensure(), 'granted');
  assert.equal(notifications.requests, 0, '已授权零弹窗请求（Review Focus 1：不骚扰用户）');
});

test('an ungranted permission prompts exactly once and reports the honest outcome', async () => {
  const granted = createNotificationPermissionFrom(stubPermissions({ currentGranted: false, requestGranted: true }));
  assert.equal(await granted.ensure(), 'granted');
  const denied = createNotificationPermissionFrom(stubPermissions({ currentGranted: false, requestGranted: false }));
  assert.equal(await denied.ensure(), 'denied');
});

test('a broken permissions surface reports unavailable and never fabricates granted', async () => {
  const brokenGet = createNotificationPermissionFrom(stubPermissions({ failGet: true }));
  assert.equal(await brokenGet.ensure(), 'unavailable');
  const brokenRequest = createNotificationPermissionFrom(stubPermissions({ failRequest: true }));
  assert.equal(await brokenRequest.ensure(), 'unavailable');
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm exec tsx --test apps/mobile/src/adapters/notification-permission.test.ts`
Expected: FAIL —— `Cannot find module './notification-permission.ts'`（模块不存在）。

- [ ] **Step 3: Write minimal implementation**

创建 `apps/mobile/src/adapters/notification-permission.ts`：

```ts
/**
 * 通知权限 Adapter（Issue #70：Android 13+ POST_NOTIFICATIONS 运行时权限）。Android 13 起
 * 通知运行时权限弹窗不随 FCM token 获取自动出现：未授权时通知静默不显示，设备注册与 Inbox
 * 权威同步形同虚设。iOS 的授权请求与本 Adapter 走同一 Expo 面（平台差异收敛在
 * expo-notifications 内部，App 层无平台分支——module-seams §10「平台差异仅在 Adapter」）。
 * 惰性 require：Node 测试链/缺原生配置的构建解析失败返回 undefined（fail closed：不注册
 * 设备、不阻塞登录/授权主流程——与 push-token/device-identity 同型先例）。
 */
export type NotificationPermissionOutcome = 'granted' | 'denied' | 'unavailable';

export interface NotificationPermissionPort {
  /** 已授权零弹窗；未授权恰好请求一次；结果如实三值，绝不伪造 granted。 */
  ensure(): Promise<NotificationPermissionOutcome>;
}

/** expo-notifications 权限面的结构切片（只依赖本 Adapter 真正消费的形状）。 */
export interface ExpoNotificationsPermissionsLike {
  getPermissionsAsync(): Promise<{ granted?: boolean }>;
  requestPermissionsAsync(): Promise<{ granted?: boolean }>;
}

/** 注入式构造（测试注入 stub；与 dictation-capture 的 ExpoAudioLike 注入同型）。 */
export function createNotificationPermissionFrom(notifications: ExpoNotificationsPermissionsLike): NotificationPermissionPort {
  return {
    async ensure() {
      try {
        const current = await notifications.getPermissionsAsync();
        if (current?.granted === true) return 'granted'; // 已授权：零弹窗请求
      } catch {
        return 'unavailable'; // 权限面不可用：如实上抛，不伪造结论
      }
      try {
        const requested = await notifications.requestPermissionsAsync();
        return requested?.granted === true ? 'granted' : 'denied';
      } catch {
        return 'unavailable';
      }
    },
  };
}

/** 原生组合路径（惰性 require；解析失败/缺导出 fail closed → undefined）。 */
export function createNativeNotificationPermissionIfAvailable(): NotificationPermissionPort | undefined {
  try {
    const notifications = require('expo-notifications') as Partial<ExpoNotificationsPermissionsLike>;
    if (typeof notifications.getPermissionsAsync !== 'function' || typeof notifications.requestPermissionsAsync !== 'function') {
      return undefined;
    }
    return createNotificationPermissionFrom(notifications as ExpoNotificationsPermissionsLike);
  } catch {
    return undefined;
  }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm exec tsx --test apps/mobile/src/adapters/notification-permission.test.ts`
Expected: PASS（5 tests, 0 fail）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/adapters/notification-permission.ts apps/mobile/src/adapters/notification-permission.test.ts
git commit -m "feat(mobile): 通知权限 Adapter（Android 13+ POST_NOTIFICATIONS，注入式 + 惰性 fail closed）"
```

### Task 2: 组合根接线——设备注册前请求通知权限

**Files:**
- Modify: `apps/mobile/src/composition.ts`（import 区 + `registerActiveDeviceIfPossible` 签名与权限短路，见下方锚点）
- Test: `apps/mobile/src/app-smoke.test.tsx`（既有 `device registration is fail-closed...` 测试之后追加一个测试）

**Interfaces:**
- Consumes: Task 1 的 `NotificationPermissionPort`/`createNativeNotificationPermissionIfAvailable`；既有 `registerActiveDeviceIfPossible(activeRuntime, tokenSource?, identity?)`（composition.ts:249-275（2026-09-26 快照行号，共享 worktree 有并行批次漂移——**以引号内代码原文为锚**），返回值联合 `'registered' | 'no-token' | 'no-device-id' | 'unauthorized' | 'failed'`）；调用方 MobileApp effect 只判 `outcome === 'registered'`，新增联合成员不破坏它。
- Produces: `registerActiveDeviceIfPossible(activeRuntime, tokenSource?, identity?, permission?: NotificationPermissionPort)` 返回值联合扩为 `'registered' | 'no-token' | 'no-device-id' | 'permission-denied' | 'unauthorized' | 'failed'`；`permission === undefined`（Node/缺原生配置）时保持既有行为——既有两个调用点（app-smoke 既有测试的位置参数两参调用、composition.ts MobileApp effect 的默认参调用）零改动。

- [ ] **Step 1: Write the failing test**

在 `apps/mobile/src/app-smoke.test.tsx` 中、`test('device registration is fail-closed without a native push token or device identity', ...)` 整个测试块之后追加：

```tsx
test('device registration requests notification permission first and reports denial honestly (#70)', async () => {
  const { registerActiveDeviceIfPossible } = await import('./composition.ts');
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const authorizedRuntime = {
    snapshot: () => ({ surface: 'authorized' as const, deployment: { origin: 'https://weknora.example.test', label: 'Test' } }),
    authorizedRequest: async () => { throw new Error('must not reach the wire without a token'); },
    scopeLease: () => undefined,
  };
  const deniedTokens: string[] = [];
  // denied：权限短路发生在 token 获取之前，绝不注册无权限设备（Review Focus 1）
  const denied = await registerActiveDeviceIfPossible(
    authorizedRuntime as never,
    { token: async () => { deniedTokens.push('fetched'); return 'tok'; } },
    { deviceId: async () => 'device-1' },
    { ensure: async () => 'denied' },
  );
  assert.equal(denied, 'permission-denied');
  assert.deepEqual(deniedTokens, [], 'denied 后不得触碰 push token 通道');
  // granted：走完既有 fail-closed 注册链（本 stub 环境 registry.register 到 wire 即抛 → failed），绝不被权限层短路
  const granted = await registerActiveDeviceIfPossible(
    authorizedRuntime as never,
    { token: async () => 'tok' },
    { deviceId: async () => 'device-1' },
    { ensure: async () => 'granted' },
  );
  assert.notEqual(granted, 'permission-denied');
  // unavailable：保持既有行为——继续走 token fail-closed 路径
  const unavailable = await registerActiveDeviceIfPossible(
    authorizedRuntime as never,
    { token: async () => undefined },
    { deviceId: async () => 'device-1' },
    { ensure: async () => 'unavailable' },
  );
  assert.equal(unavailable, 'no-token');
  // 组合根默认参必须接原生权限 Adapter（真机路径生效的唯一接线点）
  const composition = readFileSync(join(here, 'composition.ts'), 'utf8');
  assert.match(composition, /permission[^=]*=\s*createNativeNotificationPermissionIfAvailable\(\)/, '默认参必须惰性接原生权限 Adapter');
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx`
Expected: FAIL —— 新测试断言 `permission-denied` 实际得到 `'failed'`（现签名无权限参，registry.register 走到 wire 抛错）。

- [ ] **Step 3: Write minimal implementation**

修改 `apps/mobile/src/composition.ts` 三处：

①import 区（`createNativeAppStateLifecycle` 的 import 行之后）加：

```ts
import { createNativeNotificationPermissionIfAvailable, type NotificationPermissionPort } from './adapters/notification-permission.ts';
```

②`registerActiveDeviceIfPossible` 全函数替换为（锚点：现 `composition.ts:249-275`）：

```ts
/** 设备注册入口（spec §4「注册设备与 App 前后台生命周期」）：无原生 push token 或无安全
 * 设备身份时 fail closed 跳过；注册失败不阻塞授权主流程（best effort）。
 * #70：Android 13+ 通知运行时权限先于 token 获取——权限弹窗不随 token 自动出现，denied
 * 时注册出的是收不到可显示通知的幽灵设备；短路结果如实上报 'permission-denied'，bounded
 * 重试由调用方既有 attempts 上限承担（composition.ts MobileApp effect）。 */
export async function registerActiveDeviceIfPossible(
  activeRuntime: Pick<MobileRuntime, 'snapshot' | 'authorizedRequest' | 'scopeLease'>,
  tokenSource: { token(): Promise<string | undefined> } = createNativePushTokenIfAvailable(),
  identity: { deviceId(): Promise<string | undefined> } = createNativeDeviceIdentity(),
  permission: NotificationPermissionPort | undefined = createNativeNotificationPermissionIfAvailable(),
): Promise<'registered' | 'no-token' | 'no-device-id' | 'permission-denied' | 'unauthorized' | 'failed'> {
  const snapshot = activeRuntime.snapshot();
  const origin = snapshot.deployment?.origin;
  if (snapshot.surface !== 'authorized' || origin === undefined) return 'unauthorized';
  if (permission !== undefined) {
    const outcome = await permission.ensure();
    if (outcome === 'denied') return 'permission-denied';
    // 'unavailable' 继续走既有链：token 通道自身 fail closed（no-token），不做二次伪造。
  }
  const token = await tokenSource.token();
  if (token === undefined) return 'no-token';
  const deviceId = await identity.deviceId();
  if (deviceId === undefined) return 'no-device-id';
  try {
    await deviceRegistryFor(activeRuntime, origin, snapshot.identity?.activeTenantId ?? '').register({
      deviceId,
      token,
      platform: nativeDevicePlatform(),
      appId: resolveWeKnoraAppId(typeof process !== 'undefined' ? process.env.EXPO_PUBLIC_WEKNORA_APP_ID : undefined),
    });
    return 'registered';
  } catch {
    return 'failed';
  }
}
```

（函数体其余行与现状逐字一致，仅新增第 4 参、返回值联合成员与权限短路块。）

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx`
Expected: PASS（既有全部测试 + 新测试，0 fail——既有 `device registration is fail-closed...` 测试为两参调用，第 4 参默认 `createNativeNotificationPermissionIfAvailable()` 在 Node 返回 undefined → 行为不变）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/composition.ts apps/mobile/src/app-smoke.test.tsx
git commit -m "feat(mobile): 设备注册前请求通知权限，denied 如实上报 permission-denied（#70）"
```

### Task 3: mobile-core——Dictation 原始音频清理端口

**Files:**
- Modify: `packages/mobile-core/src/voice/dictation.ts`（`DictationPorts` 增可选端口 + `dropIntent` 清理，见锚点）
- Test: `packages/mobile-core/src/voice/dictation.test.ts`（追加测试）

**Interfaces:**
- Consumes: 既有 `createDictation(ports: DictationPorts)` 状态机（dictation.ts:105）；`dropIntent()` 释放点（dictation.ts:121-124，调用点 :131、:135、:157、:194、:198、:203、:217、:247）；转写失败路径**保留** requestId/pendingAudio 供幂等重试（dictation.ts:140 注释）。
- Produces: `DictationPorts.audioCleanup?: (audio: DictationAudio) => Promise<void>`——模块在 `dropIntent`（音频不再需要的唯一权威时点）对 `uri` 源音频尽最大努力删除一次；`bytes` 源不调用（内存对象无文件可删）。Task 4 的组合根按此注入原生清理。`packages/mobile-core` 不新增公共导出（端口是 `DictationPorts` 既有导出类型的成员）。

- [ ] **Step 1: Write the failing test**

在 `packages/mobile-core/src/voice/dictation.test.ts` 文件末尾追加（复用该文件既有的 scripted 构造风格，自包含新 helper）：

```ts
test('dropping an intent deletes the uri-sourced audio exactly once via audioCleanup (#70)', async () => {
  const deleted: string[] = [];
  const dictation = createDictation({
    capture: {
      start: async () => 'recording' as const,
      stop: async () => ({ uri: 'file:///cache/d1.m4a', mimeType: 'audio/mp4', fileName: 'd.m4a' }),
      cancel: async () => undefined,
    },
    transcribe: { async transcribe() { return { text: '转写成功' }; } },
    newRequestId: () => 'req-cleanup-1',
    audioCleanup: async (audio) => { if (audio.uri !== undefined) deleted.push(audio.uri); },
  });
  await dictation.begin();
  await dictation.finish();
  await dictation.confirmTranscript();
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(deleted, ['file:///cache/d1.m4a'], '确认（dropIntent 已发生）后原始音频文件被清理恰好一次');
});

test('bytes-sourced audio never triggers audioCleanup (nothing on disk to delete)', async () => {
  let cleanups = 0;
  const dictation = createDictation({
    capture: {
      start: async () => 'recording' as const,
      stop: async () => ({ bytes: new Uint8Array([1, 2, 3]), mimeType: 'audio/wav' }),
      cancel: async () => undefined,
    },
    transcribe: { async transcribe() { return { text: 'ok' }; } },
    newRequestId: () => 'req-cleanup-2',
    audioCleanup: async () => { cleanups += 1; },
  });
  await dictation.begin();
  await dictation.finish();
  await dictation.discardTranscript();
  assert.equal(cleanups, 0, 'bytes 源音频只在内存，无文件可删');
});

test('a failed transcription retains the audio for retry; the eventual drop deletes it once', async () => {
  const deleted: string[] = [];
  let attempts = 0;
  const dictation = createDictation({
    capture: {
      start: async () => 'recording' as const,
      stop: async () => ({ uri: 'file:///cache/retry.m4a', mimeType: 'audio/mp4' }),
      cancel: async () => undefined,
    },
    transcribe: {
      async transcribe() {
        attempts += 1;
        if (attempts === 1) throw new Error('network down');
        return { text: '重试成功' };
      },
    },
    newRequestId: () => 'req-cleanup-3',
    audioCleanup: async (audio) => { if (audio.uri !== undefined) deleted.push(audio.uri); },
  });
  await dictation.begin();
  await dictation.finish();
  assert.equal(deleted.length, 0, '失败重试窗口内音频必须保留（服务端同 requestId 幂等重放），绝不提前删除');
  await dictation.retryTranscription();
  assert.equal(dictation.state().phase, 'review');
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(deleted, ['file:///cache/retry.m4a'], '重试成功进入 review 后（dropIntent）清理恰好一次');
});

test('a rejecting audioCleanup never fails the flow nor leaks an unhandled rejection (Review Focus 3)', async () => {
  const unhandled: unknown[] = [];
  const onUnhandled = (reason: unknown) => { unhandled.push(reason); };
  process.on('unhandledRejection', onUnhandled);
  const dictation = createDictation({
    capture: {
      start: async () => 'recording' as const,
      stop: async () => ({ uri: 'file:///cache/gone.m4a', mimeType: 'audio/mp4' }),
      cancel: async () => undefined,
    },
    transcribe: { async transcribe() { return { text: 'ok' }; } },
    newRequestId: () => 'req-cleanup-4',
    audioCleanup: async () => { throw new Error('EFILEGONE'); },
  });
  await dictation.begin();
  await dictation.finish();
  assert.equal(dictation.state().phase, 'review', '清理失败不得影响转写主流程');
  await new Promise((resolve) => setTimeout(resolve, 20));
  process.off('unhandledRejection', onUnhandled);
  assert.deepEqual(unhandled, [], '清理 rejection 必须被模块吞掉');
});
```

（四个测试均为自包含内联 stub，只依赖该文件既有 import 的 `createDictation`；`DictationTranscriptionPort` 的内联对象形态 `{ async transcribe() { return { text } } }` 与 dictation.ts:53-55 端口逐字一致。）

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm exec tsx --test packages/mobile-core/src/voice/dictation.test.ts`
Expected: FAIL —— `audioCleanup` 尚不存在，模块忽略该端口：第 1/3 个新测试断言失败（uri 源音频的 `deleted` 为空）；第 2 个（bytes 源恒零调用）与第 4 个（无调用即无 rejection，`unhandled` 恒空）在 RED 阶段天然通过——由第 1/3 保证红→绿；`tsc --noEmit` 层面另会报对象字面量多余属性。

- [ ] **Step 3: Write minimal implementation**

修改 `packages/mobile-core/src/voice/dictation.ts` 两处：

①`DictationPorts`（现 :69-75）替换为：

```ts
export interface DictationPorts {
  capture: DictationCapturePort;
  transcribe: DictationTranscriptionPort;
  newRequestId(): string;
  /** 录音时长上限（毫秒）；缺省 DICTATION_MAX_DURATION_MS。到点自动 finish。 */
  maxDurationMs?: number;
  /** 原始音频删除端口（module-seams §8.1 + CONTEXT.md:339「原始音频默认在实时处理后删除」；
   *  #56 延迟项由 #70 闭合）：模块在 dropIntent（音频不再需要的唯一权威时点）对 uri 源音频
   *  尽最大努力删除一次；bytes 源在内存中不调用。缺省不删除（Node/集成冒烟场景）。 */
  audioCleanup?: (audio: DictationAudio) => Promise<void>;
}
```

②`dropIntent`（现 :121-124）替换为：

```ts
  const dropIntent = (): void => {
    requestId = undefined;
    const audio = pendingAudio;
    pendingAudio = undefined; // 原始音频即刻释放（失败重试窗口结束）
    if (audio?.uri !== undefined && ports.audioCleanup !== undefined) {
      void ports.audioCleanup(audio).catch(() => undefined); // 尽最大努力：清理失败不外泄、不阻塞主流程
    }
  };
```

（转写失败路径不调用 dropIntent 的既有语义不变——重试窗口内音频保留。）

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm exec tsx --test packages/mobile-core/src/voice/dictation.test.ts`
Expected: PASS（既有全部 + 新增 4 个，0 fail）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/voice/dictation.ts packages/mobile-core/src/voice/dictation.test.ts
git commit -m "feat(mobile-core): Dictation 增 audioCleanup 端口——dropIntent 即删除 uri 源原始音频（#56 延迟项闭合）"
```

### Task 4: 录音临时文件清理 Adapter（cancel 路径 + 原生清理端口）

**Files:**
- Modify: `apps/mobile/src/adapters/dictation-capture.ts`
- Modify: `apps/mobile/src/composition.ts`（`dictationFor` 注入 audioCleanup + 模块级单例）
- Test: `apps/mobile/src/adapters/dictation-capture.test.ts`（追加测试）

**Interfaces:**
- Consumes: Task 3 的 `DictationPorts.audioCleanup` 形态；既有 `createDictationCaptureFrom(expoAudio)`/`createNativeDictationCaptureIfAvailable()`（dictation-capture.ts:28/:75）；`composition.ts` `dictationFor`（deployment scope 记忆化工厂，以引号内代码原文为锚）。
- Produces: `AudioFileCleanupPort { deleteAsync(uri: string): Promise<void> }`、`createDictationCaptureFrom(expoAudio, cleanup?)`（第 2 可选参——cancel 路径删除录音机临时文件）、`createNativeAudioFileCleanupIfAvailable(): AudioFileCleanupPort | undefined`（惰性 require `expo-file-system`）——组合根注入 Task 3 的 `audioCleanup` 时注入**函数形态**（`(audio: DictationAudio) => Promise<void>` 包装 `deleteAsync`，仅 uri 源调用；类型逐字对齐 `DictationPorts.audioCleanup`，直接注入对象会 typecheck 失败且运行时同步抛 TypeError）。

- [ ] **Step 1: Write the failing test**

在 `apps/mobile/src/adapters/dictation-capture.test.ts` 文件末尾追加，并把文件顶部 import 行扩为：

```ts
import { createDictationCaptureFrom, createNativeAudioFileCleanupIfAvailable, createNativeDictationCaptureIfAvailable, type ExpoAudioLike } from './dictation-capture.ts';
```

```ts
test('cancel deletes the recorder temp file best-effort (Review Focus 2: 文件 URI 不滞留)', async () => {
  const deleted: string[] = [];
  const cleanup = { deleteAsync: async (uri: string) => { deleted.push(uri); } };
  const capture = createDictationCaptureFrom(fakeExpoAudio(), cleanup);
  await capture.start();
  await capture.cancel();
  assert.deepEqual(deleted, ['file:///cache/dictation.m4a'], '取消路径的音频从未进入模块，临时文件由 Adapter 即刻删除');
  assert.equal(await capture.stop(), undefined, '取消后无残留捕获');
});

test('a failing temp-file deletion on cancel is swallowed (cleanup is best-effort, Review Focus 3)', async () => {
  const capture = createDictationCaptureFrom(fakeExpoAudio(), {
    deleteAsync: async () => { throw new Error('EFILEGONE'); },
  });
  await capture.start();
  await capture.cancel(); // 不得抛出
  assert.equal(await capture.stop(), undefined);
});

test('stop-produced audio is the module audioCleanup responsibility; the adapter passes cleanup through the native factory wiring (#70)', async () => {
  // stop 路径不删：返回的 uri 要被模块用于 multipart 上载，删除时点由 Dictation dropIntent 权威决定（Task 3）。
  const deleted: string[] = [];
  const capture = createDictationCaptureFrom(fakeExpoAudio(), { deleteAsync: async (uri) => { deleted.push(uri); } });
  await capture.start();
  const audio = await capture.stop();
  assert.equal(audio?.uri, 'file:///cache/dictation.m4a');
  assert.deepEqual(deleted, [], 'stop 路径的删除时点属于模块（audioCleanup 端口），Adapter 不得抢先删除');

  // 原生文件清理 Adapter 在 Node 链 fail closed（expo-file-system 未安装——这正是惰性边界）
  assert.equal(createNativeAudioFileCleanupIfAvailable(), undefined);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm exec tsx --test apps/mobile/src/adapters/dictation-capture.test.ts`
Expected: FAIL —— `createNativeAudioFileCleanupIfAvailable` 尚未从被测模块导出（ESM 命名导入缺失使整个测试文件加载失败，文件内全部用例报错即为 RED）；实现后若 cleanup 注入的 cancel 路径未删文件，`deleted` 为空断言同样失败。

- [ ] **Step 3: Write minimal implementation**

修改 `apps/mobile/src/adapters/dictation-capture.ts`：

①文件 doc 注释块末尾追加一行说明（保持既有注释原样，在其后加）：

```
 * #70（文件 URI 工作流客户端侧闭合）：cancel 路径的录音机临时文件由本 Adapter 即刻尽最大
 * 力删除（模块侧 DictationPorts.audioCleanup 覆盖 stop 产物的删除时点——CONTEXT.md:339）。
```

②`createDictationCaptureFrom` 之前插入端口与原生清理 Adapter：

```ts
/** 临时音频文件删除端口（真机：expo-file-system；Node 测试注入 fake）。 */
export interface AudioFileCleanupPort {
  deleteAsync(uri: string): Promise<void>;
}

/** 原生文件删除 Adapter（惰性 require；缺包/缺导出 fail closed → undefined）。 */
export function createNativeAudioFileCleanupIfAvailable(): AudioFileCleanupPort | undefined {
  try {
    const fileSystem = require('expo-file-system') as { deleteAsync?: (uri: string) => Promise<void> };
    if (typeof fileSystem.deleteAsync !== 'function') return undefined;
    return { deleteAsync: (uri) => fileSystem.deleteAsync!(uri) };
  } catch {
    return undefined;
  }
}
```

③`createDictationCaptureFrom` 签名与 `cancel()` 替换（`start()`/`stop()` 逐字不变）：

```ts
/** 注入式构造（测试注入 fake；cleanup 可选——缺省不删，行为与 #56 合并版逐字兼容）。 */
export function createDictationCaptureFrom(expoAudio: ExpoAudioLike, cleanup?: AudioFileCleanupPort): DictationCapturePort {
  let recorder: AudioRecorderLike | undefined;
  const deleteBestEffort = (uri: string | null): void => {
    if (cleanup === undefined || typeof uri !== 'string' || uri === '') return;
    void cleanup.deleteAsync(uri).catch(() => undefined); // 删除失败交还平台回收，绝不外泄
  };
  return {
    async start() {
      const permission = await expoAudio.requestRecordingPermissionsAsync();
      if (permission?.granted !== true) return 'denied';
      await expoAudio.setAudioModeAsync?.({ allowsRecording: true, playsInSilentMode: true });
      const instance = new expoAudio.AudioModule.AudioRecorder();
      await instance.prepareToRecordAsync(expoAudio.RecordingPresets?.HIGH_QUALITY);
      instance.record();
      recorder = instance;
      return 'recording';
    },
    async stop(): Promise<DictationAudio | undefined> {
      const instance = recorder;
      recorder = undefined;
      if (instance === undefined) return undefined;
      try {
        await instance.stop();
      } finally {
        instance.release?.();
      }
      // uri 指向的临时文件在 stop() 返回后仍须存活：模块要用它发起 multipart 上载；
      // 删除时点由模块的 audioCleanup 端口在 dropIntent 权威决定（Task 3 / #70）。
      const uri = instance.uri;
      if (typeof uri !== 'string' || uri === '') return undefined;
      return { uri, mimeType: 'audio/mp4', fileName: 'dictation.m4a' };
    },
    async cancel() {
      const instance = recorder;
      recorder = undefined;
      if (instance === undefined) return;
      const cancelledUri = instance.uri;
      try {
        await instance.stop();
      } catch {
        /* 已经停止的录音机：吞掉，路径仍是丢弃 */
      }
      // CONTEXT.md:339「原始音频默认在实时处理后删除」的客户端侧取消路径（#56 延迟项 → #70）：
      // 取消的音频从未进入模块，release 释放录音机对象后由 Adapter 尽最大努力删除临时文件
      // （release 不保证删盘；删除失败交还平台回收，绝不影响回 idle）。
      instance.release?.();
      deleteBestEffort(cancelledUri);
    },
  };
}
```

④`createNativeDictationCaptureIfAvailable` 中 `return createDictationCaptureFrom(expoAudio);` 替换为：

```ts
    return createDictationCaptureFrom(expoAudio, createNativeAudioFileCleanupIfAvailable());
```

（两个函数同文件，无需新增 import。）

⑤`apps/mobile/src/composition.ts`：`createNativeDictationCaptureIfAvailable` import 行扩为：

```ts
import { createNativeAudioFileCleanupIfAvailable, createNativeDictationCaptureIfAvailable } from './adapters/dictation-capture.ts';
```

并把 type import 行 `import type { Dictation } from '@weknora/mobile-core';` 扩为：

```ts
import type { Dictation, DictationAudio } from '@weknora/mobile-core';
```

模块级单例区（`nativeDictationCapture` 声明行后）加：

```ts
/** 临时音频文件清理原生 Adapter 单例（#70 文件 URI 工作流；缺包 fail closed → undefined）。 */
const nativeAudioCleanup = createNativeAudioFileCleanupIfAvailable();
```

`dictationFor` 内 `createDictation({...})` 调用替换为：

```ts
  return cachePut(dictations, deploymentScopeKey(origin, tenantId), () => createDictation({
    capture: nativeDictationCapture,
    transcribe: createMobileVoiceTranscriptionRemote({ origin, request: (input) => activeRuntime.authorizedRequest(input) }),
    newRequestId: createNativeRequestId(),
    // #70：AudioFileCleanupPort（deleteAsync 对象）必须适配为 DictationPorts.audioCleanup
    //（(audio) => Promise 函数）——直接注入对象会在 dropIntent 调用时同步抛 TypeError（Review
    // Focus 3 的失效模式）。仅对 uri 源音频删文件；bytes 源无盘上文件（与 Task 3 模块侧语义
    // 一致）；删除 rejection 由模块 dropIntent 的 .catch 吞掉。
    ...(nativeAudioCleanup === undefined
      ? {}
      : {
          audioCleanup: async (audio: DictationAudio): Promise<void> => {
            if (audio.uri === undefined) return;
            await nativeAudioCleanup.deleteAsync(audio.uri);
          },
        }),
  }));
```

（`voiceRoomFor` 不动——VoiceRoom 的原音频删除语义属 #57。）

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm exec tsx --test apps/mobile/src/adapters/dictation-capture.test.ts apps/mobile/src/app-smoke.test.tsx`
Expected: PASS（0 fail——composition 改动被 app-smoke 全量回归覆盖）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/adapters/dictation-capture.ts apps/mobile/src/adapters/dictation-capture.test.ts apps/mobile/src/composition.ts
git commit -m "feat(mobile): 录音临时文件清理——cancel 即删 + 模块 audioCleanup 原生注入（#70 文件 URI 闭合）"
```

### Task 5: Android Release 工程配置（app.json android 段 + eas.json + 原生依赖 + CNG 约定）

**Files:**
- Modify: `apps/mobile/app.json`（android 段补全）
- Modify: `apps/mobile/.gitignore`（android/ 忽略）
- Modify: `apps/mobile/package.json`（dependencies 增 3 个原生工作流依赖）
- Create: `apps/mobile/eas.json`
- Create: `docs/plans/issue30-sweep/android-evidence/prebuild-android-manifest.txt`（执行期真实命令输出）
- Create: `docs/plans/issue30-sweep/android-evidence/android-release.md`（Release 构建/签名/验收 runbook）
- Test: `apps/mobile/src/android-release-config.test.ts`

**Interfaces:**
- Consumes: 仓库 CNG 约定（根 `.gitignore:97-98`）；`bundledNativeModules.json` 权威版本（`expo-audio ~55.0.18`、`expo-network ~55.0.18`、`expo-file-system ~55.0.26`，实读核实）；本计划差异记录第 5/6 条的 prebuild 探测事实。
- Produces: 可从零生成的 Android 原生工程配置（Task 7 矩阵引用本任务的证据文件）；Task 1/4 的原生模块（expo-notifications 已在依赖、expo-file-system 新声明）进入 Release 构建。

- [ ] **Step 1: Write the failing test**

创建 `apps/mobile/src/android-release-config.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

// String-based path math avoids the DOM-lib `URL` vs `node:url` `URL` type clash
// that Expo's tsconfig base (lib includes DOM) would otherwise raise（app-smoke 同注释）。
const here = dirname(fileURLToPath(import.meta.url));
const workspaceRoot = resolve(here, '..', '..', '..');

test('app.json declares the Android release surface: package, scheme, versionCode and runtime permissions', () => {
  const config = JSON.parse(readFileSync(resolve(here, '..', 'app.json'), 'utf8')) as {
    expo: { scheme?: string; android?: { package?: string; versionCode?: number; permissions?: string[] } };
  };
  assert.equal(config.expo.android?.package, 'com.weknora.mobile', 'android applicationId 显式声明（验收标准 1 的包身份）');
  assert.equal(config.expo.scheme, 'weknora', 'weknora:// 深链 scheme——auth-return 与通知深链的系统返回通道');
  assert.equal(typeof config.expo.android?.versionCode, 'number', 'versionCode 显式声明（Release 递增基线，prebuild 写入 build.gradle）');
  const permissions = config.expo.android?.permissions ?? [];
  for (const required of ['INTERNET', 'VIBRATE', 'RECORD_AUDIO', 'POST_NOTIFICATIONS']) {
    assert.equal(permissions.includes(required), true, `android.permissions 必须显式包含 ${required}——库 manifest 在 Gradle 期才合并，显式声明才进 prebuild 产物（差异记录 6）`);
  }
});

test('the native workflow prerequisites are declared dependencies (mic / network / file cleanup)', () => {
  const pkg = JSON.parse(readFileSync(resolve(here, '..', 'package.json'), 'utf8')) as { dependencies?: Record<string, string> };
  const expected: Array<readonly [string, string]> = [
    ['expo-audio', '~55.0.18'],
    ['expo-network', '~55.0.18'],
    ['expo-file-system', '~55.0.26'],
  ];
  for (const [name, version] of expected) {
    assert.equal(pkg.dependencies?.[name], version, `${name}@${version} 必须声明（版本源 node_modules/expo/bundledNativeModules.json，差异记录 7）`);
  }
});

test('eas.json provides an installable release path (preview apk) and the store path (production aab)', () => {
  const eas = JSON.parse(readFileSync(resolve(here, '..', 'eas.json'), 'utf8')) as {
    build?: Record<string, { distribution?: string; android?: { buildType?: string } }>;
  };
  assert.equal(eas.build?.preview?.android?.buildType, 'apk', '验收路径：preview 产直接可安装的 Release APK');
  assert.equal(eas.build?.preview?.distribution, 'internal', 'preview 走内部分发（真机安装验收）');
  assert.equal(eas.build?.production?.android?.buildType, 'app-bundle', '商店路径：production 产 AAB');
});

test('the generated android project stays out of version control (CNG, same as ios/)', () => {
  // --quiet（不带 -v）：负模式匹配（根 !apps/mobile/**）不算被忽略 → exit 1；
  // git 2.54 实测 -v 模式对负模式匹配也返回 0，会使 RED 阶段本测试假绿。
  const result = spawnSync('git', ['check-ignore', '--quiet', 'apps/mobile/android/build.gradle'], {
    cwd: workspaceRoot,
    encoding: 'utf8',
  });
  assert.equal(result.status, 0, `apps/mobile/android/ 必须被忽略（根 !apps/mobile/** 再包含之下需要更深规则）；exit=${result.status}`);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm exec tsx --test apps/mobile/src/android-release-config.test.ts`
Expected: FAIL —— versionCode 非显式数字、android.permissions 缺失、三个原生依赖缺失、eas.json 不存在（JSON parse 抛 ENOENT）；第 4 个测试 `git check-ignore --quiet` 现状 exit 1（android/ 未被忽略，`!apps/mobile/**` 负模式在生效——计划作者 git 2.54.0 实测）。

- [ ] **Step 3: Write minimal implementation**

①`apps/mobile/app.json` 的 `expo.android` 段替换为（其余逐字不动）：

```json
    "android": {
      "package": "com.weknora.mobile",
      "versionCode": 1,
      "permissions": [
        "INTERNET",
        "VIBRATE",
        "RECORD_AUDIO",
        "POST_NOTIFICATIONS"
      ]
    },
```

②`apps/mobile/.gitignore` 追加：

```
# CNG（expo prebuild）流程：android/ 目录是生成产物，勿提交（与根 .gitignore 的 ios/ 同一约定）
android/
```

③`apps/mobile/package.json` 的 `dependencies` 按字母序插入三行（`expo` 之后、`expo-auth-session` 之前插 expo-audio；`expo-crypto` 之后插 expo-file-system；`expo-linking` 之后、`expo-notifications` 之前插 expo-network）：

```json
    "expo-audio": "~55.0.18",
    "expo-file-system": "~55.0.26",
    "expo-network": "~55.0.18",
```

然后在仓库根执行 `pnpm install`（需要网络；若环境不可达，如实记录到提交说明——Node 测试链不依赖这三个包，全部测试仍绿，但 Release 构建前置缺失需在 runbook 如实标注）。

④创建 `apps/mobile/eas.json`：

```json
{
  "build": {
    "development": {
      "developmentClient": true,
      "distribution": "internal"
    },
    "preview": {
      "distribution": "internal",
      "android": {
        "buildType": "apk"
      }
    },
    "production": {
      "android": {
        "buildType": "app-bundle"
      }
    }
  }
}
```

⑤创建 `docs/plans/issue30-sweep/android-evidence/android-release.md`：

```markdown
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
```

- [ ] **Step 4: Run test to verify it passes + 采集 prebuild 证据**

Run: `pnpm exec tsx --test apps/mobile/src/android-release-config.test.ts`
Expected: PASS（4 tests, 0 fail）。

随后实跑 prebuild 并把生成物断言输出入库（真实证据，不伪造）：

```bash
mkdir -p docs/plans/issue30-sweep/android-evidence
cd apps/mobile && npx expo prebuild -p android --no-install && grep -nE 'uses-permission|android:scheme|launchMode' android/app/src/main/AndroidManifest.xml > ../docs/plans/issue30-sweep/android-evidence/prebuild-android-manifest.txt && rm -rf android && cd ..
```

Expected: prebuild exit 0；manifest 文件含 `android.permission.POST_NOTIFICATIONS`、`android.permission.RECORD_AUDIO`、`android:scheme="weknora"`、`launchMode="singleTask"` 四类行（计划作者 2026-09-26 探测实跑已验证这四类行均出现）。若某一类行缺失：**停下升级**（说明 Expo 模板行为漂移），不得改证据文件凑数。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/app.json apps/mobile/.gitignore apps/mobile/package.json apps/mobile/eas.json apps/mobile/src/android-release-config.test.ts docs/plans/issue30-sweep/android-evidence/prebuild-android-manifest.txt docs/plans/issue30-sweep/android-evidence/android-release.md pnpm-lock.yaml
git commit -m "feat(mobile): Android Release 工程配置——android 段/eas.json/原生依赖/CNG 约定 + prebuild 证据（#70）"
```

（若 `pnpm-lock.yaml` 因环境不可达未变化，git add 该路径不产生暂存内容，提交说明中如实注明。）

### Task 6: AppState 生命周期 Adapter 测试补位（后台限制 seam）

**Files:**
- Test: `apps/mobile/src/adapters/app-state.test.ts`（新建；被测 `adapters/app-state.ts` 零改动）

**Interfaces:**
- Consumes: `createNativeAppStateLifecycle()`（app-state.ts，subscribe 返回退订函数；`state === 'active' ? 'active' : 'background'` 映射；任何不可用路径 no-op）；node:module `registerHooks` resolve 注入先例（`voice-dictation-integration-smoke.test.ts:12-45`）。
- Produces: 后台限制工作流的 Adapter 层本地证据（Task 7 矩阵引用本文件）。

- [ ] **Step 1: Write the failing test（本任务为纯测试补位：先写测试即完整交付物，无需实现步骤）**

创建 `apps/mobile/src/adapters/app-state.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import * as nodeModule from 'node:module';
import { after, test } from 'node:test';

// react-native 在 Node 无真身：resolve hook 注入可控 stub（先例：voice-dictation-integration-smoke.test.ts）。
// AppState 由 getter 提供，行为经 globalThis.__RN_APPSTATE_MODE 在各测试内切换（无需重新 import）。
type ResolveNext = (specifier: string, context: unknown) => unknown;
type ResolveHook = (specifier: string, context: unknown, nextResolve: ResolveNext) => unknown;
const moduleWithHooks = nodeModule as typeof nodeModule & {
  registerHooks?: (hooks: { resolve: ResolveHook }) => void;
};

const stubDir = mkdtempSync(join(tmpdir(), 'weknora-appstate-stub-'));
const stubPath = join(stubDir, 'react-native.cjs');
writeFileSync(
  stubPath,
  `
const mode = () => globalThis.__RN_APPSTATE_MODE ?? 'ok';
module.exports = {
  get AppState() {
    if (mode() === 'throw') throw new Error('AppState unavailable');
    if (mode() === 'absent') return undefined;
    return {
      addEventListener(event, handler) {
        if (mode() === 'missing-listener') return undefined;
        globalThis.__RN_APPSTATE_LAST = { event, handler };
        return { remove() { globalThis.__RN_APPSTATE_REMOVED = true; } };
      },
    };
  },
};
`,
);
if (moduleWithHooks.registerHooks) {
  moduleWithHooks.registerHooks({
    resolve: (specifier, context, nextResolve) =>
      specifier === 'react-native'
        ? { shortCircuit: true, url: pathToFileURL(stubPath).href }
        : nextResolve(specifier, context),
  });
}
after(() => {
  rmSync(stubDir, { recursive: true, force: true });
});

const loadMod = () => import('./app-state.ts');

test('foreground/background AppState changes map onto the two-state lifecycle contract (#70 后台限制)', async () => {
  const { createNativeAppStateLifecycle } = await loadMod();
  const seen: Array<'active' | 'background'> = [];
  const stop = createNativeAppStateLifecycle().subscribe((state) => seen.push(state));
  const last = (globalThis as { __RN_APPSTATE_LAST?: { event: string; handler: (state: string) => void } }).__RN_APPSTATE_LAST;
  assert.ok(last, 'subscribe 必须注册 AppState change 监听');
  assert.equal(last!.event, 'change');
  last!.handler('active');
  last!.handler('background');
  last!.handler('unknown'); // 未知状态保守视为 background（fail closed：不误触发前台同步）
  assert.deepEqual(seen, ['active', 'background', 'background']);
  (globalThis as { __RN_APPSTATE_REMOVED?: boolean }).__RN_APPSTATE_REMOVED = false;
  stop();
  assert.equal((globalThis as { __RN_APPSTATE_REMOVED?: boolean }).__RN_APPSTATE_REMOVED, true, '退订必须移除原生监听');
});

test('an unusable AppState degrades to a no-op subscription instead of crashing startup (#70 fail closed)', async () => {
  const { createNativeAppStateLifecycle } = await loadMod();
  for (const mode of ['absent', 'missing-listener', 'throw']) {
    (globalThis as { __RN_APPSTATE_MODE?: string }).__RN_APPSTATE_MODE = mode;
    const seen: string[] = [];
    const stop = createNativeAppStateLifecycle().subscribe((state) => seen.push(state));
    stop(); // no-op 退订必须可安全调用
    assert.deepEqual(seen, [], `${mode} 模式下不得产生事件`);
  }
  delete (globalThis as { __RN_APPSTATE_MODE?: string }).__RN_APPSTATE_MODE;
});
```

- [ ] **Step 2: Run test to verify it passes（纯测试任务：预期直接 PASS；若 FAIL 说明 resolve hook/stub 与真实 adapter 行为不符，修 stub 而非改 adapter）**

Run: `pnpm exec tsx --test apps/mobile/src/adapters/app-state.test.ts`
Expected: PASS（2 tests, 0 fail——`app-state.ts` 的映射与 no-op 语义按源码 :6-21 本就正确，本任务是把它们钉进回归）。

- [ ] **Step 3: Commit**

```bash
git add apps/mobile/src/adapters/app-state.test.ts
git commit -m "test(mobile): AppState 生命周期 Adapter 两态映射与 no-op 降级回归（#70 后台限制）"
```

### Task 7: 平台一致性扫描 + Android 七工作流验收矩阵

**Files:**
- Create: `apps/mobile/src/android-acceptance.ts`（矩阵数据）
- Test: `apps/mobile/src/android-parity.test.ts`

**Interfaces:**
- Consumes: Task 1/4/6 的证据文件路径（矩阵引用）；既有集成冒烟文件（全部已存在于 HEAD，见矩阵）；既有守卫先例（app-smoke 的源级 readFileSync 断言）。
- Produces: `ANDROID_WORKFLOWS`、`AndroidWorkflow`、`AndroidWorkflowAcceptance`、`ANDROID_ACCEPTANCE_MATRIX`——#71（发布证据矩阵）可直接消费的结构化验收台账；`android-parity.test.ts` 双守卫（平台 API 扫描 + 矩阵完整性）。

- [ ] **Step 1: Write the failing test**

创建 `apps/mobile/src/android-parity.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { ANDROID_ACCEPTANCE_MATRIX, ANDROID_WORKFLOWS } from './android-acceptance.ts';

const here = dirname(fileURLToPath(import.meta.url));
const workspaceRoot = resolve(here, '..', '..', '..');

function walkSources(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) out.push(...walkSources(full));
    else if (/\.(ts|tsx)$/.test(entry.name) && !/\.test\.(ts|tsx)$/.test(entry.name)) out.push(full);
  }
  return out;
}

test('platform APIs stay inside src/adapters (验收标准 2：平台差异仅在 Adapter)', () => {
  const adaptersDir = join(here, 'adapters');
  const offenders: string[] = [];
  let adapterHits = 0;
  for (const file of walkSources(here)) {
    if (!/require\(['"]react-native['"]\)/.test(readFileSync(file, 'utf8'))) continue;
    if (file.startsWith(adaptersDir)) adapterHits += 1;
    else offenders.push(relative(here, file));
  }
  assert.deepEqual(offenders, [], 'react-native 平台 API 只允许出现在 src/adapters/（module-seams §10）');
  assert.ok(adapterHits >= 1, 'sanity：扫描必须真的看到 adapters/ 中的合法平台接线（防扫描器失效静默通过）');
});

test('screens and routes never import wire clients or shared contracts (module-seams §10)', () => {
  const offenders: string[] = [];
  for (const dir of ['screens', 'app']) {
    for (const file of walkSources(join(here, dir))) {
      if (/@weknora\/(api-client|contracts)/.test(readFileSync(file, 'utf8'))) offenders.push(relative(here, file));
    }
  }
  assert.deepEqual(offenders, [], 'Screen/路由禁止直连 wire 客户端或共享 contracts');
});

test('the Android acceptance matrix covers exactly the seven core workflows with real evidence paths (验收标准 3)', () => {
  assert.deepEqual(
    ANDROID_ACCEPTANCE_MATRIX.map((row) => row.workflow).sort(),
    [...ANDROID_WORKFLOWS].sort(),
    'What to build 的七项核心工作流一项不缺、一项不多',
  );
  for (const row of ANDROID_ACCEPTANCE_MATRIX) {
    assert.ok(
      row.adapterSeam.startsWith('apps/mobile/src/adapters/') || row.adapterSeam.startsWith('apps/mobile/src/app/'),
      `${row.workflow} 的平台 seam 必须落在 Adapter 层`,
    );
    assert.equal(existsSync(join(workspaceRoot, row.adapterSeam)), true, `${row.workflow} 的 seam 文件必须真实存在：${row.adapterSeam}`);
    assert.ok(row.localEvidence.length >= 1, `${row.workflow} 至少引用一条本地证据`);
    for (const evidence of row.localEvidence) {
      assert.equal(
        existsSync(join(workspaceRoot, evidence)),
        true,
        `${row.workflow} 引用的证据必须真实存在（引用计划产物冒充已验证即测试失败）：${evidence}`,
      );
    }
    assert.equal(row.residual.kind, 'blocked-env', `${row.workflow} 的真机残余恒为 blocked-env——绝不记为通过（验收标准 3）`);
    assert.ok(row.residual.reason.trim() !== '' && row.residual.unblock.trim() !== '', `${row.workflow} 必须写明阻塞原因与解锁条件`);
  }
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm exec tsx --test apps/mobile/src/android-parity.test.ts`
Expected: FAIL —— `Cannot find module './android-acceptance.ts'`。

- [ ] **Step 3: Write minimal implementation**

创建 `apps/mobile/src/android-acceptance.ts`：

```ts
/**
 * Android 安装包核心工作流验收矩阵（Issue #70）。把验收标准 2「与 iOS 的领域行为一致，平台
 * 差异仅在 Adapter」与验收标准 3「端到端行为通过最高稳定 Interface 验证」编码为可执行守卫：
 * 每个工作流映射到双平台共享的领域 Interface、平台 Adapter seam、仓库内真实存在的本地证据
 * （由 android-parity.test.ts existsSync 强制），以及只在真机/外部凭据环境可验证的残余项
 * （kind 恒 'blocked-env'——绝不计为通过，解锁条件必须显式）。#71 发布证据矩阵直接消费本表。
 */
export const ANDROID_WORKFLOWS = [
  'system-back',
  'background-limits',
  'notifications',
  'file-uri',
  'keystore',
  'microphone',
  'weak-network',
] as const;

export type AndroidWorkflow = (typeof ANDROID_WORKFLOWS)[number];

export interface AndroidWorkflowAcceptance {
  workflow: AndroidWorkflow;
  /** 双平台共享的领域 Interface（packages/mobile-core 深模块 / 导航面）——平台差异绝不在此层。 */
  domainInterface: string;
  /** 平台 Adapter seam（apps/mobile/src/adapters/ 或导航 adapter）——平台差异只允许在这里。 */
  adapterSeam: string;
  /** 仓库内真实存在的本地证据资产（测试/集成冒烟/源级守卫）。 */
  localEvidence: string[];
  /** 真机/外部凭据残余（blocked-env）。 */
  residual: { kind: 'blocked-env'; reason: string; unblock: string };
}

export const ANDROID_ACCEPTANCE_MATRIX: AndroidWorkflowAcceptance[] = [
  {
    workflow: 'system-back',
    domainInterface: 'expo-router Stack 导航 + weknora://oidc 回调（deliverOidcReturn 单次投递 + 已登记回调白名单）',
    adapterSeam: 'apps/mobile/src/app/auth-return.tsx',
    localEvidence: ['apps/mobile/src/app-smoke.test.tsx'],
    residual: {
      kind: 'blocked-env',
      reason: '返回键/手势逐级回退与「返回不复活已登出内容」需要 Android 真机系统行为',
      unblock: 'Release APK 安装于 Android 真机执行返回键/手势路径（android-release.md 验收清单）',
    },
  },
  {
    workflow: 'background-limits',
    domainInterface: 'createForegroundSyncLoop（单飞合并 + 失败包含 + scope 每事件重解析）→ NotificationInbox.page() 权威重投影',
    adapterSeam: 'apps/mobile/src/adapters/app-state.ts',
    localEvidence: ['apps/mobile/src/foreground-sync.test.ts', 'apps/mobile/src/adapters/app-state.test.ts'],
    residual: {
      kind: 'blocked-env',
      reason: 'Doze/电池优化下 AppState 事件与网络可用性的真实组合行为需要真机',
      unblock: 'Android 真机后台/前台往返与省电模式验收轮',
    },
  },
  {
    workflow: 'notifications',
    domainInterface: 'createDeviceRegistry（两步注册/409 重取/接管）+ createNotificationInbox（分页合并/幂等已读/安全深链）',
    adapterSeam: 'apps/mobile/src/adapters/notification-permission.ts',
    localEvidence: [
      'apps/mobile/src/adapters/notification-permission.test.ts',
      'apps/mobile/src/blind-push-integration-smoke.test.ts',
      'apps/mobile/src/device-inbox-integration-smoke.test.ts',
    ],
    residual: {
      kind: 'blocked-env',
      reason: '真实 FCM 通道送达需要推送凭据部署（本地 #67 形态为 disabled provider）；POST_NOTIFICATIONS 系统弹窗需要真机',
      unblock: '有 FCM 凭据的部署 + Android 13+ 真机（android-release.md 验收清单「通知」行）',
    },
  },
  {
    workflow: 'file-uri',
    domainInterface: 'Dictation（audio uri → multipart 上载 → dropIntent 即删原始音频）+ TaskMaterial BlobFetchPort（免凭据签名链接）',
    adapterSeam: 'apps/mobile/src/adapters/dictation-capture.ts',
    localEvidence: [
      'apps/mobile/src/adapters/dictation-capture.test.ts',
      'apps/mobile/src/voice-dictation-integration-smoke.test.ts',
      'apps/mobile/src/material-integration-smoke.test.ts',
    ],
    residual: {
      kind: 'blocked-env',
      reason: '真机上 file:// URI 的 OS 级边界（FileUriExposed）与临时文件实际落盘/删除需要真机',
      unblock: 'Android 真机录音→转写→确认→文件系统核验轮',
    },
  },
  {
    workflow: 'keystore',
    domainInterface: 'ScopedVault（scope 隔离/rotate/revoke）+ CredentialStore/DeploymentRegistry/IntentLog（SecureStore 端口族）',
    adapterSeam: 'apps/mobile/src/adapters/secure-store.ts',
    localEvidence: [
      'apps/mobile/src/adapters/vault-adapters.test.ts',
      'apps/mobile/src/adapters/intent-log.test.ts',
      'apps/mobile/src/adapters/deployment-registry.test.ts',
      'apps/mobile/src/offline-vault-integration-smoke.test.ts',
    ],
    residual: {
      kind: 'blocked-env',
      reason: 'Android Keystore 硬件 backed 密钥与真机 SecureStore 行为（含系统备份排除的实际生效）需要真机',
      unblock: 'Android 真机登录→重启→登出→密钥不可用核验轮',
    },
  },
  {
    workflow: 'microphone',
    domainInterface: 'Dictation 捕获生命周期（begin/finish/cancel + denied fail closed）',
    adapterSeam: 'apps/mobile/src/adapters/dictation-capture.ts',
    localEvidence: [
      'apps/mobile/src/adapters/dictation-capture.test.ts',
      'apps/mobile/src/voice-dictation-integration-smoke.test.ts',
    ],
    residual: {
      kind: 'blocked-env',
      reason: 'RECORD_AUDIO 系统拒权弹窗与真实录音设备行为需要真机（spec：OS audio 属 true external）',
      unblock: 'Android 真机麦克风拒权/授权两路径验收轮（#69/#70 共同口径）',
    },
  },
  {
    workflow: 'weak-network',
    domainInterface: 'createOfflineGate（探测失败=离线 fail closed）+ TaskHandle resync（有界自动重同步上限 2）+ 快照降级渲染',
    adapterSeam: 'apps/mobile/src/adapters/network-status.ts',
    localEvidence: [
      'apps/mobile/src/adapters/network-status.test.ts',
      'apps/mobile/src/offline-vault-integration-smoke.test.ts',
      'apps/mobile/src/task-detail-integration-smoke.test.ts',
    ],
    residual: {
      kind: 'blocked-env',
      reason: '真实丢包/慢速/网络切换下的 SSE 断线与恢复时序需要真机弱网环境',
      unblock: 'Android 真机飞行模式/弱网代理验收轮',
    },
  },
];
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm exec tsx --test apps/mobile/src/android-parity.test.ts`
Expected: PASS（3 tests, 0 fail）。若某矩阵证据路径不存在：停下核对该任务的交付是否真的落库——**不得删除矩阵行让测试变绿**。

- [ ] **Step 5: 全量回归（本计划全部受影响面）**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck && pnpm exec tsx --test packages/mobile-core/src/voice/dictation.test.ts`
Expected: mobile 全量 0 fail（基线 215 pass / 11 skip——skip 为既有 opt-in 集成用例，本计划新增测试全 pass）；typecheck exit 0；dictation 0 fail。

- [ ] **Step 6: Commit**

```bash
git add apps/mobile/src/android-acceptance.ts apps/mobile/src/android-parity.test.ts
git commit -m "test(mobile): 平台一致性源级扫描 + Android 七工作流验收矩阵（blocked-env 如实编码，#70）"
```

## 计划级验证命令（worktree 根执行）

```bash
pnpm exec tsx --test apps/mobile/src/adapters/notification-permission.test.ts apps/mobile/src/adapters/dictation-capture.test.ts apps/mobile/src/adapters/app-state.test.ts apps/mobile/src/android-release-config.test.ts apps/mobile/src/android-parity.test.ts packages/mobile-core/src/voice/dictation.test.ts && pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx && pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck
```

（前两段为本计划新增/受影响测试的定向验证；后两段为 apps/mobile 全量回归 + 严格类型检查。基线：计划作者 2026-09-26 实跑 `pnpm --filter @weknora/mobile test` → 215 pass / 0 fail / 11 skip、`pnpm --filter @weknora/mobile typecheck` → exit 0、`pnpm exec tsx --test packages/mobile-core/src/voice/dictation.test.ts` → 0 fail、`pnpm exec tsx --test apps/mobile/src/adapters/dictation-capture.test.ts apps/mobile/src/adapters/network-status.test.ts` → 8 pass。）

## 自我审查记录（writing-plans 四项检查）

1. **Spec 覆盖**：验收标准 1「可安装 Release 包」→ Task 5（prebuild 生成验证 + eas.json Release profile + 签名 runbook + 原生依赖声明）+ blocked-env B1 如实声明；验收标准 2「平台差异仅在 Adapter」→ Task 7 平台 API 源级扫描 + 矩阵 adapterSeam 层级断言 + 本计划全部平台改动收敛在 adapters/（Task 1/4）；验收标准 3「最高稳定 Interface、不冒充」→ 矩阵 localEvidence 全部指向既有真实集成冒烟/测试（existsSync 强制）+ residual.kind 恒 blocked-env + Review Focus 5 的反装饰测试。What to build 七工作流逐一映射（Task 7 矩阵：system-back/background-limits/notifications/file-uri/keystore/microphone/weak-network，与 Issue 正文枚举一一对应——「文件 URI、Keystore」即 file-uri/keystore 行）。CONTEXT.md:339 原音频删除 → Task 3/4。#56 延迟项显式闭合并注记。七工作流之外的 Issue 正文项（「真实 Android Release 包完成核心流程」）由既有授权面（#32–#46/#56/#66/#67 集成冒烟）覆盖，本计划不重复建设。
2. **占位符扫描**：全文无 TBD/TODO/「适当处理」/「类似 Task N」；Task 3 的四个测试均为自包含内联 stub（`DictationTranscriptionPort` 内联对象形态已对照 dictation.ts:53-55 核实），不引用本计划未定义的符号；所有代码块完整可编译（实现块为整函数/整段替换，测试块为整文件或明确锚点的整测试追加）。
3. **类型/签名一致性**：`NotificationPermissionPort.ensure(): Promise<'granted'|'denied'|'unavailable'>` 在 Task 1 定义、Task 2 消费一致；`registerActiveDeviceIfPossible` 新返回值联合成员 `'permission-denied'` 在 Task 2 实现/测试逐字一致；`AudioFileCleanupPort.deleteAsync(uri: string): Promise<void>` 在 Task 4 定义、与 Task 3 `audioCleanup?: (audio: DictationAudio) => Promise<void>` 的组合根注入（条件展开缺省省略）一致；矩阵 `AndroidWorkflowAcceptance` 字段与 Task 7 测试读取的字段逐字一致；`ANDROID_WORKFLOWS`/`ANDROID_ACCEPTANCE_MATRIX` 导出名在数据文件与测试间一致。
4. **Review Focus 落实**：五类失效模式各有归属测试——假注册/重试风暴（Task 1 已授权零弹窗 + Task 2 denied 短路不触 token）、临时文件滞留（Task 4 cancel 删除 + Task 3 dropIntent 恰好一次）、清理失败外泄（Task 3/4 各一条 swallowed-rejection 测试）、平台 API 泄漏（Task 7 扫描 + sanity）、矩阵装饰化（Task 7 existsSync + blocked-env 强制）。无空行、无未落测试的 Review Focus 条目。

## 消费与产出（Consumes / Produces）

**Consumes（前序批次接口，全部已在 HEAD 亲验）：** `createDictation`（#56）、`createDeviceRegistry`/`createNotificationInbox`（#41）、`createScopedVault`/`createVaultTaskProjectionStore`/`createOfflineGate`（#32/#40）、`createNativeSecureDeploymentRegistry`/`listDeployments`/`switchDeployment`（#66）、`createForegroundSyncLoop`（#67）、`registerActiveDeviceIfPossible`（#41，composition.ts:249）、`disallowedDeploymentHost`（runtime-integration-smoke.ts:118）、既有集成冒烟文件集合。

**Produces（供 #71 与后续真机验收消费）：** `ANDROID_ACCEPTANCE_MATRIX`/`ANDROID_WORKFLOWS`/`AndroidWorkflowAcceptance`（结构化验收台账，apps/mobile 内部模块，#71 证据矩阵按表核对）；`NotificationPermissionPort`（+`createNativeNotificationPermissionIfAvailable`）；`AudioFileCleanupPort`（+`createNativeAudioFileCleanupIfAvailable`）；`'permission-denied'` 注册结果；`eas.json` Release profile 与 `android-release.md` runbook（真机验收轮的操作权威）；`prebuild-android-manifest.txt`（生成物断言证据）。

## 边界（本计划明确不做）

- Go 服务端零改动；不触碰任何迁移文件（波级指引 1：无迁移装载需求的计划不顺手去重）。
- 共享 worktree 存在并行批次的未提交改动（Go 测试/迁移文件/其他 docs）——所有提交步骤均为**显式路径 `git add`**，实现者不得改用 `git add -A`/`git add .`，避免卷入并行改动。
- 不新增 Android config plugin（差异记录 6：prebuild 生成物无已证实的模板缺陷需修；与 iOS 的 ios-xcode27 性质不同，那里是实测 SIGTRAP 后的修复）。
- 不触碰 `voiceRoomFor`/VoiceRoom 模块与 `packages/mobile-core/src/voice-room/`（#57 并行批次范围）；不触碰 TaskMaterial/OfflineGate/Devices 的既有模块语义（验收矩阵只引用不修改）。
- 不做模板默认权限（SYSTEM_ALERT_WINDOW/外部存储）的剔除承诺（差异记录 5：blockedPermissions 探测无效）。
- miniprogram 既有 71 个 typecheck 错误豁免（波级指引 4，本计划不涉及 miniprogram）。
