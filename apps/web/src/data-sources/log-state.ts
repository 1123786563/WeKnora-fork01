import type { DataSource } from '@weknora/api-client';

export function mergeSyncLogs<T extends { id: string }>(current: readonly T[], incoming: readonly T[], reset: boolean): T[] {
  if (reset) return [...incoming];
  const byId = new Map(current.map((item) => [item.id, item]));
  for (const item of incoming) byId.set(item.id, item);
  return [...byId.values()];
}

export function isSyncRunning(source: Pick<DataSource, 'latest_sync_log'>): boolean {
  return source.latest_sync_log?.status === 'running';
}
