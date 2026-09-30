import type { PersistedTaskProjection, TaskBackendDetail, TaskBackendEvent, TaskDetailBackendPort, TaskProjectionStore, TaskStreamControlFrame } from './task-detail.ts';

/** 进程内持久化投影（module-seams §5.4 Task Store Port 的 in-memory Adapter；生产为 Scoped Vault Adapter）。 */
export function createInMemoryTaskProjectionStore(): TaskProjectionStore & { snapshot(): PersistedTaskProjection[] } {
  const rows = new Map<string, PersistedTaskProjection>();
  return {
    snapshot: () => [...rows.values()],
    async load(runId) { return rows.get(runId); },
    async save(projection) { rows.set(projection.runId, projection); },
  };
}

interface ScriptedTaskStreamSink {
  signal: AbortSignal;
  onEvent(event: TaskBackendEvent): void;
  onControl(frame: TaskStreamControlFrame): void;
  resolve(): void;
  reject(error: unknown): void;
}

export interface ScriptedTaskStream {
  opened: Array<{ runId: string; cursor: number }>;
  emit(event: TaskBackendEvent): void;
  control(frame: TaskStreamControlFrame): void;
  end(): void;
  fail(error: unknown): void;
  attach(sink: ScriptedTaskStreamSink): void;
}

/** 供测试驱动的单连接 SSE 脚本（module-seams §12 in-memory scenario Adapter）。 */
export function createScriptedTaskStream(): ScriptedTaskStream {
  const opened: Array<{ runId: string; cursor: number }> = [];
  let sink: ScriptedTaskStreamSink | undefined;
  return {
    opened,
    emit(event) { if (sink !== undefined && !sink.signal.aborted) sink.onEvent(event); },
    control(frame) { if (sink !== undefined && !sink.signal.aborted) sink.onControl(frame); },
    end() { if (sink !== undefined) { const active = sink; sink = undefined; active.resolve(); } },
    fail(error) { if (sink !== undefined) { const active = sink; sink = undefined; active.reject(error); } },
    attach(next) {
      sink = next;
      next.signal.addEventListener('abort', () => { if (sink === next) { sink = undefined; next.resolve(); } });
    },
  };
}

export interface ScenarioTaskDetailHandlers {
  detail?: (runId: string) => Promise<TaskBackendDetail>;
  /** 每次模块开流调用一次；缺省返回新空脚本（挂起直到测试驱动）。 */
  stream?: (input: { runId: string; cursor: number }) => ScriptedTaskStream | undefined;
}

export function createScenarioTaskDetailBackend(handlers: ScenarioTaskDetailHandlers = {}): TaskDetailBackendPort & { detailCalls: string[]; streams: ScriptedTaskStream[] } {
  const detailCalls: string[] = [];
  const streams: ScriptedTaskStream[] = [];
  return {
    detailCalls,
    streams,
    async detail(runId) {
      detailCalls.push(runId);
      if (handlers.detail === undefined) throw new Error(`scenario detail not scripted for ${runId}`);
      return handlers.detail(runId);
    },
    async stream({ runId, cursor, signal, onEvent, onControl }) {
      const scripted = handlers.stream?.({ runId, cursor }) ?? createScriptedTaskStream();
      streams.push(scripted);
      scripted.opened.push({ runId, cursor });
      return new Promise<void>((resolve, reject) => {
        scripted.attach({ signal, onEvent, onControl, resolve, reject });
      });
    },
  };
}
