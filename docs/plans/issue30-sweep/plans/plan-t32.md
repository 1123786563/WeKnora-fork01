# T02：Active Tenant 切换与 Scoped Vault 隔离（Issue #32）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用户可以在移动端选择并切换 Active Tenant；Token、Scope Lease、加密缓存（Scoped Vault）与在途请求按 Deployment、用户、Tenant 隔离，切换、退出时旧 scope 的加密 key 与缓存失效（fail closed），迟到响应不污染新空间。

**Architecture:** 复用已合并 T01 的 Mobile Runtime epoch/Scope Lease 机制：`activateTenant(tenantId)` 走与 `signIn` 相同的 `begin → 远端换签 → 持久化新凭据 → authenticate 复核` 流水线，天然获得迟到响应拒绝。新增 mobile-core 内部支持 Module Scoped Vault：以不透明 Scope Lease 为唯一开门凭证，按 `Deployment origin × userId × tenantId` 派生 scope key，注入 `CipherPort`（WebCrypto AES-GCM）/`KeyStorePort`/`VaultStoragePort` 三个 seam；Runtime 在每次 scope 变化（切租户/换部署/退出/销毁）时对旧 lease 派发 `vault.revoke`（先同步撤销 lease 使所有句柄立即失效，再异步把 wrapped key 轮换成新随机值并擦除行，保证部分擦除失败时旧数据也不可读）。apps/mobile 只加组装与切换 UI，不拥有业务规则。

**Tech Stack:** TypeScript（`packages/mobile-core`、`packages/api-client`、`apps/mobile`/Expo RN）、node:test + tsx（测试运行器，与现有 `mobile-runtime.test.ts` 一致）、WebCrypto `crypto.subtle` AES-GCM（Node 22 与具备 WebCrypto 的运行时可用；本计划已在 Node v22.22.3 实测：错误 key 与篡改密文均以 `OperationError` 拒绝）、Expo SecureStore（原生 KeyStore/Keychain seam）。后端复用既有 `POST /api/v1/auth/switch-tenant`（`internal/router/routes_auth_tenant.go:219` 注册，`internal/handler/auth.go:1119` handler；响应为 `AuthLoginResponse`，active tenant 字段名 `active_tenant`，`internal/handler/dto/auth.go:8-16`），api-client 侧 `createAuthApi().switchTenant` 已存在（`packages/api-client/src/auth/endpoints.ts:142-146`），本计划只补移动端 Remote 适配、Runtime API、Vault Module 与 UI。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-32.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（移动深 Module 划分、Implementation Decisions、Testing Decisions）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§4 Mobile Runtime Interface、§9 Scoped Vault Module、§13 Interface 测试面）
- ADR：`docs/adr/0007-registered-devices-and-encrypted-cache.md`（缓存加密与失效）、`docs/adr/0012-mobile-business-logic-lives-behind-deep-modules.md`（深 Module 边界）、`docs/adr/0005-weknora-native-mobile-client.md`
- 领域术语：`CONTEXT.md`（「活动空间（Active Tenant）」「部署实例（Deployment）」）
- Parent：Issue #30；Blocked by：#31（T01 登录 Deployment——其 Runtime/凭据/OIDC 骨架已合并进本仓，本计划直接消费）

## Global Constraints

以下为批准 Spec / ADR 的项目级约束，逐字引用，所有任务隐含遵守：

- 「Mobile Runtime owns Deployment, identity, Active Tenant, compatibility, device registration and Scope Lease.」（mobile-ai-office-design.md · Implementation Decisions）
- 「Scoped Vault owns encrypted scope storage, drafts, submission journal, event projection, retention and revocation.」（同上）
- 「Cache identity includes Deployment, user, Tenant, Task and Run where applicable. Scope changes invalidate subscriptions and reject late responses.」（同上）
- 「Mobile core does not depend on React Native, DOM or concrete transport. Remote and native details are injected as Adapters.」（同上）
- 「The mobile information architecture is Home, Tasks, New, Resources and Me.」（同上；本计划只动 authorized 落地页的租户切换入口，不新增一级入口）
- 「Interface 不暴露 token、query key、generation number 或 SecureStore key。Scope Lease 是不透明、可撤销的能力对象，子 Module 每次异步提交前检查其有效性。」（mobile-module-seams.md §4.2）
- 「ScopedStore 只提供按领域仓储分组的读写，不提供任意全局 key/value。任何没有有效 Scope Lease 的访问失败。」（mobile-module-seams.md §9.2）
- 「Mobile Runtime Interface tests cover login restoration, capability negotiation, Active Tenant switching, Scope Lease revocation, device revocation and late-response rejection.」（mobile-ai-office-design.md · Testing Decisions）
- 「Scoped Vault Interface tests cover encryption adapter failures, Deployment/user/Tenant isolation, key rotation, revocation, retention, offline drafts and rejection of offline side effects.」（同上）
- 「Tests target observable behavior at the highest stable Interface. Screen tests verify rendering and navigation; they do not duplicate Module internals.」（同上）
- ADR-0007：「获准离线内容按用户与 Tenant 分区并加密，空间可限制缓存范围和保留时间；登出、切换空间、设备撤销或权限失效后删除对应缓存或使其密钥不可用。」
- 安全约束（会话注入）：配置凭据只从环境变量或密钥服务读取；源码、示例和测试都不得写入可用的凭据字面量。本计划的真机/真部署集成测试沿用 T01 的 opt-in 环境变量模式，无任何回退凭据。服务端请求仅允许 http/https，且发出请求前校验 host 并拒绝 localhost、环回、私有和保留地址（Task 6 的 `disallowedDeploymentHost` 防线）。
- 工作流约束：严格 RED→GREEN→REFACTOR（每个任务先写失败测试、实跑确认失败、最小实现、通过、提交）；实现不与已批准 Spec 冲突，冲突时升级而非静默重设计。

**Issue #32 验收标准原文（docs/plans/issue30-sweep/issues/issue-32.md）：**

1. 「跨 Tenant 缓存和请求均 fail closed。」
2. 「切换、退出和撤权的加密 key 失效路径有自动化证据。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

验收标准 3 的本地可验证性说明（blocked-env 声明）：「最高稳定 Interface」的真实集成证据沿用 T01 已合并的 opt-in 真实 HTTP 测试模式（`packages/api-client/src/mobile/runtime.integration.test.ts` + `apps/mobile/src/runtime-integration-smoke.ts`，通过 `WEKNORA_MOBILE_TEST_*` 环境变量启用，无凭据回退）。租户切换的真实 HTTP 证据需要「一个真实 Deployment + 一个属于至少两个 Tenant 的测试账号」，本地无此环境时该测试以 `t.skip` 跳过（不得伪造通过）。本地替代证据为：mobile-core Interface 级测试（真实 WebCrypto、真实 Runtime 编排）+ api-client wire 契约测试（真实序列化字节）。计划内 Task 6 完成接线后，凡具备环境的运行都会自动产出租户切换证据。

## Review Focus

Spec 隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **迟到的旧租户响应**：`activateTenant('A')` 的 `me/capabilities` 还在途时用户又完成 `activateTenant('B')`，旧响应返回后不得把 active tenant 改回 A。——Task 2 测试「a late tenant verification cannot override a completed later switch」。
2. **Vault 持久化擦除失败**（Keychain 写入失败等）：擦除失败不得让已撤销句柄复活、也不得让 Runtime 调用被拒绝；wrapped key 先轮换成新随机值再删行，保证部分失败时旧密文不可读。——Task 4 测试「a failed vault erase keeps access closed and does not reject the runtime call」。
3. **密文被篡改或用错 key 解密**：必须抛错（GCM 认证失败），绝不返回明文垃圾或把损坏行当空值。——Task 3 测试「tampered ciphertext and wrong keys fail closed through the cipher seam」。
4. **非法 tenantId / 无成员资格切换**：非正整数 tenantId 在发出任何请求前抛错；服务器拒绝切换时旧 lease 必须已撤销、落 fail-closed 安全面、存储凭据不被破坏。——Task 1 测试「switchTenant rejects a non-positive or non-numeric tenant id before any request」；Task 2 测试「a rejected tenant switch fails closed without keeping the prior lease」。
5. **同 scope 重新拿到新 lease 后，旧 store 句柄仍在调用方手里**：旧句柄每次操作都要重新校验 lease 有效性，切换/退出后必须立即失败。——Task 3 测试「a vault without a valid lease fails closed on open and on every store operation」；Task 4 测试「switching tenant revokes the old scope: store ops fail and persisted rows are erased」。

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 1 | api-client Remote 适配 `switchTenant` | wire 契约 + 适配器 |
| 2 | mobile-core `activateTenant` + 快照租户选项 + 可内省 lease | Runtime API |
| 3 | Scoped Vault Module | 加密 scope 存储 + 轮换/撤销 |
| 4 | Runtime ↔ Vault 失效接线 | 切换/退出 key 失效自动化证据 |
| 5 | apps/mobile 切换 UI + 原生 Vault Adapter 组装 | 用户可见切换能力 |
| 6 | 真实 HTTP 集成证据（租户切换） | AC3 最高稳定 Interface 证据 |

前置条件：worktree 根执行过 `pnpm install`（本计划作者已在 `.worktrees/issue30-sweep` 实跑完成；全新 checkout 需先跑一次）。所有测试命令在 worktree 根（`.worktrees/issue30-sweep`）执行。

---

### Task 1: api-client 移动 Remote 增加 `switchTenant` 适配

**Files:**
- Modify: `packages/api-client/src/mobile/runtime.ts:17-32`（`MobileRuntimeRemote` 接口）、`packages/api-client/src/mobile/runtime.ts:76-109`（`createMobileRuntimeRemote` 返回对象）
- Test: `packages/api-client/src/mobile/runtime.test.ts`（文件末尾追加两个测试）

**Interfaces:**
- Consumes: `createAuthApi().switchTenant(tenantId: number, refreshToken?: string): Promise<AuthSession>`（已存在，`packages/api-client/src/auth/endpoints.ts:142-146`，POST `/api/v1/auth/switch-tenant`，body `{ tenant_id, refresh_token? }`）；`parseSession` 已把响应中的 `active_tenant` 归一为 `session.tenant`（`packages/api-client/src/auth/endpoints.ts:94`）。后端响应形状 `AuthLoginResponse`（`internal/handler/dto/auth.go:8-16`：`success/token/refresh_token/user/active_tenant/memberships`）。
- Produces: `MobileRuntimeRemote.switchTenant(input: { tenantId: string; refreshToken: string }): Promise<{ credential: { token: string; refreshToken: string }; tenant?: Record<string, unknown> | null }>`（Task 2 的 `ports.RuntimeRemote` 将以此为准）。

- [ ] **Step 1: 写失败测试**

在 `packages/api-client/src/mobile/runtime.test.ts` 文件末尾追加（复用文件顶部已有的 `recorder`/`ORIGIN`）：

```ts
test('switchTenant posts the wire body and unwraps the active tenant session', async () => {
  const spy = recorder((input) => {
    if (input.path === '/api/v1/auth/switch-tenant') {
      // 真实 AuthLoginResponse 形状（internal/handler/dto/auth.go:8-16）：
      // active_tenant 而非 tenant；parseSession 已归一。
      return {
        success: true, token: 'switched-a', refresh_token: 'switched-r',
        active_tenant: { id: 9, name: 'Beta' },
        memberships: [{ tenant_id: 9, tenant_name: 'Beta', role: 'viewer' }],
      };
    }
    throw new Error(`unexpected path ${input.path}`);
  });
  const remote = createMobileRuntimeRemote({ origin: ORIGIN, request: spy.request });

  const switched = await remote.switchTenant({ tenantId: '9', refreshToken: 'refresh-1' });

  assert.deepEqual(switched, { credential: { token: 'switched-a', refreshToken: 'switched-r' }, tenant: { id: 9, name: 'Beta' } });
  assert.equal(spy.seen.length, 1);
  assert.equal(spy.seen[0]!.method, 'POST');
  assert.deepEqual(spy.seen[0]!.body, { tenant_id: 9, refresh_token: 'refresh-1' });
});

test('switchTenant rejects a non-positive or non-numeric tenant id before any request', async () => {
  const spy = recorder(() => ({}));
  const remote = createMobileRuntimeRemote({ origin: ORIGIN, request: spy.request });

  await assert.rejects(remote.switchTenant({ tenantId: 'abc', refreshToken: 'refresh-1' }), /positive integer/);
  await assert.rejects(remote.switchTenant({ tenantId: '0', refreshToken: 'refresh-1' }), /positive integer/);
  await assert.rejects(remote.switchTenant({ tenantId: '9', refreshToken: ' ' }), /refreshToken is required/);

  assert.equal(spy.seen.length, 0, 'invalid input must not reach the wire');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/api-client/src/mobile/runtime.test.ts`
