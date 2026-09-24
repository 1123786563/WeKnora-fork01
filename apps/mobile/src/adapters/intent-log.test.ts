import test from 'node:test';
import assert from 'node:assert/strict';
import { TaskOfficeError } from '@weknora/mobile-core';
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

const bigRecord = (id: string, body: string) => ({ ...record, requestId: id, goal: { ...goal, text: body } });

test('the byte budget evicts the oldest entries once the serialized log exceeds 1536 bytes', async () => {
  const secure = memoryStore();
  const log = createSecureIntentLog(secure);
  await log.save(bigRecord('req-a', 'a'.repeat(700)));
  assert.ok(await log.load('req-a'), 'a single ~0.9KB record fits the budget');
  await log.save(bigRecord('req-b', 'b'.repeat(700)));
  assert.equal(await log.load('req-a'), undefined, 'the oldest entry is evicted when the whole log exceeds the 1536-byte budget');
  assert.ok(await log.load('req-b'), 'the newest intent survives the eviction');
});

test('a single record beyond the budget fails explicitly instead of hitting the SecureStore limit (B3-F38)', async () => {
  const secure = memoryStore();
  const log = createSecureIntentLog(secure);
  const hugeGoal = {
    text: '长'.repeat(500), // 500 中文 ≈1500B
    agentId: 'a-1', budgetUpper: 200,
    knowledgeIds: Array.from({ length: 24 }, (_unused, index) => `kb-${index}-0123456789abcdef0123456789abcdef`), // 24×39 ≈ 936B
    attachments: [],
  };
  await assert.rejects(
    log.save({ requestId: 'req-big', sessionId: 'session-1', goal: hugeGoal, scope, persistedAt: '2026-09-24T00:00:00Z' }),
    (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INVALID_INPUT',
    '单条超限必须显式抛可行动错误码，而不是 setItemAsync 底层错误让提交永久失败',
  );
  assert.equal(secure.dump(), null, '超限记录不得落盘（部分状态）');
});

test('a single oversized record is refused with an actionable error code (B3-F38 supersedes the keep-alone adjudication)', async () => {
  const secure = memoryStore();
  const log = createSecureIntentLog(secure);
  await assert.rejects(
    log.save(bigRecord('req-1', 'z'.repeat(1600))),
    (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INVALID_INPUT',
    '单条超限：maxLength 的 500 字估算漏算 knowledgeIds/attachments——合法组合越限后必须有可行动错误码而非底层抛错',
  );
  assert.equal(secure.dump(), null, '拒绝时不落任何部分状态');
});

test('concurrent saves serialize instead of overwriting each other (B3-F51)', async () => {
  const secure = memoryStore();
  const log = createSecureIntentLog(secure);
  await Promise.all([
    log.save(record),
    log.save({ ...record, requestId: 'req-2', sessionId: 'session-78' }),
  ]);
  assert.equal((await log.load('req-1'))?.sessionId, 'session-77', '先写记录不得被后写整包覆盖丢失');
  assert.equal((await log.load('req-2'))?.sessionId, 'session-78');
});

test('records within the byte budget are all kept', async () => {
  const secure = memoryStore();
  const log = createSecureIntentLog(secure);
  await log.save(record);
  await log.save({ ...record, requestId: 'req-2', sessionId: 'session-78' });
  assert.ok(await log.load('req-1'));
  assert.ok(await log.load('req-2'), 'healthy small logs are never trimmed');
});
