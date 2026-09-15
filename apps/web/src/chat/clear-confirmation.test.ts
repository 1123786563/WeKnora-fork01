import assert from 'node:assert/strict';
import test from 'node:test';

import { chatClearConfirmation } from './clear-confirmation.ts';

test('uses the Vue clear-message confirmation in every supported chat locale', () => {
  assert.equal(chatClearConfirmation('zh-CN'), '确认清空当前对话的消息吗？');
  assert.equal(chatClearConfirmation('en-US'), 'Clear messages in this conversation?');
  assert.equal(chatClearConfirmation('ja-JP'), 'この会話のメッセージを削除しますか？');
  assert.equal(chatClearConfirmation('ko-KR'), '이 대화의 메시지를 지우시겠습니까?');
  assert.equal(chatClearConfirmation('ru-RU'), 'Очистить сообщения в этом диалоге?');
});
