# T47 终局修复报告（最终审查 2 项发现，一次批次全修）

- 基线：`74e9c8959`（worktree `.worktrees/issue30-sweep-t47`）
- 修复范围：最终审查 2 项 important 发现，**2/2 全部修复，零未修项**
- 编号约定：与本批次审查发现清单一致——**发现 1 = requestRevision 缺离线拒绝断言（task-research.ts）；发现 2 = 修订命令通道未接线（composition.ts）**
- 结论：每项修复附 RED→GREEN 回归证据（**全部为本会话亲自实跑**，非引用先前批次输出）；受影响链路全量回归全绿（mobile-core 330/330、apps/mobile 234：222 pass + 12 opt-in skip / 0 fail、typecheck 干净、计划门 Go 部分 build + 三测全 ok）

> 批次说明（如实）：进场时 worktree 已有一个中断批次遗留的同套未提交修复与旧版报告（编号与本清单相反、证据属该批次）。本会话逐一核实其与两项发现、计划约束、同族先例的一致性后：保留修复内容，把 6 处代码注释中的发现编号对齐本清单，亲自重跑全部 RED/GREEN 与回归证据（RED 经「保存补丁→回退 4 个源文件到 HEAD→实跑→`git apply` 恢复→GREEN 复确认」复现），并重写本报告。

---

## 发现 1（important）：requestRevision 缺 OfflineGate 结构化断言

### 根因核实（本会话实读）

- 修复前（`git show HEAD:packages/mobile-core/src/research/task-research.ts` 的 requestRevision，对应审查引用 :131-156）仅有 `check()`（scope lease）与输入校验，无任何 gate 检查；本会话以回退源文件实跑复现了该缺陷行为（见下 RED）；
- `types.ts`（修复前）注释自认「缺失 → annotate 直接走网络」——gate 在场时也只用于 annotate 的草稿分支（task-research.ts:103），requestRevision 完全绕过；
- 违反计划 Global Constraints 逐字约束「修订请求属 Run 命令，离线一律拒绝（OfflineGate 断言）」（`docs/plans/issue30-sweep/plans/plan-t47.md:39`）、plan:61「离线修订请求不做草稿（属 Run 命令，spec 明文离线禁止）」、plan:15 引 spec「Offline mode ... prohibits Run commands」；
- 同族 Run 命令先例（本会话 grep 实证）：`packages/mobile-core/src/task-office/task-office.ts:546`（start）与 `:667`（askKnowledge）均为 `if (ports.gate !== undefined) await ports.gate.assertOnline('run');` 且在包装层之外原样上抛；`offline/guarded-ports.ts:13,23,33,43` 同族 `await gate.assertOnline(...)`；
- 后果链：真断网时命令派发失败落入 catch → 包装成 `RESEARCH_BACKEND`（文案「服务端暂时不可用」，误导）；gate 判 offline 而物理有网时命令仍会真实派发。当前生产因发现 2（commands 未接线）不可达，但模块是公共导出且 #71 将消费——正是审查所指的先行修复理由。

### 修复（3 处，同族行为对齐）

1. `packages/mobile-core/src/research/task-research.ts:142`（+6 行）：输入校验之后、命令派发 try/catch **之外**——

   ```ts
   if (ports.gate !== undefined) await ports.gate.assertOnline('run');
   ```

   OfflineGateError（`OFFLINE_ACTION_BLOCKED:run`，见 `offline/offline-gate.ts:20-25`）原样上抛，绝不包装成 `ResearchError/RESEARCH_BACKEND`；`if` 形式而非 `?.` 沿 task-office 同一注释约定（gate 缺省不引入额外微任务，物理离线仍由传输层兜底，fail closed 不变——offline-gate.ts:8-10 判定语义）。
2. `packages/mobile-core/src/research/types.ts:60-62`：gate 端口注释更新为「缺失 → annotate/requestRevision 直接走网络（物理离线由传输层兜底）。在场时 requestRevision 属 Run 命令：派发前经 assertOnline('run') 结构化拒绝」——消除注释与行为的自相矛盾。
3. `apps/mobile/src/research-view.ts:28-34`：`messageOf` 增加 `OfflineGateError` 首分支 + 导出 `RESEARCH_OFFLINE_REVISION_COPY = '当前离线：修订请求属于运行指令，请联网后再提交。'`——结构化离线判决获得诚实用户文案，终结「服务端暂时不可用」误导。

### 回归测试与输出（本会话实跑）

**模块级** `packages/mobile-core/src/research/task-research.test.ts:185` 新增「requestRevision is a Run command: offline gate rejects before any dispatch, online passes through」：真 `createOfflineGate({ online: async () => false })`（真 OfflineGate 探测语义，非手搓 stub gate）+ 记账式 commands，断言 ① 离线以 `instanceof OfflineGateError && action === 'run' && code === OFFLINE_ACTION_BLOCKED && message === 'OFFLINE_ACTION_BLOCKED:run' && !(error instanceof ResearchError)` 拒绝；② `dispatched === 0`（判决在派发之前）；③ 在线 gate 放行照常派发；④ gate 缺省直通（既有行为回归钉死）。

