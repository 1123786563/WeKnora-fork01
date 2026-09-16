import test from 'node:test';
import assert from 'node:assert/strict';
import { createSdkPort } from './sdk-port.ts';

test('public SDK adapter rejects pre-cancelled create without starting work', async () => {
  let calls = 0;
  const client = { agents: { create: async () => { calls++; return { id: 'never' }; } } } as never;
  const controller = new AbortController();
  controller.abort();
  await assert.rejects(createSdkPort(client).create({ cwd: '/safe', prompt: 'hi', provider: 'p' }, { signal: controller.signal }), /PASEO_CANCEL_UNAVAILABLE/);
  assert.equal(calls, 0);
});
