import { useState } from 'react';
import type { ChatMessage, ChatSession } from '@weknora/contracts';
import { ChatComposer, type ChatSubmission } from './composer.tsx';
import { MessageList, type PendingChatMessage } from './message-list.tsx';
import { SessionSidebar } from './session-sidebar.tsx';

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

export function ChatPage(props: ChatPageProps) {
  const [pending, setPending] = useState<PendingChatMessage | undefined>();

  async function send(submission: ChatSubmission) {
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
    }
  }

  return <main className="wk-chat-page">
    <SessionSidebar
      sessions={props.sessions}
      selectedSessionId={props.selectedSessionId}
      loading={props.loadingSessions}
      onSelect={props.onSelectSession}
      onCreate={props.onCreateSession}
    />
    <section className="wk-chat-main" aria-label="Chat">
      <h1>{props.selectedSessionId ? 'Conversation' : 'New conversation'}</h1>
      {props.agents && props.onAgentChange ? <label htmlFor="wk-chat-agent">Agent<select id="wk-chat-agent" value={props.selectedAgentId ?? ''} onChange={(event) => props.onAgentChange?.(event.target.value)}><option value="">Knowledge chat</option>{props.agents.map((agent) => <option key={agent.id} value={agent.id} disabled={agent.disabled}>{agent.name}{agent.disabled ? ' · disabled' : ''}</option>)}</select></label> : null}
      <ChatActionCards {...props} />
      {props.error ? <p role="alert">{props.error}</p> : null}
      {props.loadingMessages ? <p role="status">Loading messages…</p> : null}
      <MessageList messages={props.messages} pending={pending} onRetry={pending?.status === 'failed' ? () => void send({ content: pending.content, status: 'pending' }) : undefined} />
      <ChatComposer draft={props.draft} onDraftChange={props.onDraftChange} onSubmit={(submission) => void send(submission)} />
    </section>
  </main>;
}
