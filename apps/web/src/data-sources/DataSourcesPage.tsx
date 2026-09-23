import { useEffect, useRef, useState, type FormEvent } from 'react';
import { createPortal } from 'react-dom';
import { ApiError, type DataSource, type DataSourceConnectorType, type DataSourceResource, type DataSourceSyncItemError, type DataSourceSyncLog, type WeKnoraClient } from '@weknora/api-client';
import { Button, Checkbox, Dialog, Drawer, Input, Select, Textarea } from 'tdesign-react';
import { Card, Status } from './ui.tsx';
import { buildDataSourceInput, credentialStepKind, credentialStepReducer, credentialValue, credentialsRequiredForValidation, dataSourceFormFrom, firstMissingRequiredCredential, initialCredentialStepState, serializeAuthHeaders, VUE_CONNECTOR_GUIDES, VUE_CREDENTIAL_FIELDS, VUE_SETTINGS_FIELDS, type CredentialField, type DataSourceFormValues, type GitLabProjectInput, type HeaderRow } from './form.ts';
import { extractDriveFolderToken, isDriveConnector, resourceCheckStates, toggleResourceSelection } from './resource-selection.ts';
import { classifyDataSourceError } from './error-state.ts';
import { isSyncRunning, mergeSyncLogs } from './log-state.ts';
import { humanizeCron, relativeTime, syncResultPills } from './card.ts';
import { createTranslator, useAppLocale } from '../i18n.ts';
import { KbIcon } from '../knowledge-bases/kb-list-icons.tsx';

const newForm: DataSourceFormValues = { name: '', type: '', schedule: '0 0 */6 * * *', mode: 'incremental', conflict: 'overwrite', deletions: true, credentialsText: '', settingsText: '', resourceIds: [], authHeaders: [], gitlabProjects: [] };
const emptyGitLabProject: GitLabProjectInput = { project_id: '', ref: '', pathsText: '' };
const LOG_PAGE_SIZE = 50;
const VUE_CREATE_CONNECTOR_ORDER = ['feishu', 'lark', 'feishu_drive', 'lark_drive', 'notion', 'yuque', 'ima', 'rss', 'gitlab', 'confluence', 'dingtalk'];

const syncStatusKeys: Record<string, string> = {
  running: 'dataSource.status.running', success: 'dataSource.status.success', partial: 'dataSource.status.partial',
  failed: 'dataSource.status.failed', canceled: 'dataSource.status.canceled', active: 'dataSource.status.active',
  paused: 'dataSource.status.paused', error: 'dataSource.status.error',
};

function localizedSyncStatus(t: (key: string) => string, status: unknown): string {
  if (typeof status !== 'string' || status.length === 0) return '';
  return syncStatusKeys[status] ? t(syncStatusKeys[status]) : status;
}

// Vue t(perTypeKey, fallbackKey): the per-type prereq copy wins; the shared
// fallback applies when the per-type key is missing (formatMessage returns
// the key itself when no locale carries it).
function prereqCopy(t: (key: string) => string, perTypeKey: string, fallbackKey: string): string {
  const value = t(perTypeKey);
  return value === perTypeKey ? t(fallbackKey) : value;
}

