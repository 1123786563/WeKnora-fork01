import assert from 'node:assert/strict';
import test from 'node:test';

import { errorFromResult } from './errors.ts';

// S00 negpath3 T-2: the backend returns 403 bodies like
// {"error":"Access denied: URL workspace does not match the active workspace"}
// (error is a STRING, not the usual {code,message} record). Vue surfaces the
// backend message verbatim; React used to degrade to
// "Request failed with status 403", dropping the backend copy.
test('403 with string error body preserves the backend message (S00 T-2)', () => {
  const error = errorFromResult(403, {
    error: 'Access denied: URL workspace does not match the active workspace',
  });
  assert.equal(
    error.message,
    'Access denied: URL workspace does not match the active workspace',
  );
  assert.equal(error.status, 403);
});

test('403 with {code,message} record keeps the nested message', () => {
  const error = errorFromResult(404, { error: { code: 1003, message: 'knowledge base not found' } });
  assert.equal(error.message, 'knowledge base not found');
  assert.equal(error.code, '1003');
});

test('plain string body still becomes the message (kept verbatim)', () => {
  const error = errorFromResult(500, ' upstream exploded ');
  assert.equal(error.message, ' upstream exploded ');
});


test('career conflicts and unknown outcomes preserve backend reconciliation fields', () => {
  const conflict = errorFromResult(409, { error: { code: 'revision_conflict', message: 'stale', currentRevision: 7 } });
  assert.equal(conflict.currentRevision, 7);
  const unknown = errorFromResult(504, { error: { code: 'outcome_unknown', message: 'unknown', requestId: 'same-id' } });
  assert.equal(unknown.requestId, 'same-id');
});
