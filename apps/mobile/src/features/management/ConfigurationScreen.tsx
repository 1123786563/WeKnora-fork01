import { useCallback, useEffect, useRef, useState } from 'react';
import { ActivityIndicator, Alert, Pressable, SafeAreaView, ScrollView, Switch, Text, TextInput, View } from 'react-native';
import { useRouter } from 'expo-router';
import type { AgentConfiguration, ConfigurationRecord, McpConfiguration, ModelConfiguration, SkillConfiguration } from '@weknora/api-client';
import { formatMessage } from '@weknora/i18n';
import { useMobileRuntime } from '../../runtime.tsx';
import { configurationPayload, credentialInput, nativeConfigurationDraftFrom, newNativeConfigurationDraft, type NativeConfigurationDraft, type NativeConfigurationSection } from './configuration-form.ts';

interface ConfigurationRows { agents: AgentConfiguration[]; models: ModelConfiguration[]; mcp: McpConfiguration[]; skills: SkillConfiguration[] }

function errorText(cause: unknown, fallback: string): string { return cause instanceof Error ? cause.message : fallback; }

export function ConfigurationScreen() {
  const runtime = useMobileRuntime();
  const router = useRouter();
  const t = (key: string, values: Record<string, string | number> = {}) => formatMessage(runtime.locale, key, values);
  const [rows, setRows] = useState<ConfigurationRows>({ agents: [], models: [], mcp: [], skills: [] });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [editor, setEditor] = useState<{ section: NativeConfigurationSection; record?: ConfigurationRecord } | null>(null);
  const [draft, setDraft] = useState<NativeConfigurationDraft | null>(null);
  const [saving, setSaving] = useState(false);
  const [removing, setRemoving] = useState('');
  const loadGeneration = useRef(0);

  const load = useCallback(async () => {
    const generation = ++loadGeneration.current;
    setLoading(true);
    setError('');
    const [agents, models, mcp, skills] = await Promise.allSettled([
      runtime.client.configuration.agents.list(), runtime.client.configuration.models.list(), runtime.client.configuration.mcp.list(), runtime.client.configuration.skills.list(),
    ]);
    const failures = [agents, models, mcp, skills].filter((result): result is PromiseRejectedResult => result.status === 'rejected');
    if (generation !== loadGeneration.current) return;
    if (failures.length) setError(`${failures.length} ${t('mobileConfiguration.loadFailed')}`);
    setRows({ agents: agents.status === 'fulfilled' ? agents.value : [], models: models.status === 'fulfilled' ? models.value : [], mcp: mcp.status === 'fulfilled' ? mcp.value : [], skills: skills.status === 'fulfilled' ? skills.value : [] });
    setLoading(false);
  }, [runtime.client, runtime.locale]);

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
    } catch (cause) { setError(errorText(cause, t('mobileConfiguration.operationFailed'))); }
    finally { setSaving(false); }
  }

  async function remove(section: NativeConfigurationSection, itemId: string) {
    setRemoving(itemId); setError('');
    try {
      if (section === 'agents') await runtime.client.configuration.agents.remove(itemId);
      else if (section === 'models') await runtime.client.configuration.models.remove(itemId);
      else await runtime.client.configuration.mcp.remove(itemId);
      await load();
    } catch (cause) { setError(errorText(cause, t('mobileConfiguration.removeFailed'))); }
    finally { setRemoving(''); }
  }

  const sections: Array<{ key: NativeConfigurationSection | 'skills'; title: string; items: ConfigurationRecord[]; writable: boolean }> = [
    { key: 'agents', title: t('mobileConfiguration.agents'), items: rows.agents, writable: true },
    { key: 'models', title: t('mobileConfiguration.models'), items: rows.models, writable: true },
    { key: 'mcp', title: t('mobileConfiguration.mcp'), items: rows.mcp, writable: true },
    { key: 'skills', title: t('mobileConfiguration.skills'), items: rows.skills, writable: false },
  ];

  return <SafeAreaView style={{ flex: 1, padding: 16 }}>
    <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 10 }}><Pressable onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>{t('mobileConfiguration.back')}</Text></Pressable><Text accessibilityRole="header" style={{ flex: 1, fontSize: 22, fontWeight: '700' }}>{t('mobileConfiguration.title')}</Text><Pressable onPress={() => void load()}><Text style={{ color: '#2864dc' }}>{t('mobileConfiguration.refresh')}</Text></Pressable></View>
    <Text style={{ color: '#667085', marginBottom: 10 }}>{t('mobileConfiguration.description')}</Text>
    {loading ? <ActivityIndicator accessibilityLabel={t('mobileConfiguration.loading')} /> : null}
    {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 8 }}>{error}</Text> : null}
    <ScrollView keyboardShouldPersistTaps="handled">
      {sections.map((section) => <View key={section.key} style={{ marginBottom: 14 }}><View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' }}><Text style={{ fontSize: 16, fontWeight: '700', marginTop: 12, marginBottom: 4 }}>{section.title}</Text>{section.writable ? <Pressable onPress={() => openEditor(section.key as NativeConfigurationSection)}><Text style={{ color: '#2864dc' }}>{t('mobileConfiguration.add')}</Text></Pressable> : null}</View>{section.items.length === 0 ? <Text style={{ color: '#667085' }}>{section.writable ? t('mobileConfiguration.noConfigured') : t('mobileConfiguration.noReadable')}</Text> : section.items.map((item) => <View key={`${section.key}:${item.id}`} style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 10 }}><Text>{item.name}</Text><Text style={{ color: '#667085', fontSize: 12, marginTop: 3 }}>{section.key === 'skills' ? (typeof item.description === 'string' ? item.description : t('mobileConfiguration.readOnlyCatalog')) : section.key === 'mcp' ? `${item.url || t('mobileConfiguration.noUrl')} · ${item.enabled === false ? t('mobileConfiguration.disabled') : t('mobileConfiguration.enabled')}` : section.key === 'models' ? `${item.type || t('mobileConfiguration.unknownType')} · ${item.source || t('mobileConfiguration.unknownSource')}` : t('mobileConfiguration.configuredAgent')}</Text>{section.writable ? <View style={{ flexDirection: 'row', gap: 14, marginTop: 7 }}><Pressable disabled={Boolean(removing)} onPress={() => openEditor(section.key as NativeConfigurationSection, item)}><Text style={{ color: '#2864dc' }}>{t('mobileConfiguration.edit')}</Text></Pressable><Pressable disabled={Boolean(removing)} onPress={() => Alert.alert(t('mobileConfiguration.removeTitle'), item.name, [{ text: t('mobileConfiguration.cancel'), style: 'cancel' }, { text: t('mobileConfiguration.remove'), style: 'destructive', onPress: () => void remove(section.key as NativeConfigurationSection, item.id) }])}><Text style={{ color: removing === item.id ? '#98a2b3' : '#b42318' }}>{t('mobileConfiguration.remove')}</Text></Pressable></View> : null}</View>)}</View>)}
      {editor && draft ? <View style={{ borderTopColor: '#d0d5dd', borderTopWidth: 1, paddingTop: 14, marginTop: 4 }}><Text style={{ fontSize: 18, fontWeight: '700', marginBottom: 8 }}>{editor.record ? t('mobileConfiguration.edit') : t('mobileConfiguration.create')} {draft.section === 'agents' ? t('mobileConfiguration.agents') : draft.section === 'models' ? t('mobileConfiguration.models') : t('mobileConfiguration.mcp')}</Text><TextInput accessibilityLabel={t('mobileConfiguration.nameLabel')} value={draft.name} onChangeText={(value) => updateDraft('name', value)} placeholder={t('mobileConfiguration.namePlaceholder')} style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /><TextInput accessibilityLabel={t('mobileConfiguration.descriptionLabel')} value={draft.description} onChangeText={(value) => updateDraft('description', value)} placeholder={t('mobileConfiguration.descriptionPlaceholder')} style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} />{draft.section === 'agents' ? <TextInput accessibilityLabel={t('mobileConfiguration.avatarLabel')} value={draft.avatar} onChangeText={(value) => updateDraft('avatar', value)} placeholder={t('mobileConfiguration.avatarPlaceholder')} style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /> : null}{draft.section === 'models' ? <><TextInput accessibilityLabel={t('mobileConfiguration.modelTypeLabel')} value={draft.type} onChangeText={(value) => updateDraft('type', value)} placeholder={t('mobileConfiguration.modelTypePlaceholder')} style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /><TextInput accessibilityLabel={t('mobileConfiguration.modelSourceLabel')} value={draft.source} onChangeText={(value) => updateDraft('source', value)} placeholder={t('mobileConfiguration.modelSourcePlaceholder')} style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /><TextInput accessibilityLabel={t('mobileConfiguration.apiKeyLabel')} value={draft.apiKey} onChangeText={(value) => updateDraft('apiKey', value)} placeholder={t('mobileConfiguration.apiKeyPlaceholder')} secureTextEntry style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /><TextInput accessibilityLabel={t('mobileConfiguration.appSecretLabel')} value={draft.appSecret} onChangeText={(value) => updateDraft('appSecret', value)} placeholder={t('mobileConfiguration.appSecretPlaceholder')} secureTextEntry style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /></> : null}{draft.section === 'mcp' ? <><TextInput accessibilityLabel={t('mobileConfiguration.mcpUrlLabel')} value={draft.url} onChangeText={(value) => updateDraft('url', value)} placeholder={t('mobileConfiguration.mcpUrlPlaceholder')} autoCapitalize="none" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /><Text style={{ marginBottom: 5 }}>{t('mobileConfiguration.transportLabel')}: {draft.transportType === 'sse' ? t('mobileConfiguration.transportSse') : draft.transportType === 'http-streamable' ? t('mobileConfiguration.transportHttp') : t('mobileConfiguration.transportStdio')}</Text><View style={{ flexDirection: 'row', gap: 8, marginBottom: 8 }}><Pressable onPress={() => updateDraft('transportType', 'sse')} style={{ backgroundColor: draft.transportType === 'sse' ? '#dbeafe' : '#f2f4f7', padding: 8, borderRadius: 8 }}><Text>{t('mobileConfiguration.transportSse')}</Text></Pressable><Pressable onPress={() => updateDraft('transportType', 'http-streamable')} style={{ backgroundColor: draft.transportType === 'http-streamable' ? '#dbeafe' : '#f2f4f7', padding: 8, borderRadius: 8 }}><Text>{t('mobileConfiguration.transportHttp')}</Text></Pressable><Pressable onPress={() => updateDraft('transportType', 'stdio')} style={{ backgroundColor: draft.transportType === 'stdio' ? '#dbeafe' : '#f2f4f7', padding: 8, borderRadius: 8 }}><Text>{t('mobileConfiguration.transportStdio')}</Text></Pressable></View><View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}><Text>{t('mobileConfiguration.enabledLabel')}</Text><Switch value={draft.enabled} onValueChange={(value) => updateDraft('enabled', value)} /></View><TextInput accessibilityLabel={t('mobileConfiguration.mcpApiKeyLabel')} value={draft.apiKey} onChangeText={(value) => updateDraft('apiKey', value)} placeholder={t('mobileConfiguration.apiKeyPlaceholder')} secureTextEntry style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /><TextInput accessibilityLabel={t('mobileConfiguration.mcpTokenLabel')} value={draft.token} onChangeText={(value) => updateDraft('token', value)} placeholder={t('mobileConfiguration.tokenPlaceholder')} secureTextEntry style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} /></> : null}<TextInput accessibilityLabel={t('mobileConfiguration.detailsLabel')} value={draft.details} onChangeText={(value) => updateDraft('details', value)} placeholder={t('mobileConfiguration.detailsPlaceholder')} multiline style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, minHeight: 100, marginBottom: 10, fontFamily: 'monospace' }} /><View style={{ flexDirection: 'row', gap: 12 }}><Pressable disabled={saving} onPress={() => void save()} style={{ backgroundColor: saving ? '#98a2b3' : '#2864dc', padding: 11, borderRadius: 8, flex: 1, alignItems: 'center' }}><Text style={{ color: '#fff', fontWeight: '600' }}>{saving ? t('mobileConfiguration.saving') : t('mobileConfiguration.save')}</Text></Pressable><Pressable disabled={saving} onPress={() => { setEditor(null); setDraft(null); }} style={{ padding: 11 }}><Text style={{ color: '#2864dc' }}>{t('mobileConfiguration.cancel')}</Text></Pressable></View></View> : null}
    </ScrollView>
  </SafeAreaView>;
}
