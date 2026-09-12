import { useCallback, useEffect, useState } from 'react';
import { AppState, ActivityIndicator, Pressable, SafeAreaView, ScrollView, Text, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import type { KnowledgeDocument } from '@weknora/contracts';
import { useMobileRuntime } from '../../runtime.tsx';
import { downloadKnowledgeFile, readNativeTextFile, shareNativeFile } from '../../platform/files.ts';
import { NativeArtifactPreview } from '../chat/artifact-preview.tsx';
import { previewKindForFile, previewStatus } from '@weknora/domain/knowledge/preview';
import { selectKnowledgeDocumentLabel } from './parity.ts';

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

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true); setError('');
    try { setDocument(await runtime.client.knowledge.documents.get(id)); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to load file details'); }
    finally { setLoading(false); }
  }, [id, runtime.client]);
  useEffect(() => { void load(); }, [load]);
  useEffect(() => { const subscription = AppState.addEventListener('change', (state) => { if (state === 'active') void load(); }); return () => subscription.remove(); }, [load]);

  async function downloadAndShare() {
    if (!document || !id) return;
    setBusy(true); setError('');
    try {
      const uri = await downloadKnowledgeFile({ baseURL: runtime.baseURL, path: runtime.client.knowledge.documents.downloadPath(id), fileName: document.file_name || document.title || 'download', credential: runtime.credential });
      await shareNativeFile(uri);
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to download or share file'); }
    finally { setBusy(false); }
  }

  async function openPreview() {
    if (!document || !id) return;
    const status = previewStatus(document);
    if (status.kind !== 'ready') { setError(`Preview unavailable while the document is ${status.label.toLowerCase()}.`); return; }
    setPreview({ loading: true });
    try {
      const uri = await downloadKnowledgeFile({ baseURL: runtime.baseURL, path: runtime.client.knowledge.documents.previewPath(id), fileName: document.file_name || document.title || 'preview', credential: runtime.credential });
      const kind = previewKindForFile(document.file_name || document.title || '');
      const content = kind === 'text' || kind === 'markdown' ? await readNativeTextFile(uri) : undefined;
      setPreview({ loading: false, uri, content });
    } catch (cause) { setPreview({ loading: false, error: cause instanceof Error ? cause.message : 'Unable to load preview' }); }
  }

  return <SafeAreaView style={{ flex: 1 }}>
    <View style={{ flexDirection: 'row', alignItems: 'center', padding: 16, gap: 12 }}><Pressable onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>Back</Text></Pressable><Text accessibilityRole="header" style={{ fontSize: 21, fontWeight: '700' }}>File details</Text></View>
    {loading ? <ActivityIndicator accessibilityLabel="Loading file details" /> : <ScrollView contentContainerStyle={{ padding: 16 }}>
      {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 12 }}>{error}</Text> : null}
      {document ? <>
        <Text style={{ fontSize: 24, fontWeight: '700', marginBottom: 8 }}>{selectKnowledgeDocumentLabel(document)}</Text>
        <Text style={{ color: '#667085', marginBottom: 16 }}>{document.parse_status || 'unknown'}{document.folder_path ? ` · ${document.folder_path}` : ''}</Text>
        <View style={{ flexDirection: 'row', gap: 8, marginBottom: 16 }}><Pressable accessibilityRole="button" disabled={busy} onPress={() => void openPreview()} style={{ backgroundColor: busy ? '#98a2b3' : '#eef4ff', padding: 12, borderRadius: 8, alignItems: 'center', flex: 1 }}><Text style={{ color: '#2864dc', fontWeight: '600' }}>Preview</Text></Pressable><Pressable accessibilityRole="button" disabled={busy} onPress={() => void downloadAndShare()} style={{ backgroundColor: busy ? '#98a2b3' : '#2864dc', padding: 12, borderRadius: 8, alignItems: 'center', flex: 1 }}><Text style={{ color: '#fff', fontWeight: '600' }}>{busy ? 'Preparing…' : 'Download and share'}</Text></Pressable></View>
        <Text selectable style={{ color: '#475467' }}>Document ID: {document.id}</Text>
        {document.file_type ? <Text style={{ color: '#475467', marginTop: 6 }}>Type: {document.file_type}</Text> : null}
        {document.file_size !== undefined ? <Text style={{ color: '#475467', marginTop: 6 }}>Size: {String(document.file_size)}</Text> : null}
      </> : null}
    </ScrollView>}
    {preview ? <NativeArtifactPreview artifact={{ fileName: document?.file_name || document?.title || 'preview', fileType: document?.file_type }} uri={preview.uri} content={preview.content} loading={preview.loading} error={preview.error} onClose={() => setPreview(null)} onDownload={preview.uri ? () => { void shareNativeFile(preview.uri!).catch((cause) => setPreview((current) => current ? { ...current, error: cause instanceof Error ? cause.message : 'Unable to share preview' } : current)); } : undefined} /> : null}
  </SafeAreaView>;
}
