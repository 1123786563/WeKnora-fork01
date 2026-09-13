import type { ClientRequest } from './client.ts';

export interface DataSource {
  id: string;
  tenant_id?: number | string;
  knowledge_base_id: string;
  name: string;
  type: string;
  config?: unknown;
  sync_schedule?: string;
  sync_mode?: 'incremental' | 'full' | string;
  status?: 'active' | 'paused' | 'error' | string;
  conflict_strategy?: 'overwrite' | 'skip' | string;
  sync_deletions?: boolean;
  last_sync_at?: string | null;
  last_sync_result?: unknown;
  error_message?: string;
  latest_sync_log?: DataSourceSyncLog;
  [key: string]: unknown;
}

export interface DataSourceResource {
  external_id: string;
  name: string;
  type: string;
  description?: string;
  url?: string;
  parent_id?: string;
  has_children?: boolean;
  [key: string]: unknown;
}

export interface DataSourceSyncLog {
  id: string;
  status: string;
  started_at?: string | null;
  finished_at?: string | null;
  items_total?: number;
  items_created?: number;
  items_updated?: number;
  items_deleted?: number;
  items_skipped?: number;
  items_failed?: number;
  error_message?: string;
  [key: string]: unknown;
}

export interface DataSourceConnectorType {
  type: string;
  name: string;
  description: string;
  icon?: string;
  priority: number;
  auth_type: string;
  capabilities: string[];
}

function row(value: unknown, label: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error(`Invalid ${label}`);
  return value as Record<string, unknown>;
}

function parseDataSource(value: unknown): DataSource {
  const source = row(value, 'data source');
  for (const key of ['id', 'knowledge_base_id', 'name', 'type']) {
    if (typeof source[key] !== 'string' || source[key].trim() === '') throw new Error(`Invalid data source field: ${key}`);
  }
  return source as DataSource;
}

function parseDataSources(value: unknown): DataSource[] {
  const raw = Array.isArray(value) ? value : row(value, 'data source list').data;
  if (!Array.isArray(raw)) throw new Error('Invalid data source list');
  return raw.map(parseDataSource);
}

export function createDataSourcesApi(request: (input: ClientRequest) => Promise<unknown>) {
  const path = (id: string, suffix = '') => `/api/v1/datasource/${encodeURIComponent(id)}${suffix}`;
  return {
    async types(): Promise<DataSourceConnectorType[]> {
      const value = await request({ method: 'GET', path: '/api/v1/datasource/types' });
      const raw = Array.isArray(value) ? value : row(value, 'data source types').data;
      if (!Array.isArray(raw)) throw new Error('Invalid data source types');
      return raw.map((item) => {
        const connector = row(item, 'data source type');
        for (const key of ['type', 'name', 'description', 'auth_type']) if (typeof connector[key] !== 'string' || !connector[key]) throw new Error(`Invalid data source type field: ${key}`);
        if (typeof connector.priority !== 'number' || !Number.isSafeInteger(connector.priority)) throw new Error('Invalid data source type field: priority');
        if (!Array.isArray(connector.capabilities) || connector.capabilities.some((capability) => typeof capability !== 'string')) throw new Error('Invalid data source type capabilities');
        return connector as unknown as DataSourceConnectorType;
      });
    },
    async list(knowledgeBaseId: string): Promise<DataSource[]> {
      return parseDataSources(await request({ method: 'GET', path: `/api/v1/datasource?kb_id=${encodeURIComponent(knowledgeBaseId)}` }));
    },
    async get(id: string): Promise<DataSource> { return parseDataSource(await request({ method: 'GET', path: path(id) })); },
    async create(input: Partial<DataSource>): Promise<DataSource> { return parseDataSource(await request({ method: 'POST', path: '/api/v1/datasource', body: input })); },
    async update(id: string, input: Partial<DataSource>): Promise<DataSource> { return parseDataSource(await request({ method: 'PUT', path: path(id), body: input })); },
    async putCredentials(id: string, credentials: Record<string, unknown>): Promise<unknown> { return request({ method: 'PUT', path: path(id, '/credentials'), body: { credentials } }); },
    async logs(id: string, limit = 20, offset = 0): Promise<DataSourceSyncLog[]> {
      const value = await request({ method: 'GET', path: path(id, `/logs?limit=${encodeURIComponent(String(limit))}&offset=${encodeURIComponent(String(offset))}`) });
      const raw = Array.isArray(value) ? value : row(value, 'data source logs').data;
      if (!Array.isArray(raw)) throw new Error('Invalid data source logs');
      return raw.map((item) => { const log = row(item, 'data source log'); if (typeof log.id !== 'string' || typeof log.status !== 'string') throw new Error('Invalid data source log fields'); return log as DataSourceSyncLog; });
    },
    async remove(id: string): Promise<void> { await request({ method: 'DELETE', path: path(id) }); },
    async validate(id: string): Promise<unknown> { return request({ method: 'POST', path: path(id, '/validate'), body: {} }); },
    async validateCredentials(type: string, credentials: Record<string, unknown>): Promise<unknown> {
      return request({ method: 'POST', path: '/api/v1/datasource/validate-credentials', body: { type, credentials } });
    },
    async resources(id: string, parentId?: string): Promise<DataSourceResource[]> {
      const query = parentId ? `?parent_id=${encodeURIComponent(parentId)}` : '';
      const result = await request({ method: 'GET', path: path(id, `/resources${query}`) });
      const raw = Array.isArray(result) ? result : row(result, 'data source resources').resources;
      if (!Array.isArray(raw)) throw new Error('Invalid data source resources');
      return raw.map((item) => {
        const resource = row(item, 'data source resource');
        for (const key of ['external_id', 'name', 'type']) if (typeof resource[key] !== 'string') throw new Error(`Invalid resource field: ${key}`);
        return resource as DataSourceResource;
      });
    },
    async resourceAncestors(id: string, resourceIds: string[]): Promise<string[]> {
      const value = await request({ method: 'POST', path: path(id, '/resource-ancestors'), body: { resource_ids: resourceIds } });
      const result = row(value, 'data source resource ancestors');
      if (!Array.isArray(result.ancestors) || result.ancestors.some((item) => typeof item !== 'string')) throw new Error('Invalid data source resource ancestors');
      return result.ancestors as string[];
    },
    async sync(id: string): Promise<unknown> { return request({ method: 'POST', path: path(id, '/sync'), body: {} }); },
    async pause(id: string): Promise<unknown> { return request({ method: 'POST', path: path(id, '/pause'), body: {} }); },
    async resume(id: string): Promise<unknown> { return request({ method: 'POST', path: path(id, '/resume'), body: {} }); },
  };
}
