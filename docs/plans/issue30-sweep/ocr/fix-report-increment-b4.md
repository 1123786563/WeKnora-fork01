# OCR 增量修复批次 4（B4）执行报告

- **执行者**：OCR 修复员-B4（subagent，按计划逐任务 TDD 实施）
- **日期**：2026-09-25
- **Worktree**：`.worktrees/issue30-sweep`（分支 `codex/issue30-mobile-office`）
- **计划**：`docs/plans/issue30-sweep/plans/ocr-fix-increment-b4.md`（commit `caf5ba218`）
- **起始 HEAD**：`caf5ba218`（工作区仅 B4 发现报告 untracked，无其他脏改动）
- **结果**：5 个任务全部完成，5 项主发现 + 3 项 lowWorth 全部处置，5 个 commit

## 0. 基线复跑（与计划作者声明逐项一致）

| 检查 | 命令 | 计划声明 | 本次实跑 |
|---|---|---|---|
| 4 文件测试基线 | `pnpm exec tsx --test …/task-detail.test.ts …/knowledge-qa-office.test.ts …/guarded-ports.test.ts …/task-detail-view.test.ts` | 56 pass / 0 fail | **56 pass / 0 fail** |
| app-smoke 基线 | `pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx` | 65 pass / 0 fail | **65 pass / 0 fail** |
| mobile typecheck | `pnpm --filter @weknora/mobile typecheck` | exit 0 | **exit 0** |
| miniprogram typecheck | `npx tsc --noEmit -p apps/miniprogram`（grep 计数） | 71 错；office-views 6 错 | **71 错；office-views 6 错** |

## 1. Task 1：自然完成终态放行 parked queue-next（R1-F1；顺手 R1-F6）→ `4b86e53dc`

**改动**：
- `packages/mobile-core/src/task-office/task-detail.ts` `streamEnded` 终态分支补 `void flushQueuedIntents().catch(() => undefined)`（与 hydrate 终态分支逐字同款放行）。
- 顺手 R1-F6：`snapshotOf` 由手工逐字段拷贝改为 `const { events: _events, ...snapshot } = source` 解构（`OfflineTaskSnapshot = Omit<TaskBackendDetail, 'events'>`，纯结构重构，既有离线往返两用例钉住行为不变）。

**RED**（新增用例 `a naturally ended stream releases parked queue-next without an explicit flush (R1-F1)`）：

```
✖ a naturally ended stream releases parked queue-next without an explicit flush (R1-F1)
  AssertionError [ERR_ASSERTION]: 自然完成必须兑现「已排队（等待当前 Run 结束后发出）」承诺——parked 项自动放行
  0 !== 1
ℹ tests 43   ℹ pass 42   ℹ fail 1
```

与计划 Step 2 预期逐字吻合（前置 drained/succeeded 断言已过，`commands.calls.length` 0 !== 1）。

**GREEN**：

```
✔ act(queue-next) on an active run parks locally and flushes on terminal observation
✔ a flush racing a dead scope lease is rejected, never silently recorded (§5.3)
✔ a naturally ended stream releases parked queue-next without an explicit flush (R1-F1)
ℹ tests 43   ℹ pass 43   ℹ fail 0
```

## 2. Task 2：unknown 门核对合并口径 + 置门后自动核对（R1-F2 + R1-F3）→ `57ced4782`

**改动**（`task-detail.ts` 两处）：
- hydrate 门核对块：`resolveUnknownStop(fetched.execution.runStatus)` → `resolveUnknownStop(terminalRunStatusOf(fetched.execution.runStatus, events))`（R1-F2：与全模块终态判断口径一致）。
- act() unknown 分支：notify 后追加 `void hydrate().catch(() => undefined)`（R1-F3：与 accepted/conflict 分支同款 fire-and-forget 核对；门语义与 AC2 不变）。

**计划偏差（2 处，均为计划内测试代码与代码现状的冲突，实现本身严格照计划）**：

