import * as nodeModule from 'node:module';
import * as React from 'react';
import { act } from 'react';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/knowledge-bases' });
Object.assign(globalThis, {
  window: dom.window,
  self: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  IS_REACT_ACT_ENVIRONMENT: true,
});

const { createRoot } = await import('react-dom/client');
const { RouterProvider } = await import('@tanstack/react-router');
const mod = await import('./router.tsx');
const createWeKnoraRouter = mod.createWeKnoraRouter as (deps: unknown, options?: unknown) => import('@tanstack/react-router').Router;

const scope = { scope: { tenantId: 't1' } };
const ok = { data: [], items: [], next_cursor: null };
const anyFn: unknown = new Proxy(function () {}, { apply: () => Promise.resolve(ok), get: () => anyFn });
const deps = {
  client: new Proxy({ auth: { autoSetup: () => Promise.reject(new Error('x')) } }, { get: (t, p) => (t as Record<string | symbol, unknown>)[p] ?? anyFn }),
  scopeController: { current: () => scope },
  scopeRuntime: { capabilities: () => ({}), isSystemAdmin: () => false, role: () => 'owner', canViewChannelSessions: () => false, current: () => scope },
  session: () => ({ credential: { kind: 'bearer', accessToken: 't' }, tenantId: 't1', preferences: {} }),
  liteMode: false,
  development: false,
  apiBaseUrl: '',
  loadingText: 'L',
  initialLoginError: () => undefined,
  ensureSessionHydrated: () => Promise.resolve({ ok: true, tenantId: 't1' }),
  logout: () => Promise.resolve(),
  switchTenantFromShell: () => Promise.resolve(),
  completeAuthentication: () => undefined,
};

const router = createWeKnoraRouter(deps);
console.log('latestLocation:', (router as unknown as Record<string, unknown>).latestLocation);
console.log('stores:', typeof (router as unknown as Record<string, unknown>).stores);

const container = dom.window.document.createElement('div');
dom.window.document.body.append(container);
const root = createRoot(container);
try {
  await act(async () => root.render(React.createElement(RouterProvider, { router })));
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 80)); });
  console.log('pathname:', router.state.location.pathname);
  console.log('matches:', router.state.matches.map((m) => m.routeId));
} catch (error) {
  console.log('render error:', (error as Error).message);
}
