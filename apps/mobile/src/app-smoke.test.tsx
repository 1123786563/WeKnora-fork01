import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
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

test('app module exports an application root', async () => {
  const layout = await import('./app/_layout.tsx');
  assert.equal(
    typeof layout.default,
    'function',
    'src/app/_layout.tsx must default-export the application root component',
  );
});

test('deployment login accepts only normalized HTTPS origins without embedded credentials', async () => {
  const { validatedDeploymentOrigin } = await import('./screens/DeploymentLoginScreen.tsx');
  assert.equal(validatedDeploymentOrigin('https://weknora.example.test/'), 'https://weknora.example.test');
  assert.equal(validatedDeploymentOrigin('http://weknora.example.test'), undefined);
  assert.equal(validatedDeploymentOrigin('https://member:password@weknora.example.test'), undefined);
  assert.equal(validatedDeploymentOrigin('https://weknora.example.test/path'), undefined);
});

test('surface routing keeps upgrade-required free of authorized controls and guards incomplete identity', async () => {
  const { RuntimeSurface } = await import('./composition.ts');
  const safe = RuntimeSurface({
    snapshot: { surface: 'upgrade-required', deployment: { origin: 'https://weknora.example.test', label: 'WeKnora' }, reason: 'protocol-mismatch' },
    onSignIn: async () => {}, onBeginOidc: async () => {}, onSignOut: async () => {},
  });
  assert.equal(safe.type.name, 'UpgradeRequiredScreen');

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
