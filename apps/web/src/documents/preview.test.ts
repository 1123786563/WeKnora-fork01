import assert from 'node:assert/strict';
import test from 'node:test';

import { buildDocumentPreview } from './preview.ts';

test('builds an authenticated preview model without treating download URLs as public', () => {
  assert.deepEqual(buildDocumentPreview({ id: 'doc/a', file_name: 'guide.pdf', parse_status: 'completed' }, '/api/v1/knowledge/doc%2Fa/preview'), {
    kind: 'pdf',
    ready: true,
    path: '/api/v1/knowledge/doc%2Fa/preview',
    fileName: 'guide.pdf',
  });
});

test('blocks preview while the latest processing attempt is not completed', () => {
  assert.equal(buildDocumentPreview({ id: 'doc-1', file_name: 'guide.pdf', parse_status: 'finalizing' }, '/preview').ready, false);
});
