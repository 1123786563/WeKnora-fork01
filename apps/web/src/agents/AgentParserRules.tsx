/**
 * R486 — embedded chat-attachment parser rules editor, a port of the Vue
 * <KBParserSettings embedded> block rendered inside the agent editor's
 * attachment section (frontend/src/views/agent/AgentEditorModal.vue:869-878 →
 * frontend/src/views/knowledge/settings/KBParserSettings.vue).
 *
 * One row per file-type family: label + extension tags on the left, the engine
 * select (default engine suffixed 「(默认)」) plus the xlsx first-row checkbox
 * and the go-config warning on the right. The rule math (default engine
 * resolution, complete-rules materialisation) is reused verbatim from
 * apps/web/src/knowledge-settings/parserSettings.tsx (imported read-only),
 * which itself ports the Vue component — a single source of truth for both
 * surfaces.
 */
import { Checkbox } from '@weknora/ui';
import { navigate } from '../platform/navigation.ts';
import {
  buildCompleteParserRules,
  buildParserFileTypeGroups,
  engineForGroup,
  allParserFileTypes,
  parserEngineOptionLabel,
  parserEngineOptionsFor,
  ruleForGroup,
  type ParserEngineInfo,
  type ParserEngineRule,
  type ParserFileTypeGroup,
} from '../knowledge-settings/parserSettings.tsx';

/**
 * Vue AgentEditorModal.vue:1884-1888 CHAT_PARSER_EXTENSIONS — the attachment
 * families the chat pipeline actually parses; audio/video rows stay in the
 * knowledge-settings surface only.
 */
export const CHAT_PARSER_EXTENSIONS = [
  'pdf', 'doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx', 'epub', 'mhtml',
  'txt', 'md', 'markdown', 'csv', 'json', 'xml', 'html', 'yaml', 'yml', 'log',
  'jpg', 'jpeg', 'png', 'gif', 'bmp', 'tiff', 'webp',
];

/**
 * Vue fileTypeGroups tail (KBParserSettings.vue:181-185): keep the groups whose
 * extensions intersect the relevant set; when nothing matches, fall back to the
 * full catalogue rather than an empty editor.
 */
export function filterParserGroupsByExtensions(groups: ParserFileTypeGroup[], relevantExtensions: readonly string[]): ParserFileTypeGroup[] {
  if (relevantExtensions.length === 0) return groups;
  const relevant = new Set(relevantExtensions);
  const filtered = groups.filter((group) => group.extensions.some((ext) => relevant.has(ext)));
  return filtered.length > 0 ? filtered : groups;
}

/** Groups for the chat-attachment families, in catalogue order. */
export function chatParserGroups(t: (key: string) => string, engines: ParserEngineInfo[]): ParserFileTypeGroup[] {
  return filterParserGroupsByExtensions(buildParserFileTypeGroups(t, allParserFileTypes(engines)), CHAT_PARSER_EXTENSIONS);
}

/**
 * Vue ensureCompleteRules (KBParserSettings.vue:317-324): once the engine
 * registry is loaded, materialise a rule per group so the stored config covers
 * every family (defaults included) instead of only user-touched rows.
 */
export function ensureCompleteParserRules(rules: ParserEngineRule[], groups: ParserFileTypeGroup[], engines: ParserEngineInfo[]): ParserEngineRule[] {
  if (engines.length === 0) return rules;
  const complete = buildCompleteParserRules(groups, rules, engines);
  return complete.length > rules.length ? complete : rules;
}

export interface AgentParserRulesProps {
  engines: ParserEngineInfo[];
  rules: ParserEngineRule[];
  onChange: (rules: ParserEngineRule[]) => void;
  t: (key: string) => string;
}

