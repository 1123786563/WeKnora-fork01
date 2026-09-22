# T03：Resource Shelf 展示当前空间可用资源（Issue #33）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 移动端新增 Resources 资源页，端到端展示当前 Active Tenant 的 Available Agent、可发现知识与可用 Connection 摘要，三态（supported/unavailable/forbidden）必须解释原因；撤权（403）或 scope/capability 变化后资源投影立即失效（fail closed，不回填缓存）；Screen 只消费 Resource Shelf Interface。

**Architecture:** 按 `docs/specs/2026-09-20-mobile-module-seams.md` §6 新增 mobile-core 第三个深 Module「Resource Shelf」：`createResourceShelf(ports).open({ lease })` 返回 `ResourceShelfHandle`（browse/selection/subscribe/close），投影全部是领域类型（复用 `@weknora/domain/mobile` 的 `toAgentOptions`/`toKnowledgeResource`/`toConnectionResource` 纯策略），不暴露 wire DTO 与 token。传输经 Resource Backend Port：api-client 新增 WeKnora 适配 `createMobileResourceRemote`（组合既有三个 Viewer+ 读端点 `GET /api/v1/agents`、`GET /api/v1/knowledge-bases`、`GET /api/v1/apps/connections`，**不新增后端端点**）。生命周期归 Mobile Runtime（spec §4.1「其他 Module 的启动、失效和关闭」）：`MobileRuntimePorts.resourceShelf?: { remoteFor }`，Runtime 在每次授权面铸造 lease 时开一个 shelf、在每次 scope 变化（`revoke()`——deployment-change/tenant-switch/sign-out/dispose 单一咽喉，#32 已建立）同步 `close(reason)`；token 供给是包内 seam `accessTokenFor(origin, { refresh })`（Runtime 持有凭据 + refresh 单飞，token 不出 mobile-core），browse 对 401 恰好重试一次、对 403 判 forbidden 并广播 `authorization-revoked`。apps/mobile 只加 Resources 页 + 组装，控制器 `createResourceShelfController` 在失效事件后重取并按代次丢弃迟到结果。

**Tech Stack:** TypeScript（`packages/domain`、`packages/mobile-core`、`packages/api-client`、`apps/mobile`/Expo RN）、node:test + tsx（测试运行器，与既有 `mobile-runtime.test.ts`、`app-smoke.test.tsx` 一致）、expo-router 文件路由（`/resources`）。后端复用既有端点：`GET /api/v1/agents`（`internal/router/routes_agent.go:39`，Viewer+，响应 `{ success, data: CustomAgent[], disabled_own_agent_ids }`，`internal/handler/custom_agent.go:266-270`；`CustomAgent` 字段含 `id/name/description/is_builtin/config.system_prompt`，`internal/types/custom_agent.go:65-104`）、`GET /api/v1/knowledge-bases`（`internal/router/routes_knowledge.go:206`，Viewer+，行含 `id/name/knowledge_count/updated_at/is_processing/vector_store_status`，`internal/types/knowledgebase.go:59-140` + `internal/handler/knowledgebase.go:109-141`）、`GET /api/v1/apps/connections`（`internal/router/routes_app_connectors.go:44`，GET 过写门即任何已认证成员可读，`internal/handler/app_connector.go:60-66`；响应 `{ success, data: [{ id, kind, state, owner_id, auth_version }] }`，`internal/handler/app_connector_connection.go:55-92`；state 取值 `active/revoked/pending_reauthorization`，`internal/modules/appconnector/model.go:15-27`）。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-33.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（Implementation Decisions、Testing Decisions）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§6 Resource Shelf Module、§4 Mobile Runtime 所有权、§10 App Shell 禁止清单、§13 Interface 测试面）
- ADR：`docs/adr/0012-mobile-business-logic-lives-behind-deep-modules.md`（业务逻辑位于深 Module 后）、`docs/adr/0005-weknora-native-mobile-client.md`、`docs/adr/0006-mobile-transport-by-semantics.md`
- 领域术语：`CONTEXT.md`（「资源（Resource）」「连接（Connection）」「个人连接」「空间连接」「Available Agent」相关条目）
- Parent：Issue #30；Blocked by：#32（T02——本计划逐字消费其 Produces，见各任务 Consumes；本计划撰写时 worktree 尚未合并 #32 实现，行号锚点以 #32 合并后的代码为准并逐处说明）

## Global Constraints

以下为批准 Spec / ADR 的项目级约束，逐字引用，所有任务隐含遵守：

- 「Resource Shelf owns Available Agent, knowledge, connection and attachment selection. Marketplace governance remains on Web.」（mobile-ai-office-design.md · Implementation Decisions）
- 「Mobile core does not depend on React Native, DOM or concrete transport. Remote and native details are injected as Adapters.」（同上）
- 「Cache identity includes Deployment, user, Tenant, Task and Run where applicable. Scope changes invalidate subscriptions and reject late responses.」（同上）
- 「The mobile information architecture is Home, Tasks, New, Resources and Me.」（同上；本计划新增 Resources 一级入口，不新增其它入口）
- 「Interface 不暴露 token、query key、generation number 或 SecureStore key。Scope Lease 是不透明、可撤销的能力对象，子 Module 每次异步提交前检查其有效性。」（mobile-module-seams.md §4.2）
- 「Interface 返回领域状态，不返回原始 Agent/KB/Connection DTO。SelectionVerdict 明确 allowed、unavailable 或 forbidden 及原因；没有能力事实时不猜测。」（mobile-module-seams.md §6.2）
- 「Resource Shelf 统一呈现和选择当前 Tenant 的：Available Agent；可发现知识资源；成员可使用的 Connection 能力摘要；附件准备状态；Task 创建所需的资源兼容性和不可用原因。」（mobile-module-seams.md §6.1——附件准备状态与 Task 创建属 #36/T06，见「差异记录」）
- 「Screen 不直接导入 packages/contracts 或 packages/api-client」「禁止：把 Adapter DTO 直接作为长期 UI state」（mobile-module-seams.md §10）
- 「Tests target observable behavior at the highest stable Interface. Screen tests verify rendering and navigation; they do not duplicate Module internals.」（mobile-ai-office-design.md · Testing Decisions）
- 「Resource Shelf Interface tests cover Available Agent filtering, knowledge authorization, connection capability, attachment preparation, revoked resources and explicit unavailable/forbidden reasons.」（同上；attachment preparation 分属 #36，见「差异记录」）
- 「撤权（403）后敏感字段立即不可见——展示模型从服务端事实重建，不从缓存回填」（`packages/domain/src/mobile/resource-presentation.ts:4` 冻结规则）
- 工作流约束：严格 RED→GREEN→REFACTOR（每个任务先写失败测试、实跑确认失败、最小实现、通过、提交）；实现不与已批准 Spec 冲突，冲突时升级而非静默重设计。
- 安全约束（会话注入）：配置凭据只从环境变量或密钥服务读取；源码、示例和测试都不得写入可用的凭据字面量。本计划沿用 T01/T02 的 opt-in 环境变量模式，无任何回退凭据；真实请求仅允许 http/https 且发出前校验 host（#32 Task 6 的 `disallowedDeploymentHost` 防线已覆盖本计划复用的同一部署 URL 输入）。

**Issue #33 验收标准原文（docs/plans/issue30-sweep/issues/issue-33.md）：**

1. 「撤权或 capability 变化后资源投影及时失效。」
2. 「Screen 只消费 Resource Shelf Interface，不直接拼 wire DTO。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

验收标准 3 的本地可验证性说明（blocked-env 声明）：「最高稳定 Interface」的真实集成证据沿用 T01/T02 已合并的 opt-in 真实 HTTP 模式（`apps/mobile/src/runtime-integration-smoke.ts` + `packages/api-client/src/mobile/runtime.integration.test.ts`，`WEKNORA_MOBILE_TEST_*` 环境变量启用，无凭据回退）。本地无真实 Deployment 时该测试 `t.skip`（不得伪造通过）。**运行中真实撤权**（测试期间由管理员实时撤销成员权限）与真机渲染需外部环境，列为 blocked-env；本地替代证据为：(a) Resource Shelf Interface 级测试（真实 403 语义场景 Adapter + 真实 Runtime 编排 + 真 Scope Lease 撤销）；(b) api-client wire 契约测试（基于 handler 源码核实的服务器响应形状构造的真实序列化字节）；(c) 具备环境时自动产出的真实 HTTP browse 证据（Task 6）。

## Review Focus

Spec 隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **迟到的旧 scope browse 结果**：租户切换后旧 handle 的在途 browse 返回，不得污染新投影或静默成功。——Task 3 测试「a browse that resolves after close is discarded」；Task 5 控制器测试「a stale in-flight projection never overwrites a newer one」。
2. **401 刷新循环**：access token 过期时 browse 反复 401→refresh，必须恰好重试一次，第二次 401 判 unavailable（fail closed），绝不循环。——Task 3 测试「a 401 retries exactly once with a refreshed token and never loops」；Task 4 测试「browse retries once through the runtime refresh seam」。
3. **单一资源类失败拖垮整页**：knowledge 403/5xx 不得让 agents 消失（页面按类呈现 verdict）。——Task 3 测试「a 403 revocation invalidates the projection immediately…」与「a server failure is unavailable, not an authorization revocation」。
4. **敏感配置经 wire 进入移动投影**：Agent 的 `config.system_prompt` 等敏感 Prompt 配置不得到达 UI。——Task 2 断言适配后 JSON 不含 SECRET；Task 3 断言 `ResourcePage` 序列化后不含（双层防线）；Task 1 领域层断言 `toAgentOptions` 不透传。
5. **撤权后 Screen 仍显示旧缓存标题**（stale projection 回填）。——Task 5 控制器测试「the controller loads the page and reloads on authorization revocation」+ Screen 测试断言 forbidden 类不再渲染行、横幅显示原因。

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 1 | domain 连接资源三态投影 + 种子专属测试 | 纯策略补齐与回归防线 |
| 2 | api-client WeKnora Resource Remote 适配 | 三端点 wire→语义行 + 契约测试 |
| 3 | mobile-core Resource Shelf Module | browse/selection/subscribe + 失效 |
| 4 | Runtime 接线（启动/失效/token seam） | scope 驱动的 shelf 生命周期 |
| 5 | apps/mobile Resources 页与组装 | 用户可见资源页（AC2） |
| 6 | 真实 HTTP 集成证据（resourceShelf browse） | AC3 最高稳定 Interface 证据 |

前置条件：worktree 根执行过 `pnpm install`（本计划作者已实测 `.worktrees/issue30-sweep` 可运行：`npx tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts` 24/24 pass、`pnpm --filter @weknora/mobile test` 11/11 pass、`pnpm --filter @weknora/mobile typecheck` 0 错）。**#32 已按其计划实现并合并是硬性门控：Task 3-6 依赖 `scope-lease.ts`、`activateTenant`、`scopedVault` 接线与 #32 后的 `AuthorizedLandingScreen` 形态（本计划撰写时 worktree 均未合并，差异记录 10），必须在 #32 合并后执行，否则 Task 3/4/5/6 的锚点不成立。** 所有测试命令在 worktree 根执行。

---

### Task 1: domain 连接资源三态投影与种子专属测试

**Files:**
- Modify: `packages/domain/src/mobile/resource-presentation.ts:20-24`（`ConnectionResource` 重定义为携带生命周期三态的投影）、文件顶部（新增 type import）
- Test: `packages/domain/src/mobile/agent-options.test.ts`（新建，专属测试——现状无该文件）
- Test: `packages/domain/src/mobile/resource-presentation.test.ts`（新建，专属测试——现状无该文件）

**Interfaces:**
- Consumes: 既有 `toAgentOptions/codingAgentCapability/selectAgent/defaultAgent/filterAgents`（`packages/domain/src/mobile/agent-options.ts:30-82`）与 `toKnowledgeResource/revokedKnowledgeProjection/knowledgeRefForPrompt/scanStatusPresentation`（`packages/domain/src/mobile/resource-presentation.ts:33-91`）。
- Produces:
  - `type ConnectionLifecycleState = 'active' | 'revoked' | 'pending_reauthorization' | 'unknown'`（对齐后端 `internal/modules/appconnector/model.go:15-19`）
  - `interface ConnectionResource { id: string; kind: 'personal' | 'space'; state: ConnectionLifecycleState; connected: boolean; capability: { state: AgentCapabilityState; reason: string } }`（替换旧的 `{ id; name; connected }`——旧形态无任何消费者，`grep -rn ConnectionResource` 仅命中定义与 `index.ts` 的 re-export，已核实）
  - `connectionCapability(state: ConnectionLifecycleState): ConnectionResource['capability']`
  - `toConnectionResource(row: Record<string, unknown>): ConnectionResource`
  - Task 2/3/5 消费以上类型与 `toAgentOptions`/`toKnowledgeResource` 的输入行形状（`{ id, name, summary, kind, capability }` 与 `{ id, title, scan_status, document_count, updated_at }`）。

- [ ] **Step 1: 写失败测试**

新建 `packages/domain/src/mobile/agent-options.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  defaultAgent,
  filterAgents,
  selectAgent,
  toAgentOptions,
  type AgentOption,
} from './agent-options.ts';

test('toAgentOptions projects presentation-safe fields and defaults missing capability to unavailable', () => {
  const options = toAgentOptions([
    {
      id: 'builtin-quick-answer',
      name: 'Quick Answer',
      summary: 'Fast answers',
      kind: 'general',
      capability: { state: 'supported', reason: '' },
      config: { system_prompt: 'SECRET-PROMPT' },
    },
    { id: 'agent-2', name: 'No capability facts' },
    { name: 'missing id' },
  ]);

  assert.deepEqual(options, [
    { id: 'builtin-quick-answer', name: 'Quick Answer', summary: 'Fast answers', kind: 'general', capability: { state: 'supported', reason: '' } },
    { id: 'agent-2', name: 'No capability facts', summary: '', kind: 'general', capability: { state: 'unavailable', reason: 'capability_not_reported' } },
  ]);
  assert.equal(JSON.stringify(options).includes('SECRET-PROMPT'), false, 'raw config must not survive the projection');
});

test('selectAgent refuses revoked coding targets and never falls back silently', () => {
  const directory = {
    agents: toAgentOptions([
      { id: 'coder', name: 'Coder', kind: 'coding', authorizedTargetId: 'target-1', capability: { state: 'supported', reason: '' } },
      { id: 'helper', name: 'Helper', kind: 'general', capability: { state: 'supported', reason: '' } },
    ]),
  };

  assert.deepEqual(selectAgent(directory, 'coder', [{ id: 'target-1', revoked: true }]), { unavailableReason: 'driver_unavailable' });
  assert.equal(selectAgent(directory, 'coder', [{ id: 'target-1' }]).agent?.id, 'coder');
  assert.deepEqual(selectAgent(directory, 'missing'), { unavailableReason: 'agent_not_found' });
  assert.equal(defaultAgent(directory, [{ id: 'target-1', revoked: true }])?.id, 'helper');
});

test('filterAgents narrows display only and never changes capability verdicts', () => {
  const agents: AgentOption[] = [
    { id: 'a', name: 'Research', summary: 'deep research', kind: 'analysis', capability: { state: 'forbidden', reason: 'policy' } },
    { id: 'b', name: 'Writer', summary: '', kind: 'general', capability: { state: 'unavailable', reason: 'capability_not_reported' } },
  ];

  assert.deepEqual(filterAgents({ agents }, { kind: 'analysis' }).map((agent) => agent.id), ['a']);
  assert.deepEqual(filterAgents({ agents }, { keyword: 'wri' }).map((agent) => agent.id), ['b']);
  assert.deepEqual(filterAgents({ agents }, {}).map((agent) => agent.id), ['a', 'b']);
});
```

