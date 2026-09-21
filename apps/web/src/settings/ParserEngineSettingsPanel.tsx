import { useCallback, useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Checkbox, Input, Select, Status } from '@weknora/ui';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';

/* Full port of Vue ParserEngineSettings.vue: the engine-card grid (monogram
   badge + display name + availability status + localized description), the
   per-engine configuration drawer (mineru / mineru_cloud / paddleocr_vl /
   paddleocr_vl_cloud), the DocReader connection status for builtin, the
   WeKnoraCloud credential state inline alert, and the test-connection /
   save flows. */

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

const ENGINE_DOC_LINKS: Record<string, string> = {
  weknoracloud: 'https://developers.weixin.qq.com/doc/aispeech/knowledge/atomic_capability/atomic_interface.html',
  markitdown: 'https://github.com/microsoft/markitdown',
  mineru: 'https://github.com/opendatalab/MinerU',
  mineru_cloud: 'https://mineru.net/apiManage/docs',
  paddleocr_vl: 'https://github.com/PaddlePaddle/PaddleOCR',
  paddleocr_vl_cloud: 'https://aistudio.baidu.com/paddleocr',
};

function rowText(row: ParserEngineRow, key: keyof ParserEngineRow): string {
  return typeof row[key] === 'string' ? (row[key] as string) : '';
}

