import { Ionicons } from '@expo/vector-icons';
import * as FileSystem from 'expo-file-system/legacy';
import * as Sharing from 'expo-sharing';
import { Stack, useRouter } from 'expo-router';
import * as React from 'react';
import { ActivityIndicator, FlatList, Pressable, Text, View } from 'react-native';
import { useMobileHost } from '@/weknora/platform/host';
import { useProductAuth } from '@/weknora/auth/session';
import { createMobileKnowledgeApi } from './api';
import { bytesToBase64 } from './file';
import { documentTitle } from './model';

export default function KnowledgeScreen() {
  const host = useMobileHost();
  const auth = useProductAuth();
  const router = useRouter();
  const [bases, setBases] = React.useState<Array<{ id: string; name: string; type?: string }>>([]);
  const [state, setState] = React.useState<'loading' | 'error' | 'ready'>('loading');
  const [error, setError] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!host || !auth.credential || auth.credential.kind !== 'bearer') return;
    let active = true;
    void createMobileKnowledgeApi(host, auth.credential).listBases().then((value) => {
      if (active) { setBases(value); setState('ready'); }
    }).catch((cause) => {
      if (active) { setError(cause instanceof Error ? cause.message : 'Could not load knowledge bases'); setState('error'); }
    });
    return () => { active = false; };
  }, [auth.credential, host]);

  return <View style={styles.container}>
    <Stack.Screen options={{ title: 'Knowledge' }} />
    {state === 'loading' ? <ActivityIndicator accessibilityLabel="Loading knowledge bases" /> : null}
    {state === 'error' ? <StateMessage message={error ?? 'Could not load knowledge bases'} retry={() => setState('loading')} /> : null}
    {state === 'ready' && bases.length === 0 ? <Text style={styles.muted}>No knowledge bases found.</Text> : null}
    <FlatList data={bases} keyExtractor={(item) => item.id} renderItem={({ item }) => <Pressable accessibilityRole="button" style={styles.row} onPress={() => router.push(`/knowledge/${encodeURIComponent(item.id)}`)}>
      <Ionicons name="library-outline" size={24} color="#53708a" /><View><Text style={styles.title}>{item.name}</Text><Text style={styles.muted}>{item.type ?? 'document'}</Text></View>
    </Pressable>} />
  </View>;
}

export function KnowledgeDetailScreen({ kbId }: { kbId: string }) {
  const host = useMobileHost();
  const auth = useProductAuth();
  const router = useRouter();
  const [documents, setDocuments] = React.useState<any[]>([]);
  const [state, setState] = React.useState<'loading' | 'forbidden' | 'error' | 'ready'>('loading');
  const [message, setMessage] = React.useState<string | null>(null);
  React.useEffect(() => {
    if (!host || !auth.credential || auth.credential.kind !== 'bearer') return;
    let active = true;
    void createMobileKnowledgeApi(host, auth.credential).listDocuments(kbId).then((value) => {
      if (active) { setDocuments(value.data); setState('ready'); }
    }).catch((cause) => {
      if (!active) return;
      const status = cause as { status?: number };
      setState(status.status === 401 || status.status === 403 ? 'forbidden' : 'error');
      setMessage(cause instanceof Error ? cause.message : 'Could not load documents');
    });
    return () => { active = false; };
  }, [auth.credential, host, kbId]);
  return <View style={styles.container}>
    <Stack.Screen options={{ title: 'Documents' }} />
    {state === 'loading' ? <ActivityIndicator accessibilityLabel="Loading documents" /> : null}
    {state === 'forbidden' ? <Text accessibilityRole="alert" style={styles.error}>You do not have permission to view this knowledge base.</Text> : null}
    {state === 'error' ? <Text accessibilityRole="alert" style={styles.error}>{message ?? 'Could not load documents'}</Text> : null}
    {state === 'ready' ? <View style={styles.actions}><Pressable accessibilityRole="button" style={styles.button} onPress={() => router.push(`/knowledge/${encodeURIComponent(kbId)}/wiki`)}><Text>Wiki</Text></Pressable><Pressable accessibilityRole="button" style={styles.button} onPress={() => router.push(`/knowledge/${encodeURIComponent(kbId)}/faq`)}><Text>FAQ</Text></Pressable></View> : null}
    {state === 'ready' && documents.length === 0 ? <Text style={styles.muted}>No documents found.</Text> : null}
    <FlatList data={documents} keyExtractor={(item) => item.id} renderItem={({ item }) => <Pressable style={styles.row} onPress={() => router.push(`/knowledge/${encodeURIComponent(kbId)}/document/${encodeURIComponent(item.id)}`)}>
      <Ionicons name="document-text-outline" size={24} color="#53708a" /><Text style={styles.title}>{documentTitle(item)}</Text>
    </Pressable>} />
  </View>;
}

