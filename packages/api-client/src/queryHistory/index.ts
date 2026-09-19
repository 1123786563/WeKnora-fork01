import {
  parseActionSuccessResponse,
  parseQueryHistoryConfigResponse,
  parseQueryHistoryExportStartResponse,
  parseQueryHistoryExportStatusResponse,
  parseQueryHistorySessionListResponse,
  parseQueryHistoryShareTokenResponse,
  parseQueryHistorySnapshotResponse,
  parseSharedSessionResponse,
  type QueryHistoryFeedbackRating,
  type QueryHistoryMode,
} from '@weknora/contracts';
import type { ClientBinaryResponse, ClientRequest } from '../client.ts';

type Request = (input: ClientRequest) => Promise<unknown>;
type RequestBinary = (input: ClientRequest) => Promise<ClientBinaryResponse>;

/**
 * Filters of the Admin+ audit listing (GET /api/v1/sessions, source=all —
 * internal/handler/session/handler.go GetSessionsByTenant). The sessions
 * domain's own list params do not carry the audit-only filters
 * (user_id / time range / feedback), so this call addresses the endpoint
 * directly instead of piggybacking on chat/sessions.ts.
 * page is 0-based; start_time is inclusive, end_time exclusive
 * (RFC3339 or YYYY-MM-DD); feedback narrows to sessions carrying that rating.
 */
export interface QueryHistoryAdminListParams {
  page?: number;
  pageSize?: number;
  keyword?: string;
  source?: string;
  agentId?: string;
  userId?: string;
  startTime?: string;
  endTime?: string;
  feedback?: QueryHistoryFeedbackRating;
  signal?: AbortSignal;
}

/** Optional filters of POST /api/v1/admin/sessions/export; all-omitted body exports the whole tenant. */
export interface QueryHistoryExportInput {
  userId?: string;
  startTime?: string;
  endTime?: string;
  feedback?: QueryHistoryFeedbackRating;
  signal?: AbortSignal;
}

function sessionSegment(sessionId: string): string {
  if (typeof sessionId !== 'string' || sessionId.trim() === '') throw new Error('sessionId must not be empty');
  return encodeURIComponent(sessionId);
}

function jobSegment(jobId: string | number): string {
  if (typeof jobId === 'number') {
    if (!Number.isSafeInteger(jobId) || jobId < 1) throw new Error('jobId must be a positive integer');
    return String(jobId);
  }
  if (typeof jobId !== 'string' || jobId.trim() === '') throw new Error('jobId must not be empty');
  return encodeURIComponent(jobId);
}

function optionalFeedback(value: QueryHistoryFeedbackRating | undefined): QueryHistoryFeedbackRating | undefined {
  if (value === undefined) return undefined;
  if (value !== 'like' && value !== 'dislike') throw new Error('feedback must be like or dislike');
  return value;
}

function optionalMode(mode: QueryHistoryMode): QueryHistoryMode {
  if (mode !== 'normal' && mode !== 'anonymized' && mode !== 'disabled') {
    throw new Error('mode must be normal, anonymized, or disabled');
  }
  return mode;
}

