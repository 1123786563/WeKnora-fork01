import type { Credential } from '@weknora/api-client';

export type DevicePlatform = 'ios' | 'android';

export interface DeviceRegistrationInput {
  origin: string;
  deviceId: string;
  platform: DevicePlatform;
  token: string;
  scopeGeneration: number;
  revision?: number;
  spaceId?: string;
  credential: Credential;
  signal?: AbortSignal;
  fetchImpl?: typeof fetch;
}

export interface PendingRevocation {
  origin: string;
  deviceId: string;
  revision?: number;
}

export interface PendingRevocationStore {
  read(): Promise<PendingRevocation[]>;
  write(rows: PendingRevocation[]): Promise<void>;
}

function accessToken(credential: Credential): string {
  if (credential.kind !== 'bearer' || !credential.accessToken.trim()) throw new Error('DEVICE_AUTH_REQUIRED');
  return credential.accessToken;
}

function endpoint(origin: string, deviceId: string): string {
  if (!origin.trim() || !deviceId.trim()) throw new Error('DEVICE_SCOPE_REQUIRED');
  return `${origin.replace(/\/+$/, '')}/api/v1/mobile/devices/${encodeURIComponent(deviceId)}`;
}

function assertScopeGeneration(value: number): void {
  if (!Number.isSafeInteger(value) || value < 0) throw new Error('INVALID_SCOPE_GENERATION');
}

async function responseError(response: Response): Promise<Error> {
  if (response.status === 401) return new Error('DEVICE_AUTH_REQUIRED');
  if (response.status === 404) return new Error('DEVICE_NOT_FOUND');
  if (response.status === 409) return new Error('DEVICE_REVISION_CONFLICT');
  return new Error(`DEVICE_REQUEST_FAILED:${response.status}`);
}

export async function registerDevice(input: DeviceRegistrationInput): Promise<{ revision?: number }> {
  const fetcher = input.fetchImpl ?? fetch;
  const token = accessToken(input.credential);
  assertScopeGeneration(input.scopeGeneration);
  if (!input.token.trim() || input.token.length > 4096) throw new Error('INVALID_DEVICE_TOKEN');
  const response = await fetcher(endpoint(input.origin, input.deviceId), {
    method: 'PUT',
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({
      token: input.token,
      platform: input.platform,
      scope_generation: input.scopeGeneration,
      ...(input.revision === undefined ? {} : { revision: input.revision }),
      ...(input.spaceId ? { space_id: input.spaceId } : {}),
    }),
    signal: input.signal,
  });
  if (!response.ok) throw await responseError(response);
  const payload = (await response.json()) as { success?: unknown; data?: { revision?: unknown } };
  if (payload.success !== true || (payload.data?.revision !== undefined && typeof payload.data.revision !== 'number')) {
    throw new Error('INVALID_DEVICE_RESPONSE');
  }
  return { revision: payload.data?.revision as number | undefined };
}

