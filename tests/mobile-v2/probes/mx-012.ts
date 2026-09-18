// MX-012 probe · 磁盘满事务原子性观察器
// 真实 createExecutionStorage（真实 driver 事务/加密/AAD/cursor 逻辑）+ 注入式故障 driver：
// seq41 提交成功后，seq42 的 insertEvent 抛「磁盘满」→ 事务回滚 → cursor 不前进、事件不落库。
// 重启读回（同一持久层状态的新 storage 实例）观测最终事实。
import {
  createExecutionStorage,
  createExpoSQLiteExecutionDriver,
  type ExpoSQLiteDatabase,
  type PayloadCipher,
} from '../../../apps/mobile/sources/weknora/platform/execution-storage.ts';
import type { ExecutionEvent } from '@weknora/contracts';
import type { ExecutionScope } from '@weknora/domain/mobile';

export interface ProbeInput {
  fixture: string;
  fault: string;
}

export interface Observation {
  cursor: number;
  persistedSeqs: number[];
}

const identityCipher: PayloadCipher = {
  async encrypt(payload) { return { ...payload }; },
  async decrypt(payload) { return payload; },
};

function makeDb(): ExpoSQLiteDatabase & { events: Map<string, string>; cursors: Map<string, number> } {
  const events = new Map<string, string>();
  const cursors = new Map<string, number>();
  let diskFullAtSeq: number | null = null;
  return {
    events,
    cursors,
    withTransactionAsync: async (work: () => Promise<void>) => { await work(); },
    runAsync: async (sql: string, ...params: unknown[]) => {
      if (sql.startsWith('INSERT INTO execution_events')) {
        const seq = Number(params[2]);
        if (diskFullAtSeq !== null && seq >= diskFullAtSeq) throw new Error('SQLITE_FULL: database or disk is full');
        events.set(`${params[0]}:${params[1]}:${seq}`, String(params[3]));
        return;
      }
      if (sql.startsWith('INSERT INTO execution_cursors')) {
        cursors.set(`${params[0]}:${params[1]}`, Number(params[2]));
        return;
      }
      if (sql.startsWith('DELETE FROM execution_events')) {
        for (const key of [...events.keys()]) if (key.startsWith(`${params[0]}:`)) events.delete(key);
        return;
      }
      if (sql.startsWith('DELETE FROM execution_cursors')) {
        for (const key of [...cursors.keys()]) if (key.startsWith(`${params[0]}:`)) cursors.delete(key);
        return;
      }
      if (sql.startsWith('CREATE TABLE')) return;
      throw new Error(`unexpected sql: ${sql}`);
    },
    getFirstAsync: async <T extends Record<string, unknown>>(sql: string, ...params: unknown[]): Promise<T | null> => {
      if (sql.includes('execution_cursors')) {
        const seq = cursors.get(`${params[0]}:${params[1]}`);
        return (seq === undefined ? null : { seq } as T);
      }
      if (sql.includes('execution_events')) {
        const raw = events.get(`${params[0]}:${params[1]}:${params[2]}`);
        return (raw === undefined ? null : { event_json: raw } as T);
      }
      return null;
    },
    getAllAsync: async () => [],
    __setDiskFull(seq: number) { diskFullAtSeq = seq; },
  } as ExpoSQLiteDatabase & { events: Map<string, string>; cursors: Map<string, number>; __setDiskFull(seq: number): void };
}

function event(seq: number): ExecutionEvent {
  return {
    schema_version: 1,
    run_id: 'run-mx012',
    attempt_id: 'attempt-1',
    seq,
    type: seq === 42 ? 'disk.full.trigger' : 'text.delta',
    occurred_at: '2026-09-18T08:00:00Z',
    payload: { seq },
  };
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'persisted-seq41' || input.fault !== 'disk-full-on-seq42') {
    throw new Error(`unsupported fixture/fault: ${input.fixture}/${input.fault}`);
  }
  const scope: ExecutionScope = { origin: 'https://weknora.example', tenantID: 't1', userID: 'u1' };
  const db = makeDb();
  const driver = createExpoSQLiteExecutionDriver(db);
  const storage = createExecutionStorage(driver, identityCipher, scope);

  // 铺底 1..41（真实连续提交），随后在 42 注入磁盘满
  for (let seq = 1; seq <= 41; seq += 1) await storage.commit(event(seq));
  // 注入磁盘满：seq42 的插入失败
  db.__setDiskFull(42);
  let failed = false;
  try {
    await storage.commit(event(42));
  } catch (error) {
    failed = error instanceof Error && error.message.includes('SQLITE_FULL');
  }
  if (!failed) throw new Error('disk-full fault must surface from the storage commit');

  // 重启：同一持久层状态上的新 storage 实例读回（窗口=故障前 cursor 之后：磁盘满写入是否残留）
  const restarted = createExecutionStorage(createExpoSQLiteExecutionDriver(db), identityCipher, scope);
  const cursor = await restarted.readCursor('run-mx012');
  const persisted = await restarted.read('run-mx012', 41);
  return {
    cursor,
    persistedSeqs: persisted.map((row) => row.seq),
  };
}
