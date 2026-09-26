# plan-t57.md 执行报告

## Task 2: api-client 语音会话 Remote（`./mobile/voice-sessions`）— 实现员报告

**状态：DONE** ｜ 提交：`f7b207325`（feat(api-client): createMobileVoiceSessionRemote——语音会话 open/end wire 适配（#57））

---

## 一、实现内容

按计划 Task 2 逐字交付，恰好三个授权文件，无超范围改动：

1. **`packages/api-client/src/mobile/voice-sessions.ts`（新建）**：`createMobileVoiceSessionRemote({ origin, request })` wire 适配器——
   - `open({ productSessionId, runId?, maxSeconds? })` → `POST /api/v1/mobile/voice/sessions`，body `{ session_id, run_id?, max_seconds? }`（空 runId 不进 body；maxSeconds 校验为 (0,600] 整数）；解析 `{success:true,data:{...}}` 信封为 `MobileVoiceSessionGrant { id; token?; expiresAt; maxSeconds; priceVersion? }`——**token 在 Adapter 边界保留、由 Task 4 组合根剥离**（计划明示的有意可审计边界，grant 绝不进入 mobile-core 模块状态）。
   - `end(sessionId)` → `DELETE /api/v1/mobile/voice/sessions/:id`（`encodeURIComponent`）；回执 `MobileVoiceSessionEnd { id; state; settled; replay?; reason? }` 如实透传服务端三分支（closed 重放 / unknown 待对账 / 正常结算），未知信封字段不透传。
   - 错误语义翻译 8 个码：`VOICE_SESSION_INVALID|VOICE_BUDGET_DENIED|VOICE_SESSION_CONFLICT|VOICE_PROVIDER_UNAVAILABLE|VOICE_OPEN_UNKNOWN|VOICE_CHARGING_UNCONFIGURED|VOICE_SESSION_NOT_FOUND|VOICE_SESSION_END_FAILED`（open 按 status+message，503 按 message 细分 `voice_open_unknown` vs 计费未配置；end 404→NOT_FOUND、其余 HTTP→END_FAILED）。
   - 构造期 `requireDeploymentOrigin` 强校验 origin（HTTPS、无 userinfo、无 path/query/fragment）——与 #56 `voice.ts` 同一共享防线（`deployment-origin.ts`）。
   - 畸形信封拒绝而非信任（`success:true` + `data.id/expires_at/max_seconds` 形状校验）。
2. **`packages/api-client/src/mobile/voice-sessions.test.ts`（新建）**：计划行 855-949 的 7 个测试逐字（唯一偏差见自检发现 1）。
3. **`packages/api-client/package.json`（修改）**：exports 在 `"./mobile/voice"` 之后插入 `"./mobile/voice-sessions": "./src/mobile/voice-sessions.ts"` 一行。

结构可赋值性事实（计划 Produces 声明）：`MobileVoiceSessionRemote` 不直接赋给 mobile-core `VoiceSessionPort`（grant 带 token），组合根 Task 3 行映射剥离——本任务未改动 mobile-core。

## 二、TDD 证据

### RED（先写失败测试，实跑确认失败）

命令：`pnpm exec tsx --test packages/api-client/src/mobile/voice-sessions.test.ts`

关键输出：

```
Error [ERR_MODULE_NOT_FOUND]: Cannot find module '.../packages/api-client/src/mobile/voice-sessions.ts' imported from .../voice-sessions.test.ts
ℹ tests 1
ℹ pass 0
ℹ fail 1
```

失败原因符合预期：模块尚不存在（计划 Step 2 预期 `Cannot find module './voice-sessions.ts'`）。

### GREEN（最小实现后，实跑至通过）

命令（计划 Step 4 原文）：`pnpm exec tsx --test packages/api-client/src/mobile/voice-sessions.test.ts packages/api-client/src/mobile/voice.test.ts`

完整输出（14/14 全绿：新增 7 + 既有 #56 voice 7 个零回归，输出无杂音）：

