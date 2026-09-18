// 原生平台装配（RW-007/RW-008 运行时）：expo-sqlite + expo-secure-store 实现。
// 命名空间：weknora_mobile_next_v1（与旧 App 缓存完全隔离，不读取旧数据）。
import * as SQLite from "expo-sqlite";
import * as SecureStore from "expo-secure-store";
import type { MobileStore, PendingSubmission, StoredEvent, StoredProjection } from "./store";
import { WriteQueue } from "./store";

const DB_NAME = "weknora_mobile_next_v1.db";
const CRED_KEY = "weknora_mobile_next_v1.creds";

export interface SecureCredentials {
  origin: string;
  access: string;
  refresh: string;
  userId: string;
  tenantId: string | null;
}

export const secureCreds = {
  async read(): Promise<SecureCredentials | null> {
    try {
      const raw = await SecureStore.getItemAsync(CRED_KEY);
      return raw ? (JSON.parse(raw) as SecureCredentials) : null;
    } catch {
      return null;
    }
  },
  async write(c: SecureCredentials): Promise<void> {
    await SecureStore.setItemAsync(CRED_KEY, JSON.stringify(c));
  },
  async clear(): Promise<void> {
    try {
      await SecureStore.deleteItemAsync(CRED_KEY);
    } catch {
      // 清理失败不阻塞登出流程；服务端会话失效兜底
    }
  },
};

const DDL = `
CREATE TABLE IF NOT EXISTS execution_events (
  scope_key TEXT NOT NULL, run_id TEXT NOT NULL, seq INTEGER NOT NULL,
  envelope TEXT NOT NULL, PRIMARY KEY(scope_key, run_id, seq)
);
CREATE TABLE IF NOT EXISTS execution_cursors (
  scope_key TEXT NOT NULL, run_id TEXT NOT NULL, seq INTEGER NOT NULL,
  PRIMARY KEY(scope_key, run_id)
);
CREATE TABLE IF NOT EXISTS execution_projections (
  scope_key TEXT NOT NULL, run_id TEXT NOT NULL, watermark INTEGER NOT NULL,
  projection TEXT NOT NULL, PRIMARY KEY(scope_key, run_id)
);
CREATE TABLE IF NOT EXISTS pending_submissions (
  scope_key TEXT NOT NULL, request_id TEXT NOT NULL, input_hash TEXT NOT NULL,
  input TEXT NOT NULL, state TEXT NOT NULL, run_id TEXT, created_at TEXT NOT NULL,
  PRIMARY KEY(scope_key, request_id)
);
CREATE TABLE IF NOT EXISTS drafts (
  scope_key TEXT NOT NULL, draft_id TEXT NOT NULL, text TEXT NOT NULL,
  PRIMARY KEY(scope_key, draft_id)
);
`;

export class SqliteStore implements MobileStore {
  private db: SQLite.SQLiteDatabase | null = null;
  private queue = new WriteQueue();

  async open(): Promise<void> {
    if (this.db) return;
    this.db = await SQLite.openDatabaseAsync(DB_NAME);
    await this.db.execAsync(DDL);
  }

  private require(): SQLite.SQLiteDatabase {
    if (!this.db) throw new Error("SqliteStore 未 open");
    return this.db;
  }

  async appendEvents(events: StoredEvent[]): Promise<void> {
    return this.queue.enqueue(async () => {
      const db = this.require();
      await db.withTransactionAsync(async () => {
        for (const e of events) {
          await db.runAsync(
            "INSERT OR IGNORE INTO execution_events (scope_key, run_id, seq, envelope) VALUES (?, ?, ?, ?)",
            [e.scopeKey, e.runId, e.seq, e.envelope],
          );
        }
      });
    });
  }

  async readEvents(scopeKey: string, runId: string, afterSeq: number, limit = 500): Promise<StoredEvent[]> {
    const rows = await this.require().getAllAsync<Record<string, unknown>>(
      "SELECT scope_key, run_id, seq, envelope FROM execution_events WHERE scope_key = ? AND run_id = ? AND seq > ? ORDER BY seq LIMIT ?",
      [scopeKey, runId, afterSeq, limit],
    );
    return rows.map((r) => ({
      scopeKey: String(r["scope_key"]),
      runId: String(r["run_id"]),
      seq: Number(r["seq"]),
      envelope: String(r["envelope"]),
    }));
  }

  async readCursor(scopeKey: string, runId: string): Promise<number> {
    const row = await this.require().getFirstAsync<Record<string, unknown>>(
      "SELECT seq FROM execution_cursors WHERE scope_key = ? AND run_id = ?",
      [scopeKey, runId],
    );
    return row ? Number(row["seq"]) : 0;
  }

  async commitAtomic(update: {
    events: StoredEvent[];
    projection: StoredProjection | null;
    cursor: { scopeKey: string; runId: string; seq: number } | null;
  }): Promise<void> {
    return this.queue.enqueue(async () => {
      const db = this.require();
      await db.withTransactionAsync(async () => {
        for (const e of update.events) {
          await db.runAsync(
            "INSERT OR IGNORE INTO execution_events (scope_key, run_id, seq, envelope) VALUES (?, ?, ?, ?)",
            [e.scopeKey, e.runId, e.seq, e.envelope],
          );
        }
        if (update.projection) {
          await db.runAsync(
            "INSERT OR REPLACE INTO execution_projections (scope_key, run_id, watermark, projection) VALUES (?, ?, ?, ?)",
            [update.projection.scopeKey, update.projection.runId, update.projection.watermark, update.projection.projection],
          );
        }
        if (update.cursor) {
          await db.runAsync(
            "INSERT OR REPLACE INTO execution_cursors (scope_key, run_id, seq) VALUES (?, ?, ?)",
            [update.cursor.scopeKey, update.cursor.runId, update.cursor.seq],
          );
        }
      });
    });
  }

