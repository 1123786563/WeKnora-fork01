import { parseExecutionEvent } from '@weknora/contracts';
import { createServerSentEventParser } from '../chat/stream.ts';
import { createChatSessionsApi } from '../chat/sessions.ts';
import { createExecutionsApi, executionEventsRequest } from './executions.ts';
import { createOverviewApi } from './overview.ts';
import type { ClientRequest } from '../client.ts';
import type { RequestLookup, StartAck, StartExecutionInput } from './executions.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface TaskOfficeRemoteOptions {
  /** 部署 Origin：构造即强校验（绝对 HTTPS、无 path/query/fragment、无内嵌凭据）。 */
  origin: string;
  /** 授权通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输。 */
  request: Request;
  /** T05 authorized SSE channel（MobileRuntime.authorizedEventStream 或测试替身）；本适配器不新建传输。 */
  stream?: (input: ClientRequest, onChunk: (chunk: string) => void) => Promise<void>;
}

/** 与 mobile-core TaskBackendRun 逐字一致（结构可赋值由 apps/mobile typecheck 证明）。 */
export interface RemoteTaskRun { runId: string; taskId: string; title: string; runStatus: string; attention: 'none' | 'required'; updatedAt: string }
export interface RemoteTaskOverview {
  needsMe: Array<{ interactionId: string; kind: string; createdAt: string }>;
  running: RemoteTaskRun[];
  recentlyCompleted: RemoteTaskRun[];
  unreadNotifications: number;
  asOf: string;
}
export interface RemoteTaskPage { items: RemoteTaskRun[]; nextCursor?: string }
export interface RemoteTaskStreamEvent { runId: string; seq: number; type: string; occurredAt: string; payload: Record<string, unknown> }
export interface RemoteTaskDetail {
  taskId: string; runId: string; title: string; attention: 'none' | 'required'; archivedAt?: string;
  execution: { runStatus: string; executionStatus: string; settlementStatus: string; revision: number; seq: number };
  watermark: number; incomplete: boolean; events: RemoteTaskStreamEvent[];
}
export type TaskOfficeStreamOption = NonNullable<TaskOfficeRemoteOptions['stream']>;

function requireDeploymentOrigin(origin: string): string {
  let parsed: URL;
  if (typeof origin !== 'string' || origin.trim() === '') throw new Error('deployment origin is required');
  try { parsed = new URL(origin); } catch { throw new Error(`deployment origin must be an absolute URL: ${origin}`); }
  if (parsed.protocol !== 'https:') throw new Error('deployment origin must use HTTPS');
  if (parsed.username !== '' || parsed.password !== '') throw new Error('deployment origin must not embed user info');
  if (parsed.pathname !== '/') throw new Error('deployment origin must not include a path');
  if (parsed.search !== '' || parsed.hash !== '') throw new Error('deployment origin must not include a query or fragment');
  return parsed.origin;
}

function unwrap(value: unknown): void {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error('task office response must be a success envelope');
  const envelope = value as { success?: unknown; data?: unknown };
  if (envelope.success !== true || !Object.prototype.hasOwnProperty.call(envelope, 'data')) {
    throw new Error('task office response.success must be true with data');
  }
}

