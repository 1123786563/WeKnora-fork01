import assert from 'node:assert/strict';
import test from 'node:test';

import {
  createLegacyPlatformAdapter,
  importLegacyPlatformState,
  REACT_LEGACY_FALLBACK_STORAGE_KEY,
  REACT_LEGACY_IMPORT_MARKER_KEY,
  REACT_SESSION_STORAGE_KEY,
  readReactPlatformState,
} from './legacy-session.ts';

function storage(initial: Record<string, string> = {}, options: { throwOnGet?: boolean } = {}): Storage {
  const values = { ...initial };
  return {
    getItem(key) { if (options.throwOnGet) throw new Error('storage unavailable'); return values[key] ?? null; },
    setItem(key, value) { values[key] = value; },
    removeItem(key) { delete values[key]; },
    clear() { for (const key of Object.keys(values)) delete values[key]; },
    key(index) { return Object.keys(values)[index] ?? null; },
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

test('imports legacy credentials and preferences once into a versioned React record', () => {
  const browserStorage = storage({
    weknora_token: 'old-access',
    weknora_refresh_token: 'old-refresh',
    weknora_selected_tenant_id: 'tenant-7',
    weknora_selected_tenant_name: 'Legacy workspace',
    weknora_lite_mode: 'true',
    locale: 'zh-CN',
  });

  const first = importLegacyPlatformState(browserStorage, () => '2026-09-11T00:00:00.000Z');
  assert.deepEqual(first, {
    credential: { kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' },
    tenantId: 'tenant-7',
    preferences: {
      weknora_selected_tenant_name: 'Legacy workspace',
      weknora_lite_mode: 'true',
      locale: 'zh-CN',
    },
  });
  assert.equal(browserStorage.getItem(REACT_LEGACY_IMPORT_MARKER_KEY), 'complete');
  assert.deepEqual(readReactPlatformState(browserStorage), first);
  assert.deepEqual(JSON.parse(browserStorage.getItem(REACT_LEGACY_FALLBACK_STORAGE_KEY)!), {
    importedAt: '2026-09-11T00:00:00.000Z',
    state: first,
    version: 1,
  });

  browserStorage.setItem('weknora_token', 'changed-after-import');
  browserStorage.setItem('weknora_selected_tenant_id', 'tenant-8');
  assert.deepEqual(importLegacyPlatformState(browserStorage), first);
  assert.equal(browserStorage.getItem(REACT_SESSION_STORAGE_KEY) !== null, true);
});

test('uses the durable fallback record when the canonical React record is damaged', () => {
  const browserStorage = storage({
    weknora_token: 'old-access',
    weknora_selected_tenant_id: 'tenant-7',
  });
  const imported = importLegacyPlatformState(browserStorage, () => '2026-09-11T00:00:00.000Z');
  browserStorage.setItem(REACT_SESSION_STORAGE_KEY, '{not-json');

  assert.deepEqual(importLegacyPlatformState(browserStorage), imported);
});

test('degrades to an anonymous state when browser storage reads are unavailable', () => {
  assert.doesNotThrow(() => importLegacyPlatformState(storage({}, { throwOnGet: true })));
  assert.deepEqual(importLegacyPlatformState(storage({}, { throwOnGet: true })), {
    credential: { kind: 'anonymous' },
    tenantId: null,
    preferences: {},
  });
});
