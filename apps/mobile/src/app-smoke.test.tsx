import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { after, test } from 'node:test';
import * as nodeModule from 'node:module';

// Native-only modules (expo-router and friends) evaluate React Native globals
// such as `__DEV__` at import time, so they cannot execute under the Node test
// runner. Resolve them to inert CommonJS stubs (real files, so both `import`
// and transpiled `require` load them) while the app shell module graph itself
// stays fully loaded and inspectable in-process.
type ResolveNext = (specifier: string, context: unknown) => unknown;
type ResolveHook = (specifier: string, context: unknown, nextResolve: ResolveNext) => unknown;
const moduleWithHooks = nodeModule as typeof nodeModule & {
  registerHooks?: (hooks: { resolve: ResolveHook }) => void;
};

const NATIVE_MODULE_STUBS: Record<string, string> = {
  'expo-linking': 'module.exports = { useLinkingURL() { return null; } }',
  'expo-router': "module.exports = { Stack: function Stack() { return null; }, router: { replace() {}, push() {} }, useLocalSearchParams() { return {}; } }",
  'expo-secure-store': "module.exports = { getItemAsync: async () => null, setItemAsync: async () => {}, deleteItemAsync: async () => {} }",
  'expo-web-browser': "module.exports = { openAuthSessionAsync: async () => ({ type: 'dismiss' }) }",
  'react-native': "module.exports = { View: 'View', Text: 'Text', TextInput: 'TextInput', Button: 'Button', ScrollView: 'ScrollView' }",
  react: "let values = []; let cursor = 0; let pendingEffects = []; let effectCleanups = []; module.exports = { __beginRender() { cursor = 0; }, __reset() { values = []; cursor = 0; pendingEffects = []; effectCleanups = []; }, useState(initial) { const index = cursor++; if (!(index in values)) values[index] = initial; return [values[index], (next) => { values[index] = typeof next === 'function' ? next(values[index]) : next; }]; }, useRef(value) { const index = cursor++; if (!(index in values)) values[index] = { current: value }; return values[index]; }, useEffect(setup) { pendingEffects.push(setup); }, __mount() { for (const setup of pendingEffects.splice(0)) effectCleanups.push(setup()); }, __unmount() { for (const cleanup of effectCleanups.splice(0)) { if (typeof cleanup === 'function') cleanup(); } }, useSyncExternalStore(_subscribe, getSnapshot) { return getSnapshot(); }, createElement(type, props, ...children) { return { type, props: { ...(props || {}), ...(children.length === 0 ? {} : { children: children.length === 1 ? children[0] : children }) } }; } };",
};
const stubDir = mkdtempSync(join(tmpdir(), 'weknora-mobile-stub-'));
const stubPath = (name: string): string => join(stubDir, `${name.replaceAll('/', '+')}.cjs`);
for (const [name, source] of Object.entries(NATIVE_MODULE_STUBS)) {
  writeFileSync(stubPath(name), source);
}
if (moduleWithHooks.registerHooks) {
  moduleWithHooks.registerHooks({
    resolve: (specifier, context, nextResolve) =>
      specifier in NATIVE_MODULE_STUBS
        ? { shortCircuit: true, url: pathToFileURL(stubPath(specifier)).href }
        : nextResolve(specifier, context),
  });
}
after(() => {
  rmSync(stubDir, { recursive: true, force: true });
});

// String-based path math avoids the DOM-lib `URL` vs `node:url` `URL` type clash
// that Expo's tsconfig base (lib includes DOM) would otherwise raise.
const workspaceRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..', '..');
const require = createRequire(import.meta.url);
(globalThis as typeof globalThis & { React?: unknown }).React = require('react');

function hooks(): { __beginRender(): void; __reset(): void; __mount(): void; __unmount(): void } {
  return require('react') as { __beginRender(): void; __reset(): void; __mount(): void; __unmount(): void };
}

function render(component: (props: any) => unknown, props: any): unknown {
  hooks().__beginRender();
  return component(props);
}

function descendants(node: unknown): Array<{ type: unknown; props: Record<string, unknown> }> {
  if (Array.isArray(node)) return node.flatMap(descendants);
  if (!node || typeof node !== 'object') return [];
  const element = node as { type?: unknown; props?: Record<string, unknown> };
  const here = element.props ? [{ type: element.type, props: element.props }] : [];
  const children = element.props?.children;
  const childList = Array.isArray(children) ? children.flat(Infinity) : [children];
  return [...here, ...childList.flatMap(descendants)];
}

test('app module exports an application root', async () => {
  const layout = await import('./app/_layout.tsx');
  assert.equal(
    typeof layout.default,
    'function',
    'src/app/_layout.tsx must default-export the application root component',
  );
});

test('OIDC callback route forwards the untouched deep link before returning to the app root', async () => {
  const callback = await import('./app/auth-return.tsx');
  const seen: string[] = [];
  const callbackUrl = 'weknora://oidc?code=code%2B1&state=state%2F1';

  const delivered = await callback.deliverOidcReturn(
    callbackUrl,
    async (url: string) => { seen.push(`complete:${url}`); },
    () => { seen.push('finish:/'); },
  );

  assert.equal(delivered, true);
  assert.deepEqual(seen, [`complete:${callbackUrl}`, 'finish:/']);
  assert.equal(await callback.deliverOidcReturn('weknora://other?code=x&state=y', async () => { seen.push('invalid'); }, () => { seen.push('invalid-finish'); }), false);
  assert.deepEqual(seen, [`complete:${callbackUrl}`, 'finish:/']);
});

test('the backend-approved weknora://oidc deep link has a matching Expo file route', async () => {
  const callback = await import('./app/auth-return.tsx');
  const oidcRoute = await import('./app/oidc.tsx');

  assert.equal(oidcRoute.default, callback.default);
});