export function createQueryHistoryApi(request: Request, requestBinary?: RequestBinary) {
  return {
    /**
     * GET /api/v1/sessions (Admin+ audit view, source=all): tenant-wide
     * session rows with the audit filters. Non-admin source values behave
     * exactly like the regular sessions listing.
     */
    async adminList(params: QueryHistoryAdminListParams = {}) {
      if (params.page !== undefined && (!Number.isSafeInteger(params.page) || params.page < 0)) {
        throw new Error('page must be a non-negative integer');
      }
      if (params.pageSize !== undefined && (!Number.isSafeInteger(params.pageSize) || params.pageSize < 1)) {
        throw new Error('pageSize must be a positive integer');
      }
      const feedback = optionalFeedback(params.feedback);
      const query = new URLSearchParams();
      if (params.page !== undefined) query.set('page', String(params.page));
      if (params.pageSize !== undefined) query.set('page_size', String(params.pageSize));
      if (params.keyword) query.set('keyword', params.keyword);
      if (params.source) query.set('source', params.source);
      if (params.agentId) query.set('agent_id', params.agentId);
      if (params.userId) query.set('user_id', params.userId);
      if (params.startTime !== undefined) query.set('start_time', params.startTime);
      if (params.endTime !== undefined) query.set('end_time', params.endTime);
      if (feedback !== undefined) query.set('feedback', feedback);
      const suffix = query.toString();
      return parseQueryHistorySessionListResponse(await request({
        method: 'GET',
        path: `/api/v1/sessions${suffix ? `?${suffix}` : ''}`,
        signal: params.signal,
      }));
    },
    /**
     * GET /api/v1/admin/sessions/:session_id/snapshot (Admin+): session row +
     * most recent 200 messages + every feedback row; 403 when the tenant
     * disabled query history, user ids masked when anonymized.
     */
    async snapshot(sessionId: string, signal?: AbortSignal) {
      return parseQueryHistorySnapshotResponse(await request({
        method: 'GET',
        path: `/api/v1/admin/sessions/${sessionSegment(sessionId)}/snapshot`,
        ...(signal === undefined ? {} : { signal }),
      }));
    },
    /** POST /api/v1/admin/sessions/export: enqueue the async CSV export, answer {job_id}. */
    async startExport(input: QueryHistoryExportInput = {}) {
      const feedback = optionalFeedback(input.feedback);
      const body: Record<string, unknown> = {};
      if (input.userId !== undefined) body.user_id = input.userId;
      if (input.startTime !== undefined) body.start_time = input.startTime;
      if (input.endTime !== undefined) body.end_time = input.endTime;
      if (feedback !== undefined) body.feedback = feedback;
      return parseQueryHistoryExportStartResponse(await request({
        method: 'POST',
        path: '/api/v1/admin/sessions/export',
        body,
        signal: input.signal,
      }));
    },
    /** GET /api/v1/admin/sessions/export/:job_id/status: pending/running/done/failed + error_message. */
    async exportStatus(jobId: string | number, signal?: AbortSignal) {
      return parseQueryHistoryExportStatusResponse(await request({
        method: 'GET',
        path: `/api/v1/admin/sessions/export/${jobSegment(jobId)}/status`,
        ...(signal === undefined ? {} : { signal }),
      }));
    },
    /**
     * GET /api/v1/admin/sessions/export/:job_id/download: the finished CSV
     * stream. Rides requestBinary (usage.exportCsv precedent) so auth/tenant
     * headers match every other call; only status=done jobs answer here.
     */
    async downloadExport(jobId: string | number, signal?: AbortSignal): Promise<ClientBinaryResponse> {
      if (!requestBinary) throw new Error('Binary transport is unavailable');
      return requestBinary({
        method: 'GET',
        path: `/api/v1/admin/sessions/export/${jobSegment(jobId)}/download`,
        ...(signal === undefined ? {} : { signal }),
      });
    },
    /** POST /api/v1/sessions/:session_id/share (owner or Admin+): mint/rotate the read-only share token. */
    async share(sessionId: string, signal?: AbortSignal) {
      return parseQueryHistoryShareTokenResponse(await request({
        method: 'POST',
        path: `/api/v1/sessions/${sessionSegment(sessionId)}/share`,
        ...(signal === undefined ? {} : { signal }),
      }));
    },
    /** DELETE /api/v1/sessions/:session_id/share: revoke the token; idempotent for unshared sessions. */
    async unshare(sessionId: string, signal?: AbortSignal) {
      return parseActionSuccessResponse(await request({
        method: 'DELETE',
        path: `/api/v1/sessions/${sessionSegment(sessionId)}/share`,
        ...(signal === undefined ? {} : { signal }),
      }));
    },
    /** GET /api/v1/shared/sessions/:token (Viewer+): read-only session + messages snapshot, no feedback. */
    async shared(token: string, signal?: AbortSignal) {
      if (typeof token !== 'string' || token.trim() === '') throw new Error('token must not be empty');
      return parseSharedSessionResponse(await request({
        method: 'GET',
        path: `/api/v1/shared/sessions/${encodeURIComponent(token)}`,
        ...(signal === undefined ? {} : { signal }),
      }));
    },
    /**
     * Tenant KV policy GET/PUT /api/v1/tenants/kv/query-history-config
     * ({mode}). The settings domain's kvApi helper is module-private to
     * settings/index.ts (untyped SettingsPayload), so this typed pair
     * addresses the same key directly.
     */
    queryHistoryConfig: {
      async get(signal?: AbortSignal) {
        return parseQueryHistoryConfigResponse(await request({
          method: 'GET',
          path: '/api/v1/tenants/kv/query-history-config',
          ...(signal === undefined ? {} : { signal }),
        }));
      },
      async update(mode: QueryHistoryMode, signal?: AbortSignal) {
        return parseQueryHistoryConfigResponse(await request({
          method: 'PUT',
          path: '/api/v1/tenants/kv/query-history-config',
          body: { mode: optionalMode(mode) },
          ...(signal === undefined ? {} : { signal }),
        }));
      },
    },
  };
}
export type QueryHistoryApi = ReturnType<typeof createQueryHistoryApi>;