新建 `packages/domain/src/mobile/resource-presentation.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  connectionCapability,
  knowledgeRefForPrompt,
  revokedKnowledgeProjection,
  scanStatusPresentation,
  toConnectionResource,
  toKnowledgeResource,
} from './resource-presentation.ts';

test('toKnowledgeResource maps wire rows and reports unknown scan status as pending', () => {
  assert.deepEqual(
    toKnowledgeResource({ id: 'kb-1', title: 'Handbook', scan_status: 'indexed', document_count: 3, updated_at: '2026-09-01T00:00:00Z' }),
    { id: 'kb-1', title: 'Handbook', scanStatus: 'indexed', documentCount: 3, updatedAt: '2026-09-01T00:00:00Z' },
  );
  // 未知/缺失扫描状态按 pending 展示——不臆造 indexed（冻结规则）。
  assert.deepEqual(
    toKnowledgeResource({ id: 'kb-2', name: 'Shared', documentCount: 0 }),
    { id: 'kb-2', title: 'Shared', scanStatus: 'pending', documentCount: 0, updatedAt: '' },
  );
});

test('revocation hides sensitive knowledge fields and blocks ask-with-knowledge', () => {
  assert.deepEqual(revokedKnowledgeProjection(), { visibleSensitiveFields: [], canAskWithKnowledge: false });
  const resource = toKnowledgeResource({ id: 'kb-1', title: 'Handbook' });
  assert.deepEqual(knowledgeRefForPrompt(resource, { revoked: true }), null);
  assert.deepEqual(knowledgeRefForPrompt(resource, { revoked: false }), { kind: 'knowledge_ref', knowledgeId: 'kb-1' });
});

test('scan status presentation carries label and tone on separate channels', () => {
  assert.deepEqual(scanStatusPresentation('indexed'), { label: '已索引', tone: 'brand' });
  assert.deepEqual(scanStatusPresentation('scanning'), { label: '扫描中', tone: 'warning' });
  assert.deepEqual(scanStatusPresentation('failed'), { label: '扫描失败', tone: 'danger' });
  assert.deepEqual(scanStatusPresentation('pending'), { label: '待扫描', tone: 'neutral' });
});

test('connection capability explains supported, unavailable and never guesses unknown states', () => {
  assert.deepEqual(connectionCapability('active'), { state: 'supported', reason: '' });
  assert.deepEqual(connectionCapability('revoked'), { state: 'unavailable', reason: 'connection_revoked' });
  assert.deepEqual(connectionCapability('pending_reauthorization'), { state: 'unavailable', reason: 'reauthorization_required' });
  assert.deepEqual(connectionCapability('unknown'), { state: 'unavailable', reason: 'connection_state_not_reported' });
});

test('toConnectionResource projects wire rows without credentials or fabricated names', () => {
  assert.deepEqual(
    toConnectionResource({ id: 'conn-1', kind: 'personal', state: 'active', owner_id: 'member-1', auth_version: 4 }),
    { id: 'conn-1', kind: 'personal', state: 'active', connected: true, capability: { state: 'supported', reason: '' } },
  );
  assert.deepEqual(
    toConnectionResource({ id: 'conn-2', kind: 'space', state: 'revoked' }),
    { id: 'conn-2', kind: 'space', state: 'revoked', connected: false, capability: { state: 'unavailable', reason: 'connection_revoked' } },
  );
  assert.deepEqual(
    toConnectionResource({ id: 'conn-3', kind: 'personal' }),
    { id: 'conn-3', kind: 'personal', state: 'unknown', connected: false, capability: { state: 'unavailable', reason: 'connection_state_not_reported' } },
  );
  const projected = toConnectionResource({ id: 'conn-4', kind: 'space', state: 'active', access_token: 'SECRET', credential_ref: 'SECRET-REF' });
  assert.equal(JSON.stringify(projected).includes('SECRET'), false, 'credential-looking fields must not survive projection');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/domain/src/mobile/resource-presentation.test.ts packages/domain/src/mobile/agent-options.test.ts`
Expected: FAIL——RED 来自缺失导出的运行期行为：tsx 下缺失的具名导入解析为 `undefined`，`connectionCapability('active')` 等调用抛 `TypeError: connectionCapability is not a function`（连接相关两个测试体执行后失败；模块本身可加载）。agent-options 的三个测试为既有行为的回归防线（实现前即通过，其价值在钉住「撤权不可选/敏感字段不透传/筛选不改裁决」——与 plan-t32 Task 4 测试 4 同一处理方式）。

- [ ] **Step 3: 最小实现**

`packages/domain/src/mobile/resource-presentation.ts`：

1. 文件顶部注释块之后加：

```ts
import type { AgentCapabilityState } from './agent-options.ts';
```

2. 将现有 `ConnectionResource` 定义（20-24 行）替换为：

```ts
/** Connection 生命周期（internal/modules/appconnector/model.go:15-19：active/revoked/pending_reauthorization）。 */
export type ConnectionLifecycleState = 'active' | 'revoked' | 'pending_reauthorization' | 'unknown';

export interface ConnectionResource {
  id: string;
  kind: 'personal' | 'space';
  state: ConnectionLifecycleState;
  connected: boolean;
  capability: { state: AgentCapabilityState; reason: string };
}
```

3. `toKnowledgeResource`（33 行）之前插入两个纯函数：

```ts
/** 连接能力三态裁决：无状态记录=unavailable（没有能力事实不得放行，不猜测）。 */
export function connectionCapability(state: ConnectionLifecycleState): ConnectionResource['capability'] {
  if (state === 'active') return { state: 'supported', reason: '' };
  if (state === 'revoked') return { state: 'unavailable', reason: 'connection_revoked' };
  if (state === 'pending_reauthorization') return { state: 'unavailable', reason: 'reauthorization_required' };
  return { state: 'unavailable', reason: 'connection_state_not_reported' };
}

/** 服务端连接行 → 展示模型（只提升 id/kind/state 三个字段——wire 视图本无凭据字段，仍做结构性过滤双保险）。 */
export function toConnectionResource(row: Record<string, unknown>): ConnectionResource {
  const state: ConnectionLifecycleState =
    row.state === 'active' || row.state === 'revoked' || row.state === 'pending_reauthorization' ? row.state : 'unknown';
  return {
    id: String(row.id ?? ''),
    kind: row.kind === 'space' ? 'space' : 'personal',
    state,
    connected: state === 'active',
    capability: connectionCapability(state),
  };
}
```

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/domain/src/mobile/resource-presentation.test.ts packages/domain/src/mobile/agent-options.test.ts`
Expected: PASS（两文件共 8 个测试全绿）。

- [ ] **Step 5: Commit**

```bash
git add packages/domain/src/mobile/resource-presentation.ts packages/domain/src/mobile/resource-presentation.test.ts packages/domain/src/mobile/agent-options.test.ts
git commit -m "feat(domain): three-state connection projection plus dedicated agent-options policy tests"
```

---

### Task 2: api-client WeKnora Resource Remote 适配（三端点 wire→语义行）

**Files:**
- Create: `packages/api-client/src/mobile/resources.ts`
- Modify: `packages/api-client/package.json`（`exports` 追加 `"./mobile/resources": "./src/mobile/resources.ts"`——现状 exports 仅 `./mobile/runtime` 等五个条目，实测从 apps/mobile `import('@weknora/api-client/mobile/resources')` 得 `ERR_PACKAGE_PATH_NOT_EXPORTED`，不补此条目 Task 5/6 的导入无法解析）
- Test: `packages/api-client/src/mobile/resources.test.ts`（新建）

**Interfaces:**
- Consumes: `ClientRequest`（`packages/api-client/src/client.ts`——`{ method, path, headers, ... }`，非 2xx 时传输层抛 `ApiError`（`packages/api-client/src/errors.ts:24-45`，携带 `status`））；与 `createMobileRuntimeRemote` 相同的 `{ origin, request }` 构造约定（`packages/api-client/src/mobile/runtime.ts:7-15`）。
- Produces: `createMobileResourceRemote(options: { origin: string; request: (input: ClientRequest) => Promise<unknown> }): MobileResourceRemote`，其中

```ts
export interface MobileResourceRemote {
  availableAgents(accessToken: string): Promise<{ rows: ReadonlyArray<Record<string, unknown>>; disabledOwnAgentIds: ReadonlySet<string> }>;
  knowledgeBases(accessToken: string): Promise<ReadonlyArray<Record<string, unknown>>>;
  connections(accessToken: string): Promise<ReadonlyArray<Record<string, unknown>>>;
}
```

（Task 3 的 `ResourceRemote` 端口是它的结构子集，逐字一致可赋值；Task 4/5/6 以此为 WeKnora Adapter。）语义行形状：agents → `{ id, name, summary, kind: 'general' | 'custom', capability }`；knowledge → `{ id, title, scan_status, document_count, updated_at }`；connections → `{ id, kind, state }`。同时产出子路径导出 `@weknora/api-client/mobile/resources`（package.json exports 新条目）。

- [ ] **Step 1: 写失败测试**

新建 `packages/api-client/src/mobile/resources.test.ts`（复用 `runtime.test.ts` 的 recorder 模式；fixture 形状逐字段核实自 handler 源码，见 Tech Stack 引用）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createMobileResourceRemote } from './resources.ts';
import type { ClientRequest } from '../client.ts';

const ORIGIN = 'https://weknora.example.test';

interface Recorder {
  seen: ClientRequest[];
  request: (input: ClientRequest) => Promise<unknown>;
}

function recorder(responder: (input: ClientRequest) => unknown): Recorder {
  const seen: ClientRequest[] = [];
  return {
    seen,
    request: async (input: ClientRequest): Promise<unknown> => {
      seen.push(input);
      return responder(input);
    },
  };
}

/** 真实 GET /api/v1/agents 响应形状（internal/handler/custom_agent.go:269-273：
 * success/data/disabled_own_agent_ids；行字段见 internal/types/custom_agent.go:65-104）。 */
const AGENTS_WIRE = {
  success: true,
  data: [
    {
      id: 'builtin-quick-answer', name: '快速问答', description: '检索增强问答', avatar: '🤖',
      is_builtin: true, tenant_id: 7, created_by: '',
      config: { agent_mode: 'quick-answer', system_prompt: 'SECRET-SYSTEM-PROMPT' },
    },
    {
      id: '3f2b8c0e-1', name: '研究助手', description: '深度研究',
      is_builtin: false, created_by: 'member-1', creator_name: '张三',
      config: { agent_mode: 'smart-reasoning', agent_type: 'rag-qa', system_prompt: 'SECRET-2' },
    },
  ],
  disabled_own_agent_ids: ['3f2b8c0e-1'],
};

/** 真实 GET /api/v1/knowledge-bases 行形状（internal/types/knowledgebase.go:59-140 +
 * buildKBListResponse 附加 vector_store_status，取值 available/unavailable，internal/types/vectorstore.go:544-580）。 */
const KNOWLEDGE_WIRE = {
  success: true,
  data: [
    { id: 'kb-1', name: '员工手册', knowledge_count: 3, updated_at: '2026-09-01T00:00:00Z', is_processing: false, vector_store_status: 'available' },
    { id: 'kb-2', name: 'FAQ', knowledge_count: 0, updated_at: '2026-09-02T00:00:00Z', is_processing: true },
    { id: 'kb-3', name: '归档库', knowledge_count: 0, updated_at: '2026-09-03T00:00:00Z', is_processing: false, vector_store_status: 'unavailable' },
    { id: 'kb-4', name: '共享库', knowledge_count: 2, updated_at: '2026-09-04T00:00:00Z', is_processing: false },
  ],
};

/** 真实 GET /api/v1/apps/connections 响应形状（internal/handler/app_connector_connection.go:55-92）。 */
const CONNECTIONS_WIRE = {
  success: true,
  data: [
    { id: 'conn-1', kind: 'personal', state: 'active', owner_id: 'member-1', auth_version: 4 },
    { id: 'conn-2', kind: 'space', state: 'revoked', owner_id: null, auth_version: 1 },
  ],
};

test('availableAgents maps wire rows to presentation-safe options and reports disabled ids', async () => {
  const spy = recorder(() => AGENTS_WIRE);
  const remote = createMobileResourceRemote({ origin: ORIGIN, request: spy.request });

  const result = await remote.availableAgents('resource-token');

  assert.deepEqual(result.rows, [
    { id: 'builtin-quick-answer', name: '快速问答', summary: '检索增强问答', kind: 'general', capability: { state: 'supported', reason: '' } },
    { id: '3f2b8c0e-1', name: '研究助手', summary: '深度研究', kind: 'custom', capability: { state: 'supported', reason: '' } },
  ]);
  assert.deepEqual([...result.disabledOwnAgentIds], ['3f2b8c0e-1']);
  assert.equal(JSON.stringify(result).includes('SECRET'), false, 'agent config must never reach the semantic rows');
  assert.equal(spy.seen.length, 1);
  assert.equal(spy.seen[0]!.method, 'GET');
  assert.equal(spy.seen[0]!.path, '/api/v1/agents');
  assert.equal((spy.seen[0]!.headers as Record<string, string> | undefined)?.authorization, 'Bearer resource-token');
});

test('knowledgeBases derives scan status from processing and store status without inventing indexed', async () => {
  const spy = recorder(() => KNOWLEDGE_WIRE);
  const remote = createMobileResourceRemote({ origin: ORIGIN, request: spy.request });

  const rows = await remote.knowledgeBases('resource-token');

  assert.deepEqual(rows, [
    { id: 'kb-1', title: '员工手册', scan_status: 'indexed', document_count: 3, updated_at: '2026-09-01T00:00:00Z' },
    { id: 'kb-2', title: 'FAQ', scan_status: 'scanning', document_count: 0, updated_at: '2026-09-02T00:00:00Z' },
    { id: 'kb-3', title: '归档库', scan_status: 'failed', document_count: 0, updated_at: '2026-09-03T00:00:00Z' },
    { id: 'kb-4', title: '共享库', scan_status: 'pending', document_count: 2, updated_at: '2026-09-04T00:00:00Z' },
  ]);
  assert.equal(spy.seen[0]!.path, '/api/v1/knowledge-bases');
});

test('connections passes through lifecycle fields and nothing else', async () => {
  const spy = recorder(() => CONNECTIONS_WIRE);
  const remote = createMobileResourceRemote({ origin: ORIGIN, request: spy.request });

  const rows = await remote.connections('resource-token');

  assert.deepEqual(rows, [
    { id: 'conn-1', kind: 'personal', state: 'active' },
    { id: 'conn-2', kind: 'space', state: 'revoked' },
  ]);
  assert.equal(JSON.stringify(rows).includes('owner_id'), false);
  assert.equal(JSON.stringify(rows).includes('auth_version'), false);
  assert.equal(spy.seen[0]!.path, '/api/v1/apps/connections');
});

test('failed envelopes and malformed payloads fail closed', async () => {
  const failing = recorder(() => ({ success: false, error: { code: 'FORBIDDEN', message: 'no' } }));
  const remote = createMobileResourceRemote({ origin: ORIGIN, request: failing.request });
  await assert.rejects(remote.availableAgents('resource-token'), /success must be true/);

  const malformed = recorder(() => ({ success: true }));
  const badRemote = createMobileResourceRemote({ origin: ORIGIN, request: malformed.request });
  await assert.rejects(badRemote.knowledgeBases('resource-token'), /data is required/);

  const notArray = recorder(() => ({ success: true, data: { nope: true } }));
  const weirdRemote = createMobileResourceRemote({ origin: ORIGIN, request: notArray.request });
  await assert.rejects(weirdRemote.connections('resource-token'), /must be an array/);
});

test('origin and access token are validated before any request leaves', async () => {
  const spy = recorder(() => AGENTS_WIRE);
  assert.throws(() => createMobileResourceRemote({ origin: 'http://weknora.example.test', request: spy.request }), /HTTPS/);
  const remote = createMobileResourceRemote({ origin: ORIGIN, request: spy.request });
  await assert.rejects(remote.availableAgents('  '), /access token is required/);
  assert.equal(spy.seen.length, 0, 'invalid input must not reach the wire');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/api-client/src/mobile/resources.test.ts`
