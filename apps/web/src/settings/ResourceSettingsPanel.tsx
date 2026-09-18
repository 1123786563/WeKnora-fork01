import { useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type { Locale } from '@weknora/i18n';
import { Button, Card, Input, Status, Textarea } from '@weknora/ui';
import { settingsResourceInput, settingsResourceRows } from './surface.ts';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';

type ResourceSection = 'storage' | 'vectorstore' | 'websearch';
type ResourceRow = Record<string, unknown>;
type ResourceApi = {
  list: () => Promise<readonly ResourceRow[]>;
  create: (input: Record<string, unknown>) => Promise<ResourceRow>;
  update: (id: string, input: Record<string, unknown>) => Promise<ResourceRow>;
  remove: (id: string) => Promise<unknown>;
  testById: (id: string) => Promise<{ success: boolean; message?: string; error?: string }>;
  setDefault?: (id: string) => Promise<unknown>;
};

const RESOURCE_COPY: Record<Locale, { type: string; typePlaceholder: string; safeConfig: string; securityHint: string; unnamed: string; typeUnavailable: string; defaultLabel: string; loadFailed: string; setDefaultFailed: string }> = {
  'zh-CN': { type: '类型', typePlaceholder: '提供方类型', safeConfig: '安全配置 JSON', securityHint: '凭证不会从服务端回填；测试和删除操作均等待服务端确认。', unnamed: '未命名资源', typeUnavailable: '类型未知', defaultLabel: '默认', loadFailed: '资源列表加载失败', setDefaultFailed: '设置默认资源失败' },
  'en-US': { type: 'Type', typePlaceholder: 'Provider type', safeConfig: 'Safe configuration JSON', securityHint: 'Secrets are never prefilled from server responses. Test and delete operations wait for server confirmation.', unnamed: 'Unnamed resource', typeUnavailable: 'Type unavailable', defaultLabel: 'default', loadFailed: 'Failed to load resources', setDefaultFailed: 'Failed to set the default resource' },
  'ja-JP': { type: 'タイプ', typePlaceholder: 'プロバイダーの種類', safeConfig: '安全な設定 JSON', securityHint: 'シークレットはサーバーの応答から再表示しません。テストと削除はサーバーの確認を待ちます。', unnamed: '名前なしのリソース', typeUnavailable: '種類不明', defaultLabel: 'デフォルト', loadFailed: 'リソース一覧の読み込みに失敗しました', setDefaultFailed: 'デフォルトリソースの設定に失敗しました' },
  'ko-KR': { type: '유형', typePlaceholder: '공급자 유형', safeConfig: '안전한 구성 JSON', securityHint: '서버 응답의 비밀 값은 다시 표시하지 않습니다. 테스트와 삭제는 서버 확인 후 완료됩니다.', unnamed: '이름 없는 리소스', typeUnavailable: '유형 없음', defaultLabel: '기본값', loadFailed: '리소스 목록을 불러오지 못했습니다', setDefaultFailed: '기본 리소스를 설정하지 못했습니다' },
  'ru-RU': { type: 'Тип', typePlaceholder: 'Тип провайдера', safeConfig: 'Безопасный JSON конфигурации', securityHint: 'Секреты не подставляются из ответов сервера. Тестирование и удаление ждут подтверждения сервера.', unnamed: 'Ресурс без имени', typeUnavailable: 'Тип неизвестен', defaultLabel: 'по умолчанию', loadFailed: 'Не удалось загрузить список ресурсов', setDefaultFailed: 'Не удалось назначить ресурсом по умолчанию' },
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

function safeConfig(value: unknown): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return {};
  return Object.fromEntries(Object.entries(value as Record<string, unknown>).filter(([key]) => {
    const normalized = key.toLowerCase();
    return !normalized.includes('secret') && !normalized.includes('password') && !normalized.includes('token') && !normalized.includes('api_key') && !normalized.includes('access_key');
  }).map(([key, item]) => [key, item && typeof item === 'object' && !Array.isArray(item) ? safeConfig(item) : item]));
}

function apiFor(client: WeKnoraClient, section: ResourceSection): ResourceApi {
  if (section === 'storage') return client.settings.storage.backends;
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
  const [name, setName] = useState('');
  const [type, setType] = useState('');
  const [configText, setConfigText] = useState('{}');
  const [editingId, setEditingId] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => { setRows(settingsResourceRows(initialValue, section)); }, [initialValue, section]);

  function clearForm() {
    setEditingId(null); setName(''); setType(''); setConfigText('{}');
  }

  function edit(row: ResourceRow) {
    setEditingId(rowId(row)); setName(rowText(row, 'name')); setType(rowText(row, 'type'));
    setConfigText(JSON.stringify(safeConfig(row.config), null, 2)); setError(null); setNotice(null);
  }

  async function refresh() {
    setBusy(true); setError(null);
    try { setRows([...await api.list()]); }
    catch (reason) { setError(reason instanceof Error ? reason.message : copy.loadFailed); }
    finally { setBusy(false); }
  }

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setError(null); setNotice(null);
    try {
      const input = settingsResourceInput(name, type, configText);
      const saved = editingId ? await api.update(editingId, input) : await api.create(input);
      setRows((current) => editingId ? current.map((row) => rowId(row) === editingId ? saved : row) : [...current, saved]);
      clearForm(); setNotice(t(editingId ? keys.updated : keys.created));
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
    try { await api.remove(id); setRows((current) => current.filter((row) => rowId(row) !== id)); if (editingId === id) clearForm(); setNotice(t(keys.deleted)); }
    catch (reason) { setError(reason instanceof Error ? reason.message : t(keys.deleteFailed)); }
    finally { setBusy(false); }
  }

  async function setDefault(id: string) {
    if (!api.setDefault) return;
    setBusy(true); setError(null); setNotice(null);
    try { await api.setDefault(id); setNotice(t('settings.storageBackend.defaultUpdated')); await refresh(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : copy.setDefaultFailed); setBusy(false); }
  }

  const webSearchCards = section === 'websearch' ? <div className="provider-grid grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-4">
    {rows.map((row, index) => {
      const id = rowId(row);
      const provider = rowText(row, 'provider') || rowText(row, 'type') || rowText(row, 'engine_type');
      const nameValue = rowText(row, 'name') || id || copy.unnamed;
      const description = rowText(row, 'description');
      const proxyUrl = nestedText(row, 'parameters', 'proxy_url');
      return <article className={`provider-card provider-card--${provider || 'unknown'} rounded-card border border-line bg-surface p-4`} key={id || index}>
        <div className="provider-card__badge inline-flex h-10 w-10 items-center justify-center rounded-control border border-line-neutral bg-surface-subtle text-sm font-semibold text-accent" aria-label={provider || copy.typeUnavailable}>{providerInitial(provider || nameValue)}</div>
        <div className="provider-card__body mt-3 min-w-0">
          <div className="provider-card__header flex items-start justify-between gap-3">
            <h3 className="provider-card__title m-0 min-w-0 truncate text-[15px] font-semibold" title={nameValue}>{nameValue}</h3>
            <div className="provider-card__actions flex shrink-0 gap-1">
              <Button type="button" disabled={!id || busy} onClick={() => edit(row)} aria-label={`${t('common.edit')} ${nameValue}`}>{t('common.edit')}</Button>
              <Button type="button" disabled={!id || busy} onClick={() => void remove(id)} aria-label={`${t('common.delete')} ${nameValue}`}>{t('common.delete')}</Button>
            </div>
          </div>
          <div className="provider-card__subtitle mt-1 text-[13px] text-muted"><span className="provider-card__type">{provider || copy.typeUnavailable}</span>{description ? <><span className="provider-card__sep mx-1">·</span><span className="provider-card__desc" title={description}>{description}</span></> : null}</div>
          {proxyUrl ? <div className="provider-card__url mt-2 truncate font-mono text-[12px] text-muted" title={proxyUrl}>{proxyUrl}</div> : null}
        </div>
      </article>;
    })}
    <button type="button" className="provider-card provider-card--add flex min-h-[142px] items-center justify-center rounded-card border border-dashed border-line-control bg-surface p-4 text-accent" onClick={clearForm}>
      <span className="provider-card--add__label font-semibold">+ {t(keys.add)}</span>
    </button>
  </div> : null;

  return <div className="wk-settings-resource"><Card><h3>{editingId ? t(keys.edit) : t(keys.add)}</h3>{error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}<form className="wk-settings-editor my-4 grid max-w-[620px] gap-[.8rem] [&_label]:grid [&_label]:gap-[.35rem] [&_label]:text-[#27364d] [&_label]:font-semibold" onSubmit={(event) => void save(event)}><label>{t(keys.name)}<Input required value={name} onChange={(event) => setName(event.target.value)} /></label><label>{copy.type}<Input required value={type} onChange={(event) => setType(event.target.value)} placeholder={copy.typePlaceholder} /></label><label>{copy.safeConfig}<Textarea rows={4} value={configText} onChange={(event) => setConfigText(event.target.value)} /></label><div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]"><Button type="submit" loading={busy}>{editingId ? t('common.save') : t(keys.add)}</Button>{editingId ? <Button type="button" disabled={busy} onClick={clearForm}>{t('common.cancel')}</Button> : null}</div></form></Card><Card><div className="wk-settings-panel-heading flex items-start justify-between gap-4 border-b border-[#eef1f5] pb-4 mb-4 max-[720px]:flex-col"><div><h3>{t(keys.list)}</h3><p className="wk-muted text-muted m-0">{copy.securityHint}</p></div><Button type="button" disabled={busy} onClick={() => void refresh()}>{t('common.refresh')}</Button></div>{rows.length === 0 ? <Status>{t(keys.empty)}</Status> : webSearchCards ?? <ul className="wk-list m-0 list-none p-0">{rows.map((row, index) => { const id = rowId(row); return <li key={id || index} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><div className="wk-list-item-copy grid gap-[0.2rem] min-w-0"><strong>{rowText(row, 'name') || id || copy.unnamed}</strong><span className="font-mono text-[0.8rem] text-muted">{rowText(row, 'type') || rowText(row, 'engine_type') || copy.typeUnavailable}{row.default === true ? ` · ${copy.defaultLabel}` : ''}</span></div><div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]"><Button type="button" disabled={!id || busy} onClick={() => edit(row)}>{t('common.edit')}</Button><Button type="button" disabled={!id || busy} loading={busy} onClick={() => void test(id)}>{t(keys.test)}</Button>{api.setDefault ? <Button type="button" disabled={!id || busy} onClick={() => void setDefault(id)}>{t(keys.setDefault)}</Button> : null}<Button type="button" disabled={!id || busy} onClick={() => void remove(id)}>{t('common.delete')}</Button></div></li>; })}</ul>}</Card></div>;
}
