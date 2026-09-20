import { useEffect, useState } from 'react';
import type { CSSProperties, ReactNode } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type { Locale } from '@weknora/i18n';
import { Button, Input, Select, Sheet, Status, Switch } from '@weknora/ui';
import { settingsResourceRows } from './surface.ts';
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
  websearch: {},
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

function FormItem({ label, required, children, desc, warn }: { label: ReactNode; required?: boolean; children: ReactNode; desc?: ReactNode; warn?: boolean }) {
  return <div className="grid gap-[6px]">
    <label className="grid gap-[6px] text-[13px] font-medium leading-[1.4] text-[#101828]">
      {required ? <span className="mr-[4px] text-danger">*</span> : null}{label}
      {children}
    </label>
    {desc ? <p className={`m-0 text-[12px] leading-[1.5] ${warn ? 'text-danger' : 'text-muted'}`}>{desc}</p> : null}
  </div>;
}

function SectionTitle({ children }: { children: ReactNode }) {
  return <h4 className="m-0 text-[13px] font-semibold text-[#101828]">{children}</h4>;
}

function DrawerSection({ title, children }: { title: ReactNode; children: ReactNode }) {
  return <section className="grid gap-[14px] border-t border-[#eef0f3] pt-[14px] first:border-t-0 first:pt-0">
    <SectionTitle>{title}</SectionTitle>
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
  if (field.type === 'boolean') {
    return <FormItem label={label} required={field.required} desc={field.name === 'insecure_skip_verify' && value === true ? tlsWarning : undefined} warn>
      <Switch checked={value === true} onCheckedChange={onChange} />
    </FormItem>;
  }
  if (field.type === 'number') {
    const raw = value == null || value === '' ? '' : String(value);
    return <FormItem label={label} required={field.required}>
      <Input
        type="number"
        value={raw}
        min={field.min ?? 1}
        max={field.max ?? (isReplicaField(field.name) ? 10 : 64)}
        placeholder={field.default != null ? String(field.default) : ''}
        onChange={(event) => {
          const text = event.target.value.trim();
          if (!text) { onChange(undefined); return; }
          const parsed = Number(text);
          onChange(Number.isFinite(parsed) ? parsed : text);
        }}
      />
    </FormItem>;
  }
  if (field.enum && field.enum.length > 0) {
    return <FormItem label={label} required={field.required}>
      <Select value={typeof value === 'string' ? value : ''} onChange={(event) => onChange(event.target.value)}>
        {field.enum.map((option) => <option key={option} value={option}>{option}</option>)}
      </Select>
    </FormItem>;
  }
  return <FormItem label={label} required={field.required}>
    <Input
      type={field.sensitive ? 'password' : 'text'}
      value={typeof value === 'string' ? value : ''}
      placeholder={field.sensitive ? '********' : (field.default?.toString() || '')}
      maxLength={128}
      onChange={(event) => onChange(event.target.value)}
    />
  </FormItem>;
}

