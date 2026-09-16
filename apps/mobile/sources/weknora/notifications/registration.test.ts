import { describe, expect, it, vi } from 'vitest';
import {
  flushPendingRevocations,
  registerAfterPermission,
  registerDevice,
  revokeDevice,
  revokeOnLogout,
  type PendingRevocation,
} from './registration';

const credential = { kind: 'bearer' as const, accessToken: 'access-token' };

function response(status: number, body: unknown = { success: true, data: { revision: 2 } }): Response {
  return new Response(status === 204 ? null : JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } });
}

describe('mobile device registration', () => {
  it('does not call the server when native notification permission is denied', async () => {
    const fetcher = vi.fn();
    const result = await registerAfterPermission({
      permissionGranted: false,
      origin: 'https://api.example.test', deviceId: 'device-a', platform: 'ios', token: 'push',
      scopeGeneration: 4, credential, fetchImpl: fetcher,
    });
    expect(result).toEqual({ registered: false });
    expect(fetcher).not.toHaveBeenCalled();
  });

  it('sends the trusted scope generation and never sends owner or environment', async () => {
    const fetcher = vi.fn(async () => response(200));
    const result = await registerDevice({
      origin: 'https://api.example.test/', deviceId: 'device/a', platform: 'android', token: 'push',
      scopeGeneration: 7, revision: 1, credential, fetchImpl: fetcher,
    });
    expect(result).toEqual({ revision: 2 });
    expect(fetcher).toHaveBeenCalledWith('https://api.example.test/api/v1/mobile/devices/device%2Fa', expect.objectContaining({
      method: 'PUT',
      body: JSON.stringify({ token: 'push', platform: 'android', scope_generation: 7, revision: 1 }),
    }));
    const call = (fetcher.mock.calls as unknown[][])[0];
    const request = call?.[1] as RequestInit | undefined;
    expect(JSON.parse(String(request?.body))).not.toHaveProperty('owner_id');
  });

  it('keeps a minimal pending revocation on offline logout and flushes it later', async () => {
    let rows: PendingRevocation[] = [];
    const pending = { read: async () => rows, write: async (next: PendingRevocation[]) => { rows = next; } };
    const offline = vi.fn(async () => response(503));
    await revokeOnLogout({ origin: 'https://api.example.test', deviceId: 'd', revision: 3, tenantId: 'tenant-a', ownerId: 'user-a', credential, pending, fetchImpl: offline });
    expect(rows).toHaveLength(1);
    expect(rows[0]).not.toHaveProperty('credential');
    const online = vi.fn(async () => response(204, null));
    await expect(flushPendingRevocations(pending, credential, { tenantId: 'tenant-a', ownerId: 'user-a' }, online)).resolves.toBe(1);
    expect(rows).toEqual([]);
  });

  it('keeps another tenant and owner pending revoke for that identity', async () => {
    let rows: PendingRevocation[] = [{ origin: 'https://api.example.test', deviceId: 'd', tenantId: 'tenant-a', ownerId: 'user-a', revision: 3 }];
    const pending = { read: async () => rows, write: async (next: PendingRevocation[]) => { rows = next; } };
    const fetcher = vi.fn(async () => response(204, null));
    await expect(flushPendingRevocations(pending, credential, { tenantId: 'tenant-b', ownerId: 'user-b' }, fetcher)).resolves.toBe(0);
    expect(fetcher).not.toHaveBeenCalled();
    expect(rows).toHaveLength(1);
  });

  it('returns the server scope epoch from a successful revoke response', async () => {
    const fetcher = vi.fn(async () => new Response(null, { status: 204, headers: { 'X-Mobile-Scope-Generation': '9' } }));
    await expect(revokeDevice({ origin: 'https://api.example.test', deviceId: 'd', revision: 3, credential, fetchImpl: fetcher })).resolves.toEqual({ scopeGeneration: 9 });
  });
});
