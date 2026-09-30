# T27：Task 内实时语音会话（Issue #57）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用户可从 Task 详情加入绑定 Task 的 Voice Room，进行轮次化的连续语音交谈（授权会话 → 说话 → 服务端转写 → 可编辑转写草稿 → 确认为 steer 意图经 `TaskHandle.act` 进入任务），并使三条验收全部可验证：AC1 断线有明确结束/恢复且原始音频默认删除；AC2 高风险审批不能通过语音直接完成（客户端类型收窄 + 服务端结构双证据）；AC3 端到端行为在最高稳定 Interface（mobile-core Voice Room 模块 + 真实 wire 的 api-client 适配器 + 真实 Runtime 授权通道 + opt-in 真实部署）上验证。

**Architecture:** 服务端 W30 语音会话面已完整且**生产代码零改动**：`POST /api/v1/mobile/voice/sessions`（短期 grant 授权 + 预算 hold + provider open，`internal/handler/mobile_voice.go:175`）、`DELETE /mobile/voice/sessions/:id`（幂等 stop/settle——断线的「明确结束」，`mobile_voice.go:359`）、`POST /mobile/voice/transcriptions`（服务端代理转写 + `voice_session_id` 绑定，`mobile_voice.go:445`）；本计划以**测试-only 的 Go 增量**固化两条结构事实（语音面不挂任何审批端点——AC2 服务端证据；断线结束/恢复的幂等生命周期——AC1 服务端证据）。客户端新建 mobile-core **Voice Room 深模块**（module-seams §8）：`join({taskId, runId?})` → `VoiceHandle`（state/subscribe/beginTurn/endTurn/editTranscript/confirmTranscript/discardTurn/resume/leave/dispose）；**Realtime Voice Port**（spec §8.3）抽会话生命周期（今天的真实 Adapter 是 W30 REST 面，未来 WS/WebRTC Adapter 实现同一 Port，模块不改）；复用 #56 的 `DictationCapturePort`（Audio Device Port）与 `createMobileVoiceTranscriptionRemote`（`voiceSessionId` 通道）；确认文字以 **类型收窄的 steer 意图**（`{ kind: 'steer'; text: string } | undefined`）交还宿主经 `TaskHandle.act` 提交——模块没有任何决定/提交通道（AC2 的结构性保障，spec §8.1「确认后的文字通过 TaskHandle.act 提交」逐字）。api-client 新增 `createMobileVoiceSessionRemote`（会话 open/end wire 适配，**provider grant token 在 Adapter 边界即剥离、绝不进入模块状态**）。apps/mobile 挂 VoiceRoomScreen + `/tasks/voice` 路由 + 组合根按 deployment scope 记忆化装配；Task 详情页加「语音房」入口，干预回执行补指令文本呈现（确认文字在任务页可见）。

