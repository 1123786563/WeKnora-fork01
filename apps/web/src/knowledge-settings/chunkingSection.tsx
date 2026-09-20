import { useEffect, useState, type ReactNode } from 'react';
import type { ChunkingPreviewResult, WeKnoraClient } from '@weknora/api-client';
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

// Vue KBChunkingSettings: strategy select with the debug-drawer trigger beside
// it, size/overlap sliders with the overlap warning, the separator chips field,
// parent-child sliders, and a collapsed token/language panel.
export function ChunkingSettingsFields({ splitting, onPatch, client, t }: ChunkingSettingsFieldsProps) {
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const set = onPatch;
  // R490 #13 (KBChunkingSettings.vue:267-270): the t-slider marks render as
  // tick labels under each track — the numeric tiers were the visible Vue-only
  // diff on the chunking section (100/1000/2000/4000, 0/250/500, …).
  const CHUNK_SIZE_MARKS = [100, 1000, 2000, 4000];
  const CHUNK_OVERLAP_MARKS = [0, 250, 500];
  const PARENT_CHUNK_SIZE_MARKS = [512, 2048, 4096, 8192];
  const CHILD_CHUNK_SIZE_MARKS = [64, 384, 1024, 2048];
  const marksRow = (marks: number[]) => (
    <div data-slider-marks="" style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.75rem', color: '#6b7280', lineHeight: 1.4 }}>
      {marks.map((mark) => <span key={mark}>{mark}</span>)}
    </div>
  );
  const slider = (labelKey: string, value: number, range: { min: number; max: number; step: number }, onChange: (next: number) => void, disabled?: boolean, marks?: number[]) => (
    <div style={{ display: 'grid', gap: '0.25rem' }}>
      <input
        type="range"
        aria-label={t(labelKey)}
        min={range.min}
        max={range.max}
        step={range.step}
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(Number(event.target.value))}
      />
      {marks ? marksRow(marks) : null}
      <span style={{ fontWeight: 500 }}>{value} {t('knowledgeEditor.chunking.characters')}</span>
    </div>
  );
  const multiSelect = (labelKey: string, values: string[], options: Array<{ value: string; label: string }>, onChange: (next: string[]) => void, disabled?: boolean) => (
    <select
      multiple
      aria-label={t(labelKey)}
      value={values}
      disabled={disabled}
      onChange={(event) => onChange([...(event.target as HTMLSelectElement).selectedOptions].map((option) => option.value))}
    >
      {options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
    </select>
  );
  const strategy = splitting.strategy;
  const strategyInfo = CHUNKING_STRATEGY_VALUES.includes(strategy as (typeof CHUNKING_STRATEGY_VALUES)[number])
    ? { label: t(`knowledgeEditor.chunking.strategies.${strategy}.label`), tooltip: t(`knowledgeEditor.chunking.strategies.${strategy}.tooltip`) }
    : null;
  return (
    <div>
      <EditorSettingRow
        label={t('knowledgeEditor.chunking.strategyLabel')}
        description={t('knowledgeEditor.chunking.strategyDescription')}
        control={(
          <div style={{ display: 'grid', gap: '0.4rem', justifyItems: 'start' }}>
            <select
              aria-label={t('knowledgeEditor.chunking.strategyLabel')}
              value={strategy}
              onChange={(event) => set({ strategy: event.target.value })}
            >
              {/* Vue wk-select shows a placeholder for the not-set strategy
                  (KBChunkingSettings.vue); a bare empty option rendered blank. */}
              <option value="">{t('knowledgeEditor.chunking.strategyPlaceholder')}</option>
              {CHUNKING_STRATEGY_VALUES.map((value) => <option key={value} value={value}>{t(`knowledgeEditor.chunking.strategies.${value}.label`)}</option>)}
            </select>
            {/* Vue sits the test trigger next to the strategy picker so users
                discover it exactly when choosing a strategy. */}
            <ChunkingDebugDrawer splitting={splitting} client={client} t={t} />
          </div>
        )}
      />
      {strategyInfo ? (
        <p className="wk-muted" style={{ margin: '0 0 0.6rem', borderLeft: '3px solid #07c05f', paddingLeft: '0.6rem' }}>
          <strong>{strategyInfo.label}:</strong> {strategyInfo.tooltip}
        </p>
      ) : null}
      <EditorSettingRow
        label={t('knowledgeEditor.chunking.sizeLabel')}
        description={t('knowledgeEditor.chunking.sizeDescription')}
        control={slider('knowledgeEditor.chunking.sizeLabel', splitting.chunkSize, CHUNK_SIZE_RANGE, (next) => set({ chunkSize: next }), undefined, CHUNK_SIZE_MARKS)}
      />
      <EditorSettingRow
        label={t('knowledgeEditor.chunking.overlapLabel')}
        description={t('knowledgeEditor.chunking.overlapDescription')}
        control={slider('knowledgeEditor.chunking.overlapLabel', splitting.chunkOverlap, CHUNK_OVERLAP_RANGE, (next) => set({ chunkOverlap: next }), undefined, CHUNK_OVERLAP_MARKS)}
      />
      {isChunkOverlapTooHigh(splitting.chunkSize, splitting.chunkOverlap) ? (
        <p role="status" style={{ margin: 0, color: '#b54708', fontSize: '0.85rem' }}>{t('knowledgeEditor.chunking.overlapWarning')}</p>
      ) : null}
      <EditorSettingRow
        label={t('knowledgeEditor.chunking.separatorsLabel')}
        description={t('knowledgeEditor.chunking.separatorsDescription')}
        control={(
          <SeparatorChipsInput
            values={splitting.separators}
            t={t}
            onChange={(next) => set({ separators: next })}
          />
        )}
      />
      <EditorSettingRow
        label={t('knowledgeEditor.chunking.parentChildLabel')}
        description={t('knowledgeEditor.chunking.parentChildDescription')}
        control={(
          <input
            type="checkbox"
            aria-label={t('knowledgeEditor.chunking.parentChildLabel')}
            checked={splitting.enableParentChild}
            onChange={(event) => set({ enableParentChild: event.target.checked })}
          />
        )}
      />
      {splitting.enableParentChild ? (
        <>
          <EditorSettingRow
            label={t('knowledgeEditor.chunking.parentChunkSizeLabel')}
            description={t('knowledgeEditor.chunking.parentChunkSizeDescription')}
            control={slider('knowledgeEditor.chunking.parentChunkSizeLabel', splitting.parentChunkSize, PARENT_CHUNK_SIZE_RANGE, (next) => set({ parentChunkSize: next }), undefined, PARENT_CHUNK_SIZE_MARKS)}
          />
          <EditorSettingRow
            label={t('knowledgeEditor.chunking.childChunkSizeLabel')}
            description={t('knowledgeEditor.chunking.childChunkSizeDescription')}
            control={slider('knowledgeEditor.chunking.childChunkSizeLabel', splitting.childChunkSize, CHILD_CHUNK_SIZE_RANGE, (next) => set({ childChunkSize: next }), undefined, CHILD_CHUNK_SIZE_MARKS)}
          />
        </>
      ) : null}
      <button type="button" style={{ background: 'transparent', border: 'none', padding: '0.6rem 0', cursor: 'pointer', fontWeight: 500, color: 'inherit' }} onClick={() => setAdvancedOpen((open) => !open)}>
        {advancedOpen ? '▾' : '▸'} {t('knowledgeEditor.chunking.advancedLabel')}
      </button>
      {advancedOpen ? (
        <div>
          <EditorSettingRow
            label={t('knowledgeEditor.chunking.tokenLimitLabel')}
            description={t('knowledgeEditor.chunking.tokenLimitDescription')}
            control={(
              <input
                type="number"
                aria-label={t('knowledgeEditor.chunking.tokenLimitLabel')}
                min={TOKEN_LIMIT_RANGE.min}
                max={TOKEN_LIMIT_RANGE.max}
                step={TOKEN_LIMIT_RANGE.step}
                value={splitting.tokenLimit}
                disabled={isChunkingAdvancedDisabled(strategy)}
                onChange={(event) => set({ tokenLimit: Number(event.target.value) })}
              />
            )}
          />
          <EditorSettingRow
            label={t('knowledgeEditor.chunking.languagesLabel')}
            description={t('knowledgeEditor.chunking.languagesDescription')}
            control={multiSelect(
              'knowledgeEditor.chunking.languagesLabel',
              splitting.languages,
              CHUNKING_LANGUAGE_VALUES.map((value) => ({ value, label: t(CHUNKING_LANGUAGE_LABEL_KEYS[value]!) })),
              (next) => set({ languages: next }),
              isChunkingAdvancedDisabled(strategy),
            )}
          />
        </div>
      ) : null}
    </div>
  );
}

