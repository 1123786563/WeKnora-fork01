import test from 'node:test';
import assert from 'node:assert/strict';
import { dispatchOnce, type CommandLog } from './command-log.ts';

test('unknown previous dispatch is not started again', async () => {
  let calls = 0;
  const log: CommandLog = { begin: async () => 'unknown', complete: async () => {} };
  await assert.rejects(
    dispatchOnce(log, 'c', 'h', async () => {
      calls += 1;
      return 'external';
    }),
    /DISPATCH_UNKNOWN/,
  );
  assert.equal(calls, 0);
});

test('completed dispatch is returned without starting again', async () => {
  let calls = 0;
  const log: CommandLog = { begin: async () => ({ externalID: 'ext-1' }), complete: async () => {} };
  assert.equal(await dispatchOnce(log, 'c', 'h', async () => { calls += 1; return 'new'; }), 'ext-1');
  assert.equal(calls, 0);
});

test('new dispatch completes its durable intent after the SDK returns', async () => {
  const calls: string[] = [];
  const log: CommandLog = {
    begin: async (id, hash) => { calls.push(`begin:${id}:${hash}`); return 'new'; },
    complete: async (id, externalID) => { calls.push(`complete:${id}:${externalID}`); },
  };
  assert.equal(await dispatchOnce(log, 'c', 'h', async () => { calls.push('start'); return 'ext'; }), 'ext');
  assert.deepEqual(calls, ['begin:c:h', 'start', 'complete:c:ext']);
});

test('start failure leaves the intent for recovery and is not converted to a new dispatch', async () => {
  const log: CommandLog = { begin: async () => 'new', complete: async () => assert.fail('must not complete') };
  await assert.rejects(dispatchOnce(log, 'c', 'h', async () => { throw new Error('transport lost'); }), /transport lost/);
});

test('hash conflict is surfaced by the durable command log', async () => {
  const log: CommandLog = {
    begin: async () => { throw new Error('COMMAND_HASH_CONFLICT'); },
    complete: async () => {},
  };
  await assert.rejects(dispatchOnce(log, 'c', 'different', async () => 'ext'), /COMMAND_HASH_CONFLICT/);
});
