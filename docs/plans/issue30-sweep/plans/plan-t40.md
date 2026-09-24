# T10（#40）加密离线缓存、草稿与联网确认 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 Issue #40 交付 T10：Scoped Vault 新增 event projection 加密仓储并承载 Task 详情离线快照（AC1 按 Deployment/用户/Tenant 加密隔离）、Offline Gate 使 Run/审批/扩额/外部 Action 在离线时 fail closed 且撤权/退出/磁盘失败语义保持（AC2）、真实部署端到端集成证据（AC3 最高稳定 Interface 验证）。

**Architecture:** 离线缓存复用 #32 的 Scoped Vault 深模块：新增 `projections` 仓储（行命名空间 `.p.`、索引 `.pix`，与 drafts 同一不变量集——lease 校验、per-scope 串行、AES-GCM、30 天保留、revoke 尽力擦除），并由新 Adapter `createVaultTaskProjectionStore` 实现 #35 的 `TaskProjectionStore` 端口（module-seams §5.4「Task Store Port：Scoped Vault Adapter、in-memory Adapter」的补齐）。Task 详情（#35 的 `createTaskDetail`）的持久投影补充离线快照字段，`hydrate` 在 detail 通道失败时降级渲染加密投影（connection `interrupted` + interruption reason `'offline'`，不自动重连；联网后显式 `resync()` 恢复权威同步）。离线危险动作门是新深模块 `offline/`（`OfflineGate` + 三个端口装饰器），在组合根对 Task Office 的 backend（run=start）、interactions（approval=decide）、legacy（run=followUp）统一包装；`budget`/`external-action` 两类动作当前无服务端入口（#39/#48/#51 未实现），gate 的 `assertOnline` 已支持全部四类，供后续批次消费。New 屏（#36 控制器）增加离线门：离线时提交在派发前拒绝、草稿保持加密保存，联网后由用户手动点击提交确认发送（不自动重放）。

**Tech Stack:** TypeScript（`packages/mobile-core`、`apps/mobile` Expo RN）、node:test + tsx（TS 测试运行器，与 `scoped-vault.test.ts` 一致）、Web Crypto AES-GCM（`createWebCryptoCipher`，#32 产出）、expo-network（真机网络状态，惰性 require；Node 测试链不依赖）。**本计划零 Go 改动**（T10 是纯移动端能力）。所有测试命令在 worktree 根（`.worktrees/issue30-sweep`）执行；前置 `pnpm install` 已就绪（本计划作者已在当前 HEAD 实跑基线：`npx tsx --test packages/mobile-core/src/vault/scoped-vault.test.ts` 21 pass、`npx tsx --test packages/mobile-core/src/task-office/task-detail.test.ts` 全绿、`cd apps/mobile && npx tsx --test src/new-task-view.test.ts` 全绿）。

**Spec:** `docs/specs/2026-09-20-mobile-ai-office-design.md`（Implementation Decisions / Testing Decisions）、`docs/specs/2026-09-20-mobile-module-seams.md` §5.4 / §9、`docs/adr/0007-registered-devices-and-encrypted-cache.md`、`docs/plans/issue30-sweep/issues/issue-40.md`（验收标准原文）。

## Global Constraints（逐字引用批准需求）

- Issue #40 验收标准原文（issue-40.md:37-39 / 49-51）：
  - 「缓存按 Deployment/用户/Tenant 加密隔离。」
  - 「撤权、退出、磁盘失败和离线危险动作均 fail closed。」
  - 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」
- Issue #40 What to build 原文（issue-40.md:33）：「离线可查看获准 Task 内容并保存草稿/批注；联网后由用户确认提交，离线不能执行 Run、审批、扩额或外部 Action。」
- design spec Implementation Decisions：
  - 「Offline mode permits approved reads, drafts and annotations. It prohibits Run commands, approval, budget expansion and external Actions.」
  - 「Cache identity includes Deployment, user, Tenant, Task and Run where applicable. Scope changes invalidate subscriptions and reject late responses.」
  - 「Scoped Vault owns encrypted scope storage, drafts, submission journal, event projection, retention and revocation.」
  - 「Mobile core does not depend on React Native, DOM or concrete transport. Remote and native details are injected as Adapters.」
- design spec Testing Decisions：
  - 「Tests target observable behavior at the highest stable Interface.」
  - 「Scoped Vault Interface tests cover encryption adapter failures, Deployment/user/Tenant isolation, key rotation, revocation, retention, offline drafts and rejection of offline side effects.」
- module-seams §9.1/§9.2：「Scoped Vault 是内部支持 Module，唯一拥有：deployment/user/tenant scope 下的加密缓存；draft、submission journal、event projection、small artifact 和 preferences；key wrapping、retention、eviction、logout/撤权擦除；离线可读/可写草稿与禁止离线副作用的规则。」；「ScopedStore 只提供按领域仓储分组的读写，不提供任意全局 key/value。任何没有有效 Scope Lease 的访问失败。」
- ADR-0007：「获准离线内容按用户与 Tenant 分区并加密，空间可限制缓存范围和保留时间；登出、切换空间、设备撤销或权限失效后删除对应缓存或使其密钥不可用。」
- 安全约束（沿用 Mimosa 生成前约束，本计划不涉及服务端请求 URL 拼接与 SQL）：源码、示例和测试都不得写入可用的凭据字面量；集成证据只经 `WEKNORA_MOBILE_TEST_*` 环境变量取凭据。

**已核实代码现状（本计划作者实读，2026-09-24 HEAD = a51eeeaf 之后 worktree HEAD fb5f6653a）：**
- `packages/mobile-core/src/vault/scoped-vault.ts`（279 行）：`ScopedStore` 目前只有 `drafts` 仓储；行键 `${scopeKey}.d.<base64url(id)>`、索引 `${scopeKey}.ix`；`revoke` 先覆写新随机 wrapped key 再删行（scoped-vault.ts:257-274）；`inspectPolicy()` 返回 `{ categories: [{ category: 'drafts', retentionDays: 30 }] }`（scoped-vault.ts:275-277，其断言在 scoped-vault.test.ts:126-128）。
- `packages/mobile-core/src/task-office/task-detail.ts`（369 行）：`PersistedTaskProjection = { taskId; runId; cursor; events; savedAt }`（task-detail.ts:41-47），**不含** title/attention/execution 快照；`hydrate()` 直接 `await wrap(() => ports.backend.detail(input.runId))`（task-detail.ts:157），detail 失败即整次 reject——**离线降级不存在**；`persist()` 在 task-detail.ts:169-171。
- `packages/mobile-core/src/runtime/mobile-runtime.ts:116-135`：Runtime 在每次 scope 变化撤销 lease 并串行派发 `vault.revoke`（#32 已交付——「撤权、退出 fail closed」的 vault 面已成立，本计划不重复实现，只补测试覆盖投影仓储）。
- `apps/mobile/src/composition.ts:153-173`：`taskOfficeFor` 显式 `store: createInMemoryTaskProjectionStore()`（注释声明持久化是显式决策）；`apps/mobile/src/adapters/vault-adapters.ts`：SecureStore 行值上限 2000 字符（`SECURE_STORE_MAX_VALUE_BYTES`）。
- `apps/mobile/src/new-task-view.ts:48-52` 注释已声明 AC2 保留语义宿主（「未就绪/离线/冲突一律不清草稿」），但**离线无显式门**（submit 直接派发，离线时靠网络错误兜底，无结构化拒绝）。
- 全仓库 `rg -i 'scoped.?vault|offline.?draft|offline.?cache'`（ts/go，排除 node_modules）除上述 #32/#36 产出外零命中；旧实现已随 commit 723de9179 删除且未迁移——T10 的离线缓存/离线门/端到端证据确为 absent。

**已知延期缺口（B2-F23，如实声明不静默选型）**：持久化 `TaskProjectionStore` 的存储选型 ADR（SQLite 加密 Adapter vs SecureStore）未决。本计划的处理：vault 投影 Adapter 内置**明文体预算**（`PROJECTION_BODY_BUDGET_BYTES = 1400`，推导：SecureStore Adapter 密文值上限 2000 base64 字符 ⇒ 明文 ≈ 2000×3/4 − 28(IV+tag) ≈ 1472B，取 1400 留余量），超预算时**丢最旧事件**保留最新事件与完整状态快照（恢复正确性由 `mergeEventHistory` 的 watermark 语义保障，见 Task 3）；SQLite/文件系统加密 Adapter 的引入不在本计划内，真机长任务（200 事件全量投影）的离线时间线深度受此预算约束——离线视图保「状态卡完整 + 最近时间线」，已在计划差异记录中列为 ADR 待决项而非静默决定。

## Review Focus

spec 与 AC 蕴含但最可能伤人的五类输入/失败模式（每行注明归属任务的测试）：

1. **断网瞬间的危险动作连击**：用户在离线判定生效前后连续点提交/审批——危险动作必须在**派发前**拒绝且零服务端副作用，而不是发出后靠网络超时兜底（Task 4 测试「offline start is refused before the underlying port is touched」用调用计数断言零触达；Task 7 集成断言 `offlineBlockedActions` 含全部四类）。
2. **投影行超存储预算**：合法长 Task 详情的投影超过 SecureStore 单值上限——预算裁剪必须保完整状态快照与最新事件、且裁剪不破坏 watermark 恢复语义；行超限不得静默丢写（Task 3 测试「oversized projections shed the oldest events within budget and keep the snapshot intact」）。
3. **撤权/退出后的离线读**：signOut/切租户/换部署后，旧 scope 的投影与草稿必须不可读（`VAULT_LEASE`）且行被擦除，离线降级不得用已撤销 scope 的投影渲染（Task 1 测试「projection rows are ciphertext on disk and revoked with the scope」；Task 3 测试「a revoked lease fails closed on save and load」）。
4. **离线降级视图被误当权威**：detail 通道失败时渲染的投影必须明确标记 `interrupted` + reason `'offline'`，不自动重连、不冒充 live；联网后 resync 恢复权威视图并清除 offline 标记；scope 已变化时不得降级（Task 2 三个测试分别钉住）。
5. **磁盘损坏/密文篡改**：投影行密文被篡改或 JSON 结构损坏——解密/解析失败必须如实上抛（`VAULT_DECRYPT`/`VAULT_PROJECTION`），由 task-detail 的既有 catch 语义降级为「投影不可用」，绝不回退明文或崩溃（Task 1 既有密文测试先例 + Task 3 测试「a tampered projection row fails closed through the cipher」）。

---

## 任务结构（7 个任务，依赖序）

| # | 任务 | 交付 |
|---|---|---|
| 1 | Scoped Vault `projections` 仓储 | `ScopedStore.projections`（AC1 加密隔离的载体） |
| 2 | Task 详情投影快照与离线降级 | `PersistedTaskProjection.snapshot?`、`hydrate` 离线降级视图 |
| 3 | Vault TaskProjectionStore Adapter | `createVaultTaskProjectionStore`（含预算裁剪） |
| 4 | Offline Gate 与危险端口装饰器 | `createOfflineGate` + 三个 guard 装饰器（AC2 离线危险动作 fail closed） |
| 5 | 网络状态 Adapter 与 New 屏离线确认门 | `createNativeNetworkStatusIfAvailable`、New 屏离线拒绝 + 草稿保持（联网确认提交） |
| 6 | 组合根接线 | composition 的 vault 投影持久化 + guarded office + offline gate（AC1/AC2 落地到 App） |
| 7 | 端到端集成证据 | `runOfflineVaultIntegration`（AC3 真实部署 opt-in 证据） |

**Consumes（前三批已集成接口，逐字签名）：**
- #32：`createScopedVault(ports: ScopedVaultPorts): ScopedVault`、`ScopedStore.drafts: ScopedDraftRepository`（`put({id,body})/get(id)/list()/remove(id)`）、`createWebCryptoCipher(): CipherPort`、`createInMemoryVaultKeyStore()/createInMemoryVaultStorage()`（`packages/mobile-core/src/vault/`）；Runtime 在 scope 变化时 `vault.revoke`（mobile-runtime.ts:116-135）。
- #35：`TaskProjectionStore { load(runId): Promise<PersistedTaskProjection|undefined>; save(projection): Promise<void> }`、`createTaskDetail`（task-detail.ts:41-52, 95）、`createInMemoryTaskProjectionStore`（in-memory-task-detail.ts:4-14）。
- #36：`createNewTaskController(ports: NewTaskControllerPorts)`（new-task-view.ts:53）、`createScopedNewTaskDrafts`、New 屏 AC2 保留语义宿主（new-task-view.ts:48-52）。
- #38：`InteractionBackendPort { inbox(input: {limit}); decide(input: {item; decisionId; action}) }`（attention-inbox.ts:62-65）；decide 的错误包装在 attention-inbox.ts:154-161（`OfflineGateError` 会成为 `TASK_OFFICE_BACKEND` 的 `cause`）。
- #44：`LegacyTaskBackendPort { list/history/followUp }`（legacy-tasks.ts:60-65）。
- 集成冒烟先例：`disallowedDeploymentHost(hostname, variable)`（runtime-integration-smoke.ts:118，注释明示「导出供 task-detail smoke 复用」）、opt-in 证据契约模式（task-start-integration-smoke.ts）。

