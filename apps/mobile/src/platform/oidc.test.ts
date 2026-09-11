import assert from 'node:assert/strict';
import test from 'node:test';
import { MOBILE_OIDC_REDIRECT, matchesMobileOIDCState, parseMobileOIDCCallback } from './oidc.ts';

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
