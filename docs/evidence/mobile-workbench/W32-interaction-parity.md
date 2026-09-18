# W32 — 高级交互能力端口与保留清单闭合（交互保留对照证据）

- 日期：2026-09-17
- 工作区：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`（分支 `codex/react-vue-parity-align`）
- BASE：`ce949347`（`docs(sdd): accept W31 with blockers after fix round 1`）
- 结论层级：**全部为 harness 层证据；真机 iOS/Android 均 blocked-env**（本环境无设备、无签名身份、无 live 后端/CLI）。本文任何条目都不构成「完整 Happy 交互保留」的声明——未完成项在矩阵中保持 `open`。

## 1. 交付物

| 文件 | 说明 |
| --- | --- |
| `apps/mobile/sources/weknora/conversations/advanced.ts` | `AdvancedOperation`（goal/fork/side_chat/archive/rewind/duplicate/terminal）、`invokeAdvanced(op, capabilities, call)`（能力未证实 → `CAPABILITY_UNAVAILABLE`，call 只接固定 operation）、`deriveRemoteAdvancedCapabilities` / `deriveProductAdvancedCapabilities`（能力来自事实，不默认授权）、`REMOTE_ADVANCED_DRIVER`（driver 映射表：archive 永不映射 cancel；rewind/duplicate `claimsExternalRollback=false`）、`createSandboxTerminalOperation`（复用已验证产品端点 `POST /api/v1/sessions/:id/sandbox/terminal-ticket`，token 一次性短期、只产 scoped WS 引用、无通用 shell RPC 透传） |
| `apps/mobile/sources/weknora/conversations/advanced.test.ts` | node:test 10 例（brief 原文用例 + 七操作端口语义） |
| `apps/mobile/sources/-session/SessionView.tsx` | goal/side_chat/archive 三个远程高级处理器接线 `invokeAdvanced`（能力由会话事实派生；产品会话无远程事实时结构上不可达，而非静默回退） |
| `apps/mobile/sources/weknora/conversations/ConversationScreen.tsx` | 产品侧 advanced seam（`advanced?: ConversationAdvanced`）：终端入口经 `invokeAdvanced('terminal', deriveProductAdvancedCapabilities(...))`，无 seam 即无入口；拒绝/失败以类型化提示呈现，不伪造成功 |
| `apps/mobile/sources/app/(app)/session/[id].tsx` | 产品路由装配 terminal 操作（host+bearer 存在才有 seam） |
| `docs/migrations/happy/interaction-matrix.json` | 78 行全部新增 `productEndpoint`/`sourceCommit`/`implementationTask`/`nativeEvidence`/`status`（id 全保留）+ `statusLegend` + W32 附录 |
| `docs/migrations/happy/source-manifest.json` | 695 个 `target` 由历史 `happy-mobile/…` 校正为实际 `apps/mobile/…`；`source`（`packages/happy-app/…`）与 `sha256` 一律未动（来源 hash 含义不变） |

**范围披露（先例：W25/W29/W31 的 seam 装配模式）**：brief 的 Files 清单只列 SessionView.tsx；为满足 Step 4「终端复用已有 sandbox terminal-ticket 入口」，产品端点接线落在 `ConversationScreen.tsx` + `[id].tsx`（+两处测试文件的配套 mock/用例，见 §6）。此为「产品端点接线按先例」的扩大范围，已在任务报告中向协调者披露。

## 2. RED / GREEN（任务行为，非环境故障）

RED（实现前，brief 预期 invokeAdvanced 未定义）：

```text
$ pnpm exec tsx --test apps/mobile/sources/weknora/conversations/advanced.test.ts
Error: Cannot find module './advanced.ts'   (advanced.test.ts)
ℹ pass 0 / fail 1                            （/tmp/w32-red.txt）
```

GREEN（最小实现 + 语义用例）：

```text
$ pnpm exec tsx --test apps/mobile/sources/weknora/conversations/advanced.test.ts
ℹ tests 10  pass 10  fail 0  skipped 0
```

用例覆盖：① brief 原文（fork 无能力 → 拒绝且 call 0 次）②能力缺失（非 false）同样拒绝 ③支持的操作只送达固定 operation ④能力未证实时 handler 不启动 ⑤远程能力=facts 映射；archive driver ≠ cancel（`cancelCommand:false`）⑥fork source 缺失 → fork/side_chat/duplicate 拒绝而 archive 仍可用 ⑦产品会话只暴露 terminal（六项 Happy 操作 undefined → 必拒）⑧rewind/duplicate driver 非破坏性（`claimsExternalRollback:false`）⑨terminal 票据：恰一次 `POST …/sandbox/terminal-ticket` + Bearer 仅在该调用；产出 `wss://…?ticket=ticket-once`（30s），url 不含 bearer ⑩terminal 操作绑错 operation → `OPERATION_MISMATCH` 且不发票据。

