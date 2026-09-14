import assert from 'node:assert/strict';
import test from 'node:test';

import { reloginAfterRefreshFailure, shouldReloginAfterRefreshFailure } from './relogin.ts';

// Vue authRefresh.ts redirectToLogin parity: skip /login and /embed/* pages.
test('relogin decision matches Vue redirectToLogin guards', () => {
  assert.equal(shouldReloginAfterRefreshFailure('/platform/knowledge-bases'), true);
  assert.equal(shouldReloginAfterRefreshFailure('/platform/settings'), true);
  assert.equal(shouldReloginAfterRefreshFailure('/login'), false);
  assert.equal(shouldReloginAfterRefreshFailure('/embed/abc'), false);
});

test('relogin clears the session and navigates to /login', async () => {
  const calls: string[] = [];
  await reloginAfterRefreshFailure({
    clearSession: () => calls.push('clear'),
    pathname: '/platform/knowledge-bases',
    assign: (url) => calls.push('assign:' + url),
  });
  assert.deepEqual(calls, ['clear', 'assign:/login']);
});

test('relogin on /login clears without a redundant navigation', async () => {
  const calls: string[] = [];
  await reloginAfterRefreshFailure({
    clearSession: () => calls.push('clear'),
    pathname: '/login',
    assign: (url) => calls.push('assign:' + url),
  });
  assert.deepEqual(calls, ['clear']);
});

test('relogin on an embed page clears without navigating', async () => {
  const calls: string[] = [];
  await reloginAfterRefreshFailure({
    clearSession: () => calls.push('clear'),
    pathname: '/embed/channel-1',
    assign: (url) => calls.push('assign:' + url),
  });
  assert.deepEqual(calls, ['clear']);
});
