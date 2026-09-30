# OCR 增量修复批次 2（issue30-sweep）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 OCR 第二批增量报告（`docs/plans/issue30-sweep/ocr/ocr-increment-batch2.md`）中 15 项已验证发现：1 项后端 critical（Workbench snapshot 把租约属主当业务属主导致端点统一 404）+ 14 项移动端（deployment 注册表失败包含与数据策略、组合根接线遗漏、登录表单状态残留、任务详情错误语义与恢复入口、hydrate 并发守卫、SSE 适配层错误形态与资源清理、集成 smoke 收口），并顺手合并 24 项 lowWorth 修复中同文件、低风险的部分。

**Architecture:** 按**根因**聚合成 10 个任务：后端一处字段错配独立成任务（P0）；移动端按「Runtime 对 registry 交互的失败包含（不 reject 约定）→ SecureStore adapter 数据策略（脏数据 filter 语义 + 容量上限）→ 组合根 effect 防护 → 登录屏切换状态重置 → 任务详情错误语义/恢复入口 → hydrate epoch 守卫 → SSE 适配层错误形态/资源清理 → 集成 smoke 收口 → 授权流通道不可用的诚实语义」推进。同根因的多个发现共用同一组测试（不逐条造测试）；任务间共享文件（`mobile-runtime.ts`、`task-detail.ts`、`composition.ts`、`app-smoke.test.tsx`）按任务顺序串行执行避免冲突。所有修复维持既有架构约束：Mobile core 不依赖 RN、Runtime 状态迁移从不 reject、fail-closed 语义不放松。

**Tech Stack:** Go（`internal/handler/session`、`internal/application/repository`，go test + testify）、TypeScript（`packages/mobile-core`、`packages/api-client`、`apps/mobile`，node:test + tsx）。测试命令在 worktree 根（`.worktrees/issue30-sweep`）执行；前置 `pnpm install` 已就绪（本计划作者已实跑下列基线，全部通过：`go test ./internal/handler/session/ -run 'TestGetWorkbenchSnapshot'` ok；`pnpm exec tsx --test packages/mobile-core/src/runtime/runtime-deployments.test.ts` 9 pass；`mobile-runtime.test.ts` 37 pass；`apps/mobile/src/adapters/deployment-registry.test.ts` 4 pass；`apps/mobile/src/adapters/sse-stream.test.ts` 2 pass；`packages/mobile-core/src/task-office/task-detail.test.ts` 16 pass；`apps/mobile/src/task-detail-view.test.ts` 1 pass；`apps/mobile/src/app-smoke.test.tsx` 24 pass；`packages/api-client/src/mobile/task-office.test.ts` 8 pass）。

**Spec:**
- 发现来源：`docs/plans/issue30-sweep/ocr/ocr-increment-batch2.md`（OCR 报告原文；本计划逐项引用其编号 B2-F*，全部经作者按当前 HEAD 读码复核，行号以复核为准）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（Implementation Decisions / Testing Decisions）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§4 Mobile Runtime 所有权与 Interface）
- ADR：`docs/adr/0012-mobile-business-logic-lives-behind-deep-modules.md`、`docs/adr/0007-registered-devices-and-encrypted-cache.md`
- 领域术语：`CONTEXT.md`（「部署实例（Deployment）」：同一时刻只有一个活动实例）
- Parent：Issue #30（issue30-sweep）

## Global Constraints

以下为批准 Spec / ADR / 报告隐含的项目级约束，所有任务隐含遵守：

- 「Mobile core does not depend on React Native, DOM or concrete transport. Remote and native details are injected as Adapters.」（mobile-ai-office-design.md · Implementation Decisions）——Task 7/8/10 对 mobile-core 的修改不得引入 RN/平台依赖。
- 「The App Shell is a composition root and presentation Adapter. Screens do not call wire clients directly or maintain request IDs, cursors, revisions or scope generations.」（同上）——Task 4/5/6 只动组合根与 Screen，不把游标/generation 泄漏进 UI 层。
- 「Tests target observable behavior at the highest stable Interface.」（同上 · Testing Decisions）——回归测试落在 Interface 行为（Runtime 方法、Screen 渲染、handler HTTP 输出），不测内部实现细节。
- Runtime 状态迁移从不 reject 的既有约定（报告 B2-F17 原文「违背 Runtime 其余状态迁移从不 reject 的约定」；`switchDeployment`/`listDeployments`/`forgetDeployment` 均不得 reject）。
- Android 上 Expo SecureStore 单值约 2KB（报告 B2-F32）：注册表与任务投影存储的持久化载荷必须有界。
- 凭据与测试安全约束：测试凭据仅用 `*.example.test` 保留域形态（与 `runtime-deployments.test.ts` 的 `DEFAULT_GRANT` 模式一致）；真实 HTTP 证据仅公网 HTTPS 主机（`disallowedDeploymentHost` 防线，拒绝 localhost/环回/私网/链路本地/保留地址）；源码、示例和测试不得写入可用凭据字面量。
- 严格 RED→GREEN→REFACTOR：每个任务先写失败测试、实跑确认失败、最小实现、通过、提交；实现与已批准 Spec 冲突时升级处理，不静默重设计。
- 发现驱动的最小修复：不顺手扩大范围；lowWorth 项仅按「附录 A」的映射折叠，结构化重构（B2-F4/F28/F35 等）明确延期并记录理由。

## Review Focus

Spec/报告隐含但任务测试需钉住、最可能咬到真实用户的五类失效模式（每行后标注 owning 任务）：

1. **已 settle 的 run（`lease_owner` 为空串）请求 snapshot** —— 不得因属主字段错配统一 404；租约属主是 worker 标识，业务属主才用于 facts 查询。——Task 1 测试 `TestGetWorkbenchSnapshotFactsUseBusinessOwner`。
2. **SecureStore 读取失败（Keychain/Keystore 错误）** —— 任何 Runtime 状态迁移（`listDeployments`/`switchDeployment`/`forgetDeployment`/`authenticate`）都不得形成 unhandled rejection，也不得把已授权用户降级为未授权。——Task 2 五个测试。
3. **单条脏注册数据 / 注册表 JSON 超 2KB** —— 不得静默清空全部已注册 Deployment，不得因 `setItemAsync` 抛错把登录裁决为 `authentication-required`。——Task 3 四个测试 + Task 2 的 registry 写失败测试。
4. **SSE `onChunk` 同步抛出 / 畸形 control 帧** —— reader 必须 cancel 释放（连接不得泄漏），错误必须是真 `ApiError`（含 `code`），畸形帧必须中断流而不是崩溃读取循环。——Task 8 三个测试。
5. **迟到的旧 hydrate 响应 / 被取代流的旧事件** —— 不得以裁剪后事件集覆写 store、不得回退 `committedCursor`。——Task 7 两个测试。

---

## 任务结构与文件地图

| # | 任务 | 根因分组（发现编号） | 主要文件 | 优先级 |
|---|---|---|---|---|
| 1 | 后端：snapshot 属主字段错配 | B2-F12 | `internal/handler/session/workbench_read.go` | **P0/critical** |
| 2 | mobile-core：Runtime 对 registry 交互的失败包含 | B2-F17、B2-F42、B2-F32(runtime 侧)；顺手 F43/F44/F22/F36 | `mobile-runtime.ts` | high |
| 3 | apps/mobile：SecureStore 注册表解析与容量策略 | B2-F31、B2-F32(adapter 侧)；顺手 F21 | `deployment-registry.ts`、`in-memory-adapters.ts` | high |
| 4 | apps/mobile：部署列表 effect 防护 | B2-F33 | `composition.ts` | medium |
| 5 | apps/mobile：切换实例后的表单状态重置 | B2-F10；顺手 F2/F3 | `DeploymentLoginScreen.tsx`、`HomeScreen.tsx` | medium |
| 6 | apps/mobile：任务详情错误语义与就地重试 | B2-F13、B2-F38；顺手 F16/F5/F39/F40/F41 | `app/tasks/detail.tsx`、`TaskDetailScreen.tsx`、`task-detail-view.ts`、`TasksScreen.tsx` | medium |
| 7 | mobile-core：hydrate epoch/串行化守卫 | B2-F24；顺手 F26/F27 | `task-detail.ts` | high |
| 8 | SSE 适配层：错误形态与读取循环清理 | B2-F29、B2-F30；顺手 F8/F37 | `apps/mobile/src/adapters/sse-stream.ts`、`packages/api-client/src/mobile/task-office.ts` | high |
| 9 | apps/mobile：集成 smoke 收口 | B2-F14；顺手 F15 | `task-detail-integration-smoke.ts`、`runtime-integration-smoke.ts` | medium |
| 10 | mobile-core：授权流通道不可用的诚实语义 | B2-F34；顺手 F19/F20/F45 | `mobile-runtime.ts`、`ports.ts`、`task-detail.ts` | medium |

执行顺序即任务号顺序（依赖关系：Task 2 先于 Task 4——effect 消费「`listDeployments` 从不 reject」契约；Task 6 先于 Task 10——错误文案映射表先建立，Task 10 追加 `'stream-unavailable'` 条目；Task 7 先于 Task 10——两者同改 `task-detail.ts`）。Task 1（Go）与 Task 3 独立，可与 Task 2 并行（不同包、不同文件）。共享文件：`mobile-runtime.ts`（Task 2→10）、`task-detail.ts`（Task 7→10）、`composition.ts`（仅 Task 4）、`app-smoke.test.tsx`（Task 4/5/6 追加式）。

## 差异记录（报告 vs 代码现状，以代码现状为准）

1. B2-F23 引用的 `task-detail-view.ts:37-46` 实际位于 `apps/mobile/src/task-detail-view.ts`（apps/mobile，非 mobile-core）；B2-F8/F37 引用的 api-client task-office 实际位于 `packages/api-client/src/mobile/task-office.ts`。行号本身与报告一致。
2. B2-F34 的修复落点**不在** `composition.ts:106`（报告所指）：组合层以闭包 `(input, onChunk) => activeRuntime.authorizedEventStream(input, onChunk)` 接线，闭包无法预知 Runtime 是否为该 origin 解析出流通道（`ports.authorizedStream` 是 per-origin 惰性工厂，fail-closed 语义在 runtime 内部）。诚实语义必须在 `mobile-runtime.ts` 的 `authorizedEventStream` 里区分「未授权」与「已授权但无流通道」，Task 10 按此设计。
3. B2-F23 的完整修复（生产接持久化任务投影存储）受 SecureStore 单值约 2KB 与 `TASK_DETAIL_HISTORY_LIMIT = 200` 事件的体积冲突制约：全量投影 JSON 必然超限。本批次把它降级为**明确延期**（见附录 A），不在本批次实现半吊子的截断存储——重启合并恢复需要存储选型决策（SecureStore 截断 vs SQLite/文件），应走独立 ADR。

---

### Task 1: [P0] GetWorkbenchSnapshot 属主字段错配（B2-F12）

