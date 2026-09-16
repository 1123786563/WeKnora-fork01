// AUTO-PORTED from frontend/src/i18n/locales/*.ts -> error block
// (frontend/src/utils/request.ts:138 rejects network failures with
// t('error.networkError')). Vue's locale sets are divergent here: en-US and
// ja-JP lack the key, and Vue's fallbackLocale is zh-CN — so Vue renders the
// zh-CN copy for those locales. The values below reproduce that rendered
// result byte-exactly while keeping the React key sets identical.
import type { Locale } from '../index.ts';

export const errorMessages: Record<Locale, Record<string, string>> = {
  "zh-CN": {"error.networkError":"网络错误，请检查您的网络连接"},
  "en-US": {"error.networkError":"网络错误，请检查您的网络连接"},
  "ja-JP": {"error.networkError":"网络错误，请检查您的网络连接"},
  "ko-KR": {"error.networkError":"네트워크 오류, 연결을 확인해 주세요"},
  "ru-RU": {"error.networkError":"Ошибка сети, проверьте подключение"},
};