Expected: FAIL——`Cannot find module './resources.ts'`（文件不存在，模块加载失败即 RED 证据）。

- [ ] **Step 3: 最小实现**

(a) 新建 `packages/api-client/src/mobile/resources.ts`：

```ts
import type { ClientRequest } from '../client.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface MobileResourceRemoteOptions {
  /**
   * 部署 Origin。构造即强校验（与 createMobileRuntimeRemote 相同规则）：绝对 HTTPS URL、
   * 无内嵌 user-info、无 path/query/fragment——非法 Origin 在任何请求发出前同步抛错。
   */
  origin: string;
  /** 复用既有 ClientRequest 通道（createWeKnoraClient().request），本适配器不新建传输。 */
  request: Request;
}

/**
 * WeKnora 资源读 seam 的具体适配（spec §6.3 Resource Backend Port）。组合三个 Viewer+
 * 读端点，只做信封解包与 wire→语义行投影；能力三态裁决与失效归 mobile-core Resource Shelf。
 */
export interface MobileResourceRemote {
  availableAgents(accessToken: string): Promise<{ rows: ReadonlyArray<Record<string, unknown>>; disabledOwnAgentIds: ReadonlySet<string> }>;
  knowledgeBases(accessToken: string): Promise<ReadonlyArray<Record<string, unknown>>>;
  connections(accessToken: string): Promise<ReadonlyArray<Record<string, unknown>>>;
}

function requireDeploymentOrigin(origin: string): void {
  let parsed: URL;
  if (typeof origin !== 'string' || origin.trim() === '') throw new Error('deployment origin is required');
  try {
    parsed = new URL(origin);
  } catch {
    throw new Error(`deployment origin must be an absolute URL: ${origin}`);
  }
  if (parsed.protocol !== 'https:') throw new Error('deployment origin must use HTTPS');
  if (parsed.username !== '' || parsed.password !== '') throw new Error('deployment origin must not embed user info');
  if (parsed.hostname === '') throw new Error('deployment origin must include a host');
  if (parsed.pathname !== '/') throw new Error('deployment origin must not include a path');
  if (parsed.search !== '' || parsed.hash !== '') throw new Error('deployment origin must not include a query or fragment');
}

function requireAccessToken(accessToken: string): string {
  if (typeof accessToken !== 'string' || accessToken.trim() === '') throw new Error('access token is required');
  return accessToken;
}

function bearerRequest(request: Request, accessToken: string): Request {
  return (input: ClientRequest): Promise<unknown> =>
    request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
}

function envelope(value: unknown, path: string): { data: unknown; root: Record<string, unknown> } {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error(`${path} response must be an object`);
  const root = value as Record<string, unknown>;
  if (root.success !== true) throw new Error(`${path} response.success must be true`);
  if (!Object.prototype.hasOwnProperty.call(root, 'data')) throw new Error(`${path} response.data is required`);
  return { data: root.data, root };
}

function rowsOf(data: unknown, path: string): ReadonlyArray<Record<string, unknown>> {
  if (!Array.isArray(data)) throw new Error(`${path} response.data must be an array`);
  return data.filter((row): row is Record<string, unknown> => typeof row === 'object' && row !== null && !Array.isArray(row));
}

export function createMobileResourceRemote(options: MobileResourceRemoteOptions): MobileResourceRemote {
  requireDeploymentOrigin(options.origin);
  const request = options.request;
  const get = (accessToken: string, path: string): Promise<unknown> =>
    bearerRequest(request, requireAccessToken(accessToken))({ method: 'GET', path });
  return {
    async availableAgents(accessToken) {
      const { data, root } = envelope(await get(accessToken, '/api/v1/agents'), '/api/v1/agents');
      const rows = rowsOf(data, '/api/v1/agents').map((row) => ({
        id: row.id,
        name: row.name,
        // 展示摘要来自 wire description；config（含 system_prompt 等敏感 Prompt 配置）结构性不提升。
        summary: row.description,
        // 能力事实来源：Viewer+ 的租户范围 /agents 列表把该行返回给本成员（与 Web 会话下拉同源事实）。
        kind: row.is_builtin === true ? 'general' : 'custom',
        capability: { state: 'supported', reason: '' },
      }));
      const disabled = Array.isArray(root.disabled_own_agent_ids) ? root.disabled_own_agent_ids : [];
      return { rows, disabledOwnAgentIds: new Set(disabled.filter((id): id is string => typeof id === 'string')) };
    },
    async knowledgeBases(accessToken) {
      const { data } = envelope(await get(accessToken, '/api/v1/knowledge-bases'), '/api/v1/knowledge-bases');
      return rowsOf(data, '/api/v1/knowledge-bases').map((row) => ({
        id: row.id,
        title: row.name,
        // 扫描状态推导：is_processing 优先；store 状态 available→indexed / unavailable→failed；缺失→pending（不臆造 indexed）。
        scan_status:
          row.is_processing === true ? 'scanning'
          : row.vector_store_status === 'unavailable' ? 'failed'
          : row.vector_store_status === 'available' ? 'indexed'
          : 'pending',
        document_count: row.knowledge_count,
        updated_at: row.updated_at,
      }));
    },
    async connections(accessToken) {
      const { data } = envelope(await get(accessToken, '/api/v1/apps/connections'), '/api/v1/apps/connections');
      return rowsOf(data, '/api/v1/apps/connections').map((row) => ({ id: row.id, kind: row.kind, state: row.state }));
    },
  };
}
```

（`requireDeploymentOrigin`/`requireAccessToken`/`bearerRequest` 与 `runtime.ts` 的模块内私有助手重复三份小函数——刻意不导出复用，避免本计划改写 #32 触碰的 `runtime.ts`；重复面仅构造期校验，属记录在案的取舍。）

(b) `packages/api-client/package.json` 的 `exports` 中 `"./mobile/runtime": "./src/mobile/runtime.ts",` 之后加一行：

```json
    "./mobile/resources": "./src/mobile/resources.ts",
```

（Task 5 的 `composition.ts` 与 Task 6 的 `runtime-integration-smoke.ts` 都以 `@weknora/api-client/mobile/resources` 导入；与既有 `./mobile/runtime` seam 同构，对齐 spec §16「当前 api-client mobile exports 未在 package exports 中形成独立 seam」的既有做法——本计划补齐 resources 这一条，不重排其它条目。）

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/api-client/src/mobile/resources.test.ts`
Expected: PASS（5 个测试全绿）。随后从 apps/mobile 实测子路径导出可解析（Task 5/6 的导入前提）：

```bash
cd apps/mobile && node --input-type=module -e "import('@weknora/api-client/mobile/resources').then(() => console.log('resolved')).catch((e) => { console.error(e.code); process.exit(1); })"
```

Expected: 输出 `resolved`、退出码 0（修改前同一命令实测为 `ERR_PACKAGE_PATH_NOT_EXPORTED`）。

- [ ] **Step 5: Commit**

```bash
git add packages/api-client/src/mobile/resources.ts packages/api-client/src/mobile/resources.test.ts packages/api-client/package.json
git commit -m "feat(api-client): weknora resource remote adapter for agents, knowledge bases and connections"
```

---

### Task 3: mobile-core Resource Shelf Module（browse/selection/subscribe + 失效）

**Files:**
- Create: `packages/mobile-core/src/shelf/ports.ts`
- Create: `packages/mobile-core/src/shelf/types.ts`
- Create: `packages/mobile-core/src/shelf/resource-shelf.ts`
- Create: `packages/mobile-core/src/shelf/in-memory-resource-remote.ts`
- Test: `packages/mobile-core/src/shelf/resource-shelf.test.ts`（新建）

**Interfaces:**
- Consumes: Task 1 的 `toAgentOptions/filterAgents/toKnowledgeResource/toConnectionResource/knowledgeRefForPrompt`（`@weknora/domain/mobile`）；#32 的 `RuntimeScopeLease`/`leaseScopeOf`/`leaseActive`（`packages/mobile-core/src/runtime/scope-lease.ts`，包内可 import，不经 index.ts）。
- Produces（Task 4/5/6 消费；Task 4 将其加入 index.ts 公共导出）:
  - `createResourceShelf(ports: ResourceShelfPorts): ResourceShelf`，`ResourceShelfPorts = { remote: ResourceRemote; accessTokenFor(origin: string, options?: { refresh?: boolean }): Promise<string> }`
  - `ResourceShelf.open(scope: { lease: ScopeLease }): ResourceShelfHandle`（lease 无效时抛 `SHELF_LEASE`）
  - `ResourceShelfHandle.browse(query?: ResourceQuery): Promise<ResourcePage>`、`.selection(input: { agentId?: string; knowledgeId?: string }): SelectionVerdict`、`.subscribe(listener: (event: ShelfInvalidationEvent) => void): () => void`、`.close(reason: ShelfCloseReason): void`
  - 类型：`ResourceClass = 'agent' | 'knowledge' | 'connection'`；`ShelfCloseReason = 'deployment-change' | 'tenant-switch' | 'sign-out' | 'dispose'`（与 #32 `VaultRevokeReason` 值集一致）；`ResourceClassVerdict = { state: 'supported' | 'unavailable' | 'forbidden'; reason: string }`；`ResourcePage = { tenantId: string; agents: readonly AgentOption[]; knowledge: readonly KnowledgeResource[]; connections: readonly ConnectionResource[]; classVerdicts: Readonly<Record<ResourceClass, ResourceClassVerdict>> }`；`ResourceQuery = { kind?: AgentOption['kind']; keyword?: string }`；`SelectionVerdict = { allowed: true; selection: { kind: 'agent'; agentId: string } | { kind: 'knowledge_ref'; knowledgeId: string } } | { allowed: false; state: 'unavailable' | 'forbidden'; reason: string }`；`ShelfInvalidationEvent = { type: 'scope-closed'; reason: ShelfCloseReason } | { type: 'authorization-revoked'; resourceClass: ResourceClass }`
  - `createInMemoryResourceRemote(script: ResourceRemoteScript): ScriptedResourceRemote`（in-memory 场景 Adapter，spec §12「remote but owned」测试方式）
  - 端口 `ResourceRemote`（`shelf/ports.ts`，与 Task 2 `MobileResourceRemote` 方法签名逐字一致；HTTP 错误契约：抛出的 Error 携带可选 `status: number`——api-client `ApiError` 结构性满足）

- [ ] **Step 1: 写失败测试**

新建 `packages/mobile-core/src/shelf/resource-shelf.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import { createInMemoryResourceRemote, type ResourceRemoteScript } from './in-memory-resource-remote.ts';
import { createResourceShelf } from './resource-shelf.ts';
import type { ResourceRemote } from './ports.ts';

const SCOPE = { deploymentOrigin: 'https://weknora.example.test', userId: 'member-1', tenantId: 'tenant-1' };

function openShelf(script: ResourceRemoteScript) {
  const remote = createInMemoryResourceRemote(script);
  const refreshes: number[] = [];
  const shelf = createResourceShelf({
    remote,
    accessTokenFor: async (_origin: string, options?: { refresh?: boolean }) => {
      if (options?.refresh) {
        refreshes.push(1);
        return 'token-2';
      }
      return 'token-1';
    },
  });
  const lease = new RuntimeScopeLease(SCOPE);
  return { shelf, handle: shelf.open({ lease: lease.asScopeLease() }), remote, refreshes, lease };
}