```
✔ open posts the W30 session wire: session_id/run_id/max_seconds body and parses the grant envelope (0.963042ms)
✔ open parses an idempotent replay envelope without a token (token_issued false path) (0.102625ms)
✔ open error translation covers the full W30 status matrix (0.52975ms)
✔ end deletes the session id (encodeURIComponent) and parses settle receipts honestly (0.179125ms)
✔ end error translation: 404 is not-found, other http failures are end-failed (0.184042ms)
✔ malformed envelopes are rejected rather than trusted (0.132375ms)
✔ origin is validated at construction (0.072584ms)
✔ transcribe posts a multipart body with request_id and an audio file part to the voice endpoint (11.624834ms)
✔ an optional voice_session_id rides the form; a blank one stays absent (0.245792ms)
✔ missing request id or a source-less audio is refused before any wire call (0.333375ms)
✔ a 503 maps to VOICE_CHARGING_UNCONFIGURED; other HTTP failures map to VOICE_TRANSCRIBE_FAILED (0.268375ms)
✔ malformed envelopes are refused (success must be true with a string data.text) (0.194708ms)
✔ the origin must be a validated credential-free HTTPS origin (0.102541ms)
✔ the result surface is exactly the server envelope fields (unknown fields never pass through) (0.122084ms)
ℹ tests 14
ℹ pass 14
ℹ fail 0
```

（注：首次 GREEN 运行即 14/14；随后自检修正测试替身类型注解后复跑，仍 14/14，两次输出一致。）

### 类型检查（自检增量，计划 Task 2 未点名 tsc 命令）

命令：`pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2022 --module NodeNext --moduleResolution NodeNext --allowImportingTsExtensions --types node packages/api-client/src/mobile/voice-sessions.ts packages/api-client/src/mobile/voice-sessions.test.ts`

输出：`TYPECHECK OK`（无错误，exit 0）。

说明：仓库既有 `typecheck:shared` 脚本不覆盖本任务新文件，故按计划 Task 1 Step 4 的同型手法对授权文件直接 tsc。裸 tsc 不带 `--types node` 时对 `node:test` 报 TS2591——已用既有 `voice.test.ts` 对照复现（同一命令同样报错），确认为无 tsconfig 环境产物而非代码缺陷。

## 三、提交

```
f7b207325 feat(api-client): createMobileVoiceSessionRemote——语音会话 open/end wire 适配（#57）
3 files changed, 246 insertions(+)
```

Diff 范围核实（`git status --short` 提交前）：恰好 `M package.json` + 两个新建文件，无其他文件被触碰；未推送远端。

## 四、自检发现

1. **对计划测试代码的一处最小类型修正（运行时行为不变）**：计划给出的测试替身写法 `request: (async (input) => {...}) as never` 在严格 tsc 下报 TS7006（`as never` 抹掉参数上下文类型 → `input` 隐式 any）。已对照既有 `voice.test.ts` 的仓库模式（直接用 `request` 上下文提供 `ClientRequest` 类型），改为 `request: async (input: ClientRequest) => {...}` 并补 `import type { ClientRequest }`。修正前后各实跑一次，均 7/7 / 14/14 通过，行为零变化；修正后测试文件在 `--types node` 下 tsc 严格通过。
2. **Mimosa 提交钩子告警（环境级，非本任务产出缺陷）**：commit 时钩子报告「Mimosa 在 git commit 前没有得到完整扫描结论（scanner_enobufs）」，按兼容策略放行提交。本任务未获授权运行全项目安全扫描，如实转记，不宣称项目安全结论。
3. 无其他发现：未引入凭据字面量（origin 校验共享 B3-F11 防线；测试 origin 为 `weknora.example.test` 假域）；wire 层不出现 React Native/DOM 依赖；未超出计划范围新建任何抽象（YAGNI）。

---

## Task 3: apps/mobile 控制器/文案/屏（voice-room-view + VoiceRoomScreen）— 实现员报告

**状态：DONE** ｜ 提交：`22fdccf16`（feat(mobile): Voice Room 控制器/文案/屏——确认文字经 act 通道提交（#57））

---

## 一、实现内容

按计划 Task 3 逐字交付，恰好三个授权文件，无超范围改动：

1. **`apps/mobile/src/voice-room-view.ts`（新建）**：
   - 四组文案常量：`VOICE_ROOM_PHASE_COPY`（7 相位全覆盖）、`VOICE_ROOM_NOTICE_COPY`（6 通知原因全覆盖，`scope-revoked` 引导「重新进入」而非暴露内部码）、`VOICE_ROOM_SUBMIT_ERROR_PREFIX`、`VOICE_ROOM_APPROVAL_COPY`（AC2 审批互斥的常驻呈现——语音无法代替审批）。
   - `createVoiceRoomController(ports: VoiceRoomControllerPorts): VoiceRoomController`：把 pending 轮次 id 封装在内部（屏永不见 turnId）；`confirmTranscript()` 从 handle 取意图后经 `ports.onConfirmIntent`（宿主 TaskHandle.act 通道）提交——控制器自身零提交语义；`submitting`/`lastSubmitError` 视图状态如实呈现提交失败且不回滚已确认文字（服务端 act 幂等可重试）。
   - 错误映射用 `.code` 形状查表 `TASK_OFFICE_ERROR_COPY` 而非 `instanceof TaskOfficeError`（tsx CJS/ESM 双实例下 instanceof 跨模块不可靠——计划验证期实锤，计划行 1305 明示此手法）。
   - 订阅用 `VoiceHandle.subscribe`（非 TaskHandle 的 `updates`）；dispose 幂等并解绑。
