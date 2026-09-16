import assert from 'node:assert/strict';
import test from 'node:test';

import { previewKindForFile, previewStatus } from './preview.ts';

test('maps the supported document inventory to explicit preview kinds', () => {
  assert.equal(previewKindForFile('guide.PDF'), 'pdf');
  assert.equal(previewKindForFile('photo.png'), 'image');
  assert.equal(previewKindForFile('notes.md'), 'markdown');
  assert.equal(previewKindForFile('table.csv'), 'spreadsheet');
  assert.equal(previewKindForFile('brief.docx'), 'docx');
  assert.equal(previewKindForFile('slides.pptx'), 'pptx');
  assert.equal(previewKindForFile('voice.m4a'), 'audio');
  assert.equal(previewKindForFile('demo.webm'), 'video');
  assert.equal(previewKindForFile('archive.bin'), 'unsupported');
});

test('does not present non-terminal processing as ready to preview', () => {
  assert.deepEqual(previewStatus({ parse_status: 'finalizing' }), { kind: 'processing', label: 'Finalizing' });
  assert.deepEqual(previewStatus({ parse_status: 'completed' }), { kind: 'ready', label: 'Ready' });
  assert.deepEqual(previewStatus({ parse_status: 'failed' }), { kind: 'unavailable', label: 'Failed' });
  assert.deepEqual(previewStatus({ parse_status: 'indexed' as never }), { kind: 'unavailable', label: 'Unknown status' });
});
