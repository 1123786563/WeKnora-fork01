import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { AppState, FlatList, Pressable, SafeAreaView, Text, TextInput, View, ActivityIndicator } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import type { KnowledgeDocument, KnowledgeFolderNode, KnowledgeTag } from '@weknora/contracts';
import { useMobileRuntime } from '../../runtime.tsx';
import { pickNativeFile } from '../../platform/files.ts';
import { selectKnowledgeDocumentLabel } from './parity.ts';
import { referenceRoute } from './reference.ts';

function flattenFolders(nodes: KnowledgeFolderNode[]): KnowledgeFolderNode[] {
  return nodes.flatMap((node) => [node, ...flattenFolders(node.children || [])]);
}

export function KnowledgeDocumentsScreen() {
  const { id: rawId } = useLocalSearchParams<{ id: string }>();
  const kbId = Array.isArray(rawId) ? rawId[0] : rawId;
  const runtime = useMobileRuntime();
  const router = useRouter();
  const [documents, setDocuments] = useState<KnowledgeDocument[]>([]);
  const [keyword, setKeyword] = useState('');
  const [folderPath, setFolderPath] = useState<string | undefined>();
  const [tagIds, setTagIds] = useState<string[]>([]);
  const [folders, setFolders] = useState<KnowledgeFolderNode[]>([]);
  const [tags, setTags] = useState<KnowledgeTag[]>([]);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState('');
  const requestGeneration = useRef(0);
  const uploadController = useRef<AbortController | null>(null);

  const loadAuxiliary = useCallback(async () => {
    if (!kbId) return;
    const [folderTree, tagList] = await Promise.all([
      runtime.client.knowledge.documents.folders(kbId),
      runtime.client.knowledge.documents.tags(kbId, { page: 1, page_size: 100 }),
    ]);
    setFolders(folderTree.folders);
    setTags(tagList);
  }, [kbId, runtime.client]);

  const loadPage = useCallback(async (nextPage: number, replace: boolean) => {
    if (!kbId) return;
    const generation = ++requestGeneration.current;
    if (replace) setLoading(true); else setLoadingMore(true);
    setError('');
    try {
      const result = await runtime.client.knowledge.documents.list(kbId, {
        page: nextPage, page_size: 20, keyword: keyword.trim() || undefined, folder_path: folderPath,
        folder_recursive: folderPath !== undefined,
        tag_ids: tagIds.length ? tagIds.join(',') : undefined,
      });
      if (generation !== requestGeneration.current) return;
      setDocuments((current) => replace ? result.data : [...current, ...result.data]);
      setTotal(result.total);
      setHasMore(result.page * result.page_size < result.total);
      setPage(nextPage);
    } catch (cause) {
      if (generation === requestGeneration.current) setError(cause instanceof Error ? cause.message : 'Unable to load documents');
    } finally {
      if (generation === requestGeneration.current) { setLoading(false); setLoadingMore(false); }
    }
  }, [folderPath, kbId, keyword, runtime.client, tagIds]);

  useEffect(() => { void loadAuxiliary().catch((cause) => setError(cause instanceof Error ? cause.message : 'Unable to load filters')); }, [loadAuxiliary]);
  useEffect(() => { void loadPage(1, true); }, [loadPage]);
  useEffect(() => {
    const subscription = AppState.addEventListener('change', (state) => { if (state === 'active') { void loadAuxiliary(); void loadPage(1, true); } });
    return () => subscription.remove();
  }, [loadAuxiliary, loadPage]);

  const folderOptions = useMemo(() => flattenFolders(folders), [folders]);
  async function upload() {
    if (!kbId || uploading) return;
    const file = await pickNativeFile();
    if (!file) return;
    const controller = new AbortController();
    uploadController.current = controller;
    setUploading(true); setError('');
    try { await runtime.client.knowledge.documents.upload(kbId, { file }, controller.signal); await loadPage(1, true); }
    catch (cause) { if (!controller.signal.aborted) setError(cause instanceof Error ? cause.message : 'Unable to upload file'); }
    finally { uploadController.current = null; setUploading(false); }
  }

  return <SafeAreaView style={{ flex: 1, padding: 16 }}>
    <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 10 }}>
      <Pressable accessibilityRole="button" onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>Back</Text></Pressable>
      <Text accessibilityRole="header" style={{ flex: 1, fontSize: 21, fontWeight: '700' }}>Files</Text>
      <Pressable accessibilityRole="button" disabled={!kbId} onPress={() => kbId && router.push(referenceRoute('wiki', kbId))}><Text style={{ color: '#2864dc' }}>Wiki</Text></Pressable>
      <Pressable accessibilityRole="button" disabled={!kbId} onPress={() => kbId && router.push(referenceRoute('faq', kbId))}><Text style={{ color: '#2864dc' }}>FAQ</Text></Pressable>
      <Pressable accessibilityRole="button" onPress={() => uploading ? uploadController.current?.abort() : void upload()}><Text style={{ color: '#2864dc' }}>{uploading ? 'Cancel' : 'Upload'}</Text></Pressable>
    </View>
    <TextInput accessibilityLabel="Search files" value={keyword} onChangeText={setKeyword} onSubmitEditing={() => void loadPage(1, true)} placeholder="Search files" returnKeyType="search" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} />
    <FlatList horizontal showsHorizontalScrollIndicator={false} data={[{ path: undefined, name: 'All folders' }, ...folderOptions]} keyExtractor={(item) => item.path || 'all'} renderItem={({ item }) => <Pressable onPress={() => setFolderPath(item.path)} style={{ paddingHorizontal: 10, paddingVertical: 7, borderRadius: 14, backgroundColor: folderPath === item.path ? '#dbeafe' : '#f2f4f7', marginRight: 6 }}><Text>{item.name}</Text></Pressable>} style={{ maxHeight: 42, marginBottom: 6 }} />
    <FlatList horizontal showsHorizontalScrollIndicator={false} data={tags} keyExtractor={(item) => item.id} renderItem={({ item }) => { const selected = tagIds.includes(item.id); return <Pressable onPress={() => setTagIds((current) => selected ? current.filter((id) => id !== item.id) : [...current, item.id])} style={{ paddingHorizontal: 10, paddingVertical: 7, borderRadius: 14, backgroundColor: selected ? '#dcfce7' : '#f2f4f7', marginRight: 6 }}><Text>{item.name}</Text></Pressable>; }} style={{ maxHeight: 42, marginBottom: 8 }} />
    <Text style={{ color: '#667085', marginBottom: 6 }}>{total} files</Text>
    {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 8 }}>{error}</Text> : null}
    {loading ? <ActivityIndicator accessibilityLabel="Loading files" /> : <FlatList
      data={documents}
      keyExtractor={(item) => item.id}
      onEndReachedThreshold={0.4}
      onEndReached={() => { if (!loadingMore && hasMore) void loadPage(page + 1, false); }}
      ListEmptyComponent={<Text style={{ color: '#667085' }}>No files match these filters.</Text>}
      ListFooterComponent={loadingMore ? <ActivityIndicator /> : null}
      renderItem={({ item }) => <Pressable accessibilityRole="button" onPress={() => router.push(`/knowledge/document/${item.id}`)} style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 12 }}>
        <Text style={{ fontWeight: '600' }}>{selectKnowledgeDocumentLabel(item)}</Text>
        <Text style={{ color: '#667085', fontSize: 12 }}>{item.parse_status || 'unknown'}{item.folder_path ? ` · ${item.folder_path}` : ''}</Text>
      </Pressable>}
    />}
  </SafeAreaView>;
}
