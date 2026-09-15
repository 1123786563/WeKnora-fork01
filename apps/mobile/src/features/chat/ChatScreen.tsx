import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ActivityIndicator, AppState, FlatList, KeyboardAvoidingView, Linking, Platform, Pressable, SafeAreaView, ScrollView, Text, TextInput, View } from 'react-native';
import { useRouter } from 'expo-router';
import * as SecureStore from 'expo-secure-store';
import { formatMessage } from '@weknora/i18n';
import type { ChatMessage, ChatSession, ChatStreamEvent, KnowledgeBase, SteerDelivery, SteerQueueItem, TemporaryAttachment } from '@weknora/contracts';
import { initialChatStreamState, reduceChatStream, type ChatStreamState } from '@weknora/domain/chat/reducer';
import { artifactDownloadPath, normalizeArtifactList } from '@weknora/domain/chat/artifacts';
import { normalizeToolResult } from '@weknora/domain/chat/tool-results';
import { useMobileRuntime } from '../../runtime.tsx';
import { pickNativeFile } from '../../platform/files.ts';
import { downloadKnowledgeFile, readNativeTextFile, shareNativeFile } from '../../platform/files.ts';
import { classifyNativeArtifactPreview, NativeArtifactPreview } from './artifact-preview.tsx';
import { nativeArtifactPreviewLabels } from './artifact-preview-labels.ts';
import { buildMobileChatRequestBody, findRetryQuery, isCurrentChatRun, selectAssistantMessageId, selectIncompleteAssistant, selectMessageArtifacts, selectReferenceGroups, shouldRenderLiveAssistant, shouldRenderPendingUser, type ChatRunToken } from './parity.ts';
import { stopChatRun } from './stop-run.ts';
import { chatAppStateAction } from './appstate.ts';
import { createRunLifecyclePersistence, initialRunLifecycle, shouldApplyHydratedLifecycle, transitionRunLifecycle, type RunLifecycleEvent } from './run-lifecycle.ts';

function errorText(cause: unknown, fallback: string): string {
  return cause instanceof Error ? cause.message : fallback;
}

function uniqueMessages(messages: readonly ChatMessage[]): ChatMessage[] {
  const seen = new Set<string>();
  return messages.filter((message) => { if (seen.has(message.id)) return false; seen.add(message.id); return true; });
}