Expected: FAIL——新增两个测试报 `remote.switchTenant is not a function`（TS 类型错误在 tsx 下不拦截运行，运行期 TypeError 即 RED 证据）。

- [ ] **Step 3: 最小实现**

`packages/api-client/src/mobile/runtime.ts`：在 `MobileRuntimeRemote` 接口中 `refresh(...)` 声明之后插入：

```ts
  /**
   * POST /api/v1/auth/switch-tenant（internal/handler/auth.go:1119）。后端为目标空间
   * 重新签发令牌并把目标空间写为账号级「最近活跃租户」。适配器只做参数校验与
   * session 归一，新空间的授权复核（me/capabilities）由 Mobile Runtime 决定。
   */
  switchTenant(input: { tenantId: string; refreshToken: string }): Promise<{ credential: { token: string; refreshToken: string }; tenant?: Record<string, unknown> | null }>;
```

在 `createMobileRuntimeRemote` 返回对象中 `refresh(refreshToken) {...}` 之后插入：

```ts
    async switchTenant(input: { tenantId: string; refreshToken: string }) {
      const tenantId = Number(input.tenantId);
      if (!Number.isSafeInteger(tenantId) || tenantId <= 0) throw new Error('switchTenant tenantId must be a positive integer');
      if (typeof input.refreshToken !== 'string' || input.refreshToken.trim() === '') throw new Error('switchTenant refreshToken is required');
      const session = await auth.switchTenant(tenantId, input.refreshToken);
      return {
        credential: { token: session.token, refreshToken: session.refreshToken },
        tenant: session.tenant,
      };
    },
```

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/api-client/src/mobile/runtime.test.ts`
Expected: PASS（原有测试 + 新增 2 个全绿）。

- [ ] **Step 5: Commit**

```bash
git add packages/api-client/src/mobile/runtime.ts packages/api-client/src/mobile/runtime.test.ts
git commit -m "feat(mobile): switch-tenant remote adapter with pre-wire validation"
```

---

### Task 2: mobile-core `MobileRuntime.activateTenant` 与租户选项快照

**Files:**
- Create: `packages/mobile-core/src/runtime/scope-lease.ts`
- Modify: `packages/mobile-core/src/runtime/types.ts:23-40`（`RuntimeSnapshot.identity`、`MobileRuntime`、新增 `TenantOption`）
- Modify: `packages/mobile-core/src/runtime/ports.ts:21-29`（`RuntimeRemote` 加宽 `me` 返回类型携带 `memberships?: unknown[]`，并增加 `switchTenant`）
- Modify: `packages/mobile-core/src/runtime/mobile-runtime.ts:60-64`（删除本地 `RuntimeScopeLease`，改用共享实现）、`mobile-runtime.ts:139-169`（`authenticate` 解析 memberships、铸造带 scope 的 lease）、`mobile-runtime.ts:181-287`（新增 `activateTenant` 方法）
- Modify: `packages/mobile-core/src/index.ts`（导出 `TenantOption` 类型）
- Test: `packages/mobile-core/src/runtime/mobile-runtime.test.ts`（fake `remote()` 补默认 `switchTenant`；更新 3 处既有 deepEqual；追加 6 个测试）

**Interfaces:**
- Consumes: Task 1 的 `switchTenant` 适配签名；既有 `begin/reserve/current/safe/persistCredential/authenticate` 机制（`packages/mobile-core/src/runtime/mobile-runtime.ts:86-169`）。
- Produces:
  - `MobileRuntime.activateTenant(tenantId: string): Promise<RuntimeSnapshot>`（Spec §4.2 命名 `activateTenant`；返回值沿用既有 presentation-safe `RuntimeSnapshot` 约定而非裸 Lease，新 Scope Lease 经 `scopeLease()` 观察——与 T01 已合并的 `signIn/boot` 返回约定一致，差异记录见「差异记录」节）
  - `RuntimeSnapshot.identity?: { userId: string; activeTenantId?: string; tenants?: TenantOption[] }`，`TenantOption = { id: string; name?: string }`
  - `RuntimeRemote.switchTenant(input: { tenantId: string; refreshToken: string }): Promise<{ credential: StoredCredential; tenant?: Record<string, unknown> | null }>`（mobile-core 端口视图，与 Task 1 api-client 适配返回类型逐字一致——`Record<string, unknown> | null` 视图双向可赋值，实测 strict tsc 通过；credential 用 `StoredCredential`）；`RuntimeRemote.me` 返回类型加宽为 `{ user: { id: unknown }; tenant?: { id: unknown } | null; memberships?: unknown[] }`（api-client `AuthMe` 已含 `memberships`，加宽仅为让端口类型如实表达既有 wire 字段）
  - 包内共享 lease 内省：`class RuntimeScopeLease`、`leaseScopeOf(lease): LeaseScope | undefined`、`leaseActive(lease): boolean`、`interface LeaseScope { deploymentOrigin: string; userId: string; tenantId: string }`（`scope-lease.ts`，**不**从 `index.ts` 导出，供 Task 3/4 Vault 消费）

- [ ] **Step 1: 写失败测试**

(a) `packages/mobile-core/src/runtime/mobile-runtime.test.ts` 中 `function remote(overrides = {})`（40-51 行）的默认对象里，`refresh:` 之后补一行（否则后续新测试的端口类型不完整）：

```ts
    switchTenant: async (input) => ({ credential: { token: `tenant-${input.tenantId}-access`, refreshToken: `refresh-${input.tenantId}` }, tenant: { id: `tenant-${input.tenantId}` } }),
```

(b) 文件末尾追加测试：

```ts
test('activateTenant re-issues the credential, publishes the new active tenant, and revokes the prior lease', async () => {
  const store = fakeStore();
  const switches: Array<{ tenantId: string; refreshToken: string }> = [];
  const runtime = createMobileRuntime(ports(store, () => remote({
    me: async (token) => ({ user: { id: 'user-1' }, tenant: { id: token === 'tenant-2-access' ? 'tenant-2' : 'tenant-1' } }),
    switchTenant: async (input) => {
      switches.push(input);
      return { credential: { token: 'tenant-2-access', refreshToken: 'rotated-refresh' }, tenant: { id: 'tenant-2' } };
    },
  })));
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const priorLease = runtime.scopeLease();
  assert.ok(priorLease);

  const snapshot = await runtime.activateTenant('2');

  assert.deepEqual(switches, [{ tenantId: '2', refreshToken: 'refresh-1' }]);
  assert.equal(snapshot.identity?.activeTenantId, 'tenant-2');
  assert.notEqual(runtime.scopeLease(), priorLease);
  assert.equal(runtime.snapshot().identity?.activeTenantId, 'tenant-2');
  assert.deepEqual(await store.read(DEPLOYMENT.origin), { token: 'tenant-2-access', refreshToken: 'rotated-refresh' });
});

test('the authorized snapshot lists tenant options parsed from memberships', async () => {
  const runtime = createMobileRuntime(ports(fakeStore(), () => remote({
    me: async () => ({
      user: { id: 'user-1' }, tenant: { id: 7 },
      memberships: [{ tenant_id: 7, tenant_name: 'Acme', role: 'owner' }, { tenant_id: 9, tenant_name: 'Beta', role: 'viewer' }],
    }),
  })));
  const snapshot = await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  assert.deepEqual(snapshot.identity, {
    userId: 'user-1', activeTenantId: '7',
    tenants: [{ id: '7', name: 'Acme' }, { id: '9', name: 'Beta' }],
  });
});

test('a snapshot without memberships still exposes the active tenant as the sole option', async () => {
  const runtime = createMobileRuntime(ports(fakeStore()));
  const snapshot = await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  assert.deepEqual(snapshot.identity, { userId: 'user-1', activeTenantId: 'tenant-1', tenants: [{ id: 'tenant-1' }] });
});

test('activateTenant without an authorized surface issues no switch request', async () => {
  let switches = 0;
  const runtime = createMobileRuntime(ports(fakeStore(), () => remote({
    switchTenant: async () => { switches += 1; return { credential: { token: 'x', refreshToken: 'y' }, tenant: { id: 't' } }; },
  })));
  const before = runtime.snapshot();

  const snapshot = await runtime.activateTenant('2');
  await runtime.activateTenant('');
  await runtime.activateTenant('   ');

  assert.equal(switches, 0);
  assert.equal(snapshot, before);
});

test('a rejected tenant switch fails closed without keeping the prior lease', async () => {
  const store = fakeStore();
  const runtime = createMobileRuntime(ports(store, () => remote({
    switchTenant: async () => { throw new Error('no membership'); },
  })));
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const priorLease = runtime.scopeLease();
  assert.ok(priorLease);

  const snapshot = await runtime.activateTenant('404');

  assert.deepEqual(snapshot, { surface: 'upgrade-required', deployment: DEPLOYMENT, reason: 'authentication-required' });
  assert.notEqual(runtime.scopeLease(), priorLease);
  assert.equal(runtime.scopeLease(), undefined);
  assert.deepEqual(await store.read(DEPLOYMENT.origin), { token: 'access-1', refreshToken: 'refresh-1' });
});

test('a late tenant verification cannot override a completed later switch', async () => {
  const store = fakeStore();
  const meStarted = deferred<void>();
  const releaseMe = deferred<{ user: { id: string }; tenant: { id: string } }>();
  let meCalls = 0;
  const runtime = createMobileRuntime(ports(store, () => remote({
    me: async (token) => {
      meCalls += 1;
      if (meCalls === 2) { meStarted.resolve(); return releaseMe.promise; }
      const tenantId = token === 'tenant-2-access' ? 'tenant-2' : token === 'tenant-3-access' ? 'tenant-3' : 'tenant-1';
      return { user: { id: 'user-1' }, tenant: { id: tenantId } };
    },
  })));
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });

  const first = runtime.activateTenant('2');
  await meStarted.promise;
  const second = await runtime.activateTenant('3');
  releaseMe.resolve({ user: { id: 'user-1' }, tenant: { id: 'tenant-2' } });
  const firstSnapshot = await first;

  assert.equal(second.identity?.activeTenantId, 'tenant-3');
  assert.equal(runtime.snapshot().identity?.activeTenantId, 'tenant-3');
  assert.equal(firstSnapshot.identity?.activeTenantId, 'tenant-3');
});
```

(c) 既有断言更新（GREEN 步骤实现后 identity 增加 `tenants`，以下 3 处 deepEqual 需同步补字段；放在 Step 3 一起做，此处先记录）：
- 94 行：`identity: { userId: 'user-1', activeTenantId: 'tenant-1' }` → 追加 `, tenants: [{ id: 'tenant-1' }]`
- 172-175 行：`identity: { userId: manual.origin, activeTenantId: manual.origin }` → 追加 `, tenants: [{ id: manual.origin }]`
- 439-441 行：`identity: { userId: 'other-user', activeTenantId: 'other-tenant' }` → 追加 `, tenants: [{ id: 'other-tenant' }]`

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts`
Expected: FAIL——新增测试报 `runtime.activateTenant is not a function`；`tenants` 相关 deepEqual 断言不匹配。

- [ ] **Step 3: 最小实现**

(a) 新建 `packages/mobile-core/src/runtime/scope-lease.ts`：

