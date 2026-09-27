# T47 终局修复报告（最终审查 2 项发现，一次批次全修）

- 基线：`74e9c8959`（worktree `.worktrees/issue30-sweep-t47`，分支 `codex/issue30-t47`）
- 修复范围：终局审查 2 项 important 发现，全部修复，各附 RED→GREEN 回归测试证据
- 结论：**2/2 修复完成，零未修项**；受影响链路全量回归全绿（mobile-core 330/330、apps/mobile 222 pass + 12 opt-in skip / 0 fail、typecheck 干净）

---

## 发现 1（important）：生产装配未注入 commands 端口——修订请求在移动端生产中恒不可用

### 根因核实（本次会话实读）

- `apps/mobile/src/composition.ts:511`（修复前）`createTaskResearch({ remote, gate, ...drafts })` 确缺 `commands`；
- 消费链真实存在：路由 `apps/mobile/src/app/tasks/research.tsx:53` → 控制器 `research-view.ts:119` → `handle.requestRevision` → 模块 `packages/mobile-core/src/research/task-research.ts:133` 的 `if (ports.commands === undefined) throw new ResearchError('RESEARCH_COMMAND_UNAVAILABLE')`，用户看到文案「此部署暂不支持修订请求通道」（`research-view.ts` RESEARCH_ERROR_COPY）；
- 可复用生产端口确在 `composition.ts:185`（`taskOfficeFor` 的 `commands: remote`，`createTaskOfficeRemote` 返回的 `command()` 结构可赋值给 `TaskCommandPort`——taskOfficeFor 既有同款用法即类型证明）；
- 计划级缺陷核实：`docs/plans/issue30-sweep/plans/plan-t47.md` Task 7 的 `taskResearchFor` 代码块（plan:3551-3556）本身缺失该行，实现逐字忠实；违反计划 Goal「经既有命令通道提出绑定确定版本的修订请求」（plan:5）。

### 修复

`apps/mobile/src/composition.ts:516`（+5 行）：

```ts
commands: createTaskOfficeRemote({ origin, request: (input) => activeRuntime.authorizedRequest(input) }),
```

与 `taskOfficeFor` 的 `commands`（:185）同款 adapter（同 origin + `authorizedRequest`；`command` 只走 request 通道，`stream` 可选不传）。`createTaskOfficeRemote` 已在 `composition.ts:18` 导入，零新依赖。

### 回归测试（RED → GREEN 实跑）

测试：`apps/mobile/src/research-view.test.ts`「research route and composition wiring stay at the Interface boundary」新增两条断言——以 `function taskResearchFor` 至 `export function activeTaskResearch` 的**函数体切片**为界（不跨函数误匹配 taskOfficeFor 的既有 commands），断言切片含 `commands:` 且含 `createTaskOfficeRemote`（钉死"复用 #37 既有命令通道 adapter"）。

- RED（修复前实跑）：
  ```
  $ pnpm --filter @weknora/mobile exec tsx --test src/research-view.test.ts
  not ok 3 - research route and composition wiring stay at the Interface boundary
    error: 'taskResearchFor 必须注入 commands 端口：缺失时修订请求在生产中恒不可用（RESEARCH_COMMAND_UNAVAILABLE）'
  ```
- GREEN（修复后实跑）：同命令 4/4 pass（含本测试）。

> 测试形态说明（如实）：跨包无法构造真 `RuntimeScopeLease`（类未从 mobile-core index 导出，`leaseActive` 以 instanceof 判定），生产 runtime 单例在单测中无法达到 authorized 面，故接线缺口采用与既有同文件同款**源级断言**钉死（本测试所在文件既有断言「composition 必须装配 research 深模块」即同一惯例）；删除注入即 RED，已实证。

---

## 发现 2（important）：requestRevision 缺 OfflineGate 结构化断言

### 根因核实（本次会话实读）

- `packages/mobile-core/src/research/task-research.ts:131-156`（修复前）`requestRevision` 仅有 `check()`（scope）与输入校验，无任何 gate 检查；
- `types.ts:60-61`（修复前）注释自认「缺失 → annotate 直接走网络（物理离线由传输层兜底）」——gate 在场时也只用于 annotate 的草稿分支；
- 违反计划 Global Constraints 逐字约束「修订请求属 Run 命令，离线一律拒绝（OfflineGate 断言）」（plan:39）与 plan:61「离线修订请求不做草稿（属 Run 命令，spec 明文离线禁止）」、spec「Offline mode ... prohibits Run commands」（plan:15 引）；
- 同族 Run 命令先例：`task-office.ts:546`（start）与 `:667`（askKnowledge）均为 `if (ports.gate !== undefined) await ports.gate.assertOnline('run');` 且在包装层之外原样上抛；
- 计划 Task 6 代码块（plan:2927-2954）同样缺失该断言，属计划自身缝隙被忠实实现；
- 后果链核实：真断网时命令派发失败落入 catch → `RESEARCH_BACKEND`（文案「服务端暂时不可用」，误导）；gate 判 offline 而物理有网时命令仍会真实派发。

### 修复（3 处，同族行为对齐）

1. `packages/mobile-core/src/research/task-research.ts:140`（+6 行）：输入校验之后、命令派发 try/catch **之外**——

   ```ts
   if (ports.gate !== undefined) await ports.gate.assertOnline('run');
   ```

   OfflineGateError（`OFFLINE_ACTION_BLOCKED:run`）原样上抛，绝不包装成 `ResearchError/RESEARCH_BACKEND`；`if` 形式而非 `?.` 沿 task-office 同一注释约定（gate 缺省不引入额外微任务，物理离线仍由传输层兜底，fail closed 不变）。
