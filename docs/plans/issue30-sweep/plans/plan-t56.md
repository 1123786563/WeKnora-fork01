# T26：可编辑语音转写草稿（Issue #56）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 移动端 New 目标输入支持「录音 → 服务端转写 → 可编辑转写草稿 → 用户确认后并入目标文本」，并使「取消录音不取消 Task、转写须确认后才进入提交面、拒权/取消/迟到转写不破坏手打文字」全部在最高稳定 Interface（mobile-core Dictation 模块 + 真实 wire 的 api-client 适配器 + 真实 Runtime 授权通道）上可验证。

**Architecture:** 服务端转写代理已完整存在且零改动（`internal/handler/mobile_voice.go:445` `TranscribeAudio`，路由 `POST /api/v1/mobile/voice/transcriptions`，`internal/router/routes_workbench.go:188-191` + `internal/router/router.go:380` 挂在 `/api/v1` 下；幂等 request_id 重放、预算 hold、结算、16MiB/120s 上限全部齐备，18 个既有 Go 测试本计划作者已实跑全 PASS——最终审查 t56 勘误：实跑 `-v` 顶层测试函数计数为 18，初稿「19」为元数据笔误，不影响任何行为或结论）。本计划纯客户端：`packages/mobile-core/src/voice/` 新建 **Dictation 深模块**（module-seams §8.1「Voice Room 拥有……转写草稿」的听写子面；状态机 idle→recording→transcribing→review→idle/denied/failed，代次守卫丢弃迟到转写，失败后同 requestId 重试对齐服务端幂等重放）；`packages/api-client` 新增 `createMobileVoiceTranscriptionRemote`（multipart FormData 体经 `MobileRuntime.authorizedRequest` 原样透传——`packages/api-client/src/transport/json.ts:136` 对 FormData body 直传，已核实）；apps/mobile 在 NewTaskScreen 挂听写 UI、组合根按 deployment scope 记忆化装配、原生捕获 Adapter 惰性 require `expo-audio` fail closed（真机验收属 #69/#70，麦克风属 spec 定义的 true external——Port + scripted Adapter）。转写确认文字经 `applyConfirmedDictation` **追加**（不覆盖）进目标草稿；提交仍只走 #36 既有 Submit 通道——模块绝无提交能力（AC2 的结构性保障）。

