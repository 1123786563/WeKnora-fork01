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
  // CHAT-5 —— Vue batchManage.delete + (N) 直拼：Delete Conversations(3)。
  assert.equal(copy.batchSelectAll, 'Select All');
  assert.equal(formatChatCopy(copy, 'batchDelete', { count: 3 }), 'Delete Conversations(3)');
  assert.equal(formatChatCopy(copy, 'batchDeleteConfirm', { count: 3 }), 'Are you sure you want to delete the selected 3 conversation(s)? This action cannot be undone.');
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

test('resolveChatLocale ignores navigator.language like the Vue deployment default', () => {
  // Upstream i18n/index.ts resolves localStorage['locale'] || deployment
  // default (zh-CN) — it never sniffs the browser language, so an English
  // browser still gets the Chinese deployment default. The chat view must
  // follow the same convention (browser sniffing here produced an English
  // chat page inside an otherwise-Chinese app — 2026-09-18 round 7).
  const originalWindow = (globalThis as { window?: unknown }).window;
  const originalNavigator = globalThis.navigator;
  try {
    (globalThis as { window?: unknown }).window = {
      navigator: { language: 'en-US' },
      localStorage: { getItem: () => null },
    };
    // No stored preference: deployment default wins over the browser locale.
    assert.equal(resolveChatLocale(), 'zh-CN');
    // Explicit stored choice still wins.
    (globalThis as { window?: unknown }).window = {
      navigator: { language: 'en-US' },
      localStorage: { getItem: (key: string) => (key === 'locale' ? 'ja-JP' : null) },
    };
    assert.equal(resolveChatLocale(), 'ja-JP');
  } finally {
    (globalThis as { window?: unknown }).window = originalWindow;
    Object.defineProperty(globalThis, 'navigator', { configurable: true, value: originalNavigator });
  }
});

test('resolveChatCopy exposes the Vue tool-approval args editing labels in every locale', () => {  // Byte-exact mirrors of frontend/src/i18n/locales/*.ts agentStream.toolApproval.
  const modified = {
    'zh-CN': '已修改',
    'en-US': 'Modified',
    'ja-JP': '変更あり',
    'ko-KR': '수정됨',
    'ru-RU': 'Изменено',
  } as const;
  const rejected = {
    'zh-CN': '用户拒绝',
    'en-US': 'User rejected',
    'ja-JP': 'ユーザが拒否しました',
    'ko-KR': '사용자 거부',
    'ru-RU': 'Отклонено пользователем',
  } as const;
  for (const locale of CHAT_COPY_LOCALES) {
    assert.equal(resolveChatCopy(locale).approvalArgsModified, modified[locale]);
    assert.equal(resolveChatCopy(locale).approvalRejectedReason, rejected[locale]);
  }
});

test('resolveChatCopy exposes the Vue invalid-image placeholder in every locale', () => {
  // Byte-exact mirrors of frontend/src/i18n/locales/*.ts error.invalidImageLink
  // (botmsg.vue renders `<p>${t('error.invalidImageLink')}</p>` for images that
  // fail isValidImageURL). ru-RU ships the English string upstream — copied as-is.
  const expected = {
    'zh-CN': '无效的图片链接',
    'en-US': 'Invalid image link',
    'ja-JP': '無効な画像リンクです',
    'ko-KR': '유효하지 않은 이미지 링크',
    'ru-RU': 'Invalid image link',
  } as const;
  for (const locale of CHAT_COPY_LOCALES) {
    assert.equal(resolveChatCopy(locale).invalidImageLink, expected[locale]);
  }
});

test('resolveChatCopy exposes the Vue steer scenario toasts in every locale', () => {
  // R476-A2 — byte-exact mirrors of frontend/src/i18n/locales/*.ts
  // input.messages.{steerFailed,steerPromoteFailed,steerRemoveFailed,
  // steerAlreadyInjected}: Vue chat/index.vue MessagePlugin calls keyed per
  // scenario (enqueue/promote/remove failures + the already_injected info
  // notice) instead of one operationFailed bucket.
  const expected = {
    steerFailed: {
      'zh-CN': '追加失败，请重试',
      'en-US': 'Failed to append the message. Please try again.',
      'ja-JP': 'メッセージの追加に失敗しました。再試行してください。',
      'ko-KR': '메시지 추가에 실패했습니다. 다시 시도해주세요.',
      'ru-RU': 'Не удалось добавить сообщение. Попробуйте ещё раз.',
    },
    steerPromoteFailed: {
      'zh-CN': '立即发送失败，请重试',
      'en-US': 'Failed to send now. Please try again.',
      'ja-JP': '今すぐ送信に失敗しました。再試行してください。',
      'ko-KR': '지금 보내기에 실패했습니다. 다시 시도해주세요.',
      'ru-RU': 'Не удалось отправить сейчас. Попробуйте ещё раз.',
    },
    steerRemoveFailed: {
      'zh-CN': '删除排队消息失败，请重试',
      'en-US': 'Failed to remove the queued message. Please try again.',
      'ja-JP': '待機中のメッセージを削除できませんでした。再試行してください。',
      'ko-KR': '대기 중인 메시지를 삭제하지 못했습니다. 다시 시도해주세요.',
      'ru-RU': 'Не удалось удалить сообщение из очереди. Попробуйте ещё раз.',
    },
    steerAlreadyInjected: {
      'zh-CN': '该消息已被当前回答接收',
      'en-US': 'This message has already been taken by the running answer.',
      'ja-JP': 'このメッセージは既に現在の回答に取り込まれています。',
      'ko-KR': '이 메시지는 이미 현재 응답에 반영되었습니다.',
      'ru-RU': 'Это сообщение уже принято текущим ответом.',
    },
  } as const;
  for (const [key, perLocale] of Object.entries(expected)) {
    for (const locale of CHAT_COPY_LOCALES) {
      assert.equal(resolveChatCopy(locale)[key as keyof typeof expected], perLocale[locale], `${key} must be byte-exact for ${locale}`);
    }
  }
});

