import type { DeploymentStore } from '@weknora/mobile-core';
import type { SecureStorePort } from './secure-store.ts';

const ACTIVE_DEPLOYMENT_KEY = 'weknora.active-deployment.v1';

function parseDeployment(raw: string | null): { origin: string; label?: string } | undefined {
  try {
    const value: unknown = raw && JSON.parse(raw);
    if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
    const deployment = value as { origin?: unknown; label?: unknown };
    if (typeof deployment.origin !== 'string') return undefined;
    return { origin: deployment.origin, ...(typeof deployment.label === 'string' ? { label: deployment.label } : {}) };
  } catch {
    return undefined;
  }
}

/** OS-backed, presentation-safe active deployment selector. */
export function createSecureDeploymentStore(store: SecureStorePort): DeploymentStore {
  return {
    async read() { return parseDeployment(await store.getItemAsync(ACTIVE_DEPLOYMENT_KEY)); },
    async write(deployment) { await store.setItemAsync(ACTIVE_DEPLOYMENT_KEY, JSON.stringify(deployment)); },
    async clear() { await store.deleteItemAsync(ACTIVE_DEPLOYMENT_KEY); },
  };
}

/** Loads Expo SecureStore only in the native composition path. */
export function createNativeSecureDeploymentStore(): DeploymentStore {
  return createSecureDeploymentStore(require('expo-secure-store') as SecureStorePort);
}
