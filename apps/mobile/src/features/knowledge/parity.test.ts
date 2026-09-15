import assert from 'node:assert/strict';
import test from 'node:test';
import { isTerminalKnowledgeStatus, selectKnowledgeDocumentLabel, selectKnowledgeParity, shouldUseRecursiveFolderScope } from './parity.ts';

test('mobile selector keeps Web document count, order, and processing status', () => {
  const snapshot = selectKnowledgeParity({ success: true, total: 2, page: 1, page_size: 20, data: [
    { id: 'doc-1', file_name: 'guide.pdf', parse_status: 'processing' },
    { id: 'doc-2', title: 'FAQ', parse_status: 'completed' },
  ] });
  assert.deepEqual(snapshot, { total: 2, ids: ['doc-1', 'doc-2'], statuses: { 'doc-1': 'processing', 'doc-2': 'completed' } });
  assert.equal(selectKnowledgeDocumentLabel({ id: 'doc-2', title: 'FAQ' }), 'FAQ');
  assert.equal(isTerminalKnowledgeStatus('processing'), false);
  assert.equal(isTerminalKnowledgeStatus('completed'), true);
});

test('mobile document browsing matches Web folder scope semantics', () => {
  assert.equal(shouldUseRecursiveFolderScope({ folderPath: 'guides' }), false);
  assert.equal(shouldUseRecursiveFolderScope({ folderPath: 'guides', keyword: 'api' }), true);
  assert.equal(shouldUseRecursiveFolderScope({ folderPath: 'guides', tagIds: ['tag-1'] }), true);
});