/*
 * R477-A2 — byte-exact mirrors of frontend/src/i18n/locales/*.ts
 * input.messages.{steerAttachmentPending,steerHasAttachments}: Vue
 * Input-field.vue toasts (MessagePlugin.warning) the first key when a queued
 * steer still has an uploading attachment, the second when any attachment is
 * present on the steer path. These replace the R476 composite
 * steerAttachmentsBlocked key.
 */
test('resolveChatCopy exposes the Vue steer attachment warnings in every locale', () => {
  const expected = {
    steerAttachmentPending: {
      'zh-CN': '附件尚未上传完成，请稍后再追加',
      'en-US': 'Attachment is still uploading. Please try again shortly.',
      'ja-JP': '添付ファイルのアップロードが完了していません。しばらくしてから追加してください。',
      'ko-KR': '첨부 파일 업로드가 완료되지 않았습니다. 잠시 후 다시 시도해주세요.',
      'ru-RU': 'Вложение ещё загружается. Повторите попытку чуть позже.',
    },
    steerHasAttachments: {
      'zh-CN': '进行中的回答无法附带附件，请先移除附件或等当前回答结束后再发送',
      'en-US': 'Attachments cannot be added to a running answer. Remove them first, or send after it finishes.',
      'ja-JP': '実行中の回答には添付ファイルを追加できません。先に削除するか、終了後に送信してください。',
      'ko-KR': '진행 중인 응답에는 첨부 파일을 추가할 수 없습니다. 먼저 제거하거나 응답이 끝난 뒤 보내주세요.',
      'ru-RU': 'К текущему ответу нельзя добавить вложения. Удалите их или отправьте после завершения.',
    },
  } as const;
  for (const [key, perLocale] of Object.entries(expected)) {
    for (const locale of CHAT_COPY_LOCALES) {
      assert.equal(resolveChatCopy(locale)[key as keyof typeof expected], perLocale[locale], `${key} must be byte-exact for ${locale}`);
    }
  }
});

/*
 * CHAT-4 / CHAT-6 — byte-exact mirrors of frontend/src/i18n/locales/*.ts
 * chatHeader.{clearConfirmTitle,clearConfirmBody} and
 * batchManage.{deleteConfirmTitle,deleteConfirmBody,delete}: the styled
 * confirmation dialogs (batch delete, clear messages) must carry the Vue
 * copy, including the 「对话本身会保留」 keep-conversation semantics.
 */
test('session confirm dialogs carry the Vue clear/batch-delete copy in every locale', () => {
  const expected = {
    clearConfirmTitle: {
      'zh-CN': '清空对话消息',
      'en-US': 'Clear Conversation Messages',
      'ja-JP': '会話のメッセージをクリア',
      'ko-KR': '대화 메시지 지우기',
      'ru-RU': 'Очистить сообщения диалога',
    },
    clearConfirmBody: {
      'zh-CN': '确认清空当前对话的全部消息？对话本身会保留，此操作无法恢复。',
      'en-US': 'Clear all messages in this conversation? The conversation will remain, but this cannot be undone.',
      'ja-JP': 'この会話のメッセージをすべてクリアしますか？会話自体は残りますが、この操作は取り消せません。',
      'ko-KR': '현재 대화의 모든 메시지를 지울까요? 대화는 유지되지만 복구할 수 없습니다.',
      'ru-RU': 'Очистить все сообщения? Диалог останется, но это действие нельзя отменить.',
    },
    batchDeleteConfirmTitle: {
      'zh-CN': '删除对话',
      'en-US': 'Delete Conversations',
      'ja-JP': '会話を削除',
      'ko-KR': '대화 삭제',
      'ru-RU': 'Удалить диалоги',
    },
    batchDeleteConfirmAction: {
      'zh-CN': '删除对话',
      'en-US': 'Delete Conversations',
      'ja-JP': '会話を削除',
      'ko-KR': '대화 삭제',
      'ru-RU': 'Удалить диалоги',
    },
    batchDeleteConfirmBody: {
      'zh-CN': '确定要删除选中的 1 条对话吗？删除后无法恢复。',
      'en-US': 'Are you sure you want to delete the selected 1 conversation(s)? This action cannot be undone.',
      'ja-JP': '選択した1件の会話を削除してもよろしいですか？この操作は取り消せません。',
      'ko-KR': '선택한 1개의 대화를 삭제하시겠습니까? 삭제 후 복구할 수 없습니다.',
      'ru-RU': 'Вы уверены, что хотите удалить выбранные 1 диалог(ов)? Это действие необратимо.',
    },
  } as const;
  for (const [key, perLocale] of Object.entries(expected)) {
    for (const locale of CHAT_COPY_LOCALES) {
      const table = resolveChatCopy(locale);
      const actual = key === 'batchDeleteConfirmBody'
        ? formatChatCopy(table, 'batchDeleteConfirm', { count: 1 })
        : table[key as keyof typeof perLocale];
      assert.equal(actual, perLocale[locale], `${key} must be byte-exact for ${locale}`);
    }
  }
});
