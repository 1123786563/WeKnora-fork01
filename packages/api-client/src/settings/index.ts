import { parseActionSuccessResponse, type ActionSuccessResponse } from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';
import { array, encoded, query, record, success, withSignal, type IdentityRequest, type JsonRecord } from '../identity/common.ts';

export type SettingsRequest = IdentityRequest;
export type SettingsPayload = Record<string, unknown>;

export interface SettingsResource extends SettingsPayload {
  id?: string;
  name?: string;
}

export interface OllamaStatus extends SettingsPayload {
  available: boolean;
  version?: string;
  baseUrl?: string;
  error?: string;
}

export interface OllamaModel extends SettingsPayload {
  name: string;
  size?: number;
  digest?: string;
  modified_at?: string;
}

export interface ParserProbeResult {
  items: SettingsPayload[];
  docreader_addr?: string;
  docreader_transport?: string;
  connected?: boolean;
}

export interface SystemInfo extends SettingsPayload {
  version?: string;
  edition?: string;
  commit_id?: string;
  build_time?: string;
  go_version?: string;
  started_at?: string;
  uptime_seconds?: number;
}

export interface ConnectionTestResult extends SettingsPayload {
  success: boolean;
  error?: string;
  message?: string;
}

const secretFields = new Set([
  'api_key', 'app_secret', 'access_token', 'refresh_token', 'token', 'client_secret',
  'password', 'secret', 'secret_key', 'secret_access_key', 'hmac_secret',
]);

function isSecretField(key: string): boolean {
  const normalized = key.toLowerCase();
  return secretFields.has(normalized)
    || normalized.endsWith('_api_key')
    || normalized.endsWith('_app_secret')
    || normalized.endsWith('_secret_key')
    || normalized.endsWith('_access_token');
}

function redact(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(redact);
  if (value === null || typeof value !== 'object') return value;
  return Object.fromEntries(Object.entries(value as JsonRecord)
    .filter(([key]) => !isSecretField(key))
    .map(([key, item]) => [key, key === 'credentials' ? redactCredentialStatus(item) : redact(item)]));
}

function redactCredentialStatus(value: unknown): unknown {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return {};
  return Object.fromEntries(Object.entries(value as JsonRecord).flatMap(([key, item]) => {
    if (item === null || typeof item !== 'object' || Array.isArray(item)) return [];
    const configured = (item as JsonRecord).configured;
    return typeof configured === 'boolean' ? [[key, { configured }]] : [];
  }));
}

function data(value: unknown, path: string): unknown {
  const envelope = success(value, path);
  return redact(envelope.data);
}

function dataRecord(value: unknown, path: string): SettingsPayload {
  const result = data(value, path);
  return record(result, `${path}.data`);
}

function dataArray(value: unknown, path: string): SettingsPayload[] {
  return array(data(value, path), `${path}.data`) as SettingsPayload[];
}

/** Memory lists answer with `${success, data, total}`; Vue paginates off `total`. */
export interface MemoryListPage {
  rows: SettingsPayload[];
  total: number;
}

function dataListPage(value: unknown, path: string): MemoryListPage {
  const envelope = success(value, path);
  const rows = array(redact(envelope.data), `${path}.data`) as SettingsPayload[];
  return { rows, total: typeof envelope.total === 'number' ? envelope.total : 0 };
}

function codeData(value: unknown, path: string): unknown {
  const root = record(value, path);
  if (root.code !== 0) throw new Error(`${path}.code must be 0`);
  return redact(root.data);
}

function codeRecord(value: unknown, path: string): SettingsPayload {
  return record(codeData(value, path), `${path}.data`);
}

