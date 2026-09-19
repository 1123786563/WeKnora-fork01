import { useEffect, useState } from 'react';
import type { CSSProperties } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type { Locale } from '@weknora/i18n';
import { Button, Input, Sheet, Status, Textarea } from '@weknora/ui';
import { settingsResourceInput, settingsResourceRows } from './surface.ts';
import { providerLogo } from './providerLogos.ts';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';

type ResourceSection = 'storage' | 'vectorstore' | 'websearch';
type ResourceRow = Record<string, unknown>;
type ResourceApi = {
  list: () => Promise<readonly ResourceRow[]>;
  listEnvelope?: () => Promise<{ rows: readonly ResourceRow[]; defaultId?: string }>;
  types?: () => Promise<readonly unknown[]>;
  create: (input: Record<string, unknown>) => Promise<ResourceRow>;
  update: (id: string, input: Record<string, unknown>) => Promise<ResourceRow>;
  remove: (id: string) => Promise<unknown>;
  testById: (id: string) => Promise<{ success: boolean; message?: string; error?: string }>;
  setDefault?: (id: string) => Promise<unknown>;
};

const RESOURCE_COPY: Record<Locale, { type: string; typePlaceholder: string; safeConfig: string; securityHint: string; unnamed: string; typeUnavailable: string; defaultLabel: string; loadFailed: string; setDefaultFailed: string; localLabel: string }> = {
  'zh-CN': { type: '类型', typePlaceholder: '提供方类型', safeConfig: '安全配置 JSON', securityHint: '凭证不会从服务端回填；测试和删除操作均等待服务端确认。', unnamed: '未命名资源', typeUnavailable: '类型未知', defaultLabel: '默认', loadFailed: '资源列表加载失败', setDefaultFailed: '设置默认资源失败', localLabel: '本地存储' },
  'en-US': { type: 'Type', typePlaceholder: 'Provider type', safeConfig: 'Safe configuration JSON', securityHint: 'Secrets are never prefilled from server responses. Test and delete operations wait for server confirmation.', unnamed: 'Unnamed resource', typeUnavailable: 'Type unavailable', defaultLabel: 'default', loadFailed: 'Failed to load resources', setDefaultFailed: 'Failed to set the default resource', localLabel: 'Local storage' },
  'ja-JP': { type: 'タイプ', typePlaceholder: 'プロバイダーの種類', safeConfig: '安全な設定 JSON', securityHint: 'シークレットはサーバーの応答から再表示しません。テストと削除はサーバーの確認を待ちます。', unnamed: '名前なしのリソース', typeUnavailable: '種類不明', defaultLabel: 'デフォルト', loadFailed: 'リソース一覧の読み込みに失敗しました', setDefaultFailed: 'デフォルトリソースの設定に失敗しました', localLabel: 'ローカルストレージ' },
  'ko-KR': { type: '유형', typePlaceholder: '공급자 유형', safeConfig: '안전 구성 JSON', securityHint: '서버 응답의 비밀 값은 다시 표시하지 않습니다. 테스트와 삭제는 서버 확인 후 완료됩니다.', unnamed: '이름 없는 리소스', typeUnavailable: '유형 없음', defaultLabel: '기본값', loadFailed: '리소스 목록을 불러오지 못했습니다', setDefaultFailed: '기본 리소스를 설정하지 못했습니다', localLabel: '로컬 스토리지' },
  'ru-RU': { type: 'Тип', typePlaceholder: 'Тип провайдера', safeConfig: 'Безопасный JSON конфигурации', securityHint: 'Секреты не подставляются из ответов сервера. Тестирование и удаление ждут подтверждения сервера.', unnamed: 'Ресурс без имени', typeUnavailable: 'Тип неизвестен', defaultLabel: 'по умолчанию', loadFailed: 'Не удалось загрузить список ресурсов', setDefaultFailed: 'Не удалось назначить ресурсом по умолчанию', localLabel: 'Локальное хранилище' },
};

function rowId(row: ResourceRow): string {
  const id = row.id ?? row.uuid ?? row.name;
  return typeof id === 'string' || typeof id === 'number' ? String(id) : '';
}

function rowText(row: ResourceRow, key: string): string { return typeof row[key] === 'string' ? row[key] as string : ''; }

function nestedText(row: ResourceRow, objectKey: string, key: string): string {
  const value = row[objectKey];
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return '';
  return typeof (value as Record<string, unknown>)[key] === 'string' ? (value as Record<string, string>)[key] : '';
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

function safeConfig(value: unknown): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return {};
  return Object.fromEntries(Object.entries(value as Record<string, unknown>).filter(([key]) => {
    const normalized = key.toLowerCase();
    return !normalized.includes('secret') && !normalized.includes('password') && !normalized.includes('token') && !normalized.includes('api_key') && !normalized.includes('access_key');
  }).map(([key, item]) => [key, item && typeof item === 'object' && !Array.isArray(item) ? safeConfig(item) : item]));
}

function apiFor(client: WeKnoraClient, section: ResourceSection): ResourceApi {
  if (section === 'storage') {
    const backends = client.settings.storage.backends;
    return {
      ...backends,
      // Vue StorageBackendSettings compares backend.id against the list
      // envelope's default_storage_backend_id, so the default id travels with
      // the rows.
      listEnvelope: async () => {
        const result = await (backends as unknown as { listWithEnvelope?: () => Promise<{ rows: readonly ResourceRow[]; defaultId?: string }> }).listWithEnvelope?.();
        return result ?? { rows: await backends.list(), defaultId: undefined };
      },
    };
  }
  if (section === 'vectorstore') return client.settings.vectorStores;
  return client.settings.webSearch.providers;
}

