import assert from 'node:assert/strict';
import test from 'node:test';

import { resolveChatAttachmentValidationMessage } from './attachment-messages.ts';

test('attachment validation messages follow the active chat locale', () => {
  assert.equal(resolveChatAttachmentValidationMessage('en-US', 'too-many', 'guide.pdf', 50, 5), 'You can attach up to 5 files.');
  assert.equal(resolveChatAttachmentValidationMessage('ja-JP', 'too-large', 'guide.pdf', 50, 5), 'ファイルサイズは50MB以下にしてください。');
  assert.equal(resolveChatAttachmentValidationMessage('ko-KR', 'unsupported-type', 'guide.exe', 50, 5), '지원하지 않는 파일 형식입니다: guide.exe');
});
