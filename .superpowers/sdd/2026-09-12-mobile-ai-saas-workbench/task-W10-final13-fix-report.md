# W10 final13 fix report — mounted `/new` route evidence + seam wiring

- 任务：W10「Happy 会话视图模型和产品导航」final13 审查开放发现修复（round 14）
- 工作目录：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`（分支 `codex/react-vue-parity-align`）
- 基线 BASE：`eb43f28b`
- 日期：2026-09-17（Asia/Shanghai）

## 结论一览

| 项 | 结果 |
|---|---|
| 发现 1（P0，缺少真实挂载证据） | **已修**：新增挂载 `NewSessionScreen` 组件集成测试（`apps/mobile/sources/app/(app)/new/index.test.tsx`，2 用例） |
| 发现 2（P0，helper 未接入生产 `/new` 成功路径） | **已修**：`/new` 成功路径改经 `completeProductSessionCreation` seam（TDD RED→GREEN） |
| 发现 3（P1，missing-metadata 只有 adapter 层覆盖） | **已修**：挂载用例驱动真实 spawn 后断言不导航 + 生产 unavailable alert |
| 既有 focused 测试 | **PASS**：3 文件 26 测试（历史报告中的 25 在后续提交已增长为 26），加新挂载 2 例合计 28/28 |
| `pnpm --filter @weknora/mobile typecheck` | **PASS**（`tsc --noEmit` exit 0） |
| Expo 原生 route mount / simulator / device / live backend | **blocked-env / NOT PROVEN**（未作为通过证据；见「环境阻塞」） |

## 变更清单

### 1. `apps/mobile/sources/app/(app)/new/index.tsx`（生产代码）

- import 从 `navigateCreatedProductSession` 改为 `completeProductSessionCreation`。
- `handleSend` 的 `case 'success'` 重构：
  - 原内联序列 `await sync.refreshSessions()` → launch-mode pin（`sessionSetAgentModes`）→ 读 draft 并清空 → 首条消息（`sync.sendMessage`）→ `storage.getState().sessions[id]` 读取 → `navigateCreatedProductSession(...)`，改为：
  - 把「refresh + mode pin + draft 清空 + 首条消息」收进本地 `persistSpawnedSession` async 函数，作为 `refreshSessions` 回调传给 `completeProductSessionCreation`；durable 读取以 `readSession: (sessionId) => storage.getState().sessions[sessionId]` 传入；`navigateProduct: navigateToProduct`、`onUnavailable`（生产 `Modal.alert` 提示语原样保留）。
  - **每个生产步骤的相对顺序与改动前完全一致**（refresh → pin → 清 draft → 首条消息 → durable read → 产品导航），仍只有一次 refresh；seam 现在生产化地强制 refresh-before-read-before-navigate 与 fail-closed 导航。
- 删除了原尾部无效果的 `if (!navigated) { break; } break;`（返回值不再需要分支）。
- `useCallback` 依赖数组不变（新增引用均为模块级 import 或块内局部量）。

### 2. `apps/mobile/sources/app/(app)/new/index.test.tsx`（新增挂载测试）

挂载真实 `NewSessionScreen`（`react-test-renderer` + `act`，React 19.3.0），通过 `accessibilityLabel="Send"` 找到生产发送按钮并触发 `onPress`，驱动真实 `handleSend`。两个用例：

1. **成功 spawn 全链**：`machineSpawnNewSession` → 成功 → 断言
   - spawn 参数（machineId/agent/directory）；
   - 生产路径确实调用了 `completeProductSessionCreation`（seam 使用断言，TDD RED 驱动项），入参含 `sessionId: 'session-1'`；
   - 顺序 `refreshSessions` → `storage.sessions-read`（durable store 读取）→ `router.push`（事件数组 indexOf 严格递增）；
   - **真实产品 router 目的地（完整元数据）**：`/session/session-1?spaceId=space-1&agentId=agent-1&targetId=target-1&workspaceRef=workspace-1&resourceUserId=user-1&resourceTenantId=tenant-1&resourceSessionId=session-1&runId=run-1` —— 由真实 `useNavigateToProductSession` → `navigateToProductSession` → `sessionHref` 链路产出（该 hook 以 `vi.importActual` 真实加载，仅 `expo-router` 的 `useRouter().push` 为 spy）；
   - 生产副作用保留：`sessionSetAgentModes('session-1', { permissionMode: 'auto', modelMode: 'default', effortLevel: 'medium' })`、`sendMessage('session-1', 'Plan the launch', { source: 'new_session', attachments: [] })`、draft `input` 清空、无错误 alert。
2. **missing-metadata fail-closed**：durable session 为 `{ id: 'session-2', metadata: {} }`，断言
   - seam 仍被调用且 `refreshSessions`、durable read 均发生；
   - `router.push` **零调用**；
   - 生产 unavailable UI 恰好一次：`Modal.alert('common.error', 'The new product session is not ready yet. Please retry after synchronization.')`。

测试 harness 说明（范围扩大说明）：

- 挂载 3100 行路由组件需要在其模块边界 stub 原生/Expo 面：`react-native`（host 组件、Platform、Keyboard、LayoutAnimation、Animated、useWindowDimensions）、`expo-router`、`expo-glass-effect`、`@expo/vector-icons`、`react-native-unistyles`、`react-native-safe-area-context`、`react-native-keyboard-controller`、`expo-constants`、`expo-crypto`。
- 产品/业务逻辑尽量走真实代码：`@/utils/productSessionEntry`（seam 本体 + spy 包装）、`@/hooks/useNavigateToSession`、`@/utils/machineUtils`、`@/utils/newSessionModeSelection`、`@/utils/newSessionSidebarLayout`、`@/utils/newSessionPickerItems`、`@/utils/newSessionPickerInteraction`、`@/utils/time`、`@/sync/spawnRequestId`（真实幂等 key 逻辑）均以 `vi.importActual` 相对路径真实加载。
- 存储边界为假件：`@/sync/storage`（`storage.getState()` 以 getter 记录每次 durable 读取）、`@/sync/sync`（refreshSessions/sendMessage spy）、`@/sync/ops`（machineSpawnNewSession/sessionSetAgentModes spy）、`@/modal`、`@/hooks/useNewSessionDraft`（可调用假 store，保留 `getState()`）。
- `@/sync/machineChoices` 为忠实手写假件（单台 Happy CLI 机器形态，字段与真实 `MachineChoice` 对齐：`machineIds`/`happyMachine`/`rigMachine`/`online`/`activeAt`），避免拉入其 `@/utils/harnessCatalog` 等别名级联。
- 路由模块顶层 `require('@/assets/images/*.png')` 在 vitest 下落入 Node require 且 `@/` 别名不可解析：测试在动态 `import('./index')` 之前对 `node:module` 的 `Module._load` 打了**仅限 `@/assets/` 前缀**的拦截（返回 `asset:<id>`），其余 require 走原 loader。此为测试文件内局部行为，不改任何全局配置（未新增 vitest.config）。
- 未修改 `productSessionEntry.ts` / `productSessionEntry.test.ts`（本轮不需要；seam 语义与既有 3 个 adapter 单测全部保持）。

## TDD 证据

### RED（先写测试，生产路径未接 seam）

命令（worktree 根执行）：

```
pnpm --filter @weknora/mobile exec vitest run "sources/app/(app)/new/index.test.tsx"
```

- 退出码：**1**（pnpm `ERR_PNPM_RECURSIVE_EXEC_FIRST_FAIL`）
- 结果：`Test Files 1 failed (1)`，`Tests 2 failed (2)`
- 失败输出（节选）：

```
× mounted NewSessionScreen production route > completes a successful spawn through refresh, durable read, and the product router destination
  → expected "spy" to be called 1 times, but got 0 times
  ❯ sources/app/(app)/new/index.test.tsx:544:40
    expect(mocks.completeCreation).toHaveBeenCalledTimes(1);

