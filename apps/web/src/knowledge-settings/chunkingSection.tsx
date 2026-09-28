import { useEffect, useState, type ReactNode } from 'react';
import type { ChunkingPreviewResult, WeKnoraClient } from '@weknora/api-client';
import { Button as TButton, InputNumber as TInputNumber, Select as TSelect, Slider as TSlider, Switch as TSwitch } from 'tdesign-react';
import {
  CHILD_CHUNK_SIZE_RANGE,
  CHUNKING_LANGUAGE_LABEL_KEYS,
  CHUNKING_LANGUAGE_VALUES,
  CHUNKING_SEPARATOR_VALUES,
  CHUNKING_STRATEGY_VALUES,
  CHUNK_OVERLAP_RANGE,
  CHUNK_SIZE_RANGE,
  PARENT_CHUNK_SIZE_RANGE,
  TOKEN_LIMIT_RANGE,
  formatKnowledgeSettingsSeparatorLabel,
  isChunkOverlapTooHigh,
  isChunkingAdvancedDisabled,
} from './editorSections.ts';
import { CHUNKING_SAMPLES, DEFAULT_SAMPLE_ID } from './chunkingSamples.ts';
import './KnowledgeSettingsPage.css';
import './chunking.td.css';

// Vue renders these with tdesign-icons-vue-next SVGs; text glyphs would leak
// into innerText. The trigger icons moved to tdesign-icons-react (TIcon) with
// the tdesign control swap; the drawer-internal chevron/close glyphs stay
// inline (drawer 本体不进扫描面)。
// play-circle 取组件内联 d 逐属性复刻（KBChunkingDebug.vue 用 tdesign-icons-
// vue-next 的 PlayCircleIcon 组件——内联高精度 d，非 sprite；TIcon 的 sprite
// use 版 d 几何不同，环描边 AA 差 28px，px2-kb-settings-nav y226-237 实证）。
function PlayCircleGlyph() {
  return (
    <svg viewBox="0 0 24 24" width="1em" height="1em" fill="none" className="t-icon t-icon-play-circle" style={{ fill: 'none' }}>
      <g id="play-circle">
        <path fill="currentColor" d="M12 3C7.02944 3 3 7.02944 3 12C3 16.9706 7.02944 21 12 21C16.9706 21 21 16.9706 21 12C21 7.02944 16.9706 3 12 3ZM1 12C1 5.92487 5.92487 1 12 1C18.0751 1 23 5.92487 23 12C23 18.0751 18.0751 23 12 23C5.92487 23 1 18.0751 1 12Z" />
        <path fill="currentColor" d="M18.25 12L8.5 17.6292L8.5 6.37085L18.25 12Z" />
      </g>
    </svg>
  );
}
function ChevronGlyph({ open }: { open: boolean }) {
  return (
    <svg aria-hidden="true" focusable="false" viewBox="0 0 16 16" width="16" height="16" style={{ transform: open ? 'rotate(90deg)' : 'none', transition: 'transform .2s', verticalAlign: '-3px' }}>
      <path fill="currentColor" d="M4.5 3.5L9.5 8l-5 4.5V3.5z" />
    </svg>
  );
}
function CloseGlyph() {
  return (
    <svg aria-hidden="true" focusable="false" viewBox="0 0 16 16" width="12" height="12">
      <path fill="currentColor" d="M3.3 2.6l10.1 10.1-1.06 1.06L2.24 3.66 3.3 2.6z" />
      <path fill="currentColor" d="M13.4 3.66L3.3 13.77 2.24 12.7 12.34 2.6 13.4 3.66z" />
    </svg>
  );
}

// R490 extraction: the Vue-parity chunking form (KBChunkingSettings.vue +
// KBChunkingDebug.vue) previously lived inline in KnowledgeSettingsPage.tsx.
// The KB editor drawer (App.tsx) rendered a second, bare-implementation
// chunking section; this module is the single shared Vue-form authority both
// surfaces mount so the drawer matches the Vue drawer byte for byte.

export type ChunkingSectionTranslate = (key: string, values?: Record<string, string | number>) => string;

