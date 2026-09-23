# OCR 修复批次 Round 1 实施计划（issue30-sweep）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 OCR 第 1 轮审查的 24 条有效发现（其中 23 条纳入修复、R1-F26 需领域决策排除）与 20 条 lowWorthFixing 中的 18 条顺手项，按根因聚合为 13 个任务修复：Vault 键布局与会话/互斥健壮化、task-detail 流式守卫、组合根 remount 与显式装配、shelf 代次保护、runtime/传输卫生、iOS 插件合同对齐。

**Architecture:** 全部为存量代码修复，不新增子系统。Vault 侧先重定义键布局（Task 1），再叠加会话 single-flight（Task 2）、per-scope 互斥队列（Task 3）、行绑定与 TTL（Task 4）；task-office 侧先解运行时循环依赖并统一错误契约（Task 5），再补流式守卫（Task 6）、写放大治理（Task 7）、列表失效与显式装配（Task 8）；其余为相互独立的 composition / shelf / runtime / iOS 插件小批次。

**Tech Stack:** TypeScript（node:test + tsx runner）、React Native + Expo（config-plugins CommonJS）、AES-GCM（WebCrypto）。

**Spec:** 本计划实现 OCR Round 1 审查发现清单（本文件「发现清单与分组」节，全文转录自审查输出）；领域术语与生命周期约束以 `CONTEXT.md` 为准；模块合同以各文件头部注释为准（`task-office.ts`、`task-timeline.ts`、`scoped-vault.ts`）。相关背景报告：`docs/plans/issue30-sweep/ocr/ocr-round-1.md`、`docs/plans/issue30-sweep/ocr/ocr-increment-batch2.md`（如存在）。

## Global Constraints

- 测试运行器为 node:test + tsx；mobile-core 单文件命令统一用 `pnpm --filter @weknora/mobile-core exec tsx --test <file>`，apps/mobile 用 `pnpm --filter @weknora/mobile test`，domain 单文件用 `pnpm exec tsx --test packages/domain/src/mobile/resource-presentation.test.ts`（在仓库根执行）。
- 不引入新依赖、新包；修复只落在既有文件（每个任务的 Files 节白名单之外不改）。
- 错误码命名沿用既有 `VAULT_*` / `TASK_OFFICE_*` / `RUNTIME_*` 前缀风格；不发明与服务端合同不一致的状态值（settlement 推进必须镜像 `internal/application/repository/agent_run_snapshot.go:83-88`）。
- 本批次不改 `CONTEXT.md` 领域模型（生命周期四态），需领域决策的发现（R1-F26）明确排除（见「排除项」）。
- 每任务一个 commit，遵循仓库 message 风格（`fix(scope): ...` / `test(scope): ...`）。
- Vault 键布局升级为 v2（`weknora.vault.v2.*`）：该路径在 Android 上因 R1-F39 从未成功写入过、分支未发布，v1 键下无生产数据，不做迁移（决策记录见 Task 1）。

## Review Focus

1. **损坏索引下的 revoke 幂等**：`revoke()` 在 `readIndex` 抛 `VAULT_INDEX`（Task 3 引入）时不得半途留下密文行——revoke 必须吞索引错误并仍尽力删 wrapped key 与索引键。测试归 Task 3 Step 6。
2. **id 边界**：64 字符 id 仍可写；超 64、含 `/`、含空串 id 一律 `VAULT_ID`（get/remove 同样拒绝）。测试归 Task 1 Step 2。
3. **store.save 持续失败且自动 resync 已耗尽时视图仍可观察**：订阅者收到 `persist-failed` interruption，不静默。测试归 Task 7 Step 2。
4. **切部署时 vault revoke 排队与新登录 open 的竞争**：旧 scope 的行/键删除失败不得把旧 scope 键残留为新 scope 可读（scopeKey 摘要不同域，断言新 scope 读不到旧草稿）。测试归 Task 1 Step 5。
5. **iOS 插件幂等**：对已改写过的 AppDelegate/Podfile 二次运行插件不重复注入 scene delegate / clamp 块。测试归 Task 13 Step 2。

---

## 发现清单与分组（根因聚合）

| 组 | 任务 | 聚合的发现 | 根因摘要 |
|---|---|---|---|
| Vault | Task 1 | R1-F39(high)、R1-F10(high)、R1-F40、R1-F15(顺手) | 键布局未定义保留字/字符集/分段无歧义编码 |
| Vault | Task 2 | R1-F41、R1-F13、R1-F18(顺手)、R1-F16a(顺手) | 会话创建无 single-flight、keystore 异常静默降级 |
| Vault | Task 3 | R1-F12、R1-F11、R1-F16b(顺手) | 仓储操作无互斥、索引读写不容错、rotate 假原子 |
| Vault | Task 4 | R1-F42(顺手)、R1-F14(顺手)、R1-F17 | 行未与 id 绑定、保留策略无清理、行大小不受控 |
| Task Office | Task 5 | R1-F21、R1-F22 | 运行时循环值导入、跨包错误契约靠 message 文本 |
| Task Office | Task 6 | R1-F44、R1-F43、R1-F24、R1-F27(顺手)、R1-F45(顺手) | chain 上异步步骤缺统一 lease/epoch 复验与订阅者隔离 |
| Task Office | Task 7 | R1-F23 | 每事件全量 persist 的写放大与无背压队列 |
| Task Office | Task 8 | R1-F19、R1-F20 | 写路径不作废在途读、组合根静默回退默认 store |
| Composition | Task 9 | R1-F48(high)、R1-F49(high) | remount key 缺 deployment origin 维度 |
| Shelf | Task 10 | R1-F2、R1-F36(顺手)、R1-F35(顺手)、R1-F3(顺手) | browse 写共享快照无代次、共享可变 verdict、脏行卫生 |
| Runtime | Task 11 | R1-F46、R1-F31(顺手)、R1-F33(顺手)、R1-F47(顺手) | 同 origin 短路过宽、await 后不复核 epoch、失败耦合 |
| Runtime | Task 12 | R1-F50、R1-F32(顺手)、R1-F37(顺手)、R1-F34(顺手) | 失败响应泄漏连接、scope 变化不 abort 在途流、防线缺口 |
| iOS | Task 13 | R1-F5、R1-F6、R1-F7(顺手)、R1-F8(顺手) | 冷启动 universal link 丢投递、配置声明被硬编码覆盖、幂等/解析脆弱 |

**排除项（不纳入本批次，理由）：**
- **R1-F26**（failed run 映射 lifecycle=active）：`CONTEXT.md:81` 生命周期四态（进行中/已完成/已取消/已归档）未定义 failed 的归属，`CONTEXT.md:209` 明言运行状态不直接决定任务生命周期——机械改动等于静默重设领域决策。按仓库冲突规则应回 Matt `domain-modeling` 更新 CONTEXT.md/ADR 后再实现。推荐方向（供决策）：lifecycle 增加 `'failed'` 态，或在 timeline 摘要层表达「任务未能完成」。
- **R1-F1**（两处 snapshot stub 重复，报告称 132/204 行）：本会话无法定位——`apps/mobile/src/resources-view.test.ts` 仅 104 行，`app-smoke.test.tsx` 对应行无完整载荷 stub。证据行号已漂移，留待重新核对后再修。
- **R1-F28**（中文文案内嵌 mobile-core）：i18n/文案下沉是独立重构，非缺陷修复。
- **R1-F20 的完整持久化 TaskProjectionStore 与 R1-F17 的 SQLite 行存储后端**：依赖 SQLite 后端选型且 Task 7 必须先行（否则持久 store 接入后写放大落地）。本批次做显式装配与受控失败，完整实现留 Round 2。

---

### Task 1: Vault 键布局 v2 —— scopeKey 字符集、索引/行命名空间分离、id 校验对称

**Files:**
- Modify: `packages/mobile-core/src/vault/scoped-vault.ts:6`（DRAFT_ID_PATTERN 及 id 校验）、`:33-35`（scopeKeyOf）、`:83-84`（indexKey/rowKey）、`:126-153`（put/get/remove 校验点）
- Test: `packages/mobile-core/src/vault/scoped-vault.test.ts`

**Interfaces:**
- Consumes: 既有 `ScopedVaultPorts`、`RuntimeScopeLease`。
- Produces: `scopeKeyOf(scope): string` 签名不变，输出变为 `weknora.vault.v2.<hex40>`（SHA-256 前 40 个 hex 字符，仅 `[0-9a-f]`）；内部新增 `draftKeyId(id): string`（id 校验 + base64url 编码，供 Task 3/4 复用）；新增导出错误码语义 `VAULT_ID`（get/remove/put 一致抛出）。

**决策记录（数据兼容）**：键布局从 `weknora.vault.v1.<encodeURIComponent 分段>` 升级为 v2 摘要键。v1 键含 `%`（encodeURIComponent 对 `:` `/` 的转义），在 Android expo-secure-store 上 `writeWrappedKey` 直接抛错（R1-F39），即 Android 从未成功落盘 v1 数据；iOS 侧无字符限制但本分支未发布、drafts 仓储无 UI 消费方（仅测试引用）。因此 v1 键下数据视为不存在，不做双读迁移。

- [ ] **Step 1: 写失败测试（键冲突与 id 校验不对称——R1-F10/F40 核心）**

追加到 `scoped-vault.test.ts`：

```typescript
test('reserved index row cannot be hijacked: put/get/remove validate ids symmetrically', async () => {
  const { vault: scoped } = vault();
  const store = await scoped.open(lease());
  // R1-F10：id=index 不得命中索引键；R1-F40：get/remove 与 put 校验对称
  await assert.rejects(store.drafts.put({ id: 'index', body: 'x' }), /VAULT_ID/);
  await assert.rejects(store.drafts.get('index'), /VAULT_ID/);
  await assert.rejects(store.drafts.remove('index'), /VAULT_ID/);
  await assert.rejects(store.drafts.put({ id: 'a/b', body: 'x' }), /VAULT_ID/); // 非法字符
  await assert.rejects(store.drafts.get(''), /VAULT_ID/); // 空串
  const longId = 'x'.repeat(65);
  await assert.rejects(store.drafts.put({ id: longId, body: 'x' }), /VAULT_ID/); // 超 64
  await store.drafts.put({ id: 'y'.repeat(64), body: 'ok' }); // 上限仍可用
  assert.equal((await store.drafts.get('y'.repeat(64)))?.body, 'ok');
});

test('scope keys are SecureStore-safe and collision-free across dotted inputs (R1-F39/F15)', async () => {
  // origin 含点号/冒号：v1 分段编码会拼出相同 scopeKey；v2 摘要不冲突且仅含 [0-9a-f.]
  const a = scopeKeyOf({ deploymentOrigin: 'https://a.b.test', userId: 'u.1', tenantId: '7' });
  const b = scopeKeyOf({ deploymentOrigin: 'https://a.test', userId: 'b.u.1', tenantId: '7' });
  assert.notEqual(a, b);
  assert.match(a, /^weknora\.vault\.v2\.[0-9a-f]+$/);
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile-core exec tsx --test src/vault/scoped-vault.test.ts`
Expected: 新增两条 FAIL（`index` 写入当前成功 / scopeKey 含 `%`）。

