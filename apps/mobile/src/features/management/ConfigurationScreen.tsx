import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Alert, Pressable, SafeAreaView, ScrollView, Switch, Text, TextInput, View } from 'react-native';
import { useRouter } from 'expo-router';
import type { AgentConfiguration, ConfigurationRecord, McpConfiguration, ModelConfiguration, SkillConfiguration } from '@weknora/api-client';
import { useMobileRuntime } from '../../runtime.tsx';
import { configurationPayload, credentialInput, nativeConfigurationDraftFrom, newNativeConfigurationDraft, type NativeConfigurationDraft, type NativeConfigurationSection } from './configuration-form.ts';

interface ConfigurationRows { agents: AgentConfiguration[]; models: ModelConfiguration[]; mcp: McpConfiguration[]; skills: SkillConfiguration[] }

function errorText(cause: unknown, fallback: string): string { return cause instanceof Error ? cause.message : fallback; }

export function ConfigurationScreen() {
  const runtime = useMobileRuntime();
  const router = useRouter();
  const [rows, setRows] = useState<ConfigurationRows>({ agents: [], models: [], mcp: [], skills: [] });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [editor, setEditor] = useState<{ section: NativeConfigurationSection; record?: ConfigurationRecord } | null>(null);
  const [draft, setDraft] = useState<NativeConfigurationDraft | null>(null);
  const [saving, setSaving] = useState(false);
  const [removing, setRemoving] = useState('');

  const load = useCallback(async () => {
    setError('');
    const [agents, models, mcp, skills] = await Promise.allSettled([
      runtime.client.configuration.agents.list(), runtime.client.configuration.models.list(), runtime.client.configuration.mcp.list(), runtime.client.configuration.skills.list(),
    ]);
    const failures = [agents, models, mcp, skills].filter((result): result is PromiseRejectedResult => result.status === 'rejected');
    if (failures.length) setError(`${failures.length} configuration surface(s) are unavailable or forbidden`);
    setRows({ agents: agents.status === 'fulfilled' ? agents.value : [], models: models.status === 'fulfilled' ? models.value : [], mcp: mcp.status === 'fulfilled' ? mcp.value : [], skills: skills.status === 'fulfilled' ? skills.value : [] });
    setLoading(false);
  }, [runtime.client]);

  useEffect(() => { void load(); }, [load]);

  function openEditor(section: NativeConfigurationSection, record?: ConfigurationRecord) {
    setEditor({ section, record });
    setDraft(record ? nativeConfigurationDraftFrom(section, record) : newNativeConfigurationDraft(section));
    setError('');
  }

  function updateDraft<K extends keyof NativeConfigurationDraft>(key: K, value: NativeConfigurationDraft[K]) {
    setDraft((current) => current ? { ...current, [key]: value } : current);
  }

  async function save() {
    if (!draft) return;
    setSaving(true); setError('');
    try {
      const payload = configurationPayload(draft);
      let saved: ConfigurationRecord;
      if (draft.section === 'agents') saved = draft.id ? await runtime.client.configuration.agents.update(draft.id, payload) : await runtime.client.configuration.agents.create(payload);
      else if (draft.section === 'models') saved = draft.id ? await runtime.client.configuration.models.update(draft.id, payload) : await runtime.client.configuration.models.create(payload);
      else saved = draft.id ? await runtime.client.configuration.mcp.update(draft.id, payload) : await runtime.client.configuration.mcp.create(payload);
      const credentials = credentialInput(draft);
      if (draft.section === 'models' && Object.keys(credentials).length) await runtime.client.configuration.models.credentials.put(saved.id, credentials);
      if (draft.section === 'mcp' && Object.keys(credentials).length) await runtime.client.configuration.mcp.credentials.put(saved.id, credentials);
      setEditor(null); setDraft(null); await load();
    } catch (cause) { setError(errorText(cause, 'Configuration operation failed')); }
    finally { setSaving(false); }
  }

  async function remove(section: NativeConfigurationSection, itemId: string) {
    setRemoving(itemId); setError('');
    try {
      if (section === 'agents') await runtime.client.configuration.agents.remove(itemId);
      else if (section === 'models') await runtime.client.configuration.models.remove(itemId);
      else await runtime.client.configuration.mcp.remove(itemId);
      await load();
    } catch (cause) { setError(errorText(cause, 'Unable to remove configuration')); }
    finally { setRemoving(''); }
  }

  const sections: Array<{ key: NativeConfigurationSection | 'skills'; title: string; items: ConfigurationRecord[]; writable: boolean }> = [
    { key: 'agents', title: 'Agents', items: rows.agents, writable: true },
    { key: 'models', title: 'Models', items: rows.models, writable: true },
    { key: 'mcp', title: 'MCP services', items: rows.mcp, writable: true },
    { key: 'skills', title: 'Skills', items: rows.skills, writable: false },
  ];

  return <SafeAreaView style={{ flex: 1, padding: 16 }}>
    <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 10 }}><Pressable onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>Back</Text></Pressable><Text accessibilityRole="header" style={{ flex: 1, fontSize: 22, fontWeight: '700' }}>Configuration</Text><Pressable onPress={() => void load()}><Text style={{ color: '#2864dc' }}>Refresh</Text></Pressable></View>
    <Text style={{ color: '#667085', marginBottom: 10 }}>Agents, models, and MCP can be managed here. Credentials are write-only and never prefilled; Skills remain read-only.</Text>
    {loading ? <ActivityIndicator accessibilityLabel="Loading configuration" /> : null}
    {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 8 }}>{error}</Text> : null}
    <ScrollView keyboardShouldPersistTaps="handled">
      {sections.map((section) => <View key={section.key} style={{ marginBottom: 14 }}><View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' }}><Text style={{ fontSize: 16, fontWeight: '700', marginTop: 12, marginBottom: 4 }}>{section.title}</Text>{section.writable ? <Pressable onPress={() => openEditor(section.key as NativeConfigurationSection)}><Text style={{ color: '#2864dc' }}>Add</Text></Pressable> : null}</View>{section.items.length === 0 ? <Text style={{ color: '#667085' }}>{section.writable ? 'No configured entries.' : 'No readable entries.'}</Text> : section.items.map((item) => <View key={`${section.key}:${item.id}`} style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 10 }}><Text>{item.name}</Text><Text style={{ color: '#667085', fontSize: 12, marginTop: 3 }}>{section.key === 'skills' ? (typeof item.description === 'string' ? item.description : 'Read-only catalog entry') : section.key === 'mcp' ? `${item.url || 'No URL'} · ${item.enabled === false ? 'disabled' : 'enabled'}` : section.key === 'models' ? `${item.type || 'unknown type'} · ${item.source || 'unknown source'}` : 'Configured agent'}</Text>{section.writable ? <View style={{ flexDirection: 'row', gap: 14, marginTop: 7 }}><Pressable disabled={Boolean(removing)} onPress={() => openEditor(section.key as NativeConfigurationSection, item)}><Text style={{ color: '#2864dc' }}>Edit</Text></Pressable><Pressable disabled={Boolean(removing)} onPress={() => Alert.alert('Remove configuration?', item.name, [{ text: 'Cancel', style: 'cancel' }, { text: 'Remove', style: 'destructive', onPress: () => void remove(section.key as NativeConfigurationSection, item.id) }])}><Text style={{ color: removing === item.id ? '#98a2b3' : '#b42318' }}>Remove</Text></Pressable></View> : null}</View>)}</View>)}
      {editor && draft ? <View style={{ borderTopColor: '#d0d5dd', borderTopWidth: 1, paddingTop: 14, marginTop: 4 }}><Text style={{ fontSize: 18, fontWeight: '700', marginBottom: 8 }}>{editor.record ? 'Edit' : 'Create'} {draft.section}</Text><TextInput accessibilityLabel="Configuration name" value={draft.name} onChangeText={(value) => updateDraft('name', value)} placeholder="Name" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /><TextInput accessibilityLabel="Configuration description" value={draft.description} onChangeText={(value) => updateDraft('description', value)} placeholder="Description" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} />{draft.section === 'agents' ? <TextInput accessibilityLabel="Agent avatar" value={draft.avatar} onChangeText={(value) => updateDraft('avatar', value)} placeholder="Avatar URL (optional)" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /> : null}{draft.section === 'models' ? <><TextInput accessibilityLabel="Model type" value={draft.type} onChangeText={(value) => updateDraft('type', value)} placeholder="Model type" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /><TextInput accessibilityLabel="Model source" value={draft.source} onChangeText={(value) => updateDraft('source', value)} placeholder="Source" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /><TextInput accessibilityLabel="Model API key" value={draft.apiKey} onChangeText={(value) => updateDraft('apiKey', value)} placeholder="New API key (optional)" secureTextEntry style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /><TextInput accessibilityLabel="Model app secret" value={draft.appSecret} onChangeText={(value) => updateDraft('appSecret', value)} placeholder="New app secret (optional)" secureTextEntry style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /></> : null}{draft.section === 'mcp' ? <><TextInput accessibilityLabel="MCP URL" value={draft.url} onChangeText={(value) => updateDraft('url', value)} placeholder="https://…" autoCapitalize="none" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /><Text style={{ marginBottom: 5 }}>Transport: {draft.transportType}</Text><View style={{ flexDirection: 'row', gap: 8, marginBottom: 8 }}><Pressable onPress={() => updateDraft('transportType', 'sse')} style={{ backgroundColor: draft.transportType === 'sse' ? '#dbeafe' : '#f2f4f7', padding: 8, borderRadius: 8 }}><Text>SSE</Text></Pressable><Pressable onPress={() => updateDraft('transportType', 'http-streamable')} style={{ backgroundColor: draft.transportType === 'http-streamable' ? '#dbeafe' : '#f2f4f7', padding: 8, borderRadius: 8 }}><Text>HTTP stream</Text></Pressable><Pressable onPress={() => updateDraft('transportType', 'stdio')} style={{ backgroundColor: draft.transportType === 'stdio' ? '#dbeafe' : '#f2f4f7', padding: 8, borderRadius: 8 }}><Text>stdio</Text></Pressable></View><View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}><Text>Enabled</Text><Switch value={draft.enabled} onValueChange={(value) => updateDraft('enabled', value)} /></View><TextInput accessibilityLabel="MCP API key" value={draft.apiKey} onChangeText={(value) => updateDraft('apiKey', value)} placeholder="New API key (optional)" secureTextEntry style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /><TextInput accessibilityLabel="MCP token" value={draft.token} onChangeText={(value) => updateDraft('token', value)} placeholder="New token (optional)" secureTextEntry style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /></> : null}<TextInput accessibilityLabel="Safe configuration JSON" value={draft.details} onChangeText={(value) => updateDraft('details', value)} placeholder="Safe configuration JSON" multiline style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, minHeight: 100, marginBottom: 10, fontFamily: 'monospace' }} /><View style={{ flexDirection: 'row', gap: 12 }}><Pressable disabled={saving} onPress={() => void save()} style={{ backgroundColor: saving ? '#98a2b3' : '#2864dc', padding: 11, borderRadius: 8, flex: 1, alignItems: 'center' }}><Text style={{ color: '#fff', fontWeight: '600' }}>{saving ? 'Saving…' : 'Save'}</Text></Pressable><Pressable disabled={saving} onPress={() => { setEditor(null); setDraft(null); }} style={{ padding: 11 }}><Text style={{ color: '#2864dc' }}>Cancel</Text></Pressable></View></View> : null}
    </ScrollView>
  </SafeAreaView>;
}
