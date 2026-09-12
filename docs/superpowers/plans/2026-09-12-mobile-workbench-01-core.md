# 移动工作台 01：共享契约与服务端执行 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 提供有租户与所有权约束、可重放、可幂等受理的统一 Run API。

**Architecture:** 扩展现有 agent_runs，不创建第二张移动任务权威表。新 facade 复用产品认证、会话、审批与预算服务，原 tRPC/旧聊天端点保持兼容。

**Tech Stack:** Go/Gin/GORM、PostgreSQL/SQLite、TypeScript、tsx/node:test。

**Spec:** [技术架构](../specs/2026-09-12-mobile-ai-saas-workbench-architecture.md) §5、§7–11；[总计划](2026-09-12-mobile-ai-saas-workbench.md)。执行者同时读取本册与规格。

## Global Constraints

- “采用 **Happy 移动底座 + WeKnora 产品与执行控制 + Paseo 远程编码执行接入 + AWS 示例多模态交互复用**。”
- “分批交付不取消原 Happy 交互保留要求，未交付能力继续保留在清单中。”
- “任何 Run、事件、附件、审批都通过服务端认证上下文和持久所有权解析空间。”
- “同一请求 ID 与相同参数重放返回原 Run；同 ID 不同参数返回冲突。”
- “取消、失败和超时不抹掉实际用量，也不意味着外部副作用已回滚。”
- “预算授权、工具操作批准和连接授权是三件独立的事。”
- “子 Run 共享父任务预算树，不能复制一份可消费余额；新增子执行仍需授权和准入。”
- 保持 Expo 55 / React Native 0.83.1 / React 19.2.0 与当前 pnpm 10.28.2 工作区；执行前验证锁文件。Go 1.26.0；SQLite 与 PostgreSQL 分别验收。
- 本册所有新增接口和文件是实施目标；缺少环境记 `blocked-env`，不能把静态代码、fixture 或测试跳过写为真实运行验收。
- 实施前创建隔离工作区并带入未提交规格；保留并行任务修改。只提交本任务文件；不得 `git add .`、擅自发布或改写旧台账通过状态。

---

## 文件与责任边界

`internal/workbench` 保存纯 DTO 和控制契约；`internal/application/repository/agent_run_*` 负责持久化；`internal/handler/session/workbench_*` 复用会话所有权入口；`packages/*/src/mobile` 提供跨端契约。修改旧文件只在类型、构造和路由接缝，不整体搬迁。迁移序号在总计划登记后分配，禁止覆盖并行迁移。

### W01：统一 DTO、事件和能力合同

**依赖：** 无。

**Files：**

- Create: `packages/contracts/src/mobile/execution.ts`、`packages/contracts/test/mobile-execution.test.ts`
- Create: `internal/workbench/contracts.go`、`internal/workbench/contracts_test.go`
- Modify: `packages/contracts/src/index.ts`（导出）、`package.json`（共享测试入口）

**Interfaces：**

