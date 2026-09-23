# T36：多 Deployment 切换与兼容性降级（Issue #66）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 一台设备可登记多个官方云/自托管 Deployment 并在实例间原子切换（不携带旧凭据、不接纳迟到响应、缓存/Tenant 随 scope 撤销隔离），安全关键 capability 缺失（部署落后于客户端代际）时进入「说明 + 有限只读」降级面而非乐观调用，并通过 MobileRuntime Interface 测试 + opt-in 真实 HTTP 多实例切换证据端到端验证。

**Architecture:** 登记与切换的顺序归 Mobile Runtime（module-seams §4.1）：新增可选端口 `DeploymentRegistry`（presentation-safe 的 origin/label 清单，永不存凭据），Runtime 在每次鉴权成功时 upsert 当前实例，并新增 `listDeployments`/`switchDeployment`/`forgetDeployment` 三个方法——`switchDeployment` 复用既有 `begin()`（撤销旧 lease/vault/shelf，epoch 递增丢弃迟到响应）+ 按 origin 读各自 CredentialStore 凭据 + `authenticate()` 重验身份与 capability 的机制，未登记或无凭据一律 fail closed。降级面：`clientGate` 的 `'server_upgrade_required'`（客户端代际高于部署窗口上界 = 部署缺失本代际所需安全关键 capability）从「说明面」改判为新的 `RuntimeSurface = 'read-only'`——身份已验、lease 已铸、Resource Shelf（只读浏览）可用，而 `authorizedRequest`/`activateTenant` 维持 authorized-only（fail closed）。apps/mobile 侧新增 OS 安全存储的 registry Adapter、登录屏实例列表、Home 屏实例切换钮与 `ReadOnlyScreen`（复用 Resource Shelf 只读投影），全部经 composition 根接线，Screen 不碰 wire 层。

**Tech Stack:** TypeScript（`packages/mobile-core`、`packages/api-client`、`apps/mobile` Expo RN）、node:test + tsx（测试运行器，与 `mobile-runtime.test.ts` 一致）、expo-secure-store（原生安全存储 Adapter）。所有测试命令在 worktree 根（`.worktrees/issue30-sweep`）执行；前置：已执行过 `pnpm install`（本计划作者已实跑 `pnpm exec tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts`，33/33 pass——任务书中「24/24」为旧基数，以本次实跑为准）。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-66.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（User Story 30「connect the app to an official or self-hosted WeKnora Deployment」、Implementation Decisions、Testing Decisions）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§4 Mobile Runtime Module：所有权/Interface/隐藏实现/Adapter seam）
- ADR：`docs/adr/0012-mobile-business-logic-lives-behind-deep-modules.md`、`docs/adr/0005-weknora-native-mobile-client.md`、`docs/adr/0007-registered-devices-and-encrypted-cache.md`（设备/缓存按 Deployment 隔离的决策背景）
- 领域术语：`CONTEXT.md`（「部署实例（Deployment）」：「移动端可登记多个实例，但同一时刻只有一个活动实例。」）
- Parent：Issue #30；Blocked by：#32（T02——已合并，见下方「差异记录」）

**本计划 Consumes（前序批次已合并产出的精确签名，均在当前 HEAD）：**
- `MobileRuntime.activateTenant(tenantId: string): Promise<RuntimeSnapshot>`、`RuntimeSnapshot.identity.tenants?: Array<{ id: string; name?: string }>`（#32；`packages/mobile-core/src/runtime/types.ts:43`、`mobile-runtime.ts:367`）
- Scoped Vault：`MobileRuntimePorts.scopedVault?: ScopedVault`，scope key `weknora.vault.v1.<origin>.<userId>.<tenantId>`，Runtime 在每次 scope 变化（`deployment-change`/`tenant-switch`/`sign-out`/`dispose`）撤销旧 lease（#32；`mobile-runtime.ts:113-130`）
- `MobileRuntime.authorizedRequest(input: RuntimeAuthorizedRequest): Promise<unknown>` 与 `MobileRuntimePorts.authorizedTransport?`（#34；`mobile-runtime.ts:341-363`、`ports.ts:82`）
- Resource Shelf：`MobileRuntime.resourceShelf(): ResourceShelfHandle | undefined`、`MobileRuntimePorts.resourceShelf?: { remoteFor(origin: string): ResourceRemote }`、`createResourceShelfController`/`ResourcesScreen`（#33；`mobile-runtime.ts:220-222`、`apps/mobile/src/resources-view.ts:18`）
- `clientGate(clientVersion, serverCapabilities): ClientGateVerdict`（mode 含 `'server_upgrade_required'`，`packages/domain/src/mobile/compatibility.ts:85-96`）；`CLIENT_PROTOCOL_VERSION = 3`（`compatibility.ts:22`）
- opt-in 集成证据模式：`WEKNORA_MOBILE_TEST_*` 环境变量、`disallowedDeploymentHost` 主机防线、`MobileRuntimeIntegrationEvidence`（#32/#33；`apps/mobile/src/runtime-integration-smoke.ts:109-156`）

**本计划 Produces（供后续计划消费的精确签名）：**
- `MobileRuntimePorts.deploymentRegistry?: DeploymentRegistry`，其中 `interface DeploymentRegistry { list(): Promise<Deployment[]>; upsert(deployment: Deployment): Promise<void>; remove(origin: string): Promise<void> }`（`Deployment = { origin: string; label: string }`，复用既有类型）
- `MobileRuntime.listDeployments(): Promise<Deployment[]>`、`MobileRuntime.switchDeployment(origin: string): Promise<RuntimeSnapshot>`、`MobileRuntime.forgetDeployment(origin: string): Promise<void>`
- `createInMemoryDeploymentRegistry(initial?: Deployment[]): DeploymentRegistry`（mobile-core 公共导出，场景/证据 Adapter）
- `RuntimeSurface` 新值 `'read-only'`：该面上快照含 `reason: 'protocol-mismatch'`、`identity`、`scopeLease()`、`resourceShelf()` 可用；`authorizedRequest` 拒 `RUNTIME_UNAUTHORIZED`、`activateTenant` 不发请求（fail closed）
- apps/mobile：`createSecureDeploymentRegistry(store: SecureStorePort): DeploymentRegistry` / `createNativeSecureDeploymentRegistry()`、`ReadOnlyScreen`、`RuntimeSurfaceProps.{ deployments?, onSwitchDeployment? }`、`DeploymentLoginScreenProps.{ deployments?, onSwitchDeployment? }`、`HomeScreenProps.{ otherDeployments?, onSwitchDeployment? }`
- 集成证据：`MobileRuntimeIntegrationEvidence.{ deploymentSwitch: 'skipped' | 'switched' | 'switch-failed', registeredDeployments: number }`、`MobileRuntimeIntegrationConfig` enabled 形态新增 `alt?: { deploymentOrigin: string; email: string; password: string }`、环境变量 `WEKNORA_MOBILE_TEST_ALT_{DEPLOYMENT_URL,EMAIL,PASSWORD}`（三变量同进同出）

## Global Constraints

以下为批准 Spec / ADR / 术语表的项目级约束，逐字引用，所有任务隐含遵守：

- 「Mobile Runtime owns Deployment, identity, Active Tenant, compatibility, device registration and Scope Lease.」（mobile-ai-office-design.md · Implementation Decisions）
- Mobile Runtime 唯一拥有：「Active Deployment、用户身份和 Active Tenant；capability handshake 与 protocol gate；scope generation 和 Scope Lease；登录、退出、Deployment/Tenant 切换的顺序；注册设备与 App 前后台生命周期；其他 Module 的启动、失效和关闭。」（mobile-module-seams.md §4.1）
- 「Official cloud and self-hosted Deployments use capability negotiation. Missing security-critical capabilities produce an explanation or limited read-only mode, not optimistic calls.」（mobile-ai-office-design.md · Implementation Decisions）
- 「Cache identity includes Deployment, user, Tenant, Task and Run where applicable. Scope changes invalidate subscriptions and reject late responses.」（同上）
- 「Registered devices are separate per Deployment. Device identity assists push and key wrapping but never replaces account or Tenant authorization.」（同上）
- 「Mobile core does not depend on React Native, DOM or concrete transport. Remote and native details are injected as Adapters.」（同上）
- 「The App Shell is a composition root and presentation Adapter. Screens do not call wire clients directly or maintain request IDs, cursors, revisions or scope generations.」（同上）
- 「Interface 不暴露 token、query key、generation number 或 SecureStore key。Scope Lease 是不透明、可撤销的能力对象，子 Module 每次异步提交前检查其有效性。」（mobile-module-seams.md §4.2）
- 「Tests target observable behavior at the highest stable Interface. Screen tests verify rendering and navigation; they do not duplicate Module internals.」（mobile-ai-office-design.md · Testing Decisions）
- 「Mobile Runtime Interface tests cover login restoration, capability negotiation, Active Tenant switching, Scope Lease revocation, device revocation and late-response rejection.」（同上）
- 「Remote-owned dependencies use in-memory scenario Adapters for Module tests and real HTTP/SSE contract tests for production Adapters.」（同上）
- 「移动端可登记多个实例，但同一时刻只有一个活动实例。」（CONTEXT.md ·「部署实例（Deployment）」）
- 安全约束（会话注入 + 请求 URL）：配置凭据只从环境变量或密钥服务读取，源码、示例和测试都不得写入可用的凭据字面量；真实 HTTP 证据的部署 URL 仅允许公网 HTTPS 主机（拒绝 localhost、环回、私网、链路本地与保留地址，复用 `disallowedDeploymentHost` 防线），新增的 ALT 实例变量同样过该防线。
- 工作流约束：严格 RED→GREEN→REFACTOR（每个任务先写失败测试、实跑确认失败、最小实现、通过、提交）；实现与已批准 Spec 冲突时升级处理，不静默重设计。

**Issue #66 验收标准原文（docs/plans/issue30-sweep/issues/issue-66.md）：**

1. 「切换实例不携带旧凭据或 late response。」
2. 「安全关键 capability 缺失时只进入说明或有限只读。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

验收标准 3 的本地可验证性说明（blocked-env 声明）：真实端到端（生产 JSON transport + 具体 Remote Adapter + Mobile Runtime 多实例登记/切换编排 + 真后端）沿用 T01/T02 已合并的 opt-in 真实 HTTP 测试模式，需要「两个真实 WeKnora Deployment（公网 HTTPS origin）+ 各一个测试账号」（`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD` + 新增 `WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL/EMAIL/PASSWORD`）。本地无此环境时 Task 6 的真实 HTTP 用例以 `t.skip` 跳过（**不得伪造通过**），本地替代证据为：MobileRuntime Interface 级测试（真实编排 + in-memory scenario Adapter，Task 1/2）+ apps/mobile 组合与屏测试（Task 4/5）+ 集成 config 校验测试（含主机防线与部分变量判 invalid，Task 6）。验收标准 2 的「真部署广播落后协议窗口」本地同样不可得（需要自托管管理员设置 `WEKNORA_WORKBENCH_PROTOCOL_MAXIMUM` 低于客户端代际）；具备自托管环境的运营者可如此制造真实降级做人工核验，自动化层面以 Interface 测试（真实 `clientGate` 判定 + 场景 Remote）为替代证据。真机 Release 证据属 #69/#70，不在本 Issue。

