import { useCallback, useEffect, useState } from 'react';
import { AppState, ActivityIndicator, Pressable, SafeAreaView, ScrollView, Text, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import type { KnowledgeDocument } from '@weknora/contracts';
import { useMobileRuntime } from '../../runtime.tsx';
import { downloadKnowledgeFile, shareNativeFile } from '../../platform/files.ts';
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

  return <SafeAreaView style={{ flex: 1 }}>
    <View style={{ flexDirection: 'row', alignItems: 'center', padding: 16, gap: 12 }}><Pressable onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>Back</Text></Pressable><Text accessibilityRole="header" style={{ fontSize: 21, fontWeight: '700' }}>File details</Text></View>
    {loading ? <ActivityIndicator accessibilityLabel="Loading file details" /> : <ScrollView contentContainerStyle={{ padding: 16 }}>
      {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 12 }}>{error}</Text> : null}
      {document ? <>
        <Text style={{ fontSize: 24, fontWeight: '700', marginBottom: 8 }}>{selectKnowledgeDocumentLabel(document)}</Text>
        <Text style={{ color: '#667085', marginBottom: 16 }}>{document.parse_status || 'unknown'}{document.folder_path ? ` · ${document.folder_path}` : ''}</Text>
        <Pressable accessibilityRole="button" disabled={busy} onPress={() => void downloadAndShare()} style={{ backgroundColor: busy ? '#98a2b3' : '#2864dc', padding: 12, borderRadius: 8, alignItems: 'center', marginBottom: 16 }}><Text style={{ color: '#fff', fontWeight: '600' }}>{busy ? 'Preparing…' : 'Download and share'}</Text></Pressable>
        <Text selectable style={{ color: '#475467' }}>Document ID: {document.id}</Text>
        {document.file_type ? <Text style={{ color: '#475467', marginTop: 6 }}>Type: {document.file_type}</Text> : null}
        {document.file_size !== undefined ? <Text style={{ color: '#475467', marginTop: 6 }}>Size: {String(document.file_size)}</Text> : null}
      </> : null}
    </ScrollView>}
  </SafeAreaView>;
}
