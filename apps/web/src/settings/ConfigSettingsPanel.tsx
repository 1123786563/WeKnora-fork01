import { useEffect, useMemo, useRef, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type { Locale } from '@weknora/i18n';
import { Button as TButton, Input as TInput, Select as TSelect, Slider as TSlider, Switch as TSwitch } from 'tdesign-react';
import { WkCard as Card, WkStatus as Status } from '../shared/wk-legacy.tsx';
import { settingsConfigPatch, tenantModelIds } from './surface.ts';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';

type ConfigSection = 'retrieval' | 'parser';
type ConfigValues = Record<string, unknown>;

export interface SettingsModelOption { readonly id: string; readonly name?: string }

function object(value: unknown): ConfigValues { return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as ConfigValues : {}; }
function text(value: unknown): string { return typeof value === 'string' ? value : ''; }
function number(value: unknown, fallback: number): number { return typeof value === 'number' && Number.isFinite(value) ? value : fallback; }

const PARSER_COPY: Record<Locale, string> = {
  'zh-CN': '当前 API Key 不会从服务端返回，也不会回填到此表单。',
  'en-US': 'The current API key is never returned to or prefilled in this form.',
  'ja-JP': '現在の API Key はサーバーから返されず、このフォームにも自動入力されません。',
  'ko-KR': '현재 API Key는 서버에서 반환되지 않으며 이 양식에 자동 입력되지 않습니다.',
  'ru-RU': 'Текущий API-ключ не возвращается сервером и не подставляется в эту форму.',
};

function initialValues(section: ConfigSection, value: unknown): ConfigValues {
  const row = object(value);
  if (section === 'retrieval') return {
    // R483 F2 (R482 B2-D18): mirror RetrievalSettings.vue:148-155 — a stored
    // 0 counts as "unset" for embedding_top_k / vector_threshold /
    // keyword_threshold / rerank_top_k (cfg.x || default), while
    // rerank_threshold keeps 0 (cfg.x ?? default; the slider spans -10..10).
    embedding_top_k: number(row.embedding_top_k, 50) || 50,
    vector_threshold: number(row.vector_threshold, 0.15) || 0.15,
    keyword_threshold: number(row.keyword_threshold, 0.3) || 0.3,
    rerank_top_k: number(row.rerank_top_k, 10) || 10,
    rerank_threshold: number(row.rerank_threshold, 0.2),
    rerank_model_id: text(row.rerank_model_id),
  };
  return {
    mineru_endpoint: text(row.mineru_endpoint), mineru_api_key: '', mineru_model: text(row.mineru_model) || 'pipeline',
    mineru_vlm_server_url: text(row.mineru_vlm_server_url), mineru_enable_formula: row.mineru_enable_formula !== false,
    mineru_enable_table: row.mineru_enable_table !== false, mineru_parse_method: text(row.mineru_parse_method) || (row.mineru_enable_ocr === false ? 'txt' : 'auto'),
    mineru_language: text(row.mineru_language) || 'ch', mineru_cloud_model: text(row.mineru_cloud_model) || 'pipeline',
    mineru_cloud_enable_formula: row.mineru_cloud_enable_formula !== false, mineru_cloud_enable_table: row.mineru_cloud_enable_table !== false,
    mineru_cloud_enable_ocr: row.mineru_cloud_enable_ocr !== false, mineru_cloud_language: text(row.mineru_cloud_language) || 'ch',
    paddleocr_vl_endpoint: text(row.paddleocr_vl_endpoint), paddleocr_vl_use_seal_recognition: row.paddleocr_vl_use_seal_recognition !== false,
    paddleocr_vl_use_chart_recognition: row.paddleocr_vl_use_chart_recognition === true, paddleocr_vl_cloud_token: '',
    paddleocr_vl_cloud_model: text(row.paddleocr_vl_cloud_model) || 'PaddleOCR-VL-1.6',
    paddleocr_vl_cloud_use_seal_recognition: row.paddleocr_vl_cloud_use_seal_recognition !== false,
    paddleocr_vl_cloud_use_chart_recognition: row.paddleocr_vl_cloud_use_chart_recognition === true,
  };
}

function configApi(client: WeKnoraClient, section: ConfigSection) {
  if (section === 'retrieval') return client.settings.retrieval;
  return client.settings.parser.config;
}

function isDirty(section: ConfigSection, saved: ConfigValues, current: ConfigValues): boolean {
  return Object.keys(saved).some((key) => String(saved[key]) !== String(current[key]));
}

export function ConfigSettingsPanel({ client, section, initialValue, models, onSaved }: {
  client: WeKnoraClient;
  section: ConfigSection;
  initialValue: unknown;
  models?: readonly SettingsModelOption[];
  onSaved?: () => void;
}) {
  const api = configApi(client, section);
  const locale = readInitialLocale();
  const t = settingsT(locale);
  const parserCopy = PARSER_COPY[locale];
  const saveSuccessKey = section === 'retrieval' ? 'retrievalSettings.toasts.saveSuccess' : 'settings.parser.saveSuccess';
  const saveFailedKey = section === 'retrieval' ? 'retrievalSettings.toasts.saveFailed' : 'settings.parser.saveFailed';
  const savedValues = useMemo(() => initialValues(section, initialValue), [section, initialValue]);
  const [values, setValues] = useState<ConfigValues>(savedValues);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const savingRef = useRef(false);

  useEffect(() => { setValues(initialValues(section, initialValue)); }, [section, initialValue]);

  const modelOptions: readonly SettingsModelOption[] = models ?? [];
  const allowedModelIds = tenantModelIds(modelOptions);
  const dirty = isDirty(section, savedValues, values);

  function setValue(key: string, value: unknown) { setValues((current) => ({ ...current, [key]: value })); }

  async function saveValues(nextValues: ConfigValues) {
    if (savingRef.current) return;
    savingRef.current = true;
    setBusy(true); setError(null); setNotice(null);
    try {
      const patch = settingsConfigPatch(section, nextValues, allowedModelIds.length > 0 ? { allowedModelIds } : {});
      const saved = await api.update(patch);
      setValues((current) => ({ ...current, ...initialValues(section, saved) }));
      setNotice(t(saveSuccessKey));
      onSaved?.();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t(saveFailedKey, { message: '' }));
    } finally { savingRef.current = false; setBusy(false); }
  }

  // Vue RetrievalSettings persists changes after a 500ms debounce; retain the
  // existing submit path for parser settings only.
  useEffect(() => {
    if (section !== 'retrieval') return;
    if (!dirty || savingRef.current) return;
    if (saveTimer.current) clearTimeout(saveTimer.current);
    saveTimer.current = setTimeout(() => { saveTimer.current = null; void saveValues(values); }, 500);
    return () => { if (saveTimer.current) clearTimeout(saveTimer.current); };
  }, [section, values, dirty]);

  useEffect(() => () => { if (saveTimer.current) clearTimeout(saveTimer.current); }, []);

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setError(null); setNotice(null);
    try {
      const patch = settingsConfigPatch(section, values, allowedModelIds.length > 0 ? { allowedModelIds } : {});
      const saved = await api.update(patch);
      setValues((current) => ({ ...initialValues(section, saved), ...(section === 'parser' ? { mineru_api_key: '' } : {}) }));
      setNotice(t(saveSuccessKey));
      onSaved?.();
    } catch (reason) { setError(reason instanceof Error ? reason.message : t(saveFailedKey, { message: '' })); }
    finally { setBusy(false); }
  }

  async function testParser() {
    if (section !== 'parser') return;
    setBusy(true); setError(null); setNotice(null);
    try {
      const result = await client.settings.parser.check(settingsConfigPatch('parser', values));
      setNotice(result.connected ? t('settings.parser.checkSuccess') : t('settings.parser.checkDoneStatusUpdated'));
    } catch (reason) { setError(reason instanceof Error ? reason.message : t('settings.parser.checkFailed')); }
    finally { setBusy(false); }
  }

  const modelSelect = (key: 'rerank_model_id', disabled: boolean) => <TSelect
    className={"wk-config-sel-" + key}
    value={String(values[key] ?? '')}
    disabled={disabled}
    clearable
    options={[{ value: '', label: '—' }, ...modelOptions.map((model) => ({ value: model.id, label: model.name ? model.name + ' (' + model.id + ')' : model.id }))]}
    onChange={(value) => setValue(key, String(value))}
  />;

  const parserToggle = (key: string, label: string) => <label className="wk-config-toggle">
    <TSwitch value={values[key] === true} disabled={busy} onChange={(checked) => setValue(key, Boolean(checked))} aria-label={label} />
    {label}
  </label>;

  const slider = (key: string, min: number, max: number, step: number, label: string, format: (value: number) => string = String) => {
    const value = number(values[key], min);
    return <label className="wk-config-slider-row">
      <span className="wk-config-slider-head"><span>{label}</span><output className="wk-config-slider-value">{format(value)}</output></span>
      <TSlider min={min} max={max} step={step} value={value} disabled={busy} aria-label={label} onChange={(value) => setValue(key, Number(value))} />
    </label>;
  };

  const content = (
    <>
    <Card>{error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}{/* R490 B6 — Vue RetrievalSettings.vue:3-6 section-header: the h2 title plus
    the 配置知识库搜索和消息搜索的全局检索参数 description under it. */}
    {section === 'retrieval' ? <div className="section-header wk-config-header">
      <h2>{t('retrievalSettings.title')}</h2>
      <p className="section-description">{t('retrievalSettings.description')}</p>
    </div> : null}<form className="wk-settings-editor wk-config-editor" onSubmit={(event) => void save(event)}>{section === 'retrieval' ? <>
      {modelOptions.length > 0 ? <label className="wk-config-model-row"><span className="wk-config-model-label">{t('retrievalSettings.rerankModelLabel')} <span className="wk-config-star">*</span></span><span className="wk-config-model-desc">{t('retrievalSettings.rerankModelDescription')}</span>{!values.rerank_model_id ? <span className="wk-config-model-warn">{t('retrievalSettings.rerankModelRequired')}</span> : null}{modelSelect('rerank_model_id', busy)}</label> : null}
      {slider('embedding_top_k', 1, 100, 1, t('retrievalSettings.embeddingTopKLabel'))}
      {slider('vector_threshold', 0, 1, 0.05, t('retrievalSettings.vectorThresholdLabel'), (value) => value.toFixed(2))}
      {slider('keyword_threshold', 0, 1, 0.05, t('retrievalSettings.keywordThresholdLabel'), (value) => value.toFixed(2))}
      {slider('rerank_top_k', 1, 100, 1, t('retrievalSettings.rerankTopKLabel'))}
      {slider('rerank_threshold', -10, 10, 0.1, t('retrievalSettings.rerankThresholdLabel'), (value) => value.toFixed(2))}
      {modelOptions.length === 0 ? <label>{t('retrievalSettings.rerankModelLabel')}<TInput value={String(values.rerank_model_id)} onChange={(value) => setValue('rerank_model_id', String(value))} /></label> : null}
    </> : <>
      <section className="wk-config-section"><h3>MinerU</h3>
        <label>{t('settings.parser.selfHostedEndpoint')}<TInput data-testid="mineru-endpoint" type="url" value={String(values.mineru_endpoint)} placeholder={t('settings.parser.mineruEndpointPlaceholder')} onChange={(value) => setValue('mineru_endpoint', String(value))} /></label>
        <label>Backend<TSelect className="wk-config-sel-mineru-model" value={String(values.mineru_model)} options={[{ value: 'pipeline', label: 'pipeline' }, { value: 'vlm-auto-engine', label: 'vlm-auto-engine' }, { value: 'vlm-http-client', label: 'vlm-http-client' }, { value: 'hybrid-auto-engine', label: 'hybrid-auto-engine' }, { value: 'hybrid-http-client', label: 'hybrid-http-client' }]} onChange={(value) => setValue('mineru_model', String(value))} /></label>
        <label>vLLM {t('settings.parser.serverUrl')}<TInput data-testid="mineru-vllm-server-url" type="url" value={String(values.mineru_vlm_server_url)} placeholder={t('settings.parser.vlmServerUrlPlaceholder')} onChange={(value) => setValue('mineru_vlm_server_url', String(value))} /></label>
        <label>{t('settings.parser.parseMethodLabel')}<TSelect className="wk-config-sel-mineru-parse-method" value={String(values.mineru_parse_method)} options={[{ value: 'auto', label: t('settings.parser.parseMethodAuto') }, { value: 'ocr', label: t('settings.parser.parseMethodOCR') }, { value: 'txt', label: t('settings.parser.parseMethodText') }]} onChange={(value) => setValue('mineru_parse_method', String(value))} /></label>
        <div className="wk-config-toggles">{parserToggle('mineru_enable_formula', t('settings.parser.formulaRecognition'))}{parserToggle('mineru_enable_table', t('settings.parser.tableRecognition'))}</div>
        <label>{t('settings.parser.language')}<TInput data-testid="mineru-language" value={String(values.mineru_language)} placeholder={t('settings.parser.languagePlaceholder')} onChange={(value) => setValue('mineru_language', String(value))} /></label>
      </section>
      <section className="wk-config-section"><h3>MinerU Cloud</h3>
        <label>{t('settings.sandbox.apiKey')}<TInput type="password" autocomplete="new-password" placeholder={t('settings.parser.mineruCloudApiKeyPlaceholder')} value={String(values.mineru_api_key)} onChange={(value) => setValue('mineru_api_key', String(value))} /></label>
        <label>Model Version<TSelect className="wk-config-sel-mineru-cloud-model" value={String(values.mineru_cloud_model)} options={[{ value: 'pipeline', label: 'pipeline' }, { value: 'vlm', label: 'VLM' }, { value: 'MinerU-HTML', label: 'MinerU-HTML' }]} onChange={(value) => setValue('mineru_cloud_model', String(value))} /></label>
        <div className="wk-config-toggles">{parserToggle('mineru_cloud_enable_formula', t('settings.parser.formulaRecognition'))}{parserToggle('mineru_cloud_enable_table', t('settings.parser.tableRecognition'))}{parserToggle('mineru_cloud_enable_ocr', 'OCR')}</div>
        <label>{t('settings.parser.language')}<TInput value={String(values.mineru_cloud_language)} placeholder={t('settings.parser.languagePlaceholder')} onChange={(value) => setValue('mineru_cloud_language', String(value))} /></label>
      </section>
      <section className="wk-config-section"><h3>PaddleOCR-VL</h3>
        <label>{t('settings.parser.selfHostedEndpoint')}<TInput data-testid="paddleocr-vl-endpoint" type="url" value={String(values.paddleocr_vl_endpoint)} placeholder={t('settings.parser.paddleOcrEndpointPlaceholder')} onChange={(value) => setValue('paddleocr_vl_endpoint', String(value))} /></label>
        <div className="wk-config-toggles">{parserToggle('paddleocr_vl_use_seal_recognition', t('settings.parser.sealRecognition'))}{parserToggle('paddleocr_vl_use_chart_recognition', t('settings.parser.chartRecognition'))}</div>
      </section>
      <section className="wk-config-section"><h3>PaddleOCR-VL Cloud</h3>
        <label>Access Token<TInput type="password" autocomplete="new-password" value={String(values.paddleocr_vl_cloud_token)} onChange={(value) => setValue('paddleocr_vl_cloud_token', String(value))} /></label>
        <label>Model<TSelect className="wk-config-sel-paddleocr-vl-cloud-model" value={String(values.paddleocr_vl_cloud_model)} options={[{ value: 'PaddleOCR-VL-1.6', label: 'PaddleOCR-VL-1.6' }, { value: 'PaddleOCR-VL-1.5', label: 'PaddleOCR-VL-1.5' }]} onChange={(value) => setValue('paddleocr_vl_cloud_model', String(value))} /></label>
        <div className="wk-config-toggles">{parserToggle('paddleocr_vl_cloud_use_seal_recognition', t('settings.parser.sealRecognition'))}{parserToggle('paddleocr_vl_cloud_use_chart_recognition', t('settings.parser.chartRecognition'))}</div>
      </section><p className="wk-muted">{parserCopy}</p></>}{/* R490 B6 — Vue RetrievalSettings saves debounced exactly like
          ChatHistorySettings (RetrievalSettings.vue handleParamChange →
          debouncedSave), so neither surface renders a save button; only the
          parser section keeps its explicit 保存 + test-connection footer. */}
          {section === 'parser' ? <div className="wk-list-actions"><TButton type="submit" loading={busy} disabled={!dirty} data-testid="config-save">{t('common.save')}</TButton><TButton type="button" disabled={busy} onClick={() => void testParser()}>{t('settings.parser.testConnection')}</TButton></div> : null}</form></Card>
    </>
  );
  return content;
}
