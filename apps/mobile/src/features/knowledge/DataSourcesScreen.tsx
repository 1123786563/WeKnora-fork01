import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ActivityIndicator, Alert, FlatList, Pressable, SafeAreaView, ScrollView, Switch, Text, TextInput, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import type { DataSource, DataSourceConnectorType, DataSourceResource } from '@weknora/api-client';
import { formatMessage } from '@weknora/i18n';
import { useMobileRuntime } from '../../runtime.tsx';
import { canManageDataSources, dataSourceConnectorLabelKey, dataSourceCredentialFields, dataSourceStatusLabel, dataSourceStatusLabelKey, dataSourceSyncModeLabelKey, extractDriveFolderToken, filterSupportedDataSourceTypes, hasRunningSync, localizedOr, resourceCheckState, resourceSelectionMarker, safeDataSourceType, toggleDataSourceResourceSelection, validateDataSourceCredentials } from './data-sources.ts';
import { buildNativeDataSourceInput, nativeDataSourceDraftFrom, type NativeDataSourceDraft } from './data-source-form.ts';

const EMPTY_DRAFT: NativeDataSourceDraft = { name: '', type: '', schedule: '0 0 */6 * * *', mode: 'incremental', conflict: 'overwrite', deletions: true, credentialsText: '', settingsText: '' };
type GitLabProjectDraft = { projectId: string; ref: string; paths: string };

function resourceActionLabel(locale: string, expanded: boolean): string {
  const labels: Record<string, [string, string]> = {
    'zh-CN': ['展开', '折叠'], 'en-US': ['Expand', 'Collapse'], 'ja-JP': ['展開', '折りたたむ'], 'ko-KR': ['펼치기', '접기'], 'ru-RU': ['Развернуть', 'Свернуть'],
  };
  const pair = labels[locale] ?? labels['en-US'];
  return expanded ? pair[1] : pair[0];
}

function connectionError(value: unknown, fallback: string): string | null {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return null;
  const row = value as Record<string, unknown>;
  if (row.success === false) return typeof row.message === 'string' ? row.message : typeof row.error === 'string' ? row.error : fallback;
  return null;
}