/** The chunking controls both surfaces edit (Vue ChunkingConfig camelCase form). */
export interface ChunkingSplittingControls {
  chunkSize: number;
  chunkOverlap: number;
  separators: string[];
  enableParentChild: boolean;
  parentChunkSize: number;
  childChunkSize: number;
  strategy: string;
  tokenLimit: number;
  languages: string[];
}

interface ChunkingSettingsFieldsProps {
  splitting: ChunkingSplittingControls;
  onPatch: (patch: Partial<ChunkingSplittingControls>) => void;
  client?: WeKnoraClient;
  t: ChunkingSectionTranslate;
  /** 跳过 .section-header 段（宿主自带分区标题时用；Vue 同名 prop 语义）。 */
  embedded?: boolean;
}

// Vue .setting-row layout: info column (label + desc) and control column.
// `alert` carries the conditional warning node rendered under the description
// in the info column (Vue t-alert placement, e.g. the R444 Embedding lock).
export function EditorSettingRow({ label, description, alert, required, control }: { label: string; description?: string; alert?: ReactNode; required?: boolean; control: ReactNode }) {
  return (
    <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '1.5rem', padding: '0.9rem 0', borderBottom: '1px solid #dce3ed', flexWrap: 'wrap' }}>
      <div style={{ flex: '0 1 40%', minWidth: '12rem' }}>
        <label style={{ fontWeight: 500 }}>
          {label}
          {required ? <span aria-hidden="true"> *</span> : null}
        </label>
        {description ? <p className="wk-muted" style={{ margin: '0.2rem 0 0', fontSize: '0.85rem' }}>{description}</p> : null}
        {alert}
      </div>
      <div style={{ flex: '0 1 55%', minWidth: '12rem' }}>{control}</div>
    </div>
  );
}

