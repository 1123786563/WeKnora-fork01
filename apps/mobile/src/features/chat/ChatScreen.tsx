import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ActivityIndicator, AppState, FlatList, KeyboardAvoidingView, Platform, Pressable, SafeAreaView, ScrollView, Text, TextInput, View } from 'react-native';
import { useRouter } from 'expo-router';
import type { ChatMessage, ChatSession, ChatStreamEvent, TemporaryAttachment } from '@weknora/contracts';
import { initialChatStreamState, reduceChatStream, type ChatStreamState } from '@weknora/domain/chat/reducer';
import { artifactDownloadPath, normalizeArtifactList } from '@weknora/domain/chat/artifacts';
import { normalizeToolResult } from '@weknora/domain/chat/tool-results';
import { useMobileRuntime } from '../../runtime.tsx';
import { pickNativeFile } from '../../platform/files.ts';
import { downloadKnowledgeFile, shareNativeFile } from '../../platform/files.ts';
import { selectIncompleteAssistant, selectMessageArtifacts, selectReferenceGroups, shouldRenderPendingUser } from './parity.ts';

function errorText(cause: unknown, fallback: string): string {
  return cause instanceof Error ? cause.message : fallback;
}

function uniqueMessages(messages: readonly ChatMessage[]): ChatMessage[] {
  const seen = new Set<string>();
  return messages.filter((message) => { if (seen.has(message.id)) return false; seen.add(message.id); return true; });
}

