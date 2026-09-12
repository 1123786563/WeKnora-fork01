import { useEffect, useMemo, useRef, useState } from 'react';
import type { ChatMessage, ChatSession, MessageSuggestionSet } from '@weknora/contracts';
import { ChatComposer, type ChatSubmission } from './composer.tsx';
import { MessageList, type PendingChatMessage } from './message-list.tsx';
import { SessionSidebar } from './session-sidebar.tsx';
import { ReferenceList } from './reference-list.tsx';
import { ToolResultView } from './tool-result.tsx';

export interface ChatAgentOption {
  id: string;
  name: string;
  disabled?: boolean;
}

export interface ChatToolApprovalPrompt {
  pendingId: string;
  toolName?: string;
  status: 'pending' | 'resolved';
  decision?: string;
}

export interface ChatOAuthApprovalPrompt {
  pendingId: string;
  serviceId?: string;
  serviceName?: string;
  toolName?: string;
  status: 'pending' | 'resolved';
  authorized?: boolean;
  reason?: string;
}

export interface ChatToolCallView {
  id: string;
  name?: string;
  status: 'pending' | 'completed' | 'failed';
  result?: unknown;
}

export interface ChatStreamPresentation {
  phase: 'idle' | 'streaming' | 'completed' | 'stopped' | 'error';
  thinking: string;
  references: readonly unknown[];
  toolCalls: readonly ChatToolCallView[];
}

export interface ChatTerminalView {
  status: string;
  output: string;
}

export interface ChatPageProps {
  sessions: readonly ChatSession[];
  selectedSessionId: string | null;
  messages: readonly ChatMessage[];
  draft: string;
  loadingSessions?: boolean;
  loadingMessages?: boolean;
  error?: string;
  onSelectSession(sessionId: string): void;
  onCreateSession(): void;
  onDraftChange(value: string): void;
  send(submission: ChatSubmission): Promise<void>;
  agents?: readonly ChatAgentOption[];
  selectedAgentId?: string;
  onAgentChange?(agentId: string): void;
  toolApprovals?: readonly ChatToolApprovalPrompt[];
  oauthApprovals?: readonly ChatOAuthApprovalPrompt[];
  onResolveToolApproval?(pendingId: string, decision: 'approve' | 'reject'): Promise<void>;
  onAuthorizeOAuth?(pendingId: string, serviceId: string): Promise<void>;
  onCancelOAuth?(pendingId: string): Promise<void>;
  onSteer?(content: string): Promise<void>;
  stream?: ChatStreamPresentation;
  onRenameSession?(sessionId: string): Promise<void>;
  onToggleSessionPin?(sessionId: string, pinned: boolean): Promise<void>;
  onDeleteSession?(sessionId: string): Promise<void>;
  sessionGroups?: readonly { key: string; label?: string; items: readonly ChatSession[] }[];
  sessionSource?: string;
  sessionSourceOptions?: readonly { value: string; label: string }[];
  onSessionSourceChange?(source: string): void;
  sessionGroupMode?: 'none' | 'date';
  onSessionGroupModeChange?(mode: 'none' | 'date'): void;
  sessionKeyword?: string;
  onSessionKeywordChange?(keyword: string): void;
  sessionPage?: number;
  sessionPageCount?: number;
  onSessionPageChange?(page: number): void;
  onClearSession?(): Promise<void>;
  loadingOlderMessages?: boolean;
  hasMoreMessages?: boolean;
  onLoadOlderMessages?(): void;
  suggestions?: MessageSuggestionSet;
  onSuggestionClick?(questionId: string, text: string): void;
  onRefreshSuggestions?(): void;
  onDismissSuggestions?(): void;
  onCitationClick?(citationId: string): void;
  onArtifactDownload?(messageId: string, artifactIndex: number): Promise<void>;
  terminal?: ChatTerminalView;
  onOpenTerminal?(): Promise<void>;
  onTerminalInput?(input: string): Promise<void>;
  onTerminalResize?(cols: number, rows: number): void;
  onCloseTerminal?(): void;
}

export function messageReferenceValues(messages: readonly ChatMessage[]): unknown[] {
  const references: unknown[] = [];
  for (const message of messages) {
    const row = message as Record<string, unknown>;
    for (const key of ['knowledge_references', 'references']) {
      const value = row[key];
      if (Array.isArray(value)) references.push(...value);
    }
  }
  return references;
}

function LiveResponse({ stream }: { stream: ChatStreamPresentation }) {
  const hasDetails = Boolean(stream.thinking) || stream.toolCalls.length > 0;
  if (!hasDetails) return null;
  return <section aria-label="Live response" className="wk-chat-live-response">
    <p role="status">Status: {stream.phase}</p>
    {stream.thinking ? <details open><summary>Thinking</summary><p>{stream.thinking}</p></details> : null}
    {stream.toolCalls.length > 0 ? <div><h2>Tool calls</h2><ul className="wk-list">{stream.toolCalls.map((tool) => <li key={tool.id}><strong>{tool.name ?? tool.id}</strong><small>{tool.status}</small>{tool.result === undefined ? null : <ToolResultView toolCall={tool} />}</li>)}</ul></div> : null}
  </section>;
}

