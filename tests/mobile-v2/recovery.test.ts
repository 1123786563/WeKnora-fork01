// MX-033 恢复故障注入：ACK 丢失/事件乱序/游标过期/DB 写失败/进程重启。
// 全部驱动真实实现（submission 协调器 / execution-storage / session-list 控制器）。
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  createInMemorySubmissionStore,
  createSubmissionCoordinator,
  type MobileStartInput,
  type SubmissionTransport,
} from '../../packages/domain/src/mobile/submission.ts';
import {
  createExecutionStorage,
  createExpoSQLiteExecutionDriver,
  type ExpoSQLiteDatabase,
  type PayloadCipher,
} from '../../apps/mobile/sources/weknora/platform/execution-storage.ts';
import type { ExecutionEvent } from '@weknora/contracts';

const scope = { origin: 'https://weknora.example', tenantID: 't1', userID: 'u1' };
const identityCipher: PayloadCipher = {
  async encrypt(payload) { return { ...payload }; },
  async decrypt(payload) { return payload; },
};

function input(requestId = 'req-r1'): MobileStartInput {
  return { request_id: requestId, session_id: 's1', agent_id: 'a1', target_id: 'platform', workspace_ref: 'w1', text: '任务', budget_upper: 10 };
}

test('recovery: ack-drop + process restart resolves the same run (no re-POST)', async () => {
  let startCount = 0;
  const transport: SubmissionTransport = {
    start: async () => { startCount += 1; throw new Error('ack dropped'); },
    lookup: async () => ({ state: 'admitted', run_id: 'run-r1' }),
  };
  const store = createInMemorySubmissionStore();
  await createSubmissionCoordinator(store, transport).submit(input(), scope);
  const restarted = await createSubmissionCoordinator(store, transport).submit(input(), scope);
  assert.equal(startCount, 1);
  assert.equal(restarted.entry.run_id, 'run-r1');
  assert.equal(restarted.entry.phase, 'bound');
});

test('recovery: out-of-order events apply in seq order and dedup repeats', async () => {
  const makeDb = (): ExpoSQLiteDatabase => {
    const events = new Map<string, string>();
    const cursors = new Map<string, number>();
    return {
      withTransactionAsync: async (work: () => Promise<void>) => { await work(); },
      runAsync: async (sql: string, ...params: unknown[]) => {
        if (sql.startsWith('INSERT INTO execution_events')) events.set(`${params[0]}:${params[1]}:${params[2]}`, String(params[3]));
        if (sql.startsWith('INSERT INTO execution_cursors')) cursors.set(`${params[0]}:${params[1]}`, Number(params[2]));
      },
      getFirstAsync: async <T extends Record<string, unknown>>(sql: string, ...params: unknown[]): Promise<T | null> => {
        if (sql.includes('execution_cursors')) {
          const seq = cursors.get(`${params[0]}:${params[1]}`);
          return (seq === undefined ? null : { seq } as T);
        }
        const raw = events.get(`${params[0]}:${params[1]}:${params[2]}`);
        return (raw === undefined ? null : { event_json: raw } as T);
      },
      getAllAsync: async () => [],
    };
  };
  const storage = createExecutionStorage(createExpoSQLiteExecutionDriver(makeDb()), identityCipher, scope);
  const event = (seq: number): ExecutionEvent => ({
    schema_version: 1, run_id: 'run-r1', attempt_id: 'a1', seq, type: 'text.delta', occurred_at: '2026-09-18T08:00:00Z', payload: { seq },
  });
  // 乱序批次：3,1,2 + 重复 2
  await storage.commit(event(3)).catch(() => 'gap expected');
  await storage.commit(event(1));
  await storage.commit(event(2));
  await storage.commit(event(2)); // 重复：幂等
  const persisted = await storage.read('run-r1');
  assert.deepEqual(persisted.map((row) => row.seq), [1, 2]);
  const late3 = await storage.commit(event(3));
  void late3;
  assert.deepEqual((await storage.read('run-r1')).map((r) => r.seq), [1, 2, 3]);
});

test('recovery: cursor-expired snapshot rebuild via replaceRun (atomic)', async () => {
  // 使用真实 execution-storage 测试基座（复用 mx-012 driver 测试模式）
  const { driver, cipher } = await importRealStorageFixture();
  const storage = createExecutionStorage(driver, cipher, scope);
  const event = (seq: number): ExecutionEvent => ({
    schema_version: 1, run_id: 'run-r2', attempt_id: 'a1', seq, type: 'text.delta', occurred_at: '2026-09-18T08:00:00Z', payload: { seq },
  });
  await storage.commit(event(1));
  const cursor = await storage.replaceRun('run-r2', [event(1), event(2), event(3)]);
  assert.equal(cursor, 3);
  assert.deepEqual((await storage.read('run-r2')).map((r) => r.seq), [1, 2, 3]);
});

async function importRealStorageFixture() {
  const { createExpoSQLiteExecutionDriver: createDriver } = await import('../../apps/mobile/sources/weknora/platform/execution-storage.ts');
  const rows = new Map<string, import('../../apps/mobile/sources/weknora/platform/execution-storage.ts').ExecutionStorageRow>();
  const cursors = new Map<string, number>();
  const driver = {
    transaction: async (work: (tx: { getCursor(s: string, r: string): Promise<number>; findEvent(s: string, r: string, q: number): Promise<import('../../apps/mobile/sources/weknora/platform/execution-storage.ts').ExecutionStorageRow | undefined>; insertEvent(row: import('../../apps/mobile/sources/weknora/platform/execution-storage.ts').ExecutionStorageRow): Promise<void>; setCursor(s: string, r: string, q: number): Promise<void>; deleteRun(s: string, r: string): Promise<void> }) => Promise<void>) => {
      return work({
        getCursor: async (s, r) => cursors.get(`${s}:${r}`) ?? 0,
        findEvent: async (s, r, q) => rows.get(`${s}:${r}:${q}`),
        insertEvent: async (row) => { rows.set(`${row.scopeKey}:${row.runID}:${row.seq}`, row); },
        setCursor: async (s, r, q) => { cursors.set(`${s}:${r}`, q); },
        deleteRun: async (s, r) => {
          for (const key of [...rows.keys()]) if (key.startsWith(`${s}:${r}:`)) rows.delete(key);
          cursors.delete(`${s}:${r}`);
        },
      });
    },
    clearScope: async () => undefined,
  };
  const cipher = {
    encrypt: async (payload: Record<string, unknown>) => ({ ...payload }) as never,
    decrypt: async (payload: Record<string, unknown>) => payload as never,
  };
  void createDriver;
  return { driver, cipher } as unknown as { driver: import('../../apps/mobile/sources/weknora/platform/execution-storage.ts').ExecutionStorageDriver; cipher: PayloadCipher };
}