**Tech Stack:** TypeScript（`packages/mobile-core`、`packages/api-client`、`apps/mobile` Expo RN 55）、node:test + tsx（TS 测试运行器，与 `task-office.test.ts` 一致）。**Go 零改动**（服务端面以既有 18 个测试复跑为证据）。所有测试命令在 worktree 根（`.worktrees/issue30-sweep`）执行；前置 `pnpm install` 已就绪。本计划作者已实跑以下基线（2026-09-24，当前 HEAD `fb5f6653a`）：`go test ./internal/handler/ -run 'TestVoice|TestTranscription|TestPriceVersion' -count=1` → **ok**（18 个测试函数：`TestVoiceSessionHappyPathTokenNeverLogged` 等，见 `internal/handler/mobile_voice_test.go:313-957`）；`pnpm exec tsx --test apps/mobile/src/new-task-view.test.ts apps/mobile/src/new-task-drafts.test.ts` → **0 fail**；`pnpm exec tsx --test packages/mobile-core/src/device/device-registry.test.ts packages/api-client/src/mobile/devices.test.ts` → **0 fail**；`pnpm --filter @weknora/mobile typecheck` → **通过（无输出）**；`pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx` → **54 pass / 0 fail**；`pnpm --filter @weknora/mobile test` → **149 pass / 0 fail / 5 skip**（skip 为既有 opt-in 集成用例）。Node `v26.7.0`（全局 `FormData`/`Blob` 可用，已用 node 一行脚本核实 bytes 形态 append 返回 Blob、uri 对象形态在 Node 会字符串化——故 uri 形态只在 RN 运行时有效，Node 测试只验证 bytes 形态）。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-56.md`（验收标准原文见「Global Constraints」末尾；Blocked by #36 已在当前 HEAD 合并）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（User Story 68「voice dictation to produce an editable draft」、Implementation Decisions「Voice Room owns real-time audio and transcription. Confirmed text enters Task Office; voice never grants approval.」「REST submits commands and loads authoritative Snapshots. … WebSocket or WebRTC is reserved for real-time voice.」「Mobile core does not depend on React Native, DOM or concrete transport.」、Testing Decisions「Tests target observable behavior at the highest stable Interface.」「True external dependencies such as APNs, FCM, WebRTC, system audio and system share use mock or scripted Adapters at the Port and real-device acceptance separately.」）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§8 Voice Room Module——所有权「Voice Room 拥有实时语音 session、microphone permission、音频状态、转写草稿和断线结束语义。它不拥有 Task 权限、审批或时间线」、§8.3 seam（Audio Device Port：iOS/Android native Adapter、test Adapter）、§3 依赖方向、§10 App Shell 禁止事项、§13 Interface 测试面「Voice Room：permission、断线、转写确认、原音频删除、高风险审批拒绝」——本计划覆盖其中听写相关的 permission/转写确认子面，断线/原音频删除/高风险审批拒绝属 #57 实时 Voice Room）
- ADR：`docs/adr/0012-mobile-business-logic-lives-behind-deep-modules.md`（移动业务逻辑放在深 Module 后）、`docs/adr/0006-mobile-transport-by-semantics.md`（转写上传走授权 REST，非实时通道）、`docs/adr/0007-registered-devices-and-encrypted-cache.md`（原生 Adapter 惰性解析 fail closed 同型先例）
- 领域术语：`CONTEXT.md:339`「语音交互记录（Voice Interaction Record）……原始音频默认在实时处理后删除」
- Parent：Issue #30；下游 Blocked by 本 Issue：#57（实时 Voice Room）、#69/#70（真机 Release 验收）

## Global Constraints

以下为批准 Spec / ADR / Issue 的项目级约束，逐字引用，所有任务隐含遵守：

- 「68. As a member, I want voice dictation to produce an editable draft, so that transcription errors can be corrected before submission.」（mobile-ai-office-design.md · User Stories）——本计划的全部价值句：转写产出**可编辑**草稿，不是最终文本。
- 「Voice Room owns real-time audio and transcription. Confirmed text enters Task Office; voice never grants approval.」（同上 · Implementation Decisions）——确认后的文字进入 Task Office 的输入面（#36 的目标草稿/Submit 通道）；听写模块**没有**任何提交/审批能力（AC2 结构性落实）。
- 「REST submits commands and loads authoritative Snapshots. Cursored SSE carries durable Task/Run events. WebSocket or WebRTC is reserved for real-time voice. Push is a synchronization hint.」（同上；ADR-0006 同义）——转写是一次 multipart REST 上载（`POST /api/v1/mobile/voice/transcriptions`），**不新增任何 WebSocket/WebRTC**（那是 #57 实时 Voice Room 的通道）。
- 「Mobile core does not depend on React Native, DOM or concrete transport. Remote and native details are injected as Adapters.」（同上）——`packages/mobile-core/src/voice/dictation.ts` 不出现 FormData/Blob/require；wire 细节在 `packages/api-client/src/mobile/voice.ts`，原生麦克风在 `apps/mobile/src/adapters/dictation-capture.ts`。
- 「True external dependencies such as APNs, FCM, WebRTC, system audio and system share use mock or scripted Adapters at the Port and real-device acceptance separately.」（同上 · Testing Decisions）——麦克风（system audio）以 scripted/test Adapter 进 Interface 测试；真机麦克风验收属 #69/#70，本计划不伪造真机结论。
- 「Tests target observable behavior at the highest stable Interface. Screen tests verify rendering and navigation; they do not duplicate Module internals.」（同上）
- 「Each command that can have an unknown outcome uses a durable idempotency identity. Network failure triggers lookup or reconciliation, not silent replay.」（同上）——转写失败后的重试**复用同一 requestId**（服务端 `mobile_voice.go:465-487` 从结果行幂等重放，绝不重复计费）；重试是显式用户动作，不是静默自动重发。
- 「Voice Room 拥有实时语音 session、microphone permission、音频状态、转写草稿和断线结束语义。它不拥有 Task 权限、审批或时间线；确认后的文字通过 TaskHandle.act 提交。」（mobile-module-seams.md §8.1）——听写子面遵守同一所有权：Dictation 模块拥有录音生命周期与转写草稿，不碰 Task 权限/提交（#56 的确认文字经目标草稿进入 Task Office 的 start 通道；`TaskHandle.act` 输入面属 #37，当前 HEAD 未实现）。
- 「禁止：Screen 直接导入 packages/contracts 或 packages/api-client；Screen 自己维护 request_id、cursor、revision、scope generation；每个 Screen 建独立 query cache 或 token refresh」（mobile-module-seams.md §10）——NewTaskScreen 只见 `DictationState` + 回调；requestId 由模块经注入的 `newRequestId` 铸造；`createMobileVoiceTranscriptionRemote` 只出现在组合根。
- 「语音交互记录（Voice Interaction Record）：实时语音交互中经成员确认的文字输入和 Agent 文字答复……原始音频默认在实时处理后删除」（CONTEXT.md:339）——模块内存中的音频在转写派发成功/取消/丢弃时即刻释放（`dropIntent`），原生临时文件由捕获 Adapter 在 stop/cancel 后 `release`（release 释放录音机对象，不保证即时删盘——真机实际删除验收属 #69/#70，最终审查 t56 已在 Adapter 内如实注释）；模块不留原始音频缓存。
- 服务端 URL/凭据约束（会话注入）：转写走 `MobileRuntime.authorizedRequest`（origin 已由 `requireDeploymentOrigin` 强校验为无凭据 HTTPS）；集成冒烟复用 `disallowedDeploymentHost` 主机防线（拒 localhost/环回/私网/保留地址）；凭据只从 `WEKNORA_MOBILE_TEST_*` 环境变量读取，源码与测试不写可用凭据字面量。本计划 **Go/SQL 零改动**（参数绑定约束自动满足）。
- 工作流约束：严格 RED→GREEN→REFACTOR（每个任务先写失败测试、实跑确认失败、最小实现、通过、提交）；实现不与已批准 Spec 冲突，冲突时升级而非静默重设计。
- 上限对齐（数值为工程默认，与服务端常量逐字对齐）：单次转写音频 ≤ `16 << 20` 字节（`mobile_voice.go:51` `voiceMaxAudioBytes`）；录音时长上限 120 秒（`mobile_voice.go:49` `voiceTranscribeMaxSeconds` 预算窗）；确认并入后的目标文本 ≤ 500 字（`apps/mobile/src/screens/NewTaskScreen.tsx:14` `GOAL_TEXT_MAX_LENGTH`，R1 裁决第三层）。

**Issue #56 验收标准原文（docs/plans/issue30-sweep/issues/issue-56.md）：**

1. 「取消录音不取消 Task。」
2. 「转写必须由用户确认后才提交。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

What to build 原文：「新建和 Task 输入支持录音转写为可编辑草稿；拒权、取消和迟到转写不破坏文字输入。」

**验收标准 3 的本地可验证性说明（blocked-env 声明）：** 真实端到端（生产 JSON transport + Runtime 授权通道 + 具体 Remote Adapter + Dictation 模块 + 真实控制器 + 真后端 `POST /api/v1/mobile/voice/transcriptions`）沿用 T01–T06 已合并的 opt-in 真实 HTTP 模式，需要「一个真实 WeKnora Deployment（HTTPS 公网 origin）+ 一个测试账号 + 服务端已配置 voice admission 价格版本与真实转写 provider」（`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD` 环境变量）。本地无此环境时 Task 5 的真实 HTTP 用例以 `t.skip` 跳过（**不得伪造通过**）；部署未配置语音计价时服务端如实返回 503 `voice_charging_unconfigured`，证据记 `charging-unconfigured`（合法结论，不是失败）。本地替代证据：Dictation 模块 Interface 场景测试（Task 1，真实模块编排 + scripted Adapter，覆盖 AC1/AC2 与拒权/迟到/竞态全部分支）+ api-client wire 契约测试（真实 FormData 构造，Task 2）+ 屏渲染/接线测试与源级守卫（Task 3/4）+ 服务端既有 18 个 Go 测试复跑（幂等/预算/结算的服务端证据，本计划作者已实跑 PASS）。真机麦克风权限/录音属 #69/#70 真机验收门槛（spec：「real-device acceptance separately」），本地以 scripted 捕获 Adapter 为证据，不伪造真机结论。

**与调查结论的差异记录（以代码现状为准）：**

1. 调查缺口第 5 条「前置 #36（通用目标输入与耐久 Task 创建）未实现」**已过时**：#36 已在当前 HEAD 合并——亲眼核实 `apps/mobile/src/app/new.tsx`（/new 路由 + NewTaskRouteLifecycle）、`apps/mobile/src/new-task-view.ts`（createNewTaskController，AC2 保留语义宿主）、`apps/mobile/src/screens/NewTaskScreen.tsx`、`apps/mobile/src/new-task-drafts.ts`（Scoped Vault 加密草稿）、`apps/mobile/src/task-start-integration-smoke.ts` 均存在。本计划直接挂接其产出（`controller.update` / `GOAL_TEXT_MAX_LENGTH` / `createNativeRequestId`）。
2. 调查称服务端证据为 `mobile_voice.go:445` `TranscribeAudio` —— 亲眼核实行号正确；补充调查未记录的 wire 事实：路由挂在 `/api/v1`（`router.go:380` 传 `v1`（`router.go:296` `v1 := r.Group("/api/v1")`）→ `routes_workbench.go:184-192`），完整路径 `POST /api/v1/mobile/voice/transcriptions`；multipart 字段为 `request_id`、`voice_session_id?`、`locale?` + 文件 part 名 **`audio`**（`mobile_voice.go:450-452,488`）；响应信封 `{success:true,data:{text,audio_seconds,settled}}`（`mobile_voice.go:601`）；错误信封 `{success:false,error:"voice_charging_unconfigured"}` 503 / `"transcribe_failed"` 502。api-client 的 `ApiError.code` 对该错误体为 `HTTP_<status>`（`errors.ts:58-60`——`error` 字符串只进 message），故 503→`VOICE_CHARGING_UNCONFIGURED` 的语义翻译在 Task 2 的 Remote 内完成。
3. 调查称「11 个 Voice 测试本次实跑全 PASS」——本计划作者本次实跑 `-run 'TestVoice|TestTranscription|TestPriceVersion'` 命中 **18 个测试函数**全 PASS（`mobile_voice_test.go:313-957`；最终审查 t56 勘误：实跑 `go test -v` 顶层 `=== RUN` 计数 18、`grep -cE '^func (TestVoice|TestTranscription|TestPriceVersion)' internal/handler/mobile_voice_test.go` = 18，初稿「19」为元数据笔误，计划基线 HEAD `fb5f6653a` 当时即为 18）。数量差异不影响结论。
4. 调查缺口 1–4 条（无录音/转写/草稿 UI、无取消语义、无确认流、无 Interface 测试覆盖）经当前 HEAD 核实**全部属实**：`apps/mobile/src/adapters/` 无 dictation 文件、`packages/mobile-core/src/` 无 voice/ 目录、composition.ts 无 voice 接线、app-smoke 无听写测试。
5. 真机依赖新增：`expo-audio` 不在 `apps/mobile/package.json`——真机构建前置 `cd apps/mobile && npx expo install expo-audio`（与 #41 的 `expo-notifications` 前置同型）。Node 测试链不依赖它（Adapter 构造时 require 失败 → 返回 undefined，fail closed）。

## Review Focus

Spec 隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **迟到转写覆盖宿主文字**：用户取消/放弃后转写才返回，或新一次听写进行中上一次结果返回——若应用到状态会覆盖用户手打文字或新草稿。——Task 1 测试「a transcription that resolves after cancel is dropped entirely」「cancel during a slow stop() drops the capture without dispatching transcription」（代次守卫，迟到结果整代丢弃）。
2. **停止录音竞态取消**：`finish()` 的 `capture.stop()` 尚未返回时用户按了取消——若 stop 返回后仍派发转写，会产生一次用户已明确取消的计费转写。——Task 1 测试同上第二条（断言 transcriber 零派发）。
3. **失败重试造成重复计费**：网络失败后盲目换新 requestId 重发 = 同一段音频两次计费转写。——Task 1 测试「retry after a transcription failure reuses the same request id」（同一意图同一 requestId；服务端幂等重放由既有 `TestTranscriptionReplayServedFromResultRow` 证明）。
4. **拒权后输入面被卡死**：麦克风权限被拒后屏不可用或草稿被清——拒权必须只是「听写不可用」，手打输入与既有草稿完全不受影响。——Task 1 测试「microphone denial surfaces denied without any capture or transcription work」+ Task 3 渲染测试（denied 态下手打 TextInput 与 Submit 仍在）。
5. **确认文字挤爆 SecureStore 意图记录**：手打 + 转写追加后目标文本超过 500 字上限，破坏 #36 的 2048B 单条意图记录预算。——Task 3 测试「applyConfirmedDictation appends without clobbering and caps at the goal-text limit」。

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 1 | mobile-core：`createDictation` 深模块 | `packages/mobile-core/src/voice/dictation.ts`（端口 + 状态机：begin/finish/cancel/retryTranscription/edit/confirm/discard/dispose、代次守卫、时长/大小上限）+ `in-memory-dictation.ts`（scripted 捕获/转写 Adapter）+ index.ts 导出 |
| 2 | api-client：`createMobileVoiceTranscriptionRemote` | `packages/mobile-client/src/mobile/voice.ts`（multipart wire 适配 + 503 语义翻译）+ exports `./mobile/voice` |
| 3 | apps/mobile：视图层 | `dictation-view.ts`（applyConfirmedDictation + DICTATION_FAILURE_COPY + 结构可赋值证明）+ NewTaskScreen 听写 UI + app-smoke 渲染测试 |
| 4 | apps/mobile：组合根与路由接线 | `adapters/dictation-capture.ts`（expo-audio 惰性 require fail closed）+ composition `dictationFor`/`activeDictation` + `/new` 路由接线与卸载兜底 + app-smoke 源级守卫 |
| 5 | apps/mobile：真实 HTTP 集成证据（AC3） | `voice-dictation-integration-smoke.ts`（opt-in 真实端到端：真实 Runtime 授权通道 + 真 Remote + 真模块 + 真控制器 + scripted 捕获）+ 证据契约测试 |

**并行批次注意（本计划与同批其余计划并行实施）：** 新增文件全部为本计划独有（`packages/mobile-core/src/voice/*`、`packages/api-client/src/mobile/voice*`、`apps/mobile/src/dictation-view*`、`apps/mobile/src/adapters/dictation-capture*`、`apps/mobile/src/voice-dictation-integration-smoke*`）。共享文件修改清单与位置（均为最小、位置明确的追加）：`packages/mobile-core/src/index.ts`（末尾追加 voice 导出块）；`packages/api-client/package.json`（exports 在 `"./mobile/resources"` 后加 1 行）；`apps/mobile/src/screens/NewTaskScreen.tsx`（props 接口扩展 + 目标输入块后插入听写块 + 1 行 import）；`apps/mobile/src/composition.ts`（import 区追加 + `activeTaskMaterial()` 之后追加 dictation 工厂）；`apps/mobile/src/app/new.tsx`（import 追加 + NewTaskRouteLifecycle 内接线 + 卸载兜底 effect）；`apps/mobile/src/app-smoke.test.tsx`（文件末尾追加 4 个测试）。**Go 零改动。**

---

### Task 1: mobile-core——`createDictation` 深模块（听写状态机 + 场景 Adapter）

**Files:**
- Create: `packages/mobile-core/src/voice/dictation.ts`
- Create: `packages/mobile-core/src/voice/in-memory-dictation.ts`
- Test: `packages/mobile-core/src/voice/dictation.test.ts`
- Modify: `packages/mobile-core/src/index.ts`（末尾追加导出块）

**Interfaces:**
- Consumes: 无（全新模块；不依赖 runtime/shelf/vault 任何既有导出——转写派发通道由调用方注入）。
- Produces（Task 2/3/4/5 与 #57/#69/#70 依赖的精确签名）:
  - `createDictation(ports: DictationPorts): Dictation`
  - `interface DictationPorts { capture: DictationCapturePort; transcribe: DictationTranscriptionPort; newRequestId(): string; maxDurationMs?: number }`
  - `interface DictationCapturePort { start(): Promise<'recording' | 'denied'>; stop(): Promise<DictationAudio | undefined>; cancel(): Promise<void> }`
  - `interface DictationTranscriptionPort { transcribe(input: { requestId: string; audio: DictationAudio }): Promise<{ text: string; audioSeconds?: number }> }`
  - `interface DictationAudio { uri?: string; bytes?: Uint8Array; mimeType: string; fileName?: string }`（两者恰有其一；uri=RN 原生文件源，bytes=Node/集成冒烟字节源）
  - `interface Dictation { state(): DictationState; subscribe(listener: (state: DictationState) => void): () => void; begin(): Promise<void>; finish(): Promise<void>; cancel(): Promise<void>; retryTranscription(): Promise<void>; editTranscript(text: string): void; confirmTranscript(): string | undefined; discardTranscript(): void; dispose(): void }`
  - `type DictationPhase = 'idle' | 'recording' | 'transcribing' | 'review' | 'denied' | 'failed'`；`type DictationFailure = 'capture-failed' | 'audio-too-large' | 'transcription-failed'`；`interface DictationState { phase: DictationPhase; transcript?: string; failure?: DictationFailure; failureCode?: string }`
  - 常量 `DICTATION_MAX_AUDIO_BYTES = 16 << 20`、`DICTATION_MAX_DURATION_MS = 120_000`
  - 场景 Adapter：`createScriptedDictationCapture(options?: { start?: 'recording' | 'denied' | Error; stop?: DictationAudio | undefined | Error; manualStop?: boolean }): ScriptedDictationCapture`（`calls: string[]`、`resolveStop(result): void`）；`createScenarioDictationTranscriber(): ScenarioDictationTranscriber`（`requests: Array<{ requestId: string; audio: DictationAudio }>`、`resolve(index, result | Error): void`、`pending(): number`）

- [ ] **Step 1: 写失败测试**

创建 `packages/mobile-core/src/voice/dictation.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  createDictation,
  DICTATION_MAX_AUDIO_BYTES,
  type DictationAudio,
  type DictationPhase,
  type DictationState,
} from './dictation.ts';
import { createScenarioDictationTranscriber, createScriptedDictationCapture } from './in-memory-dictation.ts';

const audio = (): DictationAudio => ({ bytes: new Uint8Array([1, 2, 3]), mimeType: 'audio/wav', fileName: 'd.wav' });
const tick = async (): Promise<void> => { await new Promise<void>((resolve) => setImmediate(resolve)); };

test('AC2 happy path: stop → transcribe → editable review → confirm returns the edited text and never re-dispatches', async () => {
  const capture = createScriptedDictationCapture({ stop: audio() });
  const transcriber = createScenarioDictationTranscriber();
  const phases: DictationPhase[] = [];
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  dictation.subscribe((next) => phases.push(next.phase));

  await dictation.begin();
  assert.equal(dictation.state().phase, 'recording');

  const finishing = dictation.finish();
  await tick(); await tick();
  assert.equal(dictation.state().phase, 'transcribing');

  transcriber.resolve(0, { text: '整理知识库并发周报', audioSeconds: 4 });
  await finishing;
  assert.equal(dictation.state().phase, 'review');
  assert.equal(dictation.state().transcript, '整理知识库并发周报');

  dictation.editTranscript('整理知识库并发周报（已校对）');
  assert.equal(dictation.state().transcript, '整理知识库并发周报（已校对）');

  assert.equal(dictation.confirmTranscript(), '整理知识库并发周报（已校对）');
  assert.equal(dictation.state().phase, 'idle');
  assert.equal(transcriber.requests.length, 1, 'AC2：确认只消费既有转写，模块绝不再次派发，也绝不提交');
  assert.deepEqual(transcriber.requests[0]!.audio.mimeType, 'audio/wav');
  assert.deepEqual(capture.calls, ['start', 'stop']);
  assert.deepEqual(phases, ['recording', 'transcribing', 'review', 'review', 'idle']);
});

test('AC1: cancelling during recording discards the audio, never dispatches transcription, and leaves the module reusable', async () => {
  const capture = createScriptedDictationCapture({ stop: audio() });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });

  await dictation.begin();
  await dictation.cancel();
  assert.equal(dictation.state().phase, 'idle');
  assert.deepEqual(capture.calls, ['start', 'cancel'], '取消路径不调用 stop——音频被丢弃而不是被消费');
  assert.equal(transcriber.requests.length, 0, '取消录音绝不触发转写派发（也就不可能取消/影响 Task）');

  // 模块可复用：取消后再次听写走完整流
  await dictation.begin();
  const finishing = dictation.finish();
  await tick(); await tick();
  transcriber.resolve(0, { text: '第二次听写' });
  await finishing;
  assert.equal(dictation.state().phase, 'review');
});

test('microphone denial surfaces denied without any stop or transcription work, and begin can re-request', async () => {
  const capture = createScriptedDictationCapture({ start: 'denied' });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });

  await dictation.begin();
  assert.equal(dictation.state().phase, 'denied');
  assert.deepEqual(capture.calls, ['start']);
  assert.equal(transcriber.requests.length, 0);

  await dictation.begin(); // 拒权不是终态：用户授权后可重试
  assert.equal(dictation.state().phase, 'denied');
  assert.deepEqual(capture.calls, ['start', 'start']);
});

test('discarding the review transcript returns to idle without any further dispatch', async () => {
  const capture = createScriptedDictationCapture({ stop: audio() });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  await dictation.begin();
  const finishing = dictation.finish();
  await tick(); await tick();
  transcriber.resolve(0, { text: '将被放弃' });
  await finishing;

  dictation.discardTranscript();
  assert.equal(dictation.state().phase, 'idle');
  assert.equal(dictation.state().transcript, undefined);
  assert.equal(transcriber.requests.length, 1);
});

test('confirmTranscript refuses an empty transcript and keeps the review surface', async () => {
  const capture = createScriptedDictationCapture({ stop: audio() });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  await dictation.begin();
  const finishing = dictation.finish();
  await tick(); await tick();
  transcriber.resolve(0, { text: '  ' }); // 服务端成功但空文本：没有可确认的内容
  await finishing;
  assert.equal(dictation.state().failure, 'transcription-failed', '空转写按失败处理，不产出空 review');

  await dictation.begin();
  const second = dictation.finish();
  await tick(); await tick();
  transcriber.resolve(1, { text: '有内容' });
  await second;
  dictation.editTranscript('   ');
  assert.equal(dictation.confirmTranscript(), undefined, '空白确认被拒绝');
  assert.equal(dictation.state().phase, 'review');
});

test('a late transcription that resolves after cancel is dropped entirely (it must never reach review)', async () => {
  const capture = createScriptedDictationCapture({ stop: audio() });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  await dictation.begin();
  const finishing = dictation.finish();
  await tick(); await tick();
  assert.equal(dictation.state().phase, 'transcribing');

  await dictation.cancel(); // 转写在途时取消
  assert.equal(dictation.state().phase, 'idle');

  transcriber.resolve(0, { text: '迟到的转写' }); // 服务端此刻才回包
  await finishing;
  await tick();
  assert.equal(dictation.state().phase, 'idle', '迟到结果整代丢弃');
  assert.equal(dictation.state().transcript, undefined);
});

test('cancel during a slow stop() drops the capture without dispatching transcription', async () => {
  const capture = createScriptedDictationCapture({ stop: audio(), manualStop: true });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  await dictation.begin();

  const finishing = dictation.finish(); // stop() 挂起（manualStop）
  await dictation.cancel(); // 用户在 stop 返回前取消
  capture.resolveStop(audio()); // stop 此刻才返回音频
  await finishing;
  await tick();

  assert.equal(transcriber.requests.length, 0, '已取消的停止不得派发转写（一次用户已明确取消的计费转写）');
  assert.equal(dictation.state().phase, 'idle');
});

test('retry after a transcription failure reuses the same request id (idempotent replay, never a second charge)', async () => {
  const capture = createScriptedDictationCapture({ stop: audio() });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  await dictation.begin();
  const finishing = dictation.finish();
  await tick(); await tick();
  transcriber.resolve(0, new Error('network failed after dispatch'));
  await finishing;
  assert.equal(dictation.state().phase, 'failed');
  assert.equal(dictation.state().failure, 'transcription-failed');

  const retrying = dictation.retryTranscription();
  assert.equal(dictation.state().phase, 'transcribing');
  transcriber.resolve(1, { text: '重试后的转写' });
  await retrying;
  assert.equal(transcriber.requests[1]!.requestId, 'req-1', '同一意图同一 request_id（W04 重放语义，服务端从结果行幂等应答）');
  assert.equal(dictation.state().phase, 'review');
});

test('the adapter-level failure code surfaces for presentation and evidence', async () => {
  const capture = createScriptedDictationCapture({ stop: audio() });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  await dictation.begin();
  const finishing = dictation.finish();
  await tick(); await tick();
  transcriber.resolve(0, Object.assign(new Error('voice charging is not configured'), { code: 'VOICE_CHARGING_UNCONFIGURED' }));
  await finishing;
  assert.equal(dictation.state().failure, 'transcription-failed');
  assert.equal(dictation.state().failureCode, 'VOICE_CHARGING_UNCONFIGURED');
});

test('recording auto-finishes at the configured duration cap (aligned with the 120s server budget window)', async () => {
  const capture = createScriptedDictationCapture({ stop: audio() });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1', maxDurationMs: 25 });
  await dictation.begin();
  assert.equal(dictation.state().phase, 'recording');
  await new Promise((resolve) => setTimeout(resolve, 80));
  assert.equal(dictation.state().phase, 'transcribing', '到点自动 finish，录音不越过服务端预算窗');
  transcriber.resolve(0, { text: '到点转写' });
  await tick();
  assert.equal(dictation.state().phase, 'review');
});

test('an oversized capture (> 16 MiB) fails closed before any upload', async () => {
  const capture = createScriptedDictationCapture({
    stop: { bytes: new Uint8Array(DICTATION_MAX_AUDIO_BYTES + 1), mimeType: 'audio/wav' },
  });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  await dictation.begin();
  await dictation.finish();
  assert.equal(dictation.state().failure, 'audio-too-large');
  assert.equal(transcriber.requests.length, 0, '超限在派发前拒绝，不浪费一次注定 400 的上载');
});

test('a failing capture start surfaces capture-failed and never reaches transcription', async () => {
  const capture = createScriptedDictationCapture({ start: new Error('microphone busy') });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  await dictation.begin();
  assert.equal(dictation.state().phase, 'failed');
  assert.equal(dictation.state().failure, 'capture-failed');
  assert.equal(transcriber.requests.length, 0);
});

test('a second begin while recording is ignored (no double capture)', async () => {
  const capture = createScriptedDictationCapture({ stop: audio() });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  await dictation.begin();
  await dictation.begin();
  assert.deepEqual(capture.calls, ['start'], '录音中的重复 begin 零副作用');
  await dictation.cancel(); // 清理时长定时器（否则 120s 定时器挂起测试进程）
});

test('stop without any available audio is a capture failure, not an empty upload', async () => {
  const capture = createScriptedDictationCapture({ stop: undefined });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  await dictation.begin();
  await dictation.finish();
  assert.equal(dictation.state().failure, 'capture-failed');
  assert.equal(transcriber.requests.length, 0);
});
```

（`DictationPhase` 需要加入 import：`type DictationPhase`。）

创建 `packages/mobile-core/src/voice/in-memory-dictation.ts`：

```ts
import type { DictationAudio, DictationCapturePort, DictationTranscriptionPort } from './dictation.ts';

export interface ScriptedDictationCaptureOptions {
  /** start() 的脚本化结果：'recording'（默认）/ 'denied'（权限拒绝）/ Error（捕获失败）。 */
  start?: 'recording' | 'denied' | Error;
  /** stop() 的脚本化结果：音频 / undefined（无可用音频，默认）/ Error。 */
  stop?: DictationAudio | undefined | Error;
  /** true 时 stop() 挂起直至测试调用 resolveStop()（停止/取消竞态用）。 */
  manualStop?: boolean;
}

