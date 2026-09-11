import {
  parseConnectionView,
  parseInstallationView,
  parseSyncStatusView,
  type ConnectionView,
  type CreateConnectionInput,
  type CreateInstallationInput,
  type InstallationView,
  type SyncStatusView,
  type UpgradeInstallationInput,
} from '@weknora/contracts';
import type { ClientRequest } from './client.ts';
import { ApiError } from './errors.ts';

function unwrap(value: unknown): unknown {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ApiError({ code: 'INVALID_RESPONSE', message: 'Expected an app-connector response envelope' });
  }
  const envelope = value as Record<string, unknown>;
  if (envelope.success !== true) {
    const error = typeof envelope.error === 'object' && envelope.error !== null
      ? envelope.error as Record<string, unknown>
      : {};
    throw new ApiError({
      code: typeof error.code === 'string' && error.code !== '' ? error.code : 'APP_CONNECTOR_ERROR',
      message: typeof error.message === 'string' && error.message !== '' ? error.message : 'App connector request failed',
      requestId: typeof error.requestId === 'string' ? error.requestId : undefined,
    });
  }
  if (envelope.data === undefined) {
    throw new ApiError({ code: 'INVALID_RESPONSE', message: 'App connector response envelope is missing data' });
  }
  return envelope.data;
}

export function createAppConnectorApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    async listInstallations(signal?: AbortSignal): Promise<InstallationView[]> {
      const data = unwrap(await request({ method: 'GET', path: '/api/v1/apps/installations', signal }));
      if (!Array.isArray(data)) {
        throw new ApiError({ code: 'INVALID_RESPONSE', message: 'Expected an installations array' });
      }
      return data.map(parseInstallationView);
    },
    async createInstallation(input: CreateInstallationInput, signal?: AbortSignal): Promise<InstallationView> {
      return parseInstallationView(unwrap(await request({
        method: 'POST',
        path: '/api/v1/apps/installations',
        body: { app_key: input.app_key, version: input.version, expected_version: input.expected_version },
        signal,
      })));
    },
    async upgradeInstallation(id: string, input: UpgradeInstallationInput, signal?: AbortSignal): Promise<InstallationView> {
      return parseInstallationView(unwrap(await request({
        method: 'POST',
        path: `/api/v1/apps/installations/${encodeURIComponent(id)}/upgrade`,
        body: { version: input.version, expected_version: input.expected_version },
        signal,
      })));
    },
    async disableInstallation(id: string, expectedVersion: number, signal?: AbortSignal): Promise<InstallationView> {
      return parseInstallationView(unwrap(await request({
        method: 'POST',
        path: `/api/v1/apps/installations/${encodeURIComponent(id)}/disable`,
        body: { expected_version: expectedVersion },
        signal,
      })));
    },
    async listConnections(signal?: AbortSignal): Promise<ConnectionView[]> {
      const data = unwrap(await request({ method: 'GET', path: '/api/v1/apps/connections', signal }));
      if (!Array.isArray(data)) {
        throw new ApiError({ code: 'INVALID_RESPONSE', message: 'Expected a connections array' });
      }
      return data.map(parseConnectionView);
    },
    /**
     * Creating a connection starts the A02 OAuth authorize flow: the response
     * carries the connection projection and the authorize URL — NEVER any
     * access_token/refresh_token/credential payload.
     */
    async createConnection(input: CreateConnectionInput, signal?: AbortSignal): Promise<{ connection: ConnectionView; authorize_url: string }> {
      const data = unwrap(await request({
        method: 'POST',
        path: '/api/v1/apps/connections',
        body: { installation_id: input.installation_id, kind: input.kind, expected_version: input.expected_version },
        signal,
      }));
      if (typeof data !== 'object' || data === null || Array.isArray(data)) {
        throw new ApiError({ code: 'INVALID_RESPONSE', message: 'Expected a connection response object' });
      }
      const row = data as Record<string, unknown>;
      const authorize_url = row.authorize_url;
      if (typeof authorize_url !== 'string' || authorize_url === '') {
        throw new ApiError({ code: 'INVALID_RESPONSE', message: 'Invalid connection response (authorize_url)' });
      }
      return { connection: parseConnectionView(row.connection), authorize_url };
    },
    async revokeConnection(id: string, expectedVersion: number, signal?: AbortSignal): Promise<ConnectionView> {
      return parseConnectionView(unwrap(await request({
        method: 'POST',
        path: `/api/v1/apps/connections/${encodeURIComponent(id)}/revoke`,
        body: { expected_version: expectedVersion },
        signal,
      })));
    },
    async getSyncStatus(datasourceId: string, signal?: AbortSignal): Promise<SyncStatusView> {
      return parseSyncStatusView(unwrap(await request({
        method: 'GET',
        path: `/api/v1/apps/datasources/${encodeURIComponent(datasourceId)}/sync-status`,
        signal,
      })));
    },
  };
}