  async readProjection(scopeKey: string, runId: string): Promise<StoredProjection | null> {
    const row = await this.require().getFirstAsync<Record<string, unknown>>(
      "SELECT watermark, projection FROM execution_projections WHERE scope_key = ? AND run_id = ?",
      [scopeKey, runId],
    );
    if (!row) return null;
    return { scopeKey, runId, watermark: Number(row["watermark"]), projection: String(row["projection"]) };
  }

  async saveProjection(p: StoredProjection): Promise<void> {
    return this.queue.enqueue(async () => {
      await this.require().runAsync(
        "INSERT OR REPLACE INTO execution_projections (scope_key, run_id, watermark, projection) VALUES (?, ?, ?, ?)",
        [p.scopeKey, p.runId, p.watermark, p.projection],
      );
    });
  }

  async savePending(p: PendingSubmission): Promise<void> {
    return this.queue.enqueue(async () => {
      const existing = await this.readPendingRaw(p.scopeKey, p.requestId);
      if (existing && existing.input_hash !== p.inputHash) {
        throw new Error("同 request_id 与不同输入冲突，拒绝覆盖");
      }
      await this.require().runAsync(
        "INSERT OR REPLACE INTO pending_submissions (scope_key, request_id, input_hash, input, state, run_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
        [p.scopeKey, p.requestId, p.inputHash, p.input, p.state, p.runId, p.createdAt],
      );
    });
  }

  private async readPendingRaw(scopeKey: string, requestId: string): Promise<Record<string, unknown> | null> {
    return this.require().getFirstAsync(
      "SELECT * FROM pending_submissions WHERE scope_key = ? AND request_id = ?",
      [scopeKey, requestId],
    );
  }

  async readPending(scopeKey: string, requestId: string): Promise<PendingSubmission | null> {
    const row = await this.readPendingRaw(scopeKey, requestId);
    if (!row) return null;
    return {
      scopeKey,
      requestId: String(row["request_id"]),
      inputHash: String(row["input_hash"]),
      input: String(row["input"]),
      state: String(row["state"]) as PendingSubmission["state"],
      runId: row["run_id"] == null ? null : String(row["run_id"]),
      createdAt: String(row["created_at"]),
    };
  }

  async listPending(scopeKey: string, states?: PendingSubmission["state"][]): Promise<PendingSubmission[]> {
    const rows = await this.require().getAllAsync<Record<string, unknown>>(
      states && states.length
        ? "SELECT * FROM pending_submissions WHERE scope_key = ? AND state IN (" + states.map(() => "?").join(",") + ")"
        : "SELECT * FROM pending_submissions WHERE scope_key = ?",
      states && states.length ? [scopeKey, ...states] : [scopeKey],
    );
    return rows.map((row) => ({
      scopeKey,
      requestId: String(row["request_id"]),
      inputHash: String(row["input_hash"]),
      input: String(row["input"]),
      state: String(row["state"]) as PendingSubmission["state"],
      runId: row["run_id"] == null ? null : String(row["run_id"]),
      createdAt: String(row["created_at"]),
    }));
  }

  async deletePending(scopeKey: string, requestId: string): Promise<void> {
    return this.queue.enqueue(async () => {
      await this.require().runAsync("DELETE FROM pending_submissions WHERE scope_key = ? AND request_id = ?", [
        scopeKey,
        requestId,
      ]);
    });
  }

  async saveDraft(scopeKey: string, draftId: string, text: string): Promise<void> {
    await this.require().runAsync("INSERT OR REPLACE INTO drafts (scope_key, draft_id, text) VALUES (?, ?, ?)", [
      scopeKey,
      draftId,
      text,
    ]);
  }

  async readDraft(scopeKey: string, draftId: string): Promise<string | null> {
    const row = await this.require().getFirstAsync<Record<string, unknown>>(
      "SELECT text FROM drafts WHERE scope_key = ? AND draft_id = ?",
      [scopeKey, draftId],
    );
    return row ? String(row["text"]) : null;
  }

  async clearScopeData(scopeKey: string): Promise<void> {
    return this.queue.enqueue(async () => {
      const db = this.require();
      await db.withTransactionAsync(async () => {
        await db.runAsync("DELETE FROM execution_events WHERE scope_key = ?", [scopeKey]);
        await db.runAsync("DELETE FROM execution_cursors WHERE scope_key = ?", [scopeKey]);
        await db.runAsync("DELETE FROM execution_projections WHERE scope_key = ?", [scopeKey]);
        await db.runAsync("DELETE FROM pending_submissions WHERE scope_key = ?", [scopeKey]);
        await db.runAsync("DELETE FROM drafts WHERE scope_key = ?", [scopeKey]);
      });
    });
  }
}