## 3. 七操作 harness 层前后数据与 API 记录

通用前置：能力未证实时（before）所有操作 0 次 API 调用并抛 `CAPABILITY_UNAVAILABLE`——这是不可回退 shell 的结构保证（advanced.test 用例 ①②④）。

| 操作 | driver（REMOTE_ADVANCED_DRIVER） | before → after（harness 记录） | 产品层 |
| --- | --- | --- | --- |
| goal | `sync.sessionGoalAction` | before：无 dispatch；after：`sessionGoalAction(sessionId, action, objective)` 经 `invokeAdvanced('goal', …)` 派发（SessionView 接线）；行为 harness：`agentGoalActionHandler.spec.ts` 6/6（clear/edit 各带/不带 objective、in-flight 状态、错误路径） | 无产品端点 → 必拒（⑦） |
| fork | `sync.forkAndSpawn` | before：无能力 0 调用；after：`machineRPC('machine-1','codex-fork-thread',{directory,codexThreadId})` → `spawn-happy-session{agent:'codex',resumeCodexThreadId:'thread-forked',parentSessionId:'happy-source'}` → 新会话 id `happy-forked`（`ops.codexFork.test.ts`；复制获准历史/引用，不复制未决审批/预算/租约——客户端从不携带这些状态） | 无产品端点 → 必拒 |
| side_chat | `sync.spawnSideChat` | before：无 fork source → `CAPABILITY_UNAVAILABLE`（SessionView 原有 HappyError 守卫保留）；after：以 fork source 调 `spawnSideChat`（结果经闭包捕获，gate 签名只接固定 operation）。driver 级仓库自动化证据缺失 → 矩阵保持 open | 无产品端点 → 必拒 |
| archive | `sync.sessionArchive`（`cancelCommand:false`，≠ 取消） | before：rig 会话调用返回 `{success:false,message 含 'machine'}`（拒绝路径，`ops.rig.test.ts`）；after（正常路径）：kill 失败后经 `invokeAdvanced('archive',…)` 调 `sessionArchive(id)`。SessionView 中 kill（远程关闭）与 archive（生命周期归档）保持两条独立命令 | 无产品端点 → 必拒 |
| rewind | `sync.listRewindPoints+forkAndSpawn` | 非破坏性构造：列出 rewind 点后从选中点 fork **新会话**；`claimsExternalRollback:false`——已发生的外部副作用不在回滚声明内（⑧）。rewind/duplicate 无独立矩阵行，流在移植的 `DuplicateSheet` | 无产品端点 → 必拒 |
| duplicate | `sync.listRewindPoints+forkAndSpawn` | after（`ops.codexFork.test.ts`）：`codex-duplicate-thread{directory,codexThreadId,cutAfterItemId:'user-item-2'}` → `spawn-happy-session{resumeCodexThreadId:'thread-cut',forkedFromMessageId:'message-2'}` → 新会话 id `happy-cut`（原历史不变） | 无产品端点 → 必拒 |
| terminal | `product.sandboxTerminalTicket` | before：无票据；API：恰一次 `POST https://api.example/api/v1/sessions/sess%2F1/sandbox/terminal-ticket`，`authorization: Bearer bearer-long-lived`（仅此调用携带）；after：`{ticket:'ticket-once',expires_in:30}` → 目标引用 `wss://api.example/api/v1/sessions/sess%2F1/sandbox/terminal?ticket=ticket-once`（url 无 bearer；⑨）。移动端 WS 桥/受控 WebView（H33）未交付 → 入口可达、连接桥 open | **已验证产品端点**（api-client `createSandboxTerminalApi`；handler `IssueSandboxTerminalTicket`，JWT 短期一次性、`user.go` 拒绝其充当通用 token） |