export function ResourceSettingsPanel({ client, section, initialValue, role = 'owner' }: { client: WeKnoraClient; section: ResourceSection; initialValue: unknown; role?: string }) {
  const api = apiFor(client, section);
  const locale = readInitialLocale();
  const t = settingsT(locale);
  const copy = RESOURCE_COPY[locale];
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
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [testing, setTesting] = useState(false);
  const [lastTestOk, setLastTestOk] = useState<boolean | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => { setRows(settingsResourceRows(initialValue, section)); }, [initialValue, section]);

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
        if (editingId) {
          // Fresh credentials commit through /credentials before the main save
          // (Vue CredentialResource ordering), never through the entity body.
          if (apiKey && api.putCredentials) await api.putCredentials(editingId, { api_key: apiKey });
          saved = await api.update(editingId, webSearchUpdatePayload(fields));
        } else {
          saved = await api.create(webSearchCreatePayload(fields));
        }
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
      const endpoint = config.endpoint;
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
  const envPill = (row: ResourceRow) => row.source === 'env' ? (
    <span className="shrink-0 rounded-[4px] border border-[#e4e7ec] bg-[#f6f8fa] px-[6px] py-[2px] text-[11px] leading-[16px] text-[#66758b]">{t('vectorStoreSettings.envTag')}</span>
  ) : null;
  // vectorstore/websearch panels keep an inner list title (storesTitle /
  // providersTitle); storage's card grid sits directly under the section
  // description in Vue.
  const innerListTitle = section === 'storage' ? null : (
    <div className="wk-settings-panel-heading flex items-center justify-between gap-4 pb-2 pt-1">
      <h3 className="m-0 text-[15px] font-semibold text-[#101828]">{t(keys.list)}</h3>
      <Button type="button" disabled={busy} aria-label={t('common.refresh')} title={t('common.refresh')} onClick={() => void refresh()}><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false"><path d="M21 12a9 9 0 11-3-6.7L21 8" /><path d="M21 3v5h-5" /></svg></Button>
    </div>
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
        <p className="m-0 rounded-[8px] bg-[#f6f8fa] px-[12px] py-[10px] text-[13px] leading-[1.5] text-[#101828]">{t('vectorStoreSettings.immutableNotice')}</p>
        <FormItem label={t('vectorStoreSettings.nameLabel')} required>
          <Input value={name} placeholder={t('vectorStoreSettings.namePlaceholder')} onChange={(event) => setName(event.target.value)} />
        </FormItem>
        <div className="grid gap-0 rounded-[8px] bg-[#f6f8fa] px-[12px] py-[10px]">
          <div className="flex items-baseline gap-2 border-b border-[#eef0f3] py-[4px] text-[12px] last:border-b-0">
            <span className="w-[80px] shrink-0 text-[11px] text-muted">{t('vectorStoreSettings.engineTypeLabel')}</span>
            <span className="min-w-0 break-all font-mono text-[12px] text-[#101828]">{selectedVectorType?.display_name || type}</span>
          </div>
          {(selectedVectorType?.connection_fields ?? []).map((field) => {
            const value = connectionConfig[field.name];
            if (!field.sensitive && (value == null || value === '')) return null;
            return <div key={field.name} className="flex items-baseline gap-2 border-b border-[#eef0f3] py-[4px] text-[12px] last:border-b-0">
              <span className="w-[80px] shrink-0 text-[11px] text-muted">{vectorStoreFieldLabel(t, field.name)}</span>
              <span className="min-w-0 break-all font-mono text-[12px] text-[#101828]">{field.sensitive ? '********' : String(value)}</span>
            </div>;
          })}
          {(selectedVectorType?.index_fields ?? []).map((field) => {
            const value = indexConfig[field.name];
            if (value == null || value === '') return null;
            return <div key={field.name} className="flex items-baseline gap-2 border-b border-[#eef0f3] py-[4px] text-[12px] last:border-b-0">
              <span className="w-[80px] shrink-0 text-[11px] text-muted">{vectorStoreFieldLabel(t, field.name)}</span>
              <span className="min-w-0 break-all font-mono text-[12px] text-[#101828]">{String(value)}</span>
            </div>;
          })}
        </div>
      </DrawerSection>
    </>
  ) : (
    <>
      <DrawerSection title={t('vectorStoreSettings.basicSection')}>
        <FormItem label={t('vectorStoreSettings.engineTypeLabel')} required>
          <Select value={type} onChange={(event) => onTypeChange(event.target.value)}>
            {vectorTypes.map((entry) => <option key={entry.type} value={entry.type}>{entry.display_name}</option>)}
          </Select>
        </FormItem>
        <FormItem label={t('vectorStoreSettings.nameLabel')} required>
          <Input value={name} placeholder={t('vectorStoreSettings.namePlaceholder')} onChange={(event) => { setName(event.target.value); resetConnectionHint(); }} />
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
        <button type="button" className="justify-self-start border-0 bg-transparent p-0 text-[13px] text-muted hover:text-accent" onClick={() => setShowAdvanced(!showAdvanced)}>
          {showAdvanced ? copy.collapse : copy.expand}
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
          <Input value={name} placeholder={t('settings.storageBackend.namePlaceholder')} onChange={(event) => setName(event.target.value)} />
        </FormItem>
        <FormItem label={t('settings.storageBackend.providerLabel')} required>
          <Select value={type} disabled={storageDisabled} onChange={(event) => onTypeChange(event.target.value)}>
            {storageProviders.map((provider) => <option key={provider} value={provider}>{provider.toUpperCase()}</option>)}
          </Select>
        </FormItem>
        {type === 'minio' ? <FormItem label={t('settings.storageBackend.modeLabel')}>
          <div className="inline-flex gap-[4px] rounded-[8px] border border-[#e4e7ec] bg-surface p-[3px]">
            <button type="button" disabled={storageDisabled} className={`inline-flex items-center gap-[6px] rounded-[6px] border-0 px-[12px] py-[5px] text-[13px] ${storageConfig.mode !== 'docker' ? 'bg-[#e8f8f2] text-[#0a7f43]' : 'bg-transparent text-muted'}`} onClick={() => setStorageConfig({ ...storageConfig, mode: 'remote' })}>{t('settings.storageBackend.modeRemote')}</button>
            <button type="button" disabled={storageDisabled} className={`inline-flex items-center gap-[6px] rounded-[6px] border-0 px-[12px] py-[5px] text-[13px] ${storageConfig.mode === 'docker' ? 'bg-[#e8f8f2] text-[#0a7f43]' : 'bg-transparent text-muted'}`} onClick={() => setStorageConfig({ ...storageConfig, mode: 'docker' })}>{t('settings.storageBackend.modeEnv')}</button>
          </div>
        </FormItem> : null}
      </DrawerSection>
      <DrawerSection title={t('settings.storageBackend.connectionSection')}>
        {needsEndpoint ? <FormItem label="Endpoint" required>
          <Input value={storageConfig.endpoint} disabled={storageDisabled} placeholder={type === 'minio' ? 'storage.example.com:9000' : 'https://storage.example.com'} onChange={(event) => setStorageConfig({ ...storageConfig, endpoint: event.target.value })} />
        </FormItem> : null}
        {needsRegion ? <FormItem label="Region" required>
          <Input value={storageConfig.region} disabled={storageDisabled} onChange={(event) => setStorageConfig({ ...storageConfig, region: event.target.value })} />
        </FormItem> : null}
        {needsCredentials ? <>
          <FormItem label="Access Key / Secret ID" required>
            <Input value={storageConfig.access_key_id} disabled={storageDisabled} placeholder="***" onChange={(event) => setStorageConfig({ ...storageConfig, access_key_id: event.target.value })} />
          </FormItem>
          <FormItem label="Secret Key" required>
            <Input type="password" value={storageConfig.secret_access_key} disabled={storageDisabled} placeholder="***" onChange={(event) => setStorageConfig({ ...storageConfig, secret_access_key: event.target.value })} />
          </FormItem>
        </> : null}
        {type !== 'local' ? <FormItem label="Bucket" required>
          <Input value={storageConfig.bucket_name} disabled={storageDisabled} onChange={(event) => setStorageConfig({ ...storageConfig, bucket_name: event.target.value })} />
        </FormItem> : null}
        {type === 'cos' ? <FormItem label="App ID">
          <Input value={storageConfig.app_id || ''} disabled={storageDisabled} placeholder={t('settings.storageBackend.optionalPlaceholder')} onChange={(event) => setStorageConfig({ ...storageConfig, app_id: event.target.value })} />
        </FormItem> : null}
      </DrawerSection>
      <DrawerSection title={t('settings.storageBackend.advancedSection')}>
        <FormItem label={t('settings.storageBackend.pathPrefixLabel')}>
          <Input value={storageConfig.path_prefix} disabled={storageDisabled} placeholder="weknora/" onChange={(event) => setStorageConfig({ ...storageConfig, path_prefix: event.target.value })} />
        </FormItem>
        {type === 'minio' ? <div className="flex items-center gap-2">
          <Switch checked={storageConfig.use_ssl} onCheckedChange={(checked) => setStorageConfig({ ...storageConfig, use_ssl: checked })} />
          <span className="text-[12px] text-muted">{t('settings.storageBackend.useSslDesc')}</span>
        </div> : null}
        {type === 's3' ? <div className="flex items-center gap-2">
          <Switch checked={storageConfig.force_path_style === true} onCheckedChange={(checked) => setStorageConfig({ ...storageConfig, force_path_style: checked })} />
          <span className="text-[12px] text-muted">{t('settings.storageBackend.forcePathStyleDesc')}</span>
        </div> : null}
        {type === 'oss' ? <div className="flex items-center gap-2">
          <Switch checked={storageConfig.use_temp_bucket === true} onCheckedChange={(checked) => setStorageConfig({ ...storageConfig, use_temp_bucket: checked })} />
          <span className="text-[12px] text-muted">{t('settings.storageBackend.useTempBucketDesc')}</span>
        </div> : null}
        {['cos', 'tos'].includes(type) || (type === 'oss' && storageConfig.use_temp_bucket) ? <>
          <FormItem label={t('settings.storageBackend.tempBucketLabel')}>
            <Input value={storageConfig.temp_bucket_name || ''} placeholder={t('settings.storageBackend.tempBucketPlaceholder')} onChange={(event) => setStorageConfig({ ...storageConfig, temp_bucket_name: event.target.value })} />
          </FormItem>
          <FormItem label={t('settings.storageBackend.tempRegionLabel')}>
            <Input value={storageConfig.temp_region || ''} placeholder={t('settings.storageBackend.tempRegionPlaceholder')} onChange={(event) => setStorageConfig({ ...storageConfig, temp_region: event.target.value })} />
          </FormItem>
        </> : null}
      </DrawerSection>
    </>
  );

  const webCredentialFields = selectedWebType?.requires_api_key || selectedWebType?.supports_optional_api_key || selectedWebType?.requires_engine_id || selectedWebType?.requires_base_url || (selectedWebType?.config_fields?.length ?? 0) > 0;
  const webDrawerBody = (
    <>
      {selectedWebType ? <p className="m-0 flex items-center gap-[8px] text-[13px] text-muted">
        <span>{selectedWebType.name}</span>
        {selectedWebType.docs_url ? <a className="text-accent" href={selectedWebType.docs_url} target="_blank" rel="noopener noreferrer">{t('webSearchSettings.viewDocs')}</a> : null}
      </p> : null}
      <DrawerSection title={t('webSearchSettings.basicSection')}>
        <FormItem label={t('webSearchSettings.providerTypeLabel')} required>
          <Select value={type} disabled={Boolean(editingId)} onChange={(event) => onTypeChange(event.target.value)}>
            {webTypes.map((entry) => <option key={entry.id} value={entry.id}>{entry.name}</option>)}
          </Select>
        </FormItem>
        <FormItem label={t('webSearchSettings.providerNameLabel')}>
          <Input value={name} placeholder={selectedWebType?.name || t('webSearchSettings.providerNamePlaceholder')} onChange={(event) => { setName(event.target.value); resetConnectionHint(); }} />
        </FormItem>
        <FormItem label={t('webSearchSettings.providerDescLabel')}>
          <Input value={description} placeholder={t('webSearchSettings.providerDescPlaceholder')} onChange={(event) => setDescription(event.target.value)} />
        </FormItem>
      </DrawerSection>
      {webCredentialFields ? <DrawerSection title={t('webSearchSettings.credentialsSection')}>
        {selectedWebType?.requires_base_url ? <FormItem label={t('webSearchSettings.baseUrlLabel')} required>
          <Input value={baseUrl} placeholder={t('webSearchSettings.baseUrlPlaceholder')} onChange={(event) => { setBaseUrl(event.target.value); resetConnectionHint(); }} />
        </FormItem> : null}
        {selectedWebType?.requires_api_key || selectedWebType?.supports_optional_api_key ? (
          <FormItem label={selectedWebType?.supports_optional_api_key && !selectedWebType?.requires_api_key ? t('webSearchSettings.apiKeyOptionalLabel') : t('webSearchSettings.apiKeyLabel')} required={selectedWebType?.requires_api_key}>
            <Input type="password" value={apiKey} placeholder={t('webSearchSettings.apiKeyPlaceholder')} onChange={(event) => { setApiKey(event.target.value); resetConnectionHint(); }} />
          </FormItem>
        ) : null}
        {selectedWebType?.requires_engine_id ? <FormItem label={t('webSearchSettings.engineIdLabel')} required>
          <Input value={engineId} placeholder={t('webSearchSettings.engineIdLabel')} onChange={(event) => { setEngineId(event.target.value); resetConnectionHint(); }} />
        </FormItem> : null}
        {(selectedWebType?.config_fields ?? []).map((field) => (
          <FormItem key={field.key} label={webSearchConfigText(t, field.label_key, field.label)} required={field.required} desc={field.description ? webSearchConfigText(t, field.description_key, field.description) : undefined}>
            <Select value={extraConfig[field.key] ?? field.default ?? ''} onChange={(event) => setExtraConfig({ ...extraConfig, [field.key]: event.target.value })}>
              {(field.options.length > 0 ? field.options : [{ label: field.default || '', value: field.default || '' }]).map((option) => <option key={option.value} value={option.value}>{webSearchConfigText(t, option.label_key, option.label)}</option>)}
            </Select>
          </FormItem>
        ))}
      </DrawerSection> : null}
      <DrawerSection title={t('webSearchSettings.optionsSection')}>
        {selectedWebType?.supports_proxy ? <FormItem label={t('webSearchSettings.proxyUrlLabel')} desc={t('webSearchSettings.proxyUrlHelp')}>
          <Input value={proxyUrl} placeholder={t('webSearchSettings.proxyUrlPlaceholder')} onChange={(event) => setProxyUrl(event.target.value)} />
        </FormItem> : null}
        <FormItem label={t('webSearchSettings.setAsDefault')}>
          <div className="flex items-center gap-2">
            <Switch checked={isDefault} onCheckedChange={setIsDefault} />
            <span className="text-[12px] text-muted">{t('webSearchSettings.setAsDefaultDesc')}</span>
          </div>
        </FormItem>
      </DrawerSection>
    </>
  );

  return <div className="wk-settings-resource">
    {error ? <Status tone="error">{error}</Status> : null}
    {notice ? <Status tone="success">{notice}</Status> : null}
    {innerListTitle}
    <div className="backend-grid grid grid-cols-[repeat(auto-fill,minmax(240px,1fr))] gap-4">
      {rows.map((row, index) => {
        const id = rowId(row);
        const provider = rowText(row, 'provider') || rowText(row, 'type') || rowText(row, 'engine_type');
        const nameValue = rowText(row, 'name') || id || copy.unnamed;
        const isDefault = row.default === true || (typeof defaultId === 'string' && id === defaultId);
        const meta = resourceMeta(row);
        const logo = providerLogo(section, provider);
        const brand = PROVIDER_BRAND[section][provider.toLowerCase()] ?? { bg: 'rgba(0, 82, 217, 0.1)', color: '#0052D9' };
        const monoMaskStyle = logo?.mode === 'mono' ? {
          width: '22px',
          height: '22px',
          backgroundColor: 'currentColor',
          WebkitMaskImage: `url("${logo.url}")`,
          WebkitMaskPosition: 'center',
          WebkitMaskRepeat: 'no-repeat',
          WebkitMaskSize: 'contain',
          maskImage: `url("${logo.url}")`,
          maskPosition: 'center',
          maskRepeat: 'no-repeat',
          maskSize: 'contain',
        } as CSSProperties : undefined;
        return <article
          key={id || index}
          role="button"
          tabIndex={0}
          className="backend-card flex cursor-pointer items-start gap-3 rounded-[10px] border border-[#e4e7ec] bg-white p-4 transition-shadow hover:shadow-[0_4px_14px_rgba(0,0,0,0.07)]"
          onClick={() => edit(row)}
          onKeyDown={(event) => { if (event.key === 'Enter') edit(row); }}
        >
          {logo ? (
            <div className="backend-card__badge inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-[9px] border border-[rgba(0,0,0,0.06)] bg-white" style={{ color: brand.color }} aria-label={provider}>
              {logo.mode === 'color'
                ? <img src={logo.url} alt="" className="h-6 w-6 object-contain" />
                : <span style={monoMaskStyle} aria-hidden="true" />}
            </div>
          ) : (
            <div className="backend-card__badge inline-flex h-10 w-10 items-center justify-center rounded-[9px] text-[15px] font-semibold tracking-[0.02em]" style={{ backgroundColor: brand.bg, color: brand.color }} aria-label={provider}>{providerInitial(provider || nameValue)}</div>
          )}
          <div className="min-w-0 flex-1">
            <div className="backend-card__header flex items-center gap-2">
              <h3 className="backend-card__title m-0 min-w-0 truncate text-[15px] font-semibold text-[#101828]" title={nameValue}>{nameValue}</h3>
              {section !== 'storage' && row.source === 'env' ? envPill(row) : null}
              {isDefault ? <span className="shrink-0 rounded-[4px] bg-[#e8f8f2] px-[6px] py-[2px] text-[11px] leading-[16px] text-[#0a7f43]">{copy.defaultLabel}</span> : null}
            </div>
            <p className="backend-card__subtitle m-0 mt-1 flex items-center truncate text-[13px] text-muted">
              <span>{provider ? (section === 'websearch' ? providerTypeLabel(provider) : providerLabel(provider)) : copy.typeUnavailable}</span>
              {meta ? <><span className="mx-[4px]">·</span><span className="truncate">{meta}</span></> : null}
            </p>
          </div>
        </article>;
      })}
      <button
        type="button"
        className="backend-card backend-card--add flex min-h-[92px] flex-col items-center justify-center gap-2 rounded-[10px] border border-dashed border-[#c7cdd6] bg-surface text-[#07c05f] transition-colors hover:bg-[#f6f8fa]"
        onClick={openCreate}
      >
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" aria-hidden="true" focusable="false"><path d="M12 5v14M5 12h14" /></svg>
        <span className="text-[13px]">{t(keys.add)}</span>
      </button>
    </div>
    {/* Vue WebSearchSettings L13: the empty-state hint only renders for
        non-admin viewers; admins get the bare grid, no desc line. */}
    {rows.length === 0 && !(section === 'websearch' && (role === 'owner' || role === 'admin')) ? <Status>{t(keys.empty)}</Status> : null}
    <Sheet
      open={drawerOpen}
      title={editingId ? t(keys.edit) : t(keys.add)}
      onClose={closeDrawer}
      width="460px"
    >
      {error ? <Status tone="error">{error}</Status> : null}
      {notice ? <Status tone="success">{notice}</Status> : null}
      <form className="my-4 grid gap-[16px]" onSubmit={(event) => void save(event)}>
        {section === 'vectorstore' ? vectorDrawerBody : section === 'storage' ? storageDrawerBody : webDrawerBody}
        <div className="wk-list-actions mt-[4px] flex flex-wrap items-center gap-[0.5rem]">
          <Button type="button" loading={testing} disabled={!canTestConnection || busy} onClick={() => void testConnection()}>{testing ? t('vectorStoreSettings.testing') : t(keys.test)}</Button>
          {editingId && api.setDefault && section === 'storage' ? <Button type="button" disabled={!editingId || busy} onClick={() => void setDefault(editingId)}>{t(keys.setDefault)}</Button> : null}
          {editingId ? <Button type="button" disabled={busy} onClick={() => void remove(editingId)}>{t('common.delete')}</Button> : null}
          <span className="flex-1" />
          <Button type="submit" loading={busy}>{editingId ? t('common.save') : t(keys.add)}</Button>
          <Button type="button" disabled={busy} onClick={closeDrawer}>{t('common.cancel')}</Button>
        </div>
      </form>
    </Sheet>
  </div>;
}
