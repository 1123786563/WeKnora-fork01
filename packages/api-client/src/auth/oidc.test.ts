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

test('native OIDC exchange posts one-time code and verifier without URL credentials', async () => {
  const calls: any[] = [];
  const api = createOIDCApi(async (input) => { calls.push(input); return { success: true, token: 'access' }; });
  const result = await api.exchangeNative({ code: 'c', state: 's', redirect_uri: 'weknora://auth', code_verifier: 'v' });
  assert.equal(result.token, 'access');
  assert.equal(calls[0].method, 'POST');
  assert.equal(calls[0].path, '/api/v1/auth/mobile/exchange');
  assert.deepEqual(calls[0].body, { code: 'c', state: 's', redirect_uri: 'weknora://auth', code_verifier: 'v' });
  assert.equal(calls[0].path.includes('access'), false);
});

test('native OIDC exchange rejects missing values before network', async () => {
  const api = createOIDCApi(async () => ({}));
  await assert.rejects(api.exchangeNative({ code: '', state: 's', redirect_uri: 'weknora://auth', code_verifier: 'v' }), /code is required/);
});
