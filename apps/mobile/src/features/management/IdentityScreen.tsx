import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, FlatList, Pressable, SafeAreaView, Text, View } from 'react-native';
import { useRouter } from 'expo-router';
import { useMobileRuntime } from '../../runtime.tsx';

export function IdentityScreen() {
  const runtime = useMobileRuntime();
  const router = useRouter();
  const [capabilities, setCapabilities] = useState<Record<string, { supported: boolean; reason?: string }>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const load = useCallback(async () => {
    try { setCapabilities((await runtime.client.administration.capabilities()).capabilities); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to load identity capabilities'); }
    finally { setLoading(false); }
  }, [runtime.client]);
  useEffect(() => { void load(); }, [load]);
  const rows = Object.entries(capabilities);
  return <SafeAreaView style={{ flex: 1, padding: 16 }}><View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 10 }}><Pressable onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>Back</Text></Pressable><Text accessibilityRole="header" style={{ fontSize: 22, fontWeight: '700' }}>Identity and audit</Text></View><Text style={{ color: '#667085', marginBottom: 10 }}>Member and audit operations remain tenant-scoped and are shown only after a server capability check.</Text>{loading ? <ActivityIndicator accessibilityLabel="Loading identity capabilities" /> : null}{error ? <Text accessibilityRole="alert" style={{ color: '#b42318' }}>{error}</Text> : <FlatList data={rows} keyExtractor={([key]) => key} ListEmptyComponent={<Text style={{ color: '#667085' }}>No identity capabilities reported.</Text>} renderItem={({ item: [key, capability] }) => <View style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 10 }}><Text>{key}</Text><Text style={{ color: capability.supported ? '#16803c' : '#b42318' }}>{capability.supported ? 'supported' : `unavailable${capability.reason ? ` · ${capability.reason}` : ''}`}</Text></View>} />}</SafeAreaView>;
}