- [ ] **Step 3: 实现 v2 键布局**

`scoped-vault.ts` 修改要点（完整替换相关函数）：

```typescript
const DRAFT_ID_PATTERN = /^[A-Za-z0-9._-]{1,64}$/;

export function scopeKeyOf(scope: LeaseScope): string {
  // v2：分段拼接后取 SHA-256 hex 摘要。动机：(1) v1 的 encodeURIComponent 产物含 %，
  // Android expo-secure-store 键字符集（[A-Za-z0-9._-]）直接拒绝（R1-F39）；
  // (2) 点号分段在段值含点时可拼出相同键（R1-F15）。摘要同时解决两者且长度固定。
  const text = new TextEncoder().encode(`${scope.deploymentOrigin}\n${scope.userId}\n${scope.tenantId}`);
  const digest = new Uint8Array(await crypto.subtle.digest('SHA-256', text));
  return `weknora.vault.v2.${[...digest].slice(0, 20).map((b) => b.toString(16).padStart(2, '0')).join('')}`;
}
```

注意：`scopeKeyOf` 变 async 后，所有调用点（`open`/`rotate`/`revoke` 内 4 处）加 `await`；`index.ts` 已导出该函数，签名变更需同步其类型面。索引/行键分离（id 经 base64url 编码，天然落在 id 模式之外的命名空间）：

```typescript
const indexKey = (scopeKey: string): string => `${scopeKey}.ix`;
const rowKey = (scopeKey: string, id: string): string => `${scopeKey}.d.${draftKeySegment(id)}`;
const draftKeySegment = (id: string): string => btoa(id).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
const assertDraftId = (id: string): void => {
  if (!DRAFT_ID_PATTERN.test(id)) throw new Error('VAULT_ID');
};
```

`put`/`get`/`remove` 入口均先 `assertDraftId(id)`（put 已有，get/remove 新增）。

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --filter @weknora/mobile-core exec tsx --test src/vault/scoped-vault.test.ts`
Expected: 全部 PASS。

- [ ] **Step 5: 跨 scope 隔离回归（Review Focus #4）**

```typescript
test('revoke of one scope never leaks rows into another scope key', async () => {
  const { vault: scoped } = vault();
  const a = await scoped.open(lease());
  await a.drafts.put({ id: 'd1', body: 'secret-a' });
  const otherScope = { deploymentOrigin: 'https://other.example.test', userId: 'user-1', tenantId: '7' };
  const b = await scoped.open(lease(otherScope));
  await b.drafts.put({ id: 'd1', body: 'secret-b' });
  const revoked = lease(); // 新 lease（scope A）
  await scoped.revoke(revoked, 'dispose');
  assert.equal(await b.drafts.get('d1') !== undefined && (await b.drafts.get('d1'))?.body === 'secret-b', true);
});
```

Run: 同 Step 4。Expected: PASS（既有隔离测试 + 本条均绿）。

- [ ] **Step 6: Commit**

```bash
git add packages/mobile-core/src/vault/scoped-vault.ts packages/mobile-core/src/vault/scoped-vault.test.ts packages/mobile-core/src/index.ts
git commit -m "fix(mobile-core/vault): v2 scope keys, split index/row namespaces, symmetric id validation (R1-F39/F10/F40/F15)"
```

---

### Task 2: requireSession single-flight + wrapped key 异常 fail-closed + base64 工具统一

**Files:**
- Modify: `packages/mobile-core/src/vault/scoped-vault.ts:37-48`（base64 工具移出）、`:68-82`（requireSession）
- Create: `packages/mobile-core/src/vault/base64.ts`
- Modify: `apps/mobile/src/adapters/vault-adapters.ts:4-15`（复用共享工具）、`:20`（损坏值受控错误）
- Test: `packages/mobile-core/src/vault/scoped-vault.test.ts`、`apps/mobile/src/adapters/vault-adapters.test.ts`（新建）

**Interfaces:**
- Produces: `packages/mobile-core/src/vault/base64.ts` 导出 `bytesToBase64(bytes: Uint8Array): string`、`base64ToBytes(value: string): Uint8Array`；`index.ts` 不导出（包内部）；`requireSession` 返回 `Promise<ScopeSession>` 且同一 scopeKey 并发调用返回同一 Promise；wrapped key 存在但长度 ≠ 32 时抛 `VAULT_KEYSTORE`。

- [ ] **Step 1: 写失败测试（R1-F41 single-flight / R1-F13 长度异常）**

```typescript
test('concurrent first open shares one key (single-flight, R1-F41)', async () => {
  const { keyStore, storage, vault: scoped } = vault();
  let reads = 0;
  const slowKeyStore = {
    ...keyStore,
    readWrappedKey: async (k: string) => { reads += 1; await new Promise((r) => setTimeout(r, 5)); return keyStore.readWrappedKey(k); },
    writeWrappedKey: async (k: string, v: Uint8Array) => { await new Promise((r) => setTimeout(r, 5)); return keyStore.writeWrappedKey(k, v); },
  };
  const slow = createScopedVault({ keyStore: slowKeyStore, storage, cipher: createWebCryptoCipher() });
  await Promise.all([slow.open(lease()), slow.open(lease())]);
  await (await slow.open(lease())).drafts.put({ id: 'a', body: 'persisted' });
  const reopened = await slow.open(lease());
  assert.equal((await reopened.drafts.get('a'))?.body, 'persisted'); // 未持久化的 K1 不会再出现
  assert.ok(reads >= 1);
});

