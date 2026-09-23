import { useCallback, useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
// S6 抽屉收编：配置抽屉离开 @weknora/ui 表单栈（T15 硬前置），组件换 tdesign。
import { Alert, Button as TButton, Checkbox as TCheckbox, Input as TInput, Loading, Select as TSelect, Tooltip } from 'tdesign-react';
import { WkStatus as Status } from '../shared/wk-legacy.tsx';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';

/* Full port of Vue ParserEngineSettings.vue: the engine-card grid (monogram
   badge + display name + availability status + localized description), the
   per-engine configuration drawer (mineru / mineru_cloud / paddleocr_vl /
   paddleocr_vl_cloud), the DocReader connection status for builtin, the
   WeKnoraCloud credential state inline alert, and the test-connection /
   save flows.

   TDesign 同构迁移（批次 2 收尾）：列表域（section-header / loading / error /
   empty / engine-cards）按 Vue SFC 逐节点复刻，样式在 settings.td.css §13；
   配置抽屉沿用 React 表单栈（批次先例：resource/mcp/models 编辑器同口径，
   扫描稳态不可达，待后续批次收编）。 */

type Copy = (key: string, values?: Record<string, string | number>) => string;

interface ParserEngineRow {
  Name?: unknown;
  Available?: unknown;
  UnavailableReason?: unknown;
  Description?: unknown;
  FileTypes?: unknown;
}

interface ParserConfig {
  docreader_addr: string;
  docreader_transport: string;
  mineru_endpoint: string;
  mineru_api_key: string;
  mineru_model: string;
  mineru_vlm_server_url: string;
  mineru_enable_formula: boolean;
  mineru_enable_table: boolean;
  mineru_parse_method: string;
  mineru_enable_ocr: boolean;
  mineru_language: string;
  mineru_cloud_model: string;
  mineru_cloud_enable_formula: boolean;
  mineru_cloud_enable_table: boolean;
  mineru_cloud_enable_ocr: boolean;
  mineru_cloud_language: string;
  paddleocr_vl_endpoint: string;
  paddleocr_vl_use_seal_recognition: boolean;
  paddleocr_vl_use_chart_recognition: boolean;
  paddleocr_vl_cloud_token: string;
  paddleocr_vl_cloud_model: string;
  paddleocr_vl_cloud_use_seal_recognition: boolean;
  paddleocr_vl_cloud_use_chart_recognition: boolean;
}

const DEFAULT_CONFIG: ParserConfig = {
  docreader_addr: '',
  docreader_transport: 'grpc',
  mineru_endpoint: '',
  mineru_api_key: '',
  mineru_model: 'pipeline',
  mineru_vlm_server_url: '',
  mineru_enable_formula: true,
  mineru_enable_table: true,
  mineru_parse_method: 'auto',
  mineru_enable_ocr: true,
  mineru_language: 'ch',
  mineru_cloud_model: 'pipeline',
  mineru_cloud_enable_formula: true,
  mineru_cloud_enable_table: true,
  mineru_cloud_enable_ocr: true,
  mineru_cloud_language: 'ch',
  paddleocr_vl_endpoint: '',
  paddleocr_vl_use_seal_recognition: true,
  paddleocr_vl_use_chart_recognition: false,
  paddleocr_vl_cloud_token: '',
  paddleocr_vl_cloud_model: 'PaddleOCR-VL-1.6',
  paddleocr_vl_cloud_use_seal_recognition: true,
  paddleocr_vl_cloud_use_chart_recognition: false,
};

const CONFIGURABLE_ENGINES = new Set(['mineru', 'mineru_cloud', 'paddleocr_vl', 'paddleocr_vl_cloud']);

const ENGINE_ORDER: Record<string, number> = {
  builtin: 0,
  weknoracloud: 1,
  simple: 2,
  anydoc: 3,
  markitdown: 4,
  mineru: 5,
  mineru_cloud: 6,
  paddleocr_vl: 7,
  paddleocr_vl_cloud: 8,
};

function rowText(row: ParserEngineRow, key: keyof ParserEngineRow): string {
  return typeof row[key] === 'string' ? (row[key] as string) : '';
}

export function ParserEngineSettingsPanel({ client }: { client: WeKnoraClient }) {
  // Stable translator: recreating it per render would re-trigger the load
  // effect below (the callbacks close over it).
  const [t] = useState(() => settingsT(readInitialLocale()));
  const [engines, setEngines] = useState<ParserEngineRow[]>([]);
  const [docreaderAddrEnv, setDocreaderAddrEnv] = useState('');
  const [docreaderTransport, setDocreaderTransport] = useState('grpc');
  const [connected, setConnected] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [config, setConfig] = useState<ParserConfig>({ ...DEFAULT_CONFIG });
  const [drawerEngine, setDrawerEngine] = useState<ParserEngineRow | null>(null);
  const [saving, setSaving] = useState(false);
  const [checking, setChecking] = useState(false);
  const [checkMessage, setCheckMessage] = useState('');
  const [checkOk, setCheckOk] = useState(false);
  const [wkcState, setWkcState] = useState<'loading' | 'unconfigured' | 'configured' | 'expired'>('loading');

  const hasBuiltinEngine = engines.some((engine) => rowText(engine, 'Name') === 'builtin');

  const sortedEngines = [...engines].sort((a, b) => {
    const oa = ENGINE_ORDER[rowText(a, 'Name')] ?? 100;
    const ob = ENGINE_ORDER[rowText(b, 'Name')] ?? 100;
    if (oa !== ob) return oa - ob;
    return rowText(a, 'Name').localeCompare(rowText(b, 'Name'));
  });

  const displayOf = (engineName: string): string => {
    const key = `kbSettings.parser.engines.${engineName}.name`;
    const translated = t(key);
    return translated !== key ? translated : engineName;
  };
  const descOf = (engineName: string, fallback: string): string => {
    const key = `kbSettings.parser.engines.${engineName}.desc`;
    const translated = t(key);
    return translated !== key ? translated : fallback;
  };
  const initialOf = (engineName: string): string => {
    const display = displayOf(engineName);
    return (display.trim().charAt(0) || engineName.charAt(0) || '?').toUpperCase();
  };

  const loadEngines = useCallback(async () => {
    try {
      const probe = await client.settings.parser.engines();
      const items = Array.isArray(probe.items) ? (probe.items as ParserEngineRow[]) : [];
      setEngines(items);
      setDocreaderAddrEnv(typeof probe.docreader_addr === 'string' ? probe.docreader_addr : '');
      setDocreaderTransport(probe.docreader_transport === 'http' ? 'http' : 'grpc');
      setConnected(probe.connected ?? items.length > 0);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t('settings.parser.loadFailed'));
      setEngines([]);
      setConnected(false);
    }
  }, [client, t]);

  const loadConfig = useCallback(async () => {
    try {
      const data = rowOf(await client.settings.parser.config.get());
      setConfig({
        docreader_addr: str(data.docreader_addr, DEFAULT_CONFIG.docreader_addr),
        docreader_transport: str(data.docreader_transport, DEFAULT_CONFIG.docreader_transport),
        mineru_endpoint: str(data.mineru_endpoint, ''),
        mineru_api_key: str(data.mineru_api_key, ''),
        mineru_model: str(data.mineru_model, DEFAULT_CONFIG.mineru_model),
        mineru_vlm_server_url: str(data.mineru_vlm_server_url, ''),
        mineru_enable_formula: bool(data.mineru_enable_formula, true),
        mineru_enable_table: bool(data.mineru_enable_table, true),
        mineru_parse_method: str(data.mineru_parse_method, data.mineru_enable_ocr === false ? 'txt' : 'auto'),
        mineru_enable_ocr: bool(data.mineru_enable_ocr, true),
        mineru_language: str(data.mineru_language, 'ch'),
        mineru_cloud_model: str(data.mineru_cloud_model, DEFAULT_CONFIG.mineru_cloud_model),
        mineru_cloud_enable_formula: bool(data.mineru_cloud_enable_formula, true),
        mineru_cloud_enable_table: bool(data.mineru_cloud_enable_table, true),
        mineru_cloud_enable_ocr: bool(data.mineru_cloud_enable_ocr, true),
        mineru_cloud_language: str(data.mineru_cloud_language, 'ch'),
        paddleocr_vl_endpoint: str(data.paddleocr_vl_endpoint, ''),
        paddleocr_vl_use_seal_recognition: bool(data.paddleocr_vl_use_seal_recognition, true),
        paddleocr_vl_use_chart_recognition: bool(data.paddleocr_vl_use_chart_recognition, false),
        paddleocr_vl_cloud_token: str(data.paddleocr_vl_cloud_token, ''),
        paddleocr_vl_cloud_model: str(data.paddleocr_vl_cloud_model, 'PaddleOCR-VL-1.6'),
        paddleocr_vl_cloud_use_seal_recognition: bool(data.paddleocr_vl_cloud_use_seal_recognition, true),
        paddleocr_vl_cloud_use_chart_recognition: bool(data.paddleocr_vl_cloud_use_chart_recognition, false),
      });
    } catch {
      setConfig({ ...DEFAULT_CONFIG });
    }
  }, [client]);

  const checkWkcStatus = useCallback(async () => {
    setWkcState('loading');
    try {
      const status = rowOf(await client.settings.weknoraCloud.status());
      if (status.needs_reinit === true) setWkcState('expired');
      else if (status.has_models === true) setWkcState('configured');
      else setWkcState('unconfigured');
    } catch { setWkcState('unconfigured'); }
  }, [client]);

  useEffect(() => {
    let active = true;
    setLoading(true); setError(null);
    void (async () => {
      await Promise.all([loadEngines(), loadConfig(), checkWkcStatus()]);
      if (active) setLoading(false);
    })();
    return () => { active = false; };
  }, [loadEngines, loadConfig, checkWkcStatus]);

  function buildConfigPayload(): ParserConfig {
    return {
      ...config,
      docreader_addr: config.docreader_addr.trim(),
      docreader_transport: config.docreader_transport.trim() || 'grpc',
      mineru_endpoint: config.mineru_endpoint.trim(),
      mineru_api_key: config.mineru_api_key.trim(),
      mineru_model: config.mineru_model.trim(),
      mineru_vlm_server_url: config.mineru_vlm_server_url.trim(),
      // Keep the legacy toggle during rolling upgrades — new servers prefer parse_method.
      mineru_enable_ocr: config.mineru_parse_method !== 'txt',
      mineru_language: config.mineru_language.trim(),
      mineru_cloud_model: config.mineru_cloud_model.trim(),
      mineru_cloud_language: config.mineru_cloud_language.trim(),
      paddleocr_vl_endpoint: config.paddleocr_vl_endpoint.trim(),
      paddleocr_vl_cloud_token: config.paddleocr_vl_cloud_token.trim(),
      paddleocr_vl_cloud_model: config.paddleocr_vl_cloud_model.trim(),
    };
  }

  async function onCheck() {
    if (!connected) { setCheckMessage(t('settings.parser.ensureDocreaderConnected')); return; }
    setChecking(true); setCheckMessage('');
    try {
      const probe = await client.settings.parser.check(buildConfigPayload() as unknown as Record<string, unknown>);
      const items = Array.isArray(probe.items) ? (probe.items as ParserEngineRow[]) : [];
      setEngines(items);
      if (typeof probe.connected === 'boolean') setConnected(probe.connected);
      const name = rowText(drawerEngine ?? {}, 'Name');
      if (name === 'builtin') {
        setCheckMessage(t('settings.parser.checkSuccess'));
        setCheckOk(connected);
      } else {
        const updated = items.find((engine) => rowText(engine, 'Name') === name);
        if (updated) {
          const available = updated.Available === true;
          setCheckMessage(available ? t('settings.parser.checkSuccess') : (rowText(updated, 'UnavailableReason') || t('settings.parser.checkFailed')));
          setCheckOk(available);
        } else {
          setCheckMessage(t('settings.parser.checkFailed'));
          setCheckOk(false);
        }
      }
      setTimeout(() => setCheckMessage(''), 3000);
    } catch (reason) {
      setCheckMessage(reason instanceof Error ? reason.message : t('settings.parser.checkFailed'));
      setCheckOk(false);
    } finally { setChecking(false); }
  }

  async function onSave() {
    setSaving(true);
    try {
      await client.settings.parser.config.update(buildConfigPayload() as unknown as Record<string, unknown>);
      setCheckOk(true);
      setDrawerEngine(null);
      void loadEngines();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t('settings.parser.saveFailed'));
    } finally {
      setSaving(false);
    }
  }

  const drawerName = rowText(drawerEngine ?? {}, 'Name');
  const drawerFileTypes = Array.isArray(drawerEngine?.FileTypes) ? (drawerEngine?.FileTypes as unknown[]).map((ft) => String(ft)) : [];
  const needsTestButton = Boolean(drawerEngine) && (CONFIGURABLE_ENGINES.has(drawerName) || drawerName === 'builtin');

  /* Vue ParserEngineSettings.vue:2-94 列表域逐节点平移：section-header →
     loading-state（t-loading + span）→ error-inline（t-alert #operation →
     operation prop）→ empty-state / engine-cards（button 卡片 + monogram 徽章 +
     dot 状态徽；UnavailableReason 走 t-tooltip placement=top）。 */
  return <div className="parser-engine-settings" data-testid="parser-engine-settings">
    <div className="section-header">
      <h2>{t('settings.parser.title')}</h2>
      <p className="section-description">{t('settings.parser.description')}</p>
    </div>
    {loading ? <div className="loading-state">
      <Loading size="small" />
      <span>{t('settings.parser.loading')}</span>
    </div> : error ? <div className="error-inline">
      <Alert
        theme="error"
        message={error}
        operation={<TButton size="small" onClick={() => { setError(null); void loadEngines(); void loadConfig(); void checkWkcStatus(); }}>{t('settings.parser.retry')}</TButton>}
      />
    </div> : <>
      {engines.length === 0 && !hasBuiltinEngine ? <div className="empty-state">
        <p className="empty-text">{t('settings.parser.noEngineDetected')}</p>
      </div> : <div className="engine-cards">
        {/* 当后端未返回 builtin 引擎项时，仍展示 DocReader 状态卡片 */}
        {!hasBuiltinEngine ? <EngineCard
          name="builtin"
          initial={initialOf('builtin')}
          title={displayOf('builtin')}
          desc={t('settings.parser.builtinDesc')}
          statusLabel={connected ? t('settings.parser.connected') : t('settings.parser.disconnected')}
          statusTone={connected ? 'on' : 'err'}
          active={drawerName === 'builtin'}
          onClick={() => setDrawerEngine({ Name: 'builtin' })}
        /> : null}
        {sortedEngines.map((engine) => {
          const name = rowText(engine, 'Name');
          const available = engine.Available === true;
          const reason = rowText(engine, 'UnavailableReason');
          return <EngineCard
            key={name}
            name={name}
            initial={initialOf(name)}
            title={displayOf(name)}
            desc={descOf(name, rowText(engine, 'Description'))}
            statusLabel={available ? t('settings.parser.available') : t('settings.parser.unavailable')}
            statusTone={available ? 'on' : 'err'}
            statusReason={available ? undefined : (reason || undefined)}
            active={drawerName === name}
            onClick={() => setDrawerEngine(engine)}
          />;
        })}
      </div>}
    </>}
    {drawerEngine ? <EngineDrawer
      name={drawerName}
      initial={initialOf(drawerName)}
      title={displayOf(drawerName)}
      desc={descOf(drawerName, rowText(drawerEngine, 'Description'))}
      needsTest={needsTestButton}
      saving={saving}
      checking={checking}
      checkMessage={checkMessage}
      checkOk={checkOk}
      connected={connected}
      docreaderAddrEnv={docreaderAddrEnv}
      docreaderTransport={docreaderTransport}
      wkcState={wkcState}
      config={config}
      fileTypes={drawerFileTypes}
      t={t}
      onCheck={() => void onCheck()}
      onSave={() => void onSave()}
      onClose={() => setDrawerEngine(null)}
      setConfig={setConfig}
    /> : null}
  </div>;
}

