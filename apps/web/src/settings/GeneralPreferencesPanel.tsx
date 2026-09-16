import { useEffect, useMemo, useState } from 'react';
import { formatMessage, isLocale, type Locale } from '@weknora/i18n';
import { readLocalPreferences, writeLocalPreferences, isValidTheme, isValidFontSize, type ThemeMode, type FontSize } from '@weknora/domain/settings/local-preferences';
import { Select, Status, Switch } from '@weknora/ui';

function readStoredLocale(): Locale {
  const stored = window.localStorage.getItem('locale');
  return stored && isLocale(stored) ? stored : 'zh-CN';
}

// Font catalogue ported from frontend/src/composables/useFont.ts: the stacks
// are verbatim; options are filtered by host platform so the picker only
// offers fonts the user will actually see (useFont.ts visibleSansKeys).
const SANS_STACKS: Record<string, string> = {
  system: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif',
  pingfang: '"PingFang SC", "Microsoft YaHei", "Hiragino Sans GB", -apple-system, BlinkMacSystemFont, sans-serif',
  georgia: 'Georgia, "Times New Roman", "Songti SC", "SimSun", serif',
  yahei: '"Microsoft YaHei", "PingFang SC", "Hiragino Sans GB", Tahoma, Arial, sans-serif',
  times: '"Times New Roman", Times, Georgia, "SimSun", "Songti SC", serif',
  'noto-cjk': '"Noto Sans CJK SC", "Noto Sans SC", "Source Han Sans SC", "WenQuanYi Micro Hei", "Microsoft YaHei", sans-serif',
  'dejavu-serif': '"DejaVu Serif", "Liberation Serif", "Noto Serif", Georgia, "Times New Roman", serif',
  'sans-serif': 'sans-serif',
};
const MONO_STACKS: Record<string, string> = {
  system: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
  menlo: 'Menlo, Monaco, Consolas, "Courier New", monospace',
  monaco: 'Monaco, Menlo, Consolas, "Courier New", monospace',
  consolas: 'Consolas, "Courier New", Menlo, Monaco, monospace',
  cascadia: '"Cascadia Code", "Cascadia Mono", Consolas, "Courier New", monospace',
  'dejavu-mono': '"DejaVu Sans Mono", "Liberation Mono", Menlo, Consolas, monospace',
  'liberation-mono': '"Liberation Mono", "DejaVu Sans Mono", Menlo, Consolas, monospace',
  monospace: 'monospace',
};
const FONT_SCALES: Record<FontSize, number> = { small: 0.875, normal: 1, large: 1.125 };

function detectPlatform(): 'mac' | 'windows' | 'linux' {
  const ua = window.navigator.userAgent;
  if (/Mac|iPhone|iPad/i.test(ua)) return 'mac';
  if (/Windows/i.test(ua)) return 'windows';
  return 'linux';
}
function visibleSansKeys(platform: 'mac' | 'windows' | 'linux'): string[] {
  if (platform === 'mac') return ['system', 'pingfang', 'georgia', 'sans-serif'];
  if (platform === 'windows') return ['system', 'yahei', 'times', 'sans-serif'];
  return ['system', 'noto-cjk', 'dejavu-serif', 'sans-serif'];
}
function visibleMonoKeys(platform: 'mac' | 'windows' | 'linux'): string[] {
  if (platform === 'mac') return ['system', 'menlo', 'monaco', 'monospace'];
  if (platform === 'windows') return ['system', 'consolas', 'cascadia', 'monospace'];
  return ['system', 'dejavu-mono', 'liberation-mono', 'monospace'];
}

// Shared i18n ships both the field labels (font.uiFont …) and the per-option
// labels (font.sans.pingfang …) for all 5 locales; this table only backs the
// lookup up in case a key is ever pruned — it mirrors the Vue zh-CN locale
// table (frontend/src/i18n/locales/zh-CN.ts) verbatim.
const FONT_LABELS: Record<string, Record<string, string>> = {
  'zh-CN': {
    'sans:system': '系统默认', 'sans:pingfang': '苹方 PingFang SC', 'sans:georgia': 'Georgia 衬线', 'sans:yahei': '微软雅黑 Microsoft YaHei',
    'sans:times': 'Times New Roman 衬线', 'sans:noto-cjk': 'Noto Sans CJK', 'sans:dejavu-serif': 'DejaVu Serif 衬线', 'sans:sans-serif': '通用无衬线',
    'mono:system': '系统默认', 'mono:menlo': 'Menlo', 'mono:monaco': 'Monaco', 'mono:consolas': 'Consolas', 'mono:cascadia': 'Cascadia Code',
    'mono:dejavu-mono': 'DejaVu Sans Mono', 'mono:liberation-mono': 'Liberation Mono', 'mono:monospace': '通用等宽',
  },
};

