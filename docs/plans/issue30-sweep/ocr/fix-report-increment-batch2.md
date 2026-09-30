# OCR 增量修复批次 2（issue30-sweep）——修复执行报告

- 执行计划：`docs/plans/issue30-sweep/plans/ocr-fix-increment-batch2.md`
- 发现来源：`docs/plans/issue30-sweep/ocr/ocr-increment-batch2.md`（15 项已验证发现 + 24 项 lowWorth）
- 工作分支：`codex/issue30-mobile-office`（worktree `.worktrees/issue30-sweep`）
- 执行方式：严格 RED → GREEN → COMMIT，每个任务先写失败测试、实跑确认失败、最小实现、实跑通过后提交，共 10 个提交。
- 结果：**10/10 任务完成，附录 B 四条验收命令全部实跑通过。**

## 提交清单

| # | 提交 | 任务 | 发现 |
|---|---|---|---|
| 1 | `3445425d3` fix(workbench): read task facts by business owner (owner_id) instead of lease owner | Task 1 | B2-F12（P0/critical） |
| 2 | `ea94112bb` fix(mobile-core): contain deployment-registry failures inside runtime transitions | Task 2 | B2-F17/F42/F32(runtime)；顺手 F43/F44/F22/F36 |
| 3 | `aea1be8d4` fix(mobile): deployment registry skips malformed entries and caps stored size | Task 3 | B2-F31/F32(adapter)；顺手 F21 |
| 4 | `159e82bee` fix(mobile): deployment list effect gains containment, latest-wins and narrowed deps | Task 4 | B2-F33 |
| 5 | `f41f9d36d` fix(mobile): reset login form onto the switch target and gate switch entries on handler presence | Task 5 | B2-F10；顺手 F2/F3 |
| 6 | `f23779bce` fix(mobile): honest task-detail error copy with in-place retry and per-run expansion | Task 6 | B2-F13/F38；顺手 F16/F5/F39/F40/F41 |
| 7 | `15f2e5fc8` fix(mobile-core): guard task-detail hydrate and stream callbacks with stream epochs | Task 7 | B2-F24；顺手 F26/F27 |
| 8 | `afb05d8d0` fix(mobile): sse adapter throws real ApiError and always releases the reader | Task 8 | B2-F29/F30；顺手 F8/F37 |
| 9 | `095ddc9d8` test(mobile): make task-detail integration smoke total with consistent dispose and host guard | Task 9 | B2-F14；顺手 F15 |
| 10 | `3e2c17e51` fix(mobile-core): distinguish an unavailable stream channel from unauthorized and stop futile resyncs | Task 10 | B2-F34；顺手 F19/F20/F45 |

总 diff：29 个文件，+700/−105 行（`git diff --stat bbe4cf7ec..HEAD`）。

---

## Task 1：GetWorkbenchSnapshot 属主字段错配（B2-F12，P0）

**修复**：`internal/handler/session/workbench_read.go:181` 由 `run.Owner`（= `agent_runs.lease_owner`，已 settle 的 run 为空串、运行中为 worker id，两种情况都永不匹配 facts 守卫的 `owner_id` 绑定）改为 `run.UserID`（= `agent_runs.owner_id`，业务属主）。夹具强化：`stubTaskFactsReader` 记录 `seenOwnerIDs`（原实现完全忽略入参，属主错配因此漏检），run 夹具区分 `UserID: "u1"` 与 `Owner: "worker-1"`。

**RED 实跑**（`go test ./internal/handler/session/ -run TestGetWorkbenchSnapshotFactsUseBusinessOwner -v`）：

```
Error:      	Not equal:
            	expected: []string{"u1"}
            	actual  : []string{"worker-1"}
--- FAIL: TestGetWorkbenchSnapshotFactsUseBusinessOwner (0.00s)
```

**GREEN 实跑**：