× mounted NewSessionScreen production route > fails closed without navigating and shows the production unavailable alert when durable metadata is missing
  → expected "spy" to be called with arguments: [ ObjectContaining{…} ]
  Number of calls: 0
```

即：挂载组件本身可渲染、可驱动（spawn/refresh/router.push 等前置断言先通过），但生产 `/new` 成功路径没有调用 `completeProductSessionCreation` —— 精确对应 final13 发现 2。

### GREEN（接线后）

同命令：

```
pnpm --filter @weknora/mobile exec vitest run "sources/app/(app)/new/index.test.tsx"
```

- 退出码：**0**
- 结果：`Test Files 1 passed (1)`，`Tests 2 passed (2)`（19ms）

### 既有 focused 套件（不回归证明）

命令（worktree 根执行）：

```
pnpm --filter @weknora/mobile exec vitest run \
  sources/hooks/useNavigateToSession.test.ts \
  sources/utils/notificationRouting.test.ts \
  sources/utils/productSessionEntry.test.ts \
  "sources/app/(app)/new/index.test.tsx"
```

- 退出码：**0**
- 结果：`Test Files 4 passed (4)`，`Tests 28 passed (28)`
  - `useNavigateToSession.test.ts` 16 通过（产品/legacy 导航分离合同未回退）
  - `notificationRouting.test.ts` 7 通过
  - `productSessionEntry.test.ts` 3 通过（helper adapter 单测未改动、未破坏）
  - `index.test.tsx`（新增）2 通过

### typecheck

```
pnpm --filter @weknora/mobile typecheck
```

- 退出码：**0**（`tsc --noEmit` 无输出）

### 全量 mobile vitest（背景说明）

`pnpm --filter @weknora/mobile exec vitest run`：190 文件中 107 通过、83 失败 —— 失败全部为**既有问题**：`node:test` 风格文件（vitest 报 "No test suite found"）与未 mock 的 `@/` 别名直连 import（如 `useStartSessionFromDraft.test.ts` 顶层 `import ... from '@/sync/spawnRequestId'`，本仓库 vitest 无 alias 配置）。抽查确认失败文件均不 import 本轮改动的两个文件；本轮所涉文件（route、新测试、三个 focused 文件）全部通过。该全量失败状态与本任务变更无关（BASE 上同样失败）。

## 未覆盖条件 / 环境阻塞（blocked-env）

以下均**未执行、未证明**，不作为通过证据：

1. Expo 原生 route mount / Expo Router 真实例（本测试的 `useRouter().push` 是模块边界 spy；挂载走 react-test-renderer 而非 RN/Expo 渲染器）。
2. iOS/Android simulator 或真机运行 `/new` 屏幕并触发 spawn 的端到端证据。
3. live backend 会话 spawn 与 `productSession` metadata producer / 资源 ownership 链（W09 durable storage 真实读写）。
4. MMKV draft 持久化、真实 unistyles 主题、glass/keyboard 原生行为（模块边界 stub）。

## 提交

- 只 `git add` 本任务文件：`apps/mobile/sources/app/(app)/new/index.tsx`、`apps/mobile/sources/app/(app)/new/index.test.tsx`、本报告。未触碰并发进程的 apps/web、packages、docs 改动。
- message：`fix(mobile): wire /new success path through completeProductSessionCreation seam`