export function ChatScreen() {
  const runtime = useMobileRuntime();
  const router = useRouter();
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(null);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [draft, setDraft] = useState('');
  const [pendingUser, setPendingUser] = useState<string | null>(null);
  const [streamState, setStreamState] = useState<ChatStreamState>(initialChatStreamState);
  const [attachments, setAttachments] = useState<TemporaryAttachment[]>([]);
  const [loading, setLoading] = useState(true);
  const [sending, setSending] = useState(false);
  const [attaching, setAttaching] = useState(false);
  const [error, setError] = useState('');
  const streamController = useRef<AbortController | null>(null);
  const activeMessageId = useRef<string | undefined>(undefined);
  const resuming = useRef(false);

  const loadSessions = useCallback(async () => {
    try {
      const result = await runtime.client.sessions.list({ page: 1, pageSize: 30, source: 'mobile' });
      setSessions(result.data);
      setSelectedSessionId((current) => current || result.data[0]?.id || null);
    } catch (cause) { setError(errorText(cause, 'Unable to load conversations')); }
    finally { setLoading(false); }
  }, [runtime.client]);

  const loadMessages = useCallback(async (sessionId: string): Promise<ChatMessage[]> => {
    try {
      const result = await runtime.client.sessions.messages(sessionId, { limit: 50 });
      setMessages(result);
      return result;
    } catch (cause) { setError(errorText(cause, 'Unable to load messages')); return []; }
  }, [runtime.client]);

  const loadAttachments = useCallback(async (sessionId: string) => {
    try { setAttachments(await runtime.client.chat.attachments.list(sessionId)); }
    catch (cause) { setError(errorText(cause, 'Unable to load attachments')); }
  }, [runtime.client]);

  useEffect(() => { void loadSessions(); }, [loadSessions]);
  useEffect(() => {
    setStreamState(initialChatStreamState());
    activeMessageId.current = undefined;
    if (selectedSessionId) { void loadMessages(selectedSessionId); void loadAttachments(selectedSessionId); }
    else { setMessages([]); setAttachments([]); }
  }, [loadAttachments, loadMessages, selectedSessionId]);

  const applyEvent = useCallback((event: ChatStreamEvent) => {
    if (typeof event.message_id === 'string' && event.message_id) activeMessageId.current = event.message_id;
    setStreamState((current) => reduceChatStream(current, event));
  }, []);

  const finishRun = useCallback(async (sessionId: string) => {
    streamController.current = null;
    setSending(false);
    const refreshed = await loadMessages(sessionId);
    const incomplete = selectIncompleteAssistant(refreshed);
    if (!incomplete) setPendingUser(null);
  }, [loadMessages]);

  const continueMessage = useCallback(async (sessionId: string, messageId: string) => {
    if (streamController.current || resuming.current) return;
    resuming.current = true;
    const controller = new AbortController();
    streamController.current = controller;
    activeMessageId.current = messageId;
    setSending(true); setError(''); setStreamState(initialChatStreamState());
    try { await runtime.client.chat.continueStream(sessionId, messageId, applyEvent, controller.signal); }
    catch (cause) { if (!controller.signal.aborted) setError(errorText(cause, 'Unable to resume response')); }
    finally { resuming.current = false; await finishRun(sessionId); }
  }, [applyEvent, finishRun, runtime.client]);

  useEffect(() => {
    const subscription = AppState.addEventListener('change', (state) => {
      if (state !== 'active' || !selectedSessionId || streamController.current) return;
      void (async () => {
        const refreshed = await loadMessages(selectedSessionId);
        const incomplete = selectIncompleteAssistant(refreshed);
        if (incomplete) await continueMessage(selectedSessionId, incomplete.id);
      })();
    });
    return () => subscription.remove();
  }, [continueMessage, loadMessages, selectedSessionId]);

  async function createSession() {
    setError('');
    try {
      const session = await runtime.client.sessions.create({ title: 'New conversation' });
      setSessions((current) => [session, ...current.filter((item) => item.id !== session.id)]);
      setSelectedSessionId(session.id); setMessages([]); setAttachments([]); setPendingUser(null);
    } catch (cause) { setError(errorText(cause, 'Unable to create conversation')); }
  }

  async function ensureSession(): Promise<string> {
    if (selectedSessionId) return selectedSessionId;
    const session = await runtime.client.sessions.create({ title: 'New conversation' });
    setSessions((current) => [session, ...current]);
    setSelectedSessionId(session.id);
    return session.id;
  }

  async function send(value = draft) {
    const query = value.trim();
    if (!query || sending) return;
    setError('');
    let sessionId: string;
    try { sessionId = await ensureSession(); } catch (cause) { setError(errorText(cause, 'Unable to create conversation')); return; }
    setDraft(''); setPendingUser(query); setStreamState(initialChatStreamState());
    const controller = new AbortController();
    streamController.current = controller;
    setSending(true);
    try {
      await runtime.client.chat.stream({
        sessionId,
        mode: 'knowledge',
        body: { query, channel: 'mobile', attachment_ids: attachments.map((attachment) => attachment.id) },
        signal: controller.signal,
      }, applyEvent);
    } catch (cause) {
      if (!controller.signal.aborted) setError(errorText(cause, 'Unable to send message'));
    } finally { await finishRun(sessionId); }
  }

  async function stop() {
    if (!selectedSessionId || !activeMessageId.current) return;
    try { await runtime.client.chat.stop(selectedSessionId, activeMessageId.current); }
    catch (cause) { setError(errorText(cause, 'Unable to stop response')); }
    streamController.current?.abort();
    applyEvent({ response_type: 'stop', event_id: `local-stop-${Date.now()}` });
    setSending(false);
  }

  async function attach() {
    if (attaching) return;
    setAttaching(true); setError('');
    try {
      const sessionId = await ensureSession();
      const file = await pickNativeFile();
      if (!file) return;
      const attachment = await runtime.client.chat.attachments.upload(sessionId, { file });
      setAttachments((current) => [...current.filter((item) => item.id !== attachment.id), attachment]);
    } catch (cause) { setError(errorText(cause, 'Unable to attach file')); }
    finally { setAttaching(false); }
  }

  async function approve(pendingId: string, decision: 'approve' | 'reject') {
    try {
      await runtime.client.chat.approvals.resolveTool(pendingId, { decision });
      applyEvent({ response_type: 'tool_approval_resolved', data: { pending_id: pendingId, decision }, event_id: `local-approval-${pendingId}` });
    } catch (cause) { setError(errorText(cause, 'Unable to resolve tool approval')); }
  }

  async function shareArtifact(messageId: string, artifact: ReturnType<typeof normalizeArtifactList>[number]) {
    try {
      const uri = await downloadKnowledgeFile({
        baseURL: runtime.baseURL,
        path: artifactDownloadPath(selectedSessionId || '', messageId, artifact.index),
        fileName: artifact.fileName,
        credential: runtime.credential,
      });
      await shareNativeFile(uri);
    } catch (cause) { setError(errorText(cause, 'Unable to download artifact')); }
  }

  const liveAssistant = streamState.answer ? [{ id: 'mobile-live-assistant', session_id: selectedSessionId || '', role: 'assistant' as const, content: streamState.answer, is_completed: streamState.phase === 'completed' }] : [];
  const displayMessages = useMemo(() => uniqueMessages([
    ...messages,
    ...(shouldRenderPendingUser(messages, pendingUser) ? [{ id: 'mobile-pending-user', session_id: selectedSessionId || '', role: 'user' as const, content: pendingUser! }] : []),
    ...liveAssistant,
  ]), [liveAssistant, messages, pendingUser, selectedSessionId]);
  const references = selectReferenceGroups(streamState);
  const pendingApprovals = Object.values(streamState.approvals).filter((approval) => approval.status === 'pending');

  return <SafeAreaView style={{ flex: 1 }}>
    <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', padding: 12, borderBottomColor: '#eaecf0', borderBottomWidth: 1 }}>
      <Text accessibilityRole="header" style={{ fontSize: 22, fontWeight: '700' }}>Chat</Text>
      <View style={{ flexDirection: 'row', gap: 12 }}><Pressable onPress={() => router.push('/knowledge')}><Text style={{ color: '#2864dc' }}>Knowledge</Text></Pressable><Pressable onPress={() => router.push('/management')}><Text style={{ color: '#2864dc' }}>Manage</Text></Pressable><Pressable onPress={() => void createSession()}><Text style={{ color: '#2864dc' }}>New</Text></Pressable></View>
    </View>
    <FlatList horizontal data={sessions} keyExtractor={(item) => item.id} showsHorizontalScrollIndicator={false} style={{ maxHeight: 48, paddingHorizontal: 12, paddingTop: 8 }} renderItem={({ item }) => <Pressable onPress={() => { if (!sending) setSelectedSessionId(item.id); }} style={{ paddingHorizontal: 10, paddingVertical: 7, borderRadius: 14, backgroundColor: item.id === selectedSessionId ? '#dbeafe' : '#f2f4f7', marginRight: 6 }}><Text numberOfLines={1}>{item.title || 'Untitled'}</Text></Pressable>} ListEmptyComponent={loading ? <ActivityIndicator /> : <Text style={{ color: '#667085' }}>No conversations</Text>} />
    {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', paddingHorizontal: 12, paddingTop: 8 }}>{error}</Text> : null}
    <FlatList
      style={{ flex: 1, paddingHorizontal: 12 }}
      contentContainerStyle={{ paddingVertical: 12 }}
      data={displayMessages}
      keyExtractor={(item) => item.id}
      keyboardShouldPersistTaps="handled"
      ListHeaderComponent={<>
        {streamState.thinking ? <View style={{ backgroundColor: '#f8f9fc', padding: 10, borderRadius: 8, marginBottom: 8 }}><Text style={{ color: '#667085' }}>Thinking</Text><Text selectable style={{ color: '#667085' }}>{streamState.thinking}</Text></View> : null}
        {references.length ? <View style={{ marginBottom: 8 }}><Text style={{ fontWeight: '600' }}>References</Text>{references.flatMap((group) => group.items).map((reference) => <View key={reference.key} style={{ backgroundColor: '#f8f9fc', padding: 8, borderRadius: 8, marginTop: 4 }}><Text>{reference.title}</Text>{reference.snippet ? <Text selectable style={{ color: '#667085', fontSize: 12 }}>{reference.snippet}</Text> : null}</View>)}</View> : null}
        {pendingApprovals.map((approval) => <View key={approval.pendingId} style={{ backgroundColor: '#fff7ed', padding: 10, borderRadius: 8, marginBottom: 8 }}><Text>Tool approval required</Text><View style={{ flexDirection: 'row', gap: 12, marginTop: 8 }}><Pressable onPress={() => void approve(approval.pendingId, 'approve')}><Text style={{ color: '#16803c' }}>Approve</Text></Pressable><Pressable onPress={() => void approve(approval.pendingId, 'reject')}><Text style={{ color: '#b42318' }}>Reject</Text></Pressable></View></View>)}
      </>}
      renderItem={({ item }) => <View style={{ alignSelf: item.role === 'user' ? 'flex-end' : 'stretch', maxWidth: '92%', backgroundColor: item.role === 'user' ? '#eff6ff' : '#f8f9fc', padding: 10, borderRadius: 10, marginBottom: 8 }}><Text style={{ fontWeight: '600', marginBottom: 4 }}>{item.role}</Text><Text selectable>{item.content}</Text>{item.role === 'assistant' && item.is_completed === false ? <Text style={{ color: '#667085', marginTop: 4 }}>Resuming…</Text> : null}{normalizeArtifactList(selectMessageArtifacts(item)).map((artifact) => <Pressable key={`${artifact.index}-${artifact.fileName}`} onPress={() => void shareArtifact(item.id, artifact)}><Text style={{ color: '#2864dc', marginTop: 6 }}>File: {artifact.fileName} · Share</Text></Pressable>)}{item.role === 'assistant' && item.data ? <Text style={{ color: '#667085' }}>{normalizeToolResult({ output: item.data }).text}</Text> : null}</View>}
    />
    {attachments.length ? <ScrollView horizontal style={{ maxHeight: 38, paddingHorizontal: 12 }}><View style={{ flexDirection: 'row', gap: 8 }}>{attachments.map((attachment) => <View key={attachment.id} style={{ backgroundColor: '#f2f4f7', borderRadius: 12, paddingHorizontal: 10, paddingVertical: 6 }}><Text>{attachment.file_name} · {attachment.status}</Text></View>)}</View></ScrollView> : null}
    <View style={{ flexDirection: 'row', gap: 8, paddingHorizontal: 12, paddingVertical: 6 }}><Pressable onPress={() => setDraft('Summarize the selected knowledge base')}><Text style={{ color: '#2864dc', fontSize: 12 }}>Summarize</Text></Pressable><Pressable onPress={() => setDraft('Find related files')}><Text style={{ color: '#2864dc', fontSize: 12 }}>Related files</Text></Pressable></View>
    <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} keyboardVerticalOffset={90} style={{ flexDirection: 'row', alignItems: 'flex-end', gap: 8, padding: 12, borderTopColor: '#eaecf0', borderTopWidth: 1 }}>
      <Pressable accessibilityLabel="Attach file" onPress={() => void attach()}><Text style={{ color: attaching ? '#98a2b3' : '#2864dc' }}>{attaching ? '…' : '+'}</Text></Pressable>
      <TextInput accessibilityLabel="Chat message" value={draft} onChangeText={setDraft} multiline maxLength={20_000} placeholder="Ask WeKnora" style={{ flex: 1, maxHeight: 120, borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, paddingHorizontal: 10, paddingVertical: 8 }} />
      {sending ? <Pressable accessibilityRole="button" onPress={() => void stop()}><Text style={{ color: '#b42318' }}>Stop</Text></Pressable> : <Pressable accessibilityRole="button" disabled={!draft.trim()} onPress={() => void send()}><Text style={{ color: draft.trim() ? '#2864dc' : '#98a2b3', fontWeight: '600' }}>Send</Text></Pressable>}
    </KeyboardAvoidingView>
  </SafeAreaView>;
}
