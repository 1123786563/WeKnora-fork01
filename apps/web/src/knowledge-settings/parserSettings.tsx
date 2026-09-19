// Vue KBParserSettings port (frontend/src/views/knowledge/settings/
// KBParserSettings.vue) — the parser tab renders one row per file-type
// family (label + extension tags + engine select whose default engine is
// suffixed 「(默认)」), the xlsx first-row checkbox while the Excel family
// resolves to builtin, and buildCompleteRules() persisting one rule per
// group. R482 B1 差异4/R484 G1: replaces the React aggregated single-select.

export interface ParserEngineInfo {
  Name: string;
  Description?: string;
  FileTypes?: string[];
  Available?: boolean;
}

export interface ParserEngineRule {
  file_types: string[];
  engine: string;
  xlsx_first_row_as_header?: boolean;
}

export interface ParserFileTypeGroup {
  key: string;
  /** i18n key for the known families; null for the fixed/dynamic labels. */
  labelKey: string | null;
  /** Rendered label (Vue t(labelKey) / 'Markdown' / uppercased extension). */
  label: string;
  extensions: string[];
}

export interface ParserEngineOption {
  value: string;
  isDefault: boolean;
}

// Vue getEngineDisplayName: kbSettings.parser.engines.<name>.name with the
// raw name as fallback when the locale carries no display name.
export function parserEngineDisplayName(name: string, t: (key: string) => string): string {
  const key = `kbSettings.parser.engines.${name}.name`;
  const translated = t(key);
  return translated === key ? name : translated;
}

// Vue buildOptionLabel: `${label} (${t('kbSettings.parser.default')})` for
// the resolved default engine.
export function parserEngineOptionLabel(name: string, isDefault: boolean, t: (key: string) => string): string {
  const label = parserEngineDisplayName(name, t);
  return isDefault ? `${label} (${t('kbSettings.parser.default')})` : label;
}

// Vue fileTypeGroups: known families filtered by the catalogue union first,
// then one compact dynamic row per ungrouped extension (url excluded).
export function buildParserFileTypeGroups(
  t: (key: string) => string,
  fileTypes: ReadonlySet<string>,
): ParserFileTypeGroup[] {
  const ft = fileTypes;
  const groups: ParserFileTypeGroup[] = [];
  const push = (key: string, labelKey: string | null, label: string, extensions: string[]) => {
    if (extensions.length > 0) groups.push({ key, labelKey, label, extensions });
  };
  push('pdf', 'kbSettings.parser.fileTypePdf', t('kbSettings.parser.fileTypePdf'), ['pdf'].filter((e) => ft.has(e)));
  push('office', 'kbSettings.parser.fileTypeWord', t('kbSettings.parser.fileTypeWord'), ['docx', 'doc'].filter((e) => ft.has(e)));
  push('ppt', 'kbSettings.parser.fileTypePpt', t('kbSettings.parser.fileTypePpt'), ['pptx', 'ppt'].filter((e) => ft.has(e)));
  push('excel', 'kbSettings.parser.fileTypeExcel', t('kbSettings.parser.fileTypeExcel'), ['xlsx', 'xls'].filter((e) => ft.has(e)));
  push('ebook', 'kbSettings.parser.fileTypeEbook', t('kbSettings.parser.fileTypeEbook'), ['epub'].filter((e) => ft.has(e)));
  push('webarchive', 'kbSettings.parser.fileTypeWebArchive', t('kbSettings.parser.fileTypeWebArchive'), ['mhtml'].filter((e) => ft.has(e)));
  push('csv', 'kbSettings.parser.fileTypeCsv', t('kbSettings.parser.fileTypeCsv'), ['csv'].filter((e) => ft.has(e)));
  push('markdown', null, 'Markdown', ['md', 'markdown'].filter((e) => ft.has(e)));
  push('text', 'kbSettings.parser.fileTypeText', t('kbSettings.parser.fileTypeText'), ['txt'].filter((e) => ft.has(e)));
  push('json', 'kbSettings.parser.fileTypeJson', t('kbSettings.parser.fileTypeJson'), ['json'].filter((e) => ft.has(e)));
  push('image', 'kbSettings.parser.fileTypeImage', t('kbSettings.parser.fileTypeImage'), ['jpg', 'jpeg', 'png', 'gif', 'bmp', 'tiff', 'webp'].filter((e) => ft.has(e)));
  push('audiovisual', 'kbSettings.parser.fileTypeAudiovisual', t('kbSettings.parser.fileTypeAudiovisual'), ['mp3', 'wav', 'm4a', 'flac', 'ogg'].filter((e) => ft.has(e)));
  // Keep the UI driven by the backend registry (Vue comment): unknown
  // extensions render as uppercased compact rows so new plugins surface
  // without a frontend release.
  const grouped = new Set(groups.flatMap((group) => group.extensions));
  for (const ext of [...ft].filter((ext) => !grouped.has(ext) && ext !== 'url').sort()) {
    groups.push({ key: `dynamic-${ext}`, labelKey: null, label: ext.toUpperCase(), extensions: [ext] });
  }
  return groups;
}