// Vue KBChunkingSettings（frontend/src/views/knowledge/settings/
// KBChunkingSettings.vue 非嵌入式形态）：布局类族 .kb-chunking-settings /
// .settings-group / .setting-row / .setting-info / .setting-control +
// tdesign 控件（t-select / t-slider / t-switch / t-input-number）——样式平移
// 在 chunking.td.css，控件 DOM 由 tdesign-react 生成与 Vue 同族对齐。
// `embedded`（默认 false）供宿主自带分区标题的面复用：跳过 .section-header
// 段（Vue 同名 prop 语义；Vue 端现无 embedded 消费方，其唯一挂载点
// KnowledgeBaseEditorModal.vue:270 未传该 prop）。
export function ChunkingSettingsFields({ splitting, onPatch, client, t, embedded = false }: ChunkingSettingsFieldsProps) {
  const [advancedOpen, setAdvancedOpen] = useState(false);
  // Vue watch(props.config)（KBChunkingSettings.vue:356-366）在任一控件变更
  // 引发 config 回写后重排 t-select tags——分隔符空 input 在回写后取行内
  // intrinsic 宽并折到独立行（盒高 122→145，fixture 实测）。React 的
  // TagInput 空 input 由库 JS 恒写 width:0px 不折行；以 touched 态放开
  // 宽度复刻「变更后折行」（下方 .kb-sep-relaid 规则）。
  const [sepRelaid, setSepRelaid] = useState(false);
  const set = (patch: Partial<ChunkingSplittingControls>) => {
    setSepRelaid(true);
    onPatch(patch);
  };
  // R490 #13 (KBChunkingSettings.vue:267-270): the t-slider marks render as
  // tick labels under each track — the numeric tiers were the visible Vue-only
  // diff on the chunking section (100/1000/2000/4000, 0/250/500, …).
  const CHUNK_SIZE_MARKS = [100, 1000, 2000, 4000];
  const CHUNK_OVERLAP_MARKS = [0, 250, 500];
  const PARENT_CHUNK_SIZE_MARKS = [512, 2048, 4096, 8192];
  const CHILD_CHUNK_SIZE_MARKS = [64, 384, 1024, 2048];
  const separatorOptions = CHUNKING_SEPARATOR_VALUES.map((value) => ({ value, label: formatKnowledgeSettingsSeparatorLabel(value, t) }));
  const languageOptions = CHUNKING_LANGUAGE_VALUES.map((value) => ({ value, label: t(CHUNKING_LANGUAGE_LABEL_KEYS[value]!) }));
  // Vue selectStyle/sliderStyle（KBChunkingSettings.vue:264-265）：非 embedded
  // 280px / 200px（embedded 100%——本组件 embedded 仅去 header，宽度沿用非
  // embedded 值以保持行内布局不塌）。
  const selectStyle = { width: '280px' } as const;
  const sliderStyle = { width: '200px' } as const;
  const sliderControl = (value: number, range: { min: number; max: number; step: number }, marks: number[], onChange: (next: number) => void) => (
    <div className="slider-container">
      <TSlider value={value} min={range.min} max={range.max} step={range.step} marks={marks} style={sliderStyle} onChange={(next) => onChange(Number(next))} />
      <span className="value-display">{value} {t('knowledgeEditor.chunking.characters')}</span>
    </div>
  );
  const settingRow = (labelKey: string, descKey: string, control: ReactNode, extraClass?: string, warn?: boolean, controlClass?: string) => (
    <div className={`setting-row${extraClass ? ` ${extraClass}` : ''}`}>
      <div className="setting-info">
        <label>{t(labelKey)}</label>
        <p className="desc">{t(descKey)}</p>
        {warn ? <p className="warn">{t('knowledgeEditor.chunking.overlapWarning')}</p> : null}
      </div>
      <div className={`setting-control${controlClass ? ` ${controlClass}` : ''}`}>{control}</div>
    </div>
  );
  const strategy = splitting.strategy;
  const strategyInfo = CHUNKING_STRATEGY_VALUES.includes(strategy as (typeof CHUNKING_STRATEGY_VALUES)[number])
    ? { label: t(`knowledgeEditor.chunking.strategies.${strategy}.label`), tooltip: t(`knowledgeEditor.chunking.strategies.${strategy}.tooltip`) }
    : null;
  const advancedDisabled = isChunkingAdvancedDisabled(strategy);
  return (
    <div className="kb-chunking-settings">
      {embedded ? null : (
        <div className="section-header">
          <div className="section-header-text">
            <h2>{t('knowledgeEditor.chunking.title')}</h2>
            <p className="section-description">{t('knowledgeEditor.chunking.description')}</p>
          </div>
        </div>
      )}
      <div className="settings-group">
        {/* Strategy */}
        {settingRow('knowledgeEditor.chunking.strategyLabel', 'knowledgeEditor.chunking.strategyDescription', (
          <>
            <TSelect
              value={strategy}
              options={CHUNKING_STRATEGY_VALUES.map((value) => ({ value, label: t(`knowledgeEditor.chunking.strategies.${value}.label`) }))}
              placeholder={t('knowledgeEditor.chunking.strategyPlaceholder')}
              clearable
              style={selectStyle}
              onChange={(value) => set({ strategy: String(value ?? '') })}
            />
            {/* Vue sits the test trigger right next to the strategy picker so
                users discover it exactly when they're deciding which strategy
                to use on their content. */}
            <ChunkingDebugDrawer splitting={splitting} client={client} t={t} />
          </>
        ), undefined, undefined, 'strategy-control')}
        {/* Strategy explanation panel */}
        {strategyInfo ? (
          <div className="strategy-info-panel">
            <p>
              <strong>{strategyInfo.label}:</strong> {strategyInfo.tooltip}
            </p>
          </div>
        ) : null}
        {/* Chunk Size */}
        {settingRow('knowledgeEditor.chunking.sizeLabel', 'knowledgeEditor.chunking.sizeDescription', sliderControl(splitting.chunkSize, CHUNK_SIZE_RANGE, CHUNK_SIZE_MARKS, (next) => set({ chunkSize: next })))}
        {/* Chunk Overlap */}
        {settingRow('knowledgeEditor.chunking.overlapLabel', 'knowledgeEditor.chunking.overlapDescription', sliderControl(splitting.chunkOverlap, CHUNK_OVERLAP_RANGE, CHUNK_OVERLAP_MARKS, (next) => set({ chunkOverlap: next })), undefined, isChunkOverlapTooHigh(splitting.chunkSize, splitting.chunkOverlap))}
        {/* Separators */}
        {settingRow('knowledgeEditor.chunking.separatorsLabel', 'knowledgeEditor.chunking.separatorsDescription', (
          <TSelect
            className={sepRelaid ? 'kb-sep-relaid' : undefined}
            value={splitting.separators}
            options={separatorOptions}
            multiple
            creatable
            filterable
            placeholder={t('knowledgeEditor.chunking.separatorsPlaceholder')}
            style={selectStyle}
            onChange={(value) => set({ separators: (Array.isArray(value) ? value : []).map(String) })}
          />
        ), 'setting-row--separators')}
        {/* Parent-Child Chunking */}
        <div className="setting-row setting-row--toggle">
          <div className="setting-info">
            <label>{t('knowledgeEditor.chunking.parentChildLabel')}</label>
            <p className="desc">{t('knowledgeEditor.chunking.parentChildDescription')}</p>
          </div>
          <div className="setting-control">
            {/* Vue t-switch 是无标签 div（同款 DOM 差异见台账 #2）；React 根是
                button，补 aria-label 供无障碍命名与扫描 clickAria 兜底
                （px2-kb-settings-chunkswitch 双端命中链）。 */}
            <TSwitch aria-label={t('knowledgeEditor.chunking.parentChildLabel')} value={splitting.enableParentChild} onChange={(value) => set({ enableParentChild: Boolean(value) })} />
          </div>
        </div>
        {/* Parent Chunk Size */}
        {splitting.enableParentChild ? settingRow('knowledgeEditor.chunking.parentChunkSizeLabel', 'knowledgeEditor.chunking.parentChunkSizeDescription', sliderControl(splitting.parentChunkSize, PARENT_CHUNK_SIZE_RANGE, PARENT_CHUNK_SIZE_MARKS, (next) => set({ parentChunkSize: next }))) : null}
        {/* Child Chunk Size */}
        {splitting.enableParentChild ? settingRow('knowledgeEditor.chunking.childChunkSizeLabel', 'knowledgeEditor.chunking.childChunkSizeDescription', sliderControl(splitting.childChunkSize, CHILD_CHUNK_SIZE_RANGE, CHILD_CHUNK_SIZE_MARKS, (next) => set({ childChunkSize: next }))) : null}
        {/* Advanced section toggle */}
        <button type="button" className="advanced-toggle" onClick={() => setAdvancedOpen((open) => !open)}>
          {/* Vue KBChunkingSettings.vue:165-168 ChevronRightIcon 组件内联 d
              （M9.5 17.5L15 12L9.5 6.5，square cap，宽 2）。 */}
          <svg viewBox="0 0 24 24" width="1em" height="1em" fill="none" className={`t-icon t-icon-chevron-right toggle-arrow${advancedOpen ? ' open' : ''}`} style={{ fill: 'none' }}>
            <g id="chevron-right">
              <path id="stroke1" stroke="currentColor" d="M9.5 17.5L15 12L9.5 6.5" strokeLinecap="square" strokeWidth="2" />
            </g>
          </svg>
          <span>{t('knowledgeEditor.chunking.advancedLabel')}</span>
        </button>
        {advancedOpen ? (
          <div className="advanced-section">
            {/* Token Limit */}
            <div className={`setting-row${advancedDisabled ? ' disabled' : ''}`}>
              <div className="setting-info">
                <label>{t('knowledgeEditor.chunking.tokenLimitLabel')}</label>
                <p className="desc">{t('knowledgeEditor.chunking.tokenLimitDescription')}</p>
              </div>
              <div className="setting-control">
                <TInputNumber
                  value={splitting.tokenLimit}
                  min={TOKEN_LIMIT_RANGE.min}
                  max={TOKEN_LIMIT_RANGE.max}
                  step={TOKEN_LIMIT_RANGE.step}
                  disabled={advancedDisabled}
                  style={{ width: '200px' }}
                  onChange={(value) => set({ tokenLimit: Number(value) })}
                />
              </div>
            </div>
            {/* Languages */}
            <div className={`setting-row${advancedDisabled ? ' disabled' : ''}`}>
              <div className="setting-info">
                <label>{t('knowledgeEditor.chunking.languagesLabel')}</label>
                <p className="desc">{t('knowledgeEditor.chunking.languagesDescription')}</p>
              </div>
              <div className="setting-control">
                <TSelect
                  value={splitting.languages}
                  options={languageOptions}
                  multiple
                  disabled={advancedDisabled}
                  placeholder={t('knowledgeEditor.chunking.languagesPlaceholder')}
                  style={selectStyle}
                  onChange={(value) => set({ languages: (Array.isArray(value) ? value : []).map(String) })}
                />
              </div>
            </div>
          </div>
        ) : null}
      </div>
    </div>
  );
}

