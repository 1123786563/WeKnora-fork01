import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, SafeAreaView, ScrollView, Switch, Text, TextInput, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useMobileRuntime } from '../../runtime.tsx';
import { canManageKnowledgeBase } from './access.ts';
import { knowledgeListLabel } from './list.ts';
import { knowledgeBaseSettingsDraft, validateKnowledgeBaseSettings, type KnowledgeBaseSettingsDraft } from './settings.ts';

export function KnowledgeBaseSettingsScreen() {
  const { id: rawId } = useLocalSearchParams<{ id: string }>();
  const id = Array.isArray(rawId) ? rawId[0] : rawId;
  const runtime = useMobileRuntime();
  const router = useRouter();
  const label = useCallback((key: string) => knowledgeListLabel(runtime.locale, key), [runtime.locale]);
  const role = runtime.workspaces.find((workspace) => String(workspace.id) === runtime.tenantId)?.role;
  const [draft, setDraft] = useState<KnowledgeBaseSettingsDraft>({ name: '', description: '', wikiEnabled: false, graphEnabled: false });
  const [permission, setPermission] = useState<unknown>();
  const [viaShare, setViaShare] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [saved, setSaved] = useState(false);
  const canManage = canManageKnowledgeBase({ permission, viaShare, workspaceRole: role });
  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true); setError('');
    try {
      const record = await runtime.client.knowledge.settings.get(id) as Record<string, unknown>;
      setDraft(knowledgeBaseSettingsDraft(record));
      setPermission(record.my_permission ?? record.permission);
      setViaShare(record.isMine === false || record.is_mine === false);
    } catch (cause) { setError(cause instanceof Error ? cause.message : label('knowledgeList.loadFailed')); }
    finally { setLoading(false); }
  }, [id, label, runtime.client]);
  useEffect(() => { void load(); }, [load]);
  async function save() {
    const validation = validateKnowledgeBaseSettings(draft);
    if (validation) { setError(label(validation)); return; }
    if (!canManage) { setError(label('dataSource.permissionRequired')); return; }
    setSaving(true); setError(''); setSaved(false);
    try {
      await runtime.client.knowledge.settings.update(id!, { name: draft.name.trim(), description: draft.description.trim(), config: { indexing_strategy: { vector_enabled: true, keyword_enabled: true, wiki_enabled: draft.wikiEnabled, graph_enabled: draft.graphEnabled } } });
      setSaved(true);
    } catch (cause) { setError(cause instanceof Error ? cause.message : label('common.operationFailed')); }
    finally { setSaving(false); }
  }
  return <SafeAreaView style={{ flex: 1, padding: 16 }}><View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 12 }}><Pressable accessibilityRole="button" onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>{label('knowledgeBase.detail.back')}</Text></Pressable><Text accessibilityRole="header" style={{ flex: 1, fontSize: 21, fontWeight: '700' }}>{label('knowledgeBase.settings.title')}</Text>{canManage ? <Pressable accessibilityRole="button" disabled={saving || loading} onPress={() => void save()}><Text style={{ color: '#2864dc', opacity: saving ? 0.5 : 1 }}>{saving ? label('common.loading') : label('common.save')}</Text></Pressable> : null}</View>{!canManage ? <Text style={{ color: '#667085', marginBottom: 10 }}>{label('knowledgeEditor.mobile.editPermission')}</Text> : null}{error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 10 }}>{error}</Text> : null}{loading ? <ActivityIndicator accessibilityLabel={label('common.loading')} /> : <ScrollView keyboardShouldPersistTaps="handled"><Text style={{ fontWeight: '600', marginBottom: 4 }}>{label('knowledgeBase.name')}</Text><TextInput value={draft.name} onChangeText={(name) => setDraft((current) => ({ ...current, name }))} editable={canManage} style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 10 }} /><Text style={{ fontWeight: '600', marginBottom: 4 }}>{label('knowledgeBase.description')}</Text><TextInput value={draft.description} onChangeText={(description) => setDraft((current) => ({ ...current, description }))} editable={canManage} multiline style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, minHeight: 80, marginBottom: 12 }} /><View style={{ flexDirection: 'row', justifyContent: 'space-between', paddingVertical: 10 }}><Text>{label('knowledgeEditor.wikiBrowser.tabWiki')}</Text><Switch value={draft.wikiEnabled} onValueChange={(wikiEnabled) => setDraft((current) => ({ ...current, wikiEnabled }))} disabled={!canManage} /></View><View style={{ flexDirection: 'row', justifyContent: 'space-between', paddingVertical: 10 }}><Text>{label('knowledgeEditor.wikiBrowser.tabGraph')}</Text><Switch value={draft.graphEnabled} onValueChange={(graphEnabled) => setDraft((current) => ({ ...current, graphEnabled }))} disabled={!canManage} /></View>{saved ? <Text accessibilityLiveRegion="polite" style={{ color: '#067647', marginTop: 12 }}>{label('knowledgeEditor.mobile.saved')}</Text> : null}</ScrollView>}</SafeAreaView>;
}
