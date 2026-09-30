# OCR 增量修复批次 4（issue30-sweep）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 OCR 第四批增量发现（B4 报告，5 项主发现 + 3 项 lowWorth）的全部 5 项主发现，按根因聚合为 5 个任务（同根因合并、同文件串行），lowWorth 三项按文件就近折叠为顺手项不单独开流：task-detail 的终态放行与 unknown 门生命周期（3 项）、UI 离线文案映射（1 项）、askKnowledge 离线门禁（1 项）。

**Architecture:** 按**根因**聚合成 5 个任务：Task 1/2 同改 `task-detail.ts`（终态观察的 parked queue-next 放行 → unknown 门的核对口径与自动核对，串行执行）；Task 3 双 UI 消费方补 `'offline'` 文案映射；Task 4 为 askKnowledge 补 Offline Gate 纵深防御（office 级门 + 端口级 guard + 组合根接线）；Task 5 死码清理。所有修复维持既有架构约束：mobile-core 不依赖 RN/DOM/具体传输、诚实回执（绝不编造服务端准入）、unknown 门 AC2「核对前阻止一切写意图」不放松、Offline Gate fail-closed 判定语义不变、parked queue-next 本地非耐久（跨进程耐久属 #40，ADR 未决）不引入持久化。

**Tech Stack:** TypeScript（`packages/mobile-core`、`apps/mobile`、`apps/miniprogram`，node:test + tsx）。测试命令在 worktree 根（`.worktrees/issue30-sweep`）执行。本计划作者已实跑下列基线（2026-09-25，当前 HEAD b06344765）：
- `pnpm exec tsx --test packages/mobile-core/src/task-office/task-detail.test.ts packages/mobile-core/src/task-office/knowledge-qa-office.test.ts packages/mobile-core/src/offline/guarded-ports.test.ts apps/mobile/src/task-detail-view.test.ts` → **56 pass / 0 fail**。
- `pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx` → **65 pass / 0 fail**。
- `pnpm --filter @weknora/mobile typecheck` → **exit 0（绿）**。
- `pnpm --filter @weknora/miniprogram typecheck` → **FAIL：71 个预存在错误**（详见差异记录 1；本批次验收口径为「office-views.ts 错误集合不扩大」）。
- 行为复现脚本（临时、跑后已删）：R1-F1——流内 `run.completed` 先到 + 服务端正常结束流 → `connection='drained'` 而 queue_next 派发 **0 次**（承诺违背）；R1-F2——流内 `run.canceled` 先到并持久化、快照滞后仍 running → resync 后停止卡消失（应为 confirmed）。两缺陷均在本 worktree 实跑复现。

**Spec:**
- 发现来源：B4 增量发现清单（本计划的 ask 材料；编号 R1-F*，全部经本计划作者按当前 HEAD 读码复核 + 关键项实跑复现，行号以复核为准；差异见「差异记录」）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（T07 受控干预与诚实回执 / T10 Offline Gate「prohibits Run commands」/ 离线降级渲染）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§5 Task 详情三层状态与 Timeline / §5.2 act(TaskIntent) / §5.3 迟到拒绝）
- ADR：`docs/adr/0012-mobile-business-logic-lives-behind-deep-modules.md`（业务逻辑在深模块内——Task 1/2/4 的修复全部落在 mobile-core，UI 只补文案）
- 领域术语：`CONTEXT.md`
- 前序批次：`docs/plans/issue30-sweep/plans/ocr-fix-increment-b3.md`（沿用其纪律与验收口径）
- Parent：Issue #30（issue30-sweep）

## Global Constraints

以下为批准 Spec / ADR / 报告隐含的项目级约束，所有任务隐含遵守：

- **「Mobile core does not depend on React Native, DOM or concrete transport.」**（mobile-ai-office-design.md）——Task 1/2/4 对 mobile-core 的修改不得引入 RN/平台依赖。
- **「Tests target observable behavior at the highest stable Interface.」**——回归测试落在 TaskHandle/TaskOffice 公共接口行为（view/act/flushQueuedIntents/askKnowledge 的可观察输出），不测内部实现细节。
- **诚实回执（绝不编造服务端准入）**：parked/conflict/unknown 回执语义不动（task-intent.ts:21-34）；Task 1 只是把「观察到终态」与「放行」的既有承诺补全到自然完成路径，不改变任何回执形态。
- **unknown 门 AC2 不放松**：核对完成前 `act()` 一切写意图仍抛 `TASK_OFFICE_COMMAND_UNKNOWN`（task-detail.ts:502）。Task 2 只缩短核对窗口，绝不解除门禁语义。
- **Offline Gate fail-closed 判定语义不变**（offline-gate.ts:32-49）：探测失败=离线；平台无状态通道透传放行。Task 4 不改 gate 本身，只补 askKnowledge 的两道接线；`OfflineGateError` 必须在 `callBackend` 之外直接上抛，不得被包装成 `TASK_OFFICE_BACKEND`（start() :540-544 注释的既有约定）。
- **parked queue-next 本地非耐久是既有文档语义**（task-intent.ts:6-8「本地持有非耐久（跨进程耐久属 #40，且持久化选型 ADR 未决）」）——Task 1 不改 close() 的语义；run 仍活动时 close 丢弃 parked 项维持文档行为（差异记录 3）。
- miniprogram typecheck 基线已红（71 预存在错误，`@weknora/mobile-core` 未链接进 `apps/miniprogram/node_modules`）——Task 3 的 miniprogram 侧验收口径为「office-views.ts 错误集合不扩大」，不以门禁转绿为准；workspace 链接缺失属范围外（差异记录 1），不在本批次顺手修改。
- 严格 RED→GREEN→REFACTOR：每个任务先写失败测试、实跑确认失败、最小实现、通过、提交；实现与已批准 Spec 冲突时升级处理。
- 发现驱动的最小修复：lowWorth 项（R1-F6/F8/F9）仅按文件就近折叠为所在任务的顺手项，随该任务实现与验收，不单独开流。

## Review Focus

Spec/报告隐含但任务测试需钉住、最可能咬到真实用户的五类失效模式（每行后标注 owning 任务）：

