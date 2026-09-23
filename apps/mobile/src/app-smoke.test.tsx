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
  'expo-router': "module.exports = { Stack: function Stack() { return null; }, router: { replace() {}, navigate() {} } }",
  'react-native': "module.exports = { View: 'View', Text: 'Text', TextInput: 'TextInput', Button: 'Button' }",
  react: "let values = []; let cursor = 0; let pendingEffects = []; let effectCleanups = []; module.exports = { __beginRender() { cursor = 0; }, __reset() { values = []; cursor = 0; pendingEffects = []; effectCleanups = []; }, useState(initial) { const index = cursor++; if (!(index in values)) values[index] = initial; return [values[index], (next) => { values[index] = next; }]; }, useRef(value) { const index = cursor++; if (!(index in values)) values[index] = { current: value }; return values[index]; }, useEffect(setup) { pendingEffects.push(setup); }, __mount() { for (const setup of pendingEffects.splice(0)) effectCleanups.push(setup()); }, __unmount() { for (const cleanup of effectCleanups.splice(0)) { if (typeof cleanup === 'function') cleanup(); } }, useSyncExternalStore(_subscribe, getSnapshot) { return getSnapshot(); }, createElement(type, props, ...children) { return { type, props: { ...(props || {}), ...(children.length === 0 ? {} : { children: children.length === 1 ? children[0] : children }) } }; } };",
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
    snapshot: { surface: 'authorized', deployment: { origin: 'https://weknora.example.test', label: 'WeKnora' }, identity: { userId: 'member-1', activeTenantId: 'tenant-1' } },
    onSignIn: async () => {}, onBeginOidc: async () => {}, onSignOut: async () => {},
    onActivateTenant: async () => {},
  });
  assert.equal(authorized.type.name, 'AuthorizedLandingScreen');
  const authorizedText = descendants(render(authorized.type, authorized.props)).filter(({ type }) => type === 'Text').flatMap(({ props }) => props.children).join(' ');
  assert.equal(authorizedText.includes('WeKnora Task Office'), true);
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

test('authorized landing switches tenants only through non-active options', async () => {
  const { AuthorizedLandingScreen } = await import('./screens/AuthorizedLandingScreen.tsx');
  hooks().__reset();
  const activated: string[] = [];
  const element = render(AuthorizedLandingScreen, {
    deploymentLabel: 'WeKnora', userId: 'member-1', tenantId: '7',
    tenants: [{ id: '7', name: 'Acme', active: true }, { id: '9', name: 'Beta', active: false }],
    onSignOut: async () => {},
    onActivateTenant: async (id: string) => { activated.push(id); },
  });

  const texts = descendants(element).filter(({ type }) => type === 'Text').flatMap(({ props }) => props.children);
  assert.equal(texts.includes('Acme (active)'), true);
  const buttons = descendants(element).filter(({ type }) => type === 'Button');
  const switchButton = buttons.find(({ props }) => props.title === 'Switch to Beta');
  assert.ok(switchButton, 'inactive tenant must render a switch button');
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

  assert.equal(surface.type.name, 'AuthorizedLandingScreen');
  assert.deepEqual((surface.props as { tenants: Array<{ id: string; name?: string; active: boolean }> }).tenants, [
    { id: '7', name: 'Acme', active: true },
    { id: '9', name: 'Beta', active: false },
  ]);
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

test('the resources route and landing entry consume the shelf interface only', async () => {
  const route = await import('./app/resources.tsx');
  assert.equal(typeof route.default, 'function', 'src/app/resources.tsx must default-export the Expo Router screen');

  const { AuthorizedLandingScreen } = await import('./screens/AuthorizedLandingScreen.tsx');
  hooks().__reset();
  const element = render(AuthorizedLandingScreen, {
    deploymentLabel: 'WeKnora',
    userId: 'member-1',
    tenantId: '7',
    tenants: [{ id: '7', name: 'Acme', active: true }],
    onSignOut: async () => {},
    onActivateTenant: async () => {},
  });
  const buttons = descendants(element).filter(({ type }) => type === 'Button').map(({ props }) => props.title);
  assert.equal(buttons.includes('Open Resources'), true);
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
