import assert from 'node:assert/strict';
import test from 'node:test';

import { flattenKnowledgeFolders } from './folders.ts';

test('flattens nested folder trees while preserving canonical paths and counts', () => {
  assert.deepEqual(flattenKnowledgeFolders({
    root_document_count: 2,
    total_document_count: 4,
    folders: [{
      path: 'docs', name: 'docs', document_count: 1, total_count: 2,
      children: [{ path: 'docs/spec', name: 'spec', document_count: 1, total_count: 1 }],
    }],
  }), [
    { path: '', name: 'Root', document_count: 2, total_count: 4, depth: 0 },
    { path: 'docs', name: 'docs', document_count: 1, total_count: 2, depth: 0 },
    { path: 'docs/spec', name: 'spec', document_count: 1, total_count: 1, depth: 1 },
  ]);
});
