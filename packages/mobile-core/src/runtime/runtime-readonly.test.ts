import test from 'node:test';
import assert from 'node:assert/strict';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createMobileRuntime } from './mobile-runtime.ts';
import type { CredentialStore, PendingOidc, PendingOidcStore, RuntimeRemote, StoredCredential } from './ports.ts';
import type { ResourceRemote } from '../shelf/ports.ts';
import type { DeploymentInput } from './types.ts';

const DEPLOYMENT: DeploymentInput = { origin: 'https://weknora.example.test', label: 'Test Deployment' };
/** 部署落后：窗口上界低于客户端代际 → clientGate 判 'server_upgrade_required'。 */
const BEHIND_WINDOW = { protocol_minimum: 2, protocol_maximum: CLIENT_PROTOCOL_VERSION - 1 };
/** 客户端落后：窗口下界高于客户端代际 → clientGate 判 'upgrade_required'（说明面，行为不变）。 */
const AHEAD_WINDOW = { protocol_minimum: CLIENT_PROTOCOL_VERSION + 1, protocol_maximum: CLIENT_PROTOCOL_VERSION + 1 };

/** 假凭据夹具（非真实凭据）：refresh 单飞轮换返回的下一对 token，取值即简报逐字值，仅服务内存 fake remote，对任何真实部署不可用。 */
function refreshedAccess(): string { return 'access-1'; }
function refreshedRefresh(): string { return 'refresh-1'; }

function fakeStore(initial: Record<string, StoredCredential | undefined> = {}): CredentialStore {
  const values = new Map(Object.entries(initial));
  return {
    async read(deployment) { return values.get(deployment); },
    async write(deployment, credential) { values.set(deployment, { ...credential }); },
    async clear(deployment) { values.delete(deployment); },
  };
}

function remote(overrides: Partial<RuntimeRemote> = {}): RuntimeRemote {
  return {
    passwordLogin: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    me: async () => ({ user: { id: 'user-1' }, tenant: { id: 'tenant-1' } }),
    deploymentCapabilities: async () => BEHIND_WINDOW,
    oidcUrl: async () => ({ authorizationUrl: 'https://idp.example.test/authorize', state: 'state-1' }),
    oidcExchange: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    oidcNativeExchange: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    refresh: async () => ({ access_token: refreshedAccess(), refresh_token: refreshedRefresh() }),
    switchTenant: async (input) => ({ credential: { token: `tenant-${input.tenantId}-access`, refreshToken: `refresh-${input.tenantId}` }, tenant: { id: input.tenantId } }),
    ...overrides,
  };
}

function tenantResourceRemote(): ResourceRemote & { calls: Array<{ kind: string; token: string }> } {
  const calls: Array<{ kind: string; token: string }> = [];
  return {
    calls,
    async availableAgents(token) {
      calls.push({ kind: 'agents', token });
      return { rows: [{ id: `agent-${token}`, name: `Agent ${token}`, summary: '', kind: 'general', capability: { state: 'supported', reason: '' } }], disabledOwnAgentIds: new Set<string>() };
    },
    async knowledgeBases(token) { calls.push({ kind: 'knowledgeBases', token }); return []; },
    async connections(token) { calls.push({ kind: 'connections', token }); return []; },
  };
}

function pendingStore(): PendingOidcStore {
  let value: PendingOidc | undefined;
  return {
    async savePending(input) { value = { ...input }; },
    async loadPending() { return value && { ...value }; },
    async consumePending() { const claimed = value; value = undefined; return claimed && { ...claimed }; },
    async clearPending() { value = undefined; },
  };
}

test('a deployment behind the client generation degrades to a read-only surface with identity, lease and shelf', async () => {
  const resource = tenantResourceRemote();
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(), remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    resourceShelf: { remoteFor: () => resource },
  });

  const snapshot = await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });

  assert.deepEqual(snapshot, {
    surface: 'read-only',
    deployment: { origin: DEPLOYMENT.origin, label: DEPLOYMENT.label },
    identity: { userId: 'user-1', activeTenantId: 'tenant-1', tenants: [{ id: 'tenant-1' }] },
    reason: 'protocol-mismatch',
  });
  assert.ok(runtime.scopeLease(), 'the read-only scope still mints a revocable lease');
  const page = await runtime.resourceShelf()!.browse();
  assert.equal(page.tenantId, 'tenant-1');
  assert.deepEqual(page.agents.map((agent) => agent.id), ['agent-access-1']);
});

test('the read-only surface fails closed on the authorized channel and tenant switches', async () => {
  let switches = 0;
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => remote({ switchTenant: async () => { switches += 1; return { credential: { token: 'x', refreshToken: 'y' }, tenant: { id: 't' } }; } }),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    authorizedTransport: () => async () => ({ success: true, data: {} }),
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const before = runtime.snapshot();
  assert.equal(before.surface, 'read-only');

  await assert.rejects(runtime.authorizedRequest({ method: 'GET', path: '/api/v1/workbench/overview' }), /RUNTIME_UNAUTHORIZED/);
  const after = await runtime.activateTenant('2');

  assert.equal(switches, 0, 'a degraded deployment must not receive tenant switch requests');
  assert.equal(after, before);
});

test('an app older than the deployment window keeps the explanation-only upgrade surface', async () => {
  const resource = tenantResourceRemote();
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => remote({ deploymentCapabilities: async () => AHEAD_WINDOW }),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    resourceShelf: { remoteFor: () => resource },
  });

  const snapshot = await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });

  assert.deepEqual(snapshot, { surface: 'upgrade-required', deployment: { origin: DEPLOYMENT.origin, label: DEPLOYMENT.label }, reason: 'protocol-mismatch' });
  assert.equal(runtime.scopeLease(), undefined);
  assert.equal(runtime.resourceShelf(), undefined);
  assert.equal(resource.calls.length, 0, 'no resource request may leave from the explanation-only surface');
});

test('boot restores a stored credential into the read-only surface and sign-out returns to login', async () => {
  const store = fakeStore({ [DEPLOYMENT.origin]: { token: 'stored-access', refreshToken: 'stored-refresh' } });
  const runtime = createMobileRuntime({
    credentialStore: store, remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
  });

  const snapshot = await runtime.boot(DEPLOYMENT);
  assert.equal(snapshot.surface, 'read-only');

  await runtime.signOut();

  assert.deepEqual(runtime.snapshot(), { surface: 'deployment-login', reason: 'authentication-required' });
  assert.equal(await store.read(DEPLOYMENT.origin), undefined);
});

test('a duplicate OIDC callback leaves the read-only scope untouched', async () => {
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(), remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION, pendingOidcStore: pendingStore(),
  });
  const snapshot = await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  assert.equal(snapshot.surface, 'read-only');

  const after = await runtime.completeOidc('weknora://oidc?code=code-1&state=state-1');

  assert.equal(after, snapshot, 'a read-only session with a live lease must ignore duplicate callbacks');
});
