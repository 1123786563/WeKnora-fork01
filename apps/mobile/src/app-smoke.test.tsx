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
  'expo-router': "const focusEffects = []; module.exports = { Stack: function Stack() { return null; }, router: { replace() {}, push() {} }, useLocalSearchParams() { return {}; }, useFocusEffect(effect) { focusEffects.push(effect); }, __focusEffects: focusEffects }",
  'expo-secure-store': "module.exports = { getItemAsync: async () => null, setItemAsync: async () => {}, deleteItemAsync: async () => {} }",
  'expo-web-browser': "module.exports = { openAuthSessionAsync: async () => ({ type: 'dismiss' }) }",
  'react-native-safe-area-context': "module.exports = { initialWindowMetrics: null, SafeAreaProvider: function SafeAreaProvider(p) { return p.children; }, SafeAreaView: function SafeAreaView(p) { return p.children; } }",
  'react-native': "module.exports = { View: 'View', Text: 'Text', TextInput: 'TextInput', Button: 'Button', ScrollView: 'ScrollView', Image: 'Image', Switch: 'Switch' }",
  react: "let values = []; let cursor = 0; let pendingEffects = []; let effectCleanups = []; module.exports = { __beginRender() { cursor = 0; }, __reset() { values = []; cursor = 0; pendingEffects = []; effectCleanups = []; }, __values() { return values; }, __snapshot() { return { values: [...values], cursor, pendingEffects: [...pendingEffects], effectCleanups: [...effectCleanups] }; }, __restore(snapshot) { values = [...snapshot.values]; cursor = snapshot.cursor; pendingEffects = [...snapshot.pendingEffects]; effectCleanups = [...snapshot.effectCleanups]; }, useState(initial) { const index = cursor++; if (!(index in values)) values[index] = initial; return [values[index], (next) => { values[index] = typeof next === 'function' ? next(values[index]) : next; }]; }, useRef(value) { const index = cursor++; if (!(index in values)) values[index] = { current: value }; return values[index]; }, useEffect(setup) { pendingEffects.push(setup); }, useCallback(callback) { cursor++; return callback; }, __mount() { for (const setup of pendingEffects.splice(0)) effectCleanups.push(setup()); }, __unmount() { for (const cleanup of effectCleanups.splice(0)) { if (typeof cleanup === 'function') cleanup(); } }, useSyncExternalStore(_subscribe, getSnapshot) { return getSnapshot(); }, createElement(type, props, ...children) { return { type, props: { ...(props || {}), ...(children.length === 0 ? {} : { children: children.length === 1 ? children[0] : children }) } }; } };",
  'react/jsx-runtime': "const jsx = (type, props, key) => ({ type, props: { ...(props || {}), ...(key === undefined ? {} : { key }) } }); module.exports = { Fragment: Symbol.for('react.fragment'), jsx, jsxs: jsx };",
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

