import { useCallback, useEffect, useRef, useState } from 'react';
import { ActivityIndicator, FlatList, Pressable, SafeAreaView, Text, View } from 'react-native';
import { useRouter } from 'expo-router';
import { formatMessage } from '@weknora/i18n';
import { useMobileRuntime } from '../../runtime.tsx';
import { MOBILE_CAPABILITIES, capabilityAction, capabilityMessageKeys, capabilityModeMessageKey, projectMobileCapability, type MobileCapability } from './capabilities.ts';

export function ManagementHubScreen() {
  const runtime = useMobileRuntime();
  const router = useRouter();
  const [edition, setEdition] = useState('unknown');
  const [serverCapabilities, setServerCapabilities] = useState<Record<string, { supported: boolean; reason?: string }>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const loadGeneration = useRef(0);
  const t = (key: string, values: Record<string, string | number> = {}) => formatMessage(runtime.locale, key, values);
  const load = useCallback(async () => {
    const generation = ++loadGeneration.current;
    setLoading(true); setError('');
    try {
      const result = await runtime.client.administration.capabilities();
      if (generation === loadGeneration.current) { setEdition(result.edition); setServerCapabilities(result.capabilities); }
    } catch (cause) {
      if (generation === loadGeneration.current) setError(cause instanceof Error ? cause.message : t('mobileManagement.endpointUnavailable'));
    } finally {
      if (generation === loadGeneration.current) setLoading(false);
    }
  }, [runtime.client, runtime.locale]);
  useEffect(() => { void load(); }, [load]);
  const capabilities = MOBILE_CAPABILITIES.map((item) => projectMobileCapability(item, serverCapabilities));
  function open(capability: MobileCapability) {
    const action = capabilityAction(capability);
    if (action.kind === 'route') router.push(action.route);
  }
  return <SafeAreaView style={{ flex: 1, padding: 16 }}>
    <View style={{ flexDirection: 'row', justifyContent: 'space-between', marginBottom: 10 }}><Text accessibilityRole="header" style={{ fontSize: 24, fontWeight: '700' }}>{t('mobileManagement.title')}</Text><Pressable onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>{t('mobileManagement.back')}</Text></Pressable></View>
    {loading ? <ActivityIndicator accessibilityLabel={t('mobileManagement.loading')} /> : <Text style={{ color: '#667085', marginBottom: 10 }}>{t('mobileManagement.edition', { edition })}</Text>}
    {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 10 }}>{error}</Text> : null}
    <FlatList data={capabilities} keyExtractor={(item) => item.key} renderItem={({ item }) => {
      const action = capabilityAction(item);
      const keys = capabilityMessageKeys(item.key);
      const base = MOBILE_CAPABILITIES.find((candidate) => candidate.key === item.key);
      const label = keys ? t(keys.label) : item.label;
      const reason = keys && base?.reason === item.reason
        ? t(keys.reason)
        : item.reason === 'Disabled by the server deployment' ? t('mobileManagement.serverDisabled') : item.reason;
      const actionable = action.kind === 'route';
      return <Pressable disabled={!actionable} onPress={actionable ? () => open(item) : undefined} accessibilityRole={actionable ? 'button' : undefined} style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 12, opacity: item.mode === 'unsupported' ? 0.55 : 1 }}>
        <View style={{ flexDirection: 'row', justifyContent: 'space-between', gap: 12 }}><Text style={{ flex: 1, fontWeight: '600' }}>{label}</Text><Text style={{ color: item.mode === 'unsupported' ? '#b42318' : item.mode === 'web-handoff' ? '#b54708' : '#2864dc' }}>{t(capabilityModeMessageKey(item.mode))}</Text></View>
        <Text style={{ color: '#667085', fontSize: 12, marginTop: 4 }}>{reason}</Text>
        <Text style={{ color: '#667085', fontSize: 12, marginTop: 3 }}>{t('mobileManagement.requiredRoles', { roles: item.requiredRoles.join(', ') })}</Text>
      </Pressable>;
    }} />
  </SafeAreaView>;
}