test('deployment login accepts only normalized HTTPS origins without embedded credentials', async () => {
  const { DeploymentLoginScreen, validatedDeploymentOrigin } = await import('./screens/DeploymentLoginScreen.tsx');
  assert.equal(validatedDeploymentOrigin('https://weknora.example.test/'), 'https://weknora.example.test');
  assert.equal(validatedDeploymentOrigin('http://weknora.example.test'), undefined);
  assert.equal(validatedDeploymentOrigin('https://member:password@weknora.example.test'), undefined);
  assert.equal(validatedDeploymentOrigin('https://weknora.example.test/path'), undefined);
  hooks().__reset();
  const signIns: Array<{ origin: string; email: string; password: string }> = [];
  const props = { onSignIn: async (input: { origin: string; email: string; password: string }) => { signIns.push(input); }, onBeginOidc: async () => {} };
  for (const invalidOrigin of ['http://weknora.example.test', 'https://member:password@weknora.example.test', 'https://weknora.example.test/path', 'https://weknora.example.test?next=x', 'https://weknora.example.test#fragment']) {
    const form = render(DeploymentLoginScreen, props);
    const fields = descendants(form).filter(({ type }) => type === 'TextInput');
    (fields[0]!.props.onChangeText as (value: string) => void)(invalidOrigin);
    const updated = render(DeploymentLoginScreen, props);
    const signIn = descendants(updated).find(({ type, props: button }) => type === 'Button' && button.title === 'Sign in');
    (signIn!.props.onPress as () => void)();
    assert.deepEqual(signIns, [], `invalid origin ${invalidOrigin} must not invoke sign-in`);
  }
  const form = render(DeploymentLoginScreen, props);
  const fields = descendants(form).filter(({ type }) => type === 'TextInput');
  (fields[0]!.props.onChangeText as (value: string) => void)('https://weknora.example.test/');
  (fields[1]!.props.onChangeText as (value: string) => void)('member@example.test');
  (fields[2]!.props.onChangeText as (value: string) => void)('password');
  const valid = render(DeploymentLoginScreen, props);
  const signIn = descendants(valid).find(({ type, props: button }) => type === 'Button' && button.title === 'Sign in');
  (signIn!.props.onPress as () => void)();
  assert.deepEqual(signIns, [{ origin: 'https://weknora.example.test', email: 'member@example.test', password: 'password' }]);
});

test('surface routing keeps upgrade-required free of authorized controls and guards incomplete identity', async () => {
  const { RuntimeSurface } = await import('./composition.ts');
  const safe = RuntimeSurface({
    snapshot: { surface: 'upgrade-required', deployment: { origin: 'https://weknora.example.test', label: 'WeKnora' }, reason: 'protocol-mismatch' },
    onSignIn: async () => {}, onBeginOidc: async () => {}, onSignOut: async () => {},
    onActivateTenant: async () => {},
  });
  assert.equal(safe.type.name, 'UpgradeRequiredScreen');
  const safeControls = descendants(render(safe.type, safe.props)).filter(({ type }) => type === 'Button').map(({ props }) => props.title);
  const safeText = descendants(render(safe.type, safe.props)).filter(({ type }) => type === 'Text').flatMap(({ props }) => props.children).join(' ');
  assert.equal(safeControls.includes('Open Task Office'), false);
  assert.equal(safeText.includes('Task Office'), false);

  const invalidAuthorized = RuntimeSurface({
    snapshot: { surface: 'authorized', deployment: { origin: 'https://weknora.example.test', label: 'WeKnora' }, identity: { userId: 'member-1' } },
    onSignIn: async () => {}, onBeginOidc: async () => {}, onSignOut: async () => {},
    onActivateTenant: async () => {},
  });
  assert.equal(invalidAuthorized.type.name, 'UpgradeRequiredScreen');

  const authorized = RuntimeSurface({
    snapshot: { surface: 'authorized', deployment: { origin: 'https://weknora.example.test', label: 'WeKnora' }, identity: { userId: 'member-1', activeTenantId: 'tenant-1', tenants: [{ id: 'tenant-1', name: 'Acme' }] } },
    onSignIn: async () => {}, onBeginOidc: async () => {}, onSignOut: async () => {}, onActivateTenant: async () => {},
  });
  assert.equal(authorized.type.name, 'HomeScreen');
  hooks().__reset();
  const homeElement = render(authorized.type, authorized.props);
  const homeButtons = descendants(homeElement).filter(({ type }) => type === 'Button').map(({ props }) => props.title);
  assert.equal(homeButtons.includes('View all tasks'), true);
  assert.equal(homeButtons.includes('Open Resources'), true, 'removing the landing screen must not take away the only /resources entry');
  const homeText = descendants(homeElement).filter(({ type }) => type === 'Text').flatMap(({ props }) => props.children).join(' ');
  assert.equal(homeText.includes('Acme'), true);
  assert.equal((authorized.props as { key?: string }).key, 'tenant-1', 'the home screen is keyed by the active tenant');
  const switched = RuntimeSurface({
    snapshot: { surface: 'authorized', deployment: { origin: 'https://weknora.example.test', label: 'WeKnora' }, identity: { userId: 'member-1', activeTenantId: 'tenant-2', tenants: [{ id: 'tenant-2', name: 'Beta' }] } },
    onSignIn: async () => {}, onBeginOidc: async () => {}, onSignOut: async () => {}, onActivateTenant: async () => {},
  });
  assert.equal((switched.props as { key?: string }).key, 'tenant-2', 'switching tenants must remount the screen so the previous tenant content cannot survive');
});