**Files:**
- Modify: `internal/handler/session/workbench_read.go:181`
- Test: `internal/handler/session/workbench_read_task_facts_test.go`

**Interfaces:**
- Consumes: `agentruntime.Run`（`internal/modules/agentruntime/agent/runtime/contracts.go`：`Owner` = 租约属主 `agent_runs.lease_owner`（Claim 时写 worker 标识、Settle 清空），`UserID` = 业务属主 `agent_runs.owner_id`；`repository/agent_run.go:58-64` view() 映射 `UserID: r.OwnerID, Owner: r.LeaseOwner`）。
- Produces: `GetWorkbenchSnapshot` 以业务属主调用 `ReadTaskFactsForRun`——对已 settle 与运行中的 run 均返回 200（含 task 段），不再统一 404。无签名变化。

**根因与修复说明：** `workbench_read.go:181` 传 `run.Owner`（= `lease_owner`），而 `ReadTaskFactsForRun` 的守卫（`workbench_task_facts.go:39`）在 `ownerID == ""` 时直接 `ErrNotFound`、WHERE 绑定 `agent_runs.owner_id`（`:46`）。已 settle 的 run `lease_owner` 为空串 → 404；运行中 run `lease_owner` 是 worker id → 永不匹配 → 404。容器无条件接线 `WithTaskFacts(lists)`（`internal/container/workbench.go:26`），故端点对所有合法请求统一 404。现有单测未暴露：`stubTaskFactsReader.ReadTaskFactsForRun` 忽略全部入参（`workbench_read_task_facts_test.go:20`），且 run 夹具手工构造 `Owner: "u1"`（`:28`）。修复 = 传 `run.UserID`（由 `resolveOwnedRun → GetOwnedRun → row.view()` 填充，已核实 `agent_run.go:100-114`）。

- [ ] **Step 1: 写失败测试（先强化 stub 记录入参，再断言业务属主）**

```go
// workbench_read_task_facts_test.go —— stub 改为记录 ownerID 入参（原实现完全忽略，属主错配因此漏检）
type stubTaskFactsReader struct {
	facts repository.WorkbenchTaskFacts
	err   error
	calls int
	seenOwnerIDs []string
}

func (s *stubTaskFactsReader) ReadTaskFactsForRun(_ context.Context, _ uint64, ownerID, _ string) (repository.WorkbenchTaskFacts, error) {
	s.calls++
	s.seenOwnerIDs = append(s.seenOwnerIDs, ownerID)
	return s.facts, s.err
}

// 夹具区分两个属主：UserID 是业务属主（HTTP 调用方，owner_id），Owner 是租约属主（worker 标识，lease_owner）。
// workbenchRunReaderStub.GetOwnedRun 按 ownerID != "u1" 硬编码判 404（workbench_read_test.go:35），
// 该守卫不受本夹具变化影响。
func snapshotHandlerWithFacts(facts OwnedTaskFactsReader) *WorkbenchReadHandler {
	runs := &workbenchRunReaderStub{run: agentruntime.Run{
		Key:       agentruntime.RunKey{TenantID: 1, RunID: "r1"},
		UserID:    "u1",     // agent_runs.owner_id —— facts 查询的属主
		Owner:     "worker-1", // agent_runs.lease_owner —— Claim 写入的 worker 标识
		SessionID: "s1",
	}}
	snapshots := &workbenchSnapshotReaderStub{snapshot: workbench.ExecutionSnapshot{Execution: workbench.ExecutionDTO{SchemaVersion: 1, RunID: "r1", SessionID: "s1", Driver: "platform", RunStatus: "running", ExecutionStatus: "running", SettlementStatus: "pending", Capabilities: map[string]workbench.Capability{}}}}
	h := NewWorkbenchReadHandler(runs, snapshots)
	if facts != nil {
		h = h.WithTaskFacts(facts)
	}
	return h
}

func TestGetWorkbenchSnapshotFactsUseBusinessOwner(t *testing.T) {
	facts := &stubTaskFactsReader{facts: repository.WorkbenchTaskFacts{TaskID: "s1", Attention: "none"}}
	c, w := workbenchRequest(t, "u1")
	snapshotHandlerWithFacts(facts).GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []string{"u1"}, facts.seenOwnerIDs, "task facts 必须按业务属主（agent_runs.owner_id）查询，而非租约属主（agent_runs.lease_owner）")
}
```

既有测试 `TestGetWorkbenchSnapshotCarriesTaskFacts` 等使用同一夹具但不检查 owner 入参，`Owner: "u1"` → `"worker-1"` 的夹具变化对它们无影响（stub 的属主守卫是硬编码 `"u1"` 字面量）。

- [ ] **Step 2: 实跑确认失败**

Run: `go test ./internal/handler/session/ -run TestGetWorkbenchSnapshotFactsUseBusinessOwner -v`
Expected: FAIL —— `seenOwnerIDs` 实际为 `["worker-1"]`（当前 `workbench_read.go:181` 传 `run.Owner`）。

- [ ] **Step 3: 最小实现**

```go
// workbench_read.go:181 —— Owner（租约属主）改为 UserID（业务属主）
facts, factsErr := h.taskFacts.ReadTaskFactsForRun(c.Request.Context(), run.Key.TenantID, run.UserID, run.Key.RunID)
```

- [ ] **Step 4: 实跑确认通过 + 全包回归**

Run: `go test ./internal/handler/session/ -run TestGetWorkbenchSnapshot -v`
Expected: PASS（全部 snapshot 用例）。
Run: `go test ./internal/handler/session/ ./internal/application/repository/`
Expected: ok（两个包全绿；repository 包无改动，跑它证明 facts 查询侧契约未受扰动）。

- [ ] **Step 5: Commit**

```bash
git add internal/handler/session/workbench_read.go internal/handler/session/workbench_read_task_facts_test.go
git commit -m "fix(workbench): read task facts by business owner (owner_id) instead of lease owner"
```

---

### Task 2: mobile-core：Runtime 对 deployment-registry 交互的失败包含（B2-F17、B2-F42、B2-F32 runtime 侧）

**Files:**
- Modify: `packages/mobile-core/src/runtime/mobile-runtime.ts:237-240`（authenticate 的 registry.upsert 隔离）、`:412-414`（listDeployments）、`:415-436`（switchDeployment）、`:438-460`（forgetDeployment，顺手 B2-F43）
- Test: `packages/mobile-core/src/runtime/runtime-deployments.test.ts`

**Interfaces:**
- Consumes: `MobileRuntimePorts.deploymentRegistry?: DeploymentRegistry`（`{ list(); upsert(); remove() }`，plan-T66 产出）。
- Produces（Task 4 依赖的精确契约）: `listDeployments(): Promise<Deployment[]>` —— registry 读取失败时 resolve `[]`（与「未提供端口时返回空数组」的 fail-closed 语义一致），**从不 reject**；`switchDeployment`/`forgetDeployment` **从不 reject**（失败保持当前 surface）；`authenticate` 不因 `registry.upsert` 失败离开 authorized 面。

**根因与修复说明：** 同一根因——Runtime 与 presentation 辅助存储（registry）交互的异常未做失败包含：(a) `switchDeployment` 中 `await ports.deploymentRegistry.list()`（`:422`）位于内层 try(426)/catch(431) 之外、外层 try 只有 finally（B2-F17）；(b) `listDeployments`（`:412-414`）无 catch 直接透传 rejection（B2-F42）；(c) `authenticate` 的 `mutateDeployment` 内 `registry.upsert`（`:237-240`）失败落入 `:254` 的 catch，整次登录被判 `authentication-required`（B2-F32 runtime 侧）——凭据已验证却呈现未授权。顺手项：B2-F43（forgetDeployment 的持久化失败包含）、B2-F44（同 origin 短路）、B2-F22（upsert 隔离后写序无回滚的问题只剩 deploymentStore.write，语义保持并注释）、B2-F36（screens 的 `void onSwitchDeployment` 在本任务后安全）。

- [ ] **Step 1: 写失败测试（一个根因一组测试，追加到 runtime-deployments.test.ts）**

```ts
// 顶部 import 追加：import type { DeploymentRegistry } from './ports.ts';
// registry 读取/写入失败的注入夹具（async throw 推断 Promise<never>，结构兼容 DeploymentRegistry）
function failingRegistry(): DeploymentRegistry {
  const failure = async (): Promise<never> => { throw new Error('SECURESTORE_UNAVAILABLE'); };
  return { list: failure, upsert: failure, remove: failure };
}

test('listDeployments resolves to an empty list when the registry read rejects', async () => {
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(), remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION, deploymentRegistry: failingRegistry(),
  });
  assert.deepEqual(await runtime.listDeployments(), []);
});

test('switchDeployment keeps the current surface when the registry read rejects', async () => {
  const remoteCalls: string[] = [];
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(), clientVersion: CLIENT_PROTOCOL_VERSION,
    remoteFor: () => remote({ passwordLogin: async () => { remoteCalls.push('login'); return { ...DEFAULT_GRANT }; } }),
    deploymentRegistry: { list: async () => { throw new Error('SECURESTORE_UNAVAILABLE'); }, upsert: async () => {}, remove: async () => {} },
  });
  await runtime.signIn({ deployment: FIRST, email: 'user@example.test', password: 'pw' });
  const before = runtime.snapshot();
  const after = await runtime.switchDeployment(SECOND.origin); // registry.list reject —— 不得 reject、不得扰动当前面
  assert.equal(after.surface, before.surface);
  assert.equal(after.deployment?.origin, FIRST.origin);
  assert.deepEqual(remoteCalls, ['login']); // 未对目标实例发出任何远程调用
});

test('an authorized sign-in survives a registry write failure', async () => {
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(), remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION, deploymentRegistry: failingRegistry(), // upsert reject
  });
  const snapshot = await runtime.signIn({ deployment: FIRST, email: 'user@example.test', password: 'pw' });
  assert.equal(snapshot.surface, 'authorized', 'registry 是 presentation 辅助数据，写失败不得把登录裁决为 authentication-required');
});

test('forgetDeployment resolves when persistence fails', async () => {
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(), remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION, deploymentRegistry: failingRegistry(),
  });
  await runtime.signIn({ deployment: FIRST, email: 'user@example.test', password: 'pw' });
  await runtime.forgetDeployment(FIRST.origin); // 不得 reject（mutateCredential/registry.remove 失败均包含）
});

test('switchDeployment onto the active origin short-circuits without re-authentication', async () => {
  const remoteCalls: string[] = [];
  const deployments = createInMemoryDeploymentRegistry();
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(), clientVersion: CLIENT_PROTOCOL_VERSION,
    remoteFor: () => remote({ passwordLogin: async () => { remoteCalls.push('login'); return { ...DEFAULT_GRANT }; } }),
    deploymentRegistry: deployments,
  });
  const signedIn = await runtime.signIn({ deployment: FIRST, email: 'user@example.test', password: 'pw' });
  const switched = await runtime.switchDeployment(FIRST.origin);
  assert.equal(switched.surface, 'authorized');
  assert.equal(switched.deployment?.origin, FIRST.origin);
  assert.deepEqual(remoteCalls, ['login'], '同 origin 且当前会话有效时不得重走 begin/authenticate');
  assert.equal(switched, signedIn); // 快照对象不重建（短路返回当前 state）
});
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/runtime/runtime-deployments.test.ts`
Expected: FAIL —— 前 4 个新测试 reject（当前 registry 异常直接透传）；第 5 个在 `remoteCalls` 断言处失败（当前同 origin 也重走 authenticate）。