function hooks(): { __beginRender(): void; __reset(): void; __mount(): void; __unmount(): void; __values(): unknown[]; __snapshot(): any; __restore(snapshot: any): void } {
  return require('react') as { __beginRender(): void; __reset(): void; __mount(): void; __unmount(): void; __values(): unknown[]; __snapshot(): any; __restore(snapshot: any): void };
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

test('Task Office is a registered Expo file route reachable from Tasks', async () => {
  const route = await import('./app/task-office.tsx');
  assert.equal(typeof route.default, 'function');

  const { TasksScreen } = await import('./screens/TasksScreen.tsx');
  hooks().__reset();
  let opened = 0;
  const screen = render(TasksScreen, {
    taskOffice: { tasks: async () => ({ items: [], duplicateRunIds: [] }) },
    onOpenTaskOffice: () => { opened += 1; },
  });
  const officeButton = descendants(screen).find(({ type, props }) => type === 'Button' && props.title === 'Task Office');
  assert.ok(officeButton, 'authorized Tasks screen exposes the Task Office entry');
  (officeButton!.props.onPress as () => void)();
  assert.equal(opened, 1);
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
  assert.equal(homeButtons.includes('Open Approvals'), true, 'the authorized home keeps a resident attention inbox entry (T08, relocated to /attention during #41 integration)');
  assert.equal(homeButtons.includes('Open Inbox'), true, 'the authorized home keeps the notification inbox entry (#41)');
  const homeText = descendants(homeElement).filter(({ type }) => type === 'Text').flatMap(({ props }) => props.children).join(' ');
  assert.equal(homeText.includes('Acme'), true);
  assert.equal((authorized.props as { key?: string }).key, 'https://weknora.example.test::tenant-1', 'the home screen is keyed by deployment origin + active tenant');
  const switched = RuntimeSurface({
    snapshot: { surface: 'authorized', deployment: { origin: 'https://weknora.example.test', label: 'WeKnora' }, identity: { userId: 'member-1', activeTenantId: 'tenant-2', tenants: [{ id: 'tenant-2', name: 'Beta' }] } },
    onSignIn: async () => {}, onBeginOidc: async () => {}, onSignOut: async () => {}, onActivateTenant: async () => {},
  });
  assert.equal((switched.props as { key?: string }).key, 'https://weknora.example.test::tenant-2', 'switching tenants must remount the screen so the previous tenant content cannot survive');
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

test('switching deployments with equal tenant ids remounts Home and Tasks with fresh closures (R1-F48/F49)', async () => {
  const { RuntimeSurface, deploymentScopeKey } = await import('./composition.ts');
  const props = (origin: string) => ({
    snapshot: {
      surface: 'authorized', reason: undefined,
      deployment: { origin, label: origin },
      identity: { userId: 'user-1', activeTenantId: '1', tenants: [{ id: '1' }] },
    },
    onSignIn: async () => {}, onBeginOidc: async () => {}, onSignOut: async () => {}, onActivateTenant: async () => {},
  } as never);
  const first = RuntimeSurface(props('https://a.example.test'));
  const second = RuntimeSurface(props('https://b.example.test'));
  // 两部署租户 id 相同（自增小整数常见）：key 必须因 origin 维度而不同
  assert.notEqual((first.props as { key?: unknown }).key, (second.props as { key?: unknown }).key);
  assert.notEqual(deploymentScopeKey('https://a.example.test', '1'), deploymentScopeKey('https://b.example.test', '1'));
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
  // 集成合并：HEAD（T08 New task + Open Inbox）∪ t41（#41 Open Inbox）——两分支并行各自加
  // 入口导致 Open Inbox 重复；集成分流后审批收件箱迁至 /attention（Open Approvals，
  // T08 常驻入口），行动通知收件箱留守 /inbox（Open Inbox，#41）。
  assert.deepEqual(buttons, ['Acme', 'Beta', 'Sign out', 'New task', 'Ask knowledge', 'View all tasks', 'Open Approvals', 'Open Inbox', 'Open Resources', 'Load home'], 'with more than one tenant every tenant is a header switch button');  const single = render(HomeScreen, {
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

test('the /tasks root shell renders a sign-in gate instead of a blank page when unauthorized (B3 recheck)', async () => {
  const route = await import('./app/tasks.tsx');
  assert.equal(typeof route.default, 'function', 'src/app/tasks.tsx must default-export the /tasks route');
  const { MobileTasks } = await import('./composition.ts');
  const { TasksScreen } = await import('./screens/TasksScreen.tsx');
  hooks().__reset();
  // tasks.tsx 无条件委托 MobileTasks（app/tasks.tsx:6-11），桩环境 Runtime 初始面为
  // deployment-login（mobile-runtime.ts:108）：下方渲染结果即 /tasks 未授权态的真实产物。
  const element = render(MobileTasks, {});
  const texts = descendants(element)
    .filter(({ type }) => type === 'Text')
    .flatMap(({ props: p }) => p.children)
    .flatMap((part) => (typeof part === 'string' ? [part] : []));
  assert.equal(texts.includes('请先登录并激活空间，再查看任务列表。'), true, 'unauthorized /tasks must state the sign-in gate, not render a blank shell');
  assert.equal(descendants(element).some(({ type }) => type === TasksScreen), false, 'the tasks list stays unreachable without an authorized surface');
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
  assert.match(composition, /commands:\s*remote/, 'taskOfficeFor must wire the command channel; handle.act() fails closed (TASK_OFFICE_COMMAND_UNAVAILABLE) without it (T07 #37)');
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

test('the delivery receipt section offers the recovery action for partial and unknown states only (T25 #55 AC1)', async () => {
  const { TaskDetailScreen } = await import('./screens/TaskDetailScreen.tsx');
  const view: import('@weknora/mobile-core').TaskDetailView = {
    taskId: 'task-1', runId: 'run-1', title: '交付', lifecycle: 'active', runStatus: 'running', attention: 'required',
    executionStatus: 'running', settlementStatus: 'pending', revision: 1, cursor: 2, incomplete: false, connection: 'live',
    timeline: [], duplicateSeqs: [],
  };
  const pushedReceipt: import('@weknora/mobile-core').DeliveryReceiptView = {
    deliveryId: 'dlv-1', taskId: 'task-1', runId: 'run-1', state: 'pushed',
    repo: 'octocat/hello', branch: 'weknora/task/s-1', baselineSha: 'b'.repeat(40),
    commitSha: 'c1', attention: true, updatedAt: '2026-09-24T00:00:30Z',
  };
  const recoveredInput: Array<{ runId: string; deliveryId: string }> = [];
  const withRecovery = render(TaskDetailScreen, {
    view, loading: false, error: undefined, onRefresh: () => {},
    delivery: pushedReceipt, onRecoverDelivery: async (input: { runId: string; deliveryId: string }) => { recoveredInput.push(input); return pushedReceipt; },
  });
  const pushedSection = descendants(withRecovery).find(({ type, props }) => typeof type === 'function' && 'delivery' in props);
  assert.ok(pushedSection, 'delivery section is present');
  const pushedJson = JSON.stringify(render(pushedSection!.type as (props: unknown) => unknown, pushedSection!.props));
  assert.ok(pushedJson.includes('已推送，等待草稿 PR/MR 恢复'), 'the honest partial-completion copy renders');
  assert.ok(pushedJson.includes('恢复创建草稿 PR/MR'), 'the pushed state offers the recovery action');
  const recoverButton = descendants(render(pushedSection!.type as (props: unknown) => unknown, pushedSection!.props))
    .find(({ type, props }) => type === 'Button' && props.title === '恢复创建草稿 PR/MR');
  assert.ok(recoverButton, 'the actual pushed recovery button is rendered');
  (recoverButton!.props.onPress as () => void)();
  assert.deepEqual(recoveredInput, [{ runId: 'run-1', deliveryId: 'dlv-1' }], 'pressing the action forwards the exact route run and receipt id');
  const failedSection = render(TaskDetailScreen, {
    view, loading: false, error: undefined, onRefresh: () => {},
    delivery: pushedReceipt, recoveryError: '当前状态无法恢复：服务端暂不可用', onRecoverDelivery: async () => pushedReceipt,
  });
  const failedProps = descendants(failedSection).find(({ type, props }) => typeof type === 'function' && 'delivery' in props);
  assert.ok(failedProps, 'failed recovery delivery section is present');
  const failedJson = JSON.stringify(render(failedProps!.type as (props: unknown) => unknown, failedProps!.props));
  assert.ok(failedJson.includes('当前状态无法恢复：服务端暂不可用'), 'recovery failure copy renders');
  assert.ok(failedJson.includes('已推送，等待草稿 PR/MR 恢复'), 'recovery failure does not claim delivery completed');
  const unknownSection = render(TaskDetailScreen, {
    view, loading: false, error: undefined, onRefresh: () => {},
    delivery: { ...pushedReceipt, state: 'unknown' }, onRecoverDelivery: async () => pushedReceipt,
  });
  const unknownProps = descendants(unknownSection).find(({ type, props }) => typeof type === 'function' && 'delivery' in props);
  assert.ok(unknownProps, 'unknown delivery section is present');
  assert.ok(JSON.stringify(render(unknownProps!.type as (props: unknown) => unknown, unknownProps!.props)).includes('核对远端结果'), 'unknown state offers the remote verification action');
  const recovered: import('@weknora/mobile-core').DeliveryReceiptView = { ...pushedReceipt, state: 'delivered' };
  const settled = render(TaskDetailScreen, {
    view, loading: false, error: undefined, onRefresh: () => {}, delivery: recovered,
  });
  const settledSection = descendants(settled).find(({ type, props }) => typeof type === 'function' && 'delivery' in props);
  assert.ok(settledSection, 'delivered delivery section is present');
  assert.equal(JSON.stringify(render(settledSection!.type as (props: unknown) => unknown, settledSection!.props)).includes('恢复创建草稿 PR/MR'), false, 'a delivered receipt offers no recovery action');
  const noCallback = render(TaskDetailScreen, {
    view, loading: false, error: undefined, onRefresh: () => {}, delivery: pushedReceipt,
  });
  const noCallbackSection = descendants(noCallback).find(({ type, props }) => typeof type === 'function' && 'delivery' in props);
  assert.ok(noCallbackSection, 'delivery section without callback is present');
  assert.equal(JSON.stringify(render(noCallbackSection!.type as (props: unknown) => unknown, noCallbackSection!.props)).includes('恢复创建草稿 PR/MR'), false, 'without the callback the action stays hidden (fail closed)');
});

test('route-bound delivery recovery ignores stale completions and applies current success and failure (T25 #55 review repair)', async () => {
  const { createRouteBoundDeliveryRecoveryHandler } = await import('./app/tasks/detail.tsx');
  type Identity = { taskId: string; runId: string };
  type Receipt = import('@weknora/mobile-core').DeliveryReceiptView;
  const a: Identity = { taskId: 'task-a', runId: 'run-a' };
  const b: Identity = { taskId: 'task-b', runId: 'run-b' };
  let generation = 0;
  const receiptA: Receipt = {
    deliveryId: 'dlv-a', taskId: a.taskId, runId: a.runId, state: 'pushed', repo: 'octocat/hello',
    branch: 'weknora/task/a', baselineSha: 'a'.repeat(40), attention: true, updatedAt: '2026-09-24T00:00:30Z',
  };
  const receiptB: Receipt = { ...receiptA, deliveryId: 'dlv-b', taskId: b.taskId, runId: b.runId, state: 'pushed' };
  function deferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (error: unknown) => void;
    const promise = new Promise<T>((resolvePromise, rejectPromise) => { resolve = resolvePromise; reject = rejectPromise; });
    return { promise, resolve, reject };
  }

  let current: Identity = a;
  const appliedReceipts: Receipt[] = [];
  const errors: Array<string | undefined> = [];
  const staleSuccess = deferred<Receipt>();
  const staleSuccessHandler = createRouteBoundDeliveryRecoveryHandler({
    entry: { identity: a, generation: 0 }, receipt: receiptA,
    currentEntry: () => ({ identity: current, generation }), recover: () => staleSuccess.promise,
    setDelivery: (receipt: Receipt) => { appliedReceipts.push(receipt); }, setRecoveryError: (error?: string) => { errors.push(error); },
    errorCopy: (error: unknown) => String(error),
  });
  const staleSuccessPending = staleSuccessHandler({ runId: a.runId, deliveryId: receiptA.deliveryId });
  current = b;
  generation = 1;
  errors.splice(0, errors.length, 'route B error');
  staleSuccess.resolve({ ...receiptA, state: 'delivered' });
  await staleSuccessPending;
  assert.deepEqual(appliedReceipts.slice(), [], 'late success from route A cannot replace route B receipt');
  assert.deepEqual(errors.slice(), ['route B error'], 'late success from route A cannot clear or set route B error');

  const staleFailure = deferred<Receipt>();
  const staleFailureHandler = createRouteBoundDeliveryRecoveryHandler({
    entry: { identity: a, generation: 0 }, receipt: receiptA,
    currentEntry: () => ({ identity: current, generation }), recover: () => staleFailure.promise,
    setDelivery: (receipt: Receipt) => { appliedReceipts.push(receipt); }, setRecoveryError: (error?: string) => { errors.push(error); },
    errorCopy: (error: unknown) => `failure:${String(error)}`,
  });
  current = a;
  generation = 0;
  const staleFailurePending = staleFailureHandler({ runId: a.runId, deliveryId: receiptA.deliveryId });
  current = b;
  generation = 1;
  errors.splice(0, errors.length, 'route B error');
  staleFailure.reject(new Error('late A failure'));
  await assert.rejects(staleFailurePending, /late A failure/);
  assert.deepEqual(appliedReceipts.slice(), [], 'late failure does not mutate route B receipt');
  assert.deepEqual(errors.slice(), ['route B error'], 'late failure does not leak route A error into route B');

  const success = deferred<Receipt>();
  const currentHandler = createRouteBoundDeliveryRecoveryHandler({
    entry: { identity: b, generation: 1 }, receipt: receiptB,
    currentEntry: () => ({ identity: current, generation }), recover: () => success.promise,
    setDelivery: (receipt: Receipt) => { appliedReceipts.push(receipt); }, setRecoveryError: (error?: string) => { errors.push(error); },
    errorCopy: (error: unknown) => String(error),
  });
  const successPending = currentHandler({ runId: b.runId, deliveryId: receiptB.deliveryId });
  success.resolve({ ...receiptB, state: 'delivered' });
  await successPending;
  assert.deepEqual(appliedReceipts.slice(), [{ ...receiptB, state: 'delivered' }], 'current route success updates its matching receipt');
  assert.deepEqual(errors.slice(), ['route B error', undefined], 'current route recovery clears its prior error');

  const failed = deferred<Receipt>();
  const failingHandler = createRouteBoundDeliveryRecoveryHandler({
    entry: { identity: b, generation: 1 }, receipt: receiptB,
    currentEntry: () => ({ identity: current, generation }), recover: () => failed.promise,
    setDelivery: (receipt: Receipt) => appliedReceipts.push(receipt), setRecoveryError: (error?: string) => errors.push(error),
    errorCopy: (error: unknown) => `failure:${String(error)}`,
  });
  const failedPending = failingHandler({ runId: b.runId, deliveryId: receiptB.deliveryId });
  failed.reject(new Error('backend unavailable'));
  await assert.rejects(failedPending, /backend unavailable/);
  assert.deepEqual(appliedReceipts.slice(), [{ ...receiptB, state: 'delivered' }], 'current route failure leaves the last receipt unchanged');
  assert.equal(errors.at(-1), 'failure:Error: backend unavailable', 'current route failure produces visible error copy');
});

test('a route re-entry gets a new generation so stale A reads and recoveries cannot update A again', async () => {
  const { createRouteBoundDeliveryReadHandler, createRouteBoundDeliveryRecoveryHandler } = await import('./app/tasks/detail.tsx');
  type Identity = { taskId: string; runId: string };
  type Receipt = import('@weknora/mobile-core').DeliveryReceiptView;
  const a: Identity = { taskId: 'task-a', runId: 'run-a' };
  const b: Identity = { taskId: 'task-b', runId: 'run-b' };
  const receiptA: Receipt = {
    deliveryId: 'dlv-a', taskId: a.taskId, runId: a.runId, state: 'pushed', repo: 'octocat/hello',
    branch: 'weknora/task/a', baselineSha: 'a'.repeat(40), attention: true, updatedAt: '2026-09-24T00:00:30Z',
  };
  const currentDeliveredA: Receipt = { ...receiptA, state: 'delivered' };
  function deferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (error: unknown) => void;
    const promise = new Promise<T>((resolvePromise, rejectPromise) => { resolve = resolvePromise; reject = rejectPromise; });
    return { promise, resolve, reject };
  }

  let current: Identity = a;
  let generation = 0;
  const receipts: Receipt[] = [];
  const errors: Array<string | undefined> = [];
  const routeEntry = () => ({ identity: current, generation });
  const oldRead = deferred<Receipt>();
  const readHandler = createRouteBoundDeliveryReadHandler({
    entry: { identity: a, generation: 0 }, currentEntry: routeEntry,
    setDelivery: (receipt: Receipt) => { receipts.push(receipt); },
  });
  const readPending = oldRead.promise.then(readHandler);
  current = b; generation = 1;
  current = a; generation = 2;
  oldRead.resolve(receiptA);
  await readPending;
  assert.deepEqual(receipts.slice(), [], 'a stale A read cannot populate the later A visit');

  const staleSuccess = deferred<Receipt>();
  current = a; generation = 0;
  const staleSuccessHandler = createRouteBoundDeliveryRecoveryHandler({
    entry: { identity: a, generation: 0 }, receipt: receiptA, currentEntry: routeEntry,
    recover: () => staleSuccess.promise,
    setDelivery: (receipt: Receipt) => { receipts.push(receipt); }, setRecoveryError: (error?: string) => { errors.push(error); },
    errorCopy: (error: unknown) => String(error),
  });
  const staleSuccessPending = staleSuccessHandler({ runId: a.runId, deliveryId: receiptA.deliveryId });
  current = b; generation = 1;
  current = a; generation = 2;
  errors.splice(0, errors.length, 'current A error');
  staleSuccess.resolve(currentDeliveredA);
  await staleSuccessPending;
  assert.deepEqual(receipts.slice(), [], 'a stale recovery success cannot replace the later A receipt');
  assert.deepEqual(errors.slice(), ['current A error'], 'a stale success cannot clear the later A error');

  const staleFailure = deferred<Receipt>();
  current = a; generation = 0;
  const staleFailureHandler = createRouteBoundDeliveryRecoveryHandler({
    entry: { identity: a, generation: 0 }, receipt: receiptA, currentEntry: routeEntry,
    recover: () => staleFailure.promise,
    setDelivery: (receipt: Receipt) => { receipts.push(receipt); }, setRecoveryError: (error?: string) => { errors.push(error); },
    errorCopy: (error: unknown) => `failure:${String(error)}`,
  });
  const staleFailurePending = staleFailureHandler({ runId: a.runId, deliveryId: receiptA.deliveryId });
  current = b; generation = 1;
  current = a; generation = 2;
  errors.splice(0, errors.length, 'current A error');
  staleFailure.reject(new Error('old A failure'));
  await assert.rejects(staleFailurePending, /old A failure/);
  assert.deepEqual(receipts.slice(), [], 'a stale recovery failure leaves the later A receipt unchanged');
  assert.deepEqual(errors.slice(), ['current A error'], 'a stale failure cannot replace the later A error');

  const currentSuccess = deferred<Receipt>();
  const currentHandler = createRouteBoundDeliveryRecoveryHandler({
    entry: { identity: a, generation: 2 }, receipt: receiptA, currentEntry: routeEntry,
    recover: () => currentSuccess.promise,
    setDelivery: (receipt: Receipt) => { receipts.push(receipt); }, setRecoveryError: (error?: string) => { errors.push(error); },
    errorCopy: (error: unknown) => String(error),
  });
  const currentPending = currentHandler({ runId: a.runId, deliveryId: receiptA.deliveryId });
  currentSuccess.resolve(currentDeliveredA);
  await currentPending;
  assert.deepEqual(receipts.slice(), [currentDeliveredA], 'the current A entry can still update its receipt');
});

test('the mounted detail route invalidates deferred work across focus blur and refocus with unchanged A params', async () => {
  const { TaskDetailRouteLifecycle, createRouteBoundDeliveryReadHandler, createRouteBoundDeliveryRecoveryHandler } = await import('./app/tasks/detail.tsx');
  type Identity = { taskId: string; runId: string };
  type Entry = { identity: Identity; generation: number };
  type Receipt = import('@weknora/mobile-core').DeliveryReceiptView;
  type FocusEffect = () => void | (() => void);
  const expoRouter = require('expo-router') as { __focusEffects: FocusEffect[] };
  const a: Identity = { taskId: 'task-a', runId: 'run-a' };
  const b: Identity = { taskId: 'task-b', runId: 'run-b' };
  const receiptA: Receipt = {
    deliveryId: 'dlv-a', taskId: a.taskId, runId: a.runId, state: 'pushed', repo: 'octocat/hello',
    branch: 'weknora/task/a', baselineSha: 'a'.repeat(40), attention: true, updatedAt: '2026-09-24T00:00:30Z',
  };
  const deliveredA: Receipt = { ...receiptA, state: 'delivered' };
  function deferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (error: unknown) => void;
    const promise = new Promise<T>((resolvePromise, rejectPromise) => { resolve = resolvePromise; reject = rejectPromise; });
    return { promise, resolve, reject };
  }
  function routeEntryRef(): { current: Entry } {
    const found = hooks().__values().find((value) => typeof value === 'object' && value !== null
      && 'current' in value && typeof (value as { current?: unknown }).current === 'object'
      && (value as { current: Record<string, unknown> }).current !== null
      && 'identity' in (value as { current: Record<string, unknown> }).current
      && 'generation' in (value as { current: Record<string, unknown> }).current);
    assert.ok(found, 'the real lifecycle owns its route-entry ref');
    return found as { current: Entry };
  }

  hooks().__reset();
  expoRouter.__focusEffects.length = 0;
  render(TaskDetailRouteLifecycle, { ...a });
  const aFocus = expoRouter.__focusEffects.at(-1);
  assert.ok(aFocus, 'TaskDetailRouteLifecycle registers the public useFocusEffect callback');
  const blurA = aFocus!();
  assert.equal(typeof blurA, 'function', 'the focused A entry registers a blur invalidation cleanup');
  const firstA = { ...routeEntryRef().current };

  const reads: Receipt[] = [];
  const recoveries: Receipt[] = [];
  const errors: Array<string | undefined> = [];
  const oldRead = deferred<Receipt>();
  const applyOldRead = createRouteBoundDeliveryReadHandler({ entry: firstA, currentEntry: () => routeEntryRef().current, setDelivery: (receipt) => { reads.push(receipt); } });
  const readPending = oldRead.promise.then(applyOldRead);
  const oldSuccess = deferred<Receipt>();
  const oldSuccessHandler = createRouteBoundDeliveryRecoveryHandler({
    entry: firstA, receipt: receiptA, currentEntry: () => routeEntryRef().current, recover: () => oldSuccess.promise,
    setDelivery: (receipt) => { recoveries.push(receipt); }, setRecoveryError: (error) => { errors.push(error); }, errorCopy: String,
  });
  const oldSuccessPending = oldSuccessHandler({ runId: a.runId, deliveryId: receiptA.deliveryId });
  const oldFailure = deferred<Receipt>();
  const oldFailureHandler = createRouteBoundDeliveryRecoveryHandler({
    entry: firstA, receipt: receiptA, currentEntry: () => routeEntryRef().current, recover: () => oldFailure.promise,
    setDelivery: (receipt) => { recoveries.push(receipt); }, setRecoveryError: (error) => { errors.push(error); }, errorCopy: String,
  });
  const oldFailurePending = oldFailureHandler({ runId: a.runId, deliveryId: receiptA.deliveryId });

  // Save A's mounted hook state, then render/focus/blur B as the route pushed over it.
  const retainedA = hooks().__snapshot();
  (blurA as () => void)();
  hooks().__reset();
  expoRouter.__focusEffects.length = 0;
  render(TaskDetailRouteLifecycle, { ...b });
  const bFocus = expoRouter.__focusEffects.at(-1);
  assert.ok(bFocus, 'the pushed B route also registers its focus callback');
  const blurB = bFocus!();
  assert.equal(typeof blurB, 'function');
  (blurB as () => void)();

  // Restore the retained A instance and invoke its registered focus callback with identical params.
  hooks().__restore(retainedA);
  const aRefocusCleanup = aFocus!();
  const currentA = { ...routeEntryRef().current };
  assert.deepEqual(currentA.identity, a, 'the retained instance keeps the same taskId/runId');
  assert.notEqual(currentA.generation, firstA.generation, 'focus creates a fresh route entry without a prop change');
  errors.splice(0, errors.length, 'new focused A error');
  oldRead.resolve(receiptA);
  oldSuccess.resolve(deliveredA);
  oldFailure.reject(new Error('old focused A failure'));
  await readPending;
  await oldSuccessPending;
  await assert.rejects(oldFailurePending, /old focused A failure/);
  assert.deepEqual(reads.slice(), [], 'old A read does not populate the refocused A entry');
  assert.deepEqual(recoveries.slice(), [], 'old A recovery success does not replace the refocused A receipt');
  assert.deepEqual(errors.slice(), ['new focused A error'], 'old A error does not overwrite the refocused A error');

  const newRecovery = deferred<Receipt>();
  const newHandler = createRouteBoundDeliveryRecoveryHandler({
    entry: currentA, receipt: receiptA, currentEntry: () => routeEntryRef().current, recover: () => newRecovery.promise,
    setDelivery: (receipt) => { recoveries.push(receipt); }, setRecoveryError: (error) => { errors.push(error); }, errorCopy: String,
  });
  const newPending = newHandler({ runId: a.runId, deliveryId: receiptA.deliveryId });
  newRecovery.resolve(deliveredA);
  await newPending;
  assert.deepEqual(recoveries.slice(), [deliveredA], 'new focused A recovery still updates normally');
  (aRefocusCleanup as () => void)();
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

test('the no-view error branch keeps an in-place retry entry', async () => {
  const { TaskDetailScreen } = await import('./screens/TaskDetailScreen.tsx');
  hooks().__reset();
  let refreshed = 0;
  const tree = render(TaskDetailScreen, { view: undefined, loading: false, error: '任务参数缺失（taskId/runId），请从任务列表重新进入。', onRefresh: () => { refreshed += 1; } });
  const retry = descendants(tree).find(({ type, props }) => type === 'Button' && props.title === '重试');
  assert.ok(retry !== undefined, '错误分支必须提供就地重试入口（controller.refresh 支持从无 view 状态恢复）');
  (retry!.props.onPress as () => void)();
  assert.equal(refreshed, 1);
});

test('the refresh button is disabled while loading', async () => {
  const { TaskDetailScreen } = await import('./screens/TaskDetailScreen.tsx');
  hooks().__reset();
  const view: import('@weknora/mobile-core').TaskDetailView = {
    taskId: 't', runId: 'r', title: '', lifecycle: 'active', runStatus: 'running', attention: 'none',
    executionStatus: 'running', settlementStatus: 'pending', revision: 1, cursor: 1, incomplete: false,
    connection: 'live', timeline: [], duplicateSeqs: [],
  };
  const tree = render(TaskDetailScreen, { view, loading: true, error: undefined, onRefresh: () => {} });
  const refresh = descendants(tree).find(({ type, props }) => type === 'Button' && props.title === '重新同步快照');
  assert.equal((refresh!.props as { disabled?: boolean }).disabled, true, 'loading 期间刷新按钮必须禁用（B2-F39）');
});

test('the task detail error chain maps codes to copy instead of leaking raw internals', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const detailRoute = readFileSync(join(here, 'app/tasks/detail.tsx'), 'utf8');
  const detailView = readFileSync(join(here, 'task-detail-view.ts'), 'utf8');
  const screen = readFileSync(join(here, 'screens/TaskDetailScreen.tsx'), 'utf8');
  const tasks = readFileSync(join(here, 'screens/TasksScreen.tsx'), 'utf8');
  assert.match(`${detailRoute}${detailView}`, /TASK_OFFICE_INVALID_INPUT/, '路由 catch 必须按错误码分流（B2-F13）');
  assert.match(`${detailRoute}${detailView}`, /TASK_OFFICE_DETAIL_UNAVAILABLE/, '详情端口缺失不得折叠为「请先登录」（B2-F13）');
  assert.match(screen, /INTERRUPTION_COPY/, 'interruption 原因必须经文案映射（B2-F40）');
  assert.match(screen, /INTERRUPTION_COPY\[view\.interruption\.reason\]/, '不得直出内部码');
  assert.match(tasks, /title="Details"/, '打开按钮文案与同屏英文统一（B2-F5）');
});

test('the offline interruption renders mapped copy and never the raw internal code (R1-F4)', async () => {
  const { TaskDetailScreen } = await import('./screens/TaskDetailScreen.tsx');
  const offlineView = {
    taskId: 'task-1', runId: 'run-1', title: '季度竞品报告', lifecycle: 'active', runStatus: 'running',
    attention: 'none', executionStatus: 'running', settlementStatus: 'pending', revision: 4,
    cursor: 2, incomplete: false, connection: 'interrupted' as const,
    interruption: { reason: 'offline' as const, message: '当前离线：以下为最近一次同步的加密缓存内容' },
    timeline: [], duplicateSeqs: [],
  };
  const element = render(TaskDetailScreen as (props: unknown) => unknown, { view: offlineView, loading: false, onRefresh: () => undefined });
  const texts = descendants(element).filter(({ type }) => type === 'Text').flatMap(({ props }) => props.children).join(' ');
  assert.equal(texts.includes('当前离线'), true, "offline 原因必须映射为用户文案（当前渲染 'offline' 内部码）");
  assert.equal(/·\s*offline/.test(texts), false, '不得直出 offline 内部码（app-smoke 红线 B2-F40/R1-F4）');
});

test('composition wires the knowledge QA remote into Task Office and /ask consumes the office only (T15)', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const composition = readFileSync(join(here, 'composition.ts'), 'utf8');
  const askRoute = readFileSync(join(here, 'app/ask.tsx'), 'utf8');
  const home = readFileSync(join(here, 'screens/HomeScreen.tsx'), 'utf8');
  // R1-F7：组合根为 knowledgeQA 接 guardKnowledgeQABackend 端口级纵深（断言随新接线语义更新）。
  assert.match(composition, /knowledgeQA:\s*guardKnowledgeQABackend\(\s*createMobileKnowledgeQARemote\(/, 'Task Office 必须装配 knowledgeQA 端口（经 Offline Guard 纵深）');
  assert.match(composition, /import \{ createMobileKnowledgeQARemote \} from '@weknora\/api-client\/mobile\/knowledge-qa';/);
  assert.match(askRoute, /activeTaskOffice\(\)/, '/ask 只经组合根取 Task Office，不直连 api-client');
  assert.doesNotMatch(askRoute, /@weknora\/api-client/, 'Screen/路由禁止直连 wire 客户端（module-seams §10）');
  assert.match(home, /Ask knowledge/, '授权首页必须有知识问纳入口');
});

test('the universal New entry renders the recommended lead agent, budget control and a blocked-submit reason', async () => {
  const { NewTaskScreen } = await import('./screens/NewTaskScreen.tsx');
  const state = {
    draft: { text: '整理周报', agentId: 'a-general', budgetUpper: 200, attachments: [{ id: 'f-1', name: 'a.pdf', readiness: 'scanning' }], knowledgeIds: ['kb-1'] },
    agents: [
      { id: 'a-general', name: '通用主理', summary: '', kind: 'general', capability: { state: 'supported', reason: '' } },
      { id: 'a-coding', name: '编码', summary: '', kind: 'coding', capability: { state: 'supported', reason: '' } },
    ],
    knowledge: [{ id: 'kb-1', title: '团队知识库', scanStatus: 'indexed', documentCount: 3, updatedAt: '2026-09-24T00:00:00Z' }],
    recommendation: { agent: { id: 'a-general', name: '通用主理', summary: '', kind: 'general', capability: { state: 'supported', reason: '' } }, basis: 'kind-general' },
    readiness: { ready: false, reason: 'attachments_not_ready', blockingAttachments: [{ id: 'f-1', name: 'a.pdf', readiness: 'scanning' }] },
    loading: false,
    submitting: false,
    inFlight: { requestId: 'req-old', phase: 'awaiting_reconciliation', dispatched: false },
  };
  const events: string[] = [];
  const element = render(NewTaskScreen, {
    state,
    onUpdate: () => { events.push('update'); },
    onSetAttachments: () => { events.push('attachments'); },
    onToggleKnowledge: () => { events.push('knowledge'); },
    onSubmit: () => { events.push('submit'); },
    onCancel: () => { events.push('cancel'); },
    onRefreshAgents: () => { events.push('refresh'); },
  });
  const text = descendants(element).filter(({ type }) => type === 'Text').flatMap(({ props }) => props.children).join(' ');
  assert.equal(text.includes('通用主理'), true, 'the recommended lead agent is visible');
  assert.equal(text.includes('attachments_not_ready'), true, 'a blocked submit surfaces its reason, never a fake success');
  assert.equal(text.includes('req-old'), true, 'an unresolved intent from before is surfaced');
  const buttons = descendants(element).filter(({ type }) => type === 'Button').map(({ props }) => props.title);
  assert.equal(buttons.some((title) => String(title).includes('✓ 团队知识库')), true, 'attached knowledge is visibly selected');
  assert.equal(buttons.includes('Submit task'), true);
  assert.equal(buttons.includes('Keep draft'), true);
});

test('the universal New entry caps goal text at the source so one intent record stays inside the SecureStore envelope', async () => {
  const { NewTaskScreen, GOAL_TEXT_MAX_LENGTH } = await import('./screens/NewTaskScreen.tsx');
  hooks().__reset();
  assert.equal(GOAL_TEXT_MAX_LENGTH, 500, 'R1 裁决第三层：maxLength=500（主控裁决的工程默认值）');
  const baseState = {
    draft: { text: '', agentId: null, budgetUpper: 0, attachments: [] as never[], knowledgeIds: [] as string[] },
    agents: [],
    knowledge: [],
    recommendation: { agent: null, basis: 'none' },
    readiness: { ready: false, reason: 'text_required' as const, blockingAttachments: [] },
    loading: false,
    submitting: false,
    inFlight: undefined,
  };
  const props = {
    onUpdate: () => {},
    onSetAttachments: () => {},
    onToggleKnowledge: () => {},
    onSubmit: () => {},
    onCancel: () => {},
    onRefreshAgents: () => {},
  };
  const goalInputOf = (tree: unknown) => descendants(tree).find(({ type, props: input }) => type === 'TextInput' && input.placeholder === '今天想完成什么？');

  // 500 字（达上限）：maxLength 生效 + 超长提示出现
  const capped = render(NewTaskScreen, { state: { ...baseState, draft: { ...baseState.draft, text: '目'.repeat(500) } }, ...props });
  const goalInput = goalInputOf(capped);
  assert.ok(goalInput, 'the goal TextInput renders');
  assert.equal((goalInput!.props as { maxLength?: number }).maxLength, 500, 'the goal TextInput enforces maxLength=500 at the source (R1 layer 3)');
  const cappedText = descendants(capped).filter(({ type }) => type === 'Text').flatMap(({ props }) => props.children).join(' ');
  assert.equal(cappedText.includes('目标文本已达 500 字上限'), true, 'the over-limit hint is visible at the cap');

  // 499 字：提示不出现（不打扰未触界的输入）
  const under = render(NewTaskScreen, { state: { ...baseState, draft: { ...baseState.draft, text: '目'.repeat(499) } }, ...props });
  const underText = descendants(under).filter(({ type }) => type === 'Text').flatMap(({ props }) => props.children).join(' ');
  assert.equal(underText.includes('目标文本已达'), false, 'the hint stays hidden below the cap');

  // 字节预算交叉验证：最坏情形（500 个中文字 × 3B UTF-8 + UUID 形态 requestId/sessionId
  // + scope 固定开销）的单条意图记录序列化后仍在 Android SecureStore ~2048B 信封内——
  // 修复前无上限时 560+ 中文字即超限，intent log 的 setItemAsync 抛错使提交永久失败。
  const { randomUUID } = await import('node:crypto');
  const worstCaseRecord = {
    requestId: randomUUID(),
    sessionId: randomUUID(),
    goal: { text: '目'.repeat(GOAL_TEXT_MAX_LENGTH), agentId: 'a-general', budgetUpper: 200 },
    scope: { origin: 'https://weknora.example.test', tenantID: 'tenant-1', userID: 'user-1' },
    persistedAt: '2026-09-24T00:00:00Z',
  };
  const serializedBytes = new TextEncoder().encode(JSON.stringify([worstCaseRecord])).length;
  assert.ok(serializedBytes <= 2048, `worst-case single intent record must fit the Android SecureStore ~2048B envelope but was ${serializedBytes} bytes`);
});

test('the /new route reaches the Task Office through the composition root, never the wire directly', async () => {
  const { readFileSync } = await import('node:fs');
  const source = readFileSync(resolve(workspaceRoot, 'apps/mobile/src/app/new.tsx'), 'utf8');
  assert.equal(/activeTaskOffice\(\)/.test(source), true, 'the route must obtain the office via activeTaskOffice()');
  assert.equal(/workbench\/executions/.test(source), false, 'screens never call wire paths directly (module-seams §10)');
});

test('the home screen exposes the universal New entry', async () => {
  const { HomeScreen } = await import('./screens/HomeScreen.tsx');
  hooks().__reset();
  const element = render(HomeScreen, {
    deploymentLabel: 'Acme',
    tenants: [{ id: 'tenant-1', name: 'Acme' }],
    activeTenantId: 'tenant-1',
    onActivateTenant: () => {},
    onSignOut: async () => {},
    taskOffice: {
      home: async () => ({ needsMe: [], running: [], recentlyCompleted: [], unreadNotifications: 0, asOf: '' }),
      tasks: async () => ({ items: [], duplicateRunIds: [] }),
      moreTasks: async () => ({ items: [], duplicateRunIds: [] }),
      archive: async () => {},
      restore: async () => {},
      open: () => { throw new Error('unused'); },
      start: async () => { throw new Error('unused'); },
      reconcilePending: async () => [],
    },
  });
  const buttons = descendants(element).filter(({ type }) => type === 'Button').map(({ props }) => props.title);
  assert.equal(buttons.includes('New task'), true);
});

test('the /new lifecycle host disposes its controller on unmount, including after async creation', async () => {
  const route = await import('./app/new.tsx');
  const calls: string[] = [];
  const office = {
    start: async () => { calls.push('start'); throw new Error('unused'); },
    reconcilePending: async () => { calls.push('reconcilePending'); return []; },
  };
  // mount → 等待异步创建完成（init 会调用 office.reconcilePending，证明 controller 已创建）
  // → unmount → 再等一轮 microtask：卸载后不得再有任何 office 调用（cleanup 必须拿到
  // 已创建的 controller 并 dispose，而不是首帧 state 的 stale closure）。
  hooks().__beginRender();
  void route.NewTaskRouteLifecycle({ office: office as unknown as Parameters<typeof route.NewTaskRouteLifecycle>[0]['office'] });
  hooks().__mount();
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(calls.filter((entry) => entry === 'reconcilePending').length >= 1, true, 'initialization reconciled during the mounted period');
  hooks().__unmount();
  const afterUnmount = calls.length;
  await new Promise((resolve) => setImmediate(resolve));
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(calls.length, afterUnmount, 'no office calls may happen after unmount');
  // 第二轮 mount/unmount 验证循环稳定（重复挂载不残留）
  hooks().__beginRender();
  void route.NewTaskRouteLifecycle({ office: office as unknown as Parameters<typeof route.NewTaskRouteLifecycle>[0]['office'] });
  hooks().__mount();
  await new Promise((resolve) => setImmediate(resolve));
  hooks().__unmount();
  const afterSecond = calls.length;
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(calls.length, afterSecond);
});

test('the task detail screen exposes the materials entry point', async () => {
  const screen = await import('./screens/TaskDetailScreen.tsx');
  const offline = screen.TaskDetailScreen({ view: undefined, loading: false, error: 'x', onRefresh: () => {}, onOpenMaterials: () => {} });
  assert.ok(JSON.stringify(offline).includes('重试'), 'offline fallback still renders');
  const view: import('@weknora/mobile-core').TaskDetailView = {
    taskId: 'task-1', runId: 'run-1', title: '报告', lifecycle: 'active', runStatus: 'succeeded', attention: 'none',
    executionStatus: 'succeeded', settlementStatus: 'settled', revision: 1, cursor: 2, incomplete: false, connection: 'drained',
    timeline: [], duplicateSeqs: [],
  };
  const withEntry = screen.TaskDetailScreen({ view, loading: false, onRefresh: () => {}, onOpenMaterials: () => {} });
  assert.ok(JSON.stringify(withEntry).includes('任务材料'), 'the materials entry renders when the callback is provided');
  const withoutEntry = screen.TaskDetailScreen({ view, loading: false, onRefresh: () => {} });
  assert.ok(!JSON.stringify(withoutEntry).includes('任务材料'), 'no entry without the callback (older callers compile unchanged)');
});

test('the materials screen and route consume the Task Material interface only', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const composition = readFileSync(join(here, 'composition.ts'), 'utf8');
  assert.match(composition, /createTaskMaterial\(/, 'composition must instantiate the Task Material module');
  assert.match(composition, /createMobileMaterialRemote/, 'composition must bind the remote adapter to the module');
  for (const relative of ['screens/MaterialsScreen.tsx', 'materials-view.ts', 'app/tasks/materials.tsx']) {
    const source = readFileSync(join(here, relative), 'utf8');
    assert.equal(/@weknora\/(api-client|contracts)/.test(source), false, `${relative} must consume the Task Material Interface only`);
  }
  const route = await import('./app/tasks/materials.tsx');
  assert.equal(typeof route.default, 'function', 'src/app/tasks/materials.tsx must default-export the materials route');
  const screen = await import('./screens/MaterialsScreen.tsx');
  assert.equal(typeof screen.MaterialsScreen, 'function');
});

test('the malformed diff pane falls back to the raw text behind its notice', async () => {
  const { MaterialsScreen } = await import('./screens/MaterialsScreen.tsx');
  hooks().__reset();
  const view: import('@weknora/mobile-core').MaterialView = {
    kind: 'diff',
    entry: { materialId: 'msg-1:1', index: 1, kind: 'diff', name: 'changes.diff', mime: 'text/x-diff', size: 30, version: 'bbbbbbbbbbbbbbbb', sourceRun: 'run-1' },
    preview: { state: 'supported' },
    hunks: [],
    malformed: true,
    raw: 'not a parseable unified diff\n',
  };
  const element = render(MaterialsScreen, {
    index: undefined, view, loading: false,
    onOpenMaterial: () => {}, onOpenTerminal: () => {}, onOpenEvidence: () => {},
    onDownload: () => {}, onShare: () => {}, onRefresh: () => {}, onBack: () => {},
  });
  // react stub 的 createElement 不执行子组件：对 MaterialViewPane 元素二次渲染（与 RuntimeSurface 测试同模式）。
  const paneElement = descendants(element).find(({ type }) => typeof type === 'function' && (type as { name?: string }).name === 'MaterialViewPane');
  assert.ok(paneElement, 'the diff view mounts the material view pane');
  const pane = render(paneElement.type as (props: unknown) => unknown, paneElement.props);
  const texts = descendants(pane).filter(({ type }) => type === 'Text').flatMap(({ props }) => props.children).flatMap((part) => (typeof part === 'string' ? [part] : []));
  assert.equal(texts.some((text) => text.includes('无法解析为标准 diff')), true, 'the malformed notice renders');
  assert.equal(texts.some((text) => text.includes('not a parseable unified diff')), true, 'the raw text must render behind the notice (module contract: raw is always set for diff views)');
});

test('the attention inbox route, screen and view consume the task office interface only', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  // 集成说明：T08 路由原为 app/inbox.tsx；#41 行动通知收件箱并入后迁至 app/attention.tsx。
  for (const relative of ['screens/AttentionInboxScreen.tsx', 'attention-inbox-view.ts', 'app/attention.tsx']) {
    const source = readFileSync(join(here, relative), 'utf8');
    assert.equal(/@weknora\/(api-client|contracts)/.test(source), false, `${relative} must consume the Task Office Interface only (T08)`);
  }
  const composition = readFileSync(join(here, 'composition.ts'), 'utf8');
  // T10（#40）AC2 后 interactions 端口经 Offline Gate 包装——仍必须派生自 remote（缺失即 fail closed）。
  assert.match(composition, /interactions:\s*guardInteractionBackend\(remote/, 'taskOfficeFor must pass the remote (offline-guarded) as the interactions port; inbox()/decide() fail closed without it (T08/T10)');
});

test('the attention inbox screen renders honest receipt copy and per-kind matrix actions', async () => {
  const { AttentionInboxScreen } = await import('./screens/AttentionInboxScreen.tsx');
  const { ATTENTION_RECEIPT_COPY } = await import('./attention-inbox-view.ts');
  hooks().__reset();
  const state = {
    loading: false,
    items: [{
      interactionId: 'i-1', runId: 'run-1', kind: 'tool_approval' as const, argsHash: 'sha256:aa', expectedRevision: 4, createdAt: '2026-09-24T00:00:00Z',
    }],
    receipts: [{ key: 'i-1:0', copy: ATTENTION_RECEIPT_COPY['delivery-unknown'] }],
  };
  const decided: Array<{ interactionId: string; action: string }> = [];
  const element = render(AttentionInboxScreen, {
    state,
    onRefresh: () => {},
    onDecide: (item: { interactionId: string }, action: string) => { decided.push({ interactionId: item.interactionId, action }); },
  });
  const texts = descendants(element).filter(({ type }) => type === 'Text').flatMap(({ props }) => props.children).flatMap((part) => (typeof part === 'string' ? [part] : []));
  assert.equal(texts.some((text) => text.includes('工具审批')), true);
  assert.equal(texts.some((text) => text.includes('外部执行通道状态未知')), true, 'delivery-unknown 文案必须出现（AC2）');
  const buttons = descendants(element).filter(({ type }) => type === 'Button').map(({ props }) => props.title);
  assert.equal(buttons.includes('批准'), true);
  assert.equal(buttons.includes('拒绝'), true);
  assert.equal(buttons.includes('扩展预算'), false, 'tool_approval 行不得渲染 budget 域动作（矩阵冻结）');
  const approve = descendants(element).find(({ type, props }) => type === 'Button' && props.title === '批准');
  (approve!.props.onPress as () => void)();
  assert.deepEqual(decided, [{ interactionId: 'i-1', action: 'approve' }]);
});

test('the inbox route exists behind a default export', async () => {
  const inboxRoute = await import('./app/inbox.tsx');
  assert.equal(typeof inboxRoute.default, 'function', 'src/app/inbox.tsx must default-export the inbox route');
});

test('notification navigation re-authorizes, parses the safe deep link, and never performs business actions', async () => {
  const { openNotificationFromInbox } = await import('./composition.ts');

  const calls: string[] = [];
  const pushes: Array<{ path: string; params?: Record<string, string> }> = [];
  const push = (path: string, params?: Record<string, string>): void => { pushes.push({ path, params }); };
  const navigableInbox = {
    resolveTarget: () => ({ kind: 'task-detail' as const, taskId: 't-1', runId: 'r-1' }),
    markRead: async (id: string) => { calls.push(`markRead:${id}`); },
  };
  const invalidInbox = {
    resolveTarget: () => undefined,
    markRead: navigableInbox.markRead,
  };
  const authorized = { surface: 'authorized' as const };
  const unauthorized = { surface: 'deployment-login' as const };

  assert.equal(await openNotificationFromInbox(navigableInbox, authorized, { notificationId: 'n-1' }, push), 'navigated');
  assert.deepEqual(pushes, [{ path: '/tasks/detail', params: { taskId: 't-1', runId: 'r-1' } }]);
  assert.deepEqual(calls, ['markRead:n-1']);

  pushes.length = 0; calls.length = 0;
  assert.equal(await openNotificationFromInbox(navigableInbox, unauthorized, { notificationId: 'n-1' }, push), 'blocked-unauthorized');
  assert.deepEqual(pushes, [], 'an unauthorized surface must not navigate');
  assert.deepEqual(calls, [], 'an unauthorized surface must not mark read');

  assert.equal(await openNotificationFromInbox(invalidInbox, authorized, { notificationId: 'n-2' }, push), 'invalid-link');
  assert.deepEqual(pushes, [], 'a malformed deep link must not navigate');
  assert.deepEqual(calls, [], 'a malformed deep link must not mark read');
});

test('device registration is fail-closed without a native push token or device identity', async () => {
  const { registerActiveDeviceIfPossible } = await import('./composition.ts');
  const authorizedRuntime = {
    snapshot: () => ({ surface: 'authorized' as const, deployment: { origin: 'https://weknora.example.test', label: 'Test' } }),
    authorizedRequest: async () => { throw new Error('must not reach the wire without a token'); },
    scopeLease: () => undefined,
  };
  const unauthorizedRuntime = { snapshot: () => ({ surface: 'deployment-login' as const }) };

  assert.equal(await registerActiveDeviceIfPossible(authorizedRuntime as never, { token: async () => undefined }, { deviceId: async () => 'device-1' }), 'no-token');
  assert.equal(await registerActiveDeviceIfPossible(authorizedRuntime as never, { token: async () => 'tok' }, { deviceId: async () => undefined }), 'no-device-id');
  assert.equal(await registerActiveDeviceIfPossible(unauthorizedRuntime as never, { token: async () => 'tok' }, { deviceId: async () => 'device-1' }), 'unauthorized');
});

test('device registration requests notification permission first and reports denial honestly (#70)', async () => {
  const { registerActiveDeviceIfPossible } = await import('./composition.ts');
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const authorizedRuntime = {
    snapshot: () => ({ surface: 'authorized' as const, deployment: { origin: 'https://weknora.example.test', label: 'Test' } }),
    authorizedRequest: async () => { throw new Error('must not reach the wire without a token'); },
    scopeLease: () => undefined,
  };
  const deniedTokens: string[] = [];
  // denied：权限短路发生在 token 获取之前，绝不注册无权限设备（Review Focus 1）
  const denied = await registerActiveDeviceIfPossible(
    authorizedRuntime as never,
    { token: async () => { deniedTokens.push('fetched'); return 'tok'; } },
    { deviceId: async () => 'device-1' },
    { ensure: async () => 'denied' },
  );
  assert.equal(denied, 'permission-denied');
  assert.deepEqual(deniedTokens, [], 'denied 后不得触碰 push token 通道');
  // granted：走完既有 fail-closed 注册链（本 stub 环境 registry.register 到 wire 即抛 → failed），绝不被权限层短路
  const granted = await registerActiveDeviceIfPossible(
    authorizedRuntime as never,
    { token: async () => 'tok' },
    { deviceId: async () => 'device-1' },
    { ensure: async () => 'granted' },
  );
  assert.notEqual(granted, 'permission-denied');
  // unavailable：保持既有行为——继续走 token fail-closed 路径
  const unavailable = await registerActiveDeviceIfPossible(
    authorizedRuntime as never,
    { token: async () => undefined },
    { deviceId: async () => 'device-1' },
    { ensure: async () => 'unavailable' },
  );
  assert.equal(unavailable, 'no-token');
  // 组合根默认参必须接原生权限 Adapter（真机路径生效的唯一接线点）
  const composition = readFileSync(join(here, 'composition.ts'), 'utf8');
  assert.match(composition, /permission[^=]*=\s*createNativeNotificationPermissionIfAvailable\(\)/, '默认参必须惰性接原生权限 Adapter');
});

test('the home surface keeps a reachable inbox entry point and the inbox screen renders projections', async () => {
  const { HomeScreen } = await import('./screens/HomeScreen.tsx');
  hooks().__reset();
  const home = render(HomeScreen, {
    deploymentLabel: 'Test', tenants: [{ id: '7' }], activeTenantId: '7',
    onActivateTenant: () => {}, onSignOut: async () => {},
    taskOffice: { home: async () => ({ needsMe: [], running: [], recentlyCompleted: [], unreadNotifications: 0, asOf: '2026-09-24T00:00:00Z' }) },
  });
  assert.notEqual(
    descendants(home).find(({ type, props }) => type === 'Button' && props.title === 'Open Inbox'),
    undefined,
    'HomeScreen must keep an Open Inbox entry point',
  );

  const { InboxScreen } = await import('./screens/InboxScreen.tsx');
  hooks().__reset();
  const opened: string[] = [];
  const screen = render(InboxScreen, {
    view: {
      items: [
        { notificationId: 'n-1', kind: 'attention', title: '需要你处理', body: '', createdAt: '2026-09-24T01:00:00Z', read: false, deepLink: 'weknora://tasks/detail?taskId=t-1&runId=r-1' },
        { notificationId: 'n-2', kind: 'budget', title: '预算事件', body: '', createdAt: '2026-09-24T02:00:00Z', read: true },
      ],
      unreadCount: 1,
      duplicateNotificationIds: [],
    },
    loading: false,
    onRefresh: () => {}, onLoadMore: () => {},
    onOpenNotification: (item: { notificationId: string }) => { opened.push(item.notificationId); },
  });
  const texts = descendants(screen).filter(({ type }) => type === 'Text').map(({ props }) => String(props.children));
  assert.ok(texts.some((text) => text.includes('未读 1')), 'unread count is visible');
  const row = descendants(screen).find(({ type, props }) => type === 'Button' && String(props.title).includes('需要你处理'));
  (row!.props.onPress as () => void)();
  assert.deepEqual(opened, ['n-1'], 'tapping a row hands the item to the composition-owned navigation seam');
});

test('tapping a notification whose markRead fails degrades to a visible notice instead of an unhandled rejection', async () => {
  const { InboxRouteLifecycle } = await import('./app/inbox.tsx');
  const { InboxScreen } = await import('./screens/InboxScreen.tsx');
  hooks().__reset();
  const view: import('@weknora/mobile-core').InboxView = {
    items: [
      { notificationId: 'n-1', kind: 'attention', title: '需要你处理', body: '', createdAt: '2026-09-24T01:00:00Z', read: false, deepLink: 'weknora://tasks/detail?taskId=t-1&runId=r-1' },
    ],
    unreadCount: 1,
    duplicateNotificationIds: [],
  };
  const inbox = {
    subscribe(listener: (next: import('@weknora/mobile-core').InboxView) => void) { listener(view); return () => {}; },
    page: async () => view,
    more: async () => view,
    applyHint: async () => view,
    resolveTarget: () => ({ kind: 'task-detail' as const, taskId: 't-1', runId: 'r-1' }),
    markRead: async () => { throw new Error('INBOX_BACKEND'); },
  };
  const runtime = { snapshot: () => ({ surface: 'authorized' as const }) };

  const unhandled: unknown[] = [];
  const recordUnhandled = (reason: unknown) => { unhandled.push(reason); };
  process.on('unhandledRejection', recordUnhandled);
  try {
    render(InboxRouteLifecycle, { inbox, runtime });
    hooks().__mount();
    await new Promise((resolve) => setTimeout(resolve, 0));
    const withRows = render(InboxRouteLifecycle, { inbox, runtime });
    const screenWithRows = descendants(withRows).find(({ type }) => type === InboxScreen);
    assert.ok(screenWithRows, 'the route renders the inbox screen');
    const screenTree = render(InboxScreen, screenWithRows!.props);
    const row = descendants(screenTree).find(({ type, props }) => type === 'Button' && String(props.title).includes('需要你处理'));
    assert.ok(row, 'the notification row renders once the projection arrives');
    (row!.props.onPress as () => void)();
    await new Promise((resolve) => setTimeout(resolve, 0));
    await new Promise((resolve) => setTimeout(resolve, 0));
    assert.deepEqual(unhandled, [], 'a failing markRead must never escape the tap handler as an unhandled rejection');
    const after = render(InboxRouteLifecycle, { inbox, runtime });
    const screenElement = descendants(after).find(({ type }) => type === InboxScreen);
    assert.ok(screenElement, 'the route keeps rendering the inbox screen after the failure');
    assert.match(
      String((screenElement!.props as { notice?: string }).notice ?? ''),
      /已读状态同步失败.*INBOX_BACKEND/,
      'the markRead failure is surfaced as a visible notice carrying the error code',
    );
    hooks().__unmount();
  } finally {
    process.off('unhandledRejection', recordUnhandled);
  }
});

test('the inbox screen surfaces refresh failures in place when a projection is already on screen', async () => {
  const { InboxScreen } = await import('./screens/InboxScreen.tsx');
  hooks().__reset();
  const screen = render(InboxScreen, {
    view: {
      items: [
        { notificationId: 'n-1', kind: 'attention', title: '需要你处理', body: '', createdAt: '2026-09-24T01:00:00Z', read: false, deepLink: 'weknora://tasks/detail?taskId=t-1&runId=r-1' },
      ],
      unreadCount: 1,
      duplicateNotificationIds: [],
    },
    loading: false,
    error: 'INBOX_SCOPE_CHANGED',
    onRefresh: () => {},
    onOpenNotification: () => {},
  });
  const json = JSON.stringify(screen);
  assert.ok(json.includes('INBOX_SCOPE_CHANGED'), 'a refresh/load-more failure must be visible in place when a view already exists');
  assert.ok(json.includes('需要你处理'), 'the already-loaded rows stay on screen instead of being replaced');
  assert.ok(!json.includes('无法读取行动通知'), 'a screen with a projection must not fall back to the empty-view error branch');
});

test('the legacy screen renders an explicit unauthenticated state and per-card inputs', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const source = readFileSync(join(here, 'screens/LegacyTasksScreen.tsx'), 'utf8');
  // B3-F22：无授权面必须发布显式空态文案（对照 app/attention.tsx:13-16 范式），不得停留 Loading。
  assert.match(source, /请先登录/);
  // B3-F21：question 状态按 taskId 隔离（Record 槽位），不再整屏单一 useState。
  assert.match(source, /Record<string, string>/);
  assert.match(source, /questions\[card\.taskId\]/);
  assert.doesNotMatch(source, /const \[question, setQuestion\] = useState\(''\)/, '整屏单一 question state 必须移除');
});

test('the legacy screen shows the explicit empty state when no office is active', async () => {
  const hooks2 = hooks();
  hooks2.__beginRender();
  const { LegacyTasksScreen } = await import('./screens/LegacyTasksScreen.tsx');
  LegacyTasksScreen({}); // 首渲染（activeTaskOffice() 为 undefined——无授权面）
  hooks2.__mount(); // effect 挂载：发布显式空态
  hooks2.__beginRender(); // 重渲染读取挂载后的状态
  const tree = LegacyTasksScreen({});
  const json = JSON.stringify(tree);
  assert.ok(json.includes('请先登录'), 'B3-F22：显式空态文案渲染（不永久停留 loading）');
  assert.ok(!json.includes('Loading'), 'loading 已复位');
});

test('composition caches instances by deployment scope key and registration failures do not permanently occupy an origin', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const source = readFileSync(join(here, 'composition.ts'), 'utf8');
  // B3-F26：四个模块级缓存必须以 origin::tenant 为键（deploymentScopeKey）。
  const cacheFactories = ['taskOffices', 'deviceRegistries', 'notificationInboxes', 'taskMaterials'];
  for (const cache of cacheFactories) {
    assert.match(source, new RegExp(`(?:const|let)\\s+${cache}\\s*=\\s*new Map<string`), `${cache} 存在`);
  }
  assert.ok((source.match(/deploymentScopeKey\(/g) ?? []).length >= 5, '缓存工厂统一走 deploymentScopeKey（含定义自身）');
  // B3-F28：注册不再「先标记、后尝试、永不重试」——registeredFor.add 移到成功回调之后。
  assert.doesNotMatch(source, /registeredFor\.current\.add\(next\.deployment\.origin\);\s*\n\s*void registerActiveDeviceIfPossible/, '先标记模式必须移除');
  assert.match(source, /registrationAttempts/, '会话内有界重试的尝试计数存在');
  // B3-F29：openNotificationFromInbox 的 item 参数类型对齐实现（读 deepLink），删除 as 断言。
  assert.doesNotMatch(source, /as InboxItem/, 'as InboxItem 断言必须删除');
});

test('the stop card renders the module note instead of claiming a cancellation that never happened (T07 #37 终审修复)', async () => {
  const { TaskDetailScreen } = await import('./screens/TaskDetailScreen.tsx');
  hooks().__reset();
  const base: import('@weknora/mobile-core').TaskDetailView = {
    taskId: 't', runId: 'r', title: '报告', lifecycle: 'completed', runStatus: 'failed', attention: 'none',
    executionStatus: 'failed', settlementStatus: 'settled', revision: 3, cursor: 4, incomplete: false,
    connection: 'drained', timeline: [], duplicateSeqs: [],
  };
  // stopProjection 观察到自然终态获胜：confirmed + note——那次取消并未发生，
  // 硬编码「停止已确认：运行已取消」等于谎报（Spec Story 23 隐藏后果）。
  const noted = TaskDetailScreen({ view: { ...base, stop: { phase: 'confirmed', since: '2026-09-25T00:00:00Z', note: 'run ended as failed before the stop landed' } }, loading: false, onRefresh: () => {} });
  const notedJson = JSON.stringify(noted);
  assert.ok(notedJson.includes('停止流程已结束：run ended as failed before the stop landed'), '带 note 的 confirmed 必须用中性文案呈现模块 note');
  assert.ok(!notedJson.includes('停止已确认：运行已取消'), '自然终态获胜时绝不声称一次未发生的取消');

  // 无 note 的 confirmed（观察到 canceled）：确认文案保持明确。
  const plain = TaskDetailScreen({ view: { ...base, stop: { phase: 'confirmed', since: '2026-09-25T00:00:00Z' } }, loading: false, onRefresh: () => {} });
  assert.ok(JSON.stringify(plain).includes('停止已确认：运行已取消'), '观察到 canceled 的 confirmed 仍明确确认取消');

  // requested / unknown 分支不受影响。
  const requested = TaskDetailScreen({ view: { ...base, stop: { phase: 'requested', since: '2026-09-25T00:00:00Z' } }, loading: false, onRefresh: () => {} });
  assert.ok(JSON.stringify(requested).includes('停止请求已发出，等待运行确认停止'));
});

test('the task budget route keeps an Expo Router screen consuming the Task Office interface only', async () => {
  const route = await import('./app/tasks/budget.tsx');
  assert.equal(typeof route.default, 'function', 'src/app/tasks/budget.tsx must default-export the Expo Router screen');
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  for (const relative of ['screens/TaskBudgetScreen.tsx', 'task-budget-view.ts', 'app/tasks/budget.tsx']) {
    const source = readFileSync(join(here, relative), 'utf8');
    assert.equal(/@weknora\/(api-client|contracts)/.test(source), false, `${relative} must consume the Task Office Interface only (module-seams §10)`);
  }
});

test('composition wires the concrete budget remote into the Task Office (source-level, parallel-batch guard)', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const source = readFileSync(join(here, 'composition.ts'), 'utf8');
  assert.match(source, /budget:\s*createMobileTaskBudgetRemote/, 'taskOfficeFor must assemble the budget remote on the authorized channel');
});

test('the task detail screen renders the code delivery receipt section with honest state copy', async () => {
  const { TaskDetailScreen } = await import('../src/screens/TaskDetailScreen.tsx');
  const view = {
    taskId: 's-1', runId: 'run-1', title: '修复问候语', lifecycle: 'active', runStatus: 'completed',
    attention: 'required', executionStatus: 'completed', settlementStatus: 'settled', revision: 3,
    cursor: 9, incomplete: false, connection: 'drained', timeline: [], duplicateSeqs: [],
  } as const;
  const delivery = {
    deliveryId: 'dlv-1', taskId: 's-1', runId: 'run-1', state: 'pushed', repo: 'octocat/hello',
    branch: 'weknora/task/s-1', baselineSha: 'b'.repeat(40), commitSha: 'c1f0', attention: true,
    updatedAt: '2026-09-24T00:00:30Z', remoteLogin: 'octocat',
  } as const;
  const tree = render(TaskDetailScreen, { view, loading: false, delivery, onRefresh: () => {} });
  // 区块是嵌套函数组件（stub createElement 不展开子树）：按既有范式（InboxRouteLifecycle→InboxScreen）
  // 先找到组件元素、再手动渲染该组件断言文案。
  const sectionElement = descendants(tree).find(({ type, props }) => typeof type === 'function' && 'delivery' in props);
  assert.ok(sectionElement, 'delivery section is present');
  const section = render(sectionElement!.type as (props: unknown) => unknown, sectionElement!.props);
  const text = JSON.stringify(section);
  assert.ok(text.includes('代码交付'), 'delivery section is present');
  assert.ok(text.includes('已推送，等待草稿 PR/MR 恢复'), 'pushed state uses honest copy');
  assert.ok(text.includes('octocat/hello'), 'repo is shown');
  assert.ok(text.includes('c1f0'), 'commit sha is shown');
  // 终审修复：delivered 新文案「草稿 PR/MR 已创建」与回执标签「PR/MR：」
  // 与 pushed 同等强度钉住（T24 #54 PR/MR 中性化文案的移动面断言补全）。
  const delivered = {
    deliveryId: 'dlv-1', taskId: 's-1', runId: 'run-1', state: 'delivered', repo: 'octocat/hello',
    branch: 'weknora/task/s-1', baselineSha: 'b'.repeat(40), commitSha: 'c1f0', prNumber: 1,
    prUrl: 'https://gitlab.com/octocat/hello/-/merge_requests/1', attention: false,
    updatedAt: '2026-09-24T00:00:30Z', remoteLogin: 'gl-user',
  } as const;
  const deliveredTree = render(TaskDetailScreen, { view, loading: false, delivery: delivered, onRefresh: () => {} });
  const deliveredElement = descendants(deliveredTree).find(({ type, props }) => typeof type === 'function' && 'delivery' in props);
  assert.ok(deliveredElement, 'delivered delivery section is present');
  const deliveredSection = render(deliveredElement!.type as (props: unknown) => unknown, deliveredElement!.props);
  const deliveredText = JSON.stringify(deliveredSection);
  assert.ok(deliveredText.includes('草稿 PR/MR 已创建'), 'delivered state uses neutral draft PR/MR copy');
  assert.ok(deliveredText.includes('PR/MR：'), 'receipt label is provider-neutral PR/MR');
  assert.ok(deliveredText.includes('https://gitlab.com/octocat/hello/-/merge_requests/1'), 'pr url is rendered');
  // 无交付时不渲染区块。
  const without = render(TaskDetailScreen, { view, loading: false, onRefresh: () => {} });
  assert.ok(!JSON.stringify(without).includes('代码交付'));
  assert.ok(
    !descendants(without).some(({ type, props }) => typeof type === 'function' && 'delivery' in props),
    'no delivery → no section element at all',
  );
});

test('composition exposes a delivery reader only under an authorized runtime', async () => {
  const { activeDeliveryReader } = await import('../src/composition.ts');
  // 未登录（无授权面）：reader 必须是 undefined（fail closed，不抛错）。
  assert.equal(activeDeliveryReader(), undefined, 'no runtime → no reader');
  // 授权面的「reader 非 undefined 且两次调用同实例」以既有 composition 缓存测试的源码断言
  // 方式为模板钉死实现机制：deliveryReaders 走 cachePut 按 deploymentScopeKey 记忆化。
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const source = readFileSync(join(here, 'composition.ts'), 'utf8');
  assert.match(source, /const deliveryReaders = new Map<string, DeliveryReader>/, 'deliveryReaders 缓存存在');
  assert.match(source, /cachePut\(deliveryReaders, deploymentScopeKey\(origin, tenantId\)/, 'reader 按 deployment scope key 记忆化（两次调用同实例）');
  assert.match(source, /createDeliveryReader\(\{ remote, lease: \(\) => activeRuntime\.scopeLease\(\) \}\)/, 'lease 由 Runtime 提供（切租户 fail closed）');
});

test('the composition guards offline-dangerous ports and persists projections through the scoped vault (T10)', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const source = readFileSync(join(here, 'composition.ts'), 'utf8');
  // 离线危险动作门（AC2）：backend(run=start)/interactions(approval=decide)/legacy(run=followUp) 全部经 gate
  assert.match(source, /guardTaskBackend\(remote/, 'taskOfficeFor must guard the start channel');
  assert.match(source, /guardInteractionBackend\(remote/, 'taskOfficeFor must guard the decide channel');
  assert.match(source, /guardLegacyTaskBackend\(/, 'taskOfficeFor must guard the legacy follow-up channel');
  assert.match(source, /createOfflineGate\(/, 'the gate must be constructed once at the composition root');
  // 加密投影持久化（AC1）：vault 在场时 store 走 Scoped Vault Adapter
  assert.match(source, /createVaultTaskProjectionStore\(\{\s*vault:\s*nativeScopedVault/, 'the task office store must be the scoped-vault adapter when the vault exists');
  assert.match(source, /: createInMemoryTaskProjectionStore\(\)/, 'vault absence must be an explicit in-memory decision');
  // 既有防线不回归（#35 源级断言，app-smoke.test.tsx:590）
  assert.match(source, /detail:\s*remote/, 'taskOfficeFor must still pass the remote as the detail port');
});

test('the New entry renders the dictation review surface: editable transcript, confirm and discard (AC2)', async () => {
  const { NewTaskScreen } = await import('./screens/NewTaskScreen.tsx');
  hooks().__reset();
  const events: string[] = [];
  const baseState = {
    draft: { text: '手写目标', agentId: 'a-general', budgetUpper: 0, attachments: [] as never[], knowledgeIds: [] as string[] },
    agents: [],
    knowledge: [],
    recommendation: { agent: null, basis: 'none' },
    readiness: { ready: true, blockingAttachments: [] },
    loading: false,
    submitting: false,
    inFlight: undefined,
  };
  const callbacks = {
    onUpdate: () => {}, onSetAttachments: () => {}, onToggleKnowledge: () => {},
    onSubmit: () => { events.push('submit'); }, onCancel: () => {}, onRefreshAgents: () => {},
  };
  const element = render(NewTaskScreen, {
    state: baseState,
    ...callbacks,
    dictation: { phase: 'review', transcript: '整理知识库' },
    onDictationBegin: () => { events.push('begin'); },
    onDictationFinish: () => { events.push('finish'); },
    onDictationCancel: () => { events.push('cancel'); },
    onDictationEditTranscript: (text: string) => { events.push(`edit:${text}`); },
    onDictationRetryTranscription: () => { events.push('retry'); },
    onDictationConfirmTranscript: () => { events.push('confirm'); },
    onDictationDiscardTranscript: () => { events.push('discard'); },
  });
  const transcriptInput = descendants(element).find(({ type, props }) => type === 'TextInput' && props.placeholder === '转写草稿（确认前可编辑）');
  assert.ok(transcriptInput, 'review 态渲染可编辑转写输入');
  (transcriptInput!.props.onChangeText as (text: string) => void)('整理知识库（已校对）');
  assert.deepEqual(events, ['edit:整理知识库（已校对）'], '编辑先于确认（AC2：转写错误可修正）');
  const buttons = descendants(element).filter(({ type }) => type === 'Button').map(({ props }) => props.title);
  assert.equal(buttons.includes('Use transcript'), true);
  assert.equal(buttons.includes('Discard transcript'), true);
  (descendants(element).find(({ type, props }) => type === 'Button' && props.title === 'Use transcript')!.props.onPress as () => void)();
  assert.deepEqual(events, ['edit:整理知识库（已校对）', 'confirm'], '确认是显式用户动作，不是转写返回即提交');
  assert.ok(!events.includes('submit'), '确认动作本身绝不触发 Submit');
});

test('cancelling dictation from the New entry only cancels the recording surface, never the goal draft (AC1)', async () => {
  const { NewTaskScreen } = await import('./screens/NewTaskScreen.tsx');
  hooks().__reset();
  const events: string[] = [];
  const baseState = {
    draft: { text: '手写目标', agentId: 'a-general', budgetUpper: 0, attachments: [] as never[], knowledgeIds: [] as string[] },
    agents: [], knowledge: [],
    recommendation: { agent: null, basis: 'none' },
    readiness: { ready: true, blockingAttachments: [] },
    loading: false, submitting: false, inFlight: undefined,
  };
  const callbacks = {
    onUpdate: () => {}, onSetAttachments: () => {}, onToggleKnowledge: () => {},
    onSubmit: () => { events.push('submit'); }, onCancel: () => { events.push('keep-draft'); }, onRefreshAgents: () => {},
  };
  const element = render(NewTaskScreen, {
    state: baseState,
    ...callbacks,
    dictation: { phase: 'recording' },
    onDictationBegin: () => {}, onDictationFinish: () => { events.push('finish'); },
    onDictationCancel: () => { events.push('cancel'); }, onDictationEditTranscript: () => {},
    onDictationRetryTranscription: () => {}, onDictationConfirmTranscript: () => {}, onDictationDiscardTranscript: () => {},
  });
  const goalInput = descendants(element).find(({ type, props }) => type === 'TextInput' && props.placeholder === '今天想完成什么？');
  assert.ok(goalInput, '录音态下手打目标输入仍在屏');
  assert.equal((goalInput!.props as { value?: string }).value, '手写目标', '手打文字原样保留（不被清空）');
  const buttons = descendants(element).filter(({ type }) => type === 'Button').map(({ props }) => props.title);
  assert.equal(buttons.includes('Cancel recording'), true);
  assert.equal(buttons.includes('Stop dictation'), true);
  (descendants(element).find(({ type, props }) => type === 'Button' && props.title === 'Cancel recording')!.props.onPress as () => void)();
  assert.deepEqual(events, ['cancel'], '取消只作用于听写面；Submit/Keep draft 均未被触发');
});

test('a denied microphone leaves typing and submission fully usable (拒权不破坏文字输入)', async () => {
  const { NewTaskScreen } = await import('./screens/NewTaskScreen.tsx');
  hooks().__reset();
  const baseState = {
    draft: { text: '手写目标', agentId: 'a-general', budgetUpper: 0, attachments: [] as never[], knowledgeIds: [] as string[] },
    agents: [], knowledge: [],
    recommendation: { agent: null, basis: 'none' },
    readiness: { ready: true, blockingAttachments: [] },
    loading: false, submitting: false, inFlight: undefined,
  };
  const callbacks = {
    onUpdate: () => {}, onSetAttachments: () => {}, onToggleKnowledge: () => {},
    onSubmit: () => {}, onCancel: () => {}, onRefreshAgents: () => {},
  };
  const element = render(NewTaskScreen, {
    state: baseState,
    ...callbacks,
    dictation: { phase: 'denied' },
    onDictationBegin: () => {}, onDictationFinish: () => {}, onDictationCancel: () => {},
    onDictationEditTranscript: () => {}, onDictationRetryTranscription: () => {},
    onDictationConfirmTranscript: () => {}, onDictationDiscardTranscript: () => {},
  });
  const goalInput = descendants(element).find(({ type, props }) => type === 'TextInput' && props.placeholder === '今天想完成什么？');
  assert.ok(goalInput, '拒权态下手打目标输入仍在屏');
  assert.equal((goalInput!.props as { value?: string }).value, '手写目标', '手打文字原样保留（不清空）');
  assert.equal((goalInput!.props as { editable?: boolean }).editable, true, '输入未被锁死');
  const buttons = descendants(element).filter(({ type }) => type === 'Button').map(({ props }) => props.title);
  assert.equal(buttons.includes('Submit task'), true, '提交通道不受拒权影响');
  const text = descendants(element).filter(({ type }) => type === 'Text').flatMap(({ props }) => props.children).join(' ');
  assert.ok(text.includes('麦克风权限被拒绝'), '拒权有如实文案');
});

test('the /new route wires dictation through the composition root; confirmed text enters the goal draft, never the wire', async () => {
  const { readFileSync } = await import('node:fs');
  const routeSource = readFileSync(resolve(workspaceRoot, 'apps/mobile/src/app/new.tsx'), 'utf8');
  const compositionSource = readFileSync(resolve(workspaceRoot, 'apps/mobile/src/composition.ts'), 'utf8');
  const screenSource = readFileSync(resolve(workspaceRoot, 'apps/mobile/src/screens/NewTaskScreen.tsx'), 'utf8');
  assert.match(routeSource, /activeDictation\(\)/, '路由经组合根取听写模块（module-seams §10：Screen 不见 wire）');
  assert.match(routeSource, /confirmTranscript\(\)/, 'AC2：确认动作显式调用模块的 confirmTranscript');
  assert.match(routeSource, /applyConfirmedDictation/, '确认文字必须经 applyConfirmedDictation 并入目标草稿（含 500 字截断）');
  assert.match(routeSource, /dictation\.cancel\(\)/, '卸载兜底：录音/转写在途时取消（不留悬空麦克风与在途派发）');
  assert.equal(/mobile\/voice/.test(routeSource) || /mobile\/voice/.test(screenSource), false, '屏/路由不出现 wire 路径');
  assert.match(compositionSource, /createMobileVoiceTranscriptionRemote/, '转写 Remote 只在组合根装配');
  assert.match(compositionSource, /createNativeDictationCaptureIfAvailable/, '原生捕获 Adapter 只在组合根探测');
});
