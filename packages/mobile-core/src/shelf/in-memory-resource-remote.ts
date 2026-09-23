import type { ResourceRemote } from './ports.ts';

/** in-memory 场景脚本：status 可为常数或按 token 判定（用于 401 刷新重试场景）。 */
export interface ResourceRemoteScript {
  agents?: ReadonlyArray<Record<string, unknown>>;
  disabledOwnAgentIds?: ReadonlyArray<string>;
  knowledgeBases?: ReadonlyArray<Record<string, unknown>>;
  connections?: ReadonlyArray<Record<string, unknown>>;
  status?: {
    agents?: number | ((token: string) => number | undefined);
    knowledgeBases?: number | ((token: string) => number | undefined);
    connections?: number | ((token: string) => number | undefined);
  };
}

export interface ScriptedResourceRemote extends ResourceRemote {
  calls: Array<{ kind: 'agents' | 'knowledgeBases' | 'connections'; token: string }>;
}

function httpError(status: number): Error {
  const error = new Error(`HTTP ${status}`);
  (error as { status?: number }).status = status;
  return error;
}

function resolveStatus(status: number | ((token: string) => number | undefined) | undefined, token: string): number | undefined {
  return typeof status === 'function' ? status(token) : status;
}

function respond<T>(rows: T, status: number | undefined): T {
  if (status !== undefined) throw httpError(status);
  return rows;
}

export function createInMemoryResourceRemote(script: ResourceRemoteScript): ScriptedResourceRemote {
  const calls: ScriptedResourceRemote['calls'] = [];
  return {
    calls,
    async availableAgents(token) {
      calls.push({ kind: 'agents', token });
      return respond(
        { rows: [...(script.agents ?? [])], disabledOwnAgentIds: new Set(script.disabledOwnAgentIds ?? []) },
        resolveStatus(script.status?.agents, token),
      );
    },
    async knowledgeBases(token) {
      calls.push({ kind: 'knowledgeBases', token });
      return respond([...(script.knowledgeBases ?? [])], resolveStatus(script.status?.knowledgeBases, token));
    },
    async connections(token) {
      calls.push({ kind: 'connections', token });
      return respond([...(script.connections ?? [])], resolveStatus(script.status?.connections, token));
    },
  };
}
