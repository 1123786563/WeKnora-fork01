import assert from 'node:assert/strict';
import test from 'node:test';
import { classifyNativeArtifactPreview, nativeMarkdownLines } from './artifact-preview.ts';
import { nativeArtifactPreviewLabels } from './artifact-preview-labels.ts';

test('classifies mobile artifact preview without enabling executable document content', () => {
  assert.deepEqual(classifyNativeArtifactPreview({ fileName: 'guide.md', fileType: 'text/markdown' }), {
    kind: 'markdown',
    label: 'Markdown preview',
  });
  assert.deepEqual(classifyNativeArtifactPreview({ fileName: 'notes.txt', fileType: 'text/plain' }), {
    kind: 'text',
    label: 'Text preview',
  });
  assert.deepEqual(classifyNativeArtifactPreview({ fileName: 'photo.png', fileType: 'image/png' }), {
    kind: 'image',
    label: 'Image preview',
  });
  assert.deepEqual(classifyNativeArtifactPreview({ fileName: 'report.pdf', fileType: 'application/pdf' }), {
    kind: 'download-only',
    label: 'Download to view',
  });
  assert.deepEqual(classifyNativeArtifactPreview({ fileName: 'page.html', fileType: 'text/html' }), {
    kind: 'download-only',
    label: 'Download to view',
  });
});

test('native Markdown projection keeps source text and marks common readable blocks', () => {
  assert.deepEqual(nativeMarkdownLines('# 标题\n\n- 一项\n\n```ts\nconst value = 1;\n```'), [
    { kind: 'heading', text: '标题' },
    { kind: 'bullet', text: '一项' },
    { kind: 'code', text: 'const value = 1;' },
  ]);
  assert.deepEqual(nativeMarkdownLines('<script>alert(1)</script>'), [
    { kind: 'text', text: '<script>alert(1)</script>' },
  ]);
});

test('native artifact drawer labels resolve for every supported locale', async () => {
  for (const locale of ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const) {
    const labels = nativeArtifactPreviewLabels(locale);
    for (const value of Object.values(labels)) {
      assert.ok(value.trim());
      assert.notEqual(value, 'mobileChat.back');
    }
    assert.notEqual(labels.back, labels.share, locale);
    assert.notEqual(labels.loading, labels.downloadOnly, locale);
  }
});
