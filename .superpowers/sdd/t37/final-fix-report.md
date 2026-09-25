# T07 #37 最终修复报告（final-fix round）

Worktree：`.worktrees/issue30-sweep-t37`（branch `codex/issue30-t37`）
日期：2026-09-25
范围：整计划最终审查的 2 项发现（1 important + 1 minor），全部一次修复，每项附 RED→GREEN 回归测试证据。

---

## 发现 1（important）：GormRunRestartPort.Restart 对未知 schema 快照零值放行

### 问题确认（依据本 session 读取的代码）

- `internal/modules/workbench/service/workbench/command_queue_next.go:87-90`（修复前）：`json.Unmarshal(row.Snapshot, &parent)` 失败才 `ErrConflict`。
- `internal/application/service/agent_run_graph.go:53-61`：graph/tRPC Run 的 `DurableRunSnapshot` 只有 `version/query/image_urls/model_id/rerank_model_id/agent_config/runtime` 键，**无 `agent_id` 键**——unmarshal 到 restart 的 parent struct（`agent_id/target_id/workspace_ref/space_id/budget_upper`）会**零值成功**（json 无法区分「键缺失」与「空值」）。
- `internal/modules/workbench/service/workbench/admission.go:286-288`：`AdmissionCoordinator.Start` 只校验 `SessionID/RequestID/Text/BudgetUpper`，**不校验 `AgentID`**。
- 因此同 owner 的终态 chat Run（snapshot 已被引擎改写为 DurableRunSnapshot 形态）在 revision 对齐、会话槽释放后，`queue_next` 会准入一个 `agent_id` 为空的垃圾后续 Run 并占用真实会话槽；`ListOwnedExecutions` 仅按 owner 过滤，垃圾 Run 已可触达 UI。违反 `docs/specs/2026-09-20-mobile-module-seams.md` §5.3「capability 缺失和未知 schema 一律 fail closed」。

### 修复

`internal/modules/workbench/service/workbench/command_queue_next.go`：unmarshal 成功后追加守卫——`parent.AgentID` TrimSpace 为空即 `agentruntime.ErrConflict`，附英文注释说明「unmarshal 成功证明不了这是 coordinator 准入的运行」。

### 覆盖测试（RED→GREEN 均在本 session 实际运行）

新增 `internal/modules/workbench/service/workbench/command_queue_next_test.go`：

- `TestQueueNextOnDurableRunSnapshotShapeFailsClosed`：真实协调器创建父 Run 后，把行改写为 `{"version":1,"query":"chat goal","model_id":"m1","agent_config":{},"runtime":{}}`（无 agent_id 键）、置终态、释放会话槽——queue_next 全部前置门放行，唯独快照形态非准入映射。期望 `ErrConflict` 且 `agent_runs` 行数不变（不占会话槽）。
- `TestQueueNextOnExplicitEmptyAgentIDSnapshotFailsClosed`：显式 `"agent_id":""` 的同守卫另一路径，同样拒绝。

证据（实际命令与输出）：

- RED（临时用 Edit 移除守卫后运行）：
  `go test ./internal/modules/workbench/service/workbench/ -run 'TestQueueNextOnDurableRunSnapshotShapeFailsClosed|TestQueueNextOnExplicitEmptyAgentIDSnapshotFailsClosed'`
  → `--- FAIL: TestQueueNextOnDurableRunSnapshotShapeFailsClosed ... Expected error with "agent runtime conflict" in chain but got nil.`（两条新用例均 FAIL——证明缺陷真实存在且测试能抓住它）
- GREEN（恢复守卫后）：
  `go test ./internal/modules/workbench/service/workbench/ -run 'TestQueueNext'`
  → `ok  github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench 23.715s`（含既有 7 个 queue_next 用例全部通过，确认守卫不误伤合法准入路径）

### 影响面说明

守卫只拦截「快照里没有非空 agent_id」的重启请求；coordinator 自己准入的运行（admission map 形态）必然带非空 `agent_id`（`Start` 的调用方均传 `AgentID`），既有 `TestQueueNextOnTerminalRunAdmitsFollowUpOnSameSession`、`TestQueueNextMatchesTheAdmissionOwnerProjection` 等 PASS 证明合法路径不受影响。