test('mobile startup invokes Runtime boot exactly once', async () => {
  const { bootRuntimeOnce } = await import('./composition.ts');
  let calls = 0;
  const runtime = { boot: async () => { calls += 1; } };
  const booted = { current: false };
  bootRuntimeOnce(runtime, booted);
  bootRuntimeOnce(runtime, booted);
  assert.equal(calls, 1);
});

test('pnpm --filter @weknora/mobile typecheck resolves the package', () => {
  const result = spawnSync('pnpm', ['--filter', '@weknora/mobile', 'typecheck'], {
    cwd: workspaceRoot,
    encoding: 'utf8',
  });
  // pnpm exits 0 even when a filter matches no project, so resolution must be
  // asserted explicitly: a missing apps/mobile workspace shows this message.
  assert.ok(
    !/No projects matched the filters/.test(`${result.stdout}${result.stderr}`),
    `the @weknora/mobile filter matched no project\nstdout:\n${result.stdout}\nstderr:\n${result.stderr}`,
  );
  assert.equal(
    result.status,
    0,
    `expected exit code 0 but got ${result.status}\nstdout:\n${result.stdout}\nstderr:\n${result.stderr}`,
  );
});

test('the home header activates any listed tenant through the runtime callback', async () => {
  const { HomeScreen } = await import('./screens/HomeScreen.tsx');
  hooks().__reset();
  const activated: string[] = [];
  const element = render(HomeScreen, {
    deploymentLabel: 'WeKnora',
    tenants: [{ id: '7', name: 'Acme' }, { id: '9', name: 'Beta' }],
    activeTenantId: '7',
    onActivateTenant: (id: string) => { activated.push(id); },
    onSignOut: async () => {},
    taskOffice: fakeTaskOffice({}),
  });

  const buttons = descendants(element).filter(({ type }) => type === 'Button').map(({ props }) => props.title);
  assert.deepEqual(buttons, ['Acme', 'Beta', 'Sign out', 'View all tasks', 'Open Resources', 'Load home'], 'with more than one tenant every tenant is a header switch button');
  const single = render(HomeScreen, {
    deploymentLabel: 'WeKnora',
    tenants: [{ id: '7', name: 'Acme' }],
    activeTenantId: '7',
    onActivateTenant: (id: string) => { activated.push(id); },
    onSignOut: async () => {},
    taskOffice: fakeTaskOffice({}),
  });
  const singleTexts = descendants(single).filter(({ type }) => type === 'Text').flatMap(({ props }) => props.children);
  assert.equal(singleTexts.includes('Acme'), true, 'a single tenant renders as plain text, not a switch button');
  const switchButton = descendants(element).find(({ type, props: button }) => type === 'Button' && button.title === 'Beta');
  assert.ok(switchButton, 'each listed tenant must render a switch button');
  (switchButton.props.onPress as () => void)();
  assert.deepEqual(activated, ['9']);
});

test('RuntimeSurface derives tenant options from the snapshot identity', async () => {
  const { RuntimeSurface } = await import('./composition.ts');
  const surface = RuntimeSurface({
    snapshot: {
      surface: 'authorized',
      deployment: { origin: 'https://weknora.example.test', label: 'WeKnora' },
      identity: { userId: 'member-1', activeTenantId: '7', tenants: [{ id: '7', name: 'Acme' }, { id: '9', name: 'Beta' }] },
    },
    onSignIn: async () => {}, onBeginOidc: async () => {}, onSignOut: async () => {},
    onActivateTenant: async () => {},
  });

  assert.equal(surface.type.name, 'HomeScreen');
  assert.deepEqual((surface.props as { tenants: Array<{ id: string; name?: string }> }).tenants, [
    { id: '7', name: 'Acme' },
    { id: '9', name: 'Beta' },
  ]);
  const tenantless = RuntimeSurface({
    snapshot: {
      surface: 'authorized',
      deployment: { origin: 'https://weknora.example.test', label: 'WeKnora' },
      identity: { userId: 'member-1', activeTenantId: '7' },
    },
    onSignIn: async () => {}, onBeginOidc: async () => {}, onSignOut: async () => {},
    onActivateTenant: async () => {},
  });
  assert.deepEqual((tenantless.props as { tenants: Array<{ id: string; name?: string }> }).tenants, [{ id: '7' }], 'a missing tenant list falls back to the active tenant id');
});

test('the resources screen renders only the Resource Shelf projection with explicit states', async () => {
  const { ResourcesScreen } = await import('./screens/ResourcesScreen.tsx');
  hooks().__reset();
  const base = {
    loading: false,
    onRefresh: () => {},
  };
  const element = render(ResourcesScreen, {
    ...base,
    page: {
      tenantId: '7',
      agents: [
        { id: 'agent-1', name: 'Research', summary: '', kind: 'custom', capability: { state: 'supported', reason: '' } },
        { id: 'agent-2', name: 'Blocked', summary: '', kind: 'custom', capability: { state: 'forbidden', reason: 'policy' } },
      ],
      knowledge: [{ id: 'kb-1', title: 'Handbook', scanStatus: 'indexed', documentCount: 3, updatedAt: '2026-09-01T00:00:00Z' }],
      connections: [{ id: 'conn-1', kind: 'personal', state: 'revoked', connected: false, capability: { state: 'unavailable', reason: 'connection_revoked' } }],
      classVerdicts: { agent: { state: 'supported', reason: '' }, knowledge: { state: 'supported', reason: '' }, connection: { state: 'supported', reason: '' } },
    },
  });
  const texts = descendants(element)
    .filter(({ type }) => type === 'Text')
    .flatMap(({ props }) => props.children)
    .flatMap((part) => (typeof part === 'string' ? [part] : []));

  assert.equal(texts.some((text) => text.includes('tenant 7')), true);
  assert.equal(texts.some((text) => text.includes('Research')), true);
  assert.equal(texts.some((text) => text.includes('Blocked') && text.includes('policy')), true, 'forbidden agents must explain their reason');
  assert.equal(texts.some((text) => text.includes('Handbook') && text.includes('已索引')), true);
  assert.equal(texts.some((text) => text.includes('connection_revoked')), true, 'connection state must be explained');
  const buttons = descendants(element).filter(({ type }) => type === 'Button').map(({ props }) => props.title);
  assert.equal(buttons.includes('Refresh'), true);

  const revokedElement = render(ResourcesScreen, {
    ...base,
    page: {
      tenantId: '7',
      agents: [],
      knowledge: [],
      connections: [],
      classVerdicts: { agent: { state: 'supported', reason: '' }, knowledge: { state: 'forbidden', reason: 'http_403' }, connection: { state: 'supported', reason: '' } },
    },
  });
  const revokedTexts = descendants(revokedElement)
    .filter(({ type }) => type === 'Text')
    .flatMap(({ props }) => props.children)
    .flatMap((part) => (typeof part === 'string' ? [part] : []));
  assert.equal(revokedTexts.some((text) => text.includes('Access revoked') && text.includes('http_403')), true, 'the forbidden class banner must show the reason');
  assert.equal(revokedTexts.some((text) => text.includes('Handbook')), false, 'revoked knowledge rows must disappear');
});