const AUTO_UPDATE_COPY: Record<Locale, { label: string; description: string }> = {
  'zh-CN': { label: '自动检查更新', description: '开启后自动检查并在后台下载最新版本安装包。' },
  'en-US': { label: 'Automatically check for updates', description: 'When enabled, automatically check and download the latest version in the background.' },
  'ja-JP': { label: '更新を自動的に確認', description: '有効にすると、バックグラウンドで最新バージョンを自動的に確認・ダウンロードします。' },
  'ko-KR': { label: '자동 업데이트 확인', description: '활성화하면 시작 시 최신 버전을 자동으로 확인하고 백그라운드에서 다운로드합니다.' },
  'ru-RU': { label: 'Автоматически проверять обновления', description: 'При включении автоматически проверять и скачивать последнюю версию в фоновом режиме при запуске.' },
};

function readAutoCheckUpdate(): boolean {
  try {
    const raw = window.localStorage.getItem('WeKnora_settings');
    if (!raw) return true;
    const stored = JSON.parse(raw) as unknown;
    return typeof stored === 'object' && stored !== null && 'autoCheckUpdate' in stored
      ? (stored as { autoCheckUpdate?: unknown }).autoCheckUpdate !== false
      : true;
  } catch {
    return true;
  }
}

function writeAutoCheckUpdate(enabled: boolean): void {
  let stored: Record<string, unknown> = {};
  try {
    const parsed = JSON.parse(window.localStorage.getItem('WeKnora_settings') ?? '{}') as unknown;
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) stored = parsed as Record<string, unknown>;
  } catch {
    // Vue's settings store falls back to defaults when the persisted record is corrupt.
  }
  window.localStorage.setItem('WeKnora_settings', JSON.stringify({ ...stored, autoCheckUpdate: enabled }));
}
function fontLabel(locale: Locale, group: 'sans' | 'mono', key: string): string {
  const i18nKey = 'font.' + group + '.' + key;
  const viaI18n = formatMessage(locale, i18nKey);
  if (viaI18n !== i18nKey) return viaI18n;
  const table = FONT_LABELS[locale] ?? FONT_LABELS['zh-CN']!;
  return table[group + ':' + key] ?? key;
}

function applyFontCssVariables(sans: string, mono: string, size: FontSize): void {
  const root = document.documentElement;
  root.style.setProperty('--wk-font-sans', SANS_STACKS[sans] ?? SANS_STACKS.system!);
  root.style.setProperty('--wk-font-mono', MONO_STACKS[mono] ?? MONO_STACKS.system!);
  root.style.setProperty('--wk-font-scale', String(FONT_SCALES[size]));
}

