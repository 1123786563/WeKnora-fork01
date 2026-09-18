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
  deleteRun(scopeKey: string, runID: string): Promise<void>;
}

export interface ExecutionStorageDriver {
  transaction<T>(work: (tx: ExecutionStorageTransaction) => Promise<T>): Promise<T>;
  clearScope(scopeKey: string): Promise<void>;
}

/**
 * Expo SQLite surface mirroring the official expo-sqlite API (withTransactionAsync
 * resolves Promise<void> — the transaction result is NOT returned by the SDK), kept
 * injectable for restart tests (G06: never fake the native return type).
 */
export interface ExpoSQLiteDatabase {
  withTransactionAsync(work: () => Promise<void>): Promise<void>;
  runAsync(sql: string, ...params: unknown[]): Promise<unknown>;
  getFirstAsync<T extends Record<string, unknown>>(sql: string, ...params: unknown[]): Promise<T | null>;
  getAllAsync<T extends Record<string, unknown>>(sql: string, ...params: unknown[]): Promise<T[]>;
}

export function createExpoSQLiteExecutionDriver(db: ExpoSQLiteDatabase): ExecutionStorageDriver {
  const schemaEvents = `CREATE TABLE IF NOT EXISTS execution_events (
    scope_key TEXT NOT NULL, run_id TEXT NOT NULL, seq INTEGER NOT NULL,
    event_json TEXT NOT NULL, PRIMARY KEY (scope_key, run_id, seq)
  )`;
  const schemaCursors = `CREATE TABLE IF NOT EXISTS execution_cursors (
    scope_key TEXT NOT NULL, run_id TEXT NOT NULL, seq INTEGER NOT NULL,
    PRIMARY KEY (scope_key, run_id)
  )`;
  const initialized = Promise.all([db.runAsync(schemaEvents), db.runAsync(schemaCursors)]).then(() => undefined);
  return {
    async transaction<T>(work: (tx: ExecutionStorageTransaction) => Promise<T>) {
      await initialized;
      // Official expo-sqlite withTransactionAsync resolves void: capture the
      // work result through a closure instead of trusting a fabricated return.
      let result: T | undefined;
      await db.withTransactionAsync(async () => {
        result = await work({
        getCursor: async (scope, run) => (await db.getFirstAsync<{ seq: number }>('SELECT seq FROM execution_cursors WHERE scope_key = ? AND run_id = ?', scope, run))?.seq ?? 0,
        findEvent: async (scope, run, seq) => {
          const row = await db.getFirstAsync<{ event_json: string }>('SELECT event_json FROM execution_events WHERE scope_key = ? AND run_id = ? AND seq = ?', scope, run, seq);
          if (!row) return undefined;
          return JSON.parse(row.event_json) as ExecutionStorageRow;
        },
        insertEvent: async (row) => {
          await db.runAsync('INSERT INTO execution_events(scope_key, run_id, seq, event_json) VALUES (?, ?, ?, ?)', row.scopeKey, row.runID, row.seq, JSON.stringify(row));
        },
        setCursor: async (scope, run, seq) => {
          await db.runAsync('INSERT INTO execution_cursors(scope_key, run_id, seq) VALUES (?, ?, ?) ON CONFLICT(scope_key, run_id) DO UPDATE SET seq = excluded.seq', scope, run, seq);
        },
        deleteRun: async (scope, run) => {
          await db.runAsync('DELETE FROM execution_events WHERE scope_key = ? AND run_id = ?', scope, run);
          await db.runAsync('DELETE FROM execution_cursors WHERE scope_key = ? AND run_id = ?', scope, run);
        },
        });
      });
      return result as T;
    },
    async clearScope(scope) {
      await initialized;
      await db.withTransactionAsync(async () => {
        await db.runAsync('DELETE FROM execution_events WHERE scope_key = ?', scope);
        await db.runAsync('DELETE FROM execution_cursors WHERE scope_key = ?', scope);
      });
    },
  };
}

