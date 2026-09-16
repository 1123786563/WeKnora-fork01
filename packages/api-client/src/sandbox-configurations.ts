import type { ClientRequest } from './client.ts';

export type SandboxBackendType = 'cube' | 'e2b' | 'docker';

export interface CubeSandboxConfig {
  api_url?: string;
  proxy_url?: string;
  sandbox_domain?: string;
  api_key?: string;
  template_id?: string;
  http_timeout_sec?: number;
  cube_sandbox_ttl_seconds?: number;
  dns_servers?: string[];
}

export interface E2BSandboxConfig {
  api_url?: string;
  sandbox_domain?: string;
  api_key?: string;
  template_id?: string;
  proxy_url?: string;
  http_timeout_sec?: number;
  e2b_sandbox_ttl_seconds?: number;
}

export interface DockerSandboxConfig {
  image?: string;
  host?: string;
  tls_cert_path?: string;
  cpu_limit?: number;
  memory_limit_mb?: number;
  pids_limit?: number;
  network_mode?: 'bridge' | 'none' | string;
  runtime?: string;
  idle_ttl_seconds?: number;
  http_timeout_sec?: number;
}

export interface TenantSandboxConfig {
  sandbox_type?: string;
  default_timeout_sec?: number;
  terminal_idle_disconnect_sec?: number;
  allow_private_endpoints?: boolean;
  env_vars?: Record<string, string>;
  cube?: CubeSandboxConfig;
  e2b?: E2BSandboxConfig;
  docker?: DockerSandboxConfig;
  [key: string]: unknown;
}

export interface SandboxConfigRecord {
  id: string;
  name: string;
  description?: string;
  sandbox_type: string;
  config: TenantSandboxConfig;
  created_at: string;
  updated_at: string;
}

export interface SandboxConfigUpsert {
  name: string;
  description?: string;
  config: TenantSandboxConfig;
}

export interface SandboxInventory {
  sandboxCount: number;
  sessionIds: string[];
  agentNames: string[];
  unverifiable: boolean;
}

export type SandboxConfigurationConflictCode =
  | 'sandboxes_still_live'
  | 'sandbox_inventory_unverifiable'
  | 'skill_snapshot_blocks_template';

export interface SandboxConfigurationConflict {
  code: SandboxConfigurationConflictCode;
  message?: string;
  inventory?: SandboxInventory;
}

export interface SandboxConfigurationsApi {
  list(signal?: AbortSignal): Promise<{ items: SandboxConfigRecord[]; workspaceScriptsDisabled: boolean }>;
  get(id: string, signal?: AbortSignal): Promise<SandboxConfigRecord>;
  create(input: SandboxConfigUpsert, signal?: AbortSignal): Promise<SandboxConfigRecord>;
  update(id: string, input: SandboxConfigUpsert, signal?: AbortSignal): Promise<SandboxConfigRecord>;
  remove(id: string, signal?: AbortSignal, force?: boolean): Promise<void>;
  inventory(id: string, signal?: AbortSignal): Promise<SandboxInventory>;
  setWorkspacePolicy(scriptsDisabled: boolean, signal?: AbortSignal): Promise<{ workspaceScriptsDisabled: boolean }>;
}

type JsonRecord = Record<string, unknown>;
type Request = (request: ClientRequest) => Promise<unknown>;

function record(value: unknown, path: string): JsonRecord {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${path} must be an object`);
  return value as JsonRecord;
}

function string(value: unknown, path: string): string {
  if (typeof value !== 'string') throw new Error(`${path} must be a string`);
  return value;
}

function requiredString(value: unknown, path: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${path} must be a non-empty string`);
  return value;
}

function encoded(value: string, label: string): string {
  if (value.trim() === '') throw new Error(`${label} must not be empty`);
  return encodeURIComponent(value);
}

function successful(value: unknown, path: string): JsonRecord {
  const envelope = record(value, path);
  if (envelope.success !== true) throw new Error(`${path}.success must be true`);
  return envelope;
}

function config(value: unknown, path: string): TenantSandboxConfig {
  return record(value, path) as TenantSandboxConfig;
}

