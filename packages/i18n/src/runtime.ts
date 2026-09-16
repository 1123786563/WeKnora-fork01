export const supportedLocales = ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const;
export type Locale = (typeof supportedLocales)[number];

export function isLocale(value: string): value is Locale {
  return (supportedLocales as readonly string[]).includes(value);
}

const loadingLabels: Record<Locale, string> = {
  'zh-CN': '加载中…', 'en-US': 'Loading…', 'ja-JP': '読み込み中…', 'ko-KR': '로드 중…', 'ru-RU': 'Загрузка…',
};

export function loadingLabel(locale: Locale): string { return loadingLabels[locale]; }
