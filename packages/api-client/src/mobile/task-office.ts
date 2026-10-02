import { parseExecutionEvent } from '@weknora/contracts';
import type { InteractionAction } from '@weknora/contracts';
import { ApiError } from '../errors.ts';
import { createServerSentEventParser } from '../chat/stream.ts';
import { createChatSessionsApi } from '../chat/sessions.ts';
import { createExecutionsApi, executionEventsRequest } from './executions.ts';
import { createInteractionsApi } from './interactions.ts';
import { createOverviewApi } from './overview.ts';
import { requireDeploymentOrigin } from './deployment-origin.ts';
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

/** 与 mobile-core InteractionBackendPort 的 InboxItem 逐字一致（结构可赋值由 apps/mobile typecheck 证明）。 */
export interface RemoteInboxItem {
  interactionId: string;
  runId: string;
  kind: 'tool_approval' | 'budget' | 'recovery';
  argsHash: string;
  expectedRevision: number;
  createdAt: string;
}
/** 与 mobile-core InteractionBackendPort 的 ResolvedDecisionRecord 逐字一致（action 为六值联合，非 string）。 */
export interface RemoteDecisionRecord {
  interactionId: string;
  runId: string;
  kind: 'tool_approval' | 'budget' | 'recovery';
  decisionId: string;
  action: InteractionAction;
  argsHash: string;
  expectedRevision: number;
}

/** Task 6 的 TaskCommandPort 结构（remote 以结构化类型满足，无需 import）。 */
export interface RemoteTaskCommandInput { runId: string; action: 'steer' | 'queue_next' | 'cancel'; text?: string; expectedRevision: number; intentId?: string }
export interface RemoteTaskCommandAck { runId: string; action: 'steer' | 'queue_next' | 'cancel'; nextRunId?: string }

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
  const interactionsApi = createInteractionsApi(request);

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
      // engine_type 不可省略：省略即 builtin，workbench admission 会以 409 agent runtime conflict 拒绝（T39 #69 D1）。
      const session = await sessionsApi.create({ title: input.title, engine_type: 'trpc' });
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
    async inbox(): Promise<RemoteInboxItem[]> {
      const rows = await interactionsApi.inbox(50);
      return rows.map((row) => ({
        interactionId: row.id,
        runId: row.run_id,
        kind: row.kind,
        argsHash: row.args_hash,
        expectedRevision: row.expected_revision,
        createdAt: row.created_at ?? '',
      }));
    },
    async decide(input: { item: RemoteInboxItem; decisionId: string; action: InteractionAction }): Promise<RemoteDecisionRecord> {
      try {
        const ack = await interactionsApi.decide({
          id: input.item.interactionId,
          decision_id: input.decisionId,
          kind: input.item.kind,
          action: input.action,
          args_hash: input.item.argsHash,
          expected_revision: input.item.expectedRevision,
        });
        // 2xx ack 的 decision_id 非空 ⇒ action 恒为矩阵内具体动作；空串只可能出现在 pending 行，
        // 此处窄化仅为类型收敛（F3），运行时行为不变。
        if (ack.action === '') throw new Error('decide ack must carry a concrete action');
        return {
          interactionId: ack.id,
          runId: ack.run_id,
          kind: ack.kind,
          decisionId: ack.decision_id,
          action: ack.action,
          argsHash: ack.args_hash,
          expectedRevision: ack.expected_revision,
        };
      } catch (error) {
        if (error instanceof ApiError) {
          const coded = (message: string): Error => {
            const translated = new Error(message);
            // 跨包契约码（沿 TASK_STREAM_CURSOR_EXPIRED 先例）：mobile-core 按 error.code 分类，不得改名。
            (translated as unknown as { code?: string }).code = message;
            throw translated;
          };
          if (error.status === 409) coded('INTERACTION_SUPERSEDED'); // B3-F43：400 是确定性客户端错误，透传原始 ApiError
          if (error.status === 502 && error.code === 'command_recovery_unknown') coded('INTERACTION_DELIVERY_UNKNOWN');
          if (error.status === 404 || error.status === 403 || error.status === 410) coded('INTERACTION_GONE');
        }
        throw error;
      }
    },
    async command(input: RemoteTaskCommandInput): Promise<RemoteTaskCommandAck> {
      const wire = input.action === 'cancel'
        ? { action: 'cancel' as const, expected_revision: input.expectedRevision }
        : { action: input.action, text: input.text ?? '', expected_revision: input.expectedRevision, ...(input.intentId === undefined ? {} : { external_pending_id: input.intentId }) };
      try {
        const ack = await executionsApi.command(input.runId, wire);
        return { runId: ack.run_id, action: ack.action, ...(ack.next_run_id === undefined ? {} : { nextRunId: ack.next_run_id }) };
      } catch (error) {
        // 跨包契约码（沿 TASK_STREAM_CURSOR_EXPIRED / INTERACTION_* 先例，不得改名）：
        // 409/404 是确定性冲突；其余（5xx、502 command_recovery_unknown、传输失败）投递结果未知。
        // 变量级 never 注解：coded 恒抛出，TS 据此接受 catch 块无显式收尾（TS2366；
        // 箭头函数返回注解形式不被 TS 6 识别，见 decide 处等价先例以 throw error 收尾）。
        const coded: (message: string) => never = (message) => {
          const translated = new Error(message);
          (translated as unknown as { code?: string }).code = message;
          throw translated;
        };
        if (error instanceof ApiError) {
          if (error.status === 409 || error.status === 404) coded('TASK_COMMAND_CONFLICT');
          coded('TASK_COMMAND_UNKNOWN');
        }
        coded('TASK_COMMAND_UNKNOWN');
      }
    },
  };
}
