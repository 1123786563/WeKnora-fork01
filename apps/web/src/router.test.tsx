// Router smoke tests: the TanStack route tree must reproduce the pre-router
// dispatch decisions.
//
// Headless (unmounted) routers only commit beforeLoad redirects through
// navigate(); allow-path commits belong to the mounted RouterProvider
// Transitioner, so page matching is asserted with the pure matchRoutes() API.
// The guard/alias matrix itself stays unit-tested in routes.test.ts, and the
// mounted behaviour (redirects on cold start, rendering) is verified against
// the dev server.
import assert from 'node:assert/strict';
import test from 'node:test';
import { createMemoryHistory } from '@tanstack/react-router';
import { createWeKnoraRouter, type WeKnoraRouter, type WeKnoraRouterDeps } from './router.tsx';

interface FakeDeps {
  kind: 'anonymous' | 'bearer';
  tenantId: string | null;
  systemAdmin?: boolean;
  /** AGT-4 — /api/v1/system/capabilities agents 条目；缺省=无 administration 命名空间（fail-open）。 */
  agentsCapability?: { supported: boolean };
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
      ...(fake.agentsCapability !== undefined ? {
        administration: {
          capabilities: () => Promise.resolve({ edition: 'standard', capabilities: { agents: fake.agentsCapability } }),
        },
      } : {}),
    },
    scopeController: { current: () => scope },
    scopeRuntime: {
      capabilities: () => ({ agents: { supported: true }, organizations: { supported: true }, integrations: { supported: true } }),
      isSystemAdmin: () => fake.systemAdmin === true,
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
  // Hard redirect targets (window.location.replace — the pre-router semantics
  // for capability/alias redirects) are captured separately.
  const replaceCalls: string[] = [];
  const hardReplaces: string[] = [];
  const windowStub: Record<string, unknown> = {
    history: {
      replaceState: (_state: unknown, _title: string, url: string) => { replaceCalls.push(String(url)); },
      pushState: (_state: unknown, _title: string, url: string) => { replaceCalls.push(String(url)); },
    },
    location: {
      replace: (url: string) => { hardReplaces.push(String(url)); },
      assign: (url: string) => { hardReplaces.push(String(url)); },
      href: 'http://localhost/',
      pathname: '/',
      search: '',
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
  (router as unknown as { __hardReplaces: string[] }).__hardReplaces = hardReplaces;
  return router;
}

function routeBeforeLoad(router: WeKnoraRouter, routeId: string): ((ctx: { location: { pathname: string; search?: unknown }; abortSignal?: AbortSignal }) => unknown) | undefined {
  const route = (router.routeTree as { children?: Array<{ id: string; options?: { beforeLoad?: (ctx: unknown) => unknown } }> }).children?.find((child) => child.id === routeId);
  return route?.options?.beforeLoad as ((ctx: { location: { pathname: string; search?: unknown }; abortSignal?: AbortSignal }) => unknown) | undefined;
}

function matchedRouteIds(router: WeKnoraRouter, path: string): string[] {
  return (router.matchRoutes(path) as Array<{ routeId: string }>).map((match) => match.routeId);
}

const anonymous: FakeDeps = { kind: 'anonymous', tenantId: null };
const member: FakeDeps = { kind: 'bearer', tenantId: 'tenant-1' };

test('anonymous visitors on protected paths get an SPA replace to plain login', async () => {
  const router = bootRouter(anonymous, '/login');
  const replaceCalls = (router as unknown as { __replaceCalls: string[] }).__replaceCalls;
  // Headless navigate() never flushes its React transition, so drive the
  // guard directly: the platform layout beforeLoad must produce the login
  // SPA replace (verified mounted against the dev server as well).
  const platformRoute = (router.routeTree as { children?: Array<{ id: string; options?: { beforeLoad?: (ctx: unknown) => Promise<unknown> } }> }).children?.find((route) => route.id === '/platform');
  void Promise.resolve(platformRoute?.options?.beforeLoad?.({ location: { pathname: '/platform/agents', search: {} }, abortSignal: undefined })).catch(() => { /* the takeover aborts the superseded load */ });
  await new Promise((resolve) => setTimeout(resolve, 30));
  assert.ok(replaceCalls.includes('/login'), `expected a plain login SPA replace, got ${replaceCalls.join(',')}`);
  assert.ok(replaceCalls.every((url) => !url.includes('next=')), 'no return-URL query may leak into the login redirect (Vue parity)');
});

test('Opportunity evidence path with fixed snapshot ID matches its protected platform route', () => {
 const router = bootRouter(member, '/platform/career/opportunities/opp-1?snapshotId=snapshot-1');
 assert.ok(matchedRouteIds(router, '/platform/career/opportunities/opp-1?snapshotId=snapshot-1').some((id) => id.includes('career/opportunities/$opportunityId')));
});

test('Career evaluation detail path matches its protected platform route by stable evaluation ID', () => {
 const router = bootRouter(member, '/platform/career/evaluations/evaluation-1');
 assert.ok(matchedRouteIds(router, '/platform/career/evaluations/evaluation-1').some((id) => id.includes('career/evaluations/$evaluationId')));
});

test('anonymous root visits get an SPA replace to login', async () => {
  const router = bootRouter(anonymous, '/login');
  const replaceCalls = (router as unknown as { __replaceCalls: string[] }).__replaceCalls;
  void Promise.resolve((router.routeTree as { children?: Array<{ id: string; options?: { beforeLoad?: (ctx: unknown) => Promise<unknown> } }> }).children?.find((route) => route.id === '/')?.options?.beforeLoad?.({ location: { pathname: '/', search: {} }, abortSignal: undefined })).catch(() => { /* the takeover aborts the superseded load */ });
  await new Promise((resolve) => setTimeout(resolve, 30));
  assert.ok(replaceCalls.includes('/login'), `expected a plain login SPA replace, got ${replaceCalls.join(',')}`);
});

test('authenticated platform paths match their pages inside the shell layout', () => {
  const router = bootRouter(member, '/platform/knowledge-bases');
  for (const [path, routeId] of [
    ['/platform/knowledge-bases', '/platform/knowledge-bases'],
    ['/platform/knowledge-bases/kb-1?tab=wiki', '/platform/knowledge-bases/$kbId'],
    ['/platform/knowledge-bases/kb-1/creatChat', '/platform/knowledge-bases/$kbId/creatChat'],
    ['/platform/agents', '/platform/agents'],
    ['/platform/experts', '/platform/experts'],
    ['/platform/settings', '/platform/settings'],
    ['/platform/apps', '/platform/apps'],
    ['/platform/apps/authorization/app-1', '/platform/apps/authorization/$id'],
    ['/platform/creatChat', '/platform/creatChat'],
    ['/platform/chat/session-7', '/platform/chat/$'],
    ['/platform/organizations', '/platform/organizations'],
    // SP14 Task 1 — commercial billing wiring: all three pages live under the
    // platform shell (checkout reads ?order; admin gets capability.operator).
    ['/platform/billing', '/platform/billing'],
    ['/platform/billing/checkout', '/platform/billing/checkout'],
    ['/platform/billing/admin', '/platform/billing/admin'],
    // SP13 Task 8 — read-only shared session entry, pinned alongside the
    // SP14 Task 2 guardRoute allowlist closure for these paths.
    ['/platform/shared/session-token', '/platform/shared/$token'],
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

/* KBL-R1 — library 族 /knowledgeBase/* 收编为 platform 族重定向：认证守卫
 * 先行，随后 SPA replace 到映射表目标（libraryKBRedirectTarget）；未知子
 * 路径不重定向，保留给 library 布局的 404。 */
test('library knowledge-base paths SPA-replace to their platform-family equivalents', async () => {
  const router = bootRouter(member, '/platform/knowledge-bases');
  const replaceCalls = (router as unknown as { __replaceCalls: string[] }).__replaceCalls;
  const libraryBeforeLoad = routeBeforeLoad(router, '/knowledgeBase');
  assert.ok(libraryBeforeLoad, 'the library layout must run the KBL-R1 redirect beforeLoad');
  const cases: Array<[string, Record<string, string>, string]> = [
    ['/knowledgeBase', {}, '/platform/knowledge-bases'],
    ['/knowledgeBase/', {}, '/platform/knowledge-bases'],
    ['/knowledgeBase/kb-1', {}, '/platform/knowledge-bases/kb-1'],
    ['/knowledgeBase/kb-1', { tab: 'wiki', slug: 'home' }, '/platform/knowledge-bases/kb-1?tab=wiki&slug=home'],
    ['/knowledgeBase/kb-1', { knowledge_id: 'doc-1' }, '/platform/knowledge-bases/kb-1?knowledge_id=doc-1'],
    ['/knowledgeBase/kb-1/wiki', { knowledge_id: 'doc-4' }, '/platform/knowledge-bases/kb-1?knowledge_id=doc-4&tab=wiki'],
    ['/knowledgeBase/kb-1/documents/doc-9', {}, '/platform/knowledge-bases/kb-1?knowledge_id=doc-9'],
    ['/knowledgeBase/kb-1/documents/doc-9', { tab: 'wiki' }, '/platform/knowledge-bases/kb-1?knowledge_id=doc-9'],
    ['/knowledgeBase/kb%2Fa/documents/d%2F1', {}, '/platform/knowledge-bases/kb%2Fa?knowledge_id=d%2F1'],
    // FAQ 是 kb.type 就地分流、KB 设置是就地 ⚙ Dialog：两者都落 KB 详情。
    ['/knowledgeBase/kb-1/faq', {}, '/platform/knowledge-bases/kb-1'],
    ['/knowledgeBase/kb-1/settings', {}, '/platform/knowledge-bases/kb-1'],
  ];
  for (const [pathname, search, expected] of cases) {
    replaceCalls.length = 0;
    void Promise.resolve(libraryBeforeLoad({ location: { pathname, search }, abortSignal: undefined })).catch(() => { /* the takeover aborts the superseded load */ });
    await new Promise((resolve) => setTimeout(resolve, 30));
    assert.ok(replaceCalls.includes(expected), `${pathname} should SPA-replace to ${expected}, got ${replaceCalls.join(',')}`);
  }
  // 未知子路径：不重定向（路由树本身不匹配，布局渲染 404）。
  replaceCalls.length = 0;
  void Promise.resolve(libraryBeforeLoad({ location: { pathname: '/knowledgeBase/zzz/unknown', search: {} }, abortSignal: undefined })).catch(() => { /* unreachable path never redirects */ });
  await new Promise((resolve) => setTimeout(resolve, 30));
  assert.equal(replaceCalls.length, 0, `unknown library subpaths must not redirect, got ${replaceCalls.join(',')}`);
});

test('public entries stay outside the shell', () => {
  const router = bootRouter(member, '/login');
  assert.deepEqual(matchedRouteIds(router, '/login'), ['__root__', '/login']);
  assert.deepEqual(matchedRouteIds(router, '/craft'), ['__root__', '/craft', '/craft/$']);
  assert.deepEqual(matchedRouteIds(router, '/craft/session-1'), ['__root__', '/craft', '/craft/$']);
  assert.deepEqual(matchedRouteIds(router, '/embed/channel-9'), ['__root__', '/embed/$']);
  assert.deepEqual(matchedRouteIds(router, '/onboarding/workspace'), ['__root__', '/onboarding/workspace']);
});

test('legacy platform redirect entries hard-replace to their destinations with the exact query preserved', async () => {
  const admin: FakeDeps = { ...member, systemAdmin: true };
  const router = bootRouter(admin, '/platform/knowledge-bases');
  const hardReplaces = (router as unknown as { __hardReplaces: string[] }).__hardReplaces;
  const platformBeforeLoad = routeBeforeLoad(router, '/platform');
  assert.ok(platformBeforeLoad, 'the platform layout must run a guard beforeLoad');
  // R-matrix REDIRECT rows: every retired platform URL keeps its redirect
  // target and query exactly as routeRedirect/Vue define them (knowledge-search
  // carries q as cmdk; integrations folds tab into the settings section and
  // keeps unrelated params; administration/system map to settings sections).
  const cases: Array<[string, Record<string, string>, string]> = [
    ['/platform', {}, '/platform/knowledge-bases'],
    ['/platform/knowledge-search', { q: 'hello' }, '/platform/knowledge-bases?cmdk=hello'],
    ['/platform/tenant', {}, '/platform/settings'],
    ['/platform/administration', {}, '/platform/settings?section=members'],
    ['/platform/system', {}, '/platform/settings?section=system-global'],
    ['/platform/system/queues', {}, '/platform/settings?section=runtime-queues'],
    ['/platform/integrations', { tab: 'embed', agentId: 'a' }, '/platform/settings?agentId=a&section=integration-embed'],
  ];
  for (const [pathname, search, expected] of cases) {
    hardReplaces.length = 0;
    void Promise.resolve(platformBeforeLoad({ location: { pathname, search }, abortSignal: undefined })).catch(() => { /* the takeover aborts the superseded load */ });
    await new Promise((resolve) => setTimeout(resolve, 30));
    assert.ok(hardReplaces.includes(expected), `${pathname} should hard-replace to ${expected}, got ${hardReplaces.join(',')}`);
  }
});

test('join handoffs keep invite tokens and codes in their redirect targets', async () => {
  const joinBeforeLoad = (router: WeKnoraRouter) => {
    const beforeLoad = routeBeforeLoad(router, '/join');
    assert.ok(beforeLoad, 'the join route must run its handoff beforeLoad');
    return beforeLoad;
  };
  // Vue share-links land on /login|/register?token — /join?token forwards the
  // token to the registration entry instead of dead-ending.
  const anonRouter = bootRouter(anonymous, '/login');
  void Promise.resolve(joinBeforeLoad(anonRouter)({ location: { pathname: '/join', search: { token: 't-9' } }, abortSignal: undefined })).catch(() => { /* takeover */ });
  await new Promise((resolve) => setTimeout(resolve, 30));
  const anonHard = (anonRouter as unknown as { __hardReplaces: string[] }).__hardReplaces;
  assert.ok(anonHard.includes('/register?token=t-9'), `expected the share-link token handoff, got ${anonHard.join(',')}`);

  // Authenticated /join?code keeps the code as invite_code on organizations.
  const memberRouter = bootRouter(member, '/platform/organizations');
  void Promise.resolve(joinBeforeLoad(memberRouter)({ location: { pathname: '/join', search: { code: 'c-7' } }, abortSignal: undefined })).catch(() => { /* takeover */ });
  await new Promise((resolve) => setTimeout(resolve, 30));
  const memberHard = (memberRouter as unknown as { __hardReplaces: string[] }).__hardReplaces;
  assert.ok(memberHard.includes('/platform/organizations?invite_code=c-7'), `expected the invite_code handoff, got ${memberHard.join(',')}`);
});

test('knowledge-base view entries keep the tab, slug and knowledge_id query on their pages', () => {
  const router = bootRouter(member, '/platform/knowledge-bases');
  // The ?tab= documents|wiki|graph dispatch is query-driven on both knowledge
  // base paths, so the routes must match with the query attached (the page
  // reads tab/slug/knowledge_id from the location).
  const withQuery = [
    '/platform/knowledge-bases/kb-1?tab=wiki&slug=home',
    '/platform/knowledge-bases/kb-1?tab=graph&slug=g',
    '/platform/knowledge-bases/kb-1?tab=documents&knowledge_id=doc-2',
    '/knowledgeBase/kb-1?tab=wiki&knowledge_id=doc-3',
    '/knowledgeBase/kb-1/wiki?knowledge_id=doc-4',
  ];
  for (const path of withQuery) {
    const ids = matchedRouteIds(router, path);
    assert.ok(ids.length > 0, `${path} should still match its route with the query attached`);
  }
});

/* AGT-4 — Vue router/index.ts:139,453-458：/platform/agents 路由
 * meta.requiredCapability:'agents' 守卫。不支持 → SPA 重定向
 * /platform/knowledge-bases（+ MessagePlugin.warning，headless 下仅尽力而为）；
 * 无 administration 命名空间/探测失败 fail-open 放行（后端仍为权威）。 */
test('AGT-4: /platform/agents redirects to knowledge-bases when the agents capability is unsupported', async () => {
  const router = bootRouter({ kind: 'bearer', tenantId: 'tenant-1', agentsCapability: { supported: false } }, '/platform/agents');
  const replaceCalls = (router as unknown as { __replaceCalls: string[] }).__replaceCalls;
  const agentsRoute = findAgentsRoute(router);
  assert.ok(agentsRoute?.options?.beforeLoad, 'agents route carries the capability guard beforeLoad');
  void Promise.resolve(agentsRoute?.options?.beforeLoad?.({ location: { pathname: '/platform/agents', search: {} }, abortSignal: undefined })).catch(() => { /* the takeover aborts the superseded load */ });
  await new Promise((resolve) => setTimeout(resolve, 40));
  assert.ok(replaceCalls.includes('/platform/knowledge-bases'), `expected the capability redirect, got ${replaceCalls.join(',')}`);
});

test('AGT-4: the agents capability guard fails open when the capabilities probe is unavailable', async () => {
  const router = bootRouter({ kind: 'bearer', tenantId: 'tenant-1' }, '/platform/agents');
  const replaceCalls = (router as unknown as { __replaceCalls: string[] }).__replaceCalls;
  const agentsRoute = findAgentsRoute(router);
  const outcome = await Promise.resolve(agentsRoute?.options?.beforeLoad?.({ location: { pathname: '/platform/agents', search: {} }, abortSignal: undefined }));
  assert.equal(outcome, undefined, 'no redirect decision without an administration namespace (fail-open)');
  assert.equal(replaceCalls.length, 0, 'no URL rewrite happened');
});

/** /platform/agents is a grandchild of the route tree (platform layout child). */
function findAgentsRoute(router: WeKnoraRouter): { options?: { beforeLoad?: (ctx: unknown) => Promise<unknown> } } | undefined {
  const platform = (router.routeTree as { children?: Array<{ id: string; children?: Array<{ id: string; options?: { beforeLoad?: (ctx: unknown) => Promise<unknown> } }> }> }).children?.find((route) => route.id === '/platform');
  return platform?.children?.find((route) => route.id === '/platform/agents');
}