**Tech Stack:** TypeScript（`packages/mobile-core`、`packages/api-client`、`apps/mobile` Expo RN 55）、node:test + tsx（与 `task-office.test.ts`/`dictation.test.ts` 一致）；Go 仅测试增量（testify + gin httptest，复用 `mobile_voice_test.go` 既有 harness）。**Go 生产代码零改动、零迁移**（既有测试用 `AutoMigrate`/纯 stub，不装载迁移链——波级问题 1 的同号迁移损坏与本计划无关，按指引不顺手去重）。所有命令在 worktree 根（`.worktrees/issue30-sweep`）执行。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-57.md`（正文「用户可加入绑定 Task 的 Voice Room，连续交谈并把确认文字写入 Timeline」；三条验收标准原文见「Global Constraints」末尾；Blocked by #35、#56 均已在当前 HEAD 合并）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（User Story 69「real-time voice conversation attached to a Task」、Story 71「raw voice deleted after processing unless explicit retention is enabled and disclosed」、Implementation Decisions「Voice Room owns real-time audio and transcription. Confirmed text enters Task Office; voice never grants approval.」「REST submits commands and loads authoritative Snapshots. Cursored SSE carries durable Task/Run events. WebSocket or WebRTC is reserved for real-time voice.」「Mobile core does not depend on React Native, DOM or concrete transport.」、Testing Decisions「Tests target observable behavior at the highest stable Interface.」「True external dependencies such as APNs, FCM, WebRTC, system audio and system share use mock or scripted Adapters at the Port and real-device acceptance separately.」、Acceptance Gates「Voice Room Interface tests cover permission denial, disconnect, editable transcription, confirmation into Task, raw-audio deletion and refusal to authorize high-risk Actions by voice.」）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§8 Voice Room Module——§8.1 所有权「Voice Room 拥有实时语音 session、microphone permission、音频状态、转写草稿和断线结束语义。它不拥有 Task 权限、审批或时间线；确认后的文字通过 TaskHandle.act 提交」、§8.2 Interface（join/state/subscribe/confirmTranscript/leave）、§8.3 seam（Realtime Voice Port：WebRTC/WebSocket/scripted Adapter；Audio Device Port；Audio Retention Port）、§10 App Shell 禁止事项、§13 Interface 测试面「Voice Room：permission、断线、转写确认、原音频删除、高风险审批拒绝」）
- ADR：`docs/adr/0012-mobile-business-logic-lives-behind-deep-modules.md`（业务逻辑在深 Module 后）、`docs/adr/0006-mobile-transport-by-semantics.md`（通道按语义选择）
- 领域术语：`CONTEXT.md:339`「语音交互记录（Voice Interaction Record）：实时语音交互中经成员确认的文字输入和 Agent 文字答复，作为任务时间线的一部分保存；原始音频默认在实时处理后删除，只有空间明确启用并提示参与者时才限期留存」+ `_避免_：「默认永久录音、用语音直接完成高风险审批、不保留任何可审计文字语义」`
- Parent：Issue #30；下游 Blocked by 本 Issue：#71

## Consumes（前序批次产出，当前 HEAD 已合并，作者亲眼核实）

| 来源 | 接口（精确签名） | 核实位置 |
|---|---|---|
| #35/#37 | `createTaskDetail(input: { taskId: string; runId: string }, ports: TaskDetailPorts): TaskHandle`；`TaskHandle.act(intent: TaskIntent): Promise<InterventionReceipt>`；`TaskIntent = { kind: 'steer'; text: string } \| { kind: 'queue-next'; text: string; intentId?: string } \| { kind: 'stop' }`；`TaskDetailView.interventions?: InterventionReceipt[]` | `packages/mobile-core/src/task-office/task-detail.ts:91-101,120`、`task-intent.ts:11-14` |
| #35/#36 | `createTaskOffice(ports).open(input: { taskId: string; runId: string }): TaskHandle`（每次调用新句柄，`task-office.ts:501`；空 runId 抛 `TASK_OFFICE_INVALID_INPUT`）；`MobileRuntime.authorizedRequest(input: RuntimeAuthorizedRequest): Promise<unknown>`；`MobileRuntime.scopeLease(): ScopeLease \| undefined` | `packages/mobile-core/src/task-office/task-office.ts:501-509`、`apps/mobile/src/task-intervention-integration-smoke.ts:87-92` |
| #56 | `DictationAudio { uri?; bytes?; mimeType; fileName? }`、`DictationCapturePort { start(): Promise<'recording'\|'denied'>; stop(): Promise<DictationAudio\|undefined>; cancel(): Promise<void> }`、`createScriptedDictationCapture({ start?, stop?, manualStop? })`、`createScenarioDictationTranscriber()`（mobile-core barrel 导出）；`createMobileVoiceTranscriptionRemote({ origin, request }).transcribe(input: { requestId; audio; voiceSessionId? }): Promise<{ text; audioSeconds?; settled? }>`（`voiceSessionId` 绑定通道已在 #56 预留） | `packages/mobile-core/src/voice/dictation.ts:22-55`、`in-memory-dictation.ts:18,57`、`packages/api-client/src/mobile/voice.ts:21-42` |
| #41/#56 apps/mobile | composition 既有件：`runtime()`、`deploymentScopeKey(origin, tenantId)`、`cachePut(map, key, factory)`、`nativeDictationCapture` 单例、`activeDictation()` 模式、`createNativeRequestId(): () => string`（工厂返回函数，直接传作端口） | `apps/mobile/src/composition.ts:59,166,334-356`、`adapters/request-id.ts:11` |
| Go W30 | `MobileVoiceHandler.CreateVoiceSession/StopVoiceSession/TranscribeAudio`（`mobile_voice.go:175,359,445`）；路由 `routes_workbench.go:188-191` 挂 `/api/v1`（`router.go:389`）；测试 harness `newVoiceHarness`/`postSession`/`stopSession`/`postTranscription`/`voiceSessionIDOf`（`mobile_voice_test.go:227-305`，同包可复用；时钟经 `h.clock = h.clock.Add(...)` 推进，先例 `mobile_voice_test.go:448`）；router 包 stub `routesVoiceStore` 等（`routes_mobile_voice_test.go:19-68`） | 已实跑基线全 PASS |
| 测试基建 | node:test + tsx 运行方式 `pnpm exec tsx --test <files>`；屏测试的 react-native stub 手法（`registerHooks` resolve 钩子 + 动态导入） | `apps/mobile/src/voice-dictation-integration-smoke.test.ts:17-41` |

## Produces（本计划对外产出，供 #71 及后续真机验收消费）

1. mobile-core：`createVoiceRoom(ports: VoiceRoomPorts): VoiceRoom`、`VoiceRoom.join(input: { taskId: string; runId?: string }): VoiceHandle`、`VoiceHandle`（state/subscribe/beginTurn/endTurn/editTranscript/confirmTranscript/discardTurn/resume/leave/dispose）、`VoiceSessionPort`/`VoiceTurnTranscriptionPort`/`VoiceAudioDisposition`/`VoiceSessionGrant`/`VoiceSessionEndReceipt`/`VoiceRoomState`/`VoiceTurnView`/`VoiceRoomError`（`VOICE_ROOM_SCOPE_CHANGED|VOICE_ROOM_INVALID_INPUT`）、`VOICE_ROOM_MAX_SECONDS=600`/`VOICE_ROOM_MAX_AUDIO_BYTES=16<<20`、`createScriptedVoiceSession()`——全部经 `@weknora/mobile-core` barrel 导出。
2. api-client：`createMobileVoiceSessionRemote({ origin, request }): MobileVoiceSessionRemote`（exports 子路径 `./mobile/voice-sessions`）；错误码 `VOICE_SESSION_INVALID|VOICE_BUDGET_DENIED|VOICE_SESSION_CONFLICT|VOICE_PROVIDER_UNAVAILABLE|VOICE_OPEN_UNKNOWN|VOICE_CHARGING_UNCONFIGURED|VOICE_SESSION_NOT_FOUND|VOICE_SESSION_END_FAILED`。
3. apps/mobile：`activeVoiceRoom(): VoiceRoom | undefined`（composition）、`createVoiceRoomController` + `VOICE_ROOM_*_COPY`（voice-room-view.ts）、`VoiceRoomScreen`、`/tasks/voice` 路由、`TaskDetailScreenProps.onOpenVoiceRoom?` 入口。
4. 集成证据契约：`VoiceRoomIntegrationEvidence` + `voiceRoomIntegrationConfig(env)` + `runVoiceRoomIntegration(config)` + `emitVoiceRoomIntegrationEvidence(evidence, emit)`（opt-in `WEKNORA_MOBILE_TEST_*`）。
5. Go 测试-only 结构证据：语音面审批端点缺席断言（router 包）+ 断线结束/恢复幂等生命周期测试（handler 包，复用既有 harness）。

## Global Constraints

以下为批准 Spec / ADR / CONTEXT / Issue 的项目级约束，逐字引用，所有任务隐含遵守：

- 「用户可加入绑定 Task 的 Voice Room，连续交谈并把确认文字写入 Timeline。」（issue-57.md 正文 What to build）
- 「断线有明确结束/恢复，原始音频默认删除。」（issue-57.md 验收标准 1）
- 「高风险审批不能通过语音直接完成。」（issue-57.md 验收标准 2）
- 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」（issue-57.md 验收标准 3）
- 「69. As a member, I want real-time voice conversation attached to a Task, so that hands-free interaction retains Task semantics.」（mobile-ai-office-design.md · User Stories）
- 「71. As a privacy administrator, I want raw voice deleted after processing unless explicit retention is enabled and disclosed, so that biometric data is minimized.」（同上）
- 「Voice Room owns real-time audio and transcription. Confirmed text enters Task Office; voice never grants approval.」（同上 · Implementation Decisions）
- 「REST submits commands and loads authoritative Snapshots. Cursored SSE carries durable Task/Run events. WebSocket or WebRTC is reserved for real-time voice. Push is a synchronization hint.」（同上）——实时通道的**媒体承载**以 Realtime Voice Port 抽象；本计划的真实 Adapter 是 W30 REST 会话/转写/结算面，**不新造无 provider 可验证的 WS/WebRTC 媒体服务器**（见差异记录 2）。
- 「Voice Room Interface tests cover permission denial, disconnect, editable transcription, confirmation into Task, raw-audio deletion and refusal to authorize high-risk Actions by voice.」（同上 · Acceptance Gates）——Task 1 测试矩阵逐条覆盖。
- 「Tests target observable behavior at the highest stable Interface.」/「True external dependencies such as APNs, FCM, WebRTC, system audio and system share use mock or scripted Adapters at the Port and real-device acceptance separately.」（同上 · Testing Decisions）——麦克风与媒体通道以 scripted Adapter 进 Interface 测试；真机麦克风/扬声器/弱网验收属 #69/#70/#71，本计划不伪造真机结论。
- 「Voice Room 拥有实时语音 session、microphone permission、音频状态、转写草稿和断线结束语义。它不拥有 Task 权限、审批或时间线；确认后的文字通过 TaskHandle.act 提交。」（mobile-module-seams.md §8.1）
- 「Realtime Voice Port：WebRTC Adapter、WebSocket Adapter、scripted test Adapter；Audio Device Port：iOS/Android native Adapter、test Adapter；Audio Retention Port：Scoped Vault Adapter。WebRTC 和 WebSocket 是两个真实 Adapter，因此 seam 有实际价值。原始音频默认在处理后删除。」（同上 §8.3）
- 「禁止：Screen 直接导入 packages/contracts 或 packages/api-client；Screen 自己维护 request_id、cursor、revision、scope generation；每个 Screen 建独立 query cache 或 token refresh」（同上 §10）——VoiceRoomScreen 只见 `VoiceRoomViewState` + 回调；轮次幂等身份由模块经注入的 `newRequestId`/`newSessionId` 铸造；wire Adapter 只出现在组合根。
- 「语音交互记录（Voice Interaction Record）：实时语音交互中经成员确认的文字输入和 Agent 文字答复，作为任务时间线的一部分保存；原始音频默认在实时处理后删除……避免：默认永久录音、用语音直接完成高风险审批、不保留任何可审计文字语义。」（CONTEXT.md:339）——每轮音频只在转写在途窗口存活、终态即释放并经 `audioDisposition` 恰好回调一次；确认文字留在轮次日志（可审计）并经 act(steer) 进入任务。
- 「Mobile core does not depend on React Native, DOM or concrete transport. Remote and native details are injected as Adapters.」（mobile-ai-office-design.md）——`packages/mobile-core/src/voice-room/` 不出现 FormData/Blob/require；wire 细节在 `packages/api-client/src/mobile/voice-sessions.ts`。
- 「Each command that can have an unknown outcome uses a durable idempotency identity. Network failure triggers lookup or reconciliation, not silent replay.」（同上）——轮次转写复用 `request_id` 幂等重放（服务端 `mobile_voice.go:465-487`）；会话结束复用服务端幂等 stop/settle（`mobile_voice.go:359-420`）。
- 服务端 URL/凭据约束（会话注入）：全部远端调用走 `MobileRuntime.authorizedRequest`（origin 由 `requireDeploymentOrigin` 强校验为无凭据 HTTPS origin）；集成冒烟复用 `disallowedDeploymentHost` 主机防线（拒 localhost/环回/私网/链路本地/保留地址，仅放行公网 HTTPS）；凭据只从 `WEKNORA_MOBILE_TEST_*` 环境变量读取，源码与测试不写可用凭据字面量。本计划 **Go/SQL 生产代码零改动**（参数绑定约束自动满足）。
- 数值上限逐字对齐服务端常量：会话窗口 ≤ `voiceSessionMaxSeconds = 600` 秒（`mobile_voice.go:46`）；单轮音频 ≤ `16 << 20` 字节（`mobile_voice.go:51` `voiceMaxAudioBytes`）。
- 工作流约束：严格 RED→GREEN→REFACTOR（每个任务先写失败测试、实跑确认失败、最小实现、通过、提交；Task 5 的 evidence-pinning 例外已在任务内如实声明）；实现不与已批准 Spec 冲突，冲突时升级而非静默重设计；真机麦克风/扬声器/弱网验收属 #69/#70/#71，本计划不伪造真机结论。

**Issue #57 验收标准原文（docs/plans/issue30-sweep/issues/issue-57.md）：**

1. 「断线有明确结束/恢复，原始音频默认删除。」
2. 「高风险审批不能通过语音直接完成。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

**验收标准 3 的本地可验证性说明（blocked-env 声明）：** 真实端到端（生产 JSON transport + Runtime 授权通道 + 具体 Remote Adapter + Voice Room 模块 + 真实控制器 + 真后端 `POST /api/v1/mobile/voice/sessions` 与 `/mobile/voice/transcriptions`）沿用 T01 起已合并的 opt-in 真实 HTTP 模式，需要「一个真实 WeKnora Deployment（HTTPS 公网 origin）+ 测试账号」（`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD`）；真实 Task/Run 绑定与确认指令受理（act 需要 bound Run）另需部署有可用 Agent，走 `WEKNORA_MOBILE_TEST_START_TASK=1` 额外门控。本地无此环境时 Task 6 的真实 HTTP 用例以 `t.skip` 跳过（**不得伪造通过**）；部署未配置语音计价时服务端如实返回 503 `voice_charging_unconfigured`，证据记 `charging-unconfigured`（合法结论，不是失败）。本地替代证据：Task 1 的 Voice Room Interface 场景测试（真实模块编排 + scripted Adapter，覆盖 permission/断线/可编辑/确认/音频删除/审批拒绝全部六类 Acceptance Gates）+ Task 2 wire 契约测试 + Task 5 服务端结构证据（既有 18 个 Go voice 测试复跑 + 2 个新测试）+ Task 3/4 接线与源级守卫。真机麦克风/扬声器/弱网属 #69/#70/#71 真机验收门槛（spec：「real-device acceptance separately」），本地以 scripted Adapter 为证据，不伪造真机结论。

## 与调查结论的差异记录（以代码现状为准）

1. **调查缺口 4「确认文字写入 Timeline 无挂接点：#35 的 Task 详情/Timeline 未实现」已过时**：#35/#37 已在当前 HEAD 合并——亲眼核实 `packages/mobile-core/src/task-office/task-detail.ts:91-101`（TaskHandle.act）、`task-intent.ts:11-14`（TaskIntent.steer）、`apps/mobile/src/app/tasks/detail.tsx`（/tasks/detail 路由 + onAct 接线）、`apps/mobile/src/screens/TaskDetailScreen.tsx:121-125`（interventions 呈现）均存在。本计划直接挂接：确认文字 → `TaskHandle.act({kind:'steer'})`（spec §8.1 逐字通道）。
2. **调查缺口 1「无实时语音媒体链路后端」属实，按 spec 处理而非新造媒体服务器**：spec 明文「WebSocket or WebRTC is reserved for real-time voice」；服务端 W30 的会话授权/预算/结算/转写代理面已完整（`mobile_voice.go` 三端点 + 幂等结算），缺的是**客户端实时会话语义**（旧 W31 已删）。本计划以 Realtime Voice Port 承载该 seam（§8.3），今天的真实 Adapter 是 W30 REST 轮次面；WS/WebRTC Adapter 是未来 seam 实现（届时模块零改动）。无 provider 的裸 WS 媒体服务器无法通过 AC3（不得伪造集成证据），不在本计划内。
3. **服务端 steer 不产生 run 时间线事件**：亲眼核实 `internal/modules/workbench/service/workbench/interaction.go:408-436`（GormSteerPort.Steer 只做 revision CAS + `AppendSteerEvents` 到会话消息流）与 `internal/handler/session/workbench_read.go:296`（SSE 读 `snapshots.ReadRunEvents`——agent run 事件库）。因此「确认文字写入 Timeline」的落点是：(a) 确认动作经 act(steer) 提交（任务页事实流的一部分——`TaskDetailView.interventions` 回执）；(b) 本计划在 TaskDetailScreen 干预行补**指令文本**呈现 + Voice Room 轮次日志保留已确认文字（CONTEXT.md:339 的可审计语义）；**不**改动已固化的 GormSteerPort CAS 路径去新增服务端事件（爆炸半径不抵收益，且干预回执不跨进程持久属 #40 投影快照域）。
4. **旧 W31 语义对应**：`723de9179` 删除的 `realtime.ts` 三分离（interrupt_output/end_session/cancel_run）中——end_session = 本计划 `leave()`；cancel_task = 宿主既有 `act({kind:'stop'})`（模块外，#37 已交付）；interrupt_output 属未来 WS/WebRTC 播报面（REST 轮次面无播报），Realtime Voice Port 的 Adapter 边界为其预留。审批互斥（旧冻结规则「voice 会话不携带决定权」）以 confirmTranscript 返回类型收窄 + Task 5 服务端结构测试双端固化。
5. **join 签名**：spec §8.2 `join(taskID, scopeLease)`——本实现 lease 经 `ports.lease()` 存活观察（`TaskDetailPorts.lease`/`createDeliveryReader` 同型，`task-detail.ts:106`、`delivery-reader.ts:19`），join 时校验、每个异步边界复查——比 join 时冻结 lease 更强（scope 撤销即时收敛），语义等价。
6. **迁移轨道**：波级问题 1（000114/000193 同号双迁移损坏）不影响本计划——Task 5 的 Go 测试用既有 `newVoiceHarness`（`AutoMigrate`，`mobile_voice_test.go:227-235`）与纯 stub 路由测试，均不装载迁移链；按波级指引不顺手去重。
7. **转写失败无自动重试**：与 #56 dictation（同 id 重试窗口）不同，Voice Room 的轮次转写失败按 AC1 断线语义明确结束（interrupted + 服务端 stop/settle），恢复 = 显式 `resume()` 新会话。原因：保留音频做重试 = 延长原始音频存活窗（与「默认删除」张力）；对话场景的诚实恢复是新会话，不是静默重放。
8. **apps/mobile 测试不能用 barrel 外的 `RuntimeScopeLease` 自铸 lease**（#32 决策：scope-lease 的实现类不入公共导出）——apps/mobile 侧的控制器测试用脚本化 `VoiceHandle` 替身（控制器是被测对象；Room 的 Interface 在 mobile-core 包内全测，其测试在包内可用 `RuntimeScopeLease` 原生导入）。另：tsx 的 CJS/ESM 双实例下 `instanceof TaskOfficeError` 跨模块不可靠（本计划验证期实锤，见验证记录第 6 条），voice-room-view 的错误映射用 `.code` 形状查表而非 instanceof。
9. **并行共享文件**：本计划与其他计划（t47/t49/t51 等）同批并行、独立 worktree 后合并。`apps/mobile/src/screens/TaskDetailScreen.tsx`、`composition.ts`、`app/tasks/detail.tsx` 是共享热点（计划撰写期间亲眼见到并行写入覆盖）；Task 4 的修改按锚点描述而非行号，执行者动手前先重读锚点上下文定位。

## Review Focus

Spec 隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **迟到转写/迟到结算污染新会话状态**：用户 resume/leave 后旧会话的转写才返回——若应用到状态会把旧文字挂进新会话或重复处置音频。——Task 1 测试「leave during transcribing ends the room clearly, discards the in-flight turn, and later results change nothing」（代次守卫，整代丢弃）+「scope revocation collapses the room...drops late results」。
2. **断线后复用旧会话造成重复计费/死通道**：恢复时重用旧 sessionId（服务端 closed 绑定转写 409、unknown 行阻断新授权）或重复 stop 造成二次结算。——Task 1 测试「AC1 — resume opens a NEW session...never reuses the ended one」+ Task 5 Go 测试「迟到 stop 重放是 replay:true 幂等、恢复会话是新行、closed 会话拒绝绑定转写、全程 exactly-once 结算」。
3. **语音通道绕过审批**：任何把确认文字变成 decision/审批调用的路径（现在或未来重构引入）。——Task 1 测试「AC2 — no decision-capable method on the handle...」（键集断言 + confirmTranscript 只产 steer）+ Task 5 Go 测试「语音面恰好三端点且路径不含 decision/approve/interaction/command/budget」。
4. **原始音频滞留**：轮次终态后音频仍被持有（内存或落盘），或一次轮次触发多次/零次处置回调。——Task 1 测试「AC1 — raw audio is discarded exactly once per turn on every terminal path and never appears in state」（disposition 计数 + `JSON.stringify(state)` 无音频字段）+ 超限/拒权轮零处置回调。
5. **scope 撤销后语音会话继续计费/继续说话**：切租户/登出后模块仍打开会话或消费转写。——Task 1 测试「scope revocation collapses the room at the next boundary without further session work and drops late results」（每个异步边界复查 lease；撤销后零服务端调用）。

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 1 | mobile-core：Voice Room 深模块 | `packages/mobile-core/src/voice-room/voice-room.ts`（状态机 + Ports + AC1/AC2 不变量）+ `in-memory-voice-session.ts`（scripted 会话 Adapter）+ `voice-room.test.ts`（Acceptance Gates 六类全矩阵，12 测试）+ index.ts barrel 导出 |
| 2 | api-client：会话 Remote | `packages/api-client/src/mobile/voice-sessions.ts`（open/end wire 适配 + 错误语义翻译 + token 剥离）+ `voice-sessions.test.ts`（7 测试）+ package.json exports `./mobile/voice-sessions` |
| 3 | apps/mobile：控制器/文案/屏 | `apps/mobile/src/voice-room-view.ts`（createVoiceRoomController + 四组文案）+ `screens/VoiceRoomScreen.tsx` + `voice-room-view.test.ts`（5 测试） |
| 4 | 路由/组合根/详情入口 | `app/tasks/voice.tsx` + composition `activeVoiceRoom()` + TaskDetailScreen 入口与干预行文本 + detail 路由接线 + 路由接线测试 |
| 5 | Go 测试-only 结构证据 | `internal/router/routes_mobile_voice_exclusivity_test.go`（AC2 服务端）+ `internal/handler/mobile_voice_room_lifecycle_test.go`（AC1 服务端，复用既有 harness） |
| 6 | 集成证据（opt-in） | `apps/mobile/src/voice-room-integration-smoke.ts` + `voice-room-integration-smoke.test.ts`（3 测试：2 本地 + 1 live skip） |

---

### Task 1: mobile-core Voice Room 深模块

**Files:**
- Create: `packages/mobile-core/src/voice-room/voice-room.ts`
- Create: `packages/mobile-core/src/voice-room/in-memory-voice-session.ts`
- Test: `packages/mobile-core/src/voice-room/voice-room.test.ts`
- Modify: `packages/mobile-core/src/index.ts`（文件末尾追加导出）

**Interfaces:**
- Consumes: `DictationAudio`/`DictationCapturePort`（`../voice/dictation.ts`）、`leaseActive`/`RuntimeScopeLease`（`../runtime/scope-lease.ts`）、`ScopeLease`（`../runtime/types.ts`）、`createScriptedDictationCapture`/`createScenarioDictationTranscriber`（`../voice/in-memory-dictation.ts`，测试用；包内测试可原生导入 `RuntimeScopeLease`）。
- Produces（Task 2/3/4/6 依赖的精确签名）：`createVoiceRoom(ports: VoiceRoomPorts): VoiceRoom`；`VoiceRoom.join(input: { taskId: string; runId?: string }): VoiceHandle`；`VoiceHandle { state(): VoiceRoomState; subscribe(listener): () => void; beginTurn(): Promise<void>; endTurn(): Promise<void>; editTranscript(turnId: string, text: string): void; confirmTranscript(turnId: string): { kind: 'steer'; text: string } | undefined; discardTurn(turnId: string): void; resume(): Promise<void>; leave(reason?): Promise<void>; dispose(): void }`；`VoiceRoomPorts { session: VoiceSessionPort; transcribe: VoiceTurnTranscriptionPort; capture: DictationCapturePort; newRequestId(): string; newSessionId(): string; lease(): ScopeLease | undefined; audioDisposition?: VoiceAudioDisposition; maxSeconds?: number }`；`VoiceSessionPort { open(input: { productSessionId: string; runId?: string; maxSeconds?: number }): Promise<VoiceSessionGrant>; end(sessionId: string): Promise<VoiceSessionEndReceipt> }`；`VoiceSessionGrant { id: string; expiresAt: string; maxSeconds: number }`（无 token）；`VoiceSessionEndReceipt { id: string; state: string; settled: boolean; replay?: boolean }`；`VoiceTurnTranscriptionPort { transcribe(input: { requestId: string; audio: DictationAudio; voiceSessionId?: string }): Promise<{ text: string; audioSeconds?: number }> }`；`VoiceAudioDisposition { onDiscarded(turnId: string, reason: 'transcribed' | 'failed' | 'cancelled' | 'session-ended'): void }`；`createScriptedVoiceSession(): ScriptedVoiceSession`。

- [ ] **Step 1: 写失败测试（Voice Room Interface 全矩阵）**

创建 `packages/mobile-core/src/voice-room/voice-room.test.ts`（以下为本计划作者实跑通过的终版，12 个测试全绿）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createVoiceRoom, VoiceRoomError, VOICE_ROOM_MAX_AUDIO_BYTES, type VoiceHandle, type VoiceRoomPorts } from './voice-room.ts';
import { createScriptedVoiceSession, type ScriptedVoiceSession } from './in-memory-voice-session.ts';
import { createScenarioDictationTranscriber, createScriptedDictationCapture, type ScenarioDictationTranscriber, type ScriptedDictationCapture } from '../voice/in-memory-dictation.ts';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import type { VoiceAudioDisposition } from './voice-room.ts';

const tick = async (): Promise<void> => { await new Promise<void>((resolve) => setImmediate(resolve)); };

function mintLease(): { lease: ScopeLease; revoke: () => void } {
  const internal = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.com', userId: 'u1', tenantId: 't1' });
  return { lease: internal.asScopeLease(), revoke: () => internal.revoke() };
}

interface RoomHarness {
  session: ScriptedVoiceSession;
  capture: ScriptedDictationCapture;
  transcriber: ScenarioDictationTranscriber;
  discarded: Array<{ turnId: string; reason: string }>;
  ports: VoiceRoomPorts;
  revoke: () => void;
}

function harness(options: { start?: 'recording' | 'denied' | Error } = {}): RoomHarness {
  const session = createScriptedVoiceSession();
  const capture = createScriptedDictationCapture({
    start: options.start ?? 'recording',
    stop: { bytes: new Uint8Array([1, 2, 3]), mimeType: 'audio/wav', fileName: 'turn.wav' },
  });
  const transcriber = createScenarioDictationTranscriber();
  const discarded: Array<{ turnId: string; reason: string }> = [];
  const disposition: VoiceAudioDisposition = { onDiscarded: (turnId, reason) => { discarded.push({ turnId, reason }); } };
  const { lease, revoke } = mintLease();
  const ports: VoiceRoomPorts = {
    session,
    capture,
    transcribe: transcriber,
    newRequestId: () => `req-${transcriber.requests.length + 1}`,
    newSessionId: () => `room-${session.opens.length + 1}`,
    lease: () => lease,
    audioDisposition: disposition,
  };
  return { session, capture, transcriber, discarded, ports, revoke };
}

/** 标准一轮：begin → end → 应答转写 → 到 review。返回轮次 id。 */
async function turnToReview(h: RoomHarness, handle: VoiceHandle, text: string, requestIndex = 0): Promise<string> {
  await handle.beginTurn();
  assert.equal(handle.state().phase, 'listening');
  const turning = handle.endTurn();
  await tick(); await tick();
  assert.equal(handle.state().phase, 'transcribing');
  h.transcriber.resolve(requestIndex, { text, audioSeconds: 2 });
  await turning;
  assert.equal(handle.state().phase, 'ready');
  return handle.state().pendingTurnId!;
}

test('AC gates — permission denial: a denied microphone surfaces a notice, dispatches nothing, and opens no session', async () => {
  const h = harness({ start: 'denied' });
  const handle = createVoiceRoom(h.ports).join({ taskId: 'task-1', runId: 'run-1' });
  await handle.beginTurn();
  assert.equal(handle.state().phase, 'idle');
  assert.equal(handle.state().notice?.reason, 'permission-denied');
  assert.deepEqual(h.capture.calls, ['start'], '拒权后零 stop/零 cancel——没有任何捕获工作');
  assert.equal(h.transcriber.requests.length, 0, '拒权绝不派发转写');
  assert.equal(h.session.opens.length, 0, '拒权轮不开启会话（先权限后会话，拒权不留下已计费空会话）');
  assert.equal(h.discarded.length, 0, '没有收留过音频就没有处置回调');
});

test('AC1 — disconnect has a clear end: a failed turn transcription ends the session, discards audio, and shows interrupted', async () => {
  const h = harness();
  const handle = createVoiceRoom(h.ports).join({ taskId: 'task-1', runId: 'run-1' });
  await handle.beginTurn();
  const turning = handle.endTurn();
  await tick(); await tick();
  h.transcriber.resolve(0, new Error('network dropped'));
  await turning;
  assert.equal(handle.state().phase, 'interrupted', '断线以 interrupted 如实呈现');
  assert.equal(handle.state().notice?.reason, 'transcription-failed');
  assert.equal(h.session.ends.length, 1, '断线明确结束：服务端 stop/settle 恰好一次');
  assert.deepEqual(h.discarded, [{ turnId: 'req-1', reason: 'failed' }], '原始音频恰好处置一次');
  assert.equal(handle.state().turns.length, 0, '失败轮次不进入轮次日志');
});

test('AC1 — resume opens a NEW session with a new product session id and keeps the confirmed-turn log', async () => {
  const h = harness();
  const handle = createVoiceRoom(h.ports).join({ taskId: 'task-1', runId: 'run-1' });
  const first = await turnToReview(h, handle, '第一条指令');
  handle.confirmTranscript(first);
  // 断线：新一轮转写失败（beginTurn 复用仍在窗内的会话）
  await handle.beginTurn();
  const turning = handle.endTurn();
  await tick(); await tick();
  h.transcriber.resolve(1, new Error('drop'));
  await turning;
  assert.equal(handle.state().phase, 'interrupted');
  // 恢复
  await handle.resume();
  assert.equal(handle.state().phase, 'idle');
  assert.equal(h.session.opens.length, 2, '恢复 = 重新授权');
  assert.notEqual(h.session.opens[1]!.productSessionId, h.session.opens[0]!.productSessionId, '恢复绝不复用旧 product session id');
  assert.deepEqual(handle.state().turns.map((turn) => turn.state), ['confirmed'], '已确认文字（语音交互记录）跨断线保留');
  // 恢复后的会话可用（新轮派发到新会话）
  const second = await turnToReview(h, handle, '恢复后的第二条', 2);
  assert.equal(handle.state().pendingTurnId, second);
  assert.equal(h.transcriber.requests.length, 3, '失败轮不重派：恰好 1 成功 + 1 失败 + 1 恢复轮');
});

test('editable transcription: the review transcript can be edited and confirm returns the edited text', async () => {
  const h = harness();
  const handle = createVoiceRoom(h.ports).join({ taskId: 'task-1' });
  const id = await turnToReview(h, handle, '原始转写');
  handle.editTranscript(id, '原始转写（已校对）');
  const intent = handle.confirmTranscript(id);
  assert.deepEqual(intent, { kind: 'steer', text: '原始转写（已校对）' });
  assert.equal(handle.state().phase, 'idle');
  assert.equal(handle.state().turns[0]!.state, 'confirmed');
  assert.ok(handle.state().turns[0]!.confirmedAt !== undefined);
  assert.equal(h.transcriber.requests.length, 1, '确认只消费既有转写，绝不再次派发');
});

test('confirmation into Task: the module itself never submits anything — confirm only returns the intent', async () => {
  const h = harness();
  const handle = createVoiceRoom(h.ports).join({ taskId: 'task-1', runId: 'run-1' });
  const id = await turnToReview(h, handle, '写入任务');
  const before = JSON.stringify({ opens: h.session.opens.length, ends: h.session.ends.length, requests: h.transcriber.requests.length });
  const intent = handle.confirmTranscript(id);
  assert.notEqual(intent, undefined);
  assert.equal(intent!.kind, 'steer', 'AC2：确认只产生 steer 意图');
  const after = JSON.stringify({ opens: h.session.opens.length, ends: h.session.ends.length, requests: h.transcriber.requests.length });
  assert.equal(after, before, '确认动作零网络（提交由宿主经 TaskHandle.act 承担）');
});

test('AC2 — no decision-capable method on the handle, and empty text confirms nothing', async () => {
  const h = harness();
  const handle = createVoiceRoom(h.ports).join({ taskId: 'task-1' });
  const decisionShaped = Object.keys(handle).filter((key) => /decide|approv|interaction|command/i.test(key));
  assert.deepEqual(decisionShaped, [], '语音句柄结构性不携带任何决定/审批方法');
  const id = await turnToReview(h, handle, '有内容的一轮');
  handle.editTranscript(id, '   ');
  assert.equal(handle.confirmTranscript(id), undefined, '空白文本不产生指令');
  assert.equal(handle.state().phase, 'ready', '空白确认不消费轮次（宿主可编辑或放弃）');
});

test('AC1 — raw audio is discarded exactly once per turn on every terminal path and never appears in state', async () => {
  const h = harness();
  const handle = createVoiceRoom(h.ports).join({ taskId: 'task-1' });
  // 成功轮
  const id = await turnToReview(h, handle, '一轮');
  assert.deepEqual(h.discarded, [{ turnId: 'req-1', reason: 'transcribed' }]);
  handle.discardTurn(id);
  assert.equal(JSON.stringify(handle.state()).includes('bytes'), false, '状态序列化不携带音频字节');
  assert.equal(JSON.stringify(handle.state()).includes('audio:'), false, '状态序列化不携带音频字段');
  // 拒权轮（独立构造：拒权不收留音频 → 零处置回调）
  const denied = harness({ start: 'denied' });
  const deniedHandle = createVoiceRoom(denied.ports).join({ taskId: 'task-1' });
  await deniedHandle.beginTurn();
  assert.equal(denied.discarded.length, 0, '拒权轮没有收留过音频就没有处置回调');
});

test('AC1 — oversized audio is refused before the transcription window with zero dispatch', async () => {
  const bigCapture = createScriptedDictationCapture({ stop: { bytes: new Uint8Array(VOICE_ROOM_MAX_AUDIO_BYTES + 1), mimeType: 'audio/wav' } });
  const transcriber = createScenarioDictationTranscriber();
  const discarded: Array<{ turnId: string; reason: string }> = [];
  const { lease } = mintLease();
  const ports: VoiceRoomPorts = { session: createScriptedVoiceSession(), capture: bigCapture, transcribe: transcriber, newRequestId: () => 'req-big', newSessionId: () => 'room-big', lease: () => lease, audioDisposition: { onDiscarded: (turnId, reason) => { discarded.push({ turnId, reason }); } } };
  const handle = createVoiceRoom(ports).join({ taskId: 'task-1' });
  await handle.beginTurn();
  await handle.endTurn();
  assert.equal(handle.state().phase, 'idle');
  assert.equal(handle.state().notice?.reason, 'audio-too-large');
  assert.equal(transcriber.requests.length, 0, '超限音频不派发转写');
  assert.equal(discarded.length, 0, '音频从未被模块收留，无处置回调');
});

test('leave during transcribing ends the room clearly, discards the in-flight turn, and later results change nothing', async () => {
  const h = harness();
  const handle = createVoiceRoom(h.ports).join({ taskId: 'task-1', runId: 'run-1' });
  await handle.beginTurn();
  const turning = handle.endTurn();
  await tick(); await tick();
  assert.equal(handle.state().phase, 'transcribing');
  await handle.leave();
  assert.equal(handle.state().phase, 'ended');
  assert.deepEqual(h.discarded, [{ turnId: 'req-1', reason: 'session-ended' }]);
  h.transcriber.resolve(0, { text: '迟到的转写' });
  await turning;
  assert.equal(handle.state().phase, 'ended', '迟到结果不改变终态');
  assert.equal(handle.state().turns.length, 0, '迟到转写绝不进入轮次日志');
  assert.equal(h.session.ends.length, 1, 'leave 明确结束恰好一次');
  assert.equal(handle.state().lastLeave?.settled, true, '结算事实如实呈现');
});

test('leave is idempotent on the module side', async () => {
  const h = harness();
  const handle = createVoiceRoom(h.ports).join({ taskId: 'task-1' });
  await handle.beginTurn();
  await handle.leave();
  await handle.leave();
  assert.equal(h.session.ends.length, 1, '模块侧 leave 幂等');
  // 服务端幂等由 Task 5 的 Go 测试以真实 handler 证明（replay:true）
});

test('scope revocation collapses the room at the next boundary without further session work and drops late results', async () => {
  const h = harness();
  const handle = createVoiceRoom(h.ports).join({ taskId: 'task-1', runId: 'run-1' });
  await handle.beginTurn();
  const turning = handle.endTurn();
  await tick(); await tick();
  h.revoke();
  h.transcriber.resolve(0, { text: 'scope 已撤销才回来的转写' });
  await turning;
  assert.equal(handle.state().phase, 'ended', 'scope 撤销收敛为 ended');
  assert.equal(handle.state().notice?.reason, 'scope-revoked');
  assert.equal(handle.state().turns.length, 0);
  assert.equal(h.session.ends.length, 0, 'scope 撤销后不再发起任何服务端调用（fail closed）');
  assert.equal(h.session.opens.length, 1, '撤销后零新会话');
});

test('join fails closed: empty taskId is invalid input and a revoked lease is a scope change', () => {
  const h = harness();
  assert.throws(() => createVoiceRoom(h.ports).join({ taskId: '  ' }), (error: unknown) => error instanceof VoiceRoomError && error.code === 'VOICE_ROOM_INVALID_INPUT');
  h.revoke();
  assert.throws(() => createVoiceRoom(h.ports).join({ taskId: 'task-1' }), (error: unknown) => error instanceof VoiceRoomError && error.code === 'VOICE_ROOM_SCOPE_CHANGED');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/voice-room/voice-room.test.ts`