1. **Run 自然完成时用户已排队（parked）的 queue-next**——回执与 UI 文案承诺「已排队（等待当前 Run 结束后发出）」（TaskDetailScreen.tsx:39），自然完成主路径必须兑现承诺自动发出；派发失败以 conflict/unknown 回执如实呈现，绝不静默丢弃。——Task 1 测试。
2. **停止结果未知后，取消事件先于快照刷新到达（快照仍 running）**——核对必须按合并口径判 confirmed；绝不因快照滞后把已落地取消误判「未落地」而诱导用户重试（重试必得 conflict，徒增困惑）。——Task 2 测试（`in-stream cancel lands confirmed`）。
3. **停止结果未知后用户无任何外部操作**（流存活且 Run 自然完成 / 连接中断）——门必须自行安排核对解除，不得滞留到「仅剩显式 resync 一条恢复路径」；滞留期间 UI 停止卡「正在与服务端核对」承诺与事实落差扩大。——Task 2 测试 ×2（自动核对 confirmed / not-landed 解除后重试 accepted）。
4. **离线时快速提问（askKnowledge）**——必须以 `OFFLINE_ACTION_BLOCKED:run` 结构化拒绝且零后端派发（createSession 与 ask 均不得触达），不得以 transport 错误伪装成 `TASK_OFFICE_BACKEND`。——Task 4 测试 ×2（office 级门）+ guarded-ports 测试 ×1（端口级纵深）。
5. **离线降级视图的中断原因呈现**——`'offline'` 必须经文案映射为用户可读文案，不得直出内部码（app-smoke 红线 :838-839）；两个 UI 消费方（mobile 屏 + miniprogram 页）都必须补齐映射。——Task 3 测试。

---

## 任务结构与文件地图

| # | 任务 | 根因分组（发现编号） | 主要文件 | 优先级 |
|---|---|---|---|---|
| 1 | task-detail：自然完成终态放行 parked queue-next | R1-F1；顺手 R1-F6 | `packages/mobile-core/src/task-office/task-detail.ts`、`task-detail.test.ts` | **high** |
| 2 | task-detail：unknown 门核对口径与自动核对 | R1-F2 + R1-F3（同根因：unknown 门生命周期） | `packages/mobile-core/src/task-office/task-detail.ts`、`task-detail.test.ts` | **high** |
| 3 | 双 UI：'offline' 中断原因文案映射 | R1-F4 | `apps/mobile/src/screens/TaskDetailScreen.tsx`、`apps/miniprogram/src/services/office-views.ts`、`apps/mobile/src/app-smoke.test.tsx` | medium |
| 4 | task-office/guard/composition：askKnowledge 离线门禁 | R1-F7；顺手 R1-F8 | `packages/mobile-core/src/task-office/task-office.ts`、`packages/mobile-core/src/offline/guarded-ports.ts`、`packages/mobile-core/src/index.ts`、`apps/mobile/src/composition.ts`、两个测试文件 | medium |
| 5 | 契约清理：TASK_OFFICE_COMMAND_CONFLICT 死码 | R1-F9 | `packages/mobile-core/src/task-office/task-office-errors.ts`、`apps/mobile/src/task-detail-view.ts`、`task-detail-view.test.ts` | low |

**发现覆盖对照**（ask 材料逐项对账）：

| 发现 | 任务 | 处置 |
|---|---|---|
| R1-F1（high，bug） | Task 1 | 修复 |
| R1-F2（medium，bug） | Task 2 | 修复（与 F3 同根因合并） |
| R1-F3（medium，bug） | Task 2 | 修复（与 F2 同根因合并） |
| R1-F4（medium，bug） | Task 3 | 修复 |
| R1-F7（medium，bug） | Task 4 | 修复 |
| R1-F6（lowWorth） | Task 1 顺手项 | snapshotOf 改解构（新增可选字段不再被静默排除出离线快照） |
| R1-F8（lowWorth） | Task 4 顺手项 | 8000/60 魔法数提取为模块常量（与 start()/followUp() 共用一份） |
| R1-F9（lowWorth） | Task 5 | 死码移除（union 成员 + 文案条目 + 断言翻转防回潮） |

**执行顺序**：Task 1 → Task 2（同文件 `task-detail.ts` 串行，行号以各任务 Step 前实读为准）；Task 3、Task 4、Task 5 文件集互不重叠（Task 5 动 `task-office-errors.ts`，Task 1/2 所在的 `task-detail.ts` 只 import 该 union 且不引用被移除成员——grep 已证全库仅 3 处引用，无运行时冲突），可与 Task 1/2 并行。线性执行按编号顺序即可。

## 差异记录（报告 vs 代码现状，以代码现状为准）

1. **R1-F4 的「miniprogram 穷举 Record 缺键构成编译错误」在本 worktree 不可复现**：`apps/miniprogram/node_modules/@weknora/` 下仅链接 `api-client`/`contracts`/`domain`，**`mobile-core` 未链接**（`package.json:27` 声明了 `workspace:*`）；`pnpm --filter @weknora/miniprogram typecheck` 基线已红 **71 错**（`Cannot find module '@weknora/mobile-core'` 及 session.ts/account/pages.tsx 等历史错误），缺键错误被模块解析失败**掩盖**（`office-views.ts` 自身基线 6 条：4×TS2307 + :20 TS7053 + :41 TS2366，作者实跑）。修复照做——补键是源级契约，链接修复后穷举 Record 即可编译；Task 3 验收改为「office-views.ts 错误集合不扩大」。workspace 链接缺失建议升级为独立决策，不在本批次顺手修改。
2. **R1-F7 引用的 `guarded-ports.ts` 实际路径为 `packages/mobile-core/src/offline/guarded-ports.ts`**（发现写的是 `task-office/` 目录下）；其 :8-16（`guardTaskBackend` 只覆写 `start`）与 :32（legacy `followUp` 按 `'run'` 语义拒绝、注释「knowledge-chat 追问触发服务端执行」）行号经核实一致。本计划按实际路径书写。
3. **R1-F1 提到的「close():560-566 只 flushPersisted，queuedNext 随句柄丢失」**：本地 parked 非耐久是 `task-intent.ts:6-8` 的既有文档语义（跨进程耐久属 #40 且持久化选型 ADR 未决），本批次不改 close()。Task 1 修复后，「自然完成 → 用户随后 close」的正常时序下 parked 项已被放行，不再丢项；run 仍活动时 close 丢弃 parked 项维持文档语义。
4. **R1-F9 的「全库无抛出点」经 grep 复核成立**：`TASK_OFFICE_COMMAND_CONFLICT` 仅 3 处——`task-office-errors.ts:25`（union 成员）、`apps/mobile/src/task-detail-view.ts:19`（文案映射）、`task-detail-view.test.ts:99`（映射断言）。`task-detail.ts:155` `wrapCommand` 透传的 `TASK_COMMAND_CONFLICT` 是 api-client 层跨包契约码（`#38` 先例），与被移除的 `TASK_OFFICE_COMMAND_*` 是两个不同的码，不受影响。
5. **R1-F8 的 8000 上限在 domain 层无重复**（`packages/domain/src/mobile/` grep 无命中）；重复仅限 `task-office.ts` 内部——followUp:645 与 askKnowledge:659（`question.length > 8000`）、start:545 与 askKnowledge:664（`slice(0, 60)`）。常量提取收敛在本文件四处。既有边界钉子：`knowledge-qa-office.test.ts:73`（`'x'.repeat(8001)` 拒绝）与 :34（标题截 60 断言）保持全绿即证明行为不变。
6. **R1-F1 复现细节**：终态事件 seq 必须为 `committedCursor + 1`（`interventionDetail` 夹具水位 0 → 事件用 seq 1），否则走 gap 分支而非自然完成路径；计划内测试代码已按此书写。
7. **R1-F3 的滞留场景 (1)（置门后离线降级 :309-323 提前 return 不核对）**：Task 2 的修复（置门后 `void hydrate()`）对流存活场景（场景 2）完全生效；离线场景下核对本身不可达（物理离线无法向服务端求证），恢复仍走显式 `resync()`（离线模式的既有文档语义，hydrate 降级分支 :321 注释「联网后显式 resync() 恢复」）。这是物理约束而非门设计缺陷，报告的修复方向（「置门后安排核对」）即按可达场景落地。

