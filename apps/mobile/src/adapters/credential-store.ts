import type { CredentialStore, StoredCredential } from '@weknora/mobile-core';
import type { SecureStorePort } from './secure-store.ts';

const CREDENTIAL_PREFIX = 'weknora.credentials.v1.';

function normalizedOrigin(origin: string): string {
  const parsed = new URL(origin);
  if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.search || parsed.hash || parsed.pathname !== '/') throw new Error('CREDENTIAL_ORIGIN');
  return parsed.origin;
}

function key(origin: string): string {
  return `${CREDENTIAL_PREFIX}${encodeURIComponent(normalizedOrigin(origin))}`;
}

function parseCredential(raw: string | null): StoredCredential | undefined {
  try {
    const value: unknown = raw && JSON.parse(raw);
    if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
    const credential = value as Partial<StoredCredential>;
    if (typeof credential.token !== 'string' || credential.token.trim() === '') return undefined;
    if (typeof credential.refreshToken !== 'string' || credential.refreshToken.trim() === '') return undefined;
    return { token: credential.token, refreshToken: credential.refreshToken };
  } catch {
    return undefined;
  }
}

/** OS-backed credential vault scoped by the normalized Runtime deployment origin. */
export function createSecureCredentialStore(store: SecureStorePort): CredentialStore {
  return {
    async read(origin) { return parseCredential(await store.getItemAsync(key(origin))); },
    async write(origin, credential) { await store.setItemAsync(key(origin), JSON.stringify({ token: credential.token, refreshToken: credential.refreshToken })); },
    async clear(origin) { await store.deleteItemAsync(key(origin)); },
  };
}

/** Loads Expo SecureStore only in the native composition path. */
export function createNativeSecureCredentialStore(): CredentialStore {
  return createSecureCredentialStore(require('expo-secure-store') as SecureStorePort);
}
