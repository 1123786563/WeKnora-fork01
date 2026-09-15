// Translator for the documents tag surfaces: shared i18n (packages/i18n)
// first, then a byte-exact fallback table for the Vue copy that the shared
// bundle does not carry yet (common.confirm / common.clear / tenant.loadMore).
// Migration to packages/i18n is registered as a follow-up; once those keys
// land this table becomes dead weight and can be dropped.
import { formatMessage, type Locale } from '@weknora/i18n';

type FallbackKey = 'common.confirm' | 'common.clear' | 'common.operationFailed' | 'tenant.loadMore';

/** Values copied byte-exact from frontend/src/i18n/locales/*.ts. */
const FALLBACKS: Record<FallbackKey, Record<Locale, string>> = {
  // zh-CN.ts L4673 / en-US.ts L2264 / ja-JP.ts L2264 / ko-KR.ts L4671 / ru-RU.ts L4671
  'common.confirm': {
    'zh-CN': '确认',
    'en-US': 'Confirm',
    'ja-JP': '確認',
    'ko-KR': '확인',
    'ru-RU': 'Подтвердить',
  },
  // zh-CN.ts L4701 / en-US.ts L2292 / ja-JP.ts L2292 / ko-KR.ts L4699 / ru-RU.ts L4699
  'common.clear': {
    'zh-CN': '清空',
    'en-US': 'Clear',
    'ja-JP': 'クリア',
    'ko-KR': '지우기',
    'ru-RU': 'Очистить',
  },
  // zh-CN.ts L4715 / en-US.ts L2306 / ja-JP.ts L2306 / ko-KR.ts L4713 / ru-RU.ts L4713
  'common.operationFailed': {
    'zh-CN': '操作失败',
    'en-US': 'Operation failed',
    'ja-JP': '操作に失敗しました',
    'ko-KR': '작업 실패',
    'ru-RU': 'Операция не выполнена',
  },
  // zh-CN.ts L3318 / en-US.ts L3317 / ja-JP.ts L3317 / ko-KR.ts L3316 / ru-RU.ts L3316
  'tenant.loadMore': {
    'zh-CN': '加载更多',
    'en-US': 'Load more',
    'ja-JP': 'さらに読み込む',
    'ko-KR': '더 보기',
    'ru-RU': 'Загрузить еще',
  },
};

export type TagSurfaceT = (key: string, values?: Record<string, string | number>) => string;

/** Same contract as formatMessage: locale resolve, en-US fallback, key echo. */
export function tagSurfaceT(locale: Locale): TagSurfaceT {
  return (key, values) => {
    const shared = formatMessage(locale, key, values);
    if (shared !== key) return shared;
    const table = FALLBACKS[key as FallbackKey];
    if (!table) return shared;
    const template = table[locale] ?? table['en-US'];
    return template.replace(/\{(\w+)\}/g, (_match, name: string) => String(values?.[name] ?? `{${name}}`));
  };
}