// Vue pickDefaultEngineName: anydoc wins for non-simple families; otherwise
// the first available supporting engine.
export function pickDefaultParserEngineName(
  engines: Array<{ name: string; available: boolean }>,
  extensions: string[],
): string {
  const available = engines.filter((engine) => engine.available);
  const simpleExts = new Set(['md', 'markdown', 'txt', 'csv', 'json']);
  const allSimple = extensions.length > 0 && extensions.every((ext) => simpleExts.has(ext));
  if (!allSimple) {
    const anydoc = available.find((engine) => engine.name === 'anydoc');
    if (anydoc) return anydoc.name;
  }
  return available[0]?.name ?? '';
}

// Vue getEngineOptions: supporting engines filtered to available ones, each
// carrying the default flag for the label suffix.
export function parserEngineOptionsFor(engines: ParserEngineInfo[], extensions: string[]): ParserEngineOption[] {
  const raw = engines
    .filter((engine) => extensions.some((ext) => (engine.FileTypes ?? []).includes(ext)))
    .map((engine) => ({ name: engine.Name, available: engine.Available !== false }));
  const defaultName = pickDefaultParserEngineName(raw, extensions);
  return raw
    .filter((engine) => engine.available)
    .map((engine) => ({ value: engine.name, isDefault: defaultName !== '' && engine.name === defaultName }));
}

export function ruleForGroup(rules: ParserEngineRule[], extensions: string[]): ParserEngineRule | undefined {
  return rules.find((rule) => rule.file_types.some((fileType) => extensions.includes(fileType)));
}

// Vue getEngineForGroup: the explicit rule wins, otherwise the resolved
// default engine.
export function engineForGroup(rules: ParserEngineRule[], engines: ParserEngineInfo[], extensions: string[]): string {
  const rule = ruleForGroup(rules, extensions);
  if (rule) return rule.engine;
  const options = parserEngineOptionsFor(engines, extensions);
  return options.find((option) => option.isDefault)?.value ?? '';
}

// Vue buildCompleteRules: one rule per group with a resolved engine — the
// committed rule keeps its xlsx flag, everyone else materialises defaults.
export function buildCompleteParserRules(groups: ParserFileTypeGroup[], rules: ParserEngineRule[], engines: ParserEngineInfo[]): ParserEngineRule[] {
  const complete: ParserEngineRule[] = [];
  for (const group of groups) {
    const engine = engineForGroup(rules, engines, group.extensions);
    if (!engine) continue;
    const current = ruleForGroup(rules, group.extensions);
    complete.push({
      file_types: [...group.extensions],
      engine,
      ...(current?.xlsx_first_row_as_header !== undefined
        ? { xlsx_first_row_as_header: current.xlsx_first_row_as_header }
        : {}),
    });
  }
  return complete;
}

export function allParserFileTypes(engines: ParserEngineInfo[]): Set<string> {
  const fileTypes = new Set<string>();
  for (const engine of engines) for (const fileType of engine.FileTypes ?? []) fileTypes.add(fileType);
  return fileTypes;
}

interface ParserSettingsSectionProps {
  engines: ParserEngineInfo[];
  loading: boolean;
  error?: string | null;
  rules: ParserEngineRule[];
  onChange: (rules: ParserEngineRule[]) => void;
  t: (key: string) => string;
}

