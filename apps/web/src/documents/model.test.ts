import assert from 'node:assert/strict';
import test from 'node:test';

import {
  buildProcessingTimeline,
  canDocumentAction,
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
  assert.equal(canDocumentAction('download', { canView: true, canEdit: false }), true);
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

test('summarizes upload progress with completed count and failure state', () => {
  assert.deepEqual(summarizeUploadProgress([
    { uploadId: '1', fileName: 'a.pdf', progress: 40, status: 'uploading' },
    { uploadId: '2', fileName: 'b.pdf', progress: 100, status: 'success' },
    { uploadId: '3', fileName: 'c.pdf', progress: 20, status: 'error', error: 'network' },
  ]), { total: 3, completed: 2, progress: 53, hasError: true });
});