```ts
import type { ScopeLease } from './types.ts';

/** Package-private scope identity carried by a Runtime-minted lease. Never exported from index.ts. */
export interface LeaseScope {
  deploymentOrigin: string;
  userId: string;
  tenantId: string;
}

/** Runtime-minted, revocable lease. `asScopeLease()` hands out the opaque public view. */
export class RuntimeScopeLease {
  readonly scope: LeaseScope;
  #active = true;
  constructor(scope: LeaseScope) { this.scope = scope; }
  get active(): boolean { return this.#active; }
  revoke(): void { this.#active = false; }
  asScopeLease(): ScopeLease { return this as unknown as ScopeLease; }
}

/** Scope identity extraction for mobile-core Modules (Scoped Vault). Works for revoked leases too — revoke needs the scope to erase. */
export function leaseScopeOf(lease: ScopeLease | undefined): LeaseScope | undefined {
  return lease instanceof RuntimeScopeLease ? lease.scope : undefined;
}

/** Lease validity check; ScopedStore re-validates on every access. */
export function leaseActive(lease: ScopeLease | undefined): boolean {
  return lease instanceof RuntimeScopeLease && lease.active;
}
```

(b) `packages/mobile-core/src/runtime/types.ts`：`RuntimeSnapshot` 之前加 `TenantOption`，改 `identity`、`MobileRuntime`（在 `scopeLease()` 之后加 `activateTenant`）：

```ts
/** Presentation-safe tenant switcher option; ids match activateTenant input. */
export interface TenantOption {
  id: string;
  name?: string;
}

export interface RuntimeSnapshot {
  surface: RuntimeSurface;
  deployment?: Deployment;
  identity?: { userId: string; activeTenantId?: string; tenants?: TenantOption[] };
  reason?: RuntimeReason;
}
```

```ts
  scopeLease(): ScopeLease | undefined;
  /** Atomically switches the Active Tenant: revokes the prior scope, re-issues the credential server-side, re-verifies identity. */
  activateTenant(tenantId: string): Promise<RuntimeSnapshot>;
```

(c) `packages/mobile-core/src/runtime/ports.ts`：`RuntimeRemote` 中 `me(accessToken)` 的返回类型加宽（追加 `memberships?: unknown[]`——运行时行为不变，仅让端口类型匹配 `/auth/me` 既有 wire 字段，否则 Step 3(d)3 的 `me.memberships` 报 TS2339），并在 `refresh(...)` 之后加 `switchTenant`。改后的两行：

```ts
  me(accessToken: string): Promise<{ user: { id: unknown }; tenant?: { id: unknown } | null; memberships?: unknown[] }>;
  switchTenant(input: { tenantId: string; refreshToken: string }): Promise<{ credential: StoredCredential; tenant?: Record<string, unknown> | null }>;
```

(d) `packages/mobile-core/src/runtime/mobile-runtime.ts`：
1. 删除 60-64 行的本地 `class RuntimeScopeLease`，顶部加 `import { RuntimeScopeLease } from './scope-lease.ts';`
2. 新增 memberships 解析辅助（放在 `tenantId` 函数之后）：

```ts
function membershipTenantId(value: unknown): string | undefined {
  const id = typeof value === 'object' && value !== null ? (value as { tenant_id?: unknown }).tenant_id : undefined;
  if (typeof id === 'string' && id.trim() !== '') return id.trim();
  return typeof id === 'number' && Number.isSafeInteger(id) && id > 0 ? String(id) : undefined;
}

function membershipTenantName(value: unknown): string | undefined {
  const name = typeof value === 'object' && value !== null ? (value as { tenant_name?: unknown }).tenant_name : undefined;
  return typeof name === 'string' && name.trim() !== '' ? name.trim() : undefined;
}

function tenantOptions(memberships: unknown, activeTenantId: string): { tenants: Array<{ id: string; name?: string }> } {
  if (!Array.isArray(memberships)) return { tenants: [{ id: activeTenantId }] };
  const tenants: Array<{ id: string; name?: string }> = [];
  for (const membership of memberships) {
    const id = membershipTenantId(membership);
    if (!id || tenants.some((option) => option.id === id)) continue;
    const name = membershipTenantName(membership);
    tenants.push(name ? { id, name } : { id });
  }
  if (!tenants.some((option) => option.id === activeTenantId)) tenants.unshift({ id: activeTenantId });
  return { tenants };
}
```

3. `authenticate` 内 163-164 行的 lease 铸造改为携带 scope，发布快照附带 `tenants`：

```ts
      revocableLease = new RuntimeScopeLease({ deploymentOrigin: deployment.origin, userId: authenticatedUserId, tenantId: activeTenantId });
      lease = revocableLease.asScopeLease();
      return publish({ surface: 'authorized', deployment, identity: { userId: authenticatedUserId, activeTenantId, ...tenantOptions(me.memberships, activeTenantId) } });
```

4. 返回对象中 `signOut,` 之后、`dispose(): void {` 之前加 `activateTenant`（mobile-runtime.ts:278-285 现有顺序为 `scopeLease` → `signOut` → `dispose`）：

```ts
    async activateTenant(tenantId: string): Promise<RuntimeSnapshot> {
      if (typeof tenantId !== 'string' || tenantId.trim() === '') return state;
      const deployment = activeDeployment;
      if (!deployment || state.surface !== 'authorized') return state;
      const requestEpoch = begin(deployment);
      try {
        const credential = await ports.credentialStore.read(deployment.origin);
        if (!current(requestEpoch, deployment)) return state;
        if (!credential) return safe(requestEpoch, deployment, 'authentication-required');
        const switched = await ports.remoteFor(deployment.origin).switchTenant({ tenantId: tenantId.trim(), refreshToken: credential.refreshToken });
        if (!current(requestEpoch, deployment)) return state;
        if (!await persistCredential(requestEpoch, deployment, switched.credential)) return state;
        return await authenticate(requestEpoch, deployment, switched.credential);
      } catch {
        return safe(requestEpoch, deployment, 'authentication-required');
      }
    },
```

5. 按上文 (c) 更新 3 处既有 deepEqual 断言。

(e) `packages/mobile-core/src/index.ts`：类型导出行补 `TenantOption`（加入现有 `export type { Deployment, ... }` 列表）。

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts`
Expected: PASS（24 个既有 + 6 个新增全绿）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/runtime/scope-lease.ts packages/mobile-core/src/runtime/types.ts packages/mobile-core/src/runtime/ports.ts packages/mobile-core/src/runtime/mobile-runtime.ts packages/mobile-core/src/runtime/mobile-runtime.test.ts packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core): activateTenant switches the active tenant through the epoch-guarded runtime"
```

---

### Task 3: Scoped Vault Module（加密 scope 存储 + 轮换 + 撤销）

**Files:**
- Create: `packages/mobile-core/src/vault/ports.ts`
- Create: `packages/mobile-core/src/vault/web-crypto-cipher.ts`
- Create: `packages/mobile-core/src/vault/scoped-vault.ts`
- Create: `packages/mobile-core/src/vault/in-memory-adapters.ts`
- Modify: `packages/mobile-core/src/index.ts`（导出 Vault 公共接口与 in-memory Adapter）
- Test: `packages/mobile-core/src/vault/scoped-vault.test.ts`

**Interfaces:**
- Consumes: Task 2 的 `ScopeLease`/`leaseScopeOf`/`leaseActive`/`LeaseScope`（`../runtime/scope-lease.ts`、`../runtime/types.ts`）。
- Produces（`index.ts` 公开导出；Task 4/5 消费）:
  - `interface CipherPort { seal(key: Uint8Array, plaintext: Uint8Array): Promise<Uint8Array>; open(key: Uint8Array, ciphertext: Uint8Array): Promise<Uint8Array> }`
  - `interface KeyStorePort { readWrappedKey(scopeKey: string): Promise<Uint8Array | undefined>; writeWrappedKey(scopeKey: string, key: Uint8Array): Promise<void>; deleteWrappedKey(scopeKey: string): Promise<void> }`
  - `interface VaultStoragePort { read(key: string): Promise<string | null>; write(key: string, value: string): Promise<void>; delete(key: string): Promise<void> }`
  - `interface ScopedVaultPorts { keyStore: KeyStorePort; storage: VaultStoragePort; cipher: CipherPort; randomBytes?: (size: number) => Uint8Array }`
  - `interface DraftEntry { id: string; body: string; updatedAt: string }`、`interface ScopedDraftRepository { put(input: { id: string; body: string }): Promise<void>; get(id: string): Promise<DraftEntry | undefined>; list(): Promise<DraftEntry[]>; remove(id: string): Promise<void> }`、`interface ScopedStore { drafts: ScopedDraftRepository }`
  - `type VaultRevokeReason = 'tenant-switch' | 'deployment-change' | 'sign-out' | 'dispose'`
  - `interface VaultPolicy { categories: ReadonlyArray<{ category: string; retentionDays: number }> }`
  - `interface ScopedVault { open(scopeLease: ScopeLease): Promise<ScopedStore>; rotate(scopeLease: ScopeLease): Promise<void>; revoke(scopeLease: ScopeLease, reason: VaultRevokeReason): Promise<void>; inspectPolicy(): VaultPolicy }`
  - `function createScopedVault(ports: ScopedVaultPorts): ScopedVault`
  - `function createWebCryptoCipher(): CipherPort`（`web-crypto-cipher.ts`）
  - `function createInMemoryVaultKeyStore(): KeyStorePort & { entries(): ReadonlyMap<string, Uint8Array> }`、`function createInMemoryVaultStorage(): VaultStoragePort & { entries(): ReadonlyMap<string, string> }`（`in-memory-adapters.ts`，测试与场景 Adapter，镜像既有 `createInMemoryCredentialStore` 模式）
  - 包内导出（不在 index.ts）：`function scopeKeyOf(scope: LeaseScope): string`（`scoped-vault.ts`）

- [ ] **Step 1: 写失败测试**

