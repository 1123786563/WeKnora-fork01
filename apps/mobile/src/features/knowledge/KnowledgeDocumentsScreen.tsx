import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { AppState, FlatList, Pressable, SafeAreaView, Text, TextInput, View, ActivityIndicator } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import type { KnowledgeDocument, KnowledgeFolderNode, KnowledgeTag } from '@weknora/contracts';
import { useMobileRuntime } from '../../runtime.tsx';
import { pickNativeFiles } from '../../platform/files.ts';
import { dispatchUploadEvent } from './upload-progress.ts';
import { uploadKnowledgeFiles } from './upload-queue.ts';
import { knowledgeListLabel } from './list.ts';
import { selectKnowledgeDocumentLabel, shouldUseRecursiveFolderScope } from './parity.ts';
import { referenceRoute } from './reference.ts';
import { canManageKnowledgeBase, canMutateKnowledge, canOpenKnowledgeGraph, knowledgeBaseCapabilities } from './access.ts';

function flattenFolders(nodes: KnowledgeFolderNode[]): KnowledgeFolderNode[] {
  return nodes.flatMap((node) => [node, ...flattenFolders(node.children || [])]);
}

export function KnowledgeDocumentsScreen() {
  const { id: rawId } = useLocalSearchParams<{ id: string }>();
  const kbId = Array.isArray(rawId) ? rawId[0] : rawId;
  const runtime = useMobileRuntime();
  const router = useRouter();
  const label = (key: string, values: Record<string, string | number> = {}) => knowledgeListLabel(runtime.locale, key, values);
  const [knowledgeBase, setKnowledgeBase] = useState<Record<string, unknown> | null>(null);
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
  const auxiliaryGeneration = useRef(0);
  const uploadController = useRef<AbortController | null>(null);

  const loadAuxiliary = useCallback(async () => {
    if (!kbId) return;
    const generation = ++auxiliaryGeneration.current;
    try {
      const [folderTree, tagList, knowledgeBase] = await Promise.all([
        runtime.client.knowledge.documents.folders(kbId),
        runtime.client.knowledge.documents.tags(kbId, { page: 1, page_size: 100 }),
        runtime.client.knowledge.settings?.get?.(kbId) ?? Promise.resolve(null),
      ]);
      if (generation !== auxiliaryGeneration.current) return;
      setFolders(folderTree.folders);
      setTags(tagList);
      setKnowledgeBase((knowledgeBase as Record<string, unknown> | null) ?? null);
    } catch (cause) {
      if (generation === auxiliaryGeneration.current) setError(cause instanceof Error ? cause.message : label('knowledgeBase.documents.filtersLoadFailed'));
    }
  }, [kbId, runtime.client]);

  const loadPage = useCallback(async (nextPage: number, replace: boolean) => {
    if (!kbId) return;
    const generation = ++requestGeneration.current;
    if (replace) setLoading(true); else setLoadingMore(true);
    setError('');
    try {
      const result = await runtime.client.knowledge.documents.list(kbId, {
        page: nextPage, page_size: 20, keyword: keyword.trim() || undefined, folder_path: folderPath,
        folder_recursive: shouldUseRecursiveFolderScope({ folderPath, keyword, tagIds }),
        tag_ids: tagIds.length ? tagIds.join(',') : undefined,
      });
      if (generation !== requestGeneration.current) return;
      setDocuments((current) => replace ? result.data : [...current, ...result.data]);
      setTotal(result.total);
      setHasMore(result.page * result.page_size < result.total);
      setPage(nextPage);
    } catch (cause) {
      if (generation === requestGeneration.current) setError(cause instanceof Error ? cause.message : label('knowledgeBase.documents.loadFailed'));
    } finally {
      if (generation === requestGeneration.current) { setLoading(false); setLoadingMore(false); }
    }
  }, [folderPath, kbId, keyword, runtime.client, tagIds]);

  useEffect(() => { void loadAuxiliary(); }, [loadAuxiliary]);
  useEffect(() => { void loadPage(1, true); }, [loadPage]);
  useEffect(() => {
    const subscription = AppState.addEventListener('change', (state) => { if (state === 'active') { void loadAuxiliary(); void loadPage(1, true); } });
    return () => subscription.remove();
  }, [loadAuxiliary, loadPage]);

  const folderOptions = useMemo(() => flattenFolders(folders), [folders]);
  const capabilities = knowledgeBaseCapabilities(knowledgeBase ?? {});
  const workspaceRole = runtime.workspaces.find((workspace) => String(workspace.id) === runtime.tenantId)?.role;
  const permission = knowledgeBase?.my_permission ?? knowledgeBase?.permission;
  const viaShare = knowledgeBase?.isMine === false || knowledgeBase?.is_mine === false;
  const canUpload = canMutateKnowledge({ permission, viaShare, workspaceRole });
  async function upload() {
    if (!kbId || uploading) return;
    const files = await pickNativeFiles();
    if (files.length === 0) return;
    const controller = new AbortController();
    uploadController.current = controller;
    setUploading(true); setError('');
    try {
      // Vue emits one task per selected file. The existing API is intentionally
      // single-file, so keep the same semantics with a cancellable FIFO queue.
      const result = await uploadKnowledgeFiles(files, kbId, {
        signal: controller.signal,
        locale: runtime.locale,
        upload: (file, signal, onProgress) => runtime.client.knowledge.documents.upload(kbId, { file, onProgress }, signal),
        dispatch: dispatchUploadEvent,
      });
      if (result.succeeded > 0) {
        dispatchUploadEvent({ type: 'uploaded', kbId });
        await loadPage(1, true);
      }
      if (result.failures.length > 0 && !result.aborted) {
        const successCount = result.succeeded;
        const failureCount = result.failures.length;
        setError(successCount === 0
          ? knowledgeListLabel(runtime.locale, 'knowledgeBase.uploadAllFailed')
          : knowledgeListLabel(runtime.locale, 'knowledgeBase.uploadPartialSuccess', { success: successCount, fail: failureCount }));
      }
    }
    catch (cause) {
      if (!controller.signal.aborted) setError(cause instanceof Error ? cause.message : label('knowledgeBase.uploadFailed'));
    }
    finally { uploadController.current = null; setUploading(false); }
  }

  return <SafeAreaView style={{ flex: 1, padding: 16 }}>
    <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 10 }}>
      <Pressable accessibilityRole="button" onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>{label("knowledgeBase.detail.back")}</Text></Pressable>
      <Text accessibilityRole="header" style={{ flex: 1, fontSize: 21, fontWeight: '700' }}>{label("knowledgeBase.documents.title")}</Text>
      {!capabilities.isFaq && capabilities.wikiEnabled ? <Pressable accessibilityRole="button" disabled={!kbId} onPress={() => kbId && router.push(referenceRoute('wiki', kbId))}><Text style={{ color: '#2864dc' }}>{label("knowledgeBase.documents.tabWiki")}</Text></Pressable> : null}
      {capabilities.isFaq ? <Pressable accessibilityRole="button" disabled={!kbId} onPress={() => kbId && router.push(referenceRoute('faq', kbId))}><Text style={{ color: '#2864dc' }}>{label("knowledgeBase.faq.title")}</Text></Pressable> : null}
      {canOpenKnowledgeGraph(knowledgeBase ?? {}) ? <Pressable accessibilityRole="button" disabled={!kbId} onPress={() => kbId && router.push(`/knowledge/${encodeURIComponent(kbId)}/graph`)}><Text style={{ color: '#2864dc' }}>{label("knowledgeBase.documents.tabGraph")}</Text></Pressable> : null}
      {canManageKnowledgeBase({ permission, viaShare, workspaceRole }) ? <Pressable accessibilityRole="button" disabled={!kbId} onPress={() => kbId && router.push(`/knowledge/${encodeURIComponent(kbId)}/settings`)}><Text style={{ color: '#2864dc' }}>{label('knowledgeBase.settings')}</Text></Pressable> : null}
      <Pressable accessibilityRole="button" disabled={!kbId} onPress={() => kbId && router.push(`/knowledge/${encodeURIComponent(kbId)}/data-sources`)}><Text style={{ color: '#2864dc' }}>{label("dataSource.title")}</Text></Pressable>
      {canUpload ? <Pressable accessibilityRole="button" onPress={() => uploading ? uploadController.current?.abort() : void upload()}><Text style={{ color: '#2864dc' }}>{uploading ? label("common.cancel") : label("knowledgeBase.documents.uploadFile")}</Text></Pressable> : null}
    </View>
    <TextInput accessibilityLabel={label("knowledgeBase.documents.searchPlaceholder")} value={keyword} onChangeText={setKeyword} onSubmitEditing={() => void loadPage(1, true)} placeholder={label("knowledgeBase.documents.searchPlaceholder")} returnKeyType="search" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} />
    <FlatList horizontal showsHorizontalScrollIndicator={false} data={[{ path: undefined, name: label("knowledgeBase.documents.root") }, ...folderOptions]} keyExtractor={(item) => item.path || 'all'} renderItem={({ item }) => <Pressable accessibilityRole="button" accessibilityState={{ selected: folderPath === item.path }} onPress={() => setFolderPath(item.path)} style={{ paddingHorizontal: 10, paddingVertical: 7, borderRadius: 14, backgroundColor: folderPath === item.path ? '#dbeafe' : '#f2f4f7', marginRight: 6 }}><Text>{item.name}</Text></Pressable>} style={{ maxHeight: 42, marginBottom: 6 }} />
    <FlatList horizontal showsHorizontalScrollIndicator={false} data={tags} keyExtractor={(item) => item.id} renderItem={({ item }) => { const selected = tagIds.includes(item.id); return <Pressable accessibilityRole="button" accessibilityLabel={item.name} accessibilityState={{ selected }} onPress={() => setTagIds((current) => selected ? current.filter((id) => id !== item.id) : [...current, item.id])} style={{ paddingHorizontal: 10, paddingVertical: 7, borderRadius: 14, backgroundColor: selected ? '#dcfce7' : '#f2f4f7', marginRight: 6 }}><Text>{item.name}</Text></Pressable>; }} style={{ maxHeight: 42, marginBottom: 8 }} />
    <Text style={{ color: '#667085', marginBottom: 6 }}>{label("common.itemCount", { count: total })}</Text>
    {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 8 }}>{error}</Text> : null}
    {loading ? <ActivityIndicator accessibilityLabel={label("common.loading")} /> : <FlatList
      data={documents}
      keyExtractor={(item) => item.id}
      onEndReachedThreshold={0.4}
      onEndReached={() => { if (!loadingMore && hasMore) void loadPage(page + 1, false); }}
      ListEmptyComponent={<Text style={{ color: '#667085' }}>{label("knowledgeBase.documents.noDocuments")}</Text>}
      ListFooterComponent={loadingMore ? <ActivityIndicator /> : null}
      renderItem={({ item }) => <Pressable accessibilityRole="button" onPress={() => router.push(`/knowledge/document/${item.id}`)} style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 12 }}>
        <Text style={{ fontWeight: '600' }}>{selectKnowledgeDocumentLabel(item)}</Text>
        <Text style={{ color: '#667085', fontSize: 12 }}>{item.parse_status || label("knowledgeBase.documents.statusUnknown")}{item.folder_path ? ` · ${item.folder_path}` : ''}</Text>
      </Pressable>}
    />}
  </SafeAreaView>;
}
