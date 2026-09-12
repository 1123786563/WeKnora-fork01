import { useEffect, useMemo, useRef, useState } from 'react';
import type { AgentConfiguration, ChatMessage, ChatSession, MessageSuggestionSet, WeKnoraClient } from '@weknora/api-client';
import { chatDraftKey } from '@weknora/domain/chat/draft';
import { initialChatStreamState, reduceChatStream } from '@weknora/domain/chat/reducer';
import { appendMessages, hasOlderMessages, sessionGroups, sessionPageCount } from '@weknora/domain/chat/session-state';
import { ChatPage, type ChatSubmission } from '@weknora/views';
import type { ScopeController } from '@weknora/domain/scope';
import { chatSessionIdFromPath } from './session-route.ts';
import { buildWebChatStreamOptions, initialAgentSelection } from './agent-selection.ts';
import { createWebTerminalController, webSocketTarget, type WebTerminalController, type WebTerminalSnapshot } from './terminal.ts';
import { saveArtifactDownload } from './artifact-download.ts';
import { externalCitationTarget } from './citation.ts';

interface ChatRoutePageProps {
  client: WeKnoraClient;
  scopeController: ScopeController;
  apiBaseUrl?: string;
  knowledgeBaseId?: string;
  canViewChannelSessions?: boolean;
}

function draftStorageKey(scope: ReturnType<ScopeController['current']>['scope'], sessionId: string): string {
  return JSON.stringify(chatDraftKey({ ...scope, sessionId }));
}