// Vue ParserEngineSettings.vue .engine-card--{name} .engine-card__badge palette:
// 每类引擎各有浅色底（builtin/weknoracloud 绿 0.12、simple 灰 0.1、markitdown 蓝
// 0.12、mineru/paddleocr 紫 0.12），未知引擎回退蓝底（ParserEngineSettings.vue:822-843）。
const BADGE_TONES: Record<string, { bg: string; fg: string }> = {
  builtin: { bg: 'rgba(7, 192, 95, 0.12)', fg: '#07C05F' },
  weknoracloud: { bg: 'rgba(7, 192, 95, 0.12)', fg: '#07C05F' },
  simple: { bg: 'rgba(70, 70, 70, 0.1)', fg: '#464646' },
  markitdown: { bg: 'rgba(0, 137, 255, 0.12)', fg: '#0089FF' },
  mineru: { bg: 'rgba(98, 53, 187, 0.12)', fg: '#6235BB' },
  mineru_cloud: { bg: 'rgba(98, 53, 187, 0.12)', fg: '#6235BB' },
  paddleocr_vl: { bg: 'rgba(98, 53, 187, 0.12)', fg: '#6235BB' },
  paddleocr_vl_cloud: { bg: 'rgba(98, 53, 187, 0.12)', fg: '#6235BB' },
};
const BADGE_BASE = { bg: 'rgba(0, 82, 217, 0.1)', fg: '#0052D9' };

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
    } finally { setSaving(false); }
  }

  const drawerName = rowText(drawerEngine ?? {}, 'Name');
  const drawerFileTypes = Array.isArray(drawerEngine?.FileTypes) ? (drawerEngine?.FileTypes as unknown[]).map((ft) => String(ft)) : [];
  const needsTestButton = Boolean(drawerEngine) && (CONFIGURABLE_ENGINES.has(drawerName) || drawerName === 'builtin');

  return <div className="parser-engine-settings" data-testid="parser-engine-settings">
    {/* Vue ParserEngineSettings.vue:3-9 — the panel owns its section-header
        (h2 + description, no divider, margin-bottom 28px); the SettingsPage
        wrapper heading is suppressed via CSS while this panel is mounted.
        There is no outer card around the grid. */}
    <div className="section-header">
      <h2>{t('settings.parser.title')}</h2>
      <p className="section-description">{t('settings.parser.description')}</p>
    </div>
    {loading ? <Status>{t('settings.parser.loading', { defaultValue: '' }) || t('common.loading')}</Status> : null}
    {!loading ? <>
      {error ? <div className="mb-3 flex items-center gap-2"><Status tone="error">{error}</Status><Button type="button" size="small" onClick={() => { setError(null); void loadEngines(); void loadConfig(); void checkWkcStatus(); }}>{t('settings.parser.retry')}</Button></div> : null}
      {engines.length === 0 && !hasBuiltinEngine ? <Status>{t('settings.parser.noEngineDetected')}</Status> : null}
      {(engines.length > 0 || hasBuiltinEngine) ? <div className="mt-6 grid grid-cols-[repeat(auto-fill,minmax(min(100%,320px),1fr))] gap-3">
        {!hasBuiltinEngine ? <EngineCard
          name="builtin" initial={initialOf('builtin')} title={displayOf('builtin')}
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
      </div> : null}
    </> : null}
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

function EngineCard({ name, initial, title, desc, statusLabel, statusTone, statusReason, active, onClick }: {
  name: string; initial: string; title: string; desc: string;
  statusLabel: string; statusTone: 'on' | 'err'; statusReason?: string;
  active: boolean; onClick: () => void;
}) {
  return <button
    type="button"
    data-testid={`parser-engine-card-${name}`}
    className={`group flex w-full cursor-pointer items-start gap-3 rounded-[10px] border bg-surface py-[14px] pr-[14px] pl-3 text-left [font:inherit] [transition:border-color_.2s_ease,box-shadow_.2s_ease] ${active ? 'border-accent shadow-[0_0_0_1px_var(--wk-brand,#0052d9)]' : 'border-line-soft hover:border-accent/50 hover:shadow-[0_4px_14px_rgba(15,23,42,0.07)]'}`}
    onClick={onClick}
  >
    <span
      className="mt-px inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-[9px] text-[15px] font-semibold leading-[normal] tracking-[0.02em]"
      style={{ background: (BADGE_TONES[name] ?? BADGE_BASE).bg, color: (BADGE_TONES[name] ?? BADGE_BASE).fg }}
      aria-hidden="true"
    >{initial}</span>
    <span className="min-w-0 flex-1">
      <span className="flex items-center justify-between gap-[6px]">
        {/* Vue .engine-card__title: 14px/600 with line-height 1.4 (19.6px) — the
            inherited 1.5 makes each card row ~1.4px taller and drifts the grid. */}
        <h3 className="m-0 min-w-0 truncate text-[14px] font-semibold leading-[1.4] text-ink">{title}</h3>
        {/* Vue .engine-card__status: 11px/500, lh 16px, padding 1px 8px 1px 6px,
            radius 10px, neutral secondary-container background. */}
        {/* Vue .engine-card__status: 11px/500, lh 16px, padding 1px 8px 1px 6px,
            radius 10px, neutral secondary-container background; on=#067945
            (success-7) with #00a870 dot, err=#C9353F (error-7) with #e34d59 dot. */}
        <span
          className={`inline-flex shrink-0 items-center gap-[5px] rounded-[10px] bg-[#f3f3f3] py-[1px] pl-[6px] pr-[8px] text-[11px] font-medium leading-4 ${statusTone === 'on' ? 'text-[#067945]' : 'text-[#c9353f]'}`}
          title={statusReason}
        >
          <span className={`inline-block h-[6px] w-[6px] rounded-full ${statusTone === 'on' ? 'bg-[#00a870]' : 'bg-[#e34d59]'}`} />
          {statusLabel}
        </span>
      </span>
      <span className="mt-1 block text-[12px] leading-[1.5] text-muted [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2] overflow-hidden">{desc}</span>
    </span>
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
  return <div className="fixed inset-0 z-[1300] flex justify-end bg-[rgba(0,0,0,.5)]" role="presentation" onClick={props.onClose}>
    <section
      role="dialog"
      aria-modal="true"
      aria-label={props.title}
      className="flex h-full w-[560px] max-w-[92vw] flex-col bg-surface shadow-[0_12px_40px_rgba(15,23,42,0.2)]"
      onClick={(event) => event.stopPropagation()}
    >
      <header className="flex items-start gap-3 border-b border-line-soft px-5 py-4">
        <span className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-control bg-surface-wash text-[15px] font-semibold text-accent" aria-hidden="true">{props.initial}</span>
        <div className="min-w-0 flex-1">
          <h3 className="m-0 truncate text-[16px] font-semibold text-ink">{props.title}</h3>
          <p className="m-0 mt-0.5 text-[12px] text-muted">{props.desc}</p>
        </div>
        <button type="button" className="cursor-pointer border-0 bg-transparent p-1 text-muted" aria-label={t('common.cancel')} onClick={props.onClose}>✕</button>
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
        {props.fileTypes.length ? <section className="mb-5">
          <h4 className="m-0 mb-2 text-[13px] font-semibold text-ink">{t('settings.parser.supportedFileTypes')}</h4>
          <div className="flex flex-wrap gap-1.5">{props.fileTypes.map((ft) => <span key={ft} className="rounded-full border border-line-soft bg-surface-wash px-2 py-0.5 text-[11px] text-muted">{ft}</span>)}</div>
        </section> : null}
        {name === 'builtin' || name === 'weknoracloud' ? <section className="mb-5">
          <h4 className="m-0 mb-2 text-[13px] font-semibold text-ink">{t('settings.parser.statusSection')}</h4>
          {name === 'builtin' ? <div>
            <div className="flex flex-wrap items-center gap-2">
              <Status tone={props.connected ? 'success' : 'error'}>{props.connected ? t('settings.parser.connected') : t('settings.parser.disconnected')}</Status>
              <Status>{props.docreaderTransport === 'http' ? 'HTTP' : 'gRPC'}</Status>
              {props.docreaderAddrEnv ? <span className="text-[12px] text-muted-strong">{t('settings.parser.currentAddr')}: {props.docreaderAddrEnv}</span> : null}
            </div>
            <p className="mb-0 mt-2 text-[12px] text-muted">{t('settings.parser.envVarHint')}</p>
          </div> : <div>
            {props.wkcState === 'configured' ? <Status tone="success">{t('settings.weknoraCloud.credentialConfigured')}</Status>
              : props.wkcState === 'loading' ? <Status>{t('settings.weknoraCloud.checkingStatus')}</Status>
              : <div className="flex items-center gap-2 text-[13px] text-[#ad4b00]">
                  <span>{props.wkcState === 'expired' ? t('settings.weknoraCloud.credentialExpired') : t('settings.weknoraCloud.unconfigured')}</span>
                  <a className="cursor-pointer text-[#245a9b] hover:underline" href="/platform/settings?section=weknoracloud" onClick={(event) => { event.preventDefault(); window.history.pushState({}, '', '/platform/settings?section=weknoracloud'); window.dispatchEvent(new window.PopStateEvent('popstate')); }}>{t('settings.weknoraCloud.goToSettings')}</a>
                </div>}
          </div>}
        </section> : null}
        {name === 'mineru' ? <section className="mb-5 grid gap-3">
          <h4 className="m-0 text-[13px] font-semibold text-ink">{t('settings.parser.configSection')}</h4>
          <label className="grid gap-1 text-[13px] font-medium text-[#27364d]">{t('settings.parser.selfHostedEndpoint')}
            <Input value={config.mineru_endpoint} placeholder={t('settings.parser.mineruEndpointPlaceholder')} onChange={(event) => set('mineru_endpoint', event.target.value)} />
          </label>
          <label className="grid gap-1 text-[13px] font-medium text-[#27364d]">Backend
            <Select data-testid="mineru-model" value={config.mineru_model} onChange={(event) => set('mineru_model', event.target.value)}>
              <option value="pipeline">pipeline</option>
              <option value="vlm-auto-engine">vlm-auto-engine</option>
              <option value="vlm-http-client">vlm-http-client</option>
              <option value="hybrid-auto-engine">hybrid-auto-engine</option>
              <option value="hybrid-http-client">hybrid-http-client</option>
            </Select>
          </label>
          <label className="grid gap-1 text-[13px] font-medium text-[#27364d]">vLLM {t('settings.parser.serverUrl')}
            <Input data-testid="mineru-vllm-server-url" value={config.mineru_vlm_server_url} placeholder={t('settings.parser.vlmServerUrlPlaceholder')} onChange={(event) => set('mineru_vlm_server_url', event.target.value)} />
            <span className="text-[12px] text-muted">{t('settings.parser.vlmServerUrlHint')}</span>
          </label>
          <label className="grid gap-1 text-[13px] font-medium text-[#27364d]">{t('settings.parser.parseMethodLabel')}
            <Select data-testid="mineru-parse-method" value={config.mineru_parse_method} onChange={(event) => set('mineru_parse_method', event.target.value)}>
              <option value="auto">{t('settings.parser.parseMethodAuto')}</option>
              <option value="ocr">{t('settings.parser.parseMethodOCR')}</option>
              <option value="txt">{t('settings.parser.parseMethodText')}</option>
            </Select>
            <span className="text-[12px] text-muted">{t('settings.parser.parseMethodHint')}</span>
          </label>
          <div className="flex flex-wrap gap-4 text-[13px]">
            <label className="flex items-center gap-1.5"><Checkbox checked={config.mineru_enable_formula} onChange={(event) => set('mineru_enable_formula', event.target.checked)} />{t('settings.parser.formulaRecognition')}</label>
            <label className="flex items-center gap-1.5"><Checkbox checked={config.mineru_enable_table} onChange={(event) => set('mineru_enable_table', event.target.checked)} />{t('settings.parser.tableRecognition')}</label>
          </div>
          <label className="grid gap-1 text-[13px] font-medium text-[#27364d]">{t('settings.parser.language')}
            <Input data-testid="mineru-language" value={config.mineru_language} placeholder={t('settings.parser.languagePlaceholder')} onChange={(event) => set('mineru_language', event.target.value)} />
          </label>
        </section> : null}
        {name === 'mineru_cloud' ? <section className="mb-5 grid gap-3">
          <h4 className="m-0 text-[13px] font-semibold text-ink">{t('settings.parser.configSection')}</h4>
          <label className="grid gap-1 text-[13px] font-medium text-[#27364d]">API Key
            <Input type="password" autoComplete="new-password" value={config.mineru_api_key} placeholder={t('settings.parser.mineruCloudApiKeyPlaceholder')} onChange={(event) => set('mineru_api_key', event.target.value)} />
          </label>
          <label className="grid gap-1 text-[13px] font-medium text-[#27364d]">Model Version
            <Select data-testid="mineru-cloud-model" value={config.mineru_cloud_model} onChange={(event) => set('mineru_cloud_model', event.target.value)}>
              <option value="pipeline">pipeline</option>
              <option value="vlm">{t('settings.parser.vlmLabel')}</option>
              <option value="MinerU-HTML">{t('settings.parser.mineruHtmlLabel')}</option>
            </Select>
          </label>
          <div className="flex flex-wrap gap-4 text-[13px]">
            <label className="flex items-center gap-1.5"><Checkbox checked={config.mineru_cloud_enable_formula} onChange={(event) => set('mineru_cloud_enable_formula', event.target.checked)} />{t('settings.parser.formulaRecognition')}</label>
            <label className="flex items-center gap-1.5"><Checkbox checked={config.mineru_cloud_enable_table} onChange={(event) => set('mineru_cloud_enable_table', event.target.checked)} />{t('settings.parser.tableRecognition')}</label>
            <label className="flex items-center gap-1.5"><Checkbox checked={config.mineru_cloud_enable_ocr} onChange={(event) => set('mineru_cloud_enable_ocr', event.target.checked)} />OCR</label>
          </div>
          <label className="grid gap-1 text-[13px] font-medium text-[#27364d]">{t('settings.parser.language')}
            <Input value={config.mineru_cloud_language} placeholder={t('settings.parser.languagePlaceholder')} onChange={(event) => set('mineru_cloud_language', event.target.value)} />
          </label>
        </section> : null}
        {name === 'paddleocr_vl' ? <section className="mb-5 grid gap-3">
          <h4 className="m-0 text-[13px] font-semibold text-ink">{t('settings.parser.configSection')}</h4>
          <label className="grid gap-1 text-[13px] font-medium text-[#27364d]">{t('settings.parser.selfHostedEndpoint')}
            <Input data-testid="paddleocr-vl-endpoint" value={config.paddleocr_vl_endpoint} placeholder={t('settings.parser.paddleocrVlEndpointPlaceholder')} onChange={(event) => set('paddleocr_vl_endpoint', event.target.value)} />
            <span className="text-[12px] text-muted">{t('settings.parser.paddleocrVlEndpointHint')}</span>
          </label>
          <div className="flex flex-wrap gap-4 text-[13px]">
            <label className="flex items-center gap-1.5"><Checkbox checked={config.paddleocr_vl_use_seal_recognition} onChange={(event) => set('paddleocr_vl_use_seal_recognition', event.target.checked)} />{t('settings.parser.sealRecognition')}</label>
            <label className="flex items-center gap-1.5"><Checkbox checked={config.paddleocr_vl_use_chart_recognition} onChange={(event) => set('paddleocr_vl_use_chart_recognition', event.target.checked)} />{t('settings.parser.chartRecognition')}</label>
          </div>
        </section> : null}
        {name === 'paddleocr_vl_cloud' ? <section className="mb-5 grid gap-3">
          <h4 className="m-0 text-[13px] font-semibold text-ink">{t('settings.parser.configSection')}</h4>
          <label className="grid gap-1 text-[13px] font-medium text-[#27364d]">Token
            <Input type="password" autoComplete="new-password" value={config.paddleocr_vl_cloud_token} placeholder={t('settings.parser.paddleocrVlCloudTokenPlaceholder')} onChange={(event) => set('paddleocr_vl_cloud_token', event.target.value)} />
          </label>
          <label className="grid gap-1 text-[13px] font-medium text-[#27364d]">Model
            <Input data-testid="paddleocr-vl-cloud-model" value={config.paddleocr_vl_cloud_model} placeholder="PaddleOCR-VL-1.6" onChange={(event) => set('paddleocr_vl_cloud_model', event.target.value)} />
          </label>
          <div className="flex flex-wrap gap-4 text-[13px]">
            <label className="flex items-center gap-1.5"><Checkbox checked={config.paddleocr_vl_cloud_use_seal_recognition} onChange={(event) => set('paddleocr_vl_cloud_use_seal_recognition', event.target.checked)} />{t('settings.parser.sealRecognition')}</label>
            <label className="flex items-center gap-1.5"><Checkbox checked={config.paddleocr_vl_cloud_use_chart_recognition} onChange={(event) => set('paddleocr_vl_cloud_use_chart_recognition', event.target.checked)} />{t('settings.parser.chartRecognition')}</label>
          </div>
        </section> : null}
      </div>
      {props.needsTest ? <footer className="flex items-center justify-between gap-3 border-t border-line-soft px-5 py-3">
        <span className="flex items-center gap-2">
          <Button type="button" variant="default" loading={props.checking} onClick={props.onCheck}>{t('settings.parser.testConnection')}</Button>
          {props.checkMessage ? <span className={`text-[12px] ${props.checkOk ? 'text-[#0a7f43]' : 'text-[#c23434]'}`} title={props.checkMessage}>{props.checkMessage}</span> : null}
        </span>
        <span className="flex items-center gap-2">
          <Button type="button" onClick={props.onClose}>{t('common.cancel')}</Button>
          <Button type="button" loading={props.saving} onClick={props.onSave}>{t('common.save')}</Button>
        </span>
      </footer> : <footer className="flex items-center justify-end gap-2 border-t border-line-soft px-5 py-3">
        <Button type="button" onClick={props.onClose}>{t('common.cancel')}</Button>
      </footer>}
    </section>
  </div>;
}