## Review Focus

Spec 隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **切换目标实例的服务端会话已被吊销**（存的凭据过期且 refresh 被拒）：切换必须停在 fail-closed 面，不得回退复活旧实例会话。——Task 1 测试「a switch onto a revoked server session fails closed without falling back to the prior instance」。
2. **被取代实例的迟到身份响应**：切换/登出后在途的 `me`/能力响应不得覆盖新实例快照（不携带 late response）。——Task 1 测试「a late switch response superseded by sign-out is ignored」。
3. **未登记或畸形 origin 的切换调用**（手输 `http://`、全空白、未登记 origin）：不得发出任何远程请求、不得扰动当前面。——Task 1 测试「switchDeployment to an unregistered or malformed origin keeps the surface without any remote request」。
4. **只读降级面上的越权通道**：`authorizedRequest`、`activateTenant`、重复 OIDC 回调在 `'read-only'` 面一律 fail closed，不因降级放松。——Task 2 测试「the read-only surface fails closed on the authorized channel and tenant switches」与「a duplicate OIDC callback leaves the read-only scope untouched」。
5. **集成证据的输入边界**：ALT 三变量部分提供、ALT 与主实例同源、ALT 指向内网/环回主机 → 判 `invalid` 而非 `skip`；证据 JSON 不得含凭据字段。——Task 6 配置校验测试 + 既有 emit 无凭据正则断言（`assert.doesNotMatch(..., /password|token|email/i)`）。

## 任务结构与文件地图

| # | 任务 | 主要交付 | 对应 AC |
|---|---|---|---|
| 1 | mobile-core：DeploymentRegistry 端口 + Runtime 登记/列举/切换/移除 | `DeploymentRegistry` 端口、`listDeployments`/`switchDeployment`/`forgetDeployment`、in-memory Adapter | AC1 |
| 2 | mobile-core：`'read-only'` 有限只读降级面 | `RuntimeSurface` 新值、gate 判定分流、shelf 只读可用、授权通道 fail closed | AC2 |
| 3 | apps/mobile：OS 安全存储 registry Adapter | `deployment-registry.ts`（SecureStorePort 之上） | AC1 |
| 4 | apps/mobile：登录屏实例列表 + Home 实例切换 + composition 接线 | `DeploymentLoginScreen`/`HomeScreen`/`composition.ts` | AC1 |
| 5 | apps/mobile：ReadOnlyScreen 只读降级面 | `ReadOnlyScreen.tsx`、composition `'read-only'` 分支 | AC2 |
| 6 | 端到端集成证据 | ALT 实例环境变量、`deploymentSwitch`/`registeredDeployments` 证据、opt-in 真实 HTTP 测试 | AC3 |

修改文件集中在 mobile-core 的 runtime 四文件（types/ports/mobile-runtime/index + in-memory-adapters）与 apps/mobile 的 composition/两屏/冒烟测试/集成 smoke；新建测试文件 `runtime-deployments.test.ts`、`runtime-readonly.test.ts`、`deployment-registry.test.ts` 为本计划独有。与同批次并行计划的共享文件（`apps/mobile/src/composition.ts`、`apps/mobile/src/app-smoke.test.tsx`、`apps/mobile/src/runtime-integration-smoke.ts`、`packages/mobile-core/src/runtime/runtime-shelf.test.ts`）改动均为追加式且位置明确（见各任务 Files 行号），合并时按任务顺序解决。

## 差异记录（调查简报 vs 代码现状，以代码现状为准）

1. 简报称「前置 #32 未实现」——现状：#32/#33/#34 均已合并（HEAD `028f72b11` 及此前），`activateTenant`/`switchTenant`/Scoped Vault/Resource Shelf/Task Office 全部在位（亲眼核实 `packages/mobile-core/src/runtime/mobile-runtime.ts:367`、`ports.ts:32`、`index.ts:5-21`）。本计划直接消费其产出，无需等待。
2. 简报缺口「设备注册隔离未实现」：设备注册 API 归 #41（T11，blocked by #32/#34/#35），不属本 Issue 交付；本计划交付实例级隔离的承载结构（per-origin registry/credential/vault scope），#41 的设备注册将天然按 Deployment scope 落地。离线缓存加密归 #40（vault scope key 已含 `<origin>`，`#32` 交付）。
3. 简报缺口「'有限只读'降级面不存在」属实：`RuntimeSurface` 仅三态（`types.ts:21`，`UpgradeRequiredScreen.tsx:11-19` 只说明 + Sign out）。Task 2/5 补齐。
4. 既有测试 `runtime-shelf.test.ts:162`「a capability downgrade never opens a shelf」（clientVersion 99）断言落后部署不开 shelf——本计划 Task 2 将其行为**有意变更**为「部署落后于客户端代际 → 只读面 + shelf 可浏览；客户端落后于部署 → 说明面且不开 shelf」，并同步改写该测试（记录在此，评审时关注）。

---

### Task 1: mobile-core：DeploymentRegistry 端口与 Runtime 实例切换

**Files:**
- Create: `packages/mobile-core/src/runtime/runtime-deployments.test.ts`
- Modify: `packages/mobile-core/src/runtime/ports.ts`（`DeploymentStore` 接口后新增 `DeploymentRegistry`；`MobileRuntimePorts` 增可选端口，约 `ports.ts:21` 与 `ports.ts:66-83`）
- Modify: `packages/mobile-core/src/runtime/types.ts`（`MobileRuntime` 接口增三方法，`types.ts:47-63`）
- Modify: `packages/mobile-core/src/runtime/mobile-runtime.ts`（`authenticate` 的 `mutateDeployment` 块增 upsert，`mobile-runtime.ts:215`；返回对象在 `activateTenant` 后增三方法，`mobile-runtime.ts:387` 之前）
- Modify: `packages/mobile-core/src/runtime/in-memory-adapters.ts`（新增 `createInMemoryDeploymentRegistry`）
- Modify: `packages/mobile-core/src/index.ts`（导出新符号）

**Interfaces:**
- Consumes: `createMobileRuntime(ports: MobileRuntimePorts): MobileRuntime`；`normalizeDeployment`/`begin`/`authenticate`/`current`/`mutateDeployment`/`vaultTail`（`mobile-runtime.ts` 包内既有机制）；`Deployment = { origin: string; label: string }`（`types.ts:11`）。
- Produces: `interface DeploymentRegistry { list(): Promise<Deployment[]>; upsert(deployment: Deployment): Promise<void>; remove(origin: string): Promise<void> }`；`MobileRuntimePorts.deploymentRegistry?: DeploymentRegistry`；`MobileRuntime.listDeployments(): Promise<Deployment[]>`、`switchDeployment(origin: string): Promise<RuntimeSnapshot>`、`forgetDeployment(origin: string): Promise<void>`；`createInMemoryDeploymentRegistry(initial?: Deployment[]): DeploymentRegistry`。

- [ ] **Step 1: 写失败测试**

创建 `packages/mobile-core/src/runtime/runtime-deployments.test.ts`，内容完整如下：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createMobileRuntime } from './mobile-runtime.ts';
import { createInMemoryDeploymentRegistry } from './in-memory-adapters.ts';
import type { CredentialStore, RuntimeRemote, StoredCredential } from './ports.ts';
import type { ResourceRemote } from '../shelf/ports.ts';
import type { Deployment } from './types.ts';

// 夹具类型用 Deployment（label 必填）：既可作 signIn 的 DeploymentInput，又可直接喂 registry。
const FIRST: Deployment = { origin: 'https://weknora.example.test', label: 'WeKnora' };
const SECOND: Deployment = { origin: 'https://other.example.test', label: 'Other' };
const CAPABILITIES = { protocol_minimum: 2, protocol_maximum: 3 };

function fakeStore(initial: Record<string, StoredCredential | undefined> = {}): CredentialStore {
  const values = new Map(Object.entries(initial));
  return {
    async read(deployment) { return values.get(deployment); },
    async write(deployment, credential) { values.set(deployment, { ...credential }); },
    async clear(deployment) { values.delete(deployment); },
  };
}

function remote(overrides: Partial<RuntimeRemote> = {}): RuntimeRemote {
  return {
    passwordLogin: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    me: async () => ({ user: { id: 'user-1' }, tenant: { id: 'tenant-1' } }),
    deploymentCapabilities: async () => CAPABILITIES,
    oidcUrl: async () => ({ authorizationUrl: 'https://idp.example.test/authorize', state: 'state-1' }),
    oidcExchange: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    oidcNativeExchange: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    refresh: async () => ({ access_token: 'access-1', refresh_token: 'refresh-1' }),
    switchTenant: async (input) => ({ credential: { token: `tenant-${input.tenantId}-access`, refreshToken: `refresh-${input.tenantId}` }, tenant: { id: input.tenantId } }),
    ...overrides,
  };
}

/** 按 token 区分实例事实的内存资源远端（跨实例隔离断言用）。 */
function tenantResourceRemote(): ResourceRemote & { calls: Array<{ kind: string; token: string }> } {
  const calls: Array<{ kind: string; token: string }> = [];
  return {
    calls,
    async availableAgents(token) {
      calls.push({ kind: 'agents', token });
      return { rows: [{ id: `agent-${token}`, name: `Agent ${token}`, summary: '', kind: 'general', capability: { state: 'supported', reason: '' } }], disabledOwnAgentIds: new Set<string>() };
    },
    async knowledgeBases(token) { calls.push({ kind: 'knowledgeBases', token }); return []; },
    async connections(token) { calls.push({ kind: 'connections', token }); return []; },
  };
}

test('an authorized sign-in registers the deployment in the registry without duplicating it', async () => {
  const deployments = createInMemoryDeploymentRegistry();
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(), remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION, deploymentRegistry: deployments,
  });

  await runtime.signIn({ deployment: FIRST, email: 'member@example.test', password: 'password' });
  await runtime.signIn({ deployment: SECOND, email: 'member@example.test', password: 'password' });
  await runtime.signIn({ deployment: FIRST, email: 'member@example.test', password: 'password' });

  assert.deepEqual(await runtime.listDeployments(), [
    { origin: FIRST.origin, label: FIRST.label },
    { origin: SECOND.origin, label: SECOND.label },
  ], 'the most recent instance comes first and repeats never duplicate');
});

test('a failed sign-in does not register the deployment', async () => {
  const deployments = createInMemoryDeploymentRegistry();
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => remote({ passwordLogin: async () => { throw new Error('bad credentials'); } }),
    clientVersion: CLIENT_PROTOCOL_VERSION, deploymentRegistry: deployments,
  });

  const snapshot = await runtime.signIn({ deployment: FIRST, email: 'member@example.test', password: 'wrong' });

  assert.equal(snapshot.surface, 'upgrade-required');
  assert.deepEqual(await runtime.listDeployments(), [], 'only a verified authorized scope may register an instance');
});

