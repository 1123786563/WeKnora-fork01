import test from 'node:test';
import assert from 'node:assert/strict';
import {
  parseExecution,
  parseExecutionEvent,
  parseExecutionSnapshot,
} from '../src/mobile/execution.ts';

const event = {
  schema_version: 1,
  run_id: 'r',
  attempt_id: 'a',
  seq: 1,
  type: 'future.event',
  occurred_at: '2026-09-12T00:00:00Z',
  payload: {},
};

test('reject malformed sequence and retain unknown event type', () => {
  assert.equal(parseExecutionEvent(event).type, 'future.event');
  for (const seq of [-1, 1.5, Number.MAX_SAFE_INTEGER + 1, '1']) {
    assert.throws(() => parseExecutionEvent({ ...event, seq }), /INVALID_EVENT/);
  }
});

test('validate execution DTO capabilities and snapshot watermark', () => {
  const execution = parseExecution({
    schema_version: 1,
    run_id: 'r',
    session_id: 's',
    revision: 0,
    driver: 'platform',
    run_status: 'queued',
    execution_status: 'idle',
    settlement_status: 'unsettled',
    seq: 0,
    capabilities: {
      text: { state: 'supported', reason: '' },
      voice: { state: 'unavailable', reason: 'not configured' },
      admin: { state: 'forbidden', reason: 'policy' },
    },
  });
  assert.equal(execution.capabilities.text?.state, 'supported');
  assert.throws(() => parseExecution({ ...execution, schema_version: 2 }), /SCHEMA_VERSION/);
  assert.throws(() => parseExecution({ ...execution, capabilities: { voice: { state: 'unavailable', reason: '' } } }), /reason/);
  assert.throws(() => parseExecutionEvent({ ...event, occurred_at: '12/09/2026' }), /occurred_at/);
  assert.throws(() => parseExecutionEvent({ ...event, occurred_at: '2026-02-30T00:00:00Z' }), /occurred_at/);
  assert.throws(() => parseExecutionEvent({ ...event, occurred_at: '2026-09-12T00:00:00+24:00' }), /occurred_at/);
  assert.equal(parseExecutionEvent({ ...event, occurred_at: '2026-09-12T00:00:00.123+08:00' }).occurred_at, '2026-09-12T00:00:00.123+08:00');
  for (const year of ['0000', '0099', '0100']) {
    assert.equal(parseExecutionEvent({ ...event, occurred_at: `${year}-01-02T03:04:05Z` }).occurred_at, `${year}-01-02T03:04:05Z`);
  }
  assert.throws(() => parseExecutionEvent({ ...event, payload: [] }), /payload/);
  assert.throws(() => parseExecutionEvent({ ...event, run_id: undefined }), /run_id/);
  assert.throws(() => parseExecutionEvent({ ...event, attempt_id: undefined }), /attempt_id/);
  assert.deepEqual(parseExecutionSnapshot({ execution, watermark: 1, incomplete: false, confirmed_watermark: 0, events: [event] }).watermark, 1);
  assert.equal(parseExecutionSnapshot({ execution, watermark: 1, incomplete: false, confirmed_watermark: 0, events: [event] }).confirmedWatermark, 0);
  assert.throws(() => parseExecutionSnapshot({ execution, watermark: 1, events: [event] } as unknown as Record<string, unknown>), /incomplete/);
});