function ChatActionCards(props: Pick<ChatPageProps, 'toolApprovals' | 'oauthApprovals' | 'onResolveToolApproval' | 'onAuthorizeOAuth' | 'onCancelOAuth'>) {
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const toolApprovals = props.toolApprovals ?? [];
  const oauthApprovals = props.oauthApprovals ?? [];
  if (toolApprovals.length === 0 && oauthApprovals.length === 0) return null;

  async function run(key: string, action: () => Promise<void>) {
    setBusy(key); setError(null);
    try { await action(); } catch (cause) { setError(cause instanceof Error ? cause.message : 'Chat action failed'); } finally { setBusy(null); }
  }

  return <section aria-label="Chat actions" className="wk-chat-actions">
    <h2>Actions</h2>
    {error ? <p role="alert">{error}</p> : null}
    {toolApprovals.map((approval) => <div key={`tool-${approval.pendingId}`} className="wk-chat-action-card">
      <strong>Tool approval: {approval.toolName ?? 'unknown tool'}</strong>
      {approval.status === 'pending' && props.onResolveToolApproval ? <div className="wk-list-actions"><button type="button" disabled={busy !== null} onClick={() => void run(approval.pendingId, () => props.onResolveToolApproval!(approval.pendingId, 'approve'))}>Approve {approval.toolName ?? 'tool'}</button><button type="button" disabled={busy !== null} onClick={() => void run(approval.pendingId, () => props.onResolveToolApproval!(approval.pendingId, 'reject'))}>Reject</button></div> : <small>{approval.decision ? `Resolved: ${approval.decision}` : 'Resolved'}</small>}
    </div>)}
    {oauthApprovals.map((approval) => <div key={`oauth-${approval.pendingId}`} className="wk-chat-action-card">
      <strong>MCP authorization: {approval.serviceName ?? approval.serviceId ?? 'service'}</strong>
      {approval.toolName ? <small>Tool: {approval.toolName}</small> : null}
      {approval.status === 'pending' && approval.serviceId && props.onAuthorizeOAuth && props.onCancelOAuth ? <div className="wk-list-actions"><button type="button" disabled={busy !== null} onClick={() => void run(approval.pendingId, () => props.onAuthorizeOAuth!(approval.pendingId, approval.serviceId!))}>Authorize {approval.serviceName ?? 'service'}</button><button type="button" disabled={busy !== null} onClick={() => void run(approval.pendingId, () => props.onCancelOAuth!(approval.pendingId))}>Cancel</button></div> : <small>{approval.authorized ? 'Authorized' : approval.reason ?? 'Resolved'}</small>}
    </div>)}
  </section>;
}

function SteerComposer({ onSteer }: { onSteer: (content: string) => Promise<void> }) {
  const [draft, setDraft] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const content = draft.trim();
    if (!content) return;
    setBusy(true); setError(null);
    try { await onSteer(content); setDraft(''); } catch (cause) { setError(cause instanceof Error ? cause.message : 'Follow-up failed'); } finally { setBusy(false); }
  }

  return <form className="wk-chat-steer" onSubmit={(event) => void submit(event)}>
    <label htmlFor="wk-chat-steer-draft">Follow-up while running</label>
    <textarea id="wk-chat-steer-draft" rows={2} value={draft} onChange={(event) => setDraft(event.target.value)} disabled={busy} />
    {error ? <p role="alert">{error}</p> : null}
    <button type="submit" disabled={busy || !draft.trim()}>Queue follow-up</button>
  </form>;
}

function TerminalPanel(props: Pick<ChatPageProps, 'terminal' | 'onOpenTerminal' | 'onTerminalInput' | 'onTerminalResize' | 'onCloseTerminal'>) {
  const panelRef = useRef<HTMLElement>(null);
  const [input, setInput] = useState('');
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (!props.terminal || !props.onTerminalResize || typeof ResizeObserver === 'undefined' || !panelRef.current) return;
    const notify = () => {
      const element = panelRef.current;
      if (!element) return;
      props.onTerminalResize!(Math.max(1, Math.floor(element.clientWidth / 8)), Math.max(1, Math.floor(element.clientHeight / 16)));
    };
    const observer = new ResizeObserver(notify);
    observer.observe(panelRef.current);
    notify();
    return () => observer.disconnect();
  }, [props.onTerminalResize, props.terminal?.status]);
  if (!props.terminal && !props.onOpenTerminal) return null;
  async function sendInput(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!input.trim() || !props.onTerminalInput) return;
    setBusy(true);
    try { await props.onTerminalInput(input); setInput(''); } finally { setBusy(false); }
  }
  return <section ref={panelRef} aria-label="Sandbox terminal" className="wk-chat-terminal">
    <div className="wk-settings-panel-heading"><h2>Sandbox terminal</h2><span role="status">{props.terminal?.status ?? 'idle'}</span></div>
    {props.onOpenTerminal ? <button type="button" onClick={() => void props.onOpenTerminal!()}>Open terminal</button> : null}
    {props.terminal?.output ? <pre>{props.terminal.output}</pre> : null}
    {props.terminal && props.onTerminalInput ? <form onSubmit={(event) => void sendInput(event)}><label htmlFor="wk-chat-terminal-input">Terminal input</label><input id="wk-chat-terminal-input" value={input} onChange={(event) => setInput(event.target.value)} disabled={busy} /><button type="submit" disabled={busy || !input.trim()}>Send input</button></form> : null}
    {props.terminal && props.onCloseTerminal ? <button type="button" onClick={props.onCloseTerminal}>Close terminal</button> : null}
  </section>;
}

