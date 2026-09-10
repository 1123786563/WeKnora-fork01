import assert from 'node:assert/strict';
import test from 'node:test';
import { authHeaderForFileDownload, toNativeFileSource } from './file-uris.ts';

test('keeps picker URI in a platform file source and supplies safe defaults', () => {
  assert.deepEqual(toNativeFileSource({ uri: 'content://large', name: 'large.pdf', mimeType: 'application/pdf', size: 4_000_000 }), {
    uri: 'content://large', name: 'large.pdf', type: 'application/pdf', size: 4_000_000,
  });
  assert.deepEqual(toNativeFileSource({ uri: 'file:///tmp/a' }), { uri: 'file:///tmp/a', name: 'upload', type: 'application/octet-stream' });
  assert.equal(authHeaderForFileDownload('token'), 'Bearer token');
  assert.equal(authHeaderForFileDownload(undefined), undefined);
});
