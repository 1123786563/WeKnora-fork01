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
  | 'send'
  | 'suggestedQuestions'
  | 'referencesTitle'
  | 'referencesDocCount'
  | 'referencesWebCount'
  | 'referencesDocAndWebCount'
  | 'followUpQuestions'
  | 'refreshSuggestedQuestions'
  | 'close';

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
    suggestedQuestions: '你可以这样问我',
    referencesTitle: '参考了{count}个相关内容',
    referencesDocCount: '引用了{count}篇文档',
    referencesWebCount: '参考了{count}条网页',
    referencesDocAndWebCount: '引用了{docCount}篇文档和{webCount}条网页',
    followUpQuestions: '继续问',
    refreshSuggestedQuestions: '换一批',
    close: '关闭',
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
    suggestedQuestions: 'You can ask me',
    referencesTitle: 'Referenced {count} related item(s)',
    referencesDocCount: 'Referenced {count} document(s)',
    referencesWebCount: 'Referenced {count} web result(s)',
    referencesDocAndWebCount: 'Referenced {docCount} document(s) and {webCount} web result(s)',
    followUpQuestions: 'Keep asking',
    refreshSuggestedQuestions: 'More',
    close: 'Close',
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
    suggestedQuestions: 'こんな質問ができます',
    referencesTitle: '関連する内容を{count}件参照',
    referencesDocCount: '{count}件のドキュメントを参照',
    referencesWebCount: '{count}件のWebページを参照',
    referencesDocAndWebCount: '{docCount}件のドキュメントと{webCount}件のWebページを参照',
    followUpQuestions: '続けて質問',
    refreshSuggestedQuestions: '別の質問',
    close: '閉じる',
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
    suggestedQuestions: '이렇게 물어보세요',
    referencesTitle: '{count}개의 관련 내용 참조',
    referencesDocCount: '{count}개 문서 참조',
    referencesWebCount: '{count}개 웹 결과 참조',
    referencesDocAndWebCount: '{docCount}개 문서와 {webCount}개 웹 결과 참조',
    followUpQuestions: '이어서 질문',
    refreshSuggestedQuestions: '다른 질문',
    close: '닫기',
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
    suggestedQuestions: 'Вы можете спросить меня',
    referencesTitle: 'Использовано {count} связанного материала',
    referencesDocCount: 'Использовано {count} документ(ов)',
    referencesWebCount: 'Использовано {count} веб-результат(ов)',
    referencesDocAndWebCount: 'Использовано {docCount} документ(ов) и {webCount} веб-результат(ов)',
    followUpQuestions: 'Спрашивайте дальше',
    refreshSuggestedQuestions: 'Ещё',
    close: 'Закрыть',
  },
};

export function embedText(locale: Locale, key: EmbedTextKey, params?: Record<string, string | number>): string {
  const template = embedTexts[locale][key];
  if (!params) return template;
  return template.replace(/\{(\w+)\}/g, (match, name: string) =>
    Object.prototype.hasOwnProperty.call(params, name) ? String(params[name]) : match,
  );
}
