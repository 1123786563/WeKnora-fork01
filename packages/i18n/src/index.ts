export const supportedLocales = ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const;
export type Locale = (typeof supportedLocales)[number];
export type MessageValues = Record<string, string | number>;

export const messages: Record<Locale, Record<string, string>> = {
  'zh-CN': { 'auth.login': '登录', 'auth.email': '邮箱', 'auth.password': '密码', 'common.loading': '加载中…', 'common.retry': '重试', 'common.cancel': '取消', 'common.workspaceRequired': '请先选择一个工作空间', 'common.itemCount': '{count} 项', 'common.error': '发生错误', 'common.empty': '暂无内容', 'common.disabled': '不可用' },
  'en-US': { 'auth.login': 'Sign in', 'auth.email': 'Email', 'auth.password': 'Password', 'common.loading': 'Loading…', 'common.retry': 'Retry', 'common.cancel': 'Cancel', 'common.workspaceRequired': 'Choose a workspace first', 'common.itemCount': '{count} items', 'common.error': 'Something went wrong', 'common.empty': 'Nothing here yet', 'common.disabled': 'Unavailable' },
  'ja-JP': { 'auth.login': 'ログイン', 'auth.email': 'メール', 'auth.password': 'パスワード', 'common.loading': '読み込み中…', 'common.retry': '再試行', 'common.cancel': 'キャンセル', 'common.workspaceRequired': '先にワークスペースを選択してください', 'common.itemCount': '{count} 件', 'common.error': 'エラーが発生しました', 'common.empty': 'まだ内容がありません', 'common.disabled': '利用できません' },
  'ko-KR': { 'auth.login': '로그인', 'auth.email': '이메일', 'auth.password': '비밀번호', 'common.loading': '로드 중…', 'common.retry': '다시 시도', 'common.cancel': '취소', 'common.workspaceRequired': '먼저 워크스페이스를 선택하세요', 'common.itemCount': '{count}개', 'common.error': '문제가 발생했습니다', 'common.empty': '아직 내용이 없습니다', 'common.disabled': '사용할 수 없음' },
  'ru-RU': { 'auth.login': 'Войти', 'auth.email': 'Эл. почта', 'auth.password': 'Пароль', 'common.loading': 'Загрузка…', 'common.retry': 'Повторить', 'common.cancel': 'Отмена', 'common.workspaceRequired': 'Сначала выберите рабочее пространство', 'common.itemCount': '{count} элементов', 'common.error': 'Произошла ошибка', 'common.empty': 'Здесь пока ничего нет', 'common.disabled': 'Недоступно' },
};

export function formatMessage(locale: Locale, key: string, values: MessageValues = {}): string {
  const template = messages[locale][key] ?? messages['en-US'][key] ?? key;
  return template.replace(/\{(\w+)\}/g, (_match, name: string) => String(values[name] ?? `{${name}}`));
}

export function isLocale(value: string): value is Locale {
  return (supportedLocales as readonly string[]).includes(value);
}
