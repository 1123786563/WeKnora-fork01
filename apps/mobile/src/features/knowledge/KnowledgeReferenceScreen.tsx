import { useCallback, useEffect, useRef, useState } from 'react';
import { ActivityIndicator, FlatList, Pressable, SafeAreaView, Text, TextInput, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useMobileRuntime } from '../../runtime.tsx';
import { knowledgeListLabel } from './list.ts';
import { editorRoute, faqReferenceListParams, selectFaqReferenceEditKey, selectFaqReferenceLabel, selectWikiReferenceEditKey, selectWikiReferenceLabel, type KnowledgeReferenceKind } from './reference.ts';

interface ReferenceRow { id: string; label: string; detail: string; }

function referenceError(cause: unknown, kind: KnowledgeReferenceKind, label: (key: string) => string): string {
  const message = cause instanceof Error ? cause.message : '';
  if (/feature is not enabled/i.test(message)) {
    return label(kind === 'wiki' ? 'knowledgeEditor.mobile.wikiDisabled' : 'knowledgeEditor.mobile.faqDisabled');
  }
  return message || label(kind === 'wiki' ? 'knowledgeEditor.mobile.loadWikiFailed' : 'knowledgeEditor.mobile.loadFaqFailed');
}

export function KnowledgeReferenceScreen({ kind }: { kind: KnowledgeReferenceKind }) {
  const { id: rawId } = useLocalSearchParams<{ id: string }>();
  const kbId = Array.isArray(rawId) ? rawId[0] : rawId;
  const runtime = useMobileRuntime();
  const router = useRouter();
  const label = (key: string, values: Record<string, string | number> = {}) => knowledgeListLabel(runtime.locale, key, values);
  const role = runtime.workspaces.find((workspace) => String(workspace.id) === runtime.tenantId)?.role.trim().toLowerCase();
  const writable = role === 'owner' || role === 'admin';
  const [rows, setRows] = useState<ReferenceRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [faqSearchDraft, setFaqSearchDraft] = useState('');
  const [faqSearchKeyword, setFaqSearchKeyword] = useState('');
  const loadGeneration = useRef(0);

  const load = useCallback(async () => {
    if (!kbId) return;
    const generation = ++loadGeneration.current;
    setLoading(true);
    setError('');
    try {
      if (kind === 'wiki') {
        const response = await runtime.client.wiki.list(kbId, { page: 1, page_size: 100 });
        if (generation !== loadGeneration.current) return;
        setRows(response.pages.map((page) => ({ id: selectWikiReferenceEditKey(page), label: selectWikiReferenceLabel(page, { untitled: label('dataSource.untitled'), version: (version) => label('knowledgeEditor.wikiBrowser.version', { ver: version }) }), detail: page.summary || '' })));
      } else {
        const response = await runtime.client.knowledge.faq.list(kbId, faqReferenceListParams(faqSearchKeyword));
        if (generation !== loadGeneration.current) return;
        setRows(response.data.map((entry) => ({ id: selectFaqReferenceEditKey(entry), label: selectFaqReferenceLabel(entry, { untitled: label('dataSource.untitled'), enabled: label('knowledgeEditor.faq.statusEnabled'), disabled: label('knowledgeEditor.faq.statusDisabled'), recommended: label('knowledgeEditor.faq.recommended') }), detail: entry.answers[0] || '' })));
      }
    } catch (cause) {
      if (generation === loadGeneration.current) setError(referenceError(cause, kind, label));
    } finally {
      if (generation === loadGeneration.current) setLoading(false);
    }
  }, [faqSearchKeyword, kbId, kind, runtime.client, runtime.locale]);

  useEffect(() => { void load(); }, [load]);
  const title = kind === 'wiki' ? label("wikiBrowser.indexTitle") : label("knowledgeBase.faq.title");
  return <SafeAreaView style={{ flex: 1, padding: 16 }}>
    <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 10 }}>
      <Pressable accessibilityRole="button" onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>{label("knowledgeBase.detail.back")}</Text></Pressable>
      <Text accessibilityRole="header" style={{ flex: 1, fontSize: 21, fontWeight: '700' }}>{title}</Text>
      {writable ? <Pressable accessibilityRole="button" onPress={() => kbId && router.push(editorRoute(kind, kbId))}><Text style={{ color: '#2864dc' }}>{kind === 'wiki' ? label("wikiBrowser.newPageBtn") : label("knowledgeBase.faq.new")}</Text></Pressable> : null}
      <Pressable accessibilityRole="button" onPress={() => void load()}><Text style={{ color: '#2864dc' }}>{label("knowledgeBase.documents.reload")}</Text></Pressable>
    </View>
    {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 8 }}>{error}</Text> : null}
    {kind === 'faq' ? <TextInput
      accessibilityLabel={label('knowledgeEditor.faq.searchPlaceholder')}
      placeholder={label('knowledgeEditor.faq.searchPlaceholder')}
      value={faqSearchDraft}
      onChangeText={setFaqSearchDraft}
      onSubmitEditing={() => setFaqSearchKeyword(faqSearchDraft)}
      returnKeyType="search"
      style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 10 }}
    /> : null}
    {loading ? <ActivityIndicator accessibilityLabel={`${label('common.loading')}: ${title}`} /> : <FlatList
      data={rows}
      keyExtractor={(item) => item.id}
      ListEmptyComponent={<Text style={{ color: '#667085' }}>{kind === 'wiki' ? label("wikiBrowser.emptyTitle") : label("knowledgeEditor.faq.emptyTitle")}</Text>}
      renderItem={({ item }) => <View style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 12 }}><View style={{ flexDirection: 'row', alignItems: 'center', gap: 10 }}><Text style={{ flex: 1, fontWeight: '600' }}>{item.label}</Text>{writable ? <Pressable accessibilityRole="button" onPress={() => router.push(editorRoute(kind, kbId || '', item.id))}><Text style={{ color: '#2864dc' }}>{label(kind === 'wiki' ? 'wikiBrowser.editBtn' : 'knowledgeBase.faq.edit')}</Text></Pressable> : null}</View>{item.detail ? <Text style={{ color: '#667085', fontSize: 12, marginTop: 4 }}>{item.detail}</Text> : null}</View>}
    />}
  </SafeAreaView>;
}

export function WikiReferenceScreen() { return <KnowledgeReferenceScreen kind="wiki" />; }
export function FaqReferenceScreen() { return <KnowledgeReferenceScreen kind="faq" />; }
