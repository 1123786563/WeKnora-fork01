import { useEffect, useRef, useState } from 'react';
import type { CSSProperties, ReactNode } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type { Locale } from '@weknora/i18n';
import { Button as TButton, Dropdown as TDropdown, Input as TInput, Select as TSelect, Switch as TSwitch, Tag as TTag } from 'tdesign-react';
import { WkStatus as Status } from '../shared/wk-legacy.tsx';
import { Icon as TIcon } from 'tdesign-icons-react';
import { SettingDrawer } from './SettingDrawer.tsx';
import { roleAtLeast, type SettingsRole } from '@weknora/views/settings/registry';
import { settingsResourceRows, settingsSectionHeading } from './surface.ts';
import { providerLogo } from './providerLogos.ts';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';
import {
  isReplicaField,
  parseStorageProviders,
  parseVectorStoreTypes,
  parseWebSearchTypes,
  storageBackendPayload,
  storageBlankConfig,
  storageNeedsCredentials,
  storageNeedsEndpoint,
  storageNeedsRegion,
  vectorStoreCanTest,
  vectorStoreCreatePayload,
  vectorStoreFieldLabel,
  vectorStoreUpdatePayload,
  webSearchCanTest,
  webSearchConfigText,
  webSearchCreatePayload,
  webSearchParamsOut,
  webSearchUpdatePayload,
  type StorageBackendConfig,
  type VectorFieldSchema,
  type VectorStoreTypeInfo,
  type WebSearchTypeInfo,
} from './resource-forms.ts';

type ResourceSection = 'storage' | 'vectorstore' | 'websearch';
type ResourceRow = Record<string, unknown>;
type ResourceTestResult = { success: boolean; message?: string; error?: string };
type ResourceApi = {
  list: () => Promise<readonly ResourceRow[]>;
  listEnvelope?: () => Promise<{ rows: readonly ResourceRow[]; defaultId?: string }>;
  types?: () => Promise<readonly unknown[]>;
  create: (input: Record<string, unknown>) => Promise<ResourceRow>;
  update: (id: string, input: Record<string, unknown>) => Promise<ResourceRow>;
  remove: (id: string) => Promise<unknown>;
  testById: (id: string) => Promise<ResourceTestResult>;
  testRaw?: (input: Record<string, unknown>) => Promise<ResourceTestResult>;
  setDefault?: (id: string) => Promise<unknown>;
  putCredentials?: (id: string, input: Record<string, unknown>) => Promise<unknown>;
  deleteCredential?: (id: string, field: string) => Promise<unknown>;
};

const RESOURCE_COPY: Record<Locale, { unnamed: string; typeUnavailable: string; defaultLabel: string; loadFailed: string; setDefaultFailed: string; localLabel: string; expand: string; collapse: string }> = {
  'zh-CN': { unnamed: '未命名资源', typeUnavailable: '类型未知', defaultLabel: '默认', loadFailed: '资源列表加载失败', setDefaultFailed: '设置默认资源失败', localLabel: '本地存储', expand: '展开', collapse: '收起' },
  'en-US': { unnamed: 'Unnamed resource', typeUnavailable: 'Type unavailable', defaultLabel: 'default', loadFailed: 'Failed to load resources', setDefaultFailed: 'Failed to set the default resource', localLabel: 'Local storage', expand: 'Expand', collapse: 'Collapse' },
  'ja-JP': { unnamed: '名前なしのリソース', typeUnavailable: '種類不明', defaultLabel: 'デフォルト', loadFailed: 'リソース一覧の読み込みに失敗しました', setDefaultFailed: 'デフォルトリソースの設定に失敗しました', localLabel: 'ローカルストレージ', expand: '展開', collapse: '折りたたむ' },
  'ko-KR': { unnamed: '이름 없는 리소스', typeUnavailable: '유형 없음', defaultLabel: '기본값', loadFailed: '리소스 목록을 불러오지 못했습니다', setDefaultFailed: '기본 리소스를 설정하지 못했습니다', localLabel: '로컬 스토리지', expand: '펼치기', collapse: '접기' },
  'ru-RU': { unnamed: 'Ресурс без имени', typeUnavailable: 'Тип неизвестен', defaultLabel: 'по умолчанию', loadFailed: 'Не удалось загрузить список ресурсов', setDefaultFailed: 'Не удалось назначить ресурсом по умолчанию', localLabel: 'Локальное хранилище', expand: 'Развернуть', collapse: 'Свернуть' },
};

function rowId(row: ResourceRow): string {
  const id = row.id ?? row.uuid ?? row.name;
  return typeof id === 'string' || typeof id === 'number' ? String(id) : '';
}

function rowText(row: ResourceRow, key: string): string { return typeof row[key] === 'string' ? row[key] as string : ''; }

function rowRecord(row: ResourceRow, key: string): Record<string, unknown> {
  const value = row[key];
  return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {};
}

function providerInitial(provider: string): string { return (provider.trim().charAt(0) || '?').toUpperCase(); }

/** Mono brand logo rendered as a currentColor mask (Vue .header-icon__mono /
 *  .provider-card__badge--mono::before share the same recipe). */
function monoLogoMask(url: string): CSSProperties {
  return {
    width: '22px',
    height: '22px',
    backgroundColor: 'currentColor',
    WebkitMaskImage: `url("${url}")`,
    WebkitMaskPosition: 'center',
    WebkitMaskRepeat: 'no-repeat',
    WebkitMaskSize: 'contain',
    maskImage: `url("${url}")`,
    maskPosition: 'center',
    maskRepeat: 'no-repeat',
    maskSize: 'contain',
  };
}

/** Vue #headerIcon 槽 — 原始 logo/mono/monogram 三级阶梯（无徽章包装）：
 *  32px .setting-drawer__header-icon 容器与 unscoped per-engine 配色
 *  （settings.td.css §13c）承载视觉。VectorStoreSettings.vue:121-133 /
 *  StorageBackendSettings.vue:95-107 / WebSearchSettings.vue:103-115。 */
function drawerHeaderIcon(section: ResourceSection, id: string, initial: string): ReactNode {
  const logo = providerLogo(section, id);
  if (logo?.mode === 'color') {
    return <img src={logo.url} alt={id} className="header-icon__img" />;
  }
  if (logo?.mode === 'mono') {
    return <span className="header-icon__mono" style={{ '--logo-url': `url("${logo.url}")` } as CSSProperties} />;
  }
  return <span className="header-icon__text">{initial || '?'}</span>;
}

/** Vue 抽屉 :class 透传（per-engine unscoped 配色锚点）：
 *  StorageBackendSettings.vue:90 / VectorStoreSettings.vue:428-432 /
 *  WebSearchSettings.vue:428-432。 */
function drawerFamilyClass(section: ResourceSection, type: string): string {
  const id = type.trim().toLowerCase();
  if (section === 'storage') return `storage-backend-drawer storage-backend-drawer--${id || 'local'}`;
  const family = section === 'vectorstore' ? 'vectorstore-drawer' : 'websearch-drawer';
  return id ? `${family} ${family}--${id}` : family;
}

