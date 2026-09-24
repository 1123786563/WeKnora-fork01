import test from 'node:test';
import assert from 'node:assert/strict';
import { LEGACY_TASK_NEW_RUN_REASON, createScenarioLegacyTaskBackend, legacyTaskGates } from './legacy-tasks.ts';

test('the legacy gate supports ordinary follow-up and refuses every new-security-semantics intent', () => {
  const gates = legacyTaskGates();
  assert.deepEqual(gates['follow-up'], { state: 'supported', reason: 'ordinary follow-up keeps the existing chat semantics' });
  for (const intent of ['run-command', 'decision', 'budget', 'agent-version'] as const) {
    assert.deepEqual(gates[intent], { state: 'unavailable', reason: LEGACY_TASK_NEW_RUN_REASON }, intent);
  }
  assert.equal(LEGACY_TASK_NEW_RUN_REASON, 'legacy task has no Run; a new Run admission is required');
});

test('the scenario backend records calls and defaults to honest empties', async () => {
  const backend = createScenarioLegacyTaskBackend();
  const page = await backend.list({});
  assert.deepEqual(page, { items: [] });
  assert.deepEqual(await backend.history('lg-1'), []);
  await backend.followUp({ taskId: 'lg-1', question: '继续' });
  assert.deepEqual(backend.calls, [
    { kind: 'list', input: {} },
    { kind: 'history', taskId: 'lg-1' },
    { kind: 'followUp', input: { taskId: 'lg-1', question: '继续' } },
  ]);
});

test('the scenario backend forwards scripted handlers', async () => {
  const backend = createScenarioLegacyTaskBackend({
    list: async () => ({ items: [{ taskId: 'lg-1', title: '旧聊天', attention: 'none', updatedAt: '2026-09-20T08:00:00Z' }], nextCursor: 'c1' }),
    history: async () => [{ messageId: 'm1', role: 'user', content: '第一问', createdAt: '2026-09-20T08:00:01Z' }],
    followUp: async () => undefined,
  });
  const page = await backend.list({ search: '旧' });
  assert.equal(page.items[0]!.taskId, 'lg-1');
  assert.equal(page.nextCursor, 'c1');
  const messages = await backend.history('lg-1');
  assert.equal(messages[0]!.role, 'user');
  await backend.followUp({ taskId: 'lg-1', question: '第二问' });
  assert.equal(backend.calls.length, 3);
});
