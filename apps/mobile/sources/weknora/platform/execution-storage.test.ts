import { describe, expect, it } from 'vitest';
import { createExecutionStorage, type ExecutionStorageDriver, type ExecutionStorageRow, type ExecutionStorageTransaction } from './execution-storage';
import type { ExecutionEvent } from '@weknora/contracts';

const event = (seq: number): ExecutionEvent => ({ schema_version: 1, run_id: 'r', attempt_id: 'a', seq, type: 'future.event', occurred_at: '2026-09-12T00:00:00Z', payload: { value: seq } });

function driver(): ExecutionStorageDriver {
  const rows = new Map<string, ExecutionStorageRow>();
  const cursors = new Map<string, number>();
  const tx = (): ExecutionStorageTransaction => ({
    getCursor: async (scope, run) => cursors.get(`${scope}:${run}`) ?? 0,
    findEvent: async (scope, run, seq) => rows.get(`${scope}:${run}:${seq}`),
    insertEvent: async (row) => { rows.set(`${row.scopeKey}:${row.runID}:${row.seq}`, row); },
    setCursor: async (scope, run, seq) => { cursors.set(`${scope}:${run}`, seq); },
  });
  return { transaction: async (work) => work(tx()), clearScope: async (scope) => { for (const key of rows.keys()) if (key.startsWith(`${scope}:`)) rows.delete(key); } };
}

const cipher = {
  encrypt: async (payload: Record<string, unknown>) => ({ ciphertext: JSON.stringify(payload), nonce: crypto.randomUUID(), keyVersion: 1 }),
  decrypt: async (payload: { ciphertext: string }) => JSON.parse(payload.ciphertext) as Record<string, unknown>,
};

describe('execution storage', () => {
  it('commits projection and cursor through one transaction and decrypts on read', async () => {
    const storage = createExecutionStorage(driver(), cipher, { origin: 'https://a', tenantID: 't', userID: 'u' });
    await storage.commit(event(1));
    await storage.commit(event(1));
    expect(await storage.read('r')).toHaveLength(1);
    expect((await storage.read('r'))[0]?.payload.value).toBe(1);
  });

  it('rejects gaps before writing and isolates clear by scope', async () => {
    const d = driver();
    const a = createExecutionStorage(d, cipher, { origin: 'https://a', tenantID: 't1', userID: 'u' });
    const b = createExecutionStorage(d, cipher, { origin: 'https://a', tenantID: 't2', userID: 'u' });
    await expect(a.commit(event(2))).rejects.toThrow(/cursor gap/);
    await a.commit(event(1));
    await b.commit(event(1));
    await a.clear();
    expect(await a.read('r')).toEqual([]);
    expect(await b.read('r')).toHaveLength(1);
  });
});