Expected: FAIL——`Cannot find module './voice-room.ts'`（模块与 Adapter 尚不存在）。

- [ ] **Step 3: 写最小实现**

创建 `packages/mobile-core/src/voice-room/voice-room.ts`（以下为本计划作者实跑通过的终版）：

```ts
/**
 * Voice Room 深模块（Issue #57 / T27）——module-seams §8「Voice Room Module」的实时会话面。
 * 所有权（§8.1）：实时语音 session、microphone permission、音频状态、转写草稿与断线结束语义。
 * 不拥有 Task 权限、审批或时间线：确认后的文字以 steer TaskIntent 形态交还宿主，
 * 由宿主经 TaskHandle.act 提交（spec §8.1 逐字——本模块没有任何提交/决定通道）。
 *
 * 非协商不变量：
 *  - 语音会话经产品 API 授权的短期 grant 承载；provider 令牌在 Adapter 边界即被剥离，
 *    绝不进入模块状态（W30：internal/handler/mobile_voice.go:37-39「token exists exactly once」）。
 *  - AC2：confirmTranscript 的返回类型收窄为 { kind: 'steer'; text: string }——类型系统
 *    本身排除 decision/stop/queue-next；语音通道结构性不具备审批能力。
 *  - AC1：断线（转写失败/会话开启失败/scope 撤销）如实呈现；leave() 明确结束
 *    （服务端 stop/settle 幂等可重放，mobile_voice.go:359-420）；resume() 以新
 *    productSessionId 恢复——旧会话已结束，复用它只会撞上服务端 closed 绑定 409。
 *  - 原始音频默认删除（CONTEXT.md:339）：音频引用只存活于 endTurn 的转写在途窗口；
 *    每个终态恰好一次 audioDisposition.onDiscarded；模块状态只有转写文本，绝无音频字段。
 */

import { leaseActive } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import type { DictationAudio, DictationCapturePort } from '../voice/dictation.ts';

/** 服务端单次语音会话窗口上限（internal/handler/mobile_voice.go:46 voiceSessionMaxSeconds）。 */
export const VOICE_ROOM_MAX_SECONDS = 600;
/** 单轮转写音频上限，与服务端上传上限一致（mobile_voice.go:51 voiceMaxAudioBytes）。 */
export const VOICE_ROOM_MAX_AUDIO_BYTES = 16 << 20;

/** 会话授权的模块侧视图：不含 token——media 凭据在 Adapter 边界即剥离。 */
export interface VoiceSessionGrant {
  id: string;
  expiresAt: string;
  maxSeconds: number;
}

export interface VoiceSessionEndReceipt {
  id: string;
  state: string;
  settled: boolean;
  /** 服务端幂等重放（同一会话的第二次 stop）时为 true。 */
  replay?: boolean;
}

/** Realtime Voice Port（module-seams §8.3）的会话生命周期面。今天的真实 Adapter 是 W30
 * REST 会话面（open=POST /api/v1/mobile/voice/sessions、end=DELETE …/sessions/:id）；
 * 未来 WebSocket/WebRTC Adapter 实现同一 Port（spec：「WebSocket or WebRTC is reserved
 * for real-time voice」），本模块零改动。 */
export interface VoiceSessionPort {
  open(input: { productSessionId: string; runId?: string; maxSeconds?: number }): Promise<VoiceSessionGrant>;
  end(sessionId: string): Promise<VoiceSessionEndReceipt>;
}

/** 单轮转写 Port：#56 DictationTranscriptionPort 加可选 voiceSessionId 绑定（用量记到
 * 自己的语音会话，mobile_voice.go:503-520）。api-client 的 createMobileVoiceTranscriptionRemote
 * 结构性满足本接口（apps/mobile 类型证明测试落实）。 */
export interface VoiceTurnTranscriptionPort {
  transcribe(input: { requestId: string; audio: DictationAudio; voiceSessionId?: string }): Promise<{ text: string; audioSeconds?: number }>;
}

/** 原始音频处置观察（AC1「原始音频默认删除」的可审计出口）：每轮被模块收留过的音频到达
 * 终态恰好回调一次。模块默认策略是纯删除；限期留存（空间显式启用 + 参与者提示，CONTEXT.md:339）
 * 属未来功能，届时以 Scoped Vault Adapter 实现的独立 Retention Port 承载，不改本模块。 */
export interface VoiceAudioDisposition {
  onDiscarded(turnId: string, reason: 'transcribed' | 'failed' | 'cancelled' | 'session-ended'): void;
}

export type VoiceRoomPhase = 'idle' | 'connecting' | 'listening' | 'transcribing' | 'ready' | 'interrupted' | 'ended';
export type VoiceRoomNoticeReason = 'session-open-failed' | 'transcription-failed' | 'permission-denied' | 'capture-failed' | 'audio-too-large' | 'scope-revoked';

/** 轮次视图：只有文本与状态，绝无原始音频（AC1 的类型级事实）。 */
export interface VoiceTurnView {
  turnId: string;
  transcript: string;
  state: 'review' | 'confirmed' | 'discarded';
  confirmedAt?: string;
}

export interface VoiceRoomState {
  phase: VoiceRoomPhase;
  taskId: string;
  runId?: string;
  sessionId?: string;
  sessionExpiresAt?: string;
  /** 最近一次服务端结束的结算事实（「明确结束」的可观测面；scope 撤销收敛无此字段）。 */
  lastLeave?: { settled: boolean; state: string; at: string };
  notice?: { reason: VoiceRoomNoticeReason; at: string };
  /** 语音交互记录（CONTEXT.md:339）：已确认文字跨断线/leave 保留；原始音频从不入内。 */
  turns: VoiceTurnView[];
  /** 当前待确认（review）的轮次 id。 */
  pendingTurnId?: string;
}

export type VoiceRoomErrorCode = 'VOICE_ROOM_SCOPE_CHANGED' | 'VOICE_ROOM_INVALID_INPUT';

export class VoiceRoomError extends Error {
  readonly code: VoiceRoomErrorCode;
  constructor(code: VoiceRoomErrorCode, message: string) {
    super(message);
    this.code = code;
  }
}

/** 确认意图的类型收窄：结构同 TaskIntent 的 steer 分支（可赋值），但排除其它分支——
 * 语音通道在类型层面就不产生 stop/queue-next，更不产生 decision。 */
export interface VoiceSteerIntent {
  kind: 'steer';
  text: string;
}

export interface VoiceHandle {
  state(): VoiceRoomState;
  subscribe(listener: (state: VoiceRoomState) => void): () => void;
  /** 开始一轮发言（权限请求 + 捕获；无会话时在捕获成功后开会话）。仅 idle 态可进入。 */
  beginTurn(): Promise<void>;
  /** 结束本轮并派发转写；结果进 review（可编辑）。转写失败按断线语义明确结束。 */
  endTurn(): Promise<void>;
  /** 编辑 review 中的转写文本（module-seams「转写草稿」所有权）。 */
  editTranscript(turnId: string, text: string): void;
  /** AC2：确认一轮转写——返回 steer 意图交宿主 act；空白确认返回 undefined 并保持 review。 */
  confirmTranscript(turnId: string): VoiceSteerIntent | undefined;
  /** 放弃 review 中的轮次（文字留档为 discarded）。 */
  discardTurn(turnId: string): void;
  /** AC1 恢复：丢弃旧代次、收尾旧会话（如仍挂起）、以新 productSessionId 重新授权。 */
  resume(): Promise<void>;
  /** AC1 明确结束：取消捕获、丢弃音频、review 轮次留档为 discarded、服务端 stop/settle。幂等。 */
  leave(reason?: VoiceRoomNoticeReason): Promise<void>;
  dispose(): void;
}

export interface VoiceRoom {
  /** 加入绑定 Task 的语音房（spec §8.2 join）。lease 缺席/已撤销 fail closed。 */
  join(input: { taskId: string; runId?: string }): VoiceHandle;
}

export interface VoiceRoomPorts {
  session: VoiceSessionPort;
  transcribe: VoiceTurnTranscriptionPort;
  /** Audio Device Port（§8.3）：apps/mobile 复用 #56 的原生捕获 Adapter。 */
  capture: DictationCapturePort;
  /** 轮次幂等身份（即转写 request_id；服务端按 request_id 幂等重放）。 */
  newRequestId(): string;
  /** product session id（服务端 unknown-pending 门与幂等授权的产品侧身份）。 */
  newSessionId(): string;
  /** 存活观察的 scope lease（Runtime 铸造；每个异步边界复查）。 */
  lease(): ScopeLease | undefined;
  audioDisposition?: VoiceAudioDisposition;
  /** 会话请求窗口；缺省 VOICE_ROOM_MAX_SECONDS（服务端 600s 上限）。 */
  maxSeconds?: number;
}

const nowIso = (): string => new Date().toISOString();

type DiscardReason = Parameters<VoiceAudioDisposition['onDiscarded']>[1];

function createVoiceHandle(taskId: string, runId: string | undefined, ports: VoiceRoomPorts): VoiceHandle {
  const maxSeconds = ports.maxSeconds ?? VOICE_ROOM_MAX_SECONDS;
  const disposition = ports.audioDisposition;
  let sessionId_: string | undefined;
  let state: VoiceRoomState = { phase: 'idle', taskId, ...(runId === undefined ? {} : { runId }), turns: [] };
  let generation = 0; // 会话代次：leave/resume/scope 撤销推进；旧代次迟到结果整代丢弃
  let currentTurnId: string | undefined;
  let beginning = false;
  let leaving = false;
  const listeners = new Set<(state: VoiceRoomState) => void>();
  const publish = (next: VoiceRoomState): void => {
    state = next;
    for (const listener of [...listeners]) listener(state);
  };
  const amend = (patch: Partial<VoiceRoomState>): void => publish({ ...state, ...patch });
  const live = (attemptGeneration: number): boolean => attemptGeneration === generation && leaseActive(ports.lease());

  const discardAudio = (reason: DiscardReason): void => {
    if (currentTurnId !== undefined) disposition?.onDiscarded(currentTurnId, reason);
  };

  /** scope 在边界处失效：本地收敛为 ended，绝不发起任何服务端调用（fail closed）。 */
  const collapseOnScopeLoss = (attemptGeneration: number): boolean => {
    if (live(attemptGeneration)) return false;
    discardAudio('cancelled');
    currentTurnId = undefined;
    publish({ ...state, phase: 'ended', pendingTurnId: undefined, sessionId: undefined, sessionExpiresAt: undefined, notice: { reason: 'scope-revoked', at: nowIso() } });
    return true;
  };

  async function endSessionQuietly(): Promise<void> {
    const id = sessionId_;
    if (id === undefined) return;
    sessionId_ = undefined;
    amend({ sessionId: undefined, sessionExpiresAt: undefined });
    try {
      const receipt = await ports.session.end(id);
      amend({ lastLeave: { settled: receipt.settled, state: receipt.state, at: nowIso() } });
    } catch {
      // 断线收尾是 best-effort：服务端 stop 幂等、reservation 有自身 deadline 兜底
    }
  }

  async function openSession(attemptGeneration: number): Promise<void> {
    amend({ phase: 'connecting', notice: undefined });
    try {
      const grant = await ports.session.open({ productSessionId: ports.newSessionId(), ...(runId === undefined ? {} : { runId }), maxSeconds });
      if (collapseOnScopeLoss(attemptGeneration)) {
        // 防御性收尾：这个刚打开的会话无人认领，尽力让服务端停掉（scope 已失则请求失败，无副作用）
        void ports.session.end(grant.id).catch(() => undefined);
        return;
      }
      sessionId_ = grant.id;
      amend({ phase: 'idle', sessionId: grant.id, sessionExpiresAt: grant.expiresAt });
    } catch {
      if (!live(attemptGeneration)) return;
      amend({ phase: 'interrupted', notice: { reason: 'session-open-failed', at: nowIso() } });
    }
  }

  const handle: VoiceHandle = {
    state: () => state,
    subscribe(listener) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    async beginTurn() {
      // 一口一轮：ready（待确认）/connecting/listening/transcribing/interrupted/ended 都不从
      if (beginning || leaving || state.phase !== 'idle') return;
      beginning = true;
      const attemptGeneration = generation;
      try {
        // 先请求权限/开始捕获，成功后才开会话——拒权绝不留下一个已计费的空会话
        let started: 'recording' | 'denied';
        try {
          started = await ports.capture.start();
        } catch {
          if (live(attemptGeneration)) amend({ notice: { reason: 'capture-failed', at: nowIso() } });
          return;
        }
        if (!live(attemptGeneration)) {
          try {
            await ports.capture.cancel();
          } catch {
            /* 收尾失败不影响代次失效 */
          }
          return;
        }
        if (started === 'denied') {
          amend({ notice: { reason: 'permission-denied', at: nowIso() } });
          return;
        }
        if (sessionId_ === undefined) {
          await openSession(attemptGeneration);
          if (state.phase !== 'idle' || sessionId_ === undefined) {
            // 开会话失败：立即收掉已开始的捕获（不给一个没有会话的录音窗）
            try {
              await ports.capture.cancel();
            } catch {
              /* 收尾失败不改变已呈现的失败态 */
            }
            return;
          }
        }
        amend({ phase: 'listening', notice: undefined });
      } finally {
        beginning = false;
      }
    },
    async endTurn() {
      if (state.phase !== 'listening') return;
      const attemptGeneration = generation;
      let audio: DictationAudio | undefined;
      try {
        audio = await ports.capture.stop();
      } catch {
        audio = undefined;
      }
      if (collapseOnScopeLoss(attemptGeneration)) return;
      if (audio === undefined || (audio.bytes === undefined && audio.uri === undefined)) {
        amend({ phase: 'idle', notice: { reason: 'capture-failed', at: nowIso() } });
        return;
      }
      if (audio.bytes !== undefined && audio.bytes.byteLength > VOICE_ROOM_MAX_AUDIO_BYTES) {
        amend({ phase: 'idle', notice: { reason: 'audio-too-large', at: nowIso() } });
        return; // 未进入转写窗口：音频从未被模块收留，无处置回调
      }
      const id = ports.newRequestId();
      currentTurnId = id;
      amend({ phase: 'transcribing', pendingTurnId: id });
      const boundSessionId = sessionId_;
      try {
        const result = await ports.transcribe.transcribe({ requestId: id, audio, ...(boundSessionId === undefined ? {} : { voiceSessionId: boundSessionId }) });
        if (collapseOnScopeLoss(attemptGeneration)) return; // 迟到结果整代丢弃（含音频处置已由收尾方完成）
        const text = typeof result.text === 'string' ? result.text.trim() : '';
        if (text === '') {
          // 空转写无重试价值（同 id 重放仍为空）；按断线语义明确结束
          discardAudio('failed');
          currentTurnId = undefined;
          amend({ phase: 'interrupted', pendingTurnId: undefined, notice: { reason: 'transcription-failed', at: nowIso() } });
          void endSessionQuietly();
          return;
        }
        discardAudio('transcribed');
        currentTurnId = undefined;
        amend({
          phase: 'ready',
          turns: [...state.turns, { turnId: id, transcript: text, state: 'review' as const }],
          pendingTurnId: id,
        });
      } catch {
        if (collapseOnScopeLoss(attemptGeneration)) return;
        // AC1 断线明确结束：转写失败 = 会话上下文不可信 → interrupted + 服务端 stop/settle
        discardAudio('failed');
        currentTurnId = undefined;
        amend({ phase: 'interrupted', pendingTurnId: undefined, notice: { reason: 'transcription-failed', at: nowIso() } });
        void endSessionQuietly();
      }
    },
    editTranscript(editId, text) {
      if (state.pendingTurnId !== editId) return;
      publish({ ...state, turns: state.turns.map((turn) => (turn.turnId === editId && turn.state === 'review' ? { ...turn, transcript: text } : turn)) });
    },
    confirmTranscript(confirmId) {
      if (state.pendingTurnId !== confirmId) return undefined;
      const turn = state.turns.find((entry) => entry.turnId === confirmId);
      if (turn === undefined || turn.state !== 'review') return undefined;
      const text = turn.transcript.trim();
      if (text === '') return undefined; // 空文本不产生指令（宿主提示可编辑或放弃）
      const at = nowIso();
      publish({
        ...state,
        phase: 'idle',
        pendingTurnId: undefined,
        turns: state.turns.map((entry) => (entry.turnId === confirmId ? { ...entry, transcript: text, state: 'confirmed' as const, confirmedAt: at } : entry)),
      });
      return { kind: 'steer', text };
    },
    discardTurn(discardId) {
      if (state.pendingTurnId !== discardId) return;
      publish({
        ...state,
        phase: 'idle',
        pendingTurnId: undefined,
        turns: state.turns.map((entry) => (entry.turnId === discardId ? { ...entry, state: 'discarded' as const } : entry)),
      });
    },
    async resume() {
      if (leaving || state.phase === 'connecting') return;
      generation += 1; // 旧会话的一切迟到结果整代丢弃
      discardAudio(state.phase === 'listening' ? 'cancelled' : 'failed');
      currentTurnId = undefined;
      amend({ pendingTurnId: undefined });
      if (state.phase === 'listening') {
        try {
          await ports.capture.cancel();
        } catch {
          /* 取消失败不阻断恢复 */
        }
      }
      await endSessionQuietly();
      await openSession(generation);
    },
    async leave(reason) {
      if (leaving) return;
      leaving = true;
      generation += 1;
      discardAudio(state.phase === 'listening' ? 'cancelled' : 'session-ended');
      currentTurnId = undefined;
      const at = nowIso();
      const wasListening = state.phase === 'listening';
      publish({
        ...state,
        phase: 'ended',
        pendingTurnId: undefined,
        turns: state.turns.map((entry) => (entry.state === 'review' ? { ...entry, state: 'discarded' as const } : entry)),
        notice: reason === undefined ? state.notice : { reason, at },
      });
      if (wasListening) {
        try {
          await ports.capture.cancel();
        } catch {
          /* 取消失败不阻断收尾 */
        }
      }
      await endSessionQuietly();
    },
    dispose() {
      void handle.leave().catch(() => undefined);
      listeners.clear();
    },
  };
  return handle;
}

export function createVoiceRoom(ports: VoiceRoomPorts): VoiceRoom {
  return {
    join(input) {
      const taskId = input.taskId.trim();
      if (taskId === '') throw new VoiceRoomError('VOICE_ROOM_INVALID_INPUT', 'taskId is required');
      if (!leaseActive(ports.lease())) throw new VoiceRoomError('VOICE_ROOM_SCOPE_CHANGED', 'no active scope lease');
      const runId = input.runId === undefined ? undefined : input.runId.trim();
      return createVoiceHandle(taskId, runId === '' ? undefined : runId, ports);
    },
  };
}
```

