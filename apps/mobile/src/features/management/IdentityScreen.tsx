import { useCallback, useEffect, useRef, useState } from 'react';
import { ActivityIndicator, FlatList, Pressable, SafeAreaView, Text, View } from 'react-native';
import { useRouter } from 'expo-router';
import { formatMessage } from '@weknora/i18n';
import { useMobileRuntime } from '../../runtime.tsx';

export function IdentityScreen() {
  const runtime = useMobileRuntime();
  const router = useRouter();
  const t = (key: string, values: Record<string, string | number> = {}) => formatMessage(runtime.locale, key, values);
  const [capabilities, setCapabilities] = useState<Record<string, { supported: boolean; reason?: string }>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const loadGeneration = useRef(0);
  const load = useCallback(async () => {
    const generation = ++loadGeneration.current;
    setLoading(true); setError('');
    try {
      const next = await runtime.client.administration.capabilities();
      if (generation === loadGeneration.current) setCapabilities(next.capabilities);
    } catch (cause) {
      if (generation === loadGeneration.current) setError(cause instanceof Error ? cause.message : t('mobileIdentity.loadFailed'));
    } finally {
      if (generation === loadGeneration.current) setLoading(false);
    }
  }, [runtime.client]);
  useEffect(() => { void load(); }, [load]);
  const rows = Object.entries(capabilities);
  return <SafeAreaView style={{ flex: 1, padding: 16 }}><View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 10 }}><Pressable onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>{t('mobileIdentity.back')}</Text></Pressable><Text accessibilityRole="header" style={{ fontSize: 22, fontWeight: '700' }}>{t('mobileIdentity.title')}</Text></View><Text style={{ color: '#667085', marginBottom: 10 }}>{t('mobileIdentity.description')}</Text>{loading ? <ActivityIndicator accessibilityLabel={t('mobileIdentity.loading')} /> : null}{error ? <Text accessibilityRole="alert" style={{ color: '#b42318' }}>{error}</Text> : <FlatList data={rows} keyExtractor={([key]) => key} ListEmptyComponent={<Text style={{ color: '#667085' }}>{t('mobileIdentity.empty')}</Text>} renderItem={({ item: [key, capability] }) => <View style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 10 }}><Text>{key}</Text><Text style={{ color: capability.supported ? '#16803c' : '#b42318' }}>{capability.supported ? t('mobileIdentity.supported') : capability.reason ? t('mobileIdentity.unavailableWithReason', { reason: capability.reason }) : t('mobileIdentity.unavailable')}</Text></View>} />}</SafeAreaView>;
}
