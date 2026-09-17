import { useEffect, useRef, useState, type FormEvent } from 'react';
import { createPortal } from 'react-dom';
import type { DataSource, DataSourceConnectorType, DataSourceResource, DataSourceSyncLog, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Checkbox, Input, Select, Sheet, Status, Textarea } from '@weknora/ui';
import { buildDataSourceInput, credentialStepKind, credentialStepReducer, credentialValue, credentialsRequiredForValidation, dataSourceFormFrom, firstMissingRequiredCredential, initialCredentialStepState, VUE_CREDENTIAL_FIELDS, VUE_SETTINGS_FIELDS, type CredentialField, type DataSourceFormValues } from './form.ts';
import { resourceCheckStates, toggleResourceSelection } from './resource-selection.ts';
import { classifyDataSourceError } from './error-state.ts';
import { isSyncRunning, mergeSyncLogs } from './log-state.ts';
import { humanizeCron, relativeTime, syncResultPills } from './card.ts';
import { createTranslator, useAppLocale } from '../i18n.ts';

const newForm: DataSourceFormValues = { name: '', type: '', schedule: '0 0 */6 * * *', mode: 'incremental', conflict: 'overwrite', deletions: true, credentialsText: '', settingsText: '', resourceIds: [] };
const LOG_PAGE_SIZE = 50;
const VUE_CREATE_CONNECTOR_ORDER = ['feishu', 'lark', 'feishu_drive', 'lark_drive', 'notion', 'yuque', 'ima', 'rss', 'gitlab'];

const syncStatusKeys: Record<string, string> = {
  running: 'dataSource.status.running', success: 'dataSource.status.success', partial: 'dataSource.status.partial',
  failed: 'dataSource.status.failed', canceled: 'dataSource.status.canceled', active: 'dataSource.status.active',
  paused: 'dataSource.status.paused', error: 'dataSource.status.error',
};

function localizedSyncStatus(t: (key: string) => string, status: unknown): string {
  if (typeof status !== 'string' || status.length === 0) return '';
  return syncStatusKeys[status] ? t(syncStatusKeys[status]) : status;
}

// Vue ds-card__status colors: active→success, paused→warning, error→error.
function statusToneClass(status: unknown): string {
  return status === 'active' ? 'text-success-text' : status === 'paused' ? 'text-warning-text' : status === 'error' ? 'text-danger' : '';
}

// Vue ds-card__sync-result colors: success→success, failed→error,
// running→brand, partial→warning, others stay secondary.
function syncResultToneClass(status: string): string {
  return status === 'success' ? 'text-success-text' : status === 'failed' ? 'text-danger' : status === 'running' ? 'text-primary' : status === 'partial' ? 'text-warning-text' : '';
}

function connectionError(value: unknown, fallback = 'Connection test failed'): string | null {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return null;
  const row = value as Record<string, unknown>;
  if (row.success === false) return typeof row.message === 'string' ? row.message : typeof row.error === 'string' ? row.error : fallback;
  return null;
}

function setCredentialValue(text: string, key: string, value: string): string {
  const lines = text.split(/\r?\n/).filter((line) => line.trim() && !line.trim().startsWith(`${key} =`) && !line.trim().startsWith(`${key}=`));
  return value.trim() ? [...lines, `${key} = ${value}`].join('\n') : lines.join('\n');
}

