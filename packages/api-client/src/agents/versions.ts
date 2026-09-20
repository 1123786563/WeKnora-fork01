import { parseAgentVersion, type AgentVersion } from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

function required(value: string, name: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${name} must not be empty`);
  return value;
}

export function createAgentVersionsApi(request: Request) {
  return {
    async freezeVersion(agentID: string): Promise<AgentVersion> {
      const id = encodeURIComponent(required(agentID, 'agentID'));
      return parseAgentVersion(await request({ method: 'POST', path: `/api/v1/agents/${id}/versions` }));
    },
    async getVersion(agentID: string, versionID: string): Promise<AgentVersion> {
      const agent = encodeURIComponent(required(agentID, 'agentID'));
      const version = encodeURIComponent(required(versionID, 'versionID'));
      const result = parseAgentVersion(await request({ method: 'GET', path: `/api/v1/agents/${agent}/versions/${version}` }));
      if (result.agent_id !== agentID) throw new Error('AgentVersion agent_id does not match requested agentID');
      return result;
    },
  };
}

export type AgentVersionsApi = ReturnType<typeof createAgentVersionsApi>;