export function GeneralPreferencesPanel({ liteMode = false }: { liteMode?: boolean }) {
  const [locale, setLocale] = useState<Locale>(readStoredLocale);
  const [theme, setTheme] = useState<ThemeMode>(() => {
    // Vue useTheme.ts default: 'light' when storage is unavailable.
    try { return readLocalPreferences(window.localStorage).theme; } catch { return 'light'; }
  });
  const [fontSize, setFontSize] = useState<FontSize>(() => {
    try { return readLocalPreferences(window.localStorage).fontSize; } catch { return 'normal'; }
  });
  const [sansFont, setSansFont] = useState<string>(() => window.localStorage.getItem('font_sans') ?? 'system');
  const [monoFont, setMonoFont] = useState<string>(() => window.localStorage.getItem('font_mono') ?? 'system');
  const [autoCheckUpdate, setAutoCheckUpdate] = useState(readAutoCheckUpdate);
  // Vue GeneralSettings.vue closes each accepted preference change with a
  // MessagePlugin.success (language.languageSaved / common.success); the React
  // domain surfaces the same feedback as an inline success Status.
  const [notice, setNotice] = useState<string | null>(null);
  const platform = useMemo(detectPlatform, []);
  const t = (key: string) => formatMessage(locale, key);
  const autoUpdateCopy = AUTO_UPDATE_COPY[locale];

  // Vue useFont() applies persisted font preferences during initialization;
  // mirror that behavior when this panel is mounted so a reload does not
  // silently revert the application chrome until the user changes a control.
  useEffect(() => {
    applyFontCssVariables(sansFont, monoFont, fontSize);
  }, []);

  function handleLanguageChange(next: string) {
    if (!isLocale(next)) return;
    setLocale(next);
    window.localStorage.setItem('locale', next);
    window.dispatchEvent(new window.Event('weknora:locale-changed'));
    // Vue resolves the toast after locale.value updates, so the message uses
    // the NEW locale.
    setNotice(formatMessage(next, 'language.languageSaved'));
  }
  function handleThemeChange(next: string) {
    if (!isValidTheme(next)) return;
    setTheme(next);
    writeLocalPreferences(window.localStorage, { theme: next });
    window.dispatchEvent(new window.Event('weknora:theme-changed'));
    setNotice(formatMessage(locale, 'common.success'));
  }
  function handleFontSizeChange(next: string) {
    if (!isValidFontSize(next)) return;
    setFontSize(next);
    writeLocalPreferences(window.localStorage, { fontSize: next });
    applyFontCssVariables(sansFont, monoFont, next);
    setNotice(formatMessage(locale, 'common.success'));
  }
  function handleSansFontChange(next: string) {
    setSansFont(next);
    window.localStorage.setItem('font_sans', next);
    applyFontCssVariables(next, monoFont, fontSize);
    setNotice(formatMessage(locale, 'common.success'));
  }
  function handleMonoFontChange(next: string) {
    setMonoFont(next);
    window.localStorage.setItem('font_mono', next);
    applyFontCssVariables(sansFont, next, fontSize);
    setNotice(formatMessage(locale, 'common.success'));
  }

  function handleAutoCheckUpdateChange(enabled: boolean) {
    setAutoCheckUpdate(enabled);
    writeAutoCheckUpdate(enabled);
  }

  const sansOptions = visibleSansKeys(platform);
  const monoOptions = visibleMonoKeys(platform);
  const currentSansStack = SANS_STACKS[sansFont] ?? SANS_STACKS.system!;
  const currentMonoStack = MONO_STACKS[monoFont] ?? MONO_STACKS.system!;

  return (
    <div className="general-settings" data-testid="general-preferences-panel">
      <div className="section-header">
        <h2>{t('general.title')}</h2>
        <p className="section-description">{t('general.description')}</p>
      </div>
      {notice ? <div data-testid="general-preferences-notice" role="status"><Status tone="success">{notice}</Status></div> : null}
      <div className="settings-group">
        <div className="setting-row">
          <div className="setting-info">
            <label>{t('language.language')}</label>
            <p className="desc">{t('language.languageDescription')}</p>
          </div>
          <div className="setting-control">
            <Select
              className="w-[280px] max-w-[280px] max-[720px]:w-full max-[720px]:max-w-full"
              aria-label={t('language.selectLanguage')}
              value={locale}
              onChange={(event) => handleLanguageChange(event.target.value)}
            >
              <option value="zh-CN">{t('language.zhCN')}</option>
              <option value="en-US">{t('language.enUS')}</option>
              <option value="ru-RU">{t('language.ruRU')}</option>
              <option value="ko-KR">{t('language.koKR')}</option>
              <option value="ja-JP">{t('language.jaJP')}</option>
            </Select>
          </div>
        </div>
        <div className="setting-row">
          <div className="setting-info">
            <label>{t('theme.theme')}</label>
            <p className="desc">{t('theme.themeDescription')}</p>
          </div>
          <div className="setting-control">
            <Select
              className="w-[280px] max-w-[280px] max-[720px]:w-full max-[720px]:max-w-full"
              aria-label={t('theme.selectTheme')}
              value={theme}
              onChange={(event) => handleThemeChange(event.target.value)}
            >
              <option value="light">{t('theme.light')}</option>
              <option value="dark">{t('theme.dark')}</option>
              <option value="system">{t('theme.system')}</option>
            </Select>
          </div>
        </div>
        <div className="setting-row">
          <div className="setting-info">
            <label>{t('font.uiFont')}</label>
            <p className="desc">{t('font.uiFontDescription')}</p>
          </div>
          <div className="setting-control setting-control--stacked">
            <Select
              className="w-[280px] max-w-[280px] max-[720px]:w-full max-[720px]:max-w-full"
              aria-label={t('font.selectFont')}
              value={sansFont}
              onChange={(event) => handleSansFontChange(event.target.value)}
            >
              {sansOptions.map((key) => (
                <option key={key} value={key}>{fontLabel(locale, 'sans', key)}</option>
              ))}
            </Select>
            {/* Vue GeneralSettings.vue font preview box (lines 351-375): bg
                --td-bg-color-container #fff, border --td-component-stroke
                #e7e7e7, radius --td-radius-medium 6px. */}
            <div data-testid="font-preview-sans" className="box-border w-[280px] max-w-[280px] max-[720px]:w-full max-[720px]:max-w-full rounded-[6px] border border-line-neutral bg-surface px-[12px] py-[8px] text-[14px] leading-[1.4]" style={{ fontFamily: currentSansStack }}>
              {t('font.sansPreview')}
            </div>
          </div>
        </div>
        <div className="setting-row">
          <div className="setting-info">
            <label>{t('font.monoFont')}</label>
            <p className="desc">{t('font.monoFontDescription')}</p>
          </div>
          <div className="setting-control setting-control--stacked">
            <Select
              className="w-[280px] max-w-[280px] max-[720px]:w-full max-[720px]:max-w-full"
              aria-label={t('font.selectFont')}
              value={monoFont}
              onChange={(event) => handleMonoFontChange(event.target.value)}
            >
              {monoOptions.map((key) => (
                <option key={key} value={key}>{fontLabel(locale, 'mono', key)}</option>
              ))}
            </Select>
            <div data-testid="font-preview-mono" className="box-border w-[280px] max-w-[280px] max-[720px]:w-full max-[720px]:max-w-full rounded-[6px] border border-line-neutral bg-surface px-[12px] py-[8px] text-[14px] leading-[1.4] font-[family-name:var(--wk-font-mono,ui-monospace,monospace)] overflow-hidden text-ellipsis whitespace-nowrap" style={{ fontFamily: currentMonoStack }}>
              {t('font.monoPreview')}
            </div>
          </div>
        </div>
        <div className="setting-row">
          <div className="setting-info">
            <label>{t('font.fontSize')}</label>
            <p className="desc">{t('font.fontSizeDescription')}</p>
          </div>
          <div className="setting-control">
            <div className="inline-flex overflow-hidden rounded-[3px] border border-[#e7e7e7]" role="radiogroup" aria-label={t('font.fontSize')}>
              {(['small', 'normal', 'large'] as const).map((size, index) => (
                <button
                  key={size}
                  type="button"
                  role="radio"
                  aria-checked={fontSize === size}
                  className={'h-7 cursor-pointer border-0 bg-transparent px-4 py-0 font-[inherit] text-[length:inherit] leading-[inherit]'
                    + (index > 0 ? ' border-l border-l-[#e7e7e7]' : '')
                    + (fontSize === size
                      ? ' is-active bg-[#07c05f] text-white' + (index > 0 ? ' border-l-[#07c05f]' : '') + ' hover:bg-[#06b04d]'
                      : ' hover:text-[#07c05f]' + (index > 0 ? ' hover:border-l-[#07c05f]' : ''))
                  }
                  onClick={() => handleFontSizeChange(size)}
                >
                  {t('font.size.' + size)}
                </button>
              ))}
            </div>
          </div>
        </div>
        {liteMode ? <div className="setting-row">
          <div className="setting-info">
            <label>{autoUpdateCopy.label}</label>
            <p className="desc">{autoUpdateCopy.description}</p>
          </div>
          <div className="setting-control">
            <Switch checked={autoCheckUpdate} onCheckedChange={handleAutoCheckUpdateChange} aria-label={autoUpdateCopy.label} />
          </div>
        </div> : null}
      </div>
    </div>
  );
}
