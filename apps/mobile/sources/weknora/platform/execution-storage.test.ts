import { describe, expect, it } from 'vitest';
import { createExecutionStorage, createExpoSQLiteExecutionDriver, createSecureStoreAeadCipher, type ExecutionStorageDriver, type ExecutionStorageRow, type ExecutionStorageTransaction, type ExpoSQLiteDatabase } from './execution-storage';
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

  it('provides a runnable SQLite/SecureStore/AEAD seam that survives a new storage instance', async () => {
    const events = new Map<string, string>();
    const cursors = new Map<string, number>();
    const db: ExpoSQLiteDatabase = {
      withTransactionAsync: async (work) => work(),
      runAsync: async (sql, ...params) => {
        if (sql.startsWith('INSERT INTO execution_events')) events.set(`${params[0]}:${params[1]}:${params[2]}`, String(params[3]));
        if (sql.startsWith('INSERT INTO execution_cursors')) cursors.set(`${params[0]}:${params[1]}`, Number(params[2]));
        if (sql.startsWith('DELETE FROM execution_events')) for (const key of events.keys()) if (key.startsWith(`${params[0]}:`)) events.delete(key);
        if (sql.startsWith('DELETE FROM execution_cursors')) for (const key of cursors.keys()) if (key.startsWith(`${params[0]}:`)) cursors.delete(key);
      },
      getFirstAsync: async (sql, ...params) => {
        if (sql.startsWith('SELECT seq')) return (cursors.has(`${params[0]}:${params[1]}`) ? { seq: cursors.get(`${params[0]}:${params[1]}`) } : null) as never;
        const value = events.get(`${params[0]}:${params[1]}:${params[2]}`);
        return (value === undefined ? null : { event_json: value }) as never;
      },
      getAllAsync: async () => [],
    };
    const secure = new Map<string, string>();
    const store = { getItemAsync: async (key: string) => secure.get(key) ?? null, setItemAsync: async (key: string, value: string) => { secure.set(key, value); } };
    const aead = {
      randomBytes: (size: number) => new Uint8Array(size).fill(7),
      encrypt: (message: Uint8Array) => message,
      decrypt: (ciphertext: Uint8Array) => ciphertext,
    };
    const first = createExecutionStorage(createExpoSQLiteExecutionDriver(db), createSecureStoreAeadCipher(store, aead), { origin: 'https://a', tenantID: 't', userID: 'u' });
    await first.commit(event(1));
    const restarted = createExecutionStorage(createExpoSQLiteExecutionDriver(db), createSecureStoreAeadCipher(store, aead), { origin: 'https://a', tenantID: 't', userID: 'u' });
    await restarted.commit(event(1));
    expect((await restarted.read('r'))[0]?.payload.value).toBe(1);
    expect(secure.size).toBe(1);
  });

  it('round trips a large encrypted payload without argument-spread overflow', async () => {
    const secure = new Map<string, string>();
    const store = { getItemAsync: async (key: string) => secure.get(key) ?? null, setItemAsync: async (key: string, value: string) => { secure.set(key, value); } };
    const aead = {
      randomBytes: (size: number) => new Uint8Array(size).fill(9),
      encrypt: (message: Uint8Array) => message,
      decrypt: (ciphertext: Uint8Array) => ciphertext,
    };
    const cipher = createSecureStoreAeadCipher(store, aead);
    const payload = { text: 'x'.repeat(200_000) };
    const encrypted = await cipher.encrypt(payload, 'scope\u0000run\u00001');
    expect(encrypted.ciphertext.length).toBeGreaterThan(200_000);
    expect(await cipher.decrypt(encrypted, 'scope\u0000run\u00001')).toEqual(payload);
  });
});
