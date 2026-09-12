import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Alert, FlatList, Pressable, SafeAreaView, ScrollView, Switch, Text, TextInput, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import type { DataSource, DataSourceConnectorType } from '@weknora/api-client';
import { useMobileRuntime } from '../../runtime.tsx';
import { dataSourceStatusLabel, safeDataSourceType } from './data-sources.ts';
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
  const [sources, setSources] = useState<DataSource[]>([]);
  const [types, setTypes] = useState<DataSourceConnectorType[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [editing, setEditing] = useState<DataSource | null | undefined>(undefined);
  const [draft, setDraft] = useState<NativeDataSourceDraft>(EMPTY_DRAFT);
  const [saving, setSaving] = useState(false);
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
  function updateDraft<K extends keyof NativeDataSourceDraft>(key: K, value: NativeDataSourceDraft[K]) { setDraft((current) => ({ ...current, [key]: value })); }
  function openCreate() { setEditing(null); setDraft({ ...EMPTY_DRAFT, type: types[0]?.type ?? '' }); setError(''); }
  function openEdit(source: DataSource) { setEditing(source); setDraft(nativeDataSourceDraftFrom(source)); setError(''); }
  async function save() {
    setSaving(true); setError('');
    try {
      const input = buildNativeDataSourceInput(draft, knowledgeBaseId);
      const config = input.config as Record<string, unknown>;
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
  async function run(source: DataSource, operation: 'sync' | 'pause' | 'resume') {
    setAction(`${operation}:${source.id}`); setError('');
    try {
      const result = operation === 'sync' ? await runtime.client.dataSources.sync(source.id) : operation === 'pause' ? await runtime.client.dataSources.pause(source.id) : await runtime.client.dataSources.resume(source.id);
      const failure = connectionError(result); if (failure) throw new Error(failure);
      await load();
    } catch (cause) { setError(cause instanceof Error ? cause.message : `Unable to ${operation} data source`); }
    finally { setAction(null); }
  }
  async function remove(source: DataSource) {
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
    <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 10 }}><Pressable onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>Back</Text></Pressable><Text accessibilityRole="header" style={{ flex: 1, fontSize: 22, fontWeight: '700' }}>Data sources</Text><Pressable onPress={openCreate}><Text style={{ color: '#2864dc' }}>Add</Text></Pressable><Pressable onPress={() => void load()}><Text style={{ color: '#2864dc' }}>Refresh</Text></Pressable></View>
    <Text style={{ color: '#667085', marginBottom: 10 }}>Configure connectors and synchronization. Existing credentials are never shown; enter new credentials only when needed.</Text>
    {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 8 }}>{error}</Text> : null}
    {loading ? <ActivityIndicator accessibilityLabel="Loading data sources" /> : <FlatList data={sources} keyExtractor={(item) => item.id} ListEmptyComponent={<Text style={{ color: '#667085' }}>No data sources configured for this knowledge base.</Text>} renderItem={({ item }) => { const connector = typeById.get(item.type); const busy = action !== null; return <View style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 12 }}><Text style={{ fontWeight: '600' }}>{item.name}</Text><Text style={{ color: '#667085', fontSize: 12, marginTop: 3 }}>{safeDataSourceType(item)} · {dataSourceStatusLabel(item)}{item.sync_mode ? ` · ${item.sync_mode}` : ''}</Text>{item.sync_schedule ? <Text style={{ color: '#667085', fontSize: 12, marginTop: 3 }}>Schedule: {item.sync_schedule}</Text> : null}{connector ? <Text style={{ color: '#667085', fontSize: 12, marginTop: 3 }}>{connector.auth_type} connector · {connector.capabilities.join(', ') || 'no declared capabilities'}</Text> : null}<View style={{ flexDirection: 'row', gap: 12, flexWrap: 'wrap', marginTop: 8 }}><Pressable disabled={busy} onPress={() => openEdit(item)}><Text style={{ color: busy ? '#98a2b3' : '#2864dc' }}>Edit</Text></Pressable><Pressable disabled={busy} onPress={() => void run(item, 'sync')}><Text style={{ color: busy ? '#98a2b3' : '#2864dc' }}>Sync</Text></Pressable>{item.status === 'paused' ? <Pressable disabled={busy} onPress={() => void run(item, 'resume')}><Text style={{ color: busy ? '#98a2b3' : '#2864dc' }}>Resume</Text></Pressable> : <Pressable disabled={busy} onPress={() => void run(item, 'pause')}><Text style={{ color: busy ? '#98a2b3' : '#2864dc' }}>Pause</Text></Pressable>}<Pressable disabled={busy} onPress={() => void showLogs(item)}><Text style={{ color: busy ? '#98a2b3' : '#2864dc' }}>Logs</Text></Pressable><Pressable disabled={busy} onPress={() => Alert.alert('Delete data source?', item.name, [{ text: 'Cancel', style: 'cancel' }, { text: 'Delete', style: 'destructive', onPress: () => void remove(item) }])}><Text style={{ color: busy ? '#98a2b3' : '#b42318' }}>Delete</Text></Pressable></View></View>; }} />}
    {logsSource ? <ScrollView style={{ maxHeight: 180, marginTop: 12, borderColor: '#eaecf0', borderWidth: 1, borderRadius: 8, padding: 10 }}><View style={{ flexDirection: 'row', justifyContent: 'space-between' }}><Text style={{ fontWeight: '700' }}>Sync logs · {logsSource.name}</Text><Pressable onPress={() => setLogsSource(null)}><Text style={{ color: '#2864dc' }}>Close</Text></Pressable></View>{logs.length ? logs.map((log) => <View key={log.id} style={{ paddingVertical: 6, borderBottomColor: '#eaecf0', borderBottomWidth: 1 }}><Text>{log.status}</Text><Text style={{ color: '#667085', fontSize: 12 }}>{typeof log.started_at === 'string' ? log.started_at : ''}{typeof log.error_message === 'string' ? ` · ${log.error_message}` : ''}</Text></View>) : <Text style={{ color: '#667085', marginTop: 8 }}>No sync logs returned.</Text>}</ScrollView> : null}
    {editing !== undefined ? <ScrollView style={{ marginTop: 12 }} keyboardShouldPersistTaps="handled"><Text style={{ fontSize: 18, fontWeight: '700', marginBottom: 8 }}>{editing ? 'Edit data source' : 'Add data source'}</Text><TextInput accessibilityLabel="Data source name" value={draft.name} onChangeText={(value) => updateDraft('name', value)} placeholder="Name" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /><TextInput accessibilityLabel="Data source type" value={draft.type} onChangeText={(value) => updateDraft('type', value)} placeholder="Connector type" autoCapitalize="none" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} />{types.length ? <Text style={{ color: '#667085', marginBottom: 8 }}>Available: {types.map((type) => type.type).join(', ')}</Text> : null}<TextInput accessibilityLabel="Data source credentials" value={draft.credentialsText} onChangeText={(value) => updateDraft('credentialsText', value)} placeholder="Credentials: token = secret" secureTextEntry multiline style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, minHeight: 70, marginBottom: 8 }} /><TextInput accessibilityLabel="Data source settings" value={draft.settingsText} onChangeText={(value) => updateDraft('settingsText', value)} placeholder="Settings: workspace_id = example" multiline style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, minHeight: 70, marginBottom: 8 }} /><TextInput accessibilityLabel="Sync schedule" value={draft.schedule} onChangeText={(value) => updateDraft('schedule', value)} placeholder="Cron schedule" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /><Text style={{ marginBottom: 4 }}>Sync mode: {draft.mode}</Text><View style={{ flexDirection: 'row', gap: 8, marginBottom: 8 }}><Pressable onPress={() => updateDraft('mode', 'incremental')} style={{ backgroundColor: draft.mode === 'incremental' ? '#dbeafe' : '#f2f4f7', padding: 8, borderRadius: 8 }}><Text>Incremental</Text></Pressable><Pressable onPress={() => updateDraft('mode', 'full')} style={{ backgroundColor: draft.mode === 'full' ? '#dbeafe' : '#f2f4f7', padding: 8, borderRadius: 8 }}><Text>Full</Text></Pressable></View><Text style={{ marginBottom: 4 }}>Conflict strategy: {draft.conflict}</Text><View style={{ flexDirection: 'row', gap: 8, marginBottom: 8 }}><Pressable onPress={() => updateDraft('conflict', 'overwrite')} style={{ backgroundColor: draft.conflict === 'overwrite' ? '#dbeafe' : '#f2f4f7', padding: 8, borderRadius: 8 }}><Text>Overwrite</Text></Pressable><Pressable onPress={() => updateDraft('conflict', 'skip')} style={{ backgroundColor: draft.conflict === 'skip' ? '#dbeafe' : '#f2f4f7', padding: 8, borderRadius: 8 }}><Text>Skip</Text></Pressable></View><View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginBottom: 10 }}><Text>Synchronize deletions</Text><Switch value={draft.deletions} onValueChange={(value) => updateDraft('deletions', value)} /></View><View style={{ flexDirection: 'row', gap: 12 }}><Pressable disabled={saving} onPress={() => void save()} style={{ backgroundColor: saving ? '#98a2b3' : '#2864dc', padding: 11, borderRadius: 8, flex: 1, alignItems: 'center' }}><Text style={{ color: '#fff', fontWeight: '600' }}>{saving ? 'Saving…' : 'Save'}</Text></Pressable><Pressable disabled={saving} onPress={() => setEditing(undefined)} style={{ padding: 11 }}><Text style={{ color: '#2864dc' }}>Cancel</Text></Pressable></View></ScrollView> : null}
  </SafeAreaView>;
}
