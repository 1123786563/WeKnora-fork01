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