test('a wrapped key of wrong length fails closed instead of being overwritten (R1-F13)', async () => {
  const { keyStore, storage } = vault();
  const key = await scopeKeyOf(SCOPE_A);
  await keyStore.writeWrappedKey(key, new Uint8Array(31)); // 持久化值损坏/长度异常
  const scoped = createScopedVault({ keyStore, storage, cipher: createWebCryptoCipher() });
  await assert.rejects(scoped.open(lease()), /VAULT_KEYSTORE/);
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile-core exec tsx --test src/vault/scoped-vault.test.ts`
Expected: 两条新测试 FAIL（当前并发 open 各持密钥 / 长度异常静默生成新 key）。

- [ ] **Step 3: 实现**

`scoped-vault.ts`：

```typescript
import { base64ToBytes, bytesToBase64 } from './base64.ts';
// 删除本文件内的 37-48 行重复实现（R1-F16a）

const sessionFlights = new Map<string, Promise<ScopeSession>>();
const requireSession = async (scopeKey: string): Promise<ScopeSession> => {
  const existing = sessions.get(scopeKey);
  if (existing && !existing.destroyed) return existing;
  const inFlight = sessionFlights.get(scopeKey);
  if (inFlight) return inFlight;
  const flight = (async (): Promise<ScopeSession> => {
    const wrapped = await ports.keyStore.readWrappedKey(scopeKey);
    if (wrapped !== undefined) {
      if (wrapped.length !== KEY_LENGTH) throw new Error('VAULT_KEYSTORE'); // R1-F13：覆写 = 旧密文永久不可解且无告警
      const session: ScopeSession = { key: wrapped, destroyed: false };
      sessions.set(scopeKey, session);
      return session;
    }
    const key = randomBytes(KEY_LENGTH);
    await ports.keyStore.writeWrappedKey(scopeKey, key);
    const session: ScopeSession = { key, destroyed: false };
    sessions.set(scopeKey, session);
    return session;
  })();
  sessionFlights.set(scopeKey, flight);
  try { return await flight; } finally { sessionFlights.delete(scopeKey); }
};
```

`vault-adapters.ts`：删除本地 `bytesToBase64/base64ToBytes`，改从 `@weknora/mobile-core` 导入（`index.ts` 追加 `export { bytesToBase64, base64ToBytes } from './vault/base64.ts';`）；`readWrappedKey` 捕获解码失败并抛受控错误（R1-F18）：

```typescript
import { base64ToBytes, bytesToBase64 } from '@weknora/mobile-core';
// ...
async readWrappedKey(scopeKey) {
  const raw = await store.getItemAsync(scopeKey);
  if (raw === null) return undefined;
  try { return base64ToBytes(raw); } catch { throw new Error('VAULT_KEYSTORE'); } // 损坏值不得当作“无 key”触发新生成
},

- [ ] **Step 4: 适配层测试 + 全量通过**

新建 `apps/mobile/src/adapters/vault-adapters.test.ts`：

```typescript
import test from 'node:test';
import assert from 'node:assert/strict';
import { createSecureVaultKeyStore } from './vault-adapters.ts';

const memory = () => {
  const map = new Map<string, string>();
  return { getItemAsync: async (k: string) => map.get(k) ?? null, setItemAsync: async (k: string, v: string) => { map.set(k, v); }, deleteItemAsync: async (k: string) => { map.delete(k); } } as const;
};

test('a corrupt wrapped-key value fails closed with VAULT_KEYSTORE (R1-F18)', async () => {
  const store = memory();
  await store.setItemAsync('scope-key', '!!!not-base64!!!');
  const keyStore = createSecureVaultKeyStore(store as never);
  await assert.rejects(keyStore.readWrappedKey('scope-key'), /VAULT_KEYSTORE/);
});
```

Run: `pnpm --filter @weknora/mobile-core exec tsx --test src/vault/scoped-vault.test.ts && pnpm --filter @weknora/mobile test`
Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/vault/base64.ts packages/mobile-core/src/vault/scoped-vault.ts packages/mobile-core/src/vault/scoped-vault.test.ts packages/mobile-core/src/index.ts apps/mobile/src/adapters/vault-adapters.ts apps/mobile/src/adapters/vault-adapters.test.ts
git commit -m "fix(mobile-core/vault): single-flight sessions, fail-closed keystore anomalies, shared base64 utils (R1-F41/F13/F18/F16a)"
```

---

### Task 3: per-scope 互斥队列 + readIndex/rotate 健壮化 + list/revoke 并行化

**Files:**
- Modify: `packages/mobile-core/src/vault/scoped-vault.ts:85-91`（readIndex/writeIndex）、`:119-193`（open/rotate/revoke 整体入队）、`:140-148`（list 并行）、`:185-186`（revoke 并行）
- Test: `packages/mobile-core/src/vault/scoped-vault.test.ts`

**Interfaces:**
- Produces: 内部 `enqueue(scopeKey, action)`（per-scope Promise 队列）；`readIndex` 对损坏 JSON/非法结构抛 `VAULT_INDEX`（不再静默 `[]`）；`rotate` 持锁执行且重封阶段失败时 best-effort 回滚。

- [ ] **Step 1: 写失败测试（R1-F12 并发覆盖 + 索引损坏；R1-F11 rotate/put 互斥）**

```typescript
test('concurrent puts do not lose index entries (R1-F12)', async () => {
  const { vault: scoped } = vault();
  const store = await scoped.open(lease());
  await Promise.all(Array.from({ length: 20 }, (_, i) => store.drafts.put({ id: `d${i}`, body: `b${i}` })));
  const ids = (await store.drafts.list()).map((e) => e.id).sort();
  assert.deepEqual(ids, Array.from({ length: 20 }, (_, i) => `d${i}`).sort());
});

test('a corrupt index fails closed instead of being silently orphaned (R1-F12)', async () => {
  const { keyStore, storage, vault: scoped } = vault();
  const store = await scoped.open(lease());
  await store.drafts.put({ id: 'd1', body: 'x' });
  const scopeKey = await scopeKeyOf(SCOPE_A);
  await storage.write(`${scopeKey}.ix`, '{not-json');
  await assert.rejects(store.drafts.list(), /VAULT_INDEX/);
  await assert.rejects(store.drafts.put({ id: 'd2', body: 'y' }), /VAULT_INDEX/); // 不得覆写索引孤儿化全部行
});

test('rotate holds the scope lock against concurrent puts (R1-F11)', async () => {
  const { vault: scoped } = vault();
  const store = await scoped.open(lease());
  await store.drafts.put({ id: 'd1', body: 'v1' });
  const rotated = scoped.rotate(lease());
  const put = store.drafts.put({ id: 'd2', body: 'v2' });
  await Promise.all([rotated, put]);
  assert.equal((await store.drafts.get('d1'))?.body, 'v1');
  assert.equal((await store.drafts.get('d2'))?.body, 'v2');
  const reopened = await scoped.open(lease()); // 新会话强制重读 keyStore
  assert.equal((await reopened.drafts.list()).length, 2); // 重启后无 VAULT_DECRYPT
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile-core exec tsx --test src/vault/scoped-vault.test.ts`
Expected: 三条 FAIL。

- [ ] **Step 3: 实现互斥队列与健壮化**

```typescript
const scopeTails = new Map<string, Promise<unknown>>();
const enqueue = <T>(scopeKey: string, action: () => Promise<T>): Promise<T> => {
  const tail = scopeTails.get(scopeKey) ?? Promise.resolve();
  const next = tail.then(action, action);
  scopeTails.set(scopeKey, next.catch(() => undefined));
  return next;
};

const readIndex = async (scopeKey: string): Promise<string[]> => {
  const raw = await ports.storage.read(indexKey(scopeKey));
  if (raw === null) return [];
  let value: unknown;
  try { value = JSON.parse(raw); } catch { throw new Error('VAULT_INDEX'); }
  if (!Array.isArray(value) || !value.every((id) => typeof id === 'string')) throw new Error('VAULT_INDEX');
  return value;
};
```

装配方式：`open()` 返回的 `drafts` 四方法体整体包进 `enqueue(scopeKey, ...)`；`rotate`/`revoke` 同样入队（注意 revoke 里的 `requireSession` 不走队列、直接读 sessions）。`rotate` 重封循环失败时回滚：

```typescript
const rewritten: Array<{ id: string; entry: DraftEntry }> = [];
try {
  for (const { id, entry } of decrypted) {
    await ports.storage.write(rowKey(scopeKey, id), await sealRow(newKey, entry));
    rewritten.push({ id, entry });
  }
} catch (cause) {
  // best-effort 回滚至旧 key；回滚再失败则该 scope fail-closed（session 置 destroyed），不留半状态
  try {
    for (const { id, entry } of rewritten) await ports.storage.write(rowKey(scopeKey, id), await sealRow(oldKey, entry));
  } catch {
    if (session && !session.destroyed) session.destroyed = true;
    sessions.delete(scopeKey);
  }
  throw cause instanceof Error ? cause : new Error('VAULT_ROTATE', { cause });
}
```

`list()` 行读取改 `Promise.all(ids.map(...))`（单行失败仍按整体 reject）；`revoke()` 删行循环同样并行化（R1-F16b）。

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --filter @weknora/mobile-core exec tsx --test src/vault/scoped-vault.test.ts`
Expected: 全部 PASS（既有 rotate/revoke 测试不回归）。

- [ ] **Step 5: revoke 对损坏索引幂等（Review Focus #1）**

```typescript
test('revoke tolerates a corrupt index and still erases key material', async () => {
  const { storage, vault: scoped } = vault();
  const store = await scoped.open(lease());
  await store.drafts.put({ id: 'd1', body: 'x' });
  const scopeKey = await scopeKeyOf(SCOPE_A);
  await storage.write(`${scopeKey}.ix`, '{corrupt');
  await scoped.revoke(lease(), 'dispose');
  assert.equal(await storage.read(`${scopeKey}.ix`), null);
  assert.equal(await storage.read(`${scopeKey}.d.${btoa('d1').replace(/=+$/, '')}`), null);
});
```

（`revoke` 实现中 `readIndex` 的调用改为 `await readIndex(scopeKey).catch(() => [] as string[])`；btoa 在测试里可用 `Buffer` 替代以避免 Node 环境差异：`Buffer.from('d1', 'binary').toString('base64')`。）

Run: 同上。Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add packages/mobile-core/src/vault/scoped-vault.ts packages/mobile-core/src/vault/scoped-vault.test.ts
git commit -m "fix(mobile-core/vault): per-scope op serialization, fail-closed index, locked rotate with rollback (R1-F12/F11/F16b)"
```

---

### Task 4: 行-id 绑定校验 + 30 天保留清理 + 行大小受控失败

**Files:**
- Modify: `packages/mobile-core/src/vault/scoped-vault.ts:96-109`（openRow）、`:140-148`（list 惰性 GC）、`:7`（RETENTION_DAYS 已有）
- Modify: `apps/mobile/src/adapters/vault-adapters.ts:27-33`（write 大小守卫）
- Test: `packages/mobile-core/src/vault/scoped-vault.test.ts`、`apps/mobile/src/adapters/vault-adapters.test.ts`

**Interfaces:**
- Produces: `openRow(key, raw, expectedId)` 解密后校验 `entry.id === expectedId`，不匹配抛 `VAULT_DECRYPT`（阻断同 scope 行移动攻击，R1-F42 的最小充分修复——不引入 AAD 密文格式变更）；`vault-adapters.write` 对超过 2000 字节的值抛 `VAULT_ROW_TOO_LARGE`（R1-F17 受控化）；`list()` 惰性删除 `updatedAt` 早于 30 天的行（R1-F14）。

- [ ] **Step 1: 写失败测试**

```typescript
test('a row relocated under another id is rejected (R1-F42)', async () => {
  const { storage, vault: scoped } = vault();
  const store = await scoped.open(lease());
  await store.drafts.put({ id: 'a', body: 'A' });
  const scopeKey = await scopeKeyOf(SCOPE_A);
  const rowA = await storage.read(`${scopeKey}.d.${draftSegment('a')}`); // 见说明
  await storage.write(`${scopeKey}.d.${draftSegment('b')}`, rowA!); // 把 a 的密文挪到 b 的键下
  await assert.rejects(store.drafts.get('b'), /VAULT_DECRYPT/);
});

test('list prunes rows older than the retention window (R1-F14)', async () => {
  const { storage, vault: scoped } = vault();
  const store = await scoped.open(lease());
  await store.drafts.put({ id: 'fresh', body: 'keep' });
  const scopeKey = await scopeKeyOf(SCOPE_A);
  const stale = new Date(Date.now() - 31 * 24 * 3600 * 1000).toISOString();
  // 直接写入一条过期行到索引：伪造索引含 stale id，行内容过期 updatedAt
  const ids = JSON.parse((await storage.read(`${scopeKey}.ix`)) ?? '[]') as string[];
  const entry = { id: 'stale', body: 'old', updatedAt: stale };
  await storage.write(`${scopeKey}.d.${draftSegment('stale')}`, await sealWith(store, entry)); // 测试辅助：见下
  await storage.write(`${scopeKey}.ix`, JSON.stringify([...ids, 'stale']));
  const listed = await store.drafts.list();
  assert.deepEqual(listed.map((e) => e.id), ['fresh']);
  assert.equal(await storage.read(`${scopeKey}.d.${draftSegment('stale')}`), null); // 已被清理
});
```

说明：测试内以 `Buffer.from(id, 'binary').toString('base64').replace(...)`, 与实现的 `draftKeySegment` 一致；`sealWith` 用 `createWebCryptoCipher()` + 从 keyStore 读出的 key 手动封一条（或简化为直接在实现里暴露 `__testSeal`——不要，保持黑盒：伪造 stale 行可用「先 put 再改索引」法：put('stale') 正常写入，然后用新 store 读出 entry 后手工改写行内 updatedAt 不可行（密文）。替代：把断言放宽为「list 不返回过期行」需要能造过期行——在 `vault()` 测试工厂注入可控时钟：`createScopedVault` 增加 `ports.now?.() ?? new Date()`，put 用 `ports.now()`。实现顺带支持，测试注入 31 天前的 now 造 stale 行。）实现采纳：`ScopedVaultPorts` 增加可选 `now(): string`，默认 `new Date().toISOString()`；测试用受控时钟，第二条测试改用注入时钟法，无需 `sealWith` 辅助。

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile-core exec tsx --test src/vault/scoped-vault.test.ts`
Expected: 两条 FAIL（ relocated 行当前可解出 / 无清理逻辑）。

- [ ] **Step 3: 实现**

- `ScopedVaultPorts` 加 `now?: () => string`（`ports.ts`）；`put` 的 `updatedAt` 用之。
- `openRow(key, raw, expectedId)`：结构校验后追加 `if (entry.id !== expectedId) throw new Error('VAULT_DECRYPT');`；`get`/`list` 调用处传 id。
- `list()`（在 Task 3 队列内）：

```typescript
const retentionCutoff = Date.parse(ports.now?.() ?? new Date().toISOString()) - RETENTION_DAYS * 24 * 3600 * 1000;
const retained: string[] = [];
for (const entry of entries) {
  if (Date.parse(entry.updatedAt) < retentionCutoff) await ports.storage.delete(rowKey(scopeKey, entry.id));
  else retained.push(entry.id);
}
if (retained.length !== entries.length) await writeIndex(scopeKey, retained);
```

- `vault-adapters.ts`：

```typescript
const SECURE_STORE_MAX_VALUE_BYTES = 2000; // Android ~2048 软限制，留余量
async write(key, value) {
  if (value.length > SECURE_STORE_MAX_VALUE_BYTES) throw new Error('VAULT_ROW_TOO_LARGE'); // 显式失败优于静默丢写
  await store.setItemAsync(key, value);
},
```

- [ ] **Step 4: 运行测试确认通过（含 apps/mobile 侧）**

```typescript
// vault-adapters.test.ts 追加
test('oversized rows fail loudly instead of vanishing (R1-F17)', async () => {
  const store = memory();
  const storage = createSecureVaultStorage(store as never);
  await assert.rejects(storage.write('k', 'x'.repeat(2001)), /VAULT_ROW_TOO_LARGE/);
});
```

Run: `pnpm --filter @weknora/mobile-core exec tsx --test src/vault/scoped-vault.test.ts && pnpm --filter @weknora/mobile test`
Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/vault/scoped-vault.ts packages/mobile-core/src/vault/ports.ts packages/mobile-core/src/vault/scoped-vault.test.ts apps/mobile/src/adapters/vault-adapters.ts apps/mobile/src/adapters/vault-adapters.test.ts
git commit -m "fix(mobile-core/vault): row-id binding, retention pruning, oversized-row guard (R1-F42/F14/F17)"
```

---

### Task 5: TaskOfficeError 解环 + RUNTIME_STREAM_UNAVAILABLE 契约 code 化

**Files:**
- Create: `packages/mobile-core/src/task-office/task-office-errors.ts`
- Modify: `packages/mobile-core/src/task-office/task-office.ts:1-5`（改从 errors 模块导入并 re-export）、`:114-119`（类定义移走）
- Modify: `packages/mobile-core/src/task-office/task-detail.ts:2`（改导入源）、`:267-277`（streamFailed 判定）
- Modify: `apps/mobile/src/adapters/…`（无：runtime 抛点在 mobile-core）`packages/mobile-core/src/runtime/mobile-runtime.ts:386`（错误带 code）
- Test: `packages/mobile-core/src/task-office/task-detail.test.ts`

**Interfaces:**
- Produces: `task-office-errors.ts` 导出 `TaskOfficeErrorCode`、`TaskOfficeError`；`task-office.ts` re-export 保持既有导入路径兼容（`index.ts` 与测试的 `import { TaskOfficeError } from './task-office.ts'` 不变）；runtime 抛出的流通道缺失错误带 `code: 'RUNTIME_STREAM_UNAVAILABLE'` 属性（与 `TASK_STREAM_CURSOR_EXPIRED` 同形态，R1-F22）。

- [ ] **Step 1: 写失败测试（错误形态）**

`task-detail.test.ts` 追加（`officeWithDetail` 已有）：

```typescript
test('stream-unavailable is detected by error code, not message text (R1-F22)', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { office } = officeWithDetail(leaseRef, {
    detail: async () => detail$(),
    stream: async () => { throw Object.assign(new Error('wrapped: RUNTIME_STREAM_UNAVAILABLE'), { code: 'RUNTIME_STREAM_UNAVAILABLE' }); },
  });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  const view = await handle.hydrate();
  assert.equal(view.connection, 'interrupted');
  assert.equal(view.interruption?.reason, 'stream-unavailable');
});
```

（若 `createScenarioTaskDetailBackend` 的 handlers 不支持自定义 stream 抛错，则在测试内直接构造 backend 对象实现 `TaskDetailBackendPort`，与 `officeWithDetail` 同型。）

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile-core exec tsx --test src/task-office/task-detail.test.ts`
Expected: FAIL（当前按 message 全等判定，包装后不命中，落入 stream-error 自动 resync 路径）。

- [ ] **Step 3: 实现**

新建 `task-office-errors.ts`（把 `task-office.ts:105-119` 的类型与类整体移入）；`task-office.ts` 顶部改：

```typescript
import { createInMemoryTaskProjectionStore } from './in-memory-task-detail.ts';
import { createTaskDetail } from './task-detail.ts';
import { TaskOfficeError } from './task-office-errors.ts';
export { TaskOfficeError } from './task-office-errors.ts';
export type { TaskOfficeErrorCode } from './task-office-errors.ts';
```

`task-detail.ts:2` 改 `import { TaskOfficeError } from './task-office-errors.ts';`——运行时环解除（R1-F21：值导入环 `task-office ⇄ task-detail` 消失，仅剩 task-office → task-detail 单向）。

`mobile-runtime.ts:386`：

```typescript
if (!transport) throw Object.assign(new Error('RUNTIME_STREAM_UNAVAILABLE'), { code: 'RUNTIME_STREAM_UNAVAILABLE' as const });
```

`task-detail.ts` `streamFailed` 判定改（code 优先、message 全等保留为过渡兼容）：

```typescript
const code = (error as { code?: unknown } | null)?.code;
if (code === 'RUNTIME_STREAM_UNAVAILABLE' || message === 'RUNTIME_STREAM_UNAVAILABLE') { …原 stream-unavailable 分支… }
```

- [ ] **Step 4: 运行测试确认通过 + 循环依赖静态确认**

Run: `pnpm --filter @weknora/mobile-core exec tsx --test src/task-office/task-detail.test.ts && pnpm --filter @weknora/mobile-core exec tsx --test src/task-office/task-office.test.ts`
Expected: 全部 PASS。
Run: `grep -n "from './task-office.ts'" packages/mobile-core/src/task-office/task-detail.ts`
Expected: 无输出（值/类型导入均不再指向 task-office.ts）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/task-office/task-office-errors.ts packages/mobile-core/src/task-office/task-office.ts packages/mobile-core/src/task-office/task-detail.ts packages/mobile-core/src/runtime/mobile-runtime.ts packages/mobile-core/src/task-office/task-detail.test.ts
git commit -m "fix(mobile-core/task-office): break runtime import cycle, code-based stream-unavailable contract (R1-F21/F22)"
```

---

### Task 6: 流式路径 lease/epoch 守卫 + 通知隔离 + 终态视图推进 + 控制帧白名单

**Files:**
- Modify: `packages/mobile-core/src/task-office/task-detail.ts:141-145`（notify 隔离）、`:219-283`（四入口守卫与 persist 后复验）、`:120-140`（buildView settlement/execution 推进）、`:250-257`（控制帧白名单）
- Test: `packages/mobile-core/src/task-office/task-detail.test.ts`

**Interfaces:**
- Produces: `notify` 对单个 listener 异常隔离（其余订阅者仍收到通知）；`processEvent` 在 `await persist` 后复验 `epoch === streamEpoch && leaseActive`，失效则丢弃回写；`processEvent/processControl/streamEnded/streamFailed` 入口检查 lease 失效时静默停流（abortStream + 不再 notify，对齐模块注释「scope lease 撤销后的一切结果按 TASK_OFFICE_SCOPE_CHANGED 拒绝」）；`buildView` 在 runStatus 终态时 `settlementStatus: 'settled'`、`executionStatus: runStatus`（镜像 `agent_run_snapshot.go:83-88`）；`processControl` 对 `code === 'heartbeat' || code === 'keepalive'` 直接 return（R1-F45）。

- [ ] **Step 1: 写失败测试**

```typescript
test('a throwing listener does not starve later subscribers (R1-F24)', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { office } = officeWithDetail(leaseRef, { detail: async () => detail$() });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();
  const seen: number[] = [];
  handle.updates(() => { throw new Error('listener boom'); });
  handle.updates(() => { seen.push(1); });
  await handle.resync(); // 触发 notify
  assert.equal(seen.length, 1);
});

test('events arriving after lease revocation are dropped, not merged (R1-F44)', async () => {
  const { revocable, lease } = leased();
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = lease;
  let emit: ((event: TaskBackendEvent) => void) | undefined;
  const { office } = officeWithDetail(leaseRef, {
    detail: async () => detail$(),
    stream: (input) => new Promise<void>((resolve) => { emit = input.onEvent; void input.signal; }),
  });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();
  const before = handle.view()!;
  revocable.revoke();
  emit!(event$(3));
  await settle();
  assert.equal(handle.view()?.cursor, before.cursor); // 撤销后事件未合并
});

test('a stale processEvent does not rewind the cursor after resync (R1-F43)', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const gate = deferred<void>(); // 卡住第一次 persist
  const store = createInMemoryTaskProjectionStore();
  const gatedStore: TaskProjectionStore = {
    load: (runId) => store.load(runId),
    save: async (projection) => { await gate.promise; await store.save(projection); },
  };
  const detailBackend = createScenarioTaskDetailBackend({
    detail: async () => detail$(), // 首次 watermark=2
    stream: (input) => new Promise<void>((resolve) => { input.onEvent(event$(3)); resolve(); }),
  });
  const office = createTaskOffice({ backend: createScenarioTaskBackend({}), lease: () => leaseRef.lease, detail: detailBackend, store: gatedStore });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate(); // events 推进到 3 的 processEvent 挂起在 gatedStore.save 上
  const resynced = handle.resync(); // 挂起期间 resync：watermark=10 的新 detail
  detailBackend.handlers.detail = async () => detail$({ watermark: 10, events: [event$(1, 'run.started'), event$(2, 'tool.started'), event$(3), event$(10, 'text.delta')] });
  await resynced;
  gate.resolve(); // 放行陈旧 persist → 旧 processEvent 恢复，不得把游标回退到 3
  await settle();
  const view = handle.view()!;
  assert.equal(view.cursor, 10);
  assert.notEqual(view.interruption?.reason, 'gap'); // 无陈旧赋值引发的 gap 连锁抖动
});
```

（若 `createScenarioTaskDetailBackend` 的 handlers 不支持运行中替换 detail，则以直接实现 `TaskDetailBackendPort` 的本地对象替代 `detailBackend`：第一次 `detail()` 返回 watermark=2、其后返回 watermark=10。）

test('terminal run settles the execution view without waiting for resync (R1-F27)', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { office } = officeWithDetail(leaseRef, {
    detail: async () => detail$(),
    stream: (input) => new Promise<void>((resolve) => { input.onEvent(event$(3, 'run.completed')); resolve(); }),
  });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();
  await settle();
  const view = handle.view()!;
  assert.equal(view.runStatus, 'succeeded');
  assert.equal(view.settlementStatus, 'settled'); // 当前保持快照 'pending' → 失败
  assert.equal(view.executionStatus, 'succeeded');
});

test('heartbeat control frames are benign (R1-F45)', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { office } = officeWithDetail(leaseRef, {
    detail: async () => detail$(),
    stream: (input) => new Promise<void>((resolve) => { input.onControl({ code: 'heartbeat', message: '' }); resolve(); }),
  });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  const view = await handle.hydrate();
  await settle();
  assert.equal(handle.view()?.connection, 'live'); // 心跳不打断流
});
```

（`officeWithDetail` 已在测试文件头部定义；`TaskProjectionStore`/`createInMemoryTaskProjectionStore` 已有导入。）

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile-core exec tsx --test src/task-office/task-detail.test.ts`
Expected: 新增五条 FAIL（R1-F24 抛错订阅者截断广播；R1-F44 撤销后事件仍合并；R1-F43 陈旧回写回退游标；R1-F27 settlement 停留 'pending'；R1-F45 心跳被判 stream-error）。

- [ ] **Step 3: 实现**

```typescript
const notify = (connection: TaskConnectionState): void => {
  if (closed || detail === undefined) return;
  current = buildView(connection);
  for (const listener of [...listeners]) {
    try { listener(current); } catch { /* 订阅者异常不截断广播（R1-F24） */ }
  }
};
const streamGuard = (): boolean => { // 四入口共用：lease 失效即静默停流
  if (closed) return false;
  if (!ports.lease() || !leaseActive(ports.lease()!)) { abortStream(); return false; }
  return true;
};
```

- `processEvent`：入口 `if (!streamGuard() || detail === undefined) return;`；`await persist(...)` 之后、回写 `events/committedCursor/autoResyncs` 之前插入 `if (epoch !== streamEpoch || !leaseActive(ports.lease() ?? undefined)) return;`（epoch 取 chain 步骤开始时的快照；对齐 hydrate:175-179 的既有守卫语义，R1-F43）。
- `processControl`：入口 `if (!streamGuard() || closed) return;`；`frame.code === 'heartbeat' || frame.code === 'keepalive'` 直接 return。
- `streamEnded`/`streamFailed`：入口 `if (!streamGuard()) return;`。
- `buildView`：

```typescript
const runStatus = terminalRunStatusOf(base.execution.runStatus, events);
const terminal = isTerminalRunStatus(runStatus);
// 镜像 internal/application/repository/agent_run_snapshot.go:83-88：终态 run 即 settled、executionStatus=runStatus
settlementStatus: terminal ? 'settled' : base.execution.settlementStatus,
executionStatus: terminal ? runStatus : base.execution.executionStatus,
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --filter @weknora/mobile-core exec tsx --test src/task-office/task-detail.test.ts`
Expected: 全部 PASS（既有 406-420 撤销断言从「偶然通过」变为受守卫保证）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/task-office/task-detail.ts packages/mobile-core/src/task-office/task-detail.test.ts
git commit -m "fix(mobile-core/task-office): lease/epoch guards on stream paths, listener isolation, terminal execution view, benign control frames (R1-F44/F43/F24/F27/F45)"
```

---

### Task 7: persist 写放大治理 + chain 背压上限

**Files:**
- Modify: `packages/mobile-core/src/task-office/task-detail.ts:8`（常量区）、`:151-153`（persist）、`:212-217`（chain 排队）、`:238-248`（processEvent 持久化决策）、`:258-266`（streamEnded flush）
- Test: `packages/mobile-core/src/task-office/task-detail.test.ts`

**Interfaces:**
- Produces: 内部 `PERSIST_MIN_STRIDE = 50`（事件跨度阈值）与 `CHAIN_DEPTH_LIMIT = 1000`；`processEvent` 仅在 `event.seq - persistedCursor >= PERSIST_MIN_STRIDE` 或终态/中断/close 时调用 persist；chain 深度超限时 `abortStream()` + `interrupt('gap', …)`。重启恢复语义不变（cursor 最多落后 50 个事件，`hydrate` 的 `mergeEventHistory` 兜底）。

- [ ] **Step 1: 写失败测试**

```typescript
test('persist coalesces bursts: one save per 50-event stride plus terminal flush (R1-F23)', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const store = createInMemoryTaskProjectionStore();
  let saves = 0;
  const counting = { load: store.load.bind(store), save: async (p: Parameters<typeof store.save>[0]) => { saves += 1; await store.save(p); } };
  const { office } = officeWithDetail(leaseRef, {
    detail: async () => detail$(),
    stream: (input) => new Promise<void>((resolve) => {
      for (let seq = 3; seq <= 62; seq += 1) input.onEvent(event$(seq, seq === 62 ? 'run.completed' : 'text.delta'));
      resolve();
    }),
  }, counting);
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();
  await settle();
  assert.ok(saves <= 3, `expected coalesced saves, got ${saves}`); // 60 事件 ≤ 2 次 stride + 1 次终态 flush
  assert.equal(handle.view()?.cursor, 62);
  assert.equal(handle.view()?.connection, 'drained');
  const persisted = await store.load('run-1');
  assert.equal(persisted?.cursor, 62); // 终态必已 flush
});
```

（`officeWithDetail` 第三参已接受自定义 store。）

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile-core exec tsx --test src/task-office/task-detail.test.ts`
Expected: FAIL（当前每事件一次 save，60 事件 → saves=60）。

- [ ] **Step 3: 实现**

```typescript
const PERSIST_MIN_STRIDE = 50;
const CHAIN_DEPTH_LIMIT = 1000;
let persistedCursor = 0;
let chainDepth = 0;
const enqueueChain = (step: () => Promise<void>): void => {
  if (chainDepth > CHAIN_DEPTH_LIMIT) { abortStream(); void interrupt('gap', 'stream queue overflow'); return; }
  chainDepth += 1;
  chain = chain.then(step).catch(() => undefined).finally(() => { chainDepth -= 1; });
};
```

`startStream` 的四处 `chain = chain.then(...)` 全部换 `enqueueChain(...)`。`processEvent` 中：

```typescript
events = nextEvents; committedCursor = event.seq; autoResyncs = 0;
const terminal = isTerminalRunStatus(terminalRunStatusOf(detail.execution.runStatus, events));
if (terminal || event.seq - persistedCursor >= PERSIST_MIN_STRIDE) {
  try { await persist(event.seq, nextEvents); persistedCursor = event.seq; }
  catch (cause) { await interrupt('persist-failed', …); return; } // 保留既有先持久化语义的失败可见性
}
```

注意保持「先持久化后推进游标」的崩溃安全不变式：stride 模式下改为「内存先推进、每 50 事件落一次盘、终态/中断/close 强制 flush」——内存态推进不再以 persist 成功为前提（这是本任务的行为变更点）；`streamEnded` 终态分支与 `interrupt` 前追加 `await persist(committedCursor, events).catch(() => undefined)` 的 best-effort flush（失败已由 interruption 表达）。既有「persist 失败 → persist-failed interruption」测试需按新语义核对（首事件即 persist 时行为不变：`persistedCursor` 初始为 hydrate 时已成功 persist 的 watermark）。

- [ ] **Step 4: 运行测试确认通过 + 持久失败可见性（Review Focus #3）**

```typescript
test('persist failure stays observable when auto-resync budget is exhausted (R1-F23 语义保留)', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const failing = { load: async () => undefined, save: async () => { throw new Error('disk full'); } };
  const { office } = officeWithDetail(leaseRef, {
    detail: async () => detail$(),
    stream: (input) => new Promise<void>((resolve) => { input.onEvent(event$(3)); resolve(); }),
  }, failing);
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate(); // hydrate 内 persist 失败 → persist-failed interruption
  const view = handle.view()!;
  assert.equal(view.interruption?.reason, 'persist-failed');
});
```

Run: `pnpm --filter @weknora/mobile-core exec tsx --test src/task-office/task-detail.test.ts`
Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/task-office/task-detail.ts packages/mobile-core/src/task-office/task-detail.test.ts
git commit -m "fix(mobile-core/task-office): coalesce projection saves with stride+terminal flush, bound stream queue depth (R1-F23)"
```

