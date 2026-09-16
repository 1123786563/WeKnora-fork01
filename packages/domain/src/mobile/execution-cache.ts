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

  clear(): void { this.rows.clear(); }
}