- **RED**（回退 task-research.ts/types.ts/research-view.ts/composition.ts 至 HEAD 后实跑）：
  ```
  $ pnpm exec tsx --test packages/mobile-core/src/research/task-research.test.ts
  not ok 5 - requestRevision is a Run command: offline gate rejects before any dispatch, online passes through
    error: 'Missing expected rejection.'   ← 离线 requestRevision 未被拒绝
    code: 'ERR_ASSERTION'  location: task-research.test.ts:202:3
  # tests 6 / # pass 5 / # fail 1
  ```
- **GREEN**（恢复补丁后实跑，同命令）：`# tests 6 / # pass 6 / # fail 0`

**视图级** `apps/mobile/src/research-view.test.ts:117` 新增「controller requestRevision surfaces the structured offline rejection with honest copy」：stub 句柄上抛真 `OfflineGateError('run')`——断言 error 恰为 `RESEARCH_OFFLINE_REVISION_COPY` 且 `notEqual RESEARCH_ERROR_COPY.RESEARCH_BACKEND`；在线路径 notice 仍匹配 `/修订请求已提交/`。

- **RED**（同上回退态实跑）：
  ```
  not ok 4 - controller requestRevision surfaces the structured offline rejection with honest copy
    + actual   'OFFLINE_ACTION_BLOCKED:run'   ← messageOf 无 OfflineGateError 分支，裸码直出
    - expected undefined                       ← RESEARCH_OFFLINE_REVISION_COPY 尚未导出
  # tests 4 / # pass 2 / # fail 2
  ```
- **GREEN**（恢复后实跑，`pnpm --filter @weknora/mobile exec tsx --test src/research-view.test.ts`）：`# tests 4 / # pass 4 / # fail 0`

---

## 发现 2（important）：修订命令通道未接线——生产修订请求恒不可用

### 根因核实（本会话实读）

- 修复前 `composition.ts` 的 `taskResearchFor`（对应审查引用 :520-526）`createTaskResearch({ remote, gate, ...drafts })` 确缺 `commands`（HEAD 版本实核）；
- 消费链：路由 `apps/mobile/src/app/tasks/research.tsx` 经 `activeTaskResearch()` → 控制器 `research-view.ts` → `handle.requestRevision` → `task-research.ts:133`（修复前行号）的 `if (ports.commands === undefined) throw new ResearchError('RESEARCH_COMMAND_UNAVAILABLE')`——任何部署下用户都看到「此部署暂不支持修订请求通道」（`RESEARCH_ERROR_COPY`，research-view.ts:22）；
- 可复用生产端口在 `composition.ts:176`（`taskOfficeFor` 的同款 `createTaskOfficeRemote`，其 `commands: remote` 用法即 `TaskCommandPort` 可赋值的类型证明）；`createTaskOfficeRemote` 已在 composition.ts:18 导入，零新依赖；
- 计划级缺口核实：plan Task 7 的 `taskResearchFor` 代码块缺此行，实现逐字忠实——违反计划 Goal「**经既有命令通道**提出绑定确定版本的修订请求」（plan:5）。

### 所有者裁决问题的处理（如实）

审查指出该发现「需所有者裁决：接线或明示延期 #71 并记 Ledger」。本批次任务指令为「全部一次修复」，且接线是唯一满足计划 Goal 的选项（延期则发现保持未修）、成本为 1 行注入且与 :176 既有先例同款、`#71` 消费的正是接好线的公共模块（发现 1 的 gate 断言同样服务 #71）——故取「接线」，不记延期 Ledger。

### 修复

`apps/mobile/src/composition.ts:518`（+5 行含注释）：

```ts
commands: createTaskOfficeRemote({ origin, request: (input) => activeRuntime.authorizedRequest(input) }),
```

与 `taskOfficeFor` 的 commands 同款 adapter（同 origin + `authorizedRequest`；`command` 只走 request 通道，`stream` 可选不传）。离线拒绝不在此重复设防——`taskResearchFor` 已注入 `gate: nativeOfflineGate`（:513），模块级 `assertOnline('run')`（发现 1）在派发前拦截，防线正在计划 Global Constraints 指定的模块 seam 上。

### 回归测试与输出（本会话实跑）

`apps/mobile/src/research-view.test.ts:106` 既有 Interface boundary 测试新增两条断言——以 `function taskResearchFor` 至 `export function activeTaskResearch` 的**函数体切片**为界（不跨函数误匹配 taskOfficeFor 的既有 commands），断言切片含 `commands:` 且含 `createTaskOfficeRemote`（钉死「复用 #37 既有命令通道 adapter」）。