2. **`apps/mobile/src/screens/VoiceRoomScreen.tsx`（新建）**：只见 `VoiceRoomViewState` + 回调，零 wire 导入（module-seams §10）；已确认文字作为语音交互记录常驻可见（CONTEXT.md:339）；审批边界文案常驻；提交中禁用确认按钮。
3. **`apps/mobile/src/voice-room-view.test.ts`（新建）**：计划行 1149-1296 的测试逐字（4 个 `test()`，见自检发现 1）。

## 二、TDD 证据

### RED（先写失败测试，实跑确认失败）

命令：`pnpm exec tsx --test apps/mobile/src/voice-room-view.test.ts`

输出（4/4 fail，符合计划 Step 2 预期——`./voice-room-view.ts` 尚不存在）：

```
code: 'ERR_MODULE_NOT_FOUND',
url: 'file:///...apps/mobile/src/voice-room-view.ts'
✖ the screen renders state + callbacks only, and the view module never imports wire packages
  Error: ENOENT: no such file or directory, open '.../apps/mobile/src/screens/VoiceRoomScreen.tsx'
ℹ tests 4
ℹ pass 0
ℹ fail 4
```

### GREEN（最小实现后，实跑至通过）

命令（计划 Step 4 原文）：`pnpm exec tsx --test apps/mobile/src/voice-room-view.test.ts`

完整输出（4/4 全绿，输出无杂音）：

```
✔ voice room copy covers every phase, notice reason, and never leaks wire detail (357.322ms)
✔ controller wraps the handle: pending-turn identity never escapes, confirm routes through onConfirmIntent (1.667625ms)
✔ controller surfaces submit failure honestly without discarding the confirmed intent (169.258ms)
✔ the screen renders state + callbacks only, and the view module never imports wire packages (6.831917ms)
ℹ tests 4
ℹ suites 0
ℹ pass 4
ℹ fail 0
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 790.754083
```

### 类型检查（仓库完成标准，适用时）

命令：`pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2022 --module NodeNext --moduleResolution NodeNext --allowImportingTsExtensions --jsx react-jsx apps/mobile/src/voice-room-view.ts apps/mobile/src/screens/VoiceRoomScreen.tsx`

结果：exit=0，零类型错误。

### 零回归（相邻既有测试复跑）

命令：`pnpm exec tsx --test apps/mobile/src/voice-dictation-integration-smoke.test.ts apps/mobile/src/dictation-view.test.ts packages/mobile-core/src/voice-room/voice-room.test.ts`

结果：`tests 18 / pass 17 / fail 0 / skipped 1`。唯一 skipped 是 #56 既有 opt-in live 集成用例（输出原文：`live dictation end to end … # missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD`）——缺部署凭据按设计跳过，与本任务改动无关。

## 三、提交与范围

```
22fdccf16 feat(mobile): Voice Room 控制器/文案/屏——确认文字经 act 通道提交（#57）
 3 files changed, 331 insertions(+)
 create mode 100644 apps/mobile/src/screens/VoiceRoomScreen.tsx
 create mode 100644 apps/mobile/src/voice-room-view.test.ts
 create mode 100644 apps/mobile/src/voice-room-view.ts
```

Diff 范围核实（`git status --short` 提交前）：恰好三个新建授权文件，无其他文件被触碰；未推送远端。

## 四、自检发现

1. **计划测试计数笔误（不影响交付）**：计划行 1147 称「本计划作者实跑 5/5 通过」，但其给出的测试文件实含 4 个 `test()`（node:test 计数 4）。按文件逐字执行，实跑 4/4 全绿；无任何测试被省略或改动。
2. **测试文件与计划逐字（零偏差）**：与 Task 2 报告自检发现 1 不同，本任务测试代码未做任何类型修正——`scriptHandle` 替身、`@weknora/mobile-core` 类型注入写法在严格 tsc 与 tsx 下均原样可用。
3. **Mimosa 提交钩子告警（环境级，与 Task 2 报告发现 2 同源）**：commit 时钩子仍报 scanner_enobufs 兼容放行。本任务未获授权运行全项目安全扫描，如实转记。
4. 无其他发现：三个文件均不导入 `@weknora/api-client|contracts`（有测试断言守卫）；无凭据字面量；未超出计划范围新建任何抽象（YAGNI）。