export interface ScriptedDictationCapture extends DictationCapturePort {
  calls: string[];
  resolveStop(result: DictationAudio | undefined | Error): void;
}

/** 听写捕获的 scripted test Adapter（spec：system audio 以 scripted Adapter 进 Interface 测试）。 */
export function createScriptedDictationCapture(options: ScriptedDictationCaptureOptions = {}): ScriptedDictationCapture {
  const startScript = options.start ?? 'recording';
  const calls: string[] = [];
  let releaseStop: ((result: DictationAudio | undefined | Error) => void) | undefined;
  return {
    calls,
    async start() {
      calls.push('start');
      if (startScript instanceof Error) throw startScript;
      return startScript;
    },
    async stop() {
      calls.push('stop');
      if (options.manualStop === true) {
        return new Promise<DictationAudio | undefined>((resolve, reject) => {
          releaseStop = (result) => { if (result instanceof Error) reject(result); else resolve(result); };
        });
      }
      const scripted = options.stop;
      if (scripted instanceof Error) throw scripted;
      return scripted;
    },
    async cancel() { calls.push('cancel'); },
    resolveStop(result) {
      const release = releaseStop;
      releaseStop = undefined;
      if (release !== undefined) release(result);
    },
  };
}

export interface ScenarioDictationTranscriber extends DictationTranscriptionPort {
  requests: Array<{ requestId: string; audio: DictationAudio }>;
  /** 手动控制转写应答时机（迟到结果/失败重试测试）。 */
  resolve(index: number, result: { text: string; audioSeconds?: number } | Error): void;
  pending(): number;
}

