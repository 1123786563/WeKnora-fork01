import type { Credential, CredentialAdapter } from '@weknora/api-client';

export interface CredentialStore {
  get(key: string): Promise<string | null>;
  set(key: string, value: string): Promise<void>;
  remove(key: string): Promise<void>;
}

function valid(value: unknown): value is Credential {
  if (!value || typeof value !== 'object') return false;
  const record = value as Record<string, unknown>;
  if (record.kind === 'anonymous') return true;
  if (record.kind === 'bearer' && typeof record.accessToken === 'string' && record.accessToken.trim()) {
    return record.refreshToken === undefined || (typeof record.refreshToken === 'string' && record.refreshToken.trim().length > 0);
  }
  return false;
}

export function createCredentials(store: CredentialStore, key: string): CredentialAdapter {
  return {
    async read() {
      const raw = await store.get(key);
      if (!raw) return { kind: 'anonymous' };
      try {
        const parsed: unknown = JSON.parse(raw);
        return valid(parsed) ? parsed : { kind: 'anonymous' };
      } catch {
        return { kind: 'anonymous' };
      }
    },
    async write(value) {
      if (!valid(value) || value.kind === 'anonymous') {
        if (value.kind === 'anonymous') await store.remove(key);
        return;
      }
      await store.set(key, JSON.stringify(value));
    },
    clear() { return store.remove(key); },
  };
}

export function productCredentialKey(origin: string): string {
  return `weknora:credentials:${encodeURIComponent(new URL(origin).origin)}`;
}
