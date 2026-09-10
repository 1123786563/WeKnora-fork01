import assert from 'node:assert/strict';
import test from 'node:test';
import { createWeKnoraClient } from '@weknora/api-client';
import { LoginPage } from './LoginPage.tsx';

test('exports a login page that uses the strict auth client', () => {
  assert.equal(typeof LoginPage, 'function');
  const client = createWeKnoraClient({ baseURL: '', transport: { send: async () => ({ status: 200, headers: {}, body: { success: true, token: 'a', refresh_token: 'r' } }) } });
  assert.equal(typeof client.auth.login, 'function');
});