- [ ] **Step 3: 最小实现（mobile-runtime.ts）**

```ts
// :412-414 —— 失败包含：registry 读取失败按 fail-closed 语义返回空数组（与端口缺失一致）
async listDeployments(): Promise<Deployment[]> {
  if (!ports.deploymentRegistry) return [];
  try { return await ports.deploymentRegistry.list(); } catch { return []; }
},

// switchDeployment —— 两处修改：
// (a) :417 之后加同 origin 短路（B2-F44）：目标即当前活动实例且会话有效时不重走 begin/authenticate
if (state.surface !== 'deployment-login' && state.deployment?.origin === target.origin) return state;
// (b) :422 的 registry.list 加失败包含（B2-F17）：SecureStore 读取失败按未登记处理，保持当前面
let entries: Deployment[] | undefined;
try { entries = await ports.deploymentRegistry.list(); } catch { entries = undefined; }
const record = entries?.find((entry) => entry.origin === target!.origin);
if (!record) return state;

// :237-240 —— authenticate 内 registry.upsert 隔离（B2-F32 runtime 侧）：
// deploymentStore.write 是活动实例记忆（授权主流程），失败语义保持；
// registry.upsert 是 presentation 辅助，失败单独吞掉，不得把整次登录拖入 catch → authentication-required。
await mutateDeployment(async () => {
  await ports.deploymentStore?.write(deployment);
  try { await ports.deploymentRegistry?.upsert(deployment); } catch { /* presentation 辅助数据：写失败不阻塞授权 */ }
});

// forgetDeployment（B2-F43）—— 全身包 try/catch：持久化失败 resolve 而非 reject（与不 reject 约定一致）
```

- [ ] **Step 4: 实跑确认通过 + mobile-runtime 全量回归**

Run: `pnpm exec tsx --test packages/mobile-core/src/runtime/runtime-deployments.test.ts`
Expected: PASS（14 个：原 9 + 新 5）。
Run: `pnpm exec tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts packages/mobile-core/src/runtime/runtime-readonly.test.ts`
Expected: PASS（原 37 + readonly 全绿；证明失败包含未改变既有授权/降级语义）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/runtime/mobile-runtime.ts packages/mobile-core/src/runtime/runtime-deployments.test.ts
git commit -m "fix(mobile-core): contain deployment-registry failures inside runtime transitions"
```

---

### Task 3: apps/mobile：SecureStore 注册表解析与容量策略（B2-F31、B2-F32 adapter 侧；顺手 B2-F21）

**Files:**
- Modify: `apps/mobile/src/adapters/deployment-registry.ts:6-24`（parseRecords filter 语义）、`:28-33`（upsert 容量上限）
- Modify: `packages/mobile-core/src/runtime/in-memory-adapters.ts:17-23`（顺手 B2-F21：label 归一化）
- Test: `apps/mobile/src/adapters/deployment-registry.test.ts`、`packages/mobile-core/src/runtime/runtime-deployments.test.ts`

**Interfaces:**
- Consumes: `SecureStorePort`（`getItemAsync`/`setItemAsync`）、`DeploymentRegistry` 端口。
- Produces: `parseRecords(raw): Array<{origin, label}>`（**不再返回 undefined**：非数组/JSON 破损 → `[]`；数组内**单条畸形条目跳过、其余保留**）；`MAX_REGISTRY_ENTRIES = 8`（upsert 前移语义不变，超出裁掉最旧）；持久化 JSON ≤ 安全线（8 条 origin/label 远小于 2KB）。

**根因与修复说明：** 同一根因——adapter 的数据策略是「全有或全无 + 无界」：`parseRecords` 任一条目畸形即整体 `return undefined`（`:12`/`:14`），`list()` 显示为空（`:27` `?? []`）、`upsert`/`remove` 以空列表整体覆写 SecureStore（`:29`/`:35-36`）——单条脏数据即静默清空全部注册；且无条目上限（`:31-32`），Android 2KB 超限时 `setItemAsync` 抛错（runtime 侧影响已由 Task 2 隔离，adapter 侧仍应保证不超限）。顺手 B2-F21：in-memory 版 upsert 原样保存 label，与 SecureStore 版 trim/兜底不一致——统一归一化。

- [ ] **Step 1: 写失败测试（追加到 deployment-registry.test.ts）**

```ts
// 共享夹具：内存 SecureStore 记录最后一次写入
function recordingStore(initial: string | null = null): { store: SecureStorePort; writes: Array<string | null> } {
  const writes: Array<string | null> = [initial];
  return {
    writes,
    store: {
      getItemAsync: async () => writes[0] ?? null,
      setItemAsync: async (_key: string, value: string) => { writes[0] = value; },
      deleteItemAsync: async () => { writes[0] = null; },
    },
  };
}

test('list skips malformed entries and keeps the valid ones', async () => {
  const raw = JSON.stringify([
    { origin: 'https://a.example.test', label: 'A' },
    'not-an-object',                                   // 畸形：整体曾返回 undefined → 清空全部
    { origin: '', label: 'empty-origin' },             // 畸形 origin
    { origin: 'https://b.example.test' },              // 合法：label 兜底为 origin
  ]);
  const { store } = recordingStore(raw);
  const registry = createSecureDeploymentRegistry(store);
  const listed = await registry.list();
  assert.deepEqual(listed, [
    { origin: 'https://a.example.test', label: 'A' },
    { origin: 'https://b.example.test', label: 'https://b.example.test' },
  ]);
});

test('upsert over a corrupt store recovers instead of wiping siblings', async () => {
  // 预置：一条合法 + 一条畸形；upsert 新实例后，畸形条目被剔除、合法条目保留
  const { store, writes } = recordingStore(JSON.stringify([{ origin: 'https://a.example.test', label: 'A' }, 42]));
  const registry = createSecureDeploymentRegistry(store);
  await registry.upsert({ origin: 'https://new.example.test', label: 'New' });
  const persisted = JSON.parse(writes[0]!);
  assert.deepEqual(persisted.map((r: { origin: string }) => r.origin), ['https://new.example.test', 'https://a.example.test']);
});

test('upsert caps the registry at MAX_REGISTRY_ENTRIES entries', async () => {
  const { store, writes } = recordingStore();
  const registry = createSecureDeploymentRegistry(store);
  for (let index = 0; index < 12; index += 1) await registry.upsert({ origin: `https://host${index}.example.test`, label: `h${index}` });
  const persisted = JSON.parse(writes[0]!);
  assert.equal(persisted.length, 8, '注册表必须有界：超出上限裁掉最旧条目，保证 SecureStore 单值不超 2KB 级预算');
  assert.equal(persisted[0].origin, 'https://host11.example.test'); // 前移语义：最新在最前
});

test('a non-array payload degrades to an empty registry without throwing', async () => {
  const { store } = recordingStore('"just-a-string"');
  const registry = createSecureDeploymentRegistry(store);
  assert.deepEqual(await registry.list(), []);
});
```

顺手 B2-F21（追加到 `runtime-deployments.test.ts`）：

```ts
test('the in-memory registry normalizes labels like the secure adapter', async () => {
  const deployments = createInMemoryDeploymentRegistry();
  await deployments.upsert({ origin: 'https://weknora.example.test', label: '  ' }); // 空白 label → 兜底 origin
  assert.deepEqual(await deployments.list(), [{ origin: 'https://weknora.example.test', label: 'https://weknora.example.test' }]);
});
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/adapters/deployment-registry.test.ts`
Expected: FAIL（前 3 个：当前脏条目导致 `[]`/覆写、无上限）。
Run: `pnpm exec tsx --test packages/mobile-core/src/runtime/runtime-deployments.test.ts`
Expected: FAIL（B2-F21 用例：当前 in-memory 原样保存 `'  '`）。

- [ ] **Step 3: 最小实现**

```ts
// deployment-registry.ts
export const MAX_REGISTRY_ENTRIES = 8;

function parseRecords(raw: string | null): Array<{ origin: string; label: string }> {
  let value: unknown;
  try { value = raw && JSON.parse(raw); } catch { return []; }
  if (!Array.isArray(value)) return [];
  const records: Array<{ origin: string; label: string }> = [];
  for (const entry of value) {
    if (!entry || typeof entry !== 'object' || Array.isArray(entry)) continue;   // 单条畸形：跳过而非整体弃用
    const record = entry as { origin?: unknown; label?: unknown };
    if (typeof record.origin !== 'string' || record.origin.trim() === '') continue;
    const label = typeof record.label === 'string' && record.label.trim() !== '' ? record.label.trim() : record.origin;
    records.push({ origin: record.origin, label });
  }
  return records;
}
// upsert：const next = [新条目, ...records.filter((record) => record.origin !== deployment.origin)].slice(0, MAX_REGISTRY_ENTRIES)（前移后截断）

// in-memory-adapters.ts upsert —— label 归一化与 SecureStore 版一致：
const label = deployment.label.trim() !== '' ? deployment.label.trim() : deployment.origin;
const next = { origin: deployment.origin, label };
```

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `pnpm exec tsx --test apps/mobile/src/adapters/deployment-registry.test.ts`
Expected: PASS（原 4 + 新 4）。
Run: `pnpm exec tsx --test packages/mobile-core/src/runtime/runtime-deployments.test.ts apps/mobile/src/app-smoke.test.tsx`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/adapters/deployment-registry.ts apps/mobile/src/adapters/deployment-registry.test.ts packages/mobile-core/src/runtime/in-memory-adapters.ts packages/mobile-core/src/runtime/runtime-deployments.test.ts
git commit -m "fix(mobile): deployment registry skips malformed entries and caps stored size"
```

---

### Task 4: apps/mobile：部署列表 effect 的失败包含与乱序防护（B2-F33）

**Files:**
- Modify: `apps/mobile/src/composition.ts:193-195`
- Test: `apps/mobile/src/app-smoke.test.tsx`

**Interfaces:**
- Consumes: Task 2 的契约——`listDeployments()` 从不 reject（registry 失败 → `[]`）。
- Produces: `MobileApp` 的部署列表 effect：(a) 读取失败不形成 unhandled rejection、`deployments` 维持现状；(b) 以递增请求序号 latest-wins，过期完成不得覆写新列表；(c) 依赖收窄为 `[activeRuntime, snapshot.surface, snapshot.deployment?.origin]`（registry 内容只在 surface/origin 变化的发布中变化，tenant 切换等发布不再重复读安全存储；`forgetDeployment` 不发布快照，故新旧代码在此场景同样停留旧值，行为不回退）。