export async function revokeDevice(input: Omit<DeviceRegistrationInput, 'token' | 'platform' | 'scopeGeneration'>): Promise<void> {
  const fetcher = input.fetchImpl ?? fetch;
  const token = accessToken(input.credential);
  const query = input.revision === undefined ? '' : `?revision=${encodeURIComponent(String(input.revision))}`;
  const response = await fetcher(`${endpoint(input.origin, input.deviceId)}${query}`, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${token}` },
    signal: input.signal,
  });
  if (!response.ok && response.status !== 404) throw await responseError(response);
}

/** Permission denial is a valid product state: chat remains usable. */
export async function registerAfterPermission(input: DeviceRegistrationInput & { permissionGranted: boolean }): Promise<{ registered: boolean; revision?: number }> {
  if (!input.permissionGranted) return { registered: false };
  return { registered: true, ...(await registerDevice(input)) };
}

/** Logout first revokes remotely; offline logout leaves only a minimal retry reference. */
export async function revokeOnLogout(
  input: Omit<DeviceRegistrationInput, 'token' | 'platform' | 'scopeGeneration'> & { pending: PendingRevocationStore },
): Promise<void> {
  try {
    await revokeDevice(input);
  } catch {
    const rows = await input.pending.read();
    const duplicate = rows.some((row) => row.origin === input.origin && row.deviceId === input.deviceId && row.revision === input.revision);
    if (!duplicate) await input.pending.write([...rows, { origin: input.origin, deviceId: input.deviceId, revision: input.revision }]);
  }
}

/**
 * A queued intent is deliberately not self-authenticating. Flush requires a
 * fresh, in-memory product credential obtained after the next login; passing
 * only fetch is rejected by retaining the queue instead of persisting a
 * bearer token alongside it.
 */
export async function flushPendingRevocations(store: PendingRevocationStore, credential: Credential, fetchImpl?: typeof fetch): Promise<number> {
  const rows = await store.read();
  if (credential.kind !== 'bearer' || !credential.accessToken.trim()) return 0;
  const remaining: PendingRevocation[] = [];
  for (const row of rows) {
    try {
      await revokeDevice({ ...row, credential, fetchImpl });
    } catch {
      remaining.push(row);
    }
  }
  await store.write(remaining);
  return rows.length - remaining.length;
}

export interface DevicePresence {
  deviceId: string;
  environment: string;
  platform: DevicePlatform;
  scopeGeneration: number;
  revision: number;
  lastSeenAt: string | null;
}

function parsePresence(payload: unknown): DevicePresence {
  const data = (payload as { success?: unknown; data?: Record<string, unknown> })?.data;
  if (!data || (payload as { success?: unknown }).success !== true || typeof data.device_id !== 'string' ||
    (data.platform !== 'ios' && data.platform !== 'android') || typeof data.revision !== 'number' ||
    typeof data.scope_generation !== 'number') throw new Error('INVALID_DEVICE_RESPONSE');
  return { deviceId: data.device_id, environment: String(data.environment ?? ''), platform: data.platform,
    scopeGeneration: data.scope_generation, revision: data.revision,
    lastSeenAt: typeof data.last_seen_at === 'string' ? data.last_seen_at : null };
}

export async function getDevicePresence(input: Omit<DeviceRegistrationInput, 'token' | 'platform' | 'scopeGeneration'>): Promise<DevicePresence> {
  const token = accessToken(input.credential); const response = await (input.fetchImpl ?? fetch)(`${endpoint(input.origin, input.deviceId)}/presence${input.revision === undefined ? '' : `?revision=${input.revision}`}`, { method: 'GET', headers: { Authorization: `Bearer ${token}` }, signal: input.signal });
  if (!response.ok) throw await responseError(response); return parsePresence(await response.json());
}

export async function putDevicePresence(input: Omit<DeviceRegistrationInput, 'token' | 'platform' | 'scopeGeneration'>): Promise<DevicePresence> {
  const token = accessToken(input.credential); const response = await (input.fetchImpl ?? fetch)(`${endpoint(input.origin, input.deviceId)}/presence${input.revision === undefined ? '' : `?revision=${input.revision}`}`, { method: 'PUT', headers: { Authorization: `Bearer ${token}` }, signal: input.signal });
  if (!response.ok) throw await responseError(response); return parsePresence(await response.json());
}

export async function deleteDevicePresence(input: Omit<DeviceRegistrationInput, 'token' | 'platform' | 'scopeGeneration'>): Promise<void> {
  const token = accessToken(input.credential); const response = await (input.fetchImpl ?? fetch)(`${endpoint(input.origin, input.deviceId)}/presence${input.revision === undefined ? '' : `?revision=${input.revision}`}`, { method: 'DELETE', headers: { Authorization: `Bearer ${token}` }, signal: input.signal });
  if (!response.ok && response.status !== 404) throw await responseError(response);
}