// Vue KBChunkingDebug: an inline text trigger beside the strategy picker opens
// a right drawer that runs the current (draft-inclusive) chunking config over
// a sample text through POST /api/v1/chunker/preview and renders the selected
// tier, rejected tiers, doc profile, size stats and the chunk cards.
export function ChunkingDebugDrawer({ splitting, client, t }: { splitting: ChunkingSplittingControls; client?: WeKnoraClient; t: ChunkingSectionTranslate }) {
  const [open, setOpen] = useState(false);
  const [sample, setSample] = useState('');
  const [autoLoaded, setAutoLoaded] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState<ChunkingPreviewResult | null>(null);
  const [expanded, setExpanded] = useState<ReadonlySet<number>>(new Set());
  // Mirrors handler.previewMaxChars on the backend. Keep in sync.
  const MAX_CHARS = 64 * 1024;
  const loadSample = (id: string) => {
    const preset = CHUNKING_SAMPLES.find((candidate) => candidate.id === id);
    if (!preset) return;
    setSample(preset.text);
    setResult(null);
    setError('');
    setExpanded(new Set());
  };
  // Vue: opening the drawer with an empty textarea auto-loads the default
  // preset; user input on subsequent opens is never overwritten.
  useEffect(() => {
    if (open && !autoLoaded && sample.trim() === '') {
      setAutoLoaded(true);
      loadSample(DEFAULT_SAMPLE_ID);
    }
  }, [open, autoLoaded, sample]);
  if (!client) return null;
  const runPreview = () => {
    if (sample.length === 0 || loading) return;
    setLoading(true);
    setError('');
    setResult(null);
    setExpanded(new Set());
    // Send all fields explicitly (empty/0 included) so the preview reflects
    // exactly what a save would persist — the Vue buildSubmitData convention.
    void client.knowledgeBases.settings.previewChunking({
      text: sample,
      chunking_config: {
        chunk_size: splitting.chunkSize,
        chunk_overlap: splitting.chunkOverlap,
        separators: splitting.separators,
        enable_parent_child: splitting.enableParentChild,
        parent_chunk_size: splitting.parentChunkSize,
        child_chunk_size: splitting.childChunkSize,
        strategy: splitting.strategy,
        token_limit: splitting.tokenLimit,
        languages: splitting.languages,
      },
    }).then((data) => setResult(data)).catch((cause: unknown) => {
      setError(cause instanceof Error && cause.message ? cause.message : 'unknown error');
    }).finally(() => setLoading(false));
  };
  const toggleChunk = (seq: number) => {
    const next = new Set(expanded);
    if (next.has(seq)) next.delete(seq);
    else next.add(seq);
    setExpanded(next);
  };
  // `recursive` and `legacy` share the same splitter path; both surface under
  // the user-facing legacy label (Vue normalizeTier).
  const normalizeTier = (tier: string) => (tier === 'recursive' ? 'legacy' : tier);
  const tierLabel = (tier: string) => {
    const normalized = normalizeTier(tier);
    return CHUNKING_STRATEGY_VALUES.includes(normalized as (typeof CHUNKING_STRATEGY_VALUES)[number])
      ? t(`knowledgeEditor.chunking.strategies.${normalized}.label`)
      : normalized;
  };
  const fallbackWarning = result !== null && result.selected_tier === 'legacy' && result.rejected.length > 0;
  const profileCell = (labelKey: string, value: string) => (
    <div className="kb-chunking-profile-cell">
      <div className="kb-chunking-profile-value">{value}</div>
      <div className="kb-chunking-profile-label">{t(labelKey)}</div>
    </div>
  );
  const profile = result?.profile ?? null;
  const number_ = (value: unknown) => (typeof value === 'number' && Number.isFinite(value) ? value : 0);
  const chapterCount = profile
    ? number_(profile.german_chapter_count) + number_(profile.english_chapter_count) + number_(profile.chinese_chapter_count)
    : 0;
  const detectedLangs = profile && Array.isArray(profile.detected_langs) ? profile.detected_langs.filter((item): item is string => typeof item === 'string').join(', ') : '';
  return (
    <div className="kb-chunking-debug">
      {/* Vue KBChunkingDebug.vue:10-19 t-button（variant=text theme=primary
          size=medium class=debug-trigger + #icon 槽 play-circle）——icon 走
          icon prop（台账 #28 icon 槽判例），glyph 用组件内联 d 复刻
          （PlayCircleGlyph，非 TIcon sprite 版）。 */}
      <TButton type="button" theme="primary" variant="text" size="medium" className="debug-trigger" icon={<PlayCircleGlyph />} onClick={() => setOpen(true)}>
        {t('knowledgeEditor.chunking.debug.toggle')}
      </TButton>
      {open ? (
        <div className="kb-chunking-debug-layer">
          <div className="kb-chunking-debug-overlay" onClick={() => setOpen(false)} />
          <aside className="kb-chunking-drawer" role="dialog" aria-modal="true" aria-label={t('knowledgeEditor.chunking.debug.toggle')}>
            <header className="kb-chunking-drawer-header">
              <strong>{t('knowledgeEditor.chunking.debug.toggle')}</strong>
              <button type="button" aria-label={t('common.cancel')} onClick={() => setOpen(false)}><CloseGlyph /></button>
            </header>
            <div className="kb-chunking-drawer-body">
              <section className="kb-chunking-drawer-section">
                <div className="kb-chunking-sample-row">
                  <span className="kb-chunking-sample-title">{t('knowledgeEditor.chunking.debug.sampleLabel')}</span>
                  <span className="kb-chunking-presets">
                    <span>{t('knowledgeEditor.chunking.debug.presetLabel')}</span>
                    {CHUNKING_SAMPLES.map((preset) => (
                      <button type="button" key={preset.id} onClick={() => loadSample(preset.id)}>
                        {t(`knowledgeEditor.chunking.debug.${preset.labelKey}`)}
                      </button>
                    ))}
                  </span>
                </div>
                <textarea
                  aria-label={t('knowledgeEditor.chunking.debug.sampleLabel')}
                  placeholder={t('knowledgeEditor.chunking.debug.samplePlaceholder')}
                  maxLength={MAX_CHARS}
                  rows={6}
                  value={sample}
                  onChange={(event) => setSample(event.target.value)}
                />
                <div className="kb-chunking-run-row">
                  <button
                    type="button"
                    className="kb-chunking-run"
                    disabled={loading || sample.length === 0}
                    onClick={runPreview}
                  >
                    {t('knowledgeEditor.chunking.debug.runButton')}
                  </button>
                </div>
              </section>
              {loading ? (
                <p className="kb-chunking-loading" role="status">{t('knowledgeEditor.chunking.debug.loading')}</p>
              ) : error ? (
                <p className="kb-chunking-error" role="alert">
                  <strong>{t('knowledgeEditor.chunking.debug.errorPrefix')}</strong> {error}
                </p>
              ) : result ? (
                <section className="kb-chunking-result">
                  <div className="kb-chunking-tier-row">
                    <span>{t('knowledgeEditor.chunking.debug.selectedTier')}:</span>
                    <span className="kb-chunking-tier-tag" data-tier={normalizeTier(result.selected_tier)}>{tierLabel(result.selected_tier)}</span>
                    {fallbackWarning ? <span className="kb-chunking-fallback">{t('knowledgeEditor.chunking.debug.fallbackWarning')}</span> : null}
                  </div>
                  {result.rejected.length > 0 ? (
                    <div className="kb-chunking-tier-row">
                      <span>{t('knowledgeEditor.chunking.debug.rejected')}:</span>
                      {result.rejected.map((rejection, index) => {
                        const tier = typeof (rejection as { tier?: unknown })?.tier === 'string' ? (rejection as { tier: string }).tier : '';
                        const reason = typeof (rejection as { reason?: unknown })?.reason === 'string' ? (rejection as { reason: string }).reason : '';
                        return <span key={`${tier}-${index}`} className="kb-chunking-rejected-tag">{tierLabel(tier)}: {reason}</span>;
                      })}
                    </div>
                  ) : null}
                  <div className="kb-chunking-profile-grid">
                    {profileCell('knowledgeEditor.chunking.debug.profile.lines', String(number_(profile?.total_lines)))}
                    {profileCell('knowledgeEditor.chunking.debug.profile.chars', String(number_(profile?.total_chars)))}
                    {profileCell('knowledgeEditor.chunking.debug.profile.headings', String(number_(profile?.md_heading_total)))}
                    {profileCell('knowledgeEditor.chunking.debug.profile.pageBreaks', String(number_(profile?.form_feed_count)))}
                    {profileCell('knowledgeEditor.chunking.debug.profile.chapterMarkers', String(chapterCount))}
                    {profileCell('knowledgeEditor.chunking.debug.profile.languages', detectedLangs || '—')}
                  </div>
                  <div className="kb-chunking-stats">
                    <strong>{result.stats.count}</strong> {t('knowledgeEditor.chunking.debug.stats.chunks')}
                    <span>·</span>
                    <span>Ø {result.stats.avg_chars}</span>
                    <span>·</span>
                    <span>σ {result.stats.stddev_chars}</span>
                    <span>·</span>
                    <span>min {result.stats.min_chars}</span>
                    <span>·</span>
                    <span>max {result.stats.max_chars}</span>
                    {typeof result.stats.truncated_to === 'number' ? (
                      <span className="kb-chunking-truncated">{t('knowledgeEditor.chunking.debug.stats.truncated', { total: result.stats.truncated_to })}</span>
                    ) : null}
                  </div>
                  <ol className="kb-chunking-chunks">
                    {result.chunks.map((chunk) => {
                      const seq = typeof chunk.seq === 'number' ? chunk.seq : 0;
                      const isOpen = expanded.has(seq);
                      return (
                        <li key={seq} className={isOpen ? 'kb-chunking-chunk expanded' : 'kb-chunking-chunk'}>
                          <button
                            type="button"
                            className="kb-chunking-chunk-meta"
                            aria-expanded={isOpen}
                            onClick={() => toggleChunk(seq)}
                          >
                            <span className="kb-chunking-chunk-seq">#{seq}</span>
                            <span>{number_(chunk.size_chars)} {t('knowledgeEditor.chunking.characters')}</span>
                            <span>· ~{number_(chunk.size_tokens_approx)} tok</span>
                            <span>{number_(chunk.start)}–{number_(chunk.end)}</span>
                            {typeof chunk.context_header === 'string' && chunk.context_header ? (
                              <span className="kb-chunking-context-pill" title={chunk.context_header}>{chunk.context_header}</span>
                            ) : null}
                            <span className="kb-chunking-chevron"><ChevronGlyph open={isOpen} /></span>
                          </button>
                          <pre className={isOpen ? 'kb-chunking-chunk-text' : 'kb-chunking-chunk-text collapsed'}>
                            {typeof chunk.content === 'string' ? chunk.content : ''}
                          </pre>
                        </li>
                      );
                    })}
                  </ol>
                </section>
              ) : null}
            </div>
          </aside>
        </div>
      ) : null}
    </div>
  );
}