---

## Task 4: 路由、组合根装配与详情入口 — 实现员报告

**状态：DONE** ｜ 提交：`9ad529123`（feat(mobile): /tasks/voice 路由+组合根装配+详情页语音房入口（#57)）

---

### 一、实现内容

按计划 Task 4 逐字交付，恰好五个授权文件（1 新建 + 4 修改），无超范围改动：

1. **`apps/mobile/src/app/tasks/voice.tsx`（新建）**：`/tasks/voice?taskId=..&runId=..` Expo Router 文件路由。`VoiceRoomRouteLifecycle` 在 effect 内创建两个句柄——`activeVoiceRoom().join({taskId, runId?})` 语音房句柄 + `activeTaskOffice().open({ taskId, runId: runId ?? taskId })` 详情句柄（runId 兜底是有意的：`task-office.ts:503-504` 对空 runId 抛 `TASK_OFFICE_INVALID_INPUT`，而语音房允许无 Run 轮次）；确认文字经 `createVoiceRoomController` 的 `onConfirmIntent` 直通 `detailController.act`（spec §8.1 唯一提交通道，语音模块零提交）；卸载即 `voiceController.dispose()`（→ leave → 服务端 stop/settle）+ `detailController.dispose()`。缺 room/office 与 join 抛错两条路径均以 `lastSubmitError` 用户文案 fail honest。路由文件的 `instanceof TaskOfficeError` 沿用 detail.tsx:33 既有同款写法（App 运行时单实例；测试链不 import 此文件，仅源级断言）。
2. **`apps/mobile/src/composition.ts`（两处修改）**：
   - import 块在 `@weknora/api-client/mobile/voice` 之后插入 `createMobileVoiceSessionRemote`（voice-sessions 子路径）+ `createVoiceRoom`/`VoiceRoom`（mobile-core）。
   - `activeDictation()` 之后插入 `voiceRoomFor`（按 `deploymentScopeKey` 经 `cachePut` 记忆化；`nativeDictationCapture === undefined` 时返回 undefined fail closed；`createMobileVoiceSessionRemote` 的 grant 经 3 行映射 `return { id, expiresAt, maxSeconds }` **在组合根边界剥离 token**——mobile-core `VoiceSessionGrant` 无 token 字段）+ 导出 `activeVoiceRoom()`（授权面快照判定与 `activeTaskOffice` 同口径）。
3. **`apps/mobile/src/screens/TaskDetailScreen.tsx`（四处小改）**：props 接口加 `onOpenVoiceRoom?: () => void`（带 T27 注释）；组件解构追加参数；操作区在「任务预算」按钮后加 `{onOpenVoiceRoom !== undefined && <Button title="语音房" onPress={onOpenVoiceRoom} />}`；干预回执行补指令文本——`{(receipt.intent.kind === 'steer' || receipt.intent.kind === 'queue-next') && <Text numberOfLines={2}>指令：{receipt.intent.text}</Text>}`（确认文字在任务页可见——差异记录 3 的呈现面）。
4. **`apps/mobile/src/app/tasks/detail.tsx`（三处小改）**：`TaskDetailRouteLifecycle` props 类型与解构追加 `onOpenVoiceRoom?`；`TaskDetailScreen` 调用透传；默认导出路由加 `onOpenVoiceRoom={() => { router.push({ pathname: '/tasks/voice', params: { taskId: String(params.taskId ?? ''), runId: String(params.runId ?? '') } }); }}`。
5. **`apps/mobile/src/voice-room-view.test.ts`（追加 1 个测试）**：计划行 1533-1546 的路由接线源级断言测试逐字。

### 二、TDD 证据

**RED**（先追加测试，实跑确认失败）：

命令：`pnpm exec tsx --test apps/mobile/src/voice-room-view.test.ts`

关键输出（新增第 5 条失败，原因正是路由文件不存在，符合计划 Step 2 预期；既有 4 条不受影响）：

```
# Subtest: the voice route is an Expo Router screen wired through composition only
not ok 5 - the voice route is an Expo Router screen wired through composition only
  ---
  error: "ENOENT: no such file or directory, open '.../apps/mobile/src/app/tasks/voice.tsx'"
  code: 'ENOENT'
  ...
1..5
# tests 5
# pass 4
# fail 1
```

**GREEN**（实现后同命令实跑通过）：

命令：`pnpm exec tsx --test apps/mobile/src/voice-room-view.test.ts`