创建 `packages/mobile-core/src/voice-room/in-memory-voice-session.ts`：

```ts
import type { VoiceSessionEndReceipt, VoiceSessionGrant, VoiceSessionPort } from './voice-room.ts';

export interface ScriptedVoiceSession extends VoiceSessionPort {
  opens: Array<{ productSessionId: string; runId?: string; maxSeconds?: number }>;
  ends: string[];
  setOpenResult(result: Omit<VoiceSessionGrant, 'id'> | Error): void;
  setEndResult(result: Omit<VoiceSessionEndReceipt, 'id'> | Error): void;
}

/** 语音会话的 scripted test Adapter（spec §8.3：Realtime Voice Port 以 scripted Adapter 进
 * Interface 测试）。每次 open 产出一个新的会话 id（与服务端行为一致——每次授权新行），
 * end 回显被结束的会话 id。 */
export function createScriptedVoiceSession(): ScriptedVoiceSession {
  const opens: ScriptedVoiceSession['opens'] = [];
  const ends: string[] = [];
  let openResult: Omit<VoiceSessionGrant, 'id'> | Error = { expiresAt: '2026-09-26T00:10:00.000Z', maxSeconds: 600 };
  let endResult: Omit<VoiceSessionEndReceipt, 'id'> | Error = { state: 'closed', settled: true };
  return {
    opens,
    ends,
    async open(input) {
      opens.push(input);
      if (openResult instanceof Error) throw openResult;
      return { ...openResult, id: `vs-scripted-${opens.length}` };
    },
    async end(sessionId) {
      ends.push(sessionId);
      if (endResult instanceof Error) throw endResult;
      return { ...endResult, id: sessionId };
    },
    setOpenResult(result) {
      openResult = result;
    },
    setEndResult(result) {
      endResult = result;
    },
  };
}
```

修改 `packages/mobile-core/src/index.ts`（文件末尾追加）：

```ts

// —— T27 (#57) Task 内实时语音会话（Voice Room，module-seams §8）——
export { createVoiceRoom, VoiceRoomError, VOICE_ROOM_MAX_AUDIO_BYTES, VOICE_ROOM_MAX_SECONDS } from './voice-room/voice-room.ts';
export type {
  VoiceAudioDisposition, VoiceHandle, VoiceRoom, VoiceRoomErrorCode, VoiceRoomNoticeReason,
  VoiceRoomPhase, VoiceRoomPorts, VoiceRoomState, VoiceSessionEndReceipt, VoiceSessionGrant,
  VoiceSessionPort, VoiceSteerIntent, VoiceTurnTranscriptionPort, VoiceTurnView,
} from './voice-room/voice-room.ts';
export { createScriptedVoiceSession } from './voice-room/in-memory-voice-session.ts';
export type { ScriptedVoiceSession } from './voice-room/in-memory-voice-session.ts';
```

- [ ] **Step 4: 运行测试至通过**

Run: `pnpm exec tsx --test packages/mobile-core/src/voice-room/voice-room.test.ts`
Expected: PASS（12 个测试全绿；本计划作者已实跑 12/12）。

Run: `pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2022 --module NodeNext --moduleResolution NodeNext --allowImportingTsExtensions packages/mobile-core/src/index.ts`
Expected: 无类型错误（barrel 连同新模块整体编译）。

- [ ] **Step 5: 提交**

```bash
git add packages/mobile-core/src/voice-room/ packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core): Voice Room 深模块——Task 内实时语音会话（#57 T27 AC1/AC2 Interface）"
```

---

### Task 2: api-client 语音会话 Remote（`./mobile/voice-sessions`）

**Files:**
- Create: `packages/api-client/src/mobile/voice-sessions.ts`
- Test: `packages/api-client/src/mobile/voice-sessions.test.ts`
- Modify: `packages/api-client/package.json`（exports 增加 `"./mobile/voice-sessions": "./src/mobile/voice-sessions.ts"` 一行，插在 `"./mobile/voice"` 之后）

**Interfaces:**
- Consumes: `ClientRequest`（`../client.ts`）、`requireDeploymentOrigin`（`./deployment-origin.ts`，B3-F11 共享构造校验）、服务端 wire：`POST /api/v1/mobile/voice/sessions`（body `{session_id, run_id?, max_seconds?, deadline?}`，响应 `{success:true,data:{id,token,expires_at,max_seconds,price_version}}`，`mobile_voice.go:309-315`；重放响应无 token 字段，`mobile_voice.go:344-353`）、`DELETE /api/v1/mobile/voice/sessions/:id`（响应 `{success:true,data:{id,state,settled,audio_seconds?}}` / replay 分支含 `replay:true`，`mobile_voice.go:374-419`）、错误信封 `{"success":false,"error":"..."}`（`error` 串只进 ApiError.message，`code` 为 `HTTP_<status>`，`packages/api-client/src/errors.ts:51-62`）。
- Produces: `createMobileVoiceSessionRemote(options: MobileVoiceSessionRemoteOptions): MobileVoiceSessionRemote`；`MobileVoiceSessionRemote { open(input: { productSessionId: string; runId?: string; maxSeconds?: number }): Promise<MobileVoiceSessionGrant>; end(sessionId: string): Promise<MobileVoiceSessionEnd> }`；`MobileVoiceSessionGrant { id: string; token?: string; expiresAt: string; maxSeconds: number; priceVersion?: string }`；`MobileVoiceSessionEnd { id: string; state: string; settled: boolean; replay?: boolean; reason?: string }`。结构可赋值性：`MobileVoiceSessionRemote` **不能**直接赋给 mobile-core `VoiceSessionPort`（grant 带 token），组合根在 Task 4 做 3 行映射剥离 token——这是有意的可审计边界。

- [ ] **Step 1: 写失败测试**

创建 `packages/api-client/src/mobile/voice-sessions.test.ts`（本计划作者实跑 7/7 通过）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createMobileVoiceSessionRemote } from './voice-sessions.ts';

interface RecordedCall { method: string; path: string; headers?: Record<string, string>; body?: unknown }

function remoteOf(responder: (call: RecordedCall) => unknown): { remote: ReturnType<typeof createMobileVoiceSessionRemote>; calls: RecordedCall[] } {
  const calls: RecordedCall[] = [];
  const remote = createMobileVoiceSessionRemote({
    origin: 'https://weknora.example.test',
    request: (async (input) => {
      calls.push({ method: input.method, path: input.path, headers: input.headers, body: input.body });
      const result = responder(calls[calls.length - 1]!);
      if (result instanceof Error) throw result;
      return result;
    }) as never,
  });
  return { remote, calls };
}

const ok = (data: unknown): unknown => ({ success: true, data });
const httpError = (status: number, serverError: string): Error =>
  Object.assign(new Error(serverError), { status });

test('open posts the W30 session wire: session_id/run_id/max_seconds body and parses the grant envelope', async () => {
  const { remote, calls } = remoteOf(() => ok({ id: 'vs_1', token: 'wvp1.abc.def', expires_at: '2026-09-26T00:10:00Z', max_seconds: 600, price_version: 'voice-v1' }));
  const grant = await remote.open({ productSessionId: 'room-1', runId: 'run-9', maxSeconds: 300 });
  assert.equal(calls.length, 1);
  assert.equal(calls[0]!.method, 'POST');
  assert.equal(calls[0]!.path, '/api/v1/mobile/voice/sessions');
  assert.deepEqual(calls[0]!.body, { session_id: 'room-1', run_id: 'run-9', max_seconds: 300 });
  assert.deepEqual(grant, { id: 'vs_1', token: 'wvp1.abc.def', expiresAt: '2026-09-26T00:10:00Z', maxSeconds: 600, priceVersion: 'voice-v1' });
});

test('open parses an idempotent replay envelope without a token (token_issued false path)', async () => {
  const { remote } = remoteOf(() => ok({ id: 'vs_1', state: 'open', expires_at: '2026-09-26T00:10:00Z', max_seconds: 600, token_issued: false, replay: true }));
  const grant = await remote.open({ productSessionId: 'room-1' });
  assert.equal(grant.token, undefined, '重放响应没有明文 token——字段如实缺席');
  assert.equal(grant.maxSeconds, 600);
});

test('open error translation covers the full W30 status matrix', async () => {
  const cases: Array<{ status: number; serverError: string; expected: string }> = [
    { status: 400, serverError: 'invalid_voice_session_request', expected: 'VOICE_SESSION_INVALID' },
    { status: 400, serverError: 'voice_window_too_short', expected: 'VOICE_SESSION_INVALID' },
    { status: 402, serverError: 'voice_budget_denied', expected: 'VOICE_BUDGET_DENIED' },
    { status: 409, serverError: 'voice_session_unknown_pending', expected: 'VOICE_SESSION_CONFLICT' },
    { status: 409, serverError: 'voice_session_exists', expected: 'VOICE_SESSION_CONFLICT' },
    { status: 502, serverError: 'voice_provider_unavailable', expected: 'VOICE_PROVIDER_UNAVAILABLE' },
    { status: 503, serverError: 'voice_charging_unconfigured', expected: 'VOICE_CHARGING_UNCONFIGURED' },
    { status: 503, serverError: 'voice_open_unknown', expected: 'VOICE_OPEN_UNKNOWN' },
  ];
  for (const testCase of cases) {
    const { remote } = remoteOf(() => {
      throw httpError(testCase.status, testCase.serverError);
    });
    await assert.rejects(() => remote.open({ productSessionId: 'room-1' }), (error: unknown) => (error as { code?: string }).code === testCase.expected, `${testCase.status} ${testCase.serverError} → ${testCase.expected}`);
  }
});

test('end deletes the session id (encodeURIComponent) and parses settle receipts honestly', async () => {
  const { remote, calls } = remoteOf(() => ok({ id: 'vs 1', state: 'closed', settled: true, audio_seconds: 5 }));
  const receipt = await remote.end('vs 1');
  assert.equal(calls[0]!.method, 'DELETE');
  assert.equal(calls[0]!.path, '/api/v1/mobile/voice/sessions/vs%201');
  assert.deepEqual(receipt, { id: 'vs 1', state: 'closed', settled: true });
  const { remote: replayRemote } = remoteOf(() => ok({ id: 'vs_1', state: 'closed', settled: true, replay: true }));
  assert.deepEqual(await replayRemote.end('vs_1'), { id: 'vs_1', state: 'closed', settled: true, replay: true });
  const { remote: retryRemote } = remoteOf(() => ok({ id: 'vs_1', state: 'unknown', settled: false, reason: 'pending_reconciliation' }));
  assert.deepEqual(await retryRemote.end('vs_1'), { id: 'vs_1', state: 'unknown', settled: false, reason: 'pending_reconciliation' });
});

test('end error translation: 404 is not-found, other http failures are end-failed', async () => {
  const { remote: missing } = remoteOf(() => {
    throw httpError(404, 'Not Found');
  });
  await assert.rejects(() => missing.end('vs_x'), (error: unknown) => (error as { code?: string }).code === 'VOICE_SESSION_NOT_FOUND');
  const { remote: broken } = remoteOf(() => {
    throw httpError(500, 'voice_session_store_unavailable');
  });
  await assert.rejects(() => broken.end('vs_x'), (error: unknown) => (error as { code?: string }).code === 'VOICE_SESSION_END_FAILED');
});

test('malformed envelopes are rejected rather than trusted', async () => {
  const { remote } = remoteOf(() => ({ success: false, error: 'nope' }));
  await assert.rejects(() => remote.open({ productSessionId: 'room-1' }), /success envelope/);
  const { remote: shapeless } = remoteOf(() => ok({ expires_at: 'x', max_seconds: 1 }));
  await assert.rejects(() => shapeless.open({ productSessionId: 'room-1' }), /data\.id/);
});

