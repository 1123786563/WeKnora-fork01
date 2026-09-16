import assert from 'node:assert/strict';
import test from 'node:test';
import {
  canAutoResumeRun,
  createRunLifecyclePersistence,
  deserializeRunLifecycle,
  initialRunLifecycle,
  serializeRunLifecycle,
  shouldEndSendingForStreamEvent,
  shouldApplyHydratedLifecycle,
  transitionRunLifecycle,
} from './run-lifecycle.ts';

test('a stream error ends the sending UI immediately like Vue chat', () => {
  assert.equal(shouldEndSendingForStreamEvent('error'), true);
  assert.equal(shouldEndSendingForStreamEvent('complete'), true);
  assert.equal(shouldEndSendingForStreamEvent('stop'), true);
  assert.equal(shouldEndSendingForStreamEvent('answer'), false);
});

test('a user stop is terminal and keeps the assistant id for remote cancellation', () => {
  const running = transitionRunLifecycle(initialRunLifecycle(), { type: 'start' });
  const withAssistant = transitionRunLifecycle(running, { type: 'assistant-message', id: 'assistant-1' });
  const stopped = transitionRunLifecycle(withAssistant, { type: 'user-stop' });

  assert.deepEqual(stopped, { status: 'stopped', assistantMessageId: 'assistant-1' });
  assert.equal(canAutoResumeRun(stopped), false);
});

test('a background interruption remains resumable and active recovery starts a new run', () => {
  const running = transitionRunLifecycle(initialRunLifecycle(), { type: 'start' });
  const interrupted = transitionRunLifecycle(running, { type: 'background' });

  assert.equal(interrupted.status, 'background-interrupted');
  assert.equal(canAutoResumeRun(interrupted), true);

  const resumed = transitionRunLifecycle(interrupted, { type: 'start', assistantMessageId: 'assistant-1' });
  assert.deepEqual(resumed, { status: 'running', assistantMessageId: 'assistant-1' });
});

test('failed and completed runs cannot be resumed from an incomplete persisted row', () => {
  for (const event of [{ type: 'failure' as const }, { type: 'complete' as const }]) {
    const terminal = transitionRunLifecycle(
      transitionRunLifecycle(initialRunLifecycle(), { type: 'start' }),
      event,
    );
    assert.equal(canAutoResumeRun(terminal), false);
  }
});

test('terminal lifecycle states are not overwritten by late stream cleanup', () => {
  const started = transitionRunLifecycle(initialRunLifecycle(), { type: 'start' });
  const stopped = transitionRunLifecycle(started, { type: 'user-stop' });
  const failed = transitionRunLifecycle(started, { type: 'failure' });

  assert.deepEqual(transitionRunLifecycle(stopped, { type: 'complete' }), stopped);
  assert.deepEqual(transitionRunLifecycle(failed, { type: 'complete' }), failed);
});

test('a stopped lifecycle survives persistence without inventing server state', () => {
  const stopped = { status: 'stopped' as const, assistantMessageId: 'assistant-1' };
  assert.deepEqual(deserializeRunLifecycle(serializeRunLifecycle(stopped)), stopped);
  assert.deepEqual(deserializeRunLifecycle('{"status":"unknown"}'), initialRunLifecycle());
});

test('lifecycle persistence is scoped to a chat session', async () => {
  const values = new Map<string, string>();
  const persistence = createRunLifecyclePersistence({
    async getItemAsync(key) { return values.get(key) ?? null; },
    async setItemAsync(key, value) { values.set(key, value); },
  });
  const stopped = { status: 'stopped' as const, assistantMessageId: 'assistant-1' };

  await persistence.write('session-1', stopped);

  assert.deepEqual(await persistence.read('session-1'), stopped);
  assert.deepEqual(await persistence.read('session-2'), initialRunLifecycle());
});

test('late lifecycle hydration cannot overwrite a local mutation', () => {
  assert.equal(shouldApplyHydratedLifecycle('session-1', 'session-1', 4, 4), true);
  assert.equal(shouldApplyHydratedLifecycle('session-1', 'session-1', 4, 5), false);
  assert.equal(shouldApplyHydratedLifecycle('session-1', 'session-2', 4, 4), false);
});

test('lifecycle writes for one session are serialized so a late store completion cannot restore stale state', async () => {
  const pending: Array<() => void> = [];
  const values = new Map<string, string>();
  const persistence = createRunLifecyclePersistence({
    async getItemAsync(key) { return values.get(key) ?? null; },
    async setItemAsync(key, value) {
      await new Promise<void>((resolve) => pending.push(resolve));
      values.set(key, value);
    },
  });
  const running = { status: 'running' as const };
  const stopped = { status: 'stopped' as const, assistantMessageId: 'assistant-1' };
  const first = persistence.write('session-1', running);
  const second = persistence.write('session-1', stopped);
  await new Promise<void>((resolve) => setImmediate(resolve));
  assert.equal(pending.length, 1);
  pending.shift()!();
  await new Promise<void>((resolve) => setImmediate(resolve));
  assert.equal(pending.length, 1);
  pending.shift()!();
  await Promise.all([first, second]);
  assert.deepEqual(await persistence.read('session-1'), stopped);
});
