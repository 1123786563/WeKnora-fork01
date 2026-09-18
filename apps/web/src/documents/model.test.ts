import assert from 'node:assert/strict';
import test from 'node:test';

import {
  buildProcessingTimeline,
  canDocumentAction,
  mergeChunkContents,
  summarizeUploadProgress,
  getDocumentStatus,
  normalizeDocumentPage,
  toggleDocumentSelection,
  validateUpload,
} from './model.ts';

test('maps Vue document processing states including finalizing summary work', () => {
  assert.deepEqual(getDocumentStatus({ id: '1', parse_status: 'processing' }), {
    key: 'processing', tone: 'primary', busy: true,
  });
  assert.deepEqual(getDocumentStatus({ id: '2', parse_status: 'finalizing', summary_status: 'processing' }), {
    key: 'generating-summary', tone: 'primary', busy: true,
  });
  assert.deepEqual(getDocumentStatus({ id: '3', parse_status: 'failed' }), {
    key: 'failed', tone: 'danger', busy: false,
  });
  assert.deepEqual(getDocumentStatus({ id: '4', parse_status: 'completed' }), {
    key: 'completed', tone: 'success', busy: false,
  });
});

test('normalizes the contract page without accepting malformed payloads', () => {
  assert.deepEqual(normalizeDocumentPage({
    success: true,
    data: [{ id: 'doc-1' }],
    total: 1,
    page: 2,
    page_size: 10,
  }), { items: [{ id: 'doc-1' }], total: 1, page: 2, pageSize: 10 });
  assert.throws(() => normalizeDocumentPage({ success: true, data: [{ title: 'missing id' }] }), /id/);
});

test('validates upload selection like the Vue upload flow', () => {
  assert.deepEqual(validateUpload([], { maxBytes: 100 }), { valid: false, errors: ['select-file'] });
  assert.deepEqual(validateUpload([{ name: 'a.exe', size: 4 }], { maxBytes: 100, acceptedExtensions: ['.pdf'] }), {
    valid: false, errors: ['unsupported-type'],
  });
  assert.deepEqual(validateUpload([{ name: 'a.pdf', size: 101 }], { maxBytes: 100, acceptedExtensions: ['.pdf'] }), {
    valid: false, errors: ['file-too-large'],
  });
  assert.deepEqual(validateUpload([{ name: 'a.pdf', size: 4 }], { maxBytes: 100, acceptedExtensions: ['.pdf'] }), {
    valid: true, errors: [],
  });
});

test('keeps selection deterministic and supports select-all and clear', () => {
  assert.deepEqual(toggleDocumentSelection(new Set(), 'a', true), new Set(['a']));
  assert.deepEqual(toggleDocumentSelection(new Set(['a', 'b']), 'a', false), new Set(['b']));
  assert.deepEqual(toggleDocumentSelection(new Set(['a']), 'all', true, ['a', 'b']), new Set(['a', 'b']));
  assert.deepEqual(toggleDocumentSelection(new Set(['a', 'b']), 'all', false, ['a', 'b']), new Set());
});

test('only permits mutations when Vue-equivalent edit permission exists', () => {
  assert.equal(canDocumentAction('preview', { canView: true, canEdit: false }), true);
  assert.equal(canDocumentAction('download', { canView: true, canEdit: false }, { type: 'file' }), true);
  assert.equal(canDocumentAction('download', { canView: true, canEdit: false }, { type: 'manual' }), true);
  assert.equal(canDocumentAction('download', { canView: true, canEdit: false }, { type: 'url' }), false);
  assert.equal(canDocumentAction('delete', { canView: true, canEdit: false }), false);
  assert.equal(canDocumentAction('delete', { canView: true, canEdit: true }), true);
  assert.equal(canDocumentAction('preview', { canView: false, canEdit: true }), false);
});

test('builds a processing timeline that distinguishes failed and completed enrichment', () => {
  assert.deepEqual(buildProcessingTimeline({ id: '1', parse_status: 'failed' }).map((step) => step.key), [
    'uploaded', 'parsing', 'failed',
  ]);
  assert.deepEqual(buildProcessingTimeline({ id: '2', parse_status: 'finalizing', summary_status: 'pending' }).map((step) => step.key), [
    'uploaded', 'parsing', 'enriching',
  ]);
  assert.deepEqual(buildProcessingTimeline({ id: '3', parse_status: 'completed' }).map((step) => step.key), [
    'uploaded', 'parsing', 'enriching', 'indexed',
  ]);
});

test('mergeChunkContents ports the Vue mergeChunks ordering and overlap trimming', () => {
  // Vue sorts by start_at (chunk_index fallback) and trims the overlap the
  // chunker re-emits at the next chunk start via suffix matching; overlaps
  // shorter than Vue's MIN_OVERLAP (12 chars) are never trimmed.
  assert.equal(mergeChunkContents([
    { content: 'BEGIN-OF-DOC OVERLAP-CONTENT-12', start_at: 0, end_at: 100 },
    { content: 'OVERLAP-CONTENT-12 TAIL-TEXT', start_at: 70, end_at: 200 },
  ]), 'BEGIN-OF-DOC OVERLAP-CONTENT-12 TAIL-TEXT');

  // A repadded table header before the overlap is skipped (headSlack), never duplicated.
  assert.equal(mergeChunkContents([
    { content: 'DATA-TABLE\nROW-ONE-VALUE', start_at: 0, end_at: 50 },
    { content: '| H |\nROW-ONE-VALUE', start_at: 40, end_at: 90 },
  ]), 'DATA-TABLE\nROW-ONE-VALUE');

  // A real position gap joins with a blank line exactly like the Vue merged view.
  assert.equal(mergeChunkContents([
    { content: 'A', start_at: 0, end_at: 100 },
    { content: 'B', start_at: 150, end_at: 200 },
  ]), 'A\n\nB');

  // Without position info Vue's appendChunkContent concatenates directly (overlap <= 0 branch).
  assert.equal(mergeChunkContents([
    { content: 'A', chunk_index: 1 },
    { content: 'B', chunk_index: 2 },
  ]), 'AB');
});

test('summarizes upload progress with completed count and failure state', () => {
  assert.deepEqual(summarizeUploadProgress([
    { uploadId: '1', fileName: 'a.pdf', progress: 40, status: 'uploading' },
    { uploadId: '2', fileName: 'b.pdf', progress: 100, status: 'success' },
    { uploadId: '3', fileName: 'c.pdf', progress: 20, status: 'error', error: 'network' },
  ]), { total: 3, completed: 2, progress: 53, hasError: true });
});