function parserProbe(value: unknown, path: string): ParserProbeResult {
  const root = record(value, path);
  if (root.code !== 0) throw new Error(`${path}.code must be 0`);
  const rawItems = array(root.data, `${path}.data`);
  return {
    items: rawItems.map((item) => record(redact(item), `${path}.data.item`)),
    ...(typeof root.docreader_addr === 'string' ? { docreader_addr: root.docreader_addr } : {}),
    ...(typeof root.docreader_transport === 'string' ? { docreader_transport: root.docreader_transport } : {}),
    ...(typeof root.connected === 'boolean' ? { connected: root.connected } : {}),
  };
}

function actionResult(value: unknown): ActionSuccessResponse {
  return parseActionSuccessResponse(value);
}

function connectionResult(value: unknown, path: string): ConnectionTestResult {
  const row = record(value, path);
  if (typeof row.success !== 'boolean') throw new Error(`${path}.success must be a boolean`);
  return {
    success: row.success,
    ...(typeof row.error === 'string' ? { error: row.error } : {}),
    ...(typeof row.message === 'string' ? { message: row.message } : {}),
  };
}

function resourceApi(request: SettingsRequest, basePath: string) {
  return {
    async list(signal?: AbortSignal): Promise<SettingsResource[]> {
      return dataArray(await request(withSignal({ method: 'GET', path: basePath }, signal)), basePath) as SettingsResource[];
    },
    async get(id: string, signal?: AbortSignal): Promise<SettingsResource> {
      return dataRecord(await request(withSignal({ method: 'GET', path: `${basePath}/${encoded(id, 'id')}` }, signal)), basePath) as SettingsResource;
    },
    async create(input: SettingsPayload, signal?: AbortSignal): Promise<SettingsResource> {
      return dataRecord(await request(withSignal({ method: 'POST', path: basePath, body: input }, signal)), basePath) as SettingsResource;
    },
    async update(id: string, input: SettingsPayload, signal?: AbortSignal): Promise<SettingsResource> {
      return dataRecord(await request(withSignal({ method: 'PUT', path: `${basePath}/${encoded(id, 'id')}`, body: input }, signal)), basePath) as SettingsResource;
    },
    async remove(id: string, signal?: AbortSignal): Promise<ActionSuccessResponse> {
      return actionResult(await request(withSignal({ method: 'DELETE', path: `${basePath}/${encoded(id, 'id')}` }, signal)));
    },
  };
}

function kvApi(request: SettingsRequest, key: string) {
  const path = `/api/v1/tenants/kv/${key}`;
  return {
    async get(signal?: AbortSignal): Promise<SettingsPayload> {
      return dataRecord(await request(withSignal({ method: 'GET', path }, signal)), path);
    },
    async update(input: SettingsPayload, signal?: AbortSignal): Promise<SettingsPayload> {
      return dataRecord(await request(withSignal({ method: 'PUT', path, body: input }, signal)), path);
    },
  };
}