---

### Task 1: task-detail：自然完成终态放行 parked queue-next（R1-F1；顺手 R1-F6）

**Files:**
- Modify: `packages/mobile-core/src/task-office/task-detail.ts:452-461`（streamEnded 终态分支补 flushQueuedIntents）
- Modify: `packages/mobile-core/src/task-office/task-detail.ts:241-250`（顺手 R1-F6：snapshotOf 改解构）
- Test: `packages/mobile-core/src/task-office/task-detail.test.ts`

**Interfaces:**
- Consumes: `flushQueuedIntents()`（task-detail.ts:287-295，与 hydrate 终态分支 :355 共用同一 `flushInFlight` 合流去重）；`pumpQueuedNext` 的终态门（:266 `isTerminalRunStatus(currentRunStatus())`）；测试夹具 `scriptedCommandPort`/`interventionDetail`（task-detail.test.ts:704-728）、`createScriptedTaskStream`（in-memory-task-detail.ts:38-68，`emit`/`end`）、`event$`（test:24）。
- Produces: streamEnded 终态分支行为与 hydrate 终态分支（:353-356）同款——flushPersisted + interruption=undefined + notify('drained') + fire-and-forget `void flushQueuedIntents().catch(() => undefined)`。无新导出、无签名变化。

**根因与修复说明：** parked queue-next 的放行点在模块内只有一处——hydrate 终态分支（:353-356 的 `void flushQueuedIntents().catch(() => undefined)`，注释「终态观察即放行 parked queue-next」）。但「观察到终态」有三条路径：(a) hydrate/resync 拉到终态快照（已放行）；(b) **流内终态事件先到 + 服务端正常结束流 → streamEnded 终态分支（:452-459）**——只做 flushPersisted + interruption=undefined + notify('drained')，**不放行**；(c) 显式 flushQueuedIntents。自然完成（不刷新页面、Run 自己跑完）恰是主路径 (b)：`act()` 在 :509-514 给用户 `outcome:'parked'` 回执、UI 文案承诺「已排队（等待当前 Run 结束后发出）」（TaskDetailScreen.tsx:39）、模块注释 task-detail.ts:98 承诺「观察到终态后发出」——自然完成时三项承诺全部落空，作者已实跑复现（drained 后派发 0 次）。apps 层无兜底：真实控制器 `apps/mobile/src/task-detail-view.ts` 无任何 `flushQueuedIntents` 调用（grep 仅 task-detail-view.test.ts:33/:65 的 mock stub）。修复：streamEnded 终态分支补与 :355 逐字同款的 fire-and-forget 放行。顺手 R1-F6：`snapshotOf`（:241-250）手工逐字段拷贝 `OfflineTaskSnapshot`，未来给 `TaskBackendDetail` 新增可选字段时会被静默排除出离线快照（离线降级卡片缺字段）——改解构（`task-detail.test.ts:898-901` 的既有同款惯例），新增字段自动随行。

- [ ] **Step 1: 写失败测试（追加到 task-detail.test.ts 的 T07 干预合同区，`flushQueuedIntents after the scope died` 用例之后）**

```ts
test('a naturally ended stream releases parked queue-next without an explicit flush (R1-F1)', async () => {
  const commands = scriptedCommandPort({ result: { runId: 'run-1', action: 'queue_next', nextRunId: 'run-2' } });
  const { lease } = leased();
  const scripted = createScriptedTaskStream();
  const handle = createTaskDetail(
    { taskId: 's1', runId: 'run-1' },
    {
      backend: {
        detail: async () => interventionDetail({ runStatus: 'running', revision: 7 }),
        stream: (input: { runId: string; cursor: number; signal: AbortSignal; onEvent(event: TaskBackendEvent): void; onControl(frame: TaskStreamControlFrame): void }) =>
          new Promise<void>((resolve) => {
            scripted.attach({ signal: input.signal, onEvent: input.onEvent, onControl: input.onControl, resolve, reject: () => undefined });
          }),
      },
      store: createInMemoryTaskProjectionStore(),
      lease: () => lease,
      commands: commands.port,
    },
  );
  await handle.hydrate();
  const parked = await handle.act({ kind: 'queue-next', text: 'next instruction' });
  assert.equal(parked.outcome, 'parked');
  assert.equal(commands.calls.length, 0);
  // 自然完成主路径：流内终态事件先到（seq 必须 = committedCursor + 1，见差异记录 6），
  // 随后服务端正常结束流——期间无 hydrate/resync 参与，放行只能来自 streamEnded 终态分支。
  scripted.emit(event$(1, 'run.completed'));
  scripted.end();
  await settle();
  assert.equal(handle.view()?.connection, 'drained', 'streamEnded 终态分支已执行（前置：确认走的是自然完成路径）');
  assert.equal(handle.view()?.runStatus, 'succeeded');
  assert.equal(commands.calls.length, 1, '自然完成必须兑现「已排队（等待当前 Run 结束后发出）」承诺——parked 项自动放行');
  assert.equal(handle.view()?.interventions?.at(-1)?.outcome, 'accepted');
  assert.equal(handle.view()?.queuedNext?.length ?? 0, 0);
  handle.close('done');
});
```

