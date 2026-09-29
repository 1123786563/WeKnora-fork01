import type { ScopeLease } from '../runtime/types.ts';
import { leaseActive, leaseScopeOf, type LeaseScope } from '../runtime/scope-lease.ts';
import type { CipherPort, KeyStorePort, ScopedVaultPorts, VaultStoragePort } from './ports.ts';
import { base64ToBytes, bytesToBase64 } from './base64.ts';

const KEY_LENGTH = 32;
const DRAFT_ID_PATTERN = /^[A-Za-z0-9._-]{1,64}$/;
/** 保留字（R1-F10）：即使 v2 键布局已把索引/行命名空间分离，'index' 仍显式拒绝——
 * put/get/remove 一致（对称校验，R1-F40），防止任何未来键布局回退重开劫持面。 */
const DRAFT_RESERVED_IDS: ReadonlySet<string> = new Set(['index']);
const RETENTION_DAYS = 30;

/** ScopedStore 的两个领域仓储（spec §9.2 按领域仓储分组）：drafts 与 event projections。
 *  行键命名空间段沿用 R1-F10 设计——合法 id 的 base64url 编码不含 '.'，与 'd'/'p' 段
 *  及索引键（'ix'/'pix'）天然分离，id 无法劫持另一命名空间的行或索引。 */
type StoreNamespace = 'drafts' | 'projections';
const ROW_SEGMENT: Record<StoreNamespace, string> = { drafts: 'd', projections: 'p' };
const INDEX_SEGMENT: Record<StoreNamespace, string> = { drafts: 'ix', projections: 'pix' };

export type VaultRevokeReason = 'tenant-switch' | 'deployment-change' | 'sign-out' | 'dispose';

export interface DraftEntry { id: string; body: string; updatedAt: string }

export interface ScopedDraftRepository {
  put(input: { id: string; body: string }): Promise<void>;
  get(id: string): Promise<DraftEntry | undefined>;
  list(): Promise<DraftEntry[]>;
  remove(id: string): Promise<void>;
}

export interface ScopedStore { drafts: ScopedDraftRepository; projections: ScopedDraftRepository }

export interface VaultPolicy { categories: ReadonlyArray<{ category: string; retentionDays: number }> }

export interface ScopedVault {
  open(scopeLease: ScopeLease): Promise<ScopedStore>;
  rotate(scopeLease: ScopeLease): Promise<void>;
  revoke(scopeLease: ScopeLease, reason: VaultRevokeReason): Promise<void>;
  inspectPolicy(): VaultPolicy;
}

interface ScopeSession { key: Uint8Array; destroyed: boolean }

/** 行键段：base64url(id)。合法 id（[A-Za-z0-9._-]{1,64}）的编码产物落在该字符集之外，
 * 与索引键 `${scopeKey}.ix` 命名空间天然分离——id='ix' 等保留字无法劫持索引键（R1-F10）。 */
