// Router smoke tests: the TanStack route tree must reproduce the pre-router
// dispatch decisions.
//
// Headless (unmounted) routers only commit beforeLoad redirects through
// navigate(); allow-path commits belong to the mounted RouterProvider
// Transitioner, so page matching is asserted with the pure matchRoutes() API.
// The guard/alias matrix itself stays unit-tested in routes.test.ts, and the
// mounted behaviour (redirects on cold start, ?next preservation, rendering)
// is verified against the dev server.
import assert from 'node:assert/strict';
import test from 'node:test';
import { createMemoryHistory } from '@tanstack/react-router';
import { createWeKnoraRouter, type WeKnoraRouter, type WeKnoraRouterDeps } from './router.tsx';

interface FakeDeps {
  kind: 'anonymous' | 'bearer';
  tenantId: string | null;
}

function makeDeps(fake: FakeDeps): WeKnoraRouterDeps {
  const scope = { scope: { tenantId: fake.tenantId } };
  return {
    client: {
      auth: {
        autoSetup: () => Promise.reject(new Error('auto-setup unavailable')),
        registrationConfig: () => Promise.resolve({ registrationMode: 'self_serve' }),
        acceptInvitationByToken: () => Promise.resolve({}),
      },
    },
    scopeController: { current: () => scope },
    scopeRuntime: {
      capabilities: () => ({ agents: { supported: true }, organizations: { supported: true }, integrations: { supported: true } }),
      isSystemAdmin: () => false,
      role: () => 'owner',
      canViewChannelSessions: () => false,
      current: () => scope,
    },
    session: () => (
      fake.kind === 'bearer'
        ? { credential: { kind: 'bearer', accessToken: 'token' }, tenantId: fake.tenantId, preferences: {} }
        : { credential: { kind: 'anonymous' }, tenantId: null, preferences: {} }
    ),
    liteMode: false,
    development: false,
    apiBaseUrl: '',
    loadingText: 'Loading…',
    initialLoginError: () => undefined,
    ensureSessionHydrated: () => Promise.resolve(fake.kind === 'bearer' ? { ok: true, tenantId: fake.tenantId } : { ok: false }),
    logout: () => Promise.resolve(),
    switchTenantFromShell: () => Promise.resolve(),
    completeAuthentication: () => undefined,
  } as unknown as WeKnoraRouterDeps;
}

function bootRouter(fake: FakeDeps, entry: string): WeKnoraRouter {
  // Headless stand-in: guard SPA replaces write through window.history and the
  // router sync rides on PopStateEvent in the browser; capture the target here.
  const replaceCalls: string[] = [];
  const windowStub: Record<string, unknown> = {
    history: {
      replaceState: (_state: unknown, _title: string, url: string) => { replaceCalls.push(String(url)); },
      pushState: (_state: unknown, _title: string, url: string) => { replaceCalls.push(String(url)); },
    },
    localStorage: { getItem: () => null, setItem: () => undefined, removeItem: () => undefined },
    dispatchEvent: () => true,
  };
  windowStub.self = windowStub;
  (globalThis as Record<string, unknown>).window = windowStub;
  (globalThis as Record<string, unknown>).self = windowStub;
  (globalThis as Record<string, unknown>).document = {};
  const router = createWeKnoraRouter(makeDeps(fake), { history: createMemoryHistory({ initialEntries: [entry] }) });
  (router as unknown as { __replaceCalls: string[] }).__replaceCalls = replaceCalls;
  return router;
}

function matchedRouteIds(router: WeKnoraRouter, path: string): string[] {
  return (router.matchRoutes(path) as Array<{ routeId: string }>).map((match) => match.routeId);
}

const anonymous: FakeDeps = { kind: 'anonymous', tenantId: null };
const member: FakeDeps = { kind: 'bearer', tenantId: 'tenant-1' };

