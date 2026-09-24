import { useCallback, useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { roleAtLeast, type SettingsRole } from '@weknora/views/settings/registry';
import { Alert, Button as TButton, Checkbox as TCheckbox, Input as TInput, Loading, Select as TSelect, Tag as TTag, Tooltip } from 'tdesign-react';
import { Icon as TIcon } from 'tdesign-icons-react';
import { WkStatus as Status } from '../shared/wk-legacy.tsx';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';
import { SettingDrawer } from './SettingDrawer.tsx';

/* Full port of Vue ParserEngineSettings.vue: the engine-card grid (monogram
   badge + display name + availability status + localized description), the
   per-engine configuration drawer (mineru / mineru_cloud / paddleocr_vl /
   paddleocr_vl_cloud), the DocReader connection status for builtin, the
   WeKnoraCloud credential state inline alert, and the test-connection /
   save flows.

   TDesign 同构迁移（批 3 面板收敛）：列表域与配置抽屉均按 Vue SFC 逐节点
   复刻——抽屉走 SettingDrawer.tsx（SettingDrawer.vue 同构端口，t-drawer
   chrome + scrim + footer-left 测试连接），样式在 settings.td.css §13/§13b。 */

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

/* Vue ParserEngineSettings.vue:406-413 —— 副标题内联文档外链（mineru 系/
   paddleocr 系/markitdown/weknoracloud）。 */
const ENGINE_DOC_LINKS: Record<string, string> = {
  weknoracloud: 'https://developers.weixin.qq.com/doc/aispeech/knowledge/atomic_capability/atomic_interface.html',
  markitdown: 'https://github.com/microsoft/markitdown',
  mineru: 'https://github.com/opendatalab/MinerU',
  mineru_cloud: 'https://mineru.net/apiManage/docs',
  paddleocr_vl: 'https://github.com/PaddlePaddle/PaddleOCR',
  paddleocr_vl_cloud: 'https://aistudio.baidu.com/paddleocr',
};

/** Vue $t('settings.parser.checking', fallback)：键缺失时回落默认值。 */
function checkingLabel(t: Copy): string {
  const translated = t('settings.parser.checking');
  return translated !== 'settings.parser.checking' ? translated : t('settings.parser.testConnection');
}

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

export function ParserEngineSettingsPanel({ client, role = 'owner' }: { client: WeKnoraClient; role?: SettingsRole }) {
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

  /* 抽屉表单字段写入（等价旧 EngineDrawer 内部 set helper）。 */
  const setConfigField = useCallback(<K extends keyof ParserConfig>(key: K, value: ParserConfig[K]) => {
    setConfig((current) => ({ ...current, [key]: value }));
  }, []);

  /* Vue goToWkcSettings：关闭设置壳后打开 weknoracloud 分区。React 走路由
     query 切换（settings 壳常驻，等价 uiStore.openSettings('weknoracloud')）。 */
  const goToWkcSettings = useCallback(() => {
    window.history.pushState({}, '', '/platform/settings?section=weknoracloud');
    window.dispatchEvent(new window.PopStateEvent('popstate'));
  }, []);

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
    {drawerEngine ? <SettingDrawer
      visible={drawerEngine !== null}
      title={displayOf(drawerName)}
      drawerClass={`parser-engine-drawer parser-engine-drawer--${drawerName}`}
      hideFooter={!roleAtLeast(role, 'admin') && !needsTestButton}
      confirmLoading={saving}
      headerIcon={<span className="header-icon__text">{initialOf(drawerName)}</span>}
      subtitle={<>
        <span>{descOf(drawerName, rowText(drawerEngine, 'Description'))}</span>
        {ENGINE_DOC_LINKS[drawerName] ? <a
          href={ENGINE_DOC_LINKS[drawerName]}
          target="_blank"
          rel="noopener noreferrer"
          className="doc-link doc-link--inline"
        >
          {t('settings.parser.docs')}
          <TIcon name="link" className="link-icon" />
        </a> : null}
      </>}
      footerLeft={needsTestButton ? <>
        <TButton variant="outline" loading={checking} onClick={() => void onCheck()}
          icon={!checking && checkOk && checkMessage ? <TIcon name="check-circle-filled" className="status-icon available" />
            : !checking && checkMessage && !checkOk ? <TIcon name="close-circle-filled" className="status-icon unavailable" /> : undefined}
        >
          {checking ? checkingLabel(t) : t('settings.parser.testConnection')}
        </TButton>
        {checkMessage ? <span className={'footer-test-message' + (checkOk ? ' success' : ' error')} title={checkMessage}>{checkMessage}</span> : null}
      </> : undefined}
      onConfirm={() => void onSave()}
      onCancel={() => setDrawerEngine(null)}
      onVisibleChange={(visible) => { if (!visible) setDrawerEngine(null); }}
    >
      <div>
        {/* Section 1 — 支持文件类型 */}
        {drawerFileTypes.length ? <section className="setting-drawer__section">
          <h4 className="setting-drawer__section-title">{t('settings.parser.supportedFileTypes')}</h4>
          <div className="file-types">
            {drawerFileTypes.map((ft) => <span key={ft} className="file-type-chip">{ft}</span>)}
          </div>
        </section> : null}

        {/* Section 2 — 状态信息（DocReader 连接 / WeKnoraCloud 凭证） */}
        {drawerName === 'builtin' || drawerName === 'weknoracloud' ? <section className="setting-drawer__section">
          <h4 className="setting-drawer__section-title">{t('settings.parser.statusSection')}</h4>
          {drawerName === 'builtin' ? <div className="docreader-block">
            <div className="status-line">
              <TTag theme={connected ? 'success' : 'danger'} variant="light" size="small">
                {connected ? t('settings.parser.connected') : t('settings.parser.disconnected')}
              </TTag>
              <TTag theme="default" variant="light" size="small">{docreaderTransport === 'http' ? 'HTTP' : 'gRPC'}</TTag>
              {docreaderAddrEnv ? <span className="env-hint">{t('settings.parser.currentAddr')}: {docreaderAddrEnv}</span> : null}
            </div>
            <p className="form-desc">{t('settings.parser.envVarHint')}</p>
          </div> : <>
            {wkcState === 'configured' ? <div className="inline-alert inline-alert--ok">
              <TIcon name="check-circle-filled" className="inline-alert__icon" />
              <span>{t('settings.weknoraCloud.credentialConfigured')}</span>
            </div> : wkcState === 'loading' ? <div className="inline-alert">
              <TIcon name="loading" className="inline-alert__icon spinning" />
              <span>{t('settings.weknoraCloud.checkingStatus')}</span>
            </div> : <div className="inline-alert inline-alert--warn">
              <TIcon name="error-circle-filled" className="inline-alert__icon" />
              <span className="inline-alert__text">
                {wkcState === 'expired' ? t('settings.weknoraCloud.credentialExpired') : t('settings.weknoraCloud.unconfigured')}
              </span>
              <a className="inline-alert__action" onClick={goToWkcSettings}>
                {t('settings.weknoraCloud.goToSettings')}
                <TIcon name="chevron-right" />
              </a>
            </div>}
          </>}
        </section> : null}

        {/* Section 3 — mineru 自建配置 */}
        {drawerName === 'mineru' ? <section className="setting-drawer__section">
          <h4 className="setting-drawer__section-title">{t('settings.parser.configSection')}</h4>
          <div className="form-item">
            <label className="form-label">{t('settings.parser.selfHostedEndpoint')}</label>
            <TInput value={config.mineru_endpoint} placeholder={t('settings.parser.mineruEndpointPlaceholder')} clearable onChange={(value) => setConfigField('mineru_endpoint', String(value))} />
          </div>
          <div className="form-item">
            <label className="form-label">Backend</label>
            <TSelect className="wk-parser-sel-mineru-model" value={config.mineru_model} placeholder={t('settings.parser.defaultPipeline')} clearable onChange={(value) => setConfigField('mineru_model', String(value))}>
              <TSelect.Option value="pipeline" label="pipeline" />
              <TSelect.Option value="vlm-auto-engine" label="vlm-auto-engine" />
              <TSelect.Option value="vlm-http-client" label="vlm-http-client" />
              <TSelect.Option value="hybrid-auto-engine" label="hybrid-auto-engine" />
              <TSelect.Option value="hybrid-http-client" label="hybrid-http-client" />
            </TSelect>
          </div>
          <div className="form-item">
            <label className="form-label">vLLM {t('settings.parser.serverUrl')}</label>
            <TInput data-testid="mineru-vllm-server-url" value={config.mineru_vlm_server_url} placeholder={t('settings.parser.vlmServerUrlPlaceholder')} clearable onChange={(value) => setConfigField('mineru_vlm_server_url', String(value))} />
            <p className="form-desc">{t('settings.parser.vlmServerUrlHint')}</p>
          </div>
          <div className="form-item">
            <label className="form-label">{t('settings.parser.parseMethodLabel')}</label>
            <TSelect className="wk-parser-sel-mineru-parse-method" value={config.mineru_parse_method} onChange={(value) => setConfigField('mineru_parse_method', String(value))}>
              <TSelect.Option value="auto" label={t('settings.parser.parseMethodAuto')} />
              <TSelect.Option value="ocr" label={t('settings.parser.parseMethodOCR')} />
              <TSelect.Option value="txt" label={t('settings.parser.parseMethodText')} />
            </TSelect>
            <p className="form-desc">{t('settings.parser.parseMethodHint')}</p>
          </div>
          <div className="form-item">
            <label className="form-label">{t('settings.parser.featuresLabel')}</label>
            <div className="form-toggles">
              <TCheckbox checked={config.mineru_enable_formula} onChange={(checked) => setConfigField('mineru_enable_formula', Boolean(checked))} label={t('settings.parser.formulaRecognition')} />
              <TCheckbox checked={config.mineru_enable_table} onChange={(checked) => setConfigField('mineru_enable_table', Boolean(checked))} label={t('settings.parser.tableRecognition')} />
            </div>
          </div>
          <div className="form-item">
            <label className="form-label">{t('settings.parser.language')}</label>
            <TInput data-testid="mineru-language" value={config.mineru_language} placeholder={t('settings.parser.languagePlaceholder')} clearable onChange={(value) => setConfigField('mineru_language', String(value))} />
          </div>
        </section> : null}

        {/* Section 3 — mineru_cloud 云 API 配置 */}
        {drawerName === 'mineru_cloud' ? <section className="setting-drawer__section">
          <h4 className="setting-drawer__section-title">{t('settings.parser.configSection')}</h4>
          <div className="form-item">
            <label className="form-label required">API Key</label>
            <TInput type="password" value={config.mineru_api_key} placeholder={t('settings.parser.mineruCloudApiKeyPlaceholder')} clearable onChange={(value) => setConfigField('mineru_api_key', String(value))} prefixIcon={<TIcon name="lock-on" />} />
          </div>
          <div className="form-item">
            <label className="form-label">Model Version</label>
            <TSelect className="wk-parser-sel-mineru-cloud-model" value={config.mineru_cloud_model} placeholder={t('settings.parser.defaultPipeline')} clearable onChange={(value) => setConfigField('mineru_cloud_model', String(value))}>
              <TSelect.Option value="pipeline" label="pipeline" />
              <TSelect.Option value="vlm" label={t('settings.parser.vlmLabel')} />
              <TSelect.Option value="MinerU-HTML" label={t('settings.parser.mineruHtmlLabel')} />
            </TSelect>
          </div>
          <div className="form-item">
            <label className="form-label">{t('settings.parser.featuresLabel')}</label>
            <div className="form-toggles">
              <TCheckbox checked={config.mineru_cloud_enable_formula} onChange={(checked) => setConfigField('mineru_cloud_enable_formula', Boolean(checked))} label={t('settings.parser.formulaRecognition')} />
              <TCheckbox checked={config.mineru_cloud_enable_table} onChange={(checked) => setConfigField('mineru_cloud_enable_table', Boolean(checked))} label={t('settings.parser.tableRecognition')} />
              <TCheckbox checked={config.mineru_cloud_enable_ocr} onChange={(checked) => setConfigField('mineru_cloud_enable_ocr', Boolean(checked))} label="OCR" />
            </div>
          </div>
          <div className="form-item">
            <label className="form-label">{t('settings.parser.language')}</label>
            <TInput value={config.mineru_cloud_language} placeholder={t('settings.parser.languagePlaceholder')} clearable onChange={(value) => setConfigField('mineru_cloud_language', String(value))} />
          </div>
        </section> : null}

        {/* Section 3 — paddleocr_vl 自建配置 */}
        {drawerName === 'paddleocr_vl' ? <section className="setting-drawer__section">
          <h4 className="setting-drawer__section-title">{t('settings.parser.configSection')}</h4>
          <div className="form-item">
            <label className="form-label required">{t('settings.parser.selfHostedEndpoint')}</label>
            <TInput data-testid="paddleocr-vl-endpoint" value={config.paddleocr_vl_endpoint} placeholder={t('settings.parser.paddleocrVlEndpointPlaceholder')} clearable onChange={(value) => setConfigField('paddleocr_vl_endpoint', String(value))} />
            <p className="form-desc">{t('settings.parser.paddleocrVlEndpointHint')}</p>
          </div>
          <div className="form-item">
            <label className="form-label">{t('settings.parser.featuresLabel')}</label>
            <div className="form-toggles">
              <TCheckbox checked={config.paddleocr_vl_use_seal_recognition} onChange={(checked) => setConfigField('paddleocr_vl_use_seal_recognition', Boolean(checked))} label={t('settings.parser.sealRecognition')} />
              <TCheckbox checked={config.paddleocr_vl_use_chart_recognition} onChange={(checked) => setConfigField('paddleocr_vl_use_chart_recognition', Boolean(checked))} label={t('settings.parser.chartRecognition')} />
            </div>
          </div>
        </section> : null}

        {/* Section 3 — paddleocr_vl_cloud 云 API 配置 */}
        {drawerName === 'paddleocr_vl_cloud' ? <section className="setting-drawer__section">
          <h4 className="setting-drawer__section-title">{t('settings.parser.configSection')}</h4>
          <div className="form-item">
            <label className="form-label required">Token</label>
            <TInput type="password" value={config.paddleocr_vl_cloud_token} placeholder={t('settings.parser.paddleocrVlCloudTokenPlaceholder')} clearable onChange={(value) => setConfigField('paddleocr_vl_cloud_token', String(value))} prefixIcon={<TIcon name="lock-on" />} />
          </div>
          <div className="form-item">
            <label className="form-label">Model</label>
            <TInput data-testid="paddleocr-vl-cloud-model" value={config.paddleocr_vl_cloud_model} placeholder="PaddleOCR-VL-1.6" clearable onChange={(value) => setConfigField('paddleocr_vl_cloud_model', String(value))} />
          </div>
          <div className="form-item">
            <label className="form-label">{t('settings.parser.featuresLabel')}</label>
            <div className="form-toggles">
              <TCheckbox checked={config.paddleocr_vl_cloud_use_seal_recognition} onChange={(checked) => setConfigField('paddleocr_vl_cloud_use_seal_recognition', Boolean(checked))} label={t('settings.parser.sealRecognition')} />
              <TCheckbox checked={config.paddleocr_vl_cloud_use_chart_recognition} onChange={(checked) => setConfigField('paddleocr_vl_cloud_use_chart_recognition', Boolean(checked))} label={t('settings.parser.chartRecognition')} />
            </div>
          </div>
        </section> : null}
      </div>
    </SettingDrawer> : null}
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