```
$ go test ./internal/handler/session/ -run TestGetWorkbenchSnapshot -v
--- PASS: TestGetWorkbenchSnapshotCarriesTaskFacts (0.00s)
--- PASS: TestGetWorkbenchSnapshotOmitsTaskSectionWithoutFactsReader (0.00s)
--- PASS: TestGetWorkbenchSnapshotSurfacesFactsReadFailure (0.00s)
--- PASS: TestGetWorkbenchSnapshotFactsReadIsOwnerScoped (0.00s)
--- PASS: TestGetWorkbenchSnapshotFactsUseBusinessOwner (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/handler/session	1.144s
$ go test ./internal/handler/session/ ./internal/application/repository/
ok  	github.com/Tencent/WeKnora/internal/handler/session	18.867s
ok  	github.com/Tencent/WeKnora/internal/application/repository	169.848s
```

用户可见症状：带 task facts 接线的部署上 snapshot 端点统一 404 → 已 settle 与运行中 run 均返回 200（含 task 段）。

## Task 2：Runtime 对 registry 交互的失败包含（B2-F17/F42/F32 runtime 侧）

**修复**（`packages/mobile-core/src/runtime/mobile-runtime.ts`）：
- `listDeployments()`：registry 读取失败按 fail-closed 返回 `[]`，从不 reject（B2-F42）。
- `switchDeployment`：同 origin 且不在登录面时短路返回当前 state（B2-F44）；`registry.list()` 失败按未登记处理、保持当前面（B2-F17）。
- `authenticate` 内 `registry.upsert` 失败单独吞掉：registry 是 presentation 辅助数据，写失败不得把已验证的授权拖入外层 catch 而呈现 `authentication-required`（B2-F32 runtime 侧）。
- `forgetDeployment` 全身 try/catch：持久化失败 resolve 而非 reject（B2-F43）。

**RED 实跑**（`pnpm exec tsx --test packages/mobile-core/src/runtime/runtime-deployments.test.ts`）：`# tests 14 / # pass 9 / # fail 5`（新增 5 个用例全部失败）。

**GREEN 实跑**：

```
$ pnpm exec tsx --test packages/mobile-core/src/runtime/runtime-deployments.test.ts
# tests 14 / # pass 14 / # fail 0
$ pnpm exec tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts packages/mobile-core/src/runtime/runtime-readonly.test.ts
# tests 42 / # pass 42 / # fail 0
```

用户可见症状：SecureStore 读/写失败不再把已授权用户降级为未授权，切换实例不再 reject。

## Task 3：SecureStore 注册表解析与容量策略（B2-F31/F32 adapter 侧；顺手 F21）

**修复**（`apps/mobile/src/adapters/deployment-registry.ts`）：`parseRecords` 不再「单条畸形 → 整体 `undefined`」，改为单条跳过、其余保留；非数组/JSON 破损 → `[]`；新增 `MAX_REGISTRY_ENTRIES = 8`，upsert 前移后截断（8 条 origin/label 远小于 Android SecureStore 单值约 2KB 预算）。`packages/mobile-core/src/runtime/in-memory-adapters.ts` 的 upsert 同步 label 归一化（trim + origin 兜底，B2-F21）。

**RED 实跑**：`deployment-registry.test.ts` → `# tests 8 / # pass 5 / # fail 3`（计划预期的「前 3 个」失败；第 4 个 non-array 用例本就通过）；`runtime-deployments.test.ts` → `# fail 1`（B2-F21）。

**GREEN 实跑**：

```
$ pnpm exec tsx --test apps/mobile/src/adapters/deployment-registry.test.ts
# tests 8 / # pass 8 / # fail 0
$ pnpm exec tsx --test packages/mobile-core/src/runtime/runtime-deployments.test.ts apps/mobile/src/app-smoke.test.tsx
# tests 39 / # pass 39 / # fail 0
```

用户可见症状：单条脏注册数据不再静默清空全部已注册 Deployment；注册表持久化有界。

## Task 4：部署列表 effect 防护（B2-F33）

**修复**（`apps/mobile/src/composition.ts`）：新增导出 `createDeploymentListSync(runtime, setDeployments)`——递增请求序号 latest-wins（迟到的旧读取被丢弃）+ 失败包含（维持现状、不形成 unhandled rejection）；`MobileApp` 的 effect 依赖收窄为 `[activeRuntime, snapshot.surface, snapshot.deployment?.origin]`（tenant 切换等发布不再重复读安全存储）。

