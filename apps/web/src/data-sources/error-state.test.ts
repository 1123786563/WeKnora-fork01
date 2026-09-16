import assert from 'node:assert/strict';
import test from 'node:test';
import { classifyDataSourceError } from './error-state.ts';

test('classifies provider access failures into actionable states', () => {
  assert.equal(classifyDataSourceError(new Error('status=403 forbidden')).kind, 'forbidden');
  assert.equal(classifyDataSourceError({ code: 1061005, message: 'invalid auth' }).kind, 'auth');
  assert.equal(classifyDataSourceError({ error: 'folder not found' }).kind, 'not-found');
});

test('preserves a safe fallback for unknown resource failures', () => {
  assert.deepEqual(classifyDataSourceError(new Error('provider unavailable'), 'Unable to load resources'), {
    kind: 'error',
    message: 'provider unavailable',
  });
  assert.deepEqual(classifyDataSourceError({}, 'Unable to load resources'), {
    kind: 'error',
    message: 'Unable to load resources',
  });
});
