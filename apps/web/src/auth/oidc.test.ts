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
  // AUTH-5 — Vue App.vue 的 catch 直出 error.message（decodeOIDCResult 内部
  // JSON.parse/atob 抛出的原始错误），React 对齐直出原文而非固定英文文案。
  const notBase64 = parseOIDCCallbackHash('#oidc_result=not-base64');
  assert.equal(notBase64?.kind, 'error');
  assert.equal(notBase64?.code, 'invalid_callback');
  assert.ok(
    typeof notBase64?.message === 'string' && notBase64.message.length > 0 && notBase64.message !== 'The OIDC callback payload is invalid.',
    `atob error message must surface verbatim, got: ${notBase64?.message}`,
  );
  assert.deepEqual(parseOIDCCallbackHash(`#oidc_result=${encode({ success: true, token: 'access' })}`), {
    kind: 'error', code: 'invalid_callback', message: 'value must be a non-empty string'
  });
});

test('malformed oidc_result JSON surfaces the raw JSON.parse error like Vue (AUTH-5)', () => {
  // 探针口径：/login#oidc_result=Zm9v（base64 'foo'）→ Vue toast
  // 「Unexpected token 'o', "foo" is not valid JSON」，React 须逐字一致。
  const garbage = parseOIDCCallbackHash('#oidc_result=Zm9v');
  assert.equal(garbage?.kind, 'error');
  assert.equal(garbage?.code, 'invalid_callback');
  assert.match(garbage?.message ?? '', /Unexpected token/);
  assert.match(garbage?.message ?? '', /is not valid JSON/);
});
