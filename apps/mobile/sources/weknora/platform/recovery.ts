import type { ExecutionEvent, ExecutionSnapshot } from '@weknora/contracts';
import type { ExecutionStorage } from './execution-storage.ts';

/**
 * 重启恢复编排（MX-012）。
 * - 打开 Run：先读本地持久事件；本地有底→从 cursor+1 续订（Last-Event-ID）。
 * - 本地无底（新设备/清理后）→ 快照优先：拉取服务端快照，事件整批落盘后进入增量。
 * - cursor_expired（服务端历史已裁剪）→ 同样走快照重建：本地投影对齐 watermark 后续订，
 *   不重启 Run、不重复消费（快照事件按 seq 幂等提交）。
 * - 终态排空：Run 终态后仍读取到 watermark 为止（drain），随后停止订阅。
 */

export interface SnapshotSource {
  snapshot(runID: string): Promise<ExecutionSnapshot>;
}

export interface RecoveryPlan {
  mode: 'resume-from-cursor' | 'snapshot-first';
  /** resume 模式：本地 cursor（0=无本地底，需快照）。 */
  cursor: number;
  /** snapshot 模式：待落盘的快照事件与 watermark。 */
  snapshot?: ExecutionSnapshot;
}

export async function planRecovery(storage: RecoveryStorageView, runID: string, snapshots: SnapshotSource): Promise<RecoveryPlan> {
  const cursor = await storage.readCursor(runID);
  if (cursor > 0) return { mode: 'resume-from-cursor', cursor };
  const snapshot = await snapshots.snapshot(runID);
  return { mode: 'snapshot-first', cursor: 0, snapshot };
}

export interface RecoveryStorageView {
  readCursor(runID: string): Promise<number>;
}

/** 快照水合：事件按 seq 逐条提交（同 seq 幂等跳过），随后 cursor 停在末位 seq。incomplete 快照 fail-closed。 */
export async function hydrateFromSnapshot(
  storage: SnapshotCommitTarget,
  snapshot: ExecutionSnapshot,
): Promise<number> {
  if (snapshot.incomplete) throw new Error('snapshot is incomplete: refuse partial hydration');
  for (const event of snapshot.events) {
    await storage.commit(event);
  }
  return snapshot.events.length > 0 ? snapshot.events[snapshot.events.length - 1]!.seq : 0;
}

export interface SnapshotCommitTarget {
  commit(event: ExecutionEvent): Promise<void>;
}

export interface ReplaceRunTarget {
  replaceRun(runID: string, events: readonly ExecutionEvent[]): Promise<number>;
}

/**
 * cursor_expired 恢复（MX-012 R1 P2-1）：服务端历史已裁剪时，以新快照原子替换本地底
 * （replaceRun 单事务），替换后从快照末位 seq 续订；不重启 Run、不重复消费。
 */
export async function recoverFromExpiredCursor(
  storage: ReplaceRunTarget,
  runID: string,
  snapshots: SnapshotSource,
): Promise<{ cursor: number; snapshot: ExecutionSnapshot }> {
  const snapshot = await snapshots.snapshot(runID);
  if (snapshot.incomplete) throw new Error('snapshot is incomplete: retry with a complete snapshot');
  const cursor = await storage.replaceRun(runID, snapshot.events);
  return { cursor, snapshot };
}