/** 听写转写的手动场景 Adapter：每次 dispatch 记入 requests，由测试按 index 应答。 */
export function createScenarioDictationTranscriber(): ScenarioDictationTranscriber {
  const requests: Array<{ requestId: string; audio: DictationAudio }> = [];
  const resolvers: Array<(result: { text: string; audioSeconds?: number } | Error) => void> = [];
  return {
    requests,
    transcribe(input) {
      requests.push({ requestId: input.requestId, audio: input.audio });
      return new Promise((resolve, reject) => {
        resolvers.push((result) => { if (result instanceof Error) reject(result); else resolve(result); });
      });
    },
    resolve(index, result) {
      const resolve = resolvers[index];
      if (resolve !== undefined) resolve(result);
    },
    pending: () => resolvers.length,
  };
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/voice/dictation.test.ts`
Expected: FAIL——`Cannot find module './dictation.ts'`（或 `package '…/voice/dictation.ts' not found`）。

- [ ] **Step 3: 写最小实现**

创建 `packages/mobile-core/src/voice/dictation.ts`：

```ts
/**
 * Dictation 深模块（Issue #56 / T26）——module-seams §8.1「Voice Room 拥有……转写草稿」
 * 的听写子面。所有权：录音生命周期、麦克风权限结果、转写草稿（可编辑）与迟到结果丢弃。
 * 不拥有：Task 权限、提交、审批、时间线（AC2 的结构性保障——本模块没有任何提交方法）。
 *
 * 语义要点：
 *   - AC1：cancel() 在任何阶段都只回到 idle——丢音频、丢在途转写结果、绝不触发转写派发，
 *     也绝不触碰宿主（New 屏）的文字草稿；Task 与否只由宿主的既有提交通道决定。
 *   - AC2：转写结果只进入 review（可编辑）；confirmTranscript() 返回确认文本后由宿主写入
 *     目标输入；提交仍是 #36 的显式 Submit。
 *   - 幂等：一次 finish 铸造一个 requestId；失败后 retryTranscription() 复用同一 id
 *     （服务端从结果行幂等重放，mobile_voice.go:465-487）。
 *   - 迟到丢弃：代次守卫——cancel/begin 推进 generation，任何旧代次异步结果整代忽略。
 *   - 原始音频：仅存活于 dispatch 前后（失败重试窗口），成功/取消/丢弃即释放（CONTEXT.md:339）。
 */

/** 服务端单次转写捕获上限（internal/handler/mobile_voice.go:51 voiceMaxAudioBytes = 16 << 20）。 */
export const DICTATION_MAX_AUDIO_BYTES = 16 << 20;
/** 服务端单次转写预算窗（mobile_voice.go:49 voiceTranscribeMaxSeconds = 120）。 */
export const DICTATION_MAX_DURATION_MS = 120_000;

/** 一次捕获的音频源：uri（RN 原生文件源）与 bytes（Node/集成冒烟）恰有其一。 */
export interface DictationAudio {
  uri?: string;
  bytes?: Uint8Array;
  mimeType: string;
  fileName?: string;
}

export type DictationCaptureStart = 'recording' | 'denied';

/** 麦克风捕获 Port（module-seams §8.3 Audio Device Port）：原生 Adapter 与 test Adapter 共用。 */
export interface DictationCapturePort {
  /** 请求麦克风权限并开始捕获；'denied' 表示权限拒绝（零捕获）。其余失败以异常上抛（→ capture-failed）。 */
  start(): Promise<DictationCaptureStart>;
  /** 停止并取回捕获；无可用音频返回 undefined。 */
  stop(): Promise<DictationAudio | undefined>;
  /** 停止并丢弃音频（AC1 取消路径；原生 Adapter 同时释放临时文件）。 */
  cancel(): Promise<void>;
}

export interface DictationTranscriptionInput {
  requestId: string;
  audio: DictationAudio;
}

export interface DictationTranscriptionResult {
  text: string;
  audioSeconds?: number;
}

/** 服务端转写代理 Port（wire 适配在 api-client，经 Runtime 授权通道）。 */
export interface DictationTranscriptionPort {
  transcribe(input: DictationTranscriptionInput): Promise<DictationTranscriptionResult>;
}

export type DictationPhase = 'idle' | 'recording' | 'transcribing' | 'review' | 'denied' | 'failed';
export type DictationFailure = 'capture-failed' | 'audio-too-large' | 'transcription-failed';

export interface DictationState {
  phase: DictationPhase;
  /** review 阶段的可编辑转写草稿（AC2）。 */
  transcript?: string;
  failure?: DictationFailure;
  /** 适配器级错误码（如 VOICE_CHARGING_UNCONFIGURED），仅用于呈现与证据，不参与分支。 */
  failureCode?: string;
}

export interface DictationPorts {
  capture: DictationCapturePort;
  transcribe: DictationTranscriptionPort;
  newRequestId(): string;
  /** 录音时长上限（毫秒）；缺省 DICTATION_MAX_DURATION_MS。到点自动 finish。 */
  maxDurationMs?: number;
}

export interface Dictation {
  state(): DictationState;
  subscribe(listener: (state: DictationState) => void): () => void;
  /** 开始录音（先请求权限；denied 只影响听写可用性，从不影响宿主文字草稿）。 */
  begin(): Promise<void>;
  /** 停止录音并派发转写；结果进入 review，不写宿主。 */
  finish(): Promise<void>;
  /** AC1：取消——任何阶段回 idle，丢音频/丢在途结果，零转写派发，宿主不受影响。 */
  cancel(): Promise<void>;
  /** 转写失败后显式重试：复用同一 requestId 与同一段音频（幂等重放）。 */
  retryTranscription(): Promise<void>;
  /** review 中编辑转写文本。 */
  editTranscript(text: string): void;
  /** AC2：确认转写——返回（可能已编辑的）文本，模块回 idle；写入宿主由调用方承担。空白返回 undefined 并保持 review。 */
  confirmTranscript(): string | undefined;
  /** 放弃本次转写（review → idle）。 */
  discardTranscript(): void;
  dispose(): void;
}

function failureCodeOf(error: unknown): string | undefined {
  if (error instanceof Error && 'code' in error) {
    const code = (error as { code?: unknown }).code;
    if (typeof code === 'string' && code.trim() !== '') return code;
  }
  return undefined;
}

export function createDictation(ports: DictationPorts): Dictation {
  const maxDurationMs = ports.maxDurationMs ?? DICTATION_MAX_DURATION_MS;
  let state: DictationState = { phase: 'idle' };
  let generation = 0;
  let beginning = false;
  let requestId: string | undefined;
  let pendingAudio: DictationAudio | undefined;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const listeners = new Set<(state: DictationState) => void>();
  const publish = (next: DictationState): void => {
    state = next;
    for (const listener of [...listeners]) listener(state);
  };
  const clearTimer = (): void => {
    if (timer !== undefined) { clearTimeout(timer); timer = undefined; }
  };
  const dropIntent = (): void => {
    requestId = undefined;
    pendingAudio = undefined; // 原始音频即刻释放（失败重试窗口结束）
  };

  async function dispatchTranscription(attemptGeneration: number, id: string, audio: DictationAudio): Promise<void> {
    try {
      const result = await ports.transcribe.transcribe({ requestId: id, audio });
      if (attemptGeneration !== generation) return; // 迟到结果：整代丢弃
      if (typeof result.text !== 'string' || result.text.trim() === '') {
        dropIntent(); // 空转写无重试价值（同 id 重放仍为空）
        publish({ phase: 'failed', failure: 'transcription-failed' });
        return;
      }
      dropIntent();
      publish({ phase: 'review', transcript: result.text });
    } catch (error) {
      if (attemptGeneration !== generation) return; // 迟到失败同样丢弃
      const code = failureCodeOf(error);
      // 保留 requestId/pendingAudio：失败后的显式重试复用同一幂等身份
      publish({ phase: 'failed', failure: 'transcription-failed', ...(code === undefined ? {} : { failureCode: code }) });
    }
  }

  const dictation: Dictation = {
    state: () => state,
    subscribe(listener) {
      listeners.add(listener);
      return () => { listeners.delete(listener); };
    },
    async begin() {
      if (beginning || state.phase === 'recording' || state.phase === 'transcribing') return;
      beginning = true;
      const attemptGeneration = generation;
      try {
        clearTimer();
        dropIntent();
        let started: DictationCaptureStart;
        try {
          started = await ports.capture.start();
        } catch {
          if (attemptGeneration === generation) publish({ phase: 'failed', failure: 'capture-failed' });
          return;
        }
        if (attemptGeneration !== generation) {
          // begin 在途时被取消：丢弃本次开始（best-effort 停捕获）
          try { await ports.capture.cancel(); } catch { /* 已取消路径不外泄 */ }
          return;
        }
        if (started === 'denied') {
          publish({ phase: 'denied' });
          return;
        }
        publish({ phase: 'recording' });
        timer = setTimeout(() => {
          timer = undefined;
          void dictation.finish(); // 到点自动停止（录音不越过服务端 120s 预算窗）
        }, maxDurationMs);
      } finally {
        beginning = false;
      }
    },
    async finish() {
      if (state.phase !== 'recording') return;
      clearTimer();
      const attemptGeneration = generation;
      let audio: DictationAudio | undefined;
      try {
        audio = await ports.capture.stop();
      } catch {
        audio = undefined;
      }
      if (attemptGeneration !== generation) {
        dropIntent(); // stop 在途时已取消：丢弃，不转写
        return;
      }
      if (audio === undefined || (audio.bytes === undefined && audio.uri === undefined)) {
        dropIntent();
        publish({ phase: 'failed', failure: 'capture-failed' });
        return;
      }
      if (audio.bytes !== undefined && audio.bytes.byteLength > DICTATION_MAX_AUDIO_BYTES) {
        dropIntent();
        publish({ phase: 'failed', failure: 'audio-too-large' });
        return;
      }
      const id = ports.newRequestId();
      requestId = id;
      pendingAudio = audio;
      publish({ phase: 'transcribing' });
      await dispatchTranscription(attemptGeneration, id, audio);
    },
    async cancel() {
      generation += 1; // 任何在途异步（start/stop/转写）迟到即弃
      clearTimer();
      const wasRecording = state.phase === 'recording';
      dropIntent();
      publish({ phase: 'idle' }); // 先回 idle（UI 立即恢复），再收尾捕获
      if (wasRecording) {
        try { await ports.capture.cancel(); } catch { /* 取消失败不影响回 idle */ }
      }
    },
    async retryTranscription() {
      if (state.phase !== 'failed' || state.failure !== 'transcription-failed' || requestId === undefined || pendingAudio === undefined) return;
      const attemptGeneration = generation;
      publish({ phase: 'transcribing' });
      await dispatchTranscription(attemptGeneration, requestId, pendingAudio);
    },
    editTranscript(text) {
      if (state.phase !== 'review') return;
      publish({ ...state, transcript: text });
    },
    confirmTranscript() {
      if (state.phase !== 'review' || typeof state.transcript !== 'string') return undefined;
      const text = state.transcript.trim();
      if (text === '') return undefined;
      publish({ phase: 'idle' });
      return text;
    },
    discardTranscript() {
      if (state.phase !== 'review') return;
      publish({ phase: 'idle' });
    },
    dispose() {
      generation += 1;
      clearTimer();
      dropIntent();
      const wasRecording = state.phase === 'recording';
      listeners.clear();
      if (wasRecording) void ports.capture.cancel().catch(() => undefined);
      state = { phase: 'idle' };
    },
  };
  return dictation;
}
```

在 `packages/mobile-core/src/index.ts` 末尾追加：

```ts
export { createDictation, DICTATION_MAX_AUDIO_BYTES, DICTATION_MAX_DURATION_MS } from './voice/dictation.ts';
export type {
  Dictation, DictationAudio, DictationCapturePort, DictationCaptureStart, DictationFailure,
  DictationPhase, DictationPorts, DictationState, DictationTranscriptionInput,
  DictationTranscriptionPort, DictationTranscriptionResult,
} from './voice/dictation.ts';
export { createScenarioDictationTranscriber, createScriptedDictationCapture } from './voice/in-memory-dictation.ts';
export type { ScenarioDictationTranscriber, ScriptedDictationCapture, ScriptedDictationCaptureOptions } from './voice/in-memory-dictation.ts';
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test packages/mobile-core/src/voice/dictation.test.ts`
Expected: PASS（14 个测试全过，0 fail）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/voice/dictation.ts packages/mobile-core/src/voice/in-memory-dictation.ts packages/mobile-core/src/voice/dictation.test.ts packages/mobile-core/src/index.ts
git commit -m "feat(issue30-sweep): mobile-core dictation module (T26 #56 task 1)"
```

---

### Task 2: api-client——`createMobileVoiceTranscriptionRemote`（multipart wire 适配）

**Files:**
- Create: `packages/api-client/src/mobile/voice.ts`
- Test: `packages/api-client/src/mobile/voice.test.ts`
- Modify: `packages/api-client/package.json`（exports 追加一行）

**Interfaces:**
- Consumes: `requireDeploymentOrigin(origin: string)`（`packages/api-client/src/mobile/deployment-origin.ts`，devices.ts:2 同源）；`ClientRequest`（`packages/api-client/src/client.ts:38-47`——`multipartFields`/`nativeFile` 通道存在但文件 part 名硬编码 `'file'`，与服务端 `audio` part 名不符，故本适配器自建 FormData 作为 `body` 直传——`client.ts:196` body 原样透传，`transport/json.ts:136` FormData instanceof 直传 fetch，已核实）；授权通道 `(input: ClientRequest) => Promise<unknown>`（由组合根以 `(input) => runtime.authorizedRequest(input)` 提供，composition.ts:162 同型先例）。
- Produces（Task 4 组合根与 Task 5 集成冒烟依赖）:
  - `createMobileVoiceTranscriptionRemote(options: { origin: string; request: Request }): MobileVoiceTranscriptionRemote`
  - `interface MobileVoiceTranscriptionRemote { transcribe(input: { requestId: string; audio: MobileVoiceAudio; voiceSessionId?: string }): Promise<{ text: string; audioSeconds?: number; settled?: boolean }> }`（最终审查 t56：移除服务端永不下发的 `replay` 占位——结果面与 `mobile_voice.go:601` 信封 {text,audio_seconds,settled} 逐字对齐，未知信封字段不透传）
  - `interface MobileVoiceAudio { uri?: string; bytes?: Uint8Array; mimeType: string; fileName?: string }`（与 mobile-core `DictationAudio` 结构逐字一致；结构可赋值由 Task 3 的类型证明测试落实）
  - 错误契约：HTTP 503 → `{ code: 'VOICE_CHARGING_UNCONFIGURED' }`；任何其它 HTTP 失败 → `{ code: 'VOICE_TRANSCRIBE_FAILED' }`（模块经 `failureCode` 透出）；信封畸形 → 普通 Error。
  - package.json exports 新子路径 `"./mobile/voice": "./src/mobile/voice.ts"`。

- [ ] **Step 1: 写失败测试**

创建 `packages/api-client/src/mobile/voice.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createMobileVoiceTranscriptionRemote } from './voice.ts';
import type { ClientRequest } from '../client.ts';

const ORIGIN = 'https://weknora.example.test';
const WAV = { bytes: new Uint8Array([1, 2, 3]), mimeType: 'audio/wav', fileName: 'd.wav' } as const;

test('transcribe posts a multipart body with request_id and an audio file part to the voice endpoint', async () => {
  const seen: ClientRequest[] = [];
  const remote = createMobileVoiceTranscriptionRemote({
    origin: ORIGIN,
    request: async (input) => {
      seen.push(input);
      return { success: true, data: { text: '整理周报', audio_seconds: 3, settled: true } };
    },
  });
  const result = await remote.transcribe({ requestId: 'req-1', audio: { ...WAV } });
  assert.deepEqual(result, { text: '整理周报', audioSeconds: 3, settled: true });
  assert.equal(seen[0]!.method, 'POST');
  assert.equal(seen[0]!.path, '/api/v1/mobile/voice/transcriptions');
  const body = seen[0]!.body as FormData;
  assert.ok(body instanceof FormData, '上载体是 FormData（经 authorizedTransport 原样透传）');
  assert.equal(body.get('request_id'), 'req-1');
  const part = body.get('audio');
  assert.ok(part instanceof Blob, 'audio 以文件 part 上载（服务端 c.Request.FormFile("audio")，mobile_voice.go:488）');
  assert.equal((part as Blob).type, 'audio/wav');
});

test('an optional voice_session_id rides the form; a blank one stays absent', async () => {
  const seen: ClientRequest[] = [];
  const remote = createMobileVoiceTranscriptionRemote({
    origin: ORIGIN,
    request: async (input) => { seen.push(input); return { success: true, data: { text: 'x', settled: true } }; },
  });
  await remote.transcribe({ requestId: 'req-1', audio: { ...WAV }, voiceSessionId: 'vs_1' });
  assert.equal((seen[0]!.body as FormData).get('voice_session_id'), 'vs_1');
  await remote.transcribe({ requestId: 'req-2', audio: { ...WAV }, voiceSessionId: '   ' });
  assert.equal((seen[1]!.body as FormData).get('voice_session_id'), null, '空白 voice_session_id 不进表单');
});

test('missing request id or a source-less audio is refused before any wire call', async () => {
  let calls = 0;
  const remote = createMobileVoiceTranscriptionRemote({
    origin: ORIGIN,
    request: async () => { calls += 1; return { success: true, data: { text: 'x' } }; },
  });
  await assert.rejects(() => remote.transcribe({ requestId: '  ', audio: { ...WAV } }), /request id/);
  await assert.rejects(
    () => remote.transcribe({ requestId: 'req-1', audio: { mimeType: 'audio/wav' } }),
    /bytes or uri/,
  );
  assert.equal(calls, 0, '校验发生在任何网络之前');
});

test('a 503 maps to VOICE_CHARGING_UNCONFIGURED; other HTTP failures map to VOICE_TRANSCRIBE_FAILED', async () => {
  const withStatus = (status: number) =>
    createMobileVoiceTranscriptionRemote({
      origin: ORIGIN,
      request: async () => { throw Object.assign(new Error(`Request failed with status ${status}`), { status }); },
    });
  await assert.rejects(
    () => withStatus(503).transcribe({ requestId: 'r', audio: { ...WAV } }),
    (error: unknown) => (error as { code?: string }).code === 'VOICE_CHARGING_UNCONFIGURED',
  );
  await assert.rejects(
    () => withStatus(502).transcribe({ requestId: 'r', audio: { ...WAV } }),
    (error: unknown) => (error as { code?: string }).code === 'VOICE_TRANSCRIBE_FAILED',
  );
});

test('malformed envelopes are refused (success must be true with a string data.text)', async () => {
  const remote = createMobileVoiceTranscriptionRemote({
    origin: ORIGIN,
    request: async () => ({ success: false, error: 'transcribe_failed' }),
  });
  await assert.rejects(() => remote.transcribe({ requestId: 'r', audio: { ...WAV } }), /success envelope/);
  const noText = createMobileVoiceTranscriptionRemote({
    origin: ORIGIN,
    request: async () => ({ success: true, data: { audio_seconds: 1 } }),
  });
  await assert.rejects(() => noText.transcribe({ requestId: 'r', audio: { ...WAV } }), /data\.text/);
});

test('the origin must be a validated credential-free HTTPS origin', async () => {
  assert.throws(
    () => createMobileVoiceTranscriptionRemote({ origin: 'http://insecure.example', request: async () => undefined }),
    /origin/i,
  );
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test packages/api-client/src/mobile/voice.test.ts`
Expected: FAIL——`Cannot find module './voice.ts'`。

- [ ] **Step 3: 写最小实现**

创建 `packages/api-client/src/mobile/voice.ts`：

```ts
import type { ClientRequest } from '../client.ts';
import { requireDeploymentOrigin } from './deployment-origin.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface MobileVoiceRemoteOptions {
  /** 部署 Origin。构造即强校验（共享 requireDeploymentOrigin，B3-F11 收敛）：绝对 HTTPS、含 host、无 userinfo、无 path/query/fragment。 */
  origin: string;
  /** 授权通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输。 */
  request: Request;
}

/** 与 mobile-core DictationAudio 结构逐字一致（结构可赋值由 apps/mobile 的类型证明测试落实）。 */
export interface MobileVoiceAudio {
  uri?: string;
  bytes?: Uint8Array;
  mimeType: string;
  fileName?: string;
}

export interface MobileVoiceTranscriptionInput {
  requestId: string;
  audio: MobileVoiceAudio;
  /** 可选：把本次转写用量记到自己的语音 session（服务端 mobile_voice.go:503-520）；听写不用。 */
  voiceSessionId?: string;
}

export interface MobileVoiceTranscriptionResult {
  text: string;
  audioSeconds?: number;
  settled?: boolean;
}

export interface MobileVoiceTranscriptionRemote {
  /** POST /api/v1/mobile/voice/transcriptions（internal/handler/mobile_voice.go:445，multipart：request_id / voice_session_id? / 文件 part `audio`）。 */
  transcribe(input: MobileVoiceTranscriptionInput): Promise<MobileVoiceTranscriptionResult>;
}

const VOICE_TRANSCRIBE_PATH = '/api/v1/mobile/voice/transcriptions';

function requireRequestId(requestId: string): string {
  const trimmed = typeof requestId === 'string' ? requestId.trim() : '';
  if (trimmed === '') throw new Error('voice transcription request id is required');
  return trimmed;
}

export function createMobileVoiceTranscriptionRemote(options: MobileVoiceRemoteOptions): MobileVoiceTranscriptionRemote {
  requireDeploymentOrigin(options.origin);
  const request = options.request;
  return {
    async transcribe(input: MobileVoiceTranscriptionInput): Promise<MobileVoiceTranscriptionResult> {
      const requestId = requireRequestId(input.requestId);
      const audio = input.audio;
      if (typeof audio !== 'object' || audio === null) throw new Error('voice transcription audio is required');
      if (audio.uri === undefined && audio.bytes === undefined) {
        throw new Error('voice transcription audio requires bytes or uri');
      }
      if (typeof FormData === 'undefined') throw new Error('voice transcription requires multipart form data support');
      const fileName = audio.fileName ?? (audio.uri !== undefined ? 'dictation.m4a' : 'dictation.audio');
      const form = new FormData();
      form.append('request_id', requestId);
      if (input.voiceSessionId !== undefined && input.voiceSessionId.trim() !== '') {
        form.append('voice_session_id', input.voiceSessionId);
      }
      if (audio.uri !== undefined) {
        // RN FormData 原生识别 {uri,name,type}（Expo 文件上传形态；Node 环境不会进入该分支）。
        form.append('audio', { uri: audio.uri, name: fileName, type: audio.mimeType } as unknown as Blob);
      } else {
        form.append('audio', new Blob([audio.bytes!], { type: audio.mimeType }), fileName);
      }
      let envelope: unknown;
      try {
        envelope = await request({ method: 'POST', path: VOICE_TRANSCRIBE_PATH, body: form });
      } catch (error) {
        // wire→语义翻译：服务端错误体 {"success":false,"error":"..."} 的 error 串只进 ApiError.message
        // （errors.ts:51-60），code 为 HTTP_<status>；503 = 未配置语音计价（voice_charging_unconfigured）。
        const status = error instanceof Error && 'status' in error ? Number((error as { status?: unknown }).status) : NaN;
        if (status === 503) {
          throw Object.assign(new Error('voice charging is not configured on this deployment'), { code: 'VOICE_CHARGING_UNCONFIGURED' as const });
        }
        if (!Number.isNaN(status)) {
          throw Object.assign(new Error(`voice transcription failed (HTTP ${status})`), { code: 'VOICE_TRANSCRIBE_FAILED' as const });
        }
        throw error;
      }
      if (typeof envelope !== 'object' || envelope === null || Array.isArray(envelope)) {
        throw new Error('voice transcription response must be a success envelope');
      }
      const record = envelope as { success?: unknown; data?: unknown };
      if (record.success !== true || typeof record.data !== 'object' || record.data === null) {
        throw new Error('voice transcription response.success must be true with data');
      }
      const data = record.data as { text?: unknown; audio_seconds?: unknown; settled?: unknown };
      if (typeof data.text !== 'string') throw new Error('voice transcription data.text must be a string');
      return {
        text: data.text,
        ...(typeof data.audio_seconds === 'number' ? { audioSeconds: data.audio_seconds } : {}),
        ...(data.settled === true ? { settled: true } : {}),
      };
    },
  };
}
```

在 `packages/api-client/package.json` 的 `exports` 中，`"./mobile/resources": "./src/mobile/resources.ts",` 之后追加一行：

```json
    "./mobile/voice": "./src/mobile/voice.ts",
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test packages/api-client/src/mobile/voice.test.ts`
Expected: PASS（6 个测试全过，0 fail）。

- [ ] **Step 5: Commit**

```bash
git add packages/api-client/src/mobile/voice.ts packages/api-client/src/mobile/voice.test.ts packages/api-client/package.json
git commit -m "feat(issue30-sweep): api-client mobile voice transcription remote (T26 #56 task 2)"
```

---

### Task 3: apps/mobile——视图层（确认文字并入 + 听写 UI + 渲染测试）

**Files:**
- Create: `apps/mobile/src/dictation-view.ts`
- Test: `apps/mobile/src/dictation-view.test.ts`
- Modify: `apps/mobile/src/screens/NewTaskScreen.tsx:16-24`（props 接口）与目标输入块之后（`:46` 后插入听写块）与 import 区
- Test: `apps/mobile/src/app-smoke.test.tsx`（文件末尾追加 3 个渲染测试：review 确认面 / recording 取消面 / denied 手打可用面）

**Interfaces:**
- Consumes: Task 1 的 `DictationState`/`DictationFailure`（`@weknora/mobile-core` 导出）；`GOAL_TEXT_MAX_LENGTH`（`apps/mobile/src/screens/NewTaskScreen.tsx:14`）；Task 2 的 `createMobileVoiceTranscriptionRemote`（仅用于类型可赋值证明）。
- Produces（Task 4 路由接线依赖）:
  - `applyConfirmedDictation(existing: string, confirmed: string, maxLength: number): string`——空手打 → confirmed；非空 → `existing + '\n' + confirmed`；超出 maxLength 从头截断（保 R1 裁决的 500 字/2048B 意图记录预算）。
  - `DICTATION_FAILURE_COPY: Record<'denied' | DictationFailure, string>`——四态如实文案。
  - `NewTaskScreenProps` 新增可选字段：`dictation?: DictationState; onDictationBegin?(): void; onDictationFinish?(): void; onDictationCancel?(): void; onDictationEditTranscript?(text: string): void; onDictationRetryTranscription?(): void; onDictationConfirmTranscript?(): void; onDictationDiscardTranscript?(): void;`（缺省 `dictation === undefined` 时屏不渲染任何听写元素——组合根 fail closed 的呈现面）。

- [ ] **Step 1: 写失败测试**

创建 `apps/mobile/src/dictation-view.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import type { DictationTranscriptionPort } from '@weknora/mobile-core';
import { createMobileVoiceTranscriptionRemote } from '@weknora/api-client/mobile/voice';
import { applyConfirmedDictation, DICTATION_FAILURE_COPY } from './dictation-view.ts';

test('applyConfirmedDictation appends without clobbering and caps at the goal-text limit', () => {
  assert.equal(applyConfirmedDictation('', '整理周报', 500), '整理周报', '空手打：确认文本即全部内容');
  assert.equal(applyConfirmedDictation('手写目标', '整理周报', 500), '手写目标\n整理周报', '非空手打：追加一行，不覆盖既有文字');
  const capped = applyConfirmedDictation('a'.repeat(499), '很长'.repeat(50), 500);
  assert.equal(capped.length, 500, '越界在 500 字上限处截断（保住单条意图记录的 SecureStore 预算）');
  assert.ok(capped.startsWith('a'.repeat(499)), '截断保序：既有文字优先保留');
});

test('every dictation failure surface has honest user copy', () => {
  for (const key of ['denied', 'capture-failed', 'audio-too-large', 'transcription-failed'] as const) {
    assert.equal(typeof DICTATION_FAILURE_COPY[key], 'string');
    assert.notEqual(DICTATION_FAILURE_COPY[key].trim(), '');
  }
  assert.ok(DICTATION_FAILURE_COPY['transcription-failed'].includes('不受影响'), '失败文案必须声明手打文字不受影响');
});

test('the api-client voice remote structurally satisfies the mobile-core transcription port', async () => {
  // 编译期证明（MobileVoiceAudio ≡ DictationAudio、结果结构可赋值）+ 运行期一次真实调用。
  const remote = createMobileVoiceTranscriptionRemote({
    origin: 'https://weknora.example.test',
    request: async () => ({ success: true, data: { text: '结构一致' } }),
  });
  const port: DictationTranscriptionPort = remote;
  const result = await port.transcribe({ requestId: 'req-1', audio: { bytes: new Uint8Array([1]), mimeType: 'audio/wav' } });
  assert.equal(result.text, '结构一致');
});
```

在 `apps/mobile/src/app-smoke.test.tsx` 文件末尾追加（沿用文件内既有的 `render`/`descendants`/`hooks` 助手与既有 baseState 形态）：

```tsx
test('the New entry renders the dictation review surface: editable transcript, confirm and discard (AC2)', async () => {
  const { NewTaskScreen } = await import('./screens/NewTaskScreen.tsx');
  hooks().__reset();
  const events: string[] = [];
  const baseState = {
    draft: { text: '手写目标', agentId: 'a-general', budgetUpper: 0, attachments: [] as never[], knowledgeIds: [] as string[] },
    agents: [],
    knowledge: [],
    recommendation: { agent: null, basis: 'none' },
    readiness: { ready: true, blockingAttachments: [] },
    loading: false,
    submitting: false,
    inFlight: undefined,
  };
  const callbacks = {
    onUpdate: () => {}, onSetAttachments: () => {}, onToggleKnowledge: () => {},
    onSubmit: () => { events.push('submit'); }, onCancel: () => {}, onRefreshAgents: () => {},
  };
  const element = render(NewTaskScreen, {
    state: baseState,
    ...callbacks,
    dictation: { phase: 'review', transcript: '整理知识库' },
    onDictationBegin: () => { events.push('begin'); },
    onDictationFinish: () => { events.push('finish'); },
    onDictationCancel: () => { events.push('cancel'); },
    onDictationEditTranscript: (text: string) => { events.push(`edit:${text}`); },
    onDictationRetryTranscription: () => { events.push('retry'); },
    onDictationConfirmTranscript: () => { events.push('confirm'); },
    onDictationDiscardTranscript: () => { events.push('discard'); },
  });
  const transcriptInput = descendants(element).find(({ type, props }) => type === 'TextInput' && props.placeholder === '转写草稿（确认前可编辑）');
  assert.ok(transcriptInput, 'review 态渲染可编辑转写输入');
  (transcriptInput!.props.onChangeText as (text: string) => void)('整理知识库（已校对）');
  assert.deepEqual(events, ['edit:整理知识库（已校对）'], '编辑先于确认（AC2：转写错误可修正）');
  const buttons = descendants(element).filter(({ type }) => type === 'Button').map(({ props }) => props.title);
  assert.equal(buttons.includes('Use transcript'), true);
  assert.equal(buttons.includes('Discard transcript'), true);
  (descendants(element).find(({ type, props }) => type === 'Button' && props.title === 'Use transcript')!.props.onPress as () => void)();
  assert.deepEqual(events, ['edit:整理知识库（已校对）', 'confirm'], '确认是显式用户动作，不是转写返回即提交');
  assert.ok(!events.includes('submit'), '确认动作本身绝不触发 Submit');
});

test('cancelling dictation from the New entry only cancels the recording surface, never the goal draft (AC1)', async () => {
  const { NewTaskScreen } = await import('./screens/NewTaskScreen.tsx');
  hooks().__reset();
  const events: string[] = [];
  const baseState = {
    draft: { text: '手写目标', agentId: 'a-general', budgetUpper: 0, attachments: [] as never[], knowledgeIds: [] as string[] },
    agents: [], knowledge: [],
    recommendation: { agent: null, basis: 'none' },
    readiness: { ready: true, blockingAttachments: [] },
    loading: false, submitting: false, inFlight: undefined,
  };
  const callbacks = {
    onUpdate: () => {}, onSetAttachments: () => {}, onToggleKnowledge: () => {},
    onSubmit: () => { events.push('submit'); }, onCancel: () => { events.push('keep-draft'); }, onRefreshAgents: () => {},
  };
  const element = render(NewTaskScreen, {
    state: baseState,
    ...callbacks,
    dictation: { phase: 'recording' },
    onDictationBegin: () => {}, onDictationFinish: () => { events.push('finish'); },
    onDictationCancel: () => { events.push('cancel'); }, onDictationEditTranscript: () => {},
    onDictationRetryTranscription: () => {}, onDictationConfirmTranscript: () => {}, onDictationDiscardTranscript: () => {},
  });
  const goalInput = descendants(element).find(({ type, props }) => type === 'TextInput' && props.placeholder === '今天想完成什么？');
  assert.ok(goalInput, '录音态下手打目标输入仍在屏');
  assert.equal((goalInput!.props as { value?: string }).value, '手写目标', '手打文字原样保留（不被清空）');
  const buttons = descendants(element).filter(({ type }) => type === 'Button').map(({ props }) => props.title);
  assert.equal(buttons.includes('Cancel recording'), true);
  assert.equal(buttons.includes('Stop dictation'), true);
  (descendants(element).find(({ type, props }) => type === 'Button' && props.title === 'Cancel recording')!.props.onPress as () => void)();
  assert.deepEqual(events, ['cancel'], '取消只作用于听写面；Submit/Keep draft 均未被触发');
});

test('a denied microphone leaves typing and submission fully usable (拒权不破坏文字输入)', async () => {
  const { NewTaskScreen } = await import('./screens/NewTaskScreen.tsx');
  hooks().__reset();
  const baseState = {
    draft: { text: '手写目标', agentId: 'a-general', budgetUpper: 0, attachments: [] as never[], knowledgeIds: [] as string[] },
    agents: [], knowledge: [],
    recommendation: { agent: null, basis: 'none' },
    readiness: { ready: true, blockingAttachments: [] },
    loading: false, submitting: false, inFlight: undefined,
  };
  const callbacks = {
    onUpdate: () => {}, onSetAttachments: () => {}, onToggleKnowledge: () => {},
    onSubmit: () => {}, onCancel: () => {}, onRefreshAgents: () => {},
  };
  const element = render(NewTaskScreen, {
    state: baseState,
    ...callbacks,
    dictation: { phase: 'denied' },
    onDictationBegin: () => {}, onDictationFinish: () => {}, onDictationCancel: () => {},
    onDictationEditTranscript: () => {}, onDictationRetryTranscription: () => {},
    onDictationConfirmTranscript: () => {}, onDictationDiscardTranscript: () => {},
  });
  const goalInput = descendants(element).find(({ type, props }) => type === 'TextInput' && props.placeholder === '今天想完成什么？');
  assert.ok(goalInput, '拒权态下手打目标输入仍在屏');
  assert.equal((goalInput!.props as { value?: string }).value, '手写目标', '手打文字原样保留（不清空）');
  assert.equal((goalInput!.props as { editable?: boolean }).editable, true, '输入未被锁死');
  const buttons = descendants(element).filter(({ type }) => type === 'Button').map(({ props }) => props.title);
  assert.equal(buttons.includes('Submit task'), true, '提交通道不受拒权影响');
  const text = descendants(element).filter(({ type }) => type === 'Text').flatMap(({ props }) => props.children).join(' ');
  assert.ok(text.includes('麦克风权限被拒绝'), '拒权有如实文案');
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/dictation-view.test.ts apps/mobile/src/app-smoke.test.tsx`
Expected: FAIL——`dictation-view.test.ts` 报 `Cannot find module './dictation-view.ts'`；`app-smoke.test.tsx` 三个新测试报 review/recording/denied 态元素缺失（`transcriptInput` undefined / `Cancel recording` 按钮缺失 / `麦克风权限被拒绝` 文案缺失）。

- [ ] **Step 3: 写最小实现**

创建 `apps/mobile/src/dictation-view.ts`：

```ts
import type { DictationFailure } from '@weknora/mobile-core';

/** 听写失败/拒权文案（对照 ATTENTION_RECEIPT_COPY 先例：值不得出现内部 wire 细节）。 */
export const DICTATION_FAILURE_COPY: Record<'denied' | DictationFailure, string> = {
  denied: '麦克风权限被拒绝；可直接输入文字，或在系统设置授权后重试。',
  'capture-failed': '录音不可用；可直接输入文字后重试。',
  'audio-too-large': '录音过大（超过 16 MiB）；请缩短录音。',
  'transcription-failed': '转写失败；可重试或放弃，已输入的文字不受影响。',
};

/**
 * AC2 落点：确认后的转写**追加**进目标输入（不覆盖手打文字——「拒权、取消和迟到转写不破坏
 * 文字输入」的确认侧对偶），并在 maxLength（GOAL_TEXT_MAX_LENGTH=500）处截断，保住
 * #36 的单条意图记录 SecureStore ~2048B 预算（NewTaskScreen.tsx:7-13 注释同源）。
 */
export function applyConfirmedDictation(existing: string, confirmed: string, maxLength: number): string {
  const joined = existing.trim() === '' ? confirmed : `${existing}\n${confirmed}`;
  return joined.length <= maxLength ? joined : joined.slice(0, maxLength);
}
```

修改 `apps/mobile/src/screens/NewTaskScreen.tsx`：

1）import 区追加：

```tsx
import type { DictationState } from '@weknora/mobile-core';
import { DICTATION_FAILURE_COPY } from '../dictation-view.ts';
```

2）`NewTaskScreenProps`（现 `:16-24`）末尾追加：

```tsx
  /** 听写状态（组合根无原生捕获 Adapter 时为 undefined——屏不渲染任何听写元素，fail closed）。 */
  dictation?: DictationState;
  onDictationBegin?(): void;
  onDictationFinish?(): void;
  onDictationCancel?(): void;
  onDictationEditTranscript?(text: string): void;
  onDictationRetryTranscription?(): void;
  onDictationConfirmTranscript?(): void;
  onDictationDiscardTranscript?(): void;
```

3）组件签名加入新 props：`export function NewTaskScreen({ state, onUpdate, onSetAttachments, onToggleKnowledge, onSubmit, onCancel, onRefreshAgents, dictation, onDictationBegin, onDictationFinish, onDictationCancel, onDictationEditTranscript, onDictationRetryTranscription, onDictationConfirmTranscript, onDictationDiscardTranscript }: NewTaskScreenProps)`

4）在目标输入超限提示行（现 `:46` `{state.draft.text.length >= GOAL_TEXT_MAX_LENGTH && ...}`）之后插入听写块：

```tsx
      {dictation !== undefined && dictation.phase === 'idle' && (
        <Button title="Dictate" disabled={state.loading || state.submitting} onPress={() => { onDictationBegin?.(); }} />
      )}
      {dictation !== undefined && dictation.phase === 'recording' && (
        <View>
          <Text>Recording…</Text>
          <Button title="Stop dictation" onPress={() => { onDictationFinish?.(); }} />
          <Button title="Cancel recording" onPress={() => { onDictationCancel?.(); }} />
        </View>
      )}
      {dictation !== undefined && dictation.phase === 'transcribing' && (
        <View>
          <Text>Transcribing…</Text>
          <Button title="Cancel transcription" onPress={() => { onDictationCancel?.(); }} />
        </View>
      )}
      {dictation !== undefined && dictation.phase === 'review' && (
        <View>
          <Text>转写草稿（确认前可编辑）</Text>
          <TextInput
            value={dictation.transcript ?? ''}
            onChangeText={(text) => { onDictationEditTranscript?.(text); }}
            placeholder="转写草稿（确认前可编辑）"
            multiline
            maxLength={GOAL_TEXT_MAX_LENGTH}
            editable={!state.loading && !state.submitting}
          />
          <Button title="Use transcript" disabled={state.submitting} onPress={() => { onDictationConfirmTranscript?.(); }} />
          <Button title="Discard transcript" onPress={() => { onDictationDiscardTranscript?.(); }} />
        </View>
      )}
      {dictation !== undefined && dictation.phase === 'denied' && (
        <View>
          <Text>{DICTATION_FAILURE_COPY.denied}</Text>
          <Button title="Retry dictation" disabled={state.loading || state.submitting} onPress={() => { onDictationBegin?.(); }} />
        </View>
      )}
      {dictation !== undefined && dictation.phase === 'failed' && (
        <View>
          <Text>{dictation.failure !== undefined ? DICTATION_FAILURE_COPY[dictation.failure] : ''}</Text>
          {dictation.failure === 'transcription-failed' && (
            <Button title="Retry transcription" onPress={() => { onDictationRetryTranscription?.(); }} />
          )}
          <Button title="Dismiss" onPress={() => { onDictationCancel?.(); }} />
        </View>
      )}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test apps/mobile/src/dictation-view.test.ts apps/mobile/src/app-smoke.test.tsx`
Expected: PASS（dictation-view 3 个 + app-smoke 全部既有与 3 个新测试全过，0 fail）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/dictation-view.ts apps/mobile/src/dictation-view.test.ts apps/mobile/src/screens/NewTaskScreen.tsx apps/mobile/src/app-smoke.test.tsx
git commit -m "feat(issue30-sweep): new-task dictation view surface and confirm-into-draft (T26 #56 task 3)"
```

---

### Task 4: apps/mobile——原生捕获 Adapter + 组合根与 /new 路由接线

**Files:**
- Create: `apps/mobile/src/adapters/dictation-capture.ts`
- Test: `apps/mobile/src/adapters/dictation-capture.test.ts`
- Modify: `apps/mobile/src/composition.ts`（import 区 + `activeTaskMaterial()` 之后追加）
- Modify: `apps/mobile/src/app/new.tsx`（import + NewTaskRouteLifecycle 接线 + 卸载兜底）
- Test: `apps/mobile/src/app-smoke.test.tsx`（末尾追加源级守卫测试）

**Interfaces:**
- Consumes: Task 1 `createDictation`/`Dictation`/`DictationCapturePort`（`@weknora/mobile-core`）；Task 2 `createMobileVoiceTranscriptionRemote`（`@weknora/api-client/mobile/voice`）；#36 `createNativeRequestId`（`apps/mobile/src/adapters/request-id.ts`）；组合根既有 `cachePut`/`deploymentScopeKey`（`apps/mobile/src/composition.ts:134-150`）；`MobileRuntime.authorizedRequest(input: RuntimeAuthorizedRequest): Promise<unknown>`（`packages/mobile-core/src/runtime/types.ts:55`——FormData body 经 `body?: unknown` 透传）。
- Produces（Task 5 与 #57/#69/#70 依赖）:
  - `createDictationCaptureFrom(expoAudio: ExpoAudioLike): DictationCapturePort`（可测试的注入构造）；`createNativeDictationCaptureIfAvailable(): DictationCapturePort | undefined`（require `expo-audio` 失败/缺失导出 → undefined，fail closed）；`ExpoAudioLike` 导出类型。
  - composition：`dictationFor(activeRuntime: MobileRuntime, origin: string, tenantId: string): Dictation | undefined`（包内私有，deployment scope 记忆化）与 `activeDictation(): Dictation | undefined`（公开导出——无授权面或无原生捕获时 undefined）。
  - `/new` 路由：听写 props 全量接线 + `applyConfirmedDictation(controller.state().draft.text, confirmed, GOAL_TEXT_MAX_LENGTH)` 并入 + 卸载兜底（recording/transcribing 在途时 `cancel()`；review 状态留在记忆化模块实例，回屏可继续确认）。
  - 真机构建前置（#69/#70）：`cd apps/mobile && npx expo install expo-audio`。

- [ ] **Step 1: 写失败测试**

创建 `apps/mobile/src/adapters/dictation-capture.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createDictationCaptureFrom, createNativeDictationCaptureIfAvailable, type ExpoAudioLike } from './dictation-capture.ts';

