import type { Locale } from '@weknora/i18n';

const CLEAR_CONFIRMATIONS: Record<Locale, string> = {
  'zh-CN': '确认清空当前对话的消息吗？',
  'en-US': 'Clear messages in this conversation?',
  'ja-JP': 'この会話のメッセージを削除しますか？',
  'ko-KR': '이 대화의 메시지를 지우시겠습니까?',
  'ru-RU': 'Очистить сообщения в этом диалоге?',
};

export function chatClearConfirmation(locale: Locale): string {
  return CLEAR_CONFIRMATIONS[locale];
}
