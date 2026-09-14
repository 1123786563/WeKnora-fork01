import assert from 'node:assert/strict';
import test from 'node:test';

import {
  CHAT_COPY,
  CHAT_COPY_LOCALES,
  chatCopy,
  conversationTimeLabels,
  formatChatCopy,
  resolveChatCopy,
  resolveChatLocale,
  sessionGroupLabel,
} from './chat-copy.ts';

// The chat rendering layer resolves its copy per locale from these tables
// (byte-exact mirrors of the Vue locales; the shared-bundle generation is
// pinned by packages/i18n/test/chat.test.ts). These tests assert the
// per-locale resolution the components rely on.

test('every locale table carries the identical key set', () => {
  const reference = Object.keys(CHAT_COPY).sort();
  assert.ok(reference.length >= 70, `chat copy key count too small: ${reference.length}`);
  for (const locale of CHAT_COPY_LOCALES) {
    assert.deepEqual(Object.keys(resolveChatCopy(locale)).sort(), reference, `key drift for ${locale}`);
  }
});

test('resolveChatCopy returns locale tables and falls back to zh-CN', () => {
  assert.equal(resolveChatCopy('en-US').send, 'Send');
  assert.equal(resolveChatCopy('ja-JP').composerPlaceholder, 'モデルに直接質問できます');
  assert.equal(resolveChatCopy('ko-KR').sandboxPanelTitle, '샌드박스');
  assert.equal(resolveChatCopy('ru-RU').createChatTitle, 'Привет, я WeKnora — ваши знания всегда под рукой');
  assert.deepEqual(resolveChatCopy('fr-FR'), CHAT_COPY, 'unknown locale must fall back to zh-CN');
  assert.deepEqual(resolveChatCopy(undefined), CHAT_COPY);
  assert.deepEqual(resolveChatCopy('zh-CN'), CHAT_COPY);
});

test('resolveChatCopy exposes the Vue grep title-match label in every locale', () => {
  assert.equal(resolveChatCopy('zh-CN').grepTitleMatch, '标题匹配');
  assert.equal(resolveChatCopy('en-US').grepTitleMatch, 'title');
  assert.equal(resolveChatCopy('ja-JP').grepTitleMatch, 'タイトル一致');
  assert.equal(resolveChatCopy('ko-KR').grepTitleMatch, '제목');
  assert.equal(resolveChatCopy('ru-RU').grepTitleMatch, 'заголовок');
});

test('resolveChatCopy exposes the Vue unknown-link label in every locale', () => {
  const expected = {
    'zh-CN': '未知链接',
    'en-US': 'Unknown link',
    'ja-JP': '不明なリンク',
    'ko-KR': '알 수 없는 링크',
    'ru-RU': 'Неизвестная ссылка',
  } as const;
  for (const [locale, label] of Object.entries(expected)) {
    assert.equal(resolveChatCopy(locale).unknownLink, label);
  }
});

test('resolveChatCopy exposes the Vue full-content label in every locale', () => {
  const expected = {
    'zh-CN': '完整内容',
    'en-US': 'Full content',
    'ja-JP': '全文',
    'ko-KR': '전체 내용',
    'ru-RU': 'Полный текст',
  } as const;
  for (const [locale, label] of Object.entries(expected)) {
    assert.equal(resolveChatCopy(locale).fullContentLabel, label);
  }
});

test('batch session controls use translated English SSR copy', () => {
  const copy = resolveChatCopy('en-US');
  assert.equal(copy.batchManage, 'Batch manage');
  assert.equal(formatChatCopy(copy, 'batchDelete', { count: 3 }), 'Delete selected (3)');
  assert.equal(formatChatCopy(copy, 'batchDeleteConfirm', { count: 3 }), 'Delete the selected 3 chats? This cannot be undone.');
  assert.equal(copy.sourceSelectLabel, 'Chat source');
});

test('chatCopy stays bound to the zh-CN default table', () => {
  assert.equal(chatCopy('pageOf', { page: 2, total: 5 }), '第 2 页 / 共 5 页');
});

test('formatChatCopy interpolates per locale', () => {
  assert.equal(formatChatCopy(resolveChatCopy('en-US'), 'pageOf', { page: 2, total: 5 }), 'Page 2 of 5');
  assert.equal(formatChatCopy(resolveChatCopy('ja-JP'), 'pageOf', { page: 2, total: 5 }), '5 ページ中 2 ページ目');
  assert.equal(formatChatCopy(resolveChatCopy('zh-CN'), 'composerPlaceholder'), '直接向模型提问');
});

test('sessionGroupLabel resolves time groups per locale', () => {
  assert.equal(sessionGroupLabel(resolveChatCopy('zh-CN'), 'pinned'), '已置顶');
  assert.equal(sessionGroupLabel(resolveChatCopy('en-US'), 'last7Days'), 'Last 7 Days');
  assert.equal(sessionGroupLabel(resolveChatCopy('ja-JP'), 'older'), 'それ以前');
  assert.equal(sessionGroupLabel(resolveChatCopy('ko-KR'), 'today'), '오늘');
  assert.equal(sessionGroupLabel(resolveChatCopy('ru-RU'), 'yesterday'), 'Вчера');
  assert.equal(sessionGroupLabel(resolveChatCopy('en-US'), 'custom-key'), 'custom-key', 'unknown group keys pass through');
});

test('conversationTimeLabels follow the Vue conversationTime templates', () => {
  const zh = conversationTimeLabels(resolveChatCopy('zh-CN'));
  assert.equal(zh.today, '今天');
  assert.equal(zh.thisYear({ year: 2024, month: 3, day: 5 }), '3月5日');
  assert.equal(zh.otherYear({ year: 2024, month: 3, day: 5 }), '2024年3月5日');
  const en = conversationTimeLabels(resolveChatCopy('en-US'));
  assert.equal(en.today, 'Today');
  assert.equal(en.thisYear({ year: 2024, month: 3, day: 5 }), '3/5');
  assert.equal(en.otherYear({ year: 2024, month: 3, day: 5 }), '3/5/2024');
  const ru = conversationTimeLabels(resolveChatCopy('ru-RU'));
  assert.equal(ru.thisYear({ year: 2024, month: 3, day: 5 }), '5.3');
});

test('resolveChatLocale stays within the supported locale set', () => {
  // Runtime-dependent (Node >= 21 exposes navigator.language), so only the
  // contract is asserted: the resolved locale always has a copy table.
  assert.ok(CHAT_COPY_LOCALES.includes(resolveChatLocale()));
});