export function AgentParserRules({ engines, rules, onChange, t }: AgentParserRulesProps) {
  if (engines.length === 0) {
    // KBParserSettings.vue:13-15 — registry empty / unreachable: degrade to
    // the shared no-engine hint instead of blanking the block.
    return <p className="m-0 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]" data-parser-empty>{t('kbSettings.parser.noEngineAvailable')}</p>;
  }
  const groups = chatParserGroups(t, engines);

  // Vue handleEngineChange (KBParserSettings.vue:244-260): drop the rules that
  // cover this family, push the fresh selection, then emit buildCompleteRules()
  // so every family persists a rule.
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

  // Vue handleXLSXFirstRowAsHeaderChange (272-280): the flag rides on the
  // family's complete rule.
  const handleXlsxFirstRowChange = (extensions: string[], checked: boolean) => {
    const complete = buildCompleteParserRules(groups, rules, engines);
    const rule = complete.find((item) => item.file_types.some((fileType) => extensions.includes(fileType)));
    if (!rule) return;
    rule.xlsx_first_row_as_header = checked;
    onChange(complete);
  };

  return (
    <div className="overflow-hidden rounded-lg border border-[var(--td-component-stroke,#e7e7e7)]" data-parser-groups={groups.length}>
      {groups.map((group) => {
        const options = parserEngineOptionsFor(engines, group.extensions);
        const resolved = engineForGroup(rules, engines, group.extensions);
        const showXlsxOption = group.extensions.includes('xlsx') && resolved === 'builtin';
        return (
          <div key={group.key} className="flex flex-wrap items-center gap-3 border-b border-[var(--td-component-stroke,#e7e7e7)] bg-[var(--td-bg-color-container,#fff)] px-3 py-2 last:border-b-0" data-parser-row={group.key}>
            <div className="w-[168px] shrink-0">
              <p className="m-0 text-[13px] font-medium">{group.label}</p>
              <div className="mt-[2px] flex flex-wrap gap-1">
                {group.extensions.map((ext) => (
                  <code key={ext} className="rounded bg-[var(--td-bg-color-secondarycontainer,#f2f3f5)] px-1.5 py-[2px] font-[family-name:monospace] text-[11px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">.{ext}</code>
                ))}
              </div>
            </div>
            <div className="flex min-w-[180px] flex-1 flex-col gap-1.5">
              <select
                aria-label={group.label}
                className="h-8 w-full cursor-pointer rounded-md border border-[var(--td-component-stroke,#dcdcdc)] bg-[var(--td-bg-color-container,#fff)] px-2 text-[13px]"
                value={resolved}
                data-parser-group={group.key}
                data-parser-engine={group.key}
                disabled={options.length === 0}
                onChange={(event) => handleEngineChange(group.extensions, event.target.value)}
              >
                {options.length === 0 ? <option value="">{t('kbSettings.parser.noEngine')}</option> : null}
                {options.map((option) => (
                  <option key={option.value} value={option.value}>{parserEngineOptionLabel(option.value, option.isDefault, t)}</option>
                ))}
              </select>
              {showXlsxOption ? (
                <label className="inline-flex cursor-pointer items-start gap-1.5 text-left text-[12px] leading-[1.5]">
                  <Checkbox
                    checked={ruleForGroup(rules, group.extensions)?.xlsx_first_row_as_header === true}
                    onChange={(event) => handleXlsxFirstRowChange(group.extensions, event.target.checked)}
                  />
                  <span>{t('kbSettings.parser.xlsxFirstRowAsHeader')}</span>
                </label>
              ) : null}
              {options.length === 0 ? (
                <button
                  type="button"
                  data-parser-go-config
                  className="w-fit cursor-pointer border-0 bg-transparent p-0 text-[12px] text-[var(--td-brand-color,#0052d9)] hover:underline"
                  // Vue goToParserSettings → uiStore.openSettings('parser'); the
                  // React shell routes through the settings deep link like the
                  // sandbox/storage links in the editor.
                  onClick={() => navigate('/platform/settings?section=parser')}
                >
                  {t('kbSettings.parser.goConfig')}
                </button>
              ) : null}
            </div>
          </div>
        );
      })}
    </div>
  );
}