export function createSettingsApi(request: SettingsRequest) {
  const vectorBase = '/api/v1/vector-stores';
  const webSearchBase = '/api/v1/web-search-providers';
  const storageBase = '/api/v1/storage-backends';

  const ollama = {
    async status(signal?: AbortSignal): Promise<OllamaStatus> {
      return dataRecord(await request(withSignal({ method: 'GET', path: '/api/v1/initialization/ollama/status' }, signal)), '/ollama/status') as OllamaStatus;
    },
    async models(signal?: AbortSignal): Promise<OllamaModel[]> {
      const row = dataRecord(await request(withSignal({ method: 'GET', path: '/api/v1/initialization/ollama/models' }, signal)), '/ollama/models');
      return array(row.models, '/ollama/models.data.models') as OllamaModel[];
    },
    async checkModels(models: string[], signal?: AbortSignal): Promise<SettingsPayload> {
      return dataRecord(await request(withSignal({ method: 'POST', path: '/api/v1/initialization/ollama/models/check', body: { models } }, signal)), '/ollama/models/check');
    },
    async download(modelName: string, signal?: AbortSignal): Promise<SettingsPayload> {
      return dataRecord(await request(withSignal({ method: 'POST', path: '/api/v1/initialization/ollama/models/download', body: { modelName } }, signal)), '/ollama/models/download');
    },
    async progress(taskId: string, signal?: AbortSignal): Promise<SettingsPayload> {
      return dataRecord(await request(withSignal({ method: 'GET', path: `/api/v1/initialization/ollama/download/progress/${encoded(taskId, 'taskId')}` }, signal)), '/ollama/download/progress');
    },
    async tasks(signal?: AbortSignal): Promise<SettingsPayload[]> {
      const row = dataRecord(await request(withSignal({ method: 'GET', path: '/api/v1/initialization/ollama/download/tasks' }, signal)), '/ollama/download/tasks');
      return array(row.tasks, '/ollama/download/tasks.data.tasks') as SettingsPayload[];
    },
  };

  const parser = {
    async engines(signal?: AbortSignal): Promise<ParserProbeResult> {
      return parserProbe(await request(withSignal({ method: 'GET', path: '/api/v1/system/parser-engines' }, signal)), '/system/parser-engines');
    },
    async check(input: SettingsPayload, signal?: AbortSignal): Promise<ParserProbeResult> {
      return parserProbe(await request(withSignal({ method: 'POST', path: '/api/v1/system/parser-engines/check', body: input }, signal)), '/system/parser-engines/check');
    },
    config: kvApi(request, 'parser-engine-config'),
    async reconnect(addr: string, signal?: AbortSignal): Promise<ParserProbeResult> {
      return parserProbe(await request(withSignal({ method: 'POST', path: '/api/v1/system/docreader/reconnect', body: { addr } }, signal)), '/system/docreader/reconnect');
    },
  };

  const storage = {
    backends: {
      ...resourceApi(request, storageBase),
      // Vue StorageBackendSettings reads the default backend from the list
      // envelope's default_storage_backend_id, so expose it alongside rows.
      async listWithEnvelope(signal?: AbortSignal): Promise<{ rows: SettingsResource[]; defaultId: string | undefined }> {
        const payload = await request(withSignal({ method: 'GET', path: storageBase }, signal)) as unknown;
        const row = payload && typeof payload === 'object' && !Array.isArray(payload) ? payload as Record<string, unknown> : {};
        const data = row.data;
        const rows = Array.isArray(data) ? data as SettingsResource[] : [];
        const defaultId = typeof row.default_storage_backend_id === 'string' ? row.default_storage_backend_id : undefined;
        return { rows, defaultId };
      },
      async types(signal?: AbortSignal): Promise<string[]> {
        return dataArray(await request(withSignal({ method: 'GET', path: `${storageBase}/types` }, signal)), `${storageBase}/types`) as unknown as string[];
      },
      async test(input: SettingsPayload, signal?: AbortSignal): Promise<ConnectionTestResult> {
        return connectionResult(await request(withSignal({ method: 'POST', path: `${storageBase}/test`, body: input }, signal)), `${storageBase}/test`);
      },
      async testById(id: string, signal?: AbortSignal): Promise<ConnectionTestResult> {
        return connectionResult(await request(withSignal({ method: 'POST', path: `${storageBase}/${encoded(id, 'id')}/test` }, signal)), `${storageBase}/test`);
      },
      async setDefault(id: string, signal?: AbortSignal): Promise<ActionSuccessResponse> {
        return actionResult(await request(withSignal({ method: 'PUT', path: `${storageBase}/${encoded(id, 'id')}/default`, body: {} }, signal)));
      },
    },
    legacy: {
      config: kvApi(request, 'storage-engine-config'),
      async status(signal?: AbortSignal): Promise<SettingsPayload> {
        return codeRecord(await request(withSignal({ method: 'GET', path: '/api/v1/system/storage-engine-status' }, signal)), '/system/storage-engine-status');
      },
      async test(input: SettingsPayload, signal?: AbortSignal): Promise<SettingsPayload> {
        return codeRecord(await request(withSignal({ method: 'POST', path: '/api/v1/system/storage-engine-check', body: input }, signal)), '/system/storage-engine-check');
      },
    },
  };

  const vectorStores = {
    ...resourceApi(request, vectorBase),
    async types(signal?: AbortSignal): Promise<SettingsPayload[]> {
      return dataArray(await request(withSignal({ method: 'GET', path: `${vectorBase}/types` }, signal)), `${vectorBase}/types`);
    },
    async test(input: SettingsPayload, signal?: AbortSignal): Promise<ConnectionTestResult> {
      return connectionResult(await request(withSignal({ method: 'POST', path: `${vectorBase}/test`, body: input }, signal)), `${vectorBase}/test`);
    },
    async testById(id: string, signal?: AbortSignal): Promise<ConnectionTestResult> {
      return connectionResult(await request(withSignal({ method: 'POST', path: `${vectorBase}/${encoded(id, 'id')}/test` }, signal)), `${vectorBase}/test`);
    },
  };

  const webSearch = {
    providers: {
      ...resourceApi(request, webSearchBase),
      async types(signal?: AbortSignal): Promise<SettingsPayload[]> {
        return dataArray(await request(withSignal({ method: 'GET', path: `${webSearchBase}/types` }, signal)), `${webSearchBase}/types`);
      },
      async test(input: SettingsPayload, signal?: AbortSignal): Promise<ConnectionTestResult> {
        return connectionResult(await request(withSignal({ method: 'POST', path: `${webSearchBase}/test`, body: input }, signal)), `${webSearchBase}/test`);
      },
      async testById(id: string, signal?: AbortSignal): Promise<ConnectionTestResult> {
        return connectionResult(await request(withSignal({ method: 'POST', path: `${webSearchBase}/${encoded(id, 'id')}/test` }, signal)), `${webSearchBase}/test`);
      },
      async putCredentials(id: string, input: SettingsPayload, signal?: AbortSignal): Promise<SettingsPayload> {
        return dataRecord(await request(withSignal({ method: 'PUT', path: `${webSearchBase}/${encoded(id, 'id')}/credentials`, body: input }, signal)), `${webSearchBase}/credentials`);
      },
      async deleteCredential(id: string, field: string, signal?: AbortSignal): Promise<ActionSuccessResponse> {
        return actionResult(await request(withSignal({ method: 'DELETE', path: `${webSearchBase}/${encoded(id, 'id')}/credentials/${encoded(field, 'field')}` }, signal)));
      },
    },
    legacy: kvApi(request, 'web-search-config'),
  };

  const memory = {
    workspace: kvApi(request, 'memory-config'),
    personal: {
      async settings(signal?: AbortSignal): Promise<SettingsPayload> {
        return dataRecord(await request(withSignal({ method: 'GET', path: '/api/v1/memory/settings' }, signal)), '/memory/settings');
      },
      async updateEnabled(enabled: boolean, signal?: AbortSignal): Promise<SettingsPayload> {
        return dataRecord(await request(withSignal({ method: 'PUT', path: '/api/v1/memory/settings', body: { enabled } }, signal)), '/memory/settings');
      },
      async clear(signal?: AbortSignal): Promise<SettingsPayload> {
        return dataRecord(await request(withSignal({ method: 'DELETE', path: '/api/v1/memory/items' }, signal)), '/memory/items');
      },
      async export(signal?: AbortSignal): Promise<SettingsPayload[]> {
        return dataArray(await request(withSignal({ method: 'GET', path: '/api/v1/memory/export' }, signal)), '/memory/export');
      },
      async consolidate(signal?: AbortSignal): Promise<SettingsPayload> {
        return dataRecord(await request(withSignal({ method: 'POST', path: '/api/v1/memory/consolidate', body: {} }, signal)), '/memory/consolidate');
      },
      items: {
        async list(params: { status?: string; limit?: number; offset?: number } = {}, signal?: AbortSignal): Promise<MemoryListPage> {
          const path = query('/api/v1/memory/items', [['status', params.status], ['limit', params.limit], ['offset', params.offset]]);
          return dataListPage(await request(withSignal({ method: 'GET', path }, signal)), '/memory/items');
        },
        async remove(id: string, signal?: AbortSignal): Promise<ActionSuccessResponse> {
          return actionResult(await request(withSignal({ method: 'DELETE', path: `/api/v1/memory/items/${encoded(id, 'id')}` }, signal)));
        },
        async create(input: SettingsPayload, signal?: AbortSignal): Promise<SettingsPayload> {
          return dataRecord(await request(withSignal({ method: 'POST', path: '/api/v1/memory/items', body: input }, signal)), '/memory/items');
        },
        async update(id: string, input: SettingsPayload, signal?: AbortSignal): Promise<SettingsPayload> {
          return dataRecord(await request(withSignal({ method: 'PUT', path: `/api/v1/memory/items/${encoded(id, 'id')}`, body: input }, signal)), '/memory/items');
        },
        async confirm(id: string, signal?: AbortSignal): Promise<SettingsPayload> {
          return dataRecord(await request(withSignal({ method: 'POST', path: `/api/v1/memory/items/${encoded(id, 'id')}/confirm`, body: {} }, signal)), '/memory/items');
        },
        async reject(id: string, signal?: AbortSignal): Promise<ActionSuccessResponse> {
          return actionResult(await request(withSignal({ method: 'POST', path: `/api/v1/memory/items/${encoded(id, 'id')}/reject`, body: {} }, signal)));
        },
      },
      topics: {
        async list(params: { limit?: number; offset?: number } = {}, signal?: AbortSignal): Promise<MemoryListPage> {
          const path = query('/api/v1/memory/topics', [['limit', params.limit], ['offset', params.offset]]);
          return dataListPage(await request(withSignal({ method: 'GET', path }, signal)), '/memory/topics');
        },
        async promote(id: string, signal?: AbortSignal): Promise<SettingsPayload> {
          return dataRecord(await request(withSignal({ method: 'POST', path: `/api/v1/memory/topics/${encoded(id, 'id')}/promote`, body: {} }, signal)), '/memory/topics');
        },
        async remove(id: string, signal?: AbortSignal): Promise<ActionSuccessResponse> {
          return actionResult(await request(withSignal({ method: 'DELETE', path: `/api/v1/memory/topics/${encoded(id, 'id')}` }, signal)));
        },
      },
      documents: {
        async list(params: { limit?: number; offset?: number } = {}, signal?: AbortSignal): Promise<MemoryListPage> {
          const path = query('/api/v1/memory/documents', [['limit', params.limit], ['offset', params.offset]]);
          return dataListPage(await request(withSignal({ method: 'GET', path }, signal)), '/memory/documents');
        },
        async remove(id: string, signal?: AbortSignal): Promise<ActionSuccessResponse> {
          return actionResult(await request(withSignal({ method: 'DELETE', path: `/api/v1/memory/documents/${encoded(id, 'id')}` }, signal)));
        },
      },
    },
  };

  const envVars = {
    async list(signal?: AbortSignal): Promise<SettingsPayload[]> {
      return dataArray(await request(withSignal({ method: 'GET', path: '/api/v1/me/env-vars' }, signal)), '/me/env-vars');
    },
    skill: {
      async set(skillId: string, name: string, value: string, signal?: AbortSignal): Promise<ActionSuccessResponse> {
        return actionResult(await request(withSignal({ method: 'PUT', path: '/api/v1/me/env-vars/skill', body: { skill_id: skillId, name, value } }, signal)));
      },
      async remove(skillId: string, name: string, signal?: AbortSignal): Promise<ActionSuccessResponse> {
        return actionResult(await request(withSignal({ method: 'DELETE', path: '/api/v1/me/env-vars/skill', body: { skill_id: skillId, name } }, signal)));
      },
    },
    sandbox: {
      async set(configId: string, name: string, value: string, signal?: AbortSignal): Promise<ActionSuccessResponse> {
        return actionResult(await request(withSignal({ method: 'PUT', path: '/api/v1/me/env-vars/sandbox', body: { sandbox_config_id: configId, name, value } }, signal)));
      },
      async remove(configId: string, name: string, signal?: AbortSignal): Promise<ActionSuccessResponse> {
        return actionResult(await request(withSignal({ method: 'DELETE', path: '/api/v1/me/env-vars/sandbox', body: { sandbox_config_id: configId, name } }, signal)));
      },
    },
  };

  return {
    preferences: {
      async get(signal?: AbortSignal): Promise<SettingsPayload> {
        const root = dataRecord(await request(withSignal({ method: 'GET', path: '/api/v1/auth/me' }, signal)), '/auth/me');
        return record(root.user, '/auth/me.data.user').preferences as SettingsPayload;
      },
      async update(input: SettingsPayload, signal?: AbortSignal): Promise<SettingsPayload> {
        return dataRecord(await request(withSignal({ method: 'PUT', path: '/api/v1/auth/me/preferences', body: input }, signal)), '/auth/me/preferences');
      },
    },
    profile: {
      async get(signal?: AbortSignal): Promise<SettingsPayload> {
        const root = dataRecord(await request(withSignal({ method: 'GET', path: '/api/v1/auth/me' }, signal)), '/auth/me');
        return record(root.user, '/auth/me.data.user');
      },
      async changePassword(input: SettingsPayload, signal?: AbortSignal): Promise<ActionSuccessResponse> {
        return actionResult(await request(withSignal({ method: 'POST', path: '/api/v1/auth/change-password', body: input }, signal)));
      },
    },
    tenant: {
      async get(signal?: AbortSignal): Promise<SettingsPayload> {
        const root = dataRecord(await request(withSignal({ method: 'GET', path: '/api/v1/auth/me' }, signal)), '/auth/me');
        return record(root.tenant, '/auth/me.data.tenant');
      },
      async update(id: number, input: SettingsPayload, signal?: AbortSignal): Promise<SettingsPayload> {
        return dataRecord(await request(withSignal({ method: 'PUT', path: `/api/v1/tenants/${encoded(id, 'tenantId')}`, body: input }, signal)), '/tenants/:id');
      },
    },
    chatHistory: {
      config: kvApi(request, 'chat-history-config'),
      async stats(signal?: AbortSignal): Promise<SettingsPayload> {
        return dataRecord(await request(withSignal({ method: 'GET', path: '/api/v1/messages/chat-history-stats' }, signal)), '/messages/chat-history-stats');
      },
      async search(input: SettingsPayload, signal?: AbortSignal): Promise<SettingsPayload> {
        return dataRecord(await request(withSignal({ method: 'POST', path: '/api/v1/messages/search', body: input }, signal)), '/messages/search');
      },
    },
    ollama,
    parser,
    retrieval: kvApi(request, 'retrieval-config'),
    memory,
    envVars,
    storage,
    vectorStores,
    webSearch,
    system: {
      async info(signal?: AbortSignal): Promise<SystemInfo> {
        return codeRecord(await request(withSignal({ method: 'GET', path: '/api/v1/system/info' }, signal)), '/system/info') as SystemInfo;
      },
    },
    weknoraCloud: {
      async status(signal?: AbortSignal): Promise<SettingsPayload> {
        const root = record(await request(withSignal({ method: 'GET', path: '/api/v1/models/weknoracloud/status' }, signal)), '/models/weknoracloud/status');
        return record(redact(root), '/models/weknoracloud/status');
      },
      async saveCredentials(input: SettingsPayload, signal?: AbortSignal): Promise<ActionSuccessResponse> {
        return actionResult(await request(withSignal({ method: 'POST', path: '/api/v1/weknoracloud/credentials', body: input }, signal)));
      },
    },
  };
}

export type SettingsApi = ReturnType<typeof createSettingsApi>;