test('switchDeployment restores the registered instance from its own stored credential without a new login', async () => {
  const seen: Array<{ deployment: string; kind: string; token: string }> = [];
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: (origin) => remote({
      passwordLogin: async () => { seen.push({ deployment: origin, kind: 'passwordLogin', token: '' }); return { token: `${origin}-access`, refreshToken: `${origin}-refresh` }; },
      me: async (token) => { seen.push({ deployment: origin, kind: 'me', token }); return { user: { id: origin }, tenant: { id: origin } }; },
    }),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    deploymentRegistry: createInMemoryDeploymentRegistry(),
  });
  await runtime.signIn({ deployment: FIRST, email: 'member@example.test', password: 'password' });
  const firstLease = runtime.scopeLease();
  assert.ok(firstLease);
  await runtime.signIn({ deployment: SECOND, email: 'member@example.test', password: 'password' });

  const snapshot = await runtime.switchDeployment(FIRST.origin);

  assert.equal(snapshot.surface, 'authorized');
  assert.equal(snapshot.deployment?.origin, FIRST.origin);
  assert.equal(snapshot.identity?.userId, FIRST.origin);
  assert.notEqual(runtime.scopeLease(), firstLease, 'switching instances must mint a fresh lease');
  assert.equal(seen.filter((entry) => entry.kind === 'passwordLogin').length, 2, 'switching reuses the stored credential, never a second password login');
  assert.deepEqual(seen.filter((entry) => entry.kind === 'me').map((entry) => entry.token), [
    `${FIRST.origin}-access`, `${SECOND.origin}-access`, `${FIRST.origin}-access`,
  ], 'each instance is verified with its own credential only');
});

test('switching deployments closes the prior shelf and serves only the new instance scope', async () => {
  const store = fakeStore({ [SECOND.origin]: { token: 'other-access', refreshToken: 'other-refresh' } });
  const resource = tenantResourceRemote();
  const runtime = createMobileRuntime({
    credentialStore: store,
    remoteFor: (origin) => remote({ me: async () => ({ user: { id: origin }, tenant: { id: origin } }) }),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    deploymentRegistry: createInMemoryDeploymentRegistry([FIRST, SECOND]),
    resourceShelf: { remoteFor: () => resource },
  });
  await runtime.signIn({ deployment: FIRST, email: 'member@example.test', password: 'password' });
  const firstShelf = runtime.resourceShelf()!;
  const events: unknown[] = [];
  firstShelf.subscribe((event) => events.push(event));

  const snapshot = await runtime.switchDeployment(SECOND.origin);

  assert.equal(snapshot.surface, 'authorized');
  assert.equal(snapshot.deployment?.origin, SECOND.origin);
  await assert.rejects(firstShelf.browse(), /SHELF_SCOPE_CLOSED/);
  assert.deepEqual(events, [{ type: 'scope-closed', reason: 'deployment-change' }]);
  const secondShelf = runtime.resourceShelf();
  assert.ok(secondShelf);
  assert.notEqual(secondShelf, firstShelf);
  const page = await secondShelf.browse();
  assert.equal(page.tenantId, SECOND.origin);
  assert.deepEqual(page.agents.map((agent) => agent.id), ['agent-other-access'], 'the page is rebuilt from the new instance facts only');
});

test('a late switch response superseded by sign-out is ignored', async () => {
  const identity = deferred<{ user: { id: string }; tenant: { id: string } }>();
  let meCalls = 0;
  const runtime = createMobileRuntime({
    credentialStore: fakeStore({ [SECOND.origin]: { token: 'other-access', refreshToken: 'other-refresh' } }),
    remoteFor: () => remote({
      me: async () => { meCalls += 1; return meCalls === 1 ? { user: { id: 'user-1' }, tenant: { id: 'tenant-1' } } : identity.promise; },
    }),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    deploymentRegistry: createInMemoryDeploymentRegistry([FIRST, SECOND]),
  });
  await runtime.signIn({ deployment: FIRST, email: 'member@example.test', password: 'password' });

  const switching = runtime.switchDeployment(SECOND.origin);
  // 让渡一个宏任务（而非单个微任务）：switchDeployment 内 registry.list → credential read →
  // authenticate → me 须全部真正发出、第二个 me 挂起在 identity.promise 上，之后才被登出取代——
  // 这样覆盖的是 authenticate 内部（mobile-runtime.ts:207 一带）的迟到检查，而非读凭据前的 epoch 检查。
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.equal(meCalls, 2, 'the switch verification must already be in flight before sign-out supersedes it');
  await runtime.signOut();
  identity.resolve({ user: { id: 'user-1' }, tenant: { id: 'tenant-1' } });
  await switching;

  assert.deepEqual(runtime.snapshot(), { surface: 'deployment-login', reason: 'authentication-required' });
  assert.equal(runtime.scopeLease(), undefined);
});

test('switchDeployment to an unregistered or malformed origin keeps the surface without any remote request', async () => {
  const calls: string[] = [];
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: (origin) => remote({ me: async () => { calls.push(`me:${origin}`); return { user: { id: origin }, tenant: { id: origin } }; } }),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    deploymentRegistry: createInMemoryDeploymentRegistry(),
  });
  await runtime.signIn({ deployment: FIRST, email: 'member@example.test', password: 'password' });
  const authorized = runtime.snapshot();
  assert.equal(authorized.surface, 'authorized');

  assert.equal(await runtime.switchDeployment(SECOND.origin), authorized, 'an unregistered origin must not disturb the authorized scope');
  assert.equal(await runtime.switchDeployment('http://insecure.example.test'), authorized, 'a non-HTTPS origin fails closed without throwing');
  assert.equal(await runtime.switchDeployment('   '), authorized);
  assert.deepEqual(calls, [`me:${FIRST.origin}`], 'no remote call may leave for an unswitchable origin');
});

test('a switch onto a revoked server session fails closed without falling back to the prior instance', async () => {
  const store = fakeStore({ [SECOND.origin]: { token: 'expired-access', refreshToken: 'expired-refresh' } });
  const runtime = createMobileRuntime({
    credentialStore: store,
    remoteFor: (origin) => remote({
      me: async () => { if (origin === SECOND.origin) throw new Error('expired'); return { user: { id: origin }, tenant: { id: origin } }; },
      refresh: async () => { throw new Error('refresh rejected'); },
    }),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    deploymentRegistry: createInMemoryDeploymentRegistry([FIRST, SECOND]),
  });
  await runtime.signIn({ deployment: FIRST, email: 'member@example.test', password: 'password' });
  assert.ok(runtime.scopeLease());

  const snapshot = await runtime.switchDeployment(SECOND.origin);

  assert.deepEqual(snapshot, { surface: 'upgrade-required', deployment: { origin: SECOND.origin, label: SECOND.label }, reason: 'authentication-required' });
  assert.equal(runtime.scopeLease(), undefined, 'the prior instance lease must not survive the failed switch');
  assert.deepEqual(await store.read(FIRST.origin), { token: 'access-1', refreshToken: 'refresh-1' }, 'the prior instance credential stays untouched');
});

test('forgetting a non-active deployment removes only its registration and credential', async () => {
  const store = fakeStore({ [SECOND.origin]: { token: 'other-access', refreshToken: 'other-refresh' } });
  const runtime = createMobileRuntime({
    credentialStore: store, remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION, deploymentRegistry: createInMemoryDeploymentRegistry([FIRST, SECOND]),
  });
  await runtime.signIn({ deployment: FIRST, email: 'member@example.test', password: 'password' });

  await runtime.forgetDeployment(SECOND.origin);

  assert.deepEqual(await runtime.listDeployments(), [{ origin: FIRST.origin, label: FIRST.label }]);
  assert.equal(await store.read(SECOND.origin), undefined);
  assert.deepEqual(await store.read(FIRST.origin), { token: 'access-1', refreshToken: 'refresh-1' });
  assert.equal(runtime.snapshot().surface, 'authorized');
});

test('forgetting the active deployment signs out and clears its registration', async () => {
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(), remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION, deploymentRegistry: createInMemoryDeploymentRegistry(),
  });
  await runtime.signIn({ deployment: FIRST, email: 'member@example.test', password: 'password' });

  await runtime.forgetDeployment(FIRST.origin);

  assert.deepEqual(await runtime.listDeployments(), []);
  assert.deepEqual(runtime.snapshot(), { surface: 'deployment-login', reason: 'authentication-required' });
  assert.equal(runtime.scopeLease(), undefined);
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/runtime/runtime-deployments.test.ts`
Expected: FAIL——`createInMemoryDeploymentRegistry` 未导出（`TypeError: ... is not a function`）或 `runtime.switchDeployment is not a function`，全部 9 个用例失败。

- [ ] **Step 3: 最小实现**

1. `packages/mobile-core/src/runtime/ports.ts`——文件头 import 区加 `import type { Deployment } from './types.ts';`；在 `DeploymentStore` 接口（`ports.ts:17-21`）之后新增：

```ts
/** 登记实例清单：只保存 presentation-safe 的 origin/label，永不保存凭据。 */
export interface DeploymentRegistry {
  list(): Promise<Deployment[]>;
  upsert(deployment: Deployment): Promise<void>;
  remove(origin: string): Promise<void>;
}
```

`MobileRuntimePorts`（`ports.ts:67-83`）中 `deploymentStore?: DeploymentStore;` 之后加一行：

```ts
  /** 已登记 Deployment 清单；Runtime 在每次鉴权成功时 upsert 当前实例。 */
  deploymentRegistry?: DeploymentRegistry;
```

2. `packages/mobile-core/src/runtime/types.ts`——`MobileRuntime` 接口在 `activateTenant(...)` 声明（`types.ts:60`）之后、`signOut()` 之前插入：

```ts
  /** 登记实例清单（presentation-safe）；未提供 registry 端口时返回空数组。 */
  listDeployments(): Promise<Deployment[]>;
  /** 原子切换 Active Deployment：撤销旧 scope，恢复目标实例已存凭据并重新验证身份；未登记、畸形 origin 或无凭据时 fail closed。 */
  switchDeployment(origin: string): Promise<RuntimeSnapshot>;
  /** 移除一个登记实例并清除其凭据；移除活动实例等价于登出。 */
  forgetDeployment(origin: string): Promise<void>;
```

3. `packages/mobile-core/src/runtime/mobile-runtime.ts`：
   - `authenticate` 内（`mobile-runtime.ts:215`）把持久化块改为：

```ts
      await mutateDeployment(async () => {
        await ports.deploymentStore?.write(deployment);
        await ports.deploymentRegistry?.upsert(deployment);
      });
