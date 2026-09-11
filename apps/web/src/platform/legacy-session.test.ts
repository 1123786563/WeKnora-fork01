import assert from 'node:assert/strict';
import test from 'node:test';

import { createLegacyPlatformAdapter } from './legacy-session.ts';

function storage(values: Record<string, string>): Storage {
  return {
    getItem(key) { return values[key] ?? null; },
    setItem() {},
    removeItem() {},
    clear() {},
    key() { return null; },
    get length() { return Object.keys(values).length; },
  };
}

test('reads legacy bearer and tenant values without treating embed as bearer', () => {
  const adapter = createLegacyPlatformAdapter(storage({
    weknora_token: 'Embed embed-token',
    weknora_selected_tenant_id: 'tenant-7',
  }));

  assert.deepEqual(adapter.read(), {
    credential: { kind: 'embed', token: 'Embed embed-token' },
    tenantId: 'tenant-7',
  });
});

test('imports the legacy refresh token with a bearer session', () => {
  const adapter = createLegacyPlatformAdapter(storage({
    weknora_token: 'access-token',
    weknora_refresh_token: 'refresh-token',
  }));

  assert.deepEqual(adapter.read(), {
    credential: { kind: 'bearer', accessToken: 'access-token', refreshToken: 'refresh-token' },
    tenantId: null,
  });
});
