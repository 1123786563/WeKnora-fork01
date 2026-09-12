import assert from 'node:assert/strict';
import test from 'node:test';

import { saveArtifactDownload } from './artifact-download.ts';

test('saves protected artifact bytes with the server-owned safe filename', async () => {
  let saved: { content: Blob | string; filename: string } | undefined;
  await saveArtifactDownload(
    { fileName: '报告.pdf' },
    { body: new Uint8Array([1, 2]).buffer, contentType: 'application/pdf' },
    async (content, filename) => { saved = { content, filename }; },
  );

  assert.equal(saved?.filename, '报告.pdf');
  assert.equal(saved?.content instanceof Blob, true);
  assert.equal((saved?.content as Blob).type, 'application/pdf');
  assert.equal((saved?.content as Blob).size, 2);
});

test('rejects an empty artifact filename before invoking the file sink', async () => {
  let called = false;
  await assert.rejects(
    () => saveArtifactDownload({ fileName: ' ' }, { body: 'x' }, async () => { called = true; }),
    /filename/,
  );
  assert.equal(called, false);
});

test('sanitizes path separators before handing a protected filename to the browser sink', async () => {
  let filename = '';
  await saveArtifactDownload(
    { fileName: '../report\\final.txt' },
    { body: 'x' },
    async (_content, value) => { filename = value; },
  );
  assert.equal(filename, '.._report_final.txt');
});