```
ok 5 - the voice route is an Expo Router screen wired through composition only
  ---
  duration_ms: 2.142041
  ...
1..5
# tests 5
# pass 5
# fail 0
# skipped 0
```

### 三、全量验证（计划 Step 4 指定命令，原样执行）

命令：`pnpm --filter @weknora/mobile typecheck`

输出：通过（`tsc --noEmit` 零类型错误；仅无关的 node>=26 engine WARN，与既有任务报告同源）。

命令：`pnpm --filter @weknora/mobile test`

完整输出（与计划预期「231 tests / 220 pass / 0 fail / 11 skip」逐字一致，skip 为既有 opt-in 集成用例）：

```
1..231
# tests 231
# suites 0
# pass 220
# fail 0
# cancelled 0
# skipped 11
# todo 0
# duration_ms 11000.377
```

### 四、自检发现

1. **计划测试计数笔误（不影响交付，与 Task 3 报告发现 1 同型）**：计划行 1730 称「PASS（6 个测试全绿）」，按其假设 Task 3 交付 5 个测试 + 本任务追加 1 个。但 Task 3 实际交付 4 个测试（Task 3 报告自检发现 1 已如实记录：计划行 1147 声称 5 个、文件实含 4 个），故本文件现为 5 个测试，实跑 5/5 全绿。关键指标 0 fail 达成；无测试被省略。
2. **并行热点文件核对（计划「并行预警」执行）**：动手前重读了 `TaskDetailScreen.tsx`/`composition.ts`/`detail.tsx` 的锚点上下文，四处/三处修改均按锚点（props 接口、`onOpenBudget` 之后、`activeDictation()` 闭合之后、`TaskDetailRouteLifecycle` 签名）定位，未依赖行号；`git diff --stat` 确认改动形状与计划最小形状一致（+133/-3）。
3. **token 剥离边界落实**：`MobileVoiceSessionGrant.token?`（Task 2 产出保留在 Adapter）在 `voiceRoomFor` 的 open 映射中剥离，mobile-core 模块状态只收 `{id, expiresAt, maxSeconds}`——计划 Produces 声明的「有意的可审计边界」在本任务闭合。
4. **共享文件零冲突**：本任务对三个共享热点的修改均为纯追加/参数透传，未触碰他人锚点（任务材料/预算按钮、干预区、Task Office 装配、交付读器等均原样）。
5. 无其他发现：路由文件不导入 `@weknora/api-client`（有测试断言守卫）；无凭据字面量；未新建计划外抽象。

---

## Task 5: Go 测试-only 结构证据（AC1 幂等生命周期 + AC2 审批端点缺席）— 实现员报告

**状态：DONE** ｜ 提交：`514076335`（test(voice): #57 服务端结构证据——语音面审批端点缺席 + 断线结束/恢复幂等生命周期）

---

### 一、实现内容

按计划 Task 5 逐字交付，恰好两个授权文件，**Go 生产代码零改动、零迁移**：

1. **`internal/router/routes_mobile_voice_exclusivity_test.go`（新建，AC2 服务端证据）**：`TestMobileVoiceSurfaceMountsNoApprovalEndpoint` 断言移动语音面**恰好**挂载 W30 三端点（`POST /mobile/voice/sessions`、`DELETE /mobile/voice/sessions/:id`、`POST /mobile/voice/transcriptions`），且每条路径（小写化后）不含 `decision|approve|interaction|command|budget` 任一禁词——语音通道结构性不具备审批面；决定权只在登录态 `POST /workbench/interactions/:id/decisions` 通道。构造 handler 复用同包既有 stub（`routesVoiceStore`/`routesVoiceTranscriptions`/`routesVoiceGate`/`routesVoiceRates`，`routes_mobile_voice_test.go:19-68`），未重复声明。
2. **`internal/handler/mobile_voice_room_lifecycle_test.go`（新建，AC1 服务端证据）**：`TestVoiceRoomDisconnectEndsClearlyThenResumesWithNewSession` 在真实 handler + sqlite（`newVoiceHarness`，`AutoMigrate`，不装迁移链）上固化断线生命周期：断线显式 end 结算（`"settled":true`）→ 迟到 stop 重放幂等（`"replay":true`，不重复计费）→ 恢复以同一 product session id（`room-1`）开**新会话行**（`idA ≠ idB`；harness 冻结时钟推进 2s，先例 `mobile_voice_test.go:448`）→ 恢复会话受理绑定转写、closed 会话拒绝绑定转写（409 `voice_session_closed`）→ 全生命周期 exactly-once 结算（`h.gate.totalSettledAudioSeconds() == 1`：仅恢复轮的 1 秒入账；会话 A 停止时结算 0 秒，stale 绑定尝试从未执行）。

