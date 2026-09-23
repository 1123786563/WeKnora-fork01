import type { ScopeLease } from '../runtime/types.ts';
import { leaseActive, leaseScopeOf, type LeaseScope } from '../runtime/scope-lease.ts';
import type { CipherPort, KeyStorePort, ScopedVaultPorts, VaultStoragePort } from './ports.ts';

const KEY_LENGTH = 32;
const DRAFT_ID_PATTERN = /^[A-Za-z0-9._-]{1,64}$/;
/** 保留字（R1-F10）：即使 v2 键布局已把索引/行命名空间分离，'index' 仍显式拒绝——
 * put/get/remove 一致（对称校验，R1-F40），防止任何未来键布局回退重开劫持面。 */
const DRAFT_RESERVED_IDS: ReadonlySet<string> = new Set(['index']);
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

/** 行键段：base64url(id)。合法 id（[A-Za-z0-9._-]{1,64}）的编码产物落在该字符集之外，
 * 与索引键 `${scopeKey}.ix` 命名空间天然分离——id='ix' 等保留字无法劫持索引键（R1-F10）。 */
const draftKeySegment = (id: string): string => btoa(id).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');

export async function scopeKeyOf(scope: LeaseScope): Promise<string> {
  // v2：分段拼接后取 SHA-256 hex 摘要。动机：(1) v1 的 encodeURIComponent 产物含 %，
  // Android expo-secure-store 键字符集（[A-Za-z0-9._-]）直接拒绝（R1-F39）；
  // (2) 点号分段在段值含点时可拼出相同键（R1-F15）。摘要同时解决两者且长度固定。
  // 数据兼容：v1 键在 Android 上从未成功写入（R1-F39）、分支未发布，视为无生产数据，不做迁移。
  const text = new TextEncoder().encode(`${scope.deploymentOrigin}\n${scope.userId}\n${scope.tenantId}`);
  const digest = new Uint8Array(await globalThis.crypto.subtle.digest('SHA-256', text));
  return `weknora.vault.v2.${[...digest].slice(0, 20).map((byte) => byte.toString(16).padStart(2, '0')).join('')}`;
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
  const indexKey = (scopeKey: string): string => `${scopeKey}.ix`;
  const rowKey = (scopeKey: string, id: string): string => `${scopeKey}.d.${draftKeySegment(id)}`;
  const assertDraftId = (id: string): void => {
    if (!DRAFT_ID_PATTERN.test(id) || DRAFT_RESERVED_IDS.has(id)) throw new Error('VAULT_ID');
  };
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
      const scopeKey = await scopeKeyOf(scope);
      const session = await requireSession(scopeKey);
      const drafts: ScopedDraftRepository = {
        async put(input) {
          assertAccessible(scopeLease, session);
          assertDraftId(input.id);
          const ids = await readIndex(scopeKey);
          const entry: DraftEntry = { id: input.id, body: input.body, updatedAt: new Date().toISOString() };
          await ports.storage.write(rowKey(scopeKey, input.id), await sealRow(session.key, entry));
          if (!ids.includes(input.id)) await writeIndex(scopeKey, [...ids, input.id]);
        },
        async get(id) {
          assertAccessible(scopeLease, session);
          assertDraftId(id);
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
          assertDraftId(id);
          await ports.storage.delete(rowKey(scopeKey, id));
          await writeIndex(scopeKey, (await readIndex(scopeKey)).filter((existing) => existing !== id));
        },
      };
      return { drafts };
    },
    async rotate(scopeLease) {
      if (!leaseActive(scopeLease)) throw new Error('VAULT_LEASE');
      const scopeKey = await scopeKeyOf(requireScope(scopeLease));
      const session = sessions.get(scopeKey);
      const oldKey = session && !session.destroyed ? session.key : await ports.keyStore.readWrappedKey(scopeKey);
      if (!oldKey) return;
      // 两阶段原子 rotate：任一行解密失败时在任何写入之前中止，健康行保持旧 key 可读。
      const decrypted: Array<{ id: string; entry: DraftEntry }> = [];
      for (const id of await readIndex(scopeKey)) {
        const raw = await ports.storage.read(rowKey(scopeKey, id));
        if (raw === null) continue;
        decrypted.push({ id, entry: await openRow(oldKey, raw) });
      }
      const newKey = randomBytes(KEY_LENGTH);
      for (const { id, entry } of decrypted) {
        await ports.storage.write(rowKey(scopeKey, id), await sealRow(newKey, entry));
      }
      await ports.keyStore.writeWrappedKey(scopeKey, newKey);
      if (session && !session.destroyed) session.key = newKey;
    },
    async revoke(scopeLease, _reason) {
      const scope = requireScope(scopeLease);
      const scopeKey = await scopeKeyOf(scope);
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