远程 terminal 与产品 sandbox terminal 的 provider 区分（H33 要求）：`deriveRemoteAdvancedCapabilities` 永不产出 `terminal` 能力（⑤），远程会话不会解锁产品票据。

## 4. 矩阵与清单更新

- `interaction-matrix.json`：78/78 行新增五字段；id 与原字段全保留。状态分布：**12 harness / 66 open / 0 native**（native 全部 blocked-env，`statusLegend` 显式声明 harness ≠ 原生验收、`productEndpoint:null` 表示走已验证远程 driver 且无产品端点）。不支持项（产品层六项高级操作、模型/effort、WS 桥等）全部保持 `open`——没有用「不可用提示」完成任何一格。
- `source-manifest.json`：`target` 前缀 `happy-mobile/` → `apps/mobile/`（695 处）；`source`、`sha256`、`commit`、`dependencies/patches/exclusions` 未改。

## 5. 原 H27–H33 对照（逐项原生验收状态）

依据：`docs/superpowers/plans/2026-09-10-happy-mobile-07-delivery.md`（H27–H33 定义）与 `apps/mobile/sources` 实际文件盘点。

| 原任务 | 要求 | 实际状态 | 结论 |
| --- | --- | --- | --- |
| H27 设置/主题/语言/无障碍 | `weknora/management/preferences.ts`+`SettingsScreen.tsx`+无障碍矩阵 | Happy 设置屏已移植（`app/(app)/settings/*`，主题/语言/外观/语音/用量入口）；`weknora/management/` 目录不存在，无产品偏好解析，无 accessibility-matrix.md | **未完成（open）** |
| H28 Agent/模型绑定/MCP/Skill | H28.1–H28.4 四个编辑屏 | `settings/agents.tsx` 为 Happy 保留屏；四个产品编辑屏（AgentEditor 等）未实现 | **未完成（open）** |
| H29 知识内容编辑 | H29.1–H29.6（KB 增改删/文件夹/FAQ/Wiki/图谱） | 知识浏览链已存在（`app/(app)/knowledge/*` + `weknora/knowledge/{api,model,file}.ts`，api 走产品 client 只读为主）；编辑/版本/revert/图谱子屏未实现 | **部分（浏览有、编辑 open）** |
| H30 数据源 | H30.1–H30.4 | 无 DataSource* 屏 | **未完成（open）** |
| H31 组织空间 | H31.1–H31.4 | `weknora/auth/InvitationScreen.tsx`（邀请加入）存在；切空间/成员/组织屏未实现 | **部分（邀请有，其余 open）** |
| H32 系统配置 | H32.1–H32.8 | 无 ModelSettings/StorageParser/McpSkill/Memory/ApiKey/QueueAudit/Integration/SystemAdmin 移动屏 | **未完成（open）** |
| H33 受控终端 | api-client terminal + `terminal-bridge.ts` + `TerminalScreen.tsx` | api-client 侧已存在（`sandbox/terminal.ts`，W32 复用其票据端点并接线到会话屏）；`terminal-bridge.ts`/`TerminalScreen.tsx` 未实现，移动端无 WS 桥 | **入口已闭合（harness），桥 open** |

**发布结论**：仅允许以「核心能力发布」口径对外（执行链/审批/附件/语音/通知/恢复 + 本次 advanced 端口与终端票据入口）；H27–H32 各子项与 H33 的桥**不声称完整保留**，全部以 open 留在矩阵/账本。

逐项 iOS/Android 证据分层（真机均 blocked-env；harness 指针如下）：

