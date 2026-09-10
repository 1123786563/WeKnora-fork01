import assert from 'node:assert/strict';
import test from 'node:test';
import { parseMobileOIDCCallback } from './oidc.ts';

function encode(value: unknown): string {
  const bytes = new TextEncoder().encode(JSON.stringify(value));
  let binary = '';
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary).replaceAll('+', '-').replaceAll('/', '_').replace(/=+$/u, '');
}

test('parses an OIDC result from a native callback URL', () => {
  assert.deepEqual(parseMobileOIDCCallback(`weknora://oidc#oidc_result=${encode({ success: true, token: 'access', refresh_token: 'refresh', tenant: null })}`), {
    kind: 'success',
    session: { token: 'access', refreshToken: 'refresh', tenant: null, user: undefined, memberships: undefined },
    state: undefined,
  });
});

test('preserves provider errors and rejects malformed callback payloads', () => {
  assert.deepEqual(parseMobileOIDCCallback('weknora://oidc#oidc_error=denied&oidc_error_description=No%20access'), {
    kind: 'error', code: 'denied', message: 'No access', state: undefined,
  });
  assert.deepEqual(parseMobileOIDCCallback('weknora://oidc#oidc_result=not-base64'), {
    kind: 'error', code: 'invalid_callback', message: 'The OIDC callback payload is invalid.', state: undefined,
  });
  assert.equal(parseMobileOIDCCallback('weknora://oidc'), null);
});