**RED 实跑**（`pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx`）：`not ok 25/26/27`（两个行为级 + 一个源级断言失败；`createDeploymentListSync` 未导出；typecheck 测试同步失败因测试文件引用了尚未导出的符号——RED 状态的预期表现）。

**GREEN 实跑**：`# tests 27 / # pass 27 / # fail 0`（含 typecheck 测试恢复）。

## Task 5：切换实例后的表单状态重置（B2-F10；顺手 F2/F3）

**修复**：`DeploymentLoginScreen.tsx` 切换按钮的 `switchTo` 先重置本地表单（`setOrigin(目标)`、`setEmail('')`、`setPassword('')`、`setError(undefined)`）再调用 `onSwitchDeployment`——无凭据目标实例回到 `deployment-login` 面时组件复用而表单已指向目标；「Registered deployments」区块与 HomeScreen 的「Switch to …」按钮均以 handler 存在为渲染前提（缺失时不渲染死交互按钮）。

**RED 实跑**：`not ok 28/29/30`（切换后 origin 仍为 stale；无 handler 时区块/按钮仍渲染）。

**GREEN 实跑**：`# tests 30 / # pass 30 / # fail 0`。

用户可见症状：切换目标实例无凭据时，Sign in 不再登录旧 origin。

## Task 6：任务详情错误语义与就地重试（B2-F13/F38；顺手 F16/F5/F39/F40/F41）

**修复**：
- `apps/mobile/src/task-detail-view.ts`：导出 `TASK_OFFICE_ERROR_COPY`（错误码 → 中文文案），`messageOf` 对 `TaskOfficeError` 先查映射（B2-F16）。
- `apps/mobile/src/app/tasks/detail.tsx`：路由 catch 按错误码分流——`TASK_OFFICE_INVALID_INPUT`（缺参）/`TASK_OFFICE_DETAIL_UNAVAILABLE`（端口缺失）不再折叠为「请先登录」，仅未知异常保留登录引导兜底（B2-F13）。
- `apps/mobile/src/screens/TaskDetailScreen.tsx`：`view === undefined` 错误分支渲染「重试」按钮调用 `onRefresh`（controller.refresh 支持从无 view 状态恢复，B2-F38）；「重新同步快照」随 loading 禁用（B2-F39）；`INTERRUPTION_COPY` 映射替代内部码直出（B2-F40）；`expanded` 以 runId 隔离、切换任务时重置（B2-F41）。
- `apps/mobile/src/screens/TasksScreen.tsx:81`：「详情」→ `Details`（B2-F5）。

**RED 实跑**：`not ok 31/32/33/35` + typecheck（引用未导出的 `TASK_OFFICE_ERROR_COPY`）。

**GREEN 实跑**：

```
$ pnpm exec tsx --test apps/mobile/src/task-detail-view.test.ts apps/mobile/src/app-smoke.test.tsx
# tests 35 / # pass 35 / # fail 0
```

**执行注记（测试修正，非实现偏离）**：行为级用例初版在 `hydrate()` 内动态 `import('@weknora/mobile-core')` 构造 `TaskOfficeError`，`instanceof` 判定失败（apps/mobile 由 tsx 按 CJS 转译，动态 import 加载的是 ESM 入口，与被测模块 `require` 得到的类是两个实例）。改为与被测模块一致的顶层静态导入后通过——实现本身无需改动。

用户可见症状：瞬时网络故障后任务详情有就地重试入口；错误文案不再直出内部码；已登录用户缺参/无详情通道时不再被误导为「请先登录」。

## Task 7：hydrate 的 epoch/串行化守卫（B2-F24；顺手 F26/F27）

