import type { ClientRequest, WeKnoraClient } from '@weknora/api-client';

export interface CatalogEntry {
  action_id: string; app_id: string; app_version: string; provider: string;
  risk: string; connection_id: string; schema_digest: string;
  input_schema: string | Record<string, unknown>; required_scopes: string[]; published: boolean;
}
export interface ConnectionEntry { id: string; kind: 'personal' | 'space'; state: string; owner_id: string | null; auth_version: number }
export interface AuthorizationAttempt { attempt_id: string; status: string; connection_id: string; expires_at: string }
export interface InstallationEntry { id: string; app_key: string; version: string; state?: string; scopes: string[] }

function record(value: unknown, label: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error('Invalid ' + label + ' response');
  return value as Record<string, unknown>;
}
function text(value: unknown, field: string): string {
  if (typeof value !== 'string') throw new Error('Invalid app connector response (' + field + ')');
  return value;
}
function envelope(value: unknown): unknown {
  const root = record(value, 'app connector');
  if (root.success !== true) {
    const error = root.error && typeof root.error === 'object' ? root.error as Record<string, unknown> : {};
    const failure = new Error(typeof error.message === 'string' ? error.message : 'App connector request failed') as Error & { code?: string };
    failure.code = typeof error.code === 'string' ? error.code : 'APP_CONNECTOR_ERROR';
    throw failure;
  }
  if (!('data' in root)) throw new Error('Invalid app connector response (data)');
  return root.data;
}
function array(value: unknown, label: string): unknown[] { if (!Array.isArray(value)) throw new Error('Invalid ' + label + ' response'); return value; }

function parseCatalog(value: unknown): CatalogEntry {
  const row = record(value, 'catalog');
  const scopes = array(row.required_scopes, 'catalog scopes').map((scope) => text(scope, 'required_scopes'));
  if (typeof row.input_schema !== 'string' && (typeof row.input_schema !== 'object' || row.input_schema === null || Array.isArray(row.input_schema))) throw new Error('Invalid catalog response (input_schema)');
  if (typeof row.published !== 'boolean') throw new Error('Invalid catalog response (published)');
  return { action_id: text(row.action_id, 'action_id'), app_id: text(row.app_id, 'app_id'), app_version: text(row.app_version, 'app_version'), provider: text(row.provider, 'provider'), risk: text(row.risk, 'risk'), connection_id: text(row.connection_id, 'connection_id'), schema_digest: text(row.schema_digest, 'schema_digest'), input_schema: row.input_schema as string | Record<string, unknown>, required_scopes: scopes, published: row.published };
}
function parseConnection(value: unknown): ConnectionEntry {
  const row = record(value, 'connection');
  if (row.kind !== 'personal' && row.kind !== 'space') throw new Error('Invalid connection response (kind)');
  if (row.owner_id !== null && typeof row.owner_id !== 'string') throw new Error('Invalid connection response (owner_id)');
  if (typeof row.auth_version !== 'number' || !Number.isSafeInteger(row.auth_version) || row.auth_version < 0) throw new Error('Invalid connection response (auth_version)');
  return { id: text(row.id, 'id'), kind: row.kind, state: text(row.state, 'state'), owner_id: row.owner_id as string | null, auth_version: row.auth_version };
}
function parseInstallation(value: unknown): InstallationEntry {
  const row = record(value, 'installation');
  const scopes = array(row.scopes, 'installation scopes').map((scope) => text(scope, 'scopes'));
  if (row.state !== undefined && typeof row.state !== 'string') throw new Error('Invalid installation response (state)');
  return { id: text(row.id, 'id'), app_key: text(row.app_key, 'app_key'), version: text(row.version, 'version'), state: row.state as string | undefined, scopes };
}

export function createAppsApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    async listCatalog(signal?: AbortSignal): Promise<CatalogEntry[]> { return array(envelope(await request({ method: 'GET', path: '/api/v1/apps/catalog', signal })), 'catalog').map(parseCatalog); },
    async listInstallations(signal?: AbortSignal): Promise<InstallationEntry[]> { return array(envelope(await request({ method: 'GET', path: '/api/v1/apps/installations', signal })), 'installations').map(parseInstallation); },
    async listConnections(signal?: AbortSignal): Promise<ConnectionEntry[]> { return array(envelope(await request({ method: 'GET', path: '/api/v1/apps/connections', signal })), 'connections').map(parseConnection); },
    async beginAuthorization(connectionId: string, signal?: AbortSignal): Promise<AuthorizationAttempt> {
      const row = record(envelope(await request({ method: 'POST', path: `/api/v1/apps/connections/${encodeURIComponent(connectionId)}/authorization-attempts`, body: {}, signal })), 'authorization');
      return { attempt_id: text(row.attempt_id, 'attempt_id'), status: text(row.status, 'status'), connection_id: connectionId, expires_at: text(row.expires_at, 'expires_at') };
    },
    async getAuthorization(attemptId: string, signal?: AbortSignal): Promise<AuthorizationAttempt> {
      const row = record(envelope(await request({ method: 'GET', path: `/api/v1/apps/authorization-attempts/${encodeURIComponent(attemptId)}`, signal })), 'authorization');
      return { attempt_id: text(row.attempt_id, 'attempt_id'), status: text(row.status, 'status'), connection_id: text(row.connection_id, 'connection_id'), expires_at: text(row.expires_at, 'expires_at') };
    },
    async revokeConnection(connectionId: string, expectedVersion: number, signal?: AbortSignal): Promise<ConnectionEntry> {
      return parseConnection(envelope(await request({ method: 'POST', path: `/api/v1/apps/connections/${encodeURIComponent(connectionId)}/revoke`, body: { expected_version: expectedVersion }, signal })));
    },
  };
}

export function appsApi(client: Pick<WeKnoraClient, 'request'>) { return createAppsApi(client.request); }
