import { useState } from 'react';
import { formatMessage, isLocale, type Locale } from '@weknora/i18n';

// Same contract as LoginPage: default zh-CN, override via localStorage 'locale'.
export const LOCALE_STORAGE_KEY = 'locale';

export function readStoredLocale(storage?: { getItem(key: string): string | null }): Locale {
  const store = storage ?? (typeof window === 'undefined' ? undefined : window.localStorage);
  const stored = store?.getItem(LOCALE_STORAGE_KEY) ?? null;
  return stored && isLocale(stored) ? stored : 'zh-CN';
}

export function useAppLocale(): Locale {
  const [locale] = useState<Locale>(readStoredLocale);
  return locale;
}

export function createTranslator(locale: Locale) {
  return (key: string, values?: Record<string, string | number>): string => formatMessage(locale, key, values);
}
