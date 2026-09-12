import assert from 'node:assert/strict';
import test from 'node:test';

import { artifactPreviewModel } from './artifact-preview.tsx';

test('classifies protected artifact types without treating HTML as executable preview content', () => {
  assert.deepEqual(artifactPreviewModel({ fileName: 'guide.md', fileType: 'text/markdown' }), {
    kind: 'markdown',
    label: 'Markdown preview',
  });
  assert.deepEqual(artifactPreviewModel({ fileName: 'photo.png', fileType: 'image/png' }), {
    kind: 'image',
    label: 'Image preview',
  });
  assert.deepEqual(artifactPreviewModel({ fileName: 'report.pdf', fileType: 'application/pdf' }), {
    kind: 'pdf',
    label: 'PDF preview',
  });
  assert.deepEqual(artifactPreviewModel({ fileName: 'chart.html', fileType: 'text/html' }), {
    kind: 'download-only',
    label: 'Download to view',
  });
});

test('unknown artifact types remain explicitly download-only', () => {
  assert.deepEqual(artifactPreviewModel({ fileName: 'slides.pptx', fileType: 'application/vnd.openxmlformats-officedocument.presentationml.presentation' }), {
    kind: 'download-only',
    label: 'Download to view',
  });
});
