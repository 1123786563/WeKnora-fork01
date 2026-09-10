export const supportedLocales = ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const;
export type Locale = (typeof supportedLocales)[number];
export type MessageValues = Record<string, string | number>;

export const messages: Record<Locale, Record<string, string>> = {
  'zh-CN': { 'auth.login': '登录', 'auth.email': '邮箱', 'auth.password': '密码', 'common.loading': '加载中…' },
  'en-US': { 'auth.login': 'Sign in', 'auth.email': 'Email', 'auth.password': 'Password', 'common.loading': 'Loading…' },
  'ja-JP': { 'auth.login': 'ログイン', 'auth.email': 'メール', 'auth.password': 'パスワード', 'common.loading': '読み込み中…' },
  'ko-KR': { 'auth.login': '로그인', 'auth.email': '이메일', 'auth.password': '비밀번호', 'common.loading': '로드 중…' },
  'ru-RU': { 'auth.login': 'Войти', 'auth.email': 'Эл. почта', 'auth.password': 'Пароль', 'common.loading': 'Загрузка…' },
};

export function formatMessage(locale: Locale, key: string, values: MessageValues = {}): string {
  const template = messages[locale][key] ?? messages['en-US'][key] ?? key;
  return template.replace(/\{(\w+)\}/g, (_match, name: string) => String(values[name] ?? `{${name}}`));
}

export function isLocale(value: string): value is Locale {
  return (supportedLocales as readonly string[]).includes(value);
}