// Per-provider brand colors (Vue .backend-card--<id>/.store-card--<id> badge
// rules: StorageBackendSettings.vue:524-531, VectorStoreSettings.vue:949-987).
// Monogram and mono-logo badges tint with the brand color; color logos render
// the multi-color SVG as-is.
const PROVIDER_BRAND: Record<ResourceSection, Record<string, { bg: string; color: string }>> = {
  storage: {
    local: { bg: 'rgba(70, 70, 70, 0.1)', color: '#464646' },
    minio: { bg: 'rgba(225, 38, 38, 0.12)', color: '#C0382B' },
    cos: { bg: 'rgba(0, 82, 217, 0.1)', color: '#0052D9' },
    tos: { bg: 'rgba(0, 137, 255, 0.12)', color: '#0089FF' },
    s3: { bg: 'rgba(255, 153, 0, 0.12)', color: '#D97706' },
    oss: { bg: 'rgba(255, 90, 0, 0.12)', color: '#E55A00' },
    ks3: { bg: 'rgba(7, 192, 95, 0.12)', color: '#07A050' },
    obs: { bg: 'rgba(206, 17, 38, 0.1)', color: '#CE1126' },
  },
  vectorstore: {
    qdrant: { bg: 'rgba(225, 38, 38, 0.12)', color: '#E12626' },
    milvus: { bg: 'rgba(0, 137, 255, 0.12)', color: '#0089FF' },
    weaviate: { bg: 'rgba(7, 192, 95, 0.12)', color: '#07A050' },
    elasticsearch: { bg: 'rgba(255, 153, 0, 0.12)', color: '#D97706' },
    elasticfaiss: { bg: 'rgba(255, 153, 0, 0.12)', color: '#D97706' },
    postgres: { bg: 'rgba(0, 82, 217, 0.1)', color: '#0052D9' },
    opensearch: { bg: 'rgba(98, 53, 187, 0.12)', color: '#6235BB' },
    infinity: { bg: 'rgba(98, 53, 187, 0.12)', color: '#6235BB' },
    tencent_vectordb: { bg: 'rgba(0, 82, 217, 0.1)', color: '#0052D9' },
    doris: { bg: 'rgba(255, 90, 0, 0.12)', color: '#E55A00' },
    sqlite: { bg: 'rgba(70, 70, 70, 0.1)', color: '#464646' },
  },
  websearch: {
    // Vue WebSearchSettings.vue provider-card--{id} badge colors (L884-921);
    // the drawer header badge (non-scoped drawer block L1168-1203) mirrors them.
    duckduckgo: { bg: 'rgba(222, 88, 51, 0.12)', color: '#DE5833' },
    bing: { bg: 'rgba(0, 137, 255, 0.12)', color: '#0089FF' },
    google: { bg: 'rgba(66, 133, 244, 0.12)', color: '#4285F4' },
    tavily: { bg: 'rgba(98, 53, 187, 0.12)', color: '#6235BB' },
    baidu: { bg: 'rgba(41, 50, 225, 0.12)', color: '#2932E1' },
    searxng: { bg: 'rgba(33, 86, 137, 0.12)', color: '#215689' },
    ollama: { bg: 'rgba(70, 70, 70, 0.12)', color: '#464646' },
    keenable: { bg: 'rgba(20, 158, 130, 0.12)', color: '#149E82' },
    zhipu: { bg: 'rgba(37, 99, 235, 0.12)', color: '#2563EB' },
  },
};

function apiFor(client: WeKnoraClient, section: ResourceSection): ResourceApi {
  if (section === 'storage') {
    const backends = client.settings.storage.backends;
    return {
      ...backends,
      ...(backends.test ? { testRaw: backends.test.bind(backends) } : {}),
      // Vue StorageBackendSettings compares backend.id against the list
      // envelope's default_storage_backend_id, so the default id travels with
      // the rows.
      listEnvelope: async () => {
        const result = await (backends as unknown as { listWithEnvelope?: () => Promise<{ rows: readonly ResourceRow[]; defaultId?: string }> }).listWithEnvelope?.();
        return result ?? { rows: await backends.list(), defaultId: undefined };
      },
    };
  }
  if (section === 'vectorstore') {
    const stores = client.settings.vectorStores;
    return { ...stores, ...(stores.test ? { testRaw: stores.test.bind(stores) } : {}) };
  }
  const providers = client.settings.webSearch.providers;
  return { ...providers, ...(providers.test ? { testRaw: providers.test.bind(providers) } : {}) };
}

// ---------------------------------------------------------------------------
// Drawer form atoms — the shared .form-item/.form-label anatomy from the Vue
// SettingDrawer forms (label-align top, red * for required fields).
// ---------------------------------------------------------------------------

// Drawer form atoms — Vue SettingDrawer 抽屉表单解剖（label-align top、
// required 星号前置；StorageBackendSettings.vue:115-121 / VectorStoreSettings.vue
// 同款约定）：label 与控件为兄弟节点，desc 走 .form-desc（--warn 红字）。
function FormItem({ label, required, children, desc, warn }: { label: ReactNode; required?: boolean; children: ReactNode; desc?: ReactNode; warn?: boolean }) {
  return <div className="form-item">
    <label className={'form-label' + (required ? ' required' : '')}>{label}</label>
    {children}
    {desc ? <p className={'form-desc' + (warn ? ' form-desc--warn' : '')}>{desc}</p> : null}
  </div>;
}

function DrawerSection({ title, children }: { title: ReactNode; children: ReactNode }) {
  return <section className="setting-drawer__section">
    <h4 className="setting-drawer__section-title">{title}</h4>
    {children}
  </section>;
}

/** Vue number-text proxy semantics: empty input deletes the key, numbers coerce. */
function setBagValue(setter: React.Dispatch<React.SetStateAction<Record<string, unknown>>>, name: string, value: unknown) {
  setter((current) => {
    if (value === undefined) {
      if (!(name in current)) return current;
      const next = { ...current };
      delete next[name];
      return next;
    }
    return { ...current, [name]: value };
  });
}

// A connection/index field renderer shared by the vector store create form —
// mirrors VectorStoreSettings.vue L243-343 (boolean → switch + TLS warning,
// sensitive → password, number → number input, enum → select, string → input).
function VectorSchemaField({ field, value, label, onChange, tlsWarning }: { field: VectorFieldSchema; value: unknown; label: string; onChange: (value: unknown) => void; tlsWarning?: string }) {
  // Vue VectorStoreSettings.vue:246-283：boolean → .vision-toggle 包 switch +
  // 条件 form-desc--warn 红字兄弟节点；敏感 string → password + lock-on
  // prefix；number → type=number + .number-input（去原生 spinner）。
  if (field.type === 'boolean') {
    return <div className="form-item">
      <label className={'form-label' + (field.required ? ' required' : '')}>{label}</label>
      <div className="vision-toggle">
        <TSwitch value={value === true} onChange={(checked) => onChange(Boolean(checked))} />
      </div>
      {field.name === 'insecure_skip_verify' && value === true ? <p className="form-desc form-desc--warn">{tlsWarning}</p> : null}
    </div>;
  }
  if (field.type === 'number') {
    const raw = value == null || value === '' ? '' : String(value);
    return <FormItem label={label} required={field.required}>
      <TInput
        type="number"
        className="number-input"
        value={raw}
        placeholder={field.default != null ? String(field.default) : ' '}
        onChange={(value) => {
          const text = String(value).trim();
          if (!text) { onChange(undefined); return; }
          const parsed = Number(text);
          onChange(Number.isFinite(parsed) ? parsed : text);
        }}
      />
    </FormItem>;
  }
  if (field.enum && field.enum.length > 0) {
    return <FormItem label={label} required={field.required}>
      <TSelect value={typeof value === 'string' ? value : ''} options={field.enum.map((option) => ({ value: option, label: option }))} onChange={(value) => onChange(String(value))} />
    </FormItem>;
  }
  if (field.sensitive) {
    return <FormItem label={label} required={field.required}>
      <TInput
        type="password"
        value={typeof value === 'string' ? value : ''}
        placeholder="********"
        maxlength={128}
        prefixIcon={<TIcon name="lock-on" />}
        onChange={(value) => onChange(String(value))}
      />
    </FormItem>;
  }
  return <FormItem label={label} required={field.required}>
    <TInput
      value={typeof value === 'string' ? value : ''}
      placeholder={field.default?.toString() || ''}
      maxlength={128}
      onChange={(value) => onChange(String(value))}
    />
  </FormItem>;
}

