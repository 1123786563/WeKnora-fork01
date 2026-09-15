import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import test from 'node:test';
import config from '../vite.config.ts';

test('Vite dev and preview proxy Vue-compatible API and file roots', () => {
  const expectedPaths = ['/api', '/files'];
  const serverProxy = config.server?.proxy as Record<string, Record<string, unknown>>;
  const previewProxy = config.preview?.proxy as Record<string, Record<string, unknown>>;

  assert.deepEqual(Object.keys(serverProxy), expectedPaths);
  assert.deepEqual(Object.keys(previewProxy), expectedPaths);

  for (const path of expectedPaths) {
    for (const proxy of [serverProxy[path], previewProxy[path]]) {
      assert.equal(proxy.target, 'http://localhost:8080');
      assert.equal(proxy.changeOrigin, true);
      assert.equal(proxy.secure, false);
      assert.equal(proxy.ws, true);
    }
  }
});

test('Vite proxy target honors the React-local override before the legacy backend variable', () => {
  const configPath = new URL('../vite.config.ts', import.meta.url).pathname;
  const output = execFileSync(process.execPath, ['--import', 'tsx', '--eval', `
    const { default: config } = await import(${JSON.stringify(configPath)});
    console.log(config.server.proxy['/api'].target);
  `], {
    env: { ...process.env, VITE_DEV_PROXY_TARGET: 'http://vite-target:9001', FRONTEND_BACKEND_URL: 'http://legacy-target:9002' },
    encoding: 'utf8',
  }).trim();

  assert.equal(output, 'http://vite-target:9001');
});