export interface SecureKeyStore {
  getItemAsync(key: string): Promise<string | null>;
  setItemAsync(key: string, value: string): Promise<void>;
}

export interface AeadBox {
  randomBytes(size: number): Uint8Array;
  encrypt(message: Uint8Array, aad: Uint8Array, nonce: Uint8Array, key: Uint8Array): Uint8Array;
  decrypt(ciphertext: Uint8Array, aad: Uint8Array, nonce: Uint8Array, key: Uint8Array): Uint8Array;
}

function b64(bytes: Uint8Array): string {
  // Avoid spreading large payloads into a single call stack frame on Hermes/JSC.
  let binary = '';
  for (let offset = 0; offset < bytes.length; offset += 0x8000) {
    binary += String.fromCharCode(...bytes.subarray(offset, offset + 0x8000));
  }
  return btoa(binary);
}
const unb64 = (value: string): Uint8Array => Uint8Array.from(atob(value), (char) => char.charCodeAt(0));

/** SecureStore-backed key with an injected libsodium XChaCha20-Poly1305 implementation. */
export function createSecureStoreAeadCipher(store: SecureKeyStore, aead: AeadBox, keyName = 'weknora.execution.key.v1'): PayloadCipher {
  let cached: Uint8Array | undefined;
  // single-flight（G06/MX-012 R1）：密钥初始化以 in-flight promise 共享——并发首次加密
  // 必须用同一把密钥；失败时清空以便重试（不留半初始化状态）。
  let keyInFlight: Promise<Uint8Array> | undefined;
  function key(): Promise<Uint8Array> {
    if (cached) return Promise.resolve(cached);
    keyInFlight ??= (async () => {
      const existing = await store.getItemAsync(keyName);
      const value = existing ? unb64(existing) : aead.randomBytes(32);
      if (!existing) await store.setItemAsync(keyName, b64(value));
      if (value.length !== 32) throw new Error('execution encryption key must be 32 bytes');
      cached = value;
      return value;
    })().catch((error: unknown) => {
      keyInFlight = undefined;
      throw error;
    });
    return keyInFlight;
  }
  return {
    async encrypt(payload, aad) {
      const nonce = aead.randomBytes(24);
      const ciphertext = aead.encrypt(new TextEncoder().encode(JSON.stringify(payload)), new TextEncoder().encode(aad), nonce, await key());
      return { ciphertext: b64(ciphertext), nonce: b64(nonce), keyVersion: 1 };
    },
    async decrypt(payload, aad) {
      if (payload.keyVersion !== 1) throw new Error('unsupported execution encryption key version');
      const plaintext = aead.decrypt(unb64(payload.ciphertext), new TextEncoder().encode(aad), unb64(payload.nonce), await key());
      return JSON.parse(new TextDecoder().decode(plaintext)) as Record<string, unknown>;
    },
  };
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
    async readCursor(runID: string): Promise<number> {
      return driver.transaction(async (tx) => tx.getCursor(key, runID));
    },
    /**
     * cursor_expired 原子替换（MX-012 R1 P2-1）：单事务内清空该 Run 的本地事件与
     * cursor，按快照事件重建并推进到末位 seq。要么整体成功，要么保持旧本地底。
     */
    async replaceRun(runID: string, events: readonly ExecutionEvent[]): Promise<number> {
      if (events.length === 0) return 0;
      let last = 0;
      await driver.transaction(async (tx) => {
        await tx.deleteRun(key, runID);
        for (const event of [...events].sort((a, b) => a.seq - b.seq)) {
          if (event.run_id !== runID) throw new Error('replaceRun event run_id mismatch');
          if (event.seq <= last) continue;
          const encrypted = await cipher.encrypt(event.payload, aad(key, runID, event.seq));
          await tx.insertEvent({ scopeKey: key, runID, seq: event.seq, event: { ...event, payload: encrypted } });
          last = event.seq;
        }
        await tx.setCursor(key, runID, last);
      });
      return last;
    },
    clear: () => driver.clearScope(key),
  };
}

export type ExecutionStorage = ReturnType<typeof createExecutionStorage>;