**Produces（本计划对外产出，供后续计划消费）：**
- `ScopedStore = { drafts: ScopedDraftRepository; projections: ScopedDraftRepository }`（Task 1）
- `OfflineTaskSnapshot = Omit<TaskBackendDetail, 'events'>`；`PersistedTaskProjection.snapshot?: OfflineTaskSnapshot`；`TaskInterruptionReason` 增 `'offline'`（Task 2）
- `createVaultTaskProjectionStore(input: { vault: ScopedVault; lease(): ScopeLease | undefined }): TaskProjectionStore`、`PROJECTION_BODY_BUDGET_BYTES = 1400`（Task 3）
- `NetworkStatusPort { online(): Promise<boolean> }`、`OfflineActionKind = 'run'|'approval'|'budget'|'external-action'`、`OFFLINE_ACTION_BLOCKED`、`OfflineGateError { code; action }`、`createOfflineGate(status: NetworkStatusPort | undefined): OfflineGate { status(): Promise<'online'|'offline'>; assertOnline(action): Promise<void> }`、`guardTaskBackend/guardInteractionBackend/guardLegacyTaskBackend`（Task 4；`assertOnline('budget')/('external-action')` 供 #39/#48/#51 消费）
- `createNativeNetworkStatusIfAvailable(): NetworkStatusPort | undefined`（Task 5）
- `offlineVaultIntegrationConfig(env)/runOfflineVaultIntegration(config)/emitOfflineVaultIntegrationEvidence(evidence, emit)`（Task 7）

**并行批次冲突控制**：新增文件全部为本计划独有（`offline/` 目录、`vault-projection-store.ts`、`network-status.ts`、两个 smoke 文件）。共享文件修改收敛为：`scoped-vault.ts`（仓储工厂化 + 双命名空间）、`task-detail.ts`（类型扩展 + `hydrate`/`persist` 两处 + reason 枚举一项）、`index.ts`（追加导出行）、`new-task-view.ts`/`NewTaskScreen.tsx`/`app/new.tsx`（离线门小段）、`composition.ts`（`taskOfficeFor` 内四处替换 + 单例两行）、`app-smoke.test.tsx`（追加一个测试）。与同批 #37（运行中调整，改 `task-office.ts`/`TaskHandle.act`）无文件重叠面——本计划不改 `task-office.ts`。

---

### Task 1: Scoped Vault `projections` 仓储（event projection 加密缓存类别）

**Files:**
- Modify: `packages/mobile-core/src/vault/scoped-vault.ts`
- Test: `packages/mobile-core/src/vault/scoped-vault.test.ts`（追加测试 + 更新既有 inspectPolicy 断言）
- 回归（不改）: `packages/mobile-core/src/runtime/runtime-vault.test.ts`（#32 的 revoke 断言必须仍然通过）

**Interfaces:**
- Consumes: `ScopedVaultPorts`、`DraftEntry`、`RuntimeScopeLease`（scoped-vault.ts 既有内部结构）。
- Produces: `ScopedStore = { drafts: ScopedDraftRepository; projections: ScopedDraftRepository }`；projections 行键 `${scopeKey}.p.<base64url(id)>`、索引键 `${scopeKey}.pix`；`inspectPolicy()` 返回 `{ categories: [{ category: 'drafts', retentionDays: 30 }, { category: 'projections', retentionDays: 30 }] }`；`revoke` 尽力擦除两命名空间全部行与索引；`rotate` 对两命名空间行统一重加密。

- [ ] **Step 1: 写失败测试**

在 `packages/mobile-core/src/vault/scoped-vault.test.ts` 末尾追加（文件已有 `test/assert/lease/vault/scopeKeyOf/createWebCryptoCipher/createInMemoryVaultKeyStore/createInMemoryVaultStorage` 导入，见 scoped-vault.test.ts:1-9 与 helper `lease()`/`vault()`）：

```ts
// ── T10（#40）：projections 仓储（event projection 加密缓存类别）──

test('projections round-trip rows and stay isolated from drafts by namespace', async () => {
  const { vault: scoped } = vault();
  const store = await scoped.open(lease(SCOPE_A));
  await store.projections.put({ id: 'run.42', body: '{"taskId":"task-42"}' });
  assert.equal((await store.projections.get('run.42'))?.body, '{"taskId":"task-42"}');
  assert.deepEqual((await store.projections.list()).map((entry) => entry.id), ['run.42']);
  assert.equal(await store.drafts.get('run.42'), undefined, 'the same id in another namespace must not alias');
  await store.projections.remove('run.42');
  assert.equal(await store.projections.get('run.42'), undefined);
  assert.deepEqual((await store.projections.list()), []);
});

test('projections are scope-isolated exactly like drafts', async () => {
  const { vault: scoped } = vault();
  const storeA = await scoped.open(lease(SCOPE_A));
  await storeA.projections.put({ id: 'run.42', body: 'tenant-one-projection' });
  const storeOtherDeployment = await scoped.open(lease(SCOPE_B));
  const storeOtherTenant = await scoped.open(lease({ ...SCOPE_A, tenantId: '9' }));
  const storeOtherUser = await scoped.open(lease({ ...SCOPE_A, userId: 'user-2' }));
  assert.equal(await storeOtherDeployment.projections.get('run.42'), undefined);
  assert.equal(await storeOtherTenant.projections.get('run.42'), undefined);
  assert.equal(await storeOtherUser.projections.get('run.42'), undefined);
  assert.equal((await storeA.projections.get('run.42'))?.body, 'tenant-one-projection');
});

test('projection rows are ciphertext on disk and revoked with the scope', async () => {
  const { vault: scoped, keyStore, storage } = vault();
  const store = await scoped.open(lease(SCOPE_A));
  await store.drafts.put({ id: 'draft-1', body: 'keep' });
  await store.projections.put({ id: 'run.42', body: 'projection-canary' });
  for (const [key, value] of storage.entries()) {
    if (key.includes('.p.')) {
      assert.doesNotMatch(value, /projection-canary/);
      assert.match(value, /^[A-Za-z0-9+/]+={0,2}$/, 'the projection row must be base64 ciphertext');
    }
  }
  await scoped.revoke(lease(SCOPE_A), 'sign-out');
  const scopeKey = await scopeKeyOf(SCOPE_A);
  for (const rowKey of storage.entries().keys()) assert.equal(rowKey.startsWith(scopeKey), false, `row ${rowKey} must be erased`);
  assert.equal(keyStore.entries().size, 0, 'the wrapped key must be erased');
  const reopened = await scoped.open(lease(SCOPE_A));
  assert.equal(await reopened.projections.get('run.42'), undefined);
  assert.equal(await reopened.drafts.get('draft-1'), undefined);
});

test('rotate re-keys projection rows together with drafts', async () => {
  const { vault: scoped, keyStore } = vault();
  const scopeKey = await scopeKeyOf(SCOPE_A);
  const store = await scoped.open(lease(SCOPE_A));
  await store.projections.put({ id: 'run.42', body: 'projection-body' });
  const oldKey = keyStore.entries().get(scopeKey)!;
  await scoped.rotate(lease(SCOPE_A));
  const after = await (await scoped.open(lease(SCOPE_A))).projections.get('run.42');
  assert.equal(after?.body, 'projection-body');
  assert.notDeepEqual([...keyStore.entries().get(scopeKey)!], [...oldKey]);
});
```

并更新既有 inspectPolicy 测试（scoped-vault.test.ts:126-128 原文是 `assert.deepEqual(scoped.inspectPolicy(), { categories: [{ category: 'drafts', retentionDays: 30 }] });`）：

```ts
test('inspectPolicy exposes the cacheable category and its retention window', () => {
  const { vault: scoped } = vault();
  assert.deepEqual(scoped.inspectPolicy(), { categories: [{ category: 'drafts', retentionDays: 30 }, { category: 'projections', retentionDays: 30 }] });
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/mobile-core/src/vault/scoped-vault.test.ts`
Expected: FAIL——新测试因 `store.projections` 为 `undefined` 报 `TypeError: Cannot read properties of undefined (reading 'put')`（open 目前只返回 `{ drafts }`，scoped-vault.ts:220）；inspectPolicy 断言报 deepEqual 不匹配。

- [ ] **Step 3: 最小实现**

修改 `packages/mobile-core/src/vault/scoped-vault.ts`（保持 drafts 命名空间全部既有不变量，仅做参数化扩展）：

3a. 类型区：`ScopedStore` 改为双仓储（scoped-vault.ts:24 原行 `export interface ScopedStore { drafts: ScopedDraftRepository }` 替换）：

```ts
export interface ScopedStore { drafts: ScopedDraftRepository; projections: ScopedDraftRepository }
```

3b. 常量区追加（放在 `const RETENTION_DAYS = 30;` 之后）：

```ts
/** ScopedStore 的两个领域仓储（spec §9.2 按领域仓储分组）：drafts 与 event projections。
 *  行键命名空间段沿用 R1-F10 设计——合法 id 的 base64url 编码不含 '.'，与 'd'/'p' 段
 *  及索引键（'ix'/'pix'）天然分离，id 无法劫持另一命名空间的行或索引。 */
type StoreNamespace = 'drafts' | 'projections';
const ROW_SEGMENT: Record<StoreNamespace, string> = { drafts: 'd', projections: 'p' };
const INDEX_SEGMENT: Record<StoreNamespace, string> = { drafts: 'ix', projections: 'pix' };
```

3c. `createScopedVault` 内：删除单命名空间的 `indexKey`/`rowKey`/`readIndex`/`writeIndex`（scoped-vault.ts:102-103、116-129），替换为参数化版本：

```ts
  const indexKeyOf = (scopeKey: string, namespace: StoreNamespace): string => `${scopeKey}.${INDEX_SEGMENT[namespace]}`;
  const rowKeyOf = (scopeKey: string, namespace: StoreNamespace, id: string): string => `${scopeKey}.${ROW_SEGMENT[namespace]}.${draftKeySegment(id)}`;
  const readIndex = async (scopeKey: string, namespace: StoreNamespace): Promise<string[]> => {
    const raw = await ports.storage.read(indexKeyOf(scopeKey, namespace));
    if (raw === null) return [];
    let value: unknown;
    try {
      value = JSON.parse(raw);
    } catch {
      // 损坏索引 fail closed（R1-F12）：静默按 [] 处理的写入路径会覆写索引、孤儿化全部行。
      throw new Error('VAULT_INDEX');
    }
    if (!Array.isArray(value) || !value.every((id) => typeof id === 'string')) throw new Error('VAULT_INDEX');
    return value;
  };
  const writeIndex = (scopeKey: string, namespace: StoreNamespace, ids: string[]): Promise<void> =>
    ports.storage.write(indexKeyOf(scopeKey, namespace), JSON.stringify(ids));
```

3d. `open()` 内的 drafts 仓储字面量（scoped-vault.ts:167-219）提取为共享工厂，open 返回双仓储（替换整个 `const drafts: ScopedDraftRepository = {...}` 与 `return { drafts };`）：

```ts
      const makeRepository = (namespace: StoreNamespace): ScopedDraftRepository => ({
        async put(input) {
          assertAccessible(scopeLease, session);
          assertDraftId(input.id);
          await enqueue(scopeKey, async () => {
            const ids = await readIndex(scopeKey, namespace);
            const entry: DraftEntry = { id: input.id, body: input.body, updatedAt: now() };
            await ports.storage.write(rowKeyOf(scopeKey, namespace, input.id), await sealRow(session.key, entry));
            let known = knownRowIds.get(scopeKey);
            if (!known) { known = new Set(); knownRowIds.set(scopeKey, known); }
            known.add(rowKeyOf(scopeKey, namespace, input.id));
            if (!ids.includes(input.id)) await writeIndex(scopeKey, namespace, [...ids, input.id]);
          });
        },
        async get(id) {
          assertAccessible(scopeLease, session);
          assertDraftId(id);
          return enqueue(scopeKey, async () => {
            const raw = await ports.storage.read(rowKeyOf(scopeKey, namespace, id));
            if (raw === null) return undefined;
            return openRow(session.key, raw, id);
          });
        },
        async list() {
          assertAccessible(scopeLease, session);
          return enqueue(scopeKey, async () => {
            const ids = await readIndex(scopeKey, namespace);
            // 并行读行（R1-F16b）：索引顺序保持，缺行跳过；单行损坏仍按整体 reject。
            const rows = await Promise.all(ids.map(async (id) => {
              const raw = await ports.storage.read(rowKeyOf(scopeKey, namespace, id));
              return raw === null ? undefined : openRow(session.key, raw, id);
            }));
            const entries = rows.filter((row): row is DraftEntry => row !== undefined);
            // 惰性保留清理（R1-F14）：30 天窗口外的行在遍历时删除并收缩索引。
            const retentionCutoff = Date.parse(now()) - RETENTION_DAYS * 24 * 3600 * 1000;
            const retained: string[] = [];
            for (const entry of entries) {
              if (Date.parse(entry.updatedAt) < retentionCutoff) await ports.storage.delete(rowKeyOf(scopeKey, namespace, entry.id));
              else retained.push(entry.id);
            }
            if (retained.length !== entries.length) await writeIndex(scopeKey, namespace, retained);
            return entries.filter((entry) => Date.parse(entry.updatedAt) >= retentionCutoff);
          });
        },
        async remove(id) {
          assertAccessible(scopeLease, session);
          assertDraftId(id);
          await enqueue(scopeKey, async () => {
            await ports.storage.delete(rowKeyOf(scopeKey, namespace, id));
            await writeIndex(scopeKey, namespace, (await readIndex(scopeKey, namespace)).filter((existing) => existing !== id));
          });
        },
      });
      return { drafts: makeRepository('drafts'), projections: makeRepository('projections') };
```

3e. `rotate()`（scoped-vault.ts:222-256）替换为双命名空间两阶段原子轮换：