test('the resources route keeps an Expo Router screen consuming the shelf interface only', async () => {
  const route = await import('./app/resources.tsx');
  assert.equal(typeof route.default, 'function', 'src/app/resources.tsx must default-export the Expo Router screen');
});

test('the resources view modules never import contracts or api-client wire adapters', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  for (const relative of ['screens/ResourcesScreen.tsx', 'resources-view.ts', 'app/resources.tsx']) {
    const source = readFileSync(join(here, relative), 'utf8');
    assert.equal(/@weknora\/(api-client|contracts)/.test(source), false, `${relative} must consume the Resource Shelf Interface only (AC2)`);
  }
});

test('the resources route detaches its shelf controller from the long-lived handle on unmount', async () => {
  const { ResourcesScreen } = await import('./screens/ResourcesScreen.tsx');
  const { ResourcesRouteLifecycle } = await import('./app/resources.tsx');
  const page: import('@weknora/mobile-core').ResourcePage = {
    tenantId: '7',
    agents: [],
    knowledge: [],
    connections: [],
    classVerdicts: { agent: { state: 'supported', reason: '' }, knowledge: { state: 'supported', reason: '' }, connection: { state: 'supported', reason: '' } },
  };
  let browses = 0;
  const listeners = new Set<(event: import('@weknora/mobile-core').ShelfInvalidationEvent) => void>();
  const handle = {
    async browse() { browses += 1; return page; },
    selection: () => ({ allowed: false, state: 'unavailable', reason: 'not_used' }),
    subscribe(listener: (event: import('@weknora/mobile-core').ShelfInvalidationEvent) => void) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    close() {},
    emit(event: import('@weknora/mobile-core').ShelfInvalidationEvent) { for (const listener of [...listeners]) listener(event); },
    audit: () => ({ listeners: listeners.size, browses }),
  };
  hooks().__reset();
  const element = render(ResourcesRouteLifecycle, { handle });
  assert.equal(descendants(element).some(({ type }) => type === ResourcesScreen), true, 'authorized route renders the resources screen');
  hooks().__mount();
  assert.equal(handle.audit().listeners, 1, 'exactly one controller subscribes to the shelf handle while mounted');
  assert.equal(handle.audit().browses, 1, 'the mounted controller loads once');
  hooks().__unmount();
  assert.equal(handle.audit().listeners, 0, 'unmount must dispose the controller: the per-scope shelf handle keeps no leaked subscription');
  handle.emit({ type: 'authorization-revoked', resourceClass: 'knowledge' });
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.equal(handle.audit().browses, 1, 'an invalidation event after unmount must not trigger a leaked browse');
});

test('a discarded concurrent render leaves no shelf subscription or browse behind', async () => {
  const { ResourcesScreen } = await import('./screens/ResourcesScreen.tsx');
  const { ResourcesRouteLifecycle } = await import('./app/resources.tsx');
  const page: import('@weknora/mobile-core').ResourcePage = {
    tenantId: '7',
    agents: [],
    knowledge: [],
    connections: [],
    classVerdicts: { agent: { state: 'supported', reason: '' }, knowledge: { state: 'supported', reason: '' }, connection: { state: 'supported', reason: '' } },
  };
  let browses = 0;
  const listeners = new Set<(event: import('@weknora/mobile-core').ShelfInvalidationEvent) => void>();
  const handle = {
    async browse() { browses += 1; return page; },
    selection: () => ({ allowed: false, state: 'unavailable', reason: 'not_used' }),
    subscribe(listener: (event: import('@weknora/mobile-core').ShelfInvalidationEvent) => void) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    close() {},
    audit: () => ({ listeners: listeners.size, browses }),
  };
  hooks().__reset();
  // concurrent 渲染可以丢弃 render 相：这次 render 的 effect 永不提交（不调用 __mount）。
  const element = render(ResourcesRouteLifecycle, { handle });
  const screenElement = descendants(element).find(({ type }) => type === ResourcesScreen);
  assert.ok(screenElement, 'an authorized render still produces the resources screen');
  assert.equal((screenElement.props as { loading: boolean }).loading, true, 'before the effect commits, the route renders from the initial loading projection');
  assert.equal((screenElement.props as { page: unknown }).page, undefined);
  assert.deepEqual(handle.audit(), { listeners: 0, browses: 0 }, 'a discarded render must not subscribe to the shelf handle nor trigger a browse');
  hooks().__mount();
  assert.deepEqual(handle.audit(), { listeners: 1, browses: 1 }, 'the controller (subscription + first load) is created only when the effect commits');
  hooks().__unmount();
  assert.deepEqual(handle.audit(), { listeners: 0, browses: 1 }, 'unmount disposes the committed controller');
});