注：`createScriptedTaskStream`/`TaskBackendEvent`/`TaskStreamControlFrame` 已在本测试文件顶部 import（:6/:23）；`lease` 直接取 `leased()` 返回值（本用例无需 revocable）。

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/task-detail.test.ts`
Expected: FAIL——`connection` 为 `drained`、`runStatus` 为 `succeeded`（前置两断言过），但 `commands.calls.length` 为 `0`：`自然完成必须兑现「已排队（等待当前 Run 结束后发出）」承诺——parked 项自动放行` 断言失败（作者基线复现脚本已证派发 0 次）。

- [ ] **Step 3: 最小实现**

```ts
// task-detail.ts streamEnded（:452-461）——终态分支补放行（与 hydrate :355 逐字同款）：
const streamEnded = async (): Promise<void> => {
  if (!streamGuard() || detail === undefined) return;
  if (isTerminalRunStatus(terminalRunStatusOf(detail.execution.runStatus, events))) {
    await flushPersisted(); // 终态必已落盘（幂等：processEvent 已 flush 时跳过）
    interruption = undefined;
    notify('drained');
    void flushQueuedIntents().catch(() => undefined); // R1-F1：自然完成与 hydrate 终态同款放行（一次性，失败不重试）
    return;
  }
  await interrupt('stream-ended-nonterminal', 'the event stream ended before a terminal status');
};
```

顺手项（同文件，随本任务实现与验收）：
- R1-F6：`snapshotOf`（:241-250）改为解构——

```ts
// 新增可选字段自动随行进离线快照（R1-F6）：手工逐字段拷贝会把未来字段静默排除。
// 与 task-detail.test.ts:898-901 的既有同款解构惯例一致。
const snapshotOf = (source: TaskBackendDetail): OfflineTaskSnapshot => {
  const { events: _events, ...snapshot } = source;
  return snapshot;
};
```

注：纯结构重构、无可观察行为变化（`detail$` 夹具不含 `archivedAt` 键，展开结果逐键一致；持久化 JSON.stringify 对 undefined 值同样省略），不新增测试——由既有「persist carries the offline snapshot so a later offline hydrate can degrade」「offline degradation renders the persisted snapshot」两用例钉住往返不变。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/task-detail.test.ts`
Expected: PASS（既有 56 项中的本文件用例 + 新增 1 项全绿；重点回归既有「act(queue-next) on an active run parks locally and flushes on terminal observation」「a flush racing a dead scope lease is rejected」——放行时点提前不得引入重复派发，`flushInFlight` 合流保证）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/task-office/task-detail.ts packages/mobile-core/src/task-office/task-detail.test.ts
git commit -m "fix(task-detail): release parked queue-next when the stream ends naturally (R1-F1), snapshotOf via destructuring (R1-F6)"
```

---

### Task 2: task-detail：unknown 门核对口径与置门后自动核对（R1-F2 + R1-F3）

**Files:**
- Modify: `packages/mobile-core/src/task-office/task-detail.ts:344-352`（hydrate 门核对改合并口径，R1-F2）
- Modify: `packages/mobile-core/src/task-office/task-detail.ts:548-554`（act() unknown 分支置门后安排核对，R1-F3）
- Test: `packages/mobile-core/src/task-office/task-detail.test.ts`

**Interfaces:**
- Consumes: `terminalRunStatusOf`（task-timeline.ts:41-55，task-detail.ts:3 已 import；base 为 queued/running/reconciling 时按事件流推进 canceled 等）；`resolveUnknownStop`（task-intent.ts:38-40，`observedRunStatus === 'canceled'` 才 confirmed）；hydrate 内 `events` 已在 :329 完成持久化×快照合并（门核对块 :344-352 位于其后，`events` 可直接用）；act() 的 `void hydrate().catch(() => undefined)` 先例（accepted :536、conflict :545）；测试夹具 `newHandleForIntervention`（test:730-742，`detailBackend.detailResult` 可换、流永不 resolve）、`scriptedCommandPort`、`codedError`。
- Produces: `:345` 由 `resolveUnknownStop(fetched.execution.runStatus)` 改为 `resolveUnknownStop(terminalRunStatusOf(fetched.execution.runStatus, events))`——与同函数 :353 及全模块其余 5 处（:143/:187/:431/:454）终态判断口径一致；act() unknown 分支在 `unknownGate = { revision }` 之后追加 `void hydrate().catch(() => undefined);`。门语义不变：核对完成前 :502 仍拒绝一切写意图。

**根因与修复说明：** 两项同根因——unknown 门的生命周期（置入→核对→解除）有两处断裂：(a) **核对口径不一致**（R1-F2）：`hydrate` 门核对块 :345 用原始 `fetched.execution.runStatus` 喂 `resolveUnknownStop`，而 `terminalRunStatusOf`（task-timeline.ts:41-55）在 base 为 running 时按事件流推进 canceled——模块注释 :199 明确「快照未刷新前流内终态事件也要推进视图（R1-F27）」。取消事件先到场景：:345 判 not-landed → :349 `stopState = undefined` 门解除，:353 合并口径又判终态 notify('drained')——视图 `runStatus='canceled'` 但无 confirmed 停止卡；用户重试 stop 经 :541 得 `outcome:'conflict'` 回执（服务端 CAS 已落地）。作者已实跑复现（resync 后停止卡消失）。文件内其余终态判断全部用合并口径，:345 是唯一例外。(b) **置门后无核对安排**（R1-F3）：act() unknown 分支 :548-554 只置 `stopState`/`unknownGate` + notify，对照 accepted（:536）/ conflict（:545）均有 `void hydrate().catch()`；门唯一解除点是 hydrate :344-352——流存活但 Run 自然完成走 streamEnded 终态分支（不进 hydrate）时，门永久滞留，:502 阻塞一切写意图，仅剩外部 resync 一条恢复路径（离线降级场景受物理约束，见差异记录 7）。修复：unknown 分支与另两分支对齐，置门后安排一次核对。

- [ ] **Step 1: 写失败测试（3 个，追加到 task-detail.test.ts 的 `act(stop) unknown reconciles to confirmed when the run was actually canceled` 用例之后）**

```ts
test('the unknown gate reconciles on the merged caliber: an in-stream cancel lands confirmed (R1-F2)', async () => {
  const commands = scriptedCommandPort({ error: codedError('TASK_COMMAND_UNKNOWN') });
  const { lease } = leased();
  const scripted = createScriptedTaskStream();
  let detailResult = interventionDetail({ runStatus: 'running', revision: 2 });
  const handle = createTaskDetail(
    { taskId: 's1', runId: 'run-1' },
    {
      backend: {
        detail: async () => detailResult,
        stream: (input: { runId: string; cursor: number; signal: AbortSignal; onEvent(event: TaskBackendEvent): void; onControl(frame: TaskStreamControlFrame): void }) =>
          new Promise<void>((resolve) => {
            scripted.attach({ signal: input.signal, onEvent: input.onEvent, onControl: input.onControl, resolve, reject: () => undefined });
          }),
      },
      store: createInMemoryTaskProjectionStore(),
      lease: () => lease,
      commands: commands.port,
    },
  );
  await handle.hydrate();
  const receipt = await handle.act({ kind: 'stop' });
  assert.equal(receipt.outcome, 'unknown');
  assert.equal(handle.view()?.stop?.phase, 'unknown');
  // 取消事件经流先到并持久化（终态 → processEvent 立即落盘），服务端快照刷新滞后仍是 running
  scripted.emit(event$(1, 'run.canceled')); // seq 必须 = committedCursor + 1（差异记录 6）
  await settle();
  assert.equal(handle.view()?.runStatus, 'canceled', '合并口径下视图已推进取消（R1-F27 语义）');
  detailResult = interventionDetail({ runStatus: 'running', revision: 2 }); // 快照仍滞后
  await handle.resync();
  assert.equal(handle.view()?.runStatus, 'canceled', '持久化事件主导合并口径');
  assert.equal(handle.view()?.stop?.phase, 'confirmed', '已落地取消必须核对为 confirmed——不得因快照滞后被误判未落地');
  handle.close('done');
});

