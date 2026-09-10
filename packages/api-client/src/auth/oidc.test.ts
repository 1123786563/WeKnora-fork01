import test from 'node:test';
import assert from 'node:assert/strict';
import { createOIDCApi } from './oidc.ts';

test('OIDC start sends redirect URI to the product endpoint', async () => {
  const calls: any[] = [];
  const api = createOIDCApi(async (input) => { calls.push(input); return { success: true, state: 's', authorization_url: 'https://idp.example/auth' }; });
  const result = await api.start('weknora://auth');
  assert.equal(result.state, 's');
  assert.equal(calls[0].method, 'GET');
  assert.match(calls[0].path, /redirect_uri=weknora%3A%2F%2Fauth/);
});

test('native OIDC exchange stays unavailable until one-time-code endpoint exists', async () => {
  const api = createOIDCApi(async () => ({}));
  await assert.rejects(api.exchange('code', 'state', 'weknora://auth'), /OIDC_EXCHANGE_UNAVAILABLE/);
});