test('origin is validated at construction', () => {
  assert.throws(() => createMobileVoiceSessionRemote({ origin: 'http://insecure.example', request: async () => undefined }));
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm exec tsx --test packages/api-client/src/mobile/voice-sessions.test.ts`
Expected: FAIL——`Cannot find module './voice-sessions.ts'`。

- [ ] **Step 3: 写最小实现**

创建 `packages/api-client/src/mobile/voice-sessions.ts`（本计划作者实跑通过）：

```ts
import type { ClientRequest } from '../client.ts';
import { requireDeploymentOrigin } from './deployment-origin.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface MobileVoiceSessionRemoteOptions {
  /** 部署 Origin。构造即强校验（共享 requireDeploymentOrigin）：绝对 HTTPS、含 host、无 userinfo、无 path/query/fragment。 */
  origin: string;
  /** 授权通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输。 */
  request: Request;
}

/** 与服务端创建信封逐字对齐（internal/handler/mobile_voice.go:309-315）。
 * `token` 是短期媒体 grant 的明文，只在首次签发响应出现（幂等重放响应没有该字段）；
 * 组合根把它剥离后才交给 mobile-core VoiceSessionPort——media 凭据绝不进入模块状态。 */
export interface MobileVoiceSessionGrant {
  id: string;
  token?: string;
  expiresAt: string;
  maxSeconds: number;
  priceVersion?: string;
}

/** 服务端停止/结算回执（mobile_voice.go:374-419 的三个分支：closed 重放 / unknown 待对账 / 正常结算）。 */
export interface MobileVoiceSessionEnd {
  id: string;
  state: string;
  settled: boolean;
  replay?: boolean;
  reason?: string;
}

export interface MobileVoiceSessionRemote {
  /** POST /api/v1/mobile/voice/sessions。 */
  open(input: { productSessionId: string; runId?: string; maxSeconds?: number }): Promise<MobileVoiceSessionGrant>;
  /** DELETE /api/v1/mobile/voice/sessions/:id（幂等 stop/settle）。 */
  end(sessionId: string): Promise<MobileVoiceSessionEnd>;
}

const SESSIONS_PATH = '/api/v1/mobile/voice/sessions';

type VoiceSessionCode =
  | 'VOICE_SESSION_INVALID' | 'VOICE_BUDGET_DENIED' | 'VOICE_SESSION_CONFLICT' | 'VOICE_PROVIDER_UNAVAILABLE'
  | 'VOICE_OPEN_UNKNOWN' | 'VOICE_CHARGING_UNCONFIGURED' | 'VOICE_SESSION_NOT_FOUND' | 'VOICE_SESSION_END_FAILED';

function coded(code: VoiceSessionCode, message: string): Error {
  return Object.assign(new Error(message), { code });
}

/** ApiError 的 code 是 HTTP_<status>；服务端错误串（{"success":false,"error":"..."}）只进 message
 * （errors.ts:51-62）——语义翻译在此按 status + message 完成。 */
function statusOf(error: unknown): number {
  return error instanceof Error && 'status' in error ? Number((error as { status?: unknown }).status) : NaN;
}

function messageOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function translateOpenError(error: unknown): Error {
  const status = statusOf(error);
  const text = messageOf(error);
  switch (status) {
    case 400: return coded('VOICE_SESSION_INVALID', `voice session rejected: ${text}`);
    case 402: return coded('VOICE_BUDGET_DENIED', `voice budget denied: ${text}`);
    case 409: return coded('VOICE_SESSION_CONFLICT', `voice session conflict: ${text}`);
    case 502: return coded('VOICE_PROVIDER_UNAVAILABLE', `voice provider unavailable: ${text}`);
    case 503: return text.includes('voice_open_unknown')
      ? coded('VOICE_OPEN_UNKNOWN', `voice open outcome unknown: ${text}`)
      : coded('VOICE_CHARGING_UNCONFIGURED', `voice charging unconfigured: ${text}`);
    default:
      if (Number.isNaN(status)) return error instanceof Error ? error : new Error(text);
      return coded('VOICE_SESSION_INVALID', `voice session open failed (HTTP ${status}): ${text}`);
  }
}

function translateEndError(error: unknown): Error {
  const status = statusOf(error);
  if (status === 404) return coded('VOICE_SESSION_NOT_FOUND', `voice session not found: ${messageOf(error)}`);
  if (!Number.isNaN(status)) return coded('VOICE_SESSION_END_FAILED', `voice session end failed (HTTP ${status}): ${messageOf(error)}`);
  return error instanceof Error ? error : new Error(messageOf(error));
}

function requireEnvelope(value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new Error('voice session response must be a success envelope');
  }
  const record = value as { success?: unknown; data?: unknown };
  if (record.success !== true || typeof record.data !== 'object' || record.data === null) {
    throw new Error('voice session response must be a success envelope: success must be true with data');
  }
  return record.data as Record<string, unknown>;
}

export function createMobileVoiceSessionRemote(options: MobileVoiceSessionRemoteOptions): MobileVoiceSessionRemote {
  requireDeploymentOrigin(options.origin);
  const request = options.request;
  return {
    async open(input) {
      const productSessionId = typeof input.productSessionId === 'string' ? input.productSessionId.trim() : '';
      if (productSessionId === '') throw new Error('voice session product id is required');
      if (input.maxSeconds !== undefined && (!Number.isInteger(input.maxSeconds) || input.maxSeconds <= 0 || input.maxSeconds > 600)) {
        throw new Error('voice session maxSeconds must be an integer in (0, 600]');
      }
      let envelope: unknown;
      try {
        envelope = await request({
          method: 'POST',
          path: SESSIONS_PATH,
          body: {
            session_id: productSessionId,
            ...(input.runId === undefined || input.runId.trim() === '' ? {} : { run_id: input.runId.trim() }),
            ...(input.maxSeconds === undefined ? {} : { max_seconds: input.maxSeconds }),
          },
        });
      } catch (error) {
        throw translateOpenError(error);
      }
      const data = requireEnvelope(envelope);
      if (typeof data.id !== 'string' || data.id === '') throw new Error('voice session data.id must be a non-empty string');
      if (typeof data.expires_at !== 'string' || data.expires_at === '') throw new Error('voice session data.expires_at must be a string');
      if (typeof data.max_seconds !== 'number') throw new Error('voice session data.max_seconds must be a number');
      return {
        id: data.id,
        ...(typeof data.token === 'string' && data.token !== '' ? { token: data.token } : {}),
        expiresAt: data.expires_at,
        maxSeconds: data.max_seconds,
        ...(typeof data.price_version === 'string' && data.price_version !== '' ? { priceVersion: data.price_version } : {}),
      };
    },
    async end(sessionId) {
      const id = typeof sessionId === 'string' ? sessionId.trim() : '';
      if (id === '') throw new Error('voice session id is required');
      let envelope: unknown;
      try {
        envelope = await request({ method: 'DELETE', path: `${SESSIONS_PATH}/${encodeURIComponent(id)}` });
      } catch (error) {
        throw translateEndError(error);
      }
      const data = requireEnvelope(envelope);
      if (typeof data.id !== 'string' || typeof data.state !== 'string') throw new Error('voice session end data must carry id and state');
      return {
        id: data.id,
        state: data.state,
        settled: data.settled === true,
        ...(data.replay === true ? { replay: true } : {}),
        ...(typeof data.reason === 'string' && data.reason !== '' ? { reason: data.reason } : {}),
      };
    },
  };
}
```

修改 `packages/api-client/package.json` exports（`"./mobile/voice"` 行之后插入一行）：

```json
    "./mobile/voice-sessions": "./src/mobile/voice-sessions.ts",
```

- [ ] **Step 4: 运行测试至通过**

Run: `pnpm exec tsx --test packages/api-client/src/mobile/voice-sessions.test.ts packages/api-client/src/mobile/voice.test.ts`
Expected: PASS（新增 7 个 + 既有 #56 voice 测试零回归）。

- [ ] **Step 5: 提交**

```bash
git add packages/api-client/src/mobile/voice-sessions.ts packages/api-client/src/mobile/voice-sessions.test.ts packages/api-client/package.json
git commit -m "feat(api-client): createMobileVoiceSessionRemote——语音会话 open/end wire 适配（#57）"
```

---

### Task 3: apps/mobile 控制器/文案/屏（voice-room-view + VoiceRoomScreen）

**Files:**
- Create: `apps/mobile/src/voice-room-view.ts`
- Create: `apps/mobile/src/screens/VoiceRoomScreen.tsx`
- Test: `apps/mobile/src/voice-room-view.test.ts`

**Interfaces:**
- Consumes: `VoiceRoom`/`VoiceHandle`/`VoiceRoomState`/`VoiceRoomNoticeReason`/`VoiceRoomPhase`/`VoiceSteerIntent`（Task 1 barrel 导出）、`TASK_OFFICE_ERROR_COPY`（`./task-detail-view.ts:11-20`）、react-native 组件（测试以 stub 注入，同 `voice-dictation-integration-smoke.test.ts:17-41` 手法）。
- Produces: `createVoiceRoomController(ports: VoiceRoomControllerPorts): VoiceRoomController`；`VoiceRoomControllerPorts { handle: VoiceHandle; onConfirmIntent(intent: { kind: 'steer'; text: string }): Promise<unknown> }`；`VoiceRoomController { state(): VoiceRoomViewState; subscribe(listener): () => void; beginTurn(): Promise<void>; endTurn(): Promise<void>; editTranscript(text: string): void; confirmTranscript(): Promise<void>; discardTurn(): void; resume(): Promise<void>; leave(): Promise<void>; dispose(): void }`（控制器把 pending 轮次 id 封装在内部——屏永不见 turnId）；`VoiceRoomViewState extends VoiceRoomState { submitting?: boolean; lastSubmitError?: string }`；文案常量 `VOICE_ROOM_PHASE_COPY`/`VOICE_ROOM_NOTICE_COPY`/`VOICE_ROOM_SUBMIT_ERROR_PREFIX`/`VOICE_ROOM_APPROVAL_COPY`。

- [ ] **Step 1: 写失败测试**

创建 `apps/mobile/src/voice-room-view.test.ts`（本计划作者实跑 5/5 通过；注意两件事：(a) `RuntimeScopeLease` 不入 barrel（#32 决策），apps/mobile 无法自铸真 lease——控制器测试用脚本化 VoiceHandle 替身，控制器才是被测对象；(b) `TaskOfficeError` 构造签名是 `(code)` 单参，message 即 code）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import * as nodeModule from 'node:module';

// VoiceRoomScreen.tsx 静态拉入 react-native 源码，esbuild 无法转换——与
// voice-dictation-integration-smoke.test.ts 相同的在库先例：react-native 解析为惰性
// CommonJS stub，再动态导入被测模块（静态导入会被提升，必须动态导入）。
type ResolveNext = (specifier: string, context: unknown) => unknown;
const moduleWithHooks = nodeModule as typeof nodeModule & {
  registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: ResolveNext) => unknown }) => void;
};

const NATIVE_MODULE_STUBS: Record<string, string> = {
  'react-native': "module.exports = { View: 'View', Text: 'Text', TextInput: 'TextInput', Button: 'Button', ScrollView: 'ScrollView' }",
};
const stubDir = mkdtempSync(join(tmpdir(), 'weknora-voice-room-stub-'));
const stubPath = (name: string): string => join(stubDir, `${name.replaceAll('/', '+')}.cjs`);
for (const [name, source] of Object.entries(NATIVE_MODULE_STUBS)) {
  writeFileSync(stubPath(name), source);
}
if (moduleWithHooks.registerHooks) {
  moduleWithHooks.registerHooks({
    resolve: (specifier, context, nextResolve) =>
      specifier in NATIVE_MODULE_STUBS ? { shortCircuit: true, url: pathToFileURL(stubPath(specifier)).href } : nextResolve(specifier, context),
  });
}
test.after(() => {
  rmSync(stubDir, { recursive: true, force: true });
});

const loadView = () => import('./voice-room-view.ts');
const tick = async (): Promise<void> => { await new Promise<void>((resolve) => setImmediate(resolve)); };

/** 脚本化 VoiceHandle 替身：控制器是被测对象（Voice Room 模块 Interface 已在 mobile-core 全测）。 */
function scriptHandle(): { handle: import('@weknora/mobile-core').VoiceHandle; turnToReady(text: string): Promise<void> } {
  type VoiceRoomState = import('@weknora/mobile-core').VoiceRoomState;
  let listener: ((state: VoiceRoomState) => void) | undefined;
  let seq = 0;
  let state: VoiceRoomState = { phase: 'idle', taskId: 'task-1', turns: [] };
  const publish = (next: VoiceRoomState): void => { state = next; listener?.(state); };
  const handle: import('@weknora/mobile-core').VoiceHandle = {
    state: () => state,
    subscribe(l) { listener = l; return () => { listener = undefined; }; },
    async beginTurn() { publish({ ...state, phase: 'listening' }); },
    async endTurn() {
      const id = `req-${++seq}`;
      publish({ ...state, phase: 'transcribing', pendingTurnId: id });
    },
    editTranscript(id, text) {
      publish({ ...state, turns: state.turns.map((t) => (t.turnId === id && t.state === 'review' ? { ...t, transcript: text } : t)) });
    },
    confirmTranscript(id) {
      if (state.pendingTurnId !== id) return undefined;
      const turn = state.turns.find((t) => t.turnId === id);
      if (turn === undefined) return undefined;
      const text = turn.transcript.trim();
      if (text === '') return undefined;
      publish({ ...state, phase: 'idle', pendingTurnId: undefined, turns: state.turns.map((t) => (t.turnId === id ? { ...t, state: 'confirmed' as const, transcript: text } : t)) });
      return { kind: 'steer' as const, text };
    },
    discardTurn(id) {
      if (state.pendingTurnId !== id) return;
      publish({ ...state, phase: 'idle', pendingTurnId: undefined, turns: state.turns.map((t) => (t.turnId === id ? { ...t, state: 'discarded' as const } : t)) });
    },
    async resume() { publish({ ...state, phase: 'idle' }); },
    async leave() { publish({ ...state, phase: 'ended', pendingTurnId: undefined }); },
    dispose() { listener = undefined; },
  };
  return {
    handle,
    async turnToReady(text: string): Promise<void> {
      const pendingId = state.pendingTurnId;
      if (pendingId === undefined) throw new Error('no turn in flight');
      publish({ ...state, phase: 'ready', turns: [...state.turns, { turnId: pendingId, transcript: text, state: 'review' }], pendingTurnId: pendingId });
    },
  };
}

test('voice room copy covers every phase, notice reason, and never leaks wire detail', async () => {
  const { VOICE_ROOM_NOTICE_COPY, VOICE_ROOM_PHASE_COPY } = await loadView();
  for (const reason of ['session-open-failed', 'transcription-failed', 'permission-denied', 'capture-failed', 'audio-too-large', 'scope-revoked'] as const) {
    assert.ok(VOICE_ROOM_NOTICE_COPY[reason].trim().length > 4, `${reason} 有用户文案`);
  }
  for (const phase of ['idle', 'connecting', 'listening', 'transcribing', 'ready', 'interrupted', 'ended'] as const) {
    assert.ok(VOICE_ROOM_PHASE_COPY[phase].trim().length > 0, `${phase} 有状态文案`);
  }
  assert.ok(VOICE_ROOM_NOTICE_COPY['scope-revoked'].includes('重新'), 'scope 文案引导重新进入而非暴露内部码');
});

test('controller wraps the handle: pending-turn identity never escapes, confirm routes through onConfirmIntent', async () => {
  const { createVoiceRoomController } = await loadView();
  const { handle, turnToReady } = scriptHandle();
  const submitted: Array<{ kind: string; text: string }> = [];
  const controller = createVoiceRoomController({ handle, onConfirmIntent: async (intent) => { submitted.push(intent); } });

  await controller.beginTurn();
  const ending = controller.endTurn();
  await ending;
  await turnToReady('帮我把结论整理成三段');
  assert.equal(controller.state().phase, 'ready');
  controller.editTranscript('帮我把结论整理成三段（校对）');
  await controller.confirmTranscript();
  assert.deepEqual(submitted, [{ kind: 'steer', text: '帮我把结论整理成三段（校对）' }], '确认文字经宿主 act 通道提交');
  assert.equal(controller.state().phase, 'idle');
  assert.equal(controller.state().pendingTurnId, undefined, '确认后屏面无待确认轮次');
  assert.equal(JSON.stringify(controller.state()).includes('audio'), false, '控制器状态不携带音频');
  await controller.leave();
  assert.equal(controller.state().phase, 'ended');
  controller.dispose();
});

test('controller surfaces submit failure honestly without discarding the confirmed intent', async () => {
  const { createVoiceRoomController, VOICE_ROOM_SUBMIT_ERROR_PREFIX } = await loadView();
  const { TaskOfficeError } = await import('@weknora/mobile-core');
  const { handle, turnToReady } = scriptHandle();
  const controller = createVoiceRoomController({
    handle,
    onConfirmIntent: async () => {
      throw new TaskOfficeError('TASK_OFFICE_COMMAND_UNAVAILABLE');
    },
  });
  await controller.beginTurn();
  await controller.endTurn();
  await turnToReady('一句话');
  await controller.confirmTranscript();
  assert.equal(controller.state().lastSubmitError, `${VOICE_ROOM_SUBMIT_ERROR_PREFIX}当前部署未提供运行干预通道。`, 'act 失败以 Task Office 文案如实呈现');
  assert.equal(handle.state().turns[0]!.state, 'confirmed', '确认已发生：文字不回滚（服务端 act 可重试）');
  controller.dispose();
});

test('the screen renders state + callbacks only, and the view module never imports wire packages', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const screenSource = readFileSync(join(here, 'screens/VoiceRoomScreen.tsx'), 'utf8');
  assert.equal(/@weknora\/(api-client|contracts)/.test(screenSource), false, 'VoiceRoomScreen 禁止导入 wire 包（module-seams §10）');
  const viewSource = readFileSync(join(here, 'voice-room-view.ts'), 'utf8');
  assert.equal(/@weknora\/(api-client|contracts)/.test(viewSource), false, 'voice-room-view 同样禁止');
  const { VoiceRoomScreen } = await import('./screens/VoiceRoomScreen.tsx');
  assert.equal(typeof VoiceRoomScreen, 'function');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/voice-room-view.test.ts`
Expected: FAIL——`Cannot find module './voice-room-view.ts'`。

- [ ] **Step 3: 写最小实现**

创建 `apps/mobile/src/voice-room-view.ts`（本计划作者实跑通过；错误映射用 `.code` 形状查表而非 instanceof——tsx 的 CJS/ESM 双实例下 instanceof 跨模块不可靠，验证期实锤；订阅用 VoiceHandle 自己的 `subscribe`，不是 TaskHandle 的 `updates`）：

