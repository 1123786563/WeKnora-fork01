import type { Locale } from '@weknora/i18n/runtime';

// Embed entry copy, one row per Vue embedPublish key
// (frontend/src/i18n/embed.ts embedPublish blocks, 5 locales).
export type EmbedTextKey =
  | 'loadError'
  | 'missingChannel'
  | 'invalidChannel'
  | 'sessionFailed'
  | 'channelDisabled'
  | 'loading'
  | 'awaitingToken'
  | 'defaultChatTitle'
  | 'newChat'
  | 'inputPlaceholder'
  | 'send';

export const embedTexts: Record<Locale, Record<EmbedTextKey, string>> = {
  'zh-CN': {
    loadError: '加载失败',
    missingChannel: '缺少嵌入渠道或 Token',
    invalidChannel: '无效的嵌入渠道',
    sessionFailed: '无法创建对话会话，请稍后重试',
    channelDisabled: '嵌入渠道已停用，请在 Agent 编辑器的「网页嵌入」中重新启用',
    loading: '加载中...',
    awaitingToken: '等待宿主页面提供 Token…',
    defaultChatTitle: 'AI 助手',
    newChat: '新建对话',
    inputPlaceholder: '请输入您的消息...',
    send: '发送',
  },
  'en-US': {
    loadError: 'Failed to load',
    missingChannel: 'Missing embed channel or token',
    invalidChannel: 'Invalid embed channel',
    sessionFailed: 'Failed to create chat session, please try again',
    channelDisabled: 'This embed channel is disabled. Re-enable it under Agent editor → Web Page Embed',
    loading: 'Loading...',
    awaitingToken: 'Waiting for host page to provide token…',
    defaultChatTitle: 'AI Assistant',
    newChat: 'New chat',
    inputPlaceholder: 'Type your message...',
    send: 'Send',
  },
  'ja-JP': {
    loadError: '読み込みに失敗しました',
    missingChannel: '埋め込みチャネルまたはトークンがありません',
    invalidChannel: '無効な埋め込みチャネルです',
    sessionFailed: 'チャットセッションの作成に失敗しました。再試行してください',
    channelDisabled: 'この埋め込みチャネルは無効になっています。エージェントエディタの「Webページ埋め込み」で再度有効にしてください',
    loading: '読み込み中...',
    awaitingToken: 'ホストページからトークンが渡されるのを待っています…',
    defaultChatTitle: 'AIアシスタント',
    newChat: '新しいチャット',
    inputPlaceholder: 'メッセージを入力...',
    send: '送信',
  },
  'ko-KR': {
    loadError: '로드 실패',
    missingChannel: '임베드 채널 또는 토큰 없음',
    invalidChannel: '잘못된 임베드 채널',
    sessionFailed: '대화 세션을 생성할 수 없습니다. 나중에 다시 시도하세요',
    channelDisabled: '임베드 채널이 비활성화되었습니다. 에이전트 편집기 → 웹 페이지 임베드에서 다시 활성화하세요',
    loading: '로딩 중...',
    awaitingToken: '호스트 페이지에서 토큰 제공 대기 중…',
    defaultChatTitle: 'AI 어시스턴트',
    newChat: '새 대화',
    inputPlaceholder: '메시지를 입력하세요...',
    send: '보내기',
  },
  'ru-RU': {
    loadError: 'Не удалось загрузить',
    missingChannel: 'Отсутствует канал встраивания или токен',
    invalidChannel: 'Недействительный канал встраивания',
    sessionFailed: 'Не удалось создать сессию чата, попробуйте позже',
    channelDisabled: 'Канал встраивания отключён. Включите в редакторе агента → Встраивание на веб-страницу',
    loading: 'Загрузка...',
    awaitingToken: 'Ожидание токена от страницы-хоста…',
    defaultChatTitle: 'AI-ассистент',
    newChat: 'Новый чат',
    inputPlaceholder: 'Введите сообщение...',
    send: 'Отправить',
  },
};

export function embedText(locale: Locale, key: EmbedTextKey): string {
  return embedTexts[locale][key];
}
