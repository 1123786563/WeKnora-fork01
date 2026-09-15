import assert from 'node:assert/strict';
import test from 'node:test';

import { cancelParseDocuments, documentRowActions, filterReparseIds, reparseDocument } from './actions.ts';

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

test('reparse forwards the selected processing overrides to the client endpoint', async () => {
  const calls: Array<{ id: string; processConfig: unknown }> = [];
  const processConfig = { chunk_size: 640, enable_summary: true };
  await reparseDocument({
    reparse: async (id, receivedProcessConfig) => void calls.push({ id, processConfig: receivedProcessConfig }),
    cancelParse: async () => {},
  }, 'doc-1', processConfig);
  assert.deepEqual(calls, [{ id: 'doc-1', processConfig }]);
});

test('batch reparse excludes in-flight and duplicate document IDs', () => {
  assert.deepEqual(
    filterReparseIds(['done', 'done', 'working', 'missing', 'missing'], [
      { id: 'done', parse_status: 'completed' },
      { id: 'working', parse_status: 'processing' },
    ]),
    ['done', 'missing'],
  );
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

test('batch cancel submits each eligible document once and continues after an error', async () => {
  const cancelled: string[] = [];
  const expected = new Error('cancel a failed');
  const api = {
    reparse: async () => {},
    cancelParse: async (id: string) => {
      cancelled.push(id);
      if (id === 'a') throw expected;
    },
  };

  await assert.rejects(
    cancelParseDocuments(api, [
      { id: 'a', parse_status: 'processing' },
      { id: 'a', parse_status: 'processing' },
      { id: 'settled', parse_status: 'completed' },
      { id: 'b', parse_status: 'finalizing' },
    ]),
    expected,
  );
  assert.deepEqual(cancelled, ['a', 'b']);
});