test('the resources route without a handle stays on the sign-in notice without touching the shelf', async () => {
  const { ResourcesScreen } = await import('./screens/ResourcesScreen.tsx');
  const { ResourcesRouteLifecycle } = await import('./app/resources.tsx');
  hooks().__reset();
  const element = render(ResourcesRouteLifecycle, {});
  const texts = descendants(element)
    .filter(({ type }) => type === 'Text')
    .flatMap(({ props }) => props.children)
    .flatMap((part) => (typeof part === 'string' ? [part] : []));
  assert.equal(texts.includes('Sign in to browse tenant resources.'), true);
  assert.equal(descendants(element).some(({ type }) => type === ResourcesScreen), false);
  hooks().__mount();
  hooks().__unmount();
});

function fakeTaskOffice(script: {
  homeView?: import('@weknora/mobile-core').HomeView;
  homeError?: Error;
  pages?: Array<import('@weknora/mobile-core').TaskListPage>;
  listError?: Error;
  archived?: string[];
}) {
  const calls: string[] = [];
  let page = 0;
  return {
    calls,
    async home() {
      calls.push('home');
      if (script.homeError) throw script.homeError;
      return script.homeView ?? { needsMe: [], running: [], recentlyCompleted: [], unreadNotifications: 0, asOf: '2026-09-23T00:00:00Z' };
    },
    async tasks(query: { search?: string; archived?: boolean; status?: string }) {
      calls.push(`tasks:${query.search ?? ''}:${query.archived ? 'archived' : 'active'}:${query.status ?? ''}`);
      if (script.listError) throw script.listError;
      return script.pages?.[0] ?? { items: [], duplicateRunIds: [] };
    },
    async moreTasks() {
      calls.push(`more:${page}`);
      page += 1;
      return script.pages?.[page] ?? { items: [], duplicateRunIds: [] };
    },
    async archive(taskId: string) { calls.push(`archive:${taskId}`); script.archived?.push(taskId); },
    async restore(taskId: string) { calls.push(`restore:${taskId}`); },
  };
}

test('HomeScreen renders the three aggregate segments, then error and retry states', async () => {
  hooks().__reset();
  const { HomeScreen } = await import('./screens/HomeScreen.tsx');
  const office = fakeTaskOffice({
    homeView: {
      needsMe: [{ interactionId: 'i1', kind: 'tool_approval', createdAt: '2026-09-23T00:00:00Z' }],
      running: [{ taskId: 't1', runId: 'r1', title: 'weekly report', runStatus: 'running', attention: 'none', updatedAt: '2026-09-23T00:00:00Z' }],
      recentlyCompleted: [{ taskId: 't2', runId: 'r2', title: 'research', runStatus: 'succeeded', attention: 'none', updatedAt: '2026-09-22T00:00:00Z' }],
      unreadNotifications: 4,
      asOf: '2026-09-23T00:00:01Z',
    },
  });
  const props = {
    deploymentLabel: 'WeKnora', tenants: [{ id: 'tenant-1', name: 'Acme' }], activeTenantId: 'tenant-1',
    onActivateTenant: () => {}, onSignOut: async () => {}, taskOffice: office as unknown as import('@weknora/mobile-core').TaskOffice,
  };
  let tree = render(HomeScreen, props);
  const textOf = (node: unknown): string => descendants(node).filter(({ type }) => type === 'Text').flatMap(({ props: p }) => p.children).join(' ');
  assert.equal(textOf(tree).includes('WeKnora'), true, 'the header renders before any data');
  const load = descendants(tree).find(({ type, props: button }) => type === 'Button' && button.title === 'Load home');
  (load!.props.onPress as () => void)();
  await new Promise((resolve) => setTimeout(resolve, 0));
  tree = render(HomeScreen, props);
  const text = textOf(tree);
  assert.equal(text.includes('weekly report'), true);
  assert.equal(text.includes('research'), true);
  assert.equal(text.includes('tool_approval'), true);
  assert.equal(text.includes('Unread 4'), true, 'the unread badge is visible');

  const failing = fakeTaskOffice({ homeError: new Error('TASK_OFFICE_BACKEND') });
  const errorProps = { ...props, taskOffice: failing as unknown as import('@weknora/mobile-core').TaskOffice };
  let errorTree = render(HomeScreen, errorProps);
  const retryLoad = descendants(errorTree).find(({ type, props: button }) => type === 'Button' && button.title === 'Load home');
  (retryLoad!.props.onPress as () => void)();
  await new Promise((resolve) => setTimeout(resolve, 0));
  errorTree = render(HomeScreen, errorProps);
  assert.equal(textOf(errorTree).includes('TASK_OFFICE_BACKEND'), true);
});

