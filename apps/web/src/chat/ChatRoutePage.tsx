import { useEffect, useMemo, useState } from 'react';
import type { AgentConfiguration, ChatMessage, ChatSession, WeKnoraClient } from '@weknora/api-client';
import { chatDraftKey } from '@weknora/domain/chat/draft';
import { initialChatStreamState, reduceChatStream } from '@weknora/domain/chat/reducer';
import { ChatPage, type ChatSubmission } from '@weknora/views';
import type { ScopeController } from '@weknora/domain/scope';
import { chatSessionIdFromPath } from './session-route.ts';
import { buildWebChatStreamOptions, initialAgentSelection } from './agent-selection.ts';

interface ChatRoutePageProps {
  client: WeKnoraClient;
  scopeController: ScopeController;
}

function draftStorageKey(scope: ReturnType<ScopeController['current']>['scope'], sessionId: string): string {
  return JSON.stringify(chatDraftKey({ ...scope, sessionId }));
}

export function ChatRoutePage({ client, scopeController }: ChatRoutePageProps) {
  const scope = scopeController.current();
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [agents, setAgents] = useState<AgentConfiguration[]>([]);
  const [disabledAgentIds, setDisabledAgentIds] = useState<string[]>([]);
  const [selectedAgentId, setSelectedAgentId] = useState(() => new URLSearchParams(window.location.search).get('agentId')?.trim() ?? '');
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(() => chatSessionIdFromPath(window.location.pathname));
  const [draft, setDraft] = useState('');
  const [loadingSessions, setLoadingSessions] = useState(true);
  const [loadingMessages, setLoadingMessages] = useState(false);
  const [error, setError] = useState<string | undefined>();
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
      return;
    }
    let active = true;
    setLoadingMessages(true);
    setError(undefined);
    void client.sessions.messages(selectedSessionId, { limit: 50, signal: scope.signal }).then(
      (result) => { if (active && scopeController.isCurrent(scope.scope)) setMessages(result); },
      (cause: unknown) => { if (active) setError(cause instanceof Error ? cause.message : 'Unable to load messages'); },
    ).finally(() => { if (active) setLoadingMessages(false); });
    return () => { active = false; };
  }, [client, selectedSessionId, scope.signal, scope.scope, scopeController]);

  useEffect(() => {
    setDraft(storageKey ? window.localStorage.getItem(storageKey) ?? '' : '');
  }, [storageKey]);

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

  function updateDraft(value: string) {
    setDraft(value);
    if (storageKey) window.localStorage.setItem(storageKey, value);
  }

  async function send(submission: ChatSubmission): Promise<void> {
    if (!selectedSessionId) throw new Error('Create or select a conversation first.');
    const sessionId = selectedSessionId;
    setMessages((current) => [...current, {
      id: `local-${Date.now()}`, session_id: sessionId, role: 'user', content: submission.content,
    }]);
    let streamState = initialChatStreamState();
    const streamOptions = buildWebChatStreamOptions(sessionId, submission.content, selectedAgentId);
    await client.chat.stream(streamOptions, (event) => {
      streamState = reduceChatStream(streamState, event);
      if (streamState.phase === 'error') throw new Error(streamState.error ?? 'Chat stream failed');
      if (streamState.answer) setMessages((current) => [...current.filter((item) => item.id !== `stream-${sessionId}`), {
        id: `stream-${sessionId}`, session_id: sessionId, role: 'assistant', content: streamState.answer,
        is_completed: streamState.phase === 'completed',
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
    send={send}
  />;
}
