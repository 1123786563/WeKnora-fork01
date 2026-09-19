import { parseMyUsageResponse, parseUsageByUserResponse } from '@weknora/contracts';
import type { ClientBinaryResponse, ClientRequest } from '../client.ts';

type Request = (input: ClientRequest) => Promise<unknown>;
type RequestBinary = (input: ClientRequest) => Promise<ClientBinaryResponse>;

export interface UsageRangeParams { startTime?: string; endTime?: string; signal?: AbortSignal; }

/** page is 0-based (backend default 0); page_size defaults server-side to 50 (clamped 1..200). */
export interface UsageByUserParams extends UsageRangeParams { page?: number; pageSize?: number; }

function rangeQuery(params: UsageRangeParams): URLSearchParams {
  const query = new URLSearchParams();
  if (params.startTime !== undefined) query.set('start_time', params.startTime);
  if (params.endTime !== undefined) query.set('end_time', params.endTime);
  return query;
}

export function createUsageApi(request: Request, requestBinary?: RequestBinary) {
  return {
    /** GET /api/v1/usage/me (internal/handler/usage.go MyUsage, Viewer+). */
    async my(params: UsageRangeParams = {}) {
      const suffix = rangeQuery(params).toString();
      return parseMyUsageResponse(await request({ method: 'GET', path: `/api/v1/usage/me${suffix ? `?${suffix}` : ''}`, signal: params.signal }));
    },
    /** GET /api/v1/admin/usage/by-user (usage.go AllUsers, Admin). */
    async byUser(params: UsageByUserParams = {}) {
      if (params.page !== undefined && (!Number.isSafeInteger(params.page) || params.page < 0)) {
        throw new Error('page must be a non-negative integer');
      }
      if (params.pageSize !== undefined && (!Number.isSafeInteger(params.pageSize) || params.pageSize < 1)) {
        throw new Error('pageSize must be a positive integer');
      }
      const query = new URLSearchParams();
      if (params.page !== undefined) query.set('page', String(params.page));
      if (params.pageSize !== undefined) query.set('page_size', String(params.pageSize));
      for (const [key, value] of rangeQuery(params)) query.set(key, value);
      const suffix = query.toString();
      return parseUsageByUserResponse(await request({ method: 'GET', path: `/api/v1/admin/usage/by-user${suffix ? `?${suffix}` : ''}`, signal: params.signal }));
    },
    /**
     * GET /api/v1/admin/usage/export (usage.go Export, Admin): CSV stream with
     * Content-Disposition attachment. Rides requestBinary (chat artifacts /
     * knowledge download precedent) so auth/tenant headers match every other
     * call; the caller turns the returned body + content-disposition into a
     * browser download.
     */
    async exportCsv(params: UsageRangeParams = {}): Promise<ClientBinaryResponse> {
      if (!requestBinary) throw new Error('Binary transport is unavailable');
      const suffix = rangeQuery(params).toString();
      return requestBinary({ method: 'GET', path: `/api/v1/admin/usage/export${suffix ? `?${suffix}` : ''}`, signal: params.signal });
    },
  };
}
export type UsageApi = ReturnType<typeof createUsageApi>;
