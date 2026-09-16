import test from 'node:test';
import assert from 'node:assert/strict';
import { commitEvent, ExecutionCache } from './execution-cache.ts';
import type { ExecutionEvent } from '@weknora/contracts';

const event = (seq: number, runID = 'r'): ExecutionEvent => ({ schema_version: 1, run_id: runID, attempt_id: 'a', seq, type: 'text.delta', occurred_at: '2026-09-12T00:00:00Z', payload: { text: String(seq) } });

test('projection failure cannot advance cursor', async () => {
  let cursor = 0;
  await assert.rejects(commitEvent(event(1), async () => { throw new Error('disk_full'); }, async (seq) => { cursor = seq; }), /disk_full/);
  assert.equal(cursor, 0);
});

test('cache deduplicates replay and rejects gaps', async () => {
  const cache = new ExecutionCache({ origin: 'https://a', tenantID: 't', userID: 'u' });
  await cache.commit(event(1));
  await cache.commit(event(1));
  await assert.rejects(cache.commit(event(3)), /cursor gap/);
  assert.deepEqual(cache.read('r')?.events.map((item) => item.seq), [1]);
});

test('scope and run keys cannot leak projections', async () => {
  const a = new ExecutionCache({ origin: 'https://a', tenantID: 't1', userID: 'u1' });
  const b = new ExecutionCache({ origin: 'https://a', tenantID: 't2', userID: 'u1' });
  await a.commit(event(1));
  assert.equal(b.read('r'), undefined);
  assert.equal(a.read('other'), undefined);
});