export function ChatRoutePage({ client, scopeController, apiBaseUrl = '', knowledgeBaseId, canViewChannelSessions = false }: ChatRoutePageProps) {
  const scope = scopeController.current();
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [sessionSource, setSessionSource] = useState('web');
  const [sessionKeyword, setSessionKeyword] = useState('');
  const [sessionPage, setSessionPage] = useState(1);
  const [sessionPageCountValue, setSessionPageCountValue] = useState(1);
  const [sessionGroupMode, setSessionGroupMode] = useState<'none' | 'date'>('none');
  const [streamState, setStreamState] = useState(initialChatStreamState);
  const [agents, setAgents] = useState<AgentConfiguration[]>([]);
  const [disabledAgentIds, setDisabledAgentIds] = useState<string[]>([]);
  const [selectedAgentId, setSelectedAgentId] = useState(() => new URLSearchParams(window.location.search).get('agentId')?.trim() ?? '');
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(() => chatSessionIdFromPath(window.location.pathname));
  const selectedSessionIdRef = useRef(selectedSessionId);
  const chatRunIdRef = useRef(0);
  const sendInFlightRef = useRef(false);
  const [draft, setDraft] = useState('');
  const [loadingSessions, setLoadingSessions] = useState(true);
  const [loadingMessages, setLoadingMessages] = useState(false);
  const [loadingOlderMessages, setLoadingOlderMessages] = useState(false);
  const [hasMoreMessages, setHasMoreMessages] = useState(false);
  const [suggestions, setSuggestions] = useState<MessageSuggestionSet | undefined>();
  const suggestionForMessage = useRef<string | null>(null);
  const impressionForSuggestion = useRef<string | null>(null);
  const [error, setError] = useState<string | undefined>();
  const [terminal, setTerminal] = useState<WebTerminalSnapshot>({ status: 'idle', output: '' });
  const terminalController = useRef<WebTerminalController | null>(null);
  const storageKey = useMemo(
    () => selectedSessionId ? draftStorageKey(scope.scope, selectedSessionId) : null,
    [scope.scope, selectedSessionId],
  );

  useEffect(() => {
    selectedSessionIdRef.current = selectedSessionId;
  }, [selectedSessionId]);

  useEffect(() => {
    let active = true;
    setLoadingSessions(true);
    void client.sessions.list({ page: sessionPage, pageSize: 30, source: sessionSource || undefined, keyword: sessionKeyword || undefined, signal: scope.signal }).then(
      (result) => {
        if (active && scopeController.isCurrent(scope.scope)) {
          setSessions(result.data);
          const pageCount = sessionPageCount(result.total, result.page_size);
          setSessionPageCountValue(pageCount);
          if (result.page !== sessionPage) setSessionPage(result.page);
        }
      },
      (cause: unknown) => { if (active) setError(cause instanceof Error ? cause.message : 'Unable to load sessions'); },
    ).finally(() => { if (active) setLoadingSessions(false); });
    return () => { active = false; };
  }, [client, scope.signal, scope.scope, scopeController, sessionPage, sessionSource, sessionKeyword]);

  useEffect(() => {
    let active = true;
    void client.configuration.agents.listWithState({ creator: 'all', signal: scope.signal }).then(
      (result) => {
        if (!active || !scopeController.isCurrent(scope.scope)) return;
        setAgents(result.items);
        setDisabledAgentIds(result.disabledOwnAgentIds);
        setSelectedAgentId((current) => initialAgentSelection(`?agentId=${encodeURIComponent(current)}`, result.items, result.disabledOwnAgentIds));
      },
      (cause: unknown) => { if (active) setError(cause instanceof Error ? cause.message : 'Unable to load agents'); },
    );
    return () => { active = false; };
  }, [client, scope.signal, scope.scope, scopeController]);

  useEffect(() => {
    if (!selectedSessionId) {
      setMessages([]);
      setStreamState(initialChatStreamState());
      setHasMoreMessages(false);
      setSuggestions(undefined);
      suggestionForMessage.current = null;
      impressionForSuggestion.current = null;
      return;
    }
    let active = true;
    setSuggestions(undefined);
    suggestionForMessage.current = null;
    impressionForSuggestion.current = null;
    setLoadingMessages(true);
    setError(undefined);
    setStreamState(initialChatStreamState());
    void client.sessions.messages(selectedSessionId, { limit: 50, signal: scope.signal }).then(
      (result) => {
        if (active && scopeController.isCurrent(scope.scope)) {
          setMessages(appendMessages([], result));
          setHasMoreMessages(hasOlderMessages(result, 50));
          const assistant = result.filter((message) => message.role === 'assistant' && message.is_completed).at(-1);
          if (assistant) {
            suggestionForMessage.current = assistant.id;
            void loadSuggestions(selectedSessionId, assistant.id, false, scope.signal);
          }
        }
      },
      (cause: unknown) => { if (active) setError(cause instanceof Error ? cause.message : 'Unable to load messages'); },
    ).finally(() => { if (active) setLoadingMessages(false); });
    return () => { active = false; };
  }, [client, selectedSessionId, scope.signal, scope.scope, scopeController]);

  useEffect(() => {
    if (!suggestions || suggestions.status !== 'ready' || !selectedSessionId) return;
    const key = `${selectedSessionId}:${suggestions.id}`;
    if (impressionForSuggestion.current === key) return;
    impressionForSuggestion.current = key;
    void client.chat.suggestions.recordEvent(selectedSessionId, suggestions.id, 'impression', '', scope.signal).catch(() => undefined);
  }, [client, scope.signal, selectedSessionId, suggestions]);

  async function loadSuggestions(sessionId: string, messageId: string, ensure: boolean, signal?: AbortSignal): Promise<void> {
    try {
      const result = ensure
        ? await client.chat.suggestions.ensure(sessionId, messageId, false, signal)
        : await client.chat.suggestions.get(sessionId, messageId, signal);
      let current = result;
      for (let attempt = 0; current.status === 'generating' && attempt < 120; attempt += 1) {
        await new Promise<void>((resolve) => window.setTimeout(resolve, 1000));
        if (signal?.aborted || !scopeController.isCurrent(scope.scope) || selectedSessionIdRef.current !== sessionId) return;
        current = await client.chat.suggestions.get(sessionId, messageId, signal);
      }
      if (scopeController.isCurrent(scope.scope) && selectedSessionIdRef.current === sessionId && suggestionForMessage.current === messageId) {
        setSuggestions(current.status === 'ready' ? current : undefined);
      }
    } catch {
      // A deployment may have suggestion generation disabled or may not have
      // generated a set for an older message. This is not a chat-history error.
      if (scopeController.isCurrent(scope.scope) && selectedSessionIdRef.current === sessionId && suggestionForMessage.current === messageId) setSuggestions(undefined);
    }
  }

  async function loadOlderMessages(): Promise<void> {
    const sessionId = selectedSessionId;
    if (!sessionId || loadingOlderMessages || !hasMoreMessages) return;
    const oldest = messages[0]?.created_at;
    if (!oldest) { setHasMoreMessages(false); return; }
    setLoadingOlderMessages(true);
    try {
      const oldestId = messages[0]?.id;
      const batch = await client.sessions.messages(sessionId, { beforeTime: oldest, limit: 50, signal: scope.signal });
      if (!scopeController.isCurrent(scope.scope) || selectedSessionIdRef.current !== sessionId) return;
      setMessages((current) => appendMessages(current, batch));
      setHasMoreMessages(hasOlderMessages(batch, 50) && batch[0]?.id !== oldestId);
    } catch (cause) {
      if (scopeController.isCurrent(scope.scope)) setError(cause instanceof Error ? cause.message : 'Unable to load older messages');
    } finally { setLoadingOlderMessages(false); }
  }

  useEffect(() => {
    setDraft(storageKey ? window.localStorage.getItem(storageKey) ?? '' : '');
  }, [storageKey]);

  useEffect(() => {
    terminalController.current?.close();
    terminalController.current = null;
    setTerminal({ status: 'idle', output: '' });
    if (!selectedSessionId) return;
    const target = webSocketTarget(apiBaseUrl, window.location.origin);
    const controller = createWebTerminalController({
      sessionId: selectedSessionId,
      wsOrigin: target.origin,
      basePath: target.basePath,
      issueTicket: (signal) => client.sandbox.issueTicket(selectedSessionId, signal),
      socketFactory: (url) => new WebSocket(url) as unknown as import('./terminal.ts').WebTerminalSocket,
    });
    terminalController.current = controller;
    const unsubscribe = controller.subscribe(setTerminal);
    return () => {
      unsubscribe();
      controller.close();
      if (terminalController.current === controller) terminalController.current = null;
    };
  }, [apiBaseUrl, client, scope.signal, selectedSessionId]);

  function selectSession(sessionId: string) {
    chatRunIdRef.current += 1;
    selectedSessionIdRef.current = sessionId;
    setSelectedSessionId(sessionId);
    window.history.pushState({}, '', `/platform/chat/${encodeURIComponent(sessionId)}`);
  }

  function selectAgent(agentId: string) {
    setSelectedAgentId(agentId);
    const url = new URL(window.location.href);
    if (agentId) url.searchParams.set('agentId', agentId);
    else url.searchParams.delete('agentId');
    window.history.replaceState({}, '', `${url.pathname}${url.search}`);
  }

  function changeSessionSource(source: string): void {
    setSessionSource(source);
    setSessionPage(1);
  }

  function changeSessionKeyword(keyword: string): void {
    setSessionKeyword(keyword);
    setSessionPage(1);
  }

  async function resolveToolApproval(pendingId: string, decision: 'approve' | 'reject'): Promise<void> {
    await client.chat.approvals.resolveTool(pendingId, { decision }, scope.signal);
  }

  async function cancelOAuth(pendingId: string): Promise<void> {
    await client.chat.approvals.cancelOAuth(pendingId, scope.signal);
  }

  async function steer(content: string): Promise<void> {
    if (!selectedSessionId) throw new Error('Create or select a conversation first.');
    await client.chat.steer.enqueue(selectedSessionId, { query: content, delivery: 'after', channel: 'web' }, scope.signal);
  }

  async function downloadArtifact(messageId: string, artifactIndex: number): Promise<void> {
    const sessionId = selectedSessionIdRef.current;
    if (!sessionId) throw new Error('Select a conversation before downloading an artifact.');
    try {
      const artifacts = await client.chat.artifacts.message(sessionId, messageId, scope.signal);
      const artifact = artifacts.find((item) => item.index === artifactIndex);
      if (!artifact) throw new Error('Artifact is no longer available.');
      const response = await client.chat.artifacts.download(sessionId, messageId, artifact.index, scope.signal);
      await saveArtifactDownload(artifact, response, async (content, filename) => {
        const blob = content instanceof Blob ? content : new Blob([content]);
        const url = URL.createObjectURL(blob);
        const anchor = document.createElement('a');
        anchor.href = url;
        anchor.download = filename;
        anchor.click();
        URL.revokeObjectURL(url);
      });
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Unable to download artifact');
    }
  }

  async function authorizeOAuth(pendingId: string, serviceId: string): Promise<void> {
    const authorization = await client.configuration.mcp.oauth.authorizeUrl(serviceId, {
      redirectURI: `${window.location.origin}/api/v1/mcp-oauth/callback`,
      frontendRedirect: `${window.location.origin}/`,
    }, scope.signal);
    if (!authorization.authorizationUrl || !authorization.authorizationAttempt) throw new Error('MCP authorization could not be started.');
    const popup = window.open(authorization.authorizationUrl, 'mcp_oauth', 'width=600,height=720');
    if (!popup) throw new Error('MCP authorization popup was blocked.');
    try {
      for (let attempt = 0; attempt < 80; attempt += 1) {
        const status = await client.configuration.mcp.oauth.status(serviceId, authorization.authorizationAttempt, scope.signal);
        if (status.authorized) {
          await client.chat.approvals.resolveOAuth(pendingId, { serviceId, decision: 'authorize' }, scope.signal);
          return;
        }
        if (popup.closed) throw new Error('MCP authorization was cancelled before completion.');
        await new Promise<void>((resolve) => window.setTimeout(resolve, 1500));
      }
      throw new Error('MCP authorization timed out.');
    } finally {
      try { popup.close(); } catch { /* cross-origin popup close may throw */ }
    }
  }

  async function createSession() {
    setError(undefined);
    try {
      const session = await client.sessions.create();
      setSessions((current) => [session, ...current.filter((item) => item.id !== session.id)]);
      selectSession(session.id);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Unable to create a session');
    }
  }

  async function renameSession(sessionId: string): Promise<void> {
    const current = sessions.find((session) => session.id === sessionId);
    const title = window.prompt('Conversation title', current?.title ?? '')?.trim();
    if (!title || title === current?.title) return;
    try {
      const updated = await client.sessions.update(sessionId, { title, description: current?.description }, scope.signal);
      setSessions((items) => items.map((session) => session.id === sessionId ? updated : session));
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to rename conversation'); }
  }

  async function toggleSessionPin(sessionId: string, pinned: boolean): Promise<void> {
    try {
      await (pinned ? client.sessions.pin(sessionId, scope.signal) : client.sessions.unpin(sessionId, scope.signal));
      setSessions((items) => items.map((session) => session.id === sessionId ? { ...session, is_pinned: pinned } : session));
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to update conversation pin'); }
  }

  async function deleteSession(sessionId: string): Promise<void> {
    if (!window.confirm('Delete this conversation?')) return;
    try {
      await client.sessions.remove(sessionId, scope.signal);
      setSessions((items) => items.filter((session) => session.id !== sessionId));
      if (selectedSessionId === sessionId) {
        selectedSessionIdRef.current = null;
        setSelectedSessionId(null); setMessages([]); setStreamState(initialChatStreamState());
        window.history.pushState({}, '', '/platform/creatChat');
      }
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to delete conversation'); }
  }

  async function clearMessages(): Promise<void> {
    if (!selectedSessionId || !window.confirm('Clear messages in this conversation?')) return;
    try {
      await client.sessions.clear(selectedSessionId, scope.signal);
      setMessages([]);
      setSuggestions(undefined);
      setHasMoreMessages(false);
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to clear messages'); }
  }

  function updateDraft(value: string) {
    setDraft(value);
    if (storageKey) window.localStorage.setItem(storageKey, value);
  }

  function selectSuggestion(questionId: string, text: string): void {
    if (suggestions && selectedSessionId) void client.chat.suggestions.recordEvent(selectedSessionId, suggestions.id, 'click', questionId, scope.signal).catch(() => undefined);
    updateDraft(text);
  }

  function refreshSuggestions(): void {
    const messageId = streamState.assistantMessageId ?? messages.filter((message) => message.role === 'assistant' && message.is_completed).at(-1)?.id;
    if (messageId && selectedSessionId) { suggestionForMessage.current = messageId; void loadSuggestions(selectedSessionId, messageId, true, scope.signal); }
  }

  function dismissSuggestions(): void {
    if (suggestions && selectedSessionId) void client.chat.suggestions.recordEvent(selectedSessionId, suggestions.id, 'dismiss', '', scope.signal).catch(() => undefined);
    setSuggestions(undefined);
  }

  function openCitation(citationId: string): void {
    const target = externalCitationTarget(citationId);
    if (target) window.open(target, '_blank', 'noopener,noreferrer');
  }

  async function openTerminal(): Promise<void> {
    if (!terminalController.current) throw new Error('Select a conversation before opening the terminal.');
    await terminalController.current.open({ provision: true, signal: scope.signal });
  }

  async function terminalInput(input: string): Promise<void> {
    terminalController.current?.sendInput(input);
  }

  function terminalResize(cols: number, rows: number): void {
    terminalController.current?.resize(cols, rows);
  }

  function closeTerminal(): void {
    terminalController.current?.close();
  }

  async function send(submission: ChatSubmission): Promise<void> {
    if (sendInFlightRef.current) throw new Error('A chat request is already running.');
    sendInFlightRef.current = true;
    try {
      let sessionId = selectedSessionId;
      if (!sessionId) {
        const created = await client.sessions.create({ title: submission.content.slice(0, 80) });
        setSessions((current) => [created, ...current.filter((item) => item.id !== created.id)]);
        sessionId = created.id;
        selectSession(sessionId);
      }
      const runId = ++chatRunIdRef.current;
      setStreamState(initialChatStreamState());
      let runState = initialChatStreamState();
      const streamOptions = buildWebChatStreamOptions(sessionId, submission.content, selectedAgentId, knowledgeBaseId);
      await client.chat.stream(streamOptions, (event) => {
        if (runId !== chatRunIdRef.current || selectedSessionIdRef.current !== sessionId) return;
        runState = reduceChatStream(runState, event);
        setStreamState(runState);
        if (runState.phase === 'error') throw new Error(runState.error ?? 'Chat stream failed');
        if (runState.answer) setMessages((current) => [...current.filter((item) => item.id !== `stream-${sessionId}`), {
          id: `stream-${sessionId}`, session_id: sessionId, role: 'assistant', content: runState.answer,
          is_completed: runState.phase === 'completed',
        }]);
        if (runState.phase === 'completed' && runState.assistantMessageId && runState.assistantMessageId !== suggestionForMessage.current) {
          suggestionForMessage.current = runState.assistantMessageId;
          void loadSuggestions(sessionId, runState.assistantMessageId, true, scope.signal);
        }
      });
      if (runId !== chatRunIdRef.current || selectedSessionIdRef.current !== sessionId) return;
      // The server persists the user message before opening the stream. Refresh
      // the bounded history after a successful turn so the UI replaces the
      // transient assistant row with authoritative user/assistant message ids;
      // the pending composer remains the sole visible sending/failed row.
      try {
        const persisted = await client.sessions.messages(sessionId, { limit: 50, signal: scope.signal });
        if (scopeController.isCurrent(scope.scope) && selectedSessionIdRef.current === sessionId) setMessages(appendMessages([], persisted));
      } catch {
        // The streamed answer remains visible if the post-turn history refresh
        // is unavailable; a later session selection reloads authoritative data.
      }
    } finally {
      sendInFlightRef.current = false;
    }
  }

  return <ChatPage
    sessions={sessions}
    selectedSessionId={selectedSessionId}
    messages={messages}
    draft={draft}
    loadingSessions={loadingSessions}
    loadingMessages={loadingMessages}
    error={error}
    onSelectSession={selectSession}
    onCreateSession={() => void createSession()}
    onDraftChange={updateDraft}
    agents={agents.map((agent) => ({ id: agent.id, name: agent.name, disabled: disabledAgentIds.includes(agent.id) }))}
    selectedAgentId={selectedAgentId}
    onAgentChange={selectAgent}
    toolApprovals={Object.values(streamState.approvals)}
    oauthApprovals={Object.values(streamState.oauthApprovals)}
    onResolveToolApproval={resolveToolApproval}
    onAuthorizeOAuth={authorizeOAuth}
    onCancelOAuth={cancelOAuth}
    onSteer={steer}
    onRenameSession={renameSession}
    onToggleSessionPin={toggleSessionPin}
    onDeleteSession={deleteSession}
    sessionGroups={sessionGroups(sessions, new Date(), sessionGroupMode)}
    sessionSource={sessionSource}
    sessionSourceOptions={canViewChannelSessions ? [{ value: '', label: 'All sources' }, { value: 'web', label: 'Web' }, { value: 'embed', label: 'Embed' }, { value: 'api', label: 'API' }, { value: 'feishu', label: 'Feishu' }, { value: 'wechat', label: 'WeChat' }, { value: 'slack', label: 'Slack' }] : [{ value: 'web', label: 'Web' }]}
    onSessionSourceChange={changeSessionSource}
    sessionKeyword={sessionKeyword}
    onSessionKeywordChange={changeSessionKeyword}
    sessionPage={sessionPage}
    sessionPageCount={sessionPageCountValue}
    onSessionPageChange={setSessionPage}
    sessionGroupMode={sessionGroupMode}
    onSessionGroupModeChange={setSessionGroupMode}
    onClearSession={clearMessages}
    loadingOlderMessages={loadingOlderMessages}
    hasMoreMessages={hasMoreMessages}
    onLoadOlderMessages={() => void loadOlderMessages()}
    suggestions={suggestions}
    onSuggestionClick={selectSuggestion}
    onRefreshSuggestions={refreshSuggestions}
    onDismissSuggestions={dismissSuggestions}
    onCitationClick={openCitation}
    onArtifactDownload={downloadArtifact}
    terminal={selectedSessionId ? terminal : undefined}
    onOpenTerminal={selectedSessionId ? openTerminal : undefined}
    onTerminalInput={selectedSessionId ? terminalInput : undefined}
    onTerminalResize={selectedSessionId ? terminalResize : undefined}
    onCloseTerminal={selectedSessionId ? closeTerminal : undefined}
    stream={{ phase: streamState.phase, thinking: streamState.thinking, references: streamState.references, toolCalls: Object.values(streamState.toolCalls) }}
    send={send}
  />;
}