**根因与修复说明：** `composition.ts:193-195` 的 effect `void activeRuntime.listDeployments().then(setDeployments)` 无 catch（Task 2 后 runtime 侧已不 reject，此处为纵深防御）且依赖整个 `snapshot` 对象（每次 publish 新对象都重复读 SecureStore）；两次读取乱序完成时旧列表覆写新列表。

- [ ] **Step 1: 写失败测试（app-smoke.test.tsx 组件直调 + 源级断言双证据）**

```ts
// 行为级：latest-wins —— Step 3 导出的 createDeploymentListSync 可直调
test('the deployment list sync ignores out-of-order completions', async () => {
  const received: unknown[][] = [];
  const setDeployments = (value: unknown) => { received.push(JSON.parse(JSON.stringify(value))); };
  let releaseFirst: (() => void) | undefined;
  let calls = 0;
  const runtime = {
    listDeployments: (): Promise<unknown[]> => {
      calls += 1;
      if (calls === 1) return new Promise((resolve) => { releaseFirst = () => resolve([{ origin: 'https://a.example.test', label: 'A' }]); });
      return Promise.resolve([{ origin: 'https://b.example.test', label: 'B' }]);
    },
  };
  const sync = createDeploymentListSync(runtime as never, setDeployments as never);
  sync();                                                    // 第一次读取：挂起（模拟慢 SecureStore）
  sync();                                                    // 第二次读取：立即完成 → setDeployments(B)
  await new Promise((resolve) => setImmediate(resolve));
  releaseFirst?.();                                          // 旧响应迟到完成
  await new Promise((resolve) => setImmediate(resolve));
  assert.deepEqual(received, [[{ origin: 'https://b.example.test', label: 'B' }]], '只有最新一次读取的结果生效；迟到的旧结果被序号守卫丢弃');
});

test('the deployment list sync swallows read failures', async () => {
  const received: unknown[][] = [];
  const runtime = { listDeployments: () => Promise.reject(new Error('SECURESTORE_UNAVAILABLE')) };
  const sync = createDeploymentListSync(runtime as never, ((value: unknown) => { received.push(value as never); }) as never);
  sync();
  await new Promise((resolve) => setImmediate(resolve));
  assert.deepEqual(received, [], '读取失败维持现状且不形成 unhandled rejection');
});
```

源级断言（防回归，沿用 app-smoke 既有源级模式）：

```ts
test('MobileApp subscribes deployment list reads with containment and narrowed deps', async () => {
  const { readFileSync } = await import('node:fs');
  const source = readFileSync(join(here, 'composition.ts'), 'utf8');  // 沿用 app-smoke :547-554 的源级模式
  assert.match(source, /createDeploymentListSync/);
  assert.match(source, /snapshot\.surface, snapshot\.deployment\?\.origin/); // 依赖收窄：不再整快照触发重复读
});
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx`
Expected: FAIL（`createDeploymentListSync` 未导出；源级断言不匹配）。

- [ ] **Step 3: 最小实现（composition.ts）**

```ts
/** 部署列表同步：latest-wins + 失败包含。导出以供 app-smoke 行为级直调。 */
export function createDeploymentListSync(
  runtime: Pick<MobileRuntime, 'listDeployments'>,
  setDeployments: (deployments: Deployment[]) => void,
): () => void {
  let sequence = 0;
  return () => {
    const ticket = ++sequence;
    void runtime.listDeployments().then(
      (deployments) => { if (ticket === sequence) setDeployments(deployments); },
      () => undefined, // 失败包含：维持现状，不形成 unhandled rejection
    );
  };
}

// MobileApp 内：
const syncDeployments = useRef(createDeploymentListSync(activeRuntime, setDeployments));
useEffect(() => { syncDeployments.current(); }, [activeRuntime, snapshot.surface, snapshot.deployment?.origin]);
```

- [ ] **Step 4: 实跑确认通过**

Run: `pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx`
Expected: PASS（原 24 + 新 3：两个行为级 + 一个源级）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/composition.ts apps/mobile/src/app-smoke.test.tsx
git commit -m "fix(mobile): deployment list effect gains containment, latest-wins and narrowed deps"
```

---

### Task 5: apps/mobile：切换实例后的登录表单状态重置（B2-F10；顺手 B2-F2/F3）

**Files:**
- Modify: `apps/mobile/src/screens/DeploymentLoginScreen.tsx:40-45`
- Modify: `apps/mobile/src/screens/HomeScreen.tsx:40-42`
- Test: `apps/mobile/src/app-smoke.test.tsx`（react stub 提供 `__beginRender`/`__mount` 与可触发 setState 的 hooks——见 `app-smoke.test.tsx:27` 的 react stub）

**Interfaces:**
- Consumes: `DeploymentLoginScreenProps.onSwitchDeployment?(origin)`（可选）。
- Produces: 切换按钮 onPress 在调用 `onSwitchDeployment` 的同时重置本地表单：`setOrigin(deployment.origin)`、`setEmail('')`、`setPassword('')`、`setError(undefined)`——无凭据目标实例发布 `deployment-login` 新快照后组件实例复用时，表单显示的是刚切换的目标 origin，Sign in 不会再登录旧 origin。「Registered deployments」/「Switch to …」区块以 `onSwitchDeployment` 存在为渲染前提（B2-F2/F3：缺失 handler 时不渲染死交互按钮）。

**根因与修复说明：** 切换目标实例无已存凭据时，`mobile-runtime.ts:429` 发布 `surface: 'deployment-login'` 的新快照（deployment 已是目标），但组件实例复用导致本地 `origin/email/password/error` 全部残留（`DeploymentLoginScreen.tsx:43-45` 仅调 `onSwitchDeployment`），Sign in 使用本地旧 origin。

- [ ] **Step 1: 写失败测试（app-smoke 组件直调，经 react stub 的 useState 驱动）**

```ts
test('switching deployment from the login screen resets the form onto the target origin', async () => {
  const react = (await import('react')) as unknown as {
    __beginRender(): void; useState<T>(initial: T): [T, (next: T) => void];
  };
  react.__beginRender();
  const switched: string[] = [];
  const { DeploymentLoginScreen } = await import('./screens/DeploymentLoginScreen.tsx');
  const tree = DeploymentLoginScreen({
    officialCloudOrigin: 'https://cloud.example.test',
    deployments: [{ origin: 'https://selfhost.example.test', label: 'Selfhost' }],
    onSignIn: async () => {}, onBeginOidc: async () => {},
    onSwitchDeployment: async (origin) => { switched.push(origin); },
  });
  // 找到 Registered deployments 区块的 Button（react stub 的 createElement 产出 {type, props} 树）
  const registeredView = tree.props.children.find((child: { type?: string }) => child?.type === 'View' && child.props?.children?.[1]?.props?.title === 'Selfhost');
  const switchButton = registeredView.props.children[1];
  // 预置本地状态：用户先在表单里输入了另一个 origin/凭据（经 TextInput 的 onChangeText 写入 stub state）
  const textInputs = tree.props.children.filter((child: { type?: string }) => child?.type === 'TextInput');
  textInputs[0].props.onChangeText('https://stale.example.test');
  textInputs[1].props.onChangeText('user@stale.example.test');
  switchButton.props.onPress();
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(switched, ['https://selfhost.example.test']);
  // 重渲染：状态经 stub 保留，origin 输入框现在显示目标 origin（而非残留的 stale）
  react.__beginRender();
  const rerendered = DeploymentLoginScreen({ /* 同上 props */ } as never);
  const rerenderedInputs = rerendered.props.children.filter((child: { type?: string }) => child?.type === 'TextInput');
  assert.equal(rerenderedInputs[0].props.value, 'https://selfhost.example.test', '切换后表单 origin 必须指向目标实例');
});

test('the registered deployments block is hidden without an onSwitchDeployment handler', async () => {
  const { DeploymentLoginScreen } = await import('./screens/DeploymentLoginScreen.tsx');
  const tree = DeploymentLoginScreen({ deployments: [{ origin: 'https://a.example.test', label: 'A' }], onSignIn: async () => {}, onBeginOidc: async () => {} } as never);
  const titles = JSON.stringify(tree).match(/Registered deployments/g);
  assert.equal(titles, null); // handler 缺失：区块不渲染（无反馈死交互不出现）
});
// HomeScreen 同型断言：onSwitchDeployment 缺失时不渲染 Switch to … 按钮
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx`
Expected: FAIL（切换后 `value` 仍为 `https://stale.example.test`；无 handler 时区块仍渲染）。

- [ ] **Step 3: 最小实现**

```tsx
// DeploymentLoginScreen.tsx —— 渲染前提 + 切换重置
const switchTo = (deployment: { origin: string }): void => {
  setOrigin(deployment.origin); // 表单对齐目标实例：无凭据时 runtime 会回到 deployment-login，组件复用而表单已指向目标
  setEmail(''); setPassword(''); setError(undefined);
  void onSwitchDeployment?.(deployment.origin);
};
{deployments && deployments.length > 0 && onSwitchDeployment ? (
  <View>
    <Text>Registered deployments</Text>
    {deployments.map((deployment) => (
      <Button key={deployment.origin} title={deployment.label} onPress={() => { switchTo(deployment); }} />
    ))}
  </View>
) : null}

// HomeScreen.tsx —— B2-F3：handler 存在才渲染切换入口
{onSwitchDeployment
  ? (otherDeployments ?? []).map((deployment) => (
      <Button key={deployment.origin} title={`Switch to ${deployment.label}`} onPress={() => { void onSwitchDeployment(deployment.origin); }} />
    ))
  : null}
```

- [ ] **Step 4: 实跑确认通过**

Run: `pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/screens/DeploymentLoginScreen.tsx apps/mobile/src/screens/HomeScreen.tsx apps/mobile/src/app-smoke.test.tsx
git commit -m "fix(mobile): reset login form onto the switch target and gate switch entries on handler presence"
```

---

### Task 6: apps/mobile：任务详情错误语义与就地重试（B2-F13、B2-F38；顺手 B2-F16/F5/F39/F40/F41）

**Files:**
- Modify: `apps/mobile/src/app/tasks/detail.tsx:13-22`（错误分类）
- Modify: `apps/mobile/src/screens/TaskDetailScreen.tsx:22-28`（错误分支重试入口）、`:60`（loading 禁用）、`:23`（interruption 文案映射）、`:21`（expanded 按任务隔离）
- Modify: `apps/mobile/src/task-detail-view.ts:26`（messageOf 错误码文案映射，B2-F16）
- Modify: `apps/mobile/src/screens/TasksScreen.tsx:81`（「详情」→ `Details`，B2-F5）
- Test: `apps/mobile/src/app-smoke.test.tsx`、`apps/mobile/src/task-detail-view.test.ts`

