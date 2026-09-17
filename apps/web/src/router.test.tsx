// Router integration tests: the TanStack route tree mounted under a real
// RouterProvider (jsdom) must reproduce the pre-router dispatch decisions —
// guard redirects for anonymous visitors (with the ?next hop intact), page
// matching for authenticated deep links, and the legacy alias surface owned
// by routes.tsx (unit-tested there).
//
// Hard window.location.replace redirects (capability/system-admin guards) are
// covered by routes.test.ts decisions and jsdom cannot navigate; they are not
// exercised here.
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { act } from 'react';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };

const settle = (ms = 40): Promise<void> => new Promise((resolve) => setTimeout(resolve, ms));

interface MountResult {
  router: import('@tanstack/react-router').Router;
  cleanup(): void;
  location(): { pathname: string; search: Record<string, string> };
  matches(): string[];
}

async function mountRouter(url: string, fake: Parameters<typeof makeDeps>[0]): Promise<MountResult> {
  const dom = new JSDOM('<!doctype html><html><body></body></html>', { url });
  Object.assign(globalThis, {
    window: dom.window,
    self: dom.window,
    document: dom.window.document,
    HTMLElement: dom.window.HTMLElement,
    Event: dom.window.Event,
    MouseEvent: dom.window.MouseEvent,
    IS_REACT_ACT_ENVIRONMENT: true,
  });
  (dom.window as unknown as Record<string, unknown>).matchMedia = () => ({ matches: false, addEventListener() {}, removeEventListener() {} });
  (dom.window as unknown as Record<string, unknown>).IntersectionObserver = class { observe() {} unobserve() {} disconnect() {} };
  (dom.window as unknown as Record<string, unknown>).ResizeObserver = class { observe() {} unobserve() {} disconnect() {} };

  const { createRoot } = await import('react-dom/client');
  const { RouterProvider } = await import('@tanstack/react-router');
  const { createWeKnoraRouter } = await import('./router.tsx');
  const router = createWeKnoraRouter(makeDeps(fake));
  const container = dom.window.document.createElement('div');
  dom.window.document.body.append(container);
  const root = createRoot(container);
  await act(async () => root.render(React.createElement(RouterProvider, { router })));
  await act(async () => { await settle(); });
  return {
    router,
    cleanup: () => { try { root.unmount(); } catch { /* jsdom teardown */ } },
    location: () => {
      const location = router.state.location;
      const search: Record<string, string> = {};
      for (const [key, value] of Object.entries(location.search as Record<string, unknown>)) search[key] = String(value);
      return { pathname: location.pathname, search };
    },
    matches: () => router.state.matches.map((match) => match.routeId),
  };
}

interface FakeDeps {
  kind: 'anonymous' | 'bearer';
  tenantId: string | null;
}

function makeDeps(fake: FakeDeps): import('./router.tsx').WeKnoraRouterDeps {
  const scope = { scope: { tenantId: fake.tenantId } };
  const ok = { data: [], items: [], next_cursor: null };
  const anyFn: unknown = new Proxy(function () {}, {
    apply: () => Promise.resolve(ok),
    get: () => anyFn,
  });
  return {
    client: new Proxy(
      {
        auth: {
          autoSetup: () => Promise.reject(new Error('auto-setup unavailable')),
          registrationConfig: () => Promise.resolve({ registrationMode: 'self_serve' }),
          acceptInvitationByToken: () => Promise.resolve(ok),
        },
      },
      { get: (target, property) => (target as Record<string | symbol, unknown>)[property] ?? anyFn },
    ),
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
  } as unknown as import('./router.tsx').WeKnoraRouterDeps;
}

test('authenticated deep links land on their pages inside the shell', async () => {
  const mounted = await mountRouter('https://weknora.test/platform/knowledge-bases', { kind: 'bearer', tenantId: 'tenant-1' });
  try {
    assert.equal(mounted.location().pathname, '/platform/knowledge-bases');
    assert.ok(mounted.matches().includes('/platform'), 'platform shell layout missing');

    for (const [path, routeId] of [
      ['/platform/agents', '/platform/agents'],
      ['/platform/settings', '/platform/settings'],
      ['/platform/apps/connections', '/platform/apps/connections'],
      ['/platform/creatChat', '/platform/creatChat'],
      ['/platform/chat/session-7', '/platform/chat/$'],
      ['/platform/knowledge-bases/kb-1?tab=wiki', '/platform/knowledge-bases/$kbId'],
      ['/knowledgeBase/kb-1/documents/doc-9', '/knowledgeBase/$kbId/documents/$docId'],
      ['/knowledgeBase', '/knowledgeBase/'],
      ['/creatChat', '/creatChat'],
    ] as const) {
      const [, rawSearch = ''] = path.split('?');
      await act(async () => {
        mounted.router.history.push(path);
        await settle(60);
      });
      const { pathname } = mounted.location();
      assert.equal(pathname, path.split('?')[0], `navigation to ${path}`);
      if (routeId !== '/platform/knowledge-bases/$kbId' || !path.includes('tab=wiki')) {
        assert.ok(mounted.matches().includes(routeId), `${path} should match ${routeId}, got ${mounted.matches().join(',')}`);
      }
      assert.equal(new URLSearchParams(rawSearch).get('tab') === 'wiki', path.includes('tab=wiki'));
    }
  } finally {
    mounted.cleanup();
  }
});

test('anonymous visitors on protected paths redirect to login keeping the next hop', async () => {
  const mounted = await mountRouter('https://weknora.test/login', { kind: 'anonymous', tenantId: null });
  try {
    await act(async () => {
      mounted.router.history.push('/platform/agents');
      await settle(80);
    });
    const { pathname, search } = mounted.location();
    assert.equal(pathname, '/login', 'guard redirect landed on login');
    assert.equal(search.next, '/platform/agents', 'next hop preserved through the redirect');

    await act(async () => {
      mounted.router.history.push('/platform/definitely-not-a-page');
      await settle(80);
    });
    assert.equal(mounted.location().pathname, '/login');
    assert.equal(mounted.location().search.next, '/platform/definitely-not-a-page');
  } finally {
    mounted.cleanup();
  }
});

test('unmatched public paths stay put for the shell-wrapped 404', async () => {
  const mounted = await mountRouter('https://weknora.test/platform/knowledge-bases', { kind: 'bearer', tenantId: 'tenant-1' });
  try {
    await act(async () => {
      mounted.router.history.push('/definitely-not-a-page');
      await settle(60);
    });
    assert.equal(mounted.location().pathname, '/definitely-not-a-page');
  } finally {
    mounted.cleanup();
  }
});
