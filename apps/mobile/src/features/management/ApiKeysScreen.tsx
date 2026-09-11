import { useCallback, useEffect, useMemo, useState } from 'react';
import { ActivityIndicator, Alert, FlatList, Pressable, SafeAreaView, Text, TextInput, View } from 'react-native';
import { useRouter } from 'expo-router';
import type { ApiKey } from '@weknora/api-client';
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
  const role = useMemo(() => runtime.workspaces.find((item) => String(item.id) === runtime.tenantId)?.role, [runtime.tenantId, runtime.workspaces]);
  const writable = canManageApiKeys(role);
  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [name, setName] = useState('');
  const [selected, setSelected] = useState<string[]>(['retrieve']);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [newToken, setNewToken] = useState('');

  const load = useCallback(async () => {
    if (tenantId === null) { setLoading(false); setError('No active workspace is selected'); return; }
    setLoading(true); setError('');
    try { setKeys(await runtime.client.administration.tenantApiKeys.list(tenantId)); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to load API keys'); }
    finally { setLoading(false); }
  }, [runtime.client, tenantId]);
  useEffect(() => { void load(); }, [load]);

  function toggleCapability(capability: string) {
    setSelected((current) => current.includes(capability) ? current.filter((item) => item !== capability) : [...current, capability]);
  }

  async function create() {
    if (tenantId === null || !writable || saving) return;
    const validation = validateApiKeyDraft(name, selected);
    if (validation.length) { setError(validation.join('. ')); return; }
    setSaving(true); setError(''); setNewToken('');
    try {
      const key = await runtime.client.administration.tenantApiKeys.create(tenantId, { name: name.trim(), full_access: false, capabilities: selected, knowledge_base_ids: [] });
      if (typeof key.token === 'string' && key.token.trim()) setNewToken(key.token);
      setName(''); setSelected(['retrieve']); await load();
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to create API key; no local key was added'); }
    finally { setSaving(false); }
  }

  function revoke(key: ApiKey) {
    if (tenantId === null || !writable) return;
    Alert.alert('Revoke API key?', `${key.name} will stop working immediately.`, [
      { text: 'Cancel', style: 'cancel' },
      { text: 'Revoke', style: 'destructive', onPress: () => void (async () => {
        setError('');
        try { await runtime.client.administration.tenantApiKeys.revoke(tenantId, key.id); await load(); }
        catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to revoke API key; server state was kept'); }
      })() },
    ]);
  }

  return <SafeAreaView style={{ flex: 1, padding: 16 }}>
    <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 10 }}><Pressable onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>Back</Text></Pressable><Text accessibilityRole="header" style={{ flex: 1, fontSize: 22, fontWeight: '700' }}>API keys</Text><Pressable onPress={() => void load()}><Text style={{ color: '#2864dc' }}>Refresh</Text></Pressable></View>
    <Text style={{ color: '#667085', marginBottom: 10 }}>{role ? `Workspace role: ${role}` : 'Workspace role unavailable'} · secrets are shown only once</Text>
    {!writable ? <Text style={{ color: '#667085', marginBottom: 8 }}>Only the workspace owner can manage API keys. This screen remains read-only for other roles.</Text> : null}
    {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 8 }}>{error}</Text> : null}
    {newToken ? <View style={{ backgroundColor: '#ecfdf3', padding: 10, borderRadius: 8, marginBottom: 10 }}><Text style={{ fontWeight: '700' }}>Copy this token now</Text><Text selectable style={{ marginTop: 4 }}>{newToken}</Text><Text style={{ color: '#667085', fontSize: 12, marginTop: 4 }}>It will not be persisted by the mobile client.</Text></View> : null}
    {writable ? <View style={{ marginBottom: 12 }}><TextInput accessibilityLabel="API key name" value={name} onChangeText={setName} placeholder="Mobile integration" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /><View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 7, marginBottom: 8 }}>{MOBILE_API_KEY_CAPABILITIES.map((capability) => <Pressable key={capability} onPress={() => toggleCapability(capability)} style={{ paddingHorizontal: 8, paddingVertical: 6, borderRadius: 12, backgroundColor: selected.includes(capability) ? '#dbeafe' : '#f2f4f7' }}><Text>{capability}</Text></Pressable>)}</View><Pressable accessibilityRole="button" disabled={saving} onPress={() => void create()} style={{ backgroundColor: '#2864dc', padding: 11, borderRadius: 8, alignItems: 'center', opacity: saving ? 0.5 : 1 }}><Text style={{ color: '#fff', fontWeight: '600' }}>{saving ? 'Creating…' : 'Create scoped key'}</Text></Pressable></View> : null}
    {loading ? <ActivityIndicator accessibilityLabel="Loading API keys" /> : <FlatList data={keys} keyExtractor={(item) => String(item.id)} ListEmptyComponent={<Text style={{ color: '#667085' }}>No API keys returned.</Text>} renderItem={({ item }) => <View style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 10, flexDirection: 'row', justifyContent: 'space-between', gap: 8 }}><View style={{ flex: 1 }}><Text style={{ fontWeight: '600' }}>{item.name}</Text><Text style={{ color: '#667085', fontSize: 12 }}>{item.full_access ? 'full access' : (item.capabilities || []).join(', ') || 'no capabilities'} · created {item.created_at}</Text></View>{writable ? <Pressable onPress={() => revoke(item)}><Text style={{ color: '#b42318' }}>Revoke</Text></Pressable> : null}</View>} />}
  </SafeAreaView>;
}
