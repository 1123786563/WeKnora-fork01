import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';

import {
  MAX_BATCH_DOWNLOAD_BYTES,
  MAX_BATCH_DOWNLOAD_FILES,
  batchDownloadKnownBytes,
  batchDownloadPreflight,
  batchDownloadZipName,
  isBatchDownloadableKnowledge,
  saveBatchDownloadBlob,
  selectBatchDownloadIds,
} from './knowledge-batch-download.ts';

// Upstream handleBatchDownload contract: manual docs are always
// downloadable, everything else needs a stored original; the batch is
// capped at 200 files / 512 MiB of known bytes; nothing downloadable warns.

const ITEMS = new Map<string, { id: string; type?: string; file_path?: string; file_size?: number }>([
  ['doc-file', { id: 'doc-file', type: 'pdf', file_path: '/data/a.pdf', file_size: 1000 }],
  ['doc-manual', { id: 'doc-manual', type: 'manual' }],
  ['doc-web', { id: 'doc-web', type: 'webpage' }],
  ['doc-unknown-size', { id: 'doc-unknown-size', type: 'pdf', file_path: '/data/b.pdf', file_size: undefined }],
]);

test('isBatchDownloadableKnowledge mirrors the Vue filter', () => {
  assert.equal(isBatchDownloadableKnowledge(ITEMS.get('doc-file')), true);
  assert.equal(isBatchDownloadableKnowledge(ITEMS.get('doc-manual')), true);
  assert.equal(isBatchDownloadableKnowledge(ITEMS.get('doc-web')), false, 'web pages have no original file');
  assert.equal(isBatchDownloadableKnowledge(undefined), false);
  assert.equal(isBatchDownloadableKnowledge(null), false);
});

test('selectBatchDownloadIds keeps selection order and counts skipped entries', () => {
  const selection = selectBatchDownloadIds(['doc-web', 'doc-manual', 'missing', 'doc-file'], ITEMS);
  assert.deepEqual(selection.ids, ['doc-manual', 'doc-file']);
  assert.equal(selection.skipped, 2);
  assert.deepEqual(selectBatchDownloadIds([], ITEMS), { ids: [], skipped: 0 });
});

test('known byte total ignores missing and non-finite sizes', () => {
  assert.equal(batchDownloadKnownBytes(['doc-file', 'doc-manual', 'doc-unknown-size', 'missing'], ITEMS), 1000);
});

test('preflight warnings follow the Vue order and caps', () => {
  const none = selectBatchDownloadIds(['doc-web'], ITEMS);
  assert.equal(batchDownloadPreflight(none, 0), 'knowledgeBase.batchDownloadNoFiles');

  const many: string[] = [];
  for (let i = 0; i < MAX_BATCH_DOWNLOAD_FILES + 1; i += 1) many.push('doc-file');
  const tooMany = selectBatchDownloadIds(many, ITEMS);
  assert.equal(batchDownloadPreflight(tooMany, 0), 'knowledgeBase.batchDownloadHint');

  const tooLarge = selectBatchDownloadIds(['doc-file'], ITEMS);
  assert.equal(batchDownloadPreflight(tooLarge, MAX_BATCH_DOWNLOAD_BYTES + 1), 'knowledgeBase.batchDownloadTooLarge');

  const ok = selectBatchDownloadIds(['doc-file', 'doc-manual'], ITEMS);
  assert.equal(batchDownloadPreflight(ok, MAX_BATCH_DOWNLOAD_BYTES), null, 'exactly at the caps passes');
});

test('zip name matches the upstream knowledge-files-<ts>.zip pattern', () => {
  assert.equal(batchDownloadZipName(1789669000000), 'knowledge-files-1789669000000.zip');
});

test('saveBatchDownloadBlob clicks an object-URL anchor and revokes it later', () => {
  const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string) => { window: Window & typeof globalThis } };
  const dom = new JSDOM('<!doctype html><html><body></body></html>');
  const clicked: string[] = [];
  const revoked: string[] = [];
  const timeouts: Array<() => void> = [];
  const fakeDocument = {
    createElement: () => {
      const anchor = dom.window.document.createElement('a') as HTMLAnchorElement & { click(): void };
      anchor.click = () => clicked.push(anchor.download);
      return anchor;
    },
    body: dom.window.document.body,
  } as unknown as Document;
  const fakeWindow = {
    setTimeout: (fn: () => void) => { timeouts.push(fn); return 1; },
  } as unknown as Window;
  const originalCreate = URL.createObjectURL;
  const originalRevoke = URL.revokeObjectURL;
  URL.createObjectURL = (() => 'blob:batch-zip') as typeof URL.createObjectURL;
  URL.revokeObjectURL = ((url: string) => revoked.push(url)) as typeof URL.revokeObjectURL;
  try {
    saveBatchDownloadBlob(new dom.window.Blob(['zip']), 'knowledge-files-1.zip', fakeDocument, fakeWindow);
    assert.deepEqual(clicked, ['knowledge-files-1.zip']);
    assert.deepEqual(revoked, [], 'revocation waits for the timeout');
    timeouts.forEach((fn) => fn());
    assert.deepEqual(revoked, ['blob:batch-zip']);
  } finally {
    URL.createObjectURL = originalCreate;
    URL.revokeObjectURL = originalRevoke;
  }
});