---

### Task 8: archive/restore 作废在途列表 + 组合根显式装配 store

**Files:**
- Modify: `packages/mobile-core/src/task-office/task-office.ts:193-200`（mutate）
- Modify: `apps/mobile/src/composition.ts:103-115`（taskOfficeFor 显式 store）
- Test: `packages/mobile-core/src/task-office/task-office.test.ts`、`apps/mobile/src/app-smoke.test.tsx`

**Interfaces:**
- Produces: `mutate` 成功后 `listEpoch += 1; homeEpoch += 1;`（archive/restore 后在途 `tasks()/home()` 按 `TASK_OFFICE_SUPERSEDED` 拒绝，`accumulated` 同步失效）；`composition.ts` 显式传入 `store: createInMemoryTaskProjectionStore()`（显式决策替代静默回退，R1-F20 的最小修复；持久化 store 留 Round 2，见排除项）。

- [ ] **Step 1: 写失败测试（R1-F19）**

`task-office.test.ts` 追加：

```typescript
test('a successful archive rejects in-flight tasks() with SUPERSEDED and clears accumulation (R1-F19)', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const pending = deferred<TaskBackendPage>();
  const { office } = officeWith(leaseRef, {
    list: () => pending.promise,
    archive: async () => undefined,
  });
  const inflight = office.tasks({});
  await office.archive('task-1'); // 在途 list 尚未 resolve
  await assert.rejects(inflight, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SUPERSEDED');
  pending.resolve({ items: [backendRun('r-archived')] });
  await assert.rejects(office.moreTasks(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_NO_ACTIVE_QUERY');
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile-core exec tsx --test src/task-office/task-office.test.ts`
Expected: FAIL（在途 list 通过 settle 校验并以归档前快照重建 accumulated）。

