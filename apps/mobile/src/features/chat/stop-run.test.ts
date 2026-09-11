import assert from 'node:assert/strict';
import test from 'node:test';
import { stopChatRun } from './stop-run.ts';

test('stopping a chat aborts local streaming before waiting for the remote stop', async () => {
  const events: string[] = [];
  let resolveRemote!: () => void;
  const remote = new Promise<void>((resolve) => { resolveRemote = resolve; });
  const pending = stopChatRun(
    () => remote,
    () => { events.push('abort'); },
    () => { events.push('stopped'); },
  );

  await Promise.resolve();
  assert.deepEqual(events, ['abort', 'stopped']);
  resolveRemote();
  await pending;
});

test('stopping before the server emits a message id still aborts local streaming', async () => {
  const events: string[] = [];
  await stopChatRun(undefined, () => { events.push('abort'); }, () => { events.push('stopped'); });
  assert.deepEqual(events, ['abort', 'stopped']);
});

test('stopping after an assistant id appears still invokes remote stop', async () => {
  let remoteStops = 0;
  await stopChatRun(
    async () => { remoteStops += 1; },
    () => undefined,
    () => undefined,
  );
  assert.equal(remoteStops, 1);
});