// The Vue settings-group rows: label + extension tags on the left, the
// engine select (plus the xlsx option / go-config warning) on the right.
export function ParserSettingsSection({ engines, loading, error, rules, onChange, t }: ParserSettingsSectionProps) {  if (loading) {
    return <p className="wk-muted" style={{ margin: 0 }} data-parser-loading="">{t('kbSettings.parser.loading')}</p>;
  }
  const groups = buildParserFileTypeGroups(t, allParserFileTypes(engines));
  if (groups.length === 0) {
    return <p className="wk-muted" style={{ margin: 0 }} data-parser-empty="">{t('kbSettings.parser.noEngineAvailable')}</p>;
  }

  // Vue handleEngineChange: drop the rules covering this family, push the
  // fresh selection, then emit buildCompleteRules() so every group persists.
  const handleEngineChange = (extensions: string[], engine: string) => {
    const current = ruleForGroup(rules, extensions);
    const others = rules.filter((rule) => !rule.file_types.some((fileType) => extensions.includes(fileType)));
    const next = engine
      ? [
        ...others,
        {
          file_types: [...extensions],
          engine,
          ...(current?.xlsx_first_row_as_header !== undefined
            ? { xlsx_first_row_as_header: current.xlsx_first_row_as_header }
            : {}),
        },
      ]
      : others;
    onChange(buildCompleteParserRules(groups, next, engines));
  };

  // Vue handleXLSXFirstRowAsHeaderChange: the flag rides on the family's
  // complete rule.
  const handleXlsxFirstRowChange = (extensions: string[], checked: boolean) => {
    const complete = buildCompleteParserRules(groups, rules, engines);
    const rule = complete.find((item) => item.file_types.some((fileType) => extensions.includes(fileType)));
    if (!rule) return;
    rule.xlsx_first_row_as_header = checked;
    onChange(complete);
  };

  return (
    <div className="wkbs-parser-settings" data-parser-groups={groups.length} style={{ display: 'grid', gap: 0 }}>
      {groups.map((group) => {
        const options = parserEngineOptionsFor(engines, group.extensions);
        const resolved = engineForGroup(rules, engines, group.extensions);
        const showXlsxOption = group.extensions.includes('xlsx') && resolved === 'builtin';
        return (
          <div
            key={group.key}
            className="wkbs-parser-row"
            style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '1.5rem', padding: '0.9rem 0', borderBottom: '1px solid #dce3ed', flexWrap: 'wrap' }}
          >
            <div style={{ flex: '0 1 40%', minWidth: '12rem' }}>
              <label style={{ fontWeight: 500, display: 'block' }}>{group.label}</label>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.35rem', marginTop: '0.35rem' }}>
                {group.extensions.map((ext) => (
                  <code key={ext} style={{ fontSize: '0.75rem', background: '#f3f5f9', borderRadius: 4, padding: '0.15rem 0.45rem' }}>.{ext}</code>
                ))}
              </div>
            </div>
            <div style={{ flex: '0 1 55%', minWidth: '12rem', display: 'grid', gap: '0.5rem', justifyItems: 'stretch' }}>
              <select
                aria-label={group.label}
                value={resolved}
                data-parser-group={group.key}
                disabled={options.length === 0}
                onChange={(event) => handleEngineChange(group.extensions, event.target.value)}
              >
                {options.length === 0 ? <option value="">{t('kbSettings.parser.noEngine')}</option> : null}
                {options.map((option) => (
                  <option key={option.value} value={option.value}>{parserEngineOptionLabel(option.value, option.isDefault, t)}</option>
                ))}
              </select>
              {showXlsxOption ? (
                <label style={{ display: 'inline-flex', alignItems: 'flex-start', gap: '0.4rem', fontSize: '0.8rem' }}>
                  <input
                    type="checkbox"
                    checked={ruleForGroup(rules, group.extensions)?.xlsx_first_row_as_header === true}
                    onChange={(event) => handleXlsxFirstRowChange(group.extensions, event.target.checked)}
                  />
                  {t('kbSettings.parser.xlsxFirstRowAsHeader')}
                </label>
              ) : null}
              {options.length === 0 ? (
                <a
                  href="/settings?section=parser"
                  data-parser-go-config=""
                  style={{ fontSize: '0.8rem' }}
                >
                  {t('kbSettings.parser.goConfig')}
                </a>
              ) : null}
            </div>
          </div>
        );
      })}
      {error ? <p role="alert" style={{ margin: 0, color: '#b42318' }}>{error}</p> : null}
    </div>
  );
}