1. **用例 1 时序重排**：计划原文是「act(stop) → unknown → 流内 emit run.canceled → resync 断言 confirmed」。但 R1-F3 修复落地后，`void hydrate()` 在 act() 返回前同步执行到 `abortStream()`（task-detail.ts hydrate 体在首个 await 前同步调 abortStream），活动流被杀，emit 落在已中止的 sink 上不可达——计划内时序在两修复并存后物理不可达（首次实跑即复现：`合并口径下视图已推进取消` 断言 actual 'running'）。真实时序是「服务端已取消、取消事件先经流送达并持久化，随后取消命令回执丢失」——故把 emit+settle 移到 act(stop) 之前。重排后 R1-F2 修复行仍被精确执行（置门后的核对拿到的正是滞后快照）。
2. **用例 1 夹具补 `watermark: 1`**：`mergeEventHistory` 只保留 `seq <= watermark` 的事件（task-timeline.ts），计划夹具 `interventionDetail` 硬编码 `watermark: 0`，resync 时持久化的 seq 1 取消事件会被权威水位裁掉——**计划的最小实现（只改核对口径）在该夹具下无论如何无法转绿**（首次 RED 实跑证实失败点在更早的 `持久化事件主导合并口径`，actual 'running'）。给 `interventionDetail` 加可选 `watermark` 参数（向后兼容），滞后快照传 `watermark: 1`（服务端事件日志已含取消、runStatus 投影字段滞后仍报 running——正是 `terminalRunStatusOf` 合并口径的适用场景）。

重排后按计划 Step 2 的预测失败模式重新验证 RED（stash 实现文件、保测试实跑、stash pop）：

```
✖ the unknown gate reconciles on the merged caliber: an in-stream cancel lands confirmed (R1-F2)
  AssertionError [ERR_ASSERTION]: 已落地取消必须核对为 confirmed——不得因快照滞后被误判未落地
    actual: undefined,   expected: 'confirmed',
✖ the unknown gate schedules its own reconciliation: a landed cancel settles confirmed without resync (R1-F3)
    actual: 'unknown',   expected: 'confirmed',
✖ the unknown gate schedules its own reconciliation: not landed releases the gate for a retry (R1-F3)
    actual: { phase: 'unknown', … },   expected: undefined,
ℹ tests 46   ℹ pass 43   ℹ fail 3
```

**GREEN**：

```
✔ the unknown gate reconciles on the merged caliber: an in-stream cancel lands confirmed (R1-F2)
✔ the unknown gate schedules its own reconciliation: a landed cancel settles confirmed without resync (R1-F3)
✔ the unknown gate schedules its own reconciliation: not landed releases the gate for a retry (R1-F3)
ℹ tests 46   ℹ pass 46   ℹ fail 0
```

既有 unknown 门用例（显式 resync 核对、AC2 核对窗口内写意图拒绝）全绿，门禁语义未放松。

## 3. Task 3：双 UI 'offline' 中断原因文案映射（R1-F4）→ `adea98594`

**改动**：
- `apps/mobile/src/screens/TaskDetailScreen.tsx:21` `INTERRUPTION_COPY` 追加 `offline: '当前离线：展示最近一次同步的加密缓存内容'`。
- `apps/miniprogram/src/services/office-views.ts` `interruptionNotice` 的穷举 `Record<TaskInterruptionReason, string>` 追加 `'offline'` 键（源级契约补齐；本 worktree 因 mobile-core 未链接（差异记录 1）不产生编译差异）。

**RED**：

```
✖ the offline interruption renders mapped copy and never the raw internal code (R1-F4)
  AssertionError [ERR_ASSERTION]: offline 原因必须映射为用户文案（当前渲染 'offline' 内部码）
  false !== true
ℹ tests 66   ℹ pass 65   ℹ fail 1
```

**GREEN + 双侧验证**：

```
✔ the offline interruption renders mapped copy and never the raw internal code (R1-F4)
ℹ tests 66   ℹ pass 66   ℹ fail 0
pnpm --filter @weknora/mobile typecheck → exit 0
npx tsc --noEmit -p apps/miniprogram | grep -c "error TS"        → 71（持平）
npx tsc --noEmit -p apps/miniprogram | grep "office-views" | grep -c "error TS" → 6（集合持平：4×TS2307 + :20 TS7053 + :42 TS2366，行号因插行平移）
```

