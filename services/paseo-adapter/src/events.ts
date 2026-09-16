import { createHash } from 'node:crypto';

export interface SourceEvent {
  bindingID: string;
  generation: string;
  eventID: string;
  attemptID: string;
  type: string;
  payload: Record<string, unknown>;
  sourceSeq?: number;
}

export interface ProductEvent extends SourceEvent {
  seq: number;
  type: string;
}

export interface ProductSnapshot {
  events: ProductEvent[];
  watermark: number;
  executionStatus: 'queued' | 'running' | 'waiting_user' | 'reconciling' | 'succeeded' | 'failed' | 'canceled';
  settlementStatus: 'pending' | 'settled';
  incomplete: boolean;
  confirmedWatermark: number;
}

export function sourceKey(event: SourceEvent): string {
  return JSON.stringify([event.bindingID, event.generation, event.eventID]);
}

function payloadHash(event: SourceEvent): string {
  return createHash('sha256').update(JSON.stringify(event.payload)).digest('hex');
}

function productType(type: string): string {
  if (/^(text|thought|tool|run|execution|approval|usage|error|artifact|status)\./.test(type)) return type;
  return 'unknown';
}

export function deduplicateSourceEvents(events: readonly SourceEvent[]): ProductEvent[] {
  const seen = new Map<string, { hash: string; event: SourceEvent }>();
  const result: ProductEvent[] = [];
  for (const source of events) {
    const key = sourceKey(source);
    const hash = payloadHash(source);
    const prior = seen.get(key);
    if (prior) {
      if (prior.hash !== hash) throw new Error(`source event conflict: ${key}`);
      continue;
    }
    seen.set(key, { hash, event: source });
    const payload = productType(source.type) === 'unknown' ? { source_type: source.type, ...source.payload } : source.payload;
    result.push({ ...source, type: productType(source.type), payload, seq: result.length + 1 });
  }
  return result;
}

export function projectSnapshot(events: readonly ProductEvent[]): ProductSnapshot {
  const sorted = [...events].sort((a, b) => a.seq - b.seq);
  const terminal = sorted.some((event) => event.type === 'run.completed' || event.type === 'execution.succeeded' || event.type === 'status.succeeded');
  const failed = sorted.some((event) => event.type === 'run.failed' || event.type === 'execution.failed' || event.type === 'status.failed');
  const canceled = sorted.some((event) => event.type === 'run.canceled' || event.type === 'execution.canceled' || event.type === 'status.canceled');
  return {
    events: sorted,
    watermark: sorted.reduce((max, event) => Math.max(max, event.seq), 0),
    executionStatus: canceled ? 'canceled' : failed ? 'failed' : terminal ? 'succeeded' : sorted.length ? 'running' : 'queued',
    settlementStatus: terminal || failed || canceled ? 'settled' : 'pending',
    incomplete: sorted.length > 0 && sorted[0].seq > 1,
    confirmedWatermark: sorted.length > 0 && sorted[0].seq > 1 ? sorted[0].seq - 1 : sorted.reduce((max, event) => Math.max(max, event.seq), 0),
  };
}