interface FakeOptions {
  /** 权限结果（缺省 granted）。 */
  permission?: boolean;
  /** prepareToRecordAsync 必失败（录音机占用等）。 */
  failPrepare?: boolean;
}

/** 手写 fake（expo-audio 未安装于 Node 链——这正是 createNativeDictationCaptureIfAvailable 被测的惰性边界）。 */
function fakeExpoAudio(options: FakeOptions = {}): ExpoAudioLike {
  return {
    requestRecordingPermissionsAsync: async () => ({ granted: options.permission ?? true }),
    setAudioModeAsync: async () => undefined,
    RecordingPresets: { HIGH_QUALITY: { extension: '.m4a' } },
    AudioModule: {
      AudioRecorder: class {
        uri: string | null = null;
        async prepareToRecordAsync() {
          if (options.failPrepare === true) throw new Error('microphone busy');
          this.uri = 'file:///cache/dictation.m4a';
        }
        record(): void { /* fake */ }
        async stop(): Promise<void> { /* fake */ }
        release(): void { /* fake */ }
      },
    },
  } as unknown as ExpoAudioLike;
}

test('the native adapter is unavailable in the Node test chain (fail closed, no microphone surface)', () => {
  assert.equal(createNativeDictationCaptureIfAvailable(), undefined, 'expo-audio 不可 require 时必须返回 undefined');
});

