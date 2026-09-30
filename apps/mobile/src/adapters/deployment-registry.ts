import type { DeploymentRegistry } from '@weknora/mobile-core';
import type { SecureStorePort } from './secure-store.ts';

const REGISTRY_KEY = 'weknora.deployment-registry.v1';

function parseRecords(raw: string | null): Array<{ origin: string; label: string }> | undefined {
  try {
    const value: unknown = raw && JSON.parse(raw);
    if (!Array.isArray(value)) return undefined;
    const records: Array<{ origin: string; label: string }> = [];
    for (const entry of value) {
      if (!entry || typeof entry !== 'object' || Array.isArray(entry)) return undefined;
      const record = entry as { origin?: unknown; label?: unknown };
      if (typeof record.origin !== 'string' || record.origin.trim() === '') return undefined;
      const label = typeof record.label === 'string' && record.label.trim() !== '' ? record.label.trim() : record.origin;
      records.push({ origin: record.origin, label });
    }
    return records;
  } catch {
    return undefined;
  }
}

/** OS-backed registered deployment list. Persists presentation-safe origins and labels only, never credentials. */
export function createSecureDeploymentRegistry(store: SecureStorePort): DeploymentRegistry {
  return {
    async list() { return parseRecords(await store.getItemAsync(REGISTRY_KEY)) ?? []; },
    async upsert(deployment) {
      const records = parseRecords(await store.getItemAsync(REGISTRY_KEY)) ?? [];
      const label = deployment.label.trim() !== '' ? deployment.label.trim() : deployment.origin;
      const next = [{ origin: deployment.origin, label }, ...records.filter((record) => record.origin !== deployment.origin)];
      await store.setItemAsync(REGISTRY_KEY, JSON.stringify(next));
    },
    async remove(origin) {
      const records = parseRecords(await store.getItemAsync(REGISTRY_KEY)) ?? [];
      await store.setItemAsync(REGISTRY_KEY, JSON.stringify(records.filter((record) => record.origin !== origin)));
    },
  };
}

/** Loads Expo SecureStore only in the native composition path. */
export function createNativeSecureDeploymentRegistry(): DeploymentRegistry {
  return createSecureDeploymentRegistry(require('expo-secure-store') as SecureStorePort);
}