**修复**（`packages/mobile-core/src/task-office/task-detail.ts`）：
- `hydrate` 入口 `abortStream()` 递增 `streamEpoch`，并在每个 await 挂起点（detail 返回、store.load 返回、persist 完成）后校验 `epoch === streamEpoch`：被取代的旧 hydrate 抛 `TASK_OFFICE_SCOPE_CHANGED`（从未发布视图时）或返回当前视图，不再以裁剪后事件集覆写 store、不回退 `committedCursor`。
- `startStream` 的 `onEvent`/`onControl` 在 enqueue 前校验自身 epoch：被取代流的迟到事件/控制帧直接丢弃。
- `duplicateSeqs` 追加截断至最近 50 条（B2-F27）；`createTaskDetail` 头部补 `input.taskId` 契约注释（persist 以服务端权威 `detail.taskId` 为准，B2-F26）。

**RED 实跑**（`pnpm exec tsx --test packages/mobile-core/src/task-office/task-detail.test.ts`）：

```
not ok 17 - a slower stale hydrate does not roll back the committed cursor or the event set
  迟到的旧 hydrate 不得回退 committedCursor / 2 !== 4
not ok 18 - events from a superseded stream are dropped after resync
  迟到事件不得触发 gap interrupt / true !== false
# tests 18 / # pass 16 / # fail 2
```

**GREEN 实跑**：

```
$ pnpm exec tsx --test packages/mobile-core/src/task-office/task-detail.test.ts
# tests 18 / # pass 18 / # fail 0
```

**执行注记（测试夹具调整）**：计划中测试 2 原样使用 `backend.streams[0]!.emit(...)` 在 resync 后投递旧流事件，但 `createScriptedTaskStream` 的 `emit` 在 abort 后是 no-op（`in-memory-task-detail.ts:36` 的 `!sink.signal.aborted` 守卫），该写法在修复前的代码上也不会失败（非有效 RED）。为构造真实竞态，测试改为自建 detail 端口直接持有每条流的 `onEvent`/`onControl` 回调——模拟「reader 已缓冲的 chunk 在 cancel 后才送达」的传输层，并断言不产生任何 gap/cursor-expired interruption。被测契约与计划一致，仅夹具形式更强。

**回归注记**：计划 Step 4 的 `pnpm exec tsx --test packages/mobile-core/src/task-office/`（目录形式）在本仓库不可用（无 `index.ts`，ERR_MODULE_NOT_FOUND）。改用显式文件枚举等价执行同包全部 4 个测试文件：`# tests 32 / # pass 32 / # fail 0`。

## Task 8：SSE 适配层错误形态与读取循环清理（B2-F29/F30；顺手 F8/F37）

**修复**：
- `apps/mobile/src/adapters/sse-stream.ts`：非 2xx 改抛真 `ApiError`（`{ status, code: `HTTP_${status}`, message }`，code 兜底与 `errorFromResult` 约定一致）；读取循环以 `try/finally` 包裹，`finally` 中 `reader.cancel()`——`onChunk` 同步抛出（如 runtime `guardedChunk` 的 `RUNTIME_SCOPE_CHANGED`）或传输中断都释放连接，正常完成时 cancel 幂等。
- `packages/api-client/src/mobile/task-office.ts`：流回调的 `JSON.parse` 加守卫，畸形帧抛 `TASK_STREAM_MALFORMED_FRAME: <event>` 的 transport error（进入 `streamFailed → interrupted`，不再裸抛 `SyntaxError`，B2-F8）；守卫只包 `JSON.parse`，消费方回调（RUNTIME_SCOPE_CHANGED 等）与 `parseExecutionEvent` 的语义错误照常向上传播。`'TASK_STREAM_CURSOR_EXPIRED'` 处补跨包契约码注释（B2-F37，不改字符串本身）。

**RED 实跑**（`pnpm exec tsx --test apps/mobile/src/adapters/sse-stream.test.ts packages/api-client/src/mobile/task-office.test.ts`）：`not ok 3/4/13`（`instanceof ApiError` 为 false / `cancelled` 为空 / 畸形帧抛裸 SyntaxError）。

**GREEN 实跑**：

```
$ pnpm exec tsx --test apps/mobile/src/adapters/sse-stream.test.ts packages/api-client/src/mobile/task-office.test.ts
# tests 13 / # pass 13 / # fail 0
$ pnpm exec tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts packages/api-client/src/mobile/task-office-detail.integration.test.ts
# tests 39 / # pass 38 / # fail 0 / # skipped 1
```

