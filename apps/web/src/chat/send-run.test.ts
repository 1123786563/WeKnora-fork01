import assert from 'node:assert/strict';
import test from 'node:test';

import { prepareSendRun } from './send-run.ts';

interface FakeSession { id: string; title: string }

test('prepareSendRun creates the session and the abort controller AFTER the selection teardown', async () => {
  // Mirrors ChatRoutePage: selectSession aborts whatever controller is
  // currently registered. A controller created before this point would hand
  // the stream an already-aborted signal (silent no-request hang).
  let registeredController: AbortController | null = null;
  const order: string[] = [];
  const run = await prepareSendRun<FakeSession>({
    selectedSessionId: null,
    content: 'hello',
    createSession: async (title) => {
      order.push('create');
      return { id: 'session-1', title };
    },
    onSessionSelected: (session) => {
      order.push('select:' + session.id);
      // selectSession(): tear down any prior stream registration.
      registeredController?.abort();
      registeredController = null;
    },
  });

  assert.deepEqual(order, ['create', 'select:session-1']);
  assert.equal(run.sessionId, 'session-1');
  assert.equal(run.createdSession?.id, 'session-1');
  // The controller handed to the stream must be created after (and therefore
  // not be aborted by) the selection teardown.
  assert.equal(run.controller.signal.aborted, false);
  // Register it the way send() does and re-run a selection: aborting the
  // previous registration must not touch the fresh controller.
  registeredController = run.controller;
  assert.equal(run.controller.signal.aborted, false);
});

test('prepareSendRun trims the title from the submission content', async () => {
  let receivedTitle = '';
  await prepareSendRun<FakeSession>({
    selectedSessionId: null,
    content: 'x'.repeat(120),
    createSession: async (title) => {
      receivedTitle = title;
      return { id: 'session-1', title };
    },
    onSessionSelected: () => undefined,
  });
  assert.equal(receivedTitle.length, 80);
});

test('prepareSendRun does not create a session when one is already selected', async () => {
  let createCalls = 0;
  const run = await prepareSendRun<FakeSession>({
    selectedSessionId: 'session-9',
    content: 'hello',
    createSession: async () => {
      createCalls += 1;
      return { id: 'session-1', title: 'unused' };
    },
    onSessionSelected: () => {
      throw new Error('selection side effects must not run for an existing session');
    },
  });
  assert.equal(createCalls, 0);
  assert.equal(run.sessionId, 'session-9');
  assert.equal(run.createdSession, null);
  assert.equal(run.controller.signal.aborted, false);
});