test('TasksScreen drives search, filters, archive and pagination through the module', async () => {
  hooks().__reset();
  const { TasksScreen } = await import('./screens/TasksScreen.tsx');
  const archived: string[] = [];
  const office = fakeTaskOffice({
    pages: [
      { items: [{ taskId: 't1', runId: 'r1', title: 'weekly report', runStatus: 'running', attention: 'required', updatedAt: '2026-09-23T00:00:00Z' }], nextCursor: 'c1', duplicateRunIds: [] },
      { items: [{ taskId: 't2', runId: 'r2', title: 'research', runStatus: 'succeeded', attention: 'none', updatedAt: '2026-09-22T00:00:00Z' }], duplicateRunIds: [] },
    ],
    archived,
  });
  const props = { taskOffice: office as unknown as import('@weknora/mobile-core').TaskOffice };
  let tree = render(TasksScreen, props);
  const search = descendants(tree).find(({ type }) => type === 'TextInput');
  (search!.props.onChangeText as (value: string) => void)('quarterly');
  tree = render(TasksScreen, props);
  const submit = descendants(tree).find(({ type, props: button }) => type === 'Button' && button.title === 'Search');
  (submit!.props.onPress as () => void)();
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(office.calls, ['tasks:quarterly:active:']);

  tree = render(TasksScreen, props);
  const more = descendants(tree).find(({ type, props: button }) => type === 'Button' && button.title === 'Load more');
  (more!.props.onPress as () => void)();
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(office.calls, ['tasks:quarterly:active:', 'more:0']);
  tree = render(TasksScreen, props);
  const text = descendants(tree).filter(({ type }) => type === 'Text').flatMap(({ props: p }) => p.children).join(' ');
  assert.equal(text.includes('weekly report'), true);
  assert.equal(text.includes('research'), true);

  const archive = descendants(render(TasksScreen, props)).find(({ type, props: button }) => type === 'Button' && button.title === 'Archive');
  (archive!.props.onPress as () => void)();
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(archived, ['t1']);
  assert.equal(office.calls.includes('tasks:quarterly:active:'), true, 'the list reloads through the module after archiving');

  const emptyOffice = fakeTaskOffice({});
  const emptyTree = render(TasksScreen, { taskOffice: emptyOffice as unknown as import('@weknora/mobile-core').TaskOffice });
  const emptySearch = descendants(emptyTree).find(({ type }) => type === 'TextInput');
  (emptySearch!.props.onChangeText as (value: string) => void)('nothing');
  const emptySubmit = descendants(render(TasksScreen, { taskOffice: emptyOffice as unknown as import('@weknora/mobile-core').TaskOffice })).find(({ type, props: button }) => type === 'Button' && button.title === 'Search');
  (emptySubmit!.props.onPress as () => void)();
  await new Promise((resolve) => setTimeout(resolve, 0));
  const emptyText = descendants(render(TasksScreen, { taskOffice: emptyOffice as unknown as import('@weknora/mobile-core').TaskOffice })).filter(({ type }) => type === 'Text').flatMap(({ props: p }) => p.children).join(' ');
  assert.equal(emptyText.includes('No tasks yet'), true, 'empty is an empty state, never a silent success');
});

test('the task detail route and screen consume the task office interface with evidence collapsed by default', async () => {
  const route = await import('./app/tasks/detail.tsx');
  assert.equal(typeof route.default, 'function', 'src/app/tasks/detail.tsx must default-export the detail route');
  assert.equal(typeof route.TaskDetailRouteLifecycle, 'function');
  const screen = await import('./screens/TaskDetailScreen.tsx');
  assert.equal(typeof screen.TaskDetailScreen, 'function');
  const view: import('@weknora/mobile-core').TaskDetailView = {
    taskId: 'task-1', runId: 'run-1', title: '报告', lifecycle: 'active', runStatus: 'running', attention: 'none',
    executionStatus: 'running', settlementStatus: 'pending', revision: 1, cursor: 2, incomplete: false, connection: 'live',
    timeline: [{ seq: 2, occurredAt: '2026-09-23T00:00:00Z', kind: 'tool_activity', type: 'tool.started', summary: '正在使用工具', evidence: { payload: { secretArgument: 'raw' } } }],
    duplicateSeqs: [],
  };
  const element = screen.TaskDetailScreen({ view, loading: false, onRefresh: () => {} });
  const json = JSON.stringify(element);
  assert.ok(json.includes('任务时间线'), 'the timeline section renders');
  assert.ok(!json.includes('secretArgument'), 'raw evidence stays collapsed by default');
});

test('the composition wires the task office detail port and detail files stay off wire adapters', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const composition = readFileSync(join(here, 'composition.ts'), 'utf8');
  assert.match(composition, /detail:\s*remote/, 'taskOfficeFor must pass the remote as the detail port; open() fails closed without it (T05)');
  for (const relative of ['screens/TaskDetailScreen.tsx', 'task-detail-view.ts', 'app/tasks/detail.tsx']) {
    const source = readFileSync(join(here, relative), 'utf8');
    assert.equal(/@weknora\/(api-client|contracts)/.test(source), false, `${relative} must consume the Task Office Interface only`);
  }
});

test('the login surface lists registered deployments and switches through the runtime callback', async () => {
  const { DeploymentLoginScreen } = await import('./screens/DeploymentLoginScreen.tsx');
  hooks().__reset();
  const switched: string[] = [];
  const props = {
    deployments: [{ origin: 'https://weknora.example.test', label: 'WeKnora' }, { origin: 'https://other.example.test', label: 'Other' }],
    onSignIn: async () => {},
    onBeginOidc: async () => {},
    onSwitchDeployment: async (origin: string) => { switched.push(origin); },
  };
  const element = render(DeploymentLoginScreen, props);
  const texts = descendants(element).filter(({ type }) => type === 'Text').flatMap(({ props: p }) => p.children).flatMap((part) => (typeof part === 'string' ? [part] : []));
  assert.equal(texts.includes('Registered deployments'), true, 'the registered instance list is visible before manual origin entry');
  const other = descendants(element).find(({ type, props: p }) => type === 'Button' && p.title === 'Other');
  assert.ok(other, 'each registered instance renders a switch button');
  (other!.props.onPress as () => void)();
  assert.deepEqual(switched, ['https://other.example.test']);
});