## 4. Task 4：askKnowledge 离线门禁 office 级 + 端口级（R1-F7；顺手 R1-F8）→ `0dcadd708`

**改动**：
- `packages/mobile-core/src/task-office/task-office.ts`：askKnowledge 在端口检查之后、首笔派发之前补 `if (ports.gate !== undefined) await ports.gate.assertOnline('run');`（在 callBackend 之外直接上抛，OFFLINE_ACTION_BLOCKED:run 不被包装成 TASK_OFFICE_BACKEND）。
- `packages/mobile-core/src/offline/guarded-ports.ts` 新增 `guardKnowledgeQABackend`（`ask` 前置 `assertOnline('run')`，与 `guardLegacyTaskBackend` 同构同注释风格）；`packages/mobile-core/src/index.ts:90` 补导出；`apps/mobile/src/composition.ts` knowledgeQA 端口接线 guard（组合根纵深第二道）。
- 顺手 R1-F8：`QUESTION_MAX_LEN = 8000`、`SESSION_TITLE_MAX_LEN = 60` 提取为模块常量，替换 start:545 / followUp:645 / askKnowledge:659 / askKnowledge:664 四处魔法数。

**计划外必要适配（附录 A 失败集合不扩大原则）**：`apps/mobile/src/app-smoke.test.tsx` 既有源级断言 `knowledgeQA:\s*createMobileKnowledgeQARemote\(`（"composition wires the knowledge QA remote…" 用例）在 guard 包装后不再命中——按附录 A「既有用例因新语义而红时按新语义修正断言并在 commit message 注明」，断言更新为要求 `knowledgeQA:\s*guardKnowledgeQABackend\(\s*createMobileKnowledgeQARemote\(`（语义反而更强：装配必须经 guard）。已在 commit `0dcadd708` message 注明。

**RED**：

```
SyntaxError: The requested module './guarded-ports.ts' does not provide an export named 'guardKnowledgeQABackend'
✖ packages/mobile-core/src/offline/guarded-ports.test.ts（编译失败——计划 Step 2 预期的 RED 形态）
✖ offline askKnowledge is refused OFFLINE_ACTION_BLOCKED:run with zero backend dispatch (R1-F7)
  AssertionError [ERR_ASSERTION]: Missing expected rejection: 离线 askKnowledge 必须以 OFFLINE_ACTION_BLOCKED:run 结构化拒绝，…
ℹ tests 8   ℹ pass 6   ℹ fail 2
```

**GREEN + 回归**：

```
✔ offline knowledge ask is refused as run; online passes through (R1-F7 port-level defense)
✔ offline askKnowledge is refused OFFLINE_ACTION_BLOCKED:run with zero backend dispatch (R1-F7)
✔ an online gate lets askKnowledge dispatch as usual (R1-F7 no-regression)
ℹ tests 13   ℹ pass 13   ℹ fail 0
pnpm --filter @weknora/mobile typecheck → exit 0
pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx → 66 pass / 0 fail
```

既有边界钉子（`'x'.repeat(8001)` 拒绝、标题 `slice(0, 60)` 截断断言）全绿，R1-F8 常量提取零行为变化。

## 5. Task 5：TASK_OFFICE_COMMAND_CONFLICT 死码移除（R1-F9）→ `34d3e404d`

**前置复核**：`grep -rn "TASK_OFFICE_COMMAND_CONFLICT" --include="*.ts" --include="*.tsx" packages apps`（排除 node_modules）恰好 3 处——`task-office-errors.ts:25`（union 成员）、`apps/mobile/src/task-detail-view.ts:19`（文案映射）、`task-detail-view.test.ts:99`（映射断言），与计划差异记录 4 一致，无抛出点。`task-detail.ts:155` 透传的跨包契约码 `TASK_COMMAND_CONFLICT` 不受影响（不同码）。

**改动**：union 成员移除、映射条目移除、测试断言翻转为 `'TASK_OFFICE_COMMAND_CONFLICT' in TASK_OFFICE_ERROR_COPY === false`（防回潮）。

