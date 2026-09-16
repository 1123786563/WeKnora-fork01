import assert from 'node:assert/strict';
import test from 'node:test';
import { createMobileOIDCPKCE, MOBILE_OIDC_REDIRECT, matchesMobileOIDCState, parseMobileOIDCCallback } from './oidc.ts';

test('parses only a one-time OIDC code and never a bearer payload', () => {
  assert.deepEqual(parseMobileOIDCCallback(`${MOBILE_OIDC_REDIRECT}?oidc_code=short-code&state=mobile-state`), {
    kind: 'code', code: 'short-code', state: 'mobile-state',
  });
  const legacy = parseMobileOIDCCallback(`${MOBILE_OIDC_REDIRECT}?oidc_result=secret-token&state=mobile-state`);
  assert.deepEqual(legacy, { kind: 'error', code: 'invalid_callback', message: 'The OIDC callback payload is invalid.', state: 'mobile-state' });
});

test('preserves provider cancellation and rejects malformed or foreign callback URLs', () => {
  assert.deepEqual(parseMobileOIDCCallback(`${MOBILE_OIDC_REDIRECT}?oidc_error=access_denied&oidc_error_description=Cancelled&state=mobile-state`), {
    kind: 'error', code: 'access_denied', message: 'Cancelled', state: 'mobile-state',
  });
  assert.deepEqual(parseMobileOIDCCallback(`${MOBILE_OIDC_REDIRECT}?oidc_code=`), {
    kind: 'error', code: 'invalid_callback', message: 'The OIDC callback payload is invalid.', state: undefined,
  });
  assert.equal(parseMobileOIDCCallback('evil://oidc?oidc_code=code&state=mobile-state'), null);
});

test('requires the callback state to match the pending device state', () => {
  const callback = parseMobileOIDCCallback(`${MOBILE_OIDC_REDIRECT}?oidc_code=code&state=mobile-state`);
  assert.ok(callback);
  assert.equal(matchesMobileOIDCState(callback, 'mobile-state'), true);
  assert.equal(matchesMobileOIDCState(callback, 'other-state'), false);
  assert.equal(matchesMobileOIDCState(callback, null), false);
});

test('creates a standards-compliant S256 PKCE verifier and challenge', async () => {
  let algorithm = '';
  let encoding = '';
  const pkce = await createMobileOIDCPKCE({
    async getRandomBytesAsync() { return Uint8Array.from({ length: 32 }, (_, index) => index); },
    async digestStringAsync(nextAlgorithm, _data, options) {
      algorithm = nextAlgorithm;
      encoding = options.encoding;
      return 'E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM'.replace(/-/g, '+').replace(/_/g, '/') + '==';
    },
  });
  assert.match(pkce.verifier, /^[A-Za-z0-9._~-]{43,128}$/);
  assert.match(pkce.challenge, /^[A-Za-z0-9_-]{43}$/);
  assert.equal(algorithm, 'SHA-256');
  assert.equal(encoding, 'base64');
});