---

## 发现 2（minor）：停止卡 UI 硬编码「停止已确认：运行已取消」丢弃 view.stop.note

### 问题确认（依据本 session 读取的代码）

- `packages/mobile-core/src/task-office/task-detail.ts:157-158`：`stopProjection` 在停止受理后观察到**非 canceled** 终态时返回 `confirmed + note: "run ended as ${runStatus} before the stop landed"`——那次取消并未发生。
- `apps/mobile/src/screens/TaskDetailScreen.tsx:73`（修复前）：`{STOP_PHASE_COPY[view.stop.phase]}` 只按 phase 渲染，届时显示「停止已确认：运行已取消」——谎报一次未发生的取消（Spec Story 23 要防的隐藏后果）。

### 修复

`apps/mobile/src/screens/TaskDetailScreen.tsx`：新增 `stopCardCopy(stop)`——带 note 的 `confirmed` 用中性陈述「停止流程已结束：{note}」（note 由模块给出，事实性文字）；无 note 的 `confirmed` 及 `requested`/`unknown` 分支保持 `STOP_PHASE_COPY` 原文案。渲染点改为 `stopCardCopy(view.stop)`。即审查给的第二种修法（「对带 note 的 confirmed 用中性文案」），未在 Screen 层解析模块内部字符串（不与 `reconciled: canceled` 等魔法串耦合）。

### 覆盖测试（RED→GREEN 均在本 session 实际运行）

新增 `apps/mobile/src/app-smoke.test.tsx` 用例 `the stop card renders the module note instead of claiming a cancellation that never happened (T07 #37 终审修复)`：confirmed+note 渲染中性文案 + note 且**不含**「停止已确认：运行已取消」；无 note 的 confirmed 仍明确确认；requested 分支不受影响。

证据（实际命令与输出）：

- RED（临时用 Edit 把渲染改回 `STOP_PHASE_COPY[view.stop.phase]`）：
  `npx tsx --test src/app-smoke.test.tsx`（cwd apps/mobile）
  → `✖ the stop card renders the module note ... ℹ fail 1`
- GREEN（恢复 `stopCardCopy`）：
  `npx tsx --test src/app-smoke.test.tsx`（cwd apps/mobile）
  → `ℹ tests 55 ℹ pass 55 ℹ fail 0`

---

## 全量回归（实际运行）

| 检查 | 命令（cwd） | 结果 |
| --- | --- | --- |
| Go 全量构建 | `go build ./...`（worktree 根） | 通过（仅既有 ld duplicate-libraries 警告） |
| Go vet | `go vet ./internal/modules/workbench/...` | 通过 |
| workbench 包全量测试 | `go test ./internal/modules/workbench/service/workbench/` | `ok ... 42.305s` |
| mobile 全部测试 | `npm test`（apps/mobile，即 `tsx --test 'src/**/*.test.ts*'`） | 160 tests / 154 pass / 6 skipped / **0 fail**（6 个 skip 均为原有 opt-in live smoke：missing credentials skip，非本次改动引入） |
| mobile typecheck | `npm run typecheck`（apps/mobile） | 通过（exit=0） |
| mobile-core 全部测试 | `npx tsx --test "packages/mobile-core/src/**/*.test.ts"`（worktree 根） | 249 tests / 249 pass / 0 fail（含 task-detail stop 投影既有测试） |

说明：`packages/mobile-core` 自身无 test script（其测试由上述 tsx 直跑命令覆盖）；未运行 web/desktop 测试套件（本次改动不触及其代码）。

## 变更文件

- `internal/modules/workbench/service/workbench/command_queue_next.go`（守卫）
- `internal/modules/workbench/service/workbench/command_queue_next_test.go`（+2 用例）
- `apps/mobile/src/screens/TaskDetailScreen.tsx`（stopCardCopy + 渲染点）
- `apps/mobile/src/app-smoke.test.tsx`（+1 用例）