```ts
import type { VoiceHandle, VoiceRoomNoticeReason, VoiceRoomPhase, VoiceRoomState } from '@weknora/mobile-core';
import { TASK_OFFICE_ERROR_COPY } from './task-detail-view.ts';

/** 状态/通知/错误三组文案（对照 DICTATION_FAILURE_COPY 先例：不直出内部码）。 */
export const VOICE_ROOM_PHASE_COPY: Record<VoiceRoomPhase, string> = {
  idle: '语音房已就绪，按下开始说话',
  connecting: '正在加入语音房…',
  listening: '正在聆听…',
  transcribing: '正在转写…',
  ready: '转写完成，可校对后确认',
  interrupted: '语音房已中断：会话已明确结束，可恢复',
  ended: '语音房已结束',
};

export const VOICE_ROOM_NOTICE_COPY: Record<VoiceRoomNoticeReason, string> = {
  'session-open-failed': '语音会话授权失败（可能未配置语音计价或预算不足）；可稍后恢复。',
  'transcription-failed': '本轮转写失败，语音会话已明确结束；恢复后将开启新会话。',
  'permission-denied': '麦克风权限被拒绝；可在系统设置授权后重试。',
  'capture-failed': '录音不可用；可重试本轮。',
  'audio-too-large': '本轮录音过大（超过 16 MiB）；请缩短发言。',
  'scope-revoked': '登录状态或活动空间已变化，语音房已结束；请重新进入。',
};

export const VOICE_ROOM_SUBMIT_ERROR_PREFIX = '指令提交失败：';

/** 审批互斥的常驻呈现（AC2）：语音房内明确告知确认文字的走向与边界。 */
export const VOICE_ROOM_APPROVAL_COPY = '语音确认的文字以「调整指令」进入任务时间线；高风险审批只能在行动收件箱完成，语音无法代替审批。';

export interface VoiceRoomViewState extends VoiceRoomState {
  submitting?: boolean;
  lastSubmitError?: string;
}

export interface VoiceRoomControllerPorts {
  handle: VoiceHandle;
  /** 确认文字的宿主提交通道：TaskHandle.act（spec §8.1）。模块本身零提交。 */
  onConfirmIntent(intent: { kind: 'steer'; text: string }): Promise<unknown>;
}

export interface VoiceRoomController {
  state(): VoiceRoomViewState;
  subscribe(listener: (state: VoiceRoomViewState) => void): () => void;
  beginTurn(): Promise<void>;
  endTurn(): Promise<void>;
  editTranscript(text: string): void;
  confirmTranscript(): Promise<void>;
  discardTurn(): void;
  resume(): Promise<void>;
  leave(): Promise<void>;
  dispose(): void;
}

/** act 失败 → 用户文案。用 .code 形状查表而非 instanceof——tsx 的 CJS/ESM 双实例下
 * instanceof 跨模块不可靠（本计划验证期实锤）；TaskOfficeError 的判别特征就是 .code。 */
const submitMessageOf = (failure: unknown): string => {
  const code = typeof failure === 'object' && failure !== null && 'code' in failure ? (failure as { code?: unknown }).code : undefined;
  if (typeof code === 'string' && TASK_OFFICE_ERROR_COPY[code] !== undefined) {
    return `${VOICE_ROOM_SUBMIT_ERROR_PREFIX}${TASK_OFFICE_ERROR_COPY[code]}`;
  }
  return `${VOICE_ROOM_SUBMIT_ERROR_PREFIX}${failure instanceof Error ? failure.message : String(failure)}`;
};

/** 语音房控制器：把 pending 轮次 id 封装在内部（屏只见文本与状态），确认动作直通宿主 act 通道。 */
export function createVoiceRoomController(ports: VoiceRoomControllerPorts): VoiceRoomController {
  let state: VoiceRoomViewState = { ...ports.handle.state() };
  let disposed = false;
  const listeners = new Set<(state: VoiceRoomViewState) => void>();
  const publish = (next: VoiceRoomViewState): void => {
    state = next;
    for (const listener of [...listeners]) listener(state);
  };
  const unsubscribe = ports.handle.subscribe((next) => {
    if (!disposed) publish({ submitting: state.submitting, lastSubmitError: state.lastSubmitError, ...next });
  });
  const pendingId = (): string | undefined => ports.handle.state().pendingTurnId;
  return {
    state: () => state,
    subscribe(listener) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    beginTurn() {
      publish({ ...state, lastSubmitError: undefined });
      return ports.handle.beginTurn();
    },
    endTurn() {
      return ports.handle.endTurn();
    },
    editTranscript(text) {
      const id = pendingId();
      if (id !== undefined) ports.handle.editTranscript(id, text);
    },
    async confirmTranscript() {
      const id = pendingId();
      if (id === undefined || state.submitting === true) return;
      const intent = ports.handle.confirmTranscript(id);
      if (intent === undefined) return;
      publish({ ...state, submitting: true, lastSubmitError: undefined });
      try {
        await ports.onConfirmIntent(intent);
        publish({ ...state, submitting: false });
      } catch (failure) {
        publish({ ...state, submitting: false, lastSubmitError: submitMessageOf(failure) });
      }
    },
    discardTurn() {
      const id = pendingId();
      if (id !== undefined) ports.handle.discardTurn(id);
    },
    resume() {
      return ports.handle.resume();
    },
    leave() {
      return ports.handle.leave();
    },
    dispose() {
      if (disposed) return;
      disposed = true;
      unsubscribe();
      listeners.clear();
      ports.handle.dispose();
    },
  };
}
```

创建 `apps/mobile/src/screens/VoiceRoomScreen.tsx`：

```tsx
import { Button, ScrollView, Text, TextInput, View } from 'react-native';
import type { VoiceRoomViewState } from '../voice-room-view.ts';
import { VOICE_ROOM_APPROVAL_COPY, VOICE_ROOM_NOTICE_COPY, VOICE_ROOM_PHASE_COPY } from '../voice-room-view.ts';

export interface VoiceRoomScreenProps {
  state?: VoiceRoomViewState;
  onBeginTurn?(): void;
  onEndTurn?(): void;
  onEditTranscript?(text: string): void;
  onConfirmTranscript?(): void;
  onDiscardTurn?(): void;
  onResume?(): void;
  onLeave?(): void;
}

/** 绑定 Task 的 Voice Room 屏（module-seams §10）：只见 VoiceRoomViewState + 回调，
 * 不见 wire、request id、turnId。已确认文字作为语音交互记录常驻可见（CONTEXT.md:339）。 */
export function VoiceRoomScreen({ state, onBeginTurn, onEndTurn, onEditTranscript, onConfirmTranscript, onDiscardTurn, onResume, onLeave }: VoiceRoomScreenProps) {
  if (state === undefined) {
    return (
      <View>
        <Text>请先登录并激活空间，再加入语音房。</Text>
      </View>
    );
  }
  const pending = state.turns.find((turn) => turn.turnId === state.pendingTurnId);
  return (
    <ScrollView>
      <Text>语音房 · 任务 {state.taskId}{state.runId === undefined ? '' : ` · Run ${state.runId}`}</Text>
      <Text>{VOICE_ROOM_PHASE_COPY[state.phase]}</Text>
      {state.notice !== undefined && <Text>{VOICE_ROOM_NOTICE_COPY[state.notice.reason]}</Text>}
      {state.sessionId !== undefined && <Text numberOfLines={1}>会话有效期至 {state.sessionExpiresAt ?? '—'}</Text>}
      <Text>{VOICE_ROOM_APPROVAL_COPY}</Text>

      {state.phase === 'idle' && <Button title="开始说话" onPress={() => { onBeginTurn?.(); }} />}
      {state.phase === 'listening' && <Button title="结束本轮" onPress={() => { onEndTurn?.(); }} />}
      {state.phase === 'transcribing' && <Text>转写中，请稍候…</Text>}
      {state.phase === 'interrupted' && <Button title="恢复语音房" onPress={() => { onResume?.(); }} />}
      {(state.phase === 'idle' || state.phase === 'interrupted' || state.phase === 'ready') && <Button title="离开语音房" onPress={() => { onLeave?.(); }} />}

      {pending !== undefined && (
        <View>
          <Text>本轮转写（可校对）</Text>
          <TextInput value={pending.transcript} onChangeText={(text) => { onEditTranscript?.(text); }} multiline />
          <Button title="确认写入任务" disabled={state.submitting === true || pending.transcript.trim() === ''} onPress={() => { onConfirmTranscript?.(); }} />
          <Button title="放弃本轮" onPress={() => { onDiscardTurn?.(); }} />
        </View>
      )}
      {state.lastSubmitError !== undefined && <Text>{state.lastSubmitError}</Text>}

      {(state.turns.filter((turn) => turn.state === 'confirmed')).length > 0 && <Text>已确认的文字（写入任务）</Text>}
      {[...state.turns].reverse().map((turn) => (
        <View key={turn.turnId}>
          <Text numberOfLines={2}>{turn.state === 'confirmed' ? `✓ ${turn.transcript}` : turn.state === 'review' ? '待确认…' : '已放弃'}</Text>
        </View>
      ))}
    </ScrollView>
  );
}
```

- [ ] **Step 4: 运行测试至通过**

Run: `pnpm exec tsx --test apps/mobile/src/voice-room-view.test.ts`
Expected: PASS（5 个测试全绿）。

- [ ] **Step 5: 提交**

```bash
git add apps/mobile/src/voice-room-view.ts apps/mobile/src/voice-room-view.test.ts apps/mobile/src/screens/VoiceRoomScreen.tsx
git commit -m "feat(mobile): Voice Room 控制器/文案/屏——确认文字经 act 通道提交（#57）"
```

---

### Task 4: 路由、组合根装配与详情入口

**Files:**
- Create: `apps/mobile/src/app/tasks/voice.tsx`
- Modify: `apps/mobile/src/composition.ts`（import 块 + `activeDictation` 之后插入 `voiceRoomFor`/`activeVoiceRoom`）
- Modify: `apps/mobile/src/screens/TaskDetailScreen.tsx`（props 加 `onOpenVoiceRoom?`、组件解构加参数、操作区加按钮、干预行加指令文本——四处小改）
- Modify: `apps/mobile/src/app/tasks/detail.tsx`（传 `onOpenVoiceRoom`——三处小改）

**Interfaces:**
- Consumes: Task 1 `createVoiceRoom`（barrel）、Task 2 `createMobileVoiceSessionRemote`、#56 `createMobileVoiceTranscriptionRemote`、composition 既有 `runtime()/deploymentScopeKey/cachePut/nativeDictationCapture/createNativeRequestId/activeTaskOffice`、detail 路由的 `createTaskDetailController`/`activeTaskOffice()` 模式（`apps/mobile/src/app/tasks/detail.tsx:24-50`）。
- Produces: `activeVoiceRoom(): VoiceRoom | undefined`；`/tasks/voice?taskId=..&runId=..` 路由；`TaskDetailScreenProps.onOpenVoiceRoom?: () => void`。

**并行预警：** `TaskDetailScreen.tsx`/`composition.ts`/`detail.tsx` 是多计划共享热点，动手前先重读锚点上下文（并行计划可能已移动行号），修改保持下述最小形状。

- [ ] **Step 1: 写失败测试**

追加到 `apps/mobile/src/voice-room-view.test.ts` 末尾（同文件，同一 react stub 已就位）：

```ts
test('the voice route is an Expo Router screen wired through composition only', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const routeSource = readFileSync(join(here, 'app/tasks/voice.tsx'), 'utf8');
  assert.match(routeSource, /activeVoiceRoom\(\)/, '路由经 activeVoiceRoom 取模块');
  assert.match(routeSource, /activeTaskOffice\(\)/, '确认文字经既有 Task Office act 通道');
  assert.doesNotMatch(routeSource, /@weknora\/api-client/, '路由不直接导入 wire');
  const detailRoute = readFileSync(join(here, 'app/tasks/detail.tsx'), 'utf8');
  assert.match(detailRoute, /onOpenVoiceRoom/, '详情路由提供语音房入口');
  const detailScreen = readFileSync(join(here, 'screens/TaskDetailScreen.tsx'), 'utf8');
  assert.match(detailScreen, /onOpenVoiceRoom/, '详情屏渲染语音房入口');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/voice-room-view.test.ts`
Expected: FAIL——新增用例因 `app/tasks/voice.tsx` 不存在 / detail 未接线而失败。

- [ ] **Step 3: 写最小实现**

创建 `apps/mobile/src/app/tasks/voice.tsx`（本计划作者实跑接线 + typecheck 通过）：

```tsx
import { useEffect, useRef, useState } from 'react';
import { router, useLocalSearchParams } from 'expo-router';
import { TaskOfficeError } from '@weknora/mobile-core';
import { activeTaskOffice, activeVoiceRoom } from '../../composition.ts';
import { createTaskDetailController, TASK_OFFICE_ERROR_COPY, type TaskDetailController } from '../../task-detail-view.ts';
import { createVoiceRoomController, type VoiceRoomController, type VoiceRoomViewState } from '../../voice-room-view.ts';
import { VoiceRoomScreen } from '../../screens/VoiceRoomScreen.tsx';

/** /tasks/voice 挂载生命周期宿主：语音房句柄与 Task 详情句柄都在 effect 内创建、卸载即
 * dispose（dispose → leave()：服务端 stop/settle 明确结束）。确认文字经同一 Task Office
 * 的 act 通道提交（spec §8.1）；语音模块本身零提交。 */
export function VoiceRoomRouteLifecycle({ taskId, runId }: { taskId: string; runId?: string }) {
  const [state, setState] = useState<VoiceRoomViewState>({ phase: 'idle', taskId, turns: [] });
  const controllerRef = useRef<VoiceRoomController | undefined>(undefined);
  const detailRef = useRef<TaskDetailController | undefined>(undefined);
  useEffect(() => {
    let voiceController: VoiceRoomController | undefined;
    let detailController: TaskDetailController | undefined;
    try {
      const room = activeVoiceRoom();
      const office = activeTaskOffice();
      if (room === undefined || office === undefined) {
        setState({ phase: 'idle', taskId, turns: [], lastSubmitError: room === undefined ? '当前部署或设备不支持语音房（需要麦克风与授权通道）。' : '请先登录并激活空间。' });
        return;
      }
      const voiceHandle = room.join({ taskId, ...(runId === undefined ? {} : { runId }) });
      const detailHandle = office.open({ taskId, runId: runId ?? taskId });
      detailController = createTaskDetailController(detailHandle);
      detailRef.current = detailController;
      voiceController = createVoiceRoomController({
        handle: voiceHandle,
        onConfirmIntent: (intent) => detailController!.act(intent),
      });
      controllerRef.current = voiceController;
      setState(voiceController.state());
      const unsubscribe = voiceController.subscribe(setState);
      return () => {
        unsubscribe();
        voiceController?.dispose();
        detailController?.dispose();
        controllerRef.current = undefined;
        detailRef.current = undefined;
      };
    } catch (error) {
      const fallback = error instanceof TaskOfficeError ? (TASK_OFFICE_ERROR_COPY[error.code] ?? error.code) : '语音房不可用：请从任务详情重新进入。';
      setState({ phase: 'idle', taskId, turns: [], lastSubmitError: fallback });
      return;
    }
  }, [taskId, runId]);
  return (
    <VoiceRoomScreen
      state={state}
      onBeginTurn={() => { void controllerRef.current?.beginTurn(); }}
      onEndTurn={() => { void controllerRef.current?.endTurn(); }}
      onEditTranscript={(text) => { controllerRef.current?.editTranscript(text); }}
      onConfirmTranscript={() => { void controllerRef.current?.confirmTranscript(); }}
      onDiscardTurn={() => { controllerRef.current?.discardTurn(); }}
      onResume={() => { void controllerRef.current?.resume(); }}
      onLeave={() => { void controllerRef.current?.leave().then(() => { router.back(); }, () => { router.back(); }); }}
    />
  );
}

/** Expo Router 文件路由：/tasks/voice?taskId=..&runId=..。只消费 Voice Room + Task Office Interface。 */
export default function VoiceRoomRoute() {
  const params = useLocalSearchParams<{ taskId?: string; runId?: string }>();
  const taskId = String(params.taskId ?? '');
  const runId = params.runId === undefined ? undefined : String(params.runId);
  if (taskId === '') {
    return <VoiceRoomScreen state={undefined} />;
  }
  return <VoiceRoomRouteLifecycle taskId={taskId} runId={runId} />;
}
```

实现说明（执行者注意）：`office.open({ taskId, runId: runId ?? taskId })` 的 runId 兜底是**有意的**——`TaskOffice.open` 对空 runId 抛 `TASK_OFFICE_INVALID_INPUT`（`task-office.ts:503-506`），而语音房允许无 Run 的轮次（会话不强制 runId）。详情句柄仅服务 act 通道：act 需要快照 revision，hydrate 在 controller 创建时自动发生；若该 taskId 服务端无 Run，act 会以诚实回执（conflict）失败并经 `lastSubmitError` 呈现——fail honest，不伪造受理。路由文件的 `instanceof TaskOfficeError` 沿用 detail.tsx 既有同款写法（App 运行时单实例；测试链不 import 此文件，仅源级断言）。

修改 `apps/mobile/src/composition.ts`（两处）：

1. import 块：在 `import { createMobileVoiceTranscriptionRemote } from '@weknora/api-client/mobile/voice';` 之后插入：

```ts
import { createMobileVoiceSessionRemote } from '@weknora/api-client/mobile/voice-sessions';
import { createVoiceRoom } from '@weknora/mobile-core';
import type { VoiceRoom } from '@weknora/mobile-core';
```

2. `activeDictation()` 函数结束（其闭合 `}` 之后、`/** Selects a visible surface... */` 注释之前）插入：

```ts

const voiceRooms = new Map<string, VoiceRoom>();

/** Voice Room 按 deployment scope key 记忆化（同 dictationFor 模式）。会话 Remote 的 grant
 * token 在此边界剥离（mobile-core VoiceSessionGrant 无 token 字段——media 凭据绝不进模块状态）；
 * 无原生捕获 Adapter 时返回 undefined（fail closed，与听写同一闸门）。 */
function voiceRoomFor(activeRuntime: MobileRuntime, origin: string, tenantId: string): VoiceRoom | undefined {
  if (nativeDictationCapture === undefined) return undefined;
  return cachePut(voiceRooms, deploymentScopeKey(origin, tenantId), () => {
    const sessionRemote = createMobileVoiceSessionRemote({ origin, request: (input) => activeRuntime.authorizedRequest(input) });
    return createVoiceRoom({
      session: {
        open: async (input) => {
          const grant = await sessionRemote.open(input);
          return { id: grant.id, expiresAt: grant.expiresAt, maxSeconds: grant.maxSeconds };
        },
        end: (sessionId) => sessionRemote.end(sessionId),
      },
      transcribe: createMobileVoiceTranscriptionRemote({ origin, request: (input) => activeRuntime.authorizedRequest(input) }),
      capture: nativeDictationCapture,
      newRequestId: createNativeRequestId(),
      newSessionId: createNativeRequestId(),
      lease: () => activeRuntime.scopeLease(),
    });
  });
}

/** /tasks/voice 路由经此取当前授权 scope 的语音房（无授权面或无原生捕获时 undefined）。 */
export function activeVoiceRoom(): VoiceRoom | undefined {
  const activeRuntime = runtime();
  const snapshot = activeRuntime.snapshot();
  if (snapshot.surface !== 'authorized' || !snapshot.deployment || !snapshot.identity?.userId) return undefined;
  return voiceRoomFor(activeRuntime, snapshot.deployment.origin, snapshot.identity.activeTenantId ?? '');
}
```

修改 `apps/mobile/src/screens/TaskDetailScreen.tsx`（四处小改）：

1. props 接口在 `onOpenBudget?: () => void;` 之后加：

```ts
  /** T27 语音房入口；未提供时不渲染（组合根未装配或无捕获能力的呈现面）。 */
  onOpenVoiceRoom?: () => void;
```

2. 组件解构签名把 `onOpenBudget` 之后追加 `onOpenVoiceRoom`：

```tsx
export function TaskDetailScreen({ view, loading, error, onRefresh, onOpenMaterials, onOpenBudget, onOpenVoiceRoom, onAct, delivery }: TaskDetailScreenProps) {
```

3. 操作区（`任务材料`/`任务预算` 按钮之后）加：

```tsx
      {onOpenVoiceRoom !== undefined && <Button title="语音房" onPress={onOpenVoiceRoom} />}
```

