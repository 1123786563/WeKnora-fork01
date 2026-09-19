import {
  parseActiveUsersResponse, parseAgentUsageResponse, parseChannelSessionsResponse, parseQueryTrendResponse,
} from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';

export interface AnalyticsRangeParams { startTime?: string; endTime?: string; signal?: AbortSignal; }

function rangeQuery(params: AnalyticsRangeParams): URLSearchParams {
  const query = new URLSearchParams();
  if (params.startTime !== undefined) query.set('start_time', params.startTime);
  if (params.endTime !== undefined) query.set('end_time', params.endTime);
  return query;
}

export function createAnalyticsApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    async queryTrend(params: AnalyticsRangeParams = {}) {
      const suffix = rangeQuery(params).toString();
      return parseQueryTrendResponse(await request({ method: 'GET', path: `/api/v1/analytics/queries${suffix ? `?${suffix}` : ''}`, signal: params.signal }));
    },
    async activeUsers(params: AnalyticsRangeParams = {}) {
      const suffix = rangeQuery(params).toString();
      return parseActiveUsersResponse(await request({ method: 'GET', path: `/api/v1/analytics/users${suffix ? `?${suffix}` : ''}`, signal: params.signal }));
    },
    async channelSessions(params: AnalyticsRangeParams = {}) {
      const suffix = rangeQuery(params).toString();
      return parseChannelSessionsResponse(await request({ method: 'GET', path: `/api/v1/analytics/channels${suffix ? `?${suffix}` : ''}`, signal: params.signal }));
    },
    async agentUsage(agentId: string, params: AnalyticsRangeParams = {}) {
      if (agentId.trim() === '') throw new Error('agentId must not be empty');
      const suffix = rangeQuery(params).toString();
      return parseAgentUsageResponse(await request({ method: 'GET', path: `/api/v1/analytics/agents/${encodeURIComponent(agentId)}${suffix ? `?${suffix}` : ''}`, signal: params.signal }));
    },
  };
}
export type AnalyticsApi = ReturnType<typeof createAnalyticsApi>;
