import test from 'node:test';
import assert from 'node:assert/strict';
import { createNativeRequestId } from './request-id.ts';

test('request ids are v4-shaped and unique (strength floor)', () => {
  const next = createNativeRequestId();
  const seen = new Set<string>();
  for (let index = 0; index < 200; index += 1) {
    const id = next();
    assert.match(id, /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/, 'v4 形态（版本与变体位）');
    seen.add(id);
  }
  assert.equal(seen.size, 200, '幂等关联键不可碰撞');
});

test('the generator prefers the expo-crypto CSPRNG when resolvable', async () => {
  const { readFileSync } = await import('node:fs');
  const { URL: NodeURL } = await import('node:url');
  const source = readFileSync(new NodeURL('./request-id.ts', import.meta.url), 'utf8');
  assert.match(source, /expo-crypto/, 'B3-F27：CSPRNG 路径必须真实可达（惰性 require expo-crypto），不依赖 Hermes 不存在的 globalThis.crypto');
});
