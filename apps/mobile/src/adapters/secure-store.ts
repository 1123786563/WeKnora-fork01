import type { PendingOidc, PendingOidcStore } from '@weknora/mobile-core';

const PENDING_OIDC_KEY = 'weknora.pending-oidc.v1';

export interface SecureStorePort {
  getItemAsync(key: string): Promise<string | null>;
  setItemAsync(key: string, value: string): Promise<void>;
  deleteItemAsync(key: string): Promise<void>;
}

function parsePending(raw: string | null): PendingOidc | undefined {
  if (!raw) return undefined;
  try {
    const value: unknown = JSON.parse(raw);
    if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
    const pending = value as Partial<PendingOidc>;
    if ([pending.deploymentOrigin, pending.state, pending.codeVerifier, pending.redirectUri].some((field) => typeof field !== 'string' || field.trim() === '')) return undefined;
    return {
      deploymentOrigin: pending.deploymentOrigin!, state: pending.state!, codeVerifier: pending.codeVerifier!, redirectUri: pending.redirectUri!,
    };
  } catch {
    return undefined;
  }
}

/** Stores the short-lived OIDC state and verifier in the OS-backed secure store. */
export function createSecurePendingOidcStore(store: SecureStorePort): PendingOidcStore {
  return {
    async savePending(input) {
      await store.setItemAsync(PENDING_OIDC_KEY, JSON.stringify(input));
    },
    async loadPending() {
      return parsePending(await store.getItemAsync(PENDING_OIDC_KEY));
    },
    async consumePending() {
      const raw = await store.getItemAsync(PENDING_OIDC_KEY);
      await store.deleteItemAsync(PENDING_OIDC_KEY);
      return parsePending(raw);
    },
  };
}

/** Loads Expo SecureStore only in the native composition path. */
export function createNativeSecurePendingOidcStore(): PendingOidcStore {
  return createSecurePendingOidcStore(require('expo-secure-store') as SecureStorePort);
}