test('browse projects the three resource classes and never surfaces raw agent config', async () => {
  const { handle } = openShelf({
    agents: [
      { id: 'builtin-quick-answer', name: 'Quick Answer', summary: 'Fast answers', kind: 'general', capability: { state: 'supported', reason: '' }, config: { system_prompt: 'SECRET-PROMPT' } },
      { id: 'agent-2', name: 'Research', summary: 'Deep research', kind: 'custom', capability: { state: 'supported', reason: '' } },
    ],
    disabledOwnAgentIds: ['agent-2'],
    knowledgeBases: [{ id: 'kb-1', title: 'Handbook', scan_status: 'indexed', document_count: 3, updated_at: '2026-09-01T00:00:00Z' }],
    connections: [{ id: 'conn-1', kind: 'personal', state: 'active' }],
  });

  const page = await handle.browse();

  assert.equal(page.tenantId, 'tenant-1');
  assert.deepEqual(page.agents.map((agent) => agent.id), ['builtin-quick-answer'], 'disabled own agents must not appear as available');
  assert.deepEqual(page.knowledge.map((resource) => resource.title), ['Handbook']);
  assert.deepEqual(page.connections, [{ id: 'conn-1', kind: 'personal', state: 'active', connected: true, capability: { state: 'supported', reason: '' } }]);
  assert.deepEqual(page.classVerdicts, {
    agent: { state: 'supported', reason: '' },
    knowledge: { state: 'supported', reason: '' },
    connection: { state: 'supported', reason: '' },
  });
  assert.equal(JSON.stringify(page).includes('SECRET-PROMPT'), false, 'raw agent config must not survive the projection');
});

test('a 403 revocation invalidates the projection immediately and notifies subscribers', async () => {
  const script: ResourceRemoteScript = {
    agents: [{ id: 'builtin-quick-answer', name: 'Quick Answer', summary: '', kind: 'general', capability: { state: 'supported', reason: '' } }],
    knowledgeBases: [{ id: 'kb-1', title: 'Handbook', scan_status: 'indexed', document_count: 1, updated_at: '2026-09-01T00:00:00Z' }],
    connections: [],
  };
  const { handle } = openShelf(script);
  const events: unknown[] = [];
  handle.subscribe((event) => events.push(event));
  const first = await handle.browse();
  assert.equal(first.knowledge.length, 1);

  script.status = { knowledgeBases: 403 }; // 撤权：成员对该类资源失去授权
  const second = await handle.browse();

  assert.deepEqual(second.classVerdicts.knowledge, { state: 'forbidden', reason: 'http_403' });
  assert.deepEqual(second.knowledge, [], 'sensitive fields must not be refilled from the stale projection');
  assert.deepEqual(second.classVerdicts.agent, { state: 'supported', reason: '' }, 'one revoked class must not take down the others');
  assert.deepEqual(handle.selection({ knowledgeId: 'kb-1' }), { allowed: false, state: 'forbidden', reason: 'http_403' });
  assert.deepEqual(events, [{ type: 'authorization-revoked', resourceClass: 'knowledge' }]);
});

test('a server failure is unavailable, not an authorization revocation', async () => {
  const { handle } = openShelf({ connections: [{ id: 'conn-1', kind: 'space', state: 'active' }], status: { connections: 503 } });
  const events: unknown[] = [];
  handle.subscribe((event) => events.push(event));

  const page = await handle.browse();

  assert.deepEqual(page.classVerdicts.connection, { state: 'unavailable', reason: 'http_503' });
  assert.deepEqual(page.connections, []);
  assert.deepEqual(events, [], '5xx must not be reported as an authorization revocation');
});

test('a 401 retries exactly once with a refreshed token and never loops', async () => {
  const script: ResourceRemoteScript = {
    agents: [{ id: 'builtin-quick-answer', name: 'Quick Answer', summary: '', kind: 'general', capability: { state: 'supported', reason: '' } }],
  };
  script.status = { agents: (token: string) => (token === 'token-1' ? 401 : undefined) };
  const { handle, remote, refreshes } = openShelf(script);

  const page = await handle.browse();
  assert.deepEqual(page.agents.map((agent) => agent.id), ['builtin-quick-answer']);
  assert.deepEqual(remote.calls.filter((call) => call.kind === 'agents').map((call) => call.token), ['token-1', 'token-2']);
  assert.equal(refreshes.length, 1);

  script.status = { agents: () => 401 };
  const looping = await handle.browse();
  assert.deepEqual(looping.classVerdicts.agent, { state: 'unavailable', reason: 'http_401' }, 'a second 401 must fail closed instead of looping');
});

test('a revoked lease fails every operation closed even without an explicit close', async () => {
  const { handle, lease } = openShelf({ agents: [] });
  lease.revoke();

  await assert.rejects(handle.browse(), /SHELF_SCOPE_CLOSED/);
  assert.throws(() => handle.selection({ agentId: 'builtin-quick-answer' }), /SHELF_SCOPE_CLOSED/);
});

test('a browse that resolves after close is discarded', async () => {
  let resolveAgents!: (value: { rows: Array<Record<string, unknown>>; disabledOwnAgentIds: Set<string> }) => void;
  const remote: ResourceRemote = {
    availableAgents: () => new Promise((resolve) => { resolveAgents = resolve; }),
    knowledgeBases: async () => [],
    connections: async () => [],
  };
  const shelf = createResourceShelf({ remote, accessTokenFor: async () => 'token-1' });
  const lease = new RuntimeScopeLease(SCOPE);
  const handle = shelf.open({ lease: lease.asScopeLease() });

  const browsing = handle.browse();
  handle.close('tenant-switch');
  resolveAgents({ rows: [{ id: 'late-agent', name: 'Late', summary: '', kind: 'general', capability: { state: 'supported', reason: '' } }], disabledOwnAgentIds: new Set() });

  await assert.rejects(browsing, /SHELF_SCOPE_CLOSED/);
});

test('close notifies subscribers with the scope reason exactly once', () => {
  const { handle } = openShelf({});
  const events: unknown[] = [];
  handle.subscribe((event) => events.push(event));

  handle.close('sign-out');
  handle.close('sign-out');

  assert.deepEqual(events, [{ type: 'scope-closed', reason: 'sign-out' }]);
});

test('selection explains allowed, unavailable and forbidden without guessing', async () => {
  const { handle } = openShelf({
    agents: [
      { id: 'agent-1', name: 'Research', summary: '', kind: 'custom', capability: { state: 'supported', reason: '' } },
      { id: 'agent-2', name: 'Blocked', summary: '', kind: 'custom', capability: { state: 'forbidden', reason: 'policy' } },
    ],
    knowledgeBases: [{ id: 'kb-1', title: 'Handbook', scan_status: 'indexed', document_count: 0, updated_at: '' }],
  });
  await handle.browse();

  assert.deepEqual(handle.selection({ agentId: 'agent-1' }), { allowed: true, selection: { kind: 'agent', agentId: 'agent-1' } });
  assert.deepEqual(handle.selection({ agentId: 'agent-2' }), { allowed: false, state: 'forbidden', reason: 'policy' });
  assert.deepEqual(handle.selection({ agentId: 'missing' }), { allowed: false, state: 'unavailable', reason: 'agent_not_found' });
  assert.deepEqual(handle.selection({ knowledgeId: 'kb-1' }), { allowed: true, selection: { kind: 'knowledge_ref', knowledgeId: 'kb-1' } });
  assert.deepEqual(handle.selection({ knowledgeId: 'missing' }), { allowed: false, state: 'unavailable', reason: 'knowledge_not_found' });
});