- **RED**（回退态实跑，同发现 1 的 RED 运行）：
  ```
  $ pnpm --filter @weknora/mobile exec tsx --test src/research-view.test.ts
  not ok 3 - research route and composition wiring stay at the Interface boundary
    error: 'taskResearchFor 必须注入 commands 端口：缺失时修订请求在生产中恒不可用（RESEARCH_COMMAND_UNAVAILABLE）'
  ```
- **GREEN**（恢复后实跑，同命令）：`# tests 4 / # pass 4 / # fail 0`

> 测试形态说明（如实）：跨包无法构造真 `RuntimeScopeLease`（类未从 mobile-core index 导出，`leaseActive` 以 instanceof 判定），生产 runtime 单例在单测中无法达到 authorized 面，故接线缺口采用与本测试文件既有断言（「composition 必须装配 research 深模块」）同一惯例的**源级断言**钉死；删除注入即 RED，已实证。

---

## 全量回归证据（全部本会话实跑）

计划门 = plan:3837-3844（Task 9 终局验证链，含 Go 部分——虽本 diff 零 Go 文件改动，仍按计划门原样实跑以凑齐全链）：

| 命令 | 结果 |
| --- | --- |
| `pnpm exec tsx --test packages/mobile-core/src/research/task-research.test.ts` | 6/6 pass（回退源文件同命令 1 fail = RED） |
| `pnpm --filter @weknora/mobile exec tsx --test src/research-view.test.ts` | 4/4 pass（回退态 2 fail = RED） |
| `go test ./internal/database/ -run 'TestSQLiteMigrations...|TestWorkbenchSQLite' -count=1`（计划门） | `ok github.com/Tencent/WeKnora/internal/database 11.831s` |
| `go test ./internal/application/repository/ -run 'TestTaskResearchStore|...' -count=1`（计划门） | `ok github.com/Tencent/WeKnora/internal/application/repository 23.714s` |
| `go test ./internal/handler/session/ -run 'TestDelegateResearch|...' -count=1`（计划门） | `ok github.com/Tencent/WeKnora/internal/handler/session 1.618s` |
| `go build ./...`（计划门） | 通过（仅既有链接器 warning `ignoring duplicate libraries: '-lc++'`，与基线一致） |
| `pnpm exec tsx --test packages/contracts/src/mobile/research.test.ts packages/api-client/src/mobile/research.test.ts packages/mobile-core/src/research/task-research.test.ts`（计划门 TS 链） | 14 pass / 0 fail |
| `pnpm --filter @weknora/mobile exec tsx --test src/research-view.test.ts src/research-integration-smoke.test.ts`（计划门） | 8 tests：7 pass + 1 SKIP（opt-in 真实部署凭据缺席的诚实跳过，既有语义）/ 0 fail |
| `pnpm --filter @weknora/mobile typecheck`（计划门） | 通过（`tsc --noEmit` 无错误输出） |
| `pnpm --filter @weknora/mobile exec tsx --test src/materials-view.test.ts`（计划门） | 5/5 pass |
| `pnpm --filter @weknora/mobile test`（apps/mobile **全量**，含真实装载 composition.ts 的 app-smoke） | 234 tests：222 pass / 12 skipped（opt-in 凭据门控）/ **0 fail** |
| `pnpm exec tsx --test $(find packages/mobile-core/src -name "*.test.ts")`（mobile-core **全量** 36 文件） | 330/330 pass / 0 fail |

**未跑项（如实）**：无——计划门全链（Go 三测 + go build + TS 四链）与两包全量均在本会话实跑。环境注：node v22.22.3 触发 pnpm engine WARN（wanted >=26），为该仓库在当前机器的既有状态，不影响任何测试结果。

## 变更清单

| 文件 | 变更 |
| --- | --- |
| `packages/mobile-core/src/research/task-research.ts` | +6：requestRevision 入口 `assertOnline('run')`（try/catch 之外原样上抛，发现 1） |
| `packages/mobile-core/src/research/types.ts` | 注释修正：gate 端口语义覆盖 requestRevision（发现 1 配套） |
| `apps/mobile/src/composition.ts` | +5：taskResearchFor 注入 `commands: createTaskOfficeRemote({ origin, request: authorizedRequest })`（发现 2） |
| `apps/mobile/src/research-view.ts` | +8/-1：`RESEARCH_OFFLINE_REVISION_COPY` + `messageOf` OfflineGateError 首分支（发现 1 文案） |
| `packages/mobile-core/src/research/task-research.test.ts` | +57：模块级离线断言回归测试（发现 1） |
| `apps/mobile/src/research-view.test.ts` | +37/-2：接线源级断言（发现 2）+ 离线文案控制器测试（发现 1） |
