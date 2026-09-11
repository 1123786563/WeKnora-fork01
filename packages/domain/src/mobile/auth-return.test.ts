import test from 'node:test';
import assert from 'node:assert/strict';
import { consumeAuthState, createAuthState, validateAuthReturn } from './auth-return.ts';

test('auth callback cannot redirect to a foreign application', () => {
  assert.throws(() => validateAuthReturn('s', 'evil://auth?state=s', 'weknora-dev://auth'), /AUTH_RETURN/);
  assert.throws(() => validateAuthReturn('s', 'weknora-dev://auth?state=x', 'weknora-dev://auth'), /AUTH_RETURN/);
});

test('auth callback rejects wrong path, protocol and token-bearing redirects', () => {
  assert.throws(() => validateAuthReturn('s', 'weknora-dev://other?state=s', 'weknora-dev://auth'), /AUTH_RETURN/);
  assert.throws(() => validateAuthReturn('s', 'http://auth?state=s', 'weknora-dev://auth'), /AUTH_RETURN/);
  assert.throws(() => validateAuthReturn('s', 'weknora-dev://auth?state=s#access_token=secret', 'weknora-dev://auth'), /AUTH_RETURN/);
});

test('auth state is single use', () => {
  const state = createAuthState(() => 'fixed-state');
  assert.equal(consumeAuthState(state), true);
  assert.equal(consumeAuthState(state), false);
});