（skipped 1 为 opt-in 真实 HTTP 集成用例按既有语义 skip，非伪造通过。）

## Task 9：集成 smoke 收口（B2-F14；顺手 F15）

**修复**：
- `apps/mobile/src/runtime-integration-smoke.ts:111`：`disallowedDeploymentHost` 加 `export`（函数体不动）。
- `apps/mobile/src/task-detail-integration-smoke.ts`：`taskDetailIntegrationConfig` 在 HTTPS origin 校验后复用主机防线（localhost/环回/私网/链路本地/保留地址 → `invalid`，B2-F15）；`runTaskDetailIntegration` 收口为 total——`handle` 提升到 try 外声明，所有路径（含 no-tasks/未授权早退/任一步骤异常）经同一 `finally` 执行 `handle?.close(...)` 与 `runtime.dispose()`；异常路径 `opened:'failed'` + 新增可选字段 `failure?: string`（仅 error message，证据契约仍无凭据字段），从不 reject。调用方 `task-office-detail.integration.test.ts` 补 total 契约注释。

**RED 实跑**（新建 `apps/mobile/src/task-detail-integration-smoke.test.ts`）：`not ok 1`（localhost 当前判 `enabled:true`——无主机防线）、`not ok 3`（源级断言：无 finally 收口/无 failure 字段/未引用 disallowedDeploymentHost）；`ok 2`（公网 origin 判 enabled——既有行为保持）。

**GREEN 实跑**：

```
$ pnpm exec tsx --test apps/mobile/src/task-detail-integration-smoke.test.ts packages/api-client/src/mobile/task-office-detail.integration.test.ts
# tests 5 / # pass 4 / # fail 0 / # skipped 1   （skipped 为 opt-in 真实 HTTP 用例）
$ pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx
# tests 33 / # pass 33 / # fail 0
```

## Task 10：授权流通道不可用的诚实语义（B2-F34；顺手 F19/F20/F45）

**修复**：
- `packages/mobile-core/src/runtime/mobile-runtime.ts` `authorizedEventStream`：未授权抛 `RUNTIME_UNAUTHORIZED`；已授权但 `authorizedStream` 工厂返回 `undefined`（fail closed）抛 `RUNTIME_STREAM_UNAVAILABLE`——REST 仍可用（修复落点在 runtime 而非计划报告所指 `composition.ts:106`，与计划「差异记录 2」一致）。
- `packages/mobile-core/src/task-office/task-detail.ts`：`TaskInterruptionReason` 新增 `'stream-unavailable'`；`streamFailed` 对 `RUNTIME_STREAM_UNAVAILABLE` 置 interrupted 并 `autoResyncs = AUTO_RESYNC_LIMIT`（通道缺失非瞬时故障，跳过 2 次徒劳自动 resync；显式 `resync()` 重置计数仍可再试）。
- `packages/mobile-core/src/runtime/ports.ts`：`AuthorizedStreamTransport` 契约注释补第三种情形（B2-F20）；`types.ts` `resourceShelf()` 注释对齐 authenticate 的 authorized/read-only 双面行为（B2-F45，注释级）。
- `apps/mobile/src/screens/TaskDetailScreen.tsx`：`INTERRUPTION_COPY` 追加 `'stream-unavailable'` 条目（Task 6 建立的映射）。

**RED 实跑**（`pnpm exec tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts packages/mobile-core/src/task-office/task-detail.test.ts`）：`not ok 38`（当前 reject `RUNTIME_UNAUTHORIZED`）、`not ok 57`（当前 reason `'stream-error'` 且 `streams.length === 3`、`detailCalls === 3`——初始流 + 2 次徒劳自动 resync）。

**GREEN 实跑**：

```
$ pnpm exec tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts packages/mobile-core/src/task-office/task-detail.test.ts packages/mobile-core/src/runtime/runtime-deployments.test.ts apps/mobile/src/app-smoke.test.tsx
# tests 105 / # pass 105 / # fail 0
```