新建 `packages/mobile-core/src/vault/scoped-vault.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import type { ScopeLease } from '../runtime/types.ts';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import { createScopedVault, scopeKeyOf } from './scoped-vault.ts';
import { createWebCryptoCipher } from './web-crypto-cipher.ts';
import { createInMemoryVaultKeyStore, createInMemoryVaultStorage } from './in-memory-adapters.ts';

const SCOPE_A = { deploymentOrigin: 'https://a.example.test', userId: 'user-1', tenantId: '7' };
const SCOPE_B = { deploymentOrigin: 'https://b.example.test', userId: 'user-1', tenantId: '7' };

function lease(scope = SCOPE_A): ScopeLease & { revoke(): void } {
  return new RuntimeScopeLease(scope) as unknown as ScopeLease & { revoke(): void };
}

function vault() {
  const keyStore = createInMemoryVaultKeyStore();
  const storage = createInMemoryVaultStorage();
  return {
    keyStore,
    storage,
    vault: createScopedVault({ keyStore, storage, cipher: createWebCryptoCipher() }),
  };
}

test('drafts are isolated by deployment, user, and tenant scope', async () => {
  const { vault: scoped } = vault();
  const storeA = await scoped.open(lease(SCOPE_A));
  await storeA.drafts.put({ id: 'draft-1', body: 'alpha' });

  const storeOtherDeployment = await scoped.open(lease(SCOPE_B));
  const storeOtherTenant = await scoped.open(lease({ ...SCOPE_A, tenantId: '9' }));
  const storeOtherUser = await scoped.open(lease({ ...SCOPE_A, userId: 'user-2' }));

  assert.equal(await storeOtherDeployment.drafts.get('draft-1'), undefined);
  assert.equal(await storeOtherTenant.drafts.get('draft-1'), undefined);
  assert.equal(await storeOtherUser.drafts.get('draft-1'), undefined);
  assert.equal((await storeA.drafts.get('draft-1'))?.body, 'alpha');
  assert.deepEqual((await storeA.drafts.list()).map((entry) => entry.id), ['draft-1']);
});

test('stored rows never contain plaintext bodies', async () => {
  const { vault: scoped, storage } = vault();
  const store = await scoped.open(lease(SCOPE_A));
  await store.drafts.put({ id: 'draft-1', body: 'plaintext-canary' });

  for (const [key, value] of storage.entries()) {
    assert.doesNotMatch(value, /plaintext-canary/);
    // 索引行只含不透明 id（明文 JSON 属预期）；草稿行必须是 base64 密文
    if (key.endsWith('.drafts.draft-1')) assert.match(value, /^[A-Za-z0-9+/]+={0,2}$/);
  }
});

test('a vault without a valid lease fails closed on open and on every store operation', async () => {
  const { vault: scoped } = vault();
  await assert.rejects(scoped.open({} as ScopeLease), /VAULT_LEASE/);

  const handle = lease(SCOPE_A);
  const revoked = new RuntimeScopeLease(SCOPE_A);
  revoked.revoke();
  await assert.rejects(scoped.open(revoked.asScopeLease()), /VAULT_LEASE/);

  const store = await scoped.open(handle);
  await store.drafts.put({ id: 'draft-1', body: 'secret' });
  handle.revoke();
  await assert.rejects(store.drafts.put({ id: 'draft-2', body: 'x' }), /VAULT_LEASE/);
  await assert.rejects(store.drafts.get('draft-1'), /VAULT_LEASE/);
  await assert.rejects(store.drafts.list(), /VAULT_LEASE/);
  await assert.rejects(store.drafts.remove('draft-1'), /VAULT_LEASE/);
});

test('revoke rotates the wrapped key to unusable randomness and erases rows for the scope', async () => {
  const { vault: scoped, keyStore, storage } = vault();
  const scopeKey = scopeKeyOf(SCOPE_A);
  const store = await scoped.open(lease(SCOPE_A));
  await store.drafts.put({ id: 'draft-1', body: 'tenant-secret' });
  const keyBefore = keyStore.entries().get(scopeKey);
  assert.ok(keyBefore);

  await scoped.revoke(lease(SCOPE_A), 'tenant-switch');

  // 行与索引均被擦除；wrapped key 即使残留也是新随机值，读不出旧密文
  for (const rowKey of storage.entries().keys()) assert.equal(rowKey.startsWith(scopeKey), false, `row ${rowKey} must be erased`);
  const keyAfter = keyStore.entries().get(scopeKey);
  if (keyAfter) assert.notDeepEqual([...keyAfter], [...keyBefore!]);
  const reopened = await scoped.open(lease(SCOPE_A));
  assert.equal(await reopened.drafts.get('draft-1'), undefined);
});

test('rotate re-keys the scope while preserving readable drafts', async () => {
  const { vault: scoped, keyStore, storage } = vault();
  const scopeKey = scopeKeyOf(SCOPE_A);
  const store = await scoped.open(lease(SCOPE_A));
  await store.drafts.put({ id: 'draft-1', body: 'keep-me' });
  const oldKey = keyStore.entries().get(scopeKey)!;

  await scoped.rotate(lease(SCOPE_A));

  const after = await (await scoped.open(lease(SCOPE_A))).drafts.get('draft-1');
  assert.equal(after?.body, 'keep-me');
  const newKey = keyStore.entries().get(scopeKey)!;
  assert.notDeepEqual([...newKey], [...oldKey]);
  // 旧 key 无法解密轮换后的行（真实 GCM 认证失败）
  const rowB64 = storage.entries().get(`${scopeKey}.drafts.draft-1`)!;
  const rowBytes = new Uint8Array(Buffer.from(rowB64, 'base64'));
  await assert.rejects(createWebCryptoCipher().open(oldKey, rowBytes), /VAULT_DECRYPT/);
});

test('tampered ciphertext and wrong keys fail closed through the cipher seam', async () => {
  const { vault: scoped, storage } = vault();
  const store = await scoped.open(lease(SCOPE_A));
  await store.drafts.put({ id: 'draft-1', body: 'integrity' });

  const rowKey = `${scopeKeyOf(SCOPE_A)}.drafts.draft-1`;
  const bytes = Buffer.from(storage.entries().get(rowKey)!, 'base64');
  bytes[bytes.length - 1] ^= 0xff;
  (storage.entries() as Map<string, string>).set(rowKey, Buffer.from(bytes).toString('base64'));

  const reader = await scoped.open(lease(SCOPE_A));
  await assert.rejects(reader.drafts.get('draft-1'), /VAULT_DECRYPT/);
});

test('inspectPolicy exposes the cacheable category and its retention window', () => {
  const { vault: scoped } = vault();
  assert.deepEqual(scoped.inspectPolicy(), { categories: [{ category: 'drafts', retentionDays: 30 }] });
});

test('a failing encryption adapter rejects the write and leaves no row behind', async () => {
  const keyStore = createInMemoryVaultKeyStore();
  const storage = createInMemoryVaultStorage();
  const failingCipher = {
    seal: async () => { throw new Error('cipher adapter down'); },
    open: async () => { throw new Error('cipher adapter down'); },
  };
  const scoped = createScopedVault({ keyStore, storage, cipher: failingCipher });
  const store = await scoped.open(lease(SCOPE_A));

  await assert.rejects(store.drafts.put({ id: 'draft-1', body: 'x' }), /cipher adapter down/);

  assert.equal([...storage.entries().keys()].filter((key) => key.endsWith('.draft-1')).length, 0, 'no row may be written when sealing fails');
});

test('draft ids outside the safe alphabet are rejected before any storage write', async () => {
  const { vault: scoped, storage } = vault();
  const store = await scoped.open(lease(SCOPE_A));
  await assert.rejects(store.drafts.put({ id: 'bad id with spaces', body: 'x' }), /VAULT_ID/);
  await assert.rejects(store.drafts.put({ id: 'x'.repeat(65), body: 'x' }), /VAULT_ID/);
  assert.equal(storage.entries().size, 0, 'neither row nor index may be written for an invalid id');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/mobile-core/src/vault/scoped-vault.test.ts`
Expected: FAIL——`Cannot find module './scoped-vault.ts'`（及 web-crypto-cipher / in-memory-adapters 同类报错）。

- [ ] **Step 3: 最小实现**

(a) `packages/mobile-core/src/vault/ports.ts`：

```ts
/** Symmetric authenticated-encryption seam. `open` must fail on wrong key or tampered ciphertext. */
export interface CipherPort {
  seal(key: Uint8Array, plaintext: Uint8Array): Promise<Uint8Array>;
  open(key: Uint8Array, ciphertext: Uint8Array): Promise<Uint8Array>;
}

/** Persists wrapped per-scope data keys (OS keychain/SecureStore in the app, memory in tests). */
export interface KeyStorePort {
  readWrappedKey(scopeKey: string): Promise<Uint8Array | undefined>;
  writeWrappedKey(scopeKey: string, key: Uint8Array): Promise<void>;
  deleteWrappedKey(scopeKey: string): Promise<void>;
}

/** Persists opaque encrypted rows; keys carry no plaintext semantics (SQLite/SecureStore/memory adapters). */
export interface VaultStoragePort {
  read(key: string): Promise<string | null>;
  write(key: string, value: string): Promise<void>;
  delete(key: string): Promise<void>;
}

export interface ScopedVaultPorts {
  keyStore: KeyStorePort;
  storage: VaultStoragePort;
  cipher: CipherPort;
  /** 32 bytes of entropy per fresh data key; omit only where Web Crypto is available. */
  randomBytes?: (size: number) => Uint8Array;
}
```

(b) `packages/mobile-core/src/vault/web-crypto-cipher.ts`：

```ts
import type { CipherPort } from './ports.ts';

const IV_LENGTH = 12;
const KEY_LENGTH = 32;

/** AES-GCM via Web Crypto. IV is prepended to the sealed bytes; any wrong key or tamper fails the GCM tag. */
export function createWebCryptoCipher(): CipherPort {
  const subtle = globalThis.crypto?.subtle;
  if (!subtle) throw new Error('VAULT_CIPHER_UNAVAILABLE');
  /** Subtle 只接受 ArrayBuffer 视图；把调用方传入的 Uint8Array 拷贝到纯 ArrayBuffer 视图（TS BufferSource 兼容）。 */
  const buffer = (bytes: Uint8Array): Uint8Array<ArrayBuffer> => {
    const copy = new Uint8Array(bytes.byteLength);
    copy.set(bytes);
    return copy;
  };
  const importKey = (key: Uint8Array, usage: KeyUsage[]) => subtle.importKey('raw', buffer(key), 'AES-GCM', false, usage);
  return {
    async seal(key, plaintext) {
      if (key.length !== KEY_LENGTH) throw new Error('VAULT_KEY_LENGTH');
      const iv = globalThis.crypto.getRandomValues(new Uint8Array(IV_LENGTH));
      const sealed = new Uint8Array(await subtle.encrypt({ name: 'AES-GCM', iv }, await importKey(key, ['encrypt']), buffer(plaintext)));
      const out = new Uint8Array(IV_LENGTH + sealed.length);
      out.set(iv);
      out.set(sealed, IV_LENGTH);
      return out;
    },
    async open(key, ciphertext) {
      if (key.length !== KEY_LENGTH) throw new Error('VAULT_KEY_LENGTH');
      if (ciphertext.length <= IV_LENGTH) throw new Error('VAULT_DECRYPT');
      try {
        const iv = buffer(ciphertext.slice(0, IV_LENGTH));
        const body = buffer(ciphertext.slice(IV_LENGTH));
        return new Uint8Array(await subtle.decrypt({ name: 'AES-GCM', iv }, await importKey(key, ['decrypt']), body));
      } catch {
        throw new Error('VAULT_DECRYPT');
      }
    },
  };
}
```

(c) `packages/mobile-core/src/vault/scoped-vault.ts`：

