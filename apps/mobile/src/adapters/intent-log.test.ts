import test from 'node:test';
import assert from 'node:assert/strict';
import { createSecureIntentLog } from './intent-log.ts';
import type { SecureStorePort } from './secure-store.ts';

function memoryStore(): SecureStorePort & { dump(): string | null; corrupt(body: string): void } {
  let value: string | null = null;
  return {
    async getItemAsync() { return value; },
    async setItemAsync(_key: string, next: string) { value = next; },
    async deleteItemAsync() { value = null; },
    dump: () => value,
    corrupt: (body: string) => { value = body; },
  };
}

const scope = { origin: 'https://weknora.example.test', tenantID: 'tenant-1', userID: 'user-1' };
const goal = { text: '整理周报', agentId: 'a-1', budgetUpper: 200 };
const record = { requestId: 'req-1', sessionId: 'session-77', goal, scope, persistedAt: '2026-09-24T00:00:00Z' };

test('intent records survive a simulated restart and stay scope-tagged', async () => {
  const secure = memoryStore();
  await createSecureIntentLog(secure).save(record);
  // 模拟 App 重启：全新实例、同一持久键——load/listScope 直接读盘，无需 hydrate
  const second = createSecureIntentLog(secure);
  const loaded = await second.load('req-1');
  assert.deepEqual(loaded, record);
  assert.deepEqual((await second.listScope(scope)).map((row) => row.requestId), ['req-1']);
  assert.deepEqual(await second.listScope({ ...scope, tenantID: 'tenant-2' }), [], 'intent records never leak across scopes');
});

test('a missing record reads as undefined without failing', async () => {
  const log = createSecureIntentLog(memoryStore());
  assert.equal(await log.load('req-x'), undefined);
  assert.deepEqual(await log.listScope(scope), []);
});

test('corrupted persisted JSON reads as an empty log instead of crashing the New flow', async () => {
  const secure = memoryStore();
  secure.corrupt('{not json');
  const log = createSecureIntentLog(secure);
  assert.deepEqual(await log.listScope(scope), []);
  await log.save(record);
  assert.equal((await createSecureIntentLog(secure).load('req-1'))?.sessionId, 'session-77', 'a fresh write repairs the log');
});

test('remove deletes exactly one record', async () => {
  const secure = memoryStore();
  const log = createSecureIntentLog(secure);
  await log.save(record);
  await log.save({ ...record, requestId: 'req-2', sessionId: 'session-78' });
  await log.remove?.('req-1');
  assert.equal(await log.load('req-1'), undefined);
  assert.equal((await log.load('req-2'))?.sessionId, 'session-78');
});

test('malformed rows are skipped while healthy rows survive', async () => {
  const secure = memoryStore();
  secure.corrupt(JSON.stringify([record, 'garbage-string', { requestId: 42 }]));
  const log = createSecureIntentLog(secure);
  assert.deepEqual((await log.listScope(scope)).map((row) => row.requestId), ['req-1']);
});
