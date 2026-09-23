import { createExecutionsApi } from './executions.ts';
import { createOverviewApi } from './overview.ts';
import type { ClientRequest } from '../client.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface TaskOfficeRemoteOptions {
  /** 部署 Origin：构造即强校验（绝对 HTTPS、无 path/query/fragment、无内嵌凭据）。 */
  origin: string;
  /** 授权通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输。 */
  request: Request;
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
    async archive(taskId: string): Promise<void> {
      unwrap(await request({ method: 'POST', path: `/api/v1/workbench/tasks/${encodeURIComponent(taskId)}/archive` }));
    },
    async restore(taskId: string): Promise<void> {
      unwrap(await request({ method: 'DELETE', path: `/api/v1/workbench/tasks/${encodeURIComponent(taskId)}/archive` }));
    },
  };
}
