import { leaseActive } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import type { ScopedVault } from '../vault/scoped-vault.ts';
import type { PersistedTaskProjection, TaskProjectionStore, TaskBackendEvent } from './task-detail.ts';

/**
 * Task Store Port 的 Scoped Vault Adapter（module-seams §5.4；生产持久化 = #40 T10）：
 * PersistedTaskProjection 经 projections 仓储加密落盘（scope key 随 lease 派生——
 * Deployment/用户/Tenant 隔离由 vault 保证，AC1）。lease 失效即 VAULT_LEASE（AC2 撤权面）。
 *
 * 行预算（B2-F23 存储选型 ADR 未决的最小可行边界）：SecureStore Adapter 单值上限
 * 2000 base64 字符（apps/mobile/src/adapters/vault-adapters.ts:17）⇒ 明文 ≈1472B，取
 * 1400 留余量。超预算丢最旧事件——恢复正确性不受影响（hydrate 的 mergeEventHistory
 * 以服务端 watermark 为准，老事件由 detail 重取补齐），离线视图保留完整状态快照与最新时间线。
 */
export const PROJECTION_BODY_BUDGET_BYTES = 1400;

const RUN_ID_PATTERN = /^[A-Za-z0-9._-]{1,40}$/;
const projectionIdOf = (runId: string): string => `run.${runId}`;
const sizeOf = (value: string): number => new TextEncoder().encode(value).length;

function serializeWithinBudget(projection: PersistedTaskProjection): string {
  // 增量计量（final review minor）：固定开销与每事件尺寸只序列化计长一次（O(n)）——
  // 原实现每丢一事件全量 JSON.stringify+UTF8 计长（O(n²)），预算 1400B 下无感，
  // 但预算随 B2-F23 SQLite Adapter 放大后会退化。字节数与全量序列化逐字节一致：
  // 体 = 固定前缀 + "[" + events.join(",") + "]"，即 count≥1 时结构开销 = count+1
  // （两括号 + count-1 逗号），空数组 = 2（"[]"）。
  const fixed = sizeOf(JSON.stringify({ ...projection, events: [] })) - 2; // 去掉空数组的 "[]"
  const sizes = projection.events.map((event) => sizeOf(JSON.stringify(event)));
  let byteSum = sizes.reduce((total, size) => total + size, 0);
  const overheadOf = (count: number): number => (count === 0 ? 2 : count + 1);
  let dropped = 0; // 已丢弃的最旧事件数
  while (dropped < projection.events.length
    && fixed + byteSum + overheadOf(projection.events.length - dropped) > PROJECTION_BODY_BUDGET_BYTES) {
    byteSum -= sizes[dropped]!;
    dropped += 1; // 丢最旧：预算优先保快照与最新事件
  }
  return JSON.stringify({ ...projection, events: projection.events.slice(dropped) });
}

function parseProjection(raw: string, runId: string): PersistedTaskProjection {
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    throw new Error('VAULT_PROJECTION');
  }
  if (typeof value !== 'object' || value === null) throw new Error('VAULT_PROJECTION');
  const row = value as Partial<PersistedTaskProjection>;
  if (typeof row.taskId !== 'string') throw new Error('VAULT_PROJECTION');
  if (typeof row.runId !== 'string' || row.runId !== runId) throw new Error('VAULT_PROJECTION');
  if (typeof row.cursor !== 'number' || !Number.isSafeInteger(row.cursor)) throw new Error('VAULT_PROJECTION');
  if (!Array.isArray(row.events) || typeof row.savedAt !== 'string') throw new Error('VAULT_PROJECTION');
  if (row.snapshot !== undefined && (typeof row.snapshot !== 'object' || row.snapshot === null)) throw new Error('VAULT_PROJECTION');
  return { ...(row as PersistedTaskProjection) };
}

export function createVaultTaskProjectionStore(input: { vault: ScopedVault; lease(): ScopeLease | undefined }): TaskProjectionStore {
  const openStore = async () => {
    const lease = input.lease();
    if (!lease || !leaseActive(lease)) throw new Error('VAULT_LEASE');
    return input.vault.open(lease);
  };
  return {
    async load(runId) {
      if (!RUN_ID_PATTERN.test(runId)) return undefined; // 畸形 runId 不触达 vault
      const entry = await (await openStore()).projections.get(projectionIdOf(runId));
      if (entry === undefined) return undefined;
      return parseProjection(entry.body, runId);
    },
    async save(projection) {
      if (!RUN_ID_PATTERN.test(projection.runId)) throw new Error('VAULT_RUN_ID');
      await (await openStore()).projections.put({ id: projectionIdOf(projection.runId), body: serializeWithinBudget(projection) });
    },
  };
}