// Vue renders permission <code> tags only when no per-type step-2 description
// exists; formatMessage echoes the key back when it is missing, which is the
// missing-key signal here.
function hasKey(t: (key: string) => string, key: string): boolean {
  return t(key) !== key;
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

// Cap the per-item failure list so a sync that failed thousands of documents
// doesn't render an unbounded wall of text; the remainder is summarised from
// items_failed (Vue DataSourceSyncLogs.vue FAILED_ITEMS_CAP).
const FAILED_ITEMS_CAP = 50;

function failedItemSamples(log: DataSourceSyncLog): DataSourceSyncItemError[] {
  return (log.result?.errors ?? []).slice(0, FAILED_ITEMS_CAP);
}

// Vue formatSyncError: the backend sends a stable i18n `code` (+ params) so
// the reason is localised to the viewer's language; `message` is the fallback
// for old logs / codes this client doesn't know (the raw code last). The
// document title is kept separate and prefixed as "title — reason".
function formatSyncError(t: (key: string, values?: Record<string, string | number>) => string, error: DataSourceSyncItemError): string {
  let reason = '';
  if (error.code) {
    const key = `dataSource.syncError.${error.code}`;
    const localised = t(key, error.params ?? {});
    reason = localised === key ? (error.message || error.code) : localised;
  } else {
    reason = error.message || '';
  }
  return error.title ? (reason ? `${error.title} — ${reason}` : error.title) : reason;
}

export function DataSourcesPage({ client, knowledgeBaseId, canManage = false, embedded = false }: { client: WeKnoraClient; knowledgeBaseId: string; canManage?: boolean; embedded?: boolean }) {
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
  // SP2-b targeted retry: the checked failure samples' external ids across the
  // visible log rows. Reset whenever the panel switches source.
  const [retrySelection, setRetrySelection] = useState<Set<string>>(() => new Set());
  const [resourceSource, setResourceSource] = useState<DataSource | null>(null);
  const [resources, setResources] = useState<DataSourceResource[]>([]);
  const [resourceParent, setResourceParent] = useState<string | undefined>(undefined);
  const [resourceTrail, setResourceTrail] = useState<Array<{ id: string; name: string }>>([]);
  const [resourceLoading, setResourceLoading] = useState(false);
  const [resourceError, setResourceError] = useState<string | null>(null);
  const [driveToken, setDriveToken] = useState('');
  const [driveTokenError, setDriveTokenError] = useState(false);
  const [driveRootLoaded, setDriveRootLoaded] = useState(false);
  const [editing, setEditing] = useState<DataSource | null | undefined>(undefined);
  const [credentialStep, setCredentialStep] = useState(() => initialCredentialStepState(false));
  const [createStep, setCreateStep] = useState<'type' | 'form'>('type');
  // Vue DataSourceEditorDialog prereqExpanded: the step-1 setup-guide starts
  // collapsed and resets whenever the editor (re)opens or the user picks a
  // connector type (openEditor / nextStep set prereqExpanded.value = false).
  const [prereqExpanded, setPrereqExpanded] = useState(false);
  const [form, setForm] = useState<DataSourceFormValues>(newForm);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [action, setAction] = useState<string | null>(null);
  // SP2-a Task 10: dual-choice delete panel — the card delete action opens a
  // controlled Sheet instead of the native blocking confirm. Entering the
  // panel fetches the synced-documents count (documentsCount, Task 9) that
  // drives the count line, the purge checkbox label and the purge warning; a
  // request serial keeps a late count response from landing in a panel opened
  // for a different source (or after a close/reopen of the same one).
  const [deleteSource, setDeleteSource] = useState<DataSource | null>(null);
  const [deletePurge, setDeletePurge] = useState(false);
  const [deleteCount, setDeleteCount] = useState<number | null>(null);
  const [deleteCountLoading, setDeleteCountLoading] = useState(false);
  const deleteCountRequest = useRef(0);
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

  function openCreate() { if (accessDenied || !canManage) return; setEditing(null); setCreateStep('type'); setForm({ ...newForm, type: '' }); setCredentialStep(initialCredentialStepState(false)); setResourceSource(null); setMessage(null); setPrereqExpanded(() => false); }
  function chooseCreateType(type: string) {
    // Vue openEditor def branch: creating a gitlab connector seeds exactly one
    // empty project row (addGitLabProject) so the editor never starts blank.
    setForm((current) => ({ ...current, type, gitlabProjects: type === 'gitlab' && (current.gitlabProjects ?? []).length === 0 ? [{ ...emptyGitLabProject }] : current.gitlabProjects }));
    setCreateStep('form');
    setPrereqExpanded(() => false);
  }
  function openEdit(source: DataSource) { if (!canManage) return; setEditing(source); setCreateStep('form'); setForm(dataSourceFormFrom(source)); setCredentialStep(initialCredentialStepState((source as { credentials?: { credentials?: { configured?: unknown } } })?.credentials?.credentials?.configured === true)); setResourceSource(null); setMessage(null); setPrereqExpanded(() => false); }
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
    // Vue nextStep step-2 gitlab branch: after syncing the rows into settings,
    // at least one non-empty project_id must exist or the submit is blocked
    // with MessagePlugin.warning(projectRequired).
    if (form.type === 'gitlab' && !(form.gitlabProjects ?? []).some((project) => project.project_id.trim())) {
      setMessage({ tone: 'warning', text: t('dataSource.gitlab.projectRequired') });
      return;
    }
    setSaving(true); setMessage(null);
    try {
      // Vue treats the serialized rss header rows as part of the credential
      // draft: the connection test and the /credentials commit run when rows
      // exist even with an empty credentialsText.
      const rssAuthHeadersSerialized = form.type === 'rss' ? serializeAuthHeaders(form.authHeaders ?? []) : '';
      const input = buildDataSourceInput(form);
      const config = input.config as Record<string, unknown>;
      const credentials = config.credentials as Record<string, unknown>;
      if (!form.credentialsText.trim() && !rssAuthHeadersSerialized) {
        // skip: no draft typed, nothing to test or commit
      } else {
        const result = await dataSources.validateCredentials(form.type, credentials);
        const failure = connectionError(result, t('dataSource.testFailed')); if (failure) throw new Error(failure);
      }
      if (editing && !form.credentialsText.trim()) input.config = { ...config, credentials: undefined };
      const saved = editing ? await dataSources.update(editing.id, input) : await dataSources.create({ ...input, knowledge_base_id: knowledgeBaseId });
      if (form.credentialsText.trim() || rssAuthHeadersSerialized) {
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
  // SP2-a Task 10: the card delete action opens the dual-choice panel; the
  // checkbox starts unchecked (keep documents — the existing promise) and
  // the count loads behind the panel. A failed count leaves the panel on the
  // count-less purge copy instead of a misleading number.
  async function remove(source: DataSource) {
    if (!canManage) return;
    const request = ++deleteCountRequest.current;
    setDeleteSource(source);
    setDeletePurge(false);
    setDeleteCount(null);
    setDeleteCountLoading(true);
    try {
      const count = await dataSources.documentsCount(source.id);
      if (deleteCountRequest.current === request) setDeleteCount(count);
    } catch {
      // count unavailable — the panel degrades to the count-less copy
    } finally {
      if (deleteCountRequest.current === request) setDeleteCountLoading(false);
    }
  }
  // Confirm runs the dual-choice delete: purgeDocuments carries the checkbox
  // into DELETE ?purge_documents=true (Task 9) and the success toast
  // distinguishes purged from kept documents.
  async function confirmDelete() {
    if (!canManage || !deleteSource) return;
    const purge = deletePurge;
    setAction(`delete:${deleteSource.id}`);
    try {
      await dataSources.remove(deleteSource.id, { purgeDocuments: purge });
      setDeleteSource(null);
      await load();
      setMessage({ tone: 'success', text: purge ? t('dataSource.deleteSuccessPurged') : t('dataSource.deleteSuccess') });
    } catch (error) {
      setMessage({ tone: 'error', text: error instanceof Error ? error.message : t('dataSource.deleteFailed') });
    } finally { setAction(null); }
  }
  // Vue confirmRemoveCredentials: DELETE the /credentials subresource of the
  // edited data source; success falls back to the unconfigured row with the
  // removedToast. The typed api-client ships removeCredentials, so the call is
  // direct and backend failures surface the Vue removeFailed copy.
  async function confirmRemoveCredentials() {
    if (!editing) return;
    setAction('remove-credentials');
    try {
      await dataSources.removeCredentials(editing.id);
      setCredentialStep((current) => credentialStepReducer(current, 'remove-confirmed'));
      setForm((current) => ({ ...current, credentialsText: '' }));
      setMessage({ tone: 'success', text: t('dataSource.credential.removedToast') });
    } catch (error) {
      setMessage({ tone: 'error', text: error instanceof Error ? error.message : t('dataSource.credential.removeFailed') });
    } finally { setAction(null); }
  }
  async function run(source: DataSource, operation: 'sync' | 'pause' | 'resume' | 'validate') { if (!canManage) return; setAction(`${operation}:${source.id}`); try { const result = operation === 'sync' ? await dataSources.sync(source.id) : operation === 'pause' ? await dataSources.pause(source.id) : operation === 'resume' ? await dataSources.resume(source.id) : await dataSources.validate(source.id); const failure = connectionError(result, t('dataSource.testFailed')); if (failure) throw new Error(failure); if (operation !== 'validate') await load(); const successText = operation === 'sync' ? t('dataSource.syncTriggered') : operation === 'validate' ? t('dataSource.testSuccess') : operation === 'pause' ? t('dataSource.paused') : t('dataSource.resumed'); setMessage({ tone: 'success', text: successText }); } catch (error) { const fallback = operation === 'sync' ? t('dataSource.syncFailed') : operation === 'validate' ? t('dataSource.testFailed') : operation === 'pause' ? t('dataSource.pauseFailed') : t('dataSource.pauseFailed'); setMessage({ tone: 'error', text: error instanceof Error ? error.message : fallback }); } finally { setAction(null); } }
  // SP2-a cooperative cancel: POST /datasource/:id/logs/:log_id/cancel answers
  // 202 cancel_requested and the sync loop exits at its next checkpoint, so
  // the row is deliberately NOT flipped here — the 3s poll converges the
  // running pill to canceled on its own.
  async function cancelSync(source: DataSource) {
    if (!canManage) return;
    const log = source.latest_sync_log;
    if (!log || log.status !== 'running') return;
    setAction(`cancel:${source.id}`);
    try {
      await dataSources.cancelSyncLog(source.id, log.id);
    } catch (error) {
      setMessage({ tone: 'error', text: error instanceof Error ? error.message : String(error) });
    } finally { setAction(null); }
  }
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
  async function showLogs(source: DataSource) { setLogsSource(source); setLogs([]); setLogsOffset(0); setLogsHasNext(false); setLogsError(null); setRetrySelection(new Set()); await loadLogs(source, 0); }
  function toggleRetrySelection(externalId: string) {
    setRetrySelection((current) => {
      const next = new Set(current);
      if (next.has(externalId)) next.delete(externalId);
      else next.add(externalId);
      return next;
    });
  }
  // SP2-b targeted retry: the checked failure samples' external ids become one
  // scoped reindex run (POST /datasource/:id/reindex, 202) that converges into
  // its own SyncLog — a run can be status=success with per-item failures, so
  // the log rows render result.errors, not status. A fresh crypto.randomUUID()
  // rides each click (craft routes precedent) so the backend's enqueue
  // idempotency never collides; the 409 duplicate rejection (same request_id
  // still queued) maps to the duplicateRequest toast. The panel reloads its
  // first page so the scoped run's row shows up right away.
  async function retrySelected() {
    if (!canManage || !logsSource || retrySelection.size === 0) return;
    const externalIds = [...retrySelection];
    setAction(`reindex:${logsSource.id}`);
    try {
      await dataSources.reindexItems(logsSource.id, externalIds, crypto.randomUUID());
      setRetrySelection(new Set());
      setMessage({ tone: 'success', text: t('dataSource.reindexSubmitted') });
      await loadLogs(logsSource, 0);
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) setMessage({ tone: 'warning', text: t('dataSource.duplicateRequest') });
      else setMessage({ tone: 'error', text: error instanceof Error ? error.message : String(error) });
    } finally { setAction(null); }
  }
  async function loadResources(source: DataSource, parentId: string | undefined, trail: Array<{ id: string; name: string }>) { setResourceLoading(true); setResourceError(null); try { const next = await dataSources.resources(source.id, parentId); setResources((current) => { const byId = new Map(current.map((resource) => [resource.external_id, resource])); for (const resource of next) byId.set(resource.external_id, resource); return [...byId.values()]; }); setResourceParent(parentId); setResourceTrail(trail); } catch (error) { const classified = classifyDataSourceError(error, t('dataSource.resourceLoadFailed')); setResourceError(classified.kind === 'forbidden' ? t('dataSource.drive.loadForbiddenHint') : classified.kind === 'auth' ? t('dataSource.drive.loadAuthHint') : classified.kind === 'not-found' ? t('dataSource.drive.loadNotFoundHint') : classified.message); } finally { setResourceLoading(false); } }
  async function revealResourceSelections(source: DataSource) { const ids = dataSourceFormFrom(source).resourceIds; if (ids.length === 0) return; setResourceLoading(true); try { const ancestors = await dataSources.resourceAncestors(source.id, ids); const byId = new Map((await dataSources.resources(source.id)).map((resource) => [resource.external_id, resource])); for (const ancestor of ancestors) for (const child of await dataSources.resources(source.id, ancestor)) byId.set(child.external_id, child); setResources([...byId.values()]); } catch (error) { const classified = classifyDataSourceError(error, t('dataSource.resourceLoadFailed')); setResourceError(classified.kind === 'forbidden' ? t('dataSource.drive.loadForbiddenHint') : classified.kind === 'auth' ? t('dataSource.drive.loadAuthHint') : classified.kind === 'not-found' ? t('dataSource.drive.loadNotFoundHint') : classified.message); } finally { setResourceLoading(false); } }
  async function showResources(source: DataSource) {
    const nextForm = dataSourceFormFrom(source);
    setResourceSource(source);
    setForm(nextForm);
    setResources([]);
    setResourceParent(undefined);
    setResourceTrail([]);
    if (isDriveConnector(source.type)) {
      // Vue nextStep → step 2: prefill the saved root folder_token (the root
      // is the first "token[:fileToken]" segment) and auto-load when one
      // exists; otherwise the placeholder stays until the user loads.
      const token = nextForm.resourceIds[0]?.split(':')[0] ?? '';
      setDriveToken(token);
      setDriveTokenError(false);
      setDriveRootLoaded(false);
      if (token) await loadDriveRoot(source, token);
      return;
    }
    setDriveToken('');
    await loadResources(source, undefined, []);
    await revealResourceSelections(source);
  }
  // Vue loadDriveRoot: extract the folder_token (bare or from a Drive folder
  // URL), persist resource_ids=[token] via the main PUT so the backend lists
  // that root, then load its children. An empty token flags the inline
  // required error instead of calling the API; load failures reuse the shared
  // Drive error classification.
  async function loadDriveRoot(source: DataSource, explicitToken?: string) {
    const token = extractDriveFolderToken(explicitToken ?? driveToken);
    if (!token) { setDriveTokenError(true); return; }
    setDriveTokenError(false);
    setDriveToken(token);
    setForm((current) => ({ ...current, resourceIds: [token] }));
    setDriveRootLoaded(false);
    setResourceLoading(true);
    try {
      const input = buildDataSourceInput({ ...form, resourceIds: [token] });
      await dataSources.update(source.id, { ...input, knowledge_base_id: knowledgeBaseId });
      await loadResources(source, undefined, []);
      setDriveRootLoaded(true);
    } catch (error) {
      const classified = classifyDataSourceError(error, t('dataSource.resourceLoadFailed'));
      setResourceError(classified.kind === 'forbidden' ? t('dataSource.drive.loadForbiddenHint') : classified.kind === 'auth' ? t('dataSource.drive.loadAuthHint') : classified.kind === 'not-found' ? t('dataSource.drive.loadNotFoundHint') : classified.message);
    } finally { setResourceLoading(false); }
  }
  async function expandAllResources() { if (!resourceSource) return; setResourceLoading(true); setResourceError(null); try { const byId = new Map(resources.map((resource) => [resource.external_id, resource])); const queue = resources.filter((resource) => resource.has_children).map((resource) => resource.external_id); const visited = new Set<string>(); while (queue.length > 0) { const parentId = queue.shift()!; if (visited.has(parentId)) continue; visited.add(parentId); const children = await dataSources.resources(resourceSource.id, parentId); for (const child of children) { byId.set(child.external_id, child); if (child.has_children) queue.push(child.external_id); } } setResources([...byId.values()]); } catch (error) { const classified = classifyDataSourceError(error, t('dataSource.resourceLoadFailed')); setResourceError(classified.kind === 'forbidden' ? t('dataSource.drive.loadForbiddenHint') : classified.kind === 'auth' ? t('dataSource.drive.loadAuthHint') : classified.kind === 'not-found' ? t('dataSource.drive.loadNotFoundHint') : classified.message); } finally { setResourceLoading(false); } }
  const resourceStates = resourceCheckStates(resources, form.resourceIds);
  // SP2-b targeted retry: only failure samples that carry an external_id can
  // be refetched by id; dedupe across the visible runs so the header action
  // appears exactly when there is something selectable.
  const retryableFailedCount = new Set(logs.flatMap((log) => failedItemSamples(log).map((sample) => (typeof sample.external_id === 'string' ? sample.external_id : ''))).filter((id) => id !== '')).size;
  // Vue currentDef: the connectorDefs entry for the edited type drives both
  // the prereq setup-guide and the docHint/openDoc alert.
  const guide = VUE_CONNECTOR_GUIDES[form.type];
  const kind = credentialStepKind({ isEdit: Boolean(editing), credentialsConfigured: credentialStep.credentialsConfigured, replaceMode: credentialStep.replaceMode });
  const visibleResources = resources.filter((resource) => resource.parent_id === resourceParent || (!resourceParent && !resource.parent_id));
  const createTypes = VUE_CREATE_CONNECTOR_ORDER.flatMap((type) => {
    const connector = types.find((item) => item.type === type);
    return connector ? [connector] : [];
  });
  const editorTitle = editing === null && createStep === 'type' ? t('dataSource.step.selectType') : editing ? t('dataSource.editTitle') : t('dataSource.createTitle');
  const editorSurface = editing === undefined ? null : <Drawer visible header={editorTitle} onClose={() => setEditing(undefined)} size="640px" placement="right" className="wk-data-source-drawer">
    {editing === null && createStep === 'type' ? <div className="grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-3">
      {createTypes.map((type) => <button key={type.type} type="button" className="flex min-h-[92px] flex-col items-start gap-1 rounded-[10px] border border-line-soft bg-white px-4 py-3 text-left transition-[border-color,box-shadow] duration-200 hover:border-primary hover:shadow-sm focus-visible:outline-2 focus-visible:outline-primary focus-visible:outline-offset-2" onClick={() => chooseCreateType(type.type)}><strong>{t(`dataSource.connector.${type.type}`)}</strong><span className="text-xs leading-5 text-muted">{t(`dataSource.connectorDesc.${type.type}`)}</span></button>)}
    </div> : <form className="wk-wiki-editor grid gap-3" onSubmit={(event) => void save(event)}>
      {guide && guide.requiredPermissions.length > 0 ? <div className="wk-data-source-prereq grid gap-2 rounded-[10px] border border-line-soft bg-[#f7f9fc] p-3" data-kind="setup-guide">
        <button type="button" aria-expanded={prereqExpanded} className="flex items-center gap-1.5 bg-transparent p-0 text-left text-[13px] font-medium text-primary" onClick={() => setPrereqExpanded((current) => !current)}><span aria-hidden="true">ⓘ</span><span className="flex-1">{prereqCopy(t, `dataSource.prereqBarText_${form.type}`, 'dataSource.prereqBarText')}</span><span aria-hidden="true">{prereqExpanded ? '▴' : '▾'}</span></button>
        {prereqExpanded ? <div className="grid gap-2" data-kind="setup-guide-body">
          <ol className="m-0 grid list-none gap-2 p-0">
            <li className="grid gap-0.5 text-[13px]" data-step="1"><span className="font-medium">{prereqCopy(t, `dataSource.prereqStep1Brief_${form.type}`, 'dataSource.prereqBotBrief')}</span><span className="text-muted">{prereqCopy(t, `dataSource.prereqStep1Desc_${form.type}`, 'dataSource.prereqBotDesc')}</span></li>
            <li className="grid gap-0.5 text-[13px]" data-step="2"><span className="font-medium">{prereqCopy(t, `dataSource.prereqStep2Brief_${form.type}`, 'dataSource.prereqPermBrief')}</span>{hasKey(t, `dataSource.prereqStep2Desc_${form.type}`) ? <span className="text-muted">{t(`dataSource.prereqStep2Desc_${form.type}`)}</span> : <span className="flex flex-wrap gap-1">{guide.requiredPermissions.map((perm) => <code key={perm} className="rounded bg-[#eef2f7] px-1.5 py-0.5 font-mono text-[11px] text-muted">{perm}</code>)}</span>}</li>
            <li className="grid gap-0.5 text-[13px]" data-step="3"><span className="font-medium">{prereqCopy(t, `dataSource.prereqStep3Brief_${form.type}`, 'dataSource.prereqMemberBrief')}</span><span className="text-muted">{prereqCopy(t, `dataSource.prereqStep3Desc_${form.type}`, 'dataSource.prereqMemberDesc')}</span></li>
          </ol>
          {guide.permissionPageUrl ? <a className="inline-flex items-center gap-1 justify-self-start text-[13px] font-medium text-primary underline" href={guide.permissionPageUrl} target="_blank" rel="noopener">{prereqCopy(t, `dataSource.prereqOpenConsole_${form.type}`, 'dataSource.prereqOpenConsole')}<span aria-hidden="true">↗</span></a> : null}
        </div> : null}
      </div> : null}
      {guide?.docUrl ? <div className="wk-data-source-doc-hint flex flex-wrap items-center gap-2 rounded-md border border-primary/20 bg-primary/5 px-3 py-2 text-[13px]" data-kind="doc-hint"><span>{t('dataSource.docHint')}</span><a className="inline-flex items-center gap-1 font-medium text-primary underline" href={guide.docUrl} target="_blank" rel="noopener">{t('dataSource.openDoc')}</a></div> : null}
      <p className="wk-muted m-0 text-muted">{t('dataSource.credentialsLabel')}</p>
      <label>{t('dataSource.nameLabel')} <Input value={form.name} onChange={(value) => updateForm('name', String(value))} /></label>
      <label>{t('dataSource.connectorTypeLabel')} <Select value={form.type} onChange={(value) => updateForm('type', String(value))}>{(editing ? types : createTypes).map((type) => <Select.Option key={type.type} value={type.type} label={`${type.name} (${type.type})`}>{type.name} ({type.type})</Select.Option>)}</Select></label>
      {kind === 'configured' ? (credentialStep.pendingRemove ? <div className="grid gap-2 rounded-lg border border-danger/40 bg-danger/5 p-3 text-[13px]" data-kind="confirm-remove"><span className="text-danger">{t('dataSource.credential.confirmRemovePrompt')}</span><div className="flex items-center gap-2"><Button type="button" theme="default" variant="text" size="small" onClick={() => setCredentialStep((current) => credentialStepReducer(current, 'cancel-remove'))}>{t('common.cancel')}</Button><Button type="button" theme="danger" variant="text" size="small" loading={action === 'remove-credentials'} onClick={() => void confirmRemoveCredentials()}>{t('dataSource.credential.confirmRemove')}</Button></div></div> : <div className="flex items-center gap-2 rounded-lg border border-line-soft p-3 text-[13px]" data-kind="configured"><span className="text-success-text" aria-hidden="true">✓</span><span>{t('dataSource.credential.configured')}</span><div className="ml-auto flex items-center gap-2"><Button type="button" theme="default" variant="text" size="small" onClick={() => setCredentialStep((current) => credentialStepReducer(current, 'enter-replace'))}>{t('dataSource.credential.update')}</Button><Button type="button" theme="danger" variant="text" size="small" onClick={() => setCredentialStep((current) => credentialStepReducer(current, 'request-remove'))}>{t('dataSource.credential.remove')}</Button></div></div>) : null}
      {kind === 'unconfigured' ? <div className="flex items-center gap-2 rounded-lg border border-dashed border-line-soft p-3 text-[13px] text-muted" data-kind="unconfigured"><span>{t('dataSource.credential.unconfigured')}</span><Button type="button" theme="default" variant="text" size="small" className="ml-auto" onClick={() => setCredentialStep((current) => credentialStepReducer(current, 'enter-replace'))}>{t('dataSource.credential.configure')}</Button></div> : null}
      {kind === 'inputs' ? <fieldset className="grid gap-3 rounded-lg border border-line-soft p-3"><legend className="px-1 text-[13px] font-semibold">{t('dataSource.credentialsLabel')}</legend>{VUE_CREDENTIAL_FIELDS[form.type] ? VUE_CREDENTIAL_FIELDS[form.type].map((field) => <label key={field.key} className="grid gap-1 text-[13px]"><span>{t(field.label)}</span><Input type={field.secret ? 'password' : 'text'} placeholder={field.placeholder || t('dataSource.credential.inputPlaceholder')} value={credentialValue(form.credentialsText, field.key)} onChange={(value) => updateForm('credentialsText', setCredentialValue(form.credentialsText, field.key, String(value)))} />{field.hint ? <small className="text-muted">{t(field.hint)}</small> : null}</label>) : form.type === 'rss' ? <div className="grid gap-2" data-kind="rss-auth-headers"><div className="flex items-center justify-between gap-2"><span className="text-[13px] font-semibold">{t('dataSource.field.authHeaders')}</span><Button type="button" theme="default" variant="text" size="small" onClick={() => updateForm('authHeaders', [...(form.authHeaders ?? []), { key: '', value: '' }])}>{t('dataSource.credential.headerAdd')}</Button></div><small className="text-muted">{t('dataSource.field.authHeadersHint')}</small>{(form.authHeaders ?? []).map((row, index) => <div key={index} className="flex items-center gap-2"><Input aria-label={t('dataSource.credential.headerKeyPlaceholder')} autocomplete="off" placeholder={t('dataSource.credential.headerKeyPlaceholder')} value={row.key} onChange={(value) => updateForm('authHeaders', (form.authHeaders ?? []).map((item, at) => at === index ? { ...item, key: String(value) } : item))} /><Input aria-label={t('dataSource.credential.headerValuePlaceholder')} autocomplete="off" placeholder={t('dataSource.credential.headerValuePlaceholder')} value={row.value} onChange={(value) => updateForm('authHeaders', (form.authHeaders ?? []).map((item, at) => at === index ? { ...item, value: String(value) } : item))} /><Button type="button" theme="default" variant="text" size="small" aria-label={t('common.delete')} onClick={() => updateForm('authHeaders', (form.authHeaders ?? []).filter((_, at) => at !== index))}>×</Button></div>)}</div> : <Textarea autosize={{ minRows: 4, maxRows: 4 }} value={form.credentialsText} onChange={(value) => updateForm('credentialsText', String(value))} placeholder={t('dataSource.credentialsPlaceholder')} />}</fieldset> : null}
      {editing && credentialStep.replaceMode ? <Button type="button" theme="default" variant="text" size="small" className="justify-self-start" onClick={() => { setCredentialStep((current) => credentialStepReducer(current, 'cancel-replace')); setForm((current) => ({ ...current, credentialsText: '', authHeaders: [] })); }}>{t('common.cancel')}</Button> : null}
      <fieldset className="grid gap-3 rounded-lg border border-line-soft p-3"><legend className="px-1 text-[13px] font-semibold">{t('dataSource.connectorSettingsLabel')}</legend>{form.type === 'gitlab' ? <div className="grid gap-2" data-kind="gitlab-projects"><div className="flex items-center justify-between gap-2"><span className="text-[13px] font-semibold">{t('dataSource.gitlab.projects')}</span><Button type="button" theme="default" variant="text" size="small" onClick={() => updateForm('gitlabProjects', [...(form.gitlabProjects ?? []), { ...emptyGitLabProject }])}>{t('dataSource.gitlab.addProject')}</Button></div><small className="text-muted">{t('dataSource.gitlab.projectsHint')}</small>{(form.gitlabProjects ?? []).map((project, index) => <div key={index} className="grid gap-2 rounded-lg border border-line-soft p-3" data-kind="gitlab-project-row"><div className="flex items-center justify-between"><strong className="text-[13px]">{t('dataSource.gitlab.project')} {index + 1}</strong><Button type="button" theme="danger" variant="text" size="small" aria-label={t('common.delete')} onClick={() => updateForm('gitlabProjects', (form.gitlabProjects ?? []).filter((_, at) => at !== index))}>×</Button></div><label className="grid gap-1 text-[13px]"><span>{t('dataSource.gitlab.projectId')} *</span><Input placeholder={t('dataSource.gitlab.projectIdPlaceholder')} value={project.project_id} onChange={(value) => updateForm('gitlabProjects', (form.gitlabProjects ?? []).map((item, at) => at === index ? { ...item, project_id: String(value) } : item))} /></label><label className="grid gap-1 text-[13px]"><span>{t('dataSource.gitlab.ref')}</span><Input placeholder={t('dataSource.gitlab.refPlaceholder')} value={project.ref} onChange={(value) => updateForm('gitlabProjects', (form.gitlabProjects ?? []).map((item, at) => at === index ? { ...item, ref: String(value) } : item))} /></label><label className="grid gap-1 text-[13px]"><span>{t('dataSource.gitlab.paths')}</span><Textarea autosize={{ minRows: 2, maxRows: 2 }} placeholder={t('dataSource.gitlab.pathsPlaceholder')} value={project.pathsText} onChange={(value) => updateForm('gitlabProjects', (form.gitlabProjects ?? []).map((item, at) => at === index ? { ...item, pathsText: String(value) } : item))} /></label></div>)}</div> : VUE_SETTINGS_FIELDS[form.type] ? VUE_SETTINGS_FIELDS[form.type].map((field) => <label key={field.key} className="grid gap-1 text-[13px]"><span>{t(field.label)}</span><Textarea autosize={{ minRows: 3, maxRows: 3 }} placeholder={field.placeholder || t('dataSource.credential.inputPlaceholder')} value={credentialValue(form.settingsText, field.key)} onChange={(value) => updateForm('settingsText', setCredentialValue(form.settingsText, field.key, String(value)))} />{field.hint ? <small className="text-muted">{t(field.hint)}</small> : null}</label>) : <Textarea autosize={{ minRows: 4, maxRows: 4 }} value={form.settingsText} onChange={(value) => updateForm('settingsText', String(value))} placeholder={t('dataSource.settingsPlaceholder')} />}</fieldset>
      <label>{t('dataSource.syncScheduleLabel')} <Input value={form.schedule} onChange={(value) => updateForm('schedule', String(value))} /></label>
      <label>{t('dataSource.syncModeLabel')} <Select value={form.mode} onChange={(value) => updateForm('mode', value as DataSourceFormValues['mode'])}><Select.Option value="incremental" label={t('dataSource.syncMode.incremental')}>{t('dataSource.syncMode.incremental')}</Select.Option><Select.Option value="full" label={t('dataSource.syncMode.full')}>{t('dataSource.syncMode.full')}</Select.Option></Select></label>
      <label>{t('dataSource.conflictLabel')} <Select value={form.conflict} onChange={(value) => updateForm('conflict', value as DataSourceFormValues['conflict'])}><Select.Option value="overwrite" label={t('dataSource.conflict.overwrite')}>{t('dataSource.conflict.overwrite')}</Select.Option><Select.Option value="skip" label={t('dataSource.conflict.skip')}>{t('dataSource.conflict.skip')}</Select.Option></Select></label>
      <label><Checkbox checked={form.deletions} onChange={(checked) => updateForm('deletions', checked)} label={t('dataSource.syncDeletions')} /></label>
      <Button type="submit" theme="default" variant="outline" loading={saving}>{t('dataSource.save')}</Button>
    </form>}
  </Drawer>;
  // SP2-a Task 10 dual-choice delete panel: keep-documents promise by
  // default; the checked state swaps the body to the red irreversible purge
  // warning and the confirm button to deleteAndPurge. Count-less fallbacks
  // cover a failed/unresolved documentsCount. S5 评审 Important 回收：对齐
  // Vue DataSourceSettings.vue:414-441 的居中 t-dialog（width=440、
  // close-on-overlay-click=false、footer cancel/confirm 按钮 + confirm
  // theme=danger/loading、checkbox 随提交禁用），不再用右置 Drawer 自绘按钮。
  const purgeLabelText = deleteCount !== null ? t('dataSource.deletePanelPurgeLabel', { count: deleteCount }) : t('dataSource.deletePanelPurgeLabelUnknown');
  const purgeWarningText = deleteCount !== null ? t('dataSource.deletePanelPurgeWarning', { count: deleteCount }) : t('dataSource.deletePanelPurgeWarningUnknown');
  const deleteSubmitting = deleteSource === null ? false : action === `delete:${deleteSource.id}`;
  const deleteSurface = deleteSource === null ? null : <Dialog
    visible
    header={t('dataSource.deletePanelTitle', { name: deleteSource.name })}
    confirmBtn={{ content: deletePurge ? t('dataSource.deleteAndPurge') : t('dataSource.delete'), theme: 'danger', loading: deleteSubmitting }}
    cancelBtn={{ content: t('common.cancel'), disabled: deleteSubmitting }}
    closeOnOverlayClick={false}
    width={440}
    onClose={() => setDeleteSource(null)}
    onConfirm={() => void confirmDelete()}
    className="wk-data-source-delete-dialog"
  >
    <div className="grid gap-3 text-[13px]" data-kind="delete-panel">
      <p className="m-0" data-kind={deletePurge ? 'delete-purge-warning' : 'delete-keep'}>{deletePurge ? <span className="font-medium text-danger">{purgeWarningText}</span> : t('dataSource.deletePanelKeep')}</p>
      {deleteCountLoading ? <p className="m-0 text-muted" data-kind="delete-count-loading">{t('common.loading')}</p> : deleteCount !== null ? <p className="m-0 text-muted" data-kind="delete-count">{t('dataSource.deletePanelCount', { count: deleteCount })}</p> : null}
      <label className="flex items-center gap-2"><Checkbox checked={deletePurge} disabled={deleteSubmitting} onChange={(checked) => setDeletePurge(checked)} label={purgeLabelText} /></label>
    </div>
  </Dialog>;
  // R484 (R482 B1 差异4): embedded inside the KB settings drawer the page
  // header stays hidden — the drawer's tab heading already renders the Vue
  // DataSourceSettings title/description — and the standalone page frame
  // classes drop away.
  return <main data-can-manage={canManage ? 'true' : 'false'} data-embedded={embedded ? 'true' : undefined} className={embedded ? 'wk-data-source-root' : 'wk-data-source-root wk-page max-w-[1180px]! mx-auto box-border px-[1.25rem] py-12'}>{embedded ? null : <header className="wk-header mb-7 flex items-start gap-4"><div><p className="wk-eyebrow m-0 text-[0.78rem] font-bold uppercase tracking-[0.08em] text-primary">{t('common.knowledgeBases')} · {knowledgeBaseId}</p><h1 className="text-[clamp(1.8rem,5vw,2.5rem)] my-[0.35rem]">{t('dataSource.title')}</h1><p className="wk-muted text-muted">{t('dataSource.description')}</p></div></header>}
    <Card>{message ? <Status tone={message.tone}>{message.text}</Status> : null}{loading ? <Status>{t('common.loading')}</Status> : accessDenied ? <Status tone="error">{t('dataSource.permissionRequired')}</Status> : sources.length === 0 && !canManage ? <Status>{t('dataSource.empty')}</Status> : sources.length === 0 ? <div className="grid min-h-[120px] grid-cols-[repeat(auto-fill,minmax(320px,1fr))] gap-3"><button type="button" className="wk-data-source-create flex min-h-[68px] w-full flex-col items-center justify-center gap-2 rounded-[10px] border border-dashed border-line-soft bg-transparent px-4 py-[14px] text-[13px] font-medium text-muted transition-[color,border-color,background-color] duration-200 hover:border-primary hover:bg-primary/5 hover:text-primary focus-visible:outline-2 focus-visible:outline-primary focus-visible:outline-offset-2" onClick={openCreate}><span className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10 text-primary" aria-hidden="true"><KbIcon name="add" size={16} /></span><span>{t('dataSource.add')}</span></button></div> : <div className="wk-data-source-grid grid grid-cols-[repeat(auto-fill,minmax(320px,1fr))] gap-3">{sources.map((source) => { const log = source.latest_sync_log; const fullTime = source.last_sync_at ? new Date(source.last_sync_at).toLocaleString() : ''; return <div key={source.id} className={`wk-data-source-card flex min-w-0 items-start gap-3 rounded-[10px] border border-line-soft bg-white px-4 py-[14px] text-left transition-[border-color,box-shadow] duration-200 ${canManage ? 'cursor-pointer hover:border-primary hover:shadow-[0_4px_14px_rgba(15,23,42,0.06)] focus-visible:outline-2 focus-visible:outline-primary focus-visible:outline-offset-2' : ''}`} onClick={canManage ? () => openEdit(source) : undefined} role={canManage ? 'button' : undefined} tabIndex={canManage ? 0 : undefined}><span aria-hidden="true" className="wk-data-source-badge flex h-9 w-9 flex-none items-center justify-center rounded-[9px] bg-[rgba(7,192,95,0.12)] text-[15px] font-semibold tracking-[0.02em] text-[#07c05f]">{(source.type.slice(0, 1) || '?').toUpperCase()}</span><div className="min-w-0 flex-1"><div className="flex min-w-0 items-center gap-1.5"><strong title={source.name} className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap text-sm font-semibold leading-snug">{source.name}</strong><div className="wk-data-source-actions ml-auto flex flex-none flex-wrap items-center justify-end gap-[0.5rem]" onClick={(event) => event.stopPropagation()}>{canManage ? <><Button type="button" theme="default" variant="outline" onClick={() => openEdit(source)}>{t('dataSource.edit')}</Button><Button type="button" theme="default" variant="outline" disabled={action !== null} onClick={() => void run(source, 'validate')}>{t('dataSource.testConnection')}</Button><Button type="button" theme="default" variant="outline" disabled={action !== null || isSyncRunning(source)} onClick={() => void run(source, 'sync')}>{isSyncRunning(source) ? t('dataSource.status.running') : t('dataSource.syncNow')}</Button>{isSyncRunning(source) ? <Button type="button" theme="default" variant="outline" disabled={action !== null} onClick={() => void cancelSync(source)}>{t('dataSource.cancelSync')}</Button> : null}{source.status === 'paused' ? <Button type="button" theme="default" variant="outline" disabled={action !== null} onClick={() => void run(source, 'resume')}>{t('dataSource.resume')}</Button> : source.status === 'active' ? <Button type="button" theme="default" variant="outline" disabled={action !== null} onClick={() => void run(source, 'pause')}>{t('dataSource.pause')}</Button> : null}<Button type="button" theme="default" variant="outline" disabled={action !== null} onClick={() => void remove(source)}>{t('dataSource.delete')}</Button></> : null}<Button type="button" theme="default" variant="outline" onClick={() => void showLogs(source)}>{t('dataSource.logs')}</Button></div></div><p className="wk-data-source-card-subtitle mt-0.5 flex flex-wrap items-center gap-1 text-xs leading-normal text-muted"><span>{t(`dataSource.connector.${source.type}`)} · {source.sync_mode ? t(`dataSource.syncMode.${source.sync_mode}`) : t('dataSource.syncMode.incremental')}</span><span className={`inline-flex items-center gap-1 ${statusToneClass(source.status)}`}><span aria-hidden="true" className="h-1.5 w-1.5 flex-none rounded-full bg-current" />{localizedSyncStatus(t, source.status)}</span></p><p className="wk-data-source-card-detail mt-1 flex flex-wrap items-center gap-1 text-xs leading-snug text-muted"><span>{humanizeCron(source.sync_schedule, t)}</span><span aria-hidden="true">·</span><span title={fullTime || undefined}>{relativeTime(source.last_sync_at, t)}</span>{log ? <><span aria-hidden="true">·</span><span className={`font-medium ${syncResultToneClass(log.status)}`}>{localizedSyncStatus(t, log.status)}</span>{syncResultPills(source, t).map((pill) => <span key={pill.kind} className="wk-data-source-metric font-mono text-[11px] tabular-nums text-muted">{pill.text}</span>)}</> : null}</p>{source.error_message ? <div className="mt-2 flex items-start gap-1.5 rounded-md bg-danger/10 px-2.5 py-2 text-xs leading-snug text-danger">{source.error_message}</div> : null}</div></div>; })}{canManage ? <button type="button" className="wk-data-source-create flex min-h-[68px] w-full flex-col items-center justify-center gap-2 rounded-[10px] border border-dashed border-line-soft bg-transparent px-4 py-[14px] text-[13px] font-medium text-muted transition-[color,border-color,background-color] duration-200 hover:border-primary hover:bg-primary/5 hover:text-primary focus-visible:outline-2 focus-visible:outline-primary focus-visible:outline-offset-2" onClick={openCreate}><span className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10 text-primary" aria-hidden="true"><KbIcon name="add" size={16} /></span><span>{t('dataSource.add')}</span></button> : null}</div>}</Card>
    {logsSource ? <Card className="mt-4"><div className="wk-header mb-6 flex items-start justify-between gap-4"><div><h2 className="m-0">{t('dataSource.syncHistory')} · {logsSource.name}</h2><p className="wk-muted text-muted">{t('dataSource.updateSuccessSyncHint')}</p></div><div className="flex items-center gap-2">{canManage && retryableFailedCount > 0 ? <Button type="button" theme="default" variant="outline" disabled={retrySelection.size === 0} loading={action === `reindex:${logsSource.id}`} onClick={() => void retrySelected()}>{t('dataSource.retrySelected')}</Button> : null}<Button type="button" theme="default" variant="outline" onClick={() => setLogsSource(null)}>{t('dataSource.close')}</Button></div></div>{logsLoading ? <Status>{t('common.loading')}</Status> : logsError ? <><Status tone="error">{logsError}</Status><Button type="button" theme="default" variant="outline" onClick={() => void loadLogs(logsSource, logsOffset)}>{t('common.retry')}</Button></> : logs.length === 0 ? <Status>{t('dataSource.noLogs')}</Status> : <ul className="wk-list m-0 list-none p-0">{logs.map((log) => { const samples = failedItemSamples(log); const failedCount = typeof log.items_failed === 'number' ? log.items_failed : samples.length; const overflow = Math.max(0, failedCount - samples.length); return <li key={log.id} className="border-b border-line-soft py-[0.9rem]" data-kind="sync-log"><div className="flex items-baseline justify-between gap-4"><span className="font-mono text-[0.8rem] text-muted">{localizedSyncStatus(t, log.status)}</span><small>{typeof log.started_at === 'string' ? log.started_at : ''}{typeof log.finished_at === 'string' ? ` → ${log.finished_at}` : ''}{typeof log.error_message === 'string' ? ` · ${log.error_message}` : ''}</small><span className="text-[#718096]! text-[0.75rem]! font-mono">{t('dataSource.logMetric.total')} {log.items_total ?? 0} · +{log.items_created ?? 0} · ↻{log.items_updated ?? 0} · −{log.items_deleted ?? 0} · {t('dataSource.logMetric.skipped')} {log.items_skipped ?? 0} · {t('dataSource.logMetric.failed')} {log.items_failed ?? 0}</span></div>{samples.length > 0 ? <div className="mt-2 grid gap-1 rounded-md bg-danger/5 px-2.5 py-2" data-kind="failed-items"><div className="text-[11px] font-semibold text-danger">{t('dataSource.logDetail.failedItems')} ({failedCount})</div>{samples.map((sample, index) => { const id = typeof sample.external_id === 'string' ? sample.external_id : ''; const text = formatSyncError(t, sample); return <div key={index} className="flex items-center gap-2 text-xs leading-snug text-danger" data-kind="failed-item">{canManage && id !== '' ? <Checkbox checked={retrySelection.has(id)} onChange={() => toggleRetrySelection(id)} /> : null}<span title={text} className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap">{text}</span></div>; })}{overflow > 0 ? <div className="text-[11px] text-muted" data-kind="failed-items-more">{t('dataSource.logDetail.failedItemsMore', { n: overflow })}</div> : null}</div> : null}</li>; })}</ul>}{logs.length > 0 ? <nav className="wk-pagination" aria-label={t('dataSource.syncHistory')}><Button type="button" theme="default" variant="outline" disabled={logsOffset === 0 || logsLoadingMore} onClick={() => void loadLogs(logsSource!, Math.max(0, logsOffset - LOG_PAGE_SIZE))}>{t('dataSource.back')}</Button><span>{Math.floor(logsOffset / LOG_PAGE_SIZE) + 1}</span><Button type="button" theme="default" variant="outline" disabled={!logsHasNext || logsLoadingMore} loading={logsLoadingMore} onClick={() => logsSource ? void loadLogs(logsSource, logsOffset + LOG_PAGE_SIZE) : undefined}>{t('dataSource.loadMore')}</Button></nav> : null}</Card> : null}
    {resourceSource ? <Card className="mt-4"><div className="wk-header mb-6 flex items-start justify-between gap-4"><div><h2>{t('dataSource.resourceHint')} · {resourceSource.name}</h2><p className="wk-muted text-muted">{resourceTrail.length > 0 ? resourceTrail.map((item) => item.name).join(" / ") : t("dataSource.resourceHint")}</p></div><Button type="button" theme="default" variant="outline" onClick={() => setResourceSource(null)}>{t('dataSource.close')}</Button></div>{resourceSource && isDriveConnector(resourceSource.type) ? <div className="mb-4 grid gap-2 rounded-lg border border-line-soft p-3" data-kind="drive-folder-input"><label className="text-[13px] font-semibold">{t('dataSource.drive.folderTokenLabel')}</label><div className="flex items-center gap-2"><Input aria-label={t('dataSource.drive.folderTokenLabel')} value={driveToken} placeholder={t('dataSource.drive.folderTokenPlaceholder')} onChange={(value) => { setDriveToken(String(value)); setDriveTokenError(false); }} onEnter={() => void loadDriveRoot(resourceSource)} /><Button type="button" theme="default" variant="outline" loading={resourceLoading} onClick={() => void loadDriveRoot(resourceSource)}>{t('dataSource.drive.load')}</Button></div>{driveTokenError ? <small className="text-danger">{t('dataSource.drive.folderTokenRequired')}</small> : <small className="text-muted">{t('dataSource.drive.shareHint')}</small>}</div> : null}{resourceTrail.length > 0 ? <Button type="button" theme="default" variant="outline" onClick={() => { const previous = resourceTrail.slice(0, -1); void loadResources(resourceSource, previous.length > 0 ? previous[previous.length - 1]!.id : undefined, previous); }}>{t('dataSource.back')}</Button> : null}<Button type="button" theme="default" variant="outline" onClick={() => void expandAllResources()} disabled={resourceLoading || !resources.some((resource) => resource.has_children)}>{t('dataSource.loadMore')}</Button>{resourceLoading ? <Status>{t('common.loading')}</Status> : null}{resourceError ? <Status tone="error">{resourceError}</Status> : null}{!resourceLoading && !resourceError && visibleResources.length === 0 ? (resourceSource && isDriveConnector(resourceSource.type) && !driveRootLoaded ? <div className="grid gap-1 rounded-lg border border-dashed border-line-soft p-4 text-center" data-kind="drive-placeholder"><p className="m-0 text-[13px] font-semibold">{t('dataSource.drive.placeholderTitle')}</p><p className="m-0 text-[13px] text-muted">{t('dataSource.drive.placeholderDesc')}</p></div> : <Status>{t('dataSource.noResources')}</Status>) : null}{!resourceLoading && !resourceError ? <ul className="wk-list m-0 list-none p-0" role="tree">{visibleResources.map((resource) => <li key={resource.external_id} className="items-center! flex justify-between gap-4 border-b border-line-soft py-[0.9rem]" role="treeitem"><div className="grid gap-[0.2rem] min-w-0"><label className="inline-flex min-w-0 cursor-pointer items-center gap-[0.45rem]"><input type="checkbox" className="peer absolute h-px w-px opacity-0" checked={resourceStates.get(resource.external_id) === "checked"} aria-checked={resourceStates.get(resource.external_id) === "indeterminate" ? "mixed" : resourceStates.get(resource.external_id) === "checked"} data-state={resourceStates.get(resource.external_id)} onChange={() => setForm((current) => ({ ...current, resourceIds: toggleResourceSelection(resources, current.resourceIds, resource.external_id) }))} /><span className={`inline-flex h-[15px] w-[15px] flex-none items-center justify-center rounded-[3px] border font-mono text-[0.8rem] leading-none text-muted ${(resourceStates.get(resource.external_id) ?? "unchecked") === "unchecked" ? "border-[#a8b5c8] bg-white" : "border-primary bg-primary"} peer-focus-visible:outline-2 peer-focus-visible:outline-[#2e6de6] peer-focus-visible:outline-offset-2`} aria-hidden="true">{resourceStates.get(resource.external_id) === "checked" ? "✓" : resourceStates.get(resource.external_id) === "indeterminate" ? "−" : ""}</span> <strong title={resource.name} className="overflow-hidden text-ellipsis whitespace-nowrap">{resource.name}</strong></label><small className="text-[#718096]">{resource.type}</small></div>{resource.has_children ? <Button type="button" theme="default" variant="outline" onClick={() => void loadResources(resourceSource, resource.external_id, [...resourceTrail, { id: resource.external_id, name: resource.name }])}>{t('dataSource.next')}</Button> : null}</li>)}</ul> : null}</Card> : null}
    {editorSurface ? createPortal(editorSurface, document.body) : null}
    {deleteSurface ? createPortal(deleteSurface, document.body) : null}
  </main>;
}
