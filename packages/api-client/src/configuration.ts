import { parseActionSuccessResponse, type ActionSuccessResponse } from '@weknora/contracts';
import type { ClientRequest } from './client.ts';

export interface ConfigurationRecord { id: string; name: string; [key: string]: unknown }
export type AgentConfiguration = ConfigurationRecord & { config?: Record<string, unknown>; is_builtin?: boolean };
export type ModelConfiguration = ConfigurationRecord & { type?: string; source?: string; parameters?: Record<string, unknown> };
export type McpConfiguration = ConfigurationRecord & { enabled?: boolean; url?: string; tools?: unknown[] };
export type SkillConfiguration = ConfigurationRecord & { description?: string; skills_available?: boolean };
export interface AgentConfigurationList { items: AgentConfiguration[]; disabledOwnAgentIds: string[] }
export interface SkillConfigurationList { items: SkillConfiguration[]; skillsAvailable: boolean }
export interface McpOAuthAuthorization { authorizationUrl: string; authorizationAttempt: string }
export interface McpOAuthStatus { authorized: boolean; state: 'authorized' | 'refreshable' | 'reauth_required' | 'pending'; refreshAvailable: boolean; expiresAt?: string }
export interface AgentConfigurationListOptions {
  creator?: 'all' | 'mine' | 'others';
  signal?: AbortSignal;
}