```

   - 返回对象中 `activateTenant` 方法（`mobile-runtime.ts:367-387`）之后、`dispose()` 之前插入：

```ts
    async listDeployments(): Promise<Deployment[]> {
      return ports.deploymentRegistry ? await ports.deploymentRegistry.list() : [];
    },
    async switchDeployment(origin: string): Promise<RuntimeSnapshot> {
      try {
        if (typeof origin !== 'string' || origin.trim() === '') return state;
        if (!ports.deploymentRegistry) return state;
        let target: Deployment | undefined;
        try { target = normalizeDeployment({ origin }); } catch { target = undefined; }
        if (!target) return state;
        const record = (await ports.deploymentRegistry.list()).find((entry) => entry.origin === target!.origin);
        if (!record) return state;
        const deployment = normalizeDeployment({ origin: record.origin, label: record.label });
        const requestEpoch = begin(deployment);
        try {
          const credential = await ports.credentialStore.read(deployment.origin);
          if (!current(requestEpoch, deployment)) return state;
          if (!credential) return publish({ surface: 'deployment-login', deployment, reason: 'authentication-required' });
          return await authenticate(requestEpoch, deployment, credential);
        } catch {
          return safe(requestEpoch, deployment, 'authentication-required');
        }
      } finally {
        await vaultTail;
      }
    },
    async forgetDeployment(origin: string): Promise<void> {
      let target: Deployment | undefined;
      try { if (typeof origin === 'string' && origin.trim() !== '') target = normalizeDeployment({ origin }); } catch { target = undefined; }
      if (!target) return;
      const deployment = target;
      try {
        if (activeDeployment?.origin === deployment.origin) {
          await signOut();
        } else {
          await mutateCredential(async () => { await ports.credentialStore.clear(deployment.origin); });
        }
        await mutateDeployment(async () => { await ports.deploymentRegistry?.remove(deployment.origin); });
      } finally {
        await vaultTail;
      }
    },
```

4. `packages/mobile-core/src/runtime/in-memory-adapters.ts`——全文件替换为：

```ts
import type { CredentialStore, DeploymentRegistry, StoredCredential } from './ports.ts';
import type { Deployment } from './types.ts';

export function createInMemoryCredentialStore(initial: Record<string, StoredCredential | undefined> = {}): CredentialStore {
  const values = new Map(Object.entries(initial));
  return {
    async read(deployment) { return values.get(deployment); },
    async write(deployment, credential) { values.set(deployment, { ...credential }); },
    async clear(deployment) { values.delete(deployment); },
  };
}

export function createInMemoryDeploymentRegistry(initial: Deployment[] = []): DeploymentRegistry {
  const records = initial.map((deployment) => ({ origin: deployment.origin, label: deployment.label }));
  return {
    async list() { return records.map((deployment) => ({ ...deployment })); },
    async upsert(deployment) {
      // 前移语义：最近登记的实例排最前——与 Task 3 createSecureDeploymentRegistry 的顺序契约一致
      // （同一 DeploymentRegistry 端口下两个 Adapter 的顺序语义必须统一）。
      const next = { origin: deployment.origin, label: deployment.label };
      const rest = records.filter((record) => record.origin !== next.origin);
      records.splice(0, records.length, next, ...rest);
    },
    async remove(origin) {
      const index = records.findIndex((record) => record.origin === origin);
      if (index !== -1) records.splice(index, 1);
    },
  };
}
```

5. `packages/mobile-core/src/index.ts`——`createInMemoryCredentialStore` 导出行改为：

```ts
export { createInMemoryCredentialStore, createInMemoryDeploymentRegistry } from './runtime/in-memory-adapters.ts';
```

`ports.ts` 类型导出行加入 `DeploymentRegistry`：

```ts
export type { AppLifecyclePort, CredentialStore, DeploymentRegistry, DeploymentStore, MobileRuntimePorts, OidcBrowserPort, PendingOidc, PendingOidcStore, RuntimeRemote, StoredCredential } from './runtime/ports.ts';
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test packages/mobile-core/src/runtime/runtime-deployments.test.ts packages/mobile-core/src/runtime/mobile-runtime.test.ts`
Expected: PASS（新文件 9/9；既有 `mobile-runtime.test.ts` 33/33 不回归）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/runtime/runtime-deployments.test.ts packages/mobile-core/src/runtime/ports.ts packages/mobile-core/src/runtime/types.ts packages/mobile-core/src/runtime/mobile-runtime.ts packages/mobile-core/src/runtime/in-memory-adapters.ts packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core): deployment registry with runtime-scoped instance switching"
```

---

### Task 2: mobile-core：`'read-only'` 有限只读降级面

**Files:**
- Create: `packages/mobile-core/src/runtime/runtime-readonly.test.ts`
- Modify: `packages/mobile-core/src/runtime/types.ts`（`RuntimeSurface` 联合类型，`types.ts:21`）
- Modify: `packages/mobile-core/src/runtime/mobile-runtime.ts`（gate 分流 `mobile-runtime.ts:213-214`、发布面 `mobile-runtime.ts:223`、`accessTokenFor` 面检查 `mobile-runtime.ts:185`、`completeOidc` 早退 `mobile-runtime.ts:311`）
- Modify: `packages/mobile-core/src/runtime/runtime-shelf.test.ts:162-176`（既有「a capability downgrade never opens a shelf」按差异记录 4 改写）

**Interfaces:**
- Consumes: `clientGate(clientVersion, capabilities)` 的 `mode: 'server_upgrade_required'`（`packages/domain/src/mobile/compatibility.ts:85`）；Task 1 的 registry upsert 块；`RuntimeSurface`/`RuntimeReason`。
- Produces: `RuntimeSurface = 'deployment-login' | 'upgrade-required' | 'authorized' | 'read-only'`；`'read-only'` 快照形如 `{ surface: 'read-only', deployment, identity, reason: 'protocol-mismatch' }`，`scopeLease()`/`resourceShelf()` 可用，`authorizedRequest` 拒 `RUNTIME_UNAUTHORIZED`、`activateTenant` 不发请求。

- [ ] **Step 1: 写失败测试**

创建 `packages/mobile-core/src/runtime/runtime-readonly.test.ts`，内容完整如下：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createMobileRuntime } from './mobile-runtime.ts';
import type { CredentialStore, PendingOidc, PendingOidcStore, RuntimeRemote, StoredCredential } from './ports.ts';
import type { ResourceRemote } from '../shelf/ports.ts';
import type { DeploymentInput } from './types.ts';

const DEPLOYMENT: DeploymentInput = { origin: 'https://weknora.example.test', label: 'Test Deployment' };
/** 部署落后：窗口上界低于客户端代际 → clientGate 判 'server_upgrade_required'。 */
const BEHIND_WINDOW = { protocol_minimum: 2, protocol_maximum: CLIENT_PROTOCOL_VERSION - 1 };
/** 客户端落后：窗口下界高于客户端代际 → clientGate 判 'upgrade_required'（说明面，行为不变）。 */
const AHEAD_WINDOW = { protocol_minimum: CLIENT_PROTOCOL_VERSION + 1, protocol_maximum: CLIENT_PROTOCOL_VERSION + 1 };

function fakeStore(initial: Record<string, StoredCredential | undefined> = {}): CredentialStore {
  const values = new Map(Object.entries(initial));
  return {
    async read(deployment) { return values.get(deployment); },
    async write(deployment, credential) { values.set(deployment, { ...credential }); },
    async clear(deployment) { values.delete(deployment); },
  };
}

function remote(overrides: Partial<RuntimeRemote> = {}): RuntimeRemote {
  return {
    passwordLogin: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    me: async () => ({ user: { id: 'user-1' }, tenant: { id: 'tenant-1' } }),
    deploymentCapabilities: async () => BEHIND_WINDOW,
    oidcUrl: async () => ({ authorizationUrl: 'https://idp.example.test/authorize', state: 'state-1' }),
    oidcExchange: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    oidcNativeExchange: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    refresh: async () => ({ access_token: 'access-1', refresh_token: 'refresh-1' }),
    switchTenant: async (input) => ({ credential: { token: `tenant-${input.tenantId}-access`, refreshToken: `refresh-${input.tenantId}` }, tenant: { id: input.tenantId } }),
    ...overrides,
  };
}

function tenantResourceRemote(): ResourceRemote & { calls: Array<{ kind: string; token: string }> } {
  const calls: Array<{ kind: string; token: string }> = [];
  return {
    calls,
    async availableAgents(token) {
      calls.push({ kind: 'agents', token });
      return { rows: [{ id: `agent-${token}`, name: `Agent ${token}`, summary: '', kind: 'general', capability: { state: 'supported', reason: '' } }], disabledOwnAgentIds: new Set<string>() };
    },
    async knowledgeBases(token) { calls.push({ kind: 'knowledgeBases', token }); return []; },
    async connections(token) { calls.push({ kind: 'connections', token }); return []; },
  };
}

function pendingStore(): PendingOidcStore {
  let value: PendingOidc | undefined;
  return {
    async savePending(input) { value = { ...input }; },
    async loadPending() { return value && { ...value }; },
    async consumePending() { const claimed = value; value = undefined; return claimed && { ...claimed }; },
    async clearPending() { value = undefined; },
  };
}

test('a deployment behind the client generation degrades to a read-only surface with identity, lease and shelf', async () => {
  const resource = tenantResourceRemote();
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(), remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    resourceShelf: { remoteFor: () => resource },
  });

  const snapshot = await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });

  assert.deepEqual(snapshot, {
    surface: 'read-only',
    deployment: { origin: DEPLOYMENT.origin, label: DEPLOYMENT.label },
    identity: { userId: 'user-1', activeTenantId: 'tenant-1', tenants: [{ id: 'tenant-1' }] },
    reason: 'protocol-mismatch',
  });
  assert.ok(runtime.scopeLease(), 'the read-only scope still mints a revocable lease');
  const page = await runtime.resourceShelf()!.browse();
  assert.equal(page.tenantId, 'tenant-1');
  assert.deepEqual(page.agents.map((agent) => agent.id), ['agent-access-1']);
});

test('the read-only surface fails closed on the authorized channel and tenant switches', async () => {
  let switches = 0;
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => remote({ switchTenant: async () => { switches += 1; return { credential: { token: 'x', refreshToken: 'y' }, tenant: { id: 't' } }; } }),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    authorizedTransport: () => async () => ({ success: true, data: {} }),
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const before = runtime.snapshot();
  assert.equal(before.surface, 'read-only');

  await assert.rejects(runtime.authorizedRequest({ method: 'GET', path: '/api/v1/workbench/overview' }), /RUNTIME_UNAUTHORIZED/);
  const after = await runtime.activateTenant('2');

  assert.equal(switches, 0, 'a degraded deployment must not receive tenant switch requests');
  assert.equal(after, before);
});

test('an app older than the deployment window keeps the explanation-only upgrade surface', async () => {
  const resource = tenantResourceRemote();
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => remote({ deploymentCapabilities: async () => AHEAD_WINDOW }),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    resourceShelf: { remoteFor: () => resource },
  });

  const snapshot = await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });

  assert.deepEqual(snapshot, { surface: 'upgrade-required', deployment: { origin: DEPLOYMENT.origin, label: DEPLOYMENT.label }, reason: 'protocol-mismatch' });
  assert.equal(runtime.scopeLease(), undefined);
  assert.equal(runtime.resourceShelf(), undefined);
  assert.equal(resource.calls.length, 0, 'no resource request may leave from the explanation-only surface');
});

test('boot restores a stored credential into the read-only surface and sign-out returns to login', async () => {
  const store = fakeStore({ [DEPLOYMENT.origin]: { token: 'stored-access', refreshToken: 'stored-refresh' } });
  const runtime = createMobileRuntime({
    credentialStore: store, remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
  });

  const snapshot = await runtime.boot(DEPLOYMENT);
  assert.equal(snapshot.surface, 'read-only');

  await runtime.signOut();

  assert.deepEqual(runtime.snapshot(), { surface: 'deployment-login', reason: 'authentication-required' });
  assert.equal(await store.read(DEPLOYMENT.origin), undefined);
});

