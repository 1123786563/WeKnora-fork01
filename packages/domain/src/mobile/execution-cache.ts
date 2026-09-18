import type { ExecutionEvent } from '@weknora/contracts';

export interface ExecutionScope {
  origin: string;
  tenantID: string;
  userID: string;
}

export interface ExecutionProjection {
  scope: ExecutionScope;
  runID: string;
  events: ExecutionEvent[];
  cursor: number;
}

export type SaveEvent = (event: ExecutionEvent) => Promise<void>;
export type SetCursor = (seq: number) => Promise<void>;

/**
 * The ordering here is deliberate: a durable projection is the commit point.
 * A failed projection leaves the cursor unchanged so reconnect replays safely.
 */
export async function commitEvent(event: ExecutionEvent, save: SaveEvent, setCursor: SetCursor): Promise<void> {
  await save(event);
  await setCursor(event.seq);
}

function scopeKey(scope: ExecutionScope): string {
  return `${scope.origin}\u0000${scope.tenantID}\u0000${scope.userID}`;
}

function projectionKey(scope: ExecutionScope, runID: string): string {
  return `${scopeKey(scope)}\u0000${runID}`;
}

/** In-memory reference projection used by native adapters and deterministic tests. */
export class ExecutionCache {
  private readonly rows = new Map<string, ExecutionProjection>();

  constructor(private readonly scope: ExecutionScope) {}

  private row(runID: string): ExecutionProjection {
    const key = projectionKey(this.scope, runID);
    let row = this.rows.get(key);
    if (!row) {
      row = { scope: this.scope, runID, events: [], cursor: 0 };
      this.rows.set(key, row);
    }
    return row;
  }

  async commit(event: ExecutionEvent): Promise<void> {
    const row = this.row(event.run_id);
    if (event.seq <= row.cursor) return;
    if (event.seq !== row.cursor + 1) throw new Error(`execution cursor gap: expected ${row.cursor + 1}, got ${event.seq}`);
    row.events.push(event);
    row.cursor = event.seq;
  }

  read(runID: string): ExecutionProjection | undefined {
    const row = this.rows.get(projectionKey(this.scope, runID));
    return row && { ...row, events: [...row.events] };
  }

  /**
   * 快照水合（MX-012）：按 seq 升序幂等提交（重复 seq 跳过；缺口如快照不完整按 incomplete 语义
   * 由服务端 confirmed_watermark 把关——调用方仅在水合后从 watermark 续订）。
   */
  async hydrate(runID: string, events: readonly ExecutionEvent[]): Promise<number> {
    for (const event of [...events].sort((a, b) => a.seq - b.seq)) {
      const row = this.row(runID);
      if (event.seq <= row.cursor) continue;
      if (event.seq !== row.cursor + 1) throw new Error(`snapshot hydration gap at ${event.seq} (cursor ${row.cursor})`);
      row.events.push(event);
      row.cursor = event.seq;
    }
    return this.row(runID).cursor;
  }

  clear(): void { this.rows.clear(); }
}
