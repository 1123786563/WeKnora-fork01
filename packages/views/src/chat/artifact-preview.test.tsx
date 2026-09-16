import assert from 'node:assert/strict';
import test from 'node:test';

import {
  artifactPreviewModel,
  clampArtifactPreviewWidth,
  formatArtifactDateTime,
  formatArtifactSize,
  readArtifactPreviewText,
} from './artifact-preview.tsx';
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

test('artifact drawer width follows the Vue min/max and viewport clamp', () => {
  assert.equal(clampArtifactPreviewWidth(400, 1440), 520);
  assert.equal(clampArtifactPreviewWidth(760, 1440), 760);
  assert.equal(clampArtifactPreviewWidth(2000, 1440), 1368);
  assert.equal(clampArtifactPreviewWidth(760, 600), 570);
});

test('artifact drawer formats list metadata like the Vue drawer', () => {
  assert.equal(formatArtifactSize(0), '0 B');
  assert.equal(formatArtifactSize(2048), '2.0 KB');
  assert.equal(formatArtifactDateTime(''), '—');
  assert.equal(formatArtifactDateTime('not-a-date'), 'not-a-date');
  assert.match(formatArtifactDateTime('2026-09-08T04:05:00Z'), /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$/);
});

test('preview text failures remain observable to the preview error state', async () => {
  const failure = new Error('preview download failed');
  const body = new Blob(['unreadable']);
  Object.defineProperty(body, 'text', { value: async () => { throw failure; } });

  await assert.rejects(() => readArtifactPreviewText({ body }), (error: unknown) => error === failure);
});