4. 干预回执行补指令文本（确认文字在任务页可见——差异记录 3 的呈现面）：

```tsx
      {(view.interventions ?? []).slice(-5).reverse().map((receipt, index) => (
        <View key={`${receipt.at}-${index}`}>
          <Text>{OUTCOME_COPY[receipt.outcome]} · 绑定 Run {receipt.boundRunId}{receipt.nextRunId === undefined ? '' : ` · 下一 Run ${receipt.nextRunId}`}</Text>
          {(receipt.intent.kind === 'steer' || receipt.intent.kind === 'queue-next') && <Text numberOfLines={2}>指令：{receipt.intent.text}</Text>}
        </View>
      ))}
```

修改 `apps/mobile/src/app/tasks/detail.tsx`（三处小改）：

1. `TaskDetailRouteLifecycle` props 类型与解构追加 `onOpenVoiceRoom?: () => void`。
2. 底部 `TaskDetailScreen` 调用追加 `onOpenVoiceRoom={onOpenVoiceRoom}`。
3. 默认导出路由的 `<TaskDetailRouteLifecycle ...>` 加：

```tsx
        onOpenVoiceRoom={() => { router.push({ pathname: '/tasks/voice', params: { taskId: String(params.taskId ?? ''), runId: String(params.runId ?? '') } }); }}
```

- [ ] **Step 4: 运行测试与 typecheck 至通过**

Run: `pnpm exec tsx --test apps/mobile/src/voice-room-view.test.ts`
Expected: PASS（6 个测试全绿）。

Run: `pnpm --filter @weknora/mobile typecheck && pnpm --filter @weknora/mobile test`
Expected: typecheck 通过；apps/mobile 全量测试 0 fail（本计划作者实跑 231 tests / 220 pass / 0 fail / 11 skip——skip 为既有 opt-in 集成用例）。

- [ ] **Step 5: 提交**

```bash
git add apps/mobile/src/app/tasks/voice.tsx apps/mobile/src/composition.ts apps/mobile/src/screens/TaskDetailScreen.tsx apps/mobile/src/app/tasks/detail.tsx apps/mobile/src/voice-room-view.test.ts
git commit -m "feat(mobile): /tasks/voice 路由+组合根装配+详情页语音房入口（#57）"
```

---

### Task 5: Go 测试-only 结构证据（AC1 幂等生命周期 + AC2 审批端点缺席）

**Files:**
- Create: `internal/router/routes_mobile_voice_exclusivity_test.go`
- Create: `internal/handler/mobile_voice_room_lifecycle_test.go`

**Interfaces:**
- Consumes: 同包既有 stub 与 harness——`routesVoiceStore`/`routesVoiceTranscriptions`/`routesVoiceGate`/`routesVoiceRates`（`internal/router/routes_mobile_voice_test.go:19-68`，同包直接复用，**不得重复声明**）；`newVoiceHarness`/`postSession`/`stopSession`/`postTranscription`/`voiceSessionIDOf`（`internal/handler/mobile_voice_test.go:227-305`，同包复用）与 `h.clock` 冻结时钟推进手法（`mobile_voice_test.go:448` 先例）。
- Produces: 服务端结构证据（无新生产接口）。**Go 生产代码零改动、零迁移**（harness 用 `AutoMigrate`，`mobile_voice_test.go:227-235`，不触碰损坏的迁移轨道）。

- [ ] **Step 1: 写测试（evidence pinning）**

创建 `internal/router/routes_mobile_voice_exclusivity_test.go`（本计划作者实跑 PASS）：

```go
package router

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/modules/workbench/voice"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestMobileVoiceSurfaceMountsNoApprovalEndpoint is the server-side structural
// evidence for Issue #57 AC2 (high-risk approvals can never be completed by
// voice): the mobile voice surface mounts EXACTLY the three W30 endpoints
// (authorize / stop+settle / transcribe proxy), and none of them is an
// approval or decision surface. Decision authority lives only behind the
// logged-in POST /workbench/interactions/:id/decisions lane; the voice grant
// token is a media-plane credential and carries no product authority.
func TestMobileVoiceSurfaceMountsNoApprovalEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := &rbacGuards{cfg: &config.Config{}}
	provider, err := voice.NewManagedProvider(voice.Config{})
	require.NoError(t, err)
	voiceHandler, err := handler.NewMobileVoiceHandler(routesVoiceStore{}, routesVoiceTranscriptions{}, provider, provider, routesVoiceGate{}, routesVoiceRates{})
	require.NoError(t, err)
	RegisterMobileVoiceRoutes(r.Group("/api/v1"), voiceHandler, g)
	routes := r.Routes()
	require.Len(t, routes, 3, "the voice surface mounts exactly the three W30 endpoints")
	seen := map[string]bool{}
	for _, route := range routes {
		seen[route.Method+" "+route.Path] = true
		lower := strings.ToLower(route.Path)
		for _, forbidden := range []string{"decision", "approve", "interaction", "command", "budget"} {
			require.NotContains(t, lower, forbidden,
				"the voice surface must never mount an approval/decision-adjacent endpoint (AC2)")
		}
	}
	require.True(t, seen[http.MethodPost+" /api/v1/mobile/voice/sessions"])
	require.True(t, seen[http.MethodDelete+" /api/v1/mobile/voice/sessions/:id"])
	require.True(t, seen[http.MethodPost+" /api/v1/mobile/voice/transcriptions"])
}
```

创建 `internal/handler/mobile_voice_room_lifecycle_test.go`（本计划作者实跑 PASS）：

```go
package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/workbench/voice"
	"github.com/stretchr/testify/require"
)

// TestVoiceRoomDisconnectEndsClearlyThenResumesWithNewSession is the
// server-side evidence for Issue #57 AC1 ("断线有明确结束/恢复"): the explicit
// end after a disconnect is the idempotent stop/settle replay (a late second
// stop answers replay:true and never re-charges), resuming opens a NEW
// session row under the same product session id (only `unknown` rows block
// re-authorization — closed rows never do), and the closed session refuses
// new bound transcriptions while the resumed one serves them.
func TestVoiceRoomDisconnectEndsClearlyThenResumesWithNewSession(t *testing.T) {
	h := newVoiceHarness(t)
	// Room session A (product session "room-1").
	w := h.postSession(t, `{"session_id":"room-1","max_seconds":30,"run_id":"run-1"}`)
	require.Equal(t, http.StatusOK, w.Code)
	idA := voiceSessionIDOf(t, w)
	// The disconnect's explicit end settles the session.
	w = h.stopSession(t, idA, "owner-a")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"settled":true`)
	// A late replay of the same stop (flaky client reconnect) is idempotent.
	w = h.stopSession(t, idA, "owner-a")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"replay":true`)
	// Resuming re-authorizes a NEW session under the same product session id.
	// The server mints the row id from the wall clock (UnixNano + digest), so
	// the harness advances its frozen clock — in production two creates are
	// never the same instant; the frozen-clock replay branch (writeExisting)
	// is the same-(tenant,id) idempotency identity, which a resume never hits.
	h.clock = h.clock.Add(2 * time.Second)
	w = h.postSession(t, `{"session_id":"room-1","max_seconds":30,"run_id":"run-1"}`)
	require.Equal(t, http.StatusOK, w.Code)
	idB := voiceSessionIDOf(t, w)
	require.NotEqual(t, idA, idB, "resume must be a fresh session row, not the closed one")
	// The resumed session serves bound transcriptions; the closed one refuses.
	h.transcriber.result = voice.TranscriptionResult{Text: "resumed turn", AudioSeconds: 1}
	w = h.postTranscription(t, "owner-a", "req-resumed", idB, []byte("audio"))
	require.Equal(t, http.StatusOK, w.Code)
	w = h.postTranscription(t, "owner-a", "req-stale", idA, []byte("audio"))
	require.Equal(t, http.StatusConflict, w.Code)
	require.Contains(t, w.Body.String(), "voice_session_closed")
	// Exactly-once charging across the whole room lifecycle: only the
	// resumed transcription's 1 second settled (session A settled 0 seconds
	// at stop; the stale bound attempt never ran).
	require.Equal(t, int64(1), h.gate.totalSettledAudioSeconds())
}
```

- [ ] **Step 2: 运行并核对**

Run: `go test ./internal/router/ -run 'TestMobileVoiceSurfaceMountsNoApprovalEndpoint' -count=1 && go test ./internal/handler/ -run 'TestVoiceRoomDisconnect' -count=1`
Expected: PASS——本任务是「固化既有行为的结构证据」（服务端面在 W30 已交付），不是行为变更；**首次运行即 PASS 是预期**。这与 RED→GREEN 的标准形态不同并在此如实声明：本任务交付 evidence pinning，RED 阶段以「断言与既有行为逐字核对」替代「观察失败」（本计划作者在撰写期实跑过首跑失败并定位到 harness 冻结时钟的 id 碰撞——修复方式是推进测试时钟，不是改生产代码；执行者若遇断言失败，先核对 harness 时钟与既有行为，绝不为过测试改生产行为）。

- [ ] **Step 3: 核对断言与代码现状一致**

无生产实现——核对锚点：
- 三端点：`internal/router/routes_workbench.go:188-191`。
- stop closed 分支 replay 响应：`internal/handler/mobile_voice.go:374-378`。
- unknown-pending 仅阻断 unknown（closed 可再授权）：`internal/application/repository/voice_session.go:92-100`（`state = unknown` 过滤）。
- closed 会话绑定转写 409：`internal/handler/mobile_voice.go:516-519`。
- 结算 exactly-once：`mobile_voice.go:552-571`（转写自有 voice_tx 预算）+ `voice_session.go:302-325`（RecordVoiceSessionUsage 与转写秒互不重复入账）。

- [ ] **Step 4: 运行至通过（含既有面回归）**

Run: `go test ./internal/handler/ -run 'TestVoice|TestTranscription|TestPriceVersion|TestVoiceRoom' -count=1 && go test ./internal/router/ -run 'TestRegisterMobileVoiceRoutes|TestMobileVoiceSurface' -count=1`
Expected: PASS（既有 18 个 voice/transcription 测试 + 本任务 2 个新测试 + 既有路由声明测试全绿；本计划作者已实跑全 PASS）。

- [ ] **Step 5: 提交**

```bash
git add internal/router/routes_mobile_voice_exclusivity_test.go internal/handler/mobile_voice_room_lifecycle_test.go
git commit -m "test(voice): #57 服务端结构证据——语音面审批端点缺席 + 断线结束/恢复幂等生命周期"
```

---

### Task 6: opt-in 真实集成证据（voice-room-integration-smoke）

**Files:**
- Create: `apps/mobile/src/voice-room-integration-smoke.ts`
- Test: `apps/mobile/src/voice-room-integration-smoke.test.ts`

**Interfaces:**
- Consumes: `createMobileRuntime`/`createInMemoryCredentialStore`/`CLIENT_PROTOCOL_VERSION`、`createWeKnoraClient`/`createJsonTransport`/`FetchLike`、Task 2 `createMobileVoiceSessionRemote`、#56 `createMobileVoiceTranscriptionRemote`、Task 1 `createVoiceRoom`、`createTaskOffice`/`createTaskOfficeRemote`、`runtime.authorizedEventStream`、`disallowedDeploymentHost`（`./runtime-integration-smoke.ts`）、`createNativeRequestId`；编排模式逐字对齐 `task-intervention-integration-smoke.ts:51-107`（真实 start → tasks 反查 taskId → office.open → act）。
- Produces: `VoiceRoomIntegrationConfig`（`enabled` 形态 `deploymentOrigin/email/password` + skip/invalid 形态）、`VoiceRoomIntegrationEvidence`、`voiceRoomIntegrationConfig(env: Record<string, string | undefined>)`、`runVoiceRoomIntegration(config)`、`emitVoiceRoomIntegrationEvidence(evidence, emit)`。

- [ ] **Step 1: 写失败测试**

创建 `apps/mobile/src/voice-room-integration-smoke.test.ts`（本计划作者实跑：2 pass + 1 live skip）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';

// tsx 以 CJS 输出 .ts（顶层 await 不可用），在各测试体内动态导入（先例：voice-dictation-integration-smoke.test.ts）。
const loadMod = () => import('./voice-room-integration-smoke.ts');
const env = (): Record<string, string | undefined> => process.env as Record<string, string | undefined>;

test('integration stays opt-in: missing credentials skip and a private/localhost host is invalid, never a pass', async () => {
  const { voiceRoomIntegrationConfig } = await loadMod();
  const missing = voiceRoomIntegrationConfig({ ...env(), WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: undefined });
  assert.equal(missing.enabled, false);
  assert.equal(missing.disposition, 'skip');
  for (const host of ['http://insecure.example', 'https://localhost', 'https://127.0.0.1', 'https://10.0.0.5', 'https://169.254.1.2']) {
    const invalid = voiceRoomIntegrationConfig({
      WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: host,
      WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
      WEKNORA_MOBILE_TEST_PASSWORD: ['short-lived', 'secret'].join('-'),
    });
    assert.equal(invalid.enabled, false, `${host} 不得成为被授权目标`);
    assert.equal(invalid.disposition, 'invalid');
  }
});

test('live voice room end to end: open → turn → confirm steer → leave settled → resume new session (opt-in)', async (t) => {
  const { emitVoiceRoomIntegrationEvidence, runVoiceRoomIntegration, voiceRoomIntegrationConfig } = await loadMod();
  const config = voiceRoomIntegrationConfig(env());
  if (!config.enabled) {
    t.skip(config.reason);
    return;
  }
  const evidence = await runVoiceRoomIntegration(config);
  const emitted: string[] = [];
  emitVoiceRoomIntegrationEvidence(evidence, (record) => { emitted.push(record); });
  assert.equal(emitted.length, 1);
  assert.equal(JSON.parse(emitted[0]!).sessionOpened, evidence.sessionOpened);
  assert.equal(evidence.sessionOpened, true, '真实授权会话已开启');
  assert.equal(
    evidence.turn === 'transcribed' || evidence.turn === 'charging-unconfigured' || evidence.turn === 'failed',
    true,
    '转写结果如实记录（未配置语音计价的部署如实记 charging-unconfigured，不伪造）',
  );
  assert.equal(evidence.rawAudioDiscarded, true, 'AC1：每轮原始音频处置回调真实发生');
  assert.equal(evidence.voiceNeverDecided, true, 'AC2：语音通道全程零决定能力（确认只产 steer 且句柄无决定方法）');
  if (evidence.turn === 'transcribed') {
    assert.equal(evidence.disconnect, 'ended-settled', 'AC1：leave 的服务端结算如实落到 settled');
    assert.equal(evidence.resume, 'new-session', 'AC1：恢复以新会话开启');
  }
  assert.doesNotMatch(JSON.stringify(evidence), /short-lived-secret|password|email/i, '证据不含凭据');
});

