import assert from 'node:assert/strict';
import test from 'node:test';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import * as XLSX from 'xlsx';

import { DocumentPreviewContent, buildDocumentPreview, canPreviewDocument, isInlinePreviewKind, readCurrentPreviewText, readPreviewText, readSpreadsheetPreview } from './preview.ts';

test('builds an authenticated preview model without treating download URLs as public', () => {
  assert.deepEqual(buildDocumentPreview({ id: 'doc/a', type: 'file', file_name: 'guide.pdf', parse_status: 'completed' }, '/api/v1/knowledge/doc%2Fa/preview'), {
    kind: 'pdf',
    ready: true,
    path: '/api/v1/knowledge/doc%2Fa/preview',
    fileName: 'guide.pdf',
  });
});

test('preview readiness ignores parse_status exactly like the Vue canPreview gate', () => {
  assert.equal(buildDocumentPreview({ id: 'doc-1', type: 'file', file_name: 'guide.pdf', parse_status: 'pending' }, '/preview').ready, true, 'Vue never consults parse_status before loading preview content');
  assert.equal(buildDocumentPreview({ id: 'doc-1', type: 'file', file_name: 'guide.pdf', parse_status: 'finalizing' }, '/preview').ready, true);
  assert.equal(buildDocumentPreview({ id: 'doc-1', type: 'file', file_name: 'guide.pdf', parse_status: 'failed' }, '/preview').ready, true);
  assert.equal(buildDocumentPreview({ id: 'doc-1', type: 'file', file_name: 'Interview.mp3', parse_status: 'pending' }, '/preview').ready, true, 'audio stays fetchable for the embedded player regardless of processing state');
  assert.equal(buildDocumentPreview({ id: 'doc-1', file_name: 'guide.pdf', type: 'manual' }, '/preview').ready, false, 'Vue gates on details.type === "file"');
  assert.equal(buildDocumentPreview({ id: 'doc-1', type: 'file', file_name: 'brief.docx', parse_status: 'completed' }, '/preview').ready, false, 'unresolvable inline extensions never fetch');
  assert.equal(buildDocumentPreview({ id: 'doc-1', type: 'file', file_name: 'guide.' }, '/preview').ready, false);
});

test('marks spreadsheet documents as inline previewable like the Vue document preview', () => {
  assert.equal(isInlinePreviewKind('text'), true);
  assert.equal(isInlinePreviewKind('markdown'), true);
  assert.equal(isInlinePreviewKind('image'), true);
  assert.equal(isInlinePreviewKind('pdf'), true);
  assert.equal(isInlinePreviewKind('audio'), true);
  assert.equal(isInlinePreviewKind('video'), true);
  assert.equal(isInlinePreviewKind('spreadsheet'), true);
  assert.equal(isInlinePreviewKind('mermaid'), true);
  assert.equal(isInlinePreviewKind('docx'), false);
  assert.equal(isInlinePreviewKind('pptx'), false);
  assert.equal(isInlinePreviewKind('unsupported'), false);
});

test('canPreviewDocument mirrors the Vue canPreview gate: file type, resolvable extension, never audio', () => {
  assert.equal(canPreviewDocument({ type: 'file', file_name: 'Guide.pdf', source: '' }), true, 'real backend payloads carry source:"" for file knowledge — the gate keys off type');
  assert.equal(canPreviewDocument({ type: 'file', file_name: 'Guide.pdf' }), true, 'absent source must not matter; Vue checks type');
  assert.equal(canPreviewDocument({ type: 'file', file_name: 'architecture.mmd' }), true);
  assert.equal(canPreviewDocument({ type: 'file', file_type: 'pdf' }), true, 'Vue resolveFilePreviewExt falls back to file_type when the title has no extension');
  assert.equal(canPreviewDocument({ type: 'file', file_name: 'Interview.mp3' }), false, 'Vue keeps audio out of the preview tab; the player is embedded instead');
  assert.equal(canPreviewDocument({ type: 'url', file_name: 'Guide.pdf' }), false);
  assert.equal(canPreviewDocument({ type: 'manual', file_name: 'Guide.pdf' }), false);
  assert.equal(canPreviewDocument({ file_name: 'Guide.pdf', source: '' }), false, 'missing type never previews, matching Vue details?.type !== "file"');
  assert.equal(canPreviewDocument({ type: '', file_name: 'Guide.pdf' }), false, 'empty-string type is not file');
  assert.equal(canPreviewDocument({ type: 'file', file_name: 'notes.txt', file_type: 'pdf' }), true, 'Vue prefers an explicit file_type over the filename suffix');
  assert.equal(canPreviewDocument({ type: 'file', file_name: 'guide.pdf', file_type: 'txt' }), true, 'an explicit file_type wins even when the filename suffix differs — txt is text-previewable');
  assert.equal(canPreviewDocument({ type: 'file', file_name: 'clip.mp4', file_type: 'exe' }), false, 'an explicit unpreviewable file_type wins over the filename suffix');
  assert.equal(canPreviewDocument({ type: 'file', file_name: 'Guide.' }), false, 'trailing dot resolves to no extension');
  assert.equal(canPreviewDocument({ type: 'file' }), false);
});

test('recognizes Mermaid files as inline previews like the Vue document preview', () => {
  const preview = buildDocumentPreview({ id: 'doc-1', type: 'file', file_name: 'architecture.mmd', parse_status: 'completed' }, '/preview');

  assert.equal(preview.kind, 'mermaid');
  assert.equal(preview.ready, true);
});

test('reads every Vue-supported spreadsheet sheet into safe table rows', async () => {
  const workbook = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(workbook, XLSX.utils.aoa_to_sheet([
    ['Name', 'Count'],
    ['Alpha', 2],
  ]), 'Summary');
  XLSX.utils.book_append_sheet(workbook, XLSX.utils.aoa_to_sheet([
    ['Status'],
    ['<ready>'],
  ]), 'Details');
  const bytes = XLSX.write(workbook, { bookType: 'xlsx', type: 'array' });

  assert.deepEqual(await readSpreadsheetPreview(bytes as ArrayBuffer, 'report.xlsx'), {
    sheets: [
      { name: 'Summary', rows: [['Name', 'Count'], ['Alpha', '2']] },
      { name: 'Details', rows: [['Status'], ['<ready>']] },
    ],
  });
});

test('renders markdown as escaped text instead of interpreting HTML', async () => {
  const text = await readPreviewText(new Blob(['# Guide\n<script>alert(1)</script>'], { type: 'text/markdown' }));
  const markup = renderToStaticMarkup(createElement(DocumentPreviewContent, { kind: 'markdown', text }));

  assert.match(markup, /&lt;script&gt;alert\(1\)&lt;\/script&gt;/);
  assert.doesNotMatch(markup, /<script>/);
});

test('drops delayed text from a closed preview after a new preview opens', async () => {
  let firstIsActive = true;
  const delayedFirstText = readCurrentPreviewText('first document', () => firstIsActive);

  firstIsActive = false;
  const secondText = await readCurrentPreviewText('second document', () => true);

  assert.equal(await delayedFirstText, undefined);
  assert.equal(secondText, 'second document');
});