test('the unknown gate schedules its own reconciliation: a landed cancel settles confirmed without resync (R1-F3)', async () => {
  const commands = scriptedCommandPort({ error: codedError('TASK_COMMAND_UNKNOWN') });
  const { handle, detailBackend } = newHandleForIntervention({ runStatus: 'running', revision: 2 }, commands.port);
  await handle.hydrate();
  detailBackend.detailResult = interventionDetail({ runStatus: 'canceled', revision: 3 }); // 服务端其实已落地取消
  const receipt = await handle.act({ kind: 'stop' });
  assert.equal(receipt.outcome, 'unknown');
  await settle(); // 置门后安排的核对（无外部操作、无 resync）
  assert.equal(handle.view()?.stop?.phase, 'confirmed', '门不得滞留：置门后自动核对，取消其实已落地');
  assert.equal(handle.view()?.runStatus, 'canceled');
  handle.close('done');
});

test('the unknown gate schedules its own reconciliation: not landed releases the gate for a retry (R1-F3)', async () => {
  const commands = scriptedCommandPort({ error: codedError('TASK_COMMAND_UNKNOWN') });
  const { handle } = newHandleForIntervention({ runStatus: 'running', revision: 2 }, commands.port);
  await handle.hydrate();
  const receipt = await handle.act({ kind: 'stop' });
  assert.equal(receipt.outcome, 'unknown');
  assert.equal(handle.view()?.stop?.phase, 'unknown');
  // 核对窗口内写意图仍被阻止（AC2 不放松）。
  await assert.rejects(() => handle.act({ kind: 'steer', text: 'x' }), /TASK_OFFICE_COMMAND_UNKNOWN/);
  await settle(); // 置门后安排的核对：快照仍 running ⇒ 未落地 ⇒ 门解除
  assert.equal(handle.view()?.stop, undefined, '未落地则门解除、停止卡清除——滞留窗口收敛为一次核对');
  commands.script.error = undefined;
  const retry = await handle.act({ kind: 'stop' });
  assert.equal(retry.outcome, 'accepted', '门解除后写意图恢复');
  handle.close('done');
});
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/task-detail.test.ts`
Expected: FAIL ×3——用例 1：resync 后 `stop.phase` 为 `undefined`（作者复现脚本证实误判 not-landed）；用例 2：settle 后 `stop.phase` 仍为 `unknown`（无核对安排）；用例 3：settle 后 `stop` 仍为 unknown 停止卡、retry 抛 `TASK_OFFICE_COMMAND_UNKNOWN`。

- [ ] **Step 3: 最小实现**

```ts
// task-detail.ts hydrate 门核对块（:344-352）——R1-F2：核对改用合并口径
// （与 :353 终态判断及全模块其余终态判断一致；快照未刷新前流内终态事件也参与核对，
// 模块注释 :199 的 R1-F27 语义）：
if (unknownGate !== undefined) {
  const resolved = resolveUnknownStop(terminalRunStatusOf(fetched.execution.runStatus, events));
  if (resolved === 'confirmed') {
    stopState = { phase: 'confirmed', since: stopState?.since ?? new Date().toISOString(), note: 'reconciled: canceled' };
  } else {
    stopState = undefined; // 取消未落地：门解除，用户可重试。
  }
  unknownGate = undefined;
}

// task-detail.ts act() unknown 分支（:548-554）——R1-F3：置门后安排核对
// （与 accepted :536 / conflict :545 的 fire-and-forget hydrate 同款；一次核对失败
// 由流/下次 hydrate/显式 resync 兜底，门在核对完成前照常阻塞——AC2 不放松）：
// 结果未知（传输失败/5xx/502 command_recovery_unknown）：进入 unknown 门（AC2）。
if (intent.kind === 'stop') stopState = { phase: 'unknown', since: at, note: messageOf(error) };
unknownGate = { revision };
const receipt: InterventionReceipt = { intent, outcome: 'unknown', boundRunId: input.runId, revision, note: messageOf(error), at };
interventions.push(receipt); trimInterventions();
notify(current?.connection === undefined ? 'syncing' : current.connection); // 停止卡/回执立即可见
void hydrate().catch(() => undefined); // R1-F3：置门后自行安排核对，门不得滞留到仅剩显式 resync
return receipt;
```

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/task-detail.test.ts`
Expected: PASS（含既有「act(stop) with an unknown delivery outcome gates further writes until a resync reconciles」「act(stop) unknown reconciles to confirmed when the run was actually canceled」——显式 resync 核对路径不受扰动，门禁与解除语义不变）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/task-office/task-detail.ts packages/mobile-core/src/task-office/task-detail.test.ts
git commit -m "fix(task-detail): reconcile the unknown gate on the merged caliber and schedule its own reconciliation (R1-F2, R1-F3)"
```

---

### Task 3: 双 UI：'offline' 中断原因文案映射（R1-F4）

**Files:**
- Modify: `apps/mobile/src/screens/TaskDetailScreen.tsx:21`（INTERRUPTION_COPY 补 offline 条目）
- Modify: `apps/miniprogram/src/services/office-views.ts:24-31`（copy 补 'offline' 键）
- Test: `apps/mobile/src/app-smoke.test.tsx`（新增渲染用例）

**Interfaces:**
- Consumes: `TaskInterruptionReason`（task-detail.ts:13，含 `'offline'`；经 `packages/mobile-core/src/index.ts` 导出，office-views.ts:3 已 import）；`detail$` 离线降级构造的 `interruption: { reason: 'offline', message: '当前离线：以下为最近一次同步的加密缓存内容' }`（task-detail.ts:320）；app-smoke 既有 `render()`/`descendants()` helper（app-smoke.test.tsx:57-70）与红线断言（:838-839）。
- Produces: 两处文案映射对 `TaskInterruptionReason` 全集（7 键）穷举；`TaskDetailScreen` 渲染离线中断时输出中文文案，`?? view.interruption.reason` 兜底不再可达（对 'offline' 而言）。无导出变化。

**根因与修复说明：** task-detail.ts:13 新增 `'offline'` 中断原因（T10 离线降级 :320 构造），但两处 UI 消费方未同步：(a) `apps/miniprogram/src/services/office-views.ts:24` 的 `copy: Record<TaskInterruptionReason, string>` 缺 `'offline'` 键——源级契约缺口（报告称「TS 必报缺属性」，本 worktree 因 mobile-core 未链接被掩盖，见差异记录 1；:32 `return copy[reason]` 一旦类型恢复即返回 undefined → 空白提示）；(b) `apps/mobile/src/screens/TaskDetailScreen.tsx:21` 的 `INTERRUPTION_COPY: Record<string, string>` 缺键，:102 `?? view.interruption.reason` 把 `'offline'` 内部码直出用户，违反 app-smoke.test.tsx:838-839「interruption 原因必须经文案映射/不得直出内部码」红线。另：两处 UI 均只渲染 reason 映射不渲染 `interruption.message`（pages.tsx:51、TaskDetailScreen.tsx:102），:320 的 message 实际不可见——本任务补齐映射文案承载该信息（message 不可见是既有整体呈现决策，不在本批次改渲染结构）。

- [ ] **Step 1: 写失败测试（追加到 app-smoke.test.tsx 含 :838-839 红线断言的用例之后，同文件底部 helper 区之前）**

```tsx
test('the offline interruption renders mapped copy and never the raw internal code (R1-F4)', async () => {
  const { TaskDetailScreen } = await import('./screens/TaskDetailScreen.tsx');
  const offlineView = {
    taskId: 'task-1', runId: 'run-1', title: '季度竞品报告', lifecycle: 'active', runStatus: 'running',
    attention: 'none', executionStatus: 'running', settlementStatus: 'pending', revision: 4,
    cursor: 2, incomplete: false, connection: 'interrupted' as const,
    interruption: { reason: 'offline' as const, message: '当前离线：以下为最近一次同步的加密缓存内容' },
    timeline: [], duplicateSeqs: [],
  };
  const element = render(TaskDetailScreen as (props: unknown) => unknown, { view: offlineView, loading: false, onRefresh: () => undefined });
  const texts = descendants(element).filter(({ type }) => type === 'Text').flatMap(({ props }) => props.children).join(' ');
  assert.equal(texts.includes('当前离线'), true, "offline 原因必须映射为用户文案（当前渲染 'offline' 内部码）");
  assert.equal(/·\s*offline/.test(texts), false, '不得直出 offline 内部码（app-smoke 红线 B2-F40/R1-F4）');
});
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx`
Expected: FAIL——当前渲染 `连接中断，可恢复 · offline`：`texts.includes('当前离线')` 为 false，`/·\s*offline/` 命中。

- [ ] **Step 3: 最小实现**

```ts
// TaskDetailScreen.tsx:21 —— INTERRUPTION_COPY 追加（文案与 task-detail.ts:320 的 message 口径一致）：
const INTERRUPTION_COPY: Record<string, string> = { gap: '事件流出现缺口', 'cursor-expired': '同步游标过期', 'stream-error': '实时通道中断', 'stream-ended-nonterminal': '事件流提前结束', 'persist-failed': '本地保存失败', 'stream-unavailable': '此部署暂无实时通道，可手动刷新', offline: '当前离线：展示最近一次同步的加密缓存内容' };