```ts
import type { ScopeLease } from '../runtime/types.ts';
import { leaseActive, leaseScopeOf, type LeaseScope } from '../runtime/scope-lease.ts';
import type { CipherPort, KeyStorePort, ScopedVaultPorts, VaultStoragePort } from './ports.ts';

const KEY_LENGTH = 32;
const DRAFT_ID_PATTERN = /^[A-Za-z0-9._-]{1,64}$/;
const RETENTION_DAYS = 30;

export type VaultRevokeReason = 'tenant-switch' | 'deployment-change' | 'sign-out' | 'dispose';

export interface DraftEntry { id: string; body: string; updatedAt: string }

export interface ScopedDraftRepository {
  put(input: { id: string; body: string }): Promise<void>;
  get(id: string): Promise<DraftEntry | undefined>;
  list(): Promise<DraftEntry[]>;
  remove(id: string): Promise<void>;
}

export interface ScopedStore { drafts: ScopedDraftRepository }

export interface VaultPolicy { categories: ReadonlyArray<{ category: string; retentionDays: number }> }

export interface ScopedVault {
  open(scopeLease: ScopeLease): Promise<ScopedStore>;
  rotate(scopeLease: ScopeLease): Promise<void>;
  revoke(scopeLease: ScopeLease, reason: VaultRevokeReason): Promise<void>;
  inspectPolicy(): VaultPolicy;
}

interface ScopeSession { key: Uint8Array; destroyed: boolean }

export function scopeKeyOf(scope: LeaseScope): string {
  return `weknora.vault.v1.${encodeURIComponent(scope.deploymentOrigin)}.${encodeURIComponent(scope.userId)}.${encodeURIComponent(scope.tenantId)}`;
}

function bytesToBase64(bytes: Uint8Array): string {
  let binary = '';
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary);
}

function base64ToBytes(value: string): Uint8Array {
  const binary = atob(value);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index);
  return bytes;
}

/**
 * Scoped Vault（Spec §9）：唯一拥有 deployment/user/tenant scope 下的加密缓存与撤销。
 * ScopedStore 只暴露 drafts 仓储；每次访问重新校验 lease 有效性，任何没有有效
 * Scope Lease 的访问失败（VAULT_LEASE）。
 */
export function createScopedVault(ports: ScopedVaultPorts): ScopedVault {
  const sessions = new Map<string, ScopeSession>();
  const text = new TextEncoder();

  const randomBytes = (size: number): Uint8Array => {
    const injected = ports.randomBytes?.(size);
    if (injected) {
      if (injected.length !== size) throw new Error('VAULT_ENTROPY');
      return injected;
    }
    if (!globalThis.crypto?.getRandomValues) throw new Error('VAULT_ENTROPY');
    return globalThis.crypto.getRandomValues(new Uint8Array(size));
  };
  const requireSession = async (scopeKey: string): Promise<ScopeSession> => {
    const existing = sessions.get(scopeKey);
    if (existing && !existing.destroyed) return existing;
    const wrapped = await ports.keyStore.readWrappedKey(scopeKey);
    if (wrapped && wrapped.length === KEY_LENGTH) {
      const session: ScopeSession = { key: wrapped, destroyed: false };
      sessions.set(scopeKey, session);
      return session;
    }
    const key = randomBytes(KEY_LENGTH);
    await ports.keyStore.writeWrappedKey(scopeKey, key);
    const session: ScopeSession = { key, destroyed: false };
    sessions.set(scopeKey, session);
    return session;
  };
  const indexKey = (scopeKey: string): string => `${scopeKey}.drafts.index`;
  const rowKey = (scopeKey: string, id: string): string => `${scopeKey}.drafts.${id}`;
  const readIndex = async (scopeKey: string): Promise<string[]> => {
    const raw = await ports.storage.read(indexKey(scopeKey));
    if (raw === null) return [];
    const value: unknown = JSON.parse(raw);
    return Array.isArray(value) && value.every((id) => typeof id === 'string') ? value : [];
  };
  const writeIndex = (scopeKey: string, ids: string[]): Promise<void> => ports.storage.write(indexKey(scopeKey), JSON.stringify(ids));
  const sealRow = async (key: Uint8Array, entry: DraftEntry): Promise<string> => {
    const sealed = await ports.cipher.seal(key, text.encode(JSON.stringify(entry)));
    return bytesToBase64(sealed);
  };
  const openRow = async (key: Uint8Array, raw: string): Promise<DraftEntry> => {
    let bytes: Uint8Array;
    try {
      bytes = base64ToBytes(raw);
    } catch {
      throw new Error('VAULT_DECRYPT');
    }
    const plaintext = new TextDecoder().decode(await ports.cipher.open(key, bytes));
    const value: unknown = JSON.parse(plaintext);
    if (typeof value !== 'object' || value === null) throw new Error('VAULT_DECRYPT');
    const entry = value as Partial<DraftEntry>;
    if (typeof entry.id !== 'string' || typeof entry.body !== 'string' || typeof entry.updatedAt !== 'string') throw new Error('VAULT_DECRYPT');
    return { id: entry.id, body: entry.body, updatedAt: entry.updatedAt };
  };
  const requireScope = (scopeLease: ScopeLease): LeaseScope => {
    const scope = leaseScopeOf(scopeLease);
    if (!scope) throw new Error('VAULT_LEASE');
    return scope;
  };
  const assertAccessible = (scopeLease: ScopeLease, session: ScopeSession): void => {
    if (!leaseActive(scopeLease) || session.destroyed) throw new Error('VAULT_LEASE');
  };

  return {
    async open(scopeLease) {
      if (!leaseActive(scopeLease)) throw new Error('VAULT_LEASE');
      const scope = requireScope(scopeLease);
      const scopeKey = scopeKeyOf(scope);
      const session = await requireSession(scopeKey);
      const drafts: ScopedDraftRepository = {
        async put(input) {
          assertAccessible(scopeLease, session);
          if (!DRAFT_ID_PATTERN.test(input.id)) throw new Error('VAULT_ID');
          const ids = await readIndex(scopeKey);
          const entry: DraftEntry = { id: input.id, body: input.body, updatedAt: new Date().toISOString() };
          await ports.storage.write(rowKey(scopeKey, input.id), await sealRow(session.key, entry));
          if (!ids.includes(input.id)) await writeIndex(scopeKey, [...ids, input.id]);
        },
        async get(id) {
          assertAccessible(scopeLease, session);
          const raw = await ports.storage.read(rowKey(scopeKey, id));
          if (raw === null) return undefined;
          return openRow(session.key, raw);
        },
        async list() {
          assertAccessible(scopeLease, session);
          const entries: DraftEntry[] = [];
          for (const id of await readIndex(scopeKey)) {
            const raw = await ports.storage.read(rowKey(scopeKey, id));
            if (raw !== null) entries.push(await openRow(session.key, raw));
          }
          return entries;
        },
        async remove(id) {
          assertAccessible(scopeLease, session);
          await ports.storage.delete(rowKey(scopeKey, id));
          await writeIndex(scopeKey, (await readIndex(scopeKey)).filter((existing) => existing !== id));
        },
      };
      return { drafts };
    },
    async rotate(scopeLease) {
      if (!leaseActive(scopeLease)) throw new Error('VAULT_LEASE');
      const scopeKey = scopeKeyOf(requireScope(scopeLease));
      const session = sessions.get(scopeKey);
      const oldKey = session && !session.destroyed ? session.key : await ports.keyStore.readWrappedKey(scopeKey);
      if (!oldKey) return;
      const newKey = randomBytes(KEY_LENGTH);
      for (const id of await readIndex(scopeKey)) {
        const raw = await ports.storage.read(rowKey(scopeKey, id));
        if (raw === null) continue;
        const entry = await openRow(oldKey, raw);
        await ports.storage.write(rowKey(scopeKey, id), await sealRow(newKey, entry));
      }
      await ports.keyStore.writeWrappedKey(scopeKey, newKey);
      if (session && !session.destroyed) session.key = newKey;
    },
    async revoke(scopeLease, _reason) {
      const scope = requireScope(scopeLease);
      const scopeKey = scopeKeyOf(scope);
      const session = sessions.get(scopeKey);
      if (session) session.destroyed = true;
      sessions.delete(scopeKey);
      // 先把 wrapped key 覆写成全新随机值：即使后续删行/删 key 部分失败，旧密文也不可再解。
      await ports.keyStore.writeWrappedKey(scopeKey, randomBytes(KEY_LENGTH));
      const ids = await readIndex(scopeKey);
      for (const id of ids) await ports.storage.delete(rowKey(scopeKey, id));
      await ports.storage.delete(indexKey(scopeKey));
      await ports.keyStore.deleteWrappedKey(scopeKey);
    },
    inspectPolicy() {
      return { categories: [{ category: 'drafts', retentionDays: RETENTION_DAYS }] };
    },
  };
}
```

(d) `packages/mobile-core/src/vault/in-memory-adapters.ts`：

```ts
import type { KeyStorePort, VaultStoragePort } from './ports.ts';

/** In-memory KeyStorePort for Module scenario tests (mirrors createInMemoryCredentialStore). */
export function createInMemoryVaultKeyStore(): KeyStorePort & { entries(): ReadonlyMap<string, Uint8Array> } {
  const values = new Map<string, Uint8Array>();
  return {
    entries: () => values,
    async readWrappedKey(scopeKey) { const value = values.get(scopeKey); return value && new Uint8Array(value); },
    async writeWrappedKey(scopeKey, key) { values.set(scopeKey, new Uint8Array(key)); },
    async deleteWrappedKey(scopeKey) { values.delete(scopeKey); },
  };
}

/** In-memory VaultStoragePort for Module scenario tests. */
export function createInMemoryVaultStorage(): VaultStoragePort & { entries(): ReadonlyMap<string, string> } {
  const values = new Map<string, string>();
  return {
    entries: () => values,
    async read(key) { return values.get(key) ?? null; },
    async write(key, value) { values.set(key, value); },
    async delete(key) { values.delete(key); },
  };
}
```

(e) `packages/mobile-core/src/index.ts` 追加导出：

```ts
export { createScopedVault, scopeKeyOf } from './vault/scoped-vault.ts';
export { createWebCryptoCipher } from './vault/web-crypto-cipher.ts';
export { createInMemoryVaultKeyStore, createInMemoryVaultStorage } from './vault/in-memory-adapters.ts';
export type { CipherPort, KeyStorePort, ScopedVaultPorts, VaultStoragePort } from './vault/ports.ts';
export type { DraftEntry, ScopedDraftRepository, ScopedStore, ScopedVault, VaultPolicy, VaultRevokeReason } from './vault/scoped-vault.ts';
```

（`scopeKeyOf` 供后续 Task/Issue 场景断言使用，属包内事实的标准导出；`RuntimeScopeLease`/`leaseScopeOf`/`leaseActive` 仍**不**导出。）

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/mobile-core/src/vault/scoped-vault.test.ts`
Expected: PASS（9 个测试全绿；Node 22 提供 WebCrypto，已实测错误 key/篡改密文均 `OperationError` → 包装为 `VAULT_DECRYPT`）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/vault/ packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core): scoped vault with per-scope encryption, rotation, and fail-closed revocation"
```

---

### Task 4: Runtime ↔ Vault 失效接线（切换/退出/换部署的 key 失效自动化证据）

**Files:**
- Modify: `packages/mobile-core/src/runtime/ports.ts:57-67`（`MobileRuntimePorts` 增加 `scopedVault?`）
- Modify: `packages/mobile-core/src/runtime/mobile-runtime.ts:66-109`（vault 串行队列 + `revoke/begin/reserve` 携带原因）、`mobile-runtime.ts:170-179`（`signOut`）、`mobile-runtime.ts:201-212`（`signIn`）、`mobile-runtime.ts` Task 2 新增的 `activateTenant`
- Test: `packages/mobile-core/src/runtime/runtime-vault.test.ts`（新建）

**Interfaces:**
- Consumes: Task 2 的 `RuntimeScopeLease.asScopeLease()`/`begin/reserve/revoke`；Task 3 的 `ScopedVault.revoke(scopeLease, reason)` 与 `VaultRevokeReason`。
- Produces: `MobileRuntimePorts.scopedVault?: ScopedVault`（组合根注入；缺省时 Runtime 行为与 T01 完全一致）。运行时保证：任何 scope 变化先同步撤销旧 lease（所有已开 ScopedStore 立即 fail closed），vault 擦除在串行队列 `vaultTail` 上异步执行；`signIn`/`signOut`/`activateTenant` 返回前 `await vaultTail`（供测试确定性断言）。

- [ ] **Step 1: 写失败测试**

