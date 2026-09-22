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