test('browse queries narrow display without changing verdicts', async () => {
  const { handle } = openShelf({
    agents: [
      { id: 'agent-1', name: 'Research Bot', summary: 'deep research', kind: 'custom', capability: { state: 'supported', reason: '' } },
      { id: 'agent-2', name: 'Writer', summary: '', kind: 'general', capability: { state: 'supported', reason: '' } },
    ],
    knowledgeBases: [
      { id: 'kb-1', title: 'Handbook', scan_status: 'indexed', document_count: 0, updated_at: '' },
      { id: 'kb-2', title: 'Policies', scan_status: 'pending', document_count: 0, updated_at: '' },
    ],
  });

  const kindOnly = await handle.browse({ kind: 'custom' });
  assert.deepEqual(kindOnly.agents.map((agent) => agent.id), ['agent-1'], 'kind narrows agent display only');
  assert.deepEqual(kindOnly.knowledge.map((resource) => resource.id), ['kb-1', 'kb-2'], 'an agent-only filter leaves knowledge untouched');
  assert.deepEqual(kindOnly.classVerdicts.agent, { state: 'supported', reason: '' }, 'filtering never changes verdicts');

  const keyword = await handle.browse({ keyword: 'hand' });
  assert.deepEqual(keyword.agents.map((agent) => agent.id), [], 'filterAgents matches name+summary only — no agent contains "hand"');
  assert.deepEqual(keyword.knowledge.map((resource) => resource.id), ['kb-1'], 'keyword narrows knowledge by title');
  assert.deepEqual(keyword.connections, []);
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/mobile-core/src/shelf/resource-shelf.test.ts`
Expected: FAIL——`Cannot find module './in-memory-resource-remote.ts'` 或 `'./resource-shelf.ts'`（`shelf/` 目录尚不存在；`../runtime/scope-lease.ts` 由 #32 提供已存在）。

- [ ] **Step 3: 最小实现**

(a) 新建 `packages/mobile-core/src/shelf/ports.ts`：

```ts
/** Resource Backend Port（spec §6.3）：WeKnora Adapter 与 in-memory 场景 Adapter 的结构端口。 */
export interface ResourceRemote {
  /**
   * GET /api/v1/agents 语义行投影（见 api-client createMobileResourceRemote）。
   * HTTP 错误契约：以携带可选 status: number 的 Error 抛出（ApiError 结构性满足）。
   */
  availableAgents(accessToken: string): Promise<{ rows: ReadonlyArray<Record<string, unknown>>; disabledOwnAgentIds: ReadonlySet<string> }>;
  knowledgeBases(accessToken: string): Promise<ReadonlyArray<Record<string, unknown>>>;
  connections(accessToken: string): Promise<ReadonlyArray<Record<string, unknown>>>;
}

export interface ResourceShelfPorts {
  remote: ResourceRemote;
  /**
   * 包内 seam：由 createMobileRuntime 铸造（持有当前 scope 凭据与 refresh 单飞），
   * token 不出 mobile-core；browse 对 401 以 { refresh: true } 恰好重取一次。
   */
  accessTokenFor(origin: string, options?: { refresh?: boolean }): Promise<string>;
}
```

(b) 新建 `packages/mobile-core/src/shelf/types.ts`：

```ts
import type { AgentOption, ConnectionResource, KnowledgeResource } from '@weknora/domain/mobile';
import type { ScopeLease } from '../runtime/types.ts';

export type ResourceClass = 'agent' | 'knowledge' | 'connection';

/** 与 #32 VaultRevokeReason 值集一致（deployment-change/tenant-switch/sign-out/dispose）。 */
export type ShelfCloseReason = 'deployment-change' | 'tenant-switch' | 'sign-out' | 'dispose';

export interface ResourceClassVerdict {
  state: 'supported' | 'unavailable' | 'forbidden';
  reason: string;
}

/** 领域状态投影（spec §6.2：不返回原始 Agent/KB/Connection DTO）。 */
export interface ResourcePage {
  tenantId: string;
  agents: readonly AgentOption[];
  knowledge: readonly KnowledgeResource[];
  connections: readonly ConnectionResource[];
  classVerdicts: Readonly<Record<ResourceClass, ResourceClassVerdict>>;
}

export interface ResourceQuery {
  kind?: AgentOption['kind'];
  keyword?: string;
}

/** spec §6.2：SelectionVerdict 明确 allowed、unavailable 或 forbidden 及原因。 */
export type SelectionVerdict =
  | { allowed: true; selection: { kind: 'agent'; agentId: string } | { kind: 'knowledge_ref'; knowledgeId: string } }
  | { allowed: false; state: 'unavailable' | 'forbidden'; reason: string };

export type ShelfInvalidationEvent =
  | { type: 'scope-closed'; reason: ShelfCloseReason }
  | { type: 'authorization-revoked'; resourceClass: ResourceClass };

export interface ResourceShelfHandle {
  browse(query?: ResourceQuery): Promise<ResourcePage>;
  selection(input: { agentId?: string; knowledgeId?: string }): SelectionVerdict;
  subscribe(listener: (event: ShelfInvalidationEvent) => void): () => void;
  close(reason: ShelfCloseReason): void;
}

export interface ResourceShelf {
  /** 以有效 Scope Lease 开一个 shelf；lease 无效立即抛 SHELF_LEASE（fail closed）。 */
  open(scope: { lease: ScopeLease }): ResourceShelfHandle;
}
```

(c) 新建 `packages/mobile-core/src/shelf/resource-shelf.ts`：

```ts
import { filterAgents, knowledgeRefForPrompt, toAgentOptions, toConnectionResource, toKnowledgeResource } from '@weknora/domain/mobile';
import { leaseActive, leaseScopeOf } from '../runtime/scope-lease.ts';
import type { ResourceRemote, ResourceShelfPorts } from './ports.ts';
import type {
  ResourceClass,
  ResourceClassVerdict,
  ResourcePage,
  ResourceQuery,
  ResourceShelf,
  ResourceShelfHandle,
  SelectionVerdict,
  ShelfCloseReason,
  ShelfInvalidationEvent,
} from './types.ts';

const SUPPORTED: ResourceClassVerdict = { state: 'supported', reason: '' };
const RESOURCE_CLASSES: readonly ResourceClass[] = ['agent', 'knowledge', 'connection'];

function httpStatus(error: unknown): number | undefined {
  if (typeof error !== 'object' || error === null) return undefined;
  const status = (error as { status?: unknown }).status;
  return typeof status === 'number' ? status : undefined;
}

export function createResourceShelf(ports: ResourceShelfPorts): ResourceShelf {
  return {
    open({ lease }) {
      const scope = leaseScopeOf(lease);
      if (!scope || !leaseActive(lease)) throw new Error('SHELF_LEASE');
      const listeners = new Set<(event: ShelfInvalidationEvent) => void>();
      let closed = false;
      let agents = toAgentOptions([]);
      let knowledge: Array<ReturnType<typeof toKnowledgeResource>> = [];
      let connections: Array<ReturnType<typeof toConnectionResource>> = [];
      let verdicts: Record<ResourceClass, ResourceClassVerdict> = { agent: SUPPORTED, knowledge: SUPPORTED, connection: SUPPORTED };

      const notify = (event: ShelfInvalidationEvent): void => {
        for (const listener of [...listeners]) listener(event);
      };
      const guard = (): void => {
        if (closed || !leaseActive(lease)) throw new Error('SHELF_SCOPE_CLOSED');
      };
      const loadClass = async <T>(
        load: (token: string) => Promise<T>,
      ): Promise<{ ok: true; value: T } | { ok: false; verdict: ResourceClassVerdict }> => {
        const attempt = async (refresh: boolean): Promise<{ ok: true; value: T } | { ok: false; verdict: ResourceClassVerdict }> => {
          try {
            const token = await ports.accessTokenFor(scope.deploymentOrigin, refresh ? { refresh: true } : undefined);
            return { ok: true, value: await load(token) };
          } catch (error) {
            const status = httpStatus(error);
            if (status === 403) return { ok: false, verdict: { state: 'forbidden', reason: 'http_403' } };
            return { ok: false, verdict: { state: 'unavailable', reason: status === undefined ? 'fetch_failed' : `http_${status}` } };
          }
        };
        const first = await attempt(false);
        if (first.ok) return first;
        if (first.verdict.reason === 'http_401') return attempt(true); // 恰好一次刷新重试，禁止循环
        return first;
      };

      const handle: ResourceShelfHandle = {
        async browse(query: ResourceQuery = {}) {
          guard();
          const [agentResult, knowledgeResult, connectionResult] = await Promise.all([
            loadClass((token) => ports.remote.availableAgents(token)),
            loadClass((token) => ports.remote.knowledgeBases(token)),
            loadClass((token) => ports.remote.connections(token)),
          ]);
          guard(); // 在途期间 scope 已关闭/撤销 → 丢弃迟到结果（spec §5.3 不变量同源）
          const previous = verdicts;
          // 失败类必须清空旧投影（resource-presentation.ts:4 冻结规则：撤权后敏感字段立即
          // 不可见，展示模型从服务端事实重建，不从缓存回填）——forbidden/unavailable 均为空列表。
          agents = agentResult.ok
            ? toAgentOptions([...agentResult.value.rows]).filter((option) => !agentResult.value.disabledOwnAgentIds.has(option.id))
            : [];
          knowledge = knowledgeResult.ok ? knowledgeResult.value.map((row) => toKnowledgeResource(row)) : [];
          connections = connectionResult.ok ? connectionResult.value.map((row) => toConnectionResource(row)) : [];
          verdicts = {
            agent: agentResult.ok ? SUPPORTED : agentResult.verdict,
            knowledge: knowledgeResult.ok ? SUPPORTED : knowledgeResult.verdict,
            connection: connectionResult.ok ? SUPPORTED : connectionResult.verdict,
          };
          for (const resourceClass of RESOURCE_CLASSES) {
            if (verdicts[resourceClass].state === 'forbidden' && previous[resourceClass].state !== 'forbidden') {
              notify({ type: 'authorization-revoked', resourceClass });
            }
          }
          const keyword = query.keyword?.trim().toLowerCase() ?? '';
          const page: ResourcePage = {
            tenantId: scope.tenantId,
            agents: filterAgents({ agents }, { kind: query.kind, keyword: query.keyword }),
            knowledge: keyword === '' ? knowledge : knowledge.filter((resource) => resource.title.toLowerCase().includes(keyword)),
            connections,
            classVerdicts: verdicts,
          };
          return page;
        },
        selection(input): SelectionVerdict {
          guard();
          if (typeof input.agentId === 'string') {
            if (verdicts.agent.state === 'forbidden') return { allowed: false, state: 'forbidden', reason: verdicts.agent.reason };
            const base = agents.find((agent) => agent.id === input.agentId);
            if (!base) return { allowed: false, state: 'unavailable', reason: 'agent_not_found' };
            if (base.capability.state !== 'supported') {
              return { allowed: false, state: base.capability.state === 'forbidden' ? 'forbidden' : 'unavailable', reason: base.capability.reason || base.capability.state };
            }
            return { allowed: true, selection: { kind: 'agent', agentId: base.id } };
          }
          if (typeof input.knowledgeId === 'string') {
            if (verdicts.knowledge.state === 'forbidden') return { allowed: false, state: 'forbidden', reason: verdicts.knowledge.reason };
            const resource = knowledge.find((item) => item.id === input.knowledgeId);
            if (!resource) return { allowed: false, state: 'unavailable', reason: 'knowledge_not_found' };
            const ref = knowledgeRefForPrompt(resource, { revoked: false });
            return ref === null
              ? { allowed: false, state: 'unavailable', reason: 'knowledge_not_selectable' }
              : { allowed: true, selection: ref };
          }
          return { allowed: false, state: 'unavailable', reason: 'selection_required' };
        },
        subscribe(listener) {
          listeners.add(listener);
          return () => listeners.delete(listener);
        },
        close(reason: ShelfCloseReason) {
          if (closed) return;
          closed = true;
          notify({ type: 'scope-closed', reason });
        },
      };
      return handle;
    },
  };
}
```

(d) 新建 `packages/mobile-core/src/shelf/in-memory-resource-remote.ts`：

```ts
import type { ResourceRemote } from './ports.ts';

/** in-memory 场景脚本：status 可为常数或按 token 判定（用于 401 刷新重试场景）。 */
export interface ResourceRemoteScript {
  agents?: ReadonlyArray<Record<string, unknown>>;
  disabledOwnAgentIds?: ReadonlyArray<string>;
  knowledgeBases?: ReadonlyArray<Record<string, unknown>>;
  connections?: ReadonlyArray<Record<string, unknown>>;
  status?: {
    agents?: number | ((token: string) => number | undefined);
    knowledgeBases?: number | ((token: string) => number | undefined);
    connections?: number | ((token: string) => number | undefined);
  };
}

export interface ScriptedResourceRemote extends ResourceRemote {
  calls: Array<{ kind: 'agents' | 'knowledgeBases' | 'connections'; token: string }>;
}

function httpError(status: number): Error {
  const error = new Error(`HTTP ${status}`);
  (error as { status?: number }).status = status;
  return error;
}

function resolveStatus(status: number | ((token: string) => number | undefined) | undefined, token: string): number | undefined {
  return typeof status === 'function' ? status(token) : status;
}

function respond<T>(rows: T, status: number | undefined): T {
  if (status !== undefined) throw httpError(status);
  return rows;
}

export function createInMemoryResourceRemote(script: ResourceRemoteScript): ScriptedResourceRemote {
  const calls: ScriptedResourceRemote['calls'] = [];
  return {
    calls,
    async availableAgents(token) {
      calls.push({ kind: 'agents', token });
      return respond(
        { rows: [...(script.agents ?? [])], disabledOwnAgentIds: new Set(script.disabledOwnAgentIds ?? []) },
        resolveStatus(script.status?.agents, token),
      );
    },
    async knowledgeBases(token) {
      calls.push({ kind: 'knowledgeBases', token });
      return respond([...(script.knowledgeBases ?? [])], resolveStatus(script.status?.knowledgeBases, token));
    },
    async connections(token) {
      calls.push({ kind: 'connections', token });
      return respond([...(script.connections ?? [])], resolveStatus(script.status?.connections, token));
    },
  };
}
```

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/mobile-core/src/shelf/resource-shelf.test.ts`
Expected: PASS（9 个测试全绿）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/shelf/ports.ts packages/mobile-core/src/shelf/types.ts packages/mobile-core/src/shelf/resource-shelf.ts packages/mobile-core/src/shelf/in-memory-resource-remote.ts packages/mobile-core/src/shelf/resource-shelf.test.ts
git commit -m "feat(mobile-core): resource shelf module with revocation-aware projection"
```

---

### Task 4: Runtime 接线（启动/失效/token seam）

**Files:**
- Modify: `packages/mobile-core/src/runtime/ports.ts`（`MobileRuntimePorts` 增加 `resourceShelf?`——锚点：#32 加入的 `scopedVault?` 之后；文件顶部新增 type-only import）
- Modify: `packages/mobile-core/src/runtime/types.ts`（`MobileRuntime` 增加 `resourceShelf()`——锚点：`scopeLease(): ScopeLease | undefined;` 之后）
- Modify: `packages/mobile-core/src/runtime/mobile-runtime.ts`（锚点见 Step 3，全部按符号定位：#32 已重写该文件，行号以其合并结果为准）
- Modify: `packages/mobile-core/src/index.ts`（导出 shelf 公共类型与工厂）
- Test: `packages/mobile-core/src/runtime/runtime-shelf.test.ts`（新建）

**Interfaces:**
- Consumes: Task 2/3 全部 Produces；#32 的 `MobileRuntime.activateTenant(tenantId: string): Promise<RuntimeSnapshot>`、`revoke(vaultReason)` 单一咽喉（deployment-change/tenant-switch/sign-out/dispose 四个失效路径都经过它——plan-t32 Task 4 已实装）、`refreshedCredential(requestEpoch, deployment, credential)` 单飞刷新（`packages/mobile-core/src/runtime/mobile-runtime.ts:119-137`，#32 未改其签名）、`RuntimeRemote.refresh/memberships`。
- Produces:
  - `MobileRuntimePorts.resourceShelf?: { remoteFor(origin: string): ResourceRemote }`（可选端口，缺省时行为与 T01/T02 完全一致）
  - `MobileRuntime.resourceShelf(): ResourceShelfHandle | undefined`（仅 authorized 且提供端口时非空）
  - Runtime 包内 seam `accessTokenFor(origin: string, options?: { refresh?: boolean }): Promise<string>`（传给 `createResourceShelf`；非当前 scope 抛 `SHELF_SCOPE`，强制刷新失败抛 `SHELF_AUTH`；token 不出现在任何公共类型）
  - `packages/mobile-core/src/index.ts` 新增公共导出：`createResourceShelf`、`createInMemoryResourceRemote`、类型 `ResourceRemote/ResourceShelfPorts/ResourceClass/ResourceClassVerdict/ResourcePage/ResourceQuery/ResourceShelf/ResourceShelfHandle/SelectionVerdict/ShelfCloseReason/ShelfInvalidationEvent`

- [ ] **Step 1: 写失败测试**

新建 `packages/mobile-core/src/runtime/runtime-shelf.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createMobileRuntime } from './mobile-runtime.ts';
import { createInMemoryResourceRemote, type ResourceRemoteScript } from '../shelf/in-memory-resource-remote.ts';
import type { ResourceRemote } from '../shelf/ports.ts';
import type { CredentialStore, RuntimeRemote, StoredCredential } from './ports.ts';
import type { DeploymentInput } from './types.ts';

const DEPLOYMENT: DeploymentInput = { origin: 'https://weknora.example.test', label: 'Test Deployment' };
const OTHER: DeploymentInput = { origin: 'https://other.example.test', label: 'Other' };
const CAPABILITIES = { protocol_minimum: 2, protocol_maximum: 3 };

function fakeStore(initial: Record<string, StoredCredential | undefined> = {}): CredentialStore {
  const values = new Map(Object.entries(initial));
  return {
    async read(deployment) { return values.get(deployment); },
    async write(deployment, credential) { values.set(deployment, credential); },
    async clear(deployment) { values.delete(deployment); },
  };
}

function baseRemote(): RuntimeRemote {
  return {
    passwordLogin: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    me: async (token) => ({
      user: { id: 'member-1' },
      tenant: { id: token === 'tenant-2-access' ? 'tenant-2' : 'tenant-1' },
      memberships: [{ tenant_id: 1 }, { tenant_id: 2 }],
    }),
    deploymentCapabilities: async () => CAPABILITIES,
    oidcUrl: async () => ({ authorizationUrl: 'https://idp.example.test/authorize', state: 'state-1' }),
    oidcExchange: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    oidcNativeExchange: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    refresh: async () => ({ access_token: 'access-2', refresh_token: 'refresh-2' }),
    switchTenant: async (input) => ({
      credential: { token: `tenant-${input.tenantId}-access`, refreshToken: `refresh-${input.tenantId}` },
      tenant: { id: `tenant-${input.tenantId}` },
    }),
  };
}

/** 按 token 区分租户事实的内存资源远端（跨租户隔离断言用）。 */
function tenantResourceRemote(): ResourceRemote & { calls: Array<{ kind: string; token: string }> } {
  const calls: Array<{ kind: string; token: string }> = [];
  return {
    calls,
    async availableAgents(token) {
      calls.push({ kind: 'agents', token });
      return { rows: [{ id: `agent-${token}`, name: `Agent ${token}`, summary: '', kind: 'general', capability: { state: 'supported', reason: '' } }], disabledOwnAgentIds: new Set<string>() };
    },
    async knowledgeBases(token) {
      calls.push({ kind: 'knowledgeBases', token });
      return [{ id: `kb-${token}`, title: `KB ${token}`, scan_status: 'indexed', document_count: 0, updated_at: '' }];
    },
    async connections(token) {
      calls.push({ kind: 'connections', token });
      return [];
    },
  };
}

test('an authorized runtime exposes a resource shelf bound to the active tenant', async () => {
  const resource = tenantResourceRemote();
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => baseRemote(),
    clientVersion: 3,
    resourceShelf: { remoteFor: () => resource },
  });
  assert.equal(runtime.resourceShelf(), undefined, 'no shelf before authorization');

  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });

  const handle = runtime.resourceShelf();
  assert.ok(handle, 'an authorized scope must expose the shelf');
  const page = await handle.browse();
  assert.equal(page.tenantId, 'tenant-1');
  assert.deepEqual(page.agents.map((agent) => agent.id), ['agent-access-1']);
});

test('switching tenants closes the old shelf and serves the new tenant only', async () => {
  const resource = tenantResourceRemote();
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => baseRemote(),
    clientVersion: 3,
    resourceShelf: { remoteFor: () => resource },
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const first = runtime.resourceShelf()!;
  const events: unknown[] = [];
  first.subscribe((event) => events.push(event));

  const snapshot = await runtime.activateTenant('2');

  assert.equal(snapshot.identity?.activeTenantId, 'tenant-2');
  await assert.rejects(first.browse(), /SHELF_SCOPE_CLOSED/);
  assert.deepEqual(events, [{ type: 'scope-closed', reason: 'tenant-switch' }]);
  const second = runtime.resourceShelf();
  assert.ok(second);
  assert.notEqual(second, first);
  const page = await second.browse();
  assert.equal(page.tenantId, 'tenant-2');
  assert.deepEqual(page.agents.map((agent) => agent.id), ['agent-tenant-2-access'], 'the new page is rebuilt from the new tenant facts only');
});

