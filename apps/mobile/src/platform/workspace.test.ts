import assert from 'node:assert/strict';
import test from 'node:test';
import { createLatestAsyncWriter, createSessionEpoch, createSingleFlight, createWorkspaceSelectionAdapter, parseMobileWorkspaces, resetMobileSessionState, shouldHydrateWorkspaceMemberships, shouldLoadWorkspaceRoute, shouldRefreshMobileSession, toWorkspaceId } from './workspace.ts';

function store(initial: string | null = null) {
  let value = initial;
  return {
    async getItemAsync() { return value; },
    async setItemAsync(_key: string, next: string) { value = next; },
    async deleteItemAsync() { value = null; },
  };
}

test('normalizes server memberships without inventing malformed workspaces', () => {
  assert.deepEqual(parseMobileWorkspaces([
    { tenant_id: 7, tenant_name: 'Docs', role: 'admin' },
    { tenant_id: '8', tenant_name: 'Research', role: 'viewer' },
    { tenant_id: 0, tenant_name: 'invalid', role: 'admin' },
    { tenant_id: 'not-a-number', tenant_name: 'invalid', role: 'admin' },
  ]), [
    { id: 7, name: 'Docs', role: 'admin' },
    { id: 8, name: 'Research', role: 'viewer' },
  ]);
});

test('workspace switching accepts only positive safe integer ids', () => {
  assert.equal(toWorkspaceId(7), 7);
  assert.equal(toWorkspaceId('8'), 8);
  assert.equal(toWorkspaceId(''), null);
  assert.equal(toWorkspaceId('1.5'), null);
  assert.equal(toWorkspaceId(Number.MAX_SAFE_INTEGER + 1), null);
});

test('persists and clears the selected workspace id', async () => {
  const adapter = createWorkspaceSelectionAdapter(store('7'));
  assert.equal(await adapter.read(), 7);
  await adapter.write(8);
  assert.equal(await adapter.read(), 8);
  await adapter.write(null);
  assert.equal(await adapter.read(), null);
});

test('hydrates memberships only after a bearer session is restored without cached workspaces', () => {
  assert.equal(shouldHydrateWorkspaceMemberships({ hydrating: true, credentialKind: 'bearer', workspaceCount: 0 }), false);
  assert.equal(shouldHydrateWorkspaceMemberships({ hydrating: false, credentialKind: 'anonymous', workspaceCount: 0 }), false);
  assert.equal(shouldHydrateWorkspaceMemberships({ hydrating: false, credentialKind: 'bearer', workspaceCount: 1 }), false);
  assert.equal(shouldHydrateWorkspaceMemberships({ hydrating: false, credentialKind: 'bearer', workspaceCount: 0 }), true);
});

test('does not start a mobile refresh while a session transition is active', () => {
  assert.equal(shouldRefreshMobileSession({ transitionCount: 1, credentialKind: 'bearer', hasRefreshToken: true }), false);
  assert.equal(shouldRefreshMobileSession({ transitionCount: 0, credentialKind: 'bearer', hasRefreshToken: true }), true);
  assert.equal(shouldRefreshMobileSession({ transitionCount: 0, credentialKind: 'anonymous', hasRefreshToken: false }), false);
});

test('refresh failure reset clears the complete authenticated scope', () => {
  const events: unknown[] = [];
  resetMobileSessionState({
    updateCredential: (value) => events.push(['credential', value]),
    updateUserId: (value) => events.push(['user', value]),
    updateTenantId: (value) => events.push(['tenant', value]),
    setWorkspaces: (value) => events.push(['workspaces', value]),
    setCanCreateTenant: (value) => events.push(['canCreateTenant', value]),
  });
  assert.deepEqual(events, [
    ['credential', { kind: 'anonymous' }],
    ['user', null],
    ['tenant', null],
    ['workspaces', []],
    ['canCreateTenant', false],
  ]);
});

test('shares one in-flight workspace refresh instead of issuing concurrent auth requests', async () => {
  let calls = 0;
  let resolve!: (value: string) => void;
  const refresh = createSingleFlight(() => {
    calls += 1;
    return new Promise<string>((complete) => { resolve = complete; });
  });

  const first = refresh();
  const second = refresh();
  assert.strictEqual(first, second);
  assert.equal(calls, 1);
  resolve('loaded');
  assert.equal(await first, 'loaded');
  const third = refresh();
  assert.equal(calls, 2);
  resolve('loaded');
  assert.equal(await third, 'loaded');
  assert.equal(calls, 2);
});

test('does not load the workspace route until a restored bearer session is ready', () => {
  assert.equal(shouldLoadWorkspaceRoute({ hydrating: true, credentialKind: 'anonymous' }), false);
  assert.equal(shouldLoadWorkspaceRoute({ hydrating: false, credentialKind: 'anonymous' }), false);
  assert.equal(shouldLoadWorkspaceRoute({ hydrating: false, credentialKind: 'embed' }), false);
  assert.equal(shouldLoadWorkspaceRoute({ hydrating: false, credentialKind: 'bearer' }), true);
});

test('invalidates a workspace refresh when the session changes', () => {
  const epoch = createSessionEpoch();
  const started = epoch.current();
  assert.equal(epoch.isCurrent(started), true);
  epoch.invalidate();
  assert.equal(epoch.isCurrent(started), false);
  const next = epoch.current();
  assert.equal(epoch.isCurrent(next), true);
});

test('serializes workspace writes with newest selection last', async () => {
  const values: number[] = [];
  let release!: () => void;
  let started!: () => void;
  const writeStarted = new Promise<void>((resolve) => { started = resolve; });
  const gate = new Promise<void>((resolve) => { release = resolve; });
  const writer = createLatestAsyncWriter(async (value: number) => {
    if (value === 1) { started(); await gate; }
    values.push(value);
  });

  const first = writer.write(1);
  await writeStarted;
  const second = writer.write(2);
  release();
  await Promise.all([first, second]);
  assert.deepEqual(values, [1, 2]);
});