// apps/miniprogram/src/services/office-views.ts:24-31 —— copy 追加（沿用本表长句风格）：
const copy: Record<TaskInterruptionReason, string> = {
  'gap': '事件出现缺口，正在自动重新同步；不会重放已完成的工作。',
  'cursor-expired': '服务端游标已过期裁剪，正在从快照重新同步。',
  'stream-error': '连接中断；页面保留已同步状态，可手动重新同步。',
  'stream-ended-nonterminal': '连接在非终态结束，正在核对最新状态。',
  'persist-failed': '本机缓存写入失败；已提交的服务端状态不受影响。',
  'stream-unavailable': '当前部署未提供流式通道；只能整段刷新快照。',
  'offline': '当前离线：以下为最近一次同步的缓存内容；联网后可手动重新同步。',
};
```

- [ ] **Step 4: 实跑确认通过 + 双侧验证**

Run: `pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx`
Expected: PASS（新增 1 项 + 既有 65 项全绿——含 :838-839 红线源断言）。
Run: `pnpm --filter @weknora/mobile typecheck`
Expected: exit 0（映射表字面量扩键无类型影响）。
Run: `npx tsc --noEmit -p apps/miniprogram 2>&1 | grep "office-views" | grep -c "error TS"`
Expected: `6`（与基线持平——4×TS2307 + :20 TS7053 + :41 TS2366，行号可能因插行平移；**错误集合不扩大**即达标，对照差异记录 1，miniprogram 门禁整体转绿不在本批次范围）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/screens/TaskDetailScreen.tsx apps/miniprogram/src/services/office-views.ts apps/mobile/src/app-smoke.test.tsx
git commit -m "fix(ui): map the offline interruption reason to user copy in mobile screen and miniprogram (R1-F4)"
```

---

### Task 4: task-office/guard/composition：askKnowledge 离线门禁（R1-F7；顺手 R1-F8）

**Files:**
- Modify: `packages/mobile-core/src/task-office/task-office.ts:263`（顺手 R1-F8：常量定义）、`:545`/`:645`/`:659`/`:664`（魔法数替换）、`:660-667`（askKnowledge 补 office 级门）
- Modify: `packages/mobile-core/src/offline/guarded-ports.ts`（新增 `guardKnowledgeQABackend`）
- Modify: `packages/mobile-core/src/index.ts:90`（导出新 guard）
- Modify: `apps/mobile/src/composition.ts:14`（import）、`:190-194`（knowledgeQA 接线 guard）
- Test: `packages/mobile-core/src/task-office/knowledge-qa-office.test.ts`、`packages/mobile-core/src/offline/guarded-ports.test.ts`

**Interfaces:**
- Consumes: `OfflineGate.assertOnline('run')`（offline-gate.ts:29）；start() 的 office 级门先例（task-office.ts:540-544，含「if 形式而非 ?.」「在 callBackend 之外直接上抛」注释约定）；guarded-ports.ts:28-36 的 `guardLegacyTaskBackend` 同构先例（:32 注释「knowledge-chat 追问触发服务端执行：按 Run 语义拒绝」）；测试夹具 `officeWithKnowledgeQA`/`citedOfficeEvidence`（knowledge-qa-office.test.ts:16-28 与 `function citedOfficeEvidence`）、`createOfflineGate`/`OfflineGateError`（offline-gate.ts:32/20）。
- Produces: `guardKnowledgeQABackend(port: KnowledgeQABackendPort, gate: OfflineGate): KnowledgeQABackendPort`（从 `@weknora/mobile-core` 导出）；askKnowledge 行为：`ports.gate` 注入且离线时，在任何网络派发（createSession :664 / knowledgeQA.ask :671）之前抛 `OfflineGateError('run')`（消息 `OFFLINE_ACTION_BLOCKED:run`），零后端触达。常量 `QUESTION_MAX_LEN = 8000`、`SESSION_TITLE_MAX_LEN = 60` 为模块级（不导出）。

**根因与修复说明：** askKnowledge 的首笔网络派发未接入 Offline Gate，破坏 T10 的纵深防御一致性：start() 有两道门——office 级（:544 `assertOnline('run')`，AC3）+ 端口级（guarded-ports.ts:8-16 `guardTaskBackend` 拦 Start POST，AC2）；legacy followUp 有端口级门（guarded-ports.ts:32，注释明说「knowledge-chat 追问」按 'run' 语义拒绝）；而 askKnowledge（:656-685）全函数无 gate 调用、组合根 composition.ts:190-194 的 `knowledgeQA` 端口裸装配无任何 guard。离线时 createSession（快速提问的前置调用，:664）的 transport 错误经 `callBackend`（:405-412）包装成 `TASK_OFFICE_BACKEND`——离线拒绝被伪装成「服务端故障」，违背 T10「Run commands 离线结构化拒绝」语义（mobile-ai-office-design.md Implementation Decisions）。askKnowledge 有真实消费方（`apps/mobile/src/knowledge-qa-view.ts:100`、`knowledge-qa-integration-smoke.ts:107`）。修复两层同补（与 start() 的既有双门结构完全对齐）：office 级门在 :660 端口检查之后、首个派发之前（覆盖 createSession 与 ask 两笔派发）；端口级 `guardKnowledgeQABackend` 在组合根接线。顺手 R1-F8：`8000`/`60` 内联魔法数（followUp:645、askKnowledge:659、start:545、askKnowledge:664 四处两值重复维护）提取为模块常量。