```ts
    async rotate(scopeLease) {
      if (!leaseActive(scopeLease)) throw new Error('VAULT_LEASE');
      const scopeKey = await scopeKeyOf(requireScope(scopeLease));
      await enqueue(scopeKey, async () => {
        const session = sessions.get(scopeKey);
        const oldKey = session && !session.destroyed ? session.key : await ports.keyStore.readWrappedKey(scopeKey);
        if (!oldKey) return;
        // 两阶段原子 rotate：任一行解密失败时在任何写入之前中止，健康行保持旧 key 可读。
        type SealedRow = { namespace: StoreNamespace; id: string; entry: DraftEntry };
        const decrypted: SealedRow[] = [];
        for (const namespace of ['drafts', 'projections'] as const) {
          for (const id of await readIndex(scopeKey, namespace)) {
            const raw = await ports.storage.read(rowKeyOf(scopeKey, namespace, id));
            if (raw === null) continue;
            decrypted.push({ namespace, id, entry: await openRow(oldKey, raw, id) });
          }
        }
        const newKey = randomBytes(KEY_LENGTH);
        const rewritten: SealedRow[] = [];
        try {
          for (const row of decrypted) {
            await ports.storage.write(rowKeyOf(scopeKey, row.namespace, row.id), await sealRow(newKey, row.entry));
            rewritten.push(row);
          }
        } catch (cause) {
          // best-effort 回滚至旧 key；回滚再失败则该 scope fail-closed（session 置 destroyed），不留半状态。
          try {
            for (const row of rewritten) await ports.storage.write(rowKeyOf(scopeKey, row.namespace, row.id), await sealRow(oldKey, row.entry));
          } catch {
            if (session && !session.destroyed) session.destroyed = true;
            sessions.delete(scopeKey);
          }
          throw cause instanceof Error ? cause : new Error('VAULT_ROTATE', { cause });
        }
        await ports.keyStore.writeWrappedKey(scopeKey, newKey);
        if (session && !session.destroyed) session.key = newKey;
      });
    },
```

3f. `revoke()`（scoped-vault.ts:257-274）的 enqueue 体内替换索引/行擦除段（`const indexed = ...` 至 `await ports.keyStore.deleteWrappedKey(scopeKey);`）：

```ts
      await enqueue(scopeKey, async () => {
        // 先把 wrapped key 覆写成全新随机值：即使后续删行/删 key 部分失败，旧密文也不可再解。
        await ports.keyStore.writeWrappedKey(scopeKey, randomBytes(KEY_LENGTH));
        // 损坏索引不得中止撤销（Review Focus #1）：吞 VAULT_INDEX，尽力删行与索引键（两命名空间）。
        const indexedRows = async (namespace: StoreNamespace): Promise<string[]> =>
          (await readIndex(scopeKey, namespace).catch(() => [] as string[])).map((id) => rowKeyOf(scopeKey, namespace, id));
        const rows = [...new Set([
          ...await indexedRows('drafts'),
          ...await indexedRows('projections'),
          ...(knownRowIds.get(scopeKey) ?? []),
        ])];
        await Promise.all(rows.map((row) => ports.storage.delete(row)));
        knownRowIds.delete(scopeKey);
        await ports.storage.delete(indexKeyOf(scopeKey, 'drafts'));
        await ports.storage.delete(indexKeyOf(scopeKey, 'projections'));
        await ports.keyStore.deleteWrappedKey(scopeKey);
      });
```

（`knownRowIds` 语义微调：put 处记录**完整行键**（3d 的 `known.add(rowKeyOf(...))`），revoke 直接删除——两命名空间无歧义。既有注释「以写入侧记忆兜底尽力擦除」保持成立。）

3g. `inspectPolicy()`（scoped-vault.ts:275-277）替换：

```ts
    inspectPolicy() {
      return { categories: [{ category: 'drafts', retentionDays: RETENTION_DAYS }, { category: 'projections', retentionDays: RETENTION_DAYS }] };
    },
```

- [ ] **Step 4: 运行测试确认通过**

Run: `npx tsx --test packages/mobile-core/src/vault/scoped-vault.test.ts packages/mobile-core/src/runtime/runtime-vault.test.ts packages/mobile-core/src/runtime/mobile-runtime.test.ts`
Expected: PASS（原 21 + 新 4 用例；#32 的 runtime-vault revoke/rotate 断言不回归）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/vault/scoped-vault.ts packages/mobile-core/src/vault/scoped-vault.test.ts
git commit -m "feat(mobile-core): scoped vault projections repository (T10 #40 AC1 event-projection cache)"
```

---

### Task 2: Task 详情投影快照与离线降级视图

**Files:**
- Modify: `packages/mobile-core/src/task-office/task-detail.ts`
- Test: `packages/mobile-core/src/task-office/task-detail.test.ts`（追加测试）
- Modify: `packages/mobile-core/src/index.ts`（追加 `OfflineTaskSnapshot` 类型导出）

**Interfaces:**
- Consumes: `TaskBackendDetail`、`PersistedTaskProjection`、`TaskProjectionStore`、既有测试 helper `leased()`/`event$()`/`detail$()`/`officeWithDetail()`（task-detail.test.ts:19-58）、`createInMemoryTaskProjectionStore`（in-memory-task-detail.ts:4-14）。
- Produces: `export type OfflineTaskSnapshot = Omit<TaskBackendDetail, 'events'>`；`PersistedTaskProjection.snapshot?: OfflineTaskSnapshot`（旧格式无 snapshot = 离线降级不可用，如实 fail closed）；`TaskInterruptionReason` 增 `'offline'`；`hydrate()`/`resync()` 在 detail 通道失败且本 scope 有带快照投影时返回离线降级视图（`connection: 'interrupted'`、`interruption.reason: 'offline'`、不 startStream、`autoResyncs` 封顶不自动重连；联网后显式 `resync()` 恢复）。

- [ ] **Step 1: 写失败测试**

在 `packages/mobile-core/src/task-office/task-detail.test.ts` 末尾追加：

```ts
// ── T10（#40）：detail 通道失败时的离线降级（加密投影渲染）──

const snapshotOf = (source: TaskBackendDetail): OfflineTaskSnapshot => {
  const { events: _events, ...snapshot } = source;
  return snapshot;
};

test('offline degradation renders the persisted snapshot when the detail channel fails, and resync recovers', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const store = createInMemoryTaskProjectionStore();
  // 预置带快照的投影（模拟此前在线会话经 persist 落盘的形态）
  await store.save({
    taskId: 'task-1', runId: 'run-1', cursor: 2,
    events: [event$(1, 'run.started'), event$(2, 'tool.started')],
    savedAt: '2026-09-24T00:00:00Z', snapshot: snapshotOf(detail$()),
  });
  let reachable = false;
  const { office } = officeWithDetail(
    leaseRef,
    { detail: async () => { if (!reachable) throw new Error('network unreachable'); return detail$(); }, stream: () => createScriptedTaskStream() },
    store,
  );
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  const view = await handle.hydrate();
  assert.equal(view.connection, 'interrupted');
  assert.equal(view.interruption?.reason, 'offline');
  assert.equal(view.taskId, 'task-1');
  assert.equal(view.title, '季度竞品报告', 'the offline card comes from the snapshot, not the network');
  assert.equal(view.runStatus, 'running');
  assert.deepEqual(view.timeline.map((entry) => entry.seq), [1, 2], 'the offline timeline comes from the persisted projection');
  assert.equal(view.cursor, 2);
  // 联网后显式 resync 恢复权威同步，offline 标记被清除
  reachable = true;
  const recovered = await handle.resync();
  assert.notEqual(recovered.interruption?.reason, 'offline');
  assert.notEqual(recovered.connection, 'interrupted');
  handle.close();
});

