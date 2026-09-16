import type { ExecutionEvent } from '@weknora/contracts';
import type { ExecutionScope } from '@weknora/domain/mobile';

export interface EncryptedPayload {
  ciphertext: string;
  nonce: string;
  keyVersion: number;
}

export interface PayloadCipher {
  encrypt(payload: Record<string, unknown>, aad: string): Promise<EncryptedPayload>;
  decrypt(payload: EncryptedPayload, aad: string): Promise<Record<string, unknown>>;
}

export interface ExecutionStorageRow {
  scopeKey: string;
  runID: string;
  seq: number;
  event: Omit<ExecutionEvent, 'payload'> & { payload: EncryptedPayload };
}

export interface ExecutionStorageTransaction {
  getCursor(scopeKey: string, runID: string): Promise<number>;
  findEvent(scopeKey: string, runID: string, seq: number): Promise<ExecutionStorageRow | undefined>;
  insertEvent(row: ExecutionStorageRow): Promise<void>;
  setCursor(scopeKey: string, runID: string, seq: number): Promise<void>;
}

export interface ExecutionStorageDriver {
  transaction<T>(work: (tx: ExecutionStorageTransaction) => Promise<T>): Promise<T>;
  clearScope(scopeKey: string): Promise<void>;
}

export interface StoredExecutionEvent extends ExecutionEvent {}

function scopeKey(scope: ExecutionScope): string {
  return `${scope.origin}\u0000${scope.tenantID}\u0000${scope.userID}`;
}

function aad(scopeKeyValue: string, runID: string, seq: number): string {
  return `${scopeKeyValue}\u0000${runID}\u0000${seq}`;
}

/**
 * Storage boundary for the native SQLite implementation. The driver must map
 * transaction() to one SQLite transaction containing event insert and cursor
 * update; no cursor is advanced by the stream parser itself.
 */
export function createExecutionStorage(driver: ExecutionStorageDriver, cipher: PayloadCipher, scope: ExecutionScope) {
  const key = scopeKey(scope);
  return {
    async commit(event: ExecutionEvent): Promise<void> {
      if (event.seq < 1 || !Number.isSafeInteger(event.seq)) throw new Error('event.seq must be a positive safe integer');
      if (event.run_id.trim() === '') throw new Error('event.run_id must not be empty');
      await driver.transaction(async (tx) => {
        const cursor = await tx.getCursor(key, event.run_id);
        if (event.seq <= cursor) return;
        if (event.seq !== cursor + 1) throw new Error(`execution cursor gap: expected ${cursor + 1}, got ${event.seq}`);
        const existing = await tx.findEvent(key, event.run_id, event.seq);
        if (existing) {
          await tx.setCursor(key, event.run_id, event.seq);
          return;
        }
        const encrypted = await cipher.encrypt(event.payload, aad(key, event.run_id, event.seq));
        await tx.insertEvent({
          scopeKey: key,
          runID: event.run_id,
          seq: event.seq,
          event: { ...event, payload: encrypted },
        });
        await tx.setCursor(key, event.run_id, event.seq);
      });
    },
    async read(runID: string, fromSeq = 1): Promise<StoredExecutionEvent[]> {
      const rows = await driver.transaction(async (tx) => {
        const cursor = await tx.getCursor(key, runID);
        const result: StoredExecutionEvent[] = [];
        for (let seq = Math.max(1, fromSeq); seq <= cursor; seq += 1) {
          const row = await tx.findEvent(key, runID, seq);
          if (!row) continue;
          result.push({ ...row.event, payload: await cipher.decrypt(row.event.payload, aad(key, runID, seq)) });
        }
        return result;
      });
      return rows;
    },
    clear: () => driver.clearScope(key),
  };
}

export type ExecutionStorage = ReturnType<typeof createExecutionStorage>;