- [ ] **Step 1: 写失败测试**

`packages/mobile-core/src/task-office/knowledge-qa-office.test.ts` 追加（顶部 import 区追加 `import { createOfflineGate, OfflineGateError } from '../offline/offline-gate.ts';`）：

```ts
test('offline askKnowledge is refused OFFLINE_ACTION_BLOCKED:run with zero backend dispatch (R1-F7)', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const created: string[] = [];
  const backend = createScenarioTaskBackend({
    createSession: async (input) => { created.push(input.title); return { sessionId: 'new-1' }; },
  });
  const knowledgeQA = createScenarioKnowledgeQABackend({
    ask: async () => ({ answer: '不应到达', isFallback: false, evidence: citedOfficeEvidence() }),
  });
  const office = createTaskOffice({
    backend, knowledgeQA,
    gate: createOfflineGate({ online: async () => false }),
    lease: () => leaseRef.lease,
  });
  await assert.rejects(
    office.askKnowledge({ question: '离线提问' }),
    (error: unknown) => error instanceof OfflineGateError && error.action === 'run',
    '离线 askKnowledge 必须以 OFFLINE_ACTION_BLOCKED:run 结构化拒绝，而不是 transport 错误伪装成 TASK_OFFICE_BACKEND',
  );
  assert.deepEqual(created, [], '零后端派发：createSession 不得被触达');
  assert.equal(knowledgeQA.calls.length, 0, '零后端派发：ask 不得被触达');
});

test('an online gate lets askKnowledge dispatch as usual (R1-F7 no-regression)', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const created: string[] = [];
  const knowledgeQA = createScenarioKnowledgeQABackend({
    ask: async () => ({ answer: '回答', isFallback: false, evidence: citedOfficeEvidence() }),
  });
  const office = createTaskOffice({
    backend: createScenarioTaskBackend({
      createSession: async (input) => { created.push(input.title); return { sessionId: `new-${created.length}` }; },
    }),
    knowledgeQA,
    gate: createOfflineGate({ online: async () => true }),
    lease: () => leaseRef.lease,
  });
  const turn = await office.askKnowledge({ question: '在线提问' });
  assert.equal(turn.sessionId, 'new-1');
  assert.equal(turn.answer, '回答');
  assert.deepEqual(created, ['在线提问'], '在线门放行：createSession 正常触达');
  assert.equal(knowledgeQA.calls.length, 1, '在线门放行：ask 正常触达');
});
```

`packages/mobile-core/src/offline/guarded-ports.test.ts` 追加（顶部 import 区按本文件既有形态追加 `import type { KnowledgeQABackendPort } from '../task-office/knowledge-qa.ts';` 与 `import { guardKnowledgeQABackend } from './guarded-ports.ts';`——后者并入 :8 的既有具名 import 列表）：

```ts
const knowledgeTurn = {
  answer: '回答', isFallback: false,
  evidence: {
    state: 'cited' as const, semanticGraphUsed: false, retrievedAt: '2026-09-24T08:00:00Z',
    citations: [], conclusions: [],
    reasoning: { requested: false, state: 'not_requested' as const, retryable: false },
  },
};

test('offline knowledge ask is refused as run; online passes through (R1-F7 port-level defense)', async () => {
  const log: string[] = [];
  const knowledgeQA: KnowledgeQABackendPort = { ask: async () => { log.push('ask'); return knowledgeTurn; } };
  const guarded = guardKnowledgeQABackend(knowledgeQA, createOfflineGate(offline));
  await assert.rejects(
    guarded.ask({ sessionId: 's1', question: '问' }),
    (error: unknown) => error instanceof OfflineGateError && error.action === 'run',
  );
  assert.deepEqual(log, [], 'an offline dangerous action must never reach the backend');
  const onlineGuarded = guardKnowledgeQABackend(knowledgeQA, createOfflineGate(online));
  const turn = await onlineGuarded.ask({ sessionId: 's1', question: '问' });
  assert.equal(turn.answer, '回答');
  assert.deepEqual(log, ['ask']);
});
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/knowledge-qa-office.test.ts packages/mobile-core/src/offline/guarded-ports.test.ts`
Expected: FAIL——用例 1：`askKnowledge` resolve 成功（无门）或 rejects 非 `OfflineGateError`（transport 错误经 callBackend 包装为 TASK_OFFICE_BACKEND）；`guardKnowledgeQABackend` 未定义导致 guarded-ports.test.ts 编译失败也算预期 RED（先补导出与空实现使编译过、断言红）。

- [ ] **Step 3: 最小实现**

