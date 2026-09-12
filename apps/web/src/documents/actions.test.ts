import assert from 'node:assert/strict';
import test from 'node:test';

import { cancelParseDocuments, documentRowActions, reparseDocument } from './actions.ts';

test('cancel parse is only offered while a document is pending/processing/finalizing', () => {
  for (const active of ['pending', 'processing', 'finalizing']) {
    assert.deepEqual(documentRowActions(active), { canReparse: true, canCancelParse: true }, active);
  }
  for (const settled of ['completed', 'failed', 'cancelled', 'deleting']) {
    assert.deepEqual(documentRowActions(settled), { canReparse: true, canCancelParse: false }, settled);
  }
  assert.deepEqual(documentRowActions(undefined), { canReparse: false, canCancelParse: false });
  assert.deepEqual(documentRowActions('nonsense'), { canReparse: false, canCancelParse: false });
});

test('reparse delegates to the client reparse endpoint', async () => {
  const calls: string[] = [];
  await reparseDocument({ reparse: async (id) => void calls.push(id), cancelParse: async () => {} }, 'doc-1');
  assert.deepEqual(calls, ['doc-1']);
});

test('batch cancel only targets in-flight documents and skips settled ones', async () => {
  const cancelled: string[] = [];
  const api = { reparse: async () => {}, cancelParse: async (id: string) => void cancelled.push(id) };
  const documents = [
    { id: 'a', parse_status: 'processing' },
    { id: 'b', parse_status: 'completed' },
    { id: 'c', parse_status: 'pending' },
    { id: 'd', parse_status: 'failed' },
  ];
  assert.deepEqual(await cancelParseDocuments(api, documents), ['a', 'c']);
  assert.deepEqual(cancelled, ['a', 'c']);
});