- [ ] **Step 3: 实现**

```typescript
const mutate = async (taskId: string, action: (id: string) => Promise<void>): Promise<void> => {
  const lease = requireLease();
  const trimmed = taskId.trim();
  if (trimmed === '') throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
  await callBackend(() => action(trimmed));
  if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
  // 写成功作废一切在途读（R1-F19）：settle 比对 epoch，旧查询按 SUPERSEDED 拒绝。
  listEpoch += 1;
  homeEpoch += 1;
  accumulated = undefined;
};
```

`composition.ts` 的 `taskOfficeFor`：

```typescript
office = createTaskOffice({
  backend: remote,
  detail: remote,
  lease: () => activeRuntime.scopeLease(),
  // 显式装配（R1-F20 最小修复）：App 重启恢复需要持久 TaskProjectionStore（SQLite 后端，Round 2）；
  // 此处显式传 in-memory store 使「未注入持久化」成为组合根的显式决策而非静默回退。
  store: createInMemoryTaskProjectionStore(),
});
```

（import 行补 `createInMemoryTaskProjectionStore`——已从 `@weknora/mobile-core` 导出，见 `index.ts:16`。）

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --filter @weknora/mobile-core exec tsx --test src/task-office/task-office.test.ts && pnpm --filter @weknora/mobile test`
Expected: 全部 PASS（含 app-smoke 既有 taskOffice 装配断言）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/task-office/task-office.ts packages/mobile-core/src/task-office/task-office.test.ts apps/mobile/src/composition.ts
git commit -m "fix(mobile-core/task-office): archive/restore supersedes in-flight lists; explicit store wiring in composition root (R1-F19/F20)"
```

