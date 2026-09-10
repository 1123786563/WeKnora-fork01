import assert from 'node:assert/strict';
import test from 'node:test';

import { parseOIDCCallbackHash } from './oidc.ts';

function encode(value: unknown): string {
  const bytes = new TextEncoder().encode(JSON.stringify(value));
  let binary = '';
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary).replaceAll('+', '-').replaceAll('/', '_').replace(/=+$/u, '');
}

test('consumes a successful OIDC callback payload into an auth session', () => {
  assert.deepEqual(parseOIDCCallbackHash(`#oidc_result=${encode({ success: true, token: 'access', refresh_token: 'refresh', user: { id: 'u-1' }, tenant: null })}`), {
    kind: 'success',
    session: { token: 'access', refreshToken: 'refresh', user: { id: 'u-1' }, tenant: null, memberships: undefined },
  });
});

test('preserves provider error details without attempting credential storage', () => {
  assert.deepEqual(parseOIDCCallbackHash('#oidc_error=login_failed&oidc_error_description=Provider%20denied'), {
    kind: 'error',
    code: 'login_failed',
    message: 'Provider denied',
  });
  assert.equal(parseOIDCCallbackHash(''), null);
});

test('rejects malformed or incomplete OIDC callback payloads', () => {
  assert.deepEqual(parseOIDCCallbackHash('#oidc_result=not-base64'), {
    kind: 'error', code: 'invalid_callback', message: 'The OIDC callback payload is invalid.'
  });
  assert.deepEqual(parseOIDCCallbackHash(`#oidc_result=${encode({ success: true, token: 'access' })}`), {
    kind: 'error', code: 'invalid_callback', message: 'The OIDC callback payload is invalid.'
  });
});
