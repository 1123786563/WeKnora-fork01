import assert from 'node:assert/strict';
import test from 'node:test';
import { chatAppStateAction } from './appstate.ts';
import { initialRunLifecycle, transitionRunLifecycle } from './run-lifecycle.ts';

test('backgrounding an active chat stream requests local cancellation', () => {
  assert.equal(chatAppStateAction('background', true, true), 'abort');
});

test('returning active with an authenticated session and no stream requests recovery', () => {
  assert.equal(chatAppStateAction('active', true, false), 'resume');
});

test('active state never starts a second recovery while a stream is still owned', () => {
  assert.equal(chatAppStateAction('active', true, true), 'ignore');
  assert.equal(chatAppStateAction('active', false, false), 'ignore');
});

test('active state does not recover a user-stopped run', () => {
  const stopped = transitionRunLifecycle(initialRunLifecycle(), { type: 'user-stop' });
  assert.equal(chatAppStateAction('active', true, false, stopped), 'ignore');
});

test('active state does not recover a failed or completed run', () => {
  const started = transitionRunLifecycle(initialRunLifecycle(), { type: 'start' });
  const failed = transitionRunLifecycle(started, { type: 'failure' });
  const completed = transitionRunLifecycle(started, { type: 'complete' });
  assert.equal(chatAppStateAction('active', true, false, failed), 'ignore');
  assert.equal(chatAppStateAction('active', true, false, completed), 'ignore');
});
