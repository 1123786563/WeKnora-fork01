import test from 'node:test';
import assert from 'node:assert/strict';
import { createMobileRuntimeRemote } from './runtime.ts';
import type { ClientRequest } from '../client.ts';

const ORIGIN = 'https://weknora.example.test';

/** 真实 GET /api/v1/system/capabilities wire 报文（internal/handler/deployment_capabilities.go
 *  GetDeploymentCapabilities：code/msg/data 包装，data 内含协议窗口）。 */
const CAPABILITY_DATA = {
  edition: 'standard',
  capabilities: {
    organizations: { supported: true },
    agents: { supported: true },
    'integrations.embed': { supported: false, reason: 'route_not_registered' },
    'settings.sandbox.docker': { supported: true },
  },
  protocol_minimum: 2,
  protocol_maximum: 3,
};

const CAPABILITY_ENVELOPE = { code: 0, msg: 'success', data: CAPABILITY_DATA };

interface Recorder {
  seen: ClientRequest[];
  request: (input: ClientRequest) => Promise<unknown>;
}

/** 捕获每个 ClientRequest，便于断言「零请求」与具体 method/path/headers。 */
function recorder(responder: (input: ClientRequest) => unknown): Recorder {
  const seen: ClientRequest[] = [];
  return {
    seen,
    request: async (input: ClientRequest): Promise<unknown> => {
      seen.push(input);
      return responder(input);
    },
  };
}

test('deploymentCapabilities sends the bearer token and returns the raw capability payload only', async () => {
  const spy = recorder(() => CAPABILITY_ENVELOPE);
  const remote = createMobileRuntimeRemote({ origin: ORIGIN, request: spy.request });

  const payload = await remote.deploymentCapabilities('cap-token');

  // 适配器只解包成功信封，原样交出 data——协议窗口判定属于 Mobile Runtime（clientGate），
  // 适配器不得替它决定安全面。
  assert.deepEqual(payload, CAPABILITY_DATA);
  assert.equal(payload.protocol_minimum, 2);
  assert.equal(payload.protocol_maximum, 3);

  assert.equal(spy.seen.length, 1);
  const call = spy.seen[0]!;
  assert.equal(call.method, 'GET');
  assert.equal(call.path, '/api/v1/system/capabilities');
  assert.equal((call.headers as Record<string, string> | undefined)?.authorization, 'Bearer cap-token');
});

test('deploymentCapabilities rejects a failed or malformed capabilities envelope', async () => {
  const rejected = createMobileRuntimeRemote({
    origin: ORIGIN,
    request: recorder(() => ({ code: 403, msg: 'permission denied' })).request,
  });
  await assert.rejects(rejected.deploymentCapabilities('cap-token'), /permission denied/);

  const noData = createMobileRuntimeRemote({
    origin: ORIGIN,
    request: recorder(() => ({ code: 0, msg: 'success' })).request,
  });
  await assert.rejects(noData.deploymentCapabilities('cap-token'), /data must be an object/);

  const notEnvelope = createMobileRuntimeRemote({
    origin: ORIGIN,
    request: recorder(() => [CAPABILITY_DATA]).request,
  });
  await assert.rejects(notEnvelope.deploymentCapabilities('cap-token'), /must be an object/);
});

test('rejects an invalid or non-HTTPS deployment origin before any request is sent', () => {
  const invalidOrigins = [
    'http://insecure.example.test',
    'ftp://weknora.example.test',
    'https://user:pass@weknora.example.test',
    'not-a-url',
    '',
  ];
  for (const origin of invalidOrigins) {
    const spy = recorder(() => CAPABILITY_ENVELOPE);
    assert.throws(
      () => createMobileRuntimeRemote({ origin, request: spy.request }),
      /origin/i,
      `origin ${JSON.stringify(origin)} must be rejected at construction`,
    );
    assert.equal(spy.seen.length, 0, `no request may be sent for origin ${JSON.stringify(origin)}`);
  }
});

