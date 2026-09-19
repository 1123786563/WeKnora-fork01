// SP13 Task 8 — 会话分享纯函数测试（node:test）：分享链接拼装、剪贴板
// 写入（writeClipboardText 先例，注入 clipboard 断言）、无效链接 404 判定。
import assert from 'node:assert/strict';
import test from 'node:test';

import { ApiError } from '@weknora/api-client';

import {
  buildShareLink,
  isShareLinkInvalidError,
  normalizeShareToken,
  SHARED_SESSION_PATH_PREFIX,
  writeShareLinkClipboard,
} from './session-share.ts';

test('buildShareLink 拼出 platform 只读页 URL 并编码 token', () => {
  assert.equal(buildShareLink('https://weknora.example.com', 'tok_abc123'), 'https://weknora.example.com/platform/shared/tok_abc123');
  // origin 结尾斜杠归一；token 两侧空白剥离后仍需 URL 编码。
  assert.equal(buildShareLink('https://weknora.example.com/', ' tok/with+spec '), 'https://weknora.example.com/platform/shared/tok%2Fwith%2Bspec');
  assert.equal(SHARED_SESSION_PATH_PREFIX, '/platform/shared/');
});

test('buildShareLink 拒绝空 token（后端 mint 的 opaque token 必须非空）', () => {
  assert.throws(() => buildShareLink('https://weknora.example.com', ''), /share token must not be empty/);
  assert.throws(() => buildShareLink('https://weknora.example.com', '   '), /share token must not be empty/);
});

test('normalizeShareToken 剥离空白；空值返回空串（路由守卫用）', () => {
  assert.equal(normalizeShareToken('  abc  '), 'abc');
  assert.equal(normalizeShareToken(''), '');
  assert.equal(normalizeShareToken(null), '');
});

test('isShareLinkInvalidError 仅认 404（token 无效或已撤销）', () => {
  assert.equal(isShareLinkInvalidError(new ApiError({ status: 404, code: 'NOT_FOUND', message: 'not found' })), true);
  assert.equal(isShareLinkInvalidError(new ApiError({ status: 403, code: 'FORBIDDEN', message: 'forbidden' })), false);
  assert.equal(isShareLinkInvalidError(new Error('network down')), false);
  assert.equal(isShareLinkInvalidError(undefined), false);
});

test('writeShareLinkClipboard 优先走注入的 clipboard.writeText', async () => {
  const written: string[] = [];
  await writeShareLinkClipboard('https://weknora.example.com/platform/shared/tok', {
    writeText: (text: string) => { written.push(text); return Promise.resolve(); },
  });
  assert.deepEqual(written, ['https://weknora.example.com/platform/shared/tok']);
});

test('writeShareLinkClipboard 传播 clipboard 失败（宿主 toast 兜底）', async () => {
  await assert.rejects(
    writeShareLinkClipboard('https://weknora.example.com/platform/shared/tok', {
      writeText: () => Promise.reject(new Error('denied')),
    }),
    /denied/,
  );
});
