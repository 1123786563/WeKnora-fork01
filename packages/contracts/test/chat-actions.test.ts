import assert from 'node:assert/strict';
import test from 'node:test';

import * as contracts from '../src/index.ts';

test('requires success true for approval and OAuth mutation responses', () => {
  const parseActionSuccessResponse = (contracts as Record<string, unknown>).parseActionSuccessResponse;
  assert.equal(typeof parseActionSuccessResponse, 'function');
  assert.deepEqual((parseActionSuccessResponse as (value: unknown) => unknown)({ success: true }), { success: true });
  assert.throws(() => (parseActionSuccessResponse as (value: unknown) => unknown)({ success: false }));
});

test('parses every steer mutation status without dropping correlation fields', () => {
  const parseSteerMutationResponse = (contracts as Record<string, unknown>).parseSteerMutationResponse;
  assert.equal(typeof parseSteerMutationResponse, 'function');
  const parse = parseSteerMutationResponse as (value: unknown) => unknown;

  assert.deepEqual(parse({
    success: true,
    status: 'queued',
    steer_id: 'steer-1',
    assistant_message_id: 'assistant-1',
    delivery: 'after',
  }), {
    success: true,
    status: 'queued',
    steer_id: 'steer-1',
    assistant_message_id: 'assistant-1',
    delivery: 'after',
  });
  assert.deepEqual(parse({ success: true, status: 'new_run' }), { success: true, status: 'new_run' });
  assert.deepEqual(
    parse({ success: true, status: 'already_injected', steer_id: 'steer-1' }),
    { success: true, status: 'already_injected', steer_id: 'steer-1' },
  );
  assert.throws(() => parse({ success: true, status: 'queued', steer_id: 'steer-1', delivery: 'after' }));
  assert.throws(() => parse({ success: true, status: 'unknown' }));
});

test('parses the live steer queue and delete outcomes strictly', () => {
  const parseSteerListResponse = (contracts as Record<string, unknown>).parseSteerListResponse as (value: unknown) => unknown;
  const parseSteerDeleteResponse = (contracts as Record<string, unknown>).parseSteerDeleteResponse as (value: unknown) => unknown;
  assert.equal(typeof parseSteerListResponse, 'function');
  assert.equal(typeof parseSteerDeleteResponse, 'function');

  assert.deepEqual(parseSteerListResponse({
    success: true,
    assistant_message_id: 'assistant-1',
    items: [{ steer_id: 'steer-1', content: 'follow up', delivery: 'inject', mentioned_items: [{ id: 'kb-1' }] }],
  }), {
    success: true,
    assistant_message_id: 'assistant-1',
    items: [{ steer_id: 'steer-1', content: 'follow up', delivery: 'inject', mentioned_items: [{ id: 'kb-1' }] }],
  });
  assert.deepEqual(
    parseSteerDeleteResponse({ success: true, status: 'deleted', removed: true, steer_id: 'steer-1' }),
    { success: true, status: 'deleted', removed: true, steer_id: 'steer-1' },
  );
  assert.deepEqual(parseSteerDeleteResponse({ success: true, status: 'gone' }), { success: true, status: 'gone' });
  assert.throws(() => parseSteerListResponse({ success: true, items: [{ steer_id: '', content: 'x', delivery: 'after' }] }));
  assert.throws(() => parseSteerDeleteResponse({ success: true, status: 'deleted', removed: 'yes', steer_id: 'steer-1' }));
});
