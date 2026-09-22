/**
 * R486 — embedded chat-attachment parser rules editor, a port of the Vue
 * <KBParserSettings embedded> block rendered inside the agent editor's
 * attachment section (frontend/src/views/agent/AgentEditorModal.vue:869-878 →
 * frontend/src/views/knowledge/settings/KBParserSettings.vue:17-65).
 *
 * One setting-row per file-type family: group label + extension tags on the
 * left, the engine t-select (default engine suffixed 「(默认)」) plus the xlsx
 * first-row checkbox and the go-config link on the right. The rule math
 * (default engine resolution, complete-rules materialisation) is reused
 * verbatim from apps/web/src/knowledge-settings/parserSettings.tsx (imported
 * read-only), which itself ports the Vue component — a single source of truth
 * for both surfaces.
 *
 * TDesign 同构迁移（Task 9 fix round 1）：engine 选择换 tdesign-react Select，
 * DOM/类名对齐 KBParserSettings.vue embedded 态（kb-parser-settings--embedded /
 * settings-group--embedded / setting-row / group-label / ext-tag /
 * parser-control-stack / parser-engine-select--embedded / xlsx-header-option /
 * no-engine-warning），样式平移在 agents.td.css。tdesign Select 根元素丢弃
 * data-*（台账 #8）——行钩子保留 data-parser-row，select 钩子走语义
 * className（wk-ae-parser-engine-<group>）。
 */
import { Checkbox, Select } from 'tdesign-react';
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
 * extensions intersect the relevant set; when nothing matches, fall back to
 * the full catalogue rather than an empty editor.
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
    return (
      <div className="kb-parser-settings kb-parser-settings--embedded">
        <div className="empty-hint">
          <p data-parser-empty>{t('kbSettings.parser.noEngineAvailable')}</p>
        </div>
      </div>
    );
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
    <div className="kb-parser-settings kb-parser-settings--embedded" data-parser-groups={groups.length}>
      <div className="settings-group settings-group--embedded">
        {groups.map((group) => {
          const options = parserEngineOptionsFor(engines, group.extensions);
          const resolved = engineForGroup(rules, engines, group.extensions);
          const showXlsxOption = group.extensions.includes('xlsx') && resolved === 'builtin';
          return (
            <div key={group.key} className="setting-row" data-parser-row={group.key}>
              <div className="setting-info">
                <label className="group-label">{group.label}</label>
                <div className="ext-tags">
                  {group.extensions.map((ext) => (
                    <span key={ext} className="ext-tag">.{ext}</span>
                  ))}
                </div>
              </div>
              <div className="setting-control">
                <div className="parser-control-stack">
                  {/* Vue KBParserSettings.vue:34-49 的 t-select 直译（embedded 态
                      全宽；无可用引擎时 warning 态 + 下方 go-config 链接）。 */}
                  <Select
                    value={resolved || undefined}
                    onChange={(value) => handleEngineChange(group.extensions, String(value ?? ''))}
                    className={`parser-engine-select--embedded wk-ae-parser-engine-${group.key}`}
                    status={options.length === 0 ? 'warning' : 'default'}
                    placeholder={t('kbSettings.parser.noEngine')}
                    popupProps={{ overlayInnerStyle: { maxHeight: '240px' } } as never}
                  >
                    {options.map((option) => (
                      <Select.Option key={option.value} value={option.value} label={parserEngineOptionLabel(option.value, option.isDefault, t)} />
                    ))}
                  </Select>
                  {showXlsxOption ? (
                    <Checkbox
                      className="xlsx-header-option"
                      data-parser-xlsx={group.key}
                      checked={ruleForGroup(rules, group.extensions)?.xlsx_first_row_as_header === true}
                      onChange={(checked) => handleXlsxFirstRowChange(group.extensions, checked === true)}
                    >
                      {t('kbSettings.parser.xlsxFirstRowAsHeader')}
                    </Checkbox>
                  ) : null}
                  {options.length === 0 ? (
                    <div className="no-engine-warning">
                      {/* Vue goToParserSettings → uiStore.openSettings('parser'); the
                          React shell routes through the settings deep link like the
                          sandbox/storage links in the editor. */}
                      <a className="go-settings" data-parser-go-config onClick={(event) => { event.preventDefault(); navigate('/platform/settings?section=parser'); }}>
                        {t('kbSettings.parser.goConfig')}
                      </a>
                    </div>
                  ) : null}
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