test('a duplicate OIDC callback leaves the read-only scope untouched', async () => {
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(), remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION, pendingOidcStore: pendingStore(),
  });
  const snapshot = await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  assert.equal(snapshot.surface, 'read-only');

  const after = await runtime.completeOidc('weknora://oidc?code=code-1&state=state-1');

  assert.equal(after, snapshot, 'a read-only session with a live lease must ignore duplicate callbacks');
});
```

同时改写 `packages/mobile-core/src/runtime/runtime-shelf.test.ts:162-176` 的既有用例（差异记录 4 的行为变更）为：

```ts
test('a server-behind downgrade serves a read-only shelf and an app-behind downgrade opens none', async () => {
  const resource = tenantResourceRemote();
  const behindServer = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => baseRemote(),
    clientVersion: 99,
    resourceShelf: { remoteFor: () => resource },
  });

  const snapshot = await behindServer.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });

  assert.equal(snapshot.surface, 'read-only');
  const handle = behindServer.resourceShelf();
  assert.ok(handle, 'a read-only scope still exposes the browse-only shelf');
  assert.equal((await handle.browse()).tenantId, 'tenant-1');

  const untouched = tenantResourceRemote();
  const olderApp = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => baseRemote(),
    clientVersion: 1,
    resourceShelf: { remoteFor: () => untouched },
  });
  const stale = await olderApp.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  assert.equal(stale.surface, 'upgrade-required');
  assert.equal(olderApp.resourceShelf(), undefined);
  assert.equal(untouched.calls.length, 0, 'no resource request may leave without an authorized or read-only scope');
});
```

（该文件 `baseRemote`/`fakeStore`/`tenantResourceRemote`/`DEPLOYMENT` 均已在文件头定义，直接替换原测试即可。）

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/runtime/runtime-readonly.test.ts packages/mobile-core/src/runtime/runtime-shelf.test.ts`
Expected: FAIL——新文件 5 个用例全部失败（现状 surface 实际为 `'upgrade-required'`，`'read-only'` 分支与只读 shelf 不存在；「duplicate OIDC callback」用例同样因缺少 read-only 早退而失败）；`runtime-shelf.test.ts` 改写后的用例失败（clientVersion 99 现判 `'upgrade-required'`）。

- [ ] **Step 3: 最小实现**

1. `packages/mobile-core/src/runtime/types.ts:21`：

```ts
export type RuntimeSurface = 'deployment-login' | 'upgrade-required' | 'authorized' | 'read-only';
```

2. `packages/mobile-core/src/runtime/mobile-runtime.ts`：
   - `accessTokenFor`（`mobile-runtime.ts:185`）面检查放宽为：

```ts
    if (!deployment || deployment.origin !== origin || (state.surface !== 'authorized' && state.surface !== 'read-only') || !activeCredential) throw new Error('SHELF_SCOPE');
```

   - `authenticate` 中 gate 判定（`mobile-runtime.ts:213-214`）与发布（`mobile-runtime.ts:217-223`）改为（保持 Task 1 已加的 registry upsert 块不动）：

```ts
      const gate = clientGate(ports.clientVersion, capabilities);
      if (gate.mode !== 'full' && gate.mode !== 'server_upgrade_required') {
        return safe(requestEpoch, deployment, gate.mode === 'unknown_schema' ? 'unknown-capability' : 'protocol-mismatch');
      }
      const surface: RuntimeSurface = gate.mode === 'full' ? 'authorized' : 'read-only';
      await mutateDeployment(async () => {
        await ports.deploymentStore?.write(deployment);
        await ports.deploymentRegistry?.upsert(deployment);
      });
      if (!current(requestEpoch, deployment)) return state;
      revocableLease = new RuntimeScopeLease({ deploymentOrigin: deployment.origin, userId: authenticatedUserId, tenantId: activeTenantId });
      lease = revocableLease.asScopeLease();
      activeCredential = verifiedCredential;
      activeShelf = ports.resourceShelf
        ? createResourceShelf({ remote: ports.resourceShelf.remoteFor(deployment.origin), accessTokenFor }).open({ lease })
        : undefined;
      return publish({
        surface,
        deployment,
        identity: { userId: authenticatedUserId, activeTenantId, ...tenantOptions(me.memberships, activeTenantId) },
        ...(surface === 'read-only' ? { reason: 'protocol-mismatch' as const } : {}),
      });
```

（import 行 `types.ts` 的类型列表中加入 `RuntimeSurface`——当前已 import `Deployment, DeploymentInput, MobileRuntime, RuntimeAuthorizedRequest, RuntimeReason, RuntimeSnapshot, ScopeLease`，补 `RuntimeSurface`。）

   - `completeOidc` 的早退（`mobile-runtime.ts:311`）改为：

```ts
        if ((state.surface === 'authorized' || state.surface === 'read-only') && lease) return state;
```

   - `authorizedRequest`（`mobile-runtime.ts:343`）与 `activateTenant`（`mobile-runtime.ts:371`）的 `state.surface === 'authorized'` / `!== 'authorized'` 检查**保持不变**（只读面 fail closed）。

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test packages/mobile-core/src/runtime/runtime-readonly.test.ts packages/mobile-core/src/runtime/runtime-shelf.test.ts packages/mobile-core/src/runtime/mobile-runtime.test.ts packages/mobile-core/src/runtime/runtime-deployments.test.ts packages/mobile-core/src/runtime/runtime-vault.test.ts`
Expected: PASS（含既有 33 用例与 vault/shelf 全量不回归）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/runtime/runtime-readonly.test.ts packages/mobile-core/src/runtime/types.ts packages/mobile-core/src/runtime/mobile-runtime.ts packages/mobile-core/src/runtime/runtime-shelf.test.ts
git commit -m "feat(mobile-core): read-only degraded surface for capability-lagging deployments"
```

---

### Task 3: apps/mobile：OS 安全存储 Deployment Registry Adapter

**Files:**
- Create: `apps/mobile/src/adapters/deployment-registry.ts`
- Test: `apps/mobile/src/adapters/deployment-registry.test.ts`（新建）

**Interfaces:**
- Consumes: `SecureStorePort`（`apps/mobile/src/adapters/secure-store.ts:5-9`）；Task 1 的 `DeploymentRegistry`（`@weknora/mobile-core` 公共导出）。
- Produces: `createSecureDeploymentRegistry(store: SecureStorePort): DeploymentRegistry`、`createNativeSecureDeploymentRegistry(): DeploymentRegistry`（仅原生组合路径加载 expo-secure-store）。

- [ ] **Step 1: 写失败测试**

创建 `apps/mobile/src/adapters/deployment-registry.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createSecureDeploymentRegistry } from './deployment-registry.ts';
import type { SecureStorePort } from './secure-store.ts';

function secureStore(): SecureStorePort & { values: Map<string, string> } {
  const values = new Map<string, string>();
  return {
    values,
    async getItemAsync(key) { return values.get(key) ?? null; },
    async setItemAsync(key, value) { values.set(key, value); },
    async deleteItemAsync(key) { values.delete(key); },
  };
}

test('secure deployment registry upserts most-recent-first without duplicates', async () => {
  const store = secureStore();
  const registry = createSecureDeploymentRegistry(store);

  await registry.upsert({ origin: 'https://weknora.example.test', label: 'WeKnora' });
  await registry.upsert({ origin: 'https://other.example.test', label: 'Other' });
  await registry.upsert({ origin: 'https://weknora.example.test', label: 'WeKnora Cloud' });

  assert.deepEqual(await registry.list(), [
    { origin: 'https://weknora.example.test', label: 'WeKnora Cloud' },
    { origin: 'https://other.example.test', label: 'Other' },
  ]);
});

test('secure deployment registry removes one origin and keeps the rest', async () => {
  const store = secureStore();
  const registry = createSecureDeploymentRegistry(store);
  await registry.upsert({ origin: 'https://weknora.example.test', label: 'WeKnora' });
  await registry.upsert({ origin: 'https://other.example.test', label: 'Other' });

  await registry.remove('https://weknora.example.test');

  assert.deepEqual(await registry.list(), [{ origin: 'https://other.example.test', label: 'Other' }]);
  await registry.remove('https://missing.example.test');
  assert.deepEqual(await registry.list(), [{ origin: 'https://other.example.test', label: 'Other' }]);
});

test('a malformed registry payload reads as an empty list and is rebuilt on the next upsert', async () => {
  const store = secureStore();
  await store.setItemAsync('weknora.deployment-registry.v1', '{"origin":"https://not-an-array.test"}');
  const registry = createSecureDeploymentRegistry(store);

  assert.deepEqual(await registry.list(), []);

  await registry.upsert({ origin: 'https://weknora.example.test', label: 'WeKnora' });

  assert.deepEqual(await registry.list(), [{ origin: 'https://weknora.example.test', label: 'WeKnora' }]);
});

test('registry rows never store credentials and fall back to the origin as the label', async () => {
  const store = secureStore();
  const registry = createSecureDeploymentRegistry(store);
  await registry.upsert({ origin: 'https://weknora.example.test', label: '' });

  const raw = JSON.parse(store.values.get('weknora.deployment-registry.v1')!) as Array<Record<string, unknown>>;
  assert.deepEqual(raw, [{ origin: 'https://weknora.example.test', label: 'https://weknora.example.test' }]);
  assert.deepEqual(await registry.list(), [{ origin: 'https://weknora.example.test', label: 'https://weknora.example.test' }]);
});
```

注意：空 label 的兜底由 Adapter 负责（Runtime 的 `normalizeDeployment` 只在登录路径生效）。

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL——`./deployment-registry.ts` 模块不存在（`Cannot find module`）。

- [ ] **Step 3: 最小实现**

创建 `apps/mobile/src/adapters/deployment-registry.ts`：