**TDD 形态的如实声明（计划行 1868 预先声明）**：本任务是 evidence pinning（固化既有行为），非行为变更——RED 阶段以「断言与既有行为逐字核对」替代「观察失败」，**首跑即 PASS 是预期**。计划作者在撰写期实跑过首跑失败（harness 冻结时钟的 id 碰撞），修复方式是推进测试时钟而非改生产代码；本执行者在动手前亦逐字核对了全部 5 个行为锚点（见下），未遇到断言失败。

### 二、Step 3 锚点核对（断言与代码现状一致）

| 锚点 | 亲验结果 |
|---|---|
| 三端点 `routes_workbench.go:188-191` | ✅ `voice.POST("/sessions"…)` / `voice.DELETE("/sessions/:id"…)` / `voice.POST("/transcriptions"…)` |
| stop closed 分支 replay 响应 `mobile_voice.go:374-378` | ✅ closed 分支返回 `"settled":true, "replay":true`（"Repeated stop: already settled, nothing re-charged"） |
| unknown-pending 仅阻断 unknown `voice_session.go:92-100` | ✅ 过滤条件 `state = VoiceSessionStateUnknown`——closed 行不阻断再授权 |
| closed 会话绑定转写 409 `mobile_voice.go:516-519` | ✅ `bound.State == VoiceSessionStateClosed` → 409 `voice_session_closed` |
| 结算 exactly-once `mobile_voice.go:552-571` + `voice_session.go:302-325` | ✅ 转写秒在自有 `voice_tx` reservation 下恰好结算一次（I-1 注释），刻意不累计到会话行；`RecordVoiceSessionUsage` 拒收 closed 行（`state <> closed`），两条身份互不重复入账 |

### 三、测试命令与完整输出

**Step 2 — 两个新测试（实跑，首跑 PASS，符合 evidence-pinning 预期）：**

```
$ go test ./internal/router/ -run 'TestMobileVoiceSurfaceMountsNoApprovalEndpoint' -count=1 -v
=== RUN   TestMobileVoiceSurfaceMountsNoApprovalEndpoint
ERROR[2026-09-26 17:36:17.402] [] rbac.go:331[func1]   | [rbac] middleware constructed with nil/incomplete config …enforcement is permanently disabled…
--- PASS: TestMobileVoiceSurfaceMountsNoApprovalEndpoint (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/router	2.331s
```
（rbac ERROR 行是既有同型测试 `routes_mobile_voice_test.go:77` 同样的 `cfg: &config.Config{}` 最小构造在测试模式下的既有告警，非本任务引入；断言目标为路由声明，非中间件行为。）

```
$ go test ./internal/handler/ -run 'TestVoiceRoomDisconnect' -count=1 -v
（GORM "record not found" 日志为幂等查找的既有正常路径输出）
--- PASS: TestVoiceRoomDisconnectEndsClearlyThenResumesWithNewSession (0.01s)
PASS
ok  	github.com/Tencent/WeKnora/internal/handler	1.681s
```

**Step 4 — 既有面回归（计划命令逐字）：**

```
$ go test ./internal/handler/ -run 'TestVoice|TestTranscription|TestPriceVersion|TestVoiceRoom' -count=1
ok  	github.com/Tencent/WeKnora/internal/handler	3.008s
（-v 复核：--- PASS 19 个 / --- FAIL 0 个 = 18 既有 voice/transcription 测试 + 本任务 1 个新测试）

$ go test ./internal/router/ -run 'TestRegisterMobileVoiceRoutes|TestMobileVoiceSurface' -count=1
ok  	github.com/Tencent/WeKnora/internal/router	2.617s
（-v 复核：--- PASS: TestMobileVoiceSurfaceMountsNoApprovalEndpoint + --- PASS: TestRegisterMobileVoiceRoutesDeclaresTheThreeEndpoints）
```

合计 21 通过 / 0 失败 = 计划行 1882 声明的「既有 18 个 voice/transcription 测试 + 本任务 2 个新测试 + 既有路由声明测试全绿」，计数逐一吻合。

**静态检查：** `gofmt -l` 两文件无输出（clean）；`go vet ./internal/router/ ./internal/handler/` 无输出（clean）。

### 四、自检发现

1. **零范围越界**：`git show --stat 514076335` = 2 files changed, 99 insertions(+), 0 deletions——只有两个新测试文件，无生产代码/迁移改动。
2. **evidence pinning 例外已如实声明**（见上），且未以任何「更快替代」冒充计划命令：Step 2 与 Step 4 均按计划逐字命令实跑。
3. **无凭据字面量**：测试只使用 `owner-a`/`room-1` 等合成标识，token 仅以既有 happy-path 测试的形状出现（本任务未引入任何 token 字面量）。
4. 无其他发现：未派发子代理；未推送远端；未触碰并行热点文件。

