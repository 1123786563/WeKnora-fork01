import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createNativeNetworkStatusIfAvailable } from './network-status.ts';

const here = dirname(fileURLToPath(import.meta.url));

test('a native network status is only returned when expo-network resolves (fail closed to undefined)', () => {
  // Node 测试链无 expo-network：结构级断言惰性 require + 缺席返回 undefined（与 push-token/device-identity 同模式）
  const source = readFileSync(join(here, 'network-status.ts'), 'utf8');
  assert.match(source, /require\('expo-network'\)/);
  assert.match(source, /catch/, 'resolution failure must be contained, never thrown');
  const status = createNativeNetworkStatusIfAvailable();
  if (status !== undefined) {
    // 若环境意外可解析（真机），仍必须给出布尔判决而非抛错
    assert.equal(typeof status.online(), 'object'); // Promise
  }
});

test('the adapter maps isInternetReachable strictly: null/absent counts as offline', () => {
  const source = readFileSync(join(here, 'network-status.ts'), 'utf8');
  assert.match(source, /isInternetReachable === true/, 'only an explicit true counts as online (fail closed)');
});