**RED**：

```
ℹ tests 4   ℹ pass 3   ℹ fail 1
AssertionError [ERR_ASSERTION]: 死码不得回潮：该码从未有抛出点（R1-F9），冲突走跨包契约码 TASK_COMMAND_CONFLICT
```

**GREEN + 回归**：

```
pnpm exec tsx --test apps/mobile/src/task-detail-view.test.ts → 4 pass / 0 fail
pnpm --filter @weknora/mobile typecheck → exit 0（union 收窄无残留引用）
pnpm exec tsx --test packages/mobile-core/src/task-office/task-detail.test.ts → 46 pass / 0 fail
```

## 6. 附录 A 批次验收（全部实跑）

| # | 命令 | 预期 | 实跑 |
|---|---|---|---|
| 1 | `pnpm exec tsx --test packages/mobile-core/src/task-office/task-detail.test.ts packages/mobile-core/src/task-office/knowledge-qa-office.test.ts packages/mobile-core/src/offline/guarded-ports.test.ts apps/mobile/src/task-detail-view.test.ts apps/mobile/src/knowledge-qa-view.test.ts` | 全绿（基线 56 + 新增 ≥6） | **68 pass / 0 fail**（56 + 本批新增 7；另含消费侧 `knowledge-qa-view.test.ts` 5 项） |
| 2 | `pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx` | 66 pass / 0 fail | **66 pass / 0 fail** |
| 3 | `pnpm --filter @weknora/mobile typecheck` | exit 0 | **exit 0** |
| 4 | `npx tsc --noEmit -p apps/miniprogram` 计数 | ≤71 且 office-views 6 | **71 / 6**（预存在集合零扩大） |

失败集合不扩大：miniprogram 71 个预存在错误零新增；既有 app-smoke 断言适配 1 处（Task 4，已注明）；无其他既有用例变红。

## 7. 发现覆盖对账（ask 材料）

| 发现 | 任务 | 状态 |
|---|---|---|
| R1-F1（high，自然完成不放行 parked queue-next） | Task 1 | ✅ 修复（RED→GREEN 实跑） |
| R1-F2（medium，unknown 门核对口径不一致） | Task 2 | ✅ 修复（测试时序与夹具两处适配，见 §2） |
| R1-F3（medium，置门后无核对安排） | Task 2 | ✅ 修复 |
| R1-F4（medium，'offline' 文案缺映射） | Task 3 | ✅ 修复（双 UI 侧） |
| R1-F6（lowWorth，snapshotOf 手工拷贝） | Task 1 顺手项 | ✅ 改解构 |
| R1-F7（medium，askKnowledge 无离线门） | Task 4 | ✅ 修复（office 级 + 端口级 + 组合根接线） |
| R1-F8（lowWorth，8000/60 魔法数） | Task 4 顺手项 | ✅ 常量提取（4 处替换） |
| R1-F9（lowWorth，CONFLICT 死码） | Task 5 | ✅ 三处移除 + 断言翻转防回潮 |

## 8. Commit 清单

| Commit | 任务 |
|---|---|
| `4b86e53dc` | Task 1（R1-F1 + R1-F6） |
| `57ced4782` | Task 2（R1-F2 + R1-F3；message 注明测试适配） |
| `adea98594` | Task 3（R1-F4） |
| `0dcadd708` | Task 4（R1-F7 + R1-F8；message 注明 app-smoke 断言适配） |
| `34d3e404d` | Task 5（R1-F9） |

## 9. 遗留 / 范围外

- `apps/miniprogram` 的 `@weknora/mobile-core` workspace 链接缺失（71 个预存在 typecheck 错误的根源，含掩盖 office-views 穷举缺键的 4×TS2307）——计划差异记录 1 明示属独立决策，本批次未动。链接修复后 office-views 的穷举 `Record` 缺键即恢复为编译错误（本批已补 `'offline'` 键，届时不会爆出）。
- 离线降级视图中 `interruption.message` 本身不渲染（整体呈现决策，计划明示不在本批次改渲染结构）——offline 文案已承载同等信息。
