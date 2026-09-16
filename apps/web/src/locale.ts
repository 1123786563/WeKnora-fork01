// Locale resolution — Vue GeneralSettings.vue parity: the stored `locale`
// preference (default zh-CN) is the single source of truth, not
// navigator.language. Reacts to weknora:locale-changed like theme.ts does
// for weknora:theme-changed.
import { useCallback, useEffect, useState } from 'react';
import { isLocale, type Locale } from '@weknora/i18n';
import { readLocalPreferences } from '@weknora/domain/settings/local-preferences';

export function readPreferredLocale(): Locale {
  try {
    const stored = readLocalPreferences(window.localStorage).locale;
    return stored && isLocale(stored) ? stored : 'zh-CN';
  } catch {
    return 'zh-CN';
  }
}

export function usePreferredLocale(): Locale {
  const [locale, setLocale] = useState<Locale>(readPreferredLocale);
  const reread = useCallback(() => setLocale(readPreferredLocale()), []);
  useEffect(() => {
    window.addEventListener('weknora:locale-changed', reread);
    return () => window.removeEventListener('weknora:locale-changed', reread);
  }, [reread]);
  return locale;
}
