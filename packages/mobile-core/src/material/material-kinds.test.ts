import test from 'node:test';
import assert from 'node:assert/strict';
import { PREVIEW_MAX_BYTES, isInlineImageMime, materialKindOf, previewVerdictOf } from './material-kinds.ts';

test('material kinds derive from name and mime without guessing', () => {
  assert.equal(materialKindOf('changes.diff', 'text/x-diff'), 'diff');
  assert.equal(materialKindOf('feature.patch', 'text/plain'), 'diff');
  assert.equal(materialKindOf('app.patch', 'application/x-diff'), 'diff');
  assert.equal(materialKindOf('test-report.json', 'application/json'), 'test-report');
  assert.equal(materialKindOf('junit.xml', 'application/xml'), 'test-report');
  assert.equal(materialKindOf('TEST-RESULTS.txt', 'text/plain'), 'test-report');
  assert.equal(materialKindOf('report.md', 'text/markdown'), 'artifact');
  assert.equal(materialKindOf('data.csv', 'text/csv'), 'artifact');
  assert.equal(materialKindOf('monthly.tests.csv', 'text/csv'), 'artifact', '仅 test-report/junit/test-results 模式判定测试报告，普通文件名不误判');
});

test('preview verdicts gate inline previews by mime and size', () => {
  assert.deepEqual(previewVerdictOf('text/plain', 100), { state: 'supported' });
  assert.deepEqual(previewVerdictOf('text/markdown', PREVIEW_MAX_BYTES), { state: 'supported' }, '恰好在上限处仍可预览');
  assert.deepEqual(previewVerdictOf('text/plain', PREVIEW_MAX_BYTES + 1), { state: 'unsupported', reason: 'size' });
  assert.deepEqual(previewVerdictOf('application/pdf', 100), { state: 'unsupported', reason: 'mime' }, '首版无内联 PDF 渲染器：按不支持处理，走下载/分享路径');
  assert.deepEqual(previewVerdictOf('application/zip', 5), { state: 'unsupported', reason: 'mime' });
  assert.deepEqual(previewVerdictOf('image/png', 1000), { state: 'supported' });
  assert.equal(isInlineImageMime('image/png'), true);
  assert.equal(isInlineImageMime('image/svg+xml'), false, 'SVG 不做内联渲染（脚本面）');
});