| 项 | harness 层证据 | iOS/Android 真机 |
| --- | --- | --- |
| 审批 | `ConversationScreen.test.tsx`（批准/拒绝带 revision；口头永不批准） | blocked-env |
| 通知 | `live-progress.test.ts`、`native-progress-port.test.ts`（expo-notifications 端口；Live Activity 声明性） | blocked-env |
| 语音 | `dictation.test.ts` 20/20、`realtime.test.ts` 21/21（含 R-1）、`VoicePanel.test.tsx` | blocked-env |
| 主题 | `theme.ts`/light/dark json 移植；无自动化验收 | blocked-env |
| 大字体 | 无证据 | blocked-env（open） |
| 平板/横屏 | `SessionView` 响应式 hooks（useDeviceType/useIsTablet/useIsLandscape）移植；无自动化验收 | blocked-env |
| 分享 | W25 `acceptSharedFile` seam + text-selection 屏移植 | blocked-env |
| 文件/Diff | `changes/files/file` 屏移植；产品侧 `preview-policy.test.ts`+`ArtifactPreview.test.tsx` | blocked-env |

## 6. 回归（全部通过）

| 命令 | 结果 |
| --- | --- |
| `pnpm exec tsx --test`（advanced+realtime+dictation+live-progress+recovery+view-model+view-model-protocol+protocol-gate+registry+execution-projection+resources） | **110/110 pass, 0 fail, 0 skip** |
| `pnpm --filter @weknora/mobile exec vitest run`（ConversationScreen/VoicePanel/ProductConversationMessages/KnowledgeCitation/DataAnalysisResult/[id]/new） | **45/45 pass**（ConversationScreen 20 = 既有 18 + W32 新增 2；[id] 7 含既有 W25/W12/W29/W31 挂载用例） |
| `pnpm --filter @weknora/mobile exec vitest run sources/sync/ops.codexFork.test.ts sources/sync/ops.rig.test.ts sources/sync/ops.rigSpawn.test.ts sources/-session/agentGoalActionHandler.spec.ts` | **16/16 pass** |

W25/W28/W29/W31 既有挂载回归未破坏；`[id].test.tsx` 按该文件既有模式补一行 `vi.mock('@/weknora/conversations/advanced', …)`（与 W31 为 protocol-gate/realtime 补 mock 同模式——该 vitest 环境不解析 `@/` 别名）。

## 7. W31 R-1 顺手修复（TDD + 披露）

- RED：`realtime.test.ts` 新增「dispose racing the admit round-trip still settles the granted session (fix R-1)」——实现前 **fail 1**（`expected the raced admission to be released, got []`）。
- 修复：`realtime.ts` `begin`/`renewIfExpired` 在 `admitFresh()` 成功返回后遇 `disposed`，先 `input.release(admitted.id).catch(()=>undefined)` 再返回（dispose 的 `dropCurrent` 此刻 `sessionID===null` 无法结算该行）。
- GREEN：`realtime.test.ts` **21/21 pass**（恰一次 release、无双重结算）。

## 8. W30 语义提醒处置

「unknown 零用量会话 stop 保留 hold 待对账」的回收器：**本任务未实现**（brief 标注可选）。现状维持 W30 验收时声明：unknown+零用量 settle 保留 hold 等待对账。若后续实现需独立 TDD 任务（W33 清理/对账域）。

## 9. 未覆盖条件与环境阻塞

- 真机 iOS/Android（VoiceOver/TalkBack、大字体、暗色、横屏、平板、安全区、键盘、长焦列表）与原生构建/签名：blocked-env。
- 移动端终端 WS 桥/受控 WebView（H33 `terminal-bridge.ts`/`TerminalScreen.tsx`）：未实现（入口票据已闭合）。
- Happy 远程 CLI 实链（fork/rewind 真实 daemon RPC、goal 命令真机往返）：blocked-env；仓库内仅 driver 级 mock 测试（`ops.codexFork.test.ts` 等）。
- 模型/effort 产品面：产品会话不提供选项，也不继承任何未授权 Happy 默认模型（产品控制面板无模型选择器；移植 UI 的模型/effort 由 `isRigModelSelectionEnabled`/`isRigReasoningSelectionEnabled` 能力门控）。
