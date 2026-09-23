import type { ScopeLease } from '../runtime/types.ts';
import type { TaskLifecycleState, TaskTimelineEntry } from './task-timeline.ts';
import type { AttentionState } from './task-office.ts';

export const TASK_DETAIL_HISTORY_LIMIT = 200;

export type TaskConnectionState = 'syncing' | 'live' | 'interrupted' | 'drained';
export type TaskInterruptionReason = 'gap' | 'cursor-expired' | 'stream-error' | 'stream-ended-nonterminal' | 'persist-failed';

export interface TaskBackendEvent {
  runId: string;
  seq: number;
  type: string;
  occurredAt: string;
  payload: Record<string, unknown>;
}

export interface TaskBackendDetail {
  taskId: string;
  runId: string;
  title: string;
  attention: AttentionState;
  archivedAt?: string;
  execution: { runStatus: string; executionStatus: string; settlementStatus: string; revision: number; seq: number };
  watermark: number;
  incomplete: boolean;
  events: TaskBackendEvent[];
}

export interface TaskStreamControlFrame { code: string; message: string }

export interface TaskDetailBackendPort {
  detail(runId: string): Promise<TaskBackendDetail>;
  /** Resolves when the server ends the stream; rejects on transport error. Cursor trimming rejects with an Error whose code is 'TASK_STREAM_CURSOR_EXPIRED'. */
  stream(input: { runId: string; cursor: number; signal: AbortSignal; onEvent(event: TaskBackendEvent): void; onControl(frame: TaskStreamControlFrame): void }): Promise<void>;
}

export interface PersistedTaskProjection {
  taskId: string;
  runId: string;
  cursor: number;
  events: TaskBackendEvent[];
  savedAt: string;
}

export interface TaskProjectionStore {
  load(runId: string): Promise<PersistedTaskProjection | undefined>;
  save(projection: PersistedTaskProjection): Promise<void>;
}

export interface TaskDetailView {
  taskId: string;
  runId: string;
  title: string;
  lifecycle: TaskLifecycleState;
  runStatus: string;
  attention: AttentionState;
  executionStatus: string;
  settlementStatus: string;
  revision: number;
  cursor: number;
  incomplete: boolean;
  connection: TaskConnectionState;
  interruption?: { reason: TaskInterruptionReason; message?: string };
  timeline: TaskTimelineEntry[];
  duplicateSeqs: number[];
}

export interface TaskHandle {
  hydrate(): Promise<TaskDetailView>;
  view(): TaskDetailView | undefined;
  updates(listener: (view: TaskDetailView) => void): () => void;
  resync(): Promise<TaskDetailView>;
  close(reason?: string): void;
}

export interface TaskDetailPorts {
  backend: TaskDetailBackendPort;
  store: TaskProjectionStore;
  lease(): ScopeLease | undefined;
}
