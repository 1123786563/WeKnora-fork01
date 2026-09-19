import assert from 'node:assert/strict';
import test from 'node:test';
import { ContractError } from './index.ts';
import { parseMyUsageResponse, parseUsageByUserResponse } from './usage.ts';

const myRow = {
  window_start: '2026-09-19',
  model: 'glm-4.7',
  input_tokens: 1200,
  output_tokens: 340,
  cache_read_tokens: 0,
  cache_write_tokens: 80,
  cost_microcredits: 5000,
};

const byUserRow = { ...myRow, user_id: 'user-1' };

test('parseMyUsageResponse unwraps envelope and validates fields', () => {
  const result = parseMyUsageResponse({ success: true, data: [myRow] });
  assert.deepEqual(result.items, [myRow]);
});

test('parseMyUsageResponse passes extra fields through', () => {
  const result = parseMyUsageResponse({ success: true, data: [{ ...myRow, flow: 'chat' }] });
  assert.equal(result.items[0]!.flow, 'chat');
});

test('parseUsageByUserResponse keeps the required user_id column', () => {
  const result = parseUsageByUserResponse({ success: true, data: [byUserRow] });
  assert.deepEqual(result.items, [byUserRow]);
});

test('parseUsageByUserResponse rejects a row without user_id', () => {
  assert.throws(
    () => parseUsageByUserResponse({ success: true, data: [myRow] }),
    (error: unknown) => error instanceof ContractError && error.path === 'data[0].user_id',
  );
});

test('parseUsageByUserResponse rejects an empty user_id', () => {
  assert.throws(() => parseUsageByUserResponse({ success: true, data: [{ ...byUserRow, user_id: '' }] }), ContractError);
});

test('parse rejects non-envelope payload', () => {
  assert.throws(() => parseMyUsageResponse({ data: [] }), ContractError);
});

test('parse rejects a mistyped token count', () => {
  assert.throws(
    () => parseMyUsageResponse({ success: true, data: [{ ...myRow, input_tokens: '1200' }] }),
    (error: unknown) => error instanceof ContractError && error.path === 'data[0].input_tokens',
  );
});

test('parse rejects a negative cost', () => {
  assert.throws(() => parseMyUsageResponse({ success: true, data: [{ ...myRow, cost_microcredits: -1 }] }), ContractError);
});