---

### Task 9: Home/Tasks remount key 补 deployment origin 维度

**Files:**
- Modify: `apps/mobile/src/composition.ts:122-123`（TasksScreen key）、`:140-141`（HomeScreen key）
- Test: `apps/mobile/src/app-smoke.test.tsx`

**Interfaces:**
- Consumes: Task 8 之后的 `taskOfficeFor`（按 origin 记忆化已存在）。
- Produces: remount key 格式 `${origin}::${activeTenantId}`；跨部署切换（两部署 activeTenantId 相同）必然 remount，`useEffect(load, [])` / `useEffect(reload, [])` 重跑、闭包捕获新 taskOffice。

- [ ] **Step 1: 写失败测试（R1-F48/F49 同根因，一条测试覆盖两屏）**

`app-smoke.test.tsx` 追加（沿用既有 `hooks()`/`render` 测试基建）：

```typescript
test('switching deployments with equal tenant ids remounts Home and Tasks with fresh closures (R1-F48/F49)', async () => {
  const { RuntimeSurface, deploymentScopeKey } = await import('./composition.ts');
  const props = (origin: string) => ({
    snapshot: {
      surface: 'authorized', reason: undefined,
      deployment: { origin, label: origin },
      identity: { userId: 'user-1', activeTenantId: '1', tenants: [{ id: '1' }] },
    },
    onSignIn: async () => {}, onBeginOidc: async () => {}, onSignOut: async () => {}, onActivateTenant: async () => {},
  } as never);
  const first = RuntimeSurface(props('https://a.example.test'));
  const second = RuntimeSurface(props('https://b.example.test'));
  // 两部署租户 id 相同（自增小整数常见）：key 必须因 origin 维度而不同
  assert.notEqual((first as { key?: unknown }).key, (second as { key?: unknown }).key);
  assert.notEqual(deploymentScopeKey('https://a.example.test', '1'), deploymentScopeKey('https://b.example.test', '1'));
});
```

（`deploymentScopeKey` 为 Step 3 新增的导出纯函数；对 `MobileTasks` 的 TasksScreen key 同用该函数，行为级验证由 Step 3 的实现与既有 app-smoke 渲染断言覆盖——同 tenantId 切换 origin 后 HomeScreen/TasksScreen 元素 key 变化即 remount。）

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL——`deploymentScopeKey` 尚未导出（import 失败即 RED）；实现仅 key 表达式、暂不改函数时的等价 RED 为两次渲染 `key` 相同。

- [ ] **Step 3: 实现**

`composition.ts` 导出纯函数并在两处使用：

```typescript
/** remount key 必须含 deployment origin：两部署租户 id 相同（自增小整数常见）时
 * 跨部署切换也必须 remount，否则 useEffect(load,[]) 不重跑、旧闭包命中已撤销 lease（R1-F48/F49）。 */
export function deploymentScopeKey(origin: string, activeTenantId: string): string {
  return `${origin}::${activeTenantId}`;
}
```

- `MobileTasks`：`key: deploymentScopeKey(snapshot.deployment.origin, snapshot.identity.activeTenantId)`
- `RuntimeSurface` authorized 分支：`key: deploymentScopeKey(snapshot.deployment.origin, snapshot.identity.activeTenantId)`

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --filter @weknora/mobile test`
Expected: 全部 PASS（既有租户切换 remount 断言不回归——key 仍随 activeTenantId 变化）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/composition.ts apps/mobile/src/app-smoke.test.tsx
git commit -m "fix(mobile): remount keys include deployment origin to prevent cross-deployment state bleed (R1-F48/F49)"
```

---

### Task 10: shelf browse 代次保护 + verdict 冻结 + 脏行过滤 + kind 三态

**Files:**
- Modify: `packages/mobile-core/src/shelf/resource-shelf.ts:16`（SUPPORTED 冻结）、`:35/:82-85`（verdicts 复制）、`:66-100`（browse 代次）、`:80`（knowledge 空行过滤）
- Modify: `packages/domain/src/mobile/resource-presentation.ts:27`（kind 三态）、`:53`（未知映射）
- Test: `packages/mobile-core/src/shelf/resource-shelf.test.ts`、`packages/domain/src/mobile/resource-presentation.test.ts`

**Interfaces:**
- Produces: shelf 内部 `browseEpoch` 代次（`browse()` 开始自增，写共享 `agents/knowledge/connections/verdicts` 前复验，迟到请求丢弃结果且不触发 authorization-revoked 误报）；`ConnectionResource['kind']` 增 `'unknown'`；`toConnectionResource` 未知 kind 映射 `'unknown'`（与同函数 state 的 fail-closed 对齐，R1-F3）；knowledge/connection 装配处过滤 `id === ''` 脏行（R1-F35——`knowledgeRefForPrompt` 不拒空 id，过滤必须在 shelf 层）。

- [ ] **Step 1: 写失败测试（R1-F2 迟到覆盖）**

`resource-shelf.test.ts` 追加（复用既有 `openShelf(script)` 工厂；`ScriptedResourceRemote` 方法可覆写以注入延迟）：

```typescript
test('a slower stale browse cannot overwrite a newer snapshot or fire bogus revocations (R1-F2)', async () => {
  const { handle, remote } = openShelf({
    agents: [{ id: 'builtin-quick-answer', name: 'Quick Answer', summary: '', kind: 'general', capability: { state: 'supported', reason: '' } }],
    knowledgeBases: [{ id: 'kb-1', title: 'Handbook', scan_status: 'indexed', document_count: 1, updated_at: '2026-09-01T00:00:00Z' }],
    connections: [],
  });
  // 第一次 browse 的 knowledge 慢且返回 403（旧请求：撤权后又快速恢复权限的场景）；
  // 第二次（用户 refresh）立即成功。后完成者为旧请求 → forbidden 快照覆盖成功快照 + 误报 revocation。
  let knowledgeCalls = 0;
  const originalKnowledge = remote.knowledgeBases.bind(remote);
  remote.knowledgeBases = async (token: string) => {
    knowledgeCalls += 1;
    if (knowledgeCalls === 1) await new Promise((resolve) => setTimeout(resolve, 10));
    if (knowledgeCalls === 1) throw Object.assign(new Error('HTTP 403'), { status: 403 });
    return originalKnowledge(token);
  };
  const events: unknown[] = [];
  handle.subscribe((event) => events.push(event));
  const stale = handle.browse({});
  const fresh = handle.browse({});
  await Promise.all([stale, fresh]);
  // 区分度：旧实现中迟到的 403 会覆盖 fresh 的 supported 快照并误发 authorization-revoked；
  // 新实现丢弃迟到写回，共享快照保持 fresh 的 supported，selection 基于新快照裁决。
  assert.deepEqual(handle.selection({ knowledgeId: 'kb-1' }).allowed, true);
  assert.deepEqual(events.filter((event) => (event as { type?: string }).type === 'authorization-revoked'), []); // 无误报
});
```

`resource-presentation.test.ts` 追加：

```typescript
test('unknown connection kind maps to unknown, not silently personal; blank ids are filtered by the shelf', () => {
  assert.equal(toConnectionResource({ id: 'c1', kind: 'weird' }).kind, 'unknown');
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile-core exec tsx --test src/shelf/resource-shelf.test.ts && pnpm exec tsx --test packages/domain/src/mobile/resource-presentation.test.ts`
Expected: 两条 FAIL。

- [ ] **Step 3: 实现**

```typescript
const SUPPORTED: ResourceClassVerdict = Object.freeze({ state: 'supported', reason: '' }); // R1-F36：共享对象冻结
let verdicts: Record<ResourceClass, ResourceClassVerdict> = {
  agent: { ...SUPPORTED }, knowledge: { ...SUPPORTED }, connection: { ...SUPPORTED }, // 装配即复制
};
let browseEpoch = 0;
async browse(query = {}) {
  guard();
  const epoch = ++browseEpoch; // 代次：写共享快照前复验，迟到者丢弃（UI 层 resources-view 的 generation 只保护页面态）
  const […] = await Promise.all([…]);
  guard();
  if (epoch !== browseEpoch) { /* 迟到请求：以当前（更新）快照构造返回页，不回写共享态 */ }
  else { …原 77-91 行的共享快照写与 revocation 检测… }
  …原 92-100 行按（最新）共享快照构造 page 返回…
}
```

（迟到分支返回的 `page` 用当前共享 `agents/knowledge/connections/verdicts` 构造——语义为「你看到的是最新已知状态」，与 `resources-view` 的 latest-wins 一致。）knowledge/connection 装配行改为：

```typescript
knowledge = knowledgeResult.ok
  ? knowledgeResult.value.map((row) => toKnowledgeResource(row)).filter((resource) => resource.id !== '') // R1-F35：脏行不得进入投影（selection 才不会放行空引用）
  : [];
connections = connectionResult.ok ? connectionResult.value.map((row) => toConnectionResource(row)).filter((c) => c.id !== '') : [];
```

`resource-presentation.ts`：`kind: 'personal' | 'space' | 'unknown'`；`kind: row.kind === 'space' ? 'space' : row.kind === 'personal' ? 'personal' : 'unknown'`。检查 `ResourcesScreen.tsx:45` 的 `${connection.kind} connection` 文本渲染——'unknown' 直接显示，无需改屏。

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --filter @weknora/mobile-core exec tsx --test src/shelf/resource-shelf.test.ts && pnpm exec tsx --test packages/domain/src/mobile/resource-presentation.test.ts && pnpm --filter @weknora/mobile test`
Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/shelf/resource-shelf.ts packages/mobile-core/src/shelf/resource-shelf.test.ts packages/domain/src/mobile/resource-presentation.ts packages/domain/src/mobile/resource-presentation.test.ts
git commit -m "fix(mobile-core/shelf): browse epoch guard, frozen verdicts, dirty-row filtering, unknown connection kind (R1-F2/F36/F35/F3)"
```

---

### Task 11: switchDeployment 短路收窄 + accessTokenFor epoch 复核 + tenantId 归一 + forgetDeployment 解耦

