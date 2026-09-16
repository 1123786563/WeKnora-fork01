import { parseActionSuccessResponse, type ActionSuccessResponse } from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';

export type ToolApprovalDecision = 'approve' | 'reject';
export type MCPOAuthDecision = 'authorize' | 'cancel';

export interface ResolveToolApprovalInput {
  decision: ToolApprovalDecision;
  modifiedArgs?: Record<string, unknown>;
  reason?: string;
}

export interface ResolveMCPOAuthInput {
  serviceId: string;
  decision?: MCPOAuthDecision;
}

function encodedId(value: string, name: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${name} must not be empty`);
  return encodeURIComponent(value);
}

export function createChatApprovalsApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    async resolveTool(
      pendingId: string,
      input: ResolveToolApprovalInput,
      signal?: AbortSignal,
    ): Promise<ActionSuccessResponse> {
      if (input.decision !== 'approve' && input.decision !== 'reject') {
        throw new Error('decision must be approve or reject');
      }
      if (
        input.modifiedArgs !== undefined
        && (typeof input.modifiedArgs !== 'object' || input.modifiedArgs === null || Array.isArray(input.modifiedArgs))
      ) {
        throw new Error('modifiedArgs must be a non-null object');
      }
      return parseActionSuccessResponse(await request({
        method: 'POST',
        path: `/api/v1/agent/tool-approvals/${encodedId(pendingId, 'pendingId')}`,
        body: {
          decision: input.decision,
          ...(input.modifiedArgs === undefined ? {} : { modified_args: input.modifiedArgs }),
          ...(input.reason === undefined ? {} : { reason: input.reason }),
        },
        ...(signal === undefined ? {} : { signal }),
      }));
    },

    async resolveOAuth(
      pendingId: string,
      input: ResolveMCPOAuthInput,
      signal?: AbortSignal,
    ): Promise<ActionSuccessResponse> {
      if (typeof input.serviceId !== 'string' || input.serviceId.trim() === '') {
        throw new Error('serviceId must not be empty');
      }
      if (input.decision !== undefined && input.decision !== 'authorize' && input.decision !== 'cancel') {
        throw new Error('decision must be authorize or cancel');
      }
      return parseActionSuccessResponse(await request({
        method: 'POST',
        path: `/api/v1/agent/mcp-oauth-resolutions/${encodedId(pendingId, 'pendingId')}`,
        body: {
          service_id: input.serviceId,
          ...(input.decision === undefined ? {} : { decision: input.decision }),
        },
        ...(signal === undefined ? {} : { signal }),
      }));
    },

    async cancelOAuth(pendingId: string, signal?: AbortSignal): Promise<ActionSuccessResponse> {
      return parseActionSuccessResponse(await request({
        method: 'POST',
        path: `/api/v1/agent/mcp-oauth-resolutions/${encodedId(pendingId, 'pendingId')}/cancel`,
        body: {},
        ...(signal === undefined ? {} : { signal }),
      }));
    },
  };
}

export type ChatApprovalsApi = ReturnType<typeof createChatApprovalsApi>;