test('granted permission records and stop returns the native file source', async () => {
  const capture = createDictationCaptureFrom(fakeExpoAudio());
  assert.equal(await capture.start(), 'recording');
  const audio = await capture.stop();
  assert.deepEqual(audio, { uri: 'file:///cache/dictation.m4a', mimeType: 'audio/mp4', fileName: 'dictation.m4a' });
});

test('denied permission never prepares a recorder', async () => {
  const capture = createDictationCaptureFrom(fakeExpoAudio({ permission: false }));
  assert.equal(await capture.start(), 'denied');
  assert.equal(await capture.stop(), undefined, '拒权路径零捕获，stop 无音频');
});

test('cancel stops and releases the recorder (raw audio is dropped, not transcribed)', async () => {
  const capture = createDictationCaptureFrom(fakeExpoAudio());
  await capture.start();
  await capture.cancel();
  assert.equal(await capture.stop(), undefined, '取消后录音机已释放，无残留捕获');
});

test('stop without a live recorder is undefined, not a throw', async () => {
  const capture = createDictationCaptureFrom(fakeExpoAudio());
  assert.equal(await capture.stop(), undefined);
});

test('a recorder failure in start rejects (the module maps it to capture-failed)', async () => {
  const capture = createDictationCaptureFrom(fakeExpoAudio({ failPrepare: true }));
  await assert.rejects(() => capture.start(), /microphone busy/);
});
```

在 `apps/mobile/src/app-smoke.test.tsx` 末尾追加源级守卫（与既有 :929-934 的 `/new route reaches the Task Office` 守卫同风格，`readFileSync`/`resolve`/`workspaceRoot` 均为文件内既有绑定）：

```tsx
test('the /new route wires dictation through the composition root; confirmed text enters the goal draft, never the wire', async () => {
  const { readFileSync } = await import('node:fs');
  const routeSource = readFileSync(resolve(workspaceRoot, 'apps/mobile/src/app/new.tsx'), 'utf8');
  const compositionSource = readFileSync(resolve(workspaceRoot, 'apps/mobile/src/composition.ts'), 'utf8');
  const screenSource = readFileSync(resolve(workspaceRoot, 'apps/mobile/src/screens/NewTaskScreen.tsx'), 'utf8');
  assert.match(routeSource, /activeDictation\(\)/, '路由经组合根取听写模块（module-seams §10：Screen 不见 wire）');
  assert.match(routeSource, /confirmTranscript\(\)/, 'AC2：确认动作显式调用模块的 confirmTranscript');
  assert.match(routeSource, /applyConfirmedDictation/, '确认文字必须经 applyConfirmedDictation 并入目标草稿（含 500 字截断）');
  assert.match(routeSource, /dictation\.cancel\(\)/, '卸载兜底：录音/转写在途时取消（不留悬空麦克风与在途派发）');
  assert.equal(/mobile\/voice/.test(routeSource) || /mobile\/voice/.test(screenSource), false, '屏/路由不出现 wire 路径');
  assert.match(compositionSource, /createMobileVoiceTranscriptionRemote/, '转写 Remote 只在组合根装配');
  assert.match(compositionSource, /createNativeDictationCaptureIfAvailable/, '原生捕获 Adapter 只在组合根探测');
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/adapters/dictation-capture.test.ts apps/mobile/src/app-smoke.test.tsx`
Expected: FAIL——`dictation-capture.test.ts` 报 `Cannot find module './dictation-capture.ts'`；app-smoke 新守卫测试失败（new.tsx 无 `activeDictation` 接线）。

- [ ] **Step 3: 写最小实现**

创建 `apps/mobile/src/adapters/dictation-capture.ts`：

```ts
import type { DictationAudio, DictationCapturePort } from '@weknora/mobile-core';

/**
 * 原生听写捕获 Adapter（module-seams §8.3 Audio Device Port）。system audio 属 true
 * external（spec Testing Decisions）：Node 测试链/未安装 expo-audio 的构建 fail closed
 * 返回 undefined（组合根随之隐藏麦克风入口，登录与手打输入完全不受影响）；真机麦克风
 * 验收属 #69/#70。真机构建前置：cd apps/mobile && npx expo install expo-audio。
 * expo-audio 的命令式面（SDK 55，ExpoAudio.ts）：AudioModule.AudioRecorder 构造 +
 * prepareToRecordAsync(RecordingPresets.HIGH_QUALITY) + record()/stop() + uri；
 * HIGH_QUALITY 预设输出 m4a/AAC；AudioRecorder 是 SharedObject，release() 释放。
 */
export interface AudioRecorderLike {
  uri: string | null;
  prepareToRecordAsync(options?: unknown): Promise<void>;
  record(options?: unknown): void;
  stop(): Promise<void>;
  release?(): void;
}

export interface ExpoAudioLike {
  requestRecordingPermissionsAsync(): Promise<{ granted?: boolean }>;
  setAudioModeAsync?(mode: Record<string, unknown>): Promise<void>;
  RecordingPresets?: { HIGH_QUALITY?: unknown };
  AudioModule: { AudioRecorder: new (options?: unknown) => AudioRecorderLike };
}

/** 注入式构造（测试注入 fake；与 device-identity 的 SecureStorePort 注入同型）。 */
export function createDictationCaptureFrom(expoAudio: ExpoAudioLike): DictationCapturePort {
  let recorder: AudioRecorderLike | undefined;
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
      const uri = instance.uri;
      if (typeof uri !== 'string' || uri === '') return undefined;
      return { uri, mimeType: 'audio/mp4', fileName: 'dictation.m4a' };
    },
    async cancel() {
      const instance = recorder;
      recorder = undefined;
      if (instance === undefined) return;
      try {
        await instance.stop();
      } catch {
        /* 已经停止的录音机：吞掉，路径仍是丢弃 */
      }
      instance.release?.(); // 原始音频默认删除（CONTEXT.md:339）：释放即弃临时文件
    },
  };
}