**Interfaces:**
- Consumes: `TaskOfficeError`（`{ code: TaskOfficeErrorCode }`，`task-office.ts:114-119`——`super(code)`，message 即裸错误码）；`createTaskDetailController`（`task-detail-view.ts`——`refresh()` 支持从 `view === undefined` 状态恢复）。
- Produces: `TASK_OFFICE_ERROR_COPY: Record<string, string>`（`task-detail-view.ts` 导出，错误码 → 中文文案；Task 10 将为 `'stream-unavailable'` interruption 追加 UI 文案条目）；`TaskDetailScreen` 错误分支渲染 `重试` 按钮调用 `onRefresh`；`INTERRUPTION_COPY`（reason → 文案，替代内部码直出）。

**根因与修复说明：** 同一根因——错误在传播链上被折叠/丢弃，且 UI 无恢复入口：(a) 路由 catch（`detail.tsx:16-18`）把 `open()` 的一切异常折叠为「请先登录并激活空间」，而实际可抛 `TASK_OFFICE_INVALID_INPUT`（路由缺参）与 `TASK_OFFICE_DETAIL_UNAVAILABLE`（detail 端口缺失）——这些场景用户已登录（B2-F13）；(b) `view === undefined` 的早退分支（`TaskDetailScreen.tsx:22-28`）只渲染文本、完全丢弃 `onRefresh`，瞬时网络故障后无就地重试入口（B2-F38——controller.refresh() 本可恢复，`task-detail-view.ts:37-46`）；(c) `messageOf` 对 `TaskOfficeError` 只能取到裸错误码（B2-F16）。

- [ ] **Step 1: 写失败测试**

`task-detail-view.test.ts` 追加：

```ts
test('messageOf renders TaskOfficeError codes as human copy', async () => {
  const { TASK_OFFICE_ERROR_COPY } = await import('./task-detail-view.ts');
  assert.equal(TASK_OFFICE_ERROR_COPY.TASK_OFFICE_INVALID_INPUT, '任务参数缺失（taskId/runId），请从任务列表重新进入。');
  assert.equal(TASK_OFFICE_ERROR_COPY.TASK_OFFICE_DETAIL_UNAVAILABLE, '当前部署未提供任务详情通道。');
  assert.equal(TASK_OFFICE_ERROR_COPY.TASK_OFFICE_SCOPE_CHANGED, '登录状态或活动空间已变化，请重新进入。');
});
```

`app-smoke.test.tsx` 追加：

```ts
// 测试内小工具：递归收集 react stub 元素树（{type, props}）里全部 Button
const flattenButtons = (node: unknown): Array<{ props: { title?: string; onPress?: () => void; disabled?: boolean } }> => {
  if (Array.isArray(node)) return node.flatMap(flattenButtons);
  if (node !== null && typeof node === 'object' && 'type' in (node as Record<string, unknown>)) {
    const element = node as { type: unknown; props?: { children?: unknown } };
    const self = element.type === 'Button' ? [node as never] : [];
    return [...self, ...flattenButtons(element.props?.children)];
  }
  return [];
};

test('the no-view error branch keeps an in-place retry entry', async () => {
  const { TaskDetailScreen } = await import('./screens/TaskDetailScreen.tsx');
  let refreshed = 0;
  const tree = TaskDetailScreen({ view: undefined, loading: false, error: '任务参数缺失（taskId/runId），请从任务列表重新进入。', onRefresh: () => { refreshed += 1; } });
  const retry = flattenButtons(tree).find((button) => button.props.title === '重试');
  assert.ok(retry !== undefined, '错误分支必须提供就地重试入口（controller.refresh 支持从无 view 状态恢复）');
  retry!.props.onPress!();
  assert.equal(refreshed, 1);
});

test('the refresh button is disabled while loading', async () => {
  const { TaskDetailScreen } = await import('./screens/TaskDetailScreen.tsx');
  const view = { taskId: 't', runId: 'r', title: '', lifecycle: 'active', runStatus: 'running', attention: 'none', executionStatus: 'running', settlementStatus: 'pending', revision: 1, cursor: 1, incomplete: false, connection: 'live', timeline: [], duplicateSeqs: [] };
  const tree = TaskDetailScreen({ view, loading: true, error: undefined, onRefresh: () => {} });
  const refresh = flattenButtons(tree).find((button) => button.props.title === '重新同步快照');
  assert.equal(refresh!.props.disabled, true, 'loading 期间刷新按钮必须禁用（B2-F39）');
});
```

源级断言追加：`detail.tsx` 的 catch 不再是统一文案（断言 `TASK_OFFICE_INVALID_INPUT` 与 `TASK_OFFICE_DETAIL_UNAVAILABLE` 出现在 `detail.tsx` 或 `task-detail-view.ts` 的映射/分支中）；`TaskDetailScreen` 不再直出 `interruption.reason` 原始码（断言 `INTERRUPTION_COPY` 存在且 `view.interruption` 处经映射）；`TasksScreen` 的打开按钮文案为 `Details`。

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/task-detail-view.test.ts apps/mobile/src/app-smoke.test.tsx`
Expected: FAIL（`TASK_OFFICE_ERROR_COPY` 未导出；错误分支无按钮；refresh 未禁用；源级断言不匹配）。

- [ ] **Step 3: 最小实现**

```ts
// task-detail-view.ts —— B2-F16：错误码 → 文案（TaskOfficeError 的 message 即 code）
export const TASK_OFFICE_ERROR_COPY: Record<string, string> = {
  TASK_OFFICE_INVALID_INPUT: '任务参数缺失（taskId/runId），请从任务列表重新进入。',
  TASK_OFFICE_DETAIL_UNAVAILABLE: '当前部署未提供任务详情通道。',
  TASK_OFFICE_SCOPE_CHANGED: '登录状态或活动空间已变化，请重新进入。',
  TASK_OFFICE_BACKEND: '服务端暂时不可用，请稍后重试。',
  TASK_OFFICE_DETAIL_CLOSED: '该任务详情已关闭。',
};
const messageOf = (failure: unknown): string => {
  if (failure instanceof TaskOfficeError) return TASK_OFFICE_ERROR_COPY[failure.code] ?? failure.code;
  return failure instanceof Error ? failure.message : String(failure);
};
```

```tsx
// detail.tsx —— B2-F13：按错误码分流，不再统一「请先登录」
import { TaskOfficeError, type TaskDetailViewState } from '@weknora/mobile-core';
import { TASK_OFFICE_ERROR_COPY } from '../../task-detail-view.ts';
...
} catch (error) {
  controller = undefined;
  const fallback = error instanceof TaskOfficeError ? (TASK_OFFICE_ERROR_COPY[error.code] ?? error.code) : '请先登录并激活空间，再打开任务详情。';
  setState({ loading: false, error: fallback });
}

// TaskDetailScreen.tsx —— B2-F38/F39/F40/F41
const INTERRUPTION_COPY: Record<string, string> = { gap: '事件流出现缺口', 'cursor-expired': '同步游标过期', 'stream-error': '实时通道中断', 'stream-ended-nonterminal': '事件流提前结束', 'persist-failed': '本地保存失败' };
const runKey = `${view?.runId ?? ''}`;                       // B2-F41：expanded 以 runId+seq 隔离
useEffect(() => { setExpanded(new Set()); }, [runKey]);       //（组件内；app-smoke 的 react stub 支持 useEffect 收集）
if (view === undefined) {
  return (
    <View>
      <Text>{loading ? '正在读取服务端快照…' : '无法读取该任务'}</Text>
      {error !== undefined && <Text>{error}</Text>}
      <Button title="重试" onPress={onRefresh} />             {/* B2-F38：controller.refresh() 可从无 view 恢复 */}
    </View>
  );
}
...
<Text>{CONNECTION_LABELS[view.connection]}{view.interruption !== undefined ? ` · ${INTERRUPTION_COPY[view.interruption.reason] ?? view.interruption.reason}` : ''}</Text>
...
<Button title="重新同步快照" onPress={onRefresh} disabled={loading} />
```

```tsx
// TasksScreen.tsx:81 —— B2-F5：与同屏英文文案统一
{onOpenTask !== undefined && <Button title="Details" onPress={() => onOpenTask(card)} />}
```

注：`useEffect`/`useState` 均为 RN 组件既有能力；app-smoke 的 react stub（`app-smoke.test.tsx:27`）已提供 hooks 与 `__mount`，直调可测。

- [ ] **Step 4: 实跑确认通过**

Run: `pnpm exec tsx --test apps/mobile/src/task-detail-view.test.ts apps/mobile/src/app-smoke.test.tsx`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/app/tasks/detail.tsx apps/mobile/src/screens/TaskDetailScreen.tsx apps/mobile/src/screens/TasksScreen.tsx apps/mobile/src/task-detail-view.ts apps/mobile/src/task-detail-view.test.ts apps/mobile/src/app-smoke.test.tsx
git commit -m "fix(mobile): honest task-detail error copy with in-place retry and per-run expansion"
```

---

### Task 7: mobile-core：hydrate 的 epoch/串行化守卫（B2-F24；顺手 B2-F26/F27）

**Files:**
- Modify: `packages/mobile-core/src/task-office/task-detail.ts:152-186`（hydrate 守卫）、`:195-207`（startStream 旧事件丢弃）、`:222`（duplicateSeqs 有界）、`:1-60`（`input.taskId` 契约注释）
- Test: `packages/mobile-core/src/task-office/task-detail.test.ts`

**Interfaces:**
- Consumes: `streamEpoch`/`abortStream()`（既有：`abortStream` 递增 epoch 并 abort controller）；`createScriptedTaskStream`/`createScenarioTaskDetailBackend`（测试夹具，`in-memory-task-detail.ts`）。
- Produces: `hydrate` 在每个 await 挂起点后校验 `epoch === streamEpoch`（被取代即抛 `TASK_OFFICE_SCOPE_CHANGED` 或静默返回 `current`，不覆写状态）；`startStream` 的 `onEvent`/`onControl` 回调在 enqueue 前校验自身 epoch（被取代流的迟到事件直接丢弃）；`duplicateSeqs` 追加式截断至最近 50 条。Task 10 依赖本任务后的 `task-detail.ts` 稳定结构。

**根因与修复说明：** `hydrate` 对 `detail/events/committedCursor` 的赋值（`:158-161`）只有 `leaseActive` 检查、无 epoch 守卫；`interrupt()` 触发的有界自动重同步（`:192`）与用户显式 `resync()`（`:275-280`）并发时两个 hydrate 交错，较慢的旧响应后到达会以裁剪后事件集覆写 store、`committedCursor` 回退；且 hydrate 未挂到 `chain`，已入队旧流 `processEvent` 可在 await 挂起点间插入执行（`processEvent` 无 epoch 检查，`:208-234`）。对照 `startStream`（`:195-207`）已有 epoch 守卫——把 hydrate 与流回调对齐到同一守卫模式。顺手：B2-F27（`duplicateSeqs` 只增不减——追加处截断）；B2-F26（`input.taskId` 从未被读取——补契约注释说明 persist 以服务端权威 `detail.taskId` 为准，构造参数中的 taskId 仅用于调用方关联，不删参数以免破坏 `open()` 签名）。

