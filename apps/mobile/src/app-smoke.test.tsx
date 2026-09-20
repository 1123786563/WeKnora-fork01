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
  'expo-router': 'module.exports = { Stack: function Stack() { return null; } }',
  'react-native': "module.exports = { View: 'View', Text: 'Text', TextInput: 'TextInput', Button: 'Button' }",
  react: "let values = []; let cursor = 0; module.exports = { __beginRender() { cursor = 0; }, __reset() { values = []; cursor = 0; }, useState(initial) { const index = cursor++; if (!(index in values)) values[index] = initial; return [values[index], (next) => { values[index] = next; }]; }, useRef(value) { return { current: value }; }, useEffect() {}, useSyncExternalStore(_subscribe, getSnapshot) { return getSnapshot(); }, createElement(type, props, ...children) { return { type, props: { ...(props || {}), ...(children.length === 0 ? {} : { children: children.length === 1 ? children[0] : children }) } }; } };",
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

function hooks(): { __beginRender(): void; __reset(): void } {
  return require('react') as { __beginRender(): void; __reset(): void };
}

function render(component: (props: any) => unknown, props: any): unknown {
  hooks().__beginRender();
  return component(props);
}

function descendants(node: unknown): Array<{ type: unknown; props: Record<string, unknown> }> {
  if (!node || typeof node !== 'object') return [];
  const element = node as { type?: unknown; props?: Record<string, unknown> };
  const here = element.props ? [{ type: element.type, props: element.props }] : [];
  const children = element.props?.children;
  return [...here, ...(Array.isArray(children) ? children : [children]).flatMap(descendants)];
}

test('app module exports an application root', async () => {
  const layout = await import('./app/_layout.tsx');
  assert.equal(
    typeof layout.default,
    'function',
    'src/app/_layout.tsx must default-export the application root component',
  );
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
  });
  assert.equal(safe.type.name, 'UpgradeRequiredScreen');
  const safeControls = descendants(render(safe.type, safe.props)).filter(({ type }) => type === 'Button').map(({ props }) => props.title);
  const safeText = descendants(render(safe.type, safe.props)).filter(({ type }) => type === 'Text').flatMap(({ props }) => props.children).join(' ');
  assert.equal(safeControls.includes('Open Task Office'), false);
  assert.equal(safeText.includes('Task Office'), false);

  const invalidAuthorized = RuntimeSurface({
    snapshot: { surface: 'authorized', deployment: { origin: 'https://weknora.example.test', label: 'WeKnora' }, identity: { userId: 'member-1' } },
    onSignIn: async () => {}, onBeginOidc: async () => {}, onSignOut: async () => {},
  });
  assert.equal(invalidAuthorized.type.name, 'UpgradeRequiredScreen');

  const authorized = RuntimeSurface({
    snapshot: { surface: 'authorized', deployment: { origin: 'https://weknora.example.test', label: 'WeKnora' }, identity: { userId: 'member-1', activeTenantId: 'tenant-1' } },
    onSignIn: async () => {}, onBeginOidc: async () => {}, onSignOut: async () => {},
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
