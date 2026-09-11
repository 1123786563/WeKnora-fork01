import { useEffect, useMemo, useRef, useState } from 'react';
import type { AgentConfiguration, ChatMessage, ChatSession, WeKnoraClient } from '@weknora/api-client';
import { chatDraftKey } from '@weknora/domain/chat/draft';
import { initialChatStreamState, reduceChatStream } from '@weknora/domain/chat/reducer';
import { ChatPage, type ChatSubmission } from '@weknora/views';
import type { ScopeController } from '@weknora/domain/scope';
import { chatSessionIdFromPath } from './session-route.ts';
import { buildWebChatStreamOptions, initialAgentSelection } from './agent-selection.ts';
import { createWebTerminalController, webSocketTarget, type WebTerminalController, type WebTerminalSnapshot } from './terminal.ts';

interface ChatRoutePageProps {
  client: WeKnoraClient;
  scopeController: ScopeController;
  apiBaseUrl?: string;
}

function draftStorageKey(scope: ReturnType<ScopeController['current']>['scope'], sessionId: string): string {
  return JSON.stringify(chatDraftKey({ ...scope, sessionId }));
}

export function ChatRoutePage({ client, scopeController, apiBaseUrl = '' }: ChatRoutePageProps) {
  const scope = scopeController.current();
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [streamState, setStreamState] = useState(initialChatStreamState);
  const [agents, setAgents] = useState<AgentConfiguration[]>([]);
  const [disabledAgentIds, setDisabledAgentIds] = useState<string[]>([]);
  const [selectedAgentId, setSelectedAgentId] = useState(() => new URLSearchParams(window.location.search).get('agentId')?.trim() ?? '');
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(() => chatSessionIdFromPath(window.location.pathname));
  const [draft, setDraft] = useState('');
  const [loadingSessions, setLoadingSessions] = useState(true);
  const [loadingMessages, setLoadingMessages] = useState(false);
  const [error, setError] = useState<string | undefined>();
  const [terminal, setTerminal] = useState<WebTerminalSnapshot>({ status: 'idle', output: '' });
  const terminalController = useRef<WebTerminalController | null>(null);
  const storageKey = useMemo(
    () => selectedSessionId ? draftStorageKey(scope.scope, selectedSessionId) : null,
    [scope.scope, selectedSessionId],
  );

  useEffect(() => {
    let active = true;
    setLoadingSessions(true);
    void client.sessions.list({ page: 1, pageSize: 30, source: 'web', signal: scope.signal }).then(
      (result) => { if (active && scopeController.isCurrent(scope.scope)) setSessions(result.data); },
      (cause: unknown) => { if (active) setError(cause instanceof Error ? cause.message : 'Unable to load sessions'); },
    ).finally(() => { if (active) setLoadingSessions(false); });
    return () => { active = false; };
  }, [client, scope.signal, scope.scope, scopeController]);

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
      return;
    }
    let active = true;
    setLoadingMessages(true);
    setError(undefined);
    setStreamState(initialChatStreamState());
    void client.sessions.messages(selectedSessionId, { limit: 50, signal: scope.signal }).then(
      (result) => { if (active && scopeController.isCurrent(scope.scope)) setMessages(result); },
      (cause: unknown) => { if (active) setError(cause instanceof Error ? cause.message : 'Unable to load messages'); },
    ).finally(() => { if (active) setLoadingMessages(false); });
    return () => { active = false; };
  }, [client, selectedSessionId, scope.signal, scope.scope, scopeController]);

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
        setSelectedSessionId(null); setMessages([]); setStreamState(initialChatStreamState());
        window.history.pushState({}, '', '/platform/creatChat');
      }
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to delete conversation'); }
  }

  function updateDraft(value: string) {
    setDraft(value);
    if (storageKey) window.localStorage.setItem(storageKey, value);
  }

  async function openTerminal(): Promise<void> {
    if (!terminalController.current) throw new Error('Select a conversation before opening the terminal.');
    await terminalController.current.open({ provision: true, signal: scope.signal });
  }

  async function terminalInput(input: string): Promise<void> {
    terminalController.current?.sendInput(input);
  }

  function closeTerminal(): void {
    terminalController.current?.close();
  }

  async function send(submission: ChatSubmission): Promise<void> {
    if (!selectedSessionId) throw new Error('Create or select a conversation first.');
    const sessionId = selectedSessionId;
    setStreamState(initialChatStreamState());
    setMessages((current) => [...current, {
      id: `local-${Date.now()}`, session_id: sessionId, role: 'user', content: submission.content,
    }]);
    let runState = initialChatStreamState();
    const streamOptions = buildWebChatStreamOptions(sessionId, submission.content, selectedAgentId);
    await client.chat.stream(streamOptions, (event) => {
      runState = reduceChatStream(runState, event);
      setStreamState(runState);
      if (runState.phase === 'error') throw new Error(runState.error ?? 'Chat stream failed');
      if (runState.answer) setMessages((current) => [...current.filter((item) => item.id !== `stream-${sessionId}`), {
        id: `stream-${sessionId}`, session_id: sessionId, role: 'assistant', content: runState.answer,
        is_completed: runState.phase === 'completed',
      }]);
    });
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
    terminal={selectedSessionId ? terminal : undefined}
    onOpenTerminal={selectedSessionId ? openTerminal : undefined}
    onTerminalInput={selectedSessionId ? terminalInput : undefined}
    onCloseTerminal={selectedSessionId ? closeTerminal : undefined}
    stream={{ phase: streamState.phase, thinking: streamState.thinking, references: streamState.references, toolCalls: Object.values(streamState.toolCalls) }}
    send={send}
  />;
}
