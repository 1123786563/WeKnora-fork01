import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, FlatList, Pressable, SafeAreaView, Text, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useMobileRuntime } from '../../runtime.tsx';
import { editorRoute, selectFaqReferenceLabel, selectWikiReferenceLabel, type KnowledgeReferenceKind } from './reference.ts';

interface ReferenceRow { id: string; label: string; detail: string; }

export function KnowledgeReferenceScreen({ kind }: { kind: KnowledgeReferenceKind }) {
  const { id: rawId } = useLocalSearchParams<{ id: string }>();
  const kbId = Array.isArray(rawId) ? rawId[0] : rawId;
  const runtime = useMobileRuntime();
  const router = useRouter();
  const role = runtime.workspaces.find((workspace) => String(workspace.id) === runtime.tenantId)?.role.trim().toLowerCase();
  const writable = role === 'owner' || role === 'admin';
  const [rows, setRows] = useState<ReferenceRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    if (!kbId) return;
    setLoading(true);
    setError('');
    try {
      if (kind === 'wiki') {
        const response = await runtime.client.wiki.list(kbId, { page: 1, page_size: 100 });
        setRows(response.pages.map((page) => ({ id: page.id, label: selectWikiReferenceLabel(page), detail: page.summary || 'Read-only Wiki page' })));
      } else {
        const response = await runtime.client.knowledge.faq.list(kbId, { page: 1, page_size: 100 });
        setRows(response.data.map((entry) => ({ id: String(entry.id), label: selectFaqReferenceLabel(entry), detail: entry.answers[0] || 'Read-only FAQ entry' })));
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : `Unable to load ${kind}`);
    } finally {
      setLoading(false);
    }
  }, [kbId, kind, runtime.client]);

  useEffect(() => { void load(); }, [load]);
  const title = kind === 'wiki' ? 'Wiki' : 'FAQ';
  return <SafeAreaView style={{ flex: 1, padding: 16 }}>
    <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 10 }}>
      <Pressable accessibilityRole="button" onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>Back</Text></Pressable>
      <Text accessibilityRole="header" style={{ flex: 1, fontSize: 21, fontWeight: '700' }}>{title}</Text>
      {writable ? <Pressable accessibilityRole="button" onPress={() => kbId && router.push(editorRoute(kind, kbId))}><Text style={{ color: '#2864dc' }}>New</Text></Pressable> : null}
      <Pressable accessibilityRole="button" onPress={() => void load()}><Text style={{ color: '#2864dc' }}>Refresh</Text></Pressable>
    </View>
    <Text style={{ color: '#667085', marginBottom: 8 }}>{writable ? 'Owner/admin editing · server permissions remain authoritative' : 'Read-only on mobile for this workspace role'} · knowledge base {kbId || 'unknown'}</Text>
    {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 8 }}>{error}</Text> : null}
    {loading ? <ActivityIndicator accessibilityLabel={`Loading ${kind}`} /> : <FlatList
      data={rows}
      keyExtractor={(item) => item.id}
      ListEmptyComponent={<Text style={{ color: '#667085' }}>No {kind} entries available.</Text>}
      renderItem={({ item }) => <View style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 12 }}><View style={{ flexDirection: 'row', alignItems: 'center', gap: 10 }}><Text style={{ flex: 1, fontWeight: '600' }}>{item.label}</Text>{writable ? <Pressable accessibilityRole="button" onPress={() => router.push(editorRoute(kind, kbId || '', item.id))}><Text style={{ color: '#2864dc' }}>Edit</Text></Pressable> : null}</View><Text style={{ color: '#667085', fontSize: 12, marginTop: 4 }}>{item.detail}</Text></View>}
    />}
  </SafeAreaView>;
}

export function WikiReferenceScreen() { return <KnowledgeReferenceScreen kind="wiki" />; }
export function FaqReferenceScreen() { return <KnowledgeReferenceScreen kind="faq" />; }