test('detail failure without a snapshotted projection stays an honest error', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const store = createInMemoryTaskProjectionStore();
  await store.save({ taskId: 'task-1', runId: 'run-1', cursor: 0, events: [], savedAt: '2026-09-24T00:00:00Z' }); // 旧格式：无 snapshot
  const { office } = officeWithDetail(leaseRef, { detail: async () => { throw new Error('network unreachable'); } }, store);
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await assert.rejects(handle.hydrate(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_BACKEND');
  handle.close();
});

test('a revoked lease never degrades to the offline projection', async () => {
  const leasedScope = leased();
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leasedScope.lease;
  const store = createInMemoryTaskProjectionStore();
  await store.save({
    taskId: 'task-1', runId: 'run-1', cursor: 2, events: [event$(1), event$(2)],
    savedAt: '2026-09-24T00:00:00Z', snapshot: snapshotOf(detail$()),
  });
  const { office } = officeWithDetail(leaseRef, { detail: async () => { throw new Error('network unreachable'); } }, store);
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  leasedScope.revocable.revoke(); // 撤权发生在 detail 失败之前：不得用旧 scope 投影降级
  await assert.rejects(handle.hydrate(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
  handle.close();
});

test('persist carries the offline snapshot so a later offline hydrate can degrade', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const store = createInMemoryTaskProjectionStore();
  const { office } = officeWithDetail(leaseRef, { detail: async () => detail$() }, store);
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();
  handle.close();
  const persisted = store.snapshot().find((row) => row.runId === 'run-1');
  assert.ok(persisted, 'hydrate persists the projection');
  assert.equal(persisted!.snapshot?.title, '季度竞品报告', 'the persisted row carries the offline snapshot');
  assert.equal(persisted!.snapshot?.attention, 'required');
  assert.equal(persisted!.snapshot?.watermark, 2);
  assert.ok(persisted!.snapshot !== undefined);
  assert.equal('events' in persisted!.snapshot, false, 'the snapshot must not duplicate the event log');
});
```

并在文件顶部类型导入行（task-detail.test.ts:7）追加 `OfflineTaskSnapshot`：

```ts
import type { OfflineTaskSnapshot, TaskBackendDetail, TaskBackendEvent, TaskDetailBackendPort, TaskProjectionStore, TaskStreamControlFrame } from './task-detail.ts';
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/mobile-core/src/task-office/task-detail.test.ts`
Expected: FAIL——第一个测试 `hydrate()` rejects（`TASK_OFFICE_BACKEND`）导致测试失败；`OfflineTaskSnapshot` 导入报 `SyntaxError: The requested module does not provide an export named 'OfflineTaskSnapshot'`（tsx 运行时命名导出校验）。

- [ ] **Step 3: 最小实现**

修改 `packages/mobile-core/src/task-office/task-detail.ts`：

3a. 类型（task-detail.ts:11 替换 + task-detail.ts:41-47 扩展）：

```ts
export type TaskInterruptionReason = 'gap' | 'cursor-expired' | 'stream-error' | 'stream-ended-nonterminal' | 'persist-failed' | 'stream-unavailable' | 'offline';
```

```ts
/** T10（#40）离线降级渲染所需的权威快照：detail 去 events 形态（title/attention/execution 等）。
 *  旧格式投影无 snapshot = 离线降级不可用（fail closed），联网路径不受影响。 */
export type OfflineTaskSnapshot = Omit<TaskBackendDetail, 'events'>;

export interface PersistedTaskProjection {
  taskId: string;
  runId: string;
  cursor: number;
  events: TaskBackendEvent[];
  savedAt: string;
  /** T10（#40）：detail 通道失败时构建离线视图的快照；缺省（旧格式）不降级。 */
  snapshot?: OfflineTaskSnapshot;
}
```

3b. `persist`（task-detail.ts:169-171）附快照（显式字段挑选，不展开复制 events）：

```ts
  const snapshotOf = (source: TaskBackendDetail): OfflineTaskSnapshot => ({
    taskId: source.taskId,
    runId: source.runId,
    title: source.title,
    attention: source.attention,
    ...(source.archivedAt === undefined ? {} : { archivedAt: source.archivedAt }),
    execution: source.execution,
    watermark: source.watermark,
    incomplete: source.incomplete,
  });
  const persist = async (cursor: number, history: TaskBackendEvent[]): Promise<void> => {
    await ports.store.save({ taskId: detail!.taskId, runId: input.runId, cursor, events: history.slice(-TASK_DETAIL_HISTORY_LIMIT), savedAt: new Date().toISOString(), snapshot: snapshotOf(detail!) });
  };
```

3c. `hydrate`（task-detail.ts:156-158 的 `const fetched = await wrap(...)` 与其后 scope check）替换为离线降级分支：

```ts
    let fetched: TaskBackendDetail;
    try {
      fetched = await wrap(() => ports.backend.detail(input.runId));
    } catch (error) {
      // T10（#40）离线降级：detail 通道失败而本 scope 有带快照的持久投影——渲染离线快照，
      // 明确标记 interrupted/offline、不自动重连；联网后显式 resync() 恢复权威同步。
      if (error instanceof TaskOfficeError && error.code !== 'TASK_OFFICE_BACKEND') throw error;
      if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED'); // 撤权不得伪装成离线
      const persisted = await ports.store.load(input.runId).catch(() => undefined);
      if (persisted === undefined || persisted.snapshot === undefined) throw error;
      detail = { ...persisted.snapshot, events: [] };
      events = persisted.events.slice(-TASK_DETAIL_HISTORY_LIMIT);
      committedCursor = persisted.cursor;
      duplicateSeqs = [];
      interruption = { reason: 'offline', message: '当前离线：以下为最近一次同步的加密缓存内容' };
      autoResyncs = AUTO_RESYNC_LIMIT; // 离线不自动重试；显式 resync()（联网后）重置
      notify('interrupted');
      return current!;
    }
    if (epoch !== streamEpoch || !leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
```

（`wrap` 只把非 TaskOfficeError 包成 `TASK_OFFICE_BACKEND`（task-detail.ts:119-124），TaskOfficeError 原样透传——`error.code !== 'TASK_OFFICE_BACKEND'` 分支保证模块自身错误不被降级吞掉。）

3d. `packages/mobile-core/src/index.ts` 的 task-detail 类型导出行追加 `OfflineTaskSnapshot`（该行现为 `export type { PersistedTaskProjection, TaskBackendDetail, TaskBackendEvent, TaskConnectionState, TaskDetailView, TaskDetailBackendPort, TaskDetailPorts, TaskHandle, TaskInterruptionReason, TaskProjectionStore, TaskStreamControlFrame } from './task-office/task-detail.ts';`）：

```ts
export type { OfflineTaskSnapshot, PersistedTaskProjection, TaskBackendDetail, TaskBackendEvent, TaskConnectionState, TaskDetailView, TaskDetailBackendPort, TaskDetailPorts, TaskHandle, TaskInterruptionReason, TaskProjectionStore, TaskStreamControlFrame } from './task-office/task-detail.ts';
```

- [ ] **Step 4: 运行测试确认通过**

Run: `npx tsx --test packages/mobile-core/src/task-office/task-detail.test.ts packages/mobile-core/src/task-office/task-office.test.ts`
Expected: PASS（既有 689 行测试全绿 + 新 4 用例）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/task-office/task-detail.ts packages/mobile-core/src/task-office/task-detail.test.ts packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core): task detail offline degradation from snapshotted projections (T10 #40)"
```

---

### Task 3: Vault TaskProjectionStore Adapter（含行预算裁剪）

**Files:**
- Create: `packages/mobile-core/src/task-office/vault-projection-store.ts`
- Test: `packages/mobile-core/src/task-office/vault-projection-store.test.ts`
- Modify: `packages/mobile-core/src/index.ts`（追加导出）

**Interfaces:**
- Consumes: Task 1 的 `ScopedStore.projections`；Task 2 的 `PersistedTaskProjection.snapshot?`；#35 的 `TaskProjectionStore`；`RuntimeScopeLease`（scope-lease.ts，包内导入——vault 测试同模式，scoped-vault.test.ts:4）。
- Produces: `createVaultTaskProjectionStore(input: { vault: ScopedVault; lease(): ScopeLease | undefined }): TaskProjectionStore`；`PROJECTION_BODY_BUDGET_BYTES = 1400`（明文体预算；超限丢最旧事件，快照与最新事件保留——恢复正确性由 `mergeEventHistory` 的 watermark 语义保障：老事件缺失不影响一致性，detail 重取时服务端 `events` 补齐）。

- [ ] **Step 1: 写失败测试**

创建 `packages/mobile-core/src/task-office/vault-projection-store.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { createScopedVault } from '../vault/scoped-vault.ts';
import { createWebCryptoCipher } from '../vault/web-crypto-cipher.ts';
import { createInMemoryVaultKeyStore, createInMemoryVaultStorage } from '../vault/in-memory-adapters.ts';
import { createVaultTaskProjectionStore, PROJECTION_BODY_BUDGET_BYTES } from './vault-projection-store.ts';
import type { PersistedTaskProjection, TaskBackendEvent } from './task-detail.ts';

const SCOPE = { deploymentOrigin: 'https://a.example.test', userId: 'user-1', tenantId: '7' };

const event$ = (seq: number): TaskBackendEvent => ({
  runId: 'run-42', seq, type: 'text.delta', occurredAt: '2026-09-24T00:00:00Z', payload: { note: 'x'.repeat(60) },
});

const projection$ = (events: TaskBackendEvent[], cursor: number): PersistedTaskProjection => ({
  taskId: 'task-42', runId: 'run-42', cursor, events, savedAt: '2026-09-24T00:00:00Z',
  snapshot: {
    taskId: 'task-42', runId: 'run-42', title: '离线快照标题', attention: 'none',
    execution: { runStatus: 'succeeded', executionStatus: 'succeeded', settlementStatus: 'settled', revision: 3, seq: cursor },
    watermark: cursor, incomplete: false,
  },
});

function leasedVault() {
  const keyStore = createInMemoryVaultKeyStore();
  const storage = createInMemoryVaultStorage();
  const vault = createScopedVault({ keyStore, storage, cipher: createWebCryptoCipher() });
  let revocable = new RuntimeScopeLease(SCOPE);
  const lease = () => revocable.asScopeLease();
  return {
    storage, vault, lease,
    store: createVaultTaskProjectionStore({ vault, lease }),
    revokeLease: () => { revocable.revoke(); },
  };
}

test('saves and loads a full projection round-trip through the encrypted vault', async () => {
  const { store } = leasedVault();
  const projection = projection$([event$(1), event$(2)], 2);
  await store.save(projection);
  const loaded = await store.load('run-42');
  assert.deepEqual(loaded, projection);
});

test('projection rows are ciphertext and never contain the plaintext snapshot', async () => {
  const { store, storage } = leasedVault();
  await store.save(projection$([event$(1)], 1));
  const rows = [...storage.entries().values()].filter((value) => !value.startsWith('[')); // 排除索引行（明文 id 属预期）
  assert.ok(rows.length > 0);
  for (const value of rows) {
    assert.doesNotMatch(value, /离线快照标题/);
    assert.doesNotMatch(value, /task-42/);
  }
});

test('oversized projections shed the oldest events within budget and keep the snapshot intact', async () => {
  const { store } = leasedVault();
  const events = Array.from({ length: 100 }, (_unused, index) => event$(index + 1));
  await store.save(projection$(events, 100));
  const loaded = await store.load('run-42');
  assert.ok(loaded);
  assert.ok(loaded!.events.length < 100, 'oversized projections must be trimmed, not rejected or silently dropped');
  const seqs = loaded!.events.map((event) => event.seq);
  assert.ok(seqs.every((seq, index) => index === 0 || seq === seqs[index - 1]! + 1), 'the retained events are a contiguous recent window');
  assert.equal(seqs[seqs.length - 1], 100, 'the newest event is always kept');
  assert.equal(loaded!.snapshot?.title, '离线快照标题', 'the snapshot survives trimming');
  assert.equal(loaded!.cursor, 100, 'the committed cursor survives trimming');
  // 预算自证：序列化体（含同样裁剪后的 events）在预算内
  const serialized = JSON.stringify({ ...loaded });
  assert.ok(new TextEncoder().encode(serialized).length <= PROJECTION_BODY_BUDGET_BYTES + 200, `serialized body ${serialized.length}B stays near budget`);
});

test('a revoked or missing lease fails closed on save and load', async () => {
  const { store, revokeLease } = leasedVault();
  await store.save(projection$([event$(1)], 1));
  revokeLease();
  await assert.rejects(store.load('run-42'), /VAULT_LEASE/);
  await assert.rejects(store.save(projection$([event$(2)], 2)), /VAULT_LEASE/);
});

test('a runtime-style vault revoke erases the projection rows for the scope (AC2 revocation face)', async () => {
  // Runtime 在撤权/退出/切租户时串行派发 vault.revoke（#32 mobile-runtime.ts:116-135）；
  // 本测试显式执行同一序列，验证投影行随之不可读（先覆写 key 再删行的既有语义）。
  const { vault, lease, store } = leasedVault();
  await store.save(projection$([event$(1)], 1));
  await vault.revoke(lease(), 'sign-out');
  assert.equal(await store.load('run-42'), undefined, 'a revoked scope leaves no readable projection');
  await store.save(projection$([event$(1)], 1)); // 同 scope 重新落盘 = 全新 key 全新行
  assert.equal((await store.load('run-42'))?.events.length, 1);
});

test('a tampered projection row fails closed through the cipher', async () => {
  const { store, storage } = leasedVault();
  await store.save(projection$([event$(1)], 1));
  const rowKey = [...storage.entries().keys()].find((key) => key.includes('.p.'))!;
  const bytes = Buffer.from(storage.entries().get(rowKey)!, 'base64');
  bytes[bytes.length - 1] ^= 0xff;
  (storage.entries() as Map<string, string>).set(rowKey, Buffer.from(bytes).toString('base64'));
  await assert.rejects(store.load('run-42'), /VAULT_DECRYPT/);
});

test('malformed run ids never touch the vault', async () => {
  const { store } = leasedVault();
  assert.equal(await store.load('../etc/passwd'), undefined);
  assert.equal(await store.load(''), undefined);
  await assert.rejects(store.save({ ...projection$([], 0), runId: '../etc/passwd' }), /VAULT_RUN_ID/);
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/mobile-core/src/task-office/vault-projection-store.test.ts`
Expected: FAIL——`Error: Cannot find module '.../vault-projection-store.ts'`（模块尚不存在）。

- [ ] **Step 3: 最小实现**

创建 `packages/mobile-core/src/task-office/vault-projection-store.ts`：

```ts
import { leaseActive } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import type { ScopedVault } from '../vault/scoped-vault.ts';
import type { PersistedTaskProjection, TaskProjectionStore, TaskBackendEvent } from './task-detail.ts';

/**
 * Task Store Port 的 Scoped Vault Adapter（module-seams §5.4；生产持久化 = #40 T10）：
 * PersistedTaskProjection 经 projections 仓储加密落盘（scope key 随 lease 派生——
 * Deployment/用户/Tenant 隔离由 vault 保证，AC1）。lease 失效即 VAULT_LEASE（AC2 撤权面）。
 *
 * 行预算（B2-F23 存储选型 ADR 未决的最小可行边界）：SecureStore Adapter 单值上限
 * 2000 base64 字符（apps/mobile/src/adapters/vault-adapters.ts:17）⇒ 明文 ≈1472B，取
 * 1400 留余量。超预算丢最旧事件——恢复正确性不受影响（hydrate 的 mergeEventHistory
 * 以服务端 watermark 为准，老事件由 detail 重取补齐），离线视图保留完整状态快照与最新时间线。
 */
export const PROJECTION_BODY_BUDGET_BYTES = 1400;

const RUN_ID_PATTERN = /^[A-Za-z0-9._-]{1,40}$/;
const projectionIdOf = (runId: string): string => `run.${runId}`;
const sizeOf = (value: string): number => new TextEncoder().encode(value).length;

function serializeWithinBudget(projection: PersistedTaskProjection): string {
  let events = projection.events;
  let body = JSON.stringify({ ...projection, events });
  while (events.length > 0 && sizeOf(body) > PROJECTION_BODY_BUDGET_BYTES) {
    events = events.slice(1); // 丢最旧：预算优先保快照与最新事件
    body = JSON.stringify({ ...projection, events });
  }
  return body;
}

function parseProjection(raw: string, runId: string): PersistedTaskProjection {
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    throw new Error('VAULT_PROJECTION');
  }
  if (typeof value !== 'object' || value === null) throw new Error('VAULT_PROJECTION');
  const row = value as Partial<PersistedTaskProjection>;
  if (typeof row.taskId !== 'string') throw new Error('VAULT_PROJECTION');
  if (typeof row.runId !== 'string' || row.runId !== runId) throw new Error('VAULT_PROJECTION');
  if (typeof row.cursor !== 'number' || !Number.isSafeInteger(row.cursor)) throw new Error('VAULT_PROJECTION');
  if (!Array.isArray(row.events) || typeof row.savedAt !== 'string') throw new Error('VAULT_PROJECTION');
  if (row.snapshot !== undefined && (typeof row.snapshot !== 'object' || row.snapshot === null)) throw new Error('VAULT_PROJECTION');
  return { ...(row as PersistedTaskProjection) };
}

export function createVaultTaskProjectionStore(input: { vault: ScopedVault; lease(): ScopeLease | undefined }): TaskProjectionStore {
  const openStore = async () => {
    const lease = input.lease();
    if (!lease || !leaseActive(lease)) throw new Error('VAULT_LEASE');
    return input.vault.open(lease);
  };
  return {
    async load(runId) {
      if (!RUN_ID_PATTERN.test(runId)) return undefined; // 畸形 runId 不触达 vault
      const entry = await (await openStore()).projections.get(projectionIdOf(runId));
      if (entry === undefined) return undefined;
      return parseProjection(entry.body, runId);
    },
    async save(projection) {
      if (!RUN_ID_PATTERN.test(projection.runId)) throw new Error('VAULT_RUN_ID');
      await (await openStore()).projections.put({ id: projectionIdOf(projection.runId), body: serializeWithinBudget(projection) });
    },
  };
}
```

（`projection$` 测试里 runId `run-42` 与 projection.runId 一致——parseProjection 的 `row.runId !== runId` 校验在 expectedId 绑定（scoped-vault.ts openRow）之上再加一层体校验。）

`packages/mobile-core/src/index.ts` 追加导出（放在既有 `createTaskDetail` 导出行之后）：

```ts
export { createVaultTaskProjectionStore, PROJECTION_BODY_BUDGET_BYTES } from './task-office/vault-projection-store.ts';
```

- [ ] **Step 4: 运行测试确认通过**

Run: `npx tsx --test packages/mobile-core/src/task-office/vault-projection-store.test.ts packages/mobile-core/src/task-office/task-detail.test.ts`
Expected: PASS（6 新用例 + Task 2 用例不回归）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/task-office/vault-projection-store.ts packages/mobile-core/src/task-office/vault-projection-store.test.ts packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core): scoped-vault task projection store adapter with row budget (T10 #40)"
```

---

### Task 4: Offline Gate 与危险端口装饰器

**Files:**
- Create: `packages/mobile-core/src/offline/offline-gate.ts`
- Create: `packages/mobile-core/src/offline/guarded-ports.ts`
- Test: `packages/mobile-core/src/offline/offline-gate.test.ts`
- Test: `packages/mobile-core/src/offline/guarded-ports.test.ts`
- Modify: `packages/mobile-core/src/index.ts`（追加导出）

**Interfaces:**
- Consumes: `TaskBackendPort`（task-office.ts:154-165）、`InteractionBackendPort`（attention-inbox.ts:62-65）、`LegacyTaskBackendPort`（legacy-tasks.ts:60-65）、`createScenarioTaskBackend`（in-memory-task-backend.ts:29）。
- Produces:
  - `export interface NetworkStatusPort { online(): Promise<boolean> }`
  - `export type OfflineActionKind = 'run' | 'approval' | 'budget' | 'external-action'`（四类对应 spec「It prohibits Run commands, approval, budget expansion and external Actions」；budget/external-action 当前无服务端入口，`assertOnline` 已支持，供 #39/#48/#51 消费）
  - `export const OFFLINE_ACTION_BLOCKED = 'OFFLINE_ACTION_BLOCKED'`；`export class OfflineGateError extends Error { readonly code = OFFLINE_ACTION_BLOCKED; readonly action: OfflineActionKind }`
  - `export function createOfflineGate(status: NetworkStatusPort | undefined): OfflineGate`，其中 `OfflineGate = { status(): Promise<'online' | 'offline'>; assertOnline(action: OfflineActionKind): Promise<void> }`。`status === undefined`（平台无网络状态通道）时透传放行——物理离线的派发由传输层必然失败兜底（fail closed 不变；结构化提前拒绝仅在有状态通道时提供）。状态探测抛错一律视为 offline（fail closed）。
  - `export function guardTaskBackend(backend: TaskBackendPort, gate: OfflineGate): TaskBackendPort`（拦 `start` → `'run'`；读方法与 archive/restore 透传）
  - `export function guardInteractionBackend(port: InteractionBackendPort, gate: OfflineGate): InteractionBackendPort`（拦 `decide` → `'approval'`；inbox 读透传）
  - `export function guardLegacyTaskBackend(port: LegacyTaskBackendPort, gate: OfflineGate): LegacyTaskBackendPort`（拦 `followUp` → `'run'`——knowledge-chat 追问触发服务端执行；list/history 读透传）

- [ ] **Step 1: 写失败测试**

创建 `packages/mobile-core/src/offline/offline-gate.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createOfflineGate, OfflineGateError, type NetworkStatusPort, type OfflineActionKind } from './offline-gate.ts';

const KINDS: OfflineActionKind[] = ['run', 'approval', 'budget', 'external-action'];

test('an offline status blocks every dangerous action kind before any dispatch', async () => {
  const gate = createOfflineGate({ online: async () => false });
  assert.equal(await gate.status(), 'offline');
  for (const kind of KINDS) {
    await assert.rejects(gate.assertOnline(kind), (error: unknown) => error instanceof OfflineGateError && error.code === 'OFFLINE_ACTION_BLOCKED' && error.action === kind);
  }
});

test('an online status passes every dangerous action kind', async () => {
  const gate = createOfflineGate({ online: async () => true });
  assert.equal(await gate.status(), 'online');
  for (const kind of KINDS) await gate.assertOnline(kind);
});

test('a failing network probe fails closed as offline', async () => {
  const gate = createOfflineGate({ online: async () => { throw new Error('probe timeout'); } });
  assert.equal(await gate.status(), 'offline');
  await assert.rejects(gate.assertOnline('run'), (error: unknown) => error instanceof OfflineGateError);
});

test('an absent status channel passes through: physical offline still fails at the transport seam', async () => {
  const gate = createOfflineGate(undefined);
  assert.equal(await gate.status(), 'online');
  for (const kind of KINDS) await gate.assertOnline(kind);
});
```

创建 `packages/mobile-core/src/offline/guarded-ports.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { emptyOverview, createScenarioTaskBackend } from '../task-office/in-memory-task-backend.ts';
import type { TaskBackendPort, TaskBackendStartInput } from '../task-office/task-office.ts';
import type { InteractionBackendPort, InteractionActionValue, InteractionKindValue, InboxItem, ResolvedDecisionRecord } from '../task-office/attention-inbox.ts';
import type { LegacyBackendTask, LegacyFollowUpInput, LegacyMessage, LegacyTaskBackendPort, LegacyTaskBackendPage } from '../task-office/legacy-tasks.ts';
import { createOfflineGate, OfflineGateError, type NetworkStatusPort } from './offline-gate.ts';
import { guardInteractionBackend, guardLegacyTaskBackend, guardTaskBackend } from './guarded-ports.ts';

const startInput: TaskBackendStartInput = {
  request_id: 'req-1', session_id: 'session-1', agent_id: 'agent-1', target_id: 'platform', workspace_ref: '', text: 'goal', budget_upper: 10,
};

const item: InboxItem = { interactionId: 'ix-1', runId: 'run-1', kind: 'tool_approval', argsHash: 'h', expectedRevision: 1, createdAt: '2026-09-24T00:00:00Z' };

const record: ResolvedDecisionRecord = {
  interactionId: 'ix-1', runId: 'run-1', kind: 'tool_approval', decisionId: 'dec-1', action: 'approve', argsHash: 'h', expectedRevision: 1,
};

const legacyPage = (): LegacyTaskBackendPage => ({ items: [{ taskId: 'legacy-1', title: '旧会话', attention: 'none', updatedAt: '2026-09-24T00:00:00Z' } as LegacyBackendTask] });

function counters() {
  const log: string[] = [];
  const backend: TaskBackendPort = {
    overview: async () => { log.push('overview'); return emptyOverview(); },
    list: async () => { log.push('list'); return { items: [] }; },
    archive: async (taskId: string) => { log.push(`archive:${taskId}`); },
    restore: async (taskId: string) => { log.push(`restore:${taskId}`); },
    createSession: async (input: { title: string }) => { log.push(`createSession:${input.title}`); return { sessionId: 'session-1' }; },
    start: async (input: TaskBackendStartInput) => { log.push(`start:${input.request_id}`); return { run_id: 'run-1', request_id: input.request_id, status: 'running' }; },
    lookup: async (requestId: string) => { log.push(`lookup:${requestId}`); return { state: 'unknown' as const }; },
  };
  const interactions: InteractionBackendPort = {
    inbox: async () => { log.push('inbox'); return { items: [] }; },
    decide: async (input: { item: InboxItem; decisionId: string; action: InteractionActionValue }) => {
      log.push(`decide:${input.decisionId}`);
      return { ...record, decisionId: input.decisionId, action: input.action };
    },
  };
  const legacy: LegacyTaskBackendPort = {
    list: async () => { log.push('legacy:list'); return legacyPage(); },
    history: async (taskId: string) => { log.push(`legacy:history:${taskId}`); return [{ messageId: 'm-1', role: 'user', content: 'q' } as LegacyMessage]; },
    followUp: async (input: LegacyFollowUpInput) => { log.push(`legacy:followUp:${input.taskId}`); },
  };
  return { log, backend, interactions, legacy };
}

const online: NetworkStatusPort = { online: async () => true };
const offline: NetworkStatusPort = { online: async () => false };

test('offline start is refused before the underlying backend is touched (zero dispatch)', async () => {
  const { log, backend } = counters();
  const guarded = guardTaskBackend(backend, createOfflineGate(offline));
  await assert.rejects(guarded.start(startInput), (error: unknown) => error instanceof OfflineGateError && error.action === 'run');
  assert.deepEqual(log, [], 'an offline dangerous action must never reach the backend');
});

test('online start passes through; reads and archive/restore are never gated even offline', async () => {
  const { log, backend } = counters();
  const onlineGuarded = guardTaskBackend(backend, createOfflineGate(online));
  const ack = await onlineGuarded.start(startInput);
  assert.equal(ack.run_id, 'run-1');
  assert.deepEqual(log, ['start:req-1']);
  const offlineGuarded = guardTaskBackend(backend, createOfflineGate(offline)); // 全程离线
  await offlineGuarded.overview();
  await offlineGuarded.archive('task-1');
  await offlineGuarded.restore('task-1');
  assert.deepEqual(log, ['start:req-1', 'overview', 'archive:task-1', 'restore:task-1'], 'approved reads and archive lifecycle are not dangerous actions');
});

test('offline decide is refused as approval before dispatch; inbox reads pass', async () => {
  const { log, interactions } = counters();
  const guarded = guardInteractionBackend(interactions, createOfflineGate(offline));
  const view = await guarded.inbox({ limit: 10 });
  assert.deepEqual(view.items, []);
  await assert.rejects(
    guarded.decide({ item, decisionId: 'dec-1', action: 'approve' }),
    (error: unknown) => error instanceof OfflineGateError && error.action === 'approval',
  );
  assert.deepEqual(log, ['inbox'], 'the offline decision must be the only blocked call');
});

test('offline legacy follow-up is refused as run; list/history reads pass', async () => {
  const { log, legacy } = counters();
  const guarded = guardLegacyTaskBackend(legacy, createOfflineGate(offline));
  const page = await guarded.list({});
  assert.equal(page.items.length, 1);
  await guarded.history('legacy-1');
  await assert.rejects(
    guarded.followUp({ taskId: 'legacy-1', question: '追问' }),
    (error: unknown) => error instanceof OfflineGateError && error.action === 'run',
  );
  assert.deepEqual(log, ['legacy:list', 'legacy:history:legacy-1']);
});

test('guards preserve the scenario backend structure (structural compatibility with the port)', async () => {
  const scenario = createScenarioTaskBackend({});
  const guarded = guardTaskBackend(scenario, createOfflineGate(online));
  const methods: Array<keyof TaskBackendPort> = ['overview', 'list', 'archive', 'restore', 'createSession', 'start', 'lookup'];
  for (const method of methods) assert.equal(typeof guarded[method], 'function', `${String(method)} must survive the guard`);
});
```

（`online`/`offline` 两个 `NetworkStatusPort` 常量声明在 `counters()` 定义之后、第一个测试之前——即上方代码段中已给出的两行 `const online: NetworkStatusPort = ...` 与 `const offline: NetworkStatusPort = ...`，按段内顺序写入即可。）

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test 'packages/mobile-core/src/offline/*.test.ts'`
Expected: FAIL——`Error: Cannot find module '.../offline-gate.ts'`（两个文件均不存在）。

- [ ] **Step 3: 最小实现**

创建 `packages/mobile-core/src/offline/offline-gate.ts`：

```ts
/**
 * Offline Gate（design spec Implementation Decisions："Offline mode permits approved
 * reads, drafts and annotations. It prohibits Run commands, approval, budget expansion
 * and external Actions."）：离线危险动作在派发前拒绝的结构化判决。
 *
 * 判定语义（AC2 fail closed）：
 * - 状态通道探测失败（抛错）一律视为离线；
 * - 平台无状态通道（status === undefined，如 Node 测试链/未装 expo-network）时透传放行：
 *   物理离线的危险动作由传输层必然失败兜底，fail closed 不变；gate 提供的是提前、
 *   结构化（OFFLINE_ACTION_BLOCKED:kind）的拒绝，而非唯一防线。
 */
export interface NetworkStatusPort {
  online(): Promise<boolean>;
}

export type OfflineActionKind = 'run' | 'approval' | 'budget' | 'external-action';

export const OFFLINE_ACTION_BLOCKED = 'OFFLINE_ACTION_BLOCKED';

export class OfflineGateError extends Error {
  readonly code = OFFLINE_ACTION_BLOCKED;
  constructor(readonly action: OfflineActionKind) {
    super(`${OFFLINE_ACTION_BLOCKED}:${action}`);
  }
}

export interface OfflineGate {
  status(): Promise<'online' | 'offline'>;
  assertOnline(action: OfflineActionKind): Promise<void>;
}

export function createOfflineGate(status: NetworkStatusPort | undefined): OfflineGate {
  const probe = async (): Promise<boolean> => {
    if (status === undefined) return true;
    try {
      return await status.online();
    } catch {
      return false; // fail closed：探测失败 = 离线
    }
  };
  return {
    async status() {
      return (await probe()) ? 'online' : 'offline';
    },
    async assertOnline(action) {
      if (!(await probe())) throw new OfflineGateError(action);
    },
  };
}
```

创建 `packages/mobile-core/src/offline/guarded-ports.ts`：

```ts
import type { TaskBackendPort, TaskBackendStartInput, TaskBackendStartAck } from '../task-office/task-office.ts';
import type { InboxItem, InteractionActionValue, InteractionBackendPort } from '../task-office/attention-inbox.ts';
import type { LegacyFollowUpInput, LegacyTaskBackendPort } from '../task-office/legacy-tasks.ts';
import type { OfflineGate } from './offline-gate.ts';

/** 危险动作装饰器（组合根专用）：派发前经 Offline Gate 拒绝，读通道与已获准的生命周期操作透传。 */

export function guardTaskBackend(backend: TaskBackendPort, gate: OfflineGate): TaskBackendPort {
  return {
    ...backend,
    async start(input: TaskBackendStartInput): Promise<TaskBackendStartAck> {
      await gate.assertOnline('run');
      return backend.start(input);
    },
  };
}

export function guardInteractionBackend(port: InteractionBackendPort, gate: OfflineGate): InteractionBackendPort {
  return {
    ...port,
    async decide(input: { item: InboxItem; decisionId: string; action: InteractionActionValue }) {
      await gate.assertOnline('approval');
      return port.decide(input);
    },
  };
}

export function guardLegacyTaskBackend(port: LegacyTaskBackendPort, gate: OfflineGate): LegacyTaskBackendPort {
  return {
    ...port,
    async followUp(input: LegacyFollowUpInput) {
      await gate.assertOnline('run'); // knowledge-chat 追问触发服务端执行：按 Run 语义拒绝
      return port.followUp(input);
    },
  };
}
```

`packages/mobile-core/src/index.ts` 追加导出块（放在 material 导出块之后）：

```ts
export { createOfflineGate, OfflineGateError, OFFLINE_ACTION_BLOCKED } from './offline/offline-gate.ts';
export type { NetworkStatusPort, OfflineActionKind, OfflineGate } from './offline/offline-gate.ts';
export { guardInteractionBackend, guardLegacyTaskBackend, guardTaskBackend } from './offline/guarded-ports.ts';
```

- [ ] **Step 4: 运行测试确认通过**

Run: `npx tsx --test 'packages/mobile-core/src/offline/*.test.ts' && npx tsx --test packages/mobile-core/src/task-office/task-office.test.ts packages/mobile-core/src/task-office/attention-inbox.test.ts`
Expected: PASS（9 新用例 + 邻域回归）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/offline/ packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core): offline gate with dangerous-port guards (T10 #40 AC2)"
```

---

### Task 5: 网络状态 Adapter 与 New 屏离线确认门

**Files:**
- Create: `apps/mobile/src/adapters/network-status.ts`
- Test: `apps/mobile/src/adapters/network-status.test.ts`
- Modify: `apps/mobile/src/new-task-view.ts`（offline 门 + 状态）
- Test: `apps/mobile/src/new-task-view.test.ts`（追加测试）
- Modify: `apps/mobile/src/screens/NewTaskScreen.tsx`（离线提示行）
- Modify: `apps/mobile/src/app/new.tsx`（注入 network 端口）

**Interfaces:**
- Consumes: Task 4 的 `NetworkStatusPort`；#36 的 `createNewTaskController(ports: NewTaskControllerPorts)`（new-task-view.ts:53）、`NewTaskViewState`（new-task-view.ts:17-28）、`NewTaskControllerPorts`（new-task-view.ts:7-15）。
- Produces:
  - `createNativeNetworkStatusIfAvailable(): NetworkStatusPort | undefined`（惰性 require `expo-network`；解析失败/平台不可用返回 undefined = gate 透传语义；`isInternetReachable !== true` 即离线，null 视为离线 fail closed）
  - `NewTaskControllerPorts.network?: NetworkStatusPort`（缺省不拦截——组合根显式决定）
  - `NewTaskViewState.offline: boolean` + `export const OFFLINE_SUBMIT_COPY = '当前离线：目标已加密保存为草稿；恢复联网后请手动点击提交确认发送。'`
  - `submit()` 在离线时：**零** `office.start` 调用、草稿保持保存（`persistDraft` 已有）、error 置 `OFFLINE_SUBMIT_COPY`（AC2「联网后由用户确认提交」——绝不自动重放；联网后用户点击即走 #36 既有 start 幂等路径）
  - `NewTaskScreenProps` 不变（offline 经 `state.offline` 渲染提示行）

- [ ] **Step 1: 写失败测试**

创建 `apps/mobile/src/adapters/network-status.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createNativeNetworkStatusIfAvailable } from './network-status.ts';

const here = dirname(fileURLToPath(import.meta.url));

test('a native network status is only returned when expo-network resolves (fail closed to undefined)', () => {
  // Node 测试链无 expo-network：结构级断言惰性 require + 缺席返回 undefined（与 push-token/device-identity 同模式）
  const source = readFileSync(join(here, 'network-status.ts'), 'utf8');
  assert.match(source, /require\('expo-network'\)/);
  assert.match(source, /catch/, 'resolution failure must be contained, never thrown');
  const status = createNativeNetworkStatusIfAvailable();
  if (status !== undefined) {
    // 若环境意外可解析（真机），仍必须给出布尔判决而非抛错
    assert.equal(typeof status.online(), 'object'); // Promise
  }
});

test('the adapter maps isInternetReachable strictly: null/absent counts as offline', () => {
  const source = readFileSync(join(here, 'network-status.ts'), 'utf8');
  assert.match(source, /isInternetReachable === true/, 'only an explicit true counts as online (fail closed)');
});
```

在 `apps/mobile/src/new-task-view.test.ts` 末尾追加（该文件已导入 `createNewTaskController` 等；`OFFLINE_SUBMIT_COPY` 需新增导入）。**readiness 前提**：`evaluateSubmitReadiness` 要求 `text` 非空且 `agentId` 非空（task-form.ts:39-47）——`agents()` 必须返回一个 supported 的 general AgentOption，初始化才会兜底 `agentId`（new-task-view.ts:92-93），`update({ text })` 后 submit 才会到达离线门：

```ts
// ── T10（#40）：New 屏离线确认门（AC2：联网后由用户确认提交）──

const LEAD_AGENT: AgentOption = {
  id: 'agent-1', name: '通用助手', summary: '', kind: 'general',
  capability: { state: 'supported', reason: 'scenario' },
};

test('offline submit is refused before dispatch and the draft stays saved', async () => {
  const started: string[] = [];
  const saved: NewTaskDraft[] = [];
  const controller = createNewTaskController({
    office: {
      start: async (goal) => { started.push(goal.text); return { requestId: 'req-1', phase: 'bound', runId: 'run-1', dispatched: true }; },
      reconcilePending: async () => [],
    },
    agents: async () => [LEAD_AGENT],
    drafts: { load: async () => undefined, save: async (draft) => { saved.push(draft); } },
    newRequestId: () => 'req-1',
    network: { online: async () => false },
  });
  await controller.whenInitialized();
  controller.update({ text: '离线目标' });
  const receipt = await controller.submit();
  assert.equal(receipt, undefined);
  assert.deepEqual(started, [], 'an offline submit must never reach office.start');
  assert.equal(controller.state().error, OFFLINE_SUBMIT_COPY);
  assert.ok(controller.state().offline, 'the offline flag stays set on refusal');
  assert.ok(saved.some((draft) => draft.text === '离线目标'), 'the draft must stay persisted through the offline refusal');
  controller.dispose();
});

test('the offline flag is surfaced for the screen banner and cleared when back online', async () => {
  let online = false;
  const controller = createNewTaskController({
    office: {
      start: async () => ({ requestId: 'req-1', phase: 'bound', runId: 'run-1', dispatched: true }),
      reconcilePending: async () => [],
    },
    agents: async () => [LEAD_AGENT],
    newRequestId: () => 'req-1',
    network: { online: async () => online },
  });
  await controller.whenInitialized();
  assert.equal(controller.state().offline, true, 'initialization probes the network status');
  controller.update({ text: '联网目标' });
  online = true;
  const receipt = await controller.submit();
  assert.equal(receipt?.runId, 'run-1');
  assert.equal(controller.state().offline, false, 'a successful online submit clears the flag');
  controller.dispose();
});

test('no network port keeps the existing behavior (no interception)', async () => {
  const controller = createNewTaskController({
    office: { start: async () => ({ requestId: 'req-1', phase: 'bound', runId: 'run-1', dispatched: true }), reconcilePending: async () => [] },
    agents: async () => [LEAD_AGENT],
    newRequestId: () => 'req-1',
  });
  await controller.whenInitialized();
  assert.equal(controller.state().offline, false);
  controller.update({ text: '普通目标' });
  const receipt = await controller.submit();
  assert.equal(receipt?.runId, 'run-1');
  controller.dispose();
});
```

（文件顶部补充导入：`import { createNewTaskController, OFFLINE_SUBMIT_COPY } from './new-task-view.ts';`、`import type { AgentOption, NewTaskDraft } from '@weknora/domain/mobile';`——以该测试文件既有导入为准去重。）

- [ ] **Step 2: 运行确认失败**

Run: `cd apps/mobile && npx tsx --test src/adapters/network-status.test.ts src/new-task-view.test.ts && cd ../..`
Expected: FAIL——network-status.test.ts 报 `Cannot find module './network-status.ts'`；new-task-view.test.ts 报 `OFFLINE_SUBMIT_COPY` 未导出（`SyntaxError: The requested module does not provide an export named`）。

- [ ] **Step 3: 最小实现**

创建 `apps/mobile/src/adapters/network-status.ts`：

```ts
import type { NetworkStatusPort } from '@weknora/mobile-core';

/** 惰性解析 expo-network（Node 测试链无此模块；与 push-token/device-identity 同模式）。
 *  解析失败返回 undefined——组合根不装配离线拦截（gate 透传语义，见 offline-gate.ts）。 */
export function createNativeNetworkStatusIfAvailable(): NetworkStatusPort | undefined {
  try {
    const network = require('expo-network') as { getNetworkStateAsync(): Promise<{ isInternetReachable: boolean | null }> };
    return {
      async online() {
        return (await network.getNetworkStateAsync()).isInternetReachable === true; // null/undefined 一律离线（fail closed）
      },
    };
  } catch {
    return undefined;
  }
}
```

修改 `apps/mobile/src/new-task-view.ts`：

3a. 导入与常量（文件头部 import 区追加 `import type { NetworkStatusPort } from '@weknora/mobile-core';`，`SUBMISSION_REJECTED_COPY` 常量旁追加）：

```ts
/** T10（#40）离线确认门文案：草稿已加密保存，联网后由用户手动确认提交（绝不自动重放）。 */
export const OFFLINE_SUBMIT_COPY = '当前离线：目标已加密保存为草稿；恢复联网后请手动点击提交确认发送。';
```

3b. `NewTaskControllerPorts`（new-task-view.ts:7-15）追加端口：

```ts
  /** T10（#40）离线确认门：离线时 submit 在派发前拒绝、草稿保持保存；缺省不拦截。 */
  network?: NetworkStatusPort;
```

3c. `NewTaskViewState`（new-task-view.ts:17-28）追加字段：

```ts
  /** T10（#40）：网络离线标记（驱动 New 屏提示行；submit 的离线拒绝与此同源）。 */
  offline: boolean;
```

3d. 控制器实现（new-task-view.ts:54-62 的初始 state 加 `offline: false`；`initialized` 的 publish 对象加 `offline`；`submit()` 在 readiness 检查后、`publish({ ...state, submitting: true ... })` 前插入离线门）：

```ts
    async submit() {
      const readiness = evaluateSubmitReadiness(state.draft);
      if (!readiness.ready || state.submitting) {
        project({ readiness });
        persistDraft();
        return undefined;
      }
      // T10（#40）离线确认门：派发前拒绝（AC2——离线不能执行 Run），草稿保持加密保存。
      let offline = false;
      if (ports.network !== undefined) {
        offline = !(await ports.network.online().catch(() => false)); // 探测失败 = 离线（fail closed）
      }
      if (offline) {
        persistDraft();
        publish({ ...state, submitting: false, offline: true, error: OFFLINE_SUBMIT_COPY });
        return undefined;
      }
      publish({ ...state, submitting: true, offline: false, error: undefined });
      // ……（既有 start/回执处理不变）
```

（`initialized` 内同步探测一次并在最终 `publish({...})` 中带 `offline` 字段——插入位置在 `const unresolved = ...` 之前，与 submit 的探测语义一致（探测失败 = 离线，fail closed）：

```ts
      const offline = ports.network !== undefined ? !(await ports.network.online().catch(() => false)) : false;
```

并把它加入该 publish 对象：`offline,`（`NewTaskViewState.offline` 为必填布尔）。）

3e. `apps/mobile/src/screens/NewTaskScreen.tsx`（`state.inFlight` 提示行旁、`state.error` 行之前）追加：

```tsx
      {state.offline && <Text>当前离线：草稿已加密保存，恢复联网后请手动点击提交确认发送。</Text>}
```

3f. `apps/mobile/src/app/new.tsx`（NewTaskRouteLifecycle 的 `createNewTaskController({...})` 调用中，`newRequestId: createNativeRequestId(),` 之后追加一行；文件头部 import `createNativeNetworkStatusIfAvailable`）：

```ts
        ...(createNativeNetworkStatusIfAvailable() === undefined ? {} : { network: createNativeNetworkStatusIfAvailable() }),
```

（调用两次保持「缺省不注入 undefined」的显式形态；或提取局部变量 `const network = createNativeNetworkStatusIfAvailable();` 后 `...(network === undefined ? {} : { network })`——推荐后者。）

- [ ] **Step 4: 运行测试确认通过**

Run: `cd apps/mobile && npx tsx --test src/adapters/network-status.test.ts src/new-task-view.test.ts src/new-task-drafts.test.ts && cd ../..`
Expected: PASS（新 5 用例 + 既有 new-task-view/new-task-drafts 全绿）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/adapters/network-status.ts apps/mobile/src/adapters/network-status.test.ts apps/mobile/src/new-task-view.ts apps/mobile/src/new-task-view.test.ts apps/mobile/src/screens/NewTaskScreen.tsx apps/mobile/src/app/new.tsx
git commit -m "feat(mobile): offline confirmation gate for new-task submit with native network status (T10 #40)"
```

---

### Task 6: 组合根接线（vault 投影持久化 + guarded office + offline gate）

**Files:**
- Modify: `apps/mobile/src/composition.ts`
- Test: `apps/mobile/src/app-smoke.test.tsx`（追加源级断言测试）

**Interfaces:**
- Consumes: Task 3 的 `createVaultTaskProjectionStore`；Task 4 的 `createOfflineGate/guardTaskBackend/guardInteractionBackend/guardLegacyTaskBackend`；Task 5 的 `createNativeNetworkStatusIfAvailable`；#32 的 `nativeScopedVault` 单例（composition.ts:41）与 `openScopedDraftStore`（composition.ts:47-55）。
- Produces: 组合根装配——授权 scope 的 Task Office 使用 vault 投影持久化（vault 缺席显式回退 in-memory）、危险端口经 offline gate 包装；`/new` 注入网络状态（Task 5 已完成）。App 层语义：AC1/AC2 在真实 App 装配中生效。

- [ ] **Step 1: 写失败测试**

在 `apps/mobile/src/app-smoke.test.tsx` 末尾追加（该文件已有 `readFileSync`/`join`/`here` 源级断言先例，见 app-smoke.test.tsx:584-590、739-741；若该文件尚未导入 `readFileSync`/`join`/`here`，照 app-smoke.test.tsx:585-589 的既有局部导入模式在测试体内导入）：

```tsx
test('the composition guards offline-dangerous ports and persists projections through the scoped vault (T10)', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const source = readFileSync(join(here, 'composition.ts'), 'utf8');
  // 离线危险动作门（AC2）：backend(run=start)/interactions(approval=decide)/legacy(run=followUp) 全部经 gate
  assert.match(source, /guardTaskBackend\(remote/, 'taskOfficeFor must guard the start channel');
  assert.match(source, /guardInteractionBackend\(remote/, 'taskOfficeFor must guard the decide channel');
  assert.match(source, /guardLegacyTaskBackend\(/, 'taskOfficeFor must guard the legacy follow-up channel');
  assert.match(source, /createOfflineGate\(/, 'the gate must be constructed once at the composition root');
  // 加密投影持久化（AC1）：vault 在场时 store 走 Scoped Vault Adapter
  assert.match(source, /createVaultTaskProjectionStore\(\{\s*vault:\s*nativeScopedVault/, 'the task office store must be the scoped-vault adapter when the vault exists');
  assert.match(source, /: createInMemoryTaskProjectionStore\(\)/, 'vault absence must be an explicit in-memory decision');
  // 既有防线不回归（#35 源级断言，app-smoke.test.tsx:590）
  assert.match(source, /detail:\s*remote/, 'taskOfficeFor must still pass the remote as the detail port');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `cd apps/mobile && npx tsx --test src/app-smoke.test.tsx && cd ../..`
Expected: FAIL——新增测试的 `assert.match` 报 `The input did not match the regular expression`（composition.ts 尚无 guard/vault-store 装配）。

- [ ] **Step 3: 最小实现**

修改 `apps/mobile/src/composition.ts`：

3a. 导入区（mobile-core 导入附近，composition.ts:8-13 之后）追加：

```ts
import { createOfflineGate, createVaultTaskProjectionStore, guardInteractionBackend, guardLegacyTaskBackend, guardTaskBackend } from '@weknora/mobile-core';
import { createNativeNetworkStatusIfAvailable } from './adapters/network-status.ts';
```

3b. `nativeScopedVault` 单例（composition.ts:41）之后追加 gate 单例：

```ts
/** T10（#40）离线危险动作门（run/approval/budget/external-action）：原生网络状态缺席时
 *  gate 透传（物理离线的派发失败由传输层兜底，fail closed 不变）。 */
const nativeOfflineGate = createOfflineGate(createNativeNetworkStatusIfAvailable());
```

3c. `taskOfficeFor`（composition.ts:153-173）的 `createTaskOffice({...})` 装配替换四处（`backend`/`interactions`/`legacy`/`store`，其余行原样保留）：

```ts
    return createTaskOffice({
      // T10（#40）AC2：Run/审批/追问在派发前经 Offline Gate 拒绝；读通道与 detail 不拦。
      backend: guardTaskBackend(remote, nativeOfflineGate),
      detail: remote,
      interactions: guardInteractionBackend(remote, nativeOfflineGate),
      legacy: guardLegacyTaskBackend(createMobileLegacyTaskRemote({
        origin,
        request: (input) => activeRuntime.authorizedRequest(input),
        stream: (input, onChunk) => activeRuntime.authorizedEventStream(input, onChunk),
      }), nativeOfflineGate),
      lease: () => activeRuntime.scopeLease(),
      // T10（#40）AC1：获准 Task 内容的加密投影经 Scoped Vault event-projection 仓储持久化；
      // vault 缺席（无 WebCrypto/SecureStore）时显式回退 in-memory——持久化缺失是组合根的显式决策
      // （SQLite 加密 Adapter 的存储选型 ADR 未决，B2-F23 延期项；行预算见 vault-projection-store.ts）。
      store: nativeScopedVault
        ? createVaultTaskProjectionStore({ vault: nativeScopedVault, lease: () => activeRuntime.scopeLease() })
        : createInMemoryTaskProjectionStore(),
      intentLog: intentLogOf(),
      newRequestId: createNativeRequestId(),
    });
```

（原 `const remote = createTaskOfficeRemote({ origin, request: ..., stream: ... })` 行保持不变。）

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: PASS（含 app-smoke 全量与既有 `detail:\s*remote` 断言；typecheck 无输出）。typecheck 同时证明 guard 装饰器返回结构与 `TaskBackendPort`/`InteractionBackendPort`/`LegacyTaskBackendPort` 结构可赋值（strict tsc 编译通过即证据）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/composition.ts apps/mobile/src/app-smoke.test.tsx
git commit -m "feat(mobile): compose offline gate and scoped-vault projection persistence (T10 #40 AC1/AC2)"
```

---

### Task 7: 端到端集成证据（真实部署 opt-in）

**Files:**
- Create: `apps/mobile/src/offline-vault-integration-smoke.ts`
- Test: `apps/mobile/src/offline-vault-integration-smoke.test.ts`

**Interfaces:**
- Consumes: Task 1-6 全部产出；`createWeKnoraClient`/`createJsonTransport`/`FetchLike`（@weknora/api-client）、`createMobileRuntimeRemote`（api-client/mobile/runtime）、`createTaskOfficeRemote`（api-client/mobile/task-office）、`CLIENT_PROTOCOL_VERSION`（domain/mobile）、`createMobileRuntime`/`createInMemoryCredentialStore`（mobile-core）、`disallowedDeploymentHost(hostname, variable)`（runtime-integration-smoke.ts:118——「导出供 task-detail smoke 复用（B2-F15）」先例）。
- Produces:
  - `offlineVaultIntegrationConfig(env: Record<string, string | undefined>): OfflineVaultIntegrationConfig`（`{ enabled: true; deploymentOrigin; email; password } | { enabled: false; disposition: 'skip' | 'invalid'; reason }`；凭据仅经 `WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD`，HTTPS origin 校验 + `disallowedDeploymentHost` 主机防线）
  - `runOfflineVaultIntegration(config): Promise<OfflineVaultIntegrationEvidence>`（total 化：任何失败也产出证据对象，绝不 reject；断网模拟的唯一注入点是 transport fetcher）
  - `emitOfflineVaultIntegrationEvidence(evidence, emit)`（只发 redacted 证据，不含凭据）

**证据字段与 AC 对应：**
- `onlineStart: 'admitted' | 'pending' | 'rejected' | 'failed'`——联网 + 用户确认提交真实创建 Task（What to build 后半）。
- `offlineBlockedActions: string[]`——断网后 run（真实 start 重放经 guarded office）、approval（guarded decide）、budget、external-action（gate 通道）全部被 `OFFLINE_ACTION_BLOCKED` 拒绝（AC2）。
- `offlineDraftRoundTrip: boolean`——断网下草稿经 Scoped Vault 加密保存并可读回（AC1/AC2 草稿面）。
- `offlineDetailView: 'projected' | 'unavailable' | 'failed'`——断网下 detail 通道失败时 Task 详情从加密投影降级渲染（「离线可查看获准 Task 内容」，AC1）。
- `ciphertextProjectionRows: boolean`——vault 存储层的投影行不含明文 taskId（加密证据）。
- `onlineResync: 'recovered' | 'failed' | 'not-attempted'`——恢复联网后显式 resync 清除 offline 标记（不自动重放）。

- [ ] **Step 1: 写失败测试**

创建 `apps/mobile/src/offline-vault-integration-smoke.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { emitOfflineVaultIntegrationEvidence, offlineVaultIntegrationConfig, runOfflineVaultIntegration } from './offline-vault-integration-smoke.ts';

const env = () => process.env as Record<string, string | undefined>;
const here = dirname(fileURLToPath(import.meta.url));

test('integration stays opt-in: missing credentials skip, never fake a pass', () => {
  const config = offlineVaultIntegrationConfig({ ...env(), WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: undefined });
  assert.equal(config.enabled, false);
  assert.equal(config.disposition, 'skip');
  const invalid = offlineVaultIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'http://insecure.example', WEKNORA_MOBILE_TEST_EMAIL: 'e', WEKNORA_MOBILE_TEST_PASSWORD: 'p' });
  assert.equal(invalid.enabled, false);
  assert.equal(invalid.disposition, 'invalid');
  const loopback = offlineVaultIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://127.0.0.1', WEKNORA_MOBILE_TEST_EMAIL: 'e', WEKNORA_MOBILE_TEST_PASSWORD: 'p' });
  assert.equal(loopback.enabled, false);
  assert.equal(loopback.disposition, 'invalid');
});

test('live end-to-end offline cache, gate and confirmation through the highest stable interface (opt-in)', async (t) => {
  const config = offlineVaultIntegrationConfig(env());
  if (!config.enabled) {
    t.skip(config.reason);
    return;
  }
  const evidence = await runOfflineVaultIntegration(config);
  const emitted: string[] = [];
  emitOfflineVaultIntegrationEvidence(evidence, (record) => { emitted.push(record); });
  assert.equal(emitted.length, 1);
  assert.equal(JSON.parse(emitted[0]!).onlineStart, evidence.onlineStart);
  assert.equal(evidence.onlineStart, 'admitted', 'a live deployment with one agent must admit the online goal');
  assert.deepEqual(evidence.offlineBlockedActions.sort(), ['approval', 'budget', 'external-action', 'run'], 'AC2: all four dangerous action kinds are refused offline');
  assert.equal(evidence.offlineDraftRoundTrip, true, 'offline drafts round-trip through the scoped vault');
  assert.equal(evidence.offlineDetailView, 'projected', 'AC1: the task detail degrades to the encrypted projection while offline');
  assert.equal(evidence.ciphertextProjectionRows, true, 'vault rows must not contain plaintext task ids');
  assert.equal(evidence.onlineResync, 'recovered', 'an explicit resync recovers the authoritative view once back online');
});

test('the integration runner is total: a failing transport still yields evidence, not a rejection', () => {
  const source = readFileSync(join(here, 'offline-vault-integration-smoke.ts'), 'utf8');
  assert.match(source, /finally\s*\{/);
  assert.match(source, /catch \(error\)/);
  assert.match(source, /errorReason/, 'failures land in the evidence contract');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `cd apps/mobile && npx tsx --test src/offline-vault-integration-smoke.test.ts && cd ../..`
Expected: FAIL——`Error: Cannot find module '.../offline-vault-integration-smoke.ts'`。

- [ ] **Step 3: 最小实现**

创建 `apps/mobile/src/offline-vault-integration-smoke.ts`：

```ts
import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import {
  createInMemoryCredentialStore, createInMemoryVaultKeyStore, createInMemoryVaultStorage, createMobileRuntime,
  createOfflineGate, createScopedVault, createTaskOffice, createVaultTaskProjectionStore, createWebCryptoCipher,
  OfflineGateError, guardInteractionBackend, guardTaskBackend, type OfflineActionKind, type TaskOffice,
} from '@weknora/mobile-core';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';

export type OfflineVaultIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface OfflineVaultIntegrationEvidence {
  deploymentOrigin: string;
  onlineStart: 'admitted' | 'pending' | 'rejected' | 'failed';
  offlineBlockedActions: string[];
  offlineDraftRoundTrip: boolean;
  offlineDetailView: 'projected' | 'unavailable' | 'failed';
  ciphertextProjectionRows: boolean;
  onlineResync: 'recovered' | 'failed' | 'not-attempted';
  errorReason?: string;
  timestamp: string;
}

/** 与 runtime-integration-smoke.ts 相同的 opt-in 语义；主机防线复用 disallowedDeploymentHost（B2-F15）。 */
export function offlineVaultIntegrationConfig(env: Record<string, string | undefined>): OfflineVaultIntegrationConfig {
  const deploymentOrigin = env.WEKNORA_MOBILE_TEST_DEPLOYMENT_URL?.trim();
  const email = env.WEKNORA_MOBILE_TEST_EMAIL?.trim();
  const password = env.WEKNORA_MOBILE_TEST_PASSWORD;
  if (!deploymentOrigin || !email || !password) {
    return { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' };
  }
  let parsed: URL;
  try { parsed = new URL(deploymentOrigin); } catch {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL is not an absolute URL' };
  }
  if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.pathname !== '/' || parsed.search || parsed.hash) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must be a credential-free HTTPS origin' };
  }
  const hostRejection = disallowedDeploymentHost(parsed.hostname, 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL');
  if (hostRejection) return { enabled: false, disposition: 'invalid', reason: hostRejection };
  return { enabled: true, deploymentOrigin: parsed.origin, email, password };
}

/** decide 经 Task Office 的错误包装（attention-inbox.ts:154-161）：OfflineGateError 落在 cause 链上。 */
const offlineBlockOf = (error: unknown): OfflineActionKind | undefined => {
  if (error instanceof OfflineGateError) return error.action;
  if (error instanceof Error && error.cause instanceof OfflineGateError) return error.cause.action;
  return undefined;
};

/**
 * 真实端到端（AC3）：生产 JSON transport + Runtime 授权通道 + 真实 Scoped Vault（AES-GCM，
 * in-memory keyStore/storage Adapter——vault 深模块本身真实执行）+ guarded Task Office +
 * vault 投影持久化。断网模拟的唯一注入点是 transport fetcher（离线时直接拒绝）。
 * total 化收口：任何步骤异常也产出证据对象（含 errorReason，不含凭据），绝不 reject。
 */
export async function runOfflineVaultIntegration(config: Extract<OfflineVaultIntegrationConfig, { enabled: true }>): Promise<OfflineVaultIntegrationEvidence> {
  const evidence: OfflineVaultIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    onlineStart: 'failed',
    offlineBlockedActions: [],
    offlineDraftRoundTrip: false,
    offlineDetailView: 'failed',
    ciphertextProjectionRows: false,
    onlineResync: 'not-attempted',
    timestamp: new Date().toISOString(),
  };
  let networkOnline = true;
  const fetcher: FetchLike = (input, init) => {
    if (!networkOnline) return Promise.reject(new Error('offline: network unreachable'));
    return fetch(input, init as RequestInit);
  };
  const keyStore = createInMemoryVaultKeyStore();
  const storage = createInMemoryVaultStorage();
  const vault = createScopedVault({ keyStore, storage, cipher: createWebCryptoCipher() });
  const gate = createOfflineGate({ online: async () => networkOnline });
  const runtime = createMobileRuntime({
    credentialStore: createInMemoryCredentialStore(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    scopedVault: vault,
    remoteFor(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return createMobileRuntimeRemote({ origin, request: client.request });
    },
    authorizedTransport(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return (input, accessToken) => client.request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
    },
  });
  try {
    const snapshot = await runtime.signIn({ deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' }, email: config.email, password: config.password });
    if (snapshot.surface !== 'authorized' || !snapshot.deployment) {
      evidence.errorReason = `surface ${snapshot.surface}`;
      return evidence;
    }
    const remote = createTaskOfficeRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) });
    const office: TaskOffice = createTaskOffice({
      backend: guardTaskBackend(remote, gate),
      detail: remote,
      interactions: guardInteractionBackend(remote, gate),
      lease: () => runtime.scopeLease(),
      store: createVaultTaskProjectionStore({ vault, lease: () => runtime.scopeLease() }),
    });
    const agentsEnvelope = await runtime.authorizedRequest({ method: 'GET', path: '/api/v1/agents' }) as { success?: boolean; data?: Array<{ id?: unknown }> };
    const agentId = typeof agentsEnvelope?.data?.[0]?.id === 'string' ? agentsEnvelope.data[0].id : undefined;
    if (!agentId) {
      evidence.errorReason = 'no agent available on the deployment';
      return evidence;
    }
    // ① 联网 + 用户确认提交：真实 Start（新意图新 request_id）
    const goal = `T10 集成验证：${new Date().toISOString()}`;
    const receipt = await office.start({ text: goal, agentId, budgetUpper: 10 });
    evidence.onlineStart = receipt.phase === 'bound' ? 'admitted' : receipt.phase === 'rejected' ? 'rejected' : 'pending';
    if (receipt.phase !== 'bound' || receipt.runId === undefined) {
      evidence.errorReason = `receipt phase ${receipt.phase}`;
      return evidence;
    }
    // ② 在线 hydrate：投影（含离线快照）经 Scoped Vault 加密落盘
    const page = await office.tasks({});
    const card = page.items.find((item) => item.runId === receipt.runId);
    if (card === undefined) {
      evidence.errorReason = 'created run not visible in the task list';
      return evidence;
    }
    const handle = office.open({ taskId: card.taskId, runId: receipt.runId! });
    await handle.hydrate();
    // 加密证据：vault 存储层除明文索引行（'["run.<id>"]' 属预期）外全部是 base64 密文。
    // 不用 includes(taskId)——数字 taskId 作子串在 base64 中误匹配率高（scoped-vault.test.ts:40-50 同款 base64 断言）。
    const ciphertextRows = [...storage.entries().values()].filter((value) => !value.startsWith('['));
    evidence.ciphertextProjectionRows = ciphertextRows.length > 0 && ciphertextRows.every((value) => /^[A-Za-z0-9+/]+={0,2}$/.test(value));
    // ③ 断网：四类危险动作全部 fail closed（run=真实 Start 重放经 guarded office；
    //    approval=guarded decide；budget/external-action=gate 通道——两类的服务端入口属 #39/#48/#51）
    networkOnline = false;
    const blocked: string[] = [];
    try {
      await office.start({ text: `${goal} (offline replay)`, agentId, budgetUpper: 10 });
    } catch (error) { if (offlineBlockOf(error) === 'run') blocked.push('run'); }
    try {
      await office.decide({
        item: { interactionId: 'probe', runId: receipt.runId!, kind: 'tool_approval', argsHash: '', expectedRevision: 0, createdAt: new Date().toISOString() },
        action: 'approve',
      });
    } catch (error) { if (offlineBlockOf(error) === 'approval') blocked.push('approval'); }
    try { await gate.assertOnline('budget'); } catch (error) { if (offlineBlockOf(error) === 'budget') blocked.push('budget'); }
    try { await gate.assertOnline('external-action'); } catch (error) { if (offlineBlockOf(error) === 'external-action') blocked.push('external-action'); }
    evidence.offlineBlockedActions = blocked;
    // ④ 离线草稿往返（加密保存于当前 scope）
    const lease = runtime.scopeLease()!;
    const drafts = await vault.open(lease);
    await drafts.drafts.put({ id: 'new-task', body: JSON.stringify({ text: goal }) });
    evidence.offlineDraftRoundTrip = (await drafts.drafts.get('new-task'))?.body.includes(goal) === true;
    // ⑤ 离线查看获准 Task 内容：detail 通道断开 → 加密投影降级视图
    try {
      const offlineView = await handle.resync();
      evidence.offlineDetailView = offlineView.interruption?.reason === 'offline' && offlineView.taskId === card.taskId ? 'projected' : 'unavailable';
    } catch {
      evidence.offlineDetailView = 'unavailable';
    }
    // ⑥ 恢复联网：显式 resync 恢复权威同步（不自动重放）
    networkOnline = true;
    try {
      const recovered = await handle.resync();
      evidence.onlineResync = recovered.interruption?.reason === 'offline' ? 'failed' : 'recovered';
    } catch {
      evidence.onlineResync = 'failed';
    }
    handle.close('integration-complete');
    return evidence;
  } catch (error) {
    evidence.errorReason = error instanceof Error ? error.message : String(error); // 失败仍产出证据（不含凭据）
    return evidence;
  } finally {
    runtime.dispose(); // 释放 lease/凭据通道并撤销 vault scope（撤权 fail closed 的收尾）
  }
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitOfflineVaultIntegrationEvidence(evidence: OfflineVaultIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
```

（凭据纪律：email/password 只流经 config 与 runtime signIn，绝不进入证据对象——与 task-start-integration-smoke.ts 同口径。）

- [ ] **Step 4: 运行测试确认通过**

Run: `cd apps/mobile && npx tsx --test src/offline-vault-integration-smoke.test.ts && cd ../..`
Expected: PASS（无环境变量时前两个用例绿、live 用例 skip；`pnpm --filter @weknora/mobile test` 的 glob 亦覆盖本文件）。有真实凭据时（`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD`，公网 HTTPS 主机）live 用例执行并产出 AC3 证据。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/offline-vault-integration-smoke.ts apps/mobile/src/offline-vault-integration-smoke.test.ts
git commit -m "test(mobile): end-to-end offline cache, gate and confirmation evidence (T10 #40 AC3)"
```

---

## 计划级验证命令（testCommand）

在 worktree 根执行（覆盖本计划全部测试：mobile-core 四组定向 + offline 目录 + apps/mobile 全量与 typecheck；`@weknora/mobile test` 的 glob `src/**/*.test.ts*` 含集成冒烟与新测试）：

```bash
npx tsx --test packages/mobile-core/src/vault/scoped-vault.test.ts packages/mobile-core/src/runtime/runtime-vault.test.ts packages/mobile-core/src/task-office/task-detail.test.ts packages/mobile-core/src/task-office/vault-projection-store.test.ts && npx tsx --test 'packages/mobile-core/src/offline/*.test.ts' && pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck
```

真机构建前置（Node 测试链不依赖；一次性）：`cd apps/mobile && npx expo install expo-network`。

## blocked-env 与已知限制（如实声明，不伪造通过）

1. **真机持久化的存储选型 ADR 未决（B2-F23 延期项）**：`createVaultTaskProjectionStore` 的行预算 1400B 让 SecureStore Adapter 可承载「状态快照 + 最近事件」的离线视图；完整 200 事件投影需 Expo SQLite + encryption Adapter（module-seams §9.3「Encrypted Database Port」），**该选型不在本计划内静默决定**——离线时间线深度受预算约束，列为 ADR 待决项。本地可验证替代证据：Task 3 预算裁剪测试 + Task 7 `offlineDetailView='projected'`（in-memory keyStore/storage Adapter 上的真实 AES-GCM vault 行为）。
2. **AC3 live 证据需真实部署凭据**（`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD`，公网 HTTPS 主机，过 `disallowedDeploymentHost` 防线）：无凭据环境按 opt-in skip 语义跳过（`disposition: 'skip'`），不伪造；断网语义在集成中以 fetcher 注入模拟（物理断网不可在 CI 编排），网络状态端口 `NetworkStatusPort` 的真机行为（`expo-network` `isInternetReachable`）属真机验收（iOS #69 / Android #70）。
3. **`budget`/`external-action` 两类危险动作无服务端入口**（#39 扩额、#48/#51 外部发布未实现）：gate 的 `assertOnline('budget')/('external-action')` 已交付并测试（Task 4），集成证据经 gate 通道如实记录（Task 7 ③），不构造不存在的 wire 调用。
4. **语音/批注（annotations）不在本计划**：批注 UI 属 #47（#46 边界声明）；草稿通道（drafts 仓储 + New 屏）即 What to build 的「保存草稿」面。

## 差异记录（调查结论 vs 代码现状）

- 调查摘要称 scope key 为 `weknora.vault.v1.<origin>.<userId>.<tenantId>`——**代码现状为 v2 摘要键** `weknora.vault.v2.<sha256前20字节hex>`（scoped-vault.ts:41-49，R1-F39/F15 修复；v1 键从未在 Android 成功写入，无迁移负担）。本计划以代码现状为准。
- 调查摘要称「撤权/退出 fail closed 无任何实现」——**vault 面已由 #32 交付**（mobile-runtime.ts:116-135 的 `queueVaultRevoke` + scoped-vault revoke 先覆写 key 再删行），本计划只补 projections 仓储的同等覆盖（Task 1 测试）与离线门（Task 4），不重复实现。
- `packages/domain/src/mobile/execution-cache.ts` 仍存在（scope 键投影、无加密）——它是 domain 层遗留纯投影，非本计划改动面；module-seams §5.4 的「Task Store Port：Scoped Vault Adapter」由 Task 3 交付后，该文件的收编属后续清理（spec Testing Decisions「redundant shallow white-box tests should be removed」的既定节奏），不在本计划。

## 自我审查记录（writing-plans 四项检查）

1. **Spec 覆盖**：AC1（缓存按 Deployment/用户/Tenant 加密隔离）→ Task 1（projections 仓储 + scope 隔离/密文/revoke 测试）+ Task 3（vault Adapter）+ Task 6（组合根）+ Task 7（`ciphertextProjectionRows`/`offlineDetailView`）；AC2（撤权、退出、磁盘失败和离线危险动作均 fail closed）→ 撤权/退出：Task 1 revoke 测试 + Task 3 撤权 lease 测试（#32 既有 runtime-vault 回归）；磁盘失败：Task 1 既有损坏索引/密文测试先例 + Task 3 篡改行测试 + Task 2 persist 失败语义既有；离线危险动作：Task 4 全四类 + Task 5 New 屏门 + Task 7 集成；AC3（端到端最高稳定 Interface）→ Task 7 真实部署 opt-in 证据（真实 transport/Runtime/vault/office，mock 不冒充）。What to build 三段（离线查看、草稿+联网确认、禁止危险动作）分别由 Task 2/5、Task 5+7①、Task 4/5/6/7③ 覆盖。Testing Decisions 的「Scoped Vault Interface tests … offline drafts and rejection of offline side effects」由 Task 1/4 测试集覆盖。
2. **占位符扫描**：全文无 TBD/TODO/「稍后实现」；所有代码步骤给出完整代码（Task 5 Step 3d 的「……（既有 start/回执处理不变）」处给出了插入点上下文与完整新段，既有段为 #36 已集成代码不需重抄——插入位置以行号锚定）。审查过程中修正的七处（均已落到正文）：Task 3 畸形 runId 断言改构造畸形 `runId`、Task 3 撤权测试区分「lease 撤销」与「vault revoke」两个语义（lease 撤销只断 VAULT_LEASE；行擦除显式执行 Runtime 的 `vault.revoke` 序列）、Task 3 `parseProjection` 的 `taskId` 收紧为必填、Task 4 `decide` 装饰器签名显式化、Task 5 三个测试补 readiness 前提（supported general AgentOption + `update({text})`，否则 submit 在 text/agentId 检查处早退、离线门不可达）、Task 5 第二个测试 start 改成功回执（成功路径清除 offline 标记）、Task 7 密文断言改 base64 模式（数字 taskId 作 `includes` 子串在 base64 中误匹配率高）。
3. **类型一致性**：`ScopedDraftRepository`（Task 1 产出即 Task 3 消费的 `store.projections.put/get` 签名一致）；`OfflineTaskSnapshot`（Task 2 定义 = Task 3 测试 `projection$().snapshot` 字段集逐字一致：taskId/runId/title/attention/archivedAt?/execution/watermark/incomplete）；`NetworkStatusPort`（Task 4 定义 = Task 5 `createNativeNetworkStatusIfAvailable` 返回类型与 new-task-view 端口一致）；`OfflineGateError.action`（Task 4 = Task 7 `offlineBlockOf` 判定）；`createVaultTaskProjectionStore` 参数名 `vault`/`lease`（Task 3 = Task 6/7 调用逐字一致）；`guardTaskBackend(remote, nativeOfflineGate)` 等三装饰器签名（Task 4 = Task 6/7 调用）。
4. **Review Focus 落实**：#1 断网连击→Task 4 零触达断言 + Task 7 ③；#2 投影超预算→Task 3 裁剪测试；#3 撤权后离线读→Task 1 revoke 测试 + Task 3 撤权 lease 测试 + Task 2 撤权不降级测试；#4 离线视图误当权威→Task 2 三测试（offline 标记/resync 恢复/旧格式不降级）；#5 磁盘损坏/篡改→Task 3 篡改行测试 + Task 1 既有密文测试集。