**Files:**
- Modify: `packages/mobile-core/src/runtime/mobile-runtime.ts:26-28`（tenantId trim）、`:202-210`（accessTokenFor）、`:423-431`（switchDeployment 短路）、`:451-468`（forgetDeployment）
- Test: `packages/mobile-core/src/runtime/runtime-deployments.test.ts`（既有部署切换测试文件）、`packages/mobile-core/src/runtime/runtime-shelf.test.ts`

**Interfaces:**
- Produces: `switchDeployment` 短路条件收窄为 `state.surface === 'authorized' || state.surface === 'read-only'`（upgrade-required/read-only 之外的失败面重走完整认证，R1-F46）；`accessTokenFor` 在 `refreshedCredential` resolve 后复验 `current(requestEpoch, deployment)` 失败抛 `SHELF_SCOPE`（R1-F31）；`tenantId` 字符串分支 `.trim()`（R1-F33）；`forgetDeployment` 中 credential 清理失败不再吞掉 `registry.remove`（各自 try/catch，R1-F47）。

- [ ] **Step 1: 写失败测试**

`runtime-deployments.test.ts` 追加（复用本文件既有 `fakeStore`/`remote(overrides)`/`createInMemoryDeploymentRegistry` 工厂与 `FIRST` 常量）：

```typescript
test('re-selecting the same origin from upgrade-required retries authentication instead of short-circuiting (R1-F46)', async () => {
  let healthy = false;
  const failingThenHealthy = remote({
    me: async () => {
      if (!healthy) throw Object.assign(new Error('server exploded'), { name: 'ApiError', status: 500 });
      return { user: { id: 'user-1' }, tenant: { id: 'tenant-1' } };
    },
  });
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(), deploymentStore: undefined, deploymentRegistry: createInMemoryDeploymentRegistry(),
    clientVersion: CLIENT_PROTOCOL_VERSION, remoteFor: () => failingThenHealthy,
  });
  await runtime.signIn({ deployment: FIRST, email: 'u@example.test', password: 'pw' });
  assert.equal(runtime.snapshot().surface, 'upgrade-required'); // 首次 authenticate 失败
  healthy = true; // 服务端恢复
  await runtime.switchDeployment(FIRST.origin);
  assert.equal(runtime.snapshot().surface, 'authorized'); // 当前被静默短路 → FAIL
});

test('forgetDeployment removes the registry entry even when credential clearing fails (R1-F47)', async () => {
  const locked = fakeStore();
  const originalClear = locked.clear.bind(locked);
  locked.clear = async (deployment: string) => { if (deployment === FIRST.origin) throw new Error('secure store locked'); return originalClear(deployment); };
  const registry = createInMemoryDeploymentRegistry();
  await registry.upsert(FIRST);
  const runtime = createMobileRuntime({
    credentialStore: locked, deploymentStore: undefined, deploymentRegistry: registry,
    clientVersion: CLIENT_PROTOCOL_VERSION, remoteFor: () => remote(),
  });
  await runtime.forgetDeployment(FIRST.origin);
  assert.deepEqual(await registry.list(), []); // 清理失败不得吞掉登记移除
});
```

`runtime-shelf.test.ts` 追加（R1-F31，复用本文件 `fakeStore`/`baseRemote` 工厂模式）：构造进入 authorized 面后，用 deferred 卡住 `remote.refresh`，在刷新在途时 `switchDeployment(SECOND)`（epoch 变化），随后 resolve 刷新——断言旧部署的 shelf 侧 `accessTokenFor(FIRST.origin, { refresh: true })` reject `SHELF_SCOPE` 且 `activeCredential` 未被旧刷新结果写回（新部署面 `authorizedRequest` 仍用新部署凭据）。

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile-core exec tsx --test src/runtime/runtime-deployments.test.ts && pnpm --filter @weknora/mobile-core exec tsx --test src/runtime/runtime-shelf.test.ts`
Expected: 新增用例 FAIL。

- [ ] **Step 3: 实现**

```typescript
// R1-F33
if (typeof id === 'string' && id.trim() !== '') return id.trim();

// R1-F31：refresh 跨越了 scope 变化时不得把旧部署凭据写回活动态
const refreshed = await refreshedCredential(epoch, deployment, activeCredential);
if (!refreshed) throw new Error('SHELF_AUTH');
if (!current(epoch, deployment)) throw new Error('SHELF_SCOPE');
activeCredential = refreshed;

// R1-F46：同 origin 短路只覆盖已授权/只读面；upgrade-required 等失败面必须允许重试
if ((state.surface === 'authorized' || state.surface === 'read-only') && state.deployment?.origin === target.origin) return state;

// R1-F47：清理与登记移除各自包含
try { …credential 清理… } catch { /* 记录失败，继续移除登记 */ }
try { await mutateDeployment(async () => { await ports.deploymentRegistry?.remove(deployment.origin); }); } catch { /* 保持从不 reject */ }
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --filter @weknora/mobile-core test`
Expected: 全部 PASS（mobile-core 全量，含 runtime 四个测试文件）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/runtime/mobile-runtime.ts packages/mobile-core/src/runtime/runtime-deployments.test.ts packages/mobile-core/src/runtime/runtime-shelf.test.ts
git commit -m "fix(mobile-core/runtime): narrow same-origin short-circuit, post-refresh epoch recheck, tenant trim, decoupled forget cleanup (R1-F46/F31/F33/F47)"
```

---

### Task 12: SSE 失败响应连接释放 + scope 变化中止在途流 + IPv4 保留网段 + 类型导出

**Files:**
- Modify: `apps/mobile/src/adapters/sse-stream.ts:18-22`（cancel body）
- Modify: `packages/mobile-core/src/runtime/mobile-runtime.ts:380-393`（authorizedEventStream abort 桥）、`:139-142`（revoke/begin 中止活动流）
- Modify: `apps/mobile/src/runtime-integration-smoke.ts:40-48`（保留网段）
- Modify: `packages/mobile-core/src/index.ts:2`（补导出类型）
- Test: `apps/mobile/src/adapters/sse-stream.test.ts`（如不存在则新建；先查 `ls apps/mobile/src/adapters/*.test.ts`）、`apps/mobile/src/task-detail-integration-smoke.test.ts`（IPv4 用例扩充处）、`packages/mobile-core/src/runtime/mobile-runtime.test.ts`

**Interfaces:**
- Produces: `streamAuthorizedSse` 非 2xx 分支先 `await response.body?.cancel().catch(() => undefined)` 再 throw（R1-F50）；runtime 维护 `activeStreams = new Set<AbortController>()`——`authorizedEventStream` 为无 signal 的调用注册 controller 并在 `revoke()`（begin/reserve/signOut 共用路径）里全部 abort（R1-F32）；`disallowedIpv4Octets` 补 `100.64-127/10`（CGNAT）、`198.18-19/15`（benchmark）、`192.0.0/24`、`192.0.2/24`、`198.51.100/24`、`203.0.113/24`（R1-F37）；`index.ts` 补 `export type { AuthorizedTransport, AuthorizedStreamTransport }`（R1-F34）。

- [ ] **Step 1: 写失败测试**

```typescript
// sse-stream.test.ts（若文件不存在则新建，风格同 vault-adapters.test.ts）
test('a failed SSE response releases its body (R1-F50)', async () => {
  let cancelled = 0;
  const fake = { ok: false, status: 409, body: { cancel: async () => { cancelled += 1; } } } as unknown as Response;
  const failing = (async () => fake) as SseFetchLike;
  await assert.rejects(
    streamAuthorizedSse('https://x.example.test', { method: 'GET', path: '/api/v1/events' }, 'token', () => {}, failing),
    (error: unknown) => (error as { code?: string }).code === 'HTTP_409',
  );
  assert.equal(cancelled, 1);
});

// runtime 侧（mobile-runtime.test.ts，复用既有 ports()/deferred() 夹具与 :712 起的 authorizedStream 用例模式）
test('revoking the scope aborts an in-flight authorized stream without waiting for the next chunk (R1-F32)', async () => {
  let aborted = false;
  const streamPorts = ports(fakeStore(), remote());
  streamPorts.authorizedStream = () => async (input) => {
    await new Promise<void>((_resolve, reject) => {
      input.signal?.addEventListener('abort', () => { aborted = true; reject(new Error('aborted')); });
    });
  };
  const streaming = createMobileRuntime(streamPorts);
  await streaming.signIn({ deployment: { origin: 'https://weknora.example.test' }, email: 'u@example.test', password: 'pw' });
  const reading = streaming.authorizedEventStream({ method: 'GET', path: '/api/v1/events' }, () => {});
  await new Promise((resolve) => setImmediate(resolve));
  await streaming.signOut(); // revoke 路径
  await reading.catch(() => undefined);
  assert.ok(aborted);
});
```

`task-detail-integration-smoke.test.ts:15` 的 host 列表追加：`'https://100.64.0.1'`、`'https://198.18.0.1'`、`'https://192.0.2.1'`、`'https://203.0.113.1'`（该测试断言这些 origin 被判 invalid——R1-F37 的验收即此测试通过）。

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile-core exec tsx --test src/runtime/mobile-runtime.test.ts`
Expected: 新增用例 FAIL（`cancelled === 0` / `aborted === false` / 新增 IP 未被拒）。

- [ ] **Step 3: 实现**

```typescript
// sse-stream.ts
if (!response.ok || !response.body) {
  await response.body?.cancel().catch(() => undefined); // 失败响应也必须释放连接（R1-F50；401 刷新重试反复放大占用）
  throw new ApiError({ …原样… });
}

