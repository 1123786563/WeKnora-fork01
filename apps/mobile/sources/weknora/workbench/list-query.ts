/**
 * Query-string builder for the owned workbench execution list.
 *
 * The server derives tenant and owner from the authenticated credential, so
 * this builder can never introduce them as query parameters: a caller-supplied
 * tenant/owner hint must not be able to widen the ownership scope. Only the
 * optional status/agent/cursor/limit facets are encoded, via URLSearchParams
 * so values are percent-encoded rather than string-interpolated.
 */
export interface ExecutionListQueryFilter {
  status?: string;
  agentID?: string;
  cursor?: string;
  limit?: number;
}

export function buildExecutionListQuery(filter: ExecutionListQueryFilter = {}): string {
  const params = new URLSearchParams();
  const status = filter.status?.trim();
  if (status) params.set('status', status);
  const agentID = filter.agentID?.trim();
  if (agentID) params.set('agent_id', agentID);
  const cursor = filter.cursor?.trim();
  if (cursor) params.set('cursor', cursor);
  if (typeof filter.limit === 'number' && Number.isFinite(filter.limit) && filter.limit > 0) {
    params.set('limit', String(Math.floor(filter.limit)));
  }
  return params.toString();
}