test('sign-out and deployment changes close the shelf with their reasons', async () => {
  const resource = tenantResourceRemote();
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => baseRemote(),
    clientVersion: 3,
    resourceShelf: { remoteFor: () => resource },
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const first = runtime.resourceShelf()!;
  const firstEvents: unknown[] = [];
  first.subscribe((event) => firstEvents.push(event));

  await runtime.signOut();
  assert.equal(runtime.resourceShelf(), undefined);
  await assert.rejects(first.browse(), /SHELF_SCOPE_CLOSED/);
  assert.deepEqual(firstEvents, [{ type: 'scope-closed', reason: 'sign-out' }]);

  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const second = runtime.resourceShelf()!;
  const secondEvents: unknown[] = [];
  second.subscribe((event) => secondEvents.push(event));

  await runtime.signIn({ deployment: OTHER, email: 'member@example.test', password: 'password' });

  await assert.rejects(second.browse(), /SHELF_SCOPE_CLOSED/);
  assert.deepEqual(secondEvents, [{ type: 'scope-closed', reason: 'deployment-change' }]);
});

test('browse retries once through the runtime refresh seam and persists the rotated credential', async () => {
  const script: ResourceRemoteScript = {
    agents: [{ id: 'builtin-quick-answer', name: 'Quick Answer', summary: '', kind: 'general', capability: { state: 'supported', reason: '' } }],
  };
  script.status = { agents: (token: string) => (token === 'access-1' ? 401 : undefined) };
  const resource = createInMemoryResourceRemote(script);
  const store = fakeStore();
  const runtime = createMobileRuntime({
    credentialStore: store,
    remoteFor: () => baseRemote(),
    clientVersion: 3,
    resourceShelf: { remoteFor: () => resource },
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });

  const page = await runtime.resourceShelf()!.browse();

  assert.deepEqual(page.agents.map((agent) => agent.id), ['builtin-quick-answer']);
  assert.deepEqual(resource.calls.filter((call) => call.kind === 'agents').map((call) => call.token), ['access-1', 'access-2']);
  assert.equal((await store.read(DEPLOYMENT.origin))?.token, 'access-2', 'the rotated credential must be persisted through the runtime single-flight');
});

test('a capability downgrade never opens a shelf', async () => {
  const resource = tenantResourceRemote();
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => baseRemote(),
    clientVersion: 99,
    resourceShelf: { remoteFor: () => resource },
  });

  const snapshot = await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });

  assert.equal(snapshot.surface, 'upgrade-required');
  assert.equal(runtime.resourceShelf(), undefined);
  assert.equal(resource.calls.length, 0, 'no resource request may leave without an authorized scope');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/mobile-core/src/runtime/runtime-shelf.test.ts`
Expected: FAIL——首个测试 `runtime.resourceShelf is not a function`（tsx 不做类型检查，运行期 TypeError 即 RED 证据；`ports.resourceShelf` 未声明时被 Runtime 忽略属预期）。

- [ ] **Step 3: 最小实现**

(a) `packages/mobile-core/src/runtime/ports.ts`：顶部（`RuntimeRemote` 声明之后任意位置，沿用 #32 对 `ScopedVault` 的 type-only 引入方式）加：

```ts
import type { ResourceRemote } from '../shelf/ports.ts';
```

`MobileRuntimePorts` 中 #32 加入的 `scopedVault?: ScopedVault;` 之后加：

```ts
  /** T03: Resource Shelf bindings. When present the Runtime opens one shelf per authorized scope and closes it on every scope change. */
  resourceShelf?: { remoteFor(origin: string): ResourceRemote };
```

(b) `packages/mobile-core/src/runtime/types.ts`：顶部加 `import type { ResourceShelfHandle } from '../shelf/types.ts';`（type-only 循环引用合法，同 #32 先例）；`MobileRuntime` 的 `scopeLease(): ScopeLease | undefined;` 之后加：

```ts
  /** Resource Shelf for the active scope; undefined unless authorized with ports.resourceShelf provided. */
  resourceShelf(): ResourceShelfHandle | undefined;
```

(c) `packages/mobile-core/src/runtime/mobile-runtime.ts`（锚点全部按符号；#32 合并后该文件已有 `RuntimeScopeLease` 导入、`queueVaultRevoke`/`revoke(vaultReason)`/`begin(deployment, vaultReason)`/`reserve(vaultReason)`、`authenticate` 内 `revocableLease = new RuntimeScopeLease({ deploymentOrigin, userId, tenantId }); lease = revocableLease.asScopeLease();`）：

1. 顶部加：

```ts
import { createResourceShelf } from '../shelf/resource-shelf.ts';
import type { ResourceShelfHandle } from '../shelf/types.ts';
```

2. 闭包变量区（`let revocableLease: RuntimeScopeLease | undefined;` 之后）加：

```ts
  let activeShelf: ResourceShelfHandle | undefined;
  let activeCredential: StoredCredential | undefined;
```

3. `refreshedCredential` 函数定义之后加（token seam——包内唯一持有当前 scope 凭据的位置，token 不进任何公共类型）：

```ts
  const accessTokenFor = async (origin: string, options?: { refresh?: boolean }): Promise<string> => {
    const deployment = activeDeployment;
    if (!deployment || deployment.origin !== origin || state.surface !== 'authorized' || !activeCredential) throw new Error('SHELF_SCOPE');
    if (!options?.refresh) return activeCredential.token;
    const refreshed = await refreshedCredential(epoch, deployment, activeCredential);
    if (!refreshed) throw new Error('SHELF_AUTH');
    activeCredential = refreshed;
    return refreshed.token;
  };
```

4. `revoke(vaultReason)`（#32 版本）体内、`lease = undefined;` 之后加三行——所有 scope 失效路径（deployment-change/tenant-switch/sign-out/dispose）的单一咽喉：

```ts
    activeShelf?.close(vaultReason);
    activeShelf = undefined;
    activeCredential = undefined;
```

5. `authenticate` 内（#32 版本）`lease = revocableLease.asScopeLease();` 之后、`return publish({...})` 之前加：

```ts
      activeCredential = verifiedCredential;
      activeShelf = ports.resourceShelf
        ? createResourceShelf({ remote: ports.resourceShelf.remoteFor(deployment.origin), accessTokenFor }).open({ lease })
        : undefined;
```

6. 返回对象中 `scopeLease: () => lease,` 之后加：

```ts
    resourceShelf: () => activeShelf,
```

(d) `packages/mobile-core/src/index.ts` 末尾追加：

```ts
export { createResourceShelf } from './shelf/resource-shelf.ts';
export { createInMemoryResourceRemote } from './shelf/in-memory-resource-remote.ts';
export type { ResourceRemote, ResourceShelfPorts } from './shelf/ports.ts';
export type { ResourceClass, ResourceClassVerdict, ResourcePage, ResourceQuery, ResourceShelf, ResourceShelfHandle, SelectionVerdict, ShelfCloseReason, ShelfInvalidationEvent } from './shelf/types.ts';
```

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/mobile-core/src/runtime/runtime-shelf.test.ts packages/mobile-core/src/runtime/mobile-runtime.test.ts packages/mobile-core/src/runtime/runtime-vault.test.ts packages/mobile-core/src/shelf/resource-shelf.test.ts`
Expected: PASS（新文件 5 个测试 + #32 既有 runtime/vault/shelf 测试全绿，证明接线无回归）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/runtime/ports.ts packages/mobile-core/src/runtime/types.ts packages/mobile-core/src/runtime/mobile-runtime.ts packages/mobile-core/src/runtime/runtime-shelf.test.ts packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core): runtime owns resource shelf lifecycle with scope-bound token seam"
```

---

### Task 5: apps/mobile Resources 页与组装

**Files:**
- Create: `apps/mobile/src/screens/ResourcesScreen.tsx`
- Create: `apps/mobile/src/resources-view.ts`
- Create: `apps/mobile/src/app/resources.tsx`（expo-router 文件路由 → `/resources`）
- Test: `apps/mobile/src/resources-view.test.ts`（新建）
- Modify: `apps/mobile/src/composition.ts`（`createNativeMobileRuntime` 注入 `resourceShelf.remoteFor`；新增 `activeMobileRuntime` 导出）
- Modify: `apps/mobile/src/screens/AuthorizedLandingScreen.tsx`（#32 后形态：新增「Open Resources」入口按钮）
- Modify: `apps/mobile/src/app-smoke.test.tsx`（expo-router stub 补 `router.navigate`；`descendants` 助手支持嵌套数组 children；末尾追加 3 个测试）

**Interfaces:**
- Consumes: Task 4 的 `MobileRuntime.resourceShelf(): ResourceShelfHandle | undefined`、`ResourcePage/ResourceShelfHandle/ShelfInvalidationEvent`（`@weknora/mobile-core`）；Task 2 的 `createMobileResourceRemote`（`@weknora/api-client/mobile/resources`）；Task 1 的 `scanStatusPresentation`（`@weknora/domain/mobile`）；既有 `createWeKnoraClient`/`createJsonTransport`/`nativeFetch`（`apps/mobile/src/composition.ts:20-45`）；#32 后的 `AuthorizedLandingScreenProps`（含 `tenants`/`onActivateTenant`）。
- Produces:
  - `ResourcesScreenProps { page?: ResourcePage; loading: boolean; error?: string; onRefresh(): void }` 与 `ResourcesScreen`（纯展示，仅消费接口投影）
  - `createResourceShelfController(handle: ResourceShelfHandle): ResourceShelfController`，`ResourceShelfController = { state(): { page?: ResourcePage; loading: boolean; error?: string }; subscribe(listener): () => void; refresh(): Promise<void>; whenSettled(): Promise<void>; dispose(): void }`
  - `activeMobileRuntime(): MobileRuntime`（composition 导出；路由文件经它取 singleton runtime）
  - `/resources` 路由（`src/app/resources.tsx` 默认导出）与 landing 的「Open Resources」按钮（`router.navigate('/resources')`）
  - Task 6 消费同一 composition 端口形状。

- [ ] **Step 1: 写失败测试**

(a) 新建 `apps/mobile/src/resources-view.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createResourceShelfController } from './resources-view.ts';
import type { ResourcePage, ResourceShelfHandle, ShelfInvalidationEvent } from '@weknora/mobile-core';

const PAGE: ResourcePage = {
  tenantId: '7',
  agents: [{ id: 'agent-1', name: 'Research', summary: '', kind: 'custom', capability: { state: 'supported', reason: '' } }],
  knowledge: [],
  connections: [],
  classVerdicts: { agent: { state: 'supported', reason: '' }, knowledge: { state: 'supported', reason: '' }, connection: { state: 'supported', reason: '' } },
};

function fakeHandle(pages: ResourcePage[]): ResourceShelfHandle & { emit(event: ShelfInvalidationEvent): void } {
  let next = 0;
  const listeners = new Set<(event: ShelfInvalidationEvent) => void>();
  return {
    async browse() {
      const page = pages[Math.min(next, pages.length - 1)]!;
      next += 1;
      return page;
    },
    selection: () => ({ allowed: false, state: 'unavailable', reason: 'not_used' }),
    subscribe(listener) { listeners.add(listener); return () => listeners.delete(listener); },
    close() {},
    emit(event) { for (const listener of [...listeners]) listener(event); },
  };
}

test('the controller loads the page and reloads on authorization revocation', async () => {
  const revoked: ResourcePage = {
    ...PAGE,
    knowledge: [],
    classVerdicts: { ...PAGE.classVerdicts, knowledge: { state: 'forbidden', reason: 'http_403' } },
  };
  const handle = fakeHandle([PAGE, revoked]);
  const controller = createResourceShelfController(handle);

  await controller.whenSettled();
  assert.equal(controller.state().page?.classVerdicts.knowledge.state, 'supported');

  handle.emit({ type: 'authorization-revoked', resourceClass: 'knowledge' });
  await controller.whenSettled();

  assert.equal(controller.state().page?.classVerdicts.knowledge.state, 'forbidden', 'revocation must replace the projection with server facts');
  assert.equal(controller.state().page?.knowledge.length, 0);
});

/** LIFO 解锁：resolveNext 总是解决最新一次在途 browse，便于构造「旧请求后返回」的迟到场景。 */
function deferredHandle(): ResourceShelfHandle & { resolveNext(page: ResourcePage): void } {
  const pending: Array<(page: ResourcePage) => void> = [];
  return {
    browse: () => new Promise<ResourcePage>((resolve) => { pending.push(resolve); }),
    selection: () => ({ allowed: false, state: 'unavailable', reason: 'not_used' }),
    subscribe: () => () => {},
    close() {},
    resolveNext(page) { pending.pop()?.(page); },
  };
}

test('a stale in-flight projection never overwrites a newer one', async () => {
  const handle = deferredHandle();
  const controller = createResourceShelfController(handle);
  const newer: ResourcePage = { ...PAGE, tenantId: '9' };

  const reloaded = controller.refresh();
  handle.resolveNext(newer); // 最新一次 browse 先返回
  await reloaded;
  handle.resolveNext(PAGE); // 迟到的旧结果必须被丢弃
  await new Promise((resolve) => setTimeout(resolve, 0));

  assert.equal(controller.state().page?.tenantId, '9');
});
```

(b) `apps/mobile/src/app-smoke.test.tsx` 修改与追加：

1. `NATIVE_MODULE_STUBS` 的 `'expo-router'` stub 改为（补 `navigate`）：

```ts
  'expo-router': "module.exports = { Stack: function Stack() { return null; }, router: { replace() {}, navigate() {} } }",
```

2. `descendants` 助手中这一行：

```ts
  return [...here, ...(Array.isArray(children) ? children : [children]).flatMap(descendants)];
```

改为（支持 JSX `.map()` 产生的嵌套数组 children）：

```ts
  const childList = Array.isArray(children) ? children.flat(Infinity) : [children];
  return [...here, ...childList.flatMap(descendants)];