```ts
import type { DeploymentRegistry } from '@weknora/mobile-core';
import type { SecureStorePort } from './secure-store.ts';

const REGISTRY_KEY = 'weknora.deployment-registry.v1';

function parseRecords(raw: string | null): Array<{ origin: string; label: string }> | undefined {
  try {
    const value: unknown = raw && JSON.parse(raw);
    if (!Array.isArray(value)) return undefined;
    const records: Array<{ origin: string; label: string }> = [];
    for (const entry of value) {
      if (!entry || typeof entry !== 'object' || Array.isArray(entry)) return undefined;
      const record = entry as { origin?: unknown; label?: unknown };
      if (typeof record.origin !== 'string' || record.origin.trim() === '') return undefined;
      const label = typeof record.label === 'string' && record.label.trim() !== '' ? record.label.trim() : record.origin;
      records.push({ origin: record.origin, label });
    }
    return records;
  } catch {
    return undefined;
  }
}

/** OS-backed registered deployment list. Persists presentation-safe origins and labels only, never credentials. */
export function createSecureDeploymentRegistry(store: SecureStorePort): DeploymentRegistry {
  return {
    async list() { return parseRecords(await store.getItemAsync(REGISTRY_KEY)) ?? []; },
    async upsert(deployment) {
      const records = parseRecords(await store.getItemAsync(REGISTRY_KEY)) ?? [];
      const label = deployment.label.trim() !== '' ? deployment.label.trim() : deployment.origin;
      const next = [{ origin: deployment.origin, label }, ...records.filter((record) => record.origin !== deployment.origin)];
      await store.setItemAsync(REGISTRY_KEY, JSON.stringify(next));
    },
    async remove(origin) {
      const records = parseRecords(await store.getItemAsync(REGISTRY_KEY)) ?? [];
      await store.setItemAsync(REGISTRY_KEY, JSON.stringify(records.filter((record) => record.origin !== origin)));
    },
  };
}

/** Loads Expo SecureStore only in the native composition path. */
export function createNativeSecureDeploymentRegistry(): DeploymentRegistry {
  return createSecureDeploymentRegistry(require('expo-secure-store') as SecureStorePort);
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --filter @weknora/mobile test`
Expected: PASS（新 4 用例 + 既有 apps/mobile 用例全绿）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/adapters/deployment-registry.ts apps/mobile/src/adapters/deployment-registry.test.ts
git commit -m "feat(mobile): secure deployment registry adapter"
```

---

### Task 4: apps/mobile：登录屏实例列表、Home 实例切换与 composition 接线

**Files:**
- Modify: `apps/mobile/src/screens/DeploymentLoginScreen.tsx`（props 与渲染，`DeploymentLoginScreen.tsx:4-8` 与 `:35-46`）
- Modify: `apps/mobile/src/screens/HomeScreen.tsx`（props 与头部，`HomeScreen.tsx:6-13` 与 `:32-41`）
- Modify: `apps/mobile/src/composition.ts`（import、`RuntimeSurfaceProps`、`RuntimeSurface` 两分支、`MobileApp`，`composition.ts:1-25`、`:80-128`、`:150-164`）
- Test: `apps/mobile/src/app-smoke.test.tsx`（文件末尾追加三个用例，复用文件内既有 `descendants`/`render`/`hooks`/`fakeTaskOffice`）

**Interfaces:**
- Consumes: Task 1 的 `MobileRuntime.listDeployments/switchDeployment`、`Deployment` 类型；Task 3 的 `createNativeSecureDeploymentRegistry`。
- Produces: `DeploymentLoginScreenProps.{ deployments?: Array<{ origin: string; label: string }>, onSwitchDeployment?(origin: string): Promise<void> }`；`HomeScreenProps.{ otherDeployments?: Array<{ origin: string; label: string }>, onSwitchDeployment?(origin: string): Promise<void> }`；`RuntimeSurfaceProps.{ deployments?: Deployment[], onSwitchDeployment?: (origin: string) => Promise<void> }`。

- [ ] **Step 1: 写失败测试**

在 `apps/mobile/src/app-smoke.test.tsx` 文件末尾（`fakeTaskOffice` 定义之后的现有用例之后）追加：

```tsx
test('the login surface lists registered deployments and switches through the runtime callback', async () => {
  const { DeploymentLoginScreen } = await import('./screens/DeploymentLoginScreen.tsx');
  hooks().__reset();
  const switched: string[] = [];
  const props = {
    deployments: [{ origin: 'https://weknora.example.test', label: 'WeKnora' }, { origin: 'https://other.example.test', label: 'Other' }],
    onSignIn: async () => {},
    onBeginOidc: async () => {},
    onSwitchDeployment: async (origin: string) => { switched.push(origin); },
  };
  const element = render(DeploymentLoginScreen, props);
  const texts = descendants(element).filter(({ type }) => type === 'Text').flatMap(({ props: p }) => p.children).flatMap((part) => (typeof part === 'string' ? [part] : []));
  assert.equal(texts.includes('Registered deployments'), true, 'the registered instance list is visible before manual origin entry');
  const other = descendants(element).find(({ type, props: p }) => type === 'Button' && p.title === 'Other');
  assert.ok(other, 'each registered instance renders a switch button');
  (other!.props.onPress as () => void)();
  assert.deepEqual(switched, ['https://other.example.test']);
});

test('the home header switches to another registered deployment through the callback', async () => {
  const { HomeScreen } = await import('./screens/HomeScreen.tsx');
  hooks().__reset();
  const switched: string[] = [];
  const element = render(HomeScreen, {
    deploymentLabel: 'WeKnora',
    tenants: [{ id: '7', name: 'Acme' }],
    activeTenantId: '7',
    onActivateTenant: () => {},
    onSignOut: async () => {},
    taskOffice: fakeTaskOffice({}),
    otherDeployments: [{ origin: 'https://other.example.test', label: 'Other' }],
    onSwitchDeployment: async (origin: string) => { switched.push(origin); },
  });
  const button = descendants(element).find(({ type, props: p }) => type === 'Button' && p.title === 'Switch to Other');
  assert.ok(button, 'another registered deployment must render a switch button on the authorized surface');
  (button!.props.onPress as () => void)();
  assert.deepEqual(switched, ['https://other.example.test']);
});

