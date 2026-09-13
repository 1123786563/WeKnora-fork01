import assert from 'node:assert/strict';
import test from 'node:test';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import { DocumentPreviewContent, buildDocumentPreview, isInlinePreviewKind, readPreviewText } from './preview.ts';

test('builds an authenticated preview model without treating download URLs as public', () => {
  assert.deepEqual(buildDocumentPreview({ id: 'doc/a', file_name: 'guide.pdf', parse_status: 'completed' }, '/api/v1/knowledge/doc%2Fa/preview'), {
    kind: 'pdf',
    ready: true,
    downloadOnly: false,
    path: '/api/v1/knowledge/doc%2Fa/preview',
    fileName: 'guide.pdf',
  });
});

test('blocks preview while the latest processing attempt is not completed', () => {
  assert.equal(buildDocumentPreview({ id: 'doc-1', file_name: 'guide.pdf', parse_status: 'finalizing' }, '/preview').ready, false);
});

test('only marks safe text, markdown, image, and PDF kinds as inline previewable', () => {
  assert.equal(isInlinePreviewKind('text'), true);
  assert.equal(isInlinePreviewKind('markdown'), true);
  assert.equal(isInlinePreviewKind('image'), true);
  assert.equal(isInlinePreviewKind('pdf'), true);
  assert.equal(isInlinePreviewKind('audio'), true);
  assert.equal(isInlinePreviewKind('video'), true);
  assert.equal(isInlinePreviewKind('docx'), false);
  assert.equal(buildDocumentPreview({ id: 'doc-1', file_name: 'brief.docx', parse_status: 'completed' }, '/preview').downloadOnly, true);
});

test('renders markdown as escaped text instead of interpreting HTML', async () => {
  const text = await readPreviewText(new Blob(['# Guide\n<script>alert(1)</script>'], { type: 'text/markdown' }));
  const markup = renderToStaticMarkup(createElement(DocumentPreviewContent, { kind: 'markdown', text }));

  assert.match(markup, /&lt;script&gt;alert\(1\)&lt;\/script&gt;/);
  assert.doesNotMatch(markup, /<script>/);
});
