import type { ExecutionDTO, ExecutionEvent, ExecutionSnapshot } from '@weknora/contracts';
export interface RunProjection { execution: ExecutionDTO; cursor: number; events: readonly ExecutionEvent[] }
const HISTORY_LIMIT = 200;
function validateEvent(event: ExecutionEvent, runId: string): void {
  if (event.schema_version !== 1) throw new Error('Unsupported event schema');
  if (event.run_id !== runId) throw new Error('Event belongs to another run');
  if (!Number.isSafeInteger(event.seq) || event.seq < 1) throw new Error('Invalid event sequence');
}
export function installSnapshot(snapshot: ExecutionSnapshot, runId: string): RunProjection {
  if (snapshot.execution.run_id !== runId) throw new Error('Snapshot belongs to another run');
  if (snapshot.execution.schema_version !== 1 || !Number.isSafeInteger(snapshot.watermark) || snapshot.watermark < snapshot.execution.seq) throw new Error('Invalid snapshot watermark');
  const unique = new Map<number, ExecutionEvent>();
  for (const event of snapshot.events) { validateEvent(event, runId); if (event.seq > snapshot.watermark) throw new Error('Event is ahead of snapshot watermark'); unique.set(event.seq, event); }
  return { execution: snapshot.execution, cursor: snapshot.watermark, events: [...unique.values()].sort((a,b) => a.seq - b.seq).slice(-HISTORY_LIMIT) };
}
export function appendEvent(projection: RunProjection, event: ExecutionEvent): RunProjection {
  validateEvent(event, projection.execution.run_id);
  if (event.seq <= projection.cursor) return projection;
  if (event.seq !== projection.cursor + 1) throw new Error('Event sequence gap; snapshot required');
  return { ...projection, cursor: event.seq, events: [...projection.events, event].slice(-HISTORY_LIMIT) };
}
export function persistEvent(projection: RunProjection, event: ExecutionEvent, persist: (next: RunProjection) => void): RunProjection {
  const next = appendEvent(projection, event); if (next === projection) return projection;
  persist(next); // A failed atomic write must not advance the committed cursor.
  return next;
}
export function terminal(status: ExecutionDTO['run_status']): boolean { return ['succeeded','failed','canceled'].includes(status); }
/** Never render arbitrary event payloads, prompts or tool arguments. */
export function eventLabel(event: ExecutionEvent): string {
  const labels: Record<string,string> = { 'run.started':'任务已开始', 'run.completed':'任务已完成', 'run.failed':'任务未能完成', 'tool.started':'正在使用工具', 'tool.completed':'工具处理完成', 'interaction.required':'需要你的确认', 'artifact.created':'已生成产物' };
  return labels[event.type] ?? '执行状态已更新';
}