- [ ] **Step 1: 写失败测试（追加到 task-detail.test.ts，用既有 scenario 夹具）**

```ts
// 复用本文件既有夹具：leased()/detail$()/event$()/officeWithDetail()/settle()（task-detail.test.ts:17-47）
test('a slower stale hydrate does not roll back the committed cursor or the event set', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  let call = 0;
  const { office } = officeWithDetail(leaseRef, {
    detail: async () => {
      call += 1;
      if (call === 1) { await settle(8); return detail$(); } // 旧响应：慢，watermark 2
      return detail$({ watermark: 4, events: [event$(1, 'run.started'), event$(2, 'tool.started'), event$(3, 'text.delta'), event$(4, 'run.completed')] }); // 新响应：watermark 4
    },
  });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  const stale = handle.hydrate();                     // 旧请求先发（挂起多个宏任务）
  await settle(1);
  await handle.resync();                              // 新请求后发先回：watermark 4 已提交
  await stale.catch(() => undefined);                 // 旧响应迟到到达（修复后抛 SCOPE_CHANGED 被吞）
  const view = handle.view()!;
  assert.equal(view.cursor, 4, '迟到的旧 hydrate 不得回退 committedCursor');
  assert.deepEqual(view.timeline.map((entry) => entry.seq), [1, 2, 3, 4], '不得以旧响应的事件集覆写新事件集');
});

test('events from a superseded stream are dropped after resync', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { backend, office } = officeWithDetail(leaseRef, {
    detail: async () => detail$({ watermark: 2 }),
    stream: () => createScriptedTaskStream(),          // 每次开一条新脚本流（backend.streams 依序记录）
  });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();                              // 开流 A = backend.streams[0]
  await handle.resync();                               // 开流 B = backend.streams[1]，A 被取代
  backend.streams[0]!.emit(event$(9, 'tool.started')); // 旧流迟到事件（seq 错位）：必须被丢弃，不得触发 gap interrupt
  backend.streams[0]!.fail(new Error('A_TRANSPORT_ABORTED')); // 旧流迟到失败：不得打断 B
  await settle();
  const view = handle.view()!;
  assert.equal(view.connection, 'live', '被取代流的迟到事件/失败不得打断新流');
  assert.deepEqual(view.timeline.map((entry) => entry.seq), [1, 2]);
});
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/task-detail.test.ts`
Expected: FAIL（旧响应覆写：cursor 回退到 5；旧流事件触发 interrupt）。

- [ ] **Step 3: 最小实现（task-detail.ts）**

```ts
const hydrate = async (): Promise<TaskDetailView> => {
  requireOpen();
  const lease = requireLease();
  abortStream();                          // 进入即递增 streamEpoch：作废在途流与并发旧 hydrate
  const epoch = streamEpoch;
  if (detail !== undefined) notify('syncing');
  const fetched = await wrap(() => ports.backend.detail(input.runId));
  if (epoch !== streamEpoch || !leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
  detail = fetched;
  const persisted = await ports.store.load(input.runId).catch(() => undefined);
  if (epoch !== streamEpoch) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');   // 第二个挂起点同样守卫
  events = mergeEventHistory(persisted?.events ?? [], fetched.events, fetched.watermark).slice(-TASK_DETAIL_HISTORY_LIMIT);
  committedCursor = fetched.watermark;
  duplicateSeqs = [];
  interruption = undefined;
  try {
    await persist(committedCursor, events);
  } catch (cause) {
    interruption = { reason: 'persist-failed', message: cause instanceof Error ? cause.message : String(cause) };
  }
  if (epoch !== streamEpoch) return current!;           // persist 期间被取代：不再 startStream/notify
  ... // 终态/interrupted 判定与 startStream() 保持原样
};

// startStream —— 旧流迟到事件在 enqueue 前丢弃：
onEvent: (event) => { if (epoch === streamEpoch) chain = chain.then(() => processEvent(event)).catch(() => undefined); },
onControl: (frame) => { if (epoch === streamEpoch) chain = chain.then(() => processControl(frame)).catch(() => undefined); },

// processEvent :222 —— B2-F27：重复 seq 可观测但有界
duplicateSeqs = [...duplicateSeqs, event.seq].slice(-50);

// createTaskDetail 头部 —— B2-F26 契约注释：
// input.taskId 仅用于调用方关联；持久化写入服务端权威的 detail.taskId（hydrate 后 detail!.taskId），
// 故本函数不读取 input.taskId，这是有意为之。
```

注：`interrupt()`/`resync()` 已各自先 `abortStream()` 再 `await hydrate()`，hydrate 入口的 `abortStream()` 是幂等加固；行为不变路径（单次 hydrate）的 epoch 恒等，既有 16 个测试必须保持全绿。