export function ChatScreen() {
  const runtime = useMobileRuntime();
  const label = useCallback((key: string) => formatMessage(runtime.locale, key), [runtime.locale]);
  const router = useRouter();
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [knowledgeBases, setKnowledgeBases] = useState<KnowledgeBase[]>([]);
  const [selectedKnowledgeBaseId, setSelectedKnowledgeBaseId] = useState<string | null>(null);
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(null);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [draft, setDraft] = useState('');
  const [pendingUser, setPendingUser] = useState<string | null>(null);
  const [streamState, setStreamState] = useState<ChatStreamState>(initialChatStreamState);
  const [attachments, setAttachments] = useState<TemporaryAttachment[]>([]);
  const [steerQueue, setSteerQueue] = useState<SteerQueueItem[]>([]);
  const [steerLoading, setSteerLoading] = useState(false);
  const [steerBusy, setSteerBusy] = useState('');
  const [loading, setLoading] = useState(true);
  const [sending, setSending] = useState(false);
  const [attaching, setAttaching] = useState(false);
  const [error, setError] = useState('');
  const [artifactPreview, setArtifactPreview] = useState<{ artifact: ReturnType<typeof normalizeArtifactList>[number]; uri?: string; content?: string; loading: boolean; error?: string } | null>(null);
  const streamController = useRef<AbortController | null>(null);
  const activeRun = useRef<(ChatRunToken & { controller: AbortController }) | null>(null);
  const runSequence = useRef(0);
  const activeMessageId = useRef<string | undefined>(undefined);
  const activeOAuth = useRef<{ pendingId: string; serviceId: string; authorizationAttempt: string } | null>(null);
  const steerAction = useRef<string | null>(null);
  const failedAssistantMessageId = useRef<string | undefined>(undefined);
  const resuming = useRef(false);
  const selectedSessionRef = useRef<string | null>(null);
  const runLifecycle = useRef(initialRunLifecycle());
  const lifecycleHydrated = useRef(false);
  const lifecycleRevision = useRef(0);
  const sessionsGeneration = useRef(0);
  const knowledgeBasesGeneration = useRef(0);
  const messagesGeneration = useRef(0);
  const attachmentsGeneration = useRef(0);
  const steerGeneration = useRef(0);
  const attachmentUploadGeneration = useRef(0);
  const lifecyclePersistence = useMemo(() => createRunLifecyclePersistence(SecureStore), []);

  const updateRunLifecycle = useCallback((sessionId: string, event: RunLifecycleEvent) => {
    lifecycleRevision.current += 1;
    const next = transitionRunLifecycle(runLifecycle.current, event);
    runLifecycle.current = next;
    void lifecyclePersistence.write(sessionId, next).catch(() => undefined);
  }, [lifecyclePersistence]);

  const loadSessions = useCallback(async () => {
    const generation = ++sessionsGeneration.current;
    try {
      const result = await runtime.client.sessions.list({ page: 1, pageSize: 30, source: 'mobile' });
      if (generation !== sessionsGeneration.current) return;
      setSessions(result.data);
      setSelectedSessionId((current) => current || result.data[0]?.id || null);
    } catch (cause) {
      if (generation === sessionsGeneration.current) setError(errorText(cause, label('mobileChat.loadSessionsFailed')));
    } finally {
      if (generation === sessionsGeneration.current) setLoading(false);
    }
  }, [label, runtime.client]);

  const loadKnowledgeBases = useCallback(async () => {
    const generation = ++knowledgeBasesGeneration.current;
    try {
      const result = await runtime.client.knowledgeBases.list();
      if (generation !== knowledgeBasesGeneration.current) return;
      setKnowledgeBases(result);
      setSelectedKnowledgeBaseId((current) => current && result.some((item) => item.id === current) ? current : result[0]?.id || null);
    } catch (cause) {
      if (generation === knowledgeBasesGeneration.current) setError(errorText(cause, label('mobileChat.loadKnowledgeBasesFailed')));
    }
  }, [label, runtime.client]);

  const loadMessages = useCallback(async (sessionId: string): Promise<ChatMessage[]> => {
    const generation = ++messagesGeneration.current;
    try {
      const result = await runtime.client.sessions.messages(sessionId, { limit: 50 });
      if (generation !== messagesGeneration.current || selectedSessionRef.current !== sessionId) return [];
      setMessages(result);
      return result;
    } catch (cause) {
      if (generation === messagesGeneration.current && selectedSessionRef.current === sessionId) setError(errorText(cause, label('mobileChat.loadMessagesFailed')));
      return [];
    }
  }, [label, runtime.client]);

  const loadAttachments = useCallback(async (sessionId: string) => {
    const generation = ++attachmentsGeneration.current;
    try {
      const result = await runtime.client.chat.attachments.list(sessionId);
      if (generation === attachmentsGeneration.current && selectedSessionRef.current === sessionId) setAttachments(result);
    } catch (cause) {
      if (generation === attachmentsGeneration.current && selectedSessionRef.current === sessionId) setError(errorText(cause, label('mobileChat.loadAttachmentsFailed')));
    }
  }, [label, runtime.client]);

  const loadSteerQueue = useCallback(async (sessionId: string) => {
    const generation = ++steerGeneration.current;
    setSteerLoading(true);
    try {
      const result = await runtime.client.chat.steer.list(sessionId);
      if (generation === steerGeneration.current && selectedSessionRef.current === sessionId) setSteerQueue(result.items);
    } catch (cause) {
      if (generation === steerGeneration.current && selectedSessionRef.current === sessionId) setError(errorText(cause, label('mobileChat.loadSteerQueueFailed')));
    } finally {
      if (generation === steerGeneration.current && selectedSessionRef.current === sessionId) setSteerLoading(false);
    }
  }, [label, runtime.client]);

  useEffect(() => { void loadSessions(); }, [loadSessions]);
  useEffect(() => { void loadKnowledgeBases(); }, [loadKnowledgeBases]);
  useEffect(() => {
    selectedSessionRef.current = selectedSessionId;
    lifecycleRevision.current += 1;
    runLifecycle.current = initialRunLifecycle();
    lifecycleHydrated.current = !selectedSessionId;
    setStreamState(initialChatStreamState());
    activeMessageId.current = undefined;
    failedAssistantMessageId.current = undefined;
    if (selectedSessionId) {
      const sessionId = selectedSessionId;
      const hydrationRevision = lifecycleRevision.current;
      void lifecyclePersistence.read(sessionId).then((stored) => {
        if (!shouldApplyHydratedLifecycle(selectedSessionRef.current, sessionId, hydrationRevision, lifecycleRevision.current)) return;
        runLifecycle.current = stored;
        lifecycleHydrated.current = true;
      }).catch(() => {
        if (selectedSessionRef.current === sessionId) lifecycleHydrated.current = true;
      });
      void loadMessages(sessionId); void loadAttachments(sessionId);
    }
    if (selectedSessionId) void loadSteerQueue(selectedSessionId);
    else { setMessages([]); setAttachments([]); setSteerQueue([]); }
  }, [lifecyclePersistence, loadAttachments, loadMessages, loadSteerQueue, selectedSessionId]);

  const applyEvent = useCallback((event: ChatStreamEvent, token?: ChatRunToken) => {
    if (token && !isCurrentChatRun(activeRun.current, token)) return;
    const eventSessionId = token?.sessionId ?? selectedSessionRef.current;
    if (!eventSessionId) return;
    const type = event.response_type ?? event.type;
    const assistantMessageId = selectAssistantMessageId(event);
    if (assistantMessageId) {
      activeMessageId.current = assistantMessageId;
      updateRunLifecycle(eventSessionId, { type: 'assistant-message', id: assistantMessageId });
    }
    if (type === 'error') {
      updateRunLifecycle(eventSessionId, { type: 'failure' });
      failedAssistantMessageId.current = assistantMessageId ?? activeMessageId.current;
      const data = typeof event.data === 'object' && event.data !== null && !Array.isArray(event.data)
        ? event.data as Record<string, unknown>
        : undefined;
      setError(typeof event.error === 'string' ? event.error : typeof data?.error === 'string' ? data.error : label('mobileChat.streamFailed'));
    }
    if (type === 'complete') updateRunLifecycle(eventSessionId, { type: 'complete' });
    if (type === 'stop') updateRunLifecycle(eventSessionId, { type: 'user-stop' });
    setStreamState((current) => reduceChatStream(current, event));
  }, [label, updateRunLifecycle]);

  const finishRun = useCallback(async (sessionId: string, controller: AbortController) => {
    if (streamController.current !== controller) return;
    if (activeRun.current?.controller !== controller) return;
    activeRun.current = null;
    streamController.current = null;
    setSending(false);
    const refreshed = await loadMessages(sessionId);
    await loadSteerQueue(sessionId);
    if (failedAssistantMessageId.current === activeMessageId.current && failedAssistantMessageId.current) {
      setPendingUser(null);
      return;
    }
    if (runLifecycle.current.status === 'failed') {
      setPendingUser(null);
      return;
    }
    const incomplete = selectIncompleteAssistant(refreshed);
    if (!incomplete) {
      updateRunLifecycle(sessionId, { type: 'complete' });
      setPendingUser(null);
    }
  }, [loadMessages, loadSteerQueue, updateRunLifecycle]);

  const continueMessage = useCallback(async (sessionId: string, messageId: string) => {
    if (streamController.current || resuming.current) return;
    resuming.current = true;
    const controller = new AbortController();
    const token: ChatRunToken = { sessionId, runId: `${sessionId}:${++runSequence.current}` };
    activeRun.current = { ...token, controller };
    streamController.current = controller;
    activeMessageId.current = messageId;
    failedAssistantMessageId.current = undefined;
    updateRunLifecycle(sessionId, { type: 'start', assistantMessageId: messageId });
    setSending(true); setError(''); setStreamState(initialChatStreamState());
    try { await runtime.client.chat.continueStream(sessionId, messageId, (event) => applyEvent(event, token), controller.signal); }
    catch (cause) {
      if (!controller.signal.aborted) {
        updateRunLifecycle(sessionId, { type: 'failure' });
        setError(errorText(cause, label('mobileChat.resumeFailed')));
      }
    }
    finally { resuming.current = false; await finishRun(sessionId, controller); }
  }, [applyEvent, finishRun, label, runtime.client, updateRunLifecycle]);

  useEffect(() => {
    const subscription = AppState.addEventListener('change', (state) => {
      const action = chatAppStateAction(
        state,
        Boolean(selectedSessionId) && lifecycleHydrated.current,
        Boolean(streamController.current),
        runLifecycle.current,
      );
      if (action === 'abort') {
        if (selectedSessionId) updateRunLifecycle(selectedSessionId, { type: 'background' });
        streamController.current?.abort();
        return;
      }
      if (action !== 'resume' || !selectedSessionId) return;
      void (async () => {
        const refreshed = await loadMessages(selectedSessionId);
        const incomplete = selectIncompleteAssistant(refreshed);
        if (incomplete && incomplete.id !== failedAssistantMessageId.current) await continueMessage(selectedSessionId, incomplete.id);
        else if (!incomplete) updateRunLifecycle(selectedSessionId, { type: 'complete' });
      })();
    });
    return () => subscription.remove();
  }, [continueMessage, loadMessages, selectedSessionId, updateRunLifecycle]);

  useEffect(() => {
    const subscription = Linking.addEventListener('url', ({ url }) => {
      const hash = url.includes('#') ? url.slice(url.indexOf('#') + 1) : url.includes('?') ? url.slice(url.indexOf('?') + 1) : '';
      const params = new URLSearchParams(hash);
      if (!params.has('mcp_oauth_result') && !params.has('mcp_oauth_error')) return;
      const pending = activeOAuth.current;
      if (!pending) return;
      activeOAuth.current = null;
      if (params.get('mcp_oauth_error')) { setError(params.get('mcp_oauth_error') || label('mobileChat.mcpAuthorizationFailed')); return; }
      void runtime.client.configuration.mcp.oauth.status(pending.serviceId, pending.authorizationAttempt).then((status) => {
        if (!status.authorized) throw new Error(label('mobileChat.mcpNotComplete'));
        return runtime.client.chat.approvals.resolveOAuth(pending.pendingId, { serviceId: pending.serviceId, decision: 'authorize' });
      }).then(() => {
        applyEvent({ response_type: 'mcp_oauth_resolved', data: { pending_id: pending.pendingId, service_id: pending.serviceId, authorized: true }, event_id: `local-oauth-${pending.pendingId}` });
      }).catch((cause) => setError(errorText(cause, label('mobileChat.mcpFinishFailed'))));
    });
    return () => subscription.remove();
  }, [applyEvent, label, runtime.client]);

  async function createSession() {
    setError('');
    try {
      const session = await runtime.client.sessions.create({ title: formatMessage(runtime.locale, 'mobileChat.newConversation') });
      setSessions((current) => [session, ...current.filter((item) => item.id !== session.id)]);
      setSelectedSessionId(session.id); setMessages([]); setAttachments([]); setSteerQueue([]); setPendingUser(null);
    } catch (cause) { setError(errorText(cause, label('mobileChat.createConversationFailed'))); }
  }

  async function ensureSession(): Promise<string> {
    if (selectedSessionId) return selectedSessionId;
    const session = await runtime.client.sessions.create({ title: formatMessage(runtime.locale, 'mobileChat.newConversation') });
    setSessions((current) => [session, ...current]);
    setSelectedSessionId(session.id); selectedSessionRef.current = session.id; setSteerQueue([]);
    return session.id;
  }

  async function send(value = draft, options: { allowWhileSending?: boolean } = {}) {
    const query = value.trim();
    if (!query || (sending && !options.allowWhileSending)) return;
    if (!selectedKnowledgeBaseId) { setError(label('mobileChat.selectKnowledgeBase')); return; }
    setError('');
    let sessionId: string;
    try { sessionId = await ensureSession(); } catch (cause) { setError(errorText(cause, label('mobileChat.createConversationFailed'))); return; }
    setDraft(''); setPendingUser(query); setStreamState(initialChatStreamState());
    activeMessageId.current = undefined;
    failedAssistantMessageId.current = undefined;
    const controller = new AbortController();
    const token: ChatRunToken = { sessionId, runId: `${sessionId}:${++runSequence.current}` };
    activeRun.current = { ...token, controller };
    streamController.current = controller;
    updateRunLifecycle(sessionId, { type: 'start' });
    setSending(true);
    try {
      await runtime.client.chat.stream({
        sessionId,
        mode: 'knowledge',
        body: buildMobileChatRequestBody(query, [selectedKnowledgeBaseId], attachments.map((attachment) => attachment.id)),
        signal: controller.signal,
      }, (event) => applyEvent(event, token));
    } catch (cause) {
      if (!controller.signal.aborted) {
        updateRunLifecycle(sessionId, { type: 'failure' });
        setError(errorText(cause, label('mobileChat.sendFailed')));
      }
    } finally { await finishRun(sessionId, controller); }
  }

  async function stop() {
    if (!selectedSessionId) return;
    const sessionId = selectedSessionId;
    const messageId = activeMessageId.current ?? runLifecycle.current.assistantMessageId;
    updateRunLifecycle(sessionId, { type: 'user-stop' });
    try {
      await stopChatRun(
        messageId ? async () => { await runtime.client.chat.stop(sessionId, messageId); } : undefined,
        () => streamController.current?.abort(),
        () => {
          const token = activeRun.current && activeRun.current.sessionId === sessionId ? activeRun.current : undefined;
          applyEvent({ response_type: 'stop', event_id: `local-stop-${Date.now()}` }, token || undefined);
          setSending(false);
        },
      );
    } catch (cause) { setError(errorText(cause, label('mobileChat.stopFailed'))); }
  }

  async function attach() {
    if (attaching) return;
    const generation = ++attachmentUploadGeneration.current;
    setAttaching(true); setError('');
    try {
      const sessionId = await ensureSession();
      const file = await pickNativeFile();
      if (!file) return;
      const attachment = await runtime.client.chat.attachments.upload(sessionId, { file });
      if (generation === attachmentUploadGeneration.current && selectedSessionRef.current === sessionId) setAttachments((current) => [...current.filter((item) => item.id !== attachment.id), attachment]);
    } catch (cause) {
      if (generation === attachmentUploadGeneration.current) setError(errorText(cause, label('mobileChat.attachFailed')));
    } finally {
      if (generation === attachmentUploadGeneration.current) setAttaching(false);
    }
  }

  async function approve(pendingId: string, decision: 'approve' | 'reject') {
    try {
      await runtime.client.chat.approvals.resolveTool(pendingId, { decision });
      applyEvent({ response_type: 'tool_approval_resolved', data: { pending_id: pendingId, decision }, event_id: `local-approval-${pendingId}` });
    } catch (cause) { setError(errorText(cause, label('mobileChat.toolApprovalFailed'))); }
  }

  async function openOAuth(approval: ChatStreamState['oauthApprovals'][string]) {
    if (!approval.serviceId) { setError(label('mobileChat.mcpMissingService')); return; }
    try {
      const result = await runtime.client.configuration.mcp.oauth.authorizeUrl(approval.serviceId, {
        redirectURI: `${runtime.baseURL.replace(/\/+$/, '')}/api/v1/mcp-oauth/callback`,
        frontendRedirect: 'weknora://mcp-oauth',
      });
      activeOAuth.current = { pendingId: approval.pendingId, serviceId: approval.serviceId, authorizationAttempt: result.authorizationAttempt };
      await Linking.openURL(result.authorizationUrl);
    } catch (cause) { setError(errorText(cause, label('mobileChat.startMcpFailed'))); }
  }

  async function cancelOAuth(approval: ChatStreamState['oauthApprovals'][string]) {
    if (!approval.serviceId) return;
    try {
      await runtime.client.chat.approvals.cancelOAuth(approval.pendingId);
      applyEvent({ response_type: 'mcp_oauth_resolved', data: { pending_id: approval.pendingId, service_id: approval.serviceId, authorized: false }, event_id: `local-oauth-cancel-${approval.pendingId}` });
    } catch (cause) { setError(errorText(cause, label('mobileChat.cancelMcpFailed'))); }
  }

  async function enqueueSteer(delivery: SteerDelivery) {
    const query = draft.trim();
    if (!query) return;
    if (!selectedSessionId || !sending) { await send(query); return; }
    if (steerAction.current) return;
    const sessionId = selectedSessionId;
    steerAction.current = `enqueue:${delivery}`;
    setSteerBusy(delivery); setError('');
    try {
      const result = await runtime.client.chat.steer.enqueue(sessionId, {
        query,
        delivery,
        expectedAssistantMessageId: activeMessageId.current,
        channel: 'mobile',
      });
      setDraft('');
      if (result.status === 'new_run') await send(query, { allowWhileSending: true });
      else await loadSteerQueue(sessionId);
    } catch (cause) { setError(errorText(cause, label('mobileChat.steerFailed'))); }
    finally { steerAction.current = null; setSteerBusy(''); }
  }

  async function promoteSteer(item: SteerQueueItem) {
    if (!selectedSessionId || steerAction.current) return;
    const sessionId = selectedSessionId;
    steerAction.current = `promote:${item.steer_id}`;
    setSteerBusy(item.steer_id); setError('');
    try {
      await runtime.client.chat.steer.promote(sessionId, item.steer_id);
      await loadSteerQueue(sessionId);
    } catch (cause) { setError(errorText(cause, label('mobileChat.injectSteerFailed'))); }
    finally { steerAction.current = null; setSteerBusy(''); }
  }

  async function removeSteer(item: SteerQueueItem) {
    if (!selectedSessionId || steerAction.current) return;
    const sessionId = selectedSessionId;
    steerAction.current = `remove:${item.steer_id}`;
    setSteerBusy(item.steer_id); setError('');
    try {
      await runtime.client.chat.steer.remove(sessionId, item.steer_id);
      await loadSteerQueue(sessionId);
    } catch (cause) { setError(errorText(cause, label('mobileChat.removeSteerFailed'))); }
    finally { steerAction.current = null; setSteerBusy(''); }
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
    } catch (cause) { setError(errorText(cause, label('mobileChat.downloadArtifactFailed'))); }
  }

  async function openArtifact(messageId: string, artifact: ReturnType<typeof normalizeArtifactList>[number]) {
    const model = classifyNativeArtifactPreview(artifact);
    if (model.kind === 'download-only') { await shareArtifact(messageId, artifact); return; }
    setArtifactPreview({ artifact, loading: true });
    try {
      const uri = await downloadKnowledgeFile({ baseURL: runtime.baseURL, path: artifactDownloadPath(selectedSessionId || '', messageId, artifact.index), fileName: artifact.fileName, credential: runtime.credential });
      const content = model.kind === 'text' || model.kind === 'markdown' ? await readNativeTextFile(uri) : undefined;
      setArtifactPreview((current) => current?.artifact === artifact ? { artifact, uri, content, loading: false } : current);
    } catch (cause) {
      setArtifactPreview((current) => current?.artifact === artifact ? { artifact, loading: false, error: errorText(cause, label('mobileChat.previewArtifactFailed')) } : current);
    }
  }

  const liveAssistant = shouldRenderLiveAssistant(sending, streamState.answer) ? [{ id: 'mobile-live-assistant', session_id: selectedSessionId || '', role: 'assistant' as const, content: streamState.answer, is_completed: streamState.phase === 'completed' }] : [];
  const displayMessages = useMemo(() => uniqueMessages([
    ...messages,
    ...(shouldRenderPendingUser(messages, pendingUser) ? [{ id: 'mobile-pending-user', session_id: selectedSessionId || '', role: 'user' as const, content: pendingUser! }] : []),
    ...liveAssistant,
  ]), [liveAssistant, messages, pendingUser, selectedSessionId]);
  const references = selectReferenceGroups(streamState);
  const pendingApprovals = Object.values(streamState.approvals).filter((approval) => approval.status === 'pending');
  const pendingOAuthApprovals = Object.values(streamState.oauthApprovals).filter((approval) => approval.status === 'pending');
  const steerButtonsDisabled = !draft.trim() || Boolean(steerBusy);

  return <SafeAreaView style={{ flex: 1 }}>
    <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', padding: 12, borderBottomColor: '#eaecf0', borderBottomWidth: 1 }}>
      <Text accessibilityRole="header" style={{ fontSize: 22, fontWeight: '700' }}>{label('mobileChat.title')}</Text>
      <View style={{ flexDirection: 'row', gap: 12 }}><Pressable onPress={() => router.push('/knowledge')}><Text style={{ color: '#2864dc' }}>{formatMessage(runtime.locale, 'input.knowledgeBase')}</Text></Pressable><Pressable onPress={() => router.push('/management')}><Text style={{ color: '#2864dc' }}>{formatMessage(runtime.locale, 'menu.settings')}</Text></Pressable><Pressable onPress={() => void createSession()}><Text style={{ color: '#2864dc' }}>{formatMessage(runtime.locale, 'mobileChat.new')}</Text></Pressable></View>
    </View>
    <FlatList horizontal data={sessions} keyExtractor={(item) => item.id} showsHorizontalScrollIndicator={false} style={{ maxHeight: 48, paddingHorizontal: 12, paddingTop: 8 }} renderItem={({ item }) => <Pressable onPress={() => { if (!sending) setSelectedSessionId(item.id); }} style={{ paddingHorizontal: 10, paddingVertical: 7, borderRadius: 14, backgroundColor: item.id === selectedSessionId ? '#dbeafe' : '#f2f4f7', marginRight: 6 }}><Text numberOfLines={1}>{item.title || label('mobileChat.untitled')}</Text></Pressable>} ListEmptyComponent={loading ? <ActivityIndicator /> : <Text style={{ color: '#667085' }}>{label('mobileChat.noConversations')}</Text>} />
    <FlatList horizontal data={knowledgeBases} keyExtractor={(item) => item.id} showsHorizontalScrollIndicator={false} style={{ maxHeight: 48, paddingHorizontal: 12, paddingTop: 8 }} renderItem={({ item }) => <Pressable accessibilityRole="button" onPress={() => { if (!sending) setSelectedKnowledgeBaseId(item.id); }} style={{ paddingHorizontal: 10, paddingVertical: 7, borderRadius: 14, backgroundColor: item.id === selectedKnowledgeBaseId ? '#dcfce7' : '#f2f4f7', marginRight: 6 }}><Text numberOfLines={1}>{item.name}</Text></Pressable>} ListEmptyComponent={<Text style={{ color: '#667085' }}>{label('mobileChat.noKnowledgeBases')}</Text>} />
    {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', paddingHorizontal: 12, paddingTop: 8 }}>{error}</Text> : null}
    <FlatList
      style={{ flex: 1, paddingHorizontal: 12 }}
      contentContainerStyle={{ paddingVertical: 12 }}
      data={displayMessages}
      keyExtractor={(item) => item.id}
      keyboardShouldPersistTaps="handled"
      ListHeaderComponent={<>
        {streamState.thinking ? <View style={{ backgroundColor: '#f8f9fc', padding: 10, borderRadius: 8, marginBottom: 8 }}><Text style={{ color: '#667085' }}>{formatMessage(runtime.locale, 'chat.thinkingAlt')}</Text><Text selectable style={{ color: '#667085' }}>{streamState.thinking}</Text></View> : null}
        {references.length ? <View style={{ marginBottom: 8 }}><Text style={{ fontWeight: '600' }}>{formatMessage(runtime.locale, 'chat.requestInfoTitle')}</Text>{references.flatMap((group) => group.items).map((reference) => <View key={reference.key} style={{ backgroundColor: '#f8f9fc', padding: 8, borderRadius: 8, marginTop: 4 }}><Text>{reference.title}</Text>{reference.snippet ? <Text selectable style={{ color: '#667085', fontSize: 12 }}>{reference.snippet}</Text> : null}</View>)}</View> : null}
        {pendingApprovals.map((approval) => <View key={approval.pendingId} style={{ backgroundColor: '#fff7ed', padding: 10, borderRadius: 8, marginBottom: 8 }}><Text>{label('mobileChat.toolApprovalRequired')}</Text><View style={{ flexDirection: 'row', gap: 12, marginTop: 8 }}><Pressable onPress={() => void approve(approval.pendingId, 'approve')}><Text style={{ color: '#16803c' }}>{label('mobileChat.approve')}</Text></Pressable><Pressable onPress={() => void approve(approval.pendingId, 'reject')}><Text style={{ color: '#b42318' }}>{label('mobileChat.reject')}</Text></Pressable></View></View>)}
        {pendingOAuthApprovals.map((approval) => <View key={approval.pendingId} style={{ backgroundColor: '#eef4ff', padding: 10, borderRadius: 8, marginBottom: 8 }}><Text>{label('mobileChat.mcpAuthorizationRequired')}{approval.serviceName ? ` · ${approval.serviceName}` : ''}</Text>{approval.toolName ? <Text style={{ color: '#667085', marginTop: 4 }}>{label('mobileChat.tool')}: {approval.toolName}</Text> : null}<View style={{ flexDirection: 'row', gap: 12, marginTop: 8 }}><Pressable onPress={() => void openOAuth(approval)}><Text style={{ color: '#2864dc' }}>{label('mobileChat.authorize')}</Text></Pressable><Pressable onPress={() => void cancelOAuth(approval)}><Text style={{ color: '#b42318' }}>{formatMessage(runtime.locale, 'common.cancel')}</Text></Pressable></View></View>)}
        {selectedSessionId ? <View style={{ backgroundColor: '#f8f9fc', padding: 10, borderRadius: 8, marginBottom: 8 }}><View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' }}><Text style={{ fontWeight: '600' }}>{label('mobileChat.steerQueue')}</Text><Pressable disabled={steerLoading} onPress={() => void loadSteerQueue(selectedSessionId)}><Text style={{ color: steerLoading ? '#98a2b3' : '#2864dc' }}>{steerLoading ? formatMessage(runtime.locale, 'common.loading') : formatMessage(runtime.locale, 'common.retry')}</Text></Pressable></View>{steerQueue.length === 0 ? <Text style={{ color: '#667085', marginTop: 6 }}>{label('mobileChat.noQueuedInstructions')}</Text> : steerQueue.map((item) => <View key={item.steer_id} style={{ backgroundColor: '#fff', padding: 8, borderRadius: 8, marginTop: 6 }}><Text selectable>{item.content}</Text><Text style={{ color: '#667085', fontSize: 12, marginTop: 4 }}>{item.delivery === 'inject' ? label('mobileChat.injectingNext') : label('mobileChat.afterCurrentResponse')}</Text><View style={{ flexDirection: 'row', gap: 12, marginTop: 6 }}>{item.delivery === 'after' ? <Pressable disabled={Boolean(steerBusy)} onPress={() => void promoteSteer(item)}><Text style={{ color: steerBusy === item.steer_id ? '#98a2b3' : '#2864dc' }}>{label('mobileChat.injectingNext')}</Text></Pressable> : null}<Pressable disabled={Boolean(steerBusy)} onPress={() => void removeSteer(item)}><Text style={{ color: steerBusy === item.steer_id ? '#98a2b3' : '#b42318' }}>{formatMessage(runtime.locale, 'common.delete')}</Text></Pressable></View></View>)}</View> : null}
      </>}
      renderItem={({ item }) => <View style={{ alignSelf: item.role === 'user' ? 'flex-end' : 'stretch', maxWidth: '92%', backgroundColor: item.role === 'user' ? '#eff6ff' : '#f8f9fc', padding: 10, borderRadius: 10, marginBottom: 8 }}><Text style={{ fontWeight: '600', marginBottom: 4 }}>{item.role}</Text><Text selectable>{item.content}</Text>{item.role === 'assistant' && item.is_completed === false ? <>{<Text style={{ color: '#667085', marginTop: 4 }}>{item.id === failedAssistantMessageId.current ? label('mobileChat.responseFailedRetry') : label('mobileChat.resuming')}</Text>}{item.id === failedAssistantMessageId.current ? (() => { const query = findRetryQuery(messages, item.id); return query ? <Pressable accessibilityRole="button" onPress={() => void send(query)}><Text style={{ color: '#2864dc', marginTop: 6, fontWeight: '600' }}>{formatMessage(runtime.locale, 'common.retry')}</Text></Pressable> : null; })() : null}</> : null}{normalizeArtifactList(selectMessageArtifacts(item)).map((artifact) => <Pressable key={`${artifact.index}-${artifact.fileName}`} onPress={() => void openArtifact(item.id, artifact)}><Text style={{ color: '#2864dc', marginTop: 6 }}>{label('mobileChat.file')}: {artifact.fileName} · {classifyNativeArtifactPreview(artifact).kind === 'download-only' ? formatMessage(runtime.locale, 'common.share') : label('mobileChat.preview')}</Text></Pressable>)}{item.role === 'assistant' && item.data ? <Text style={{ color: '#667085' }}>{normalizeToolResult({ output: item.data }).text}</Text> : null}</View>}
    />
    {attachments.length ? <ScrollView horizontal style={{ maxHeight: 38, paddingHorizontal: 12 }}><View style={{ flexDirection: 'row', gap: 8 }}>{attachments.map((attachment) => <View key={attachment.id} style={{ backgroundColor: '#f2f4f7', borderRadius: 12, paddingHorizontal: 10, paddingVertical: 6 }}><Text>{attachment.file_name} · {attachment.status}</Text></View>)}</View></ScrollView> : null}
    <View style={{ flexDirection: 'row', gap: 8, paddingHorizontal: 12, paddingVertical: 6 }}><Pressable onPress={() => setDraft(label('mobileChat.summarizePrompt'))}><Text style={{ color: '#2864dc', fontSize: 12 }}>{label('mobileChat.summarize')}</Text></Pressable><Pressable onPress={() => setDraft(label('mobileChat.relatedFilesPrompt'))}><Text style={{ color: '#2864dc', fontSize: 12 }}>{label('mobileChat.relatedFiles')}</Text></Pressable></View>
    <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} keyboardVerticalOffset={90} style={{ flexDirection: 'row', alignItems: 'flex-end', gap: 8, padding: 12, borderTopColor: '#eaecf0', borderTopWidth: 1 }}>
      <Pressable accessibilityLabel={label('mobileChat.attachFile')} onPress={() => void attach()}><Text style={{ color: attaching ? '#98a2b3' : '#2864dc' }}>{attaching ? '…' : '+'}</Text></Pressable>
      <TextInput accessibilityLabel={label('mobileChat.chatMessage')} value={draft} onChangeText={setDraft} multiline maxLength={20_000} placeholder={formatMessage(runtime.locale, 'input.placeholder')} style={{ flex: 1, maxHeight: 120, borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, paddingHorizontal: 10, paddingVertical: 8 }} />
      {sending ? <><Pressable accessibilityRole="button" onPress={() => void stop()}><Text style={{ color: '#b42318' }}>{formatMessage(runtime.locale, 'input.stopGeneration')}</Text></Pressable><Pressable accessibilityLabel={formatMessage(runtime.locale, 'input.steerCurrent')} accessibilityRole="button" disabled={steerButtonsDisabled} onPress={() => void enqueueSteer('inject')}><Text style={{ color: steerButtonsDisabled ? '#98a2b3' : '#2864dc' }}>{formatMessage(runtime.locale, 'input.steerCurrent')}</Text></Pressable><Pressable accessibilityLabel={formatMessage(runtime.locale, 'input.steerAfter')} accessibilityRole="button" disabled={steerButtonsDisabled} onPress={() => void enqueueSteer('after')}><Text style={{ color: steerButtonsDisabled ? '#98a2b3' : '#2864dc' }}>{formatMessage(runtime.locale, 'input.steerAfter')}</Text></Pressable></> : <Pressable accessibilityRole="button" disabled={!draft.trim()} onPress={() => void send()}><Text style={{ color: draft.trim() ? '#2864dc' : '#98a2b3', fontWeight: '600' }}>{formatMessage(runtime.locale, 'input.send')}</Text></Pressable>}
    </KeyboardAvoidingView>
    {artifactPreview ? <NativeArtifactPreview artifact={artifactPreview.artifact} uri={artifactPreview.uri} content={artifactPreview.content} loading={artifactPreview.loading} error={artifactPreview.error} onClose={() => setArtifactPreview(null)} labels={nativeArtifactPreviewLabels(runtime.locale)} onDownload={artifactPreview.uri ? () => { void shareNativeFile(artifactPreview.uri!).catch((cause) => setArtifactPreview((current) => current ? { ...current, error: errorText(cause, label('mobileChat.shareArtifactFailed')) } : current)); } : undefined} /> : null}
  </SafeAreaView>;
}
