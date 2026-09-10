import assert from 'node:assert/strict';
import test from 'node:test';

import { isKnowledgeProcessingTerminal, normalizeKnowledgeProcessingStatus } from './processing.ts';

test('preserves every backend processing state', () => {
  for (const status of ['pending', 'processing', 'finalizing', 'completed', 'failed', 'deleting', 'cancelled']) {
    assert.equal(normalizeKnowledgeProcessingStatus(status), status);
  }
  assert.equal(isKnowledgeProcessingTerminal('completed'), true);
  assert.equal(isKnowledgeProcessingTerminal('processing'), false);
});

test('rejects unknown processing states instead of presenting them as completed', () => {
  assert.throws(() => normalizeKnowledgeProcessingStatus('indexed'), /Unknown knowledge processing status/);
});