/** 原生组合路径（惰性 require；解析失败/缺导出 fail closed → undefined）。 */
export function createNativeDictationCaptureIfAvailable(): DictationCapturePort | undefined {
  try {
    const expoAudio = require('expo-audio') as ExpoAudioLike;
    if (typeof expoAudio.requestRecordingPermissionsAsync !== 'function' || expoAudio.AudioModule?.AudioRecorder === undefined) {
      return undefined;
    }
    return createDictationCaptureFrom(expoAudio);
  } catch {
    return undefined;
  }
}
```

修改 `apps/mobile/src/composition.ts`：

1）import 区（现有 `@weknora/api-client/mobile/*` import 之后）追加：

```ts
import { createDictation } from '@weknora/mobile-core';
import type { Dictation } from '@weknora/mobile-core';
import { createMobileVoiceTranscriptionRemote } from '@weknora/api-client/mobile/voice';
import { createNativeDictationCaptureIfAvailable } from './adapters/dictation-capture.ts';
```

2）模块顶部单例区（`nativeScopedVault` 之后）追加：

```ts
/** 听写捕获原生 Adapter 单例（组合根唯一探测点；不可用时全 App 无麦克风入口，fail closed）。 */
const nativeDictationCapture = createNativeDictationCaptureIfAvailable();
```

3）`activeTaskMaterial()` 之后追加：

```ts
const dictations = new Map<string, Dictation>();

/** 听写 Dictation 按 deployment scope key 记忆化（同 taskOfficeFor 模式）：转写 multipart
 * FormData 体经 Runtime.authorizedRequest 原样透传（RuntimeAuthorizedRequest.body →
 * client.ts:196 → transport/json.ts:136 FormData 直传 fetch，已核实）；无原生捕获 Adapter
 * 时返回 undefined（fail closed）。 */
function dictationFor(activeRuntime: MobileRuntime, origin: string, tenantId: string): Dictation | undefined {
  if (nativeDictationCapture === undefined) return undefined;
  return cachePut(dictations, deploymentScopeKey(origin, tenantId), () => createDictation({
    capture: nativeDictationCapture,
    transcribe: createMobileVoiceTranscriptionRemote({ origin, request: (input) => activeRuntime.authorizedRequest(input) }),
    newRequestId: createNativeRequestId(),
  }));
}

/** /new 路由经此取当前授权 scope 的听写模块（无授权面或无原生捕获时 undefined）。 */
export function activeDictation(): Dictation | undefined {
  const activeRuntime = runtime();
  const snapshot = activeRuntime.snapshot();
  if (snapshot.surface !== 'authorized' || !snapshot.deployment || !snapshot.identity?.userId) return undefined;
  return dictationFor(activeRuntime, snapshot.deployment.origin, snapshot.identity.activeTenantId ?? '');
}
```

修改 `apps/mobile/src/app/new.tsx`：

1）import 区追加/扩展（`composition.ts` 既有 import 行 `import { activeMobileRuntime, activeTaskOffice, openScopedDraftStore } from '../composition.ts';` 改为含 `activeDictation`；`import { NewTaskScreen } from '../screens/NewTaskScreen.tsx';` 改为含 `GOAL_TEXT_MAX_LENGTH`；另起两行新 import）：

```tsx
import { activeMobileRuntime, activeDictation, activeTaskOffice, openScopedDraftStore } from '../composition.ts';
import { NewTaskScreen, GOAL_TEXT_MAX_LENGTH } from '../screens/NewTaskScreen.tsx';
import { applyConfirmedDictation } from '../dictation-view.ts';
import type { DictationState } from '@weknora/mobile-core';
```

2）`NewTaskRouteLifecycle` 内（`const [controller, setController] = useState...` 之后）追加：

```tsx
  const dictation = activeDictation();
  const [dictationState, setDictationState] = useState<DictationState | undefined>(dictation?.state());
  useEffect(() => (dictation === undefined ? undefined : dictation.subscribe(setDictationState)), [dictation]);
  // 卸载兜底：录音/转写在途时停止麦克风与在途派发（迟到结果由模块代次守卫丢弃）；
  // review 转写草稿留在记忆化模块实例中，返回 /new 可继续编辑确认（不丢已转写文本）。
  useEffect(() => () => {
    if (dictation === undefined) return;
    const phase = dictation.state().phase;
    if (phase === 'recording' || phase === 'transcribing') void dictation.cancel();
  }, [dictation]);
```

3）`<NewTaskScreen ...>` props 追加（现有 7 个 prop 之后）：

```tsx
      dictation={dictationState}
      onDictationBegin={() => { if (dictation !== undefined) void dictation.begin(); }}
      onDictationFinish={() => { if (dictation !== undefined) void dictation.finish(); }}
      onDictationCancel={() => { if (dictation !== undefined) void dictation.cancel(); }}
      onDictationEditTranscript={(text) => { dictation?.editTranscript(text); }}
      onDictationRetryTranscription={() => { if (dictation !== undefined) void dictation.retryTranscription(); }}
      onDictationConfirmTranscript={() => {
        if (dictation === undefined) return;
        const confirmed = dictation.confirmTranscript();
        if (confirmed === undefined) return;
        controller.update({ text: applyConfirmedDictation(controller.state().draft.text, confirmed, GOAL_TEXT_MAX_LENGTH) });
      }}
      onDictationDiscardTranscript={() => { dictation?.discardTranscript(); }}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test apps/mobile/src/adapters/dictation-capture.test.ts apps/mobile/src/app-smoke.test.tsx && pnpm --filter @weknora/mobile typecheck`
Expected: PASS（dictation-capture 6 个 + app-smoke 全部测试 0 fail；typecheck 无输出通过——含 NewTaskScreen 新 props、composition 新工厂、MobileVoiceTranscriptionRemote→DictationTranscriptionPort 结构可赋值）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/adapters/dictation-capture.ts apps/mobile/src/adapters/dictation-capture.test.ts apps/mobile/src/composition.ts apps/mobile/src/app/new.tsx apps/mobile/src/app-smoke.test.tsx
git commit -m "feat(issue30-sweep): wire dictation into composition root and /new route (T26 #56 task 4)"
```

---

### Task 5: apps/mobile——opt-in 真实 HTTP 集成证据（AC3）

**Files:**
- Create: `apps/mobile/src/voice-dictation-integration-smoke.ts`
- Test: `apps/mobile/src/voice-dictation-integration-smoke.test.ts`

**Interfaces:**
- Consumes: Task 1 `createDictation`；Task 2 `createMobileVoiceTranscriptionRemote`；#36 `createNewTaskController`（`apps/mobile/src/new-task-view.ts:53`）、`createNativeRequestId`（`apps/mobile/src/adapters/request-id.ts`）；Task 3 `applyConfirmedDictation`、`GOAL_TEXT_MAX_LENGTH`；`createMobileRuntime`/`createInMemoryCredentialStore`/`CLIENT_PROTOCOL_VERSION`/`createWeKnoraClient`/`createMobileRuntimeRemote`/`createJsonTransport`（Task 3/4 同链）；`disallowedDeploymentHost(hostname: string, variable: string): string | undefined`（`apps/mobile/src/runtime-integration-smoke.ts:118` 公开导出）。
- Produces（审计与 #57/#69/#70 复用）:
  - `voiceDictationIntegrationConfig(env: Record<string, string | undefined>): VoiceDictationIntegrationConfig`（opt-in：`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD` 三变量 + HTTPS/无凭据 origin 校验 + `disallowedDeploymentHost` 公网主机防线）。
  - `runVoiceDictationIntegration(config): Promise<VoiceDictationIntegrationEvidence>`——总化收口（任何异常也产出证据对象，含 `errorReason`，绝不让函数 reject 丢证据；不含凭据）。
  - `VoiceDictationIntegrationEvidence = { deploymentOrigin: string; recordingCancelled: boolean; cancelKeptGoalText: boolean; dictationNeverSubmitted: boolean; transcription: 'transcribed' | 'charging-unconfigured' | 'failed'; confirmedTextLength?: number; editedBeforeConfirm?: boolean; errorReason?: string; timestamp: string }`。
  - `emitVoiceDictationIntegrationEvidence(evidence, emit): void`。