## Task 6: opt-in 真实集成证据（voice-room-integration-smoke）— 实现员报告

**状态：DONE** ｜ 提交：`7b949f985`（test(mobile): 语音房 opt-in 真实集成证据——会话/转写/确认 steer/断线结束恢复（#57 AC3)）

---

### 一、实现内容

按计划 Task 6（plan-t57.md:1893-2233）逐字交付，恰好两个授权文件，无超范围改动：

1. **`apps/mobile/src/voice-room-integration-smoke.ts`（新建，245 行）**：
   - `voiceRoomIntegrationConfig(env)`：opt-in 语义与 `task-intervention-integration-smoke.ts` 同型——凭据缺失 → `{enabled:false, disposition:'skip'}`；非 HTTPS / 带 userinfo / 带 path·query·fragment / 主机命中 `disallowedDeploymentHost`（复用 `runtime-integration-smoke.ts` 防线，拒 localhost/环回/私网/链路本地/保留地址）→ `{enabled:false, disposition:'invalid'}`。凭据只从 `WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD` 环境变量读取，源码零凭据字面量。
   - `VoiceRoomIntegrationEvidence`：九字段如实契约——`sessionOpened`/`taskBound`/`turn`（`transcribed|charging-unconfigured|failed`，未配置语音计价的 503 是诚实结论不是失败）/`confirmSteer`/`disconnect`/`resume`/`rawAudioDiscarded`/`voiceNeverDecided`/`errorReason?`。
   - `runVoiceRoomIntegration(config)`：真实端到端（AC3 live）——生产 `createJsonTransport` + `createMobileRuntime` 授权通道 + `createMobileVoiceSessionRemote`（Task 2，3 行映射剥离 grant token）/`createMobileVoiceTranscriptionRemote`（#56）+ Task 1 `createVoiceRoom` + 真实 `createTaskOffice`。scripted 捕获供给确定性 1 秒静音 WAV（`silentWavBytes` 纯合成，与 #56 同源手法），真实 multipart 上载。`WEKNORA_MOBILE_TEST_START_TASK=1` 门控真实 Task 绑定（office.start → tasks 反查 taskId，编排与 task-intervention 冒烟逐字同序）；未门控时以探针 taskId 验证无 Run 的会话/转写/结束/恢复路径。诚实性要点按计划落实：confirmTranscript 只消费一次（取意图 → AC2 结构断言 `decisionShapedKeys` → 真实 `detailController.act`）；open/转写失败时以一次裸 `sessionRemote.open` 的错误码区分 `VOICE_CHARGING_UNCONFIGURED`（诚实 503）与其它失败。runner 是 total 的：任何异常落 `errorReason` 返回 evidence，绝不 reject、绝不伪造通过。
   - `emitVoiceRoomIntegrationEvidence(evidence, emit)`：只发射脱敏后的 evidence JSON（不含凭据）。
2. **`apps/mobile/src/voice-room-integration-smoke.test.ts`（新建，56 行，3 个测试）**：计划行 1907-1964 逐字——(a) opt-in 双形态（缺凭据 skip + 5 类私网/环回/非 HTTPS 主机 invalid，invalid 绝不是 pass）；(b) live 端到端（无凭据环境 `t.skip`，skip 不是 fail；有凭据时断言 sessionOpened/turn 三态合法/rawAudioDiscarded/voiceNeverDecided，transcribed 分支进一步断言 `disconnect==='ended-settled'` + `resume==='new-session'`，且证据不匹配 `/short-lived-secret|password|email/i`）；(c) runner total 性（`weknora.invalid.test` 登录失败仍产出 evidence、`sessionOpened:false` 而非伪造）。

### 二、TDD 证据

**RED（先写失败测试，实跑确认失败）**

命令：`pnpm exec tsx --test apps/mobile/src/voice-room-integration-smoke.test.ts`

关键输出（3 个测试全部因模块缺失失败，形态与计划 plan-t57.md:1969 预期逐字一致）：

```
✖ the integration runner is total: a failing transport still yields evidence, not a rejection (29.610417ms)
  Error [ERR_MODULE_NOT_FOUND]: Cannot find module '/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t57/apps/mobile/src/voice-room-integration-smoke.ts' imported from .../apps/mobile/src/voice-room-integration-smoke.test.ts
```