test('RuntimeSurface passes other registered deployments to the home screen and the full list to login', async () => {
  const { RuntimeSurface } = await import('./composition.ts');
  const authorized = RuntimeSurface({
    snapshot: { surface: 'authorized', deployment: { origin: 'https://weknora.example.test', label: 'WeKnora' }, identity: { userId: 'member-1', activeTenantId: '7', tenants: [{ id: '7' }] } },
    deployments: [{ origin: 'https://weknora.example.test', label: 'WeKnora' }, { origin: 'https://other.example.test', label: 'Other' }],
    onSignIn: async () => {}, onBeginOidc: async () => {}, onSignOut: async () => {}, onActivateTenant: async () => {},
    onSwitchDeployment: async () => {},
  });
  assert.equal(authorized.type.name, 'HomeScreen');
  assert.deepEqual((authorized.props as { otherDeployments?: Array<{ origin: string; label: string }> }).otherDeployments, [{ origin: 'https://other.example.test', label: 'Other' }]);

  const login = RuntimeSurface({
    snapshot: { surface: 'deployment-login' },
    deployments: [{ origin: 'https://other.example.test', label: 'Other' }],
    onSignIn: async () => {}, onBeginOidc: async () => {}, onSignOut: async () => {}, onActivateTenant: async () => {},
    onSwitchDeployment: async () => {},
  });
  assert.equal(login.type.name, 'DeploymentLoginScreen');
  assert.deepEqual((login.props as { deployments?: Array<{ origin: string; label: string }> }).deployments, [{ origin: 'https://other.example.test', label: 'Other' }]);
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL——新 3 用例失败（props 不存在：`texts.includes('Registered deployments')` 为 false、找不到 `'Switch to Other'`/`otherDeployments` 为 undefined）。

- [ ] **Step 3: 最小实现**

1. `apps/mobile/src/screens/DeploymentLoginScreen.tsx`——props 接口与组件签名改为：

```tsx
export interface DeploymentLoginScreenProps {
  officialCloudOrigin?: string;
  deployments?: Array<{ origin: string; label: string }>;
  onSignIn(input: { origin: string; email: string; password: string }): Promise<void>;
  onBeginOidc(input: { origin: string }): Promise<void>;
  onSwitchDeployment?(origin: string): Promise<void>;
}
```

```tsx
export function DeploymentLoginScreen({ officialCloudOrigin, deployments, onSignIn, onBeginOidc, onSwitchDeployment }: DeploymentLoginScreenProps) {
```

在 `<Text>Sign in to WeKnora</Text>` 之后、`officialCloudOrigin` 按钮之前插入：

```tsx
      {deployments && deployments.length > 0 ? (
        <View>
          <Text>Registered deployments</Text>
          {deployments.map((deployment) => (
            <Button key={deployment.origin} title={deployment.label} onPress={() => { void onSwitchDeployment?.(deployment.origin); }} />
          ))}
        </View>
      ) : null}
```

2. `apps/mobile/src/screens/HomeScreen.tsx`——props 接口加两行、组件签名解参、头部渲染（租户切换之后、`Sign out` 之前）：

```tsx
export interface HomeScreenProps {
  deploymentLabel: string;
  tenants: Array<{ id: string; name?: string }>;
  activeTenantId: string;
  onActivateTenant(tenantId: string): void;
  onSignOut(): Promise<void>;
  taskOffice: TaskOffice;
  otherDeployments?: Array<{ origin: string; label: string }>;
  onSwitchDeployment?(origin: string): Promise<void>;
}
```

```tsx
export function HomeScreen({ deploymentLabel, tenants, activeTenantId, onActivateTenant, onSignOut, taskOffice, otherDeployments, onSwitchDeployment }: HomeScreenProps) {
```

在 `<Button title="Sign out" ... />` 之前插入：

```tsx
      {(otherDeployments ?? []).map((deployment) => (
        <Button key={deployment.origin} title={`Switch to ${deployment.label}`} onPress={() => { void onSwitchDeployment?.(deployment.origin); }} />
      ))}
```

3. `apps/mobile/src/composition.ts`：
   - import 区：`import { createElement, useEffect, useRef, useState, useSyncExternalStore } from 'react';`（补 `useState`）；`import type { MobileRuntime, RuntimeSnapshot, ScopedVault, Deployment } from '@weknora/mobile-core';`（补 `Deployment`）；加 `import { createNativeSecureDeploymentRegistry } from './adapters/deployment-registry.ts';`
   - `createNativeMobileRuntime`（`composition.ts:46-78`）的 ports 中 `deploymentStore: createNativeSecureDeploymentStore(),` 之后加：

```ts
    deploymentRegistry: createNativeSecureDeploymentRegistry(),
```

   - `RuntimeSurfaceProps`（`composition.ts:80-86`）加两行：

```ts
  deployments?: Deployment[];
  onSwitchDeployment?: (origin: string) => Promise<void>;
```

   - `RuntimeSurface`（`composition.ts:112-128`）：authorized 分支的 `createElement(HomeScreen, {...})` 增加 `otherDeployments: (deployments ?? []).filter((deployment) => deployment.origin !== snapshot.deployment?.origin), onSwitchDeployment,`；deployment-login 分支改为：

```ts
  if (snapshot.surface === 'deployment-login') {
    return createElement(DeploymentLoginScreen, { officialCloudOrigin, deployments, onSignIn, onBeginOidc, onSwitchDeployment });
  }
```

   - `MobileApp`（`composition.ts:150-164`）增加登记清单状态并在快照变化后重取：

```tsx
export function MobileApp() {
  const activeRuntime = runtime();
  const booted = useRef(false);
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  useEffect(() => {
    bootRuntimeOnce(activeRuntime, booted);
  }, [activeRuntime]);
  const snapshot = useSyncExternalStore(activeRuntime.subscribe, activeRuntime.snapshot, activeRuntime.snapshot);
  useEffect(() => {
    void activeRuntime.listDeployments().then(setDeployments);
  }, [activeRuntime, snapshot]);
  return createElement(RuntimeSurface, {
    snapshot,
    deployments,
    onSignIn: async ({ origin, email, password }) => { await activeRuntime.signIn({ deployment: { origin }, email, password }); },
    onBeginOidc: async ({ origin }) => { await activeRuntime.beginOidc({ deployment: { origin }, redirectUri: OIDC_REDIRECT_URI }); },
    onSignOut: () => activeRuntime.signOut(),
    onActivateTenant: async (tenantId) => { await activeRuntime.activateTenant(tenantId); },
    onSwitchDeployment: async (origin) => { await activeRuntime.switchDeployment(origin); },
  });
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: PASS（新增 3 用例 + 既有全部用例 + 类型检查零错误；既有用例因新 props 均为可选而不受影响）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/screens/DeploymentLoginScreen.tsx apps/mobile/src/screens/HomeScreen.tsx apps/mobile/src/composition.ts apps/mobile/src/app-smoke.test.tsx
git commit -m "feat(mobile): registered instance list and in-app deployment switching"
```

---

### Task 5: apps/mobile：ReadOnlyScreen 只读降级面

**Files:**
- Create: `apps/mobile/src/screens/ReadOnlyScreen.tsx`
- Modify: `apps/mobile/src/composition.ts`（import + `RuntimeSurface` 新分支，`composition.ts:112-128`）
- Test: `apps/mobile/src/app-smoke.test.tsx`（文件末尾再追加两个用例）

**Interfaces:**
- Consumes: Task 2 的 `RuntimeSurface = 'read-only'` 与该面上可用的 `MobileRuntime.resourceShelf()`；`createResourceShelfController`/`ResourceShelfViewState`（`apps/mobile/src/resources-view.ts`）、`ResourcesScreen`（`apps/mobile/src/screens/ResourcesScreen.tsx`）。
- Produces: `ReadOnlyScreenProps { deploymentLabel?: string; handle?: ResourceShelfHandle; onSignOut(): Promise<void> }` 与 `ReadOnlyScreen`（组合根 `'read-only'` 分支唯一渲染目标；不 import `app/` 路由文件，避免 composition→screens→app→composition 环）。

- [ ] **Step 1: 写失败测试**

在 `apps/mobile/src/app-smoke.test.tsx` 末尾追加：

```tsx
test('RuntimeSurface routes the read-only snapshot to a restricted explanation surface', async () => {
  const { RuntimeSurface } = await import('./composition.ts');
  hooks().__reset();
  const surface = RuntimeSurface({
    snapshot: { surface: 'read-only', deployment: { origin: 'https://weknora.example.test', label: 'WeKnora' }, identity: { userId: 'member-1', activeTenantId: '7' }, reason: 'protocol-mismatch' },
    onSignIn: async () => {}, onBeginOidc: async () => {}, onSignOut: async () => {}, onActivateTenant: async () => {},
  });
  assert.equal(surface.type.name, 'ReadOnlyScreen');
  const element = render(surface.type, surface.props);
  const texts = descendants(element).filter(({ type }) => type === 'Text').flatMap(({ props: p }) => p.children).flatMap((part) => (typeof part === 'string' ? [part] : []));
  assert.equal(texts.some((text) => text.includes('Limited read-only mode')), true);
  assert.equal(texts.some((text) => text.includes('behind this version')), true);
  assert.equal(texts.some((text) => text.includes('Read-only browsing is unavailable.')), true, 'without a shelf handle the surface says so instead of guessing content');
  const buttons = descendants(element).filter(({ type }) => type === 'Button').map(({ props: p }) => p.title);
  assert.equal(buttons.includes('Sign out'), true);
  assert.equal(buttons.includes('View all tasks'), false, 'read-only must not expose task controls');
});

test('ReadOnlyScreen mounts the browse-only shelf once and renders its projection without authorized controls', async () => {
  const { ReadOnlyScreen } = await import('./screens/ReadOnlyScreen.tsx');
  const { ResourcesScreen } = await import('./screens/ResourcesScreen.tsx');
  hooks().__reset();
  const page: import('@weknora/mobile-core').ResourcePage = {
    tenantId: '7',
    agents: [{ id: 'agent-1', name: 'Research', summary: '', kind: 'custom', capability: { state: 'supported', reason: '' } }],
    knowledge: [],
    connections: [],
    classVerdicts: { agent: { state: 'supported', reason: '' }, knowledge: { state: 'supported', reason: '' }, connection: { state: 'supported', reason: '' } },
  };
  let browses = 0;
  const listeners = new Set<(event: import('@weknora/mobile-core').ShelfInvalidationEvent) => void>();
  const handle = {
    async browse() { browses += 1; return page; },
    selection: () => ({ allowed: false as const, state: 'unavailable' as const, reason: 'not_used' }),
    subscribe(listener: (event: import('@weknora/mobile-core').ShelfInvalidationEvent) => void) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    close() {},
  };
  const element = render(ReadOnlyScreen, { deploymentLabel: 'WeKnora', handle: handle as import('@weknora/mobile-core').ResourceShelfHandle, onSignOut: async () => {} });
  assert.equal(descendants(element).some(({ type }) => type === ResourcesScreen), true, 'the read-only surface renders the resource projection');
  hooks().__mount();
  assert.equal(browses, 1, 'the read-only shelf loads exactly once on mount');
  hooks().__unmount();
  assert.equal(listeners.size, 0, 'unmount detaches the shelf subscription');
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL——`./screens/ReadOnlyScreen.tsx` 模块不存在（`Cannot find module`）。

- [ ] **Step 3: 最小实现**

创建 `apps/mobile/src/screens/ReadOnlyScreen.tsx`：

```tsx
import { useEffect, useRef, useState } from 'react';
import { Button, Text, View } from 'react-native';
import type { ResourceShelfHandle } from '@weknora/mobile-core';
import { createResourceShelfController, type ResourceShelfViewState } from '../resources-view.ts';
import { ResourcesScreen } from './ResourcesScreen.tsx';

export interface ReadOnlyScreenProps {
  deploymentLabel?: string;
  handle?: ResourceShelfHandle;
  onSignOut(): Promise<void>;
}

/**
 * 有限只读降级面（spec：Missing security-critical capabilities produce an explanation or
 * limited read-only mode, not optimistic calls）。只渲染说明 + Resource Shelf 只读投影；
 * 不挂 Task Office，不提供任何授权控制。
 */
export function ReadOnlyScreen({ deploymentLabel, handle, onSignOut }: ReadOnlyScreenProps) {
  const controllerRef = useRef<ReturnType<typeof createResourceShelfController> | undefined>(undefined);
  // 初始投影是纯计算：与 app/resources.tsx 的 ResourcesRouteLifecycle 相同的 commit 后副作用纪律。
  const [state, setState] = useState<ResourceShelfViewState>(handle ? { loading: true } : { loading: false });
  useEffect(() => {
    if (!handle) return;
    const controller = createResourceShelfController(handle);
    controllerRef.current = controller;
    setState(controller.state());
    const unsubscribe = controller.subscribe(setState);
    return () => {
      unsubscribe();
      controller.dispose();
      controllerRef.current = undefined;
    };
  }, [handle]);
  return (
    <View>
      <Text>Limited read-only mode</Text>
      <Text>{deploymentLabel ? `${deploymentLabel} is behind this version of WeKnora.` : 'This deployment is behind this version of WeKnora.'}</Text>
      <Text>Task commands are disabled. Ask the deployment administrator to upgrade, or switch to another deployment.</Text>
      <Button title="Sign out" onPress={() => { void onSignOut(); }} />
      {handle
        ? <ResourcesScreen page={state.page} loading={state.loading} error={state.error} onRefresh={() => { controllerRef.current?.refresh(); }} />
        : <Text>Read-only browsing is unavailable.</Text>}
    </View>
  );
}
```

`apps/mobile/src/composition.ts`：import 区加 `import { ReadOnlyScreen } from './screens/ReadOnlyScreen.tsx';`；`RuntimeSurface` 中 authorized 分支之后、deployment-login 分支之前插入：

```ts
  if (snapshot.surface === 'read-only') {
    return createElement(ReadOnlyScreen, {
      deploymentLabel: snapshot.deployment?.label,
      handle: runtime().resourceShelf(),
      onSignOut,
    });
  }
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: PASS（新增 2 用例 + 全部既有用例 + 类型检查零错误）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/screens/ReadOnlyScreen.tsx apps/mobile/src/composition.ts apps/mobile/src/app-smoke.test.tsx
git commit -m "feat(mobile): read-only downgrade screen with browse-only resources"
```

---

### Task 6: 端到端集成证据：多 Deployment 真实 HTTP 切换（opt-in）

**Files:**
- Modify: `apps/mobile/src/runtime-integration-smoke.ts`（config/evidence 类型与校验、`runMobileRuntimeIntegration`、主机防线参数化，`runtime-integration-smoke.ts:11-26`、`:37-120`、`:123-156`、`:180-235`）
- Test: `packages/api-client/src/mobile/runtime.integration.test.ts`（末尾追加 4 用例 + 更新既有 emit 用例的字面量，`runtime.integration.test.ts:50-75`）

**Interfaces:**
- Consumes: Task 1 的 `createInMemoryDeploymentRegistry`/`MobileRuntime.switchDeployment`/`listDeployments`；既有 `WEKNORA_MOBILE_TEST_*` 配置骨架与 `disallowedDeploymentHost` 防线（`runtime-integration-smoke.ts:109-156`）；`runMobileRuntimeIntegration`/`emitMobileRuntimeIntegrationEvidence`。
- Produces: `MobileRuntimeIntegrationConfig` enabled 形态新增 `alt?: { deploymentOrigin: string; email: string; password: string }`；`MobileRuntimeIntegrationEvidence` 新增 `deploymentSwitch: 'skipped' | 'switched' | 'switch-failed'` 与 `registeredDeployments: number`；环境变量 `WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL/_EMAIL/_PASSWORD`（三变量同进同出，ALT 源不得与主源相同，均过公网主机防线）。

- [ ] **Step 1: 写失败测试**

1. 更新 `packages/api-client/src/mobile/runtime.integration.test.ts:50-75` 既有 emit 用例的字面量（新增两个必填证据字段）：

```ts
test('non-authorized evidence is emitted before assertions without credential fields', () => {
  const emitted: string[] = [];
  emitMobileRuntimeIntegrationEvidence({
    deploymentOrigin: 'https://deployment.example',
    clientProtocol: 3,
    capabilityMode: 'incompatible',
    identity: 'absent',
    outcome: 'not-authorized',
    tenantSwitch: 'skipped',
    deploymentSwitch: 'skipped',
    registeredDeployments: 1,
    resourceShelf: 'browse-failed',
    commandTimestamp: '2026-09-21T00:00:00.000Z',
  }, (record) => emitted.push(record));

  assert.equal(emitted.length, 1);
  assert.deepEqual(JSON.parse(emitted[0]!), {
    deploymentOrigin: 'https://deployment.example',
    clientProtocol: 3,
    capabilityMode: 'incompatible',
    identity: 'absent',
    outcome: 'not-authorized',
    tenantSwitch: 'skipped',
    deploymentSwitch: 'skipped',
    registeredDeployments: 1,
    resourceShelf: 'browse-failed',
    commandTimestamp: '2026-09-21T00:00:00.000Z',
  });
  assert.doesNotMatch(emitted[0]!, /short-lived-secret|password|token|email/i);
});
```

2. 在同一文件末尾追加：

```ts
test('real HTTP multi-deployment switch restores the prior instance without a second login', async (t) => {
  const config = mobileRuntimeIntegrationConfig(process.env);
  if (!config.enabled) {
    if (config.disposition === 'skip') t.skip(`MOBILE_RUNTIME_HTTP_SKIPPED: ${config.reason}`);
    else assert.fail(`MOBILE_RUNTIME_HTTP_INVALID: ${config.reason}`);
    return;
  }
  if (!config.alt) {
    t.skip('MOBILE_RUNTIME_HTTP_DEPLOYMENT_SWITCH_SKIPPED: missing WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL/_EMAIL/_PASSWORD');
    return;
  }

  const evidence = await runMobileRuntimeIntegration(config);
  emitMobileRuntimeIntegrationEvidence(evidence, (record) => t.diagnostic(record));

  assert.equal(evidence.outcome, 'authorized');
  assert.equal(evidence.deploymentSwitch, 'switched', 'switching back must reuse the stored credential instead of a new login');
  assert.equal(evidence.registeredDeployments, 2, 'both live deployments must be registered on the device');
});

test('integration config marks partial alternate deployment credentials invalid rather than skippable', () => {
  const config = mobileRuntimeIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://deployment.example',
    WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
    WEKNORA_MOBILE_TEST_PASSWORD: nonEmptyPassword,
    WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL: 'https://alt.example',
  });

  assert.equal(config.enabled, false);
  assert.equal(config.disposition, 'invalid');
  assert.match(config.disposition === 'invalid' ? config.reason : '', /must be provided together/);
});