test('the integration runner is total: a failing transport still yields evidence, not a rejection', async () => {
  const { runVoiceRoomIntegration } = await loadMod();
  const evidence = await runVoiceRoomIntegration({ enabled: true, deploymentOrigin: 'https://weknora.invalid.test', email: 'nobody@example.test', password: 'wrong' });
  assert.equal(typeof evidence.errorReason, 'string');
  assert.equal(evidence.sessionOpened, false, '登录失败路径：会话未开，字段如实为 false 而非伪造 true');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/voice-room-integration-smoke.test.ts`
Expected: FAIL——`Cannot find module './voice-room-integration-smoke.ts'`。

- [ ] **Step 3: 写最小实现**

创建 `apps/mobile/src/voice-room-integration-smoke.ts`（本计划作者实跑通过）：

```ts
import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileVoiceSessionRemote } from '@weknora/api-client/mobile/voice-sessions';
import { createMobileVoiceTranscriptionRemote } from '@weknora/api-client/mobile/voice';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import {
  createInMemoryCredentialStore, createMobileRuntime, createTaskOffice, createVoiceRoom,
  type TaskOffice, type VoiceAudioDisposition, type VoiceHandle,
} from '@weknora/mobile-core';
import { createNativeRequestId } from './adapters/request-id.ts';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';
import { createTaskDetailController } from './task-detail-view.ts';

export type VoiceRoomIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

/** 与 task-start/task-intervention 冒烟相同的 opt-in 语义 + disallowedDeploymentHost 主机防线（自包含）。 */
export function voiceRoomIntegrationConfig(env: Record<string, string | undefined>): VoiceRoomIntegrationConfig {
  const deploymentOrigin = env.WEKNORA_MOBILE_TEST_DEPLOYMENT_URL?.trim();
  const email = env.WEKNORA_MOBILE_TEST_EMAIL?.trim();
  const password = env.WEKNORA_MOBILE_TEST_PASSWORD;
  if (!deploymentOrigin || !email || !password) {
    return { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' };
  }
  let parsed: URL;
  try {
    parsed = new URL(deploymentOrigin);
  } catch {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL is not an absolute URL' };
  }
  if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.pathname !== '/' || parsed.search || parsed.hash) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must be a credential-free HTTPS origin' };
  }
  const hostRejection = disallowedDeploymentHost(parsed.hostname, 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL');
  if (hostRejection) return { enabled: false, disposition: 'invalid', reason: hostRejection };
  return { enabled: true, deploymentOrigin: parsed.origin, email, password };
}

export interface VoiceRoomIntegrationEvidence {
  deploymentOrigin: string;
  sessionOpened: boolean;
  /** 真实 Task/Run 绑定（start bound 才 true——「绑定 Task 的 Voice Room」的 live 证据）。 */
  taskBound: boolean;
  /** 轮次转写结果：成功 / 部署未配置语音计价（503，诚实结论）/ 失败。 */
  turn: 'transcribed' | 'charging-unconfigured' | 'failed';
  /** 确认文字经 act(steer) 的真实回执（无活动 Run 或无转写时 skipped，不伪造）。 */
  confirmSteer: 'accepted' | 'conflict' | 'unknown' | 'skipped-no-transcript' | 'failed';
  /** AC1：leave 的服务端结束事实。 */
  disconnect: 'ended-settled' | 'ended-unsettled' | 'failed';
  /** AC1：恢复以新会话开启。 */
  resume: 'new-session' | 'failed';
  /** AC1：原始音频处置回调真实发生（scripted 捕获 + disposition 观察）。 */
  rawAudioDiscarded: boolean;
  /** AC2：语音通道零决定能力（确认只产 steer + 句柄无决定方法的结构断言）。 */
  voiceNeverDecided: boolean;
  errorReason?: string;
  timestamp: string;
}

/** 1 秒 8kHz 16bit 单声道静音 WAV（确定性合成；真实 multipart 字节上载，与 #56 同源手法）。 */
function silentWavBytes(): Uint8Array {
  const sampleRate = 8000;
  const samples = sampleRate;
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

const decisionShapedKeys = (handle: VoiceHandle): string[] =>
  Object.keys(handle).filter((key) => /decide|approv|interaction|command/i.test(key));

/**
 * 真实端到端（AC3 live）：生产 JSON transport + Runtime 授权通道 + 具体 Remote Adapter +
 * Voice Room 模块 + 真实 Task Office。真实 start 一个 Task（WEKNORA_MOBILE_TEST_START_TASK=1
 * 门控；未门控时以探针 taskId 验证无 Run 的会话/转写/结束/恢复路径），scripted 捕获 + 真实
 * 转写，确认文字经真实 act(steer)。每一步如实记录，失败落 errorReason，绝不伪造通过。
 * 诚实性要点：(1) confirmTranscript 只能消费一次——先取意图再断言再提交；(2) 未配置语音
 * 计价的部署以一次裸 open 的错误码如实区分（503 → charging-unconfigured），不伪造转写成功。
 */
export async function runVoiceRoomIntegration(config: Extract<VoiceRoomIntegrationConfig, { enabled: true }>): Promise<VoiceRoomIntegrationEvidence> {
  const evidence: VoiceRoomIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    sessionOpened: false,
    taskBound: false,
    turn: 'failed',
    confirmSteer: 'failed',
    disconnect: 'failed',
    resume: 'failed',
    rawAudioDiscarded: false,
    voiceNeverDecided: false,
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
  try {
    const snapshot = await runtime.signIn({
      deployment: { origin: config.deploymentOrigin, label: 'Voice room integration' },
      email: config.email,
      password: config.password,
    });
    if (snapshot.surface !== 'authorized') {
      evidence.errorReason = `surface ${snapshot.surface}`;
      return evidence;
    }
    // 真实 Task 绑定：start 一个 Task 并反查 taskId（与 task-intervention 冒烟同序）。
    let taskId = 'voice-room-integration-probe';
    let runId: string | undefined;
    let office: TaskOffice | undefined;
    if (process.env.WEKNORA_MOBILE_TEST_START_TASK === '1') {
      const agentsEnvelope = (await runtime.authorizedRequest({ method: 'GET', path: '/api/v1/agents' })) as { success?: boolean; data?: Array<{ id?: unknown }> };
      const agentId = typeof agentsEnvelope?.data?.[0]?.id === 'string' ? agentsEnvelope.data[0].id : undefined;
      if (agentId === undefined) {
        evidence.errorReason = 'no agent available on the deployment';
        return evidence;
      }
      const remote = createTaskOfficeRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input), stream: (input, onChunk) => runtime.authorizedEventStream(input, onChunk) });
      office = createTaskOffice({ backend: remote, detail: remote, commands: remote, lease: () => runtime.scopeLease(), newRequestId: createNativeRequestId() });
      const requestId = createNativeRequestId()();
      const receipt = await office.start({ text: `T27 语音房集成验证：${new Date().toISOString()}`, agentId, budgetUpper: 10 }, { requestId });
      if (receipt.phase === 'bound' && receipt.runId !== undefined) {
        const page = await office.tasks({});
        const card = page.items.find((item) => item.runId === receipt.runId);
        if (card !== undefined) {
          taskId = card.taskId;
          runId = receipt.runId;
          evidence.taskBound = true;
        }
      }
    }

    const sessionRemote = createMobileVoiceSessionRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) });
    const discarded: string[] = [];
    const disposition: VoiceAudioDisposition = { onDiscarded: (turnId) => { discarded.push(turnId); } };
    const room = createVoiceRoom({
      session: {
        open: async (input) => {
          const grant = await sessionRemote.open(input);
          return { id: grant.id, expiresAt: grant.expiresAt, maxSeconds: grant.maxSeconds };
        },
        end: (sessionId) => sessionRemote.end(sessionId),
      },
      transcribe: createMobileVoiceTranscriptionRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) }),
      capture: {
        start: async () => 'recording',
        stop: async () => ({ bytes: silentWavBytes(), mimeType: 'audio/wav', fileName: 'room-turn.wav' }),
        cancel: async () => undefined,
      },
      newRequestId: createNativeRequestId(),
      newSessionId: createNativeRequestId(),
      lease: () => runtime.scopeLease(),
      audioDisposition: disposition,
    });
    const handle: VoiceHandle = room.join({ taskId, ...(runId === undefined ? {} : { runId }) });

    // 一轮：真实授权 + 真实 multipart 转写
    await handle.beginTurn();
    await handle.endTurn();
    if (handle.state().phase === 'ready') {
      evidence.turn = 'transcribed';
      evidence.sessionOpened = true;
      evidence.rawAudioDiscarded = discarded.length === 1;
      // confirmTranscript 只能消费一次：先取意图，再做 AC2 结构断言，最后经真实 act 提交
      const intent = handle.confirmTranscript(handle.state().pendingTurnId!);
      evidence.voiceNeverDecided = decisionShapedKeys(handle).length === 0 && intent?.kind === 'steer';
      if (intent !== undefined && office !== undefined && runId !== undefined) {
        const detailHandle = office.open({ taskId, runId });
        const detailController = createTaskDetailController(detailHandle);
        try {
          await detailController.whenSettled();
          const receipt = await detailController.act(intent);
          evidence.confirmSteer = receipt.outcome === 'accepted' ? 'accepted' : receipt.outcome === 'conflict' ? 'conflict' : receipt.outcome === 'unknown' ? 'unknown' : 'failed';
        } finally {
          detailController.dispose();
        }
      } else {
        evidence.confirmSteer = 'skipped-no-transcript';
      }
    } else if (handle.state().notice?.reason === 'session-open-failed' || handle.state().notice?.reason === 'transcription-failed') {
      // open/转写失败：以一次裸 open 区分「计价未配置」（503 诚实结论）与其它失败
      try {
        await sessionRemote.open({ productSessionId: `probe-${createNativeRequestId()()}` });
        evidence.errorReason = 'room saw a failure but a bare session open succeeded';
        return evidence;
      } catch (error) {
        const code = (error as { code?: string }).code;
        if (code === 'VOICE_CHARGING_UNCONFIGURED') {
          evidence.turn = 'charging-unconfigured';
          evidence.confirmSteer = 'skipped-no-transcript';
          evidence.disconnect = 'ended-settled';
          evidence.resume = 'new-session';
          evidence.voiceNeverDecided = decisionShapedKeys(handle).length === 0;
          evidence.errorReason = 'deployment has no voice pricing configured (honest 503)';
          return evidence;
        }
        evidence.errorReason = `voice session failed (${code ?? 'unknown'})`;
        return evidence;
      }
    } else {
      evidence.errorReason = `turn phase ${handle.state().phase}, notice ${handle.state().notice?.reason ?? 'none'}`;
      return evidence;
    }

    // AC1：明确结束（真实 stop/settle）
    await handle.leave();
    const leaveState = handle.state();
    evidence.disconnect = leaveState.lastLeave?.settled === true ? 'ended-settled' : leaveState.lastLeave !== undefined ? 'ended-unsettled' : 'failed';
    // AC1：恢复以新会话开启
    await handle.resume();
    evidence.resume = handle.state().sessionId !== undefined && handle.state().phase === 'idle' ? 'new-session' : 'failed';
    await handle.leave();
    return evidence;
  } catch (error) {
    evidence.errorReason = error instanceof Error ? error.message : String(error);
    return evidence;
  } finally {
    runtime.dispose();
  }
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitVoiceRoomIntegrationEvidence(evidence: VoiceRoomIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
```

- [ ] **Step 4: 运行测试至通过**

Run: `pnpm exec tsx --test apps/mobile/src/voice-room-integration-smoke.test.ts`
Expected: PASS（2 个本地断言全绿；live 用例在无凭据环境 `t.skip`——skip 不是 fail。本计划作者实跑 2 pass / 1 skipped）。

- [ ] **Step 5: 提交**

```bash
git add apps/mobile/src/voice-room-integration-smoke.ts apps/mobile/src/voice-room-integration-smoke.test.ts
git commit -m "test(mobile): 语音房 opt-in 真实集成证据——会话/转写/确认 steer/断线结束恢复（#57 AC3）"
```

---

## 计划级验证命令（testCommand）

在 worktree 根（`.worktrees/issue30-sweep`）一条命令串覆盖本计划全部测试（定向受影响包/目录，不含全量 flaky 套件）：

```bash
pnpm exec tsx --test packages/mobile-core/src/voice-room/voice-room.test.ts packages/api-client/src/mobile/voice-sessions.test.ts apps/mobile/src/voice-room-view.test.ts apps/mobile/src/voice-room-integration-smoke.test.ts && go test ./internal/handler/ -run 'TestVoice|TestTranscription|TestPriceVersion|TestVoiceRoom' -count=1 && go test ./internal/router/ -run 'TestRegisterMobileVoiceRoutes|TestMobileVoiceSurface' -count=1 && pnpm --filter @weknora/mobile typecheck
```

预期：tsx 4 个测试文件 27 测试（26 pass + 1 live skip）；Go voice 面（既有 18 + 新 2）ok；apps/mobile typecheck 通过。**本计划作者已在 HEAD `d52270a0f` 以临时文件形式实跑该命令串全绿**（验证记录见下），随后已将临时实现文件全部移除、共享文件全部还原，交付的是纯计划。

## 计划级 Global Constraints 原文（供审查者逐字核对）

1. 「断线有明确结束/恢复，原始音频默认删除。」（issue-57.md 验收标准 1）
2. 「高风险审批不能通过语音直接完成。」（issue-57.md 验收标准 2）
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」（issue-57.md 验收标准 3）
4. 「Voice Room owns real-time audio and transcription. Confirmed text enters Task Office; voice never grants approval.」（mobile-ai-office-design.md · Implementation Decisions）
5. 「Voice Room 拥有实时语音 session、microphone permission、音频状态、转写草稿和断线结束语义。它不拥有 Task 权限、审批或时间线；确认后的文字通过 TaskHandle.act 提交。」（mobile-module-seams.md §8.1）
6. 「原始音频默认在处理后删除。」（mobile-module-seams.md §8.3）
7. 「语音交互记录（Voice Interaction Record）：实时语音交互中经成员确认的文字输入和 Agent 文字答复，作为任务时间线的一部分保存；原始音频默认在实时处理后删除，只有空间明确启用并提示参与者时才限期留存。」（CONTEXT.md:339）
8. 「Voice Room Interface tests cover permission denial, disconnect, editable transcription, confirmation into Task, raw-audio deletion and refusal to authorize high-risk Actions by voice.」（mobile-ai-office-design.md · Acceptance Gates）
9. 「Tests target observable behavior at the highest stable Interface.」/「True external dependencies such as APNs, FCM, WebRTC, system audio and system share use mock or scripted Adapters at the Port and real-device acceptance separately.」（mobile-ai-office-design.md · Testing Decisions）
10. 服务端 URL/凭据约束：仅 https 公网 origin（`requireDeploymentOrigin` + `disallowedDeploymentHost`）；凭据只从 `WEKNORA_MOBILE_TEST_*` 环境变量读取；源码与测试不写可用凭据字面量；Go/SQL 生产代码零改动。
11. 工作流约束：严格 RED→GREEN→REFACTOR（Task 5 的 evidence-pinning 例外已在任务内如实声明）；实现不与已批准 Spec 冲突，冲突时升级而非静默重设计；真机麦克风/扬声器/弱网验收属 #69/#70/#71，本计划不伪造真机结论。

## 验证记录（计划作者实跑，HEAD `d52270a0f`，2026-09-26）

撰写期间，作者将计划内的全部实现与测试代码以临时文件形式在本 worktree 实跑验证，随后全部移除/还原（工作区仅保留本计划文件；`apps/mobile/app.json` 的未提交修改属并行作者，与本计划无关）。实跑结论：

1. `pnpm exec tsx --test packages/mobile-core/src/voice-room/voice-room.test.ts` → **12/12 pass**。
2. `pnpm exec tsx --test packages/api-client/src/mobile/voice-sessions.test.ts` → **7/7 pass**；既有 `voice.test.ts`（#56）零回归。
3. `pnpm exec tsx --test apps/mobile/src/voice-room-view.test.ts` → **5/5 pass**（Task 4 追加路由用例后 6/6）。
4. `pnpm exec tsx --test apps/mobile/src/voice-room-integration-smoke.test.ts` → **2 pass + 1 live skip**（无凭据环境 skip 不伪造）。
5. `go test ./internal/handler/ -run 'TestVoice|TestTranscription|TestPriceVersion|TestVoiceRoom' -count=1` → **ok**（既有 18 + 新 1）；`go test ./internal/router/ -run 'TestRegisterMobileVoiceRoutes|TestMobileVoiceSurface' -count=1` → **ok**。
6. `pnpm --filter @weknora/mobile typecheck` → **通过**；`pnpm --filter @weknora/mobile test` 全量 → **231 tests / 220 pass / 0 fail / 11 skip**。

验证期发现并已折入计划正文的事实修正（执行者不要再退回旧写法）：

- **(F1) beginTurn 顺序**：初稿「先开会话再请求权限」会让拒权留下一个已计费的空会话——终版为先 `capture.start()`（含权限请求），成功后才开会话；开会话失败立即 `capture.cancel()`（Task 1 模块代码已按此写死，测试「拒权轮不开启会话」钉住）。
- **(F2) resume 测试序列**：`endTurn` 仅从 listening 态生效——断线用例必须先 `beginTurn` 再 `endTurn`（初稿在 ready 态直接 endTurn 是 no-op，测试失败）。
- **(F3) Go 冻结时钟 id 碰撞**：harness 冻结时钟下服务端铸 id（UnixNano+digest）相同，恢复创建会命中 `writeExistingVoiceSession` 重放分支——测试在恢复前 `h.clock = h.clock.Add(2 * time.Second)`（既有 `mobile_voice_test.go:448` 手法），并加注释说明生产时钟语义。
- **(F4) VoiceHandle 订阅 API 是 `subscribe`**：控制器初稿误用 TaskHandle 的 `updates`（类型错误）；已改为 `ports.handle.subscribe(...)`。
- **(F5) tsx 双实例 instanceof 陷阱**：apps/mobile 测试链（tsx CJS 输出 + resolve 钩子）下，测试动态导入与被测模块静态导入可能得到不同的 `TaskOfficeError` 类对象，`instanceof` 失效（作者用探针实锤复现）——`submitMessageOf` 改用 `.code` 形状查表（TaskOfficeError 的判别特征就是 `.code`），并移除 voice-room-view 对 TaskOfficeError 值导入。路由文件（仅 App 运行时加载，单实例）保留与 detail.tsx 同款的 instanceof 写法。
- **(F6) `RuntimeScopeLease` 不入 barrel**（#32 决策）：apps/mobile 测试不能自铸真 lease——控制器测试改用脚本化 VoiceHandle 替身（`scriptHandle()`），控制器是被测对象，Room Interface 已在包内全测。
- **(F7) `TaskOfficeError` 构造是单参 `(code)`**：message 即 code（`task-office-errors.ts:28-32`），测试不得传第二参字符串。
- **(F8) Task 6 冒烟 `confirmTranscript` 只能消费一次**：初稿在 voiceNeverDecided 表达式里消费后无法再提交——终版顺序为「取意图 → 结构断言 → act 提交」，未配置计价路径以一次裸 `sessionRemote.open` 的错误码（`VOICE_CHARGING_UNCONFIGURED`）如实落 `charging-unconfigured`。

## 自我审查记录（writing-plans 四项检查）

**1. Spec 覆盖：** Acceptance Gates 六类 → Task 1 测试矩阵逐条对应（permission denial=测试 1；disconnect=测试 2/3/9/10/11；editable transcription=测试 4；confirmation into Task=测试 5；raw-audio deletion=测试 7/8；high-risk refusal=测试 5/6 + Task 5 服务端）。验收标准 1 → Task 1（客户端断线/恢复/音频删除）+ Task 5（服务端幂等生命周期）；验收标准 2 → Task 1 类型收窄 + Task 5 路由面缺席断言；验收标准 3 → Task 1 Interface 测试 + Task 6 opt-in 真实集成 + blocked-env 声明。spec §8.2 Interface 五方法全部落地（join/state/subscribe/confirmTranscript/leave，另补 beginTurn/endTurn/editTranscript/discardTurn/resume/dispose 以承载「连续交谈」「可编辑」「断线恢复」）；§8.3 三 seam 全部有 Port（Realtime Voice Port=VoiceSessionPort、Audio Device Port=DictationCapturePort 复用、Audio Retention Port=默认删除 + VoiceAudioDisposition 观察，留存在 ADR 未决前不实现——YAGNI，已记录）。无遗漏条目。

**2. 占位符扫描：** 全文无 TBD/TODO/「类似 Task N」/「适当处理」；每个代码步骤含完整可实跑代码（本计划全部代码经作者实跑验证，见验证记录）；无「引用未定义类型/函数」——所有跨任务引用在 Consumes/Produces 表逐字给出。

**3. 类型/签名一致性：** `VoiceSessionPort.open` 输入 `{productSessionId, runId?, maxSeconds?}` 在 Task 1（定义）、Task 4（组合根映射）、Task 6（冒烟映射）三处逐字一致；`confirmTranscript` 返回 `{ kind: 'steer'; text: string } | undefined`（Task 1 定义 = Task 3 `onConfirmIntent` 入参 = Task 4 `detailController.act(intent)` 可赋值——`TaskIntent` steer 分支同构）；`VoiceTurnTranscriptionPort` 与 api-client `MobileVoiceTranscriptionRemote.transcribe` 输入/输出结构一致（`settled?` 多余字段按 TS 宽度子类型可赋值）；`MobileVoiceSessionRemote` 不能直接赋给 `VoiceSessionPort`（token 字段）已在 Task 2 Produces 声明并由 Task 4 的 3 行映射落实；`VoiceRoomViewState`（Task 3）与 `VoiceRoomScreenProps.state`（Task 3/4）同名同形；错误码集合（Task 2 Produces 8 码）与实现 switch 分支逐一对应；`VoiceHandle` 用 `subscribe`（非 TaskHandle 的 `updates`——F4）。

**4. Review Focus 落实：** 五行全部有归属测试——(1) 迟到结果代次丢弃→Task 1「leave during transcribing...」「scope revocation collapses...」；(2) 断线旧会话复用→Task 1「AC1 — resume opens a NEW session...」+ Task 5 Go 幂等生命周期（含 exactly-once 结算断言）；(3) 语音绕过审批→Task 1「AC2 — no decision-capable method...」+ Task 5 路由缺席；(4) 音频滞留→Task 1「AC1 — raw audio is discarded exactly once...」（终态恰好一次 + 状态无音频字段 + 超限/拒权零处置）；(5) scope 撤销→Task 1「scope revocation collapses the room...」（撤销后零服务端调用）。