新建 `packages/mobile-core/src/runtime/runtime-vault.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createMobileRuntime } from './mobile-runtime.ts';
import { createScopedVault } from '../vault/scoped-vault.ts';
import { createWebCryptoCipher } from '../vault/web-crypto-cipher.ts';
import { createInMemoryVaultKeyStore, createInMemoryVaultStorage } from '../vault/in-memory-adapters.ts';
import type { CredentialStore, RuntimeRemote, StoredCredential } from './ports.ts';
import type { DeploymentInput } from './types.ts';

const DEPLOYMENT: DeploymentInput = { origin: 'https://weknora.example.test', label: 'Test Deployment' };
const OTHER: DeploymentInput = { origin: 'https://other.example.test', label: 'Other' };
const CAPABILITIES = { protocol_minimum: 2, protocol_maximum: 3 };

function fakeStore(initial: Record<string, StoredCredential | undefined> = {}): CredentialStore {
  const values = new Map(Object.entries(initial));
  return {
    async read(deployment) { return values.get(deployment); },
    async write(deployment, credential) { values.set(deployment, { ...credential }); },
    async clear(deployment) { values.delete(deployment); },
  };
}

function switchRemote(): RuntimeRemote {
  return {
    passwordLogin: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    me: async (token) => ({ user: { id: 'user-1' }, tenant: { id: token === 'tenant-2-access' ? 'tenant-2' : token === 'other-access' ? 'other-tenant' : 'tenant-1' } }),
    deploymentCapabilities: async () => CAPABILITIES,
    oidcUrl: async () => ({ authorizationUrl: 'https://idp.example.test/authorize', state: 'state-1' }),
    oidcExchange: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    oidcNativeExchange: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    refresh: async () => ({ access_token: 'access-1', refresh_token: 'refresh-1' }),
    switchTenant: async (input) => ({ credential: { token: `tenant-${input.tenantId}-access`, refreshToken: `refresh-${input.tenantId}` }, tenant: { id: `tenant-${input.tenantId}` } }),
  };
}

function vaultRuntime() {
  const keyStore = createInMemoryVaultKeyStore();
  const storage = createInMemoryVaultStorage();
  const vault = createScopedVault({ keyStore, storage, cipher: createWebCryptoCipher() });
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => switchRemote(),
    clientVersion: 3,
    scopedVault: vault,
  });
  return { runtime, vault, keyStore, storage };
}

test('switching tenant revokes the old scope: store ops fail and persisted rows are erased', async () => {
  const { runtime, vault, keyStore, storage } = vaultRuntime();
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const firstLease = runtime.scopeLease();
  assert.ok(firstLease);
  const store = await vault.open(firstLease);
  await store.drafts.put({ id: 'draft-1', body: 'tenant-one-draft' });

  const snapshot = await runtime.activateTenant('2');

  assert.equal(snapshot.identity?.activeTenantId, 'tenant-2');
  assert.notEqual(runtime.scopeLease(), firstLease);
  await assert.rejects(store.drafts.get('draft-1'), /VAULT_LEASE/);
  // 旧租户 scope 的 wrapped key 与行必须被擦除（AC2「切换」路径的直接断言，须先于下方 reopen 再建 key）
  assert.equal(keyStore.entries().size, 0, 'the prior tenant wrapped key must be erased');
  assert.equal(storage.entries().size, 0, 'the prior tenant rows must be erased');
  const reopened = await vault.open(runtime.scopeLease()!);
  assert.equal(await reopened.drafts.get('draft-1'), undefined);
  assert.deepEqual((await reopened.drafts.list()), []);
});

test('sign-out revokes the vault scope and erases wrapped keys and rows', async () => {
  const { runtime, vault, keyStore, storage } = vaultRuntime();
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const store = await vault.open(runtime.scopeLease()!);
  await store.drafts.put({ id: 'draft-1', body: 'keep-out' });

  await runtime.signOut();

  await assert.rejects(store.drafts.put({ id: 'draft-2', body: 'x' }), /VAULT_LEASE/);
  assert.equal(keyStore.entries().size, 0);
  assert.equal(storage.entries().size, 0);
});

test('a deployment change revokes the prior deployment vault scope', async () => {
  const { runtime, vault, keyStore } = vaultRuntime();
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const store = await vault.open(runtime.scopeLease()!);
  await store.drafts.put({ id: 'draft-1', body: 'a' });

  await runtime.signIn({ deployment: OTHER, email: 'member@example.test', password: 'password' });

  await assert.rejects(store.drafts.get('draft-1'), /VAULT_LEASE/);
  assert.equal(keyStore.entries().size, 0, 'the first deployment scope key must be erased');
  const other = await vault.open(runtime.scopeLease()!);
  assert.equal(await other.drafts.get('draft-1'), undefined);
});

test('a failed vault erase keeps access closed and does not reject the runtime call', async () => {
  // keychain 写一次成功（open 建key），此后全部失败：revoke 第一阶段覆写新随机 key 即失败
  const backing = createInMemoryVaultKeyStore();
  let writes = 0;
  const flaky = {
    ...backing,
    async writeWrappedKey(scopeKey: string, key: Uint8Array) {
      writes += 1;
      if (writes > 1) throw new Error('keychain down');
      await backing.writeWrappedKey(scopeKey, key);
    },
  };
  const vault = createScopedVault({ keyStore: flaky, storage: createInMemoryVaultStorage(), cipher: createWebCryptoCipher() });
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => switchRemote(),
    clientVersion: 3,
    scopedVault: vault,
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const store = await vault.open(runtime.scopeLease()!);
  await store.drafts.put({ id: 'draft-1', body: 'x' });

  const snapshot = await runtime.activateTenant('2');

  assert.equal(snapshot.identity?.activeTenantId, 'tenant-2');
  await assert.rejects(store.drafts.get('draft-1'), /VAULT_LEASE/);
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/mobile-core/src/runtime/runtime-vault.test.ts`
Expected: FAIL——RED 来自测试 1/2/3 的擦除断言 `keyStore.entries().size === 0` / `storage.entries().size === 0`：Task 4 实现前无人调用 `vault.revoke`，scope key 与行原样留存。注意：`/VAULT_LEASE/` 拒绝断言在实现前也会通过（Task 2 `activateTenant` 的 `begin()`→`revoke()` 已同步撤销 lease），不构成 RED；测试 4（failed erase）在实现前同样通过，其价值在回归防线。

- [ ] **Step 3: 最小实现**

`packages/mobile-core/src/runtime/mobile-runtime.ts`：

1. `mobile-runtime.ts` 顶部加 `import type { ScopedVault, VaultRevokeReason } from '../vault/scoped-vault.ts';`
2. `packages/mobile-core/src/runtime/ports.ts` 顶部加 `import type { ScopedVault } from '../vault/scoped-vault.ts';`（type-only 循环引用合法），`MobileRuntimePorts` 增加可选端口（放在 `lifecycle?` 之后）：

```ts
  /** Scoped Vault Module; the Runtime revokes its scopes on every scope change. Optional so T01-only compositions stay valid. */
  scopedVault?: ScopedVault;
```

3. Runtime 工厂内（`let credentialMutation ...` 声明区之后）加串行 vault 队列，并改造 `revoke/begin/reserve`：

```ts
  let vaultTail: Promise<void> = Promise.resolve();
  const queueVaultRevoke = (reason: VaultRevokeReason): void => {
    const expired = revocableLease;
    const vault = ports.scopedVault;
    if (!expired || !vault) return;
    const attempt = (): Promise<void> => vault.revoke(expired.asScopeLease(), reason);
    const next = vaultTail.then(attempt, attempt);
    vaultTail = next.then(() => {}, () => {});
  };
```

```ts
  const revoke = (vaultReason: VaultRevokeReason): void => {
    queueVaultRevoke(vaultReason);
    revocableLease?.revoke();
    revocableLease = undefined;
    lease = undefined;
  };
  const begin = (deployment: Deployment, vaultReason: VaultRevokeReason = 'deployment-change'): number => {
    epoch += 1;
    revoke(vaultReason);
    activeDeployment = deployment;
    return epoch;
  };
  const reserve = (vaultReason: VaultRevokeReason = 'deployment-change'): number => {
    epoch += 1;
    revoke(vaultReason);
    activeDeployment = undefined;
    return epoch;
  };
```

4. `signOut` 内 `reserve()` 改为 `reserve('sign-out')`，并在函数体末尾（`Promise.all` await 之后）加 `await vaultTail;`
5. Task 2 新增的 `activateTenant` 中 `const requestEpoch = begin(deployment);` 改为 `const requestEpoch = begin(deployment, 'tenant-switch');`
6. `signIn`：整体包上 `try { ...现有体... } finally { await vaultTail; }`（现有体内部多个 `return` 不动，finally 统一等待擦除完成）
7. `activateTenant`（Task 2 新增）：同样包 `try/finally { await vaultTail; }`（`catch` 分支保持 `return safe(...)`）
8. `dispose()`：`epoch += 1; revoke('dispose'); ...`（同步撤销 lease；擦除为 fire-and-forget，组合根 dispose 后进程即弃）
9. `boot`/`completeOidc`/`beginOidc` 不等待 `vaultTail`（其 scope 变化同样会排队擦除；无测试依赖其完成时机，避免无谓延迟——已在计划中记录该取舍）

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/mobile-core/src/runtime/runtime-vault.test.ts packages/mobile-core/src/runtime/mobile-runtime.test.ts`
Expected: PASS（新文件 4 个测试 + 既有 30 个全绿，证明接线无回归）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/runtime/ports.ts packages/mobile-core/src/runtime/mobile-runtime.ts packages/mobile-core/src/runtime/runtime-vault.test.ts
git commit -m "feat(mobile-core): runtime revokes scoped vault keys on tenant switch, sign-out, and deployment change"
```

---

### Task 5: apps/mobile 租户切换 UI 与原生 Vault Adapter 组装

**Files:**
- Create: `apps/mobile/src/adapters/vault-adapters.ts`
- Create: `apps/mobile/src/adapters/vault-adapters.test.ts`
- Modify: `apps/mobile/src/screens/AuthorizedLandingScreen.tsx:1-21`
- Modify: `apps/mobile/src/composition.ts:25-46`（`createNativeMobileRuntime` 注入 `scopedVault`）、`composition.ts:48-64`（`RuntimeSurfaceProps`/`RuntimeSurface` 增加 `onActivateTenant` 与租户选项投影）、`composition.ts:81-94`（`MobileApp` 回调）
- Test: `apps/mobile/src/app-smoke.test.tsx`（130-155 行三处 `RuntimeSurface` 调用补 `onActivateTenant`；末尾追加两个测试）

**Interfaces:**
- Consumes: Task 2 的 `snapshot.identity.tenants`/`MobileRuntime.activateTenant`；Task 3 的 `KeyStorePort`/`VaultStoragePort`/`createScopedVault`/`createWebCryptoCipher`；既有 `SecureStorePort`（`apps/mobile/src/adapters/secure-store.ts:5-9`）。
- Produces:
  - `createSecureVaultKeyStore(store: SecureStorePort): KeyStorePort`、`createSecureVaultStorage(store: SecureStorePort): VaultStoragePort`（`vault-adapters.ts`；scope key/行 key 直接作为 SecureStore key，值 base64）
  - `AuthorizedLandingScreenProps { deploymentLabel: string; userId: string; tenantId: string; tenants: Array<{ id: string; name?: string; active: boolean }>; onSignOut(): Promise<void>; onActivateTenant(tenantId: string): Promise<void> }`
  - `RuntimeSurfaceProps` 增加 `onActivateTenant: (tenantId: string) => Promise<void>`

- [ ] **Step 1: 写失败测试**

(a) 新建 `apps/mobile/src/adapters/vault-adapters.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createSecureVaultKeyStore, createSecureVaultStorage } from './vault-adapters.ts';
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

test('secure vault key store round-trips wrapped keys as base64 under the scope key', async () => {
  const store = secureStore();
  const keys = createSecureVaultKeyStore(store);
  const key = new Uint8Array([1, 2, 3, 250, 255]);

  await keys.writeWrappedKey('scope-1', key);

  assert.match(store.values.get('scope-1')!, /^[A-Za-z0-9+/]+={0,2}$/);
  assert.deepEqual(await keys.readWrappedKey('scope-1'), key);
  await keys.deleteWrappedKey('scope-1');
  assert.equal(await keys.readWrappedKey('scope-1'), undefined);
  assert.equal(store.values.has('scope-1'), false);
});

test('secure vault storage delegates opaque rows without transformation', async () => {
  const store = secureStore();
  const storage = createSecureVaultStorage(store);
  await storage.write('row-1', 'opaque-ciphertext');
  assert.equal(await storage.read('row-1'), 'opaque-ciphertext');
  assert.equal(await storage.read('missing'), null);
  await storage.delete('row-1');
  assert.equal(await storage.read('row-1'), null);
});
```

(b) `apps/mobile/src/app-smoke.test.tsx` 130-155 行的三个 `RuntimeSurface({...})` 调用（`safe`、`invalidAuthorized`、`authorized`）各补一个 prop：`onActivateTenant: async () => {},`。

(c) 文件末尾追加：