**GREEN（实现后实跑通过）**

命令：`pnpm exec tsx --test apps/mobile/src/voice-room-integration-smoke.test.ts`

完整输出：

```
✔ integration stays opt-in: missing credentials skip and a private/localhost host is invalid, never a pass (2937.470334ms)
﹣ live voice room end to end: open → turn → confirm steer → leave settled → resume new session (opt-in) (1.747167ms) # missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD
✔ the integration runner is total: a failing transport still yields evidence, not a rejection (1034.678166ms)
ℹ tests 3
ℹ pass 2
ℹ fail 0
ℹ skipped 1
ℹ duration_ms 6316.513791
```

**2 pass + 1 live skip**——与计划 plan-t57.md:2226 预期逐字一致（本地无 `WEKNORA_MOBILE_TEST_*` 凭据，live 用例以 `t.skip` 诚实跳过，不伪造通过；skip 原因如实打印）。

### 三、收尾验证（作为 6/6 任务的计划级回归）

新增文件位于 apps/mobile 内，追加运行计划级验证命令（plan-t57.md:2242 整条命令串）：

```
$ pnpm exec tsx --test packages/mobile-core/src/voice-room/voice-room.test.ts packages/api-client/src/mobile/voice-sessions.test.ts apps/mobile/src/voice-room-view.test.ts apps/mobile/src/voice-room-integration-smoke.test.ts && go test ./internal/handler/ -run 'TestVoice|TestTranscription|TestPriceVersion|TestVoiceRoom' -count=1 && go test ./internal/router/ -run 'TestRegisterMobileVoiceRoutes|TestMobileVoiceSurface' -count=1 && pnpm --filter @weknora/mobile typecheck
```

完整结果：

```
ℹ tests 27
ℹ pass 26
ℹ fail 0
ℹ skipped 1        ← 唯一 skip 即本任务的 live 用例（无凭据环境诚实跳过）
ok  	github.com/Tencent/WeKnora/internal/handler	3.244s
ok  	github.com/Tencent/WeKnora/internal/router	2.730s
> tsc --noEmit    ← typecheck 零错误退出
```

tsx 27 测试 = 26 pass + 1 live skip、Go voice 面 ok、typecheck 通过——与计划 plan-t57.md:2245 声明的预期（「27 测试（26 pass + 1 live skip）；Go voice 面 ok；apps/mobile typecheck 通过」）逐字吻合。Task 1-5 的交付零回归。

### 四、提交与范围

```
$ git show --stat 7b949f985
 2 files changed, 301 insertions(+)
 create mode 100644 apps/mobile/src/voice-room-integration-smoke.test.ts
 create mode 100644 apps/mobile/src/voice-room-integration-smoke.ts
```

零生产代码改动（两个文件均为新增测试/证据文件，非 apps/mobile 生产接线面）；未触碰任何共享热点文件；未推送远端；未派发子代理。

### 五、自检发现

1. **逐字交付核验**：两文件与计划 plan-t57.md:1907-1964（测试）/1975-2221（实现）逐字一致，无计划外偏差。Consumes 的全部前置接口在动手前逐一亲眼核实存在：`createVoiceRoom`（mobile-core index.ts:119，Task 1 产出）、`createMobileVoiceSessionRemote`（voice-sessions.ts:95，Task 2 产出）、`disallowedDeploymentHost`（runtime-integration-smoke.ts:118）、`createNativeRequestId`（adapters/request-id.ts:7）、`createTaskDetailController` 及其 `act`/`whenSettled`（task-detail-view.ts:32-33,66-67）、api-client exports `./mobile/task-office` 与 `./transport`、`CLIENT_PROTOCOL_VERSION`（packages/domain/src/mobile）。
2. **凭据卫生**：源码与测试无真实凭据字面量——测试中 `['short-lived','secret'].join('-')` 为合成串、`nobody@example.test`/`wrong` 为明示的失败用例替身；evidence 契约不含 email/password 字段，且测试 (b) 以 `/short-lived-secret|password|email/i` 负向断言钉住。
3. **live 用例未真实执行属预期**：本环境无 `WEKNORA_MOBILE_TEST_*` 部署凭据，AC3 的 live 端到端以 skip 呈现（计划「验收标准 3 的本地可验证性说明」plan-t57.md:68 预先声明的 blocked-env 形态）；测试 (c) 的 `weknora.invalid.test` 是计划自身的 total 性用例（真实 DNS 失败路径，~0.4-1s），非集成证据冒充。
4. 无其他发现：本任务无生产实现面，无证据 pinning 例外（标准 RED→GREEN 全程实跑）。
