import assert from 'node:assert/strict';
import test from 'node:test';

import type { Credential, CredentialAdapter } from './ports.ts';
import { AuthError, createRefreshCoordinator } from './auth/refresh-coordinator.ts';

function adapter(initial: Credential): CredentialAdapter & { value: Credential; writes: Credential[]; clears: number } {
  const state = { value: initial, writes: [] as Credential[], clears: 0 };
  return {
    get value() { return state.value; },
    set value(value: Credential) { state.value = value; },
    get writes() { return state.writes; },
    get clears() { return state.clears; },
    async read() { return state.value; },
    async write(value) { state.value = value; state.writes.push(value); },
    async clear() { state.value = { kind: 'anonymous' }; state.clears += 1; },
  };
}

test('refreshes a bearer profile once for concurrent callers and rotates both tokens', async () => {
  const credentials = adapter({ kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' });
  let calls = 0;
  let release!: () => void;
  const refresh = new Promise<void>((resolve) => { release = resolve; });
  const coordinator = createRefreshCoordinator({
    credentials,
    refresh: async (refreshToken) => {
      calls += 1;
      assert.equal(refreshToken, 'old-refresh');
      await refresh;
      return { success: true, access_token: 'new-access', refresh_token: 'new-refresh' };
    },
  });

  const first = coordinator.refresh();
  const second = coordinator.refresh();
  release();

  assert.deepEqual(await Promise.all([first, second]), [
    { kind: 'bearer', accessToken: 'new-access', refreshToken: 'new-refresh' },
    { kind: 'bearer', accessToken: 'new-access', refreshToken: 'new-refresh' },
  ]);
  assert.equal(calls, 1);
  assert.deepEqual(credentials.value, { kind: 'bearer', accessToken: 'new-access', refreshToken: 'new-refresh' });
});

test('refresh failure clears bearer credentials and rejects every waiter', async () => {
  const credentials = adapter({ kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' });
  const failure = new Error('refresh denied');
  const coordinator = createRefreshCoordinator({ credentials, refresh: async () => { throw failure; } });

  const results = await Promise.allSettled([coordinator.refresh(), coordinator.refresh()]);

  assert.equal(credentials.clears, 1);
  assert.deepEqual(results.map((result) => result.status), ['rejected', 'rejected']);
  assert.equal((results[0] as PromiseRejectedResult).reason, failure);
  assert.equal((results[1] as PromiseRejectedResult).reason, failure);
});

test('invalidate advances generation and prevents a late refresh from writing credentials', async () => {
  const credentials = adapter({ kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' });
  let release!: () => void;
  const pending = new Promise<void>((resolve) => { release = resolve; });
  const coordinator = createRefreshCoordinator({
    credentials,
    refresh: async () => {
      await pending;
      return { success: true, access_token: 'late-access', refresh_token: 'late-refresh' };
    },
  });

  const refreshing = coordinator.refresh();
  await coordinator.invalidate();
  release();

  await assert.rejects(refreshing, (error: unknown) => error instanceof AuthError && error.code === 'AUTH_INVALIDATED');
  assert.deepEqual(credentials.value, { kind: 'anonymous' });
  assert.equal(credentials.writes.length, 0);
});

test('embed profiles never invoke refresh and are not cleared', async () => {
  const credentials = adapter({ kind: 'embed', token: 'embed-token', visitorId: 'visitor-1' });
  let calls = 0;
  const coordinator = createRefreshCoordinator({
    credentials,
    refresh: async () => { calls += 1; return { success: true, access_token: 'wrong' }; },
  });

  await assert.rejects(coordinator.refresh(), (error: unknown) => error instanceof AuthError && error.code === 'AUTH_NOT_REFRESHABLE');
  assert.equal(calls, 0);
  assert.equal(credentials.clears, 0);
  assert.deepEqual(credentials.value, { kind: 'embed', token: 'embed-token', visitorId: 'visitor-1' });
});

test('malformed refresh responses fail strictly and clear the bearer profile', async () => {
  const malformed = [
    null,
    { success: false, access_token: 'new-access', refresh_token: 'new-refresh' },
    { success: true, access_token: '', refresh_token: 'new-refresh' },
    { success: true, access_token: '   ', refresh_token: 'new-refresh' },
    { success: true, access_token: 'new-access', refresh_token: 42 },
  ];

  for (const response of malformed) {
    const credentials = adapter({ kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' });
    const coordinator = createRefreshCoordinator({ credentials, refresh: async () => response });
    await assert.rejects(coordinator.refresh(), (error: unknown) => error instanceof AuthError && error.code === 'AUTH_REFRESH_INVALID');
    assert.equal(credentials.clears, 1);
    assert.deepEqual(credentials.value, { kind: 'anonymous' });
  }
});
