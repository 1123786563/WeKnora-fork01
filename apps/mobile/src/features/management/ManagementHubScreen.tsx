import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, FlatList, Pressable, SafeAreaView, Text, View } from 'react-native';
import { useRouter } from 'expo-router';
import { useMobileRuntime } from '../../runtime.tsx';
import { MOBILE_CAPABILITIES, type MobileCapability } from './capabilities.ts';

export function ManagementHubScreen() {
  const runtime = useMobileRuntime();
  const router = useRouter();
  const [edition, setEdition] = useState('unknown');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const load = useCallback(async () => {
    try { setEdition((await runtime.client.administration.capabilities()).edition); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Capability endpoint unavailable'); }
    finally { setLoading(false); }
  }, [runtime.client]);
  useEffect(() => { void load(); }, [load]);
  function open(capability: MobileCapability) {
    if (capability.key === 'configuration') router.push('/management/configuration');
    else if (capability.key === 'identity') router.push('/management/administration');
  }
  return <SafeAreaView style={{ flex: 1, padding: 16 }}>
    <View style={{ flexDirection: 'row', justifyContent: 'space-between', marginBottom: 10 }}><Text accessibilityRole="header" style={{ fontSize: 24, fontWeight: '700' }}>Manage</Text><Pressable onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>Back</Text></Pressable></View>
    {loading ? <ActivityIndicator accessibilityLabel="Loading capabilities" /> : <Text style={{ color: '#667085', marginBottom: 10 }}>Server edition: {edition}</Text>}
    {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 10 }}>{error}</Text> : null}
    <FlatList data={MOBILE_CAPABILITIES} keyExtractor={(item) => item.key} renderItem={({ item }) => <Pressable disabled={item.support === 'unsupported' || (!['configuration', 'identity'].includes(item.key))} onPress={() => open(item)} style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 12, opacity: item.support === 'unsupported' ? 0.55 : 1 }}><View style={{ flexDirection: 'row', justifyContent: 'space-between' }}><Text style={{ fontWeight: '600' }}>{item.label}</Text><Text style={{ color: item.support === 'unsupported' ? '#b42318' : '#2864dc' }}>{item.support}</Text></View>{item.reason ? <Text style={{ color: '#667085', fontSize: 12, marginTop: 4 }}>{item.reason}</Text> : null}</Pressable>} />
  </SafeAreaView>;
}