test('delegates auth flows to createAuthApi endpoints on the same transport', async () => {
  const spy = recorder((input) => {
    if (input.path === '/api/v1/auth/login') return { success: true, token: 'a-1', refresh_token: 'r-1', user: { id: 'u-1' } };
    if (input.path === '/api/v1/auth/me') return { success: true, data: { user: { id: 'u-1' }, tenant: { id: 7 } } };
    if (input.path === '/api/v1/auth/oidc/config') return { success: true, enabled: true, provider_display_name: 'Acme SSO' };
    if (input.path.startsWith('/api/v1/auth/oidc/url')) return { success: true, authorization_url: 'https://idp.test/authorize', state: 'state-1' };
    if (input.path === '/api/v1/auth/oidc/exchange') return { success: true, token: 'a-2', refresh_token: 'r-2' };
    if (input.path === '/api/v1/auth/refresh') return { success: true, access_token: 'a-3', refresh_token: 'r-3' };
    throw new Error(`unexpected path ${input.path}`);
  });
  const remote = createMobileRuntimeRemote({ origin: ORIGIN, request: spy.request });

  assert.deepEqual(
    await remote.passwordLogin({ email: 'user@example.test', password: 'secret' }),
    { token: 'a-1', refreshToken: 'r-1', user: { id: 'u-1' }, tenant: undefined, memberships: undefined },
  );
  assert.deepEqual(await remote.me('a-1'), { user: { id: 'u-1' }, tenant: { id: 7 }, memberships: undefined, tenant_required: false, capabilities: undefined });
  assert.deepEqual(await remote.oidcConfig(), { enabled: true, providerDisplayName: 'Acme SSO' });
  assert.deepEqual(
    await remote.oidcUrl('https://app.test/callback', 'https://app.test/finish', 'challenge-1'),
    { authorizationUrl: 'https://idp.test/authorize', state: 'state-1' },
  );
  assert.deepEqual(await remote.oidcExchange('code-1', 'state-1', 'verifier-1'), { token: 'a-2', refreshToken: 'r-2', user: undefined, tenant: undefined, memberships: undefined });
  assert.deepEqual(await remote.refresh('r-1'), { access_token: 'a-3', refresh_token: 'r-3' });

  const byPath = (path: string): ClientRequest => {
    const call = spy.seen.find((item) => item.path === path || item.path.startsWith(`${path}?`));
    assert.ok(call, `expected a request to ${path}`);
    return call;
  };
  assert.equal(byPath('/api/v1/auth/login').method, 'POST');
  assert.deepEqual(byPath('/api/v1/auth/login').body, { email: 'user@example.test', password: 'secret' });
  assert.equal(byPath('/api/v1/auth/me').method, 'GET');
  assert.equal((byPath('/api/v1/auth/me').headers as Record<string, string> | undefined)?.authorization, 'Bearer a-1');
  assert.ok(byPath('/api/v1/auth/oidc/url').path.includes('redirect_uri=https%3A%2F%2Fapp.test%2Fcallback'));
  assert.ok(byPath('/api/v1/auth/oidc/url').path.includes('frontend_redirect_uri=https%3A%2F%2Fapp.test%2Ffinish'));
  assert.ok(byPath('/api/v1/auth/oidc/url').path.includes('code_challenge=challenge-1'));
  assert.deepEqual(byPath('/api/v1/auth/oidc/exchange').body, { code: 'code-1', state: 'state-1', code_verifier: 'verifier-1' });
  assert.deepEqual(byPath('/api/v1/auth/refresh').body, { refreshToken: 'r-1' });
});

test('rejects an empty access token before any request is sent', async () => {
  const spy = recorder(() => CAPABILITY_ENVELOPE);
  const remote = createMobileRuntimeRemote({ origin: ORIGIN, request: spy.request });
  await assert.rejects(remote.deploymentCapabilities(''), /access token/);
  await assert.rejects(remote.me('   '), /access token/);
  assert.equal(spy.seen.length, 0);
});
