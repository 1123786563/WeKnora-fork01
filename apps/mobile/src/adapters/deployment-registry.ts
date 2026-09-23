import type { DeploymentRegistry } from '@weknora/mobile-core';
import type { SecureStorePort } from './secure-store.ts';

const REGISTRY_KEY = 'weknora.deployment-registry.v1';

/** 注册表条目上限：8 条 origin/label 的持久化 JSON 远小于 Android SecureStore 单值约 2KB 的预算。 */
export const MAX_REGISTRY_ENTRIES = 8;

function parseRecords(raw: string | null): Array<{ origin: string; label: string }> {
  let value: unknown;
  try { value = raw && JSON.parse(raw); } catch { return []; }
  if (!Array.isArray(value)) return [];
  const records: Array<{ origin: string; label: string }> = [];
  for (const entry of value) {
    // 单条畸形：跳过而非整体弃用（整体弃用会让下一次 upsert 覆写掉全部兄弟条目）
    if (!entry || typeof entry !== 'object' || Array.isArray(entry)) continue;
    const record = entry as { origin?: unknown; label?: unknown };
    if (typeof record.origin !== 'string' || record.origin.trim() === '') continue;
    const label = typeof record.label === 'string' && record.label.trim() !== '' ? record.label.trim() : record.origin;
    records.push({ origin: record.origin, label });
  }
  return records;
}

/** OS-backed registered deployment list. Persists presentation-safe origins and labels only, never credentials. */
export function createSecureDeploymentRegistry(store: SecureStorePort): DeploymentRegistry {
  return {
    async list() { return parseRecords(await store.getItemAsync(REGISTRY_KEY)); },
    async upsert(deployment) {
      const records = parseRecords(await store.getItemAsync(REGISTRY_KEY));
      const label = deployment.label.trim() !== '' ? deployment.label.trim() : deployment.origin;
      const next = [{ origin: deployment.origin, label }, ...records.filter((record) => record.origin !== deployment.origin)].slice(0, MAX_REGISTRY_ENTRIES);
      await store.setItemAsync(REGISTRY_KEY, JSON.stringify(next));
    },
    async remove(origin) {
      const records = parseRecords(await store.getItemAsync(REGISTRY_KEY));
      await store.setItemAsync(REGISTRY_KEY, JSON.stringify(records.filter((record) => record.origin !== origin)));
    },
  };
}

/** Loads Expo SecureStore only in the native composition path. */
export function createNativeSecureDeploymentRegistry(): DeploymentRegistry {
  return createSecureDeploymentRegistry(require('expo-secure-store') as SecureStorePort);
}