```tsx
test('authorized landing switches tenants only through non-active options', async () => {
  const { AuthorizedLandingScreen } = await import('./screens/AuthorizedLandingScreen.tsx');
  hooks().__reset();
  const activated: string[] = [];
  const element = render(AuthorizedLandingScreen, {
    deploymentLabel: 'WeKnora', userId: 'member-1', tenantId: '7',
    tenants: [{ id: '7', name: 'Acme', active: true }, { id: '9', name: 'Beta', active: false }],
    onSignOut: async () => {},
    onActivateTenant: async (id: string) => { activated.push(id); },
  });

  const texts = descendants(element).filter(({ type }) => type === 'Text').flatMap(({ props }) => props.children);
  assert.equal(texts.includes('Acme (active)'), true);
  const buttons = descendants(element).filter(({ type }) => type === 'Button' });
  const switchButton = buttons.find(({ props }) => props.title === 'Switch to Beta');
  assert.ok(switchButton, 'inactive tenant must render a switch button');
  (switchButton.props.onPress as () => void)();
  assert.deepEqual(activated, ['9']);
});

test('RuntimeSurface derives tenant options from the snapshot identity', async () => {
  const { RuntimeSurface } = await import('./composition.ts');
  const surface = RuntimeSurface({
    snapshot: {
      surface: 'authorized',
      deployment: { origin: 'https://weknora.example.test', label: 'WeKnora' },
      identity: { userId: 'member-1', activeTenantId: '7', tenants: [{ id: '7', name: 'Acme' }, { id: '9', name: 'Beta' }] },
    },
    onSignIn: async () => {}, onBeginOidc: async () => {}, onSignOut: async () => {},
    onActivateTenant: async () => {},
  });

  assert.equal(surface.type.name, 'AuthorizedLandingScreen');
  assert.deepEqual(surface.props.tenants, [
    { id: '7', name: 'Acme', active: true },
    { id: '9', name: 'Beta', active: false },
  ]);
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL——vault-adapters 测试 `Cannot find module './vault-adapters.ts'`；AuthorizedLandingScreen 测试因缺少 `tenants`/`onActivateTenant` props 报断言失败（`switchButton` undefined）；RuntimeSurface 测试报 props 不匹配。`RuntimeSurface` 三处既有调用因新增必填 prop `onActivateTenant` 亦失败。

- [ ] **Step 3: 最小实现**

(a) `apps/mobile/src/adapters/vault-adapters.ts`：

```ts
import type { KeyStorePort, VaultStoragePort } from '@weknora/mobile-core';
import type { SecureStorePort } from './secure-store.ts';

function bytesToBase64(bytes: Uint8Array): string {
  let binary = '';
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary);
}

function base64ToBytes(value: string): Uint8Array {
  const binary = atob(value);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index);
  return bytes;
}

/** OS-backed wrapped-key storage; the vault scope key doubles as the SecureStore key. */
export function createSecureVaultKeyStore(store: SecureStorePort): KeyStorePort {
  return {
    async readWrappedKey(scopeKey) { const raw = await store.getItemAsync(scopeKey); return raw === null ? undefined : base64ToBytes(raw); },
    async writeWrappedKey(scopeKey, key) { await store.setItemAsync(scopeKey, bytesToBase64(key)); },
    async deleteWrappedKey(scopeKey) { await store.deleteItemAsync(scopeKey); },
  };
}

/** OS-backed encrypted-row storage; values are opaque base64 ciphertext from the vault. */
export function createSecureVaultStorage(store: SecureStorePort): VaultStoragePort {
  return {
    async read(key) { return store.getItemAsync(key); },
    async write(key, value) { await store.setItemAsync(key, value); },
    async delete(key) { await store.deleteItemAsync(key); },
  };
}
```

(b) `apps/mobile/src/screens/AuthorizedLandingScreen.tsx` 整体替换为：

```tsx
import { Button, Text, View } from 'react-native';

export interface TenantOptionView {
  id: string;
  name?: string;
  active: boolean;
}

export interface AuthorizedLandingScreenProps {
  deploymentLabel: string;
  userId: string;
  tenantId: string;
  tenants: TenantOptionView[];
  onSignOut(): Promise<void>;
  onActivateTenant(tenantId: string): Promise<void>;
}

/** Placeholder handoff point for the Task Office Module; tenant switching stays a pure Runtime callback. */
export function AuthorizedLandingScreen({ deploymentLabel, userId, tenantId, tenants, onSignOut, onActivateTenant }: AuthorizedLandingScreenProps) {
  return (
    <View>
      <Text>WeKnora Task Office</Text>
      <Text>{deploymentLabel}</Text>
      <Text>{`Signed in as ${userId} for tenant ${tenantId}`}</Text>
      <Text>The Task Office will appear here.</Text>
      {tenants.map((tenant) => tenant.active
        ? <Text key={tenant.id}>{`${tenant.name ?? tenant.id} (active)`}</Text>
        : <Button key={tenant.id} title={`Switch to ${tenant.name ?? tenant.id}`} onPress={() => { void onActivateTenant(tenant.id); }} />)}
      <Button title="Sign out" onPress={() => { void onSignOut(); }} />
    </View>
  );
}
```

(c) `apps/mobile/src/composition.ts`：
1. import 区加：

```ts
import { createScopedVault, createWebCryptoCipher } from '@weknora/mobile-core';
import type { ScopedVault } from '@weknora/mobile-core';
import { createSecureVaultKeyStore, createSecureVaultStorage } from './adapters/vault-adapters.ts';
```

2. `createNativeMobileRuntime` 之前加：

```ts
/** Wires the Scoped Vault only where Web Crypto exists. Native crypto seam completes in T10 (#40); absence must not break login. */
function createNativeScopedVaultIfAvailable(): ScopedVault | undefined {
  try {
    const secure = require('expo-secure-store') as SecureStorePort;
    return createScopedVault({
      keyStore: createSecureVaultKeyStore(secure),
      storage: createSecureVaultStorage(secure),
      cipher: createWebCryptoCipher(),
    });
  } catch {
    return undefined;
  }
}
```

（composition.ts:9 已有 `import { createNativeSecurePendingOidcStore } from './adapters/secure-store.ts';` 值导入，此处另起一行加 `import type { SecureStorePort } from './adapters/secure-store.ts';`）

3. `createMobileRuntime({...})` 的 ports 里加 `scopedVault: createNativeScopedVaultIfAvailable(),`
4. `RuntimeSurfaceProps` 加 `onActivateTenant: (tenantId: string) => Promise<void>;`；`RuntimeSurface` 的 authorized 分支改为：

```ts
  if (snapshot.surface === 'authorized' && snapshot.deployment && snapshot.identity?.userId && snapshot.identity.activeTenantId) {
    const activeTenantId = snapshot.identity.activeTenantId;
    const tenants = (snapshot.identity.tenants ?? [{ id: activeTenantId }]).map((tenant) => ({ ...tenant, active: tenant.id === activeTenantId }));
    return createElement(AuthorizedLandingScreen, { deploymentLabel: snapshot.deployment.label, userId: snapshot.identity.userId, tenantId: activeTenantId, tenants, onSignOut, onActivateTenant });
  }
```

5. `MobileApp` 的 `RuntimeSurface` props 加：`onActivateTenant: async (tenantId) => { await activeRuntime.activateTenant(tenantId); },`

- [ ] **Step 4: 运行确认通过**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: PASS（原 11 个 + 新增 4 个测试全绿；typecheck 0 错误——typecheck 同时覆盖 mobile-core 源的传递编译）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/adapters/vault-adapters.ts apps/mobile/src/adapters/vault-adapters.test.ts apps/mobile/src/screens/AuthorizedLandingScreen.tsx apps/mobile/src/composition.ts apps/mobile/src/app-smoke.test.tsx
git commit -m "feat(mobile): active-tenant switcher UI and secure vault composition"
```

---

### Task 6: 真实 HTTP 集成证据扩展（租户切换，AC3）

**Files:**
- Modify: `apps/mobile/src/runtime-integration-smoke.ts:9-21`（Config/Evidence 类型）、`runtime-integration-smoke.ts:31-55`（环境解析）、`runtime-integration-smoke.ts:62-96`（`runMobileRuntimeIntegration`）、`runtime-integration-smoke.ts:98-101`（evidence 输出不变，字段自然携带）
- Modify: `packages/api-client/src/mobile/runtime.integration.test.ts:18-69`（主测试断言 + evidence fixture 补字段；追加配置校验测试）

**Interfaces:**
- Consumes: Task 2 的 `MobileRuntime.activateTenant`；既有 opt-in 环境变量契约 `WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/_EMAIL/_PASSWORD`（`apps/mobile/src/runtime-integration-smoke.ts:32-55`）。
- Produces:
  - `MobileRuntimeIntegrationConfig` enabled 分支增加 `switchTenantId?: string`（来自 `WEKNORA_MOBILE_TEST_SWITCH_TENANT_ID`，可选；非空时必须是正整数字符串，否则 `disposition: 'invalid'`）
  - `disallowedDeploymentHost(hostname: string): string | undefined`（`runtime-integration-smoke.ts` 模块级辅助：拒绝 localhost/环回/私网/链路本地/保留主机；真实请求只允许公网 HTTPS 主机）
  - `MobileRuntimeIntegrationEvidence` 增加 `tenantSwitch: 'skipped' | 'switched' | 'switch-failed'`（evidence 仍不含任何凭据字段）

- [ ] **Step 1: 写失败测试**

`packages/api-client/src/mobile/runtime.integration.test.ts`：

(a) 主测试（18-32 行）在 `assert.equal(evidence.identity, 'present', ...)` 之后追加：

```ts
  assert.equal(evidence.tenantSwitch, config.switchTenantId ? 'switched' : 'skipped');
```

(b) 48-69 行 evidence 回显测试的 fixture 与 `JSON.parse` 期望各补一个字段 `tenantSwitch: 'skipped'`：

```ts
  emitMobileRuntimeIntegrationEvidence({
    deploymentOrigin: 'https://deployment.example',
    clientProtocol: 3,
    capabilityMode: 'incompatible',
    identity: 'absent',
    outcome: 'not-authorized',
    tenantSwitch: 'skipped',
    commandTimestamp: '2026-09-21T00:00:00.000Z',
  }, (record) => emitted.push(record));
```

（`assert.deepEqual(JSON.parse(emitted[0]!), {...})` 的期望对象同步加 `tenantSwitch: 'skipped'`。）

(c) 文件末尾追加：

```ts
test('integration config rejects a non-numeric switch tenant id as invalid rather than skippable', () => {
  const config = mobileRuntimeIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://deployment.example',
    WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
    WEKNORA_MOBILE_TEST_PASSWORD: 'short-lived-secret',
    WEKNORA_MOBILE_TEST_SWITCH_TENANT_ID: 'abc',
  });

  assert.deepEqual(config, {
    enabled: false,
    disposition: 'invalid',
    reason: 'WEKNORA_MOBILE_TEST_SWITCH_TENANT_ID must be a positive integer tenant id',
  });
});

test('integration config rejects loopback, private, and reserved deployment hosts', () => {
  for (const url of ['https://localhost', 'https://sub.localhost', 'https://127.0.0.1', 'https://10.0.0.2', 'https://172.16.0.9', 'https://192.168.1.10', 'https://169.254.1.1', 'https://0.0.0.0', 'https://240.0.0.1']) {
    const config = mobileRuntimeIntegrationConfig({
      WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: url,
      WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
      WEKNORA_MOBILE_TEST_PASSWORD: 'short-lived-secret',
    });

    assert.equal(config.enabled, false, `${url} must not enable a real HTTP run`);
    assert.equal(config.disposition, 'invalid', `${url} is invalid, not skippable`);
  }
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/api-client/src/mobile/runtime.integration.test.ts`
Expected: FAIL——RED 证据来自新增配置校验测试：当前 `mobileRuntimeIntegrationConfig` 对 `WEKNORA_MOBILE_TEST_SWITCH_TENANT_ID: 'abc'` 返回 `{ enabled: true, ... }`（无 invalid 分支），deepEqual 失败。主测试在无环境变量时以 `t.skip` 结束（blocked-env 属预期，不算 RED）；evidence 回显 fixture 补字段后仍通过（实现由 Step 3 落地）。

- [ ] **Step 3: 最小实现**

`apps/mobile/src/runtime-integration-smoke.ts`：

1. 类型：

```ts
export type MobileRuntimeIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string; switchTenantId?: string }
  | { enabled: false; disposition: 'skip'; reason: string }
  | { enabled: false; disposition: 'invalid'; reason: string };

export interface MobileRuntimeIntegrationEvidence {
  deploymentOrigin: string;
  clientProtocol: number;
  capabilityMode: 'compatible' | 'incompatible' | 'unknown';
  identity: 'present' | 'absent';
  outcome: 'authorized' | 'not-authorized';
  tenantSwitch: 'skipped' | 'switched' | 'switch-failed';
  commandTimestamp: string;
}
```

