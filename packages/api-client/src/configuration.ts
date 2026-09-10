import { parseActionSuccessResponse, type ActionSuccessResponse } from '@weknora/contracts';
import type { ClientRequest } from './client.ts';

export interface ConfigurationRecord { id: string; name: string; [key: string]: unknown }
export type AgentConfiguration = ConfigurationRecord & { config?: Record<string, unknown>; is_builtin?: boolean };
export type ModelConfiguration = ConfigurationRecord & { type?: string; source?: string; parameters?: Record<string, unknown> };
export type McpConfiguration = ConfigurationRecord & { enabled?: boolean; url?: string; tools?: unknown[] };
export type SkillConfiguration = ConfigurationRecord & { description?: string; skills_available?: boolean };

type RecordValue = Record<string, unknown>;
function record(value: unknown, path: string): RecordValue {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${path} must be an object`);
  return value as RecordValue;
}
function required(value: unknown, path: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${path} must be a non-empty string`);
  return value;
}
function stripSecrets(value: RecordValue): RecordValue {
  const copy: RecordValue = { ...value };
  delete copy.api_key; delete copy.app_secret; delete copy.access_token; delete copy.refresh_token;
  if (copy.parameters && typeof copy.parameters === 'object' && !Array.isArray(copy.parameters)) {
    copy.parameters = stripSecrets(copy.parameters as RecordValue);
  }
  return copy;
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
  const envelope = record(value, '/skills');
  if (envelope.success !== true || !Array.isArray(envelope.data)) throw new Error('/skills must be a successful list envelope');
  return envelope.data.map((item, index) => {
    const row = stripSecrets(record(item, `/skills.data[${index}]`));
    const name = required(row.name, `/skills.data[${index}].name`);
    return { ...row, id: name, name } as SkillConfiguration;
  });
}
function id(value: string, name: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${name} must not be empty`);
  return encodeURIComponent(value);
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
  const agents = collection<AgentConfiguration>('/api/v1/agents', parseAgent);
  const models = collection<ModelConfiguration>('/api/v1/models', parseModel);
  const mcp = collection<McpConfiguration>('/api/v1/mcp-services', parseMcp);
  return {
    agents,
    models,
    mcp,
    skills: {
      async list(sandboxConfigId?: string, signal?: AbortSignal): Promise<SkillConfiguration[]> {
        const query = sandboxConfigId ? `?sandbox_config_id=${encodeURIComponent(sandboxConfigId)}` : '';
        return parseSkills(await request({ method: 'GET', path: `/api/v1/skills${query}`, ...(signal === undefined ? {} : { signal }) }));
      },
    },
  };
}

export type ConfigurationApi = ReturnType<typeof createConfigurationApi>;