- [ ] **Step 1: 写失败测试**

创建 `apps/mobile/src/voice-dictation-integration-smoke.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  emitVoiceDictationIntegrationEvidence,
  runVoiceDictationIntegration,
  voiceDictationIntegrationConfig,
} from './voice-dictation-integration-smoke.ts';

const env = () => process.env as Record<string, string | undefined>;

test('integration stays opt-in: missing credentials skip and a private/localhost host is invalid, never a pass', () => {
  const missing = voiceDictationIntegrationConfig({ ...env(), WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: undefined });
  assert.equal(missing.enabled, false);
  assert.equal(missing.disposition, 'skip');
  for (const host of ['http://insecure.example', 'https://localhost', 'https://127.0.0.1', 'https://10.0.0.5']) {
    const invalid = voiceDictationIntegrationConfig({
      WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: host,
      WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
      WEKNORA_MOBILE_TEST_PASSWORD: ['short-lived', 'secret'].join('-'),
    });
    assert.equal(invalid.enabled, false, `${host} 不得成为被授权目标`);
    assert.equal(invalid.disposition, 'invalid');
  }
});

test('live dictation end to end: cancel keeps the typed goal, confirm is explicit, nothing is ever submitted (opt-in)', async (t) => {
  const config = voiceDictationIntegrationConfig(env());
  if (!config.enabled) {
    t.skip(config.reason);
    return;
  }
  const evidence = await runVoiceDictationIntegration(config);
  const emitted: string[] = [];
  emitVoiceDictationIntegrationEvidence(evidence, (record) => { emitted.push(record); });
  assert.equal(emitted.length, 1);
  assert.equal(JSON.parse(emitted[0]!).transcription, evidence.transcription);
  assert.equal(evidence.recordingCancelled, true, 'AC1：取消录音路径已真实执行');
  assert.equal(evidence.cancelKeptGoalText, true, 'AC1：取消不破坏手写目标文本');
  assert.equal(evidence.dictationNeverSubmitted, true, 'AC2：听写全程零 Task 提交');
  assert.ok(
    evidence.transcription === 'transcribed' || evidence.transcription === 'charging-unconfigured' || evidence.transcription === 'failed',
    '转写结果如实记录（未配置语音计价的部署如实记 charging-unconfigured，不伪造）',
  );
  assert.doesNotMatch(JSON.stringify(evidence), /short-lived-secret|password|token|email/i, '证据不含凭据');
});

test('the integration runner is total: a failing transport still yields evidence, not a rejection', async () => {
  const evidence = await runVoiceDictationIntegration({ enabled: true, deploymentOrigin: 'https://weknora.invalid.test', email: 'nobody@example.test', password: 'wrong' });
  assert.equal(typeof evidence.errorReason, 'string');
  assert.equal(evidence.dictationNeverSubmitted, false, '登录失败路径：提交计数未被消费，字段如实为 false 而非伪造 true');
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/voice-dictation-integration-smoke.test.ts`
Expected: FAIL——`Cannot find module './voice-dictation-integration-smoke.ts'`。

- [ ] **Step 3: 写最小实现**

创建 `apps/mobile/src/voice-dictation-integration-smoke.ts`：

```ts
import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileVoiceTranscriptionRemote } from '@weknora/api-client/mobile/voice';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createDictation, createInMemoryCredentialStore, createMobileRuntime, type TaskStartReceipt } from '@weknora/mobile-core';
import { createNativeRequestId } from './adapters/request-id.ts';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';
import { createNewTaskController } from './new-task-view.ts';
import { applyConfirmedDictation } from './dictation-view.ts';
import { GOAL_TEXT_MAX_LENGTH } from './screens/NewTaskScreen.tsx';

export type VoiceDictationIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

/** 与 task-start-integration-smoke.ts 相同的 opt-in 语义 + disallowedDeploymentHost 主机防线（自包含，不跨计划 import）。 */
export function voiceDictationIntegrationConfig(env: Record<string, string | undefined>): VoiceDictationIntegrationConfig {
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

export interface VoiceDictationIntegrationEvidence {
  deploymentOrigin: string;
  /** AC1：取消路径真实执行（scripted 捕获 granted → recording → cancel → idle）。 */
  recordingCancelled: boolean;
  /** AC1：取消后手写目标文本逐字保留。 */
  cancelKeptGoalText: boolean;
  /** AC2：整个听写流（含取消与确认）期间 Task start 通道零调用。 */
  dictationNeverSubmitted: boolean;
  /** 转写结果如实记录：成功 / 部署未配置语音计价（503）/ 失败。 */
  transcription: 'transcribed' | 'charging-unconfigured' | 'failed';
  confirmedTextLength?: number;
  editedBeforeConfirm?: boolean;
  errorReason?: string;
  timestamp: string;
}

/** 1 秒 8kHz 16bit 单声道静音 WAV（确定性合成，无外部 fixture；真实 multipart 字节上载）。 */
function silentWavBytes(): Uint8Array {
  const sampleRate = 8000;
  const samples = sampleRate; // 1 秒
  const buffer = new ArrayBuffer(44 + samples * 2);
  const view = new DataView(buffer);
  const ascii = (offset: number, text: string): void => {
    for (let i = 0; i < text.length; i += 1) view.setUint8(offset + i, text.charCodeAt(i));
  };
  ascii(0, 'RIFF'); view.setUint32(4, 36 + samples * 2, true); ascii(8, 'WAVE');
  ascii(12, 'fmt '); view.setUint32(16, 16, true); view.setUint16(20, 1, true); view.setUint16(22, 1, true);
  view.setUint32(24, sampleRate, true); view.setUint32(28, sampleRate * 2, true);
  view.setUint16(32, 2, true); view.setUint16(34, 16, true);
  ascii(36, 'data'); view.setUint32(40, samples * 2, true);
  return new Uint8Array(buffer);
}

/**
 * 真实端到端（AC3）：生产 JSON transport + Runtime 授权通道 + 具体 Remote Adapter +
 * Dictation 模块 + 真实 New 控制器（提交通道以 spy 断言零调用——AC2 的行为证据）。
 * 捕获是 scripted Adapter（spec：system audio 属 true external，真机验收 #69/#70）。
 * 服务器若未配置 voice admission，如实记录 charging-unconfigured（合法结论，不伪造转写成功）。
 */
export async function runVoiceDictationIntegration(config: Extract<VoiceDictationIntegrationConfig, { enabled: true }>): Promise<VoiceDictationIntegrationEvidence> {
  const evidence: VoiceDictationIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    recordingCancelled: false,
    cancelKeptGoalText: false,
    dictationNeverSubmitted: false,
    transcription: 'failed',
    timestamp: new Date().toISOString(),
  };
  const fetcher: FetchLike = (input, init) => fetch(input, init as RequestInit);
  const runtime = createMobileRuntime({
    credentialStore: createInMemoryCredentialStore(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    remoteFor(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return createMobileRuntimeRemote({ origin, request: client.request });
    },
    authorizedTransport(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return (input, accessToken) => client.request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
    },
  });
  let submissions = 0;
  try {
    const snapshot = await runtime.signIn({
      deployment: { origin: config.deploymentOrigin, label: 'Voice dictation integration' },
      email: config.email,
      password: config.password,
    });
    if (snapshot.surface !== 'authorized') {
      evidence.errorReason = `surface ${snapshot.surface}`;
      return evidence;
    }
    const controller = createNewTaskController({
      office: {
        start: async (): Promise<TaskStartReceipt> => {
          submissions += 1;
          throw new Error('dictation must never submit a task');
        },
        reconcilePending: async () => [],
      },
      agents: async () => [],
      newRequestId: createNativeRequestId(),
    });
    await controller.whenInitialized();
    const typedGoal = '手写目标：周报';
    controller.update({ text: typedGoal });

    const dictation = createDictation({
      capture: {
        start: async () => 'recording',
        stop: async () => ({ bytes: silentWavBytes(), mimeType: 'audio/wav', fileName: 'dictation.wav' }),
        cancel: async () => undefined,
      },
      transcribe: createMobileVoiceTranscriptionRemote({
        origin: config.deploymentOrigin,
        request: (input) => runtime.authorizedRequest(input),
      }),
      newRequestId: createNativeRequestId(),
    });

    // AC1 live：取消录音不取消 Task（草稿原样、零转写派发、零提交）
    await dictation.begin();
    if (dictation.state().phase === 'recording') {
      await dictation.cancel();
      evidence.recordingCancelled = dictation.state().phase === 'idle';
      evidence.cancelKeptGoalText = controller.state().draft.text === typedGoal;
    }

    // 转写 live：真实 multipart POST /api/v1/mobile/voice/transcriptions
    await dictation.begin();
    await dictation.finish();
    const settled = dictation.state();
    if (settled.phase === 'review' && typeof settled.transcript === 'string') {
      const edited = `${settled.transcript.trim()}（已校对）`;
      dictation.editTranscript(edited);
      const confirmed = dictation.confirmTranscript();
      if (confirmed !== undefined) {
        evidence.transcription = 'transcribed';
        evidence.editedBeforeConfirm = confirmed === edited;
        controller.update({ text: applyConfirmedDictation(controller.state().draft.text, confirmed, GOAL_TEXT_MAX_LENGTH) });
        evidence.cancelKeptGoalText = evidence.cancelKeptGoalText && controller.state().draft.text.startsWith(typedGoal);
        evidence.confirmedTextLength = controller.state().draft.text.length;
      }
    } else if (settled.failureCode === 'VOICE_CHARGING_UNCONFIGURED') {
      evidence.transcription = 'charging-unconfigured';
    } else {
      evidence.transcription = 'failed';
      evidence.errorReason = `${settled.failure ?? 'unknown'}${settled.failureCode === undefined ? '' : ` (${settled.failureCode})`}`;
    }
    evidence.dictationNeverSubmitted = submissions === 0;
    return evidence;
  } catch (error) {
    evidence.transcription = 'failed';
    evidence.errorReason = error instanceof Error ? error.message : String(error); // 失败仍产出证据（不含凭据）
    // dictationNeverSubmitted 保持初始 false：听写流未走完时不得以「提交计数为 0」伪造 AC2 证据。
    return evidence;
  } finally {
    runtime.dispose(); // 释放 lease/凭据通道（先例：task-start-integration-smoke.ts:125）
  }
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitVoiceDictationIntegrationEvidence(evidence: VoiceDictationIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test apps/mobile/src/voice-dictation-integration-smoke.test.ts`
Expected: PASS——本地无凭据时：第 1 个测试 PASS（skip/invalid 语义）、第 2 个测试 SKIP（`t.skip`，如实不伪造）、第 3 个测试 PASS（`https://weknora.invalid.test` 登录失败 → total 化证据）。0 fail。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/voice-dictation-integration-smoke.ts apps/mobile/src/voice-dictation-integration-smoke.test.ts
git commit -m "test(issue30-sweep): opt-in real-http voice dictation integration evidence (T26 #56 task 5)"
```

---

## 计划级验证

在 worktree 根执行（覆盖本计划全部测试面 + 服务端既有语音面复跑 + 类型检查；刻意排除全量 flaky 套件）：

```bash
go test ./internal/handler/ -run 'TestVoice|TestTranscription|TestPriceVersion' -count=1 && pnpm exec tsx --test packages/mobile-core/src/voice/dictation.test.ts packages/api-client/src/mobile/voice.test.ts apps/mobile/src/dictation-view.test.ts apps/mobile/src/adapters/dictation-capture.test.ts apps/mobile/src/voice-dictation-integration-smoke.test.ts apps/mobile/src/app-smoke.test.tsx && pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck
```

预期：Go `ok`；tsx 六组测试 0 fail（集成冒烟 live 用例本地 `skip` 属合法）；`@weknora/mobile test` 全绿（含 app-smoke 既有测试 + 本计划新增 4 个：3 渲染 + 1 源级守卫）；typecheck 无输出通过。

## 本计划 Produces 摘要（后续计划 Consumes）

- **#57（实时 Voice Room）**：`DictationTranscriptionPort`/`DictationAudio`/`createScenarioDictationTranscriber`（`@weknora/mobile-core`）可直接作为 Voice Room 内「听写时刻」的转写 seam；`createMobileVoiceTranscriptionRemote` 的 `voiceSessionId` 入参已为绑定语音 session 的转写留好通道（服务端 `mobile_voice.go:503-520` 语义）。
- **#69/#70（真机验收）**：真机构建前置 `cd apps/mobile && npx expo install expo-audio`；`createNativeDictationCaptureIfAvailable` 为唯一麦克风入口（权限拒绝始终 fail closed 为 `'denied'`）；真机验收口径：拒权/录音/转写/确认/取消五路径。
- **审计**：`VoiceDictationIntegrationEvidence` 证据契约（`emitVoiceDictationIntegrationEvidence` 单行 JSON，不含凭据）。