```ts
// task-office.ts :263 附近（searchMaxLen 旁）——R1-F8 常量：
const QUESTION_MAX_LEN = 8000; // legacy followUp 与 askKnowledge 共用的提问长度上限
const SESSION_TITLE_MAX_LEN = 60; // 目标会话标题截断（start 与 askKnowledge 共用）
// 替换四处：:545 `slice(0, 60)` → `slice(0, SESSION_TITLE_MAX_LEN)`；
// :645/:659 `question.length > 8000` → `> QUESTION_MAX_LEN`；:664 `slice(0, 60)` → `slice(0, SESSION_TITLE_MAX_LEN)`。

// task-office.ts askKnowledge（:660 端口检查之后、:661 sessionId 解析之前）——R1-F7 office 级门：
if (ports.knowledgeQA === undefined) throw new TaskOfficeError('TASK_OFFICE_KNOWLEDGE_QA_UNAVAILABLE');
// R1-F7：与 start()（:544）同款纵深防御第一道——askKnowledge 的两笔网络派发（无 sessionId
// 时的 createSession、knowledgeQA.ask）都触发服务端执行，离线时在首笔之前结构化拒绝。
// 在 callBackend 之外直接上抛（不被包装成 TASK_OFFICE_BACKEND）；if 形式而非 ?.：gate
// 缺省时不引入额外微任务（与 start :543 注释同理）。
if (ports.gate !== undefined) await ports.gate.assertOnline('run');

// offline/guarded-ports.ts —— 新增（沿用 guardLegacyTaskBackend 同构与注释风格）：
export function guardKnowledgeQABackend(port: KnowledgeQABackendPort, gate: OfflineGate): KnowledgeQABackendPort {
  return {
    ...port,
    async ask(input: KnowledgeQAAskInput) {
      await gate.assertOnline('run'); // 知识问答触发服务端执行：按 Run 语义拒绝（与 legacy followUp 同语义）
      return port.ask(input);
    },
  };
}
// 文件顶部 import 区追加：
// import type { KnowledgeQABackendPort, KnowledgeQAAskInput } from '../task-office/knowledge-qa.ts';

// packages/mobile-core/src/index.ts:90 —— 导出新 guard：
export { guardInteractionBackend, guardKnowledgeQABackend, guardLegacyTaskBackend, guardTaskBackend } from './offline/guarded-ports.ts';

// apps/mobile/src/composition.ts:14 —— import 追加 guardKnowledgeQABackend；
// :190-194 —— knowledgeQA 端口接线 guard（组合根的端口级纵深，与 backend/interactions/legacy 同款）：
knowledgeQA: guardKnowledgeQABackend(createMobileKnowledgeQARemote({
  origin,
  request: (input) => activeRuntime.authorizedRequest(input),
  stream: (input, onChunk) => activeRuntime.authorizedEventStream(input, onChunk),
}), nativeOfflineGate),
```

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/knowledge-qa-office.test.ts packages/mobile-core/src/offline/guarded-ports.test.ts`
Expected: PASS（既有 + 新 3 个；既有「askKnowledge fails closed without a knowledgeQA port and rejects invalid input」——`'x'.repeat(8001)` 边界用例即 R1-F8 的行为钉子——与「askKnowledge creates a session…」的 `slice(0, 60)` 截断断言全绿，证明常量提取零行为变化）。
Run: `pnpm --filter @weknora/mobile typecheck`
Expected: exit 0（composition 接线类型正确）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/task-office/task-office.ts packages/mobile-core/src/task-office/knowledge-qa-office.test.ts packages/mobile-core/src/offline/guarded-ports.ts packages/mobile-core/src/offline/guarded-ports.test.ts packages/mobile-core/src/index.ts apps/mobile/src/composition.ts
git commit -m "fix(task-office): gate askKnowledge through the offline gate at office and port level (R1-F7); name the 8000/60 magic numbers (R1-F8)"
```

---

### Task 5: 契约清理：TASK_OFFICE_COMMAND_CONFLICT 死码移除（R1-F9）

**Files:**
- Modify: `packages/mobile-core/src/task-office/task-office-errors.ts:25`（union 成员移除）
- Modify: `apps/mobile/src/task-detail-view.ts:19`（文案映射条目移除）
- Test: `apps/mobile/src/task-detail-view.test.ts:99`（断言翻转防回潮）

**Interfaces:**
- Consumes: `TASK_OFFICE_ERROR_COPY`（task-detail-view.ts:11-21 的公开导出映射）；`TaskOfficeErrorCode` union（task-office-errors.ts:10-27）。grep 复核（差异记录 4）：全库仅 3 处引用，无抛出点、无运行时消费者。
- Produces: `TaskOfficeErrorCode` union 收窄一个成员；`TASK_OFFICE_ERROR_COPY` 不再含死键。无行为变化（该码从未被抛出，文案永不命中）。

**根因与修复说明：** `'TASK_OFFICE_COMMAND_CONFLICT'`（task-office-errors.ts:25）是死码：全库无任何 `new TaskOfficeError('TASK_OFFICE_COMMAND_CONFLICT')` 抛出点，唯一语义出口是 `apps/mobile/src/task-detail-view.ts:19` 的文案映射「任务状态已变化，正在刷新最新状态。」——永不命中。确定性冲突的真实通道是 api-client 层跨包契约码 `TASK_COMMAND_CONFLICT`（task-detail.ts:155 透传 → act() :541-547 转 `outcome:'conflict'` 回执），与本码无关（差异记录 4）。死键停留在 union 与映射里会误导后续开发者以为存在该错误路径。修复：三处一并移除，测试断言翻转为「死键不得回潮」。

- [ ] **Step 1: 写失败测试（task-detail-view.test.ts:99 的既有断言翻转）**

现有（:99）：

```ts
assert.equal(TASK_OFFICE_ERROR_COPY.TASK_OFFICE_COMMAND_CONFLICT, '任务状态已变化，正在刷新最新状态。');
```

改为：

```ts
assert.equal('TASK_OFFICE_COMMAND_CONFLICT' in TASK_OFFICE_ERROR_COPY, false, '死码不得回潮：该码从未有抛出点（R1-F9），冲突走跨包契约码 TASK_COMMAND_CONFLICT');
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/task-detail-view.test.ts`
Expected: FAIL——当前映射仍含死键，`in` 判定为 true。

- [ ] **Step 3: 最小实现**

```ts
// task-office-errors.ts —— :25 整行移除：
//   | 'TASK_OFFICE_COMMAND_CONFLICT'
// （union 其余成员与 TaskOfficeError 类不动。）

// apps/mobile/src/task-detail-view.ts —— :19 整行移除：
//   TASK_OFFICE_COMMAND_CONFLICT: '任务状态已变化，正在刷新最新状态。',
```

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `pnpm exec tsx --test apps/mobile/src/task-detail-view.test.ts`
Expected: PASS。
Run: `pnpm --filter @weknora/mobile typecheck`
Expected: exit 0（union 收窄后任何残留引用会在此爆出——grep 已证无）。
Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/task-detail.test.ts`
Expected: PASS（mobile-core 侧消费不受扰动）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/task-office/task-office-errors.ts apps/mobile/src/task-detail-view.ts apps/mobile/src/task-detail-view.test.ts
git commit -m "chore(task-office): remove the never-thrown TASK_OFFICE_COMMAND_CONFLICT dead code (R1-F9)"
```

---

## 附录 A：批次验收（全部任务完成后）

- Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/task-detail.test.ts packages/mobile-core/src/task-office/knowledge-qa-office.test.ts packages/mobile-core/src/offline/guarded-ports.test.ts apps/mobile/src/task-detail-view.test.ts apps/mobile/src/knowledge-qa-view.test.ts`
  Expected: 全绿（基线 56 + 本批次新增 ≥6；`knowledge-qa-view.test.ts` 为 Task 4 消费侧回归）。
- Run: `pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx`
  Expected: **66 pass / 0 fail**（基线 65 + Task 3 新增 1）。
- Run: `pnpm --filter @weknora/mobile typecheck`
  Expected: exit 0。
- Run: `npx tsc --noEmit -p apps/miniprogram 2>&1 | grep -c "error TS"`
  Expected: `≤ 71` 且 `grep "office-views" | grep -c "error TS"` 为 `6`（预存在集合不扩大、不新增错误类别；门禁转绿不在本批次范围，见差异记录 1）。
- 失败集合不扩大原则：以上任何预存在失败（miniprogram 71 错）不得因本批次新增；若既有用例因新语义而红（如未来对 streamEnded 放行时序有依赖的用例），按新语义修正断言并在 commit message 注明。
