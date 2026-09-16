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

test('accepts the normalized token pair returned by auth.refresh', async () => {
  const credentials = adapter({ kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' });
  const coordinator = createRefreshCoordinator({
    credentials,
    refresh: async () => ({ access_token: 'new-access', refresh_token: 'new-refresh' }),
  });

  assert.deepEqual(await coordinator.refresh(), { kind: 'bearer', accessToken: 'new-access', refreshToken: 'new-refresh' });
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

test('can invalidate a late refresh without clearing the current credential', async () => {
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
  await coordinator.invalidate({ clear: false });
  assert.deepEqual(credentials.value, { kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' });
  release();

  await assert.rejects(refreshing, (error: unknown) => error instanceof AuthError && error.code === 'AUTH_INVALIDATED');
  assert.deepEqual(credentials.value, { kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' });
  assert.equal(credentials.writes.length, 0);
});

test('reconciles a refresh write that was already in progress when invalidated', async () => {
  const base = adapter({ kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' });
  let writes = 0;
  let release!: () => void;
  let started!: () => void;
  const writeStarted = new Promise<void>((resolve) => { started = resolve; });
  const writeGate = new Promise<void>((resolve) => { release = resolve; });
  const credentials: CredentialAdapter & { value: Credential } = {
    get value() { return base.value; },
    set value(value: Credential) { base.value = value; },
    async read() { return base.read(); },
    async write(value) {
      writes += 1;
      started();
      if (writes === 1) await writeGate;
      await base.write(value);
    },
    async clear() { await base.clear(); },
  };
  const coordinator = createRefreshCoordinator({ credentials, refresh: async () => ({ success: true, access_token: 'late-access' }) });

  const refreshing = coordinator.refresh();
  await writeStarted;
  await coordinator.invalidate({ clear: false });
  release();

  await assert.rejects(refreshing, (error: unknown) => error instanceof AuthError && error.code === 'AUTH_INVALIDATED');
  assert.deepEqual(credentials.value, { kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' });
  assert.equal(writes, 2);
});

test('does not reuse an invalidated in-flight refresh for a new session', async () => {
  const credentials = adapter({ kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' });
  let calls = 0;
  let releaseFirst!: () => void;
  const firstPending = new Promise<void>((resolve) => { releaseFirst = resolve; });
  const coordinator = createRefreshCoordinator({
    credentials,
    refresh: async () => {
      calls += 1;
      if (calls === 1) await firstPending;
      return { success: true, access_token: `access-${calls}` };
    },
  });

  const first = coordinator.refresh();
  await coordinator.invalidate({ clear: false });
  const second = coordinator.refresh();
  assert.notEqual(first, second);
  assert.deepEqual(await second, { kind: 'bearer', accessToken: 'access-2', refreshToken: 'old-refresh' });
  releaseFirst();
  await assert.rejects(first, (error: unknown) => error instanceof AuthError && error.code === 'AUTH_INVALIDATED');
  assert.equal(calls, 2);
  assert.deepEqual(credentials.value, { kind: 'bearer', accessToken: 'access-2', refreshToken: 'old-refresh' });
});

test('restores a session write that was invalidated after adapter persistence started', async () => {
  const base = adapter({ kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' });
  let release!: () => void;
  let started!: () => void;
  let calls = 0;
  const writeStarted = new Promise<void>((resolve) => { started = resolve; });
  const writeGate = new Promise<void>((resolve) => { release = resolve; });
  const credentials: CredentialAdapter & { value: Credential } = {
    get value() { return base.value; },
    set value(value: Credential) { base.value = value; },
    async read() { return base.read(); },
    async write(value) {
      calls += 1;
      started();
      if (calls === 1) await writeGate;
      await base.write(value);
    },
    async clear() { await base.clear(); },
  };
  const coordinator = createRefreshCoordinator({ credentials, refresh: async () => ({}) });

  const storing = coordinator.write({ kind: 'bearer', accessToken: 'new-access', refreshToken: 'new-refresh' });
  await writeStarted;
  await coordinator.invalidate({ clear: false });
  release();

  await assert.rejects(storing, (error: unknown) => error instanceof AuthError && error.code === 'AUTH_INVALIDATED');
  assert.deepEqual(credentials.value, { kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' });
  assert.equal(calls, 2);
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

test('advanceGeneration does not reuse a refresh from the retired scope', async () => {
  const credentials = adapter({ kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' });
  let release!: () => void;
  const pending = new Promise<void>((resolve) => { release = resolve; });
  let calls = 0;
  const coordinator = createRefreshCoordinator({
    credentials,
    refresh: async (refreshToken) => {
      calls += 1;
      if (calls === 1) await pending;
      return { success: true, access_token: `access-${calls}`, refresh_token: `refresh-${calls}` };
    },
  });
  const oldRefresh = coordinator.refresh();
  await Promise.resolve();
  coordinator.advanceGeneration();
  const newRefresh = coordinator.refresh();
  await Promise.resolve();
  assert.equal(calls, 2);
  release();
  await assert.rejects(oldRefresh, (error: unknown) => error instanceof AuthError && error.code === 'AUTH_INVALIDATED');
  assert.deepEqual(await newRefresh, { kind: 'bearer', accessToken: 'access-2', refreshToken: 'refresh-2' });
});

test('replace advances generation before writing a new account credential', async () => {
  const credentials = adapter({ kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' });
  let release!: () => void;
  const pending = new Promise<void>((resolve) => { release = resolve; });
  const coordinator = createRefreshCoordinator({
    credentials,
    refresh: async () => { await pending; return { success: true, access_token: 'late', refresh_token: 'late' }; },
  });
  const oldRefresh = coordinator.refresh();
  await Promise.resolve();
  await coordinator.replace({ kind: 'bearer', accessToken: 'new-access', refreshToken: 'new-refresh' });
  release();
  await assert.rejects(oldRefresh, (error: unknown) => error instanceof AuthError && error.code === 'AUTH_INVALIDATED');
  assert.deepEqual(credentials.value, { kind: 'bearer', accessToken: 'new-access', refreshToken: 'new-refresh' });
});

test('serialized credential writes leave the replacement account as the final value', async () => {
  let release!: () => void;
  const blocked = new Promise<void>((resolve) => { release = resolve; });
  let firstWrite = true;
  const writes: Credential[] = [];
  const credentials: CredentialAdapter & { value: Credential } = {
    value: { kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' },
    async read() { return this.value; },
    async write(value) {
      writes.push(value);
      this.value = value;
      if (firstWrite) { firstWrite = false; await blocked; }
    },
    async clear() { this.value = { kind: 'anonymous' }; },
  };
  const coordinator = createRefreshCoordinator({
    credentials,
    refresh: async () => ({ success: true, access_token: 'late-access', refresh_token: 'late-refresh' }),
  });
  const refreshing = coordinator.refresh();
  while (writes.length === 0) await Promise.resolve();
  const replacing = coordinator.replace({ kind: 'bearer', accessToken: 'new-access', refreshToken: 'new-refresh' });
  release();
  await assert.rejects(refreshing, (error: unknown) => error instanceof AuthError && error.code === 'AUTH_INVALIDATED');
  await replacing;
  assert.deepEqual(credentials.value, { kind: 'bearer', accessToken: 'new-access', refreshToken: 'new-refresh' });
  assert.deepEqual(writes, [
    { kind: 'bearer', accessToken: 'late-access', refreshToken: 'late-refresh' },
    { kind: 'bearer', accessToken: 'new-access', refreshToken: 'new-refresh' },
  ]);
});

test('a delayed refresh clear cannot remove a replacement account credential', async () => {
  let releaseClear!: () => void;
  const clearGate = new Promise<void>((resolve) => { releaseClear = resolve; });
  let clearStarted = false;
  const credentials = adapter({ kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' });
  const clear = credentials.clear;
  credentials.clear = async () => { clearStarted = true; await clearGate; await clear(); };
  const coordinator = createRefreshCoordinator({
    credentials,
    refresh: async () => { throw new Error('refresh denied'); },
  });
  const failedRefresh = coordinator.refresh();
  await Promise.resolve();
  while (!clearStarted) await Promise.resolve();
  const replacing = coordinator.replace({ kind: 'bearer', accessToken: 'new-access', refreshToken: 'new-refresh' });
  releaseClear();
  await assert.rejects(failedRefresh, (error: unknown) => error instanceof AuthError && error.code === 'AUTH_INVALIDATED');
  await replacing;
  assert.deepEqual(credentials.value, { kind: 'bearer', accessToken: 'new-access', refreshToken: 'new-refresh' });
});