function sandboxRecord(value: unknown, path: string): SandboxConfigRecord {
  const row = record(value, path);
  return {
    id: requiredString(row.id, `${path}.id`),
    name: requiredString(row.name, `${path}.name`),
    ...(typeof row.description === 'string' ? { description: row.description } : {}),
    sandbox_type: requiredString(row.sandbox_type, `${path}.sandbox_type`),
    config: config(row.config, `${path}.config`),
    created_at: string(row.created_at, `${path}.created_at`),
    updated_at: string(row.updated_at, `${path}.updated_at`),
  };
}

function inventory(value: unknown, path: string): SandboxInventory {
  const row = record(value, path);
  if (typeof row.sandbox_count !== 'number' || !Number.isFinite(row.sandbox_count)) throw new Error(`${path}.sandbox_count must be a number`);
  const sessionIds = row.session_ids === undefined ? [] : row.session_ids;
  const agentNames = row.agent_names === undefined ? [] : row.agent_names;
  if (!Array.isArray(sessionIds) || !sessionIds.every((item) => typeof item === 'string')) throw new Error(`${path}.session_ids must be an array of strings`);
  if (!Array.isArray(agentNames) || !agentNames.every((item) => typeof item === 'string')) throw new Error(`${path}.agent_names must be an array of strings`);
  return {
    sandboxCount: row.sandbox_count,
    sessionIds: sessionIds as string[],
    agentNames: agentNames as string[],
    unverifiable: row.unverifiable === true,
  };
}

function data(value: unknown, path: string): unknown {
  return successful(value, path).data;
}

export function parseSandboxConfigurationConflict(value: unknown): SandboxConfigurationConflict | null {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return null;
  const root = value as JsonRecord;
  if (root.error === null || typeof root.error !== 'object' || Array.isArray(root.error)) return null;
  const detail = root.error as JsonRecord;
  const code = detail.code;
  if (code !== 'sandboxes_still_live' && code !== 'sandbox_inventory_unverifiable' && code !== 'skill_snapshot_blocks_template') return null;
  return {
    code,
    ...(typeof detail.message === 'string' ? { message: detail.message } : {}),
    ...(detail.data !== undefined ? { inventory: inventory(detail.data, '/error.data') } : {}),
  };
}

export function createSandboxConfigurationsApi(request: Request): SandboxConfigurationsApi {
  const basePath = '/api/v1/sandbox-configs';
  const withSignal = (input: ClientRequest, signal?: AbortSignal): ClientRequest => signal ? { ...input, signal } : input;

  return {
    async list(signal) {
      const raw = record(await request(withSignal({ method: 'GET', path: basePath }, signal)), basePath);
      if (raw.success !== true || !Array.isArray(raw.data)) throw new Error(`${basePath} must be a successful list envelope`);
      const envelope = raw as JsonRecord & { data: unknown[] };
      return {
        items: envelope.data.map((item, index) => sandboxRecord(item, `${basePath}.data[${index}]`)),
        workspaceScriptsDisabled: envelope.workspace_scripts_disabled === true,
      };
    },
    async get(id, signal) {
      return sandboxRecord(data(await request(withSignal({ method: 'GET', path: `${basePath}/${encoded(id, 'id')}` }, signal)), `${basePath}/:id`), `${basePath}/:id.data`);
    },
    async create(input, signal) {
      return sandboxRecord(data(await request(withSignal({ method: 'POST', path: basePath, body: input }, signal)), basePath), `${basePath}.data`);
    },
    async update(id, input, signal) {
      return sandboxRecord(data(await request(withSignal({ method: 'PUT', path: `${basePath}/${encoded(id, 'id')}`, body: input }, signal)), `${basePath}/:id`), `${basePath}/:id.data`);
    },
    async remove(id, signal, force = false) {
      successful(await request(withSignal({ method: 'DELETE', path: `${basePath}/${encoded(id, 'id')}${force ? '?force=true' : ''}` }, signal)), `${basePath}/:id`);
    },
    async inventory(id, signal) {
      return inventory(data(await request(withSignal({ method: 'GET', path: `${basePath}/${encoded(id, 'id')}/sandboxes` }, signal)), `${basePath}/:id/sandboxes`), `${basePath}/:id/sandboxes.data`);
    },
    async setWorkspacePolicy(scriptsDisabled, signal) {
      const envelope = successful(await request(withSignal({ method: 'PUT', path: `${basePath}/workspace-policy`, body: { scripts_disabled: scriptsDisabled } }, signal)), `${basePath}/workspace-policy`);
      return { workspaceScriptsDisabled: envelope.workspace_scripts_disabled === true };
    },
  };
}
