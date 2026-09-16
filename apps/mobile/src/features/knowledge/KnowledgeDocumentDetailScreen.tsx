import { useCallback, useEffect, useRef, useState } from 'react';
import { AppState, ActivityIndicator, Pressable, SafeAreaView, ScrollView, Text, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import type { KnowledgeDocument } from '@weknora/contracts';
import { useMobileRuntime } from '../../runtime.tsx';
import { downloadKnowledgeFile, readNativeTextFile, shareNativeFile } from '../../platform/files.ts';
import { NativeArtifactPreview } from '../chat/artifact-preview.tsx';
import { previewKindForFile, previewStatus } from '@weknora/domain/knowledge/preview';
import { selectKnowledgeDocumentLabel } from './parity.ts';
import { knowledgeListLabel } from './list.ts';

export function KnowledgeDocumentDetailScreen() {
  const { id: rawId } = useLocalSearchParams<{ id: string }>();
  const id = Array.isArray(rawId) ? rawId[0] : rawId;
  const runtime = useMobileRuntime();
  const router = useRouter();
  const [document, setDocument] = useState<KnowledgeDocument>();
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [preview, setPreview] = useState<{ loading: boolean; error?: string; uri?: string; content?: string } | null>(null);
  const loadGeneration = useRef(0);
  const previewGeneration = useRef(0);
  const label = useCallback((key: string, values: Record<string, string | number> = {}) => knowledgeListLabel(runtime.locale, key, values), [runtime.locale]);

  const load = useCallback(async () => {
    if (!id) return;
    const generation = ++loadGeneration.current;
    setLoading(true); setError('');
    try {
      const next = await runtime.client.knowledge.documents.get(id);
      if (generation === loadGeneration.current) setDocument(next);
    } catch (cause) {
      if (generation === loadGeneration.current) setError(cause instanceof Error ? cause.message : label('knowledgeBase.detail.loadFailed'));
    } finally {
      if (generation === loadGeneration.current) setLoading(false);
    }
  }, [id, label, runtime.client]);
  useEffect(() => { void load(); }, [load]);
  useEffect(() => { const subscription = AppState.addEventListener('change', (state) => { if (state === 'active') void load(); }); return () => subscription.remove(); }, [load]);

  async function downloadAndShare() {
    if (!document || !id) return;
    setBusy(true); setError('');
    try {
      const uri = await downloadKnowledgeFile({ baseURL: runtime.baseURL, path: runtime.client.knowledge.documents.downloadPath(id), fileName: document.file_name || document.title || 'download', credential: runtime.credential });
      await shareNativeFile(uri);
    } catch (cause) { setError(cause instanceof Error ? cause.message : label('knowledgeBase.detail.downloadShareFailed')); }
    finally { setBusy(false); }
  }

  async function openPreview() {
    if (!document || !id) return;
    const generation = ++previewGeneration.current;
    const status = previewStatus(document);
    if (status.kind !== 'ready') {
      const statusLabel = status.kind === 'processing'
        ? label('knowledgeBase.timeline.running')
        : label(document.parse_status ? 'knowledgeBase.timeline.failed' : 'knowledgeBase.documents.statusUnknown');
      if (generation === previewGeneration.current) setError(label('knowledgeBase.detail.previewUnavailable', { status: statusLabel }));
      return;
    }
    setBusy(true);
    setPreview({ loading: true });
    try {
      const uri = await downloadKnowledgeFile({ baseURL: runtime.baseURL, path: runtime.client.knowledge.documents.previewPath(id), fileName: document.file_name || document.title || 'preview', credential: runtime.credential });
      const kind = previewKindForFile(document.file_name || document.title || '');
      const content = kind === 'text' || kind === 'markdown' ? await readNativeTextFile(uri) : undefined;
      if (generation === previewGeneration.current) setPreview({ loading: false, uri, content });
    } catch (cause) {
      if (generation === previewGeneration.current) setPreview({ loading: false, error: cause instanceof Error ? cause.message : label('knowledgeBase.detail.previewFailed') });
    } finally {
      if (generation === previewGeneration.current) setBusy(false);
    }
  }

  return <SafeAreaView style={{ flex: 1 }}>
    <View style={{ flexDirection: 'row', alignItems: 'center', padding: 16, gap: 12 }}><Pressable onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>{label('knowledgeBase.detail.backShort')}</Text></Pressable><Text accessibilityRole="header" style={{ fontSize: 21, fontWeight: '700' }}>{label('knowledgeBase.detail.title')}</Text></View>
    {loading ? <ActivityIndicator accessibilityLabel={label('knowledgeBase.detail.loading')} /> : <ScrollView contentContainerStyle={{ padding: 16 }}>
      {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 12 }}>{error}</Text> : null}
      {document ? <>
        <Text style={{ fontSize: 24, fontWeight: '700', marginBottom: 8 }}>{selectKnowledgeDocumentLabel(document)}</Text>
        <Text style={{ color: '#667085', marginBottom: 16 }}>{document.parse_status || 'unknown'}{document.folder_path ? ` · ${document.folder_path}` : ''}</Text>
        <View style={{ flexDirection: 'row', gap: 8, marginBottom: 16 }}><Pressable accessibilityRole="button" disabled={busy} onPress={() => void openPreview()} style={{ backgroundColor: busy ? '#98a2b3' : '#eef4ff', padding: 12, borderRadius: 8, alignItems: 'center', flex: 1 }}><Text style={{ color: '#2864dc', fontWeight: '600' }}>{label('knowledgeBase.detail.preview')}</Text></Pressable><Pressable accessibilityRole="button" disabled={busy} onPress={() => void downloadAndShare()} style={{ backgroundColor: busy ? '#98a2b3' : '#2864dc', padding: 12, borderRadius: 8, alignItems: 'center', flex: 1 }}><Text style={{ color: '#fff', fontWeight: '600' }}>{busy ? label('knowledgeBase.detail.preparing') : label('knowledgeBase.detail.downloadShare')}</Text></Pressable></View>
        <Text selectable style={{ color: '#475467' }}>{label('knowledgeBase.detail.documentId')}: {document.id}</Text>
        {document.file_type ? <Text style={{ color: '#475467', marginTop: 6 }}>{label('knowledgeBase.detail.fileType')}: {document.file_type}</Text> : null}
        {document.file_size !== undefined ? <Text style={{ color: '#475467', marginTop: 6 }}>{label('knowledgeBase.detail.fileSize')}: {String(document.file_size)}</Text> : null}
      </> : null}
    </ScrollView>}
    {preview ? <NativeArtifactPreview artifact={{ fileName: document?.file_name || document?.title || 'preview', fileType: document?.file_type }} uri={preview.uri} content={preview.content} loading={preview.loading} error={preview.error} labels={{ back: label('knowledgeBase.detail.backShort'), share: label('common.share'), loading: label('knowledgeBase.detail.previewLoading'), downloadOnly: label('knowledgeBase.detail.downloadOnly') }} onClose={() => { previewGeneration.current += 1; setPreview(null); }} onDownload={preview.uri ? () => { void shareNativeFile(preview.uri!).catch((cause) => setPreview((current) => current ? { ...current, error: cause instanceof Error ? cause.message : label('knowledgeBase.detail.shareFailed') } : current)); } : undefined} /> : null}
  </SafeAreaView>;
}