test('anonymous visitors on protected paths get an SPA replace to login with the next hop', async () => {
  const router = bootRouter(anonymous, '/login');
  const replaceCalls = (router as unknown as { __replaceCalls: string[] }).__replaceCalls;
  // Headless navigate() never flushes its React transition, so drive the
  // guard directly: the platform layout beforeLoad must produce the login
  // SPA replace with the next hop preserved (verified mounted against the
  // dev server as well).
  const platformRoute = (router.routeTree as { children?: Array<{ id: string; options?: { beforeLoad?: (ctx: unknown) => Promise<unknown> } }> }).children?.find((route) => route.id === '/platform');
  void Promise.resolve(platformRoute?.options?.beforeLoad?.({ location: { pathname: '/platform/agents', search: {} }, abortSignal: undefined })).catch(() => { /* the takeover aborts the superseded load */ });
  await new Promise((resolve) => setTimeout(resolve, 30));
  assert.ok(replaceCalls.includes('/login?next=%2Fplatform%2Fagents'), `expected a login SPA replace, got ${replaceCalls.join(',')}`);
});

test('anonymous root visits get an SPA replace to login', async () => {
  const router = bootRouter(anonymous, '/login');
  const replaceCalls = (router as unknown as { __replaceCalls: string[] }).__replaceCalls;
  void Promise.resolve((router.routeTree as { children?: Array<{ id: string; options?: { beforeLoad?: (ctx: unknown) => Promise<unknown> } }> }).children?.find((route) => route.id === '/')?.options?.beforeLoad?.({ location: { pathname: '/', search: {} }, abortSignal: undefined })).catch(() => { /* the takeover aborts the superseded load */ });
  await new Promise((resolve) => setTimeout(resolve, 30));
  assert.ok(replaceCalls.includes('/login?next=%2F'), `expected a login SPA replace, got ${replaceCalls.join(',')}`);
});

test('authenticated platform paths match their pages inside the shell layout', () => {
  const router = bootRouter(member, '/platform/knowledge-bases');
  for (const [path, routeId] of [
    ['/platform/knowledge-bases', '/platform/knowledge-bases'],
    ['/platform/knowledge-bases/kb-1?tab=wiki', '/platform/knowledge-bases/$kbId'],
    ['/platform/knowledge-bases/kb-1/creatChat', '/platform/knowledge-bases/$kbId/creatChat'],
    ['/platform/agents', '/platform/agents'],
    ['/platform/settings', '/platform/settings'],
    ['/platform/apps', '/platform/apps'],
    ['/platform/apps/authorization/app-1', '/platform/apps/authorization/$id'],
    ['/platform/creatChat', '/platform/creatChat'],
    ['/platform/chat/session-7', '/platform/chat/$'],
    ['/platform/organizations', '/platform/organizations'],
  ] as const) {
    const ids = matchedRouteIds(router, path.split('?')[0]!);
    assert.ok(ids.includes(routeId), `${path} should match ${routeId}, got ${ids.join(',')}`);
    assert.ok(ids.includes('/platform'), `${path} should render inside the platform shell`);
  }
});

test('knowledge-base paths match their owning routes with the shell', () => {
  const router = bootRouter(member, '/platform/knowledge-bases');
  for (const [path, routeId] of [
    ['/knowledgeBase', '/knowledgeBase/'],
    ['/knowledgeBase/kb-1', '/knowledgeBase/$kbId'],
    ['/knowledgeBase/kb-1/documents/doc-9', '/knowledgeBase/$kbId/documents/$docId'],
    ['/knowledgeBase/kb-1/wiki', '/knowledgeBase/$kbId/wiki'],
    ['/knowledgeBase/kb-1/faq', '/knowledgeBase/$kbId/faq'],
    ['/knowledgeBase/kb-1/settings', '/knowledgeBase/$kbId/settings'],
  ] as const) {
    const ids = matchedRouteIds(router, path);
    assert.ok(ids.includes(routeId), `${path} should match ${routeId}, got ${ids.join(',')}`);
    assert.ok(ids.includes('/knowledgeBase'), `${path} should render inside the library shell`);
  }
});

test('public entries stay outside the shell', () => {
  const router = bootRouter(member, '/login');
  assert.deepEqual(matchedRouteIds(router, '/login'), ['__root__', '/login']);
  assert.deepEqual(matchedRouteIds(router, '/craft'), ['__root__', '/craft', '/craft/$']);
  assert.deepEqual(matchedRouteIds(router, '/craft/session-1'), ['__root__', '/craft', '/craft/$']);
  assert.deepEqual(matchedRouteIds(router, '/embed/channel-9'), ['__root__', '/embed/$']);
  assert.deepEqual(matchedRouteIds(router, '/onboarding/workspace'), ['__root__', '/onboarding/workspace']);
});