type RecordValue = Record<string, unknown>;
function record(value: unknown, path: string): RecordValue {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${path} must be an object`);
  return value as RecordValue;
}
function required(value: unknown, path: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${path} must be a non-empty string`);
  return value;
}
const secretFields = new Set([
  'api_key', 'app_secret', 'access_token', 'refresh_token', 'token', 'client_secret', 'password', 'secret',
]);
function credentialStatus(value: unknown): RecordValue {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return {};
  return Object.fromEntries(Object.entries(value as RecordValue).flatMap(([field, metadata]) => {
    if (metadata === null || typeof metadata !== 'object' || Array.isArray(metadata)) return [];
    const configured = (metadata as RecordValue).configured;
    return typeof configured === 'boolean' ? [[field, { configured }]] : [];
  }));
}
function withoutSecrets(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(withoutSecrets);
  if (value === null || typeof value !== 'object') return value;
  return Object.fromEntries(
    Object.entries(value as RecordValue)
      .filter(([key]) => !secretFields.has(key.toLowerCase()))
      .map(([key, item]) => [key, key === 'credentials' ? credentialStatus(item) : withoutSecrets(item)]),
  );
}
function stripSecrets(value: RecordValue): RecordValue {
  return withoutSecrets(value) as RecordValue;
}
function parseRecord(value: unknown, path: string): ConfigurationRecord {
  const row = stripSecrets(record(value, path));
  return { ...row, id: required(row.id, `${path}.id`), name: required(row.name, `${path}.name`) };
}
function parseList(value: unknown, path: string): ConfigurationRecord[] {
  const envelope = record(value, path);
  if (envelope.success !== true || !Array.isArray(envelope.data)) throw new Error(`${path} must be a successful list envelope`);
  return envelope.data.map((item, index) => parseRecord(item, `${path}.data[${index}]`));
}
function parseOne(value: unknown, path: string): ConfigurationRecord {
  const envelope = record(value, path);
  if (envelope.success !== true) throw new Error(`${path}.success must be true`);
  return parseRecord(envelope.data, `${path}.data`);
}
function parseSkills(value: unknown): SkillConfiguration[] {
  return parseSkillList(value).items;
}
function parseSkillList(value: unknown): SkillConfigurationList {
  const envelope = record(value, '/skills');
  if (envelope.success !== true || !Array.isArray(envelope.data)) throw new Error('/skills must be a successful list envelope');
  if (typeof envelope.skills_available !== 'boolean') throw new Error('/skills.skills_available must be a boolean');
  const items = envelope.data.map((item, index) => {
    const row = stripSecrets(record(item, `/skills.data[${index}]`));
    const name = required(row.name, `/skills.data[${index}].name`);
    return { ...row, id: name, name } as SkillConfiguration;
  });
  return { items, skillsAvailable: envelope.skills_available };
}
function parseAgentList(value: unknown, path: string): AgentConfigurationList {
  const envelope = record(value, path);
  const items = parseList(value, path) as AgentConfiguration[];
  const rawDisabled = envelope.disabled_own_agent_ids;
  if (rawDisabled !== undefined && (!Array.isArray(rawDisabled) || rawDisabled.some((item) => typeof item !== 'string'))) {
    throw new Error(`${path}.disabled_own_agent_ids must be a string array`);
  }
  return { items, disabledOwnAgentIds: (rawDisabled as string[] | undefined) ?? [] };
}
function id(value: string, name: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${name} must not be empty`);
  return encodeURIComponent(value);
}

function parseOAuthAuthorization(value: unknown): McpOAuthAuthorization {
  const envelope = record(value, '/mcp-services/oauth/authorize-url');
  if (envelope.success !== true) throw new Error('/mcp-services/oauth/authorize-url.success must be true');
  const data = record(envelope.data, '/mcp-services/oauth/authorize-url.data');
  return {
    authorizationUrl: required(data.authorization_url, '/mcp-services/oauth/authorize-url.data.authorization_url'),
    authorizationAttempt: required(data.authorization_attempt, '/mcp-services/oauth/authorize-url.data.authorization_attempt'),
  };
}

function parseOAuthStatus(value: unknown): McpOAuthStatus {
  const envelope = record(value, '/mcp-services/oauth/status');
  if (envelope.success !== true) throw new Error('/mcp-services/oauth/status.success must be true');
  const data = record(envelope.data, '/mcp-services/oauth/status.data');
  if (typeof data.authorized !== 'boolean') throw new Error('/mcp-services/oauth/status.data.authorized must be a boolean');
  if (data.state !== 'authorized' && data.state !== 'refreshable' && data.state !== 'reauth_required' && data.state !== 'pending') {
    throw new Error('/mcp-services/oauth/status.data.state is invalid');
  }
  if (typeof data.refresh_available !== 'boolean') throw new Error('/mcp-services/oauth/status.data.refresh_available must be a boolean');
  if (data.expires_at !== undefined && typeof data.expires_at !== 'string') throw new Error('/mcp-services/oauth/status.data.expires_at must be a string');
  return { authorized: data.authorized, state: data.state, refreshAvailable: data.refresh_available, ...(data.expires_at === undefined ? {} : { expiresAt: data.expires_at }) };
}

export function createConfigurationApi(request: (input: ClientRequest) => Promise<unknown>) {
  const collection = <T extends ConfigurationRecord>(path: string, parse: (value: unknown, path: string) => T) => ({
    async list(signal?: AbortSignal): Promise<T[]> {
      return parseList(await request({ method: 'GET', path, ...(signal === undefined ? {} : { signal }) }), path) as T[];
    },
    async get(itemId: string, signal?: AbortSignal): Promise<T> {
      return parseOne(await request({ method: 'GET', path: `${path}/${id(itemId, 'id')}`, ...(signal === undefined ? {} : { signal }) }), path) as T;
    },
    async create(input: Record<string, unknown>, signal?: AbortSignal): Promise<T> {
      return parse(await request({ method: 'POST', path, body: input, ...(signal === undefined ? {} : { signal }) }), path);
    },
    async update(itemId: string, input: Record<string, unknown>, signal?: AbortSignal): Promise<T> {
      return parse(await request({ method: 'PUT', path: `${path}/${id(itemId, 'id')}`, body: input, ...(signal === undefined ? {} : { signal }) }), path);
    },
    async remove(itemId: string, signal?: AbortSignal): Promise<ActionSuccessResponse> {
      return parseActionSuccessResponse(await request({ method: 'DELETE', path: `${path}/${id(itemId, 'id')}`, ...(signal === undefined ? {} : { signal }) }));
    },
  });

  const parseAgent = (value: unknown, path: string) => parseOne(value, path) as AgentConfiguration;
  const parseModel = (value: unknown, path: string) => parseOne(value, path) as ModelConfiguration;
  const parseMcp = (value: unknown, path: string) => parseOne(value, path) as McpConfiguration;
  const agentCollection = collection<AgentConfiguration>('/api/v1/agents', parseAgent);
  const models = collection<ModelConfiguration>('/api/v1/models', parseModel);
  const mcp = collection<McpConfiguration>('/api/v1/mcp-services', parseMcp);
  const mcpOAuth = {
    async authorizeUrl(serviceId: string, input: { redirectURI: string; frontendRedirect?: string }, signal?: AbortSignal): Promise<McpOAuthAuthorization> {
      if (typeof input.redirectURI !== 'string' || input.redirectURI.trim() === '') throw new Error('redirectURI must not be empty');
      return parseOAuthAuthorization(await request({
        method: 'POST', path: `/api/v1/mcp-services/${id(serviceId, 'serviceId')}/oauth/authorize-url`,
        body: { redirect_uri: input.redirectURI, ...(input.frontendRedirect === undefined ? {} : { frontend_redirect: input.frontendRedirect }) },
        ...(signal === undefined ? {} : { signal }),
      }));
    },
    async status(serviceId: string, authorizationAttempt?: string, signal?: AbortSignal): Promise<McpOAuthStatus> {
      const query = authorizationAttempt === undefined ? '' : `?authorization_attempt=${encodeURIComponent(authorizationAttempt)}`;
      return parseOAuthStatus(await request({ method: 'GET', path: `/api/v1/mcp-services/${id(serviceId, 'serviceId')}/oauth/status${query}`, ...(signal === undefined ? {} : { signal }) }));
    },
    async revoke(serviceId: string, signal?: AbortSignal): Promise<ActionSuccessResponse> {
      return parseActionSuccessResponse(await request({ method: 'DELETE', path: `/api/v1/mcp-services/${id(serviceId, 'serviceId')}/oauth/token`, ...(signal === undefined ? {} : { signal }) }));
    },
  };
  return {
    agents: {
      ...agentCollection,
      async listWithState(options: AgentConfigurationListOptions = {}): Promise<AgentConfigurationList> {
        if (options.creator !== undefined && !['all', 'mine', 'others'].includes(options.creator)) {
          throw new Error('creator must be all, mine, or others');
        }
        const query = options.creator && options.creator !== 'all' ? `?creator=${options.creator}` : '';
        const path = `/api/v1/agents${query}`;
        return parseAgentList(await request({
          method: 'GET', path, ...(options.signal === undefined ? {} : { signal: options.signal }),
        }), '/api/v1/agents');
      },
    },
    models,
    mcp: { ...mcp, oauth: mcpOAuth },
    skills: {
      async list(sandboxConfigId?: string, signal?: AbortSignal): Promise<SkillConfiguration[]> {
        const query = sandboxConfigId ? `?sandbox_config_id=${encodeURIComponent(sandboxConfigId)}` : '';
        return parseSkills(await request({ method: 'GET', path: `/api/v1/skills${query}`, ...(signal === undefined ? {} : { signal }) }));
      },
      async listWithAvailability(sandboxConfigId?: string, signal?: AbortSignal): Promise<SkillConfigurationList> {
        const query = sandboxConfigId ? `?sandbox_config_id=${encodeURIComponent(sandboxConfigId)}` : '';
        return parseSkillList(await request({
          method: 'GET', path: `/api/v1/skills${query}`, ...(signal === undefined ? {} : { signal }),
        }));
      },
    },
  };
}

export type ConfigurationApi = ReturnType<typeof createConfigurationApi>;