- [ ] **Step 4: 实跑确认通过 + 全量回归**

Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/task-detail.test.ts`
Expected: PASS（原 16 + 新 2；既有用例证明单流行为未回归）。
Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/`
Expected: PASS（task-office/in-memory-task-detail/task-timeline 同包全绿）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/task-office/task-detail.ts packages/mobile-core/src/task-office/task-detail.test.ts
git commit -m "fix(mobile-core): guard task-detail hydrate and stream callbacks with stream epochs"
```

---

### Task 8: SSE 适配层：错误形态与读取循环清理（B2-F29、B2-F30；顺手 B2-F8/F37）

**Files:**
- Modify: `apps/mobile/src/adapters/sse-stream.ts:16-29`
- Modify: `packages/api-client/src/mobile/task-office.ts:129-150`（畸形帧守卫 + 契约码注释）
- Test: `apps/mobile/src/adapters/sse-stream.test.ts`、`packages/api-client/src/mobile/task-office.test.ts`

**Interfaces:**
- Consumes: `ApiError`（`packages/api-client/src/errors.ts:24-37`，`{ status?, code, message, requestId?, details? }`；`index.ts:3` re-export；apps/mobile 已依赖 `@weknora/api-client`——`apps/mobile/package.json:17`）。`errorFromResult` 的 code 兜底约定：`` `HTTP_${status}` ``（`errors.ts:60`）。
- Produces: `streamAuthorizedSse` 非 2xx 时抛 `new ApiError({ status, code: \`HTTP_${status}\`, message })`（`instanceof ApiError === true`、含 `code`）；读取循环以 `try/finally` 包裹，`onChunk` 同步抛出时 `reader.cancel()` 释放连接后原错误继续传播；api-client 流回调对 `JSON.parse` 畸形帧抛带 `TASK_STREAM_MALFORMED_FRAME` 消息的错误（作为 transport error 进入 `streamFailed → interrupted`，不再裸抛 `SyntaxError`）。

**根因与修复说明：** 同一根因——SSE 适配层以手工形态绕过真实错误类型、以裸循环绕过资源清理：(a) `sse-stream.ts:17-20` 伪造 ApiError 形态（改 `name` + 挂 `status`），缺 `code` 字段且与类定义漂移（消费方 `instanceof`/`error.code` 将静默失效，B2-F29）；(b) `:22-29` 读取循环无 `try/finally`，`onChunk` 同步抛出（runtime `guardedChunk` 的 `RUNTIME_SCOPE_CHANGED`，`mobile-runtime.ts:382-385`；api-client 回调对 `frame.data` 直接 `JSON.parse` 遇畸形数据即抛，`task-office.ts:136/:140`）时 reader 既不 cancel 也不释放（B2-F30）。顺手：B2-F8（control 帧 `JSON.parse` 断言无守卫——并入畸形帧守卫）；B2-F37（`'TASK_STREAM_CURSOR_EXPIRED'` 手工挂 code 的跨包契约——补两侧契约注释，不改字符串本身以免破坏 mobile-core 的 `streamFailed` 识别）。

- [ ] **Step 1: 写失败测试**

`sse-stream.test.ts` 追加（顶部 import 补 `import { ApiError } from '@weknora/api-client';`）：

```ts
test('a non-2xx response rejects with a real ApiError carrying code', async () => {
  const fetchLike = async () => new Response('nope', { status: 503 });
  await assert.rejects(
    streamAuthorizedSse('https://weknora.example.test', { method: 'GET', path: '/api/x' }, 'token', () => {}, fetchLike),
    (error: unknown) => error instanceof ApiError && error.status === 503 && error.code === 'HTTP_503',
  );
});

test('a throwing onChunk cancels the reader before the error propagates', async () => {
  const cancelled: boolean[] = [];
  const body = new ReadableStream<Uint8Array>({
    start(controller) { controller.enqueue(new TextEncoder().encode('data: x\n\n')); /* 保持打开：只有 cancel 能释放 */ },
    cancel() { cancelled.push(true); },
  });
  const fetchLike = async () => new Response(body, { status: 200, headers: { 'content-type': 'text/event-stream' } });
  await assert.rejects(
    streamAuthorizedSse('https://weknora.example.test', { method: 'GET', path: '/api/x' }, 'token', () => { throw new Error('RUNTIME_SCOPE_CHANGED'); }, fetchLike),
    /RUNTIME_SCOPE_CHANGED/,
  );
  assert.deepEqual(cancelled, [true], 'onChunk 同步抛出时 reader 必须 cancel，连接不得保持打开');
});
```

`task-office.test.ts` 追加（复用既有 `createTaskOfficeRemote` + `stream: async (input, onChunk) => onChunk(wire)` 直喂夹具形态，见该文件 :126-150）：

```ts
test('a malformed SSE frame fails the stream as a transport error, not a bare SyntaxError', async () => {
  const wire = 'event: control\ndata: {not-json}\n\n';
  const remote = createTaskOfficeRemote({
    origin: 'https://weknora.example.test',
    request: async () => { throw new Error('no JSON call expected'); },
    stream: async (_input, onChunk) => { onChunk(wire); },
  });
  await assert.rejects(
    remote.stream({ runId: 'r1', cursor: 5, signal: new AbortController().signal, onEvent: () => {}, onControl: () => {} }),
    /TASK_STREAM_MALFORMED_FRAME/,
  );
});
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/adapters/sse-stream.test.ts packages/api-client/src/mobile/task-office.test.ts`
Expected: FAIL（`instanceof ApiError` 为 false / `code` undefined；`cancelled` 为空；畸形帧抛裸 `SyntaxError`）。

- [ ] **Step 3: 最小实现**

```ts
// sse-stream.ts
import { ApiError } from '@weknora/api-client';
...
  if (!response.ok || !response.body) {
    throw new ApiError({ status: response.status, code: `HTTP_${response.status}`, message: `workbench event stream failed with HTTP ${response.status}` });
  }
  const reader = (response.body as ReadableStream<Uint8Array>).getReader();
  const decoder = new TextDecoder();
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      if (value !== undefined && value.length > 0) onChunk(decoder.decode(value, { stream: true }));
    }
    onChunk(decoder.decode());
  } finally {
    await reader.cancel().catch(() => undefined); // 同步抛出/传输中断都释放连接；正常完成时 cancel 幂等
  }

// packages/api-client/src/mobile/task-office.ts —— createServerSentEventParser 回调包守卫
const parser = createServerSentEventParser((frame) => {
  try {
    ... // 原有 control/event 解析
  } catch {
    throw new Error(`TASK_STREAM_MALFORMED_FRAME: ${frame.event ?? 'message'}`);
  }
});
// :142-146 'TASK_STREAM_CURSOR_EXPIRED' 处补注释：该字符串是 api-client → mobile-core 的跨包契约码
//（mobile-core task-detail.ts streamFailed 按 error.code 识别），不得改名或改用 ApiError 形态。
```

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `pnpm exec tsx --test apps/mobile/src/adapters/sse-stream.test.ts packages/api-client/src/mobile/task-office.test.ts`
Expected: PASS（2+4 → 2+2 新增全绿）。
Run: `pnpm exec tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts packages/api-client/src/mobile/task-office-detail.integration.test.ts`
Expected: PASS（runtime 401 重试与远端 409 映射消费 ApiError 形态——真 ApiError 的 `name === 'ApiError'` 且 `status` 保留，既有消费路径不回归）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/adapters/sse-stream.ts apps/mobile/src/adapters/sse-stream.test.ts packages/api-client/src/mobile/task-office.ts packages/api-client/src/mobile/task-office.test.ts
git commit -m "fix(mobile): sse adapter throws real ApiError and always releases the reader"
```

---

### Task 9: apps/mobile：集成 smoke 的证据契约与资源清理收口（B2-F14；顺手 B2-F15）

**Files:**
- Modify: `apps/mobile/src/task-detail-integration-smoke.ts:49-102`（try/catch/finally 收口）、`:22-40`（config 主机防线）
- Modify: `apps/mobile/src/runtime-integration-smoke.ts:111`（导出 `disallowedDeploymentHost`，B2-F15）
- Test: Create `apps/mobile/src/task-detail-integration-smoke.test.ts`

**Interfaces:**
- Consumes: `disallowedDeploymentHost(hostname, variable): string | undefined`（`runtime-integration-smoke.ts:111`，模块私有——本任务导出）；`TaskDetailIntegrationEvidence.opened: 'hydrated' | 'no-tasks' | 'failed'`（既有契约）。
- Produces: `runTaskDetailIntegration` **total**（从不 reject）：任何步骤异常 → `evidence.opened === 'failed'` + 新增可选字段 `failure?: string`（仅 error message，证据契约仍无凭据字段）；所有路径（含 no-tasks/未授权早退）经 `finally` 执行 `handle?.close(...)` 与 `runtime.dispose()`；`taskDetailIntegrationConfig` 增加主机防线（localhost/环回/私网/链路本地/保留地址 → `invalid`）。

**根因与修复说明：** 同一根因——smoke 的异常路径绕过证据契约且清理不一致：`signIn`(:69) 之外 `office.tasks()`(:81)/`open()`(:84)/`hydrate()`(:85) 均无 try/catch，任一抛错整函数 reject（调用方 `task-office-detail.integration.test.ts:20` `await` 无 catch），未处理 rejection 打断测试且不留证据（`opened:'failed'` 实际只在初始值/未授权早退产生）；no-tasks(:82)/未授权早退直接 `return` 不执行 `runtime.dispose()`，与 happy path(:101-102) 清理不一致。顺手 B2-F15：config 校验缺主机防线（对照 `runtime-integration-smoke.ts:147` 的语义）。B2-F46（alt URL 校验重复）仅记录延期（见附录 A）。

- [ ] **Step 1: 写失败测试（新建 task-detail-integration-smoke.test.ts）**

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { taskDetailIntegrationConfig } from './task-detail-integration-smoke.ts';

test('the config rejects loopback, private and reserved deployment hosts as invalid', () => {
  const vars = { WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://127.0.0.1:8080', WEKNORA_MOBILE_TEST_EMAIL: 'user@example.test', WEKNORA_MOBILE_TEST_PASSWORD: 'pw' };
  const verdict = taskDetailIntegrationConfig(vars);
  assert.equal(verdict.enabled, false);
  assert.equal(verdict.enabled === false && verdict.disposition, 'invalid');
  for (const host of ['https://localhost', 'https://10.0.0.5', 'https://192.168.1.4', 'https://[fe80::1]', 'https://169.254.1.1']) {
    const rejected = taskDetailIntegrationConfig({ ...vars, WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: host });
    assert.equal(rejected.enabled === false && rejected.disposition, 'invalid', host);
  }
});

test('a public https origin still enables the integration config', () => {
  const verdict = taskDetailIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://weknora.example.org', WEKNORA_MOBILE_TEST_EMAIL: 'user@example.test', WEKNORA_MOBILE_TEST_PASSWORD: 'pw' });
  assert.equal(verdict.enabled, true);
});
```

源级断言（防回归，可入 app-smoke 或本文件读源码）：`runTaskDetailIntegration` 含 `finally` 块且块内同时出现 `close(` 与 `dispose()`；`task-detail-integration-smoke.ts` 引用 `disallowedDeploymentHost`。

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/task-detail-integration-smoke.test.ts`
Expected: FAIL（localhost 目前判 enabled:true——无主机防线；源级断言不匹配）。

- [ ] **Step 3: 最小实现**

```ts
// runtime-integration-smoke.ts:111 —— 仅在函数声明前加 export 关键字，函数体不动
export function disallowedDeploymentHost(hostname: string, variable: string): string | undefined { /* 函数体保持原样 */ }

// task-detail-integration-smoke.ts —— config 校验追加（在 HTTPS origin 校验之后）：
const hostRejection = disallowedDeploymentHost(parsed.hostname, 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL');
if (hostRejection) return { enabled: false, disposition: 'invalid', reason: hostRejection };

// runTaskDetailIntegration 结构收口：以下骨架中「原样保留」= 当前 :52-100 的装配与业务语句逐字保留，
// 只做三处结构变化：整体包 try/catch/finally、handle 提升到 try 外声明、早退路径不再自带清理。
export async function runTaskDetailIntegration(config: Extract<TaskDetailIntegrationConfig, { enabled: true }>): Promise<TaskDetailIntegrationEvidence> {
  const evidence: TaskDetailIntegrationEvidence = { deploymentOrigin: config.deploymentOrigin, opened: 'failed', resync: 'skipped', commandTimestamp: new Date().toISOString() };
  const runtime = createMobileRuntime({ /* :52-77 的装配逐字保留 */ });
  let handle: TaskHandle | undefined;
  try {
    const snapshot = await runtime.signIn({ deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' }, email: config.email, password: config.password });
    if (snapshot.surface !== 'authorized' || !snapshot.deployment) return evidence;  // 早退也经 finally 清理
    // :74-79 的 remote/office 装配逐字保留
    const page = await office.tasks({});
    if (page.items.length === 0) { evidence.opened = 'no-tasks'; return evidence; }
    const target = page.items[0]!;
    handle = office.open({ taskId: target.taskId, runId: target.runId });
    const view = await handle.hydrate();
    await settle();
    evidence.opened = 'hydrated';
    // :87-99 的 settled 取值与既有 resync try/catch 逐字保留
  } catch (error) {
    evidence.opened = 'failed';
    evidence.failure = error instanceof Error ? error.message : String(error);   // 新增可选字段：异常路径也留证据（不含凭据）
  } finally {
    try { handle?.close('integration-complete'); } catch { /* 清理不得再抛 */ }
    runtime.dispose();
  }
  return evidence;
}
// TaskDetailIntegrationEvidence 增补：failure?: string;
```

调用方 `packages/api-client/src/mobile/task-office-detail.integration.test.ts:20` 的 `await runTaskDetailIntegration(config)` 在函数 total 后不再需要 catch——保持不动，并在该行补注释说明此契约。

- [ ] **Step 4: 实跑确认通过**

Run: `pnpm exec tsx --test apps/mobile/src/task-detail-integration-smoke.test.ts packages/api-client/src/mobile/task-office-detail.integration.test.ts`
Expected: PASS（新建测试全绿；integration 测试本地 opt-in skip，不得伪造通过）。
Run: `pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx`
Expected: PASS（runtime-integration-smoke 导出符号未破坏其源级断言）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/task-detail-integration-smoke.ts apps/mobile/src/task-detail-integration-smoke.test.ts apps/mobile/src/runtime-integration-smoke.ts
git commit -m "test(mobile): make task-detail integration smoke total with consistent dispose and host guard"
```

---

### Task 10: mobile-core：授权流通道不可用的诚实语义（B2-F34；顺手 B2-F19/F20/F45）

**Files:**
- Modify: `packages/mobile-core/src/runtime/mobile-runtime.ts:377-388`（`authorizedEventStream` 错误分流）
- Modify: `packages/mobile-core/src/runtime/ports.ts:56-58`（`AuthorizedStreamTransport` 契约注释，B2-F20）
- Modify: `packages/mobile-core/src/task-office/task-detail.ts`（`streamFailed` 分类 + `TaskInterruptionReason` 新值）
- Modify: `apps/mobile/src/screens/TaskDetailScreen.tsx`（`INTERRUPTION_COPY` 追加 `'stream-unavailable'` 条目——Task 6 建立的映射）
- Test: `packages/mobile-core/src/runtime/mobile-runtime.test.ts`、`packages/mobile-core/src/task-office/task-detail.test.ts`

**Interfaces:**
- Consumes: `ports.authorizedStream?: (origin) => AuthorizedStreamTransport | undefined`（per-origin 工厂；平台无流式 fetch 时返回 `undefined`，fail-closed，`ports.ts:57` 注释确认）；Task 6 的 `INTERRUPTION_COPY`、Task 7 后的 `task-detail.ts` 结构。
- Produces: `authorizedEventStream` 在「面未授权/无活动 deployment」时仍抛 `RUNTIME_UNAUTHORIZED`；在「已授权但该 origin 无流通道」时抛 `RUNTIME_STREAM_UNAVAILABLE`；`TaskInterruptionReason` 新增 `'stream-unavailable'`，`streamFailed` 对其**不触发自动 resync**（REST 详情仍可用，用户显式刷新可再试）；`AuthorizedStreamTransport` 契约注释补第三种情形（B2-F20）。`resourceShelf()` 接口契约注释与 authenticate 的 read-only 行为对齐（B2-F45，注释级）。

**根因与修复说明：** 组合层以闭包无条件把 `authorizedEventStream` 接为 office 的 stream 通道（`composition.ts:106`——闭包无法预知 transport 是否解析成功，见「差异记录 2」），runtime 对 `undefined` 传输统一抛 `'RUNTIME_UNAUTHORIZED'`（`mobile-runtime.ts:379-380`）——用户实际已授权、仅流通道不可用，错误语义误导，且 `task-detail` 按该错误触发 `AUTO_RESYNC_LIMIT(2)` 次徒劳的自动 resync（每次重走 REST 详情 + 流）。修复在 runtime 侧分流错误码，task-detail 侧识别新码并停止无意义重试。顺手 B2-F19（fail-closed 的 undefined 通道如何在面上呈现——由本任务的错误分流覆盖）。

- [ ] **Step 1: 写失败测试**

`mobile-runtime.test.ts` 追加（复用本文件既有 `ports()`/`fakeStore()`/`remote()`/`DEPLOYMENT` 夹具；对照 :699 的未授权用例——该用例不 signIn，本用例 signIn 后无 authorizedStream 端口）：

```ts
test('authorizedEventStream distinguishes an unavailable stream channel from unauthorized', async () => {
  const runtime = createMobileRuntime(ports(fakeStore(), () => remote())); // 无 authorizedStream 端口
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  await assert.rejects(
    runtime.authorizedEventStream({ method: 'GET', path: '/api/v1/workbench/executions/r1/events?version=2' }, () => {}),
    /RUNTIME_STREAM_UNAVAILABLE/,
  );
});
```

`task-detail.test.ts` 追加（复用 `officeWithDetail`/`leased()`/`detail$()` 夹具与既有 `queueMicrotask(() => failing.fail(...))` 失败流模式，见该文件 :289-296）：

```ts
test('an unavailable stream channel interrupts without futile auto-resyncs', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  let detailCalls = 0;
  const { backend, office } = officeWithDetail(leaseRef, {
    detail: async () => { detailCalls += 1; return detail$(); },
    stream: () => {
      const failing = createScriptedTaskStream();
      queueMicrotask(() => failing.fail(new Error('RUNTIME_STREAM_UNAVAILABLE')));
      return failing;
    },
  });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();
  await settle(30);                                   // 给自动 resync 的窗口（对照既有 bounded-resync 用例的 settle(30)）
  assert.equal(handle.view()!.connection, 'interrupted');
  assert.equal(handle.view()!.interruption?.reason, 'stream-unavailable');
  assert.equal(backend.streams.length, 1, '流通道不可用不是瞬时故障：不得触发 AUTO_RESYNC_LIMIT 次徒劳 resync');
  assert.equal(detailCalls, 1);
});
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts packages/mobile-core/src/task-office/task-detail.test.ts`
Expected: FAIL（runtime 用例当前 reject `RUNTIME_UNAUTHORIZED`；task-detail 用例当前 reason 是 `'stream-error'` 且 `backend.streams.length === 3`、`detailCalls === 3`——初始流 + 2 次徒劳自动 resync）。

- [ ] **Step 3: 最小实现**

```ts
// mobile-runtime.ts :377-388 —— 分流
const deployment = activeDeployment;
const authorized = deployment !== undefined && state.surface === 'authorized';
const transport = authorized ? ports.authorizedStream?.(deployment.origin) : undefined;
if (!authorized) throw new Error('RUNTIME_UNAUTHORIZED');
if (!transport) throw new Error('RUNTIME_STREAM_UNAVAILABLE'); // 已授权但平台/该实例无流通道：REST 仍可用

// task-detail.ts —— reason 类型追加 'stream-unavailable'；streamFailed 分类：
const streamFailed = async (error: unknown): Promise<void> => {
  if (closed) return;
  const message = error instanceof Error ? error.message : String(error);
  if (message === 'RUNTIME_STREAM_UNAVAILABLE') {
    interruption = { reason: 'stream-unavailable', message: '此部署未提供实时流通道，可手动刷新同步' };
    autoResyncs = AUTO_RESYNC_LIMIT;   // 通道缺失非瞬时故障：跳过自动 resync，显式 resync() 仍可再试
    notify('interrupted');
    return;
  }
  ... // 原有 cursor-expired / stream-error 分支保持
};

// ports.ts :57 契约注释追加：transport 解析失败（返回 undefined）时 runtime 以
// 'RUNTIME_STREAM_UNAVAILABLE' 拒绝——面已授权、仅流通道不可用（B2-F20）。

// TaskDetailScreen.tsx —— INTERRUPTION_COPY 追加：'stream-unavailable': '此部署暂无实时通道，可手动刷新'
```

- [ ] **Step 4: 实跑确认通过 + 跨包回归**

Run: `pnpm exec tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts packages/mobile-core/src/task-office/task-detail.test.ts packages/mobile-core/src/runtime/runtime-deployments.test.ts apps/mobile/src/app-smoke.test.tsx`
Expected: PASS（runtime 37+1、task-detail 18+1、deployments 15、app-smoke 全绿——证明新 reason 值不破坏 Task 6 的映射与 Task 2 的契约）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/runtime/mobile-runtime.ts packages/mobile-core/src/runtime/ports.ts packages/mobile-core/src/task-office/task-detail.ts packages/mobile-core/src/task-office/task-detail.test.ts packages/mobile-core/src/runtime/mobile-runtime.test.ts apps/mobile/src/screens/TaskDetailScreen.tsx
git commit -m "fix(mobile-core): distinguish an unavailable stream channel from unauthorized and stop futile resyncs"
```

---

## 附录 A：lowWorthFixing 项的处置映射

**折叠进任务（同文件、低风险，随所属任务实现与验收）：**

| 项 | 折叠进 | 处置 |
|---|---|---|
| B2-F2 登录屏 Registered 区块未判断 onSwitch | Task 5 | 渲染前提 `&& onSwitchDeployment` |
| B2-F3 HomeScreen 同型死交互 | Task 5 | 同上 |
| B2-F36 onSwitchDeployment 仅 await 无 catch | Task 2 | switchDeployment 从不 reject 后自然安全（Task 2 验收含「不 reject」断言） |
| B2-F21 in-memory registry label 未归一化 | Task 3 | trim + origin 兜底，与 SecureStore 版一致 |
| B2-F22 deploymentStore.write 与 registry.upsert 无回滚 | Task 2 | upsert 失败隔离（不阻塞授权）；deploymentStore.write 的写序/回滚语义保持并注释——完整回滚属新设计，延期 |
| B2-F43 forgetDeployment 失败未包含 | Task 2 | try/catch 收口 |
| B2-F44 switchDeployment 同 origin 短路 | Task 2 | 见 Task 2 Step 1 第 5 个测试 |
| B2-F5 「详情」中英混杂 | Task 6 | `Details` |
| B2-F16 messageOf 裸错误码 | Task 6 | `TASK_OFFICE_ERROR_COPY` 映射 |
| B2-F39 刷新按钮未随 loading 禁用 | Task 6 | `disabled={loading}` |
| B2-F40 interruption.reason 内部码直出 | Task 6 | `INTERRUPTION_COPY` |
| B2-F41 expanded 未按任务隔离 | Task 6 | runId 变化时重置 |
| B2-F26 input.taskId 未被读取 | Task 7 | 契约注释（persist 以服务端权威 detail.taskId 为准） |
| B2-F27 duplicateSeqs 无界 | Task 7 | 追加截断至最近 50 条 |
| B2-F8 control 帧 JSON.parse 无守卫 | Task 8 | `TASK_STREAM_MALFORMED_FRAME` transport error |
| B2-F37 跨包契约码注释缺失 | Task 8 | 两侧契约注释（不改字符串） |
| B2-F15 smoke 缺主机防线 | Task 9 | 导出并复用 `disallowedDeploymentHost` |
| B2-F19 authorizedStream undefined 呈现语义 | Task 10 | 错误分流覆盖 |
| B2-F20 AuthorizedStreamTransport 契约注释过期 | Task 10 | 补第三种情形 |
| B2-F45 resourceShelf 契约注释过期 | Task 10 | 注释级对齐 |

**明确延期（记录理由，不在本批次）：**

- **B2-F23 生产缺持久化任务投影存储**：`TASK_DETAIL_HISTORY_LIMIT = 200` 事件的投影 JSON 必然超出 SecureStore 单值约 2KB 的限制，截断存储会改变合并语义；需要存储选型决策（SecureStore 截断 vs SQLite/文件 + 加密），应先写 ADR 再实现。本批次不实现半吊子方案；`task-office.ts:101` 的「缺省为 in-memory」注释已如实描述现状（不算静默——但组合根未显式传 store 的接线意图应在 ADR 落地时一并补上）。
- **B2-F4 ReadOnlyScreen 与 resources.tsx 控制器生命周期重复**：提取共享宿主组件是结构化重构，超出修复批次；漂移风险已由两处同型测试兜底。
- **B2-F6 handle 从有值变 undefined 时 dispose controller**：复核 `detail.tsx` effect 依赖为 `[taskId, runId]`，handle 变化不经由 effect 重入；scope 失效路径由 Task 6 的错误分类覆盖。判定为当前无实际触发路径，不动。
- **B2-F11 降级引导文案承诺 switch 入口但只渲染 Sign out**：给 `ReadOnlyScreen` 增加 `onSwitchDeployment` 传参是新 UI 功能（含组合根接线），超出修复批次；文案维持现状。
- **B2-F28 事件类型字面量三处平行维护**：统一分类表是重构，需独立任务与全量回归。
- **B2-F35 taskOffices 按 origin 缓存不失效**：复核 `composition.ts:103-112`——remote 闭包委托 `activeRuntime.authorizedRequest/EventStream`，scope 检查由 runtime 内部 epoch 执行，`lease()` 闭包每次取当前 lease，`open()` 的 `requireLease` 兜底 fail-closed；缓存不失效不产生越权，仅存留实例句柄。判定为可接受，不动（如后续出现句柄泄漏证据再立任务）。
- **B2-F46 alt 部署 URL 校验重复**：等 B2-F15 的导出复用模式在更多调用点出现时统一提取 helper，本批次仅复用 `disallowedDeploymentHost` 一处。

## 附录 B：批次验收（全部任务完成后）

1. `go test ./internal/handler/session/ ./internal/application/repository/` —— ok。
2. `pnpm exec tsx --test packages/mobile-core/src/runtime/runtime-deployments.test.ts packages/mobile-core/src/runtime/mobile-runtime.test.ts packages/mobile-core/src/runtime/runtime-readonly.test.ts packages/mobile-core/src/task-office/task-detail.test.ts packages/mobile-core/src/task-office/task-office.test.ts` —— 全绿。
3. `pnpm exec tsx --test apps/mobile/src/adapters/deployment-registry.test.ts apps/mobile/src/adapters/sse-stream.test.ts apps/mobile/src/task-detail-view.test.ts apps/mobile/src/task-detail-integration-smoke.test.ts apps/mobile/src/app-smoke.test.tsx` —— 全绿。
4. `pnpm exec tsx --test packages/api-client/src/mobile/task-office.test.ts packages/api-client/src/mobile/task-office-detail.integration.test.ts` —— 全绿（integration 本地无环境时按既有 opt-in 语义 skip，不得伪造通过）。
5. 抽查清单（人工复核，无需自动化）：报告 B2-F12/F17/F31/F32/F33/F34/F38 各对应的「用户可见症状」在修复描述中逐条对上（404 → 200；切换不残留旧 origin；脏数据不清空注册表；登录不被注册表写失败降级；无流通道时不再误导为未授权且不再徒劳 resync；错误分支有重试按钮）。
