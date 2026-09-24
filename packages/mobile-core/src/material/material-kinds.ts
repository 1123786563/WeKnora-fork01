import type { MaterialEntryKind, PreviewVerdict } from './types.ts';

/** 内联预览的移动端大小上限：超过即判 size 不支持，走下载/分享路径（spec User Story 32）。 */
export const PREVIEW_MAX_BYTES = 2 * 1024 * 1024;

const DIFF_NAME = /\.(diff|patch)$/i;
const DIFF_MIME = new Set(['text/x-diff', 'text/x-patch', 'application/x-diff', 'application/x-patch']);
/** 测试报告仅按明确的文件名模式判定；普通名字不猜测。 */
const REPORT_NAME = /(^|[^a-z])(test[-_]?reports?|test[-_]?results?|junit)([^a-z]|$)/i;
const INLINE_TEXT_MIME = new Set([
  'application/json', 'application/x-ndjson', 'text/csv', 'text/markdown', 'text/x-markdown',
  'text/x-diff', 'text/x-patch',
]);
const INLINE_IMAGE_MIME = new Set(['image/png', 'image/jpeg', 'image/gif', 'image/webp']);

export function materialKindOf(name: string, mime: string): MaterialEntryKind {
  const trimmedName = typeof name === 'string' ? name.trim() : '';
  const normalizedMime = typeof mime === 'string' ? mime.trim().toLowerCase() : '';
  if (DIFF_NAME.test(trimmedName) || DIFF_MIME.has(normalizedMime)) return 'diff';
  if (REPORT_NAME.test(trimmedName)) return 'test-report';
  return 'artifact';
}

export function previewVerdictOf(mime: string, size: number): PreviewVerdict {
  const normalizedMime = typeof mime === 'string' ? mime.trim().toLowerCase() : '';
  const previewable = normalizedMime.startsWith('text/') || INLINE_TEXT_MIME.has(normalizedMime) || INLINE_IMAGE_MIME.has(normalizedMime);
  if (!previewable) return { state: 'unsupported', reason: 'mime' };
  if (typeof size !== 'number' || !Number.isFinite(size) || size < 0 || size > PREVIEW_MAX_BYTES) {
    return { state: 'unsupported', reason: 'size' };
  }
  return { state: 'supported' };
}

export function isInlineImageMime(mime: string): boolean {
  return INLINE_IMAGE_MIME.has(mime.trim().toLowerCase());
}