function rowOf(value: unknown): Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {};
}
function str(value: unknown, fallback: string): string { return typeof value === 'string' ? value : fallback; }
function bool(value: unknown, fallback: boolean): boolean { return typeof value === 'boolean' ? value : fallback; }

/* Vue engine-card button（ParserEngineSettings.vue:32-91）：class 静态段在前、
   active 条件类按对象键序追加；状态徽三分支 on / err+help(Tooltip) / err。
   per-engine 徽章配色走 §13 .engine-card--{name} .engine-card__badge。 */
function EngineCard({ name, initial, title, desc, statusLabel, statusTone, statusReason, active, onClick }: {
  name: string; initial: string; title: string; desc: string;
  statusLabel: string; statusTone: 'on' | 'err'; statusReason?: string;
  active: boolean; onClick: () => void;
}) {
  return <button
    type="button"
    data-testid={`parser-engine-card-${name}`}
    className={`engine-card engine-card--${name}${active ? ' engine-card--active' : ''}`}
    onClick={onClick}
  >
    <div className="engine-card__badge">{initial}</div>
    <div className="engine-card__body">
      <div className="engine-card__header">
        <h3 className="engine-card__title">{title}</h3>
        {statusTone === 'on' ? <span className="engine-card__status engine-card__status--on">
          <span className="engine-card__status-dot" />
          {statusLabel}
        </span> : statusReason ? <Tooltip content={statusReason} placement="top">
          <span className="engine-card__status engine-card__status--err engine-card__status--help">
            <span className="engine-card__status-dot" />
            {statusLabel}
          </span>
        </Tooltip> : <span className="engine-card__status engine-card__status--err">
          <span className="engine-card__status-dot" />
          {statusLabel}
        </span>}
      </div>
      <p className="engine-card__desc">{desc}</p>
    </div>
  </button>;
}