// Vue separator control (KBChunkingSettings.vue): a multiple + creatable +
// filterable t-select. Selected values render as removable tag chips, the
// input adds custom separators (Enter commits), the dropdown offers the preset
// values filtered by the draft, Backspace on an empty input pops the last chip
// and Esc clears the pending draft without committing it.
export function SeparatorChipsInput({ values, onChange, t }: { values: string[]; onChange: (next: string[]) => void; t: ChunkingSectionTranslate }) {
  const [draft, setDraft] = useState('');
  const [open, setOpen] = useState(false);
  const label = (value: string) => formatKnowledgeSettingsSeparatorLabel(value, t);
  const addValue = (value: string) => {
    if (value === '' || values.includes(value)) return;
    onChange([...values, value]);
  };
  const removeValue = (value: string) => onChange(values.filter((item) => item !== value));
  const commitDraft = () => {
    addValue(draft.trim());
    setDraft('');
  };
  const pending = CHUNKING_SEPARATOR_VALUES.filter((value) =>
    !values.includes(value)
    && (draft === '' || value.includes(draft) || label(value).toLowerCase().includes(draft.toLowerCase())));
  return (
    <div className="kb-separator-field">
      <div className="kb-separator-box">
        {values.map((value) => (
          <span key={value} className="kb-separator-chip" data-separator-chip="">
            {label(value)}
            <button
              type="button"
              className="kb-separator-chip-remove"
              aria-label={`${t('common.remove')}: ${label(value)}`}
              onClick={() => removeValue(value)}
            >
              ×
            </button>
          </span>
        ))}
        <input
          aria-label={t('knowledgeEditor.chunking.separatorsLabel')}
          placeholder={t('knowledgeEditor.chunking.separatorsPlaceholder')}
          value={draft}
          onChange={(event) => { setDraft(event.target.value); setOpen(true); }}
          onFocus={() => setOpen(true)}
          onBlur={() => setOpen(false)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              event.preventDefault();
              commitDraft();
            } else if (event.key === 'Backspace' && draft === '' && values.length > 0) {
              removeValue(values[values.length - 1]!);
            } else if (event.key === 'Escape') {
              setDraft('');
              setOpen(false);
            }
          }}
        />
      </div>
      {open && pending.length > 0 ? (
        <div className="kb-separator-options" role="listbox" aria-label={t('knowledgeEditor.chunking.separatorsLabel')}>
          {pending.map((value) => (
            <button
              type="button"
              key={value}
              role="option"
              aria-selected="false"
              data-separator-option=""
              onMouseDown={(event) => { event.preventDefault(); addValue(value); }}
            >
              {label(value)}
            </button>
          ))}
        </div>
      ) : null}
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
      <button type="button" className="kb-chunking-debug-trigger" onClick={() => setOpen(true)}>
        ▶ {t('knowledgeEditor.chunking.debug.toggle')}
      </button>
      {open ? (
        <div className="kb-chunking-debug-layer">
          <div className="kb-chunking-debug-overlay" onClick={() => setOpen(false)} />
          <aside className="kb-chunking-drawer" role="dialog" aria-modal="true" aria-label={t('knowledgeEditor.chunking.debug.toggle')}>
            <header className="kb-chunking-drawer-header">
              <strong>{t('knowledgeEditor.chunking.debug.toggle')}</strong>
              <button type="button" aria-label={t('common.cancel')} onClick={() => setOpen(false)}>×</button>
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
                            <span className="kb-chunking-chevron">{isOpen ? '▾' : '▸'}</span>
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