export function KnowledgeDocumentScreen({ documentId }: { documentId: string }) {
  const host = useMobileHost(); const auth = useProductAuth();
  const [detail, setDetail] = React.useState<any>(null); const [preview, setPreview] = React.useState<string | null>(null);
  const [state, setState] = React.useState<'loading' | 'forbidden' | 'error' | 'ready'>('loading'); const [busy, setBusy] = React.useState<'preview' | 'download' | null>(null); const [message, setMessage] = React.useState<string | null>(null);
  const api = React.useMemo(() => host && auth.credential?.kind === 'bearer' ? createMobileKnowledgeApi(host, auth.credential) : null, [auth.credential, host]);
  React.useEffect(() => { if (!api) return; let active = true; void api.detail(documentId).then((value) => { if (active) { setDetail(value); setState('ready'); } }).catch((cause) => { if (active) { const status = cause as { status?: number }; setState(status.status === 401 || status.status === 403 ? 'forbidden' : 'error'); setMessage(cause instanceof Error ? cause.message : 'Could not load document'); } }); return () => { active = false; }; }, [api, documentId]);
  const runFileAction = async (kind: 'preview' | 'download') => { if (!api || busy) return; setBusy(kind); setMessage(null); try { const file = await api[kind](documentId); if (kind === 'preview') { const text = new TextDecoder().decode(file.bytes); setPreview(text); } else { const name = String(detail?.file_name || detail?.title || `document-${documentId}`); const uri = `${FileSystem.cacheDirectory}${name.replace(/[^a-zA-Z0-9._-]/g, '_')}`; await FileSystem.writeAsStringAsync(uri, bytesToBase64(file.bytes), { encoding: FileSystem.EncodingType.Base64 }); if (await Sharing.isAvailableAsync()) await Sharing.shareAsync(uri, { mimeType: file.contentType }); else setMessage(`Downloaded to ${uri}`); } } catch (cause) { setMessage(cause instanceof Error ? cause.message : `Could not ${kind} document`); } finally { setBusy(null); } };
  return <View style={styles.container}><Stack.Screen options={{ title: 'Document' }} />{state === 'loading' ? <ActivityIndicator accessibilityLabel="Loading document" /> : null}{state === 'forbidden' ? <Text accessibilityRole="alert" style={styles.error}>You do not have permission to view this document.</Text> : null}{state === 'error' ? <Text accessibilityRole="alert" style={styles.error}>{message}</Text> : null}{state === 'ready' ? <><Text style={styles.heading}>{documentTitle(detail)}</Text><Text style={styles.muted}>Status: {String(detail?.parse_status ?? 'unknown')}</Text><View style={styles.actions}><Pressable accessibilityRole="button" disabled={busy !== null} onPress={() => void runFileAction('preview')} style={styles.button}><Text>Preview</Text></Pressable><Pressable accessibilityRole="button" disabled={busy !== null} onPress={() => void runFileAction('download')} style={styles.button}><Text>{busy === 'download' ? 'Downloading…' : 'Download'}</Text></Pressable></View>{preview ? <Text selectable style={styles.preview}>{preview}</Text> : null}{message ? <Text accessibilityRole="alert" style={styles.error}>{message}</Text> : null}</> : null}</View>;
}

export function KnowledgeCollectionScreen({ kbId, kind }: { kbId: string; kind: 'wiki' | 'faq' }) {
  const host = useMobileHost(); const auth = useProductAuth();
  const [items, setItems] = React.useState<any[]>([]); const [state, setState] = React.useState<'loading' | 'forbidden' | 'error' | 'ready'>('loading'); const [message, setMessage] = React.useState<string | null>(null);
  const api = React.useMemo(() => host && auth.credential?.kind === 'bearer' ? createMobileKnowledgeApi(host, auth.credential) : null, [auth.credential, host]);
  React.useEffect(() => { if (!api) return; let active = true; void api[kind === 'wiki' ? 'listWiki' : 'listFaq'](kbId).then((value) => { if (!active) return; const raw = value as any; const data = kind === 'wiki' ? (raw?.pages ?? raw?.data ?? raw) : (raw?.data?.data ?? raw?.data ?? raw); setItems(Array.isArray(data) ? data : []); setState('ready'); }).catch((cause) => { if (!active) return; const status = cause as { status?: number }; setState(status.status === 401 || status.status === 403 ? 'forbidden' : 'error'); setMessage(cause instanceof Error ? cause.message : `Could not load ${kind}`); }); return () => { active = false; }; }, [api, kbId, kind]);
  return <View style={styles.container}><Stack.Screen options={{ title: kind === 'wiki' ? 'Wiki' : 'FAQ' }} />{state === 'loading' ? <ActivityIndicator accessibilityLabel={`Loading ${kind}`} /> : null}{state === 'forbidden' ? <Text accessibilityRole="alert" style={styles.error}>You do not have permission to view this {kind}.</Text> : null}{state === 'error' ? <Text accessibilityRole="alert" style={styles.error}>{message}</Text> : null}{state === 'ready' && items.length === 0 ? <Text style={styles.muted}>No {kind} entries found.</Text> : null}<FlatList data={items} keyExtractor={(item, index) => String(item.id ?? item.slug ?? index)} renderItem={({ item }) => <View style={styles.row}><Text style={styles.title}>{String(item.title ?? item.standard_question ?? item.question ?? item.slug ?? 'Untitled')}</Text></View>} /></View>;
}

function StateMessage({ message, retry }: { message: string; retry: () => void }) { return <Pressable accessibilityRole="button" onPress={retry}><Text style={styles.error}>{message} Tap to retry.</Text></Pressable>; }
const styles = { container: { flex: 1, padding: 16, gap: 12 } as const, row: { flexDirection: 'row', alignItems: 'center', gap: 12, paddingVertical: 14, borderBottomWidth: 1, borderBottomColor: '#d7e0e7' } as const, title: { fontSize: 16, color: '#14212b' } as const, heading: { fontSize: 22, fontWeight: '600', color: '#14212b' } as const, muted: { color: '#637783' } as const, error: { color: '#b42318' } as const, actions: { flexDirection: 'row', gap: 12 } as const, button: { padding: 12, borderWidth: 1, borderColor: '#9db2c0', borderRadius: 8 } as const, preview: { marginTop: 12, color: '#14212b', lineHeight: 22 } as const };
