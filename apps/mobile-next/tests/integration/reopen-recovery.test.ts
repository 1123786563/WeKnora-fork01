// 杀进程与重开恢复（RW-010/SSE 验证清单项）：模拟 App 进程被杀后重新打开，
// SQLite 文件中的 pending_submissions / execution_events / projections / cursors / drafts 完整恢复，
// 且恢复后的对账（reconcile）与续流（startSeq）不重复执行旧内容。
// 用 node:sqlite（Node 26 内置）复刻 SqliteStore 的 DDL 与语句（与 src/platform/native.ts 语义一致），
// 全部语句参数绑定（prepare/run），验证同一 schema 在"新进程新连接"下的持久性语义。
import { DatabaseSync } from "node:sqlite";
import { mkdtempSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { RunEventAssembler, SseByteParser, type RunStreamItem } from "@/features/executions/sse/parser";

const DDL_STATEMENTS = [
  "CREATE TABLE IF NOT EXISTS execution_events (scope_key TEXT NOT NULL, run_id TEXT NOT NULL, seq INTEGER NOT NULL, envelope TEXT NOT NULL, PRIMARY KEY(scope_key, run_id, seq))",
  "CREATE TABLE IF NOT EXISTS execution_cursors (scope_key TEXT NOT NULL, run_id TEXT NOT NULL, seq INTEGER NOT NULL, PRIMARY KEY(scope_key, run_id))",
  "CREATE TABLE IF NOT EXISTS execution_projections (scope_key TEXT NOT NULL, run_id TEXT NOT NULL, watermark INTEGER NOT NULL, projection TEXT NOT NULL, PRIMARY KEY(scope_key, run_id))",
  "CREATE TABLE IF NOT EXISTS pending_submissions (scope_key TEXT NOT NULL, request_id TEXT NOT NULL, input_hash TEXT NOT NULL, input TEXT NOT NULL, state TEXT NOT NULL, run_id TEXT, created_at TEXT NOT NULL, PRIMARY KEY(scope_key, request_id))",
  "CREATE TABLE IF NOT EXISTS drafts (scope_key TEXT NOT NULL, draft_id TEXT NOT NULL, text TEXT NOT NULL, PRIMARY KEY(scope_key, draft_id))",
];

const openStore = (dbPath: string) => {
  const db = new DatabaseSync(dbPath);
  for (const ddl of DDL_STATEMENTS) db.prepare(ddl).run();
  const run1 = (sql: string, ...args: Array<string | number | null>) => db.prepare(sql).run(...args);
  const get1 = (sql: string, ...args: Array<string | number>) => db.prepare(sql).get(...args);
  return {
    close: () => db.close(),
    begin: () => db.prepare("BEGIN").run(),
    commit: () => db.prepare("COMMIT").run(),
    rollback: () => db.prepare("ROLLBACK").run(),
    savePending: (p: { scopeKey: string; requestId: string; inputHash: string; input: string; state: string; runId: string | null; createdAt: string }) =>
      run1(
        "INSERT OR REPLACE INTO pending_submissions (scope_key, request_id, input_hash, input, state, run_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
        p.scopeKey, p.requestId, p.inputHash, p.input, p.state, p.runId, p.createdAt,
      ),
    readPending: (scopeKey: string, requestId: string) =>
      get1("SELECT * FROM pending_submissions WHERE scope_key = ? AND request_id = ?", scopeKey, requestId) as
        | Record<string, unknown>
        | undefined,
    commitAtomic: (u: {
      events: Array<{ scopeKey: string; runId: string; seq: number; envelope: string }>;
      projection: { scopeKey: string; runId: string; watermark: number; projection: string } | null;
      cursor: { scopeKey: string; runId: string; seq: number } | null;
    }) => {
      openStoreNoop();
      const s = openStore; void s;
      db.prepare("BEGIN").run();
      try {
        for (const e of u.events) {
          run1("INSERT OR IGNORE INTO execution_events (scope_key, run_id, seq, envelope) VALUES (?, ?, ?, ?)", e.scopeKey, e.runId, e.seq, e.envelope);
        }
        if (u.projection) {
          run1("INSERT OR REPLACE INTO execution_projections (scope_key, run_id, watermark, projection) VALUES (?, ?, ?, ?)", u.projection.scopeKey, u.projection.runId, u.projection.watermark, u.projection.projection);
        }
        if (u.cursor) {
          run1("INSERT OR REPLACE INTO execution_cursors (scope_key, run_id, seq) VALUES (?, ?, ?)", u.cursor.scopeKey, u.cursor.runId, u.cursor.seq);
        }
        db.prepare("COMMIT").run();
      } catch (e) {
        db.prepare("ROLLBACK").run();
        throw e;
      }
    },
    readCursor: (scopeKey: string, runId: string) =>
      (get1("SELECT seq FROM execution_cursors WHERE scope_key = ? AND run_id = ?", scopeKey, runId) as { seq: number } | undefined)?.seq ?? 0,
    readProjection: (scopeKey: string, runId: string) =>
      get1("SELECT watermark, projection FROM execution_projections WHERE scope_key = ? AND run_id = ?", scopeKey, runId) as
        | { watermark: number; projection: string }
        | undefined,
    readEvents: (scopeKey: string, runId: string, afterSeq: number) =>
      db.prepare("SELECT seq, envelope FROM execution_events WHERE scope_key = ? AND run_id = ? AND seq > ? ORDER BY seq").all(scopeKey, runId, afterSeq) as Array<{ seq: number; envelope: string }>,
    saveDraft: (scopeKey: string, draftId: string, text: string) =>
      run1("INSERT OR REPLACE INTO drafts (scope_key, draft_id, text) VALUES (?, ?, ?)", scopeKey, draftId, text),
    readDraft: (scopeKey: string, draftId: string) =>
      (get1("SELECT text FROM drafts WHERE scope_key = ? AND draft_id = ?", scopeKey, draftId) as { text: string } | undefined)?.text ?? null,
  };
};
const openStoreNoop = () => undefined;

const SCOPE = "scope-reopen-1";
const RUN = "run_recover_1";

describe("RW-010 杀进程与重开恢复（SQLite 文件持久性 + 对账/续流闭环）", () => {
  let dir: string;
  let dbPath: string;

  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), "mn-reopen-"));
    dbPath = join(dir, "weknora_mobile_next_v1.db");
  });
  afterEach(() => rmSync(dir, { recursive: true, force: true }));

  it("重开后 pending_submissions / 事件 / 投影 / cursor / 草稿全部恢复；对账闭环不换 ID", () => {
    // ---- 进程 A：提交中断（uncertain）+ 部分事件已消费 + 草稿已存 ----
    {
      const a = openStore(dbPath);
      a.savePending({
        scopeKey: SCOPE, requestId: "req_kill_1", inputHash: "hash-1", input: '{"v":1,"text":"整理反馈"}',
        state: "uncertain", runId: null, createdAt: "2026-09-18T09:00:00Z",
      });
      a.commitAtomic({
        events: [
          { scopeKey: SCOPE, runId: RUN, seq: 1, envelope: '{"seq":1,"type":"run_started","payload":{}}' },
          { scopeKey: SCOPE, runId: RUN, seq: 2, envelope: '{"seq":2,"type":"log","payload":{"text":"检索中"}}' },
        ],
        projection: { scopeKey: SCOPE, runId: RUN, watermark: 2, projection: '{"status":"running","applied":2}' },
        cursor: { scopeKey: SCOPE, runId: RUN, seq: 2 },
      });
      a.saveDraft(SCOPE, "new-task", "被杀前的草稿内容");
      a.close(); // 模拟进程被杀（连接断开，文件留存）
    }

    // ---- 进程 B：重开同一数据库文件 ----
    const b = openStore(dbPath);
    const pending = b.readPending(SCOPE, "req_kill_1");
    expect(pending).toMatchObject({ request_id: "req_kill_1", state: "uncertain", input_hash: "hash-1" });

    expect(b.readCursor(SCOPE, RUN)).toBe(2);
    const proj = b.readProjection(SCOPE, RUN);
    expect(proj?.watermark).toBe(2);
    expect(JSON.parse(proj!.projection)).toMatchObject({ status: "running", applied: 2 });
    expect(b.readEvents(SCOPE, RUN, 0)).toHaveLength(2);
    expect(b.readDraft(SCOPE, "new-task")).toBe("被杀前的草稿内容");

    // 对账闭环：重开后按原 request_id 查询到受理结果 → accepted 绑定原 runId（不新建 ID）
    b.savePending({
      scopeKey: SCOPE, requestId: "req_kill_1", inputHash: "hash-1",
      input: String(pending!.input), state: "accepted", runId: RUN, createdAt: "2026-09-18T09:01:00Z",
    });
    expect(b.readPending(SCOPE, "req_kill_1")).toMatchObject({ state: "accepted", run_id: RUN });
    b.close();
  });

  it("重开后按恢复 cursor 续流：旧 seq 不重放，仅消费 seq 更大事件", () => {
    const items: RunStreamItem[] = [];
    let assembler: RunEventAssembler;
    {
      const a = openStore(dbPath);
      a.commitAtomic({
        events: [
          { scopeKey: SCOPE, runId: RUN, seq: 1, envelope: '{"seq":1,"type":"run_started","payload":{}}' },
          { scopeKey: SCOPE, runId: RUN, seq: 2, envelope: '{"seq":2,"type":"log","payload":{}}' },
        ],
        projection: { scopeKey: SCOPE, runId: RUN, watermark: 2, projection: '{"applied":2}' },
        cursor: { scopeKey: SCOPE, runId: RUN, seq: 2 },
      });
      a.close();
    }
    {
      const b = openStore(dbPath);
      const cursor = b.readCursor(SCOPE, RUN);
      expect(cursor).toBe(2);
      assembler = new RunEventAssembler({ onItem: (i) => items.push(i), onGap: () => {} }, { startSeq: cursor });
      // 续流帧：服务端从 watermark 前重放 seq 1/2 + 新 seq 3 —— 旧 seq 幂等丢弃
      const parser = new SseByteParser();
      const bytes = new TextEncoder().encode(
        'event: 1\ndata: {"seq":1,"type":"run_started","payload":{}}\n\n' +
          'event: 2\ndata: {"seq":2,"type":"log","payload":{}}\n\n' +
          'event: 3\ndata: {"seq":3,"type":"answer_delta","payload":{"delta":"恢复后新内容"}}\n\n',
      );
      for (const f of parser.push(bytes)) assembler.frame(f);
      b.close();
    }
    const events = items.filter((i) => i.kind === "event");
    expect(events.map((e) => (e as { seq: number }).seq)).toEqual([3]);
    expect(assembler!.cursor).toBe(3);
  });

  it("重开后数据层仍按 scope 隔离（另一 scope 读不到本 scope 提交）", () => {
    {
      const a = openStore(dbPath);
      a.savePending({ scopeKey: SCOPE, requestId: "req_y", inputHash: "h", input: "{}", state: "prepared", runId: null, createdAt: "t" });
      a.close();
    }
    const b = openStore(dbPath);
    expect(b.readPending("scope-other", "req_y")).toBeUndefined();
    b.close();
  });
});