用户可见症状：平台无流式 fetch 时不再误导为「未授权」，且不再触发 2 次徒劳的 REST 重同步。

---

## 附录 B 批次验收（全部实跑）

```
$ go test ./internal/handler/session/ ./internal/application/repository/
ok  	github.com/Tencent/WeKnora/internal/handler/session	12.379s
ok  	github.com/Tencent/WeKnora/internal/application/repository	(cached)

$ pnpm exec tsx --test packages/mobile-core/src/runtime/runtime-deployments.test.ts packages/mobile-core/src/runtime/mobile-runtime.test.ts packages/mobile-core/src/runtime/runtime-readonly.test.ts packages/mobile-core/src/task-office/task-detail.test.ts packages/mobile-core/src/task-office/task-office.test.ts
# tests 84 / # pass 84 / # fail 0

$ pnpm exec tsx --test apps/mobile/src/adapters/deployment-registry.test.ts apps/mobile/src/adapters/sse-stream.test.ts apps/mobile/src/task-detail-view.test.ts apps/mobile/src/task-detail-integration-smoke.test.ts apps/mobile/src/app-smoke.test.tsx
# tests 50 / # pass 50 / # fail 0

$ pnpm exec tsx --test packages/api-client/src/mobile/task-office.test.ts packages/api-client/src/mobile/task-office-detail.integration.test.ts
# tests 11 / # pass 10 / # fail 0 / # skipped 1
```

第 4 条中 skipped 1 为 opt-in 真实 HTTP 用例（无 `WEKNORA_MOBILE_TEST_*` 环境变量时按既有语义 skip），未伪造通过。`app-smoke` 内含 `pnpm --filter @weknora/mobile typecheck`（tsc --noEmit）实跑且通过。

抽查清单（附录 B 第 5 条，逐条对上）：

| 发现 | 用户可见症状 | 对应证据 |
|---|---|---|
| B2-F12 | snapshot 404 → 200 | `TestGetWorkbenchSnapshotFactsUseBusinessOwner` 断言 200 + `seenOwnerIDs == ["u1"]` |
| B2-F10/F17 | 切换不残留旧 origin / 不 reject | Task 5 重渲染断言 origin 指向目标；Task 2 `switchDeployment` 失败包含断言 |
| B2-F31 | 脏数据不清空注册表 | `list skips malformed entries...` + `upsert over a corrupt store recovers...` |
| B2-F32 | 登录不被注册表写失败降级 | `an authorized sign-in survives a registry write failure`（surface 断言 authorized） |
| B2-F33 | 列表 effect 失败包含/乱序防护 | `the deployment list sync ignores out-of-order completions` / `swallows read failures` |
| B2-F34 | 无流通道不误导为未授权、不徒劳 resync | Task 10 两用例（错误分流 + `streams.length === 1`） |
| B2-F38 | 错误分支有重试按钮 | `the no-view error branch keeps an in-place retry entry` |

## 偏离与执行注记汇总

1. **Task 6**：`TaskOfficeError` 行为级用例的动态 import 在 apps/mobile 的 CJS 转译下产生双类实例，改为顶层静态导入（测试修正，实现不变）。
2. **Task 7**：计划测试 2 的 `backend.streams[0].emit` 写法因 scripted 夹具 abort 后 no-op 而无法构成 RED，改为自建 detail 端口直接持有流回调以构造真实竞态（被测契约不变，夹具更强）。
3. **Task 7 Step 4**：`pnpm exec tsx --test packages/mobile-core/src/task-office/`（目录形式）在本仓库不可用（无 index.ts），改用显式枚举同包 4 个测试文件等价执行（32/32 通过）。
4. **Task 9 RED 范围**：计划预期「localhost 目前判 enabled:true」与实跑一致；non-array/公网 origin 用例为既有行为钉子，非 RED 项。

## 明确延期项（按计划附录 A，未实现）

B2-F23（生产持久化任务投影存储——SecureStore 2KB 与 200 事件投影体积冲突，需存储选型 ADR）、B2-F4、B2-F6、B2-F11、B2-F28、B2-F35、B2-F46——理由与判定见计划附录 A，本批次未触碰。