```

3. 文件末尾追加：

```tsx
test('the resources screen renders only the Resource Shelf projection with explicit states', async () => {
  const { ResourcesScreen } = await import('./screens/ResourcesScreen.tsx');
  hooks().__reset();
  const base = {
    loading: false,
    onRefresh: () => {},
  };
  const element = render(ResourcesScreen, {
    ...base,
    page: {
      tenantId: '7',
      agents: [
        { id: 'agent-1', name: 'Research', summary: '', kind: 'custom', capability: { state: 'supported', reason: '' } },
        { id: 'agent-2', name: 'Blocked', summary: '', kind: 'custom', capability: { state: 'forbidden', reason: 'policy' } },
      ],
      knowledge: [{ id: 'kb-1', title: 'Handbook', scanStatus: 'indexed', documentCount: 3, updatedAt: '2026-09-01T00:00:00Z' }],
      connections: [{ id: 'conn-1', kind: 'personal', state: 'revoked', connected: false, capability: { state: 'unavailable', reason: 'connection_revoked' } }],
      classVerdicts: { agent: { state: 'supported', reason: '' }, knowledge: { state: 'supported', reason: '' }, connection: { state: 'supported', reason: '' } },
    },
  });
  const texts = descendants(element)
    .filter(({ type }) => type === 'Text')
    .flatMap(({ props }) => props.children)
    .flatMap((part) => (typeof part === 'string' ? [part] : []));

  assert.equal(texts.some((text) => text.includes('tenant 7')), true);
  assert.equal(texts.some((text) => text.includes('Research')), true);
  assert.equal(texts.some((text) => text.includes('Blocked') && text.includes('policy')), true, 'forbidden agents must explain their reason');
  assert.equal(texts.some((text) => text.includes('Handbook') && text.includes('已索引')), true);
  assert.equal(texts.some((text) => text.includes('connection_revoked')), true, 'connection state must be explained');
  const buttons = descendants(element).filter(({ type }) => type === 'Button').map(({ props }) => props.title);
  assert.equal(buttons.includes('Refresh'), true);

  const revokedElement = render(ResourcesScreen, {
    ...base,
    page: {
      tenantId: '7',
      agents: [],
      knowledge: [],
      connections: [],
      classVerdicts: { agent: { state: 'supported', reason: '' }, knowledge: { state: 'forbidden', reason: 'http_403' }, connection: { state: 'supported', reason: '' } },
    },
  });
  const revokedTexts = descendants(revokedElement)
    .filter(({ type }) => type === 'Text')
    .flatMap(({ props }) => props.children)
    .flatMap((part) => (typeof part === 'string' ? [part] : []));
  assert.equal(revokedTexts.some((text) => text.includes('Access revoked') && text.includes('http_403')), true, 'the forbidden class banner must show the reason');
  assert.equal(revokedTexts.some((text) => text.includes('Handbook')), false, 'revoked knowledge rows must disappear');
});

test('the resources route and landing entry consume the shelf interface only', async () => {
  const route = await import('./app/resources.tsx');
  assert.equal(typeof route.default, 'function', 'src/app/resources.tsx must default-export the Expo Router screen');

  const { AuthorizedLandingScreen } = await import('./screens/AuthorizedLandingScreen.tsx');
  hooks().__reset();
  const element = render(AuthorizedLandingScreen, {
    deploymentLabel: 'WeKnora',
    userId: 'member-1',
    tenantId: '7',
    tenants: [{ id: '7', name: 'Acme', active: true }],
    onSignOut: async () => {},
    onActivateTenant: async () => {},
  });
  const buttons = descendants(element).filter(({ type }) => type === 'Button').map(({ props }) => props.title);
  assert.equal(buttons.includes('Open Resources'), true);
});

