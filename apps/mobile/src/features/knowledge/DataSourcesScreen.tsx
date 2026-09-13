import { useCallback, useEffect, useMemo, useState } from 'react';
import { ActivityIndicator, Alert, FlatList, Pressable, SafeAreaView, ScrollView, Switch, Text, TextInput, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import type { DataSource, DataSourceConnectorType, DataSourceResource } from '@weknora/api-client';
import { useMobileRuntime } from '../../runtime.tsx';
import { canManageDataSources, dataSourceStatusLabel, safeDataSourceType } from './data-sources.ts';
import { buildNativeDataSourceInput, nativeDataSourceDraftFrom, type NativeDataSourceDraft } from './data-source-form.ts';

const EMPTY_DRAFT: NativeDataSourceDraft = { name: '', type: '', schedule: '0 0 */6 * * *', mode: 'incremental', conflict: 'overwrite', deletions: true, credentialsText: '', settingsText: '' };

function connectionError(value: unknown): string | null {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return null;
  const row = value as Record<string, unknown>;
  if (row.success === false) return typeof row.message === 'string' ? row.message : typeof row.error === 'string' ? row.error : 'Connection test failed';
  return null;
}

export function DataSourcesScreen() {
  const { id: rawId } = useLocalSearchParams<{ id: string }>();
  const knowledgeBaseId = Array.isArray(rawId) ? rawId[0] : rawId;
  const runtime = useMobileRuntime();
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
  const [action, setAction] = useState<string | null>(null);
  const [logsSource, setLogsSource] = useState<DataSource | null>(null);
  const [logs, setLogs] = useState<import('@weknora/api-client').DataSourceSyncLog[]>([]);

  const load = useCallback(async () => {
    if (!knowledgeBaseId) { setLoading(false); setError('Knowledge base is required'); return; }
    setLoading(true); setError('');
    const [sourceResult, typeResult] = await Promise.allSettled([
      runtime.client.dataSources.list(knowledgeBaseId),
      runtime.client.dataSources.types(),
    ]);
    if (sourceResult.status === 'fulfilled') setSources(sourceResult.value); else setError(sourceResult.reason instanceof Error ? sourceResult.reason.message : 'Unable to load data sources');
    if (typeResult.status === 'fulfilled') setTypes(typeResult.value);
    setLoading(false);
  }, [knowledgeBaseId, runtime.client]);

  useEffect(() => { void load(); }, [load]);
  const typeById = new Map(types.map((type) => [type.type, type]));
  function updateDraft<K extends keyof NativeDataSourceDraft>(key: K, value: NativeDataSourceDraft[K]) { setDraft((current) => ({ ...current, [key]: value })); setTestResult(null); }
  function openCreate() { setEditing(null); setResources([]); setSelectedResourceIds([]); setExpandedResourceIds([]); setDraft({ ...EMPTY_DRAFT, type: types[0]?.type ?? '' }); setError(''); }
  function sourceResourceIds(source: DataSource): string[] {
    const config = source.config && typeof source.config === 'object' && !Array.isArray(source.config) ? source.config as Record<string, unknown> : {};
    const value = Array.isArray(config.resource_ids) ? config.resource_ids : source.resource_ids;
    return Array.isArray(value) ? value.filter((id): id is string => typeof id === 'string') : [];
  }
  async function loadResources(source: DataSource) {
    setResourcesLoading(true); setError(''); setExpandedResourceIds([]);
    try { setResources(await runtime.client.dataSources.resources(source.id)); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to load data source resources'); }
    finally { setResourcesLoading(false); }
  }
  function openEdit(source: DataSource) { setEditing(source); setResources([]); setSelectedResourceIds(sourceResourceIds(source)); setExpandedResourceIds([]); setDraft(nativeDataSourceDraftFrom(source)); setError(''); void loadResources(source); }
  function toggleResource(id: string) { setSelectedResourceIds((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]); }
  async function toggleResourceExpand(resource: DataSourceResource) {
    if (!editing || !resource.has_children) return;
    if (expandedResourceIds.includes(resource.external_id)) { setExpandedResourceIds((current) => current.filter((id) => id !== resource.external_id)); return; }
    const hasLoadedChildren = resources.some((item) => item.parent_id === resource.external_id);
    if (!hasLoadedChildren) {
      setResourceLoadingId(resource.external_id); setError('');
      try {
        const children = await runtime.client.dataSources.resources(editing.id, resource.external_id);
        setResources((current) => [...current, ...children.filter((child) => !current.some((item) => item.external_id === child.external_id))]);
      } catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to load data source resources'); return; }
      finally { setResourceLoadingId(null); }
    }
    setExpandedResourceIds((current) => [...current, resource.external_id]);
  }
  const visibleResources = useMemo(() => {
    const result: Array<{ resource: DataSourceResource; depth: number }> = [];
    const walk = (items: DataSourceResource[], depth: number) => items.forEach((item) => { result.push({ resource: item, depth }); if (item.has_children && expandedResourceIds.includes(item.external_id)) walk(resources.filter((child) => child.parent_id === item.external_id), depth + 1); });
    walk(resources.filter((item) => !item.parent_id), 0); return result;
  }, [expandedResourceIds, resources]);
  async function save() {
    if (!canManage) { setError('Data-source changes require an owner or admin workspace role'); return; }
    setSaving(true); setError('');
    try {
      const input = buildNativeDataSourceInput(draft, knowledgeBaseId);
      const config: Record<string, unknown> = { ...(input.config as Record<string, unknown>), resource_ids: selectedResourceIds };
      const credentials = config.credentials as Record<string, unknown>;
      if (draft.credentialsText.trim()) {
        const result = await runtime.client.dataSources.validateCredentials(draft.type, credentials);
        const failure = connectionError(result);
        if (failure) throw new Error(failure);
      }
      if (editing) {
        const updateInput = { ...input, knowledge_base_id: undefined, config: { ...config, credentials: draft.credentialsText.trim() ? credentials : undefined } };
        const saved = await runtime.client.dataSources.update(editing.id, updateInput);
        if (draft.credentialsText.trim()) await runtime.client.dataSources.putCredentials(saved.id, credentials);
      } else await runtime.client.dataSources.create(input);
      setEditing(undefined); await load();
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to save data source'); }
    finally { setSaving(false); }
  }
  async function testConnection() {
    if (!canManage) { setError('Data-source changes require an owner or admin workspace role'); return; }
    setTesting(true); setTestResult(null); setError('');
    try {
      const input = buildNativeDataSourceInput(draft, knowledgeBaseId);
      const config = input.config as Record<string, unknown>;
      const result = await runtime.client.dataSources.validateCredentials(draft.type, config.credentials as Record<string, unknown>);
      const failure = connectionError(result);
      if (failure) throw new Error(failure);
      setTestResult('success');
    } catch (cause) {
      setTestResult('error'); setError(cause instanceof Error ? cause.message : 'Connection test failed');
    } finally { setTesting(false); }
  }
  async function run(source: DataSource, operation: 'sync' | 'pause' | 'resume') {
    if (!canManage) { setError('Data-source changes require an owner or admin workspace role'); return; }
    setAction(`${operation}:${source.id}`); setError('');
    try {
      const result = operation === 'sync' ? await runtime.client.dataSources.sync(source.id) : operation === 'pause' ? await runtime.client.dataSources.pause(source.id) : await runtime.client.dataSources.resume(source.id);
      const failure = connectionError(result); if (failure) throw new Error(failure);
      await load();
    } catch (cause) { setError(cause instanceof Error ? cause.message : `Unable to ${operation} data source`); }
    finally { setAction(null); }
  }
  async function remove(source: DataSource) {
    if (!canManage) { setError('Data-source changes require an owner or admin workspace role'); return; }
    setAction(`delete:${source.id}`); setError('');
    try { await runtime.client.dataSources.remove(source.id); await load(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to delete data source'); }
    finally { setAction(null); }
  }
  async function showLogs(source: DataSource) {
    setLogsSource(source); setLogs([]); setError('');
    try { setLogs(await runtime.client.dataSources.logs(source.id)); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to load sync logs'); }
  }
  return <SafeAreaView style={{ flex: 1, padding: 16 }}>
    <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 10 }}><Pressable onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>Back</Text></Pressable><Text accessibilityRole="header" style={{ flex: 1, fontSize: 22, fontWeight: '700' }}>Data sources</Text>{canManage ? <Pressable onPress={openCreate}><Text style={{ color: '#2864dc' }}>Add</Text></Pressable> : null}<Pressable onPress={() => void load()}><Text style={{ color: '#2864dc' }}>Refresh</Text></Pressable></View>
    <Text style={{ color: '#667085', marginBottom: 10 }}>Configure connectors and synchronization. Existing credentials are never shown; enter new credentials only when needed.</Text>
    {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 8 }}>{error}</Text> : null}
    {loading ? <ActivityIndicator accessibilityLabel="Loading data sources" /> : <FlatList data={sources} keyExtractor={(item) => item.id} ListEmptyComponent={<Text style={{ color: '#667085' }}>No data sources configured for this knowledge base.</Text>} renderItem={({ item }) => { const connector = typeById.get(item.type); const busy = action !== null; return <View style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 12 }}><Text style={{ fontWeight: '600' }}>{item.name}</Text><Text style={{ color: '#667085', fontSize: 12, marginTop: 3 }}>{safeDataSourceType(item)} · {dataSourceStatusLabel(item)}{item.sync_mode ? ` · ${item.sync_mode}` : ''}</Text>{item.sync_schedule ? <Text style={{ color: '#667085', fontSize: 12, marginTop: 3 }}>Schedule: {item.sync_schedule}</Text> : null}{connector ? <Text style={{ color: '#667085', fontSize: 12, marginTop: 3 }}>{connector.auth_type} connector · {connector.capabilities.join(', ') || 'no declared capabilities'}</Text> : null}<View style={{ flexDirection: 'row', gap: 12, flexWrap: 'wrap', marginTop: 8 }}>{canManage ? <><Pressable disabled={busy} onPress={() => openEdit(item)}><Text style={{ color: busy ? '#98a2b3' : '#2864dc' }}>Edit</Text></Pressable><Pressable disabled={busy} onPress={() => void run(item, 'sync')}><Text style={{ color: busy ? '#98a2b3' : '#2864dc' }}>Sync</Text></Pressable>{item.status === 'paused' ? <Pressable disabled={busy} onPress={() => void run(item, 'resume')}><Text style={{ color: busy ? '#98a2b3' : '#2864dc' }}>Resume</Text></Pressable> : <Pressable disabled={busy} onPress={() => void run(item, 'pause')}><Text style={{ color: busy ? '#98a2b3' : '#2864dc' }}>Pause</Text></Pressable>}<Pressable disabled={busy} onPress={() => Alert.alert('Delete data source?', item.name, [{ text: 'Cancel', style: 'cancel' }, { text: 'Delete', style: 'destructive', onPress: () => void remove(item) }])}><Text style={{ color: busy ? '#98a2b3' : '#b42318' }}>Delete</Text></Pressable></> : null}<Pressable disabled={busy} onPress={() => void showLogs(item)}><Text style={{ color: busy ? '#98a2b3' : '#2864dc' }}>Logs</Text></Pressable></View></View>; }} />}
    {logsSource ? <ScrollView style={{ maxHeight: 180, marginTop: 12, borderColor: '#eaecf0', borderWidth: 1, borderRadius: 8, padding: 10 }}><View style={{ flexDirection: 'row', justifyContent: 'space-between' }}><Text style={{ fontWeight: '700' }}>Sync logs · {logsSource.name}</Text><Pressable onPress={() => setLogsSource(null)}><Text style={{ color: '#2864dc' }}>Close</Text></Pressable></View>{logs.length ? logs.map((log) => <View key={log.id} style={{ paddingVertical: 6, borderBottomColor: '#eaecf0', borderBottomWidth: 1 }}><Text>{log.status}</Text><Text style={{ color: '#667085', fontSize: 12 }}>{typeof log.started_at === 'string' ? log.started_at : ''}{typeof log.error_message === 'string' ? ` · ${log.error_message}` : ''}</Text></View>) : <Text style={{ color: '#667085', marginTop: 8 }}>No sync logs returned.</Text>}</ScrollView> : null}
    {editing !== undefined ? <ScrollView style={{ marginTop: 12 }} keyboardShouldPersistTaps="handled"><Text style={{ fontSize: 18, fontWeight: '700', marginBottom: 8 }}>{editing ? 'Edit data source' : 'Add data source'}</Text><TextInput accessibilityLabel="Data source name" value={draft.name} onChangeText={(value) => updateDraft('name', value)} placeholder="Name" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} />{types.length ? <View accessibilityLabel="Data source type" style={{ gap: 6, marginBottom: 8 }}>{types.map((type) => <Pressable key={type.type} onPress={() => updateDraft('type', type.type)} style={{ backgroundColor: draft.type === type.type ? '#dbeafe' : '#f2f4f7', borderRadius: 8, padding: 10 }}><Text style={{ fontWeight: '600' }}>{type.name}</Text><Text style={{ color: '#667085', fontSize: 12 }}>{type.description}</Text></Pressable>)}</View> : <TextInput accessibilityLabel="Data source type" value={draft.type} onChangeText={(value) => updateDraft('type', value)} placeholder="Connector type" autoCapitalize="none" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} />}<TextInput accessibilityLabel="Data source credentials" value={draft.credentialsText} onChangeText={(value) => updateDraft('credentialsText', value)} placeholder="Credentials: token = secret" secureTextEntry multiline style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, minHeight: 70, marginBottom: 8 }} /><TextInput accessibilityLabel="Data source settings" value={draft.settingsText} onChangeText={(value) => updateDraft('settingsText', value)} placeholder="Settings: workspace_id = example" multiline style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, minHeight: 70, marginBottom: 8 }} /><Text style={{ fontWeight: "600", marginBottom: 4 }}>Resources</Text>{resourcesLoading ? <ActivityIndicator accessibilityLabel="Loading resources" /> : resources.length ? <View accessibilityLabel="Data source resources" style={{ marginBottom: 8 }}>{visibleResources.map(({ resource, depth }) => <View key={resource.external_id} style={{ marginLeft: depth * 16, flexDirection: "row", alignItems: "center", marginBottom: 4 }}><Pressable disabled={!resource.has_children || resourceLoadingId === resource.external_id} onPress={() => void toggleResourceExpand(resource)} style={{ width: 28, paddingVertical: 7 }}><Text>{resource.has_children ? (resourceLoadingId === resource.external_id ? "…" : expandedResourceIds.includes(resource.external_id) ? "⌄" : "›") : ""}</Text></Pressable><Pressable onPress={() => toggleResource(resource.external_id)} style={{ paddingVertical: 7, paddingHorizontal: 8, backgroundColor: selectedResourceIds.includes(resource.external_id) ? "#dbeafe" : "#f2f4f7", borderRadius: 6, flex: 1 }}><Text>{selectedResourceIds.includes(resource.external_id) ? "✓ " : ""}{resource.name} · {resource.type}</Text></Pressable></View>)}</View> : editing ? <Text style={{ color: "#667085", marginBottom: 8 }}>No resources returned.</Text> : null}<TextInput accessibilityLabel="Sync schedule" value={draft.schedule} onChangeText={(value) => updateDraft('schedule', value)} placeholder="Cron schedule" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /><Text style={{ marginBottom: 4 }}>Sync mode: {draft.mode}</Text><View style={{ flexDirection: 'row', gap: 8, marginBottom: 8 }}><Pressable onPress={() => updateDraft('mode', 'incremental')} style={{ backgroundColor: draft.mode === 'incremental' ? '#dbeafe' : '#f2f4f7', padding: 8, borderRadius: 8 }}><Text>Incremental</Text></Pressable><Pressable onPress={() => updateDraft('mode', 'full')} style={{ backgroundColor: draft.mode === 'full' ? '#dbeafe' : '#f2f4f7', padding: 8, borderRadius: 8 }}><Text>Full</Text></Pressable></View><Text style={{ marginBottom: 4 }}>Conflict strategy: {draft.conflict}</Text><View style={{ flexDirection: 'row', gap: 8, marginBottom: 8 }}><Pressable onPress={() => updateDraft('conflict', 'overwrite')} style={{ backgroundColor: draft.conflict === 'overwrite' ? '#dbeafe' : '#f2f4f7', padding: 8, borderRadius: 8 }}><Text>Overwrite</Text></Pressable><Pressable onPress={() => updateDraft('conflict', 'skip')} style={{ backgroundColor: draft.conflict === 'skip' ? '#dbeafe' : '#f2f4f7', padding: 8, borderRadius: 8 }}><Text>Skip</Text></Pressable></View><View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginBottom: 10 }}><Text>Synchronize deletions</Text><Switch value={draft.deletions} onValueChange={(value) => updateDraft('deletions', value)} /></View>{testResult === 'success' ? <Text  style={{ color: '#067647', marginBottom: 8 }}>Connection successful</Text> : null}<View style={{ flexDirection: 'row', gap: 12 }}><Pressable disabled={testing || saving} onPress={() => void testConnection()} style={{ padding: 11, borderRadius: 8 }}><Text style={{ color: testing ? '#98a2b3' : '#2864dc' }}>{testing ? 'Testing…' : 'Test connection'}</Text></Pressable><Pressable disabled={saving || testing} onPress={() => void save()} style={{ backgroundColor: saving ? '#98a2b3' : '#2864dc', padding: 11, borderRadius: 8, flex: 1, alignItems: 'center' }}><Text style={{ color: '#fff', fontWeight: '600' }}>{saving ? 'Saving…' : 'Save'}</Text></Pressable><Pressable disabled={saving || testing} onPress={() => setEditing(undefined)} style={{ padding: 11 }}><Text style={{ color: '#2864dc' }}>Cancel</Text></Pressable></View></ScrollView> : null}
  </SafeAreaView>;
}