// mobile-runtime.ts
const activeStreams = new Set<AbortController>();
// authorizedEventStream 内：
const controller = new AbortController();
activeStreams.add(controller);
input.signal?.addEventListener('abort', () => controller.abort(input.signal!.reason), { once: true });
controller.signal.addEventListener('abort', () => activeStreams.delete(controller), { once: true });
try {
  await sendWithCredential(requestEpoch, deployment, (token) => transport({ ...input, signal: controller.signal }, token, guardedChunk));
} finally { activeStreams.delete(controller); }
// revoke()（begin/reserve/dispose 共用）开头追加：
for (const controller of activeStreams) controller.abort();
activeStreams.clear();
```

`runtime-integration-smoke.ts` 的 `disallowedIpv4Octets` 追加：

```typescript
if (a === 100 && b >= 64 && b <= 127) return `${variable} must not target carrier-grade NAT addresses`;
if (a === 198 && (b === 18 || b === 19)) return `${variable} must not target benchmarking addresses`;
if (a === 192 && b === 0 && (octets[2]! <= 0 || octets[2]! === 2)) return `${variable} must not target reserved addresses`; // 192.0.0/24 与 192.0.2/24
if (a === 198 && b === 51 && octets[2]! === 100) return `${variable} must not target documentation addresses`;
if (a === 203 && b === 0 && octets[2]! === 113) return `${variable} must not target documentation addresses`;
```

（192.0.0/24 判定按 `(a === 192 && b === 0 && octets[2] === 0)` 与 TEST-NET-1 `192.0.2/24` 分开写，避免一行混叠。）`index.ts:2` 行补 `AuthorizedTransport, AuthorizedStreamTransport` 两个类型名。

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile-core test`
Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/adapters/sse-stream.ts apps/mobile/src/adapters/sse-stream.test.ts packages/mobile-core/src/runtime/mobile-runtime.ts packages/mobile-core/src/runtime/mobile-runtime.test.ts apps/mobile/src/runtime-integration-smoke.ts apps/mobile/src/task-detail-integration-smoke.test.ts packages/mobile-core/src/index.ts
git commit -m "fix(mobile): release failed SSE bodies, abort streams on scope revoke, cover reserved IPv4 ranges, export transport types (R1-F50/F32/F37/F34)"
```

---

### Task 13: iOS 插件 —— 冷启动 universal link 转发、newArchEnabled 读声明、幂等标记、引号解析

**Files:**
- Modify: `apps/mobile/plugins/ios-xcode27.js:111-154`（SCENE_DELEGATE_SOURCE）、`:183-217`（PODFILE_CLAMP 幂等标记）、`:220-232`（raiseDeploymentTargets 引号 strip）、`:234-251`（withIosXcode27 读 config.newArchEnabled）
- Test: `apps/mobile/src/plugins/ios-xcode27.test.ts`

**Interfaces:**
- Produces: `willConnectTo` 内转发 `connectionOptions.userActivities`（通用链接冷启动投递，R1-F5——对齐文件头 18-23 行注释承诺）；插件从 `config.newArchEnabled`（Expo SDK 55 app.json 顶层字段）读取声明，声明为 true 时 `console.warn` 说明 iOS 27 模拟器 Fabric 不渲染、仍按 false 处理（R1-F46 对应的显式告警而非静默覆盖）；`applyPodfileClamp` 幂等标记改为唯一注释锚 `# weknora_ios_xcode27_clamp`（R1-F7）；`raiseDeploymentTargets` 对带引号的目标值先 `replace(/^"|"$/g, '')`（R1-F8）。

- [ ] **Step 1: 写失败测试**

`ios-xcode27.test.ts` 追加（沿用既有 TEMPLATE_APP_DELEGATE 常量构造）：

```typescript
test('willConnectTo forwards userActivities for cold-launch universal links (R1-F5)', () => {
  const rewritten = plugin.applySceneLifecycle(TEMPLATE_APP_DELEGATE);
  const willConnect = rewritten.slice(rewritten.indexOf('willConnectTo'), rewritten.indexOf('openURLContexts'));
  assert.match(willConnect, /userActivities/);
  assert.match(rewritten, /forwardUserActivities\(connectionOptions\.userActivities\)/);
});

test('the clamp is idempotent by a unique anchor, not by its own substring (R1-F7)', () => {
  const podfile = TEMPLATE_PODFILE_WITH_PARTIAL_CLAMP; // 构造：模板 post_install + 一段含 'Xcodeproj::Plist.read_from_path' 的无关注入
  const once = plugin.applyPodfileClamp(podfile);
  assert.ok(once.includes('# weknora_ios_xcode27_clamp'));
  const twice = plugin.applyPodfileClamp(once);
  assert.equal(twice.split('# weknora_ios_xcode27_clamp').length - 1, 1); // 不重复注入
});

test('quoted deployment targets are still raised (R1-F8)', () => {
  const project = { pbxXCBuildConfigurationSection: () => ({ BC1: { buildSettings: { IPHONEOS_DEPLOYMENT_TARGET: '"15.1"' } } }) };
  const result = plugin.raiseDeploymentTargets(project) as typeof project;
  assert.equal(result.pbxXCBuildConfigurationSection().BC1.buildSettings.IPHONEOS_DEPLOYMENT_TARGET, plugin.DEPLOYMENT_TARGET);
});

test('the plugin honors and warns about an opt-in newArchEnabled declaration (R1-F6)', () => {
  // withInfoPlist/withPodfileProperties 的行为需要经 expo/config-plugins 包装——直接测导出的
  // 决策函数：把 newArchEnabled 决策提为导出函数 resolveNewArchEnabled(config) 并单测。
  assert.equal(plugin.resolveNewArchEnabled({ newArchEnabled: true }).effective, false);
  assert.equal(plugin.resolveNewArchEnabled({ newArchEnabled: true }).warned, true);
  assert.equal(plugin.resolveNewArchEnabled({}).effective, false);
  assert.equal(plugin.resolveNewArchEnabled({}).warned, false);
});
```

（`TEMPLATE_PODFILE_WITH_PARTIAL_CLAMP`：`TEMPLATE_POST_INSTALL` + 一行含 `Xcodeproj::Plist.read_from_path` 的其它脚本——当前实现会误判已注入而跳过，即 R1-F7 复现。）

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: 四条 FAIL。

- [ ] **Step 3: 实现**

`SCENE_DELEGATE_SOURCE` 的 `willConnectTo` 追加：

```swift
    forwardURLContexts(connectionOptions.urlContexts)
    // Cold launch by universal link: UIKit delivers NSUserActivity here (not
    // through scene(_:continue:)); forward before the JS bundle runs so
    // expo-router resolves the initial route (file-header promise, R1-F5).
    forwardUserActivities(connectionOptions.userActivities)
```

并新增私有转发方法（镜像既有 `forwardURLContexts` 的 appDelegate 委托形态，调用 `appDelegate.application(_:continue:restorationHandler:)`——与热路径 `scene(_:continue:)` 相同调用目标）：

```swift
  private func forwardUserActivities(_ activities: Set<NSUserActivity>) {
    guard let appDelegate = UIApplication.shared.delegate as? AppDelegate else { return }
    for activity in activities where activity.activityType == NSUserActivityTypeBrowsingWeb {
      _ = appDelegate.application(
        UIApplication.shared,
        continue: activity,
        restorationHandler: { (_: [UIUserActivityRestoring]?) in })
    }
  }
```

插件入口：

```javascript
/** R1-F6：app.json 声明优先于硬编码；iOS 27 模拟器 Fabric 不渲染，opt-in 时显式告警。 */
function resolveNewArchEnabled(config) {
  const declared = config?.newArchEnabled === true;
  if (declared) {
    console.warn(
      'ios-xcode27 plugin: app.json sets newArchEnabled=true, but Fabric renders nothing on the iOS 27.0 simulator runtime; forcing old architecture on iOS. Remove this plugin when RN/Expo support the iOS 27 SDK.',
    );
  }
  return { effective: false, warned: declared };
}
```

`withIosXcode27` 内调用一次，`withInfoPlist`/`withPodfileProperties` 使用其结果（`mod.modResults.RCTNewArchEnabled = resolveNewArchEnabled(config).effective`）；`module.exports.resolveNewArchEnabled = resolveNewArchEnabled;` 加入导出行。幂等标记：`PODFILE_CLAMP` 首行加 `# weknora_ios_xcode27_clamp`，`applyPodfileClamp` 的判断改为 `contents.includes('# weknora_ios_xcode27_clamp')`。`raiseDeploymentTargets`：`const current = parseFloat(String(settings.IPHONEOS_DEPLOYMENT_TARGET).replace(/^"|"$/g, ''));`

- [ ] **Step 4: 运行测试确认通过 + 幂等复跑（Review Focus #5）**

Run: `pnpm --filter @weknora/mobile test`
Expected: 全部 PASS；既有「applySceneLifecycle 对已改写内容二次运行不变」用例仍绿。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/plugins/ios-xcode27.js apps/mobile/src/plugins/ios-xcode27.test.ts
git commit -m "fix(mobile/ios): forward cold-launch universal links, honor newArchEnabled with warning, unique clamp anchor, quoted target parsing (R1-F5/F6/F7/F8)"
```

---

## 批次收尾验收

- [ ] **全量回归**：`pnpm --filter @weknora/mobile-core test && pnpm --filter @weknora/mobile test && pnpm exec tsx --test packages/domain/src/mobile/resource-presentation.test.ts`
- [ ] **类型检查**：`pnpm --filter @weknora/mobile exec tsc --noEmit`（覆盖 apps/mobile + 其引用的 mobile-core/domain 源）。
- [ ] **发现覆盖核对**：对照「发现清单与分组」表逐条打勾（24 条有效发现中 23 条 + 20 条顺手项中 18 条；完全排除 3 条见排除项一节，另 R1-F17/F20 为部分修复并声明 Round 2 范围）。
- [ ] **验证不新增依赖**：`git diff --stat pnpm-lock.yaml` 为空。

## Self-Review 记录

1. **覆盖核对**：24 条有效发现中 23 条映射到 Task 1-13（见分组表）；R1-F26 按领域决策排除（排除项一节给出升级路径）。20 条 lowWorthFixing 中纳入 18 条（F3/F7/F8/F14/F15/F16/F18/F27/F31/F32/F33/F34/F35/F36/F37/F42/F45/F47，见各任务括号标注）；F1（证据漂移无法定位）与 F28（独立重构）排除。R1-F17/R1-F20 做部分修复（受控失败 / 显式装配）并声明 Round 2 范围。
2. **占位符扫描**：各任务测试均给出完整断言；复用测试文件内既有夹具（`vault()`、`openShelf(script)`、`officeWithDetail`、`fakeStore`/`remote()`/`ports()`、`TEMPLATE_APP_DELEGATE`）处已点名具体工厂。唯一以文字描述而非代码给出的是 Task 11 的 R1-F31 用例（deferred 卡 refresh + 切部署的时序编排，断言目标已明确：`SHELF_SCOPE` 拒绝且 `activeCredential` 不被旧刷新写回）——执行者按 `runtime-shelf.test.ts` 相邻用例模式补齐编排。
3. **类型一致性**：`scopeKeyOf` 变 async 影响 Task 1-4 全部调用点（已在 Task 1 Step 3 列明）；`TaskOfficeError` 迁移保持 `task-office.ts` re-export 兼容（Task 5）；`ConnectionKind` 增 `'unknown'` 仅影响 `ResourcesScreen.tsx:45` 的文本插值（Task 10 Step 3 已核对）。
4. **Review Focus**：5 项均指定 owning task 与测试步骤（Task 3 Step 5、Task 1 Step 2/5、Task 7 Step 4、Task 13 Step 4）。