test('the resources view modules never import contracts or api-client wire adapters', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  for (const relative of ['screens/ResourcesScreen.tsx', 'resources-view.ts', 'app/resources.tsx']) {
    const source = readFileSync(join(here, relative), 'utf8');
    assert.equal(/@weknora\/(api-client|contracts)/.test(source), false, `${relative} must consume the Resource Shelf Interface only (AC2)`);
  }
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL——`resources-view.test.ts` 报 `Cannot find module './resources-view.ts'`；app-smoke 新增测试报 `Cannot find module './app/resources.tsx'` / `'./screens/ResourcesScreen.tsx'`（文件不存在即 RED 证据；既有 11 个测试仍绿）。

- [ ] **Step 3: 最小实现**

(a) 新建 `apps/mobile/src/screens/ResourcesScreen.tsx`：

```tsx
import { Button, Text, View } from 'react-native';
import { scanStatusPresentation } from '@weknora/domain/mobile';
import type { ResourcePage } from '@weknora/mobile-core';

export interface ResourcesScreenProps {
  page?: ResourcePage;
  loading: boolean;
  error?: string;
  onRefresh(): void;
}

const CLASS_TITLES = { agent: 'Agents', knowledge: 'Knowledge', connection: 'Connections' } as const;
const CAPABILITY_LABELS = { supported: 'Available', unavailable: 'Unavailable', forbidden: 'Access revoked' } as const;

function capabilitySuffix(state: 'supported' | 'unavailable' | 'forbidden', reason: string): string {
  return state === 'supported' ? '' : ` — ${CAPABILITY_LABELS[state]} (${reason})`;
}

/** Resources 页：只消费 Resource Shelf Interface 的投影（AC2）；三态必须解释原因，撤权类不渲染行。 */
export function ResourcesScreen({ page, loading, error, onRefresh }: ResourcesScreenProps) {
  if (!page) {
    return (
      <View>
        <Text>{error ?? (loading ? 'Loading resources…' : 'No resources')}</Text>
        {error ? <Button title="Retry" onPress={onRefresh} /> : null}
      </View>
    );
  }
  return (
    <View>
      <Text>{`Resources for tenant ${page.tenantId}`}</Text>
      <Button title="Refresh" onPress={onRefresh} />
      {(['agent', 'knowledge', 'connection'] as const).flatMap((resourceClass) => {
        const verdict = page.classVerdicts[resourceClass];
        if (verdict.state === 'supported') return [];
        return [<Text key={`${resourceClass}-verdict`}>{`${CLASS_TITLES[resourceClass]}: ${CAPABILITY_LABELS[verdict.state]} (${verdict.reason})`}</Text>];
      })}
      {page.agents.map((agent) => (
        <Text key={agent.id}>{`${agent.name}${capabilitySuffix(agent.capability.state, agent.capability.reason)}`}</Text>
      ))}
      {page.knowledge.map((resource) => (
        <Text key={resource.id}>{`${resource.title} — ${scanStatusPresentation(resource.scanStatus).label}`}</Text>
      ))}
      {page.connections.map((connection) => (
        <Text key={connection.id}>{`${connection.kind} connection — ${connection.connected ? 'connected' : connection.capability.reason}`}</Text>
      ))}
    </View>
  );
}
```

(b) 新建 `apps/mobile/src/resources-view.ts`：

```ts
import type { ResourcePage, ResourceShelfHandle } from '@weknora/mobile-core';

export interface ResourceShelfViewState {
  page?: ResourcePage;
  loading: boolean;
  error?: string;
}

export interface ResourceShelfController {
  state(): ResourceShelfViewState;
  subscribe(listener: (state: ResourceShelfViewState) => void): () => void;
  refresh(): Promise<void>;
  whenSettled(): Promise<void>;
  dispose(): void;
}

/** Resources 页控制器：失效事件触发重取；迟到结果按代次丢弃（不从缓存回填旧投影——AC1）。 */
export function createResourceShelfController(handle: ResourceShelfHandle): ResourceShelfController {
  let state: ResourceShelfViewState = { loading: true };
  let generation = 0;
  let tail: Promise<void> = Promise.resolve();
  const listeners = new Set<(state: ResourceShelfViewState) => void>();

  const publish = (next: ResourceShelfViewState): void => {
    state = next;
    for (const listener of [...listeners]) listener(state);
  };
  const load = (): Promise<void> => {
    const run = ++generation;
    publish({ ...state, loading: true });
    const attempt = (async (): Promise<void> => {
      try {
        const page = await handle.browse();
        if (run === generation) publish({ page, loading: false });
      } catch (cause) {
        if (run === generation) publish({ page: undefined, loading: false, error: cause instanceof Error ? cause.message : String(cause) });
      }
    })();
    tail = attempt;
    return attempt;
  };
  const unsubscribe = handle.subscribe((event) => {
    if (event.type === 'authorization-revoked' || event.type === 'scope-closed') void load();
  });
  void load();
  return {
    state: () => state,
    subscribe(listener) { listeners.add(listener); return () => listeners.delete(listener); },
    refresh: load,
    whenSettled: () => tail,
    dispose() { unsubscribe(); listeners.clear(); generation += 1; },
  };
}
```

(c) 新建 `apps/mobile/src/app/resources.tsx`：

```tsx
import { useEffect, useRef, useState } from 'react';
import { Text, View } from 'react-native';
import { activeMobileRuntime } from '../composition.ts';
import { createResourceShelfController, type ResourceShelfViewState } from '../resources-view.ts';
import { ResourcesScreen } from '../screens/ResourcesScreen.tsx';

/** Expo Router 文件路由：/resources。只消费 Resource Shelf Interface（AC2）。 */
export default function ResourcesRoute() {
  const controllerRef = useRef<ReturnType<typeof createResourceShelfController> | undefined>(undefined);
  if (!controllerRef.current) {
    const handle = activeMobileRuntime().resourceShelf();
    controllerRef.current = handle ? createResourceShelfController(handle) : undefined;
  }
  const [state, setState] = useState<ResourceShelfViewState>(controllerRef.current?.state() ?? { loading: false });
  useEffect(() => {
    const controller = controllerRef.current;
    if (!controller) return;
    setState(controller.state());
    return controller.subscribe(setState);
  }, []);
  if (!controllerRef.current) {
    return (
      <View>
        <Text>Sign in to browse tenant resources.</Text>
      </View>
    );
  }
  return <ResourcesScreen page={state.page} loading={state.loading} error={state.error} onRefresh={() => { controllerRef.current?.refresh(); }} />;
}
```

(d) `apps/mobile/src/composition.ts`：

1. import 区（`createMobileRuntimeRemote` 导入之后）加：

```ts
import { createMobileResourceRemote } from '@weknora/api-client/mobile/resources';
```

2. `createNativeMobileRuntime` 的 ports 中 `remoteFor(origin) {...}` 之后加（复用同一传输构造方式，不新建 HTTP client）：

```ts
    resourceShelf: {
      remoteFor(origin) {
        const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(nativeFetch) });
        return createMobileResourceRemote({ origin, request: client.request });
      },
    },
```

3. `function runtime(): MobileRuntime {...}` 之后加：

```ts
/** Route files reach the app-lifetime runtime through this accessor only. */
export function activeMobileRuntime(): MobileRuntime {
  return runtime();
}
```

(e) `apps/mobile/src/screens/AuthorizedLandingScreen.tsx`（#32 后形态，props 含 `tenants`/`onActivateTenant`）：顶部加 `import { router } from 'expo-router';`；在租户切换按钮与「Sign out」之间加：

```tsx
      <Button title="Open Resources" onPress={() => { router.navigate('/resources'); }} />
```

- [ ] **Step 4: 运行确认通过**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: PASS（既有 11 + 新增 5 个测试全绿；strict tsc 0 错）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/screens/ResourcesScreen.tsx apps/mobile/src/resources-view.ts apps/mobile/src/resources-view.test.ts apps/mobile/src/app/resources.tsx apps/mobile/src/composition.ts apps/mobile/src/screens/AuthorizedLandingScreen.tsx apps/mobile/src/app-smoke.test.tsx
git commit -m "feat(mobile): resources screen consumes the resource shelf interface with invalidation reload"
```

---

### Task 6: 真实 HTTP 集成证据（resourceShelf browse，AC3）

**Files:**
- Modify: `apps/mobile/src/runtime-integration-smoke.ts`（imports、runtime ports 增加 `resourceShelf`、Evidence 类型、`collectResourceShelfEvidence` 导出、`runMobileRuntimeIntegration` 末段）
- Modify: `packages/api-client/src/mobile/runtime.integration.test.ts`（主测试断言 + evidence fixture 补字段 + 新增本地 `collectResourceShelfEvidence` 测试）

**Interfaces:**
- Consumes: Task 4 的 `MobileRuntime.resourceShelf()`；Task 2 的 `createMobileResourceRemote`；T01/T02 的 `mobileRuntimeIntegrationConfig`/`runMobileRuntimeIntegration`/`emitMobileRuntimeIntegrationEvidence` 与 opt-in 环境变量 `WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/_EMAIL/_PASSWORD`（+ 可选 `_SWITCH_TENANT_ID`）。
- Produces:
  - `collectResourceShelfEvidence(runtime: { resourceShelf(): ResourceShelfHandle | undefined }): Promise<Pick<MobileRuntimeIntegrationEvidence, 'resourceShelf' | 'resourceCounts'>>`
  - `MobileRuntimeIntegrationEvidence` 增加 `resourceShelf: 'not-authorized' | 'browsed' | 'browse-failed'` 与 `resourceCounts?: { agents: number; knowledge: number; connections: number }`（仅计数，无任何凭据/响应原文）

- [ ] **Step 1: 写失败测试**

`packages/api-client/src/mobile/runtime.integration.test.ts`：

(a) 顶部 import 中加入 `collectResourceShelfEvidence`：

```ts
import { collectResourceShelfEvidence, emitMobileRuntimeIntegrationEvidence, mobileRuntimeIntegrationConfig, runMobileRuntimeIntegration } from '../../../../apps/mobile/src/runtime-integration-smoke.ts';
```

(b) 主测试 `assert.equal(evidence.tenantSwitch, ...)`（#32 Task 6 加入）之后追加：

```ts
  assert.equal(evidence.resourceShelf, 'browsed', 'an authorized runtime must browse real tenant resources through the shelf');
```

(c) evidence 回显测试（48-69 行，#32 后含 `tenantSwitch: 'skipped'`）的 fixture 与 `JSON.parse` 期望对象各补一个字段 `resourceShelf: 'browse-failed'`。

(d) 文件末尾追加（本地可跑，不依赖环境变量）：

```ts
test('resource shelf evidence distinguishes not-authorized, browsed and failed handles', async () => {
  const healthyHandle = {
    browse: async () => ({
      tenantId: '7',
      agents: [{ id: 'a' }],
      knowledge: [],
      connections: [{ id: 'c1' }, { id: 'c2' }],
      classVerdicts: {},
    }),
    selection: () => ({ allowed: false, state: 'unavailable' as const, reason: 'unused' }),
    subscribe: () => () => {},
    close: () => {},
  };
  const failingHandle = { ...healthyHandle, browse: async () => { throw new Error('SHELF_SCOPE_CLOSED'); } };

  assert.deepEqual(
    await collectResourceShelfEvidence({ resourceShelf: () => healthyHandle } as Parameters<typeof collectResourceShelfEvidence>[0]),
    { resourceShelf: 'browsed', resourceCounts: { agents: 1, knowledge: 0, connections: 2 } },
  );
  assert.deepEqual(
    await collectResourceShelfEvidence({ resourceShelf: () => undefined } as Parameters<typeof collectResourceShelfEvidence>[0]),
    { resourceShelf: 'not-authorized' },
  );
  assert.deepEqual(
    await collectResourceShelfEvidence({ resourceShelf: () => failingHandle } as Parameters<typeof collectResourceShelfEvidence>[0]),
    { resourceShelf: 'browse-failed' },
  );
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/api-client/src/mobile/runtime.integration.test.ts`
Expected: FAIL——新增本地测试的 `collectResourceShelfEvidence` 具名导入不存在（模块加载失败即 RED 证据）；主测试无环境变量时 `t.skip`（blocked-env 属预期，不算 RED）。

- [ ] **Step 3: 最小实现**

`apps/mobile/src/runtime-integration-smoke.ts`：

1. imports 区加：

```ts
import { createMobileResourceRemote } from '@weknora/api-client/mobile/resources';
import type { ResourceShelfHandle } from '@weknora/mobile-core';
```

2. `MobileRuntimeIntegrationEvidence` 增加（`tenantSwitch` 之后）：

```ts
  resourceShelf: 'not-authorized' | 'browsed' | 'browse-failed';
  resourceCounts?: { agents: number; knowledge: number; connections: number };
```

3. `runMobileRuntimeIntegration` 之前新增导出函数：

```ts
/** Browses the real tenant resources through the shelf interface; evidence carries counts only. */
export async function collectResourceShelfEvidence(
  runtime: { resourceShelf(): ResourceShelfHandle | undefined },
): Promise<Pick<MobileRuntimeIntegrationEvidence, 'resourceShelf' | 'resourceCounts'>> {
  const handle = runtime.resourceShelf();
  if (!handle) return { resourceShelf: 'not-authorized' };
  try {
    const page = await handle.browse();
    return {
      resourceShelf: 'browsed',
      resourceCounts: { agents: page.agents.length, knowledge: page.knowledge.length, connections: page.connections.length },
    };
  } catch {
    return { resourceShelf: 'browse-failed' };
  }
}
```

4. `runMobileRuntimeIntegration` 中 `createMobileRuntime({...})` 的 ports 在 `remoteFor` 之后加：

```ts
    resourceShelf: {
      remoteFor(origin) {
        const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
        return createMobileResourceRemote({ origin, request: client.request });
      },
    },
```

5. 返回对象（#32 后的 `tenantSwitch` 计算段之后、`return {...}` 之前）加：

```ts
  const shelfEvidence = await collectResourceShelfEvidence(runtime);
```

并在 return 对象中 `tenantSwitch,` 之后展开：

```ts
    ...shelfEvidence,
```

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/api-client/src/mobile/runtime.integration.test.ts`
Expected: PASS（1 个 skip [主测试无环境变量] + 全部本地断言测试绿，skip 非 fail）。真部署验收（环境具备时）：

```bash
WEKNORA_MOBILE_TEST_DEPLOYMENT_URL=https://<deployment-origin> \
WEKNORA_MOBILE_TEST_EMAIL=<tenant-member-account> \
WEKNORA_MOBILE_TEST_PASSWORD=<short-lived-secret> \
npx tsx --test packages/api-client/src/mobile/runtime.integration.test.ts
```

Expected: 全绿，`t.diagnostic` 输出的 JSON evidence 含 `"resourceShelf":"browsed"` 与三类资源计数。凭据只来自环境变量（Mimosa 约束），占位符由运行者替换，不得写入任何文件。运行中实时撤权（browse 后由管理员撤销再观察失效）与真机渲染为 blocked-env 项，本地替代证据见「Global Constraints」末段。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/runtime-integration-smoke.ts packages/api-client/src/mobile/runtime.integration.test.ts
git commit -m "test(mobile): real-HTTP resource shelf browse evidence behind opt-in env credentials"
```

---

## 计划级验证

在 worktree 根（`.worktrees/issue30-sweep`）执行（作者已实跑各分量的既有部分：`npx tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts` 24/24 pass、`pnpm --filter @weknora/mobile test` 11/11 pass、`pnpm --filter @weknora/mobile typecheck` 0 错；`packages/mobile-core/src/runtime/runtime-vault.test.ts` 与 #32 其余产物由 #32 执行者实跑；新增测试由本计划执行者按任务逐步实跑）：

```bash
npx tsx --test packages/domain/src/mobile/agent-options.test.ts packages/domain/src/mobile/resource-presentation.test.ts packages/api-client/src/mobile/resources.test.ts packages/api-client/src/mobile/runtime.integration.test.ts packages/mobile-core/src/shelf/resource-shelf.test.ts packages/mobile-core/src/runtime/runtime-shelf.test.ts packages/mobile-core/src/runtime/mobile-runtime.test.ts packages/mobile-core/src/runtime/runtime-vault.test.ts && pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck
```

覆盖说明：Task 1-5 的全部定向单测/接口测 + Task 6 集成测试（无环境变量时真实 HTTP 用例 skip，本地断言全跑）+ apps/mobile 测试与 strict 类型检查（Task 5，同时传递编译 mobile-core/api-client 新文件）。有意不含 `pnpm test:shared` 全量与 web/desktop 套件（flaky 且与本计划无关）。

## 差异记录（调查结论 vs 代码现状）

1. 调查称「apps/mobile 依赖未安装（node_modules 缺失），无法给出测试证据」——与 worktree 现状冲突：`apps/mobile/node_modules` 存在（含 `@weknora/api-client|domain|mobile-core` 链接）。本计划作者在本 worktree 实跑：`npx tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts` 24/24 pass（fail 0）、`pnpm --filter @weknora/mobile test` 11/11 pass（fail 0）、`pnpm --filter @weknora/mobile typecheck` 0 错。以代码现状为准。
2. 调查称「agent-options.ts/resource-presentation.ts 种子无专属测试」——属实（`ls packages/domain/src/mobile/` 无对应 test 文件）；Task 1 补齐专属测试，且为撤权投影规则（resource-presentation.ts:4-7 冻结规则）建立回归防线。
3. 调查称「后端无聚合 Agent/知识/Connection 投影端点」——属实。本计划**不新增后端端点**：Resource Backend Port 的 WeKnora Adapter 组合既有三个 Viewer+ 读端点（/agents、/knowledge-bases、/apps/connections）。Marketplace 新模型的 Available Agent 投影属 #59（T29，Blocked by 本 Issue）；spec 2026-09-20-mobile-ai-office-design.md:195 已自认该缺口，本计划交付的是「当前 Tenant 可用资源」的移动端展示与失效，不冒充 Marketplace 投影。
4. Spec §6.1「附件准备状态」与 §6.2 `prepare(TaskResourceDraft)`、§6.3 Upload Port：分属 #36（T06 通用目标输入与耐久 Task 创建，Blocked by 本 Issue）；Local Favorites Port（Scoped Vault Adapter）：无对应验收标准与消费方，延后到出现收藏需求时实现（YAGNI）。本计划交付 browse/selection/subscribe——#33 验收标准仅涉展示、失效与 Interface 消费。`target-options.ts` 同为 #36+ 的执行目标选择策略，本计划不动它。
5. Spec §6.3「现有 agent-options.ts、resource-presentation.ts 和 target-options.ts 收入此 Module，不再让 Screen 拼接授权目标」——本计划实现其语义（Resource Shelf 成为 Screen 唯一可见入口；domain 纯策略文件留在 `packages/domain/src/mobile` 作为 Module 内部策略，符合 §11「domain 纯策略」与 §13「降为少量纯 policy tests」的落点），不做文件搬家（搬家会破坏 `@weknora/domain/mobile` 既有导出面，收益为零）。
6. 连接 wire 无显示名字段：`appConnectionView`（app_connector_connection.go:55-66）仅 `id/kind/state/owner_id/auth_version`；app_key 在 installations 视图且连接视图无 join 键。因此领域 `ConnectionResource` 不设 `name`（旧 seed 的 `name` 字段无消费者，已核实并替换），移动端以 kind+state+capability 呈现；待后端提供命名投影后再补显示名。
7. Agent `kind` 映射：WeKnora Agent 无 coding/analysis 类型（`config.agent_mode` 为 quick-answer/smart-reasoning，`agent_type` 为 rag-qa/wiki-qa/hybrid-rag-wiki/custom，internal/types/custom_agent.go:113-130）。映射为 `is_builtin → 'general'`、其余 → `'custom'`；`authorizedTargetId`（编码类 Agent 授权目标）语义留给 #52-#55（T22-T25 代码交付）。
8. 「supported」能力事实来源（显式记录，非猜测）：Viewer+ 的租户范围 `GET /api/v1/agents` 把该行返回给本成员——与 Web 会话下拉选择 Agent 使用同一服务端事实；403 → 类级 forbidden；无记录/未知 → unavailable。`disabled_own_agent_ids`（本租户「我停用」的自建 Agent，Web 下拉同规则过滤）从 Available 列表剔除。
9. 「capability 变化后失效」的实现映射：capability 降级（protocol mismatch/unknown schema）使 Runtime 停在 `upgrade-required` 面——该路径发生在 `authenticate` 内、lease 铸造之前，且进入前已由 `begin()/reserve()`（#32 咽喉）撤销旧 lease 并 `close(reason)` 旧 shelf（Task 4 测试 5 覆盖「降级后 handle undefined、零资源请求」）。运行中已授权后的 capability 变化（如服务端协议窗口收紧后重新协商）同样经 boot/signIn 的新 epoch 走同一咽喉。
10. 本计划撰写时 worktree 尚未合并 #32 实现（`grep activateTenant packages/mobile-core` 无命中）；#33 为 #32 的下游（Issue 声明 Blocked by #32），Task 3/4/5/6 对 #32 触碰文件的修改锚点全部按符号名（`scopedVault?`、`revoke(vaultReason)`、`activateTenant`、`tenantSwitch` 等）定位，执行顺序上必须在 #32 合并后进行。
11. `packages/mobile-core` 的 `package.json` 无 test script（现状核实）；mobile-core 测试沿用仓库既有的根目录 `npx tsx --test <files>` 方式（与 plan-t32、`mobile-runtime.test.ts` 一致），不新增 script。
12. 诚实声明（残余风险）：browse 对 403 的「及时失效」依赖下一次 browse 或 subscribe 事件（拉取式刷新）；服务端主动推送式失效（SSE 通知撤权）属 #35（T05 SSE 恢复）与 #41（T11 推送）的接线范围。本计划交付的时效性 = 每次 browse 从服务端事实重建 + 失效事件即时通知已订阅控制器（Task 5 测试覆盖）。

## Consumes / Produces 汇总

**Consumes（前序计划 #32 / T02 的 Produces，逐字）：**
- `MobileRuntime.activateTenant(tenantId: string): Promise<RuntimeSnapshot>`；`RuntimeSnapshot.identity.tenants?: Array<{ id: string; name?: string }>`
- `RuntimeRemote.switchTenant(input: { tenantId: string; refreshToken: string }): Promise<{ credential: StoredCredential; tenant?: Record<string, unknown> | null }>` 与 api-client 实现；`RuntimeRemote.me` 携带 `memberships?: unknown[]`
- `MobileRuntimePorts.scopedVault?: ScopedVault` 建立的「scope 变化单一咽喉 revoke(vaultReason)」接线模式与 `VaultRevokeReason` 值集（deployment-change/tenant-switch/sign-out/dispose）
- 包内 `RuntimeScopeLease`/`leaseScopeOf`/`leaseActive`/`LeaseScope`（`packages/mobile-core/src/runtime/scope-lease.ts`）
- apps/mobile `AuthorizedLandingScreen` 租户切换 UI 形态（props 含 `tenants`/`onActivateTenant`）
- 集成证据 `MobileRuntimeIntegrationEvidence.tenantSwitch` 与 `WEKNORA_MOBILE_TEST_SWITCH_TENANT_ID`、`disallowedDeploymentHost` 主机防线
- 已合并 T01：`createMobileRuntime`/`RuntimeSnapshot`/`ScopeLease`/`refreshedCredential` 单飞/`createMobileRuntimeRemote`/opt-in 集成证据骨架

**Produces（后续 #34-#41、#45、#59、#66 等消费）：**
- `packages/mobile-core/src/shell/`：`createResourceShelf(ports)`、`ResourceShelf.open({ lease }) → ResourceShelfHandle`（`browse(query?)`/`selection(input)`/`subscribe(listener)`/`close(reason)`）、`ResourcePage/ResourceQuery/ResourceClass/ResourceClassVerdict/SelectionVerdict/ShelfInvalidationEvent/ShelfCloseReason` 类型、`createInMemoryResourceRemote(script)` 场景 Adapter
- `MobileRuntime.resourceShelf(): ResourceShelfHandle | undefined`；`MobileRuntimePorts.resourceShelf?: { remoteFor(origin: string): ResourceRemote }`；包内 token seam `accessTokenFor(origin, { refresh })`
- `packages/api-client/src/mobile/resources.ts`：`createMobileResourceRemote({ origin, request })`（GET /api/v1/agents、/api/v1/knowledge-bases、/api/v1/apps/connections）
- domain：`ConnectionResource`（新形态）、`ConnectionLifecycleState`、`toConnectionResource`、`connectionCapability`
- apps/mobile：`ResourcesScreen`、`createResourceShelfController`、`/resources` 路由、landing「Open Resources」入口、`activeMobileRuntime()`
- 集成证据字段 `resourceShelf`/`resourceCounts` 与 `collectResourceShelfEvidence`

## 验收标准 → 证据映射

| 验收标准 | 证据 |
|---|---|
| 1. 撤权或 capability 变化后资源投影及时失效 | Task 3（403 → forbidden + `authorization-revoked` 事件 + 敏感字段不回填；5xx ≠ 撤权；lease 撤销/迟到结果 fail closed）+ Task 4（tenant-switch/sign-out/deployment-change 同步 close + 事件原因；capability 降级不开 shelf、零资源请求）+ Task 5（控制器失效重取、迟到投影丢弃、forbidden 类行消失横幅显因）+ Task 1（撤权投影/选择不可用的领域规则回归防线） |
| 2. Screen 只消费 Resource Shelf Interface，不直接拼 wire DTO | Task 5（ResourcesScreen/路由/控制器仅 import `@weknora/mobile-core` 与 `@weknora/domain/mobile`；静态测试断言源码无 `@weknora/api-client`/`@weknora/contracts` import；wire→语义行只在 api-client Adapter 内发生——Task 2） |
| 3. 端到端行为通过最高稳定 Interface 验证；底层单测/静态检查/mock 不冒充真实集成证据 | Task 6 opt-in 真实 HTTP 测试（生产 JSON transport + 具体 Resource Remote Adapter + Runtime 编排 + `runtime.resourceShelf().browse()`，证据含 browse 结果与计数），blocked-env 见「Global Constraints」末段声明；本地替代证据 = Task 3/4 Interface 级测试（真实 403/401 语义 + 真 Runtime/lease 编排）+ Task 2 wire 契约测试（handler 源码核实的服务器响应形状） |