const draftKeySegment = (id: string): string => bytesToBase64(new TextEncoder().encode(id)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');

export async function scopeKeyOf(scope: LeaseScope): Promise<string> {
  // v2：分段拼接后取 SHA-256 hex 摘要。动机：(1) v1 的 encodeURIComponent 产物含 %，
  // Android expo-secure-store 键字符集（[A-Za-z0-9._-]）直接拒绝（R1-F39）；
  // (2) 点号分段在段值含点时可拼出相同键（R1-F15）。摘要同时解决两者且长度固定。
  // 数据兼容：v1 键在 Android 上从未成功写入（R1-F39）、分支未发布，视为无生产数据，不做迁移。
  const text = new TextEncoder().encode(`${scope.deploymentOrigin}\n${scope.userId}\n${scope.tenantId}`);
  const digest = new Uint8Array(await globalThis.crypto.subtle.digest('SHA-256', text));
  return `weknora.vault.v2.${[...digest].slice(0, 20).map((byte) => byte.toString(16).padStart(2, '0')).join('')}`;
}

/**
 * Scoped Vault（Spec §9）：唯一拥有 deployment/user/tenant scope 下的加密缓存与撤销。
 * ScopedStore 暴露 drafts 与 projections 两个领域仓储；每次访问重新校验 lease 有效性，
 * 任何没有有效 Scope Lease 的访问失败（VAULT_LEASE）。
 */
export function createScopedVault(ports: ScopedVaultPorts): ScopedVault {
  const sessions = new Map<string, ScopeSession>();
  const sessionFlights = new Map<string, Promise<ScopeSession>>();
  // 本实例写入过的行 id（Review Focus #1）：索引损坏时 revoke 无法枚举行，
  // 以写入侧记忆兜底尽力擦除（跨实例残留行的 wrapped key 已被覆写，密文不可解）。
  const knownRowIds = new Map<string, Set<string>>();
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
  // 会话创建 single-flight（R1-F41）：并发首次 open 必须共享同一次 keyStore 读+写，
  // 否则两个调用方各持一把 key，先写者的行被后写者的 key 永久锁死。
  const requireSession = async (scopeKey: string): Promise<ScopeSession> => {
    const existing = sessions.get(scopeKey);
    if (existing && !existing.destroyed) return existing;
    const inFlight = sessionFlights.get(scopeKey);
    if (inFlight) return inFlight;
    const flight = (async (): Promise<ScopeSession> => {
      const wrapped = await ports.keyStore.readWrappedKey(scopeKey);
      if (wrapped !== undefined) {
        // 长度异常 = 持久化值损坏：覆写等于把旧密文永久变成不可解且无告警（R1-F13），必须 fail closed。
        if (wrapped.length !== KEY_LENGTH) throw new Error('VAULT_KEYSTORE');
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
    try {
      return await flight;
    } finally {
      sessionFlights.delete(scopeKey);
    }
  };
  const indexKeyOf = (scopeKey: string, namespace: StoreNamespace): string => `${scopeKey}.${INDEX_SEGMENT[namespace]}`;
  const rowKeyOf = (scopeKey: string, namespace: StoreNamespace, id: string): string => `${scopeKey}.${ROW_SEGMENT[namespace]}.${draftKeySegment(id)}`;
  const assertDraftId = (id: string): void => {
    if (!DRAFT_ID_PATTERN.test(id) || DRAFT_RESERVED_IDS.has(id)) throw new Error('VAULT_ID');
  };
  // per-scope 互斥队列（R1-F12/F11）：同一 scope 的仓储操作（含 rotate/revoke）串行执行，
  // 消除「并发 put 各自读旧索引 → 后写覆盖前写丢条目」与「rotate 期间 put 半新半旧」两类竞态。
  const scopeTails = new Map<string, Promise<unknown>>();
  const enqueue = <T>(scopeKey: string, action: () => Promise<T>): Promise<T> => {
    const tail = scopeTails.get(scopeKey) ?? Promise.resolve();
    const next = tail.then(action, action);
    scopeTails.set(scopeKey, next.catch(() => undefined));
    return next;
  };
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
  const now = (): string => ports.now?.() ?? new Date().toISOString();
  const sealRow = async (key: Uint8Array, entry: DraftEntry): Promise<string> => {
    const sealed = await ports.cipher.seal(key, text.encode(JSON.stringify(entry)));
    return bytesToBase64(sealed);
  };
  // expectedId 绑定（R1-F42 最小充分修复）：行内容必须与键上的 id 一致，阻断同 scope 内
  // 把 A 的密文挪到 B 键下的行移动攻击；不引入 AAD 密文格式变更。
  const openRow = async (key: Uint8Array, raw: string, expectedId: string): Promise<DraftEntry> => {
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
    if (entry.id !== expectedId) throw new Error('VAULT_DECRYPT');
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
      const scopeKey = await scopeKeyOf(scope);
      const session = await requireSession(scopeKey);
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
    },
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
    async revoke(scopeLease, _reason) {
      const scope = requireScope(scopeLease);
      const scopeKey = await scopeKeyOf(scope);
      const session = sessions.get(scopeKey);
      if (session) session.destroyed = true;
      sessions.delete(scopeKey);
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
    },
    inspectPolicy() {
      return { categories: [{ category: 'drafts', retentionDays: RETENTION_DAYS }, { category: 'projections', retentionDays: RETENTION_DAYS }] };
    },
  };
}