export function ChatPage(props: ChatPageProps) {
  const [pending, setPending] = useState<PendingChatMessage | undefined>();
  const [sending, setSending] = useState(false);
  const sendingRef = useRef(false);
  const [activeCitationId, setActiveCitationId] = useState<string | null>(null);
  const references = useMemo(() => [...messageReferenceValues(props.messages), ...(props.stream?.references ?? [])], [props.messages, props.stream?.references]);

  function activateCitation(referenceId: string): void {
    setActiveCitationId(referenceId);
    props.onCitationClick?.(referenceId);
  }

  useEffect(() => { setPending(undefined); }, [props.selectedSessionId]);

  async function send(submission: ChatSubmission) {
    if (sendingRef.current) return;
    sendingRef.current = true;
    setSending(true);
    setPending(submission);
    try {
      await props.send(submission);
      setPending(undefined);
    } catch (error) {
      setPending({
        ...submission,
        status: 'failed',
        error: error instanceof Error ? error.message : 'Message failed to send',
      });
    } finally {
      sendingRef.current = false;
      setSending(false);
    }
  }

  return <main className="wk-chat-page">
    <SessionSidebar
      sessions={props.sessions}
      selectedSessionId={props.selectedSessionId}
      loading={props.loadingSessions}
      onSelect={props.onSelectSession}
      onCreate={props.onCreateSession}
      onRename={props.onRenameSession}
      onTogglePin={props.onToggleSessionPin}
      onDelete={props.onDeleteSession}
      groups={props.sessionGroups}
      source={props.sessionSource}
      sourceOptions={props.sessionSourceOptions}
      onSourceChange={props.onSessionSourceChange}
      groupMode={props.sessionGroupMode}
      onGroupModeChange={props.onSessionGroupModeChange}
      keyword={props.sessionKeyword}
      onKeywordChange={props.onSessionKeywordChange}
      page={props.sessionPage}
      pageCount={props.sessionPageCount}
      onPageChange={props.onSessionPageChange}
    />
    <section className="wk-chat-main" aria-label="Chat">
      <h1>{props.selectedSessionId ? 'Conversation' : 'New conversation'}</h1>
      {props.agents && props.onAgentChange ? <label htmlFor="wk-chat-agent">Agent<select id="wk-chat-agent" value={props.selectedAgentId ?? ''} onChange={(event) => props.onAgentChange?.(event.target.value)}><option value="">Knowledge chat</option>{props.agents.map((agent) => <option key={agent.id} value={agent.id} disabled={agent.disabled}>{agent.name}{agent.disabled ? ' · disabled' : ''}</option>)}</select></label> : null}
      <ChatActionCards {...props} />
      {props.stream ? <LiveResponse stream={props.stream} /> : null}
      <ReferenceList references={references} activeId={activeCitationId} onActivate={activateCitation} />
      <TerminalPanel terminal={props.terminal} onOpenTerminal={props.onOpenTerminal} onTerminalInput={props.onTerminalInput} onTerminalResize={props.onTerminalResize} onCloseTerminal={props.onCloseTerminal} />
      {props.error ? <p role="alert">{props.error}</p> : null}
      {props.loadingMessages ? <p role="status">Loading messages…</p> : null}
      <MessageList
        messages={props.messages}
        pending={pending}
        onRetry={pending?.status === 'failed' ? () => void send({ content: pending.content, status: 'pending' }) : undefined}
        loadingOlder={props.loadingOlderMessages}
        hasMore={props.hasMoreMessages}
        onLoadOlder={props.onLoadOlderMessages}
        sessionId={props.selectedSessionId}
        suggestions={props.suggestions}
        onSuggestionClick={props.onSuggestionClick}
        onRefreshSuggestions={props.onRefreshSuggestions}
        onDismissSuggestions={props.onDismissSuggestions}
        onCitationClick={activateCitation}
        onArtifactDownload={props.onArtifactDownload}
      />
      {props.selectedSessionId && props.onClearSession ? <button type="button" onClick={() => void props.onClearSession!()}>Clear messages</button> : null}
      {props.selectedSessionId && props.onSteer ? <SteerComposer onSteer={props.onSteer} /> : null}
      <ChatComposer draft={props.draft} disabled={sending || pending !== undefined || props.stream?.phase === 'streaming'} onDraftChange={props.onDraftChange} onSubmit={(submission) => void send(submission)} />
    </section>
  </main>;
}