export function DataSourcesScreen() {
  const { id: rawId } = useLocalSearchParams<{ id: string }>();
  const knowledgeBaseId = Array.isArray(rawId) ? rawId[0] : rawId;
  const runtime = useMobileRuntime();
  const locale = runtime.locale ?? 'en-US';
  const t = useCallback((key: string, values: Record<string, string | number> = {}) => formatMessage(locale, key, values), [locale]);
  const router = useRouter();
  const workspaceRole = useMemo(() => runtime.workspaces.find((workspace) => String(workspace.id) === runtime.tenantId)?.role, [runtime.tenantId, runtime.workspaces]);
  const canManage = canManageDataSources(workspaceRole);
  const [sources, setSources] = useState<DataSource[]>([]);
  const [types, setTypes] = useState<DataSourceConnectorType[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [editing, setEditing] = useState<DataSource | null | undefined>(undefined);
  const [draft, setDraft] = useState<NativeDataSourceDraft>(EMPTY_DRAFT);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<'success' | 'error' | null>(null);
  const [resources, setResources] = useState<DataSourceResource[]>([]);
  const [selectedResourceIds, setSelectedResourceIds] = useState<string[]>([]);
  const [resourcesLoading, setResourcesLoading] = useState(false);
  const [expandedResourceIds, setExpandedResourceIds] = useState<string[]>([]);
  const [resourceLoadingId, setResourceLoadingId] = useState<string | null>(null);
  const [driveFolderToken, setDriveFolderToken] = useState('');
  const [temporarySourceId, setTemporarySourceId] = useState<string | null>(null);
  const [gitlabProjects, setGitlabProjects] = useState<GitLabProjectDraft[]>([]);
  const [rssFeedUrls, setRssFeedUrls] = useState('');
  const [initialRssFeedUrls, setInitialRssFeedUrls] = useState('');
  const [rssAuthHeaders, setRssAuthHeaders] = useState('');
  const [credentialValues, setCredentialValues] = useState<Record<string, string>>({});
  const [action, setAction] = useState<string | null>(null);
  const [logsSource, setLogsSource] = useState<DataSource | null>(null);
  const [logs, setLogs] = useState<import('@weknora/api-client').DataSourceSyncLog[]>([]);
  const [logsLoading, setLogsLoading] = useState(false);
  const [logsLoadingMore, setLogsLoadingMore] = useState(false);
  const [logsHasMore, setLogsHasMore] = useState(false);
  const [expandedLogId, setExpandedLogId] = useState<string | null>(null);
  const logsRequestId = useRef(0);
  const loadGeneration = useRef(0);
  const resourceGeneration = useRef(0);
  const resourceChildrenGeneration = useRef(0);

  const load = useCallback(async (silent = false) => {
    if (!knowledgeBaseId) { setLoading(false); setError(t('dataSource.knowledgeBaseRequired')); return; }
    const generation = ++loadGeneration.current;
    if (!silent) setLoading(true); setError('');
    const [sourceResult, typeResult] = await Promise.allSettled([
      runtime.client.dataSources.list(knowledgeBaseId),
      runtime.client.dataSources.types(),
    ]);
    if (generation !== loadGeneration.current) return;
    if (sourceResult.status === 'fulfilled') setSources(sourceResult.value); else setError(sourceResult.reason instanceof Error ? sourceResult.reason.message : t('dataSource.loadFailed'));
    if (typeResult.status === 'fulfilled') setTypes(filterSupportedDataSourceTypes(typeResult.value));
    if (!silent && generation === loadGeneration.current) setLoading(false);
  }, [knowledgeBaseId, runtime.client]);

  useEffect(() => { void load(); }, [load]);
  useEffect(() => {
    if (!sources.some(hasRunningSync)) return;
    const timer = setTimeout(() => { void load(true); }, 3000);
    return () => clearTimeout(timer);
  }, [load, sources]);
  const typeById = new Map(types.map((type) => [type.type, type]));
  function updateDraft<K extends keyof NativeDataSourceDraft>(key: K, value: NativeDataSourceDraft[K]) { setDraft((current) => ({ ...current, [key]: value })); if (key === 'type') { setCredentialValues({}); setGitlabProjects(value === 'gitlab' && gitlabProjects.length === 0 ? [{ projectId: '', ref: '', paths: '' }] : value === 'gitlab' ? gitlabProjects : []); } setTestResult(null); }
  function openCreate() { const type = types[0]?.type ?? ''; setEditing(null); setTemporarySourceId(null); setResources([]); setSelectedResourceIds([]); setExpandedResourceIds([]); setDriveFolderToken(''); setGitlabProjects(type === 'gitlab' ? [{ projectId: '', ref: '', paths: '' }] : []); setRssFeedUrls(''); setInitialRssFeedUrls(''); setRssAuthHeaders(''); setCredentialValues({}); setDraft({ ...EMPTY_DRAFT, type }); setError(''); }
  function sourceResourceIds(source: DataSource): string[] {
    const config = source.config && typeof source.config === 'object' && !Array.isArray(source.config) ? source.config as Record<string, unknown> : {};
    const value = Array.isArray(config.resource_ids) ? config.resource_ids : source.resource_ids;
    return Array.isArray(value) ? value.filter((id): id is string => typeof id === 'string') : [];
  }
  async function loadResources(source: DataSource) {
    const generation = ++resourceGeneration.current;
    setResourcesLoading(true); setError(''); setExpandedResourceIds([]);
    try {
      const next = await runtime.client.dataSources.resources(source.id);
      if (generation !== resourceGeneration.current) return;
      const byId = new Map(next.map((resource) => [resource.external_id, resource]));
      const revealedAncestors: string[] = [];
      const selectedIds = sourceResourceIds(source);
      if (selectedIds.length > 0) {
        const ancestors = await runtime.client.dataSources.resourceAncestors(source.id, selectedIds);
        for (const ancestor of ancestors) {
          if (generation !== resourceGeneration.current) return;
          const children = await runtime.client.dataSources.resources(source.id, ancestor);
          for (const child of children) byId.set(child.external_id, child);
          revealedAncestors.push(ancestor);
        }
      }
      if (generation === resourceGeneration.current) {
        setResources([...byId.values()]);
        setExpandedResourceIds(revealedAncestors);
      }
    }
    catch (cause) { if (generation === resourceGeneration.current) setError(cause instanceof Error ? cause.message : t('dataSource.resourceLoadFailed')); }
    finally { if (generation === resourceGeneration.current) setResourcesLoading(false); }
  }
  function openEdit(source: DataSource) { const ids = sourceResourceIds(source); const config = source.config && typeof source.config === 'object' && !Array.isArray(source.config) ? source.config as Record<string, unknown> : {}; const settings = config.settings && typeof config.settings === 'object' && !Array.isArray(config.settings) ? config.settings as Record<string, unknown> : {}; const projects = Array.isArray(settings.projects) ? settings.projects : []; const savedRssFeedUrls = source.type === 'rss' && typeof settings.feed_urls === 'string' ? settings.feed_urls : ''; setEditing(source); setTemporarySourceId(null); setResources([]); setSelectedResourceIds(ids); setDriveFolderToken(source.type === 'feishu_drive' || source.type === 'lark_drive' ? ids[0] ?? '' : ''); setRssFeedUrls(savedRssFeedUrls); setInitialRssFeedUrls(savedRssFeedUrls); setRssAuthHeaders(''); setCredentialValues({}); setGitlabProjects(source.type === 'gitlab' ? projects.map((project) => { const row = project as Record<string, unknown>; return { projectId: typeof row.project_id === 'string' ? row.project_id : '', ref: typeof row.ref === 'string' ? row.ref : '', paths: Array.isArray(row.paths) ? row.paths.filter((path): path is string => typeof path === 'string').join('\n') : '' }; }) : []); setExpandedResourceIds([]); setDraft(nativeDataSourceDraftFrom(source)); setError(''); void loadResources(source); }
  function toggleResource(id: string) { setSelectedResourceIds((current) => toggleDataSourceResourceSelection(resources, current, id)); }
  function resourceState(id: string) { return resourceCheckState(resources, selectedResourceIds, id); }
  function isDriveConnector(type: string) { return type === 'feishu_drive' || type === 'lark_drive'; }
  async function loadDriveRoot() {
    const token = extractDriveFolderToken(driveFolderToken);
    if (!token) { setError(t('dataSource.drive.folderTokenRequired')); return; }
    const generation = ++resourceGeneration.current;
    setDriveFolderToken(token); setSelectedResourceIds([token]); setResourcesLoading(true); setError('');
    try {
      let source = editing;
      if (!source) {
        const input = buildNativeDataSourceInput(draft, knowledgeBaseId);
        source = await runtime.client.dataSources.create({ ...input, status: 'paused', config: { ...(input.config as Record<string, unknown>), resource_ids: [token] } });
        setEditing(source); setTemporarySourceId(source.id);
      } else {
        await runtime.client.dataSources.update(source.id, { config: { resource_ids: [token] } });
      }
      const next = await runtime.client.dataSources.resources(source.id);
      if (generation !== resourceGeneration.current) return;
      setResources(next);
      setExpandedResourceIds([]);
    } catch (cause) {
      if (generation === resourceGeneration.current) setError(cause instanceof Error ? cause.message : t('dataSource.resourceLoadFailed'));
    } finally {
      if (generation === resourceGeneration.current) setResourcesLoading(false);
    }
  }
  async function toggleResourceExpand(resource: DataSourceResource) {
    if (!editing || !resource.has_children) return;
    if (expandedResourceIds.includes(resource.external_id)) { setExpandedResourceIds((current) => current.filter((id) => id !== resource.external_id)); return; }
    const hasLoadedChildren = resources.some((item) => item.parent_id === resource.external_id);
    if (!hasLoadedChildren) {
      const generation = ++resourceChildrenGeneration.current;
      setResourceLoadingId(resource.external_id); setError('');
      try {
        const children = await runtime.client.dataSources.resources(editing.id, resource.external_id);
        if (generation !== resourceChildrenGeneration.current) return;
        setResources((current) => [...current, ...children.filter((child) => !current.some((item) => item.external_id === child.external_id))]);
      } catch (cause) {
        if (generation === resourceChildrenGeneration.current) setError(cause instanceof Error ? cause.message : t('dataSource.resourceLoadFailed'));
        return;
      } finally {
        if (generation === resourceChildrenGeneration.current) setResourceLoadingId(null);
      }
    }
    setExpandedResourceIds((current) => [...current, resource.external_id]);
  }
  const visibleResources = useMemo(() => {
    const result: Array<{ resource: DataSourceResource; depth: number }> = [];
    const walk = (items: DataSourceResource[], depth: number) => items.forEach((item) => { result.push({ resource: item, depth }); if (item.has_children && expandedResourceIds.includes(item.external_id)) walk(resources.filter((child) => child.parent_id === item.external_id), depth + 1); });
    walk(resources.filter((item) => !item.parent_id), 0); return result;
  }, [expandedResourceIds, resources]);
  function expandAllResources() { setExpandedResourceIds(resources.filter((resource) => resource.has_children).map((resource) => resource.external_id)); }
  function collapseAllResources() { setExpandedResourceIds([]); }
  function credentialWarning(fields: ReturnType<typeof validateDataSourceCredentials>): string | null {
    const field = fields[0];
    return field ? `${t(field.labelKey)} ${t('dataSource.isRequired')}` : null;
  }
  async function save() {
    if (!canManage) { setError(t('dataSource.permissionRequired')); return; }
    setSaving(true); setError('');
    try {
      const input = buildNativeDataSourceInput(draft, knowledgeBaseId);
      const config: Record<string, unknown> = { ...(input.config as Record<string, unknown>), resource_ids: selectedResourceIds };
      const credentials = config.credentials as Record<string, unknown>;
      const credentialFields = dataSourceCredentialFields(draft.type);
      const enteredCredentials = Object.fromEntries(Object.entries(credentialValues).filter(([, value]) => value.trim()).map(([key, value]) => [key, value.trim()]));
      if (credentialFields.length) {
        if (!editing || Object.keys(enteredCredentials).length > 0) {
          const warning = credentialWarning(validateDataSourceCredentials(draft.type, enteredCredentials));
          if (warning) throw new Error(warning);
        }
        Object.assign(credentials, enteredCredentials);
      }
      if (draft.type === 'rss') {
        if (!rssFeedUrls.trim()) throw new Error(`${t('dataSource.field.feedUrls')} ${t('dataSource.isRequired')}`);
        config.settings = { ...(config.settings as Record<string, unknown>), feed_urls: rssFeedUrls.trim() };
        if (rssAuthHeaders.trim()) credentials.auth_headers = rssAuthHeaders.trim();
      }
      if (draft.type === 'gitlab') {
        if (!gitlabProjects.some((project) => project.projectId.trim())) throw new Error(t('dataSource.gitlab.projectRequired'));
        config.settings = { ...(config.settings as Record<string, unknown>), projects: gitlabProjects.filter((project) => project.projectId.trim()).map((project) => ({ project_id: project.projectId.trim(), ref: project.ref.trim(), paths: project.paths.split(/[,\n]/).map((path) => path.trim()).filter(Boolean) })) };
      }
      const hasCredentialChanges = draft.credentialsText.trim() || Object.keys(enteredCredentials).length > 0 || (draft.type === 'rss' && rssAuthHeaders.trim());
      const hasValidationInput = hasCredentialChanges || (draft.type === 'rss' && rssFeedUrls.trim() !== initialRssFeedUrls.trim());
      if (editing && !hasValidationInput) {
        const result = await runtime.client.dataSources.validate(editing.id);
        const failure = connectionError(result, t('dataSource.connectionTestFailed'));
        if (failure) throw new Error(failure);
      } else if (hasValidationInput || !editing) {
        const validationCredentials = draft.type === 'rss' ? { ...credentials, feed_urls: rssFeedUrls.trim() } : credentials;
        const result = await runtime.client.dataSources.validateCredentials(draft.type, validationCredentials);
        const failure = connectionError(result, t('dataSource.connectionTestFailed'));
        if (failure) throw new Error(failure);
      }
      if (editing) {
        if (hasCredentialChanges) await runtime.client.dataSources.putCredentials(editing.id, credentials);
        const updateInput = { ...input, knowledge_base_id: undefined, config: { ...config, credentials: undefined } };
        await runtime.client.dataSources.update(editing.id, updateInput);
      } else await runtime.client.dataSources.create({ ...input, config });
        setEditing(undefined); setTemporarySourceId(null); await load();
    } catch (cause) { setError(cause instanceof Error ? cause.message : t('dataSource.saveFailed')); }
    finally { setSaving(false); }
  }
  async function cancelEditor() {
    if (temporarySourceId) {
      try { await runtime.client.dataSources.remove(temporarySourceId); }
      catch (cause) { setError(cause instanceof Error ? cause.message : t('dataSource.discardTemporaryFailed')); return; }
      setTemporarySourceId(null);
    }
    setEditing(undefined);
  }
  async function testConnection() {
    if (!canManage) { setError(t('dataSource.permissionRequired')); return; }
    setTesting(true); setTestResult(null); setError('');
    try {
      const input = buildNativeDataSourceInput(draft, knowledgeBaseId);
      const config = input.config as Record<string, unknown>;
      if (draft.type === 'rss' && !rssFeedUrls.trim()) throw new Error(`${t('dataSource.field.feedUrls')} ${t('dataSource.isRequired')}`);
      if (draft.type === 'rss' && rssAuthHeaders.trim()) (config.credentials as Record<string, unknown>).auth_headers = rssAuthHeaders.trim();
      const fields = dataSourceCredentialFields(draft.type);
      const entered = Object.fromEntries(Object.entries(credentialValues).filter(([, value]) => value.trim()).map(([key, value]) => [key, value.trim()]));
      const hasCredentialInput = Object.keys(entered).length > 0 || draft.credentialsText.trim() !== '' || (draft.type === 'rss' && (rssAuthHeaders.trim() !== '' || rssFeedUrls.trim() !== initialRssFeedUrls.trim()));
      if (!editing && fields.length) {
        const warning = credentialWarning(validateDataSourceCredentials(draft.type, entered));
        if (warning) throw new Error(warning);
      }
      const validationCredentials = { ...(config.credentials as Record<string, unknown>), ...entered, ...(draft.type === 'rss' ? { feed_urls: rssFeedUrls.trim() } : {}) };
      const result = editing && !hasCredentialInput ? await runtime.client.dataSources.validate(editing.id) : await runtime.client.dataSources.validateCredentials(draft.type, validationCredentials);
      const failure = connectionError(result, t('dataSource.connectionTestFailed'));
      if (failure) throw new Error(failure);
      setTestResult('success');
    } catch (cause) {
      setTestResult('error'); setError(cause instanceof Error ? cause.message : t('dataSource.testFailed'));
    } finally { setTesting(false); }
  }
  async function run(source: DataSource, operation: 'sync' | 'pause' | 'resume') {
    if (!canManage) { setError(t('dataSource.permissionRequired')); return; }
    setAction(`${operation}:${source.id}`); setError('');
    try {
      const result = operation === 'sync' ? await runtime.client.dataSources.sync(source.id) : operation === 'pause' ? await runtime.client.dataSources.pause(source.id) : await runtime.client.dataSources.resume(source.id);
      const failure = connectionError(result, t('dataSource.connectionTestFailed')); if (failure) throw new Error(failure);
      await load();
    } catch (cause) { setError(cause instanceof Error ? cause.message : operation === 'sync' ? t('dataSource.syncFailed') : operation === 'pause' ? t('dataSource.pauseFailed') : t('dataSource.resumeFailed')); }
    finally { setAction(null); }
  }
  async function remove(source: DataSource) {
    if (!canManage) { setError(t('dataSource.permissionRequired')); return; }
    setAction(`delete:${source.id}`); setError('');
    try { await runtime.client.dataSources.remove(source.id); await load(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : t('dataSource.deleteFailed')); }
    finally { setAction(null); }
  }
  async function showLogs(source: DataSource) {
    const requestId = ++logsRequestId.current;
    setLogsSource(source); setLogs([]); setLogsHasMore(false); setExpandedLogId(null); setLogsLoading(true); setLogsLoadingMore(false); setError('');
    try { const next = await runtime.client.dataSources.logs(source.id, 50, 0); if (requestId !== logsRequestId.current) return; setLogs(next); setLogsHasMore(next.length === 50); }
    catch (cause) { if (requestId === logsRequestId.current) setError(cause instanceof Error ? cause.message : t('dataSource.syncLogsLoadFailed')); }
    finally { if (requestId === logsRequestId.current) setLogsLoading(false); }
  }
  async function loadMoreLogs() {
    if (!logsSource || logsLoading || logsLoadingMore || !logsHasMore) return;
    const requestId = logsRequestId.current;
    setLogsLoadingMore(true); setError('');
    try { const next = await runtime.client.dataSources.logs(logsSource.id, 50, logs.length); if (requestId !== logsRequestId.current) return; setLogs((current) => [...current, ...next]); setLogsHasMore(next.length === 50); }
    catch (cause) { if (requestId === logsRequestId.current) setError(cause instanceof Error ? cause.message : t('dataSource.syncLogsLoadMoreFailed')); }
    finally { if (requestId === logsRequestId.current) setLogsLoadingMore(false); }
  }
  const connectorLabel = (type: string, fallback: string) => { const key = dataSourceConnectorLabelKey(type); return localizedOr(t(key), key, fallback); };
  const statusLabel = (status: string) => { const key = dataSourceStatusLabelKey(status); return localizedOr(t(key), key, dataSourceStatusLabel({ status })); };
  const syncModeLabel = (mode: string | undefined) => mode ? localizedOr(t(dataSourceSyncModeLabelKey(mode)), dataSourceSyncModeLabelKey(mode), mode) : '';
  return <SafeAreaView style={{ flex: 1, padding: 16 }}>
    <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 10 }}><Pressable onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>{t('knowledgeBase.detail.back')}</Text></Pressable><Text accessibilityRole="header" style={{ flex: 1, fontSize: 22, fontWeight: '700' }}>{t('dataSource.title')}</Text>{canManage ? <Pressable onPress={openCreate}><Text style={{ color: '#2864dc' }}>{t('dataSource.add')}</Text></Pressable> : null}<Pressable onPress={() => void load()}><Text style={{ color: '#2864dc' }}>{t('knowledgeBase.documents.reload')}</Text></Pressable></View>
    <Text style={{ color: '#667085', marginBottom: 10 }}>{t('dataSource.description')}</Text>
    {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 8 }}>{error}</Text> : null}
    {loading ? <ActivityIndicator accessibilityLabel={t('common.loading')} /> : <FlatList data={sources} keyExtractor={(item) => item.id} ListEmptyComponent={<Text style={{ color: '#667085' }}>{t('dataSource.empty')}</Text>} renderItem={({ item }) => { const connector = typeById.get(item.type); const busy = action !== null; const latestLog = item.latest_sync_log; return <View style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 12 }}><Text style={{ fontWeight: '600' }}>{item.name}</Text><Text style={{ color: '#667085', fontSize: 12, marginTop: 3 }}>{connectorLabel(item.type, safeDataSourceType(item))}{item.sync_mode ? ` · ${syncModeLabel(item.sync_mode)}` : ''} · {statusLabel(typeof item.status === 'string' ? item.status : '')}</Text>{latestLog ? <Text style={{ color: '#667085', fontSize: 12, marginTop: 3 }}>{localizedOr(t('dataSource.status.' + latestLog.status), 'dataSource.status.' + latestLog.status, latestLog.status)}{typeof latestLog.items_created === 'number' && latestLog.items_created > 0 ? ` · +${latestLog.items_created}` : ''}{typeof latestLog.items_updated === 'number' && latestLog.items_updated > 0 ? ` · ~${latestLog.items_updated}` : ''}{typeof latestLog.items_deleted === 'number' && latestLog.items_deleted > 0 ? ` · -${latestLog.items_deleted}` : ''}{typeof latestLog.items_failed === 'number' && latestLog.items_failed > 0 ? ` · ${latestLog.items_failed} ${t('dataSource.logMetric.failed')}` : ''}</Text> : <Text style={{ color: '#667085', fontSize: 12, marginTop: 3 }}>{t('dataSource.neverSynced')}</Text>}{item.sync_schedule ? <Text style={{ color: '#667085', fontSize: 12, marginTop: 3 }}>{t('dataSource.syncScheduleLabel')}: {item.sync_schedule}</Text> : null}{connector ? <Text style={{ color: '#667085', fontSize: 12, marginTop: 3 }}>{connector.auth_type} connector · {connector.capabilities.join(', ') || t('dataSource.noDeclaredCapabilities')}</Text> : null}<View style={{ flexDirection: 'row', gap: 12, flexWrap: 'wrap', marginTop: 8 }}>{canManage ? <><Pressable disabled={busy} onPress={() => openEdit(item)}><Text style={{ color: busy ? '#98a2b3' : '#2864dc' }}>{t('dataSource.edit')}</Text></Pressable><Pressable disabled={busy} onPress={() => void run(item, 'sync')}><Text style={{ color: busy ? '#98a2b3' : '#2864dc' }}>{t('dataSource.syncNow')}</Text></Pressable>{item.status === 'paused' ? <Pressable disabled={busy} onPress={() => void run(item, 'resume')}><Text style={{ color: busy ? '#98a2b3' : '#2864dc' }}>{t('dataSource.resume')}</Text></Pressable> : <Pressable disabled={busy} onPress={() => void run(item, 'pause')}><Text style={{ color: busy ? '#98a2b3' : '#2864dc' }}>{t('dataSource.pause')}</Text></Pressable>}<Pressable disabled={busy} onPress={() => Alert.alert(t('dataSource.deleteConfirm'), item.name, [{ text: t('common.cancel'), style: 'cancel' }, { text: t('dataSource.delete'), style: 'destructive', onPress: () => void remove(item) }])}><Text style={{ color: busy ? '#98a2b3' : '#b42318' }}>{t('dataSource.delete')}</Text></Pressable></> : null}<Pressable disabled={busy} onPress={() => void showLogs(item)}><Text style={{ color: busy ? '#98a2b3' : '#2864dc' }}>{t('dataSource.logs')}</Text></Pressable></View></View>; }} />}
    {logsSource ? <ScrollView style={{ maxHeight: 260, marginTop: 12, borderColor: '#eaecf0', borderWidth: 1, borderRadius: 8, padding: 10 }}><View style={{ flexDirection: 'row', justifyContent: 'space-between' }}><Text style={{ fontWeight: '700' }}>{t('dataSource.syncHistory')} · {logsSource.name}</Text><Pressable onPress={() => { logsRequestId.current += 1; setLogsLoading(false); setLogsLoadingMore(false); setLogsSource(null); }}><Text style={{ color: '#2864dc' }}>{t('dataSource.close')}</Text></Pressable></View><Pressable disabled={logsLoading} onPress={() => void (logsSource && showLogs(logsSource))}><Text style={{ color: logsLoading ? '#98a2b3' : '#2864dc' }}>{t('dataSource.refreshLogs')}</Text></Pressable>{logsLoading ? <ActivityIndicator accessibilityLabel={t('common.loading')} /> : logs.length ? <><Text style={{ color: '#667085', fontSize: 12, marginVertical: 8 }}>{t('dataSource.summary.total')} {logs.length} · {t('dataSource.summary.success')} {logs.filter((log) => log.status === 'success').length} · {t('dataSource.summary.failed')} {logs.filter((log) => log.status === 'failed').length} · {t('dataSource.summary.items')} {logs.reduce((sum, log) => sum + (typeof log.items_created === 'number' ? log.items_created : 0) + (typeof log.items_updated === 'number' ? log.items_updated : 0), 0)}</Text>{logs.map((log) => <View key={log.id} style={{ paddingVertical: 6, borderBottomColor: '#eaecf0', borderBottomWidth: 1 }}><Pressable onPress={() => setExpandedLogId((current) => current === log.id ? null : log.id)}><Text>{expandedLogId === log.id ? '⌄' : '›'} {t('dataSource.status.' + log.status)}</Text></Pressable><Text style={{ color: '#667085', fontSize: 12 }}>{typeof log.started_at === 'string' ? log.started_at : ''}{typeof log.finished_at === 'string' ? ' · ' + log.finished_at : ''}{typeof log.error_message === 'string' && log.error_message ? ' · ' + log.error_message : ''}</Text>{expandedLogId === log.id ? <Text style={{ color: '#667085', fontSize: 12, marginTop: 4 }}>{t('dataSource.detail.created')} {log.items_created ?? 0} · {t('dataSource.detail.updated')} {log.items_updated ?? 0} · {t('dataSource.detail.deleted')} {log.items_deleted ?? 0} · {t('dataSource.detail.skipped')} {log.items_skipped ?? 0} · {t('dataSource.detail.failed')} {log.items_failed ?? 0}</Text> : null}</View>)}{logsHasMore ? <Pressable disabled={logsLoadingMore} onPress={() => void loadMoreLogs()}><Text style={{ color: logsLoadingMore ? '#98a2b3' : '#2864dc', marginTop: 8 }}>{logsLoadingMore ? t('dataSource.loadingMore') : t('dataSource.loadMore')}</Text></Pressable> : null}</> : <Text style={{ color: '#667085', marginTop: 8 }}>{t('dataSource.noLogs')}</Text>}</ScrollView> : null}
    {editing !== undefined ? <ScrollView style={{ marginTop: 12 }} keyboardShouldPersistTaps="handled"><Text style={{ fontSize: 18, fontWeight: '700', marginBottom: 8 }}>{editing ? t('dataSource.editTitle') : t('dataSource.createTitle')}</Text><Text style={{ fontWeight: '600', marginBottom: 4 }}>{t('dataSource.sectionBasic')}</Text><TextInput accessibilityLabel={t('dataSource.nameLabel')} value={draft.name} onChangeText={(value) => updateDraft('name', value)} placeholder={t('dataSource.namePlaceholder')} style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} />{types.length ? <View accessibilityLabel={t('dataSource.step.selectType')} style={{ gap: 6, marginBottom: 8 }}>{types.map((type) => { const nameKey = `dataSource.connector.${type.type}`; const descKey = `dataSource.connectorDesc.${type.type}`; return <Pressable key={type.type} onPress={() => updateDraft('type', type.type)} style={{ backgroundColor: draft.type === type.type ? '#dbeafe' : '#f2f4f7', borderRadius: 8, padding: 10 }}><Text style={{ fontWeight: '600' }}>{localizedOr(t(nameKey), nameKey, type.name)}</Text><Text style={{ color: '#667085', fontSize: 12 }}>{localizedOr(t(descKey), descKey, type.description)}</Text></Pressable>; })}</View> : <TextInput accessibilityLabel={t('dataSource.step.selectType')} value={draft.type} onChangeText={(value) => updateDraft('type', value)} placeholder={t('dataSource.connectorTypePlaceholder')} autoCapitalize="none" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} />}{isDriveConnector(draft.type) ? <View style={{ marginBottom: 8 }}><Text style={{ fontWeight: '600', marginBottom: 4 }}>{t('dataSource.drive.folderTokenLabel')}</Text><TextInput accessibilityLabel={t('dataSource.drive.folderTokenLabel')} value={driveFolderToken} onChangeText={(value) => { setDriveFolderToken(value); setError(''); }} placeholder={t('dataSource.drive.folderTokenPlaceholder')} autoCapitalize="none" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 6 }} /><Pressable disabled={resourcesLoading} onPress={() => void loadDriveRoot()}><Text style={{ color: resourcesLoading ? '#98a2b3' : '#2864dc' }}>{resourcesLoading ? t('common.loading') : t('dataSource.drive.load')}</Text></Pressable></View> : null}{draft.type === 'rss' ? <View style={{ marginBottom: 8 }}><Text style={{ fontWeight: '600', marginBottom: 4 }}>{t('dataSource.field.feedUrls')}</Text><TextInput accessibilityLabel={t('dataSource.field.feedUrls')} value={rssFeedUrls} onChangeText={(value) => { setRssFeedUrls(value); setTestResult(null); }} placeholder="https://example.com/feed.xml" multiline style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, minHeight: 70, marginBottom: 4 }} /><Text style={{ color: '#667085', fontSize: 12, marginBottom: 8 }}>{t('dataSource.field.feedUrlsHint')}</Text><Text style={{ fontWeight: '600', marginBottom: 4 }}>{t('dataSource.field.authHeaders')}</Text><TextInput accessibilityLabel={t('dataSource.field.authHeaders')} value={rssAuthHeaders} onChangeText={(value) => { setRssAuthHeaders(value); setTestResult(null); }} placeholder="Authorization: Bearer …" multiline style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, minHeight: 55, marginBottom: 4 }} /><Text style={{ color: '#667085', fontSize: 12, marginBottom: 8 }}>{t('dataSource.field.authHeadersHint')}</Text></View> : null}{dataSourceCredentialFields(draft.type).length ? <View accessibilityLabel={t('dataSource.credentialsLabel')}>{dataSourceCredentialFields(draft.type).map((field) => <View key={field.key}><TextInput accessibilityLabel={t(field.labelKey)} value={credentialValues[field.key] ?? ''} onChangeText={(value) => { setCredentialValues((current) => ({ ...current, [field.key]: value })); setTestResult(null); }} placeholder={field.placeholder || t(field.labelKey)} secureTextEntry={field.secret} autoCapitalize="none" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 4 }} />{field.hintKey ? <Text style={{ color: '#667085', fontSize: 12, marginBottom: 8 }}>{t(field.hintKey)}</Text> : null}</View>)}</View> : draft.type !== 'rss' ? <TextInput accessibilityLabel={t('dataSource.credentialsLabel')} value={draft.credentialsText} onChangeText={(value) => updateDraft('credentialsText', value)} placeholder={t('dataSource.credentialsPlaceholder')} secureTextEntry multiline style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, minHeight: 70, marginBottom: 8 }} /> : null}<TextInput accessibilityLabel={t('dataSource.connectorSettingsLabel')} value={draft.settingsText} onChangeText={(value) => updateDraft('settingsText', value)} placeholder={t('dataSource.settingsPlaceholder')} multiline style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, minHeight: 70, marginBottom: 8 }} />{draft.type === 'gitlab' ? <View accessibilityLabel={t('dataSource.gitlab.projects')} style={{ marginBottom: 8 }}><Text style={{ fontWeight: '600', marginBottom: 4 }}>{t('dataSource.gitlab.projects')}</Text><Text style={{ color: '#667085', fontSize: 12, marginBottom: 6 }}>{t('dataSource.gitlab.projectsHint')}</Text>{gitlabProjects.map((project, index) => <View key={index} style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 8, marginBottom: 6 }}><TextInput accessibilityLabel={`${t('dataSource.gitlab.project')} ${index + 1}`} value={project.projectId} onChangeText={(value) => setGitlabProjects((current) => current.map((item, itemIndex) => itemIndex === index ? { ...item, projectId: value } : item))} placeholder={t('dataSource.gitlab.projectIdPlaceholder')} style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 6, padding: 8, marginBottom: 5 }} /><TextInput accessibilityLabel={t('dataSource.gitlab.ref')} value={project.ref} onChangeText={(value) => setGitlabProjects((current) => current.map((item, itemIndex) => itemIndex === index ? { ...item, ref: value } : item))} placeholder={t('dataSource.gitlab.refPlaceholder')} style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 6, padding: 8, marginBottom: 5 }} /><TextInput accessibilityLabel={t('dataSource.gitlab.paths')} value={project.paths} onChangeText={(value) => setGitlabProjects((current) => current.map((item, itemIndex) => itemIndex === index ? { ...item, paths: value } : item))} placeholder={t('dataSource.gitlab.pathsPlaceholder')} multiline style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 6, padding: 8, marginBottom: 5 }} />{gitlabProjects.length > 1 ? <Pressable onPress={() => setGitlabProjects((current) => current.filter((_, itemIndex) => itemIndex !== index))}><Text style={{ color: '#b42318' }}>{t('common.delete')}</Text></Pressable> : null}</View>)}<Pressable onPress={() => setGitlabProjects((current) => [...current, { projectId: '', ref: '', paths: '' }])}><Text style={{ color: '#2864dc' }}>{t('dataSource.gitlab.addProject')}</Text></Pressable></View> : null}<Text style={{ fontWeight: '600', marginBottom: 4 }}>{t('dataSource.step.resources')}</Text><Text style={{ color: '#667085', fontSize: 12, marginBottom: 6 }}>{t('dataSource.resourceHint')} · {t('knowledgeBase.selectedCount', { count: selectedResourceIds.length })}</Text>{isDriveConnector(draft.type) && !editing && !resourcesLoading && resources.length === 0 ? <Text style={{ color: '#667085', marginBottom: 8 }}>{t('dataSource.drive.placeholderDesc')}</Text> : null}{resourcesLoading ? <ActivityIndicator accessibilityLabel={t('common.loading')} /> : resources.length ? <View accessibilityLabel={t('dataSource.step.resources')} style={{ marginBottom: 8 }}>{visibleResources.map(({ resource, depth }) => <View key={resource.external_id} style={{ marginLeft: depth * 16, flexDirection: 'row', alignItems: 'center', marginBottom: 4 }}><Pressable accessibilityRole="button" accessibilityLabel={`${resource.name || t('dataSource.untitled')} ${resourceActionLabel(locale, expandedResourceIds.includes(resource.external_id))}`} disabled={!resource.has_children || resourceLoadingId === resource.external_id} onPress={() => void toggleResourceExpand(resource)} style={{ width: 28, paddingVertical: 7 }}><Text>{resource.has_children ? (resourceLoadingId === resource.external_id ? '…' : expandedResourceIds.includes(resource.external_id) ? '⌄' : '›') : ''}</Text></Pressable><Pressable accessibilityRole="checkbox" accessibilityState={{ checked: resourceState(resource.external_id) !== 'unchecked' }} accessibilityLabel={`${resource.name || t('dataSource.untitled')} ${t('dataSource.step.resources')}`} onPress={() => toggleResource(resource.external_id)} style={{ paddingVertical: 7, paddingHorizontal: 8, backgroundColor: resourceState(resource.external_id) !== 'unchecked' ? '#dbeafe' : '#f2f4f7', borderRadius: 6, flex: 1 }}><Text>{resourceSelectionMarker(resourceState(resource.external_id))}{resource.name || t('dataSource.untitled')} · {resource.type}</Text></Pressable></View>)}</View> : editing ? <View style={{ marginBottom: 8 }}><Text style={{ color: '#667085' }}>{t('dataSource.noResources')}</Text><Pressable onPress={() => void loadResources(editing)}><Text style={{ color: '#2864dc', marginTop: 6 }}>{t('dataSource.retryLoadResources')}</Text></Pressable></View> : null}<TextInput accessibilityLabel={t('dataSource.syncScheduleLabel')} value={draft.schedule} onChangeText={(value) => updateDraft('schedule', value)} placeholder={t('dataSource.cronSchedulePlaceholder')} style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /><Text style={{ marginBottom: 4 }}>{t('dataSource.syncModeLabel')}: {syncModeLabel(draft.mode)}</Text><View style={{ flexDirection: 'row', gap: 8, marginBottom: 8 }}><Pressable onPress={() => updateDraft('mode', 'incremental')} style={{ backgroundColor: draft.mode === 'incremental' ? '#dbeafe' : '#f2f4f7', padding: 8, borderRadius: 8 }}><Text>{t('dataSource.syncMode.incremental')}</Text></Pressable><Pressable onPress={() => updateDraft('mode', 'full')} style={{ backgroundColor: draft.mode === 'full' ? '#dbeafe' : '#f2f4f7', padding: 8, borderRadius: 8 }}><Text>{t('dataSource.syncMode.full')}</Text></Pressable></View><Text style={{ marginBottom: 4 }}>{t('dataSource.conflictLabel')}: {localizedOr(t('dataSource.conflict.' + draft.conflict), 'dataSource.conflict.' + draft.conflict, draft.conflict)}</Text><View style={{ flexDirection: 'row', gap: 8, marginBottom: 8 }}><Pressable onPress={() => updateDraft('conflict', 'overwrite')} style={{ backgroundColor: draft.conflict === 'overwrite' ? '#dbeafe' : '#f2f4f7', padding: 8, borderRadius: 8 }}><Text>{t('dataSource.conflict.overwrite')}</Text></Pressable><Pressable onPress={() => updateDraft('conflict', 'skip')} style={{ backgroundColor: draft.conflict === 'skip' ? '#dbeafe' : '#f2f4f7', padding: 8, borderRadius: 8 }}><Text>{t('dataSource.conflict.skip')}</Text></Pressable></View><View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginBottom: 10 }}><Text>{t('dataSource.syncDeletions')}</Text><Switch value={draft.deletions} onValueChange={(value) => updateDraft('deletions', value)} /></View>{testResult === 'success' ? <Text style={{ color: '#067647', marginBottom: 8 }}>{t('dataSource.testSuccess')}</Text> : null}<View style={{ flexDirection: 'row', gap: 12 }}><Pressable disabled={testing || saving} onPress={() => void testConnection()} style={{ padding: 11, borderRadius: 8 }}><Text style={{ color: testing ? '#98a2b3' : '#2864dc' }}>{testing ? t('dataSource.testing') : t('dataSource.testConnection')}</Text></Pressable><Pressable disabled={saving || testing} onPress={() => void save()} style={{ backgroundColor: saving ? '#98a2b3' : '#2864dc', padding: 11, borderRadius: 8, flex: 1, alignItems: 'center' }}><Text style={{ color: '#fff', fontWeight: '600' }}>{saving ? t('common.loading') : t('dataSource.save')}</Text></Pressable><Pressable disabled={saving || testing} onPress={() => void cancelEditor()} style={{ padding: 11 }}><Text style={{ color: '#2864dc' }}>{t('common.cancel')}</Text></Pressable></View></ScrollView> : null}
    </SafeAreaView>;
}