export function DataSourcesPage({ client, knowledgeBaseId, canManage = false }: { client: WeKnoraClient; knowledgeBaseId: string; canManage?: boolean }) {
  const t = createTranslator(useAppLocale());
  const [sources, setSources] = useState<DataSource[]>([]);
  const [types, setTypes] = useState<DataSourceConnectorType[]>([]);
  const [logs, setLogs] = useState<DataSourceSyncLog[]>([]);
  const [logsOffset, setLogsOffset] = useState(0);
  const [logsHasNext, setLogsHasNext] = useState(false);
  const [logsLoading, setLogsLoading] = useState(false);
  const [logsLoadingMore, setLogsLoadingMore] = useState(false);
  const [logsError, setLogsError] = useState<string | null>(null);
  const [logsSource, setLogsSource] = useState<DataSource | null>(null);
  const [resourceSource, setResourceSource] = useState<DataSource | null>(null);
  const [resources, setResources] = useState<DataSourceResource[]>([]);
  const [resourceParent, setResourceParent] = useState<string | undefined>(undefined);
  const [resourceTrail, setResourceTrail] = useState<Array<{ id: string; name: string }>>([]);
  const [resourceLoading, setResourceLoading] = useState(false);
  const [resourceError, setResourceError] = useState<string | null>(null);
  const [editing, setEditing] = useState<DataSource | null | undefined>(undefined);
  const [credentialStep, setCredentialStep] = useState(() => initialCredentialStepState(false));
  const [createStep, setCreateStep] = useState<'type' | 'form'>('type');
  const [form, setForm] = useState<DataSourceFormValues>(newForm);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [action, setAction] = useState<string | null>(null);
  const [message, setMessage] = useState<{ tone: 'error' | 'success' | 'warning'; text: string } | null>(null);
  const [accessDenied, setAccessDenied] = useState(false);
  const pollTimer = useRef<number | null>(null);
  const dataSources = client.dataSources;

  function stopPolling() {
    if (pollTimer.current !== null) {
      window.clearTimeout(pollTimer.current);
      pollTimer.current = null;
    }
  }

  async function load(silent = false) {
    if (!silent) setLoading(true);
    setAccessDenied(false);
    try {
      const next = await dataSources.list(knowledgeBaseId);
      setSources(next);
      stopPolling();
      if (next.some(isSyncRunning)) pollTimer.current = window.setTimeout(() => { void load(true); }, 3000);
    }
    catch (error) {
      stopPolling();
      const classified = classifyDataSourceError(error, t('dataSource.loadFailed'));
      setAccessDenied(classified.kind === 'forbidden');
      setMessage({ tone: 'error', text: classified.kind === 'forbidden' ? t('dataSource.permissionRequired') : classified.message });
    }
    finally { if (!silent) setLoading(false); }
  }
  useEffect(() => {
    void load();
    void dataSources.types().then((items) => setTypes(items.sort((left, right) => left.priority - right.priority))).catch((error) => setMessage({ tone: 'warning', text: error instanceof Error ? error.message : t('dataSource.resourceLoadFailed') }));
    return stopPolling;
  }, [client, knowledgeBaseId]);

  function openCreate() { if (accessDenied || !canManage) return; setEditing(null); setCreateStep('type'); setForm({ ...newForm, type: '' }); setCredentialStep(initialCredentialStepState(false)); setResourceSource(null); setMessage(null); }
  function chooseCreateType(type: string) { setForm((current) => ({ ...current, type })); setCreateStep('form'); }
  function openEdit(source: DataSource) { if (!canManage) return; setEditing(source); setCreateStep('form'); setForm(dataSourceFormFrom(source)); setCredentialStep(initialCredentialStepState((source as { credentials?: { credentials?: { configured?: unknown } } })?.credentials?.credentials?.configured === true)); setResourceSource(null); setMessage(null); }
  function updateForm<K extends keyof DataSourceFormValues>(key: K, value: DataSourceFormValues[K]) { setForm((current) => ({ ...current, [key]: value })); }

  async function save(event?: FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    if (!canManage) return;
    if (!form.name.trim() || !form.type.trim() || !form.schedule.trim()) {
      setMessage({ tone: 'error', text: t('dataSource.saveFailed') });
      return;
    }
    // Vue validateStep1Fields runs the per-field required-credential walk only
    // when credentials are required: an edit of an already-configured
    // connector stays on the configured row until the user opts in to Replace,
    // so the walk is keyed off the replace-mode flag (needsConnectionTest).
    if (credentialsRequiredForValidation({ isEdit: Boolean(editing), credentialsConfigured: credentialStep.credentialsConfigured, replacementTyped: credentialStep.replaceMode })) {
      const missingCredential = firstMissingRequiredCredential(form.type, form.credentialsText);
      if (missingCredential) {
        setMessage({ tone: 'warning', text: `${t(missingCredential)} ${t('dataSource.isRequired')}` });
        return;
      }
    }
    setSaving(true); setMessage(null);
    try {
      const input = buildDataSourceInput(form);
      const config = input.config as Record<string, unknown>;
      const credentials = config.credentials as Record<string, unknown>;
      if (form.credentialsText.trim()) {
        const result = await dataSources.validateCredentials(form.type, credentials);
        const failure = connectionError(result, t('dataSource.testFailed')); if (failure) throw new Error(failure);
      }
      if (editing && !form.credentialsText.trim()) input.config = { ...config, credentials: undefined };
      const saved = editing ? await dataSources.update(editing.id, input) : await dataSources.create({ ...input, knowledge_base_id: knowledgeBaseId });
      if (form.credentialsText.trim()) {
        if (editing) await dataSources.putCredentials(saved.id, credentials);
        // Vue commitCredentialsIfNeeded: the replacement is now the stored
        // credential set; collapse back to the configured row.
        if (editing) setCredentialStep((current) => credentialStepReducer(current, 'replace-committed'));
      }
      setEditing(undefined);
      if (editing) {
        // Vue edit branch: MessagePlugin.warning(updateSuccessSyncHint) — no auto sync.
        setMessage({ tone: 'warning', text: t('dataSource.updateSuccessSyncHint') });
      } else {
        // Vue create branch: trigger the first sync immediately; a failed
        // trigger still keeps the row and degrades to a warning toast.
        try {
          await dataSources.sync(saved.id);
          setMessage({ tone: 'success', text: t('dataSource.createAndSyncSuccess') });
        } catch (syncError) {
          setMessage({ tone: 'warning', text: syncError instanceof Error ? syncError.message : t('dataSource.createButSyncFailed') });
        }
      }
      await load();
    } catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : t('dataSource.saveFailed') }); }
    finally { setSaving(false); }
  }
  async function remove(source: DataSource) { if (!canManage) return; if (!window.confirm(`${t('dataSource.deleteConfirm')} ${source.name}`)) return; setAction(source.id); try { await dataSources.remove(source.id); await load(); setMessage({ tone: 'success', text: t('dataSource.deleteSuccess') }); } catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : t('dataSource.deleteFailed') }); } finally { setAction(null); } }
  // Vue confirmRemoveCredentials: DELETE the /credentials subresource of the
  // edited data source; success falls back to the unconfigured row with the
  // removedToast. The typed api-client may not ship removeCredentials yet (the
  // package is file-frozen for this lane), so the call is feature-detected and
  // surfaces the Vue removeFailed copy when absent.
  async function confirmRemoveCredentials() {
    if (!editing) return;
    setAction('remove-credentials');
    try {
      const api = dataSources as { removeCredentials?: (id: string) => Promise<unknown> };
      if (typeof api.removeCredentials !== 'function') throw new Error(t('dataSource.credential.removeFailed'));
      await api.removeCredentials(editing.id);
      setCredentialStep((current) => credentialStepReducer(current, 'remove-confirmed'));
      setForm((current) => ({ ...current, credentialsText: '' }));
      setMessage({ tone: 'success', text: t('dataSource.credential.removedToast') });
    } catch (error) {
      setMessage({ tone: 'error', text: error instanceof Error ? error.message : t('dataSource.credential.removeFailed') });
    } finally { setAction(null); }
  }
  async function run(source: DataSource, operation: 'sync' | 'pause' | 'resume' | 'validate') { if (!canManage) return; setAction(`${operation}:${source.id}`); try { const result = operation === 'sync' ? await dataSources.sync(source.id) : operation === 'pause' ? await dataSources.pause(source.id) : operation === 'resume' ? await dataSources.resume(source.id) : await dataSources.validate(source.id); const failure = connectionError(result, t('dataSource.testFailed')); if (failure) throw new Error(failure); if (operation !== 'validate') await load(); const successText = operation === 'sync' ? t('dataSource.syncTriggered') : operation === 'validate' ? t('dataSource.testSuccess') : operation === 'pause' ? t('dataSource.paused') : t('dataSource.resumed'); setMessage({ tone: 'success', text: successText }); } catch (error) { const fallback = operation === 'sync' ? t('dataSource.syncFailed') : operation === 'validate' ? t('dataSource.testFailed') : operation === 'pause' ? t('dataSource.pauseFailed') : t('dataSource.pauseFailed'); setMessage({ tone: 'error', text: error instanceof Error ? error.message : fallback }); } finally { setAction(null); } }
  async function loadLogs(source: DataSource, offset: number) {
    const append = offset > 0;
    if (append) setLogsLoadingMore(true); else setLogsLoading(true);
    setLogsError(null);
    try {
      const next = await dataSources.logs(source.id, LOG_PAGE_SIZE, offset);
      setLogs((current) => mergeSyncLogs(current, next, !append));
      setLogsOffset(offset);
      setLogsHasNext(next.length === LOG_PAGE_SIZE);
    } catch (error) {
      const classified = classifyDataSourceError(error, t('dataSource.syncLogsLoadFailed'));
      setLogsError(classified.kind === 'forbidden' ? t('dataSource.permissionRequired') : classified.message);
    } finally {
      if (append) setLogsLoadingMore(false); else setLogsLoading(false);
    }
  }
  async function showLogs(source: DataSource) { setLogsSource(source); setLogs([]); setLogsOffset(0); setLogsHasNext(false); setLogsError(null); await loadLogs(source, 0); }
  async function loadResources(source: DataSource, parentId: string | undefined, trail: Array<{ id: string; name: string }>) { setResourceLoading(true); setResourceError(null); try { const next = await dataSources.resources(source.id, parentId); setResources((current) => { const byId = new Map(current.map((resource) => [resource.external_id, resource])); for (const resource of next) byId.set(resource.external_id, resource); return [...byId.values()]; }); setResourceParent(parentId); setResourceTrail(trail); } catch (error) { const classified = classifyDataSourceError(error, t('dataSource.resourceLoadFailed')); setResourceError(classified.kind === 'forbidden' ? t('dataSource.drive.loadForbiddenHint') : classified.kind === 'auth' ? t('dataSource.drive.loadAuthHint') : classified.kind === 'not-found' ? t('dataSource.drive.loadNotFoundHint') : classified.message); } finally { setResourceLoading(false); } }
  async function revealResourceSelections(source: DataSource) { const ids = dataSourceFormFrom(source).resourceIds; if (ids.length === 0) return; setResourceLoading(true); try { const ancestors = await dataSources.resourceAncestors(source.id, ids); const byId = new Map((await dataSources.resources(source.id)).map((resource) => [resource.external_id, resource])); for (const ancestor of ancestors) for (const child of await dataSources.resources(source.id, ancestor)) byId.set(child.external_id, child); setResources([...byId.values()]); } catch (error) { const classified = classifyDataSourceError(error, t('dataSource.resourceLoadFailed')); setResourceError(classified.kind === 'forbidden' ? t('dataSource.drive.loadForbiddenHint') : classified.kind === 'auth' ? t('dataSource.drive.loadAuthHint') : classified.kind === 'not-found' ? t('dataSource.drive.loadNotFoundHint') : classified.message); } finally { setResourceLoading(false); } }
  async function showResources(source: DataSource) { setResourceSource(source); setForm(dataSourceFormFrom(source)); setResources([]); setResourceParent(undefined); setResourceTrail([]); await loadResources(source, undefined, []); await revealResourceSelections(source); }
  async function expandAllResources() { if (!resourceSource) return; setResourceLoading(true); setResourceError(null); try { const byId = new Map(resources.map((resource) => [resource.external_id, resource])); const queue = resources.filter((resource) => resource.has_children).map((resource) => resource.external_id); const visited = new Set<string>(); while (queue.length > 0) { const parentId = queue.shift()!; if (visited.has(parentId)) continue; visited.add(parentId); const children = await dataSources.resources(resourceSource.id, parentId); for (const child of children) { byId.set(child.external_id, child); if (child.has_children) queue.push(child.external_id); } } setResources([...byId.values()]); } catch (error) { const classified = classifyDataSourceError(error, t('dataSource.resourceLoadFailed')); setResourceError(classified.kind === 'forbidden' ? t('dataSource.drive.loadForbiddenHint') : classified.kind === 'auth' ? t('dataSource.drive.loadAuthHint') : classified.kind === 'not-found' ? t('dataSource.drive.loadNotFoundHint') : classified.message); } finally { setResourceLoading(false); } }
  const resourceStates = resourceCheckStates(resources, form.resourceIds);
  const kind = credentialStepKind({ isEdit: Boolean(editing), credentialsConfigured: credentialStep.credentialsConfigured, replaceMode: credentialStep.replaceMode });
  const visibleResources = resources.filter((resource) => resource.parent_id === resourceParent || (!resourceParent && !resource.parent_id));
  const createTypes = VUE_CREATE_CONNECTOR_ORDER.flatMap((type) => {
    const connector = types.find((item) => item.type === type);
    return connector ? [connector] : [];
  });
  const editorTitle = editing === null && createStep === 'type' ? t('dataSource.step.selectType') : editing ? t('dataSource.editTitle') : t('dataSource.createTitle');
  const editorSurface = editing === undefined ? null : <Sheet open title={editorTitle} onClose={() => setEditing(undefined)} closeLabel={t('dataSource.close')} width="640px" className="wk-data-source-drawer">
    {editing === null && createStep === 'type' ? <div className="grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-3">
      {createTypes.map((type) => <button key={type.type} type="button" className="flex min-h-[92px] flex-col items-start gap-1 rounded-[10px] border border-line-soft bg-white px-4 py-3 text-left transition-[border-color,box-shadow] duration-200 hover:border-primary hover:shadow-sm focus-visible:outline-2 focus-visible:outline-primary focus-visible:outline-offset-2" onClick={() => chooseCreateType(type.type)}><strong>{t(`dataSource.connector.${type.type}`)}</strong><span className="text-xs leading-5 text-muted">{t(`dataSource.connectorDesc.${type.type}`)}</span></button>)}
    </div> : <form className="wk-wiki-editor grid gap-3" onSubmit={(event) => void save(event)}>
      <p className="wk-muted m-0 text-muted">{t('dataSource.credentialsLabel')}</p>
      <label>{t('dataSource.nameLabel')} <Input required value={form.name} onChange={(event) => updateForm('name', event.target.value)} /></label>
      <label>{t('dataSource.connectorTypeLabel')} <Select required value={form.type} onChange={(event) => updateForm('type', event.target.value)}>{(editing ? types : createTypes).map((type) => <option key={type.type} value={type.type}>{type.name} ({type.type})</option>)}</Select></label>
      {kind === 'configured' ? (credentialStep.pendingRemove ? <div className="grid gap-2 rounded-lg border border-danger/40 bg-danger/5 p-3 text-[13px]" data-kind="confirm-remove"><span className="text-danger">{t('dataSource.credential.confirmRemovePrompt')}</span><div className="flex items-center gap-2"><Button type="button" variant="text" size="small" onClick={() => setCredentialStep((current) => credentialStepReducer(current, 'cancel-remove'))}>{t('common.cancel')}</Button><Button type="button" variant="danger" size="small" loading={action === 'remove-credentials'} onClick={() => void confirmRemoveCredentials()}>{t('dataSource.credential.confirmRemove')}</Button></div></div> : <div className="flex items-center gap-2 rounded-lg border border-line-soft p-3 text-[13px]" data-kind="configured"><span className="text-success-text" aria-hidden="true">✓</span><span>{t('dataSource.credential.configured')}</span><div className="ml-auto flex items-center gap-2"><Button type="button" variant="text" size="small" onClick={() => setCredentialStep((current) => credentialStepReducer(current, 'enter-replace'))}>{t('dataSource.credential.update')}</Button><Button type="button" variant="danger" size="small" onClick={() => setCredentialStep((current) => credentialStepReducer(current, 'request-remove'))}>{t('dataSource.credential.remove')}</Button></div></div>) : null}
      {kind === 'unconfigured' ? <div className="flex items-center gap-2 rounded-lg border border-dashed border-line-soft p-3 text-[13px] text-muted" data-kind="unconfigured"><span>{t('dataSource.credential.unconfigured')}</span><Button type="button" variant="text" size="small" className="ml-auto" onClick={() => setCredentialStep((current) => credentialStepReducer(current, 'enter-replace'))}>{t('dataSource.credential.configure')}</Button></div> : null}
      {kind === 'inputs' ? <fieldset className="grid gap-3 rounded-lg border border-line-soft p-3"><legend className="px-1 text-[13px] font-semibold">{t('dataSource.credentialsLabel')}</legend>{VUE_CREDENTIAL_FIELDS[form.type] ? VUE_CREDENTIAL_FIELDS[form.type].map((field) => <label key={field.key} className="grid gap-1 text-[13px]"><span>{t(field.label)}</span><Input type={field.secret ? 'password' : 'text'} required={!field.optional} placeholder={field.placeholder || t('dataSource.credential.inputPlaceholder')} value={credentialValue(form.credentialsText, field.key)} onChange={(event) => updateForm('credentialsText', setCredentialValue(form.credentialsText, field.key, event.target.value))} />{field.hint ? <small className="text-muted">{t(field.hint)}</small> : null}</label>) : <Textarea rows={4} value={form.credentialsText} onChange={(event) => updateForm('credentialsText', event.target.value)} placeholder={t('dataSource.credentialsPlaceholder')} />}</fieldset> : null}
      {editing && credentialStep.replaceMode ? <Button type="button" variant="text" size="small" className="justify-self-start" onClick={() => { setCredentialStep((current) => credentialStepReducer(current, 'cancel-replace')); setForm((current) => ({ ...current, credentialsText: '' })); }}>{t('common.cancel')}</Button> : null}
      <fieldset className="grid gap-3 rounded-lg border border-line-soft p-3"><legend className="px-1 text-[13px] font-semibold">{t('dataSource.connectorSettingsLabel')}</legend>{VUE_SETTINGS_FIELDS[form.type] ? VUE_SETTINGS_FIELDS[form.type].map((field) => <label key={field.key} className="grid gap-1 text-[13px]"><span>{t(field.label)}</span><Textarea rows={3} required={!field.optional} placeholder={field.placeholder || t('dataSource.credential.inputPlaceholder')} value={credentialValue(form.settingsText, field.key)} onChange={(event) => updateForm('settingsText', setCredentialValue(form.settingsText, field.key, event.target.value))} />{field.hint ? <small className="text-muted">{t(field.hint)}</small> : null}</label>) : <Textarea rows={4} value={form.settingsText} onChange={(event) => updateForm('settingsText', event.target.value)} placeholder={t('dataSource.settingsPlaceholder')} />}</fieldset>
      <label>{t('dataSource.syncScheduleLabel')} <Input required value={form.schedule} onChange={(event) => updateForm('schedule', event.target.value)} /></label>
      <label>{t('dataSource.syncModeLabel')} <Select value={form.mode} onChange={(event) => updateForm('mode', event.target.value as DataSourceFormValues['mode'])}><option value="incremental">{t('dataSource.syncMode.incremental')}</option><option value="full">{t('dataSource.syncMode.full')}</option></Select></label>
      <label>{t('dataSource.conflictLabel')} <Select value={form.conflict} onChange={(event) => updateForm('conflict', event.target.value as DataSourceFormValues['conflict'])}><option value="overwrite">{t('dataSource.conflict.overwrite')}</option><option value="skip">{t('dataSource.conflict.skip')}</option></Select></label>
      <label><Checkbox checked={form.deletions} onChange={(event) => updateForm('deletions', event.target.checked)} /> {t('dataSource.syncDeletions')}</label>
      <Button type="submit" loading={saving}>{t('dataSource.save')}</Button>
    </form>}
  </Sheet>;
  return <main data-can-manage={canManage ? 'true' : 'false'} className="wk-data-source-root wk-page max-w-[1180px]! mx-auto box-border px-[1.25rem] py-12"><header className="wk-header mb-7 flex items-start gap-4"><div><p className="wk-eyebrow m-0 text-[0.78rem] font-bold uppercase tracking-[0.08em] text-primary">{t('common.knowledgeBases')} · {knowledgeBaseId}</p><h1 className="text-[clamp(1.8rem,5vw,2.5rem)] my-[0.35rem]">{t('dataSource.title')}</h1><p className="wk-muted text-muted">{t('dataSource.description')}</p></div></header>
    <Card>{message ? <Status tone={message.tone}>{message.text}</Status> : null}{loading ? <Status>{t('common.loading')}</Status> : accessDenied ? <Status tone="error">{t('dataSource.permissionRequired')}</Status> : sources.length === 0 && !canManage ? <Status>{t('dataSource.empty')}</Status> : sources.length === 0 ? <div className="grid min-h-[120px] grid-cols-[repeat(auto-fill,minmax(320px,1fr))] gap-3"><button type="button" className="wk-data-source-create flex min-h-[68px] w-full flex-col items-center justify-center gap-2 rounded-[10px] border border-dashed border-line-soft bg-transparent px-4 py-[14px] text-[13px] font-medium text-muted transition-[color,border-color,background-color] duration-200 hover:border-primary hover:bg-primary/5 hover:text-primary focus-visible:outline-2 focus-visible:outline-primary focus-visible:outline-offset-2" onClick={openCreate}><span className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10 text-lg text-primary" aria-hidden="true">+</span><span>{t('dataSource.add')}</span></button></div> : <div className="wk-data-source-grid grid grid-cols-[repeat(auto-fill,minmax(320px,1fr))] gap-3">{sources.map((source) => { const log = source.latest_sync_log; const fullTime = source.last_sync_at ? new Date(source.last_sync_at).toLocaleString() : ''; return <div key={source.id} className={`wk-data-source-card flex min-w-0 items-start gap-3 rounded-[10px] border border-line-soft bg-white px-4 py-[14px] text-left transition-[border-color,box-shadow] duration-200 ${canManage ? 'cursor-pointer hover:border-primary hover:shadow-[0_4px_14px_rgba(15,23,42,0.06)] focus-visible:outline-2 focus-visible:outline-primary focus-visible:outline-offset-2' : ''}`} onClick={canManage ? () => openEdit(source) : undefined} role={canManage ? 'button' : undefined} tabIndex={canManage ? 0 : undefined}><span aria-hidden="true" className="wk-data-source-badge flex h-9 w-9 flex-none items-center justify-center rounded-[9px] bg-[rgba(7,192,95,0.12)] text-[15px] font-semibold tracking-[0.02em] text-[#07c05f]">{(source.type.slice(0, 1) || '?').toUpperCase()}</span><div className="min-w-0 flex-1"><div className="flex min-w-0 items-center gap-1.5"><strong title={source.name} className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap text-sm font-semibold leading-snug">{source.name}</strong><div className="wk-data-source-actions ml-auto flex flex-none flex-wrap items-center justify-end gap-[0.5rem]" onClick={(event) => event.stopPropagation()}>{canManage ? <><Button type="button" onClick={() => openEdit(source)}>{t('dataSource.edit')}</Button><Button type="button" disabled={action !== null} onClick={() => void run(source, 'validate')}>{t('dataSource.testConnection')}</Button><Button type="button" disabled={action !== null || isSyncRunning(source)} onClick={() => void run(source, 'sync')}>{isSyncRunning(source) ? t('dataSource.status.running') : t('dataSource.syncNow')}</Button>{source.status === 'paused' ? <Button type="button" disabled={action !== null} onClick={() => void run(source, 'resume')}>{t('dataSource.resume')}</Button> : source.status === 'active' ? <Button type="button" disabled={action !== null} onClick={() => void run(source, 'pause')}>{t('dataSource.pause')}</Button> : null}<Button type="button" disabled={action !== null} onClick={() => void remove(source)}>{t('dataSource.delete')}</Button></> : null}<Button type="button" onClick={() => void showLogs(source)}>{t('dataSource.logs')}</Button></div></div><p className="wk-data-source-card-subtitle mt-0.5 flex flex-wrap items-center gap-1 text-xs leading-normal text-muted"><span>{t(`dataSource.connector.${source.type}`)} · {source.sync_mode ? t(`dataSource.syncMode.${source.sync_mode}`) : t('dataSource.syncMode.incremental')}</span><span className={`inline-flex items-center gap-1 ${statusToneClass(source.status)}`}><span aria-hidden="true" className="h-1.5 w-1.5 flex-none rounded-full bg-current" />{localizedSyncStatus(t, source.status)}</span></p><p className="wk-data-source-card-detail mt-1 flex flex-wrap items-center gap-1 text-xs leading-snug text-muted"><span>{humanizeCron(source.sync_schedule, t)}</span><span aria-hidden="true">·</span><span title={fullTime || undefined}>{relativeTime(source.last_sync_at, t)}</span>{log ? <><span aria-hidden="true">·</span><span className={`font-medium ${syncResultToneClass(log.status)}`}>{localizedSyncStatus(t, log.status)}</span>{syncResultPills(source, t).map((pill) => <span key={pill.kind} className="wk-data-source-metric font-mono text-[11px] tabular-nums text-muted">{pill.text}</span>)}</> : null}</p>{source.error_message ? <div className="mt-2 flex items-start gap-1.5 rounded-md bg-danger/10 px-2.5 py-2 text-xs leading-snug text-danger">{source.error_message}</div> : null}</div></div>; })}{canManage ? <button type="button" className="wk-data-source-create flex min-h-[68px] w-full flex-col items-center justify-center gap-2 rounded-[10px] border border-dashed border-line-soft bg-transparent px-4 py-[14px] text-[13px] font-medium text-muted transition-[color,border-color,background-color] duration-200 hover:border-primary hover:bg-primary/5 hover:text-primary focus-visible:outline-2 focus-visible:outline-primary focus-visible:outline-offset-2" onClick={openCreate}><span className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10 text-lg text-primary" aria-hidden="true">+</span><span>{t('dataSource.add')}</span></button> : null}</div>}</Card>
    {logsSource ? <Card className="mt-4"><div className="wk-header mb-6 flex items-start justify-between gap-4"><div><h2 className="m-0">{t('dataSource.syncHistory')} · {logsSource.name}</h2><p className="wk-muted text-muted">{t('dataSource.updateSuccessSyncHint')}</p></div><Button type="button" onClick={() => setLogsSource(null)}>{t('dataSource.close')}</Button></div>{logsLoading ? <Status>{t('common.loading')}</Status> : logsError ? <><Status tone="error">{logsError}</Status><Button type="button" onClick={() => void loadLogs(logsSource, logsOffset)}>{t('common.retry')}</Button></> : logs.length === 0 ? <Status>{t('dataSource.noLogs')}</Status> : <ul className="wk-list m-0 list-none p-0">{logs.map((log) => <li key={log.id} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><span className="font-mono text-[0.8rem] text-muted">{localizedSyncStatus(t, log.status)}</span><small>{typeof log.started_at === 'string' ? log.started_at : ''}{typeof log.finished_at === 'string' ? ` → ${log.finished_at}` : ''}{typeof log.error_message === 'string' ? ` · ${log.error_message}` : ''}</small><span className="text-[#718096]! text-[0.75rem]! font-mono">{t('dataSource.logMetric.total')} {log.items_total ?? 0} · +{log.items_created ?? 0} · ↻{log.items_updated ?? 0} · −{log.items_deleted ?? 0} · {t('dataSource.logMetric.skipped')} {log.items_skipped ?? 0} · {t('dataSource.logMetric.failed')} {log.items_failed ?? 0}</span></li>)}</ul>}{logs.length > 0 ? <nav className="wk-pagination" aria-label={t('dataSource.syncHistory')}><Button type="button" disabled={logsOffset === 0 || logsLoadingMore} onClick={() => void loadLogs(logsSource!, Math.max(0, logsOffset - LOG_PAGE_SIZE))}>{t('dataSource.back')}</Button><span>{Math.floor(logsOffset / LOG_PAGE_SIZE) + 1}</span><Button type="button" disabled={!logsHasNext || logsLoadingMore} loading={logsLoadingMore} onClick={() => logsSource ? void loadLogs(logsSource, logsOffset + LOG_PAGE_SIZE) : undefined}>{t('dataSource.loadMore')}</Button></nav> : null}</Card> : null}
    {resourceSource ? <Card className="mt-4"><div className="wk-header mb-6 flex items-start justify-between gap-4"><div><h2>{t('dataSource.resourceHint')} · {resourceSource.name}</h2><p className="wk-muted text-muted">{resourceTrail.length > 0 ? resourceTrail.map((item) => item.name).join(" / ") : t("dataSource.resourceHint")}</p></div><Button type="button" onClick={() => setResourceSource(null)}>{t('dataSource.close')}</Button></div>{resourceTrail.length > 0 ? <Button type="button" onClick={() => { const previous = resourceTrail.slice(0, -1); void loadResources(resourceSource, previous.length > 0 ? previous[previous.length - 1]!.id : undefined, previous); }}>{t('dataSource.back')}</Button> : null}<Button type="button" onClick={() => void expandAllResources()} disabled={resourceLoading || !resources.some((resource) => resource.has_children)}>{t('dataSource.loadMore')}</Button>{resourceLoading ? <Status>{t('common.loading')}</Status> : null}{resourceError ? <Status tone="error">{resourceError}</Status> : null}{!resourceLoading && !resourceError && visibleResources.length === 0 ? <Status>{t('dataSource.noResources')}</Status> : null}{!resourceLoading && !resourceError ? <ul className="wk-list m-0 list-none p-0" role="tree">{visibleResources.map((resource) => <li key={resource.external_id} className="items-center! flex justify-between gap-4 border-b border-line-soft py-[0.9rem]" role="treeitem"><div className="grid gap-[0.2rem] min-w-0"><label className="inline-flex min-w-0 cursor-pointer items-center gap-[0.45rem]"><Checkbox className="peer absolute h-px w-px opacity-0" checked={resourceStates.get(resource.external_id) === "checked"} aria-checked={resourceStates.get(resource.external_id) === "indeterminate" ? "mixed" : resourceStates.get(resource.external_id) === "checked"} data-state={resourceStates.get(resource.external_id)} onChange={() => setForm((current) => ({ ...current, resourceIds: toggleResourceSelection(resources, current.resourceIds, resource.external_id) }))} /><span className={`inline-flex h-[15px] w-[15px] flex-none items-center justify-center rounded-[3px] border font-mono text-[0.8rem] leading-none text-muted ${(resourceStates.get(resource.external_id) ?? "unchecked") === "unchecked" ? "border-[#a8b5c8] bg-white" : "border-primary bg-primary"} peer-focus-visible:outline-2 peer-focus-visible:outline-[#2e6de6] peer-focus-visible:outline-offset-2`} aria-hidden="true">{resourceStates.get(resource.external_id) === "checked" ? "✓" : resourceStates.get(resource.external_id) === "indeterminate" ? "−" : ""}</span> <strong title={resource.name} className="overflow-hidden text-ellipsis whitespace-nowrap">{resource.name}</strong></label><small className="text-[#718096]">{resource.type}</small></div>{resource.has_children ? <Button type="button" onClick={() => void loadResources(resourceSource, resource.external_id, [...resourceTrail, { id: resource.external_id, name: resource.name }])}>{t('dataSource.next')}</Button> : null}</li>)}</ul> : null}</Card> : null}
    {editorSurface ? createPortal(editorSurface, document.body) : null}
  </main>;
}
