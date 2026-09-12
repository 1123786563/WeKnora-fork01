import { useState } from 'react';
import { Button, Card, Status } from '@weknora/ui';
import { formatMessage, isLocale, type Locale } from '@weknora/i18n';
import { readLocalPreferences, writeLocalPreferences, type ThemeMode, type FontSize } from '@weknora/domain/settings/local-preferences';

function readInitialLocale(): Locale {
  const stored = window.localStorage.getItem('locale');
  return stored && isLocale(stored) ? stored : 'zh-CN';
}

const T = (locale: Locale) => ({
  theme: locale === 'zh-CN' ? '主题模式' : locale === 'ja-JP' ? 'テーマ' : locale === 'ko-KR' ? '테마' : locale === 'ru-RU' ? 'Тема' : 'Theme',
  themeDesc: locale === 'zh-CN' ? '选择界面的显示主题' : locale === 'ja-JP' ? 'インターフェースの表示テーマを選択します' : locale === 'ko-KR' ? '인터페이스 표시 테마를 선택합니다' : locale === 'ru-RU' ? 'Выберите тему интерфейса' : 'Select the display theme',
  themeLight: locale === 'zh-CN' ? '浅色' : 'Light',
  themeDark: locale === 'zh-CN' ? '深色' : 'Dark',
  themeSystem: locale === 'zh-CN' ? '跟随系统' : 'System',
  saved: locale === 'zh-CN' ? '设置已保存' : 'Settings saved',
});

export function GeneralPreferencesPanel() {
  const [locale, setLocale] = useState<Locale>(() => {
    const stored = window.localStorage.getItem('locale');
    return stored && isLocale(stored) ? stored : 'zh-CN';
  });
  const [theme, setTheme] = useState<ThemeMode>(() => {
    try { return readLocalPreferences(window.localStorage).theme; } catch { return 'system'; }
  });
  const [saved, setSaved] = useState(false);

  function applyTheme(next: ThemeMode) {
    setTheme(next);
    writeLocalPreferences(window.localStorage, { theme: next });
    setSaved(true);
    setTimeout(() => setSaved(false), 2000);
  }

  const L = T(locale);
  return <Card data-testid="general-preferences-panel">
    <h3>{L.theme}</h3>
    <p className="wk-muted">{L.themeDesc}</p>
    <div className="wks-actions">
      <Button type="button" aria-pressed={theme === 'light'} onClick={() => applyTheme('light')}>{L.themeLight}</Button>
      <Button type="button" aria-pressed={theme === 'dark'} onClick={() => applyTheme('dark')}>{L.themeDark}</Button>
      <Button type="button" aria-pressed={theme === 'system'} onClick={() => applyTheme('system')}>{L.themeSystem}</Button>
    </div>
    {saved ? <Status tone="success">{L.saved}</Status> : null}
  </Card>;
}