test('the home header switches to another registered deployment through the callback', async () => {
  const { HomeScreen } = await import('./screens/HomeScreen.tsx');
  hooks().__reset();
  const switched: string[] = [];
  const element = render(HomeScreen, {
    deploymentLabel: 'WeKnora',
    tenants: [{ id: '7', name: 'Acme' }],
    activeTenantId: '7',
    onActivateTenant: () => {},
    onSignOut: async () => {},
    taskOffice: fakeTaskOffice({}),
    otherDeployments: [{ origin: 'https://other.example.test', label: 'Other' }],
    onSwitchDeployment: async (origin: string) => { switched.push(origin); },
  });
  const button = descendants(element).find(({ type, props: p }) => type === 'Button' && p.title === 'Switch to Other');
  assert.ok(button, 'another registered deployment must render a switch button on the authorized surface');
  (button!.props.onPress as () => void)();
  assert.deepEqual(switched, ['https://other.example.test']);
});

test('RuntimeSurface passes other registered deployments to the home screen and the full list to login', async () => {
  const { RuntimeSurface } = await import('./composition.ts');
  const authorized = RuntimeSurface({
    snapshot: { surface: 'authorized', deployment: { origin: 'https://weknora.example.test', label: 'WeKnora' }, identity: { userId: 'member-1', activeTenantId: '7', tenants: [{ id: '7' }] } },
    deployments: [{ origin: 'https://weknora.example.test', label: 'WeKnora' }, { origin: 'https://other.example.test', label: 'Other' }],
    onSignIn: async () => {}, onBeginOidc: async () => {}, onSignOut: async () => {}, onActivateTenant: async () => {},
    onSwitchDeployment: async () => {},
  });
  assert.equal(authorized.type.name, 'HomeScreen');
  assert.deepEqual((authorized.props as { otherDeployments?: Array<{ origin: string; label: string }> }).otherDeployments, [{ origin: 'https://other.example.test', label: 'Other' }]);

  const login = RuntimeSurface({
    snapshot: { surface: 'deployment-login' },
    deployments: [{ origin: 'https://other.example.test', label: 'Other' }],
    onSignIn: async () => {}, onBeginOidc: async () => {}, onSignOut: async () => {}, onActivateTenant: async () => {},
    onSwitchDeployment: async () => {},
  });
  assert.equal(login.type.name, 'DeploymentLoginScreen');
  assert.deepEqual((login.props as { deployments?: Array<{ origin: string; label: string }> }).deployments, [{ origin: 'https://other.example.test', label: 'Other' }]);
});

test('RuntimeSurface routes the read-only snapshot to a restricted explanation surface', async () => {
  const { RuntimeSurface } = await import('./composition.ts');
  hooks().__reset();
  const surface = RuntimeSurface({
    snapshot: { surface: 'read-only', deployment: { origin: 'https://weknora.example.test', label: 'WeKnora' }, identity: { userId: 'member-1', activeTenantId: '7' }, reason: 'protocol-mismatch' },
    onSignIn: async () => {}, onBeginOidc: async () => {}, onSignOut: async () => {}, onActivateTenant: async () => {},
  });
  assert.equal(surface.type.name, 'ReadOnlyScreen');
  const element = render(surface.type, surface.props);
  const texts = descendants(element).filter(({ type }) => type === 'Text').flatMap(({ props: p }) => p.children).flatMap((part) => (typeof part === 'string' ? [part] : []));
  assert.equal(texts.some((text) => text.includes('Limited read-only mode')), true);
  assert.equal(texts.some((text) => text.includes('behind this version')), true);
  assert.equal(texts.some((text) => text.includes('Read-only browsing is unavailable.')), true, 'without a shelf handle the surface says so instead of guessing content');
  const buttons = descendants(element).filter(({ type }) => type === 'Button').map(({ props: p }) => p.title);
  assert.equal(buttons.includes('Sign out'), true);
  assert.equal(buttons.includes('View all tasks'), false, 'read-only must not expose task controls');
});

test('ReadOnlyScreen mounts the browse-only shelf once and renders its projection without authorized controls', async () => {
  const { ReadOnlyScreen } = await import('./screens/ReadOnlyScreen.tsx');
  const { ResourcesScreen } = await import('./screens/ResourcesScreen.tsx');
  hooks().__reset();
  const page: import('@weknora/mobile-core').ResourcePage = {
    tenantId: '7',
    agents: [{ id: 'agent-1', name: 'Research', summary: '', kind: 'custom', capability: { state: 'supported', reason: '' } }],
    knowledge: [],
    connections: [],
    classVerdicts: { agent: { state: 'supported', reason: '' }, knowledge: { state: 'supported', reason: '' }, connection: { state: 'supported', reason: '' } },
  };
  let browses = 0;
  const listeners = new Set<(event: import('@weknora/mobile-core').ShelfInvalidationEvent) => void>();
  const handle = {
    async browse() { browses += 1; return page; },
    selection: () => ({ allowed: false as const, state: 'unavailable' as const, reason: 'not_used' }),
    subscribe(listener: (event: import('@weknora/mobile-core').ShelfInvalidationEvent) => void) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    close() {},
  };
  const element = render(ReadOnlyScreen, { deploymentLabel: 'WeKnora', handle: handle as import('@weknora/mobile-core').ResourceShelfHandle, onSignOut: async () => {} });
  assert.equal(descendants(element).some(({ type }) => type === ResourcesScreen), true, 'the read-only surface renders the resource projection');
  hooks().__mount();
  assert.equal(browses, 1, 'the read-only shelf loads exactly once on mount');
  hooks().__unmount();
  assert.equal(listeners.size, 0, 'unmount detaches the shelf subscription');
});