Produces（字段为跨语言 wire 权威；Go 使用同名 JSON tag）：
```ts
export interface Capability { state: 'supported'|'unavailable'|'forbidden'; reason: string }
export interface ExecutionDTO {
  schema_version: 1; run_id: string; session_id: string; revision: number;
  driver: 'platform'|'paseo'; run_status: string; execution_status: string;
  settlement_status: string; seq: number; capabilities: Record<string, Capability>;
}
export interface ExecutionEvent {
  schema_version: 1; run_id: string; attempt_id: string; seq: number;
  type: string; occurred_at: string; payload: Record<string, unknown>;
}
export interface ExecutionSnapshot { execution: ExecutionDTO; watermark: number; events: ExecutionEvent[] }
export function parseExecution(value: unknown): ExecutionDTO;
export function parseExecutionEvent(value: unknown): ExecutionEvent;
```
`run_status` 初版允许 queued/running/waiting_user/reconciling/succeeded/failed/canceled；未知状态拒绝用于控制命令。事件未知 type 允许安全展示。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { parseExecutionEvent } from '../src/mobile/execution.ts';
test('reject malformed sequence and retain unknown event type', () => {
  const event = {schema_version:1,run_id:'r',attempt_id:'a',seq:1,type:'future.event',occurred_at:'2026-09-12T00:00:00Z',payload:{}};
  assert.equal(parseExecutionEvent(event).type, 'future.event');
  for (const seq of [-1, 1.5, Number.MAX_SAFE_INTEGER + 1, '1']) {
    assert.throws(() => parseExecutionEvent({...event,seq}), /INVALID_EVENT/);
  }
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test packages/contracts/test/mobile-execution.test.ts
```

预期：模块或 parser 不存在；补齐导入后应仅因非法序号未拒绝而失败。

- [ ] **Step 3：实现最小行为。**

实现显式 object、字符串、枚举与整数检查，校验每个字段，不以类型断言代替运行校验。事件允许 seq ≥ 1，快照 watermark 和 DTO.seq 允许 0。
```ts
function validSeq(value: unknown, minimum: number): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= minimum;
}
function record(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
```
Go 对 int64 转客户端数字前检查 ≤ 9007199254740991；超限返回 `sequence_overflow`，不可截断。

- [ ] **Step 4：接通实际入口。**

在包入口导出 DTO/parser；根 `test:shared` 显式加入 `packages/contracts/test/mobile-*.test.ts`。Go `ExecutionEvent` 的 Seq 为 int64、Payload 为 json.RawMessage；JSON fixture 用同一字段和时间格式，`go test ./internal/workbench -run TestExecutionWire -count=1` 核对 TS fixture 的往返结果。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 缺 run_id、空 attempt_id、数组 payload、非 ISO 时间、未知 schema_version 均拒绝。
- capability forbidden 与 unavailable 不混淆；空原因只允许 supported。
- 保留原 contracts 导出；`pnpm run test:shared` 回归，不安装新 schema 框架。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add packages/contracts/src/mobile/execution.ts packages/contracts/test/mobile-execution.test.ts packages/contracts/src/index.ts internal/workbench/contracts.go internal/workbench/contracts_test.go package.json
git diff --cached --check
git diff --cached --stat
git commit -m "feat(workbench): define versioned execution contracts"
```

### W02：持久 Run 驱动分流与所有权查询

**依赖：** W01。

**Files：**

- Modify: `internal/agent/runtime/contracts.go`、`internal/application/repository/agent_run.go`、`internal/application/service/agent_run_worker.go`
- Create: `internal/application/repository/agent_run_driver.go`、`internal/application/repository/agent_run_driver_test.go`
- Create: `migrations/versioned/000121_workbench_runs.{up,down}.sql`、`migrations/sqlite/000041_workbench_runs.{up,down}.sql`（执行前核对序号空闲）

**Interfaces：**

Produces：`Run.Driver/TargetID/BudgetRef string`、`Admission.Driver/TargetID/BudgetRef string`；`(*AgentRunStore).ScanDriver(ctx context.Context, driver string, limit int) ([]runtime.RunKey,error)`；`GetOwnedRun(ctx context.Context, tenant uint64, owner, runID string) (runtime.Run,error)`。旧 `Scan(ctx,limit)` 仅查 platform；旧 Admission.Driver 空值映射 platform。Paseo 与 tRPC 的 engine_type 不混写。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```go
package repository
import (
 "context"
 "testing"
 "github.com/stretchr/testify/require"
)
func TestWorkbenchDriverIsolation(t *testing.T) {
 s := NewAgentRunStore(openRunTestDB(t))
 in := testAdmission(); in.Driver = "paseo"
 _, err := s.Admit(context.Background(), in); require.NoError(t, err)
 rows, err := s.Scan(context.Background(), 10); require.NoError(t, err)
 require.Empty(t, rows)
 rows, err = s.ScanDriver(context.Background(), "paseo", 10)
 require.NoError(t, err); require.Len(t, rows, 1)
 _, err = s.GetOwnedRun(context.Background(), 1, "other", "r1")
 require.Error(t, err)
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/application/repository -run TestWorkbenchDriver -count=1
```

预期：Admission.Driver 或 ScanDriver 未定义，随后以旧 Scan 错领 Paseo 行为作为回归失败点。

- [ ] **Step 3：实现最小行为。**

先写双数据库迁移，再显式列映射。迁移默认 platform、target_id/budget_ref 为空，不改变旧会话 engine_type。新增 driver/status/lease_until 索引。
```sql
ALTER TABLE agent_runs ADD COLUMN driver TEXT NOT NULL DEFAULT 'platform';
ALTER TABLE agent_runs ADD COLUMN target_id TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_runs ADD COLUMN budget_ref TEXT NOT NULL DEFAULT '';
CREATE INDEX agent_runs_driver_scan ON agent_runs(driver, status, lease_until);
```
读取与 claim 必须同时检查 driver；只在 Scan 过滤不足以阻止调用错误 Claim。新增 `ClaimDriver` 与原 Claim(platform) 包装，签名同 Claim 加 driver 参数。

- [ ] **Step 4：接通实际入口。**

`AgentRunWorker` 继续调用原 Scan/Claim，其语义收紧为 platform；远程 W20 使用 ScanDriver/ClaimDriver。GetOwnedRun 的 SQL 包含 tenant_id、owner_id、run_id；不先按裸 run_id 查询再应用客户端 tenant。Admit 保存 driver 前验证 platform/paseo 白名单；Paseo 的 EngineType 使用独立兼容值策略：允许空值且不进入 ParseAgentEngine，旧 platform 仍 trpc，迁移前核对数据库 CHECK。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 将本测试增加 sqlite/postgres 子测试，复用 `openRunTestDB`；PG 使用已有 TRPC_TEST_POSTGRES_DSN，skip 不算通过。
- 旧 Admit 重放、会话活动槽、worker 回归；`go test ./internal/application/repository ./internal/application/service -run 'TestAgentRun|TestWorkbenchDriver' -count=1`。
- driver 错配 claim、未知 driver、不同 owner/tenant 和空 driver 兼容各有负例；迁移 up/down/up 验证旧行不丢失。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add internal/agent/runtime/contracts.go internal/application/repository/agent_run.go internal/application/repository/agent_run_driver.go internal/application/repository/agent_run_driver_test.go internal/application/service/agent_run_worker.go migrations/versioned/000121_workbench_runs.up.sql migrations/versioned/000121_workbench_runs.down.sql migrations/sqlite/000041_workbench_runs.up.sql migrations/sqlite/000041_workbench_runs.down.sql
git diff --cached --check
git diff --cached --stat
git commit -m "feat(runtime): isolate platform and remote run drivers"
```

### W03：所有权 facade、快照和标准 SSE

**依赖：** W02。

**Files：**

- Create: `internal/handler/session/workbench_read.go`、`internal/handler/session/workbench_read_test.go`
- Create: `internal/application/repository/agent_run_snapshot.go`、`internal/application/repository/agent_run_snapshot_test.go`
- Create: `internal/router/routes_workbench.go`
- Modify: `internal/router/router.go`、`internal/handler/session/agent_run.go`（复用 DTO，不破坏旧 event 名）

**Interfaces：**

Consumes W01 ExecutionDTO/ExecutionEvent；W02 GetOwnedRun。Produces `GetWorkbenchExecution`、`GetWorkbenchSnapshot`、`StreamWorkbenchEvents`（Gin handler）；仓储 `ReadRunSnapshot(ctx context.Context,key runtime.RunKey) (workbench.ExecutionSnapshot,error)`。新 GET `/workbench/executions/:run_id`、`/:run_id/snapshot`、`/:run_id/events`。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```go
package session
import (
 "bytes"
 "encoding/json"
 "testing"
 "github.com/stretchr/testify/require"
)
func TestWorkbenchSSEFrameHasID(t *testing.T) {
 var b bytes.Buffer
 err := writeWorkbenchSSE(&b, 7, "text.delta", json.RawMessage(`{"seq":7}`))
 require.NoError(t, err)
 require.Equal(t, "id: 7\nevent: text.delta\ndata: {\"seq\":7}\n\n", b.String())
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/handler/session -run TestWorkbenchSSE -count=1
```

预期：writeWorkbenchSSE 尚未实现或标准 id 缺失。

- [ ] **Step 3：实现最小行为。**

实现只接受受控事件名和有效 JSON 的 frame writer；JSON 使用紧凑编码，拒绝 CR/LF 事件名。
```go
func writeWorkbenchSSE(w io.Writer, seq int64, kind string, raw json.RawMessage) error {
 if seq < 1 || strings.ContainsAny(kind, "\r\n") || !json.Valid(raw) { return errors.New("invalid_event") }
 var data bytes.Buffer
 if err := json.Compact(&data, raw); err != nil { return err }
 _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", seq, kind, data.Bytes())
 return err
}
```
ReadRunSnapshot 在同一个可重复读/一致事务中读取权威消息投影和水位；不能用已裁剪的事件拼出缺失历史。events 字段承载构建消息/工具/产物的完整投影事件，保留稳定实体 ID。

- [ ] **Step 4：接通实际入口。**

先 GetOwnedRun，再加载 Session 当前权限；源 runtime 事件转换到 W01 DTO。capabilities 由 driver 与实际配置得出，不写死 trpc。旧 handler 保留旧协议，新 handler 标准 id。route 使用现有 Viewer 与 chat API-key guard；不增加全访问默认许可。cursor_expired HTTP 前置为409，流中为 error 事件后关闭。限制分页和连接数，15秒心跳；断开不取消 Run。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- HTTP 正例必须实际访问仓储后成功；另测跨空间、同空间其他 owner、缺权限均拒绝，避免所有请求都404的伪测试。
- 快照与并发写事件夹击测试：watermark 前内容全有、之后内容可重放，无丢失重复。
- 大于256事件分页、裁剪后快照、终态尾部事件全部读完再关闭；时间复杂度避免每次查询扫描100000条。
- `go test ./internal/handler/session ./internal/application/repository -run 'TestWorkbench|TestAgentRunEvents' -count=1`。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add internal/handler/session/workbench_read.go internal/handler/session/workbench_read_test.go internal/application/repository/agent_run_snapshot.go internal/application/repository/agent_run_snapshot_test.go internal/router/routes_workbench.go internal/router/router.go internal/handler/session/agent_run.go
git diff --cached --check
git diff --cached --stat
git commit -m "feat(workbench): serve owned snapshots and replayable events"
```

### W04：请求幂等、预算绑定和平台执行入口

**依赖：** W02、W03。

**Files：**

- Create: `internal/application/service/workbench/admission.go`、`internal/application/service/workbench/admission_test.go`
- Create: `internal/application/repository/workbench_request.go`、`internal/application/repository/workbench_request_test.go`
- Create: `internal/handler/session/workbench_start.go`
- Modify: `internal/router/routes_workbench.go`、`internal/container/agent_runtime.go`
- Create: 请求协调记录迁移（见总计划序号表）

**Interfaces：**

Produces `StartInput{SessionID,AgentID,TargetID,WorkspaceRef,RequestID,Text string; BudgetUpper int64}`；`AdmissionCoordinator.Start(ctx context.Context,in StartInput) (runtime.Run,error)`；`LookupRequest(ctx,requestID) (RequestState,error)`，RequestState 为 pending/admitted/rejected/unknown + run_id。请求记录以可信 tenant/actor/request_id 唯一，hash 覆盖会话、目标、输入和预算。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```go
package workbench
import (
 "errors"
 "testing"
)
func TestAdmissionNeverDispatchesWithoutBudget(t *testing.T) {
 called := false
 deny := errors.New("budget_denied")
 err := admitThenPublish(func() error { return deny }, func() error { called=true; return nil })
 if !errors.Is(err,deny) || called { t.Fatal("unfunded execution was published") }
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/application/service/workbench -run TestAdmission -count=1
```

预期：admitThenPublish 未定义。

- [ ] **Step 3：实现最小行为。**

最小顺序函数只是可测接缝，持久正确性由请求仓储测试证明。
```go
func admitThenPublish(admit func() error, publish func() error) error {
 if err := admit(); err != nil { return err }
 return publish()
}
```
将请求 CAS 为 pending，持久化不可变输入，取得预算并保存 reservation_ref，再 Admit 原 Run 和 outbox 命令。使用原预算 store 的稳定逻辑键，不直接把 ExecutionGate.Begin 当作整个任务无限预占；Begin 会标 dispatched，调用级预占仍由具体收费执行边界负责。任务预算创建/绑定与每调用消耗分别持久化，禁止嵌套重复预占。

- [ ] **Step 4：接通实际入口。**

`POST /workbench/executions` 只在参数与权限校验后调用 Start；平台 driver 复用现有 Agent 请求准备和持久 worker 提交接缝，不循环请求本机 HTTP。响应202 + run_id；预算/权限失败无执行。GET requests 只查看同 actor 请求，pending/unknown 不返回“未执行”；同 key 不同 hash 409。故障注入位置覆盖预占后、Run落盘后、outbox提交后，恢复只完成缺失步骤。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 20个并发相同请求只创建一 Run、一用户消息、一任务预算；不同参数409。
- 任务预算创建成功后崩溃，再启动得到同一个 Run 或释放确证未派发占用；未知不释放。
- 实际数据库集成 + API handler 正/负例 + tRPC worker 启动一无KB Agent；模型不可用记 blocked-env。
- 记录预算树根 ID 与 RequestID，客户端给的 tenant/funding 不生效。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add internal/application/service/workbench/admission.go internal/application/service/workbench/admission_test.go internal/application/repository/workbench_request.go internal/application/repository/workbench_request_test.go internal/handler/session/workbench_start.go internal/router/routes_workbench.go internal/container/agent_runtime.go
git diff --cached --check
git diff --cached --stat
git commit -m "feat(workbench): admit idempotent budgeted executions"
```

### W05：有类型的命令与跨权限域交互

**依赖：** W04。

**Files：**

- Create: `internal/workbench/interaction.go`、`internal/workbench/interaction_test.go`
- Create: `internal/handler/session/workbench_commands.go`、`internal/handler/session/workbench_commands_test.go`
- Create: `internal/application/service/workbench/interaction.go`
- Modify: `internal/router/routes_workbench.go`

**Interfaces：**

Produces `InteractionKind`=tool_approval/budget/recovery；`ValidateInteractionAction(kind,action string) error`；`InteractionDecision{ID,DecisionID,Kind,Action,ArgsHash string; ExpectedRevision int64}`。Command 为 cancel/steer 严格联合；恢复动作 retry/provide_result/terminate，工具 approve/reject，预算 extend。真实批准仍由既有审批服务进行。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```go
package workbench
import "testing"
func TestInteractionDomainsDoNotGrantEachOther(t *testing.T) {
 for _, pair := range [][2]string{{"recovery","approve"},{"budget","approve"},{"tool_approval","extend"}} {
  if ValidateInteractionAction(pair[0],pair[1]) == nil { t.Fatalf("accepted %v",pair) }
 }
 if err := ValidateInteractionAction("tool_approval","approve"); err != nil { t.Fatal(err) }
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/workbench -run TestInteraction -count=1
```

预期：ValidateInteractionAction 未定义。

- [ ] **Step 3：实现最小行为。**

```go
func ValidateInteractionAction(kind, action string) error {
 allowed := map[string]map[string]bool{
  "tool_approval":{"approve":true,"reject":true},
  "budget":{"extend":true},
  "recovery":{"retry":true,"provide_result":true,"terminate":true},
 }
 if !allowed[kind][action] { return errors.New("interaction_action_mismatch") }
 return nil
}
```
服务端从持久记录读取 kind、实际参数与归属，客户端 kind 仅一致性校验。先查当前成员/Connection/参数 hash，再按原服务 CAS 提交 decision_id；不能用纯函数测试替代真实审批。

- [ ] **Step 4：接通实际入口。**

新增 GET interactions、POST interactions/:id/decisions、POST executions/:run_id/commands。cancel 先保持现有产品终态，远程停止由W22扩展；steer 复用现有注入契约，不将队列中未发送文本标已执行。handler 对未实现 driver 操作返回 capability_unavailable，不 fallback 到别的动作。返回冲突当前 revision 供 UI 重取。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 双设备相同 decision_id 得到同结果，不同决定只有一个获胜；版本变化409、过期410、撤销拒绝。
- 工具批准不扩额度；扩额度不批准工具；同一 Action 不重复扣审批次数。
- HTTP 测试验证真正调用原审批服务且有成功控制组；`go test ./internal/handler/session ./internal/workbench -run 'TestWorkbenchCommand|TestInteraction' -count=1`。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add internal/workbench/interaction.go internal/workbench/interaction_test.go internal/handler/session/workbench_commands.go internal/handler/session/workbench_commands_test.go internal/application/service/workbench/interaction.go internal/router/routes_workbench.go
git diff --cached --check
git diff --cached --stat
git commit -m "feat(workbench): route typed commands and scoped decisions"
```

### W06：共享 SDK 与请求对账接口

**依赖：** W01、W03–W05。

**Files：**

- Create: `packages/api-client/src/mobile/executions.ts`、`packages/api-client/src/mobile/executions.test.ts`
- Modify: `packages/api-client/src/index.ts`、`package.json`

**Interfaces：**

Consumes W01 parser 与既有 ClientRequest。Produces `createExecutionsApi(request:(input:ClientRequest)=>Promise<unknown>)`，方法 `get(runID:string,signal?:AbortSignal):Promise<ExecutionDTO>`、`snapshot(runID,signal):Promise<ExecutionSnapshot>`、`start(input:StartExecutionInput,signal):Promise<ExecutionDTO>`、`lookup(requestID,signal):Promise<RequestLookup>`、`command(runID,input,signal):Promise<ExecutionDTO>`。StartExecutionInput 是 W04 字段的 snake_case；RequestLookup 明确 state，不返回裸 boolean。响应统一 success/data envelope。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createExecutionsApi } from './executions.ts';
test('timeout cannot silently retry a billable start', async () => {
 let calls=0;
 const api=createExecutionsApi(async () => {calls++; throw new Error('TIMEOUT');});
 await assert.rejects(api.start({request_id:'q',session_id:'s',agent_id:'a',target_id:'platform',workspace_ref:'',text:'hello',budget_upper:10}), /TIMEOUT/);
 assert.equal(calls,1);
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test packages/api-client/src/mobile/executions.test.ts
```

预期：createExecutionsApi 未定义。

- [ ] **Step 3：实现最小行为。**

把每个端点写成单次 request，统一 unwrap 和 parser；返回不符合 contracts 的响应即失败。
```ts
function path(runID: string): string {
 if (!runID.trim()) throw new Error('RUN_REQUIRED');
 return `/api/v1/workbench/executions/${encodeURIComponent(runID)}`;
}
```
start 不自带重试。上层在 TIMEOUT 后调用 lookup，unknown/pending 只轮询，不调用新 start；rejected 确定未执行后才允许用户重提。

- [ ] **Step 4：接通实际入口。**

根 index 导出 SDK 与 Input 类型；根共享 test/typecheck 加入 mobile 目录。所有方法传 signal；GET 不带请求 body。对 commands 用明确的联合类型，不允许 method/URL 任意字符串穿透。snapshot parser 验证水位与所有事件 run_id 一致。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- fake transport 验证 URL 编码、exact body、signal、401/403/409/503 原错误映射。
- malformed 200不得变成成功；相同 request ID 的重放不改参数。
- `pnpm run test:shared`；fixture 响应与W03–W05 HTTP集成样本双向核对。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add packages/api-client/src/mobile/executions.ts packages/api-client/src/mobile/executions.test.ts packages/api-client/src/index.ts package.json
git diff --cached --check
git diff --cached --stat
git commit -m "feat(sdk): add execution facade and request reconciliation"
```


