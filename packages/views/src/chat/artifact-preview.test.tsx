import assert from 'node:assert/strict';
import test from 'node:test';

import { artifactPreviewModel } from './artifact-preview.tsx';
import { resolveChatCopy } from './chat-copy.ts';

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
  assert.deepEqual(artifactPreviewModel({ fileName: 'diagram.svg', fileType: 'image/svg+xml; charset=utf-8' }), {
    kind: 'download-only',
    label: 'Download to view',
  });
});

test('artifact classification uses the active locale copy when provided', () => {
  const copy = resolveChatCopy('zh-CN');
  assert.deepEqual(artifactPreviewModel({ fileName: 'guide.md', fileType: 'text/markdown' }, copy), {
    kind: 'markdown',
    label: 'Markdown 预览',
  });
  assert.deepEqual(artifactPreviewModel({ fileName: 'chart.html', fileType: 'text/html' }, copy), {
    kind: 'download-only',
    label: '下载后查看',
  });
});
