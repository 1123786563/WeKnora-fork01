import assert from 'node:assert/strict';
import test from 'node:test';

import { ContractError, parseApiErrorPayload, parseKnowledgeBaseListResponse } from '../src/index.ts';

test('parses the common API error payload', () => {
  assert.deepEqual(
    parseApiErrorPayload({ code: 'forbidden', message: 'No access', request_id: 'req-1', details: { role: 'viewer' } }),
    { code: 'forbidden', message: 'No access', requestId: 'req-1', details: { role: 'viewer' } },
  );
});

test('parses the knowledge-base list envelope without trusting Swagger types', () => {
  const result = parseKnowledgeBaseListResponse({
    success: true,
    data: [{ id: 'kb-1', name: 'Docs', type: 'document', tenant_id: '9007199254740993' }],
  });

  assert.equal(result[0]?.id, 'kb-1');
  assert.equal(result[0]?.tenant_id, '9007199254740993');
});

test('rejects a malformed knowledge-base list response', () => {
  assert.throws(
    () => parseKnowledgeBaseListResponse({ success: true, data: [{ name: 'missing id' }] }),
    (error: unknown) => error instanceof ContractError && error.path === 'data[0].id',
  );
});