export function ResourceSettingsPanel({ client, section, initialValue, role = 'owner' }: { client: WeKnoraClient; section: ResourceSection; initialValue: unknown; role?: string }) {
  const api = apiFor(client, section);
  const locale = readInitialLocale();
  const t = settingsT(locale);
  const copy = RESOURCE_COPY[locale];
  // Vue authStore.hasRole('admin')：三域卡可点/菜单/add 卡的统一守卫。
  const isAdmin = roleAtLeast(role as SettingsRole, 'admin');
  // Per-section i18n keys ported from the Vue settings editors.
  const keys = section === 'storage' ? {
    add: 'settings.storageBackend.createTitle', edit: 'settings.storageBackend.editTitle',
    name: 'settings.storageBackend.nameLabel',
    created: 'settings.storageBackend.saveSuccess', updated: 'settings.storageBackend.saveSuccess', list: 'settings.storage.title',
    saveFailed: 'settings.storageBackend.saveFailed', deleteFailed: 'settings.storageBackend.deleteFailed',
    deleteConfirm: 'settings.storageBackend.deleteConfirm', deleted: 'settings.storageBackend.deleted',
    test: 'settings.storageBackend.testConnection', testFailed: 'settings.storageBackend.testFailed', testSuccess: 'settings.storageBackend.testSuccess',
    setDefault: 'settings.storageBackend.setDefault', empty: 'settings.storageBackend.empty',
  } : section === 'vectorstore' ? {
    add: 'vectorStoreSettings.addStore', edit: 'vectorStoreSettings.editStore',
    name: 'vectorStoreSettings.nameLabel',
    created: 'vectorStoreSettings.toasts.storeCreated', updated: 'vectorStoreSettings.toasts.storeUpdated', list: 'vectorStoreSettings.storesTitle',
    saveFailed: 'vectorStoreSettings.toasts.errorGeneric', deleteFailed: 'vectorStoreSettings.toasts.errorGeneric',
    deleteConfirm: 'vectorStoreSettings.deleteConfirm', deleted: 'vectorStoreSettings.toasts.storeDeleted',
    test: 'vectorStoreSettings.testConnection', testFailed: 'vectorStoreSettings.toasts.testFailed', testSuccess: 'vectorStoreSettings.toasts.testSuccess',
    setDefault: 'settings.storageBackend.setDefault', empty: 'vectorStoreSettings.emptyDesc',
  } : {
    add: 'webSearchSettings.addProvider', edit: 'webSearchSettings.editProvider',
    name: 'webSearchSettings.providerNameLabel',
    created: 'webSearchSettings.toasts.providerCreated', updated: 'webSearchSettings.toasts.providerUpdated', list: 'webSearchSettings.providersTitle',
    saveFailed: 'webSearchSettings.toasts.errorGeneric', deleteFailed: 'webSearchSettings.toasts.errorGeneric',
    deleteConfirm: 'webSearchSettings.deleteConfirm', deleted: 'webSearchSettings.toasts.providerDeleted',
    test: 'webSearchSettings.testConnection', testFailed: 'webSearchSettings.toasts.testFailed', testSuccess: 'webSearchSettings.toasts.testSuccess',
    setDefault: 'webSearchSettings.setAsDefault', empty: 'webSearchSettings.noProvidersDesc',
  };
  const [rows, setRows] = useState<ResourceRow[]>(() => settingsResourceRows(initialValue, section));
  const [defaultId, setDefaultId] = useState<string | undefined>(undefined);
  // Engine type metadata — one per section shape (vector: field schemas,
  // storage: provider ids, websearch: capability flags).
  const [vectorTypes, setVectorTypes] = useState<VectorStoreTypeInfo[]>([]);
  const [storageProviders, setStorageProviders] = useState<string[]>([]);
  const [webTypes, setWebTypes] = useState<WebSearchTypeInfo[]>([]);
  const [providerTypes, setProviderTypes] = useState<Array<{ id: string; name: string }>>([]);
  // Drawer state — the structured fields of the Vue drawers (R482 B3-D1).
  const [name, setName] = useState('');
  const [type, setType] = useState('');
  const [connectionConfig, setConnectionConfig] = useState<Record<string, unknown>>({});
  const [indexConfig, setIndexConfig] = useState<Record<string, unknown>>({});
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [storageConfig, setStorageConfig] = useState<StorageBackendConfig>(storageBlankConfig);
  const [description, setDescription] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [engineId, setEngineId] = useState('');
  const [baseUrl, setBaseUrl] = useState('');
  const [proxyUrl, setProxyUrl] = useState('');
  const [extraConfig, setExtraConfig] = useState<Record<string, string>>({});
  const [isDefault, setIsDefault] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editingRow, setEditingRow] = useState<ResourceRow | null>(null);
  // Websearch edit-mode credential card (Vue CredentialResource): the api_key
  // is an independent /credentials subresource with its own committed state —
  // configured / unconfigured / editing / inline confirm-remove.
  const [credentialConfigured, setCredentialConfigured] = useState(false);
  const [credentialStep, setCredentialStep] = useState<'idle' | 'editing' | 'confirm-remove'>('idle');
  const [credentialDraft, setCredentialDraft] = useState('');
  const [credentialBusy, setCredentialBusy] = useState<'save' | 'remove' | null>(null);
  const [credentialFlashRemoved, setCredentialFlashRemoved] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [testing, setTesting] = useState(false);
  const [lastTestOk, setLastTestOk] = useState<boolean | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => { setRows(settingsResourceRows(initialValue, section)); }, [initialValue, section]);

  // Clear the credential card's removed-flash timer on unmount so it never
  // writes into a torn-down component (Vue onBeforeUnmount equivalent).
  const flashTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => () => { if (flashTimerRef.current) clearTimeout(flashTimerRef.current); }, []);

  // The 默认 tag needs the list envelope's default id (storage section), which
  // the parent's prefetched initialValue does not carry — refetch on mount.
  useEffect(() => { void loadRows().catch(() => undefined); }, []);

  // Engine type metadata drives the structured drawer forms (Vue loads it in
  // onMounted for every section; websearch cards also read display names).
  useEffect(() => {
    if (!api.types) return;
    let active = true;
    void api.types().then((entries) => {
      if (!active) return;
      const list = entries.filter((entry): entry is Record<string, unknown> => Boolean(entry) && typeof entry === 'object' && !Array.isArray(entry));
      setProviderTypes(entries
        .filter((entry): entry is Record<string, unknown> => Boolean(entry) && typeof entry === 'object' && !Array.isArray(entry))
        .map((entry) => ({ id: String(entry.id ?? ''), name: String(entry.name ?? '') }))
        .filter((entry) => entry.id));
      if (section === 'vectorstore') setVectorTypes(parseVectorStoreTypes(entries));
      else if (section === 'storage') setStorageProviders(parseStorageProviders(entries));
      else setWebTypes(parseWebSearchTypes(entries));
    }).catch(() => undefined);
    return () => { active = false; };
  }, [section]);

  // Vue openAddDialog preselects the first engine; types may land after the
  // drawer opens, so backfill the empty selection once they arrive.
  useEffect(() => {
    if (!drawerOpen || editingId || type) return;
    if (section === 'vectorstore' && vectorTypes.length > 0) setType(vectorTypes[0]!.type);
    else if (section === 'storage' && storageProviders.length > 0) setType(storageProviders[0]!);
    else if (section === 'websearch' && webTypes.length > 0) setType(webTypes[0]!.id);
  }, [drawerOpen, editingId, type, section, vectorTypes, storageProviders, webTypes]);

  async function loadRows() {
    if (api.listEnvelope) {
      const envelope = await api.listEnvelope();
      setRows([...envelope.rows]);
      setDefaultId(envelope.defaultId);
      return;
    }
    setRows([...await api.list()]);
  }

  function resetConnectionHint() { setLastTestOk(null); }

  function clearForm() {
    setEditingId(null); setEditingRow(null); setName(''); setType('');
    setConnectionConfig({}); setIndexConfig({}); setShowAdvanced(false);
    setStorageConfig(storageBlankConfig());
    setDescription(''); setApiKey(''); setEngineId(''); setBaseUrl(''); setProxyUrl(''); setExtraConfig({});
    setIsDefault(false); setLastTestOk(null);
    setCredentialConfigured(false); setCredentialStep('idle'); setCredentialDraft('');
    setCredentialBusy(null); setCredentialFlashRemoved(false);
    if (flashTimerRef.current) { clearTimeout(flashTimerRef.current); flashTimerRef.current = null; }
  }

  function openCreate() {
    clearForm();
    if (section === 'vectorstore') setType(vectorTypes[0]?.type || '');
    else if (section === 'storage') setType(storageProviders[0] || 'local');
    else if (section === 'websearch') {
      const first = webTypes[0]?.id || 'duckduckgo';
      setType(first);
      setIsDefault(rows.length === 0);
    }
    setError(null); setNotice(null); setDrawerOpen(true);
  }

  function edit(row: ResourceRow) {
    const id = rowId(row);
    setEditingId(id); setEditingRow(row);
    setName(rowText(row, 'name'));
    setError(null); setNotice(null); setLastTestOk(null);
    if (section === 'vectorstore') {
      setType(rowText(row, 'engine_type'));
      setConnectionConfig({ ...rowRecord(row, 'connection_config') });
      setIndexConfig({ ...rowRecord(row, 'index_config') });
      setShowAdvanced(false);
    } else if (section === 'storage') {
      setType(rowText(row, 'provider') || 'local');
      setStorageConfig({ ...storageBlankConfig(), ...rowRecord(row, 'config') } as StorageBackendConfig);
    } else {
      setType(rowText(row, 'provider'));
      setDescription(rowText(row, 'description'));
      const parameters = rowRecord(row, 'parameters');
      // Never prefill the api_key — "non-empty means the user typed it" (Vue).
      setApiKey('');
      setEngineId(typeof parameters.engine_id === 'string' ? parameters.engine_id : '');
      setBaseUrl(typeof parameters.base_url === 'string' ? parameters.base_url : '');
      setProxyUrl(typeof parameters.proxy_url === 'string' ? parameters.proxy_url : '');
      const storedExtra = rowRecord(parameters, 'extra_config');
      setExtraConfig(Object.fromEntries(Object.entries(storedExtra).filter((entry): entry is [string, string] => typeof entry[1] === 'string')));
      setIsDefault(row.is_default === true);
      // Credential card seed: configured? rides on the main row payload
      // (row.credentials.api_key.configured — Vue credentialMeta L421).
      setCredentialConfigured(rowRecord(rowRecord(row, 'credentials'), 'api_key').configured === true);
      setCredentialStep('idle'); setCredentialDraft(''); setCredentialBusy(null); setCredentialFlashRemoved(false);
    }
    setDrawerOpen(true);
  }

  function closeDrawer() {
    setDrawerOpen(false); clearForm();
  }

  async function refresh() {
    setBusy(true); setError(null);
    try { await loadRows(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : copy.loadFailed); }
    finally { setBusy(false); }
  }

  // Vue onEngineTypeChange / resetConfig / onProviderTypeChange — switching the
  // engine wipes the connection fields it parameterizes.
  function onTypeChange(next: string) {
    setType(next); resetConnectionHint();
    if (section === 'vectorstore') { setConnectionConfig({}); setIndexConfig({}); setShowAdvanced(false); }
    else if (section === 'storage') setStorageConfig(storageBlankConfig());
    else {
      const defaults = Object.fromEntries(
        (webTypes.find((entry) => entry.id === next)?.config_fields || [])
          .filter((field) => field.default !== undefined)
          .map((field) => [field.key, field.default as string]),
      );
      setExtraConfig(defaults);
    }
  }

  const selectedVectorType = vectorTypes.find((entry) => entry.type === type) ?? null;
  const selectedWebType = webTypes.find((entry) => entry.id === type) ?? null;

  function validateDrawer(): string | null {
    if (!name.trim()) {
      if (section !== 'websearch') return t(section === 'storage' ? 'settings.storageBackend.nameRequired' : 'vectorStoreSettings.validation.nameRequired');
    }
    if (!type.trim()) return t('vectorStoreSettings.validation.engineTypeRequired');
    if (section === 'vectorstore' && !editingId) {
      for (const field of selectedVectorType?.connection_fields || []) {
        if (!field.required) continue;
        const value = connectionConfig[field.name];
        if (value == null || value === '' || (typeof value === 'string' && value.trim() === '')) {
          return t('vectorStoreSettings.validation.fieldRequired', { field: vectorStoreFieldLabel(t, field.name) });
        }
      }
    }
    return null;
  }

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const validation = validateDrawer();
    if (validation) { setError(validation); return; }
    setBusy(true); setError(null); setNotice(null);
    try {
      let saved: ResourceRow;
      if (section === 'vectorstore') {
        saved = editingId
          ? await api.update(editingId, vectorStoreUpdatePayload(name))
          : await api.create(vectorStoreCreatePayload({ name, engineType: type, connectionConfig, indexConfig, includeIndex: showAdvanced }));
      } else if (section === 'storage') {
        const input = storageBackendPayload(name, type, storageConfig);
        saved = editingId ? await api.update(editingId, input) : await api.create(input);
      } else {
        const fields = { name, provider: type, providerDisplayName: selectedWebType?.name || '', description, apiKey, engineId, baseUrl, proxyUrl, extraConfig, isDefault };
        // Edit mode commits credentials only through the /credentials card
        // (Vue CredentialResource) — the update body never carries api_key,
        // and no PUT /credentials runs on the main save.
        saved = editingId
          ? await api.update(editingId, webSearchUpdatePayload(fields))
          : await api.create(webSearchCreatePayload(fields));
      }
      setRows((current) => editingId ? current.map((row) => rowId(row) === editingId ? saved : row) : [...current, saved]);
      clearForm(); setDrawerOpen(false); setNotice(t(editingId ? keys.updated : keys.created));
    } catch (reason) { setError(reason instanceof Error ? reason.message : t(keys.saveFailed)); }
    finally { setBusy(false); }
  }

  // Test connection mirrors each Vue drawer: vectorstore tests the live form
  // (create only — engine is immutable in edit mode), storage tests the form
  // on create and the stored backend on edit, websearch tests the live form
  // whenever a fresh key is present and falls back to the stored credentials.
  const canTestConnection = section === 'vectorstore'
    ? (!editingId && vectorStoreCanTest(selectedVectorType, connectionConfig))
    : section === 'websearch'
      ? (Boolean(editingId) || webSearchCanTest(selectedWebType, { apiKey, engineId, baseUrl, extraConfig }))
      : true;

  async function testConnection() {
    if (!canTestConnection) return;
    setTesting(true); setError(null); setNotice(null);
    try {
      let result: ResourceTestResult;
      if (section === 'vectorstore') {
        result = await api.testRaw!({ engine_type: type, connection_config: { ...connectionConfig } });
      } else if (section === 'storage') {
        result = editingId
          ? await api.testById(editingId)
          : await api.testRaw!(storageBackendPayload(name, type, storageConfig));
      } else {
        const useStored = Boolean(editingId) && !apiKey;
        if (useStored) {
          result = await api.testById(editingId!);
        } else {
          const parameters: Record<string, unknown> = {
            engine_id: engineId,
            base_url: baseUrl,
            proxy_url: proxyUrl,
            extra_config: { ...extraConfig },
            ...(apiKey ? { api_key: apiKey } : {}),
          };
          result = await api.testRaw!({ provider: type, parameters });
        }
      }
      setLastTestOk(result.success);
      if (!result.success) throw new Error(result.error || t(keys.testFailed));
      setNotice(result.message || t(keys.testSuccess));
    } catch (reason) { setLastTestOk(false); setError(reason instanceof Error ? reason.message : t(keys.testFailed)); }
    finally { setTesting(false); }
  }

  async function remove(id: string) {
    if (!id || !window.confirm(t(keys.deleteConfirm))) return;
    setBusy(true); setError(null); setNotice(null);
    try { await api.remove(id); setRows((current) => current.filter((row) => rowId(row) !== id)); if (editingId === id) { clearForm(); setDrawerOpen(false); } setNotice(t(keys.deleted)); }
    catch (reason) { setError(reason instanceof Error ? reason.message : t(keys.deleteFailed)); }
    finally { setBusy(false); }
  }

  async function setDefault(id: string) {
    if (!api.setDefault) return;
    setBusy(true); setError(null); setNotice(null);
    try { await api.setDefault(id); setNotice(t('settings.storageBackend.defaultUpdated')); await refresh(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : copy.setDefaultFailed); setBusy(false); }
  }

  // Vue storage 卡菜单「测试连接」（handleMenuAction 'test' → testSaved）：
  // 用存储配置测，成功/失败走面板内 notice/error 行（StorageBackendSettings.vue:315）。
  async function testSavedCard(id: string) {
    if (!id) return;
    setBusy(true); setError(null); setNotice(null);
    try {
      const result = await api.testById(id);
      if (!result.success) throw new Error(result.error || t(keys.testFailed));
      setNotice(result.message || t(keys.testSuccess));
    } catch (reason) { setError(reason instanceof Error ? reason.message : t(keys.testFailed)); }
    finally { setBusy(false); }
  }

  // Vue CredentialResource actions (frontend/src/components/credentials/
  // CredentialResource.vue). Every commit is an explicit call to the
  // /credentials subresource — never piggybacked on the main drawer save.
  function enterCredentialEdit() {
    setCredentialDraft(''); setCredentialStep('editing');
  }

  function cancelCredentialEdit() {
    setCredentialDraft(''); setCredentialStep('idle');
  }

  async function saveCredential() {
    if (!editingId || !credentialDraft.trim() || !api.putCredentials) return;
    setCredentialBusy('save'); setError(null);
    try {
      const result = await api.putCredentials(editingId, { api_key: credentialDraft });
      const fields = rowRecord(result && typeof result === 'object' ? result as ResourceRow : {}, 'fields');
      setCredentialConfigured(rowRecord(fields, 'api_key').configured === true);
      setCredentialDraft(''); setCredentialStep('idle');
      setNotice(t('common.saveSuccess'));
    } catch (reason) { setError(reason instanceof Error ? reason.message : t('dataSource.credential.saveFailed')); }
    finally { setCredentialBusy(null); }
  }

  async function removeCredential() {
    if (!editingId || !api.deleteCredential) return;
    setCredentialBusy('remove'); setError(null);
    try {
      await api.deleteCredential(editingId, 'api_key');
      setCredentialConfigured(false); setCredentialStep('idle');
      // Anchored inline flash (Vue flashInlineToast ~2.4s) instead of a
      // global toast — the row is where the user just clicked.
      setCredentialFlashRemoved(true);
      if (flashTimerRef.current) clearTimeout(flashTimerRef.current);
      flashTimerRef.current = setTimeout(() => { flashTimerRef.current = null; setCredentialFlashRemoved(false); }, 2400);
    } catch (reason) { setError(reason instanceof Error ? reason.message : t('dataSource.credential.removeFailed')); }
    finally { setCredentialBusy(null); }
  }

  // Per-section card meta, mirroring the Vue surfaces: storage cards show
  // endpoint → bucket_name → path_prefix → 本地存储 (backendMeta), vectorstore
  // cards show the store endpoint, websearch cards the provider description.
  const resourceMeta = (row: ResourceRow): string => {
    const config = row.config && typeof row.config === 'object' && !Array.isArray(row.config) ? row.config as Record<string, unknown> : {};
    if (section === 'storage') {
      for (const key of ['endpoint', 'bucket_name', 'path_prefix']) {
        const value = config[key];
        if (typeof value === 'string' && value.trim()) return value;
      }
      return copy.localLabel;
    }
    if (section === 'vectorstore') {
      // Vue getStoreEndpoint reads connection_config.addr || .host
      // (VectorStoreSettings.vue L505-508) — endpoint is not consulted.
      const endpoint = config.addr || config.host;
      return typeof endpoint === 'string' ? endpoint : '';
    }
    return rowText(row, 'description');
  };
  // Vue renders the raw engine_type on vectorstore/websearch cards and the
  // upper-cased provider on storage cards (LOCAL · 本地存储).
  const providerLabel = (provider: string) => (section === 'storage' ? provider.toUpperCase() : provider);
  // Vue providerTypeLabel (WebSearchSettings.vue:486): the card subtitle shows
  // the provider type's display name from /web-search-providers/types.
  const providerTypeLabel = (providerId: string) => providerTypes.find((entry) => entry.id === providerId)?.name || providerId;
  // Vue VectorStoreSettings marks .env-sourced stores with a DEFAULT pill.
  // Vue .store-card__pill: no border, warning-tint bg/text (L1019-1028).
  const envPill = (row: ResourceRow) => row.source === 'env' ? (
    <span className="rs-env-pill">{t('vectorStoreSettings.envTag')}</span>
  ) : null;
  // vectorstore/websearch panels keep an inner list title (storesTitle /
  // providersTitle); storage's card grid sits directly under the section
  // header in Vue. Vue has no refresh affordance on these lists — the
  // heading is a plain .list-section-title (16px/600, mb 16).
  const innerListTitle = section === 'storage' ? null : (
    <h3 className="list-section-title">{t(keys.list)}</h3>
  );

  // -------------------------------------------------------------------------
  // Structured drawer forms (R482 B3-D1 / R485 H2). Each mirrors the matching
  // Vue drawer field-for-field; no fields are invented.
  // -------------------------------------------------------------------------

  const vectorCreateFields = selectedVectorType?.connection_fields ?? [];
  const vectorIndexFields = selectedVectorType?.index_fields ?? [];

  const vectorDrawerBody = editingId ? (
    <>
      <DrawerSection title={t('vectorStoreSettings.basicSection')}>
        <div className="inline-alert inline-alert--info">
          <TIcon name="info-circle-filled" className="inline-alert__icon" />
          <span className="inline-alert__text">{t('vectorStoreSettings.immutableNotice')}</span>
        </div>
        <FormItem label={t('vectorStoreSettings.nameLabel')} required>
          <TInput value={name} placeholder={t('vectorStoreSettings.namePlaceholder')} onChange={(value) => setName(String(value))} />
        </FormItem>
        <div className="readonly-fields">
          <div className="readonly-row">
            <span className="readonly-label">{t('vectorStoreSettings.engineTypeLabel')}</span>
            <span className="readonly-value">{selectedVectorType?.display_name || type}</span>
          </div>
          {(selectedVectorType?.connection_fields ?? []).map((field) => {
            const value = connectionConfig[field.name];
            if (!field.sensitive && (value == null || value === '')) return null;
            return <div key={field.name} className="readonly-row">
              <span className="readonly-label">{vectorStoreFieldLabel(t, field.name)}</span>
              <span className="readonly-value">{field.sensitive ? '********' : String(value)}</span>
            </div>;
          })}
          {(selectedVectorType?.index_fields ?? []).map((field) => {
            const value = indexConfig[field.name];
            if (value == null || value === '') return null;
            return <div key={field.name} className="readonly-row">
              <span className="readonly-label">{vectorStoreFieldLabel(t, field.name)}</span>
              <span className="readonly-value">{String(value)}</span>
            </div>;
          })}
        </div>
      </DrawerSection>
    </>
  ) : (
    <>
      <DrawerSection title={t('vectorStoreSettings.basicSection')}>
        <FormItem label={t('vectorStoreSettings.engineTypeLabel')} required>
          <TSelect value={type} options={vectorTypes.map((entry) => ({ value: entry.type, label: entry.display_name }))} onChange={(value) => onTypeChange(String(value))} />
        </FormItem>
        <FormItem label={t('vectorStoreSettings.nameLabel')} required>
          <TInput value={name} placeholder={t('vectorStoreSettings.namePlaceholder')} onChange={(value) => { setName(String(value)); resetConnectionHint(); }} />
        </FormItem>
      </DrawerSection>
      {selectedVectorType ? <DrawerSection title={t('vectorStoreSettings.connectionInfo')}>
        {vectorCreateFields.map((field) => (
          <VectorSchemaField
            key={field.name}
            field={field}
            value={connectionConfig[field.name]}
            label={vectorStoreFieldLabel(t, field.name)}
            tlsWarning={t('vectorStoreSettings.insecureSkipVerifyWarning')}
            onChange={(next) => { setBagValue(setConnectionConfig, field.name, next); resetConnectionHint(); }}
          />
        ))}
      </DrawerSection> : null}
      {vectorIndexFields.length > 0 ? <DrawerSection title={t('vectorStoreSettings.advancedIndexConfig')}>
        <button type="button" className="advanced-toggle" onClick={() => setShowAdvanced(!showAdvanced)}>
          <TIcon name={showAdvanced ? 'chevron-down' : 'chevron-right'} />
          <span>{showAdvanced ? copy.collapse : copy.expand}</span>
        </button>
        {showAdvanced ? vectorIndexFields.map((field) => (
          <VectorSchemaField
            key={field.name}
            field={field}
            value={indexConfig[field.name]}
            label={vectorStoreFieldLabel(t, field.name)}
            onChange={(next) => setBagValue(setIndexConfig, field.name, next)}
          />
        )) : null}
      </DrawerSection> : null}
    </>
  );

  const needsEndpoint = storageNeedsEndpoint(type, storageConfig.mode);
  const needsRegion = storageNeedsRegion(type);
  const needsCredentials = storageNeedsCredentials(type, storageConfig.mode);
  const storageDisabled = Boolean(editingId); // Vue locks every connection field in edit mode
  const storageDrawerBody = (
    <>
      <DrawerSection title={t('settings.storageBackend.basicSection')}>
        <FormItem label={t('settings.storageBackend.nameLabel')} required>
          <TInput value={name} clearable placeholder={t('settings.storageBackend.namePlaceholder')} onChange={(value) => setName(String(value))} />
        </FormItem>
        <FormItem label={t('settings.storageBackend.providerLabel')} required>
          <TSelect value={type} disabled={storageDisabled} options={storageProviders.map((provider) => ({ value: provider, label: provider.toUpperCase() }))} onChange={(value) => onTypeChange(String(value))} />
        </FormItem>
        {type === 'minio' ? <FormItem label={t('settings.storageBackend.modeLabel')}>
          <div className="source-options" role="radiogroup">
            <button type="button" disabled={storageDisabled} className={'source-option' + (storageConfig.mode !== 'docker' ? ' is-active' : '')} onClick={() => setStorageConfig({ ...storageConfig, mode: 'remote' })}>
              <TIcon name="cloud" className="source-option__icon" />
              <span className="source-option__label">{t('settings.storageBackend.modeRemote')}</span>
            </button>
            <button type="button" disabled={storageDisabled} className={'source-option' + (storageConfig.mode === 'docker' ? ' is-active' : '')} onClick={() => setStorageConfig({ ...storageConfig, mode: 'docker' })}>
              <TIcon name="server" className="source-option__icon" />
              <span className="source-option__label">{t('settings.storageBackend.modeEnv')}</span>
            </button>
          </div>
        </FormItem> : null}
      </DrawerSection>
      <DrawerSection title={t('settings.storageBackend.connectionSection')}>
        {/* Vue storage 连接字段全家 clearable + 凭据双字段 lock-on prefix
            （StorageBackendSettings.vue:159-193）。 */}
        {needsEndpoint ? <FormItem label="Endpoint" required>
          <TInput value={storageConfig.endpoint} disabled={storageDisabled} clearable placeholder={type === 'minio' ? 'storage.example.com:9000' : 'https://storage.example.com'} onChange={(value) => setStorageConfig({ ...storageConfig, endpoint: String(value) })} />
        </FormItem> : null}
        {needsRegion ? <FormItem label="Region" required>
          <TInput value={storageConfig.region} disabled={storageDisabled} clearable onChange={(value) => setStorageConfig({ ...storageConfig, region: String(value) })} />
        </FormItem> : null}
        {needsCredentials ? <>
          {/* Vue 凭据双字段编辑态可改（密钥轮换入口，无 :disabled——
              StorageBackendSettings.vue:164-178），仅 endpoint/region/bucket/
              appid/path_prefix 编辑态锁定。 */}
          <FormItem label="Access Key / Secret ID" required>
            <TInput value={storageConfig.access_key_id} clearable placeholder="***" prefixIcon={<TIcon name="lock-on" />} onChange={(value) => setStorageConfig({ ...storageConfig, access_key_id: String(value) })} />
          </FormItem>
          <FormItem label="Secret Key" required>
            <TInput type="password" value={storageConfig.secret_access_key} clearable placeholder="***" prefixIcon={<TIcon name="lock-on" />} onChange={(value) => setStorageConfig({ ...storageConfig, secret_access_key: String(value) })} />
          </FormItem>
        </> : null}
        {type !== 'local' ? <FormItem label="Bucket" required>
          <TInput value={storageConfig.bucket_name} disabled={storageDisabled} clearable onChange={(value) => setStorageConfig({ ...storageConfig, bucket_name: String(value) })} />
        </FormItem> : null}
        {type === 'cos' ? <FormItem label="App ID">
          <TInput value={storageConfig.app_id || ''} disabled={storageDisabled} clearable placeholder={t('settings.storageBackend.optionalPlaceholder')} onChange={(value) => setStorageConfig({ ...storageConfig, app_id: String(value) })} />
        </FormItem> : null}
      </DrawerSection>
      <DrawerSection title={t('settings.storageBackend.advancedSection')}>
        <FormItem label={t('settings.storageBackend.pathPrefixLabel')}>
          <TInput value={storageConfig.path_prefix} disabled={storageDisabled} clearable placeholder="weknora/" onChange={(value) => setStorageConfig({ ...storageConfig, path_prefix: String(value) })} />
        </FormItem>
        {/* Vue switch 行包一层 .form-item（StorageBackendSettings.vue:203-220）。 */}
        {type === 'minio' ? <div className="form-item"><div className="vision-toggle">
          <TSwitch value={storageConfig.use_ssl} onChange={(checked) => setStorageConfig({ ...storageConfig, use_ssl: Boolean(checked) })} />
          <span className="form-desc form-desc--inline">{t('settings.storageBackend.useSslDesc')}</span>
        </div></div> : null}
        {type === 's3' ? <div className="form-item"><div className="vision-toggle">
          <TSwitch value={storageConfig.force_path_style === true} onChange={(checked) => setStorageConfig({ ...storageConfig, force_path_style: Boolean(checked) })} />
          <span className="form-desc form-desc--inline">{t('settings.storageBackend.forcePathStyleDesc')}</span>
        </div></div> : null}
        {type === 'oss' ? <div className="form-item"><div className="vision-toggle">
          <TSwitch value={storageConfig.use_temp_bucket === true} onChange={(checked) => setStorageConfig({ ...storageConfig, use_temp_bucket: Boolean(checked) })} />
          <span className="form-desc form-desc--inline">{t('settings.storageBackend.useTempBucketDesc')}</span>
        </div></div> : null}
        {['cos', 'tos'].includes(type) || (type === 'oss' && storageConfig.use_temp_bucket) ? <>
          <FormItem label={t('settings.storageBackend.tempBucketLabel')}>
            <TInput value={storageConfig.temp_bucket_name || ''} clearable placeholder={t('settings.storageBackend.tempBucketPlaceholder')} onChange={(value) => setStorageConfig({ ...storageConfig, temp_bucket_name: String(value) })} />
          </FormItem>
          <FormItem label={t('settings.storageBackend.tempRegionLabel')}>
            <TInput value={storageConfig.temp_region || ''} clearable placeholder={t('settings.storageBackend.tempRegionPlaceholder')} onChange={(value) => setStorageConfig({ ...storageConfig, temp_region: String(value) })} />
          </FormItem>
        </> : null}
      </DrawerSection>
    </>
  );

  const webCredentialFields = selectedWebType?.requires_api_key || selectedWebType?.supports_optional_api_key || selectedWebType?.requires_engine_id || selectedWebType?.requires_base_url || (selectedWebType?.config_fields?.length ?? 0) > 0;
  const webDrawerBody = (
    <>
      {/* Vue WebSearchSettings 的 provider 名 + 查看文档外链在 SettingDrawer
          #subtitle 槽（L121-132），不是 body 头段——见 drawerSubtitleNode。 */}
      <DrawerSection title={t('webSearchSettings.basicSection')}>
        <FormItem label={t('webSearchSettings.providerTypeLabel')} required>
          <TSelect value={type} disabled={Boolean(editingId)} options={webTypes.map((entry) => ({ value: entry.id, label: entry.name }))} onChange={(value) => onTypeChange(String(value))} />
        </FormItem>
        <FormItem label={t('webSearchSettings.providerNameLabel')}>
          <TInput value={name} placeholder={selectedWebType?.name || t('webSearchSettings.providerNamePlaceholder')} onChange={(value) => { setName(String(value)); resetConnectionHint(); }} />
        </FormItem>
        <FormItem label={t('webSearchSettings.providerDescLabel')}>
          <TInput value={description} placeholder={t('webSearchSettings.providerDescPlaceholder')} onChange={(value) => setDescription(String(value))} />
        </FormItem>
      </DrawerSection>
      {webCredentialFields ? <DrawerSection title={t('webSearchSettings.credentialsSection')}>
        {selectedWebType?.requires_base_url ? <FormItem label={t('webSearchSettings.baseUrlLabel')} required>
          <TInput value={baseUrl} placeholder={t('webSearchSettings.baseUrlPlaceholder')} onChange={(value) => { setBaseUrl(String(value)); resetConnectionHint(); }} />
        </FormItem> : null}
        {selectedWebType?.requires_api_key || selectedWebType?.supports_optional_api_key ? (
          <FormItem label={selectedWebType?.supports_optional_api_key && !selectedWebType?.requires_api_key ? t('webSearchSettings.apiKeyOptionalLabel') : t('webSearchSettings.apiKeyLabel')} required={selectedWebType?.requires_api_key}>
            {editingId ? (
              // Edit mode — Vue CredentialResource（frontend/src/components/
              // credentials/CredentialResource.vue）：api_key 是独立 /credentials
              // 子资源，configured/unconfigured/editing 三态卡 + 两步移除内联
              // 确认，提交走显式 PUT/DELETE，不搭主表单 save。类名与 Vue 逐字
              // 同构（credential-faux-input 家族），data-kind 为测试锚。
              credentialStep === 'editing' ? (
                <div className="credential-edit" data-kind="editing">
                  <TInput
                    type="password"
                    value={credentialDraft}
                    placeholder={t('dataSource.credential.inputPlaceholder')}
                    prefixIcon={<TIcon name="lock-on" />}
                    onChange={(value) => setCredentialDraft(String(value))}
                    onKeydown={(_, context) => { if (context.e.key === 'Enter') { context.e.preventDefault(); void saveCredential(); } }}
                  />
                  <div className="credential-edit-actions">
                    <TButton size="small" variant="text" disabled={credentialBusy !== null} onClick={cancelCredentialEdit}>{t('common.cancel')}</TButton>
                    <TButton size="small" theme="primary" loading={credentialBusy === 'save'} disabled={!credentialDraft} onClick={() => void saveCredential()}>{t('common.save')}</TButton>
                  </div>
                </div>
              ) : credentialConfigured ? (
                credentialStep === 'confirm-remove' ? (
                  <div data-kind="confirm-remove" className="credential-faux-input is-confirm-remove">
                    <TIcon name="error-circle-filled" className="status-icon warn" />
                    <span className="credential-faux-text danger">{t('dataSource.credential.confirmRemovePrompt')}</span>
                    <div className="credential-actions">
                      <TButton size="small" variant="text" disabled={credentialBusy !== null} onClick={() => setCredentialStep('idle')}>{t('common.cancel')}</TButton>
                      <span className="action-divider" />
                      <TButton size="small" variant="text" theme="danger" loading={credentialBusy === 'remove'} onClick={() => void removeCredential()}>{t('dataSource.credential.confirmRemove')}</TButton>
                    </div>
                  </div>
                ) : (
                  <div data-kind="configured" className="credential-faux-input" title={t('dataSource.credential.configured')}>
                    <TIcon name="check-circle-filled" className="status-icon success" />
                    <span className="credential-faux-text">{t('dataSource.credential.configured')}</span>
                    <div className="credential-actions">
                      <TButton size="small" variant="text" onClick={enterCredentialEdit}>{t('dataSource.credential.update')}</TButton>
                      <span className="action-divider" />
                      <TButton size="small" variant="text" theme="danger" onClick={() => setCredentialStep('confirm-remove')}>{t('dataSource.credential.remove')}</TButton>
                    </div>
                  </div>
                )
              ) : (
                <div data-kind="unconfigured" className={'credential-faux-input is-empty' + (credentialFlashRemoved ? ' is-just-removed' : '')} onClick={credentialFlashRemoved ? undefined : enterCredentialEdit}>
                  {credentialFlashRemoved ? <>
                    <TIcon name="check-circle-filled" className="status-icon success" />
                    <span className="credential-faux-text">{t('dataSource.credential.removedToast')}</span>
                  </> : <>
                    <TIcon name="lock-on" className="status-icon muted" />
                    <span className="credential-faux-text muted">{t('dataSource.credential.unconfigured')}</span>
                    <div className="credential-actions">
                      <TButton size="small" variant="text" theme="primary" onClick={(event) => { event.stopPropagation(); enterCredentialEdit(); }}>{t('dataSource.credential.configure')}</TButton>
                    </div>
                  </>}
                </div>
              )
            ) : (
              // Create mode keeps the plain password input (Vue L239-246) —
              // the key rides the initial POST only.
              <TInput type="password" value={apiKey} placeholder={t('webSearchSettings.apiKeyPlaceholder')} onChange={(value) => { setApiKey(String(value)); resetConnectionHint(); }} />
            )}
          </FormItem>
        ) : null}
        {selectedWebType?.requires_engine_id ? <FormItem label={t('webSearchSettings.engineIdLabel')} required>
          <TInput value={engineId} placeholder={t('webSearchSettings.engineIdLabel')} onChange={(value) => { setEngineId(String(value)); resetConnectionHint(); }} />
        </FormItem> : null}
        {(selectedWebType?.config_fields ?? []).map((field) => (
          <FormItem key={field.key} label={webSearchConfigText(t, field.label_key, field.label)} required={field.required} desc={field.description ? webSearchConfigText(t, field.description_key, field.description) : undefined}>
            <TSelect value={extraConfig[field.key] ?? field.default ?? ''} options={(field.options.length > 0 ? field.options : [{ label: field.default || '', value: field.default || '' }]).map((option) => ({ value: option.value, label: webSearchConfigText(t, option.label_key, option.label) }))} onChange={(value) => setExtraConfig({ ...extraConfig, [field.key]: String(value) })} />
          </FormItem>
        ))}
      </DrawerSection> : null}
      <DrawerSection title={t('webSearchSettings.optionsSection')}>
        {selectedWebType?.supports_proxy ? <FormItem label={t('webSearchSettings.proxyUrlLabel')}>
          <TInput value={proxyUrl} placeholder={t('webSearchSettings.proxyUrlPlaceholder')} onChange={(value) => setProxyUrl(String(value))} />
          <p className="form-desc">{t('webSearchSettings.proxyUrlHelp')}</p>
        </FormItem> : null}
        <div className="form-item">
          <label className="form-label">{t('webSearchSettings.setAsDefault')}</label>
          <div className="vision-toggle">
            <TSwitch value={isDefault} onChange={(checked) => setIsDefault(Boolean(checked))} />
            <span className="form-desc form-desc--inline">{t('webSearchSettings.setAsDefaultDesc')}</span>
          </div>
        </div>
      </DrawerSection>
    </>
  );

  return <div className="wk-settings-resource" data-resource-section={section}>
    {/* Vue 三个资源面板各自渲染 section-header（VectorStoreSettings.vue L3-6 /
        StorageBackendSettings.vue L3-11 / WebSearchSettings.vue L2-8）— 壳层
        wrapper heading 由 settings-wrapper.css :has(.wk-settings-resource) 隐藏，
        文案沿用 settingsSectionHeading（与壳层 heading 完全一致）。 */}
    <header className="section-header">
      <h2>{settingsSectionHeading(locale, section).title}</h2>
      {settingsSectionHeading(locale, section).description ? <p className="section-description">{settingsSectionHeading(locale, section).description}</p> : null}
    </header>
    {error ? <Status tone="error">{error}</Status> : null}
    {notice ? <Status tone="success">{notice}</Status> : null}
    {innerListTitle}
    <div className="backend-grid">
      {rows.map((row, index) => {
        const id = rowId(row);
        const provider = rowText(row, 'provider') || rowText(row, 'type') || rowText(row, 'engine_type');
        const nameValue = rowText(row, 'name') || id || copy.unnamed;
        const isDefault = row.default === true || (typeof defaultId === 'string' && id === defaultId);
        const meta = resourceMeta(row);
        const logo = providerLogo(section, provider);
        const brand = PROVIDER_BRAND[section][provider.toLowerCase()] ?? { bg: 'rgba(0, 82, 217, 0.1)', color: '#0052D9' };
        // Vue 卡片可点守卫：admin 且（storage/vectorstore）非 env 来源
        // （StorageBackendSettings.vue:299 canEdit / VectorStoreSettings.vue:619
        // isStoreCardClickable / WebSearchSettings.vue:672）。env 卡不可点、
        // 无 role/tabindex —— 点击不打开编辑抽屉（React 曾无条件可点，行为分歧）。
        const envSourced = row.source === 'env';
        const clickable = isAdmin && (section === 'websearch' || !envSourced);
        // Vue 每节卡片家族类：backend-card--{provider} / store-card--{engine} /
        // provider-card--{id} + --clickable 修饰（storage:22-25 / vector:28-34 /
        // websearch:19-25）——扫描触发器与 unscoped 徽章配色锚点。
        const familyClass = section === 'storage'
          ? `backend-card--${provider || 'local'}`
          : section === 'vectorstore'
            ? `store-card store-card--${provider || 'unknown'}`
            : `provider-card provider-card--${provider || 'unknown'}`;
        return <article
          key={id || index}
          role={clickable ? 'button' : undefined}
          tabIndex={clickable ? 0 : undefined}
          className={'backend-card rs-card ' + familyClass
            + (section === 'vectorstore' && envSourced ? ' is-env store-card--env' : '')
            + (clickable ? ' backend-card--clickable store-card--clickable provider-card--clickable' : '')
            + (section === 'storage' ? ' is-storage' : ' is-list')}
          onClick={clickable ? () => edit(row) : undefined}
          onKeyDown={clickable ? (event) => { if (event.key === 'Enter') edit(row); } : undefined}
        >
          {logo ? (
            <div className="backend-card__badge rs-card__badge rs-card__badge--frame" style={{ backgroundColor: brand.bg, color: brand.color }} aria-label={provider}>
              {logo.mode === 'color'
                ? <img src={logo.url} alt="" />
                : <span style={monoLogoMask(logo.url)} aria-hidden="true" />}
            </div>
          ) : (
            <div className="backend-card__badge rs-card__badge rs-card__badge--text" style={{ backgroundColor: brand.bg, color: brand.color }} aria-label={provider}>{providerInitial(provider || nameValue)}</div>
          )}
          <div className="rs-card__main">
            <div className="backend-card__header rs-card__header">
              {/* Vue StorageBackendSettings .backend-card__title: flex:1, lh 1.4,
                  color var(--td-text-color-primary)=rgba(0,0,0,0.9)（store/provider 卡同款） */}
              <h3 className="backend-card__title rs-card__title" title={nameValue}>{nameValue}</h3>
              {section !== 'storage' && row.source === 'env' ? envPill(row) : null}
              {/* Vue t-tag small light：h20/lh20、padding 0 4px、radius 3px，且因标题 flex:1 靠右 */}
              {/* Vue StorageBackendSettings.vue:48 t-tag theme=primary variant=light size=small（直译，playbook §1 #13 tone 换算）。 */}
              {isDefault ? <TTag theme="primary" variant="light" size="small" className="rs-card__tag">{copy.defaultLabel}</TTag> : null}
              {/* Vue websearch 卡 header 尾部对 admin 常驻 provider-card__actions
                 （t-dropdown ellipsis，编辑/删除；WebSearchSettings.vue:44-62）。 */}
              {section === 'websearch' && isAdmin ? <div className="provider-card__actions" onClick={(event) => event.stopPropagation()}>
                <TDropdown
                  options={[
                    { content: t('common.edit'), value: 'edit' },
                    { content: t('common.delete'), value: 'delete', theme: 'error' as never },
                  ]}
                  placement="bottom-right"
                  trigger="click"
                  onClick={(data) => {
                    const value = String(data?.value ?? '');
                    if (value === 'edit') edit(row);
                    else if (value === 'delete') void remove(rowId(row));
                  }}
                >
                  <TButton variant="text" shape="square" size="small" className="provider-card__more" icon={<TIcon name="ellipsis" />} />
                </TDropdown>
              </div> : null}
              {/* Vue 每张 storage 卡尾部都有 opacity:0 的 24px 操作按钮
                  （.backend-card__action-btn，hover 才显形）挂三点菜单
                  （测试连接/设为默认/编辑/删除，StorageBackendSettings.vue:45-56
                  getBackendOptions:305-312）——不可见但参与布局。 */}
              {section === 'storage' ? <div className="backend-card__actions" onClick={(event) => event.stopPropagation()}>
                <TDropdown
                  options={[
                    { content: t('settings.storageBackend.testConnection'), value: 'test' },
                    ...(isAdmin && id !== defaultId ? [{ content: t('settings.storageBackend.setDefault'), value: 'default' }] : []),
                    ...(clickable ? [{ content: t('settings.storageBackend.edit'), value: 'edit' }] : []),
                    ...(isAdmin && !envSourced && !row.legacy_alias ? [{ content: t('settings.storageBackend.delete'), value: 'delete', theme: 'error' as never }] : []),
                  ]}
                  placement="bottom-right"
                  trigger="click"
                  onClick={(data) => {
                    const value = String(data?.value ?? '');
                    if (value === 'test') void testSavedCard(id);
                    else if (value === 'default') void setDefault(id);
                    else if (value === 'edit') edit(row);
                    else if (value === 'delete') void remove(id);
                  }}
                >
                  <TButton variant="text" shape="square" size="small" className="backend-card__action-btn" icon={<TIcon name="ellipsis" />} />
                </TDropdown>
              </div> : null}
            </div>
            <p className={'backend-card__subtitle rs-card__subtitle' + (section === 'storage' ? ' is-storage' : '')}>
              {/* Vue storage 版 type 无加粗（vectorstore .store-card__type 才是 500），
                  sep margin 0 6px、颜色 placeholder token rgba(0,0,0,0.4) */}
              <span className={'rs-card__type' + (section === 'storage' ? '' : ' is-strong')}>{provider ? (section === 'websearch' ? providerTypeLabel(provider) : providerLabel(provider)) : copy.typeUnavailable}</span>
              {meta ? <><span className={'rs-card__sep' + (section === 'storage' ? ' is-storage' : '')}>·</span><span className="rs-card__meta">{meta}</span></> : null}
            </p>
          </div>
        </article>;
      })}
      {/* Vue 三个 add 卡均 admin-only（StorageBackendSettings.vue / VectorStoreSettings.vue:94）。 */}
      {isAdmin ? <button
        type="button"
        className={'backend-card backend-card--add rs-card-add' + (section === 'storage' ? ' is-storage' : ' is-list')}
        onClick={openCreate}
      >
        <span className="rs-card-add__icon" aria-hidden="true"><TIcon name="add" /></span>
        <span className="rs-card-add__label">{t(keys.add)}</span>
      </button> : null}
    </div>
    {/* Vue WebSearchSettings L13: the empty-state hint only renders for
        non-admin viewers; admins get the bare grid, no desc line. */}
    {rows.length === 0 && !(section === 'websearch' && (role === 'owner' || role === 'admin')) ? <Status>{t(keys.empty)}</Status> : null}
    {/* Vue 三个资源抽屉均走 SettingDrawer（StorageBackendSettings.vue:87-243 /
        VectorStoreSettings.vue:109-348 / WebSearchSettings.vue:90-307）：
        自定义 header（#headerIcon 徽章 + 标题 + #subtitle 副标题）+ footer-left
        测试连接 + 取消/保存 默认按钮对。设为默认/删除入口在卡片三点菜单（卡片域）。 */}
    <SettingDrawer
      visible={drawerOpen}
      title={editingId ? t(keys.edit) : t(keys.add)}
      drawerClass={drawerFamilyClass(section, type)}
      confirmLoading={busy}
      headerIcon={section === 'storage'
        ? drawerHeaderIcon(section, type, providerInitial(type))
        : section === 'vectorstore'
          /* Vue engineInitial：raw engine_type 首字母（VectorStoreSettings.vue:511）。 */
          ? (type ? drawerHeaderIcon(section, type, providerInitial(type)) : undefined)
          /* Vue providerInitial 查 providerTypes display name（WebSearchSettings.vue:464-467），
             如 zhipu → 智谱搜索 → 「智」，不是 id 的「Z」。 */
          : (selectedWebType ? drawerHeaderIcon(section, type, providerInitial(providerTypeLabel(type))) : undefined)}
      subtitle={section === 'storage'
        ? <span>{t(editingId ? 'settings.storageBackend.editSubtitle' : 'settings.storageBackend.createSubtitle')}</span>
        : section === 'vectorstore'
          ? (selectedVectorType ? <span>{selectedVectorType.display_name || type}</span> : undefined)
          : (selectedWebType ? <>
            <span>{selectedWebType.name}</span>
            {selectedWebType.docs_url ? <a href={selectedWebType.docs_url} target="_blank" rel="noopener noreferrer" className="doc-link doc-link--inline">
              {t('webSearchSettings.viewDocs')}
              <TIcon name="link" className="link-icon" />
            </a> : null}
          </> : undefined)}
      footerLeft={section !== 'websearch' || selectedWebType ? <TButton variant="outline" loading={testing} disabled={!canTestConnection} onClick={() => void testConnection()}
        icon={!testing && lastTestOk === true ? <TIcon name="check-circle-filled" className="status-icon available" />
          : !testing && lastTestOk === false ? <TIcon name="close-circle-filled" className="status-icon unavailable" /> : undefined}
      >
        {testing ? t(section === 'vectorstore' ? 'vectorStoreSettings.testing' : section === 'websearch' ? 'webSearchSettings.testing' : keys.test) : t(keys.test)}
      </TButton> : undefined}
      onConfirm={() => void save(new Event('submit') as unknown as React.FormEvent<HTMLFormElement>)}
      onCancel={closeDrawer}
      onVisibleChange={(visible) => { if (!visible) closeDrawer(); }}
    >
      {error ? <Status tone="error">{error}</Status> : null}
      {notice ? <Status tone="success">{notice}</Status> : null}
      <form className={section === 'vectorstore' ? 'store-form' : section === 'websearch' ? 'provider-form' : ''} onSubmit={(event) => void save(event)}>
        {section === 'vectorstore' ? vectorDrawerBody : section === 'storage' ? storageDrawerBody : webDrawerBody}
      </form>
    </SettingDrawer>
  </div>;
}
