import { describe, expect, it, vi } from 'vitest';
import {
  flushPendingRevocations,
  registerAfterPermission,
  registerDevice,
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
    expect(JSON.parse(fetcher.mock.calls[0][1].body as string)).not.toHaveProperty('owner_id');
  });

  it('keeps a minimal pending revocation on offline logout and flushes it later', async () => {
    let rows: PendingRevocation[] = [];
    const pending = { read: async () => rows, write: async (next: PendingRevocation[]) => { rows = next; } };
    const offline = vi.fn(async () => response(503));
    await revokeOnLogout({ origin: 'https://api.example.test', deviceId: 'd', revision: 3, credential, pending, fetchImpl: offline });
    expect(rows).toHaveLength(1);
    const online = vi.fn(async () => response(204, null));
    await expect(flushPendingRevocations(pending, online)).resolves.toBe(1);
    expect(rows).toEqual([]);
  });
});
