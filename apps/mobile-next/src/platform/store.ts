// 持久化端口与内存实现（RW-007）。
// 运行时实现（expo-sqlite/expo-secure-store）在 native.ts 装配；本文件保持纯逻辑可测。
// 表结构对齐详细设计 §5.3（execution_events / execution_cursors / execution_projections / pending_submissions）。

export interface StoredEvent {
  scopeKey: string;
  runId: string;
  seq: number;
  envelope: string; // 完整事件 JSON
}

export interface StoredProjection {
  scopeKey: string;
  runId: string;
  watermark: number;
  projection: string;
}

export interface PendingSubmission {
  scopeKey: string;
  requestId: string;
  inputHash: string;
  input: string; // 规范输入 JSON
  state: "prepared" | "sending" | "uncertain" | "accepted" | "rejected";
  runId: string | null;
  createdAt: string;
}

export interface MobileStore {
  // 事件与投影（受控事务：事件+投影+cursor 同事务提交）
  appendEvents(events: StoredEvent[]): Promise<void>;
  readEvents(scopeKey: string, runId: string, afterSeq: number, limit?: number): Promise<StoredEvent[]>;
  readCursor(scopeKey: string, runId: string): Promise<number>;
  commitAtomic(update: {
    events: StoredEvent[];
    projection: StoredProjection | null;
    cursor: { scopeKey: string; runId: string; seq: number } | null;
  }): Promise<void>;
  readProjection(scopeKey: string, runId: string): Promise<StoredProjection | null>;
  saveProjection(p: StoredProjection): Promise<void>;

  // 未确认提交（request_id 先落盘）
  savePending(p: PendingSubmission): Promise<void>;
  readPending(scopeKey: string, requestId: string): Promise<PendingSubmission | null>;
  listPending(scopeKey: string, states?: PendingSubmission["state"][]): Promise<PendingSubmission[]>;
  deletePending(scopeKey: string, requestId: string): Promise<void>;

  // 草稿（按 scope 隔离；离线保存、恢复不自动补发）
  saveDraft(scopeKey: string, draftId: string, text: string): Promise<void>;
  readDraft(scopeKey: string, draftId: string): Promise<string | null>;
  clearScopeData(scopeKey: string): Promise<void>;
}

/** 串行写队列：所有写经同一队列，避免事务交错（详细设计 §5.4） */
export class WriteQueue {
  private tail: Promise<unknown> = Promise.resolve();
  enqueue<T>(op: () => Promise<T>): Promise<T> {
    const run = this.tail.then(op, op);
    this.tail = run.catch(() => {});
    return run;
  }
}

export class InMemoryStore implements MobileStore {
  private events: StoredEvent[] = [];
  private cursors = new Map<string, number>();
  private projections = new Map<string, StoredProjection>();
  private pendings = new Map<string, PendingSubmission>();
  private drafts = new Map<string, string>();
  readonly queue = new WriteQueue();

  private ck(scope: string, run: string) {
    return `${scope}\u0001${run}`;
  }

  async appendEvents(events: StoredEvent[]): Promise<void> {
    return this.queue.enqueue(async () => {
      for (const e of events) {
        if (this.events.some((x) => x.scopeKey === e.scopeKey && x.runId === e.runId && x.seq === e.seq)) continue; // 幂等
        this.events.push(e);
      }
      this.events.sort((a, b) => a.seq - b.seq);
    });
  }

  async readEvents(scopeKey: string, runId: string, afterSeq: number, limit = 500): Promise<StoredEvent[]> {
    return this.events
      .filter((e) => e.scopeKey === scopeKey && e.runId === runId && e.seq > afterSeq)
      .slice(0, limit);
  }

  async readCursor(scopeKey: string, runId: string): Promise<number> {
    return this.cursors.get(this.ck(scopeKey, runId)) ?? 0;
  }

  async commitAtomic(update: {
    events: StoredEvent[];
    projection: StoredProjection | null;
    cursor: { scopeKey: string; runId: string; seq: number } | null;
  }): Promise<void> {
    return this.queue.enqueue(async () => {
      await this.appendEvents(update.events);
      if (update.projection) this.projections.set(this.ck(update.projection.scopeKey, update.projection.runId), update.projection);
      if (update.cursor) this.cursors.set(this.ck(update.cursor.scopeKey, update.cursor.runId), update.cursor.seq);
    });
  }

  async readProjection(scopeKey: string, runId: string): Promise<StoredProjection | null> {
    return this.projections.get(this.ck(scopeKey, runId)) ?? null;
  }

  async saveProjection(p: StoredProjection): Promise<void> {
    return this.queue.enqueue(async () => {
      this.projections.set(this.ck(p.scopeKey, p.runId), p);
    });
  }

  async savePending(p: PendingSubmission): Promise<void> {
    return this.queue.enqueue(async () => {
      const key = `${p.scopeKey}\u0001${p.requestId}`;
      const existing = this.pendings.get(key);
      if (existing && existing.inputHash !== p.inputHash && existing.requestId === p.requestId) {
        throw new Error("同 request_id 与不同输入冲突，拒绝覆盖");
      }
      this.pendings.set(key, p);
    });
  }

  async readPending(scopeKey: string, requestId: string): Promise<PendingSubmission | null> {
    return this.pendings.get(`${scopeKey}\u0001${requestId}`) ?? null;
  }

  async listPending(scopeKey: string, states?: PendingSubmission["state"][]): Promise<PendingSubmission[]> {
    return [...this.pendings.values()].filter(
      (p) => p.scopeKey === scopeKey && (!states || states.includes(p.state)),
    );
  }

  async deletePending(scopeKey: string, requestId: string): Promise<void> {
    return this.queue.enqueue(async () => {
      this.pendings.delete(`${scopeKey}\u0001${requestId}`);
    });
  }

  async saveDraft(scopeKey: string, draftId: string, text: string): Promise<void> {
    this.drafts.set(`${scopeKey}\u0001${draftId}`, text);
  }

  async readDraft(scopeKey: string, draftId: string): Promise<string | null> {
    return this.drafts.get(`${scopeKey}\u0001${draftId}`) ?? null;
  }

  async clearScopeData(scopeKey: string): Promise<void> {
    return this.queue.enqueue(async () => {
      this.events = this.events.filter((e) => e.scopeKey !== scopeKey);
      for (const k of [...this.cursors.keys()]) if (k.startsWith(`${scopeKey}\u0001`)) this.cursors.delete(k);
      for (const k of [...this.projections.keys()]) if (k.startsWith(`${scopeKey}\u0001`)) this.projections.delete(k);
      for (const k of [...this.pendings.keys()]) if (k.startsWith(`${scopeKey}\u0001`)) this.pendings.delete(k);
      for (const k of [...this.drafts.keys()]) if (k.startsWith(`${scopeKey}\u0001`)) this.drafts.delete(k);
    });
  }
}
