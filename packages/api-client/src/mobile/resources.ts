import type { ClientRequest } from '../client.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface MobileResourceRemoteOptions {
  /**
   * 部署 Origin。构造即强校验（与 createMobileRuntimeRemote 相同规则）：绝对 HTTPS URL、
   * 无内嵌 user-info、无 path/query/fragment——非法 Origin 在任何请求发出前同步抛错。
   */
  origin: string;
  /** 复用既有 ClientRequest 通道（createWeKnoraClient().request），本适配器不新建传输。 */
  request: Request;
}

/**
 * WeKnora 资源读 seam 的具体适配（spec §6.3 Resource Backend Port）。组合三个 Viewer+
 * 读端点，只做信封解包与 wire→语义行投影；能力三态裁决与失效归 mobile-core Resource Shelf。
 */
export interface MobileResourceRemote {
  availableAgents(accessToken: string): Promise<{ rows: ReadonlyArray<Record<string, unknown>>; disabledOwnAgentIds: ReadonlySet<string> }>;
  knowledgeBases(accessToken: string): Promise<ReadonlyArray<Record<string, unknown>>>;
  connections(accessToken: string): Promise<ReadonlyArray<Record<string, unknown>>>;
}

function requireDeploymentOrigin(origin: string): void {
  let parsed: URL;
  if (typeof origin !== 'string' || origin.trim() === '') throw new Error('deployment origin is required');
  try {
    parsed = new URL(origin);
  } catch {
    throw new Error(`deployment origin must be an absolute URL: ${origin}`);
  }
  if (parsed.protocol !== 'https:') throw new Error('deployment origin must use HTTPS');
  if (parsed.username !== '' || parsed.password !== '') throw new Error('deployment origin must not embed user info');
  if (parsed.hostname === '') throw new Error('deployment origin must include a host');
  if (parsed.pathname !== '/') throw new Error('deployment origin must not include a path');
  if (parsed.search !== '' || parsed.hash !== '') throw new Error('deployment origin must not include a query or fragment');
}

function requireAccessToken(accessToken: string): string {
  if (typeof accessToken !== 'string' || accessToken.trim() === '') throw new Error('access token is required');
  return accessToken;
}

function bearerRequest(request: Request, accessToken: string): Request {
  return (input: ClientRequest): Promise<unknown> =>
    request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
}

function envelope(value: unknown, path: string): { data: unknown; root: Record<string, unknown> } {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error(`${path} response must be an object`);
  const root = value as Record<string, unknown>;
  if (root.success !== true) throw new Error(`${path} response.success must be true`);
  if (!Object.prototype.hasOwnProperty.call(root, 'data')) throw new Error(`${path} response.data is required`);
  return { data: root.data, root };
}

function rowsOf(data: unknown, path: string): ReadonlyArray<Record<string, unknown>> {
  if (!Array.isArray(data)) throw new Error(`${path} response.data must be an array`);
  return data.filter((row): row is Record<string, unknown> => typeof row === 'object' && row !== null && !Array.isArray(row));
}

export function createMobileResourceRemote(options: MobileResourceRemoteOptions): MobileResourceRemote {
  requireDeploymentOrigin(options.origin);
  const request = options.request;
  const get = (accessToken: string, path: string): Promise<unknown> =>
    bearerRequest(request, requireAccessToken(accessToken))({ method: 'GET', path });
  return {
    async availableAgents(accessToken) {
      const { data, root } = envelope(await get(accessToken, '/api/v1/agents'), '/api/v1/agents');
      const rows = rowsOf(data, '/api/v1/agents').map((row) => ({
        id: row.id,
        name: row.name,
        // 展示摘要来自 wire description；config（含 system_prompt 等敏感 Prompt 配置）结构性不提升。
        summary: row.description,
        // 能力事实来源：Viewer+ 的租户范围 /agents 列表把该行返回给本成员（与 Web 会话下拉同源事实）。
        kind: row.is_builtin === true ? 'general' : 'custom',
        capability: { state: 'supported', reason: '' },
      }));
      const disabled = Array.isArray(root.disabled_own_agent_ids) ? root.disabled_own_agent_ids : [];
      return { rows, disabledOwnAgentIds: new Set(disabled.filter((id): id is string => typeof id === 'string')) };
    },
    async knowledgeBases(accessToken) {
      const { data } = envelope(await get(accessToken, '/api/v1/knowledge-bases'), '/api/v1/knowledge-bases');
      return rowsOf(data, '/api/v1/knowledge-bases').map((row) => ({
        id: row.id,
        title: row.name,
        // 扫描状态推导：is_processing 优先；store 状态 available→indexed / unavailable→failed；缺失→pending（不臆造 indexed）。
        scan_status:
          row.is_processing === true ? 'scanning'
          : row.vector_store_status === 'unavailable' ? 'failed'
          : row.vector_store_status === 'available' ? 'indexed'
          : 'pending',
        document_count: row.knowledge_count,
        updated_at: row.updated_at,
      }));
    },
    async connections(accessToken) {
      const { data } = envelope(await get(accessToken, '/api/v1/apps/connections'), '/api/v1/apps/connections');
      return rowsOf(data, '/api/v1/apps/connections').map((row) => ({ id: row.id, kind: row.kind, state: row.state }));
    },
  };
}