test('the deployment list sync ignores out-of-order completions', async () => {
  const { createDeploymentListSync } = await import('./composition.ts');
  const received: unknown[][] = [];
  const setDeployments = (value: unknown) => { received.push(JSON.parse(JSON.stringify(value))); };
  let releaseFirst: (() => void) | undefined;
  let calls = 0;
  const runtime = {
    listDeployments: (): Promise<unknown[]> => {
      calls += 1;
      if (calls === 1) return new Promise((resolve) => { releaseFirst = () => resolve([{ origin: 'https://a.example.test', label: 'A' }]); });
      return Promise.resolve([{ origin: 'https://b.example.test', label: 'B' }]);
    },
  };
  const sync = createDeploymentListSync(runtime as never, setDeployments as never);
  sync();                                                    // 第一次读取：挂起（模拟慢 SecureStore）
  sync();                                                    // 第二次读取：立即完成 → setDeployments(B)
  await new Promise((resolve) => setImmediate(resolve));
  releaseFirst?.();                                          // 旧响应迟到完成
  await new Promise((resolve) => setImmediate(resolve));
  assert.deepEqual(received, [[{ origin: 'https://b.example.test', label: 'B' }]], '只有最新一次读取的结果生效；迟到的旧结果被序号守卫丢弃');
});

test('the deployment list sync swallows read failures', async () => {
  const { createDeploymentListSync } = await import('./composition.ts');
  const received: unknown[][] = [];
  const runtime = { listDeployments: () => Promise.reject(new Error('SECURESTORE_UNAVAILABLE')) };
  const sync = createDeploymentListSync(runtime as never, ((value: unknown) => { received.push(value as never); }) as never);
  sync();
  await new Promise((resolve) => setImmediate(resolve));
  assert.deepEqual(received, [], '读取失败维持现状且不形成 unhandled rejection');
});

test('MobileApp subscribes deployment list reads with containment and narrowed deps', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const source = readFileSync(join(here, 'composition.ts'), 'utf8');
  assert.match(source, /createDeploymentListSync/);
  assert.match(source, /snapshot\.surface, snapshot\.deployment\?\.origin/, '依赖收窄：不再以整快照对象触发重复读取安全存储');
});

test('switching deployment from the login screen resets the form onto the target origin', async () => {
  const { DeploymentLoginScreen } = await import('./screens/DeploymentLoginScreen.tsx');
  hooks().__reset();
  const switched: string[] = [];
  const props = {
    officialCloudOrigin: 'https://cloud.example.test',
    deployments: [{ origin: 'https://selfhost.example.test', label: 'Selfhost' }],
    onSignIn: async () => {},
    onBeginOidc: async () => {},
    onSwitchDeployment: async (origin: string) => { switched.push(origin); },
  };
  let tree = render(DeploymentLoginScreen, props);
  // 预置本地状态：用户先在表单里输入了另一个 origin/凭据（经 TextInput 的 onChangeText 写入 stub state）
  const textInputs = descendants(tree).filter(({ type }) => type === 'TextInput');
  (textInputs[0]!.props.onChangeText as (value: string) => void)('https://stale.example.test');
  (textInputs[1]!.props.onChangeText as (value: string) => void)('user@stale.example.test');
  (textInputs[2]!.props.onChangeText as (value: string) => void)('stale-password');
  const switchButton = descendants(tree).find(({ type, props: p }) => type === 'Button' && p.title === 'Selfhost');
  (switchButton!.props.onPress as () => void)();
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(switched, ['https://selfhost.example.test']);
  // 重渲染：状态经 stub 保留，origin 输入框现在显示目标 origin（而非残留的 stale）
  tree = render(DeploymentLoginScreen, props);
  const rerenderedInputs = descendants(tree).filter(({ type }) => type === 'TextInput');
  assert.equal((rerenderedInputs[0]!.props as { value?: string }).value, 'https://selfhost.example.test', '切换后表单 origin 必须指向目标实例');
  assert.equal((rerenderedInputs[1]!.props as { value?: string }).value, '', 'email 残留必须清空');
  assert.equal((rerenderedInputs[2]!.props as { value?: string }).value, '', 'password 残留必须清空');
});

test('the registered deployments block is hidden without an onSwitchDeployment handler', async () => {
  const { DeploymentLoginScreen } = await import('./screens/DeploymentLoginScreen.tsx');
  hooks().__reset();
  const tree = render(DeploymentLoginScreen, {
    deployments: [{ origin: 'https://a.example.test', label: 'A' }],
    onSignIn: async () => {},
    onBeginOidc: async () => {},
  });
  const texts = descendants(tree).filter(({ type }) => type === 'Text').flatMap(({ props: p }) => p.children).flatMap((part) => (typeof part === 'string' ? [part] : []));
  assert.equal(texts.includes('Registered deployments'), false, 'handler 缺失：区块不渲染（无反馈死交互不出现）');
  assert.equal(descendants(tree).some(({ type, props: p }) => type === 'Button' && p.title === 'A'), false);
});

test('the home header hides switch entries without an onSwitchDeployment handler', async () => {
  const { HomeScreen } = await import('./screens/HomeScreen.tsx');
  hooks().__reset();
  const element = render(HomeScreen, {
    deploymentLabel: 'WeKnora',
    tenants: [{ id: '7', name: 'Acme' }],
    activeTenantId: '7',
    onActivateTenant: () => {},
    onSignOut: async () => {},
    taskOffice: fakeTaskOffice({}),
    otherDeployments: [{ origin: 'https://other.example.test', label: 'Other' }],
    // onSwitchDeployment 缺失
  });
  assert.equal(descendants(element).some(({ type, props: p }) => type === 'Button' && p.title === 'Switch to Other'), false, '无 handler 时不渲染 Switch to … 按钮');
});
