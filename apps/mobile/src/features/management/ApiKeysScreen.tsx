import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ActivityIndicator, Alert, FlatList, Pressable, SafeAreaView, Text, TextInput, View } from 'react-native';
import { useRouter } from 'expo-router';
import type { ApiKey } from '@weknora/api-client';
import { formatMessage } from '@weknora/i18n';
import { useMobileRuntime } from '../../runtime.tsx';
import { canManageApiKeys, MOBILE_API_KEY_CAPABILITIES, validateApiKeyDraft } from './api-keys.ts';

function tenantNumber(value: string | null): number | null {
  const id = value === null ? NaN : Number(value);
  return Number.isSafeInteger(id) && id > 0 ? id : null;
}

export function ApiKeysScreen() {
  const runtime = useMobileRuntime();
  const router = useRouter();
  const tenantId = tenantNumber(runtime.tenantId);
  const t = (key: string, values: Record<string, string | number> = {}) => formatMessage(runtime.locale, key, values);
  const role = useMemo(() => runtime.workspaces.find((item) => String(item.id) === runtime.tenantId)?.role, [runtime.tenantId, runtime.workspaces]);
  const writable = canManageApiKeys(role);
  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [name, setName] = useState('');
  const [selected, setSelected] = useState<string[]>(['retrieve']);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [newToken, setNewToken] = useState('');
  const loadGeneration = useRef(0);

  const load = useCallback(async () => {
    if (tenantId === null) { setLoading(false); setError(t('mobileApiKeys.noWorkspace')); return; }
    const generation = ++loadGeneration.current;
    setLoading(true); setError('');
    try {
      const next = await runtime.client.administration.tenantApiKeys.list(tenantId);
      if (generation === loadGeneration.current) setKeys(next);
    } catch (cause) {
      if (generation === loadGeneration.current) setError(cause instanceof Error ? cause.message : t('mobileApiKeys.loadFailed'));
    } finally {
      if (generation === loadGeneration.current) setLoading(false);
    }
  }, [runtime.client, runtime.locale, tenantId]);
  useEffect(() => { void load(); }, [load]);

  function toggleCapability(capability: string) {
    setSelected((current) => current.includes(capability) ? current.filter((item) => item !== capability) : [...current, capability]);
  }

  async function create() {
    if (tenantId === null || !writable || saving) return;
    const validation = validateApiKeyDraft(name, selected);
    if (validation.length) {
      setError(validation.map((item) => item === 'Name is required' ? t('mobileApiKeys.nameRequired') : item === 'Select at least one capability' ? t('mobileApiKeys.capabilityRequired') : item).join('. '));
      return;
    }
    setSaving(true); setError(''); setNewToken('');
    try {
      const key = await runtime.client.administration.tenantApiKeys.create(tenantId, { name: name.trim(), full_access: false, capabilities: selected, knowledge_base_ids: [] });
      if (typeof key.token === 'string' && key.token.trim()) setNewToken(key.token);
      setName(''); setSelected(['retrieve']); await load();
    } catch (cause) { setError(cause instanceof Error ? cause.message : t('mobileApiKeys.createFailed')); }
    finally { setSaving(false); }
  }

  function revoke(key: ApiKey) {
    if (tenantId === null || !writable) return;
    Alert.alert(t('mobileApiKeys.revokeTitle'), t('mobileApiKeys.revokeMessage', { name: key.name }), [
      { text: t('mobileApiKeys.cancel'), style: 'cancel' },
      { text: t('mobileApiKeys.revoke'), style: 'destructive', onPress: () => void (async () => {
        setError('');
        try { await runtime.client.administration.tenantApiKeys.revoke(tenantId, key.id); await load(); }
        catch (cause) { setError(cause instanceof Error ? cause.message : t('mobileApiKeys.revokeFailed')); }
      })() },
    ]);
  }

  return <SafeAreaView style={{ flex: 1, padding: 16 }}>
    <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 10 }}><Pressable onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>{t('mobileApiKeys.back')}</Text></Pressable><Text accessibilityRole="header" style={{ flex: 1, fontSize: 22, fontWeight: '700' }}>{t('mobileApiKeys.title')}</Text><Pressable onPress={() => void load()}><Text style={{ color: '#2864dc' }}>{t('mobileApiKeys.refresh')}</Text></Pressable></View>
    <Text style={{ color: '#667085', marginBottom: 10 }}>{role ? t('mobileApiKeys.description', { role }) : t('mobileApiKeys.roleUnavailable')}</Text>
    {!writable ? <Text style={{ color: '#667085', marginBottom: 8 }}>{t('mobileApiKeys.readOnly')}</Text> : null}
    {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 8 }}>{error}</Text> : null}
    {newToken ? <View style={{ backgroundColor: '#ecfdf3', padding: 10, borderRadius: 8, marginBottom: 10 }}><Text style={{ fontWeight: '700' }}>{t('mobileApiKeys.tokenTitle')}</Text><Text selectable style={{ marginTop: 4 }}>{newToken}</Text><Text style={{ color: '#667085', fontSize: 12, marginTop: 4 }}>{t('mobileApiKeys.tokenNote')}</Text></View> : null}
    {writable ? <View style={{ marginBottom: 12 }}><TextInput accessibilityLabel={t('mobileApiKeys.nameLabel')} value={name} onChangeText={setName} placeholder={t('mobileApiKeys.namePlaceholder')} style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /><View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 7, marginBottom: 8 }}>{MOBILE_API_KEY_CAPABILITIES.map((capability) => <Pressable key={capability} accessibilityRole="button" accessibilityLabel={t('mobileApiKeys.selectCapability') + `: ${capability}`} onPress={() => toggleCapability(capability)} style={{ paddingHorizontal: 8, paddingVertical: 6, borderRadius: 12, backgroundColor: selected.includes(capability) ? '#dbeafe' : '#f2f4f7' }}><Text>{capability}</Text></Pressable>)}</View><Pressable accessibilityRole="button" disabled={saving} onPress={() => void create()} style={{ backgroundColor: '#2864dc', padding: 11, borderRadius: 8, alignItems: 'center', opacity: saving ? 0.5 : 1 }}><Text style={{ color: '#fff', fontWeight: '600' }}>{saving ? t('mobileApiKeys.creating') : t('mobileApiKeys.create')}</Text></Pressable></View> : null}
    {loading ? <ActivityIndicator accessibilityLabel={t('mobileApiKeys.loading')} /> : <FlatList data={keys} keyExtractor={(item) => String(item.id)} ListEmptyComponent={<Text style={{ color: '#667085' }}>{t('mobileApiKeys.empty')}</Text>} renderItem={({ item }) => <View style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 10, flexDirection: 'row', justifyContent: 'space-between', gap: 8 }}><View style={{ flex: 1 }}><Text style={{ fontWeight: '600' }}>{item.name}</Text><Text style={{ color: '#667085', fontSize: 12 }}>{item.full_access ? t('mobileApiKeys.fullAccess') : (item.capabilities || []).join(', ') || t('mobileApiKeys.noCapabilities')} · {t('mobileApiKeys.created')} {item.created_at}</Text></View>{writable ? <Pressable accessibilityRole="button" accessibilityLabel={t('mobileApiKeys.revoke') + `: ${item.name}`} onPress={() => revoke(item)}><Text style={{ color: '#b42318' }}>{t('mobileApiKeys.revoke')}</Text></Pressable> : null}</View>} />}
  </SafeAreaView>;
}
