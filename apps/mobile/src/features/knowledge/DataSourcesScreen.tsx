import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, FlatList, Pressable, SafeAreaView, Text, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import type { DataSource, DataSourceConnectorType } from '@weknora/api-client';
import { useMobileRuntime } from '../../runtime.tsx';
import { dataSourceStatusLabel, safeDataSourceType } from './data-sources.ts';

export function DataSourcesScreen() {
  const { id: rawId } = useLocalSearchParams<{ id: string }>();
  const knowledgeBaseId = Array.isArray(rawId) ? rawId[0] : rawId;
  const runtime = useMobileRuntime();
  const router = useRouter();
  const [sources, setSources] = useState<DataSource[]>([]);
  const [types, setTypes] = useState<DataSourceConnectorType[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

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
  return <SafeAreaView style={{ flex: 1, padding: 16 }}>
    <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 10 }}><Pressable onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>Back</Text></Pressable><Text accessibilityRole="header" style={{ flex: 1, fontSize: 22, fontWeight: '700' }}>Data sources</Text><Pressable onPress={() => void load()}><Text style={{ color: '#2864dc' }}>Refresh</Text></Pressable></View>
    <Text style={{ color: '#667085', marginBottom: 10 }}>Read-only mobile inventory. Credentials and connector config are never rendered here.</Text>
    {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 8 }}>{error}</Text> : null}
    {loading ? <ActivityIndicator accessibilityLabel="Loading data sources" /> : <FlatList data={sources} keyExtractor={(item) => item.id} ListEmptyComponent={<Text style={{ color: '#667085' }}>No data sources configured for this knowledge base.</Text>} renderItem={({ item }) => { const connector = typeById.get(item.type); return <View style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 12 }}><Text style={{ fontWeight: '600' }}>{item.name}</Text><Text style={{ color: '#667085', fontSize: 12, marginTop: 3 }}>{safeDataSourceType(item)} · {dataSourceStatusLabel(item)}{item.sync_mode ? ` · ${item.sync_mode}` : ''}</Text>{item.sync_schedule ? <Text style={{ color: '#667085', fontSize: 12, marginTop: 3 }}>Schedule: {item.sync_schedule}</Text> : null}{connector ? <Text style={{ color: '#667085', fontSize: 12, marginTop: 3 }}>{connector.auth_type} connector · {connector.capabilities.join(', ') || 'no declared capabilities'}</Text> : null}</View>; }} />}
  </SafeAreaView>;
}