2. `mobileRuntimeIntegrationConfig` 之前新增模块级辅助函数（真实请求只允许发往公网 HTTPS 主机——localhost、环回、私网、链路本地与保留地址一律拒绝，防测试配置被指向内网）：

```ts
/** 部署 URL 主机防线：仅允许公网主机，拒绝 localhost、环回、私网、链路本地与保留地址。 */
function disallowedDeploymentHost(hostname: string): string | undefined {
  const host = hostname.toLowerCase().replace(/^\[|\]$/g, '');
  if (host === 'localhost' || host.endsWith('.localhost')) return 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must not target localhost';
  if (host === '::1' || host === '0.0.0.0') return 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must not target a loopback or wildcard address';
  const match = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/.exec(host);
  if (match) {
    const a = Number(match[1]);
    const b = Number(match[2]);
    if (a === 127 || a === 0 || a >= 240) return 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must not target loopback or reserved addresses';
    if (a === 10) return 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must not target private addresses';
    if (a === 172 && b >= 16 && b <= 31) return 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must not target private addresses';
    if (a === 192 && b === 168) return 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must not target private addresses';
    if (a === 169 && b === 254) return 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must not target link-local addresses';
  }
  return undefined;
}
```

`mobileRuntimeIntegrationConfig` 在 URL 校验通过后（`parsed` 可用之后）先加主机防线，再加切换租户 id 校验：

```ts
  const hostRejection = disallowedDeploymentHost(parsed.hostname);
  if (hostRejection) return { enabled: false, disposition: 'invalid', reason: hostRejection };

  const switchTenantId = env.WEKNORA_MOBILE_TEST_SWITCH_TENANT_ID?.trim();
  if (switchTenantId !== undefined && switchTenantId !== '' && !/^\d+$/.test(switchTenantId)) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_SWITCH_TENANT_ID must be a positive integer tenant id' };
  }
  if (/^0+$/.test(switchTenantId ?? '')) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_SWITCH_TENANT_ID must be a positive integer tenant id' };
  }
```

返回改为 `return { enabled: true, deploymentOrigin: parsed.origin, email, password, ...(switchTenantId ? { switchTenantId } : {}) };`

3. `runMobileRuntimeIntegration` 末尾改为（`signIn` 之后追加切换段）：

```ts
  const snapshot = await runtime.signIn({
    deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' },
    email: config.email,
    password: config.password,
  });
  const identityPresent = Boolean(snapshot.identity?.userId && snapshot.identity.activeTenantId);

  let tenantSwitch: MobileRuntimeIntegrationEvidence['tenantSwitch'] = 'skipped';
  if (config.switchTenantId && snapshot.surface === 'authorized') {
    const leaseBefore = runtime.scopeLease();
    const switched = await runtime.activateTenant(config.switchTenantId);
    const switchIdentityPresent = Boolean(switched.identity?.userId && switched.identity.activeTenantId && switched.identity.activeTenantId !== snapshot.identity?.activeTenantId);
    tenantSwitch = switched.surface === 'authorized' && switchIdentityPresent && runtime.scopeLease() !== undefined && runtime.scopeLease() !== leaseBefore
      ? 'switched'
      : 'switch-failed';
  }

  return {
    deploymentOrigin: config.deploymentOrigin,
    clientProtocol: CLIENT_PROTOCOL,
    capabilityMode,
    identity: identityPresent ? 'present' : 'absent',
    outcome: snapshot.surface === 'authorized' && identityPresent ? 'authorized' : 'not-authorized',
    tenantSwitch,
    commandTimestamp: new Date().toISOString(),
  };
```

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/api-client/src/mobile/runtime.integration.test.ts`
Expected: PASS（共 5 个测试 = 1 个 skip [主测试无环境变量] + 4 个本地断言测试全绿，skip 非 fail）。真部署验收（环境具备时）：

```bash
WEKNORA_MOBILE_TEST_DEPLOYMENT_URL=https://<deployment-origin> \
WEKNORA_MOBILE_TEST_EMAIL=<multi-tenant-test-account> \
WEKNORA_MOBILE_TEST_PASSWORD=<short-lived-secret> \
WEKNORA_MOBILE_TEST_SWITCH_TENANT_ID=<second-tenant-id> \
npx tsx --test packages/api-client/src/mobile/runtime.integration.test.ts
```

Expected: 4/4 pass，`t.diagnostic` 输出的 JSON evidence 含 `"tenantSwitch":"switched"`。凭据只来自环境变量（Mimosa 约束），命令中的占位符由运行者替换，不得写入任何文件。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/runtime-integration-smoke.ts packages/api-client/src/mobile/runtime.integration.test.ts
git commit -m "test(mobile): real-HTTP tenant-switch evidence behind opt-in env credentials"
```

---

## 计划级验证

在 worktree 根（`.worktrees/issue30-sweep`）执行（作者已实跑各分量的既有部分：`packages/mobile-core/src/runtime/mobile-runtime.test.ts` 24/24 pass、`pnpm --filter @weknora/mobile test` 11/11 pass、typecheck 0 错；新增测试由执行者按任务逐步实跑）：

```bash
npx tsx --test packages/api-client/src/mobile/runtime.test.ts packages/api-client/src/mobile/runtime.integration.test.ts packages/mobile-core/src/runtime/mobile-runtime.test.ts packages/mobile-core/src/runtime/runtime-vault.test.ts packages/mobile-core/src/vault/scoped-vault.test.ts && pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck
```

覆盖说明：Task 1-4 的全部定向单测/接口测 + Task 6 的集成测试（无环境变量时其真实 HTTP 用例 skip，其余断言全跑）+ apps/mobile 测试与类型检查（Task 5，同时传递编译 mobile-core）。有意不含 `pnpm test:shared` 全量与 web/desktop 套件（flaky 且与本计划无关）。

## 差异记录（调查结论 vs 代码现状）

1. 调查称「无 switchTenant」——属实：`packages/mobile-core/src/runtime/types.ts:30-40` 无该方法；但 api-client 的 `createAuthApi().switchTenant`（endpoints.ts:142-146）与后端 `POST /auth/switch-tenant`（routes_auth_tenant.go:219、auth.go:1119）已存在，为 Web 能力。本计划复用之，不改后端。
2. Spec §4.2 写 `activateTenant(tenantID)：原子切换 scope，返回新的 Scope Lease`。已合并 T01 的接口约定是所有授权操作返回 presentation-safe `RuntimeSnapshot`、lease 经 `scopeLease()` 观察。本计划采用 `activateTenant(tenantId: string): Promise<RuntimeSnapshot>`（命名遵循 Spec；返回类型遵循已确立接口，快照含新 activeTenantId，新 lease 可经 `scopeLease()` 取得），属记录在案的实现取舍而非静默重设计。
3. Spec §9.2 `revoke(scopeLease, reason)` 的 `reason` 在 T02 无消费方（审计/保留策略在 #43），保留参数以贴合接口，不实现行为。
4. Spec 列出的 Scoped Vault 测试面中「offline drafts / rejection of offline side effects / retention 执行」分属 #40（T10 离线缓存/草稿）与 #43（T13 Retention）；本计划交付 `inspectPolicy()` 与 drafts 仓储的隔离/轮换/撤销，离线读写与保留期执行不在本 Issue 验收标准内。
5. 原生端加密：Expo/Hermes 当前无 `crypto.subtle`，`createNativeScopedVaultIfAvailable()` 在无 WebCrypto 的运行时返回 `undefined`（登录不受影响，fail closed）。SQLite 加密数据库 seam（Spec §9.3）与原生熵/Cipher Adapter 由 #40 完成；T02 的原生持久化用 expo-secure-store（OS 加密）承载微量值。
6. 「旧订阅」隔离：SSE 订阅在 #35（T05）引入；T02 交付其依赖的 lease 撤销机制并以在途请求/迟到响应测试证明。
7. Vault 撤销的残余风险（诚实声明）：若 Keychain/SecureStore `writeWrappedKey` 在 revoke 第一阶段就失败，旧 wrapped key 残留，此后同 scope 重新登录可读到旧行；Runtime 侧同步 lease 撤销保证所有已开句柄与在途访问立即 fail closed（Task 4 测试覆盖），持久层清理是尽力而为。该残余在 OS 存储写失败这一前提之外不出现。

## Consumes / Produces 汇总

**Consumes（前序计划：无。消费已合并主干能力）：**
- T01 Mobile Runtime：`createMobileRuntime`/`MobileRuntimePorts`/`RuntimeSnapshot`/`ScopeLease`（`packages/mobile-core/src/runtime/*`）
- T01 api-client Remote：`createMobileRuntimeRemote`（`packages/api-client/src/mobile/runtime.ts`）
- 既有 `createAuthApi().switchTenant`（`packages/api-client/src/auth/endpoints.ts:142`）与后端 `POST /api/v1/auth/switch-tenant`
- T01 集成证据骨架：`mobileRuntimeIntegrationConfig`/`runMobileRuntimeIntegration`（`apps/mobile/src/runtime-integration-smoke.ts`）

**Produces（后续 #33-#41、#66 等消费）：**
- `MobileRuntime.activateTenant(tenantId: string): Promise<RuntimeSnapshot>`；`RuntimeSnapshot.identity.tenants?: TenantOption[]`（`{ id: string; name?: string }`）
- `RuntimeRemote.switchTenant(input: { tenantId: string; refreshToken: string }): Promise<{ credential: StoredCredential; tenant?: Record<string, unknown> | null }>`（mobile-core 端口，与 api-client 适配返回类型逐字一致）与 api-client 适配实现；`RuntimeRemote.me` 返回类型加宽携带 `memberships?: unknown[]`
- `createScopedVault(ports: ScopedVaultPorts): ScopedVault`（`open/rotate/revoke/inspectPolicy`；`ScopedStore.drafts: { put/get/list/remove }`）
- `createWebCryptoCipher(): CipherPort`；`createInMemoryVaultKeyStore()/createInMemoryVaultStorage()`；`KeyStorePort`/`VaultStoragePort`/`CipherPort` seam
- `MobileRuntimePorts.scopedVault?: ScopedVault`（scope 变化自动 revoke）
- 包内 `RuntimeScopeLease`/`leaseScopeOf`/`leaseActive`/`LeaseScope`（`packages/mobile-core/src/runtime/scope-lease.ts`，供 mobile-core 内后续 Module 消费，不入公共导出）
- apps/mobile：`createSecureVaultKeyStore/createSecureVaultStorage`（SecureStore Adapter）、`AuthorizedLandingScreen` 租户切换 UI、`RuntimeSurfaceProps.onActivateTenant`
- 集成证据字段 `tenantSwitch` 与环境变量 `WEKNORA_MOBILE_TEST_SWITCH_TENANT_ID`

## 验收标准 → 证据映射

| 验收标准 | 证据 |
|---|---|
| 1. 跨 Tenant 缓存和请求均 fail closed | Task 3（scope 三维隔离、无/撤销 lease 拒绝、篡改密文拒绝）+ Task 4（切租户后旧 store 立即失败、旧租户行擦除）+ Task 2（无成员资格/迟到响应 fail closed、非法 tenantId 零请求） |
| 2. 切换、退出和撤权的加密 key 失效路径有自动化证据 | Task 4 三个测试（tenant-switch/sign-out/deployment-change 均 erase wrapped key + rows）+ Task 3（revoke 轮换 wrapped key、rotate 后旧 key 解密失败）——真实 AES-GCM |
| 3. 端到端最高稳定 Interface 验证 | Task 6 opt-in 真实 HTTP 测试（生产 transport + 具体 Remote Adapter + Runtime 编排 + 真换签），blocked-env 见「Global Constraints」末段声明；本地替代证据 = Task 2/3/4 Interface 级测试 + Task 1 wire 契约测试 |