function EngineDrawer(props: {
  name: string; initial: string; title: string; desc: string;
  needsTest: boolean; saving: boolean; checking: boolean;
  checkMessage: string; checkOk: boolean; connected: boolean;
  docreaderAddrEnv: string; docreaderTransport: string;
  wkcState: 'loading' | 'unconfigured' | 'configured' | 'expired';
  config: ParserConfig; fileTypes: string[];
  t: Copy; onCheck: () => void; onSave: () => void; onClose: () => void;
  setConfig: React.Dispatch<React.SetStateAction<ParserConfig>>;
}) {
  const { name, config, setConfig, t } = props;
  const set = <K extends keyof ParserConfig>(key: K, value: ParserConfig[K]) => setConfig((current) => ({ ...current, [key]: value }));
  return <div className="engine-drawer-overlay" role="presentation" onClick={props.onClose}>
    <section
      role="dialog"
      aria-modal="true"
      aria-label={props.title}
      className="engine-drawer"
      onClick={(event) => event.stopPropagation()}
    >
      <header className="engine-drawer__header">
        <span className="engine-drawer__badge" aria-hidden="true">{props.initial}</span>
        <div className="engine-drawer__head-text">
          <h3 className="engine-drawer__title">{props.title}</h3>
          <p className="engine-drawer__desc">{props.desc}</p>
        </div>
        <button type="button" className="engine-drawer__close" aria-label={t('common.cancel')} onClick={props.onClose}>✕</button>
      </header>
      <div className="engine-drawer__body">
        {props.fileTypes.length ? <section className="engine-drawer__section">
          <h4 className="engine-drawer__section-title">{t('settings.parser.supportedFileTypes')}</h4>
          <div className="engine-drawer__filetypes">{props.fileTypes.map((ft) => <span key={ft} className="engine-drawer__filetype">{ft}</span>)}</div>
        </section> : null}
        {name === 'builtin' || name === 'weknoracloud' ? <section className="engine-drawer__section">
          <h4 className="engine-drawer__section-title">{t('settings.parser.statusSection')}</h4>
          {name === 'builtin' ? <div>
            <div className="engine-drawer__status-row">
              <Status tone={props.connected ? 'success' : 'error'}>{props.connected ? t('settings.parser.connected') : t('settings.parser.disconnected')}</Status>
              <Status>{props.docreaderTransport === 'http' ? 'HTTP' : 'gRPC'}</Status>
              {props.docreaderAddrEnv ? <span className="engine-drawer__addr">{t('settings.parser.currentAddr')}: {props.docreaderAddrEnv}</span> : null}
            </div>
            <p className="engine-drawer__hint">{t('settings.parser.envVarHint')}</p>
          </div> : <div>
            {props.wkcState === 'configured' ? <Status tone="success">{t('settings.weknoraCloud.credentialConfigured')}</Status>
              : props.wkcState === 'loading' ? <Status>{t('settings.weknoraCloud.checkingStatus')}</Status>
              : <div className="engine-drawer__cred-row">
                  <span>{props.wkcState === 'expired' ? t('settings.weknoraCloud.credentialExpired') : t('settings.weknoraCloud.unconfigured')}</span>
                  <a className="engine-drawer__cred-link" href="/platform/settings?section=weknoracloud" onClick={(event) => { event.preventDefault(); window.history.pushState({}, '', '/platform/settings?section=weknoracloud'); window.dispatchEvent(new window.PopStateEvent('popstate')); }}>{t('settings.weknoraCloud.goToSettings')}</a>
                </div>}
          </div>}
        </section> : null}
        {name === 'mineru' ? <section className="engine-drawer__form">
          <h4 className="engine-drawer__section-title">{t('settings.parser.configSection')}</h4>
          <label className="engine-drawer__label">{t('settings.parser.selfHostedEndpoint')}
            <TInput value={config.mineru_endpoint} placeholder={t('settings.parser.mineruEndpointPlaceholder')} onChange={(value) => set('mineru_endpoint', String(value))} />
          </label>
          <label className="engine-drawer__label">Backend
            <TSelect className="wk-parser-sel-mineru-model" value={config.mineru_model} onChange={(value) => set('mineru_model', String(value))}>
              <TSelect.Option value="pipeline" label="pipeline" />
              <TSelect.Option value="vlm-auto-engine" label="vlm-auto-engine" />
              <TSelect.Option value="vlm-http-client" label="vlm-http-client" />
              <TSelect.Option value="hybrid-auto-engine" label="hybrid-auto-engine" />
              <TSelect.Option value="hybrid-http-client" label="hybrid-http-client" />
            </TSelect>
          </label>
          <label className="engine-drawer__label">vLLM {t('settings.parser.serverUrl')}
            <TInput data-testid="mineru-vllm-server-url" value={config.mineru_vlm_server_url} placeholder={t('settings.parser.vlmServerUrlPlaceholder')} onChange={(value) => set('mineru_vlm_server_url', String(value))} />
            <span className="engine-drawer__hint">{t('settings.parser.vlmServerUrlHint')}</span>
          </label>
          <label className="engine-drawer__label">{t('settings.parser.parseMethodLabel')}
            <TSelect className="wk-parser-sel-mineru-parse-method" value={config.mineru_parse_method} onChange={(value) => set('mineru_parse_method', String(value))}>
              <TSelect.Option value="auto" label={t('settings.parser.parseMethodAuto')} />
              <TSelect.Option value="ocr" label={t('settings.parser.parseMethodOCR')} />
              <TSelect.Option value="txt" label={t('settings.parser.parseMethodText')} />
            </TSelect>
            <span className="engine-drawer__hint">{t('settings.parser.parseMethodHint')}</span>
          </label>
          <div className="engine-drawer__checks">
            <label className="engine-drawer__check"><TCheckbox checked={config.mineru_enable_formula} onChange={(checked) => set('mineru_enable_formula', Boolean(checked))} label={t('settings.parser.formulaRecognition')} /></label>
            <label className="engine-drawer__check"><TCheckbox checked={config.mineru_enable_table} onChange={(checked) => set('mineru_enable_table', Boolean(checked))} label={t('settings.parser.tableRecognition')} /></label>
          </div>
          <label className="engine-drawer__label">{t('settings.parser.language')}
            <TInput data-testid="mineru-language" value={config.mineru_language} placeholder={t('settings.parser.languagePlaceholder')} onChange={(value) => set('mineru_language', String(value))} />
          </label>
        </section> : null}
        {name === 'mineru_cloud' ? <section className="engine-drawer__form">
          <h4 className="engine-drawer__section-title">{t('settings.parser.configSection')}</h4>
          <label className="engine-drawer__label">API Key
            <TInput type="password" autocomplete="new-password" value={config.mineru_api_key} placeholder={t('settings.parser.mineruCloudApiKeyPlaceholder')} onChange={(value) => set('mineru_api_key', String(value))} />
          </label>
          <label className="engine-drawer__label">Model Version
            <TSelect className="wk-parser-sel-mineru-cloud-model" value={config.mineru_cloud_model} onChange={(value) => set('mineru_cloud_model', String(value))}>
              <TSelect.Option value="pipeline" label="pipeline" />
              <TSelect.Option value="vlm" label={t('settings.parser.vlmLabel')} />
              <TSelect.Option value="MinerU-HTML" label={t('settings.parser.mineruHtmlLabel')} />
            </TSelect>
          </label>
          <div className="engine-drawer__checks">
            <label className="engine-drawer__check"><TCheckbox checked={config.mineru_cloud_enable_formula} onChange={(checked) => set('mineru_cloud_enable_formula', Boolean(checked))} label={t('settings.parser.formulaRecognition')} /></label>
            <label className="engine-drawer__check"><TCheckbox checked={config.mineru_cloud_enable_table} onChange={(checked) => set('mineru_cloud_enable_table', Boolean(checked))} label={t('settings.parser.tableRecognition')} /></label>
            <label className="engine-drawer__check"><TCheckbox checked={config.mineru_cloud_enable_ocr} onChange={(checked) => set('mineru_cloud_enable_ocr', Boolean(checked))} label="OCR" /></label>
          </div>
          <label className="engine-drawer__label">{t('settings.parser.language')}
            <TInput value={config.mineru_cloud_language} placeholder={t('settings.parser.languagePlaceholder')} onChange={(value) => set('mineru_cloud_language', String(value))} />
          </label>
        </section> : null}
        {name === 'paddleocr_vl' ? <section className="engine-drawer__form">
          <h4 className="engine-drawer__section-title">{t('settings.parser.configSection')}</h4>
          <label className="engine-drawer__label">{t('settings.parser.selfHostedEndpoint')}
            <TInput data-testid="paddleocr-vl-endpoint" value={config.paddleocr_vl_endpoint} placeholder={t('settings.parser.paddleocrVlEndpointPlaceholder')} onChange={(value) => set('paddleocr_vl_endpoint', String(value))} />
            <span className="engine-drawer__hint">{t('settings.parser.paddleocrVlEndpointHint')}</span>
          </label>
          <div className="engine-drawer__checks">
            <label className="engine-drawer__check"><TCheckbox checked={config.paddleocr_vl_use_seal_recognition} onChange={(checked) => set('paddleocr_vl_use_seal_recognition', Boolean(checked))} label={t('settings.parser.sealRecognition')} /></label>
            <label className="engine-drawer__check"><TCheckbox checked={config.paddleocr_vl_use_chart_recognition} onChange={(checked) => set('paddleocr_vl_use_chart_recognition', Boolean(checked))} label={t('settings.parser.chartRecognition')} /></label>
          </div>
        </section> : null}
        {name === 'paddleocr_vl_cloud' ? <section className="engine-drawer__form">
          <h4 className="engine-drawer__section-title">{t('settings.parser.configSection')}</h4>
          <label className="engine-drawer__label">Token
            <TInput type="password" autocomplete="new-password" value={config.paddleocr_vl_cloud_token} placeholder={t('settings.parser.paddleocrVlCloudTokenPlaceholder')} onChange={(value) => set('paddleocr_vl_cloud_token', String(value))} />
          </label>
          <label className="engine-drawer__label">Model
            <TInput data-testid="paddleocr-vl-cloud-model" value={config.paddleocr_vl_cloud_model} placeholder="PaddleOCR-VL-1.6" onChange={(value) => set('paddleocr_vl_cloud_model', String(value))} />
          </label>
          <div className="engine-drawer__checks">
            <label className="engine-drawer__check"><TCheckbox checked={config.paddleocr_vl_cloud_use_seal_recognition} onChange={(checked) => set('paddleocr_vl_cloud_use_seal_recognition', Boolean(checked))} label={t('settings.parser.sealRecognition')} /></label>
            <label className="engine-drawer__check"><TCheckbox checked={config.paddleocr_vl_cloud_use_chart_recognition} onChange={(checked) => set('paddleocr_vl_cloud_use_chart_recognition', Boolean(checked))} label={t('settings.parser.chartRecognition')} /></label>
          </div>
        </section> : null}
      </div>
      {props.needsTest ? <footer className="engine-drawer__footer">
        <span className="engine-drawer__footer-left">
          <TButton type="button" loading={props.checking} onClick={props.onCheck}>{t('settings.parser.testConnection')}</TButton>
          {props.checkMessage ? <span className={"engine-drawer__footer-msg" + (props.checkOk ? " engine-drawer__footer-msg--ok" : " engine-drawer__footer-msg--err")} title={props.checkMessage}>{props.checkMessage}</span> : null}
        </span>
        <span className="engine-drawer__footer-right">
          <TButton type="button" onClick={props.onClose}>{t('common.cancel')}</TButton>
          <TButton type="button" theme="primary" loading={props.saving} onClick={props.onSave}>{t('common.save')}</TButton>
        </span>
      </footer> : <footer className="engine-drawer__footer engine-drawer__footer--end">
        <TButton type="button" onClick={props.onClose}>{t('common.cancel')}</TButton>
      </footer>}
    </section>
  </div>;
}
