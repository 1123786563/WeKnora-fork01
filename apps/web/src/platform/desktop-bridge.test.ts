import assert from 'node:assert/strict';
import test from 'node:test';
import { resolveApiBaseUrl } from './desktop-bridge.ts';

test('prefers the Wails-injected API base over web configuration', async () => {
  const base = await resolveApiBaseUrl({
    injected: 'http://127.0.0.1:43123/api/v1/',
    configured: 'https://configured.example/api/v1',
    origin: 'http://wails.local',
  });

  assert.equal(base, 'http://127.0.0.1:43123/api/v1');
});

test('uses the generated Wails App bridge when injection is not ready', async () => {
  const base = await resolveApiBaseUrl({
    bridge: { GetAPIBaseURL: async () => 'http://127.0.0.1:43124/api/v1/' },
    configured: 'https://configured.example/api/v1',
    origin: 'http://wails.local',
  });

  assert.equal(base, 'http://127.0.0.1:43124/api/v1');
});

test('falls back to configured API base and then the current origin', async () => {
  assert.equal(
    await resolveApiBaseUrl({ configured: 'https://configured.example/api/v1/', origin: 'http://wails.local' }),
    'https://configured.example/api/v1',
  );
  assert.equal(
    await resolveApiBaseUrl({ configured: '', origin: 'http://wails.local/' }),
    'http://wails.local',
  );
});