export function createTaskOfficeRemote(options: TaskOfficeRemoteOptions) {
  requireDeploymentOrigin(options.origin);
  const request = options.request;
  const overviewApi = createOverviewApi(request);
  const executionsApi = createExecutionsApi(request);
  const sessionsApi = createChatSessionsApi(request);

  const runFromSummary = (summary: { run_id: string; session_id: string; title?: string; run_status: string; attention?: 'none' | 'required'; updated_at: string }): RemoteTaskRun => ({
    runId: summary.run_id,
    taskId: summary.session_id,
    title: summary.title ?? '',
    runStatus: summary.run_status,
    attention: summary.attention ?? 'none',
    updatedAt: summary.updated_at,
  });
  const runFromItem = (item: { run_id: string; session_id: string; title?: string; status: string; attention?: 'none' | 'required'; updated_at: string }): RemoteTaskRun => ({
    runId: item.run_id,
    taskId: item.session_id,
    title: item.title ?? '',
    runStatus: item.status,
    attention: item.attention ?? 'none',
    updatedAt: item.updated_at,
  });

  return {
    async overview(): Promise<RemoteTaskOverview> {
      const value = await overviewApi.overview();
      return {
        needsMe: value.pending_interactions.map((item) => ({ interactionId: item.id, kind: item.kind, createdAt: item.created_at })),
        running: value.in_progress.map(runFromSummary),
        recentlyCompleted: value.recently_completed.map(runFromSummary),
        unreadNotifications: value.counts.unread_notifications,
        asOf: value.as_of,
      };
    },
    async list(input: { status?: string; agentId?: string; search?: string; archived?: boolean; cursor?: string; limit?: number }): Promise<RemoteTaskPage> {
      const page = await executionsApi.list({
        ...(input.status === undefined ? {} : { status: input.status }),
        ...(input.agentId === undefined ? {} : { agent_id: input.agentId }),
        ...(input.search === undefined ? {} : { search: input.search }),
        ...(input.archived === undefined ? {} : { archived: input.archived }),
        ...(input.cursor === undefined ? {} : { cursor: input.cursor }),
        ...(input.limit === undefined ? {} : { limit: input.limit }),
      });
      return {
        items: page.items.map(runFromItem),
        ...(page.next_cursor === undefined ? {} : { nextCursor: page.next_cursor }),
      };
    },
    async createSession(input: { title: string }): Promise<{ sessionId: string }> {
      const session = await sessionsApi.create({ title: input.title });
      return { sessionId: session.id };
    },
    async start(input: StartExecutionInput): Promise<StartAck> {
      return executionsApi.start(input);
    },
    async lookup(requestId: string): Promise<RequestLookup> {
      return executionsApi.lookup(requestId);
    },
    async archive(taskId: string): Promise<void> {
      unwrap(await request({ method: 'POST', path: `/api/v1/workbench/tasks/${encodeURIComponent(taskId)}/archive` }));
    },
    async restore(taskId: string): Promise<void> {
      unwrap(await request({ method: 'DELETE', path: `/api/v1/workbench/tasks/${encodeURIComponent(taskId)}/archive` }));
    },
    async detail(runId: string): Promise<RemoteTaskDetail> {
      const snapshot = await executionsApi.snapshot(runId);
      const task = snapshot.task;
      return {
        taskId: snapshot.execution.session_id,
        runId: snapshot.execution.run_id,
        title: task?.title ?? '',
        ...(task?.archived_at === undefined ? {} : { archivedAt: task.archived_at }),
        attention: task?.attention ?? 'none',
        execution: {
          runStatus: snapshot.execution.run_status,
          executionStatus: snapshot.execution.execution_status,
          settlementStatus: snapshot.execution.settlement_status,
          revision: snapshot.execution.revision,
          seq: snapshot.execution.seq,
        },
        watermark: snapshot.watermark,
        incomplete: snapshot.incomplete,
        events: snapshot.events.map((event) => ({ runId: event.run_id, seq: event.seq, type: event.type, occurredAt: event.occurred_at, payload: event.payload })),
      };
    },
    async stream(input: { runId: string; cursor: number; signal: AbortSignal; onEvent(event: RemoteTaskStreamEvent): void; onControl(frame: { code: string; message: string }): void }): Promise<void> {
      const transport = options.stream;
      if (!transport) throw new Error('task office stream transport is required for event streams');
      const request = executionEventsRequest(input.runId, String(input.cursor));
      const parser = createServerSentEventParser((frame) => {
        // 畸形帧守卫（B2-F8/F30 顺手）：JSON.parse 对破损 data 抛裸 SyntaxError 会击穿读取循环，
        // 统一转成带 TASK_STREAM_MALFORMED_FRAME 消息的 transport error（进入 streamFailed → interrupted）。
        // 只包 JSON.parse：消费方回调（如 runtime guardedChunk 的 RUNTIME_SCOPE_CHANGED）照常向上传播。
        let payload: unknown;
        try {
          payload = JSON.parse(frame.data);
        } catch {
          throw new Error(`TASK_STREAM_MALFORMED_FRAME: ${frame.event ?? 'message'}`);
        }
        if (frame.event === 'control') {
          const control = payload as { code?: string; message?: string };
          input.onControl({ code: control.code ?? 'stream_error', message: control.message ?? '' });
          return;
        }
        const event = parseExecutionEvent(payload);
        input.onEvent({ runId: event.run_id, seq: event.seq, type: event.type, occurredAt: event.occurred_at, payload: event.payload });
      });
      await transport({ method: request.method, path: request.path, headers: request.headers, signal: input.signal }, (chunk) => parser.push(chunk)).catch((error: unknown) => {
        const shape = error as { name?: unknown; status?: unknown } | null;
        if (typeof shape === 'object' && shape !== null && shape.name === 'ApiError' && shape.status === 409) {
          // 'TASK_STREAM_CURSOR_EXPIRED' 是 api-client → mobile-core 的跨包契约码
          // （mobile-core task-detail.ts 的 streamFailed 按 error.code 识别），不得改名或改用 ApiError 形态。
          const expired = new Error('workbench event stream cursor expired');
          (expired as unknown as { code?: string }).code = 'TASK_STREAM_CURSOR_EXPIRED';
          throw expired;
        }
        throw error;
      });
      parser.finish();
    },
  };
}