2. `packages/mobile-core/src/research/types.ts:60-62`：gate 端口注释更新为「缺失 → annotate/requestRevision 直接走网络（物理离线由传输层兜底）；在场时 requestRevision 属 Run 命令：派发前经 assertOnline('run') 结构化拒绝」——消除注释与行为的自相矛盾。
3. `apps/mobile/src/research-view.ts`：`messageOf` 增加 `OfflineGateError` 首分支 + 导出文案常量 `RESEARCH_OFFLINE_REVISION_COPY = '当前离线：修订请求属于运行指令，请联网后再提交。'`——结构化离线判决获得诚实用户文案，终结「服务端暂时不可用」误导。

### 回归测试（RED → GREEN 实跑）

**模块级** `packages/mobile-core/src/research/task-research.test.ts` 新增测试「requestRevision is a Run command: offline gate rejects before any dispatch, online passes through」：真 `createOfflineGate({ online: async () => false })`（非手搓 stub gate）+ 记账式 commands——断言 ① 离线时以 `instanceof OfflineGateError && action === 'run' && code === OFFLINE_ACTION_BLOCKED && message === 'OFFLINE_ACTION_BLOCKED:run' && !(error instanceof ResearchError)` 拒绝；② `dispatched === 0`（判决在派发之前，零后端派发）；③ 在线 gate 放行照常派发；④ gate 缺省直通（既有行为回归钉死）。

- RED（修复前实跑）：
  ```
  $ pnpm exec tsx --test packages/mobile-core/src/research/task-research.test.ts
  not ok 5 - requestRevision is a Run command: offline gate rejects before any dispatch, online passes through
    error: 'Missing expected rejection.'   ← 离线请求Revision未拒绝，命令被真实派发
  ```
- GREEN（修复后实跑）：同命令 6/6 pass。

**视图级** `apps/mobile/src/research-view.test.ts` 新增测试「controller requestRevision surfaces the structured offline rejection with honest copy」：stub 句柄 `requestRevision` 上抛真 `OfflineGateError('run')`——断言控制器 error 恰为 `RESEARCH_OFFLINE_REVISION_COPY` 且 `notEqual RESEARCH_ERROR_COPY.RESEARCH_BACKEND`；在线路径 notice 仍为「修订请求已提交」。

- RED（修复前实跑）：
  ```
  not ok 4 - controller requestRevision surfaces the structured offline rejection with honest copy
    + actual   'OFFLINE_ACTION_BLOCKED:run'   ← messageOf 无 OfflineGateError 分支，裸码直出
    - expected undefined                       ← RESEARCH_OFFLINE_REVISION_COPY 尚未导出
  ```
- GREEN（修复后实跑）：同命令 4/4 pass。

---

## 全量回归证据（全部本次会话实跑）

| 命令 | 结果 |
| --- | --- |
| `pnpm exec tsx --test packages/mobile-core/src/research/task-research.test.ts` | 6/6 pass（修复前同命令 1 fail = RED） |
| `pnpm --filter @weknora/mobile exec tsx --test src/research-view.test.ts` | 4/4 pass（修复前同命令 2 fail = RED） |
| `pnpm exec tsx --test packages/contracts/src/mobile/research.test.ts packages/api-client/src/mobile/research.test.ts packages/mobile-core/src/research/task-research.test.ts`（计划门 TS 链） | 14 pass / 0 fail |
| `pnpm --filter @weknora/mobile exec tsx --test src/research-view.test.ts src/research-integration-smoke.test.ts`（计划门） | 7 pass + 1 SKIP（opt-in 真实部署凭据缺席的诚实跳过，既有语义）/ 0 fail |
| `pnpm --filter @weknora/mobile typecheck`（计划门） | 通过（无输出） |
| `pnpm --filter @weknora/mobile exec tsx --test src/materials-view.test.ts`（计划门） | 5/5 pass |
| `pnpm --filter @weknora/mobile test`（apps/mobile **全量**，含真实装载 composition.ts 的 app-smoke） | 234 tests：222 pass / 12 skipped（opt-in 凭据门控）/ **0 fail** |
| `pnpm exec tsx --test $(find packages/mobile-core/src -name "*.test.ts")`（mobile-core **全量** 36 文件） | 330/330 pass / 0 fail |

**未跑项（如实）**：计划门 Go 部分（`go test ./internal/...`、`go build ./...`）未跑——本批次 `git diff --stat` 仅 6 个 TS 文件（apps/mobile 3 + packages/mobile-core 3），零 Go 文件改动，Go 面与本 diff 无交集。

## 变更清单

| 文件 | 变更 |
| --- | --- |
| `apps/mobile/src/composition.ts` | +5：taskResearchFor 注入 `commands: createTaskOfficeRemote({ origin, request: authorizedRequest })`（发现 1） |
| `packages/mobile-core/src/research/task-research.ts` | +6：requestRevision 入口 `assertOnline('run')`（try/catch 之外原样上抛，发现 2） |
| `packages/mobile-core/src/research/types.ts` | 注释修正：gate 端口语义覆盖 requestRevision（发现 2 配套） |
| `apps/mobile/src/research-view.ts` | +8/-1：`RESEARCH_OFFLINE_REVISION_COPY` + `messageOf` OfflineGateError 首分支（发现 2 文案） |
| `packages/mobile-core/src/research/task-research.test.ts` | +57：模块级离线断言回归测试 |
| `apps/mobile/src/research-view.test.ts` | +37/-2：接线源级断言 + 离线文案控制器测试 |
