import test from 'node:test';
import assert from 'node:assert/strict';
import { resolveUnknownStop } from './task-intent.ts';

test('resolveUnknownStop: canceled is the only proof a stop landed (cancellation is irreversible)', () => {
  assert.equal(resolveUnknownStop('canceled'), 'confirmed');
  for (const status of ['queued', 'running', 'waiting_user', 'reconciling', 'recovering', 'succeeded', 'failed', 'anything']) {
    assert.equal(resolveUnknownStop(status), 'not-landed', status);
  }
});