test('integration config rejects an alternate deployment identical to the primary', () => {
  const config = mobileRuntimeIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://deployment.example',
    WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
    WEKNORA_MOBILE_TEST_PASSWORD: nonEmptyPassword,
    WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL: 'https://deployment.example',
    WEKNORA_MOBILE_TEST_ALT_EMAIL: 'alt@example.test',
    WEKNORA_MOBILE_TEST_ALT_PASSWORD: nonEmptyPassword,
  });

  assert.deepEqual(config, {
    enabled: false,
    disposition: 'invalid',
    reason: 'WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL must differ from WEKNORA_MOBILE_TEST_DEPLOYMENT_URL',
  });
});

test('integration config rejects a loopback alternate deployment host', () => {
  const config = mobileRuntimeIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://deployment.example',
    WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
    WEKNORA_MOBILE_TEST_PASSWORD: nonEmptyPassword,
    WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL: 'https://127.0.0.1',
    WEKNORA_MOBILE_TEST_ALT_EMAIL: 'alt@example.test',
    WEKNORA_MOBILE_TEST_ALT_PASSWORD: nonEmptyPassword,
  });

  assert.equal(config.enabled, false);
  assert.equal(config.disposition, 'invalid');
  assert.match(config.disposition === 'invalid' ? config.reason : '', /must not target/);
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test packages/api-client/src/mobile/runtime.integration.test.ts`
Expected: FAIL——类型/断言失败：`deploymentSwitch`/`registeredDeployments` 不在 `MobileRuntimeIntegrationEvidence` 上（emit 用例与新增配置用例报错，alt 相关 config 用例返回的仍是 enabled 或 skip 形态）。

- [ ] **Step 3: 最小实现**

`apps/mobile/src/runtime-integration-smoke.ts`：

1. 类型（`runtime-integration-smoke.ts:11-26`）：

```ts
export type MobileRuntimeIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string; switchTenantId?: string; alt?: { deploymentOrigin: string; email: string; password: string } }
  | { enabled: false; disposition: 'skip'; reason: string }
  | { enabled: false; disposition: 'invalid'; reason: string };

export interface MobileRuntimeIntegrationEvidence {
  deploymentOrigin: string;
  clientProtocol: number;
  capabilityMode: 'compatible' | 'incompatible' | 'unknown';
  identity: 'present' | 'absent';
  outcome: 'authorized' | 'not-authorized';
  tenantSwitch: 'skipped' | 'switched' | 'switch-failed';
  deploymentSwitch: 'skipped' | 'switched' | 'switch-failed';
  registeredDeployments: number;
  resourceShelf: 'not-authorized' | 'browsed' | 'browse-failed';
  resourceCounts?: { agents: number; knowledge: number; connections: number };
  commandTimestamp: string;
}
```

2. 主机防线参数化（`runtime-integration-smoke.ts:37-120`）：`disallowedIpv4Octets`、`disallowedIpv6Bytes`、`disallowedDeploymentHost` 三个函数各加一个首参 `variable: string`，函数体内所有消息前缀 `'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL'` 改为模板 `` `${variable}` ``（主调用点传入 `'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL'`，既有断言的消息文本逐字不变）。注意 `disallowedIpv6Bytes` 内部对 `disallowedIpv4Octets` 的 3 处调用点（IPv4-mapped `::ffff:0:0/96`、IPv4-compatible `::/96`、6to4 `2002::/16`，即 `runtime-integration-smoke.ts:85/90/101`）在首参增加后同样必须透传 `variable`，否则 typecheck 缺参报错。`parseIpv6Literal` 不动。

3. `mobileRuntimeIntegrationConfig`（`:123-156`）：primary 校验之后、`switchTenantId` 校验之前插入 ALT 解析，并在返回值携带 `alt`：

```ts
  const altOrigin = env.WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL?.trim();
  const altEmail = env.WEKNORA_MOBILE_TEST_ALT_EMAIL?.trim();
  const altPassword = env.WEKNORA_MOBILE_TEST_ALT_PASSWORD;
  const altPresent = [altOrigin, altEmail, altPassword].filter((value) => value !== undefined && value !== '').length;
  if (altPresent !== 0 && altPresent !== 3) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL, WEKNORA_MOBILE_TEST_ALT_EMAIL and WEKNORA_MOBILE_TEST_ALT_PASSWORD must be provided together' };
  }
  let alt: { deploymentOrigin: string; email: string; password: string } | undefined;
  if (altPresent === 3) {
    let parsedAlt: URL;
    try {
      parsedAlt = new URL(altOrigin!);
    } catch {
      return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL is not an absolute URL' };
    }
    if (parsedAlt.protocol !== 'https:' || parsedAlt.username || parsedAlt.password || parsedAlt.pathname !== '/' || parsedAlt.search || parsedAlt.hash) {
      return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL must be a credential-free HTTPS origin' };
    }
    const altHostRejection = disallowedDeploymentHost(parsedAlt.hostname, 'WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL');
    if (altHostRejection) return { enabled: false, disposition: 'invalid', reason: altHostRejection };
    if (parsedAlt.origin === parsed.origin) {
      return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL must differ from WEKNORA_MOBILE_TEST_DEPLOYMENT_URL' };
    }
    alt = { deploymentOrigin: parsedAlt.origin, email: altEmail!, password: altPassword! };
  }
```

既有 primary 调用点改为 `disallowedDeploymentHost(parsed.hostname, 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL')`；返回行改为：

```ts
  return { enabled: true, deploymentOrigin: parsed.origin, email, password, ...(switchTenantId ? { switchTenantId } : {}), ...(alt ? { alt } : {}) };
```

4. `runMobileRuntimeIntegration`（`:180-235`）：import 行加 `createInMemoryDeploymentRegistry`；runtime ports 加 `deploymentRegistry: createInMemoryDeploymentRegistry(),`；`remoteFor` 内包装 `passwordLogin` 计数：

```ts
  let passwordLogins = 0;
```

```ts
      const remote = createMobileRuntimeRemote({ origin, request: client.request });
      return {
        ...remote,
        async passwordLogin(input: { email: string; password: string }) {
          passwordLogins += 1;
          return remote.passwordLogin(input);
        },
        async deploymentCapabilities(accessToken: string) {
          const capabilities = await remote.deploymentCapabilities(accessToken);
          capabilityMode = capabilityEvidenceMode(capabilities);
          return capabilities;
        },
      };
```

在 tenantSwitch 证据段之后、shelf 证据之前插入：

```ts
  let deploymentSwitch: MobileRuntimeIntegrationEvidence['deploymentSwitch'] = 'skipped';
  if (config.alt && snapshot.surface === 'authorized') {
    const altSignIn = await runtime.signIn({
      deployment: { origin: config.alt.deploymentOrigin, label: 'Alt deployment' },
      email: config.alt.email,
      password: config.alt.password,
    });
    const loginsAfterAlt = passwordLogins;
    const leaseBeforeSwitch = runtime.scopeLease();
    const switchedBack = await runtime.switchDeployment(config.deploymentOrigin);
    deploymentSwitch = altSignIn.surface === 'authorized' &&
      switchedBack.surface === 'authorized' &&
      switchedBack.deployment?.origin === config.deploymentOrigin &&
      runtime.scopeLease() !== undefined &&
      runtime.scopeLease() !== leaseBeforeSwitch &&
      passwordLogins === loginsAfterAlt
      ? 'switched'
      : 'switch-failed';
  }
  const registeredDeployments = (await runtime.listDeployments()).length;
```

返回的 evidence 对象加入 `deploymentSwitch` 与 `registeredDeployments`（其余字段不动）。

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test packages/api-client/src/mobile/runtime.integration.test.ts && pnpm --filter @weknora/mobile test`
Expected: PASS——无环境变量时：第一个用例 skip（`MOBILE_RUNTIME_HTTP_SKIPPED`）、多实例用例 skip（`MOBILE_RUNTIME_HTTP_DEPLOYMENT_SWITCH_SKIPPED`）、全部配置校验/emit 用例 pass；apps/mobile 测试不回归（`runtime-integration-smoke.ts` 属其类型依赖面）。具备两个真实部署凭据时：

```bash
WEKNORA_MOBILE_TEST_DEPLOYMENT_URL=https://deployment.example \
WEKNORA_MOBILE_TEST_EMAIL=mobile-test@example.test \
WEKNORA_MOBILE_TEST_PASSWORD=short-lived-secret \
WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL=https://alt.example \
WEKNORA_MOBILE_TEST_ALT_EMAIL=alt-mobile-test@example.test \
WEKNORA_MOBILE_TEST_ALT_PASSWORD=short-lived-secret \
pnpm exec tsx --test packages/api-client/src/mobile/runtime.integration.test.ts
```

Expected: 两个真实 HTTP 用例 pass，诊断输出仅含脱敏证据 JSON（origin/枚举/计数/时间戳，无 token/凭据）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/runtime-integration-smoke.ts packages/api-client/src/mobile/runtime.integration.test.ts
git commit -m "test(mobile): opt-in real HTTP multi-deployment switch evidence (T36 AC3)"
```

---

## 计划级验证

在 worktree 根（`.worktrees/issue30-sweep`）执行：

```bash
pnpm exec tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts packages/mobile-core/src/runtime/runtime-deployments.test.ts packages/mobile-core/src/runtime/runtime-readonly.test.ts packages/mobile-core/src/runtime/runtime-shelf.test.ts packages/mobile-core/src/runtime/runtime-vault.test.ts packages/api-client/src/mobile/runtime.integration.test.ts && pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck
```

预期：mobile-core runtime 五个测试文件全绿（既有 33 + 新 14 + 改写 1）；api-client 集成文件在无环境变量时真实 HTTP 用例 skip、其余 pass；apps/mobile 测试与 `tsc --noEmit` 全绿。真机 Release 证据（#69/#70）与真实降级部署证据（自托管 `WEKNORA_WORKBENCH_PROTOCOL_MAXIMUM` 调低）不在本地自动化范围，见 Global Constraints 的 blocked-env 声明。