export function ResourceSettingsPanel({ client, section, initialValue }: { client: WeKnoraClient; section: ResourceSection; initialValue: unknown }) {
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
  const [providerTypes, setProviderTypes] = useState<Array<{ id: string; name: string }>>([]);
  const [name, setName] = useState('');
  const [type, setType] = useState('');
  const [configText, setConfigText] = useState('{}');
  const [editingId, setEditingId] = useState<string | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => { setRows(settingsResourceRows(initialValue, section)); }, [initialValue, section]);

  // The 默认 tag needs the list envelope's default id (storage section), which
  // the parent's prefetched initialValue does not carry — refetch on mount.
  useEffect(() => { void loadRows().catch(() => undefined); }, []);

  // Websearch cards show the provider type's display name (Vue
  // providerTypeLabel) sourced from /web-search-providers/types.
  useEffect(() => {
    if (section !== 'websearch' || !api.types) return;
    let active = true;
    void api.types().then((entries) => {
      if (!active) return;
      setProviderTypes(entries
        .filter((entry): entry is Record<string, unknown> => Boolean(entry) && typeof entry === 'object' && !Array.isArray(entry))
        .map((entry) => ({ id: String(entry.id ?? ''), name: String(entry.name ?? '') }))
        .filter((entry) => entry.id));
    }).catch(() => undefined);
    return () => { active = false; };
  }, [section]);

  async function loadRows() {
    if (api.listEnvelope) {
      const envelope = await api.listEnvelope();
      setRows([...envelope.rows]);
      setDefaultId(envelope.defaultId);
      return;
    }
    setRows([...await api.list()]);
  }

  function clearForm() {
    setEditingId(null); setName(''); setType(''); setConfigText('{}');
  }

  function openCreate() {
    clearForm(); setDrawerOpen(true);
  }

  function edit(row: ResourceRow) {
    setEditingId(rowId(row)); setName(rowText(row, 'name')); setType(rowText(row, 'provider') || rowText(row, 'type'));
    setConfigText(JSON.stringify(safeConfig(row.config), null, 2)); setError(null); setNotice(null); setDrawerOpen(true);
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

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setError(null); setNotice(null);
    try {
      const input = settingsResourceInput(name, type, configText);
      const saved = editingId ? await api.update(editingId, input) : await api.create(input);
      setRows((current) => editingId ? current.map((row) => rowId(row) === editingId ? saved : row) : [...current, saved]);
      clearForm(); setDrawerOpen(false); setNotice(t(editingId ? keys.updated : keys.created));
    } catch (reason) { setError(reason instanceof Error ? reason.message : t(keys.saveFailed)); }
    finally { setBusy(false); }
  }

  async function test(id: string) {
    setBusy(true); setError(null); setNotice(null);
    try { const result = await api.testById(id); if (!result.success) throw new Error(result.error || t(keys.testFailed)); setNotice(result.message || t(keys.testSuccess)); }
    catch (reason) { setError(reason instanceof Error ? reason.message : t(keys.testFailed)); }
    finally { setBusy(false); }
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

  // Vue backend-card anatomy (StorageBackendSettings.vue:18-71): provider
  // badge, name + 默认 tag, subtitle provider·meta; the whole card opens the
  // edit drawer, and the dashed add-card closes the grid.
  const resourceMeta = (row: ResourceRow, provider: string): string => {
    const description = rowText(row, 'description');
    if (description) return description;
    if (provider === 'local') return copy.localLabel;
    return '';
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
        const meta = resourceMeta(row, provider);
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
            <div className="backend-card__badge inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-[9px] text-[15px] font-semibold tracking-[0.02em]" style={{ backgroundColor: brand.bg, color: brand.color }} aria-label={provider}>{providerInitial(provider || nameValue)}</div>
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
    {rows.length === 0 ? <Status>{t(keys.empty)}</Status> : null}
    <Sheet
      open={drawerOpen}
      title={editingId ? t(keys.edit) : t(keys.add)}
      onClose={closeDrawer}
      width="460px"
    >
      {error ? <Status tone="error">{error}</Status> : null}
      {notice ? <Status tone="success">{notice}</Status> : null}
      <form className="my-4 grid max-w-[620px] gap-[.8rem] [&_label]:grid [&_label]:gap-[.35rem] [&_label]:text-[#27364d] [&_label]:font-semibold" onSubmit={(event) => void save(event)}>
        <label>{t(keys.name)}<Input required value={name} onChange={(event) => setName(event.target.value)} /></label>
        <label>{copy.type}<Input required value={type} onChange={(event) => setType(event.target.value)} placeholder={copy.typePlaceholder} /></label>
        <label>{copy.safeConfig}<Textarea rows={4} value={configText} onChange={(event) => setConfigText(event.target.value)} /></label>
        <div className="wk-list-actions mb-[0.75rem] flex flex-wrap items-center justify-end gap-[0.5rem]">
          {editingId ? <Button type="button" disabled={!editingId || busy} onClick={() => void test(editingId)}>{t(keys.test)}</Button> : null}
          {editingId && api.setDefault ? <Button type="button" disabled={!editingId || busy} onClick={() => void setDefault(editingId)}>{t(keys.setDefault)}</Button> : null}
          {editingId ? <Button type="button" disabled={busy} onClick={() => void remove(editingId)}>{t('common.delete')}</Button> : null}
          <Button type="submit" loading={busy}>{editingId ? t('common.save') : t(keys.add)}</Button>
          <Button type="button" disabled={busy} onClick={closeDrawer}>{t('common.cancel')}</Button>
        </div>
      </form>
      <p className="m-0 text-[12px] text-muted">{copy.securityHint}</p>
    </Sheet>
  </div>;
}
